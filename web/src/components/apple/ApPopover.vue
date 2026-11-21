<template>
  <div ref="triggerRef" :style="wrapperStyle">
    <div
      :style="triggerStyle"
      :aria-expanded="internalOpen"
      :aria-haspopup="true"
      :aria-label="label || undefined"
      @click="toggle"
      @mouseenter="trigger === 'hover' && show()"
      @mouseleave="trigger === 'hover' && hide()"
    >
      <slot />
    </div>
    <teleport to="body">
      <transition name="apple-popover">
        <div v-if="internalOpen" ref="contentRef" class="apple-glass-layer" :style="popoverStyle" role="tooltip">
          <div v-if="$slots.title" :style="titleStyle">
            <slot name="title" />
          </div>
          <div :style="bodyStyle">
            <slot name="content" />
          </div>
        </div>
      </transition>
    </teleport>
  </div>
</template>

<script setup lang="ts">
import { ref, shallowRef, computed, onMounted, onBeforeUnmount, nextTick, watch } from 'vue';
import { AppleSpace, AppleType, cv, menuSurfaceStyle } from '@/utils/core/apple-tokens';
import { calcPosition } from '@/utils/core/position';
import { useClickOutsideClose } from '@/composables/ui/useClickOutsideClose';

const props = withDefaults(
  defineProps<{
    placement?: 'top' | 'bottom' | 'left' | 'right';
    trigger?: 'click' | 'hover';
    width?: string;
    /** 受控模式：外部控制开关 */
    open?: boolean;
    /** 触发器只放图标时用它给个可读名字；带文字内容时可省略 */
    label?: string;
  }>(),
  { placement: 'bottom', trigger: 'click', width: '240px', open: undefined, label: '' },
);

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void;
}>();

const internalOpen = shallowRef(false);
const positionReady = shallowRef(false);

watch(() => props.open, (val) => {
  if (val !== undefined && val !== internalOpen.value) {
    if (val) {
      show();
    } else {
      hide();
    }
  }
});
const triggerRef = ref<HTMLElement | null>(null);
const contentRef = ref<HTMLElement | null>(null);
const position = ref({ top: '-9999px', left: '-9999px' });

function toggle() {
  if (props.trigger === 'click') {
    if (!internalOpen.value) {
      internalOpen.value = true;
      positionReady.value = false;
      nextTick().then(() => {
        updatePosition();
        positionReady.value = true;
      });
    } else {
      internalOpen.value = false;
      positionReady.value = false;
    }
    emit('update:open', internalOpen.value);
  }
}

function show() {
  if (internalOpen.value) return;
  internalOpen.value = true;
  positionReady.value = false;
  nextTick().then(() => {
    updatePosition();
    positionReady.value = true;
  });
  emit('update:open', true);
}

function hide() {
  if (!internalOpen.value) return;
  internalOpen.value = false;
  positionReady.value = false;
  emit('update:open', false);
}

function close(e?: MouseEvent) {
  if (e && contentRef.value && contentRef.value.contains(e.target as Node)) {
    return;
  }
  internalOpen.value = false;
  positionReady.value = false;
  emit('update:open', false);
}

function updatePosition() {
  if (!triggerRef.value || !contentRef.value) return;
  const rect = triggerRef.value.getBoundingClientRect();
  const contentRect = contentRef.value.getBoundingClientRect();
  position.value = calcPosition(rect, contentRect, props.placement, 8, 8);
}

// 面板是 position:fixed，滚动或缩放后必须重算，否则会与触发器脱离
// 内容宽也会晚于首次定位才稳定（图标/字体异步加载把 max-content 撑宽）——
// 尺寸一变就重算，否则 left 还是按旧宽度夹取的，右缘会溢出视口（实测超 4px）。
let contentResizeObserver: ResizeObserver | null = null;
watch(internalOpen, (open) => {
  if (open) {
    document.addEventListener('scroll', updatePosition, true);
    window.addEventListener('resize', updatePosition);
    // 面板（Teleport）要 nextTick 后才挂载，此刻 contentRef 还是 null ——
    // RO 必须在挂载后挂，否则打开瞬间量到的旧宽度永远不会被重算纠正。
    nextTick().then(() => {
      if (!contentRef.value || contentResizeObserver) return;
      contentResizeObserver = new ResizeObserver(() => updatePosition());
      contentResizeObserver.observe(contentRef.value);
    });
  } else {
    document.removeEventListener('scroll', updatePosition, true);
    window.removeEventListener('resize', updatePosition);
    contentResizeObserver?.disconnect();
    contentResizeObserver = null;
  }
});

function handleKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && internalOpen.value) {
    close();
  }
}

// 点别处收起走全站唯一实现（原来这里自己挂了一份 click 冒泡版）：
// 面板是 Teleport 到 body 的，所以"自己人"必须同时给触发器与面板两个元素。
useClickOutsideClose(
  () => internalOpen.value,
  () => close(),
  () => [triggerRef.value, contentRef.value],
);

onMounted(() => {
  document.addEventListener('keydown', handleKeydown);
});

onBeforeUnmount(() => {
  document.removeEventListener('keydown', handleKeydown);
  document.removeEventListener('scroll', updatePosition, true);
  window.removeEventListener('resize', updatePosition);
  contentResizeObserver?.disconnect();
  contentResizeObserver = null;
});

const wrapperStyle = { display: 'inline-flex' };

const triggerStyle = { display: 'inline-flex', cursor: 'pointer' as const };

const popoverStyle = computed(() => {
  const style: Record<string, string> = menuSurfaceStyle({
    position: 'fixed',
    top: position.value.top,
    left: position.value.left,
    // 缩放的起点必须是"靠触发元素那一条边"：默认 center 会让面板最近触发点的那条边
    // 也跟着往外退，展开的一瞬间它和触发它的那个按钮就断开了（§7 空间一致性）。
    transformOrigin: { bottom: 'top center', top: 'bottom center', left: 'right center', right: 'left center' }[props.placement] || 'top center',
    width: props.width,
    // CSS 层兜底：ResizeObserver 重算前的一帧内内容再变宽也不许顶出视口
    maxWidth: `calc(100vw - ${AppleSpace.md})`,
    overflow: 'hidden',
    visibility: positionReady.value ? 'visible' : 'hidden',
  });

  return style;
});

const titleStyle = {
  padding: `${AppleSpace.md} ${AppleSpace.md}`,
  borderBottom: `1px solid ${cv('border-light')}`,
  fontFamily: AppleType.bodyEmphasis.font,
  fontSize: AppleType.bodyEmphasis.size,
  fontWeight: String(AppleType.bodyEmphasis.weight),
  color: cv('text-primary'),
};

const bodyStyle = {
  padding: AppleSpace.md,
};
</script>

<style scoped>
/* Apple HIG 风格 Popover 展开动效 */
.apple-popover-enter-active {
  transition: opacity var(--apple-dur-interactive) var(--apple-easing-emphasized),
              transform var(--apple-dur-default) var(--apple-easing-emphasized);
}
.apple-popover-leave-active {
  /* 0.12s 不在 AppleTransition 任何一档上（浮层出场统一走 0.15s = fast 档） */
  transition: opacity var(--apple-dur-fast) var(--apple-easing-standard),
              transform var(--apple-dur-fast) var(--apple-easing-standard);
}
.apple-popover-enter-from {
  opacity: 0;
  transform: scale(0.95) translateY(-4px);
}
.apple-popover-leave-to {
  opacity: 0;
  transform: scale(0.97);
}
</style>
