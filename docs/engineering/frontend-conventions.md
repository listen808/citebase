# 前端工程约定（Vue 3）

> 适用：`web/`。后端规则见 [backend-conventions.md](backend-conventions.md)；
> 每个色值/尺寸该取哪个令牌，细节在 [`web/src/styles/README.md`](../../web/src/styles/README.md)（它住在被描述的那个目录里，改样式时抬头就能看到）。

## 1. 技术栈与目录

Vue 3.5 + TypeScript 6.0 + Vite 8 + Pinia 3 + Vue Router 5；Node `^20.19.0 || >=22.12.0`。
`vue` / `vue-router` / `pinia` 与 `composables`、`stores` 由 `unplugin-auto-import` + `unplugin-vue-components` 自动导入，**不要再手写这些 import**（手写与自动导入混用会出现两套同名符号）。

```
web/src/
├── api/          # HTTP 封装，按领域分模块；client.ts 是唯一 axios 实例
├── components/   # apple/ 是设计系统（Ap* 组件），其余按领域
├── composables/  # 组合式逻辑（collab 在 composables/collab/）
├── router/       # routes-main.ts / routes-admin.ts / routes-public.ts
├── stores/       # Pinia；ui.ts 是外观与个性化的唯一行为落点
├── styles/       # 共享样式对象 + theme/ 主题数据（无 .css 文件）
├── utils/        # core/（令牌、品牌、偏好同步）、format/、responsive.ts
├── views/        # 路由页面
└── i18n/         # en-US / zh-CN 语言包（server-messages.ts 是生成物）
```

## 2. 没有 CSS 文件：三层样式体系

**禁止新增 `.css` / `.scss`。** 样式全部是 `:style` 绑定的对象，取值来自设计令牌：

| 层 | 位置 | 职责 |
|---|---|---|
| 令牌层 | `utils/core/apple-tokens.ts` | 唯一取值来源：颜色、字号、间距、圆角、阴影、动效、层级、断点；`injectCSSVariables()` 向 `:root` 注入浅色默认值 |
| 主题层 | `styles/theme/themes.ts` | 外观 × 材质的差异表 + 契约必填 token（见 [theme-system.md](theme-system.md)） |
| 组合层 | `styles/**` | 用令牌拼装可复用对象（卡片、表格行、区块头……），**不得出现裸值** |
| 视图层 | `views/**`、`components/**` | `:style` 绑定；需要新形态时先在组合层补对象，不要就地写字面量 |

`<style>` 块只在必须用伪类/伪元素/关键帧时出现，且必须 `scoped`；白名单只有 `components/ai/chat/ChatMessageBubble.vue` 一处（`v-html` 渲染的内容命中不了 scoped 选择器）。

硬规则（由 `check:style` 的 T/G 系列拦）：

- 颜色一律 `cv('token')`，入参类型由注册表推导，写错名字编译就报；组件里写 `#fff` / `rgba(0,0,0,.x)` 属于绕过主题（T11），只有令牌层可以出现黑/白原色；
- 间距/圆角/字号/层级/时长一律走 `AppleSpace` / `AppleRadius` / `AppleType` / `AppleZIndex` / `AppleDuration`，禁止裸 px（`trackingFor(px)` 是字距的唯一派生入口，G15）；
- 材质与阴影走 CSS 变量（`cv('glass-blur')`、`cv('shadow-modal')`），不要用 `AppleShadow.*` / `AppleGlassBlur.*` 常量（T8）；
- 强调色上的文字用 `on-accent`，亮色实心块上用 `on-blue` / `on-{hue}`（T10 / G33）。

## 3. 页面骨架与布局契约

**每个路由页面都必须用 `ApPageContainer`**（R1）。它负责：面包屑派生（唯一实现 `ApBreadcrumb`，R7/R8）、页头锚点 `data-ap-page-title`（运行时工装靠它找页头，R15）、返回按钮、`#actions` 槽、内容区。

关键 props：

