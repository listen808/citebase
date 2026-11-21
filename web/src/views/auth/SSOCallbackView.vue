<template>
  <ApPageContainer no-scroll>
    <div :style="wrapperStyle">
      <div v-if="loading" :style="loadingContentStyle">
      <div :style="spinnerWrapStyle">
        <ApIcon name="Loader" :size="AppleIconSize.status" :color="cv('blue')" />
      </div>
      <p :style="textStyle">{{ $t('vAuthUi.k24') }}</p>
    </div>
    <ApErrorPage
      v-else-if="error"
      :icon="icons.AlertTriangle"
      icon-color="red"
      :status-code="$t('lastAttrsUi.k6')"
      :title="$t('vAuthUi.k26')"
      :description="error"
      :primary-label="$t('lastAttrsUi.k5')"
      :secondary-label="$t('lastAttrsUi.retry')"
      :on-primary-action="goToLogin"
      :on-secondary-action="retry"
    />
    <div v-else :style="successStyle">
      <p :style="successTextStyle">{{ $t('vAuthUi.k25') }}</p>
    </div>
    </div>
  </ApPageContainer>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { shallowRef, onMounted, onBeforeUnmount } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { handleSSOCallback, handleOIDCCallback } from '@/api/modules/auth/sso';
import { useAuthStore } from '@/stores/auth/auth';
import {AppleAnimation, AppleFont, AppleIconSize, AppleInteractionState, AppleLayout, AppleSpace, AppleType, cv} from '@/utils/core/apple-tokens';
import { loadingTextStyle } from '@/styles/layout/page-styles';
import ApErrorPage from '@/components/apple/ApErrorPage.vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
// ApErrorPage 的 icon 是 Component、经 :is 渲染，必须传注册表组件（字符串会被当成组件渲染失败）。
import { icons } from '@/utils/icons';

const route = useRoute();
const router = useRouter();
const auth = useAuthStore();

const loading = shallowRef(true);
const error = shallowRef<string>('');
let redirectTimer: ReturnType<typeof setTimeout> | null = null;
let callbackTimeoutId: ReturnType<typeof setTimeout> | null = null;

async function processCallback() {
  const code = route.query.code as string;
  const state = route.query.state as string;
  const sso_id = route.query.sso_id as string;
  const provider_id = route.query.provider_id as string;

  // Detect callback type: OIDC (provider_id) or legacy SSO (sso_id)
  const isOIDC = !!provider_id;
const { t } = useI18n();

  if (!code || !state || (!sso_id && !provider_id)) {
    error.value = t('sentUi.k47');
    loading.value = false;
    return;
  }

  callbackTimeoutId = setTimeout(() => {
    error.value = t('sentUi.k48');
    loading.value = false;
  }, 15000);

  const timeoutId = callbackTimeoutId;

  try {
    let res: { access_token: string; refresh_token: string; expires_in: number };

    if (isOIDC) {
      res = await handleOIDCCallback(code, state, parseInt(provider_id, 10));
    } else {
      res = await handleSSOCallback(code, state, parseInt(sso_id, 10));
    }

    auth.setTokens(res.access_token, res.refresh_token, res.expires_in || 3600);
    await auth.fetchUser();

    if (localStorage.getItem('author_login') === '1') {
      localStorage.removeItem('author_login');
      if (auth.user?.role_id !== 1) {
        auth.logout();
        clearTimeout(timeoutId);
        loading.value = false;
        error.value = t('sentUi.k49');
        redirectTimer = setTimeout(() => {
          router.push('/login');
        }, 1500);
        return;
      }
    }

    clearTimeout(timeoutId);
    loading.value = false;

    redirectTimer = setTimeout(() => {
      router.push('/');
    }, 300);
  } catch (err: unknown) {
    clearTimeout(timeoutId);
    error.value = (err as Error).message || t('fallback.k4');
    loading.value = false;
  }
}

onMounted(() => {
  processCallback();
});

onBeforeUnmount(() => {
  if (redirectTimer) {
    clearTimeout(redirectTimer);
    redirectTimer = null;
  }
  if (callbackTimeoutId) {
    clearTimeout(callbackTimeoutId);
    callbackTimeoutId = null;
  }
});

function retry() {
  loading.value = true;
  error.value = '';
  processCallback();
}

function goToLogin() {
  router.push('/login');
}

const wrapperStyle = {
  flex: '1',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
};

const loadingContentStyle = {
  ...loadingTextStyle,
  width: '100%',
  maxWidth: AppleLayout.cardNarrow,
  display: 'flex',
  flexDirection: 'column' as const,
  alignItems: 'center',
};

const spinnerWrapStyle = {
  display: 'flex',
  marginBottom: AppleSpace.md,
  animation: `${AppleAnimation.spin} ${AppleInteractionState.spinnerDuration} linear infinite`,
};

const textStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.linkActionHeading.size,
  color: cv('text-primary'),
  margin: '0' };

const successStyle = {
  width: '100%',
  maxWidth: AppleLayout.cardNarrow,
  display: 'flex' as const,
  flexDirection: 'column' as const,
  alignItems: 'center' as const,
  gap: AppleSpace.md,
  textAlign: 'center' as const };

const successTextStyle = {
  ...textStyle,
  color: cv('green'),
  margin: '0' };
</script>


