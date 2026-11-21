# 静态门禁（check:style 七道）

> 这些门禁只读源码，不开浏览器，秒级可跑。运行时那批见 [runtime-inspectors.md](runtime-inspectors.md)。
> 规则编号写在每个脚本首部的注释里，**编号本身就是契约**：跳过号说明那条已合并或删除，不要复用旧号。

## 1. 一道命令，七个守卫

```bash
cd web && npm run check:style
```

等价于按顺序跑：

| 顺序 | 脚本 | 管什么 | 规则族 |
|---|---|---|---|
| 1 | `check-style-tokens.mjs` | token 注册表与用法：`cv()` 引用必须已注册、禁止 `var(--apple-x)15` 这类 hex alpha 拼接、禁止裸 `zIndex` 数字、像素键只降不升、`<style>` 必须 `scoped`、模糊必须走 `cv('glass-blur')` | —（项 1–7） |
| 2 | `check-theme-contract.mjs` | 主题契约：每套主题覆盖全部必填 token、主题 id 三处一致、材质/阴影不许绕过 token、黑白原色只准在令牌层、表面阶梯不许回潮别名 | T1–T17 |
| 3 | `check-icon-consistency.mjs` | 图标：注册表完整性、禁止手绘 `<svg>`/emoji/glyph、按钮图标 16、尺寸必须落 `AppleIconSize`、≥24px 必须解描边 | I1–I9 |
| 4 | `check-design-doctrine.mjs` | 设计教条：装饰机制不许复活、营销级大字号白名单、表面阶梯、渐变必须主题化、圆角/字号不许写死、全大写 kicker、收藏开关唯一实现、动效时长与强调色前景 | G1–G34 |
| 5 | `check-contrast.mjs` | WCAG 对比度：按主题差异表把 `color-mix`/`var` 求值成真实 sRGB，再按 alpha 合成到实际背景上算比值 | —（阈值见 §3） |
| 6 | `check-glass-layers.mjs` | 玻璃层普查：每处 `backdrop-filter` 走令牌、每处半透明壳面交代"压在谁上面" | —（普查为 0 判失败） |
| 7 | `check-layout-contract.mjs` | 布局契约：页面容器、版心、compact、`100dvh`、面包屑唯一、网格地板、壳层表面、页头锚点、单轴滚动 | R1–R19 |

`npm run ci` 的顺序是：`lint:check` → `type-check` → `check:style`（七道）→ `check:i18n-msg` → `vitest` → `build`。

## 2. 规则族里最常撞上的几条

**主题（T）**

- T2：每套主题必须覆盖全部契约键，且不得写未注册的键——加外观只改一半，编译就报；
- T7：JS 里不许出现主题 id 字面量比较（`if (theme === 'dark')` 一律换成 `isDarkThemeId()` / `supportsGlassClear()`）；
- T8：`AppleShadow.*` / `AppleGlassBlur.*` 只准在 `apple-tokens.ts` 里出现；
- T10/T11：强调色/遮罩上的文字用 `on-accent`；黑白原色只准在令牌层（邮件模板在 `RAW_COLOR_EXEMPT` 里带理由登记）；
- T12：一种颜色只有一个名字——`bg-primary`、`glass-bg`、`pure-white-canvas` 这类别名不得回潮。

**设计教条（G）**

- G1：`ApGradientCard` / `format/gradient` / `Sparkles` / 交错入场不许复活；
- G2：`heroDisplayXL/L`、`sectionDisplay` 这类展示级大字只准在错误页/登录页/安装页（白名单）；
- G3：UI chrome 禁 emoji 作图标；G13：单图标按钮必须 `icon-only`；
- G14：收藏开关只有 `ApFavoriteButton`；G22：进度条只有 `ApProgress`；G29：`ApModal` 只准 `size=`；
- G32：动效时长只准 `var(--apple-dur-*)`；G33：实心强调色上的前景用 `on-blue`；G34：入场编排只准在登记的公开版面。

**布局（R）**

- R1 页面必须用 `ApPageContainer`；R2 版心必须显式；R3 表格主导页 1600；R4 非全屏页 `compact`；
- R6 只允许 `100dvh`；R9 正文 680 行度量；R10/R11 `AppleGridFloor` 四档 + `entity` 等高；
- R12/R13 壳层表面阶梯、角色不许被壳层之外复述；R14 壳层三条横边共用 `shellBarHeight`；
- R15 页头必须挂 `data-ap-page-title`；R16 主对象档只给真正的主角；R18 `ApTable` 用 `density`；R19 单轴滚动走 `scrollSingleAxis`。