| prop | 作用 | 约定 |
|---|---|---|
| `title` / `subtitle` | 页头 | 页头文案走 i18n；`title` 会渲染成 `h1[data-ap-page-title]` |
| `maxWidth` | 版心宽度 | **必须显式**（全屏页的 `noScroll` 例外，R2）；默认 `1200px`，宽版 `1600px` |
| `compact` | 横向留白 16px | 非全屏页必须开（R4），不得与非 compact 的 20~40px 混用 |
| `cardContent` | 内容区套卡片表面 | 表单/详情页常用 |
| `noScroll` | 容器不滚动 | 全屏画布类页面（编辑器、图表大屏） |
| `showBreadcrumb` / `breadcrumbParent` | 面包屑 | 层级由路由 `meta.group` + 显式父级合成 |

宽度与度量的三把尺：

- **版心**：普通页 `maxContentWidth` = 1200px；表格主导页 `maxWideContentWidth` = 1600px（R3）；
- **正文**：阅读类正文 680px 行度量（R9）；
- **网格地板**：`AppleGridFloor` 四档——`tile` 140（纯图形）、`stat` 200（一块数值）、`entity` 320（内容实体卡，空间卡与文档卡同档）、`panel` 440（图表与并排信息块）。`entity` 必须等高（`gridAutoRows:'1fr'` 或 `appleCardGrid('entity')`，R11）；固定列数网格走 `appleFixedColumns()`（R17）。

页面根节点的结构规则：

1. 根容器 `display:flex; flex:1; minHeight:0`——**不写 `100vh`**（R6 只允许 `100dvh`）；
2. 整页**只有一个** `overflow-y:auto`（滚动容器走 `scrollSingleAxis`，R19）；
3. 壳层横边（顶栏 / 页脚 / 侧栏用户行）共用 `AppleLayout.shellBarHeight`（R14）；
4. 布局层不再追加左右留白（R5）——留白只由容器的 `compact` 决定；
5. 壳层表面只用 `surface-chrome`（顶栏/侧栏）与 `surface-canvas`（页面底），阶梯角色不许被壳层之外的文件复述（R12/R13）。

## 4. 组件

- 一律优先 `Ap*`（`components/apple/`）：`ApButton`、`ApModal`、`ApTable`、`ApInput`、`ApProgress`、`ApSegmentedControl`、`ApEntityCard`、`ApFavoriteButton`、`ApIcon` 等。
- **不要用 `:style` 改 `ApButton` 的颜色**——用 `variant`；压在图片或深色遮罩上用 `variant="overlay"`。
- 按钮体只有一个图标时声明 `icon-only`（G13）；按钮内图标尺寸全站 16（I5）；图标一律 lucide 组件 + `ApIcon` 注册表，禁止 emoji、禁止模板里手写 `<svg>`（G3 / I1 / I3）。
- 弹窗宽度只用 `AppleModalSize`（`size="small|compact|regular|medium|large|xlarge"`），不要传 `width`（G29）。
- 表格密度用 `density`，不要写 `size`（R18，prop 已改名）。
- 收藏状态只有 `ApFavoriteButton` 一处实现：实心橙=已收藏、描边灰=未收藏，位置固定在 `ApEntityCard` 的 `#actions`（G14）。
- 进度条、分段控件、面包屑、菜单表面各只有一个实现（`ApProgress` / `ApSegmentedControl` / `ApBreadcrumb` / `menuSurfaceStyle()`）——第二份实现会被 G22–G29 判红，清单见 [one-implementation.md](one-implementation.md)。

## 5. 动效与手感

- 时长只准 `var(--apple-dur-*)`（唯一事实源 `BASE_DURATION_MS`，G32）；写死秒数会让个性化「动效强度」那一档调不到它；
- 缓动曲线只住在 `AppleEasing` 一处（G30）；
- 无交错入场、无整卡 `scale` 抖动；hover 只改描边与底色（G1）；
- 动效必须能被 `prefers-reduced-motion` 与个性化档位削弱：CSS 侧由 `data-motion` + `--apple-motion-scale` 驱动，JS 侧用 `scaledMs()` / `scaledResponse()` 换算同一个倍数，**不许各自 `matchMedia`**；
- 版面人格（`persona=signature`）的入场编排只准出现在登记过的公开版面（登录 / 授权登录 / 安装 / 分享 / 整页错误，G34），产品内页出现即红。

