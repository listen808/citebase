package rag

import (
	"strings"

	"github.com/listen808/citebase/internal/ai/embedding"
)

type ChunkStrategy string

const (
	StrategyAuto     ChunkStrategy = "auto"
	StrategyMarkdown ChunkStrategy = "markdown"
	StrategySemantic ChunkStrategy = "semantic"
	StrategyCode     ChunkStrategy = "code"
	StrategyFlat     ChunkStrategy = "flat"
)

type AdaptiveChunker struct {
	maxChunkSize int
	embedder     embedding.Embedder
	strategy     ChunkStrategy
}

func NewAdaptiveChunker(maxChunkSize int, emb embedding.Embedder, strategy ChunkStrategy) *AdaptiveChunker {
	if maxChunkSize <= 0 {
		maxChunkSize = 1500
	}
	if strategy == "" {
		strategy = StrategyAuto
	}
	return &AdaptiveChunker{
		maxChunkSize: maxChunkSize,
		embedder:     emb,
		strategy:     strategy,
	}
}

func (ac *AdaptiveChunker) Chunk(content string) []Chunk {
	if content == "" {
		return nil
	}

	chunker := ac.selectChunker(content)
	return chunker.Chunk(content)
}

func (ac *AdaptiveChunker) selectChunker(content string) Chunker {
	if ac.strategy != StrategyAuto {
		return ac.strategyToChunker(ac.strategy)
	}

	docType := detectDocumentType(content)
	switch docType {
	case "markdown":
		if ac.embedder != nil {
			return NewSemanticChunker(ac.maxChunkSize, ac.embedder)
		}
		return NewMarkdownChunker(ac.maxChunkSize)
	case "code":
		return NewCodeChunker(ac.maxChunkSize)
	case "structured":
		return NewMarkdownChunker(ac.maxChunkSize)
	default:
		return NewRecursiveChunker(ac.maxChunkSize)
	}
}

func (ac *AdaptiveChunker) strategyToChunker(s ChunkStrategy) Chunker {
	switch s {
	case StrategyMarkdown:
		return NewMarkdownChunker(ac.maxChunkSize)
	case StrategySemantic:
		if ac.embedder != nil {
			return NewSemanticChunker(ac.maxChunkSize, ac.embedder)
		}
		return NewMarkdownChunker(ac.maxChunkSize)
	case StrategyCode:
		return NewCodeChunker(ac.maxChunkSize)
	case StrategyFlat:
		return NewRecursiveChunker(ac.maxChunkSize)
	default:
		return NewMarkdownChunker(ac.maxChunkSize)
	}
}

func detectDocumentType(content string) string {
	lines := strings.Split(content, "\n")
	totalLines := len(lines)
	if totalLines == 0 {
		return "flat"
	}

	headingCount := 0
	codeBlockOpen := false
	codeLineCount := 0
	listCount := 0
	tableLineCount := 0

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "#") {
			headingCount++
		}
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			codeBlockOpen = !codeBlockOpen
			continue
		}
		if codeBlockOpen {
			codeLineCount++
			continue
		}
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
			listCount++
		}
		if len(trimmed) > 0 && trimmed[0] == '|' && strings.Contains(trimmed, "|") {
			tableLineCount++
		}
	}

	codeRatio := float64(codeLineCount) / float64(totalLines)
	markdownFeatures := headingCount + listCount + tableLineCount

	if codeRatio > 0.6 {
		return "code"
	}
	if headingCount >= 2 || markdownFeatures >= 3 {
		return "markdown"
	}
	if headingCount == 1 || listCount >= 2 || tableLineCount >= 1 {
		return "structured"
	}
	return "flat"
}

type CodeChunker struct {
	maxChunkSize int
}

func NewCodeChunker(maxChunkSize int) *CodeChunker {
	if maxChunkSize <= 0 {
		maxChunkSize = 1500
	}
	return &CodeChunker{maxChunkSize: maxChunkSize}
}

func (cc *CodeChunker) Chunk(content string) []Chunk {
	if content == "" {
		return nil
	}

	blocks := cc.extractCodeBlocks(content)
	if len(blocks) > 0 {
		return blocks
	}

	return cc.chunkByFunction(content)
}

func (cc *CodeChunker) extractCodeBlocks(content string) []Chunk {
	lines := strings.Split(content, "\n")
	var chunks []Chunk
	var current strings.Builder
	inBlock := false
	lang := ""
	startLine := 1

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			if inBlock {
				current.WriteString(line)
				current.WriteString("\n")
				chunks = append(chunks, Chunk{
					Content:     current.String(),
					StartLine:   startLine,
					EndLine:     i + 1,
					HeadingPath: lang,
				})
				current.Reset()
				inBlock = false
				lang = ""
			} else {
				inBlock = true
				startLine = i + 1
				marker := "```"
				if strings.HasPrefix(trimmed, "~~~") {
					marker = "~~~"
				}
				lang = strings.TrimSpace(strings.TrimPrefix(trimmed, marker))
			}
			continue
		}

		if inBlock {
			if current.Len() > 0 {
				current.WriteString("\n")
			}
			current.WriteString(line)
		}
	}

	return chunks
}

func (cc *CodeChunker) chunkByFunction(content string) []Chunk {
	lines := strings.Split(content, "\n")
	var chunks []Chunk
	var current strings.Builder
	currentLen := 0
	startLine := 1

	functionPattern := []string{"func ", "function ", "def ", "class ", "async def ", "pub fn ", "fn ", "public ", "private ", "protected "}

	for i, line := range lines {
		isFunctionStart := false
		for _, pat := range functionPattern {
			if strings.HasPrefix(strings.TrimSpace(line), pat) {
				isFunctionStart = true
				break
			}
		}

		if isFunctionStart && currentLen > 0 {
			chunks = append(chunks, Chunk{
				Content:   current.String(),
				StartLine: startLine,
				EndLine:   i,
			})
			current.Reset()
			currentLen = 0
			startLine = i + 1
		}

		if currentLen > 0 {
			current.WriteString("\n")
		}
		current.WriteString(line)
		currentLen += len(line) + 1

		if currentLen > cc.maxChunkSize {
			chunks = append(chunks, Chunk{
				Content:   current.String(),
				StartLine: startLine,
				EndLine:   i + 1,
			})
			current.Reset()
			currentLen = 0
			startLine = i + 2
		}
	}

	if currentLen > 0 {
		chunks = append(chunks, Chunk{
			Content:   current.String(),
			StartLine: startLine,
			EndLine:   len(lines),
		})
	}

	return chunks
}
