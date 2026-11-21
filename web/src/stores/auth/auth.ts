import { defineStore } from 'pinia';
import { ref, shallowRef, computed } from 'vue';
import { login as apiLogin, getCurrentUser, logout as apiLogout, refreshToken as apiRefreshToken } from '@/api/modules/auth/auth';
import { getApiErrorStatus } from '@/api/client';
import type { LoginRequest } from '@/api/modules/auth/auth';
import type { User } from '@/types/models';
import { ROLE } from '@/utils/core/constants';
import { storageGet, storageSet, storageRemove, StorageKeys } from '@/utils/core/storage';
import { useUIStore } from '@/stores/app/ui';
import { logError } from '@/utils/core/error-logger';

const MAX_REFRESH_RETRIES = 3;

function readToken(): string {
  return storageGet(StorageKeys.AUTH_TOKEN) || '';
}

function readRefreshToken(): string {
  return storageGet(StorageKeys.AUTH_REFRESH) || '';
}

function readExpiresAt(): number {
  const val = storageGet(StorageKeys.AUTH_EXPIRES_AT);
  return val ? Number(val) : 0;
}

function _writeTokens(accessToken: string, refreshToken: string, expiresAt: number) {
  storageSet(StorageKeys.AUTH_TOKEN, accessToken);
  storageSet(StorageKeys.AUTH_REFRESH, refreshToken);
  storageSet(StorageKeys.AUTH_EXPIRES_AT, String(expiresAt));
}

function clearAuthStorage() {
  storageRemove(StorageKeys.AUTH_TOKEN);
  storageRemove(StorageKeys.AUTH_REFRESH);
  storageRemove(StorageKeys.AUTH_EXPIRES_AT);
  storageRemove(StorageKeys.AUTH_REMEMBER_ME);
  _syncAuthCookie('');
}

// _syncAuthCookie 把令牌镜像进 cookie，供"浏览器自己发起的请求"带身份：
// 图片 / 附件 / 导出这类直接进地址栏或 <img src> 的请求发不出 Authorization 头，
// 后端 internal/middleware/auth.go 因此会读这条 cookie（优先级 Bearer > ?token= > cookie）。
//
// 必须说清楚的代价：这条 cookie 是 JS 写的，所以不可能 HttpOnly ——
// 任何 XSS 都能从 document.cookie 读走整枚 JWT。真正的修法是登录接口自己
// 下发 HttpOnly Set-Cookie、前端不再持有令牌副本，那是一次认证流程改造，不是这里能顺手做完的。
// 现阶段能做的两件事：只在 https 页面下加 Secure（别在明文 HTTP 上回传令牌），
// 以及保持 SameSite=Lax（跨站的 POST 带不上这条 cookie）。
function _syncAuthCookie(token: string) {
  if (typeof document === 'undefined') return;
  const secure = typeof location !== 'undefined' && location.protocol === 'https:' ? '; Secure' : '';
  if (token) {
    const maxAge = 7 * 24 * 60 * 60;
    document.cookie = `auth_token=${encodeURIComponent(token)}; path=/; max-age=${maxAge}; SameSite=Lax${secure}`;
  } else {
    // 清除时属性必须与写入时一致（path/SameSite/Secure），否则浏览器视为另一条 cookie，清不掉。
    document.cookie = `auth_token=; path=/; max-age=0; SameSite=Lax${secure}`;
  }
}

