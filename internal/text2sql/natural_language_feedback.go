package text2sql

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

// FeedbackType 反馈类型
type FeedbackType string

const (
	FeedbackModify  FeedbackType = "modify"  // 修改现有SQL
	FeedbackCorrect FeedbackType = "correct" // 修正错误
	FeedbackRefine  FeedbackType = "refine"  // 优化结果
	FeedbackExpand  FeedbackType = "expand"  // 扩展查询
	FeedbackReduce  FeedbackType = "reduce"  // 缩减查询
)

// FeedbackResult 反馈处理结果
type FeedbackResult struct {
	Accepted       bool     `json:"accepted"`        // 是否采纳
	NewSQL         string   `json:"new_sql"`         // 新的SQL
	Explanation    string   `json:"explanation"`     // 解释
	Confidence     float64  `json:"confidence"`      // 置信度
	ChangesApplied []string `json:"changes_applied"` // 应用的变更
}

// NaturalLanguageFeedback 自然语言反馈处理器
// P1增强：支持用户自然语言反馈，自动修正SQL
type NaturalLanguageFeedback struct {
	provider  provider.Provider
	validator *SQLSelfValidator
}

// NewNaturalLanguageFeedback 创建反馈处理器
func NewNaturalLanguageFeedback(p provider.Provider) *NaturalLanguageFeedback {
	return &NaturalLanguageFeedback{
		provider:  p,
		validator: NewSQLSelfValidator(p),
	}
}

// ProcessFeedback 处理自然语言反馈
func (f *NaturalLanguageFeedback) ProcessFeedback(ctx context.Context, currentSQL, question string, execResult *ExecResult, feedback string) (*FeedbackResult, error) {
	// 1. 分类反馈类型
	feedbackType := f.classifyFeedback(feedback)

	// 2. 根据类型处理
	switch feedbackType {
	case FeedbackCorrect:
		return f.handleCorrection(ctx, currentSQL, question, execResult, feedback)
	case FeedbackModify:
		return f.handleModification(ctx, currentSQL, question, feedback)
	case FeedbackRefine:
		return f.handleRefinement(ctx, currentSQL, question, execResult, feedback)
	case FeedbackExpand:
		return f.handleExpansion(ctx, currentSQL, question, feedback)
	case FeedbackReduce:
		return f.handleReduction(ctx, currentSQL, question, feedback)
	default:
		return f.handleGeneralFeedback(ctx, currentSQL, question, feedback)
	}
}

// classifyFeedback 分类反馈类型
func (f *NaturalLanguageFeedback) classifyFeedback(feedback string) FeedbackType {
	feedbackLower := strings.ToLower(feedback)

	// 修正错误
	correctKeywords := []string{"错了", "不对", "错误", "wrong", "incorrect", "error", "修正", "修复"}
	for _, kw := range correctKeywords {
		if strings.Contains(feedbackLower, kw) {
			return FeedbackCorrect
		}
	}

	// 修改/调整
	modifyKeywords := []string{"改成", "改为", "change", "modify", "adjust"}
	for _, kw := range modifyKeywords {
		if strings.Contains(feedbackLower, kw) {
			return FeedbackModify
		}
	}

	// 优化
	refineKeywords := []string{"优化", "更好", "改进", "improve", "better", "optimize"}
	for _, kw := range refineKeywords {
		if strings.Contains(feedbackLower, kw) {
			return FeedbackRefine
		}
	}

	// 扩展
	expandKeywords := []string{"加上", "增加", "扩展", "包含", "add", "include", "expand"}
	for _, kw := range expandKeywords {
		if strings.Contains(feedbackLower, kw) {
			return FeedbackExpand
		}
	}

	// 缩减
	reduceKeywords := []string{"去掉", "删除", "不要", "减少", "remove", "exclude", "delete", "don't"}
	for _, kw := range reduceKeywords {
		if strings.Contains(feedbackLower, kw) {
			return FeedbackReduce
		}
	}

	return FeedbackModify // 默认按修改处理
}

