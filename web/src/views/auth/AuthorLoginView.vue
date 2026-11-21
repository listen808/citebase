<template>
  <ApPageContainer no-scroll>
    <div :style="wrapperStyle">
      <!-- 与顶栏同一套主题菜单：原先这里是 light↔dark 二值按钮，
           另外三套主题（液态玻璃/纸感/编辑部）在登录页根本点不到 -->
      <ApThemeMenu glass :style="themeToggleStyle" size="small" />
      <div :style="formContainerStyle">
        <LoginForm
          :title="$t('auth.authorTitle')"
          :subtitle="$t('auth.authorSubtitle')"
          :loading="loading"
          :error="error"
          @submit="handleLogin"
        >
          <template #icon>
            <ApBrand :size="48" :show-wordmark="false" enter />
          </template>
          <template #form-extra>
            <div :style="rememberStyle">
              <ApCheckbox v-model="rememberMe" :label="$t('auth.rememberMeState')" />
            </div>
          </template>
          <template #default>
            <template v-if="authorConfig && authorConfig.sso_enabled && authorConfig.sso_list.length > 0">
              <div :style="dividerStyle">
                <span :style="dividerTextStyle">{{ $t('auth.or') }}</span>
              </div>
              <div :style="ssoListStyle">
                <ApButton
                  v-for="sso in authorConfig.sso_list"
                  :key="sso.id"
                  variant="secondary"
                  size="medium"
                  :style="ssoBtnStyle"
                  @click="handleSSOLogin(sso.id)"
                >
                  {{ $t('auth.loginWithProvider', { name: sso.name }) }}
                </ApButton>
              </div>
            </template>
          </template>
        </LoginForm>

        <div :style="cardFooterStyle">
          <router-link to="/login" :style="backLinkStyle">{{ $t('auth.backToLogin') }}</router-link>
        </div>
      </div>
    </div>
  </ApPageContainer>
</template>

<script setup lang="ts">
import { ref, shallowRef, onMounted } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';
import ApBrand from '@/components/apple/ApBrand.vue';
import { AppleZIndex, cv, AppleType, AppleSpace, AppleRadius, AppleTransition, AppleGlass, AppleLayout } from '@/utils/core/apple-tokens';
import { authCardPadding } from '@/styles/layout/page-styles';
import { isSafeRedirect } from '@/utils/core/shared';
import ApButton from '@/components/apple/ApButton.vue';
import ApCheckbox from '@/components/apple/ApCheckbox.vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import { storageGet, storageSet, storageRemove, StorageKeys } from '@/utils/core/storage';
import LoginForm from '@/components/auth/LoginForm.vue';
import { useAuthStore } from '@/stores/auth/auth';
import { getAuthorConfig, authorLogin } from '@/api/modules/auth/author';
import type { AuthorConfig } from '@/api/modules/auth/author';
import { initiateSSOLogin } from '@/api/modules/auth/sso';

const router = useRouter();
const { t } = useI18n();
const auth = useAuthStore();
const loading = shallowRef(false);
const error = shallowRef('');
const authorConfig = ref<AuthorConfig | null>(null);
const rememberMe = shallowRef(false);

onMounted(async () => {
  try {
    const saved = storageGet(StorageKeys.AUTH_REMEMBER_ME);
    if (saved) {
      // 与 LoginView 同一份历史数据，可能是 '1' 也可能是 JSON 的 true/false
      rememberMe.value = saved === '1' || saved === 'true';
    }
    authorConfig.value = await getAuthorConfig();
  } catch {
    authorConfig.value = null;
  }
});

async function handleLogin(username: string, password: string) {
  if (!username || !password) {
    error.value = t('auth.needCredentials');
    return;
  }
  loading.value = true;
  error.value = '';
  try {
    const res = await authorLogin({ username, password });
    if (!res.user_info || res.user_info.role_id !== 1) {
      error.value = t('auth.authorOnly');
      return;
    }
    auth.setTokens(res.access_token, res.refresh_token, res.expires_in || 3600);
    auth.setUser(res.user_info);
    // 与 auth.login 同一判据：init() 只认 '1'，决定 token 过期后是否用 refresh token 自动续
    if (rememberMe.value) {
      storageSet(StorageKeys.AUTH_REMEMBER_ME, '1');
    } else {
      storageRemove(StorageKeys.AUTH_REMEMBER_ME);
    }
    router.push('/');
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : t('auth.loginFailed');
  } finally {
    loading.value = false;
  }
}

async function handleSSOLogin(ssoId: number) {
  error.value = '';
  try {
    const res = await initiateSSOLogin(ssoId);
    if (res.redirect_url) {
      if (!isSafeRedirect(res.redirect_url)) {
        error.value = t('auth.ssoRedirectInvalid');
        return;
      }
      localStorage.setItem('author_login', '1');
      window.location.href = res.redirect_url;
    }
  } catch (e: unknown) {
    error.value = e instanceof Error ? e.message : t('auth.ssoFailed');
  }
}

const wrapperStyle = {
  position: 'relative' as const,
  flex: '1',
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
};

const formContainerStyle = {
  width: '100%',
  maxWidth: AppleLayout.cardNarrow,
  ...AppleGlass.standard,
  borderRadius: AppleRadius.card,
  padding: authCardPadding,
  // 同 LoginView：棱线要跟着阴影一起留，否则 AppleGlass.standard 的 edge 被整条顶掉
  boxShadow: `${cv('glass-edge')}, ${cv('shadow-subtle')}`,
  textAlign: 'center' as const };

const rememberStyle = {
  display: 'flex',
  alignItems: 'center' };

const dividerStyle = {
  display: 'flex',
  alignItems: 'center' as const,
  gap: AppleSpace.md,
  marginTop: AppleSpace.md };

const dividerTextStyle = {
  fontFamily: AppleType.microUI.font,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
  whiteSpace: 'nowrap' as const };

const ssoListStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm };

const ssoBtnStyle = {
  width: '100%' };

const cardFooterStyle = { marginTop: AppleSpace.lg, paddingTop: AppleSpace.md, borderTop: `1px solid ${cv('border-light')}` };
const backLinkStyle = { fontFamily: AppleType.microUI.font, fontSize: AppleType.microUI.size, color: cv('text-quaternary'), textDecoration: 'none', transition: AppleTransition.fast };

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
