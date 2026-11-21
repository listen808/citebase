package admin

import (
	"database/sql"
	"errors"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrEmailNotFound = errors.New("email not found")
)

// EmailRepository 邮件服务器数据访问层
type EmailRepository struct {
	db *gorm.DB
}

func NewEmailRepository() *EmailRepository {
	return &EmailRepository{db: repos.DB}
}

func (r *EmailRepository) Create(email *adminmodel.Email) error {
	return r.db.Create(email).Error
}

func (r *EmailRepository) FindByID(id int) (*adminmodel.Email, error) {
	var email adminmodel.Email
	err := r.db.First(&email, id).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEmailNotFound
		}
		return nil, err
	}
	return &email, nil
}

func (r *EmailRepository) FindByName(name string) (*adminmodel.Email, error) {
	var email adminmodel.Email
	err := r.db.Where("name = ?", name).First(&email).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrEmailNotFound
		}
		return nil, err
	}
	return &email, nil
}

func (r *EmailRepository) HasName(name string) (bool, error) {
	var count int64
	err := r.db.Model(&adminmodel.Email{}).Where("name = ?", name).Count(&count).Error
	return count > 0, err
}

func (r *EmailRepository) HasNameExcludeID(id int, name string) (bool, error) {
	var count int64
	err := r.db.Model(&adminmodel.Email{}).Where("email_id <> ? AND name = ?", id, name).Count(&count).Error
	return count > 0, err
}

func (r *EmailRepository) Update(email *adminmodel.Email) error {
	return r.db.Save(email).Error
}

func (r *EmailRepository) Delete(id int) error {
	return r.db.Delete(&adminmodel.Email{}, id).Error
}

func (r *EmailRepository) FindAll() ([]*adminmodel.Email, error) {
	var emails []*adminmodel.Email
	err := r.db.Order("email_id DESC").Find(&emails).Error
	return emails, err
}

func (r *EmailRepository) FindByKeyword(keyword string, offset, limit int) ([]*adminmodel.Email, int64, error) {
	var emails []*adminmodel.Email
	var total int64
	query := r.db.Model(&adminmodel.Email{})
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	err := query.Offset(offset).Limit(limit).Order("email_id DESC").Find(&emails).Error
	return emails, total, err
}

func (r *EmailRepository) Count() (int64, error) {
	var count int64
	err := r.db.Model(&adminmodel.Email{}).Count(&count).Error
	return count, err
}

func (r *EmailRepository) GetUsedEmail() (*adminmodel.Email, error) {
	var email adminmodel.Email
	err := r.db.Where("is_used = ?", 1).First(&email).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &email, nil
}

func (r *EmailRepository) UnsetAllUsed() error {
	return r.db.Model(&adminmodel.Email{}).Where("is_used = ?", 1).Update("is_used", 0).Error
}

func (r *EmailRepository) SetUsed(id int) error {
	return r.db.Model(&adminmodel.Email{}).Where("email_id = ?", id).Update("is_used", 1).Error
}

func (r *EmailRepository) SetUsedTx(tx *gorm.DB, id int) error {
	return tx.Model(&adminmodel.Email{}).Where("email_id = ?", id).Update("is_used", 1).Error
}