export const useAuthStore = defineStore('auth', () => {
  const _token = ref<string>(readToken());
  const _refreshToken = ref<string>(readRefreshToken());
  const _expiresAt = ref<number>(readExpiresAt());
  const user = ref<User | null>(null);
  const loading = shallowRef(false);
  const loginError = shallowRef('');
  const refreshRetryCount = shallowRef<number>(0);
  const isFetchingUser = shallowRef(false);

  const token = computed({
    get: () => _token.value,
    set: (val: string) => {
      _token.value = val;
      if (val) {
        storageSet(StorageKeys.AUTH_TOKEN, val);
      } else {
        storageRemove(StorageKeys.AUTH_TOKEN);
      }
      _syncAuthCookie(val);
    },
  });

  const refreshTokenVal = computed({
    get: () => _refreshToken.value,
    set: (val: string) => {
      _refreshToken.value = val;
      if (val) {
        storageSet(StorageKeys.AUTH_REFRESH, val);
      } else {
        storageRemove(StorageKeys.AUTH_REFRESH);
      }
    },
  });

  const tokenExpiresAt = computed({
    get: () => _expiresAt.value,
    set: (val: number) => {
      _expiresAt.value = val;
      storageSet(StorageKeys.AUTH_EXPIRES_AT, String(val));
    },
  });

  const isLoggedIn = computed(() => !!token.value && !!user.value);
  const isAdmin = computed(() => user.value?.role_id === ROLE.SUPER_ADMIN || user.value?.role_id === ROLE.ADMIN);
  const isSuperAdmin = computed(() => user.value?.role_id === ROLE.SUPER_ADMIN);
  function isEditor(spacePrivilege: number | undefined): boolean {
    if (isSuperAdmin.value) return true;
    return spacePrivilege !== undefined && spacePrivilege >= 1;
  }

  function isSpaceAdmin(spacePrivilege: number | undefined): boolean {
    if (isSuperAdmin.value) return true;
    return spacePrivilege !== undefined && spacePrivilege >= 2;
  }

  function _isTokenExpired() {
    return Date.now() >= tokenExpiresAt.value;
  }

  function isTokenAboutToExpire() {
    return Date.now() >= (tokenExpiresAt.value - 5 * 60 * 1000);
  }

  async function login(credentials: LoginRequest, rememberMe = false) {
    loading.value = true;
    loginError.value = '';
    try {
      const res = await apiLogin(credentials);
      const expiresIn = res.expires_in || 24 * 3600;
      const expiresAt = Date.now() + expiresIn * 1000;

      token.value = res.access_token;
      refreshTokenVal.value = res.refresh_token;
      tokenExpiresAt.value = expiresAt;
      refreshRetryCount.value = 0;

      if (rememberMe) {
        storageSet(StorageKeys.AUTH_REMEMBER_ME, '1');
      } else {
        storageRemove(StorageKeys.AUTH_REMEMBER_ME);
      }

      await fetchUser();
      return true;
    } catch (e: unknown) {
      const err = e as { response?: { data?: { message?: string } }; message?: string } | undefined;
      const msg = err?.response?.data?.message || err?.message || '登录失败';
      loginError.value = msg;
      return false;
    } finally {
      loading.value = false;
    }
  }

  // 最近一次拉用户信息的失败原因（HTTP 状态码；网络层失败记 0）。
  // 守卫要靠它区分「凭据真失效」与「后端忙/超时」——两者对用户是完全不同的两件事。
  const userFetchFailure = ref<number | null>(null);

  async function fetchUser(silent = false) {
    try {
      const userInfo = await getCurrentUser();
      user.value = userInfo;
      userFetchFailure.value = null;
      // 偏好跟账号走：/auth/me 带回来的 preferences 交给 ui store 判谁赢（纯函数在 preference-sync）。
      // 不写这行 = 后端那一列白存；顺序也不能挪到 catch 之后 —— 失败时什么都不该同步。
      useUIStore().applyServerPreferences(userInfo.preferences);
    } catch (e) {
      user.value = null;
      userFetchFailure.value = getApiErrorStatus(e) ?? 0;
      if (!silent) {
        throw new Error('获取用户信息失败', { cause: e });
      }
    }
  }

  async function tryRefreshToken() {
    if (!refreshTokenVal.value) return false;

    if (refreshRetryCount.value >= MAX_REFRESH_RETRIES) {
      refreshRetryCount.value = 0;
      logout();
      return false;
    }

    try {
      refreshRetryCount.value++;
      const res = await apiRefreshToken(refreshTokenVal.value);
      const expiresIn = res.expires_in || 3600;
      const expiresAt = Date.now() + expiresIn * 1000;

      token.value = res.access_token;
      tokenExpiresAt.value = expiresAt;
      refreshRetryCount.value = 0;

      return true;
    } catch (error) {
      logError({ module: 'AuthStore', operation: 'fetchUser' }, error);
      logout();
      return false;
    }
  }

  function setUser(userInfo: User) {
    user.value = userInfo;
  }

  function setTokens(accessToken: string, refreshToken: string, expiresIn: number) {
    const expiresAt = Date.now() + expiresIn * 1000;
    token.value = accessToken;
    refreshTokenVal.value = refreshToken;
    tokenExpiresAt.value = expiresAt;
    refreshRetryCount.value = 0;
  }

  function logout() {
    apiLogout().catch((e) => {
      logError({ module: 'AuthStore', operation: 'logout' }, e);
    });
    token.value = '';
    refreshTokenVal.value = '';
    tokenExpiresAt.value = 0;
    user.value = null;
    refreshRetryCount.value = 0;
    clearAuthStorage();
    // P1-11: 退出登录必须同时失效上一个用户的业务缓存（文档正文缓存、空间缓存、
    // 浏览历史、最近文档、搜索历史），否则换账号后前端缓存会绕过身份复核串数据
    void clearUserScopedCaches();
  }

  async function clearUserScopedCaches() {
    try {
      const [{ useDocumentStore }, { useSpaceStore }, { useHistoryStore }] = await Promise.all([
        import('@/stores/workspace/document'),
        import('@/stores/workspace/space'),
        import('@/stores/workspace/history'),
      ]);
      const docStore = useDocumentStore();
      docStore.reset();
      docStore.invalidateDocCache();
      useSpaceStore().invalidateSpacesCache();
      useHistoryStore().clear();
      storageRemove(StorageKeys.RECENT_DOCS);
      storageRemove(StorageKeys.SEARCH_SPOTLIGHT_HISTORY);
    } catch (e) {
      logError({ module: 'AuthStore', operation: 'clearUserScopedCaches' }, e);
    }
  }

  async function init() {
    const savedToken = readToken();
    const savedRefresh = readRefreshToken();
    const savedExpiresAt = readExpiresAt();
    const isRemembered = storageGet(StorageKeys.AUTH_REMEMBER_ME) === '1';

    if (!savedToken && !savedRefresh) {
      _syncAuthCookie('');
      return;
    }

    _token.value = savedToken;
    _refreshToken.value = savedRefresh;
    _expiresAt.value = savedExpiresAt;
    _syncAuthCookie(savedToken);

    const expired = Date.now() >= savedExpiresAt;

    if (!expired) {
      if (isTokenAboutToExpire()) {
        const refreshed = await tryRefreshToken();
        if (!refreshed) return;
      }
      await fetchUser(true);
    } else if (savedRefresh && isRemembered) {
      const refreshed = await tryRefreshToken();
      if (refreshed) {
        await fetchUser(true);
      }
    } else {
      logout();
    }
  }

  return {
    token,
    refreshTokenVal,
    user,
    loading,
    loginError,
    tokenExpiresAt,
    refreshRetryCount,
    isFetchingUser,
    userFetchFailure,
    isLoggedIn,
    isAdmin,
    isSuperAdmin,
    isEditor,
    isSpaceAdmin,
    login,
    setTokens,
    setUser,
    fetchUser,
    logout,
    init,
  };
});
