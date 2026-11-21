package search

import (
	"context"
	"math"
	"testing"
)

// ============================================================
// Score Fusion Tests
// ============================================================

func TestReciprocalRankFusion_Basic(t *testing.T) {
	list1 := []RankedResult{
		{DocumentID: 1, Score: 10.0},
		{DocumentID: 2, Score: 8.0},
		{DocumentID: 3, Score: 6.0},
	}
	list2 := []RankedResult{
		{DocumentID: 2, Score: 0.9},
		{DocumentID: 3, Score: 0.8},
		{DocumentID: 4, Score: 0.7},
	}

	fused := ReciprocalRankFusion([][]RankedResult{list1, list2}, 60)

	if len(fused) == 0 {
		t.Fatal("expected fused results, got empty")
	}

	// Document 2 appears at rank 2 in list1 and rank 1 in list2:
	// score = 1/(60+2) + 1/(60+1) = 1/62 + 1/61 ≈ 0.03252
	// Document 1 only in list1 at rank 1: 1/(60+1) = 1/61 ≈ 0.01639
	// Document 2 should score higher than document 1
	scoreMap := make(map[uint]float64)
	for _, r := range fused {
		scoreMap[r.DocumentID] = r.Score
	}

	if scoreMap[2] <= scoreMap[1] {
		t.Errorf("doc 2 (%f) should score higher than doc 1 (%f) due to appearing in both lists", scoreMap[2], scoreMap[1])
	}

	// All 4 documents should be present
	for _, id := range []uint{1, 2, 3, 4} {
		if _, ok := scoreMap[id]; !ok {
			t.Errorf("missing document %d in fused results", id)
		}
	}
}

func TestReciprocalRankFusion_Empty(t *testing.T) {
	fused := ReciprocalRankFusion([][]RankedResult{}, 60)
	if len(fused) != 0 {
		t.Errorf("expected empty results, got %d", len(fused))
	}

	fused = ReciprocalRankFusion([][]RankedResult{{}, {}}, 60)
	if len(fused) != 0 {
		t.Errorf("expected empty results for empty lists, got %d", len(fused))
	}
}

func TestReciprocalRankFusion_SingleList(t *testing.T) {
	list := []RankedResult{
		{DocumentID: 10, Score: 5.0},
		{DocumentID: 20, Score: 3.0},
	}

	fused := ReciprocalRankFusion([][]RankedResult{list}, 60)
	if len(fused) != 2 {
		t.Fatalf("expected 2 results, got %d", len(fused))
	}

	// First result should be doc 10 (rank 0 → 1/61 > 1/62)
	if fused[0].DocumentID != 10 {
		t.Errorf("expected doc 10 first, got %d", fused[0].DocumentID)
	}
}

func TestReciprocalRankFusion_DefaultK(t *testing.T) {
	list := []RankedResult{{DocumentID: 1, Score: 1.0}}
	fused := ReciprocalRankFusion([][]RankedResult{list}, 0)
	if len(fused) != 1 {
		t.Fatalf("expected 1 result, got %d", len(fused))
	}
	// k should default to 60: score = 1/61
	expected := 1.0 / 61.0
	if math.Abs(fused[0].Score-expected) > 1e-9 {
		t.Errorf("expected score %f, got %f", expected, fused[0].Score)
	}
}

func TestWeightedLinearFusion_Basic(t *testing.T) {
	ftResults := []RankedResult{
		{DocumentID: 1, Score: 10.0},
		{DocumentID: 2, Score: 5.0},
	}
	vecResults := []RankedResult{
		{DocumentID: 2, Score: 0.9},
		{DocumentID: 3, Score: 0.5},
	}

	fused := WeightedLinearFusion(ftResults, vecResults, 0.5, 0.5)

	scoreMap := make(map[uint]float64)
	for _, r := range fused {
		scoreMap[r.DocumentID] = r.Score
	}

	// Doc 2 should appear in both and have combined score
	if _, ok := scoreMap[2]; !ok {
		t.Error("doc 2 missing from fused results")
	}

	// All 3 docs should be present
	for _, id := range []uint{1, 2, 3} {
		if _, ok := scoreMap[id]; !ok {
			t.Errorf("missing document %d", id)
		}
	}
}

func TestWeightedLinearFusion_Empty(t *testing.T) {
	fused := WeightedLinearFusion(nil, nil, 0.5, 0.5)
	if len(fused) != 0 {
		t.Errorf("expected empty, got %d", len(fused))
	}

	ft := []RankedResult{{DocumentID: 1, Score: 5.0}}
	fused = WeightedLinearFusion(ft, nil, 0.5, 0.5)
	if len(fused) != 1 {
		t.Errorf("expected 1 result from ft-only, got %d", len(fused))
	}
}

