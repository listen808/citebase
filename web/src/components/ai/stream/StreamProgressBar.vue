<template>
  <div :style="cardStyle">
    <div v-if="data.title" :style="titleStyle">{{ data.title }}</div>
    <div :style="progressContainerStyle">
      <ApProgress
        :value="data.percent"
        :label="data.title || $t('fallback.k7')"
        :tone="statusTone"
        size="bar"
        :style="{ flex: 1 }"
      />
      <div v-if="data.show_label" :style="labelStyle">
        {{ Math.round(data.percent) }}%
      </div>
    </div>
    <div v-if="data.status" :style="statusStyle">
      {{ statusText }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { cv, AppleSpace, AppleType, AppleFontWeight } from '@/utils/core/apple-tokens';
import { toneColor, type ApTone } from '@/utils/core/progress-tone';
import { streamCardStyle as cardStyle } from '@/styles/ai/ai-page-styles';
import { rowStyle } from '@/styles/layout/page-styles';

interface StreamComponent {
  id: string;
  type: string;
  title?: string;
  percent: number;
  status?: string;
  show_label?: boolean;
  [key: string]: unknown;
}

const props = defineProps<{
  data: StreamComponent;
}>();

const statusText = computed(() => {
  switch (props.data.status) {
    case 'running':
      return '进行中';
    case 'success':
      return '已完成';
    case 'error':
      return '失败';
    default:
      return '';
  }
});

const statusTone = computed<ApTone>(() => {
  switch (props.data.status) {
    case 'success':
      return 'success';
    case 'error':
      return 'danger';
    default:
      return 'accent';
  }
});

// 状态文字比进度条浅一档：无状态时回到次要文字色
const statusColor = computed(() =>
  (props.data.status ? toneColor(statusTone.value) : cv('text-secondary')));

const titleStyle = {
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  marginBottom: AppleSpace.sm,
};

const progressContainerStyle = rowStyle(AppleSpace.sm);

const labelStyle = {
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-secondary'),
  minWidth: '40px',
  textAlign: 'right' as const,
};

const statusStyle = computed(() => ({
  fontSize: AppleType.microUI.size,
  color: statusColor.value,
  marginTop: AppleSpace.xs,
}));
</script>
