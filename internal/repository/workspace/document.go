package workspace

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	adminmodel "github.com/listen808/citebase/internal/model/admin"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrDocumentNotFound = errors.New("document not found")
)

// DocumentRepository 文档数据访问层
type DocumentRepository struct {
	db *gorm.DB
}

// NewDocumentRepository 创建文档仓储实例
func NewDocumentRepository() *DocumentRepository {
	return &DocumentRepository{db: repos.DB}
}

// NewDocumentRepositoryWithDB 使用指定 repos.DB 创建文档仓储实例（用于事务）
func NewDocumentRepositoryWithDB(db *gorm.DB) *DocumentRepository {
	return &DocumentRepository{db: db}
}

// Transaction 在事务中执行回调函数
func (r *DocumentRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// Create 创建文档
func (r *DocumentRepository) Create(doc *workspacemodel.Document) error {
	return r.db.Create(doc).Error
}

// FindByID 根据 ID 查找文档（排除已删除的）
func (r *DocumentRepository) FindByID(id int) (*workspacemodel.Document, error) {
	var doc workspacemodel.Document
	err := r.db.Where("document_id = ?", id).Scopes(repos.NotDeleted).First(&doc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrDocumentNotFound
		}
		return nil, err
	}
	return &doc, nil
}

// Update 更新文档
func (r *DocumentRepository) Update(doc *workspacemodel.Document) error {
	return r.applyUpdate(r.db, doc)
}

// applyUpdate 是 Update 的实现体，抽出来是为了能在事务句柄上跑同一条更新
// （见 UpdateWithVersion），而不是把这份 Select 列表抄第二遍。
func (r *DocumentRepository) applyUpdate(tx *gorm.DB, doc *workspacemodel.Document) error {
	// update_time 统一在此维护：Document 的 BeforeUpdate 钩子在
	// Model(&X{}).Select(...).Updates(dest) 组合下不生效，依赖钩子会导致
	// update_time 永不落库（全部调用方经由此方法，单点收敛）
	doc.UpdateTime = int(time.Now().Unix())
	return tx.Model(&workspacemodel.Document{}).
		Where("document_id = ?", doc.ID).
		Select("name", "sequence", "edit_user_id", "update_time", "parent_id", "space_id", "path").
		Updates(doc).Error
}

// UpdateWithVersion 文档行与版本记录在同一条事务里写。
//
// 加这条的原因：保存正文时原先是"先 CreateVersion、再 Update"，且版本写失败只记日志 ——
// 于是用户在版本框里写了说明的那一次修改，历史里根本没有那一条，
// 而"回滚到版本 N"后来恢复的是别的版本；反过来文档写失败、版本却留下了，
// 历史里就多出一条正文和文档对不上的记录。两条都只有并成一条事务才成立。
func (r *DocumentRepository) UpdateWithVersion(doc *workspacemodel.Document, version *workspacemodel.DocumentVersion) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if version != nil {
			if err := tx.Create(version).Error; err != nil {
				return err
			}
		}
		return r.applyUpdate(tx, doc)
	})
}

// TombstoneName 生成软删除墓碑名：原名称截断后追加 "__del_<id>"，总长（rune）不超过 150。
func TombstoneName(name string, id int) string {
	suffix := fmt.Sprintf("__del_%d", id)
	maxBase := 150 - len([]rune(suffix))
	base := []rune(name)
	if len(base) > maxBase {
		base = base[:maxBase]
	}
	return string(base) + suffix
}

// Delete 软删除文档（设置 is_delete=1）。
// 唯一索引 uk_space_parent_name(space_id,parent_id,name) 对已删除行仍然生效：
// 保留原名会让「删除后同父新建同名文档」撞 1062（应用层防重只看 is_delete=0），
// 「从回收站恢复」也会与新建的同名行互撞。软删时改写为墓碑名让出唯一键，
// 名称原文由 DocumentTrash 行保存，恢复时写回。
func (r *DocumentRepository) Delete(id int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		var doc workspacemodel.Document
		if err := tx.Select("name").Where("document_id = ?", id).First(&doc).Error; err != nil {
			return err
		}
		return tx.Model(&workspacemodel.Document{}).Where("document_id = ?", id).
			Updates(map[string]interface{}{"is_delete": 1, "name": TombstoneName(doc.Name, id)}).Error
	})
}

// HardDelete 物理删除文档（用于软删除失败后的兜底）
func (r *DocumentRepository) HardDelete(id int) error {
	return r.db.Unscoped().Where("document_id = ?", id).Delete(&workspacemodel.Document{}).Error
}

// #7 草稿/发布分离

