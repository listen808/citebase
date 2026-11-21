package observability

import (
	"context"
	"time"
)

type SpanStatus string

const (
	SpanStatusOK      SpanStatus = "ok"
	SpanStatusError   SpanStatus = "error"
	SpanStatusRunning SpanStatus = "running"
)

type Span struct {
	ID         string                 `json:"id"`
	TraceID    string                 `json:"trace_id"`
	ParentID   string                 `json:"parent_id,omitempty"`
	Name       string                 `json:"name"`
	Service    string                 `json:"service"`
	Status     SpanStatus             `json:"status"`
	StartTime  time.Time              `json:"start_time"`
	EndTime    time.Time              `json:"end_time,omitempty"`
	DurationMs int64                  `json:"duration_ms"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
	Events     []SpanEvent            `json:"events,omitempty"`
	Children   []*Span                `json:"children,omitempty"`
	ErrorMsg   string                 `json:"error_msg,omitempty"`
}

type SpanEvent struct {
	Name       string                 `json:"name"`
	Time       time.Time              `json:"time"`
	Attributes map[string]interface{} `json:"attributes,omitempty"`
}

type TraceProvider interface {
	StartSpan(ctx context.Context, name string, opts ...SpanOption) (context.Context, *Span)
	EndSpan(span *Span, status SpanStatus)
	SetAttribute(span *Span, key string, value interface{})
	AddEvent(span *Span, name string, attrs map[string]interface{})
	GetTrace(traceID string) (*Span, error)
	ListRecentTraces(limit int) []*Span
}

type SpanOption func(*spanConfig)

type spanConfig struct {
	traceID    string
	parentID   string
	service    string
	attributes map[string]interface{}
}

func WithTraceID(traceID string) SpanOption {
	return func(c *spanConfig) { c.traceID = traceID }
}

func WithParent(parentID string) SpanOption {
	return func(c *spanConfig) { c.parentID = parentID }
}

func WithService(service string) SpanOption {
	return func(c *spanConfig) { c.service = service }
}

func WithAttributes(attrs map[string]interface{}) SpanOption {
	return func(c *spanConfig) { c.attributes = attrs }
}

type contextKey string

const spanContextKey contextKey = "current_span"
const traceIDContextKey contextKey = "trace_id"

func SpanFromContext(ctx context.Context) *Span {
	if span, ok := ctx.Value(spanContextKey).(*Span); ok {
		return span
	}
	return nil
}

func TraceIDFromContext(ctx context.Context) string {
	if traceID, ok := ctx.Value(traceIDContextKey).(string); ok {
		return traceID
	}
	return ""
}
