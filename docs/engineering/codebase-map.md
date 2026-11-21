# 代码地图

> `AGENTS.md` 只留命令与红线，模块细节在这里。数字均可用文中命令重算，与 `README.md` 的 Known gaps 表分开维护。

## 启动顺序

Logger → Config → DB（GORM）→ Cache（memory/Redis）→ Search（Bleve，自动重建索引）→ Vector（Bolt HNSW / Chroma / Qdrant）→ AI 模块 → Text2SQL → Gin `:3001`。

入口三文件在 `app/server/`：`main.go`（启动编排）、`ai_init.go`（AI 装配与 LLM 适配）、`server.go`（Gin 引擎、路由、静态资源）。

## 后端分层

API（`internal/api/`）→ Service（`internal/service/`）→ Repository（`internal/repository/`）→ Model（`internal/model/`），每层按域分子包。

| 层 | 域子包 | 说明 |
|---|---|---|
| service / repository | 9 个：admin、ai、auth、knowledge、schema、search、social、user、workspace | 两层同构 |
| api | 上述 9 + collab、common、search、websocket（12） | 路由按域注册 |
| model | 去掉 auth，加 collab、common、eval、search（11） | **无 `model/auth/`**，auth 模型在 `model/user/` 与 `model/admin/oidc_provider.go` |

只有 `repository/` 有根级文件（`db.go` + 6 个 `migrate_*.go` + `migration_registry.go`）；`api/`、`service/`、`model/` 根级无 `.go`。

新增模块：`collab/`（Yjs 实时协作）、`cache/`（内存 + Redis）、`queue/`、`response/`（统一响应）、`brand/`（产品名唯一落点，见 `docs/design/brand.md`）。

## AI 模块（19 个子包）

| 包 | 职责 | 关键点 |
|---|---|---|
| `provider/` | 多 LLM 适配 | OpenAI 兼容 + Anthropic 原生；三级 primary/fast/reasoning；熔断 5 次/30s；指数退避；SSE；令牌配额 |
| `rag/` | 检索增强 | 96 个根级 `.go`（88 源 + 8 测试），含 6 个子包共 107；九阶段管线见 `docs/architecture/rag-pipeline.md` |
| `analyzer/` | 知识分析 | 实体抽取/消解、分类、去重、矛盾检测、质量评分、图谱增强；增量/快检/单文档/全量 |
| `text2sql/` | 自然语言转 SQL | CoT 生成、执行反馈、多候选、AST 校验、外键发现、策略与信任子包；复杂度 0–100 决定档位 |
| `embedding/` | 向量化与存储 | Bolt HNSW（M=16/efSearch=50）、Chroma v2、Qdrant REST；启动自载入，失败退化为暴力扫描 |
| `schema/` | 库表结构解析与检索 | 表/列向量化、切块 |
| `mcp/` | MCP 客户端 | stdio / SSE / streamable HTTP；多服务、30s 健康检查、3 次重连、`server:tool` 前缀 |
| `connector/` | 文件与网页接入 | PDF/DOCX/XLSX 解析 + 抓取 |
| `delivery/` | 投递 | 邮件、webhook、通知、订阅 |
| `eval/` | 评估 | RAGAS |
| `shared/` | 共用件 | SQL 校验、RRF（k=60）、关键词、历史 |
| `scheduler/` | 定时分析 | Cron |
| `observability/` | 可观测 | 指标、追踪、provider 埋点 |
| `voice/` `agent/` `router/` `facade/` `writer/` `tokenizer/` | 语音、Agent、路由、门面、写回、分词 | `facade` 是 AI/Schema/Text2SQL 的统一编排入口 |

AI 模块共 232 个 Go 文件（`internal/ai/` 递归）；Text2SQL 另 88 个（其中 82 在包根）。`internal/ai/data/` 是运行期数据目录，不是包。

## 前端

