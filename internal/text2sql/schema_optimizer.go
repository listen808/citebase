package text2sql

import (
	"math"
	"sort"
	"strconv"
	"strings"

	modelai "github.com/listen808/citebase/internal/model/ai"
)

// SchemaOptimizer Schema优化器，用于精简Schema上下文
// P0增强：基于相关性动态选择字段，层级摘要
type SchemaOptimizer struct {
	MaxTables         int
	MaxContextLength  int
	MaxDDLLength      int
	MaxFieldsPerTable int // 每张表最大字段数
	FieldBudget       int // 字段预算总数
}

// TableScore 表评分结果
type TableScore struct {
	Name  string
	Score float64
}

// ColumnScore 字段评分结果
type ColumnScore struct {
	Name       string
	Type       string
	Comment    string
	Score      float64
	IsPrimary  bool
	IsForeign  bool
	IsSelected bool
}

// NewSchemaOptimizer 创建Schema优化器
func NewSchemaOptimizer() *SchemaOptimizer {
	return &SchemaOptimizer{
		MaxTables:         8,
		MaxContextLength:  20000,
		MaxDDLLength:      4000,
		MaxFieldsPerTable: 25,
		FieldBudget:       100,
	}
}

// NewSchemaOptimizerWithConfig 创建带配置的优化器
func NewSchemaOptimizerWithConfig(maxTables, maxContextLen, maxFieldsPerTable int) *SchemaOptimizer {
	return &SchemaOptimizer{
		MaxTables:         maxTables,
		MaxContextLength:  maxContextLen,
		MaxDDLLength:      4000,
		MaxFieldsPerTable: maxFieldsPerTable,
		FieldBudget:       maxFieldsPerTable * maxTables,
	}
}

// CalculateTableRelevance 计算表与查询的相关性分数
// mentionedTableNames: 用户在查询中显式提到的表名（精确匹配），获得最高优先级
func (o *SchemaOptimizer) CalculateTableRelevance(tableName, tableComment string, keywords []string, mentionedTableNames ...string) float64 {
	score := 0.0
	lowerName := strings.ToLower(tableName)

	// P0增强：用户显式提到的表名获得最高优先级，确保一定进入Top-N
	for _, mt := range mentionedTableNames {
		if strings.EqualFold(tableName, mt) {
			score += 1000.0
			return score
		}
	}

	for _, kw := range keywords {
		kwLower := strings.ToLower(kw)

		// 表名匹配（权重最高）
		if strings.Contains(lowerName, kwLower) {
			score += 10.0
		}

		// P2增强：反向匹配 — 如果关键词包含完整表名，也是强信号
		// 例如用户输入包含 "bm_config_version" 作为完整标识符
		if len(lowerName) > 3 && strings.Contains(kwLower, lowerName) {
			score += 10.0
		}

		// 注释匹配
		if strings.Contains(strings.ToLower(tableComment), kwLower) {
			score += 5.0
		}
	}

	return score
}

// CalculateColumnRelevance 计算字段与查询的相关性分数
// P0增强：更精细的字段评分
func (o *SchemaOptimizer) CalculateColumnRelevance(col modelai.ColumnInfo, keywords []string) float64 {
	score := 0.0
	lowerName := strings.ToLower(col.Name)
	lowerComment := strings.ToLower(col.Comment)

	// 1. 主键字段优先级最高
	if col.IsPK {
		score += 20.0
	}

	// 2. 外键字段（关联查询必需）
	if false || strings.HasSuffix(lowerName, "_id") {
		score += 15.0
	}

	// 3. 关键词匹配
	for _, kw := range keywords {
		kwLower := strings.ToLower(kw)

		// 字段名完全匹配
		if lowerName == kwLower {
			score += 10.0
		}

		// 字段名包含关键词
		if strings.Contains(lowerName, kwLower) {
			score += 5.0
		}

		// 注释包含关键词
		if strings.Contains(lowerComment, kwLower) {
			score += 3.0
		}
	}

	// 4. 常见查询目标类型加分
	selectableTypes := map[string]bool{
		"varchar": true, "text": true, "char": true,
		"int": true, "bigint": true, "decimal": true, "float": true, "double": true,
		"date": true, "datetime": true, "timestamp": true,
	}
	if selectableTypes[strings.ToLower(col.Type)] {
		score += 1.0
	}

	return score
}

