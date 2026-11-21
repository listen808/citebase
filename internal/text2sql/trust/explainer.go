package trust

import (
	"context"
	"encoding/json"
	"fmt"

	aimodel "github.com/listen808/citebase/internal/model/ai"
)

// ExplainLLMCaller 解释器依赖的 LLM 调用函数签名
// 实现者负责将 messages 和 JSON Schema 发给 LLM，并返回原始文本。
type ExplainLLMCaller func(ctx context.Context, systemPrompt string, messages []Message, schema string) (string, error)

// Message 简单的 LLM 消息结构
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// SQLExplainer 生成 SQL 查询解释
type SQLExplainer struct {
	caller ExplainLLMCaller
}

func NewSQLExplainer(caller ExplainLLMCaller) *SQLExplainer {
	return &SQLExplainer{caller: caller}
}

const explanationSchema = `{
  "type": "object",
  "properties": {
    "natural_language": {"type": "string"},
    "field_mapping": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "user_term": {"type": "string"},
          "table": {"type": "string"},
          "field": {"type": "string"}
        },
        "required": ["user_term", "table", "field"]
      }
    },
    "table_mapping": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "user_concept": {"type": "string"},
          "table": {"type": "string"}
        },
        "required": ["user_concept", "table"]
      }
    },
    "confidence": {"type": "number", "minimum": 0, "maximum": 1},
    "reasoning_steps": {"type": "array", "items": {"type": "string"}},
    "caveats": {"type": "array", "items": {"type": "string"}}
  },
  "required": ["natural_language", "confidence"]
}`

// Explain 基于用户问题和 SQL 生成解释
func (e *SQLExplainer) Explain(ctx context.Context, question, sqlStr string) (*aimodel.SQLExplanation, error) {
	if e.caller == nil {
		return nil, fmt.Errorf("llm caller not configured")
	}

	systemPrompt := `你是一位 SQL 解释专家。请基于用户的自然语言问题和生成的 SQL，生成一段结构化的中文解释。解释需要让非技术用户也能理解。`

	userPrompt := fmt.Sprintf("问题：%s\n\n生成的 SQL：%s\n\n请按 JSON Schema 输出解释。", question, sqlStr)

	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	resp, err := e.caller(ctx, "", messages, explanationSchema)
	if err != nil {
		return nil, err
	}

	var explanation aimodel.SQLExplanation
	if err := json.Unmarshal([]byte(resp), &explanation); err != nil {
		return nil, fmt.Errorf("parse explanation json failed: %w", err)
	}

	return &explanation, nil
}

// EmptyExplanation 返回空解释（降级）
func EmptyExplanation() *aimodel.SQLExplanation {
	return &aimodel.SQLExplanation{
		NaturalLanguage: "未生成解释",
		Confidence:      0,
	}
}
