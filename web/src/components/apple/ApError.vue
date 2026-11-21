<script setup lang="ts">
import { AppleIconSize, cv, AppleType, AppleSpace, AppleFont, AppleFontWeight, AppleLineHeight } from '@/utils/core/apple-tokens';
import ApIcon from '@/components/apple/ApIcon.vue';

defineProps<{
  message?: string;
  error?: Error;
  showRetry?: boolean;
}>();

const emit = defineEmits<{
  retry: [];
}>();

const errorStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  alignItems: 'center',
  justifyContent: 'center',
  padding: `${AppleSpace['2xl']} ${AppleSpace.xl}`,
  gap: AppleSpace.md,
  textAlign: 'center' as const,
};

const iconStyle = {
  color: cv('red'),
  marginBottom: AppleSpace.sm,
};

const messageStyle = {
  color: cv('text-secondary'),
  fontFamily: AppleFont.text,
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.medium),
  // 400 不是"整页错误的容器宽"，别去和 ApErrorPage / ErrorBoundary 的 480 对齐 ——
  // 那两个是**独立页面**的兜底版面（404 / 500 / SSO 回调），这个是嵌在表格区、
  // 卡片区里的错误块，限的是**一句错误信息的行长**（读屏与扫读都按行段计）。
  // 角色不同，所以两档并存；哪天 ApError 被放到整页上，该换的是组件而不是这个数。
  maxWidth: '400px',
  lineHeight: AppleLineHeight.loose,
};
</script>

<template>
  <div :style="errorStyle">
    <div :style="iconStyle">
      <ApIcon name="AlertCircle" :size="AppleIconSize.empty" />
    </div>
    <div :style="messageStyle">{{ message || $t('common.errorRetry') }}</div>
    <ApButton v-if="showRetry" size="small" @click="emit('retry')">
      {{ $t('apUi1.retry') }}
    </ApButton>
  </div>
</template>

<script lang="ts">
export default {
  name: 'ApError'
};
</script>

