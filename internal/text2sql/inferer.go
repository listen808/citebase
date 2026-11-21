package text2sql

import (
	modelai "github.com/listen808/citebase/internal/model/ai"
)

type RelationInferer struct {
	kb *KnowledgeBase
}

func NewRelationInferer(kb *KnowledgeBase) *RelationInferer {
	return &RelationInferer{kb: kb}
}

type JoinPath struct {
	SourceTable string
	SourceField string
	TargetTable string
	TargetField string
	JoinType    string
}

func (ri *RelationInferer) InferJoins(tables []string) []JoinPath {
	if len(tables) <= 1 {
		return nil
	}

	var joins []JoinPath
	visited := make(map[string]bool)

	for i := 0; i < len(tables)-1; i++ {
		path := ri.findShortestPath(tables[i], tables[i+1])
		if path != nil {
			for _, rel := range path {
				if !visited[rel.SourceTable+"."+rel.TargetTable] {
					joins = append(joins, JoinPath{
						SourceTable: rel.SourceTable,
						SourceField: rel.SourceField,
						TargetTable: rel.TargetTable,
						TargetField: rel.TargetField,
						JoinType:    "INNER JOIN",
					})
					visited[rel.SourceTable+"."+rel.TargetTable] = true
				}
			}
		}
	}

	return joins
}

func (ri *RelationInferer) findShortestPath(source, target string) []modelai.TableRelation {
	if source == target {
		return nil
	}

	queue := [][]modelai.TableRelation{{}}
	visited := make(map[string]bool)
	visited[source] = true

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		lastTable := source
		if len(current) > 0 {
			lastTable = current[len(current)-1].TargetTable
		}

		relations := ri.kb.GetRelations(lastTable)
		for _, rel := range relations {
			if visited[rel.TargetTable] {
				continue
			}

			newPath := make([]modelai.TableRelation, len(current)+1)
			copy(newPath, current)
			newPath[len(current)] = rel

			if rel.TargetTable == target {
				return newPath
			}

			visited[rel.TargetTable] = true
			queue = append(queue, newPath)
		}
	}

	return nil
}

func (ri *RelationInferer) DetectAmbiguity(tables []string) []string {
	var warnings []string

	for _, table := range tables {
		relations := ri.kb.GetRelations(table)
		targetCounts := make(map[string]int)

		for _, rel := range relations {
			targetCounts[rel.TargetTable]++
		}

		for target, count := range targetCounts {
			if count > 1 {
				warnings = append(warnings,
					"表 "+table+" 与 "+target+" 存在多条关联路径，请明确使用哪个字段")
			}
		}
	}

	return warnings
}
