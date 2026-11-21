package knowledge

import (
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	"gorm.io/gorm"
)

type KnowledgeGraphRepository struct {
	db *gorm.DB
}

func NewKnowledgeGraphRepository(db *gorm.DB) *KnowledgeGraphRepository {
	return &KnowledgeGraphRepository{db: db}
}

func (r *KnowledgeGraphRepository) CreateEntity(entity *knowledgemodel.KnowledgeEntity) error {
	return r.db.Create(entity).Error
}

func (r *KnowledgeGraphRepository) CreateRelation(relation *knowledgemodel.KnowledgeRelation) error {
	return r.db.Create(relation).Error
}

func (r *KnowledgeGraphRepository) FindEntitiesByDocID(docID int) ([]knowledgemodel.KnowledgeEntity, error) {
	var entities []knowledgemodel.KnowledgeEntity
	err := r.db.Where("doc_id = ?", docID).Find(&entities).Error
	return entities, err
}

func (r *KnowledgeGraphRepository) FindEntitiesBySpaceID(spaceID int) ([]knowledgemodel.KnowledgeEntity, error) {
	var entities []knowledgemodel.KnowledgeEntity
	err := r.db.Where("space_id = ?", spaceID).Find(&entities).Error
	return entities, err
}

func (r *KnowledgeGraphRepository) FindEntityByName(name string) (*knowledgemodel.KnowledgeEntity, error) {
	var entity knowledgemodel.KnowledgeEntity
	err := r.db.Where("name = ?", name).First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

type RelatedEntity struct {
	DocID        int
	EntityName   string
	RelationType string
	Confidence   float64
}

func (r *KnowledgeGraphRepository) FindRelatedEntities(docID int, depth int) ([]RelatedEntity, error) {
	if depth <= 0 {
		return nil, nil
	}

	var sourceEntities []knowledgemodel.KnowledgeEntity
	if err := r.db.Where("doc_id = ?", docID).Find(&sourceEntities).Error; err != nil {
		return nil, err
	}

	if len(sourceEntities) == 0 {
		return nil, nil
	}

	entityIDs := make([]int, len(sourceEntities))
	for i, e := range sourceEntities {
		entityIDs[i] = e.ID
	}

	var relations []knowledgemodel.KnowledgeRelation
	if err := r.db.Where("source_entity_id IN ? OR target_entity_id IN ?", entityIDs, entityIDs).
		Find(&relations).Error; err != nil {
		return nil, err
	}

	relatedEntityIDs := make(map[int]bool)
	for _, rel := range relations {
		if !containsInt(entityIDs, rel.SourceEntityID) {
			relatedEntityIDs[rel.SourceEntityID] = true
		}
		if !containsInt(entityIDs, rel.TargetEntityID) {
			relatedEntityIDs[rel.TargetEntityID] = true
		}
	}

	if len(relatedEntityIDs) == 0 {
		return nil, nil
	}

	relatedIDs := make([]int, 0, len(relatedEntityIDs))
	for id := range relatedEntityIDs {
		relatedIDs = append(relatedIDs, id)
	}

	var relatedEntities []knowledgemodel.KnowledgeEntity
	if err := r.db.Where("id IN ?", relatedIDs).Find(&relatedEntities).Error; err != nil {
		return nil, err
	}

	result := make([]RelatedEntity, 0, len(relatedEntities))
	for _, e := range relatedEntities {
		for _, rel := range relations {
			if rel.SourceEntityID == e.ID || rel.TargetEntityID == e.ID {
				result = append(result, RelatedEntity{
					DocID:        e.DocID,
					EntityName:   e.Name,
					RelationType: rel.RelationType,
					Confidence:   rel.Confidence,
				})
			}
		}
	}

	return result, nil
}

func (r *KnowledgeGraphRepository) DeleteEntitiesByDocID(docID int) error {
	return r.db.Where("doc_id = ?", docID).Delete(&knowledgemodel.KnowledgeEntity{}).Error
}

func (r *KnowledgeGraphRepository) BatchCreateEntities(entities []knowledgemodel.KnowledgeEntity) error {
	if len(entities) == 0 {
		return nil
	}
	return r.db.CreateInBatches(entities, 50).Error
}

func (r *KnowledgeGraphRepository) BatchCreateRelations(relations []knowledgemodel.KnowledgeRelation) error {
	if len(relations) == 0 {
		return nil
	}
	return r.db.CreateInBatches(relations, 50).Error
}

func (r *KnowledgeGraphRepository) FindRelationsBySpaceID(spaceID int) ([]knowledgemodel.KnowledgeRelation, error) {
	var entityIDs []int
	if err := r.db.Model(&knowledgemodel.KnowledgeEntity{}).Where("space_id = ?", spaceID).Pluck("id", &entityIDs).Error; err != nil {
		return nil, err
	}
	if len(entityIDs) == 0 {
		return nil, nil
	}
	var relations []knowledgemodel.KnowledgeRelation
	if err := r.db.Where("source_entity_id IN ? OR target_entity_id IN ?", entityIDs, entityIDs).Find(&relations).Error; err != nil {
		return nil, err
	}
	return relations, nil
}

func (r *KnowledgeGraphRepository) FindAllEntities() ([]knowledgemodel.KnowledgeEntity, error) {
	var entities []knowledgemodel.KnowledgeEntity
	err := r.db.Find(&entities).Error
	return entities, err
}

func (r *KnowledgeGraphRepository) FindAllRelations() ([]knowledgemodel.KnowledgeRelation, error) {
	var relations []knowledgemodel.KnowledgeRelation
	err := r.db.Find(&relations).Error
	return relations, err
}

func containsInt(slice []int, item int) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
