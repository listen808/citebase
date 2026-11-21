package social

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	socialmodel "github.com/listen808/citebase/internal/model/social"
	socialrepo "github.com/listen808/citebase/internal/repository/social"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/response"
	socialsvc "github.com/listen808/citebase/internal/service/social"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
)

type CommentAPI struct {
	commentService *socialsvc.CommentService
}

func NewCommentAPI() *CommentAPI {
	return &CommentAPI{
		commentService: socialsvc.NewCommentService(),
	}
}

func (api *CommentAPI) Create(c *gin.Context) {
	var req struct {
		DocumentID     int    `json:"document_id" binding:"required"`
		Content        string `json:"content" binding:"required"`
		SelectionText  string `json:"selection_text"`
		SelectionRange string `json:"selection_range"`
		ParentID       int    `json:"parent_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	currentUserID, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	comment, err := api.commentService.CreateComment(req.DocumentID, currentUserID, req.Content, req.SelectionText, req.SelectionRange, req.ParentID)
	if err != nil {
		if err == workspacerepo.ErrDocumentNotFound {
			response.NotFound(c, "document not found")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Created(c, comment)
}

func (api *CommentAPI) List(c *gin.Context) {
	documentIDStr := c.Param("document_id")
	documentID, err := strconv.Atoi(documentIDStr)
	if err != nil {
		response.BadRequest(c, "invalid document_id")
		return
	}

	uid, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	comments, err := api.commentService.GetDocumentComments(documentID, uid)
	if err != nil {
		if err == workspacesvc.ErrNoDocumentPermission {
			response.ForbiddenKey(c, response.MsgNoPermissionDocComments)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	if comments == nil {
		comments = make([]*socialmodel.Comment, 0)
	}

	response.Success(c, comments)
}

func (api *CommentAPI) Delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	currentUserID, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	if err := api.commentService.DeleteComment(id, currentUserID); err != nil {
		if err == socialrepo.ErrCommentNotFound {
			response.NotFound(c, "comment not found")
			return
		}
		if err == socialsvc.ErrCommentNotOwner {
			response.Forbidden(c, "not your comment")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, nil)
}

func (api *CommentAPI) Resolve(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	currentUserID, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	var req struct {
		Resolved bool `json:"resolved"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := api.commentService.ResolveComment(id, currentUserID, req.Resolved); err != nil {
		if err == socialrepo.ErrCommentNotFound {
			response.NotFound(c, "comment not found")
			return
		}
		if err == socialsvc.ErrCannotResolve {
			response.Forbidden(c, err.Error())
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, nil)
}
