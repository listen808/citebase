package main

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"gorm.io/gorm/schema"

	"github.com/listen808/citebase/internal/brand"
	"github.com/listen808/citebase/internal/repository"
)

// registryIndexes 表名 -> repository 中声明的复合/前缀索引，供生成 DDL 时合并。
var registryIndexes map[string][]repository.IndexSpec

func main() {
	// 表清单取自 AutoMigrate 注册表（internal/repository/migrate_*.go），
	// 保证 databases/table.sql 与启动时真实建表结果同源，不会再出现清单落后于模型的漂移。
	models := repository.RegisteredModels()

	// 复合/前缀索引不写在模型标签里，统一取自 repository 的 allIndexes，
	// 与启动时 ensureAllIndexes 完全同源，避免 DDL 与运行时索引集合出现两套真相。
	registryIndexes = map[string][]repository.IndexSpec{}
	for _, spec := range repository.AllIndexSpecs() {
		registryIndexes[spec.Table] = append(registryIndexes[spec.Table], spec)
	}

	ns := schema.NamingStrategy{
		TablePrefix:   "",
		SingularTable: true,
	}

	var sb strings.Builder
	sb.WriteString("-- ============================================================\n")
	sb.WriteString(fmt.Sprintf("-- %s 数据库表结构 DDL\n", brand.DefaultName))
	sb.WriteString("-- 由 tools/genddl 自动生成，模型清单取自 internal/repository 的 AutoMigrate 注册表，\n")
	sb.WriteString("-- 与启动时真实建表结果同源；请勿手工编辑本文件。\n")
	sb.WriteString("-- ⚠️ 每张表都带 DROP TABLE，仅适用于空库初始化；已有库请依赖启动时的 AutoMigrate 升级。\n")
	sb.WriteString("-- ============================================================\n\n")
	sb.WriteString("SET NAMES utf8mb4;\n")
	sb.WriteString("SET FOREIGN_KEY_CHECKS = 0;\n\n")

	for _, m := range models {
		ddl, err := generateTableDDL(m, ns)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error generating DDL for %T: %v\n", m, err)
			os.Exit(1)
		}
		sb.WriteString(ddl)
		sb.WriteString("\n")
	}

	sb.WriteString("SET FOREIGN_KEY_CHECKS = 1;\n")

	outPath := filepath.Join("databases", "table.sql")
	if err := os.WriteFile(outPath, []byte(sb.String()), 0600); err != nil {
		fmt.Fprintf(os.Stderr, "Error writing file: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("DDL generated: %s (%d models)\n", outPath, len(models))
}

type colDef struct {
	name     string
	sqlType  string
	notNull  bool
	def      string
	comment  string
	isPK     bool
	autoIncr bool
}

func generateTableDDL(model interface{}, ns schema.NamingStrategy) (string, error) {
	s, err := schema.Parse(model, &sync.Map{}, ns)
	if err != nil {
		return "", err
	}

	tableName := s.Table
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("-- %s\n", tableName))
	sb.WriteString(fmt.Sprintf("DROP TABLE IF EXISTS `%s`;\n", tableName))
	sb.WriteString(fmt.Sprintf("CREATE TABLE `%s` (\n", tableName))

	var cols []colDef
	var pkCols []string

	type idxCol struct {
		name   string
		fields []string
		unique bool
	}
	var extraIndexes []idxCol
	idxMap := map[string]*idxCol{}

	addIdx := func(name string, field string, unique bool) {
		if name == "" || name == "index" || name == "unique" || name == "uniqueIndex" {
			name = "idx_" + field
		}
		if existing, ok := idxMap[name]; ok {
			existing.fields = append(existing.fields, field)
		} else {
			ic := &idxCol{name: name, unique: unique, fields: []string{field}}
			idxMap[name] = ic
			extraIndexes = append(extraIndexes, *ic)
		}
	}

	for _, field := range s.Fields {
		if field.IgnoreMigration || field.DBName == "" {
			continue
		}

		c := colDef{
			name:     field.DBName,
			sqlType:  mysqlType(field),
			notNull:  field.NotNull || field.PrimaryKey,
			isPK:     field.PrimaryKey,
			autoIncr: field.AutoIncrement,
		}

		gormTag := field.Tag.Get("gorm")
		if gormTag != "" {
			settings := strings.Split(gormTag, ";")
			for _, setting := range settings {
				parts := strings.SplitN(setting, ":", 2)
				key := parts[0]
				val := ""
				if len(parts) > 1 {
					val = parts[1]
				}
				switch key {
				case "column":
					if val != "" {
						c.name = val
					}
				case "type":
					if val != "" {
						c.sqlType = val
					}
				case "size":
					if val != "" {
						c.sqlType = fmt.Sprintf("varchar(%s)", val)
					}
				case "not null", "notNull":
					c.notNull = true
				case "default":
					c.def = formatDefault(val)
				case "comment":
					c.comment = val
				case "primaryKey", "primary_key":
					c.isPK = true
				case "autoIncrement", "auto_increment":
					// autoIncrement:false 是显式关闭（复合主键列必须关闭，否则 ERROR 1075 建不了表）
					c.autoIncr = val != "false"
				case "unique":
					addIdx("uk_"+c.name, c.name, true)
				case "uniqueIndex":
					idxName := val
					if idxName == "" {
						idxName = "uk_" + c.name
					}
					addIdx(idxName, c.name, true)
				case "index":
					idxName := val
					if idxName == "" {
						idxName = "idx_" + c.name
					}
					addIdx(idxName, c.name, false)
				}
			}
		}

		// Set defaults for common patterns
		if c.isPK {
			pkCols = append(pkCols, "`"+c.name+"`")
			if c.autoIncr && strings.HasPrefix(c.sqlType, "int") == false && strings.HasPrefix(c.sqlType, "bigint") == false {
				c.sqlType = "bigint unsigned"
			}
		} else {
			if c.def == "" && c.notNull {
				if strings.HasPrefix(c.sqlType, "varchar") {
					c.def = "DEFAULT ''"
				} else if strings.HasPrefix(c.sqlType, "int") || strings.HasPrefix(c.sqlType, "bigint") ||
					strings.HasPrefix(c.sqlType, "tinyint") || strings.HasPrefix(c.sqlType, "double") ||
					strings.HasPrefix(c.sqlType, "decimal") {
					c.def = "DEFAULT 0"
				}
			}
			// 模型显式写了 default 也不行：LOB 列带任何常量默认值都建不了表
			if isLobType(c.sqlType) && c.def != "" && !strings.HasPrefix(c.def, "DEFAULT NULL") {
				c.def = ""
			}
		}

		// Timestamp fields
		if (c.name == "create_time" || c.name == "update_time") && c.def == "" {
			c.notNull = true
			c.def = "DEFAULT 0"
		}

		cols = append(cols, c)
	}

	// Add GORM-parsed indexes (dedup by field set to avoid duplicates with tag-parsed indexes)
	idxSig := func(fields []string, unique bool) string {
		sorted := make([]string, len(fields))
		copy(sorted, fields)
		sort.Strings(sorted)
		var prefix string
		if unique {
			prefix = "u:"
		} else {
			prefix = "i:"
		}
		return prefix + strings.Join(sorted, ",")
	}
	// Build signatures of already-added indexes
	existingSigs := map[string]bool{}
	for _, ic := range idxMap {
		existingSigs[idxSig(ic.fields, ic.unique)] = true
	}
	for name, idx := range s.ParseIndexes() {
		var idxFields []string
		for _, f := range idx.Fields {
			idxFields = append(idxFields, f.DBName)
		}
		isUnique := idx.Class == "UNIQUE"
		if len(idxFields) > 0 {
			sig := idxSig(idxFields, isUnique)
			if existingSigs[sig] {
				continue
			}
			if _, exists := idxMap[name]; !exists {
				idxMap[name] = &idxCol{name: name, fields: idxFields, unique: isUnique}
				existingSigs[sig] = true
			}
		}
	}

	// 合并 repository.allIndexes 声明的复合/前缀索引（模型标签无法表达的部分）
	for _, spec := range registryIndexes[tableName] {
		var fields []string
		for _, col := range strings.Split(spec.Cols, ",") {
			if col = strings.TrimSpace(col); col != "" {
				fields = append(fields, col)
			}
		}
		if len(fields) == 0 {
			continue
		}
		if _, exists := idxMap[spec.Name]; exists || existingSigs[idxSig(fields, spec.Unique)] {
			continue
		}
		idxMap[spec.Name] = &idxCol{name: spec.Name, fields: fields, unique: spec.Unique}
		extraIndexes = append(extraIndexes, *idxMap[spec.Name])
	}

	// Build column definitions
	var lines []string
	for _, c := range cols {
		line := fmt.Sprintf("  `%s` %s", c.name, c.sqlType)
		if c.notNull {
			line += " NOT NULL"
		} else if c.def == "" {
			// Only add DEFAULT NULL if no explicit default was set
			line += " DEFAULT NULL"
		}
		if c.autoIncr {
			line += " AUTO_INCREMENT"
		}
		if c.def != "" && !c.autoIncr {
			// Don't add DEFAULT twice
			if !strings.Contains(line, "DEFAULT") {
				line += " " + c.def
			}
		}
		if c.comment != "" {
			escaped := strings.ReplaceAll(c.comment, "'", "''")
			line += fmt.Sprintf(" COMMENT '%s'", escaped)
		}
		lines = append(lines, line)
	}

	// Primary key
	if len(pkCols) > 0 {
		lines = append(lines, fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(pkCols, ", ")))
	}

	// Indexes - collect unique and regular, sorted
	// Rebuild from idxMap to avoid duplicates
	var uniqIdx []idxCol
	var regIdx []idxCol
	seen := map[string]bool{}
	// First add extra indexes from tag parsing
	for _, ic := range extraIndexes {
		if ic2, ok := idxMap[ic.name]; ok {
			if seen[ic.name] {
				continue
			}
			seen[ic.name] = true
			if ic.unique {
				uniqIdx = append(uniqIdx, *ic2)
			} else {
				regIdx = append(regIdx, *ic2)
			}
		}
	}
	// Then add GORM-parsed indexes
	for name, ic := range idxMap {
		if seen[name] {
			continue
		}
		seen[name] = true
		if ic.unique {
			uniqIdx = append(uniqIdx, *ic)
		} else {
			regIdx = append(regIdx, *ic)
		}
	}

	sort.Slice(uniqIdx, func(i, j int) bool { return uniqIdx[i].name < uniqIdx[j].name })
	sort.Slice(regIdx, func(i, j int) bool { return regIdx[i].name < regIdx[j].name })

	for _, ic := range uniqIdx {
		quoted := make([]string, len(ic.fields))
		for i, f := range ic.fields {
			quoted[i] = quoteIdxCol(f)
		}
		lines = append(lines, fmt.Sprintf("  UNIQUE KEY `%s` (%s)", ic.name, strings.Join(quoted, ", ")))
	}
	for _, ic := range regIdx {
		quoted := make([]string, len(ic.fields))
		for i, f := range ic.fields {
			quoted[i] = quoteIdxCol(f)
		}
		lines = append(lines, fmt.Sprintf("  KEY `%s` (%s)", ic.name, strings.Join(quoted, ", ")))
	}

	sb.WriteString(strings.Join(lines, ",\n"))
	comment := tableComments[tableName]
	if comment == "" {
		comment = tableName
	}
	// 与既有 75 张表保持一致的排序规则：AutoMigrate 新建表跟随 DSN，DDL 引导则必须显式对齐，
	// 否则同一套模型会产出生成两套 collation 的库，JOIN 时触发 Illegal mix of collations。
	sb.WriteString(fmt.Sprintf("\n) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='%s';\n\n", comment))

	return sb.String(), nil
}

