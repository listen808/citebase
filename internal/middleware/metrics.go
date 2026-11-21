package middleware

import (
	"fmt"
	"io"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/ai/observability"
)

// HTTP metrics 中间件 + 运行时指标导出
// 输出 Prometheus text exposition format，不引入外部依赖

// ============================================================
// 指标数据结构
// ============================================================

type httpMetricsData struct {
	mu              sync.Mutex
	requestTotal    map[string]float64 // key: method_status
	requestDuration map[string][]float64
	requestBytes    map[string]float64
}

var globalHTTPMetrics = &httpMetricsData{
	requestTotal:    make(map[string]float64),
	requestDuration: make(map[string][]float64),
	requestBytes:    make(map[string]float64),
}

// MetricsMiddleware HTTP 指标采集中间件
func MetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestSize := c.Request.ContentLength

		c.Next()

		duration := time.Since(start).Seconds()
		method := c.Request.Method
		status := c.Writer.Status()
		statusClass := fmt.Sprintf("%dxx", status/100)
		key := fmt.Sprintf("%s_%s", method, statusClass)

		globalHTTPMetrics.mu.Lock()
		globalHTTPMetrics.requestTotal[key]++
		globalHTTPMetrics.requestDuration[key] = append(globalHTTPMetrics.requestDuration[key], duration)
		if requestSize > 0 {
			globalHTTPMetrics.requestBytes[key] += float64(requestSize)
		}
		// 只保留最近 1000 个延迟样本
		if len(globalHTTPMetrics.requestDuration[key]) > 1000 {
			globalHTTPMetrics.requestDuration[key] = globalHTTPMetrics.requestDuration[key][len(globalHTTPMetrics.requestDuration[key])-1000:]
		}
		globalHTTPMetrics.mu.Unlock()
	}
}

