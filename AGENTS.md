# AGENTS.md — Citebase

Enterprise AI knowledge management platform. Go + Gin backend, Vue 3 + Vite frontend, Apple-style design system.

## Quick Commands

```bash
# Backend (from repo root)
go run ./app/server              # run dev
go build -o citebase.exe ./app/server  # build binary

# Frontend (from web/)
npm run dev        # dev server :3000
npm run lint:check # check lint
npm run type-check # check types
npm run test       # vitest run
npm run check:style # 七道门禁：设计 token + 主题契约 + 图标一致性 + 设计教条 + WCAG 对比度 + 玻璃层普查 + 布局契约（pre-commit 只跑第一道）
npm run check:theme # 只跑主题契约门禁 web/scripts/check-theme-contract.mjs
npm run check:design # 只跑设计教条门禁 web/scripts/check-design-doctrine.mjs
npm run check:contrast # 只跑 WCAG 对比度门禁 web/scripts/check-contrast.mjs（实心四套 + 玻璃/透明增强按 alpha 合成后再量，浮层自适应的两端与回落值一起钉）
npm run check:glass    # 只跑玻璃层普查 web/scripts/check-glass-layers.mjs（每处 backdrop-filter 走材质令牌、登记在册、逐条交代压在谁上面）
npm run check:glass:runtime # 玻璃运行时普查（需 SNAP_USER/SNAP_PASS）：同屏模糊层数/嵌套/滚动边缘，渲染结果上判并截图（默认跑满四档玻璃轮）
npm run check:theme-rounds # 主题轮巡检（需 SNAP_USER/SNAP_PASS）：--themes=all 的 8 档外观×材质逐档验「真的切过去了 / 材质轴真的改变渲染 / 浮层档形态与材质一致 / 对比度过线」。Git Bash 下要 MSYS_NO_PATHCONV=1
npm run check:layout # 只跑布局契约门禁 web/scripts/check-layout-contract.mjs
npm run check:i18n-runtime # 运行时 i18n 巡检（需前端 :3000 + 后端 :3001）：英文档下逐条路由量"看得见的中文"，
                           # 并按 /api/ 响应体把「代码文案」与「用户数据」分开 —— 代码文案预算 0，出现即红
npm run gen:theme-preview # 从主题数据生成 docs/ui-previews/theme-live-preview.html
npm run ci         # lint + typecheck + test + build (full gate)
npm run format     # prettier format
npm run format:check # verify formatting (no writes)
npm run e2e        # playwright E2E tests

# Makefile targets (from repo root) — run `make help` for the full list
make dev           # start backend dev
make dev-frontend  # start frontend dev
make build         # build backend
make build-frontend # build frontend
make build-all     # build everything
make test          # go test ./internal/... ./tools/repocheck/...（repocheck 管 .github 模板与 SECURITY.md）
make test-coverage # tests with coverage report
make lint          # golangci-lint or gofmt + go vet
make lint-frontend # frontend lint
make typecheck     # frontend type check
make check-docs    # markdown 相对链接必须落到真实文件（tools/validate-doc-links.mjs）
make changelog     # 从 git log 重生成 CHANGELOG.md（tools/gen-changelog.mjs，勿手编）
make test-middleware / make test-response  # targeted Go test suites
make ci            # full CI gate (lint + test + build + frontend build + check-docs)
make swagger       # regenerate Swagger docs
make docker-build  # build Docker image
make docker-up     # docker-compose up -d
make docker-down   # docker-compose down
make docker-logs   # view Compose logs
make db-init       # initialize database (MySQL required)
make db-dump       # 整库导出 SQL（tools/aidump，纯 Go，不需要 mysqldump）
make clean         # remove build artifacts

# Windows helpers (in scripts/)
scripts/dev.bat              # builds backend + starts both dev servers
scripts/package-windows.bat  # builds the Windows zip, plus Citebase-Setup-<version>.exe when Inno Setup is installed
```

## Architecture

