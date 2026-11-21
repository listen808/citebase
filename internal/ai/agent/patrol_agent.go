package agent

import (
	"context"
	"fmt"
	"time"

	"github.com/listen808/citebase/internal/brand"
)

// PatrolAgent Always-On 巡检 Agent
type PatrolAgent struct {
	retrieverID int
	provider    LLMProvider
	delivery    DeliveryService
}

// LLMProvider LLM 提供者接口（用于巡检分析）
type LLMProvider interface {
	Chat(ctx context.Context, prompt string) (string, error)
}

// DeliveryService 交付服务接口
type DeliveryService interface {
	SendEmail(ctx context.Context, to []string, subject, body string) error
	SendWebhook(ctx context.Context, url, payload string) error
}

// PatrolReport 巡检报告
type PatrolReport struct {
	Date            time.Time `json:"date"`
	OutdatedDocs    []DocInfo `json:"outdated_docs"`
	LowQualityDocs  []DocInfo `json:"low_quality_docs"`
	KnowledgeGaps   []GapInfo `json:"knowledge_gaps"`
	Summary         string    `json:"summary"`
	Recommendations []string  `json:"recommendations"`
}

// DocInfo 文档信息
type DocInfo struct {
	ID     int    `json:"id"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

// GapInfo 知识缺口信息
type GapInfo struct {
	Type    string `json:"type"`
	SpaceID int    `json:"space_id"`
	Detail  string `json:"detail"`
}

// NewPatrolAgent 创建巡检 Agent
func NewPatrolAgent(provider LLMProvider, delivery DeliveryService) *PatrolAgent {
	return &PatrolAgent{provider: provider, delivery: delivery}
}

// RunPatrol 执行巡检
func (p *PatrolAgent) RunPatrol(ctx context.Context, checks PatrolChecks) (*PatrolReport, error) {
	report := &PatrolReport{Date: time.Now()}

	// 1. 检测过时文档
	report.OutdatedDocs = checks.FindOutdatedDocs()

	// 2. 检测质量问题
	report.LowQualityDocs = checks.FindLowQualityDocs()

	// 3. 检测知识缺口
	report.KnowledgeGaps = checks.FindKnowledgeGaps()

	// 4. 生成摘要和推荐
	if p.provider != nil {
		summary, err := p.generateSummary(ctx, report)
		if err == nil {
			report.Summary = summary
		}
		report.Recommendations = p.generateRecommendations(report)
	}

	return report, nil
}

// PatrolChecks 巡检检查接口
type PatrolChecks interface {
	FindOutdatedDocs() []DocInfo
	FindLowQualityDocs() []DocInfo
	FindKnowledgeGaps() []GapInfo
}

// generateSummary 生成巡检摘要
func (p *PatrolAgent) generateSummary(ctx context.Context, report *PatrolReport) (string, error) {
	prompt := fmt.Sprintf(`生成知识库巡检报告摘要。

巡检日期：%s
过时文档数：%d
低质量文档数：%d
知识缺口数：%d

请生成一段 200 字以内的中文摘要，概括巡检结果和建议行动。`,
		report.Date.Format("2006-01-02"),
		len(report.OutdatedDocs),
		len(report.LowQualityDocs),
		len(report.KnowledgeGaps))

	return p.provider.Chat(ctx, prompt)
}

// generateRecommendations 生成推荐
func (p *PatrolAgent) generateRecommendations(report *PatrolReport) []string {
	var recs []string

	if len(report.OutdatedDocs) > 0 {
		recs = append(recs, fmt.Sprintf("发现 %d 个过时文档，建议更新或归档", len(report.OutdatedDocs)))
	}
	if len(report.LowQualityDocs) > 0 {
		recs = append(recs, fmt.Sprintf("发现 %d 个低质量文档，建议重新分析或重写", len(report.LowQualityDocs)))
	}
	if len(report.KnowledgeGaps) > 0 {
		recs = append(recs, fmt.Sprintf("发现 %d 个知识缺口，建议补充相关文档", len(report.KnowledgeGaps)))
	}
	if len(recs) == 0 {
		recs = append(recs, "知识库状态良好，无需紧急操作")
	}
	return recs
}

// SendReport 发送简报
func (p *PatrolAgent) SendReport(ctx context.Context, report *PatrolReport, recipients []string) error {
	if p.delivery == nil {
		return nil
	}

	body := fmt.Sprintf(`
知识库巡检报告 - %s

摘要：
%s

问题统计：
- 过时文档：%d 个
- 低质量文档：%d 个
- 知识缺口：%d 个

建议行动：
%s
`,
		report.Date.Format("2006-01-02 15:04:05"),
		report.Summary,
		len(report.OutdatedDocs),
		len(report.LowQualityDocs),
		len(report.KnowledgeGaps),
		joinRecommendations(report.Recommendations))

	subject := fmt.Sprintf("[%s] 知识库巡检报告 %s", brand.SiteName(), report.Date.Format("2006-01-02"))
	return p.delivery.SendEmail(ctx, recipients, subject, body)
}

// joinRecommendations 拼接推荐列表
func joinRecommendations(recs []string) string {
	result := ""
	for i, r := range recs {
		result += fmt.Sprintf("%d. %s\n", i+1, r)
	}
	return result
}
