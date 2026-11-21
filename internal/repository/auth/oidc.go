package auth

import (
	"database/sql"
	"errors"

	adminmodel "github.com/listen808/citebase/internal/model/admin"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrOIDCProviderNotFound = errors.New("OIDC provider not found")
	ErrOIDCMappingNotFound  = errors.New("OIDC user mapping not found")
)

// OIDCProviderRepository OIDC 提供方数据访问层
type OIDCProviderRepository struct {
	db *gorm.DB
}

func NewOIDCProviderRepository() *OIDCProviderRepository {
	return &OIDCProviderRepository{db: repos.DB}
}

func (r *OIDCProviderRepository) Create(provider *adminmodel.OIDCProvider) error {
	return r.db.Create(provider).Error
}

func (r *OIDCProviderRepository) FindByID(id uint) (*adminmodel.OIDCProvider, error) {
	var provider adminmodel.OIDCProvider
	err := r.db.First(&provider, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOIDCProviderNotFound
		}
		return nil, err
	}
	return &provider, nil
}

func (r *OIDCProviderRepository) FindByName(name string) (*adminmodel.OIDCProvider, error) {
	var provider adminmodel.OIDCProvider
	err := r.db.Where("name = ?", name).First(&provider).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOIDCProviderNotFound
		}
		return nil, err
	}
	return &provider, nil
}

func (r *OIDCProviderRepository) FindEnabled() ([]*adminmodel.OIDCProvider, error) {
	var providers []*adminmodel.OIDCProvider
	err := r.db.Where("enabled = ?", true).Order("sort_order ASC, id ASC").Find(&providers).Error
	return providers, err
}

func (r *OIDCProviderRepository) FindAll() ([]*adminmodel.OIDCProvider, error) {
	var providers []*adminmodel.OIDCProvider
	err := r.db.Order("sort_order ASC, id ASC").Find(&providers).Error
	return providers, err
}

func (r *OIDCProviderRepository) HasName(name string) (bool, error) {
	var count int64
	err := r.db.Model(&adminmodel.OIDCProvider{}).Where("name = ?", name).Count(&count).Error
	return count > 0, err
}

func (r *OIDCProviderRepository) HasNameExcludeID(id uint, name string) (bool, error) {
	var count int64
	err := r.db.Model(&adminmodel.OIDCProvider{}).Where("id <> ? AND name = ?", id, name).Count(&count).Error
	return count > 0, err
}

func (r *OIDCProviderRepository) Update(provider *adminmodel.OIDCProvider) error {
	return r.db.Save(provider).Error
}

func (r *OIDCProviderRepository) Delete(id uint) error {
	return r.db.Delete(&adminmodel.OIDCProvider{}, id).Error
}

// OIDCUserMappingRepository OIDC 用户映射数据访问层
type OIDCUserMappingRepository struct {
	db *gorm.DB
}

func NewOIDCUserMappingRepository() *OIDCUserMappingRepository {
	return &OIDCUserMappingRepository{db: repos.DB}
}

func (r *OIDCUserMappingRepository) Create(mapping *adminmodel.OIDCUserMapping) error {
	return r.db.Create(mapping).Error
}

func (r *OIDCUserMappingRepository) FindByProviderAndSubject(providerID uint, subject string) (*adminmodel.OIDCUserMapping, error) {
	var mapping adminmodel.OIDCUserMapping
	err := r.db.Where("provider_id = ? AND subject = ?", providerID, subject).First(&mapping).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOIDCMappingNotFound
		}
		return nil, err
	}
	return &mapping, nil
}

func (r *OIDCUserMappingRepository) FindByProviderAndEmail(providerID uint, email string) (*adminmodel.OIDCUserMapping, error) {
	var mapping adminmodel.OIDCUserMapping
	err := r.db.Where("provider_id = ? AND email = ?", providerID, email).First(&mapping).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOIDCMappingNotFound
		}
		return nil, err
	}
	return &mapping, nil
}

func (r *OIDCUserMappingRepository) Update(mapping *adminmodel.OIDCUserMapping) error {
	return r.db.Save(mapping).Error
}

func (r *OIDCUserMappingRepository) DeleteByUserID(userID int) error {
	return r.db.Where("user_id = ?", userID).Delete(&adminmodel.OIDCUserMapping{}).Error
}

func (r *OIDCUserMappingRepository) FindByUserID(userID int) ([]*adminmodel.OIDCUserMapping, error) {
	var mappings []*adminmodel.OIDCUserMapping
	err := r.db.Where("user_id = ?", userID).Find(&mappings).Error
	return mappings, err
}
