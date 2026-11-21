package rag

import (
	"testing"
	"time"

	aimodel "github.com/listen808/citebase/internal/model/ai"
)

func TestMemoryDecayWeight(t *testing.T) {
	now := time.Now()

	cases := []struct {
		name string
		mem  *aimodel.AIMemory
		want float64
	}{
		{"刚访问", &aimodel.AIMemory{LastAccessedAt: &now}, 1},
		{"零值时间按满权重", &aimodel.AIMemory{}, 1},
	}
	for _, c := range cases {
		if got := memoryDecayWeight(c.mem); got < c.want-1e-9 || got > c.want+1e-9 {
			t.Fatalf("%s: want ~%v got %v", c.name, c.want, got)
		}
	}

	// 半衰期：30 天未命中 → 权重约 0.5
	thirtyDaysAgo := now.AddDate(0, 0, -30)
	w := memoryDecayWeight(&aimodel.AIMemory{LastAccessedAt: &thirtyDaysAgo})
	if w < 0.49 || w > 0.51 {
		t.Fatalf("30 days idle should decay to ~0.5, got %v", w)
	}

	// 半衰期内按指数曲线：15 天未命中 → 权重约 0.707
	fifteenDaysAgo := now.AddDate(0, 0, -15)
	w15 := memoryDecayWeight(&aimodel.AIMemory{LastAccessedAt: &fifteenDaysAgo})
	if w15 < 0.70 || w15 > 0.71 {
		t.Fatalf("15 days idle should decay to ~0.707, got %v", w15)
	}

	// 保底：任何更久的未命中（30 天 = 半衰期、60 天、一年）都不低于 0.5
	for _, days := range []int{30, 60, 365} {
		ago := now.AddDate(0, 0, -days)
		if w := memoryDecayWeight(&aimodel.AIMemory{LastAccessedAt: &ago}); w < 0.5-1e-9 {
			t.Fatalf("decay floor is 0.5, %d days idle got %v", days, w)
		}
		if w := memoryDecayWeight(&aimodel.AIMemory{LastAccessedAt: &ago}); w > 1+1e-9 {
			t.Fatalf("decay weight must not exceed 1, %d days idle got %v", days, w)
		}
	}
}
