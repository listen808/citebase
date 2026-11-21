package workspace

import (
	"time"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

// DocumentPermissionRepository 文档级 ACL 仓库
type DocumentPermissionRepository struct {
	db *gorm.DB
}

func NewDocumentPermissionRepository() *DocumentPermissionRepository {
	return &DocumentPermissionRepository{db: repos.DB}
}

// FilterDenied 返回 docIDs 中被显式 deny 的子集（一次 IN 查询）。
//
// 语义：没有 ACL 记录 = 沿用空间成员权限（允许）。只有显式 deny 才拦，
// 这样老数据零迁移成本，也避免"配了 ACL 反而把所有文档都藏了"的事故。
// deny 优先于 allow：只要命中一条 deny 就拒绝。
func (r *DocumentPermissionRepository) FilterDenied(userID int, roleIDs []int, docIDs []int) (map[int]bool, error) {
	denied := make(map[int]bool)
	if len(docIDs) == 0 || userID <= 0 {
		return denied, nil
	}

	q := r.db.Model(&workspacemodel.DocumentPermission{}).
		Where("doc_id IN ?", docIDs).
		Where("effect = ?", workspacemodel.ACLEffectDeny).
		Where("(subject_type = ? AND subject_id = ?)", workspacemodel.ACLSubjectUser, userID)
	if len(roleIDs) > 0 {
		q = q.Or("doc_id IN ? AND effect = ? AND subject_type = ? AND subject_id IN ?",
			docIDs, workspacemodel.ACLEffectDeny, workspacemodel.ACLSubjectRole, roleIDs)
	}

	var rows []struct{ DocID int }
	if err := q.Distinct().Select("doc_id").Scan(&rows).Error; err != nil {
		return nil, err
	}
	for _, row := range rows {
		denied[row.DocID] = true
	}
	return denied, nil
}

// Upsert 写入或更新一条 ACL 记录
func (r *DocumentPermissionRepository) Upsert(p *workspacemodel.DocumentPermission) error {
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	p.UpdatedAt = time.Now()
	return r.db.Where("doc_id = ? AND subject_type = ? AND subject_id = ?",
		p.DocID, p.SubjectType, p.SubjectID).
		Assign(workspacemodel.DocumentPermission{
			SpaceID:   p.SpaceID,
			Effect:    p.Effect,
			CreatedBy: p.CreatedBy,
			UpdatedAt: p.UpdatedAt,
		}).FirstOrCreate(p).Error
}

// Delete 删除一条 ACL 记录
func (r *DocumentPermissionRepository) Delete(docID int, subjectType string, subjectID int) error {
	return r.db.Where("doc_id = ? AND subject_type = ? AND subject_id = ?",
		docID, subjectType, subjectID).Delete(&workspacemodel.DocumentPermission{}).Error
}

// ListByDoc 列出某文档的全部 ACL 记录
func (r *DocumentPermissionRepository) ListByDoc(docID int) ([]*workspacemodel.DocumentPermission, error) {
	var out []*workspacemodel.DocumentPermission
	if err := r.db.Where("doc_id = ?", docID).
		Order("subject_type, subject_id").Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}