### Backend
- **Entry**: 3 files at `app/server/` — `main.go` (boot orchestrator), `ai_init.go` (AI module wiring, LLM provider adapter), `server.go` (Gin engine, routes, static serving).
- **Boot sequence**: Logger → Config → DB (GORM) → Cache (memory/Redis) → Search (Bleve, auto-reindex) → Vector (Bolt HNSW/Chroma/Qdrant) → AI module → Text2SQL → Gin server on `:3001`.
- **Layered**: API (`internal/api/`) → Service (`internal/service/`) → Repository (`internal/repository/`) → Model (`internal/model/`).
- **Sub-package pattern**: each layer has domain sub-packages. `service/` and `repository/` share the same 9 (`admin`, `ai`, `auth`, `knowledge`, `schema`, `search`, `social`, `user`, `workspace`); `api/` adds `collab`, `common`, `search`, `websocket` (12); `model/` drops `auth/` and adds `collab`, `common`, `eval`, `search` (11) — **there is no `internal/model/auth/`**, auth models live in `model/user/` and `model/admin/oidc_provider.go`. **Only `repository/` has root-level files** (`db.go` + 6 `migrate_*.go` + `migration_registry.go`); `api/`, `service/` and `model/` have zero root-level `.go`.
- **New modules**: `collab/` (Yjs CRDT real-time collaborative editing), `cache/` (memory + Redis dual-mode), `queue/` (in-memory message queue), `response/` (unified API response), `brand/` (产品名的唯一落点，见下).
- **品牌名只有一个落点：`internal/brand/brand.go`**（2026-10-11 收口）。`DefaultName` 是代码默认名，
  `SiteName()` 按 `main_title` → `system_name` → 默认名三级取值（`ResolveSiteName` 是那条唯一判据），
  `SiteDescription()` 供分享页那句通用描述。改名只改这一处 + `web/src/utils/core/app-info.ts` 的 `APP_NAME`。
  **`brand` 零依赖**（只 import `strings`）：`repository` 引它取种子值，它反过来引 `repository` 就成环，
  所以配置读取由 `app/server/main.go` 在 DB 就绪后 `InstallBrandReader` 注入。
  公开端点 `GET /api/v1/site` 回 `{name, description}`（前端 `web/src/utils/core/site-brand.ts` 是唯一消费者；
  标签页标题、登录页大标题、字标、移动端头部、分享页脚、邮件预览都取它；
  `description` 是「主页描述」那一栏，没填就是空串，前端据此退回语言包里那句一句话定位，
  由 `applySiteMetaDescription()` 盖掉 `index.html` 里写死的静态默认值 —— 换语言时重写同一处；
  管理端「系统设置 → 基础」存完这三栏（键清单在前端 `BRAND_CONFIG_KEYS` / `affectsSiteBrand()`）
  会各自 `refreshSiteBrand()`，单行保存与批量保存两条路径都接了 —— 只接一条的症状是"改完要手动刷新一次"）。
  **接线会把存量库的脏值第一次暴露到界面上**：2026-10-12 实跑该端点回的是 `AIWiki` ——
  改名那次没动过任何库里的 `main_title`，那两行以前没人读所以看不见。
  现在由 `repository.normalizeLegacySiteNames()` 在启动时归一（只认整串等于旧产品名的写法，
  清单在 `brand.LegacySiteNames()`，改行数>0 时打一条 WARN 告诉管理员去哪儿改回来）；
  不在读取端偷偷绕过 —— 那样后台一栏写着 AIWiki、界面写着 Citebase，等于再造一处不一致。
  `brand_hygiene_test.go` 因此多了一条**带理由的豁免**（`internal/brand/`），
  而清单内容本身被 `TestLegacySiteNamesAreNothingButOldBrand` 钉住：混进 "Wiki" 这类泛词就红。
   三台常驻门禁：`tools/repocheck/brand_source_test.go`（后端字面量只准一处 + 三份种子逐字段一致 +
   前后端默认名一致 + 前后端品牌配置键清单一致 + 安装向导不许再抄清单 + swagger `@title` 首词必须是 `DefaultName`
   —— 注解在注释里，
   字面量判据一条也数不到，`make swagger` 还会把旧名字原样再生一遍）、
   `web/tests/brand-closure.test.ts`（前端同两条 + 语言包不许把品牌名当译文 + `{app}` 占位符每个调用点必须传参
   + **静态外壳四件**：`index.html` 的 `<title>` 与 meta description、PWA manifest 的 `name`/`short_name`、
   两枚图标的 `aria-label`，一律断言等于 `APP_NAME` —— 这四处构建期定死、JS 起来之前就被看到，
   却不在 `sourceFiles()` 的 `src/**` 射程内，2026-10-16 的品牌全面检查就是从它们数出裸副本的）、
   `internal/brand/brand_test.go`。
   **刻意见外的几份**（改名靠人，`git grep -n Citebase`）：`web/e2e/smoke.spec.ts` 的用例名、
   `scripts/*.bat` 的 echo/title 行、`assets/readme/*.svg` 设计稿与 `web/scripts/gen-*` 生成物 ——
   测试名、终端提示、图片和构建产物里的字，进门禁只会换来一堆为过门禁而改的假一致。
  **旧名 `AIWIKI_*` 的射程另有 `tools/repocheck/brand_hygiene_test.go` 管**（AST 只拦用户可见串）。
