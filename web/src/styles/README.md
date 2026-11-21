# 前端样式体系使用指南

> 本目录是 **共享样式对象** 的仓库。项目规范（`AGENTS.md`）：**禁止 `.css` / `.scss` 文件**，全部样式通过
> `utils/core/apple-tokens.ts` 的设计令牌 + `:style` 内联对象实现；仅在必须使用伪类/动画时允许 `<style scoped>`。
>
> 最后更新：2026-09-28（主题契约化：暗色与扩展主题统一为差异表，新增主题守卫脚本）

---

## 1. 三层结构

| 层 | 位置 | 职责 |
|---|---|---|
| 令牌层 | `utils/core/apple-tokens.ts` | 唯一取值来源：颜色 / 字号 / 间距 / 圆角 / 阴影 / 动效 / 层级 / 断点；`injectCSSVariables()` 注入 `:root` 浅色默认值 |
| 主题层 | `styles/theme/themes.ts` | 主题差异表（含暗色）+ 契约必填 token + 主题元数据；`themeExtraCss` 放非变量类签名 |
| 组合层 | `styles/**`（本目录） | 用令牌拼装可复用样式对象（卡片、表格行、区块头……），**不得出现裸值** |
| 视图层 | `views/**`、`components/**` | `:style` 绑定；需要新形态时先在本目录补组合对象，而不是就地写字面量 |

守卫脚本：`npm run check:style` = `check-style-tokens.mjs`（变量注册表、非法拼接、裸 z-index、非 scoped、材质令牌）
+ `check-theme-contract.mjs`（主题契约 T1–T11）+ `check-design-doctrine.mjs`（设计教条 G1–G7，见 §8）+ `check-contrast.mjs`（WCAG 对比度）+ `check-layout-contract.mjs`（页面布局契约）。
预览：`npm run gen:theme-preview` 从主题数据生成 `docs/ui-previews/theme-live-preview.html`。

## 2. 目录清单

| 模块 | 领域 | 消费方式 |
|---|---|---|
| `layout/page-styles.ts` | 通用页面 / 列表 / 表单 / 详情 | `@/styles/layout/page-styles` |
| `layout/doc-sidebar-styles.ts` | 文档侧栏 | 直连 |
| `ai/ai-page-styles.ts` | AI 子页通用（含 `export * from '../layout/page-styles'`） | 直连 |
| `ai/chat-styles.ts`、`ai/schema-styles.ts` | AI 聊天 / Schema 页 | 直连 |
| `component/chart-styles.ts` | 图表容器 / 图例 / 热力图 | 经 `page-styles` 再导出 |
| `component/crud-card-styles.ts` | 后台 CRUD 卡片 | 经 `page-styles` 再导出 |
| `component/dashboard-styles.ts` | 仪表盘 | 经 `page-styles` 再导出 |
| `component/dialog.ts`、`component/overlay.ts` | 对话框、遮罩浮层 | 直连 |
| `component/doc-styles.ts` | 文档阅读页 / TOC | 经 `page-styles` 再导出 |
| `component/form-field-styles.ts` | 表单字段（label / error / hint） | 直连（`ApFieldWrapper` 等） |
| `component/monitor-styles.ts` | 运维监控 | 经 `page-styles` 再导出 |
| `component/search-result-styles.ts` | 搜索结果卡片 | 经 `page-styles` 再导出 |
| `component/spotlight-search-styles.ts` | 全局搜索浮层 | 直连 |

| `theme/themes.ts` | 主题差异表 + 契约 + 元数据（唯一主题清单来源） | `apple-tokens.injectCSSVariables`、菜单 / 设置页 |
| `theme/highlight-tokens.ts` | 代码高亮三调色板（light / dark / paper）+ Monaco 主题 + mermaid 取色 | `useMarkdownRenderer`、`MonacoEditor` |

> 已删除：`component/table-styles.ts`（2026-09-18，全仓零引用；表格样式统一由 `ApTable` 承担）。
>
> **消费路径规则**：新增样式一律 **直连所属领域模块**；`page-styles.ts` 的具名再导出仅作历史兼容，
> 不要再往 barrel 里添加名字。若某个符号只能从 barrel 取到，说明它本该移出 `page-styles.ts`。

## 3. 命名约定

