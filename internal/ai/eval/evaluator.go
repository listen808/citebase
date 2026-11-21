package eval

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"math"
	"strings"
	"time"
	"unicode"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/ai/rag"
	evalmodel "github.com/listen808/citebase/internal/model/eval"
	"github.com/listen808/citebase/internal/utils"
)

// Evaluator RAG 评估器
type Evaluator struct {
	provider  provider.Provider
	retriever *rag.Retriever
	repo      *DatasetRepository
}

// NewEvaluator 创建评估器
func NewEvaluator(p provider.Provider, r *rag.Retriever, repo *DatasetRepository) *Evaluator {
	return &Evaluator{
		provider:  p,
		retriever: r,
		repo:      repo,
	}
}

// Evaluate 运行评估，计算 RAGAS 指标
func (e *Evaluator) Evaluate(ctx context.Context, questions []EvalQuestion, spaceID int, datasetID int64, datasetName string) (*ResultWithDetails, error) {
	if e.provider == nil {
		return nil, fmt.Errorf("eval: no LLM provider configured")
	}
	if len(questions) == 0 {
		return nil, fmt.Errorf("eval: no questions to evaluate")
	}

	start := time.Now()
	result := &ResultWithDetails{
		DatasetID:   datasetID,
		DatasetName: datasetName,
		SpaceID:     spaceID,
		Status:      "completed",
	}

	details := make([]QuestionEval, 0, len(questions))
	aggMetrics := RAGASMetrics{}

	for _, q := range questions {
		qEval, err := e.evaluateQuestion(ctx, q, spaceID)
		if err != nil {
			logx.Infof("[eval] question eval error: %v", err)
			qEval = &QuestionEval{
				Question: q.Question,
				Metrics:  RAGASMetrics{},
			}
		}
		details = append(details, *qEval)

		aggMetrics.Faithfulness += qEval.Metrics.Faithfulness
		aggMetrics.AnswerRelevance += qEval.Metrics.AnswerRelevance
		aggMetrics.ContextPrecision += qEval.Metrics.ContextPrecision
		aggMetrics.ContextRecall += qEval.Metrics.ContextRecall
		aggMetrics.AnswerCorrectness += qEval.Metrics.AnswerCorrectness
		aggMetrics.RetrievalHitRate += qEval.Metrics.RetrievalHitRate
		// W4: 四段式
		aggMetrics.RoutingAccuracy += qEval.Metrics.RoutingAccuracy
		aggMetrics.RetrievalHitAtK += qEval.Metrics.RetrievalHitAtK
		aggMetrics.RerankNDCG += qEval.Metrics.RerankNDCG
		aggMetrics.NoiseRobustness += qEval.Metrics.NoiseRobustness
	}

	n := float64(len(details))
	if n > 0 {
		aggMetrics.Faithfulness = round4(aggMetrics.Faithfulness / n)
		aggMetrics.AnswerRelevance = round4(aggMetrics.AnswerRelevance / n)
		aggMetrics.ContextPrecision = round4(aggMetrics.ContextPrecision / n)
		aggMetrics.ContextRecall = round4(aggMetrics.ContextRecall / n)
		aggMetrics.AnswerCorrectness = round4(aggMetrics.AnswerCorrectness / n)
		aggMetrics.RetrievalHitRate = round4(aggMetrics.RetrievalHitRate / n)
		// W4: 四段式
		aggMetrics.RoutingAccuracy = round4(aggMetrics.RoutingAccuracy / n)
		aggMetrics.RetrievalHitAtK = round4(aggMetrics.RetrievalHitAtK / n)
		aggMetrics.RerankNDCG = round4(aggMetrics.RerankNDCG / n)
		aggMetrics.NoiseRobustness = round4(aggMetrics.NoiseRobustness / n)
	}

	overall := (aggMetrics.Faithfulness + aggMetrics.AnswerRelevance + aggMetrics.ContextPrecision + aggMetrics.ContextRecall + aggMetrics.AnswerCorrectness) / 5.0
	result.OverallScore = round4(overall)
	result.Metrics = &aggMetrics
	result.Details = details
	result.Duration = time.Since(start).Round(time.Millisecond).String()

	// Persist to DB
	if e.repo != nil {
		dbResult := e.toDBModel(result)
		if err := e.repo.SaveResult(dbResult); err != nil {
			logx.Infof("[eval] failed to save result: %v", err)
		} else {
			result.ID = dbResult.ID
		}
	}

	return result, nil
}

