package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/log"
)

// Common business errors
//
// 文案住在 messages.go 那张表里（键 → 中文 → 前端译文），这里只按键取名 ——
// 同一个"未登录"不该在两个文件里各写一遍。
var (
	ErrUnauthorized = errors.New(Text(MsgSignedOutOrTokenInvalid))
	ErrNotFound     = errors.New(Text(MsgResourceNotFound))
	ErrBadRequest   = errors.New(Text(MsgRequestParameterError))
	ErrInternal     = errors.New(Text(MsgInternalError))
)

// Response unified API response format
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	// MessageKey 是给前端的稳定文案键（契约见 messages.go）。
	// omitempty：绝大多数响应没有用户文案（成功响应、纯数据），不带这个字段就是今天的样子。
	MessageKey string `json:"message_key,omitempty"`
	// MessageDetail 是给界面看的具体原因（哪个字段没填对、哪个约束冲突），不是句子。
	// 为什么要把"句子"和"原因"分成两个字段：句子要翻译，原因通常不用（它是 gin binding 的
	// 字段名、或某个具体值）；混在一句话里拼字符串，英文界面就会拿到半英半中的句子。
	// 数据库/驱动原文**不许**进这个字段 —— 那是 internal/api/error_leak_test.go 拦的泄漏面。
	MessageDetail string `json:"message_detail,omitempty"`
}

// JSON returns a unified JSON response
func JSON(c *gin.Context, httpCode int, code int, message string, data interface{}) {
	c.JSON(httpCode, Response{
		Code:    code,
		Message: message,
		Data:    data,
	})
}

// Success returns a success response
func Success(c *gin.Context, data interface{}) {
	JSON(c, http.StatusOK, 0, "success", data)
}

// SuccessWithMsg returns a success response with custom message
func SuccessWithMsg(c *gin.Context, data interface{}, message string) {
	JSON(c, http.StatusOK, 0, message, data)
}

// Created returns a created response
func Created(c *gin.Context, data interface{}) {
	JSON(c, http.StatusCreated, 0, "created", data)
}

// Error returns an error response
func Error(c *gin.Context, httpCode int, code int, message string) {
	JSON(c, httpCode, code, message, nil)
}

// Fail returns an error response with detailed error message.
//
// 警告：这条会把内部错误原文拼进响应体。GORM 的原文里带表名、列名、SQL 片段，
// 有时还有连接信息 —— 对拿到一个 500 的人来说，那些正是做指纹识别要的东西。
// 新代码请用 FailSafe；API 层残留的 Fail(err.Error()) 由
// internal/api/error_leak_test.go 逐条清点，只减不增。
func Fail(c *gin.Context, httpCode int, message string, err string) {
	JSON(c, httpCode, httpCode, message+": "+err, nil)
}

// FailSafe 记完整错误到日志，只把 message 返给客户端。
// 加这条是为了让"运维要看细节"和"客户端不该看细节"不再是同一个参数。
func FailSafe(c *gin.Context, httpCode int, message string, err error) {
	log.Errorf("[API %d] %s: %v", httpCode, message, err)
	Error(c, httpCode, httpCode, message)
}

// BadRequest returns a 400 error
func BadRequest(c *gin.Context, message string) {
	Error(c, http.StatusBadRequest, 400, message)
}

// Unauthorized returns a 401 error
func Unauthorized(c *gin.Context, message string) {
	Error(c, http.StatusUnauthorized, 401, message)
}

// Forbidden returns a 403 error
func Forbidden(c *gin.Context, message string) {
	Error(c, http.StatusForbidden, 403, message)
}

// Conflict returns a 409 error
func Conflict(c *gin.Context, message string) {
	Error(c, http.StatusConflict, 409, message)
}

// NotFound returns a 404 error
func NotFound(c *gin.Context, message string) {
	Error(c, http.StatusNotFound, 404, message)
}

// ServerError returns a 500 error
func ServerError(c *gin.Context, message string) {
	Error(c, http.StatusInternalServerError, 500, message)
}

// ServerErrorSafe returns a generic 500 error without exposing internal details
// The real error is logged server-side for debugging
func ServerErrorSafe(c *gin.Context, err error) {
	log.Errorf("[ServerError] %v", err)
	Error(c, http.StatusInternalServerError, 500, "internal server error")
}

// TooManyRequests returns a 429 error
func TooManyRequests(c *gin.Context, message string) {
	Error(c, http.StatusTooManyRequests, 429, message)
}

// PaginatedData paginated response data
type PaginatedData struct {
	Data     interface{} `json:"data"`
	Total    int64       `json:"total"`
	Page     int         `json:"page"`
	PageSize int         `json:"page_size"`
}

// Paginated returns a paginated success response
func Paginated(c *gin.Context, data interface{}, total int64, page int, pageSize int) {
	JSON(c, http.StatusOK, 0, "success", PaginatedData{
		Data:     data,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	})
}

// ErrorWithErr returns an error response from error type, compatible with common.Error signature
// Default HTTP status is 400, can be overridden with optional status parameter
func ErrorWithErr(c *gin.Context, err error, status ...int) {
	code := http.StatusBadRequest
	if len(status) > 0 {
		code = status[0]
	}
	// 认得出键的共享 sentinel 走带键出口（前端才能按界面语言显示）；
	// 认不出的照旧原样返回 —— 这里不猜、也不压成通用文案，那是把细节弄丢。
	if key, ok := KeyForError(err); ok {
		ErrorKey(c, code, key)
		return
	}
	Error(c, code, code, err.Error())
}

// AbortWithError aborts the request and returns an error response
func AbortWithError(c *gin.Context, httpCode int, code int, message string) {
	Error(c, httpCode, code, message)
	c.Abort()
}
