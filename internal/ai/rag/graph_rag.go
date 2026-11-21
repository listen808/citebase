package rag

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/provider"
	repos "github.com/listen808/citebase/internal/repository"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/utils"
)

type KnowledgeEntity struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	DocID       int    `json:"doc_id"`
	SpaceID     int    `json:"space_id"`
	Aliases     string `json:"aliases"`
}

type KnowledgeRelation struct {
	ID             int     `json:"id"`
	SourceEntityID int     `json:"source_entity_id"`
	TargetEntityID int     `json:"target_entity_id"`
	RelationType   string  `json:"relation_type"`
	Description    string  `json:"description"`
	Confidence     float64 `json:"confidence"`
}

// GraphPath represents a single hop or multi-hop path found during graph traversal.
type GraphPath struct {
	FromEntity string  `json:"from_entity"`
	ToEntity   string  `json:"to_entity"`
	Relation   string  `json:"relation"`
	DocIDs     []int   `json:"doc_ids"`   // documents involved in this hop
	HopDepth   int     `json:"hop_depth"` // 0-based depth from source
	Confidence float64 `json:"confidence"`
}

// GraphPathResult holds the complete traversal result with all paths found.
type GraphPathResult struct {
	SourceDocIDs   []int       `json:"source_doc_ids"`
	ExpandedDocIDs []int       `json:"expanded_doc_ids"`
	Paths          []GraphPath `json:"paths"`
	TotalHops      int         `json:"total_hops"`
}

type GraphRAG struct {
	provider   provider.Provider
	graphRepo  *knowledgerepo.KnowledgeGraphRepository
	docRepo    *workspacerepo.DocumentRepository
	spaceRepo  *workspacerepo.SpaceRepository
	docStorage *utils.DocumentStorage
}

func NewGraphRAG(p provider.Provider) *GraphRAG {
	docDir := "./data/markdowns"
	var graphRepo *knowledgerepo.KnowledgeGraphRepository
	if repos.DB != nil {
		graphRepo = knowledgerepo.NewKnowledgeGraphRepository(repos.DB)
	}
	return &GraphRAG{
		provider:   p,
		graphRepo:  graphRepo,
		docRepo:    workspacerepo.NewDocumentRepository(),
		spaceRepo:  workspacerepo.NewSpaceRepository(),
		docStorage: utils.NewDocumentStorageWithDir(docDir),
	}
}

