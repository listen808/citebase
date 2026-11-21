/**
 * Apple Design Tokens —— 全项目唯一样式取值来源
 * 规范入口：根目录 AGENTS.md「Frontend Conventions」+ web/src/styles/README.md（语义色对照表、硬规则）
 * 早期 DESIGN.md 规范文件未随仓库归档，数值以本文件为准
 * 禁止创建 CSS 文件，所有样式通过此 Token + 组件内联样式实现
 * 校验：npm run check:style（变量注册表一致性 / 非法颜色拼接 / 裸 z-index / 非 scoped 样式块）
 */

import {
  THEME_REGISTRY,
  themeExtraCss,
  glassExtraCss,
  resolveThemeTokens,
  isDarkThemeId,
  supportsGlass,
} from '../../styles/theme/themes';
import { MOTION_GUARD } from '../../styles/theme/personalization';

// ==================== CSS 变量注册 ====================
// 在 main.ts 中调用 injectCSSVariables() 将自动注入到 :root

// ==================== 颜色 ====================
// 命名规范（详见 web/src/styles/README.md §4）：
//   · 语义角色名（bg-primary / text-secondary / border-light 等）为首选，描述"用在哪"，浅深两套值由 cssVariables 成对维护；
//   · DESIGN.md 的 primitive 名（pureWhiteCanvas / nearBlackInk / softBorderGray 等）与角色名同值并存，保留但不推动迁移；
//   · 写错变量名会被 cv() 的 AppleCssVarName 类型直接拦下，不依赖 @deprecated 提醒。
export const AppleColor = {
  // Primary - DESIGN.md §2
  absoluteBlack: '#000000',
  paleAppleGray: '#f5f5f7',
  nearBlackInk: '#1d1d1f',

  // Secondary & Accent - DESIGN.md §2
  appleActionBlue: '#0071e3',
  bodyLinkBlue: '#0066cc',
  highLuminanceLinkBlue: '#2997ff',

  // Surface & Background - DESIGN.md §2
  pureWhiteCanvas: '#ffffff',

  // Neutrals & Text - DESIGN.md §2
  secondaryNeutralGray: '#6e6e73',
  softBorderGray: '#d2d2d7',
  midBorderGray: '#86868b',
  utilityDarkGray: '#424245',

  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  white: '#ffffff',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  lightGray: '#f5f5f7',

  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  textPrimary: '#1d1d1f',
  /**
   * 次级文字梯度：按 WCAG AA 单调排列（白底 5.30 / 5.07 / 4.64，卡片底 #f5f5f7 4.87 / 4.66 / 4.27）。
   * 早期三者取 #86868b 时正文对比度仅 3.62，故与 primitive 灰不再同值。
   */
  textSecondary: '#6b6b70',
  textTertiary: '#6e6e73',
  textQuaternary: '#74747a',

  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  appleBlue: '#0071e3',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  appleBlueLight: '#e6f2ff',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  appleGreen: '#34c759',
  systemGreen: '#34c759',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  appleGreenLight: '#e6f9ef',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  appleRed: '#ff3b30',
  systemRed: '#ff3b30',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  appleRedLight: '#ffebeb',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  appleOrange: '#ff9500',
  systemOrange: '#ff9500',
  systemYellow: '#ffcc00',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  orangeLight: '#fff3e0',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  applePurple: '#af52de',
  systemPurple: '#af52de',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  applePurpleLight: '#f5e6ff',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  appleTeal: '#5ac8fa',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  appleYellow: '#ffcc00',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  appleIndigo: '#5856d6',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  applePink: '#ff2d55',

  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  border: '#e5e5e5',
  /** 极浅分隔线（分隔线/表格线首选，等价 cv('border-light')） */
  borderLight: '#f0f0f0',

  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  bgPrimary: '#ffffff',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  bgSecondary: '#f5f5f7',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  bgTertiary: '#f0f0f0',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  bgHover: '#f5f5f7',

  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  success: '#34c759',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  warning: '#ff9500',
  /** 语义角色名（首选）；对应 primitive 名同值并存，不要求迁移 */
  error: '#ff3b30',

  // 半透明文字
  black80: 'rgba(29,29,31,0.8)',
  black48: 'rgba(29,29,31,0.48)',

  // 遮罩
  overlay: 'rgba(0,0,0,0.5)',

  gray: '#86868b',
  gray50: '#fafafa',

  // 代码语法高亮
  codeBg: '#f6f8fa',
  codeFg: '#24292e',
  codeGreen: '#22863a',
  codeBlue: '#0366d6',
  codePurple: '#6f42c1',
  codeOrange: '#e36209',

  // 半透明白色（图片预览覆盖层、全局搜索提示栏）
  whiteAlpha15: 'rgba(255,255,255,0.15)',
  whiteAlpha20: 'rgba(255,255,255,0.2)',
  whiteAlpha30: 'rgba(255,255,255,0.3)',
  whiteAlpha70: 'rgba(255,255,255,0.7)',
  whiteAlpha80: 'rgba(255,255,255,0.8)',

  // 滚动条
  scrollbarThumb: 'rgba(0,0,0,0.18)',
  scrollbarThumbHover: 'rgba(0,0,0,0.32)',
} as const;

// ==================== 状态色分组 - 语义化引用 ====================
// 设计规范 §2: 状态色用于反馈、提示、警示场景
// 所有值复用 AppleColor，避免重复定义
export const AppleStatusColor = {
  /** 成功 - 复用 appleGreen */
  success: AppleColor.appleGreen,
  successLight: AppleColor.appleGreenLight,
  /** 警告 - 复用 appleOrange */
  warning: AppleColor.appleOrange,
  warningLight: AppleColor.orangeLight,
  /** 错误/危险 - 复用 appleRed */
  error: AppleColor.appleRed,
  errorLight: AppleColor.appleRedLight,
  /** 信息 - 复用 appleActionBlue */
  info: AppleColor.appleActionBlue,
  infoLight: AppleColor.appleBlueLight,
} as const;

// ==================== 中性色板分组 - 语义化引用 ====================
// 设计规范 §2: 中性色用于文字、背景、边框层级
// 所有值复用 AppleColor，避免重复定义
export const AppleNeutralColor = {
  // 文字层级（由深到浅）
  textPrimary: AppleColor.nearBlackInk,
  textSecondary: AppleColor.textSecondary,
  textTertiary: AppleColor.textTertiary,
  textQuaternary: AppleColor.textQuaternary,
  textDisabled: 'rgba(29,29,31,0.4)',
  // 背景层级（由浅到深）
  bgPrimary: AppleColor.pureWhiteCanvas,
  bgSecondary: AppleColor.paleAppleGray,
  bgTertiary: AppleColor.bgTertiary,
  bgHover: AppleColor.paleAppleGray,
  // 边框层级（由浅到深）
  borderLight: AppleColor.borderLight,
  border: AppleColor.softBorderGray,
  borderStrong: AppleColor.midBorderGray,
  borderEmphasis: AppleColor.utilityDarkGray,
} as const;

// ==================== 交互状态令牌 - hover/active/focus/disabled/loading ====================
// 设计规范 §4: 统一所有 Ap 交互组件的状态视觉表达
export const AppleInteractionState = {
  /** Disabled 透明度 - 统一 0.5 */
  disabledOpacity: '0.5',
  /** Disabled 光标 */
  disabledCursor: 'not-allowed',
  /** Active 按下缩放 - 统一 0.97 */
  activeScale: '0.97',
  /** Hover 微缩放 - 1.02 */
  hoverScale: '1.02',
  /** Hover 卡片提升 - translateY(-2px) */
  hoverLift: 'translateY(-2px)',
  /** Hover 卡片强提升 - translateY(-6px) */
  hoverLiftStrong: 'translateY(-6px)',
  /** Hover 亮度增强 - 用于深色背景按钮 */
  hoverBrighten: 'brightness(1.08)',
  /** Active 亮度减弱 - 用于深色背景按钮 */
  activeDim: 'brightness(0.88)',
  /** Focus 环样式（直接用于 boxShadow） */
  focusRing: 'var(--apple-blue-focus-ring)',
  /** Focus 环偏移 */
  focusRingOffset: '3px',
  /** Spinner 默认尺寸 */
  spinnerSizeSmall: '14px',
  spinnerSizeMedium: '24px',
  /** Spinner 边框宽度 */
  spinnerBorderWidthSmall: '2px',
  spinnerBorderWidthMedium: '3px',
  /** Spinner 时长：原先 0.8s×10 与 1s×1（SSOCallbackView）并存 */
  spinnerDuration: '0.8s',
} as const;

// ==================== 字体家族 - DESIGN.md §3 ====================
// 基础字族串（浅色默认值；主题通过覆盖 --apple-font-* CSS 变量切换衬线/等宽观感）
const FONT_DISPLAY_BASE =
  "-apple-system, BlinkMacSystemFont, 'SF Pro Display', 'SF Pro Icons', 'Helvetica Neue', Helvetica, Arial, sans-serif";
const FONT_TEXT_BASE = "-apple-system, BlinkMacSystemFont, 'SF Pro Text', 'SF Pro Icons', 'Helvetica Neue', Helvetica, Arial, sans-serif";

export const AppleFont = {
  // 引用 CSS 变量而非字面值：editorial 等主题可在不改动 270+ 处内联引用的前提下整体换字族
  display: 'var(--apple-font-display)',
  text: 'var(--apple-font-text)',
  systemBold: 'var(--apple-font-display)',
  mono: "'SF Mono', Monaco, Consolas, 'Liberation Mono', 'Courier New', monospace",
} as const;

// ==================== 字号阶梯 - 严格按照 DESIGN.md §3 ====================
export interface TypeStyle {
  size: string;
  weight: number;
  lineHeight: number;
  letterSpacing: string;
  font: string;
}

/**
 * TypeStyle → 真正的 CSS 属性。
 *
 * AppleType.* 的字段名是设计稿口径（size / weight / font），不是合法 CSS 属性名：
 * 直接 `...AppleType.microUI` 展开进 :style，浏览器丢掉 size 与 weight，而 `font`
 * 会被当作 font shorthand 解析（拿到的是 font-family 串，整条失效）——结果就是
 * 「以为设了 12px 小标签，实际继承 17px 正文」。要铺排版样式一律走这个函数。
 */
export function cssType(t: TypeStyle): {
  fontFamily: string;
  fontSize: string;
  fontWeight: string;
  lineHeight: number;
  letterSpacing: string;
} {
  return {
    fontFamily: t.font,
    fontSize: t.size,
    fontWeight: String(t.weight),
    lineHeight: t.lineHeight,
    letterSpacing: t.letterSpacing,
  };
}

/**
 * 字距是字号的函数，不是每个条目各抄一个数 —— 大字要负字距、小字要略微正字距，
 * 一个固定值必然在某些尺寸上是错的。
 *
 * 这张表原先是 22 处手写值，实测下来两头反了：17px 正文 -0.374px 是**全站最紧**，
 * 21px 页头反而 +0.231px 是**全站最松**，而 20px 数值(-0.2) 与 21px 标题(+0.231)
 * 这两个相邻档符号相反 —— 同一屏上并排的两种字号往相反方向收。跨页看每个尺寸都是
 * 单值（所以排版巡检一直绿），但"一致"指的是随字号单调，不是"每处都抄同一个数"。
 *
 * 现在只准由这里派生：单调递减，小尺寸微正、大尺寸微负，端点贴着原先已经正确的
 * display 档。每个尺寸的改动量都 < 0.5px/字，观感是"正文不再发挤、标题不再发松"。
 */
const TRACKING_ANCHORS: ReadonlyArray<readonly [number, number]> = [
  [10, 0.020], [12, 0.017], [13, 0.015], [14, 0.013], [15, 0.011], [16, 0.008],
  [17, 0.006], [19, 0.000], [20, -0.002], [21, -0.004], [24, -0.006], [28, -0.008],
  [32, -0.010], [40, -0.014], [48, -0.018], [56, -0.022], [80, -0.030],
];

/** 按字号取字距（该档 em × 字号，线性插值，落到 px）。超出端点取端点，不外推。 */
export function trackingFor(px: number): string {
  const [firstPx, firstEm] = TRACKING_ANCHORS[0] ?? [10, 0.02];
  const [lastPx, lastEm] = TRACKING_ANCHORS[TRACKING_ANCHORS.length - 1] ?? [80, -0.03];
  if (px <= firstPx) return `${(px * firstEm).toFixed(3)}px`;
  if (px >= lastPx) return `${(px * lastEm).toFixed(3)}px`;
  for (let i = 0; i < TRACKING_ANCHORS.length - 1; i++) {
    const a = TRACKING_ANCHORS[i];
    const b = TRACKING_ANCHORS[i + 1];
    if (!a || !b) continue;
    const [x0, y0] = a;
    const [x1, y1] = b;
    if (px >= x0 && px <= x1) {
      const t = (px - x0) / (x1 - x0);
      return `${(px * (y0 + t * (y1 - y0))).toFixed(3)}px`;
    }
  }
  return '0px';
}

