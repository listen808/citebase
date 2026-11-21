package text2sql

import (
	"fmt"
	"strings"
)

// CoTPromptConfig CoT提示词配置
type CoTPromptConfig struct {
	EnableCoT       bool // 是否启用思维链
	MaxThoughtSteps int  // 最大思考步骤
	IncludeExamples bool // 是否包含Few-Shot示例
	ExampleCount    int  // 示例数量
	ForceJSONOutput bool // 强制JSON输出
}

// DefaultCoTPromptConfig 默认CoT配置
func DefaultCoTPromptConfig() *CoTPromptConfig {
	return &CoTPromptConfig{
		EnableCoT:       true,
		MaxThoughtSteps: 5,
		IncludeExamples: true,
		ExampleCount:    3,
		ForceJSONOutput: true,
	}
}

// CoTPromptBuilder CoT提示词构建器
// P0增强：Chain-of-Thought思维链引导
type CoTPromptBuilder struct {
	config *CoTPromptConfig
}

// NewCoTPromptBuilder 创建CoT提示词构建器
func NewCoTPromptBuilder() *CoTPromptBuilder {
	return &CoTPromptBuilder{
		config: DefaultCoTPromptConfig(),
	}
}

// NewCoTPromptBuilderWithConfig 创建带配置的构建器
func NewCoTPromptBuilderWithConfig(cfg *CoTPromptConfig) *CoTPromptBuilder {
	return &CoTPromptBuilder{config: cfg}
}

// BuildSystemPrompt 构建CoT增强的系统提示词
func (b *CoTPromptBuilder) BuildSystemPrompt(schemaContext string) string {
	var sb strings.Builder

	sb.WriteString(`你是一个专业的SQL生成专家，擅长将自然语言问题转换为准确的SQL查询。

## 核心能力
- 理解业务需求，生成高效的SQL
- 处理单表查询、多表JOIN、子查询、聚合查询
- 确保SQL语法正确、逻辑清晰

`)

	// CoT思维链部分
	if b.config.EnableCoT {
		sb.WriteString(b.buildCoTInstructions())
	}

	// Schema上下文
	if schemaContext != "" {
		sb.WriteString("\n## 数据库Schema\n")
		sb.WriteString(schemaContext)
	}

	// Few-Shot示例
	if b.config.IncludeExamples {
		sb.WriteString(b.buildFewShotExamples())
	}

	// JSON输出格式
	if b.config.ForceJSONOutput {
		sb.WriteString(b.buildJSONFormat())
	}

	return sb.String()
}

// buildCoTInstructions 构建思维链指令
func (b *CoTPromptBuilder) buildCoTInstructions() string {
	return `
## 思维链（Chain-of-Thought）分析步骤

对于复杂查询，请按以下步骤逐步分析：

### 步骤1：问题拆解
- 识别问题的核心目标（统计/查询/更新）
- 提取关键实体和数据范围
- 确定需要哪些表和字段

### 步骤2：关系推理
- 分析表之间的关联关系
- 确定JOIN条件和顺序
- 识别必要的过滤条件

### 步骤3：聚合逻辑
- 确定分组维度
- 选择合适的聚合函数
- 设置HAVING条件（如需要）

### 步骤4：SQL构建
- SELECT：选择需要的字段和聚合结果
- FROM：确定主表
- JOIN：添加必要的关联表
- WHERE：添加过滤条件
- GROUP BY：添加分组（如需要）
- HAVING：添加聚合过滤（如需要）
- ORDER BY/LIMIT：排序和限制（如需要）

### 步骤5：自检优化
- 检查字段映射是否正确
- 验证JOIN条件是否完整
- 确认聚合逻辑是否符合需求
`
}

