package compression

import (
	"context"
	"strings"

	"github.com/listen808/citebase/internal/ai/embedding"
)

// Source 压缩器使用的最小来源结构。
// 不直接引用 rag.Source 以避免 import 循环（rag → compression → rag）。
// 由 rag 包通过 ConvertSources 转换后传入。
type Source struct {
	Snippet string
	Score   float64
}

// ContextualCompressor 压缩检索结果，只保留与查询相关的部分
type ContextualCompressor struct {
	embedder embedding.Embedder
	topK     int
	minScore float64
}

// CompressStats 压缩统计
type CompressStats struct {
	OriginalTokens   int
	CompressedTokens int
	SavedTokens      int
	SavedPercent     float64
	ChunksProcessed  int
	ChunksFiltered   int
	KeptIndices      []int // 原始列表中被保留的来源下标（按原始顺序）
}

// NewContextualCompressor 创建默认压缩器
func NewContextualCompressor(embedder embedding.Embedder) *ContextualCompressor {
	return &ContextualCompressor{
		embedder: embedder,
		topK:     3,
		minScore: 0.5,
	}
}

// Compress 压缩检索结果
func (c *ContextualCompressor) Compress(
	ctx context.Context,
	question string,
	sources []Source,
) ([]Source, CompressStats, error) {
	stats := CompressStats{ChunksProcessed: len(sources)}
	if c.embedder == nil || len(sources) == 0 {
		return sources, stats, nil
	}

	// 问题向量
	qVec, err := c.embedder.GetEmbeddings([]string{question})
	if err != nil || len(qVec) == 0 {
		return sources, stats, err
	}

	var compressed []Source
	for i, src := range sources {
		stats.OriginalTokens += len(src.Snippet) / 4

		compressedContent, score, err := c.CompressSingleChunk(ctx, question, src.Snippet, qVec[0])
		if err != nil || score < c.minScore {
			stats.ChunksFiltered++
			continue
		}

		src.Snippet = compressedContent
		src.Score = (src.Score + score) / 2 // 综合分数
		compressed = append(compressed, src)
		stats.KeptIndices = append(stats.KeptIndices, i)
		stats.CompressedTokens += len(compressedContent) / 4
	}

	stats.SavedTokens = stats.OriginalTokens - stats.CompressedTokens
	if stats.OriginalTokens > 0 {
		stats.SavedPercent = float64(stats.SavedTokens) / float64(stats.OriginalTokens) * 100
	}
	return compressed, stats, nil
}

// CompressSingleChunk 压缩单个 chunk，提取与查询相关的句子
func (c *ContextualCompressor) CompressSingleChunk(
	ctx context.Context,
	question string,
	chunkContent string,
	qVec []float64,
) (string, float64, error) {
	sentences := splitSentencesForCompress(chunkContent)
	if len(sentences) == 0 {
		return chunkContent, 1.0, nil
	}
	if len(sentences) <= c.topK {
		return chunkContent, 1.0, nil
	}

	// 计算每个句子与问题的相似度
	sims, err := c.embedder.GetEmbeddings(sentences)
	if err != nil {
		return chunkContent, 1.0, err
	}

	type scored struct {
		idx   int
		score float64
	}
	scoredSentences := make([]scored, len(sentences))
	for i, s := range sims {
		sim := cosineSimilarity(qVec, s)
		scoredSentences[i] = scored{idx: i, score: sim}
	}

	// 选择 topK 个最相似的句子
	selected := make(map[int]bool)
	avgScore := 0.0
	for i := 0; i < c.topK && i < len(scoredSentences); i++ {
		maxIdx := i
		for j := i + 1; j < len(scoredSentences); j++ {
			if scoredSentences[j].score > scoredSentences[maxIdx].score {
				maxIdx = j
			}
		}
		scoredSentences[i], scoredSentences[maxIdx] = scoredSentences[maxIdx], scoredSentences[i]
		selected[scoredSentences[i].idx] = true
		avgScore += scoredSentences[i].score
	}
	avgScore /= float64(c.topK)

	// 按原文顺序输出选中的句子
	var parts []string
	for i, s := range sentences {
		if selected[i] {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, " "), avgScore, nil
}

// splitSentencesForCompress 分句
func splitSentencesForCompress(text string) []string {
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}
	var sentences []string
	var current strings.Builder
	for _, r := range text {
		current.WriteRune(r)
		if r == '。' || r == '．' || r == '.' || r == '？' || r == '?' || r == '！' || r == '!' || r == '\n' {
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

// cosineSimilarity 余弦相似度
func cosineSimilarity(a, b []float64) float64 {
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
	return dot / (sqrtFast(na) * sqrtFast(nb))
}

func sqrtFast(x float64) float64 {
	if x <= 0 {
		return 0
	}
	g := x / 2
	for i := 0; i < 10; i++ {
		g = (g + x/g) / 2
	}
	return g
}
