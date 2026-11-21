package embedding

import (
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"os"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"
)

type SchemaChunk struct {
	ChunkType     string            `json:"chunk_type"`
	SchemaSpaceID int               `json:"schema_space_id"`
	TableName     string            `json:"table_name"`
	ColumnName    string            `json:"column_name,omitempty"`
	Content       string            `json:"content"`
	Metadata      map[string]string `json:"metadata"`
}

type SchemaSearchResult struct {
	DocID   int     `json:"doc_id"`
	SpaceID int     `json:"space_id"`
	Name    string  `json:"name"`
	Score   float64 `json:"score"`
}

type SchemaVectorStore struct {
	store        *VectorStore
	hnsw         *HNSWVectorStore
	embedder     Embedder
	closedBoltDB *bbolt.DB
}

func NewSchemaVectorStore(dataDir string, emb Embedder) *SchemaVectorStore {
	if emb == nil {
		return nil
	}

	if err := os.MkdirAll(dataDir, 0755); err != nil {
		logx.Infof("[SchemaVectorStore] Failed to create dir: %v", err)
		return nil
	}

	dbPath := filepath.Join(dataDir, "schema_vectors.bolt")
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 5 * time.Second})
	if err != nil {
		logx.Infof("[SchemaVectorStore] Failed to open schema vector db: %v", err)
		return nil
	}

	store := NewVectorStore(db, dataDir, "schema_vectors")
	hnswStore := NewHNSWVectorStore(store)
	hnswStore.StartAutoLoad()

	logx.Infof("[SchemaVectorStore] Initialized BoltDB+HNSW at: %s", dbPath)

	return &SchemaVectorStore{
		store:        store,
		hnsw:         hnswStore,
		embedder:     emb,
		closedBoltDB: db,
	}
}

func (s *SchemaVectorStore) AddChunks(schemaSpaceID int, chunks []SchemaChunk) error {
	if s.embedder == nil {
		return fmt.Errorf("embedder not configured")
	}

	tableSeqMap := make(map[string]int)
	tableSeq := 0
	for _, chunk := range chunks {
		if _, ok := tableSeqMap[chunk.TableName]; !ok {
			tableSeqMap[chunk.TableName] = tableSeq
			tableSeq++
		}
	}

	tableChunkSeq := make(map[string]int)

	for _, chunk := range chunks {
		tSeq := tableSeqMap[chunk.TableName]
		cSeq := tableChunkSeq[chunk.TableName]
		tableChunkSeq[chunk.TableName] = cSeq + 1

		docID := schemaSpaceID*1000000 + tSeq*1000 + cSeq

		vector, err := s.embedder.GetEmbedding(chunk.Content)
		if err != nil {
			logx.Infof("[SchemaVectorStore] Failed to embed chunk: %v", err)
			continue
		}

		name := fmt.Sprintf("schema:%s:%s:%s", chunk.ChunkType, chunk.TableName, chunk.ColumnName)

		if err := s.store.StoreVector(docID, schemaSpaceID, name, vector); err != nil {
			logx.Infof("[SchemaVectorStore] Failed to store vector: %v", err)
		}
	}

	return nil
}

func (s *SchemaVectorStore) Search(query string, topK int) ([]SchemaSearchResult, error) {
	if s == nil || s.hnsw == nil {
		return nil, fmt.Errorf("schema vector store not initialized")
	}

	if s.embedder == nil {
		return nil, fmt.Errorf("embedder not configured")
	}

	queryVec, err := s.embedder.GetEmbedding(query)
	if err != nil {
		return nil, fmt.Errorf("failed to embed query: %w", err)
	}

	results, err := s.hnsw.Search(queryVec, topK*3)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	var searchResults []SchemaSearchResult
	for _, r := range results {
		if len(r.Name) < 7 || r.Name[:7] != "schema:" {
			continue
		}
		searchResults = append(searchResults, SchemaSearchResult{
			DocID:   r.DocID,
			SpaceID: r.SpaceID,
			Name:    r.Name,
			Score:   r.Score,
		})
	}

	return searchResults, nil
}

func (s *SchemaVectorStore) ClearCollection(schemaSpaceID int) error {
	return s.store.DeleteVectorsBySpaceID(schemaSpaceID)
}

func (s *SchemaVectorStore) DocCount() int {
	return s.store.DocCount()
}

func (s *SchemaVectorStore) Close() error {
	if s.closedBoltDB != nil {
		err := s.closedBoltDB.Close()
		s.closedBoltDB = nil
		return err
	}
	return nil
}
