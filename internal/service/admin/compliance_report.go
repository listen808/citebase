package admin

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"sort"
	"strconv"
	"strings"
	"time"

	airepo "github.com/listen808/citebase/internal/repository/ai"
)

// ComplianceReportService 合规报告：基于 AI 审计日志聚合成可审计的合规视图。
type ComplianceReportService struct {
	repo *airepo.AIAuditLogRepository
}

// NewComplianceReportService 创建合规报告服务。
func NewComplianceReportService() *ComplianceReportService {
	return &ComplianceReportService{repo: airepo.NewAIAuditLogRepository()}
}

// ActionStat 单个动作维度的统计。
type ActionStat struct {
	Action       string `json:"action"`
	Count        int64  `json:"count"`
	SuccessCount int64  `json:"success_count"`
	FailCount    int64  `json:"fail_count"`
}

// ComplianceReport 合规报告聚合结果。
type ComplianceReport struct {
	GeneratedAt  time.Time               `json:"generated_at"`
	PeriodDays   int                     `json:"period_days"`
	TotalEvents  int64                   `json:"total_events"`
	SuccessCount int64                   `json:"success_count"`
	FailCount    int64                   `json:"fail_count"`
	SuccessRate  float64                 `json:"success_rate"`
	ByAction     []ActionStat            `json:"by_action"`
	TopUsers     []airepo.UserUsageStat  `json:"top_users"`
	DailyTrend   []airepo.DailyUsageStat `json:"daily_trend"`
}

// GenerateReport 聚合指定天数内的合规数据。
func (s *ComplianceReportService) GenerateReport(ctx context.Context, days int) (*ComplianceReport, error) {
	if days <= 0 {
		days = 30
	}
	rep := &ComplianceReport{
		GeneratedAt: time.Now(),
		PeriodDays:  days,
	}

	// 总量与成功数必须与 PeriodDays 同窗口，否则报告标注 30 天、数字却是全期口径，
	// 旧实现还把 CountByResult("") 当「全部事件」用（实际统计 result='' 的行，恒为 0），
	// 导致 FailCount 为负、SuccessRate 恒为 0。
	total, err := s.repo.CountByResultWithinDays("", days)
	if err != nil {
		return nil, err
	}
	success, err := s.repo.CountByResultWithinDays("success", days)
	if err != nil {
		return nil, err
	}
	rep.TotalEvents = total
	rep.SuccessCount = success
	rep.FailCount = total - success
	if total > 0 {
		rep.SuccessRate = float64(success) / float64(total) * 100
	}

	// 旧实现 FindRecent(100000) 把最多 10 万条审计日志整行拉进内存做分组聚合，
	// 改为 SQL GROUP BY action + 时间窗过滤，一次查询完成。
	actions, err := s.repo.ActionStatsWithinDays(days)
	if err != nil {
		return nil, err
	}
	for _, a := range actions {
		rep.ByAction = append(rep.ByAction, ActionStat{
			Action:       a.Action,
			Count:        a.Count,
			SuccessCount: a.SuccessCount,
			FailCount:    a.FailCount,
		})
	}
	sort.Slice(rep.ByAction, func(i, j int) bool {
		return rep.ByAction[i].Count > rep.ByAction[j].Count
	})

	topUsers, err := s.repo.UsageByUser(days, 10)
	if err != nil {
		return nil, err
	}
	rep.TopUsers = topUsers

	trend, err := s.repo.UsageDailyTrend(days)
	if err != nil {
		return nil, err
	}
	rep.DailyTrend = trend

	return rep, nil
}

// ExportCSV 将报告导出为 CSV 字符串（供审计留档）。
func (s *ComplianceReportService) ExportCSV(rep *ComplianceReport) (string, error) {
	var sb strings.Builder
	w := csv.NewWriter(&sb)

	if err := w.Write([]string{"metric", "value"}); err != nil {
		return "", err
	}
	rows := [][]string{
		{"period_days", strconv.Itoa(rep.PeriodDays)},
		{"total_events", strconv.FormatInt(rep.TotalEvents, 10)},
		{"success_count", strconv.FormatInt(rep.SuccessCount, 10)},
		{"fail_count", strconv.FormatInt(rep.FailCount, 10)},
		{"success_rate", strconv.FormatFloat(rep.SuccessRate, 'f', 2, 64)},
	}
	if err := w.WriteAll(rows); err != nil {
		return "", err
	}

	if err := w.Write([]string{"action", "count", "success", "fail"}); err != nil {
		return "", err
	}
	for _, a := range rep.ByAction {
		if err := w.Write([]string{a.Action, strconv.FormatInt(a.Count, 10),
			strconv.FormatInt(a.SuccessCount, 10), strconv.FormatInt(a.FailCount, 10)}); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return sb.String(), nil
}

// ExportJSON 将报告导出为 JSON 字符串。
func (s *ComplianceReportService) ExportJSON(rep *ComplianceReport) (string, error) {
	b, err := json.MarshalIndent(rep, "", "  ")
	if err != nil {
		return "", err
	}
	return string(b), nil
}
