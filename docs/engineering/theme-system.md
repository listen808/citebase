# 主题系统：外观 × 材质

> 真相源：`web/src/styles/theme/themes.ts`（主题与材质）、`apple-tokens.ts`（token 注册表与注入）、
> `personalization.ts`（行为档位）、`accents.ts`（强调色）。本文解释结构，数值以代码为准。
> 材质相关的实现细节见 [liquid-glass.md](liquid-glass.md)。

## 1. 两条正交轴，不是一张主题列表

主题 = **外观（base）× 材质（material）**：

| 轴 | 取值 | 说明 |
|---|---|---|
| base | `light`、`dark`、`editorial`、`warm-paper` | 各是一套完整调色板。`editorial`（杂志）与 `warm-paper`（暖纸）是阅读类扩展外观 |
| material | `solid`、`glass` | `glass` 只对 `light` / `dark` 开放：纸感与编辑部本来就 `glass-blur: none`，"纸没有玻璃" |

这是 Apple 的真实模型：**Liquid Glass 是一种材质**，同时有浅/深形态，可以叠在任何外观上，而不是与浅/深并列的第三种配色。旧的独立主题 `liquid-glass` 已降级为别名（`light` + `glass`），`LEGACY_THEME_ALIAS` 负责读历史存储，避免老用户升级后丢主题。

DOM 上有三个属性描述"现在是哪一档"：`data-theme`（base）、`data-material`、`data-glass`（Clear 变体）。三者必须与实际渲染一致——运行时巡检 `check:theme-rounds` 的第①条就是量这个"真的切过去了"。

## 2. 主题契约：缺一个 token 就编译不过

`themes.ts` 的 `THEME_CONTRACT_KEYS` 列出**每套主题都必须显式给值**的 token（当前约 72 个，含强调色、文字四级、表面阶梯、边框、阴影、材质模糊、圆角与断点等），由 `satisfies` 约束成"必须是已注册的变量名"：

- 缺一套主题的值 → 类型报错；
- 写错变量名 → 类型报错；
- 契约外的键沿用 `:root` 浅色默认或自动派生，不需要每套主题抄一遍。

**派生族**（`*-bg-subtle`、`*-light`、`callout-*`、`*-selection`、`*-focus-ring`）在 `:root` 用 `color-mix(in srgb, var(--apple-纯色) N%, transparent)` 表达：主题只改纯色，整族自动跟随，不存在"写两遍"的机会。代价是**它们必须留在 `:root`**（`data-theme` 也挂在 `<html>` 上）——挪到某个子容器上作用域化，派生就不会跟着主题变了。

注入顺序：`injectCSSVariables()` 把浅色默认值写进 `:root` → 主题差异表按 `data-theme` 覆盖 → 材质与透明变体再覆盖。`--apple-on-blue` 这类"压在实心色上的前景"**不是 CSS 能算的**（浏览器不能按亮度分支），由 `readableForegroundVars()` 在注入时按色值算出来，这也是为什么强调色只能是固定色板（见 §4）。

## 3. 表面阶梯：五档角色，逐主题给值

| 角色 | 语义 | 典型落点 |
|---|---|---|
| `surface-chrome` | 壳层：顶栏 + 侧栏 | 页面最外层框架 |
| `surface-canvas` | 页面底：卡片之下的画布 | 内容区背景 |
| `surface-panel` | 卡片 / 面板 / 区块 | 内容卡、表单面板 |
| `surface-elevated` | 壳层加厚：**内容从它底下滚过去**时的那一档 | 顶栏 under 态、吸顶表头 |
| `surface-float` | 浮层面板 | 菜单、弹窗、抽屉、气泡、聚光搜索 |
| `surface-field` | 输入控件的凹陷面 | input / textarea / select 触发器 / 代码块内小控件 |

后两档在实心主题下同色、在玻璃下分开，因为**它们身后的东西不是一回事**：`elevated` 要扛任意滚过来的内容（极值纯黑），`float` 只扛本应用环境（极值一张暗图）。浅色下 canvas `#f5f5f7`、卡片与壳层 `#ffffff`，层级靠"台阶 + 1px 发丝边框"而不是阴影——静置卡片不投影是这套设计的地板（唯一例外是玻璃材质的镜面内高光）。