// buildFewShotExamples 构建Few-Shot示例
func (b *CoTPromptBuilder) buildFewShotExamples() string {
	examples := b.getExamples()
	count := b.config.ExampleCount
	if count > len(examples) {
		count = len(examples)
	}

	var sb strings.Builder
	sb.WriteString("\n## 示例\n\n")

	for i := 0; i < count; i++ {
		ex := examples[i]
		sb.WriteString(fmt.Sprintf("### 示例%d\n", i+1))
		sb.WriteString(fmt.Sprintf("**问题**: %s\n\n", ex.Question))
		sb.WriteString(fmt.Sprintf("**分析**: %s\n\n", ex.Analysis))
		sb.WriteString("**生成的SQL**:\n```sql\n" + ex.SQL + "\n```\n\n")
	}

	return sb.String()
}

// Example 示例结构
type Example struct {
	Question string
	Analysis string
	SQL      string
}

// getExamples 获取示例库
func (b *CoTPromptBuilder) getExamples() []Example {
	return []Example{
		{
			Question: "统计每个部门的员工数量和平均工资",
			Analysis: "1) 目标：统计-每部门-员工数量和平均工资\n2) 涉及表：员工表(emp/dept_id/salary)、部门表\n3) 聚合维度：部门(dept_id)\n4) 聚合函数：COUNT(*), AVG(salary)\n5) 关联：员工表JOIN部门表获取部门名称",
			SQL: `SELECT d.name AS 部门名称,
       COUNT(e.id) AS 员工数量,
       ROUND(AVG(e.salary), 2) AS 平均工资
FROM employee e
INNER JOIN department d ON e.dept_id = d.id
GROUP BY d.id, d.name
ORDER BY 员工数量 DESC`,
		},
		{
			Question: "查找本月销售额超过10万的商品及其销售总额",
			Analysis: "1) 目标：查找-本月-销售额超过10万的商品\n2) 时间条件：本月开始至今\n3) 涉及表：商品表(product/name)、销售表(sale/product_id/amount/date)\n4) 聚合维度：商品(product_id)\n5) 聚合函数：SUM(amount)\n6) 过滤：金额>100000，时间在当月\n7) 需要关联商品表获取商品名称",
			SQL: `SELECT p.name AS 商品名称,
       SUM(s.amount) AS 销售总额
FROM sale s
INNER JOIN product p ON s.product_id = p.id
WHERE s.date >= DATE_FORMAT(CURDATE(), '%Y-%m-01')
GROUP BY p.id, p.name
HAVING SUM(s.amount) > 100000
ORDER BY 销售总额 DESC`,
		},
		{
			Question: "查询所有订单及其客户信息，按订单金额降序排列",
			Analysis: "1) 目标：查询-所有订单-及其客户信息\n2) 涉及表：订单表(order/customer_id/total_amount)、客户表(customer/name/phone)\n3) 关联：通过customer_id关联\n4) 排序：按订单金额降序",
			SQL: `SELECT o.id AS 订单号,
       o.order_date AS 订单日期,
       o.total_amount AS 订单金额,
       c.name AS 客户名称,
       c.phone AS 联系电话
FROM ` + "`order`" + ` o
LEFT JOIN customer c ON o.customer_id = c.id
ORDER BY o.total_amount DESC`,
		},
		{
			Question: "查找同时买过商品A和商品B的客户",
			Analysis: "1) 目标：查找-同时-买过商品A和B的客户\n2) 需要找出购买过指定商品的所有客户\n3) 方法：子查询或自连接\n4) 涉及表：订单明细表(order_item/order_id/product_id)、订单表(order/customer_id)\n5) 过滤：商品名为'A'和'B'",
			SQL: `SELECT DISTINCT c.id, c.name AS 客户名称
FROM customer c
INNER JOIN ` + "`order`" + ` o ON c.id = o.customer_id
INNER JOIN order_item oi ON o.id = oi.order_id
INNER JOIN product p ON oi.product_id = p.id
WHERE p.name IN ('商品A', '商品B')
GROUP BY c.id, c.name
HAVING COUNT(DISTINCT p.id) = 2`,
		},
		{
			Question: "计算每个季度的销售额和订单数",
			Analysis: "1) 目标：计算-每个季度-的销售额和订单数\n2) 时间维度：按季度分组\n3) 涉及表：订单表(order/date/total_amount)\n4) 聚合维度：季度(QUARTER(date))\n5) 聚合函数：SUM(total_amount), COUNT(*)",
			SQL: `SELECT CONCAT(YEAR(date), '-Q', QUARTER(date)) AS 季度,
       SUM(total_amount) AS 销售总额,
       COUNT(*) AS 订单数量
FROM ` + "`order`" + `
WHERE date >= DATE_SUB(CURDATE(), INTERVAL 1 YEAR)
GROUP BY YEAR(date), QUARTER(date)
ORDER BY 季度 DESC`,
		},
	}
}

