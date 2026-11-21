package text2sql

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/listen808/citebase/internal/ai/provider"
)

// Text2SQL 生成器：LLM Provider 抽象、调用选项、消息与响应类型、结构化输出 schema、全局注入。

// LLMProvider 定义 LLM 调用接口
type LLMProvider interface {
	Chat(ctx context.Context, systemPrompt string, messages []Message) (*ChatResponse, error)
	// ChatWithOptions 支持结构化输出选项（JSON Schema、Function Calling等）
	ChatWithOptions(ctx context.Context, systemPrompt string, messages []Message, opts ...CallOption) (*ChatResponse, error)
}

// CallOption LLM 调用选项
type CallOption func(*CallOptions)

// ProviderTier 模型档位
type ProviderTier string

const (
	TierPrimary   ProviderTier = "primary"
	TierFast      ProviderTier = "fast"
	TierReasoning ProviderTier = "reasoning"
)

// CallOptions 调用选项集合
type CallOptions struct {
	ResponseFormat *ResponseFormat
	Temperature    float64
	MaxTokens      int
	Tier           ProviderTier // 指定模型档位，空值表示由调用方决定
}

// WithTier 指定 LLM 档位
func WithTier(tier ProviderTier) CallOption {
	return func(o *CallOptions) {
		o.Tier = tier
	}
}

// ResponseFormat 响应格式配置
type ResponseFormat struct {
	Type   string                 `json:"type"` // "text" or "json_object"
	Schema map[string]interface{} `json:"schema,omitempty"`
}

// WithJSONSchema 设置 JSON Schema 约束输出
func WithJSONSchema(schema map[string]interface{}) CallOption {
	return func(o *CallOptions) {
		o.ResponseFormat = &ResponseFormat{
			Type:   "json_object",
			Schema: schema,
		}
	}
}

// WithTemperature 设置温度参数
func WithTemperature(t float64) CallOption {
	return func(o *CallOptions) {
		o.Temperature = t
	}
}

// WithMaxTokens 设置最大 token 数
func WithMaxTokens(n int) CallOption {
	return func(o *CallOptions) {
		o.MaxTokens = n
	}
}

// SQLGenerationResponse LLM 结构化输出响应
type SQLGenerationResponse struct {
	SQL          string   `json:"sql"`
	Explanation  string   `json:"explanation"`
	Confidence   float64  `json:"confidence"`
	Tables       []string `json:"tables"`
	FieldMapping []struct {
		UserTerm  string `json:"user_term"`
		DDLField  string `json:"ddl_field"`
		TableName string `json:"table_name"`
	} `json:"field_mapping"`
}

// tryParseStructuredJSON 尝试将 LLM 响应解析为结构化 JSON
// P1-4: 当启用 WithJSONSchema 时，LLM 返回 JSON 格式，需要优先尝试 JSON 解析
// 成功返回解析结果，失败返回 ok=false（调用方应走正则回退路径）
func tryParseStructuredJSON(content string) (*SQLGenerationResponse, bool) {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil, false
	}

	// 必须看起来像 JSON 才尝试解析（以 { 开头）
	if !strings.HasPrefix(content, "{") {
		return nil, false
	}

	// 尝试直接解析
	var resp SQLGenerationResponse
	if err := json.Unmarshal([]byte(content), &resp); err == nil && resp.SQL != "" {
		return &resp, true
	}

	// 尝试提取 ```json ... ``` 包裹的内容
	if idx := strings.Index(content, "```json"); idx >= 0 {
		start := idx + len("```json")
		end := strings.Index(content[start:], "```")
		if end > 0 {
			jsonContent := strings.TrimSpace(content[start : start+end])
			if err := json.Unmarshal([]byte(jsonContent), &resp); err == nil && resp.SQL != "" {
				return &resp, true
			}
		}
	}

	return nil, false
}

// GetSQLGenerationJSONSchema 返回 SQL 生成的 JSON Schema
func GetSQLGenerationJSONSchema() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"sql": map[string]interface{}{
				"type":        "string",
				"description": "生成的 SQL 查询语句",
			},
			"explanation": map[string]interface{}{
				"type":        "string",
				"description": "SQL 生成逻辑的自然语言解释",
			},
			"confidence": map[string]interface{}{
				"type":        "number",
				"description": "生成结果的置信度 (0.0-1.0)",
				"minimum":     0,
				"maximum":     1,
			},
			"tables": map[string]interface{}{
				"type":        "array",
				"description": "SQL 中涉及的表名列表",
				"items": map[string]interface{}{
					"type": "string",
				},
			},
			"field_mapping": map[string]interface{}{
				"type":        "array",
				"description": "用户术语到 DDL 字段的映射",
				"items": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"user_term": map[string]interface{}{
							"type":        "string",
							"description": "用户提到的术语",
						},
						"ddl_field": map[string]interface{}{
							"type":        "string",
							"description": "对应的 DDL 字段名",
						},
						"table_name": map[string]interface{}{
							"type":        "string",
							"description": "字段所属的表名",
						},
					},
					"required": []string{"user_term", "ddl_field", "table_name"},
				},
			},
		},
		"required": []string{"sql", "explanation", "confidence", "tables"},
	}
}

// Message 聊天消息
type Message struct {
	Role    string
	Content string
}

// ChatResponse 聊天响应
type ChatResponse struct {
	Content string
}

// NewTextMessage 创建文本消息
func NewTextMessage(role, content string) Message {
	return Message{Role: role, Content: content}
}

// RAGRetriever 定义RAG检索接口，由外部注入实现
type RAGRetriever interface {
	RetrieveContextWithRAG(question string, spaceID int, limit int) ([]RAGSource, string)
}

type RAGSource struct {
	DocumentID   int
	DocumentName string
	SpaceID      int
	Snippet      string
}

var globalRAGRetriever RAGRetriever

func SetRAGRetriever(r RAGRetriever) {
	globalRAGRetriever = r
}

// globalLLMProvider 全局 LLM provider
var globalLLMProvider LLMProvider

// SetGlobalLLMProvider 设置全局 LLM provider
func SetGlobalLLMProvider(p LLMProvider) {
	globalLLMProvider = p
}

// P0-11: globalRealProvider 全局真实 provider（用于 Agent 流式输出）
var globalRealProvider provider.Provider

// SetGlobalRealProvider 设置全局真实 provider（支持 StreamChatWithTools）
func SetGlobalRealProvider(p provider.Provider) {
	globalRealProvider = p
}

// P2-5: globalModelRouter 全局模型路由器适配器
var globalModelRouter ModelRouterAdapter

// SetModelRouterAdapter 设置全局模型路由器适配器
func SetModelRouterAdapter(adapter ModelRouterAdapter) {
	globalModelRouter = adapter
}