// SetStatus 设置文档状态（published / draft）
func (r *DocumentRepository) SetStatus(id int, status string, publisherID int) error {
	updates := map[string]interface{}{
		"status":       status,
		"edit_user_id": publisherID,
	}
	if status == workspacemodel.DocumentStatusPublished {
		updates["published_at"] = int(time.Now().Unix())
	} else {
		// 草稿态必须清零：多处（含 /documents/view/*）以 published_at>0 推导「已发布」，
		// 只改 status 会让下线文档在列表/看板/画廊里永久显示为已发布。
		updates["published_at"] = 0
	}
	return r.db.Model(&workspacemodel.Document{}).
		Where("document_id = ?", id).
		Updates(updates).Error
}

// FindDraftsByAuthor 获取某用户创建的草稿文档
func (r *DocumentRepository) FindDraftsByAuthor(userID int) ([]*workspacemodel.Document, error) {
	var docs []*workspacemodel.Document
	err := r.db.Where("create_user_id = ? AND status = ?", userID, workspacemodel.DocumentStatusDraft).
		Scopes(repos.NotDeleted).
		Order("update_time DESC").
		Find(&docs).Error
	return docs, err
}

// FindBySpaceID 获取空间下的所有文档
func (r *DocumentRepository) FindBySpaceID(spaceID int) ([]*workspacemodel.Document, error) {
	var docs []*workspacemodel.Document
	err := r.db.Where("space_id = ?", spaceID).Scopes(repos.NotDeleted).
		Order("sequence ASC, document_id ASC").
		Find(&docs).Error
	return docs, err
}

// FindByParentID 获取子文档
func (r *DocumentRepository) FindByParentID(parentID int) ([]*workspacemodel.Document, error) {
	var docs []*workspacemodel.Document
	err := r.db.Where("parent_id = ?", parentID).Scopes(repos.NotDeleted).
		Order("sequence ASC, document_id ASC").
		Find(&docs).Error
	return docs, err
}

// FindRootDocuments 获取空间根文档（顶级目录）
func (r *DocumentRepository) FindRootDocuments(spaceID int) ([]*workspacemodel.Document, error) {
	var docs []*workspacemodel.Document
	err := r.db.Where("space_id = ? AND parent_id = 0", spaceID).Scopes(repos.NotDeleted).
		Order("sequence ASC, document_id ASC").
		Find(&docs).Error
	return docs, err
}

// HasChildren 判断文档是否有子节点
func (r *DocumentRepository) HasChildren(parentID int) bool {
	var count int64
	if err := r.db.Model(&workspacemodel.Document{}).Where("parent_id = ?", parentID).Scopes(repos.NotDeleted).Count(&count).Error; err != nil {
		return true
	}
	return count > 0
}

type ChildCountResult struct {
	ParentID int
	Count    int64
}

// CountChildrenByParents 批量查询多个父文档的子节点数量（消除N+1）
func (r *DocumentRepository) CountChildrenByParents(parentIDs []int) ([]ChildCountResult, error) {
	if len(parentIDs) == 0 {
		return nil, nil
	}
	var results []ChildCountResult
	err := r.db.Model(&workspacemodel.Document{}).
		Select("parent_id, COUNT(*) as count").
		Where("parent_id IN ?", parentIDs).Scopes(repos.NotDeleted).
		Group("parent_id").
		Scan(&results).Error
	return results, err
}

