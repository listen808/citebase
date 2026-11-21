package social

import (
	"database/sql"
	"errors"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	socialmodel "github.com/listen808/citebase/internal/model/social"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrFollowNotFound = errors.New("follow not found")
)

// FollowRepository 用户关注数据访问层
type FollowRepository struct {
	db *gorm.DB
}

// NewFollowRepository 创建关注仓储实例
func NewFollowRepository() *FollowRepository {
	return &FollowRepository{db: repos.DB}
}

// Create 创建关注记录
func (r *FollowRepository) Create(follow *socialmodel.Follow) error {
	return r.db.Create(follow).Error
}

// Delete 删除关注记录
func (r *FollowRepository) Delete(followID int) error {
	return r.db.Delete(&socialmodel.Follow{}, "follow_id = ?", followID).Error
}

// FindByUserAndObject 查找特定用户的特定关注
func (r *FollowRepository) FindByUserAndObject(userID int, followType int, objectID int) (*socialmodel.Follow, error) {
	var follow socialmodel.Follow
	err := r.db.Where("user_id = ? AND type = ? AND object_id = ?", userID, followType, objectID).First(&follow).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrFollowNotFound
		}
		return nil, err
	}
	return &follow, nil
}

// FindByUserID 获取用户的所有关注（指定类型）
func (r *FollowRepository) FindByUserID(userID int, followType int) ([]*socialmodel.Follow, error) {
	var follows []*socialmodel.Follow
	err := r.db.Where("user_id = ? AND type = ?", userID, followType).Order("create_time DESC").Find(&follows).Error
	return follows, err
}

// FindFollowers 获取关注某对象的用户列表
func (r *FollowRepository) FindFollowers(objectID int, followType int) ([]*socialmodel.Follow, error) {
	var follows []*socialmodel.Follow
	err := r.db.Where("object_id = ? AND type = ?", objectID, followType).Find(&follows).Error
	return follows, err
}

// CountFollowers 统计粉丝数
func (r *FollowRepository) CountFollowers(objectID int, followType int) (int64, error) {
	var count int64
	err := r.db.Model(&socialmodel.Follow{}).Where("object_id = ? AND type = ?", objectID, followType).Count(&count).Error
	return count, err
}

// CountFollowing 统计关注数
func (r *FollowRepository) CountFollowing(userID int, followType int) (int64, error) {
	var count int64
	err := r.db.Model(&socialmodel.Follow{}).Where("user_id = ? AND type = ?", userID, followType).Count(&count).Error
	return count, err
}

// GetUserFollowing 获取用户关注的人列表（type=2 表示关注用户）
func (r *FollowRepository) GetUserFollowing(userID int) ([]*socialmodel.Follow, error) {
	return r.FindByUserID(userID, socialmodel.FollowTypeUser)
}

// GetUserFollowers 获取用户的粉丝列表（type=2 表示被关注）
func (r *FollowRepository) GetUserFollowers(userID int) ([]*socialmodel.Follow, error) {
	return r.FindFollowers(userID, socialmodel.FollowTypeUser)
}

// GetUserFollowDocs 获取用户关注的文档列表（type=1）
func (r *FollowRepository) GetUserFollowDocs(userID int) ([]*socialmodel.Follow, error) {
	return r.FindByUserID(userID, socialmodel.FollowTypeDocument)
}

// IsFollowing 检查是否已关注
func (r *FollowRepository) IsFollowing(userID int, followType int, objectID int) bool {
	var count int64
	if err := r.db.Model(&socialmodel.Follow{}).Where("user_id = ? AND type = ? AND object_id = ?", userID, followType, objectID).Count(&count).Error; err != nil {
		return false
	}
	return count > 0
}

// DeleteByUserAndObject 根据用户和对象删除关注
func (r *FollowRepository) DeleteByUserAndObject(userID int, followType int, objectID int) error {
	return r.db.Where("user_id = ? AND type = ? AND object_id = ?", userID, followType, objectID).Delete(&socialmodel.Follow{}).Error
}

// GetTopFollower 获取粉丝最多的用户
func (r *FollowRepository) GetTopFollower() *adminmodel.UserStatItem {
	var result struct {
		UserID    int
		Username  string
		GivenName string
		Count     int
	}
	err := r.db.Table("aw_follow").
		Select("aw_follow.object_id as user_id, aw_user.username, aw_user.given_name, COUNT(*) as count").
		Joins("LEFT JOIN aw_user ON aw_follow.object_id = aw_user.user_id").
		Where("aw_follow.type = 2"). // type=2 为关注用户
		Group("aw_follow.object_id").
		Order("count DESC").
		Limit(1).
		Scan(&result).Error
	if err != nil || result.UserID == 0 {
		return nil
	}
	return &adminmodel.UserStatItem{UserID: result.UserID, Username: result.Username, GivenName: result.GivenName, Count: result.Count}
}
