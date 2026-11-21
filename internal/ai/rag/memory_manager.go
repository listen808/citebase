package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/embedding"
	"github.com/listen808/citebase/internal/ai/provider"
	aimodel "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/utils"
)

// MemoryManager 增强的记忆管理器
// 支持跨会话学习、查询模式学习、记忆相关性评分
type MemoryManager struct {
	repo     *airepo.AIMemoryRepository
	provider provider.Provider
	mu       sync.RWMutex

	// 用户查询模式缓存
	queryPatterns map[uint64][]*QueryPattern // userID -> patterns
}

// QueryPattern 查询模式（从成功的SQL查询中学习）
type QueryPattern struct {
	Pattern     string    `json:"pattern"`      // 模式描述，如"按时间统计"
	SQLTemplate string    `json:"sql_template"` // SQL模板
	Frequency   int       `json:"frequency"`    // 使用频率
	LastUsed    time.Time `json:"last_used"`
	SuccessRate float64   `json:"success_rate"` // 成功率
}

// MemoryRelevance 记忆相关性评分
type MemoryRelevance struct {
	Memory  *aimodel.AIMemory `json:"memory"`
	Score   float64           `json:"score"`   // 相关性分数 0-1
	Reason  string            `json:"reason"`  // 为什么相关
	Context string            `json:"context"` // 使用场景
}

// NewMemoryManager 创建增强的记忆管理器
func NewMemoryManager(repo *airepo.AIMemoryRepository, p provider.Provider) *MemoryManager {
	return &MemoryManager{
		repo:          repo,
		provider:      p,
		queryPatterns: make(map[uint64][]*QueryPattern),
	}
}

// ============================================================
// 跨会话学习
// ============================================================

// savePatternsToDB 将查询模式持久化到数据库
// 调用者无需持有锁：内部会安全地快照模式列表后再执行 DB 写入
func (mm *MemoryManager) savePatternsToDB(userID uint64) {
	mm.mu.RLock()
	patterns := mm.queryPatterns[userID]
	if len(patterns) == 0 || mm.repo == nil {
		mm.mu.RUnlock()
		return
	}
	snapshot := make([]*QueryPattern, len(patterns))
	copy(snapshot, patterns)
	mm.mu.RUnlock()

	data, err := json.Marshal(snapshot)
	if err != nil {
		logx.Infof("[MemoryManager] Failed to marshal patterns for user %d: %v", userID, err)
		return
	}
	// 查找已有的模式记录
	existing, _, _ := mm.repo.FindByUserID(uint(userID), "query_pattern", 1, 0)
	if len(existing) > 0 {
		existing[0].Content = string(data)
		existing[0].AccessCount++
		mm.repo.DB().Save(existing[0])
	} else {
		mem := &aimodel.AIMemory{
			UserID:     uint(userID),
			MemoryType: "query_pattern",
			Content:    string(data),
			Summary:    fmt.Sprintf("查询模式 (%d条)", len(patterns)),
			Importance: 0.8,
		}
		_ = mm.repo.Create(mem)
	}
}

// loadPatternsFromDB 从数据库加载查询模式
func (mm *MemoryManager) loadPatternsFromDB(userID uint64) {
	if mm.repo == nil {
		return
	}
	memories, _, _ := mm.repo.FindByUserID(uint(userID), "query_pattern", 1, 0)
	if len(memories) == 0 {
		return
	}
	var patterns []*QueryPattern
	if err := json.Unmarshal([]byte(memories[0].Content), &patterns); err != nil {
		logx.Infof("[MemoryManager] Failed to unmarshal patterns for user %d: %v", userID, err)
		return
	}
	mm.queryPatterns[userID] = patterns
}

