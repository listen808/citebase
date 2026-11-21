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

Run the gate that matches what you touched — the pre-commit hook only covers a subset.

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

The pre-commit hook (`.husky/pre-commit`) runs ESLint → the token gate → type-check → vitest. It
does **not** run the other four style gates, doctrine checks, or any browser inspector.
`HUSKY=0 git commit …` skips a single run; please don't use it to skip a failing gate.

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

## Frontend conventions that are not obvious

- **No CSS/SCSS files.** Styling is inline `:style` bound to Apple tokens (`cv()`, `AppleType`,
  `AppleSpace`, `AppleRadius`, …). Use `Ap*` components rather than raw HTML.
- **Never override an `ApButton`'s colour with `:style`.** Use the `variant` prop. On images or dark
  veils use `variant="overlay"`.
- **One implementation per concept.** Progress bars live in `ApProgress`, segmented controls in
  `ApSegmentedControl`, modals in `ApModal`, breadcrumbs in `ApBreadcrumb`, menus in
  `menuSurfaceStyle()`. A second implementation of the same concept is what the G22–G29 gates fail.
- **Layout contract:** page roots are `flex: 1; minHeight: 0`; no `100vh`; exactly one
  page-level `overflow-y: auto`; grid floors come from `AppleGridFloor`, not hand-written `minmax()`.
- Type scale is `AppleType` only, and letter-spacing is derived via `trackingFor(px)`.
- The About page's version string is injected from `web/package.json` at build time. Do not hard-code
  a version anywhere; `internal/version/version_test.go` fails the drift.

## Backend conventions that bite

- All tables use the `aw_` prefix. `databases/table.sql` is generated by `go run ./tools/genddl`
  from `repository.RegisteredModels()` — never hand-edit it; register the model and re-run.
- Adding a table means: model in `internal/model/<domain>`, registration in the matching
  `internal/repository/migrate_*.go`, index specs in `db.go`'s `allIndexes`, then re-run genddl.
- New admin-facing route → register a `PrivilegeMap` rule **and** confirm the family is in
  `sensitiveFamilies`. Outside the admin prefix, "not registered" means *allowed*, not "denied by
  default".
- Background goroutines go through `internal/utils/safego` (`Go(name, fn)`); the bare `go func`
  count is a ratchet that only decreases.
- Outbound HTTP must go through `internal/utils/netguard` — it dials the IP it validated, so DNS
  rebinding doesn't reopen the check.

## Commits and pull requests

Conventional Commits: `fix(rag): …`, `feat(web): …`, ~72 characters, describing the behaviour that
changed rather than the file you touched. Chinese and English subjects are both used.

PR descriptions follow `.github/PULL_REQUEST_TEMPLATE.md`. Include what you ran, and for UI changes,
screenshots at 1440 **and** 390 — touch-target and wrapping behaviour only shows up narrow.

The habit we ask for in a PR body is the same one we ask of code: say what would have gone unnoticed.
"If I deleted this rule, would anything turn red?" is a question worth answering before you submit.

## Where to start

- **OpenAPI annotations.** 121 annotations over 502 registrations; `internal/api/ai` has 197
  registrations and 1 annotation. Adding `@Summary/@Tags/@Param/@Success/@Router` to a handler is
  mechanical, reviewable, and the ratchet in `tools/repocheck/swagger_coverage_test.go` rewards it.
- **Ratchets that are stuck at their current value.** `localStyleClones`, `pagePxLiterals`,
  `spacePixelKeys` in `web/scripts/check-style-tokens.mjs` — each unit you consolidate lets the
  baseline come down.
- **UI strings.** `web/src/views` is where i18n is missing. Start with what an outsider sees first:
  login, landing, AI workspace. Move copy into `web/src/i18n/locales/*` and let
  `web/tests/i18n-sync.test.ts` keep the two bundles in step.

Ask before starting anything that renames a public API, changes the RBAC default, or touches the
theme token contract (`THEME_CONTRACT_KEYS`) — those are decisions, not chores.

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
