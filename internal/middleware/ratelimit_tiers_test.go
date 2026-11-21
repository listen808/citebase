package middleware

import (
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

// TestLimiterTiers 钉住三档数字。它们是本批修复的全部事实来源：
// refresh 从 10 放宽到 30，login 保持 10（口令猜测防护不受影响）。
// 谁把 refresh 改回和 strict 同档，这条就红。
func TestLimiterTiers(t *testing.T) {
	cases := []struct {
		name       string
		rl         *RateLimiter
		wantLimit  int
		wantWindow time.Duration
	}{
		{"apiLimiter", apiLimiter, 300, time.Minute},
		{"strictLimiter", strictLimiter, 10, time.Minute},
		{"refreshLimiter", refreshLimiter, 30, time.Minute},
	}
	for _, c := range cases {
		if c.rl.limit != c.wantLimit {
			t.Fatalf("%s = %d 次/窗口，期望 %d", c.name, c.rl.limit, c.wantLimit)
		}
		if c.rl.window != c.wantWindow {
			t.Fatalf("%s 窗口 = %v，期望 %v", c.name, c.rl.window, c.wantWindow)
		}
	}
	if strictLimiter == refreshLimiter {
		t.Fatal("refresh 与 strict 共用同一个桶 —— 放宽 refresh 就必须分开")
	}
}

// TestRefreshAllowsWhatStrictDenies 用同一串请求跑两档：
// 第 12 次在 strict 被拒、在 refresh 放行。这才是"多标签页 SPA 不再天天撞 429"的直接证据。
func TestRefreshAllowsWhatStrictDenies(t *testing.T) {
	origStrict, origRefresh := strictLimiter, refreshLimiter
	defer func() { strictLimiter, refreshLimiter = origStrict, origRefresh }()
	strictLimiter = NewRateLimiter(10, time.Minute)
	refreshLimiter = NewRateLimiter(30, time.Minute)

	call := func(mw gin.HandlerFunc, path string) *httptest.ResponseRecorder {
		router := gin.New()
		router.Use(mw)
		router.POST(path, func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"ok": true}) })
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", path, nil)
		req.RemoteAddr = "10.0.0.77:40000"
		router.ServeHTTP(w, req)
		return w
	}

	for i := 0; i < 11; i++ {
		if w := call(StrictRateLimitMiddleware(), "/login"); i == 10 && w.Code != http.StatusTooManyRequests {
			t.Fatalf("第 11 次 login 应被 strict 拒，实得 %d", w.Code)
		}
	}
	for i := 0; i < 11; i++ {
		w := call(RefreshRateLimitMiddleware(), "/auth/refresh")
		if w.Code != http.StatusOK {
			t.Fatalf("第 %d 次 refresh 应放行（30/分钟档），实得 %d", i+1, w.Code)
		}
	}
}

// TestDeniedRequestCarriesRetryAfter 每个 429 都要带 Retry-After，且至少 1 秒。
// 没有这个头，客户端只能瞎猜：要么立刻重试（正是限流要挡的），要么把可恢复的等待渲染成整页崩溃。
func TestDeniedRequestCarriesRetryAfter(t *testing.T) {
	orig := apiLimiter
	defer func() { apiLimiter = orig }()
	apiLimiter = NewRateLimiter(1, time.Minute)

	router := gin.New()
	router.Use(RateLimitMiddleware())
	router.GET("/api/v1/ping", func(c *gin.Context) { c.JSON(http.StatusOK, nil) })

	var last *httptest.ResponseRecorder
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/api/v1/ping", nil)
		req.RemoteAddr = "10.0.0.88:50000"
		router.ServeHTTP(w, req)
		last = w
	}
	if last.Code != http.StatusTooManyRequests {
		t.Fatalf("第二次应被限，实得 %d", last.Code)
	}
	h := last.Header().Get("Retry-After")
	if h == "" {
		t.Fatal("429 没带 Retry-After")
	}
	secs, err := strconv.Atoi(h)
	if err != nil || secs < 1 {
		t.Fatalf("Retry-After = %q，应为不小于 1 的秒数", h)
	}
}

// TestStaticAssetsAndHealthBypassLimiter 豁免清单是"限流别把自己的静态资源与探活算进额度"。
// 少一条豁免，页面加载期间的一串 /assets 请求就会吃掉 API 额度。
func TestStaticAssetsAndHealthBypassLimiter(t *testing.T) {
	orig := apiLimiter
	defer func() { apiLimiter = orig }()
	apiLimiter = NewRateLimiter(1, time.Millisecond)

	router := gin.New()
	router.Use(RateLimitMiddleware())
	for _, p := range []string{"/api/v1/health", "/assets/app.js", "/images/1.png", "/static/x.css"} {
		router.GET(p, func(c *gin.Context) { c.JSON(http.StatusOK, nil) })
	}
	// 先耗掉额度
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/api/v1/health", nil)
	req.RemoteAddr = "10.0.0.99:60000"
	router.ServeHTTP(w, req)

	for _, p := range []string{"/api/v1/health", "/assets/app.js", "/images/1.png", "/static/x.css"} {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", p, nil)
		req.RemoteAddr = "10.0.0.99:60000"
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Fatalf("%s 应豁免限流，实得 %d", p, w.Code)
		}
	}
}