// quoteIdxCol 处理前缀索引列名：name(100) -> `name`(100)
func quoteIdxCol(col string) string {
	if i := strings.Index(col, "("); i > 0 {
		return "`" + col[:i] + "`" + col[i:]
	}
	return "`" + col + "`"
}

func mysqlType(f *schema.Field) string {
	// If GORM has explicitly determined a data type from the struct tag, use it
	if gormType := strings.TrimSpace(string(f.DataType)); gormType != "" && gormType != "-" {
		// If explicit type: is set in gorm tag, that takes priority
		if explicitType := f.Tag.Get("gorm"); explicitType != "" {
			for _, setting := range strings.Split(explicitType, ";") {
				if strings.HasPrefix(setting, "type:") {
					t := strings.TrimPrefix(setting, "type:")
					if t != "" {
						return t
					}
				}
			}
		}
		// If size is set and it's a string-like type, use varchar
		if f.Size > 0 {
			switch gormType {
			case "string", "text":
				return fmt.Sprintf("varchar(%d)", f.Size)
			}
		}
		switch gormType {
		case "bool":
			return "tinyint"
		case "int", "integer":
			if f.Size > 0 && f.Size <= 8 {
				return "tinyint"
			}
			// Check actual Go type to determine int width
			rt := f.IndirectFieldType
			if rt.Kind() == reflect.Ptr {
				rt = rt.Elem()
			}
			switch rt.Kind() {
			case reflect.Int64, reflect.Uint64:
				return "bigint"
			case reflect.Int16, reflect.Uint16:
				return "smallint"
			case reflect.Int8, reflect.Uint8:
				return "tinyint"
			default:
				return "int"
			}
		case "uint":
			// Check actual Go type to determine uint width
			rt := f.IndirectFieldType
			if rt.Kind() == reflect.Ptr {
				rt = rt.Elem()
			}
			switch rt.Kind() {
			case reflect.Uint64:
				return "bigint unsigned"
			case reflect.Uint16:
				return "smallint unsigned"
			case reflect.Uint8:
				return "tinyint unsigned"
			default:
				return "int unsigned"
			}
		case "bigint":
			return "bigint"
		case "bigint unsigned":
			return "bigint unsigned"
		case "float":
			return "float"
		case "double":
			return "double"
		case "decimal":
			return "decimal"
		case "string":
			return "longtext"
		case "text":
			return "longtext"
		case "longtext":
			return "longtext"
		case "mediumtext":
			return "mediumtext"
		case "time", "datetime":
			return "datetime"
		case "json":
			return "json"
		case "bytes":
			return "longblob"
		}
	}

	// Fallback to reflect-based detection
	rt := f.IndirectFieldType
	if rt.Kind() == reflect.Ptr {
		rt = rt.Elem()
	}

	// Check for time.Time first by pkg path
	if rt.Kind() == reflect.Struct {
		if rt.PkgPath() == "time" && rt.Name() == "Time" {
			return "datetime"
		}
		return "longtext"
	}

	switch rt.Kind() {
	case reflect.Bool:
		return "tinyint"
	case reflect.Int, reflect.Int32:
		return "int"
	case reflect.Int64:
		return "bigint"
	case reflect.Int16:
		return "smallint"
	case reflect.Int8:
		return "tinyint"
	case reflect.Uint, reflect.Uint32:
		return "int unsigned"
	case reflect.Uint64:
		return "bigint unsigned"
	case reflect.Uint16:
		return "smallint unsigned"
	case reflect.Uint8:
		return "tinyint unsigned"
	case reflect.Float32:
		return "float"
	case reflect.Float64:
		return "double"
	case reflect.String:
		if f.Size > 0 && f.Size < 65535 {
			return fmt.Sprintf("varchar(%d)", f.Size)
		}
		return "longtext"
	default:
		return "longtext"
	}
}

