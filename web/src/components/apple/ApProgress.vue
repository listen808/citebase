<template>
  <div
    :style="trackStyle"
    role="progressbar"
    :aria-label="label"
    aria-valuemin="0"
    :aria-valuemax="max"
    :aria-valuenow="rounded"
  >
    <div :style="fillStyle" />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { AppleRadius, AppleSpace, AppleTiming, AppleZIndex, cv } from '@/utils/core/apple-tokens';
import { toneColor, type ApTone } from '@/utils/core/progress-tone';

const props = withDefaults(defineProps<{
  value: number;
  label: string;
  max?: number;
  size?: 'hair' | 'thin' | 'bar';
  tone?: ApTone;
  edge?: boolean;
}>(), {
  max: 100,
  size: 'thin',
  tone: 'accent',
  edge: false,
});

// 三档都取自 AppleSpace 的纵向尺（2/4/8），不再出现 3px、6px 这种自造高度。
const HEIGHTS = { hair: AppleSpace.xxs, thin: AppleSpace.xs, bar: AppleSpace.sm };

const ratio = computed(() => Math.min(1, Math.max(0, props.value / props.max)));
const rounded = computed(() => Math.round(props.value));

const trackStyle = computed(() => ({
  height: HEIGHTS[props.size],
  backgroundColor: props.edge ? 'transparent' : cv('bg-tertiary'),
  borderRadius: props.edge ? '0' : AppleRadius.pill,
  overflow: 'hidden' as const,
  ...(props.edge
    ? {
        position: 'fixed' as const,
        top: '0',
        left: '0',
        right: '0',
        zIndex: String(AppleZIndex.banner),
        pointerEvents: 'none' as const,
      }
    : {}),
}));

// 进度值是全站最容易"每帧都在变"的宽度：动 width 就是每帧一次重排，
// 所以整条铺满后用 scaleX 裁，只动合成器属性（与 G17 同一条口径）。
const fillStyle = computed(() => ({
  height: '100%',
  width: '100%',
  transformOrigin: 'left center',
  transform: `scaleX(${ratio.value})`,
  backgroundColor: toneColor(props.tone),
  transition: `transform ${AppleTiming.smooth}`,
}));
</script>
