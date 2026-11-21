<script setup lang="ts">
import { computed } from 'vue';
import { icons, type IconName } from '@/utils/icons';
import { AppleIconSize, iconStrokeFor } from '@/utils/core/apple-tokens';

const props = defineProps<{
  name: IconName;
  /**
   * 尺寸优先传 `AppleIconSize` 的档位。历史调用点传裸数字照旧可用，
   * 但那些数字由门禁 I6 记在「只准降不准升」的基线里，新代码不该再写。
   */
  size?: number;
  color?: string;
  /**
   * 显式描边宽度（viewBox 单位）。留空时由 `iconStrokeFor(size)` 按尺寸反解 ——
   * 这一行是整个图标族的强制点：lucide 的 stroke-width 是 viewBox 单位、随尺寸等比放大，
   * 不接就出现「16px 画 1.33px 线、48px 画 4px 线」，看着像两套图标。
   */
  strokeWidth?: number;
}>();

// 默认 20 = listLead 档：沿用改造前写死的 20，未显式传 size 的调用点才不会集体变小
const resolvedSize = computed(() => props.size ?? AppleIconSize.listLead);
const resolvedStroke = computed(() => props.strokeWidth ?? iconStrokeFor(resolvedSize.value));
</script>

<template>
  <component :is="icons[name]" :size="resolvedSize" :color="color" :stroke-width="resolvedStroke" />
</template>
