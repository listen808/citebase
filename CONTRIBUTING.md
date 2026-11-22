# Contributing to Citebase

English · [简体中文 section](#中文说明)

This repo has an unusual amount of machine-enforced convention — 42 static design gates, 14
browser-based runtime inspectors, Go gates that scan the route table, and a habit of
reverse-verifying every check. That machinery is here so review can be about behaviour rather than
spacing. This file says what to run and which gate will catch what.

## Before you write code

```bash
cp configs/config_sample.yaml configs/config.yaml   # edit database.dsn
# Backend (Go 1.26+, MySQL 8)
go run ./app/server                                 # :3001
# Frontend (Node ^20.19 || >=22.12)
cd web && npm install && npm run dev                # :3000, proxies /api/v1
```

Read [AGENTS.md](AGENTS.md) before a non-trivial change. It is the engineering contract: commands,
layer and sub-package rules, key files, and pointers to where the detail lives — Go rules and the
RBAC fall-through semantics in [`docs/engineering/backend-conventions.md`](docs/engineering/backend-conventions.md),
frontend rules in [`docs/engineering/frontend-conventions.md`](docs/engineering/frontend-conventions.md),
the gate fleet in [`docs/gates/`](docs/gates), and the dated "why" in
[`docs/decision-log/`](docs/decision-log). `README.md` is a tour; `AGENTS.md` is the index of law.

## What has to be green

Run the gate that matches what you touched — CI runs a subset, and nothing runs at commit time.

| You changed | Run |
|---|---|
| Any Go | `go build ./... && go vet ./internal/... ./app/... ./tools/...` · `go test ./internal/... ./tools/repocheck/... -count=1` · `gofmt -l .` (must print nothing) |
| API routes or middleware | `go test ./internal/middleware/ -run RBAC -count=1` (the route-coverage gate) |
| Any frontend file | `cd web && npm run ci` — lint → type-check → vitest → `check:style` → build |
| Styles, tokens, themes | `cd web && npm run check:style` (all five gates; the hook runs only the first) |
| Layout or components | `cd web && npm run check:layout`, and if a size/density prop was renamed, `check:gates:rv` |
| Anything a browser can see | `cd web && npm run check:geom` and/or `snap` — needs the dev server on :3000 and backend on :3001 |
| Docs | `node tools/validate-doc-links.mjs` **and** `go test ./tools/repocheck/ -count=1` — docs now have real gates: `release_claims_test.go` recomputes every number in README's "Known gaps" table, `doc_redaction_test.go` forbids leak coordinates (commit hashes next to leak narrative, placeholder hosts with live ports), `untracked_ref_test.go` catches references to artifacts that `.gitignore` keeps out of the repo. Register the file in `docs/README.md`; nothing checks that a new doc is indexed |
| `.github/` or `SECURITY.md` | `go test ./tools/repocheck/ -count=1` |
| Handlers or OpenAPI annotations | `make swagger` (CI re-runs it and diffs `docs/swagger`) |

CI (`.github/workflows/ci.yml`) runs `gofmt`/`go build`/`go vet`/`go test`, the doc-link check, and
`npm run ci` for the web side. It does **not** run the four extra style gates, doctrine checks, or any
browser inspector — those stay manual. There is no pre-commit hook in this tree: the husky dependency and
`prepare` script are in `web/package.json`, but the `.husky/` directory was never committed, so nothing
runs when you commit. Treat "committed" as "unverified" and run the table above before you push.

## The one rule that matters most: prove the check can fail

When you add or tighten a gate, inject the violation it is meant to catch, confirm it goes red, then
undo the injection **by editing the lines back**.

> A gate with a broken criterion is permanently green, and "permanently green" prints exactly like
> "everything passes". This has happened here repeatedly: a dead selector in `[data-ap=segmented]`,
> a `\b` that counted hyphenated names as matches, a `Set` that didn't survive `page.evaluate`,
> an exit code that was computed but never set. Each of those reported zero violations for weeks.

Useful commands for the verification habit:

```bash
grep -c "expect:" web/scripts/rv-design-gates.mjs   # the reverse-verification suite: 47 injected violations
cd web && npm run check:gates:rv                    # every static design gate must actually catch its probe
cd web && npm run check:geom:selftest               # each runtime geometry rule must go red on a real host
```

## Conventions that bite (details in docs)

前端：零 CSS 文件（内联 `:style` + token）；`ApButton` 用 `variant` 不覆写颜色；一个概念一处实现（进度条 `ApProgress`、分段 `ApSegmentedControl`、弹窗 `ApModal`、面包屑 `ApBreadcrumb`）；页面根 `flex: 1; minHeight: 0`，禁 `100vh`，且只有一个页面级滚动容器。完整规则见
[`docs/engineering/frontend-conventions.md`](docs/engineering/frontend-conventions.md)。

后端：表前缀 `aw_`；`table.sql` 由 genddl 生成；新表要动 model → `migrate_*.go` → `allIndexes` 三处；管理端路由要登记 `PrivilegeMap`（管理前缀外"未登记"等于放行）；后台 goroutine 走 `safego`；出站 HTTP 走 `netguard`。完整规则见
[`docs/engineering/backend-conventions.md`](docs/engineering/backend-conventions.md)。

## Commits and pull requests

Conventional Commits: `fix(rag): …`, `feat(web): …`, ~72 characters, describing the behaviour that
changed rather than the file you touched. Chinese and English subjects are both used.

PR descriptions follow `.github/PULL_REQUEST_TEMPLATE.md`. Include what you ran, and for UI changes,
screenshots at 1440 **and** 390 — touch-target and wrapping behaviour only shows up narrow.

The habit we ask for in a PR body is the same one we ask of code: say what would have gone unnoticed.
"If I deleted this rule, would anything turn red?" is a question worth answering before you submit.

## Where to start

三处机械且可审查的欠账：**OpenAPI 注解**（125 条注解覆盖 513 条路由注册，`internal/api/ai` 197 条注册仅 1 条注解，补 `@Summary/@Tags/@Param/@Success/@Router`）；**卡住的棘轮**（`localStyleClones` / `pagePxLiterals` / `spacePixelKeys`，每合并一处就能降一格）；**界面文案**（`web/src/views` 下缺 i18n，先从访客最先看到的登录、落地页、AI 工作台开始，`web/tests/i18n-sync.test.ts` 会守住两份语言包同步）。

改公开 API、RBAC 默认或主题契约（`THEME_CONTRACT_KEYS`）之前先问——那是决策，不是杂活。

## 中文说明

上面的规则一条都不打折，这里只补三句本地同事常问的：

- **提交前至少跑** `go build ./...`、`go test ./internal/... ./tools/repocheck/...`、`cd web && npm run ci`；
  改了样式再跑 `npm run check:style`（钩子只跑五道里的第一道）。
- **新增或收紧任何门禁，必须反向验证**：造一条违例确认它变红，再用编辑撤回去
  —— 别用 `git checkout`，那会连你未提交的工作一起丢。
- **判据不许靠"看起来对"**。这个仓库里最贵的几类 bug 都是"门禁永远绿"而不是"代码有 bug"：
  选择器写错、退出码没置、集合过了结构化克隆变成空对象。写检查时先想"它怎么会漏"。

## Code of conduct

There is no separate code-of-conduct file — the standard is short enough to state here: be precise,
be kind, and report problems privately rather than publicly. Security issues follow
[SECURITY.md](SECURITY.md) — they are not issues.
