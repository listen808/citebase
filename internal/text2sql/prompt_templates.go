package text2sql

type QueryType int

const (
	QueryTypeUnknown QueryType = iota
	QueryTypeBasic
	QueryTypeMultiJoin
	QueryTypeAggregation
	QueryTypeSubquery
	QueryTypeCTE
	QueryTypeComplexReport
)

func (q QueryType) String() string {
	switch q {
	case QueryTypeBasic:
		return "basic"
	case QueryTypeMultiJoin:
		return "multi_join"
	case QueryTypeAggregation:
		return "aggregation"
	case QueryTypeSubquery:
		return "subquery"
	case QueryTypeCTE:
		return "cte"
	case QueryTypeComplexReport:
		return "complex_report"
	default:
		return "basic"
	}
}

var queryTypeKeywords = map[QueryType][]string{
	QueryTypeAggregation:   {"total", "count", "sum", "avg", "max", "min", "stat", "group by", "top", "each", "rank", "percent", "统计", "总数", "平均", "合计", "分组"},
	QueryTypeSubquery:      {"subquery", "exists", "in (select", "子查询", "存在"},
	QueryTypeMultiJoin:     {"join", "link", "associate", "connect", "both", "关联", "连接", "一样", "相同", "对应", "对比"},
	QueryTypeCTE:           {"cte", "with as", "recursive", "cumulative"},
	QueryTypeComplexReport: {"report", "monthly", "annual", "quarterly", "yoy", "qoq", "trend", "compare", "analysis", "报表", "月度", "年度", "趋势"},
}

func ClassifyQuery(text string) QueryType {
	lower := toLower(text)
	maxCount := 0
	detected := QueryTypeUnknown

	for qType, keywords := range queryTypeKeywords {
		count := 0
		for _, kw := range keywords {
			if containsLower(lower, kw) {
				count++
			}
		}
		if count > maxCount {
			maxCount = count
			detected = qType
		}
	}

	return detected
}

func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			result[i] = c + 32
		} else {
			result[i] = c
		}
	}
	return string(result)
}

