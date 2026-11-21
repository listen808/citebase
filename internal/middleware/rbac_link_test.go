package middleware

import "testing"

// resolvePrivilege 复刻 AutoRBAC 的判定：按特异性排序取第一条命中的规则，
// 没有命中再看是否落在 adminPathPrefixes 上（落上=要管理员，否则放行给 Service 层）。
func resolvePrivilege(t *testing.T, method, path string) (int, bool) {
	t.Helper()
	for _, rule := range compiledPrivilegeRules() {
		if rule.method != method {
			continue
		}
		if matchRoutePattern(rule.pattern, path) {
			return rule.priv.MinRole, true
		}
	}
	return 0, false
}

// TestLinkClickRouteNeedsOnlyLogin 打开计数必须任何登录用户都能报。
// /api/v1/links 整段在 adminPathPrefixes 里，所以"不写规则"不等于"放行"，
// 而是默认要管理员 —— 将来链接摆到用户面前时，每一次点击都会 403，
// 而管理页自己看到的计数仍然是对的（管理员点的那几次），这种错最难发现。
func TestLinkClickRouteNeedsOnlyLogin(t *testing.T) {
	minRole, defined := resolvePrivilege(t, "POST", "/api/v1/links/7/click")
	if !defined {
		t.Fatal("POST /api/v1/links/:id/click 没有显式权限规则")
	}
	if minRole != 3 {
		t.Fatalf("打开计数应要求 MinRole 3（任何登录用户），实得 %d", minRole)
	}
	// 前提本身：这条路径确实落在管理员前缀里，所以上面那条规则是唯一让它放宽的东西。
	inAdminPrefix := false
	for _, p := range adminPathPrefixes {
		if len("/api/v1/links/7/click") >= len(p) && "/api/v1/links/7/click"[:len(p)] == p {
			inAdminPrefix = true
		}
	}
	if !inAdminPrefix {
		t.Fatal("前提变了：/api/v1/links 已不在 adminPathPrefixes，本测试的对照意义消失，请一并更新")
	}
}

// TestLinkManagementRoutesStillNeedAdmin 反向对照：放宽的只有"报一次点击"，
// 增删改与列表仍然是管理员。少了这条断言，将来有人为了省事把整段改成 MinRole 3 也不会被发现。
func TestLinkManagementRoutesStillNeedAdmin(t *testing.T) {
	for _, c := range []struct {
		method, path string
	}{
		{"GET", "/api/v1/links"},
		{"GET", "/api/v1/links/all"},
		{"POST", "/api/v1/links"},
		{"PUT", "/api/v1/links/7"},
		{"DELETE", "/api/v1/links/7"},
	} {
		minRole, defined := resolvePrivilege(t, c.method, c.path)
		if !defined {
			t.Fatalf("%s %s 没有显式权限规则", c.method, c.path)
		}
		if minRole != 2 {
			t.Fatalf("%s %s 应要求 MinRole 2（管理员），实得 %d", c.method, c.path, minRole)
		}
	}
}
