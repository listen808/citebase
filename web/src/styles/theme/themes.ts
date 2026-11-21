/**
 * 主题注册表：所有主题（含暗色）都是「:root 默认值 + 一份差异表」的数据。
 *
 * 一致性靠三件事保证，而不是靠 review：
 * 1. THEME_CONTRACT_KEYS —— 每套主题都必须覆盖的 token；缺一个 key 直接编译不过。
 * 2. 派生族（*-bg-subtle / *-light / callout-* / selection / focus）在 :root 用
 *    color-mix(纯色, transparent) 表达，主题改纯色即整族跟随，不存在写两遍的机会。
 * 3. THEME_REGISTRY 是主题列表的唯一事实源，菜单/设置页/门禁都从它派生。
 *
 * 历史说明：docs/theme-design-spec.md（11 套蓝图）已在 commit b0a5e8aa 删除，
 * 不要再按那个路径去找 —— 已落地的组合只由下面的 THEME_REGISTRY × MATERIAL_REGISTRY
 * 加 supportsGlass() 决定，本文件就是唯一事实源。
 */
import type { AppleCssVarName } from '../../utils/core/apple-tokens';

/**
 * 主题 = **外观(base) × 材质(material)** 两条正交轴。这是 Apple 的真实模型：Liquid Glass 是一种
 * **材质**，同时有浅/深形态，叠在任何外观上，而不是与浅/深并列的第三种配色。
 *   base：light | dark | editorial | warm-paper —— 各是一套完整调色板
 *   material：solid | glass —— glass 只对 light/dark 开放（纸感/编辑部本就 glass-blur:none，"纸没有玻璃"）
 * 旧的独立主题 'liquid-glass' 降级为 legacy 别名 = light + glass（见 LEGACY_THEME_ALIAS）。
 */
export const BASE_THEME_IDS = ['light', 'dark', 'editorial', 'warm-paper'] as const;
/** 阅读类扩展主题：完整替代调色板，不是材质 */
export const EXTENDED_THEME_IDS = ['editorial', 'warm-paper'] as const;

export type BaseThemeId = (typeof BASE_THEME_IDS)[number];
export type ExtendedThemeId = (typeof EXTENDED_THEME_IDS)[number];
/** 主题 id 现在就是 base（外观）；材质走独立轴 */
export type ThemeId = BaseThemeId;

/** 材质轴：实心 / 液态玻璃 */
export const MATERIALS = ['solid', 'glass'] as const;
export type Material = (typeof MATERIALS)[number];
export const isMaterial = (v: unknown): v is Material =>
  typeof v === 'string' && (MATERIALS as readonly string[]).includes(v);

/** 全部合法 base id（按 UI 展示顺序） */
export const THEME_IDS = [...BASE_THEME_IDS] as const;

/** 旧存储/深链里的独立玻璃主题 → 双轴组合。读历史偏好时用，避免老用户升级后丢主题 */
export const LEGACY_THEME_ALIAS: Record<string, { base: ThemeId; material: Material }> = {
  'liquid-glass': { base: 'light', material: 'glass' },
};

/**
 * 主题契约：任何一套主题（暗色也算）都必须显式给值的 token。
 * 契约外的键沿用 :root 浅色默认或自动派生，不需要每套主题抄一遍。
 * satisfies 把这里限制成「必须是已注册的变量」，写错变量名同样编译不过。
 */
export const THEME_CONTRACT_KEYS = [
  // 强调色 / 链接
  '--apple-apple-action-blue',
  '--apple-body-link-blue',
  '--apple-high-luminance-link-blue',
  // 文字
  '--apple-near-black-ink',
  '--apple-text-primary',
  '--apple-text-secondary',
  '--apple-text-tertiary',
  '--apple-text-quaternary',
  '--apple-text-80',
  '--apple-text-48',
  '--apple-text-disabled',
  '--apple-secondary-neutral-gray',
  // 中性面 / 纸
  '--apple-pale-apple-gray',
  '--apple-light-gray',
  '--apple-gray',
  '--apple-gray-50',
  '--apple-pure-white-canvas',
  // 表面阶梯：壳层 / 画布 / 面板 / 壳层加厚 / 浮层面板五档必须逐主题显式给值，
  // 否则「内容区和上栏、侧栏颜色不一致」这类漂移无法在编译期被发现。
  // elevated 与 float 分档的理由见 apple-tokens.ts：一个扛任意内容（顶栏 under 态），
  // 一个只扛本应用环境（菜单/弹窗），合成一档就会为了前者的地板把后者过成白板。
  '--apple-surface-chrome',
  '--apple-surface-canvas',
  '--apple-surface-panel',
  '--apple-surface-elevated',
  '--apple-surface-float',
  '--apple-surface-field',
  // 主对象档：一页只有一个"用户是来看它的东西"（文档正文、分享正文）。
  // 它和 surface-panel 的区别是**成对的 elevation**（§12 越大的面越厚），
  // 所以两个键必须一起给值——只加底色不加阴影等于又造一个同层重名。
  '--apple-surface-primary',
  '--apple-shadow-primary',
  // 页面底
  '--apple-bg-secondary',
  '--apple-bg-tertiary',
  '--apple-bg-hover',
  // 面板 / 浮层
  '--apple-surface-knob',
  '--apple-on-accent',
  '--apple-overlay',
  // 描边
  '--apple-border',
  '--apple-border-light',
  '--apple-soft-border-gray',
  '--apple-mid-border-gray',
  '--apple-utility-dark-gray',
  // 纯色族（染色底 / 选中 / 焦点环都从这里的纯色派生）
  '--apple-blue',
  '--apple-green',
  '--apple-red',
  '--apple-orange',
  '--apple-yellow',
  '--apple-purple',
  '--apple-teal',
  '--apple-indigo',
  '--apple-pink',
  // 代码块
  '--apple-code-bg',
  '--apple-code-fg',
  '--apple-code-green',
  '--apple-code-blue',
  '--apple-code-purple',
  '--apple-code-orange',
  // 滚动条
  '--apple-scrollbar-thumb',
  '--apple-scrollbar-thumb-hover',
  // 材质层（背景色已并入 surface 阶梯，这里只剩「边缘」与「模糊」两件事）
  '--apple-glass-border-nav',
  '--apple-glass-blur',
  '--apple-blur-panel',
  // 形状
  '--apple-radius-card',
  '--apple-radius-control',
  '--apple-radius-panel',
  // 阴影族（暗色靠描边+内高光，纸感靠极浅暖影，玻璃靠镜面高光——差异最大的族）
  '--apple-shadow-card',
  '--apple-shadow-card-hover',
  '--apple-shadow-card-hover-refined',
  '--apple-shadow-modal',
  '--apple-shadow-dropdown',
  '--apple-shadow-level1',
  '--apple-shadow-level2',
  '--apple-shadow-level3',
  '--apple-shadow-subtle',
  '--apple-shadow-nav',
  '--apple-shadow-inner-focus',
  // 字族
  '--apple-font-display',
  '--apple-font-text',
] as const satisfies readonly `--apple-${AppleCssVarName}`[];

export type ThemeContractKey = (typeof THEME_CONTRACT_KEYS)[number];

/** 契约键必填 + 其余已注册键可选；写漏或写错变量名都会在类型检查阶段报错 */
export type ThemeOverrideMap = { [K in ThemeContractKey]: string } & {
  [K in `--apple-${AppleCssVarName}`]?: string;
};

/** base 主题元数据（UI 展示用）。label 简短，desc 说明气质，family 用于分组 */
export interface ThemeMeta {
  id: ThemeId;
  label: string;
  family: '基础' | '阅读' | '工程' | '现代' | '个性';
  desc: string;
  /** 渲染器（hljs / monaco / mermaid / canvas）按此选明或暗调色板 */
  appearance: 'light' | 'dark';
  /** 该 base 是否可叠加 glass 材质（只有 light/dark 有玻璃形态；纸感/编辑部没有） */
  supportsGlass?: boolean;
}

export const THEME_REGISTRY: readonly ThemeMeta[] = [
  { id: 'light', label: '浅色', family: '基础', desc: 'Apple 浅色基线', appearance: 'light', supportsGlass: true },
  { id: 'dark', label: '深色', family: '基础', desc: 'Apple 暗色，描边与内高光分层', appearance: 'dark', supportsGlass: true },
  { id: 'editorial', label: '杂志', family: '阅读', desc: '刊头横线 + kicker 章节题 + 编辑红', appearance: 'light' },
  { id: 'warm-paper', label: '暖纸', family: '阅读', desc: 'Oil Book 同款：暖白纸面 + 琥珀金 + 现代无衬线', appearance: 'light' },
];

/** 材质元数据（UI 展示用） */
export interface MaterialMeta {
  id: Material;
  label: string;
  desc: string;
}
export const MATERIAL_REGISTRY: readonly MaterialMeta[] = [
  { id: 'solid', label: '实心', desc: '不透明表面，Apple 基线材质' },
  { id: 'glass', label: '液态玻璃', desc: '清玻璃面板浮在渐变环境上，随外观折射' },
];

export function themeMetaFor(id: string | null | undefined): ThemeMeta | undefined {
  return THEME_REGISTRY.find((t) => t.id === id);
}

/** 该 base 能否叠加玻璃材质 —— 组件用这个判断，不写 base id 字面量 */
export const supportsGlass = (id: string | null | undefined): boolean =>
  themeMetaFor(id)?.supportsGlass === true;

/**
 * Clear（更透明）是 glass 材质的子档，只在 material=glass 且 base 支持玻璃时可用。
 * 旧签名收 theme id，现在材质是独立轴，改收 (base, material)。
 */
export const supportsGlassClear = (
  id: string | null | undefined,
  material: Material | null | undefined,
): boolean => material === 'glass' && supportsGlass(id);

export type ThemePreference = 'auto' | ThemeId;

export const isThemeId = (v: unknown): v is ThemeId =>
  typeof v === 'string' && (THEME_IDS as readonly string[]).includes(v);

export const isThemePreference = (v: unknown): v is ThemePreference =>
  v === 'auto' || isThemeId(v);

/** 暗色观感主题（hljs / monaco / mermaid / canvas 取色都走这里，禁止再写 === 'dark'） */
const DARK_IDS = THEME_REGISTRY.filter((t) => t.appearance === 'dark').map((t) => t.id);
export function isDarkThemeId(id: string | null | undefined): boolean {
  return !!id && (DARK_IDS as readonly string[]).includes(id);
}

const SF_DISPLAY =
  "'SF Pro Display', -apple-system, BlinkMacSystemFont, 'Helvetica Neue', 'PingFang SC', sans-serif";
const SF_TEXT =
  "-apple-system, BlinkMacSystemFont, 'SF Pro Text', 'SF Pro Icons', 'Helvetica Neue', Helvetica, Arial, sans-serif";
const SERIF = "Georgia, 'Songti SC', 'Noto Serif SC', 'Times New Roman', serif";
/**
 * 暖纸（Oil Book 同款）的字体栈：Plus Jakarta Sans 是现代几何无衬线，中文回落到
 * MiSans / HarmonyOS / Noto 与系统默认。不加载 webfont —— 本机没装 Plus Jakarta Sans
 * 时就自然落到 MiSans / 苹方，气质不变，不为一个字体去加一条网络请求。
 */
const WARM_PAPER_SANS =
  "'Plus Jakarta Sans', 'MiSans', 'HarmonyOS Sans SC', 'Noto Sans SC', 'PingFang SC', -apple-system, BlinkMacSystemFont, sans-serif";

