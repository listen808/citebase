package middleware

import (
	"context"
	"errors"
	"testing"
)

type testMiddleware struct {
	name           string
	beforeRet      int
	afterRet       int
	beforeGen      int
	afterGen       int
	failAt         string // "before_retrieval" etc.
	modifyQuestion string
}

func (m *testMiddleware) Name() string { return m.name }

func (m *testMiddleware) BeforeRetrieval(ctx *RAGContext) {
	m.beforeRet++
	if m.failAt == "before_retrieval" {
		ctx.Err = errors.New("before retrieval error")
		return
	}
	if m.modifyQuestion != "" {
		ctx.Question = m.modifyQuestion
	}
}

func (m *testMiddleware) AfterRetrieval(ctx *RAGContext) {
	m.afterRet++
	if m.failAt == "after_retrieval" {
		ctx.Err = errors.New("after retrieval error")
	}
}

func (m *testMiddleware) BeforeGeneration(ctx *RAGContext) {
	m.beforeGen++
	if m.failAt == "before_generation" {
		ctx.Err = errors.New("before generation error")
	}
}

func (m *testMiddleware) AfterGeneration(ctx *RAGContext) {
	m.afterGen++
	if m.failAt == "after_generation" {
		ctx.Err = errors.New("after generation error")
	}
}

func TestNewChain(t *testing.T) {
	mw1 := &testMiddleware{name: "mw1"}
	mw2 := &testMiddleware{name: "mw2"}
	chain := NewChain(mw1, mw2)
	if chain == nil {
		t.Fatal("expected non-nil chain")
	}
	if len(chain.middlewares) != 2 {
		t.Errorf("expected 2 middlewares, got %d", len(chain.middlewares))
	}
}

func TestChain_Use(t *testing.T) {
	chain := NewChain()
	if len(chain.middlewares) != 0 {
		t.Errorf("expected 0 middlewares, got %d", len(chain.middlewares))
	}

	mw := &testMiddleware{name: "mw1"}
	chain.Use(mw)
	if len(chain.middlewares) != 1 {
		t.Errorf("expected 1 middleware after Use, got %d", len(chain.middlewares))
	}
}

func TestChain_RunBeforeRetrieval(t *testing.T) {
	mw1 := &testMiddleware{name: "mw1"}
	mw2 := &testMiddleware{name: "mw2", modifyQuestion: "rewritten question"}
	chain := NewChain(mw1, mw2)

	ctx := &RAGContext{
		Ctx:      context.Background(),
		Question: "original question",
	}
	chain.RunBeforeRetrieval(ctx)

	if mw1.beforeRet != 1 {
		t.Errorf("mw1.BeforeRetrieval called %d times, want 1", mw1.beforeRet)
	}
	if mw2.beforeRet != 1 {
		t.Errorf("mw2.BeforeRetrieval called %d times, want 1", mw2.beforeRet)
	}
	if ctx.Question != "rewritten question" {
		t.Errorf("expected rewritten question, got %s", ctx.Question)
	}
	if ctx.Err != nil {
		t.Errorf("expected no error, got %v", ctx.Err)
	}
}

func TestChain_RunBeforeRetrieval_ErrorStopsChain(t *testing.T) {
	mw1 := &testMiddleware{name: "mw1", failAt: "before_retrieval"}
	mw2 := &testMiddleware{name: "mw2"}
	chain := NewChain(mw1, mw2)

	ctx := &RAGContext{Ctx: context.Background()}
	chain.RunBeforeRetrieval(ctx)

	if mw1.beforeRet != 1 {
		t.Errorf("mw1.BeforeRetrieval called %d times, want 1", mw1.beforeRet)
	}
	if mw2.beforeRet != 0 {
		t.Errorf("mw2.BeforeRetrieval called %d times, want 0 (stopped by error)", mw2.beforeRet)
	}
	if ctx.Err == nil {
		t.Error("expected error, got nil")
	}
}

func TestChain_RunAfterRetrieval(t *testing.T) {
	mw1 := &testMiddleware{name: "mw1"}
	mw2 := &testMiddleware{name: "mw2"}
	chain := NewChain(mw1, mw2)

	ctx := &RAGContext{Ctx: context.Background()}
	chain.RunAfterRetrieval(ctx)

	if mw1.afterRet != 1 {
		t.Errorf("mw1.AfterRetrieval called %d times, want 1", mw1.afterRet)
	}
	if mw2.afterRet != 1 {
		t.Errorf("mw2.AfterRetrieval called %d times, want 1", mw2.afterRet)
	}
}

func TestChain_RunAfterRetrieval_ErrorStopsChain(t *testing.T) {
	mw1 := &testMiddleware{name: "mw1", failAt: "after_retrieval"}
	mw2 := &testMiddleware{name: "mw2"}
	chain := NewChain(mw1, mw2)

	ctx := &RAGContext{Ctx: context.Background()}
	chain.RunAfterRetrieval(ctx)

	if mw1.afterRet != 1 {
		t.Errorf("mw1.AfterRetrieval called %d times, want 1", mw1.afterRet)
	}
	if mw2.afterRet != 0 {
		t.Errorf("mw2.AfterRetrieval called %d times, want 0", mw2.afterRet)
	}
	if ctx.Err == nil {
		t.Error("expected error, got nil")
	}
}

func TestChain_RunBeforeGeneration(t *testing.T) {
	mw1 := &testMiddleware{name: "mw1"}
	mw2 := &testMiddleware{name: "mw2"}
	chain := NewChain(mw1, mw2)

	ctx := &RAGContext{Ctx: context.Background()}
	chain.RunBeforeGeneration(ctx)

	if mw1.beforeGen != 1 {
		t.Errorf("mw1.BeforeGeneration called %d times, want 1", mw1.beforeGen)
	}
	if mw2.beforeGen != 1 {
		t.Errorf("mw2.BeforeGeneration called %d times, want 1", mw2.beforeGen)
	}
}

