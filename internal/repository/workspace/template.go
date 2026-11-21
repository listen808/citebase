package workspace

import (
	"errors"
	"time"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

// ErrTemplateNotFound 模板不存在
var ErrTemplateNotFound = errors.New("template not found")

// DocumentTemplateRepository 文档模板数据访问层
type DocumentTemplateRepository struct {
	db *gorm.DB
}

// NewDocumentTemplateRepository 创建仓储实例
func NewDocumentTemplateRepository() *DocumentTemplateRepository {
	return &DocumentTemplateRepository{db: repos.DB}
}

// Create 创建模板
func (r *DocumentTemplateRepository) Create(t *workspacemodel.DocumentTemplate) error {
	now := int(time.Now().Unix())
	t.CreateTime = now
	t.UpdateTime = now
	return r.db.Create(t).Error
}

// Update 更新模板
func (r *DocumentTemplateRepository) Update(id int, req *workspacemodel.UpdateTemplateRequest) error {
	updates := map[string]interface{}{"update_time": int(time.Now().Unix())}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Content != "" {
		updates["content"] = req.Content
	}
	return r.db.Model(&workspacemodel.DocumentTemplate{}).Where("id = ?", id).Updates(updates).Error
}

// Delete 删除模板
func (r *DocumentTemplateRepository) Delete(id int) error {
	return r.db.Delete(&workspacemodel.DocumentTemplate{}, "id = ?", id).Error
}

// FindByID 按主键查询
func (r *DocumentTemplateRepository) FindByID(id int) (*workspacemodel.DocumentTemplate, error) {
	var t workspacemodel.DocumentTemplate
	if err := r.db.First(&t, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrTemplateNotFound
		}
		return nil, err
	}
	return &t, nil
}

// ListBySpace 列出某空间及全局模板（分页）
func (r *DocumentTemplateRepository) ListBySpace(spaceID, offset, limit int) ([]workspacemodel.DocumentTemplate, int64, error) {
	var total int64
	r.db.Model(&workspacemodel.DocumentTemplate{}).
		Where("space_id = ? OR space_id = 0", spaceID).Count(&total)
	var list []workspacemodel.DocumentTemplate
	err := r.db.Where("space_id = ? OR space_id = 0", spaceID).
		Order("create_time DESC").Offset(offset).Limit(limit).Find(&list).Error
	return list, total, err
}
