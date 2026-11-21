package workspace

import (
	"database/sql"
	"errors"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
	"time"
)

var (
	ErrAttachmentNotFound = errors.New("attachment not found")
)

// AttachmentRepository 附件数据访问层
type AttachmentRepository struct {
	db *gorm.DB
}

func NewAttachmentRepository() *AttachmentRepository {
	return &AttachmentRepository{db: repos.DB}
}

func NewAttachmentRepositoryWithDB(db *gorm.DB) *AttachmentRepository {
	return &AttachmentRepository{db: db}
}

func (r *AttachmentRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// Create 创建附件记录
func (r *AttachmentRepository) Create(attachment *workspacemodel.Attachment) error {
	attachment.CreateTime = int(time.Now().Unix())
	attachment.UpdateTime = attachment.CreateTime
	return r.db.Create(attachment).Error
}

// FindByID 根据 ID 查找附件
func (r *AttachmentRepository) FindByID(id int) (*workspacemodel.Attachment, error) {
	var a workspacemodel.Attachment
	err := r.db.First(&a, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAttachmentNotFound
		}
		return nil, err
	}
	return &a, nil
}

// FindByDocumentID 获取文档的所有附件
func (r *AttachmentRepository) FindByDocumentID(documentID int) ([]*workspacemodel.Attachment, error) {
	var attachments []*workspacemodel.Attachment
	err := r.db.Where("document_id = ?", documentID).Find(&attachments).Error
	return attachments, err
}

// FindByDocIDs 批量按文档 ID 取附件（空间导出等场景避免逐文档查询的 N+1）。
func (r *AttachmentRepository) FindByDocIDs(documentIDs []int) ([]*workspacemodel.Attachment, error) {
	if len(documentIDs) == 0 {
		return nil, nil
	}
	var attachments []*workspacemodel.Attachment
	err := r.db.Where("document_id IN ?", documentIDs).Find(&attachments).Error
	return attachments, err
}

// FindByDocumentIDAndSource 根据文档ID和来源类型获取附件
func (r *AttachmentRepository) FindByDocumentIDAndSource(documentID int, source int) ([]*workspacemodel.Attachment, error) {
	var attachments []*workspacemodel.Attachment
	err := r.db.Where("document_id = ? AND source = ?", documentID, source).Find(&attachments).Error
	return attachments, err
}

// FindByUserID 获取用户上传的所有附件
func (r *AttachmentRepository) FindByUserID(userID int, limit int) ([]*workspacemodel.Attachment, error) {
	var attachments []*workspacemodel.Attachment
	query := r.db.Where("user_id = ?", userID).Order("create_time DESC")
	if limit > 0 {
		query = query.Limit(limit)
	}
	err := query.Find(&attachments).Error
	return attachments, err
}

// Delete 删除附件记录
func (r *AttachmentRepository) Delete(id int) error {
	return r.db.Delete(&workspacemodel.Attachment{}, id).Error
}

// FindByPath 根据存储路径查找附件（用于静态文件服务时获取原始文件名）
func (r *AttachmentRepository) FindByPath(path string) (*workspacemodel.Attachment, error) {
	var a workspacemodel.Attachment
	err := r.db.Where("path = ?", path).First(&a).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrAttachmentNotFound
		}
		return nil, err
	}
	return &a, nil
}

// DeleteByDocumentID 删除文档的所有附件记录
func (r *AttachmentRepository) DeleteByDocumentID(documentID int) error {
	return r.db.Where("document_id = ?", documentID).Delete(&workspacemodel.Attachment{}).Error
}
