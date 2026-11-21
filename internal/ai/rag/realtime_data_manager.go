package rag

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	logx "github.com/listen808/citebase/internal/log"
)

// CachedData 缓存的数据
type CachedData struct {
	Key         string
	Value       string
	FetchedAt   time.Time
	ExpiresAt   time.Time
	Source      string
	Query       string
	AccessCount int
}

// RealtimeDataManager 实时数据管理器
// 管理搜索结果和实时数据的缓存，过期自动刷新
type RealtimeDataManager struct {
	mu         sync.RWMutex
	cache      map[string]*CachedData
	defaultTTL time.Duration // 默认缓存时长
	maxSize    int           // 最大缓存条目数
}

// NewRealtimeDataManager 创建实时数据管理器
func NewRealtimeDataManager() *RealtimeDataManager {
	return &RealtimeDataManager{
		cache:      make(map[string]*CachedData),
		defaultTTL: 1 * time.Hour, // 默认缓存 1 小时
		maxSize:    1000,          // 最多缓存 1000 条
	}
}

// NewRealtimeDataManagerWithConfig 创建带配置的管理器
func NewRealtimeDataManagerWithConfig(ttl time.Duration, maxSize int) *RealtimeDataManager {
	return &RealtimeDataManager{
		cache:      make(map[string]*CachedData),
		defaultTTL: ttl,
		maxSize:    maxSize,
	}
}

// CacheSearchResult 缓存搜索结果
func (m *RealtimeDataManager) CacheSearchResult(query, source, result string) string {
	key := m.generateKey(query, source)

	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否需要清理缓存
	if len(m.cache) >= m.maxSize {
		m.evictOldest()
	}

	m.cache[key] = &CachedData{
		Key:         key,
		Value:       result,
		FetchedAt:   time.Now(),
		ExpiresAt:   time.Now().Add(m.defaultTTL),
		Source:      source,
		Query:       query,
		AccessCount: 0,
	}

	logx.Infof("[RealtimeDataManager] Cached search result for key=%s, ttl=%v", key[:8], m.defaultTTL)
	return key
}

// GetCachedResult 获取缓存的搜索结果
func (m *RealtimeDataManager) GetCachedResult(query, source string) (string, bool) {
	key := m.generateKey(query, source)

	// P3：AccessCount 写操作需在写锁下进行，避免并发写竞争
	m.mu.Lock()
	defer m.mu.Unlock()

	data, ok := m.cache[key]
	if !ok {
		return "", false
	}

	// 检查是否过期
	if time.Now().After(data.ExpiresAt) {
		return "", false
	}

	// 更新访问计数
	data.AccessCount++

	return data.Value, true
}

// InvalidateCache 使缓存失效
func (m *RealtimeDataManager) InvalidateCache(query, source string) {
	key := m.generateKey(query, source)

	m.mu.Lock()
	defer m.mu.Unlock()

	delete(m.cache, key)
	logx.Infof("[RealtimeDataManager] Invalidated cache for key=%s", key[:8])
}

// InvalidateAll 使所有缓存失效
func (m *RealtimeDataManager) InvalidateAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cache = make(map[string]*CachedData)
	logx.Infof("[RealtimeDataManager] All cache invalidated")
}

// RefreshExpired 刷新过期的缓存
func (m *RealtimeDataManager) RefreshExpired(searchFn func(query string) (string, error)) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	refreshed := 0

	for key, data := range m.cache {
		if now.After(data.ExpiresAt) {
			// 计算新的过期时间
			data.ExpiresAt = now.Add(m.defaultTTL)
			data.FetchedAt = now
			refreshed++
			logx.Infof("[RealtimeDataManager] Refreshed expired cache for key=%s", key[:8])
		}
	}

	return refreshed
}

// CleanupExpired 清理已过期的缓存
func (m *RealtimeDataManager) CleanupExpired() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	removed := 0

	for key, data := range m.cache {
		if now.After(data.ExpiresAt) {
			delete(m.cache, key)
			removed++
		}
	}

	if removed > 0 {
		logx.Infof("[RealtimeDataManager] Cleaned up %d expired cache entries", removed)
	}

	return removed
}

