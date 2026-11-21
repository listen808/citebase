package observability

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

type InMemoryProvider struct {
	mu           sync.RWMutex
	traces       map[string]*Span
	recentTraces []string
	maxTraces    int

	metricsMu  sync.RWMutex
	counters   map[string]*MetricValue
	gauges     map[string]*MetricValue
	histograms map[string]*histogramEntry
}

type histogramEntry struct {
	data   HistogramData
	labels map[string]string
}

var (
	defaultProvider *InMemoryProvider
	once            sync.Once
)

func InitProvider() {
	once.Do(func() {
		defaultProvider = &InMemoryProvider{
			traces:       make(map[string]*Span),
			recentTraces: make([]string, 0, 1000),
			maxTraces:    1000,
			counters:     make(map[string]*MetricValue),
			gauges:       make(map[string]*MetricValue),
			histograms:   make(map[string]*histogramEntry),
		}
	})
}

func DefaultProvider() *InMemoryProvider {
	if defaultProvider == nil {
		InitProvider()
	}
	return defaultProvider
}

func generateID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func metricKey(name string, labels map[string]string) string {
	keys := make([]string, 0, len(labels))
	for k := range labels {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var parts []string
	parts = append(parts, name)
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%s", k, labels[k]))
	}
	return strings.Join(parts, ":")
}

func (p *InMemoryProvider) StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, *Span) {
	cfg := &spanConfig{
		service: "ai",
	}
	for _, opt := range opts {
		opt(cfg)
	}

	parentSpan := SpanFromContext(ctx)
	existingTraceID := TraceIDFromContext(ctx)

	var traceID, parentID string
	if cfg.traceID != "" {
		traceID = cfg.traceID
	} else if existingTraceID != "" {
		traceID = existingTraceID
	} else if parentSpan != nil {
		traceID = parentSpan.TraceID
	} else {
		traceID = generateID()
	}

	if cfg.parentID != "" {
		parentID = cfg.parentID
	} else if parentSpan != nil {
		parentID = parentSpan.ID
	}

	span := &Span{
		ID:         generateID(),
		TraceID:    traceID,
		ParentID:   parentID,
		Name:       name,
		Service:    cfg.service,
		Status:     SpanStatusRunning,
		StartTime:  time.Now(),
		Attributes: make(map[string]interface{}),
		Events:     make([]SpanEvent, 0),
		Children:   make([]*Span, 0),
	}

	if cfg.attributes != nil {
		for k, v := range cfg.attributes {
			span.Attributes[k] = v
		}
	}

	p.mu.Lock()
	if parentSpan != nil {
		parentSpan.Children = append(parentSpan.Children, span)
	}

	traceRoot := p.findRootSpan(traceID)
	if traceRoot == nil {
		p.traces[traceID] = span
		p.recentTraces = append([]string{traceID}, p.recentTraces...)
		if len(p.recentTraces) > p.maxTraces {
			oldest := p.recentTraces[len(p.recentTraces)-1]
			delete(p.traces, oldest)
			p.recentTraces = p.recentTraces[:len(p.recentTraces)-1]
		}
	}
	p.mu.Unlock()

	newCtx := context.WithValue(ctx, spanContextKey, span)
	newCtx = context.WithValue(newCtx, traceIDContextKey, traceID)
	return newCtx, span
}

func (p *InMemoryProvider) findRootSpan(traceID string) *Span {
	if root, ok := p.traces[traceID]; ok {
		return root
	}
	return nil
}

func (p *InMemoryProvider) EndSpan(span *Span, status SpanStatus) {
	if span == nil {
		return
	}
	// P0：span 对象同时存放在 p.traces 中，会被 GetTrace/ListRecentTraces 的读取方
	// （HTTP 序列化）并发访问，所有写操作必须持 p.mu。
	p.mu.Lock()
	defer p.mu.Unlock()
	span.EndTime = time.Now()
	span.DurationMs = span.EndTime.Sub(span.StartTime).Milliseconds()
	span.Status = status
	// P6：OTLP 导出钩子（未配置 endpoint 时是一次原子读，零开销）
	otlpHook(span)
}

func (p *InMemoryProvider) SetAttribute(span *Span, key string, value interface{}) {
	if span == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if span.Attributes == nil {
		span.Attributes = make(map[string]interface{})
	}
	span.Attributes[key] = value
}

func (p *InMemoryProvider) AddEvent(span *Span, name string, attrs map[string]interface{}) {
	if span == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	// 复制 attrs：否则调用方后续修改该 map 会与读取方的遍历产生竞争
	var copied map[string]interface{}
	if attrs != nil {
		copied = make(map[string]interface{}, len(attrs))
		for k, v := range attrs {
			copied[k] = v
		}
	}
	event := SpanEvent{
		Name:       name,
		Time:       time.Now(),
		Attributes: copied,
	}
	span.Events = append(span.Events, event)
}

