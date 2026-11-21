package workspace

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	logx "github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/middleware"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
)

type AttachmentAPI struct {
	attachmentService *workspacesvc.AttachmentService
}

func NewAttachmentAPI() *AttachmentAPI {
	return &AttachmentAPI{
		attachmentService: workspacesvc.NewAttachmentService(),
	}
}

func (api *AttachmentAPI) UploadAttachment(c *gin.Context) {
	logx.Infof("[UploadAttachment] === 请求开始 ===")
	documentIDStr := c.Param("document_id")
	logx.Infof("[UploadAttachment] document_id 参数: %s", documentIDStr)
	documentID, err := strconv.Atoi(documentIDStr)
	if err != nil || documentID <= 0 {
		logx.Warnf("[UploadAttachment] 无效的文档ID: %s, err=%v", documentIDStr, err)
		response.BadRequestKey(c, response.MsgInvalidDocID)
		return
	}

	logx.Infof("[UploadAttachment] 准备读取文件, documentID=%d", documentID)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		logx.Errorf("[UploadAttachment] 读取文件失败: %v", err)
		response.BadRequestKey(c, response.MsgNoFileUploaded)
		return
	}
	defer file.Close()
	logx.Infof("[UploadAttachment] 文件读取成功: name=%s, size=%d, content_type=%s", header.Filename, header.Size, header.Header.Get("Content-Type"))

	userID, err := middleware.GetUserID(c)
	if err != nil {
		logx.Warnf("[UploadAttachment] 获取用户ID失败: %v", err)
		response.UnauthorizedKey(c, response.MsgNotSignedIn)
		return
	}
	logx.Infof("[UploadAttachment] 用户ID=%d, 开始调用 service", userID)

	result, err := api.attachmentService.UploadAttachment(documentID, file, header, userID)
	if err != nil {
		logx.Errorf("[UploadAttachment] service 返回错误: %v (类型=%T)", err, err)
		switch err {
		case workspacesvc.ErrDocumentNotFound:
			response.BadRequestKey(c, response.MsgDocNotFound)
		case workspacesvc.ErrInvalidFilename, workspacesvc.ErrUnsupportedFileType, workspacesvc.ErrFileTooLarge, workspacesvc.ErrFileValidationFailed:
			response.BadRequest(c, err.Error())
		case workspacesvc.ErrNoEditPermission:
			response.ForbiddenKey(c, response.MsgNoPermissionUploadAttach)
		default:
			response.ServerErrorSafe(c, err)
		}
		return
	}

	logx.Infof("[UploadAttachment] 上传成功: attachmentID=%d, url=%s", result.ID, result.URL)
	response.Success(c, result)
}

func (api *AttachmentAPI) GetDocumentAttachments(c *gin.Context) {
	documentIDStr := c.Param("document_id")
	documentID, err := strconv.Atoi(documentIDStr)
	if err != nil || documentID <= 0 {
		response.BadRequestKey(c, response.MsgInvalidDocID)
		return
	}

	sourceStr := c.DefaultQuery("source", "-1")
	source, err := strconv.Atoi(sourceStr)
	if err != nil {
		response.BadRequestKey(c, response.MsgInvalidSourceKind)
		return
	}

	attachments, err := api.attachmentService.GetDocumentAttachments(documentID, source)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, attachments)
}

func (api *AttachmentAPI) DeleteAttachment(c *gin.Context) {
	attachmentIDStr := c.Param("id")
	attachmentID, err := strconv.Atoi(attachmentIDStr)
	if err != nil || attachmentID <= 0 {
		response.BadRequestKey(c, response.MsgInvalidAttachmentID)
		return
	}

	userID, err := middleware.GetUserID(c)
	if err != nil {
		response.UnauthorizedKey(c, response.MsgNotSignedIn)
		return
	}

	err = api.attachmentService.DeleteAttachment(attachmentID, userID)
	if err != nil {
		if err == workspacesvc.ErrAttachmentNotFound {
			response.NotFoundKey(c, response.MsgAttachmentNotFound)
			return
		}
		if err == workspacesvc.ErrNoEditPermission {
			response.ForbiddenKey(c, response.MsgNoPermissionDeleteAttach)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, nil)
}

func (api *AttachmentAPI) DownloadAttachment(c *gin.Context) {
	uid, err := middleware.GetUserID(c)
	if err != nil {
		response.UnauthorizedKey(c, response.MsgNotSignedIn)
		return
	}

	attachmentIDStr := c.Param("id")
	attachmentID, err := strconv.Atoi(attachmentIDStr)
	if err != nil || attachmentID <= 0 {
		response.BadRequestKey(c, response.MsgInvalidAttachmentID)
		return
	}

	reader, fileName, size, err := api.attachmentService.DownloadAttachment(attachmentID, uid)
	if err != nil {
		switch err {
		case workspacesvc.ErrAttachmentNotFound, workspacesvc.ErrDocumentNotFound, workspacesvc.ErrSpaceNotFound, workspacesvc.ErrFileNotFound:
			response.NotFound(c, err.Error())
		case workspacesvc.ErrNoAccessPermission:
			response.ForbiddenKey(c, response.MsgNoPermissionReadAttach)
		default:
			response.ServerErrorSafe(c, err)
		}
		return
	}
	defer reader.Close()

	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename*=UTF-8''%s", workspacesvc.GetAttachmentFilenameForDownload(fileName)))
	c.DataFromReader(200, size, "application/octet-stream", reader, nil)
}
