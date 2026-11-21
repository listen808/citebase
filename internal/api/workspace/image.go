package workspace

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
)

// ImageAPI 仅保留图片查询能力；
// 图片上传已统一并入 AttachmentAPI.UploadAttachment (POST /attachments/upload/:document_id)。
type ImageAPI struct {
	imageService *workspacesvc.ImageService
}

func NewImageAPI() *ImageAPI {
	return &ImageAPI{
		imageService: workspacesvc.NewImageService(),
	}
}

func (api *ImageAPI) GetDocumentImages(c *gin.Context) {
	documentIDStr := c.Param("document_id")
	documentID, err := strconv.Atoi(documentIDStr)
	if err != nil || documentID <= 0 {
		response.BadRequest(c, "invalid document id")
		return
	}

	images, err := api.imageService.GetDocumentImages(documentID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, images)
}
