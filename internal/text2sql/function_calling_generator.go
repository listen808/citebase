package text2sql

import (
	"context"
	"encoding/json"
	"fmt"
)

// GetSQLGenerationToolDefinition 返回 SQL 生成的 Tool Definition（用于 Function Calling）
// 注意：此函数需要外部 provider.Tools 支持，当前为占位实现
func GetSQLGenerationToolDefinition() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"sql": map[string]interface{}{
				"type":        "string",
				"description": "生成的SQL查询语句",
			},
			"explanation": map[string]interface{}{
				"type":        "string",
				"description": "SQL语句的简要解释",
			},
			"confidence": map[string]interface{}{
				"type":        "number",
				"description": "置信度",
			},
		},
		"required": []string{"sql", "explanation", "confidence"},
	}
}

// FunctionCallingGenerator 封装使用 Function Calling 的 SQL 生成器
type FunctionCallingGenerator struct {
	generator *Generator
}

// NewFunctionCallingGenerator 创建 Function Calling 增强生成器
func NewFunctionCallingGenerator(g *Generator) *FunctionCallingGenerator {
	return &FunctionCallingGenerator{generator: g}
}

// GenerateWithFunctionCalling 使用 Function Calling 生成 SQL
// 相比 JSON Schema，Function Calling 的解析成功率更高
func (fcg *FunctionCallingGenerator) GenerateWithFunctionCalling(
	ctx context.Context,
	question string,
	schemaContext string,
	relationsContext string,
) (*SQLGenerationResponse, error) {
	// 构建消息
	systemPrompt := fcg.buildSystemPrompt(schemaContext, relationsContext)
	messages := []Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: question},
	}

	// 使用结构化输出
	schema := GetSQLGenerationJSONSchema()
	llmResp, err := fcg.generator.llmProvider.ChatWithOptions(ctx, systemPrompt, messages,
		WithJSONSchemaOption(schema))
	if err != nil {
		return nil, fmt.Errorf("ChatWithOptions failed: %w", err)
	}

	// 解析响应
	return fcg.parseResponse(llmResp.Content)
}

// buildSystemPrompt 构建系统提示词
func (fcg *FunctionCallingGenerator) buildSystemPrompt(schemaContext, relationsContext string) string {
	return fmt.Sprintf("你是SQL生成专家。请根据用户问题生成SQL查询语句。\n\n## 数据库 Schema\n%s\n\n## 表关系\n%s\n\n## 输出要求\n请严格按JSON格式输出，包含sql、explanation、confidence字段。", schemaContext, relationsContext)
}

// parseResponse 解析LLM响应
func (fcg *FunctionCallingGenerator) parseResponse(content string) (*SQLGenerationResponse, error) {
	// 尝试解析JSON
	var resp SQLGenerationResponse
	if err := json.Unmarshal([]byte(content), &resp); err != nil {
		// JSON解析失败，尝试提取SQL
		sql := extractSQLFromText(content)
		if sql == "" {
			return nil, fmt.Errorf("failed to parse response")
		}
		return &SQLGenerationResponse{
			SQL:         sql,
			Explanation: "Generated via fallback",
			Confidence:  0.7,
		}, nil
	}
	return &resp, nil
}

// WithJSONSchemaOption 创建JSON Schema选项
func WithJSONSchemaOption(schema map[string]interface{}) CallOption {
	return func(o *CallOptions) {
		o.ResponseFormat = &ResponseFormat{
			Type:   "json_object",
			Schema: schema,
		}
	}
}
