package workspace

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
)

// VersionTagAPI handles version tag API requests.
type VersionTagAPI struct {
	service *workspacesvc.DocumentVersionTagService
}

func NewVersionTagAPI() *VersionTagAPI {
	return &VersionTagAPI{
		service: workspacesvc.NewDocumentVersionTagService(),
	}
}

// CreateVersionTag creates a new version tag.
// @Summary Create a version tag
// @Tags version-tag
// @Accept json
// @Produce json
// @Param id path int true "Document ID"
// @Param body body workspace.CreateVersionTagRequest true "Version tag data"
// @Success 200 {object} response.Response{data=workspace.DocumentVersionTagResponse}
// @Router /documents/{id}/version-tags [post]
func (api *VersionTagAPI) CreateVersionTag(c *gin.Context) {
	documentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}
	// 版本标签是空间内数据：只有该文档所属空间的成员才能创建
	if !requireDocMember(c, int(documentID)) {
		return
	}

	userID, _ := middleware.GetUserID(c)

	var req workspacemodel.CreateVersionTagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tag, err := api.service.CreateTag(int64(userID), documentID, &req)
	if err != nil {
		if err == workspacesvc.ErrVersionTagExists {
			response.ErrorWithErr(c, err, http.StatusConflict)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, tag)
}

// GetDocumentVersionTags returns all version tags for a document.
// @Summary Get document version tags
// @Tags version-tag
// @Produce json
// @Param id path int true "Document ID"
// @Success 200 {object} response.Response{data=[]workspace.DocumentVersionTagResponse}
// @Router /documents/{id}/version-tags [get]
func (api *VersionTagAPI) GetDocumentVersionTags(c *gin.Context) {
	documentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	// 版本标签含里程碑说明与编辑者信息，按文档所属空间校验可读性
	if !requireDocReadable(c, int(documentID)) {
		return
	}

	tags, err := api.service.GetDocumentVersionTags(documentID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, tags)
}

// GetVersionTags returns tags for a specific version.
// @Summary Get version tags
// @Tags version-tag
// @Produce json
// @Param id path int true "Document ID"
// @Param version path int true "Version number"
// @Success 200 {object} response.Response{data=[]workspace.DocumentVersionTagResponse}
// @Router /documents/{id}/versions/{version}/tags [get]
func (api *VersionTagAPI) GetVersionTags(c *gin.Context) {
	documentID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid document id")
		return
	}

	version, err := strconv.Atoi(c.Param("version"))
	if err != nil {
		response.BadRequest(c, "invalid version number")
		return
	}
	if !requireDocReadable(c, int(documentID)) {
		return
	}

	tags, err := api.service.GetVersionTags(documentID, version)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, tags)
}

// DeleteVersionTag removes a version tag by ID.
// @Summary Delete a version tag
// @Tags version-tag
// @Param tagId path int true "Tag ID"
// @Success 200 {object} response.Response
// @Router /version-tags/{tagId} [delete]
func (api *VersionTagAPI) DeleteVersionTag(c *gin.Context) {
	tagID, err := strconv.ParseInt(c.Param("tagId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid tag id")
		return
	}
	// 按 tagId 反查所属文档再校验成员身份，否则是典型 IDOR 删除
	if !requireVersionTagWritable(c, tagID) {
		return
	}

	if err := api.service.DeleteTag(tagID); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, nil)
}

// GetSpaceVersionTags returns all version tags in a space.
// @Summary Get space version tags
// @Tags version-tag
// @Produce json
// @Param id path int true "Space ID"
// @Success 200 {object} response.Response{data=[]workspace.DocumentVersionTagResponse}
// @Router /spaces/{id}/version-tags [get]
func (api *VersionTagAPI) GetSpaceVersionTags(c *gin.Context) {
	spaceID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid space id")
		return
	}

	if !requireSpaceReadable(c, int(spaceID)) {
		return
	}

	tags, err := api.service.GetSpaceVersionTags(spaceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, tags)
}
