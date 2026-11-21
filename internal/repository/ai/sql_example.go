package ai

import (
	"strings"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

type SQLExampleRepo struct {
	db *gorm.DB
}

func NewSQLExampleRepo() *SQLExampleRepo {
	return &SQLExampleRepo{db: repos.DB}
}

func (r *SQLExampleRepo) ListBySpace(spaceID int, verifiedOnly bool, page, pageSize int) ([]*aimodel.SQLExample, int64, error) {
	var examples []*aimodel.SQLExample
	var total int64
	q := r.db.Model(&aimodel.SQLExample{}).Where("space_id = ?", spaceID)
	if verifiedOnly {
		q = q.Where("verified = 1")
	}
	q.Count(&total)
	err := q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&examples).Error
	return examples, total, err
}

func (r *SQLExampleRepo) Create(example *aimodel.SQLExample) error {
	return r.db.Create(example).Error
}

func (r *SQLExampleRepo) FindByID(id int) (*aimodel.SQLExample, error) {
	var example aimodel.SQLExample
	err := r.db.First(&example, id).Error
	if err != nil {
		return nil, err
	}
	return &example, nil
}

func (r *SQLExampleRepo) Verify(id int) error {
	return r.db.Model(&aimodel.SQLExample{}).Where("id = ?", id).Update("verified", 1).Error
}

func (r *SQLExampleRepo) Update(example *aimodel.SQLExample) error {
	return r.db.Save(example).Error
}

func (r *SQLExampleRepo) Delete(id int) error {
	return r.db.Delete(&aimodel.SQLExample{}, id).Error
}

func (r *SQLExampleRepo) FindVerifiedBySpace(spaceID int, limit int) ([]*aimodel.SQLExample, error) {
	var examples []*aimodel.SQLExample
	err := r.db.Where("space_id = ? AND verified = 1", spaceID).Order("created_at DESC").Limit(limit).Find(&examples).Error
	return examples, err
}

func (r *SQLExampleRepo) SearchByQuestionKeyword(spaceID int, keywords []string, limit int) ([]*aimodel.SQLExample, error) {
	var examples []*aimodel.SQLExample
	q := r.db.Model(&aimodel.SQLExample{}).Where("space_id = ? AND verified = 1", spaceID)

	if len(keywords) > 0 {
		conditions := make([]string, 0, len(keywords))
		args := make([]interface{}, 0, len(keywords))
		for _, kw := range keywords {
			if len(kw) >= 2 {
				conditions = append(conditions, "(LOWER(question) LIKE ? OR LOWER(sql) LIKE ?)")
				args = append(args, interface{}("%"+kw+"%"), interface{}("%"+kw+"%"))
			}
		}
		if len(conditions) > 0 {
			orClause := strings.Join(conditions, " OR ")
			q = q.Where(orClause, args...)
		}
	}

	err := q.Order("created_at DESC").Limit(limit).Find(&examples).Error
	return examples, err
}
