import { computed, type ShallowRef } from 'vue';
import { AppleAnimation, AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleTiming, AppleTransition, AppleType, cv } from '@/utils/core/apple-tokens';

export function getSpaceIconStyle() {
  return computed(() => ({
    background: cv('bg-tertiary'),
    width: AppleSpace.xl, height: AppleSpace.xl, borderRadius: AppleRadius.small,
    display: 'flex', alignItems: 'center', justifyContent: 'center',
    color: cv('text-secondary'), fontFamily: AppleFont.text, fontSize: AppleType.controlLabel.size,
    fontWeight: String(AppleFontWeight.semibold), flexShrink: 0,
  }));
}

export const sidebarStyle = (isMobile: boolean, sidebarWidth: ShallowRef<number>, collapsed?: boolean) => computed(() => {
  // 文档树这一列与**应用侧栏是同一个角色**（固定在正文左侧的壳层），所以走同一档：
  // chrome 底色 + 玻璃发丝边 + 静止档模糊。此前它铺 surface-panel + 1px 实线，
  // 于是同一屏里"侧栏"有两种材质（内容板 vs 玻璃），是普查点名的头号不统一。
  // 朝向内容那条边上的滑光亮段由模板上的 `data-glass-surface` 提供（见 DocSidebar.vue）。
  const glass = {
    borderRight: `0.5px solid ${cv('glass-rim')}`,
    background: cv('surface-chrome'),
    backdropFilter: cv('blur-panel'),
    WebkitBackdropFilter: cv('blur-panel'),
  };
  if (collapsed) {
    return {
      width: AppleSpace['2xl'], minWidth: AppleSpace['2xl'], flexShrink: 0,
      ...glass,
      display: 'flex', flexDirection: 'column' as const, position: 'relative' as const,
      overflow: 'hidden',
    };
  }
  const baseWidth = isMobile ? '80%' : `${sidebarWidth.value}px`;
  const maxWidth = isMobile ? '320px' : '500px';
  return {
    width: baseWidth, maxWidth, flexShrink: 0,
    ...glass,
    display: 'flex', flexDirection: 'column' as const, position: 'relative' as const,
  };
});

export const headerStyle = {
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`, borderBottom: `1px solid ${cv('border-light')}`,
  flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'space-between',
  gap: AppleSpace.xs, minWidth: 0, overflow: 'hidden' as const,
  height: '48px',
};

export const headerLeftStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.xs, minWidth: 0, flex: 1 };

export const headerTitleWrapStyle = { display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xxs, minWidth: 0, overflow: 'hidden' as const };

export const titleStyle = {
  fontFamily: AppleType.bodyEmphasis.font, fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleType.bodyEmphasis.weight), color: cv('text-primary'),
  overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const,
};

export const docCountStyle = { fontFamily: AppleFont.text, fontSize: AppleType.microUI.size, color: cv('text-quaternary') };

export const actionsStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.xxs, flexShrink: 0, overflow: 'hidden' as const };

export const searchStyle = { padding: AppleSpace.sm, borderBottom: `1px solid ${cv('border-light')}`, overflow: 'hidden' as const };

export const bodyStyle = { flex: '1', overflow: 'auto', padding: `${AppleSpace.xs} 0`, display: 'flex', flexDirection: 'column' as const };

export const segmentedWrapStyle = { padding: `${AppleSpace.xs} ${AppleSpace.sm}`, display: 'flex', justifyContent: 'center' as const };

export const treeSkeletonStyle = {
  padding: `${AppleSpace.sm} ${AppleSpace.xs}`, display: 'flex',
  flexDirection: 'column' as const, gap: AppleSpace.xxs,
};

export function treeSkeletonItemStyle(index: number) {
  return {
    display: 'flex', alignItems: 'center', gap: AppleSpace.xs,
    padding: `${AppleSpace.xs} ${AppleSpace.sm} ${AppleSpace.xs} ${AppleSpace.xs}`,
    marginLeft: `${(index % 3) * 16}px`,
  };
}

export function skeletonLineStyle(widthPercent: number) {
  return {
    width: `${widthPercent}%`, height: '14px', borderRadius: AppleRadius.micro,
    background: `linear-gradient(90deg, ${cv('border-light')} 25%, ${cv('bg-hover')} 50%, ${cv('border-light')} 75%)`,
    backgroundSize: '200% 100%', animation: `${AppleAnimation.shimmer} ${AppleTiming.skeletonSweep} infinite`,
  };
}

export const collapsedHeaderBarStyle = {
  padding: `${AppleSpace.xs} 0`, borderBottom: `1px solid ${cv('border-light')}`,
  flexShrink: 0, display: 'flex', alignItems: 'center', justifyContent: 'center',
  minWidth: 0, overflow: 'hidden' as const,
  height: '48px',
};

export const collapsedHeaderStyle = {
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  width: '100%', flexShrink: 0,
};

export const collapsedBodyStyle = {
  flex: '1', display: 'flex', flexDirection: 'column' as const,
  alignItems: 'center', padding: `${AppleSpace.xs} 0`,
};

export const flatListStyle = {
  display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xxs,
  padding: `${AppleSpace.xxs} 0`,
};

export const flatItemStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace.xs,
  height: '26px',
  padding: `0 ${AppleSpace.xxs} 0 ${AppleSpace.xs}`,
  margin: `0 ${AppleSpace.xs}`,
  borderRadius: AppleRadius.small, cursor: 'pointer', transition: AppleTransition.fast,
};

export const flatNameStyle = {
  fontFamily: AppleFont.text, fontSize: AppleType.controlLabel.size,
  color: cv('text-primary'), overflow: 'hidden', textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const, flex: 1, marginLeft: AppleSpace.xs,
};

export const flatSubStyle = {
  fontFamily: AppleFont.text, fontSize: AppleType.microUI.size, color: cv('text-quaternary'), flexShrink: 0,
};

export const footerStyle = {
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  borderTop: `1px solid ${cv('border-light')}`,
  flexShrink: 0,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
};

export const resizeHandleStyle = {
  position: 'absolute' as const, right: 0, top: 0, bottom: 0,
  width: '6px', cursor: 'col-resize', background: 'transparent', transition: AppleTransition.fast,
};