export const AppleType = {
  // Hero & Display
  heroDisplayXL: {
    size: '80px',
    weight: 600,
    lineHeight: 1.05,
    letterSpacing: trackingFor(80),
    font: AppleFont.display,
  },
  heroDisplayL: {
    size: '56px',
    weight: 600,
    lineHeight: 1.07,
    letterSpacing: trackingFor(56),
    font: AppleFont.display,
  },
  sectionDisplay: {
    size: '48px',
    weight: 600,
    lineHeight: 1.1,
    letterSpacing: trackingFor(48),
    font: AppleFont.display,
  },
  productHeading: {
    size: '40px',
    weight: 600,
    lineHeight: 1.10,
    letterSpacing: trackingFor(40),
    font: AppleFont.display,
  },
  featureDisplay: {
    size: '38px',
    weight: 600,
    lineHeight: 1.21,
    letterSpacing: trackingFor(38),
    font: AppleFont.display,
  },
  promoDisplay: {
    size: '32px',
    weight: 600,
    lineHeight: 1.2,
    letterSpacing: trackingFor(32),
    font: AppleFont.display,
  },

  // Card & Tile
  cardProductTitle: {
    size: '28px',
    weight: 600,
    lineHeight: 1.14,
    letterSpacing: trackingFor(28),
    font: AppleFont.display,
  },
  /** 页面内分区标题：与卡片标题、页面标题拉开三级层次（28px 那档只留给营销页/错误页） */
  sectionTitle: {
    size: '15px',
    weight: 600,
    lineHeight: 1.3,
    letterSpacing: trackingFor(15),
    font: AppleFont.text,
  },
  utilityHeading: {
    size: '24px',
    weight: 600,
    lineHeight: 1.17,
    letterSpacing: trackingFor(24),
    font: AppleFont.text,
  },
  linkActionHeading: {
    size: '21px',
    weight: 600,
    lineHeight: 1.26,
    letterSpacing: trackingFor(21),
    font: AppleFont.text,
  },
  subhead: {
    size: '19px',
    weight: 600,
    lineHeight: 1.21,
    letterSpacing: trackingFor(19),
    font: AppleFont.text,
  },
  /**
   * 统计数值唯一档位（取代 ApStatCard 里 19/24/32 三档 sizeConfig）。
   * 同一指标在全站只有一种字号，等宽数字保证列对齐。
   */
  statValue: {
    size: '20px',
    weight: 600,
    lineHeight: 1.2,
    letterSpacing: trackingFor(20),
    font: AppleFont.text,
  },
  /**
   * 页面主标题（ApPageContainer title 专用）。
   * 21→24：页头原先与"文档自己的标题"（阅读区里那枚 h1，linkActionHeading 21）同档，
   * 于是层级只剩间距一根轴在扛。抬到 24 之后天阶是 24 页头 → 21 对象标题 → 19 区块 → 17 正文 → 15 卡片标题，
   * 字距仍由 trackingFor 派生（24 那档是 -0.006em，大字号收紧），行高按"越大越紧"从 1.26 收到 1.17。
   */
  pageTitle: {
    size: '24px',
    weight: 600,
    lineHeight: 1.17,
    letterSpacing: trackingFor(24),
    font: AppleFont.text,
  },

  // Body & Content
  bodyPrimary: {
    size: '17px',
    weight: 400,
    lineHeight: 1.47,
    letterSpacing: trackingFor(17),
    font: AppleFont.text,
  },
  bodyEmphasis: {
    size: '17px',
    weight: 600,
    lineHeight: 1.24,
    letterSpacing: trackingFor(17),
    font: AppleFont.text,
  },
  controlLabel: {
    size: '14px',
    weight: 400,
    lineHeight: 1.43,
    letterSpacing: trackingFor(14),
    font: AppleFont.text,
  },

  // Micro & Caption
  microUI: {
    size: '12px',
    weight: 400,
    lineHeight: 1.33,
    letterSpacing: trackingFor(12),
    font: AppleFont.text,
  },
  microUIBold: {
    size: '13px',
    weight: 600,
    lineHeight: 1.38,
    letterSpacing: trackingFor(13),
    font: AppleFont.text,
  },
  // P1-12：全站辅助元信息的最小档。10px 在 1440 下已难读、200% 缩放下更糟，
  // 抬到 11px 作为下限；正文/说明仍应走 microUI(12) 及以上。
  legalMeta: {
    size: '11px',
    weight: 400,
    lineHeight: 1.38,
    letterSpacing: trackingFor(11),
    font: AppleFont.text,
  },

  /**
   * 13px 常规档：紧凑行文本（时间线次级信息、密集列表）。
   * 这是本档的唯一名字；原先旁边还挂着 7 个 @deprecated 同义键
   * （sectionHeading/cardTitle/subHeading/body/button/link/captionBold），已删——
   * 它们与 bodyPrimary/bodyEmphasis/microUIBold 各差 0.1 的 lineHeight，
   * 留着就是"同一个字号有两种写法"，改一档不会传导到另一档。
   * 要粗体就写 weight 覆盖，别再开一个新键名。
   */
  caption: {
    size: '13px',
    weight: 400,
    lineHeight: 1.38,
    letterSpacing: trackingFor(13),
    font: AppleFont.text,
  },

  codeInline: {
    size: '85%',
    weight: 400,
    lineHeight: 1.6,
    letterSpacing: 'normal',
    font: AppleFont.mono,
  },
  codeBlock: {
    size: '13px',
    weight: 400,
    lineHeight: 1.6,
    letterSpacing: trackingFor(13),
    font: AppleFont.mono,
  },
} as const;

// ==================== 字号：只有 AppleType 一张表 ====================
// 这里原先挂着一张独立的 `AppleFontSize` 裸字号表（14 个键），注释自称"仅用于 SVG 内文字、
// 图标数字"，但实测 28 处消费**全是 DOM `:style`，没有一处是 SVG**。它造成三件实事：
//   1. 同名异值：`AppleFontSize.subhead` = 15px，而 `AppleType.subhead` = 19px（区块标题那一档）。
//      写的人以为自己在用区块标题档，实际拿到 15px，而 check:ui 按页量的是 19 那一档。
//   2. 天阶外字号：micro 11 / reading 16 / sub 18 —— 这三个值在 AppleType 的阶梯上根本不存在。
//   3. 注释警告的那句"避免 micro(11) / microUI(12) / caption(13) 三档小字互相矛盾"，
//      矛盾正是由它自己提供的（它同时定义了 11 和 13）。
// 消费点已全部迁到 AppleType 对应档（nano→legalMeta、micro→microUI、small→controlLabel、
// subhead/reading→sectionTitle、body/sub→bodyPrimary/bodyEmphasis、title→statValue、
// navHeading→pageTitle），字号从此只有一处来源。要加一档就加进 AppleType（带派生字距）。

export const AppleFontWeight = {
  regular: 400,
  medium: 500,
  semibold: 600,
  bold: 700,
} as const;

export const AppleLineHeight = {
  tight: '1.05',
  heading: '1.1',
  display: '1.2',
  title: '1.3',
  snug: '1.25',
  default: '1.4',
  relaxed: '1.47',
  loose: '1.5',
  spacious: '1.6',
  wide: '1.7',
  extraWide: '1.8',
} as const;

// ==================== 间距 - DESIGN.md §5 推荐 8px 基 ====================
export const AppleSpace = {
  '1px': '1px',
  '6px': '6px',
  '7px': '7px',
  '12px': '12px',
  '20px': '20px',
  '40px': '40px',
  '200px': '200px',
  '220px': '220px',
  '260px': '260px',
  '300px': '300px',
  '400px': '400px',
  xxs: '2px',
  xs: '4px',
  sm: '8px',
  md: '16px',
  lg: '24px',
  xl: '32px',
  '2xl': '48px',
  '3xl': '64px',
  '4xl': '80px',
} as const;

// ==================== 圆角 - 严格按照 DESIGN.md §5 ====================
export const AppleRadius = {
  '1px': '1px',
  '2px': '2px',
  '3px': '3px',
  '4px': '4px',
  '5px': '5px',
  '6px': '6px',
  '8px': '8px',
  '9px': '9px',
  '10px': '10px',
  '12px': '12px',
  '13px': '13px',
  '16px': '16px',
  '18px': '18px',
  '22px': '22px',
  '24px': '24px',
  '28px': '28px',
  '36px': '36px',
  '56px': '56px',
  '100px': '100px',
  '980px': '980px',
  micro: '4px',
  small: '8px',
  medium: '12px',
  /** 表单控件/按钮统一圆角（走 CSS 变量，主题可覆写为直角等） */
  control: 'var(--apple-radius-control)',
  large: '16px',
  xl: '20px',
  pill: '980px',
  circle: '50%',
  /** 卡片圆角（走 CSS 变量：克制档 10px，liquid-glass 14px / warm-paper 10px / editorial 0px） */
  card: 'var(--apple-radius-card)',
  /** 面板级小块（表格容器、内嵌区块、标签条）：比卡片更方，避免「所有东西都是同一块圆角玻璃」 */
  panel: 'var(--apple-radius-panel)',
  /** 窗口级浮层（弹窗 / 聚光 / 抽屉）：浮层圆角只此一档，见 cssVariables 的注释 */
  float: 'var(--apple-radius-float)',
} as const;

// ==================== 阴影 - DESIGN.md §6 Depth & Elevation ====================
export const AppleShadow = {
  level1: '0 0 0 1px rgba(0,0,0,0.06)',
  level2: '0 2px 8px rgba(0,0,0,0.1)',
  level3: '0 4px 16px rgba(0,0,0,0.14)',
  none: 'none',
  card: '0 2px 8px rgba(0,0,0,0.06), 0 1px 2px rgba(0,0,0,0.08)',
  cardHover: '0 8px 24px rgba(0,0,0,0.1), 0 2px 8px rgba(0,0,0,0.08)',
  cardHoverRefined: '0 8px 30px rgba(0, 0, 0, 0.1), 0 2px 8px rgba(0, 0, 0, 0.06)',
  subtle: '0 1px 4px rgba(0,0,0,0.06)',
  dropdown: '0 4px 24px rgba(0,0,0,0.12), 0 0 0 1px rgba(0,0,0,0.06)',
  modal: '0 16px 48px rgba(0,0,0,0.16), 0 0 0 1px rgba(0,0,0,0.06)',
  nav: '0 1px 0 rgba(0,0,0,0.08)',
  innerFocus: 'inset 0 0 0 1px rgba(0,0,0,0.06)',
} as const;

// ==================== 过渡 ====================
/**
 * 动效基准时长（毫秒）—— **全站唯一的时长事实源**。
 *
 * 这张表里的每个数都是原先写在各处的实际值（`AppleTiming` 的 11 档 + `AppleTransition`
 * 自己点名的 3 档 + `App.vue` 页过渡的 4 处 + 玻璃 materialize 的 280ms + 打字跳动的 1.4s），
 * 搬进令牌时**一个数字都没改** —— 默认档观感不变是这一批的验收线。
 *
 * 实际时长 = 本表 × `var(--apple-motion-scale)`。倍数由个性化档位给
 * （`styles/theme/personalization.ts`），所以「把动效调慢/调没」不需要再动这一张表里的任何数。
 *
 * 为什么键按"声明处的语义"分而不是按"数值去重"：150 出现两次（fast / 需要单独调的没有）、
 * 200 出现三次、300/350/400 各有两三个同值不同用途的档。合并成同一个键的话，
 * 将来想把"页面离场"调快会连带把 hover 一起改掉 —— 而这两件事的调参理由是独立的。
 */
export const BASE_DURATION_MS = {
  fast: 150,
  interactive: 200,
  modalOut: 200,
  pageLeave: 200,
  default: 250,
  dropdownIn: 250,
  glassArrive: 280,
  smooth: 300,
  cardLift: 300,
  modalIn: 350,
  pageEnterOpacity: 350,
  themeCrossfade: 350,
  bounce: 400,
  pageEnter: 400,
  pageMove: 400,
  gentle: 400,
  spring: 500,
  typingBounce: 1400,
  skeletonSweep: 1500,
  pulseSoft: 1500,
} as const;
export type DurationKey = keyof typeof BASE_DURATION_MS;

/** 时长令牌名：`--apple-dur-<key>`（`<style>` 块与行内 `:style` 都只能走这个名字） */
export const durVar = (key: DurationKey) => `var(--apple-dur-${key})`;

/**
 * 系统「减弱动态效果」与应用内档位的优先级护栏住在一个地方：
 * `styles/theme/personalization.ts` 的 `MOTION_GUARD`（材质层的 `themes.ts` 也要用同一条，
 * 两条写成两处就会出现"系统开了减弱动效、卡片背景还在跟着滚"那种半接线）。
 * `data-motion` 缺席 = 这台设备没表过态 → 尊重系统；显式选了 `full` / `soft` → 由应用档位说话
 * （`soft` 不该被这条通配压成 0，它自己就是"更慢但仍在动"）。
 */

/**
 * 版面人格（批4）的入场编排。**一份 CSS 字符串，由注入层发出**，而不是每个版面各写一套动画：
 * `check:design` 的 G34 判"这套编排只准出现在登记的公开版面"，判据就是这个类名与这批关键帧
 * —— 各版面自己写动画，G34 就永远看不见新增的那一处。
 *
 * 三条规矩都在这一段里体现：
 *  1. **只动 transform 与 opacity**（G17/G27：不许动布局属性，否则每帧重排）；
 *  2. **时长与逐条延迟全部走 `--apple-dur-*`**（G32），所以「柔和 / 减弱」两档真的能调到这里；
 *  3. **起始态只存在于 @keyframes 的 0% 里，不写成静态 `opacity: 0`** ——
 *     `data-persona` 缺席（JS 没跑）或动画被通配压成 0.01ms 时，元素停在 100% 那帧，
 *     也就是"看得见的内容"。写成静态起始态会让这类用户拿到一块永远透明的东西，
 *     而这正是本仓「只会往安全的方向错」那条口径的反面。
 */
function personaCss(): string {
  return `
@keyframes apple-persona-rise {
  from { opacity: 0; transform: translateY(0.6rem); }
  to { opacity: 1; transform: none; }
}
@keyframes apple-brand-rise {
  from { opacity: 0; transform: translateY(0.3rem) scale(0.94); }
  to { opacity: 1; transform: none; }
}
@keyframes apple-node-arrive {
  from { opacity: 0; transform: translateY(0.45rem) scale(0.4); }
  to { opacity: 1; transform: none; }
}

/* 逐条落位：按 DOM 顺序而不是按"第几个看起来该来的元素"，所以顺序即语义顺序
   （登录页是 品牌 → 论点 → 表单 → 辅助链接；分享页是 标题 → 元信息 → 正文块）。
   上限 6 条是刻意的：再多的条目一起动就变成"页面在表演"，读不出层次了。 */
:root[data-persona='signature'] .apple-persona-stack > :nth-child(-n + 6) {
  animation: apple-persona-rise var(--apple-dur-pageEnter) var(--apple-easing-emphasized) both;
}
:root[data-persona='signature'] .apple-persona-stack > :nth-child(2) { animation-delay: var(--apple-dur-pageLeave); }
:root[data-persona='signature'] .apple-persona-stack > :nth-child(3) { animation-delay: calc(var(--apple-dur-pageLeave) * 2); }
:root[data-persona='signature'] .apple-persona-stack > :nth-child(4) { animation-delay: calc(var(--apple-dur-pageLeave) * 3); }
:root[data-persona='signature'] .apple-persona-stack > :nth-child(5) { animation-delay: calc(var(--apple-dur-pageLeave) * 4); }
:root[data-persona='signature'] .apple-persona-stack > :nth-child(6) { animation-delay: calc(var(--apple-dur-pageLeave) * 5); }

/* 品牌标记：书先摊开，顶端那一点智能后到位 —— 这就是这个标记自己的语义顺序。
   只在 .ap-brand-enter 上加了动画，所以顶栏、favicon、分享页脚那些静态品牌位零影响。 */
:root[data-persona='signature'] .ap-brand-enter > svg {
  animation: apple-brand-rise var(--apple-dur-modalIn) var(--apple-easing-emphasized) both;
}
:root[data-persona='signature'] .ap-brand-enter [data-brand-part='node'] {
  transform-origin: 12px 5px;
  animation: apple-node-arrive var(--apple-dur-spring) var(--apple-easing-emphasized) both;
  animation-delay: var(--apple-dur-smooth);
}`;
}

