package rag

import (
	"fmt"
	"sync"
	"time"

	"github.com/listen808/citebase/internal/ai/embedding"
)

type cacheEntry struct {
	answer    *AIAnswer
	queryVec  []float64
	spaceID   int
	userID    int
	createdAt time.Time
}

type QueryCache struct {
	mu        sync.RWMutex
	entries   map[string]*cacheEntry
	index     *CacheIndex // 新增：向量索引
	maxSize   int
	ttl       time.Duration
	embedder  embedding.Embedder
	threshold float64
}

var queryCache *QueryCache

func InitQueryCache(emb embedding.Embedder, maxSize int, ttl time.Duration) {
	if maxSize <= 0 {
		maxSize = 100
	}
	if ttl <= 0 {
		ttl = 24 * time.Hour
	}
	queryCache = &QueryCache{
		entries:   make(map[string]*cacheEntry),
		index:     NewCacheIndex(maxSize),
		maxSize:   maxSize,
		ttl:       ttl,
		embedder:  emb,
		threshold: 0.95,
	}
}

func GetQueryCache() *QueryCache {
	if queryCache == nil {
		return nil
	}
	return queryCache
}

func (qc *QueryCache) Lookup(question string, spaceID int, userID int) *AIAnswer {
	if qc == nil {
		return nil
	}
	// compute embedding outside of lock
	var questionEmbedding []float64
	if qc.embedder != nil {
		emb, err := qc.embedder.GetEmbedding(question)
		if err == nil {
			questionEmbedding = emb
		}
	}

	qc.mu.RLock()
	defer qc.mu.RUnlock()

	key := cacheKey(question, spaceID, userID)

	// 1. 精确匹配
	if entry, ok := qc.entries[key]; ok {
		if time.Since(entry.createdAt) < qc.ttl {
			return entry.answer
		}
		return nil
	}

	// 2. 语义匹配（使用向量索引）
	if questionEmbedding != nil && qc.index != nil {
		// 将float64转换为float32
		queryVec32 := make([]float32, len(questionEmbedding))
		for i, v := range questionEmbedding {
			queryVec32[i] = float32(v)
		}

		results := qc.index.Search(queryVec32, qc.threshold, 1)
		if len(results) > 0 {
			if entry, ok := qc.entries[results[0].Key]; ok {
				if time.Since(entry.createdAt) < qc.ttl && entry.spaceID == spaceID && entry.userID == userID {
					return entry.answer
				}
			}
		}
	}

	return nil
}

func (qc *QueryCache) Store(question string, spaceID int, userID int, answer *AIAnswer) {
	if qc == nil {
		return
	}

	// 计算嵌入（在锁外）
	var queryVec []float64
	var queryVec32 []float32
	if qc.embedder != nil {
		vec, err := qc.embedder.GetEmbedding(question)
		if err == nil {
			queryVec = vec
			// 转换为float32用于索引
			queryVec32 = make([]float32, len(vec))
			for i, v := range vec {
				queryVec32[i] = float32(v)
			}
		}
	}

	qc.mu.Lock()
	defer qc.mu.Unlock()

	key := cacheKey(question, spaceID, userID)

	// 添加到索引
	if queryVec32 != nil && qc.index != nil {
		qc.index.Add(key, queryVec32)
	}

	qc.entries[key] = &cacheEntry{
		answer:    answer,
		queryVec:  queryVec,
		spaceID:   spaceID,
		userID:    userID,
		createdAt: time.Now(),
	}

	// 批量淘汰
	if len(qc.entries) > qc.maxSize {
		qc.evict()
	}
}

func (qc *QueryCache) evict() {
	now := time.Now()

	// 1. 批量清理过期条目
	for key, entry := range qc.entries {
		if now.Sub(entry.createdAt) > qc.ttl {
			if qc.index != nil {
				qc.index.Remove(key)
			}
			delete(qc.entries, key)
		}
	}

	// 2. 如果仍然超过容量，批量清理最旧条目
	for len(qc.entries) > qc.maxSize {
		var oldestKey string
		oldestTime := now

		for k, v := range qc.entries {
			if v.createdAt.Before(oldestTime) {
				oldestTime = v.createdAt
				oldestKey = k
			}
		}

		if oldestKey != "" {
			if qc.index != nil {
				qc.index.Remove(oldestKey)
			}
			delete(qc.entries, oldestKey)
		} else {
			break
		}
	}
}

func (qc *QueryCache) Clear() {
	if qc == nil {
		return
	}
	qc.mu.Lock()
	defer qc.mu.Unlock()
	qc.entries = make(map[string]*cacheEntry)
	// 重建索引
	if qc.index != nil {
		qc.index = NewCacheIndex(qc.maxSize)
	}
}

func (qc *QueryCache) Size() int {
	if qc == nil {
		return 0
	}
	qc.mu.RLock()
	defer qc.mu.RUnlock()
	return len(qc.entries)
}

// PreloadEntry 预加载缓存条目
type PreloadEntry struct {
	Question string
	SpaceID  int
	UserID   int
	Answer   *AIAnswer
}

// Preload 批量预加载缓存条目（缓存预热）
func (qc *QueryCache) Preload(entries []PreloadEntry) {
	if qc == nil || len(entries) == 0 {
		return
	}

	for _, e := range entries {
		key := cacheKey(e.Question, e.SpaceID, e.UserID)

		var queryVec []float64
		var queryVec32 []float32
		if qc.embedder != nil {
			vec, err := qc.embedder.GetEmbedding(e.Question)
			if err == nil {
				queryVec = vec
				// 转换为float32用于索引
				queryVec32 = make([]float32, len(vec))
				for i, v := range vec {
					queryVec32[i] = float32(v)
				}
			}
		}

		qc.mu.Lock()
		// 添加到索引
		if queryVec32 != nil && qc.index != nil {
			qc.index.Add(key, queryVec32)
		}
		qc.entries[key] = &cacheEntry{
			answer:    e.Answer,
			queryVec:  queryVec,
			spaceID:   e.SpaceID,
			userID:    e.UserID,
			createdAt: time.Now(),
		}
		if len(qc.entries) > qc.maxSize {
			qc.evict()
		}
		qc.mu.Unlock()
	}
}

func cacheKey(question string, spaceID int, userID int) string {
	return fmt.Sprintf("%d:%d:%s", userID, spaceID, question)
}
