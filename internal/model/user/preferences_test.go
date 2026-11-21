package user

import (
	"strings"
	"testing"
)

// 这台是纯函数测试（不连库）：偏好这一层的价值全在"写进去的一定合法、读出来的一定能用"，
// 而那两件事不需要 MySQL 就能证。

func TestNormalizePreferences(t *testing.T) {
	tests := []struct {
		name         string
		in           Preferences
		wantTheme    string
		wantMaterial string
		wantClear    bool
		wantErr      error
	}{
		{"合法组合原样通过", Preferences{Theme: "dark", Material: "glass", GlassClear: true}, "dark", "glass", true, nil},
		{"auto 是合法外观", Preferences{Theme: "auto", Material: "solid"}, "auto", "solid", false, nil},
		{"空外观按跟随系统处理", Preferences{Material: "solid"}, "auto", "solid", false, nil},
		{"大小写与空白归一", Preferences{Theme: "  Dark ", Material: " GLASS "}, "dark", "glass", false, nil},
		{"solid 下的 clear 被强制关掉", Preferences{Theme: "light", Material: "solid", GlassClear: true}, "light", "solid", false, nil},
		{"不认识的外观整体报错", Preferences{Theme: "neon", Material: "solid"}, "", "", false, ErrPreferencesTheme},
		{"不认识的材质整体报错", Preferences{Theme: "light", Material: "frosted"}, "", "", false, ErrPreferencesMaterial},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.in
			err := NormalizePreferences(&in)
			if err != tc.wantErr {
				t.Fatalf("err = %v，想要 %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if in.Theme != tc.wantTheme || in.Material != tc.wantMaterial || in.GlassClear != tc.wantClear {
				t.Errorf("归一结果 = %+v，想要 theme=%q material=%q clear=%v", in, tc.wantTheme, tc.wantMaterial, tc.wantClear)
			}
		})
	}
}

func TestNormalizePreferencesNil(t *testing.T) {
	if err := NormalizePreferences(nil); err != ErrPreferencesEmpty {
		t.Fatalf("nil 载荷应当报 ErrPreferencesEmpty，实得 %v", err)
	}
}

// 客户端给的 updated_at 必须被丢掉：那个时钟在用户机器上，
// 留着它做多设备新旧判定等于让客户端自己决定谁赢。
func TestMarshalPreferencesDropsClientUpdatedAt(t *testing.T) {
	raw, err := MarshalPreferences(&Preferences{Theme: "light", Material: "glass", UpdatedAt: 999})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(raw, "999") {
		t.Errorf("序列化结果仍然带着客户端给的时间戳：%s", raw)
	}
}

func TestMarshalPreferencesSizeCap(t *testing.T) {
	// 材质清单里没有长值，所以要造出超限只能靠 theme —— 它在校验里就被拒了，
	// 这条测的是"万一清单里以后放进一个长值"，容量这一格仍然兜得住。
	p := &Preferences{Theme: "light", Material: "glass"}
	long := make([]byte, PreferencesColumnMaxLen+10)
	for i := range long {
		long[i] = 'a'
	}
	p.Theme = string(long)
	if _, err := MarshalPreferences(p); err != ErrPreferencesTheme {
		t.Fatalf("超长值应先在清单校验这一步被拒，实得 %v", err)
	}
}

func TestParsePreferences(t *testing.T) {
	t.Run("空串等于账号上没配过", func(t *testing.T) {
		p, err := ParsePreferences("   ")
		if err != nil || p != nil {
			t.Fatalf("空值应当返回 (nil, nil)，实得 (%v, %v)", p, err)
		}
	})
	t.Run("写进去读出来一致", func(t *testing.T) {
		raw, err := MarshalPreferences(&Preferences{Theme: "editorial", Material: "solid"})
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		p, err := ParsePreferences(raw)
		if err != nil {
			t.Fatalf("parse: %v", err)
		}
		if p == nil || p.Theme != "editorial" || p.Material != "solid" {
			t.Fatalf("读回的结果不对：%+v", p)
		}
	})
	// 库里可能躺着"这一档外观后来被删掉了"的存量行。读的一边也必须判，
	// 否则非法值一路带到界面上，前端在 THEME_REGISTRY 里找不到它就什么都不画 ——
	// 表现是"设置页空白"，而没人会觉得这跟三年前的一个 JSON 有关。
	t.Run("存量非法值报错而不是带病返回", func(t *testing.T) {
		if _, err := ParsePreferences(`{"theme":"retro-blue","material":"solid","glass_clear":false,"updated_at":1}`); err != ErrPreferencesTheme {
			t.Fatalf("非法外观应当报错，实得 %v", err)
		}
	})
	t.Run("坏 JSON 报错", func(t *testing.T) {
		if _, err := ParsePreferences(`{"theme":`); err == nil {
			t.Fatal("截断的 JSON 应当报错")
		}
	})
}

