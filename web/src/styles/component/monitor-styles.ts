import { AppleFont, AppleFontWeight, AppleSpace, AppleType, cv } from '@/utils/core/apple-tokens';

export const monitorChartTitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  margin: '0',
};

export const monitorHealthDescStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-secondary'),
  margin: '0',
  textAlign: 'center' as const,
};

export const monitorHealthMetricItemStyle = {
  // 名称与数值同一行、两端对齐：这组是「事实清单」不是进度条，
  // 之前竖排 + 满格 ApProgress 让三项读起来像三条蓝色分隔线。
  display: 'flex',
  alignItems: 'baseline',
  justifyContent: 'space-between',
  gap: AppleSpace.sm,
};

export const monitorHealthMetricLabelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
};

export const monitorHealthMetricValueStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
};

export const monitorProgressHeaderStyle = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  marginBottom: AppleSpace.sm,
};

export const monitorProgressLabelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
};

export const monitorProgressPercentStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  color: cv('blue'),
  fontWeight: String(AppleFontWeight.medium),
};

export const monitorProgressDetailStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
  marginTop: AppleSpace.xs,
};

export const monitorInfoGridStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
};

export const monitorInfoItemStyle = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  padding: `${AppleSpace.xs} 0`,
};

export const monitorInfoLabelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
};

export const monitorInfoValueStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
};
