package ai

import (
	"fmt"

	"github.com/gin-gonic/gin"
	aimod "github.com/listen808/citebase/internal/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/response"
	aiSvc "github.com/listen808/citebase/internal/service/ai"
)

// MemoryAPI 记忆系统API
type MemoryAPI struct {
	repo   *airepo.AIMemoryRepository
	memSvc *aiSvc.MemoryService
}

func NewMemoryAPI() *MemoryAPI {
	repo := airepo.NewAIMemoryRepository()
	return &MemoryAPI{repo: repo, memSvc: aiSvc.NewMemoryService(repo)}
}

// ===== 请求/响应结构 =====

type createMemoryReq struct {
	Content      string   `json:"content" binding:"required"`
	MemoryType   string   `json:"memory_type"`
	Summary      string   `json:"summary"`
	SourceConvID uint     `json:"source_conv_id"`
	Tags         []string `json:"tags"`
	Importance   float64  `json:"importance"`
}

type searchMemoryReq struct {
	Query      string `json:"query"`
	MemoryType string `json:"memory_type"`
	Limit      int    `json:"limit"`
}

type extractMemoryReq struct {
	ConversationID uint                     `json:"conversation_id" binding:"required"`
	Messages       []map[string]interface{} `json:"messages" binding:"required"`
	UserID         uint                     `json:"user_id" binding:"required"`
}

// ===== CRUD API =====

// CreateMemory 创建记忆
func (api *MemoryAPI) CreateMemory(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	var req createMemoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "content is required")
		return
	}

	// 调用 MemoryService 创建记忆
	mem, err := api.memSvc.CreateMemory(userID, &aiSvc.CreateMemoryRequest{
		Content:      req.Content,
		MemoryType:   req.MemoryType,
		Summary:      req.Summary,
		SourceConvID: req.SourceConvID,
		Tags:         req.Tags,
		Importance:   req.Importance,
	})
	if err != nil {
		response.ServerErrorKey(c, response.MsgMemoryCreateFailed)
		return
	}
	response.Success(c, gin.H{"memory": mem})
}

// ListMemories 获取记忆列表
func (api *MemoryAPI) ListMemories(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	memoryType := c.Query("type")
	page := getPage(c)
	pageSize := getPageSize(c, 20)

	mems, total, err := api.memSvc.ListMemories(userID, memoryType, pageSize, (page-1)*pageSize)
	if err != nil {
		response.ServerErrorKey(c, response.MsgMemoryListFailed)
		return
	}
	response.Success(c, gin.H{"memories": mems, "total": total, "page": page, "page_size": pageSize})
}

// SearchMemories 搜索记忆
func (api *MemoryAPI) SearchMemories(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	var req searchMemoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}
	if req.Limit <= 0 {
		req.Limit = 20
	}
	if req.Limit > 50 {
		req.Limit = 50
	}

	mems, err := api.memSvc.SearchMemories(userID, &aiSvc.SearchMemoryRequest{
		Query:      req.Query,
		MemoryType: req.MemoryType,
		Limit:      req.Limit,
	})
	if err != nil {
		response.ServerErrorKey(c, response.MsgMemorySearchFailed)
		return
	}
	response.Success(c, gin.H{"memories": mems, "total": len(mems)})
}

// ListPendingMemories 列出待确认的记忆（P2-8 全量待确认区）。
// @Summary 列出待确认的记忆
// @Tags AI
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/memory/pending [get]
func (api *MemoryAPI) ListPendingMemories(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	mems, err := api.repo.FindPendingByUserID(userID, getPageSize(c, 50))
	if err != nil {
		response.ServerErrorKey(c, response.MsgMemoryListFailed)
		return
	}
	response.Success(c, gin.H{"memories": mems, "total": len(mems)})
}

// ConfirmMemory 确认一条待确认记忆，使其进入召回（P2-8）。
// @Summary 确认一条记忆（使其进入召回）
// @Tags AI
// @Produce json
// @Param id path int true "记忆ID"
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/ai/memory/{id}/confirm [post]
func (api *MemoryAPI) ConfirmMemory(c *gin.Context) {
	id := parseUintParam(c, "id")
	if id == 0 {
		response.BadRequest(c, "invalid id")
		return
	}
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	if err := api.repo.ConfirmMemory(id, userID); err != nil {
		response.ServerErrorKey(c, response.MsgMemoryUpdateFailed)
		return
	}
	response.Success(c, gin.H{"id": id, "status": "confirmed"})
}

// DeleteMemory 删除记忆
func (api *MemoryAPI) DeleteMemory(c *gin.Context) {
	id := parseUintParam(c, "id")
	if id == 0 {
		response.BadRequest(c, "invalid id")
		return
	}
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	if err := api.memSvc.DeleteMemory(userID, id); err != nil {
		response.ServerErrorKey(c, response.MsgMemoryDeleteFailed)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

// ===== 智能提取 API =====

// ExtractMemories 从对话中提取记忆（结合LLM）
func (api *MemoryAPI) ExtractMemories(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	var req extractMemoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "conversation_id and messages are required")
		return
	}
	req.UserID = userID

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	result, err := api.memSvc.ExtractMemories(c.Request.Context(), m, &aiSvc.ExtractMemoriesRequest{
		ConversationID: req.ConversationID,
		Messages:       req.Messages,
		UserID:         req.UserID,
	})
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"extracted": result.Extracted, "memories": result.Memories})
}

// RecallMemories 召回相关记忆（用于注入到prompt中）
func (api *MemoryAPI) RecallMemories(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	var req searchMemoryReq
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	mems, recallCtx, err := api.memSvc.RecallMemories(userID, &aiSvc.SearchMemoryRequest{
		Query:      req.Query,
		MemoryType: req.MemoryType,
		Limit:      req.Limit,
	})
	if err != nil {
		response.ServerErrorKey(c, response.MsgMemoryRecallFailed)
		return
	}
	response.Success(c, gin.H{
		"memories":     mems,
		"recall_count": len(mems),
		"context":      recallCtx,
	})
}

// GetMemoryStats 获取记忆统计
func (api *MemoryAPI) GetMemoryStats(c *gin.Context) {
	userID, ok := requireUserID(c)
	if !ok {
		return
	}
	stats, err := api.memSvc.GetMemoryStats(userID)
	if err != nil {
		response.ServerErrorKey(c, response.MsgMemoryStatsFailed)
		return
	}
	response.Success(c, gin.H{"stats": stats})
}

// ===== 分页辅助函数（仍在 API 层，因为依赖 gin.Context） =====

func parseUintParam(c *gin.Context, key string) uint {
	var val uint
	fmt.Sscanf(c.Param(key), "%d", &val)
	return val
}

func getPage(c *gin.Context) int {
	page := 1
	fmt.Sscanf(c.Query("page"), "%d", &page)
	if page < 1 {
		page = 1
	}
	return page
}

func getPageSize(c *gin.Context, defaultSize int) int {
	size := defaultSize
	fmt.Sscanf(c.Query("page_size"), "%d", &size)
	if size < 1 {
		size = defaultSize
	}
	if size > 100 {
		size = 100
	}
	return size
}
