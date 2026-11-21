package rag

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"io"
	"net/http"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/config"
)

type Reranker interface {
	Rerank(ctx context.Context, query string, sources []Source) []Source
}

type CrossEncoderReranker struct {
	endpoint string
	apiKey   string
	model    string
	client   *http.Client
}

func NewCrossEncoderReranker(endpoint, apiKey, model string) *CrossEncoderReranker {
	if model == "" {
		model = "rerank-v1"
	}
	return &CrossEncoderReranker{
		endpoint: strings.TrimRight(endpoint, "/"),
		apiKey:   apiKey,
		model:    model,
		client:   &http.Client{Timeout: 10 * time.Second},
	}
}

type rerankRequest struct {
	Model     string      `json:"model"`
	Query     string      `json:"query"`
	Documents []rerankDoc `json:"documents"`
	TopN      int         `json:"top_n"`
}

type rerankDoc struct {
	Text string `json:"text"`
}

type rerankResponse struct {
	Results []rerankResult `json:"results"`
}

type rerankResult struct {
	Index          int     `json:"index"`
	RelevanceScore float64 `json:"relevance_score"`
}

func (cr *CrossEncoderReranker) Rerank(ctx context.Context, query string, sources []Source) []Source {
	if len(sources) <= 1 {
		return sources
	}

	docs := make([]rerankDoc, len(sources))
	for i, src := range sources {
		text := utils.TruncateRunes(src.DocumentName+": "+src.Snippet, 300)
		docs[i] = rerankDoc{Text: text}
	}

	reqBody := rerankRequest{
		Model:     cr.model,
		Query:     query,
		Documents: docs,
		TopN:      len(sources),
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		atomic.AddInt64(&rerankFallbackTotal, 1)
		logx.Warnf("[Reranker] marshal error, falling back to original order: %v", err)
		return sources
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", cr.endpoint+"/v1/rerank", bytes.NewReader(bodyBytes))
	if err != nil {
		atomic.AddInt64(&rerankFallbackTotal, 1)
		logx.Warnf("[Reranker] create request error, falling back to original order: %v", err)
		return sources
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if cr.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+cr.apiKey)
	}

	resp, err := cr.client.Do(httpReq)
	if err != nil {
		atomic.AddInt64(&rerankFallbackTotal, 1)
		logx.Warnf("[Reranker] API request error, falling back to original order: %v", err)
		return sources
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		atomic.AddInt64(&rerankFallbackTotal, 1)
		logx.Warnf("[Reranker] read response error, falling back to original order: %v", err)
		return sources
	}

	if resp.StatusCode != 200 {
		atomic.AddInt64(&rerankFallbackTotal, 1)
		logx.Warnf("[Reranker] API status %d, falling back to original order: %s", resp.StatusCode, string(respBody))
		return sources
	}

	var rerankResp rerankResponse
	if err := json.Unmarshal(respBody, &rerankResp); err != nil {
		atomic.AddInt64(&rerankFallbackTotal, 1)
		logx.Warnf("[Reranker] parse response error, falling back to original order: %v", err)
		return sources
	}

	scoreMap := make(map[int]float64)
	for _, result := range rerankResp.Results {
		if result.Index >= 0 && result.Index < len(sources) {
			scoreMap[result.Index] = result.RelevanceScore
		}
	}

	reranked := make([]Source, len(sources))
	copy(reranked, sources)
	for i := range reranked {
		if sc, ok := scoreMap[i]; ok && sc > 0 {
			reranked[i].Score = sc
		}
	}

	sort.Slice(reranked, func(i, j int) bool {
		return reranked[i].Score > reranked[j].Score
	})

	return reranked
}

type ScoreReranker struct {
	vectorWeight  float64
	keywordWeight float64
	freshWeight   float64
	// P1-2：知识质量分参与排序。analyzer 算出的 quality_score 此前只躺在
	// ai_analysis_result 表里，从未影响检索——analysis 与 retrieval 是两条平行链路。
	qualityFn func(docIDs []int) (map[int]float64, error)
}

func NewScoreReranker() *ScoreReranker {
	return &ScoreReranker{
		vectorWeight:  0.5,
		keywordWeight: 0.3,
		freshWeight:   0.2,
	}
}

// SetQualityProvider 注入质量分查询（批量，一次 IN 查询）
func (sr *ScoreReranker) SetQualityProvider(fn func(docIDs []int) (map[int]float64, error)) {
	sr.qualityFn = fn
}

// qualityBoost 把 0-100 的质量分映射成 ±10% 的乘数。
// 幅度刻意压小：质量分是 LLM 打的分，本身有噪声，不该盖过向量相关性。
func qualityBoost(score float64) float64 {
	if score <= 0 {
		return 1.0
	}
	if score > 100 {
		score = 100
	}
	return 1.0 + (score-50)/500 // 0 分 → 0.9，100 分 → 1.1
}

