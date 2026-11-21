package auth

import "testing"

// TestResolveRoleIDNeverMintsSuperAdmin 钉住 IdP 自动建号的角色档位。
//
// 原写法是 "admin" → 1、"editor" → 2。而 aw_role 里 role 1 的原文描述是
// 「系统内置超级管理员，拥有所有权限」，AutoRBAC 一认到 role 1 就跳过全部权限判定；
// 于是管理页那个写着「管理员（admin）」的下拉，实际发的是超级管理员 ——
// 任何从这个 IdP 首次登录的人直接拿到全站最高档，且和 RBAC 那套漏登记洞叠加时
// 连"默认拒绝"都拦不住（超管无条件放行）。
//
// role 2 才是这个系统里的"管理员"（「拥有大部分管理权限」）；
// "editor" 在本系统没有对应角色（种子只有 1/2/3 三行），落到 role 3，
// 文档编辑权由空间成员权限决定，不需要全局管理角色。
func TestResolveRoleIDNeverMintsSuperAdmin(t *testing.T) {
	s := NewOIDCService()
	for _, c := range []struct {
		name string
		want int
	}{
		{"admin", 2},
		{"editor", 3},
		{"viewer", 3},
		{"", 3},
		{"anything-unknown", 3},
	} {
		if got := s.resolveRoleID(c.name); got != c.want {
			t.Errorf("resolveRoleID(%q) = %d，期望 %d", c.name, got, c.want)
		}
	}
}

// TestResolveRoleIDSuperAdminIsUnreachable 是上一条的正面表述，单独写是因为它才是
// 这条修复的全部意义：这条函数在任何输入下都不该返回 1 —— 超管只能由现有超管授予。
func TestResolveRoleIDSuperAdminIsUnreachable(t *testing.T) {
	s := NewOIDCService()
	for _, name := range []string{"admin", "superadmin", "super", "super_admin", "root", "SYSTEM", "Administrator", "editor", "viewer", "未知"} {
		if got := s.resolveRoleID(name); got == 1 {
			t.Fatalf("resolveRoleID(%q) 返回 1 —— IdP 自动建号不得产生超级管理员", name)
		}
	}
}