// SelectTopTables 选择Top N张表
func (o *SchemaOptimizer) SelectTopTables(tables []TableScore, limit int) []TableScore {
	// 按分数降序排序
	sort.Slice(tables, func(i, j int) bool {
		return tables[i].Score > tables[j].Score
	})

	// 限制数量
	if len(tables) > limit {
		tables = tables[:limit]
	}

	return tables
}

// SelectTopFields 选择Top N个字段（基于相关性）
// P0增强：智能字段选择
func (o *SchemaOptimizer) SelectTopFields(columns []modelai.ColumnInfo, keywords []string, limit int) []modelai.ColumnInfo {
	// 评分
	var scored []ColumnScore
	for _, col := range columns {
		scored = append(scored, ColumnScore{
			Name:       col.Name,
			Type:       col.Type,
			Comment:    col.Comment,
			Score:      o.CalculateColumnRelevance(col, keywords),
			IsPrimary:  col.IsPK,
			IsForeign:  false || strings.HasSuffix(strings.ToLower(col.Name), "_id"),
			IsSelected: false,
		})
	}

	// 按分数排序
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].Score > scored[j].Score
	})

	// 选择Top N（优先保留主键和外键）
	var selected []modelai.ColumnInfo
	selectedSet := make(map[string]bool)

	// 构建 name -> column 映射
	columnByName := make(map[string]modelai.ColumnInfo)
	for _, col := range columns {
		columnByName[col.Name] = col
	}

	// 第一遍：选择主键和外键（必须保留）
	for i := range scored {
		if scored[i].IsPrimary || scored[i].IsForeign {
			if len(selected) < limit {
				if col, ok := columnByName[scored[i].Name]; ok {
					selected = append(selected, col)
				}
				selectedSet[scored[i].Name] = true
			}
		}
	}

	// 第二遍：按分数选择其他字段
	remaining := limit - len(selected)
	for i := range scored {
		if remaining <= 0 {
			break
		}
		if !selectedSet[scored[i].Name] {
			if col, ok := columnByName[scored[i].Name]; ok {
				selected = append(selected, col)
			}
			selectedSet[scored[i].Name] = true
			remaining--
		}
	}

	return selected
}

// BuildCompactContext 构建紧凑的表上下文
func (o *SchemaOptimizer) BuildCompactContext(tableName, tableComment string, columns []string) string {
	var sb strings.Builder

	// 简化的表信息
	sb.WriteString("### ")
	if tableComment != "" {
		sb.WriteString(tableComment)
	} else {
		sb.WriteString(tableName)
	}
	sb.WriteString(" (`" + tableName + "`)")

	// 简化的字段列表
	if len(columns) > 0 {
		sb.WriteString("\n字段: ")
		sb.WriteString(strings.Join(columns, ", "))
	}

	return sb.String()
}

// BuildOptimizedContext 构建优化后的Schema上下文
// mentionedTableNames: 用户在查询中显式提到的表名，强制纳入上下文
func (o *SchemaOptimizer) BuildOptimizedContext(query string, tables map[string]*modelai.TableCache, keywords []string, mentionedTableNames ...string) string {
	// 计算每张表的相关性分数
	var tableScores []TableScore
	for name, cache := range tables {
		score := o.CalculateTableRelevance(name, cache.TableComment, keywords, mentionedTableNames...)
		tableScores = append(tableScores, TableScore{Name: name, Score: score})
	}

	// 选择Top N张表
	selectedTables := o.SelectTopTables(tableScores, o.MaxTables)

	// 构建精简上下文
	return o.buildOptimizedSchemaContext(query, selectedTables, tables, keywords)
}

// buildOptimizedSchemaContext 构建优化后的Schema上下文
func (o *SchemaOptimizer) buildOptimizedSchemaContext(query string, selectedTables []TableScore, tables map[string]*modelai.TableCache, keywords []string) string {
	var parts []string
	parts = append(parts, "以下是与当前问题相关的数据库表结构信息：\n")

	totalLen := len(parts[0])
	remainingBudget := o.FieldBudget

	for _, ts := range selectedTables {
		cache := tables[ts.Name]
		if cache == nil {
			continue
		}

		// 构建精简的表上下文（使用智能字段选择）
		fieldLimit := o.MaxFieldsPerTable
		if remainingBudget < fieldLimit {
			fieldLimit = remainingBudget
		}

		tableContext := o.buildCompactTableContextWithFieldSelection(ts.Name, cache, keywords, fieldLimit)
		contextLen := len(tableContext)

		// 检查长度限制
		if totalLen+contextLen > o.MaxContextLength || remainingBudget <= 0 {
			break
		}

		parts = append(parts, tableContext)
		totalLen += contextLen
		remainingBudget -= fieldLimit
	}

	return strings.Join(parts, "\n---\n")
}

