# Citebase

[English](README.md) · 简体中文

> 企业级 AI 知识管理与智能问答平台。原以 **AIWiki** 之名开发，2026-10-07 改名：数据模型未变，表前缀仍是 `aw_`，`AIWIKI_*` 环境变量仍可用（作为 `CITEBASE_*` 的兜底）。

知识空间里的每一篇文档自动进入全文索引、向量索引与知识图谱，再由 RAG 管线、Text-to-SQL 和 Agent 工具统一提供检索与问答。前端是 Apple 风格设计系统（Design Token + 内联样式，零 CSS 文件）。

## 核心特性

- **知识管理** —— 多级空间、树形目录、版本与交叉引用、附件、标签、审批流、回收站
- **实时协同** —— Yjs CRDT + WebSocket，在线状态与冲突自动合并
- **搜索** —— Bleve 中文分词（gse / jieba）+ 拼音索引 + 向量召回 + 混合融合与重排
- **RAG** —— 十步管线：安全护栏 → 语义缓存 → 问题拆解 → 查询改写 → 混合检索（向量 + 关键词 + HyDE + GraphRAG）→ 交叉编码重排 → 权限过滤 → 生成 → 幻觉检测 → 引用补全
- **知识分析器** —— 实体抽取与消歧、分类、重复/矛盾检测、质量评估、图谱构建，支持增量与定时
- **Text-to-SQL** —— CoT 生成、执行反馈修正、多候选、AST 校验、安全策略、复杂度分级路由
- **多模型 Provider** —— OpenAI 兼容 + Anthropic 原生，三档分层，熔断、退避、SSE、Token 配额
- **MCP 与 Agent** —— stdio / SSE / streamable HTTP，20+ 工具，ReAct 与多 Agent 编排
- **企业集成与可观测** —— OIDC / SSO、Git 同步、Webhook、邮件、SQL 数据源；链路追踪、Token 用量、RAGAS、A/B

能力清单以源码为准：后端路由在 `internal/api/*/routes.go`，前端页面在 `web/src/router/`。

## 界面

真截图、复刻示意图与逐项取值对照（每个色值、圆角、宽度指到唯一实现处）：
[docs/product/interface.md](docs/product/interface.md)。

## 技术栈与环境

| 层 | 选型 |
|---|---|
| 后端 | Go 1.26 · Gin · GORM |
| 前端 | Vue 3.5 · TypeScript 6 · Vite 8 · Pinia 3 · Monaco |
| 数据库 | MySQL 8.0（utf8mb4）；Redis 可选 |
| 检索 | Bleve v2（gse / jieba）· Bolt HNSW / Chroma / Qdrant |
| 协同 | Yjs CRDT + WebSocket |

Node.js `^20.19.0 || >=22.12.0`。

## 快速开始

```bash
# 1. 配置
cp configs/config_sample.yaml configs/config.yaml   # 改 database.dsn 与 AI Provider

# 2. 空库导入（table.sql 含 DROP TABLE）
mysql -u root -p -e "CREATE DATABASE citebase DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci"
mysql -u root -p citebase < databases/table.sql
mysql -u root -p citebase < databases/data.sql

# 3. 后端 :3001
go run ./app/server

# 4. 前端 :3000（已代理 /api/v1）
cd web && npm install && npm run dev
```

首次访问进入安装向导，创建管理员与站点配置。

Docker：`cp .env.example .env`（`MYSQL_ROOT_PASSWORD` / `MYSQL_PASSWORD` 必填，缺失则 compose 拒绝启动）→ `docker-compose up -d`。生产环境必须设 `JWT_SECRET`（≥32 字符随机串）与加密密钥，反向代理后必须填 `server.trusted_proxies`——详见 [SECURITY.md](SECURITY.md)。

## 常用命令

```bash
make dev / make build-all / make test / make lint / make ci
make swagger          # 重新生成 OpenAPI，勿手改 docs/swagger
make check-docs       # markdown 相对链接必须落地
node tools/gen-changelog.mjs   # 生成 CHANGELOG，勿手编
cd web && npm run ci  # lint → type-check → check:style → vitest → build
```