// toDBModel 将结果转换为数据库模型
func (e *Evaluator) toDBModel(result *ResultWithDetails) *evalmodel.EvalResult {
	dbResult := &evalmodel.EvalResult{
		DatasetID:    result.DatasetID,
		DatasetName:  result.DatasetName,
		SpaceID:      result.SpaceID,
		OverallScore: result.OverallScore,
		Duration:     result.Duration,
		Status:       result.Status,
	}
	if result.Metrics != nil {
		if b, err := json.Marshal(result.Metrics); err == nil {
			dbResult.MetricsJSON = string(b)
		}
	}
	if result.Details != nil {
		if b, err := json.Marshal(result.Details); err == nil {
			dbResult.DetailsJSON = string(b)
		}
	}
	return dbResult
}

// evaluateQuestion 评估单个问题
func (e *Evaluator) evaluateQuestion(ctx context.Context, q EvalQuestion, spaceID int) (*QuestionEval, error) {
	qEval := &QuestionEval{
		Question:        q.Question,
		ExpectedAnswer:  q.ExpectedAnswer,
		ExpectedContext: q.ExpectedContext,
	}

	// Step 1: Retrieve context using RAG retriever
	var retrievedContexts []string
	if e.retriever != nil {
		sources, _ := e.retriever.RetrieveContextWithRAG(ctx, q.Question, spaceID, 5)
		for _, s := range sources {
			retrievedContexts = append(retrievedContexts, s.Snippet)
		}
	}
	qEval.RetrievedContext = retrievedContexts

	// Step 2: Generate answer using RAG
	var generatedAnswer string
	if e.retriever != nil {
		answer, err := e.retriever.Ask(ctx, q.Question, spaceID, 5)
		if err == nil && answer != nil {
			generatedAnswer = answer.Answer
		}
	}
	qEval.GeneratedAnswer = generatedAnswer

	// Step 3: Compute metrics
	metrics := RAGASMetrics{}

	// Faithfulness
	faith, err := e.computeFaithfulness(ctx, generatedAnswer, retrievedContexts)
	if err != nil {
		logx.Infof("[eval] faithfulness error: %v", err)
	} else {
		metrics.Faithfulness = faith
	}

	// Answer Relevance
	ar, err := e.computeAnswerRelevance(ctx, q.Question, generatedAnswer)
	if err != nil {
		logx.Infof("[eval] answer relevance error: %v", err)
	} else {
		metrics.AnswerRelevance = ar
	}

	// Context Precision
	cp, err := e.computeContextPrecision(ctx, retrievedContexts, q.ExpectedContext)
	if err != nil {
		logx.Infof("[eval] context precision error: %v", err)
	} else {
		metrics.ContextPrecision = cp
	}

	// Context Recall
	cr, err := e.computeContextRecall(ctx, q.ExpectedContext, retrievedContexts)
	if err != nil {
		logx.Infof("[eval] context recall error: %v", err)
	} else {
		metrics.ContextRecall = cr
	}

	// Answer Correctness
	ac, err := e.computeAnswerCorrectness(ctx, q.ExpectedAnswer, generatedAnswer)
	if err != nil {
		logx.Infof("[eval] answer correctness error: %v", err)
	} else {
		metrics.AnswerCorrectness = ac
	}

	// Retrieval Hit Rate 检索层命中率（零 LLM，与 ContextRecall 同源但单独归检索层）
	if len(q.ExpectedContext) > 0 {
		metrics.RetrievalHitRate = e.retrievalHitRate(q.ExpectedContext, retrievedContexts)
	}

	// ---- W4: 四段式新增指标 ----

	// 1. RoutingAccuracy 路由准确率：实际分类 vs 期望意图
	qEval.ExpectedIntent = q.ExpectedIntent
	if q.ExpectedIntent != "" && e.retriever != nil {
		detected := e.retriever.ClassifyQuery(ctx, q.Question)
		qEval.DetectedIntent = string(detected)
		if strings.EqualFold(string(detected), q.ExpectedIntent) {
			metrics.RoutingAccuracy = 1.0
		} else {
			metrics.RoutingAccuracy = 0.0
		}
	}

	// 2. RetrievalHitAtK 检索层 Top-K 命中率：期望文档在检索来源中的占比
	if len(q.ExpectedDocIDs) > 0 && e.retriever != nil {
		sources, _ := e.retriever.RetrieveContextWithRAG(ctx, q.Question, spaceID, 10)
		docIDSet := make(map[int]bool, len(sources))
		for _, s := range sources {
			docIDSet[s.DocumentID] = true
		}
		hit := 0
		for _, id := range q.ExpectedDocIDs {
			if docIDSet[id] {
				hit++
			}
		}
		metrics.RetrievalHitAtK = round4(float64(hit) / float64(len(q.ExpectedDocIDs)))
	}

	// 3. RerankNDCG 重排质量：期望文档在检索结果中的排序质量
	if len(q.ExpectedDocIDs) > 0 && e.retriever != nil {
		sources, _ := e.retriever.RetrieveContextWithRAG(ctx, q.Question, spaceID, 10)
		if len(sources) > 0 {
			metrics.RerankNDCG = e.computeNDCG(q.ExpectedDocIDs, sources)
		}
	}

	// 4. NoiseRobustness 噪声鲁棒性：噪声问题下的忠实度保持率
	qEval.NoiseType = q.NoiseType
	if q.NoiseType != "" {
		// 以 Faithfulness 作为噪声鲁棒性的代理指标：
		// 噪声问题下，回答仍应忠实于检索到的上下文。
		metrics.NoiseRobustness = metrics.Faithfulness
	}

	qEval.Metrics = metrics
	return qEval, nil
}