/**
 * 减弱动效那组 `!important` 声明 —— **一份声明，两个入口**：
 * ① 系统的「减弱动态效果」（且这台设备没在应用内表过态，由 `MOTION_GUARD` 把关）；
 * ② 应用内显式选了 `reduced`（系统开关没打开的用户也能主动要这一档）。
 * 两处共用同一段声明，所以将来要加一条（例如 `animation-play-state`）只会有一个地方要改。
 * `animation-iteration-count: 1` 是这条通配里最容易被漏的一项：只压时长的话，
 * `infinite` 的关键帧动画会变成"每帧随机相位"的抖动 —— 那不是减弱动效，那是频闪。
 */
const REDUCE_MOTION_DECLS = `animation-duration: 0.01ms !important;
    animation-iteration-count: 1 !important;
    transition-duration: 0.01ms !important;
    scroll-behavior: auto !important;`;

function reduceMotionCss(): string {
  return `/* 减弱动效：系统信号（未被应用内档位覆盖时）与应用内 reduced 档共用同一组声明 */
@media (prefers-reduced-motion: reduce) {
  ${MOTION_GUARD} { scroll-behavior: auto; }
  ${MOTION_GUARD} *, ${MOTION_GUARD} *::before, ${MOTION_GUARD} *::after {
    ${REDUCE_MOTION_DECLS}
  }
}
:root[data-motion='reduced'] { scroll-behavior: auto; }
:root[data-motion='reduced'] *, :root[data-motion='reduced'] *::before, :root[data-motion='reduced'] *::after {
  ${REDUCE_MOTION_DECLS}
}
/* 页过渡在减弱档里不许做位移（只淡入淡出）。原先这条住在 App.vue 的 @media 里，
   挪到这里是因为应用内选了 reduced 时那条 @media 根本不成立 —— 而"一份声明两个入口"
   只可能有一个地方写。 */
${MOTION_GUARD} .page-enter-from, ${MOTION_GUARD} .page-leave-to,
:root[data-motion='reduced'] .page-enter-from, :root[data-motion='reduced'] .page-leave-to {
  transform: none;
}`;
}

// ==================== 缓动曲线（唯一来源）====================
export const AppleEasing = {
  standard: 'cubic-bezier(0.25, 0.1, 0.25, 1)',
  emphasized: 'cubic-bezier(0.32, 0.72, 0, 1)',
  spring: 'cubic-bezier(0.34, 1.56, 0.64, 1)',
} as const;

/**
 * 时长 + 缓动，**不带属性** —— 只给"自己点名属性"的写法用：
 * `transition: \`transform ${AppleTiming.smooth}\``。
 * 这一档非独立不可：原先有 7 处把带属性的 `AppleTransition.fast`（值是 `all 0.15s …`）
 * 当前缀拼，拼出来是 `opacity all 0.15s …` —— 整条声明非法，浏览器直接丢掉，
 * 那个过渡其实一直没生效（`ApSlider` 的 opacity、`ApSwitch` 的 background 都是）。
 */
export const AppleTiming = {
  fast: `${durVar('fast')} ${AppleEasing.standard}`,
  default: `${durVar('default')} ${AppleEasing.standard}`,
  smooth: `${durVar('smooth')} ${AppleEasing.standard}`,
  bounce: `${durVar('bounce')} ${AppleEasing.spring}`,
  /** Modal 弹入 - Apple HIG 标准弹性缓动 */
  modalIn: `${durVar('modalIn')} ${AppleEasing.emphasized}`,
  /** Modal 弹出 */
  modalOut: `${durVar('modalOut')} ${AppleEasing.standard}`,
  /** 下拉展开 */
  dropdownIn: `${durVar('dropdownIn')} ${AppleEasing.emphasized}`,
  /** 页面入场 */
  pageEnter: `${durVar('pageEnter')} ${AppleEasing.standard}`,
  /** 骨架屏微光扫过：时长与缓动只这一处（ApSkeleton 与文档侧栏共用同一条）*/
  skeletonSweep: `${durVar('skeletonSweep')} ease-in-out`,
  spring: `${durVar('spring')} ${AppleEasing.emphasized}`,
  gentle: `${durVar('gentle')} ${AppleEasing.standard}`,
} as const;

/**
 * 弹簧参数（手势层用，见 `composables/ui/useSpring.ts`）。
 * `response` 是"多快到位"（秒），`damping` 是回弹比例（1 = 临界阻尼、不弹）。
 * 可拖走浮层的落位一律临界阻尼（damping 1.0）：甩动"像被甩出去"的手感来自
 * `useSheetGesture` 的**速度接力 + 动量投影**（§5/§6），不是欠阻尼回弹——
 * dismiss 的过冲发生在屏幕外，看不见，欠阻尼只会让安定发飘。
 */
export const AppleSpring = {
  /** 跟手 sheet 的落位：不弹，快速安定 */
  sheet: { response: 0.34, damping: 1.0 },
} as const;

export type SpringSpec = { response: number; damping: number };

/**
 * 状态过渡**只准点名这些属性**（§11：动画要落在合成器上）。
 * 颜色 / 描边 / 阴影 / 透明 / 变换 / 滤镜 / 轮廓 —— 没有一个会触发布局重排。
 * 想动"尺寸与位置"不许加进这张表，改用 transform：进度条 `scaleX`、开关滑块 `translateX`。
 */
const STATE_PROPS = ['transform', 'opacity', 'background', 'border-color', 'color', 'fill', 'box-shadow', 'filter', 'outline-color'] as const;
const qualify = (timing: string) => STATE_PROPS.map((p) => `${p} ${timing}`).join(', ');

export const AppleTransition = {
  /** 交互组件多属性过渡（按钮/卡片 hover-active 共用） */
  interactive: `transform ${durVar('interactive')} ${AppleEasing.standard}, box-shadow ${durVar('interactive')} ${AppleEasing.standard}, background ${durVar('interactive')} ${AppleEasing.standard}, filter ${durVar('interactive')} ${AppleEasing.standard}`,
  default: qualify(AppleTiming.default),
  fast: qualify(AppleTiming.fast),
  smooth: qualify(AppleTiming.smooth),
  bounce: qualify(AppleTiming.bounce),
  spring: qualify(AppleTiming.spring),
  gentle: qualify(AppleTiming.gentle),
  micro: `transform ${durVar('fast')} ${AppleEasing.standard}`,
  modalIn: qualify(AppleTiming.modalIn),
  modalOut: qualify(AppleTiming.modalOut),
  dropdownIn: qualify(AppleTiming.dropdownIn),
  /** 卡片悬浮提升 */
  cardLift: `transform ${durVar('cardLift')} ${AppleEasing.standard}, box-shadow ${durVar('cardLift')} ${AppleEasing.standard}`,
  /** 页面入场 */
  pageEnter: qualify(AppleTiming.pageEnter),
} as const;

// ==================== 动画关键帧名称 ====================
export const AppleAnimation = {
  pulseSoft: 'apple-pulse-soft',
  shimmer: 'apple-shimmer',
  spin: 'spin',
} as const;

// ==================== 毛玻璃模糊值（唯一来源，禁止就地写 blur）====================
export const AppleGlassBlur = {
  /** 标准玻璃面板：卡片、浮层 */
  standard: 'blur(20px) saturate(180%)',
  /** 导航栏 */
  nav: 'blur(24px) saturate(180%)',
  /** 下拉 / 吸顶表格行等轻浮层 */
  dropdown: 'blur(16px) saturate(180%)',
  /** 遮罩后的重玻璃（模态） */
  heavy: 'blur(40px) saturate(200%)',
  /** 遮罩层自身的轻模糊 */
  mask: 'blur(4px)',
} as const;

// ==================== 毛玻璃效果 ====================
/**
 * 材质层由主题的 --apple-glass-blur / --apple-glass-* 决定：玻璃主题给 blur+saturate，
 * 纸感/杂志主题给 none（不透明纸面）。组件不要再写 AppleGlassBlur 的字面档位，
 * 否则主题换不动材质。
 */
const GLASS_FILTER = 'var(--apple-glass-blur)';
/**
 * 玻璃的方向性棱线 / 亮边 / 镜面高光都已上移到材质令牌（`--apple-glass-edge` /
 * `--apple-glass-rim` / `--apple-glass-specular`），这里只留引用。
 * 原因不是"少写几个字"：这些装饰此前是常量，纸感与杂志的浮层也照着画，
 * 而那两套外观本来就没有玻璃；解析到材质轴之后，非玻璃材质的默认值是一条透明 inset。
 */
/**
 * 材质层只负责「模糊 + 边缘」这一件事，底色一律取自表面阶梯（surface-*）。
 * 原先这里写的是 var(--apple-glass-bg) / var(--apple-glass-bg-nav)，那两个名字只是阶梯的
 * 转发别名——同一层颜色有两条命名路径，主题就只能靠门禁拦着不许覆写别名了。
 */
export const AppleGlass = {
  /** 标准毛玻璃（卡片、弹窗） */
  standard: {
    background: 'var(--apple-surface-panel)',
    // 液态玻璃：亮边 + 上缘高光斜面 + 浮起阴影（与 .apple-glass-layer 同源语言）。
    // 三件套都是令牌：非玻璃材质解析成中性发丝边 + 透明棱线，纸面不会长出玻璃高光。
    border: '0.5px solid var(--apple-glass-rim)',
    boxShadow: 'var(--apple-glass-edge), 0 10px 30px rgba(0,0,0,0.18)',
    backdropFilter: GLASS_FILTER,
    WebkitBackdropFilter: GLASS_FILTER,
  },
  /** 导航栏毛玻璃 */
  nav: {
    background: 'var(--apple-surface-chrome)',
    backdropFilter: GLASS_FILTER,
    WebkitBackdropFilter: GLASS_FILTER,
    borderBottom: '0.5px solid var(--apple-glass-rim)',
  },
  /**
   * 控件玻璃档（secondary 按钮、图标钮）：半透板 + 镜面高光 + 发丝边 + 内棱线，
   * **底色与模糊由调用点决定** —— 底色是控件所在那块的浅染/次级面，模糊只在数量可控时付
   * （图标钮 idle 实测同屏 66 层，见 ApButton），非玻璃材质下高光与棱线自动退成 none / 透明。
   * 这一档存在的理由不是省字：此前 secondary 只有 border-light、图标钮却整包用浮层档
   * （float 底色 + dropdown 阴影），同一屏里"控件"有两种语法。
   */
  control: {
    backgroundImage: 'var(--apple-glass-specular)',
    border: '0.5px solid var(--apple-glass-rim)',
    boxShadow: 'var(--apple-glass-edge)',
  },
  /** 下拉/浮层毛玻璃（浮层比面板更实一档，见 menuSurfaceStyle） */
  // 模糊**不在这里**：菜单/下拉的 materialize 由消费元素上的 .apple-glass-layer 伪元素承载
  // （入场时模糊随动画一起到位）。共享样式里带 blur 的话，10 个消费点就没法逐个接入场动画，
  // 而"谁没接"在静态上看不出来 —— 现在由 check:glass 的 MATERIALIZED 表逐个点名。
  // 底色走 float 而不是 elevated：elevated 是「内容滚到顶栏底下」那一档，它必须扛住任意内容；
  // 菜单/弹窗压的是本应用自己的环境，跟着 elevated 一起加厚就是把玻璃过成白板。
  dropdown: {
    // 镜面高光与底色分开声明：背景图是玻璃令牌（非玻璃材质为 none），底色永远走浮层阶梯。
    // 若继续用 `background` 简写，`none` 作为图层会让整条声明不合法 —— 这里拆开是为了让
    // "非玻璃 = 没有高光" 这件事是合法的 CSS，而不是靠组件里再写一个 if。
    backgroundImage: 'var(--apple-glass-specular)',
    backgroundColor: 'var(--apple-surface-float)',
    border: '0.5px solid var(--apple-glass-rim)',
    // 液态玻璃：上缘高光斜面 + 浮起阴影（与 .apple-glass-layer 同源，按钮是缩微版）
    boxShadow: 'var(--apple-glass-edge), 0 2px 8px rgba(0,0,0,0.14)',
  },
  /** 重度毛玻璃（遮罩层） */
  heavy: {
    background: 'var(--apple-surface-panel)',
    border: '0.5px solid var(--apple-glass-rim)',
    boxShadow: 'var(--apple-glass-edge), 0 14px 40px rgba(0,0,0,0.24)',
    backdropFilter: GLASS_FILTER,
    WebkitBackdropFilter: GLASS_FILTER,
  },
} as const;

// ==================== 时间常量 ====================
/**
 * JS 侧的毫秒数。`transition*` 三档是 `BASE_DURATION_MS` 的**镜像**而不是第二份数字
 * —— 此前这里写着 `transitionDefault: 200` 而 `AppleTiming.default` 是 250ms，
 * 两处各说一件事（同一档过渡 CSS 与 JS 认的两个数）。
 * **等动画真的动完的调用方必须再套一层 `scaledMs()`**（`styles/theme/personalization.ts`）：
 * 个性化把 CSS 时长乘过之后，直接用这个数会"JS 先认为动完了、CSS 还在动"。
 * `debounce*` / `autoSaveInterval` / `toastDuration` / `renderCheck*` / `userInfoTimeout`
 * 是**等待与轮询**，不是动效，所以它们不随动效档位缩放。
 */