/**
 * Apple 暗色：原先是 injectCSSVariables() 里一坨手写 CSS 串，现收进同一契约。
 * 派生族（染色底 / 选中 / 焦点环 / callout）沿用 :root 的 color-mix，只把强度调到 18%，
 * 因为同一纯色在黑底上的可见度低于白底。
 */
export const DARK_THEME: ThemeOverrideMap = {
  '--apple-apple-action-blue': '#0a84ff',
  '--apple-body-link-blue': '#2997ff',
  '--apple-high-luminance-link-blue': '#2997ff',

  '--apple-near-black-ink': '#ffffff',
  '--apple-text-primary': '#f5f5f7',
  '--apple-text-secondary': '#ebebf599',
  '--apple-text-tertiary': '#a1a1a6',
  '--apple-text-quaternary': '#8e8e93',
  '--apple-text-80': 'rgba(245,245,247,0.8)',
  '--apple-text-48': 'rgba(245,245,247,0.48)',
  '--apple-text-disabled': 'rgba(245,245,247,0.4)',
  '--apple-secondary-neutral-gray': '#ebebf599',

  '--apple-pale-apple-gray': '#2c2c2e',
  '--apple-light-gray': '#2c2c2e',
  '--apple-gray': '#48484a',
  '--apple-gray-50': '#1c1c1e',
  '--apple-pure-white-canvas': '#000000',

  // 暗色阶梯：壳层比画布抬一档，卡片再抬一档，浮层最高（原先 bg-card / surface-panel 各写一份同值）
  '--apple-surface-chrome': '#1c1c1e',
  '--apple-surface-canvas': '#000000',
  '--apple-surface-panel': '#1c1c1e',
  // 加厚档与壳层同色（2026-10-27，与 d:\Study\citebase 工作区同步）：暗色的「内容滚到顶栏底下」
  // 加厚是**实心度**不是亮度 —— elevated 取 #2c2c2e 时，scroll-edge 一触发整条顶栏亮成灰
  // （用户判词「topbar 发灰」）；暗色实心没有「透出去」的问题，真正的浮层档由 float 承担。
  '--apple-surface-elevated': '#1c1c1e',
  // 实心材质没有「透出去」这回事，float 在实心档就是 elevated 的另一个名字。
  // 分档是给材质的，不是给层级多加一层的；契约逼实心也写出来，是为了改一处不会悄悄掉档。
  '--apple-surface-float': '#2c2c2e',
  // 输入面要比卡片更亮一档（凹陷在暗色里是「抬起来的深灰」，用画布黑会糊成一片）
  '--apple-surface-field': '#2c2c2e',
  // 主对象档：介于卡片与浮层之间的一档，配自己的 elevation——一页只有一个「用户是来看它的东西」
  '--apple-surface-primary': '#232325',
  '--apple-shadow-primary': '0 1px 2px rgba(0,0,0,0.4), 0 10px 30px rgba(0,0,0,0.55)',

  '--apple-bg-secondary': '#1d1d1f',
  '--apple-bg-tertiary': '#2c2c2e',
  '--apple-bg-hover': '#2c2c2e',

  '--apple-surface-knob': '#ffffff',
  '--apple-on-accent': '#ffffff',
  '--apple-overlay': 'rgba(0,0,0,0.6)',

  '--apple-border': '#38383a',
  '--apple-border-light': '#2c2c2e',
  '--apple-soft-border-gray': '#38383a',
  '--apple-mid-border-gray': '#48484a',
  '--apple-utility-dark-gray': '#48484a',

  '--apple-blue': '#0a84ff',
  '--apple-green': '#30d158',
  '--apple-red': '#ff453a',
  '--apple-orange': '#ff9f0a',
  '--apple-yellow': '#ffd60a',
  '--apple-purple': '#bf5af2',
  '--apple-teal': '#64d2ff',
  '--apple-indigo': '#5e5ce6',
  '--apple-pink': '#ff375f',

  '--apple-code-bg': '#1e1e1e',
  '--apple-code-fg': '#d4d4d4',
  '--apple-code-green': '#7ee787',
  '--apple-code-blue': '#79c0ff',
  '--apple-code-purple': '#d2a8ff',
  '--apple-code-orange': '#ffa657',

  '--apple-scrollbar-thumb': 'rgba(255,255,255,0.22)',
  '--apple-scrollbar-thumb-hover': 'rgba(255,255,255,0.38)',

  '--apple-glass-border-nav': 'rgba(255,255,255,0.12)',
  // 非玻璃材质的"玻璃亮边"回退：浅色是黑 0.08，深色必须是白（见 cssVariables 的同名令牌）
  '--apple-glass-rim': 'rgba(255,255,255,0.16)',
  // dark 的五档表面全是实心 hex（chrome/panel/elevated #1c1c1e、float #2c2c2e）—— 挂 blur 只会
  // 每帧快照一次背景再被自己不透明的底盖掉，一个像素都看不见。纸感/编辑部两套本来就写 none。
  // 唯一需要它的是液态玻璃（表面是 rgba 0.58~0.86），那套保持 blur(22px)。
  '--apple-glass-blur': 'none',
  '--apple-blur-panel': 'none',

  '--apple-radius-card': '10px',
  '--apple-radius-control': '8px',
  '--apple-radius-panel': '4px',

  // 暗色分层：投影在黑底上几乎不可见，靠 1px 内高光 + 描边撑层级
  '--apple-shadow-card': 'none',
  '--apple-shadow-card-hover': '0 0 0 1px rgba(255,255,255,0.14), 0 8px 24px rgba(0,0,0,0.55)',
  '--apple-shadow-card-hover-refined': '0 0 0 1px rgba(255,255,255,0.14), 0 4px 14px rgba(0,0,0,0.5)',
  '--apple-shadow-modal': '0 16px 48px rgba(0,0,0,0.6), 0 0 0 1px rgba(255,255,255,0.1)',
  '--apple-shadow-dropdown': '0 4px 24px rgba(0,0,0,0.5), 0 0 0 1px rgba(255,255,255,0.1)',
  '--apple-shadow-level1': '0 0 0 1px rgba(255,255,255,0.1)',
  '--apple-shadow-level2': '0 2px 8px rgba(0,0,0,0.3)',
  '--apple-shadow-level3': '0 4px 16px rgba(0,0,0,0.4)',
  '--apple-shadow-subtle': 'inset 0 1px 0 rgba(255,255,255,0.06)',
  '--apple-shadow-nav': '0 1px 0 rgba(255,255,255,0.1)',
  '--apple-shadow-inner-focus': 'inset 0 0 0 1px rgba(255,255,255,0.1)',

  '--apple-font-display': SF_DISPLAY,
  '--apple-font-text': SF_TEXT,

  // 派生族的暗色强度覆盖（:root 的 12%~14% 在黑底上偏弱）
  '--apple-blue-selection': 'color-mix(in srgb, var(--apple-blue) 18%, transparent)',
  '--apple-blue-selection-strong': 'color-mix(in srgb, var(--apple-blue) 24%, transparent)',
  '--apple-blue-bg-subtle': 'color-mix(in srgb, var(--apple-blue) 20%, transparent)',
  '--apple-green-bg-subtle': 'color-mix(in srgb, var(--apple-green) 18%, transparent)',
  '--apple-purple-bg-subtle': 'color-mix(in srgb, var(--apple-purple) 18%, transparent)',
  '--apple-teal-bg-subtle': 'color-mix(in srgb, var(--apple-teal) 18%, transparent)',
  '--apple-indigo-bg-subtle': 'color-mix(in srgb, var(--apple-indigo) 22%, transparent)',
  '--apple-pink-bg-subtle': 'color-mix(in srgb, var(--apple-pink) 18%, transparent)',
  '--apple-yellow-bg-subtle': 'color-mix(in srgb, var(--apple-yellow) 20%, transparent)',
  '--apple-gray-bg-subtle': 'color-mix(in srgb, var(--apple-gray) 20%, transparent)',
  '--apple-red-bg-subtle': 'color-mix(in srgb, var(--apple-red) 18%, transparent)',
  '--apple-orange-bg-subtle': 'color-mix(in srgb, var(--apple-orange) 18%, transparent)',
  '--apple-blue-text': 'color-mix(in srgb, var(--apple-blue) 78%, #fff)',
  '--apple-green-text': 'color-mix(in srgb, var(--apple-green) 76%, #fff)',
  '--apple-red-text': 'color-mix(in srgb, var(--apple-red) 78%, #fff)',
  '--apple-orange-text': 'color-mix(in srgb, var(--apple-orange) 80%, #fff)',
  '--apple-yellow-text': 'color-mix(in srgb, var(--apple-yellow) 82%, #fff)',
  '--apple-purple-text': 'color-mix(in srgb, var(--apple-purple) 78%, #fff)',
  '--apple-teal-text': 'color-mix(in srgb, var(--apple-teal) 76%, #fff)',
  '--apple-indigo-text': 'color-mix(in srgb, var(--apple-indigo) 72%, #fff)',
  '--apple-pink-text': 'color-mix(in srgb, var(--apple-pink) 78%, #fff)',
  '--apple-gray-text': 'color-mix(in srgb, var(--apple-gray) 20%, #fff)',
  '--apple-callout-note-bg': 'color-mix(in srgb, var(--apple-blue) 15%, transparent)',
  '--apple-callout-note-border': 'color-mix(in srgb, var(--apple-blue) 40%, transparent)',
  '--apple-callout-warning-bg': 'color-mix(in srgb, var(--apple-orange) 15%, transparent)',
  '--apple-callout-warning-border': 'color-mix(in srgb, var(--apple-orange) 40%, transparent)',
  '--apple-callout-tip-bg': 'color-mix(in srgb, var(--apple-green) 15%, transparent)',
  '--apple-callout-tip-border': 'color-mix(in srgb, var(--apple-green) 40%, transparent)',
  '--apple-callout-danger-bg': 'color-mix(in srgb, var(--apple-red) 15%, transparent)',
  '--apple-callout-danger-border': 'color-mix(in srgb, var(--apple-red) 40%, transparent)',
  '--apple-callout-info-bg': 'color-mix(in srgb, var(--apple-gray) 15%, transparent)',
  '--apple-callout-info-border': 'color-mix(in srgb, var(--apple-gray) 40%, transparent)',

  // 暗色专属的语义反转键（不参与契约：浅色主题沿用字面默认值才是对的）
  '--apple-absolute-black': '#f5f5f7',
  '--apple-white': '#000000',
  '--apple-white-alpha-15': 'rgba(255,255,255,0.15)',
  '--apple-white-alpha-20': 'rgba(255,255,255,0.2)',
  '--apple-white-alpha-30': 'rgba(255,255,255,0.3)',
  '--apple-white-alpha-70': 'rgba(255,255,255,0.7)',
  '--apple-white-alpha-80': 'rgba(255,255,255,0.8)',
  '--apple-shadow-thumb': '0 1px 3px rgba(0,0,0,0.4)',
  '--apple-shadow-thumb-hover': '0 2px 6px rgba(0,0,0,0.5)',
  '--apple-pink-selection': 'color-mix(in srgb, var(--apple-pink) 32%, transparent)',
  '--apple-search-highlight': 'color-mix(in srgb, var(--apple-yellow) 25%, transparent)',
  '--apple-folder-icon': 'rgb(255, 159, 10)',
};

