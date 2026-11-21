package rag

import (
	"context"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/ai/tokenizer"
)

// ContextCompressionLayer 上下文压缩层。
//
// 将分散的压缩能力（历史压缩 HistoryCompressor、文档上下文策略 ContextManager /
// LongContextHandler、Token 预算 ContextBudgetManager）统一编排，为对话主链路提供
// 一个「一键压缩」入口：在 token 总量超过模型上下文上限时，自动对对话历史与检索
// 文档上下文进行压缩，保证 prompt 不溢出且尽量保留高价值信息。
type ContextCompressionLayer struct {
	historyComp  *HistoryCompressor
	ctxMgr       *ContextManager
	longHandler  *LongContextHandler
	budget       *ContextBudgetManager
	tk           tokenizer.Tokenizer
	maxTokens    int
	systemTokens int // 系统提示预留 token
	questionPad  int // 问题 + 生成预留 token
}

// CompressionConfig 压缩层配置
type CompressionConfig struct {
	MaxTokens    int // 模型最大上下文 token
	SystemTokens int // 系统提示预留
	QuestionPad  int // 问题与生成输出预留
}

// DefaultCompressionConfig 默认配置（面向 128k 上下文模型）
func DefaultCompressionConfig() CompressionConfig {
	return CompressionConfig{
		MaxTokens:    128000,
		SystemTokens: 1500,
		QuestionPad:  2000,
	}
}

// NewContextCompressionLayer 创建压缩层。
// p 用于历史摘要 / 文档 map-reduce（nil 时自动降级为截断策略）。
func NewContextCompressionLayer(p provider.Provider, cfg CompressionConfig) *ContextCompressionLayer {
	if cfg.MaxTokens <= 0 {
		cfg = DefaultCompressionConfig()
	}
	return &ContextCompressionLayer{
		historyComp:  NewHistoryCompressor(p),
		ctxMgr:       NewContextManager(p),
		longHandler:  NewLongContextHandler(p),
		budget:       NewContextBudgetManager(cfg.MaxTokens),
		tk:           tokenizer.NewSimpleTokenizer(),
		maxTokens:    cfg.MaxTokens,
		systemTokens: cfg.SystemTokens,
		questionPad:  cfg.QuestionPad,
	}
}

// CompressResult 压缩结果
type CompressResult struct {
	// Docs 压缩后的文档上下文文本
	Docs string
	// History 压缩后的对话历史
	History []ChatMessage
	// Compressed 本次是否触发了压缩
	Compressed bool
	// Strategy 使用的压缩策略
	Strategy ContextStrategy
	// TotalTokens 压缩后估计的总 token 数
	TotalTokens int
	// Budget 预算使用情况
	Budget *BudgetStatus
}

// Compress 对「文档上下文 + 历史」联合压缩。
// question: 用户问题；docs: 检索到的文档内容片段；history: 对话历史。
func (l *ContextCompressionLayer) Compress(ctx context.Context, question string, docs []string, history []ChatMessage) CompressResult {
	reserved := l.systemTokens + l.questionPad + l.tk.CountTokens(question)
	available := l.maxTokens - reserved
	if available < 1000 {
		available = 1000
	}

	rawDocs := strings.Join(docs, "\n")
	rawHistoryTokens := l.historyTokens(history)
	rawDocTokens := l.tk.CountTokens(rawDocs)

	totalRaw := reserved + rawDocTokens + rawHistoryTokens

	// 未超预算：原样返回
	if totalRaw <= l.maxTokens {
		return CompressResult{
			Docs:        rawDocs,
			History:     history,
			Compressed:  false,
			Strategy:    ContextStrategyDirect,
			TotalTokens: totalRaw,
			Budget:      l.budget.GetBudgetStatus(totalRaw),
		}
	}

	logx.Infof("[CompressionLayer] token=%d > max=%d, compressing", totalRaw, l.maxTokens)

	// 1. 压缩历史（旧消息摘要化）
	compressedHistory := l.historyComp.CompressHistory(ctx, history)
	compressedHistoryTokens := l.historyTokens(compressedHistory)

	// 2. 给文档上下文分配剩余预算
	docBudget := available - compressedHistoryTokens
	if docBudget < 1000 {
		docBudget = 1000
	}

	var compressedDocs string
	var strategy ContextStrategy
	if rawDocTokens <= docBudget {
		compressedDocs = rawDocs
		strategy = ContextStrategyDirect
	} else if rawDocTokens <= l.maxTokens/4 {
		// 中等长度：截断
		compressedDocs = l.truncateToTokens(rawDocs, docBudget)
		strategy = ContextStrategyTruncate
	} else {
		// 超长：走 LongContextHandler 的语义分块 + 重要性 + map-reduce
		processed, err := l.longHandler.ProcessLongDocument(ctx, rawDocs, question)
		if err != nil || processed == "" {
			processed = l.truncateToTokens(rawDocs, docBudget)
		}
		// 二次约束到预算
		if l.tk.CountTokens(processed) > docBudget {
			processed = l.truncateToTokens(processed, docBudget)
		}
		compressedDocs = processed
		strategy = ContextStrategyMapReduce
	}

	totalAfter := reserved + l.tk.CountTokens(compressedDocs) + compressedHistoryTokens
	return CompressResult{
		Docs:        compressedDocs,
		History:     compressedHistory,
		Compressed:  true,
		Strategy:    strategy,
		TotalTokens: totalAfter,
		Budget:      l.budget.GetBudgetStatus(totalAfter),
	}
}

