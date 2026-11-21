package text2sql

import (
	"fmt"
	"io"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// P2-2 (§4.3.3): Metrics 暴露
//
// 目标：暴露 Prometheus 文本格式的 Text2SQL 指标，支持监控。
//
// 设计原则（Karpathy: simplicity first）：
//   - 不引入 prometheus/client_golang 依赖（项目 go.mod 无该库）
//   - 自实现轻量级 Counter / Histogram，输出 Prometheus text exposition format
//   - 全局单例 MetricsRegistry，线程安全
//   - 指标定义与文档 §4.3.3 对齐

// ============================================================
// 指标类型
// ============================================================

// Counter 单调递增计数器
type Counter struct {
	name   string
	help   string
	labels []string
	mu     sync.Mutex
	values map[string]float64 // key = labels 序列化后的字符串
}

// Histogram 直方图（固定桶）
type Histogram struct {
	name         string
	help         string
	labels       []string
	buckets      []float64
	mu           sync.Mutex
	sums         map[string]float64
	counts       map[string]uint64
	bucketCounts map[string]map[float64]uint64 // key → bucket → count
}

// MetricsRegistry 指标注册中心（全局单例）
type MetricsRegistry struct {
	mu         sync.RWMutex
	counters   map[string]*Counter
	histograms map[string]*Histogram
}

var globalMetrics = &MetricsRegistry{
	counters:   make(map[string]*Counter),
	histograms: make(map[string]*Histogram),
}

// GetMetricsRegistry 获取全局指标注册中心
func GetMetricsRegistry() *MetricsRegistry {
	return globalMetrics
}

// ============================================================
// Counter 方法
// ============================================================

// NewCounter 创建计数器（若已存在则返回已有实例）
func (r *MetricsRegistry) NewCounter(name, help string, labels ...string) *Counter {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.counters[name]; ok {
		return c
	}
	c := &Counter{
		name:   name,
		help:   help,
		labels: labels,
		values: make(map[string]float64),
	}
	r.counters[name] = c
	return c
}

// Inc 计数器 +1
func (c *Counter) Inc(labelValues ...string) {
	c.Add(1, labelValues...)
}

// Add 计数器 +n
func (c *Counter) Add(n float64, labelValues ...string) {
	key := serializeLabels(c.labels, labelValues)
	c.mu.Lock()
	c.values[key] += n
	c.mu.Unlock()
}

// ============================================================
// Histogram 方法
// ============================================================

// DefaultBuckets 默认延迟桶（单位：秒）
var DefaultBuckets = []float64{0.1, 0.5, 1, 2, 5, 10, 30, 60}

// NewHistogram 创建直方图
func (r *MetricsRegistry) NewHistogram(name, help string, buckets []float64, labels ...string) *Histogram {
	r.mu.Lock()
	defer r.mu.Unlock()
	if h, ok := r.histograms[name]; ok {
		return h
	}
	if buckets == nil {
		buckets = DefaultBuckets
	}
	h := &Histogram{
		name:         name,
		help:         help,
		labels:       labels,
		buckets:      buckets,
		sums:         make(map[string]float64),
		counts:       make(map[string]uint64),
		bucketCounts: make(map[string]map[float64]uint64),
	}
	r.histograms[name] = h
	return h
}

// Observe 记录一个观测值
func (h *Histogram) Observe(value float64, labelValues ...string) {
	key := serializeLabels(h.labels, labelValues)
	h.mu.Lock()
	h.sums[key] += value
	h.counts[key]++
	if h.bucketCounts[key] == nil {
		h.bucketCounts[key] = make(map[float64]uint64)
	}
	for _, b := range h.buckets {
		if value <= b {
			h.bucketCounts[key][b]++
		}
	}
	// +Inf 桶
	if math.IsInf(h.buckets[len(h.buckets)-1], 0) {
		// 已有 +Inf 桶
	} else {
		h.bucketCounts[key][math.Inf(1)]++
	}
	h.mu.Unlock()
}

// ============================================================
// 序列化与输出
// ============================================================

// serializeLabels 将标签值序列化为字符串 key
func serializeLabels(labelNames, labelValues []string) string {
	if len(labelNames) == 0 {
		return ""
	}
	parts := make([]string, len(labelNames))
	for i, name := range labelNames {
		val := ""
		if i < len(labelValues) {
			val = labelValues[i]
		}
		parts[i] = fmt.Sprintf("%s=%q", name, val)
	}
	return strings.Join(parts, ",")
}

// formatLabels Prometheus 标签格式：{name="value",...}
func formatLabels(labelNames, labelValues []string) string {
	if len(labelNames) == 0 {
		return ""
	}
	parts := make([]string, len(labelNames))
	for i, name := range labelNames {
		val := ""
		if i < len(labelValues) {
			val = labelValues[i]
		}
		parts[i] = fmt.Sprintf(`%s="%s"`, name, escapeLabelValue(val))
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func escapeLabelValue(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", "\\n")
	return s
}

// WritePrometheus 输出所有指标为 Prometheus text exposition format
func (r *MetricsRegistry) WritePrometheus(w io.Writer) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 计数器（按 name 排序确保输出稳定）
	counterNames := make([]string, 0, len(r.counters))
	for name := range r.counters {
		counterNames = append(counterNames, name)
	}
	sort.Strings(counterNames)

	for _, name := range counterNames {
		c := r.counters[name]
		c.mu.Lock()
		fmt.Fprintf(w, "# HELP %s %s\n", c.name, c.help)
		fmt.Fprintf(w, "# TYPE %s counter\n", c.name)
		// 按 key 排序
		keys := make([]string, 0, len(c.values))
		for k := range c.values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			// 反序列化 key 为 labelValues
			labelValues := deserializeLabels(c.labels, k)
			fmt.Fprintf(w, "%s%s %g\n", c.name, formatLabels(c.labels, labelValues), c.values[k])
		}
		c.mu.Unlock()
	}

	// 直方图
	histNames := make([]string, 0, len(r.histograms))
	for name := range r.histograms {
		histNames = append(histNames, name)
	}
	sort.Strings(histNames)

	for _, name := range histNames {
		h := r.histograms[name]
		h.mu.Lock()
		fmt.Fprintf(w, "# HELP %s %s\n", h.name, h.help)
		fmt.Fprintf(w, "# TYPE %s histogram\n", h.name)
		keys := make([]string, 0, len(h.counts))
		for k := range h.counts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			labelValues := deserializeLabels(h.labels, k)
			labels := formatLabels(h.labels, labelValues)
			// 桶前缀：bucket label le="..."
			bc := h.bucketCounts[k]
			for _, b := range h.buckets {
				le := fmt.Sprintf("%g", b)
				var bucketLabels string
				if labels == "" {
					bucketLabels = fmt.Sprintf(`{le="%s"}`, le)
				} else {
					bucketLabels = fmt.Sprintf(`{le="%s",%s`, le, labels[1:]) // 去掉开头的 {，加 le 前缀
				}
				fmt.Fprintf(w, "%s_bucket%s %d\n", h.name, bucketLabels, bc[b])
			}
			// +Inf 桶（如果最后一个 bucket 不是 +Inf）
			if !math.IsInf(h.buckets[len(h.buckets)-1], 0) {
				var infLabels string
				if labels == "" {
					infLabels = `{le="+Inf"}`
				} else {
					infLabels = fmt.Sprintf(`{le="+Inf",%s`, labels[1:])
				}
				fmt.Fprintf(w, "%s_bucket%s %d\n", h.name, infLabels, h.counts[k])
			}
			// sum 和 count
			fmt.Fprintf(w, "%s_sum%s %g\n", h.name, labels, h.sums[k])
			fmt.Fprintf(w, "%s_count%s %d\n", h.name, labels, h.counts[k])
		}
		h.mu.Unlock()
	}
}

