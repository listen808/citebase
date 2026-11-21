<template>
  <div :style="cardStyle">
    <div v-if="data.title" :style="titleStyle">{{ data.title }}</div>
    <div :style="gridStyle">
      <div v-for="(item, idx) in data.items" :key="idx" :style="itemStyle">
        <div :style="itemValueStyle(item.color)">{{ item.value }}</div>
        <div :style="itemLabelStyle">{{ item.label }}</div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { cv, AppleSpace, AppleRadius, AppleType, AppleFontWeight, AppleGridFloor } from '@/utils/core/apple-tokens';
import { streamCardStyle as cardStyle } from '@/styles/ai/ai-page-styles';

interface StatusItem {
  label: string;
  value: string;
  icon?: string;
  color?: string;
}

interface StreamComponent {
  id: string;
  type: string;
  title?: string;
  items?: StatusItem[];
  [key: string]: unknown;
}

defineProps<{
  data: StreamComponent;
}>();

function getColor(color?: string) {
  switch (color) {
    case 'success':
      return cv('green');
    case 'warning':
      return cv('orange');
    case 'error':
      return cv('red');
    case 'info':
      return cv('blue');
    default:
      return cv('text-primary');
  }
}

const titleStyle = {
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  marginBottom: AppleSpace.sm,
};

const gridStyle = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fit, ${AppleGridFloor.stat})`,
  gap: AppleSpace.sm,
};

const itemStyle = {
  background: cv('surface-panel'),
  borderRadius: AppleRadius.small,
  padding: AppleSpace.md,
  textAlign: 'center' as const,
};

const itemValueStyle = (color?: string) => ({
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: getColor(color),
  marginBottom: AppleSpace.xxs,
});

const itemLabelStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};
</script>