// cloneSpan 深拷贝 span 树，供只读快照返回给调用方。
// P0：直接返回内部 *Span 会让 HTTP 序列化与 StartSpan/EndSpan/SetAttribute 并发读写
// 同一个 map（Attributes）→ 并发 map 读写是 runtime fatal，recover 拦不住。
func cloneSpan(s *Span) *Span {
	if s == nil {
		return nil
	}
	cp := *s
	if s.Attributes != nil {
		cp.Attributes = make(map[string]interface{}, len(s.Attributes))
		for k, v := range s.Attributes {
			cp.Attributes[k] = v
		}
	} else {
		cp.Attributes = nil
	}
	if s.Events != nil {
		cp.Events = make([]SpanEvent, len(s.Events))
		for i, ev := range s.Events {
			if ev.Attributes != nil {
				attrs := make(map[string]interface{}, len(ev.Attributes))
				for k, v := range ev.Attributes {
					attrs[k] = v
				}
				ev.Attributes = attrs
			}
			cp.Events[i] = ev
		}
	} else {
		cp.Events = nil
	}
	if len(s.Children) > 0 {
		cp.Children = make([]*Span, 0, len(s.Children))
		for _, child := range s.Children {
			cp.Children = append(cp.Children, cloneSpan(child))
		}
	} else {
		cp.Children = nil
	}
	return &cp
}

func (p *InMemoryProvider) GetTrace(traceID string) (*Span, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	if root, ok := p.traces[traceID]; ok {
		return cloneSpan(root), nil
	}
	return nil, fmt.Errorf("trace not found: %s", traceID)
}

func (p *InMemoryProvider) ListRecentTraces(limit int) []*Span {
	p.mu.RLock()
	defer p.mu.RUnlock()
	result := make([]*Span, 0, limit)
	for i, traceID := range p.recentTraces {
		if i >= limit {
			break
		}
		if root, ok := p.traces[traceID]; ok {
			result = append(result, cloneSpan(root))
		}
	}
	return result
}

func (p *InMemoryProvider) Inc(name string, labels map[string]string, value float64) {
	key := metricKey(name, labels)
	p.metricsMu.Lock()
	defer p.metricsMu.Unlock()
	if m, ok := p.counters[key]; ok {
		m.Value += value
		m.UpdatedAt = time.Now()
	} else {
		p.counters[key] = &MetricValue{
			Name:      name,
			Type:      MetricCounter,
			Value:     value,
			Labels:    labels,
			UpdatedAt: time.Now(),
		}
	}
}

func (p *InMemoryProvider) SetGauge(name string, labels map[string]string, value float64) {
	key := metricKey(name, labels)
	p.metricsMu.Lock()
	defer p.metricsMu.Unlock()
	if m, ok := p.gauges[key]; ok {
		m.Value = value
		m.UpdatedAt = time.Now()
	} else {
		p.gauges[key] = &MetricValue{
			Name:      name,
			Type:      MetricGauge,
			Value:     value,
			Labels:    labels,
			UpdatedAt: time.Now(),
		}
	}
}

func (p *InMemoryProvider) AddGauge(name string, labels map[string]string, value float64) {
	key := metricKey(name, labels)
	p.metricsMu.Lock()
	defer p.metricsMu.Unlock()
	if m, ok := p.gauges[key]; ok {
		m.Value += value
		m.UpdatedAt = time.Now()
	} else {
		p.gauges[key] = &MetricValue{
			Name:      name,
			Type:      MetricGauge,
			Value:     value,
			Labels:    labels,
			UpdatedAt: time.Now(),
		}
	}
}

func (p *InMemoryProvider) ObserveHistogram(name string, labels map[string]string, value float64) {
	key := metricKey(name, labels)
	p.metricsMu.Lock()
	defer p.metricsMu.Unlock()
	if h, ok := p.histograms[key]; ok {
		h.data.Count++
		h.data.Sum += value
		if value < h.data.Min {
			h.data.Min = value
		}
		if value > h.data.Max {
			h.data.Max = value
		}
	} else {
		p.histograms[key] = &histogramEntry{
			data: HistogramData{
				Count: 1,
				Sum:   value,
				Min:   value,
				Max:   value,
			},
			labels: labels,
		}
	}
}

func (p *InMemoryProvider) GetMetric(name string, labels map[string]string) (*MetricValue, bool) {
	key := metricKey(name, labels)
	p.metricsMu.RLock()
	defer p.metricsMu.RUnlock()
	if m, ok := p.counters[key]; ok {
		return m, true
	}
	if m, ok := p.gauges[key]; ok {
		return m, true
	}
	return nil, false
}

func (p *InMemoryProvider) GetHistogram(name string, labels map[string]string) (*HistogramData, bool) {
	key := metricKey(name, labels)
	p.metricsMu.RLock()
	defer p.metricsMu.RUnlock()
	if h, ok := p.histograms[key]; ok {
		return &h.data, true
	}
	return nil, false
}

func (p *InMemoryProvider) ListMetrics(prefix string) []*MetricValue {
	p.metricsMu.RLock()
	defer p.metricsMu.RUnlock()
	result := make([]*MetricValue, 0)
	for _, m := range p.counters {
		if prefix == "" || strings.HasPrefix(m.Name, prefix) {
			result = append(result, m)
		}
	}
	for _, m := range p.gauges {
		if prefix == "" || strings.HasPrefix(m.Name, prefix) {
			result = append(result, m)
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result
}
