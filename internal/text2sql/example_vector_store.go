package text2sql

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/embedding"
	"github.com/listen808/citebase/internal/ai/shared"
	modelai "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/utils"
)

// SQLExampleVectorStore SQL 示例向量存储
type SQLExampleVectorStore struct {
	embedder    embedding.Embedder
	schemaVS    *embedding.SchemaVectorStore
	exampleRepo *airepo.SQLExampleRepo
}

// NewSQLExampleVectorStore 创建 SQL 示例向量存储
func NewSQLExampleVectorStore(emb embedding.Embedder, schemaVS *embedding.SchemaVectorStore, exampleRepo *airepo.SQLExampleRepo) *SQLExampleVectorStore {
	return &SQLExampleVectorStore{
		embedder:    emb,
		schemaVS:    schemaVS,
		exampleRepo: exampleRepo,
	}
}

// SQLExampleSearchResult SQL 示例检索结果
type SQLExampleSearchResult struct {
	ExampleID int     `json:"example_id"`
	Question  string  `json:"question"`
	SQL       string  `json:"sql"`
	Score     float64 `json:"score"`
	Source    string  `json:"source"` // "keyword" or "vector"
}

// RetrieveRelevantExamples 检索与问题相关的 SQL 示例
// 融合关键词匹配和向量语义检索，使用 RRF 融合排序
func (s *SQLExampleVectorStore) RetrieveRelevantExamples(ctx context.Context, question string, spaceID int, limit int) ([]SQLExampleSearchResult, error) {
	if limit <= 0 || limit > 10 {
		limit = 5
	}

	// 1. 关键词检索
	keywordResults := s.keywordRetrieve(question, spaceID, limit*2)

	// 2. 向量语义检索
	vectorResults := s.vectorRetrieve(question, spaceID, limit*2)

	// 3. RRF 融合
	merged := s.rrfMerge(keywordResults, vectorResults, limit)

	logx.Infof("[SQLExampleRAG] Retrieved %d relevant examples (keyword=%d, vector=%d) for: %.50s",
		len(merged), len(keywordResults), len(vectorResults), question)

	return merged, nil
}