Windows 辅助脚本在 `scripts/`：`dev.bat`（双服务）、`package-windows.bat`（免安装 zip，装了 Inno Setup 再出 `Citebase-Setup-<version>.exe`；详见 [docs/release/windows-packaging.md](docs/release/windows-packaging.md)）。

## 项目结构

```
app/server/           入口与启动编排（main.go / ai_init.go / server.go）
internal/api/         HTTP 层        internal/service/   业务逻辑
internal/repository/  数据访问        internal/model/     模型（aw_ 前缀）
internal/ai/          provider / rag / analyzer / embedding / mcp …
internal/text2sql/    自然语言转 SQL
internal/collab/      Yjs 协同服务端
web/                  Vue 3 SPA（Apple 令牌，Ap* 组件）
configs/ databases/ docs/ scripts/ tools/
```

分层契约与模块地图：[AGENTS.md](AGENTS.md) · [docs/engineering/codebase-map.md](docs/engineering/codebase-map.md)。

## 配置要点

- 全部配置项与注释以 [`configs/config_sample.yaml`](configs/config_sample.yaml) 为准（比任何文档都新）。
- 支持环境变量覆盖的键：`CITEBASE_DB_DSN` / `JWT_SECRET` / `ENCRYPT_KEY` / `REDIS_PASSWORD` / `ENV` / `LOG_LEVEL` / `LOG_FORMAT` / `DATA_DIR` / `SKIP_MIGRATE` / `SWAGGER` / `TRUSTED_PROXIES` / `JIEBA_DICT`——同时认改名前的 `AIWIKI_*`，新名优先、旧名兜底，老 `.env` 不用动。
- AI 走**无前缀**变量：`AI_ENABLED` / `AI_PROVIDER` / `AI_BASE_URL` / `AI_API_KEY` / `AI_MODEL`。
- Docker 路径的 `.env` 请写 `AIWIKI_ENCRYPT_KEY`（compose 转发的就是旧名，为不砸老部署；见 [launch-checklist](docs/release/launch-checklist.md) 1.1）；直接跑二进制两种前缀都认。留空则每个副本各自生成密钥，跨副本密文解不开。
- `server.trusted_proxies` 留空时 gin 不读 `X-Forwarded-For`，全站算一个 IP，300 次/分钟变成整站共享。
- `config.yaml` 与 `.env` 已 gitignore；模板有门禁：`config_sample_load_test.go` 校验可被 viper 解析且键与结构体一一对应。

## 测试与门禁

```bash
go test ./internal/... ./tools/repocheck/... -count=1 -timeout 120s
cd web && npm run test / npm run e2e
```

托管 CI 已在 2026-11-22 入库（`.github/workflows/ci.yml`：Go 的 gofmt/build/vet/test/doc-links 与 web 的 lint/type-check/style/vitest/build，推送即触发；详见 [CI 落地与钩子取舍](docs/decision-log/2026-11-22-ci-and-hooks.md)）。提交钩子仍不在树里，所以本地提交在推送前未经验证；本地等价的执行点是 `make ci` 与 `cd web && npm run ci`。规则与踩坑：[static-gates.md](docs/gates/static-gates.md) · [runtime-inspectors.md](docs/gates/runtime-inspectors.md)。判据类改动要反向验证：注入违例确认变红，再编辑撤回。

## 文档与许可

门面四卡在根目录，其余全部由 [docs/README.md](docs/README.md) 索引——那份是唯一一份，别在这里再抄清单。

| 文档 | 内容 |
|---|---|
| [AGENTS.md](AGENTS.md) | 工程约定：命令、分层、红线、指针 |
| [CONTRIBUTING.md](CONTRIBUTING.md) | 改哪类跑哪道门禁、反向验证、可上手的欠账 |
| [SECURITY.md](SECURITY.md) | 报告渠道、上线必改配置、已有防线与未解问题 |
| [CHANGELOG.md](CHANGELOG.md) | 更新日志（生成物） |

MIT License，详见 [LICENSE](LICENSE)；第三方组件许可见 [NOTICE.md](NOTICE.md)。
