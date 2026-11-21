<template>
  <ApFloatingActionButton :label="$t('spaceUi.k48')" variant="primary" @click="openSheet">
    <ApIcon name="List" :size="14" />
  </ApFloatingActionButton>

  <Teleport to="body">
    <div
      v-show="showDrawer"
      :style="maskStyle"
      aria-hidden="true"
      @click="closeSheet"
    />
    <div
      v-show="showDrawer"
      ref="sheetRef"
      v-bind="springAttrs"
      class="apple-glass-layer"
      :style="{ ...drawerStyle, ...transformStyle }"
      role="dialog"
      aria-modal="true"
      :aria-label="$t('spaceUi.k62')"
    >
      <!-- 把手条 + 页眉是唯一的拖拽区：正文列表要继续能纵向滚，手势不能抢它 -->
      <div
        data-ap="sheet-grabber"
        :style="grabberStyle"
        @pointerdown="onPointerdown"
      >
        <span :style="grabberPillStyle" />
      </div>
      <div :style="drawerHeaderStyle" @pointerdown="onPointerdown">
        <span :style="drawerTitleStyle">{{ $t('spaceUi.k48') }}</span>
        <span :style="countStyle">{{ tocItems.length }} {{ $t('cExtraUi.k10') }}</span>
        <ApButton variant="ghost" size="mini" icon-only :aria-label="$t('spaceUi.k63')" @click="closeSheet">
          <ApIcon name="X" :size="16" />
        </ApButton>
      </div>
      <div :style="drawerBodyStyle">
        <div
          v-for="item in tocItems"
          :key="item.id"
          :style="getItemStyle(item)"
          @click="handleItemClick(item.id)"
        >
          {{ item.text }}
        </div>
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, nextTick, shallowRef } from 'vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApFloatingActionButton from '@/components/apple/ApFloatingActionButton.vue';
import { AppleFont, AppleFontWeight, AppleLineHeight, AppleRadius, AppleSpace, AppleSpring, AppleTransition, AppleType, AppleZIndex, cv } from '@/utils/core/apple-tokens';
import { useToc, type TocItem } from '@/composables/space/useToc';
import { useSheetGesture } from '@/composables/ui/useSheetGesture';
import { getOverlayStyle } from '@/styles/component/overlay';
import { useEscapeClose } from '@/composables/ui/useEscapeClose';

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

const showDrawer = shallowRef(false);
const sheetRef = shallowRef<HTMLElement | null>(null);

/** 满程 = sheet 自己的高度；现读，标题数变了、横竖屏换了都跟着走 */
const travel = () => sheetRef.value?.offsetHeight || 0;

const gesture = useSheetGesture({
  axis: 'y',
  travel,
  onDismiss: () => { showDrawer.value = false; },
  spec: AppleSpring.sheet,
});
const { offset, transformStyle, springAttrs, enter, leave, onPointerdown } = gesture;

/**
 * 遮罩的透明度直接挂在同一个 `offset` 上：开、拖、关三条路径共用一套物理，
 * 不另外铺 CSS transition —— 弹簧每帧都在写透明度，再加一层过渡就是双重动画。
 */
// 带遮罩的底部面板就是**抽屉**，与 ApDrawer 同一档、同一写法（遮罩与面板同档，靠 DOM 次序分先后）。
// 原先写 `modal - 1` / `modal`：那是自造的档位，而且让它压过了真正的抽屉（drawer 9998）。
const maskStyle = computed(() => getOverlayStyle(AppleZIndex.drawer, {
  opacity: String(Math.max(0, 1 - offset.value / (travel() || 1))),
}));

function openSheet() {
  showDrawer.value = true;
  nextTick(() => { enter(); });
}

function closeSheet() {
  leave();
}

// 目录 sheet 自己画的（不是 ApDrawer），Esc 走 useEscapeClose；关也走同一条动画
useEscapeClose(() => showDrawer.value, closeSheet);

function handleItemClick(id: string) {
  scrollToHeading(id);
  closeSheet();
}

const drawerStyle = {
  position: 'fixed' as const,
  bottom: '0',
  left: '0',
  right: '0',
  maxHeight: '70vh',
  background: cv('surface-float'),
  borderRadius: `${AppleRadius.card} ${AppleRadius.card} 0 0`,
  boxShadow: cv('shadow-modal'), // 抽屉与模态同级，阴影取 modal 档（与 ApDrawer 一致）
  zIndex: String(AppleZIndex.drawer),
  display: 'flex',
  flexDirection: 'column' as const,
  overflow: 'hidden',
};

const grabberStyle = {
  padding: `${AppleSpace.sm} ${AppleSpace.lg} 0`,
  display: 'flex',
  justifyContent: 'center',
  cursor: 'grab',
  // 不写这句，浏览器会把"向下拖"当成滚页面，手势永远等不到第二个 pointermove
  touchAction: 'none',
};

const grabberPillStyle = {
  width: AppleSpace.xl,
  height: AppleSpace.xs,
  borderRadius: AppleRadius.pill,
  background: cv('border'),
  display: 'block',
};

const drawerHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  padding: `${AppleSpace.md} ${AppleSpace.lg}`,
  borderBottom: `1px solid ${cv('border-light')}`,
  flexShrink: 0,
  cursor: 'grab',
  touchAction: 'none',
};

const drawerTitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  flex: '1',
};

const countStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
  marginRight: AppleSpace.sm,
};

const drawerBodyStyle = {
  flex: '1',
  overflowY: 'auto' as const,
  padding: AppleSpace.md,
  touchAction: 'pan-y',
};

function getItemStyle(item: TocItem) {
  const isActive = activeHeadingId.value === item.id;
  return {
    fontFamily: AppleFont.text,
    fontSize: AppleType.controlLabel.size,
    lineHeight: AppleLineHeight.loose,
    color: isActive ? cv('blue') : cv('text-secondary'),
    cursor: 'pointer',
    padding: `${AppleSpace.sm} ${AppleSpace.md}`,
    paddingLeft: `${12 + (item.level - 1) * 16}px`,
    borderRadius: AppleRadius.small,
    transition: AppleTransition.fast,
    background: isActive ? cv('blue-selection') : 'transparent',
    fontWeight: isActive ? String(AppleFontWeight.semibold) : String(AppleFontWeight.regular),
    whiteSpace: 'nowrap' as const,
    overflow: 'hidden',
    textOverflow: 'ellipsis',
  };
}
</script>
