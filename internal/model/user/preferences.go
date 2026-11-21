package user

import (
	"encoding/json"
	"errors"
	"sort"
	"strings"
)

// 这些校验错只进日志：API 层把它们统一折成 ErrBadRequest 发给前端，
// 因为"你的偏好里那个外观名字我不认识"这种话直出去，等于把服务端的取值清单暴露成接口面。
var (
	ErrPreferencesEmpty        = errors.New("偏好载荷为空")
	ErrPreferencesTheme        = errors.New("偏好里的外观取值不在登记清单内")
	ErrPreferencesMaterial     = errors.New("偏好里的材质取值不在登记清单内")
	ErrPreferencesMotion       = errors.New("偏好里的动效强度取值不在登记清单内")
	ErrPreferencesContrast     = errors.New("偏好里的对比度取值不在登记清单内")
	ErrPreferencesTransparency = errors.New("偏好里的透明度取值不在登记清单内")
	ErrPreferencesAccent       = errors.New("偏好里的强调色取值不在登记清单内")
	ErrPreferencesPersona      = errors.New("偏好里的版面人格取值不在登记清单内")
	ErrPreferencesTooLong      = errors.New("偏好载荷超过落库上限")
)

// 个性化偏好的服务端落点（2026-10-16）。
//
// 为什么要有这一层：外观 × 材质 × 透明变体以前只住浏览器 localStorage，
// 换设备、清缓存、换个浏览器就全丢 —— 而 "Citebase 支持四套外观" 这句话在产品上是真的，
// 在账号上却是假的。这一层把它变成账号属性。
//
// **存的是校验过的 JSON 文本，不是三个列**：这三项永远一起读写、永远不需要按值查询，
// 拆成列只会在每次加一档偏好时多一次 DDL。代价是要有一个**唯一的校验入口**（下面那个），
// 否则"任意 JSON"就是个注入与脏值面。
const (
	// PrefThemeAuto 是"跟随系统"这个伪外观：它不是调色板，而是"由 matchMedia 决定用哪个"。
	PrefThemeAuto = "auto"
	// PrefModeAuto 是三条行为档（动效强度 / 对比度 / 透明度）共用的"跟随系统"取值。
	// 它们各自是独立信号，但"没表过态"这件事在协议里只有一个写法。
	PrefModeAuto = "auto"
	// PrefPersonaPlain 是版面人格的默认档（这一档没有 auto，见 PersonaIDs 注释）。
	PrefPersonaPlain = "plain"
	// PreferencesColumnMaxLen 是落库前的硬上限。偏好载荷正常不到 200 字节；
	// 这个数的作用不是宽容，而是**拒绝** —— 没有它，PUT /auth/me/preferences 就是一个
	// 每个登录用户可写、无上限的 TEXT 列，等于把数据库当别人的免费存储。
	PreferencesColumnMaxLen = 2048
)

// ThemeIDs / MaterialIDs / MotionIDs / ContrastIDs / TransparencyIDs 是服务端认得的取值清单。
//
// **它们不是真相源**，前端 `web/src/styles/theme/themes.ts` 的 THEME_REGISTRY / MATERIALS 才是。
// 两份清单跨语言，没法互相 import，所以由 `tools/repocheck/preference_contract_test.go`
// 逐条比对两边并当场红 —— 那条测试是这整层唯一的安全带：加了外观不登记，
// 结果不是"新档不能用"，而是**用户在另一台设备上的选择被当成非法值丢掉**。
var (
	ThemeIDs    = []string{"light", "dark", "editorial", "warm-paper"}
	MaterialIDs = []string{"solid", "glass"}
	// 行为三档（2026-10-16 批2）：真相源在前端 `web/src/styles/theme/personalization.ts`。
	// 与外观那条**不同的写法**：`auto` 直接写进数组里，不再单独住一个常量。
	// 上一次踩的坑正是"伪取值住在另一个常量里、判据只比主清单"（theme 的 auto 漏在
	// PrefThemeAuto，跨语言门禁第一次跑就红）；清单自带 auto，比对就不需要再拼一次。
	MotionIDs       = []string{"auto", "full", "soft", "reduced"}
	ContrastIDs     = []string{"auto", "standard", "more"}
	TransparencyIDs = []string{"auto", "full", "reduce"}
	// 强调色清单（批3）。真相源是前端 web/src/styles/theme/accents.ts 的 ACCENT_REGISTRY，
	// 跨语言比对由 preference_contract_test.go 按色板逐条解析（同时校验每档都有浅/深两枚 hex）。
	AccentIDs = []string{"auto", "blue", "purple", "pink", "red", "orange", "yellow", "green", "teal"}
	// 版面人格（批4）：plain = 静置（默认），signature = 公开版面的入场编排。
	// 这一档【没有 auto】：系统里没有对应开关，所以也不存在“跟随系统”那种回落。
	PersonaIDs = []string{"plain", "signature"}
)

