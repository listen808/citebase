import { AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleTransition, AppleType, AppleLineHeight, cv } from '@/utils/core/apple-tokens';

// ==================== 文档页面样式 ====================

export const docLayoutStyle = {
  display: 'flex',
  gap: AppleSpace.xl,
  alignItems: 'flex-start',
};

export const tocNavStyle = {
  width: '200px',
  minWidth: '200px',
  flex: '1',
  overflowY: 'auto' as const,
};

export const tocHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  marginBottom: AppleSpace.md,
  paddingBottom: AppleSpace.sm,
  borderBottom: `1px solid ${cv('border-light')}`,
};

export const tocHeaderLabelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-secondary'),
  letterSpacing: '0.02em',
};

export const tocListStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
};

export const tocItemStyle = (level: number) => ({
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-secondary'),
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.small,
  cursor: 'pointer',
  transition: AppleTransition.fast,
  paddingLeft: level === 2 ? AppleSpace.lg : AppleSpace.sm,
});

export const tocItemActiveStyle = () => ({
  color: cv('blue'),
  background: cv('blue-selection'),
});

export const readingAreaStyle = {
  flex: '1',
  minWidth: '0',
  maxWidth: '680px',
};

export const contentStyle = {
  marginTop: '0',
};

export const docHeadingStyle = {
  fontFamily: AppleType.utilityHeading.font,
  fontSize: AppleType.utilityHeading.size,
  fontWeight: String(AppleType.utilityHeading.weight),
  lineHeight: String(AppleType.utilityHeading.lineHeight),
  letterSpacing: AppleType.utilityHeading.letterSpacing,
  color: cv('text-primary'),
  margin: `0 0 ${AppleSpace.lg} 0`,
  scrollMarginTop: '100px',
};

export const docSubheadingStyle = {
  fontFamily: AppleType.subhead.font,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleType.subhead.weight),
  lineHeight: String(AppleType.subhead.lineHeight),
  letterSpacing: AppleType.subhead.letterSpacing,
  color: cv('text-primary'),
  margin: `${AppleSpace.xl} 0 ${AppleSpace.md} 0`,
  scrollMarginTop: '100px',
};

export const docBodyTextStyle = {
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleType.bodyPrimary.weight),
  lineHeight: String(AppleType.bodyPrimary.lineHeight),
  letterSpacing: AppleType.bodyPrimary.letterSpacing,
  color: cv('text-secondary'),
  margin: `0 0 ${AppleSpace.lg} 0`,
};

export const docCodeBlockStyle = {
  background: cv('bg-tertiary'),
  border: `1px solid ${cv('border')}`,
  borderRadius: AppleRadius.medium,
  padding: AppleSpace.lg,
  marginBottom: AppleSpace.lg,
  overflowX: 'auto' as const,
};

export const docPreStyle = {
  margin: '0',
  fontFamily: AppleFont.mono,
  fontSize: AppleType.microUI.size,
  color: cv('text-primary'),
  lineHeight: AppleLineHeight.wide,
  whiteSpace: 'pre' as const,
};

export const docTableStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: '0',
  marginBottom: AppleSpace.lg,
  borderRadius: AppleRadius.medium,
  border: `1px solid ${cv('border-light')}`,
  overflow: 'hidden',
};

export const docTableRowStyle = {
  display: 'flex',
  gap: AppleSpace.md,
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  borderBottom: `1px solid ${cv('border-light')}`,
  alignItems: 'center',
};

export const docFieldCodeStyle = {
  fontFamily: AppleFont.mono,
  fontSize: AppleType.microUI.size,
  color: cv('blue'),
  minWidth: '200px',
  padding: `${AppleSpace.xxs} ${AppleSpace.xs}`,
  background: cv('bg-tertiary'),
  borderRadius: AppleRadius.small,
  flexShrink: '0',
};

export const docFieldDescStyle = {
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-secondary'),
  lineHeight: String(AppleType.bodyPrimary.lineHeight),
};

export const docStepsListStyle = {
  paddingLeft: AppleSpace.md,
  margin: `0 0 ${AppleSpace.lg} 0`,
};

export const docStepItemStyle = {
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.bodyPrimary.size,
  lineHeight: String(AppleType.bodyPrimary.lineHeight),
  letterSpacing: AppleType.bodyPrimary.letterSpacing,
  color: cv('text-secondary'),
  marginBottom: AppleSpace.sm,
};

export const docTabSwitchStyle = {
  marginTop: AppleSpace.xl,
  display: 'flex',
  justifyContent: 'center',
};
