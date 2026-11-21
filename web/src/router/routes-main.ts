import type { RouteRecordRaw } from 'vue-router';
import { icons } from '@/utils/icons';

// meta.group / meta.icon 是导航的单一事实源：
// 侧边栏分组（ApSidebar）与面包屑父级（ApPageContainer）都从这里派生，
// 不再各自硬编码，避免"面包屑父级是兄弟页面""菜单死链"这类漂移。
export const mainRoutes: RouteRecordRaw[] = [
  {
    path: '/',
    name: 'home',
    component: () => import('@/views/workspace/home/HomeView.vue'),
    meta: { layout: 'main', titleKey: 'route.home' },
  },
  {
    path: '/collections',
    name: 'collections',
    component: () => import('@/views/social/CollectionsView.vue'),
    meta: { layout: 'main', titleKey: 'route.collections', group: '工作区', icon: icons.Bookmark },
  },
  {
    path: '/follow',
    name: 'follow',
    component: () => import('@/views/social/FollowView.vue'),
    meta: { layout: 'main', titleKey: 'route.follow', group: '工作区', icon: icons.UserCheck },
  },
  {
    path: '/activities',
    name: 'activities',
    component: () => import('@/views/social/ActivitiesView.vue'),
    meta: { layout: 'main', titleKey: 'route.activities', group: '工作区', icon: icons.Activity },
  },
  {
    path: '/notifications',
    name: 'notifications',
    component: () => import('@/views/social/NotificationsView.vue'),
    meta: { layout: 'main', titleKey: 'route.notifications', group: '工作区', icon: icons.Bell },
  },
  {
    path: '/saved-searches',
    name: 'saved-searches',
    component: () => import('@/views/search/SavedSearchView.vue'),
    meta: { layout: 'main', titleKey: 'route.saved_searches', group: '工作区', icon: icons.BookmarkCheck },
  },
  {
    path: '/webhooks',
    name: 'webhooks',
    component: () => import('@/views/social/WebhookManageView.vue'),
    meta: { layout: 'main', titleKey: 'route.webhooks', group: '工作区', icon: icons.Send },
  },
  {
    path: '/tags',
    name: 'tags',
    component: () => import('@/views/workspace/TagManageView.vue'),
    meta: { layout: 'main', titleKey: 'route.tags', group: '工作区', icon: icons.Tags },
  },
  {
    path: '/templates',
    name: 'templates',
    component: () => import('@/views/workspace/TemplateManageView.vue'),
    // 模板原用 FileText，与「日志」撞字；模板是"版式骨架"，收编到 LayoutTemplate 专属符号。
    meta: { layout: 'main', titleKey: 'route.templates', group: '工作区', icon: icons.LayoutTemplate },
  },
  {
    // 2026-10-26 从孤儿路由 /space/:spaceId/views 改为静态路径接入侧栏：
    // 导航派生会把含参数的 path 排除出菜单（navigation.ts），而这个页面自带空间切换器
    // （spaceId 缺省时自动落到第一个空间），本来就是独立功能页的形态。
    // 旧路径全项目零引用（审计确认），无需兼容重定向。
    path: '/doc-views',
    name: 'document-views',
    component: () => import('@/views/workspace/DocumentViewsView.vue'),
    meta: { roles: ['user', 'admin', 'super_admin'], layout: 'main', titleKey: 'route.document_views', group: '工作区', icon: icons.LayoutGrid },
  },
  {
    path: '/doc-collab',
    name: 'doc-collab',
    component: () => import('@/views/workspace/DocumentCollabCenterView.vue'),
    meta: { roles: ['user', 'admin', 'super_admin'], layout: 'main', titleKey: 'route.doc_collab', group: '工作区', icon: icons.ShieldCheck },
  },
  // Git 同步已下线（后端 PullFromGit 随割接 P3 移除），路由与视图一并删除
  // === AI 三大主页面 ===
  // AI 中心 - 重定向到对话
  { path: '/ai', redirect: '/ai/workspace' },
  // 1. AI 对话
  {
    path: '/ai/workspace',
    name: 'ai-workspace',
    component: () => import('@/views/ai/AIChatView.vue'),
    meta: { roles: ['user', 'admin', 'super_admin'], layout: 'main', titleKey: 'route.ai_workspace', group: 'AI', icon: icons.Brain },
  },
  // 2. 知识图谱
  {
    path: '/ai/graph',
    name: 'ai-graph',
    component: () => import('@/views/ai/AIKnowledgeGraphView.vue'),
    meta: { roles: ['user', 'admin', 'super_admin'], layout: 'main', titleKey: 'route.ai_graph', group: 'AI', icon: icons.Network },
  },
  // 3. AI 设置（管理控制台：其下子页调用的全是 admin-only 接口，
  //    对普通用户开放只会得到整页 403，收权到管理员）
  {
    path: '/ai/settings',
    name: 'ai-settings',
    component: () => import('@/views/ai/AISettingsLayout.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.ai_settings', group: 'AI', icon: icons.SlidersHorizontal },
  },
  {
    path: '/search',
    name: 'search',
    component: () => import('@/views/search/SearchView.vue'),
    meta: { layout: 'main', titleKey: 'route.search' },
  },
  {
    path: '/space/:spaceId',
    name: 'space',
    component: () => import('@/views/workspace/SpaceView.vue'),
    meta: { layout: 'main', titleKey: 'route.space' },
  },
  {
    path: '/space/:spaceId/doc/:docId',
    name: 'document',
    component: () => import('@/views/workspace/SpaceView.vue'),
    meta: { layout: 'main', titleKey: 'route.document' },
  },
  {
    path: '/space/:spaceId/doc/:docId/edit',
    name: 'document-edit',
    component: () => import('@/views/workspace/editor/DocumentEditView.vue'),
    meta: { roles: ['user', 'admin', 'super_admin'], layout: 'main', titleKey: 'route.document_edit' },
  },
  {
    path: '/profile',
    name: 'profile',
    component: () => import('@/views/user/ProfileSettingsView.vue'),
    meta: { roles: ['user', 'admin', 'super_admin'], layout: 'main', titleKey: 'route.profile' },
  },
  {
    path: '/user/:userId',
    name: 'user',
    component: () => import('@/views/user/UserProfileView.vue'),
    meta: { layout: 'main', titleKey: 'route.user' },
  },
  {
    path: '/about',
    name: 'about',
    component: () => import('@/views/common/AboutView.vue'),
    meta: { layout: 'main', titleKey: 'route.about' },
  },
  {
    path: '/error',
    name: 'server-error',
    component: () => import('@/views/common/ServerErrorView.vue'),
    meta: { public: true, layout: 'blank', titleKey: 'route.server_error' },
  },
];
