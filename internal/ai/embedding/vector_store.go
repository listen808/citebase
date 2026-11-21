package embedding

import (
	"bytes"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/config"
	"go.etcd.io/bbolt"
)

// bucket name is now per-instance (VectorStore.bucketName field)

type DocVector struct {
	DocID   int       `json:"doc_id"`
	SpaceID int       `json:"space_id"`
	Name    string    `json:"name"`
	Vector  []float64 `json:"vector"`
	Content string    `json:"content,omitempty"` // Phase2: 用于索引的文档内容
}

type VectorSearchResult struct {
	DocID   int     `json:"doc_id"`
	SpaceID int     `json:"space_id"`
	Name    string  `json:"name"`
	Score   float64 `json:"score"`
}

type VectorStore struct {
	db         *bbolt.DB
	mu         sync.RWMutex
	path       string
	bucketName string
}

// NewVectorStore creates a new VectorStore with the given BoltDB instance and bucket name.
func NewVectorStore(db *bbolt.DB, dataDir string, bucketName string) *VectorStore {
	return &VectorStore{db: db, path: dataDir, bucketName: bucketName}
}

var (
	vecStore      *VectorStore
	vecStoreOnce  sync.Once
	vecStoreIface VectorStoreInterface
)

func InitVectorStore(dataDir string) error {
	var initErr error
	vecStoreOnce.Do(func() {
		storeType := config.AppConfig.AI.VectorStoreType

		if storeType == "chroma" {
			endpoint := config.AppConfig.AI.ChromaEndpoint
			vecStoreIface = NewChromaVectorStore(endpoint)
			logx.Infof("[VectorStore] Initialized Chroma at: %s", endpoint)
			return
		}

		if storeType == "qdrant" {
			endpoint := config.AppConfig.AI.QdrantEndpoint
			apiKey := config.AppConfig.AI.QdrantApiKey
			vecStoreIface = NewQdrantVectorStore(endpoint, apiKey)
			logx.Infof("[VectorStore] Initialized Qdrant at: %s", endpoint)
			return
		}

		if err := os.MkdirAll(dataDir, 0755); err != nil {
			initErr = fmt.Errorf("failed to create vector dir: %w", err)
			return
		}
		dbPath := filepath.Join(dataDir, "vectors.bolt")
		db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 5 * time.Second})
		if err != nil {
			initErr = fmt.Errorf("failed to open vector db: %w", err)
			return
		}
		vecStore = NewVectorStore(db, dataDir, "doc_vectors")
		hnswStore := NewHNSWVectorStore(vecStore)
		vecStoreIface = hnswStore
		hnswStore.StartAutoLoad()
		logx.Infof("[VectorStore] Initialized BoltDB+HNSW at: %s", dbPath)
	})
	return initErr
}

func GetVectorStore() *VectorStore {
	return vecStore
}

func GetVectorStoreInterface() VectorStoreInterface {
	return vecStoreIface
}

func (vs *VectorStore) Close() error {
	if vs.db != nil {
		return vs.db.Close()
	}
	return nil
}

func (vs *VectorStore) StoreVector(docID int, spaceID int, name string, vector []float64) error {
	if vs.db == nil || len(vector) == 0 {
		return nil
	}
	vs.mu.Lock()
	defer vs.mu.Unlock()

	dv := DocVector{DocID: docID, SpaceID: spaceID, Name: name, Vector: vector}
	data, err := json.Marshal(dv)
	if err != nil {
		return err
	}

	key := fmt.Sprintf("%d", docID)
	return vs.db.Update(func(tx *bbolt.Tx) error {
		b, err := tx.CreateBucketIfNotExists([]byte(vs.bucketName))
		if err != nil {
			return err
		}
		return b.Put([]byte(key), data)
	})
}

func (vs *VectorStore) DeleteVector(docID int) error {
	if vs.db == nil {
		return nil
	}
	vs.mu.Lock()
	defer vs.mu.Unlock()

	key := fmt.Sprintf("%d", docID)
	return vs.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(vs.bucketName))
		if b == nil {
			return nil
		}
		return b.Delete([]byte(key))
	})
}

// DeleteVectorsBySpaceID deletes all vectors belonging to a specific space
func (vs *VectorStore) DeleteVectorsBySpaceID(spaceID int) error {
	if vs.db == nil {
		return nil
	}
	vs.mu.Lock()
	defer vs.mu.Unlock()

	return vs.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(vs.bucketName))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			var dv DocVector
			if err := json.Unmarshal(v, &dv); err == nil && dv.SpaceID == spaceID {
				if err := c.Delete(); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func (vs *VectorStore) DeleteVectorsByPrefix(prefix string) error {
	if vs.db == nil {
		return nil
	}
	vs.mu.Lock()
	defer vs.mu.Unlock()

	return vs.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(vs.bucketName))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, _ := c.Seek([]byte(prefix)); k != nil && bytes.HasPrefix(k, []byte(prefix)); k, _ = c.Next() {
			if err := c.Delete(); err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteDocVectors 删除文档及其所有 chunk 向量
// chunk key 格式为 docID*1000+i（i 从 0 到 maxChunks-1）
func (vs *VectorStore) DeleteDocVectors(docID int, maxChunks int) error {
	if vs.db == nil {
		return nil
	}
	vs.mu.Lock()
	defer vs.mu.Unlock()

	return vs.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(vs.bucketName))
		if b == nil {
			return nil
		}
		// 删除主文档向量
		_ = b.Delete([]byte(fmt.Sprintf("%d", docID)))
		// 删除所有 chunk 向量
		for i := 0; i < maxChunks; i++ {
			_ = b.Delete([]byte(fmt.Sprintf("%d", docID*1000+i)))
		}
		return nil
	})
}

