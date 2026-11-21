package text2sql

import (
	"context"
	"fmt"
	"strings"

	"github.com/listen808/citebase/internal/ai/embedding"
	schemamodel "github.com/listen808/citebase/internal/model/schema"
)

// SchemaRepoInterface Schema Repository 接口（支持 root-level 和 sub-package 两种实现）
type SchemaRepoInterface interface {
	FindTableByName(schemaSpaceID int, tableName string) (*schemamodel.SchemaTable, error)
	ListAllTables(schemaSpaceID int) ([]*schemamodel.SchemaTable, error)
	ListRelations(schemaSpaceID int) ([]*schemamodel.SchemaRelation, error)
	ListColumnsByTableID(tableID int) ([]*schemamodel.SchemaColumn, error)
	FindSpaceByWikiSpaceID(spaceID int) (*schemamodel.SchemaSpace, error)
	ListSpaces(page, pageSize int) ([]*schemamodel.SchemaSpace, int64, error)
	ListColumnsByTableName(schemaSpaceID int, tableName string) ([]*schemamodel.SchemaColumn, error)
}

// GraphEntityInterface P2-6: 知识图谱实体（接口抽象，避免循环依赖）
type GraphEntityInterface interface {
	GetName() string
	GetType() string
	GetAliases() []string
	GetMappedTable() string // 实体映射的表名（可能为空）
}

// GraphRelationInterface P2-6: 知识图谱关系（接口抽象）
type GraphRelationInterface interface {
	GetSourceEntity() string
	GetTargetEntity() string
	GetRelationType() string
	GetSourceTable() string // 映射的源表名（可能为空）
	GetTargetTable() string // 映射的目标表名（可能为空）
}

// GraphRepoInterface P2-6: 知识图谱仓库接口（由 knowledge/graph.go 实现）
type GraphRepoInterface interface {
	FindEntitiesBySpaceID(spaceID int) ([]GraphEntityInterface, error)
	FindRelationsBySpaceID(spaceID int) ([]GraphRelationInterface, error)
}

// LinkedSchema Schema Linker 的输出：精简后的 Schema 子集
type LinkedSchema struct {
	Tables    []*schemamodel.SchemaTable             // 候选表
	Columns   map[string][]*schemamodel.SchemaColumn // tableName -> columns
	Relations []*schemamodel.SchemaRelation          // 表间关系
}

// SchemaLinker 三阶段 Schema Linking：向量检索候选表 + 列过滤 + 外键自动发现
type SchemaLinker struct {
	schemaVS   *embedding.SchemaVectorStore
	schemaRepo SchemaRepoInterface
	fkDiscover *ForeignKeyDiscoverer
	graphRepo  GraphRepoInterface // P2-6: 知识图谱仓库（可选）
}

// NewSchemaLinker 创建 Schema Linker（接受 SchemaRepoInterface，支持任意实现）
func NewSchemaLinker(svs *embedding.SchemaVectorStore, repo SchemaRepoInterface) *SchemaLinker {
	return &SchemaLinker{schemaVS: svs, schemaRepo: repo}
}

// SetForeignKeyDiscoverer 设置外键发现器
func (sl *SchemaLinker) SetForeignKeyDiscoverer(fkd *ForeignKeyDiscoverer) {
	sl.fkDiscover = fkd
}

// SetGraphRepo P2-6: 设置知识图谱仓库（用于增强 Schema Linking）
func (sl *SchemaLinker) SetGraphRepo(repo GraphRepoInterface) {
	sl.graphRepo = repo
}

// Link 执行三阶段 Schema Linking
// schemaSpaceID 为 schema_space_id（不是 wiki space_id）
func (sl *SchemaLinker) Link(ctx context.Context, question string, schemaSpaceID int) (*LinkedSchema, error) {
	tables, err := sl.filterTables(ctx, question, schemaSpaceID)
	if err != nil {
		return nil, fmt.Errorf("stage1 filter tables: %w", err)
	}
	linked, err := sl.filterColumns(ctx, question, tables, schemaSpaceID)
	if err != nil {
		return nil, err
	}

	// Stage3: 自动发现外键关系并合并到 Relations 中
	if sl.fkDiscover != nil && len(tables) > 1 {
		discoveredFKs, err := sl.fkDiscover.Discover(ctx, tables, schemaSpaceID)
		if err == nil && len(discoveredFKs) > 0 {
			// 合并发现的外键到 Relations
			for _, fk := range discoveredFKs {
				if fk.Confidence >= GetText2SQLConfig().FKConfidenceThreshold {
					linked.Relations = append(linked.Relations, &schemamodel.SchemaRelation{
						SourceTable:  fk.SourceTable,
						SourceColumn: fk.SourceColumn,
						TargetTable:  fk.TargetTable,
						TargetColumn: fk.TargetColumn,
						RelationType: "inferred",
					})
				}
			}
		}
	}

	// P2-6: 知识图谱业务关系补充（增强 Schema Linking 精度）
	if sl.graphRepo != nil && schemaSpaceID > 0 {
		sl.enrichWithGraphRelations(linked, schemaSpaceID)
	}

	return linked, nil
}