- **后端说给人听的话只有一张表：`internal/response/messages.go`**（2026-10-11 批次十六收口）。
  形状是「稳定键 + 中文原文」；响应封套多带一个 `message_key`，`message` 仍是中文（curl、CLI、
  邮件模板与不认识键的旧部署读它）。**为什么要有这条链**：此前实测 245 处把中文直接写进
  `response.Xxx(c, "中文")`（37 个文件、去重 123 条），而前端只有 `err.message` 一根管子 ——
  报错页、toast、登录页错误位显示的都是它，所以英文界面必然读到中文，且**前端修不了**
  （那句话不是前端产的串，换语言包也盖不住）。这是 i18n 十四批够不到的唯一一类欠账。
  消费点只有一处：`web/src/api/client.ts` 的 `serverMessage()` 在构造 `ApiError` 时按当前语言替换，
  于是两百多个显示 `err.message` 的调用点不改就跟着变。认不出键就退回服务端原文，但先用
  `i18n.global.te()` 探一次 —— 直接 `t()` 遇到缺失键会把 `server.xxx` 这种键名印到界面上。
  英文文案住 `web/scripts/server-messages.en.json`（前端拥有），语言包
  `web/src/i18n/locales/server-messages.ts` 是**生成物**：`cd web && npm run gen:server-messages`。
  加新话术的顺序：表里加常量+中文 → en.json 加英文 → 跑生成 → 把新键登记进
  `serverMessageKeyBaseline`（那台门禁拒绝「加了键没登记基线」）。三台常驻门禁：
  `internal/response/messages_test.go`（出口发出去的那句逐字等于登记值、sentinel 映射、未登记不猜）、
  `tools/repocheck/server_message_test.go`（键集基线只增不改、死键、插值、未迁移数只降不升）、
  `web/tests/server-messages.test.ts`（生成物是否过期、中英键集双向、英文侧不许夹中文）。
  直出中文现在归零（`serverMessageUnmigratedBaseline = 0`）。最后 5 处是拼接型
  （`"参数错误: " + err.Error()` 那一族），解法是**再开一格** `message_detail`：句子要翻译，
  原因（哪个字段没填对、冲突的是哪个值）通常不用；拼成一句的结果是英文界面读到半中半英的串，
  而且永远翻不干净。前端把两格按语言包里的 `chrome.errorWithDetail` 合起来（括号全角/半角
  本身也是语言事实，不在 JS 里拼）。
  **同一类里的第二档也已钉住**：`42` 处中文写在**数据字段**里（`gin.H{"message": "..."}` 这一族，
  20 个文件），封套的键帮不上忙 —— 由 `serverMessageFieldBaseline` 管，只许降。
  **服务层不再是盲区，但也只收了真到用户屏幕的那部分**：263 处中文 `errors.New` 里，
  经核实只有 7 句会走到用户面前（其余经 `ServerErrorSafe`/`FailSafe` 只进日志）——
  这 7 句已加哨兵接上键（`ErrSQLHistoryNotFound` 等），服务层剩下的中文按"日志用"留着。
  SSE 帧与通知是另外两档：帧里的工具错误原文直出、通知标题正文是**事件时落库的冻结文本**
  （读时翻译不可能），要本地化得存键 + 参数，属于产品决策，还没做。
- **Total**: 731 Go files tracked (611 source + 120 test) and 208 `.vue` files, counted 2026-11-16 after token-quota enforcement and doc-alignment pass. Counts drift — verify with `git ls-files '*.go' | wc -l`, `git ls-files '*_test.go' | wc -l` and `git ls-files '*.vue' | wc -l`.

### Frontend
- **SPA** in `web/`. Vue 3.5 + TypeScript 6.0 + Vite 8 + Pinia 3 + Vue Router 5.
- **Node.js**: Requires `^20.19.0 || >=22.12.0` (see `web/package.json` engines).
- **Auto-imports** enabled for vue/vue-router/pinia and composables/stores via `unplugin-auto-import` + `unplugin-vue-components`.
- **个性化与动效（2026-10-16 批1–批4）**: 档位清单的真相源是 `web/src/styles/theme/personalization.ts`（动效强度/对比度/透明度/版面人格）
  与 `web/src/styles/theme/accents.ts`（强调色色板，8 档 + `auto`）；行为只有一个落点 `web/src/stores/app/ui.ts`。
  **时长只准写 `var(--apple-dur-*)`**（唯一事实源 `BASE_DURATION_MS`，教条 G32），**实心强调色上的前景只准 `on-blue`**（G33），
  **入场编排只准出现在登记的公开版面**（G34：登录 / 作者登录 / 安装 / 分享 / 整页错误）。方案与取舍见
  `docs/design/personalization.md`，档位口径见 `docs/engineering/theme-system.md`。
- **Apple Design System**: All UI uses `Ap*` components (`web/src/components/apple/`, 50 `Ap*.vue` plus `SpotlightSearchResult.vue`) styled via Apple Design Tokens (`web/src/utils/core/apple-tokens.ts`). 数一下就是 `ls web/src/components/apple/Ap*.vue | wc -l` —— 这里曾长期写着 40，因为 `ApTooltip` 删除、若干新组件加入后没人重数过。
- **Shared Styles**: `web/src/styles/layout/page-styles.ts` for general pages, `web/src/styles/ai/ai-page-styles.ts` for AI sub-pages. AI pages re-export from page-styles and add AI-specific styles (`aiSub*` prefix).
- **Inline Style Pattern**: No CSS/SCSS files — all styling via inline `:style` bindings using Apple Token functions (`cv()`, `AppleType`, `AppleSpace`, `AppleRadius`, etc.).
- **i18n**: `vue-i18n` with `en-US` and `zh-CN` locales (`web/src/i18n/`).
- **PWA**: `vite-plugin-pwa` with service worker, caches doc content and vendor chunks.
- **Collaborative Editing**: Yjs CRDT real-time collaboration. Frontend entry is the composable `web/src/composables/collab/useCollaboration.ts`, consumed via `composables/editor/useEditMode.ts` — there is **no** `components/collab/` directory; editor UI lives in `web/src/components/document/`, server hub in `internal/collab/`.

