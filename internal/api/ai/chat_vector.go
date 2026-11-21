package ai

import (
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	aimod "github.com/listen808/citebase/internal/ai"
	"github.com/listen808/citebase/internal/middleware"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
	config "github.com/listen808/citebase/internal/utils/config"
)

// 向量索引管理：统计 / 同步通知 / 重建索引。

func (api *ChatAPI) VectorStats(c *gin.Context) {
	m := aimod.GetModule()
	vs := m.VectorStore()
	if vs == nil {
		response.Success(c, gin.H{"enabled": false, "embedder_ready": false, "doc_count": 0, "reason": "vector store not initialized"})
		return
	}

	pending, _ := knowledgerepo.VectorSyncTaskRepo().CountByStatus("pending")
	processing, _ := knowledgerepo.VectorSyncTaskRepo().CountByStatus("processing")
	done, _ := knowledgerepo.VectorSyncTaskRepo().CountByStatus("done")
	failed, _ := knowledgerepo.VectorSyncTaskRepo().CountByStatus("failed")
	dead, _ := knowledgerepo.VectorSyncTaskRepo().CountByStatus("dead")

	recentFailed, _ := knowledgerepo.VectorSyncTaskRepo().FetchByStatus("dead", 10)
	var recentErrors []gin.H
	for _, t := range recentFailed {
		recentErrors = append(recentErrors, gin.H{
			"doc_id":     t.DocID,
			"doc_name":   t.DocName,
			"status":     t.Status,
			"last_error": t.LastError,
			"retry":      t.RetryCount,
		})
	}

	// 向量配置信息（从 config 读取，避免前端硬编码）
	embeddingModel := ""
	if config.AppConfig.AI.Embedding != nil {
		embeddingModel = config.AppConfig.AI.Embedding.Model
	}
	vectorStoreType := config.AppConfig.AI.VectorStoreType
	if vectorStoreType == "" {
		vectorStoreType = "bolt"
	}

	response.Success(c, gin.H{
		"enabled":           m.Enabled(),
		"doc_count":         vs.DocCount(),
		"embedder_ready":    m.Embedder() != nil,
		"syncer_ready":      m.VectorSyncer() != nil,
		"embedding_model":   embeddingModel,
		"vector_store_type": vectorStoreType,
		"tasks": gin.H{
			"pending":    pending,
			"processing": processing,
			"done":       done,
			"failed":     failed,
			"dead":       dead,
		},
		"recent_errors": recentErrors,
	})
}

