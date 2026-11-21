package ai

import (
	aimodel "github.com/listen808/citebase/internal/model/ai"
	"gorm.io/gorm"
)

// SQLSecurityPolicyRepo SQL 执行安全策略仓库
type SQLSecurityPolicyRepo struct {
	db *gorm.DB
}

func NewSQLSecurityPolicyRepo(db *gorm.DB) *SQLSecurityPolicyRepo {
	return &SQLSecurityPolicyRepo{db: db}
}

func (r *SQLSecurityPolicyRepo) Create(policy *aimodel.SQLSecurityPolicy) error {
	return r.db.Create(policy).Error
}

func (r *SQLSecurityPolicyRepo) Update(policy *aimodel.SQLSecurityPolicy) error {
	return r.db.Save(policy).Error
}

func (r *SQLSecurityPolicyRepo) GetBySpace(spaceID uint64) (*aimodel.SQLSecurityPolicy, error) {
	var policy aimodel.SQLSecurityPolicy
	if err := r.db.Where("space_id = ?", spaceID).First(&policy).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return &policy, nil
}

func (r *SQLSecurityPolicyRepo) GetGlobalDefault() (*aimodel.SQLSecurityPolicy, error) {
	return r.GetBySpace(0)
}

func (r *SQLSecurityPolicyRepo) GetEffectivePolicy(spaceID uint64) (*aimodel.SQLSecurityPolicy, error) {
	policy, err := r.GetBySpace(spaceID)
	if err != nil {
		return nil, err
	}
	if policy != nil {
		return policy, nil
	}
	return r.GetGlobalDefault()
}

func (r *SQLSecurityPolicyRepo) CreateOrUpdateBySpace(policy *aimodel.SQLSecurityPolicy) error {
	existing, err := r.GetBySpace(policy.SpaceID)
	if err != nil {
		return err
	}
	if existing == nil {
		return r.Create(policy)
	}
	policy.ID = existing.ID
	return r.Update(policy)
}
