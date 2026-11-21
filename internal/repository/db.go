package repository

import (
	"crypto/rand"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/brand"
	"github.com/listen808/citebase/internal/cache"
	logx "github.com/listen808/citebase/internal/log"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	usermodel "github.com/listen808/citebase/internal/model/user"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/config"
	"github.com/listen808/citebase/internal/utils/envname"
	"github.com/listen808/citebase/internal/version"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

var DB *gorm.DB

// AppCache is the global cache instance initialized by InitCache.
// It is either a RedisCache (when Redis is enabled and reachable)
// or a MemoryCache fallback.
var AppCache cache.Cache

// indexDef 索引定义
type indexDef struct {
	table  string // 表名
	name   string // 索引名
	unique bool   // 是否唯一索引
	cols   string // 索引列（逗号分隔）
}

// allIndexes 复合索引与前缀索引定义汇总
// 单列索引/唯一索引通过 GORM 标签(uniqueIndex/index)由 AutoMigrate 自动管理
// 这里只声明：①复合索引（多列）；②前缀索引（如 name(100)，GORM 标签不支持）
var allIndexes = []indexDef{
	// aw_document_permission（P1-3 文档级 ACL）
	{table: "aw_document_permission", name: "uk_doc_subject", unique: true, cols: "doc_id, subject_type, subject_id"},
	// aw_wiki_page_history（Wiki 页版本历史：按页取全部版本）
	{table: "aw_wiki_page_history", name: "idx_wiki_hist_space_slug", unique: false, cols: "space_id, slug, version"},
	// aw_ai_usage_daily（P1-6 用量日聚合底账）
	{table: "aw_ai_usage_daily", name: "uk_usage_day", unique: true, cols: "date, user_id, provider, model, action"},
	// aw_space_user 的唯一索引名必须与 model 的 uniqueIndex tag 同名（idx_user_space），
	// 原先写 uk_user_space 会让 ensureAllIndexes 在按 table.sql 建好的库上重建一份同列副本。
	{table: "aw_space_user", name: "idx_user_space", unique: true, cols: "user_id, space_id"},
	{table: "aw_space_user", name: "idx_space_user", unique: false, cols: "space_id, user_id"},
	// aw_document
	{table: "aw_document", name: "idx_space_parent_delete", unique: false, cols: "space_id, parent_id, is_delete"},
	{table: "aw_document", name: "uk_space_parent_name", unique: true, cols: "space_id, parent_id, name"},
	// aw_document_version
	{table: "aw_document_version", name: "idx_doc_version", unique: false, cols: "document_id, version"},
	// aw_document_tag
	{table: "aw_document_tag", name: "uk_doc_tag", unique: true, cols: "document_id, tag_name"},
	{table: "aw_document_tag", name: "idx_tag_name", unique: false, cols: "tag_name"},
	// aw_collection
	{table: "aw_collection", name: "uk_user_resource", unique: true, cols: "user_id, type, resource_id"},
	// aw_follow
	// aw_follow 的索引名必须与 model/social/follow.go 的 uniqueIndex tag 同名（idx_user_object_type）。
	// 这里原先写的是 uk_user_object：ensureAllIndexes 是按**索引名**判断存在与否的，
	// 于是用 table.sql 建好的库（只有 idx_user_object_type）一起服务就被判成"缺索引"，
	// 再补一条列完全相同的第二个唯一索引 —— 白付一份写放大，还让 DDL 与运行时注册表各执一名。
	{table: "aw_follow", name: "idx_user_object_type", unique: true, cols: "user_id, type, object_id"},
	// aw_log
	{table: "aw_log", name: "idx_level_username", unique: false, cols: "level, username"},
	// aw_search_log（索引监控页的趋势/热词/无结果率聚合底账）
	// (create_time, keyword)：范围扫窗口 + 按词分组，趋势与热词共用
	// (user_id, create_time)：独立搜索用户数，以及按人回看检索行为
	{table: "aw_search_log", name: "idx_search_log_time_keyword", unique: false, cols: "create_time, keyword"},
	{table: "aw_search_log", name: "idx_search_log_user_time", unique: false, cols: "user_id, create_time"},
	// aw_attachment
	{table: "aw_attachment", name: "idx_document_source", unique: false, cols: "document_id, source"},
	// Schema Intelligence
	{table: "aw_schema_table", name: "uk_schema_table", unique: true, cols: "schema_space_id, table_name"},
	{table: "aw_schema_column", name: "uk_table_column", unique: true, cols: "table_id, column_name"},
	{table: "aw_schema_relation", name: "uk_relation", unique: true, cols: "schema_space_id, source_table, source_column, target_table, target_column"},
	{table: "aw_schema_relation", name: "idx_target", unique: false, cols: "schema_space_id, target_table"},
	{table: "aw_schema_business_tag", name: "idx_schema_tag", unique: false, cols: "schema_space_id, tag_name"},
	// aw_ai_memory
	{table: "aw_ai_memory", name: "idx_user_type", unique: false, cols: "user_id, memory_type"},
	{table: "aw_ai_memory", name: "idx_user_importance", unique: false, cols: "user_id, importance"},
	// P1-7: 记忆分层查询索引（按用户+层级过滤，如"某用户的长期记忆"）
	{table: "aw_ai_memory", name: "idx_user_tier", unique: false, cols: "user_id, memory_tier"},
	// aw_ai_sql_example
	{table: "aw_ai_sql_example", name: "idx_space_verified", unique: false, cols: "space_id, verified"},
	// aw_ai_business_glossary
	// 模型 tag 是 space_id 单列（business_glossary.go 的 index:idx_space_term），此处列集与之对齐
	{table: "aw_ai_business_glossary", name: "idx_space_term", unique: false, cols: "space_id"},
	// aw_knowledge_entity (前缀索引，GORM 标签不支持)
	{table: "aw_knowledge_entity", name: "idx_ke_name", unique: false, cols: "name(100)"},
	// aw_ai_data_sensitivity_rules
	{table: "aw_ai_data_sensitivity_rules", name: "idx_space_table_field", unique: false, cols: "space_id, table_name, field_name"},
	// aw_ai_text2sql_feedback
	{table: "aw_ai_text2sql_feedback", name: "idx_t2sf_space_created", unique: false, cols: "space_id, created_at"},
	// aw_ai_semantic_metric
	{table: "aw_ai_semantic_metric", name: "idx_sm_space_code", unique: true, cols: "space_id, code"},
	// aw_ai_semantic_dimension
	{table: "aw_ai_semantic_dimension", name: "idx_sd_space_code", unique: true, cols: "space_id, code"},
	// aw_ai_column_stats
	{table: "aw_ai_column_stats", name: "idx_cs_space_table", unique: false, cols: "space_id, table_name"},
	{table: "aw_ai_column_stats", name: "idx_cs_table_col", unique: true, cols: "space_id, table_name, column_name"},
	// aw_ai_prompt_version
	{table: "aw_ai_prompt_version", name: "idx_pv_space_query", unique: false, cols: "space_id, query_type"},
	// aw_ai_sql_trace
	{table: "aw_ai_sql_trace", name: "idx_sql_trace_space_created", unique: false, cols: "space_id, created_at"},
	// P1: @提及
	{table: "aw_document_mention", name: "idx_mention_user", unique: false, cols: "user_id"},
	// P1: 审批流
	{table: "aw_document_approval", name: "idx_approval_status", unique: false, cols: "status, approver"},
	{table: "aw_document_approval", name: "idx_approval_space", unique: false, cols: "space_id"},
	// P1: 文档模板
	{table: "aw_document_template", name: "idx_tpl_space", unique: false, cols: "space_id"},
	// P1: G029 API 开放平台
	{table: "aw_api_key", name: "idx_owner_id", unique: false, cols: "owner_id"},
	{table: "aw_api_key", name: "idx_apikey_status", unique: false, cols: "status"},
	// 原 table.sql 手工维护、模型标签无法表达的复合索引
	{table: "aw_log", name: "idx_level_create_time", unique: false, cols: "level, create_time"},
	{table: "aw_notification", name: "idx_user_read", unique: false, cols: "user_id, is_read"},
	{table: "aw_oidc_user_mapping", name: "idx_oidc_provider_subject", unique: false, cols: "provider_id, subject"},
	// LLM Wiki 编译产物
	{table: "aw_wiki_page", name: "uk_wiki_space_slug", unique: true, cols: "space_id, slug"},
	{table: "aw_wiki_page", name: "idx_wiki_space_status", unique: false, cols: "space_id, status"},
	{table: "aw_wiki_link", name: "uk_wiki_link", unique: true, cols: "space_id, from_slug, to_slug"},
	{table: "aw_wiki_pin", name: "idx_wiki_pin_page", unique: false, cols: "space_id, page_slug, status"},
	{table: "aw_wiki_review", name: "idx_wiki_review_status", unique: false, cols: "space_id, status"},
	{table: "aw_wiki_lint_issue", name: "idx_wiki_lint_space", unique: false, cols: "space_id, status, category"},
}

// gormLogWriter 过滤GORM启动时无害的Error 1091 (Can't DROP)警告，保留慢查询等其他重要日志
type gormLogWriter struct {
	w io.Writer
}

func newGORMLogWriter(w io.Writer) io.Writer {
	return &gormLogWriter{w: w}
}

func (g *gormLogWriter) Write(p []byte) (n int, err error) {
	msg := string(p)
	// 1091 的真实文本是 "Can't DROP '<名字>'; check that column/key exists"——
	// 无论误判成外键还是索引都长这样，早期按 "Can't DROP FOREIGN KEY"/"Can't DROP INDEX"
	// 匹配一个都套不上，190 条警告全数漏过。这些 DROP 全部来自 AutoMigrate 对
	// dropLegacyIndexes 已清掉的旧 uni_aw_* 约束名的例行尝试，属启动噪音。
	if strings.Contains(msg, "Error 1091 (42000)") {
		return len(p), nil
	}
	return g.w.Write(p)
}

func InitDB() {
	var err error
	cfg := config.AppConfig.Database

	// 修复 MySQL 时区表缺失导致的连接错误
	// 将 loc=Local/UTC 替换为 loc=UTC（先尝试），失败时回退到去除 loc 参数
	// MySQL 未导入时区表时，loc=Local 和 loc=UTC 都会报错 "unknown time zone"
	// 去除 loc 参数后 Go 驱动默认使用 UTC
	dsn := cfg.DSN
	if strings.Contains(dsn, "loc=Local") || strings.Contains(dsn, "loc=UTC") {
		// 去除整个 loc=xxx 参数（保留其他参数）
		dsn = strings.Replace(dsn, "&loc=Local", "", 1)
		dsn = strings.Replace(dsn, "&loc=UTC", "", 1)
		dsn = strings.Replace(dsn, "?loc=Local&", "?", 1)
		dsn = strings.Replace(dsn, "?loc=UTC&", "?", 1)
		dsn = strings.Replace(dsn, "?loc=Local", "", 1)
		dsn = strings.Replace(dsn, "?loc=UTC", "", 1)
		logx.Infof("[DB] Removed loc parameter from DSN (MySQL timezone tables not loaded)")
	}

	gormLogger := gormlogger.New(
		log.New(newGORMLogWriter(utils.LogWriter()), "[gorm] ", log.LstdFlags|log.Lmicroseconds),
		gormlogger.Config{
			// P2-4: 慢查询阈值由 200ms 提升至 500ms，避免复杂联表查询误报
			SlowThreshold:             500 * time.Millisecond,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{
		SkipDefaultTransaction: true,
		Logger:                 gormLogger,
		// 外键约束不交给 GORM 迁移管理：模型未声明关联外键，让 AutoMigrate
		// 自作主张 DROP 只会在老库上反复刷 "Error 1091 Can't DROP FOREIGN KEY" 警告。
		DisableForeignKeyConstraintWhenMigrating: true,
	})
	if err != nil {
		logx.Fatalf("Failed to connect to database: %v", err)
	}

	sqlDB, err := DB.DB()
	if err != nil {
		logx.Fatalf("Failed to get sql.DB: %v", err)
	}

	// 设置连接池配置
	sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)

	// 设置连接生命周期（如果配置了）
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(cfg.ConnMaxLifetime) * time.Second)
	} else {
		sqlDB.SetConnMaxLifetime(time.Hour) // 默认1小时
	}

	if cfg.ConnMaxIdleTime > 0 {
		sqlDB.SetConnMaxIdleTime(time.Duration(cfg.ConnMaxIdleTime) * time.Second)
	} else {
		sqlDB.SetConnMaxIdleTime(10 * time.Minute) // 默认10分钟
	}

	// 清理旧版本残留的冗余索引（uni_aw_* 前缀的单列唯一索引会导致 GORM 误识别为外键）
	dropLegacyIndexes()

	// 第一步：AutoMigrate 创建/更新表结构
	// 生产环境多实例部署时，设置 CITEBASE_SKIP_MIGRATE=true 仅在一个实例上运行迁移（旧名 AIWIKI_SKIP_MIGRATE 仍可读）
	skipMigrate := envname.Getenv("CITEBASE_SKIP_MIGRATE", "AIWIKI_SKIP_MIGRATE") == "true"
	if skipMigrate {
		logx.Info("[migrate] Skipping AutoMigrate (CITEBASE_SKIP_MIGRATE=true)")
	} else {
		// 从注册表读取所有领域注册的模型（见 migrate_*.go），注册顺序即迁移顺序。
		models := registeredModels()
		skippedUniqueNoise := 0
		for _, m := range models {
			if err := DB.AutoMigrate(m); err != nil {
				// GORM v1.25 的 MigrateColumnUnique 对"模型用 uniqueIndex tag（列带 UNI
				// 标记）但无 unique tag"的字段，每次启动都会尝试 DROP 一个它自己幻想的
				// uni_<表>_<列> 约束，而 mysql driver 一律用 DROP FOREIGN KEY 语法 →
				// 必然 Error 1091。该错误纯噪音、无法根治（约束本就不存在），
				// 精确过滤并计数；其余 migrate 错误照常告警。
				if strings.Contains(err.Error(), "Error 1091") {
					skippedUniqueNoise++
					continue
				}
				logx.Warnf("[migrate] Warning: auto-migrate %T failed: %v", m, err)
			}
		}
		logx.Infof("[migrate] AutoMigrate completed for %d registered models", len(models))
		if skippedUniqueNoise > 0 {
			logx.Infof("[migrate] Skipped %d harmless unique-constraint warnings (gorm MigrateColumnUnique quirk)", skippedUniqueNoise)
		}
	} // end if !skipMigrate

	// 第二步：AutoMigrate 不为已有表补索引，手动确保全表索引完整
	// P6 修复：ensureAllIndexes / ensureMissingColumns / seedDefaults 也尊重 CITEBASE_SKIP_MIGRATE，
	// 避免每次启动都跑 100+ 次 information_schema 查询拖慢启动 5-10s
	if skipMigrate {
		logx.Info("[migrate] Skipping ensureAllIndexes/ensureMissingColumns/seedDefaults (CITEBASE_SKIP_MIGRATE=true)")
	} else {
		ensureAllIndexes()

		// 第三步：自动补全核心表缺失的字段（兼容旧版本升级）
		ensureMissingColumns()

		ensureUtf8mb4()

		// 第四步：首次启动自动补充种子数据
		seedDefaults()

		// 第五步：把改名前留下的旧品牌值归一掉（存量库升级专用，见函数注释）
		normalizeLegacySiteNames()
	}

	// 关键列的启动核对。**放在 skipMigrate 分支之外**：跳过迁移的实例本来就不该 ALTER，
	// 但"我这台正在跑的功能其实缺一个列"这件事不能让 100+ 次 information_schema 查询的
	// 性能考虑一起静音掉 —— 一条 COUNT 查询换一个"永久静默降级"的可发现性，是划算的。
	verifyCriticalColumns(!skipMigrate)

	logx.Info("[migrate] Database initialized successfully")
}