// LearnFromQuery 从成功的SQL查询中学习模式
func (mm *MemoryManager) LearnFromQuery(userID uint64, question, sql string, spaceID uint64) {
	mm.mu.Lock()

	pattern := &QueryPattern{
		Pattern:     mm.extractPattern(question),
		SQLTemplate: mm.normalizeSQL(sql),
		Frequency:   1,
		LastUsed:    time.Now(),
		SuccessRate: 1.0,
	}

	// 检查是否有相似的已有模式
	for _, existing := range mm.queryPatterns[userID] {
		if mm.patternsSimilar(existing, pattern) {
			existing.Frequency++
			existing.LastUsed = time.Now()
			existing.SuccessRate = (existing.SuccessRate + 1.0) / 2
			mm.mu.Unlock()
			mm.savePatternsToDB(userID)
			return
		}
	}

	mm.queryPatterns[userID] = append(mm.queryPatterns[userID], pattern)

	// 限制每个用户的模式数量
	if len(mm.queryPatterns[userID]) > 100 {
		mm.queryPatterns[userID] = mm.queryPatterns[userID][len(mm.queryPatterns[userID])-100:]
	}

	// 持久化到数据库
	mm.mu.Unlock()
	mm.savePatternsToDB(userID)
}

// LearnFromFeedback 从用户反馈中学习
func (mm *MemoryManager) LearnFromFeedback(userID uint64, question, sql string, positive bool) {
	mm.mu.Lock()

	for _, pattern := range mm.queryPatterns[userID] {
		if mm.patternMatchesQuestion(pattern, question) {
			if positive {
				pattern.SuccessRate = min(1.0, pattern.SuccessRate+0.1)
			} else {
				pattern.SuccessRate = max(0.0, pattern.SuccessRate-0.2)
			}
			mm.mu.Unlock()
			mm.savePatternsToDB(userID)
			return
		}
	}
	mm.mu.Unlock()
}

