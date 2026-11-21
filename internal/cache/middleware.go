package cache

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// responseRecorder captures the response body and status code.
type responseRecorder struct {
	gin.ResponseWriter
	body       *bytes.Buffer
	statusCode int
}

func (r *responseRecorder) Write(data []byte) (int, error) {
	r.body.Write(data)
	return r.ResponseWriter.Write(data)
}

func (r *responseRecorder) WriteHeader(code int) {
	r.statusCode = code
	r.ResponseWriter.WriteHeader(code)
}

// CacheMiddleware creates a Gin middleware that caches GET request responses.
func CacheMiddleware(c Cache, ttl time.Duration) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		// Only cache GET requests
		if ctx.Request.Method != http.MethodGet {
			ctx.Next()
			return
		}

		// Skip caching for admin routes
		if strings.HasPrefix(ctx.Request.URL.Path, "/api/v1/admin") {
			ctx.Next()
			return
		}

		// Generate cache key from request path and query
		key := generateHTTPCacheKey(ctx.Request)

		// Try to get cached response
		var cached cachedResponse
		err := c.GetJSON(ctx, key, &cached)
		if err == nil {
			// Cache hit - serve from cache
			for k, v := range cached.Headers {
				ctx.Header(k, v)
			}
			ctx.Header("X-Cache", "HIT")
			ctx.Data(cached.StatusCode, cached.ContentType, cached.Body)
			ctx.Abort()
			return
		}

		// Cache miss - record the response
		recorder := &responseRecorder{
			ResponseWriter: ctx.Writer,
			body:           &bytes.Buffer{},
			statusCode:     http.StatusOK,
		}
		ctx.Writer = recorder

		ctx.Next()

		// Only cache successful responses
		if recorder.statusCode == http.StatusOK {
			resp := cachedResponse{
				StatusCode:  recorder.statusCode,
				ContentType: recorder.Header().Get("Content-Type"),
				Body:        recorder.body.Bytes(),
				Headers: map[string]string{
					"Content-Type": recorder.Header().Get("Content-Type"),
				},
			}
			_ = c.SetJSON(ctx, key, resp, ttl)
		}
	}
}

// cachedResponse holds a serializable HTTP response.
type cachedResponse struct {
	StatusCode  int               `json:"status_code"`
	ContentType string            `json:"content_type"`
	Body        []byte            `json:"body"`
	Headers     map[string]string `json:"headers"`
}

// generateHTTPCacheKey creates a deterministic cache key from an HTTP request.
func generateHTTPCacheKey(req *http.Request) string {
	raw := req.URL.Path + "?" + req.URL.RawQuery
	hash := sha256.Sum256([]byte(raw))
	return KeySearchPrefix + fmt.Sprintf("%x", hash)
}

// InvalidateHTTPCache removes a cached HTTP response.
func InvalidateHTTPCache(c Cache, path string) {
	raw := path + "?"
	hash := sha256.Sum256([]byte(raw))
	key := KeySearchPrefix + fmt.Sprintf("%x", hash)
	_ = c.Del(context.Background(), key)
}
