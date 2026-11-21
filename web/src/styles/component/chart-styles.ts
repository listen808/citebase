import { AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleType, cv, AppleGridFloor } from '@/utils/core/apple-tokens';

// ==================== Chart 样式 ====================

/** Apple 图表标准色板 - 8 色和谐调色板 */
export const AppleChartColors = [
  cv('blue'),
  cv('green'),
  cv('orange'),
  cv('red'),
  cv('purple'),
  cv('indigo'),
  cv('teal'),
  cv('yellow'),
  cv('text-secondary'),
] as const;

export function chartContainerStyle(): Record<string, string> {
  return {
    padding: `${AppleSpace.md} ${AppleSpace.lg}`,
    borderRadius: AppleRadius.card,
    background: cv('surface-panel'),
    border: `1px solid ${cv('border-light')}`,
    boxShadow: cv('shadow-card'),
    marginBottom: AppleSpace.xl,
  };
}

export function pieChartStyle(segments: Array<{ color: string; percent: number }>): Record<string, string> {
  const gradientParts = segments.map((seg, i) => {
    const start = segments.slice(0, i).reduce((sum, s) => sum + s.percent, 0);
    const end = start + seg.percent;
    return `${seg.color} ${start}% ${end}%`;
  });
  return {
    width: AppleSpace['4xl'],
    height: AppleSpace['4xl'],
    borderRadius: AppleRadius.circle,
    background: `conic-gradient(${gradientParts.join(', ')})`,
  };
}

export function chartTitleStyle(): Record<string, string> {
  return {
    fontFamily: AppleType.bodyEmphasis.font,
    fontSize: AppleType.controlLabel.size,
    fontWeight: String(AppleFontWeight.semibold),
    color: cv('text-primary'),
  };
}

export function legendDotStyle(): Record<string, string> {
  return {
    display: 'inline-flex',
    alignItems: 'center',
    gap: AppleSpace.xs,
    fontFamily: AppleType.microUI.font,
    fontSize: AppleType.microUI.size,
    color: cv('text-secondary'),
    padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
    borderRadius: AppleRadius.control,
  };
}

export function barLabelStyle(): Record<string, string> {
  return {
    fontFamily: AppleType.microUI.font,
    fontSize: AppleType.microUI.size,
    color: cv('text-tertiary'),
  };
}

export function chartRowStyle(): Record<string, string> {
  return {
    display: 'grid',
    gridTemplateColumns: `repeat(auto-fill, ${AppleGridFloor.panel})`,
    gap: AppleSpace.lg,
  };
}

export function chartColStyle(): Record<string, string> {
  return {
    minWidth: '0',
  };
}

export function barChartContainerStyle(): Record<string, string> {
  return {
    display: 'flex',
    alignItems: 'flex-end',
    gap: AppleSpace.xs,
    height: '200px',
    padding: `${AppleSpace.md} 0`,
    overflowX: 'auto',
  };
}

export function barChartColumnStyle(): Record<string, string> {
  return {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: AppleSpace.xs,
    flex: '1',
    minWidth: '20px',
  };
}

export function barChartBarWrapperStyle(): Record<string, string> {
  // 原先每根柱后面挂着一条满高的灰底轨道（bg-tertiary + control 圆角）：
  // 一排等宽灰胶囊在空数据日尤其像加载骨架，柱子一高就成蓝白相间的色块。
  // 轨道去掉，柱宽收到 28px（见各页柱体 maxWidth），基线以下不再画东西。
  return {
    width: '100%',
    height: '160px',
    display: 'flex',
    alignItems: 'flex-end',
    justifyContent: 'center',
  };
}

export function barChartLabelStyle(): Record<string, string> {
  return {
    fontFamily: AppleFont.text,
    fontSize: AppleType.legalMeta.size,
    color: cv('text-tertiary'),
    whiteSpace: 'nowrap',
  };
}

/**
 * 柱图 y 轴刻度列（上限 / 中值 / 0）。
 * 评审 P1-4：三张手写柱图此前只有柱子和 x 轴标签，读者无法判断量级；
 * 高度必须与柱区一致（调用方传柱区高度），三档刻度按 justify-between 贴住上下沿。
 */
export function barChartAxisStyle(height: string): Record<string, string> {
  return {
    display: 'flex',
    flexDirection: 'column',
    justifyContent: 'space-between',
    alignItems: 'flex-end',
    height,
    flexShrink: '0',
  };
}

export function barChartAxisLabelStyle(): Record<string, string> {
  return {
    fontFamily: AppleFont.mono,
    fontSize: AppleType.legalMeta.size,
    color: cv('text-quaternary'),
    fontVariantNumeric: 'tabular-nums',
    whiteSpace: 'nowrap',
  };
}

export function pieChartContainerStyle(): Record<string, string> {
  return {
    display: 'flex',
    alignItems: 'center',
    gap: AppleSpace.lg,
    padding: AppleSpace.md,
    height: '200px',
  };
}

export function pieLegendStyle(): Record<string, string> {
  return {
    display: 'flex',
    flexDirection: 'column',
    gap: AppleSpace.sm,
    flex: '1',
    overflow: 'hidden',
  };
}

export function pieLegendItemStyle(): Record<string, string> {
  return {
    display: 'flex',
    alignItems: 'center',
    gap: AppleSpace.sm,
  };
}

export function heatmapContainerStyle(): Record<string, string> {
  return {
    padding: AppleSpace.md,
    display: 'flex',
    flexDirection: 'column',
    gap: AppleSpace.sm,
    alignItems: 'center',
  };
}

export function heatmapLegendStyle(): Record<string, string> {
  return {
    display: 'flex',
    alignItems: 'center',
    gap: AppleSpace.sm,
    marginTop: AppleSpace.sm,
  };
}