// GetStats 获取缓存统计信息
type CacheStats struct {
	TotalEntries   int
	ExpiredEntries int
	TotalAccesses  int
	AvgAccessCount float64
}

// GetStats 获取缓存统计
func (m *RealtimeDataManager) GetStats() *CacheStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &CacheStats{
		TotalEntries: len(m.cache),
	}

	now := time.Now()
	var totalAccess int

	for _, data := range m.cache {
		if now.After(data.ExpiresAt) {
			stats.ExpiredEntries++
		}
		totalAccess += data.AccessCount
	}

	stats.TotalAccesses = totalAccess
	if stats.TotalEntries > 0 {
		stats.AvgAccessCount = float64(totalAccess) / float64(stats.TotalEntries)
	}

	return stats
}

// GetCachedQueries 获取所有缓存的查询
func (m *RealtimeDataManager) GetCachedQueries() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	queries := make([]string, 0, len(m.cache))
	for _, data := range m.cache {
		queries = append(queries, data.Query)
	}

	return queries
}

// GetCacheEntry 获取单个缓存条目详情
// P3：返回内部指针的浅拷贝，避免调用方修改影响缓存
func (m *RealtimeDataManager) GetCacheEntry(query, source string) *CachedData {
	key := m.generateKey(query, source)

	m.mu.RLock()
	defer m.mu.RUnlock()

	data, ok := m.cache[key]
	if !ok {
		return nil
	}
	// 返回拷贝，避免外部修改影响缓存内部状态
	cp := *data
	return &cp
}

// SetTTL 设置缓存时长
func (m *RealtimeDataManager) SetTTL(ttl time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.defaultTTL = ttl
	logx.Infof("[RealtimeDataManager] TTL updated to %v", ttl)
}

// SetMaxSize 设置最大缓存条目数
func (m *RealtimeDataManager) SetMaxSize(size int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.maxSize = size
	// 如果当前缓存超过最大尺寸，清理最旧的
	if len(m.cache) > size {
		m.evictOldestUnlocked()
	}
}

// generateKey 生成缓存键
func (m *RealtimeDataManager) generateKey(query, source string) string {
	// 规范化查询：去除多余空格，转小写
	normalized := strings.ToLower(strings.TrimSpace(query))
	combined := fmt.Sprintf("%s:%s", source, normalized)

	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:])
}

// evictOldest 驱逐最旧的缓存条目
func (m *RealtimeDataManager) evictOldest() {
	m.evictOldestUnlocked()
}

