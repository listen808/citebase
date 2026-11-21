# 后端工程约定（Go）

> 适用：`app/`、`internal/`、`tools/`。前端规则见 [frontend-conventions.md](frontend-conventions.md)。
> 本文写"为什么"和"落到哪个文件"，命令清单在 `AGENTS.md`。

## 1. 分层与包布局

请求方向只有一条：**API → Service → Repository → Model**。

| 层 | 目录 | 该做什么 | 不该做什么 |
|---|---|---|---|
| API | `internal/api/<domain>/` | 解析参数、鉴权后的归属校验、调用 Service、组装响应 | 写 SQL、写业务分支 |
| Service | `internal/service/<domain>/` | 业务规则、事务编排、跨仓储组合 | 直接读 `*gin.Context`（拿用户身份靠参数） |
| Repository | `internal/repository/<domain>/` | GORM 查询、索引与迁移 | 决定权限、决定文案 |
| Model | `internal/model/<domain>/` | 表结构与纯函数校验 | 引用 repository / gin |

子包命名按领域对齐：`service/` 与 `repository/` 共用同一组 9 个域（`admin` `ai` `auth` `knowledge` `schema` `search` `social` `user` `workspace`）；`api/` 多出 `collab`、`common`、`websocket`；`model/` 没有 `auth/`（认证模型住在 `model/user/` 与 `model/admin/oidc_provider.go`）。

**根级文件只允许出现在 `repository/`**：`db.go`（连接、索引清单、迁移核对）+ 一组 `migrate_*.go` + `migration_registry.go`。`api/`、`service/`、`model/` 的根目录不放 `.go` 文件——新增能力就开子包，避免包根变成第二个"什么都往里塞"的目录。

新增一个模块时的落点：

1. model：`internal/model/<domain>/` + `TableName()` 返回 `aw_` 前缀表名；
2. repository：在对应的 `migrate_<domain>.go` 注册模型，索引写进 `db.go` 的 `allIndexes`；
3. service：业务逻辑，错误用 sentinel（见 §4）；
4. api：路由与处理器，权限登记进 `middleware/rbac.go` 的 `PrivilegeMap`（见 §5）；
5. 装配：需要跨模块协同时由 `app/server/ai_init.go` 或 `app/server/main.go` 显式注入，不用包级单例互相找。

## 2. 启动与装配

`app/server/` 只有三个文件：

- `main.go`：启动编排——Logger → Config → DB（GORM）→ Cache（memory/Redis）→ Search（Bleve，缺索引自动重建）→ Vector（Bolt HNSW / Chroma / Qdrant）→ AI 模块 → Text2SQL → Gin 监听 `:3001`；
- `ai_init.go`：AI 模块与 LLM provider 适配器的接线；
- `server.go`：Gin 引擎、路由注册、静态资源与前端托管。

装配原则是"**谁需要谁注入**"：跨层依赖由 `main.go` 在资源就绪后 `InstallXxx` 注入。典型例子是 `brand`——它零依赖（只 import `strings`），如果反过来 import `repository` 取配置就成环，所以由 `main.go` 在 DB 就绪后 `InstallBrandReader`。**不要为了少写一个参数，让底层包反向依赖上层。**

## 3. 响应封套与"说给人听的话"

统一封套（`internal/response/response.go`）：

```json
{ "code": 0, "message": "success", "data": {...},
  "message_key": "too_many_requests", "message_detail": "email" }
```

- `code`：业务码，成功为 `0`，失败通常与 HTTP 状态同值；
- `message`：中文原文，**永远保留**——curl、CLI、邮件模板、不认识键的旧部署都在读它；
- `message_key`：稳定键，前端按界面语言换成译文；`omitempty`，没有用户文案时不出现；
- `message_detail`：具体原因（哪个字段没填对、冲突的是哪个值），**不是句子**。句子要翻译，原因通常不用；把两者拼成一句，英文界面必然读到半中半英。

文案只有一张表：`internal/response/messages.go`（`MsgXxx` 常量 → 中文原文）。三条硬规矩：

1. **键是公开契约，只增不改不删**——改了旧前端读不到译文，会退回中文（不崩但语言错了）；
2. **不许插值**（`%s` / `{x}` 都不行）；要带参数就另开键，原因走 `message_detail`；
3. **前端语言包是生成物**：`web/scripts/server-messages.en.json` 由前端拥有，`web/src/i18n/locales/server-messages.ts` 由 `npm run gen:server-messages` 生成；新增话术顺序是"表里加键+中文 → en.json 加英文 → 跑生成 → 登记进 `serverMessageKeyBaseline`"。

对应门禁：`internal/response/messages_test.go`（出口逐字等于登记值）、`tools/repocheck/server_message_test.go`（键集基线只增、死键、插值、未迁移数只降）、`web/tests/server-messages.test.ts`（生成物过期、中英键集双向、英文侧不夹中文）。

