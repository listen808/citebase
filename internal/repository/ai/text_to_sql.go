package ai

import (
	"errors"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	"gorm.io/gorm"
)

type TextToSQLRepo struct {
	db *gorm.DB
}

func NewTextToSQLRepo(db *gorm.DB) *TextToSQLRepo {
	return &TextToSQLRepo{db: db}
}

func (r *TextToSQLRepo) CreateRule(rule *aimodel.BusinessRule) error {
	return r.db.Create(rule).Error
}

func (r *TextToSQLRepo) FindByID(id uint64) (*aimodel.BusinessRule, error) {
	var rule aimodel.BusinessRule
	err := r.db.First(&rule, id).Error
	return &rule, err
}

func (r *TextToSQLRepo) UpdateRule(rule *aimodel.BusinessRule) error {
	return r.db.Save(rule).Error
}

func (r *TextToSQLRepo) DeleteRule(id uint64) error {
	return r.db.Delete(&aimodel.BusinessRule{}, id).Error
}

func (r *TextToSQLRepo) ListRules(page, pageSize int, ruleType, tableName string) ([]aimodel.BusinessRule, int64, error) {
	var rules []aimodel.BusinessRule
	var total int64

	query := r.db.Model(&aimodel.BusinessRule{})
	if ruleType != "" {
		query = query.Where("type = ?", ruleType)
	}
	if tableName != "" {
		query = query.Where("table_name = ?", tableName)
	}

	query.Count(&total)
	err := query.Offset((page - 1) * pageSize).Limit(pageSize).Order("id DESC").Find(&rules).Error
	return rules, total, err
}

func (r *TextToSQLRepo) GetRulesByTable(tableName string) ([]aimodel.BusinessRule, error) {
	var rules []aimodel.BusinessRule
	err := r.db.Where("table_name = ? AND status = 1", tableName).Find(&rules).Error
	return rules, err
}

func (r *TextToSQLRepo) UpsertTableCache(cache *aimodel.TableCache) error {
	// 表缓存是多空间设计（模型带 space_id/schema_space_id），旧实现只按 table_name 匹配：
	// 不同空间存在同名表（如 orders）时，后扫描的空间会把原行的 space_id 与 DDL 整体改写。
	// 必须以 (space_id, table_name) 复合键定位；space_id 为 NULL 也要显式匹配，避免误命中其他空间。
	q := r.db.Where("table_name = ?", cache.TableNam)
	if cache.SpaceID != nil {
		q = q.Where("space_id = ?", *cache.SpaceID)
	} else {
		q = q.Where("space_id IS NULL")
	}
	var existing aimodel.TableCache
	err := q.First(&existing).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// 仅「确认不存在」才创建；连接抖动等其它错误必须上抛，不能误创建重复行
		return r.db.Create(cache).Error
	}
	if err != nil {
		return err
	}
	cache.ID = existing.ID
	return r.db.Save(cache).Error
}

func (r *TextToSQLRepo) GetTableCache(tableName string) (*aimodel.TableCache, error) {
	var cache aimodel.TableCache
	err := r.db.Where("table_name = ?", tableName).First(&cache).Error
	return &cache, err
}

// GetTableCacheBySpace 按 (space_id, table_name) 复合键精确取表缓存；
// spaceID 为 nil 时匹配全局（无空间）记录，与 UpsertTableCache 的键语义一致。
func (r *TextToSQLRepo) GetTableCacheBySpace(tableName string, spaceID *uint64) (*aimodel.TableCache, error) {
	var cache aimodel.TableCache
	q := r.db.Where("table_name = ?", tableName)
	if spaceID != nil {
		q = q.Where("space_id = ?", *spaceID)
	} else {
		q = q.Where("space_id IS NULL")
	}
	err := q.First(&cache).Error
	return &cache, err
}

func (r *TextToSQLRepo) GetAllTableCaches() ([]aimodel.TableCache, error) {
	var caches []aimodel.TableCache
	err := r.db.Find(&caches).Error
	return caches, err
}