/**
 * 玻璃材质 overlay：只声明**材质固有**的键（半透明表面阶梯 / blur / 玻璃描边 / 镜面高光阴影 /
 * 圆角 / 环境相关的 code-bg / 对比色），中性 palette（文字/ink/gray/canvas/字体）一律**继承当前 base**。
 * 于是「玻璃」不再重抄一整套调色板，且改 base palette 时对应的玻璃自动跟随。
 *
 * 两套 overlay 按 appearance 分：
 *   light —— 半透明白底会吃掉对比度，所以纯色压深一档（沿用旧 liquid-glass 的验证过的值）
 *   dark  —— 深色玻璃，纯色继承 dark 已调亮的值，表面是深色半透明 + 更弱的白镜面高光
 */
export type GlassOverlayMap = Partial<ThemeOverrideMap>;

/**
 * 玻璃的**边缘语言**：亮边 + 方向性棱线（顶亮 / 侧弱白 / 底暗）。
 * 它是一枚令牌而不是组件里的常量，因为这不是"所有材质都该有"的分层手段：
 * solid 是 Apple 基线材质，纸感/杂志是纸面 —— 三者都把它解析成一条完全透明的 inset，
 * 于是同一份 `AppleGlass.*` 常量和 `.apple-glass-layer` 伪元素在非玻璃材质下
 * 不会画出只有玻璃才成立的高光与棱线。此前它写死在组件层，纸感浮层一直带着白斜面。
 */
const GLASS_EDGE =
  'inset 0 0.5px 0.5px rgba(255,255,255,0.30), ' +
  'inset 0.5px 0 0.5px rgba(255,255,255,0.14), inset -0.5px 0 0.5px rgba(255,255,255,0.14), ' +
  'inset 0 -0.5px 0.5px rgba(0,0,0,0.14)';

/**
 * 深色的边缘语言比浅色**多一条内发光**，棱线也更弱：
 *  · 真正的镜面亮段由 `--apple-glass-rim-specular` 按光源方向画（见上），
 *    这枚 inset 只补"斜切边本身的体积"——顶亮、侧弱、底暗，四边强度不等；
 *  · `inset 0 0 14px -2px` 让四周比中心亮一点：玻璃的厚度感来自从边往里渗的光，
 *    它用 px，所以在 48px 顶栏和 900px 侧栏上是同一档强度；
 *  · 底部内侧压暗（光在上面）给平板一点体积，不然只是一块均匀色片。
 * 上缘从 0.5 收到 0.16：0.5 的白线在深色上就是满屏宽的一条硬白线，而镜面亮段已经
 * 负责"受光那一侧"的亮度，这里再压一条满宽的高亮只会读成画上去的描边。
 */
const GLASS_EDGE_DARK =
  'inset 0 0.5px 0.5px rgba(255,255,255,0.16), ' +
  'inset 0.5px 0 0.5px rgba(255,255,255,0.10), inset -0.5px 0 0.5px rgba(255,255,255,0.10), ' +
  'inset 0 -0.5px 0.5px rgba(0,0,0,0.14), ' +
  'inset 0 0 18px -4px rgba(255,255,255,0.12), ' +
  'inset 0 -9px 16px -9px rgba(0,0,0,0.40)';

/**
 * 镜面高光是**贴着边的光**，不是铺在面上的一整块白。分两层表达，消费者不同：
 *
 *  1. `--apple-glass-specular`（面，background-image）只负责两件事：
 *     上缘 10px 的薄斜面（固定 px —— 顶栏 48px 与侧栏 900px 拿到同样厚度）
 *     + 一枚跟着光源走的极弱面光。它让玻璃不至于是一块均匀色片。
 *  2. `--apple-glass-rim-specular`（边，1px 环，画法见 apple-tokens.ts 的 `::after`）
 *     才是真正的高光：光源在哪一侧，环上那一段就亮，沿周长衰减到 0。
 *     指针一动，亮段就沿边滑动 —— "液态"来自这里，而不是来自一整片白色。
 *
 * 为什么不能按百分比做纵向衰减：同一枚令牌在 48px 顶栏上是 9px 白带，在 900px 侧栏上
 * 变成 288px 白雾；而且旧版 0.16→0.03@14% 的折点在顶栏上意味着 9px 内掉 36 级亮度，
 * 实拍读作"刷了一条白漆、末端一条硬边"。光属于形状的边，不该按盒子的高度铺。
 * 非玻璃材质两层都解析成 none。
 */
const GLASS_SPECULAR =
  'linear-gradient(to bottom, rgba(255,255,255,0.10) 0, rgba(255,255,255,0) 10px), ' +
  'radial-gradient(120% 140% at var(--apple-light-x) var(--apple-light-y), ' +
  'rgba(255,255,255,0.06), rgba(255,255,255,0) 68%)';

// 2026-11-01 四轮：顶部斜面从"10px 内掉到 0"改成 32px 的**分级衰减**
// （0.09 → 0.030@6px → 0.009@18px → 0）。原来的写法在暗底上是一条硬边收口的白漆，
// 读起来像"刷上去的"，而不是一块被光从上方照到的玻璃。px 仍写死：
// 48px 顶栏与 900px 侧栏拿到同一段斜面厚度，不会因为盒子变高而糊成白雾。
// 2026-11-01 七轮（「不自然，穿透效果都没了」）：四级衰减仍在，但峰值从 0.09 收到 0.06、
// 长度 32px → 18px。实测顶栏剖面 93 → 62 是 31 级、跨整条 32px 的一条"亮搁板"，
// 比它身后环境层（同一个 x 上只有 ±10 级）更像画上去的漆；此后**横向**的明暗差
// 才是玻璃的证据（左 62 → 右 49 是环境层透过来的），纵向这条反光只做边，不做面。
// 面光同步 0.05 → 0.035：它和顶部斜面相叠，原先两笔光都压在左上角。
const GLASS_SPECULAR_DARK =
  'linear-gradient(to bottom, rgba(255,255,255,0.06) 0, rgba(255,255,255,0.018) 6px, ' +
  'rgba(255,255,255,0) 18px), ' +
  'radial-gradient(120% 140% at var(--apple-light-x) var(--apple-light-y), ' +
  'rgba(255,255,255,0.035), rgba(255,255,255,0) 68%)';

/**
 * 边上那圈镜面：径向渐变以光源为圆心，所以"最亮的一段"永远落在光源那一侧的边上，
 * 远端衰减到全透明。这与真实斜切边的反光一致 —— 迎光的一侧有亮棱，背光的一侧只剩
 * 底部的深色棱线，而不是四边等亮的一圈描边。
 */
const GLASS_RIM_SPECULAR =
  'radial-gradient(140% 140% at var(--apple-light-x) var(--apple-light-y), ' +
  'rgba(255,255,255,0.72), rgba(255,255,255,0.26) 32%, rgba(255,255,255,0.08) 60%, ' +
  'rgba(255,255,255,0) 82%)';

const GLASS_RIM_SPECULAR_DARK =
  'radial-gradient(140% 140% at var(--apple-light-x) var(--apple-light-y), ' +
  'rgba(255,255,255,0.42), rgba(255,255,255,0.15) 32%, rgba(255,255,255,0.05) 60%, ' +
  'rgba(255,255,255,0) 88%)';

