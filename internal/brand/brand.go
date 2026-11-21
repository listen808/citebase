// Package brand 是产品名在后端的唯一落点。
//
// 为什么要专门开一个包：2026-10-07 改名之后，"Citebase" 又被抄回了六处 ——
// 安装向导的种子写 "Citebase 知识库"、db.go 与 data.sql 写 "Citebase"、
// 分享页的 og:site_name、测试邮件的主题与正文、巡检邮件主题、/health 的 message。
// 更糟的是 aw_config 里的 main_title / system_name / main_description 三行
// **从头到尾没有任何读取方**（model 里的 ConfigKeyMainTitle 声明完就没人用过），
// 于是管理员在「系统设置」里改"主页标题"改的是空气，界面上那几个名字来自另一处硬编码。
//
// 现在真值只有两层：代码默认（DefaultName）与管理员覆盖（aw_config），
// 两者的优先级由 ResolveSiteName 定义一次；后端要显示名字的地方一律调 SiteName()。
// 前端对应的一份是 web/src/utils/core/app-info.ts 的 APP_NAME，
// 三处种子与两侧一致由 tools/repocheck/brand_source_test.go 校验。
package brand

import "strings"

// DefaultName 是代码默认站点名。改名只改这一处 + 前端 APP_NAME，其余引用。
const DefaultName = "Citebase"

// 站点名与描述的配置键。main_title 是正脸，system_name 是同表里的历史键 ——
// 两个都读（见 ResolveSiteName），因为现存库里管理员只填过其中一个的情况真实存在。
const (
	ConfigKeySiteName        = "main_title"
	ConfigKeySiteNameLegacy  = "system_name"
	ConfigKeySiteDescription = "main_description"
)

// 对外发请求时自报的身份（网页抓取与 AI 工具调用）。
//
// 写成常量而不是调 SiteName()：管理员在后台改"主页标题"，改的是给自己用户看的名字，
// 不该顺带改变本项目向别人的服务器自报的身份 —— UA 会被对方记进访问日志，
// 而且抓取行为要能被稳定归因。
const (
	UserAgent   = DefaultName + "-Bot/1.0 (+compatible; https://github.com/listen808/citebase)"
	UserAgentAI = DefaultName + "-AI/1.0"
)

// reader 由 app/server 在数据库就绪后注入。brand 不直接 import repository 或 service：
// 种子那边是 repository 引 brand，brand 反过来引它就会成环。
var reader func(key string) (string, error)

// InstallBrandReader 注入读配置的口子。
func InstallBrandReader(r func(key string) (string, error)) {
	reader = r
}

// LegacySiteNames 是改名前那几个产品名的常见写法，给启动归一用
// （见 repository.normalizeLegacySiteNames）。
// 逐个列写法而不是在 SQL 里做归一化：MySQL 的比较本身大小写不敏感，
// 但空格与连字符的变体不会被视为相等，而"我的 AIWiki 笔记"这种带别的内容的值必须不动。
func LegacySiteNames() []string {
	return []string{"AIWiki", "AI Wiki", "AIWIKI", "aiwiki", "MM-Wiki", "MMWiki", "mm-wiki", "MM Wiki"}
}

// ResolveSiteName 定义"管理员覆盖 vs 代码默认"唯一的优先级：
// main_title 去掉首尾空白后非空就用它，否则退到历史键 system_name，两者都没填才用默认名。
// 值本身原样返回（不改大小写、不去内部空格）—— 悄悄改写管理员输入的字符串，
// 比名字里带一个尾随空格更糟。
func ResolveSiteName(mainTitle, systemName string) string {
	if trimmed := strings.TrimSpace(mainTitle); trimmed != "" {
		return mainTitle
	}
	if trimmed := strings.TrimSpace(systemName); trimmed != "" {
		return systemName
	}
	return DefaultName
}

// SiteName 是当前站点展示名，后端所有对外文案（分享页标签、邮件、巡检报告）都走这里。
// 未注入、未建表或查询失败都退回默认名 —— 这是"名字"，不值得为它让一次请求 500。
func SiteName() string {
	mainTitle, err := read(ConfigKeySiteName)
	if err != nil {
		return DefaultName
	}
	systemName, err := read(ConfigKeySiteNameLegacy)
	if err != nil {
		// 历史键读不到（老库里没这一行）不影响 main_title 已经填过的值。
		systemName = ""
	}
	return ResolveSiteName(mainTitle, systemName)
}

// SiteDescription 是站点描述，供分享页与 meta 标签用；没填就是空串（前端据此退回语言包）。
func SiteDescription() string {
	value, err := read(ConfigKeySiteDescription)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(value)
}

func read(key string) (string, error) {
	if reader == nil {
		return "", errNoReader
	}
	return reader(key)
}

var errNoReader = brandError("brand: 未注入配置读取口子")

type brandError string

func (e brandError) Error() string { return string(e) }