// ListTableCaches 分页读取表缓存（LIMIT/OFFSET 下推到 SQL）。
// 旧调用方把 GetAllTableCaches 全表读进内存再切片，随表数量线性放大。
func (r *TextToSQLRepo) ListTableCaches(page, pageSize int) ([]aimodel.TableCache, int64, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	var caches []aimodel.TableCache
	var total int64
	query := r.db.Model(&aimodel.TableCache{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	err := query.Order("id ASC").Offset(offset).Limit(pageSize).Find(&caches).Error
	return caches, total, err
}

func (r *TextToSQLRepo) SearchTables(keyword string) ([]aimodel.TableCache, error) {
	var caches []aimodel.TableCache
	err := r.db.Where("table_name LIKE ? OR table_comment LIKE ?", "%"+keyword+"%", "%"+keyword+"%").Find(&caches).Error
	return caches, err
}

func (r *TextToSQLRepo) CreateRelation(rel *aimodel.TableRelation) error {
	return r.db.Create(rel).Error
}

func (r *TextToSQLRepo) UpsertRelation(rel *aimodel.TableRelation) error {
	return r.db.Where("source_table = ? AND source_field = ? AND target_table = ? AND target_field = ?",
		rel.SourceTable, rel.SourceField, rel.TargetTable, rel.TargetField).
		Assign(rel).FirstOrCreate(rel).Error
}

func (r *TextToSQLRepo) GetRelationsByTable(tableName string) ([]aimodel.TableRelation, error) {
	var relations []aimodel.TableRelation
	err := r.db.Where("source_table = ? OR target_table = ?", tableName, tableName).Find(&relations).Error
	return relations, err
}

func (r *TextToSQLRepo) GetAllRelations() ([]aimodel.TableRelation, error) {
	var relations []aimodel.TableRelation
	err := r.db.Find(&relations).Error
	return relations, err
}

func (r *TextToSQLRepo) GetRelationsBySpace(spaceID uint64) ([]aimodel.TableRelation, error) {
	var relations []aimodel.TableRelation
	err := r.db.Where("space_id = ?", spaceID).Find(&relations).Error
	return relations, err
}

func (r *TextToSQLRepo) GetTableCachesBySpace(spaceID uint64) ([]aimodel.TableCache, error) {
	var caches []aimodel.TableCache
	err := r.db.Where("space_id = ?", spaceID).Find(&caches).Error
	return caches, err
}

func (r *TextToSQLRepo) DeleteRelation(id uint64) error {
	return r.db.Delete(&aimodel.TableRelation{}, id).Error
}

// DeleteTableCachesBySpace 删除指定空间下的所有表缓存
func (r *TextToSQLRepo) DeleteTableCachesBySpace(spaceID uint64) error {
	return r.db.Unscoped().Where("space_id = ?", spaceID).Delete(&aimodel.TableCache{}).Error
}

// DeleteTableCacheBySpaceAndName 按 (space_id, table_name) 删除单条表缓存：
// 增量扫描发现文档里已移除的表时必须同步删除 DB 行，否则残留幽灵表，
// 后续 FindTableByName/知识库会查到已不存在的表定义。
func (r *TextToSQLRepo) DeleteTableCacheBySpaceAndName(tableName string, spaceID uint64) error {
	return r.db.Unscoped().Where("space_id = ? AND table_name = ?", spaceID, tableName).Delete(&aimodel.TableCache{}).Error
}

// DeleteRelationsBySpace 删除指定空间下的所有表关系
func (r *TextToSQLRepo) DeleteRelationsBySpace(spaceID uint64) error {
	return r.db.Unscoped().Where("space_id = ?", spaceID).Delete(&aimodel.TableRelation{}).Error
}

// CountTableCachesBySpace 统计指定空间下的表缓存数量
func (r *TextToSQLRepo) CountTableCachesBySpace(spaceID uint64) (int64, error) {
	var count int64
	err := r.db.Model(&aimodel.TableCache{}).Where("space_id = ?", spaceID).Count(&count).Error
	return count, err
}