// criticalColumns 是"缺了不会让启动失败、但会让某个功能永久静默不工作"的列。
// 每一项都必须同时出现在 ensureMissingColumns 的显式清单里（否则这里只报红不治病），
// 这条约束由 migrate_columns_test.go 钉住 —— 它读源码文本比对，不需要连数据库。
var criticalColumns = []struct{ table, col string }{
	// 偏好跟账号走（2026-10-16）：缺这列时前端表现为"换设备主题就丢"，
	// 而后端只在日志里留一行 auto-migrate 警告，没有任何界面提示。
	{"aw_user", "preferences"},
	// Wiki 页面置信度：缺列时治理/检索会静默降级（低置信度页不进审核队列）。
	{"aw_wiki_page", "confidence"},
	// Wiki 页面取代链：缺列时 supersession 静默丢失（被取代页面仍可被检索）。
	{"aw_wiki_page", "superseded_by"},
}

// verifyCriticalColumns 核对关键列真的存在。
//
//	migrateNow=true  → 缺列就直接补（复用显式清单里那份列定义，不另抄第二个定义），补完再核一次；
//	migrateNow=false → 只报错并给出可执行的下一步，**不动库**（多实例部署里非迁移实例的规矩）。
//
// 两种情况都打 Error 级：这条存在的理由就是"缺列不报错"，所以它必须是响的。
func verifyCriticalColumns(migrateNow bool) {
	if DB == nil {
		return
	}
	var dbName string
	DB.Raw("SELECT DATABASE()").Scan(&dbName)
	if dbName == "" {
		return
	}
	for _, c := range criticalColumns {
		var exists int64
		DB.Raw(
			"SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?",
			dbName, c.table, c.col,
		).Scan(&exists)
		if exists > 0 {
			continue
		}
		def := ""
		for _, f := range missingColumnFallbacks() {
			if f.table == c.table && f.col == c.col {
				def = f.def
				break
			}
		}
		if def == "" {
			logx.Errorf("[migrate] 关键列 `%s`.`%s` 不存在，而显式补列清单里没有它的定义 —— 去 ensureMissingColumns 的清单补一行，别只加模型字段", c.table, c.col)
			continue
		}
		if !migrateNow {
			logx.Errorf("[migrate] 关键列 `%s`.`%s` 不存在，而本实例按 CITEBASE_SKIP_MIGRATE=true 跳过了迁移：%s 这条功能会静默降级（不是报错）。请在负责迁移的那个实例上重启，或去掉该环境变量", c.table, c.col, whatNeeds(c.table, c.col))
			continue
		}
		sql := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` %s", c.table, c.col, def)
		if err := DB.Exec(sql).Error; err != nil {
			logx.Errorf("[migrate] 补关键列 `%s`.`%s` 失败（多半是 DB 账号没有 ALTER 权限）：%v —— 需要人工执行：%s", c.table, c.col, err, sql)
			continue
		}
		var again int64
		DB.Raw(
			"SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?",
			dbName, c.table, c.col,
		).Scan(&again)
		if again == 0 {
			logx.Errorf("[migrate] ALTER 之后关键列 `%s`.`%s` 仍然查不到：这条链已经不能靠启动自愈，请人工执行 %s", c.table, c.col, sql)
			continue
		}
		logx.Infof("[migrate] 关键列 `%s`.`%s` 启动时补齐（此前缺失，功能一直静默降级）", c.table, c.col)
	}
}

// whatNeeds 给关键列缺位时写清"坏掉的是哪件事"，让日志本身能指到用户可复现的症状。
func whatNeeds(table, col string) string {
	if table == "aw_user" && col == "preferences" {
		return "个性化偏好无法写库（保存偏好会 500，前端按设计不弹提示，只记 pending 下次重试）——"
	}
	return "该列缺失——"
}

// normalizeLegacySiteNames 把 aw_config 里仍写着**旧产品名**的站点名行换成当前默认名。
//
// 为什么需要：2026-10-07 改名换了代码与文档，但没动任何存量库里的 aw_config 行 ——
// 那时 main_title / system_name 没人读，所以看不见。批次十五把站点名接成真的之后，
// 2026-10-12 实跑 GET /api/v1/site 直接回了 "AIWiki"：老库里上一个产品的默认值，
// 被当成"管理员起的名字"印到了登录页与标签页标题上。
//
// 判据只认"整串等于旧品牌名"（MySQL 的比较本身大小写不敏感，另外几种写法在
// brand.LegacySiteNames 里逐个列）。带别的内容的值（"XX 部门知识库"、"我的 AIWiki 笔记"）
// 是人起的名字，一律不动 —— 这不是改写用户数据，是清掉一个已被替换的产品留下的默认值。
func normalizeLegacySiteNames() {
	if DB == nil {
		return
	}
	names := brand.LegacySiteNames()
	where, args, sets := legacySiteNameUpdate(names, int(time.Now().Unix()))

	result := DB.Table("aw_config").Where(where, args...).Updates(sets)
	if result.Error != nil {
		// 归一失败不拦启动，但必须喊出来：悄悄失败就等于改名没做完。
		logx.Warnf("[migrate] 旧品牌站点名归一失败（不影响启动）: %v", result.Error)
		return
	}
	if result.RowsAffected > 0 {
		logx.Warnf("[migrate] aw_config 里 %d 行仍写着改名前的产品名，已归一为 %q；"+
			"如果这是管理员自己起的名字，请到「系统设置 → 基础」改回来", result.RowsAffected, brand.DefaultName)
	}
}

// legacySiteNameUpdate 组装那条归一 UPDATE 的三部分：WHERE 片段、按顺序对应的参数、SET 的列。
//
// 为什么单独提成纯函数：上一版把 SET 要用的值也塞进了同一个参数切片，MySQL 直接报
// "sql: expected 11 arguments, got 13" —— 于是归一**静默没发生**，界面继续显示 AIWiki。
// 这种"占位符与参数不同数"的错编译期抓不到，而它失败的方式是"什么都没发生"，
// 只有连真库跑一次才看得见（这次就是实跑 /api/v1/site 抓出来的）。
// 提出来之后不连库也能钉住：数一下问号就行。
func legacySiteNameUpdate(names []string, now int) (string, []any, map[string]any) {
	args := make([]any, 0, len(names)+2)
	args = append(args, brand.ConfigKeySiteName, brand.ConfigKeySiteNameLegacy)
	placeholders := make([]string, 0, len(names))
	for _, name := range names {
		placeholders = append(placeholders, "?")
		args = append(args, name)
	}
	where := "`key` IN (?, ?) AND value IN (" + strings.Join(placeholders, ",") + ")"
	sets := map[string]any{
		"value":       brand.DefaultName,
		"update_time": now,
	}
	return where, args, sets
}

// InitCache initializes the application cache backend.
//
// Behavior:
//   - If redis.enabled is true in config and the connection succeeds, uses RedisCache.
//   - If Redis connection fails, logs a warning and falls back to MemoryCache.
//   - If redis is not enabled, uses MemoryCache directly.
//
// The result is stored in the package-level AppCache variable.
func InitCache() {
	cfg := config.AppConfig.Redis

	if !cfg.Enabled {
		AppCache = cache.NewMemoryCache()
		return
	}

	// Apply environment variable override for password (mirrors DB DSN pattern).
	if pwd := envname.Getenv("CITEBASE_REDIS_PASSWORD", "AIWIKI_REDIS_PASSWORD"); pwd != "" {
		cfg.Password = pwd
	}

	rc, err := cache.NewRedisCache(cache.RedisConfig{
		Addr:         cfg.Addr,
		Password:     cfg.Password,
		DB:           cfg.DB,
		PoolSize:     cfg.PoolSize,
		MinIdleConns: cfg.MinIdleConns,
	})
	if err != nil {
		logx.Warnf("[Cache] Redis connection failed, falling back to memory cache: %v", err)
		AppCache = cache.NewMemoryCache()
		return
	}

	AppCache = rc
	logx.Info("[Cache] Redis cache initialized successfully")
}

// dropLegacyIndexes 清理旧版本残留的冗余索引
// 历史问题：之前 allIndexes 中定义了大量与 GORM 标签重复的单列唯一索引（uni_aw_* 前缀），
// 这些索引与 GORM AutoMigrate 创建的索引(uk_*前缀)重复，且会被 GORM 误识别为外键约束，
// 导致每次启动报 "Can't DROP FOREIGN KEY" 警告。
func dropLegacyIndexes() {
	legacyIndexes := []struct {
		table string
		index string
	}{
		{"aw_user", "uni_aw_user_username"},
		{"aw_user", "uni_aw_user_mobile"},
		{"aw_user", "uni_aw_user_email"},
		{"aw_config", "uni_aw_config_key"},
		{"aw_plugin", "uni_aw_plugin_plugin_key"},
		{"aw_ai_task", "uni_aw_ai_task_task_id"},
		{"aw_ai_doc_mapping", "uni_aw_ai_doc_mapping_ai_doc_id"},
		{"aw_ai_table_cache", "uni_aw_ai_table_cache_table_name"},
		{"aw_ai_sql_security_policy", "uni_aw_ai_sql_security_policy_space_id"},
		{"aw_schema_space", "uni_aw_schema_space_space_id"},
		{"aw_ai_memory_summary", "uni_aw_ai_memory_summary_conv_id"},
		{"aw_text2sql_conversation_summary", "uni_aw_text2sql_conversation_summary_conversation_id"},
		{"aw_text2sql_turn_embedding", "uni_aw_text2sql_turn_embedding_turn_id"},
		{"aw_text2sql_session", "uni_aw_text2sql_session_session_key"},
		{"aw_ai_sql_trace", "uni_aw_ai_sql_trace_trace_id"},
		{"aw_ai_ab_experiment", "uni_aw_ai_ab_experiment_experiment_id"},
		{"aw_document_collab_state", "uni_aw_document_collab_state_document_id"},
		{"aw_git_sync_config", "uni_aw_git_sync_config_space_id"},
		{"aw_document_approval", "uni_aw_document_approval_document_id"},
		{"aw_document_template", "uni_aw_document_template_name"},
		{"aw_tag_hierarchy", "uni_aw_tag_hierarchy_child_tag"},
		{"aw_api_key", "uni_aw_api_key_key_hash"},
		{"aw_oidc_provider", "uni_aw_oidc_provider_name"},
		// 旧 table.sql 给 aw_ai_table_cache 建了全局唯一 uk_table_name，而该表是多空间设计：
		// 不同空间的同名表 upsert 时会互相覆盖或直接 1062。模型要求的是 (space_id, table_name)。
		{"aw_ai_table_cache", "uk_table_name"},
		// 审批历史上限：uk_approval_doc 让同一文档第二次送审（上次已通过/驳回）直接 1062，
		// 现改为普通索引 idx_approval_doc，保留一文档多条审批记录。
		{"aw_document_approval", "uk_approval_doc"},

		// 下一组（2026-11-01 从生产库/开发库逐条清点）：模型里裸 `index` tag 的 GORM 默认名
		// （idx_<表>_<列>），与设计 DDL 的显式名（idx_<列>，genddl 规则）分叉。裸 tag 已全部
		// 补成显式名（index:idx_<列> / uniqueIndex:uk_<列>），此后 AutoMigrate 只建设计名，
		// 本清单把存量库里按旧默认名建出的索引一次性清掉，删除后无功能影响。
		{"aw_ai_analysis_result", "idx_aw_ai_analysis_result_space_id"},
		{"aw_ai_analysis_result", "idx_aw_ai_analysis_result_doc_id"},
		{"aw_ai_analysis_result", "idx_aw_ai_analysis_result_category"},
		{"aw_ai_audit_log", "idx_aw_ai_audit_log_user_id"},
		{"aw_ai_audit_log", "idx_aw_ai_audit_log_action"},
		{"aw_ai_business_rule", "idx_aw_ai_business_rule_deleted_at"},
		{"aw_ai_business_rule", "idx_aw_ai_business_rule_table_nam"},
		{"aw_ai_business_rule", "idx_aw_ai_business_rule_schema_space_id"},
		{"aw_ai_conversation", "idx_aw_ai_conversation_user_id"},
		{"aw_ai_conversation", "idx_aw_ai_conversation_type"},
		{"aw_ai_data_sensitivity_rules", "idx_ds_space_table_field"},
		{"aw_ai_datasource", "idx_aw_ai_datasource_deleted_at"},
		{"aw_ai_datasource", "idx_aw_ai_datasource_space_id"},
		{"aw_ai_doc_cluster", "idx_aw_ai_doc_cluster_cluster_id"},
		{"aw_ai_doc_cluster", "idx_aw_ai_doc_cluster_doc_id"},
		{"aw_ai_doc_cluster", "idx_aw_ai_doc_cluster_space_id"},
		{"aw_ai_extracted_entities", "idx_aw_ai_extracted_entities_doc_id"},
		{"aw_ai_feedback", "idx_aw_ai_feedback_user_id"},
		{"aw_ai_feedback", "idx_aw_ai_feedback_conversation_id"},
		{"aw_ai_memory", "idx_aw_ai_memory_user_id"},
		{"aw_ai_memory", "idx_aw_ai_memory_memory_type"},
		{"aw_ai_prompt_template", "idx_aw_ai_prompt_template_deleted_at"},
		{"aw_ai_prompt_version", "idx_aw_ai_prompt_version_deleted_at"},
		{"aw_ai_prompt_version", "idx_aw_ai_prompt_version_space_id"},
		{"aw_ai_semantic_dimension", "idx_aw_ai_semantic_dimension_deleted_at"},
		{"aw_ai_semantic_dimension", "idx_aw_ai_semantic_dimension_space_id"},
		{"aw_ai_semantic_metric", "idx_aw_ai_semantic_metric_deleted_at"},
		{"aw_ai_semantic_metric", "idx_aw_ai_semantic_metric_space_id"},
		{"aw_ai_sql_history", "idx_aw_ai_sql_history_deleted_at"},
		{"aw_ai_sql_history", "idx_aw_ai_sql_history_space_id"},
		{"aw_ai_sql_history", "idx_aw_ai_sql_history_user_id"},
		{"aw_ai_sql_history", "idx_aw_ai_sql_history_trace_id"},
		{"aw_ai_table_cache", "idx_aw_ai_table_cache_deleted_at"},
		{"aw_ai_table_relation", "idx_aw_ai_table_relation_deleted_at"},
		{"aw_api_key", "idx_apikey_owner"},
		{"aw_collection", "idx_aw_collection_user_id"},
		{"aw_comment", "idx_aw_comment_document_id"},
		{"aw_comment", "idx_aw_comment_parent_id"},
		{"aw_comment", "idx_aw_comment_creator_id"},
		{"aw_document", "idx_aw_document_edit_user_id"},
		{"aw_document", "idx_aw_document_status"},
		{"aw_document", "idx_aw_document_create_user_id"},
		{"aw_document_approval", "idx_aw_document_approval_submitter"},
		{"aw_document_approval", "idx_aw_document_approval_approver"},
		{"aw_document_collab_session", "idx_aw_document_collab_session_document_id"},
		{"aw_document_collab_session", "idx_aw_document_collab_session_user_id"},
		{"aw_document_edit_lock", "idx_aw_document_edit_lock_user_id"},
		{"aw_document_permission", "idx_aw_document_permission_space_id"},
		{"aw_document_tag", "idx_aw_document_tag_document_id"},
		{"aw_document_translation", "idx_aw_document_translation_translated_doc_id"},
		{"aw_document_translation", "idx_aw_document_translation_document_id"},
		{"aw_document_trash", "idx_aw_document_trash_deleted_at"},
		{"aw_document_trash", "idx_aw_document_trash_document_id"},
		{"aw_document_trash", "idx_aw_document_trash_space_id"},
		{"aw_document_version", "idx_aw_document_version_document_id"},
		{"aw_document_view_log", "idx_aw_document_view_log_document_id"},
		{"aw_document_view_log", "idx_aw_document_view_log_space_id"},
		{"aw_document_view_log", "idx_aw_document_view_log_user_id"},
		{"aw_document_view_log", "idx_aw_document_view_log_viewed_at"},
		{"aw_eval_result", "idx_aw_eval_result_dataset_id"},
		{"aw_follow", "uk_user_object"},
		{"aw_follow", "idx_aw_follow_user_id"},
		{"aw_follow", "idx_aw_follow_object_id"},
		{"aw_git_sync_log", "idx_aw_git_sync_log_config_id"},
		{"aw_git_sync_log", "idx_aw_git_sync_log_space_id"},
		{"aw_knowledge_entity", "idx_aw_knowledge_entity_doc_id"},
		{"aw_knowledge_entity", "idx_aw_knowledge_entity_space_id"},
		{"aw_knowledge_relation", "idx_aw_knowledge_relation_source_entity_id"},
		{"aw_knowledge_relation", "idx_aw_knowledge_relation_target_entity_id"},
		{"aw_log", "idx_aw_log_user_id"},
		{"aw_log", "idx_aw_log_create_time"},
		{"aw_log_document", "idx_aw_log_document_document_id"},
		{"aw_log_document", "idx_aw_log_document_space_id"},
		{"aw_log_document", "idx_aw_log_document_user_id"},
		{"aw_notification", "idx_aw_notification_user_id"},
		{"aw_space_user", "uk_user_space"},
		{"aw_text2sql_conversation_turn", "idx_aw_text2sql_conversation_turn_deleted_at"},
		{"aw_vector_sync_task", "idx_aw_vector_sync_task_doc_id"},
		{"aw_vector_sync_task", "idx_aw_vector_sync_task_status"},
		{"aw_wiki_link", "idx_aw_wiki_link_to_slug"},
		{"aw_wiki_link", "idx_aw_wiki_link_space_id"},
		{"aw_wiki_lint_issue", "idx_aw_wiki_lint_issue_space_id"},
		{"aw_wiki_page", "idx_aw_wiki_page_space_id"},
		{"aw_wiki_pin", "idx_aw_wiki_pin_space_id"},
		{"aw_wiki_pin", "idx_aw_wiki_pin_page_slug"},
		{"aw_wiki_review", "idx_aw_wiki_review_space_id"},
		{"aw_wiki_review", "idx_aw_wiki_review_page_slug"},
		// 唯一的裸 uniqueIndex（doc_id）：GORM 默认名带 idx_ 前缀，设计名是 uk_doc_id。
		// 2026-11-01 与开发库实存索引逐条核对后补录。
		{"aw_vector_sync_state", "idx_aw_vector_sync_state_doc_id"},
	}

	dropped := 0
	for _, idx := range legacyIndexes {
		var count int
		checkSQL := fmt.Sprintf("SELECT COUNT(*) FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = DATABASE() AND TABLE_NAME = '%s' AND INDEX_NAME = '%s'", idx.table, idx.index)
		if err := DB.Raw(checkSQL).Scan(&count).Error; err != nil {
			logx.Infof("[migrate] Check index %s.%s error: %v", idx.table, idx.index, err)
			continue
		}
		if count == 0 {
			continue
		}

		dropSQL := fmt.Sprintf("DROP INDEX `%s` ON `%s`", idx.index, idx.table)
		if err := DB.Exec(dropSQL).Error; err != nil {
			logx.Infof("[migrate] Drop legacy index %s.%s warning: %v", idx.table, idx.index, err)
		} else {
			dropped++
			logx.Infof("[migrate] Dropped legacy index: %s.%s", idx.table, idx.index)
		}
	}
	if dropped > 0 {
		logx.Infof("[migrate] Cleaned up %d legacy duplicate indexes", dropped)
	}
}

// ensureAllIndexes 确保复合索引和前缀索引存在
// 单列索引/唯一索引通过 GORM 标签(uniqueIndex/index)由 AutoMigrate 自动管理
// 使用 information_schema.STATISTICS 查询现有索引，仅创建缺失的索引
// 避免每次启动都报 "Duplicate key name" 错误
func ensureAllIndexes() {
	// 查询当前数据库名
	var dbName string
	DB.Raw("SELECT DATABASE()").Scan(&dbName)
	if dbName == "" {
		logx.Info("[migrate] Cannot determine database name, skipping index check")
		return
	}

	// 批量查询已存在的索引
	type existingIndex struct {
		TableName string `gorm:"column:TABLE_NAME"`
		IndexName string `gorm:"column:INDEX_NAME"`
	}
	var existing []existingIndex
	DB.Raw("SELECT DISTINCT TABLE_NAME, INDEX_NAME FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = ?", dbName).Scan(&existing)

	existingSet := make(map[string]bool)
	for _, idx := range existing {
		key := idx.TableName + "." + idx.IndexName
		existingSet[key] = true
	}

	// 创建缺失的索引
	created := 0
	skipped := 0
	dataIssues := 0
	for _, idx := range allIndexes {
		key := idx.table + "." + idx.name
		if existingSet[key] {
			skipped++
			continue
		}

		// 对 UNIQUE 索引：先检查数据是否有重复，避免 Error 1062 和 GORM Warn 日志
		if idx.unique {
			colsClean := strings.ReplaceAll(idx.cols, "`", "")
			colList := strings.Split(colsClean, ",")
			groupByCols := strings.Join(colList, ", ")
			var dupCount int64
			DB.Raw(fmt.Sprintf("SELECT IFNULL(SUM(cnt), 0) FROM (SELECT COUNT(*) AS cnt FROM `%s` GROUP BY %s HAVING COUNT(*) > 1) AS dups",
				idx.table, groupByCols)).Scan(&dupCount)
			if dupCount > 0 {
				dataIssues++
				fallbackName := idx.name + "_nonunique"
				fallbackKey := idx.table + "." + fallbackName
				if existingSet[fallbackKey] {
					logx.Warnf("[migrate] ⚠️  Table `%s` has %d duplicate rows for UNIQUE index `%s` (fallback `%s` already exists)",
						idx.table, dupCount, idx.name, fallbackName)
					skipped++
				} else {
					logx.Warnf("[migrate] ⚠️  Table `%s` has %d duplicate rows for UNIQUE index `%s` on (%s), creating non-unique index instead",
						idx.table, dupCount, idx.name, colsClean)
					logx.Infof("[migrate]    To fix: SELECT %s, COUNT(*) FROM `%s` GROUP BY %s HAVING COUNT(*) > 1",
						colsClean, idx.table, colsClean)
					fallbackSQL := fmt.Sprintf("CREATE INDEX `%s` ON `%s` (%s)", fallbackName, idx.table, idx.cols)
					if fbErr := DB.Exec(fallbackSQL).Error; fbErr == nil {
						logx.Infof("[migrate] Created non-unique index `%s` on `%s`", fallbackName, idx.table)
						created++
					}
				}
				continue
			}
		}

		uniqueStr := ""
		if idx.unique {
			uniqueStr = "UNIQUE "
		}
		createSQL := fmt.Sprintf("CREATE %sINDEX `%s` ON `%s` (%s)", uniqueStr, idx.name, idx.table, idx.cols)
		if err := DB.Exec(createSQL).Error; err != nil {
			errMsg := err.Error()
			// Error 1061: 索引已存在（并发创建，information_schema 查询时序差导致）
			if strings.Contains(errMsg, "Error 1061") {
				skipped++
				continue
			}
			logx.Infof("[migrate] Index `%s` on `%s`: %v (safe to ignore)", idx.name, idx.table, err)
		} else {
			logx.Infof("[migrate] Created index `%s` on `%s`", idx.name, idx.table)
			created++
		}
	}
	if created > 0 || skipped > 0 || dataIssues > 0 {
		logx.Infof("[migrate] Index check complete: %d created, %d existing, %d data issues", created, skipped, dataIssues)
	}
}