// enrichWithGraphRelations P2-6: 用知识图谱的业务关系补充 Schema Relations
// 业务关系（如"订单属于用户"）即使无物理 FK，也能指导 LLM 生成正确的 JOIN
func (sl *SchemaLinker) enrichWithGraphRelations(linked *LinkedSchema, schemaSpaceID int) {
	relations, err := sl.graphRepo.FindRelationsBySpaceID(schemaSpaceID)
	if err != nil || len(relations) == 0 {
		return
	}

	// 已有 Relations 的 key 集合（去重用）
	existing := make(map[string]bool)
	for _, r := range linked.Relations {
		key := r.SourceTable + "|" + r.TargetTable
		existing[key] = true
	}

	// 把图谱关系映射为 SchemaRelation（仅当源/目标表名都有效时）
	tableSet := make(map[string]bool)
	for _, t := range linked.Tables {
		tableSet[t.Name] = true
	}

	for _, gr := range relations {
		srcTable := gr.GetSourceTable()
		tgtTable := gr.GetTargetTable()
		if srcTable == "" || tgtTable == "" {
			continue
		}
		// 仅补充涉及候选表的关系
		if !tableSet[srcTable] || !tableSet[tgtTable] {
			continue
		}
		key := srcTable + "|" + tgtTable
		if existing[key] {
			continue
		}
		linked.Relations = append(linked.Relations, &schemamodel.SchemaRelation{
			SourceTable:  srcTable,
			TargetTable:  tgtTable,
			RelationType: "business_graph", // 标记为业务图谱关系
		})
		existing[key] = true
	}
}

// filterTables Stage1: 精确表名匹配 + 向量检索 + 别名匹配 + 知识图谱实体映射 -> 候选表
func (sl *SchemaLinker) filterTables(ctx context.Context, question string, schemaSpaceID int) ([]*schemamodel.SchemaTable, error) {
	var tables []*schemamodel.SchemaTable
	tableSet := make(map[string]bool) // 去重用

	// 0. 优先：从问题中直接提取表名做精确匹配（用户明确提到表名时最可靠）
	explicitTables := extractTableNamesFromQuestion(question)
	for _, name := range explicitTables {
		if tableSet[name] {
			continue
		}
		t, err := sl.schemaRepo.FindTableByName(schemaSpaceID, name)
		if err == nil && t != nil {
			tables = append(tables, t)
			tableSet[name] = true
		}
	}

	// 1. 向量检索
	if sl.schemaVS != nil {
		results, err := sl.schemaVS.Search(question, GetText2SQLConfig().SchemaTopKTables)
		if err == nil && len(results) > 0 {
			tableNames := extractUniqueTableNames(results, 10)
			for _, name := range tableNames {
				if tableSet[name] {
					continue
				}
				t, err := sl.schemaRepo.FindTableByName(schemaSpaceID, name)
				if err == nil && t != nil {
					tables = append(tables, t)
					tableSet[name] = true
				}
			}
		}
	}

	// P2-6: 知识图谱实体映射增强（向量检索召回率低时的补充）
	if sl.graphRepo != nil && schemaSpaceID > 0 {
		graphTables := sl.findTablesByGraphEntities(question, schemaSpaceID, tableSet)
		tables = append(tables, graphTables...)
	}

	if len(tables) > 0 {
		return tables, nil
	}
	// fallback: 返回全部表
	return sl.schemaRepo.ListAllTables(schemaSpaceID)
}

