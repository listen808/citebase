package middleware

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// 这批测试把「AutoRBAC 对未登记路由直接放行」这条静默通道变成显式清单。
//
// 起因：/api/v1/roles/:id/users 既没登记权限规则，/api/v1/roles 也不在
// adminPathPrefixes 里，于是 AutoRBAC 走 `c.Next()` 放行 —— 任何登录用户
// POST 一次就把自己设成 role 2，而 IsAdmin 认 roleID==1||==2，一次调用拿到
// 全站管理员权限。同一条通道上当时还挂着 reset-users、privileges 的增删改、
// GET /users/:id，以及根本没经过 RBAC 的 GET /search/debug/:id
// （返回索引绝对路径 + 任意私有文档正文前 200 字）。
//
// 为什么按「敏感前缀」立判据，而不是按「这条路由挂没挂 AutoRBAC」：
// 后者要解析每组注册的中间件，而 ai/routes.go 是把中间件写在**每一条注册行**
// 上的（ai.GET("/config", AuthMiddleware(), AutoRBAC(), ...)），组级 .Use()
// 看不见它 —— 解析稍有出入就会漏判成"没过 RBAC"，于是门禁安静地放过。
// 敏感前缀这条判据与中间件写法无关：只要这条路径存在、又拿不出显式规则，就红。

type registeredRoute struct {
	method string
	path   string
	file   string
	line   int
}

// collectRegisteredRoutes 扫描 internal/api/**/routes.go，取出每条 `X.METHOD("/path"` 注册。
// 只认注册形状，不解析 handler 与中间件。
func collectRegisteredRoutes(t *testing.T) []registeredRoute {
	t.Helper()
	dir := filepath.Join("..", "api")
	var files []string
	err := filepath.Walk(dir, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(p, "routes.go") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("扫描 %s 失败: %v", dir, err)
	}
	sort.Strings(files)

	var routes []registeredRoute
	// 组变量名 → 路径前缀。v1 由 app/server/server.go 挂在 /api/v1。
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("读 %s 失败: %v", f, err)
		}
		routes = append(routes, parseRegisteredRoutes(string(data), filepath.Base(f))...)
	}
	return routes
}

var httpVerbs = map[string]bool{
	"GET": true, "POST": true, "PUT": true,
	"DELETE": true, "PATCH": true, "HEAD": true, "OPTIONS": true,
}

func parseRegisteredRoutes(src, file string) []registeredRoute {
	groups := map[string]string{"v1": "/api/v1"}
	var routes []registeredRoute

	for i, raw := range strings.Split(src, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "//") {
			continue
		}
		if name, parent, prefix, ok := groupAssign(line); ok {
			base, known := groups[parent]
			if !known {
				base = "/api/v1" // 认不出的父组按 v1 处理：宁可多算一条也别漏判
			}
			groups[name] = base + prefix
			continue
		}
		if name, verb, path, ok := routeDecl(line); ok {
			base, known := groups[name]
			if !known {
				continue
			}
			routes = append(routes, registeredRoute{
				method: verb,
				path:   strings.ReplaceAll(base+path, "*", ":wildcard"),
				file:   file,
				line:   i + 1,
			})
		}
	}
	return routes
}

// groupAssign 认 `child := parent.Group("/sub", ...)`。
func groupAssign(line string) (name, parent, prefix string, ok bool) {
	i := strings.Index(line, ":=")
	if i < 0 {
		return "", "", "", false
	}
	rest := strings.TrimSpace(line[i+2:])
	k := strings.Index(rest, ".Group(")
	if k < 0 {
		return "", "", "", false
	}
	p, found := quotedFirst(rest[k:])
	if !found {
		return "", "", "", false
	}
	return strings.TrimSpace(line[:i]), strings.TrimSpace(rest[:k]), p, true
}

