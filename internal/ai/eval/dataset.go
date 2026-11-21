package eval

import (
	"time"

	evalmodel "github.com/listen808/citebase/internal/model/eval"
)

// EvalQuestion 评估问题
type EvalQuestion struct {
	Question        string   `json:"question"`
	ExpectedAnswer  string   `json:"expected_answer"`
	ExpectedContext []string `json:"expected_context"`
	Difficulty      string   `json:"difficulty"` // easy/medium/hard
	// W4: 四段式标注
	ExpectedIntent string `json:"expected_intent,omitempty"` // 期望路由意图（factual/conceptual/procedural/comparison/creative）
	ExpectedDocIDs []int  `json:"expected_doc_ids,omitempty"` // 期望命中的文档 ID 列表
	// W4: RGB 噪声标注
	NoiseType string `json:"noise_type,omitempty"` // none/noise/contradiction/reasoning
}

// RAGASMetrics RAGAS 评估指标
type RAGASMetrics struct {
	Faithfulness      float64 `json:"faithfulness"`
	AnswerRelevance   float64 `json:"answer_relevance"`
	ContextPrecision  float64 `json:"context_precision"`
	ContextRecall     float64 `json:"context_recall"`
	AnswerCorrectness float64 `json:"answer_correctness"`
	// RetrievalHitRate 检索层命中率：期望上下文有多少被检索到了（纯 overlap，零 LLM）。
	// 与 ContextRecall 同源，但单独归到「检索层」用于三层归因——「答案错了」先定位是哪层坏的。
	RetrievalHitRate float64 `json:"retrieval_hit_rate"`
	// W4: 四段式过程指标
	RoutingAccuracy float64 `json:"routing_accuracy"` // 路由层：查询类型分类是否匹配期望意图
	RetrievalHitAtK float64 `json:"retrieval_hit_at_k"` // 检索层：Top-K 是否包含期望文档
	RerankNDCG     float64 `json:"rerank_ndcg"`  // 重排层：NDCG@K
	NoiseRobustness float64 `json:"noise_robustness"` // 生成层：噪声鲁棒性（RGB 噪声下答案质量保持比）
}

// LayerScores 把指标按 RAG 三层聚合，供前端归因展示：
// 答案错了 → 先看检索层（没找到资料）、再看增强层（资料没进 prompt）、最后看生成层（有资料却答错）。
type LayerScores struct {
	Retrieval  float64 `json:"retrieval"`  // 检索层：RetrievalHitRate
	Enrich     float64 `json:"enrich"`     // 增强层：ContextPrecision ∪ ContextRecall
	Generation float64 `json:"generation"` // 生成层：Faithfulness / AnswerRelevance / AnswerCorrectness
	// W4: 四段式过程指标
	Routing  float64 `json:"routing"`  // 路由层：RoutingAccuracy
	Rerank   float64 `json:"rerank"`   // 重排层：RerankNDCG
	NoiseRes float64 `json:"noise_res"` // 噪声鲁棒性：NoiseRobustness
}

// LayerScoresFrom 从一组指标计算三层聚合分（各层内取有值指标的平均）。
func LayerScoresFrom(m RAGASMetrics) LayerScores {
	avg := func(vals ...float64) float64 {
		if len(vals) == 0 {
			return 0
		}
		sum := 0.0
		for _, v := range vals {
			sum += v
		}
		return round4(sum / float64(len(vals)))
	}
	return LayerScores{
		Retrieval:  m.RetrievalHitRate,
		Enrich:     avg(m.ContextPrecision, m.ContextRecall),
		Generation: avg(m.Faithfulness, m.AnswerRelevance, m.AnswerCorrectness),
		Routing:    m.RoutingAccuracy,
		Rerank:     m.RerankNDCG,
		NoiseRes:   m.NoiseRobustness,
	}
}

// QuestionEval 单题评估详情
type QuestionEval struct {
	Question         string       `json:"question"`
	GeneratedAnswer  string       `json:"generated_answer"`
	RetrievedContext []string     `json:"retrieved_context"`
	ExpectedAnswer   string       `json:"expected_answer"`
	ExpectedContext  []string     `json:"expected_context"`
	Metrics          RAGASMetrics `json:"metrics"`
	// W4: 四段式过程信息
	DetectedIntent string `json:"detected_intent,omitempty"` // 实际检测到的查询类型
	ExpectedIntent string `json:"expected_intent,omitempty"` // 期望的查询类型
	NoiseType      string `json:"noise_type,omitempty"`      // RGB 噪声类型
}

// DatasetWithQuestions 带问题列表的数据集（用于 API 传输）
type DatasetWithQuestions struct {
	ID        int64          `json:"id"`
	Name      string         `json:"name"`
	SpaceID   int            `json:"space_id"`
	Questions []EvalQuestion `json:"questions"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
}

// ResultWithDetails 带详情的评估结果（用于 API 传输）
type ResultWithDetails struct {
	ID           int64          `json:"id"`
	DatasetID    int64          `json:"dataset_id"`
	DatasetName  string         `json:"dataset_name"`
	SpaceID      int            `json:"space_id"`
	OverallScore float64        `json:"overall_score"`
	Metrics      *RAGASMetrics  `json:"metrics,omitempty"`
	Details      []QuestionEval `json:"details,omitempty"`
	Duration     string         `json:"duration"`
	Status       string         `json:"status"`
	CreatedAt    time.Time      `json:"created_at"`
}

// ToModel 转换为数据库模型（保存时调用）
func (ds *DatasetWithQuestions) ToModel() *evalmodel.EvalDataset {
	return &evalmodel.EvalDataset{
		ID:        ds.ID,
		Name:      ds.Name,
		SpaceID:   ds.SpaceID,
		CreatedAt: ds.CreatedAt,
		UpdatedAt: ds.UpdatedAt,
	}
}
