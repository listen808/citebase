package common

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/brand"
)

// 这个文件钉的是"管理员在后台改的站点名，真的能从公开端点读到"。
//
// 起因：main_title / system_name / main_description 三行配置此前只有写入没有读取，
// 后台那一栏编辑的是空气，界面上的名字来自前端硬编码。接上读取路径之后，
// 端点必须按 internal/brand 的优先级回话，且不能因为配置表读不到就 500
// —— 登录页在装表之前也要显示名字。
//
// 反向验证做过：把 handler 改成直接返回 brand.DefaultName（不读配置）→ 第一条红；
// 把 reader 报错时的兜底去掉 → 第三条红。

func newSiteRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/site", SiteInfo)
	return r
}

func fetchSiteName(t *testing.T, r *gin.Engine) map[string]any {
	t.Helper()

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/site", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("公开端点不该要求认证：HTTP %d", rec.Code)
	}

	var body struct {
		Code int            `json:"code"`
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("解析响应体：%v（原文 %s）", err, rec.Body.String())
	}
	if body.Code != 0 {
		t.Fatalf("业务码期望 0，得到 %d", body.Code)
	}
	return body.Data
}

func TestSiteInfoReturnsAdminOverride(t *testing.T) {
	defer brand.InstallBrandReader(nil)

	brand.InstallBrandReader(func(key string) (string, error) {
		if key == brand.ConfigKeySiteName {
			return "团队百科", nil
		}
		return "", nil
	})

	data := fetchSiteName(t, newSiteRouter())
	if data["name"] != "团队百科" {
		t.Errorf("端点没把管理员的覆盖值回给前端：得到 %v", data["name"])
	}
}

func TestSiteInfoSurvivesUnreadableConfig(t *testing.T) {
	defer brand.InstallBrandReader(nil)

	// 装表之前（或 DB 抖动时）读配置会失败。此时仍要 200 并给出代码默认名 ——
	// 登录页拿不到名字是一次正常的降级，不该变成白屏或 500。
	brand.InstallBrandReader(func(string) (string, error) { return "", errors.New("no such table") })

	data := fetchSiteName(t, newSiteRouter())
	if data["name"] != brand.DefaultName {
		t.Errorf("读不到配置时应退回默认名，得到 %v", data["name"])
	}
}

func TestSiteInfoWithoutReader(t *testing.T) {
	defer brand.InstallBrandReader(nil)

	brand.InstallBrandReader(nil)
	data := fetchSiteName(t, newSiteRouter())
	if data["name"] != brand.DefaultName {
		t.Errorf("未注入读取口子时应退回默认名，得到 %v", data["name"])
	}
}

// 描述与名字同批进这个端点（前端拿它盖掉 index.html 里写死的静态默认值）。
// 反向验证：把 description 从 gin.H 里删掉 → 下面第二条红（缺字段，不是值错）。
func TestSiteInfoReturnsAdminDescription(t *testing.T) {
	defer brand.InstallBrandReader(nil)

	brand.InstallBrandReader(func(key string) (string, error) {
		if key == brand.ConfigKeySiteDescription {
			return "  一句话定位  ", nil
		}
		return "", nil
	})

	data := fetchSiteName(t, newSiteRouter())
	if data["description"] != "一句话定位" {
		t.Errorf("主页描述应裁掉首尾空白后回给前端：得到 %q", data["description"])
	}
}

func TestSiteInfoDescriptionPresentEvenWhenUnset(t *testing.T) {
	defer brand.InstallBrandReader(nil)

	brand.InstallBrandReader(func(string) (string, error) { return "", nil })

	data := fetchSiteName(t, newSiteRouter())
	// 空串是合法值（前端据此退回语言包那句一句话定位），但字段本身不能缺：
	// 缺了前端读到 undefined，判断分支会当成"后端没这功能"而不是"没填"。
	v, ok := data["description"]
	if !ok {
		t.Fatal("响应里没有 description 字段")
	}
	if v != "" {
		t.Errorf("没填描述时应是空串，得到 %v", v)
	}
}