// keywordRetrieve 关键词检索
func (s *SQLExampleVectorStore) keywordRetrieve(question string, spaceID int, limit int) []SQLExampleSearchResult {
	if s.exampleRepo == nil {
		return nil
	}

	examples, err := s.exampleRepo.FindVerifiedBySpace(spaceID, limit)
	if err != nil || len(examples) == 0 {
		return nil
	}

	keywords := extractKeywords(question)
	questionLower := strings.ToLower(question)

	type scoredExample struct {
		example *modelai.SQLExample
		score   float64
	}
	var scored []scoredExample

	for _, ex := range examples {
		score := 0.0
		exLower := strings.ToLower(ex.Question)

		// 关键词匹配
		for _, kw := range keywords {
			if strings.Contains(exLower, kw) {
				score += 2.0
			}
			if strings.Contains(strings.ToLower(ex.SQL), kw) {
				score += 1.0
			}
		}

		// 问题相似度
		if strings.Contains(exLower, questionLower) || strings.Contains(questionLower, exLower) {
			score += 5.0
		}

		if score > 0 {
			scored = append(scored, scoredExample{example: ex, score: score})
		}
	}

	if len(scored) == 0 {
		return nil
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	var results []SQLExampleSearchResult
	for _, s := range scored {
		results = append(results, SQLExampleSearchResult{
			ExampleID: s.example.ID,
			Question:  s.example.Question,
			SQL:       s.example.SQL,
			Score:     s.score,
			Source:    "keyword",
		})
	}

	return results
}

// vectorRetrieve 向量语义检索
func (s *SQLExampleVectorStore) vectorRetrieve(question string, spaceID int, limit int) []SQLExampleSearchResult {
	if s.embedder == nil || s.exampleRepo == nil {
		return nil
	}

	// 获取该空间的所有已验证示例
	examples, err := s.exampleRepo.FindVerifiedBySpace(spaceID, 100)
	if err != nil || len(examples) == 0 {
		return nil
	}

	// 对每个示例计算语义相似度
	type scoredExample struct {
		example *modelai.SQLExample
		score   float64
	}
	var scored []scoredExample

	questionVec, err := s.embedder.GetEmbedding(question)
	if err != nil {
		logx.Infof("[SQLExampleRAG] Failed to embed question: %v", err)
		return nil
	}

	// P3：批量获取 embedding，避免循环内逐个调用 LLM API（N+1 问题）
	questions := make([]string, len(examples))
	for i, ex := range examples {
		questions[i] = ex.Question
	}
	exVecs, err := s.embedder.GetEmbeddings(questions)
	if err != nil {
		logx.Infof("[SQLExampleRAG] Failed to batch embed examples: %v", err)
		return nil
	}

	for i, ex := range examples {
		if i >= len(exVecs) || exVecs[i] == nil {
			continue
		}
		similarity := utils.CosineSimilarity(questionVec, exVecs[i])
		if similarity > 0.5 { // 阈值过滤
			scored = append(scored, scoredExample{example: ex, score: similarity})
		}
	}

	if len(scored) == 0 {
		return nil
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	var results []SQLExampleSearchResult
	for _, s := range scored {
		results = append(results, SQLExampleSearchResult{
			ExampleID: s.example.ID,
			Question:  s.example.Question,
			SQL:       s.example.SQL,
			Score:     s.score,
			Source:    "vector",
		})
	}

	return results
}

// rrfMerge RRF 结果融合（k=60）
func (s *SQLExampleVectorStore) rrfMerge(listA, listB []SQLExampleSearchResult, limit int) []SQLExampleSearchResult {
	itemsA := make([]shared.RRFItem, len(listA))
	for i, item := range listA {
		itemsA[i] = shared.RRFItem{Key: fmt.Sprintf("%d", item.ExampleID), Score: item.Score, Data: item}
	}
	itemsB := make([]shared.RRFItem, len(listB))
	for i, item := range listB {
		itemsB[i] = shared.RRFItem{Key: fmt.Sprintf("%d", item.ExampleID), Score: item.Score, Data: item}
	}

	merged := shared.RRFMerge([][]shared.RRFItem{itemsA, itemsB}, 60.0, limit)
	result := make([]SQLExampleSearchResult, len(merged))
	for i, item := range merged {
		result[i] = item.Data.(SQLExampleSearchResult)
	}
	return result
}

// FormatExamplesForPrompt 将检索到的示例格式化为 Prompt 文本
func FormatExamplesForPrompt(examples []SQLExampleSearchResult) string {
	if len(examples) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Few-Shot 示例（学习正确的字段映射方式）\n\n")

	for i, ex := range examples {
		sb.WriteString(fmt.Sprintf("### 示例 %d\n", i+1))
		sb.WriteString(fmt.Sprintf("**问题**: %s\n\n", ex.Question))
		sb.WriteString("**SQL**:\n```sql\n")
		sb.WriteString(ex.SQL)
		sb.WriteString("\n```\n\n")
	}

	return sb.String()
}

// SaveExampleWithVector 保存 SQL 示例并生成向量
func (s *SQLExampleVectorStore) SaveExampleWithVector(ctx context.Context, question, sql string, spaceID int, userID int) error {
	if s.exampleRepo == nil {
		return fmt.Errorf("example repo not configured")
	}

	example := &modelai.SQLExample{
		SpaceID:   spaceID,
		Question:  question,
		SQL:       sql,
		Verified:  0,
		CreatedBy: userID,
	}

	if err := s.exampleRepo.Create(example); err != nil {
		return fmt.Errorf("failed to create example: %w", err)
	}

	// 生成向量（异步，不阻塞）
	if s.embedder != nil {
		go func() {
			_, err := s.embedder.GetEmbedding(question)
			if err != nil {
				logx.Infof("[SQLExampleRAG] Failed to embed example question: %v", err)
			}
		}()
	}

	return nil
}

// BatchIndexExamples 批量索引 SQL 示例（生成向量）
func (s *SQLExampleVectorStore) BatchIndexExamples(ctx context.Context, spaceID int) (int, error) {
	if s.embedder == nil || s.exampleRepo == nil {
		return 0, fmt.Errorf("embedder or example repo not configured")
	}

	examples, err := s.exampleRepo.FindVerifiedBySpace(spaceID, 1000)
	if err != nil {
		return 0, fmt.Errorf("failed to load examples: %w", err)
	}

	indexed := 0
	for _, ex := range examples {
		_, err := s.embedder.GetEmbedding(ex.Question)
		if err != nil {
			logx.Infof("[SQLExampleRAG] Failed to embed example %d: %v", ex.ID, err)
			continue
		}
		indexed++
	}

	logx.Infof("[SQLExampleRAG] Indexed %d examples for space %d", indexed, spaceID)
	return indexed, nil
}

// MarshalJSON 自定义 JSON 序列化
func (r SQLExampleSearchResult) MarshalJSON() ([]byte, error) {
	type Alias SQLExampleSearchResult
	return json.Marshal(&struct {
		Alias
	}{
		Alias: (Alias)(r),
	})
}
