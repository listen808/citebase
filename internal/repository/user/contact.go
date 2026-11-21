package user

import (
	"database/sql"
	"errors"

	usermodel "github.com/listen808/citebase/internal/model/user"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

var (
	ErrContactNotFound = errors.New("contact not found")
)

type ContactRepository struct {
	db *gorm.DB
}

func NewContactRepository() *ContactRepository {
	return &ContactRepository{db: repos.DB}
}

func NewContactRepositoryWithDB(db *gorm.DB) *ContactRepository {
	return &ContactRepository{db: db}
}

func (r *ContactRepository) Transaction(fn func(tx *gorm.DB) error) error {
	return r.db.Transaction(fn)
}

func (r *ContactRepository) Create(contact *usermodel.Contact) error {
	return r.db.Create(contact).Error
}

func (r *ContactRepository) Update(contact *usermodel.Contact) error {
	return r.db.Save(contact).Error
}

func (r *ContactRepository) Delete(contactID uint) error {
	return r.db.Delete(&usermodel.Contact{}, contactID).Error
}

func (r *ContactRepository) FindByID(contactID uint) (*usermodel.Contact, error) {
	var contact usermodel.Contact
	err := r.db.First(&contact, contactID).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) || errors.Is(err, sql.ErrNoRows) {
			return nil, ErrContactNotFound
		}
		return nil, err
	}
	return &contact, nil
}

func (r *ContactRepository) List(page, pageSize int, keyword string) ([]usermodel.Contact, int64, error) {
	var contacts []usermodel.Contact
	var total int64

	query := r.db.Model(&usermodel.Contact{})
	if keyword != "" {
		query = query.Where("name LIKE ?", "%"+keyword+"%")
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	err := query.Order("contact_id DESC").Offset(offset).Limit(pageSize).Find(&contacts).Error
	return contacts, total, err
}

func (r *ContactRepository) ListAll() ([]usermodel.Contact, error) {
	var contacts []usermodel.Contact
	err := r.db.Find(&contacts).Error
	return contacts, err
}
