<template>
  <div :style="sectionStyle">
    <ApButton variant="ghost" :style="toggleStyle" size="small" @click="expanded = !expanded">
      <ApIcon name="Brain" :size="16" />
      <span>{{ expanded ? $t('aiUi.k22') : $t('aiUi.k23') }}</span>
      <ApIcon name="ChevronDown" :size="16" :style="{ transform: expanded ? 'rotate(180deg)' : 'none', transition: 'transform var(--apple-dur-interactive)' }" />
    </ApButton>
    <div v-show="expanded" :style="contentStyle">
      <div :style="textStyle">{{ content }}</div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import ApButton from '@/components/apple/ApButton.vue';
import { cv, AppleSpace, AppleRadius, AppleType } from '@/utils/core/apple-tokens';

defineProps<{
  content: string;
}>();

const expanded = ref(false);

const sectionStyle = { marginBottom: AppleSpace.xs };
const toggleStyle = { gap: AppleSpace.xxs };
const contentStyle = { marginTop: AppleSpace.xs, padding: `${AppleSpace.xs} ${AppleSpace.sm}`, background: cv('bg-tertiary'), borderRadius: AppleRadius.small };
const textStyle = { fontSize: AppleType.microUI.size, color: cv('text-secondary'), whiteSpace: 'pre-wrap' as const };
</script>