func (sr *ScoreReranker) Rerank(ctx context.Context, query string, sources []Source) []Source {
	if len(sources) <= 1 {
		return sources
	}

	keywords := extractKeywords(query)
	now := time.Now().Unix()

	var qualityScores map[int]float64
	if sr.qualityFn != nil {
		docIDs := make([]int, 0, len(sources))
		for _, s := range sources {
			docIDs = append(docIDs, s.DocumentID)
		}
		if scores, err := sr.qualityFn(docIDs); err == nil {
			qualityScores = scores
		} else {
			logx.Warnf("[Reranker] quality score lookup failed, ranking without quality boost: %v", err)
		}
	}

	for i := range sources {
		vectorScore := sources[i].Score
		if vectorScore > 1 {
			vectorScore = 1
		}

		keywordScore := sr.keywordMatchScore(sources[i], keywords)

		freshScore := sr.freshnessScore(sources[i], now)

		score := vectorScore*sr.vectorWeight + keywordScore*sr.keywordWeight + freshScore*sr.freshWeight
		if qualityScores != nil {
			score *= qualityBoost(qualityScores[sources[i].DocumentID])
		}
		sources[i].Score = score

		// GraphRAG 已接入主检索链路（hybridRetrieveWithHyDE），不再对 graph_expansion 来源降权
		// if sources[i].SourceType == "graph_expansion" {
		// 	sources[i].Score *= 0.7
		// }
	}

	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Score > sources[j].Score
	})

	return sources
}

func (sr *ScoreReranker) keywordMatchScore(src Source, keywords []string) float64 {
	if len(keywords) == 0 {
		return 0
	}

	text := strings.ToLower(src.DocumentName + " " + src.Snippet)
	matched := 0
	for _, kw := range keywords {
		if strings.Contains(text, strings.ToLower(kw)) {
			matched++
		}
	}
	return float64(matched) / float64(len(keywords))
}

func (sr *ScoreReranker) freshnessScore(src Source, now int64) float64 {
	if src.UpdatedAt <= 0 {
		return 0.5
	}
	ageSeconds := now - src.UpdatedAt
	if ageSeconds <= 0 {
		return 1.0
	}
	halfLifeSeconds := int64(365 * 24 * 3600)
	return 1.0 / (1.0 + float64(ageSeconds)/float64(halfLifeSeconds))
}

// reranker 运行态。默认配置没有 rerank_endpoint，会静默降级成打分模拟
// （0.5*向量 + 0.3*字面包含 + 0.2*时新度），而模拟与真 cross-encoder 的效果差距
// 很大。降级不可见 = "排序调不好"无从排查，所以这里必须留运行态可查。
var (
	rerankerModeMu      sync.RWMutex
	currentRerankerMode = "unknown"
	rerankFallbackTotal int64
)

// RerankerMode 返回当前生效的 reranker：cross_encoder / score_simulation / unknown
func RerankerMode() string {
	rerankerModeMu.RLock()
	defer rerankerModeMu.RUnlock()
	return currentRerankerMode
}

// RerankFallbackTotal 返回 cross-encoder 调用失败、静默退回原排序的累计次数
func RerankFallbackTotal() int64 {
	return atomic.LoadInt64(&rerankFallbackTotal)
}

func setRerankerMode(mode string) {
	rerankerModeMu.Lock()
	currentRerankerMode = mode
	rerankerModeMu.Unlock()
}

func newRerankerFromConfig() Reranker {
	cfg := config.AppConfig.AI
	if cfg.RerankEnabled && cfg.RerankEndpoint != "" {
		logx.Infof("[Reranker] Using CrossEncoderReranker: %s model=%s", cfg.RerankEndpoint, cfg.RerankModel)
		setRerankerMode("cross_encoder")
		return NewCrossEncoderReranker(cfg.RerankEndpoint, cfg.RerankApiKey, cfg.RerankModel)
	}
	logx.Warnf("[Reranker] ScoreReranker SIMULATED in use (not a cross-encoder). rerank_enabled=%v endpoint=%q — set ai.rerank_endpoint to enable real rerank", cfg.RerankEnabled, cfg.RerankEndpoint)
	setRerankerMode("score_simulation")
	sr := NewScoreReranker()
	// P1-2：即使用打分模拟，也让知识质量分参与排序
	sr.SetQualityProvider(func(docIDs []int) (map[int]float64, error) {
		return defaultQualityLookup(docIDs)
	})
	return sr
}

// defaultQualityLookup 默认的质量分查询实现（惰性构造仓库，避免 init 期依赖 DB）
var qualityRepoOnce sync.Once
var qualityRepo *knowledgerepo.AIAnalysisResultRepository

func defaultQualityLookup(docIDs []int) (map[int]float64, error) {
	qualityRepoOnce.Do(func() {
		qualityRepo = knowledgerepo.NewAIAnalysisResultRepository()
	})
	if qualityRepo == nil {
		return nil, fmt.Errorf("analysis result repository unavailable")
	}
	return qualityRepo.QualityScoresByDocIDs(docIDs)
}
