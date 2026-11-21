import { labelStyle as formFieldLabelStyle } from '@/styles/component/form-field-styles';
import { AppleFont, AppleFontWeight, AppleLayout, AppleLineHeight, AppleRadius, AppleSpace, AppleTransition, AppleType, cv, AppleGridFloor, subtleBg, appleCardGrid, APPLE_ICON_TINT_ALPHA } from '@/utils/core/apple-tokens';
import { hexToRgba } from '@/utils/core/shared';

// 网格的唯一标准是 utils/core/apple-tokens.ts 的 appleCardGrid(floor, gap?)。
// 本模块只保留地板档位不同的 gridStandard(stat) / gridWide(panel)；
// entity 档的 gridStyle / cardGridStyle / gridContent 三个同义导出已删除，消费方直接用 appleCardGrid('entity')。

// ==================== Domain Module Re-exports ====================
export { AppleChartColors, chartContainerStyle, pieChartStyle, chartTitleStyle, legendDotStyle, barLabelStyle, chartRowStyle, chartColStyle, barChartContainerStyle, barChartColumnStyle, barChartBarWrapperStyle, barChartLabelStyle, barChartAxisStyle, barChartAxisLabelStyle, pieChartContainerStyle, pieLegendStyle, pieLegendItemStyle, heatmapContainerStyle, heatmapLegendStyle } from '@/styles/component/chart-styles';
export { crudStatsRowStyle, crudStatsRowFitStyle, crudFilterBarStyle, toolbarDividerStyle, toolbarSpacerStyle, crudPaginationStyle, crudCardStyle, crudCardInnerStyle, crudCardHeaderRowStyle, crudCardTitleRowStyle, crudCardTitleWrapStyle, crudCardBodyDetailStyle, crudInfoRowStyle, crudInfoLabelStyle, crudInfoValueStyle, crudCardFooterStyle, crudCodeStyle, crudSearchStyle } from '@/styles/component/crud-card-styles';
export { monitorGridStyle, monitorLabelStyle, monitorValueStyle, rankListStyle, rankItemStyle, rankNumberStyle, rankNameStyle, rankCountStyle } from '@/styles/component/dashboard-styles';
export { searchResultCardStyle } from '@/styles/component/search-result-styles';

export { docLayoutStyle, tocNavStyle, tocHeaderStyle, tocHeaderLabelStyle, tocListStyle, tocItemStyle, tocItemActiveStyle, readingAreaStyle, contentStyle, docHeadingStyle, docSubheadingStyle, docBodyTextStyle, docCodeBlockStyle, docPreStyle, docTableStyle, docTableRowStyle, docFieldCodeStyle, docFieldDescStyle, docStepsListStyle, docStepItemStyle, docTabSwitchStyle } from '@/styles/component/doc-styles';
export { monitorChartTitleStyle, monitorHealthDescStyle, monitorHealthMetricItemStyle, monitorHealthMetricLabelStyle, monitorHealthMetricValueStyle, monitorProgressHeaderStyle, monitorProgressLabelStyle, monitorProgressPercentStyle, monitorProgressDetailStyle, monitorInfoGridStyle, monitorInfoItemStyle, monitorInfoLabelStyle, monitorInfoValueStyle } from '@/styles/component/monitor-styles';



export const loadingStyle = {
  padding: AppleSpace.xl,
  textAlign: 'center' as const,
  color: cv('text-quaternary'),
  fontFamily: AppleFont.text,
};

export const loadingTextStyle = {
  textAlign: 'center' as const,
  color: cv('text-quaternary'),
  fontFamily: AppleFont.text,
};

export const emptyStateStyle = {
  padding: `${AppleSpace.lg} 0`,
  textAlign: 'center' as const,
  color: cv('text-tertiary'),
  display: 'flex',
  flexDirection: 'column' as const,
  alignItems: 'center',
  gap: AppleSpace.md,
  fontFamily: AppleFont.text,
  fontSize: AppleType.bodyPrimary.size,
};

export const filterBarStyle = {
  display: 'flex',
  alignItems: 'center',
  flexWrap: 'wrap' as const,
  gap: AppleSpace.md,
  marginBottom: AppleSpace.md,
};

export const formRowStyle = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fit, ${AppleGridFloor.stat})`,
  gap: AppleSpace.md,
  marginBottom: AppleSpace.md,
};

/** 标准网格 - 数值块 / 指标卡 */
export const gridStandard = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fill, ${AppleGridFloor.stat})`,
  gap: AppleSpace.md,
  gridAutoRows: '1fr',
  alignItems: 'stretch',
  // 行轨不吸收容器余量：卡片网格被 flex 父级拉高时，1fr 行轨会把余量摊进每一行，
  // 于是「三张联系人卡撑满整屏、卡中间一片空洞」。等高靠 align-items:stretch，
  // 不靠让行轨长高，所以这里必须 start。
  alignContent: 'start',
};