export const AppleDuration = {
  transitionFast: BASE_DURATION_MS.fast,
  transitionDefault: BASE_DURATION_MS.default,
  transitionSmooth: BASE_DURATION_MS.smooth,
  debounceInput: 150,
  debounceSearch: 400,
  debounceResize: 150,
  autoSaveInterval: 3000,
  toastDuration: 3000,
  animationClose: 50,
  renderCheck: 50,
  renderCheckSlow: 100,
  userInfoTimeout: 10000,
} as const;

// ==================== 卡片网格尺度 ====================
/**
 * 网格只有四档地板，按「格子里装什么」划分，不按页面划分：
 *   tile   纯图形格（媒体缩略图 / 热力图单元 / 附件）
 *   stat   一块数值（标签 + 数字，没有正文）
 *   entity 内容实体卡（空间 / 文档 / 知识库 / 联系人——标题 + 元信息）
 *   panel  并排信息块（图表 / 大段配置）
 * 同一区块里的卡片必须等高：grid-auto-rows:1fr 把每行拉到该行最高卡的高度，
 * 卡片自己是 flex 列（见 ApCard / ApEntityCard），元信息自然压到底部对齐。
 * 页面不要再手写 minmax(<任意数>px) —— 由 check-layout-contract 拦。
 */
export const AppleGridFloor = {
  tile: 'minmax(min(140px, 100%), 1fr)',
  stat: 'minmax(min(200px, 100%), 1fr)',
  entity: 'minmax(min(320px, 100%), 1fr)',
  panel: 'minmax(min(440px, 100%), 1fr)',
} as const;

export type AppleGridFloorName = keyof typeof AppleGridFloor;

/** 内容卡网格：地板 + 间距（默认 16px）+ 等高行 */
export function appleCardGrid(
  floor: AppleGridFloorName = 'entity',
  gap: string = AppleSpace.md,
): Record<string, string> {
  return {
    display: 'grid',
    gridTemplateColumns: `repeat(auto-fill, ${AppleGridFloor[floor]})`,
    gap,
    gridAutoRows: '1fr',
    alignItems: 'stretch',
    // 卡片网格常被 flex 父级拉满剩余高度，此时 1fr 行轨会把余量摊进每一行，
    // 产出「三张卡撑满整屏、卡内容飘在上下两端、中间一片空洞」。
    // 同行等高由 align-items:stretch 负责，行轨本身不该长高，所以必须 start。
    alignContent: 'start',
  };
}

// ==================== 固定列数网格（不是卡片地板）====================
/**
 * 列数由设计定死的网格（两栏表单、统计并排…）：`minmax(0, 1fr)` 让列可以缩到内容宽度以下，
 * 这是它和 `1fr` 的唯一区别 —— `1fr` 的最小尺寸是 min-content，长单词会把整行撑破。
 * 与 `appleCardGrid` 的区别：没有 auto-fill 地板，也没有等高行轨（列数固定，行轨按内容走）。
 */
export const appleFixedColumns = (cols: number): string => `repeat(${cols}, minmax(0, 1fr))`;

// ==================== 布局常量 ====================
/**
 * 壳层横条的唯一高度：顶栏、页脚、侧栏底部用户行三处必须同高。
 * 原先顶栏 60、侧栏用户行 49、页脚 47 —— 三条边线不在一个水平上，看着就是散的。
 * 48px 同时保住 44px 触控最小目标与 32px 头像的呼吸空间。
 */
const SHELL_BAR_HEIGHT = '48px';

export const AppleLayout = {
  shellBarHeight: SHELL_BAR_HEIGHT,
  sidebarWidth: '240px',
  sidebarCollapsed: '64px',
  maxContentWidth: '1200px',
  maxWideContentWidth: '1600px',
  maxSearchWidth: '800px',
  drawerWidth: '400px',
  /** 窄卡片度量（登录卡/侧栏列宽；弹窗宽度请统一用 AppleModalSize） */
  cardNarrow: '440px',
  cardWide: '500px',
  spotlightWidth: '560px',
  spaceSidebarWidth: '280px',
  tocWidth: '200px',
  propertyPanelWidth: '240px',
  touchTargetMin: '44px',

  /** 图标按钮尺寸（唯一标尺，替代借用 inputHeight 的写法） */
  iconButtonSmall: '32px',
  iconButtonMedium: '40px',
  iconButtonLarge: '48px',
  avatarSizeSmall: '32px',
  avatarSizeMedium: '44px',
  avatarSizeLarge: '64px',
  /** 表格度量（尺寸档位唯一标尺） */
  tableRowHeight: '48px',
  tableRowHeightCompact: '40px',
  tableHeaderHeight: '40px',
  tableHeaderHeightCompact: '36px',
  /** 图标盒（统计卡 / 确认弹窗等） */
  /** 描述列表标签列宽（现值偏窄，待设计复核；此处仅修正"用间距 token 当列宽"的语义） */
  descListLabelWidth: '32px',
  iconBoxSmall: '24px',
  iconBoxMedium: '36px',
  iconBoxLarge: '48px',
} as const;

// ==================== 图标尺寸与描边 ====================
/**
 * 图标尺寸的唯一标尺（此前 src 里实测 16/14/18/12/20/32/48/24/40/22/64 十一种并存，
 * 「解释性图标」这一档从来没被登记，于是每个页面自己挑一个数）。
 * 只登记真正有人用的六档：16 是绝对主力（行内、按钮、侧栏项），18 是统计卡，
 * 20 是列表条目导语与页头，24 是分区/模板卡，40 是空态，48 是大号占位。
 * **12/14 没有登记**：它们是密集控件里 16 的复制品居多，逐处收编要看版面，
 * 所以留在门禁的「只准降不准升」基线里（check-icon-consistency I6），不替它们发明档位。
 * 门禁与组件都读这张表，别在别处再抄一份数字（G20 曾因抄了第二份而比这里松）。
 */
export const AppleIconSize = {
  /**
   * 复选勾号、行内极小符号。12 与 14 不是"漏网的随手数"：实测全站 12×45、14×72 处，
   * 集中在 ApCheckbox 的勾与流式任务的状态点 —— 把它们并进 16 会把勾画出格子。
   * 登记进来的理由：I6 要判"这个数有没有档"，而不是"这个数好不好"。
   */
  tick: 12,
  /** 行内微图标：流式任务行、状态点、紧凑统计芯片 */
  inline: 14,
  /** 行内/按钮/侧栏项：也是 lucide stroke-width 默认值 2 当初调校的尺寸 */
  control: 16,
  /** 统计卡指标 */
  stat: 18,
  /** 列表条目导语图标、页头图标 */
  listLead: 20,
  /** 分区头/模板卡 */
  section: 24,
  /**
   * 状态与占位的中图：索引健康三态、媒体格占位、SSO 载入、附件缩略图位。
   * 实测 6 处同族用法都落在 32 —— 给它一档，而不是把它们硬塞进 24（会明显缩小）
   * 或 40（会盖过它旁边的正文）。
   */
  status: 32,
  /**
   * 空态与错误态**唯一一档**。原先是 empty:40 与 hero:48 两档并存，
   * 而 48 那五处（附件空列表、图片网格、目录空态、ApEmpty 非错误分支）语义与 40 完全重叠 ——
   * 同一个"这里没有东西"的意思画两种大小，就是"不统一"本身。
   */
  empty: 40,
} as const;

export type AppleIconSizeTier = keyof typeof AppleIconSize;

/**
 * 图标浅染底（plate）的唯一 alpha。
 * 起因：同一个「图标坐在浅染方块里」的想法，ApStatCard 写 10、
 * page-styles 的 entityIconBoxStyle 写 12 —— 两处各调一次，屏幕上就是两种灰。
 */
export const APPLE_ICON_TINT_ALPHA = 12;

/**
 * 按绘制尺寸反解 lucide 的 stroke-width（单位是 viewBox 0 0 24 24，不是像素）。
 * lucide 的描边随尺寸等比缩放：stroke-width: 2 画 16px 得到 1.33px 实描、
 * 画 48px 得到 4px —— 设计师眼里这就是两套图标族。
 * 所以尺寸越大越要在 viewBox 单位里写细，让落到屏幕上的那条线基本不变。
 * 表里的数按实测档位给（40 档 1.45 ≈ 2.4px 实描，与 32 档的 1.6 ≈ 2.1px、24 档的 1.75 ≈ 1.75px 同向收敛）；
 * 最大档以上按比例继续减，但地板 1.1 —— 再细就不足一像素、渲染出来发虚。
 */
const ICON_STROKE_STEPS: ReadonlyArray<{ readonly max: number; readonly stroke: number }> = [
  { max: AppleIconSize.stat, stroke: 2 },
  { max: AppleIconSize.section, stroke: 1.75 },
  { max: AppleIconSize.status, stroke: 1.6 },
  { max: AppleIconSize.empty, stroke: 1.45 },
];
export function iconStrokeFor(size: number): number {
  const step = ICON_STROKE_STEPS.find((s) => size <= s.max);
  return step ? step.stroke : Math.max(1.1, Math.round(((AppleIconSize.empty * 1.45) / size) * 100) / 100);
}

// ==================== z-index 层级 ====================
export const AppleZIndex = {
  base: 1,
  /** 顶部条幅 / 阅读进度指示：高于吸顶层，低于下拉与浮层 */
  banner: 500,
  raised: 10,
  sticky: 49,
  overlay: 100,
  dropdown: 1000,
  drawer: 9998,
  modal: 9999,
  tooltip: 10000,
  toast: 10001,
} as const;

/**
 * 浮层菜单的唯一表面：底色 float、发丝边随材质、圆角 card、下拉阴影 + 玻璃边缘厚度、z=dropdown。
 * 收口前全站有 10 份各写一套（elevated/panel 两种底、border-light/glass-border-nav/border 三种边、
 * card/medium/control/small 四种圆角、z 有 dropdown/toast/raised+10 三档）。
 * 其中一条是真缺陷而不只是不齐：dark 下 `--apple-border-light` 与 `--apple-surface-elevated`
 * 同为 #2c2c2e，用 border-light 描边的 4 个菜单在暗色里**没有可见边缘**，看起来像飘在背景上的一块色。
 * 需要偏离默认（如右键菜单要压过 toast）时用 `extra` 显式覆盖并写明理由，不要另起一份表面。
 */
export const menuSurfaceStyle = <T extends Record<string, string | number | undefined>>(extra?: T) => ({
  ...AppleGlass.dropdown,
  borderRadius: AppleRadius.card,
  boxShadow: cv('shadow-dropdown'),
  zIndex: String(AppleZIndex.dropdown),
  ...(extra || {}),
});

/**
 * 主对象表面：一页只有一个「用户是来看它的东西」（文档正文、分享正文）。
 * 底色与 elevation **成对给出**（§12 越大的面越厚）——静置卡片不投影是写死的教条，
 * 这一档是它唯一的例外，所以谁能用它由 `check:layout` 的 R16 白名单管着，
 * 否则它会退化成"第二个默认卡片底"，层级又回到靠阴影喊。
 */
export const primarySurfaceStyle = <T extends Record<string, string | number | undefined>>(extra?: T) => ({
  background: cv('surface-primary'),
  borderRadius: AppleRadius.card,
  boxShadow: cv('shadow-primary'),
  ...(extra || {}),
});

// ==================== 响应式断点 - 严格按照 DESIGN.md §8 ====================// ==================== 状态徽章尺寸（ApStatusBadge 专用）====================
/**
 * 徽章只有高度与圆点尺寸是自己的事；字号与内距**不在这里定义** ——
 * `ApStatusBadge` 一直直接读 `AppleType.legalMeta/microUI/controlLabel` 与 `AppleSpace.*`，
 * 这两项原先是抄在表里的第二份值（数值恰好一致，但改表不会有任何效果，属于漂移隐患）。
 */
export const AppleStatusBadgeSize = {
  small: { height: '20px', dot: '6px' },
  medium: { height: '24px', dot: '8px' },
  large: { height: '32px', dot: '10px' },
} as const;

// ==================== 开关尺寸（ApSwitch 专用）====================
export const AppleSwitchSize = {
  trackWidth: '46px',
  trackHeight: '26px',
  thumbSize: '22px',
  thumbTravel: '22px',
  thumbGap: '2px',
} as const;

// ==================== 分段控件尺寸（ApSegmentedControl 专用）====================
// 分段控件的高度并入同一条控件阶梯（small 32 / medium 40），
// 否则它和并排的搜索框、按钮各矮一截，工具栏基线对不上。
export const AppleSegmentSize = {
  small: { padding: '0 12px', minWidth: '60px', height: '32px' },
  medium: { padding: '0 16px', minWidth: '80px', height: '40px' },
} as const;

export const AppleBreakpoint = {
  smallMobile: 0,
  mobile: 375,
  tablet: 641,
  tabletWide: 834,
  desktop: 1024,
  desktopWide: 1241,
  largeDesktop: 1441,
} as const;

// ==================== 按钮尺寸 ====================
// 字号一律引 AppleType 的档：这张表原先各档自己写死一个 px，是继 AppleFontSize 之后
// 又一张独立的字号表（`large` 那档的 16px 在文字天阶上根本没有对应档，属于天阶外字号）。
export const AppleButtonSize = {
  mini: { height: '24px', padding: '0 8px', fontSize: AppleType.microUI.size },
  small: { height: '32px', padding: '0 12px', fontSize: AppleType.caption.size },
  medium: { height: '40px', padding: '0 16px', fontSize: AppleType.controlLabel.size },
  // 16px → bodyPrimary(17)：全站仅 4 处 large 按钮，把最后一档天阶外字号并回 ladder
  large: { height: '44px', padding: '0 20px', fontSize: AppleType.bodyPrimary.size },
} as const;

// ==================== 输入框尺寸 —— 表单控件唯一高度标尺 ====================
// 与 AppleButtonSize 共用同一条阶梯（small 32 / medium 40 / large 44），
// 否则按钮与输入框并排时基线必然错开（历史上 medium 是 40 vs 38）。
export const AppleInputSize = {
  small: { height: '32px', padding: '0 8px', fontSize: AppleType.caption.size },
  medium: { height: '40px', padding: '0 10px', fontSize: AppleType.controlLabel.size },
  large: { height: '44px', padding: '0 12px', fontSize: AppleType.controlLabel.size },
} as const;