// GetRelevantPatterns 获取与当前问题相关的查询模式
func (mm *MemoryManager) GetRelevantPatterns(userID uint64, question string, limit int) []*QueryPattern {
	mm.mu.Lock()
	// 懒加载：首次访问时从DB加载（loadPatternsFromDB 假定调用方持有写锁）
	if _, ok := mm.queryPatterns[userID]; !ok {
		mm.queryPatterns[userID] = nil
		mm.loadPatternsFromDB(userID)
	}
	// 快照后立即解锁，避免在评分计算期间长期持有锁
	patterns := mm.queryPatterns[userID]
	snapshot := make([]*QueryPattern, len(patterns))
	copy(snapshot, patterns)
	mm.mu.Unlock()

	if len(snapshot) == 0 {
		return nil
	}

	type scoredPattern struct {
		pattern *QueryPattern
		score   float64
	}

	var scored []scoredPattern
	for _, p := range snapshot {
		score := mm.calculatePatternScore(p, question)
		if score > 0.3 {
			scored = append(scored, scoredPattern{pattern: p, score: score})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	if len(scored) > limit {
		scored = scored[:limit]
	}

	var result []*QueryPattern
	for _, s := range scored {
		result = append(result, s.pattern)
	}
	return result
}

// ============================================================
// 增强的记忆召回
// ============================================================

// RecallWithRelevance 带相关性评分的记忆召回
// P1-7: 召回时更新 LastAccessedAt 与 AccessCount（TouchOnRecall）
func (mm *MemoryManager) RecallWithRelevance(ctx context.Context, userID uint64, question string, memoryType string, limit int) []MemoryRelevance {
	// 1. 关键词搜索
	mems, err := mm.repo.SearchByContent(uint(userID), question, memoryType, limit*2)
	if err != nil {
		logx.Infof("[MemoryManager] search error: %v", err)
		return nil
	}

	// P1-7: 命中的记忆异步更新访问时间（避免阻塞召回）
	if mm.repo != nil {
		for _, mem := range mems {
			_ = mm.repo.TouchOnRecall(mem.ID)
		}
	}

	// 2. 使用 LLM 评分相关性
	if mm.provider != nil && len(mems) > 0 {
		scored := mm.scoreRelevanceWithLLM(ctx, question, mems)
		if len(scored) > limit {
			scored = scored[:limit]
		}
		return scored
	}

	// 3. 降级：基于关键词匹配评分
	var result []MemoryRelevance
	for _, mem := range mems {
		score := mm.keywordRelevanceScore(question, mem)
		if score > 0.2 {
			result = append(result, MemoryRelevance{
				Memory:  mem,
				Score:   score,
				Reason:  "关键词匹配",
				Context: "记忆召回",
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

// ============================================================
// P1-7 §3.2.2: 记忆分层与 TTL
// ============================================================

// 记忆分层阈值（可按需调整）
const (
	MemoryPromoteImportance = 0.7                  // Importance >= 0.7 升级为长期
	MemoryStaleThreshold    = 90 * 24 * time.Hour  // 90 天未命中 → 降级
	MemoryDeleteThreshold   = 180 * 24 * time.Hour // 180 天未命中 → 删除
)

// PromoteToLongTerm 将高重要性短期记忆升级为长期记忆
// 触发时机：记忆写入后、定时任务批量升级
func (mm *MemoryManager) PromoteToLongTerm(userID uint64) (int64, error) {
	if mm.repo == nil {
		return 0, fmt.Errorf("memory repo not configured")
	}
	promoted, err := mm.repo.PromoteToLongTerm(uint(userID), MemoryPromoteImportance)
	if err != nil {
		logx.Infof("[MemoryManager] PromoteToLongTerm failed for user %d: %v", userID, err)
		return 0, err
	}
	if promoted > 0 {
		logx.Infof("[MemoryManager] Promoted %d short-term memories to long-term for user %d", promoted, userID)
	}
	return promoted, nil
}

// ExpireStaleMemories 清理过期记忆（long → stale → delete）
// 触发时机：定时任务（建议每日一次）
func (mm *MemoryManager) ExpireStaleMemories() (int64, int64, error) {
	if mm.repo == nil {
		return 0, 0, fmt.Errorf("memory repo not configured")
	}
	staled, deleted, err := mm.repo.ExpireStaleMemories(MemoryStaleThreshold, MemoryDeleteThreshold)
	if err != nil {
		logx.Infof("[MemoryManager] ExpireStaleMemories failed: %v (staled=%d deleted=%d)", err, staled, deleted)
		return staled, deleted, err
	}
	if staled > 0 || deleted > 0 {
		logx.Infof("[MemoryManager] Memory TTL cleanup: %d demoted to stale, %d deleted", staled, deleted)
	}
	return staled, deleted, nil
}

// ============================================================
// 增强的记忆提取
// ============================================================

// ExtractMemoriesEnhanced 增强版记忆提取
func (mm *MemoryManager) ExtractMemoriesEnhanced(ctx context.Context, userID uint64, convText string) ([]*aimodel.AIMemory, error) {
	if mm.provider == nil {
		return nil, fmt.Errorf("provider not available")
	}

	systemPrompt := `你是一个专业的记忆提取助手。从对话中提取有价值的信息，分为以下类型：

## 记忆类型
1. **preference**（用户偏好）：用户的习惯、喜好、工作方式
   - 例：用户总是关注财务数据、偏好使用特定的SQL写法
   
2. **knowledge**（知识记忆）：用户学到的业务知识、概念理解
   - 例：bm_dc_param表存储配置参数、STATUS字段的枚举值含义

3. **entity**（实体记忆）：重要的表名、字段名、项目名、人名
   - 例：bm_dc_param是核心配置表、张三是项目负责人

4. **pattern**（查询模式）：用户常用的查询模式和SQL模式
   - 例：用户经常查询最近7天的数据、喜欢按部门分组统计

## 提取原则
- 只提取有价值的信息，忽略闲聊和无意义内容
- 每条记忆应该是独立的、可理解的
- 重要性评分：0.8+（关键业务知识）、0.5-0.8（有用信息）、0.3-0.5（次要信息）
- 为每条记忆生成简洁的标签

## 输出格式
返回JSON数组，每个元素包含：
{
  "type": "preference|knowledge|entity|pattern",
  "content": "记忆内容（完整描述）",
  "summary": "简短摘要（20字以内）",
  "tags": ["标签1", "标签2"],
  "importance": 0.7
}

只返回JSON数组，不要其他内容。`

	contentBytes, _ := json.Marshal(convText)
	messages := []provider.Message{
		{Role: "user", Content: contentBytes},
	}

	resp, err := mm.provider.Chat(ctx, systemPrompt, messages,
		provider.WithMaxTokens(1500),
		provider.WithTemperature(0.2),
	)
	if err != nil {
		return nil, fmt.Errorf("LLM call failed: %w", err)
	}

	// 解析结果
	var extracted []extractedMemoryEnhanced
	content := strings.TrimSpace(resp.Content)
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start >= 0 && end > start {
		jsonStr := content[start : end+1]
		if err := json.Unmarshal([]byte(jsonStr), &extracted); err != nil {
			// 尝试简单解析
			extracted = mm.simpleParseExtracted(jsonStr)
		}
	}

	// 转换为 AIMemory
	var memories []*aimodel.AIMemory
	for _, item := range extracted {
		if item.Content == "" {
			continue
		}
		mem := &aimodel.AIMemory{
			UserID:     uint(userID),
			MemoryType: item.Type,
			Content:    item.Content,
			Summary:    item.Summary,
			Tags:       item.Tags,
			Importance: item.Importance,
		}
		if mem.Summary == "" {
			mem.Summary = utils.TruncateWithEllipsis(item.Content, 200)
		}
		if mem.Importance <= 0 {
			mem.Importance = 0.5
		}
		memories = append(memories, mem)
	}

	return memories, nil
}

// ============================================================
// 内部辅助方法
// ============================================================

func (mm *MemoryManager) extractPattern(question string) string {
	lower := strings.ToLower(question)

	// 时间模式
	timePatterns := []struct {
		keywords []string
		pattern  string
	}{
		{[]string{"最近", "近", "过去"}, "时间范围查询"},
		{[]string{"今天", "今日"}, "当日查询"},
		{[]string{"本周", "这周"}, "本周查询"},
		{[]string{"本月", "这个月"}, "本月查询"},
		{[]string{"去年", "上年"}, "年度查询"},
	}

	for _, tp := range timePatterns {
		for _, kw := range tp.keywords {
			if strings.Contains(lower, kw) {
				return tp.pattern
			}
		}
	}

	// 聚合模式
	aggPatterns := []struct {
		keywords []string
		pattern  string
	}{
		{[]string{"统计", "总数", "数量", "多少"}, "聚合统计"},
		{[]string{"平均", "均值"}, "平均值计算"},
		{[]string{"最大", "最高", "最多"}, "极值查询"},
		{[]string{"分组", "按...分"}, "分组统计"},
		{[]string{"排名", "top", "前几"}, "排名查询"},
	}

	for _, ap := range aggPatterns {
		for _, kw := range ap.keywords {
			if strings.Contains(lower, kw) {
				return ap.pattern
			}
		}
	}

	// 关联模式
	if strings.Contains(lower, "关联") || strings.Contains(lower, "连接") || strings.Contains(lower, "join") {
		return "多表关联"
	}

	return "通用查询"
}

func (mm *MemoryManager) normalizeSQL(sql string) string {
	// 简单的SQL模板化：替换具体值为占位符
	sql = strings.TrimSpace(sql)
	// 替换字符串字面量
	sql = strings.ReplaceAll(sql, "'", "?")
	// 替换数字
	// 这里简化处理，实际可以更精确
	return sql
}

func (mm *MemoryManager) patternsSimilar(a, b *QueryPattern) bool {
	if a.Pattern != b.Pattern {
		return false
	}
	// 简单的SQL结构相似度
	aNorm := mm.normalizeSQL(a.SQLTemplate)
	bNorm := mm.normalizeSQL(b.SQLTemplate)
	return aNorm == bNorm
}

func (mm *MemoryManager) patternMatchesQuestion(pattern *QueryPattern, question string) bool {
	lower := strings.ToLower(question)
	patternLower := strings.ToLower(pattern.Pattern)

	switch patternLower {
	case "时间范围查询":
		return strings.Contains(lower, "最近") || strings.Contains(lower, "近") || strings.Contains(lower, "过去")
	case "聚合统计":
		return strings.Contains(lower, "统计") || strings.Contains(lower, "总数") || strings.Contains(lower, "多少")
	case "多表关联":
		return strings.Contains(lower, "关联") || strings.Contains(lower, "连接") || strings.Contains(lower, "join")
	default:
		return false
	}
}

func (mm *MemoryManager) calculatePatternScore(pattern *QueryPattern, question string) float64 {
	score := 0.0

	// 频率加分
	score += float64(pattern.Frequency) * 0.05
	if score > 0.3 {
		score = 0.3
	}

	// 成功率加分
	score += pattern.SuccessRate * 0.3

	// 时效性加分（最近使用的模式更相关）
	hoursSinceLastUse := time.Since(pattern.LastUsed).Hours()
	if hoursSinceLastUse < 24 {
		score += 0.2
	} else if hoursSinceLastUse < 168 { // 一周
		score += 0.1
	}

	// 问题匹配加分
	if mm.patternMatchesQuestion(pattern, question) {
		score += 0.3
	}

	return min(1.0, score)
}

func (mm *MemoryManager) scoreRelevanceWithLLM(ctx context.Context, question string, mems []*aimodel.AIMemory) []MemoryRelevance {
	if len(mems) == 0 {
		return nil
	}

	// 构建记忆列表
	var memList strings.Builder
	for i, mem := range mems {
		memList.WriteString(fmt.Sprintf("%d. [%s] %s\n", i+1, mem.MemoryType, mem.Summary))
	}

	prompt := fmt.Sprintf(`请评估以下记忆与查询的相关性。

查询：%s

记忆列表：
%s

为每条记忆评分（0-1），并说明原因。
返回JSON数组：[{"index": 1, "score": 0.8, "reason": "..."}]

只返回JSON数组。`, question, memList.String())

	messages := []provider.Message{
		provider.NewTextMessage("user", prompt),
	}

	resp, err := mm.provider.Chat(ctx, "你是一个相关性评估专家。只输出JSON。", messages,
		provider.WithMaxTokens(1000),
		provider.WithTemperature(0.1),
	)
	if err != nil {
		logx.Infof("[MemoryManager] LLM scoring error: %v", err)
		return nil
	}

	// 解析评分结果
	type scoreResult struct {
		Index  int     `json:"index"`
		Score  float64 `json:"score"`
		Reason string  `json:"reason"`
	}

	var scores []scoreResult
	content := strings.TrimSpace(resp.Content)
	start := strings.Index(content, "[")
	end := strings.LastIndex(content, "]")
	if start >= 0 && end > start {
		jsonStr := content[start : end+1]
		_ = json.Unmarshal([]byte(jsonStr), &scores)
	}

	// 构建结果
	var result []MemoryRelevance
	for _, s := range scores {
		if s.Index >= 1 && s.Index <= len(mems) && s.Score > 0.2 {
			result = append(result, MemoryRelevance{
				Memory:  mems[s.Index-1],
				Score:   s.Score,
				Reason:  s.Reason,
				Context: "LLM相关性评估",
			})
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})

	return result
}

func (mm *MemoryManager) keywordRelevanceScore(question string, mem *aimodel.AIMemory) float64 {
	questionWords := strings.Fields(strings.ToLower(question))
	memText := strings.ToLower(mem.Content + " " + mem.Summary + " " + strings.Join(mem.Tags, " "))

	matched := 0
	for _, word := range questionWords {
		if len(word) < 2 {
			continue
		}
		if strings.Contains(memText, word) {
			matched++
		}
	}

	if len(questionWords) == 0 {
		return 0
	}
	return float64(matched) / float64(len(questionWords))
}

// extractedMemoryEnhanced 增强的记忆提取结果
type extractedMemoryEnhanced struct {
	Type       aimodel.MemoryType `json:"type"`
	Content    string             `json:"content"`
	Summary    string             `json:"summary"`
	Tags       []string           `json:"tags"`
	Importance float64            `json:"importance"`
}

func (mm *MemoryManager) simpleParseExtracted(jsonStr string) []extractedMemoryEnhanced {
	var results []extractedMemoryEnhanced
	// 简单的JSON数组解析
	items := strings.Split(jsonStr, "},{")
	for _, item := range items {
		item = strings.Trim(item, "[]{}")
		em := extractedMemoryEnhanced{Importance: 0.5, Type: aimodel.MemoryTypeKnowledge}
		for _, pair := range strings.Split(item, ",") {
			kv := strings.SplitN(pair, ":", 2)
			if len(kv) != 2 {
				continue
			}
			key := strings.Trim(strings.TrimSpace(kv[0]), "\"")
			val := strings.Trim(strings.TrimSpace(kv[1]), "\"")
			switch key {
			case "type":
				em.Type = aimodel.MemoryType(val)
			case "content":
				em.Content = val
			case "summary":
				em.Summary = val
			case "importance":
				fmt.Sscanf(val, "%f", &em.Importance)
			}
		}
		if em.Content != "" {
			results = append(results, em)
		}
	}
	return results
}

// ============================================================
// P2-7: 向量语义记忆召回
// ============================================================

// VectorMemoryEntry represents a memory with its precomputed embedding vector.
type VectorMemoryEntry struct {
	Memory    *aimodel.AIMemory `json:"memory"`
	Embedding []float64         `json:"-"`
}

// VectorMemoryIndex is an in-memory vector index for a single user's memories.
type VectorMemoryIndex struct {
	entries []VectorMemoryEntry
	mu      sync.RWMutex
}

// NewVectorMemoryIndex creates an empty vector index.
func NewVectorMemoryIndex() *VectorMemoryIndex {
	return &VectorMemoryIndex{}
}

// BuildIndex computes embeddings for all memories and builds the index.
func (vi *VectorMemoryIndex) BuildIndex(ctx context.Context, embedder embedding.Embedder, memories []*aimodel.AIMemory) error {
	if embedder == nil || len(memories) == 0 {
		return nil
	}

	texts := make([]string, len(memories))
	for i, mem := range memories {
		texts[i] = mem.Content + " " + mem.Summary
	}

	vectors, err := embedder.GetEmbeddings(texts)
	if err != nil {
		return fmt.Errorf("vector index build failed: %w", err)
	}

	vi.mu.Lock()
	defer vi.mu.Unlock()
	vi.entries = make([]VectorMemoryEntry, 0, len(memories))
	for i, mem := range memories {
		if i < len(vectors) {
			vi.entries = append(vi.entries, VectorMemoryEntry{
				Memory:    mem,
				Embedding: vectors[i],
			})
		}
	}
	return nil
}

// Search returns the top-k most similar memories by cosine similarity.
func (vi *VectorMemoryIndex) Search(queryVec []float64, topK int) []VectorMemoryEntry {
	if topK <= 0 {
		topK = 5
	}

	vi.mu.RLock()
	defer vi.mu.RUnlock()

	type scored struct {
		entry VectorMemoryEntry
		score float64
	}
	var scores []scored
	for _, entry := range vi.entries {
		if len(entry.Embedding) == 0 || len(queryVec) == 0 {
			continue
		}
		sim := cosineSim(queryVec, entry.Embedding)
		scores = append(scores, scored{entry: entry, score: sim})
	}

	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	if len(scores) > topK {
		scores = scores[:topK]
	}

	result := make([]VectorMemoryEntry, len(scores))
	for i, s := range scores {
		result[i] = s.entry
	}
	return result
}

// RecallWithVector performs semantic vector-based memory recall.
// It embeds the query, computes cosine similarity against the memory index,
// and returns the most relevant memories. Falls back to keyword search if
// embedding is unavailable.
func (mm *MemoryManager) RecallWithVector(ctx context.Context, userID uint64, question string, memoryType string, limit int, embedder embedding.Embedder) []MemoryRelevance {
	if limit <= 0 {
		limit = 5
	}

	// Load memories from DB
	if mm.repo == nil {
		return nil
	}
	mems, err := mm.repo.SearchByContent(uint(userID), question, memoryType, limit*4)
	if err != nil {
		logx.Infof("[MemoryManager] vector recall: db load error: %v", err)
		return nil
	}
	if len(mems) == 0 {
		return nil
	}

	// Touch on recall
	for _, mem := range mems {
		_ = mm.repo.TouchOnRecall(mem.ID)
	}

	// If no embedder available, fall back to keyword scoring
	if embedder == nil {
		return mm.keywordRecallFallback(question, mems, limit)
	}

	// Build vector index
	vi := NewVectorMemoryIndex()
	if err := vi.BuildIndex(ctx, embedder, mems); err != nil {
		logx.Infof("[MemoryManager] vector index build failed: %v, falling back to keyword", err)
		return mm.keywordRecallFallback(question, mems, limit)
	}

	// Embed the query
	queryVec, err := embedder.GetEmbedding(question)
	if err != nil {
		logx.Infof("[MemoryManager] query embedding failed: %v, falling back to keyword", err)
		return mm.keywordRecallFallback(question, mems, limit)
	}

	// Search
	results := vi.Search(queryVec, limit)

	var relevance []MemoryRelevance
	for _, r := range results {
		// Only return results above a threshold
		sim := cosineSim(queryVec, r.Embedding)
		if sim < 0.3 {
			continue
		}
		// P7 指数衰减：久未命中的记忆排位下降但不消失（保底 0.5），
		// 半衰期 30 天 —— 与 TTL 断崖式降级互补：TTL 管生死，衰减管排序。
		score := sim * memoryDecayWeight(r.Memory)
		relevance = append(relevance, MemoryRelevance{
			Memory:  r.Memory,
			Score:   score,
			Reason:  fmt.Sprintf("向量语义相似度 %.3f（含时近衰减）", sim),
			Context: "vector_recall",
		})
	}

	return relevance
}

// memoryDecayHalfLifeDays 衰减半衰期：30 天未命中的记忆权重减半。
const memoryDecayHalfLifeDays = 30.0

// memoryDecayWeight 基于 LastAccessedAt 的指数衰减权重 ∈ [0.5, 1]。
// 从未被访问过的记忆按创建时间算 —— 新记忆权重满格。
func memoryDecayWeight(mem *aimodel.AIMemory) float64 {
	ref := mem.CreatedAt
	if mem.LastAccessedAt != nil {
		ref = *mem.LastAccessedAt
	}
	if ref.IsZero() {
		return 1
	}
	idleDays := time.Since(ref).Hours() / 24
	if idleDays <= 0 {
		return 1
	}
	w := math.Pow(0.5, idleDays/memoryDecayHalfLifeDays)
	if w < 0.5 {
		w = 0.5
	}
	return w
}

// keywordRecallFallback is the non-vector fallback for RecallWithVector.
func (mm *MemoryManager) keywordRecallFallback(question string, mems []*aimodel.AIMemory, limit int) []MemoryRelevance {
	var result []MemoryRelevance
	for _, mem := range mems {
		score := mm.keywordRelevanceScore(question, mem) * memoryDecayWeight(mem)
		if score > 0.2 {
			result = append(result, MemoryRelevance{
				Memory:  mem,
				Score:   score,
				Reason:  "关键词匹配（降级）",
				Context: "keyword_fallback",
			})
		}
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].Score > result[j].Score
	})
	if len(result) > limit {
		result = result[:limit]
	}
	return result
}

// cosineSim computes cosine similarity between two vectors.
func cosineSim(a, b []float64) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