// extractTableNamesFromQuestion 从用户问题中提取可能的表名
// 支持下划线命名、驼峰命名、以及常见 SQL 表名模式
func extractTableNamesFromQuestion(question string) []string {
	lower := strings.ToLower(question)
	var names []string
	seen := make(map[string]bool)

	// 策略1: 匹配常见的下划线命名表名（如 bm_config_version, t_user_info）
	// 表名通常包含字母、数字、下划线，至少两个单词段
	words := strings.FieldsFunc(lower, func(r rune) bool {
		return r == ' ' || r == ',' || r == '，' || r == '、' || r == ';' || r == '；' || r == '\n' || r == '\t'
	})
	for _, w := range words {
		// 匹配 bm_xxx, t_xxx, sys_xxx 等常见前缀的表名
		if strings.Contains(w, "_") && len(w) >= 4 {
			// 清理可能的标点
			clean := strings.TrimFunc(w, func(r rune) bool {
				return r == '.' || r == '?' || r == '！' || r == '"' || r == '\'' || r == ')' || r == '】'
			})
			if len(clean) >= 4 && !seen[clean] {
				names = append(names, clean)
				seen[clean] = true
			}
		}
	}

	// 策略2: 匹配 "表 xxx" / "table xxx" 后的词
	if idx := strings.Index(lower, "表"); idx >= 0 {
		rest := strings.TrimSpace(lower[idx+3:]) // 跳过"表"字
		parts := strings.FieldsFunc(rest, func(r rune) bool {
			return r == ' ' || r == ',' || r == '，' || r == '、'
		})
		if len(parts) > 0 {
			first := parts[0]
			first = strings.TrimFunc(first, func(r rune) bool {
				return r == '的' || r == '中' || r == '里' || r == '内' || r == '下'
			})
			if len(first) >= 2 && !seen[first] {
				names = append(names, first)
				seen[first] = true
			}
		}
	}

	return names
}

// findTablesByGraphEntities P2-6: 通过知识图谱实体名/别名匹配问题文本，发现候选表
func (sl *SchemaLinker) findTablesByGraphEntities(question string, schemaSpaceID int, existingTables map[string]bool) []*schemamodel.SchemaTable {
	entities, err := sl.graphRepo.FindEntitiesBySpaceID(schemaSpaceID)
	if err != nil || len(entities) == 0 {
		return nil
	}

	qLower := strings.ToLower(question)
	var tables []*schemamodel.SchemaTable

	for _, ent := range entities {
		matched := false
		// 检查实体名
		if name := ent.GetName(); name != "" && strings.Contains(qLower, strings.ToLower(name)) {
			matched = true
		}
		// 检查别名
		if !matched {
			for _, alias := range ent.GetAliases() {
				if alias != "" && strings.Contains(qLower, strings.ToLower(alias)) {
					matched = true
					break
				}
			}
		}
		if !matched {
			continue
		}

		// 实体匹配成功，尝试映射到表
		mappedTable := ent.GetMappedTable()
		if mappedTable == "" || existingTables[mappedTable] {
			continue
		}
		t, err := sl.schemaRepo.FindTableByName(schemaSpaceID, mappedTable)
		if err == nil && t != nil {
			tables = append(tables, t)
			existingTables[mappedTable] = true
		}
	}
	return tables
}

// filterColumns Stage2: 列级语义检索，只返回与问题相关的列 + 必需列（主键/外键）
// 这是 C3-SQL/DIN-SQL 的核心策略：减少 Prompt 噪声，提高 LLM 注意力聚焦
func (sl *SchemaLinker) filterColumns(ctx context.Context, question string, tables []*schemamodel.SchemaTable, schemaSpaceID int) (*LinkedSchema, error) {
	linked := &LinkedSchema{
		Tables:    tables,
		Columns:   make(map[string][]*schemamodel.SchemaColumn),
		Relations: make([]*schemamodel.SchemaRelation, 0),
	}

	// 获取所有关系（用于确定外键列）
	allRels, _ := sl.schemaRepo.ListRelations(schemaSpaceID)

	// 构建外键列集合（必须包含）
	fkColumns := make(map[string]map[string]bool) // tableName -> {columnName: true}
	for _, rel := range allRels {
		if _, ok := fkColumns[rel.SourceTable]; !ok {
			fkColumns[rel.SourceTable] = make(map[string]bool)
		}
		if _, ok := fkColumns[rel.TargetTable]; !ok {
			fkColumns[rel.TargetTable] = make(map[string]bool)
		}
		fkColumns[rel.SourceTable][rel.SourceColumn] = true
		fkColumns[rel.TargetTable][rel.TargetColumn] = true
	}

	// 用向量检索获取与问题相关的列级 chunk
	var relevantColumns map[string]map[string]bool // tableName -> {columnName: true}
	if sl.schemaVS != nil {
		relevantColumns = sl.retrieveRelevantColumns(question, tables)
	}

	for _, t := range tables {
		allCols, _ := sl.schemaRepo.ListColumnsByTableID(t.ID)

		// 如果列数 <= 15，直接返回全部列（无需过滤）
		if len(allCols) <= 15 {
			linked.Columns[t.Name] = allCols
			continue
		}

		// 列级过滤策略：
		// 1. 主键列（必须包含）
		// 2. 外键列（必须包含）
		// 3. 向量检索相关的列
		// 4. 关键词匹配的列
		// 5. 最多 20 列
		selectedCols := make([]*schemamodel.SchemaColumn, 0, 20)
		selectedNames := make(map[string]bool)

		// 策略1+2: 必须包含的列（主键 + 外键）
		for _, col := range allCols {
			isPK := col.IsPrimaryKey == 1
			isFK := fkColumns[t.Name] != nil && fkColumns[t.Name][col.ColumnName]
			if isPK || isFK {
				selectedCols = append(selectedCols, col)
				selectedNames[col.ColumnName] = true
			}
		}

		// 策略3: 向量检索相关的列
		if relevantColumns != nil && relevantColumns[t.Name] != nil {
			for _, col := range allCols {
				if selectedNames[col.ColumnName] {
					continue
				}
				if relevantColumns[t.Name][col.ColumnName] {
					selectedCols = append(selectedCols, col)
					selectedNames[col.ColumnName] = true
				}
			}
		}

		// 策略4: 关键词匹配的列
		keywords := extractColumnKeywords(question)
		for _, col := range allCols {
			if selectedNames[col.ColumnName] {
				continue
			}
			if len(selectedCols) >= 20 {
				break
			}
			colLower := strings.ToLower(col.ColumnName)
			colCommentLower := strings.ToLower(col.ColumnComment)
			for _, kw := range keywords {
				if strings.Contains(colLower, kw) || strings.Contains(colCommentLower, kw) {
					selectedCols = append(selectedCols, col)
					selectedNames[col.ColumnName] = true
					break
				}
			}
		}

		// 如果过滤后为空，返回全部列（兜底）
		if len(selectedCols) == 0 {
			linked.Columns[t.Name] = allCols
		} else {
			linked.Columns[t.Name] = selectedCols
		}
	}

	linked.Relations = allRels
	return linked, nil
}

