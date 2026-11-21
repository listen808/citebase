<template>
  <li :style="itemStyle" class="ap-timeline-item">
    <span :style="railStyle" aria-hidden="true" class="ap-timeline-rail">
      <span :style="dotStyle" />
      <span :style="lineStyle" class="ap-timeline-line" />
    </span>
    <div :style="contentStyle">
      <!-- 标题被 ellipsis 截断时，全量文本只有这里给得出（窄视口实测"赵富康 创建了
           156 篇文档"少 40px、"POST /api/v1/…"少 79px），不给 title 就是信息直接丢了 -->
      <div v-if="title" :style="titleStyle" :title="title">{{ title }}</div>
      <div v-if="time" :style="timeStyle">{{ time }}</div>
      <slot />
    </div>
  </li>
</template>

<script setup lang="ts">
import { computed, inject } from 'vue';
import { AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleType, cv } from '@/utils/core/apple-tokens';

const props = withDefaults(defineProps<{
  title?: string;
  time?: string;
  /** 圆点颜色：传色板令牌取值（cv('green')），与页面其它着色方式一致 */
  color?: string;
}>(), {
  title: '',
  time: '',
  color: '',
});

const density = inject<'default' | 'compact'>('ap-timeline-density', 'default');

const itemStyle = {
  display: 'flex' as const,
  alignItems: 'stretch' as const,
  gap: AppleSpace.sm,
};

const railStyle = {
  display: 'flex' as const,
  flexDirection: 'column' as const,
  alignItems: 'center' as const,
  flexShrink: '0',
  width: AppleSpace.xs,
};

const dotStyle = computed(() => ({
  width: AppleSpace.xs,
  height: AppleSpace.xs,
  borderRadius: AppleRadius.circle,
  backgroundColor: props.color || cv('blue'),
  flexShrink: '0',
  marginTop: density === 'compact' ? '3px' : AppleSpace.xxs,
}));

const lineStyle = {
  flex: '1',
  width: '2px',
  marginTop: AppleSpace.xxs,
  backgroundColor: cv('border-light'),
};

const contentStyle = {
  flex: '1',
  minWidth: '0',
  display: 'flex' as const,
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
};

const titleStyle = computed(() => ({
  fontFamily: AppleFont.text,
  fontSize: density === 'compact' ? AppleType.caption.size : AppleType.bodyPrimary.size,
  fontWeight: AppleFontWeight.medium,
  color: cv('text-primary'),
  overflow: 'hidden' as const,
  textOverflow: 'ellipsis' as const,
  whiteSpace: 'nowrap' as const,
}));

const timeStyle = computed(() => ({
  fontFamily: AppleFont.text,
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
  fontVariantNumeric: 'tabular-nums',
}));
</script>