// FindByNameAndSpaceID 根据名称和空间查找文档（排除已删除的）
func (r *DocumentRepository) FindByNameAndSpaceID(name string, spaceID int) (*workspacemodel.Document, error) {
	var doc workspacemodel.Document
	err := r.db.Where("name = ? AND space_id = ?", name, spaceID).Scopes(repos.NotDeleted).First(&doc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	// 双重确认：确保返回的文档未被删除
	if doc.IsDelete == 1 {
		return nil, nil
	}
	return &doc, nil
}

// FindByNameAndParentID 根据名称和父文档ID查找文档（排除已删除的）
func (r *DocumentRepository) FindByNameAndParentID(name string, parentID int, spaceID int) (*workspacemodel.Document, error) {
	var doc workspacemodel.Document
	err := r.db.Where("name = ? AND parent_id = ? AND space_id = ?", name, parentID, spaceID).Scopes(repos.NotDeleted).First(&doc).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	if doc.IsDelete == 1 {
		return nil, nil
	}
	return &doc, nil
}

func (r *DocumentRepository) MaxSequenceByParent(parentID int, spaceID int) (int, error) {
	var maxSeq int
	err := r.db.Model(&workspacemodel.Document{}).
		Where("parent_id = ? AND space_id = ?", parentID, spaceID).Scopes(repos.NotDeleted).
		Select("COALESCE(MAX(sequence), 0)").
		Scan(&maxSeq).Error
	return maxSeq, err
}

// BuildTree 构建文档树（单次查询+内存构建，消除N+1）
func (r *DocumentRepository) BuildTree(spaceID int) ([]*workspacemodel.Document, error) {
	allDocs, err := r.FindBySpaceID(spaceID)
	if err != nil {
		return nil, err
	}
	return allDocs, nil
}

// UpdatePath 更新文档路径
func (r *DocumentRepository) UpdatePath(id int, path string) error {
	return r.db.Model(&workspacemodel.Document{}).Where("document_id = ?", id).Update("path", path).Error
}

// UpdateSequence 更新文档顺序
func (r *DocumentRepository) UpdateSequence(id int, sequence int) error {
	return r.db.Model(&workspacemodel.Document{}).Where("document_id = ?", id).Update("sequence", sequence).Error
}

// MarkImmutable 标记文档为 raw 层不可变（wiki 源文档），内容变更将触发 wiki 重编译。
func (r *DocumentRepository) MarkImmutable(docID int, immutable bool) error {
	val := 0
	if immutable {
		val = 1
	}
	return r.db.Model(&workspacemodel.Document{}).Where("document_id = ?", docID).Update("is_immutable", val).Error
}

// CreateVersion 创建文档版本
func (r *DocumentRepository) CreateVersion(version *workspacemodel.DocumentVersion) error {
	return r.db.Create(version).Error
}

// GetVersions 获取文档所有版本
func (r *DocumentRepository) GetVersions(documentID int) ([]*workspacemodel.DocumentVersion, error) {
	var versions []*workspacemodel.DocumentVersion
	err := r.db.Where("document_id = ?", documentID).
		Order("version DESC").
		Find(&versions).Error
	return versions, err
}

// GetVersionsPaginated 获取文档版本（分页）
func (r *DocumentRepository) GetVersionsPaginated(documentID int, page int, pageSize int) ([]*workspacemodel.DocumentVersion, int64, error) {
	var versions []*workspacemodel.DocumentVersion
	var total int64
	db := r.db.Model(&workspacemodel.DocumentVersion{}).Where("document_id = ?", documentID)
	db.Count(&total)
	offset := (page - 1) * pageSize
	err := db.Order("version DESC").
		Offset(offset).
		Limit(pageSize).
		Find(&versions).Error
	return versions, total, err
}

// GetVersionByID 获取指定版本（联合 document_id 过滤，防止越权访问其他文档版本）
func (r *DocumentRepository) GetVersionByID(documentID int, versionID int) (*workspacemodel.DocumentVersion, error) {
	var version workspacemodel.DocumentVersion
	err := r.db.Where("id = ? AND document_id = ?", versionID, documentID).First(&version).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, errors.New("version not found")
		}
		return nil, err
	}
	return &version, nil
}

// GetLatestVersion 获取最新版本的版本号
// P9 修复：合并 Count + First 为单次查询，少一次 DB 往返
func (r *DocumentRepository) GetLatestVersion(documentID int) (int, error) {
	var version workspacemodel.DocumentVersion
	err := r.db.Where("document_id = ?", documentID).
		Order("version DESC").
		First(&version).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return 0, nil
		}
		return 0, err
	}
	return version.Version, nil
}

// RollbackToVersion 回滚到指定版本（通过版本号获取内容）
func (r *DocumentRepository) GetVersionContent(documentID int, version int) (string, error) {
	var docVersion workspacemodel.DocumentVersion
	err := r.db.Where("document_id = ? AND version = ?", documentID, version).
		First(&docVersion).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return "", errors.New("version content not found")
		}
		return "", err
	}
	return docVersion.Content, nil
}

// SearchDocuments 已于 P2-1 移除（原为 title LIKE 小搜索，与 Bleve /search/fulltext 重复）

// CountBySpaceID 统计空间文档数量
func (r *DocumentRepository) CountBySpaceID(spaceID int) (int64, error) {
	var count int64
	err := r.db.Model(&workspacemodel.Document{}).Where("space_id = ?", spaceID).Scopes(repos.NotDeleted).Count(&count).Error
	return count, err
}

