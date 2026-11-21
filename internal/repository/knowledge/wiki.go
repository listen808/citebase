package knowledge

import (
	"errors"
	"time"

	"gorm.io/gorm"

	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	repos "github.com/listen808/citebase/internal/repository"
)

// ErrStaleDraft 草稿基线版本与页面当前版本不一致。
// 两个并发草稿可能都基于 V1 生成，先批准的那个把页面推到 V2；此时直接批准第二个
// 会静默覆盖第一个刚加入的知识。因此发布必须走版本号比对（乐观锁）。
var ErrStaleDraft = errors.New("wiki page has been updated, draft must be rebased")

type WikiPageRepository struct {
	db *gorm.DB
}

func NewWikiPageRepository() *WikiPageRepository {
	return &WikiPageRepository{db: repos.DB}
}

func (r *WikiPageRepository) FindBySlug(spaceID int, slug string) (*knowledgemodel.WikiPage, error) {
	var p knowledgemodel.WikiPage
	err := r.db.Where("space_id = ? AND slug = ?", spaceID, slug).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (r *WikiPageRepository) FindByID(id int) (*knowledgemodel.WikiPage, error) {
	var p knowledgemodel.WikiPage
	err := r.db.Where("id = ?", id).First(&p).Error
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// ListBySpace 列出空间下指定状态的页面；status 为空表示不限状态。
// 固定按 slug 排序：目录构建必须确定性，否则 Prompt Caching 的前缀会失效。
func (r *WikiPageRepository) ListBySpace(spaceID int, status string) ([]*knowledgemodel.WikiPage, error) {
	var pages []*knowledgemodel.WikiPage
	q := r.db.Where("space_id = ?", spaceID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Order("slug ASC").Find(&pages).Error
	return pages, err
}

func (r *WikiPageRepository) CountBySpace(spaceID int, status string) (int64, error) {
	var n int64
	q := r.db.Model(&knowledgemodel.WikiPage{}).Where("space_id = ?", spaceID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	err := q.Count(&n).Error
	return n, err
}

// PlanPlaceholder 串行写入 planned 占位页。
// 这一步必须在"决定知识落到哪一页"的临界区内完成且立即落库，
// 后续并发的 Plan 才能看到前序任务已经做出的划分。
func (r *WikiPageRepository) PlanPlaceholder(spaceID int, slug, pageType, title, summary string, sourceDocID int) (*knowledgemodel.WikiPage, error) {
	existing, err := r.FindBySlug(spaceID, slug)
	if err == nil {
		// 已有同 slug 页面：把该源并入其溯源列表，不创建新页
		if sourceDocID > 0 && !containsInt(existing.Sources, sourceDocID) {
			existing.Sources = append(existing.Sources, sourceDocID)
			existing.SourceCount = len(existing.Sources)
			if err := r.db.Save(existing).Error; err != nil {
				return nil, err
			}
		}
		return existing, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}

	sources := knowledgemodel.IntArray{}
	if sourceDocID > 0 {
		sources = append(sources, sourceDocID)
	}
	p := &knowledgemodel.WikiPage{
		SpaceID:     spaceID,
		Slug:        slug,
		Type:        pageType,
		Title:       title,
		Summary:     summary,
		Sources:     sources,
		Status:      string(knowledgemodel.WikiPageStatusPlanned),
		SourceCount: len(sources),
		Provenance:  knowledgemodel.WikiProvenanceLLM,
	}
	if err := r.db.Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

// SaveDraft 写入草稿内容，记录基线版本与置信度。
func (r *WikiPageRepository) SaveDraft(spaceID int, slug, content, summary, contentHash string, sources []int, confidence float64) error {
	if confidence <= 0 {
		confidence = 0.50
	}
	now := time.Now()
	return r.db.Model(&knowledgemodel.WikiPage{}).
		Where("space_id = ? AND slug = ?", spaceID, slug).
		Updates(map[string]interface{}{
			"content":      content,
			"summary":      summary,
			"content_hash": contentHash,
			"sources":      knowledgemodel.IntArray(sources),
			"source_count": len(sources),
			"status":       string(knowledgemodel.WikiPageStatusDraft),
			"compiled_at":  now,
			"confidence":   confidence,
		}).Error
}

// UpsertSystemPage 系统自动维护页（overview / log）的直写通道。
// 这类页面是纯派生物、随时可从目录/日志重建，不进 draft→审核流，
// 直接写内容并置为 published，版本号 +1 保留变更轨迹。
func (r *WikiPageRepository) UpsertSystemPage(spaceID int, slug, pageType, title, summary, content, contentHash string) (*knowledgemodel.WikiPage, error) {
	now := time.Now()
	existing, err := r.FindBySlug(spaceID, slug)
	if err == nil {
		updates := map[string]interface{}{
			"content":      content,
			"summary":      summary,
			"content_hash": contentHash,
			"status":       string(knowledgemodel.WikiPageStatusPublished),
			"compiled_at":  now,
			"title":        title,
		}
		// 人工编辑过的 overview 不再被自动覆盖（与 Pin 治理同一条原则）
		if existing.Provenance != knowledgemodel.WikiProvenanceHuman {
			updates["version"] = gorm.Expr("version + 1")
		}
		if err := r.db.Model(&knowledgemodel.WikiPage{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
			return nil, err
		}
		return r.FindBySlug(spaceID, slug)
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, err
	}
	p := &knowledgemodel.WikiPage{
		SpaceID:     spaceID,
		Slug:        slug,
		Type:        pageType,
		Title:       title,
		Summary:     summary,
		Content:     content,
		Status:      string(knowledgemodel.WikiPageStatusPublished),
		Version:     1,
		BaseVersion: 1,
		Provenance:  knowledgemodel.WikiProvenanceLLM,
		ContentHash: contentHash,
		CompiledAt:  &now,
	}
	if err := r.db.Create(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

// Publish 发布草稿：比对基线版本后版本号 +1。
// 只有基于当前版本的写入能成功，另一个草稿会显式进入冲突处理而不是覆盖已有内容。
// 已发布页面被替换前，旧版本内容落入版本历史（回滚的基线来源）。
func (r *WikiPageRepository) Publish(spaceID int, slug string, baseVersion int) (*knowledgemodel.WikiPage, error) {
	p, err := r.FindBySlug(spaceID, slug)
	if err != nil {
		return nil, err
	}
	if p.BaseVersion != baseVersion {
		return nil, ErrStaleDraft
	}
	if p.Version >= 2 && p.Status == string(knowledgemodel.WikiPageStatusPublished) {
		if err := r.snapshotPublished(p); err != nil {
			return nil, err
		}
	}
	now := time.Now()
	p.Status = string(knowledgemodel.WikiPageStatusPublished)
	p.Version = p.Version + 1
	p.BaseVersion = p.Version
	p.CompiledAt = &now
	if err := r.db.Save(p).Error; err != nil {
		return nil, err
	}
	return p, nil
}

// snapshotPublished 把当前已发布版本的内容存入历史表。
// 快照失败会中止发布（宁可发布失败，不可丢版本）。
func (r *WikiPageRepository) snapshotPublished(p *knowledgemodel.WikiPage) error {
	h := &knowledgemodel.WikiPageHistory{
		SpaceID:     p.SpaceID,
		Slug:        p.Slug,
		Version:     p.Version,
		Title:       p.Title,
		Summary:     p.Summary,
		Content:     p.Content,
		Sources:     p.Sources,
		Provenance:  p.Provenance,
		ContentHash: p.ContentHash,
	}
	return r.db.Create(h).Error
}

// MarkHumanEdited 人工修改后把 provenance 切到 human，
// 源文档下架触发的重编译不得自动删除人工新增的知识。
func (r *WikiPageRepository) MarkHumanEdited(spaceID int, slug string) error {
	return r.db.Model(&knowledgemodel.WikiPage{}).
		Where("space_id = ? AND slug = ?", spaceID, slug).
		Update("provenance", knowledgemodel.WikiProvenanceHuman).Error
}

func (r *WikiPageRepository) UpdateStatus(id int, status string) error {
	return r.db.Model(&knowledgemodel.WikiPage{}).Where("id = ?", id).Update("status", status).Error
}

func (r *WikiPageRepository) Delete(id int) error {
	return r.db.Where("id = ?", id).Delete(&knowledgemodel.WikiPage{}).Error
}

// RemoveSource 从页面的溯源列表中摘除一个源文档。
// 共享实体页（多个源贡献）只摘除该源，不删除整页 —— 级联删除的核心规则。
func (r *WikiPageRepository) RemoveSource(id int, sourceDocID int) error {
	p, err := r.FindByID(id)
	if err != nil {
		return err
	}
	p.Sources = removeInt(p.Sources, sourceDocID)
	p.SourceCount = len(p.Sources)
	if len(p.Sources) == 0 && p.Provenance == knowledgemodel.WikiProvenanceLLM {
		return r.Delete(id)
	}
	return r.db.Save(p).Error
}

// ListAllForSpace 级联删除/死链清理用：返回空间下全部页面（含 planned/draft）。
func (r *WikiPageRepository) ListAllForSpace(spaceID int) ([]*knowledgemodel.WikiPage, error) {
	var pages []*knowledgemodel.WikiPage
	err := r.db.Where("space_id = ?", spaceID).Order("slug ASC").Find(&pages).Error
	return pages, err
}

// Rollback 回滚到指定历史版本：历史内容写入草稿区，走 Publish 发布。
// 回滚本身不产生新版本 —— 版本语义保持"一次内容提交 = 一个版本"。
// 返回 (页面, 目标版本号, err)。
func (r *WikiPageRepository) Rollback(spaceID int, slug string, toVersion int) (*knowledgemodel.WikiPage, int, error) {
	var h knowledgemodel.WikiPageHistory
	err := r.db.Where("space_id = ? AND slug = ? AND version = ?", spaceID, slug, toVersion).First(&h).Error
	if err != nil {
		return nil, 0, err
	}
	p, err := r.FindBySlug(spaceID, slug)
	if err != nil {
		return nil, 0, err
	}
	if err := r.SaveDraft(spaceID, slug, h.Content, h.Summary, h.ContentHash, []int(h.Sources), p.Confidence); err != nil {
		return nil, 0, err
	}
	return p, toVersion, nil
}

// MarkSuperseded 标记页面被新页面取代（取代链）。
// 旧页面保留但不再被检索命中；新页面通过 SupersededBy 指向自己。
func (r *WikiPageRepository) MarkSuperseded(spaceID int, oldSlug string, newSlug string) error {
	return r.db.Model(&knowledgemodel.WikiPage{}).
		Where("space_id = ? AND slug = ?", spaceID, oldSlug).
		Update("superseded_by", newSlug).Error
}

// ListActivePages 列出空间下未被取代的已发布页面（检索用）。
func (r *WikiPageRepository) ListActivePages(spaceID int) ([]*knowledgemodel.WikiPage, error) {
	var pages []*knowledgemodel.WikiPage
	err := r.db.Where("space_id = ? AND status = ? AND superseded_by = ''", spaceID, string(knowledgemodel.WikiPageStatusPublished)).
		Order("slug ASC").Find(&pages).Error
	return pages, err
}

func removeInt(arr []int, v int) []int {
	out := arr[:0]
	for _, x := range arr {
		if x != v {
			out = append(out, x)
		}
	}
	if len(out) == 0 {
		return []int{}
	}
	return out
}

type WikiLinkRepository struct {
	db *gorm.DB
}

func NewWikiLinkRepository() *WikiLinkRepository {
	return &WikiLinkRepository{db: repos.DB}
}

// Upsert 幂等写入一条交叉引用。
func (r *WikiLinkRepository) Upsert(spaceID int, fromSlug, toSlug, linkType string, weight float64) error {
	if fromSlug == "" || toSlug == "" || fromSlug == toSlug {
		return nil
	}
	var existing knowledgemodel.WikiLink
	err := r.db.Where("space_id = ? AND from_slug = ? AND to_slug = ?", spaceID, fromSlug, toSlug).First(&existing).Error
	if err == nil {
		return r.db.Model(&existing).Updates(map[string]interface{}{
			"link_type": linkType,
			"weight":    weight,
		}).Error
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return err
	}
	return r.db.Create(&knowledgemodel.WikiLink{
		SpaceID:  spaceID,
		FromSlug: fromSlug,
		ToSlug:   toSlug,
		LinkType: linkType,
		Weight:   weight,
	}).Error
}

func (r *WikiLinkRepository) ListBySpace(spaceID int) ([]*knowledgemodel.WikiLink, error) {
	var links []*knowledgemodel.WikiLink
	err := r.db.Where("space_id = ?", spaceID).Find(&links).Error
	return links, err
}

func (r *WikiLinkRepository) DeleteBySlug(spaceID int, slug string) error {
	return r.db.Where("space_id = ? AND (from_slug = ? OR to_slug = ?)", spaceID, slug, slug).
		Delete(&knowledgemodel.WikiLink{}).Error
}
