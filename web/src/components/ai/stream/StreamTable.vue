<template>
  <div :style="cardStyle">
    <div v-if="data.title" :style="titleStyle">{{ data.title }}</div>
    <ApTable :columns="columns" :data="rows" />
    <div v-if="data.total" :style="footerStyle">{{ $t('common.recordCount', { n: data.total }) }}</div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import ApTable from '@/components/apple/ApTable.vue';
import { cv, AppleSpace, AppleType, AppleFontWeight } from '@/utils/core/apple-tokens';
import { streamCardStyle as cardStyle } from '@/styles/ai/ai-page-styles';

interface DataTableColumn {
  key: string;
  title: string;
  width?: string;
  align?: string;
}

interface StreamComponent {
  id: string;
  type: string;
  title?: string;
  columns?: DataTableColumn[];
  rows?: unknown[][];
  total?: number;
  [key: string]: unknown;
}

const props = defineProps<{
  data: StreamComponent;
}>();

const columns = computed(() => {
  return (props.data.columns || []).map((col: DataTableColumn) => ({
    key: col.key,
    title: col.title,
  }));
});

const rows = computed(() => {
  const cols = props.data.columns || [];
  return (props.data.rows || []).map((row: unknown[]) => {
    const obj: Record<string, unknown> = {};
    cols.forEach((col, idx) => {
      obj[col.key] = row[idx];
    });
    return obj;
  });
});

const titleStyle = {
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  marginBottom: AppleSpace.xxs,
};

const footerStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  marginTop: AppleSpace.xs,
  textAlign: 'right' as const,
};
</script>
