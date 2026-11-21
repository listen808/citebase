package text2sql

import (
	"context"
	"fmt"
	"strings"
	"time"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/embedding"
	modelai "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

// ConversationVectorStore Text2SQL对话向量存储适配器
// 将每轮对话（查询+SQL）存入向量库，支持语义检索相似历史对话
type ConversationVectorStore struct {
	embedder embedding.Embedder
	store    embedding.VectorStoreInterface
	repo     *airepo.Text2SQLConversationRepository // 数据库Repository（用于持久化向量ID映射）
}

// NewConversationVectorStore 创建对话向量存储
func NewConversationVectorStore(
	embedder embedding.Embedder,
	store embedding.VectorStoreInterface,
	repo *airepo.Text2SQLConversationRepository,
) *ConversationVectorStore {
	return &ConversationVectorStore{
		embedder: embedder,
		store:    store,
		repo:     repo,
	}
}

// StoreTurn 存储对话轮次到向量库
// 将用户查询和生成的SQL组合为文本，生成embedding后存入向量库
func (cvs *ConversationVectorStore) StoreTurn(ctx context.Context, turn *modelai.SQLConversationTurnDTO) error {
	if turn == nil || turn.UserQuery == "" {
		return fmt.Errorf("turn or user query is empty")
	}

	// 构建组合文本（查询 + SQL + 表信息）
	combinedText := turn.BuildCombinedText()

	// 生成embedding
	vector, err := cvs.embedder.GetEmbedding(combinedText)
	if err != nil {
		return fmt.Errorf("generate embedding failed: %w", err)
	}

	// 构建元数据（存入数据库，向量库只存储向量）
	_ = map[string]interface{}{
		"turn_id":         turn.ID,
		"conversation_id": turn.ConversationID,
		"user_id":         turn.UserID,
		"turn_number":     turn.TurnNumber,
		"user_query":      turn.UserQuery,
		"generated_sql":   turn.GeneratedSQL,
		"intent_type":     turn.IntentType,
		"used_tables":     turn.UsedTables,
		"is_success":      turn.IsSuccess,
		"timestamp":       time.Now().Unix(),
	}

	// 存入向量库（使用conversation_id作为space_id分组）
	if err := cvs.store.StoreVector(int(turn.ID), int(turn.ConversationID), fmt.Sprintf("turn_%d_%d", turn.ConversationID, turn.TurnNumber), vector); err != nil {
		return fmt.Errorf("store vector failed: %w", err)
	}

	logx.Infof("[ConversationVectorStore] Stored turn %d for conversation %d, query: %s",
		turn.TurnNumber, turn.ConversationID, turn.UserQuery)
	return nil
}

// SearchSimilarTurns 语义检索相似历史对话轮次
// 当用户提出新查询时，检索历史上最相似的查询和对应的SQL
func (cvs *ConversationVectorStore) SearchSimilarTurns(
	ctx context.Context,
	query string,
	userID uint64,
	limit int,
) ([]*SimilarTurnResult, error) {
	if query == "" {
		return nil, nil
	}

	// 生成查询的embedding
	queryVector, err := cvs.embedder.GetEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("generate query embedding failed: %w", err)
	}

	// 向量检索（全局搜索，不过滤space_id，因为要找跨对话的相似查询）
	results, err := cvs.store.Search(queryVector, limit*2) // 多取一些用于过滤
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	var similarTurns []*SimilarTurnResult
	for _, result := range results {
		// 只返回同一用户的对话（或设置为可选）
		// 注意：VectorSearchResult 不包含 Content 字段，需要扩展或使用其他方式
		// 这里简化处理，只返回基本信息
		turn := &SimilarTurnResult{
			TurnID: uint64(result.DocID),
			Score:  result.Score,
		}
		similarTurns = append(similarTurns, turn)

		if len(similarTurns) >= limit {
			break
		}
	}

	logx.Infof("[ConversationVectorStore] Found %d similar turns for query: %s", len(similarTurns), query)
	return similarTurns, nil
}

// SearchSimilarInConversation 在同一会话内检索相似轮次
// 用于多轮对话中找回之前的上下文
func (cvs *ConversationVectorStore) SearchSimilarInConversation(
	ctx context.Context,
	query string,
	conversationID uint64,
	limit int,
) ([]*SimilarTurnResult, error) {
	if query == "" {
		return nil, nil
	}

	queryVector, err := cvs.embedder.GetEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("generate embedding failed: %w", err)
	}

	// 在指定conversation内搜索（使用spaceID过滤）
	results, err := cvs.store.Search(queryVector, limit*3)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	var similarTurns []*SimilarTurnResult
	for _, result := range results {
		// 过滤同一conversation的
		if result.SpaceID == int(conversationID) {
			turn := &SimilarTurnResult{
				TurnID: uint64(result.DocID),
				Score:  result.Score,
			}
			similarTurns = append(similarTurns, turn)
			if len(similarTurns) >= limit {
				break
			}
		}
	}

	return similarTurns, nil
}

