package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/provider"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
	"github.com/listen808/citebase/internal/utils"
)

type RAGASMetrics struct {
	Faithfulness     float64 `json:"faithfulness"`
	AnswerRelevancy  float64 `json:"answer_relevancy"`
	ContextPrecision float64 `json:"context_precision"`
	ContextRecall    float64 `json:"context_recall"`
}

type RAGASEvalResult struct {
	ID             int          `json:"id"`
	Question       string       `json:"question"`
	Answer         string       `json:"answer"`
	ExpectedAnswer string       `json:"expected_answer"`
	Contexts       []string     `json:"contexts"`
	GroundTruth    string       `json:"ground_truth"`
	Metrics        RAGASMetrics `json:"metrics"`
	AvgScore       float64      `json:"avg_score"`
	Duration       string       `json:"duration"`
	CreatedAt      time.Time    `json:"created_at"`
}

type RAGASEvaluator struct {
	provider provider.Provider
	results  map[int]*RAGASEvalResult
	nextID   int
	mu       sync.RWMutex
	repo     *knowledgerepo.AiRagasEvalRepository
}

var ragasEvaluator *RAGASEvaluator
var ragasOnce sync.Once

func InitRAGASEvaluator(p provider.Provider, repo *knowledgerepo.AiRagasEvalRepository) {
	ragasOnce.Do(func() {
		ragasEvaluator = &RAGASEvaluator{
			provider: p,
			results:  make(map[int]*RAGASEvalResult),
			nextID:   1,
			repo:     repo,
		}
		logx.Info("[RAGAS] Evaluator initialized with DB persistence")
	})
}

func GetRAGASEvaluator() *RAGASEvaluator {
	// P3：用 sync.Once 替代无锁 nil 检查，避免并发时多个 goroutine 同时初始化
	ragasOnce.Do(func() {
		if ragasEvaluator == nil {
			ragasEvaluator = &RAGASEvaluator{
				results: make(map[int]*RAGASEvalResult),
				nextID:  1,
			}
		}
	})
	return ragasEvaluator
}

func (e *RAGASEvaluator) Evaluate(question, answer string, contexts []string, groundTruth string) (*RAGASEvalResult, error) {
	if e.provider == nil {
		return nil, fmt.Errorf("RAGAS evaluator: no LLM provider configured")
	}

	start := time.Now()
	result := &RAGASEvalResult{
		Question:    question,
		Answer:      answer,
		Contexts:    contexts,
		GroundTruth: groundTruth,
	}

	ctx := context.Background()
	metrics := RAGASMetrics{}

	faith, err := e.computeFaithfulness(ctx, answer, contexts)
	if err != nil {
		logx.Infof("[RAGAS] Faithfulness error: %v", err)
		faith = 0
	}
	metrics.Faithfulness = faith

	ar, err := e.computeAnswerRelevancy(ctx, question, answer)
	if err != nil {
		logx.Infof("[RAGAS] Answer Relevancy error: %v", err)
		ar = 0
	}
	metrics.AnswerRelevancy = ar

	cp, err := e.computeContextPrecision(ctx, question, contexts)
	if err != nil {
		logx.Infof("[RAGAS] Context Precision error: %v", err)
		cp = 0
	}
	metrics.ContextPrecision = cp

	if groundTruth != "" {
		cr, err := e.computeContextRecall(ctx, groundTruth, contexts)
		if err != nil {
			logx.Infof("[RAGAS] Context Recall error: %v", err)
			cr = 0
		}
		metrics.ContextRecall = cr
	}

	count := 3
	total := metrics.Faithfulness + metrics.AnswerRelevancy + metrics.ContextPrecision
	if groundTruth != "" {
		total += metrics.ContextRecall
		count = 4
	}
	metricsAvg := total / float64(count)

	result.Metrics = metrics
	result.AvgScore = math.Round(metricsAvg*1000) / 1000
	result.Duration = time.Since(start).Round(time.Millisecond).String()
	result.CreatedAt = time.Now()

	e.mu.Lock()
	result.ID = e.nextID
	e.nextID++
	e.results[result.ID] = result
	e.mu.Unlock()

	if e.repo != nil {
		e.saveToDB(result)
	}

	return result, nil
}

func (e *RAGASEvaluator) saveToDB(result *RAGASEvalResult) {
	contextsJSON := "[]"
	if len(result.Contexts) > 0 {
		if b, err := json.Marshal(result.Contexts); err == nil {
			contextsJSON = string(b)
		}
	}

	record := &knowledgemodel.AiRagasEval{
		Question:         result.Question,
		Answer:           result.Answer,
		ExpectedAnswer:   result.ExpectedAnswer,
		Contexts:         contextsJSON,
		GroundTruth:      result.GroundTruth,
		Faithfulness:     result.Metrics.Faithfulness,
		AnswerRelevancy:  result.Metrics.AnswerRelevancy,
		ContextPrecision: result.Metrics.ContextPrecision,
		ContextRecall:    result.Metrics.ContextRecall,
		AvgScore:         result.AvgScore,
		Duration:         result.Duration,
		CreatedAt:        result.CreatedAt,
	}

	if err := e.repo.Create(record); err != nil {
		logx.Infof("[RAGAS] Failed to persist evaluation result: %v", err)
	}
}