/** 宽块网格 - 图表 / 详情面板 */
export const gridWide = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fill, ${AppleGridFloor.panel})`,
  gap: AppleSpace.md,
  gridAutoRows: '1fr',
  alignItems: 'stretch',
  alignContent: 'start',
};

export const avatarStyle = {
  width: AppleLayout.avatarSizeMedium,
  height: AppleLayout.avatarSizeMedium,
  borderRadius: AppleRadius.control,
  background: cv('blue'),
  color: cv('on-blue'),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  fontSize: AppleType.linkActionHeading.size,
  fontWeight: String(AppleFontWeight.semibold),
  flexShrink: '0',
};

export const cardTitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  marginBottom: AppleSpace.xs,
};

export const cardMetaStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  display: 'flex',
  gap: AppleSpace.sm,
  alignItems: 'center',
  flexWrap: 'wrap' as const,
};

export const actionGroupStyle = {
  display: 'flex',
  gap: AppleSpace.sm,
  flexWrap: 'wrap' as const,
  flexShrink: '0',
};

export const cardBodyStyle = {
  flex: '1',
  minWidth: 0,
};

export const timeStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
};

export const searchRowStyle = {
  marginBottom: AppleSpace.lg,
  maxWidth: '360px',
};

export const searchBarStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  marginBottom: AppleSpace.md,
  flexWrap: 'wrap' as const,
};

export const tabBarStyle = {
  marginBottom: AppleSpace.lg,
};

export const badgeStyle = {
  display: 'inline-flex',
  alignItems: 'center',
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.micro,
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
};

/**
 * 纵向列表容器唯一入口。默认 gap md；紧凑档传 AppleSpace.sm。
 * 原 listCompactStyle / cardListStyle 是同一条 gap:sm 的两个别名，已删除。
 */
export function listStyle(gap: string = AppleSpace.md): Record<string, string> {
  return {
    display: 'flex',
    flexDirection: 'column',
    gap,
  };
}

/**
 * 横向一行的唯一入口（`listStyle` 的横排对称版）：垂直居中，默认 gap xs。
 * 需要换行就 `{ ...rowStyle(gap), flexWrap: 'wrap' }` —— 不再各写一份三行对象，
 * 否则改对齐方式时整站要逐个文件追。
 */
export function rowStyle(gap: string = AppleSpace.xs): Record<string, string> {
  return {
    display: 'flex',
    alignItems: 'center',
    gap,
  };
}

export const formGroupStyle = {
  marginBottom: AppleSpace.md,
};

export const labelStyle = {
  display: 'block',
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  // 原先没有 fontWeight：同一批筛选条件里它当 <label> 用（媒体库）也当 <span> 用（日志页），
  // 无字重时两处读起来的分量不一样
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-tertiary'),
  marginBottom: `${AppleSpace.xs}`,
};

export const formStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.lg,
};

export const sectionStyle = {
  marginBottom: AppleSpace.lg,
};

/** 纵向堆叠的头部/区块（标题 + 描述 + 操作） */
export const stackColumnStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
};

export const sectionTitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  marginBottom: AppleSpace.md,
};

export const dividerStyle = {
  width: '100%',
  height: '1px',
  background: cv('border-light'),
  margin: `${AppleSpace.md} 0`,
};

export const searchBoxStyle = {
  flex: '1',
  maxWidth: '360px',
};

export const cardHeaderStyle = {
  marginBottom: AppleSpace.md,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
};


export const statusRowStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
};

export const toolbarStyle = {
  display: 'flex',
  // 工具栏一行里每组的**形态必须相同**（label 与控件同行），但**高度天然不同**：
  // 下拉 40px、徽章 24px、纯文字 19px。底对齐会把矮组整条拽下去，label 就比
  // 邻居低 10px（文档协作中心「当前审批」正是这样）。同形态行按中心对齐，
  // 于是所有 label 的中线重合 = 落在同一条水平线上。
  // 只有含「label 在控件上方」那种高组的行才用底对齐共用控件基线
  // （见 crudFilterBarStyle），check:boxes 的 B7/B8 分别把这两条钉住。
  alignItems: 'center' as const,
  gap: AppleSpace.md,
  marginBottom: AppleSpace.lg,
  flexWrap: 'wrap' as const,
};

export const descStyle = {
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
  margin: '0',
};

export const metaStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  flexWrap: 'wrap' as const,
  marginTop: AppleSpace.xs,
};

export const metaItemStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};

export const badgeBaseStyle = {
  display: 'inline-block',
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.micro,
  fontSize: AppleType.microUI.size,
};

export function iconBoxStyle(color?: string, size?: number) {
  const s = size ? `${size}px` : '44px';
  return {
    width: s,
    height: s,
    background: color || cv('blue'),
    borderRadius: AppleRadius.control,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: '0',
  };
}

export const totalStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-secondary'),
};

/** 警示条（琥珀）：不可逆操作前的独立提示行，避免与普通说明同权重（P1-10） */
export const warningBannerStyle = {
  display: 'flex',
  alignItems: 'flex-start' as const,
  gap: AppleSpace.xs,
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  borderRadius: AppleRadius.control,
  border: `1px solid ${cv('callout-warning-border')}`,
  background: cv('callout-warning-bg'),
  color: cv('text-primary'),
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  lineHeight: 1.5,
};

/**
 * 页面/表单里的"说明文字"。**故意不带任何 margin**：
 * 纵向间距由所在容器的 row-gap（布局尺上的一档）决定，元素自己再加 4px 就会
 * 把 24 变成 28 —— 不落尺，且每页随容器变化（check:boxes 的 B4 抓到过一条）。
 */
export const noteStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.legalMeta.size,
  lineHeight: AppleLineHeight.default,
  color: cv('text-tertiary'),
};

export const timelineLinkStyle = {
  // P1-3：活动/日志/用户主页里操作者是"谁做的"这层次要信息，不该压过 14px 的对象标题。
  // 原先继承父级 14px 且是蓝色中等字重，视觉上比对象名更重；降到 12px 次级色，可点性靠 pointer 光标保留。
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
  cursor: 'pointer',
  fontWeight: String(AppleFontWeight.medium),
};

// ==================== 媒体管理样式 ====================
// （图片预览的七份样式已随 ApImageLightbox 收口：那份浮层以前在四个地方各写一遍）

export const mediaImgGridStyle = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fill, ${AppleGridFloor.tile})`,
  gap: AppleSpace.lg,
};