- 导出名 `xxxStyle`；需要入参的写成工厂函数 `xxxStyle(variant)`；同模块内不要再混用 `get*Style` 与裸名两种风格（新增一律裸名）。
- 领域前缀必填：AI 用 `ai*`、监控用 `monitor*`、CRUD 用 `crud*`；禁止 `headerStyle` / `containerStyle` 这类无前缀泛型名跨模块重名。
- 变体只走参数（如 `crudStatsRowStyle('compact')`），不要再复制出 `compact*Style` 平行函数。

## 4. 语义色对照表（**照这张表取值，不要发明新变量名**）

`cv('name')` 的入参类型是 `AppleCssVarName`（由注册表推导），写错会直接编译失败。

| 语义意图 | 正确令牌 | 已清理的错误写法 |
|---|---|---|
| 主文字 | `cv('text-primary')` | `cv('accent')`、`#1d1d1f` |
| 次级文字 | `cv('text-secondary')` | `cv('text-48')` |
| 辅助 / 占位文字 | `cv('text-tertiary')` | 与 `cv('mid-border-gray')` 混用 |
| 禁用文字 | `cv('text-disabled')` | `cv('text-quaternary')`（现仅用于极弱化文字） |
| 主色 / 强调 | `cv('blue')` | `cv('accent')`、`cv('apple-action-blue')` |
| 状态色 | `cv('green' \| 'orange' \| 'red' \| 'yellow' \| 'purple' \| 'teal')` | `cv('amber')`、`cv('lime')`、`cv('yellow_dark')` |
| 分隔线 / 卡片边框 | `cv('border-light')` | `cv('separator')`、`cv('border-dark')` |
| 控件边框 / 强边框 | `cv('soft-border-gray')`、`cv('border')` | `#e5e5e5` |
| 页面底（卡片之下的画布） | `cv('surface-canvas')` | `cv('bg-primary')`——**该名字已从 `:root` 删除**，写了 `cv()` 类型不过（门禁 T12 也拦） |
| 壳层（顶栏 + 侧栏） | `cv('surface-chrome')` | `cv('glass-bg-nav')`——同上，名字已删；`<style>` 块里的 `var(--apple-glass-bg-nav)` 由门禁 G10 拦 |
| 输入控件的凹陷面 | `cv('surface-field')`（input / textarea / select 触发器 / 代码块内的小控件） | 蹭页面底（玻璃主题下 canvas 是 **transparent**，输入框直接变成洞）、`cv('surface-panel')`（与面板同色，读不出字段） |
| 凹陷 / 悬停 / 骨架 | `cv('bg-secondary')`（凹陷与 chip）、`cv('bg-tertiary')`（骨架占位）、`cv('bg-hover')`（悬停反馈） | 把 `bg-secondary` 当卡片底用 |
| 彩色弱底纹（徽章、选中行） | `cv('blue-bg-subtle')` 及 `green / orange / red / purple / teal / indigo / pink / yellow / gray-bg-subtle` | **禁止** `${cv('blue')}15` 之类字符串拼接 |
| 选中蓝底 | `cv('blue-selection')`、`cv('blue-selection-strong')` | `cv('light-blue-bg')` |
| 焦点环 | `cv('blue-focus-ring')`（整条 shadow）、`cv('focus-ring-color')`（纯颜色） | `0 0 0 3px var(--apple-blue-focus-ring)`（长度前缀重复，非法） |
| 悬浮层阴影 | `cv('shadow-modal')`（模态 / 抽屉）、`cv('shadow-dropdown')`（下拉 / 浮层） | `AppleShadow.modal` 等 TS 常量（不随主题变，门禁 T8 拦截）、手写 `blur + rgba` 串 |
| 卡片 / 面板表面 | `cv('surface-panel')`（卡片、面板、区块） | `cv('bg-card')` / `cv('glass-bg')`（两个名字都已从 `:root` 删除，它们原先就是这一档的转发）、`cv('pure-white-canvas')`（它是「画布」，暗色下是**黑**） |
| 浮层面板表面 | `cv('surface-float')`（模态、抽屉、下拉、气泡、聚光） | `cv('surface-elevated')`（那一档是给"任意内容从底下滚过去"的壳面留的地板，菜单跟着它就会被过成白板）、`cv('bg-elevated')`（名字已删）、`cv('surface-panel')`（玻璃主题下浮层要比面板更实，台阶会断在这里） |
| 壳层加厚表面 | `cv('surface-elevated')`（顶栏 under 态、吸顶表头 —— 判据是**身后有没有正在滚动的内容**，不是"看起来像不像浮层"）。顶栏那一处的 alpha 在运行时由 `useShellGlassLift` 按实测底色改写，令牌值是"量不出来"时的兜底 | 给静态浮层面板用（会把 float 那一档的通透白付掉）、直接把这一档当"浮起来一点"的静态底色（滚动时它必然跳到 0.88/0.96，浅色读作白板、深色读作灰板） |
| 开关滑块 | `cv('surface-knob')`（**抬起面，且上面不许有文字**） | `cv('surface-panel')`（玻璃主题下会变半透明） |
| 压在强调色 / 图片上的文字与图标 | `cv('on-accent')` | `cv('pure-white-canvas')`、`#fff`（门禁 T10 / T11） |
| 材质模糊 | `cv('glass-blur')` / `cv('blur-panel')`（**不要**把 SVG 滤镜 url() 拼进 backdrop-filter——Chromium 整条声明不渲染，连 blur 一起失效，见 docs/engineering/liquid-glass.md） | `AppleGlassBlur.standard` 等字面档位（门禁 T8） |
| 控件圆角 | `AppleRadius.control`（引用 `--apple-radius-control`，主题可改） | `AppleRadius['13px']`（旧写法，脚本已迁移） |
| 卡片圆角 | `AppleRadius.card`（引用 `--apple-radius-card`） | `AppleRadius['18px']` |
| 全圆 | `AppleRadius.pill`（980px）、`AppleRadius.circle`（50%） | `AppleRadius['980px']` |
| 遮罩透明度 | `cv('overlay')`、`cv('pink-selection')`、`cv('white-alpha-20')` | `cv('pink') + '4D'` |
| 同色浅染上的文字（徽章、药丸、选中行） | `cv('green-text')` 等 `{hue}-text`，动态色相用 `textOnTint(cv('green'))` | `color: cv('green')`（压在自己 14% 浅染上只有 2~3:1） |
| 实心色块上的前景 | `cv('on-green')` 等 `on-{hue}`，动态用 `onSolid(color)` | 亮色相上写 `cv('on-accent')`/白字（亮绿只有 2.2:1） |

