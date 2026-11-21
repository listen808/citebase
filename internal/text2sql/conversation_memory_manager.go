package text2sql

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/embedding"
	modelai "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

// ConversationMemoryManager Text2SQL对话记忆管理器
// 三层记忆架构：
// 1. 短期记忆（Short-term）：当前对话的内存状态（ConversationState）
// 2. 工作记忆（Working）：从数据库加载的最近N轮对话
// 3. 长期记忆（Long-term）：向量库存储的历史对话，支持语义检索
type ConversationMemoryManager struct {
	// 数据层
	repo        *airepo.Text2SQLConversationRepository
	vectorStore *ConversationVectorStore

	// 嵌入层
	embedder embedding.Embedder

	// 配置
	maxWorkingMemoryTurns int  // 工作记忆最大轮次（默认10）
	maxLongTermResults    int  // 长期记忆检索最大结果数（默认5）
	sessionTTL            int  // 会话过期时间（小时，默认24）
	enableVectorStore     bool // 是否启用向量存储
	enableCrossConvSearch bool // 是否启用跨对话搜索
}

// MemoryManagerConfig 记忆管理器配置
type MemoryManagerConfig struct {
	MaxWorkingMemoryTurns int  // 工作记忆最大轮次
	MaxLongTermResults    int  // 长期记忆检索最大结果数
	SessionTTLHours       int  // 会话过期时间（小时）
	EnableVectorStore     bool // 是否启用向量存储
	EnableCrossConvSearch bool // 是否启用跨对话搜索
}

// DefaultMemoryManagerConfig 默认配置
func DefaultMemoryManagerConfig() *MemoryManagerConfig {
	return &MemoryManagerConfig{
		MaxWorkingMemoryTurns: 10,
		MaxLongTermResults:    5,
		SessionTTLHours:       24,
		EnableVectorStore:     true,
		EnableCrossConvSearch: true,
	}
}

// NewConversationMemoryManager 创建对话记忆管理器
func NewConversationMemoryManager(
	repo *airepo.Text2SQLConversationRepository,
	vectorStore *ConversationVectorStore,
	embedder embedding.Embedder,
	config *MemoryManagerConfig,
) *ConversationMemoryManager {
	if config == nil {
		config = DefaultMemoryManagerConfig()
	}
	return &ConversationMemoryManager{
		repo:                  repo,
		vectorStore:           vectorStore,
		embedder:              embedder,
		maxWorkingMemoryTurns: config.MaxWorkingMemoryTurns,
		maxLongTermResults:    config.MaxLongTermResults,
		sessionTTL:            config.SessionTTLHours,
		enableVectorStore:     config.EnableVectorStore,
		enableCrossConvSearch: config.EnableCrossConvSearch,
	}
}

// ============================================================
// 对话状态持久化（数据库层）
// ============================================================

