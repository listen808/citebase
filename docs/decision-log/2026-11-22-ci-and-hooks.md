# CI 落地与钩子取舍

- 日期：2026-11-22
- 状态：已定
- 影响面：门禁 / 部署

## 背景

仓库长期处于"有门禁、无自动化"的状态：`make ci` 与 `web/package.json` 的 `npm run ci` 串起了
gofmt、vet、40 包测试、eslint、vue-tsc、7 道样式门禁、i18n 审计与 vitest，但

- `.github/` 下只有 issue 模板，没有 workflow；
- `web/package.json` 声明了 husky 与 lint-staged、也写了 `prepare`，但 `.husky/` 目录没有入库，
  所以提交时实际什么都不跑（`Test-Path .husky` 为 false）；
- 于是"门禁全绿"这个结论，只能由执行提交的人手工复现。

## 决策

1. 新建 `.github/workflows/ci.yml`：Go job 跑 gofmt（零输出）、`go build ./...`、`go vet`、
   `go test ./internal/... ./tools/repocheck/...`，随后用 Node 跑 `tools/validate-doc-links.mjs`；
   web job 跑 `npm ci --ignore-scripts` + `npm run ci`。
2. **CI 里用 `--ignore-scripts`**：`prepare` 会去装钩子，CI 不需要，且 `.husky/` 缺失时它会失败。
3. **不在本次补交提交钩子**：钩子一旦入库，会改变每个人本机的提交行为；而钩子只覆盖
   ESLint → token → type-check → vitest，恰好是最容易产生 30 秒以上等待的四道。
   L1 的职责改由"推送即触发 CI + 本地手工跑表"承担。

## 代价与否掉的方案

- 代价：提交仍然是"未验证"状态，本地钩子缺失这一条债没有还；CI 首次运行尚未发生，
  workflow 文件本身不等于一次绿色记录。
- 否掉：把 `.husky/pre-commit` 一并入库（会强制所有人装依赖才能提交，且覆盖不到样式门禁，
  反而制造"提交过了就是绿的"错觉）。
- 否掉：把 7 道样式门禁全部搬进 CI 的提交前阶段（耗时远超 10 秒，且其中多道需要 dev server）。

## 后续

首次 push 后看一次运行结果；若 web job 因 `check:style` 里的渲染类脚本失败，就把这类脚本
拆成独立的 nightly job，主 job 只保留静态部分。钩子是否补，等 CI 稳定后再定。