### AI Frontend Architecture
- **AIChatView** (`web/src/views/ai/AIChatView.vue`) → chat UI with **two panes only**: a 220px fixed conversation list (`components/ai/chat/ChatSidebar.vue`, `flexShrink: 0`) + the message column (`flex: 1`). **There is no right panel** — do not add one from this description. `token_usage` / `duration_ms` do arrive in the stream's terminal meta frame but are never rendered. Under `isMobile` (<641px) the list becomes a 260px left drawer titled `会话列表`. Rendered at `/ai/workspace`.
- **AISettingsLayout** (`web/src/views/ai/AISettingsLayout.vue`) → sidebar navigation with 6 groups, 31 sub-pages rendered dynamically via `<component :is>` with `defineAsyncComponent` lazy loading and `KeepAlive` caching (rendered at `/ai/settings`). 子页按 `?section=<key>` 寻址，注册三处必须同步：`sectionComponents` / `sectionMeta` / `groups`。
  - **概览**: 系统概览, 知识分析, 搜索概览
  - **配置**: 模型配置, RAG + 向量, Prompt 模板
  - **数据**: Schema 管理, 知识库管理, 文档导入, 智能标签, SQL 执行历史, SQL 审计追踪, 少样本示例, 高级问答试跑
  - **工具**: MCP 服务器, 工作流管理, Widget 嵌入, 写作助手, 知识编译, 词条生成, Wiki 治理
  - **监控**: 可观测性, Token 用量, 运维管理, SQL 安全策略, RAGAS 评估, 数据敏感性规则, 评估控制台, 能力成熟度, 记忆管理
  - **高级**: A/B 实验管理

### Database
- **个性化偏好跟账号走（2026-10-16）**：`aw_user.preferences`（TEXT，存校验后的 JSON）。判据与规则只有一处：`internal/model/user/preferences.go`
  （`NormalizePreferences` / `MarshalPreferences` / `ParsePreferences`）；接口 `GET /auth/me` 带 `preferences`（`null` = 该账号从没同步过，
  **不许合成成默认档** —— 那样每次登录都会盖掉本地选择）、`PUT /auth/me/preferences`（self-service，登记在
  `rbac_coverage_test.go` 的 `sensitivePublic`）。前端同步判据是纯函数 `web/src/utils/core/preference-sync.ts` 的 `decideSync`
  （本地刚改过直接推、不比时间戳；失败记 `ui:pref-pending` 下次补推；采纳期间不回推，防回声）。
  **载荷现在是八格**：`theme / material / glass_clear`（批前）+ `motion / contrast / transparency`（批1–2）
  + `accent / persona`（批3–4）。加一格**不需要 DDL**（它是同一列 JSON），但 `databases/table.sql` 里那一列
  本身仍未落到存量库 —— 要重启后端才由 AutoMigrate + `ensureMissingColumns` 补，这件事归维护者排期。
  **清单的真相源有两处前端文件**：外观/材质在 `themes.ts`，行为与个性化档在 `personalization.ts` 与 `accents.ts`；
  服务端那份是为校验持有的第二份，由 `tools/repocheck/preference_contract_test.go` 逐条比对（含每档的**默认值**：
  前三条行为档默认 `auto`，persona 默认 `plain` —— 它没有 auto，系统里没有“要不要编排”这个开关）。
  加一档只改一边，用户在另一台设备上的选择会被当非法值丢掉。
  **老数据行的读法有例外**：外观/材质/透明变体认不得 → 整份判“没配过”；新格缺失或认不得 → 用本机值补。
  把后者也判成非法，症状是升级到批1–4 之后**这些账号连主题都同步不了**（老行看起来像坏数据）。
  `docs/engineering/theme-system.md`「偏好跟账号走」与「个性化五档」两节是完整口径。
- MySQL. Two scripts only: `databases/table.sql` (DDL, 96 tables) + `databases/data.sql` (DML, seed rows). All tables use the `aw_` prefix.
- **`table.sql` 是生成物，勿手工编辑**：由 `go run ./tools/genddl` 产出，模型清单取自 `repository.RegisteredModels()`（`internal/repository/migrate_*.go`），复合/前缀索引取自 `repository.AllIndexSpecs()`（`db.go` 的 `allIndexes`）。新增表只需在对应 `migrate_*.go` 注册模型 + 重跑 genddl，DDL 与启动时 AutoMigrate 不会再分叉。
- `table.sql` 每条 `CREATE TABLE` 前都带 `DROP TABLE` — **仅适用于空库**；已有库靠启动 AutoMigrate + `ensureMissingColumns` / `ensureAllIndexes` / `dropLegacyIndexes` 升级（`AIWIKI_SKIP_MIGRATE=true` 可跳过）。
  **给已有表加列时，"AutoMigrate 会自动补"不是可靠的假设**：它有四条静默不补的路径（SKIP_MIGRATE、
  跑的是旧构建的二进制/镜像、该模型 migrate 报错只 Warn、DB 账号无 ALTER 权限），而缺列的症状是功能静默降级不是启动失败。
  所以关键列要同时进 `missingColumnFallbacks()`（显式补列清单）与 `criticalColumns`（启动核对，**不受 SKIP_MIGRATE 影响**，
  见 `internal/repository/db.go` 的 `verifyCriticalColumns`），三条一致性由 `internal/repository/migrate_columns_test.go` 钉住。
- `data.sql` 的 `aw_config` 种子行与 `repository.DefaultConfigSeeds()` 逐行对齐；改后者须同步改前者。
  **校验者是 `tools/repocheck/brand_source_test.go`，不是 `tools/validate-dml.mjs`** —— 后者只查
  "INSERT 的目标表与列在 table.sql 里存在"，从不比值；这句指错判据的写法存在了很久，
  期间安装向导确实抄出了第二份清单并与种子分叉（值、标签、类型、排序号四处全不同）。
  种子清单现在只准 `DefaultConfigSeeds()` 一处写，安装向导（`internal/api/auth/install.go`）引用它。