func TestWeightedLinearFusion_ScoreNormalization(t *testing.T) {
	ftResults := []RankedResult{
		{DocumentID: 1, Score: 100.0},
		{DocumentID: 2, Score: 50.0},
	}
	vecResults := []RankedResult{
		{DocumentID: 1, Score: 1.0},
		{DocumentID: 2, Score: 0.5},
	}

	fused := WeightedLinearFusion(ftResults, vecResults, 0.5, 0.5)

	// After normalization, doc 1 in ft = 1.0, doc 1 in vec = 1.0
	// Combined: 0.5 * 1.0 + 0.5 * 1.0 = 1.0
	scoreMap := make(map[uint]float64)
	for _, r := range fused {
		scoreMap[r.DocumentID] = r.Score
	}

	if math.Abs(scoreMap[1]-1.0) > 1e-9 {
		t.Errorf("doc 1 expected score 1.0, got %f", scoreMap[1])
	}

	// Doc 2: ft = 0.5, vec = 0.5 → 0.5 * 0.5 + 0.5 * 0.5 = 0.5
	if math.Abs(scoreMap[2]-0.5) > 1e-9 {
		t.Errorf("doc 2 expected score 0.5, got %f", scoreMap[2])
	}
}

func TestNormalizeScores(t *testing.T) {
	results := []RankedResult{
		{DocumentID: 1, Score: 10.0},
		{DocumentID: 2, Score: 5.0},
		{DocumentID: 3, Score: 0.0},
	}

	normalized := normalizeScores(results)
	if len(normalized) != 3 {
		t.Fatalf("expected 3, got %d", len(normalized))
	}

	if math.Abs(normalized[0].Score-1.0) > 1e-9 {
		t.Errorf("max score should normalize to 1.0, got %f", normalized[0].Score)
	}
	if math.Abs(normalized[1].Score-0.5) > 1e-9 {
		t.Errorf("half-max should normalize to 0.5, got %f", normalized[1].Score)
	}
	if math.Abs(normalized[2].Score-0.0) > 1e-9 {
		t.Errorf("zero score should normalize to 0.0, got %f", normalized[2].Score)
	}
}

func TestNormalizeScores_Empty(t *testing.T) {
	normalized := normalizeScores(nil)
	if normalized != nil {
		t.Error("expected nil for empty input")
	}
}

// ============================================================
// Query Processor Tests
// ============================================================

func TestDetectIntent_Keyword(t *testing.T) {
	tests := []struct {
		query string
		want  QueryIntent
	}{
		{"Go", IntentKeyword},
		{"API design", IntentKeyword},
		{"微服务", IntentKeyword},
		{"K8s 部署", IntentKeyword},
	}

	for _, tt := range tests {
		got := DetectIntent(tt.query)
		if got != tt.want {
			t.Errorf("DetectIntent(%q) = %s, want %s", tt.query, got, tt.want)
		}
	}
}

func TestDetectIntent_Question(t *testing.T) {
	tests := []struct {
		query string
		want  QueryIntent
	}{
		{"什么是微服务架构？", IntentQuestion},
		{"如何部署 Kubernetes 集群到生产环境", IntentQuestion},
		{"为什么需要使用消息队列来处理异步任务", IntentQuestion},
		{"How to configure SSL certificates?", IntentQuestion},
	}

	for _, tt := range tests {
		got := DetectIntent(tt.query)
		if got != tt.want {
			t.Errorf("DetectIntent(%q) = %s, want %s", tt.query, got, tt.want)
		}
	}
}

func TestDetectIntent_Hybrid(t *testing.T) {
	tests := []struct {
		query string
		want  QueryIntent
	}{
		{"Kubernetes deployment configuration best practices production cluster", IntentHybrid},
	}

	for _, tt := range tests {
		got := DetectIntent(tt.query)
		if got != tt.want {
			t.Errorf("DetectIntent(%q) = %s, want %s", tt.query, got, tt.want)
		}
	}
}

func TestDetectIntent_Empty(t *testing.T) {
	got := DetectIntent("")
	if got != IntentKeyword {
		t.Errorf("DetectIntent(\"\") = %s, want %s", got, IntentKeyword)
	}
}

