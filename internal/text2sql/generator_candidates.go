package text2sql

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	modelai "github.com/listen808/citebase/internal/model/ai"
)

// Text2SQL 生成器：多候选 SQL 生成与评估。

// SQLCandidate 单个SQL候选
type SQLCandidate struct {
	SQL          string
	Explanation  string
	ExecSuccess  bool
	ExecError    string
	StructIssues int
	Confidence   float64
}

// GenerateMultipleCandidates 生成多个SQL候选，通过执行验证+结构校验排序选出最优
// 参考: RAT-SQL, BRIDGE 的多候选策略
// 兼容旧签名：无调用者身份时候选执行验证被 fail-closed 拒绝；
// 需要恢复“执行验证选优”的调用方请改用 GenerateMultipleCandidatesContext。
func (g *Generator) GenerateMultipleCandidates(text string, spaceID uint64, numCandidates int) (*modelai.SQLResult, []SQLCandidate, error) {
	return g.GenerateMultipleCandidatesContext(context.Background(), text, spaceID, numCandidates)
}

// GenerateMultipleCandidatesContext P1 首要任务 1：候选验证沿请求 ctx 携带真实调用者身份
func (g *Generator) GenerateMultipleCandidatesContext(ctx context.Context, text string, spaceID uint64, numCandidates int) (*modelai.SQLResult, []SQLCandidate, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	if numCandidates < 2 {
		numCandidates = 2
	}
	if numCandidates > 5 {
		numCandidates = 5
	}

	kb := NewKnowledgeBase(g.repo)
	if err := kb.LoadFromSpace(spaceID); err != nil {
		return nil, nil, fmt.Errorf("加载空间知识库失败: %w", err)
	}
	if len(kb.GetAllTables()) == 0 {
		return nil, nil, fmt.Errorf("知识库中没有表结构")
	}
	if err := kb.LoadRules(); err != nil {
		return nil, nil, fmt.Errorf("加载规则失败: %w", err)
	}

	text = sanitizeQuestionFields(text, kb)
	text = g.applyGlossaryMapping(text, int(spaceID))

	schemaContext, relationsContext := g.retrieveSchemaContext(ctx, text, kb, int(spaceID))

	var schemaTables []string
	for name := range kb.GetAllTables() {
		schemaTables = append(schemaTables, name)
	}

	qType := ClassifyQuery(text)
	fewShotBlock := g.retrieveFewShotExamples(text, int(spaceID), 3)

	var candidates []SQLCandidate

	for i := 0; i < numCandidates; i++ {
		candidate := g.generateSingleCandidate(ctx, text, kb, int(spaceID), schemaContext, relationsContext, schemaTables, fmt.Sprintf("%d", qType), fewShotBlock, i)
		if candidate.SQL != "" {
			candidates = append(candidates, candidate)
		}
	}

	if len(candidates) == 0 {
		return nil, nil, fmt.Errorf("未能生成任何有效的SQL候选")
	}

	// 执行验证 + 排序
	if g.executor != nil && spaceID > 0 {
		for i := range candidates {
			if candidates[i].SQL == "" {
				continue
			}
			// P1 首要任务 1：候选验证沿请求 ctx 携带真实调用者身份（清单 35 同源问题）
			execResult := g.executor.ExecuteWithContext(ctx, candidates[i].SQL, spaceID)
			if execResult.Success {
				candidates[i].ExecSuccess = true
				candidates[i].Confidence += 0.4
			} else {
				candidates[i].ExecError = execResult.Error
			}
		}
	}

	// 结构校验评分
	for i := range candidates {
		if candidates[i].SQL == "" {
			continue
		}
		issues := ValidateSQLStructure(candidates[i].SQL, schemaTables)
		candidates[i].StructIssues = len(issues)
		candidates[i].Confidence += float64(10-len(issues)) * 0.03
		if len(issues) == 0 {
			candidates[i].Confidence += 0.2
		}
	}

	// 排序：执行成功优先，然后按置信度降序
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].ExecSuccess != candidates[j].ExecSuccess {
			return candidates[i].ExecSuccess
		}
		return candidates[i].Confidence > candidates[j].Confidence
	})

	best := candidates[0]
	result := &modelai.SQLResult{
		SQL:          best.SQL,
		Tables:       extractTablesFromSQL(best.SQL, schemaTables),
		Explanations: []string{best.Explanation},
	}

	return result, candidates, nil
}