// buildJSONFormat 构建JSON输出格式说明
func (b *CoTPromptBuilder) buildJSONFormat() string {
	return "## 输出格式要求\n\n" +
		"请按以下JSON格式输出结果：\n\n" +
		"```json\n" +
		"{\n" +
		"  \"reasoning\": \"你的分析推理过程\",\n" +
		"  \"sql\": \"生成的SQL语句\",\n" +
		"  \"confidence\": 0.95,\n" +
		"  \"tables_used\": [\"table1\", \"table2\"],\n" +
		"  \"explanation\": \"SQL的简要说明\"\n" +
		"}\n" +
		"```\n\n" +
		"注意：\n" +
		"- reasoning：描述你的分析过程，帮助用户理解\n" +
		"- sql：完整的SQL语句，确保语法正确\n" +
		"- confidence：对这个SQL正确性的自信度（0-1）\n" +
		"- tables_used：使用的表列表\n" +
		"- explanation：SQL的简要说明"
}

// BuildThoughtPrompt 构建思考过程提示词（用于强制CoT）
func (b *CoTPromptBuilder) BuildThoughtPrompt(question string, schemaContext string) string {
	return "请分析以下问题并生成SQL：\n\n" +
		"问题：" + question + "\n\n" +
		"Schema：\n" + schemaContext + "\n\n" +
		"请先思考，然后输出结果：\n\n" +
		"[THOUGHT]你的分析过程：\n" +
		"1. 问题拆解：...\n" +
		"2. 关系推理：...\n" +
		"3. 聚合逻辑：...\n" +
		"4. SQL构建：...\n" +
		"[THOUGHT]\n\n" +
		"[SQL]\n" +
		"生成的SQL...\n" +
		"[SQL]\n\n" +
		"[RESULT]\n" +
		"JSON格式输出...\n" +
		"[RESULT]"
}

// BuildDynamicPrompt 根据问题类型动态构建提示词
func (b *CoTPromptBuilder) BuildDynamicPrompt(question string, queryType string, schemaContext string, examples []Example) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf(`你是一个SQL生成专家。请根据以下问题生成SQL。

## 问题
%s

## 查询类型识别
类型：%s

`, question, queryType))

	// 根据查询类型添加特定指导
	sb.WriteString(b.getTypeSpecificGuidance(queryType))

	// Schema
	if schemaContext != "" {
		sb.WriteString("\n## 数据库Schema\n")
		sb.WriteString(schemaContext)
	}

	// 相关示例
	if len(examples) > 0 {
		sb.WriteString("\n## 相关示例\n")
		for i, ex := range examples {
			if i >= b.config.ExampleCount {
				break
			}
			sb.WriteString(fmt.Sprintf("问题：%s\nSQL：%s\n\n", ex.Question, ex.SQL))
		}
	}

	// 输出格式
	sb.WriteString(b.buildJSONFormat())

	return sb.String()
}

