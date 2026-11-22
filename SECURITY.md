# Security Policy

Citebase 是自带账号体系、SSO、Webhook、Git 同步、SQL 数据源与 LLM 凭据的服务端程序，
所以"安全"在这里不是加密细节，而是**谁能通过哪条路由拿到谁的什么数据**。
本页说明：怎么报告漏洞、上线前必须改的配置、代码里已存在的防线，以及**已知还没解决的问题**。

## Reporting a Vulnerability

**请用私有渠道报告，不要公开发帖**（issue、群、社媒都不行）——在修复版发布之前，
公开细节等于给现网部署发攻击脚本。

| 渠道 | 状态 | 用法 |
| --- | --- | --- |
| GitHub 私有漏洞报告（Security → Report a vulnerability） | 需在仓库设置里打开（见下方） | 首选。走草稿对话，修复发布前对其他访客不可见 |
| 维护者邮件 | **尚未公布** | 见下方说明 |
| 公开 issue / PR 描述 | ❌ 不要用 | 即使是"我注意到一个可能的越权"也算公开披露 |

本仓库的发布主机定为 GitHub（`github.com/listen808/citebase`；2026-10-07 把 module 路径与站内链接都迁了过去，
但**代码尚未推送** —— 2026-10-10 实测该仓库 `size=0`、默认分支 `main`，而 Gitee 上的两个仓库仍是公开可读的）。
私有漏洞报告需要在仓库 Settings → Advanced → **Pull Requests → Private vulnerability reporting** 里打开；
打开之前，本表第一行的地址会 404 —— 那是"仓库刚建好还没开这个功能"的状态，不是渠道失效。
本文件**没有写邮件地址**，因为此刻没有一个真实可公布的地址；宁可少一个渠道，也不放一个假地址。

打开之后，第一行就是唯一入口。在此之前请走第一行的地址（若已可用），否则开一条
**只写"我发现一个疑似越权，想私下报告"的 issue 并在此停住**，等维护者把对话转到私有渠道 ——
不要在 issue 正文里写触发条件、路由或数据。

报告里请尽量带上，缺项也能收，但齐全会显著加快处理：

1. 受影响版本与提交（`git rev-parse --short HEAD`）
2. 触发条件：认证态（匿名 / 普通用户 / 管理员 / 超管）、所需角色、是否要已存在的资源 id
3. 最小复现：一条 `curl` 或"点哪几步"，包含 HTTP 方法与路径
4. 预期 vs 实际（"应该 403，实际返回了别人的邮箱"）
5. 影响面：读越权 / 写越权 / 提权 / 泄漏密钥 / 拒绝服务 / 服务端主动外连

处理约定：48 小时内确认收到，7 天内给出结论与修复计划；确认成立的漏洞会随下一次
发布一起出 advisory，致谢报告者（可匿名）。

## 不在范围内

- 需要已获得管理员权限才能做的事（管理面本身能读全部数据是设计如此）
- 只对物理接触服务器/数据库有效的问题
- 第三方 LLM 供应商、被检索文档内容里的社会工程诱导（提示注入的**产品级**风险除外）
- 依赖传递漏洞中上游已不再维护的组件，除非它在本仓库的默认部署路径上可达

## Supported Versions

只维护最新发布分支，无 LTS（`master`）。

```bash
cd web && node -p "require('./package.json').version"   # 前端版本
grep -m1 "^module " ../go.mod                            # 模块路径与 Go 基线
```

## 上线前必须改的配置

这几条是默认值与生产要求不一致造成的，属于部署缺陷而不是代码漏洞——**默认配置直接上公网是不安全的**：

