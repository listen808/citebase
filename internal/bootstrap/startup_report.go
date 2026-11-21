package bootstrap

import (
	"fmt"
	"strings"

	"github.com/listen808/citebase/internal/brand"
)

type ModuleStatus string

const (
	StatusOK      ModuleStatus = "OK"
	StatusWarning ModuleStatus = "WARN"
	StatusFailed  ModuleStatus = "FAIL"
	StatusSkipped ModuleStatus = "SKIP"
)

type ModuleInfo struct {
	Name   string
	Status ModuleStatus
	Detail string
}

type StartupReport struct {
	modules []ModuleInfo
}

func NewStartupReport() *StartupReport {
	return &StartupReport{
		modules: make([]ModuleInfo, 0, 10),
	}
}

func (r *StartupReport) Add(name string, status ModuleStatus, detail string) {
	r.modules = append(r.modules, ModuleInfo{
		Name:   name,
		Status: status,
		Detail: detail,
	})
}

func (r *StartupReport) AddOK(name string) {
	r.Add(name, StatusOK, "")
}

func (r *StartupReport) AddWarn(name, detail string) {
	r.Add(name, StatusWarning, detail)
}

func (r *StartupReport) AddFail(name, detail string) {
	r.Add(name, StatusFailed, detail)
}

func (r *StartupReport) AddSkip(name, detail string) {
	r.Add(name, StatusSkipped, detail)
}

func (r *StartupReport) Format() string {
	var sb strings.Builder
	maxNameLen := 0
	for _, m := range r.modules {
		if len(m.Name) > maxNameLen {
			maxNameLen = len(m.Name)
		}
	}

	sb.WriteString("\n")
	sb.WriteString("=============================================\n")
	sb.WriteString("  " + brand.DefaultName + " 启动状态汇总\n")
	sb.WriteString("=============================================\n")

	okCount := 0
	warnCount := 0
	failCount := 0
	skipCount := 0

	for _, m := range r.modules {
		statusIcon := r.statusIcon(m.Status)
		paddedName := m.Name + strings.Repeat(" ", maxNameLen-len(m.Name))
		line := fmt.Sprintf("  %s  %s  %s", statusIcon, paddedName, m.Status)
		if m.Detail != "" {
			line += "  (" + m.Detail + ")"
		}
		sb.WriteString(line + "\n")

		switch m.Status {
		case StatusOK:
			okCount++
		case StatusWarning:
			warnCount++
		case StatusFailed:
			failCount++
		case StatusSkipped:
			skipCount++
		}
	}

	sb.WriteString("=============================================\n")
	summary := fmt.Sprintf("  总计: %d  正常: %d  警告: %d  失败: %d  跳过: %d",
		len(r.modules), okCount, warnCount, failCount, skipCount)
	sb.WriteString(summary + "\n")
	sb.WriteString("=============================================\n")

	if warnCount > 0 {
		sb.WriteString("  ⚠ 部分模块启动异常，相关功能可能不可用\n")
	}
	if failCount > 0 {
		sb.WriteString("  ✖ 有模块启动失败，请检查日志\n")
	}
	sb.WriteString("\n")

	return sb.String()
}

func (r *StartupReport) statusIcon(s ModuleStatus) string {
	switch s {
	case StatusOK:
		return "✔"
	case StatusWarning:
		return "⚠"
	case StatusFailed:
		return "✖"
	case StatusSkipped:
		return "○"
	default:
		return " "
	}
}

func (r *StartupReport) HasFailures() bool {
	for _, m := range r.modules {
		if m.Status == StatusFailed {
			return true
		}
	}
	return false
}

func (r *StartupReport) HasWarnings() bool {
	for _, m := range r.modules {
		if m.Status == StatusWarning {
			return true
		}
	}
	return false
}