// routeDecl 认 `group.GET("/path", ...)`。
func routeDecl(line string) (name, verb, path string, ok bool) {
	dot := strings.Index(line, ".")
	if dot <= 0 {
		return "", "", "", false
	}
	rest := line[dot+1:]
	open := strings.Index(rest, "(")
	if open < 0 || !httpVerbs[rest[:open]] {
		return "", "", "", false
	}
	p, found := quotedFirst(rest[open:])
	if !found {
		return "", "", "", false
	}
	return strings.TrimSpace(line[:dot]), rest[:open], p, true
}

func quotedFirst(s string) (string, bool) {
	q := strings.Index(s, "\"")
	if q < 0 {
		return "", false
	}
	tail := s[q+1:]
	e := strings.Index(tail, "\"")
	if e < 0 {
		return "", false
	}
	return tail[:e], true
}

func resolveLikeMiddleware(method, path string) (minRole int, matchedRule bool, hitAdminPrefix bool) {
	for _, rule := range compiledPrivilegeRules() {
		if rule.method != method {
			continue
		}
		if matchRoutePattern(rule.pattern, path) {
			return rule.priv.MinRole, true, false
		}
	}
	for _, prefix := range adminPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return 2, false, true
		}
	}
	return 0, false, false
}

// sensitiveFamilies 是「拿出去就能改全局状态或读别人读不到的东西」的路由族：
// 角色/权限/用户本身、能建外连与改全局配置的基础设施面、以及会返回文件系统
// 路径或全库聚合的诊断面。这些族里的每一条路由都必须有显式 MinRole 规则 ——
// 只落在 adminPathPrefixes 里不算（那是"没登记就默认管理员"的兜底，
// 而这条兜底恰恰就是这次事故里被 c.Next() 绕过的那一半）。
var sensitiveFamilies = []string{
	"/api/v1/roles",
	"/api/v1/privileges",
	"/api/v1/users",
	"/api/v1/webhooks",
	"/api/v1/git-sync",
	"/api/v1/datasources",
	"/api/v1/backups",
	"/api/v1/schema-spaces",
	"/api/v1/schema-scan-tasks",
	"/api/v1/schema-export-csv",
	"/api/v1/search/monitor",
	"/api/v1/search/debug",
	"/api/v1/search/reindex",
	"/api/v1/search/index",
	"/api/v1/search/init",
	"/api/v1/auth/oidc/admin",
	"/api/v1/open/api-keys",
	"/api/v1/configs",
	"/api/v1/logs",
}

// sensitivePublic 是敏感族里**故意**公开或故意只要登录的端点。
// 每条都必须写清为什么，否则就从这里挪回规则表里去。
var sensitivePublic = map[string]string{
	// 用户自助：只能读写自己，handler 里按 ctx 的 user_id 取值，不接受 id 参数
	"GET:/api/v1/users/me":            "self-service profile",
	"PUT:/api/v1/users/me":            "self-service profile",
	"GET:/api/v1/auth/me":             "self-service session",
	"PUT:/api/v1/auth/me":             "self-service profile",
	"PUT:/api/v1/auth/me/password":    "self-service password change",
	"PUT:/api/v1/auth/me/preferences": "self-service UI preferences（外观 × 材质 × 透明变体；handler 只取 ctx 的 uid，不接受 id 参数）",
}

