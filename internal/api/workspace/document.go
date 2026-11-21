package workspace

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"
	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/ai"
	embedding "github.com/listen808/citebase/internal/ai/embedding"
	logx "github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/middleware"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	adminrepo "github.com/listen808/citebase/internal/repository/admin"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/response"
	adminsvc "github.com/listen808/citebase/internal/service/admin"
	searchsvc "github.com/listen808/citebase/internal/service/search"
	socialsvc "github.com/listen808/citebase/internal/service/social"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/safego"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"golang.org/x/crypto/bcrypt"
	"html/template"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type DocumentAPI struct {
	docService        *workspacesvc.DocumentService
	logService        *adminsvc.LogService
	spaceService      *workspacesvc.SpaceService
	followService     *socialsvc.FollowService
	attachmentService *workspacesvc.AttachmentService
	searchService     *searchsvc.SearchService
}

func NewDocumentAPI() *DocumentAPI {
	return &DocumentAPI{
		docService:        workspacesvc.NewDocumentService(),
		logService:        adminsvc.NewLogService(),
		spaceService:      workspacesvc.NewSpaceService(),
		followService:     socialsvc.NewFollowService(),
		attachmentService: workspacesvc.NewAttachmentService(),
		searchService:     searchsvc.NewSearchService(),
	}
}

// checkSpaceAccessForDoc 校验当前用户（可能未登录）对指定空间的访问权限
// 用于公开路由，防止未授权访问私有空间文档
func (api *DocumentAPI) checkSpaceAccessForDoc(c *gin.Context, spaceID int) error {
	userID, _ := middleware.GetUserID(c)
	ok, err := api.spaceService.CheckSpaceAccess(spaceID, userID)
	if err != nil {
		if errors.Is(err, workspacerepo.ErrSpaceNotFound) {
			return workspacerepo.ErrSpaceNotFound
		}
		return errors.New("no permission")
	}
	if !ok {
		return errors.New("no permission")
	}
	return nil
}

