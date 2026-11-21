package embedding

import (
	logx "github.com/listen808/citebase/internal/log"
	"math"
	"math/rand"
	"sort"
	"sync"

	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/config"
)

// S7: HNSWVectorStore 基于HNSW近似最近邻的向量存储
// 在BBolt持久化基础上增加内存HNSW索引加速检索
type HNSWVectorStore struct {
	inner  *VectorStore
	index  *hnswIndex
	mu     sync.RWMutex
	loaded bool
}

const (
	hnswM        = 16
	hnswEfSearch = 50
	hnswMaxLevel = 4
)

type hnswNode struct {
	id      int
	vec     []float64
	level   int
	links   [][]int
	spaceID int
	name    string
}

type hnswIndex struct {
	nodes    map[int]*hnswNode
	entryID  int
	maxLevel int
	m        int
	efSearch int
	mu       sync.RWMutex
}

func newHNSWIndex() *hnswIndex {
	return &hnswIndex{
		nodes:    make(map[int]*hnswNode),
		entryID:  -1,
		maxLevel: 0,
		m:        hnswM,
		efSearch: hnswEfSearch,
	}
}

func NewHNSWVectorStore(inner *VectorStore) *HNSWVectorStore {
	return &HNSWVectorStore{
		inner:  inner,
		index:  newHNSWIndex(),
		loaded: false,
	}
}

func (h *HNSWVectorStore) StoreVector(docID int, spaceID int, name string, vector []float64) error {
	if err := h.inner.StoreVector(docID, spaceID, name, vector); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.insertToIndex(docID, spaceID, name, vector)
	return nil
}

func (h *HNSWVectorStore) DeleteVector(docID int) error {
	if err := h.inner.DeleteVector(docID); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.index.mu.Lock()
	defer h.index.mu.Unlock()
	h.removeNodeFromIndex(docID)
	return nil
}

// DeleteDocVectors 删除文档及其所有 chunk 向量
func (h *HNSWVectorStore) DeleteDocVectors(docID int, maxChunks int) error {
	if err := h.inner.DeleteDocVectors(docID, maxChunks); err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.index.mu.Lock()
	defer h.index.mu.Unlock()
	// 删除主文档和所有 chunk 的 HNSW 节点（含邻居 links 清理）
	h.removeNodeFromIndex(docID)
	for i := 0; i < maxChunks; i++ {
		h.removeNodeFromIndex(docID*1000 + i)
	}
	return nil
}

// removeNodeFromIndex 从 HNSW 索引中删除节点并清理所有邻居的反向链接
// 标准 HNSW 要求：删除节点时必须遍历其 links，从每个邻居的 links 中移除该节点 ID
// 调用方必须持有 h.index.mu 写锁
func (h *HNSWVectorStore) removeNodeFromIndex(docID int) {
	node, ok := h.index.nodes[docID]
	if !ok {
		return
	}
	// 遍历该节点所有层的 links，从邻居的对应层中移除 docID
	for lvl := 0; lvl <= node.level; lvl++ {
		for _, neighborID := range node.links[lvl] {
			neighbor, ok := h.index.nodes[neighborID]
			if !ok {
				continue
			}
			for l := 0; l <= neighbor.level; l++ {
				if len(neighbor.links[l]) == 0 {
					continue
				}
				filtered := neighbor.links[l][:0]
				for _, id := range neighbor.links[l] {
					if id != docID {
						filtered = append(filtered, id)
					}
				}
				neighbor.links[l] = filtered
			}
		}
	}
	delete(h.index.nodes, docID)
	// 若删除的是入口节点，重新选择 maxLevel 最高的节点作为入口
	if h.index.entryID == docID {
		h.index.entryID = -1
		h.index.maxLevel = 0
		for id, n := range h.index.nodes {
			if h.index.entryID == -1 || n.level > h.index.maxLevel {
				h.index.entryID = id
				h.index.maxLevel = n.level
			}
		}
	}
}

func (h *HNSWVectorStore) DeleteVectorsByPrefix(prefix string) error {
	return nil
}

