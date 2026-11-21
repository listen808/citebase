<template>
  <ApPageContainer no-scroll>
    <div :style="wrapperStyle">
      <!-- 与顶栏同一套主题菜单：原先这里是 light↔dark 二值按钮，
           另外三套主题（液态玻璃/纸感/编辑部）在登录页根本点不到 -->
      <ApThemeMenu glass :style="themeToggleStyle" size="small" />
      <!-- 入场编排（批4）：类名常驻，是否真的动由 :root[data-persona] 决定，所以这里不需要读 store -->
      <div :style="formContainerStyle" class="apple-persona-stack">
        <LoginForm
          ref="loginFormRef"
          :title="siteBrandName"
          :subtitle="$t('auth.subtitle')"
          :loading="auth.loading"
          :error="errorMsg"
          @submit="handleLogin"
        >
          <template #icon>
            <ApBrand :size="40" :show-wordmark="false" enter />
          </template>
          <template #form-extra>
            <div :style="rememberStyle">
              <ApCheckbox v-model="rememberMe" :label="$t('auth.rememberMeState')" />
            </div>
          </template>
        </LoginForm>
        <div :style="cardFooterStyle">
          <router-link to="/author-login" :style="authorLinkStyle">{{ $t('auth.authorEntry') }}</router-link>
        </div>
      </div>

      <p v-if="!backendOnline" :style="offlineStyle" role="alert">
        <ApIcon name="AlertTriangle" :size="16" :color="cv('red')" /> {{ $t('auth.offline') }}
      </p>

      <p v-if="showInstallHint" :style="installHintStyle">
        {{ $t('auth.notInstalled') }}
        <router-link to="/install" :style="installLinkStyle">{{ $t('auth.goInstall') }}</router-link>
      </p>

      <div v-if="ssoList.length" :style="dividerStyle">
        <span :style="dividerTextStyle">{{ $t('auth.or') }}</span>
      </div>

      <div v-if="ssoList.length" :style="ssoSectionStyle">
        <ApButton
          v-for="sso in ssoList"
          :key="sso.id"
          variant="secondary"
          size="medium"
          :style="ssoBtnStyle"
          @click="handleSSOLogin(sso)"
        >
          <ApIcon name="Key" :size="16" />
          {{ $t('auth.loginWith', { name: sso.name }) }}
        </ApButton>
      </div>

      <div v-if="oidcProviders.length" :style="dividerStyle">
        <span :style="dividerTextStyle">{{ $t('auth.or') }}</span>
      </div>

      <div v-if="oidcProviders.length" :style="ssoSectionStyle">
        <ApButton
          v-for="provider in oidcProviders"
          :key="'oidc-' + provider.id"
          variant="secondary"
          size="medium"
          :style="ssoBtnStyle"
          @click="handleOIDCLogin(provider)"
        >
          <img v-if="provider.icon_url" :src="provider.icon_url" :alt="provider.display_name" :style="oidcIconStyle" />
          <ApIcon v-else name="Shield" :size="16" />
          {{ $t('auth.loginWith', { name: provider.display_name }) }}
        </ApButton>
      </div>
    </div>
  </ApPageContainer>
</template>

