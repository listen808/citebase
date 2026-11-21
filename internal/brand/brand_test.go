package brand

import (
	"errors"
	"strings"
	"testing"
)

// 这个文件钉的是"名字从哪来"这一条判据，所以它必须不依赖数据库就能跑。
// 反向验证做过：把两个参数的先后次序颠倒、把 TrimSpace 判据改成 != "" 之外的写法、
// 或把默认名分支删掉，对应的用例立刻红。

func TestResolveSiteNamePrefersMainTitle(t *testing.T) {
	if got := ResolveSiteName("团队百科", "旧的系统名"); got != "团队百科" {
		t.Errorf("main_title 应当优先，得到 %q", got)
	}
}

func TestResolveSiteNameFallsBackToLegacyKey(t *testing.T) {
	// 现存库里"只填过系统名称、没填主页标题"的情况是真实存在的，
	// 接上新键不能把老库已经生效的名字换成默认名。
	if got := ResolveSiteName("   ", "科研笔记库"); got != "科研笔记库" {
		t.Errorf("main_title 空白时应退到 system_name，得到 %q", got)
	}
}

func TestResolveSiteNameFallsBackToDefault(t *testing.T) {
	for _, pair := range [][2]string{{"", ""}, {"  ", "\t\n "}} {
		if got := ResolveSiteName(pair[0], pair[1]); got != DefaultName {
			t.Errorf("两个键都空白（%q/%q）应退回默认名，得到 %q", pair[0], pair[1], got)
		}
	}
}

func TestResolveSiteNameKeepsTheValueVerbatim(t *testing.T) {
	// 判据是"整串空白才算没填"；填了就原样返回，不改写管理员输入。
	if got := ResolveSiteName(" 团队百科 ", ""); got != " 团队百科 " {
		t.Errorf("覆盖值被改写了：得到 %q，期望原样 %q", got, " 团队百科 ")
	}
}

func TestSiteNameWithoutReader(t *testing.T) {
	restore := reader
	defer InstallBrandReader(restore)

	InstallBrandReader(nil)
	if got := SiteName(); got != DefaultName {
		t.Errorf("没注入读取口子时应返回默认名，得到 %q", got)
	}
}

func TestSiteNameFallsBackWhenConfigTableMissing(t *testing.T) {
	restore := reader
	defer InstallBrandReader(restore)

	// 安装向导第一步时 aw_config 还是空表 / 不存在，分享页仍然要能渲染。
	InstallBrandReader(func(string) (string, error) { return "", errors.New("no such table") })
	if got := SiteName(); got != DefaultName {
		t.Errorf("读配置失败时应退回默认名，得到 %q", got)
	}
}

func TestSiteNameStillWorksWithoutLegacyRow(t *testing.T) {
	restore := reader
	defer InstallBrandReader(restore)

	InstallBrandReader(func(key string) (string, error) {
		if key == ConfigKeySiteNameLegacy {
			return "", errors.New("row not found")
		}
		return "新名字", nil
	})
	if got := SiteName(); got != "新名字" {
		t.Errorf("历史键缺失不该影响 main_title 的值，得到 %q", got)
	}
}

func TestSiteNameReadsBothWiredKeys(t *testing.T) {
	restore := reader
	defer InstallBrandReader(restore)

	var asked []string
	InstallBrandReader(func(key string) (string, error) {
		asked = append(asked, key)
		return "", nil
	})
	SiteName()

	// 这条钉的是"读哪两个键"。种子、管理端 UI、门禁三处都按这两个键对齐，
	// 哪天改了键名却没动种子，管理员在界面上改的还是空气。
	want := []string{ConfigKeySiteName, ConfigKeySiteNameLegacy}
	if len(asked) != len(want) || asked[0] != want[0] || asked[1] != want[1] {
		t.Errorf("SiteName 读的键序列是 %v，期望 %v", asked, want)
	}
}

// 这份清单会被启动迁移拿去 UPDATE aw_config，所以它的每一条都必须真的只是"旧产品名的写法"。
// 混进 "Wiki"、"AI" 这类泛词的后果不是编译错误，是**安静地把管理员自己起的站点名改成默认名**。
// 这条测试就是那颗刹车；反向验证：往 LegacySiteNames 里加一个 "Wiki" 或 "AIWiki 笔记"，本条立刻红。
func TestLegacySiteNamesAreNothingButOldBrand(t *testing.T) {
	allowed := map[string]bool{"aiwiki": true, "mmwiki": true}
	seen := map[string]bool{}

	for _, name := range LegacySiteNames() {
		key := strings.ToLower(strings.NewReplacer(" ", "", "-", "", "_", "").Replace(name))
		if !allowed[key] {
			t.Errorf("LegacySiteNames 里有 %q（归一后 %q）—— 不是旧品牌名，不该被自动改写", name, key)
		}
		if seen[name] {
			t.Errorf("LegacySiteNames 里 %q 重复", name)
		}
		seen[name] = true
	}
	// 清单空了 = 归一变成 no-op，而老库存量值还在界面上；这种"静默不再生效"要能被发现。
	if len(seen) < 6 {
		t.Errorf("LegacySiteNames 只剩 %d 条，旧写法覆盖不足（改名前 README 与配置里同时出现过 AIWiki / AI Wiki / MM-Wiki）", len(seen))
	}
}

func TestSiteDescription(t *testing.T) {
	restore := reader
	defer InstallBrandReader(restore)

	InstallBrandReader(func(key string) (string, error) {
		if key == ConfigKeySiteDescription {
			return "  一句话描述  ", nil
		}
		return "", errors.New("unwired key: " + key)
	})
	if got := SiteDescription(); got != "一句话描述" {
		t.Errorf("描述应去掉首尾空白，得到 %q", got)
	}

	InstallBrandReader(func(string) (string, error) { return "", errors.New("boom") })
	if got := SiteDescription(); got != "" {
		t.Errorf("读不到描述应返回空串（由调用方退回语言包），得到 %q", got)
	}
}