// CountBySpaceIDsGrouped 一条 GROUP BY 查出多个空间各自的文档数。
// 加这条是因为「知识库列表」原先在 for 里逐空间 Count —— N 个空间 N 次查询，
// 而那一列数字还是可能悄悄查询失败并显示成 0 的那一列。
func (r *DocumentRepository) CountBySpaceIDsGrouped(spaceIDs []int) (map[int]int64, error) {
	counts := make(map[int]int64, len(spaceIDs))
	if len(spaceIDs) == 0 {
		return counts, nil
	}
	var rows []struct {
		SpaceID int
		N       int64
	}
	err := r.db.Model(&workspacemodel.Document{}).
		Select("space_id, COUNT(*) AS n").
		Where("space_id IN ? AND is_delete = 0", spaceIDs).
		Group("space_id").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		counts[row.SpaceID] = row.N
	}
	return counts, nil
}

// CountBySpaceAndType 按文档类型（页面/目录）统计某空间的数量
func (r *DocumentRepository) CountBySpaceAndType(spaceID, docType int) (int64, error) {
	var count int64
	err := r.db.Model(&workspacemodel.Document{}).
		Where("space_id = ? AND is_delete = 0 AND type = ?", spaceID, docType).
		Count(&count).Error
	return count, err
}

// FindAllDocuments 获取所有文档（用于全量索引重建）
func (r *DocumentRepository) FindAllDocuments() ([]*workspacemodel.Document, error) {
	var docs []*workspacemodel.Document
	err := r.db.Scopes(repos.NotDeleted).Where("type = ?", workspacemodel.DocumentTypePage).
		Order("document_id ASC").
		Find(&docs).Error
	return docs, err
}

// FindAllDocumentsIncludeDirs 获取所有文档（含目录，用于索引重建）
func (r *DocumentRepository) FindAllDocumentsIncludeDirs() ([]*workspacemodel.Document, error) {
	var docs []*workspacemodel.Document
	err := r.db.Scopes(repos.NotDeleted).
		Order("document_id ASC").
		Find(&docs).Error
	return docs, err
}

// CountAll 统计所有文档数
func (r *DocumentRepository) CountAll() (int64, error) {
	var count int64
	err := r.db.Model(&workspacemodel.Document{}).Scopes(repos.NotDeleted).Count(&count).Error
	return count, err
}

// CountTodayDocuments 统计今日新增文档数
func (r *DocumentRepository) CountTodayDocuments() (int64, error) {
	today := time.Date(time.Now().Year(), time.Now().Month(), time.Now().Day(), 0, 0, 0, 0, time.Local)
	timestamp := today.Unix()
	var count int64
	err := r.db.Model(&workspacemodel.Document{}).Scopes(repos.NotDeleted).
		Where("create_time >= ?", timestamp).
		Count(&count).Error
	return count, err
}

// GetDocCountByTime 获取最近 N 天每日文档活跃数（新增+编辑+浏览）
func (r *DocumentRepository) GetDocCountByTime(days int) ([]adminmodel.DocCountItem, error) {
	now := time.Now()
	startTime := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location()).AddDate(0, 0, -(days - 1)).Unix()
	endTime := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location()).Unix()

	type simpleDoc struct {
		CreateTime int
		UpdateTime int
	}
	var docs []simpleDoc
	if err := r.db.Table("aw_document").
		Select("create_time, update_time").Scopes(repos.NotDeleted).
		Where("(create_time >= ? AND create_time <= ? OR update_time >= ? AND update_time <= ?)", startTime, endTime, startTime, endTime).
		Find(&docs).Error; err != nil {
		return nil, err
	}

	countMap := make(map[string]int64)
	for i := 0; i < days; i++ {
		d := now.AddDate(0, 0, -i)
		countMap[d.Format("2006-01-02")] = 0
	}

	for _, doc := range docs {
		if doc.CreateTime > 0 {
			t := time.Unix(int64(doc.CreateTime), 0)
			key := t.Format("2006-01-02")
			if _, ok := countMap[key]; ok {
				countMap[key]++
			}
		}
		if doc.UpdateTime > 0 && doc.UpdateTime != doc.CreateTime {
			t := time.Unix(int64(doc.UpdateTime), 0)
			key := t.Format("2006-01-02")
			if _, ok := countMap[key]; ok {
				countMap[key]++
			}
		}
	}

	type simpleLog struct {
		CreateTime int
	}
	var logs []simpleLog
	if err := r.db.Table("aw_log_document").
		Select("create_time").
		Where("create_time >= ? AND create_time <= ?", startTime, endTime).
		Find(&logs).Error; err != nil {
		return nil, err
	}

	for _, l := range logs {
		if l.CreateTime > 0 {
			t := time.Unix(int64(l.CreateTime), 0)
			key := t.Format("2006-01-02")
			if _, ok := countMap[key]; ok {
				countMap[key]++
			}
		}
	}

	items := make([]adminmodel.DocCountItem, 0, days)
	for i := days - 1; i >= 0; i-- {
		d := now.AddDate(0, 0, -i)
		dateStr := d.Format("2006-01-02")
		items = append(items, adminmodel.DocCountItem{
			Date:  dateStr,
			Count: countMap[dateStr],
		})
	}
	return items, nil
}