分页响应用 `response.Paginated`，数据形状固定为 `{data,total,page,page_size}`——不要在某个接口里另发明一套。

## 4. 错误处理

- **共享 sentinel 在 `response`**：`ErrUnauthorized` / `ErrNotFound` / `ErrBadRequest` / `ErrInternal` 等，`ErrorWithErr` 用 `KeyForError` 把认识的 sentinel 接到 `message_key`；认不出的原样返回，不猜也不压成通用文案。
- **API 层**：
  - 正常失败 → `response.BadRequest/Unauthorized/Forbidden/Conflict/NotFound`；
  - 内部错误 → `ServerErrorSafe(c, err)`（完整错误进日志，客户端只拿到通用句）；
  - `Fail(c, msg, err.Error())` 会把 GORM/SQL 原文拼进响应体，**新代码禁用**；存量由 `internal/api/error_leak_test.go` 逐条清点，只减不增。
- **Service 层**：中文 `errors.New` 默认只进日志；**只有真的会出现在用户屏幕上的句子**才加哨兵、接键。其余经 `ServerErrorSafe` / `FailSafe` 落到日志即可——把 263 处内部错误全翻译一遍，是在给日志写译文。
- 数据库/驱动原文**不许**进 `message` / `message_detail`：它带着表名、列名、SQL 片段，对拿到 500 的人来说正好是做指纹识别的材料。

## 5. 鉴权与 RBAC 的默认语义

入口是 `internal/middleware/rbac.go` 的 `AutoRBAC()`，规则是"**先按特异性匹配，未定义再按前缀兜底**"：

1. `PrivilegeMap` 里显式登记的 `METHOD:pattern` 按特异性排序后取第一条命中（旧实现遍历 map 随机 `break`，会让同一路径在"要管理员"和"任意登录用户"之间漂移）；
2. 未登记且路径命中 `adminPathPrefixes` → 视为 `MinRole = 2`（管理员），照常校验身份；
3. 未登记且不在任何管理前缀 → **放行**，由 Service 层做空间级权限控制。

**"没登记 = 放行"是射程外的规则，射程内（管理前缀）恰恰相反：没登记 = 要管理员。** 新增管理面接口时：

- 在 `PrivilegeMap` 登记 `MinRole`（1=超管、2=管理员、3=普通登录用户）；
- 确认该接口族在 `sensitiveFamilies` 里；
- 跑 `go test ./internal/middleware/ -run RBAC`（`rbac_coverage_test.go` 会数"有路由没规则"）。

反例值得记住：`/api/v1/links` 整段在管理前缀内，点击计数接口要"任何登录用户可调用"，就必须显式登记 `MinRole: 3`——不登记等于要管理员，而症状是普通用户点击全 403、管理员看到的计数却是对的。

## 6. 日志

`internal/log` 是唯一出口：包 `log/slog` + `lumberjack` 轮转，级别由 `CITEBASE_LOG_LEVEL`（旧名 `AIWIKI_LOG_LEVEL`）控制，文件落在运行目录的 `data/logs/`。

- 业务代码用 `log.Infof/Warnf/Errorf`，不要 `fmt.Print` 或另起 `log` 实例；
- **日志里不出现口令、令牌、连接串、用户私密内容**；需要定位就记 ID；
- `ServerErrorSafe` / `FailSafe` 已经负责"内部细节进日志"，处理器里不要再手写一遍。

## 7. 配置与环境变量

- 主配置：`configs/config.yaml`（本机文件，**不入库**）；模板是 `configs/config_sample.yaml`（37 KB 注释齐全，改配置先读它）。
- 覆盖顺序：环境变量 > `config.yaml` > 代码默认值。
- 改名后的双读：`internal/utils/envname.Getenv("CITEBASE_X", "AIWIKI_X")`——新名优先、旧名兜底。**不要**只在一边写 `os.Getenv`，那会让老部署静默失效（viper 与 `os.Getenv` 对不认识的键都是一声不吭地丢弃）。
- `.env` 只用于 Docker Compose；`.env.example` 是入库模板，**永远不填真实凭据**。
- 有一个键不支持环境变量覆盖：`server.port` 只能改文件——发布文档里会反复用到这一点。

## 8. 数据库与迁移