// CompressContextText 便捷方法：直接对已经拼接好的上下文文本做压缩。
// 适用于 StreamAsk 等已经拿到整段 citedContext 的调用方。
// 返回压缩后的上下文文本与是否触发压缩。
func (l *ContextCompressionLayer) CompressContextText(ctx context.Context, question, contextText string, history []ChatMessage) (string, bool) {
	res := l.Compress(ctx, question, []string{contextText}, history)
	return res.Docs, res.Compressed
}

// historyTokens 估算历史总 token
func (l *ContextCompressionLayer) historyTokens(history []ChatMessage) int {
	n := 0
	for _, h := range history {
		n += l.tk.CountTokens(h.Question) + l.tk.CountTokens(h.Answer) + 8
	}
	return n
}

// truncateToTokens 将文本截断到指定 token 数（复用长距离处理器的估算逻辑）
func (l *ContextCompressionLayer) truncateToTokens(text string, maxTokens int) string {
	runes := []rune(text)
	currentTokens := 0
	for i, r := range runes {
		var cost int
		if r > 0x7F {
			cost = 2
		} else {
			cost = 1
		}
		if (currentTokens+cost)/4 > maxTokens {
			return string(runes[:i]) + "..."
		}
		currentTokens += cost
	}
	return text
}

// String 便于日志输出策略信息
func (r CompressResult) String() string {
	return fmt.Sprintf("compressed=%v strategy=%s totalTokens=%d", r.Compressed, r.Strategy, r.TotalTokens)
}

// ---------------------------------------------------------------------------
// TOON — Token-Optimized Object Notation
//
// A compact serialization format that reduces token usage when passing
// structured context (sources, entities, paths) to LLM prompts.
// Instead of verbose JSON field names, TOON uses abbreviated keys:
//   - s  = snippet, d = doc_name, i = index, c = confidence, r = relation
//   - e  = entities, p = paths, n = name, t = type, v = value
//
// Example: [i:0|d:report|s:"The revenue is..."]
// vs JSON: [{"index":0,"doc_name":"report","snippet":"The revenue is..."}]
// ---------------------------------------------------------------------------

// TOONField maps a long key to a short TOON key.
type TOONField struct {
	Short string
	Long  string
}

// TOONSchema defines the field mappings for a particular entity type.
type TOONSchema struct {
	EntityName string
	Fields     []TOONField
}

// Predefined TOON schemas for common RAG entities.
var (
	TOONSourceSchema = TOONSchema{
		EntityName: "src",
		Fields: []TOONField{
			{Short: "i", Long: "index"},
			{Short: "d", Long: "doc_name"},
			{Short: "s", Long: "snippet"},
			{Short: "c", Long: "confidence"},
			{Short: "u", Long: "url"},
			{Short: "v", Long: "verify_status"},
		},
	}

	TOONEntitySchema = TOONSchema{
		EntityName: "ent",
		Fields: []TOONField{
			{Short: "n", Long: "name"},
			{Short: "t", Long: "type"},
			{Short: "d", Long: "description"},
			{Short: "c", Long: "confidence"},
		},
	}

	TOONPathSchema = TOONSchema{
		EntityName: "pth",
		Fields: []TOONField{
			{Short: "f", Long: "from"},
			{Short: "o", Long: "to"},
			{Short: "r", Long: "relation"},
			{Short: "h", Long: "hop_depth"},
			{Short: "c", Long: "confidence"},
		},
	}
)

