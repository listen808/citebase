package response

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

// 这台测的是"表里的话"与"真正发出去的话"是不是同一句。
//
// 为什么必须有它：文案收进 internal/response/messages.go 之后，中文只住在那张表里，
// 所以"改表就是改用户看到的话"，而改错了没人会发现 —— 前端按 message_key 查译文，
// 后端按 message 给 curl，两边都从表里取，取错一次就分叉一次。
// tools/repocheck 那边试过用"在源码里找原句"来钉这件事，那条是自证判据（它搜的就是这张表），
// 所以真正的证明只能在这里：把出口跑一遍，读响应体。
//
// 反向验证：把 zhMessages 里任一条改一个字 → TestKeyedHelpersEmitRegistryText 立刻红；
// 把 ErrorKey 写成只发 message 不发 key → 第二条红。

func decodeBody(t *testing.T, rec *httptest.ResponseRecorder) Response {
	t.Helper()
	var body Response
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应体：%v（原文 %s）", err, rec.Body.String())
	}
	return body
}

func TestKeyedHelpersEmitRegistryText(t *testing.T) {
	gin.SetMode(gin.TestMode)

	cases := []struct {
		name   string
		call   func(c *gin.Context, key string)
		status int
		key    string
	}{
		{"BadRequestKey", BadRequestKey, http.StatusBadRequest, MsgSpaceNameTaken},
		{"UnauthorizedKey", UnauthorizedKey, http.StatusUnauthorized, MsgInvalidCredentials},
		{"ForbiddenKey", ForbiddenKey, http.StatusForbidden, MsgNoPermissionExportDoc},
		{"ConflictKey", ConflictKey, http.StatusConflict, MsgAlreadyInitialized},
		{"NotFoundKey", NotFoundKey, http.StatusNotFound, MsgDocNotFound},
		{"ServerErrorKey", ServerErrorKey, http.StatusInternalServerError, MsgAIServiceDisabled},
		{"TooManyRequestsKey", TooManyRequestsKey, http.StatusTooManyRequests, MsgRefreshTooFrequent},
	}

	for _, tc := range cases {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		tc.call(c, tc.key)

		if rec.Code != tc.status {
			t.Errorf("%s 的 HTTP 状态是 %d，期望 %d", tc.name, rec.Code, tc.status)
		}
		body := decodeBody(t, rec)
		if body.MessageKey != tc.key {
			t.Errorf("%s 没带 message_key：得到 %q，期望 %q", tc.name, body.MessageKey, tc.key)
		}
		if want := zhMessages[tc.key]; body.Message != want {
			t.Errorf("%s 的 message 与登记表不一致：发出 %q，表里 %q", tc.name, body.Message, want)
		}
	}
}

// 全表逐条过一遍：任何一条登记了却没被任何出口发过、或发出来对不上，都该在这里红。
func TestEveryRegisteredKeyIsEmittable(t *testing.T) {
	gin.SetMode(gin.TestMode)

	for key, want := range zhMessages {
		rec := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(rec)
		ErrorKey(c, http.StatusBadRequest, key)

		body := decodeBody(t, rec)
		if body.Message != want {
			t.Errorf("键 %q 发的是 %q，表里登记的是 %q", key, body.Message, want)
		}
		if body.MessageKey != key {
			t.Errorf("键 %q 没随响应发出（message_key = %q）", key, body.MessageKey)
		}
	}
}

