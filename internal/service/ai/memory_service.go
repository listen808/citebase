package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	aimod "github.com/listen808/citebase/internal/ai"
	provider "github.com/listen808/citebase/internal/ai/provider"
	logx "github.com/listen808/citebase/internal/log"
	aimodel "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/utils"
)

// MemoryService 记忆系统业务逻辑层
// 从 internal/api/ai/memory.go 中提取的业务逻辑
type MemoryService struct {
	repo *airepo.AIMemoryRepository
}

// NewMemoryService 创建 MemoryService 实例
func NewMemoryService(repo *airepo.AIMemoryRepository) *MemoryService {
	return &MemoryService{repo: repo}
}

// CreateMemoryRequest 创建记忆请求参数
type CreateMemoryRequest struct {
	Content      string   `json:"content" binding:"required"`
	MemoryType   string   `json:"memory_type"`
	Summary      string   `json:"summary"`
	SourceConvID uint     `json:"source_conv_id"`
	Tags         []string `json:"tags"`
	Importance   float64  `json:"importance"`
}

// CreateMemory 创建记忆
func (s *MemoryService) CreateMemory(userID uint, req *CreateMemoryRequest) (*aimodel.AIMemory, error) {
	memType := aimodel.MemoryType(req.MemoryType)
	if memType == "" {
		memType = aimodel.MemoryTypeKnowledge
	}

	// 用户手动创建的记忆视为已确认（区别于 LLM 推断的 pending）。
	mem := &aimodel.AIMemory{
		UserID:       userID,
		MemoryType:   memType,
		Content:      req.Content,
		Summary:      req.Summary,
		SourceConvID: req.SourceConvID,
		Tags:         req.Tags,
		Importance:   req.Importance,
		Status:       aimodel.MemoryStatusConfirmed,
	}
	if mem.Importance <= 0 {
		mem.Importance = 0.5
	}
	if mem.Summary == "" {
		mem.Summary = utils.TruncateWithEllipsis(req.Content, 200)
	}

	if err := s.repo.Create(mem); err != nil {
		return nil, fmt.Errorf("创建记忆失败: %w", err)
	}
	return mem, nil
}

// ListMemories 列出用户记忆
func (s *MemoryService) ListMemories(userID uint, memType string, limit, offset int) ([]*aimodel.AIMemory, int64, error) {
	return s.repo.FindByUserID(userID, memType, limit, offset)
}

// SearchMemories 搜索记忆
func (s *MemoryService) SearchMemories(userID uint, req *SearchMemoryRequest) ([]*aimodel.AIMemory, error) {
	return s.repo.SearchByContent(userID, req.Query, req.MemoryType, req.Limit)
}

// DeleteMemory 删除记忆
func (s *MemoryService) DeleteMemory(userID uint, memoryID uint) error {
	// 先查询确认 ownership
	var mem aimodel.AIMemory
	err := s.repo.DB().Where("id = ? AND user_id = ?", memoryID, userID).First(&mem).Error
	if err != nil {
		return fmt.Errorf("查询记忆失败或无权限: %w", err)
	}
	return s.repo.Delete(memoryID)
}

// GetMemoryByID 按 ID 获取记忆（内部使用）
func (s *MemoryService) GetMemoryByID(memoryID uint) (*aimodel.AIMemory, error) {
	var mem aimodel.AIMemory
	err := s.repo.DB().Where("id = ?", memoryID).First(&mem).Error
	if err != nil {
		return nil, err
	}
	return &mem, nil
}

// ExtractMemoriesRequest 提取记忆请求参数
type ExtractMemoriesRequest struct {
	ConversationID uint                     `json:"conversation_id" binding:"required"`
	Messages       []map[string]interface{} `json:"messages" binding:"required"`
	UserID         uint                     `json:"user_id" binding:"required"`
}

// SearchMemoryRequest 搜索记忆请求参数
type SearchMemoryRequest struct {
	Query      string `json:"query"`
	MemoryType string `json:"memory_type"`
	Limit      int    `json:"limit"`
}

// ExtractMemoriesResult 提取记忆结果
type ExtractMemoriesResult struct {
	Extracted int                 `json:"extracted"`
	Memories  []*aimodel.AIMemory `json:"memories"`
}

// extractedMemory LLM 提取的记忆项（内部类型）
type extractedMemory struct {
	Type       aimodel.MemoryType `json:"type"`
	Content    string             `json:"content"`
	Summary    string             `json:"summary"`
	Tags       []string           `json:"tags"`
	Importance float64            `json:"importance"`
}