func containsLower(s, substr string) bool {
	if len(s) < len(substr) {
		return false
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if s[i+j] != substr[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

type PromptTemplate struct {
	Type         QueryType
	Name         string
	SystemPrompt string
	UserTemplate string
}

// promptTemplates 内置提示词模板（中文，模仿知识问答的"基于上下文→分析→生成"模式）
var promptTemplates = []PromptTemplate{
	{
		Type:         QueryTypeBasic,
		Name:         "基础查询",
		SystemPrompt: "你是一个资深MySQL数据库专家。请先仔细分析提供的表结构（DDL、字段类型、主键、约束、注释、表间关系），理解每个字段的含义和用途，然后生成准确高效的SQL查询。如果表结构信息不足以生成准确SQL，请如实说明。",
		UserTemplate: "基于以下数据库表结构信息，生成符合用户需求的MySQL SELECT查询。\n\n" +
			"## 用户问题\n{question}\n\n" +
			"## 表结构信息\n{schema_context}\n\n" +
			"## 表间关系\n{relations}\n\n" +
			"## 语义上下文\n{semantic_context}\n\n" +
			"## 🚨 最高优先级规则（违反将导致SQL完全不可用）\n" +
			"1. **字段名必须从DDL中获取**：SQL中的每个字段名都必须是DDL中实际存在的列名\n" +
			"2. **用户提到的字段名仅作语义参考**：用户可能说\"val\"但DDL中实际是\"param_val\"，必须用DDL中的名\n" +
			"3. **查看\"可能被称作\"列**：字段详情表中有\"可能被称作\"列，帮助你将用户的称呼映射到实际字段名\n" +
			"4. **生成前先列出字段映射**：在解释中先写出\"用户说的X → DDL中的Y\"映射表\n" +
			"5. 使用MySQL标准SQL语法，所有字段使用 `表别名.字段名` 格式\n" +
			"6. 为每张表定义简短别名（如 t, u, p）\n" +
			"7. 只返回SELECT查询\n\n" +
			"## Few-Shot 示例（学习正确的字段映射方式）\n" +
			"问题：查询bm_dc_param表中val包含caftp的参数值和BM_DATA_DICT表中的字典值，两个表val值相同\n" +
			"字段映射：val → bm_dc_param.param_val, VALUE → BM_DATA_DICT.PARAM_VALUE\n" +
			"```sql\n" +
			"SELECT p.param_val AS 参数值, d.PARAM_VALUE AS 字典值\n" +
			"FROM bm_dc_param p\n" +
			"INNER JOIN BM_DATA_DICT d ON p.param_val = d.PARAM_VALUE\n" +
			"WHERE p.param_val LIKE '%caftp%';\n" +
			"```\n\n" +
			"## 输出格式\n" +
			"SQL:\n" + "```sql\n" +
			"-- 查询说明：...\n" +
			"SELECT t.field1, t.field2\n" +
			"FROM table_name t\n" +
			"[WHERE condition]\n" +
			"[ORDER BY field]\n" +
			"[LIMIT n]\n" + "```\n" +
			"解释:\n" +
			"1. 字段映射：用户说的X → DDL中的Y\n" +
			"2. 使用的表和别名：列出涉及的表及别名\n" +
			"3. 表间关联：说明JOIN方式和关联字段\n" +
			"4. 查询逻辑：说明SELECT字段、WHERE条件、排序方式等\n",
	},
	{
		Type:         QueryTypeMultiJoin,
		Name:         "多表关联查询",
		SystemPrompt: "你是一个资深MySQL数据库专家，擅长多表关联查询。请先仔细分析各表结构、关联关系和字段含义，然后生成准确的多表JOIN查询。如果表结构信息不足，请如实说明。",
		UserTemplate: "基于以下数据库表结构信息，生成多表关联查询的MySQL SELECT语句。\n\n" +
			"## 用户问题\n{question}\n\n" +
			"## 表结构信息\n{schema_context}\n\n" +
			"## 表间关系\n{relations}\n\n" +
			"## 语义上下文\n{semantic_context}\n\n" +
			"## 🚨 最高优先级规则（违反将导致SQL完全不可用）\n" +
			"1. **字段名必须从DDL中获取**：SQL中的每个字段名都必须是DDL中实际存在的列名\n" +
			"2. **用户提到的字段名仅作语义参考**：用户可能说\"val\"但DDL中实际是\"param_val\"，必须用DDL中的名\n" +
			"3. **查看\"可能被称作\"列**：字段详情表中有\"可能被称作\"列，帮助你将用户的称呼映射到实际字段名\n" +
			"4. **生成前先列出字段映射**：在解释中先写出\"用户说的X → DDL中的Y\"映射表\n" +
			"5. 选择合适的JOIN类型（INNER/LEFT/RIGHT），确保关联条件正确\n" +
			"6. 所有字段使用 `表别名.字段名` 格式\n" +
			"7. 避免选择重复字段，添加中文注释\n" +
			"8. 只返回SELECT查询\n\n" +
			"## Few-Shot 示例（学习正确的字段映射方式）\n" +
			"问题：查询bm_dc_param表中val包含caftp的参数值和BM_DATA_DICT表中的字典值，两个表val值相同\n" +
			"字段映射：val → bm_dc_param.param_val, VALUE → BM_DATA_DICT.PARAM_VALUE\n" +
			"```sql\n" +
			"SELECT p.param_val AS 参数值, d.PARAM_VALUE AS 字典值\n" +
			"FROM bm_dc_param p\n" +
			"INNER JOIN BM_DATA_DICT d ON p.param_val = d.PARAM_VALUE\n" +
			"WHERE p.param_val LIKE '%caftp%';\n" +
			"```\n\n" +
			"## 输出格式\n" +
			"SQL:\n" + "```sql\n" +
			"-- 查询说明：...\n" +
			"SELECT t1.field1, t2.field2\n" +
			"FROM table1 t1\n" +
			"INNER JOIN table2 t2 ON t1.id = t2.ref_id\n" +
			"[LEFT JOIN table3 t3 ON t2.id = t3.ref_id]\n" +
			"[WHERE condition]\n" +
			"[ORDER BY field]\n" +
			"[LIMIT n]\n" + "```\n" +
			"解释:\n" +
			"1. 字段映射：用户说的X → DDL中的Y\n" +
			"2. 使用的表和别名：列出涉及的表及别名\n" +
			"3. 表间关联：说明JOIN类型（INNER/LEFT/RIGHT）和关联字段\n" +
			"4. 查询逻辑：说明返回哪些字段、筛选条件、排序方式\n",
	},
	{
		Type:         QueryTypeAggregation,
		Name:         "聚合统计分析",
		SystemPrompt: "你是一个资深MySQL数据库专家，擅长聚合统计分析。请先分析表结构中的数值字段和分组维度，然后生成准确的聚合查询。",
		UserTemplate: "基于以下数据库表结构信息，生成聚合统计的MySQL SELECT查询。\n\n" +
			"## 用户问题\n{question}\n\n" +
			"## 表结构信息\n{schema_context}\n\n" +
			"## 表间关系\n{relations}\n\n" +
			"## 语义上下文\n{semantic_context}\n\n" +
			"## 🚨 最高优先级规则\n" +
			"1. **字段名必须从DDL中获取**：SQL中的每个字段名都必须是DDL中实际存在的列名\n" +
			"2. **用户提到的字段名仅作语义参考**：用户可能说\"val\"但DDL中实际是\"param_val\"\n" +
			"3. **查看\"可能被称作\"列**：帮助将用户的称呼映射到实际字段名\n" +
			"4. 使用正确的聚合函数：COUNT/SUM/AVG/MAX/MIN\n" +
			"5. GROUP BY 必须包含 SELECT 中所有非聚合字段\n" +
			"6. 使用 HAVING 过滤聚合结果\n" +
			"7. 使用 `表别名.字段名` 格式，添加中文注释\n" +
			"8. 只返回SELECT查询\n\n" +
			"## 输出格式\n" +
			"SQL:\n" + "```sql\n" +
			"-- 统计说明：...\n" +
			"SELECT t.dimension, COUNT(*) AS total, SUM(t.amount) AS total_amount\n" +
			"FROM table_name t\n" +
			"[JOIN other_table o ON condition]\n" +
			"[WHERE filter]\n" +
			"GROUP BY t.dimension\n" +
			"[HAVING COUNT(*) > 0]\n" +
			"[ORDER BY total DESC]\n" +
			"[LIMIT n]\n" + "```\n" +
			"解释:\n" +
			"1. 字段映射\n" +
			"2. 统计维度：...\n" +
			"3. 统计指标：...\n" +
			"4. 查询逻辑：...\n",
	},
	{
		Type:         QueryTypeSubquery,
		Name:         "子查询",
		SystemPrompt: "你是一个资深MySQL数据库专家，擅长子查询和嵌套查询。请先分析表结构中的数据层次关系，然后选择合适的子查询方式生成SQL。",
		UserTemplate: "基于以下数据库表结构信息，生成包含子查询的MySQL SELECT语句。\n\n" +
			"## 用户问题\n{question}\n\n" +
			"## 表结构信息\n{schema_context}\n\n" +
			"## 表间关系\n{relations}\n\n" +
			"## 语义上下文\n{semantic_context}\n\n" +
			"## 🚨 最高优先级规则\n" +
			"1. **字段名必须从DDL中获取**：SQL中的每个字段名都必须是DDL中实际存在的列名\n" +
			"2. **用户提到的字段名仅作语义参考**：用户可能说\"val\"但DDL中实际是\"param_val\"\n" +
			"3. 选择合适的子查询类型：IN/NOT IN/EXISTS/标量子查询\n" +
			"4. 使用 `表别名.字段名` 格式，添加中文注释\n" +
			"5. 只返回SELECT查询\n\n" +
			"## 输出格式\n" +
			"SQL:\n" + "```sql\n" +
			"-- 查询说明：...\n" +
			"SELECT t.field1, t.field2\n" +
			"FROM table1 t\n" +
			"WHERE t.field [NOT] IN (\n" +
			"    SELECT field FROM table2 WHERE condition\n" +
			")\n" +
			"[ORDER BY field]\n" +
			"[LIMIT n]\n" + "```\n" +
			"解释:\n" +
			"1. 字段映射\n" +
			"2. 子查询类型和原因：...\n" +
			"3. 查询逻辑：...\n",
	},
	{
		Type:         QueryTypeCTE,
		Name:         "CTE通用表表达式",
		SystemPrompt: "你是一个资深MySQL 8.0数据库专家，擅长CTE（公共表表达式）查询。请先分析数据处理的层次结构，然后使用WITH AS构建清晰的CTE查询。",
		UserTemplate: "基于以下数据库表结构信息，使用CTE（WITH AS）生成MySQL 8.0查询。\n\n" +
			"## 用户问题\n{question}\n\n" +
			"## 表结构信息\n{schema_context}\n\n" +
			"## 表间关系\n{relations}\n\n" +
			"## 语义上下文\n{semantic_context}\n\n" +
			"## 🚨 最高优先级规则\n" +
			"1. **字段名必须从DDL中获取**：SQL中的每个字段名都必须是DDL中实际存在的列名\n" +
			"2. **用户提到的字段名仅作语义参考**\n" +
			"3. 使用 WITH AS 定义CTE，每个CTE有明确的中文注释\n" +
			"4. 使用 `表别名.字段名` 格式\n" +
			"5. 只返回SELECT查询\n\n" +
			"## 输出格式\n" +
			"SQL:\n" + "```sql\n" +
			"-- 查询说明：...\n" +
			"WITH\n" +
			"    -- 第一步：...\n" +
			"    cte1 AS (\n" +
			"        SELECT field FROM table1 WHERE condition\n" +
			"    ),\n" +
			"    -- 第二步：...\n" +
			"    cte2 AS (\n" +
			"        SELECT field FROM table2 JOIN cte1 ON condition\n" +
			"    )\n" +
			"SELECT field FROM cte1 JOIN cte2 ON condition\n" +
			"[ORDER BY field]\n" +
			"[LIMIT n]\n" + "```\n" +
			"解释:\n" +
			"1. 字段映射\n" +
			"2. 数据处理步骤：...\n" +
			"3. 查询逻辑：...\n",
	},
	{
		Type:         QueryTypeComplexReport,
		Name:         "复杂报表查询",
		SystemPrompt: "你是一个资深MySQL数据库专家，擅长复杂报表查询。请先全面分析表结构、字段含义和数据关系，然后构建包含多维度分析、条件判断和聚合统计的复杂查询。",
		UserTemplate: "基于以下数据库表结构信息，生成复杂报表的MySQL SELECT查询。\n\n" +
			"## 用户问题\n{question}\n\n" +
			"## 表结构信息\n{schema_context}\n\n" +
			"## 表间关系\n{relations}\n\n" +
			"## 语义上下文\n{semantic_context}\n\n" +
			"## 🚨 最高优先级规则\n" +
			"1. **字段名必须从DDL中获取**：SQL中的每个字段名都必须是DDL中实际存在的列名\n" +
			"2. **用户提到的字段名仅作语义参考**：用户可能说\"val\"但DDL中实际是\"param_val\"\n" +
			"3. **查看\"可能被称作\"列**：帮助将用户的称呼映射到实际字段名\n" +
			"4. 多表JOIN、聚合、分组、排序综合使用\n" +
			"5. 使用 CASE WHEN 处理条件逻辑\n" +
			"6. 使用 `表别名.字段名` 格式，添加中文注释\n" +
			"7. 只返回SELECT查询\n\n" +
			"## 输出格式\n" +
			"SQL:\n" + "```sql\n" +
			"-- 报表说明：...\n" +
			"SELECT\n" +
			"    t.dimension_field AS 维度,\n" +
			"    COUNT(*) AS 总数,\n" +
			"    SUM(t.amount) AS 总金额,\n" +
			"    AVG(t.field) AS 平均值,\n" +
			"    CASE WHEN condition THEN '是' ELSE '否' END AS 标识\n" +
			"FROM table1 t\n" +
			"[JOIN table2 t2 ON condition]\n" +
			"[WHERE filter]\n" +
			"GROUP BY t.dimension_field\n" +
			"[HAVING condition]\n" +
			"[ORDER BY field]\n" +
			"[LIMIT n]\n" + "```\n" +
			"解释:\n" +
			"1. 字段映射\n" +
			"2. 报表维度：...\n" +
			"3. 报表指标：...\n" +
			"4. 查询逻辑和数据来源：...\n",
	},
}

func GetPromptTemplate(qType QueryType) PromptTemplate {
	for _, tpl := range promptTemplates {
		if tpl.Type == qType {
			return tpl
		}
	}
	return promptTemplates[0]
}

// BuiltInPromptItem 代码层内置提示词的导出格式
type BuiltInPromptItem struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	QueryType    string `json:"query_type"`
	SystemPrompt string `json:"system_prompt"`
	UserTemplate string `json:"user_template"`
	Description  string `json:"description"`
	IsBuiltIn    bool   `json:"is_built_in"`
}

// GetBuiltInPromptTemplates 导出所有代码层内置提示词模板
func GetBuiltInPromptTemplates() []BuiltInPromptItem {
	items := make([]BuiltInPromptItem, 0, len(promptTemplates))
	for _, tpl := range promptTemplates {
		items = append(items, BuiltInPromptItem{
			ID:           "builtin_" + tpl.Type.String(),
			Name:         tpl.Name,
			QueryType:    tpl.Type.String(),
			SystemPrompt: tpl.SystemPrompt,
			UserTemplate: tpl.UserTemplate,
			Description:  tpl.Name + "（代码内置）",
			IsBuiltIn:    true,
		})
	}
	return items
}

var reflectionPromptTemplate string

func init() {
	reflectionPromptTemplate = "你是一个严格的MySQL SQL代码审查员。所有修正必须使用MySQL兼容语法，严禁使用Oracle专有函数。\n\n" +
		"## 原始问题\n{question}\n\n" +
		"## AI生成的SQL\n{sql}\n\n" +
		"## 表结构信息\n{schema_context}\n\n" +
		"## 表间关系\n{relations}\n\n" +
		"## 审查清单（逐项检查，给出通过/不通过）\n" +
		"1. 表选择：是否使用了正确的表？\n" +
		"2. 字段存在性：每个字段是否都在对应的表中存在？\n" +
		"3. JOIN正确性：多表查询是否包含了必要的JOIN和关联条件？\n" +
		"4. 重复字段：SELECT中是否有重复的字段名？\n" +
		"5. 条件逻辑：WHERE条件是否符合用户需求？\n" +
		"6. 聚合正确性：GROUP BY是否匹配聚合函数？\n" +
		"7. MySQL语法兼容性：SQL语法是否对MySQL有效？严禁使用Oracle专有语法（DBMS_LOB、VARCHAR2、CAST AS VARCHAR2、ROWNUM、SYSDATE等）。DDL中的CLOB类型在MySQL中就是TEXT，直接用=和LIKE比较即可，无需任何类型转换函数。\n" +
		"8. 性能：是否有明显的性能问题（如 SELECT *）？\n" +
		"9. 表别名：所有表别名是否正确定义和使用？\n" +
		"10. 字段限定：所有模糊字段是否用表别名限定了？\n" +
		"11. NULL处理：NULL值是否正确处理？\n" +
		"12. 数据类型：JOIN/WHERE中的比较是否兼容？注意：MySQL中TEXT类型可以直接用=和LIKE比较，不需要CAST或SUBSTR转换。\n" +
		"13. 语义正确性：SQL是否真正回答了用户的问题？例如用户问'总数'但SQL没有COUNT。\n" +
		"14. 边界情况：是否处理了空结果、NULL值、重复数据等边界情况？\n\n" +
		"## 修正要求\n" +
		"如果发现问题，请在corrected_sql中给出修正后的完整SQL，而不是片段。\n" +
		"如果所有检查通过，将passed设为true，corrected_sql填入原始SQL。\n\n" +
		"## 输出格式\n" +
		"严格使用JSON格式输出，放在markdown代码块内：\n\n" +
		"```json\n" +
		"{\n" +
		"  \"passed\": true/false,\n" +
		"  \"issues\": [\n" +
		"    {\"type\": \"问题类型\", \"detail\": \"问题描述\", \"suggestion\": \"修复建议\"}\n" +
		"  ],\n" +
		"  \"corrected_sql\": \"修正后的SQL或通过时原SQL\"\n" +
		"}\n" +
		"```"
}
