package chunking

import (
	"testing"
)

func TestNewContextualChunker_Defaults(t *testing.T) {
	chunker := NewContextualChunker(0, -1)
	if chunker.maxChunkSize != 800 {
		t.Errorf("expected default maxChunkSize=800, got %d", chunker.maxChunkSize)
	}
	if chunker.overlap != 100 {
		t.Errorf("expected default overlap=100, got %d", chunker.overlap)
	}
}

func TestNewContextualChunker_Custom(t *testing.T) {
	chunker := NewContextualChunker(500, 50)
	if chunker.maxChunkSize != 500 {
		t.Errorf("expected maxChunkSize=500, got %d", chunker.maxChunkSize)
	}
	if chunker.overlap != 50 {
		t.Errorf("expected overlap=50, got %d", chunker.overlap)
	}
}

func TestContextualChunker_Chunk_SingleLine(t *testing.T) {
	chunker := NewContextualChunker(800, 100)
	doc := Document{
		ID:      1,
		Name:    "test.md",
		Content: "Hello world",
	}
	chunks, err := chunker.Chunk(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Content != "Hello world" {
		t.Errorf("expected content 'Hello world', got %q", chunks[0].Content)
	}
	if chunks[0].StartLine != 1 {
		t.Errorf("expected StartLine=1, got %d", chunks[0].StartLine)
	}
	if chunks[0].EndLine != 1 {
		t.Errorf("expected EndLine=1, got %d", chunks[0].EndLine)
	}
	if chunks[0].ID != "doc1-chunk0" {
		t.Errorf("expected ID=doc1-chunk0, got %s", chunks[0].ID)
	}
}

func TestContextualChunker_Chunk_EmptyContent(t *testing.T) {
	chunker := NewContextualChunker(800, 100)
	doc := Document{
		ID:      1,
		Name:    "empty.md",
		Content: "",
	}
	chunks, err := chunker.Chunk(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for empty content, got %d", len(chunks))
	}
}

func TestContextualChunker_Chunk_WhitespaceOnly(t *testing.T) {
	chunker := NewContextualChunker(800, 100)
	doc := Document{
		ID:      1,
		Name:    "ws.md",
		Content: "\n\n   \n\n",
	}
	chunks, err := chunker.Chunk(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 全空白内容应该不产生 chunk
	if len(chunks) != 0 {
		t.Errorf("expected 0 chunks for whitespace-only content, got %d", len(chunks))
	}
}

func TestContextualChunker_Chunk_MultipleChunks(t *testing.T) {
	chunker := NewContextualChunker(100, 0) // 小的分块大小，0 重叠
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, "This is a line with some content to fill up the chunk size.")
	}
	doc := Document{
		ID:      1,
		Name:    "multi.md",
		Content: joinLines(lines),
	}
	chunks, err := chunker.Chunk(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Errorf("expected at least 2 chunks, got %d", len(chunks))
	}
	// 验证第一个 chunk 的行号
	if chunks[0].StartLine != 1 {
		t.Errorf("first chunk StartLine should be 1, got %d", chunks[0].StartLine)
	}
	// 验证最后一个 chunk 的行号
	if chunks[len(chunks)-1].EndLine != 20 {
		t.Errorf("last chunk EndLine should be 20, got %d", chunks[len(chunks)-1].EndLine)
	}
}

func TestContextualChunker_Chunk_WithOverlap(t *testing.T) {
	chunker := NewContextualChunker(200, 80)
	var lines []string
	for i := 0; i < 10; i++ {
		lines = append(lines, "Line with sufficient content to exceed the small chunk limit.")
	}
	doc := Document{
		ID:      1,
		Name:    "overlap.md",
		Content: joinLines(lines),
	}
	chunks, err := chunker.Chunk(doc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks with overlap, got %d", len(chunks))
	}
	// 重叠时，chunk2 的开头应该包含 chunk1 末尾的一些行
	// 验证 ID 正确
	for i, c := range chunks {
		expectedID := "doc1-chunk" + itoa(i)
		if c.ID != expectedID {
			t.Errorf("chunk %d: expected ID=%s, got %s", i, expectedID, c.ID)
		}
	}
}

func TestBuildContextHeader_NoHeadings(t *testing.T) {
	chunker := NewContextualChunker(800, 100)
	doc := Document{
		ID:       1,
		Name:     "test.md",
		Content:  "content",
		Headings: nil,
	}
	header := chunker.buildContextHeader(doc, 1, 10)
	if header != "test.md" {
		t.Errorf("expected header 'test.md', got %q", header)
	}
}