// CreateDocument 创建文档
// @Summary 创建文档
// @Tags 文档
// @Accept json
// @Produce json
// @Param request body object true "文档信息"
// @Success 201 {object} object
// @Router /api/v1/documents [post]
func (api *DocumentAPI) CreateDocument(c *gin.Context) {
	var req workspacemodel.CreateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	doc, err := api.docService.CreateDocument(uid, &req)
	if err != nil {
		if err == workspacesvc.ErrDocumentNameExists {
			response.Conflict(c, "document name already exists")
			return
		}
		if err == workspacesvc.ErrNoDocumentPermission {
			response.Forbidden(c, "no permission")
			return
		}
		if err == workspacesvc.ErrNotSpaceMember {
			response.Forbidden(c, "you are not a member of this space")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	// 记录文档创建日志（同步，确保记录成功）
	_ = api.logService.LogDocumentAction(doc.ID, doc.SpaceID, uid, adminmodel.DocActionCreate, "创建文档: "+doc.Name)

	response.Created(c, doc.ToResponse())
}

// GetDocumentByID 获取文档详情
// @Summary 获取文档详情
// @Tags 文档
// @Produce json
// @Param id path int true "文档 ID"
// @Success 200 {object} object
// @Router /api/v1/documents/:id [get]
func (api *DocumentAPI) GetDocumentByID(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	docResp, err := api.docService.GetDocumentByID(id)
	if err != nil {
		if err == workspacesvc.ErrDocumentNotFound {
			response.NotFound(c, "document not found")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	// #7 草稿可见性：草稿仅作者和超管可见
	if docResp.Status == workspacemodel.DocumentStatusDraft {
		uid, _ := middleware.GetUserID(c)
		if uid != docResp.CreateUserID && !middleware.IsSuperAdmin(c) {
			response.NotFound(c, "document not found")
			return
		}
	}

	// 公开路由需校验空间访问权限
	if err := api.checkSpaceAccessForDoc(c, docResp.SpaceID); err != nil {
		if errors.Is(err, workspacerepo.ErrSpaceNotFound) {
			response.NotFound(c, "space not found")
			return
		}
		response.Forbidden(c, "no permission to access this document")
		return
	}

	// 异步记录访问日志（P1-4 访问统计）
	uid, _ := middleware.GetUserID(c)
	api.docService.RecordView(id, docResp.SpaceID, uid, c.ClientIP(), c.Request.UserAgent())

	api.autoFollowOnView(c, docResp.ID, docResp.SpaceID)

	response.Success(c, docResp)
}

// autoFollowOnView 系统设置「自动关注文档」（basic 组）：
// 开关开启且（auto_follow_doc 范围未配置 = 全部空间，或文档空间 ID 在逗号分隔范围内）时，
// 登录用户打开文档详情/完整信息即自动建立关注（幂等）。任何失败只记日志，不影响查看。
func (api *DocumentAPI) autoFollowOnView(c *gin.Context, docID, spaceID int) {
	uid, err := middleware.GetUserID(c)
	if err != nil || uid <= 0 {
		return
	}
	cfgRepo := adminrepo.NewConfigRepository()
	open, err := cfgRepo.GetValueByKey("auto_follow_doc_open")
	if err != nil || open != "1" {
		return
	}
	scope, _ := cfgRepo.GetValueByKey("auto_follow_doc")
	if strings.TrimSpace(scope) != "" {
		allowed := false
		for _, idStr := range strings.Split(scope, ",") {
			if id, err := strconv.Atoi(strings.TrimSpace(idStr)); err == nil && id == spaceID {
				allowed = true
				break
			}
		}
		if !allowed {
			return
		}
	}
	if err := api.followService.FollowDocument(uid, docID); err != nil {
		logx.Warnf("[auto-follow] user=%d doc=%d: %v", uid, docID, err)
	}
}

// GetDocumentFull 一次性返回文档元数据+内容+路径，减少 3 次 HTTP 往返为 1 次
// @Summary 获取文档完整信息（合并接口）
// @Tags 文档
// @Param id path int true "文档 ID"
// @Success 200 {object} object
// @Router /api/v1/documents/{id}/full [get]
func (api *DocumentAPI) GetDocumentFull(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	full, err := api.docService.GetDocumentFull(id)
	if err != nil {
		if err == workspacesvc.ErrDocumentNotFound {
			response.NotFound(c, "document not found")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	// #7 草稿可见性
	if full.Document.Status == workspacemodel.DocumentStatusDraft {
		uid, _ := middleware.GetUserID(c)
		if uid != full.Document.CreateUserID && !middleware.IsSuperAdmin(c) {
			response.NotFound(c, "document not found")
			return
		}
	}

	if err := api.checkSpaceAccessForDoc(c, full.Document.SpaceID); err != nil {
		if errors.Is(err, workspacerepo.ErrSpaceNotFound) {
			response.NotFound(c, "space not found")
			return
		}
		response.Forbidden(c, "no permission to access this document")
		return
	}

	uid, _ := middleware.GetUserID(c)
	api.docService.RecordView(id, full.Document.SpaceID, uid, c.ClientIP(), c.Request.UserAgent())

	api.autoFollowOnView(c, full.Document.ID, full.Document.SpaceID)

	response.Success(c, full)
}

// #7 草稿/发布分离

// PublishDocument 发布文档
// @Summary 发布文档
// @Tags 文档
// @Param id path int true "文档 ID"
// @Produce json
// @Success 200
// @Router /api/v1/documents/{id}/publish [post]
func (api *DocumentAPI) PublishDocument(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}
	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}
	if err := api.docService.PublishDocument(id, uid); err != nil {
		if err == workspacesvc.ErrDocumentNotFound {
			response.NotFound(c, "document not found")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}
	response.SuccessWithMsg(c, nil, "文档已发布")
}

// UnpublishDocument 转为草稿
// @Summary 转为草稿
// @Tags 文档
// @Param id path int true "文档 ID"
// @Produce json
// @Success 200
// @Router /api/v1/documents/{id}/unpublish [post]
func (api *DocumentAPI) UnpublishDocument(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}
	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}
	if err := api.docService.UnpublishDocument(id, uid); err != nil {
		if err == workspacesvc.ErrDocumentNotFound {
			response.NotFound(c, "document not found")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}
	response.SuccessWithMsg(c, nil, "文档已转为草稿")
}

// GetDocumentContent 获取文档内容
// @Summary 获取文档内容
// @Tags 文档
// @Produce json
// @Param id path int true "文档 ID"
// @Success 200 {object} object
// @Router /api/v1/documents/:id/content [get]
func (api *DocumentAPI) GetDocumentContent(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	// 一次查询拿到 raw doc 用于权限检查 + 读取内容（避免 GetDocumentByID + GetDocumentContent 重复查询）
	doc, err := api.docService.GetDocumentRawByID(id)
	if err != nil {
		response.NotFound(c, "document not found")
		return
	}
	// #7 草稿可见性：草稿仅作者和超管可见
	if doc.Status == workspacemodel.DocumentStatusDraft {
		uid, _ := middleware.GetUserID(c)
		if uid != doc.CreateUserID && !middleware.IsSuperAdmin(c) {
			response.NotFound(c, "document not found")
			return
		}
	}
	if err := api.checkSpaceAccessForDoc(c, doc.SpaceID); err != nil {
		if errors.Is(err, workspacerepo.ErrSpaceNotFound) {
			response.NotFound(c, "space not found")
			return
		}
		response.Forbidden(c, "no permission to access this document")
		return
	}

	content, err := api.docService.GetDocumentContentByDoc(doc)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"document_id": id,
		"content":     content,
	})
}

// GetDocumentTree 获取空间文档树
// @Summary 获取空间文档树
// @Tags 文档
// @Produce json
// @Param space_id query int true "空间 ID"
// @Success 200 {array} object
// @Router /api/v1/documents/tree [get]
func (api *DocumentAPI) GetDocumentTree(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	if spaceID == 0 {
		response.BadRequest(c, "space_id is required")
		return
	}

	if err := api.checkSpaceAccessForDoc(c, spaceID); err != nil {
		if errors.Is(err, workspacerepo.ErrSpaceNotFound) {
			response.NotFound(c, "space not found")
			return
		}
		response.Forbidden(c, "no permission to access this space")
		return
	}

	tree, err := api.docService.GetDocumentTree(spaceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, tree)
}

// GetDocumentChildren 获取文档子节点（懒加载）
func (api *DocumentAPI) GetDocumentChildren(c *gin.Context) {
	parentID, _ := strconv.Atoi(c.Param("id"))
	if parentID == 0 {
		response.BadRequest(c, "parent_id is required")
		return
	}

	// 通过父文档获取 spaceID 并校验权限
	parentDoc, err := api.docService.GetDocumentByID(parentID)
	if err != nil {
		response.NotFound(c, "document not found")
		return
	}
	if err := api.checkSpaceAccessForDoc(c, parentDoc.SpaceID); err != nil {
		if errors.Is(err, workspacerepo.ErrSpaceNotFound) {
			response.NotFound(c, "space not found")
			return
		}
		response.Forbidden(c, "no permission to access this document")
		return
	}

	children, err := api.docService.GetDocumentChildren(parentID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, children)
}

// GetRootDocuments 获取空间根文档（懒加载首层）
func (api *DocumentAPI) GetRootDocuments(c *gin.Context) {
	spaceID, _ := strconv.Atoi(c.Query("space_id"))
	if spaceID == 0 {
		response.BadRequest(c, "space_id is required")
		return
	}

	if err := api.checkSpaceAccessForDoc(c, spaceID); err != nil {
		if errors.Is(err, workspacerepo.ErrSpaceNotFound) {
			response.NotFound(c, "space not found")
			return
		}
		response.Forbidden(c, "no permission to access this space")
		return
	}

	roots, err := api.docService.GetRootDocuments(spaceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, roots)
}

// UpdateDocument 更新文档
// @Summary 更新文档
// @Tags 文档
// @Accept json
// @Produce json
// @Param id path int true "文档 ID"
// @Param request body object true "文档信息"
// @Success 200 {object} object
// @Router /api/v1/documents/:id [put]
func (api *DocumentAPI) UpdateDocument(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	var req workspacemodel.UpdateDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	doc, err := api.docService.UpdateDocument(id, uid, &req)
	if err != nil {
		if err == workspacesvc.ErrDocumentNameExists {
			response.Conflict(c, err.Error())
			return
		}
		if err == workspacesvc.ErrDocumentConflict {
			response.ConflictKey(c, response.MsgDocModifiedByOther)
			return
		}
		if err == workspacesvc.ErrNoDocumentPermission {
			response.Forbidden(c, err.Error())
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	changeLog := req.ChangeLog
	if changeLog == "" {
		changeLog = "更新文档内容"
	}
	_ = api.logService.LogDocumentAction(id, doc.SpaceID, uid, adminmodel.DocActionModify, changeLog)

	// P2-5：已移除“编辑文档后自动关注”副作用，避免 UpdateDocument 跨模块副作用。
	// 前端如需同时关注文档，请单独调用 POST /api/v1/follow/document。

	response.Success(c, doc.ToResponse())
}

// DeleteDocument 删除文档
// @Summary 删除文档
// @Tags 文档
// @Produce json
// @Param id path int true "文档 ID"
// @Success 200 {message} string
// @Router /api/v1/documents/:id [delete]
func (api *DocumentAPI) DeleteDocument(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	// 在删除前获取文档信息，以便记录日志
	doc, _ := api.docService.GetDocumentByID(id)
	spaceID := 0
	if doc != nil {
		spaceID = doc.SpaceID
	}

	if err := api.docService.DeleteDocument(id, uid); err != nil {
		if err == workspacesvc.ErrNoDocumentPermission {
			response.Forbidden(c, err.Error())
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	// 记录删除日志（同步）
	_ = api.logService.LogDocumentAction(id, spaceID, uid, adminmodel.DocActionDelete, "删除文档")

	response.SuccessWithMsg(c, nil, "document moved to trash")
}

// RestoreDocument 从回收站恢复文档
// @Summary 从回收站恢复文档
// @Tags 文档
// @Produce json
// @Param id path int true "文档 ID"
// @Success 200 {message} string
// @Router /api/v1/documents/:id/restore [post]
func (api *DocumentAPI) RestoreDocument(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	trashItem, _ := api.docService.GetTrashByDocumentID(id)
	spaceID := 0
	if trashItem != nil {
		spaceID = trashItem.SpaceID
	}

	if err := api.docService.RestoreDocument(id, uid); err != nil {
		if err == workspacesvc.ErrNoDocumentPermission {
			response.Forbidden(c, err.Error())
			return
		}
		if err == workspacesvc.ErrDocumentNotFound {
			response.NotFound(c, "document not found in trash")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	_ = api.logService.LogDocumentAction(id, spaceID, uid, adminmodel.DocActionRestore, "从回收站恢复文档")
	response.SuccessWithMsg(c, nil, "document restored successfully")
}

// PermanentDeleteDocument 彻底删除文档
// @Summary 彻底删除文档
// @Tags 文档
// @Produce json
// @Param id path int true "文档 ID"
// @Success 200 {message} string
// @Router /api/v1/documents/:id/permanent [delete]
func (api *DocumentAPI) PermanentDeleteDocument(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	trashItem, _ := api.docService.GetTrashByDocumentID(id)
	spaceID := 0
	if trashItem != nil {
		spaceID = trashItem.SpaceID
	}

	if err := api.docService.PermanentDeleteDocument(id, uid); err != nil {
		if err == workspacesvc.ErrNoDocumentPermission {
			response.Forbidden(c, err.Error())
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	_ = api.logService.LogDocumentAction(id, spaceID, uid, adminmodel.DocActionDelete, "彻底删除文档")
	response.SuccessWithMsg(c, nil, "document permanently deleted")
}

// GetSpaceTrash 获取空间回收站列表
// @Summary 获取空间回收站列表
// @Tags 文档
// @Produce json
// @Param id path int true "空间 ID"
// @Success 200 {array} object
// @Router /api/v1/spaces/:id/trash [get]
func (api *DocumentAPI) GetSpaceTrash(c *gin.Context) {
	idStr := c.Param("id")
	spaceID, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid space id")
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	items, err := api.docService.GetSpaceTrash(spaceID, uid)
	if err != nil {
		if err == workspacesvc.ErrNoDocumentPermission {
			response.Forbidden(c, err.Error())
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, items)
}

// ImportDocuments 批量导入文档（支持 Markdown / Confluence HTML / Notion CSV）
// @Summary 批量导入文档
// @Tags 文档
// @Produce json
// @Param id path int true "空间 ID"
// @Param body body object true "导入内容"
// @Success 200 {object} object
// @Router /api/v1/spaces/:id/import [post]
func (api *DocumentAPI) ImportDocuments(c *gin.Context) {
	idStr := c.Param("id")
	spaceID, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid space id")
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req workspacemodel.ImportDocumentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// 格式转换：Confluence HTML / Notion CSV → Markdown
	for i := range req.Items {
		switch req.Items[i].Format {
		case "confluence":
			md, err := workspacesvc.ConvertConfluenceHTML(req.Items[i].Content)
			if err != nil {
				response.BadRequestKeyDetail(c, response.MsgConfluenceConvertFailed, err.Error())
				return
			}
			req.Items[i].Content = md
		case "notion":
			md, err := workspacesvc.ConvertNotionCSV(req.Items[i].Content)
			if err != nil {
				response.BadRequestKeyDetail(c, response.MsgNotionConvertFailed, err.Error())
				return
			}
			req.Items[i].Content = md
		}
	}

	result, err := api.docService.ImportDocuments(spaceID, uid, req.Items)
	if err != nil {
		if err == workspacesvc.ErrNoDocumentPermission {
			response.Forbidden(c, err.Error())
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, result)
}

// MoveDocument 移动文档
// @Summary 移动文档
// @Tags 文档
// @Accept json
// @Produce json
// @Param id path int true "文档 ID"
// @Param request body object true "移动目标"
// @Success 200 {message} string
// @Router /api/v1/documents/:id/move [put]
func (api *DocumentAPI) MoveDocument(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	var req workspacemodel.MoveDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	if err := api.docService.MoveDocument(id, uid, &req); err != nil {
		if err == workspacesvc.ErrInvalidParent {
			response.BadRequest(c, err.Error())
			return
		}
		if err == workspacesvc.ErrNoDocumentPermission {
			response.Forbidden(c, err.Error())
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	// 记录文档移动日志（使用正确的 space_id）
	movedDoc, _ := api.docService.GetDocumentByID(id)
	moveSpaceID := 0
	if movedDoc != nil {
		moveSpaceID = movedDoc.SpaceID
	}
	safego.Go("LogDocumentAction", func() {
		_ = api.logService.LogDocumentAction(id, moveSpaceID, uid, adminmodel.DocActionModify, "移动文档")
	})

	response.SuccessWithMsg(c, nil, "document moved successfully")
}

// CopyDocument 复制文档
// @Summary 复制文档
// @Tags 文档
// @Accept json
// @Produce json
// @Param id path int true "文档 ID"
// @Param request body object true "复制目标"
// @Success 200 {message} string
// @Router /api/v1/documents/:id/copy [post]
func (api *DocumentAPI) CopyDocument(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	var req workspacemodel.CopyDocumentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	if err := api.docService.CopyDocument(id, uid, &req); err != nil {
		if err == workspacesvc.ErrInvalidParent {
			response.BadRequest(c, err.Error())
			return
		}
		if err == workspacesvc.ErrNoDocumentPermission {
			response.Forbidden(c, err.Error())
			return
		}
		if err == workspacesvc.ErrDocumentNameExists {
			response.Conflict(c, "document name already exists")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	// 记录文档复制日志（使用正确的 space_id）
	copiedDoc, _ := api.docService.GetDocumentByID(id)
	copySpaceID := 0
	if copiedDoc != nil {
		copySpaceID = copiedDoc.SpaceID
	}
	safego.Go("LogDocumentAction", func() {
		_ = api.logService.LogDocumentAction(id, copySpaceID, uid, adminmodel.DocActionModify, "复制文档")
	})

	response.SuccessWithMsg(c, nil, "document copied successfully")
}

// SortDocuments 批量排序文档（拖拽后保存顺序）
// @Summary 批量排序文档
// @Tags 文档
// @Accept json
// @Produce json
// @Param request body object true "排序请求"
// @Success 200
// @Router /api/v1/documents/sort [put]
func (api *DocumentAPI) SortDocuments(c *gin.Context) {
	var req workspacemodel.SortDocumentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	if err := api.docService.SortDocuments(uid, &req); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	// 记录文档排序日志（记录第一个文档的移动，使用正确的 space_id）
	if len(req.Orders) > 0 {
		sortDoc, _ := api.docService.GetDocumentByID(req.Orders[0].ID)
		sortSpaceID := 0
		if sortDoc != nil {
			sortSpaceID = sortDoc.SpaceID
		}
		safego.Go("LogDocumentAction", func() {
			_ = api.logService.LogDocumentAction(req.Orders[0].ID, sortSpaceID, uid, adminmodel.DocActionModify, "批量排序文档")
		})
	}

	response.SuccessWithMsg(c, nil, "排序已保存")
}

// GetDocumentVersions 获取文档版本列表
// @Summary 获取文档版本
// @Tags 文档
// @Produce json
// @Param id path int true "文档 ID"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(10)
// @Success 200 {object} map[string]interface{}
// @Router /api/v1/documents/:id/versions [get]
func (api *DocumentAPI) GetDocumentVersions(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	page, pageSize, _ := utils.ParsePagination(c, 10)
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	// 版本读路径必须按空间可见性校验：只校验登录会让任何人用别人的 id 读出私有正文
	uid, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	versions, total, err := api.docService.GetDocumentVersionsPaginatedForUser(id, page, pageSize, uid)
	if err != nil {
		if err == workspacesvc.ErrNoDocumentPermission {
			response.ForbiddenKey(c, response.MsgNoPermissionVersionHistory)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"data":      versions,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// GetVersionContent 获取指定版本内容
// @Summary 获取版本内容
// @Tags 文档
// @Produce json
// @Param id path int true "文档 ID"
// @Param version query int true "版本号"
// @Success 200 {object} object
// @Router /api/v1/documents/:id/versions/:version [get]
func (api *DocumentAPI) GetVersionContent(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	versionStr := c.Param("version")
	version, err := strconv.Atoi(versionStr)
	if err != nil {
		response.BadRequest(c, "invalid version")
		return
	}

	uid, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	content, err := api.docService.GetVersionContentForUser(id, version, uid)
	if err != nil {
		if err == workspacesvc.ErrNoDocumentPermission {
			response.ForbiddenKey(c, response.MsgNoPermissionHistoricalDoc)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"document_id": id,
		"version":     version,
		"content":     content,
	})
}

// DiffVersions 对比两个版本的差异
// @Summary 版本差异对比
// @Tags 文档
// @Produce json
// @Param id path int true "文档 ID"
// @Param from query int true "起始版本号（0=当前内容）"
// @Param to query int true "目标版本号（0=当前内容）"
// @Success 200 {object} object
// @Router /api/v1/documents/:id/diff [get]
func (api *DocumentAPI) DiffVersions(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	fromVersion, err := strconv.Atoi(c.DefaultQuery("from", "0"))
	if err != nil {
		response.BadRequest(c, "invalid from version")
		return
	}
	toVersion, err := strconv.Atoi(c.DefaultQuery("to", "0"))
	if err != nil {
		response.BadRequest(c, "invalid to version")
		return
	}

	uid, ok := middleware.RequireUser(c)
	if !ok {
		return
	}

	diff, err := api.docService.DiffVersionsForUser(id, fromVersion, toVersion, uid)
	if err != nil {
		if err == workspacesvc.ErrNoDocumentPermission {
			response.ForbiddenKey(c, response.MsgNoPermissionCompareVersions)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, diff)
}

// GetDocumentStats 获取文档访问统计
// @Summary 文档访问统计
// @Tags 文档
// @Produce json
// @Param id path int true "文档 ID"
// @Success 200 {object} object
// @Router /api/v1/documents/:id/stats [get]
func (api *DocumentAPI) GetDocumentStats(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	// 权限校验
	docResp, err := api.docService.GetDocumentByID(id)
	if err != nil {
		if err == workspacesvc.ErrDocumentNotFound {
			response.NotFound(c, "document not found")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}
	if err := api.checkSpaceAccessForDoc(c, docResp.SpaceID); err != nil {
		response.Forbidden(c, "no permission")
		return
	}

	stats, err := api.docService.GetViewStats(id)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, stats)
}

// RollbackToVersion 回滚到指定版本
// @Summary 回滚文档版本
// @Tags 文档
// @Accept json
// @Produce json
// @Param id path int true "文档 ID"
// @Param request body object true "{version: int}"
// @Success 200 {message} string
// @Router /api/v1/documents/:id/rollback [post]
func (api *DocumentAPI) RollbackToVersion(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	var req struct {
		Version int `json:"version" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.Unauthorized(c, "unauthorized")
		return
	}

	if err := api.docService.RollbackToVersion(id, req.Version, uid); err != nil {
		if err == workspacesvc.ErrNoDocumentPermission {
			response.Forbidden(c, err.Error())
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	// 获取文档信息以记录space_id
	doc, _ := api.docService.GetDocumentByID(id)
	spaceID := 0
	if doc != nil {
		spaceID = doc.SpaceID
	}
	go func() {
		_ = api.logService.LogDocumentAction(id, spaceID, uid, adminmodel.DocActionModify, "回滚到版本: "+strconv.Itoa(req.Version))
	}()

	response.SuccessWithMsg(c, nil, "document rolled back successfully")
}

// SearchDocuments 已于 P2-1 移除：
// 原 DB LIKE 搜索与 /search/fulltext (Bleve 全文搜索) 重复，且能力弱。
// 前端统一走 fulltextSearch(type: 'title')，本 handler/Service/Repo 方法同时删除。

// GetDocumentPath 获取文档路径（面包屑）
// @Summary 获取文档路径
// @Tags 文档
// @Produce json
// @Param id path int true "文档 ID"
// @Success 200 {array} object
// @Router /api/v1/documents/:id/path [get]
func (api *DocumentAPI) GetDocumentPath(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	path, err := api.docService.GetDocumentPathNames(id)
	if err != nil {
		// 取不存在的文档不是服务端错误。仓储层已经有 ErrDocumentNotFound 这个哨兵，
		// 而这里原先一律 ServerErrorSafe —— 前端因此收到 500，把"这篇文档没了"报成
		// "服务暂时不可用"（运行时巡检实测：GET /documents/356/path → 500，同 id 的
		// /356、/356/content、/356/full 都是正常的 404）。同文件的 ErrSpaceNotFound 就是
		// 这么分的，照它。
		if errors.Is(err, workspacerepo.ErrDocumentNotFound) {
			response.NotFound(c, "document not found")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, path)
}

// ShareDocument 分享文档（公开，无需登录）
// @Summary 分享文档
// @Tags 文档
// @Produce json
// @Param id path int true "文档 ID"
// @Success 200 {object} object
// @Router /api/v1/documents/{id}/share [get]
func (api *DocumentAPI) ShareDocument(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	// 获取文档基本信息
	doc, err := api.docService.GetDocumentByID(id)
	if err != nil {
		response.NotFound(c, "document not found")
		return
	}
	if doc.IsDelete == 1 {
		response.NotFound(c, "document not found")
		return
	}
	// 草稿不可被公开分享（2026-10-26 审计发现）：本端点是匿名可达的，此前只查 IsDelete
	// 和空间分享开关 —— 空间开了分享后，未发布草稿的全文任何拿到链接的人都能读。
	// 普通查看端点有草稿可见性检查（见本文档 GetDocumentDetail 的可见性分支），分享端点必须同样收紧；
	// 返回与「不存在」一致，避免用响应差异探测草稿是否存在。作者预览请先发布。
	if doc.PublishedAt == 0 {
		response.NotFound(c, "document not found")
		return
	}

	// 检查空间是否允许分享
	// 注意：本端点是「分享链接的匿名消费端」，可见性由 space.IsShare + 分享密码控制，
	// 不能要求登录，否则公开分享链接会失效。
	space, err := api.spaceService.GetSpaceByID(doc.SpaceID)
	if err != nil || space == nil {
		response.NotFound(c, "space not found")
		return
	}
	if space.IsShare != 1 {
		response.ForbiddenKey(c, response.MsgSpaceDisallowsShare)
		return
	}

	// 如果空间设置了分享密码，需要验证
	spaceModel, _ := workspacerepo.NewSpaceRepository().FindByID(doc.SpaceID)
	sharePwd := ""
	if spaceModel != nil {
		sharePwd = spaceModel.SharePassword
	}
	hasPassword := sharePwd != ""

	if hasPassword {
		password := c.Query("password")
		if password == "" {
			// 只说"这道门需要密码"，不说门后面是什么。
			// 原先这一支把 name / space_name / creator_name / breadcrumb 一起返回了，
			// 于是分享密码只保护正文，标题与它所在的整条路径（空间名 + 各级父文档名）
			// 对任何拿到链接的人可见 —— 而链接本身就是被到处转发的那个东西。
			// 前端判定用的是 has_password && !content，去掉这些字段不影响密码流程。
			response.Success(c, gin.H{
				"has_password": true,
			})
			return
		}
		if err := bcrypt.CompareHashAndPassword([]byte(sharePwd), []byte(password)); err != nil {
			response.ForbiddenKey(c, response.MsgWrongSharePassword)
			return
		}
	}

	// 获取文档内容
	content, err := api.docService.GetDocumentContent(id)
	if err != nil {
		response.NotFound(c, "document content not found")
		return
	}

	// 获取面包屑路径
	breadcrumb, _ := api.docService.GetDocumentPathNames(id)

	response.Success(c, gin.H{
		"id":           doc.ID,
		"name":         doc.Name,
		"type":         doc.Type,
		"content":      content,
		"space_name":   space.Name,
		"space_id":     space.ID,
		"creator_name": doc.CreatorName,
		"creator_id":   doc.CreatorID,
		"editor_name":  doc.EditorName,
		"editor_id":    doc.EditorID,
		"create_time":  doc.CreateTime,
		"update_time":  doc.UpdateTime,
		"breadcrumb":   breadcrumb,
		"has_password": hasPassword,
	})
}

// ExportDocument 导出文档为 zip 打包
func (api *DocumentAPI) ExportDocument(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	doc, err := api.docService.GetDocumentByID(id)
	if err != nil || doc == nil || doc.IsDelete == 1 {
		response.NotFound(c, "document not found")
		return
	}

	// 导出/分享前必须确认调用者对该文档所在空间可见：只校验登录等于任何人都能取走他人私有正文
	actorUID, actorOK := middleware.RequireUser(c)
	if !actorOK {
		return
	}
	if err := api.docService.EnsureDocReadable(id, actorUID); err != nil {
		if err == workspacesvc.ErrNoDocumentPermission {
			response.ForbiddenKey(c, response.MsgNoPermissionExportDoc)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	space, err := api.spaceService.GetSpaceByID(doc.SpaceID)
	if err != nil || space == nil {
		response.Forbidden(c, "this document cannot be exported")
		return
	}
	if space.IsExport == 0 {
		response.Forbidden(c, "space export is disabled")
		return
	}

	docContent, err := api.docService.GetDocumentContent(id)
	if err != nil {
		response.ServerError(c, "failed to read document content")
		return
	}

	attachments, _ := api.attachmentService.GetRawAttachmentsByDocumentID(id)
	if attachments == nil {
		attachments = []*workspacemodel.Attachment{}
	}

	safeName := doc.Name
	for _, ch := range []string{"/", "\\", ":", "*", "?", "<", ">", "|"} {
		safeName = strings.ReplaceAll(safeName, ch, "_")
	}
	safeName = strings.ReplaceAll(safeName, "\"", "_")
	zipName := safeName + ".zip"

	tmpZip := filepath.Join(os.TempDir(), fmt.Sprintf("citebase_export_%d_%s.zip", id, safeName))
	f, err := os.Create(tmpZip)
	if err != nil {
		response.ServerError(c, "failed to create zip file")
		return
	}
	defer f.Close()
	defer os.Remove(tmpZip)

	zw := zip.NewWriter(f)
	defer zw.Close()

	if w, err := zw.Create("README.md"); err == nil {
		_, _ = w.Write([]byte("# " + doc.Name + "\n\n"))
		_, _ = w.Write([]byte(docContent))
	}

	usedAttNames := make(map[string]bool, len(attachments))
	for _, att := range attachments {
		if att.Path == "" {
			continue
		}
		attName := att.Name
		if attName == "" {
			attName = filepath.Base(att.Path)
		}
		entry := filepath.Join("attachments", attName)
		if usedAttNames[entry] {
			entry = filepath.Join("attachments", fmt.Sprintf("%d_%s", att.ID, attName))
		}
		usedAttNames[entry] = true
		w, err := zw.Create(entry)
		if err != nil {
			continue
		}
		// P3：db 模式 kind=1 流式读块表（内存峰值一个块），file 模式与未迁移记录读磁盘
		if err := api.attachmentService.CopyAttachmentData(att, w); err != nil {
			continue
		}
	}

	zw.Close()
	f.Close()

	c.Header("Content-Type", "application/zip")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(zipName)))
	c.Header("Cache-Control", "no-store")
	c.File(tmpZip)
}

// ExportDocumentHTML 导出文档为 HTML 页面
func (api *DocumentAPI) ExportDocumentHTML(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	doc, err := api.docService.GetDocumentByID(id)
	if err != nil || doc == nil || doc.IsDelete == 1 {
		response.NotFound(c, "document not found")
		return
	}

	// 导出/分享前必须确认调用者对该文档所在空间可见：只校验登录等于任何人都能取走他人私有正文
	actorUID, actorOK := middleware.RequireUser(c)
	if !actorOK {
		return
	}
	if err := api.docService.EnsureDocReadable(id, actorUID); err != nil {
		if err == workspacesvc.ErrNoDocumentPermission {
			response.ForbiddenKey(c, response.MsgNoPermissionExportDoc)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	space, err := api.spaceService.GetSpaceByID(doc.SpaceID)
	if err != nil || space == nil {
		response.Forbidden(c, "this document cannot be exported")
		return
	}
	if space.IsExport == 0 {
		response.Forbidden(c, "space export is disabled")
		return
	}

	docContent, err := api.docService.GetDocumentContent(id)
	if err != nil {
		response.ServerError(c, "failed to read document content")
		return
	}

	// Markdown -> HTML body
	var buf bytes.Buffer
	md := goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough))
	if err := md.Convert([]byte(docContent), &buf); err != nil {
		response.ServerError(c, "failed to render markdown")
		return
	}
	// 将图片相对路径替换为 base64 data URI
	renderedHTML := api.embedImagesInHTML(buf.String())
	bodyHTML := template.HTML(renderedHTML) //nolint:gosec // rendered markdown intended here

	safeName := template.HTMLEscapeString(doc.Name)

	// Build complete HTML page
	htmlPage := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + safeName + `</title>
<style>
  *, *::before, *::after { box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif;
    max-width: 800px;
    margin: 0 auto;
    padding: 40px 24px 80px;
    color: #1d1d1f;
    line-height: 1.7;
    background: #fff;
  }
  h1 { font-size: 2em; font-weight: 700; margin: 0 0 8px; letter-spacing: -0.02em; }
  h2 { font-size: 1.5em; font-weight: 600; margin-top: 2em; border-bottom: 1px solid #e5e5e7; padding-bottom: 0.3em; }
  h3 { font-size: 1.2em; font-weight: 600; margin-top: 1.5em; }
  p { margin: 0.8em 0; }
  code {
    background: #f5f5f7;
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 0.9em;
    font-family: "SF Mono", Menlo, Consolas, monospace;
  }
  pre {
    background: #f5f5f7;
    padding: 16px;
    border-radius: 8px;
    overflow-x: auto;
    line-height: 1.5;
  }
  pre code { background: none; padding: 0; }
  blockquote {
    border-left: 4px solid #e5e5e7;
    margin: 1em 0;
    padding: 0.5em 1em;
    color: #6e6e73;
  }
  table { border-collapse: collapse; width: 100%; margin: 1em 0; }
  th, td { border: 1px solid #e5e5e7; padding: 8px 12px; text-align: left; }
  th { background: #f5f5f7; font-weight: 600; }
  img { max-width: 100%; height: auto; border-radius: 8px; }
  a { color: #0066cc; text-decoration: none; }
  a:hover { text-decoration: underline; }
  hr { border: none; border-top: 1px solid #e5e5e7; margin: 2em 0; }
  ul, ol { padding-left: 1.5em; }
  li { margin: 0.3em 0; }
  .meta { color: #6e6e73; font-size: 0.85em; margin-bottom: 2em; }
  @media (prefers-color-scheme: dark) {
    body { background: #1d1d1f; color: #f5f5f7; }
    h2 { border-color: #38383a; }
    code { background: #2c2c2e; }
    pre { background: #2c2c2e; }
    blockquote { border-color: #38383a; color: #98989d; }
    th, td { border-color: #38383a; }
    th { background: #2c2c2e; }
    a { color: #2997ff; }
    .meta { color: #98989d; }
  }
</style>
</head>
<body>
<h1>` + safeName + `</h1>
<div class="meta">` + template.HTMLEscapeString(space.Name) + `</div>
<div class="content">` + string(bodyHTML) + `</div>
</body>
</html>`

	safeFileName := doc.Name
	for _, ch := range []string{"/", "\\", ":", "*", "?", "<", ">", "|", "\""} {
		safeFileName = strings.ReplaceAll(safeFileName, ch, "_")
	}

	c.Header("Content-Type", "text/html; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(safeFileName+".html")))
	c.Header("Cache-Control", "no-store")
	c.String(200, htmlPage)
}

// ExportDocumentWord 导出文档为 Word (.docx) 格式
// 纯标准库手写最小化 OOXML，支持图片嵌入
func (api *DocumentAPI) ExportDocumentWord(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	doc, err := api.docService.GetDocumentByID(id)
	if err != nil || doc == nil || doc.IsDelete == 1 {
		response.NotFound(c, "document not found")
		return
	}

	// 导出/分享前必须确认调用者对该文档所在空间可见：只校验登录等于任何人都能取走他人私有正文
	actorUID, actorOK := middleware.RequireUser(c)
	if !actorOK {
		return
	}
	if err := api.docService.EnsureDocReadable(id, actorUID); err != nil {
		if err == workspacesvc.ErrNoDocumentPermission {
			response.ForbiddenKey(c, response.MsgNoPermissionExportDoc)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	space, err := api.spaceService.GetSpaceByID(doc.SpaceID)
	if err != nil || space == nil {
		response.Forbidden(c, "this document cannot be exported")
		return
	}
	if space.IsExport == 0 {
		response.Forbidden(c, "space export is disabled")
		return
	}

	docContent, err := api.docService.GetDocumentContent(id)
	if err != nil {
		response.ServerError(c, "failed to read document content")
		return
	}

	docx, err := buildDocxFromMarkdown(doc.Name, docContent, api.loadImageForEmbed)
	if err != nil {
		logx.Errorf("ExportDocumentWord build failed: %v", err)
		response.ServerError(c, "failed to build docx")
		return
	}

	safeName := doc.Name
	for _, ch := range []string{"/", "\\", ":", "*", "?", "<", ">", "|", "\""} {
		safeName = strings.ReplaceAll(safeName, ch, "_")
	}

	c.Header("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(safeName+".docx")))
	c.Header("Cache-Control", "no-store")
	c.Data(200, "application/vnd.openxmlformats-officedocument.wordprocessingml.document", docx)
}

// mdImgRegex 匹配 markdown 图片语法 ![alt](url)
var mdImgRegex = regexp.MustCompile(`^!\[([^\]]*)\]\(([^)]+)\)\s*$`)

// EMU 单位换算：1 inch = 914400 EMU；96 DPI 下 1 像素 = 9525 EMU
const (
	emuPerInch    = 914400
	emuPerPixel96 = 9525
	// 最大图片宽度 6 inch，避免溢出 A4 页面
	maxImageWidthEMU = 6 * emuPerInch
	// pdfRenderTimeout 是"导出 PDF"里让 Chrome 渲染一页的上限。
	// 60s 不是新发明的一档：前端给导出类请求的 axios timeout 是 300000ms，
	// 而一次正常渲染是个位数秒；60s 已经把"页面里挂了一个取不到的远程资源"
	// 这种卡死同一次请求的浏览器进程封顶了。
	pdfRenderTimeout = 60 * time.Second
)

// buildDocxFromMarkdown 将 markdown 内容渲染为 .docx 字节流
// 支持：标题（H1-H3）、段落、图片嵌入。其他 markdown 语法按普通段落渲染。
// 纯 archive/zip + image 标准库实现，零第三方依赖。
// loadImage 由调用方注入（P3：db 块表优先读图，磁盘兜底），nil 时图片段落退化为占位文本。
func buildDocxFromMarkdown(title, mdContent string, loadImage func(src string) ([]byte, string, bool)) ([]byte, error) {
	bd := &docxBuilder{
		images:    []docxImage{},
		xmlEscape: template.HTMLEscapeString,
		loadImage: loadImage,
	}

	// 文档标题（标题样式）
	bd.addParagraph(title, "Title")

	for _, line := range strings.Split(mdContent, "\n") {
		trimmed := strings.TrimRight(line, "\r")

		if m := mdImgRegex.FindStringSubmatch(trimmed); m != nil {
			bd.addImageParagraph(m[1], m[2])
			continue
		}

		if style, text, ok := matchHeading(trimmed); ok {
			bd.addParagraph(text, style)
			continue
		}

		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		bd.addParagraph(trimmed, "")
	}

	return bd.build()
}

// matchHeading 识别 markdown 标题（#/##/###）
func matchHeading(line string) (string, string, bool) {
	switch {
	case strings.HasPrefix(line, "### "):
		return "Heading3", strings.TrimPrefix(line, "### "), true
	case strings.HasPrefix(line, "## "):
		return "Heading2", strings.TrimPrefix(line, "## "), true
	case strings.HasPrefix(line, "# "):
		return "Heading1", strings.TrimPrefix(line, "# "), true
	}
	return "", "", false
}

// docxImage 单个图片资源
type docxImage struct {
	rID       string // 关系 ID，如 rId2
	filename  string // word/media/imageN.png
	data      []byte
	widthEMU  int64
	heightEMU int64
}

// docxBuilder 渐进式构建 docx zip 包
type docxBuilder struct {
	paragraphs []string // 已渲染的 <w:p> 片段
	images     []docxImage
	xmlEscape  func(string) string
	loadImage  func(src string) ([]byte, string, bool)
}

// addParagraph 添加文本段落
func (b *docxBuilder) addParagraph(text, style string) {
	p := "<w:p>"
	if style != "" {
		p += `<w:pPr><w:pStyle w:val="` + style + `"/></w:pPr>`
	}
	p += `<w:r><w:t xml:space="preserve">` + b.xmlEscape(text) + `</w:t></w:r>`
	p += "</w:p>"
	b.paragraphs = append(b.paragraphs, p)
}

// addImageParagraph 添加图片段落
func (b *docxBuilder) addImageParagraph(alt, src string) {
	if b.loadImage == nil {
		b.addParagraph("[图片: "+alt+"]", "")
		return
	}
	data, ext, ok := b.loadImage(src)
	if !ok {
		b.addParagraph("[图片: "+alt+"]", "")
		return
	}
	ext = strings.ToLower(strings.TrimPrefix(ext, "."))
	if ext == "jpeg" {
		ext = "jpg"
	}
	if ext != "png" && ext != "jpg" && ext != "gif" && ext != "bmp" {
		ext = "png"
	}

	// 读取图片实际尺寸
	w, h := imageSize(data, ext)
	wEMU := int64(w) * emuPerPixel96
	hEMU := int64(h) * emuPerPixel96
	if wEMU > maxImageWidthEMU {
		scale := float64(maxImageWidthEMU) / float64(wEMU)
		wEMU = maxImageWidthEMU
		hEMU = int64(float64(hEMU) * scale)
	}
	if wEMU <= 0 || hEMU <= 0 {
		wEMU = 5 * emuPerInch
		hEMU = 3 * emuPerInch
	}

	imgIdx := len(b.images) + 2 // rId1 留给 styles
	rID := fmt.Sprintf("rId%d", imgIdx)
	filename := fmt.Sprintf("image%d.%s", imgIdx-1, ext)
	docPrID := imgIdx // wp:docPr id 必须是数字

	b.images = append(b.images, docxImage{
		rID:       rID,
		filename:  filename,
		data:      data,
		widthEMU:  wEMU,
		heightEMU: hEMU,
	})

	// <w:drawing> 上声明所有命名空间，按 python-docx 标准模板构建
	// 关键：wp 命名空间(WordprocessingDrawing) 必须显式声明，否则 XML 解析失败 → Word 打不开
	drawing := `<w:p><w:r><w:drawing>` +
		`<wp:inline distT="0" distB="0" distL="0" distR="0" xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing" xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">` +
		fmt.Sprintf(`<wp:extent cx="%d" cy="%d"/>`, wEMU, hEMU) +
		fmt.Sprintf(`<wp:docPr id="%d" name="Picture %d"/>`, docPrID, docPrID) +
		`<wp:cNvGraphicFramePr><a:graphicFrameLocks noChangeAspect="1"/></wp:cNvGraphicFramePr>` +
		`<a:graphic>` +
		`<a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">` +
		`<pic:pic>` +
		fmt.Sprintf(`<pic:nvPicPr><pic:cNvPr id="%d" name="%s"/><pic:cNvPicPr/></pic:nvPicPr>`, docPrID, filename) +
		`<pic:blipFill>` +
		`<a:blip r:embed="` + rID + `"/>` +
		`<a:stretch><a:fillRect/></a:stretch>` +
		`</pic:blipFill>` +
		fmt.Sprintf(`<pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm><a:prstGeom prst="rect"/></pic:spPr>`, wEMU, hEMU) +
		`</pic:pic>` +
		`</a:graphicData>` +
		`</a:graphic>` +
		`</wp:inline>` +
		`</w:drawing></w:r></w:p>`
	b.paragraphs = append(b.paragraphs, drawing)
}

// build 生成最终的 .docx 字节流
func (b *docxBuilder) build() ([]byte, error) {
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)

	// 1. [Content_Types].xml — 对齐 python-docx 标准结构
	contentTypes := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">` +
		`<Default Extension="rels" ContentType="application/vnd.openxmlformats-package.relationships+xml"/>` +
		`<Default Extension="xml" ContentType="application/xml"/>`
	seenExt := map[string]bool{}
	for _, img := range b.images {
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(img.filename), "."))
		if !seenExt[ext] {
			mt := mime.TypeByExtension("." + ext)
			if mt == "" {
				mt = "image/png"
			}
			contentTypes += `<Default Extension="` + ext + `" ContentType="` + mt + `"/>`
			seenExt[ext] = true
		}
	}
	contentTypes +=
		`<Override PartName="/word/document.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.document.main+xml"/>` +
			`<Override PartName="/word/styles.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.styles+xml"/>` +
			`<Override PartName="/word/settings.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.settings+xml"/>` +
			`<Override PartName="/word/fontTable.xml" ContentType="application/vnd.openxmlformats-officedocument.wordprocessingml.fontTable+xml"/>` +
			`</Types>`
	if err := addToZip(zw, "[Content_Types].xml", contentTypes); err != nil {
		return nil, err
	}

	// 2. _rels/.rels
	if err := addToZip(zw, "_rels/.rels",
		`<?xml version="1.0" encoding="UTF-8" standalone="yes"?>`+"\n"+
			`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">`+
			`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/officeDocument" Target="word/document.xml"/>`+
			`</Relationships>`); err != nil {
		return nil, err
	}

	// 3. word/_rels/document.xml.rels — 包含 styles/settings/fontTable/image
	rels := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<Relationships xmlns="http://schemas.openxmlformats.org/package/2006/relationships">` +
		`<Relationship Id="rId1" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/styles" Target="styles.xml"/>` +
		`<Relationship Id="rIdSettings" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/settings" Target="settings.xml"/>` +
		`<Relationship Id="rIdFont" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/fontTable" Target="fontTable.xml"/>`
	for _, img := range b.images {
		rels += `<Relationship Id="` + img.rID + `" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image" Target="media/` + img.filename + `"/>`
	}
	rels += `</Relationships>`
	if err := addToZip(zw, "word/_rels/document.xml.rels", rels); err != nil {
		return nil, err
	}

	// 4. word/styles.xml
	if err := addToZip(zw, "word/styles.xml", docxStylesXML); err != nil {
		return nil, err
	}

	// 5. word/settings.xml — Word 必需部件
	if err := addToZip(zw, "word/settings.xml", docxSettingsXML); err != nil {
		return nil, err
	}

	// 6. word/fontTable.xml — Word 必需部件
	if err := addToZip(zw, "word/fontTable.xml", docxFontTableXML); err != nil {
		return nil, err
	}

	// 7. word/document.xml
	body := strings.Join(b.paragraphs, "")
	docXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` + "\n" +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main" xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><w:body>` +
		body +
		`<w:sectPr><w:pgSz w:w="11906" w:h="16838"/><w:pgMar w:top="1440" w:right="1440" w:bottom="1440" w:left="1440" w:header="720" w:footer="720"/></w:sectPr>` +
		`</w:body></w:document>`
	if err := addToZip(zw, "word/document.xml", docXML); err != nil {
		return nil, err
	}

	// 8. word/media/imageN.ext
	for _, img := range b.images {
		if err := addToZipBytes(zw, "word/media/"+img.filename, img.data); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, err
	}
	return zipBuf.Bytes(), nil
}

// docxStylesXML 最小化样式表：Normal + Heading1/2/3 + Title
const docxStylesXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:styles xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:style w:type="paragraph" w:styleId="Normal" w:default="1">
    <w:name w:val="Normal"/>
    <w:rPr><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/><w:sz w:val="22"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Title">
    <w:name w:val="Title"/>
    <w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="56"/><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading1">
    <w:name w:val="heading 1"/>
    <w:pPr><w:spacing w:before="240" w:after="120"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="48"/><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading2">
    <w:name w:val="heading 2"/>
    <w:pPr><w:spacing w:before="200" w:after="100"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="36"/><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/></w:rPr>
  </w:style>
  <w:style w:type="paragraph" w:styleId="Heading3">
    <w:name w:val="heading 3"/>
    <w:pPr><w:spacing w:before="160" w:after="80"/></w:pPr>
    <w:rPr><w:b/><w:sz w:val="28"/><w:rFonts w:ascii="Calibri" w:hAnsi="Calibri"/></w:rPr>
  </w:style>
</w:styles>`

// docxSettingsXML Word settings.xml 最小化模板
const docxSettingsXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:settings xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:defaultTabStop w:val="709"/>
  <w:characterSpacingControl w:val="compressPunctuation"/>
</w:settings>`

// docxFontTableXML Word fontTable.xml 最小化模板
const docxFontTableXML = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:fonts xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:font w:name="Calibri">
    <w:panose1 w:val="020F0502040202020203"/>
    <w:charset w:val="00"/>
    <w:family w:val="modern"/>
    <w:pitch w:val="variable"/>
  </w:font>
  <w:font w:name="Arial">
    <w:panose1 w:val="020B0604020202020203"/>
    <w:charset w:val="00"/>
    <w:family w:val="modern"/>
    <w:pitch w:val="variable"/>
  </w:font>
  <w:font w:name="Times New Roman">
    <w:panose1 w:val="02020603050405020304"/>
    <w:charset w:val="00"/>
    <w:family w:val="roman"/>
    <w:pitch w:val="variable"/>
  </w:font>
</w:fonts>`

func addToZip(zw *zip.Writer, name, content string) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(content))
	return err
}

func addToZipBytes(zw *zip.Writer, name string, data []byte) error {
	w, err := zw.Create(name)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// imageSize 解码图片字节数据获取实际像素尺寸
// 失败时返回 0,0（调用方需兜底处理）
func imageSize(data []byte, ext string) (w, h int) {
	cfg, _, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// ExportDocumentPDF 导出文档为 PDF（使用 chromedp 服务端渲染）
func (api *DocumentAPI) ExportDocumentPDF(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	doc, err := api.docService.GetDocumentByID(id)
	if err != nil || doc == nil || doc.IsDelete == 1 {
		response.NotFound(c, "document not found")
		return
	}

	// 导出/分享前必须确认调用者对该文档所在空间可见：只校验登录等于任何人都能取走他人私有正文
	actorUID, actorOK := middleware.RequireUser(c)
	if !actorOK {
		return
	}
	if err := api.docService.EnsureDocReadable(id, actorUID); err != nil {
		if err == workspacesvc.ErrNoDocumentPermission {
			response.ForbiddenKey(c, response.MsgNoPermissionExportDoc)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	space, err := api.spaceService.GetSpaceByID(doc.SpaceID)
	if err != nil || space == nil {
		response.Forbidden(c, "this document cannot be exported")
		return
	}
	if space.IsExport == 0 {
		response.Forbidden(c, "space export is disabled")
		return
	}

	docContent, err := api.docService.GetDocumentContent(id)
	if err != nil {
		response.ServerError(c, "failed to read document content")
		return
	}

	// Markdown -> HTML body
	var buf bytes.Buffer
	md := goldmark.New(goldmark.WithExtensions(extension.Table, extension.Strikethrough))
	if err := md.Convert([]byte(docContent), &buf); err != nil {
		response.ServerError(c, "failed to render markdown")
		return
	}
	// 将图片相对路径替换为 base64 data URI
	renderedHTML := api.embedImagesInHTML(buf.String())
	bodyHTML := template.HTML(renderedHTML) //nolint:gosec // rendered markdown intended here

	safeName := template.HTMLEscapeString(doc.Name)

	// Build print-optimized HTML page
	htmlPage := `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>` + safeName + `</title>
<style>
  @page { margin: 2cm; size: A4; }
  *, *::before, *::after { box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", Arial, "PingFang SC", "Hiragino Sans GB", "Microsoft YaHei", sans-serif;
    max-width: 800px;
    margin: 0 auto;
    padding: 40px 24px 80px;
    color: #1d1d1f;
    line-height: 1.7;
    background: #fff;
  }
  h1 { font-size: 2em; font-weight: 700; margin: 0 0 8px; letter-spacing: -0.02em; }
  h2 { font-size: 1.5em; font-weight: 600; margin-top: 2em; border-bottom: 1px solid #e5e5e7; padding-bottom: 0.3em; }
  h3 { font-size: 1.2em; font-weight: 600; margin-top: 1.5em; }
  p { margin: 0.8em 0; }
  code {
    background: #f5f5f7;
    padding: 2px 6px;
    border-radius: 4px;
    font-size: 0.9em;
    font-family: "SF Mono", Menlo, Consolas, monospace;
  }
  pre {
    background: #f5f5f7;
    padding: 16px;
    border-radius: 8px;
    overflow-x: auto;
    line-height: 1.5;
  }
  pre code { background: none; padding: 0; }
  blockquote {
    border-left: 4px solid #e5e5e7;
    margin: 1em 0;
    padding: 0.5em 1em;
    color: #6e6e73;
  }
  table { border-collapse: collapse; width: 100%; margin: 1em 0; }
  th, td { border: 1px solid #e5e5e7; padding: 8px 12px; text-align: left; }
  th { background: #f5f5f7; font-weight: 600; }
  img { max-width: 100%; height: auto; border-radius: 8px; }
  a { color: #0066cc; text-decoration: none; }
  hr { border: none; border-top: 1px solid #e5e5e7; margin: 2em 0; }
  ul, ol { padding-left: 1.5em; }
  li { margin: 0.3em 0; }
  .meta { color: #6e6e73; font-size: 0.85em; margin-bottom: 2em; }
  @media print {
    body { padding: 0; }
    .no-print { display: none; }
  }
</style>
</head>
<body>
<h1>` + safeName + `</h1>
<div class="meta">` + template.HTMLEscapeString(space.Name) + `</div>
<div class="content">` + string(bodyHTML) + `</div>
</body>
</html>`

	// Use chromedp to render HTML to PDF
	// 必须带超时：这条 ctx 原先是裸 context.Background()，而 chromedp 底下是一个真实的
	// Chrome 进程 —— 页面里挂着取不到的远程资源时 Run 会一直等，
	// 于是"导出 PDF"这个请求连同它拉起的浏览器一起卡死，只能靠重启进程解。
	// 判据也只能是超时：handler 已经攒完整份 HTML 才走到这里，客户端断开不会打断渲染。
	renderCtx, cancel := context.WithTimeout(c.Request.Context(), pdfRenderTimeout)
	defer cancel()
	ctx, cancelCtx := chromedp.NewContext(renderCtx)
	defer cancelCtx()

	var pdfBuf []byte
	if err := chromedp.Run(ctx,
		chromedp.Navigate("data:text/html;charset=utf-8,"+url.PathEscape(htmlPage)),
		chromedp.WaitReady("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			data, _, err := page.PrintToPDF().
				WithPrintBackground(true).
				WithPaperWidth(8.27).
				WithPaperHeight(11.69).
				WithMarginTop(0.79).
				WithMarginBottom(0.79).
				WithMarginLeft(0.79).
				WithMarginRight(0.79).
				Do(ctx)
			pdfBuf = data
			return err
		}),
	); err != nil {
		logx.Warnf("[ExportPDF] chromedp failed, falling back to HTML: %v", err)
		// Fallback to HTML export if chromedp fails
		c.Header("Content-Type", "text/html; charset=utf-8")
		safeFileName := doc.Name
		for _, ch := range []string{"/", "\\", ":", "*", "?", "<", ">", "|", "\""} {
			safeFileName = strings.ReplaceAll(safeFileName, ch, "_")
		}
		c.Header("Content-Disposition", fmt.Sprintf("inline; filename*=UTF-8''%s", url.PathEscape(safeFileName+".pdf.html")))
		c.Header("Cache-Control", "no-store")
		c.String(200, htmlPage)
		return
	}

	safeFileName := doc.Name
	for _, ch := range []string{"/", "\\", ":", "*", "?", "<", ">", "|", "\""} {
		safeFileName = strings.ReplaceAll(safeFileName, ch, "_")
	}

	c.Header("Content-Type", "application/pdf")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(safeFileName+".pdf")))
	c.Header("Content-Length", strconv.Itoa(len(pdfBuf)))
	c.Header("Cache-Control", "no-store")
	c.Data(200, "application/pdf", pdfBuf)
}

// GetBacklinks 获取引用当前文档的其他文档
func (api *DocumentAPI) GetBacklinks(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	backlinks, err := api.docService.GetBacklinks(id)
	if err != nil {
		response.ServerError(c, "get backlinks failed")
		return
	}
	if backlinks == nil {
		backlinks = []*workspacerepo.BacklinkResult{}
	}

	response.Success(c, backlinks)
}

// ExportDocumentMarkdown 导出文档为 Markdown (.md) 文件
func (api *DocumentAPI) ExportDocumentMarkdown(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	doc, err := api.docService.GetDocumentByID(id)
	if err != nil || doc == nil || doc.IsDelete == 1 {
		response.NotFound(c, "document not found")
		return
	}

	// 导出/分享前必须确认调用者对该文档所在空间可见：只校验登录等于任何人都能取走他人私有正文
	actorUID, actorOK := middleware.RequireUser(c)
	if !actorOK {
		return
	}
	if err := api.docService.EnsureDocReadable(id, actorUID); err != nil {
		if err == workspacesvc.ErrNoDocumentPermission {
			response.ForbiddenKey(c, response.MsgNoPermissionExportDoc)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	space, err := api.spaceService.GetSpaceByID(doc.SpaceID)
	if err != nil || space == nil {
		response.Forbidden(c, "this document cannot be exported")
		return
	}
	if space.IsExport == 0 {
		response.Forbidden(c, "space export is disabled")
		return
	}

	docContent, err := api.docService.GetDocumentContent(id)
	if err != nil {
		response.ServerError(c, "failed to read document content")
		return
	}

	// 将图片相对路径替换为 base64 data URI，确保导出后图片可查看
	mdContent := "# " + doc.Name + "\n\n" + api.embedImagesInMarkdown(docContent)

	safeName := doc.Name
	for _, ch := range []string{"/", "\\", ":", "*", "?", "<", ">", "|", "\""} {
		safeName = strings.ReplaceAll(safeName, ch, "_")
	}

	c.Header("Content-Type", "text/markdown; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", url.PathEscape(safeName+".md")))
	c.Header("Cache-Control", "no-store")
	c.String(200, mdContent)
}

// storedAttachmentPath 把文档内图片 URL 归一成 aw_attachment.path 的存储形式
// （images/... 或 attachment/...），供 db 块表按路径取件；无法识别返回空串。
// 前缀映射对齐 server.go 的静态路由：/attachments/ URL 对应存储前缀 attachment/（无 s）。
func storedAttachmentPath(rawSrc string) string {
	if strings.HasPrefix(rawSrc, "http") {
		u, err := url.Parse(rawSrc)
		if err != nil {
			return ""
		}
		rawSrc = u.Path
	}
	rawSrc = strings.SplitN(rawSrc, "?", 2)[0]
	switch {
	case strings.HasPrefix(rawSrc, "/images/"):
		return strings.TrimPrefix(rawSrc, "/")
	case strings.HasPrefix(rawSrc, "/attachments/"):
		return "attachment/" + strings.TrimPrefix(rawSrc, "/attachments/")
	case strings.HasPrefix(rawSrc, "images/"):
		return rawSrc
	case strings.HasPrefix(rawSrc, "attachments/"):
		return "attachment/" + strings.TrimPrefix(rawSrc, "attachments/")
	}
	return ""
}

// loadImageForEmbed 解析文档内图片引用为 (字节, 扩展名含点)。db 块表优先，磁盘兜底。
func (api *DocumentAPI) loadImageForEmbed(rawSrc string) ([]byte, string, bool) {
	stored := storedAttachmentPath(rawSrc)
	if stored == "" {
		return nil, "", false
	}
	data, err := api.attachmentService.ReadAttachmentBytes(stored)
	if err != nil {
		return nil, "", false
	}
	return data, strings.ToLower(filepath.Ext(stored)), true
}

// embedImagesInHTML 将 HTML 中的图片相对路径替换为 base64 data URI
func (api *DocumentAPI) embedImagesInHTML(htmlStr string) string {
	imgSrcRegex := regexp.MustCompile(`src="(https?://[^"]*?/(images|attachments)/[^"]*|/(images|attachments)/[^"]*|images/[^"]*|attachments/[^"]*)"`)
	return imgSrcRegex.ReplaceAllStringFunc(htmlStr, func(match string) string {
		sub := imgSrcRegex.FindStringSubmatch(match)
		if len(sub) < 2 {
			return match
		}
		data, ext, ok := api.loadImageForEmbed(sub[1])
		if !ok {
			return match
		}
		mimeType := mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = "image/png"
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		return `src="data:` + mimeType + ";base64," + b64 + `"`
	})
}

// embedImagesInMarkdown 将 Markdown 中的图片相对路径替换为 base64 data URI
func (api *DocumentAPI) embedImagesInMarkdown(mdStr string) string {
	mdImgRegex := regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`)
	return mdImgRegex.ReplaceAllStringFunc(mdStr, func(match string) string {
		sub := mdImgRegex.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		alt := sub[1]
		data, ext, ok := api.loadImageForEmbed(sub[2])
		if !ok {
			return match
		}
		mimeType := mime.TypeByExtension(ext)
		if mimeType == "" {
			mimeType = "image/png"
		}
		b64 := base64.StdEncoding.EncodeToString(data)
		return `![` + alt + `](data:` + mimeType + ";base64," + b64 + `)`
	})
}

// GetForwardLinks 获取当前文档引用的其他文档
func (api *DocumentAPI) GetForwardLinks(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	forwardLinks, err := api.docService.GetForwardLinks(id)
	if err != nil {
		response.ServerError(c, "get forward links failed")
		return
	}
	if forwardLinks == nil {
		forwardLinks = []*workspacerepo.ForwardLinkResult{}
	}

	response.Success(c, forwardLinks)
}

// S5: GetRecommendations 获取相关文档推荐
func (api *DocumentAPI) GetRecommendations(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "5"))
	if limit <= 0 || limit > 20 {
		limit = 5
	}

	var recommendations []map[string]interface{}

	// 1. 向量相似度推荐
	m := ai.GetModule()
	if m.Enabled() && m.Embedder() != nil {
		vectorResults := embedding.VectorSearch(fmt.Sprintf("doc:%d", id), 0, limit+1, m.Embedder())
		for _, vr := range vectorResults {
			if vr.DocID == id {
				continue
			}
			recommendations = append(recommendations, map[string]interface{}{
				"document_id":   vr.DocID,
				"document_name": vr.Name,
				"space_id":      vr.SpaceID,
				"score":         vr.Score,
				"reason":        "语义相似",
			})
		}
	}

	// 2. 文档链接关系推荐
	if len(recommendations) < limit {
		forwardLinks, _ := api.docService.GetForwardLinks(id)
		if forwardLinks != nil {
			for _, link := range forwardLinks {
				if len(recommendations) >= limit {
					break
				}
				alreadyAdded := false
				for _, r := range recommendations {
					if r["document_id"] == link.TargetDocID {
						alreadyAdded = true
						break
					}
				}
				if !alreadyAdded {
					recommendations = append(recommendations, map[string]interface{}{
						"document_id":   link.TargetDocID,
						"document_name": link.TargetDocName,
						"space_id":      link.SpaceID,
						"score":         0.5,
						"reason":        "文档链接",
					})
				}
			}
		}

		backlinks, _ := api.docService.GetBacklinks(id)
		if backlinks != nil {
			for _, link := range backlinks {
				if len(recommendations) >= limit {
					break
				}
				alreadyAdded := false
				for _, r := range recommendations {
					if r["document_id"] == link.SourceDocID {
						alreadyAdded = true
						break
					}
				}
				if !alreadyAdded {
					recommendations = append(recommendations, map[string]interface{}{
						"document_id":   link.SourceDocID,
						"document_name": link.SourceDocName,
						"space_id":      link.SpaceID,
						"score":         0.4,
						"reason":        "反向引用",
					})
				}
			}
		}
	}

	if recommendations == nil {
		recommendations = []map[string]interface{}{}
	}

	logx.Infof("Recommendations for doc %d: %d results", id, len(recommendations))
	response.Success(c, gin.H{"recommendations": recommendations})
}

// S17: 编辑冲突预判 API
func (da *DocumentAPI) LockEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		response.BadRequest(c, "invalid document id")
		return
	}
	userID, _ := middleware.GetUserID(c)

	// Parse optional permanent flag from request body
	var req struct {
		Permanent bool `json:"permanent"`
	}
	_ = c.ShouldBindJSON(&req) // Ignore error - body is optional

	if err := da.docService.LockForEdit(id, userID, req.Permanent); err != nil {
		// 锁被其他用户占用 → 返回 409 Conflict 并附带当前持有者信息
		if errors.Is(err, workspacerepo.ErrEditLockHeldByOther) {
			conflict := da.docService.CheckEditConflict(id, userID)
			response.JSON(c, http.StatusConflict, 40901, "document is locked by another user", conflict)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"permanent": req.Permanent})
}

func (da *DocumentAPI) UnlockEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		response.BadRequest(c, "invalid document id")
		return
	}
	userID, _ := middleware.GetUserID(c)
	if err := da.docService.UnlockEdit(id, userID); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, nil)
}

// HeartbeatEdit 编辑锁心跳续约
func (da *DocumentAPI) HeartbeatEdit(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		response.BadRequest(c, "invalid document id")
		return
	}
	userID, _ := middleware.GetUserID(c)
	if err := da.docService.HeartbeatEdit(id, userID); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, nil)
}

func (da *DocumentAPI) CheckEditConflict(c *gin.Context) {
	id, _ := strconv.Atoi(c.Param("id"))
	if id <= 0 {
		response.BadRequest(c, "invalid document id")
		return
	}
	userID, _ := middleware.GetUserID(c)
	info := da.docService.CheckEditConflict(id, userID)
	response.Success(c, info)
}
