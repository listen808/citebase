package schema

import (
	"errors"
	aimodel "github.com/listen808/citebase/internal/model/ai"
	schemamodel "github.com/listen808/citebase/internal/model/schema"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

var ErrSchemaSpaceNotFound = errors.New("schema space not found")

// SchemaRepository Schema数据访问层
type SchemaRepository struct {
	db *gorm.DB
}

func NewSchemaRepository() *SchemaRepository {
	return &SchemaRepository{db: repos.DB}
}

// NewSchemaRepositoryWithDB 用调用方给进来的连接建仓库。
// 原先写的是 {db: repos.DB} —— 传进来的 db 被直接丢掉，
// 于是"注入连接"这条路一直走的是全局连接：单测没法用 sqlite/事务包住它，
// 而调用方以为自己给了一个连接。
func NewSchemaRepositoryWithDB(db *gorm.DB) *SchemaRepository {
	return &SchemaRepository{db: db}
}

// Transaction 在一条事务里跑 fn，供跨表级联写用（见 ClearSpaceCascade）。
func (r *SchemaRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

// ClearSpaceCascade 在一个事务里删掉某个 Schema 空间的全部下游数据。
//
// 这条方法存在的理由：原先服务侧是六行各调各的 Delete*，一行都没检查错误 ——
// 中途任何一步失败（连接断了、外键挡住、表被锁）都会留下"只剩一半"的空间：
// 表删了、列还在，向量集合也还留着旧数据，而接口照样返回成功。
// 删空间与重扫清空走的是同一份级联，所以口径只有这一处。
func (r *SchemaRepository) ClearSpaceCascade(id int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, model := range []interface{}{
			&schemamodel.SchemaScanTask{},
			&schemamodel.SchemaTable{},
			&schemamodel.SchemaColumn{},
			&schemamodel.SchemaRelation{},
			&schemamodel.SchemaBusinessTag{},
		} {
			if err := tx.Where("schema_space_id = ?", id).Delete(model).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// DeleteSpaceCascade 先清下游再删空间本体，整条在一个事务里。
func (r *SchemaRepository) DeleteSpaceCascade(id int) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		for _, model := range []interface{}{
			&schemamodel.SchemaScanTask{},
			&schemamodel.SchemaTable{},
			&schemamodel.SchemaColumn{},
			&schemamodel.SchemaRelation{},
			&schemamodel.SchemaBusinessTag{},
		} {
			if err := tx.Where("schema_space_id = ?", id).Delete(model).Error; err != nil {
				return err
			}
		}
		return tx.Delete(&schemamodel.SchemaSpace{}, id).Error
	})
}

// --- SchemaSpace ---

func (r *SchemaRepository) CreateSpace(space *schemamodel.SchemaSpace) error {
	return r.db.Create(space).Error
}

func (r *SchemaRepository) FindSpaceByID(id int) (*schemamodel.SchemaSpace, error) {
	var space schemamodel.SchemaSpace
	err := r.db.First(&space, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSchemaSpaceNotFound
		}
		return nil, err
	}
	return &space, nil
}

func (r *SchemaRepository) FindSpaceByWikiSpaceID(spaceID int) (*schemamodel.SchemaSpace, error) {
	var space schemamodel.SchemaSpace
	err := r.db.Where("space_id = ?", spaceID).First(&space).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrSchemaSpaceNotFound
		}
		return nil, err
	}
	return &space, nil
}

func (r *SchemaRepository) ListSpaces(page, pageSize int) ([]*schemamodel.SchemaSpace, int64, error) {
	var spaces []*schemamodel.SchemaSpace
	var total int64
	r.db.Model(&schemamodel.SchemaSpace{}).Count(&total)
	err := r.db.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&spaces).Error
	return spaces, total, err
}

func (r *SchemaRepository) UpdateSpace(space *schemamodel.SchemaSpace) error {
	return r.db.Save(space).Error
}

func (r *SchemaRepository) DeleteSpace(id int) error {
	return r.db.Delete(&schemamodel.SchemaSpace{}, id).Error
}

// --- SchemaTable ---

func (r *SchemaRepository) CreateTable(table *schemamodel.SchemaTable) error {
	return r.db.Create(table).Error
}

