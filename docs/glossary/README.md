# 术语表（glossary）

`web/scripts/glossary-audit.mjs` 的**输入**是 `docs/glossary/frontend-glossary.md`：脚本读那份清单，逐条在运行中的前端上量，报表里同时给出"命中实例 / 命中页 / 差异样本"。所以术语清单只维护一份，**不要在脚本里再抄一份名单**。

## 当前状态（2026-11-22）

`frontend-glossary.md` 在这份检出里**尚未恢复**（`docs/` 目录此前整体为空，见根目录审计报告）。也就是说：

- `glossary-audit.mjs` 现在会因找不到输入而无法产出有效结论；
- 术语巡检不阻塞 `npm run ci`（它不在门禁链里），但重新启用前必须先补这份清单。

## 恢复时注意

格式以 `glossary-audit.mjs` 的解析逻辑为准（它按表格/条目取术语名与建议写法），不要自行发明结构；术语应覆盖"组件名、布局模式、材质、层级、交互动词"这几类，与 `web/src/styles/README.md`、`docs/engineering/` 的用词一致——术语表的价值是**同一个东西只有一个叫法**，与 [one-implementation](../engineering/one-implementation.md) 是同一件事的两个侧面。