// buildCompactTableContextWithFieldSelection 构建带字段选择的精简表上下文
// P0增强：只包含相关字段
func (o *SchemaOptimizer) buildCompactTableContextWithFieldSelection(tableName string, cache *modelai.TableCache, keywords []string, fieldLimit int) string {
	var sb strings.Builder

	// 表标题
	sb.WriteString("### ")
	if cache.TableComment != "" {
		sb.WriteString(cache.TableComment)
	} else {
		sb.WriteString(tableName)
	}
	sb.WriteString(" (`" + tableName + "`)\n")

	// 如果有DDL，添加精简版
	if cache.DDLContent != "" {
		ddl := cache.DDLContent
		if len(ddl) > o.MaxDDLLength {
			ddl = ddl[:o.MaxDDLLength] + "\n-- ... (truncated)"
		}
		sb.WriteString("```sql\n" + ddl + "\n```\n")
	}

	// 智能选择关键字段
	if len(cache.ColumnsJSON) > 0 {
		selectedFields := o.SelectTopFields(cache.ColumnsJSON, keywords, fieldLimit)

		sb.WriteString("**关键字段**:\n")
		for _, col := range selectedFields {
			// 添加主键/外键标记
			marker := ""
			if col.IsPK {
				marker = " [PK]"
			} else if false || strings.HasSuffix(strings.ToLower(col.Name), "_id") {
				marker = " [FK]"
			}
			sb.WriteString("- `" + col.Name + "` (" + col.Type + ")" + marker + ": " + col.Comment + "\n")
		}

		// 如果字段被截断，提示总数
		if len(cache.ColumnsJSON) > fieldLimit {
			sb.WriteString("- ... (共 " + strconv.Itoa(len(cache.ColumnsJSON)) + " 个字段，显示前 " + strconv.Itoa(fieldLimit) + " 个)\n")
		}
	}

	return sb.String()
}

// buildCompactTableContext 构建单张表的精简上下文（兼容旧接口）
func (o *SchemaOptimizer) buildCompactTableContext(tableName string, cache *modelai.TableCache, keywords []string) string {
	return o.buildCompactTableContextWithFieldSelection(tableName, cache, keywords, o.MaxFieldsPerTable)
}

// BuildHierarchicalSummary 构建层级摘要
// P0增强：对长Schema进行层级摘要
func (o *SchemaOptimizer) BuildHierarchicalSummary(tables map[string]*modelai.TableCache, keywords []string) string {
	// 第一层：计算每张表的整体相关度
	var tableScores []TableScore
	for name, cache := range tables {
		score := o.CalculateTableRelevance(name, cache.TableComment, keywords)
		tableScores = append(tableScores, TableScore{Name: name, Score: score})
	}

	// 按分数排序
	sort.Slice(tableScores, func(i, j int) bool {
		return tableScores[i].Score > tableScores[j].Score
	})

	// 高相关表：完整信息
	// 中相关表：只包含主键和外键
	// 低相关表：只包含表名和注释
	return o.buildTieredSchemaSummary(tableScores, tables)
}

// buildTieredSchemaSummary 构建分层Schema摘要
func (o *SchemaOptimizer) buildTieredSchemaSummary(tableScores []TableScore, tables map[string]*modelai.TableCache) string {
	var sb strings.Builder

	sb.WriteString("## Schema 分层摘要\n\n")

	highRelevance := o.MaxTables / 2
	mediumRelevance := o.MaxTables

	// 高相关表（Top 50%）
	if highRelevance > 0 && len(tableScores) > 0 {
		sb.WriteString("### 核心表（完整信息）\n")
		for i := 0; i < highRelevance && i < len(tableScores); i++ {
			ts := tableScores[i]
			cache := tables[ts.Name]
			if cache != nil {
				sb.WriteString(o.formatTableFull(ts.Name, cache))
			}
		}
	}

	// 中相关表（Top 50-80%）
	if mediumRelevance > highRelevance && len(tableScores) > highRelevance {
		sb.WriteString("\n### 关联表（主键外键）\n")
		for i := highRelevance; i < mediumRelevance && i < len(tableScores); i++ {
			ts := tableScores[i]
			cache := tables[ts.Name]
			if cache != nil {
				sb.WriteString(o.formatTableKeysOnly(ts.Name, cache))
			}
		}
	}

	// 低相关表（剩余）
	if len(tableScores) > mediumRelevance {
		sb.WriteString("\n### 其他表（仅表名）\n")
		for i := mediumRelevance; i < len(tableScores); i++ {
			ts := tableScores[i]
			cache := tables[ts.Name]
			if cache != nil {
				name := ts.Name
				if cache.TableComment != "" {
					name = cache.TableComment + " (" + name + ")"
				}
				sb.WriteString("- " + name + "\n")
			}
		}
	}

	return sb.String()
}