// computeNDCG 计算 NDCG@K：期望文档在检索结果中的排序质量。
// 对每个检索位置，若文档在 ExpectedDocIDs 中则 relevance=1，否则 0。
func (e *Evaluator) computeNDCG(expectedDocIDs []int, sources []rag.Source) float64 {
	expectedSet := make(map[int]bool, len(expectedDocIDs))
	for _, id := range expectedDocIDs {
		expectedSet[id] = true
	}

	// DCG: sum of (2^rel - 1) / log2(i+1) for i from 0 to len-1
	var dcg float64
	for i, s := range sources {
		rel := 0.0
		if expectedSet[s.DocumentID] {
			rel = 1.0
		}
		if rel > 0 {
			dcg += (math.Pow(2, rel) - 1) / math.Log2(float64(i+1)+1)
		}
	}

	// IDCG: all expected docs at the top
	nRelevant := 0
	for _, s := range sources {
		if expectedSet[s.DocumentID] {
			nRelevant++
		}
	}
	if nRelevant == 0 {
		return 0
	}
	var idcg float64
	for i := 0; i < nRelevant; i++ {
		idcg += (math.Pow(2, 1.0) - 1) / math.Log2(float64(i+1)+1)
	}

	if idcg == 0 {
		return 0
	}
	return round4(dcg / idcg)
}

// retrievalHitRate 期望上下文中有多少被检索到（纯 overlap，供三层归因的检索层）。
func (e *Evaluator) retrievalHitRate(expected, retrieved []string) float64 {
	if len(expected) == 0 || len(retrieved) == 0 {
		return 0
	}
	retrievedStr := strings.ToLower(strings.Join(retrieved, " "))
	hit := 0
	for _, exp := range expected {
		if keywordOverlap(retrievedStr, strings.ToLower(exp)) > 0.3 {
			hit++
		}
	}
	return round4(float64(hit) / float64(len(expected)))
}