> **`surface-knob` 这条档的成立前提是"块上没有文字"**，不是"恒不透明"：玻璃材质下它实际是
> `rgba(255,255,255,0.96)`（`themes.ts` 的 `GLASS_OVERLAY`），而深色玻璃下
> `--apple-surface-panel` 是 `rgba(30,30,32,.62)`。所以**别把带文字的东西搬上 knob**——
> 实测把 `ApSegmentedControl` 的选中块从 panel 改成 knob 后，dark 下选中项文字
> （`text-primary` `#f5f5f7`）压在 knob `#ffffff` 上只有 **1.09:1**（压现在的 panel 是 15.63:1），
> 直接不可读。开关滑块没有标签，才成立。

## 5. 硬规则

1. **颜色一律 `cv()`**，且必须是 §4 中存在的名字；主题差异由 `data-theme` 覆盖自动生效，不要写 `#rrggbb`。
   黑 / 白原色（`#fff`、`#000`、`rgba(0,0,0,·)`、`rgba(255,255,255,·)`）只允许出现在令牌层——门禁 T11 拦截，
   唯一例外是 HTML 邮件模板（在 `RAW_COLOR_EXEMPT` 里带理由登记）。
2. **要半透明**：优先 `-bg-subtle` / `-selection` / `white-alpha-*` 令牌；没有合适的就**先在 `cssVariables` 注册**再用，禁止拼接颜色字符串。
   染色族（`*-bg-subtle` / `*-light` / `callout-*` / `blue-selection` / `blue-focus-ring`）在 `:root` 用
   `color-mix(in srgb, var(--apple-纯色) N%, transparent)` 表达——主题只要改纯色就整族跟随，不必逐套手写。
   **注意**：这类派生只在「纯色与派生式同元素」时成立，所以它们必须留在 `:root`（`data-theme` 也在 `<html>` 上），
   不要把它们挪到某个子容器上作用域化，否则不会跟随。
3. **主题相关判断不许写主题 id 字面量**（门禁 T7）：明暗用 `isDarkThemeId()`，带 Clear 变体用 `supportsGlassClear()`，
   其它元数据用 `themeMetaFor()` / `THEME_REGISTRY`。新增主题的落点是 `styles/theme/themes.ts`，
   菜单与设置页会自动出现它（`npm run gen:theme-preview` 可重新生成对照预览）。
