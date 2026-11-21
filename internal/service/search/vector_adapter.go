package search

import (
	"context"
	"fmt"

	"github.com/listen808/citebase/internal/ai/embedding"
	logx "github.com/listen808/citebase/internal/log"
)

// VectorAdapter wraps the existing embedding system and vector store
// to implement VectorSearcher.
type VectorAdapter struct {
	embedder embedding.Embedder
	store    embedding.VectorStoreInterface
}

// NewVectorAdapter creates a VectorAdapter from the global embedding infrastructure.
// Returns nil if the embedder or vector store is not available.
func NewVectorAdapter(embedder embedding.Embedder, store embedding.VectorStoreInterface) *VectorAdapter {
	if embedder == nil || store == nil {
		return nil
	}
	return &VectorAdapter{
		embedder: embedder,
		store:    store,
	}
}

// Embed converts text to an embedding vector using the configured embedder.
func (a *VectorAdapter) Embed(ctx context.Context, text string) ([]float64, error) {
	if a.embedder == nil {
		return nil, fmt.Errorf("embedder not configured")
	}
	vec, err := a.embedder.GetEmbedding(text)
	if err != nil {
		return nil, fmt.Errorf("vector embed: %w", err)
	}
	return vec, nil
}

// Search performs a vector similarity search using a pre-computed query embedding.
func (a *VectorAdapter) Search(ctx context.Context, queryEmbedding []float64, topK int) ([]VectorHit, error) {
	if a.store == nil {
		return nil, fmt.Errorf("vector store not initialized")
	}
	if len(queryEmbedding) == 0 {
		return nil, fmt.Errorf("empty query embedding")
	}

	if topK <= 0 {
		topK = 20
	}

	results, err := a.store.Search(queryEmbedding, topK)
	if err != nil {
		logx.Infof("[VectorAdapter] Search error: %v", err)
		return nil, err
	}

	hits := make([]VectorHit, 0, len(results))
	for _, r := range results {
		hits = append(hits, VectorHit{
			DocumentID: uint(r.DocID),
			SpaceID:    uint(r.SpaceID),
			Score:      r.Score,
		})
	}

	return hits, nil
}
