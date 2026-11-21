package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	aimod "github.com/listen808/citebase/internal/ai"
	provider "github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/log"
)

// IntentService 意图识别业务逻辑层
type IntentService struct{}

// NewIntentService 创建 IntentService 实例
func NewIntentService() *IntentService {
	return &IntentService{}
}

// DetectIntentRequest 意图识别请求
type DetectIntentRequest struct {
	Question string `json:"question" binding:"required"`
}

// DetectIntentResponse 意图识别响应
type DetectIntentResponse struct {
	Intent     string   `json:"intent"`
	Confidence float64  `json:"confidence"`
	Hints      []string `json:"hints"`
}

// DetectIntent 识别用户输入意图
func (s *IntentService) DetectIntent(ctx context.Context, module *aimod.Module, req *DetectIntentRequest) *DetectIntentResponse {
	if !module.Enabled() {
		intent := ruleDetectIntent(req.Question)
		return &DetectIntentResponse{Intent: intent, Confidence: 0.6, Hints: []string{"AI 未启用，使用规则识别"}}
	}

	// 先走规则，高置信度直接返回
	intent, confidence, hints := ruleDetectIntentWithConfidence(req.Question)
	if confidence >= 0.9 {
		return &DetectIntentResponse{Intent: intent, Confidence: confidence, Hints: hints}
	}

	// 规则置信度不足时，用 fast provider 做 LLM 确认
	fastProvider := module.FastProvider()
	if fastProvider == nil {
		return &DetectIntentResponse{Intent: intent, Confidence: confidence, Hints: append(hints, "fast provider 不可用")}
	}

	llmIntent, llmConfidence, llmHints, err := llmDetectIntent(ctx, fastProvider, req.Question)
	if err != nil {
		log.Errorf("[IntentService] llm detect failed: %v", err)
		return &DetectIntentResponse{Intent: intent, Confidence: confidence, Hints: append(hints, "LLM 识别失败，使用规则结果")}
	}

	// LLM 置信度较高时采用 LLM 结果，否则回退规则
	if llmConfidence >= 0.7 {
		return &DetectIntentResponse{Intent: llmIntent, Confidence: llmConfidence, Hints: llmHints}
	}
	return &DetectIntentResponse{Intent: intent, Confidence: confidence, Hints: append(hints, llmHints...)}
}

func ruleDetectIntent(question string) string {
	intent, _, _ := ruleDetectIntentWithConfidence(question)
	return intent
}

func ruleDetectIntentWithConfidence(question string) (string, float64, []string) {
	q := strings.ToLower(strings.TrimSpace(question))

	// 强制前缀模式
	if strings.HasPrefix(q, "#sql") || strings.HasPrefix(q, "#数据") {
		return "data_query", 1.0, []string{"命中强制前缀 #sql/#数据"}
	}
	if strings.HasPrefix(q, "#agent") || strings.HasPrefix(q, "#代理") {
		return "agent", 1.0, []string{"命中强制前缀 #agent/#代理"}
	}
	if strings.HasPrefix(q, "#research") || strings.HasPrefix(q, "#研究") {
		return "research", 1.0, []string{"命中强制前缀 #research/#研究"}
	}
	if strings.HasPrefix(q, "#wiki") || strings.HasPrefix(q, "#wiki查询") {
		return "wiki_query", 1.0, []string{"命中强制前缀 #wiki"}
	}
	if strings.HasPrefix(q, "#chat") || strings.HasPrefix(q, "#问答") {
		return "chat", 1.0, []string{"命中强制前缀 #chat/#问答"}
	}

	// 数据查询关键词
	dataKeywords := []string{"查询", "统计", "多少", "总数", "平均", "最大", "最小", "sql", "数据库", "表", "字段", "列", "where", "select", "group by", "order by", "趋势", "同比", "环比"}
	for _, kw := range dataKeywords {
		if strings.Contains(q, kw) {
			return "data_query", 0.85, []string{"命中数据查询关键词: " + kw}
		}
	}

	// Agent 任务关键词
	agentKeywords := []string{"执行", "调用", "发送邮件", "创建", "删除", "更新", "同步", "导出", "导入", "分析文档", "扫描"}
	for _, kw := range agentKeywords {
		if strings.Contains(q, kw) {
			return "agent", 0.75, []string{"命中 Agent 任务关键词: " + kw}
		}
	}

	// Wiki 知识库查询关键词
	wikiKeywords := []string{"wiki", "知识库", "词条", "概念页", "知识页面", "wiki 里", "wiki中"}
	for _, kw := range wikiKeywords {
		if strings.Contains(q, kw) {
			return "wiki_query", 0.80, []string{"命中 Wiki 查询关键词: " + kw}
		}
	}

	// 深度研究关键词
	researchKeywords := []string{"研究", "调研", "综述", "报告", "总结", "深度分析", "全面分析", "compare", "对比"}
	for _, kw := range researchKeywords {
		if strings.Contains(q, kw) {
			return "research", 0.75, []string{"命中研究类关键词: " + kw}
		}
	}

	// 默认聊天
	return "chat", 0.6, []string{"未命中特定关键词，默认走知识问答"}
}

func llmDetectIntent(ctx context.Context, p provider.Provider, question string) (string, float64, []string, error) {
	system := `你是一个意图识别助手。请判断用户输入应该路由到哪个功能模块。
可选意图：chat（知识库问答）、data_query（数据/SQL查询）、agent（执行操作任务）、research（深度研究/报告）、wiki_query（Wiki 知识库页面查询）。
wiki_query 用于用户明确想查询已编译的 Wiki 知识页面（概念页/实体页/源摘要页），而非原始文档。
只输出 JSON：{"intent": "chat|data_query|agent|research|wiki_query", "confidence": 0.0-1.0, "reason": "简短原因"}`
	messages := []provider.Message{
		provider.NewTextMessage("system", system),
		provider.NewTextMessage("user", question),
	}

	resp, err := p.Chat(ctx, "", messages, provider.WithTemperature(0.0), provider.WithMaxTokens(256))
	if err != nil {
		return "", 0, nil, err
	}

	var result struct {
		Intent     string  `json:"intent"`
		Confidence float64 `json:"confidence"`
		Reason     string  `json:"reason"`
	}
	content := strings.TrimSpace(resp.Content)
	// 如果模型输出包裹在 markdown code block 中，剥离
	if strings.HasPrefix(content, "```") {
		content = strings.Trim(content, "`")
		content = strings.TrimPrefix(content, "json")
		content = strings.TrimSpace(content)
	}
	if err := json.Unmarshal([]byte(content), &result); err != nil {
		return "", 0, nil, fmt.Errorf("parse intent json failed: %w", err)
	}

	validIntents := map[string]bool{"chat": true, "data_query": true, "agent": true, "research": true, "wiki_query": true}
	if !validIntents[result.Intent] {
		result.Intent = "chat"
	}
	if result.Confidence < 0 || result.Confidence > 1 {
		result.Confidence = 0.5
	}

	hints := []string{}
	if result.Reason != "" {
		hints = append(hints, result.Reason)
	}
	return result.Intent, result.Confidence, hints, nil
}
