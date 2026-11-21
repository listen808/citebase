<template>
  <div :style="containerStyle">
    <component
      :is="compMap[comp.type as keyof typeof compMap]"
      v-for="comp in components"
      :key="comp.id"
      :data="comp"
      @action="(a: StreamAction) => emit('action', { ...a, compId: comp.id })"
    />
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import StreamTable from './StreamTable.vue';
import StreamChart from './StreamChart.vue';
import StreamButtonGroup from './StreamButtonGroup.vue';
import StreamStatusCard from './StreamStatusCard.vue';
import StreamTaskTracker from './StreamTaskTracker.vue';
import StreamProgressBar from './StreamProgressBar.vue';
import { AppleSpace } from '@/utils/core/apple-tokens';

export interface StreamComponent {
  id: string;
  type: string;
  title?: string;
  updated_at?: number;
  [key: string]: unknown;
}

interface StreamAction {
  action: string;
  payload: Record<string, unknown>;
}

defineProps<{
  components: StreamComponent[];
}>();

const emit = defineEmits<{
  (e: 'action', action: StreamAction & { compId: string }): void;
}>();

const compMap = computed(() => ({
  data_table: StreamTable,
  chart: StreamChart,
  button_group: StreamButtonGroup,
  status_card: StreamStatusCard,
  task_tracker: StreamTaskTracker,
  progress_bar: StreamProgressBar,
}));

const containerStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
  marginTop: AppleSpace.xs,
};
</script>