## 6. 断点与响应式

- 断点唯一出口 `utils/responsive.ts`（数据源 `AppleBreakpoint`）；不要在组件里写 `@media` 或 `window.innerWidth` 判断。
- `isMobile` 的界是 `<641px`。移动端典型形态是抽屉：AI 工作区的会话列表在移动端变 260px 左抽屉（标题「会话列表」），桌面是 220px 固定侧栏。
- 触控目标：控件高度阶梯 `AppleInputSize`（32/38/44），按钮 `AppleButtonSize`；并排的按钮/输入/分段必须同档同高（G12）。

## 7. i18n

- 语言只有 `en-US` 与 `zh-CN`（`web/src/i18n/`）。**视图里不许出现中文字面量**，文案进语言包；`web/tests/i18n-sync.test.ts` 保证两份键集同步。
- 后端返回的用户文案走 `message_key`：`api/client.ts` 的 `serverMessage()` 在构造 `ApiError` 时按当前语言替换；认不出键才退回服务端原文，且先用 `i18n.global.te()` 探一次（直接 `t()` 会把 `server.xxx` 印到界面上）。
- 品牌名**不是译文**：语言包里不许把 `Citebase` 当可翻译项（`web/tests/brand-closure.test.ts` 拦）；模板里的 `{app}` 占位符每个调用点都必须传参。
- 英文档下的"看得见的中文"由运行时巡检 `check:i18n-runtime` 量：它会把 `/api/` 响应体与节点文本比对，把"用户数据"（空间名、文档标题）从"代码文案"里分出来——代码文案预算为 0。

## 8. 品牌与静态外壳

`APP_NAME` 在 `utils/core/app-info.ts` 是唯一落点。构建期就定死、JS 起来之前就会被看到的四处必须等于它：

1. `index.html` 的 `<title>` 与 meta description；
2. PWA manifest 的 `name` / `short_name`；
3. 两枚图标的 `aria-label`；
4. 页面字标 / 登录页大标题 / 移动端头部 / 分享页脚（都取 `GET /api/v1/site`）。

运行时站点名由 `utils/core/site-brand.ts` 消费 `/api/v1/site`，`description` 为空时退回语言包的一句话定位，并用 `applySiteMetaDescription()` 盖掉 `index.html` 的静态默认值——换语言时重写同一处。

## 9. 状态与路由

- Pinia store 按领域拆；`stores/app/ui.ts` 是外观、材质、个性化档位的**唯一行为落点**（读写 localStorage、推服务端、写 DOM 属性），组件不自己存这些值。
- 路由拆三份：`routes-main.ts`（登录后主界面）、`routes-admin.ts`、`routes-public.ts`（分享页/登录/安装）。页面组件一律 `component: () => import('@/views/...')` 懒加载。
- 路由 `meta.group` 决定面包屑的分组父级，与侧边栏同名分区保持一致。

## 10. 测试与门禁

- 单测：Vitest + `happy-dom`，`globals: false`——测试文件必须显式 `import { describe, it, expect } from 'vitest'`；命名 `*.test.ts` / `*.spec.ts`，与源码同目录或放 `tests/`。
- E2E：Playwright（`npm run e2e`）。
- 提交前：`cd web && npm run ci`（lint → type-check → `check:style` 七道 → `check:i18n-msg` → vitest → build）。
- 只改样式/主题：至少 `npm run check:style`；只改布局：`npm run check:layout`；任何浏览器看得见的东西：`npm run check:geom` 或 `snap`（需要 :3000/:3001 在跑）。
- 门禁的规则族与"怎么证明门禁会红"见 [static-gates.md](../gates/static-gates.md) 与 [runtime-inspectors.md](../gates/runtime-inspectors.md)。