// TOONSerializer converts structured data to the compact TOON format.
type TOONSerializer struct {
	schemas map[string]*TOONSchema
}

// NewTOONSerializer creates a serializer with the built-in schemas.
func NewTOONSerializer() *TOONSerializer {
	return &TOONSerializer{
		schemas: map[string]*TOONSchema{
			"src": &TOONSourceSchema,
			"ent": &TOONEntitySchema,
			"pth": &TOONPathSchema,
		},
	}
}

// SerializeSourceToTOON serializes a Source slice to TOON format.
// Output example: [i:0|d:readme|s:"chunk text"|v:cited][i:1|d:guide|s:"..."]
func (ts *TOONSerializer) SerializeSourceToTOON(sources []Source) string {
	if len(sources) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("[")
	for idx, src := range sources {
		if idx > 0 {
			sb.WriteString("][")
		}
		sb.WriteString("i:")
		sb.WriteString(fmt.Sprintf("%d", src.CitationIndex))
		sb.WriteString("|d:")
		sb.WriteString(escapeTOON(src.DocumentName))
		sb.WriteString("|s:")
		sb.WriteString(escapeTOON(src.Snippet))
		if src.VerifyStatus != "" {
			sb.WriteString("|v:")
			sb.WriteString(src.VerifyStatus)
		}
	}
	sb.WriteString("]")
	return sb.String()
}

// SerializeEntitiesToTOON serializes entities to TOON format.
func (ts *TOONSerializer) SerializeEntitiesToTOON(entities []map[string]interface{}) string {
	if len(entities) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("[")
	for idx, ent := range entities {
		if idx > 0 {
			sb.WriteString("][")
		}
		sb.WriteString("n:")
		sb.WriteString(escapeTOON(toString(ent["name"])))
		sb.WriteString("|t:")
		sb.WriteString(toString(ent["type"]))
		if desc := toString(ent["description"]); desc != "" {
			sb.WriteString("|d:")
			sb.WriteString(escapeTOON(desc))
		}
	}
	sb.WriteString("]")
	return sb.String()
}

// SerializePathsToTOON serializes graph paths to TOON format.
func (ts *TOONSerializer) SerializePathsToTOON(paths []GraphPath) string {
	if len(paths) == 0 {
		return "[]"
	}
	var sb strings.Builder
	sb.WriteString("[")
	for idx, p := range paths {
		if idx > 0 {
			sb.WriteString("][")
		}
		sb.WriteString("f:")
		sb.WriteString(escapeTOON(p.FromEntity))
		sb.WriteString("|o:")
		sb.WriteString(escapeTOON(p.ToEntity))
		sb.WriteString("|r:")
		sb.WriteString(p.Relation)
		sb.WriteString("|h:")
		sb.WriteString(fmt.Sprintf("%d", p.HopDepth))
	}
	sb.WriteString("]")
	return sb.String()
}

// CompressForLLM returns the best available compact format for passing
// sources to the LLM prompt. When sources are few (< 5), uses TOON.
// When sources are many, falls back to numbered list to save tokens.
func CompressForLLM(sources []Source) string {
	if len(sources) == 0 {
		return ""
	}

	ts := NewTOONSerializer()

	// For small source sets, TOON is more compact and parseable
	if len(sources) <= 5 {
		return ts.SerializeSourceToTOON(sources)
	}

	// For larger sets, use numbered list with truncated snippets
	var sb strings.Builder
	for i, src := range sources {
		if i > 0 {
			sb.WriteString("\n")
		}
		snippet := src.Snippet
		runes := []rune(snippet)
		if len(runes) > 120 {
			snippet = string(runes[:120]) + "..."
		}
		sb.WriteString(fmt.Sprintf("[%d] %s: %s", src.CitationIndex, src.DocumentName, snippet))
	}
	return sb.String()
}

// escapeTOON escapes pipe characters and quotes in TOON values.
func escapeTOON(s string) string {
	s = strings.ReplaceAll(s, "|", "/")
	s = strings.ReplaceAll(s, "]", ")")
	s = strings.ReplaceAll(s, "[", "(")
	if strings.ContainsAny(s, " \t\n\"") {
		return "\"" + strings.ReplaceAll(s, "\"", "'") + "\""
	}
	return s
}

func toString(v interface{}) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}