| 项 | 默认 | 生产要求 | 不改的后果 | 校验 |
| --- | --- | --- | --- | --- |
| `JWT_SECRET` / `AIWIKI_JWT_SECRET` | 空 | 随机 ≥32 字符（`openssl rand -hex 32`） | 长度 <32 时启动即失败（已有守卫），留空则退到 `data/.jwt_secret` 本机生成 | `grep -n "too weak" internal/middleware/jwt/jwt.go` |
| `AIWIKI_ENCRYPT_KEY` | 空 | 固定 32 字节密钥并纳入密钥管理（Docker 部署经 `.env` → compose 透传） | 退到 `data/.encrypt_key`；换机器/重装后**数据源口令解不开**，且被当作明文回灌给 DSN。多副本更要命：每个副本各一把，A 加密的 B 解不开，表现是"目标库认证失败"而不是"密钥换了" | `grep -n "AIWIKI_ENCRYPT_KEY" docker-compose.yml .env.example` |
| `server.trusted_proxies` | `[]` | 填真实代理网段 | gin 不读 `X-Forwarded-For` → 全站用户都算代理那一个 IP，限流变成共享额度；反过来填成 `0.0.0.0/0` 等于允许伪造来源 IP 绕过限流 | `grep -n "trusted_proxies" configs/config_sample.yaml` |
| `swagger_open` | `false` | 保持 `false` | `/swagger` 免鉴权枚举全部接口与参数 | `grep -n "swagger_open" configs/config_sample.yaml` |
| `server.allowed_origins` | 本地开发地址 | 只留真实部署域名 | 内网地址写进去等于把拓扑泄露给被跟踪的配置文件 | `grep -n "allowed_origins" -A5 configs/config_sample.yaml` |
| `configs/config.yaml`、`.env`、`data/` | gitignored | 不要提交、不要打进镜像 | 真实 DSN 与密钥进版本库 | `git check-ignore -v configs/config.yaml .env data/` |
| `databases/table.sql` | 含 `DROP TABLE` | **仅用于空库** | 对着有数据的库执行 = 清库 | `grep -c "^DROP TABLE" databases/table.sql` |
| 超管初始口令 | 首启随机生成 16 位 | 登录后立刻改，并**清理或限制首次启动那几行日志的访问权** | 口令是随机的（不是 `123456`），但以明文打在启动日志里一次——能读日志的人就能登超管 | `grep -n "Initial password" internal/repository/db.go` |

## 代码里已有的防线

每一条都能指到文件和一台常驻测试，改坏会红：

- **RBAC 默认值取决于路径在不在管理前缀里**。`internal/middleware/rbac.go` 的 `AutoRBAC` 先查
  `PrivilegeMap`，未命中但路径落在 `adminPathPrefixes` 则按管理员档拦，两者都不落则放行。
  "漏登记一条规则"在射程内是变严、在射程外是洞开，所以这条由
  `internal/middleware/rbac_coverage_test.go` 三条测试守住：敏感前缀族必须有显式规则、
  12 条历史提权路由逐条钉住、兜底前缀表里不许留已经没有路由的条目。
- **出站请求走统一护栏**。`internal/utils/netguard` 的 `DialContext` 在**已校验过的那个 IP** 上建连，
  因此"先解析校验、再由库自己重新解析"的 DNS rebinding TOCTOU 不成立；`CheckURL` 拒绝回环、
  链路本地、私网与元数据地址。Webhook、Git 同步、网页抓取、MCP 都从这里出网。
- **Text-to-SQL 只允许只读单语句**。`internal/ai/rag/sql_ast_validator.go` 做 AST 校验（多语句、
  行注释吞掉后追加的 `LIMIT`、字符串字面量与反引号标识符各由预编译正则处理）；
  `internal/text2sql/trust/policy.go` 的 `DefaultDenyPatterns` 会**并入**管理员自定义规则，
  清空配置字段不等于关闭防护。
- **令牌解析锁算法**。`ParseRefreshToken` 用 `jwt.WithValidMethods([]string{"HS256"})`，
  拒绝 `alg=none` 与密钥混淆类伪造，并要求 `exp` 存在、`userID > 0`。
- **凭据静态加密**。`internal/utils/crypto.EncryptSecret`（AES-GCM，密钥由配置或环境变量派生）
  用于数据源口令与 SMTP 口令；`LooksEncrypted` 防止把已加密值再套一层。
- **限流三档 + 可退避**。`internal/middleware/ratelimit.go`：通用 300/分钟/IP，
  口令猜测面（登录、超管入口、安装向导）10/分钟/IP/路径，刷新 30/分钟/IP/路径。
  每个 429 带 `Retry-After`。档位数字由 `TestLimiterTiers` 钉住。
- **分享页元数据不越隐私边界**。`internal/api/workspace/share_page.go` 只在
  空间开放分享、无访问口令、文档未删除时才把文档标题写进 og 标签，其余一律站点级标签 + `noindex`；
  所有取值过 `html.EscapeString`（标题是用户起的）。带口令的分享在解锁前只回 `has_password`，
  不再返回标题、空间名、创建者与祖先路径。
- **错误不泄漏内部信息**。响应层 `FailSafe` 对外只给一句人话，原始错误进日志
  （`internal/api/error_leak_test.go` 是按这一条建的门禁）。
- **后台任务必须有 recover**。`internal/utils/safego` 的 `Go(name, fn)` 是唯一入口，
  `internal/api/goroutine_guard_test.go` 拦裸 `go func`（基线 14 处，只降不升）。

## 已知问题与限制