`surface-knob`（开关滑块）是抬起的实心面，**前提是上面不许有文字**：玻璃下它是 `rgba(255,255,255,.96)`，把 `ApSegmentedControl` 的选中块搬上去会让深色文字对比度掉到 1.09:1（实测过）。

## 4. 强调色：8 档色板 + `auto`，不做自由取色器

`accents.ts` 给 8 档色相，每档两个亮度（浅色外观一个、深色外观一个），在 OKLCH 上等亮度等彩度、只有色相 H 不同。默认值是 `auto`——用每套外观自己设计的强调色（浅 `#0066f5`、深 `#0a84ff`、纸感 `#d9730d`、编辑部 `#b3261e`），不改默认观感。

为什么不是取色器：`--apple-on-blue`（实心强调色上的前景）由色值算出来，一旦允许任意 hex，就有一条没人验过的路径——浅色板（如 `#ffe100`）上白字只有 1.4:1。固定色板的代价是"选不了任意色"，换来的是**整个色板一次过 `check:contrast`**，而不是逐色碰运气。

## 5. 个性化五档

| 档位 | 取值 | 默认 | 作用 |
|---|---|---|---|
| 动效强度 `motion` | `auto` / `full` / `soft` / `reduced` | `auto` | 时长倍数 1 / 0.6 / 0.0001；`reduced` 另加"不位移、只淡入淡出"的通配规则 |
| 对比度 `contrast` | `auto` / `standard` / `more` | `auto` | `more` = 近实心 + 把描边一族画实 |
| 透明度 `transparency` | `auto` / `full` / `reduce` | `auto` | `reduce` = 玻璃退回该外观的不透明表面 |
| 强调色 `accent` | `auto` + 8 档 | `auto` | 覆盖强调色（见 §4） |
| 版面人格 `persona` | `plain` / `signature` | `plain` | 公开版面的入场编排；**没有 `auto`**，因为系统里没有"要不要入场"这个开关 |

前三档与 `accent` 的 `auto` = "这台设备没表过态"，行为完全由系统信号决定；`persona` 的 `plain` = 今天的样子（新访客不该被替着做决定）。

系统信号是**三个独立信号**，不要合并：`prefers-reduced-motion`、`prefers-contrast: more`、`prefers-reduced-transparency`——用户可能只开其中一个。

**应用内的显式选择可以盖过系统信号**（`full` 压 `prefers-reduced-motion`）。理由：默认是 `auto`，没表过态的人继续尊重系统；显式选 `full` 是用户在这个应用上下文里的一次表态，且随时能改回去。Apple 的原话是 "Make motion optional"，可选性本来就是双向的。代价是账号同步会把这台的选择带到那台，所以界面上必须显示"**当前生效的是哪一档**"，而不只是"你选了什么"（`ApThemeMenu` 负责）。

写 DOM 时三条属性**恒写**（`data-motion` / `data-a11y-contrast` / `data-a11y-transparency`）——缺席只能意味着一件事："JS 还没跑 / 没表过态"，CSS 里的 `:not([data-...])` 护栏才能放心让系统信号接管。若缺席同时表示"选了默认档"，系统开关与用户选择冲突时谁赢就没法判了。

CSS 侧与 JS 侧必须用同一个倍数：CSS 由 `--apple-motion-scale` 乘，JS 用 `scaledMs()` / `scaledResponse()`。弹簧的 `response` 是**除以**倍数（倍数 < 1 表示更慢，response 越小到位越快），写反的症状是"选了柔和档，手势反而更冲"。

## 6. 偏好跟账号走

外观 × 材质 × 个性化以前只住 localStorage，换设备、清缓存就全丢——"Citebase 支持四套外观"在产品上是真的，在账号上却是假的。现在它是账号属性：

