package ai

import (
	"strings"
)

// ValidateAIConfig 启动时检查 AI 配置，返回告警列表
// 注意：仅检查 ai.AIConfig 中的字段（provider/embedding/mcp），
// vector_store_type 等字段在 config.AIConfig 中，由调用方在 main.go 中检查。
func ValidateAIConfig(cfg *AIConfig) []string {
	var warnings []string

	// 1. primary 配置检查
	if cfg.Primary != nil {
		if cfg.Primary.Provider == "" {
			warnings = append(warnings, "primary.provider is empty")
		}
		if cfg.Primary.APIKey == "" {
			warnings = append(warnings, "primary.api_key is empty, primary provider will not be created")
		}
		if cfg.Primary.BaseURL == "" {
			warnings = append(warnings, "primary.base_url is empty")
		}
		if cfg.Primary.Model == "" {
			warnings = append(warnings, "primary.model is empty")
		}
	} else if cfg.Legacy == nil {
		warnings = append(warnings, "primary section is not configured and no legacy config found")
	}

	// 2. embedding 配置检查
	if cfg.Embedding != nil {
		if cfg.Embedding.Provider == "" && cfg.Embedding.BaseURL == "" && cfg.Embedding.APIKey == "" {
			warnings = append(warnings, "embedding section exists but all fields are empty, vector search will be disabled")
		}
		// anthropic 不支持 embedding
		if strings.ToLower(cfg.Embedding.Provider) == "anthropic" {
			warnings = append(warnings, "embedding.provider is anthropic, which does not support embeddings; use openai-compatible provider instead")
		}
	}

	// 3. 如果 primary 未配置但 fast/reasoning 配置了，它们会回退到 primary（即 nil）
	if cfg.Primary == nil && cfg.Legacy == nil {
		if cfg.Fast != nil {
			warnings = append(warnings, "fast provider configured but primary is not, fast will fall back to nil")
		}
		if cfg.Reasoning != nil {
			warnings = append(warnings, "reasoning provider configured but primary is not, reasoning will fall back to nil")
		}
	}

	return warnings
}