// handleCorrection 处理错误修正
func (f *NaturalLanguageFeedback) handleCorrection(ctx context.Context, currentSQL, question string, execResult *ExecResult, feedback string) (*FeedbackResult, error) {
	var errorInfo string
	if execResult != nil && execResult.Error != "" {
		errorInfo = fmt.Sprintf("执行错误: %s", execResult.Error)
	} else {
		errorInfo = "结果不符合用户预期"
	}

	prompt := "用户指出当前的SQL存在问题，需要修正。\n\n" +
		"## 原始问题\n" + question + "\n\n" +
		"## 当前SQL\n" + currentSQL + "\n\n" +
		"## 执行结果\n" + errorInfo + "\n\n" +
		"## 用户反馈\n" + feedback + "\n\n" +
		"## 任务\n1. 分析用户反馈，找出SQL中的问题\n2. 生成修正后的SQL\n3. 解释修正的原因\n\n" +
		"## 输出格式\n" +
		`{"new_sql": "修正后的SQL", "explanation": "修正原因说明", "changes": ["变更1"], "confidence": 0.0到1.0的数字}`

	resp, err := f.provider.Chat(ctx, "你是一个SQL修正专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(500), provider.WithTemperature(0.1))

	if err != nil {
		return nil, err
	}

	return f.parseFeedbackResult(resp.Content, currentSQL)
}

// handleModification 处理修改请求
func (f *NaturalLanguageFeedback) handleModification(ctx context.Context, currentSQL, question string, feedback string) (*FeedbackResult, error) {
	prompt := "用户要求修改当前的SQL。\n\n" +
		"## 原始问题\n" + question + "\n\n" +
		"## 当前SQL\n" + currentSQL + "\n\n" +
		"## 用户修改要求\n" + feedback + "\n\n" +
		"## 任务\n1. 理解用户的修改要求\n2. 按照要求修改SQL\n3. 确保修改后的SQL仍然正确\n\n" +
		"## 输出格式\n" +
		`{"new_sql": "修改后的SQL", "explanation": "修改说明", "changes": ["变更1"], "confidence": 0.0到1.0的数字}`

	resp, err := f.provider.Chat(ctx, "你是一个SQL修改专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(500), provider.WithTemperature(0.1))

	if err != nil {
		return nil, err
	}

	return f.parseFeedbackResult(resp.Content, currentSQL)
}

// handleRefinement 处理优化请求
func (f *NaturalLanguageFeedback) handleRefinement(ctx context.Context, currentSQL, question string, execResult *ExecResult, feedback string) (*FeedbackResult, error) {
	var resultSummary string
	if execResult != nil && execResult.RowCount > 0 {
		resultSummary = fmt.Sprintf("当前返回 %d 行结果", execResult.RowCount)
	} else {
		resultSummary = "当前结果"
	}

	prompt := "用户要求优化当前的SQL或结果。\n\n" +
		"## 原始问题\n" + question + "\n\n" +
		"## 当前SQL\n" + currentSQL + "\n\n" +
		"## 当前结果\n" + resultSummary + "\n\n" +
		"## 优化要求\n" + feedback + "\n\n" +
		"## 任务\n1. 分析优化需求\n2. 提供优化建议或修改SQL\n3. 说明优化效果\n\n" +
		"## 输出格式\n" +
		`{"new_sql": "优化后的SQL（如需修改）", "explanation": "优化说明", "improvements": ["改进点1"], "confidence": 0.0到1.0的数字}`

	resp, err := f.provider.Chat(ctx, "你是一个SQL优化专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(500), provider.WithTemperature(0.1))

	if err != nil {
		return nil, err
	}

	return f.parseFeedbackResult(resp.Content, currentSQL)
}

// handleExpansion 处理扩展请求
func (f *NaturalLanguageFeedback) handleExpansion(ctx context.Context, currentSQL, question string, feedback string) (*FeedbackResult, error) {
	prompt := "用户要求扩展当前的SQL查询。\n\n" +
		"## 原始问题\n" + question + "\n\n" +
		"## 当前SQL\n" + currentSQL + "\n\n" +
		"## 扩展要求\n" + feedback + "\n\n" +
		"## 任务\n1. 理解扩展需求\n2. 在原有基础上添加新内容\n3. 保持SQL的正确性\n\n" +
		"## 输出格式\n" +
		`{"new_sql": "扩展后的SQL", "explanation": "扩展说明", "additions": ["新增内容1"], "confidence": 0.0到1.0的数字}`

	resp, err := f.provider.Chat(ctx, "你是一个SQL扩展专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(500), provider.WithTemperature(0.1))

	if err != nil {
		return nil, err
	}

	return f.parseFeedbackResult(resp.Content, currentSQL)
}

// handleReduction 处理缩减请求
func (f *NaturalLanguageFeedback) handleReduction(ctx context.Context, currentSQL, question string, feedback string) (*FeedbackResult, error) {
	prompt := "用户要求缩减当前的SQL查询。\n\n" +
		"## 原始问题\n" + question + "\n\n" +
		"## 当前SQL\n" + currentSQL + "\n\n" +
		"## 缩减要求\n" + feedback + "\n\n" +
		"## 任务\n1. 理解缩减需求\n2. 移除不需要的内容\n3. 保持SQL的正确性\n\n" +
		"## 输出格式\n" +
		`{"new_sql": "缩减后的SQL", "explanation": "缩减说明", "removals": ["移除内容1"], "confidence": 0.0到1.0的数字}`

	resp, err := f.provider.Chat(ctx, "你是一个SQL缩减专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(500), provider.WithTemperature(0.1))

	if err != nil {
		return nil, err
	}

	return f.parseFeedbackResult(resp.Content, currentSQL)
}

// handleGeneralFeedback 处理通用反馈
func (f *NaturalLanguageFeedback) handleGeneralFeedback(ctx context.Context, currentSQL, question string, feedback string) (*FeedbackResult, error) {
	prompt := "用户对当前的SQL给出了反馈。\n\n" +
		"## 原始问题\n" + question + "\n\n" +
		"## 当前SQL\n" + currentSQL + "\n\n" +
		"## 用户反馈\n" + feedback + "\n\n" +
		"## 任务\n1. 理解用户反馈\n2. 如果需要修改SQL，生成修改后的版本\n3. 给出解释\n\n" +
		"## 输出格式\n" +
		`{"new_sql": "修改后的SQL（如无需修改则与原SQL相同）", "explanation": "说明", "changes": ["变更列表"], "confidence": 0.0到1.0的数字}`

	resp, err := f.provider.Chat(ctx, "你是一个SQL修改专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(500), provider.WithTemperature(0.1))

	if err != nil {
		return nil, err
	}

	return f.parseFeedbackResult(resp.Content, currentSQL)
}

// parseFeedbackResult 解析反馈结果
func (f *NaturalLanguageFeedback) parseFeedbackResult(content, originalSQL string) (*FeedbackResult, error) {
	result := &FeedbackResult{
		Accepted:       true,
		NewSQL:         originalSQL,
		Confidence:     0.8,
		ChangesApplied: []string{},
	}

	// 提取JSON
	jsonStr := utils.ExtractJSON(content)
	if jsonStr == "" {
		logx.Infof("[NaturalLanguageFeedback] Failed to extract JSON from response")
		return result, nil
	}

	var parsed struct {
		NewSQL      string   `json:"new_sql"`
		Explanation string   `json:"explanation"`
		Changes     []string `json:"changes"`
		Confidence  float64  `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		logx.Infof("[NaturalLanguageFeedback] Parse feedback result failed: %v", err)
		return result, nil
	}

	if parsed.NewSQL != "" {
		result.NewSQL = parsed.NewSQL
		// 验证新SQL
		if !f.validator.QuickCheck(result.NewSQL) {
			result.NewSQL = originalSQL
			result.Accepted = false
			logx.Infof("[NaturalLanguageFeedback] Generated SQL failed validation, keeping original")
		}
	}

	if parsed.Explanation != "" {
		result.Explanation = parsed.Explanation
	}

	if len(parsed.Changes) > 0 {
		result.ChangesApplied = parsed.Changes
	}

	if parsed.Confidence > 0 {
		result.Confidence = parsed.Confidence
	}

	return result, nil
}

// DetectResultAnomaly 检测结果异常
// P1增强：自动检测执行结果的异常
func (f *NaturalLanguageFeedback) DetectResultAnomaly(execResult *ExecResult, question string) (bool, string) {
	if execResult == nil {
		return false, ""
	}

	// 错误检测
	if execResult.Error != "" {
		return true, fmt.Sprintf("SQL执行错误: %s", execResult.Error)
	}

	// 空结果检测
	if execResult.RowCount == 0 {
		// 检查是否可能有数据
		qLower := strings.ToLower(question)
		expectDataKeywords := []string{"统计", "查询", "查找", "获取", "所有"}
		for _, kw := range expectDataKeywords {
			if strings.Contains(qLower, kw) {
				return true, "查询返回空结果，可能过滤条件过于严格或数据不存在"
			}
		}
	}

	// 异常数量检测
	if execResult.RowCount > 10000 {
		return true, fmt.Sprintf("结果数量过大（%d行），可能需要添加更多过滤条件", execResult.RowCount)
	}

	return false, ""
}

// SuggestCorrections 建议修正
// 根据检测到的异常，建议用户如何反馈
func (f *NaturalLanguageFeedback) SuggestCorrections(anomaly, question string) []string {
	suggestions := []string{}

	switch {
	case strings.Contains(anomaly, "执行错误"):
		suggestions = append(suggestions, "请描述具体的错误表现")
		suggestions = append(suggestions, "检查SQL语法是否正确")

	case strings.Contains(anomaly, "空结果"):
		suggestions = append(suggestions, "放宽过滤条件试试？比如去掉某些WHERE条件")
		suggestions = append(suggestions, "是否需要使用LEFT JOIN而不是INNER JOIN？")

	case strings.Contains(anomaly, "结果数量过大"):
		suggestions = append(suggestions, "可以添加日期范围限制，如只查最近一个月")
		suggestions = append(suggestions, "可以添加分类过滤，如只查某个部门")
	}

	return suggestions
}

// FeedbackHistory 反馈历史
type FeedbackHistory struct {
	SQL      string          `json:"sql"`
	Feedback string          `json:"feedback"`
	Type     FeedbackType    `json:"type"`
	Result   *FeedbackResult `json:"result"`
}

// ConversationFeedbackManager 对话反馈管理器
// 管理多轮对话中的反馈历史
type ConversationFeedbackManager struct {
	history []FeedbackHistory
	maxSize int
}

// NewConversationFeedbackManager 创建反馈管理器
func NewConversationFeedbackManager() *ConversationFeedbackManager {
	return &ConversationFeedbackManager{
		history: make([]FeedbackHistory, 0),
		maxSize: 10,
	}
}

// Add 添加反馈记录
func (m *ConversationFeedbackManager) Add(sql, feedback string, feedbackType FeedbackType, result *FeedbackResult) {
	m.history = append(m.history, FeedbackHistory{
		SQL:      sql,
		Feedback: feedback,
		Type:     feedbackType,
		Result:   result,
	})

	// 限制历史大小
	if len(m.history) > m.maxSize {
		m.history = m.history[1:]
	}
}

// GetLastSQL 获取上一条SQL
func (m *ConversationFeedbackManager) GetLastSQL() string {
	if len(m.history) == 0 {
		return ""
	}
	return m.history[len(m.history)-1].SQL
}

// GetContext 获取反馈上下文
func (m *ConversationFeedbackManager) GetContext() string {
	if len(m.history) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("\n## 历史修改记录\n")

	for i, h := range m.history {
		sb.WriteString(fmt.Sprintf("%d. 原SQL: %s\n", i+1, h.SQL))
		sb.WriteString(fmt.Sprintf("   反馈: %s\n", h.Feedback))
		if h.Result != nil && h.Result.NewSQL != h.SQL {
			sb.WriteString(fmt.Sprintf("   修正: %s\n", h.Result.NewSQL))
		}
	}

	return sb.String()
}

// Clear 清空历史
func (m *ConversationFeedbackManager) Clear() {
	m.history = make([]FeedbackHistory, 0)
}
