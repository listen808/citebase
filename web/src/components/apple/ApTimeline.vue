<template>
  <!-- 时间线的骨架：一个有语义的有序列表。两个页面原先各自手绘「左边框 + 圆点」，
       读屏既数不出条目也分不清是活动流；收口到这里之后只剩行内容不同。 -->
  <ol :style="listStyle" :aria-label="ariaLabel">
    <slot />
  </ol>
</template>

<script setup lang="ts">
import { computed, provide } from 'vue';
import { AppleSpace } from '@/utils/core/apple-tokens';

export type ApTimelineDensity = 'default' | 'compact';

const props = withDefaults(defineProps<{
  /** 列表名。缺省由调用方给，比如「最近活动」「系统日志」 */
  ariaLabel?: string;
  density?: ApTimelineDensity;
}>(), {
  ariaLabel: '时间线',
  density: 'default',
});

provide('ap-timeline-density', props.density);

const listStyle = computed(() => ({
  listStyle: 'none' as const,
  margin: '0',
  padding: '0',
  display: 'flex' as const,
  flexDirection: 'column' as const,
  gap: props.density === 'compact' ? AppleSpace.xs : AppleSpace.md,
}));
</script>