4. **canvas / 图表库不能解析 `var()`**（ECharts、位图导出等）：在渲染函数内用 `resolvedColor('blue')` 取实际值，主题切换时重算（见 `AIKnowledgeGraphView` 的 `refreshPalette()` + `watch(() => ui.theme, renderChart)`）；透明度用 `withAlpha(color, 0.5)`。
5. **数值一律走令牌**：间距 `AppleSpace.*`、圆角 `AppleRadius.*`、排版 `AppleType.*`（成组含字重 / 行高 / 字距）、层级 `AppleZIndex.*`、时长曲线 `AppleTransition.*` / `AppleEasing.*`。
6. **材质与阴影走 CSS 变量**：`cv('shadow-modal')` / `cv('glass-blur')`，不要用 `AppleShadow.*` / `AppleGlassBlur.*` 字面常量（门禁 T8）；后者只在令牌层内部使用。
7. **一把尺一个角色**：控件高度 `AppleInputSize`（32/38/44）、按钮 `AppleButtonSize`、对话框宽度 `AppleModalSize`、窄卡度量 `AppleLayout.cardNarrow/cardWide`、图标按钮 `AppleLayout.iconButton*`、断点唯一出口 `utils/responsive.ts`（数据源 `AppleBreakpoint`）。
8. **`<style>` 块必须 `scoped`**，且只承载伪类 / 伪元素 / 关键帧；出现 `!important` 或成段排版规则视为违规（现存例外仅 `components/ai/chat/ChatMessageBubble.vue` —— v-html 渲染内容命中不了 scoped 选择器；门禁白名单 `SCOPED_ALLOWLIST` 也只有它一个）。

## 6. 用法示例

```typescript
// ✅ 复用组合层对象，按需扩展
import { cardStyle } from '@/styles/layout/page-styles';
import { AppleSpace, cv } from '@/utils/core/apple-tokens';

const featureCardStyle = { ...cardStyle, padding: AppleSpace.lg };

// ❌ 不要就地发明值
const bad = { background: '#f5f5f7', borderRadius: '13px', boxShadow: '0 2px 8px rgba(0,0,0,.06)' };
```

扩展共享对象时用展开，不要改动导出的常量（会波及所有使用方）：

```typescript
// ❌ formGroupStyle.marginBottom = '24px';
// ✅ const customFormGroup = { ...formGroupStyle, marginBottom: AppleSpace.lg };
```

## 7. 新增一个样式的流程

1. 先查 §4 与 `apple-tokens.ts`，确认能否用既有令牌 + 既有组合对象表达；
2. 能复用就复用，差异用展开覆盖少量属性；确属新模式时，在 **对应领域模块** 新增导出（带一行用途注释）；
3. 需要新颜色 / 底纹时，在 `cssVariables` 注册浅色默认值；若每套主题都该有不同的表现，把变量名加进
   `themes.ts` 的 `THEME_CONTRACT_KEYS`——契约类型会立刻报出哪套主题还没给值（也可在主题里写
   `color-mix(纯色 N%)` 让它自动跟随）；最后补进 §4 对照表；
4. 提交前跑 `npm run check:style && npm run type-check`（`check:style` 已包含主题契约守卫）。

---

## 8. 设计教条：宁可素，不要「通用 AI 界面」（2026-09-28 重绘）

审计出的事实（不是审美偏好）：138 个卡片共用同一套 18px 圆角 + `shadow-card` + 1px 边框（tile soup）；
同一个统计指标出现 19 / 24 / 32px 三种字号；卡片内边距 8 / 16 / 24 / 32 混用（4 倍差）；首页是
56px 问候大字 + 斜体语录 + 按 id 分配的 Notion 彩虹渐变卡 + shimmer 闪动 + 交错入场。
这些既是最典型的「一眼 AI」特征，也和主题系统冲突（写死渐变与白字无法随主题变）。

当前档位，改代码前先对齐这里：

