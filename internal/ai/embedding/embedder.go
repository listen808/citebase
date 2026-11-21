package embedding

import (
	"context"
	"fmt"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

type Embedder interface {
	GetEmbedding(text string) ([]float64, error)
	GetEmbeddings(texts []string) ([][]float64, error)
	Name() string
}

type ProviderEmbedder struct {
	provider provider.Provider
}

func NewEmbedder(p provider.Provider) Embedder {
	return &ProviderEmbedder{provider: p}
}

func (e *ProviderEmbedder) Name() string {
	return "provider-embedding"
}

func (e *ProviderEmbedder) GetEmbedding(text string) ([]float64, error) {
	if len(text) > 8000 {
		text = utils.TruncateRunes(text, 8000)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	vec, err := e.provider.GetEmbedding(ctx, text)
	if err != nil {
		return nil, fmt.Errorf("provider embedding: %w", err)
	}
	return vec, nil
}

func (e *ProviderEmbedder) GetEmbeddings(texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, fmt.Errorf("empty texts")
	}
	truncated := make([]string, len(texts))
	for i, t := range texts {
		if len(t) > 8000 {
			truncated[i] = utils.TruncateRunes(t, 8000)
		} else {
			truncated[i] = t
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	vecs, err := e.provider.GetEmbeddings(ctx, truncated)
	if err != nil {
		return nil, fmt.Errorf("provider embeddings: %w", err)
	}
	return vecs, nil
}
