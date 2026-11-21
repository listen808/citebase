/**
 * 强调色档位（个性化批3，2026-10-16）。真相源在这里，服务端为校验持有第二份清单
 * （`internal/model/user/preferences.go` 的 `AccentIDs`），由
 * `tools/repocheck/preference_contract_test.go` 逐条比对。
 *
 * **为什么是固定色板而不是自由取色器**：`--apple-on-blue`（实心强调色上的前景）不是 CSS 能算的
 * —— 浏览器不能按亮度分支，所以它由 `readableForegroundVars()` 在注入时按色值算出来。
 * 一旦允许任意 hex，就有一条没人验过的路径：浅色板（例如 `#ffe100`）上白字只有 1.4:1。
 * Apple 也是同一取舍：系统给一排色卡 + graphite + 「自动」，而不是一个取色器。
 *
 * **为什么默认档是 `auto`**：`auto` = 用每一套外观自己设计的那个强调色
 * （浅 `#0066f5`、深 `#0a84ff`、纸感 `#d9730d`、编辑部 `#b3261e`）。
 * 品牌定位是「技术中立」，所以这一层不改默认观感，只是在用户明确选过时才覆盖。
 * 等价物是 Apple 的 Multicolor / Automatic。
 *
 * **色板取法**：8 档在 OKLCH 上等亮度、等彩度，只有色相 H 不同（浅色外观 L≈0.47、
 * 深色外观 L≈0.60 —— 为什么深色不是"更亮就行"，见下面 ACCENT_REGISTRY 上那段）。
 * 这样"换强调色"不会让某个色相在浅底上把同色浅染上的文字压到不可读 ——
 * 对比度是全色板一次性过 `check:contrast`，而不是逐色碰运气。
 * 存的仍是 **hex 纯色**：`readableForegroundVars()` 与 `check:contrast` 都按 hex/rgba 算，
 * 不依赖浏览器色彩引擎（本机只实测过 Chromium 的 `color-mix(in oklch, …)`，另两个内核没量过）。
 */

import { contrastInkOn } from '../../utils/core/apple-tokens';

/** 每档给两个值：浅色外观用的（要压得住浅底）与深色外观用的（要在深底上够亮） */
export interface AccentDef {
  id: string;
  /** 语言包里的名字键 */
  labelKey: string;
  light: string;
  dark: string;
}

export const ACCENT_AUTO = 'auto';

/** 默认档：不覆盖任何颜色。改这一行等于改全站新访客的强调色，先跑 `check:contrast`。 */
export const DEFAULT_ACCENT: AccentId = ACCENT_AUTO;

/**
 * 8 档色板。浅色外观一档、深色外观一档，同一色相两个亮度。
 *
 * **8 档都是同一组约束逼出来的取值**，不是审美挑选：门槛来自门禁里四条一起判的断言
 *   · 恒白（`on-accent`，主按钮的标签色）压在实心强调色上 ≥3
 *   · 同色浅染上的文字（`blue-text` 压 `blue-bg-subtle`）≥4.5
 *   · 强调色本身对面板（焦点环/描边，WCAG 1.4.11 非文本）≥3
 *   · 实心强调色上的前景（`on-blue`，由 `contrastInkOn()` 算）≥4.5
 * 过程存照：深色候选最初取 OKLCH L≈0.72，白字压上去只剩 2.3~2.6 → 降到 0.60；
 * 浅色候选最初 L≈0.52，紫/粉/红/橙这四档在白字那一格只有 2.79~2.90 → 再降到 0.47。
 * **两个方向不能各自调**：更亮会丢白字，更深会丢焦点环可见度，只有门禁那四条能同时判住。
 * 以后要动任何一个 hex，先跑 `npm run check:contrast`。
 */
export const ACCENT_REGISTRY = [
  { id: 'blue', labelKey: 'chrome.accentBlue', light: '#1256b2', dark: '#427fd8' },
  { id: 'purple', labelKey: 'chrome.accentPurple', light: '#7b3697', dark: '#a161bd' },
  { id: 'pink', labelKey: 'chrome.accentPink', light: '#972767', dark: '#c0558d' },
  { id: 'red', labelKey: 'chrome.accentRed', light: '#a12628', dark: '#ca5551' },
  { id: 'orange', labelKey: 'chrome.accentOrange', light: '#9a3700', dark: '#c26300' },
  { id: 'yellow', labelKey: 'chrome.accentYellow', light: '#7b5400', dark: '#a07b00' },
  { id: 'green', labelKey: 'chrome.accentGreen', light: '#00711b', dark: '#2e9848' },
  { id: 'teal', labelKey: 'chrome.accentTeal', light: '#007376', dark: '#009a9c' },
] as const satisfies readonly AccentDef[];

/**
 * 合法取值清单。**`auto` 写在数组里**，不住另一个常量 ——
 * 上一批 `auto` 住在 `PrefThemeAuto` 里，跨语言判据只比了 `ThemeIDs`，第一次跑就红。
 */
export const ACCENT_IDS = [ACCENT_AUTO, ...ACCENT_REGISTRY.map((a) => a.id)] as const;
export type AccentId = (typeof ACCENT_IDS)[number];

export function isAccentId(v: unknown): v is AccentId {
  return typeof v === 'string' && (ACCENT_IDS as readonly string[]).includes(v);
}

/**
 * 这一档在当前外观下应该覆写成什么颜色；`auto` 返回 null = **不覆写**，
 * 让外观自己那张调色板说话（这也是"删掉覆写"和"覆写成外观色"必须区分开的原因：
 * 前者恢复继承链，后者会把 `--apple-blue` 钉死在一个 hex 上，纸感/编辑部换回浅色时颜色不跟）。
 */
export function accentColorFor(id: AccentId, appearance: 'light' | 'dark'): string | null {
  if (id === ACCENT_AUTO) return null;
  const def = ACCENT_REGISTRY.find((a) => a.id === id);
  if (!def) return null;
  return appearance === 'dark' ? def.dark : def.light;
}

/**
 * 把选择写到 DOM 上：覆写 `--apple-blue` 本体 + 由它派生的实心前景 `--apple-on-blue`。
 *
 * 两枚一起写是必须的，不是保险：
 *  · `--apple-blue-selection` / `-bg-subtle` / `-text` / `callout-note-*` / `-focus-ring`
 *    都是 `color-mix(... var(--apple-blue) …)` 声明在 `:root` / 主题块上，
 *    而 `:root` 就是 `documentElement` —— 同一元素上行内覆写追得回来（theme-system.md
 *    记的「一个 CSS 事实」讲的是**后代**覆写追不回来，别把它当成这里也不能覆写的理由）；
 *  · 但 `--apple-on-blue` 是 JS 按亮度算出来的常量，不跟着 `--apple-blue` 走。
 *    只覆写前者的症状：按钮底变成用户选的紫，白字还在按原来的蓝算 —— 压不住的色相上直接不可读。
 *
 * `data-accent` 恒写（含 `auto`）：缺席只该意味着"JS 没跑"，与批1 那三档同一个口径。
 */
export function applyAccent(id: AccentId, appearance: 'light' | 'dark'): void {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  root.setAttribute('data-accent', id);
  const hex = accentColorFor(id, appearance);
  if (!hex) {
    root.style.removeProperty('--apple-blue');
    root.style.removeProperty('--apple-on-blue');
    return;
  }
  root.style.setProperty('--apple-blue', hex);
  root.style.setProperty('--apple-on-blue', contrastInkOn(hex));
}