func (r *SchemaRepository) FindTableByName(schemaSpaceID int, tableName string) (*schemamodel.SchemaTable, error) {
	var table schemamodel.SchemaTable
	err := r.db.Where("schema_space_id = ? AND table_name = ?", schemaSpaceID, tableName).First(&table).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, errors.New("table not found")
		}
		return nil, err
	}
	return &table, nil
}

func (r *SchemaRepository) ListTables(schemaSpaceID int, keyword string, page, pageSize int) ([]*schemamodel.SchemaTable, int64, error) {
	var tables []*schemamodel.SchemaTable
	var total int64
	query := r.db.Model(&schemamodel.SchemaTable{}).Where("schema_space_id = ?", schemaSpaceID)
	if keyword != "" {
		query = query.Where("table_name LIKE ? OR table_comment LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	query.Count(&total)
	err := query.Order("table_name ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&tables).Error
	return tables, total, err
}

func (r *SchemaRepository) ListAllTables(schemaSpaceID int) ([]*schemamodel.SchemaTable, error) {
	var tables []*schemamodel.SchemaTable
	err := r.db.Where("schema_space_id = ?", schemaSpaceID).Order("table_name ASC").Find(&tables).Error
	return tables, err
}

func (r *SchemaRepository) DeleteTablesBySpace(schemaSpaceID int) error {
	return r.db.Where("schema_space_id = ?", schemaSpaceID).Delete(&schemamodel.SchemaTable{}).Error
}

// --- SchemaColumn ---

func (r *SchemaRepository) CreateColumn(column *schemamodel.SchemaColumn) error {
	return r.db.Create(column).Error
}

func (r *SchemaRepository) ListColumnsByTableID(tableID int) ([]*schemamodel.SchemaColumn, error) {
	var columns []*schemamodel.SchemaColumn
	err := r.db.Where("table_id = ?", tableID).Order("ordinal_position ASC").Find(&columns).Error
	return columns, err
}

func (r *SchemaRepository) ListColumnsByTableName(schemaSpaceID int, tableName string) ([]*schemamodel.SchemaColumn, error) {
	var columns []*schemamodel.SchemaColumn
	err := r.db.Where("schema_space_id = ? AND table_name = ?", schemaSpaceID, tableName).Order("ordinal_position ASC").Find(&columns).Error
	return columns, err
}

// CountColumnsBySpace 一个空间里一共有多少列。
// 加这条是因为 Schema 统计原先在服务侧 `for table { ListColumnsByTableName(...) }` ——
// 一张表一次查询，把整张表的列**全部读进内存**只为了 len()。
// 100 张表的库就是 100 次往返加 100 份完整列数据，而答案只是一个数。
func (r *SchemaRepository) CountColumnsBySpace(schemaSpaceID int) (int64, error) {
	var count int64
	err := r.db.Model(&schemamodel.SchemaColumn{}).
		Where("schema_space_id = ?", schemaSpaceID).
		Count(&count).Error
	return count, err
}

func (r *SchemaRepository) FindColumnsByName(schemaSpaceID int, columnName string) ([]*schemamodel.SchemaColumn, error) {
	var columns []*schemamodel.SchemaColumn
	err := r.db.Where("schema_space_id = ? AND column_name = ?", schemaSpaceID, columnName).Find(&columns).Error
	return columns, err
}

func (r *SchemaRepository) DeleteColumnsBySpace(schemaSpaceID int) error {
	return r.db.Where("schema_space_id = ?", schemaSpaceID).Delete(&schemamodel.SchemaColumn{}).Error
}

// --- SchemaRelation ---

func (r *SchemaRepository) CreateRelation(relation *schemamodel.SchemaRelation) error {
	return r.db.Create(relation).Error
}

func (r *SchemaRepository) ListRelations(schemaSpaceID int) ([]*schemamodel.SchemaRelation, error) {
	var relations []*schemamodel.SchemaRelation
	err := r.db.Where("schema_space_id = ?", schemaSpaceID).Find(&relations).Error
	return relations, err
}

func (r *SchemaRepository) ListRelationsByTable(schemaSpaceID int, tableName string) ([]*schemamodel.SchemaRelation, error) {
	var relations []*schemamodel.SchemaRelation
	err := r.db.Where("schema_space_id = ? AND (source_table = ? OR target_table = ?)", schemaSpaceID, tableName, tableName).Find(&relations).Error
	return relations, err
}

func (r *SchemaRepository) DeleteRelationsBySpace(schemaSpaceID int) error {
	return r.db.Where("schema_space_id = ?", schemaSpaceID).Delete(&schemamodel.SchemaRelation{}).Error
}

// --- SchemaBusinessTag ---

func (r *SchemaRepository) CreateTag(tag *schemamodel.SchemaBusinessTag) error {
	return r.db.Create(tag).Error
}

func (r *SchemaRepository) ListTagsBySpace(schemaSpaceID int) ([]*schemamodel.SchemaBusinessTag, error) {
	var tags []*schemamodel.SchemaBusinessTag
	err := r.db.Where("schema_space_id = ?", schemaSpaceID).Find(&tags).Error
	return tags, err
}

func (r *SchemaRepository) ListTagsByTable(schemaSpaceID int, tableName string) ([]*schemamodel.SchemaBusinessTag, error) {
	var tags []*schemamodel.SchemaBusinessTag
	err := r.db.Where("schema_space_id = ? AND table_name = ?", schemaSpaceID, tableName).Find(&tags).Error
	return tags, err
}

func (r *SchemaRepository) DeleteTagsBySpace(schemaSpaceID int) error {
	return r.db.Where("schema_space_id = ?", schemaSpaceID).Delete(&schemamodel.SchemaBusinessTag{}).Error
}

// --- SchemaScanTask ---

func (r *SchemaRepository) CreateScanTask(task *schemamodel.SchemaScanTask) error {
	return r.db.Create(task).Error
}

func (r *SchemaRepository) FindScanTaskByID(id int) (*schemamodel.SchemaScanTask, error) {
	var task schemamodel.SchemaScanTask
	err := r.db.First(&task, id).Error
	if err != nil {
		return nil, err
	}
	return &task, nil
}

func (r *SchemaRepository) UpdateScanTask(task *schemamodel.SchemaScanTask) error {
	return r.db.Model(&schemamodel.SchemaScanTask{}).Where("id = ?", task.ID).Updates(map[string]interface{}{
		"status":         task.Status,
		"processed_docs": task.ProcessedDocs,
		"total_docs":     task.TotalDocs,
		"found_tables":   task.FoundTables,
		"error_message":  task.ErrorMessage,
		"completed_at":   task.CompletedAt,
	}).Error
}

func (r *SchemaRepository) ListScanTasks(schemaSpaceID int) ([]*schemamodel.SchemaScanTask, error) {
	var tasks []*schemamodel.SchemaScanTask
	err := r.db.Where("schema_space_id = ?", schemaSpaceID).Order("created_at DESC").Find(&tasks).Error
	return tasks, err
}

func (r *SchemaRepository) DeleteScanTask(id int) error {
	return r.db.Delete(&schemamodel.SchemaScanTask{}, id).Error
}

func (r *SchemaRepository) DeleteScanTasksBySpace(schemaSpaceID int) error {
	return r.db.Where("schema_space_id = ?", schemaSpaceID).Delete(&schemamodel.SchemaScanTask{}).Error
}

// --- TableCache ---

func (r *SchemaRepository) ListTableCache(schemaSpaceID int, keyword string, page, pageSize int) ([]aimodel.TableCache, int64, error) {
	var caches []aimodel.TableCache
	var total int64
	query := r.db.Model(&aimodel.TableCache{}).Where("schema_space_id = ?", schemaSpaceID)
	if keyword != "" {
		query = query.Where("table_name LIKE ? OR table_comment LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	query.Count(&total)
	err := query.Order("table_name ASC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&caches).Error
	return caches, total, err
}

// --- BusinessRule ---

func (r *SchemaRepository) ListBusinessRules(schemaSpaceID int, page, pageSize int) ([]aimodel.BusinessRule, int64, error) {
	var rules []aimodel.BusinessRule
	var total int64
	query := r.db.Model(&aimodel.BusinessRule{}).Where("schema_space_id = ?", schemaSpaceID)
	query.Count(&total)
	err := query.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&rules).Error
	return rules, total, err
}

func (r *SchemaRepository) CreateBusinessRule(rule *aimodel.BusinessRule) error {
	return r.db.Create(rule).Error
}

func (r *SchemaRepository) DeleteBusinessRule(id uint64) error {
	return r.db.Delete(&aimodel.BusinessRule{}, id).Error
}

func (r *SchemaRepository) DeleteBusinessRulesBySpace(schemaSpaceID int) error {
	return r.db.Where("schema_space_id = ?", schemaSpaceID).Delete(&aimodel.BusinessRule{}).Error
}