func (h *HNSWVectorStore) Search(queryVec []float64, limit int) ([]VectorSearchResult, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if !h.loaded || len(h.index.nodes) == 0 {
		return h.inner.Search(queryVec, limit)
	}

	h.index.mu.RLock()
	defer h.index.mu.RUnlock()

	ef := h.index.efSearch
	if ef < limit {
		ef = limit
	}

	if len(h.index.nodes) == 0 || h.index.entryID == -1 {
		return nil, nil
	}

	entryNode, ok := h.index.nodes[h.index.entryID]
	if !ok {
		return nil, nil
	}

	visited := make(map[int]bool)
	candidates := make(heapItems, 0, ef+1)
	resultSet := make(heapItems, 0, ef+1)

	entryDist := 1 - utils.CosineSimilarity(queryVec, entryNode.vec)
	heapPush(&candidates, heapItem{id: h.index.entryID, dist: entryDist})
	heapPush(&resultSet, heapItem{id: h.index.entryID, dist: entryDist})
	visited[h.index.entryID] = true

	for candidates.Len() > 0 {
		c := heapPop(&candidates)
		if len(resultSet) == 0 {
			break
		}
		furthest := resultSet[0]

		if c.dist > furthest.dist {
			break
		}

		node, ok := h.index.nodes[c.id]
		if !ok {
			continue
		}

		for lvl := 0; lvl <= node.level; lvl++ {
			for _, neighborID := range node.links[lvl] {
				if visited[neighborID] {
					continue
				}
				visited[neighborID] = true

				neighbor, ok := h.index.nodes[neighborID]
				if !ok {
					continue
				}

				dist := 1 - utils.CosineSimilarity(queryVec, neighbor.vec)
				if len(resultSet) > 0 {
					furthest = resultSet[0]
				}

				if len(resultSet) == 0 || dist < furthest.dist || resultSet.Len() < ef {
					heapPush(&candidates, heapItem{id: neighborID, dist: dist})
					heapPush(&resultSet, heapItem{id: neighborID, dist: dist})
					if resultSet.Len() > ef {
						heapPop(&resultSet)
					}
				}
			}
		}
	}

	results := make([]VectorSearchResult, 0, limit)
	sort.Slice(resultSet, func(i, j int) bool {
		return resultSet[i].dist < resultSet[j].dist
	})
	for i := 0; i < resultSet.Len() && i < limit; i++ {
		item := resultSet[i]
		node, ok := h.index.nodes[item.id]
		if !ok {
			continue
		}
		score := 1 - item.dist
		minScore := config.AppConfig.AI.VectorMinScore
		if minScore <= 0 {
			minScore = 0.3 // fallback
		}
		if score > minScore {
			results = append(results, VectorSearchResult{
				DocID:   item.id,
				SpaceID: node.spaceID,
				Name:    node.name,
				Score:   score,
			})
		}
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results, nil
}

type heapItem struct {
	id   int
	dist float64
}

type heapItems []heapItem

func (h heapItems) Len() int           { return len(h) }
func (h heapItems) Less(i, j int) bool { return h[i].dist < h[j].dist }
func (h heapItems) Swap(i, j int)      { h[i], h[j] = h[j], h[i] }

func heapPush(h *heapItems, item heapItem) {
	*h = append(*h, item)
	siftUp(*h, len(*h)-1)
}

func heapPop(h *heapItems) heapItem {
	items := *h
	n := len(items)
	item := items[0]
	items[0], items[n-1] = items[n-1], items[0]
	*h = items[:n-1]
	if len(*h) > 0 {
		siftDown(*h, 0)
	}
	return item
}

func siftUp(h heapItems, i int) {
	for i > 0 {
		parent := (i - 1) / 2
		if h[parent].dist <= h[i].dist {
			break
		}
		h[parent], h[i] = h[i], h[parent]
		i = parent
	}
}

func siftDown(h heapItems, i int) {
	n := len(h)
	for {
		smallest := i
		left := 2*i + 1
		right := 2*i + 2
		if left < n && h[left].dist < h[smallest].dist {
			smallest = left
		}
		if right < n && h[right].dist < h[smallest].dist {
			smallest = right
		}
		if smallest == i {
			break
		}
		h[i], h[smallest] = h[smallest], h[i]
		i = smallest
	}
}

func (h *HNSWVectorStore) DocCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if h.loaded {
		return len(h.index.nodes)
	}
	return h.inner.DocCount()
}

func (h *HNSWVectorStore) Close() error {
	return h.inner.Close()
}

func (h *HNSWVectorStore) insertToIndex(docID int, spaceID int, name string, vec []float64) {
	h.index.mu.Lock()
	defer h.index.mu.Unlock()

	// P2-#36: 使用 HNSW 论文标准的几何分布层级分配
	// level = floor(-ln(uniform(0,1)) * mL), where mL = 1/ln(M)
	// 独立于当前 maxLevel，保证层级分布稳定
	level := 0
	if len(h.index.nodes) > 0 {
		mL := 1.0 / math.Log(float64(hnswM))
		level = int(-math.Log(rand.Float64()+1e-10) * mL)
		if level > hnswMaxLevel {
			level = hnswMaxLevel
		}
	}

	node := &hnswNode{
		id:      docID,
		vec:     vec,
		level:   level,
		links:   make([][]int, level+1),
		spaceID: spaceID,
		name:    name,
	}
	for i := 0; i <= level; i++ {
		node.links[i] = make([]int, 0, h.index.m)
	}

	if h.index.entryID == -1 {
		h.index.entryID = docID
		h.index.maxLevel = level
		h.index.nodes[docID] = node
		return
	}

	if len(h.index.nodes) > 0 {
		neighbors := h.findNearest(docID, vec, h.index.m)
		for _, neighborID := range neighbors {
			if n, ok := h.index.nodes[neighborID]; ok {
				// 连接层级不能超过任一节点的层级：n.links 长度为 n.level+1，
				// 用新节点 level 索引会越界（panic: index out of range）
				connLevel := min(level, n.level)
				if len(n.links[connLevel]) < h.index.m {
					n.links[connLevel] = append(n.links[connLevel], docID)
					node.links[connLevel] = append(node.links[connLevel], neighborID)
				}
			}
		}
	}

	h.index.nodes[docID] = node
	if level > h.index.maxLevel {
		h.index.maxLevel = level
		h.index.entryID = docID
	}
}