export const GLASS_OVERLAY: Record<'light' | 'dark', GlassOverlayMap> = {
  light: {
    // 强调/链接压深：半透明白底上原色对比不足
    '--apple-apple-action-blue': '#0066f5',
    '--apple-body-link-blue': '#0052cc',
    '--apple-high-luminance-link-blue': '#338cff',

    // 玻璃表面阶梯：透明度要真的低，环境色相才会折射出来；field 比 panel 更透
    // 2026-11-14 unslop（用户判词「浅色页面背景色都不统一」）：环境层在上一轮已经收成
    // 中性纸面 #f2f1ed，但这一组表面还留着旧蓝底时代的冷白（252,253,255 / 253,254,255
    // / 250,252,255），叠在暖灰画布上就是「有的页面发蓝、有的页面发黄」。
    // 现在表面阶梯与画布同一族中性色：白只保留 255 纯白，凹陷面走暖中性灰。
    '--apple-surface-chrome': 'rgba(255,255,255,0.58)',
    '--apple-surface-canvas': 'transparent',
    '--apple-surface-panel': 'rgba(255,255,255,0.66)',
    // 浮层档同时是「内容从壳层底下滚过时」顶栏的那一档（见 glassExtraCss 的 scroll-edge 规则）：
    // 静止态顶栏用 chrome 保持通透，一旦有内容滚进来就换到这一档。所以它必须扛得住
    // **任意外部内容**当背景（极值 = 纯黑）—— 判据是 check-contrast.mjs 的「浮层壳面」断言组。
    // 也正因为要扛纯黑，这一档降不下来。上一轮它同时被菜单/弹窗复用，后果就是那句肉眼判词
    // 「根本就不是液态玻璃」：0.84~0.88 的白把环境色相全吃掉，浮层成了带发丝边的白板。
    '--apple-surface-elevated': 'rgba(255,255,255,0.88)',
    // 浮层面板档（菜单 / 下拉 / 弹窗 / 抽屉）：**身后是什么，板就多实**。
    // 0.78 是「没人量过」时的保守回落 —— 运行时由 useGlassAdaptive 按面板身后被画出来的
    // 底色改写这块板（浅环境降到 ~0.43，撞上位图抬到 0.94）。
    // **为什么覆写的是 `--apple-surface-float` 本身，而不是在这里嵌一层 `var(--apple-glass-float-a, 0.78)`**：
    // 自定义属性的计算值是「在它被声明的那个元素上把 var() 换完」的结果，所以写在 :root 里的
    // 嵌套 var 会先在 :root 求值，元素上再改那个内层属性已经追不上了 ——
    // 实测过：元素拿到的是解好的 rgba(...,0.78)，运行时设了值却一点效果没有。
    // 底色与上面同一调整：浮层的"面"靠 alpha 与发丝边区分，不再靠冷色相
    '--apple-surface-float': 'rgba(252,252,250,0.78)',
    '--apple-surface-field': 'color-mix(in srgb, var(--apple-near-black-ink) 7%, transparent)',
    '--apple-surface-primary': 'rgba(255,255,255,0.78)',
    '--apple-shadow-primary': '0 2px 6px rgba(31,41,75,0.06), 0 16px 40px rgba(31,41,75,0.12)',

    // 玻璃上的文字档必须压深：半透明白底合成后亮度会掉一档，沿用实心的 #6b6b70/#6e6e73
    // 在暗内容滚过壳层时只剩 2.1:1。理由与上面「强调/链接压深」完全同一条，只是对象换成文字。
    '--apple-text-secondary': '#4a4a50',
    '--apple-text-tertiary': '#58585e',
    // 四级文字也要压深：它大量用于浮层里的小号元信息（时间、计数、折叠箭头），
    // 根级 #74747a 压在最坏照片上只有 ~4.1:1，连自适应抬到 CEIL 也过不了 4.5。
    // 这一档此前不在 check:contrast 的 TEXT_TIERS 里，属于"用得最多、却没人判"的那一档。
    '--apple-text-quaternary': '#6b6b70',

    '--apple-bg-secondary': 'rgba(250,250,248,0.74)',
    '--apple-bg-tertiary': 'rgba(245,245,243,0.84)',
    '--apple-bg-hover': 'rgba(239,239,237,0.90)',

    '--apple-surface-knob': 'rgba(255,255,255,0.96)',
    '--apple-on-accent': '#ffffff',
    // 遮罩降一档且不带模糊（理由见深色那一档）：0.45 的黑 + 4px 预磨砂会把浅色页面染成灰紫，
    // 浮层的模糊在糊过的背景上读不出来。色相同步去蓝：暖纸画布上压一层蓝黑就是发灰紫。
    '--apple-overlay': 'rgba(28,28,26,0.32)',

    // 面板描边用极淡冷灰（白发光线在白面板上不可见），导航边缘仍留高光
    '--apple-border': 'rgba(31,41,75,0.13)',
    '--apple-border-light': 'rgba(31,41,75,0.08)',
    '--apple-soft-border-gray': 'rgba(31,41,75,0.13)',
    '--apple-mid-border-gray': 'rgba(30,45,90,0.16)',
    '--apple-utility-dark-gray': 'rgba(30,45,90,0.18)',

    // 纯色在玻璃上压深一档
    '--apple-blue': '#0066f5',
    '--apple-green': '#1d7a43',
    '--apple-red': '#d7281f',
    '--apple-orange': '#c2740b',
    '--apple-yellow': '#c9a100',
    '--apple-purple': '#8944ab',
    '--apple-teal': '#0a84a8',
    '--apple-indigo': '#4f4cc0',
    '--apple-pink': '#d6305a',

    '--apple-code-bg': 'rgba(246,246,244,0.86)',
    '--apple-code-fg': '#1a1c22',
    '--apple-code-green': '#1d7a43',
    '--apple-code-blue': '#0066f5',
    '--apple-code-purple': '#8944ab',
    '--apple-code-orange': '#c2740b',

    '--apple-scrollbar-thumb': 'rgba(30,45,90,0.22)',
    '--apple-scrollbar-thumb-hover': 'rgba(30,45,90,0.38)',

    // 底边的满宽发丝线让位给「随光源滑动的亮段」：0.85 与它叠加成双线，
    // 边界上能量出两层白（见深色那一档的剖面）。0.55 只留一个"有边"的底子。
    '--apple-glass-border-nav': 'rgba(255,255,255,0.55)',
    // 玻璃语言三件套：边缘棱线 / 亮边 / 镜面高光。非玻璃材质在 cssVariables 默认值里
    // 解析成透明 inset / 中性发丝边 / none —— 纸感与杂志因此不会继承任何玻璃装饰。
    '--apple-glass-edge': GLASS_EDGE,
    '--apple-glass-rim': 'rgba(255,255,255,0.35)',
    '--apple-glass-specular': GLASS_SPECULAR,
    '--apple-glass-rim-specular': GLASS_RIM_SPECULAR,
    '--apple-glass-rim-line': 'rgba(255,255,255,0.75)',
    // 滚动边缘态：浅色壳面本来就接近白，亮段压在它上面几乎不可见，暗一档只是让同一条
    // 规则在两套外观下行为一致（静止态发光、内容压上来时收住），不另开一支判断。
    '--apple-glass-rim-line-dim': 'rgba(255,255,255,0.5)',
    '--apple-glass-nav-shadow': '0 8px 24px rgba(0,0,0,0.10)',
    '--apple-glass-foot-shadow': '0 -8px 24px rgba(0,0,0,0.10)',
    '--apple-glass-side-shadow': '8px 0 24px -12px rgba(0,0,0,0.16)',
    '--apple-glass-blur': 'blur(22px) saturate(180%)',
    // 面板档付**轻量**模糊：卡片/统计块是内容层，用比壳层薄一档的 10px
    // （壳层 22px、浮层走同一档，层级靠模糊深度而不是靠"糊/不糊"编码）。
    // 2026-10-25 之前这里是 none，理由是首页同屏 23 个 backdrop-filter 层太贵；
    // 实测那条成本仍然成立（23 次实时采样），但视觉代价更大：卡片只剩半透明，
    // 读起来是"淡色纸板"不是玻璃，用户两轮判词都是"玻璃效果没了"。
    // 折中是**降低单层成本**（10px 而非 22px）而不是取消这一层：模糊面积小、半径小，
    // 单层开销约为壳层的 1/3。若弱机实测掉帧，先降半径再考虑按屏限量。
    '--apple-blur-panel': 'blur(10px) saturate(150%)',

    '--apple-radius-card': '14px',
    '--apple-radius-control': '10px',
    '--apple-radius-panel': '6px',
    '--apple-radius-float': '20px',

    // 玻璃阴影：低不透明度外投影 + 顶部镜面内高光 + 边缘增亮环
    //
    // modal / dropdown 这两档额外画一圈**边缘厚度**（亮环 + 上缘高光 + 下缘暗线 + 内散射）。
    // 为什么塞在阴影族而不是另立一枚玻璃令牌：这一圈边是「这块面有多厚」的表达，
    // 和投影同属分层手段、同按主题分档，再开一个名字就是同一件事第二条命名路径。
    // 为什么必须靠边而不是只靠模糊：浮层里嵌在已模糊祖先下的那些（侧栏里的用户菜单就是），
    // backdrop 是祖先那块平板，糊了等于没糊 —— 边在任何背景上都看得见，它是
    // 「浅环境里读得出玻璃」的另一半，且不受身后有没有内容影响。
    '--apple-shadow-card': '0 8px 32px rgba(31,41,75,0.12), inset 0 1px 0 rgba(255,255,255,0.85), inset 0 0 0 1px rgba(255,255,255,0.28)',
    '--apple-shadow-card-hover': '0 14px 44px rgba(31,41,75,0.16), inset 0 1px 0 rgba(255,255,255,0.92)',
    '--apple-shadow-card-hover-refined': '0 10px 30px rgba(31,41,75,0.13), inset 0 1px 0 rgba(255,255,255,0.90)',
    '--apple-shadow-modal': '0 24px 64px rgba(31,41,75,0.18), inset 0 1px 0 rgba(255,255,255,0.95), inset 0 0 0 0.5px rgba(255,255,255,0.55), inset 0 -1px 0 rgba(31,41,75,0.12), inset 0 0 24px rgba(255,255,255,0.22)',
    '--apple-shadow-dropdown': '0 12px 40px rgba(31,41,75,0.16), inset 0 1px 0 rgba(255,255,255,0.95), inset 0 0 0 0.5px rgba(255,255,255,0.55), inset 0 -1px 0 rgba(31,41,75,0.12), inset 0 0 12px rgba(255,255,255,0.26)',
    '--apple-shadow-level1': 'inset 0 1px 0 rgba(255,255,255,0.80)',
    '--apple-shadow-level2': '0 2px 8px rgba(31,41,75,0.10), inset 0 1px 0 rgba(255,255,255,0.80)',
    '--apple-shadow-level3': '0 4px 16px rgba(31,41,75,0.14), inset 0 1px 0 rgba(255,255,255,0.85)',
    '--apple-shadow-subtle': '0 1px 4px rgba(31,41,75,0.08), inset 0 1px 0 rgba(255,255,255,0.70)',
    '--apple-shadow-nav': 'inset 0 1px 0 rgba(255,255,255,0.70)',
    '--apple-shadow-inner-focus': 'inset 0 0 0 1px rgba(255,255,255,0.60)',

    // 派生族在玻璃上更淡（半透明底本就轻）
    '--apple-blue-selection': 'color-mix(in srgb, var(--apple-blue) 14%, transparent)',
    '--apple-blue-bg-subtle': 'color-mix(in srgb, var(--apple-blue) 10%, transparent)',
  },

  dark: {
    // 深色玻璃：表面是半透明色层，环境色相折射出来；纯色/文字全部继承 dark palette（已为暗底调亮）
    // 2026-11-01 材质三档：chrome 从近黑 tint 换成提亮 tint（0.38）。
    // 二档降到 0.34 后实拍量到侧栏与内容只差 (7,5,1) —— 玻璃是"透"了，可**面**没了：
    // 近黑 tint 压在近黑壁纸上，壳层和画布落在同一明度，只剩一条发丝边。
    // 深色玻璃的提亮方向与浅色相反：Apple 的深色 vibrancy 是"提亮 + 轻微去饱和"，
    // 不是"再涂一层深色"。
    // panel 不动：它是卡片/编辑器这些**内容面**，跟着提亮会把整页内容一起漂白。
    // 2026-11-01 二轮：0.38 → 0.32。提亮本身没错，但 0.38 压在深色壁纸上时，
    // 壳层比内容高 39 个亮度单位，边界读成"一刀切的色块"（用户仍看不过去）；
    // 0.32 保住玻璃的"面"，同时把台阶收到 30 上下，剩下的交给侧向软影与棱线。
    // 2026-11-01 三轮（去 AI 感）：tint 从冷灰蓝 118,130,158 换成中性石墨 134,137,144。
    // 壳层的色相来自环境层折射，环境层已去蓝紫（见 GLASS_ENV_BASE_COLOR），tint 再带蓝
    // 就成了整屏唯一的色源 —— Apple 深色材质是中性近黑，微冷即可，不带可辨识色相。
    // 2026-11-01 九轮（判词「降低深色主题的灰度」）：四档表面一起压深 ~7 级，**只动 rgb 不动 alpha**
    // （穿透是七轮好不容易找回来的，不许回退）。色相仍是中性石墨，明度整体下移：
    // chrome 0.32×134 → 0.32×114，合成 50 → 44。
    '--apple-surface-chrome': 'rgba(114,116,122,0.32)',
    '--apple-surface-canvas': 'transparent',
    // panel 仍是深色板（不跟着 chrome 一起提亮 —— 那条试过，整页内容会被漂白），
    // 但**底色必须取壳层同一族**：原先 30,30,32 是中性近黑、与壳层脱档的教训见上；
    // 跟着 chrome 一起换到石墨族（58,60,67），同屏不再有两套色温。
    // 2026-11-01 四轮（「依然很粗糙」）：38,40,45 / 0.50 → 58,60,67 / 0.58。
    // 实测旧值压在 #101014 上只比画布高 11 个亮度单位，而 1px 描边（0.08 白）高 18 ——
    // **边比面亮**，卡片读成线框。提亮底色 + 提 alpha 让面先立起来，描边退回补边（见 border-light）。
    // 2026-11-01 九轮：58,60,67 → 46,48,54（压在 #0b0b0e 上合成 38 → 31，"灰度"判词的主病灶就是它）。
    '--apple-surface-panel': 'rgba(46,48,54,0.58)',
    // 同浅色那一档的理由，方向反过来：深色浮层壳面的最坏背景是**亮内容**（白图、白色代码块）。
    // 2026-11-01 五轮（用户判词「顶栏侧栏底栏行为不一致 / 穿透过度」）：24,24,26 / 0.96 → 50,51,56 / 0.90。
    // 上一版两块板**不同族**：静止档是提亮的石墨 134、加厚档是近黑 24。合成到环境底上，
    // 静止 = 45、加厚 = 23 —— 内容一滚进来，同一条壳边就换成一块明显更黑的板；
    // 三条壳边又各滚各的，于是"顶栏/侧栏/底栏是三种灰"。
    // 这一档的取法是把加厚板做成**与静止板同明度**的深石墨：0.90 × (44,45,50) 压在 #0b0b0e 上
    // 合成 41，与静止档的 44 几乎重合（Δ3），加厚只发生在亮内容真的滑到身下时。
    // 同时 0.90 是这一档的地板 —— 10% 的透出让橙色图标、白色大标题只留一层极淡的色偏，
    // 不再是糊在壳上的色斑（旧值 0.28 透 72%）。AA 仍按纯白极值判（三级 4.58 / 四级 3.5）。
    // 2026-11-01 九轮：跟着 chrome 一起压深（50,51,56 → 44,45,50），两档合成差仍 ≤3。
    '--apple-surface-elevated': 'rgba(44,45,50,0.90)',
    // 浮层面板档：同浅色那一档，0.78 只是没量过时的保守回落（为什么由运行时覆写整枚令牌，
    // 理由写在浅色那一档的注释里）。0.74 会让三级文字压最坏照片时掉到 4.30（门槛 4.5）——
    // 令牌回落必须自己站得住，运行时那层自适应才有意义。底色比 elevated 再压深一档，留余量。
    // 2026-11-01 九轮：40,40,42 → 36,36,40（跟着 elevated 压深）。
    '--apple-surface-float': 'rgba(36,36,40,0.78)',
    // 控件凹陷档要在玻璃上看得见，而且**方向要对**：凹陷是"压进去"，不是"贴一块更亮的板"。
    // 原先深色给的是 16% 白 —— 它压在 chrome 0.24 上比壳层更亮，读成一块贴上去的亮蓝
    // （/space/1 的文档树搜索框实测就是这条：field 0.18 压 chrome 0.24）。
    // 深色玻璃的凹陷一律走**压暗**，浅色玻璃本来就是压暗（ink 7%），两外观同方向。
    '--apple-surface-field': 'color-mix(in srgb, #000000 26%, transparent)',
    // 主对象档跟 panel 同一族石墨（42,48,66 的蓝灰与去紫后的环境层脱档）
    // 2026-11-01 四轮：44,46,52 / 0.78 → 64,66,73 / 0.62。它压在阅读正文下面，
    // 必须比卡片（panel）亮一档、又压在壳层（chrome）之下 —— 三档同屏时是
    // 画布 < 卡片 < 正文面 < 壳层，这条阶梯不能有一级越过壳层，否则侧栏读作凹进墙里。
    // 2026-11-01 九轮：64,66,73 → 54,56,62（合成 44 → 38，与 panel 的台阶 7、与 chrome 的台阶 6 都保住）。
    '--apple-surface-primary': 'rgba(54,56,62,0.62)',

    // 深色玻璃上的文字档要提亮：半透明深底压上亮内容后会被抬亮，
    // 沿用 dark 的 #a1a1a6/#86868b 在亮图上只剩 3.x:1。
    // 留了余量而不是刚好压线：压着 4.50 过的值，下一次谁动 alpha 都会静默崩掉这一档。
    '--apple-text-secondary': '#cacacf',
    '--apple-text-tertiary': '#b8b8be',
    // 同浅色：四级文字在深度玻璃浮层上要能过 4.5（亮照片是最坏背景）
    '--apple-text-quaternary': '#a1a1a6',
    '--apple-shadow-primary': '0 2px 6px rgba(0,0,0,0.4), 0 16px 40px rgba(0,0,0,0.55)',

    // 与上面四档表面同一色温：这三个值原本是 Apple 深色原样的中性灰（B-R 只差 2），
    // 而 panel/primary/chrome 是石墨族（B-R 差 6~8）—— 同屏卡片与卡片上的按钮、hover
    // 就是两块冷暖不同的灰。只补蓝通道，明度与原值逐档对齐（44,45,50 与 elevated 同值）。
    '--apple-bg-secondary': 'rgba(29,30,34,0.74)',
    '--apple-bg-tertiary': 'rgba(44,45,50,0.84)',
    '--apple-bg-hover': 'rgba(58,59,65,0.90)',

    '--apple-surface-knob': '#ffffff',
    '--apple-on-accent': '#ffffff',
    // 遮罩只染色、不预磨砂（模糊由 overlay.ts 那一侧关掉）：把锐利背景留给浮层自己去折射，
    // 否则浮层那 16px 模糊身后已经是糊的，玻璃唯一的证据就没了。0.6 → 0.45：够压住注意力，
    // 又不至于把整页压成一块黑板（实测弹窗与聚光在 0.6 下都读成实心盒）。
    '--apple-overlay': 'rgba(0,0,0,0.45)',

    // 深色玻璃描边：靠极淡白发光边分层（暗底上深灰边不可见）
    '--apple-border': 'rgba(255,255,255,0.14)',
    // 2026-11-01 四轮：0.08 → 0.055。面板底色压深时这条线比面还亮，
    // 卡片于是读成"画了一圈框"；降到 0.055 之后它只负责把面收住，
    // "这块板是立着的"交给上缘高光（见 shadow-card 的 inset 0 1px 0）。
    '--apple-border-light': 'rgba(255,255,255,0.055)',
    // 0.14 → 0.07：凹陷档改成"压暗"之后，槽自己已经看得出来了，这条描边再亮就变成
    // "描了一个框"而不是"挖了一个槽"（文档树搜索框实测：框比底抢眼）。描边退成补边。
    '--apple-soft-border-gray': 'rgba(255,255,255,0.07)',
    '--apple-mid-border-gray': 'rgba(255,255,255,0.18)',
    '--apple-utility-dark-gray': 'rgba(255,255,255,0.20)',

    // 代码块与 bg-secondary 同族（原来 30,30,32 是中性灰，夹在石墨族的阅读面里偏绿）
    '--apple-code-bg': 'rgba(30,31,36,0.70)',

    // 0.22 → 0.14 → 0.06：底边已经有一条随光源滑动的亮段，满宽的发丝线只负责
    // "远端也不至于完全没边"，压到 0.06 才读不出"画出来的线"（实拍剖面：
    // 0.14 时边界上多出约 19 个亮度单位的硬台阶）。
    // 浮层的边框同步收到 0.05 —— 它们有整圈镜面环，发丝线只是补形状。
    '--apple-glass-border-nav': 'rgba(255,255,255,0.06)',
    '--apple-glass-edge': GLASS_EDGE_DARK,
    '--apple-glass-rim': 'rgba(255,255,255,0.05)',
    '--apple-glass-specular': GLASS_SPECULAR_DARK,
    '--apple-glass-rim-specular': GLASS_RIM_SPECULAR_DARK,
    // 静止态 0.36（原 0.45）：随光滑动的高光配上惯性平滑后，峰值再高就读成"一条白线贴着边走"
    // （用户判词「跟随光设计太粗糙」）。降峰值 + 拉长衰减尾巴（见 GLASS_RIM_SPECULAR_DARK 同理），
    // 材料信号还在，但更像光而不是描边。
    '--apple-glass-rim-line': 'rgba(255,255,255,0.36)',
    // 滚动边缘态：内容压上来后壳面被拉暗（实拍 82 → 39~55），同一条 0.45 的白在它上面
    // 是 +37~+75 的单像素台阶，读起来就是一条画上去的线（用户的"生硬"）。
    // 0.16 把相对壳面的峰值台阶收到约 +25，仍留得下迎光的那一小段棱。规则在 apple-tokens.ts。
    '--apple-glass-rim-line-dim': 'rgba(255,255,255,0.16)',
    '--apple-glass-nav-shadow': '0 10px 28px rgba(0,0,0,0.34)',
    '--apple-glass-foot-shadow': '0 -10px 28px rgba(0,0,0,0.34)',
    // 侧栏右边是**唯一一条侧向的材质边界**（顶栏底边靠 nav-shadow 分离）。
    // 没有这条软影，侧栏与内容之间就是一刀切的明度台阶（实测 39 个亮度单位），
    // 读起来像"贴上去的色块"；阴影把它摊到 20px 上，边界才有前后关系。
    '--apple-glass-side-shadow': '6px 0 26px -10px rgba(0,0,0,0.55)',
    // 深色上 saturate 调低，避免折射出霓虹感
    '--apple-glass-blur': 'blur(22px) saturate(150%)',
    // 同浅色：面板档比壳层薄一档（理由见 light 那一档的注释）
    '--apple-blur-panel': 'blur(10px) saturate(140%)',

    '--apple-radius-card': '14px',
    '--apple-radius-control': '10px',
    '--apple-radius-panel': '6px',
    '--apple-radius-float': '20px',

    // 深色玻璃阴影：深黑外投影 + 弱白镜面内高光（比浅色弱得多，暗底上强白内发光会脏）
    // 2026-11-01 四轮：删掉 `inset 0 0 0 1px rgba(255,255,255,0.06)`。
    // 卡片自身已经有一条 1px border，这圈内环画在紧邻的下一像素上，两者相加 ≈ 0.14 白 ——
    // 正是"边比面亮"的另一半来源（ApEntityCard 实测边框读作 0.14）。上缘高光保留：
    // 它才是"板立着、光从上方来"的物理线索；四周等亮的内环只是描边。
    // 2026-11-01 五轮：补一条**下缘压暗**。只有上缘提亮时，板读起来是"一片浮着的膜"；
    // 上亮 + 下暗才是"一块有厚度的板"。这一像素也是深色里唯一还能被看见的投影 ——
    // 32px 的黑色外投影压在 11 的画布上等于没有。
    '--apple-shadow-card': '0 8px 32px rgba(0,0,0,0.5), inset 0 1px 0 rgba(255,255,255,0.09), inset 0 -1px 0 rgba(0,0,0,0.30)',
    '--apple-shadow-card-hover': '0 14px 44px rgba(0,0,0,0.55), inset 0 1px 0 rgba(255,255,255,0.14)',
    '--apple-shadow-card-hover-refined': '0 10px 30px rgba(0,0,0,0.5), inset 0 1px 0 rgba(255,255,255,0.12)',
    '--apple-shadow-modal': '0 24px 64px rgba(0,0,0,0.6), inset 0 1px 0 rgba(255,255,255,0.20), inset 0 0 0 0.5px rgba(255,255,255,0.10), inset 0 -1px 0 rgba(0,0,0,0.55), inset 0 0 24px rgba(255,255,255,0.05)',
    '--apple-shadow-dropdown': '0 12px 40px rgba(0,0,0,0.55), inset 0 1px 0 rgba(255,255,255,0.18), inset 0 0 0 0.5px rgba(255,255,255,0.09), inset 0 -1px 0 rgba(0,0,0,0.50), inset 0 0 12px rgba(255,255,255,0.05)',
    '--apple-shadow-level1': 'inset 0 1px 0 rgba(255,255,255,0.08)',
    '--apple-shadow-level2': '0 2px 8px rgba(0,0,0,0.4), inset 0 1px 0 rgba(255,255,255,0.08)',
    '--apple-shadow-level3': '0 4px 16px rgba(0,0,0,0.5), inset 0 1px 0 rgba(255,255,255,0.10)',
    '--apple-shadow-subtle': '0 1px 4px rgba(0,0,0,0.3), inset 0 1px 0 rgba(255,255,255,0.06)',
    '--apple-shadow-nav': 'inset 0 1px 0 rgba(255,255,255,0.08)',
    '--apple-shadow-inner-focus': 'inset 0 0 0 1px rgba(255,255,255,0.12)',

    // 派生族沿用 dark 的强度（18~20%），玻璃上再略提，保证在半透明深底上可见
    '--apple-blue-bg-subtle': 'color-mix(in srgb, var(--apple-blue) 24%, transparent)',
  },
};