// ==================== 弹窗尺寸（对话框宽度唯一标尺）====================
export const AppleModalSize = {
  small: '400px',
  compact: '480px',
  regular: '520px',
  medium: '600px',
  large: '800px',
  xlarge: '960px',
} as const;

// ==================== 尺寸与密度类型 ====================
/** 控件尺寸轴（输入 / 下拉 / 文本域） */
export type AppleControlSize = 'small' | 'medium' | 'large';
/** 按钮比输入类多一档 mini */
export type AppleButtonSizeName = 'mini' | AppleControlSize;
/** 卡片比输入类多一档 xlarge */
export type AppleCardSizeName = AppleControlSize | 'xlarge';
/** 密度轴（表格 / 弹窗 / 确认框）：与尺寸轴正交，不要用 size 表达紧凑度 */
export type AppleDensity = 'default' | 'compact';

// ==================== CSS 变量映射 ====================
/**
 * 动效时长令牌（由 `BASE_DURATION_MS` 逐档派生，一张表两个用途：JS 拼字符串、CSS 乘倍数）。
 *
 * `--apple-motion-scale` 的默认值写死 `1`：**JS 没跑时不许有任何缩放** ——
 * 档位是个性化选择，回落值必须是"设计时长原样"，否则首屏量到的和设置页说的不是一回事。
 * 实际值由 `applyPersonalizationAttrs()` 以行内覆写的形式给（覆写令牌本身，
 * 不在 `calc()` 里嵌一层再让人后代覆盖 —— 那条 CSS 事实见 theme-system.md）。
 *
 * 不进 `THEME_CONTRACT_KEYS`：时长不是材质，四套外观与两种材质都该用同一档倍数
 * （与 `--apple-easing-*` / `--apple-spinner-duration` 同一个理由）。
 *
 * 令牌名用 **camelCase 尾**（`--apple-dur-modalOut`）而不是本文件其它令牌的 kebab：
 * 它们必须由 `durVar(key)` 从 `BASE_DURATION_MS` 的键**机械派生**，
 * 一旦再写一份"键 → kebab 名"的映射表，就会有两处名字要同步 ——
 * 而"两处各抄一份"正是这个仓库反复付学费的那件事。
 */
export const durationVariables = {
  '--apple-motion-scale': '1',
  '--apple-dur-fast': `calc(${BASE_DURATION_MS.fast}ms * var(--apple-motion-scale))`,
  '--apple-dur-interactive': `calc(${BASE_DURATION_MS.interactive}ms * var(--apple-motion-scale))`,
  '--apple-dur-modalOut': `calc(${BASE_DURATION_MS.modalOut}ms * var(--apple-motion-scale))`,
  '--apple-dur-pageLeave': `calc(${BASE_DURATION_MS.pageLeave}ms * var(--apple-motion-scale))`,
  '--apple-dur-default': `calc(${BASE_DURATION_MS.default}ms * var(--apple-motion-scale))`,
  '--apple-dur-dropdownIn': `calc(${BASE_DURATION_MS.dropdownIn}ms * var(--apple-motion-scale))`,
  '--apple-dur-glassArrive': `calc(${BASE_DURATION_MS.glassArrive}ms * var(--apple-motion-scale))`,
  '--apple-dur-smooth': `calc(${BASE_DURATION_MS.smooth}ms * var(--apple-motion-scale))`,
  '--apple-dur-cardLift': `calc(${BASE_DURATION_MS.cardLift}ms * var(--apple-motion-scale))`,
  '--apple-dur-modalIn': `calc(${BASE_DURATION_MS.modalIn}ms * var(--apple-motion-scale))`,
  '--apple-dur-pageEnterOpacity': `calc(${BASE_DURATION_MS.pageEnterOpacity}ms * var(--apple-motion-scale))`,
  '--apple-dur-themeCrossfade': `calc(${BASE_DURATION_MS.themeCrossfade}ms * var(--apple-motion-scale))`,
  '--apple-dur-bounce': `calc(${BASE_DURATION_MS.bounce}ms * var(--apple-motion-scale))`,
  '--apple-dur-pageEnter': `calc(${BASE_DURATION_MS.pageEnter}ms * var(--apple-motion-scale))`,
  '--apple-dur-pageMove': `calc(${BASE_DURATION_MS.pageMove}ms * var(--apple-motion-scale))`,
  '--apple-dur-gentle': `calc(${BASE_DURATION_MS.gentle}ms * var(--apple-motion-scale))`,
  '--apple-dur-spring': `calc(${BASE_DURATION_MS.spring}ms * var(--apple-motion-scale))`,
  '--apple-dur-typingBounce': `calc(${BASE_DURATION_MS.typingBounce}ms * var(--apple-motion-scale))`,
  '--apple-dur-skeletonSweep': `calc(${BASE_DURATION_MS.skeletonSweep}ms * var(--apple-motion-scale))`,
  '--apple-dur-pulseSoft': `calc(${BASE_DURATION_MS.pulseSoft}ms * var(--apple-motion-scale))`,
} as const;

