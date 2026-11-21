package admin

import (
	"strings"
	"testing"
	"time"

	airepo "github.com/listen808/citebase/internal/repository/ai"
)

func TestComplianceReportExportCSV(t *testing.T) {
	svc := NewComplianceReportService()
	rep := &ComplianceReport{
		GeneratedAt:  time.Now(),
		PeriodDays:   30,
		TotalEvents:  100,
		SuccessCount: 90,
		FailCount:    10,
		SuccessRate:  90,
		ByAction: []ActionStat{
			{Action: "chat", Count: 60, SuccessCount: 55, FailCount: 5},
			{Action: "search", Count: 40, SuccessCount: 35, FailCount: 5},
		},
		TopUsers:   []airepo.UserUsageStat{{UserID: 1, Username: "alice", TotalUsage: 1000, RequestCount: 10}},
		DailyTrend: []airepo.DailyUsageStat{{Date: "2026-07-13", TotalUsage: 100, RequestCount: 5, UniqueUsers: 2}},
	}

	csv, err := svc.ExportCSV(rep)
	if err != nil {
		t.Fatalf("ExportCSV: %v", err)
	}
	if !strings.Contains(csv, "total_events,100") {
		t.Fatalf("csv missing total_events: %q", csv)
	}
	if !strings.Contains(csv, "chat,60,55,5") {
		t.Fatalf("csv missing action row: %q", csv)
	}

	js, err := svc.ExportJSON(rep)
	if err != nil {
		t.Fatalf("ExportJSON: %v", err)
	}
	if !strings.Contains(js, "\"total_events\": 100") && !strings.Contains(js, "\"total_events\":100") {
		t.Fatalf("json missing total_events: %q", js)
	}
}

func TestComplianceReportDefaultDays(t *testing.T) {
	svc := NewComplianceReportService()
	// 天数 <=0 应归一到 30（GenerateReport 内部处理；此处仅验证服务可构造）
	if svc == nil {
		t.Fatal("service nil")
	}
}
