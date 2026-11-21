<template>
  <slot v-if="!error" :key="attempt" />
  <div v-else :style="containerStyle" role="alert" aria-live="assertive">
    <div :style="cardStyle">
      <div :style="iconContainerStyle">
        <span :style="iconStyle" aria-hidden="true">&#x26A0;</span>
      </div>

      <h2 id="error-boundary-title" :style="titleStyle">{{ headline }}</h2>
      <p :style="messageStyle">
        {{ blurb }}
      </p>

      <div v-if="showDetail" :style="detailStyle" aria-labelledby="error-boundary-title">
        <p :style="detailTextStyle" role="log">{{ errorMessage }}</p>
      </div>

      <div :style="actionsStyle">
        <ApButton variant="primary" size="medium" :disabled="waitLeft > 0" :aria-label="$t('commonUi.k2')" @click="handleRetry">
          {{ waitLeft > 0 ? $t('common.retryIn', { n: waitLeft }) : $t('common.retry') }}
        </ApButton>
        <ApButton variant="secondary" size="medium" :aria-label="$t('commonUi.k1')" @click="handleGoHome">
          {{ $t('commonUi.k1') }}
        </ApButton>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, shallowRef, onErrorCaptured, computed, onBeforeUnmount } from 'vue';
import { errorPresentation, type ErrorPresentation } from './error-presentation';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';
import { cv, AppleFont, AppleType, AppleSpace, AppleRadius } from '@/utils/core/apple-tokens';
import { logger } from '@/utils/logger';
import ApButton from '@/components/apple/ApButton.vue';

const error = ref<Error | null>(null);
const errorMessage = ref('');
const { t } = useI18n();
// 限流与服务端抖动是可恢复状态，不该崩成「渲染内容时发生了未知错误」——
// 用户读到的是"这系统坏了"，而真相只是"你刚才太频繁，等 N 秒"，
// 而且 N 服务端已经放在 Retry-After 里了。判断本身在 error-presentation.ts（可单测）。
const presentation = shallowRef<ErrorPresentation>(errorPresentation(null, t));
const waitLeft = shallowRef(0);
let waitTimer: ReturnType<typeof setInterval> | null = null;

const headline = computed(() => presentation.value.headline);
const blurb = computed(() => presentation.value.blurb);

function stopWaiting() {
  if (waitTimer) { clearInterval(waitTimer); waitTimer = null; }
  waitLeft.value = 0;
}

function startWaiting(seconds: number) {
  stopWaiting();
  waitLeft.value = seconds;
  waitTimer = setInterval(() => {
    waitLeft.value -= 1;
    if (waitLeft.value <= 0) {
      stopWaiting();
      handleRetry();
    }
  }, 1000);
}

onBeforeUnmount(stopWaiting);
const attempt = ref(0);
const router = useRouter();
const showDetail = computed(() => import.meta.env.DEV);

onErrorCaptured((err: Error) => {
  error.value = err;
  errorMessage.value = err.message || String(err);
  presentation.value = errorPresentation(err, t);
  logger.error('[ErrorBoundary] Caught error', err);
  if (presentation.value.waitSeconds !== null) startWaiting(presentation.value.waitSeconds);
  return false;
});

const containerStyle = computed<Record<string, string>>(() => ({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  minHeight: '60vh',
  padding: AppleSpace.lg,
  background: cv('surface-canvas'),
}));

const cardStyle = computed<Record<string, string>>(() => ({
  display: 'flex',
  flexDirection: 'column',
  alignItems: 'center',
  textAlign: 'center',
  maxWidth: '480px',
  width: '100%',
  padding: `${AppleSpace['40px']} ${AppleSpace.xl}`,
  background: cv('surface-panel'),
  borderRadius: AppleRadius.card,
  boxShadow: cv('shadow-level2'),
  border: `1px solid ${cv('border-light')}`,
}));

const iconContainerStyle = computed<Record<string, string>>(() => ({
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  width: '64px',
  height: '64px',
  borderRadius: AppleRadius.circle,
  background: cv('red-light'),
  marginBottom: AppleSpace.lg,
}));

const iconStyle = computed<Record<string, string>>(() => ({
  fontSize: AppleType.pageTitle.size,
  lineHeight: '1',
}));

const titleStyle = computed<Record<string, string>>(() => ({
  fontFamily: AppleType.utilityHeading.font,
  fontSize: AppleType.utilityHeading.size,
  fontWeight: String(AppleType.utilityHeading.weight),
  lineHeight: String(AppleType.utilityHeading.lineHeight),
  letterSpacing: AppleType.utilityHeading.letterSpacing,
  color: cv('text-primary'),
  margin: '0',
  marginBottom: AppleSpace.sm,
}));

const messageStyle = computed<Record<string, string>>(() => ({
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleType.bodyPrimary.weight),
  lineHeight: String(AppleType.bodyPrimary.lineHeight),
  letterSpacing: AppleType.bodyPrimary.letterSpacing,
  color: cv('text-secondary'),
  margin: '0',
  marginBottom: AppleSpace.lg,
}));

const detailStyle = computed<Record<string, string>>(() => ({
  width: '100%',
  padding: AppleSpace.md,
  marginBottom: AppleSpace.lg,
  background: cv('bg-secondary'),
  borderRadius: AppleRadius.medium,
  border: `1px solid ${cv('border-light')}`,
  textAlign: 'left',
  overflow: 'auto',
  maxHeight: '120px',
}));

const detailTextStyle = computed<Record<string, string>>(() => ({
  fontFamily: AppleFont.mono,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleType.microUI.weight),
  lineHeight: String(AppleType.microUI.lineHeight),
  color: cv('text-secondary'),
  margin: '0',
  wordBreak: 'break-all',
  whiteSpace: 'pre-wrap',
}));

const actionsStyle = computed<Record<string, string>>(() => ({
  display: 'flex',
  gap: AppleSpace.sm,
  alignItems: 'center',
}));

function handleRetry() {
  stopWaiting();
  error.value = null;
  errorMessage.value = '';
  presentation.value = errorPresentation(null, t);
  attempt.value++;
}

function handleGoHome() {
  stopWaiting();
  error.value = null;
  errorMessage.value = '';
  presentation.value = errorPresentation(null, t);
  router.push('/');
}
</script>