// ensureMissingColumns 自动补全核心表缺失的字段（兼容旧版本数据库升级）
// missingColumn 是显式补列清单的一项。
type missingColumn struct {
	table string
	col   string
	def   string
}

// missingColumnFallbacks 是「不依赖 GORM 列差异比较」的显式补列清单：
// 每一项都是 (表, 列, 列定义)。它存在的理由是 AutoMigrate 那条链有静默不补的路径
// （见 verifyCriticalColumns 的注释）；关键列还必须同时进 criticalColumns 做启动核对，
// 这条双向约束由 migrate_columns_test.go 钉住。
// 清单从函数里抽出来的第二个理由：不抽出来，单测就只能靠读源码文本猜，那是第二份真相。
func missingColumnFallbacks() []missingColumn {
	return []missingColumn{
		// ========== 用户表扩展字段 ==========
		// preferences 必须在**这张显式清单**里，不能只挂 GORM AutoMigrate：
		// 那条链有四种"静默不补"的路径（CITEBASE_SKIP_MIGRATE=true、跑的是加字段之前构建的
		// 二进制/镜像、该模型整体 migrate 报错只 Warn、DB 账号没有 ALTER 权限），
		// 而它缺列的症状不是启动失败，是**换设备主题永久同步不上**（前端把 PUT 失败吞成 pending，
		// 设计上就是不给 toast）。所以既进显式补列清单，也进 criticalColumns 启动核对。
		{"aw_user", "preferences", "text COMMENT '个性化偏好(校验后的 JSON 文本)'"},
		{"aw_user", "given_name", "varchar(50) DEFAULT NULL COMMENT '显示名'"},
		{"aw_user", "phone", "varchar(13) DEFAULT NULL COMMENT '办公电话'"},
		{"aw_user", "department", "varchar(50) DEFAULT NULL COMMENT '部门'"},
		{"aw_user", "position", "varchar(50) DEFAULT NULL COMMENT '职位'"},
		{"aw_user", "location", "varchar(50) DEFAULT NULL COMMENT '办公地点'"},
		{"aw_user", "im", "varchar(50) DEFAULT NULL COMMENT '即时通讯账号'"},
		{"aw_user", "last_ip", "varchar(15) DEFAULT NULL COMMENT '最后登录IP'"},
		{"aw_user", "last_time", "int DEFAULT NULL COMMENT '最后登录时间'"},

		// ========== 角色表扩展字段 ==========
		{"aw_role", "type", "int DEFAULT 0 COMMENT '角色类型: 0自定义 1系统内置'"},
		{"aw_role", "description", "varchar(255) DEFAULT NULL COMMENT '角色描述'"},
		{"aw_role", "list_order", "int DEFAULT 0 COMMENT '排序'"},

		// ========== 空间表扩展字段 ==========
		{"aw_space", "tags", "varchar(255) DEFAULT NULL COMMENT '标签'"},
		{"aw_space", "visit_level", "varchar(20) DEFAULT NULL COMMENT '访问级别'"},
		{"aw_space", "is_share", "int DEFAULT 0 COMMENT '是否可分享'"},
		{"aw_space", "is_export", "int DEFAULT 0 COMMENT '是否可导出'"},
		{"aw_space", "share_password", "varchar(100) DEFAULT '' COMMENT '分享密码(bcrypt hash)'"},
		{"aw_space", "edit_user_id", "int DEFAULT NULL COMMENT '最后编辑用户ID'"},
		{"aw_space", "sequence", "int NOT NULL DEFAULT 0 COMMENT '排序序号'"},

		// ========== 附件表扩展字段 ==========
		{"aw_attachment", "media_type", "varchar(32) DEFAULT '' COMMENT 'MIME类型'"},
		{"aw_attachment", "duration", "decimal(10,2) DEFAULT 0 COMMENT '音视频时长(秒)'"},

		// ========== Text-to-SQL 相关表 ==========
		{"aw_ai_table_cache", "space_id", "bigint unsigned DEFAULT NULL COMMENT '所属空间ID'"},
		{"aw_ai_table_cache", "doc_id", "bigint unsigned DEFAULT NULL COMMENT '所属文档ID'"},
		{"aw_ai_table_cache", "schema_space_id", "bigint unsigned DEFAULT NULL COMMENT 'Schema空间ID'"},
		{"aw_ai_table_cache", "constraints_json", "json COMMENT '约束信息(索引/外键/唯一键)'"},
		{"aw_ai_table_cache", "db_type", "varchar(50) DEFAULT '' COMMENT '数据库类型'"},
		{"aw_ai_table_cache", "row_count_estimate", "bigint DEFAULT 0 COMMENT '行数估算'"},
		{"aw_ai_table_cache", "version", "int DEFAULT 1 COMMENT '版本号'"},
		{"aw_ai_table_relation", "space_id", "bigint unsigned DEFAULT NULL COMMENT '所属空间ID'"},
		{"aw_ai_business_rule", "schema_space_id", "bigint unsigned NOT NULL DEFAULT 0 COMMENT 'Schema空间ID'"},
		{"aw_ai_prompt_template", "type", "varchar(20) NOT NULL DEFAULT 'sql' COMMENT '模板类型'"},
		{"aw_ai_sql_history", "explanation_json", "longtext COMMENT '查询解释JSON'"},
		{"aw_ai_sql_history", "trace_id", "varchar(64) DEFAULT '' COMMENT '审计traceID'"},
		{"aw_ai_sql_history", "generator_tier", "varchar(20) DEFAULT '' COMMENT '生成器层级'"},
		{"aw_ai_sql_history", "datasource_id", "bigint unsigned DEFAULT 0 COMMENT '数据源ID'"},
		{"aw_ai_sql_history", "duration_ms", "int DEFAULT 0 COMMENT '执行耗时(毫秒)'"},
		{"aw_ai_sql_history", "status", "varchar(20) DEFAULT 'success' COMMENT '执行状态'"},
		{"aw_ai_task", "checkpoint", "text COMMENT '断点续传JSON'"},
		{"aw_vector_sync_task", "retry_count", "tinyint DEFAULT 0 COMMENT '重试次数'"},
		{"aw_vector_sync_task", "last_error", "varchar(500) DEFAULT '' COMMENT '最后错误信息'"},

		// ========== #7 草稿/发布分离 ==========
		{"aw_document", "status", "varchar(20) DEFAULT 'published' COMMENT '文档状态: published/draft'"},
		{"aw_document", "published_at", "int DEFAULT 0 COMMENT '发布时间(Unix时间戳)'"},

		// ========== Wiki 置信度 ==========
		{"aw_wiki_page", "confidence", "decimal(4,2) NOT NULL DEFAULT 0.50 COMMENT '提取置信度 0-1'"},

		// ========== Raw 层不可变标记 ==========
		{"aw_document", "is_immutable", "tinyint NOT NULL DEFAULT 0 COMMENT 'raw层不可变标记: 1=内容变更触发wiki重编译'"},

		// ========== Wiki 取代链 ==========
		{"aw_wiki_page", "superseded_by", "varchar(190) NOT NULL DEFAULT '' COMMENT '被哪个新页面取代(slug)，空=当前有效'"},
	}
}