func TestValidListsAreSortedAndComplete(t *testing.T) {
	themes := ValidThemeIDs()
	if len(themes) != len(ThemeIDs)+1 {
		t.Fatalf("合法外观应当是清单 + auto，实得 %d 个（清单 %d）", len(themes), len(ThemeIDs))
	}
	for i := 1; i < len(themes); i++ {
		if themes[i-1] > themes[i] {
			t.Fatalf("ValidThemeIDs 没排序：%v", themes)
		}
	}
	if !strings.Contains(strings.Join(themes, ","), PrefThemeAuto) {
		t.Errorf("合法外观里没有 auto：%v", themes)
	}
}

// TestNormalizeBehaviorAxes 管三条行为档（2026-10-16 批2）。
// 这一层唯一要紧的性质是：**没表过态 = auto**，而 auto 必须等于"由系统信号决定"，
// 所以服务端绝不能把空值落成任何一档"生效值"（落成 full 就等于替系统开了减弱动效的用户表了态）。
func TestNormalizeBehaviorAxes(t *testing.T) {
	tests := []struct {
		name             string
		in               Preferences
		wantMotion       string
		wantContrast     string
		wantTransparency string
		wantErr          error
	}{
		{"三项齐全原样通过", Preferences{Theme: "dark", Material: "solid", Motion: "soft", Contrast: "more", Transparency: "reduce"}, "soft", "more", "reduce", nil},
		{"空值落 auto", Preferences{Theme: "light", Material: "glass"}, "auto", "auto", "auto", nil},
		{"大小写与空白归一", Preferences{Theme: "light", Material: "glass", Motion: "  Reduced "}, "reduced", "auto", "auto", nil},
		{"不认识的动作档整体报错", Preferences{Theme: "light", Material: "glass", Motion: "turbo"}, "", "", "", ErrPreferencesMotion},
		{"不认识的对比度档整体报错", Preferences{Theme: "light", Material: "glass", Contrast: "max"}, "", "", "", ErrPreferencesContrast},
		{"不认识的透明度档整体报错", Preferences{Theme: "light", Material: "glass", Transparency: "solid"}, "", "", "", ErrPreferencesTransparency},
		{"不认识的强调色整体报错", Preferences{Theme: "light", Material: "glass", Accent: "chartreuse"}, "auto", "auto", "auto", ErrPreferencesAccent},
		{"强调色 auto 原样通过", Preferences{Theme: "light", Material: "glass", Accent: "auto"}, "auto", "auto", "auto", nil},
		{"人格空值落 plain（这一档没有 auto）", Preferences{Theme: "light", Material: "glass", Persona: ""}, "auto", "auto", "auto", nil},
		{"不认识的人格整体报错", Preferences{Theme: "light", Material: "glass", Persona: "cinematic"}, "auto", "auto", "auto", ErrPreferencesPersona},
		{"三项齐全含强调色", Preferences{Theme: "dark", Material: "solid", Motion: "soft", Contrast: "more", Transparency: "reduce", Accent: "teal"}, "soft", "more", "reduce", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			in := tc.in
			err := NormalizePreferences(&in)
			if err != tc.wantErr {
				t.Fatalf("err = %v，想要 %v", err, tc.wantErr)
			}
			if err != nil {
				return
			}
			if in.Motion != tc.wantMotion || in.Contrast != tc.wantContrast || in.Transparency != tc.wantTransparency {
				t.Errorf("归一结果 = %+v，想要 motion=%q contrast=%q transparency=%q", in, tc.wantMotion, tc.wantContrast, tc.wantTransparency)
			}
		})
	}
}

// 升级路径：2026-10-16 之前落库的那些行里没有这三格。
// 读的一边如果按"缺字段就报错"处理，症状是**这些账号连主题都同步不了**（老数据看起来像坏数据）。
func TestParseLegacyRowWithoutBehaviorAxes(t *testing.T) {
	// 老行里缺的是 motion/contrast/transparency/accent/persona 五格
	p, err := ParsePreferences(`{"theme":"dark","material":"glass","glass_clear":true,"updated_at":5}`)
	if err != nil {
		t.Fatalf("老行应当能读出来，实得 %v", err)
	}
	if p.Motion != "auto" || p.Contrast != "auto" || p.Transparency != "auto" || p.Accent != "auto" || p.Persona != "plain" {
		t.Errorf("缺格应当补 auto（= 交回系统信号），实得 %+v", p)
	}
	if p.Theme != "dark" || !p.GlassClear {
		t.Errorf("已有的三格不能被补齐动作改掉：%+v", p)
	}
}

// 清单跨语言：三份行为档都必须含 auto，且必须排序稳定（判据按集合比，但基线打印要可 diff）。
func TestBehaviorListsIncludeAuto(t *testing.T) {
	for name, list := range map[string][]string{
		"MotionIDs":       ValidMotionIDs(),
		"ContrastIDs":     ValidContrastIDs(),
		"TransparencyIDs": ValidTransparencyIDs(),
	} {
		if !strings.Contains(strings.Join(list, ","), PrefModeAuto) {
			t.Errorf("%s 里没有 auto：%v", name, list)
		}
		for i := 1; i < len(list); i++ {
			if list[i-1] > list[i] {
				t.Fatalf("%s 没排序：%v", name, list)
			}
		}
	}
}
