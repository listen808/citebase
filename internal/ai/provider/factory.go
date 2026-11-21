package provider

import (
	"fmt"
	"strings"
)

type ProviderConfig struct {
	Name        string
	BaseURL     string
	APIKey      string
	Model       string
	MaxTokens   int
	Temperature float64
}

func NewProvider(cfg *ProviderConfig) (Provider, error) {
	switch strings.ToLower(cfg.Name) {
	case "openai", "deepseek", "qwen", "zhipu", "ernie", "gemini", "ollama":
		return NewOpenAIProvider(cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.MaxTokens, cfg.Temperature), nil
	case "anthropic":
		return NewAnthropicProvider(cfg.BaseURL, cfg.APIKey, cfg.Model, cfg.MaxTokens, cfg.Temperature), nil
	default:
		return nil, fmt.Errorf("unsupported provider: %s (supported: openai, deepseek, qwen, zhipu, ernie, gemini, ollama, anthropic)", cfg.Name)
	}
}

func NewProviderFromConfig(name, baseURL, apiKey, model string, maxTokens int, temperature float64) (Provider, error) {
	cfg := &ProviderConfig{
		Name:        name,
		BaseURL:     baseURL,
		APIKey:      apiKey,
		Model:       model,
		MaxTokens:   maxTokens,
		Temperature: temperature,
	}
	p, err := NewProvider(cfg)
	if err != nil {
		return nil, fmt.Errorf("create provider %s: %w", name, err)
	}
	return p, nil
}
