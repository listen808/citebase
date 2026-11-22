# Citebase

**Enterprise AI knowledge management — wiki, RAG and Text-to-SQL on one data model.**

English · [简体中文](README.zh-CN.md)

Every page in a knowledge space is indexed for full text, embedded into vectors and added to a
knowledge graph; a RAG pipeline, a Text-to-SQL engine and an agent tool layer serve retrieval and
answers over that index. Frontend is a Vue 3 SPA on an Apple-style token system — zero CSS files.

Go 1.26 + Gin + GORM (MySQL) · Vue 3.5 + TypeScript + Vite + Pinia.

> Formerly **AIWiki**, renamed 2026-10-07. The data model did not change: table prefix `aw_` and the
> `AIWIKI_*` env vars still work (fallback for `CITEBASE_*`).

## What it does

- **Knowledge** — nested spaces, tree nav, versions, cross-references, attachments, tags, approvals, trash
- **Collaboration** — Yjs CRDT over WebSocket, presence, automatic merge
- **Search** — Bleve + gse/jieba, pinyin index, vector recall, hybrid fusion, rerank
- **RAG** — ten stages: safety guard → semantic cache → decomposition → rewrite → hybrid retrieval
  (vector + keyword + HyDE + GraphRAG) → cross-encoder rerank → permission filter → generation →
  hallucination check → citation enrichment
- **Analyzer** — entity extraction, classification, duplicate/contradiction detection, quality scoring, graph building
- **Text-to-SQL** — CoT generation, execution feedback, multi-candidate, AST validation, policy, complexity routing
- **Providers** — OpenAI-compatible + Anthropic native, three tiers, circuit breaker, SSE, token quotas
- **MCP & agents** — stdio/SSE/streamable-HTTP client, 20+ tools, ReAct, multi-agent orchestration
- **Enterprise** — OIDC/SSO, Git sync, webhooks, email, SQL data sources
- **Observability** — traces, token usage, RAGAS evaluation, A/B experiments

The authoritative capability list is the source: routes in `internal/api/*/routes.go`, pages in `web/src/router/`.

## Interface

Screenshots, reconstructed diagrams and the value-by-value mapping to code:
[docs/product/interface.md](docs/product/interface.md).

## Requirements

| | |
|---|---|
| Go | 1.26+ |
| Node.js | `^20.19.0 \|\| >=22.12.0` |
| Database | MySQL 8.0 (utf8mb4); Redis optional |

## Quick start

```bash
# 1. Config
cp configs/config_sample.yaml configs/config.yaml   # edit database.dsn + AI provider

# 2. Database (table.sql contains DROP TABLE — empty database only)
mysql -u root -p -e "CREATE DATABASE citebase DEFAULT CHARSET utf8mb4 COLLATE utf8mb4_unicode_ci"
mysql -u root -p citebase < databases/table.sql
mysql -u root -p citebase < databases/data.sql

# 3. Backend :3001
go run ./app/server

# 4. Frontend :3000 (proxies /api/v1)
cd web && npm install && npm run dev
```

First visit runs the installer that creates the admin account and site config.

Docker: `cp .env.example .env` (set `MYSQL_ROOT_PASSWORD` / `MYSQL_PASSWORD`; compose refuses to start
without them) → `docker-compose up -d` → `docker-compose logs -f app`. For production set
`JWT_SECRET` (≥32 random chars) and an encrypt key, and fill `server.trusted_proxies` if you terminate
TLS behind a proxy — see [SECURITY.md](SECURITY.md). Keep `AIWIKI_ENCRYPT_KEY` in `.env` for the
Compose path (that is the name compose forwards); direct binaries accept either prefix. AI settings
use the **unprefixed** `AI_*` variables.

### Commands

GNU make is not installed everywhere (absent on a stock Windows box), so each target has a direct
equivalent:

| Goal | make | directly |
|---|---|---|
| Run backend | `make dev` | `go run ./app/server` |
| Build backend | `make build` | `go build -o citebase ./app/server` |
| Go tests | `make test` | `go test ./internal/... ./tools/repocheck/... -count=1` |
| Regenerate OpenAPI | `make swagger` | `swag init -g app/server/main.go -o docs/swagger` |
| Changelog | `make changelog` | `node tools/gen-changelog.mjs` |
| Docs links | `make check-docs` | `node tools/validate-doc-links.mjs` |

Frontend (`cd web`): `npm run lint:check`, `npm run type-check`, `npm run test`, `npm run check:style`
(seven design gates), `npm run ci` (all of it). Windows helpers in `scripts/`: `dev.bat`,
`package-windows.bat` (portable zip, plus the installer when Inno Setup is present).

## Layout

