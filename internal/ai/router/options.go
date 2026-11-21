package router

import (
	"errors"

	"github.com/listen808/citebase/internal/ai/provider"
)

var (
	errNoProvider    = errors.New("router: no available provider")
	errEmptyResponse = errors.New("router: provider returned empty response")
)

// Option 路由器级别的调用参数（透传给底层 provider）。
type Option func(*provider.CallOptions)

// WithMaxTokens 设置最大生成 token。
func WithMaxTokens(n int) Option {
	return func(c *provider.CallOptions) { c.MaxTokens = n }
}

// WithTemperature 设置采样温度。
func WithTemperature(t float64) Option {
	return func(c *provider.CallOptions) { c.Temperature = t }
}

// WithModel 强制指定底层模型名。
func WithModel(m string) Option {
	return func(c *provider.CallOptions) { c.Model = m }
}

// toProviderOpts 将路由器 Option 转换为 provider Option 列表。
func toProviderOpts(opts ...Option) []provider.Option {
	callOpts := &provider.CallOptions{}
	for _, o := range opts {
		o(callOpts)
	}
	out := make([]provider.Option, 0, 3)
	if callOpts.MaxTokens > 0 {
		out = append(out, provider.WithMaxTokens(callOpts.MaxTokens))
	}
	if callOpts.Temperature != 0 {
		out = append(out, provider.WithTemperature(callOpts.Temperature))
	}
	if callOpts.Model != "" {
		out = append(out, provider.WithModel(callOpts.Model))
	}
	return out
}
