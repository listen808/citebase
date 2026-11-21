import type { Router } from 'vue-router';
import { useAuthStore } from '@/stores/auth/auth';
import { useHistoryStore } from '@/stores/workspace/history';
import { useDocumentStore } from '@/stores/workspace/document';
import { useUIStore } from '@/stores/app/ui';
import { useAIStore } from '@/stores/ai/ai';
import { getInstallState } from '@/api/modules/auth/install';
import { storageGet, storageRemove, StorageKeys } from '@/utils/core/storage';
import { logger } from '@/utils/logger';
import i18n from '@/i18n';
import { siteDocumentTitle } from '@/utils/core/site-brand';

let installStateCache: { checked: boolean; installed: boolean; timestamp: number } = { checked: false, installed: false, timestamp: 0 };
const INSTALL_CACHE_TTL = 60 * 1000;


export function clearInstallCache() {
  installStateCache = { checked: false, installed: false, timestamp: 0 };
}

/**
 * 只有服务端明确否定这份凭据（401/403）才判定「登录已失效」。
 * 429 限流、5xx、超时、断网、以及拿不到状态码的网络层失败，都只是「暂时不知道你是谁」——
 * 把它们当被拒会清掉还有效的 token，用户被迫重新登录，对值守型产品是最坏的一条路径。
 */
export function isCredentialRejected(status: number | null | undefined): boolean {
  return status === 401 || status === 403;
}

// 身份拉取失败但会话没被否掉时，延迟再试一次：用户停在页面上，不该永远卡在「不知道你是谁」。
// 只留一枚挂起的定时器，避免连续导航把重试堆成风暴。
let identityRetryTimer: ReturnType<typeof setTimeout> | null = null;
function scheduleIdentityRetry(auth: ReturnType<typeof useAuthStore>, delay = 3000) {
  if (identityRetryTimer) return;
  identityRetryTimer = setTimeout(() => {
    identityRetryTimer = null;
    if (auth.user) return;
    void auth.fetchUser(true).catch(() => { /* 仍然失败就交给下一次导航或请求 */ });
  }, delay);
}

