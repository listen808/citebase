<template>
  <template v-if="tocItems.length > 0">
    <!-- 面板：从右下角按钮左侧弹出 -->
    <Transition name="toc-slide">
      <div v-if="!collapsed" ref="panelRef" class="apple-glass-layer" :style="panelStyle">
        <div :style="headerStyle">
          <span :style="titleStyle">{{ $t('spaceUi.k48') }}</span>
          <span :style="countStyle">{{ tocItems.length }}</span>
        </div>
        <div :style="listStyle">
          <div
            v-for="item in tocItems"
            :key="item.id"
            :ref="(el) => setItemRef(item.id, el as HTMLElement)"
            :style="getItemStyle(item)"
            @click="scrollToHeading(item.id)"
          >
            {{ item.text }}
          </div>
        </div>
      </div>
    </Transition>

    <!-- 固定切换按钮 -->
    <ApFloatingActionButton :label="$t('spaceUi.k48')" variant="secondary" bottom="100px" right="20px" @click="collapsed = !collapsed">
      <ApIcon name="List" :size="14" />
    </ApFloatingActionButton>
  </template>
</template>

<script setup lang="ts">
import { shallowRef, watch, ref } from 'vue';
import ApFloatingActionButton from '@/components/apple/ApFloatingActionButton.vue';
import { AppleFont, AppleFontWeight, AppleLayout, AppleLineHeight, AppleRadius, AppleSpace, AppleTransition, AppleType, cv, menuSurfaceStyle } from '@/utils/core/apple-tokens';
import { useToc, type TocItem } from '@/composables/space/useToc';

const props = defineProps<{
  contentRef: () => HTMLElement | null;
  previewRef: () => { headings?: TocItem[] } | null;
  currentContent: () => string;
}>();

const { tocItems, activeHeadingId, scrollToHeading } = useToc(
  props.contentRef,
  props.previewRef,
  props.currentContent,
);

const collapsed = shallowRef(true);
const panelRef = ref<HTMLElement | null>(null);
const itemRefs = new Map<string, HTMLElement>();

function setItemRef(id: string, el: HTMLElement | null) {
  if (el) itemRefs.set(id, el);
  else itemRefs.delete(id);
}

// 活动标题切换时，自动滚动 TOC 面板使其可见
watch(activeHeadingId, (id) => {
  if (!id || collapsed.value || !panelRef.value) return;
  const el = itemRefs.get(id);
  if (!el) return;
  const panel = panelRef.value;
  const panelRect = panel.getBoundingClientRect();
  const elRect = el.getBoundingClientRect();
  if (elRect.bottom > panelRect.bottom - 4 || elRect.top < panelRect.top + 4) {
    el.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  }
});

// 与同一页的「回到顶部」悬浮按钮共用同一把尺（AppleLayout.touchTargetMin）。
// 原先这里是私有常数 36，两个浮在右下角的圆形按钮一个 44 一个 36，
// 挨在一起时大小不一眼就能看出来 —— 逐组件几何普查（check:boxes B1）报的就是它。
const BTN_SIZE = parseInt(AppleLayout.touchTargetMin, 10);
const BTN_RIGHT = 20;
const BTN_BOTTOM = 100;
const PANEL_WIDTH = 220;
const PANEL_GAP = 8;

// 表面走全站唯一的浮层实现（menuSurfaceStyle）：原先这里自己抄了一份 elevated + border-light
// + shadow-card，正是 G23 拦的那种"第二套菜单表面"—— 浮层档换色、玻璃边缘厚度这些材质收口
// 都轮不到它。位置与尺寸是本组件自己的事，留在 extra 里。
const panelStyle = {
  ...menuSurfaceStyle({
    position: 'fixed',
    right: `${BTN_RIGHT + BTN_SIZE + PANEL_GAP}px`,
    bottom: `${BTN_BOTTOM}px`,
    width: `${PANEL_WIDTH}px`,
    maxHeight: '50vh',
    // 玻璃壳自己不滚：::before 绝对定位的模糊/亮边层会跟着内容滚出去，
    // 滚动后看起来像两块面板上下叠着（深色主题尤其明显）。滚动交给内层列表。
    display: 'flex',
    flexDirection: 'column' as const,
    overflow: 'hidden',
    padding: AppleSpace.md,
  }),
  // 这是一块"用户主动叫出来的浮板"，与 ApPopover / 下拉菜单同一族，取 dropdown 档。
  // 原先取 raised(10)：比壳层 overlay(100) 还低两级，会被顶栏压住；
  // 而移动端同角色的 MobileTocFab 在 drawer 档 —— 同一个角色两端差 999 倍。
};

const headerStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  marginBottom: AppleSpace.xs,
  padding: `0 ${AppleSpace.xs}`,
};

const titleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
};

const countStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};

const listStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
  flex: '1 1 auto',
  minHeight: 0,
  overflowY: 'auto' as const,
};

function getItemStyle(item: TocItem) {
  const isActive = activeHeadingId.value === item.id;
  return {
    fontFamily: AppleFont.text,
    fontSize: AppleType.microUI.size,
    lineHeight: AppleLineHeight.loose,
    color: isActive ? cv('blue') : cv('text-tertiary'),
    cursor: 'pointer',
    flexShrink: 0,
    padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
    paddingLeft: `${8 + (item.level - 1) * 12}px`,
    borderRadius: AppleRadius.micro,
    transition: AppleTransition.fast,
    background: isActive ? cv('blue-selection') : 'transparent',
    fontWeight: isActive ? String(AppleFontWeight.semibold) : String(AppleFontWeight.regular),
    whiteSpace: 'nowrap' as const,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  };
}
</script>

<style scoped>
.toc-slide-enter-active,
.toc-slide-leave-active {
  /* 进出场只改 opacity + transform，点名它们而不是 all */
  transition: opacity var(--apple-dur-interactive) var(--apple-easing-standard), transform var(--apple-dur-interactive) var(--apple-easing-standard);
}
.toc-slide-enter-from,
.toc-slide-leave-to {
  opacity: 0;
  transform: translateX(12px);
}
</style>