// isLobType 判断 MySQL 里不能带默认值的列类型：LOB/JSON/几何类型会直接触发
// ERROR 1101（"BLOB, TEXT, GEOMETRY or JSON column can't have a default value"），
// 整条 CREATE TABLE 失败，table.sql 就灌不进去。
func isLobType(sqlType string) bool {
	for _, p := range []string{"tinytext", "mediumtext", "longtext", "text",
		"tinyblob", "blob", "mediumblob", "longblob", "json", "geometry"} {
		if strings.HasPrefix(sqlType, p) {
			return true
		}
	}
	return false
}

func formatDefault(val string) string {
	if val == "" {
		return "DEFAULT ''"
	}
	// NULL / CURRENT_TIMESTAMP 是关键字，加引号就变成字符串字面量：
	// datetime DEFAULT 'NULL' 会直接 ERROR 1067 建不了表。
	if strings.EqualFold(val, "null") {
		return "DEFAULT NULL"
	}
	if isNumeric(val) || strings.EqualFold(val, "CURRENT_TIMESTAMP") {
		return "DEFAULT " + val
	}
	// Boolean literals
	if val == "true" {
		return "DEFAULT 1"
	}
	if val == "false" {
		return "DEFAULT 0"
	}
	if (strings.HasPrefix(val, "'") && strings.HasSuffix(val, "'")) ||
		(strings.HasPrefix(val, "\"") && strings.HasSuffix(val, "\"")) {
		return "DEFAULT " + val
	}
	return fmt.Sprintf("DEFAULT '%s'", val)
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	dotSeen := false
	for i, c := range s {
		if c == '.' && !dotSeen {
			dotSeen = true
			continue
		}
		if c == '-' && i == 0 {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

var tableComments = map[string]string{
	"aw_user":                          "用户表",
	"aw_role":                          "角色表",
	"aw_privilege":                     "权限表",
	"aw_role_privilege":                "角色权限关联表",
	"aw_contact":                       "联系人表",
	"aw_space":                         "空间表",
	"aw_space_user":                    "空间成员表",
	"aw_document":                      "文档表",
	"aw_document_version":              "文档版本表",
	"aw_document_link":                 "文档链接表",
	"aw_document_edit_lock":            "文档编辑锁",
	"aw_document_trash":                "文档回收站",
	"aw_attachment":                    "附件表",
	"aw_collection":                    "收藏表",
	"aw_follow":                        "关注表",
	"aw_search_log":                    "搜索日志表",
	"aw_comment":                       "评论表",
	"aw_config":                        "系统配置表",
	"aw_email":                         "邮件配置表",
	"aw_login_auth":                    "第三方登录配置表",
	"aw_link":                          "快捷链接表",
	"aw_plugin":                        "插件表",
	"aw_log":                           "系统日志表",
	"aw_log_document":                  "文档操作日志表",
	"aw_ai_conversation":               "AI对话表",
	"aw_ai_audit_log":                  "AI审计日志",
	"aw_ai_memory":                     "AI记忆片段",
	"aw_ai_memory_summary":             "AI记忆摘要",
	"aw_ai_task":                       "AI分析任务",
	"aw_ai_doc_mapping":                "AI文档映射",
	"aw_ai_extracted_entities":         "AI提取实体",
	"aw_ai_doc_cluster":                "AI文档聚类",
	"aw_vector_sync_task":              "向量同步任务",
	"aw_knowledge_entity":              "知识图谱实体",
	"aw_knowledge_relation":            "知识图谱关系",
	"aw_ai_feedback":                   "AI反馈表",
	"aw_ai_ragas_eval":                 "RAGAS评估表",
	"aw_ai_analysis_result":            "AI分析结果",
	"aw_ai_datasource":                 "AI数据源",
	"aw_ai_sql_history":                "SQL执行历史",
	"aw_ai_sql_security_policy":        "SQL安全策略",
	"aw_ai_business_rule":              "业务规则",
	"aw_ai_table_cache":                "表结构缓存",
	"aw_ai_table_relation":             "表关系",
	"aw_ai_prompt_template":            "提示词模板",
	"aw_ai_sql_example":                "SQL示例",
	"aw_ai_business_glossary":          "业务术语表",
	"aw_ai_data_sensitivity_rules":     "数据敏感性规则",
	"aw_ai_text2sql_feedback":          "Text2SQL反馈",
	"aw_text2sql_conversation_turn":    "Text2SQL对话轮次",
	"aw_text2sql_conversation_summary": "Text2SQL对话摘要",
	"aw_text2sql_turn_embedding":       "Text2SQL轮次嵌入",
	"aw_text2sql_session":              "Text2SQL会话",
	"aw_ai_semantic_metric":            "语义指标",
	"aw_ai_semantic_dimension":         "语义维度",
	"aw_ai_column_stats":               "列统计信息",
	"aw_ai_sql_trace":                  "SQL追踪",
	"aw_ai_prompt_version":             "提示词版本",
	"aw_ai_ab_experiment":              "AB实验",
	"aw_schema_space":                  "Schema空间",
	"aw_schema_table":                  "Schema表",
	"aw_schema_column":                 "Schema列",
	"aw_schema_relation":               "Schema关系",
	"aw_schema_business_tag":           "Schema业务标签",
	"aw_schema_scan_task":              "Schema扫描任务",

	// 以下原为 table.sql 手工维护的表，纳入注册表后由本工具统一生成
	"aw_document_approval_step":  "审批步骤表",
	"aw_document_collab_session": "Document collaborative editing sessions",
	"aw_document_collab_state":   "Document collaborative editing state",
	"aw_document_version_tag":    "文档版本标签/里程碑表",
	"aw_notification":            "通知表",
	"aw_oidc_provider":           "OIDC 身份提供方配置",
	"aw_oidc_user_mapping":       "用户 OIDC 身份映射",
	"aw_saved_search":            "保存的搜索表",
	"aw_tag_hierarchy":           "标签层级表",
	"aw_webhook_config":          "Webhook 配置表",
	"aw_webhook_delivery":        "Webhook 投递记录表",

	// 此前仅靠 AutoMigrate 建出、未进入 DDL 的表
	"aw_api_key":              "开放平台API Key",
	"aw_document_approval":    "文档审批单",
	"aw_document_mention":     "文档@提及表",
	"aw_document_tag":         "文档标签关联表",
	"aw_document_template":    "文档模板表",
	"aw_document_translation": "文档翻译表",
	"aw_document_view_log":    "文档视图模式记录",
	"aw_eval_dataset":         "RAG评估数据集",
	"aw_eval_result":          "RAG评估结果",
	"aw_git_sync_config":      "Git同步配置",
	"aw_git_sync_log":         "Git同步日志",
	"aw_wiki_link":            "Wiki链接表",
	"aw_wiki_lint_issue":      "Wiki治理问题表",
	"aw_wiki_page":            "Wiki页面表",
	"aw_wiki_pin":             "Wiki置顶表",
	"aw_wiki_review":          "Wiki评审表",
}
