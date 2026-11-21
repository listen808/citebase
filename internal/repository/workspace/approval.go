package workspace

import (
	"errors"
	"fmt"
	"time"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

// ErrApprovalNotFound 审批记录不存在
var ErrApprovalNotFound = errors.New("approval not found")

// DocumentApprovalRepository 文档审批流数据访问层
type DocumentApprovalRepository struct {
	db *gorm.DB
}

// NewDocumentApprovalRepository 创建仓储实例
func NewDocumentApprovalRepository() *DocumentApprovalRepository {
	return &DocumentApprovalRepository{db: repos.DB}
}

// DB 返回底层 *gorm.DB（用于按主键查询等）
func (r *DocumentApprovalRepository) DB() *gorm.DB {
	return r.db
}

// FindByID 按主键查询
func (r *DocumentApprovalRepository) FindByID(id int) (*workspacemodel.DocumentApproval, error) {
	var a workspacemodel.DocumentApproval
	if err := r.db.First(&a, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApprovalNotFound
		}
		return nil, err
	}
	return &a, nil
}

// Create 创建审批记录
func (r *DocumentApprovalRepository) Create(a *workspacemodel.DocumentApproval) error {
	now := int(time.Now().Unix())
	a.SubmitTime = now
	a.HandleTime = 0
	return r.db.Create(a).Error
}

// FindByDocument 获取文档的最新一条审批记录
func (r *DocumentApprovalRepository) FindByDocument(docID int) (*workspacemodel.DocumentApproval, error) {
	var a workspacemodel.DocumentApproval
	err := r.db.Where("document_id = ?", docID).Order("id DESC").First(&a).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApprovalNotFound
		}
		return nil, err
	}
	return &a, nil
}

// FindByApprover 获取待某用户审批的记录（分页）
func (r *DocumentApprovalRepository) FindByApprover(approver, offset, limit int) ([]workspacemodel.DocumentApproval, int64, error) {
	var total int64
	r.db.Model(&workspacemodel.DocumentApproval{}).Where("approver = ? AND status = ?", approver, workspacemodel.ApprovalStatusPending).Count(&total)
	var list []workspacemodel.DocumentApproval
	err := r.db.Where("approver = ? AND status = ?", approver, workspacemodel.ApprovalStatusPending).
		Order("submit_time ASC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

// FindBySubmitter 获取某用户提交的审批记录（分页）
func (r *DocumentApprovalRepository) FindBySubmitter(submitter, offset, limit int) ([]workspacemodel.DocumentApproval, int64, error) {
	var total int64
	r.db.Model(&workspacemodel.DocumentApproval{}).Where("submitter = ?", submitter).Count(&total)
	var list []workspacemodel.DocumentApproval
	err := r.db.Where("submitter = ?", submitter).
		Order("submit_time DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}

// UpdateStatus 更新审批状态与意见
// P3：加 status='pending' 条件，避免并发重复处理同一审批
func (r *DocumentApprovalRepository) UpdateStatus(id, approver int, status, comment string) error {
	now := int(time.Now().Unix())
	result := r.db.Model(&workspacemodel.DocumentApproval{}).
		Where("id = ? AND approver = ? AND status = ?", id, approver, workspacemodel.ApprovalStatusPending).
		Updates(map[string]interface{}{
			"status":      status,
			"comment":     comment,
			"handle_time": now,
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("approval %d not found, not pending, or approver mismatch", id)
	}
	return nil
}
