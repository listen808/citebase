package rag

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/connector"
	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

const (
	maxCorrectionRounds = 3
	minRelevanceScore   = 0.3
)

type CorrectiveRAG struct {
	provider      provider.Provider
	retriever     *Retriever
	hydeRetriever *HyDERetriever
	scraper       *connector.WebScraper
	webSearchEn   bool
	queryType     QueryType // C1：由调用方注入，影响相关性阈值
}

func NewCorrectiveRAG(p provider.Provider, r *Retriever, hyde *HyDERetriever) *CorrectiveRAG {
	return &CorrectiveRAG{
		provider:      p,
		retriever:     r,
		hydeRetriever: hyde,
		scraper:       connector.NewWebScraper(),
		webSearchEn:   false, // disabled by default, enable via SetWebSearch
		queryType:     QueryTypeFactual,
	}
}

// SetWebSearch enables or disables the web search fallback.
func (cr *CorrectiveRAG) SetWebSearch(enabled bool) {
	cr.webSearchEn = enabled
}

// SetQueryType 设置查询类型（C1），影响相关性评分阈值。
// 概念型/创造型降低阈值，允许更多"近似相关"的结果进入生成。
func (cr *CorrectiveRAG) SetQueryType(qt QueryType) {
	cr.queryType = qt
}

// relevanceThreshold 按查询类型返回相关性评分阈值。
func (cr *CorrectiveRAG) relevanceThreshold() float64 {
	switch cr.queryType {
	case QueryTypeConceptual, QueryTypeCreative:
		return 0.2
	case QueryTypeComparison:
		return 0.25
	default:
		return minRelevanceScore
	}
}

type RetrievalQuality struct {
	Score             float64  `json:"score"`
	ReformulatedQuery string   `json:"reformulated_query,omitempty"`
	Issues            []string `json:"issues,omitempty"`
}

func (cr *CorrectiveRAG) RetrieveAndAnswer(ctx context.Context, question string, spaceID int, limit int) (*AIAnswer, error) {
	if limit <= 0 {
		limit = 5
	}
	var allSources []Source
	currentQuery := question

	for round := 0; round < maxCorrectionRounds; round++ {
		logx.Infof("[CorrectiveRAG] Round %d, query: %s", round+1, currentQuery)

		sources, nextQuery, ok := cr.retrieveWithFallback(ctx, currentQuery, spaceID, limit, question, round+1)
		if !ok {
			currentQuery = nextQuery
			continue
		}

		quality := cr.assessRetrievalQuality(ctx, question, sources)
		if quality.Score >= cr.relevanceThreshold() {
			return cr.generateAnswer(ctx, question, sources), nil
		}

		logx.Infof("[CorrectiveRAG] Quality too low (%.2f), reformulating", quality.Score)
		if quality.ReformulatedQuery != "" {
			currentQuery = quality.ReformulatedQuery
		} else {
			currentQuery = cr.reformulateQuery(ctx, question, round+1)
		}
		allSources = append(allSources, sources...)
	}

	if len(allSources) == 0 {
		return &AIAnswer{Answer: "未找到相关文档，无法回答该问题。", Sources: []Source{}}, nil
	}
	return cr.generateAnswer(ctx, question, allSources), nil
}

func (cr *CorrectiveRAG) retrieveWithFallback(ctx context.Context, query string, spaceID int, limit int, question string, round int) ([]Source, string, bool) {
	sources, _ := cr.retriever.RetrieveContextWithRAG(ctx, query, spaceID, limit)

	if len(sources) == 0 {
		logx.Infof("[CorrectiveRAG] No results, trying HyDE")
		hydeResults, err := cr.hydeRetriever.RetrieveWithHyDE(ctx, query, spaceID, limit)
		if err == nil && len(hydeResults) > 0 {
			sources = cr.hydeResultsToSources(hydeResults)
		}
	}

	// Web search fallback when both regular and HyDE retrieval fail
	if len(sources) == 0 && cr.webSearchEn {
		logx.Infof("[CorrectiveRAG] HyDE also failed, trying web search fallback")
		webSources, err := cr.webSearchFallback(ctx, question)
		if err == nil && len(webSources) > 0 {
			sources = webSources
			logx.Infof("[CorrectiveRAG] Web search returned %d results", len(webSources))
		} else {
			logx.Infof("[CorrectiveRAG] Web search fallback failed: %v", err)
		}
	}

	if len(sources) == 0 {
		nextQuery := cr.reformulateQuery(ctx, question, round)
		return nil, nextQuery, false
	}

	return sources, "", true
}

