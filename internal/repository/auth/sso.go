package auth

import (
	"database/sql"
	"errors"

	adminmodel "github.com/listen808/citebase/internal/model/admin"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrSSONotFound = errors.New("sso not found")
)

// AuthSSORepository SSO认证数据访问层
type AuthSSORepository struct {
	db *gorm.DB
}

func NewAuthSSORepository() *AuthSSORepository {
	return &AuthSSORepository{db: repos.DB}
}

func (r *AuthSSORepository) Create(sso *adminmodel.AuthSSO) error {
	return r.db.Create(sso).Error
}

func (r *AuthSSORepository) FindByID(id int) (*adminmodel.AuthSSO, error) {
	var sso adminmodel.AuthSSO
	err := r.db.First(&sso, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrSSONotFound
		}
		return nil, err
	}
	return &sso, nil
}

func (r *AuthSSORepository) HasName(name string) (bool, error) {
	var count int64
	err := r.db.Model(&adminmodel.AuthSSO{}).Where("name = ?", name).Count(&count).Error
	return count > 0, err
}

func (r *AuthSSORepository) HasNameExcludeID(id int, name string) (bool, error) {
	var count int64
	err := r.db.Model(&adminmodel.AuthSSO{}).Where("login_auth_id <> ? AND name = ?", id, name).Count(&count).Error
	return count > 0, err
}

func (r *AuthSSORepository) HasPrefix(prefix string) (bool, error) {
	var count int64
	err := r.db.Model(&adminmodel.AuthSSO{}).Where("username_prefix = ?", prefix).Count(&count).Error
	return count > 0, err
}

func (r *AuthSSORepository) HasPrefixExcludeID(id int, prefix string) (bool, error) {
	var count int64
	err := r.db.Model(&adminmodel.AuthSSO{}).Where("login_auth_id <> ? AND username_prefix = ?", id, prefix).Count(&count).Error
	return count > 0, err
}

func (r *AuthSSORepository) Update(sso *adminmodel.AuthSSO) error {
	return r.db.Save(sso).Error
}

func (r *AuthSSORepository) Delete(id int) error {
	return r.db.Delete(&adminmodel.AuthSSO{}, id).Error
}

func (r *AuthSSORepository) FindAll() ([]*adminmodel.AuthSSO, error) {
	var ssos []*adminmodel.AuthSSO
	err := r.db.Order("login_auth_id DESC").Find(&ssos).Error
	return ssos, err
}

func (r *AuthSSORepository) FindByKeyword(keyword string, offset, limit int) ([]*adminmodel.AuthSSO, int64, error) {
	var ssos []*adminmodel.AuthSSO
	var total int64
	query := r.db.Model(&adminmodel.AuthSSO{})
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Offset(offset).Limit(limit).Order("login_auth_id DESC").Find(&ssos).Error
	return ssos, total, err
}

func (r *AuthSSORepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&adminmodel.AuthSSO{}).Count(&count).Error
	return count, err
}

func (r *AuthSSORepository) GetUsed() (*adminmodel.AuthSSO, error) {
	var sso adminmodel.AuthSSO
	err := r.db.Where("is_used = ?", 1).Limit(1).Find(&sso).Error
	if err != nil {
		return nil, err
	}
	if sso.ID == 0 {
		return nil, nil
	}
	return &sso, nil
}

func (r *AuthSSORepository) UnsetAllUsed() error {
	return r.db.Model(&adminmodel.AuthSSO{}).Where("is_used = ?", 1).Update("is_used", 0).Error
}

func (r *AuthSSORepository) SetUsed(id int) error {
	return r.db.Model(&adminmodel.AuthSSO{}).Where("login_auth_id = ?", id).Update("is_used", 1).Error
}

func (r *AuthSSORepository) GetUsedAuthSSOs() ([]*adminmodel.AuthSSO, error) {
	var ssos []*adminmodel.AuthSSO
	err := r.db.Where("is_used = ?", 1).Find(&ssos).Error
	return ssos, err
}
