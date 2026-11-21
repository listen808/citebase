// 导航单一事实源的回归守卫。
// 背景：侧边栏、路由表、面包屑曾三头硬编码，出现过菜单死链
// （/ai/knowledge-graph）、面包屑父级错挂兄弟页面（索引监控←数据仪表盘）。
// 这些用例保证：菜单里出现的每条路径都可解析、分组归属一致。
import { describe, expect, it } from 'vitest';
import { createMemoryHistory, createRouter } from 'vue-router';
import { adminRoutes } from './routes-admin';
import { mainRoutes } from './routes-main';
import { NAV_GROUP_ORDER, collectNavGroups, navGroupTarget } from './navigation';

function makeRouter() {
  return createRouter({
    history: createMemoryHistory(),
    routes: [...mainRoutes, ...adminRoutes],
  });
}

const allRoutes = [...mainRoutes, ...adminRoutes];

describe('导航单一事实源（routes meta → 侧边栏/面包屑）', () => {
  it('带 group 的路由必须有 title 或 titleKey——它是菜单文案与面包屑末级的唯一来源', () => {
    for (const record of allRoutes) {
      if (record.meta?.group) {
        const meta = record.meta as Record<string, unknown>;
        expect(meta.title || meta.titleKey, `路由 ${record.path} 缺少 meta.title / meta.titleKey`).toBeTruthy();
      }
    }
  });

  it('group 值必须在 NAV_GROUP_ORDER 白名单内，防止分组名漂移出侧边栏', () => {
    for (const record of allRoutes) {
      const group = record.meta?.group as string | undefined;
      if (group) {
        expect(NAV_GROUP_ORDER, `路由 ${record.path} 的 group "${group}" 不在白名单`).toContain(group);
      }
    }
  });

  it('侧边栏派生出的每个菜单项路由都可解析——死链守卫', () => {
    const router = makeRouter();
    const groups = collectNavGroups(router, '/', true);
    const visited = new Set<string>();
    for (const group of groups.values()) {
      expect(group.children.length, `分组 "${group.name}" 不应为空`).toBeGreaterThan(0);
      for (const child of group.children) {
        expect(visited.has(child.path), `菜单项 "${child.name}" (${child.path}) 重复`).toBe(false);
        visited.add(child.path);
        expect(router.resolve(child.path).matched.length, `菜单项 "${child.name}" 指向不存在的路由`).toBeGreaterThan(0);
      }
    }
  });

  it('带参数的详情路由不进菜单，但高亮归属到同组最长前缀菜单项', () => {
    const router = makeRouter();
    // /roles/:roleId/users 不应成为菜单项
    for (const group of collectNavGroups(router, '/', true).values()) {
      for (const child of group.children) {
        expect(child.path.includes(':'), `详情路由 ${child.path} 混进了菜单`).toBe(false);
      }
    }
    // 访问 /roles/5/users 时，"角色管理"应高亮
    const groups = collectNavGroups(router, '/roles/42/users', true);
    const roles = groups.get('用户');
    expect(roles?.active).toBe(true);
    expect(roles?.children.find((c) => c.path === '/roles')?.active).toBe(true);
  });

  it('面包屑父级与侧边栏分组一致：历史上的兄弟错挂不再回归', () => {
    // 历史缺陷：索引监控(/search-monitor)的面包屑父级曾是数据仪表盘(/dashboard)，
    // 两者在侧边栏是"数据"组下的平级兄弟。现在面包屑父级由 meta.group 派生，
    // 同组即同父级。
    const dashboard = allRoutes.find((r) => r.path === '/dashboard');
    const monitor = allRoutes.find((r) => r.path === '/search-monitor');
    expect(dashboard?.meta?.group).toBe(monitor?.meta?.group);
    // 同理：邮箱/SSO 配置与系统设置在侧边栏是"配置"组平级项
    const email = allRoutes.find((r) => r.path === '/settings/email');
    const settings = allRoutes.find((r) => r.path === '/settings');
    expect(email?.meta?.group).toBe(settings?.meta?.group);
  });

  it('历史死链 /ai/knowledge-graph 不得出现在任何菜单中', () => {
    const router = makeRouter();
    for (const group of collectNavGroups(router, '/', true).values()) {
      for (const child of group.children) {
        expect(child.path, '死链 /ai/knowledge-graph 回归').not.toBe('/ai/knowledge-graph');
      }
    }
  });

  it('非管理员看不到 admin 分组', () => {
    const router = makeRouter();
    const groups = collectNavGroups(router, '/', false);
    expect(groups.has('数据')).toBe(false);
    expect(groups.has('配置')).toBe(false);
    expect(groups.has('工作区')).toBe(true);
  });

  it('管理控制台路由不得对 user 角色开放——普通用户进入只会得到整页 403', () => {
    // 这些分组下的页面调用的后端接口全部要求管理员（RBAC MinRole>=2 或 RequireAdmin）
    const adminOnlyGroups = ['数据', '用户', '内容', '配置'];
    for (const record of allRoutes) {
      const group = record.meta?.group as string | undefined;
      if (group && adminOnlyGroups.includes(group)) {
        const roles = record.meta?.roles as string[] | undefined;
        expect(roles, `路由 ${record.path} 未声明 roles，守卫将放行所有角色`).toBeTruthy();
        expect(roles, `路由 ${record.path} 对普通用户开放，但其后端接口是 admin-only`).not.toContain('user');
      }
      // /ai/settings 是 AI 管理控制台，其下约 18 个子页全部调用 admin-only 接口，
      // 曾对 user 角色开放导致普通用户点进去整页 403
      if (record.path === '/ai/settings') {
        expect(record.meta?.roles as string[], '/ai/settings 不得对普通用户开放').not.toContain('user');
      }
    }
  });

  it('分组落地页 = 该组第一个菜单项（面包屑"分组"级的点击目标）', () => {
    const router = makeRouter();
    expect(navGroupTarget('AI')).toBe('/ai/workspace');
    expect(navGroupTarget('数据')).toBe('/dashboard');
    expect(navGroupTarget('配置')).toBe('/settings');
    expect(navGroupTarget('工作区')).toBe('/collections');
    // 落地页必须真实存在，且落在自己组内（不出"点了 AI 跳到别的组"的错位）
    for (const groupName of NAV_GROUP_ORDER) {
      const target = navGroupTarget(groupName);
      const group = collectNavGroups(router, '/', true).get(groupName);
      if (!group) continue;
      expect(group.children.some((c) => c.path === target), `${groupName} 落地页 ${target} 不在本组菜单中`).toBe(true);
    }
  });

  it('菜单顺序 = 路由定义顺序（不被 getRoutes 的路径排序打乱）', () => {
    const router = makeRouter();
    const groups = collectNavGroups(router, '/', true);
    expect(groups.get('配置')?.children.map((c) => c.path)).toEqual([
      '/settings', '/settings/email', '/settings/sso', '/settings/oidc', '/settings/api-keys', '/plugins', '/backups', '/data-export',
    ]);
    expect(groups.get('AI')?.children.map((c) => c.path)).toEqual(['/ai/workspace', '/ai/graph', '/ai/settings']);
  });
});