// SaveTurn 保存对话轮次到数据库（并可选存入向量库）
func (mm *ConversationMemoryManager) SaveTurn(
	ctx context.Context,
	userID, conversationID, spaceID uint64,
	turnNumber int,
	userQuery, resolvedQuery, intentType, generatedSQL, explanation string,
	tables, fields []string,
	isSuccess bool,
	executionError string,
) (*modelai.SQLConversationTurn, error) {
	turn := &modelai.SQLConversationTurn{
		ConversationID: conversationID,
		UserID:         userID,
		SpaceID:        spaceID,
		TurnNumber:     turnNumber,
		UserQuery:      userQuery,
		ResolvedQuery:  resolvedQuery,
		IntentType:     intentType,
		GeneratedSQL:   generatedSQL,
		SQLExplanation: explanation,
		UsedTables:     modelai.JSONString(tables),
		UsedFields:     modelai.JSONString(fields),
		IsSuccess:      isSuccess,
		ExecutionError: executionError,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	// 保存到数据库
	if err := mm.repo.CreateTurn(turn); err != nil {
		return nil, fmt.Errorf("save turn to db failed: %w", err)
	}

	// 如果启用向量存储，存入向量库
	if mm.enableVectorStore && mm.vectorStore != nil {
		turnDTO := &modelai.SQLConversationTurnDTO{
			ID:             turn.ID,
			ConversationID: conversationID,
			UserID:         userID,
			TurnNumber:     turnNumber,
			UserQuery:      userQuery,
			GeneratedSQL:   generatedSQL,
			IntentType:     intentType,
			UsedTables:     tables,
			IsSuccess:      isSuccess,
		}
		if err := mm.vectorStore.StoreTurn(ctx, turnDTO); err != nil {
			// 向量存储失败不影响主流程，只记录日志
			logx.Infof("[MemoryManager] Store turn to vector store failed: %v", err)
		} else {
			// 更新向量存储ID
			_ = mm.repo.UpdateTurnEmbeddingID(turn.ID, fmt.Sprintf("turn_%d_%d", conversationID, turnNumber))
		}
	}

	logx.Infof("[MemoryManager] Saved turn %d for conversation %d", turnNumber, conversationID)
	return turn, nil
}

// LoadConversationHistory 从数据库加载对话历史到ConversationState
// 这是强制使用对话历史的核心方法：每次生成前必须调用
func (mm *ConversationMemoryManager) LoadConversationHistory(
	convID uint64,
	cs *ConversationState,
) error {
	if cs == nil {
		return fmt.Errorf("conversation state is nil")
	}

	// 从数据库加载最近N轮
	turns, err := mm.repo.GetTurnsByConversationLimit(convID, mm.maxWorkingMemoryTurns)
	if err != nil {
		return fmt.Errorf("load turns from db failed: %w", err)
	}

	if len(turns) == 0 {
		logx.Infof("[MemoryManager] No history found for conversation %d", convID)
		return nil
	}

	// 清空现有内存状态（防止重复）
	cs.SQLHistory = nil
	cs.IntentHistory = nil
	cs.UsedTables = make(map[string]*TableSchemaInfo)
	cs.UsedFields = make(map[string][]string)

	// 加载到ConversationState
	for _, turn := range turns {
		// 重建SQLTurn
		sqlTurn := SQLTurn{
			TurnID:       turn.TurnNumber,
			UserQuery:    turn.UserQuery,
			GeneratedSQL: turn.GeneratedSQL,
			Timestamp:    turn.CreatedAt,
		}
		cs.SQLHistory = append(cs.SQLHistory, sqlTurn)

		// 重建意图
		intent := UserIntent{
			TurnID:     turn.TurnNumber,
			Query:      turn.UserQuery,
			IntentType: turn.IntentType,
		}
		cs.IntentHistory = append(cs.IntentHistory, intent)

		// 重建表使用记录
		for _, tableName := range turn.UsedTables {
			if _, exists := cs.UsedTables[tableName]; !exists {
				cs.UsedTables[tableName] = &TableSchemaInfo{Name: tableName}
			}
		}

		// 重建字段使用记录
		for _, field := range turn.UsedFields {
			parts := strings.SplitN(field, ".", 2)
			if len(parts) == 2 {
				cs.UsedFields[parts[0]] = append(cs.UsedFields[parts[0]], parts[1])
			}
		}
	}

	// 设置当前SQL为最后一轮
	if len(turns) > 0 {
		cs.CurrentSQL = turns[len(turns)-1].GeneratedSQL
		cs.currentTurn = turns[len(turns)-1].TurnNumber
	}

	// 解析最后一轮SQL结构
	if cs.CurrentSQL != "" {
		cs.LastParsedSQL = parseSQLStructure(cs.CurrentSQL)
	}

	logx.Infof("[MemoryManager] Loaded %d turns into conversation state for conv %d", len(turns), convID)
	return nil
}

// UpdateTurnResult 更新轮次的执行结果
func (mm *ConversationMemoryManager) UpdateTurnResult(turnID uint64, isSuccess bool, result, errMsg string) error {
	return mm.repo.UpdateTurnExecutionResult(turnID, result, isSuccess, errMsg)
}

// UpdateTurnFeedback 更新用户反馈
func (mm *ConversationMemoryManager) UpdateTurnFeedback(turnID uint64, rating int8) error {
	return mm.repo.UpdateTurnFeedback(turnID, rating)
}

// ============================================================
// 语义检索（向量层）
// ============================================================

// RetrieveSimilarHistory 检索相似历史对话（跨对话）
// 用于Few-shot学习和模式推荐
func (mm *ConversationMemoryManager) RetrieveSimilarHistory(
	ctx context.Context,
	query string,
	userID uint64,
) ([]*SimilarTurnResult, error) {
	if !mm.enableVectorStore || mm.vectorStore == nil {
		return nil, nil
	}

	results, err := mm.vectorStore.SearchSimilarTurns(ctx, query, userID, mm.maxLongTermResults)
	if err != nil {
		logx.Infof("[MemoryManager] Search similar history failed: %v", err)
		return nil, err
	}

	return results, nil
}

// RetrieveConversationContext 在同一会话内检索相关上下文
// 用于多轮对话中找回之前的特定轮次
func (mm *ConversationMemoryManager) RetrieveConversationContext(
	ctx context.Context,
	query string,
	conversationID uint64,
) ([]*SimilarTurnResult, error) {
	if !mm.enableVectorStore || mm.vectorStore == nil {
		return nil, nil
	}

	results, err := mm.vectorStore.SearchSimilarInConversation(ctx, query, conversationID, mm.maxLongTermResults)
	if err != nil {
		logx.Infof("[MemoryManager] Search conversation context failed: %v", err)
		return nil, err
	}

	return results, nil
}

// BuildFewShotFromHistory 从历史记录构建Few-shot示例
func (mm *ConversationMemoryManager) BuildFewShotFromHistory(
	ctx context.Context,
	query string,
	userID uint64,
	limit int,
) string {
	results, err := mm.RetrieveSimilarHistory(ctx, query, userID)
	if err != nil || len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 历史相似查询参考（基于语义检索）\n\n")
	for i, result := range results {
		if i >= limit {
			break
		}
		// 从数据库获取完整信息
		turn, err := mm.repo.FindByID(result.TurnID)
		if err != nil {
			continue
		}
		sb.WriteString(fmt.Sprintf("### 示例 %d（相似度: %.2f）\n", i+1, result.Score))
		sb.WriteString(fmt.Sprintf("用户问题: %s\n", turn.UserQuery))
		sb.WriteString(fmt.Sprintf("生成SQL:\n```sql\n%s\n```\n\n", turn.GeneratedSQL))
	}
	return sb.String()
}

// ============================================================
// 对话摘要管理
// ============================================================

// UpdateConversationSummary 更新对话摘要
func (mm *ConversationMemoryManager) UpdateConversationSummary(
	convID, userID, spaceID uint64,
	currentSQL string,
	keyTables, keyFields []string,
	turnCount, successCount int,
) error {
	summary := &modelai.SQLConversationSummary{
		ConversationID: convID,
		UserID:         userID,
		SpaceID:        spaceID,
		CurrentSQL:     currentSQL,
		KeyTables:      modelai.JSONString(keyTables),
		KeyFields:      modelai.JSONString(keyFields),
		TurnCount:      turnCount,
		SuccessCount:   successCount,
		UpdatedAt:      time.Now(),
	}

	return mm.repo.CreateOrUpdateSummary(summary)
}

// GetConversationSummary 获取对话摘要
func (mm *ConversationMemoryManager) GetConversationSummary(convID uint64) (*modelai.SQLConversationSummary, error) {
	return mm.repo.GetSummaryByConversation(convID)
}

// ============================================================
// 会话状态管理
// ============================================================

// SaveSessionState 保存会话状态到数据库
func (mm *ConversationMemoryManager) SaveSessionState(
	userID, conversationID, spaceID uint64,
	cs *ConversationState,
) error {
	if cs == nil {
		return nil
	}

	// 序列化状态
	stateJSON, err := SerializeConversationState(cs)
	if err != nil {
		return fmt.Errorf("serialize state failed: %w", err)
	}

	sessionKey := fmt.Sprintf("%d_%d", userID, conversationID)
	session := &modelai.SQLConversationSession{
		SessionKey:     sessionKey,
		UserID:         userID,
		ConversationID: conversationID,
		SpaceID:        spaceID,
		StateJSON:      stateJSON,
		IsActive:       true,
		LastAccessAt:   time.Now(),
	}

	// 检查是否已存在
	existing, err := mm.repo.GetSessionByKey(sessionKey)
	if err != nil {
		// P3：err 可能是"未找到"也可能是真实 DB 错误，统一创建；记录错误便于排查
		logx.Infof("[ConversationMemory] GetSessionByKey failed (will create new): %v", err)
		return mm.repo.CreateSession(session)
	}

	// 存在则更新
	session.ID = existing.ID
	session.CreatedAt = existing.CreatedAt
	if err := mm.repo.UpdateSessionState(sessionKey, stateJSON); err != nil {
		logx.Infof("[ConversationMemory] UpdateSessionState failed: %v", err)
		return err
	}
	return nil
}

// LoadSessionState 从数据库加载会话状态
func (mm *ConversationMemoryManager) LoadSessionState(
	userID, conversationID uint64,
) (*ConversationState, error) {
	sessionKey := fmt.Sprintf("%d_%d", userID, conversationID)
	session, err := mm.repo.GetSessionByKey(sessionKey)
	if err != nil {
		return nil, fmt.Errorf("session not found: %w", err)
	}

	// 反序列化状态
	cs, err := DeserializeConversationState(session.StateJSON)
	if err != nil {
		return nil, fmt.Errorf("deserialize state failed: %w", err)
	}

	// 更新最后访问时间
	if err := mm.repo.UpdateSessionState(sessionKey, session.StateJSON); err != nil {
		logx.Infof("[ConversationMemory] UpdateSessionState on load failed: %v", err)
	}

	return cs, nil
}

// DeactivateSession 停用会话
func (mm *ConversationMemoryManager) DeactivateSession(userID, conversationID uint64) error {
	sessionKey := fmt.Sprintf("%d_%d", userID, conversationID)
	return mm.repo.DeactivateSession(sessionKey)
}

// CleanupExpiredSessions 清理过期会话
func (mm *ConversationMemoryManager) CleanupExpiredSessions() error {
	return mm.repo.DeleteExpiredSessions(mm.sessionTTL)
}

// ============================================================
// 状态序列化/反序列化
// ============================================================

// ConversationStateSerializable 可序列化的对话状态
type ConversationStateSerializable struct {
	ConversationID int                         `json:"conversation_id"`
	SpaceID        int                         `json:"space_id"`
	UsedTables     map[string]*TableSchemaInfo `json:"used_tables"`
	UsedFields     map[string][]string         `json:"used_fields"`
	UsedJoins      []JoinRelation              `json:"used_joins"`
	SQLHistory     []SQLTurn                   `json:"sql_history"`
	CurrentSQL     string                      `json:"current_sql"`
	CurrentTurn    int                         `json:"current_turn"`
	IntentHistory  []UserIntent                `json:"intent_history"`
	ActiveFilters  []FilterCondition           `json:"active_filters"`
	LastParsedSQL  *ParsedSQLStructure         `json:"last_parsed_sql,omitempty"`
}

// SerializeConversationState 序列化对话状态
func SerializeConversationState(cs *ConversationState) (string, error) {
	if cs == nil {
		return "", nil
	}
	serializable := ConversationStateSerializable{
		ConversationID: cs.ConversationID,
		SpaceID:        cs.SpaceID,
		UsedTables:     cs.UsedTables,
		UsedFields:     cs.UsedFields,
		UsedJoins:      cs.UsedJoins,
		SQLHistory:     cs.SQLHistory,
		CurrentSQL:     cs.CurrentSQL,
		CurrentTurn:    cs.currentTurn,
		IntentHistory:  cs.IntentHistory,
		ActiveFilters:  cs.ActiveFilters,
		LastParsedSQL:  cs.LastParsedSQL,
	}
	bytes, err := json.Marshal(serializable)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

// DeserializeConversationState 反序列化对话状态
func DeserializeConversationState(data string) (*ConversationState, error) {
	if data == "" {
		return NewConversationState(0, 0), nil
	}
	var serializable ConversationStateSerializable
	if err := json.Unmarshal([]byte(data), &serializable); err != nil {
		return nil, err
	}
	return &ConversationState{
		ConversationID: serializable.ConversationID,
		SpaceID:        serializable.SpaceID,
		UsedTables:     serializable.UsedTables,
		UsedFields:     serializable.UsedFields,
		UsedJoins:      serializable.UsedJoins,
		SQLHistory:     serializable.SQLHistory,
		CurrentSQL:     serializable.CurrentSQL,
		currentTurn:    serializable.CurrentTurn,
		IntentHistory:  serializable.IntentHistory,
		ActiveFilters:  serializable.ActiveFilters,
		LastParsedSQL:  serializable.LastParsedSQL,
	}, nil
}

// ============================================================
// 完整对话上下文构建（供LLM使用）
// ============================================================

// BuildFullContextForLLM 构建完整的LLM上下文
// 强制包含：工作记忆 + 长期记忆 + 修改模式指令
func (mm *ConversationMemoryManager) BuildFullContextForLLM(
	ctx context.Context,
	cs *ConversationState,
	userID, conversationID uint64,
	currentQuery string,
	intentType string,
) string {
	var sb strings.Builder

	// 1. 修改模式指令（如果是follow_up/modify）
	if intentType == "follow_up" || intentType == "modify" {
		modContext := cs.BuildModificationContext(currentQuery)
		if modContext != "" {
			sb.WriteString(modContext)
			sb.WriteString("\n\n")
		}
	}

	// 2. 工作记忆（数据库加载的最近N轮）
	workContext := cs.GetContextForLLM(intentType)
	if workContext != "" {
		sb.WriteString(workContext)
		sb.WriteString("\n\n")
	}

	// 3. 长期记忆（向量检索的相似历史）
	if mm.enableCrossConvSearch && mm.vectorStore != nil {
		fewShot := mm.BuildFewShotFromHistory(ctx, currentQuery, userID, 3)
		if fewShot != "" {
			sb.WriteString(fewShot)
			sb.WriteString("\n\n")
		}
	}

	// 4. 强制保留指令（如果是follow_up/modify）
	if intentType == "follow_up" || intentType == "modify" {
		lastTurn := cs.GetLastTurn()
		if lastTurn != nil && lastTurn.GeneratedSQL != "" {
			forceKeep := fmt.Sprintf(`
## 【强制约束】基于上一轮SQL修改

上一轮SQL：
%s

**绝对规则**：
1. 你必须基于上述SQL进行修改，不是重新生成
2. 用户没有要求修改的部分，必须一字不差保留原样
3. 只修改用户明确要求的部分
4. 如果用户说"列出所有数据"，SELECT改为 SELECT * 或列出所有字段
5. 如果用户说"保持where"，WHERE子句完全保留，不要删除或修改
6. 最终SQL必须包含上一轮SQL的所有部分（FROM、JOIN、WHERE等），除非用户明确要求删除某部分
`, lastTurn.GeneratedSQL)
			sb.WriteString(forceKeep)
			sb.WriteString("\n\n")
		}
	}

	return sb.String()
}

// ============================================================
// 统计和监控
// ============================================================

// GetStats 获取用户对话统计
func (mm *ConversationMemoryManager) GetStats(userID uint64) (totalTurns, successTurns, totalConvs int64, err error) {
	return mm.repo.GetUserConversationStats(userID)
}

// GetRecentSuccessfulTurns 获取最近的成功轮次（用于调试和分析）
func (mm *ConversationMemoryManager) GetRecentSuccessfulTurns(userID uint64, limit int) ([]*modelai.SQLConversationTurn, error) {
	return mm.repo.GetRecentTurns(userID, limit)
}