/** 该 (base, material) 组合下玻璃是否真的生效 */
export const isGlassActive = (base: ThemeId, material: Material): boolean =>
  material === 'glass' && supportsGlass(base);

/** 玻璃 overlay（按外观取浅/深那套），非玻璃返回 null */
export function glassOverlayFor(base: ThemeId, material: Material): GlassOverlayMap | null {
  if (!isGlassActive(base, material)) return null;
  return GLASS_OVERLAY[isDarkThemeId(base) ? 'dark' : 'light'];
}

export const themeOverrides: Record<ExtendedThemeId, ThemeOverrideMap> = {
  /**
   * Editorial（浅 · 阅读）：暖纸底 + 编辑红 + 全直角 + 发丝分隔线。
   * 层级不靠投影靠横线，所以静置阴影一律 none，只有浮层给极淡一档。
   */
  editorial: {
    '--apple-apple-action-blue': '#b3261e',
    '--apple-body-link-blue': '#a02218',
    '--apple-high-luminance-link-blue': '#a02218',

    '--apple-near-black-ink': '#141210',
    '--apple-text-primary': '#141210',
    '--apple-text-secondary': '#5d574f',
    '--apple-text-tertiary': '#6e675d',
    '--apple-text-quaternary': '#998f83',
    '--apple-text-80': 'rgba(20,18,16,0.8)',
    '--apple-text-48': 'rgba(20,18,16,0.48)',
    '--apple-text-disabled': 'rgba(20,18,16,0.32)',
    '--apple-secondary-neutral-gray': '#5d574f',

    '--apple-pale-apple-gray': '#f6f4ef',
    '--apple-light-gray': '#f6f4ef',
    '--apple-gray': '#8b8378',
    '--apple-gray-50': '#faf9f5',
    '--apple-pure-white-canvas': '#ffffff',
    // 编辑部：壳层是一条更深的纸带，内容纸面略亮，浮层最白——层级靠色阶 + 横线
    '--apple-surface-chrome': '#f1eee6',
    '--apple-surface-canvas': '#fbfaf7',
    '--apple-surface-panel': '#fffefb',
    '--apple-surface-elevated': '#ffffff',
    // 实心材质下 float 与 elevated 同色：分档服务材质的透出量，纸面没有可透的东西
    '--apple-surface-float': '#ffffff',
    '--apple-surface-field': '#f6f4ef',
    // 编辑部：层级本来就靠色阶 + 横线，主对象给最白的一张纸配一档克制的投影
    '--apple-surface-primary': '#ffffff',
    '--apple-shadow-primary': '0 1px 3px rgba(20,18,16,0.08), 0 12px 28px rgba(20,18,16,0.12)',

    '--apple-bg-secondary': '#f6f4ef',
    '--apple-bg-tertiary': '#efece4',
    '--apple-bg-hover': '#f3f0e9',

    '--apple-surface-knob': '#ffffff',
    '--apple-on-accent': '#ffffff',
    '--apple-overlay': 'rgba(20,18,16,0.4)',

    '--apple-border': '#dcd8cd',
    '--apple-border-light': '#eae7de',
    '--apple-soft-border-gray': 'rgba(20,18,16,0.18)',
    '--apple-mid-border-gray': '#8b8378',
    '--apple-utility-dark-gray': '#5d574f',

    '--apple-blue': '#b3261e',
    '--apple-green': '#3f6b52',
    '--apple-red': '#b3261e',
    '--apple-orange': '#b26a24',
    '--apple-yellow': '#a5811c',
    '--apple-purple': '#6b4a7a',
    '--apple-teal': '#33696d',
    '--apple-indigo': '#4a4a72',
    '--apple-pink': '#a03a52',

    '--apple-code-bg': '#f6f4ef',
    '--apple-code-fg': '#141210',
    '--apple-code-green': '#3f6b52',
    '--apple-code-blue': '#33696d',
    '--apple-code-purple': '#6b4a7a',
    '--apple-code-orange': '#b26a24',

    '--apple-scrollbar-thumb': 'rgba(20,18,16,0.20)',
    '--apple-scrollbar-thumb-hover': 'rgba(20,18,16,0.36)',

    '--apple-glass-border-nav': 'rgba(20,18,16,0.14)',
    '--apple-glass-blur': 'none',
    '--apple-blur-panel': 'none',

    '--apple-radius-card': '0px',
    '--apple-radius-control': '0px',
    '--apple-radius-panel': '0px',
    '--apple-radius-float': '0px',

    '--apple-shadow-card': 'none',
    '--apple-shadow-card-hover': '0 2px 8px rgba(20,18,16,0.10)',
    '--apple-shadow-card-hover-refined': '0 2px 8px rgba(20,18,16,0.10)',
    '--apple-shadow-modal': '0 18px 44px rgba(20,18,16,0.18), 0 0 0 1px rgba(20,18,16,0.14)',
    '--apple-shadow-dropdown': '0 8px 22px rgba(20,18,16,0.12), 0 0 0 1px rgba(20,18,16,0.12)',
    '--apple-shadow-level1': '0 0 0 1px rgba(20,18,16,0.12)',
    '--apple-shadow-level2': '0 2px 8px rgba(20,18,16,0.10)',
    '--apple-shadow-level3': '0 4px 14px rgba(20,18,16,0.12)',
    '--apple-shadow-subtle': 'none',
    '--apple-shadow-nav': '0 1px 0 rgba(20,18,16,0.16)',
    '--apple-shadow-inner-focus': 'inset 0 0 0 1px rgba(20,18,16,0.16)',

    // 编辑部排版：正文也上衬线，标题字距收紧
    '--apple-font-display': SERIF,
    '--apple-font-text': SERIF,
  },

  /**
   * 暖纸（浅 · 阅读）：Oil Book（book.oiloil.org）同款气质 —— 暖白纸面 + 炭黑墨 +
   * 琥珀金强调 + 现代无衬线。取色直接对照其线上 CSS（:root 与 [data-theme=dark] 两套）：
   * 纸 #faf8f4 / 面 #fffefc / 墨 #262421 / 二三级 #57534c·#6e685f / 线 #e7e1d6 / 琥珀 #f2c45a。
   * 琥珀只做**装饰与图标**（它的 1.5:1 在原站也只够干这个）；文字级强调一律压深到
   * #7c5522 一档（对纸 6.4:1），按钮底与选中态由派生族从 --apple-blue 染色跟随。
   * 纸面没有玻璃（同纸感/编辑部：glass-blur none），层级靠暖影与发丝线。
   */
  'warm-paper': {
    '--apple-apple-action-blue': '#7c5522',
    '--apple-body-link-blue': '#8a5e26',
    '--apple-high-luminance-link-blue': '#9c6a2c',

    '--apple-near-black-ink': '#262421',
    '--apple-text-primary': '#262421',
    '--apple-text-secondary': '#57534c',
    '--apple-text-tertiary': '#6e685f',
    '--apple-text-quaternary': '#a29a8d',
    '--apple-text-80': 'rgba(38,36,33,0.8)',
    '--apple-text-48': 'rgba(38,36,33,0.48)',
    '--apple-text-disabled': 'rgba(38,36,33,0.32)',
    '--apple-secondary-neutral-gray': '#57534c',

    '--apple-pale-apple-gray': '#f5f2ec',
    '--apple-light-gray': '#f5f2ec',
    '--apple-gray': '#c9c2b6',
    '--apple-gray-50': '#faf8f4',
    '--apple-pure-white-canvas': '#faf8f4',
    // 壳层比纸面暖灰一档，卡片最白（对照原站 paper / cover-bg / surface 三档）
    '--apple-surface-chrome': '#f2ede4',
    '--apple-surface-canvas': '#faf8f4',
    '--apple-surface-panel': '#fffefc',
    '--apple-surface-elevated': '#fffefc',
    '--apple-surface-float': '#fffefc',
    '--apple-surface-field': '#f2ede4',
    '--apple-surface-primary': '#fffefc',
    '--apple-shadow-primary': '0 1px 3px rgba(38,36,33,0.06), 0 12px 28px rgba(38,36,33,0.10)',

    '--apple-bg-secondary': '#f5f2ec',
    '--apple-bg-tertiary': '#efe9df',
    '--apple-bg-hover': '#f2ece1',

    '--apple-surface-knob': '#ffffff',
    // 琥珀底上白字只有 1.5:1；深墨压在 #f2c45a 上约 5.9:1，原站也是深字压琥珀
    '--apple-on-accent': '#262421',
    '--apple-overlay': 'rgba(38,36,33,0.32)',

    '--apple-border': '#e7e1d6',
    '--apple-border-light': '#efe9df',
    '--apple-soft-border-gray': '#e7e1d6',
    '--apple-mid-border-gray': '#c9c2b6',
    '--apple-utility-dark-gray': '#8a8275',

    // 状态色：琥珀金是主强调，其余取大地色系，不跳色相
    '--apple-blue': '#7c5522',
    '--apple-green': '#3f7d5a',
    '--apple-red': '#b04a38',
    '--apple-orange': '#c07a2e',
    '--apple-yellow': '#a97e1f',
    '--apple-purple': '#7d5f86',
    '--apple-teal': '#4a8a80',
    '--apple-indigo': '#565e82',
    '--apple-pink': '#a0506a',

    '--apple-code-bg': '#f5f1ea',
    '--apple-code-fg': '#262421',
    '--apple-code-green': '#3f7d5a',
    '--apple-code-blue': '#4a6d8c',
    '--apple-code-purple': '#7d5f86',
    '--apple-code-orange': '#b06a2a',

    '--apple-scrollbar-thumb': 'rgba(38,36,33,0.18)',
    '--apple-scrollbar-thumb-hover': 'rgba(38,36,33,0.32)',

    '--apple-glass-border-nav': 'rgba(38,36,33,0.10)',
    '--apple-glass-blur': 'none',
    '--apple-blur-panel': 'none',

    '--apple-radius-card': '10px',
    '--apple-radius-control': '8px',
    '--apple-radius-panel': '6px',
    '--apple-radius-float': '12px',

    '--apple-shadow-card': '0 1px 2px rgba(38,36,33,0.05)',
    '--apple-shadow-card-hover': '0 4px 14px rgba(38,36,33,0.10)',
    '--apple-shadow-card-hover-refined': '0 3px 10px rgba(38,36,33,0.08)',
    '--apple-shadow-modal': '0 16px 44px rgba(38,36,33,0.16), 0 0 0 1px rgba(38,36,33,0.08)',
    '--apple-shadow-dropdown': '0 8px 24px rgba(38,36,33,0.12), 0 0 0 1px rgba(38,36,33,0.06)',
    '--apple-shadow-level1': '0 1px 2px rgba(38,36,33,0.06)',
    '--apple-shadow-level2': '0 2px 8px rgba(38,36,33,0.08)',
    '--apple-shadow-level3': '0 4px 14px rgba(38,36,33,0.10)',
    '--apple-shadow-subtle': '0 1px 2px rgba(38,36,33,0.04)',
    '--apple-shadow-nav': '0 1px 0 rgba(38,36,33,0.06)',
    '--apple-shadow-inner-focus': 'inset 0 0 0 1px rgba(38,36,33,0.12)',

    '--apple-font-display': WARM_PAPER_SANS,
    '--apple-font-text': WARM_PAPER_SANS,
  },
};