func TestSelectSearchMode(t *testing.T) {
	tests := []struct {
		intent QueryIntent
		mode   string
		want   string
	}{
		{IntentKeyword, "", ModeFulltext},
		{IntentQuestion, "", ModeSemantic},
		{IntentHybrid, "", ModeHybrid},
		{IntentKeyword, "semantic", ModeSemantic},  // user override
		{IntentQuestion, "fulltext", ModeFulltext}, // user override
		{IntentHybrid, "hybrid", ModeHybrid},       // explicit hybrid
		{IntentKeyword, "hybrid", ModeFulltext},    // hybrid mode lets intent decide
	}

	for _, tt := range tests {
		got := SelectSearchMode(tt.intent, tt.mode)
		if got != tt.want {
			t.Errorf("SelectSearchMode(%s, %q) = %s, want %s", tt.intent, tt.mode, got, tt.want)
		}
	}
}

func TestCountWords(t *testing.T) {
	tests := []struct {
		text string
		want int
	}{
		{"hello world", 2},
		{"微服务架构", 5},    // 5 CJK characters: 微/服/务/架/构
		{"K8s 部署方案", 5}, // "K8s" (1 latin word) + 4 CJK chars = 5
		{"", 0},
	}

	for _, tt := range tests {
		got := countWords(tt.text)
		if got != tt.want {
			t.Errorf("countWords(%q) = %d, want %d", tt.text, got, tt.want)
		}
	}
}

func TestExpandQuery(t *testing.T) {
	result := ExpandQuery("  hello world  ")
	if result != "hello world" {
		t.Errorf("ExpandQuery should trim whitespace, got %q", result)
	}
}

// ============================================================
// Hybrid Search Integration (mock-based)
// ============================================================

type mockFullTextSearcher struct {
	results []SearchHit
	err     error
}

func (m *mockFullTextSearcher) Search(_ context.Context, _ string, _ FullTextSearchOptions) ([]SearchHit, error) {
	return m.results, m.err
}

type mockVectorSearcher struct {
	embedResult []float64
	embedErr    error
	results     []VectorHit
	searchErr   error
}

func (m *mockVectorSearcher) Embed(_ context.Context, _ string) ([]float64, error) {
	return m.embedResult, m.embedErr
}

func (m *mockVectorSearcher) Search(_ context.Context, _ []float64, _ int) ([]VectorHit, error) {
	return m.results, m.searchErr
}

func TestHybridSearch_FulltextOnly(t *testing.T) {
	ft := &mockFullTextSearcher{
		results: []SearchHit{
			{DocumentID: 1, Score: 5.0, Snippet: "snippet 1"},
			{DocumentID: 2, Score: 3.0, Snippet: "snippet 2"},
		},
	}

	cfg := DefaultHybridSearchConfig()
	cfg.MinScore = 0
	svc := NewHybridSearchService(ft, nil, cfg, nil)

	req := &HybridSearchRequest{
		Query:      "test",
		SearchMode: ModeFulltext,
		PageSize:   10,
	}

	results, total, _, err := svc.Search(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 2 {
		t.Errorf("expected total 2, got %d", total)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
}

func TestHybridSearch_EmptyQuery(t *testing.T) {
	ft := &mockFullTextSearcher{}
	cfg := DefaultHybridSearchConfig()
	svc := NewHybridSearchService(ft, nil, cfg, nil)

	req := &HybridSearchRequest{Query: "", PageSize: 10}
	_, _, _, err := svc.Search(context.Background(), req)
	if err == nil {
		t.Error("expected error for empty query")
	}
}

func TestHybridSearch_Pagination(t *testing.T) {
	ft := &mockFullTextSearcher{
		results: []SearchHit{
			{DocumentID: 1, Score: 10.0},
			{DocumentID: 2, Score: 9.0},
			{DocumentID: 3, Score: 8.0},
			{DocumentID: 4, Score: 7.0},
			{DocumentID: 5, Score: 6.0},
		},
	}

	cfg := DefaultHybridSearchConfig()
	cfg.MinScore = 0
	svc := NewHybridSearchService(ft, nil, cfg, nil)

	req := &HybridSearchRequest{
		Query:      "test",
		SearchMode: ModeFulltext,
		Page:       1,
		PageSize:   2,
	}

	results, total, _, err := svc.Search(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if total != 5 {
		t.Errorf("expected total 5, got %d", total)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results on page 1, got %d", len(results))
	}
}

func TestFilterBySpaceIDs(t *testing.T) {
	results := []HybridSearchResult{
		{DocumentID: 1, SpaceID: 10},
		{DocumentID: 2, SpaceID: 20},
		{DocumentID: 3, SpaceID: 10},
	}

	filtered := FilterBySpaceIDs(results, []uint{10})
	if len(filtered) != 2 {
		t.Errorf("expected 2 results after space filter, got %d", len(filtered))
	}

	// Empty space IDs should return all
	all := FilterBySpaceIDs(results, nil)
	if len(all) != 3 {
		t.Errorf("expected 3 results with no filter, got %d", len(all))
	}
}
