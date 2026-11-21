import type { RouteRecordRaw } from 'vue-router';
import { icons } from '@/utils/icons';

// meta.group / meta.icon 是导航的单一事实源（见 routes-main.ts 顶部说明）。
// 分组归属与侧边栏一致：数据 / 用户 / 内容 / 配置。
export const adminRoutes: RouteRecordRaw[] = [
  {
    path: '/dashboard',
    name: 'dashboard',
    component: () => import('@/views/admin/dashboard/DashboardView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.dashboard', group: '数据', icon: icons.BarChart3 },
  },
  {
    path: '/users',
    name: 'users',
    component: () => import('@/views/admin/users/UserManageView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.users', group: '用户', icon: icons.Users },
  },
  {
    path: '/roles',
    name: 'roles',
    component: () => import('@/views/admin/users/RoleManageView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.roles', group: '用户', icon: icons.Shield },
  },
  {
    // 详情页不进菜单；带 group 使面包屑/高亮归属"用户"组
    path: '/roles/:roleId/users',
    name: 'role-users',
    component: () => import('@/views/admin/users/RoleUsersView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.role_users', group: '用户' },
  },
  {
    path: '/privileges',
    name: 'privileges',
    component: () => import('@/views/admin/users/PrivilegeListView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.privileges', group: '用户', icon: icons.LockKeyhole },
  },
  {
    path: '/spaces',
    name: 'spaces',
    component: () => import('@/views/admin/contents/SpaceManageView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.spaces', group: '内容', icon: icons.FolderOpen },
  },
  {
    path: '/logs',
    name: 'logs',
    component: () => import('@/views/admin/system/LogView.vue'),
    // 日志原用 FileText，与「模板」撞字（两个功能同一符号）；日志本身是一条条流水记录，
    // 收编到 ScrollText（卷轴 = 流水记录），与模板的 LayoutTemplate 各占一枚专属符号。
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.logs', group: '数据', icon: icons.ScrollText },
  },
  {
    path: '/links',
    name: 'links',
    component: () => import('@/views/admin/contents/LinkManageView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.links', group: '内容', icon: icons.Link },
  },
  {
    path: '/contacts',
    name: 'contacts',
    component: () => import('@/views/admin/contents/ContactManageView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.contacts', group: '内容', icon: icons.Contact },
  },
  {
    path: '/media',
    name: 'media',
    component: () => import('@/views/admin/contents/MediaManageView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.media', group: '内容', icon: icons.Image },
  },
  {
    path: '/settings',
    name: 'settings',
    component: () => import('@/views/admin/system/SettingsView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.settings', group: '配置', icon: icons.Settings },
  },
  {
    path: '/settings/email',
    name: 'settings-email',
    component: () => import('@/views/admin/system/EmailConfigView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.settings_email', group: '配置', icon: icons.Mail },
  },
  {
    path: '/settings/sso',
    name: 'settings-sso',
    component: () => import('@/views/admin/system/SSOConfigView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.settings_sso', group: '配置', icon: icons.Fingerprint },
  },
  {
    // 文档页不进菜单（无 icon），面包屑由视图显式指定父级"SSO 配置"；
    // group 照旧给「配置」——面包屑层级要与其他设置页一致（审计 B10），只是没 icon 就不进侧栏
    path: '/settings/sso/doc',
    name: 'settings-sso-doc',
    component: () => import('@/views/admin/system/SSODocView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.settings_sso_doc', group: '配置' },
  },
  {
    // OIDC 提供方：后端 5 条 admin 路由早已注册、登录页也在读，此前没有任何创建入口
    path: '/settings/oidc',
    name: 'settings-oidc',
    component: () => import('@/views/admin/system/OIDCProviderView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.settings_oidc', group: '配置', icon: icons.Globe },
  },
  {
    path: '/settings/api-keys',
    name: 'settings-api-keys',
    component: () => import('@/views/admin/system/ApiKeyManageView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.settings_api_keys', group: '配置', icon: icons.Key },
  },
  {
    path: '/plugins',
    name: 'plugins',
    component: () => import('@/views/admin/system/PluginManageView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.plugins', group: '配置', icon: icons.Puzzle },
  },
  {
    path: '/search-monitor',
    name: 'search-monitor',
    component: () => import('@/views/admin/dashboard/SearchMonitorView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.search_monitor', group: '数据', icon: icons.Database },
  },
  {
    path: '/backups',
    name: 'backups',
    component: () => import('@/views/admin/system/BackupView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.backups', group: '配置', icon: icons.DatabaseBackup },
  },
  {
    // 归类修正：导出与合规是配置/治理动作，不是数据分析（此前误放"数据"组）
    path: '/data-export',
    name: 'data-export',
    component: () => import('@/views/admin/system/DataExportView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.data_export', group: '配置', icon: icons.FileCheck },
  },
  {
    path: '/search-ops',
    name: 'search-ops',
    component: () => import('@/views/search/SearchOpsView.vue'),
    meta: { roles: ['admin', 'super_admin'], layout: 'main', titleKey: 'route.search_ops', group: '数据', icon: icons.BookOpen },
  },
  // === 向后兼容重定向（旧 -manage 后缀路径 → 新顶级复数路径） ===
  { path: '/spaces-manage', redirect: '/spaces' },
  { path: '/links-manage', redirect: '/links' },
  { path: '/contacts-manage', redirect: '/contacts' },
  { path: '/media-manage', redirect: '/media' },
  { path: '/dashboard/search', redirect: '/search-monitor' },
];