func modelToResult(m *knowledgemodel.AiRagasEval) *RAGASEvalResult {
	var contexts []string
	if m.Contexts != "" {
		if err := json.Unmarshal([]byte(m.Contexts), &contexts); err != nil {
			contexts = nil
		}
	}

	return &RAGASEvalResult{
		ID:             m.ID,
		Question:       m.Question,
		Answer:         m.Answer,
		ExpectedAnswer: m.ExpectedAnswer,
		Contexts:       contexts,
		GroundTruth:    m.GroundTruth,
		Metrics: RAGASMetrics{
			Faithfulness:     m.Faithfulness,
			AnswerRelevancy:  m.AnswerRelevancy,
			ContextPrecision: m.ContextPrecision,
			ContextRecall:    m.ContextRecall,
		},
		AvgScore:  m.AvgScore,
		Duration:  m.Duration,
		CreatedAt: m.CreatedAt,
	}
}

func (e *RAGASEvaluator) callLLM(ctx context.Context, prompt string) (string, error) {
	msgs := []provider.Message{provider.NewTextMessage("user", prompt)}
	resp, err := e.provider.Chat(ctx, "", msgs)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

func (e *RAGASEvaluator) computeFaithfulness(ctx context.Context, answer string, contexts []string) (float64, error) {
	if len(contexts) == 0 || answer == "" {
		return 0, nil
	}

	claimsPrompt := fmt.Sprintf(`Given the answer below, extract each factual claim as a separate statement. Return a JSON array of strings.

Answer: %s

Return only the JSON array, e.g. ["claim1", "claim2"]`, answer)

	content, err := e.callLLM(ctx, claimsPrompt)
	if err != nil {
		return 0, err
	}

	var claims []string
	trimmed := utils.ExtractJSONArray(content)
	if err := json.Unmarshal([]byte(trimmed), &claims); err != nil || len(claims) == 0 {
		claims = []string{answer}
	}

	contextStr := strings.Join(contexts, "\n\n")
	supported := 0
	for _, claim := range claims {
		verifyPrompt := fmt.Sprintf(`Given this context, determine if the claim is supported. Answer only "yes" or "no".

Context: %s

Claim: %s`, utils.TruncateRunes(contextStr, 3000), claim)

		vContent, err := e.callLLM(ctx, verifyPrompt)
		if err != nil {
			continue
		}
		if strings.Contains(strings.ToLower(vContent), "yes") {
			supported++
		}
	}

	return float64(supported) / float64(len(claims)), nil
}

func (e *RAGASEvaluator) computeAnswerRelevancy(ctx context.Context, question, answer string) (float64, error) {
	if question == "" || answer == "" {
		return 0, nil
	}

	genPrompt := fmt.Sprintf(`Given this question, generate 3 possible questions that would lead to the given answer. Return a JSON array of strings.

Question: %s
Answer: %s

Return only the JSON array, e.g. ["q1", "q2", "q3"]`, question, utils.TruncateRunes(answer, 1000))

	content, err := e.callLLM(ctx, genPrompt)
	if err != nil {
		return 0, err
	}

	var genQuestions []string
	trimmed := utils.ExtractJSONArray(content)
	if err := json.Unmarshal([]byte(trimmed), &genQuestions); err != nil || len(genQuestions) == 0 {
		return 0.5, nil
	}

	score := 0.0
	for _, gq := range genQuestions {
		sim := wordSimilarity(strings.ToLower(question), strings.ToLower(gq))
		score += sim
	}

	return score / float64(len(genQuestions)), nil
}

func (e *RAGASEvaluator) computeContextPrecision(ctx context.Context, question string, contexts []string) (float64, error) {
	if len(contexts) == 0 || question == "" {
		return 0, nil
	}

	relevant := 0
	weightedSum := 0.0

	for i, c := range contexts {
		verifyPrompt := fmt.Sprintf(`Is this context relevant to answering the question? Answer only "yes" or "no".

Question: %s
Context: %s`, question, utils.TruncateRunes(c, 1000))

		content, err := e.callLLM(ctx, verifyPrompt)
		if err != nil {
			continue
		}

		isRelevant := strings.Contains(strings.ToLower(content), "yes")
		if isRelevant {
			relevant++
			weightedSum += float64(relevant) / float64(i+1)
		}
	}

	if relevant == 0 {
		return 0, nil
	}
	return weightedSum / float64(relevant), nil
}

func (e *RAGASEvaluator) computeContextRecall(ctx context.Context, groundTruth string, contexts []string) (float64, error) {
	if len(contexts) == 0 || groundTruth == "" {
		return 0, nil
	}

	contextStr := strings.Join(contexts, "\n\n")
	verifyPrompt := fmt.Sprintf(`For each fact in the ground truth, determine if it can be inferred from the context. 
Return a JSON array of objects with "fact" and "found" (boolean) fields.

Ground Truth: %s
Context: %s

Return only the JSON array.`, utils.TruncateRunes(groundTruth, 1000), utils.TruncateRunes(contextStr, 3000))

	content, err := e.callLLM(ctx, verifyPrompt)
	if err != nil {
		return 0, err
	}

	type factCheck struct {
		Fact  string `json:"fact"`
		Found bool   `json:"found"`
	}
	var checks []factCheck
	trimmed := utils.ExtractJSONArray(content)
	if err := json.Unmarshal([]byte(trimmed), &checks); err != nil || len(checks) == 0 {
		return 0.5, nil
	}

	found := 0
	for _, c := range checks {
		if c.Found {
			found++
		}
	}

	return float64(found) / float64(len(checks)), nil
}

func (e *RAGASEvaluator) GetResults(limit, offset int) []*RAGASEvalResult {
	e.mu.RLock()
	defer e.mu.RUnlock()

	all := make([]*RAGASEvalResult, 0, len(e.results))
	for _, r := range e.results {
		all = append(all, r)
	}

	if offset >= len(all) {
		return nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return all[offset:end]
}

func (e *RAGASEvaluator) GetResultByID(id int) (*RAGASEvalResult, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	r, ok := e.results[id]
	return r, ok
}

// DeleteResult 删除一条评估结果。落库后 id 是 aw_ai_ragas_eval 主键，
// 必须同时删库，否则列表重启即恢复、删了还在。
func (e *RAGASEvaluator) DeleteResult(id int) error {
	e.mu.Lock()
	delete(e.results, id)
	e.mu.Unlock()

	if e.repo != nil {
		return e.repo.DeleteByID(id)
	}
	return nil
}

func (e *RAGASEvaluator) ClearResults() {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.results = make(map[int]*RAGASEvalResult)
}

func (e *RAGASEvaluator) Stats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()

	total := len(e.results)
	if total == 0 {
		return map[string]interface{}{"total": 0}
	}

	avgFaith := 0.0
	avgAR := 0.0
	avgCP := 0.0
	avgCR := 0.0
	crCount := 0

	for _, r := range e.results {
		avgFaith += r.Metrics.Faithfulness
		avgAR += r.Metrics.AnswerRelevancy
		avgCP += r.Metrics.ContextPrecision
		if r.GroundTruth != "" {
			avgCR += r.Metrics.ContextRecall
			crCount++
		}
	}

	stats := map[string]interface{}{
		"total":                 total,
		"avg_faithfulness":      math.Round(avgFaith/float64(total)*100) / 100,
		"avg_answer_relevancy":  math.Round(avgAR/float64(total)*100) / 100,
		"avg_context_precision": math.Round(avgCP/float64(total)*100) / 100,
	}
	if crCount > 0 {
		stats["avg_context_recall"] = math.Round(avgCR/float64(crCount)*100) / 100
	}
	return stats
}

func (e *RAGASEvaluator) GetStats() map[string]interface{} {
	if e.repo == nil {
		return map[string]interface{}{
			"total":        0,
			"source":       "memory",
			"db_available": false,
		}
	}

	dbStats, err := e.repo.Stats()
	if err != nil {
		logx.Infof("[RAGAS] Failed to get stats from DB: %v", err)
		return e.Stats()
	}

	stats := map[string]interface{}{
		"total":                 dbStats.Total,
		"avg_faithfulness":      math.Round(dbStats.AvgFaithfulness*100) / 100,
		"avg_answer_relevancy":  math.Round(dbStats.AvgAnswerRelevancy*100) / 100,
		"avg_context_precision": math.Round(dbStats.AvgContextPrecision*100) / 100,
		"source":                "db",
	}
	if dbStats.AvgContextRecall > 0 {
		stats["avg_context_recall"] = math.Round(dbStats.AvgContextRecall*100) / 100
	}
	return stats
}

func (e *RAGASEvaluator) GetHistory(limit, offset int) ([]*RAGASEvalResult, int64, error) {
	if e.repo == nil {
		results := e.GetResults(limit, offset)
		return results, int64(len(e.results)), nil
	}

	dbResults, total, err := e.repo.FindWithOffset(limit, offset)
	if err != nil {
		return nil, 0, err
	}

	converted := make([]*RAGASEvalResult, 0, len(dbResults))
	for _, r := range dbResults {
		converted = append(converted, modelToResult(r))
	}
	return converted, total, nil
}

func wordSimilarity(a, b string) float64 {
	if a == b {
		return 1.0
	}
	aWords := strings.Fields(a)
	bWords := strings.Fields(b)
	if len(aWords) == 0 || len(bWords) == 0 {
		return 0.0
	}
	bWordSet := make(map[string]bool)
	for _, w := range bWords {
		bWordSet[w] = true
	}
	matches := 0
	for _, w := range aWords {
		if bWordSet[w] {
			matches++
		}
	}
	return float64(matches) / float64(len(aWords))
}