- MySQL 8，全部表 `aw_` 前缀（`TableName()` 写死）。改前缀要同时动每个模型、`internal/repository/` 里的手写 SQL、`databases/*.sql`。
- **`databases/table.sql` 是生成物**：`go run ./tools/genddl` 从 `repository.RegisteredModels()` 与 `repository.AllIndexSpecs()` 产出，当前 96 张表，每条 `CREATE TABLE` 前带 `DROP TABLE`——**只能对空库执行**。
- 已有库靠启动时 AutoMigrate + `ensureMissingColumns` / `ensureAllIndexes` / `dropLegacyIndexes` 升级；`CITEBASE_SKIP_MIGRATE=true`（旧名 `AIWIKI_SKIP_MIGRATE`）可跳过，多实例部署时只让一个实例跑迁移。
- **"AutoMigrate 会自动补列"不是可靠假设**：SKIP_MIGRATE、跑的是旧二进制、该模型 migrate 报错只 Warn、DB 账号无 ALTER 权限——四条都会静默不补，而症状是功能降级不是启动失败。所以关键列要同时进 `missingColumnFallbacks()`（显式补列清单）与 `criticalColumns`（启动核对，**不受 SKIP_MIGRATE 影响**），一致性由 `internal/repository/migrate_columns_test.go` 钉住。
- `databases/data.sql` 的 `aw_config` 种子行与 `repository.DefaultConfigSeeds()` 逐行对齐；安装向导 `internal/api/auth/install.go` 直接引用后者，**不许抄第二份**（历史上抄出过一次，值、标签、类型、排序号四处全不同）。
- 备份/回灌引擎在 `internal/utils/dbdump`（纯 Go，不依赖 `mysqldump` / `mysql` 客户端）：导出一条一致快照事务，回灌逐条在同一连接执行。改它之前先读 `AGENTS.md` 的 Database 节，那里记了标识符转义、非 UTF-8 字节、`max_allowed_packet`、`DELIMITER` 拒绝这些边界。

加表 / 加列的完整步骤：模型 → `migrate_*.go` 注册 → `allIndexes` 索引 → 跑 `genddl` 重生成 `table.sql` → 关键列补 `missingColumnFallbacks` + `criticalColumns` → 跑迁移测试。

## 9. 并发与外部请求

- **后台 goroutine 一律走 `internal/utils/safego.Go(name, fn)`**：它带 recover 与命名，裸 `go func` 的数量是只降不升的棘轮（`internal/api/goroutine_guard_test.go`）。
- **出站 HTTP 一律走 `internal/utils/netguard`**：它先解析并校验目标 IP，再拨这个 IP（而不是重新解析域名），DNS rebinding 不能把校验绕过去。抓网页、调外部 API、探活都用它。
- 任务队列是 `internal/queue`（内存实现），跨实例不共享——需要多实例语义时先想清楚再改。
- 缓存是 `internal/cache`（memory / Redis 双模），键集中在 `keys.go`，不要在业务代码里拼字符串键。

## 10. 版本与品牌：两个"只此一处"

- **版本号**：`internal/version/version.go` 的 `Version` 是 `const`（当前 `2.0.0`），必须与 `web/package.json` 的 `version` 一致（去掉前导 `v`）。`-ldflags -X` **注入不进 const**，所以只注入 `Commit` 与 `BuildDate`；默认值刻意是 `dev` / 空串，"没注入"必须一眼看得出来。
- **品牌名**：`internal/brand/brand.go` 的 `DefaultName` 是代码默认名，`SiteName()` 按 `main_title` → `system_name` → 默认名三级取值（判据只有 `ResolveSiteName` 一处）；前端对应 `web/src/utils/core/app-info.ts` 的 `APP_NAME`。改名只改这两处。
- 存量库里的旧产品名由 `repository.normalizeLegacySiteNames()` 在启动时归一（清单在 `brand.LegacySiteNames()`），只认"整串等于旧产品名"，改动行数 > 0 时打 WARN 指路。**不要在读取端偷偷替换**——那样后台显示旧名、界面显示新名，等于再造一处不一致。

## 11. 测试

- 位置：与被测代码同包，文件 `*_test.go`；跨仓库的"规矩守卫"放 `tools/repocheck/`（它扫描路由表、模板、文档与品牌字面量）。
- 默认带 `-race`：`make test` 跑 `go test -race ./internal/... ./tools/repocheck/...`。
- 纯函数优先做成 hermetic 测试（不连库、不起服务），例如搜索日志的窗口归一化、偏好校验。数据库行为用真实 MySQL，别用 mock 假装 SQL 对了。
- 修 bug 先写能红的用例，再修——本仓最贵的 bug 类型是"门禁永远绿"（判据写错、退出码没置、选择器失效），不是代码写错。
- 定向套件：`make test-middleware`、`make test-response`；改路由/中间件跑 `go test ./internal/middleware/ -run RBAC`。

## 12. 提交前

```bash
go build ./...                       # 编译不过就不要提交
go vet ./internal/... ./app/... ./tools/...
gofmt -l .                           # 必须没有任何输出
go test ./internal/... ./tools/repocheck/... -count=1
go test ./internal/middleware/ -run RBAC -count=1   # 动了路由/中间件时
```

`make ci` 把 lint、test、build、前端构建与 `check-docs` 串在一起；**不要留一个编译不过的树**给下一个人。
