package schema

import (
	"fmt"

	"github.com/listen808/citebase/internal/ai/embedding"
)

type VectorStore struct {
	schemaVS *embedding.SchemaVectorStore
}

func NewVectorStore(schemaVS *embedding.SchemaVectorStore) *VectorStore {
	return &VectorStore{
		schemaVS: schemaVS,
	}
}

func (s *VectorStore) IsReady() bool {
	return s.schemaVS != nil
}

func (s *VectorStore) AddChunks(schemaSpaceID int, chunks []*SchemaChunk) error {
	if !s.IsReady() {
		return fmt.Errorf("vector store not ready")
	}

	embChunks := make([]embedding.SchemaChunk, len(chunks))
	for i, c := range chunks {
		embChunks[i] = embedding.SchemaChunk{
			ChunkType:     c.ChunkType,
			SchemaSpaceID: c.SchemaSpaceID,
			TableName:     c.TableName,
			ColumnName:    c.ColumnName,
			Content:       c.Content,
			Metadata:      c.Metadata,
		}
	}

	return s.schemaVS.AddChunks(schemaSpaceID, embChunks)
}

type SearchResult struct {
	ChunkType  string
	TableName  string
	ColumnName string
	Content    string
	Score      float64
}

func (s *VectorStore) Search(schemaSpaceID int, query string, topK int) ([]*SearchResult, error) {
	if !s.IsReady() {
		return nil, fmt.Errorf("vector store not ready")
	}

	results, err := s.schemaVS.Search(query, topK*3)
	if err != nil {
		return nil, fmt.Errorf("vector search failed: %w", err)
	}

	var searchResults []*SearchResult
	seenTables := make(map[string]bool)

	for _, r := range results {
		if len(r.Name) < 7 || r.Name[:7] != "schema:" {
			continue
		}

		parts := splitName(r.Name)
		if len(parts) < 3 {
			continue
		}

		chunkType := parts[1]
		tableName := parts[2]
		columnName := ""
		if len(parts) > 3 {
			columnName = parts[3]
		}

		key := tableName + ":" + chunkType
		if seenTables[key] {
			continue
		}
		seenTables[key] = true

		searchResults = append(searchResults, &SearchResult{
			ChunkType:  chunkType,
			TableName:  tableName,
			ColumnName: columnName,
			Score:      r.Score,
		})

		if len(searchResults) >= topK {
			break
		}
	}

	return searchResults, nil
}

func (s *VectorStore) ClearCollection(schemaSpaceID int) error {
	if !s.IsReady() {
		return nil
	}
	return s.schemaVS.ClearCollection(schemaSpaceID)
}

func splitName(name string) []string {
	var result []string
	var current string
	for _, ch := range name {
		if ch == ':' {
			result = append(result, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		result = append(result, current)
	}
	return result
}