// deserializeLabels 从序列化的 key 反序列化为 labelValues
func deserializeLabels(labelNames []string, key string) []string {
	if len(labelNames) == 0 || key == "" {
		return nil
	}
	// key 格式：name1="val1",name2="val2"
	parts := strings.Split(key, ",")
	values := make([]string, len(labelNames))
	for i, name := range labelNames {
		prefix := name + "=\""
		for _, part := range parts {
			if strings.HasPrefix(part, prefix) {
				val := strings.TrimPrefix(part, prefix)
				val = strings.TrimSuffix(val, "\"")
				values[i] = val
				break
			}
		}
	}
	return values
}

// ============================================================
// 预定义指标（与文档 §4.3.3 对齐）
// ============================================================

var (
	metricGenerationDuration *Histogram
	metricSuccessRate        *Counter
	metricRetryCount         *Histogram
	metricCandidateCount     *Histogram
	metricsOnce              sync.Once
)

// initMetrics 懒初始化预定义指标（首次调用时创建）
func initMetrics() {
	metricsOnce.Do(func() {
		reg := GetMetricsRegistry()
		metricGenerationDuration = reg.NewHistogram(
			"text2sql_generation_duration_seconds",
			"Text2SQL generation duration in seconds",
			DefaultBuckets,
			"space_id",
		)
		metricSuccessRate = reg.NewCounter(
			"text2sql_success_total",
			"Total Text2SQL generation attempts by outcome (success/fail)",
			"space_id", "outcome",
		)
		metricRetryCount = reg.NewHistogram(
			"text2sql_retry_count",
			"Number of retries during Text2SQL generation",
			[]float64{0, 1, 2, 3, 5, 10},
			"space_id",
		)
		metricCandidateCount = reg.NewHistogram(
			"text2sql_candidate_count",
			"Number of candidates generated in multi-candidate mode",
			[]float64{1, 2, 3, 5, 10},
			"space_id",
		)
	})
}

// RecordGeneration P2-2: 记录一次 Text2SQL 生成的指标
// spaceID: 空间 ID；duration: 耗时；success: 是否成功；retries: 重试次数；candidates: 候选数
func RecordGeneration(spaceID int, duration time.Duration, success bool, retries, candidates int) {
	initMetrics()
	spaceLabel := fmt.Sprintf("%d", spaceID)
	metricGenerationDuration.Observe(duration.Seconds(), spaceLabel)
	if success {
		metricSuccessRate.Inc(spaceLabel, "success")
	} else {
		metricSuccessRate.Inc(spaceLabel, "fail")
	}
	metricRetryCount.Observe(float64(retries), spaceLabel)
	if candidates > 0 {
		metricCandidateCount.Observe(float64(candidates), spaceLabel)
	}
}
