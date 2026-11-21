/**
 * AI 页面样式 — re-export 入口
 * 主要样式已迁移至 page-styles.ts，此处仅保留 aiSub* 子页面专用样式
 */
export * from '@/styles/layout/page-styles'

// ============================================================
// AI sub-page specific styles (not in page-styles)
// ============================================================
import { cv, AppleFont, AppleRadius, AppleSpace, AppleType, AppleFontWeight, AppleLineHeight, AppleGridFloor } from '@/utils/core/apple-tokens';

// ====== 子页面共享样式（AI Settings 内嵌页面） ======

export const AI_TABLE_WIDTH = {
  xs: '40px',
  sm: '80px',
  md: '120px',
  lg: '160px',
  xl: '200px',
  xxl: '240px',
} as const

export const aiSubSectionStyle = {
  padding: AppleSpace.md,
  backgroundColor: cv('surface-panel'),
  borderRadius: AppleRadius.card,
  border: `1px solid ${cv('border-light')}`,
  marginBottom: AppleSpace.sm,
}

// P2-3：space-between 原本把「图标 + 标题」这一对也拆到两端（左侧只剩图标、标题贴右）。
// 标题用 margin-right:auto 吃掉剩余空间，右侧控件仍能被推到行尾。
export const aiSubSectionHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  gap: AppleSpace.sm,
  marginBottom: AppleSpace.sm,
}

export const aiSubSectionHeaderGapStyle = {
  ...aiSubSectionHeaderStyle,
  gap: AppleSpace.sm,
}

export const aiSubSectionTitleStyle = {
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  // 见 aiSubSectionHeaderStyle：标题永远紧跟左侧图标，不再被 space-between 甩到右端
  marginRight: 'auto',
}

export const aiSubSectionMetaStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
}

export const aiSubCardStyle = {
  padding: AppleSpace.sm,
  backgroundColor: cv('bg-secondary'),
  // 圆角走卡片档而不是 medium(12px)：同一层嵌套块在 AI 子页与列表页曾是两个半径
  borderRadius: AppleRadius.card,
  border: `1px solid ${cv('border-light')}`,
  boxShadow: cv('shadow-card'),
}

export const aiSubCardHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
}

export const aiSubCardNameRowStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
}

export const aiSubCardNameStyle = {
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
}

export const aiSubCardInfoStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  fontSize: AppleType.legalMeta.size,
  color: cv('text-secondary'),
  marginTop: AppleSpace.sm,
}

export const aiSubBadgeStyle = {
  fontFamily: AppleFont.mono,
  fontSize: AppleType.legalMeta.size,
  padding: `1px ${AppleSpace.sm}`,
  borderRadius: AppleRadius.micro,
  backgroundColor: cv('bg-tertiary'),
  color: cv('text-secondary'),
}

export const aiSubFormGridStyle = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fit, ${AppleGridFloor.stat})`,
  gap: AppleSpace.md,
  marginBottom: AppleSpace.lg,
}

export const aiSubFilterStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  // 组内**不许换行**：这一层是"label + 控件"一对，之前带 flexWrap:wrap，
  // 宽度一紧 label 就被折到控件上方，而内容短的组（一个徽章、一行提示）不折 ——
  // 同一行里于是同时出现「label 在上」和「label 同行」两种形态，顶边与中线都不可能齐。
  // 换行只该发生在**组与组之间**（toolbarStyle / filterBarStyle 那两层负责）。
  flexWrap: 'nowrap' as const,
  flexShrink: '0',
}

export const aiSubActionRowStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.md,
}

export const aiSubToolbarBetweenStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  marginBottom: AppleSpace.md,
  gap: AppleSpace.md,
  flexWrap: 'wrap' as const,
}

export const aiSubPageStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.md,
}

export const aiSubTagRowStyle = {
  display: 'flex',
  flexWrap: 'wrap' as const,
  gap: AppleSpace.xs,
}

export const aiSubTagStyle = {
  padding: `${AppleSpace.xxs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.micro,
  background: cv('bg-tertiary'),
  color: cv('text-secondary'),
  fontSize: AppleType.legalMeta.size,
}

export const aiSubCountBadgeStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
  backgroundColor: cv('bg-tertiary'),
  padding: `${AppleSpace.xxs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.pill,
}

export const aiSubResultStyle = (success: boolean) => ({
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  fontSize: AppleType.legalMeta.size,
  color: success ? cv('green') : cv('red'),
})

export const aiSubServerErrorStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  fontSize: AppleType.legalMeta.size,
  color: cv('red'),
  marginTop: AppleSpace.sm,
}

export const aiSubStatusDotStyle = (active: boolean) => ({
  width: AppleSpace.sm,
  height: AppleSpace.sm,
  borderRadius: AppleRadius.circle,
  backgroundColor: active ? cv('green') : cv('text-tertiary'),
})

export const aiSubToggleLabelStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-primary'),
}

export const aiSubLabelStyle = {
  fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-quaternary'),
  marginBottom: AppleSpace.xs,
  display: 'block',
}

export const aiSubFilterLabelStyle = {
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
  whiteSpace: 'nowrap' as const,
}

export const aiSubBodyStyle = {
  fontFamily: AppleFont.mono,
  fontSize: AppleType.codeBlock.size,
  lineHeight: AppleLineHeight.loose,
  padding: AppleSpace.md,
  backgroundColor: cv('bg-secondary'),
  borderRadius: AppleRadius.small,
  color: cv('text-primary'),
  whiteSpace: 'pre-wrap' as const,
  wordBreak: 'break-word' as const,
  maxHeight: '400px',
  overflowY: 'auto' as const,
  cursor: 'pointer',
}

export const aiSubDescStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-secondary'),
  marginBottom: AppleSpace.sm,
  lineHeight: AppleLineHeight.default,
}

// ============================================================
// Stream component shared styles (used by Stream* components)
// ============================================================

export const streamCardStyle = {
  background: cv('bg-secondary'),
  borderRadius: AppleRadius.card,
  padding: AppleSpace.sm,
}

// ============================================================
// AI table column width constants
// ============================================================