// WriteMetrics 输出 Prometheus 格式的指标
func WriteMetrics(w io.Writer) {
	globalHTTPMetrics.mu.Lock()
	defer globalHTTPMetrics.mu.Unlock()

	var sb strings.Builder

	// ========== HTTP 请求指标 ==========

	// 1. 请求总数
	sb.WriteString("# HELP http_requests_total HTTP 请求总数（按方法和状态码分类）\n")
	sb.WriteString("# TYPE http_requests_total counter\n")
	keys := make([]string, 0, len(globalHTTPMetrics.requestTotal))
	for k := range globalHTTPMetrics.requestTotal {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts := strings.SplitN(k, "_", 2)
		method, statusClass := parts[0], parts[1]
		sb.WriteString(fmt.Sprintf("http_requests_total{method=%q,status=%q} %.0f\n", method, statusClass, globalHTTPMetrics.requestTotal[k]))
	}

	// 2. 请求延迟
	sb.WriteString("# HELP http_request_duration_seconds HTTP 请求延迟（秒）\n")
	sb.WriteString("# TYPE http_request_duration_seconds summary\n")
	for _, k := range keys {
		parts := strings.SplitN(k, "_", 2)
		method, statusClass := parts[0], parts[1]
		durations := globalHTTPMetrics.requestDuration[k]
		if len(durations) == 0 {
			continue
		}
		// 计算百分位
		sorted := make([]float64, len(durations))
		copy(sorted, durations)
		sort.Float64s(sorted)
		count := len(sorted)
		p50 := sorted[count*50/100]
		p90 := sorted[count*90/100]
		p99 := sorted[min(count*99/100, count-1)]
		sum := 0.0
		for _, d := range durations {
			sum += d
		}
		sb.WriteString(fmt.Sprintf("http_request_duration_seconds{method=%q,status=%q,quantile=\"0.5\"} %f\n", method, statusClass, p50))
		sb.WriteString(fmt.Sprintf("http_request_duration_seconds{method=%q,status=%q,quantile=\"0.9\"} %f\n", method, statusClass, p90))
		sb.WriteString(fmt.Sprintf("http_request_duration_seconds{method=%q,status=%q,quantile=\"0.99\"} %f\n", method, statusClass, p99))
		sb.WriteString(fmt.Sprintf("http_request_duration_seconds_sum{method=%q,status=%q} %f\n", method, statusClass, sum))
		sb.WriteString(fmt.Sprintf("http_request_duration_seconds_count{method=%q,status=%q} %d\n", method, statusClass, count))
	}

	// 3. 请求字节
	sb.WriteString("# HELP http_request_bytes_total HTTP 请求字节总数\n")
	sb.WriteString("# TYPE http_request_bytes_total counter\n")
	for _, k := range keys {
		parts := strings.SplitN(k, "_", 2)
		method, statusClass := parts[0], parts[1]
		sb.WriteString(fmt.Sprintf("http_request_bytes_total{method=%q,status=%q} %f\n", method, statusClass, globalHTTPMetrics.requestBytes[k]))
	}

	// ========== Go 运行时指标 ==========

	// 4. Goroutine 数量
	sb.WriteString("# HELP go_goroutines 当前 Goroutine 数量\n")
	sb.WriteString("# TYPE go_goroutines gauge\n")
	sb.WriteString(fmt.Sprintf("go_goroutines %d\n", runtime.NumGoroutine()))

	// 5. 内存使用
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	sb.WriteString("# HELP go_memstats_alloc_bytes 已分配内存字节数\n")
	sb.WriteString("# TYPE go_memstats_alloc_bytes gauge\n")
	sb.WriteString(fmt.Sprintf("go_memstats_alloc_bytes %d\n", m.Alloc))
	sb.WriteString("# HELP go_memstats_sys_bytes 系统内存字节数\n")
	sb.WriteString("# TYPE go_memstats_sys_bytes gauge\n")
	sb.WriteString(fmt.Sprintf("go_memstats_sys_bytes %d\n", m.Sys))
	sb.WriteString("# HELP go_memstats_heap_inuse_bytes 堆使用字节数\n")
	sb.WriteString("# TYPE go_memstats_heap_inuse_bytes gauge\n")
	sb.WriteString(fmt.Sprintf("go_memstats_heap_inuse_bytes %d\n", m.HeapInuse))

	// 6. GC 统计
	sb.WriteString("# HELP go_gc_duration_seconds GC 耗时（秒）\n")
	sb.WriteString("# TYPE go_gc_duration_seconds summary\n")
	sb.WriteString(fmt.Sprintf("go_gc_duration_seconds_sum %f\n", float64(m.PauseTotalNs)/1e9))
	sb.WriteString(fmt.Sprintf("go_gc_duration_seconds_count %d\n", m.NumGC))

	// 7. 启动时间
	sb.WriteString("# HELP process_start_time_seconds 进程启动时间\n")
	sb.WriteString("# TYPE process_start_time_seconds gauge\n")
	sb.WriteString(fmt.Sprintf("process_start_time_seconds %d\n", processStartTime))

	_, _ = w.Write([]byte(sb.String()))
}

// WriteAIMetrics P1-5：把 AI 侧指标追加到 Prometheus 输出。
// 此前 ai_requests_total / ai_token_usage_total 等只在管理端 JSON 接口里可见，
// 进不了监控系统，成本与调用量长期没人看。
func WriteAIMetrics(w io.Writer) {
	provider := observability.DefaultProvider()
	if provider == nil {
		return
	}
	var sb strings.Builder
	for _, m := range provider.ListMetrics("ai_") {
		sb.WriteString(fmt.Sprintf("# TYPE %s gauge\n", m.Name))
		if len(m.Labels) == 0 {
			sb.WriteString(fmt.Sprintf("%s %f\n", m.Name, m.Value))
			continue
		}
		labels := make([]string, 0, len(m.Labels))
		for k := range m.Labels {
			labels = append(labels, k)
		}
		sort.Strings(labels)
		parts := make([]string, 0, len(labels))
		for _, k := range labels {
			parts = append(parts, fmt.Sprintf("%s=%q", k, m.Labels[k]))
		}
		sb.WriteString(fmt.Sprintf("%s{%s} %f\n", m.Name, strings.Join(parts, ","), m.Value))
	}
	_, _ = w.Write([]byte(sb.String()))
}

var processStartTime = time.Now().Unix()
