# 行尾与编码：仓库是 LF，工作区别再造 CRLF

- 日期：2026-10-06
- 状态：已定
- 影响面：全仓库（Windows 检出、脚本、编辑器）

## 背景

在 Windows 上用 PowerShell/记事本打开文件后，`git status` 曾一次列出 **189 个文本文件**为 `M`，而 `git diff` 一行内容都没有。伴随症状是 `gofmt -l .` 报出大量文件、编辑器里出现 mojibake、以及"提交一个只改行尾的 commit"的冲动。

排查结论有两条，都和直觉相反：

1. **仓库里的 blob 本来就是 LF**。`.gitattributes` 早已统一为 LF 存储，问题只在**这次检出**：`core.autocrlf=true` 会在检出的那一刻把工作区写成 CRLF，而 git 下次检出前不会自动改回来。
2. **`git add --renormalize .` 在这里是空操作**。它比较的是"仓库内容 vs 索引"，而 blob 已经是 LF，所以它什么也不做；工作区仍是 CRLF，`gofmt` 读的却是工作区。

所以那个"chore: normalize line endings"的提交是**0 变更提交**——`git diff --numstat` 一行都没有，status 里的 189 个 `M` 只是 stat 缓存失效。

## 决策

- `.gitattributes` 作为唯一基线：`* text=auto eol=lf`，再对 `*.go` / `*.ts` / `*.vue` / `*.json` / `*.yaml` / `*.sql` / `*.md` / `*.sh` 显式 `eol=lf`；
- `*.bat` / `*.cmd` 保留 `eol=crlf`（Windows 命令解释器对 LF 的行为不保证）；二进制扩展名标 `binary`，不让 git 尝试转换；
- Windows 上写文件用 **UTF-8 + LF**；编辑器默认保存格式要设成这样，别依赖提交时修正。

## 代价与边界

- 本机把工作区 189 个文件就地转成 LF 后，`git diff` 为空、**没有产生任何提交**——这是正确结果，不是"没生效"。
- 看到 status 里成片 `M` 而 `git diff` 为空时，先 `git diff --numstat`；为空就是 stat 缓存，不要提交，也不要 `git add`。
- 判断"是不是真的被改了行尾"：`git ls-files --eol <path>` 看 `w/lf` 与 `i/lf`。blob 为 LF 而工作区为 CRLF，就是检出配置问题，不是仓库内容问题。

## 后续

换机器或重装 git 后，先确认 `core.autocrlf` 与 `.gitattributes` 的组合不会把工作区写成 CRLF；如果再次出现成片 `M`，按上面的判据处理，不要新增“规范化行尾”的提交。
