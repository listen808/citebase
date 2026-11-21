# 审计报告（audits）

一次性审计、对标、排查的**人读结论**放这里。命名 `YYYY-MM-DD-<主题>.md`，结论要能被下一个人直接执行，不要只贴一堆数字。

约定：

- **机读结果不入库**：例如术语审计的 `glossary-audit.json` 由 `.gitignore` 忽略；人读表（如 `glossary-audit.md`）才进本目录。
- **带结论与证据**：每条结论后面写"在哪看到的、怎么量的"，否则三个月后没人敢用。
- **与 decision-log 的分工**：本目录记"我们发现了什么"，`docs/engineering/` 记"现在的规矩"，`docs/decision-log/` 记"为什么定了这条规矩"。
- 审计里如果产生了结构性决策，正文指向对应的 decision-log，不要在两处各写一遍。

当前由脚本产出的报告：`web/scripts/glossary-audit.mjs` 会把术语清单逐条在运行中的前端上量，输出"命中实例 / 命中页 / 差异样本"三列（需 :3000/:3001 与 `SNAP_USER` / `SNAP_PASS`）。它的输入是 `docs/glossary/frontend-glossary.md`。