// GORM AutoMigrate 会自动补全新表字段，但部分从 v1 升级或早期版本的库可能缺少关键字段
func ensureMissingColumns() {
	if DB == nil {
		return
	}

	// 获取当前数据库名
	var dbName string
	DB.Raw("SELECT DATABASE()").Scan(&dbName)
	if dbName == "" {
		return
	}

	// 定义需要检查的字段: 表名 -> [(字段名, 字段定义)]
	columnsToCheck := missingColumnFallbacks()

	added := 0
	for _, c := range columnsToCheck {
		var colExists int64
		DB.Raw(
			"SELECT COUNT(*) FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND COLUMN_NAME = ?",
			dbName, c.table, c.col,
		).Scan(&colExists)

		if colExists == 0 {
			sql := fmt.Sprintf("ALTER TABLE `%s` ADD COLUMN `%s` %s", c.table, c.col, c.def)
			if err := DB.Exec(sql).Error; err != nil {
				logx.Errorf("[migrate] Failed to add column `%s` to `%s`: %v", c.col, c.table, err)
			} else {
				logx.Infof("[migrate] Added column `%s` to `%s`", c.col, c.table)
				added++
			}
		}
	}

	if added > 0 {
		logx.Infof("[migrate] Missing columns: %d added", added)
	}
}

