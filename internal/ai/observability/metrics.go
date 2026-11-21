package observability

import "time"

type MetricType string

const (
	MetricCounter   MetricType = "counter"
	MetricGauge     MetricType = "gauge"
	MetricHistogram MetricType = "histogram"
)

type MetricValue struct {
	Name      string            `json:"name"`
	Type      MetricType        `json:"type"`
	Value     float64           `json:"value"`
	Labels    map[string]string `json:"labels,omitempty"`
	UpdatedAt time.Time         `json:"updated_at"`
}

type HistogramData struct {
	Count   int       `json:"count"`
	Sum     float64   `json:"sum"`
	Min     float64   `json:"min"`
	Max     float64   `json:"max"`
	Buckets []float64 `json:"buckets,omitempty"`
}

type MetricsProvider interface {
	Inc(name string, labels map[string]string, value float64)
	SetGauge(name string, labels map[string]string, value float64)
	AddGauge(name string, labels map[string]string, value float64)
	ObserveHistogram(name string, labels map[string]string, value float64)
	GetMetric(name string, labels map[string]string) (*MetricValue, bool)
	GetHistogram(name string, labels map[string]string) (*HistogramData, bool)
	ListMetrics(prefix string) []*MetricValue
}

const (
	MetricAIRequestsTotal   = "ai_requests_total"
	MetricAIRequestDuration = "ai_request_duration_ms"
	MetricAITokenUsageTotal = "ai_token_usage_total" //nolint:gosec // G101: metric name, not a credential
	MetricAICacheHitRatio   = "ai_cache_hit_ratio"
	MetricAIActiveRequests  = "ai_active_requests"
)
