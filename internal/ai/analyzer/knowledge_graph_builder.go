package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/listen808/citebase/internal/log"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

// KnowledgeGraphBuilder 知识图谱构建器 - 从文本自动构建知识图谱
type KnowledgeGraphBuilder struct {
	provider provider.Provider
}

// NewKnowledgeGraphBuilder 创建知识图谱构建器
func NewKnowledgeGraphBuilder(p provider.Provider) *KnowledgeGraphBuilder {
	return &KnowledgeGraphBuilder{provider: p}
}

// KGEntity 知识图谱实体
type KGEntity struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Type        string            `json:"type"` // person/organization/technology/concept/document/event
	Properties  map[string]string `json:"properties"`
	Description string            `json:"description"`
}

// KGRelation 知识图谱关系
type KGRelation struct {
	SourceID   string            `json:"source_id"`
	TargetID   string            `json:"target_id"`
	Type       string            `json:"type"` // belongs_to/depends_on/uses/implements/related_to
	Properties map[string]string `json:"properties"`
	Weight     float64           `json:"weight"`
}

// KnowledgeGraph 知识图谱
type KnowledgeGraph struct {
	Entities  []KGEntity             `json:"entities"`
	Relations []KGRelation           `json:"relations"`
	Metadata  map[string]interface{} `json:"metadata"`
}

// BuildKnowledgeGraph 从文本构建知识图谱
func (kgb *KnowledgeGraphBuilder) BuildKnowledgeGraph(ctx context.Context, content string) (*KnowledgeGraph, error) {
	if kgb.provider == nil {
		return nil, fmt.Errorf("provider not configured")
	}

	if len(content) > 8000 {
		content = utils.TruncateRunes(content, 8000)
	}

	prompt := `请从以下文本中构建知识图谱，提取实体和它们之间的关系。

实体类型包括：
- person: 人物
- organization: 组织/公司/团队
- technology: 技术/工具/框架
- concept: 概念/术语
- document: 文档/资料
- event: 事件/活动
- system: 系统/平台
- process: 流程/方法

关系类型包括：
- belongs_to: 属于（如：员工属于部门）
- depends_on: 依赖（如：模块依赖其他模块）
- uses: 使用（如：应用使用技术栈）
- implements: 实现（如：类实现接口）
- related_to: 相关（如：概念相关）
- part_of: 部分（如：组件是系统的一部分）
- causes: 导致（如：事件导致结果）
- requires: 需要（如：任务需要资源）

文本内容：
%s

请以JSON格式输出知识图谱：
{
  "entities": [
    {
      "id": "entity_1",
      "name": "实体名称",
      "type": "实体类型",
      "properties": {
        "key": "value"
      },
      "description": "实体描述"
    }
  ],
  "relations": [
    {
      "source_id": "entity_1",
      "target_id": "entity_2",
      "type": "关系类型",
      "properties": {
        "key": "value"
      },
      "weight": 0.9
    }
  ],
  "metadata": {
    "entity_count": 10,
    "relation_count": 5,
    "graph_type": "knowledge"
  }
}

只输出JSON，不要其他内容。`

	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()

	resp, err := kgb.provider.Chat(ctx, "你是一个知识图谱构建专家，擅长从文本中提取实体和关系并构建图谱。只输出JSON。", []provider.Message{
		provider.NewTextMessage("user", fmt.Sprintf(prompt, content)),
	}, provider.WithMaxTokens(4096), provider.WithTemperature(0.1))
	if err != nil {
		log.Errorf("[KnowledgeGraphBuilder] LLM build error: %v", err)
		return nil, err
	}

	graph := &KnowledgeGraph{
		Entities:  []KGEntity{},
		Relations: []KGRelation{},
		Metadata:  map[string]interface{}{},
	}

	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), graph); err != nil {
		log.Errorf("[KnowledgeGraphBuilder] JSON parse error: %v", err)
		return nil, err
	}

	return graph, nil
}

// MergeGraphs 合并多个知识图谱
func (kgb *KnowledgeGraphBuilder) MergeGraphs(graphs []*KnowledgeGraph) *KnowledgeGraph {
	if len(graphs) == 0 {
		return &KnowledgeGraph{
			Entities:  []KGEntity{},
			Relations: []KGRelation{},
			Metadata:  map[string]interface{}{},
		}
	}

	entityMap := make(map[string]KGEntity)
	relationMap := make(map[string]KGRelation)

	for _, graph := range graphs {
		// 合并实体（去重）
		for _, entity := range graph.Entities {
			if existing, exists := entityMap[entity.ID]; exists {
				// 合并属性
				for k, v := range entity.Properties {
					if _, ok := existing.Properties[k]; !ok {
						existing.Properties[k] = v
					}
				}
				entityMap[entity.ID] = existing
			} else {
				entityMap[entity.ID] = entity
			}
		}

		// 合并关系（去重）
		for _, relation := range graph.Relations {
			key := fmt.Sprintf("%s-%s-%s", relation.SourceID, relation.TargetID, relation.Type)
			if existing, exists := relationMap[key]; exists {
				// 取更高的权重
				if relation.Weight > existing.Weight {
					relationMap[key] = relation
				}
			} else {
				relationMap[key] = relation
			}
		}
	}

	// 转换为数组
	entities := make([]KGEntity, 0, len(entityMap))
	for _, entity := range entityMap {
		entities = append(entities, entity)
	}

	relations := make([]KGRelation, 0, len(relationMap))
	for _, relation := range relationMap {
		relations = append(relations, relation)
	}

	return &KnowledgeGraph{
		Entities:  entities,
		Relations: relations,
		Metadata: map[string]interface{}{
			"entity_count":   len(entities),
			"relation_count": len(relations),
			"merged_from":    len(graphs),
		},
	}
}

// ExportToJSON 导出知识图谱为JSON
func (kg *KnowledgeGraph) ExportToJSON() (string, error) {
	data, err := json.MarshalIndent(kg, "", "  ")
	if err != nil {
		return "", err
	}
	return string(data), nil
}
