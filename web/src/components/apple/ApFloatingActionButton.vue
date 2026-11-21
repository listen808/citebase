<template>
  <ApButton
    :variant="variant"
    size="mini"
    icon-only
    :aria-label="label"
    :style="fabStyle"
    @mouseenter="hovered = true"
    @mouseleave="hovered = false; pressed = false"
    @mousedown="pressed = true"
    @mouseup="pressed = false"
    @click="onClick"
  >
    <slot />
  </ApButton>
</template>

<script setup lang="ts">
import { computed, shallowRef } from 'vue';
import ApButton from '@/components/apple/ApButton.vue';
import { AppleLayout, AppleRadius, AppleTransition, AppleZIndex, cv } from '@/utils/core/apple-tokens';

/**
 * 全站唯一的浮动操作按钮（FAB）：固定右下角的 44×44 圆形按钮。
 * 把原先散在 DocContentPanel(回到顶部) / MobileTocFab(目录) / FloatingToc 里的
 * 同一套 position:fixed + circle + shadow-card + z:raised 收在一处，
 * 并统一 hover 提亮 + 按下缩放的手势反馈（原先只有 MobileTocFab 手写了这套，另两处没有）。
 */
const props = withDefaults(defineProps<{
  label: string;
  variant?: 'primary' | 'ghost' | 'secondary';
  bottom?: string;
  right?: string;
}>(), {
  variant: 'primary',
  bottom: '24px',
  right: '24px',
});

const emit = defineEmits<{ (e: 'click'): void }>();

const hovered = shallowRef(false);
const pressed = shallowRef(false);

const onClick = () => emit('click');

const fabStyle = computed(() => ({
  position: 'fixed' as const,
  bottom: props.bottom,
  right: props.right,
  width: AppleLayout.touchTargetMin,
  height: AppleLayout.touchTargetMin,
  borderRadius: AppleRadius.circle,
  boxShadow: cv('shadow-card'),
  // banner(500)：高于底栏 overlay(sticky 49)，否则会被页脚遮住；低于下拉(1000)/弹窗(9999)。
  zIndex: String(AppleZIndex.banner),
  transition: AppleTransition.default,
  cursor: 'pointer',
  filter: hovered.value ? 'brightness(1.1)' : 'none',
  transform: pressed.value ? 'scale(0.95)' : 'scale(1)',
}));
</script>
