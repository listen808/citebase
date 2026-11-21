# CI 与模块路径：把"强制"放回本地

- 日期：2026-10-07
- 状态：已定（CI 仍处于未启用状态）
- 影响面：构建、发布、提交流程、所有 import 路径

## 背景

两件事在同一天收口，因为它们是同一个"改名"的两面：

1. **Go 模块路径**。项目从 AIWiki 改名为 Citebase 时，模块路径先迁到 `github.com/listen808/AIWiki`，随后随品牌收口一起迁到 **`github.com/listen808/citebase`**（当前 `go.mod` 第一行即此值）。模块路径不是显示名：它决定所有内部 import、`Makefile` 里 `-X` 注入的符号名（`github.com/listen808/citebase/internal/version`）以及任何按包路径写死的工具。
2. **CI 从未在这个远端触发**。仓库里曾经有 CI 工作流文件，但这个远端不会触发它；于是"PR 检查"从来没有真正跑过，真正的强制只有本地命令与提交钩子。

## 决策

- **模块路径以 `github.com/listen808/citebase` 为准**，不再引入 `AIWiki` 路径。改名只做一次，避免出现"一半 import 用旧路径"的混合态（Go 编译能过，但两个符号名指向同一份代码的幻觉会持续误导人）。
- **强制前移到本地**：
  - `make ci` = lint + test + build + 前端构建 + `check-docs`；
  - `cd web && npm run ci` = lint → type-check → `check:style`（七道）→ `check:i18n-msg` → vitest → build；
  - 提交钩子（有钩子时）只覆盖一个子集：ESLint（error 级）→ `check-style-tokens` → `type-check` → `vitest`。
- **接受"钩子是子集"这个事实**：主题契约、设计教条、对比度、玻璃普查、布局契约都不在提交时跑，所以"提交成功"不等于"门禁全绿"，改前端后要手动 `npm run check:style`。

## 代价

- 没有远端 CI，就没有"别人替你在干净环境里跑一遍"这层保险。同一台机器上"我这儿能过"可能是缓存、Node 版本或本机依赖造成的假象。
- 钩子只跑第一道样式门禁，最容易漏的恰恰是主题/对比度/布局那几道——这类问题在 code review 里也最难用肉眼发现。
- 版本注入只剩 `Commit` 与 `BuildDate`：`Version` 是 `internal/version` 的 `const`，`-ldflags -X` 注入不进 const（会被静默忽略），所以版本号变更必须走代码 + 门禁，不走构建参数。

## 现状补充（2026-11-22 核对）

这份检出里**没有** `.github/` 与 `.husky/` 目录：`git ls-files` 中不存在 `.github/workflows`，`Test-Path .husky` 为 false（`.gitignore` 只忽略 `/.husky/_/`，不忽略钩子本身）。也就是说，本文所述的钩子与工作流目前都不在库内，"唯一强制 = 本地命令"不是权宜之计，而是当前事实。

若要恢复远端强制，需要同时做三件事：把工作流与钩子文件重新入库、让它们与本文件列出的门禁清单对齐、并在远端确认触发条件。只恢复其中一件，会得到"看起来有 CI、实际没有"的第二种假象。

## 后续

- 发布前一律在本机跑 `make ci` 与 `cd web && npm run ci`，把输出贴进 PR 描述；
- 新增门禁时，先反向验证它会红（见 [../gates/static-gates.md](../gates/static-gates.md)），再决定它进不进提交钩子；
- 若将来启用远端 CI，优先把七道 `check:style` 与 `check-docs` 放进去——它们最便宜、最容易被本地跳过。
