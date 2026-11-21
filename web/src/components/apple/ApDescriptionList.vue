<template>
  <div :style="listStyle">
    <div
      v-for="(item, index) in displayItems"
      :key="index"
      :style="itemStyle"
    >
      <span :style="labelStyle">{{ item.label }}</span>
      <span :style="valueStyle">
        <slot :name="`value-${index}`" :item="item" :value="item.value">
          {{ item.value }}
        </slot>
      </span>
    </div>
    <slot />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { cv, AppleFont, AppleFontWeight, AppleLayout, AppleSpace, AppleType } from '@/utils/core/apple-tokens';

export interface DescriptionItem {
  label: string;
  value?: string | number | null;
}

const props = defineProps<{
  items?: DescriptionItem[];
}>();

const displayItems = computed(() =>
  (props.items || []).filter(item => item.value !== undefined && item.value !== null && item.value !== '')
);

const listStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: '0',
};

const itemStyle = {
  display: 'flex',
  alignItems: 'center',
  padding: `${AppleSpace.md} ${AppleSpace.lg}`,
  borderBottom: `1px solid ${cv('border-light')}`,
};

const labelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-secondary'),
  width: AppleLayout.descListLabelWidth,
  flexShrink: '0',
};

const valueStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-primary'),
  flex: '1',
  minWidth: '0',
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};
</script>