- **导出与回灌引擎在 `internal/utils/dbdump`（纯 Go，不需要 mysqldump / mysql 客户端）**：导出用一条 `REPEATABLE READ` + `START TRANSACTION WITH CONSISTENT SNAPSHOT` 事务读全库，等价于 `mysqldump --single-transaction`；标识符加反引号、值按 MySQL 转义规则渲染（非 UTF-8 字节走 `0x` 十六进制，datetime 保留微秒），单条 INSERT 的字节上限还按服务端 `max_allowed_packet` 再收敛，免得产出回灌时被拒的文件。默认**不输出 `DROP TABLE`**（覆盖式回灌才 `-add-drop-table`）。恢复侧按 MySQL 词法切语句（字符串与标识符里的分号不算结束符，认 `--`/`#`/`/* */` 注释），逐条在**同一条连接**上执行（`SET SESSION` 才有效），`RestoreOptions.ReplaceTables` 在建表前先 DROP 同名表——备份文件默认不带 DROP，直接往有数据的库上建表会撞 Error 1050；含 `DELIMITER`（存储过程）的文件明确拒绝。调用方：
  - `tools/aidump`（`make db-dump`）命令行导出，DSN 与 app 同源：`-dsn` > `AIDUMP_DSN` > `configs/config.yaml`（含 `AIWIKI_DB_DSN` 覆盖）。
  - `internal/service/admin/backup.go` 的 `CreateBackup` / `RestoreBackup`（"系统管理 → 备份"）：SQL 与数据目录一起打成 `data/backups/<id>.zip`。**改前动机**：原先两处分别 `exec` mysqldump 与 mysql，只装了数据库客户端的机器（如 chocolatey 的 mysql-cli 只有 `mysql.exe`）上必然失败，且口令以 `-p<password>` 形式出现在子进程命令行里。

### Search
- Bleve v2 full-text index (Chinese tokenization via gse). Vector store: Bolt (embedded HNSW), Chroma, Qdrant.
- **限流是三档，且 429 不该崩页（2026-10-03）**：`internal/middleware/ratelimit.go` 里 `apiLimiter` 300/分钟/IP（全局，`/api/v1/health`、`/images/`、`/attachments/`、`/assets/`、`/static/` 豁免）、`strictLimiter` 10/分钟/IP/路径（口令猜测面：login、超管入口、安装向导）、`refreshLimiter` **30/分钟/IP/路径**（`/auth/refresh` 独占一档）。refresh 原先和 login 共用 10/分钟，但它是 SPA 的常规动作（令牌一过期就发一次），多开几个标签页或一轮 73 页巡检必然撞上 —— 撞上的表现是整页被 ErrorBoundary 接住，用户读到「渲染内容时发生了未知错误」，而真相只是「你太频繁，等 N 秒」。三档数字由 `TestLimiterTiers` 钉住（把 refresh 改回 10 立刻红，实测过）。每个 429 都带 `Retry-After`（`RateLimiter.RetryAfter()` 按窗口内最早那条请求算剩余秒数，至少 1 秒），前端 `ApiError.retryAfter` 接住，`components/common/error-presentation.ts` 决定呈现：429=「操作太频繁了」、5xx=「服务暂时不可用」（两者都带倒计时并锁住重试按钮，到点自动重试），其余才是一句「页面出现了问题」。**这个决策单独成模块是为了能被测到** —— 挂 ErrorBoundary 测渲染走不通：slot 内容的父链归属让 `onErrorCaptured` 在测试环境里收不到抛错（setup 里抛 → 子节点照常渲染；render 里抛 → 整棵子树空白），两种写法都测的是假东西。另一条部署前提：`server.trusted_proxies` 默认为空 = gin 不读 `X-Forwarded-For`，**放在反向代理后面时所有用户都算成代理那一个 IP**，300/分钟变成整站共享，这才是"经常一直异常"最可能的原因；`configs/config_sample.yaml` 已写明，但部署时必须真的填。
- **展示型指标必须能追到写入路径**（2026-10-03 查出四处假数据）：快捷链接的 `click_count` 在前端类型里是 `click_count?: number`（可选），而后端 `internal/model/admin/link.go` **根本没有这个字段**、全仓 Go `grep click_count` 0 命中 —— 于是卡片「0 次点击」、表格「点击量」列、统计卡「总点击量」=0、「最热链接」=永远取列表第一条，四处一起把不存在的数据源演得很像真的。现在的真实现：`aw_link` 加 `click_count` 列（由 AutoMigrate + `ensureMissingColumns` 补，`databases/table.sql` 是生成物不手改）、`POST /api/v1/links/:id/click` 用 `gorm.Expr("click_count + 1")` **原子自增**（读-改-写会互相吞点击；`is_delete = 0` 也要带上，否则已删链接还能计数）、`RetryAfter` 之外这条走 `MinRole 3`（任何登录用户）—— `/api/v1/links` 整段在 `adminPathPrefixes` 里，**在这条前缀的射程内，"不显式登记规则"等于默认要管理员而不是放行**（射程外恰恰相反，见 Conventions 里 RBAC 那条），将来链接摆到用户面前时每次点击都会 403，而管理页看到的计数还是对的（管理员点的那几次）。两条 RBAC 测试分别钉住"计数只要登录"与"增删改列表仍要管理员"。同批修掉的产品缺陷：卡片视图**一个按钮都没有**（编辑/删除只活在表格视图的 `v-else` 分支里，而默认视图是卡片），URL 也只是 `:subtitle` 纯文本、全站没有任何 `<a href>` 指向它 —— 现在卡片有 `#actions`，URL 是真的可点新标签（`rel="noopener noreferrer"`），文字链接的样式收成一条共享 `inlineLinkStyle`（此前全站没有"可点文字"这一档）。**要写进规矩的判断法**：看到任何计数/统计，先 `grep` 它在后端模型里的字段名；0 命中就是假数据。
- **搜索行为底账落在 `aw_search_log`**（`internal/model/search/search_log.go`），由 `SearchDocuments` / `HybridSearch` 在**权限过滤之后**异步写入（`searchsvc.RecordSearch`，goroutine + recover，写失败不影响检索；记的是用户看得见的结果数，所以"命中 5 篇但都无权访问"算无结果）。`GET /search/stats` 是可选鉴权的公开面，只回 `is_open` + `doc_count`；索引目录绝对路径与日志聚合一律走管理端 `GET /search/monitor`（Auth+AutoRBAC，与 `/search/reindex`、`/search/init` 同组——"含文件系统路径的诊断信息仅限管理员"这条边界是 routes.go 里既有的约定，别为了省事把它并进公开端点）。趋势分桶**不用** `FROM_UNIXTIME`：那个函数按 MySQL 会话时区切自然日，而 DSN 默认不带 `loc`，与服务端时区不一致时整条趋势会平移一天且与 Go 侧生成的日期标签错位；改成由应用时钟锚定的 `(create_time - startOfWindow) DIV 86400`（口径见 `internal/repository/search/search_log.go`），"一天"的定义只有一处来源。窗口计算与归一化/补零/防 NaN 的纯函数有 hermetic 单测（`internal/model/search/search_log_test.go`、`internal/service/search/search_log_test.go`），不依赖数据库。