export const cssVariables = {
  // Primary
  '--apple-absolute-black': AppleColor.absoluteBlack,
  '--apple-pale-apple-gray': AppleColor.paleAppleGray,
  '--apple-near-black-ink': AppleColor.nearBlackInk,

  // Accent
  '--apple-apple-action-blue': AppleColor.appleActionBlue,
  '--apple-body-link-blue': AppleColor.bodyLinkBlue,
  '--apple-high-luminance-link-blue': AppleColor.highLuminanceLinkBlue,

  // Surface
  '--apple-pure-white-canvas': AppleColor.pureWhiteCanvas,

  // Neutrals
  '--apple-secondary-neutral-gray': AppleColor.secondaryNeutralGray,
  '--apple-soft-border-gray': AppleColor.softBorderGray,
  '--apple-mid-border-gray': AppleColor.midBorderGray,
  '--apple-utility-dark-gray': AppleColor.utilityDarkGray,
  '--apple-text-disabled': AppleNeutralColor.textDisabled,

  // 兼容旧命名
  '--apple-white': AppleColor.white,
  '--apple-light-gray': AppleColor.lightGray,
  '--apple-text-primary': AppleColor.textPrimary,
  '--apple-text-secondary': AppleColor.textSecondary,
  '--apple-text-tertiary': AppleColor.textTertiary,
  '--apple-text-quaternary': AppleColor.textQuaternary,
  // 纯色族：主题只改这些，同色的染色底 / 描边由下方 color-mix 派生自动跟随
  '--apple-blue': AppleColor.appleBlue,
  '--apple-green': AppleColor.appleGreen,
  '--apple-red': AppleColor.appleRed,
  '--apple-orange': AppleColor.appleOrange,
  '--apple-purple': AppleColor.applePurple,
  '--apple-teal': AppleColor.appleTeal,
  '--apple-yellow': AppleColor.appleYellow,
  '--apple-indigo': AppleColor.appleIndigo,
  '--apple-pink': AppleColor.applePink,
  '--apple-border': AppleColor.border,
  '--apple-border-light': AppleColor.borderLight,
  // 凹面 / 悬停族：不属于表面阶梯，是「同一表面上的凹陷与反馈」，主题按色温给值
  '--apple-bg-secondary': AppleColor.bgSecondary,
  '--apple-bg-tertiary': AppleColor.bgTertiary,
  '--apple-bg-hover': AppleColor.bgHover,
  // 表面阶梯（surface ladder）：壳层与内容各有专职角色，一套主题必须把五档都定出来。
  // chrome（顶栏+侧栏，同一个壳层表面）→ canvas（页面底，卡片之下的画布）→ panel（卡片/面板）
  // → elevated（**壳层加厚档**：内容滚到顶栏底下时顶栏换的那一档，以及吸顶表头）
  // → float（**浮层面板档**：菜单 / 下拉 / 弹窗 / 抽屉）。
  // 一致性靠「角色互不相同」而非靠阴影：见 design doctrine「层级靠发丝边框与表面台阶」。
  '--apple-surface-chrome': AppleColor.pureWhiteCanvas,
  '--apple-surface-canvas': AppleColor.paleAppleGray,
  '--apple-surface-panel': AppleColor.pureWhiteCanvas,
  '--apple-surface-elevated': AppleColor.pureWhiteCanvas,
  // elevated 与 float 必须分成两档，因为**它们身后的东西不是一回事**：
  // 顶栏在 scroll-edge 的 under 态下压的是「别人家的任意内容」（极值 = 纯黑），
  // 而菜单/弹窗压的是本应用自己的环境（极值 = 一张偏暗的图）。
  // 合成一档的后果就是本轮要修的缺陷：为了扛住顶栏那个极值，浮层被一起钉在 0.84~0.88 白，
  // 环境色相一点都透不出来，液态玻璃在菜单上退化成带发丝边的白板。
  // 两档各按自己的最坏背景定透明度，判据在 check-contrast.mjs 里分开写。
  '--apple-surface-float': AppleColor.pureWhiteCanvas,
  // 输入控件的凹陷面：必须「压在自己所在的板上仍然看得见」。
  // 不能复用 canvas——液态玻璃主题的 canvas 是 transparent，输入框会变成洞。
  '--apple-surface-field': AppleColor.bgSecondary,
  // 主对象档：一页只有一个「用户是来看它的东西」（文档正文、分享正文）。
  // 它与 surface-panel 的区别是**成对的 elevation**（§12 越大的面越厚）——
  // 浅色下底色同为白，靠投影分开；所以这两键必须一起给值，只加底色等于造一个同层重名。
  '--apple-surface-primary': AppleColor.pureWhiteCanvas,
  '--apple-shadow-primary': '0 1px 2px rgba(0,0,0,0.04), 0 8px 24px rgba(0,0,0,0.06)',
  // 表面阶梯就是唯一命名路径。原先这里另有四条转发别名
  // （bg-primary→canvas、bg-card→panel、bg-elevated→elevated、glass-bg/glass-bg-nav），
  // 后果不止是"一个东西两个名字"：别名在 :root 写死成 var(...)，主题覆写 surface-* 时
  // 别名跟着变，但谁要是覆写了别名就把阶梯反向遮蔽（门禁 T12/G10 全在为这件事服务）。
  // 别名已删，用不到 cv('bg-card') 这个名字本身就是保护：AppleCssVarName 类型直接拦下。
  // 开关滑块：必须是「不透明抬起面」，不能复用 surface-panel（玻璃主题下会变半透明）
  '--apple-surface-knob': AppleColor.pureWhiteCanvas,
  // 压在强调色 / 图片遮罩上的文字与图标：恒定浅色，不随主题反色（pure-white-canvas 在暗色里是黑，会糊）
  '--apple-on-accent': AppleColor.pureWhiteCanvas,
  // 语义状态色 = 纯色族别名，主题改一处即可全族跟随
  '--apple-success': 'var(--apple-green)',
  '--apple-warning': 'var(--apple-orange)',
  '--apple-error': 'var(--apple-red)',
  '--apple-text-80': AppleColor.black80,
  '--apple-text-48': AppleColor.black48,
  '--apple-overlay': AppleColor.overlay,

  // ---- 派生染色族：一律 color-mix(纯色, transparent)，主题不写第二份值 ----
  '--apple-blue-light': 'color-mix(in srgb, var(--apple-blue) 12%, transparent)',
  '--apple-green-light': 'color-mix(in srgb, var(--apple-green) 12%, transparent)',
  '--apple-red-light': 'color-mix(in srgb, var(--apple-red) 12%, transparent)',
  '--apple-orange-light': 'color-mix(in srgb, var(--apple-orange) 12%, transparent)',
  '--apple-purple-light': 'color-mix(in srgb, var(--apple-purple) 12%, transparent)',

  '--apple-gray': AppleColor.gray,
  '--apple-gray-50': AppleColor.gray50,

  // 代码语法高亮
  '--apple-code-bg': AppleColor.codeBg,
  '--apple-code-fg': AppleColor.codeFg,
  '--apple-code-green': AppleColor.codeGreen,
  '--apple-code-blue': AppleColor.codeBlue,
  '--apple-code-purple': AppleColor.codePurple,
  '--apple-code-orange': AppleColor.codeOrange,

  // 半透明白色
  '--apple-white-alpha-15': AppleColor.whiteAlpha15,
  '--apple-white-alpha-20': AppleColor.whiteAlpha20,
  '--apple-white-alpha-30': AppleColor.whiteAlpha30,
  '--apple-white-alpha-70': AppleColor.whiteAlpha70,
  '--apple-white-alpha-80': AppleColor.whiteAlpha80,

  // 滚动条
  '--apple-scrollbar-thumb': AppleColor.scrollbarThumb,
  '--apple-scrollbar-thumb-hover': AppleColor.scrollbarThumbHover,

  // 毛玻璃材质（登录卡 / 遮罩层）：材质 = 阶梯角色 + 模糊 + 这条边缘。
  // 曾经另有 --apple-glass-bg / --apple-glass-bg-nav 两条转发别名，与上面的表面别名同病，已删。
  '--apple-glass-border-nav': 'rgba(0,0,0,0.12)',
  /**
   * 玻璃语言的**默认值 = 不是玻璃**。四件套只在 GLASS_OVERLAY 里覆写成真正的玻璃：
   *   · edge：方向性棱线（顶亮/侧弱白/底暗），默认是一条完全透明的 inset；
   *   · rim：玻璃亮边，默认退成实心外观自己的发丝边；
   *   · specular：面上的薄斜面 + 弱面光，默认 none；
   *   · rim-specular：贴边的、随光源沿周长滑动的那圈高光，默认 none。
   * 为什么做成令牌：`AppleGlass.*` 与 `.apple-glass-layer::before` 是**共享**的表面语言，
   * 组件不该在每次调用时判断材质；把四件套解析到材质轴上，纸感/杂志（data-material=solid）
   * 拿到的就是"没有玻璃装饰"的那一档，而浅/深 glass 拿到的仍是原来那套。
   */
  '--apple-glass-edge': 'inset 0 0 0 0 rgba(0,0,0,0)',
  // 非玻璃材质没有"玻璃亮边"，回退是主题自己的中性发丝线。
  // 写成字面值而不是 var(--apple-border-light)：那是同一个概念的第二条命名路径，
  // T12 会（正确地）把它当成转发别名拦下；深色外观在 DARK_THEME 里另给一档。
  '--apple-glass-rim': 'rgba(0,0,0,0.08)',
  '--apple-glass-specular': 'none',
  '--apple-glass-rim-specular': 'none',
  // 停靠壳层朝向内容那一条边上的滑动亮段（悬停面用整圈 rim-specular，不用这一枚）
  '--apple-glass-rim-line': 'transparent',
  // 同一枚亮段在 scroll-edge='under' 时的暗档。内容压上来之后壳面被身后的深色拉暗，
  // 固定的亮段在近黑壳上会变成一条"画上去的线"（暗色档实拍 +37~+75 的单像素台阶）。
  // 暗档的默认值仍然不是玻璃：solid / 纸感 / 杂志拿到 transparent。
  '--apple-glass-rim-line-dim': 'transparent',
  // 浮起阴影：壳层是压在内容上的一层材料，没有投影就只剩一块变色区域
  '--apple-glass-nav-shadow': 'none',
  // 侧栏右边那条侧向边界的软影（顶栏用上面那条 nav-shadow）。
  // 默认写成一条零尺寸投影而不是 none：侧栏的 boxShadow 是两枚令牌拼成的列表，
  // 列表里出现 none 会让整条声明失效（连棱线一起丢）。
  '--apple-glass-side-shadow': '0 0 0 0 rgba(0,0,0,0)',
  // 底栏朝上的外投影（它是停靠视口下沿的壳层横边，分离方向与顶栏相反）
  '--apple-glass-foot-shadow': 'none',

  // 字体
  '--apple-font-display': FONT_DISPLAY_BASE,
  '--apple-font-text': FONT_TEXT_BASE,
  '--apple-font-mono': AppleFont.mono,

  // 滑块尺寸（scoped CSS 只能引用 CSS 变量，故注册为变量而非 TS 令牌）
  '--apple-slider-track-height': '4px',
  '--apple-slider-thumb-size': '16px',
  '--apple-slider-thumb-border': '2px',

  // 字号（全部指向 AppleType 的档，`<style>` 块读不到 JS 常量才需要这几枚变量）
  '--apple-font-size-nano': AppleType.legalMeta.size,
  '--apple-font-size-micro': AppleType.microUI.size,
  '--apple-font-size-caption': AppleType.caption.size,
  '--apple-font-size-small': AppleType.controlLabel.size,
  '--apple-font-size-body': AppleType.bodyPrimary.size,
  '--apple-font-size-title': AppleType.statValue.size,

  // 间距
  '--apple-space-2px': AppleSpace.xxs,
  '--apple-space-6px': '6px',
  '--apple-space-8px': AppleSpace.sm,
  '--apple-space-10px': '10px',
  '--apple-space-12px': '12px',
  '--apple-space-14px': '14px',
  '--apple-space-16px': AppleSpace.md,
  '--apple-space-24px': AppleSpace.lg,

  // 圆角
  '--apple-radius-micro': AppleRadius.micro,
  '--apple-radius-small': AppleRadius.small,
  '--apple-radius-medium': AppleRadius.medium,
  // 主题化圆角（AppleRadius.card / .control / .panel 引用；扩展主题按需覆写）
  // 克制档：卡片 10px、控件 8px、面板级小块 4px——大圆角 + 同一套阴影是「AI 仪表盘」观感的主要来源
  '--apple-radius-card': '10px',
  '--apple-radius-control': '8px',
  '--apple-radius-panel': '4px',
  // 窗口级浮层一档（弹窗 / 聚光面板 / 抽屉）：圆角只准两档 —— 卡片与菜单同用 card，
  // 盖住整页的窗口级浮层用这一档。此前是 14 / 20 / 22 三档混用，圆角不再编码层级。
  // 各外观按自己的克制程度覆写（玻璃 20 / 纸 10 / 编辑部 0），默认比卡片高一档。
  '--apple-radius-float': '14px',
  // 材质层：浮层模糊（导航 / 弹层 / 下拉）。light 的五档表面全是实心 hex，挂在这里的 blur
  // 每帧快照一次背景、再被自己不透明的底盖掉 —— 一个像素都看不见，纯付合成成本。
  // 只有表面真的半透明的那套（液态玻璃）才覆写回 blur+saturate；纸感/编辑部同为 none。
  '--apple-glass-blur': 'none',
  // 面板模糊（卡片 / 统计块）：默认 none——非玻璃主题不该为每张卡片付一次 backdrop-filter，
  // 也不会「所有东西都是磨砂玻璃」；只有材质主题覆写
  '--apple-blur-panel': 'none',
  // 镜面高光的光源位置（视口百分比）：默认左上，与旧的静态 135° 渐变观感一致。
  // 由 useSpecularLight 跟随指针 / 设备倾斜实时改写 —— 这就是「陀螺仪技巧」的网页等价物，
  // 也是截图/录屏看起来扁平而真机不扁平的原因（截图丢掉了光源在动这件事）。
  '--apple-light-x': '18%',
  '--apple-light-y': '0%',

  // 缓动曲线：值只准住在上面的 `AppleEasing`，这里只是把它暴露给 `<style>` 块
  // （SFC 的 style 段读不到 JS 常量，之前 34 处因此各自手抄同一条曲线）。
  // 不进 THEME_CONTRACT_KEYS：曲线不是材质，五套主题该用同一条。
  '--apple-easing-standard': AppleEasing.standard,
  '--apple-easing-emphasized': AppleEasing.emphasized,
  '--apple-easing-spring': AppleEasing.spring,

  // 时长档 + 动效倍数：曲线住上面那张表，时长住 `BASE_DURATION_MS`，这里只做暴露
  ...durationVariables,

  // spinner 时长：值只准住在 `AppleInteractionState.spinnerDuration`。
  // 同一条曲线/时长的老故事：SFC 的 `<style>` 段与模板里的 `:style` 都读不到 JS 常量，
  // 于是 6 处各自抄了 `spin 0.8s linear infinite`（改一处要动 6 个文件）。
  // 不进 THEME_CONTRACT_KEYS：时长不是材质，四套外观与两种材质都该共用同一个转法。
  //
  // **这一档刻意不乘 --apple-motion-scale**：spinner 是"还在转 = 还没完"的进度指示，
  // 把转速调慢会被读成"卡住了"，调快则什么都读不出来。减弱动效下它由上面那组通配
  // 把 iteration-count 压成 1（转一下停住），而不是跟着 soft 档一起变慢。
  '--apple-spinner-duration': AppleInteractionState.spinnerDuration,

  // 阴影
  '--apple-shadow-level1': AppleShadow.level1,
  '--apple-shadow-level2': AppleShadow.level2,
  '--apple-shadow-level3': AppleShadow.level3,
  // 静置卡片不给投影：层级由 1px 发丝边框承担，浮起（hover / 弹层）才用阴影。
  // 「所有面板同一套 圆角+阴影+边框」正是通用 AI 界面的来源，各主题在此分化。
  '--apple-shadow-card': 'none',
  '--apple-shadow-card-hover': AppleShadow.cardHover,
  '--apple-shadow-card-hover-refined': AppleShadow.cardHoverRefined,
  '--apple-shadow-modal': AppleShadow.modal,
  '--apple-shadow-dropdown': AppleShadow.dropdown,
  '--apple-shadow-subtle': AppleShadow.subtle,

  '--apple-shadow-nav': AppleShadow.nav,
  '--apple-shadow-inner-focus': AppleShadow.innerFocus,
  '--apple-shadow-thumb': '0 1px 3px rgba(0,0,0,0.2)',
  '--apple-shadow-thumb-hover': '0 2px 6px rgba(0,0,0,0.2)',

  // 选中 / 焦点 / 染色底：全部由纯色族派生，主题只需覆写纯色令牌即可整族跟随
  '--apple-blue-selection': 'color-mix(in srgb, var(--apple-blue) 12%, transparent)',
  '--apple-blue-selection-strong': 'color-mix(in srgb, var(--apple-blue) 16%, transparent)',
  // 压在图片或深色遮罩上的控件，hover/按下那层浅染只能由"恒浅色"的 on-accent 稀释出来。
  // 不能用主题的 bg-hover：light 下它是近白，白图标压上去整块糊成一个白方块（实测截图就是这样）。
  '--apple-veil-on-dark': 'color-mix(in srgb, var(--apple-on-accent) 18%, transparent)',
  '--apple-veil-on-dark-active': 'color-mix(in srgb, var(--apple-on-accent) 30%, transparent)',
  // ghost 按钮悬停薄纱。原先用不透明的 bg-hover（light 下 #f5f5f7）再叠 shadow-level2，
  // 压在粉色玻璃头部上像贴了张白纸还带一圈光晕（2026-10-25 用户报「按钮类背景不美观」）。
  // 由 text-primary 按 alpha 稀释：浅外观是灰纱、深外观是白纱，随主题自动成立，且不再配阴影。
  '--apple-hover-veil': 'color-mix(in srgb, var(--apple-text-primary) 8%, transparent)',
  // 焦点环统一为"一圈 2px"：内圈实心主色保证任意底色下 ≥3:1（WCAG 1.4.11）。
  // 旧值是 1px 实心 + 6px 14% 光晕，和键盘聚焦时 :focus-visible 的 2px outline
  // 是两种粗细两套形状，并排看就不像一套控件（用户判词「焦点显示不和谐」）。
  '--apple-blue-focus-ring': '0 0 0 2px var(--apple-focus-ring-color)',
  '--apple-focus-ring-color': 'color-mix(in srgb, var(--apple-blue) 90%, transparent)',
  '--apple-blue-glow': 'color-mix(in srgb, var(--apple-blue) 40%, transparent)',
  '--apple-pink-selection': 'color-mix(in srgb, var(--apple-pink) 30%, transparent)',
  '--apple-blue-bg-subtle': 'color-mix(in srgb, var(--apple-blue) 14%, transparent)',
  '--apple-green-bg-subtle': 'color-mix(in srgb, var(--apple-green) 14%, transparent)',
  '--apple-purple-bg-subtle': 'color-mix(in srgb, var(--apple-purple) 14%, transparent)',
  '--apple-teal-bg-subtle': 'color-mix(in srgb, var(--apple-teal) 14%, transparent)',
  '--apple-indigo-bg-subtle': 'color-mix(in srgb, var(--apple-indigo) 14%, transparent)',
  '--apple-pink-bg-subtle': 'color-mix(in srgb, var(--apple-pink) 14%, transparent)',
  '--apple-yellow-bg-subtle': 'color-mix(in srgb, var(--apple-yellow) 18%, transparent)',
  '--apple-gray-bg-subtle': 'color-mix(in srgb, var(--apple-gray) 14%, transparent)',
  '--apple-red-bg-subtle': 'color-mix(in srgb, var(--apple-red) 14%, transparent)',
  '--apple-orange-bg-subtle': 'color-mix(in srgb, var(--apple-orange) 14%, transparent)',
  // 可读文本色：纯色直接当文字压在自己的浅染上，浅色主题下只有 3:1 上下；
  // 这里按色相分别配比压暗（黄色最亮所以压得最狠），暗色主题反向提亮。
  '--apple-blue-text': 'color-mix(in srgb, var(--apple-blue) 68%, #000)',
  '--apple-green-text': 'color-mix(in srgb, var(--apple-green) 55%, #000)',
  '--apple-red-text': 'color-mix(in srgb, var(--apple-red) 62%, #000)',
  '--apple-orange-text': 'color-mix(in srgb, var(--apple-orange) 50%, #000)',
  '--apple-yellow-text': 'color-mix(in srgb, var(--apple-yellow) 38%, #000)',
  '--apple-purple-text': 'color-mix(in srgb, var(--apple-purple) 60%, #000)',
  '--apple-teal-text': 'color-mix(in srgb, var(--apple-teal) 52%, #000)',
  '--apple-indigo-text': 'color-mix(in srgb, var(--apple-indigo) 64%, #000)',
  '--apple-pink-text': 'color-mix(in srgb, var(--apple-pink) 58%, #000)',
  '--apple-gray-text': 'color-mix(in srgb, var(--apple-gray) 46%, #000)',
  // 实心色相上的前景占位：真实值由 readableForegroundVars() 在注入时按色值算出
  // （CSS 无法按亮度分支：白字压亮绿只有 2.2:1，压亮蓝却有 4.7:1）
  '--apple-on-blue': '#ffffff',
  '--apple-on-green': '#ffffff',
  '--apple-on-red': '#ffffff',
  '--apple-on-orange': '#ffffff',
  '--apple-on-yellow': '#ffffff',
  '--apple-on-purple': '#ffffff',
  '--apple-on-teal': '#ffffff',
  '--apple-on-indigo': '#ffffff',
  '--apple-on-pink': '#ffffff',
  '--apple-on-gray': '#ffffff',

  '--apple-search-highlight': 'rgba(255,235,0,0.4)',
  '--apple-folder-icon': 'rgb(245, 166, 35)',

  '--apple-callout-note-bg': 'color-mix(in srgb, var(--apple-blue) 10%, transparent)',
  '--apple-callout-note-border': 'color-mix(in srgb, var(--apple-blue) 30%, transparent)',
  '--apple-callout-warning-bg': 'color-mix(in srgb, var(--apple-orange) 10%, transparent)',
  '--apple-callout-warning-border': 'color-mix(in srgb, var(--apple-orange) 30%, transparent)',
  '--apple-callout-tip-bg': 'color-mix(in srgb, var(--apple-green) 10%, transparent)',
  '--apple-callout-tip-border': 'color-mix(in srgb, var(--apple-green) 30%, transparent)',
  '--apple-callout-danger-bg': 'color-mix(in srgb, var(--apple-red) 10%, transparent)',
  '--apple-callout-danger-border': 'color-mix(in srgb, var(--apple-red) 30%, transparent)',
  '--apple-callout-info-bg': 'color-mix(in srgb, var(--apple-gray) 10%, transparent)',
  '--apple-callout-info-border': 'color-mix(in srgb, var(--apple-gray) 30%, transparent)',

  // 过渡
  '--apple-transition-fast': AppleTransition.fast,

  // 布局：壳层横条标尺（顶栏 = 页脚 = 侧栏用户行）。CSS 侧取不到 TS 常量，故同步导出为变量。
  '--apple-shell-bar-height': AppleLayout.shellBarHeight,

  // z-index（<style> 块与注入样式串无法引用 TS 常量，故同步导出为 CSS 变量）
  '--apple-z-index-base': String(AppleZIndex.base),
  '--apple-z-index-raised': String(AppleZIndex.raised),
  '--apple-z-index-sticky': String(AppleZIndex.sticky),
  '--apple-z-index-overlay': String(AppleZIndex.overlay),
  '--apple-z-index-banner': String(AppleZIndex.banner),
  '--apple-z-index-dropdown': String(AppleZIndex.dropdown),
  '--apple-z-index-drawer': String(AppleZIndex.drawer),
  '--apple-z-index-modal': String(AppleZIndex.modal),
  '--apple-z-index-tooltip': String(AppleZIndex.tooltip),
  '--apple-z-index-toast': String(AppleZIndex.toast),

  // 断点
} satisfies Record<string, string>;

