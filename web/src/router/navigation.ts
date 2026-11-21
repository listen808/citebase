// 导航派生模块：侧边栏（ApSidebar）与面包屑父级（ApPageContainer）的分组信息
// 全部来自路由 meta（group / icon / title），这里只负责把扁平路由表组装成树。
// 动机：此前三套各自硬编码，出现过"面包屑父级是兄弟页面"、"/ai/knowledge-graph"
// 死链、菜单名与页面标题漂移等问题。
import type { Component } from 'vue';
import i18n from '@/i18n';
import type { Router } from 'vue-router';
import { icons } from '@/utils/icons';
import { adminRoutes } from './routes-admin';
import { mainRoutes } from './routes-main';

// 重要：不能用 router.getRoutes() 取菜单顺序——它按路径字典序返回，
// 会把菜单顺序打乱。这里用路由数组的定义顺序（= 期望的菜单顺序）。
const orderedRecords = [...mainRoutes, ...adminRoutes];

/** 分组的展示顺序，同时是合法 group 值的白名单 */
export const NAV_GROUP_ORDER = ['工作区', 'AI', '数据', '用户', '内容', '配置'] as const;
export type NavGroupName = (typeof NAV_GROUP_ORDER)[number];

/** 分组头图标（分组本身不可点击跳转，图标是纯展示语义） */
export const NAV_GROUP_ICONS: Record<NavGroupName, Component> = {
  工作区: icons.Users,
  AI: icons.Zap,
  数据: icons.BarChart3,
  用户: icons.UserCog,
  内容: icons.FolderOpen,
  配置: icons.SlidersHorizontal,
};

export interface NavChild {
  /** 数据键（来自路由 meta.title / 分组名），同时是展开态等状态的 key —— 不翻译 */
  name: string;
  /** 菜单上真正显示的文字：有 meta.titleKey 时走语言包 */
  label: string;
  path: string;
  icon?: Component;
  active: boolean;
  onClick: () => void;
}

export interface NavGroup {
  name: string;
  label: string;
  icon: Component;
  active: boolean;
  children: NavChild[];
}

/**
 * 从路由表派生全部导航分组。
 * - 带 group 的路由成为菜单项（title 为文案、icon 为图标）；
 * - 带参数的详情路由（如 /roles/:roleId/users）不进菜单，但高亮按最长前缀
 *   归属到同组菜单项（/roles/5/users → 角色管理）；
 * - 非 visibleTo(user) 的 admin 路由对非管理员隐藏。
 */
/** 分组头的显示名走语言包；分组字面量本身是数据键（白名单、展开态都按它匹配） */
export const NAV_GROUP_LABEL_KEYS: Record<NavGroupName, string> = {
  '工作区': 'route.group_workspace',
  AI: 'route.group_ai',
  '数据': 'route.group_data',
  '用户': 'route.group_user',
  '内容': 'route.group_content',
  '配置': 'route.group_config',
};

/**
 * 分组字面量是数据键（白名单、展开态、图标映射都按它匹配），显示名必须过这层映射。
 * 直接把 meta.group 渲染到界面上，英文界面就会印出"数据"——面包屑干过这件事。
 */
export function navGroupLabel(group: string): string {
  const key = NAV_GROUP_LABEL_KEYS[group as NavGroupName];
  return key ? i18n.global.t(key) : group;
}

/** 路由 meta → 该显示的文字。titleKey 优先，裸 title 是还没迁过去的路由 */export function navLabelOf(meta: Record<string, unknown> | undefined): string {
  const key = meta?.titleKey as string | undefined;
  if (key) return i18n.global.t(key);
  return (meta?.title as string) || '';
}

export function collectNavGroups(router: Router, currentPath: string, isAdmin: boolean): Map<NavGroupName, NavGroup> {
  const groups = new Map<NavGroupName, NavGroup>();
  for (const groupName of NAV_GROUP_ORDER) {
    groups.set(groupName, { name: groupName, label: navGroupLabel(groupName), icon: NAV_GROUP_ICONS[groupName], active: false, children: [] });
  }

  for (const record of orderedRecords) {
    const groupName = record.meta?.group as NavGroupName | undefined;
    const group = groupName ? groups.get(groupName) : undefined;
    if (!group) continue;
    if (record.path.includes(':')) continue; // 详情页不进菜单
    if (!record.meta?.icon) continue; // 无 icon = 文档/详情页（如 /settings/sso/doc），不进菜单；group 仍供面包屑与高亮归属
    const roles = record.meta?.roles;
    if (!isAdmin && roles && !roles.includes('user')) continue;
    const path = record.path;
    group.children.push({
      name: record.meta?.title || path,
      label: navLabelOf(record.meta) || (record.meta?.title as string) || path,
      path,
      icon: record.meta?.icon,
      onClick: () => router.push(path),
      active: currentPath === path,
    });
  }

  for (const group of groups.values()) {
    group.active = group.children.some((child) => child.active);
    if (group.active) continue;
    // 无精确命中时，把详情路由高亮归属到同组最长前缀的菜单项
    let best: NavChild | null = null;
    for (const child of group.children) {
      if (currentPath.startsWith(`${child.path}/`) && (!best || child.path.length > best.path.length)) {
        best = child;
      }
    }
    if (best) {
      best.active = true;
      group.active = true;
    }
  }

  // 过滤空组（如非管理员时 admin 各组无可见子项）
  for (const [name, group] of groups) {
    if (group.children.length === 0) groups.delete(name);
  }
  return groups;
}

/**
 * 分组的落地页：该分组路由定义中第一个菜单项（与侧边栏折叠组点击跳第一个子项一致）。
 * 侧边栏分组头与面包屑分组层级都用它作为点击目标。
 */
export function navGroupTarget(groupName: string): string {
  for (const record of orderedRecords) {
    if (record.meta?.group === groupName && !record.path.includes(':')) {
      return record.path;
    }
  }
  return '';
}