export const mediaImgCardStyle = {
  overflow: 'hidden',
};

export const mediaImgMetaStyle = {
  padding: AppleSpace.sm,
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
};

export const mediaImgNameStyle = {
  fontFamily: AppleType.bodyEmphasis.font,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleType.bodyEmphasis.weight),
  color: cv('text-primary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

export const mediaImgSizeStyle = {
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
};

export const mediaImgActionRowStyle = {
  display: 'flex',
  borderTop: `1px solid ${cv('border-light')}`,
};

/**
 * 媒体/附件文件网格：entity 档网格，唯一差异是 gap 用 lg 而非 md
 * （文件卡缩略图较宽，8px 的默认间距会让相邻卡片视觉上粘在一起）。
 * 除此之外不要新写网格常量——用 appleCardGrid(floor, gap?)。
 */
export const mediaFileListStyle = appleCardGrid('entity', AppleSpace.lg);

export const mediaFileCardStyle = {
  padding: AppleSpace.md,
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.md,
};

export const mediaFileAvatarStyle = {
  width: AppleSpace['2xl'],
  height: AppleSpace['2xl'],
  borderRadius: AppleRadius.control,
  background: cv('blue'),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  flexShrink: '0',
};

export const mediaFileBodyStyle = {
  flex: '1',
  minWidth: '0',
};

export const mediaFileTitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  marginBottom: AppleSpace.xs,
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

export const mediaFileMetaStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  display: 'flex',
  gap: AppleSpace.sm,
  alignItems: 'center',
};

export const mediaFileActionStyle = {
  display: 'flex',
  gap: AppleSpace.sm,
  flexShrink: '0',
};

export const sectionHeaderStyle = {
  marginBottom: AppleSpace.md,
};

export const formActionsStyle = {
  paddingTop: AppleSpace.md,
  display: 'flex' as const,
  gap: AppleSpace.md,
  justifyContent: 'flex-start' as const,
};



// ==================== Section 扩展样式 ====================

export function sectionDescStyle(): Record<string, string> {
  return {
    fontFamily: AppleType.bodyPrimary.font,
    fontSize: AppleType.bodyPrimary.size,
    fontWeight: String(AppleType.bodyPrimary.weight),
    color: cv('text-secondary'),
    margin: '0',
  };
}

// ==================== Card 扩展样式 ====================

export function cardDescStyle(): Record<string, string> {
  return {
    fontFamily: AppleType.bodyPrimary.font,
    fontSize: AppleType.microUI.size,
    color: cv('text-secondary'),
    margin: '0',
    lineHeight: AppleLineHeight.default,
  };
}

export function cardActionsStyle(): Record<string, string> {
  return {
    display: 'flex',
    alignItems: 'center',
    gap: AppleSpace.sm,
    marginTop: AppleSpace.sm,
  };
}

// ==================== Tag/Badge 样式 ====================

export function tagStyle(color?: string): Record<string, string> {
  const c = color || cv('blue');
  return {
    display: 'inline-flex',
    alignItems: 'center',
    padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
    // 标签圆角走控件档（与 ApStatusBadge 同一档），['5px'] 是这一处独有的像素键
    borderRadius: AppleRadius.control,
    fontFamily: AppleFont.text,
    fontSize: AppleType.microUI.size,
    fontWeight: String(AppleFontWeight.medium),
    background: hexToRgba(color || cv('blue'), 0.1),
    color: c,
  };
}