- **存储**：`aw_user.preferences`（TEXT，存校验后的 JSON，硬上限 2048 字节）。不是拆成列——这些值永远一起读写、不需要按值查询，加一档只是同一列 JSON，不需要 DDL。
- **唯一校验入口**：`internal/model/user/preferences.go` 的 `NormalizePreferences` / `MarshalPreferences` / `ParsePreferences`。服务端认得的清单（`ThemeIDs` / `MaterialIDs` / `MotionIDs` / `ContrastIDs` / `TransparencyIDs` / `AccentIDs` / `PersonaIDs`）**不是真相源**，只是为校验持有的第二份，由 `tools/repocheck/preference_contract_test.go` 与前端逐条比对。加一档只改一边的用户体验不是"新档不能用"，而是**用户在另一台设备上的选择被当非法值丢掉**。
- **接口**：`GET /auth/me` 带 `preferences`（`null` = 该账号从没同步过，**不许合成成默认档**——那样每次登录都会盖掉本地选择）；`PUT /auth/me/preferences` 是 self-service，登记在 `rbac_coverage_test.go` 的 `sensitivePublic`。
- **前端同步判据**：`utils/core/preference-sync.ts` 的纯函数 `decideSync`——本地刚改过就直接推（不比时间戳）；推送失败记 `ui:pref-pending` 下次补推；采纳服务端值期间不回推（防回声）。
- **老数据行的例外**：外观/材质/透明变体认不得 → 整份判"没配过"；新格缺失或认不得 → 用本机值补。把后者也判成非法，症状是升级到批 1–4 之后这些账号连主题都同步不了（老行看起来像坏数据）。
- **载荷八格**：`theme` / `material` / `glass_clear`（批前）+ `motion` / `contrast` / `transparency`（批1–2）+ `accent` / `persona`（批3–4）。

## 7. 一条 CSS 事实（踩过的学费）

自定义属性的 `var()` 在**声明它的那个元素上**就替换完了，后代覆写追不回来。所以动效倍数是"**行内覆写令牌本身**"（`root.style.setProperty('--apple-motion-scale', ...)`），而不是在令牌里再嵌一层 `var(--apple-motion-scale)`——后者看起来更"干净"，但乘出来的值在子元素上不会跟着变。

## 8. 加一档 / 加一套主题的检查表

加外观：

1. `themes.ts` 的 `BASE_THEME_IDS` + `THEME_REGISTRY` 加一条；
2. 覆盖 `THEME_CONTRACT_KEYS` 全部键（缺一个编译不过）；
3. `highlight-tokens.ts` 登记 hljs 调色板与 Monaco 主题（T6）；
4. 前端 `personalization.ts`/`themes.ts` 与服务端 `ThemeIDs` 同步，跑 `preference_contract_test.go`；
5. `npm run check:style` + `npm run check:contrast` + `npm run check:theme-rounds -- --themes=all`。

加行为档（如再加一档动效）：

1. `personalization.ts` 加取值、默认值、`isXxxMode`、`resolveXxx`；
2. `ui.ts` 的读写与 DOM 属性接线；
3. 服务端 `XxxIDs` 同步 + 契约测试；
4. 涉及 CSS 的写选择器，不要写主题/档位 id 字面量的 `if`（T7 拦）。

## 9. 守这一层的门禁

| 门禁 | 阻断什么 |
|---|---|
| `check:theme`（T1–T12） | 契约缺键、主题漏覆盖、主题 id 字面量比较、材质/阴影绕过 token、别名回潮 |
| `check:contrast` | 文字压在实际背景上的 WCAG 比：正文 ≥7、次级 ≥4.5、辅助 ≥3、强调色上的字 ≥3、同色浅染上的字 ≥4.5 |
| `check:glass` | 每一处 `backdrop-filter` 是否走材质令牌、半透明壳面是否交代了"压在谁上面" |
| `check:theme-rounds`（运行时） | 八档外观×材质是否真的切过去、材质轴是否真的改变渲染、浮层 alpha 是否在 [0.40, 0.94]、运行时对比度是否过线 |
