<template>
  <div :style="cardStyle">
    <div v-if="data.title" :style="titleStyle">{{ data.title }}</div>
    <div :style="buttonsStyle">
      <ApButton
        v-for="(btn, idx) in data.buttons"
        :key="idx"
        :variant="btn.variant as ButtonVariant"
        size="small"
        :disabled="btn.disabled"
        @click="handleClick(btn)"
      >
        {{ btn.label }}
      </ApButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import ApButton from '@/components/apple/ApButton.vue';
import { cv, AppleSpace, AppleType, AppleFontWeight } from '@/utils/core/apple-tokens';
import { streamCardStyle as cardStyle } from '@/styles/ai/ai-page-styles';
import { rowStyle } from '@/styles/layout/page-styles';

// 与 ApButton.vue 的 variant prop 保持同步
type ButtonVariant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'outline';

interface ButtonAction {
  action: string;
  payload: Record<string, unknown>;
}

interface ButtonItem {
  label: string;
  variant: ButtonVariant;
  action: ButtonAction;
  disabled?: boolean;
}

interface StreamComponent {
  id: string;
  type: string;
  title?: string;
  buttons?: ButtonItem[];
  [key: string]: unknown;
}

defineProps<{
  data: StreamComponent;
}>();

const emit = defineEmits<{
  (e: 'action', action: { action: string; payload: Record<string, unknown> }): void;
}>();

function handleClick(btn: ButtonItem) {
  emit('action', {
    action: btn.action.action,
    payload: btn.action.payload || {},
  });
}

const titleStyle = {
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  marginBottom: AppleSpace.xxs,
};

const buttonsStyle = { ...rowStyle(AppleSpace.xs), flexWrap: 'wrap' as const };
</script>
