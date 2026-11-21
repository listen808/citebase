package social

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
	socialmodel "github.com/listen808/citebase/internal/model/social"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

// mysqlErrDuplicateEntry 是 MySQL 的唯一键冲突错误号。
const mysqlErrDuplicateEntry = 1062

// IsDuplicateKey 判断错误是否为"这一行已经存在"。
//
// 收藏与关注都是「查一次没有 → 插一条」的写法，两个请求并发（或用户双击、
// 或两个标签页同时点）就会都看见"没有"，其中一条 INSERT 撞 uk_user_resource /
// idx_user_object_type 的 1062。库里建了唯一索引是对的（它是这条竞态的最后一道防线），
// 但把它当 500 抛给用户是错的 —— 用户要的是"我已经收藏了"，不是"操作失败"。
//
// 之所以自己判而不用 gorm.ErrDuplicatedKey：DB 连接没开 TranslateError
// （internal/repository/db.go 的 gorm.Config 里没有这一项），
// 那个哨兵值恒不会出现。开 TranslateError 是全站行为改动，不该为两处 toggle 顺手做。
func IsDuplicateKey(err error) bool {
	if err == nil {
		return false
	}
	var myErr *mysql.MySQLError
	if errors.As(err, &myErr) {
		return myErr.Number == mysqlErrDuplicateEntry
	}
	// 兜底：错误被包装成非驱动类型时（例如迁移到别的驱动或被上层重写），
	// 仍按 1062 的文案识别。字符串判据脆，所以只放在 errors.As 已经失败之后。
	return strings.Contains(err.Error(), "Error 1062") || strings.Contains(err.Error(), "Duplicate entry")
}

// CollectionRepository 收藏数据访问层
type CollectionRepository struct {
	db *gorm.DB
}

func NewCollectionRepository() *CollectionRepository {
	return &CollectionRepository{db: repos.DB}
}

// Create 创建收藏
func (r *CollectionRepository) Create(c *socialmodel.Collection) error {
	c.CreateTime = int(time.Now().Unix())
	return r.db.Create(c).Error
}

// FindByUserAndResource 查找用户的收藏记录
func (r *CollectionRepository) FindByUserAndResource(userID, resourceType, resourceID int) (*socialmodel.Collection, error) {
	var col socialmodel.Collection
	err := r.db.Where("user_id = ? AND type = ? AND resource_id = ?", userID, resourceType, resourceID).First(&col).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &col, nil
}

// GetUserCollections 获取用户的所有收藏
func (r *CollectionRepository) GetUserCollections(userID, resourceType int, limit, offset int) ([]*socialmodel.Collection, int64, error) {
	var cols []*socialmodel.Collection
	var total int64

	query := r.db.Model(&socialmodel.Collection{}).Where("user_id = ?", userID)
	if resourceType > 0 {
		query = query.Where("type = ?", resourceType)
	}

	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	err := query.Order("create_time DESC").Offset(offset).Limit(limit).Find(&cols).Error
	return cols, total, err
}

// Delete 删除收藏
func (r *CollectionRepository) Delete(id int) error {
	return r.db.Where("collection_id = ?", id).Delete(&socialmodel.Collection{}).Error
}

// DeleteByUserAndResource 删除指定收藏
func (r *CollectionRepository) DeleteByUserAndResource(userID, resourceType, resourceID int) error {
	return r.db.Where("user_id = ? AND type = ? AND resource_id = ?", userID, resourceType, resourceID).
		Delete(&socialmodel.Collection{}).Error
}

// DeleteByResourceID 删除某资源的所有收藏（不限用户，级联清理用）
func (r *CollectionRepository) DeleteByResourceID(resourceType, resourceID int) error {
	return r.db.Where("type = ? AND resource_id = ?", resourceType, resourceID).
		Delete(&socialmodel.Collection{}).Error
}

// DocCollectItem 文档收藏排行项
type DocCollectItem struct {
	DocumentID int    `json:"document_id"`
	Name       string `json:"name"`
	Count      int64  `json:"count"`
}

// GetCollectDocRank 获取文档收藏排行（按收藏数降序）
func (r *CollectionRepository) GetCollectDocRank(limit int) ([]DocCollectItem, error) {
	var results []DocCollectItem
	err := r.db.Model(&socialmodel.Collection{}).
		Select("aw_collection.resource_id AS document_id, aw_document.name, COUNT(*) AS count").
		Joins("JOIN aw_document ON aw_collection.resource_id = aw_document.document_id AND aw_document.is_delete = 0").
		Where("aw_collection.type = ?", socialmodel.CollectionTypeDocument).
		Group("aw_collection.resource_id, aw_document.name").
		Order("count DESC").
		Limit(limit).
		Find(&results).Error
	return results, err
}
