package workspace

import (
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

// ImageService 仅保留图片查询能力；
// 图片上传已统一并入 AttachmentService.UploadAttachment（AttachmentSourceImage 由 MIME/扩展名自动识别）。
type ImageService struct {
	attachmentRepo *workspacerepo.AttachmentRepository
}

func NewImageService() *ImageService {
	return &ImageService{
		attachmentRepo: workspacerepo.NewAttachmentRepository(),
	}
}

func (s *ImageService) GetDocumentImages(documentID int) ([]workspacemodel.AttachmentResponse, error) {
	attachments, err := s.attachmentRepo.FindByDocumentIDAndSource(documentID, workspacemodel.AttachmentSourceImage)
	if err != nil {
		return nil, err
	}

	responses := make([]workspacemodel.AttachmentResponse, len(attachments))
	for i, a := range attachments {
		responses[i] = a.ToResponse()
	}
	return responses, nil
}