### AI Backend
- **Module Assembly**: `internal/ai/ai.go` — core wiring for all AI capabilities, initializes 27+ components with 3-tier provider architecture.
- **Provider**: Multi-LLM (`internal/ai/provider/`, 10 files) — OpenAI-compatible + Anthropic native, with connection pooling, circuit breaker (5 failures / 30s reset), exponential backoff retry, SSE streaming. Three tiers: `primary` (core tasks), `fast` (simple extraction), `reasoning` (complex analysis). Token quota system with daily/per-user limits. Factory routes: openai/deepseek/qwen/zhipu/ernie/gemini/ollama → OpenAIProvider; anthropic → AnthropicProvider.
- **RAG**: `internal/ai/rag/` — 96 `.go` at the package root (88 source + 8 test), 107 including the 6 sub-packages (`chunking` `compression` `enricher` `filter` `middleware` `multiagent`). Mind the ruler: `git ls-files 'internal/ai/rag/*.go'` returns **107** because git's `*` crosses `/`; use `find internal/ai/rag -maxdepth 1 -name '*.go' | wc -l` for the 96. Measured 2026-11-16. — 10-step retrieval pipeline: safety guard → semantic cache → question decomposition → query rewriting → hybrid retrieval (vector + keyword + HyDE + GraphRAG) → reranking (CrossEncoder + MMR) → permission filtering → LLM generation → hallucination detection (2 retries) → post-processing (PII masking, citation enrichment). Components: Self-RAG, HyDE, memory manager, multi-hop reasoning, workflow engine (DAG with llm/tool/condition/parallel nodes), graph RAG (depth-2 expansion), adaptive retrieval, corrective RAG, query cache (500 entries, 24h TTL), safety guard, multimodal, 20+ agent tools (search/create/move/query + MCP tools), ReAct agent with streaming, multi-agent orchestration.
- **Analyzer**: `internal/ai/analyzer/` (22 files) — entity extraction/resolution, doc classification, duplicate detection (content hash diffing), contradiction detection, quality assessment, graph enhancement, batch processing, doc parsing (PDF/DOCX/XLSX), event/relation/sentiment/keyword extraction, knowledge graph building. Supports incremental/quick/single-doc/multi-doc/full analysis modes with manual-edit protection.
- **Text-to-SQL**: `internal/text2sql/` (88 files incl. `dialect/` and `trust/` sub-packages) — natural language → SQL with CoT generation, execution feedback loop, multi-candidate generation (parallel), structural validation, FK discovery, RAG adapter, knowledge integration, trust/explain/policy sub-package, function calling generator, multi-hop SQL, query cache, conversation memory. 20+ sub-components: EntityRecognizer, RelationInferer, SQLAssembler, SQLValidator, SchemaLinker, ConversationState, SchemaCachePool, SQLTemplateLibrary, ASTCorrector, FeedbackLoop, CoTPromptBuilder, ExecutionFeedbackLoop, MultiCandidateGenerator, MultiHopSQLReasoner, ExecutionPlanAnalyzer, IntentChecker, LLMRefiner, DialectRewriter, ModelRouterAdapter. Complexity scoring 0-100 routes complex queries to reasoning tier.
- **Schema**: `internal/ai/schema/` (6 files) — DB schema parsing, vectorization, retrieval, chunking.
- **Embedding**: `internal/ai/embedding/` (7 files) — embedder + 3 vector store backends (Bolt HNSW with M=16/efSearch=50, Chroma v2, Qdrant REST) + schema vector store. HNSW auto-loads from BoltDB on startup, falls back to brute-force scan.
- **MCP**: `internal/ai/mcp/` (4 files) — Model Context Protocol client with multiple transports (stdio, SSE, streamable HTTP). Multi-server management, health check (30s ping), auto-reconnect (3 retries, exponential backoff), tool discovery with `server:tool` prefixed naming.
- **Connector**: `internal/ai/connector/` (8 files) — File parsing (PDF, DOCX, XLSX) + web scraper.
- **Delivery**: `internal/ai/delivery/` (8 files) — Email, webhook, notification, subscription.
- **Eval**: `internal/ai/eval/` (4 files) — RAGAS evaluation framework.
- **Shared**: `internal/ai/shared/` (8 files) — SQL validation, RRF fusion (k=60), keywords, history.
- **Scheduler**: `internal/ai/scheduler/` — Cron-based automated analysis.
- **Observability**: `internal/ai/observability/` — metrics, tracing, and provider instrumentation.
- **Voice**: `internal/ai/voice/` — Voice input support.
- **Agent**: `internal/ai/agent/` — Standalone agent module.
- **Router**: `internal/ai/router/` — LLM request routing with complexity-based tier selection.
- **Facade**: `internal/ai/facade/` — Unified AI/Schema/Text2SQL coordination layer with audit trail.
- **Writer**: `internal/ai/writer/` — Analysis result writing.
- **Tokenizer**: `internal/ai/tokenizer/` — Text tokenization.
- **Total AI module**: 232 Go files across 19 sub-packages (`internal/ai/`, measured recursively); Text2SQL adds another 88 under `internal/text2sql/` (82 at its package root). `internal/ai/data/` is a runtime data dir (JWT secret, indexes) — not a package.