// getTypeSpecificGuidance 根据查询类型提供特定指导
func (b *CoTPromptBuilder) getTypeSpecificGuidance(queryType string) string {
	switch strings.ToLower(queryType) {
	case "aggregation":
		return `
## 聚合查询指导
- 明确定义分组维度（GROUP BY）
- 选择合适的聚合函数：COUNT/SUM/AVG/MIN/MAX
- 注意聚合后过滤用HAVING，不是WHERE
- 使用AS给聚合字段起别名
`
	case "join":
		return `
## JOIN查询指导
- 确定主表（FROM）
- 识别所有需要关联的表
- 明确JOIN类型：INNER/LEFT/RIGHT
- 确保ON条件完整且正确
- 避免笛卡尔积
`
	case "subquery":
		return `
## 子查询指导
- 区分标量子查询、列子查询、表子查询
- 注意子查询的执行顺序
- 考虑使用WITH (CTE)替代嵌套子查询
- 子查询放在WHERE或FROM中
`
	case "cte":
		return `
## CTE(公用表表达式)指导
- 使用WITH定义CTE
- CTE可以提高SQL可读性
- 多个CTE用逗号分隔
- CTE只在整个查询中有效
`
	case "complex":
		return `
## 复杂查询指导
- 将复杂问题分解为多个简单步骤
- 考虑使用临时表或CTE
- 逐步验证每步结果
- 注意执行顺序和性能
`
	default:
		return `
## 通用指导
- 确保SQL语法正确
- 使用合适的表别名
- 添加必要的过滤条件
- 考虑查询性能
`
	}
}

// DetectQueryType 检测查询类型
func (b *CoTPromptBuilder) DetectQueryType(question string) string {
	questionLower := strings.ToLower(question)

	// 聚合关键词
	aggKeywords := []string{"统计", "汇总", "合计", "平均", "总数", "最多", "最少", "排名", "count", "sum", "avg", "max", "min"}
	for _, kw := range aggKeywords {
		if strings.Contains(questionLower, kw) {
			return "aggregation"
		}
	}

	// JOIN关键词
	joinKeywords := []string{"和", "以及", "关联", "连接", "join"}
	if b.containsAny(questionLower, joinKeywords) {
		return "join"
	}

	// CTE关键词
	cteKeywords := []string{"递归", "with", "累计", "running"}
	if b.containsAny(questionLower, cteKeywords) {
		return "cte"
	}

	// 子查询关键词
	subKeywords := []string{"所有", "既", "同时", "满足", "子查询"}
	if b.containsAny(questionLower, subKeywords) {
		return "subquery"
	}

	// 复杂查询识别
	complexKeywords := []string{"每月", "每季度", "每年", "同比", "环比", "趋势", "多维度"}
	if b.containsAny(questionLower, complexKeywords) {
		return "complex"
	}

	return "basic"
}

// containsAny 检查是否包含任意关键词
func (b *CoTPromptBuilder) containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// SelectRelevantExamples 选择与当前问题相关的示例
func (b *CoTPromptBuilder) SelectRelevantExamples(question string, allExamples []Example) []Example {
	if len(allExamples) == 0 {
		return nil
	}

	questionLower := strings.ToLower(question)
	var scored []struct {
		example Example
		score   int
	}

	queryType := b.DetectQueryType(question)

	for _, ex := range allExamples {
		score := 0

		// 类型匹配加分
		exType := b.DetectQueryType(ex.Question)
		if exType == queryType {
			score += 10
		}

		// 关键词匹配
		exLower := strings.ToLower(ex.Question)
		keywords := []string{"部门", "员工", "订单", "客户", "商品", "销售", "统计", "金额"}
		for _, kw := range keywords {
			if strings.Contains(questionLower, kw) && strings.Contains(exLower, kw) {
				score += 2
			}
		}

		scored = append(scored, struct {
			example Example
			score   int
		}{ex, score})
	}

	// 按分数排序
	for i := 0; i < len(scored)-1; i++ {
		for j := i + 1; j < len(scored); j++ {
			if scored[i].score < scored[j].score {
				scored[i], scored[j] = scored[j], scored[i]
			}
		}
	}

	// 选择Top N
	count := b.config.ExampleCount
	if count > len(scored) {
		count = len(scored)
	}

	result := make([]Example, count)
	for i := 0; i < count; i++ {
		result[i] = scored[i].example
	}

	return result
}