```
app/server/           entry and boot orchestration (main.go / ai_init.go / server.go)
internal/api/         HTTP layer          internal/service/    business logic
internal/repository/  data access         internal/model/      models (aw_ prefix)
internal/ai/          provider / rag / analyzer / embedding / mcp / …
internal/text2sql/    natural language → SQL
internal/collab/      Yjs collaboration server
web/                  Vue 3 SPA (Apple tokens, Ap* components)
configs/              config_sample.yaml  databases/  DDL + seed data
tools/                standalone CLIs     docs/       documentation index
```

Module map, layer rules and every convention: [AGENTS.md](AGENTS.md) and
[docs/engineering/codebase-map.md](docs/engineering/codebase-map.md).

## Known gaps (measured)

Stated with numbers rather than adjectives, because "polish needed" is not actionable.
Last re-run against HEAD: **2026-11-22** — `tools/repocheck/release_claims_test.go` recomputes these
and fails if the table drifts or goes stale, because a stale "measured" number is worse than none.

| Gap | Measured | Where it bites |
|---|---|---|
| **No UI translation** | 99 view files; `170` use `$t()` / `useI18n` (counting views **and** shared components — the shell appears on every page); 2,263 hard-coded Chinese characters remain in rendered positions (template text, attribute values, string literals — comments excluded). Done so far: login, AI chat, share, search, profile/settings, the workspace domain, the social pages, the shared shell, every route title, the whole admin area, all fourteen AI settings consoles, twenty-one top-level AI views, the Apple component kit and every shared component, plus every `\|\| '中文'` fallback slot (154 of them, moved into the `fallback:` namespace on 2026-10-15) | Visitor-facing and admin pages are translated; what is left is mostly script-side literals that need a data-value vs display-value triage, plus sentences split around an interpolation. A runtime pass over every route in the en-US shell (`npm run check:i18n-runtime`, needs both dev servers) now finds **0** code-authored Chinese strings — the budget is pinned at zero, so a new hardcoded string that renders on any of the 51 routes fails the gate; what remains in the 2,263 is script-side literals that only appear behind an interaction, plus user data, which is never translated |
| **Server-produced messages were untranslated** | Closed on 2026-10-12 for both shapes of the problem. Envelope: `245` sites wrote Chinese into `response.Xxx(c, "中文")` across `37` files (`123` distinct strings) — now `138` keyed messages in `internal/response/messages.go` carry `message_key` and **`0` sites are left**; the last 5 were concatenations, which now ride a separate `message_detail` field (a sentence is translated, a reason isn't). Verifier: `serverMessageUnmigratedBaseline = 0` in `tools/repocheck/server_message_test.go`. **Still open, and measured**: `43` sites across `20` files write Chinese straight into *data* fields (`gin.H{"message": "AI Provider 未配置"}`) — `message_key` can't help there, and that count is pinned by `serverMessageFieldBaseline` so it can only fall. Service-layer `errors.New` is no longer a blind spot for the client-visible subset: the `7` strings that actually reached the screen were given sentinels and keys; the rest stay log-only by design | Before this an English UI necessarily showed Chinese for every server error: `err.message` is the only pipe the shell has (error pages, toasts, the login error slot). English text lives in `web/scripts/server-messages.en.json`; `web/src/i18n/locales/server-messages.ts` is generated (`npm run gen:server-messages`) |
| **OpenAPI covers ~¼ of the API** | 125 `@Router` annotations over 513 route registrations; `docs/swagger` has 108 paths | `internal/api/ai` (197 registrations, 1 annotation) and `internal/api/schema` (37, 0) are effectively undocumented. Note: `docs/swagger` is generated — after adding an annotation run `make swagger`, otherwise the JSON lags the source |
| **CI defined, never run** | `.github/workflows/ci.yml` now carries the Go and web jobs (2026-11-22), but **no run has ever been triggered on the remote**; the pre-commit hook is still not in the tree | Until the first green run lands, enforcement is still the local `make ci` / `npm run ci` pair. A workflow file is not the same thing as a passing check |
| **Reverse proxy config is opt-in** | `server.trusted_proxies` defaults to `[]` | Behind a proxy every user counts as one IP, so the 300/min limit becomes site-wide |

Full launch checklist — each item with the command that reproduces it — is
[docs/release/launch-checklist.md](docs/release/launch-checklist.md).

## Security · Contributing · Docs · License

- **Security**: report privately, never a public issue — [SECURITY.md](SECURITY.md) covers the channel,
  required production config, existing defences and open weaknesses.
- **Contributing**: start with [CONTRIBUTING.md](CONTRIBUTING.md); the repo's habit is
  **reverse-verifying every new check** (inject a violation, prove it goes red, undo by editing).
- **Docs**: everything beyond the four root cards is indexed in [docs/README.md](docs/README.md) —
  that index is the only one.
- **License**: MIT — see [LICENSE](LICENSE). Third-party components keep their own licenses, listed in
  [NOTICE.md](NOTICE.md) (generated by `node tools/gen-notice.mjs`, pinned by `TestNoticeCoversBuildList`).
