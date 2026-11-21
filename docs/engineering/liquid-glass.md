# 液态玻璃材质（Liquid Glass）

> 主题两轴中的 `material=glass` 这一支。外观与档位口径见 [theme-system.md](theme-system.md)；
> 校验工具的用法见 [../gates/static-gates.md](../gates/static-gates.md) 与 [../gates/runtime-inspectors.md](../gates/runtime-inspectors.md)。

## 1. 它是什么

玻璃不是"加个半透明背景"，而是**三层结构**：

| 层 | 作用 | 落点 |
|---|---|---|
| 环境层 | 被折射/模糊的底色（渐变或图片），随视口与主题走 | `themes.ts` 的 `GLASS_ENV_BASE_COLOR` 与材质差异表 |
| 面板层 | 卡片、输入面等"贴着内容"的表面 | `surface-panel` / `surface-field`，模糊走 `--apple-blur-panel`（默认 `none`） |
| 浮层/壳层 | 壳面（顶栏、侧栏）与真浮层（菜单、弹窗、抽屉） | `surface-chrome` / `surface-elevated` / `surface-float`，模糊走 `--apple-glass-blur` |

**只有壳层、真浮层与按钮控件档付 `backdrop-filter`**。卡片、正文块这类"流水面"不付——它们身后没有内容滚过去，付一次模糊只是白烧 GPU。这条预算以前只活在注释里，现在由 `check:glass` 按"声明站点"执行。

## 2. 允许与禁止

允许：

- `cv('glass-blur')` / `cv('blur-panel')` 两个材质令牌；
- 浮层 alpha 落在 `[GLASS_FLOAT_ALPHA_FLOOR, GLASS_FLOAT_ALPHA_CEIL]` = **`[0.40, 0.94]`** 之间：低于地板读作半透明，等于 `1.0` 则"该主题下没有玻璃"；
- Clear 变体（`supportsGlassClear()` 判定），它改的是 overlay 的透明度与边框，不是另起一套主题；
- 镜面内高光：位置由 `--apple-light-x/y` 跟随指针（`useSpecularLight`），无指针或减弱动效时静态。

禁止：

- **把 SVG 滤镜 `url()` 拼进 `backdrop-filter`**：Chromium 会整条声明不渲染，连 `blur` 一起失效——看上去像"玻璃没生效"，实际是语法被吞了。折射位移只能做在环境层，不进 `backdrop-filter`；
- 写死 `blur(20px)`：切到 `solid` / 纸感时它照样付一次 GPU 模糊，外观上什么都看不出来（`check:glass` 直接拦）；
- 用 `AppleGlassBlur.*` 字面常量（T8）；
- 把 `surface-elevated` 当静态浮层表面用：它会在滚动时跳到 `0.88/0.96`，浅色读作白板、深色读作灰板；浮层要用 `surface-float`；
- 给 `surface-knob` 放文字（见 theme-system §3）。

## 3. 可读性地板：玻璃最先坏的地方

半透明表面上的文字对比度取决于"它身后恰好是什么"。静态门禁按令牌 + alpha 合成算一遍（正文 ≥7、次级 ≥4.5、辅助 ≥3），但运行时还会多出三类只在渲染时成立的情况：

1. **一屏同叠了几层实时模糊**：声明站点 17 处 ≠ 同屏 17 层，也 ≠ 2 层——`v-for` 出来的卡片会把一处声明放大成二十层（2026-10-15 实测 23 层，基线 `MAX_BLUR_LAYERS = 16`）；
2. **玻璃压玻璃**（嵌套深度 > 0）：静态上两个选择器谁套谁看不出来；
3. **滚动边缘效应真的在触发吗**：内容滚到浮层壳底下时，壳有没有按 `shellSurfaceKey()` 加厚。

`check:glass:runtime` 就是为这三件事开的浏览器；它没登录凭据起不来（故意如此——拿一个"登录失败兜底页"当样本，报表会绿得毫无意义）。

## 4. 加厚时机：`useScrollEdge`

静止时壳面通透，内容滚到底下时升到 `surface-elevated` 档。判据是**身后有没有正在滚动的内容**，不是"看起来像不像浮层"：

- 顶栏那一处的 alpha 在运行时由 `useShellGlassLift` 按实测底色改写，令牌值是"量不出来"时的兜底；
- 这不是"再加一层半透明"——它是把同一档角色的值往上抬，所以 `check:theme-rounds` 第④条（运行时对比度）能把它抓出来：如果加厚把正文压到 7:1 以下，说明有东西在运行时覆盖令牌。

## 5. 玻璃与实心的差异必须"看得见"

`check:theme-rounds` 第②条量的是：同一外观的 `solid` 轮与 `glass` 轮，顶栏算出来的背景必须不同，且玻璃轮真的带 `backdrop-filter`。否则"接了材质令牌"只是 JS 里多存了一个值——这是 2026-10-16 实测过的坑（主题键被拼成 `citebase:citebase:ui:theme`，八轮全在默认档上量，报表却按 `--themes` 标成玻璃档）。

所以每轮巡检的第①条永远是：**DOM 上的 `data-theme` / `data-material` / `data-glass` 等于本轮请求的那一档**。没有第①条，后面三条测的都是同一档四次。

## 6. 自查清单

- [ ] 这个表面身后有内容滚过吗？没有就别付 `backdrop-filter`。
- [ ] 模糊值来自 `cv('glass-blur')` / `cv('blur-panel')`，不是手写 `blur()`。
- [ ] 半透明壳面交代了"压在谁上面"：用浮层档、或走 `shellSurfaceKey()`、或在 `SHELL_EXEMPT` 里带理由登记。
- [ ] 浮层 alpha 在 0.40–0.94 之间。
- [ ] 把主题切到 `light+solid` 与 `light+glass`，顶栏观感确实变了（不是只有 class 变了）。
- [ ] 跑了 `npm run check:glass`；改动了同屏叠层/滚动边缘时再跑 `check:glass:runtime`。