| 维度 | 规则 | 落点 |
|---|---|---|
| 圆角 | 卡片 10px / 控件 8px / 面板小块 4px；各主题自行覆写（玻璃 14·10·6，纸 8·6·3，杂志全 0） | `--apple-radius-card` / `-control` / `-panel` |
| 静置阴影 | 卡片静置**不投影**，层级靠 1px 发丝边框；只有浮层（modal / dropdown）与 hover 才有阴影。唯一例外是液态玻璃的镜面内高光 | `--apple-shadow-card` |
| 内边距 | 卡片四档：紧凑 8 / 默认 12 / 舒适 16 / 宽松 24（`ApCard size`），原 8/16/24/32 让同类页面密度差到 4 倍 |
| 材质 | 面板模糊与浮层模糊分档：`--apple-blur-panel`（默认 none）≠ `--apple-glass-blur`；只有壳层 / 真浮层 / 按钮控件档付 `backdrop-filter`，卡片等流水面不付。镜面高光位置由 `--apple-light-x/y` 跟随指针（`useSpecularLight`；无指针或减弱动效时静态）；折射位移滤镜**不进 backdrop-filter**（Chromium 不渲染，见 docs/engineering/liquid-glass.md） | `cv('blur-panel')` / `cv('glass-blur')` |
| 排版层级 | 页面标题 `pageTitle`(21) → 分区 `sectionTitle`(15/600) → 正文 `bodyPrimary`(17)；统计数值只有 `statValue`(20) + `tabular-nums` 一档 | `AppleType` |
| 展示级大字 | `heroDisplayXL/L`、`sectionDisplay`、`productHeading`、`featureDisplay`、`promoDisplay`、`cardProductTitle` 只准出现在错误页 / 登录页 / 安装页（门禁白名单） | 门禁 G2 |
| 图标 | 一律 lucide 组件，禁止 emoji 当图标；不做「图标装进彩色圆盘 / 方块」；选中态用 `blue-selection` + `blue-text`，不用白底白字，也不用实心强调色块 | 门禁 G3 |
| 收藏开关 | 状态只用一颗星的**亮暗**表达：实心橙 = 已收藏、描边灰 = 未收藏；可访问名与气泡随状态一起变；位置固定在 `ApEntityCard` 的 `#actions`（与元信息同行、右对齐）。斜线星 `StarOff`、红色 `variant="danger"` 的「取消」、`fill: currentColor`（星跟着文字色走）都是历史上四五种写法，已收敛 | `ApFavoriteButton` + 门禁 G14 |
| 表面阶梯 | 五档角色逐主题给值：`surface-chrome`（顶栏 + 侧栏）→ `surface-canvas`（页面底）→ `surface-panel`（卡片）→ `surface-elevated`（壳层加厚：内容滚到顶栏底下时的那一档）→ `surface-float`（浮层面板：菜单 / 弹窗 / 抽屉）。后两档在实心主题下同色、在玻璃下分开，因为**它们身后的东西不是一回事**：elevated 要扛任意滚过来的内容（极值纯黑），float 只扛本应用环境（极值一张暗图）。浅色下 canvas `#f5f5f7` / 卡片与壳层 `#ffffff`，层级靠「台阶 + 发丝边框」而不是阴影 | 契约 + 门禁 T12 / R12；两档各自的地板在 check-contrast 的两组浮层断言 |
| 卡片网格 | 地板只有四档：`tile` 140（纯图形格）/ `stat` 200（一块数值）/ `entity` 320（内容实体卡，**空间卡与文档卡同档**）/ `panel` 440（图表与并排信息块）。用 `entity` 档必须等高（`gridAutoRows:'1fr'` 或 `appleCardGrid('entity')`），卡片自己是 flex 列，元信息压到底部对齐 | `AppleGridFloor` + 门禁 R10 / R11 |
| 动效 | 无交错入场（`v-stagger` / `useStaggerEnter` 已删），无整卡 `scale` 抖动；hover 只改描边与底色 | 门禁 G1 |
| 渐变 | 卡片级渐变机制（`ApGradientCard`、`utils/format/gradient`）已删除；渐变只准由主题令牌拼接（骨架微光、热力图图例） | 门禁 G1 / G5 |
| 文案呈现 | 问候语与语录的**文字保留**，但降级为普通标题 + muted 小字，不再是 56px 大字 + 斜体 | `PageHeader.vue` |
| 表面令牌 | `pure-white-canvas` 是「画布/反色」语义，暗色下是黑色：**不得**当背景、不得当强调色上的文字（用 `surface-panel` / `on-accent`） | 门禁 G4 + 主题 T10/T11 |

机器化：`npm run check:design`（G1–G7，已串进 `npm run check:style`）。新增页面自查三件事——
圆角/阴影是否走令牌、字号是否走 `AppleType`（别用展示级大字）、图标是不是 lucide。

---

**维护者**: Citebase Development Team
