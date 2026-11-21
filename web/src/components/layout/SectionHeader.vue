<script setup lang="ts">
import { computed } from 'vue';
import { LayoutStyles } from '@/styles/layout/page-styles';

interface Props {
  title: string;
  subtitle?: string;
  align?: 'left' | 'between';
}

const props = withDefaults(defineProps<Props>(), {
  align: 'between',
});

const headerStyle = computed(() => {
  return props.align === 'left' 
    ? LayoutStyles.sectionHeaderLeftAligned 
    : LayoutStyles.sectionHeader;
});
</script>

<template>
  <div :style="headerStyle">
    <div>
      <h2 :style="LayoutStyles.sectionHeaderTitle">{{ title }}</h2>
      <div v-if="subtitle" :style="LayoutStyles.sectionHeaderSubtitle">{{ subtitle }}</div>
    </div>
    <div v-if="$slots.actions" :style="LayoutStyles.sectionHeaderActions">
      <slot name="actions" />
    </div>
  </div>
</template>