<script setup lang="ts">
import { ref, shallowRef, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';
import { useAuthStore } from '@/stores/auth/auth';
import { AppleZIndex, cv, AppleType, AppleFontWeight, AppleSpace, AppleRadius, AppleTransition, AppleLayout, AppleGlass } from '@/utils/core/apple-tokens';
import { authCardPadding } from '@/styles/layout/page-styles';
import { getEnabledSSOList, initiateSSOLogin, getOIDCProviders, initiateOIDCLogin, type OIDCProviderPublicInfo } from '@/api/modules/auth/sso';
import { getInstallState, isBackendHealthy } from '@/api/modules/auth/install';
import { siteBrandName } from '@/utils/core/site-brand';
import ApButton from '@/components/apple/ApButton.vue';
import ApCheckbox from '@/components/apple/ApCheckbox.vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApThemeMenu from '@/components/apple/ApThemeMenu.vue';
import ApBrand from '@/components/apple/ApBrand.vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import LoginForm from '@/components/auth/LoginForm.vue';
import { storageGet, storageSet, StorageKeys } from '@/utils/core/storage';
import { isSafeRedirect } from '@/utils/core/shared';

const router = useRouter();
const auth = useAuthStore();
const { t } = useI18n();

const errorMsg = shallowRef('');
const ssoList = ref<{ id: number; name: string; username_prefix: string }[]>([]);
const oidcProviders = ref<OIDCProviderPublicInfo[]>([]);
const rememberMe = shallowRef(false);
const showInstallHint = shallowRef(false);
const backendOnline = shallowRef(true);
const loginFormRef = ref<{ clearPassword: () => void } | null>(null);

onMounted(async () => {
  backendOnline.value = await isBackendHealthy();
  try {
    const saved = storageGet(StorageKeys.AUTH_REMEMBER_ME);
    if (saved) {
      // 历史数据可能存的是 1/0（数字），ApCheckbox 的 modelValue 要布尔 —— 强制收敛
      rememberMe.value = !!JSON.parse(saved);
    }
    ssoList.value = await getEnabledSSOList();

    try {
      oidcProviders.value = await getOIDCProviders();
    } catch {
      oidcProviders.value = [];
    }

    const state = await getInstallState();
    if (state && !state.admin_exists) {
      showInstallHint.value = true;
    }
  } catch {
    ssoList.value = [];
  }
});

async function handleLogin(username: string, password: string) {
  errorMsg.value = '';
  if (!username || !password) {
    errorMsg.value = t('auth.needCredentials');
    return;
  }
  storageSet(StorageKeys.AUTH_REMEMBER_ME, JSON.stringify(rememberMe.value));

  const success = await auth.login({ username, password }, rememberMe.value);
  if (success) {
    const redirect = (router.currentRoute.value.query.redirect as string) || '';
    if (redirect.startsWith('/') && !redirect.startsWith('//')) {
      router.push(redirect);
    } else {
      router.push('/');
    }
  } else {
    errorMsg.value = auth.loginError || t('auth.badCredentials');
    loginFormRef.value?.clearPassword();
  }
}

async function handleSSOLogin(sso: { id: number; name: string }) {
  try {
    const res = await initiateSSOLogin(sso.id);
    if (!isSafeRedirect(res.redirect_url)) {
      errorMsg.value = t('auth.ssoRedirectInvalid');
      return;
    }
    window.location.href = res.redirect_url;
  } catch {
    errorMsg.value = t('auth.ssoStartFailed');
  }
}

async function handleOIDCLogin(provider: OIDCProviderPublicInfo) {
  try {
    errorMsg.value = '';
    const res = await initiateOIDCLogin(provider.id);
    if (!isSafeRedirect(res.redirect_url)) {
      errorMsg.value = t('auth.oidcRedirectInvalid');
      return;
    }
    window.location.href = res.redirect_url;
  } catch {
    errorMsg.value = t('auth.oidcStartFailed');
  }
}

const wrapperStyle = {
  position: 'relative' as const,
  flex: '1',
  display: 'flex',
  flexDirection: 'column' as const,
  alignItems: 'center',
  justifyContent: 'center',
  gap: AppleSpace.md,
  padding: `${AppleSpace.lg} ${AppleSpace.md}`,
};

const formContainerStyle = {
  width: '100%',
  maxWidth: AppleLayout.cardNarrow,
  ...AppleGlass.standard,
  borderRadius: AppleRadius.card,
  padding: authCardPadding,
  // 棱线与阴影是两件事：AppleGlass.standard 展开来的 boxShadow 被这行整条顶掉，
  // 玻璃的方向性棱线（--apple-glass-edge）就跟着没了 —— 卡片退化成一块带发丝边的半透板。
  boxShadow: `${cv('glass-edge')}, ${cv('shadow-subtle')}`,
  textAlign: 'center' as const };

const rememberStyle = {
  display: 'flex',
  alignItems: 'center' };

const dividerStyle = {
  display: 'flex',
  alignItems: 'center',
  alignSelf: 'center' as const,
  width: '100%',
  maxWidth: AppleLayout.cardNarrow,
  gap: AppleSpace.md };

const dividerTextStyle = {
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
  whiteSpace: 'nowrap' as const,
  padding: `0 ${AppleSpace.xs}` };

const ssoSectionStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
  width: '100%',
  maxWidth: AppleLayout.cardNarrow };

const ssoBtnStyle = {
  width: '100%' };

const oidcIconStyle = {
  width: '16px',
  height: '16px',
  objectFit: 'contain' as const,
  marginRight: AppleSpace.xs };

const cardFooterStyle = {
  marginTop: AppleSpace.lg,
  paddingTop: AppleSpace.md,
  borderTop: `1px solid ${cv('border-light')}` };

const authorLinkStyle = {
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
  textDecoration: 'none',
  transition: AppleTransition.fast };

const installHintStyle = {
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  background: cv('blue-light'),
  borderRadius: AppleRadius.card,
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
  width: '100%',
  maxWidth: AppleLayout.cardNarrow,
  textAlign: 'center' as const };

const installLinkStyle = {
  color: cv('blue'),
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.semibold),
  textDecoration: 'none' };

const offlineStyle = {
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  borderRadius: AppleRadius.small,
  background: cv('red-light'),
  color: cv('red'),
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  width: '100%',
  maxWidth: AppleLayout.cardNarrow,
  textAlign: 'center' as const };

const themeToggleStyle = {
  position: 'absolute' as const,
  top: AppleSpace.lg,
  right: AppleSpace.lg,
  width: AppleLayout.touchTargetMin,
  height: AppleLayout.touchTargetMin,
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  borderRadius: AppleRadius.circle,
  transition: AppleTransition.default,
  zIndex: String(AppleZIndex.base),
};
</script>