// ensureUtf8mb4 ensures critical text tables use utf8mb4 charset to support 4-byte UTF-8 characters (emoji, etc.)
func ensureUtf8mb4() {
	if DB == nil {
		return
	}
	tables := []string{
		"aw_ai_audit_log",
		"aw_ai_analysis_result",
		"aw_document",
		"aw_ai_memory",
	}
	for _, table := range tables {
		var charset string
		DB.Raw(
			"SELECT CCSA.CHARACTER_SET_NAME FROM information_schema.TABLES T JOIN information_schema.COLLATION_CHARACTER_SET_APPLICABILITY CCSA ON T.TABLE_COLLATION = CCSA.COLLATION_NAME WHERE T.TABLE_SCHEMA = DATABASE() AND T.TABLE_NAME = ?",
			table,
		).Scan(&charset)
		if charset != "utf8mb4" {
			if err := DB.Exec(fmt.Sprintf("ALTER TABLE `%s` CONVERT TO CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci", table)).Error; err != nil {
				logx.Errorf("[migrate] Failed to convert table `%s` to utf8mb4: %v", table, err)
			} else {
				logx.Infof("[migrate] Converted table `%s` from %s to utf8mb4", table, charset)
			}
		}
	}
}

// DefaultConfigSeeds 是「系统配置种子」的唯一清单（不含时间戳，由调用方按写入时刻盖）。
//
// 为什么要把它从 seedDefaults 里提成导出函数：这张表原先在 db.go 里是一份局部切片，
// 安装向导 internal/api/auth/install.go 又另抄了一份，databases/data.sql 是第三份 ——
// 三份从值、标签到排序号各自漂移过（main_title 一份写 "Citebase"、另一份写 "Citebase 知识库"，
// doc_search_timer 一份 type=number、另一份 type=text），而"走哪条路初始化数据库"
// 决定了管理员在后台看到哪一份，没有任何一台工装会发现。
// 现在只准这里写一次，安装向导引用它，data.sql 由
// tools/repocheck/brand_source_test.go 逐行比对拦住再抄第二份。
func DefaultConfigSeeds() []adminmodel.Config {
	return []adminmodel.Config{
		{Key: "main_title", Value: brand.DefaultName, Label: "主页标题", GroupKey: "basic", Type: "text", Sort: 4},
		{Key: "main_description", Value: "", Label: "主页描述", GroupKey: "basic", Type: "textarea", Sort: 5},
		{Key: "system_name", Value: brand.DefaultName, Label: "系统名称", GroupKey: "basic", Type: "text", Sort: 6},
		{Key: "system_version", Value: version.Version, Label: "系统版本", GroupKey: "system", Type: "text", Sort: 10},
		{Key: "fulltext_search_open", Value: "1", Label: "开启全文搜索", GroupKey: "search", Type: "switch", Sort: 1},
		{Key: "auto_follow_doc_open", Value: "0", Label: "自动关注文档", GroupKey: "basic", Type: "switch", Sort: 1},
		{Key: "auto_follow_doc", Value: "", Label: "自动关注文档配置", GroupKey: "basic", Type: "text", Sort: 7},
		{Key: "send_email_open", Value: "0", Label: "发送邮件通知", GroupKey: "basic", Type: "switch", Sort: 2},
		{Key: "sso_open", Value: "0", Label: "统一登录(SSO)", GroupKey: "basic", Type: "switch", Sort: 3},
		{Key: "wiki.multiagent.enabled", Value: "0", Label: "多智能体深度研究", GroupKey: "wiki", Type: "switch", Sort: 1},
		// AI 配置默认值已移除：系统设置里那份从未被消费（运行时真源是 configs/config.yaml，
		// 管理入口是 AI 设置页 GET/POST /api/v1/ai/config）。存量库里的旧行由
		// seedDefaults 的 deprecatedConfigKeys 清理。
	}
}

