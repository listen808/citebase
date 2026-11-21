package repository

import (
	"strings"
	"testing"

	"github.com/listen808/citebase/internal/brand"
)

// 这条测试钉的是"占位符数量 == 参数数量"。
//
// 它值得单独存在，因为上一版的错不是编译错、也不是 panic，而是 MySQL 报
// "sql: expected 11 arguments, got 13" —— 归一于是**静默没发生**，界面上继续挂着
// 改名前的产品名（2026-10-12 实跑 GET /api/v1/site 才看见）。
// 那条错误只会在真实库里出现一次，而它失败的方式是"什么都没发生"，
// 所以必须有一条不连库就能跑的判据顶在前面。
//
// 反向验证：把 SET 用的值再加进 args 头部 → 第一、二条立刻红；
// 把某个键名写成两个逗号（占位符多一个）→ 第一条红。

func TestLegacySiteNameUpdatePlaceholderArithmetic(t *testing.T) {
	names := []string{"AIWiki", "AI Wiki", "MM-Wiki"}
	where, args, _ := legacySiteNameUpdate(names, 1791744103)

	if got := strings.Count(where, "?"); got != len(args) {
		t.Errorf("占位符 %d 个、参数 %d 个，数量不等（MySQL 会直接报错，而归一会静默不发生）：%s",
			got, len(args), where)
	}
	if len(args) != len(names)+2 {
		t.Errorf("参数应是「两个键 + %d 个旧名」= %d 个，得到 %d 个", len(names), len(names)+2, len(args))
	}
}

func TestLegacySiteNameUpdateArgumentOrder(t *testing.T) {
	names := brand.LegacySiteNames()
	_, args, sets := legacySiteNameUpdate(names, 1791744103)

	// 前两个参数必须是键名，其余才是旧值 —— 顺序与占位符顺序一致，
	// 上一版就是把 SET 的值排在前面喂进 WHERE 的。
	if args[0] != brand.ConfigKeySiteName || args[1] != brand.ConfigKeySiteNameLegacy {
		t.Errorf("WHERE 的键位参数不对：%v / %v", args[0], args[1])
	}
	for i, name := range names {
		if got, ok := args[i+2].(string); !ok || got != name {
			t.Errorf("第 %d 个旧名参数应是 %q，得到 %v", i+2, name, args[i+2])
		}
	}
	// 要写的值走 SET 的 map，不许同时出现在 WHERE 的参数里（那正是上一版的形状）。
	for _, arg := range args {
		if v, ok := arg.(string); ok && v == sets["value"] {
			t.Errorf("SET 的值 %q 混进了 WHERE 参数 —— 参数个数就会比占位符多", v)
		}
	}
}

func TestLegacySiteNameUpdateCoversTheRealStaleValue(t *testing.T) {
	// 2026-10-12 实测这台机器上的库里存的就是 "AIWiki"。清单少这一条，
	// 归一就成了 no-op，而唯一能发现的人是打开登录页的用户。
	names := brand.LegacySiteNames()
	found := false
	for _, n := range names {
		if n == "AIWiki" {
			found = true
		}
	}
	if !found {
		t.Errorf("LegacySiteNames 不含实跑量到的那个存量值 AIWiki：%v", names)
	}
}
