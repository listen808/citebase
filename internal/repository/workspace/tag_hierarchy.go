package workspace

import (
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

type TagHierarchyRepository struct {
	db *gorm.DB
}

func NewTagHierarchyRepository() *TagHierarchyRepository {
	return &TagHierarchyRepository{db: repository.DB}
}

func NewTagHierarchyRepositoryWithDB(db *gorm.DB) *TagHierarchyRepository {
	return &TagHierarchyRepository{db: db}
}

func (r *TagHierarchyRepository) Create(hierarchy *workspacemodel.TagHierarchy) error {
	return r.db.Create(hierarchy).Error
}

func (r *TagHierarchyRepository) FindByParentTag(parentTag string, spaceID int64) ([]workspacemodel.TagHierarchy, error) {
	var items []workspacemodel.TagHierarchy
	query := r.db.Where("parent_tag = ?", parentTag)
	if spaceID > 0 {
		query = query.Where("space_id = ? OR space_id = 0", spaceID)
	}
	err := query.Find(&items).Error
	return items, err
}

func (r *TagHierarchyRepository) FindByChildTag(childTag string) (*workspacemodel.TagHierarchy, error) {
	var item workspacemodel.TagHierarchy
	err := r.db.Where("child_tag = ?", childTag).First(&item).Error
	if err != nil {
		return nil, err
	}
	return &item, nil
}

func (r *TagHierarchyRepository) FindRootTags(spaceID int64) ([]string, error) {
	var rootTags []string
	err := r.db.Raw(`
		SELECT DISTINCT parent_tag FROM aw_tag_hierarchy 
		WHERE parent_tag NOT IN (SELECT child_tag FROM aw_tag_hierarchy)
		AND (space_id = ? OR space_id = 0)
	`, spaceID).Pluck("parent_tag", &rootTags).Error
	return rootTags, err
}

func (r *TagHierarchyRepository) Delete(id int64) error {
	return r.db.Delete(&workspacemodel.TagHierarchy{}, id).Error
}

func (r *TagHierarchyRepository) DeleteByParent(parentTag string) error {
	return r.db.Where("parent_tag = ?", parentTag).Delete(&workspacemodel.TagHierarchy{}).Error
}

func (r *TagHierarchyRepository) DeleteByChild(childTag string) error {
	return r.db.Where("child_tag = ?", childTag).Delete(&workspacemodel.TagHierarchy{}).Error
}

func (r *TagHierarchyRepository) GetAll(spaceID int64) ([]workspacemodel.TagHierarchy, error) {
	var items []workspacemodel.TagHierarchy
	query := r.db
	if spaceID > 0 {
		query = query.Where("space_id = ? OR space_id = 0", spaceID)
	}
	err := query.Order("parent_tag, child_tag").Find(&items).Error
	return items, err
}