/** base → palette 差异表（'light' 无差异表 = :root 默认） */
export function themeOverrideMapFor(id: ThemeId): ThemeOverrideMap | null {
  if (id === 'light') return null;
  if (id === 'dark') return DARK_THEME;
  return themeOverrides[id as ExtendedThemeId];
}

/**
 * (base, material) → 该组合实际生效的 token 差异表（相对 :root 浅色默认）。
 * solid 就是 base palette；glass 是 base palette 叠上对应外观的玻璃 overlay。
 * 注入、缩略色块、门禁、运行时测试都以这里为唯一事实源，不会出现「注入和校验各算一套」。
 */
export function resolveThemeTokens(base: ThemeId, material: Material): Record<string, string> {
  const palette = themeOverrideMapFor(base);
  const glass = glassOverlayFor(base, material);
  return { ...(palette ?? {}), ...(glass ?? {}) };
}

// ==================== 主题额外部样式（非变量类） ====================
type ThemeExtraCss = Partial<Record<ThemeId, string>>;

/** base 专属非变量样式。玻璃的环境层属于**材质轴**，另走 glassExtraCss() */
export const themeExtraCss: ThemeExtraCss = {
  /** 杂志排版：全直角 + 大标题字距收紧 + 数据表数字等宽对齐 */
  editorial: `
:root[data-theme='editorial'] h1,
:root[data-theme='editorial'] h2,
:root[data-theme='editorial'] h3 { letter-spacing: -0.015em; }
:root[data-theme='editorial'] table { border-collapse: collapse; font-variant-numeric: tabular-nums; }
:root[data-theme='editorial'] th,
:root[data-theme='editorial'] td { border-color: var(--apple-border); }`,
};

