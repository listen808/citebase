<template>
  <div :style="statusBarStyle">
    <span :style="wordCountStyle">
      {{ wordCount }} {{ $t('cExtraUi.k7') }}
      <span v-if="wordCountStatus?.level === 'warning'" :style="warningStyle">（{{ wordCountStatus.message }}）</span>
      <span v-if="wordCountStatus?.level === 'error'" :style="errorStyle">（{{ wordCountStatus.message }}）</span>
    </span>
    <span :style="statusIndicatorStyle">
      <ApIcon v-if="saveStatus === 'saving'" name="Loader2" :size="12" :style="spinningStyle" />
      <ApIcon v-else-if="saveStatus === 'saved'" name="Check" :size="12" :style="savedStyle" />
      <ApIcon v-else-if="saveStatus === 'error'" name="AlertCircle" :size="12" :style="errorIconStyle" />
      <span>{{ statusText }}</span>
    </span>
    <span v-if="collabVisible" :style="collabIndicatorStyle">
      <span :style="collabDotStyle" />
      <span>{{ collabStatusText }}</span>
      <span v-if="collabUsers && collabUsers.length > 0">{{ collabUsers.length }} {{ $t('cExtraUi.k8') }}</span>
    </span>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { cv, AppleFont, AppleType, AppleFontWeight, AppleRadius, AppleSpace } from '@/utils/core/apple-tokens';
import { useMobile } from '@/composables/ui/useMobile';

interface CollabUser {
  id: number;
  name: string;
  color: string;
}

const props = defineProps<{
  wordCount: number;
  saveStatus: string;
  wordCountStatus?: { level: 'normal' | 'warning' | 'error'; message: string };
  collabVisible?: boolean;
  collabConnected?: boolean;
  collabSynced?: boolean;
  collabUsers?: CollabUser[];
}>();

const { isMobile } = useMobile();

const statusText = computed(() => {
  switch (props.saveStatus) {
    case 'saving': return '保存中...';
    case 'saved': return '已保存';
    case 'error': return '保存失败';
    case 'unsaved': return '未保存';
    default: return '';
  }
});

const collabStatusText = computed(() => {
  if (!props.collabConnected) return '连接中...';
  if (!props.collabSynced) return '同步中...';
  return '协作中';
});

const collabDotColor = computed(() => {
  if (!props.collabConnected) return cv('orange');
  return cv('green');
});

const statusBarStyle = computed(() => ({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: isMobile.value ? `0 ${AppleSpace.sm}` : `0 ${AppleSpace.md}`,
  background: cv('bg-secondary'),
  borderTop: `1px solid ${cv('border-light')}`,
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  minHeight: '24px',
}));

const wordCountStyle = {
  display: 'flex',
  alignItems: 'center' as const,
  gap: AppleSpace.xs,
};

const statusIndicatorStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
};

const spinningStyle = {
  animation: 'spin var(--apple-spinner-duration) linear infinite',
};

const savedStyle = {
  color: cv('green'),
};

const errorIconStyle = {
  color: cv('red'),
};

const warningStyle = {
  color: cv('orange'),
  fontWeight: String(AppleFontWeight.medium),
};

const errorStyle = {
  color: cv('red'),
  fontWeight: String(AppleFontWeight.medium),
};

const collabIndicatorStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xxs,
  color: cv('text-tertiary'),
};

const collabDotStyle = computed(() => ({
  width: '6px',
  height: '6px',
  borderRadius: AppleRadius.circle,
  background: collabDotColor.value,
  display: 'inline-block',
}));
</script>