func (h *HNSWVectorStore) findNearest(excludeID int, vec []float64, k int) []int {
	if len(h.index.nodes) == 0 || h.index.entryID == -1 {
		return nil
	}

	if len(h.index.nodes) <= h.index.m*2 {
		return h.bruteForceNearest(excludeID, vec, k)
	}

	visited := make(map[int]bool)
	visited[excludeID] = true
	candidates := make(heapItems, 0, k+1)
	resultSet := make(heapItems, 0, k+1)

	entryDist := 1 - utils.CosineSimilarity(vec, h.index.nodes[h.index.entryID].vec)
	heapPush(&candidates, heapItem{id: h.index.entryID, dist: entryDist})
	heapPush(&resultSet, heapItem{id: h.index.entryID, dist: entryDist})
	visited[h.index.entryID] = true

	ef := h.index.efSearch
	if ef < k {
		ef = k
	}

	for candidates.Len() > 0 {
		c := heapPop(&candidates)
		if len(resultSet) > 0 && resultSet.Len() >= k && c.dist > resultSet[0].dist {
			break
		}

		node, ok := h.index.nodes[c.id]
		if !ok {
			continue
		}

		for lvl := 0; lvl <= node.level; lvl++ {
			for _, neighborID := range node.links[lvl] {
				if visited[neighborID] || neighborID == excludeID {
					continue
				}
				visited[neighborID] = true

				neighbor, ok := h.index.nodes[neighborID]
				if !ok {
					continue
				}

				dist := 1 - utils.CosineSimilarity(vec, neighbor.vec)
				if len(resultSet) == 0 || resultSet.Len() < k || dist < resultSet[0].dist {
					heapPush(&candidates, heapItem{id: neighborID, dist: dist})
					heapPush(&resultSet, heapItem{id: neighborID, dist: dist})
					if resultSet.Len() > ef {
						heapPop(&resultSet)
					}
				}
			}
		}
	}

	result := make([]int, 0, k)
	sort.Slice(resultSet, func(i, j int) bool {
		return resultSet[i].dist < resultSet[j].dist
	})
	for i := 0; i < resultSet.Len() && len(result) < k; i++ {
		if resultSet[i].id != excludeID {
			result = append(result, resultSet[i].id)
		}
	}
	return result
}

func (h *HNSWVectorStore) bruteForceNearest(excludeID int, vec []float64, k int) []int {
	type scoredNode struct {
		id    int
		score float64
	}
	var scoredNodes []scoredNode
	for id, node := range h.index.nodes {
		if id == excludeID {
			continue
		}
		score := utils.CosineSimilarity(vec, node.vec)
		scoredNodes = append(scoredNodes, scoredNode{id: id, score: score})
	}
	sort.Slice(scoredNodes, func(i, j int) bool {
		return scoredNodes[i].score > scoredNodes[j].score
	})
	if len(scoredNodes) > k {
		scoredNodes = scoredNodes[:k]
	}
	result := make([]int, len(scoredNodes))
	for i, s := range scoredNodes {
		result[i] = s.id
	}
	return result
}

// LoadFromBolt 从BBolt加载所有向量到内存索引
func (h *HNSWVectorStore) LoadFromBolt() error {
	if h.inner == nil || h.inner.db == nil {
		return nil
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	h.index = newHNSWIndex()

	// 读取所有向量
	vectors, err := h.inner.ReadAllVectors()
	if err != nil {
		return err
	}

	for _, dv := range vectors {
		h.insertToIndex(dv.DocID, dv.SpaceID, dv.Name, dv.Vector)
	}

	h.loaded = true
	logx.Infof("[HNSW] Loaded %d vectors into memory index", len(vectors))
	return nil
}

// StartAutoLoad 启动时自动加载
func (h *HNSWVectorStore) StartAutoLoad() {
	go func() {
		if err := h.LoadFromBolt(); err != nil {
			logx.Infof("[HNSW] Auto-load failed: %v", err)
		}
	}()
}

// Ensure HNSWVectorStore implements VectorStoreInterface
var _ VectorStoreInterface = (*HNSWVectorStore)(nil)