/**
 * Clear（透明增强）变体：比默认玻璃更透一档。
 * **它是数据表不是 CSS 字符串** —— 早先这段直接写死在 glassExtraCss() 的模板里，
 * 于是 check-contrast 想量玻璃档就只能自己抄一份值，而"判据的合法值不许自己抄一份"
 * 正是这个仓库反复踩过的坑（E1 圆角那次抄了一份空集，把 1164 处玻璃圆角全报成违例）。
 * DOM 上由 `html[data-glass='clear']` 触发。
 */
export const GLASS_CLEAR_OVERLAY: Record<'light' | 'dark', GlassOverlayMap> = {
  light: {
    // 2026-11-15 前端评审 P1-1：clear 档的 panel 0.56 压在 #f2f1ed 画布上只差 8 级明度，
    // 卡片边界读不出来（统计卡被当成"页面空隙"）。clear 的方向是"更透"，但不是"没有面"：
    // panel 抬到 0.70（仍比标准玻璃 0.66 只高一档、比 opaque 0.94 低），chrome 同步 0.60 保住
    // 壳层 < 面板的层级；border-light 提到 0.14，让 1px 发丝边在浅底上真的看得见。
    '--apple-surface-chrome': 'rgba(255,255,255,0.60)',
    '--apple-surface-panel': 'rgba(255,255,255,0.70)',
    '--apple-border-light': 'rgba(31,41,75,0.14)',
    // 浮层两档不跟着一起变透：顶栏在内容滚过来时压的是**别人家的内容**，菜单/弹窗上的
    // 三级文字也各有自己的地板，这两件事和 clear 是两回事（判据见 check-contrast.mjs 的两组浮层断言）。
    // 所以 clear 表与 opaque 表**故意不同形**：后者必须把 float 也退回实心，否则
    // "减弱透明度"偏好下浮层还是半透明、只是没了模糊 —— 那正是这一档要避免的状态。
    '--apple-surface-elevated': 'rgba(255,255,255,0.84)',
    '--apple-surface-field': 'rgba(255,255,255,0.80)',
    '--apple-glass-blur': 'blur(16px) saturate(160%)',
    '--apple-blur-panel': 'blur(8px) saturate(140%)',
  },
  dark: {
    // 2026-11-01 材质三档：chrome 0.42 → 0.34（近黑 tint）→ 0.32（提亮 tint）。
    // 只降 alpha 的二档实测：顶栏确实透出壁纸色相了，但侧栏与内容只差 (7,5,1) ——
    // 近黑 tint 在近黑壁纸上提不起明度，壳层与画布落在同一级，玻璃没有"面"。
    // 深色 vibrancy 的方向是"提亮 + 轻微去饱和"，不是再涂一层深色。
    // 2026-11-01 三轮（去 AI 感）：tint 跟常规档一起换到中性石墨 134,137,144（口径见常规档注释）。
    // clear 再薄一档（0.24）：clear 的卖点是"看得见壁纸"，比标准档少 0.08 的 tint 就够，
    // 再薄会掉到 0.30 那一轮的"没有面"。
    // 2026-11-01 四轮：0.24 → 0.28。画布压深到 #0b0b0e 之后，0.24 的壳层落回 41，
    // 比正文面（primary 0.62）还低 —— 侧栏就读作"凹进墙里"。0.28 让壳层重新站在
    // 内容面之上，仍是四档里最透的一档（70% 壁纸可见）。
    // 2026-11-01 九轮：跟常规档一起压深（134,137,144 → 114,116,122，合成 45 → 40），
    // 与 primary（38）的台阶仍有 2 —— clear 是最透的一档，台阶本来就最窄，别再压深 primary。
    '--apple-surface-chrome': 'rgba(114,116,122,0.28)',
    // panel 与常规档同一条口径：内容面保持深色板、同一族，只是 clear 再薄一点。
    // 2026-11-01 四轮：38,40,45 / 0.44 → 58,60,67 / 0.50（同常规档的理由：面先立起来）。
    // 2026-11-01 九轮：58,60,67 → 46,48,54（跟常规档，合成 35 → 29）。
    '--apple-surface-panel': 'rgba(46,48,54,0.50)',
    // 与常规玻璃的 elevated 同一处理：clear 的通透体现在 chrome/panel 两档，
    // 压上滚动内容的壳面（顶栏 under 态）一样要扛亮内容 —— 理由见常规档注释。
    // clear 的两个档位在这里必须**同明度**（0.28 × 114 与 0.90 × 44 都落在 40 上下）：
    // clear 是更透的变体，但"更透"只许发生在静止档，不许变成"滚起来换一块更黑的板"。
    '--apple-surface-elevated': 'rgba(44,45,50,0.90)',
    '--apple-surface-field': 'rgba(0,0,0,0.22)',
    // saturate 与 static 档统一到 150%：环境层去紫后剩的是明度纹理，saturate 对灰是空操作，
    // 保留同一档只是让材质三档的 blur 表达式一致，不隐含"clear 更艳"
    '--apple-glass-blur': 'blur(16px) saturate(150%)',
    '--apple-blur-panel': 'blur(8px) saturate(140%)',
  },
};

/** 无障碍：用户偏好减弱透明度时，玻璃退回该外观的不透明表面（浮层两档都必须在这里收口，理由见上面 clear 的注释） */
export const GLASS_OPAQUE_OVERLAY: Record<'light' | 'dark', GlassOverlayMap> = {
  light: {
    // 与 GLASS_OVERLAY.light 同一族中性色：减弱透明度后画布就是环境层底色本身，
    // 不能在这里又冒出一块蓝白（旧值 #f3f6fc 在暖纸画布上像另一套主题）。
    '--apple-surface-chrome': '#f7f7f5',
    '--apple-surface-canvas': '#f4f4f2',
    '--apple-surface-panel': '#f6f6f4',
    '--apple-surface-elevated': '#ffffff',
    '--apple-surface-float': '#ffffff',
    '--apple-surface-field': '#ffffff',
    '--apple-glass-blur': 'none',
    '--apple-blur-panel': 'none',
  },
  dark: {
    // 与 GLASS_OVERLAY.dark 同一套石墨族，逐档取「该 alpha 压在 #0b0b0e 上的合成值」：
    // 旧值用的是 Apple 深色原样（#1c1c1e / #000），减弱透明度后整页会换一套色温，
    // 与浅色那一档的教训相同（canvas 直接取环境层底色，不再掉到纯黑）。
    '--apple-surface-chrome': '#2c2d31',
    '--apple-surface-canvas': '#0b0b0e',
    '--apple-surface-panel': '#1f2025',
    // 减弱透明度档同样是实心：加厚档跟壳层同色（理由见 DARK_THEME 的 elevated 那条注释）
    '--apple-surface-elevated': '#292a2e',
    '--apple-surface-float': '#1f1f22',
    '--apple-surface-field': '#18191c',
    '--apple-glass-blur': 'none',
    '--apple-blur-panel': 'none',
  },
};

function renderOverlayBlock(map: GlassOverlayMap, indent: string): string {
  return Object.entries(map)
    .map(([key, value]) => `${indent}${key}: ${value};`)
    .join('\n');
}

/**
 * 「提高对比度」偏好下的补强：**这是与 reduce-transparency 独立的一条信号**，
 * 只处理前者等于漏掉一批用户 —— Apple 的材料在 Increase Contrast 下给的是
 * "近实心 + 明确的实边"，边不再是装饰而是分层手段。
 * 叠在 GLASS_OPAQUE_OVERLAY 之上（那条负责把表面退回不透明，这条负责把边描实）。
 */
export const GLASS_CONTRAST_OVERLAY: Record<'light' | 'dark', GlassOverlayMap> = {
  light: {
    '--apple-border': 'rgba(31,41,75,0.42)',
    '--apple-border-light': 'rgba(31,41,75,0.30)',
    '--apple-soft-border-gray': 'rgba(31,41,75,0.42)',
    '--apple-mid-border-gray': 'rgba(30,45,90,0.46)',
    '--apple-utility-dark-gray': 'rgba(30,45,90,0.50)',
    '--apple-glass-border-nav': 'rgba(31,41,75,0.30)',
    // 模糊在这一档下没有意义（表面已近实心），留着只会让边缘发虚、把刚描实的边糊掉
    '--apple-blur-panel': 'none',
  },
  dark: {
    '--apple-border': 'rgba(255,255,255,0.44)',
    '--apple-border-light': 'rgba(255,255,255,0.30)',
    '--apple-soft-border-gray': 'rgba(255,255,255,0.44)',
    '--apple-mid-border-gray': 'rgba(255,255,255,0.48)',
    '--apple-utility-dark-gray': 'rgba(255,255,255,0.52)',
    '--apple-glass-border-nav': 'rgba(255,255,255,0.34)',
    '--apple-blur-panel': 'none',
  },
};

/**
 * 玻璃环境层的底色（那些 radial-gradient 铺在它上面）。
 * 单独导出是因为**两个方向都要读它**：glassExtraCss 用它写 CSS，
 * check-contrast 用它当「流水面」的合成背景 —— 门禁不许自己抄一份色值，
 * 这是 E1 圆角那次"合法集自己抄了一份空集，把 1164 处玻璃圆角全报成违例"学到的。
 */
export const GLASS_ENV_BASE_COLOR: Record<'light' | 'dark', string> = {
  // 2026-11-14 unslop：浅色环境层原先也是蓝/粉/青/紫四片光斑（#eef3fc 底 + 饱和色 stop），
  // 与深色那轮同一个问题 —— 玻璃一折射就成了 AI 产品的渐变海报。这里同步收为中性纸面：
  // 底色只保留纸的微暖，起伏全部走明度（一盏顶光 + 一点地面暗部），不再有可数的色斑。
  // 2026-11-14 二轮（用户判词「浅色页面背景色都不统一」）：上一版 #f2f1ed 的暖调（B-R=-5）
  // 压在纯白表面下仍是"侧栏发白、内容区发黄"——表面阶梯已经中性，画布却还偏黄。
  // 收到 #f4f4f2（B-R=-2，与 surface/bg 族的 -1~-2 同档），明度抬到 244：
  // 与卡片合成色（≈250）只差 6，页面上不再出现"白一片、黄一片"的两种底色。
  light: '#f4f4f2',
  // 深色底取中性石墨（微冷不带紫）：玻璃要折射出质感，但 Apple 的深色材质语言是
  // 中性近黑 + 低饱和，环境层给蓝紫彩色光斑读出来就是 AI 产品的渐变海报（用户判词
  // 「太有 AI 的感觉」）。纹理靠明度起伏，不靠色相。
  // 2026-11-01 四轮（「依然很粗糙」）：#101014 → #0b0b0e。上一版的房间太亮，
  // 卡片（panel 0.44 压在它上面）只比画布高 11 个亮度单位，而描边高 18 —— 面没了、
  // 只剩一圈线，整页读成"线框图"。深色玻璃的层级必须靠**明度台阶**，台阶的高度取决于
  // 地板有多低：地板压深到 11，同一块半透明板才会显形。
  dark: '#0b0b0e',
};