// formatTableFull 格式化完整表信息
func (o *SchemaOptimizer) formatTableFull(tableName string, cache *modelai.TableCache) string {
	var sb strings.Builder
	name := tableName
	if cache.TableComment != "" {
		name = cache.TableComment + " (" + tableName + ")"
	}
	sb.WriteString("- **" + name + "**\n")

	if len(cache.ColumnsJSON) > 0 {
		fields := make([]string, 0, len(cache.ColumnsJSON))
		for _, col := range cache.ColumnsJSON {
			marker := ""
			if col.IsPK {
				marker = " [PK]"
			} else if strings.HasSuffix(strings.ToLower(col.Name), "_id") {
				marker = " [FK]"
			}
			fields = append(fields, col.Name+" ("+col.Type+")"+marker)
		}
		sb.WriteString("  字段: " + strings.Join(fields, ", ") + "\n")
	}

	return sb.String()
}

// formatTableKeysOnly 只格式化主键和外键
func (o *SchemaOptimizer) formatTableKeysOnly(tableName string, cache *modelai.TableCache) string {
	var sb strings.Builder
	name := tableName
	if cache.TableComment != "" {
		name = cache.TableComment + " (" + tableName + ")"
	}
	sb.WriteString("- **" + name + "**\n")

	if len(cache.ColumnsJSON) > 0 {
		keys := make([]string, 0)
		for _, col := range cache.ColumnsJSON {
			if col.IsPK || false || strings.HasSuffix(strings.ToLower(col.Name), "_id") {
				marker := "[PK]"
				if false || strings.HasSuffix(strings.ToLower(col.Name), "_id") {
					marker = "[FK]"
				}
				keys = append(keys, col.Name+" ("+col.Type+")"+marker)
			}
		}
		if len(keys) > 0 {
			sb.WriteString("  键: " + strings.Join(keys, ", ") + "\n")
		}
	}

	return sb.String()
}

// OptimizeFieldBudget 优化字段预算分配
// 根据表的相关性动态分配字段预算
func (o *SchemaOptimizer) OptimizeFieldBudget(tableScores []TableScore, totalBudget int) map[string]int {
	allocation := make(map[string]int)

	if len(tableScores) == 0 {
		return allocation
	}

	// 计算总分
	totalScore := 0.0
	for _, ts := range tableScores {
		if ts.Score > 0 {
			totalScore += ts.Score
		}
	}

	if totalScore == 0 {
		// 平均分配
		perTable := int(math.Ceil(float64(totalBudget) / float64(len(tableScores))))
		for _, ts := range tableScores {
			allocation[ts.Name] = perTable
		}
		return allocation
	}

	// 按比例分配
	remaining := totalBudget
	for i, ts := range tableScores {
		if ts.Score <= 0 {
			continue
		}

		// 最后一张表分完剩余预算
		if i == len(tableScores)-1 {
			allocation[ts.Name] = remaining
			break
		}

		// 按相关性比例分配
		share := int(math.Ceil((ts.Score / totalScore) * float64(totalBudget)))
		if share > remaining {
			share = remaining
		}
		allocation[ts.Name] = share
		remaining -= share
	}

	return allocation
}

// isColumnRelevant 判断字段是否与查询相关
func (o *SchemaOptimizer) isColumnRelevant(col modelai.ColumnInfo, keywords []string) bool {
	// 主键和外键总是包含
	if col.IsPK || strings.HasSuffix(strings.ToLower(col.Name), "_id") {
		return true
	}

	// 检查字段名或注释是否匹配关键词
	lowerName := strings.ToLower(col.Name)
	lowerComment := strings.ToLower(col.Comment)

	for _, kw := range keywords {
		kwLower := strings.ToLower(kw)
		if strings.Contains(lowerName, kwLower) || strings.Contains(lowerComment, kwLower) {
			return true
		}
	}

	return false
}
