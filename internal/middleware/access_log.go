package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"strings"
	"unicode/utf8"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/model/admin"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	adminsvc "github.com/listen808/citebase/internal/service/admin"
	"github.com/listen808/citebase/internal/utils"
)

// AccessLog 中间件：记录所有 HTTP 请求到 aw_log 表
func AccessLog(logService *adminsvc.LogService) gin.HandlerFunc {
	return func(c *gin.Context) {
		// 跳过不需要记录的路径
		path := c.Request.URL.Path
		if skipPath(path) {
			c.Next()
			return
		}

		// 记录开始时间
		// 读取 body（POST/PUT）：只窥探前 8KB 用于日志，其余原样交给 handler，
		// 避免为了记录日志把大请求体（长文档正文、Base64 图片）整体缓冲进内存。
		var postBody string
		if c.Request.Body != nil && (c.Request.Method == "POST" || c.Request.Method == "PUT" || c.Request.Method == "PATCH") {
			contentType := c.Request.Header.Get("Content-Type")
			// multipart/form-data 包含二进制文件内容，不记录原始 body，避免写入无效 UTF-8 到 MySQL
			if strings.HasPrefix(contentType, "multipart/form-data") {
				postBody = "[multipart/form-data]"
			} else {
				peek := make([]byte, maxLoggedBodyBytes)
				n, _ := io.ReadFull(c.Request.Body, peek)
				peek = peek[:n]
				c.Request.Body = io.NopCloser(io.MultiReader(bytes.NewReader(peek), c.Request.Body))
				// 清理无效 UTF-8 字节，防止 MySQL 1366 错误
				postBody = sanitizeUTF8(peek)
			}
		}

		// 执行处理
		c.Next()

		// 请求结束后记录日志
		level := admin.LogLevelInfo
		statusCode := c.Writer.Status()
		message := c.Request.Method + " " + path

		switch {
		case statusCode >= 500:
			level = admin.LogLevelError
			message = message + " - " + statusText(statusCode)
		case statusCode >= 400:
			level = admin.LogLevelWarn
			message = message + " - " + statusText(statusCode)
		default:
			message = message + " - 200 OK"
		}

		// 脱敏
		postBody = maskSensitive(postBody)

		userID, _ := c.Get("user_id")
		username, _ := c.Get("username")

		uid, _ := userID.(int)
		uname, _ := username.(string)

		// 复制 Request 字段到局部变量，避免 goroutine 中的数据竞争
		// 查询串同样必须脱敏：认证支持 ?token=<JWT>（WebSocket/图片/附件场景），
		// 原样入库等于把可用凭据永久留存在 aw_log 里并在管理端回显。
		rawQuery := maskQueryForLog(c.Request.URL.RawQuery)
		clientIP := c.ClientIP()
		userAgent := c.Request.UserAgent()
		referer := c.Request.Referer()

		go func() {
			defer func() { _ = recover() }()
			// 日志记录失败不影响主流程
			_ = logService.LogSystemAction(
				level,
				path,
				rawQuery,
				postBody,
				message,
				clientIP,
				userAgent,
				referer,
				uid,
				uname,
			)
		}()

		// 更新用户最后活跃时间（已认证用户）
		if uid > 0 {
			go func(userID int) {
				defer func() { _ = recover() }()
				repo := userrepo.NewUserRepository()
				_ = repo.UpdateLastActivity(userID)
			}(uid)
		}
	}
}

// skipPath 判断路径是否跳过记录
func skipPath(path string) bool {
	skipPrefixes := []string{
		"/api/v1/health",
		"/images/",
		"/attachments/",
		"/static/",
		"/assets/",
		"/swagger/",
	}
	for _, prefix := range skipPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

// maxLoggedBodyBytes 请求体日志窥探上限：日志本身只保留前若干字符，
// 但必须保证不会为记录日志而无上限地缓冲整个请求体。
const maxLoggedBodyBytes = 8 << 10

// sensitiveQueryKeys 查询参数中出现这些键时，其值一律替换为 ****
var sensitiveQueryKeys = map[string]bool{
	"token": true, "access_token": true, "refresh_token": true, "auth_token": true,
	"password": true, "passwd": true, "old_password": true, "new_password": true,
	"secret": true, "client_secret": true, "api_key": true, "apikey": true,
	"code": true, "state": true, "key": true,
}

// maskQueryForLog 对查询串中的敏感参数值脱敏，保留键名与其他参数便于排查。
func maskQueryForLog(raw string) string {
	if raw == "" {
		return ""
	}
	parts := strings.Split(raw, "&")
	changed := false
	for i, p := range parts {
		k := p
		if eq := strings.IndexByte(p, '='); eq >= 0 {
			k = p[:eq]
		}
		if decoded, err := url.QueryUnescape(k); err == nil {
			k = decoded
		}
		if sensitiveQueryKeys[strings.ToLower(strings.TrimSpace(k))] {
			parts[i] = k + "=****"
			changed = true
		}
	}
	if !changed {
		return raw
	}
	return strings.Join(parts, "&")
}

// maskSensitive 脱敏敏感字段
func maskSensitive(post string) string {
	if post == "" {
		return ""
	}

	if len(post) > 500 {
		post = post[:500] + "...(truncated)"
	}

	// 使用公共工具函数脱敏
	return utils.MaskSensitiveInfo(post)
}

// sanitizeUTF8 清理字节切片中的无效 UTF-8 序列，替换为 Unicode 替换字符
// 防止二进制或损坏数据写入 MySQL 时触发 1366 (Incorrect string value) 错误
func sanitizeUTF8(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if utf8.Valid(b) {
		return string(b)
	}
	// 逐 rune 解码，无效字节替换为 U+FFFD
	var buf strings.Builder
	buf.Grow(len(b))
	for i := 0; i < len(b); {
		r, size := utf8.DecodeRune(b[i:])
		if r == utf8.RuneError && size == 1 {
			buf.WriteRune('\uFFFD')
			i++
		} else {
			buf.WriteRune(r)
			i += size
		}
	}
	return buf.String()
}

// statusText 返回 HTTP 状态码文本
func statusText(code int) string {
	switch code {
	case 200:
		return "200 OK"
	case 201:
		return "201 Created"
	case 204:
		return "204 No Content"
	case 301:
		return "301 Moved Permanently"
	case 302:
		return "302 Found"
	case 304:
		return "304 Not Modified"
	case 400:
		return "400 Bad Request"
	case 401:
		return "401 Unauthorized"
	case 403:
		return "403 Forbidden"
	case 404:
		return "404 Not Found"
	case 405:
		return "405 Method Not Allowed"
	case 409:
		return "409 Conflict"
	case 422:
		return "422 Unprocessable Entity"
	case 429:
		return "429 Too Many Requests"
	case 500:
		return "500 Internal Server Error"
	case 502:
		return "502 Bad Gateway"
	case 503:
		return "503 Service Unavailable"
	default:
		return http.StatusText(code) + " Unknown"
	}
}
