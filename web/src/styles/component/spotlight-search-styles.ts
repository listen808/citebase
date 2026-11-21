import { AppleFont, AppleLayout, AppleRadius, AppleSpace, AppleTransition, AppleType, cv, AppleZIndex, AppleLineHeight } from '@/utils/core/apple-tokens';
import { getOverlayStyle } from '@/styles/component/overlay';
import { loadingStyle as spotlightLoadingStyle, emptyStateStyle as spotlightEmptyStyle } from '@/styles/layout/page-styles';

export const overlayStyle = {
  // 遮罩只染色、不预磨砂：模糊留给浮层自己做，否则浮层身后已经是糊的，
  // 玻璃唯一的证据（锐利背景被折射）就没了 —— 2026-11-01 普查实测弹窗与聚光都读成实心盒。
  ...getOverlayStyle(AppleZIndex.modal, {
    display: 'flex',
    flexDirection: 'column' as const,
    alignItems: 'center',
    justifyContent: 'flex-start',
    paddingTop: '20vh',
  }),
};

/**
 * 聚光面板 = 浮层家族的一员，语言与 ApModal 完全同源：
 * 底色走 float 档，模糊 / 镜面高光 / 镜面环 / materialize 入场全部由
 * 模板上的 `.apple-glass-layer` 伪元素承载（见 apple-tokens.ts），行内不再各写一份。
 * 此前这里把 135° 白色高光与两枚 inset 棱线写成字面值 —— 那是行内样式，
 * 材质门（只扫令牌层的规则体）扫不到，于是 dark+solid 这种"没有玻璃"的档位
 * 也照样在面板上画一道玻璃斜光（2026-11-01 实测复现）。
 */
export const containerStyle = {
  width: AppleLayout.spotlightWidth,
  maxWidth: '90vw',
  backgroundColor: cv('surface-float'),
  borderRadius: AppleRadius.float,
  border: `0.5px solid ${cv('glass-rim')}`,
  boxShadow: cv('shadow-modal'),
  overflow: 'hidden',
};

export const searchBoxStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  padding: `${AppleSpace.md} ${AppleSpace.md}`,
  borderBottom: `1px solid ${cv('border-light')}`,
};

export const searchIconStyle = {
  color: cv('text-tertiary'),
  flexShrink: '0',
};

export const inputStyle = {
  flex: '1',
  border: 'none',
  background: 'transparent',
  // 原先 font/weight 取 bodyEmphasis（17px/600）而 fontSize 单取一张独立表的 18px，
  // 于是渲染出 18px/600 且完全没有字距——看着像 bodyEmphasis 那一档，其实哪一项都不是。
  // 统一回 bodyEmphasis，并带上它派生出来的字距。
  fontFamily: AppleType.bodyEmphasis.font,
  fontSize: AppleType.bodyEmphasis.size,
  fontWeight: String(AppleType.bodyEmphasis.weight),
  letterSpacing: AppleType.bodyEmphasis.letterSpacing,
  color: cv('text-primary'),
  lineHeight: AppleLineHeight.default,
};

export const loadingIconStyle = {
  display: 'inline-flex',
  color: cv('text-tertiary'),
};

export const resultsStyle = {
  maxHeight: '400px',
  overflowY: 'auto' as const,
};

export { spotlightLoadingStyle as loadingStyle };

export const suggestionHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: `${AppleSpace.md} ${AppleSpace.md} ${AppleSpace.sm}`,
};

export const suggestionTitleStyle = {
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleType.microUI.weight),
  color: cv('text-tertiary'),
  // 手写 0.5px 而 trackingFor(12) 是 0.204px —— 同文件同档的 resultCountStyle 又根本没有字距。
  // 字距只准由字号派生，不然后果是"同一档小字在同一块面板里往两个方向收"。
  letterSpacing: AppleType.microUI.letterSpacing,
};

export const resultCountStyle = {
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  letterSpacing: AppleType.microUI.letterSpacing,
  color: cv('text-tertiary'),
};

export const suggestionListStyle = {
  paddingBottom: AppleSpace.sm,
};

export function suggestionItemStyle(selected: boolean) {
  return {
    display: 'flex',
    alignItems: 'center',
    gap: AppleSpace.sm,
    padding: `${AppleSpace.sm} ${AppleSpace.md}`,
    background: selected ? cv('blue-selection') : 'transparent',
    cursor: 'pointer',
    transition: AppleTransition.fast,
  };
}

export const itemIconStyle = {
  color: cv('text-secondary'),
  flexShrink: '0',
};

export const itemTextStyle = {
  flex: '1',
  // 这一格装的是用户起的名（文档标题 / 最近访问 / 历史词），长度不由前端决定，
  // 而它右边的 recentSpaceStyle 是 flexShrink:0（永不收缩）。flex 项默认 min-width:auto
  // 意为"收缩不到内容宽度以下"，于是长标题 + 空间名 = 必然超出行宽，被容器
  // `overflow:hidden` 直接裁掉——没有省略号、也没有 title，用户看到的是断掉的标题。
  minWidth: '0',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
  fontFamily: AppleType.sectionTitle.font,
  fontSize: AppleType.sectionTitle.size,
  letterSpacing: AppleType.sectionTitle.letterSpacing,
  color: cv('text-primary'),
};

export const itemShortcutStyle = {
  fontFamily: AppleFont.mono,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  background: cv('bg-tertiary'),
  padding: `3px ${AppleSpace.sm}`,
  borderRadius: AppleRadius.small,
  flexShrink: '0',
};

export { spotlightEmptyStyle as emptyStyle };

export const historySectionStyle = {
  borderTop: `1px solid ${cv('border-light')}`,
};

export const historyListStyle = {
  paddingBottom: AppleSpace.sm,
};

export function historyItemStyle(isSelected: boolean) {
  return {
    padding: `${AppleSpace.sm} ${AppleSpace.md}`,
    display: 'flex',
    alignItems: 'center',
    gap: AppleSpace.sm,
    cursor: 'pointer',
    background: isSelected ? cv('blue-selection') : 'transparent',
    borderRadius: AppleRadius['5px'],
    transition: AppleTransition.fast,
  };
}

export const viewAllStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  gap: AppleSpace.sm,
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('blue'),
  cursor: 'pointer',
  borderTop: `1px solid ${cv('border-light')}`,
  transition: AppleTransition.fast,
};

export const hintStyle = {
  marginTop: AppleSpace.md,
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  color: cv('text-tertiary'),
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
};

export const kbdStyle = {
  fontFamily: AppleFont.mono,
  fontSize: AppleType.microUI.size,
  background: cv('white-alpha-20'),
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.small,
  border: `1px solid ${cv('white-alpha-30')}`,
};

export const hintDividerStyle = {
  opacity: '0.5',
};

export const recentSpaceStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  flexShrink: '0',
};