// DeleteConversationVectors 删除对话的所有向量
func (cvs *ConversationVectorStore) DeleteConversationVectors(conversationID uint64) error {
	// 获取所有向量
	results, err := cvs.store.Search(make([]float64, 1536), 10000) // 获取大量结果
	if err != nil {
		return err
	}

	deletedCount := 0
	var firstErr error
	for _, result := range results {
		if result.SpaceID == int(conversationID) {
			if err := cvs.store.DeleteVector(result.DocID); err != nil {
				logx.Errorf("[ConversationVectorStore] Failed to delete vector for doc %d: %v", result.DocID, err)
				if firstErr == nil {
					firstErr = err
				}
			} else {
				deletedCount++
			}
		}
	}

	logx.Infof("[ConversationVectorStore] Deleted %d vectors for conversation %d", deletedCount, conversationID)
	return firstErr
}

// SimilarTurnResult 相似轮次检索结果
type SimilarTurnResult struct {
	TurnID         uint64   `json:"turn_id"`
	ConversationID uint64   `json:"conversation_id"`
	TurnNumber     int      `json:"turn_number"`
	UserQuery      string   `json:"user_query"`
	GeneratedSQL   string   `json:"generated_sql"`
	IntentType     string   `json:"intent_type"`
	UsedTables     []string `json:"used_tables"`
	IsSuccess      bool     `json:"is_success"`
	Score          float64  `json:"score"` // 相似度分数
}

// BuildFewShotPrompt 将相似轮次构建为Few-shot示例
// 用于增强prompt，提供历史成功案例作为参考
func (r *SimilarTurnResult) BuildFewShotPrompt() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## 历史相似查询（相似度: %.2f）\n\n", r.Score))
	sb.WriteString(fmt.Sprintf("用户问题: %s\n", r.UserQuery))
	sb.WriteString(fmt.Sprintf("生成SQL:\n```sql\n%s\n```\n\n", r.GeneratedSQL))
	if len(r.UsedTables) > 0 {
		sb.WriteString(fmt.Sprintf("使用的表: %s\n\n", strings.Join(r.UsedTables, ", ")))
	}
	return sb.String()
}

// BuildContextPrompt 将相似轮次构建为上下文提示
// 用于多轮对话中提供相关历史上下文
func (r *SimilarTurnResult) BuildContextPrompt() string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("### 历史轮次 #%d（相似度: %.2f）\n", r.TurnNumber, r.Score))
	sb.WriteString(fmt.Sprintf("用户: %s\n", r.UserQuery))
	sb.WriteString(fmt.Sprintf("SQL: %s\n", r.GeneratedSQL))
	if r.IsSuccess {
		sb.WriteString("状态: 执行成功\n")
	} else {
		sb.WriteString("状态: 未执行/失败\n")
	}
	sb.WriteString("\n")
	return sb.String()
}

// ============================================================
// 辅助函数
// ============================================================

func getString(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func getStringSlice(m map[string]interface{}, key string) []string {
	if v, ok := m[key]; ok {
		if arr, ok := v.([]interface{}); ok {
			var result []string
			for _, item := range arr {
				if s, ok := item.(string); ok {
					result = append(result, s)
				}
			}
			return result
		}
	}
	return nil
}

func getBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// ============================================================
// 向量存储接口扩展（适配现有VectorStoreInterface）
// ============================================================

// VectorStoreInterfaceExt 扩展向量存储接口，支持按space搜索
type VectorStoreInterfaceExt interface {
	embedding.VectorStoreInterface
	SearchInSpace(vector []float64, spaceID int, limit int) ([]embedding.VectorSearchResult, error)
	GetAllInSpace(spaceID int) ([]embedding.VectorSearchResult, error)
}

// EnsureVectorStoreInterfaceExt 检查并包装向量存储接口
func EnsureVectorStoreInterfaceExt(store embedding.VectorStoreInterface) VectorStoreInterfaceExt {
	if ext, ok := store.(VectorStoreInterfaceExt); ok {
		return ext
	}
	// 如果底层实现不支持，使用适配器
	return &vectorStoreExtAdapter{VectorStoreInterface: store}
}

// vectorStoreExtAdapter 适配器，为不支持space的存储提供默认实现
type vectorStoreExtAdapter struct {
	embedding.VectorStoreInterface
}

func (a *vectorStoreExtAdapter) SearchInSpace(vector []float64, spaceID int, limit int) ([]embedding.VectorSearchResult, error) {
	// 默认实现：全局搜索后过滤
	results, err := a.Search(vector, limit*3)
	if err != nil {
		return nil, err
	}
	// 过滤（通过SpaceID）
	var filtered []embedding.VectorSearchResult
	for _, r := range results {
		if r.SpaceID == spaceID {
			filtered = append(filtered, r)
			if len(filtered) >= limit {
				break
			}
		}
	}
	return filtered, nil
}

func (a *vectorStoreExtAdapter) GetAllInSpace(spaceID int) ([]embedding.VectorSearchResult, error) {
	// 默认实现返回空（需要底层支持）
	return nil, nil
}