// callLLM 调用 LLM
func (e *Evaluator) callLLM(ctx context.Context, prompt string) (string, error) {
	msgs := []provider.Message{provider.NewTextMessage("user", prompt)}
	resp, err := e.provider.Chat(ctx, "", msgs)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// computeFaithfulness 忠实度：answer 中的声明是否都能从 context 推导出来
func (e *Evaluator) computeFaithfulness(ctx context.Context, answer string, contexts []string) (float64, error) {
	if answer == "" || len(contexts) == 0 {
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

// computeAnswerRelevance 回答相关性：answer 是否切题
func (e *Evaluator) computeAnswerRelevance(ctx context.Context, question, answer string) (float64, error) {
	if question == "" || answer == "" {
		return 0, nil
	}

	prompt := fmt.Sprintf(`Rate how well the answer addresses the question on a scale from 0 to 1.
0 means completely irrelevant, 1 means perfectly answers the question.

Question: %s
Answer: %s

Return only a number between 0 and 1.`, question, utils.TruncateRunes(answer, 1500))

	content, err := e.callLLM(ctx, prompt)
	if err != nil {
		return 0, err
	}

	score := parseScore(content)
	return score, nil
}

// computeContextPrecision 上下文精确度：retrieved context 与 expected context 的重叠度
func (e *Evaluator) computeContextPrecision(_ context.Context, retrieved, expected []string) (float64, error) {
	if len(retrieved) == 0 {
		return 0, nil
	}
	if len(expected) == 0 {
		// P0-3：旧实现返回 0.5 占位。没有期望上下文就是"无法评估"，按 0 保守计，
		// 否则缺失标注反而把指标抬高，评测就指向不了动作了。
		return 0, nil
	}

	totalRelevant := 0
	for _, ret := range retrieved {
		retLower := strings.ToLower(ret)
		for _, exp := range expected {
			if keywordOverlap(retLower, strings.ToLower(exp)) > 0.3 {
				totalRelevant++
				break
			}
		}
	}

	return float64(totalRelevant) / float64(len(retrieved)), nil
}

// computeContextRecall 上下文召回率：期望的上下文中有多少被检索到了
func (e *Evaluator) computeContextRecall(_ context.Context, expected, retrieved []string) (float64, error) {
	if len(expected) == 0 {
		// P0-3：旧实现返回 1.0 —— 没有期望上下文时召回率"满分"，这是虚高，
		// 会让整条评测看起来很好而实际没有任何标注支撑。改为保守的 0。
		return 0, nil
	}
	if len(retrieved) == 0 {
		return 0, nil
	}

	retrievedStr := strings.ToLower(strings.Join(retrieved, " "))
	hit := 0
	for _, exp := range expected {
		expLower := strings.ToLower(exp)
		if keywordOverlap(retrievedStr, expLower) > 0.3 {
			hit++
		}
	}

	return float64(hit) / float64(len(expected)), nil
}

// computeAnswerCorrectness 回答正确性：比较生成回答与期望回答
func (e *Evaluator) computeAnswerCorrectness(ctx context.Context, expected, generated string) (float64, error) {
	if expected == "" || generated == "" {
		return 0, nil
	}

	prompt := fmt.Sprintf(`Compare the expected answer and the generated answer. Rate their semantic similarity on a scale from 0 to 1.
0 means completely different, 1 means semantically identical.

Expected Answer: %s
Generated Answer: %s

Return only a number between 0 and 1.`, utils.TruncateRunes(expected, 1500), utils.TruncateRunes(generated, 1500))

	content, err := e.callLLM(ctx, prompt)
	if err != nil {
		return 0, err
	}

	score := parseScore(content)
	return score, nil
}

// parseScore 从 LLM 响应中解析分数
func parseScore(content string) float64 {
	content = strings.TrimSpace(content)
	var score float64
	if _, err := fmt.Sscanf(content, "%f", &score); err == nil {
		if score >= 0 && score <= 1 {
			return round4(score)
		}
	}
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if _, err := fmt.Sscanf(line, "%f", &score); err == nil {
			if score >= 0 && score <= 1 {
				return round4(score)
			}
		}
	}
	// P0-3：旧实现兜底 0.5。LLM 没给出可解析分数时按 0 保守计——
	// 一个凭空给的中位数会让"评估通过率"失去意义。
	return 0
}

// keywordOverlap 计算两段文本的关键词重叠度
func keywordOverlap(a, b string) float64 {
	aWords := tokenize(a)
	bWords := tokenize(b)
	if len(aWords) == 0 || len(bWords) == 0 {
		return 0
	}

	bSet := make(map[string]bool, len(bWords))
	for _, w := range bWords {
		bSet[w] = true
	}

	matches := 0
	for _, w := range aWords {
		if bSet[w] {
			matches++
		}
	}

	return float64(matches) / float64(len(aWords))
}

// tokenize 分词。
//
// P0-3：原实现只按标点与空格切词并过滤 len<2 的词——中文没有空格，整句会被当成
// 一个 token，keywordOverlap 因此恒为 0 或 1，ContextPrecision / ContextRecall
// 实际是伪算（指标数字好看但指向不了任何动作）。现在汉字段按 bigram 切分，
// 英文/数字仍按词切分。
func tokenize(s string) []string {
	replacer := strings.NewReplacer(
		",", " ", ".", " ", "!", " ", "?", " ",
		";", " ", ":", " ", "(", " ", ")", " ",
		"\"", " ", "'", " ", "\n", " ", "\t", " ",
	)
	cleaned := replacer.Replace(s)
	fields := strings.Fields(cleaned)
	result := make([]string, 0, len(fields)*2)
	for _, f := range fields {
		runes := []rune(f)
		for i := 0; i < len(runes); {
			if unicode.Is(unicode.Han, runes[i]) {
				j := i
				for j < len(runes) && unicode.Is(unicode.Han, runes[j]) {
					j++
				}
				seg := runes[i:j]
				for k := 0; k+1 < len(seg); k++ {
					result = append(result, string(seg[k:k+2]))
				}
				i = j
				continue
			}
			j := i
			for j < len(runes) && !unicode.Is(unicode.Han, runes[j]) {
				j++
			}
			if word := strings.TrimSpace(string(runes[i:j])); len([]rune(word)) >= 2 {
				result = append(result, word)
			}
			i = j
		}
	}
	return result
}

func round4(f float64) float64 {
	return math.Round(f*10000) / 10000
}