func (cr *CorrectiveRAG) assessRetrievalQuality(ctx context.Context, question string, sources []Source) *RetrievalQuality {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	contextText := buildCitedContext(sources)
	prompt := "评估以下检索结果对问题的相关性和完整性。\n\n问题：" + question + "\n\n检索结果：" + contextText + "\n\n输出JSON格式：\n{\n  \"score\": 0.0-1.0,\n  \"reformulated_query\": \"如果检索质量差，建议一个更好的查询\",\n  \"issues\": [\"问题1\"]\n}\n\n评分标准：\n- 0.8-1.0: 结果高度相关\n- 0.5-0.8: 结果部分相关\n- 0.0-0.5: 结果不相关或不充分"
	resp, err := cr.provider.Chat(ctx, "你是一个检索质量评估专家。只输出JSON。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(256), provider.WithTemperature(0.1))
	if err != nil {
		return &RetrievalQuality{Score: 0}
	}
	var quality RetrievalQuality
	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), &quality); err != nil {
		return &RetrievalQuality{Score: 0}
	}
	return &quality
}

func (cr *CorrectiveRAG) reformulateQuery(ctx context.Context, question string, attempt int) string {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	prompt := "改写以下查询使其更适合知识库检索。这是第 " + fmt.Sprintf("%d", attempt) + " 次尝试。\n输出一个改进后的查询，不要输出其他内容。\n\n原始查询：" + question
	resp, err := cr.provider.Chat(ctx, "你是查询改写专家。只输出改写后的查询。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(128), provider.WithTemperature(0.3))
	if err != nil {
		return question
	}
	return strings.TrimSpace(resp.Content)
}

func (cr *CorrectiveRAG) generateAnswer(ctx context.Context, question string, sources []Source) *AIAnswer {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	contextText := buildCitedContext(sources)
	resp, err := cr.provider.Chat(ctx, "你是一个知识库AI助手。请基于提供的文档内容回答问题。", []provider.Message{
		provider.NewTextMessage("user", "文档内容：\n"+contextText+"\n\n问题："+question),
	}, provider.WithMaxTokens(2048), provider.WithTemperature(0.3))
	if err != nil {
		return &AIAnswer{Answer: "生成答案时出错: " + err.Error(), Sources: sources}
	}
	return &AIAnswer{Answer: resp.Content, Sources: sources}
}

func (cr *CorrectiveRAG) hydeResultsToSources(results []VectorResult) []Source {
	sources := make([]Source, 0, len(results))
	for _, r := range results {
		sources = append(sources, Source{DocumentID: r.DocID, DocumentName: r.Name, Score: r.Score})
	}
	return sources
}

// webSearchFallback uses the WebScraper to fetch relevant web pages when
// both regular retrieval and HyDE return no results. It extracts the top
// snippets from scraped content and converts them to Source objects.
func (cr *CorrectiveRAG) webSearchFallback(ctx context.Context, query string) ([]Source, error) {
	if cr.scraper == nil {
		return nil, fmt.Errorf("web scraper not initialized")
	}

	// Build a search-optimized URL using a search engine
	searchURL := fmt.Sprintf("https://www.bing.com/search?q=%s", strings.ReplaceAll(query, " ", "+"))
	scraped, err := cr.scraper.ScrapeSite(ctx, searchURL, 3)
	if err != nil {
		return nil, fmt.Errorf("web scrape failed: %w", err)
	}

	var sources []Source
	for i, page := range scraped {
		if page.Content == "" {
			continue
		}
		snippet := page.Content
		runes := []rune(snippet)
		if len(runes) > 500 {
			snippet = string(runes[:500]) + "..."
		}
		sources = append(sources, Source{
			DocumentID:    -(i + 1), // negative IDs for web sources
			DocumentName:  page.Title,
			Snippet:       snippet,
			Score:         0.5, // default moderate confidence for web results
			CitationIndex: i + 1,
		})
	}

	if len(sources) == 0 {
		return nil, fmt.Errorf("no usable content from web search")
	}

	return sources, nil
}
