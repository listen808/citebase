package knowledge

import (
	"fmt"

	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	"github.com/listen808/citebase/internal/repository"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
)

type KnowledgeGraphService struct {
	graphRepo *knowledgerepo.KnowledgeGraphRepository
}

func NewKnowledgeGraphService() *KnowledgeGraphService {
	return &KnowledgeGraphService{
		graphRepo: knowledgerepo.NewKnowledgeGraphRepository(repository.DB),
	}
}

type GraphResult struct {
	Entities  []knowledgemodel.KnowledgeEntity   `json:"entities"`
	Relations []knowledgemodel.KnowledgeRelation `json:"relations"`
}

func (s *KnowledgeGraphService) GetGraphBySpaceID(spaceID int) (*GraphResult, error) {
	entities, err := s.graphRepo.FindEntitiesBySpaceID(spaceID)
	if err != nil {
		return nil, fmt.Errorf("query entities: %w", err)
	}
	relations, err := s.graphRepo.FindRelationsBySpaceID(spaceID)
	if err != nil {
		return nil, fmt.Errorf("query relations: %w", err)
	}
	return &GraphResult{Entities: entities, Relations: relations}, nil
}

func (s *KnowledgeGraphService) ExtractAndSave(docID, spaceID int, entities []knowledgemodel.KnowledgeEntity, relations []knowledgemodel.KnowledgeRelation) error {
	if err := s.graphRepo.DeleteEntitiesByDocID(docID); err != nil {
		return fmt.Errorf("clear old entities: %w", err)
	}
	if len(entities) > 0 {
		for i := range entities {
			entities[i].DocID = docID
			entities[i].SpaceID = spaceID
		}
		if err := s.graphRepo.BatchCreateEntities(entities); err != nil {
			return fmt.Errorf("save entities: %w", err)
		}
	}
	if len(relations) > 0 {
		if err := s.graphRepo.BatchCreateRelations(relations); err != nil {
			return fmt.Errorf("save relations: %w", err)
		}
	}
	return nil
}
