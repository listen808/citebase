package auth

import (
	"testing"

	adminmodel "github.com/listen808/citebase/internal/model/admin"
)

// 这条测的是"重复走安装向导会不会抹掉管理员自己的设置"。
//
// 反向验证：把 mergeConfigSeed 里的 `if isNew` 去掉（恢复成原先的无条件覆盖 Value），
// 第二条 TestMergeConfigSeedKeepsAdminValueOnExistingRow 立刻红。

func TestMergeConfigSeedWritesValueOnlyWhenCreated(t *testing.T) {
	seed := adminmodel.Config{Key: "main_title", Value: "Citebase", Label: "主页标题", GroupKey: "basic", Type: "text", Sort: 4}
	var zero adminmodel.Config

	got := mergeConfigSeed(zero, seed, true, 1000)

	if got.Value != "Citebase" {
		t.Errorf("新建的行应落下种子默认值，得到 %q", got.Value)
	}
	if got.CreateTime != 1000 || got.UpdateTime != 1000 {
		t.Errorf("新建的行应同时盖创建与更新时间，得到 create=%d update=%d", got.CreateTime, got.UpdateTime)
	}
	if got.Key != "main_title" {
		t.Errorf("键名被改写了：%q（键名是 FirstOrCreate 的匹配条件，动了就会插出第二行）", got.Key)
	}
}

func TestMergeConfigSeedKeepsAdminValueOnExistingRow(t *testing.T) {
	// 站点名现在真的生效了（internal/brand 读它），所以"再点一次向导"绝不能再把它冲回默认值 ——
	// 那正是本项目"能编辑不生效"之后紧接着的另一种骗人形态：能编辑，但随时会被抹掉。
	existing := adminmodel.Config{ID: 7, Key: "main_title", Value: "团队百科", Label: "旧标签", GroupKey: "basic", Type: "text", Sort: 99, CreateTime: 500}
	seed := adminmodel.Config{Key: "main_title", Value: "Citebase", Label: "主页标题", GroupKey: "basic", Type: "text", Sort: 4}

	got := mergeConfigSeed(existing, seed, false, 1000)

	if got.Value != "团队百科" {
		t.Errorf("已存在的行被写回种子值：得到 %q，期望保留管理员的 %q", got.Value, "团队百科")
	}
	// 值不动，但结构属性必须跟上清单，否则清单的漂移永远修不到现存库上。
	if got.Label != "主页标题" || got.Sort != 4 {
		t.Errorf("元数据没对齐清单：label=%q sort=%d", got.Label, got.Sort)
	}
	if got.ID != 7 || got.CreateTime != 500 {
		t.Errorf("把已存在的行当成新行写了：id=%d create=%d", got.ID, got.CreateTime)
	}
	if got.UpdateTime != 1000 {
		t.Errorf("更新时间没刷新：%d", got.UpdateTime)
	}
}

func TestMergeConfigSeedBackfillsSelectOptions(t *testing.T) {
	// 旧写法从不写 Options：走向导装出来的库，select 那一栏下拉是空的（表现是"没得选"而不是报错）。
	// 现行种子里已没有 select 键（AI 分组 2026-10-19 删除，其真源在 configs/config.yaml），
	// 但 mergeConfigSeed 对 select 行补 Options 的行为仍在 —— 用构造的 seed 钉住，
	// 将来清单再出现 select 键时不会复发。
	seed := adminmodel.Config{Key: "sample_select", Value: "a", Label: "示例", GroupKey: "basic", Type: "select", Options: "a,b,c", Sort: 1}
	existing := adminmodel.Config{ID: 3, Key: "sample_select", Value: "b", Options: ""}

	got := mergeConfigSeed(existing, seed, false, 1000)

	if got.Options == "" {
		t.Errorf("select 类型的行没有补齐 Options，管理端下拉会是空的")
	}
	if got.Value != "b" {
		t.Errorf("补 Options 时把用户选的值也改了：%q", got.Value)
	}
}
