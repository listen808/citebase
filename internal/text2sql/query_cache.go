package text2sql

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"
)

// QueryCacheEntry 查询缓存条目
type QueryCacheEntry struct {
	SQL         string        `json:"sql"`         // 生成的SQL
	Explanation string        `json:"explanation"` // SQL解释
	Timestamp   time.Time     `json:"timestamp"`   // 创建时间
	TTL         time.Duration `json:"ttl"`         // 过期时间
	SpaceID     uint64        `json:"space_id"`    // 所属空间
	HitCount    int           `json:"hit_count"`   // 命中次数
}

// QueryCache 查询缓存 - 基于问题语义缓存SQL生成结果
type QueryCache struct {
	entries    map[string]*QueryCacheEntry
	mutex      sync.RWMutex
	maxSize    int
	defaultTTL time.Duration
}

// NewQueryCache 创建查询缓存
func NewQueryCache(maxSize int, defaultTTL time.Duration) *QueryCache {
	if maxSize <= 0 {
		maxSize = 1000
	}
	if defaultTTL <= 0 {
		defaultTTL = 1 * time.Hour
	}
	return &QueryCache{
		entries:    make(map[string]*QueryCacheEntry),
		maxSize:    maxSize,
		defaultTTL: defaultTTL,
	}
}

// CacheKey 生成缓存键
// 使用问题文本的hash，支持中英文
func (qc *QueryCache) CacheKey(question string, spaceID uint64) string {
	// 标准化问题文本
	normalized := normalizeQuestion(question)

	// 添加spaceID区分不同空间
	keyMaterial := normalized + fmt.Sprintf("|space:%d", spaceID)

	// 计算SHA256 hash
	hash := sha256.Sum256([]byte(keyMaterial))
	return hex.EncodeToString(hash[:])
}

// normalizeQuestion 标准化问题文本
func normalizeQuestion(q string) string {
	// 转小写
	q = strings.ToLower(q)

	// 移除多余空白
	q = strings.Join(strings.Fields(q), " ")

	// 移除常见停用词（可选，保留以提高区分度）
	return q
}

// Get 获取缓存条目
func (qc *QueryCache) Get(key string) (*QueryCacheEntry, bool) {
	qc.mutex.RLock()
	defer qc.mutex.RUnlock()

	entry, exists := qc.entries[key]
	if !exists {
		return nil, false
	}

	// 检查是否过期
	if time.Since(entry.Timestamp) > entry.TTL {
		return nil, false
	}

	// 增加命中计数
	entry.HitCount++
	return entry, true
}

// Set 设置缓存条目
func (qc *QueryCache) Set(key string, sql, explanation string, spaceID uint64) {
	qc.SetWithTTL(key, sql, explanation, spaceID, qc.defaultTTL)
}

// SetWithTTL 设置缓存条目（指定TTL）
func (qc *QueryCache) SetWithTTL(key string, sql, explanation string, spaceID uint64, ttl time.Duration) {
	qc.mutex.Lock()
	defer qc.mutex.Unlock()

	// 如果缓存已满，移除最旧的条目
	if len(qc.entries) >= qc.maxSize {
		qc.evictOldest()
	}

	qc.entries[key] = &QueryCacheEntry{
		SQL:         sql,
		Explanation: explanation,
		Timestamp:   time.Now(),
		TTL:         ttl,
		SpaceID:     spaceID,
		HitCount:    0,
	}
}

// evictOldest 移除最旧的缓存条目
func (qc *QueryCache) evictOldest() {
	var oldestKey string
	var oldestTime time.Time

	for key, entry := range qc.entries {
		if oldestTime.IsZero() || entry.Timestamp.Before(oldestTime) {
			oldestTime = entry.Timestamp
			oldestKey = key
		}
	}

	if oldestKey != "" {
		delete(qc.entries, oldestKey)
	}
}

// Delete 删除缓存条目
func (qc *QueryCache) Delete(key string) {
	qc.mutex.Lock()
	defer qc.mutex.Unlock()
	delete(qc.entries, key)
}

// Clear 清空缓存
func (qc *QueryCache) Clear() {
	qc.mutex.Lock()
	defer qc.mutex.Unlock()
	qc.entries = make(map[string]*QueryCacheEntry)
}