// generateSingleCandidate 生成单个SQL候选（使用不同的 temperature/seed 变化）
func (g *Generator) generateSingleCandidate(ctx context.Context, text string, kb *KnowledgeBase, spaceID int, schemaContext, relationsContext string, schemaTables []string, qTypeStr string, fewShotBlock string, candidateIdx int) SQLCandidate {
	qType := QueryType(0)
	fmt.Sscanf(qTypeStr, "%d", &qType)
	tpl, ok := g.getPromptTemplateFromDB(qType, spaceID)
	if !ok {
		tpl = GetPromptTemplate(qType)
	}

	prompt := strings.ReplaceAll(strings.ReplaceAll(tpl.UserTemplate, "{question}", text), "{schema_context}", schemaContext)
	prompt = strings.ReplaceAll(prompt, "{relations}", relationsContext)
	if fewShotBlock != "" {
		prompt = fewShotBlock + "\n\n" + prompt
	}

	// 为不同候选添加变化指令
	if candidateIdx > 0 {
		variations := []string{
			"\n\n请尝试使用不同的JOIN策略或子查询方式来实现相同的目标。",
			"\n\n请尝试使用不同的WHERE条件组织方式，确保逻辑等价但表达不同。",
			"\n\n请尝试使用CTE（WITH子句）或不同的GROUP BY策略来实现。",
		}
		prompt += variations[candidateIdx%len(variations)]
	}

	messages := []Message{NewTextMessage("user", prompt)}
	if ctx == nil {
		ctx = context.Background()
	}
	response, err := g.llmProvider.Chat(ctx, tpl.SystemPrompt, messages)
	if err != nil {
		return SQLCandidate{}
	}

	result := g.parseLLMResponse(response.Content, kb)
	if result.SQL == "" {
		return SQLCandidate{}
	}

	processedSQL, _, _ := g.postProcessSQL(ctx, text, result.SQL, schemaContext, relationsContext, kb, schemaTables)

	return SQLCandidate{
		SQL:         processedSQL,
		Explanation: strings.Join(result.Explanations, "; "),
		Confidence:  0.3,
	}
}

// SQLEvalResult SQL评估结果
type SQLEvalResult struct {
	Question        string  `json:"question"`
	GeneratedSQL    string  `json:"generated_sql"`
	ExecutionOK     bool    `json:"execution_ok"`
	ExecError       string  `json:"exec_error,omitempty"`
	StructValid     bool    `json:"struct_valid"`
	StructIssues    int     `json:"struct_issues"`
	TablesCorrect   bool    `json:"tables_correct"`
	ExecutionTimeMs int64   `json:"execution_time_ms"`
	Score           float64 `json:"score"`
}

// EvaluateSQL 评估生成的SQL质量
// 兼容旧签名；需要执行验证生效时调用 EvaluateSQLContext 传入带身份的请求 ctx。
func (g *Generator) EvaluateSQL(question, sqlStr string, spaceID uint64) *SQLEvalResult {
	return g.EvaluateSQLContext(context.Background(), question, sqlStr, spaceID)
}

// EvaluateSQLContext P1 首要任务 1：执行验证沿 ctx 携带真实调用者身份
func (g *Generator) EvaluateSQLContext(ctx context.Context, question, sqlStr string, spaceID uint64) *SQLEvalResult {
	if ctx == nil {
		ctx = context.Background()
	}
	result := &SQLEvalResult{
		Question:     question,
		GeneratedSQL: sqlStr,
	}

	// 1. 结构校验
	kb := NewKnowledgeBase(g.repo)
	if err := kb.LoadFromSpace(spaceID); err == nil {
		var schemaTables []string
		for name := range kb.GetAllTables() {
			schemaTables = append(schemaTables, name)
		}
		issues := ValidateSQLStructure(sqlStr, schemaTables)
		result.StructIssues = len(issues)
		result.StructValid = len(issues) == 0

		result.TablesCorrect = true
		extracted := extractTablesFromSQL(sqlStr, schemaTables)
		if len(extracted) == 0 {
			result.TablesCorrect = false
		}
	}

	// 2. 执行验证
	if g.executor != nil && spaceID > 0 {
		start := time.Now()
		execResult := g.executor.ExecuteWithContext(ctx, sqlStr, spaceID)
		result.ExecutionTimeMs = time.Since(start).Milliseconds()
		result.ExecutionOK = execResult.Success
		if !execResult.Success {
			result.ExecError = execResult.Error
		}
	}

	// 3. 综合评分
	score := 0.0
	if result.StructValid {
		score += 0.3
	}
	if result.TablesCorrect {
		score += 0.2
	}
	if result.ExecutionOK {
		score += 0.4
		if result.ExecutionTimeMs < 1000 {
			score += 0.1
		}
	}
	result.Score = score

	return result
}

// classifyErrorType 分类 SQL 执行错误类型
func classifyErrorType(errMsg string) string {
	if errMsg == "" {
		return ""
	}

	errLower := strings.ToLower(errMsg)

	// 语法错误
	if strings.Contains(errLower, "syntax error") ||
		strings.Contains(errLower, "unexpected") {
		return "syntax_error"
	}

	// 字段不存在
	if strings.Contains(errLower, "unknown column") ||
		strings.Contains(errLower, "doesn't exist") {
		return "unknown_column"
	}

	// 表不存在
	if strings.Contains(errLower, "table") &&
		(strings.Contains(errLower, "doesn't exist") ||
			strings.Contains(errLower, "not found")) {
		return "unknown_table"
	}

	// 歧义字段
	if strings.Contains(errLower, "ambiguous") {
		return "ambiguous_column"
	}

	// 权限错误
	if strings.Contains(errLower, "access denied") ||
		strings.Contains(errLower, "permission") {
		return "permission_error"
	}

	// 连接错误
	if strings.Contains(errLower, "connection") ||
		strings.Contains(errLower, "timeout") {
		return "connection_error"
	}

	return "execution_error"
}