// Preferences 是六项偏好的载荷。放在同一个包里，因为它同时是
// API 的请求体、落库前校验的形状，和 GET /auth/me 回给前端的那几格。
type Preferences struct {
	Theme      string `json:"theme"`
	Material   string `json:"material"`
	GlassClear bool   `json:"glass_clear"`
	// 行为三档：动效强度 / 对比度 / 透明度。默认值一律 auto = "这台设备没表过态"，
	// 也就是继续由系统的三个开关决定（与前端 DEFAULT_MOTION/CONTRAST/TRANSPARENCY 同一条）。
	Motion       string `json:"motion"`
	Contrast     string `json:"contrast"`
	Transparency string `json:"transparency"`
	// 强调色档：auto = 用每套外观自己设计的那枚（品牌定位=技术中立，默认观感一个像素不变）。
	Accent string `json:"accent"`
	// 版面人格：只影响登录 / 分享 / 安装 / 错误页这些低频版面（射程由设计教条 G34 拦）。
	Persona string `json:"persona"`
	// UpdatedAt 由服务端盖写（客户端给的一律丢弃）：它是"哪一份更新"的**唯一判据**。
	// 前端拿它和本地记录的改动时间比，谁新用谁 —— 没有它，多设备之间只能后写覆盖先写，
	// 而"我刚在这台调好的主题被另一台的旧选择冲掉了"是这类同步最容易被忽略的坏味道。
	UpdatedAt int64 `json:"updated_at"`
}

// ValidThemeIDs 返回含 'auto' 的合法外观清单（排序后），给校验与门禁共用。
func ValidThemeIDs() []string {
	out := append([]string{}, ThemeIDs...)
	out = append(out, PrefThemeAuto)
	sort.Strings(out)
	return out
}

// ValidMaterialIDs 返回材质清单（排序后）。
func ValidMaterialIDs() []string {
	out := append([]string{}, MaterialIDs...)
	sort.Strings(out)
	return out
}

// ValidMotionIDs / ValidContrastIDs / ValidTransparencyIDs 返回三条行为档的合法取值（排序后）。
// 清单里已经自带 "auto"（见上面那段注释），所以这里**不需要再补伪取值**。
func ValidMotionIDs() []string { return sortedCopy(MotionIDs) }

func ValidContrastIDs() []string { return sortedCopy(ContrastIDs) }

func ValidTransparencyIDs() []string { return sortedCopy(TransparencyIDs) }

func ValidAccentIDs() []string { return sortedCopy(AccentIDs) }

func ValidPersonaIDs() []string { return sortedCopy(PersonaIDs) }

func sortedCopy(in []string) []string {
	out := append([]string{}, in...)
	sort.Strings(out)
	return out
}