/** 前缀剥离：得到可用于 cv()/var() 的变量名字面量联合 */
export type StripApplePrefix<T> = T extends `--apple-${infer N}` ? N : never;
/** 所有已注册 CSS 变量名（不含 --apple- 前缀）；cv() 的合法入参集合 */
export type AppleCssVarName = StripApplePrefix<keyof typeof cssVariables>;

// ==================== 主题注册表 ====================
// 差异表、契约与元数据都在 styles/theme/themes.ts；这里只做转发 + 注入，
// 免得 1700 行的令牌文件再塞四份主题色板。
export {
  BASE_THEME_IDS,
  EXTENDED_THEME_IDS,
  THEME_CONTRACT_KEYS,
  THEME_IDS,
  THEME_REGISTRY,
  MATERIALS,
  MATERIAL_REGISTRY,
  LEGACY_THEME_ALIAS,
  GLASS_OVERLAY,
  isDarkThemeId,
  isThemeId,
  isThemePreference,
  isMaterial,
  supportsGlass,
  supportsGlassClear,
  isGlassActive,
  glassOverlayFor,
  resolveThemeTokens,
  themeOverrideMapFor,
  themeOverrides,
  themeExtraCss,
  glassExtraCss,
  themeSwatch,
} from '../../styles/theme/themes';
export type {
  BaseThemeId,
  ExtendedThemeId,
  ThemeContractKey,
  ThemeId,
  ThemeMeta,
  ThemeOverrideMap,
  ThemePreference,
  Material,
  MaterialMeta,
  GlassOverlayMap,
} from '../../styles/theme/themes';

/**
 * 注入 CSS 变量到 :root
 * 应在 main.ts 中调用一次
 */
export function injectCSSVariables(): void {
  if (typeof document === 'undefined') return;

  const lightVars = Object.entries({ ...cssVariables, ...readableForegroundVars(cssVariables) })
    .map(([key, value]) => `  ${key}: ${value};`)
    .join('\n');

  const lightStyle = document.createElement('style');
  lightStyle.id = 'apple-light-theme';
  lightStyle.textContent = `:root {\n${lightVars}\n}`;
  document.head.appendChild(lightStyle);

  // 把一份 resolved 差异表写成 <style> 块。前景色按该组合自己的纯色重算：CSS 不能按亮度分支，只能在这里派生。
  const emit = (styleId: string, selector: string, resolved: Record<string, string>, extraCss: string) => {
    const vars = Object.entries({ ...resolved, ...readableForegroundVars({ ...cssVariables, ...resolved }) })
      .map(([key, value]) => `  ${key}: ${value};`)
      .join('\n');
    const node = document.createElement('style');
    node.id = styleId;
    node.textContent = `${selector} {\n${vars}\n}${extraCss}`;
    document.head.appendChild(node);
  };

  // 两条正交轴：base（外观）走 [data-theme]，glass（材质）叠 [data-material='glass']。
  // light+solid = :root 默认无差异块；玻璃块选择器多一个属性，specificity 压过同 base 的 solid 块，
  // 且它带的是 resolveThemeTokens(base,'glass') 的**完整**解析结果，不会漏出 solid 的值。
  for (const { id } of THEME_REGISTRY) {
    if (id !== 'light') {
      const solid = resolveThemeTokens(id, 'solid');
      if (Object.keys(solid).length) {
        emit(`apple-${id}-theme`, `:root[data-theme='${id}']`, solid, themeExtraCss[id] ?? '');
      }
    }
    if (supportsGlass(id)) {
      const appearance = isDarkThemeId(id) ? 'dark' : 'light';
      emit(
        `apple-${id}-glass-theme`,
        `:root[data-theme='${id}'][data-material='glass']`,
        resolveThemeTokens(id, 'glass'),
        glassExtraCss(appearance),
      );
    }
  }

  const keyframesStyle = document.createElement('style');
  keyframesStyle.id = 'apple-keyframes';
  keyframesStyle.textContent = `
/* spin 原先住在 styles/global-animations.ts 的第二块 <style> 里 ——
   两处注入块、两套"谁负责全局动画"。并到这一处，那个文件随之删除。 */
@keyframes spin {
  from { transform: rotate(0deg); }
  to { transform: rotate(360deg); }
}
@keyframes apple-pulse-soft {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.7; }
}
@keyframes apple-shimmer {
  0% { background-position: -200% 0; }
  100% { background-position: 200% 0; }
}
/* 尊重系统「减弱动态效果」：内联 transition/animation 也会被下面的 !important 覆盖。
   这一组现在有两个入口（系统信号 / 应用内 reduced 档）与一个 MOTION_GUARD，
   理由与"为什么 infinite 动画必须一起压 iteration-count"都写在 reduceMotionCss() 的注释里。 */
${reduceMotionCss()}
/* 版面人格（批4）的入场编排：只在 :root[data-persona='signature'] 下生效，
   类名与关键帧都由这里单点发出，G34 才有得可判。 */
${personaCss()}
tr.apple-last-row td {
  border-bottom: none;
}

/* ===== Apple 全局基础样式 ===== */
html {
  scroll-behavior: smooth;
  /* 设计字体必须落在 html 上：原先全站只给容器/组件写 font-family，html/body 从没设过，
     于是计算值停在浏览器默认（Windows 上是 Times New Roman）。凡 Teleport 到 body 的层
     ——modal / toast / drawer——只要有一个文字节点靠继承拿字体，就直接渲染成衬体。
     实测：modal 根节点与其 label/p 全是 "Times New Roman"。 */
  font-family: var(--apple-font-text);
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
  text-rendering: optimizeLegibility;
}

/* Apple 风格滚动条 */
::-webkit-scrollbar {
  width: 8px;
  height: 8px;
}
::-webkit-scrollbar-track {
  background: transparent;
}
::-webkit-scrollbar-thumb {
  background: var(--apple-scrollbar-thumb);
  border-radius: 4px;
}
::-webkit-scrollbar-thumb:hover {
  background: var(--apple-scrollbar-thumb-hover);
}
::-webkit-scrollbar-corner {
  background: transparent;
}

/* 顶栏玻璃工具类：底色走壳层档，模糊强度一律取主题令牌，
   纸感/杂志主题把它设成 none 就不会白付一次 GPU。
   （同族的 .apple-glass 已删 —— G19 把"被 .apple-glass-nav 的子串和 Monaco 主题 id
   冒充成有人用"的假绿修掉之后，它现出原形：0 个 class 上下文消费者。） */
.apple-glass-nav {
  background-color: var(--apple-surface-chrome);
  /* 镜面高光与底色分开声明：非玻璃材质下 --apple-glass-specular 是 none，
     用 background 简写写 none 会让整条声明不合法（同 AppleGlass.dropdown 的做法）。 */
  background-image: var(--apple-glass-specular);
  /* 只有底边与内容相接，顶/左/右三边贴着视口 —— 那三边不画边。
     亮边、棱线、浮起阴影都走材质令牌：非玻璃外观拿到的是一条中性发丝边与透明棱线。 */
  border-bottom: 0.5px solid var(--apple-glass-rim);
  box-shadow:
    var(--apple-glass-edge),
    var(--apple-glass-nav-shadow);
  /* blur 不在这条工具类里付：它的费率随"内容有没有压上来"变，写在 ApNavBar 的行内样式。
     2026-11-01 改为两档常驻：静止态付一档轻模糊（blur-panel，10px / clear 8px），
     scroll-edge='under' 时升到全档（glass-blur，22px / 16px）。
     此前只在 under 态付费（2026-10-27 性能收口），省下了静止态的常驻 GPU，
     代价是暗色下顶栏读成一块黑板 —— 玻璃的材料感需要一层模糊垫底，而这层在静止态的
     成本远低于滚动中逐帧重滤波。 */
}
/* 非玻璃材质：玻璃投影也不属于这套外观，退回主题自己的导航边与阴影。
   纸感/杂志此前仍带 0 8px 24px 的黑投影与白色斜面 —— 那是玻璃语言，不是纸的。 */
:root:not([data-material='glass']) .apple-glass-nav {
  border-color: var(--apple-border);
  box-shadow: var(--apple-shadow-nav);
}

/* 折射位移为什么不进 backdrop-filter（2026-10-27 用 Chromium 实测钉死的结论）：
   Chromium 对 backdrop-filter 里的 SVG 引用滤镜（url(#f)）**整条声明不渲染**——
   哪怕滤镜只是一个 no-op 的 feOffset 也会被丢，连写在同一串里的 blur() 一起消失，
   元素退化成「没有模糊的半透板」，内容滚到底下就是一块发白发灰的实板。
   且 @supports 对 url(#f) 判真，CSS 层面探测不到这种"解析过、
   渲染丢弃"的差异；Firefox 虽支持但占比太小，不值得为它留一条主流浏览器必坏的声明。
   位移贴图滤镜（index.html 的 #apple-liquid-glass）保留，等哪天 Chromium 支持了、
   或要给元素自身的 filter（不是 backdrop-filter）做边缘折射时再用。
   详见 docs/engineering/liquid-glass.md。 */

/* 玻璃材质入场（materialize）。Apple 的说法是 "materialize, don't just fade"：
   一块玻璃出现时是**模糊一起到位**，而不是一块已经模糊好的板子淡进来。
   做法是一个只带 backdrop-filter 的伪元素随入场淡入，理由有三：
   1. 不动 --apple-glass-blur 这令牌的形状（它是一整串 blur()+saturate()，拆成数值要改遍主题层）；
   2. 不依赖 @property —— 自定义属性的插值动画 Firefox 到今天还不支持，
      用 transition 动 --apple-glass-blur 会退化成一帧切换，比不做还糟；
   3. 文字仍然画在伪元素之上，所以入场过程中正文不会被一起模糊。
   只在元素创建时跑一次（打开浮层就是它的入场），闭合不需要反向，Apple 也只要求入场到位。
   prefers-reduced-motion 下直接停在到位态。solid / 纸感材质下 --apple-glass-blur 是 none，
   这条规则天然惰性，不需要任何材质判断。 */
.apple-glass-layer {
  position: relative;
}
/* 只保留定位与层序：模糊、棱线、亮边、镜面高光都属于玻璃材质，
   统一放进下面那条 data-material='glass' 规则。此前这些装饰写在这里，
   于是 solid / 纸感 / 杂志的浮层也一直画着白色斜面与 0 10px 30px 的黑投影 ——
   观感上就是"实心主题里还残留液态玻璃"。 */
.apple-glass-layer::before {
  content: '';
  position: absolute;
  inset: 0;
  z-index: -1;
  border-radius: inherit;
  pointer-events: none;
}
/* 液态玻璃视觉语言（2026-10-26 升级，2026-10-27 镜面高光随光源动）：镜面高光 + 方向性棱线
   + 亮边 + 浮起阴影。三件套都从材质令牌取（solid/纸感/杂志解析成透明值与 none），
   所以这条选择器同时是"材质门"：非玻璃材质下连伪元素的背景都不会被绘制。
   全部画在模糊伪元素上、位于半透底色之下，靠底色透出，绝不糊到文字。 */
:root[data-material='glass'] .apple-glass-layer::before {
  background: var(--apple-glass-specular);
  border: 0.5px solid var(--apple-glass-rim);
  box-shadow:
    var(--apple-glass-edge),
    0 10px 30px rgba(0, 0, 0, 0.18);
  backdrop-filter: var(--apple-glass-blur);
  -webkit-backdrop-filter: var(--apple-glass-blur);
  animation: apple-glass-arrive var(--apple-dur-glassArrive) var(--apple-easing-emphasized) both;
}
@keyframes apple-glass-arrive {
  from { opacity: 0; }
  to { opacity: 1; }
}
/* 减弱动效下停在到位态：两个入口 + 一个护栏，与上面那组通配同源（不留第二份判断） */
${MOTION_GUARD} .apple-glass-layer::before,
:root[data-motion='reduced'] .apple-glass-layer::before { animation: none; }

/* 镜面环 —— **悬浮面**（菜单/弹窗/气泡）的高光主体。
   光是"贴着边"的：光源在哪一侧，那一侧的棱就亮，沿周长衰减到 0；指针一动亮段沿边滑，
   这是玻璃的"液态"信号。做法是 1px 环：背景铺满盒子，再用 mask 把 content-box 抠掉，
   只剩贴边那一圈（border-box 全局统一，所以 padding:1px 正好是一圈）。
   为什么不用 box-shadow：inset 阴影只能四边等亮或整边等亮，没法让亮度沿周长变化，
   上一版"满宽白带"就是因为只能用整条渐变来表达高光。
   环在内容之上（z-index:2），但只有 1px 宽且 pointer-events:none，不吃点击也遮不住字。
   槽位：.apple-glass-layer::before 是 materialize 模糊层，所以环必须用 ::after。 */
:root[data-material='glass'] .apple-glass-layer::after {
  content: '';
  position: absolute;
  inset: 0;
  z-index: 2;
  border-radius: inherit;
  padding: 1px;
  background: var(--apple-glass-rim-specular);
  /* mask 只关心 alpha，不关心颜色；用 rgba 而不是 hex 是为了过 G5（渐变里不许写死颜色） */
  -webkit-mask: linear-gradient(rgba(0,0,0,1) 0 0) content-box, linear-gradient(rgba(0,0,0,1) 0 0);
  -webkit-mask-composite: xor;
  mask: linear-gradient(rgba(0,0,0,1) 0 0) content-box, linear-gradient(rgba(0,0,0,1) 0 0);
  mask-composite: exclude;
  pointer-events: none;
}

/* 停靠在视口边缘的壳层**不画整圈环**。顶栏的顶/左/右、侧栏的顶/左/底都与视口重合，
   那三条边没有"棱"可言 —— 在视口边缘画亮线，读起来就是一个描边盒子，不是玻璃。
   只画朝向内容的那一条（顶栏底边 / 侧栏右边），并且让它随光源滑动：
   径向渐变的圆心在 --apple-light-x/y，所以亮段永远落在光源那一侧，远端衰减到 0。
   槽位：顶栏 ::after 已被「滚动边缘渐隐」占用，所以顶栏用 ::before；侧栏用 ::after。 */
:root[data-material='glass'] .apple-glass-nav::before {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  bottom: 0;
  height: 1px;
  z-index: 2;
  background: radial-gradient(64% 1px at var(--apple-light-x) 100%,
    var(--apple-glass-rim-line), transparent 72%);
  pointer-events: none;
}
/* 顶栏进入滚动边缘态（内容已经滚到它底下）时，同一条边换暗档、亮段收窄。
   此时壳面被身后内容拉暗（暗色档实测 82 → 39~55），而边界这一侧的职责也从
   "说明这是玻璃"变成"把两块面分开"；内容正以高频细节贴着它运动，
   固定 0.45 的白会读成描边。换档后剩下的是迎光的一小段棱，亮段宽度 64% → 52%。 */
:root[data-material='glass'] .apple-glass-nav[data-scroll-edge='under']::before {
  background: radial-gradient(52% 1px at var(--apple-light-x) 100%,
    var(--apple-glass-rim-line-dim), transparent 66%);
}
/* 底栏：停靠视口下沿的壳层横边，朝向内容的那条边是**上沿**，所以亮段画在上面
   （顶栏画底边、侧栏画右边，同一条规则族）。2026-11-01 浮层化后它身后也有内容滚过了，
   所以同顶栏一样参与 scroll-edge 降档：内容压上来时棱线换暗档、亮段收窄。 */
:root[data-material='glass'] [data-ap-shell='footer']::before {
  content: '';
  position: absolute;
  left: 0;
  right: 0;
  top: 0;
  height: 1px;
  z-index: 2;
  background: radial-gradient(64% 1px at var(--apple-light-x) 0%,
    var(--apple-glass-rim-line), transparent 72%);
  pointer-events: none;
}
:root[data-material='glass'] [data-ap-shell='footer'][data-scroll-edge='under']::before {
  background: radial-gradient(52% 1px at var(--apple-light-x) 0%,
    var(--apple-glass-rim-line-dim), transparent 66%);
}
:root[data-material='glass'] [data-glass-surface]::after {
  content: '';
  position: absolute;
  top: 0;
  bottom: 0;
  right: 0;
  width: 1px;
  z-index: 2;
  background: radial-gradient(1px 64% at 100% var(--apple-light-y),
    var(--apple-glass-rim-line), transparent 72%);
  pointer-events: none;
}

/* Apple 焦点环。
   这里原先写 outline: none 再用 box-shadow 代替，但可聚焦的卡片（ApCard 是
   role=region + tabindex=0）自己用行内样式设了 boxShadow —— 行内优先级压过本条规则，
   于是 outline 被这行抹掉、shadow 又轮不到，键盘聚焦时什么环都不画。
   焦点环改用 outline：它不跟行内 boxShadow 抢同一个属性。 */
.apple-focus-ring:focus-visible {
  outline: 2px solid var(--apple-focus-ring-color);
  outline-offset: 2px;
  border-radius: inherit;
}

/* Apple 选中态 */
::selection {
  background: var(--apple-blue-selection);
  color: inherit;
}

/* 盒模型全站统一。原先只有 ApButton / ApSearch / ApTextarea 等零星自己声明
   box-sizing: border-box，ApSelect 容器没有 —— 同一个 40px 高度令牌，
   下拉因为多算 1px 上下边框渲染成 42，与并排的按钮差 2px（实测 25 页）。 */
*, *::before, *::after {
  box-sizing: border-box;
}

/* 代码块自己横向滚动。markdown 渲染出来的 <pre> 挂不上 :style，长行会把版面撑破：
   430px 视口下文档页实测有 26 个元素画到视口外（pre>code>span 单行 413px 起于 left 67）。 */
pre {
  max-width: 100%;
  overflow-x: auto;
}

/* 时间线最后一项不再往下拖连接线。组件不能靠调用方传 tail 标志，
   否则每页都要自己算一遍 idx === items.length - 1。 */
.ap-timeline-item:last-child .ap-timeline-line {
  display: none;
}

/* 跳转到主内容：键盘用户进页面第一个 Tab 就该能跳过整条导航与侧栏。
   不占位、不显示，只有拿到焦点才出现在左上角。 */
.apple-skip-link {
  position: absolute;
  left: ${AppleSpace.md};
  top: ${AppleSpace.md};
  z-index: ${AppleZIndex.tooltip};
  padding: ${AppleSpace.xs} ${AppleSpace.md};
  border-radius: ${AppleRadius.control};
  background: var(--apple-surface-elevated);
  color: var(--apple-text-primary);
  font-family: ${AppleType.controlLabel.font};
  font-size: ${AppleType.controlLabel.size};
  font-weight: ${AppleType.controlLabel.weight};
  text-decoration: none;
  /* 藏与现用 transform，不动 top：后者每帧都在重排（§11 只准动合成器属性）。
     这曾是全站源码里最后一条"过渡布局属性"，其余都在早几批并档时改完了。 */
  transform: translateY(-200%);
  transition: transform 0.15s ${AppleEasing.standard};
}
.apple-skip-link:focus {
  transform: none;
}

/* 搜索命中高亮。v-html 渲染的内容挂不上 :style 绑定，所以规则必须在注入层；
   浏览器默认 mark 是纯黄底 + 黑字，暗色主题下就是一块刺眼的黄斑。 */
mark.highlight {
  background: var(--apple-search-highlight);
  color: inherit;
  border-radius: ${AppleRadius.small};
  padding: 0 ${AppleSpace.xxs};
  font-weight: ${AppleFontWeight.medium};
}

/* 焦点可见性全站默认开启。原先只有显式挂了 .apple-focus-ring 的元素有焦点框，
   漏挂的自绘控件（侧栏分组标题、目录条目、原生 date 输入）键盘用户完全看不出
   焦点在哪 —— 巡检按真实 Tab 逐个量，三轮都稳定报同一类问题。 */
:focus-visible {
  outline: 2px solid var(--apple-focus-ring-color);
  outline-offset: 2px;
}

/* 按下即反馈，全站默认开启（和上面 :focus-visible 是同一个道理）。按钮自己有 active 态
   （scale 0.97），运行时真按住一遍才发现漏的是三类"看着能点"的东西：裸链接、侧栏里
   role=button 的行、页脚链接 —— 按住与悬停的计算值一模一样，用户要点下去才知道它接不接这一下。
   iOS 的按压语义是"变暗"不是"缩放"，所以用 opacity：整行、行内文字、图标都不会因缩放错位。 */
a[href]:active,
[role="button"]:active,
/* 分段项以前靠组件自己的 pressedIndex 记这个 0.62，而 pointerup 一旦因为重渲染丢掉，
   那一档就永远停在"看着像被按着"的状态（check:motion 的 M1 报的"按下与悬停完全相同"正是它的残影）。
   交给 :active 之后没有状态要维护，浏览器自己会清。 */
[role="tab"]:active {
  opacity: 0.62;
}

/* 可点击角色统一手型光标。[role=button|tab|menuitem|...] 的 div 是靠 @click
   生效的，浏览器不会给它光标反馈，用户只能靠试错知道能不能点。 */
[role="button"]:not([disabled]),
[role="tab"],
[role="menuitem"],
[role="combobox"],
[role="treeitem"],
[role="option"],
[aria-haspopup]:not([aria-disabled="true"]) {
  cursor: pointer;
}
  `;
  document.head.appendChild(keyframesStyle);
}