export function setupRouterGuards(router: Router) {
  router.beforeEach(async (to, from) => {
    // 保存离开前的状态到 historyStore（如果有实际路径变化）
    // 注意：SpaceView.vue 中在选择文档时已经有显式 push，这里主要处理其他页面
    if (to.path !== from.path 
        && from.path !== '/' 
        && from.matched.length > 0) {
      try {
        const history = useHistoryStore();
        const docStore = useDocumentStore();
        const ui = useUIStore();
        
        history.push({
          path: from.fullPath,
          scrollTop: window.scrollY,
          expandedNodes: [...docStore.expandedNodes],
          sidebarCollapsed: ui.sidebarCollapsed,
        });
      } catch {
        // 静默失败，不影响导航
      }
    }

    const auth = useAuthStore();

    // 只在「确实拿到了 401/403」时判定凭据失效。429/5xx/超时/断网是后端忙或网络抖，
    // 不是你没登录——对值守型产品来说，把这种失败当被拒会直接把人踢下线、还得重输口令。
    const token = storageGet(StorageKeys.AUTH_TOKEN);
    let identityUnavailable = false;
    if (token && !auth.user && !auth.isFetchingUser) {
      auth.isFetchingUser = true;
      let timedOut = false;
      try {
        await Promise.race([
          auth.fetchUser(true),
          new Promise((_, reject) => setTimeout(() => reject(new Error(i18n.global.t('chrome.fetchUserTimeout'))), 10000)),
        ]).catch((e) => {
          // fetchUser(silent) 自己吞掉异常并记状态；只有超时/主动抛的才会到这里
          timedOut = true;
          auth.userFetchFailure ??= 0;
          void e;
        });
      } finally {
        auth.isFetchingUser = false;
      }
      const status = timedOut ? 0 : (auth.userFetchFailure ?? 0);
      if (!auth.user && status !== null) {
        if (isCredentialRejected(status)) {
          const ui = useUIStore();
          ui.showToast(i18n.global.t('chrome.sessionExpired'), 'warning');
          storageRemove(StorageKeys.AUTH_TOKEN);
          storageRemove(StorageKeys.AUTH_REFRESH);
          storageRemove(StorageKeys.AUTH_EXPIRES_AT);
          auth.token = '';
          auth.refreshTokenVal = '';
          auth.tokenExpiresAt = 0;
          auth.user = null;
        } else {
          // 会话原样保留，本次导航放行到不依赖角色的页面；下一次请求成功后状态自然恢复。
          // 顺手补一次延迟重试，避免用户停在页面上就再也拿不到身份。
          identityUnavailable = true;
          scheduleIdentityRetry(auth);
        }
      }
    }

    if (to.path === '/install' && auth.isLoggedIn) {
      return '/';
    }

    if (to.meta.public) {
      return true;
    }

    // 后端忙/超时/断网：会话还在，身份未知不等于被拒。放行了也只是渲染外壳——
    // 真正的权限边界在服务端，每个接口自己会 401/403，客户端守卫从来不是安全边界。
    // 例外：带 meta.roles 的页面在身份未知时直接放行，用户会看到自己未必有权的管理页骨架；
    // 这里挡回首页，scheduleIdentityRetry 拿到身份后再进。
    if (identityUnavailable) {
      if (to.meta.roles && to.path !== '/') {
        useUIStore().showToast(i18n.global.t('chrome.identityPending'), 'info');
        return '/';
      }
      return true;
    }

    if (!auth.isLoggedIn) {
      const cacheExpired = !installStateCache.checked || (Date.now() - installStateCache.timestamp > INSTALL_CACHE_TTL);
      if (cacheExpired) {
        try {
          const state = await getInstallState();
          installStateCache = { checked: true, installed: !!state?.admin_exists, timestamp: Date.now() };
        } catch (error) {
          // API失败时，保持现有缓存状态或假设已安装
          // 避免因网络错误导致错误跳转到安装页面
          logger.error('Failed to check install state', error);
          if (!installStateCache.checked) {
            // 如果从未检查过，假设已安装，让用户去登录页
            installStateCache = { checked: true, installed: true, timestamp: Date.now() };
          }
        }
      }
      if (!installStateCache.installed) {
        if (to.path !== '/install') return '/install';
        return true;
      }
      if (to.path !== '/login') {
        return { path: '/login', query: { redirect: to.fullPath } };
      }
      return true;
    }

    if (to.meta.roles && Array.isArray(to.meta.roles)) {
      const userRoles: ('admin' | 'super_admin' | 'user')[] = ['user'];
      if (auth.isAdmin) userRoles.push('admin');
      if (auth.isSuperAdmin) userRoles.push('super_admin');
      const requiredRoles = to.meta.roles;
      if (!requiredRoles.some((r) => userRoles.includes(r))) {
        const ui = useUIStore();
        ui.showToast(i18n.global.t('chrome.noAccess'), 'error');
        return '/';
      }
    }

    if (to.meta.permissions && Array.isArray(to.meta.permissions)) {
      const userPerms = auth.user?.permissions || [];
      const requiredPerms = to.meta.permissions as string[];
      const hasAllPerms = requiredPerms.every((p) => userPerms.includes(p));
      if (!hasAllPerms) {
        const ui = useUIStore();
        ui.showToast(i18n.global.t('chrome.noAccess'), 'error');
        return '/';
      }
    }

    // AI 页面守卫：未启用时禁止访问
    if (to.path.startsWith('/ai')) {
      const aiStore = useAIStore();
      if (!aiStore.configLoaded) {
        try {
          await Promise.race([
            aiStore.loadConfig(),
            new Promise((resolve) => setTimeout(resolve, 2000)),
          ]);
        } catch {
          // 加载失败，使用默认 disabled 状态
        }
      }
      if (!aiStore.enabled) {
        const ui = useUIStore();
        // 「配置没加载到」与「真的没启用」分开说，别让网络抖动背「AI 未开启」的锅。
        ui.showToast(i18n.global.t(aiStore.configError ? 'chrome.aiConfigLoadFailed' : 'chrome.aiDisabled'), 'error');
        return '/';
      }
    }

    return true;
  });

  router.afterEach((to, from) => {
    const history = useHistoryStore();
    const state = history.getCurrentState();

    if (to.path === from.path && state && state.path === to.path && state.scrollTop !== undefined) {
      requestAnimationFrame(() => {
        window.scrollTo({ top: state.scrollTop });
      });
    }

    // titleKey 优先：路由文案进了语言包之后，标签页标题才不会在英文界面里仍然是中文。
    // 保留 title 这条路是因为其余路由还在按域迁移中，一批只动这一族。
    const meta = to.meta as { title?: string; titleKey?: string };
    const title = meta.titleKey ? i18n.global.t(meta.titleKey) : meta.title;
    document.title = siteDocumentTitle(title);
  });
}