// FindDescendants 获取文档的所有后代文档（递归）
func (r *DocumentRepository) FindDescendants(parentID int) ([]*workspacemodel.Document, error) {
	var allDescendants []*workspacemodel.Document
	queue := []int{parentID}
	for len(queue) > 0 {
		currentID := queue[0]
		queue = queue[1:]
		children, err := r.FindByParentID(currentID)
		if err != nil {
			return nil, err
		}
		allDescendants = append(allDescendants, children...)
		for _, child := range children {
			queue = append(queue, child.ID)
		}
	}
	return allDescendants, nil
}

// UpdatePathBatch 批量更新文档路径
func (r *DocumentRepository) UpdatePathBatch(updates []DocumentPathUpdate) error {
	if len(updates) == 0 {
		return nil
	}
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, u := range updates {
			if err := tx.Model(&workspacemodel.Document{}).Where("document_id = ?", u.ID).Update("path", u.Path).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DocumentPathUpdate 路径更新项
type DocumentPathUpdate struct {
	ID   int
	Path string
}

// FindByIDs 根据 ID 列表批量查询文档（排除已删除的）
func (r *DocumentRepository) FindByIDs(ids []int) ([]*workspacemodel.Document, error) {
	if len(ids) == 0 {
		return []*workspacemodel.Document{}, nil
	}
	var docs []*workspacemodel.Document
	err := r.db.Where("document_id IN ?", ids).Scopes(repos.NotDeleted).Find(&docs).Error
	return docs, err
}

// FindByCreateUserID 按创建者（作者）查找文档
func (r *DocumentRepository) FindByCreateUserID(userID int) ([]*workspacemodel.Document, error) {
	if userID <= 0 {
		return []*workspacemodel.Document{}, nil
	}
	var docs []*workspacemodel.Document
	err := r.db.Where("create_user_id = ?", userID).Scopes(repos.NotDeleted).Find(&docs).Error
	return docs, err
}

// GetHotDocuments 获取热门文档 TOP N（按文档日志操作次数统计）
func (r *DocumentRepository) GetHotDocuments(limit int) ([]adminmodel.HotDocument, error) {
	var results []adminmodel.HotDocument
	err := r.db.Table("aw_log_document").
		Select("aw_log_document.document_id, aw_document.name, aw_space.name as space_name, COUNT(*) as count").
		Joins("LEFT JOIN aw_document ON aw_log_document.document_id = aw_document.document_id AND aw_document.is_delete = 0").
		Joins("LEFT JOIN aw_space ON aw_document.space_id = aw_space.space_id AND aw_space.is_delete = 0").
		Where("aw_document.is_delete = 0").
		Group("aw_log_document.document_id").
		Order("count DESC").
		Limit(limit).
		Scan(&results).Error
	if err != nil {
		return nil, err
	}
	return results, nil
}

// DeleteDocumentRecursive 使用事务递归软删除文档及其子文档
func (r *DocumentRepository) DeleteDocumentRecursive(tx *gorm.DB, id int) error {
	// 查找所有子文档
	var children []*workspacemodel.Document
	if err := tx.Where("parent_id = ?", id).Scopes(repos.NotDeleted).Order("document_id ASC").Find(&children).Error; err != nil {
		return err
	}
	for _, child := range children {
		if err := r.DeleteDocumentRecursive(tx, child.ID); err != nil {
			return err
		}
	}
	// 软删除当前文档
	now := time.Now().Unix()
	return tx.Model(&workspacemodel.Document{}).Where("document_id = ?", id).Updates(map[string]interface{}{
		"is_delete":   1,
		"update_time": now,
	}).Error
}

// FindBySpaceIDs 根据空间 ID 列表查询文档，spaceIDs 为空时返回所有文档
func (r *DocumentRepository) FindBySpaceIDs(spaceIDs []int) ([]*workspacemodel.Document, error) {
	var docs []*workspacemodel.Document
	query := r.db.Model(&workspacemodel.Document{}).Scopes(repos.NotDeleted)
	if len(spaceIDs) > 0 {
		query = query.Where("space_id IN ?", spaceIDs)
	}
	if err := query.Find(&docs).Error; err != nil {
		return nil, err
	}
	return docs, nil
}
