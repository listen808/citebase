package rag

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
)

type ReportGenerator struct {
	provider provider.Provider
}

func NewReportGenerator(p provider.Provider) *ReportGenerator {
	return &ReportGenerator{provider: p}
}

type ReportType string

const (
	ReportTypeWeekly  ReportType = "weekly"
	ReportTypeMonthly ReportType = "monthly"
	ReportTypeTech    ReportType = "tech"
	ReportTypeMeeting ReportType = "meeting"
	ReportTypeProject ReportType = "project"
	ReportTypeCustom  ReportType = "custom"
)

type ReportRequest struct {
	Type     ReportType `json:"type"`
	Title    string     `json:"title"`
	Sources  []string   `json:"sources"`
	Extra    string     `json:"extra"`
	Sections []string   `json:"sections"`
	Language string     `json:"language"`
}

type ReportResult struct {
	Title    string          `json:"title"`
	Content  string          `json:"content"`
	Sections []ReportSection `json:"sections"`
}

type ReportSection struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func (g *ReportGenerator) Generate(ctx context.Context, req ReportRequest) (*ReportResult, provider.TokenUsage, error) {
	if g.provider == nil {
		return nil, provider.TokenUsage{}, fmt.Errorf("AI服务未启用")
	}

	if req.Language == "" {
		req.Language = "zh"
	}

	sectionsGuide := g.getSections(req.Type, req.Sections)
	combinedSources := strings.Join(req.Sources, "\n\n---\n\n")
	if len(combinedSources) > 10000 {
		combinedSources = combinedSources[:10000] + "..."
	}

	systemPrompt := fmt.Sprintf(`你是一个专业的文档汇报生成助手。请根据提供的素材生成结构化的汇报文档。

汇报类型：%s
要求：
1. 使用 Markdown 格式输出
2. 语言：%s
3. 结构清晰，层次分明
4. 如有数据支撑，用表格或列表展示
5. 重点突出，避免冗余
6. 保留关键的数字和指标

输出章节结构：
%s

请直接输出完整的 Markdown 文档。`, g.getTypeName(req.Type), req.Language, sectionsGuide)

	extraPart := ""
	if req.Extra != "" {
		extraPart = fmt.Sprintf("\n\n额外要求：%s", req.Extra)
	}

	userPrompt := fmt.Sprintf(`标题：%s

素材内容：
%s%s`, req.Title, combinedSources, extraPart)

	timeoutCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	resp, err := g.provider.Chat(timeoutCtx, systemPrompt, []provider.Message{
		provider.NewTextMessage("user", userPrompt),
	}, provider.WithMaxTokens(8192), provider.WithTemperature(0.5))
	if err != nil {
		return nil, provider.TokenUsage{}, err
	}

	result := &ReportResult{
		Title:   req.Title,
		Content: resp.Content,
	}
	result.Sections = parseReportSections(resp.Content)

	return result, resp.TokenUsage, nil
}

func (g *ReportGenerator) getSections(reportType ReportType, customSections []string) string {
	switch reportType {
	case ReportTypeWeekly:
		return `- 本周概要（概述本周工作重点）
- 完成事项（列出已完成的任务和成果）
- 进行中（当前正在推进的工作）
- 下周计划（下周的工作安排）
- 风险与阻塞（存在的问题和风险）`
	case ReportTypeMonthly:
		return `- 月度概览（整体工作总结）
- 重点项目进展（各项目的推进情况）
- 关键指标（核心数据指标变化）
- 团队协作（协作情况和亮点）
- 问题与改进（需要改进的地方）
- 下月规划（下个月的重点方向）`
	case ReportTypeTech:
		return `- 背景与目标（技术选型的背景）
- 技术方案（详细的技术设计）
- 对比分析（方案间的优劣对比）
- 实施计划（落地执行方案）
- 风险评估（技术风险和应对）`
	case ReportTypeMeeting:
		return `- 会议信息（时间、地点、参会人）
- 议题列表（讨论的主要议题）
- 讨论要点（各议题的核心讨论内容）
- 决议事项（达成的决定）
- 行动项（后续待办和负责人）`
	case ReportTypeProject:
		return `- 项目概述（项目背景和目标）
- 当前状态（项目整体进展）
- 里程碑完成情况（各里程碑的完成度）
- 资源使用（人力和物力投入）
- 风险与问题（当前面临的风险）
- 下一步计划（后续工作安排）`
	default:
		if len(customSections) > 0 {
			return "- " + strings.Join(customSections, "\n- ")
		}
		return `- 概述
- 主要内容
- 总结与建议`
	}
}

func (g *ReportGenerator) getTypeName(reportType ReportType) string {
	names := map[ReportType]string{
		ReportTypeWeekly:  "周报",
		ReportTypeMonthly: "月报",
		ReportTypeTech:    "技术报告",
		ReportTypeMeeting: "会议纪要",
		ReportTypeProject: "项目汇报",
		ReportTypeCustom:  "自定义汇报",
	}
	if name, ok := names[reportType]; ok {
		return name
	}
	return "汇报文档"
}

func parseReportSections(content string) []ReportSection {
	var sections []ReportSection
	lines := strings.Split(content, "\n")
	var current *ReportSection

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "## ") {
			if current != nil {
				sections = append(sections, *current)
			}
			current = &ReportSection{
				Title: strings.TrimPrefix(trimmed, "## "),
			}
		} else if strings.HasPrefix(trimmed, "# ") && !strings.HasPrefix(trimmed, "## ") {
			if current != nil {
				sections = append(sections, *current)
			}
			current = &ReportSection{
				Title: strings.TrimPrefix(trimmed, "# "),
			}
		} else if current != nil {
			current.Content += line + "\n"
		}
	}
	if current != nil {
		sections = append(sections, *current)
	}
	return sections
}