// deprecatedConfigKeys 已废弃的系统配置键：AI 分组 8 键是同一能力的第二份清单
// （真源在 configs/config.yaml + AI 设置页），doc_search_timer 语义无从考证且无消费点。
// 启动时从存量库删除，系统设置页不再显示这些改了也不生效的摆设。
var deprecatedConfigKeys = []string{
	"ai_primary_provider", "ai_primary_model", "ai_fast_provider", "ai_fast_model",
	"ai_analyzer_batch_size", "ai_analyzer_schedule", "ai_space_name", "ai_space_auto_create",
	"doc_search_timer",
}

// seedDefaults 首次启动自动补充基础种子数据
func seedDefaults() {
	now := int(time.Now().Unix())

	// 1. 系统角色（仅在首次创建时插入）
	roles := []struct {
		id   int
		name string
	}{
		{1, "超级管理员"},
		{2, "管理员"},
		{3, "普通用户"},
	}
	for _, r := range roles {
		var count int64
		DB.Model(&usermodel.Role{}).Where("role_id = ?", r.id).Count(&count)
		if count == 0 {
			DB.Create(&usermodel.Role{
				ID:         r.id,
				Name:       r.name,
				Type:       1, // 系统内置
				IsDelete:   0,
				CreateTime: now,
			})
			logx.Infof("[migrate] Created default role: %s (id=%d)", r.name, r.id)
		}
	}

	// 2. 默认超级管理员（无管理员时创建默认账号，随机生成初始密码并打印一次）
	// P0-8 修复：原默认密码 "123456" 弱密码风险，改为随机生成
	var adminCount int64
	DB.Model(&usermodel.User{}).Where("role_id = ?", 1).Count(&adminCount)
	if adminCount == 0 {
		defaultPwd := generateRandomPassword(16)
		hashedPwd, err := bcrypt.GenerateFromPassword([]byte(defaultPwd), bcrypt.DefaultCost)
		if err == nil {
			DB.Create(&usermodel.User{
				Username:    "admin",
				Password:    string(hashedPwd),
				GivenName:   "管理员",
				RoleID:      1,
				IsDelete:    0,
				IsForbidden: 0,
				CreateTime:  now,
			})
			logx.Info("[migrate] ============================================")
			logx.Info("[migrate] Created default admin user: admin")
			logx.Infof("[migrate] Initial password (randomly generated): %s", defaultPwd)
			logx.Warnf("[migrate] IMPORTANT: Please change this password immediately!")
			logx.Info("[migrate] ============================================")
		}
	}

	// 3. 系统配置默认值（清单见 DefaultConfigSeeds —— 安装向导与 data.sql 用的是同一份）
	defaultConfigs := DefaultConfigSeeds()
	for i := range defaultConfigs {
		defaultConfigs[i].CreateTime = now
		defaultConfigs[i].UpdateTime = now
	}
	seeded := 0
	for _, cfg := range defaultConfigs {
		var count int64
		DB.Model(&adminmodel.Config{}).Where("`key` = ?", cfg.Key).Count(&count)
		if count == 0 {
			DB.Create(&cfg)
			seeded++
		}
	}
	if seeded > 0 {
		logx.Infof("[migrate] Seeded %d default config entries", seeded)
	}

	// 3.1 修复存量库的裸键名标签：早期迁移把 label 写成了 key 本身，
	// 系统设置页就会把 auto_follow_doc 这类键名直接当字段名展示出来。
	// 只在标签为空或等于键名时覆盖，管理员自定义过的标签不动。
	repaired := int64(0)
	for _, cfg := range defaultConfigs {
		if cfg.Label == "" {
			continue
		}
		res := DB.Model(&adminmodel.Config{}).
			Where("`key` = ?", cfg.Key).
			Where("(label = '' OR label = `key`)").
			Update("label", cfg.Label)
		repaired += res.RowsAffected
	}
	if repaired > 0 {
		logx.Infof("[migrate] Repaired %d config labels", repaired)
	}

	// 3.2 清理已废弃的系统配置键（见 deprecatedConfigKeys 注释）
	removed := int64(0)
	for _, key := range deprecatedConfigKeys {
		res := DB.Where("`key` = ?", key).Delete(&adminmodel.Config{})
		removed += res.RowsAffected
	}
	if removed > 0 {
		logx.Infof("[migrate] Removed %d deprecated config entries", removed)
	}

	// 3.5 默认 Prompt 模板
	seedPromptTemplates()

	// 4. 系统权限（仅在首次创建时插入）
	seedPrivileges(now)

	// 5. 角色权限关系（仅在首次创建时插入）
	seedRolePrivileges(now)
}

