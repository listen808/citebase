package observability

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// OTLP/HTTP(JSON) 导出器（P6）：把已结束的 span 推给任意兼容 OTLP 的
// 收集器（OpenTelemetry Collector / Grafana Tempo / Jaeger v2 等）。
//
// 刻意不引入 otel-go SDK：
//  1. 本项目的 span 模型（自定义 Span/StartSpan/EndSpan）已稳定运行，
//     引 SDK 等于重写整套观测调用点，收益只是协议适配；
//  2. OTLP/HTTP JSON 是规范里的标准编码，stdlib 的 net/http 就能发。
//
// 语义：批量异步导出——导出失败只丢数据不丢请求，绝不阻塞业务链路。
// 未配置 endpoint 时零开销（EndSpan 的入队检查是一次原子读）。

const (
	otlpQueueSize     = 2048
	otlpFlushInterval = 5 * time.Second
	otlpBatchMax      = 256
	otlpHTTPTimeout   = 10 * time.Second
)

var otlpExporter struct {
	mu       sync.RWMutex
	endpoint string
	client   *http.Client
	queue    chan *Span
	started  bool
}

// InitOTLPExporter 启动 OTLP 导出。endpoint 为空时不启动（默认关）。
// 幂等：重复调用只更新 endpoint。
func InitOTLPExporter(endpoint string) {
	if endpoint == "" {
		return
	}
	otlpExporter.mu.Lock()
	defer otlpExporter.mu.Unlock()
	otlpExporter.endpoint = endpoint
	if otlpExporter.started {
		return
	}
	otlpExporter.started = true
	otlpExporter.client = &http.Client{Timeout: otlpHTTPTimeout}
	otlpExporter.queue = make(chan *Span, otlpQueueSize)
	go otlpExportLoop()
}

// otlpHook EndSpan 完成后调用；队列满直接丢弃（观测数据不能反向压垮业务）。
func otlpHook(span *Span) {
	otlpExporter.mu.RLock()
	started := otlpExporter.started
	q := otlpExporter.queue
	otlpExporter.mu.RUnlock()
	if !started || span == nil {
		return
	}
	select {
	case q <- span:
	default:
	}
}

func otlpExportLoop() {
	batch := make([]*Span, 0, otlpBatchMax)
	ticker := time.NewTicker(otlpFlushInterval)
	defer ticker.Stop()
	flush := func() {
		if len(batch) == 0 {
			return
		}
		body, err := buildOTLPPayload(batch)
		batch = batch[:0]
		if err != nil {
			return
		}
		otlpPost(body)
	}
	for {
		select {
		case span := <-otlpExporter.queue:
			batch = append(batch, span)
			if len(batch) >= otlpBatchMax {
				flush()
			}
		case <-ticker.C:
			flush()
		}
	}
}

// otlpAttr OTLP 属性键值对。
type otlpAttr struct {
	Key   string      `json:"key"`
	Value otlpAttrVal `json:"value"`
}

type otlpAttrVal struct {
	StringValue string `json:"stringValue,omitempty"`
	IntValue    string `json:"intValue,omitempty"`
	DoubleValue string `json:"doubleValue,omitempty"`
	BoolValue   *bool  `json:"boolValue,omitempty"`
}

// otlpSpan OTLP trace service 的 span 记录。
type otlpSpan struct {
	TraceID           string     `json:"traceId"`
	SpanID            string     `json:"spanId"`
	ParentSpanID      string     `json:"parentSpanId,omitempty"`
	Name              string     `json:"name"`
	Kind              string     `json:"kind"`
	StartTimeUnixNano string     `json:"startTimeUnixNano"`
	EndTimeUnixNano   string     `json:"endTimeUnixNano,omitempty"`
	Attributes        []otlpAttr `json:"attributes,omitempty"`
	Status            otlpStatus `json:"status"`
}

type otlpStatus struct {
	Code    string `json:"code"`
	Message string `json:"message,omitempty"`
}

// buildOTLPPayload 编码为 OTLP/HTTP JSON（trace service）请求体。
func buildOTLPPayload(spans []*Span) ([]byte, error) {
	out := make([]otlpSpan, 0, len(spans))
	for _, s := range spans {
		os := otlpSpan{
			// OTLP 规定 spanId 为 8 字节 hex；本项目 ID 是 16 字节 hex，取前 8 字节
			TraceID:           s.TraceID,
			SpanID:            truncHex(s.ID, 16),
			ParentSpanID:      truncHex(s.ParentID, 16),
			Name:              s.Name,
			Kind:              "SPAN_KIND_INTERNAL",
			StartTimeUnixNano: strconv.FormatInt(s.StartTime.UnixNano(), 10),
		}
		if !s.EndTime.IsZero() {
			os.EndTimeUnixNano = strconv.FormatInt(s.EndTime.UnixNano(), 10)
		}
		if s.Status == SpanStatusError {
			os.Status.Code = "STATUS_CODE_ERROR"
			os.Status.Message = s.ErrorMsg
		} else {
			os.Status.Code = "STATUS_CODE_OK"
		}
		os.Attributes = append(os.Attributes, otlpAttr{Key: "service.name", Value: otlpAttrVal{StringValue: s.Service}})
		for k, v := range s.Attributes {
			os.Attributes = append(os.Attributes, toOTLPAttr(k, v))
		}
		out = append(out, os)
	}

	payload := map[string]interface{}{
		"resourceSpans": []map[string]interface{}{{
			"resource": map[string]interface{}{
				"attributes": []otlpAttr{{Key: "service.name", Value: otlpAttrVal{StringValue: "citebase"}}},
			},
			"scopeSpans": []map[string]interface{}{{
				"scope": map[string]string{"name": "citebase.ai"},
				"spans": out,
			}},
		}},
	}
	return json.Marshal(payload)
}

func toOTLPAttr(k string, v interface{}) otlpAttr {
	a := otlpAttr{Key: k}
	switch val := v.(type) {
	case string:
		a.Value.StringValue = val
	case int:
		a.Value.IntValue = strconv.Itoa(val)
	case int64:
		a.Value.IntValue = strconv.FormatInt(val, 10)
	case float64:
		a.Value.DoubleValue = strconv.FormatFloat(val, 'f', -1, 64)
	case bool:
		b := val
		a.Value.BoolValue = &b
	default:
		a.Value.StringValue = fmt.Sprintf("%v", val)
	}
	return a
}

func truncHex(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func otlpPost(body []byte) {
	otlpExporter.mu.RLock()
	endpoint := otlpExporter.endpoint
	client := otlpExporter.client
	otlpExporter.mu.RUnlock()
	if endpoint == "" || client == nil {
		return
	}
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		// 失败静默：观测导出不许打扰业务日志，只在极端情况下留痕
		if !strings.Contains(err.Error(), "context deadline exceeded") {
			log.Printf("[OTLP] export failed: %v", err)
		}
		return
	}
	resp.Body.Close()
}
