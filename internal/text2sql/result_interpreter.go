package text2sql

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/listen808/citebase/internal/ai/provider"
)

// ResultInterpreter 查询结果智能解读器
type ResultInterpreter struct {
	provider provider.Provider
}

// NewResultInterpreter 创建结果解读器
func NewResultInterpreter(p provider.Provider) *ResultInterpreter {
	return &ResultInterpreter{provider: p}
}

// Interpret 对查询结果进行自然语言解读
func (r *ResultInterpreter) Interpret(ctx context.Context, question, sql string, execResult *ExecResult) (string, error) {
	if r.provider == nil {
		return r.fallbackInterpret(execResult), nil
	}

	// 构建结果摘要
	summary := r.buildResultSummary(execResult)

	systemPrompt := `你是一个数据分析助手。请根据用户的查询问题和SQL执行结果，生成简洁的自然语言解读。

要求：
1. 用中文输出
2. 先总结数据量（如"共查到 X 条记录"）
3. 然后指出关键发现（如最大值、最小值、趋势、异常值等）
4. 如果有分组聚合，解读各组数据的特点
5. 控制在 3-5 句话以内
6. 不要重复 SQL 语句`

	userPrompt := fmt.Sprintf("用户问题：%s\n\nSQL：%s\n\n执行结果：%s\n\n请对以上结果进行解读。", question, sql, summary)

	messages := []provider.Message{
		provider.NewTextMessage("user", userPrompt),
	}

	resp, err := r.provider.Chat(ctx, systemPrompt, messages)
	if err != nil {
		return r.fallbackInterpret(execResult), nil
	}

	return strings.TrimSpace(resp.Content), nil
}

// buildResultSummary 构建结果摘要
func (r *ResultInterpreter) buildResultSummary(execResult *ExecResult) string {
	if execResult == nil {
		return "无结果"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("共 %d 条记录\n", execResult.RowCount))
	sb.WriteString(fmt.Sprintf("列: %s\n", strings.Join(execResult.Columns, ", ")))

	if len(execResult.Rows) > 0 {
		// 显示前5行
		maxRows := 5
		if len(execResult.Rows) < maxRows {
			maxRows = len(execResult.Rows)
		}
		sb.WriteString(fmt.Sprintf("数据预览（前%d行）:\n", maxRows))
		for i := 0; i < maxRows; i++ {
			b, _ := json.Marshal(execResult.Rows[i])
			sb.WriteString(fmt.Sprintf("  %s\n", string(b)))
		}
		if len(execResult.Rows) > maxRows {
			sb.WriteString(fmt.Sprintf("  ... 还有 %d 行\n", len(execResult.Rows)-maxRows))
		}
	}

	return sb.String()
}

// fallbackInterpret 降级解读（不依赖LLM）
func (r *ResultInterpreter) fallbackInterpret(execResult *ExecResult) string {
	if execResult == nil || execResult.RowCount == 0 {
		return "查询未返回任何数据"
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("共查询到 %d 条记录。", execResult.RowCount))

	if len(execResult.Columns) > 0 {
		sb.WriteString(fmt.Sprintf("包含字段：%s。", strings.Join(execResult.Columns, "、")))
	}

	if execResult.RowCount == 1 && len(execResult.Rows) > 0 {
		// 单行结果，展示更多细节
		var details []string
		for k, v := range execResult.Rows[0] {
			details = append(details, fmt.Sprintf("%s=%v", k, v))
		}
		sb.WriteString(fmt.Sprintf(" 详情：%s", strings.Join(details, "，")))
	}

	return sb.String()
}