func (vs *VectorStore) Search(queryVec []float64, limit int) ([]VectorSearchResult, error) {
	if vs.db == nil || len(queryVec) == 0 {
		return nil, nil
	}
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	var results []VectorSearchResult
	err := vs.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(vs.bucketName))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var dv DocVector
			if err := json.Unmarshal(v, &dv); err != nil {
				return nil
			}
			score := utils.CosineSimilarity(queryVec, dv.Vector)
			if score > 0.3 {
				results = append(results, VectorSearchResult{
					DocID:   dv.DocID,
					SpaceID: dv.SpaceID,
					Name:    dv.Name,
					Score:   score,
				})
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

func (vs *VectorStore) DocCount() int {
	if vs.db == nil {
		return 0
	}
	count := 0
	_ = vs.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(vs.bucketName))
		if b == nil {
			return nil
		}
		c := b.Cursor()
		for k, _ := c.First(); k != nil; k, _ = c.Next() {
			count++
		}
		return nil
	})
	return count
}

// ReadAllVectors 读取所有向量（用于HNSW索引加载）
func (vs *VectorStore) ReadAllVectors() ([]DocVector, error) {
	if vs.db == nil {
		return nil, nil
	}
	vs.mu.RLock()
	defer vs.mu.RUnlock()

	var vectors []DocVector
	err := vs.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(vs.bucketName))
		if b == nil {
			return nil
		}
		return b.ForEach(func(k, v []byte) error {
			var dv DocVector
			if err := json.Unmarshal(v, &dv); err != nil {
				return nil
			}
			vectors = append(vectors, dv)
			return nil
		})
	})
	return vectors, err
}

func VectorSearch(question string, spaceID int, limit int, embedder Embedder) []VectorSearchResult {
	vsi := GetVectorStoreInterface()
	if vsi == nil {
		logx.Infof("[VectorSearch] Vector store not initialized, falling back to keyword")
		return nil
	}
	if vsi.DocCount() == 0 {
		logx.Infof("[VectorSearch] Vector store is empty (0 docs), falling back to keyword")
		return nil
	}
	if embedder == nil {
		logx.Infof("[VectorSearch] Embedder not configured, falling back to keyword")
		return nil
	}
	queryVec, err := embedder.GetEmbedding(question)
	if err != nil {
		logx.Infof("[VectorSearch] Embedding error (falling back to keyword): %v", err)
		return nil
	}
	results, err := vsi.Search(queryVec, limit*2)
	if err != nil {
		logx.Infof("[VectorSearch] Vector search error: %v", err)
		return nil
	}
	if spaceID > 0 {
		filtered := make([]VectorSearchResult, 0, len(results))
		for _, r := range results {
			if r.SpaceID == spaceID {
				filtered = append(filtered, r)
			}
		}
		results = filtered
	}
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

// ReindexDocuments 重建向量索引（批量Embedding优化）
func ReindexDocuments(docs []DocVector, embedder Embedder, batchSize int) error {
	vsi := GetVectorStoreInterface()
	if vsi == nil {
		return fmt.Errorf("vector store not initialized")
	}
	if embedder == nil {
		return fmt.Errorf("embedder not configured")
	}

	for i := 0; i < len(docs); i += batchSize {
		end := i + batchSize
		if end > len(docs) {
			end = len(docs)
		}
		batch := docs[i:end]

		texts := make([]string, len(batch))
		for j, d := range batch {
			indexContent := d.Name
			if d.Content != "" {
				runes := []rune(d.Content)
				if len(runes) > 2000 {
					indexContent = string(runes[:2000])
				} else {
					indexContent = d.Content
				}
			}
			texts[j] = indexContent
		}

		vecs, batchErr := embedder.GetEmbeddings(texts)
		if batchErr != nil {
			logx.Infof("[Reindex] Batch embedding error %d-%d, falling back: %v", i, end, batchErr)
			for _, d := range batch {
				indexContent := d.Name
				if d.Content != "" {
					runes := []rune(d.Content)
					if len(runes) > 2000 {
						indexContent = string(runes[:2000])
					} else {
						indexContent = d.Content
					}
				}
				vec, err := embedder.GetEmbedding(indexContent)
				if err != nil {
					logx.Infof("[Reindex] Embedding error for doc %d: %v", d.DocID, err)
					continue
				}
				if err := vsi.StoreVector(d.DocID, d.SpaceID, d.Name, vec); err != nil {
					logx.Infof("[Reindex] Store error for doc %d: %v", d.DocID, err)
				}
			}
		} else {
			for j, vec := range vecs {
				if len(vec) == 0 {
					continue
				}
				d := batch[j]
				if err := vsi.StoreVector(d.DocID, d.SpaceID, d.Name, vec); err != nil {
					logx.Infof("[Reindex] Store error for doc %d: %v", d.DocID, err)
				}
			}
		}
		logx.Infof("[Reindex] Batch %d-%d/%d completed", i, end, len(docs))
	}
	return nil
}