// ==================== 便捷函数 ====================

// CSS 变量引用（用于暗黑主题自动切换）
export const cv = (name: AppleCssVarName) => `var(--apple-${name})`;
/**
 * 读取 CSS 变量在**当前主题**下的实际值。
 * 仅用于 canvas / 图表库（ECharts、Chart.js、位图导出）等无法解析 var() 的渲染路径；
 * DOM 样式一律用 cv()。主题切换后必须重新求值，否则取到的是过期颜色。
 */
/**
 * 半透明底纹的唯一出口。
 * 现代浏览器走 color-mix（跟随主题变量），Safari < 16.4 / Chrome < 111 自动降级为已注册的 *-bg-subtle 令牌。
 * @param color 具体色值或 var()/cv() 结果
 * @param percent 不透明度百分比（0-100）
 * @param fallback 降级时使用的 CSS 变量名
 */
export function subtleBg(color: string, percent: number, fallback: AppleCssVarName = 'blue-bg-subtle'): string {
  const ok = typeof CSS !== 'undefined' && typeof CSS.supports === 'function' && CSS.supports('color', 'color-mix(in srgb, red 50%, transparent)');
  return ok ? `color-mix(in srgb, ${color} ${percent}%, transparent)` : cv(fallback);
}

const HUE_NAMES = ['blue', 'green', 'red', 'orange', 'yellow', 'purple', 'teal', 'indigo', 'pink', 'gray'] as const;
function hueOf(color: string): string | null {
  const name = /^var\(--apple-([a-z]+)\)$/.exec((color ?? '').trim())?.[1];
  return name && (HUE_NAMES as readonly string[]).includes(name) ? name : null;
}

/** 实心色相背景上可读的前景色（亮色相不能用白字） */
export function onSolid(color: string): string {
  const hue = hueOf(color);
  return hue ? cv(`on-${hue}` as AppleCssVarName) : cv('on-accent');
}

/** 同色浅染（*-bg-subtle / *-light）上的文字色 */
export function textOnTint(color: string): string {
  const hue = hueOf(color);
  return hue ? cv(`${hue}-text` as AppleCssVarName) : cv('text-primary');
}


// ==================== 可读前景派生（WCAG） ====================
const READABLE_HUES = ['blue', 'green', 'red', 'orange', 'yellow', 'purple', 'teal', 'indigo', 'pink', 'gray'] as const;
type Rgb = [number, number, number];

function toRgb(color: string | undefined): Rgb | null {
  const s = (color ?? '').trim();
  const hex = /^#([0-9a-f]{3,8})$/i.exec(s);
  if (hex) {
    let h = hex[1] ?? '';
    if (h.length === 3) h = h.split('').map((c) => c + c).join('');
    if (h.length < 6) return null;
    h = h.slice(0, 6);
    return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16)];
  }
  const fn = /^rgba?\(([^)]+)\)$/i.exec(s);
  if (fn?.[1]) {
    const a = fn[1].replace(/\//g, ' ').split(/[\s,]+/).filter(Boolean).map(Number);
    const [r, g, b] = a;
    if (r === undefined || g === undefined || b === undefined) return null;
    return [r, g, b];
  }
  return null;
}
const channelLum = (v: number) => { const s = v / 255; return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4; };
const luminance = (c: Rgb) => 0.2126 * channelLum(c[0]) + 0.7152 * channelLum(c[1]) + 0.0722 * channelLum(c[2]);
const contrastRatio = (a: Rgb, b: Rgb) => {
  const l1 = luminance(a);
  const l2 = luminance(b);
  return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
};
const mixRgb = (c: Rgb, target: Rgb, p: number): Rgb =>
  [
    Math.round(c[0] * p + target[0] * (1 - p)),
    Math.round(c[1] * p + target[1] * (1 - p)),
    Math.round(c[2] * p + target[2] * (1 - p)),
  ] as Rgb;
const toHex = (c: Rgb) => '#' + c.map((v) => v.toString(16).padStart(2, '0')).join('');

/** 实心色上的可读前景：白字够用就用白，否则取「最浅但仍 ≥4.5:1」的同色压暗墨 */
export function contrastInkOn(color: string | undefined): string {
  const rgb = toRgb(color);
  if (!rgb) return '#ffffff';
  const TARGET = 4.5;
  const whiteRatio = contrastRatio(rgb, [255, 255, 255]);
  if (whiteRatio >= TARGET) return '#ffffff';
  let best = '#ffffff';
  let bestRatio = whiteRatio;
  // 从浅到深找第一个达标的墨色：越浅越保留色相性格，达标即停
  for (let p = 0.6; p >= 0.08; p -= 0.02) {
    const ink = mixRgb(rgb, [0, 0, 0], p);
    const r = contrastRatio(rgb, ink);
    if (r > bestRatio) { bestRatio = r; best = toHex(ink); }
    if (r >= TARGET) return toHex(ink);
  }
  return best;
}

/** 由生效中的令牌表派生 --apple-on-*（注入与对比度门禁共用同一份算法） */
export function readableForegroundVars(effective: Record<string, string>): Record<string, string> {
  const out: Record<string, string> = {};
  for (const hue of READABLE_HUES) out['--apple-on-' + hue] = contrastInkOn(effective['--apple-' + hue]);
  return out;
}

export function resolvedColor(name: AppleCssVarName): string {
  if (typeof document === 'undefined') return '';
  return getComputedStyle(document.documentElement).getPropertyValue(`--apple-${name}`).trim();
}

/* 已删除的便捷函数（0 消费者，且与既有教条冲突）：
   touchTargetStyle / focusRingStyle / monoFontStyle / appleStyle / cardStyle / sidebarStyle。
   focusRingStyle 用 outline:'none'+boxShadow，正是注入层 .apple-focus-ring:focus-visible
   改成 outline 以修掉的那个 bug（内联 boxShadow 会压过它）；cardStyle 硬编码 22px 圆角，
   与「卡片走 var(--apple-radius-card)=10px、静置不投影」的设计教条相反。
   替代：触控最小尺寸用 AppleLayout.touchTargetMin，焦点环用注入层 :focus-visible，
   等宽字体用 AppleFont.mono，成组排版用 cssType(AppleType.*)，
   卡片用 ApCard，侧栏用 doc-sidebar-styles.ts 的 sidebarStyle。 */

