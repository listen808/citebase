package response

import (
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"
)

// Text 返回一个键的中文原文，也就是响应里 message 字段的值。
// 没登记的键返回键名本身：界面上出现一个 snake_case 词比出现一句空白好定位 ——
// 前者一眼看得出"这里漏登记了"，后者只会让人以为服务端没说话。
func Text(key string) string {
	if value, ok := zhMessages[key]; ok {
		return value
	}
	return key
}

// ErrorKey 是带 message_key 的错误出口。
//
// 为什么要同时发两句：message 留给不看键的一方（curl、CLI、邮件模板、
// 以及还没升级前端的老部署），message_key 给前端选语言。
// 前端在 ApiError 构造处按当前界面语言替换（web/src/api/client.ts），
// 所以显示 err.message 的那两百多个调用点不用改就跟着变。
func ErrorKey(c *gin.Context, httpCode int, key string) {
	ErrorKeyDetail(c, httpCode, key, "")
}

// ErrorKeyDetail 在句子之外再带一句"具体原因"（哪个字段没填对、哪个名字撞了）。
//
// 为什么拆成两个字段而不是把原因拼进句子：句子要翻译，原因通常不用 ——
// gin binding 报的字段名、冲突的那个具体值，本来就不是可翻译的句子。
// 拼在一起的结果是英文界面上出现「Parameter error: Key: 'name' Error:field validation...」
// 这种半中半英的串，而且句子永远翻不干净。
//
// 数据库/驱动原文**不许**进这个字段：那是 internal/api/error_leak_test.go 拦的泄漏面
// （表名、列名、SQL 片段、连接信息）。要给用户看细节就带"他听得懂的那一半"，
// 另一半走 FailSafe 进日志。
func ErrorKeyDetail(c *gin.Context, httpCode int, key, detail string) {
	c.JSON(httpCode, Response{
		Code:          httpCode,
		Message:       Text(key),
		MessageKey:    key,
		MessageDetail: detail,
	})
}

// 与既有 BadRequest / Unauthorized… 一一对应的带键版本。
// 保留不带键的那些：它们还要接收服务端临时拼出来的句子（含内部细节的已经由
// internal/api/error_leak_test.go 清点），迁移是逐批做的，不能一次砍断。

// BadRequestKey returns a 400 error with a message key
func BadRequestKey(c *gin.Context, key string) { ErrorKey(c, http.StatusBadRequest, key) }

// BadRequestKeyDetail returns a 400 whose sentence is keyed and whose detail is the
// 具体原因（字段名、冲突值）。见 ErrorKeyDetail 的分工说明。
func BadRequestKeyDetail(c *gin.Context, key, detail string) {
	ErrorKeyDetail(c, http.StatusBadRequest, key, detail)
}

// UnauthorizedKey returns a 401 error with a message key
func UnauthorizedKey(c *gin.Context, key string) { ErrorKey(c, http.StatusUnauthorized, key) }

// ForbiddenKey returns a 403 error with a message key
func ForbiddenKey(c *gin.Context, key string) { ErrorKey(c, http.StatusForbidden, key) }

// ConflictKey returns a 409 error with a message key
func ConflictKey(c *gin.Context, key string) { ErrorKey(c, http.StatusConflict, key) }

// NotFoundKey returns a 404 error with a message key
func NotFoundKey(c *gin.Context, key string) { ErrorKey(c, http.StatusNotFound, key) }

// ServerErrorKey returns a 500 error with a message key
func ServerErrorKey(c *gin.Context, key string) { ErrorKey(c, http.StatusInternalServerError, key) }

// TooManyRequestsKey returns a 429 error with a message key
func TooManyRequestsKey(c *gin.Context, key string) { ErrorKey(c, http.StatusTooManyRequests, key) }

// sentinelKeys 把"共享 sentinel 错误"映射到键。
// 这张表存在的理由：ErrUnauthorized 这类错误在服务层被 `errors.New` 出来、
// 到了出口由 ErrorWithErr 原样写进响应 —— 前端拿到的永远是那句中文。
// 认不出来的一律照旧返回 err.Error()（不猜、不覆盖成通用文案，那是把细节弄丢）。
var sentinelKeys = []struct {
	err error
	key string
}{
	{ErrUnauthorized, MsgSignedOutOrTokenInvalid},
	{ErrNotFound, MsgResourceNotFound},
	{ErrBadRequest, MsgRequestParameterError},
	{ErrInternal, MsgInternalError},
}

// KeyForError 供出口与测试复用：这条错误是不是有登记的用户文案键。
func KeyForError(err error) (string, bool) {
	for _, item := range sentinelKeys {
		if errors.Is(err, item.err) {
			return item.key, true
		}
	}
	return "", false
}
