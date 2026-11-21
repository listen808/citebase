<template>
  <Teleport to="body">
    <div v-show="visible" :style="overlayStyle" aria-hidden="true" @click.self="onOverlayClick" />
    <div
      v-if="visible"
      ref="panelRef"
      class="apple-glass-layer"
      data-ap="drawer"
      v-bind="springAttrs"
      :style="{ ...panelStyle, ...transformStyle }"
      role="dialog"
      aria-modal="true"
      :aria-labelledby="titleId"
      tabindex="-1"
    >
      <!-- 靠内容那一侧 20px 是拖拽条：抽屉里装着长列表，整面抢手势会把纵向滚动吃掉 -->
      <div data-ap="drawer-edge" :style="edgeStyle" @pointerdown="onPointerdown" />
      <div :style="headerStyle">
        <h3 :id="titleId" :style="titleStyle">
          <slot name="title" />
        </h3>
        <ApButton variant="ghost" size="mini" icon-only :aria-label="$t('common.close')" @click="requestClose">
          <X :size="16" />
        </ApButton>
      </div>
      <div :style="bodyStyle">
        <slot />
      </div>
      <div v-if="$slots.footer" :style="footerStyle">
        <slot name="footer" />
      </div>
    </div>
  </Teleport>
</template>

<script setup lang="ts">
import { computed, onMounted, onBeforeUnmount, watch, nextTick, ref, shallowRef, useId } from 'vue';
import { X } from 'lucide-vue-next';
import { AppleSpace, cv, AppleZIndex, AppleLayout, AppleSpring } from '@/utils/core/apple-tokens';
import { useFocusTrap, useScrollLock } from '@/composables/ui/useFocusTrap';
import { useSheetGesture } from '@/composables/ui/useSheetGesture';
import { getOverlayStyle } from '@/styles/component/overlay';
import { dialogHeaderStyle, dialogTitleStyle, dialogFooterStyle } from '@/styles/component/dialog';
import ApButton from '@/components/apple/ApButton.vue';

const props = withDefaults(
  defineProps<{
    modelValue: boolean;
    width?: string;
    placement?: 'left' | 'right';
    closeOnOverlay?: boolean;
  }>(),
  {
    width: AppleLayout.drawerWidth,
    placement: 'right',
    closeOnOverlay: true,
  },
);

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void;
}>();

const panelRef = ref<HTMLElement | null>(null);
/** 真的挂在 DOM 上没有：父级的 modelValue 说要关，还要等落位动画跑完 */
const visible = shallowRef(false);
const drawerId = useId();
const titleId = `ap-drawer-title-${drawerId}`;

const { setupFocusTrap, handleTabKey, saveActiveElement, restoreActiveElement } = useFocusTrap(panelRef);
const { lock: lockScroll, unlock: unlockScroll } = useScrollLock();

const gesture = useSheetGesture({
  axis: 'x',
  // 左抽屉朝屏幕左划是 dismiss（dx 为负），右抽屉相反
  direction: props.placement === 'left' ? -1 : 1,
  travel: () => panelRef.value?.offsetWidth || 0,
  onDismiss: () => {
    visible.value = false;
    // 被划走时父级还留着 modelValue=true：不同步 emit 出去，下一次"打开"就没有 watch 可触发
    if (props.modelValue) emit('update:modelValue', false);
  },
  spec: AppleSpring.sheet,
});
const { onPointerdown, offset, transformStyle, springAttrs, enter, leave } = gesture;

watch(() => props.modelValue, (open) => {
  if (open) {
    visible.value = true;
    lockScroll();
    saveActiveElement();
    nextTick(() => {
      setupFocusTrap();
      // 首帧摆到屏外再回到 0；减弱动效时 enter() 直接落位
      enter();
    });
  } else if (visible.value) {
    unlockScroll();
    restoreActiveElement();
    leave();
  } else {
    unlockScroll();
    restoreActiveElement();
  }
}, { immediate: true });

function requestClose() {
  emit('update:modelValue', false);
}

function onOverlayClick() {
  if (props.closeOnOverlay) {
    requestClose();
  }
}

/** 遮罩和面板共用同一个 offset：拖到一半，遮罩就只剩一半的量，不存在"面板跟手、背景等动画" */
const overlayStyle = computed(() => ({
  ...getOverlayStyle(AppleZIndex.drawer),
  opacity: String(Math.max(0, 1 - offset.value / (panelRef.value?.offsetWidth || 1))),
}));

const panelStyle = computed(() => ({
  position: 'fixed' as const,
  top: '0',
  [props.placement]: '0',
  width: props.width,
  maxWidth: '90vw',
  height: '100dvh',
  // 抽屉是浮层档，不是卡片档。原先这里取 surface-panel（= ApCard 那一档），
  // 于是抽屉摊开时和它底下的卡片**同一个颜色**，台阶断了，只剩阴影在硬撑；
  // 而下面那行注释与 SpaceView 自己的移动端抽屉都已经按"浮层=elevated"在写了。
  background: cv('surface-float'),
  // 模糊由模板上的 .apple-glass-layer 伪元素承载（入场一起到位），行内不重复铺，
  // 否则与伪元素叠成两层 backdrop-filter：白花一次 GPU，还吃掉"玻璃层 ≤3"的预算。
  boxShadow: cv('shadow-modal'), // 抽屉与模态同级，阴影也应取 modal 档
  zIndex: String(AppleZIndex.drawer),
  display: 'flex',
  flexDirection: 'column' as const,
}));

const edgeStyle = computed(() => ({
  position: 'absolute' as const,
  top: '0',
  bottom: '0',
  [props.placement === 'left' ? 'right' : 'left']: '0',
  width: AppleSpace.md,
  cursor: 'grab',
  // 不写这句，浏览器会把横拖当成滚页面，手势永远等不到第二个 pointermove
  touchAction: 'none',
  zIndex: String(AppleZIndex.raised),
}));

const headerStyle = dialogHeaderStyle(AppleSpace.md);

const titleStyle = dialogTitleStyle();

const bodyStyle = {
  flex: '1',
  overflowY: 'auto' as const,
  padding: AppleSpace.md,
  minHeight: '0',
};

const footerStyle = dialogFooterStyle(AppleSpace.md);

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && props.modelValue) {
    e.preventDefault();
    requestClose();
  } else if (props.modelValue) {
    handleTabKey(e);
  }
}

onMounted(() => {
  document.addEventListener('keydown', handleKeydown);
});

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeydown);
  unlockScroll();
  restoreActiveElement();
});
</script>