// ==================== Timeline 样式 ====================

export function timelineStyle(): Record<string, string> {
  return {
    display: 'flex',
    flexDirection: 'column',
    gap: '0',
    position: 'relative',
    paddingLeft: AppleSpace.lg,
  };
}

export function timelineNodeStyle(color: string): Record<string, string> {
  return {
    width: AppleSpace.sm,
    height: AppleSpace.sm,
    borderRadius: AppleRadius.circle,
    background: color,
    position: 'absolute',
    left: `-${AppleSpace.sm}`,
    top: AppleSpace.xs,
    border: `2px solid ${cv('pure-white-canvas')}`,
    boxShadow: cv('shadow-card'),
  };
}

export function timelineItemStyle(): Record<string, string> {
  return {
    position: 'relative',
    paddingBottom: AppleSpace.lg,
    borderLeft: `2px solid ${cv('border-light')}`,
    paddingLeft: AppleSpace.md,
  };
}

export const timelineContentStyle = {
  flex: '1',
  minWidth: '0',
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
  cursor: 'pointer',
};

export const timelineEntryHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  flexWrap: 'wrap' as const,
};

export const timelineDocNameStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
  flex: '1',
  minWidth: '0',
};

export const timelineEntryMetaStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  flexWrap: 'wrap' as const,
};

// ==================== Form 扩展样式 ====================

export function formSectionStyle(): Record<string, string> {
  return {
    padding: AppleSpace.lg,
    borderRadius: AppleRadius.card,
    background: cv('surface-panel'),
    boxShadow: cv('shadow-card'),
    marginBottom: AppleSpace.lg,
  };
}

export function formLabelStyle(): Record<string, string> {
  return {
    ...formFieldLabelStyle(),
    display: 'block',
    marginBottom: AppleSpace.xs,
  };
}

// ==================== Split Layout 样式 ====================

export function splitLayoutStyle(): Record<string, string> {
  return {
    display: 'flex',
    flex: '1',
    minHeight: '0',
    overflow: 'hidden',
  };
}

// ==================== Status 样式 ====================

export function statusDotStyle(status: 'online' | 'offline' | 'enabled' | 'disabled'): Record<string, string> {
  const colors: Record<string, string> = {
    online: cv('green'),
    offline: cv('text-quaternary'),
    enabled: cv('blue'),
    disabled: cv('text-quaternary'),
  };
  return {
    width: AppleSpace.sm,
    height: AppleSpace.sm,
    borderRadius: AppleRadius.circle,
    background: colors[status] ?? cv('text-quaternary'),
    display: 'inline-block',
    flexShrink: '0',
  };
}

export function statusIndicatorLargeStyle(status: 'healthy' | 'warning' | 'error'): Record<string, string> {
  const colors: Record<string, string> = {
    healthy: cv('green'),
    warning: cv('orange'),
    error: cv('red'),
  };
  const bgs: Record<string, string> = {
    healthy: cv('green-bg-subtle'),
    warning: cv('orange-bg-subtle'),
    error: cv('red-bg-subtle'),
  };
  return {
    display: 'inline-flex',
    alignItems: 'center',
    gap: AppleSpace.sm,
    padding: `${AppleSpace.xs} ${AppleSpace.md}`,
    borderRadius: AppleRadius.pill,
    background: bgs[status] ?? 'transparent',
    color: colors[status] ?? cv('text-secondary'),
    fontFamily: AppleFont.text,
    fontSize: AppleType.controlLabel.size,
    fontWeight: String(AppleFontWeight.medium),
  };
}

// ==================== Alphabet Index 样式 ====================

export function alphabetIndexStyle(): Record<string, string> {
  return {
    display: 'flex',
    flexDirection: 'column',
    alignItems: 'center',
    gap: '2px',
    padding: `${AppleSpace.xs} ${AppleSpace.xs}`,
    position: 'sticky',
    top: AppleSpace.md,
    // 索引列被 flex 挤压时字母会换行竖叠，右侧留白再窄也不能缩它
    flexShrink: '0',
    maxHeight: 'calc(100dvh - 200px)',
    overflowY: 'auto',
  };
}

export function alphabetIndexItemStyle(active: boolean): Record<string, string> {
  return {
    width: AppleSpace.md,
    height: AppleSpace.md,
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    borderRadius: AppleRadius.circle,
    fontFamily: AppleFont.text,
    fontSize: AppleType.microUI.size,
    fontWeight: String(active ? AppleFontWeight.semibold : AppleFontWeight.regular),
    color: active ? cv('blue') : cv('text-tertiary'),
    background: active ? cv('blue-bg-subtle') : 'transparent',
    cursor: 'pointer',
    transition: AppleTransition.smooth,
  };
}