func (g *GraphRAG) ExtractEntitiesAndRelations(ctx context.Context, docID int, content string) ([]KnowledgeEntity, []KnowledgeRelation, provider.TokenUsage, error) {
	if g.provider == nil {
		return nil, nil, provider.TokenUsage{}, fmt.Errorf("AI服务未启用")
	}

	if len(content) > 8000 {
		content = utils.TruncateRunes(content, 8000)
	}

	prompt := fmt.Sprintf(`请从以下文档中提取实体和关系，输出JSON格式。

实体类型包括：concept(概念)、person(人物)、organization(组织)、technology(技术)、product(产品)、event(事件)、metric(指标)

关系类型包括：depends_on(依赖)、related_to(相关)、part_of(部分)、prerequisite_of(前提)、alternative_of(替代)、contradicts(矛盾)

输出格式：
{
  "entities": [{"name": "实体名", "type": "concept", "description": "描述", "aliases": ["别名1"]}],
  "relations": [{"source": "源实体名", "target": "目标实体名", "type": "depends_on", "description": "描述", "confidence": 0.9}]
}

文档内容：
%s`, content)

	resp, err := g.provider.Chat(ctx, "你是一个知识图谱构建助手。请精确提取文档中的实体和关系。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(2048), provider.WithTemperature(0.2))
	if err != nil {
		return nil, nil, provider.TokenUsage{}, fmt.Errorf("LLM extraction error: %w", err)
	}

	trimmed := strings.TrimSpace(resp.Content)
	if idx := strings.Index(trimmed, "{"); idx >= 0 {
		trimmed = trimmed[idx:]
		if last := strings.LastIndex(trimmed, "}"); last >= 0 {
			trimmed = trimmed[:last+1]
		}
	}

	var result struct {
		Entities []struct {
			Name        string   `json:"name"`
			Type        string   `json:"type"`
			Description string   `json:"description"`
			Aliases     []string `json:"aliases"`
		} `json:"entities"`
		Relations []struct {
			Source      string  `json:"source"`
			Target      string  `json:"target"`
			Type        string  `json:"type"`
			Description string  `json:"description"`
			Confidence  float64 `json:"confidence"`
		} `json:"relations"`
	}

	if err := json.Unmarshal([]byte(trimmed), &result); err != nil {
		logx.Infof("[GraphRAG] JSON parse error: %v, raw: %s", err, trimmed[:min(200, len(trimmed))])
		// P3：解析失败应返回 error，让调用方感知并决定降级策略
		return nil, nil, resp.TokenUsage, fmt.Errorf("graph rag parse failed: %w", err)
	}

	entities := make([]KnowledgeEntity, 0, len(result.Entities))
	for i, e := range result.Entities {
		aliases := ""
		if len(e.Aliases) > 0 {
			aliasesBytes, _ := json.Marshal(e.Aliases)
			aliases = string(aliasesBytes)
		}
		entities = append(entities, KnowledgeEntity{
			ID:          i + 1,
			Name:        e.Name,
			Type:        e.Type,
			Description: e.Description,
			DocID:       docID,
			Aliases:     aliases,
		})
	}

	relations := make([]KnowledgeRelation, 0, len(result.Relations))
	for i, r := range result.Relations {
		sourceID := g.findEntityID(entities, r.Source)
		targetID := g.findEntityID(entities, r.Target)
		if sourceID == 0 || targetID == 0 {
			continue
		}
		relations = append(relations, KnowledgeRelation{
			ID:             i + 1,
			SourceEntityID: sourceID,
			TargetEntityID: targetID,
			RelationType:   r.Type,
			Description:    r.Description,
			Confidence:     r.Confidence,
		})
	}

	return entities, relations, resp.TokenUsage, nil
}

func (g *GraphRAG) findEntityID(entities []KnowledgeEntity, name string) int {
	for _, e := range entities {
		if e.Name == name {
			return e.ID
		}
	}
	return 0
}

func (g *GraphRAG) ExpandContextWithGraph(ctx context.Context, docIDs []int, depth int) ([]int, error) {
	if depth <= 0 || len(docIDs) == 0 {
		return docIDs, nil
	}

	relatedDocs := make(map[int]bool)
	for _, id := range docIDs {
		relatedDocs[id] = true
	}

	docIDSet := make(map[int]bool)
	for _, id := range docIDs {
		docIDSet[id] = true
	}

	if g.graphRepo != nil {
		for _, docID := range docIDs {
			related, err := g.graphRepo.FindRelatedEntities(docID, depth)
			if err != nil {
				continue
			}
			for _, r := range related {
				if !docIDSet[r.DocID] {
					relatedDocs[r.DocID] = true
				}
			}
		}
	}

	result := make([]int, 0, len(relatedDocs))
	for id := range relatedDocs {
		result = append(result, id)
	}
	return result, nil
}

// ExpandContextWithGraphPaths performs BFS graph traversal and records every hop path
// with entity names, relation types, and document IDs for citation tracking.
func (g *GraphRAG) ExpandContextWithGraphPaths(ctx context.Context, docIDs []int, depth int) *GraphPathResult {
	result := &GraphPathResult{
		SourceDocIDs: docIDs,
	}

	if depth <= 0 || len(docIDs) == 0 {
		result.ExpandedDocIDs = docIDs
		return result
	}

	visited := make(map[int]bool)
	for _, id := range docIDs {
		visited[id] = true
		result.ExpandedDocIDs = append(result.ExpandedDocIDs, id)
	}

	if g.graphRepo == nil {
		return result
	}

	// BFS frontier: map of docID -> list of entities found at that docID
	type frontierEntry struct {
		docID    int
		entities []struct {
			Name         string
			RelationType string
			DocID        int
			Confidence   float64
		}
	}

	currentFrontier := make([]frontierEntry, 0)
	for _, docID := range docIDs {
		entities, err := g.graphRepo.FindRelatedEntities(docID, 1)
		if err != nil {
			continue
		}
		if len(entities) > 0 {
			entry := frontierEntry{docID: docID}
			for _, e := range entities {
				entry.entities = append(entry.entities, struct {
					Name         string
					RelationType string
					DocID        int
					Confidence   float64
				}{Name: e.EntityName, RelationType: e.RelationType, DocID: e.DocID, Confidence: e.Confidence})
			}
			currentFrontier = append(currentFrontier, entry)
		}
	}

	for hop := 0; hop < depth && len(currentFrontier) > 0; hop++ {
		var nextFrontier []frontierEntry
		for _, fe := range currentFrontier {
			for _, ent := range fe.entities {
				if visited[ent.DocID] {
					continue
				}
				visited[ent.DocID] = true
				result.ExpandedDocIDs = append(result.ExpandedDocIDs, ent.DocID)

				result.Paths = append(result.Paths, GraphPath{
					FromEntity: fmt.Sprintf("doc:%d", fe.docID),
					ToEntity:   fmt.Sprintf("doc:%d", ent.DocID),
					Relation:   ent.RelationType,
					DocIDs:     []int{fe.docID, ent.DocID},
					HopDepth:   hop + 1,
					Confidence: ent.Confidence,
				})

				// Expand next hop
				subEntities, err := g.graphRepo.FindRelatedEntities(ent.DocID, 1)
				if err != nil {
					continue
				}
				if len(subEntities) > 0 {
					subEntry := frontierEntry{docID: ent.DocID}
					for _, se := range subEntities {
						if !visited[se.DocID] {
							subEntry.entities = append(subEntry.entities, struct {
								Name         string
								RelationType string
								DocID        int
								Confidence   float64
							}{Name: se.EntityName, RelationType: se.RelationType, DocID: se.DocID, Confidence: se.Confidence})
						}
					}
					if len(subEntry.entities) > 0 {
						nextFrontier = append(nextFrontier, subEntry)
					}
				}
			}
		}
		currentFrontier = nextFrontier
		result.TotalHops = hop + 1
	}

	return result
}