// ExtractMemories 从对话中提取记忆（结合LLM）
func (s *MemoryService) ExtractMemories(ctx context.Context, module *aimod.Module, req *ExtractMemoriesRequest) (*ExtractMemoriesResult, error) {
	userID := req.UserID

	// 构建对话文本
	convText := buildConversationText(req.Messages)

	// 用快速模型提取记忆
	fastProvider := module.FastProvider()
	if fastProvider == nil {
		fastProvider = module.PrimaryProvider()
	}
	if fastProvider == nil {
		return nil, fmt.Errorf("AI Provider未配置")
	}

	extracted, err := extractMemoryWithLLM(ctx, fastProvider, convText)
	if err != nil {
		return nil, fmt.Errorf("记忆提取失败: %w", err)
	}

	// 保存提取的记忆
	var savedMemories []*aimodel.AIMemory
	for _, item := range extracted {
		// P2-8 全量待确认：LLM 推断的记忆一律先进 pending，用户在记忆页点头后才
		// 进入召回。存量记忆不受影响（迁移默认 confirmed）。
		mem := &aimodel.AIMemory{
			UserID:       userID,
			MemoryType:   item.Type,
			Content:      item.Content,
			Summary:      item.Summary,
			SourceConvID: req.ConversationID,
			Tags:         item.Tags,
			Importance:   item.Importance,
			Status:       aimodel.MemoryStatusPending,
		}
		if err := s.repo.Create(mem); err != nil {
			// 旧实现吞掉落库错误，用户只看到「提取到 N 条」与真实结果不符
			logx.Warnf("[Memory] 保存记忆失败 (user=%d, conv=%d): %v", userID, req.ConversationID, err)
		} else {
			savedMemories = append(savedMemories, mem)
		}
	}

	// 保存会话摘要
	if len(extracted) > 0 {
		summary := buildSessionSummary(extracted)
		keyPoints := extractKeyPoints(extracted)
		entities := extractEntities(extracted)
		sum := &aimodel.AIMemorySummary{
			UserID:       userID,
			ConvID:       req.ConversationID,
			Summary:      summary,
			KeyPoints:    keyPoints,
			Entities:     entities,
			TokenCount:   estimateTokens(convText),
			MessageCount: uint(len(req.Messages)),
		}
		// upsert 必须按 (conv_id, 当前用户) 定位：conversation_id 来自请求体，
		// 旧实现只按 conv_id 查询，用户 A 传 B 的 conv_id 即可覆写 B 的会话摘要。
		existing, _ := s.repo.FindSummaryByConvID(req.ConversationID, userID)
		if existing != nil {
			sum.ID = existing.ID
			_ = s.repo.UpdateSummary(sum)
		} else {
			_ = s.repo.CreateSummary(sum)
		}
	}

	return &ExtractMemoriesResult{
		Extracted: len(savedMemories),
		Memories:  savedMemories,
	}, nil
}

// RecallMemories 召回相关记忆（用于注入到prompt中）
func (s *MemoryService) RecallMemories(userID uint, req *SearchMemoryRequest) ([]*aimodel.AIMemory, string, error) {
	if req.Limit <= 0 {
		req.Limit = 10
	}
	mems, err := s.repo.SearchByContent(userID, req.Query, req.MemoryType, req.Limit)
	if err != nil {
		return nil, "", fmt.Errorf("召回记忆失败: %w", err)
	}
	recallCtx := formatRecallContext(mems)
	return mems, recallCtx, nil
}

// MemoryStats 记忆统计
type MemoryStats struct {
	Total        int64 `json:"total"`
	Preferences  int64 `json:"preferences"`
	Knowledge    int64 `json:"knowledge"`
	Sessions     int64 `json:"sessions"`
	Entities     int64 `json:"entities"`
	UserFeedback int64 `json:"user_feedback"`
}

