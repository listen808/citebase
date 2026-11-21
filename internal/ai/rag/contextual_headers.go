package rag

import (
	"context"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
)

// ContextualHeaderConfig 上下文头配置
type ContextualHeaderConfig struct {
	Enabled             bool `yaml:"enabled" json:"enabled"`
	MaxDocHeaderLen     int  `yaml:"max_doc_header_len" json:"max_doc_header_len"`         // 文档级摘要最大长度
	MaxSectionHeaderLen int  `yaml:"max_section_header_len" json:"max_section_header_len"` // 章节级摘要最大长度
	IncludeDocTitle     bool `yaml:"include_doc_title" json:"include_doc_title"`           // 是否包含文档标题
	IncludeHeadingPath  bool `yaml:"include_heading_path" json:"include_heading_path"`     // 是否包含章节路径
}

func DefaultContextualHeaderConfig() ContextualHeaderConfig {
	return ContextualHeaderConfig{
		Enabled:             true,
		MaxDocHeaderLen:     200,
		MaxSectionHeaderLen: 100,
		IncludeDocTitle:     true,
		IncludeHeadingPath:  true,
	}
}

// ContextualHeaderChunker 上下文头增强的chunker
// 在每个chunk前添加文档级和章节级上下文头，提升检索准确率
type ContextualHeaderChunker struct {
	provider provider.Provider
	config   ContextualHeaderConfig
	fallback Chunker
	docCache map[int]string // docID -> 生成的文档头
}

func NewContextualHeaderChunker(p provider.Provider, cfg ContextualHeaderConfig, fallback Chunker) *ContextualHeaderChunker {
	if cfg.MaxDocHeaderLen <= 0 {
		cfg.MaxDocHeaderLen = 200
	}
	if cfg.MaxSectionHeaderLen <= 0 {
		cfg.MaxSectionHeaderLen = 100
	}
	if fallback == nil {
		fallback = NewMarkdownChunker(1500)
	}
	return &ContextualHeaderChunker{
		provider: p,
		config:   cfg,
		fallback: fallback,
		docCache: make(map[int]string),
	}
}

// Chunk 实现Chunker接口 - 在chunk前添加上下文头
func (c *ContextualHeaderChunker) Chunk(content string) []Chunk {
	return c.fallback.Chunk(content)
}

// ChunkWithContext 带文档上下文的chunking
func (c *ContextualHeaderChunker) ChunkWithContext(ctx context.Context, content string, docID int, docTitle string, headingPath string) []Chunk {
	chunks := c.fallback.Chunk(content)
	if len(chunks) == 0 {
		return nil
	}

	// 生成文档级摘要头
	docHeader := c.generateDocHeader(ctx, content, docID, docTitle)

	// 为每个chunk添加上下文头
	for i := range chunks {
		chunks[i].Content = c.buildHeader(docHeader, chunks[i], headingPath) + chunks[i].Content
	}

	return chunks
}

// BatchChunkWithContext 批量处理多个文档的chunking
func (c *ContextualHeaderChunker) BatchChunkWithContext(ctx context.Context, docs []DocumentChunkInput) map[int][]Chunk {
	result := make(map[int][]Chunk)

	for _, doc := range docs {
		chunks := c.ChunkWithContext(ctx, doc.Content, doc.DocID, doc.Title, doc.HeadingPath)
		result[doc.DocID] = chunks
	}

	return result
}

// generateDocHeader 生成文档级上下文头
func (c *ContextualHeaderChunker) generateDocHeader(ctx context.Context, content string, docID int, docTitle string) string {
	// 检查缓存
	if cached, ok := c.docCache[docID]; ok {
		return cached
	}

	var header strings.Builder

	// 文档标题
	if c.config.IncludeDocTitle && docTitle != "" {
		header.WriteString(fmt.Sprintf("文档: %s\n", docTitle))
	}

	// 使用LLM生成文档摘要
	if c.provider != nil {
		summary := c.generateSummary(ctx, content)
		if summary != "" {
			header.WriteString(fmt.Sprintf("摘要: %s\n", summary))
		}
	}

	result := header.String()
	c.docCache[docID] = result
	return result
}

// generateSummary 使用LLM生成内容摘要
func (c *ContextualHeaderChunker) generateSummary(ctx context.Context, content string) string {
	if c.provider == nil {
		return ""
	}

	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	// 只使用前2000字符生成摘要
	text := truncateText(content, 2000)
	prompt := fmt.Sprintf(`请用一句话（不超过%d个字）概括以下文档的核心内容：

%s

只输出概括内容，不要其他文字。`, c.config.MaxDocHeaderLen, text)

	resp, err := c.provider.Chat(ctx, "你是一个文档分析专家。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(128), provider.WithTemperature(0.2))
	if err != nil {
		logx.Infof("[ContextualHeader] Failed to generate summary: %v", err)
		return ""
	}

	return strings.TrimSpace(resp.Content)
}

// buildHeader 构建chunk的上下文头
func (c *ContextualHeaderChunker) buildHeader(docHeader string, chunk Chunk, headingPath string) string {
	var header strings.Builder

	// 文档级头
	if docHeader != "" {
		header.WriteString(docHeader)
	}

	// 章节路径
	if c.config.IncludeHeadingPath && (headingPath != "" || chunk.HeadingPath != "") {
		path := headingPath
		if path == "" {
			path = chunk.HeadingPath
		}
		if path != "" {
			header.WriteString(fmt.Sprintf("章节: %s\n", path))
		}
	}

	if header.Len() == 0 {
		return ""
	}

	return header.String() + "---\n"
}

// ClearCache 清除文档头缓存
func (c *ContextualHeaderChunker) ClearCache() {
	c.docCache = make(map[int]string)
}

// DocumentChunkInput 文档chunk输入
type DocumentChunkInput struct {
	DocID       int
	Title       string
	Content     string
	HeadingPath string
}
