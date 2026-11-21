package text2sql

import (
	"context"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"regexp"
	"strings"
)

// LLMRefiner LLM SQL 修正器
// P1-8: 当 AST 规则修正失败时，用 LLM 修正复杂 SQL 错误
// 替代仅 AST 规则修正，复杂错误修正率 +25%
type LLMRefiner struct {
	provider LLMProvider
}

// NewLLMRefiner 创建 LLM 修正器
func NewLLMRefiner(p LLMProvider) *LLMRefiner {
	return &LLMRefiner{provider: p}
}

// Refine 用 LLM 修正 SQL
// sql: 原始 SQL
// question: 用户问题
// schemaContext: Schema 上下文
// execError: 执行错误信息
// 返回修正后的 SQL（如果修正失败返回空字符串）
func (r *LLMRefiner) Refine(ctx context.Context, sql string, question string, schemaContext string, execError string) (string, error) {
	if r == nil || r.provider == nil {
		return "", fmt.Errorf("LLM Refiner 未启用")
	}

	prompt := fmt.Sprintf(`## 任务
请修正以下 SQL 语句的错误。

## 用户问题
%s

## 原始 SQL
%s

## Schema 上下文
%s

## 执行错误
%s

## 修正要求
1. 只修正错误，不要改变查询意图
2. 常见错误类型：字段名错误、表名错误、JOIN 条件错误、类型不匹配、语法错误
3. 仔细对照 Schema 上下文中的字段名和表名
4. 只返回修正后的 SQL，不要输出其他内容
5. 用 sql 代码块（三个反引号包裹）输出`, question, sql, truncateContext(schemaContext, 3000), execError)

	resp, err := r.provider.Chat(ctx, "你是一个 SQL 修正专家，请根据错误信息修正 SQL。", []Message{
		NewTextMessage("user", prompt),
	})
	if err != nil {
		return "", fmt.Errorf("LLM 修正调用失败: %w", err)
	}

	refinedSQL := extractSQLFromResponse(resp.Content)
	if refinedSQL == "" {
		logx.Infof("[LLMRefiner] Failed to extract SQL from response: %s", resp.Content)
		return "", fmt.Errorf("无法从 LLM 响应中提取 SQL")
	}

	if strings.EqualFold(strings.TrimSpace(refinedSQL), strings.TrimSpace(sql)) {
		logx.Infof("[LLMRefiner] Refined SQL is same as original, no change")
		return "", nil
	}

	logx.Infof("[LLMRefiner] SQL refined: original_len=%d refined_len=%d", len(sql), len(refinedSQL))
	return refinedSQL, nil
}

// extractSQLFromResponse 从 LLM 响应中提取 SQL
func extractSQLFromResponse(content string) string {
	// 优先匹配 ```sql ... ```
	sqlRegex := regexp.MustCompile("(?s)```sql\\s*(.*?)\\s*```")
	if m := sqlRegex.FindStringSubmatch(content); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// 回退：匹配 SELECT ... 到分号
	selectRegex := regexp.MustCompile(`(?is)(SELECT\s+.+;)`)
	if m := selectRegex.FindStringSubmatch(content); len(m) > 1 {
		return strings.TrimSpace(m[1])
	}
	// 回退：直接返回内容（去除前后空白）
	return strings.TrimSpace(content)
}

// truncateContext 截断上下文到指定长度
func truncateContext(ctx string, maxLen int) string {
	if len(ctx) <= maxLen {
		return ctx
	}
	return ctx[:maxLen] + "\n...(已截断)"
}
