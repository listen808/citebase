package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func init() {
	gin.SetMode(gin.TestMode)
}

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)

	// 前 3 次应该允许
	for i := 0; i < 3; i++ {
		if !rl.Allow("test-key") {
			t.Errorf("Expected Allow() to return true for request %d", i+1)
		}
	}

	// 第 4 次应该被拒绝
	if rl.Allow("test-key") {
		t.Error("Expected Allow() to return false after exceeding limit")
	}
}

func TestRateLimiter_DifferentKeys(t *testing.T) {
	rl := NewRateLimiter(2, time.Minute)

	// key1 用满配额
	rl.Allow("key1")
	rl.Allow("key1")

	// key1 应该被拒绝
	if rl.Allow("key1") {
		t.Error("Expected key1 to be rate limited")
	}

	// key2 应该不受影响
	if !rl.Allow("key2") {
		t.Error("Expected key2 to be allowed (different key)")
	}
}

func TestRateLimiter_WindowExpiry(t *testing.T) {
	rl := NewRateLimiter(2, 50*time.Millisecond)

	rl.Allow("test-key")
	rl.Allow("test-key")

	// 应该被拒绝
	if rl.Allow("test-key") {
		t.Error("Expected rate limit to be reached")
	}

	// 等待窗口过期
	time.Sleep(60 * time.Millisecond)

	// 应该重新允许
	if !rl.Allow("test-key") {
		t.Error("Expected Allow() to return true after window expiry")
	}
}

func TestRateLimiter_Cleanup(t *testing.T) {
	rl := NewRateLimiter(5, 50*time.Millisecond)

	rl.Allow("key1")
	rl.Allow("key2")

	// 等待窗口过期
	time.Sleep(60 * time.Millisecond)

	// 执行清理
	rl.Cleanup()

	// 内部 map 应该被清空
	rl.mu.RLock()
	count := len(rl.requests)
	rl.mu.RUnlock()

	if count != 0 {
		t.Errorf("Expected 0 keys after cleanup, got %d", count)
	}
}

func TestRateLimitMiddleware(t *testing.T) {
	// 创建一个低限制的限流器用于测试
	originalLimiter := apiLimiter
	apiLimiter = NewRateLimiter(2, time.Minute)
	defer func() { apiLimiter = originalLimiter }()

	router := gin.New()
	router.Use(RateLimitMiddleware())
	router.GET("/test", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 前 2 次请求应该成功
	for i := 0; i < 2; i++ {
		w := httptest.NewRecorder()
		req, _ := http.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.168.1.100:12345"
		router.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("Expected 200 for request %d, got %d", i+1, w.Code)
		}
	}

	// 第 3 次应该被限流
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/test", nil)
	req.RemoteAddr = "192.168.1.100:12345"
	router.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429, got %d", w.Code)
	}
}

func TestStrictRateLimitMiddleware(t *testing.T) {
	originalLimiter := strictLimiter
	strictLimiter = NewRateLimiter(1, time.Minute)
	defer func() { strictLimiter = originalLimiter }()

	router := gin.New()
	router.Use(StrictRateLimitMiddleware())
	router.POST("/login", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	// 第 1 次应该成功
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/login", nil)
	req.RemoteAddr = "10.0.0.1:54321"
	router.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("Expected 200, got %d", w.Code)
	}

	// 第 2 次应该被限流
	w = httptest.NewRecorder()
	req, _ = http.NewRequest("POST", "/login", nil)
	req.RemoteAddr = "10.0.0.1:54321"
	router.ServeHTTP(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("Expected 429, got %d", w.Code)
	}
}
