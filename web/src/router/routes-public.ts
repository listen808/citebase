import type { RouteRecordRaw } from 'vue-router';

export const publicRoutes: RouteRecordRaw[] = [
  {
    path: '/login',
    name: 'login',
    component: () => import('@/views/auth/LoginView.vue'),
    meta: { public: true, layout: 'blank', titleKey: 'auth.login' },
  },
  {
    path: '/author-login',
    name: 'author-login',
    component: () => import('@/views/auth/AuthorLoginView.vue'),
    meta: { public: true, layout: 'blank', titleKey: 'auth.authorLogin' },
  },
  {
    path: '/install',
    name: 'install',
    component: () => import('@/views/auth/InstallView.vue'),
    meta: { public: true, layout: 'blank', titleKey: 'chrome.titleInstall' },
  },
  {
    path: '/auth/sso/callback',
    name: 'sso-callback',
    component: () => import('@/views/auth/SSOCallbackView.vue'),
    meta: { public: true, layout: 'blank', titleKey: 'chrome.titleSso' },
  },
  {
    path: '/share/:id',
    name: 'share',
    component: () => import('@/views/share/ShareView.vue'),
    meta: { public: true, layout: 'blank', titleKey: 'chrome.titleShare' },
  },
];