- Vue 3.5 + TS + Vite + Pinia + Vue Router，SPA 在 `web/`；Node `^20.19.0 || >=22.12.0`。
- 自动导入 `vue` / `vue-router` / `pinia` 与 `composables`、`stores`。
- Apple 设计系统：`web/src/components/apple/` 下 50 个 `Ap*.vue`（`ls web/src/components/apple/Ap*.vue | wc -l`）+ `SpotlightSearchResult.vue`。
- **零 CSS 文件**：全部走内联 `:style` + token 函数（`cv()`、`AppleType`、`AppleSpace`、`AppleRadius`）。共享样式在 `styles/layout/page-styles.ts`，AI 子页在 `styles/ai/ai-page-styles.ts`（re-export 前者）。
- 个性化档位真相源：`styles/theme/personalization.ts`、`accents.ts`；行为唯一落点在 `stores/app/ui.ts`；口径见 `docs/engineering/theme-system.md`。
- AI 设置页 `AISettingsLayout.vue`：6 组 31 个子页，`?section=<key>` 寻址，注册三处必须同步（`sectionComponents` / `sectionMeta` / `groups`）。
- AI 对话页 `AIChatView.vue`：**只有两栏**（220px 会话列表 + 消息列），没有右栏；`token_usage` / `duration_ms` 到达但不渲染。
- 协作编辑：`composables/collab/useCollaboration.ts`，经 `composables/editor/useEditMode.ts` 消费；无 `components/collab/` 目录。

## 数据库

- MySQL 8.0（utf8mb4），表前缀 `aw_`；两份脚本：`databases/table.sql`（DDL，96 表）+ `data.sql`（种子）。
- **`table.sql` 是生成物**：`go run ./tools/genddl`，模型来自 `repository.RegisteredModels()`，索引来自 `AllIndexSpecs()`。每条 `CREATE TABLE` 前带 `DROP TABLE`，**只适用于空库**。
- 已有库升级靠启动 AutoMigrate + `ensureMissingColumns` / `ensureAllIndexes` / `dropLegacyIndexes`；关键列另进 `missingColumnFallbacks()` 与 `criticalColumns`，由 `migrate_columns_test.go` 钉住（AutoMigrate 有四条静默不补的路径：SKIP_MIGRATE、旧构建、迁移报错只 Warn、无 ALTER 权限）。
- `data.sql` 的 `aw_config` 种子与 `repository.DefaultConfigSeeds()` 逐行对齐，安装向导引用同一份。
- 导出/回灌在 `internal/utils/dbdump`（纯 Go）：一致性快照事务、标识符与值转义、`max_allowed_packet` 收敛、默认不输出 `DROP TABLE`、按词法切语句；调用方 `tools/aidump` 与 `service/admin/backup.go`。

## 检索与限流

- Bleve v2（gse 中文分词）+ 拼音索引；向量 Bolt / Chroma / Qdrant。
- 三档限流：`apiLimiter` 300/分钟/IP（health、images、attachments、assets、static 豁免）、`strictLimiter` 10/分钟/IP/路径（登录、超管、安装）、`refreshLimiter` 30/分钟/IP/路径（`/auth/refresh` 独占）。数字由 `TestLimiterTiers` 钉住；429 带 `Retry-After`，前端按状态码分档呈现。
- 部署前提：`server.trusted_proxies` 为空时 gin 不读 `X-Forwarded-For`，反向代理后全站共用一个 IP 配额。
- 搜索行为落 `aw_search_log`，权限过滤后异步写；趋势分桶用应用时钟锚定的 `(create_time - startOfWindow) DIV 86400`，不用 `FROM_UNIXTIME`。

## 统计口径

- Go：`git ls-files '*.go' | wc -l`；测试：`git ls-files '*_test.go' | wc -l`；Vue：`git ls-files '*.vue' | wc -l`。
- 包内文件数用 `find <dir> -maxdepth 1 -name '*.go' | wc -l`——git 的 `*` 会跨 `/`，`git ls-files 'internal/ai/rag/*.go'` 返回的是递归数。
- 写进文档的每个数字都要附命令；`README.md` 的 Known gaps 由 `tools/repocheck/release_claims_test.go` 现场重算。