## Key Files

| File | Purpose |
|------|---------|
| `app/server/main.go` | Backend entry, boot orchestrator |
| `app/server/ai_init.go` | AI module wiring, LLM provider adapter |
| `app/server/server.go` | Gin engine, route registration, static serving |
| `internal/ai/ai.go` | AI module assembly |
| `internal/ai/facade/ai_facade.go` | Unified AI facade layer |
| `internal/text2sql/generator.go` | Text-to-SQL core (split into generator_*.go by domain) |
| `internal/ai/rag/retriever.go` | RAG retrieval pipeline (hybrid search + rerank + HyDE) |
| `internal/ai/rag/agent_chat.go` | Agent chat orchestration |
| `internal/ai/rag/agent_chat_stream.go` | Streaming agent chat |
| `internal/ai/rag/memory_manager.go` | Cross-session learning, query pattern tracking |
| `internal/ai/rag/workflow_engine.go` | Analysis workflow orchestration |
| `internal/ai/rag/react_agent.go` | ReAct agent implementation |
| `internal/ai/analyzer/analyzer.go` | Knowledge analyzer |
| `internal/ai/analyzer/batch_processor.go` | Batch analysis pipeline |
| `internal/ai/provider/provider.go` | LLM provider interface + dispatch |
| `internal/ai/provider/pooled_provider.go` | Connection-pooled provider |
| `internal/repository/db.go` | DB connection + auto-migration registry |
| `internal/collab/hub.go` | Real-time collaboration hub |
| `internal/collab/server.go` | WebSocket collaboration server |
| `configs/config_sample.yaml` | Config template (copy to config.yaml) |
| `web/vite.config.ts` | Frontend build + proxy config |
| `web/vitest.config.ts` | Test config (happy-dom) |
| `web/src/main.ts` | Frontend entry |
| `web/src/utils/core/apple-tokens.ts` | Apple Design Token system |
| `web/src/styles/layout/page-styles.ts` | Shared page styles |
| `web/src/styles/ai/ai-page-styles.ts` | AI sub-page styles (re-exports page-styles) |
| `web/src/views/ai/AIChatView.vue` | AI workspace main chat view |
| `web/src/views/ai/AISettingsLayout.vue` | AI settings sidebar layout (6 groups, 31 sub-pages) |
| `web/src/components/document/DocumentEditor.vue` | Main editor wrapper (Monaco + Markdown preview split) |
| `web/src/components/document/MonacoEditor.vue` | Monaco-based markdown editor (slash commands, AI ghost text) |
| `web/src/composables/collab/useCollaboration.ts` | Yjs CRDT composable (y-websocket, presence/sync), consumed via `composables/editor/useEditMode.ts` |

## Dev Setup

### Local Development
1. Copy `configs/config_sample.yaml` → `configs/config.yaml`, edit database DSN
2. Create MySQL database, run `databases/table.sql` + `data.sql`
3. Backend: `go run ./app/server` (port 3001)
4. Frontend: `cd web && npm install && npm run dev` (port 3000, proxies `/api/v1` to backend)

### Docker
```bash
cp .env.example .env        # set JWT_SECRET (min 32 chars)
docker-compose up -d        # starts MySQL + Redis + App
docker-compose logs -f app  # view logs
```

## Testing

Backend: `go test ./internal/... -count=1 -timeout 120s` (or `make test`); `make test-coverage` for coverage. Keep the build green with `go build ./app/server`.

Frontend unit tests run under Vitest with the `happy-dom` environment. `globals: false`, so test files must explicitly `import { describe, it, expect } from 'vitest'`. Name files `*.test.ts` / `*.spec.ts` next to the source they cover (or under `tests/`). Coverage: `npm run test:coverage` (v8 provider).

