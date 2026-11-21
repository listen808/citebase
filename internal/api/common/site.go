package common

import (
	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/brand"
	"github.com/listen808/citebase/internal/response"
)

// SiteInfo 是"这个站点叫什么名字、怎么描述自己"的公开读取口。
//
// 为什么需要它：aw_config 里的 main_title / system_name / main_description 三行
// 此前只有写入没有读取 —— 全仓找不到一处消费方（连 model 里声明的 ConfigKeyMainTitle
// 都没被用过），于是管理员在「系统设置 → 基础」里改站点标题改的是空气，
// 而登录页、标签页标题、分享页上显示的名字来自前端另写的硬编码。
// 这个端点就是把"管理员能改的"和"用户看见的"接上的一环。
//
// 为什么是公开端点：登录页在拿到任何令牌之前就要显示名字。
// 为什么不塞进 /health：/health 是给编排层与监控脚本探活用的（它带 version 已经是妥协），
// 展示数据混进存活探针里，等于让每次健康检查都依赖配置表。
//
// 回名字与描述。描述（main_description）此前被判过"不进这个端点"，理由是它唯一的生效点
// 在服务端（分享页的 shareGenericDescription），多带一个字段就是下一份没人取的数据。
// 现在消费点出现了：SPA 自己的 <meta name="description"> 是构建产物 index.html 里写死的中文，
// 静态 HTML 不跟着界面语言走，只有前端落地时拿这个字段（没填则退回语言包那句一句话定位）
// 才盖得掉 —— 于是它和名字一样是"管理员改了就得读得到"的那一栏。
// 分享页仍走服务端渲染那条路，两处不共用一个出口。
func SiteInfo(c *gin.Context) {
	response.Success(c, gin.H{
		"name":        brand.SiteName(),
		"description": brand.SiteDescription(),
	})
}
