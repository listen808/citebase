<template>
  <span data-ap="stat-chip" :style="chipStyle" :title="`${label}：${value}`">
    <ApIcon v-if="isStringIcon" :name="icon as IconName" :size="14" :color="tone" />
    <component :is="icon" v-else-if="icon" :size="14" :color="tone" :stroke-width="iconStrokeFor(14)" />
    <span :style="labelStyle">{{ label }}</span>
    <span :style="valueStyle">{{ value }}</span>
  </span>
</template>

<script setup lang="ts">
import { computed, type Component } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import { AppleFont, AppleFontWeight, AppleLineHeight, AppleSpace, AppleType, cv, iconStrokeFor } from '@/utils/core/apple-tokens';
import type { IconName } from '@/utils/icons';

/**
 * 统计数的紧凑形态：一行工具区里放不下 217×60 的卡片，
 * 但"这一页有多少条"该跟着筛选一起被看见。
 * 与 `ApStatCard` 是同一条数据的两种摆法，数值仍是那一档（`statValue` + 等宽数字），
 * 差别只在没有卡片面 —— 它是行内文字，不是一块面板。
 * icon 与 ApStatCard 同一契约：接受注册表字符串名（走 ApIcon 统一描边）或组件引用。
 */
const props = withDefaults(defineProps<{
  label: string;
  value: string | number;
  icon?: Component | IconName;
  color?: string;
}>(), {
  color: '',
});

const isStringIcon = computed(() => typeof props.icon === 'string');

const tone = computed(() => props.color || cv('text-secondary'));

const chipStyle = {
  display: 'inline-flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  minHeight: AppleSpace.xl,
  flexShrink: 0 as const,
  whiteSpace: 'nowrap' as const,
};

const labelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.regular),
  lineHeight: AppleLineHeight.tight,
  color: cv('text-tertiary'),
};

const valueStyle = {
  fontFamily: AppleType.statValue.font,
  fontSize: AppleType.statValue.size,
  fontWeight: String(AppleType.statValue.weight),
  fontVariantNumeric: 'tabular-nums',
  lineHeight: String(AppleType.statValue.lineHeight),
  letterSpacing: AppleType.statValue.letterSpacing,
  color: tone.value,
};
</script>
