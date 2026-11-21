package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"github.com/gin-gonic/gin"

	aimod "github.com/listen808/citebase/internal/ai"
	"github.com/listen808/citebase/internal/ai/mcp"
	"github.com/listen808/citebase/internal/ctxkeys"
	"github.com/listen808/citebase/internal/middleware"
	"github.com/listen808/citebase/internal/response"
)

// HandleMCPServer 对外提供 MCP 服务端（streamable HTTP，JSON-RPC 2.0）。
// 外部 Agent（Claude Code / Codex 等）可以像查询外部知识源一样查询本知识库。
// 走 AuthMiddleware：Agent 使用用户令牌，检索结果继承该用户的权限范围。
func (api *ChatAPI) HandleMCPServer(c *gin.Context) {
	m := aimod.GetModule()

	// 原始文档检索回调：把 rag.Retriever.RetrieveScoped（带 ctx 权限过滤）
	// 映射成 mcp 包自己的 Source 类型，避免 mcp ↔ rag 循环依赖。
	var docSearch mcp.DocSearchFn
	if r := m.Retriever(); r != nil {
		docSearch = func(ctx context.Context, question string, spaceID int, topK int) ([]mcp.Source, error) {
			sources, err := r.RetrieveScoped(ctx, question, spaceID, topK)
			if err != nil {
				return nil, err
			}
			out := make([]mcp.Source, 0, len(sources))
			for _, s := range sources {
				out = append(out, mcp.Source{
					DocumentName:  s.DocumentName,
					DocumentID:    s.DocumentID,
					Snippet:       s.Snippet,
					HeadingPath:   s.HeadingPath,
					CitationIndex: s.CitationIndex,
					Score:         s.Score,
				})
			}
			return out, nil
		}
	}

	srv := mcp.NewWikiServer(m.PrimaryProvider(), docSearch)

	body, err := io.ReadAll(io.LimitReader(c.Request.Body, 1<<20))
	if err != nil {
		response.BadRequestKey(c, response.MsgRequestBodyUnreadable)
		return
	}

	// 注入 user_id：原始文档检索（doc_search）依赖它做权限过滤，
	// 与 wiki 工具一致，确保外部 Agent 借检索不能越权读文档。
	ctx := c.Request.Context()
	userID, _ := middleware.GetUserID(c)
	if userID > 0 {
		ctx = ctxkeys.WithUserID(ctx, userID)
	}
	result, err := srv.Handle(ctx, body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"jsonrpc": "2.0",
			"id":      nil,
			"error":   gin.H{"code": -32603, "message": err.Error()},
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"jsonrpc": "2.0", "id": requestID(body), "result": result})
}

func requestID(raw []byte) interface{} {
	var probe struct {
		ID interface{} `json:"id"`
	}
	if err := json.Unmarshal(raw, &probe); err != nil {
		return nil
	}
	return probe.ID
}
