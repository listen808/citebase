package bootstrap

import (
	"strings"
	"testing"
)

func TestNewStartupReport(t *testing.T) {
	r := NewStartupReport()
	if r == nil {
		t.Fatal("expected non-nil StartupReport")
	}
	if len(r.modules) != 0 {
		t.Errorf("expected 0 modules, got %d", len(r.modules))
	}
}

func TestStartupReport_AddOK(t *testing.T) {
	r := NewStartupReport()
	r.AddOK("database")
	if len(r.modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(r.modules))
	}
	if r.modules[0].Name != "database" {
		t.Errorf("expected name 'database', got %s", r.modules[0].Name)
	}
	if r.modules[0].Status != StatusOK {
		t.Errorf("expected StatusOK, got %s", r.modules[0].Status)
	}
}

func TestStartupReport_AddWarn(t *testing.T) {
	r := NewStartupReport()
	r.AddWarn("search", "index not found")
	if r.modules[0].Status != StatusWarning {
		t.Errorf("expected StatusWarning, got %s", r.modules[0].Status)
	}
	if r.modules[0].Detail != "index not found" {
		t.Errorf("expected detail 'index not found', got %s", r.modules[0].Detail)
	}
}

func TestStartupReport_AddFail(t *testing.T) {
	r := NewStartupReport()
	r.AddFail("ai", "api key missing")
	if r.modules[0].Status != StatusFailed {
		t.Errorf("expected StatusFailed, got %s", r.modules[0].Status)
	}
}

func TestStartupReport_AddSkip(t *testing.T) {
	r := NewStartupReport()
	r.AddSkip("redis", "not configured")
	if r.modules[0].Status != StatusSkipped {
		t.Errorf("expected StatusSkipped, got %s", r.modules[0].Status)
	}
}

func TestStartupReport_HasFailures(t *testing.T) {
	r := NewStartupReport()
	if r.HasFailures() {
		t.Error("expected HasFailures=false for empty report")
	}

	r.AddOK("db")
	r.AddWarn("search", "slow")
	if r.HasFailures() {
		t.Error("expected HasFailures=false with only OK and WARN")
	}

	r.AddFail("ai", "error")
	if !r.HasFailures() {
		t.Error("expected HasFailures=true with FAIL")
	}
}

func TestStartupReport_HasWarnings(t *testing.T) {
	r := NewStartupReport()
	if r.HasWarnings() {
		t.Error("expected HasWarnings=false for empty report")
	}

	r.AddOK("db")
	if r.HasWarnings() {
		t.Error("expected HasWarnings=false with only OK")
	}

	r.AddWarn("search", "slow")
	if !r.HasWarnings() {
		t.Error("expected HasWarnings=true with WARN")
	}
}

func TestStartupReport_Format(t *testing.T) {
	r := NewStartupReport()
	r.AddOK("Database")
	r.AddWarn("Search Index", "rebuilding in background")
	r.AddSkip("Redis", "not enabled")
	r.AddOK("AI Module")

	output := r.Format()

	// 验证包含标题
	if !strings.Contains(output, "启动状态汇总") {
		t.Error("expected output to contain '启动状态汇总'")
	}

	// 验证包含各模块
	if !strings.Contains(output, "Database") {
		t.Error("expected output to contain 'Database'")
	}
	if !strings.Contains(output, "Search Index") {
		t.Error("expected output to contain 'Search Index'")
	}

	// 验证统计
	if !strings.Contains(output, "正常: 2") {
		t.Errorf("expected '正常: 2' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "警告: 1") {
		t.Errorf("expected '警告: 1' in output, got:\n%s", output)
	}
	if !strings.Contains(output, "跳过: 1") {
		t.Errorf("expected '跳过: 1' in output, got:\n%s", output)
	}
}

func TestStartupReport_Format_AllOK(t *testing.T) {
	r := NewStartupReport()
	r.AddOK("db")
	r.AddOK("cache")
	r.AddOK("search")

	output := r.Format()
	if strings.Contains(output, "部分模块启动异常") {
		t.Error("should not show warning when all OK")
	}
	if strings.Contains(output, "模块启动失败") {
		t.Error("should not show failure when all OK")
	}
}

func TestStartupReport_Format_WithFailure(t *testing.T) {
	r := NewStartupReport()
	r.AddFail("db", "connection refused")

	output := r.Format()
	if !strings.Contains(output, "有模块启动失败") {
		t.Error("should show failure message when there are failures")
	}
}

func TestStartupReport_statusIcon(t *testing.T) {
	r := NewStartupReport()
	tests := []struct {
		status   ModuleStatus
		expected string
	}{
		{StatusOK, "✔"},
		{StatusWarning, "⚠"},
		{StatusFailed, "✖"},
		{StatusSkipped, "○"},
		{ModuleStatus("unknown"), " "},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			result := r.statusIcon(tt.status)
			if result != tt.expected {
				t.Errorf("statusIcon(%s) = %q, want %q", tt.status, result, tt.expected)
			}
		})
	}
}

func TestModuleStatus_Values(t *testing.T) {
	if StatusOK != "OK" {
		t.Errorf("StatusOK = %q, want 'OK'", StatusOK)
	}
	if StatusWarning != "WARN" {
		t.Errorf("StatusWarning = %q, want 'WARN'", StatusWarning)
	}
	if StatusFailed != "FAIL" {
		t.Errorf("StatusFailed = %q, want 'FAIL'", StatusFailed)
	}
	if StatusSkipped != "SKIP" {
		t.Errorf("StatusSkipped = %q, want 'SKIP'", StatusSkipped)
	}
}