// 细节通道：句子与原因是两个字段。
//
// 为什么必须有这条测试：把原因拼回句子里（原先那 5 处 "参数错误: " + err.Error() 就是这种写法）
// 的表现是英文界面读到半中半英的串，而拼回来的动作在编译期完全合法 —— 只有测试能拦住。
func TestErrorKeyDetailSeparatesSentenceFromReason(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	BadRequestKeyDetail(c, MsgParameterError, "name: required")

	body := decodeBody(t, rec)
	if body.Message != zhMessages[MsgParameterError] {
		t.Errorf("句子不是登记值：%q", body.Message)
	}
	if body.MessageDetail != "name: required" {
		t.Errorf("原因没随响应发出：%q", body.MessageDetail)
	}
	if body.MessageKey != MsgParameterError {
		t.Errorf("键没发出：%q", body.MessageKey)
	}
	if strings.Contains(body.Message, "name: required") {
		t.Error("原因被拼进句子里了 —— 那正是这一格要拆开的问题")
	}

	// 没有原因时不许出现空字段：老前端逐字段渲染，多一个空 message_detail 就多看一个空位。
	rec2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(rec2)
	BadRequestKey(c2, MsgParameterError)
	var raw map[string]any
	if err := json.Unmarshal(rec2.Body.Bytes(), &raw); err != nil {
		t.Fatalf("解析响应体：%v", err)
	}
	if _, ok := raw["message_detail"]; ok {
		t.Error("没带原因时不该出现 message_detail 字段（omitempty 没生效）")
	}
}

func TestTextFallsBackToTheKeyItself(t *testing.T) {
	// 没登记的键返回键名。不是空白，也不是猜一句通用文案 ——
	// 界面上出现一个 snake_case 词，一眼就是"这里漏登记了"。
	if got := Text("no_such_key_yet"); got != "no_such_key_yet" {
		t.Errorf("未登记键应原样返回键名，得到 %q", got)
	}
}

func TestKeyForErrorRecognisesSentinels(t *testing.T) {
	for _, item := range []struct {
		err error
		key string
	}{
		{ErrUnauthorized, MsgSignedOutOrTokenInvalid},
		{ErrNotFound, MsgResourceNotFound},
		{ErrBadRequest, MsgRequestParameterError},
		{ErrInternal, MsgInternalError},
	} {
		key, ok := KeyForError(item.err)
		if !ok || key != item.key {
			t.Errorf("%v 应映射到 %q，得到 %q (ok=%v)", item.err, item.key, key, ok)
		}
		// 包装过的也要认出来（服务层常按 %w 往上抛）
		wrapped := errors.Join(errors.New("上下文"), item.err)
		if got, ok := KeyForError(wrapped); !ok || got != item.key {
			t.Errorf("包装后的 %v 没被认出来：got=%q ok=%v", item.err, got, ok)
		}
	}
}

func TestKeyForErrorLeavesUnknownErrorsAlone(t *testing.T) {
	// 认不出来就不猜：压成通用文案等于把细节弄丢，而细节常常正是用户要的那句。
	if _, ok := KeyForError(errors.New("某条服务层的中文错误")); ok {
		t.Error("未登记的错误不该被认成有文案键")
	}
}

func TestErrorWithErrUsesTheKeyForSentinels(t *testing.T) {
	gin.SetMode(gin.TestMode)

	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)
	ErrorWithErr(c, ErrUnauthorized, http.StatusForbidden)

	body := decodeBody(t, rec)
	if body.MessageKey != MsgSignedOutOrTokenInvalid {
		t.Errorf("共享 sentinel 没带 message_key：%+v", body)
	}
	if rec.Code != http.StatusForbidden {
		t.Errorf("状态码没透传：%d", rec.Code)
	}

	rec2 := httptest.NewRecorder()
	c2, _ := gin.CreateTestContext(rec2)
	plain := errors.New("某条服务层的中文错误")
	ErrorWithErr(c2, plain)
	body2 := decodeBody(t, rec2)
	if body2.MessageKey != "" {
		t.Error("未登记的错误不该被安上一个键")
	}
	if body2.Message != plain.Error() {
		t.Errorf("未登记的错误应原样返回：%q", body2.Message)
	}
}

func TestRegisteredTextsAreDistinctAndFilled(t *testing.T) {
	// 两句一模一样的话开两个键，前端就得维护两份相同的译文，
	// 而它们会在下一次改文案时分头走。重复在这里点名，合并由人决定。
	seen := map[string]string{}
	for key, text := range zhMessages {
		if text == "" {
			t.Errorf("键 %q 登记了空文案", key)
		}
		if other, ok := seen[text]; ok {
			t.Errorf("键 %q 与 %q 的中文完全相同（%q）—— 该合并成一个键", other, key, text)
		}
		seen[text] = key
	}
}
