<template>
  <div :style="cardStyle">
    <div v-if="data.title" :style="titleStyle">{{ data.title }}</div>
    <div :style="stepsStyle">
      <div v-for="(step, idx) in data.steps" :key="step.id || idx" :style="stepStyle">
        <div :style="stepIndicatorStyle(step.status)">
          <ApIcon v-if="step.status === 'success'" name="Check" :size="14" :style="checkIconStyle" />
          <ApIcon v-else-if="step.status === 'failed'" name="X" :size="14" :style="failIconStyle" />
          <span v-else-if="step.status === 'running'" :style="spinnerStyle" />
          <span v-else :style="numStyle">{{ idx + 1 }}</span>
        </div>
        <div :style="stepContentStyle">
          <div :style="stepNameStyle(step.status)">{{ step.name }}</div>
          <div v-if="step.detail" :style="stepDetailStyle">{{ step.detail }}</div>
        </div>
        <div v-if="step.duration_ms" :style="durationStyle">
          {{ formatDurationMs(step.duration_ms) }}
        </div>
      </div>
    </div>
    <div v-if="data.total_ms" :style="totalStyle">
      总耗时：{{ formatDurationMs(data.total_ms) }}
    </div>
  </div>
</template>

<script setup lang="ts">
import { cv, AppleSpace, AppleRadius, AppleType, AppleFontWeight, AppleLineHeight } from '@/utils/core/apple-tokens';
import { formatDurationMs } from '@/utils/format/format-time';
import { streamCardStyle as cardStyle } from '@/styles/ai/ai-page-styles';
import { listStyle } from '@/styles/layout/page-styles';

type TaskStatus = 'pending' | 'running' | 'success' | 'failed' | 'skipped';

interface TaskStep {
  id?: string;
  name: string;
  status: TaskStatus;
  detail?: string;
  duration_ms?: number;
}

interface StreamComponent {
  id: string;
  type: string;
  title?: string;
  steps?: TaskStep[];
  current_idx?: number;
  total_ms?: number;
  [key: string]: unknown;
}

defineProps<{
  data: StreamComponent;
}>();

function getStatusColor(status: TaskStatus): string {
  switch (status) {
    case 'success':
      return cv('green');
    case 'failed':
      return cv('red');
    case 'running':
      return cv('blue');
    case 'skipped':
      return cv('text-tertiary');
    default:
      return cv('text-quaternary');
  }
}

const titleStyle = {
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  marginBottom: AppleSpace.sm,
};

const stepsStyle = listStyle(AppleSpace.sm);

const stepStyle = {
  display: 'flex',
  alignItems: 'flex-start',
  gap: AppleSpace.sm,
};

const stepIndicatorStyle = (status: TaskStatus) => ({
  width: '24px',
  height: '24px',
  borderRadius: AppleRadius.circle,
  background: status === 'running' ? cv('blue-light') : status === 'success' ? cv('green') : status === 'failed' ? cv('red') : cv('bg-tertiary'),
  color: status === 'success' || status === 'failed' ? cv('on-accent') : getStatusColor(status),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  flexShrink: '0',
  fontSize: AppleType.microUI.size,
  letterSpacing: AppleType.microUI.letterSpacing,
  fontWeight: String(AppleFontWeight.semibold),
});

const checkIconStyle = {
  fontSize: AppleType.microUI.size,
  lineHeight: '1',
};

const failIconStyle = {
  fontSize: AppleType.microUI.size,
  lineHeight: '1',
};

const spinnerStyle = {
  width: '12px',
  height: '12px',
  border: `2px solid ${cv('blue')}`,
  borderTopColor: 'transparent',
  borderRadius: AppleRadius.circle,
  animation: 'spin var(--apple-spinner-duration) linear infinite',
};

const numStyle = {
  fontSize: AppleType.microUI.size,
  letterSpacing: AppleType.microUI.letterSpacing,
  lineHeight: '1',
};

const stepContentStyle = {
  flex: '1',
  minWidth: '0',
};

const stepNameStyle = (status: TaskStatus) => ({
  fontSize: AppleType.controlLabel.size,
  fontWeight: status === 'running' ? '500' : '400',
  color: getStatusColor(status),
});

const stepDetailStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  marginTop: AppleSpace.xxs,
  lineHeight: AppleLineHeight.default,
};

const durationStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  flexShrink: '0',
  marginTop: AppleSpace.xxs,
};

const totalStyle = {
  marginTop: AppleSpace.sm,
  paddingTop: AppleSpace.sm,
  borderTop: `1px solid ${cv('border-light')}`,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
  textAlign: 'right' as const,
};
</script>