// retrieveRelevantColumns 使用向量检索获取与问题相关的列
func (sl *SchemaLinker) retrieveRelevantColumns(question string, tables []*schemamodel.SchemaTable) map[string]map[string]bool {
	results, err := sl.schemaVS.Search(question, GetText2SQLConfig().SchemaTopKColumns)
	if err != nil {
		return nil
	}

	relevantCols := make(map[string]map[string]bool) // tableName -> {columnName: true}
	for _, r := range results {
		tableName, colName := parseTableAndColumnFromSchemaName(r.Name)
		if tableName == "" || colName == "" {
			continue
		}
		// 检查是否在候选表中
		found := false
		for _, t := range tables {
			if t.Name == tableName {
				found = true
				break
			}
		}
		if !found {
			continue
		}
		if relevantCols[tableName] == nil {
			relevantCols[tableName] = make(map[string]bool)
		}
		relevantCols[tableName][colName] = true
	}
	return relevantCols
}

// parseTableAndColumnFromSchemaName 从 "schema:chunkType:tableName:columnName" 提取表名和列名
func parseTableAndColumnFromSchemaName(name string) (string, string) {
	parts := strings.Split(name, ":")
	// schema:chunkType:tableName[:columnName]
	if len(parts) >= 4 {
		return parts[2], parts[3]
	}
	if len(parts) == 3 {
		return parts[2], ""
	}
	return "", ""
}

// extractColumnKeywords 从问题中提取用于列匹配的关键词
func extractColumnKeywords(question string) []string {
	lower := strings.ToLower(question)
	// 常见字段相关关键词
	keywords := []string{}
	// 英文分词
	words := strings.Fields(lower)
	for _, w := range words {
		if len(w) >= 2 {
			keywords = append(keywords, w)
		}
	}
	// 常见业务术语
	termPatterns := []string{
		"id", "name", "code", "type", "status", "value", "val",
		"date", "time", "desc", "amount", "count", "price",
		"编码", "名称", "类型", "状态", "值", "时间", "描述", "金额", "数量",
		"标识", "代码", "备注", "说明", "排序", "分类", "级别",
	}
	for _, p := range termPatterns {
		if strings.Contains(lower, p) {
			keywords = append(keywords, p)
		}
	}
	return keywords
}

// extractUniqueTableNames 从向量检索结果中提取去重的表名
// SchemaSearchResult.Name 格式: "schema:chunkType:tableName:columnName"
func extractUniqueTableNames(results []embedding.SchemaSearchResult, max int) []string {
	seen := make(map[string]bool)
	names := make([]string, 0, max)
	for _, r := range results {
		tableName := parseTableNameFromSchemaName(r.Name)
		if tableName == "" || seen[tableName] {
			continue
		}
		seen[tableName] = true
		names = append(names, tableName)
		if len(names) >= max {
			break
		}
	}
	return names
}

// parseTableNameFromSchemaName 从 "schema:chunkType:tableName:columnName" 中提取 tableName
func parseTableNameFromSchemaName(name string) string {
	parts := strings.Split(name, ":")
	// schema:chunkType:tableName[:columnName]
	if len(parts) >= 3 {
		return parts[2]
	}
	return ""
}
