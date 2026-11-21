package rag

import (
	"container/list"
	"sync"

	"github.com/listen808/citebase/internal/utils"
)

// CacheIndexEntry 缓存索引条目
type CacheIndexEntry struct {
	Key     string
	Vector  []float32
	Element *list.Element
}

// SearchResult 搜索结果
type SearchResult struct {
	Key   string
	Score float64
}

// CacheIndex 向量缓存索引（LRU + 简单向量搜索）
type CacheIndex struct {
	mu      sync.RWMutex
	entries map[string]*CacheIndexEntry
	order   *list.List
	maxSize int
}

// NewCacheIndex 创建缓存索引
func NewCacheIndex(maxSize int) *CacheIndex {
	return &CacheIndex{
		entries: make(map[string]*CacheIndexEntry),
		order:   list.New(),
		maxSize: maxSize,
	}
}

// Add 添加条目
func (c *CacheIndex) Add(key string, vector []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 如果已存在，先删除
	if entry, exists := c.entries[key]; exists {
		c.order.Remove(entry.Element)
		delete(c.entries, key)
	}

	// 检查是否需要淘汰
	for c.order.Len() >= c.maxSize {
		c.evictOldest()
	}

	// 添加新条目
	entry := &CacheIndexEntry{
		Key:    key,
		Vector: vector,
	}
	entry.Element = c.order.PushBack(key)
	c.entries[key] = entry
}

// Remove 删除条目
func (c *CacheIndex) Remove(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, exists := c.entries[key]; exists {
		c.order.Remove(entry.Element)
		delete(c.entries, key)
	}
}

// Search 搜索相似向量
func (c *CacheIndex) Search(query []float32, threshold float64, limit int) []SearchResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var results []SearchResult

	for key, entry := range c.entries {
		score := utils.CosineSimilarityFloat32(query, entry.Vector)
		if score >= threshold {
			results = append(results, SearchResult{
				Key:   key,
				Score: score,
			})
		}
	}

	// 按分数降序排序
	sortSearchResults(results)

	// 限制结果数量
	if len(results) > limit {
		results = results[:limit]
	}

	return results
}

// Size 返回当前大小
func (c *CacheIndex) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.order.Len()
}

// evictOldest 淘汰最旧条目（内部调用，需持有锁）
func (c *CacheIndex) evictOldest() {
	if c.order.Len() == 0 {
		return
	}

	oldest := c.order.Front()
	if oldest != nil {
		key := oldest.Value.(string)
		c.order.Remove(oldest)
		delete(c.entries, key)
	}
}

// sortSearchResults 对搜索结果按分数降序排序
func sortSearchResults(results []SearchResult) {
	for i := 1; i < len(results); i++ {
		for j := i; j > 0 && results[j].Score > results[j-1].Score; j-- {
			results[j], results[j-1] = results[j-1], results[j]
		}
	}
}