/**
 * 玻璃材质的环境层 + scroll edge + Clear 变体 + 无障碍退化，按外观分浅/深两套。
 * 选择器 `[data-theme='<appearance>'][data-material='glass']`，与注入的 token 块同源。
 * 环境层是玻璃要折射的「壁纸」：浅色是中性纸面的明度起伏，深色是石墨族的明度起伏（去 AI 感口径见下）。
 */
export function glassExtraCss(appearance: 'light' | 'dark'): string {
  const g = `:root[data-theme='${appearance}'][data-material='glass']`;
  const env =
    appearance === 'light'
      ? `background-color: ${GLASS_ENV_BASE_COLOR.light};
  /* 2026-11-14 unslop：浅色环境层与深色同一口径 —— 一盏顶光 + 一点地面暗部，
     只动明度不动色相。原来的四片蓝/粉/青/紫光斑在玻璃折射后读作"AI 渐变海报"，
     且每片形状清晰可数；收成一盏后画面里只有光照关系，没有可数的光斑个数。 */
  background-image:
    radial-gradient(70% 100% at 18% 0%, rgba(255,255,255,0.92), rgba(255,255,255,0.34) 42%, rgba(255,255,255,0.08) 68%, rgba(255,255,255,0) 92%),
    radial-gradient(120% 90% at 92% 108%, rgba(30,30,30,0.05), transparent 72%);`
      : `background-color: ${GLASS_ENV_BASE_COLOR.dark};
  /* 2026-11-01 去 AI 感：原来五层是蓝/紫/青的饱和彩色光斑（紫 0.44、蓝 0.50），
     玻璃一折射就成 AI 产品的渐变海报（用户判词）。Apple 深色材质的语言是中性近黑 +
     低饱和 —— 环境层改成石墨族的**明度**起伏：只动亮度、几乎不动色相，玻璃照样有
     可折射的明暗层次，但读出来是"磨砂石墨"而不是霓虹。
     2026-11-01 四轮：峰值统一收到 0.18 以下。环境层是**墙**不是**灯** ——
     光斑最亮处一旦高过卡片（面板 0.5 alpha 压在它上面），卡片就被背景吞掉，
     层级关系倒转。深色玻璃的观感靠"墙暗、板亮、壳最亮"这条阶梯，
     所以墙的起伏只负责让玻璃有东西可折射，不负责抢明度。
     2026-11-01 五轮：布局也换了。原来五个光斑全钉在四角，屏幕中段是空的，
     于是**同一块壳面从上到下掉 20+ 个亮度单位**（侧栏实测上 #2a、下 #14）——
     那读起来是一条渐变漆，不是一种材质。现在改成"顶部一条宽光 + 几片大尺度低幅
     起伏铺满整屏"：任何一列（侧栏、正文、底栏）在自己的高度上都只有 ~10 级的缓慢
     变化，玻璃才有资格被读成一块**均匀**的磨砂板，而不是一面镜子。
     2026-11-01 六轮（「环境光不真实」）：五片椭圆仍有形状。不是振幅的问题，是**轮廓**：
     五片不同圆心的椭圆叠在一起，明暗交界处会拼出一条斜向的亮楔（用户截图里那道对角光带），
     眼睛先读到"有几块光斑"再读到"有光"。环境光不该有可数出来的个数。
     于是收成一盏、中心挪到视口外，视野里只截到平滑中段。
     2026-11-01 七轮（「穿透效果为什么都没了」）：六轮把灯挪得太远、振幅也压得太狠，
     壁纸在画面里几乎是一条平黑（实测顶部 18、两侧 16）—— 壳层 alpha 0.28 透出去的是
     **同一块灰**，玻璃读成了刷上去的漆。穿透感不是靠把壳层调透，而是靠**身后有可透的东西**。
     灯因此从视口外挪进画面：圆心回到默认光源坐标（18% / 0%，与 --apple-light-x/y 同一处，
     环境光与玻璃上的反光是同一盏灯），半径收到 70%×100% —— 比视口还大，但仍让左上角
     截到**峰**而不是肩部。顶栏左端与侧栏顶端因此同亮（实测合成 70，接缝两侧差 0.2 级，
     不再有"侧栏自己又亮一截"的错觉），侧栏从上到下走完整条 70 → 46 的明暗（玻璃在折射），
     右端与底部仍是近黑。仍未超过一盏、单个椭圆，三段 stop（42% / 68% / 92%）铺得很缓，
     画面里没有可数的边界。
     对比度手算（合成值 → 文字档；不是拍脑袋）：受光处壳面 70，次级 #cacacf 5.7、正文 8.3；
     侧栏唯一的裸三级字「管理」处壳面 55，三级 6.0 —— 门禁只按平底色量，这几处是按峰值核的。
     卡片在受光处 50 比画布 33 高 17、暗处 41 比 12 高 29 —— 墙不亮过板这条阶梯没破。
     底部一片极弱、左右对称的地面反光，只为把下半屏从纯黑里托起来。 */
  background-image:
    radial-gradient(70% 100% at 18% 0%, rgba(206,210,219,0.24), rgba(206,210,219,0.06) 42%, rgba(206,210,219,0.015) 68%, rgba(206,210,219,0) 92%),
    radial-gradient(150% 90% at 50% 116%, rgba(150,154,163,0.05), transparent 70%);`;
  const clearBlock = renderOverlayBlock(GLASS_CLEAR_OVERLAY[appearance], '  ');
  const opaqueBlock = renderOverlayBlock(GLASS_OPAQUE_OVERLAY[appearance], '    ');
  const contrastBlock = renderOverlayBlock(GLASS_CONTRAST_OVERLAY[appearance], '    ');
  return `
${g} body {
  ${env}
  background-attachment: fixed;
}

/* 滚动边缘效应（scroll edge）的下半：内容不是被顶栏下沿一条硬边切断，而是溶解进壳里。
   顶栏 2026-10-15 浮层化之后这条才成立 —— 此前顶栏在 flex 列里独占一行，
   给它加渐隐等于在不透明主题的内容上凭空画一条白带，所以当时刻意没做。
   只在玻璃材质出现；solid/纸感下这整块规则不存在。
   下沿渐隐的颜色跟着壳的档位走：静止态 chrome，内容压上来时 elevated（与加厚同一档）。
   2026-11-01 七轮：这条带子原先 left:0 横贯整行，可**侧栏那一列没有内容从底下滚过**
   （它自己就是壳，从顶栏下沿起算，见 MainLayout 的 sidebarShellStyle）—— 铺过去等于
   在侧栏顶端再盖一遍 chrome，又一处"一份材质涂两遍"，实测侧栏 y=48 处 87 对自身中段 70。
   更糟的是它跟着顶栏的滚动档位走：同样的像素在滚动后变成 elevated 的 53 —— 侧栏顶端
   会因为**正文滚动**而变色，这在物理上毫无理由。
   让位宽度由 MainLayout 写在根上的 --ap-shell-sidebar-width 提供（移动端抽屉/全屏页是 0px，
   退回整行）。变量缺席时按 0px 兜底，即老行为，不会把带子整条吃掉。 */
${g} [data-ap='nav-bar']::after {
  content: '';
  position: absolute;
  left: var(--ap-shell-sidebar-width, 0px);
  right: 0;
  bottom: -18px;
  height: 18px;
  background: linear-gradient(to bottom, var(--apple-surface-chrome), transparent);
  pointer-events: none;
}
${g} [data-ap='nav-bar'][data-scroll-edge='under']::after {
  background: linear-gradient(to bottom, var(--apple-surface-elevated), transparent);
}

/* Clear 变体：更高透明度 + 更轻模糊（html[data-glass='clear']） */
${g}[data-glass='clear'] {
${clearBlock}
}

/* 无障碍：用户偏好减弱透明度时，玻璃退化为对应外观的不透明表面。
   **两个入口**：① 系统 prefers-reduced-transparency（且这台没在应用内选 full，由护栏把关）；
   ② 应用内显式选了 reduce。两份声明由同一个 opaqueBlock 变量拼出来 ——
   各抄一遍的话，将来调那一档只调了一处，症状是"从系统开关进来的用户和从偏好页进来的用户看到不同的板子"。 */
@media (prefers-reduced-transparency: reduce) {
  ${g}:not([data-a11y-transparency='full']) {
${opaqueBlock}
  }
  /* 表面不透明之后，下沿那条渐隐会变成内容上凭空一条白带 —— 一起关掉 */
  ${g}:not([data-a11y-transparency='full']) [data-ap='nav-bar']::after { display: none; }
}
${g}[data-a11y-transparency='reduce'] {
${opaqueBlock}
}
${g}[data-a11y-transparency='reduce'] [data-ap='nav-bar']::after { display: none; }

/* 无障碍：提高对比度偏好 —— 与上一条是**两个独立信号**（系统里可以只开其中一个）。
   Apple 在这一档给的是"近实心 + 明确的实边"：边从装饰变成分层手段，
   所以除了退回表面，还要把 border 一族描实、并撤掉会把边糊掉的模糊。
   只处理 reduced-transparency 就等于漏掉这批用户。 */
@media (prefers-contrast: more) {
  ${g}:not([data-a11y-contrast='standard']) {
${opaqueBlock}
${contrastBlock}
  }
}
${g}[data-a11y-contrast='more'] {
${opaqueBlock}
${contrastBlock}
}

/* 减弱动效：环境层不再跟随视口固定。同样两个入口（系统信号 + 应用内 reduced 档），
   护栏与 apple-tokens.ts 的全局通配共用同一条判断（MOTION_GUARD 那个形状）。 */
@media (prefers-reduced-motion: reduce) {
  ${g}:not([data-motion='full']):not([data-motion='soft']) body { background-attachment: scroll; }
}
${g}[data-motion='reduced'] body { background-attachment: scroll; }`;
}

/**
 * 从 (base, material) 实际生效的差异表算出缩略色块，供设置页/菜单画预览——预览不可能与实现分叉。
 * baseVars 传 cssVariables（:root 默认值），light+solid 没有差异表时取默认。
 */
export function themeSwatch(
  id: ThemeId,
  baseVars: Record<string, string>,
  material: Material = 'solid',
): { bg: string; surface: string; accent: string; text: string; radius: string } {
  const map = resolveThemeTokens(id, material);
  const pick = (key: string) => map[key] ?? baseVars[key] ?? '';
  return {
    // 玻璃的 canvas 是 transparent，缩略块用它身后的环境底色代替（浅/深各一档）
    bg:
      pick('--apple-surface-canvas') === 'transparent'
        ? isDarkThemeId(id)
          ? '#0a0a12'
          : '#eef2fb'
        : pick('--apple-surface-canvas'),
    surface: pick('--apple-surface-panel'),
    accent: pick('--apple-blue'),
    text: pick('--apple-text-primary'),
    radius: pick('--apple-radius-card'),
  };
}
