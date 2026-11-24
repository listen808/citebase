# 设计登记 · citebase

> 依据 `../../standards/前端设计标准.md`（v1）与 `../../standards/docs/audits/2026-11-24-前端设计合理性与样式一致性.md`。
> 本仓是这套标准的**样例实现**，别的仓对照它抄结构，不抄品牌。

## 令牌分层

| 层 | 位置 | 职责 |
|---|---|---|
| 布局/空间 | `web/src/styles/theme` 的 `AppleSpace` | 间距、圆角、控件高度、层级 |
| 字体 | `AppleType` | 字号、字重、行高、字距 |
| 组件 | `web/src/styles/component` | 卡片、表格、按钮等原语样式 |
| 视图 | `*.vue` 的 `<style scoped>` | 只允许 `cv()` 取令牌 |

## 主色与主题

- 主色：Apple 蓝 `#0071e3`（邮件模板等 HTML 内联样式里同样用这个值，邮件不认 CSS 变量）。
- 主题 6 套：`light` / `dark` / `editorial` / `warm-paper` / `solid` / `glass`，每套独立取值，不做"浅色变量直接翻转"。
- 圆角：3 个命名档 + pill（按主题取值）。

## 字号

`AppleType` 是唯一来源，视图层禁止裸 px。例外只有一处，见豁免表。

## 门禁

本仓已有 7 道样式门禁（详见 `web/src/styles/README.md`）。新增的跨仓门禁在这里跑：

```powershell
node ../../standards/docs/design/check-design.mjs web
```

## 豁免表

| 位置 | 内容 | 理由 |
|---|---|---|
| `web/src/components/apple/ApMarkdownPreview.vue` `.callout-title` | `font-size: 12px`（已标 `design-allow`） | 富文本预览按宿主正文档位缩小，随 `.md-preview` 的 AppleType 基准走 |
| 邮件模板（`views/admin/system/EmailConfigView.vue`） | HTML 内联 `font-size` | 邮件客户端不支持 CSS 变量，只能写死 |

## 季度表（P2）

- 组件分层：已四层齐全。
- 待办：视图层残留的硬编码色（27 处，见审计报告）分批回指语义令牌。
