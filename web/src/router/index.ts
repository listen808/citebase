import type { Component } from 'vue';
import { createRouter, createWebHistory } from 'vue-router';
import { publicRoutes } from './routes-public';
import { mainRoutes } from './routes-main';
import { adminRoutes } from './routes-admin';
import { setupRouterGuards } from './guards';

declare module 'vue-router' {
  interface RouteMeta {
    title?: string;
    roles?: ('admin' | 'super_admin' | 'user')[];
    permissions?: string[];
    /** 公开页（登录/分享/状态等），守卫据此放行未登录访问；没标 public 的路由都需要登录 */
    public?: boolean;
    layout?: 'main' | 'blank';
    /** 导航分组（侧边栏分区名 + 面包屑父级），缺省表示不出现在侧边栏 */
    group?: string;
    /** 侧边栏菜单图标 */
    icon?: Component;
  }
}

const router = createRouter({
  history: createWebHistory(import.meta.env.BASE_URL),
  routes: [
    ...publicRoutes,
    ...mainRoutes,
    ...adminRoutes,
    {
      path: '/:pathMatch(.*)*',
      name: 'not-found',
      component: () => import('@/views/common/NotFoundView.vue'),
      meta: { public: true, layout: 'blank', titleKey: 'route.not_found' },
    },
  ],
  scrollBehavior(_to, _from, savedPosition) {
    if (savedPosition) {
      return savedPosition;
    }
    return { top: 0 };
  },
});

setupRouterGuards(router);

// 路由级错误兜底：此前 /error 页注册了却没有任何代码跳它（审计 2026-10-26 P2）——
// 懒加载 chunk 失效（发版后旧 hash 404）、守卫抛错这类路由级失败，用户只会看到卡死。
// - 防递归：错误发生在 /error 自身时直接放行，否则循环重定向；
// - 初始导航（to 为 undefined）也不跳：此时页面还没挂载，跳了也白跳。
router.onError((error, to) => {
  if (!to || to.path === '/error') return;
  console.error('[router] navigation failed:', error);
  router.push({ path: '/error', query: { from: to.fullPath } });
});

export default router;