列在这里而不是藏在代码注释里，因为这些是**当前真实存在的弱点**：

1. **历史明文口令仍可被接受**。`DecryptSecretOrPlain` 在 AES-GCM 验签失败时原样返回存储值——
   这是为了让修复前明文入库的老行继续可用（无需迁移脚本）。代价：改了 `AIWIKI_ENCRYPT_KEY`
   之后，真·密文解不开也会被当明文回灌给 DSN，看起来像"口令错了"而不是"密钥换了"。
   老行被重新保存一次即自动变密文；一次性迁移尚未提供。
2. **`internal/middleware/rbac.go` 的放行分支仍是设计而非白名单**。新增管理面路由若不登记规则且
   路径不在管理前缀里，仍然是放行。覆盖测试按"敏感前缀族"检查，**族以外的新接口不会红**——
   判据只能覆盖已知的那几族，新领域要手工补一条 `sensitiveFamilies`。
3. **协同编辑 WebSocket 的授权粒度是"空间成员"，不是"文档权限"**。`internal/collab/server.go`
   校验 token 有效且未登出、账号未被禁用/删除，然后要求"公开空间 / 超管 / 该空间成员"三者之一。
   缺的两层：**同一空间内不区分只读与可写**（任何成员连上就能改，与 HTTP 侧的文档编辑权限不是同一套判据），
   且接入只按 `documentId` 路径参数走，没有文档级可见性检查——空间成员可以连该空间内**任意**文档的房间。
4. **WebSocket 鉴权接受 `?token=` 查询参数**。查询串会进反向代理访问日志、浏览器历史与 Referer，
   比 `Authorization` 头更容易意外泄漏。头是首选，查询参数仅为兼容非浏览器客户端而保留。
5. **提示注入未做端到端防御**。被检索文档的内容会进 LLM 上下文，Agent 工具链里含写操作
   （创建/移动文档、执行 SQL）。检索结果侧有权限过滤，但"文档正文里写一段指令让模型去调工具"
   这类路径目前依赖模型自身，没有独立的意图确认层。
6. **无审计导出与集中告警**。越权尝试、登录失败、Webhook 外连只进进程日志，
   没有 SIEM 出口；也就是说"有人正在试"这件事现在只能靠人翻日志发现。
7. **CI 尚未真实跑过第一次**（2026-11-22 已把 workflow 入库：`.github/workflows/ci.yml` 跑 Go 侧
   gofmt/build/vet/test 与前端 `npm run ci`，另起一个 job 校验文档链接），
   但远端从未触发过运行，也没有绿色记录；本地钩子文件仍不在库内。所以上述常驻测试目前的唯一执行处
   仍是手工 `make ci` / `npm run ci`。
8. **另一条历史血统上的凭据一律按已泄漏处理。** 本仓库有两条互不相干的历史。`master` 从 2026-09-24 重写，
   实测它的**全部** 175 个提交（不只是 HEAD 的跟踪内容）只含 `YOUR_PASSWORD`、`${MYSQL_PASSWORD}`
   这类占位与测试夹具，没有真凭据。6-7 月的另一条历史曾以三个 ref 挂在 Gitee 上，
   里面提交过一个真实的 JWT secret 文件（`internal/ai/schema/data/.jwt_secret`）与两条写在注释里的
   数据库 `root` 口令（指向一台内网 MySQL）。
   2026-10-10 实测：Gitee 上那两个仓库现在都只有 `master` 与 `v2.0.0`，脏 ref 已不在远端 ——
   但它们在**公开可读**的仓库上存在过约三个月，服务端对象被回收之前、以及这期间任何一次
   fork/克隆里，这些值仍可能被拿到。所以正确处置是**轮换口令与密钥**，不是"删掉 ref 就算完"。
   提交号、ref 名与端口已从本文档移除：那是取证线索，不是发布材料该递给读者的东西
   （要查就去 Gitee 仓库的事件日志，只有维护者能看到）。
   常驻门禁两道：`tools/repocheck/secret_scan_test.go` 扫 HEAD 跟踪内容，
   `tools/repocheck/history_secret_test.go` 扫 master 的全部提交历史。

## 加密与凭据处理约定

- 密钥一律来自配置或环境变量，代码里**没有**内置默认密钥；落盘的只有本机首启生成的
  `data/.jwt_secret` 与 `data/.encrypt_key`（`data/` 已 gitignored，`.dockerignore` 同步）。
- 口令不以 `-p<password>` 形式出现在子进程命令行（数据库导出/回灌走纯 Go 实现，
  见 `internal/utils/dbdump`）。
- 前端不会拿到密文再解密；密文只在服务端存在。