func TestSensitiveRoutesHaveExplicitPrivilegeRules(t *testing.T) {
	routes := collectRegisteredRoutes(t)
	// 解析退化会让下面所有断言安静地全绿，所以先钉住"确实扫到了东西"。
	if len(routes) < 150 {
		t.Fatalf("只解析出 %d 条注册路由，明显低于本仓规模 —— 先修解析器再看下面的结论", len(routes))
	}

	var missing []string
	checked := 0
	for _, r := range routes {
		if !hasSensitivePrefix(r.path) {
			continue
		}
		if _, public := sensitivePublic[r.method+":"+r.path]; public {
			continue
		}
		checked++
		minRole, matchedRule, hitPrefix := resolveLikeMiddleware(r.method, r.path)
		key := r.method + ":" + r.path + "（" + r.file + ":" + strconv.Itoa(r.line) + "）"
		switch {
		case matchedRule && minRole <= 2:
			// 有显式规则且是管理档：正解。
		case matchedRule:
			missing = append(missing, key+" 的 MinRole="+strconv.Itoa(minRole)+" —— 敏感族被放给了普通用户，请确认它真的属于这一族")
		case hitPrefix:
			// 只靠 adminPathPrefixes 兜底：那仍然是 403，不是洞，所以只记录不当门禁。
			t.Logf("仅靠管理前缀兜底（无显式规则）：%s", key)
		default:
			missing = append(missing, key+" 既没登记权限规则、也不在管理前缀里 —— AutoRBAC 直接 c.Next()，任何登录用户可调")
		}
	}

	if len(missing) > 0 {
		sort.Strings(missing)
		t.Errorf("敏感族共校验 %d 条，其中 %d 条没有管理档闸门：\n  %s\n要么在 PrivilegeMap 登记 MinRole 1/2，要么进 adminPathPrefixes，要么确认它不该受管控并进 sensitivePublic 写理由",
			checked, len(missing), strings.Join(missing, "\n  "))
	}
}

// TestEscalationRoutesAreClosed 单独钉住这次修的那几条 —— 敏感族测试给的是覆盖面，
// 这条给的是"角色/权限面一次调用就能提权"这个具体事实，两者缺一都会漂。
func TestEscalationRoutesAreClosed(t *testing.T) {
	for _, c := range []struct {
		method, path string
		why          string
	}{
		{"POST", "/api/v1/roles/2/users", "把自己设成 role 2，而 IsAdmin 认 1 和 2 ⇒ 一次调用拿到全站管理员权限"},
		{"POST", "/api/v1/roles/2/reset-users", "把某个角色下全部用户批量降级"},
		{"POST", "/api/v1/privileges", "新增权限条目"},
		{"PUT", "/api/v1/privileges/7", "改权限条目"},
		{"DELETE", "/api/v1/privileges/7", "删权限条目"},
		{"GET", "/api/v1/users/7", "他人账号记录（含邮箱/手机号）"},
		{"GET", "/api/v1/search/debug/7", "索引绝对路径 + 任意私有文档正文前 200 字"},
		{"GET", "/api/v1/search/monitor", "索引目录 + aw_search_log 里别人的查询原文"},
		{"POST", "/api/v1/webhooks", "让服务端主动向外发请求 = 现成的内网探针"},
		{"PUT", "/api/v1/git-sync/config", "写远程地址并让服务端真的执行 pull（往磁盘改文件）"},
		{"GET", "/api/v1/schema-spaces", "已配置数据源的库结构"},
		{"GET", "/api/v1/ai/data-query/error-summary", "全库 SQL 失败聚合"},
	} {
		minRole, matchedRule, hitPrefix := resolveLikeMiddleware(c.method, c.path)
		switch {
		case !matchedRule && !hitPrefix:
			t.Errorf("%s %s 没有任何闸门 —— %s", c.method, c.path, c.why)
		case !matchedRule:
			t.Errorf("%s %s 只靠管理前缀兜底，缺显式规则（%s）", c.method, c.path, c.why)
		case minRole > 2:
			t.Errorf("%s %s MinRole=%d —— 管理面被放给了普通用户（%s）", c.method, c.path, minRole, c.why)
		}
	}
}

// TestAdminPrefixesStillResolveToRoutes 防的是另一半漂移：前缀兜底表里留着
// 已经没有路由的条目，读代码的人会以为那一段还被管着。
func TestAdminPrefixesStillResolveToRoutes(t *testing.T) {
	routes := collectRegisteredRoutes(t)
	for _, prefix := range adminPathPrefixes {
		found := false
		for _, r := range routes {
			if strings.HasPrefix(r.path, prefix) {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("adminPathPrefixes 里的 %q 已经匹配不到任何注册路由，请删掉或改成实际前缀", prefix)
		}
	}
}

func hasSensitivePrefix(path string) bool {
	for _, p := range sensitiveFamilies {
		if strings.HasPrefix(path, p) {
			return true
		}
	}
	return false
}