func contains(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

// NormalizePreferences 校验并归一一份偏好载荷。
//
// 三件事它必须做，每一件都对应一个真实的坏结果：
//  1. **取值不在清单里就整体报错**，不"认得出多少算多少" —— 静默丢一半会让人以为同步生效了，
//     下次开页面却是另一套（最难复现的那类 bug）；
//  2. **solid 材质下强制 glass_clear=false**：透明变体依附玻璃，app 侧 effectiveMaterial 会把它塌回实心，
//     服务端存一个"永远不会生效 true"等于存脏值，还会让两端的判据分叉；
//  3. **丢掉客户端给的 updated_at**：时钟在用户机器上，拿它做新旧判定就是让客户端自己决定谁赢。
func NormalizePreferences(p *Preferences) error {
	if p == nil {
		return ErrPreferencesEmpty
	}
	p.Theme = strings.ToLower(strings.TrimSpace(p.Theme))
	p.Material = strings.ToLower(strings.TrimSpace(p.Material))
	p.Motion = strings.ToLower(strings.TrimSpace(p.Motion))
	p.Contrast = strings.ToLower(strings.TrimSpace(p.Contrast))
	p.Transparency = strings.ToLower(strings.TrimSpace(p.Transparency))
	if p.Theme == "" {
		p.Theme = PrefThemeAuto
	}
	// 三条行为档的空值同样落到 auto（= 把判断权交回系统的三个开关），
	// **不落到任何"生效档"**：空串意味着客户端没说话，服务端替它选 full 还是 reduced 就是替用户表态。
	if p.Motion == "" {
		p.Motion = PrefModeAuto
	}
	if p.Contrast == "" {
		p.Contrast = PrefModeAuto
	}
	if p.Transparency == "" {
		p.Transparency = PrefModeAuto
	}
	if p.Accent == "" {
		p.Accent = PrefModeAuto
	}
	// persona 的空值**不落到 auto**（清单里根本没这一档），落到 plain：
	// 这一档的默认语义是“没要求编排”，与“系统开关缺席时该怎么做”是同一件事。
	if p.Persona == "" {
		p.Persona = PrefPersonaPlain
	}
	if !contains(ValidThemeIDs(), p.Theme) {
		return ErrPreferencesTheme
	}
	if !contains(MaterialIDs, p.Material) {
		return ErrPreferencesMaterial
	}
	if !contains(MotionIDs, p.Motion) {
		return ErrPreferencesMotion
	}
	if !contains(ContrastIDs, p.Contrast) {
		return ErrPreferencesContrast
	}
	if !contains(TransparencyIDs, p.Transparency) {
		return ErrPreferencesTransparency
	}
	if !contains(AccentIDs, p.Accent) {
		return ErrPreferencesAccent
	}
	if !contains(PersonaIDs, p.Persona) {
		return ErrPreferencesPersona
	}
	if p.Material != "glass" {
		p.GlassClear = false
	}
	return nil
}

// MarshalPreferences 校验后序列化成落库文本。
func MarshalPreferences(p *Preferences) (string, error) {
	if err := NormalizePreferences(p); err != nil {
		return "", err
	}
	p.UpdatedAt = 0 // 由调用方（服务层）盖写；序列化前归零，免得客户端的值落进 JSON
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	if len(raw) > PreferencesColumnMaxLen {
		return "", ErrPreferencesTooLong
	}
	return string(raw), nil
}

// ParsePreferences 从落库文本读回偏好。空串/NULL 表示"这个账号从没同步过"，返回 (nil, nil)
// —— 前端要靠这个区分"没有服务器值"和"服务器值就是默认的浅色实心"，
// 少了这个区分，第一次登录就会把用户本地调好的档位盖掉。
func ParsePreferences(raw string) (*Preferences, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}
	var p Preferences
	if err := json.Unmarshal([]byte(trimmed), &p); err != nil {
		return nil, err
	}
	// 库里可能躺着旧版本写入的值（比如删掉某一档外观之后的存量行）：读的一样要判，
	// 否则非法值会一路带到界面上，而前端拿它去 THEME_REGISTRY 里找不到就什么都不画。
	if err := NormalizePreferences(&p); err != nil {
		return nil, err
	}
	return &p, nil
}