E2E: Playwright via `cd web && npm run e2e` (Chromium).

Full gate: `cd web && npm run ci` (lint + type-check + unit tests + build).

## Conventions


### Where the detail lives (2026-10-08 split)

- Go backend rules → [docs/engineering/backend-conventions.md](docs/engineering/backend-conventions.md)
- Frontend rules (layout contract, content width, components, shared styles, touch & typography) → [docs/engineering/frontend-conventions.md](docs/engineering/frontend-conventions.md)
- "One concept, one implementation" closure list → [docs/engineering/one-implementation.md](docs/engineering/one-implementation.md)
- Theme system (appearance × material) → [docs/engineering/theme-system.md](docs/engineering/theme-system.md)
- Static gates (G/T/R rules, five `check:style` scripts, pre-commit) → [docs/gates/static-gates.md](docs/gates/static-gates.md)
- Runtime inspectors (`snap` and the `check:*` fleet) → [docs/gates/runtime-inspectors.md](docs/gates/runtime-inspectors.md)
- Dated decisions and incident records → [docs/decision-log/](docs/decision-log)

### Frontend

- Frontend code quality target: 0 TypeScript errors, 0 ESLint errors.


## Commit & Pull Request Guidelines

- Use Conventional Commits: `feat:`, `fix:`, `docs:`, `style:`, `refactor:`, `test:`, `chore:` with an optional scope, e.g. `fix(rag): ...`. Chinese and English subjects are both used in this repo; keep the subject under ~72 characters and describe the change rather than one-word messages like `优化`.
- Open PRs against `master`/`main`. Include a short description of the change and rationale, what was tested, and screenshots for UI changes; link any related issues.
- Keep the local gates green before requesting review — the hosted workflow is not wired to this repo's remote (see CI/CD under Gotchas), so `make ci` / `cd web && npm run ci` plus the pre-commit hook are the only enforcement. Note when reading the hook: `.husky/pre-commit` runs `npm run lint:check` (ESLint, error-level only — the 267 `no-console` warnings don't block, since `eslint .` carries no `--max-warnings`) and then `node scripts/check-style-tokens.mjs`, i.e. only the **first** of the five style gates — theme contract, design doctrine, WCAG contrast and the layout contract are NOT checked at commit time, so `npm run check:style` must be run explicitly (the CI workflow does run it, but CI never fires from this remote).
- Local gate before committing: `go build ./app/server` + `go test ./internal/...`; `npm run lint:check`, `npm run type-check`, `npm run test`.

## Gotchas

- Backend serves on `:3001`, frontend dev on `:3000`. Vite proxies `/api/v1`, `/images`, `/attachments` to backend. WebSocket proxy for `/api/v1/collab/ws`.
- All tables use the `aw_` prefix (see `TableName()` in `internal/model/`). Renaming it means touching every model, every hand-written SQL in `internal/repository/` and `databases/*.sql` together.
- Project renamed to Citebase on 2026-10-07 (was AIWiki); three things were deliberately *not* renamed — table prefix `aw_`, dual-read env vars, and historical documents. Rationale: [docs/decision-log/2026-10-07-rename-scope.md](docs/decision-log/2026-10-07-rename-scope.md).
- How the docs are laid out (root vs `docs/` vs `web/src/styles/README.md`): [docs/README.md](docs/README.md).
- Go tests exist in repo (120 test files as of 2026-11-16; recount with `git ls-files '*_test.go' | wc -l`) — verify backend changes by running `go build ./app/server` and `go test ./internal/...`.
- Windows env: use PowerShell, write files as UTF-8 with LF endings; the repo is already all-LF in its blobs — verify before "fixing" mojibake. Line-ending investigation: [docs/decision-log/2026-10-06-line-endings.md](docs/decision-log/2026-10-06-line-endings.md).
- After ANY code change, compile/verify before moving on: `go build ./...` for the backend, and the `web` gates for frontend. Do not leave the tree in a state that does not build.
- Never start a background service (backend, dev server, Docker) unless explicitly asked to.
- `ai-page-styles.ts` re-exports everything from `page-styles.ts` — import from `ai-page-styles` in AI pages to get both general and AI-specific styles.
- AI settings uses `AISettingsLayout.vue` directly as the `/ai/settings` route component with 6 sidebar groups and 31 lazy-loaded sub-pages. Count them with `grep -c "^      { key:" web/src/views/ai/AISettingsLayout.vue`. Modify that file or the sub-views under `views/ai/` for actual changes.
- `configs/config.yaml` and `.env` are gitignored (and in `.dockerignore`). `.env.example` is the committed template — never put real credentials in it.
- Collaborative editing uses Yjs CRDT over WebSocket (`/api/v1/collab/ws`). Backend hub in `internal/collab/`.
- CI/CD: the workflow exists but has never run (this remote never fires); `make ci` and `.husky/pre-commit` are the actual enforcement. Full history: [docs/decision-log/2026-10-07-ci-and-module-path.md](docs/decision-log/2026-10-07-ci-and-module-path.md).
- Release: `scripts/package-windows.bat` is the one build chain (portable zip, plus `Citebase-Setup-<version>.exe` when Inno Setup is present); `scripts/dev.bat` runs both dev servers. Contract and traps: [docs/release/windows-packaging.md](docs/release/windows-packaging.md).