func TestBuildContextHeader_WithHeadings(t *testing.T) {
	chunker := NewContextualChunker(800, 100)
	doc := Document{
		ID:   1,
		Name: "doc.md",
		Headings: []Heading{
			{Level: 1, Title: "Introduction", LineNum: 1},
			{Level: 2, Title: "Background", LineNum: 5},
			{Level: 2, Title: "Methods", LineNum: 20},
		},
	}
	// 范围包含前两个标题
	header := chunker.buildContextHeader(doc, 1, 10)
	expected := "doc.md > Introduction > Background"
	if header != expected {
		t.Errorf("expected header %q, got %q", expected, header)
	}
}

func TestBuildContextHeader_HeadingsOutsideRange(t *testing.T) {
	chunker := NewContextualChunker(800, 100)
	doc := Document{
		ID:   1,
		Name: "doc.md",
		Headings: []Heading{
			{Level: 1, Title: "Intro", LineNum: 1},
			{Level: 2, Title: "Methods", LineNum: 50},
		},
	}
	// 范围不包含任何标题
	header := chunker.buildContextHeader(doc, 10, 20)
	if header != "doc.md" {
		t.Errorf("expected header 'doc.md', got %q", header)
	}
}

func TestEstimateTokens(t *testing.T) {
	chunker := NewContextualChunker(800, 100)
	tests := []struct {
		text     string
		expected int
	}{
		{"", 0},
		{"abcd", 1},
		{"abcdefgh", 2},
		{"1234567890123456", 4},
	}
	for _, tt := range tests {
		t.Run(tt.text, func(t *testing.T) {
			result := chunker.estimateTokens(tt.text)
			if result != tt.expected {
				t.Errorf("estimateTokens(%q) = %d, want %d", tt.text, result, tt.expected)
			}
		})
	}
}

func TestSplitSentences_Empty(t *testing.T) {
	result := splitSentences("")
	if result != nil {
		t.Errorf("expected nil for empty string, got %v", result)
	}
}

func TestSplitSentences_Whitespace(t *testing.T) {
	result := splitSentences("   ")
	if result != nil {
		t.Errorf("expected nil for whitespace-only string, got %v", result)
	}
}

func TestSplitSentences_SingleEnglish(t *testing.T) {
	result := splitSentences("Hello world.")
	if len(result) != 1 {
		t.Fatalf("expected 1 sentence, got %d", len(result))
	}
	if result[0] != "Hello world." {
		t.Errorf("expected 'Hello world.', got %q", result[0])
	}
}

func TestSplitSentences_MultipleEnglish(t *testing.T) {
	result := splitSentences("Hello world. How are you? I am fine!")
	if len(result) != 3 {
		t.Fatalf("expected 3 sentences, got %d: %v", len(result), result)
	}
	expected := []string{"Hello world.", "How are you?", "I am fine!"}
	for i, s := range expected {
		if result[i] != s {
			t.Errorf("sentence %d: expected %q, got %q", i, s, result[i])
		}
	}
}

func TestSplitSentences_Chinese(t *testing.T) {
	result := splitSentences("你好世界。今天天气怎么样？非常好！")
	if len(result) != 3 {
		t.Fatalf("expected 3 Chinese sentences, got %d: %v", len(result), result)
	}
}

func TestSplitSentences_Mixed(t *testing.T) {
	result := splitSentences("Hello. 你好。How are you? 很好！")
	if len(result) != 4 {
		t.Fatalf("expected 4 mixed sentences, got %d: %v", len(result), result)
	}
}

func TestSplitSentences_NoEndingPunctuation(t *testing.T) {
	result := splitSentences("This is a sentence without ending punctuation")
	if len(result) != 1 {
		t.Fatalf("expected 1 sentence, got %d", len(result))
	}
}

func TestCosineSim_ZeroLength(t *testing.T) {
	result := cosineSim([]float64{}, []float64{})
	if result != 0 {
		t.Errorf("expected 0 for empty vectors, got %f", result)
	}
}

func TestCosineSim_DifferentLength(t *testing.T) {
	result := cosineSim([]float64{1, 2}, []float64{1, 2, 3})
	if result != 0 {
		t.Errorf("expected 0 for different length vectors, got %f", result)
	}
}

func TestCosineSim_SameVector(t *testing.T) {
	vec := []float64{3, 4}
	result := cosineSim(vec, vec)
	if result < 0.999 {
		t.Errorf("expected ~1.0 for same vector, got %f", result)
	}
}

func TestCosineSim_Orthogonal(t *testing.T) {
	a := []float64{1, 0}
	b := []float64{0, 1}
	result := cosineSim(a, b)
	if result != 0 {
		t.Errorf("expected 0 for orthogonal vectors, got %f", result)
	}
}

func TestCosineSim_Opposite(t *testing.T) {
	a := []float64{1, 0}
	b := []float64{-1, 0}
	result := cosineSim(a, b)
	if result > -0.999 {
		t.Errorf("expected ~-1.0 for opposite vectors, got %f", result)
	}
}

