package middleware

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// newHeadersTestRouter 挂上 SecurityHeadersMiddleware 后返回一个 echo 路由，
// 用 httptest 打 / 即可断言"中间件给响应加的头"。
func newHeadersTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(SecurityHeadersMiddleware())
	r.GET("/", func(c *gin.Context) { c.String(http.StatusOK, "ok") })
	return r
}

// 钉住三条安全头的存在性与取值 —— 它们是登录态 cookie 维持 JS 可写时
// XSS 的收口手段（docs/decision-log/2026-11-07-auth-cookie-and-csp.md），
// 谁把某条头删了或改松了，这里必须红。
func TestSecurityHeadersMiddlewareSetsHeaders(t *testing.T) {
	r := newHeadersTestRouter()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", w.Code)
	}

	csp := w.Header().Get("Content-Security-Policy")
	if csp != securityHeadersCSP {
		t.Errorf("Content-Security-Policy = %q, want %q", csp, securityHeadersCSP)
	}
	if got := w.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("X-Content-Type-Options = %q, want nosniff", got)
	}
	if got := w.Header().Get("Referrer-Policy"); got != "same-origin" {
		t.Errorf("Referrer-Policy = %q, want same-origin", got)
	}
}

// CSP 的关键指令逐条断言：整体串相等已经能抓"改松"，这里再把
// "该收紧的三条"单拎出来，让失败信息直指是哪一条被动过。
func TestSecurityHeadersCSPLockdownDirectives(t *testing.T) {
	for want := range map[string]bool{
		"default-src 'self'":         true,
		"script-src 'self'":          true, // 严禁出现 'unsafe-inline' / 'unsafe-eval'
		"object-src 'none'":          true,
		"base-uri 'self'":            true,
		"form-action 'self'":         true,
		"connect-src 'self'":         true,
		"img-src 'self' data: blob:": true,
		"media-src 'self' blob:":     true,
	} {
		found := false
		for _, directive := range splitDirectives(securityHeadersCSP) {
			if directive == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("CSP missing exact directive %q in %q", want, securityHeadersCSP)
		}
	}

	// script-src 里出现任何 unsafe-* 都算放宽，直接判负。
	for _, directive := range splitDirectives(securityHeadersCSP) {
		if strings.HasPrefix(directive, "script-src") {
			for _, part := range strings.Fields(directive) {
				if part == "'unsafe-inline'" || part == "'unsafe-eval'" {
					t.Errorf("script-src must not grant %q", part)
				}
			}
		}
	}
}

// 刻意不发的头也钉住：X-Frame-Options 不在全局档里，因为嵌入 widget
// 允许任意站点 iframe（见 security_headers.go 头部注释），全局锁 frame
// 会打死它。若将来有人在中间件里补上这条头，必须同时回 decision-log
// 交代 widget 的去留，测试先红。
func TestSecurityHeadersDoesNotSetFrameHeaders(t *testing.T) {
	r := newHeadersTestRouter()
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/", nil))

	if got := w.Header().Get("X-Frame-Options"); got != "" {
		t.Errorf("X-Frame-Options = %q, want empty (widget 需要任意站点可嵌，见 decision-log)", got)
	}
	// frame-ancestors 也不得出现在全局 CSP 里（widget 自己那份才有）。
	for _, directive := range splitDirectives(securityHeadersCSP) {
		if len(directive) >= len("frame-ancestors") && directive[:len("frame-ancestors")] == "frame-ancestors" {
			t.Errorf("global CSP must not set frame-ancestors, got %q", directive)
		}
	}
}

func splitDirectives(csp string) []string {
	parts := strings.Split(csp, ";")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