// ClearSpace 清空指定空间的缓存
func (qc *QueryCache) ClearSpace(spaceID uint64) {
	qc.mutex.Lock()
	defer qc.mutex.Unlock()

	for key, entry := range qc.entries {
		if entry.SpaceID == spaceID {
			delete(qc.entries, key)
		}
	}
}

// Size 返回缓存大小
func (qc *QueryCache) Size() int {
	qc.mutex.RLock()
	defer qc.mutex.RUnlock()
	return len(qc.entries)
}

// Stats 返回缓存统计信息
func (qc *QueryCache) Stats() CacheStats {
	qc.mutex.RLock()
	defer qc.mutex.RUnlock()

	var totalHits int
	oldestTimestamp := time.Now()

	for _, entry := range qc.entries {
		totalHits += entry.HitCount
		if entry.Timestamp.Before(oldestTimestamp) {
			oldestTimestamp = entry.Timestamp
		}
	}

	return CacheStats{
		Size:      len(qc.entries),
		MaxSize:   qc.maxSize,
		TotalHits: totalHits,
		Oldest:    oldestTimestamp,
	}
}

// CacheStats 缓存统计
type CacheStats struct {
	Size      int       `json:"size"`
	MaxSize   int       `json:"max_size"`
	TotalHits int       `json:"total_hits"`
	Oldest    time.Time `json:"oldest_entry"`
}

// SimilarQuestionCache 相似问题缓存
// 用于记录相似问题及其对应的缓存键，避免重复计算
type SimilarQuestionCache struct {
	cache     *QueryCache
	threshold float64 // 相似度阈值
}

// NewSimilarQuestionCache 创建相似问题缓存
func NewSimilarQuestionCache(maxSize int, threshold float64) *SimilarQuestionCache {
	if threshold <= 0 {
		threshold = 0.85
	}
	return &SimilarQuestionCache{
		cache:     NewQueryCache(maxSize, 24*time.Hour), // 相似缓存TTL更长
		threshold: threshold,
	}
}

// FindSimilar 查找相似问题
func (sqc *SimilarQuestionCache) FindSimilar(question string, spaceID uint64) (string, bool) {
	// 这里可以接入语义相似度模型
	// 当前实现：基于关键词的简单匹配

	key := sqc.cache.CacheKey(question, spaceID)
	entry, found := sqc.cache.Get(key)
	if !found {
		return "", false
	}

	// 检查相似度（简化实现：基于问题结构相似度）
	normalizedQ := normalizeQuestion(question)
	normalizedCached := normalizeQuestion(entry.Explanation) // 借用Explanation存储原问题

	similarity := calculateSimpleSimilarity(normalizedQ, normalizedCached)
	return entry.SQL, similarity >= sqc.threshold
}

// StoreSimilar 存储相似问题
func (sqc *SimilarQuestionCache) StoreSimilar(question, sql string, spaceID uint64) {
	key := sqc.cache.CacheKey(question, spaceID)
	// 借用Explanation字段存储原问题
	sqc.cache.SetWithTTL(key, sql, question, spaceID, 24*time.Hour)
}

// calculateSimpleSimilarity 计算简单相似度
func calculateSimpleSimilarity(a, b string) float64 {
	// 基于关键词重叠度计算
	aWords := strings.Fields(a)
	bWords := strings.Fields(b)

	if len(aWords) == 0 || len(bWords) == 0 {
		return 0
	}

	// 构建集合
	aSet := make(map[string]bool)
	for _, w := range aWords {
		aSet[w] = true
	}

	// 计算重叠
	overlap := 0
	for _, w := range bWords {
		if aSet[w] {
			overlap++
		}
	}

	// Jaccard相似度
	union := len(aWords) + len(bWords) - overlap
	if union == 0 {
		return 0
	}

	return float64(overlap) / float64(union)
}

// GlobalQueryCache 全局查询缓存实例
var globalQueryCache *QueryCache
var globalQueryCacheOnce sync.Once

// GetGlobalQueryCache 获取全局查询缓存
func GetGlobalQueryCache() *QueryCache {
	globalQueryCacheOnce.Do(func() {
		globalQueryCache = NewQueryCache(5000, 2*time.Hour)
	})
	return globalQueryCache
}
