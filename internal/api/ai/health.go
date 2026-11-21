package ai

import (
	"github.com/gin-gonic/gin"
	aimod "github.com/listen808/citebase/internal/ai"
	"github.com/listen808/citebase/internal/response"
)

// Health AI 健康检查
func (api *ChatAPI) Health(c *gin.Context) {
	m := aimod.GetModule()

	enabled := m.Enabled()
	primaryOK := m.PrimaryProvider() != nil
	embeddingOK := m.Embedder() != nil
	retrieverOK := m.Retriever() != nil
	vectorStoreOK := m.VectorStore() != nil

	overall := "UP"
	if !enabled || !primaryOK {
		overall = "DOWN"
	} else if !embeddingOK || !retrieverOK || !vectorStoreOK {
		overall = "DEGRADED"
	}

	response.Success(c, gin.H{
		"enabled":              enabled,
		"primary_provider":     primaryOK,
		"embedding_configured": embeddingOK,
		"retriever_ready":      retrieverOK,
		"vector_store_ready":   vectorStoreOK,
		"overall":              overall,
	})
}
