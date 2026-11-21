package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
	repos "github.com/listen808/citebase/internal/repository"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
	"github.com/listen808/citebase/internal/utils"
)

type GraphEnhancer struct {
	provider  provider.Provider
	graphRepo *knowledgerepo.KnowledgeGraphRepository
}

func NewGraphEnhancer(p provider.Provider) *GraphEnhancer {
	return &GraphEnhancer{
		provider:  p,
		graphRepo: knowledgerepo.NewKnowledgeGraphRepository(repos.DB),
	}
}

type EntityCluster struct {
	ClusterID   int    `json:"cluster_id"`
	Name        string `json:"name"`
	Description string `json:"description"`
	EntityIDs   []int  `json:"entity_ids"`
	Centroid    string `json:"centroid"`
}

type SuggestedRelation struct {
	SourceEntityID int     `json:"source_entity_id"`
	TargetEntityID int     `json:"target_entity_id"`
	RelationType   string  `json:"relation_type"`
	Description    string  `json:"description"`
	Confidence     float64 `json:"confidence"`
}

func (g *GraphEnhancer) ClusterEntities(ctx context.Context, spaceID int) ([]EntityCluster, error) {
	if g.provider == nil || g.graphRepo == nil {
		return nil, fmt.Errorf("graph enhancer not initialized")
	}

	entities, err := g.graphRepo.FindEntitiesBySpaceID(spaceID)
	if err != nil {
		return nil, fmt.Errorf("find entities: %w", err)
	}

	if len(entities) < 2 {
		return nil, nil
	}

	var entityDescs []string
	for _, e := range entities {
		entityDescs = append(entityDescs, fmt.Sprintf("ID:%d 名称:%s 类型:%s 描述:%s", e.ID, e.Name, e.Type, e.Description))
	}

	prompt := `请对以下实体进行聚类分析，将相关实体分组。

实体列表：
` + strings.Join(entityDescs, "\n") + `

输出JSON数组格式：
[{
  "cluster_id": 1,
  "name": "聚类名称",
  "description": "聚类描述",
  "entity_ids": [1, 2, 3],
  "centroid": "聚类中心描述"
}]`

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := g.provider.Chat(ctx, "你是一个知识图谱分析专家。只输出JSON。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(2048), provider.WithTemperature(0.2))
	if err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}

	var clusters []EntityCluster
	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), &clusters); err != nil {
		return nil, fmt.Errorf("parse clusters: %w", err)
	}

	return clusters, nil
}

func (g *GraphEnhancer) SuggestNewRelations(ctx context.Context, spaceID int) ([]SuggestedRelation, error) {
	if g.provider == nil || g.graphRepo == nil {
		return nil, fmt.Errorf("graph enhancer not initialized")
	}

	entities, err := g.graphRepo.FindEntitiesBySpaceID(spaceID)
	if err != nil {
		return nil, fmt.Errorf("find entities: %w", err)
	}

	relations, err := g.graphRepo.FindRelationsBySpaceID(spaceID)
	if err != nil {
		return nil, fmt.Errorf("find relations: %w", err)
	}

	existingRelations := make(map[string]bool)
	for _, r := range relations {
		key := fmt.Sprintf("%d-%d", r.SourceEntityID, r.TargetEntityID)
		existingRelations[key] = true
	}

	var entityPairs []string
	entityMap := make(map[int]string)
	for _, e := range entities {
		entityMap[e.ID] = e.Name
	}

	limit := 20
	if len(entities) < limit {
		limit = len(entities)
	}

	for i := 0; i < limit; i++ {
		for j := i + 1; j < limit; j++ {
			key1 := fmt.Sprintf("%d-%d", entities[i].ID, entities[j].ID)
			key2 := fmt.Sprintf("%d-%d", entities[j].ID, entities[i].ID)
			if !existingRelations[key1] && !existingRelations[key2] {
				entityPairs = append(entityPairs, fmt.Sprintf("%s (ID:%d) <-> %s (ID:%d)",
					entities[i].Name, entities[i].ID, entities[j].Name, entities[j].ID))
			}
		}
	}

	if len(entityPairs) == 0 {
		return nil, nil
	}

	prompt := `基于以下实体对，建议可能存在但尚未建立的关系。

实体对：
` + strings.Join(entityPairs, "\n") + `

输出JSON数组：
[{
  "source_entity_id": 1,
  "target_entity_id": 2,
  "relation_type": "related_to",
  "description": "关系描述",
  "confidence": 0.8
}]`

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := g.provider.Chat(ctx, "你是一个知识图谱构建专家。只输出JSON。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(1024), provider.WithTemperature(0.3))
	if err != nil {
		return nil, fmt.Errorf("llm call: %w", err)
	}

	var suggested []SuggestedRelation
	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), &suggested); err != nil {
		return nil, fmt.Errorf("parse suggestions: %w", err)
	}

	return suggested, nil
}