export function entityMetaRowStyle(): Record<string, string> {
  return {
    fontFamily: AppleType.microUI.font,
    fontSize: AppleType.microUI.size,
    color: cv('text-tertiary'),
    display: 'flex',
    gap: AppleSpace.sm,
    alignItems: 'center',
  };
}

export function infoValueStyle(): Record<string, string> {
  return {
    fontFamily: AppleFont.text,
    fontSize: AppleType.bodyPrimary.size,
    fontWeight: String(AppleFontWeight.medium),
    color: cv('text-primary'),
    lineHeight: AppleLineHeight.title,
    wordBreak: 'break-all',
  };
}

// ==================== Log 页面样式 ====================

export function filterGroupStyle(): Record<string, string> {
  return {
    display: 'flex',
    flexDirection: 'column',
    gap: AppleSpace.xs,
  };
}

export function dateRangeStyle(): Record<string, string> {
  return {
    display: 'flex',
    alignItems: 'center',
    gap: AppleSpace.sm,
  };
}

export function dateInputStyle(): Record<string, string> {
  return {
    width: '140px',
  };
}

export function filterButtonsStyle(): Record<string, string> {
  return {
    display: 'flex',
    gap: AppleSpace.xs,
  };
}

/**
 * 页面纵向节奏唯一入口。maxWidth / pageSpacing 曾与 ApPageContainer 的内置壳层重复
 * （页宽与外边距由容器统一负责），importer 为 0，已删除。
 */
export const PageLayout = {
  sectionGap: {
    display: 'flex',
    flexDirection: 'column' as const,
    gap: AppleSpace.lg,
  },
};

export function filterSelectStyle(): Record<string, string> {
  return {
    width: '140px',
  };
}


// ==================== Gradient Card 扩展样式 ====================

export function entityMetaItemStyle(): Record<string, string> {
  return {
    display: 'inline-flex',
    alignItems: 'center',
    gap: AppleSpace.xs,
    fontFamily: AppleType.microUI.font,
    fontSize: AppleType.microUI.size,
    color: cv('text-tertiary'),
  };
}

export function groupHeaderStyle(): Record<string, string> {
  return {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    padding: `${AppleSpace.sm} ${AppleSpace.md}`,
    background: cv('bg-secondary'),
    borderRadius: AppleRadius.control,
    marginBottom: AppleSpace.md,
    cursor: 'pointer',
    transition: AppleTransition.smooth,
  };
}

export function groupTitleWrapperStyle(): Record<string, string> {
  return {
    display: 'flex',
    alignItems: 'center',
    gap: AppleSpace.sm,
  };
}

export function groupTitleStyle(): Record<string, string> {
  return {
    fontFamily: AppleType.bodyEmphasis.font,
    fontSize: AppleType.controlLabel.size,
    fontWeight: String(AppleFontWeight.semibold),
    color: cv('text-primary'),
  };
}

export function groupCountStyle(): Record<string, string> {
  return {
    fontFamily: AppleType.microUI.font,
    fontSize: AppleType.microUI.size,
    color: cv('text-tertiary'),
    background: cv('bg-tertiary'),
    padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
    borderRadius: AppleRadius.control,
  };
}

// 列表条目的图标底块唯一标准：32px 浅染底 + 彩色图标（收藏页/关注页/仪表盘系统信息共用）
// 注释曾写 32px 而实现取 AppleSpace['2xl']=48px，三个消费方一起大了一档；这里以注释为准。
export function entityIconBoxStyle(color: string): Record<string, string> {
  return {
    width: AppleSpace.xl,
    height: AppleSpace.xl,
    borderRadius: AppleRadius.small,
    background: subtleBg(color, APPLE_ICON_TINT_ALPHA),
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'center',
    flexShrink: '0',
  };
}

export function detailPanelStyle(): Record<string, string> {
  return {
    background: cv('surface-panel'),
    borderRadius: AppleRadius.card,
    border: `1px solid ${cv('border-light')}`,
    boxShadow: cv('shadow-card'),
    overflow: 'hidden',
  };
}

/**
 * 标准区块样式
 * 底部使用更宽松的间距
 */
export const standardSectionStyle = {
  marginBottom: AppleSpace.lg,
};

export function logCardDividerStyle(): Record<string, string> {
  return {
    color: cv('text-quaternary'),
    fontSize: AppleType.microUI.size,
  };
}

export function criticalBadgeStyle(): Record<string, string> {
  return {
    marginLeft: 'auto',
  };
}

export function logPathStyle(): Record<string, string> {
  return {
    fontFamily: AppleFont.text,
    fontSize: AppleType.microUI.size,
    color: cv('text-primary'),
    flex: '1',
    overflow: 'hidden',
    textOverflow: 'ellipsis',
    whiteSpace: 'nowrap',
  };
}

export function statusTitleStyle(): Record<string, string> {
  return {
    display: 'flex',
    alignItems: 'center',
  };
}