func TestChain_RunBeforeGeneration_ErrorStopsChain(t *testing.T) {
	mw1 := &testMiddleware{name: "mw1", failAt: "before_generation"}
	mw2 := &testMiddleware{name: "mw2"}
	chain := NewChain(mw1, mw2)

	ctx := &RAGContext{Ctx: context.Background()}
	chain.RunBeforeGeneration(ctx)

	if mw1.beforeGen != 1 {
		t.Errorf("mw1.BeforeGeneration called %d times, want 1", mw1.beforeGen)
	}
	if mw2.beforeGen != 0 {
		t.Errorf("mw2.BeforeGeneration called %d times, want 0", mw2.beforeGen)
	}
	if ctx.Err == nil {
		t.Error("expected error, got nil")
	}
}

func TestChain_RunAfterGeneration(t *testing.T) {
	mw1 := &testMiddleware{name: "mw1"}
	mw2 := &testMiddleware{name: "mw2"}
	chain := NewChain(mw1, mw2)

	ctx := &RAGContext{Ctx: context.Background()}
	chain.RunAfterGeneration(ctx)

	if mw1.afterGen != 1 {
		t.Errorf("mw1.AfterGeneration called %d times, want 1", mw1.afterGen)
	}
	if mw2.afterGen != 1 {
		t.Errorf("mw2.AfterGeneration called %d times, want 1", mw2.afterGen)
	}
}

func TestChain_RunAfterGeneration_ErrorStopsChain(t *testing.T) {
	mw1 := &testMiddleware{name: "mw1", failAt: "after_generation"}
	mw2 := &testMiddleware{name: "mw2"}
	chain := NewChain(mw1, mw2)

	ctx := &RAGContext{Ctx: context.Background()}
	chain.RunAfterGeneration(ctx)

	if mw1.afterGen != 1 {
		t.Errorf("mw1.AfterGeneration called %d times, want 1", mw1.afterGen)
	}
	if mw2.afterGen != 0 {
		t.Errorf("mw2.AfterGeneration called %d times, want 0", mw2.afterGen)
	}
	if ctx.Err == nil {
		t.Error("expected error, got nil")
	}
}

func TestChain_EmptyChain(t *testing.T) {
	chain := NewChain()
	ctx := &RAGContext{Ctx: context.Background(), Question: "test"}

	chain.RunBeforeRetrieval(ctx)
	chain.RunAfterRetrieval(ctx)
	chain.RunBeforeGeneration(ctx)
	chain.RunAfterGeneration(ctx)

	if ctx.Err != nil {
		t.Errorf("expected no error from empty chain, got %v", ctx.Err)
	}
}

func TestRAGContext_DataMap(t *testing.T) {
	ctx := &RAGContext{
		Ctx:  context.Background(),
		Data: make(map[string]interface{}),
	}
	ctx.Data["key1"] = "value1"
	ctx.Data["key2"] = 42

	if v, ok := ctx.Data["key1"].(string); !ok || v != "value1" {
		t.Errorf("expected key1=value1, got %v", ctx.Data["key1"])
	}
	if v, ok := ctx.Data["key2"].(int); !ok || v != 42 {
		t.Errorf("expected key2=42, got %v", ctx.Data["key2"])
	}
}

func TestRAGDebugInfo_Struct(t *testing.T) {
	debug := &RAGDebugInfo{
		OriginalQuestion:  "original?",
		RewrittenQuestion: "rewritten?",
		SubQuestions:      []string{"q1", "q2"},
		CacheHit:          true,
		TotalDurationMs:   1500,
		RetrievalSteps: []RAGRetrievalStep{
			{Step: "1", Name: "retrieve", Description: "first step", DurationMs: 100, ResultCount: 5},
		},
	}

	if debug.OriginalQuestion != "original?" {
		t.Errorf("expected OriginalQuestion='original?', got %s", debug.OriginalQuestion)
	}
	if !debug.CacheHit {
		t.Error("expected CacheHit=true")
	}
	if len(debug.SubQuestions) != 2 {
		t.Errorf("expected 2 sub questions, got %d", len(debug.SubQuestions))
	}
	if len(debug.RetrievalSteps) != 1 {
		t.Errorf("expected 1 retrieval step, got %d", len(debug.RetrievalSteps))
	}
	if debug.RetrievalSteps[0].ResultCount != 5 {
		t.Errorf("expected ResultCount=5, got %d", debug.RetrievalSteps[0].ResultCount)
	}
}

func TestSource_Struct(t *testing.T) {
	src := Source{
		DocumentID:    123,
		DocumentName:  "test.md",
		SpaceID:       1,
		Score:         0.95,
		Snippet:       "test snippet",
		StartLine:     10,
		EndLine:       20,
		HeadingPath:   "/Chapter1/Section1",
		Keywords:      []string{"k1", "k2"},
		CitationIndex: 1,
		Embedding:     []float64{0.1, 0.2, 0.3},
		SourceType:    "document",
		Cited:         true,
		VerifyStatus:  "verified",
		DocURL:        "/doc/123",
	}

	if src.DocumentID != 123 {
		t.Errorf("expected DocumentID=123, got %d", src.DocumentID)
	}
	if src.Score != 0.95 {
		t.Errorf("expected Score=0.95, got %f", src.Score)
	}
	if len(src.Keywords) != 2 {
		t.Errorf("expected 2 keywords, got %d", len(src.Keywords))
	}
	if !src.Cited {
		t.Error("expected Cited=true")
	}
}