// seedPromptTemplates 补充默认 Prompt 模板
func seedPromptTemplates() {
	var count int64
	DB.Table("aw_ai_prompt_template").Count(&count)
	if count > 0 {
		return
	}

	type promptSeed struct {
		Name         string
		Type         string
		QueryType    string
		SystemPrompt string
		UserTemplate string
		Description  string
		IsDefault    bool
	}

	seeds := []promptSeed{
		{
			Name:      "标准 Text-to-SQL",
			Type:      "sql",
			QueryType: "basic",
			SystemPrompt: `你是一个专业的 Text-to-SQL 转换助手。根据用户的自然语言问题和数据库 Schema 信息，生成准确的 SQL 查询。

规则：
1. 只生成 SELECT 查询，禁止 DML/DDL 操作
2. 使用标准 SQL 语法
3. 如果信息不足，给出最合理的假设
4. 返回 SQL 语句和简要解释`,
			UserTemplate: `数据库 Schema：
{{schema}}

用户问题：{{question}}

请生成 SQL 查询：`,
			Description: "通用 Text-to-SQL 模板，适用于大多数查询场景",
			IsDefault:   true,
		},
		{
			Name:      "复杂分析查询",
			Type:      "sql",
			QueryType: "analysis",
			SystemPrompt: `你是一个高级数据分析助手，擅长处理复杂的聚合、分组和多表关联查询。

规则：
1. 优先使用 JOIN 而非子查询
2. 合理使用 GROUP BY、HAVING、窗口函数
3. 对大数据量查询建议添加 LIMIT
4. 提供查询性能说明`,
			UserTemplate: `数据库 Schema：
{{schema}}

分析需求：{{question}}

请生成分析 SQL：`,
			Description: "适用于需要聚合、分组或多表关联的复杂分析查询",
			IsDefault:   false,
		},
		{
			Name:      "SQL 解释",
			Type:      "explain",
			QueryType: "explain",
			SystemPrompt: `你是一个 SQL 代码审查专家，负责解释 SQL 查询的逻辑和执行流程。

请用通俗易懂的语言解释：
1. 查询的整体目的
2. 每个子句的作用
3. 涉及的表和关联关系
4. 可能的性能影响`,
			UserTemplate: `SQL 语句：
{{sql}}

请解释这条 SQL 的含义：`,
			Description: "将 SQL 查询转换为自然语言解释",
			IsDefault:   false,
		},
		{
			Name:      "数据洞察",
			Type:      "insight",
			QueryType: "insight",
			SystemPrompt: `你是一个数据洞察专家，能够从查询结果中发现有价值的业务洞察。

请分析：
1. 数据的整体趋势
2. 异常值或离群点
3. 潜在的业务含义
4. 可行的改进建议`,
			UserTemplate: `查询结果：
{{result}}

原始问题：{{question}}

请给出数据洞察：`,
			Description: "从查询结果中提取业务洞察和建议",
			IsDefault:   false,
		},
	}

	for _, s := range seeds {
		tpl := map[string]interface{}{
			"name":          s.Name,
			"type":          s.Type,
			"query_type":    s.QueryType,
			"system_prompt": s.SystemPrompt,
			"user_template": s.UserTemplate,
			"description":   s.Description,
			"is_active":     true,
			"is_default":    s.IsDefault,
			"sort":          0,
		}
		DB.Table("aw_ai_prompt_template").Create(tpl)
	}
	logx.Infof("[migrate] Seeded %d default prompt templates", len(seeds))
}