export function actionsGridStyle(): Record<string, string> {
  return {
    display: 'flex',
    gap: AppleSpace.md,
    flexWrap: 'wrap',
  };
}


// ==================== Contact 样式 ====================

export function contactViewWrapperStyle(): Record<string, string> {
  return {
    display: 'flex',
    gap: AppleSpace.lg,
    position: 'relative',
  };
}

// ==================== 行内文字链接 ====================

/**
 * 指向站外/站内的可点文字。此前"URL 只是文本"的页面不止一处，
 * 而按钮形态（ApButton）不支持 href —— 真链接必须是 `<a href>`：
 * 能进 Tab 序、能中键/新标签打开、吃全局 a[href] 的按压反馈，也不会被弹窗拦截器挡掉。
 */
/**
 * 行内链接的**命中区补偿**：`padding + 等值负 margin`。
 *
 * 为什么不是 `minHeight`：行内链接的高度由字号决定（12–17px 那几档），补 minHeight
 * 会把整行连页头/卡片一起顶高，并把短标签往两边推开 —— 而这一族要的是
 * **一个像素都不移动，只把可点面积换到手**。负 margin 与 padding 同值，
 * flex/块流里该元素的布局贡献净为零，而 `getBoundingClientRect()` 量的是含 padding
 * 的边框盒，所以 check:apple A5 认它达标。
 *
 * 为什么单独成一条而不是各写各的：同一个手法今天用在 URL 链接与关于页的许可链接上，
 * 面包屑那条是它的**两轴版**（那里横向也要补，因为短标签只有 11px 宽），
 * 再各写第三份就是本仓库反复付学费的那件"第二处定义"。
 * 纵向 8px 走 AppleSpace.sm（在 R5 的尺子上）。
 */
export const inlineHitArea = {
  padding: `${AppleSpace.sm} 0`,
  margin: `-${AppleSpace.sm} 0`,
} as const;

export const inlineLinkStyle = {
  display: 'inline-flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  minWidth: '0',
  maxWidth: '100%',
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  color: cv('blue'),
  textDecoration: 'none',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
  // 命中区：13px 字号 + 零内边距 → 实测 16px 高（check:apple A5 在 /links 上量到 382×16 ×7，
  // 就是链接卡上那个真 URL）。
  ...inlineHitArea,
};

// ==================== Media 扩展样式 ====================

/**
 * 工具条行内的「label 在上 + 下拉」定宽组。
 * 原先是 `flex:1 + minWidth:200px`：它独占一行时撑满是想要的，但并进工具条那一行后，
 * 两个撑满组把同一行的类型/视图切换挤到下一行（媒体页实测一行被切成三行）。
 * 定宽、不 grow，和 `crudSearchStyle` 的 240px 同一档。
 */
export function filterSelectGroupStyle(): Record<string, string> {
  return {
    display: 'flex',
    flexDirection: 'column',
    gap: AppleSpace.sm,
    flex: '0 0 auto',
    width: '240px',
  };
}

export const mediaDownloadActionStyle = {
  flex: '1',
  padding: AppleSpace.sm,
  border: 'none',
  borderRight: `1px solid ${cv('border-light')}`,
  background: 'transparent',
  color: cv('blue'),
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
  cursor: 'pointer',
  textDecoration: 'none',
  textAlign: 'center' as const,
};

export const mediaDownloadLinkStyle = {
  padding: `5px ${AppleSpace.sm}`,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('blue'),
  background: 'transparent',
  border: `1px solid ${cv('blue')}`,
  borderRadius: AppleRadius.control,
  cursor: 'pointer',
  textDecoration: 'none',
};

// ==================== 通用卡片样式 ====================

/** 认证/安装类页面的卡片内边距（LoginView / AuthorLoginView / InstallView 共用，别再各写一份 clamp） */
export const authCardPadding = 'clamp(24px, 4vw, 40px)';

export const cardStyle = {
  background: cv('surface-panel'),
  borderRadius: AppleRadius.card,
  border: `1px solid ${cv('border-light')}`,
  boxShadow: cv('shadow-card'),
  transition: AppleTransition.smooth,
};

// ==================== 通用列表样式 ====================

export const listItemStyle = {
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  borderRadius: AppleRadius.card,
  background: cv('surface-panel'),
  transition: AppleTransition.smooth,
  border: `1px solid ${cv('border-light')}`,
};

// ============================================================
// Layout component styles (migrated from layout-styles.ts)
// ============================================================

export const layoutTabBar = {
  // 与 tabBarStyle 同一档：原先一个 24 一个 16，同类页头下沿两种留白
  marginBottom: AppleSpace.lg,
  display: 'flex',
  gap: AppleSpace.sm,
};