**图标（I）**

- I1 注册表完整性（`<ApIcon name="X">` 必须是 `icons.ts` 的键）；I3 模板禁 `<svg>`；I5 按钮图标 16；I7 禁 emoji；I8 ≥24px 必须解描边；I9 裸图标标签必须 import/全局注册。

完整清单以脚本首部注释为准；上面只列高频项，不是全量。

## 3. 对比度阈值（`check-contrast.mjs`）

不按"看起来够不够深"判，按 WCAG 相对亮度比：

| 场景 | 阈值 |
|---|---|
| 正文 | ≥ 7（AAA） |
| 次级 / 三级文字 | ≥ 4.5（AA） |
| 四级（辅助文字、图标） | ≥ 3（WCAG 1.4.11） |
| 强调色上的文字 | ≥ 3（按钮都是 14px+ 半粗） |
| 色相文字压同色浅染（徽章/选中行/chip） | ≥ 4.5 |

浮层自适应还有两端与回落值一起钉：alpha 地板/天花板、浅色与深色下的极值外部内容。判据值从 TS 源码现编译取回（`lib/ts-source.mjs`），**工装里不写第二份数字**——否则改了令牌而门禁还在用旧值，就是"永远绿"。

## 4. 基线与棘轮

不是所有违规都能一次清零，所以存量登记为**只降不升**的基线，新增零容忍：

| 基线 | 当前值 | 文件 |
|---|---|---|
| `localStyleClones`（.vue 本地样式对象与 `styles/**` 重复） | 127 | `check-style-tokens.mjs` |
| `spacePixelKeys` / `radiusPixelKeys`（使用点） | 22 / 15 | 同上 |
| `spaceRegistryKeys` / `radiusRegistryKeys` | 31 / 20 | 同上 |
| `serverMessageUnmigratedBaseline` | 0 | `tools/repocheck/server_message_test.go` |
| `serverMessageFieldBaseline` | 43 | 同上 |

整改一处就把数字下调一处；**上调等于给自己发新债**，评审直接拒。

## 5. 提交时到底跑了什么

有钩子时，`.husky/pre-commit` 只跑：ESLint（error 级）→ `check-style-tokens` → `type-check` → `vitest`。也就是说：

- 主题契约、设计教条、对比度、玻璃普查、布局契约**都不在提交时跑**；
- 浏览器巡检一道都不跑。

所以"提交成功"不等于"门禁全绿"。改前端后至少手动 `npm run check:style`；`HUSKY=0 git commit` 只能用来跳过单次误报，不要用来跳过真失败。

> 当前这份检出里没有 `.husky/` 与 `.github/` 目录（`.gitignore` 只忽略 `/.husky/_/`）。CI 从未在这个远端触发、钩子文件也不在库内的历史原因见 [../decision-log/2026-10-07-ci-and-module-path.md](../decision-log/2026-10-07-ci-and-module-path.md)。在没有钩子的机器上，`make ci` 与 `npm run ci` 是唯一强制。

## 6. 写门禁的唯一要求：证明它会红

新增或收紧任何门禁，必须：

1. 注入一条它该抓的违例；
2. 确认它变红（不是"看起来应该红"）；
3. **按行改回去**撤回注入，不要 `git checkout`——那会连你未提交的工作一起丢。

反向验证套件是 `rv-design-gates.mjs`（`npm run check:gates:rv`），里面登记了 76 条注入断言；几个几何/图标/手势门禁另有 `--selftest`。这个仓库反复出现过"门禁永远绿"的事故：失效选择器、`\b` 把连字符名也算命中、`Set` 过不了 `page.evaluate` 的结构化克隆、退出码算了却没置——每一种都安静地报了几周"0 违规"。

## 7. 常见失败怎么读

| 报错前缀 | 先看哪 |
|---|---|
| `T2 主题 X 缺 ...` | 是不是新加了外观但没覆盖契约键 |
| `G6 ... 写死圆角/字号` | 换成 `AppleRadius.*` / `AppleType.*` |
| `R3 ... 版心不是 1600` | 这个页面是不是表格主导页 |
| `R11 entity 网格不等高` | 补 `gridAutoRows:'1fr'` 或 `appleCardGrid('entity')` |
| `I1 ... 不在注册表` | `icons.ts` 补键，或换成注册表里已有的图标 |
| `对比度 3.2 < 4.5` | 先确认"文字实际压在哪个表面"，再换 `{hue}-text` / `on-{hue}` 档 |