// evictOldestUnlocked 驱逐最旧的缓存条目（无锁版本）
func (m *RealtimeDataManager) evictOldestUnlocked() {
	if len(m.cache) == 0 {
		return
	}

	var oldestKey string
	var oldestTime time.Time

	for key, data := range m.cache {
		if oldestTime.IsZero() || data.FetchedAt.Before(oldestTime) {
			oldestTime = data.FetchedAt
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(m.cache, oldestKey)
		logx.Infof("[RealtimeDataManager] Evicted oldest cache entry: %s", oldestKey[:8])
	}
}

// RealtimeCacheWithContext 带上下文的缓存查询
// 如果缓存存在且未过期，直接返回；否则执行查询函数并缓存结果
func (m *RealtimeDataManager) GetOrFetch(ctx context.Context, query, source string, fetchFn func(ctx context.Context, query string) (string, error)) (string, error) {
	// 先检查缓存
	if result, ok := m.GetCachedResult(query, source); ok {
		return result, nil
	}

	// 执行查询
	result, err := fetchFn(ctx, query)
	if err != nil {
		return "", err
	}

	// 缓存结果
	m.CacheSearchResult(query, source, result)

	return result, nil
}

// SearchResultProcessor 搜索结果处理器
// 对搜索结果进行去重、排序、质量评估
type SearchResultProcessor struct {
	provider Provider
}

// Provider LLM Provider 接口（简化定义）
type Provider interface {
	Chat(ctx context.Context, system string, messages []Message, opts ...Option) (*LLMResponse, error)
}

// Message 消息结构
type Message struct {
	Role    string
	Content string
}

// LLMResponse LLM 响应
type LLMResponse struct {
	Content string
}

// Option LLM 选项
type Option func(*Options)

// Options LLM 选项
type Options struct {
	MaxTokens   int
	Temperature float64
}

// WithMaxTokens 设置最大 token
func WithMaxTokens(n int) Option {
	return func(o *Options) {
		o.MaxTokens = n
	}
}

// WithTemperature 设置温度
func WithTemperature(t float64) Option {
	return func(o *Options) {
		o.Temperature = t
	}
}

// NewSearchResultProcessor 创建搜索结果处理器
func NewSearchResultProcessor(p Provider) *SearchResultProcessor {
	return &SearchResultProcessor{provider: p}
}

// ProcessedSearchResult 处理后的搜索结果
type ProcessedSearchResult struct {
	Results  []WebSearchResult
	Deduped  int // 去重数量
	Filtered int // 过滤数量
	Ranked   bool
}

// ProcessResults 处理搜索结果
func (p *SearchResultProcessor) ProcessResults(results []WebSearchResult, query string) *ProcessedSearchResult {
	processed := &ProcessedSearchResult{
		Results: make([]WebSearchResult, 0, len(results)),
	}

	seen := make(map[string]bool)

	for _, r := range results {
		// 去重：基于 URL
		urlKey := strings.ToLower(strings.TrimSpace(r.URL))
		if urlKey == "" || seen[urlKey] {
			processed.Deduped++
			continue
		}
		seen[urlKey] = true

		// 过滤：移除空标题或空摘要
		if strings.TrimSpace(r.Title) == "" || strings.TrimSpace(r.Snippet) == "" {
			processed.Filtered++
			continue
		}

		// 过滤：移除明显的低质量来源
		if isLowQualitySource(r.URL) {
			processed.Filtered++
			continue
		}

		processed.Results = append(processed.Results, r)
	}

	// 简单排序：优先标题匹配度高的
	p.sortByRelevance(processed.Results, query)
	processed.Ranked = true

	return processed
}

// sortByRelevance 按相关性排序
func (p *SearchResultProcessor) sortByRelevance(results []WebSearchResult, query string) {
	queryLower := strings.ToLower(query)
	queryWords := strings.Fields(queryLower)

	// 冒泡排序
	for i := 0; i < len(results)-1; i++ {
		for j := 0; j < len(results)-i-1; j++ {
			score1 := calculateRelevance(results[j], queryWords)
			score2 := calculateRelevance(results[j+1], queryWords)
			if score1 < score2 {
				results[j], results[j+1] = results[j+1], results[j]
			}
		}
	}
}

// calculateRelevance 计算相关性评分
func calculateRelevance(result WebSearchResult, queryWords []string) int {
	score := 0
	titleLower := strings.ToLower(result.Title)
	snippetLower := strings.ToLower(result.Snippet)

	for _, word := range queryWords {
		// 标题中匹配 +2
		if strings.Contains(titleLower, word) {
			score += 2
		}
		// 摘要中匹配 +1
		if strings.Contains(snippetLower, word) {
			score++
		}
	}

	// 近期结果加分
	if result.PublishedAt != "" {
		if t, err := time.Parse(time.RFC3339, result.PublishedAt); err == nil {
			daysSince := time.Since(t).Hours() / 24
			if daysSince < 7 {
				score += 3
			} else if daysSince < 30 {
				score += 1
			}
		}
	}

	return score
}

// isLowQualitySource 判断是否为低质量来源
func isLowQualitySource(url string) bool {
	lowQualityDomains := []string{
		"click.links.yahoo.com",
		"search.yahoo.com",
		"r.search.yahoo.com",
	}

	urlLower := strings.ToLower(url)
	for _, domain := range lowQualityDomains {
		if strings.Contains(urlLower, domain) {
			return true
		}
	}

	return false
}
