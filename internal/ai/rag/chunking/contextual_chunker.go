package chunking

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/embedding"
)

// Document 待分块文档
type Document struct {
	ID       int
	Name     string
	Content  string
	Headings []Heading
}

// Heading 文档标题层级
type Heading struct {
	Level   int
	Title   string
	LineNum int
}

// ContextualChunk 带上下文头部的分块
type ContextualChunk struct {
	ID            string
	Content       string
	ContextHeader string
	StartLine     int
	EndLine       int
	Tokens        int
}

// ContextualChunker 为每个分块添加上下文头部
type ContextualChunker struct {
	maxChunkSize int
	overlap      int
}

// NewContextualChunker 创建上下文分块器
func NewContextualChunker(maxChunkSize, overlap int) *ContextualChunker {
	if maxChunkSize <= 0 {
		maxChunkSize = 800
	}
	if overlap < 0 {
		overlap = 100
	}
	return &ContextualChunker{maxChunkSize: maxChunkSize, overlap: overlap}
}

// Chunk 将文档分块并为每个块添加上下文头部
func (c *ContextualChunker) Chunk(doc Document) ([]ContextualChunk, error) {
	lines := strings.Split(doc.Content, "\n")
	var chunks []ContextualChunk

	currentLines := make([]string, 0, c.maxChunkSize/80)
	currentSize := 0
	startLine := 1
	chunkIdx := 0

	for i, line := range lines {
		currentLines = append(currentLines, line)
		currentSize += len(line) + 1

		if currentSize >= c.maxChunkSize {
			content := strings.Join(currentLines, "\n")
			header := c.buildContextHeader(doc, startLine, i+1)
			chunk := ContextualChunk{
				ID:            fmt.Sprintf("doc%d-chunk%d", doc.ID, chunkIdx),
				Content:       content,
				ContextHeader: header,
				StartLine:     startLine,
				EndLine:       i + 1,
				Tokens:        c.estimateTokens(content),
			}
			chunks = append(chunks, chunk)
			chunkIdx++

			// overlap：保留最后几行
			overlapStart := len(currentLines) - c.overlap/80
			if overlapStart < 0 {
				overlapStart = 0
			}
			currentLines = currentLines[overlapStart:]
			currentSize = 0
			for _, l := range currentLines {
				currentSize += len(l) + 1
			}
			startLine = i + 2 - len(currentLines)
		}
	}

	// 剩余内容
	if len(currentLines) > 0 {
		content := strings.Join(currentLines, "\n")
		if strings.TrimSpace(content) != "" {
			header := c.buildContextHeader(doc, startLine, len(lines))
			chunk := ContextualChunk{
				ID:            fmt.Sprintf("doc%d-chunk%d", doc.ID, chunkIdx),
				Content:       content,
				ContextHeader: header,
				StartLine:     startLine,
				EndLine:       len(lines),
				Tokens:        c.estimateTokens(content),
			}
			chunks = append(chunks, chunk)
		}
	}

	return chunks, nil
}

// buildContextHeader 构建上下文头部：文档名 > 章节 > 段落
func (c *ContextualChunker) buildContextHeader(doc Document, startLine, endLine int) string {
	parts := []string{doc.Name}
	for _, h := range doc.Headings {
		if h.LineNum >= startLine && h.LineNum <= endLine {
			parts = append(parts, h.Title)
		}
	}
	return strings.Join(parts, " > ")
}

// estimateTokens 粗略估算 token 数（4 字符 ≈ 1 token）
func (c *ContextualChunker) estimateTokens(text string) int {
	return len(text) / 4
}

// SemanticChunkerEnhanced 基于语义边界分块
type SemanticChunkerEnhanced struct {
	embedder     embedding.Embedder
	threshold    float64
	minChunkSize int
	maxChunkSize int
}

// NewSemanticChunkerEnhanced 创建语义分块器
func NewSemanticChunkerEnhanced(embedder embedding.Embedder) *SemanticChunkerEnhanced {
	return &SemanticChunkerEnhanced{
		embedder:     embedder,
		threshold:    0.65,
		minChunkSize: 200,
		maxChunkSize: 1200,
	}
}

// Chunk 基于语义相似度在自然断点处分块
func (s *SemanticChunkerEnhanced) Chunk(ctx context.Context, doc Document) ([]ContextualChunk, error) {
	sentences := splitSentences(doc.Content)
	if len(sentences) <= 1 {
		return []ContextualChunk{{
			ID:            fmt.Sprintf("doc%d-chunk0", doc.ID),
			Content:       doc.Content,
			ContextHeader: doc.Name,
			Tokens:        len(doc.Content) / 4,
		}}, nil
	}

	// 计算相邻句子向量的相似度
	embeddings, err := s.embedder.GetEmbeddings(sentences)
	if err != nil || len(embeddings) != len(sentences) {
		// 降级为固定长度分块
		return NewContextualChunker(s.maxChunkSize, 0).Chunk(doc)
	}

	// 寻找语义断点（相似度低于阈值的位置）
	breakPoints := []int{0}
	for i := 0; i < len(sentences)-1; i++ {
		sim := cosineSim(embeddings[i], embeddings[i+1])
		if sim < s.threshold && i*20 > s.minChunkSize {
			breakPoints = append(breakPoints, i+1)
		}
	}
	breakPoints = append(breakPoints, len(sentences))

	// 组装分块
	var chunks []ContextualChunk
	for i := 0; i < len(breakPoints)-1; i++ {
		start := breakPoints[i]
		end := breakPoints[i+1]
		content := strings.Join(sentences[start:end], " ")
		if len(content) < 20 {
			continue
		}
		chunk := ContextualChunk{
			ID:            fmt.Sprintf("doc%d-seg%d", doc.ID, i),
			Content:       content,
			ContextHeader: doc.Name,
			StartLine:     start + 1,
			EndLine:       end,
			Tokens:        len(content) / 4,
		}
		chunks = append(chunks, chunk)
	}
	return chunks, nil
}

// splitSentences 按句号/问号/感叹号分割句子
func splitSentences(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var sentences []string
	var current strings.Builder
	for _, r := range text {
		current.WriteRune(r)
		if r == '。' || r == '．' || r == '.' || r == '？' || r == '?' || r == '！' || r == '!' {
			if s := strings.TrimSpace(current.String()); s != "" {
				sentences = append(sentences, s)
			}
			current.Reset()
		}
	}
	if s := strings.TrimSpace(current.String()); s != "" {
		sentences = append(sentences, s)
	}
	return sentences
}

// cosineSim 余弦相似度
func cosineSim(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, na, nb float64
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (sqrtf(na) * sqrtf(nb))
}

func sqrtf(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// 牛顿法
	g := x / 2
	for i := 0; i < 10; i++ {
		g = (g + x/g) / 2
	}
	return g
}

// ChunkEnhancer 为已存在的分块批量增强上下文
type ChunkEnhancer struct {
	embedder embedding.Embedder
}

// NewChunkEnhancer 创建分块增强器
func NewChunkEnhancer(embedder embedding.Embedder) *ChunkEnhancer {
	return &ChunkEnhancer{embedder: embedder}
}

// EnhanceChunks 为分块列表添加上下文头部和元数据
func (e *ChunkEnhancer) EnhanceChunks(ctx context.Context, doc Document, chunks []ContextualChunk) error {
	if e.embedder == nil {
		return nil
	}
	for i := range chunks {
		if chunks[i].ContextHeader == "" {
			chunks[i].ContextHeader = doc.Name
		}
		chunks[i].Tokens = len(chunks[i].Content) / 4
	}
	_ = time.Now // 避免 time 未使用
	return nil
}
