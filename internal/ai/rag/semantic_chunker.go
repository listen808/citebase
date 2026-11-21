package rag

import (
	"math"
	"strings"

	"github.com/listen808/citebase/internal/ai/embedding"
	"github.com/listen808/citebase/internal/utils"
)

type SemanticChunker struct {
	maxChunkSize        int
	embedder            embedding.Embedder
	similarityThreshold float64
	fallbackChunker     Chunker
}

func NewSemanticChunker(maxChunkSize int, emb embedding.Embedder) *SemanticChunker {
	if maxChunkSize <= 0 {
		maxChunkSize = 1500
	}
	return &SemanticChunker{
		maxChunkSize:        maxChunkSize,
		embedder:            emb,
		similarityThreshold: 0.5,
		fallbackChunker:     NewMarkdownChunker(maxChunkSize),
	}
}

func (sc *SemanticChunker) Chunk(content string) []Chunk {
	if content == "" {
		return nil
	}
	if sc.embedder == nil {
		return sc.fallbackChunker.Chunk(content)
	}

	paragraphs := sc.splitIntoParagraphs(content)
	if len(paragraphs) <= 1 {
		return sc.fallbackChunker.Chunk(content)
	}

	similarities := sc.computeAdjacentSimilarities(paragraphs)
	breakPoints := sc.findBreakPoints(similarities)

	return sc.buildChunks(paragraphs, breakPoints, content)
}

func (sc *SemanticChunker) splitIntoParagraphs(content string) []string {
	var paragraphs []string
	var current strings.Builder

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if current.Len() > 0 {
				paragraphs = append(paragraphs, current.String())
				current.Reset()
			}
			continue
		}

		if strings.HasPrefix(trimmed, "#") && current.Len() > 0 {
			paragraphs = append(paragraphs, current.String())
			current.Reset()
		}

		if current.Len() > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
	}

	if current.Len() > 0 {
		paragraphs = append(paragraphs, current.String())
	}

	return paragraphs
}

func (sc *SemanticChunker) computeAdjacentSimilarities(paragraphs []string) []float64 {
	if len(paragraphs) < 2 {
		return nil
	}

	similarities := make([]float64, len(paragraphs)-1)
	type embResult struct {
		idx int
		vec []float64
		err error
	}

	embeddings := make([][]float64, len(paragraphs))
	for i, p := range paragraphs {
		text := p
		if len(text) > 8000 {
			text = utils.TruncateRunes(text, 8000)
		}
		vec, err := sc.embedder.GetEmbedding(text)
		if err != nil {
			embeddings[i] = nil
		} else {
			embeddings[i] = vec
		}
	}

	for i := 0; i < len(paragraphs)-1; i++ {
		if embeddings[i] == nil || embeddings[i+1] == nil {
			similarities[i] = 0.5
			continue
		}
		similarities[i] = utils.CosineSimilarity(embeddings[i], embeddings[i+1])
	}

	return similarities
}

func (sc *SemanticChunker) findBreakPoints(similarities []float64) []int {
	if len(similarities) == 0 {
		return nil
	}

	var breakPoints []int
	for i, sim := range similarities {
		if sim < sc.similarityThreshold {
			breakPoints = append(breakPoints, i+1)
		}
	}

	if len(breakPoints) == 0 {
		mean := 0.0
		for _, s := range similarities {
			mean += s
		}
		mean /= float64(len(similarities))

		variance := 0.0
		for _, s := range similarities {
			d := s - mean
			variance += d * d
		}
		variance /= float64(len(similarities))
		stdDev := math.Sqrt(variance)
		threshold := mean - stdDev

		for i, sim := range similarities {
			if sim < threshold {
				breakPoints = append(breakPoints, i+1)
			}
		}
	}

	return breakPoints
}

func (sc *SemanticChunker) buildChunks(paragraphs []string, breakPoints []int, originalContent string) []Chunk {
	if len(breakPoints) == 0 {
		if len(strings.Join(paragraphs, "\n\n")) <= sc.maxChunkSize {
			lineCount := strings.Count(originalContent, "\n") + 1
			return []Chunk{{
				Content:     originalContent,
				StartLine:   1,
				EndLine:     lineCount,
				HeadingPath: "",
			}}
		}
		return sc.fallbackChunker.Chunk(originalContent)
	}

	breakSet := make(map[int]bool)
	for _, bp := range breakPoints {
		breakSet[bp] = true
	}

	var chunks []Chunk
	var current strings.Builder
	currentLen := 0
	startLine := 1
	linesProcessed := 0

	for i, para := range paragraphs {
		paraLen := len(para)

		shouldBreak := breakSet[i]
		shouldOverflow := currentLen+paraLen+2 > sc.maxChunkSize && currentLen > 0

		if shouldBreak || shouldOverflow {
			if currentLen > 0 {
				content := current.String()
				lineCount := strings.Count(content, "\n") + 1
				chunks = append(chunks, Chunk{
					Content:     content,
					StartLine:   startLine,
					EndLine:     startLine + lineCount - 1,
					HeadingPath: extractHeadingPath(content),
				})
				startLine = linesProcessed + 1
				current.Reset()
				currentLen = 0
			}
		}

		if currentLen > 0 {
			current.WriteString("\n\n")
			currentLen += 2
		}
		current.WriteString(para)
		currentLen += paraLen
		linesProcessed += strings.Count(para, "\n") + 1
	}

	if currentLen > 0 {
		content := current.String()
		lineCount := strings.Count(content, "\n") + 1
		chunks = append(chunks, Chunk{
			Content:     content,
			StartLine:   startLine,
			EndLine:     startLine + lineCount - 1,
			HeadingPath: extractHeadingPath(content),
		})
	}

	for i := range chunks {
		if len(chunks[i].Content) > sc.maxChunkSize*2 {
			subChunks := sc.fallbackChunker.Chunk(chunks[i].Content)
			if len(subChunks) > 1 {
				newChunks := make([]Chunk, 0, len(chunks)-1+len(subChunks))
				newChunks = append(newChunks, chunks[:i]...)
				for _, sc := range subChunks {
					sc.StartLine += chunks[i].StartLine - 1
					sc.EndLine += chunks[i].StartLine - 1
					newChunks = append(newChunks, sc)
				}
				newChunks = append(newChunks, chunks[i+1:]...)
				chunks = newChunks
			}
		}
	}

	return chunks
}

func extractHeadingPath(content string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if matches := headingRegex.FindStringSubmatch(trimmed); matches != nil {
			return strings.TrimSpace(matches[2])
		}
	}
	return ""
}
