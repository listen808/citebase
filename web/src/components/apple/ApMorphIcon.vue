<script setup lang="ts">
import { computed } from 'vue';
import { MorphIcon, type IconInput, type ReducedMotionMode, type SpringPreset } from 'morphicons/vue';
import { useUIStore } from '@/stores/app/ui';

/**
 * 图标形变的唯一入口。morphicons 消费的是 lucide 的**数据对象**（IconNode=[tag,attrs][]），
 * 不是 lucide-vue-next 组件，所以调用方传 `:icon="state ? IconA : IconB"`（A/B 从 @/components/apple/morph-icon-data 取自维护，不依赖 npm 包 lucide）。
 * color 默认 currentColor 让图标跟随文字色、不硬编码。
 *
 * **2026-10-16 起 reducedMotion 不再默认 'user'**：'user' 只认系统的 prefers-reduced-motion，
 * 而个性化加了应用内档位（动效强度 full/soft/reduced + 系统信号一起解算）。
 * 解算结果已经由 store 给出生效档，所以这里传 'always' / 'never' —— 继续传 'user'
 * 会出现"偏好页选了减弱、图标还在形变"的半接线。
 */
const props = withDefaults(defineProps<{
  icon: IconInput;
  size?: number | string;
  color?: string;
  strokeWidth?: number | string;
  absoluteStrokeWidth?: boolean;
  spring?: SpringPreset;
  /** 显式传入优先于当前生效档（少数页面要反过来用，例如刻意保留形变的营销版面） */
  reducedMotion?: ReducedMotionMode;
  label?: string;
}>(), {
  size: 24,
  color: 'currentColor',
  absoluteStrokeWidth: false,
  spring: 'snappy',
  reducedMotion: undefined,
});

const ui = useUIStore();
const resolvedReducedMotion = computed<ReducedMotionMode>(
  () => props.reducedMotion ?? (ui.motionEffective === 'reduced' ? 'always' : 'never'),
);
</script>

<template>
  <MorphIcon v-bind="$props" :reduced-motion="resolvedReducedMotion" />
</template>
