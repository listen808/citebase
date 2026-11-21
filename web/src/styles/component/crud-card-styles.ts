import { AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleTransition, AppleType, cv, AppleGridFloor, appleFixedColumns } from '@/utils/core/apple-tokens';

// ==================== CRUD Card 样式 ====================

export function crudStatsRowStyle(size: 'default' | 'compact' = 'default'): Record<string, string> {
  return {
    display: 'grid',
    // auto-fit 会把空轨道塌掉，于是「只有 1 个统计」时那张卡拉成通栏一条（收藏页实测）。
    // auto-fill 保留轨道，统计卡恒定一块的宽度，多出来的空间留在右侧。
    gridTemplateColumns: `repeat(auto-fill, ${AppleGridFloor.stat})`,
    gap: size === 'compact' ? AppleSpace.sm : AppleSpace.md,
    marginBottom: size === 'compact' ? AppleSpace.md : AppleSpace.lg,
  };
}

/**
 * 按卡数排布的统计行（审查 §四.4）：卡数除不尽轨道数时，auto-fill 会让孤儿卡独占一行。
 * 宽屏 ≤5 卡一行等分，≥6 卡回落 3 列（两行各半）；窄屏仍走 auto-fill 自然换行。
 */
export function crudStatsRowFitStyle(count: number, isNarrow: boolean, size: 'default' | 'compact' = 'compact'): Record<string, string> {
  const base = crudStatsRowStyle(size);
  if (isNarrow || count <= 0) return base;
  return { ...base, gridTemplateColumns: appleFixedColumns(count <= 5 ? count : 3) };
}

export function crudFilterBarStyle(size: 'default' | 'compact' = 'default'): Record<string, string> {
  return {
    display: 'flex',
    // flex-end 而不是 center：工具栏里既有 40px 的按钮/搜索框，也有「上方带 label」的
    // 下拉（合计 ~62px 高）。center 会让按钮悬在 label 块的垂直中间，看着就是没对齐。
    // 底对齐让所有控件共用一条基线，label 往上长。
    alignItems: 'flex-end',
    gap: size === 'compact' ? AppleSpace.sm : AppleSpace.md,
    marginBottom: size === 'compact' ? AppleSpace.md : AppleSpace.lg,
    flexWrap: 'wrap',
  };
}

/**
 * 工具条**行内**的竖分隔线。别拿 `dividerStyle` 顶替：那条是 100% 宽的横分隔线，
 * 放进 `flexWrap:wrap` 的工具条里每插一条就把同一行切成两行（媒体页实测一行被切成三行）。
 */
export function toolbarDividerStyle(): Record<string, string> {
  return {
    width: '1px',
    height: '24px',
    alignSelf: 'center',
    flexShrink: '0',
    background: cv('border-light'),
  };
}

/** 工具条里「看哪些」与「怎么看」之间的那根弹簧 —— 全站只有这一处定义。 */
export function toolbarSpacerStyle(): Record<string, string> {
  return {
    flex: '1',
  };
}

export function crudPaginationStyle(size: 'default' | 'compact' = 'default'): Record<string, string> {
  return {
    display: 'flex',
    justifyContent: 'center',
    alignItems: 'center',
    gap: size === 'compact' ? AppleSpace.sm : AppleSpace.md,
    flexWrap: 'wrap',
    marginTop: size === 'compact' ? AppleSpace.md : AppleSpace.lg,
  };
}

export function crudCardStyle(): Record<string, string> {
  return {
    borderLeft: `4px solid ${cv('blue-selection-strong')}`,
    transition: AppleTransition.smooth,
  };
}

export function crudCardInnerStyle(): Record<string, string> {
  return {
    padding: AppleSpace.md,
    display: 'flex',
    flexDirection: 'column',
    gap: AppleSpace.md,
  };
}

export function crudCardHeaderRowStyle(): Record<string, string> {
  return {
    display: 'flex',
    flexDirection: 'column',
    gap: AppleSpace.sm,
  };
}

export function crudCardTitleRowStyle(): Record<string, string> {
  return {
    display: 'flex',
    alignItems: 'center',
    gap: AppleSpace.md,
  };
}

export function crudCardTitleWrapStyle(): Record<string, string> {
  return {
    flex: '1',
    minWidth: '0',
    display: 'flex',
    flexDirection: 'column',
    gap: AppleSpace.xs,
  };
}

export function crudCardBodyDetailStyle(): Record<string, string> {
  return {
    display: 'flex',
    flexDirection: 'column',
    gap: AppleSpace.sm,
    flex: '1',
  };
}

export function crudInfoRowStyle(): Record<string, string> {
  return {
    display: 'flex',
    justifyContent: 'space-between',
    alignItems: 'center',
    gap: AppleSpace.sm,
  };
}

export function crudInfoLabelStyle(): Record<string, string> {
  return {
    fontSize: AppleType.controlLabel.size,
    color: cv('text-secondary'),
    flexShrink: '0',
  };
}

export function crudInfoValueStyle(): Record<string, string> {
  return {
    fontSize: AppleType.controlLabel.size,
    color: cv('text-primary'),
    fontWeight: String(AppleFontWeight.medium),
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
    textAlign: 'right',
    flex: '1',
  };
}

export function crudCardFooterStyle(): Record<string, string> {
  return {
    display: 'flex',
    gap: AppleSpace.xs,
    paddingTop: AppleSpace.md,
    borderTop: `1px solid ${cv('border-light')}`,
    flexWrap: 'wrap',
  };
}

export function crudCodeStyle(): Record<string, string> {
  return {
    fontFamily: AppleFont.mono,
    fontSize: AppleType.microUI.size,
    background: cv('bg-tertiary'),
    padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
    borderRadius: AppleRadius.micro,
    color: cv('text-primary'),
  };
}

export function crudSearchStyle(): Record<string, string> {
  return {
    width: '240px',
    flexShrink: '0',
  };
}
