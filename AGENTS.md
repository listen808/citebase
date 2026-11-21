# AGENTS.md — Citebase

企业 AI 知识管理平台：Go + Gin 后端，Vue 3 + Vite 前端，Apple 风格设计系统。
**本文件只留命令、红线与指针；模块细节在 [docs/engineering/codebase-map.md](docs/engineering/codebase-map.md)，规则正文在 [docs/](docs/README.md)。**

## 命令

```bash
# 后端（仓库根）
go run ./app/server                 # 开发，:3001
go build -o citebase.exe ./app/server
go test ./internal/... ./tools/repocheck/... -count=1 -timeout 120s

# 前端（web/）
npm run dev          # :3000，代理 /api/v1
npm run lint:check   # ESLint，仅 error 级
npm run type-check
npm run test         # vitest
npm run check:style  # 七道样式门禁（提交钩子只跑第一道）
npm run ci           # lint + 类型 + 单测 + 构建
npm run e2e          # Playwright

# 其他
make check-docs      # 所有 markdown 相对链接必须落地
make swagger         # swag init -o docs/swagger（生成物，勿手改）
make changelog       # 从 git log 生成 CHANGELOG.md，勿手编
node tools/gen-notice.mjs            # NOTICE.md 由构建清单生成
scripts/dev.bat / scripts/package-windows.bat   # Windows 双服务 / 打包
```

单道样式门禁：`check:theme` / `check:design` / `check:contrast` / `check:glass` / `check:layout`；运行时巡检 `check:glass:runtime`、`check:theme-rounds`、`check:i18n-runtime` 需要 `SNAP_USER` / `SNAP_PASS` 与两个服务在跑。判据见 [docs/gates/](docs/gates/static-gates.md)。

## 架构（一句话版）

`app/server/` 是入口（main / ai_init / server 三文件）；后端四层 `api → service → repository → model`，每层按 9 个域分子包；AI 能力集中在 `internal/ai/`（19 个子包），Text2SQL 独立在 `internal/text2sql/`；前端是 `web/` 下的单页应用，零 CSS 文件、全内联 token。
启动顺序、包清单、文件数口径、数据库生成链：见 codebase-map。

**红线：**

1. 产品名只有一个落点 `internal/brand/brand.go`（改名连同 `web/src/utils/core/app-info.ts` 的 `APP_NAME`），前端配置键与后端清单必须一致。
2. 面向用户的文案只走 `internal/response/messages.go` 的稳定键；英文进 `web/scripts/server-messages.en.json` 后跑 `npm run gen:server-messages`，新键要登记基线。
3. `databases/table.sql` 由 `go run ./tools/genddl` 生成，禁止手改；`data.sql` 的种子与 `DefaultConfigSeeds()` 逐行对齐。
4. 一个概念一处实现（清单：[one-implementation.md](docs/engineering/one-implementation.md)）；新样式必须走 token，禁止新增 CSS 文件。
5. 任何代码改动后必须 `go build ./...` 与前端门禁通过再离开；树不允许留在不能构建的状态。
6. 未经明确要求，不要启动后台服务（后端、dev server、Docker）。

## 关键文件

| 文件 | 作用 |
|---|---|
| `app/server/main.go` · `ai_init.go` · `server.go` | 启动编排 / AI 装配 / 路由与静态资源 |
| `internal/ai/ai.go` · `facade/ai_facade.go` | AI 装配与统一门面 |
| `internal/ai/rag/retriever.go` | 检索管线（混合检索 + 重排 + HyDE） |
| `internal/text2sql/generator.go` | Text-to-SQL 核心（按域拆分 `generator_*.go`） |
| `internal/repository/db.go` | 连接、迁移注册、关键列核对 |
| `internal/response/messages.go` | 后端话术唯一表 |
| `web/src/views/ai/AIChatView.vue` · `AISettingsLayout.vue` | AI 对话页 / 设置页骨架 |
| `web/src/styles/theme/personalization.ts` · `accents.ts` | 个性化档位真相源 |
| `configs/config_sample.yaml` | 配置模板（复制为 config.yaml） |

## 开发与测试

1. 复制 `configs/config_sample.yaml` 为 `configs/config.yaml`，改 `database.dsn` 与 AI provider。
2. 空库导入 `databases/table.sql` + `data.sql`（含 DROP TABLE，仅限空库）。
3. 后端 `go run ./app/server`；前端 `cd web && npm install && npm run dev`。
4. Docker：`cp .env.example .env`（设 `MYSQL_ROOT_PASSWORD` / `MYSQL_PASSWORD`）后 `docker-compose up -d`。

前端单测跑在 `happy-dom`，`globals: false`，测试文件显式 `import { describe, it, expect } from 'vitest'`，与被测源码同目录或放 `tests/`。

## 提交

Conventional Commits（`feat:` / `fix:` / `docs:` / `style:` / `refactor:` / `test:` / `chore:` + 可选 scope），主题 ≤72 字符，写清改了什么而不是"优化"。提交前跑 `go build ./app/server` + `go test ./internal/...` + 前端 `lint:check` / `type-check` / `test`。CI 从未在本远端触发（见 Gotchas），本地门禁就是唯一执行点。

## 约定指针

- 后端规则 → [backend-conventions.md](docs/engineering/backend-conventions.md)
- 前端规则 → [frontend-conventions.md](docs/engineering/frontend-conventions.md)
- 收口清单 → [one-implementation.md](docs/engineering/one-implementation.md)
- 主题与个性化 → [theme-system.md](docs/engineering/theme-system.md) · [personalization.md](docs/design/personalization.md)
- 静态门禁 / 运行时巡检 → [static-gates.md](docs/gates/static-gates.md) · [runtime-inspectors.md](docs/gates/runtime-inspectors.md)
- 决策与事故 → [docs/decision-log/](docs/decision-log/)
- 上线与打包 → [launch-checklist.md](docs/release/launch-checklist.md) · [windows-packaging.md](docs/release/windows-packaging.md)

## Gotchas

- 后端 `:3001`、前端 `:3000`；Vite 代理 `/api/v1`、`/images`、`/attachments`，协作 WebSocket 走 `/api/v1/collab/ws`。
- 表前缀 `aw_` 不可随意改：模型、仓库层手写 SQL、`databases/*.sql` 要一起动。
- 2026-10-07 由 AIWiki 改名；表前缀 `aw_`、双读环境变量、历史文档刻意未改（[决策记录](docs/decision-log/2026-10-07-rename-scope.md)）。
- 文档布局只在 [docs/README.md](docs/README.md) 维护；`web/src/styles/README.md` 是唯一随目录走的例外。
- Windows 下用 PowerShell，文件存 UTF-8 + LF；仓库 blob 已是全 LF，别把编码问题当"修 CRLF"（[记录](docs/decision-log/2026-10-06-line-endings.md)）。
- `configs/config.yaml` 与 `.env` 不入库；`.env.example` 只放键名与说明，绝不填真实凭据。
- AI 设置页 31 个子页：`grep -c "^      { key:" web/src/views/ai/AISettingsLayout.vue`。
- CI 从未执行：`.github/` 只有 issue 模板，提交钩子也不在树里；实际执行点是 `make ci` / `npm run ci`。
- 发布链只有 `scripts/package-windows.bat`：便携 zip，装了 Inno Setup 时再出 `Citebase-Setup-<version>.exe`。