func TestCosineSim_Partial(t *testing.T) {
	a := []float64{1, 1}
	b := []float64{1, 0}
	result := cosineSim(a, b)
	// 45度角，余弦 = √2/2 ≈ 0.707
	expected := 0.707
	if result < expected-0.01 || result > expected+0.01 {
		t.Errorf("expected ~%f for 45 degree, got %f", expected, result)
	}
}

func TestCosineSim_ZeroVector(t *testing.T) {
	a := []float64{0, 0}
	b := []float64{1, 1}
	result := cosineSim(a, b)
	if result != 0 {
		t.Errorf("expected 0 for zero vector, got %f", result)
	}
}

func TestSqrtf_Zero(t *testing.T) {
	result := sqrtf(0)
	if result != 0 {
		t.Errorf("sqrtf(0) = %f, want 0", result)
	}
}

func TestSqrtf_Negative(t *testing.T) {
	result := sqrtf(-1)
	if result != 0 {
		t.Errorf("sqrtf(-1) = %f, want 0", result)
	}
}

func TestSqrtf_PerfectSquares(t *testing.T) {
	tests := []struct {
		input    float64
		expected float64
	}{
		{1, 1},
		{4, 2},
		{9, 3},
		{16, 4},
		{100, 10},
	}
	for _, tt := range tests {
		t.Run(itoa(int(tt.input)), func(t *testing.T) {
			result := sqrtf(tt.input)
			if result < tt.expected-0.001 || result > tt.expected+0.001 {
				t.Errorf("sqrtf(%f) = %f, want %f", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSqrtf_Two(t *testing.T) {
	result := sqrtf(2)
	expected := 1.414
	if result < expected-0.001 || result > expected+0.001 {
		t.Errorf("sqrtf(2) = %f, want ~%f", result, expected)
	}
}

func TestContextualChunk_Struct(t *testing.T) {
	chunk := ContextualChunk{
		ID:            "test-chunk-1",
		Content:       "some content",
		ContextHeader: "doc > section",
		StartLine:     10,
		EndLine:       20,
		Tokens:        50,
	}
	if chunk.ID != "test-chunk-1" {
		t.Errorf("expected ID='test-chunk-1', got %s", chunk.ID)
	}
	if chunk.Content != "some content" {
		t.Errorf("expected Content='some content', got %s", chunk.Content)
	}
	if chunk.ContextHeader != "doc > section" {
		t.Errorf("expected ContextHeader='doc > section', got %s", chunk.ContextHeader)
	}
	if chunk.StartLine != 10 {
		t.Errorf("expected StartLine=10, got %d", chunk.StartLine)
	}
	if chunk.EndLine != 20 {
		t.Errorf("expected EndLine=20, got %d", chunk.EndLine)
	}
	if chunk.Tokens != 50 {
		t.Errorf("expected Tokens=50, got %d", chunk.Tokens)
	}
}

func TestHeading_Struct(t *testing.T) {
	h := Heading{
		Level:   2,
		Title:   "Section 1",
		LineNum: 15,
	}
	if h.Level != 2 {
		t.Errorf("expected Level=2, got %d", h.Level)
	}
	if h.Title != "Section 1" {
		t.Errorf("expected Title='Section 1', got %s", h.Title)
	}
	if h.LineNum != 15 {
		t.Errorf("expected LineNum=15, got %d", h.LineNum)
	}
}

func TestDocument_Struct(t *testing.T) {
	doc := Document{
		ID:      42,
		Name:    "guide.md",
		Content: "full content",
		Headings: []Heading{
			{Level: 1, Title: "Intro", LineNum: 1},
		},
	}
	if doc.ID != 42 {
		t.Errorf("expected ID=42, got %d", doc.ID)
	}
	if doc.Name != "guide.md" {
		t.Errorf("expected Name='guide.md', got %s", doc.Name)
	}
	if doc.Content != "full content" {
		t.Errorf("expected Content='full content', got %s", doc.Content)
	}
	if len(doc.Headings) != 1 {
		t.Errorf("expected 1 heading, got %d", len(doc.Headings))
	}
}

func TestNewChunkEnhancer(t *testing.T) {
	enhancer := NewChunkEnhancer(nil)
	if enhancer == nil {
		t.Fatal("expected non-nil ChunkEnhancer")
	}
}

func TestChunkEnhancer_EnhanceChunks_NilEmbedder(t *testing.T) {
	enhancer := NewChunkEnhancer(nil)
	doc := Document{ID: 1, Name: "test.md"}
	chunks := []ContextualChunk{
		{Content: "chunk1"},
	}
	err := enhancer.EnhanceChunks(nil, doc, chunks)
	if err != nil {
		t.Errorf("expected no error with nil embedder, got %v", err)
	}
}

// 辅助函数
func joinLines(lines []string) string {
	result := ""
	for i, l := range lines {
		if i > 0 {
			result += "\n"
		}
		result += l
	}
	return result
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
