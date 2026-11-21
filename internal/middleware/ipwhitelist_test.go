package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/utils/config"
)

func newIPTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(IPWhitelistMiddleware())
	r.GET("/ping", func(c *gin.Context) { c.String(200, "pong") })
	return r
}

func resetIPWhitelist() {
	ipWhitelistOnce = sync.Once{}
	parsedIPRanges = nil
}

func TestIPWhitelistAllow(t *testing.T) {
	resetIPWhitelist()
	config.AppConfig.Security = config.SecurityConfig{
		IPWhitelistEnabled: true,
		IPWhitelist:        []string{"192.168.1.0/24", "10.0.0.5"},
		IPWhitelistMode:    "allow",
	}
	r := newIPTestRouter()

	// 命中 CIDR，放行
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ping", nil)
	req.RemoteAddr = "192.168.1.10:1234"
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200 for CIDR match, got %d", w.Code)
	}

	// 单 IP 命中，放行
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/ping", nil)
	req2.RemoteAddr = "10.0.0.5:1234"
	r.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("expected 200 for single IP match, got %d", w2.Code)
	}

	// 未命中，拒绝
	w3 := httptest.NewRecorder()
	req3 := httptest.NewRequest("GET", "/ping", nil)
	req3.RemoteAddr = "8.8.8.8:1234"
	r.ServeHTTP(w3, req3)
	if w3.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-match, got %d", w3.Code)
	}
}

func TestIPWhitelistDeny(t *testing.T) {
	resetIPWhitelist()
	config.AppConfig.Security = config.SecurityConfig{
		IPWhitelistEnabled: true,
		IPWhitelist:        []string{"8.8.8.8"},
		IPWhitelistMode:    "deny",
	}
	r := newIPTestRouter()

	// 命中封锁列表，拒绝
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ping", nil)
	req.RemoteAddr = "8.8.8.8:1234"
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for blocked IP, got %d", w.Code)
	}

	// 未命中，放行
	w2 := httptest.NewRecorder()
	req2 := httptest.NewRequest("GET", "/ping", nil)
	req2.RemoteAddr = "1.1.1.1:1234"
	r.ServeHTTP(w2, req2)
	if w2.Code != 200 {
		t.Fatalf("expected 200 for unblocked IP, got %d", w2.Code)
	}
}

func TestIPWhitelistDisabled(t *testing.T) {
	resetIPWhitelist()
	config.AppConfig.Security = config.SecurityConfig{IPWhitelistEnabled: false}
	r := newIPTestRouter()
	w := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/ping", nil)
	req.RemoteAddr = "9.9.9.9:1234"
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("expected 200 when disabled, got %d", w.Code)
	}
}
