package text2sql

import (
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"sort"
	"strings"

	"github.com/listen808/citebase/internal/ai/embedding"
	"github.com/listen808/citebase/internal/ai/shared"
	modelai "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

// SQLRAGAdapter 封装 SQL Schema 的 RAG 检索能力
type SQLRAGAdapter struct {
	embedder     embedding.Embedder
	schemaVS     *embedding.SchemaVectorStore
	chunker      *SQLSchemaChunker
	text2sqlRepo *airepo.TextToSQLRepo
}

// NewSQLRAGAdapter 创建 SQL RAG 适配器
func NewSQLRAGAdapter(emb embedding.Embedder, schemaVS *embedding.SchemaVectorStore, repo *airepo.TextToSQLRepo) *SQLRAGAdapter {
	return &SQLRAGAdapter{
		embedder:     emb,
		schemaVS:     schemaVS,
		chunker:      NewSQLSchemaChunker(),
		text2sqlRepo: repo,
	}
}

// RAGSchemaSource RAG 检索到的表结构来源
type RAGSchemaSource struct {
	TableName string  `json:"table_name"`
	Score     float64 `json:"score"`
	RichText  string  `json:"rich_text"`
}

// RetrieveRelevantSchemas 通过 RAG 检索与问题相关的表结构
// 融合结构化检索（关键词匹配）与向量语义检索，使用 RRF 融合排序
func (a *SQLRAGAdapter) RetrieveRelevantSchemas(question string, spaceID int, limit int) []RAGSchemaSource {
	if limit <= 0 || limit > 20 {
		limit = 10
	}

	// 1. 结构化检索（关键词匹配表名/注释/字段名）
	structuredSources := a.structuredRetrieve(question, limit)

	// 2. 向量语义检索
	var vectorSources []RAGSchemaSource
	if a.schemaVS != nil {
		vectorSources = a.vectorRetrieve(question, limit)
	}

	// 3. RRF 融合
	merged := a.rrfMerge(structuredSources, vectorSources, limit)

	logx.Infof("[SQLRAG] Retrieved %d relevant schemas (structured=%d, vector=%d) for: %.50s",
		len(merged), len(structuredSources), len(vectorSources), question)
	return merged
}

// structuredRetrieve 结构化检索（关键词匹配表名/注释/字段名，使用 TableCache 数据）
func (a *SQLRAGAdapter) structuredRetrieve(question string, limit int) []RAGSchemaSource {
	if a.text2sqlRepo == nil {
		return nil
	}

	var sources []RAGSchemaSource
	seenTables := make(map[string]bool)

	// 从问题中提取关键词（简单启发式）
	words := strings.Fields(strings.ToLower(question))

	// 查询所有表缓存，看是否有匹配的
	tables, err := a.text2sqlRepo.GetAllTableCaches()
	if err != nil {
		logx.Infof("[SQLRAG] structured retrieve error: %v", err)
		return nil
	}

	for _, table := range tables {
		score := 0.0
		tableNameLower := strings.ToLower(table.TableNam)
		tableCommentLower := strings.ToLower(table.TableComment)

		// 表名匹配
		for _, word := range words {
			if word == tableNameLower {
				score += 1.0
			} else if strings.Contains(tableNameLower, word) || strings.Contains(word, tableNameLower) {
				score += 0.5
			}
			if strings.Contains(tableCommentLower, word) {
				score += 0.3
			}
		}

		if score > 0 {
			seenTables[table.TableNam] = true
			sources = append(sources, RAGSchemaSource{
				TableName: table.TableNam,
				Score:     score,
			})
		}
	}

	// 按字段名匹配补充表
	for _, word := range words {
		for _, table := range tables {
			if seenTables[table.TableNam] {
				continue
			}
			for _, col := range table.ColumnsJSON {
				if strings.EqualFold(col.Name, word) {
					seenTables[table.TableNam] = true
					sources = append(sources, RAGSchemaSource{
						TableName: table.TableNam,
						Score:     0.4,
					})
					break
				}
			}
		}
	}

	// 排序取 Top
	sort.Slice(sources, func(i, j int) bool {
		return sources[i].Score > sources[j].Score
	})
	if len(sources) > limit {
		sources = sources[:limit]
	}

	return sources
}

// vectorRetrieve 向量语义检索
func (a *SQLRAGAdapter) vectorRetrieve(question string, limit int) []RAGSchemaSource {
	schemaResults, err := a.schemaVS.Search(question, limit*2)
	if err != nil {
		logx.Infof("[SQLRAG] Schema vector search error: %v", err)
		return nil
	}

	var sources []RAGSchemaSource
	for _, r := range schemaResults {
		if r.Score < 0.3 {
			continue
		}
		sources = append(sources, RAGSchemaSource{
			TableName: r.Name,
			Score:     r.Score,
		})
	}

	return sources
}

// rrfMerge RRF 结果融合（k=60）
func (a *SQLRAGAdapter) rrfMerge(listA, listB []RAGSchemaSource, limit int) []RAGSchemaSource {
	itemsA := make([]shared.RRFItem, len(listA))
	for i, item := range listA {
		itemsA[i] = shared.RRFItem{Key: item.TableName, Score: item.Score, Data: item}
	}
	itemsB := make([]shared.RRFItem, len(listB))
	for i, item := range listB {
		itemsB[i] = shared.RRFItem{Key: item.TableName, Score: item.Score, Data: item}
	}

	merged := shared.RRFMerge([][]shared.RRFItem{itemsA, itemsB}, 60.0, limit)
	result := make([]RAGSchemaSource, len(merged))
	for i, item := range merged {
		result[i] = item.Data.(RAGSchemaSource)
	}
	return result
}

// BuildSchemaContext 构建 Schema 上下文，优先使用 RAG 检索到的表
// 如果 RAG 结果为空或表数量少于阈值，则使用全量表缓存
func (a *SQLRAGAdapter) BuildSchemaContext(kb *KnowledgeBase, ragSources []RAGSchemaSource, question string) string {
	allTables := kb.GetAllTables()

	// 如果表数量不多（<20），直接使用全量 schema
	if len(allTables) <= 20 {
		return a.buildFullSchemaContext(kb)
	}

	// 使用 RAG 检索结果构建精简上下文
	if len(ragSources) > 0 {
		return a.buildRAGSchemaContext(kb, ragSources)
	}

	// 回退到全量 schema
	return a.buildFullSchemaContext(kb)
}

// buildFullSchemaContext 构建全量 Schema 上下文（含硬长度限制）
func (a *SQLRAGAdapter) buildFullSchemaContext(kb *KnowledgeBase) string {
	var parts []string

	tables := kb.GetAllTables()
	if len(tables) == 0 {
		return "暂无表结构信息"
	}

	parts = append(parts, "### 表结构\n")

	maxTotalLen := 55000
	var totalLen int

	for name, cache := range tables {
		alias := suggestAlias(name)
		comment := ""
		if cache.TableComment != "" {
			comment = fmt.Sprintf(" (%s)", cache.TableComment)
		}
		rowHint := ""
		if cache.RowCountEstimate > 0 {
			rowHint = fmt.Sprintf(" [约%d行]", cache.RowCountEstimate)
		}
		tableBlock := fmt.Sprintf("#### %s%s%s\n别名: %s", name, comment, rowHint, alias)
		tableBlock += "\n"

		// 字段列表
		if len(cache.ColumnsJSON) > 0 {
			tableBlock += "| 字段名 | 类型 | 约束 | 注释 |\n|--------|------|------|------|\n"
			for _, col := range cache.ColumnsJSON {
				var constraints []string
				if col.IsPK {
					constraints = append(constraints, "PK")
				}
				if col.IsAutoInc {
					constraints = append(constraints, "AUTO_INC")
				}
				if !col.IsNullable {
					constraints = append(constraints, "NOT NULL")
				}
				constraintStr := strings.Join(constraints, ", ")
				comment := col.Comment
				if comment == "" {
					comment = "-"
				}
				colType := classifyFieldType(col.Name, col.Type)
				typeHint := ""
				if colType != "" {
					typeHint = fmt.Sprintf(" [%s]", colType)
				}
				tableBlock += fmt.Sprintf("| `%s` | %s%s | %s | %s |\n", col.Name, col.Type, typeHint, constraintStr, comment)
			}
		}

		potentialLen := len(tableBlock) + 50
		if totalLen+potentialLen > maxTotalLen {
			break
		}
		parts = append(parts, tableBlock)
		totalLen += potentialLen
	}

	// 关联关系
	relations := kb.GetAllRelations()
	if len(relations) > 0 {
		relBlock := "### 关联关系\n| 源表 | 源字段 | 目标表 | 目标字段 | 类型 |\n|------|--------|--------|--------|------|\n"
		relCount := 0
		for _, rel := range relations {
			relLine := fmt.Sprintf("| %s | %s | %s | %s | %s |\n",
				rel.SourceTable, rel.SourceField,
				rel.TargetTable, rel.TargetField,
				rel.RelationType)
			if totalLen+len(relBlock)+len(relLine) > maxTotalLen {
				break
			}
			relBlock += relLine
			relCount++
		}
		if relCount > 0 {
			parts = append(parts, relBlock)
		}
	}

	return strings.Join(parts, "\n")
}

// buildRAGSchemaContext 基于 RAG 检索结果构建精简 Schema 上下文
func (a *SQLRAGAdapter) buildRAGSchemaContext(kb *KnowledgeBase, ragSources []RAGSchemaSource) string {
	var parts []string

	parts = append(parts, "### 相关表结构（基于语义检索）\n")

	for _, src := range ragSources {
		table := kb.GetTable(src.TableName)
		if table == nil {
			continue
		}

		comment := ""
		if table.TableComment != "" {
			comment = fmt.Sprintf(" (%s)", table.TableComment)
		}
		parts = append(parts, fmt.Sprintf("#### %s%s [相关度: %.2f]", src.TableName, comment, src.Score))
		parts = append(parts, "")

		// 字段列表
		if len(table.ColumnsJSON) > 0 {
			parts = append(parts, "| 字段名 | 类型 | 约束 | 注释 |")
			parts = append(parts, "|--------|------|------|------|")
			for _, col := range table.ColumnsJSON {
				var constraints []string
				if col.IsPK {
					constraints = append(constraints, "PK")
				}
				if col.IsAutoInc {
					constraints = append(constraints, "AUTO_INC")
				}
				if !col.IsNullable {
					constraints = append(constraints, "NOT NULL")
				}
				constraintStr := strings.Join(constraints, ", ")
				comment := col.Comment
				if comment == "" {
					comment = "-"
				}
				colType := classifyFieldType(col.Name, col.Type)
				typeHint := ""
				if colType != "" {
					typeHint = fmt.Sprintf(" [%s]", colType)
				}
				parts = append(parts, fmt.Sprintf("| `%s` | %s%s | %s | %s |", col.Name, col.Type, typeHint, constraintStr, comment))
			}
			parts = append(parts, "")
		}

		// 当前表的关联关系
		relations := kb.GetRelations(src.TableName)
		if len(relations) > 0 {
			parts = append(parts, "关联关系:")
			for _, rel := range relations {
				parts = append(parts, fmt.Sprintf("- %s.%s → %s.%s (%s)",
					rel.SourceTable, rel.SourceField,
					rel.TargetTable, rel.TargetField,
					rel.RelationType))
			}
			parts = append(parts, "")
		}
	}

	// 补充业务规则
	rules := a.getRelevantRules(kb, ragSources)
	if len(rules) > 0 {
		parts = append(parts, "### 业务规则\n")
		for _, rule := range rules {
			parts = append(parts, fmt.Sprintf("- [%s] %s: %s", rule.Type, rule.Name, rule.Content))
		}
		parts = append(parts, "")
	}

	return strings.Join(parts, "\n")
}

// getRelevantRules 获取与 RAG 检索表相关的业务规则
func (a *SQLRAGAdapter) getRelevantRules(kb *KnowledgeBase, ragSources []RAGSchemaSource) []modelai.BusinessRule {
	var rules []modelai.BusinessRule
	seen := make(map[uint64]bool)

	for _, src := range ragSources {
		tableRules := kb.GetRules(src.TableName)
		for _, rule := range tableRules {
			if !seen[rule.ID] {
				seen[rule.ID] = true
				rules = append(rules, rule)
			}
		}
	}

	return rules
}

// suggestAlias 根据表名生成推荐的别名
func suggestAlias(tableName string) string {
	name := strings.ToLower(tableName)
	// 取前缀
	if idx := strings.Index(name, "_"); idx > 0 {
		return name[:idx]
	}
	// 取首字母
	if len(name) > 0 {
		return name[:1]
	}
	return "t"
}

// classifyFieldType 根据字段名和类型推断语义类别
func classifyFieldType(name, sqlType string) string {
	lower := strings.ToLower(name)
	typeLower := strings.ToLower(sqlType)

	if strings.Contains(typeLower, "date") || strings.Contains(typeLower, "time") {
		return "date"
	}
	if strings.Contains(typeLower, "int") || strings.Contains(typeLower, "float") ||
		strings.Contains(typeLower, "decimal") || strings.Contains(typeLower, "numeric") {
		if strings.Contains(lower, "id") || strings.Contains(lower, "code") {
			return "identifier"
		}
		if strings.Contains(lower, "qty") || strings.Contains(lower, "amt") ||
			strings.Contains(lower, "amount") || strings.Contains(lower, "price") ||
			strings.Contains(lower, "count") || strings.Contains(lower, "total") ||
			strings.Contains(lower, "rate") || strings.Contains(lower, "percent") {
			return "measure"
		}
		return "numeric"
	}
	if strings.Contains(typeLower, "char") || strings.Contains(typeLower, "text") {
		if strings.Contains(lower, "name") {
			return "name"
		}
		if strings.Contains(lower, "desc") || strings.Contains(lower, "remark") ||
			strings.Contains(lower, "comment") || strings.Contains(lower, "note") {
			return "description"
		}
		if strings.Contains(lower, "addr") || strings.Contains(lower, "address") {
			return "address"
		}
		if strings.Contains(lower, "email") || strings.Contains(lower, "phone") ||
			strings.Contains(lower, "mobile") || strings.Contains(lower, "url") {
			return "contact"
		}
		if strings.Contains(lower, "flag") || strings.Contains(lower, "status") ||
			strings.Contains(lower, "type") || strings.Contains(lower, "category") {
			return "enum"
		}
		return "text"
	}
	return ""
}