// seedPrivileges 补充系统权限
func seedPrivileges(now int) {
	var count int64
	DB.Model(&usermodel.Privilege{}).Count(&count)
	if count > 0 {
		return
	}

	privileges := []usermodel.Privilege{
		// 个人中心
		{ID: 1, Name: "个人中心", ParentID: 0, Type: "menu", Controller: "", Action: "", Icon: "icon-user", Target: "", IsDisplay: 1, IsMenu: 1, Sequence: 1, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 2, Name: "个人资料", ParentID: 1, Type: "controller", Controller: "profile", Action: "info", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 11, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 3, Name: "修改资料", ParentID: 1, Type: "controller", Controller: "profile", Action: "edit", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 12, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 4, Name: "修改资料保存", ParentID: 1, Type: "controller", Controller: "profile", Action: "modify", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 13, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 5, Name: "关注用户列表", ParentID: 1, Type: "controller", Controller: "profile", Action: "followUser", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 14, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 6, Name: "关注文档列表", ParentID: 1, Type: "controller", Controller: "profile", Action: "followDoc", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 15, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 7, Name: "我的活动", ParentID: 1, Type: "controller", Controller: "profile", Action: "activity", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 16, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 8, Name: "修改密码", ParentID: 1, Type: "controller", Controller: "profile", Action: "password", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 17, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 9, Name: "修改密码保存", ParentID: 1, Type: "controller", Controller: "profile", Action: "savePass", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 18, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		// 用户管理
		{ID: 10, Name: "用户管理", ParentID: 0, Type: "menu", Controller: "", Action: "", Icon: "icon-users", Target: "", IsDisplay: 1, IsMenu: 1, Sequence: 2, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 11, Name: "添加用户", ParentID: 10, Type: "controller", Controller: "user", Action: "add", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 21, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 12, Name: "添加用户保存", ParentID: 10, Type: "controller", Controller: "user", Action: "save", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 22, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 13, Name: "用户列表", ParentID: 10, Type: "controller", Controller: "user", Action: "list", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 23, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 14, Name: "修改用户", ParentID: 10, Type: "controller", Controller: "user", Action: "edit", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 24, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 15, Name: "修改用户保存", ParentID: 10, Type: "controller", Controller: "user", Action: "modify", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 25, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 16, Name: "屏蔽用户", ParentID: 10, Type: "controller", Controller: "user", Action: "forbidden", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 26, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 17, Name: "恢复用户", ParentID: 10, Type: "controller", Controller: "user", Action: "recover", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 27, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 18, Name: "用户详情", ParentID: 10, Type: "controller", Controller: "user", Action: "info", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 28, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		// 角色管理
		{ID: 19, Name: "角色管理", ParentID: 0, Type: "menu", Controller: "", Action: "", Icon: "icon-role", Target: "", IsDisplay: 1, IsMenu: 1, Sequence: 3, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 20, Name: "添加角色", ParentID: 19, Type: "controller", Controller: "role", Action: "add", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 31, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 21, Name: "添加角色保存", ParentID: 19, Type: "controller", Controller: "role", Action: "save", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 32, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 22, Name: "角色列表", ParentID: 19, Type: "controller", Controller: "role", Action: "list", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 33, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 23, Name: "修改角色", ParentID: 19, Type: "controller", Controller: "role", Action: "edit", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 34, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 24, Name: "修改角色保存", ParentID: 19, Type: "controller", Controller: "role", Action: "modify", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 35, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 25, Name: "角色用户列表", ParentID: 19, Type: "controller", Controller: "role", Action: "user", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 36, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 26, Name: "角色权限", ParentID: 19, Type: "controller", Controller: "role", Action: "privilege", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 37, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 27, Name: "角色权限保存", ParentID: 19, Type: "controller", Controller: "role", Action: "grantPrivilege", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 38, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 28, Name: "删除角色", ParentID: 19, Type: "controller", Controller: "role", Action: "delete", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 39, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 29, Name: "重置用户角色", ParentID: 19, Type: "controller", Controller: "role", Action: "resetUser", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 310, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		// 权限管理
		{ID: 30, Name: "权限管理", ParentID: 0, Type: "menu", Controller: "", Action: "", Icon: "icon-lock", Target: "", IsDisplay: 1, IsMenu: 1, Sequence: 4, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 31, Name: "添加权限", ParentID: 30, Type: "controller", Controller: "privilege", Action: "add", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 41, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 32, Name: "添加权限保存", ParentID: 30, Type: "controller", Controller: "privilege", Action: "save", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 42, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 33, Name: "权限列表", ParentID: 30, Type: "controller", Controller: "privilege", Action: "list", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 43, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 34, Name: "修改权限", ParentID: 30, Type: "controller", Controller: "privilege", Action: "edit", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 44, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 35, Name: "修改权限保存", ParentID: 30, Type: "controller", Controller: "privilege", Action: "modify", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 45, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 36, Name: "删除权限", ParentID: 30, Type: "controller", Controller: "privilege", Action: "delete", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 46, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		// 空间管理
		{ID: 37, Name: "空间管理", ParentID: 0, Type: "menu", Controller: "", Action: "", Icon: "icon-space", Target: "", IsDisplay: 1, IsMenu: 1, Sequence: 5, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 38, Name: "添加空间", ParentID: 37, Type: "controller", Controller: "space", Action: "add", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 51, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 39, Name: "添加空间保存", ParentID: 37, Type: "controller", Controller: "space", Action: "save", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 52, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 40, Name: "空间列表", ParentID: 37, Type: "controller", Controller: "space", Action: "list", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 53, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 41, Name: "修改空间", ParentID: 37, Type: "controller", Controller: "space", Action: "edit", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 54, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 42, Name: "修改空间保存", ParentID: 37, Type: "controller", Controller: "space", Action: "modify", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 55, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 43, Name: "空间成员列表", ParentID: 37, Type: "controller", Controller: "space", Action: "member", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 56, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 44, Name: "添加空间成员", ParentID: 37, Type: "controller", Controller: "space_user", Action: "save", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 57, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 45, Name: "移除空间成员", ParentID: 37, Type: "controller", Controller: "space_user", Action: "remove", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 58, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 46, Name: "更新空间成员权限", ParentID: 37, Type: "controller", Controller: "space_user", Action: "modify", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 59, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 47, Name: "删除空间", ParentID: 37, Type: "controller", Controller: "space", Action: "delete", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 510, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 48, Name: "空间备份", ParentID: 37, Type: "controller", Controller: "space", Action: "download", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 512, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		// 日志管理
		{ID: 49, Name: "日志管理", ParentID: 0, Type: "menu", Controller: "", Action: "", Icon: "icon-log", Target: "", IsDisplay: 1, IsMenu: 1, Sequence: 6, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 50, Name: "系统日志", ParentID: 49, Type: "controller", Controller: "log", Action: "system", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 61, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 51, Name: "系统日志详情", ParentID: 49, Type: "controller", Controller: "log", Action: "info", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 62, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 52, Name: "文档日志", ParentID: 49, Type: "controller", Controller: "log", Action: "document", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 63, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		// 配置管理
		{ID: 53, Name: "配置管理", ParentID: 0, Type: "menu", Controller: "", Action: "", Icon: "icon-setting", Target: "", IsDisplay: 1, IsMenu: 1, Sequence: 7, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 54, Name: "全局配置", ParentID: 53, Type: "controller", Controller: "config", Action: "global", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 71, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 55, Name: "全局配置保存", ParentID: 53, Type: "controller", Controller: "config", Action: "modify", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 72, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 56, Name: "邮箱配置", ParentID: 53, Type: "controller", Controller: "email", Action: "list", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 73, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 57, Name: "添加邮件服务器", ParentID: 53, Type: "controller", Controller: "email", Action: "add", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 74, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 58, Name: "添加邮件服务器保存", ParentID: 53, Type: "controller", Controller: "email", Action: "save", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 75, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 59, Name: "修改邮件服务器", ParentID: 53, Type: "controller", Controller: "email", Action: "edit", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 76, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 60, Name: "修改邮件服务器保存", ParentID: 53, Type: "controller", Controller: "email", Action: "modify", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 77, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 61, Name: "启用邮件服务器", ParentID: 53, Type: "controller", Controller: "email", Action: "used", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 78, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 62, Name: "删除邮件服务器", ParentID: 53, Type: "controller", Controller: "email", Action: "delete", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 79, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 63, Name: "登录认证", ParentID: 53, Type: "controller", Controller: "auth", Action: "list", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 81, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 64, Name: "添加登录认证", ParentID: 53, Type: "controller", Controller: "auth", Action: "add", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 82, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 65, Name: "添加登录认证保存", ParentID: 53, Type: "controller", Controller: "auth", Action: "save", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 83, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 66, Name: "修改登录认证", ParentID: 53, Type: "controller", Controller: "auth", Action: "edit", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 84, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 67, Name: "修改登录认证保存", ParentID: 53, Type: "controller", Controller: "auth", Action: "modify", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 85, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 68, Name: "删除登录认证", ParentID: 53, Type: "controller", Controller: "auth", Action: "delete", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 86, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 69, Name: "启用登录认证", ParentID: 53, Type: "controller", Controller: "auth", Action: "used", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 87, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 70, Name: "登录认证文档", ParentID: 53, Type: "controller", Controller: "auth", Action: "doc", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 88, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 71, Name: "系统管理", ParentID: 0, Type: "menu", Controller: "", Action: "", Icon: "icon-system", Target: "", IsDisplay: 1, IsMenu: 1, Sequence: 8, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 72, Name: "快捷链接", ParentID: 71, Type: "controller", Controller: "link", Action: "list", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 81, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 73, Name: "添加链接", ParentID: 71, Type: "controller", Controller: "link", Action: "add", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 82, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 74, Name: "添加链接保存", ParentID: 71, Type: "controller", Controller: "link", Action: "save", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 83, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 75, Name: "修改链接", ParentID: 71, Type: "controller", Controller: "link", Action: "edit", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 84, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 76, Name: "修改链接保存", ParentID: 71, Type: "controller", Controller: "link", Action: "modify", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 85, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 77, Name: "删除链接", ParentID: 71, Type: "controller", Controller: "link", Action: "delete", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 86, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 78, Name: "系统联系人", ParentID: 71, Type: "controller", Controller: "contact", Action: "list", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 91, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 79, Name: "添加联系人", ParentID: 71, Type: "controller", Controller: "contact", Action: "add", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 92, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 80, Name: "添加联系人保存", ParentID: 71, Type: "controller", Controller: "contact", Action: "save", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 93, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 81, Name: "修改联系人", ParentID: 71, Type: "controller", Controller: "contact", Action: "edit", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 94, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 82, Name: "修改联系人保存", ParentID: 71, Type: "controller", Controller: "contact", Action: "modify", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 95, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 83, Name: "删除联系人", ParentID: 71, Type: "controller", Controller: "contact", Action: "delete", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 96, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 84, Name: "导入联系人", ParentID: 71, Type: "controller", Controller: "contact", Action: "import", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 97, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		// 统计管理
		{ID: 85, Name: "统计管理", ParentID: 0, Type: "menu", Controller: "", Action: "", Icon: "icon-chart", Target: "", IsDisplay: 1, IsMenu: 1, Sequence: 9, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 86, Name: "数据统计", ParentID: 85, Type: "controller", Controller: "static", Action: "default", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 91, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 87, Name: "ajax获取空间文档排行", ParentID: 85, Type: "controller", Controller: "static", Action: "spaceDocsRank", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 92, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 88, Name: "ajax获取收藏文档排行", ParentID: 85, Type: "controller", Controller: "static", Action: "collectDocRank", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 93, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 89, Name: "ajax获取文档数量增长趋势", ParentID: 85, Type: "controller", Controller: "static", Action: "docCountByTime", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 94, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 90, Name: "系统监控", ParentID: 85, Type: "controller", Controller: "static", Action: "monitor", Icon: "", Target: "", IsDisplay: 1, IsMenu: 0, Sequence: 95, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 91, Name: "ajax获取服务器状态", ParentID: 85, Type: "controller", Controller: "static", Action: "serverStatus", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 96, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 92, Name: "ajax获取服务器时间", ParentID: 85, Type: "controller", Controller: "static", Action: "serverTime", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 97, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
		{ID: 93, Name: "测试邮件服务器", ParentID: 53, Type: "controller", Controller: "email", Action: "test", Icon: "", Target: "", IsDisplay: 0, IsMenu: 0, Sequence: 80, ListOrder: 0, IsDelete: 0, CreateTime: now, UpdateTime: now},
	}

	for _, p := range privileges {
		DB.Create(&p)
	}
	logx.Infof("[migrate] Seeded %d default privileges", len(privileges))
}

// seedRolePrivileges 补充角色权限关系
func seedRolePrivileges(now int) {
	var count int64
	DB.Model(&usermodel.RolePrivilege{}).Count(&count)
	if count > 0 {
		return
	}

	// 普通用户权限
	userPrivileges := []int{1, 2, 3, 4, 5, 6, 7, 8, 9}
	for _, pid := range userPrivileges {
		DB.Create(&usermodel.RolePrivilege{
			RoleID:      3,
			PrivilegeID: pid,
			CreateTime:  now,
			UpdateTime:  now,
		})
	}

	// 管理员权限
	adminPrivileges := []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 37, 38, 39, 40, 41, 42, 43, 44, 45, 46, 47, 48}
	for _, pid := range adminPrivileges {
		DB.Create(&usermodel.RolePrivilege{
			RoleID:      2,
			PrivilegeID: pid,
			CreateTime:  now,
			UpdateTime:  now,
		})
	}

	logx.Info("[migrate] Seeded default role privileges")
}

// NotDeleted 统一的未删除过滤 Scope（is_delete = 0）
// 自动添加表名前缀，避免多表JOIN时Column 'is_delete' in WHERE is ambiguous错误
func NotDeleted(db *gorm.DB) *gorm.DB {
	if db.Statement.Table != "" {
		return db.Where(fmt.Sprintf("`%s`.is_delete = ?", db.Statement.Table), 0)
	}
	return db.Where("is_delete = ?", 0)
}

// generateRandomPassword 使用 crypto/rand 生成指定长度的随机密码
// 字符集：A-Za-z0-9（避免特殊字符导致的转义/复制问题）
func generateRandomPassword(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		// 失败时降级为时间戳派生（极少触发）
		for i := range b {
			b[i] = charset[time.Now().UnixNano()%int64(len(charset))]
		}
		return string(b)
	}
	for i := range b {
		b[i] = charset[int(b[i])%len(charset)]
	}
	return string(b)
}