export const layoutStatsRow = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fill, ${AppleGridFloor.stat})`,
  gap: AppleSpace.md,
  gridAutoRows: '1fr',
  alignContent: 'start',
  marginBottom: AppleSpace.lg,
};

export const layoutContentGrid = {
  display: 'grid',
  gridTemplateColumns: '1fr',
  gap: AppleSpace.md,
};

export const layoutContentGridTwoColumn = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fit, ${AppleGridFloor.entity})`,
  gridAutoRows: '1fr',
  alignContent: 'start',
  gap: AppleSpace.md,
};

export const layoutContentGridThreeColumn = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fit, ${AppleGridFloor.stat})`,
  gridAutoRows: '1fr',
  alignContent: 'start',
  gap: AppleSpace.md,
};

export const layoutContentGridWithSidebar = {
  display: 'grid',
  gridTemplateColumns: '1fr 320px',
  gap: AppleSpace.lg,
};

export const layoutContentGridSidebarLeft = {
  display: 'grid',
  gridTemplateColumns: '280px 1fr',
  gap: AppleSpace.lg,
};

export const layoutCard = {
  background: cv('surface-panel'),
  borderRadius: AppleRadius.card,
  // 卡片内边距只有 ApCard 的四档（8/12/16/24）；xl=32 是这一处独有的第五档
  padding: AppleSpace.lg,
  border: `1px solid ${cv('border-light')}`,
  boxShadow: cv('shadow-card'),
  transition: AppleTransition.smooth,
};

export const layoutSectionHeader = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  marginBottom: AppleSpace.md,
};

export const layoutSectionHeaderLeftAligned = {
  display: 'flex',
  justifyContent: 'flex-start',
  alignItems: 'center',
  gap: AppleSpace.md,
  marginBottom: AppleSpace.md,
};

// 区块标题走 subhead(19)：linkActionHeading(21) 与页标题 h1 同档，
// 用它会让"区块标题"和"页面标题"看起来是同一级（全站 20 处区块标题实测都是 19px）。
export const layoutSectionHeaderTitle = {
  fontFamily: AppleType.subhead.font,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleType.subhead.weight),
  color: cv('text-primary'),
  margin: '0',
};

export const layoutSectionHeaderSubtitle = {
  fontFamily: AppleType.controlLabel.font,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-secondary'),
  margin: '0',
  marginTop: AppleSpace.xs,
};

export const layoutSectionHeaderActions = {
  display: 'flex',
  gap: AppleSpace.sm,
  flexWrap: 'wrap' as const,
};

export const layoutFormGroup = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
};

export const layoutFormLabel = {
  fontFamily: AppleType.controlLabel.font,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleType.controlLabel.weight),
  color: cv('text-primary'),
};

// ====== 操作卡片 ======
export const aiActionGridStyle = {
  display: 'grid', gridTemplateColumns: `repeat(auto-fit, ${AppleGridFloor.stat})`, gap: AppleSpace.md,
  gridAutoRows: '1fr', alignItems: 'stretch', alignContent: 'start',
}

export const aiActionCardStyle = {
  display: 'flex', flexDirection: 'column' as const, alignItems: 'center',
  padding: AppleSpace.md, backgroundColor: cv('surface-panel'),
  borderRadius: AppleRadius.card, border: `1px solid ${cv('border-light')}`,
  textAlign: 'center' as const, transition: AppleTransition.default,
}

export const aiActionTitleStyle = {
  fontFamily: AppleFont.text, fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary'), marginTop: AppleSpace.sm,
}

export const aiActionDescStyle = {
  fontFamily: AppleFont.text, fontSize: AppleType.microUI.size,
  color: cv('text-secondary'), marginTop: '4px', marginBottom: AppleSpace.md,
}

export const aiFormGridStyle = {
  display: 'grid', gridTemplateColumns: `repeat(auto-fit, ${AppleGridFloor.stat})`,
  gap: AppleSpace.md, marginBottom: AppleSpace.lg,
}

// ====== 结果卡片（内嵌在某个区域内的结果展示） ======
export const aiResultCardStyle = {
  marginTop: AppleSpace.lg,
  padding: AppleSpace.lg,
  backgroundColor: cv('surface-panel'),
  borderRadius: AppleRadius.medium,
  border: `1px solid ${cv('border-light')}`,
}

export const aiResultTitleStyle = {
  fontFamily: AppleFont.text, fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary'),
  marginBottom: AppleSpace.sm,
}

// ====== 区域标题 ======
export const aiSectionTitleStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  fontFamily: AppleType.subhead.font, fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary'),
  marginBottom: AppleSpace.lg,
}

// ====== 用户卡片行样式（UserProfileView / FollowView 共用） ======
export const userCardRowStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.md,
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  borderRadius: AppleRadius.medium,
  border: `1px solid ${cv('border-light')}`,
  background: cv('surface-panel'),
  transition: AppleTransition.smooth,
};

// ====== 设置页表单样式（ProfileForm / PasswordSection 共用） ======
export const settingsFormStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.md,
};

// ====== 紧凑加载样式 ======
export const compactLoadingStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
  padding: AppleSpace.lg,
};

// ============================================================
// LayoutStyles compatibility object
// Provides backward-compatible import: import { LayoutStyles } from './page-styles'
// ============================================================
export const LayoutStyles = {
  section: standardSectionStyle,
  sectionCompact: sectionStyle,
  tabBar: layoutTabBar,
  statsRow: layoutStatsRow,
  contentGrid: layoutContentGrid,
  contentGridTwoColumn: layoutContentGridTwoColumn,
  contentGridThreeColumn: layoutContentGridThreeColumn,
  contentGridWithSidebar: layoutContentGridWithSidebar,
  contentGridSidebarLeft: layoutContentGridSidebarLeft,
  card: layoutCard,
  sectionHeader: layoutSectionHeader,
  sectionHeaderLeftAligned: layoutSectionHeaderLeftAligned,
  sectionHeaderTitle: layoutSectionHeaderTitle,
  sectionHeaderSubtitle: layoutSectionHeaderSubtitle,
  sectionHeaderActions: layoutSectionHeaderActions,
  formGroup: layoutFormGroup,
  formLabel: layoutFormLabel,
} as const;

// ==================== 面包屑（单一来源） ====================
// ApPageContainer 的内置面包屑与 ApBreadcrumb 共用这一组 token。
// 此前两处各自定义：当前项一个 text-secondary、一个 text-primary，gap 一个 8px 一个 2px，
// 同一站点的两级导航看起来像两个组件。字体放在条目上（而非行容器），两个宿主都能继承到位。
export const breadcrumbRowStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  lineHeight: '1',
  minWidth: '0',
};

// 面包屑每一项都必须能收缩并省略，不能折行：文档名是用户起的，长度不由前端决定。
// 长内容抗压巡检（check:stress S3）实测原先 90 个页面在标题变长时把整条面包屑
// 撑成两行 —— 页头那一行的高度是被内容推着走的。全名靠 title 属性取回。
export const breadcrumbLinkStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.regular),
  color: cv('blue'),
  textDecoration: 'none',
  lineHeight: '1',
  display: 'inline-flex',
  alignItems: 'center',
  minWidth: '0',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
  /**
   * 命中区从 11×12 撑到 27×27（check:apple A5 实测面包屑链接是 11×12 / 24×12 / 38×12，
   * 12px 高、11px 宽的链接在触屏与轨迹板上都是点不中的那一类）。
   * 用「padding + 等值负 margin」而不是 minHeight/minWidth：后者会把整条面包屑连页头
   * 一起长高、并把短标签往两边推开，而这一版要的是**一个像素都不移动**只换命中区。
   * 负 margin 与 padding 同值 → flex 行里该条目的布局贡献净为零；`rectOf` 量的是含
   * padding 的边框盒，所以 A5 认它达标。配套把分隔线设成 pointer-events:none。
   */
  padding: AppleSpace.sm,
  margin: `-${AppleSpace.sm}`,
};

export const breadcrumbCurrentStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-secondary'),
  lineHeight: '1',
  display: 'inline-flex',
  alignItems: 'center',
  minWidth: '0',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap',
};

export const breadcrumbSepStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
  lineHeight: '1',
  display: 'inline-flex',
  alignItems: 'center',
  // 分隔线是纯装饰：链接的命中区现在横向各外扩 8px（padding + 等值负 margin），
  // 不吃点击才能保证落在重叠区时不会把"点链接"变成"什么都没发生"。
  pointerEvents: 'none' as const,
  // 分隔符不参与收缩：否则长标题会把「›」压没，面包屑读起来像少了一级
  flexShrink: '0',
};

/**
 * 单轴滚动容器：纵向可滚，横向一律截断，竖向槽位常驻。
 * 侧栏导航 / 下拉选项列表 / AI 会话列表共用这一条 —— 它们是同一个 bug 的三个现场。
 *
 * 为什么两轴必须一起交代：只写 `overflowY: 'auto'` 时，CSS 不允许一对轴里一个 visible 一个非
 * visible，另一轴会按 `auto` 处理。于是条目文字一旦顶出宽度就冒出**横向滚动条**，
 * 而横向条抢走约 15px 可视高度 → 竖向条出现/消失 → 内容宽度又回弹 → 两条一起闪
 * （首页侧栏报上来的就是这个抖）。导航列表没有"横向平移"这个动作，溢出只能截断。
 *
 * 为什么槽位要常驻：与 MainLayout.contentStyle 上那条 `scrollbarGutter: 'stable'` 同一个理由
 * —— 滚动条出现/消失会让内容左右跳，只不过这里跳的是列表自己。
 */
export const scrollSingleAxis = {
  overflowY: 'auto',
  overflowX: 'hidden',
  scrollbarGutter: 'stable',
} as const;