// GetMemoryStats 获取记忆统计
//
// 必须覆盖全部 MemoryType：user_feedback（点踩样本）是 P0-4 才加进来的第五种，
// 当时只改了写入侧，这里仍按四种求和，于是 Total 恒小于列表实际条数——
// 记忆管理页会显示"共 3 条"却列出 5 条。新增类型时同步这张表。
func (s *MemoryService) GetMemoryStats(userID uint) (*MemoryStats, error) {
	var stats MemoryStats
	for _, mt := range []struct {
		field *int64
		typ   string
	}{
		{&stats.Preferences, "preference"},
		{&stats.Knowledge, "knowledge"},
		{&stats.Sessions, "session"},
		{&stats.Entities, "entity"},
		{&stats.UserFeedback, "user_feedback"},
	} {
		_, total, _ := s.repo.FindByUserID(userID, mt.typ, 1, 0)
		*mt.field = total
	}
	stats.Total = stats.Preferences + stats.Knowledge + stats.Sessions + stats.Entities + stats.UserFeedback
	return &stats, nil
}

// ===== 辅助函数 =====

func extractMemoryWithLLM(ctx context.Context, prov provider.Provider, convText string) ([]extractedMemory, error) {
	systemPrompt := `你是一个记忆提取助手。从对话中提取关键信息，分为以下类型：

1. preference（用户偏好）：用户提到的偏好、习惯、设置
2. knowledge（知识记忆）：用户学到的知识、理解的概念
3. entity（实体记忆）：提到的表名、字段名、项目名等

返回JSON数组格式，每个元素包含：
- type: "preference"|"knowledge"|"entity"
- content: 记忆内容（一句话描述）
- summary: 简短摘要（20字以内）
- tags: 相关标签数组
- importance: 重要性 0-1（越高越重要）

只返回JSON数组，不要其他内容。`

	contentBytes, _ := json.Marshal(convText)
	messages := []provider.Message{
		{Role: "user", Content: contentBytes},
	}

	resp, err := prov.Chat(ctx, systemPrompt, messages,
		provider.WithMaxTokens(1000),
		provider.WithTemperature(0.3),
	)
	if err != nil {
		return nil, err
	}

	var results []extractedMemory
	content := strings.TrimSpace(resp.Content)
	// 尝试提取JSON数组
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start >= 0 && end > start {
		jsonStr := content[start : end+1]
		// 旧实现按 "},{"、","、":" 手工切分：记忆内容里只要含逗号/冒号（中文句读、URL、时间）
		// 就会被截断成多条错误记录，且 tags 字段完全没解析。改为标准 JSON 反序列化，失败即上抛。
		var items []extractedMemory
		if err := json.Unmarshal([]byte(jsonStr), &items); err != nil {
			return nil, fmt.Errorf("解析记忆提取结果失败: %w", err)
		}
		for _, em := range items {
			if em.Content == "" {
				continue
			}
			if em.Importance <= 0 {
				em.Importance = 0.5 // 未给定时回退默认重要性
			}
			if em.Type == "" {
				em.Type = aimodel.MemoryTypeKnowledge
			}
			results = append(results, em)
		}
	}
	return results, nil
}

func buildConversationText(messages []map[string]interface{}) string {
	var sb strings.Builder
	for _, msg := range messages {
		role, _ := msg["role"].(string)
		content, _ := msg["content"].(string)
		sb.WriteString(fmt.Sprintf("%s: %s\n", role, content))
	}
	return sb.String()
}

func buildSessionSummary(items []extractedMemory) string {
	var parts []string
	for _, item := range items {
		if item.Summary != "" {
			parts = append(parts, item.Summary)
		} else {
			parts = append(parts, utils.TruncateWithEllipsis(item.Content, 50))
		}
	}
	return strings.Join(parts, "；")
}

func extractKeyPoints(items []extractedMemory) []string {
	var points []string
	for _, item := range items {
		points = append(points, utils.TruncateWithEllipsis(item.Content, 100))
	}
	if len(points) > 5 {
		points = points[:5]
	}
	return points
}

func extractEntities(items []extractedMemory) []string {
	seen := make(map[string]bool)
	var entities []string
	for _, item := range items {
		if item.Type == aimodel.MemoryTypeEntity && !seen[item.Content] {
			seen[item.Content] = true
			entities = append(entities, item.Content)
		}
	}
	return entities
}

func formatRecallContext(mems []*aimodel.AIMemory) string {
	if len(mems) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## 用户记忆（相关上下文）\n")
	for i, mem := range mems {
		sb.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, mem.MemoryType, mem.Summary))
	}
	return sb.String()
}

func estimateTokens(text string) uint {
	// 粗略估算: 中文约1.5字符/token, 英文约4字符/token
	return uint(len([]rune(text)) / 2)
}