func (api *ChatAPI) VectorSyncNotify(c *gin.Context) {
	var req struct {
		DocID   int    `json:"doc_id" binding:"required"`
		SpaceID int    `json:"space_id"`
		Name    string `json:"name"`
		Content string `json:"content"`
		OpType  string `json:"op_type" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "doc_id and op_type are required")
		return
	}

	m := aimod.GetModule()
	vs := m.VectorSyncer()
	if vs == nil {
		response.Success(c, gin.H{"synced": false})
		return
	}

	vs.Enqueue(aimod.VectorSyncOp{
		DocID:   req.DocID,
		SpaceID: req.SpaceID,
		DocName: req.Name,
		Content: req.Content,
		OpType:  req.OpType,
	})

	response.Success(c, gin.H{"synced": true})
}

func (api *ChatAPI) ReindexVectors(c *gin.Context) {
	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}
	if m.Embedder() == nil {
		response.ServerErrorKey(c, response.MsgAIEmbeddingUnconfigured)
		return
	}

	startTime := time.Now()

	userID, _ := middleware.GetUserID(c)

	providerName := ""
	modelName := ""
	if p := m.PrimaryProvider(); p != nil {
		providerName = p.Name()
	}
	if cfg := config.AppConfig.AI.Primary; cfg != nil {
		modelName = cfg.Model
	} else if config.AppConfig.AI.Model != "" {
		modelName = config.AppConfig.AI.Model
	}

	go func() {
		count, err := workspacesvc.ReindexAllVectors()
		durationMs := time.Since(startTime).Milliseconds()

		resultStr := "success"
		reasonStr := ""
		if err != nil {
			resultStr = "failed"
			reasonStr = err.Error()
		}

		RecordAIAudit(nil, userID, "reindex_vectors", "", "重建向量索引", fmt.Sprintf("处理文档数: %d", count), modelName, providerName, resultStr, reasonStr, 0, 0, durationMs)

		if err != nil {
			logx.Infof("[ChatAPI] Vector reindex failed: %v", err)
		} else {
			logx.Infof("[ChatAPI] Vector reindex completed: %d documents", count)
		}
	}()

	response.SuccessKey(c, nil, response.MsgVectorReindexStarted)
}

func (api *ChatAPI) ReindexSpaceVectors(c *gin.Context) {
	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}
	if m.Embedder() == nil {
		response.ServerErrorKey(c, response.MsgAIEmbeddingUnconfigured)
		return
	}

	spaceIDStr := c.Param("space_id")
	spaceID, err := strconv.Atoi(spaceIDStr)
	if err != nil || spaceID <= 0 {
		response.BadRequest(c, "invalid space_id")
		return
	}

	go func() {
		count, err := workspacesvc.ReindexSpaceVectors(spaceID)
		if err != nil {
			logx.Infof("[ChatAPI] Space vector reindex failed: space=%d err=%v", spaceID, err)
		} else {
			logx.Infof("[ChatAPI] Space vector reindex completed: space=%d docs=%d", spaceID, count)
		}
	}()

	response.Success(c, gin.H{
		"message": fmt.Sprintf("空间 %d 向量索引重建已启动", spaceID),
	})
}

func (api *ChatAPI) ReindexDocVector(c *gin.Context) {
	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}
	if m.Embedder() == nil {
		response.ServerErrorKey(c, response.MsgAIEmbeddingUnconfigured)
		return
	}

	docIDStr := c.Param("doc_id")
	docID, err := strconv.Atoi(docIDStr)
	if err != nil || docID <= 0 {
		response.BadRequest(c, "invalid doc_id")
		return
	}

	go func() {
		if err := workspacesvc.ReindexDocVector(docID); err != nil {
			logx.Infof("[ChatAPI] Doc vector reindex failed: doc=%d err=%v", docID, err)
		} else {
			logx.Infof("[ChatAPI] Doc vector reindex completed: doc=%d", docID)
		}
	}()

	response.Success(c, gin.H{
		"message": fmt.Sprintf("文档 %d 向量索引重建已启动", docID),
	})
}

// ReindexVectorsUnified 向量重建统一入口（P2-3）：
// POST /ai/vector/reindex/unified
// body: {scope: "all"|"space"|"doc", target_id?: int}
// 旧的 3 个端点（/reindex, /reindex/space/:space_id, /reindex/doc/:doc_id）仍保留作为 deprecated 兼容。
func (api *ChatAPI) ReindexVectorsUnified(c *gin.Context) {
	var req struct {
		Scope    string `json:"scope" binding:"required"`
		TargetID int    `json:"target_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "scope is required (all|space|doc)")
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}
	if m.Embedder() == nil {
		response.ServerErrorKey(c, response.MsgAIEmbeddingUnconfigured)
		return
	}

	switch req.Scope {
	case "all":
		api.reindexAllInternal(c)
	case "space":
		if req.TargetID <= 0 {
			response.BadRequest(c, "target_id is required for scope=space")
			return
		}
		api.reindexSpaceInternal(c, req.TargetID)
	case "doc":
		if req.TargetID <= 0 {
			response.BadRequest(c, "target_id is required for scope=doc")
			return
		}
		api.reindexDocInternal(c, req.TargetID)
	default:
		response.BadRequest(c, "invalid scope: must be all|space|doc")
	}
}

func (api *ChatAPI) reindexAllInternal(c *gin.Context) {
	startTime := time.Now()
	userID, _ := middleware.GetUserID(c)
	m := aimod.GetModule()
	providerName := ""
	modelName := ""
	if p := m.PrimaryProvider(); p != nil {
		providerName = p.Name()
	}
	if cfg := config.AppConfig.AI.Primary; cfg != nil {
		modelName = cfg.Model
	} else if config.AppConfig.AI.Model != "" {
		modelName = config.AppConfig.AI.Model
	}

	go func() {
		count, err := workspacesvc.ReindexAllVectors()
		durationMs := time.Since(startTime).Milliseconds()
		resultStr := "success"
		reasonStr := ""
		if err != nil {
			resultStr = "failed"
			reasonStr = err.Error()
		}
		RecordAIAudit(nil, userID, "reindex_vectors", "", "重建向量索引", fmt.Sprintf("处理文档数: %d", count), modelName, providerName, resultStr, reasonStr, 0, 0, durationMs)
		if err != nil {
			logx.Infof("[ChatAPI] Vector reindex failed: %v", err)
		} else {
			logx.Infof("[ChatAPI] Vector reindex completed: %d documents", count)
		}
	}()

	response.SuccessKey(c, nil, response.MsgVectorReindexStarted)
}

func (api *ChatAPI) reindexSpaceInternal(c *gin.Context, spaceID int) {
	go func() {
		count, err := workspacesvc.ReindexSpaceVectors(spaceID)
		if err != nil {
			logx.Infof("[ChatAPI] Space vector reindex failed: space=%d err=%v", spaceID, err)
		} else {
			logx.Infof("[ChatAPI] Space vector reindex completed: space=%d docs=%d", spaceID, count)
		}
	}()
	response.Success(c, gin.H{"message": fmt.Sprintf("空间 %d 向量索引重建已启动", spaceID)})
}

func (api *ChatAPI) reindexDocInternal(c *gin.Context, docID int) {
	go func() {
		if err := workspacesvc.ReindexDocVector(docID); err != nil {
			logx.Infof("[ChatAPI] Doc vector reindex failed: doc=%d err=%v", docID, err)
		} else {
			logx.Infof("[ChatAPI] Doc vector reindex completed: doc=%d", docID)
		}
	}()
	response.Success(c, gin.H{"message": fmt.Sprintf("文档 %d 向量索引重建已启动", docID)})
}
