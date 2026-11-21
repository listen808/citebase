package text2sql

import (
	"context"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"sort"
	"strings"

	modelai "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

// Text2SQL 生成器：few-shot 示例检索与格式化。

// retrieveFewShotExamples 检索 Few-shot 示例（融合向量语义检索 + 关键词匹配 + 全局默认示例）
func (g *Generator) retrieveFewShotExamples(question string, spaceID int, maxExamples int) string {
	var results string

	// 优先使用向量语义检索
	if g.exampleVS != nil {
		ctx := context.Background()
		vectorResults, err := g.exampleVS.RetrieveRelevantExamples(ctx, question, spaceID, maxExamples)
		if err == nil && len(vectorResults) > 0 {
			logx.Infof("[FewShot] Using vector retrieval, found %d examples", len(vectorResults))
			results = g.formatVectorExamples(vectorResults)
			if results != "" {
				return results
			}
		}
	}

	// 回退到关键词匹配
	if g.exampleRepo != nil && spaceID > 0 {
		examples, err := g.exampleRepo.FindVerifiedBySpace(spaceID, maxExamples*3)
		if err == nil && len(examples) > 0 {
			results = g.matchAndFormatExamples(examples, question, maxExamples)
			if results != "" {
				return results
			}
		}
		logx.Infof("[FewShot] No verified examples found for space %d", spaceID)
	}

	// P0: 全局默认 FewShot 示例（新空间零示例时的兜底）
	defaultExamples := getDefaultFewShotExamples(question, maxExamples)
	if defaultExamples != "" {
		logx.Infof("[FewShot] Using default examples (space %d has no verified examples)", spaceID)
		return defaultExamples
	}

	return ""
}

// matchAndFormatExamples 关键词匹配 + 格式化示例
// P1-9 (§4.3.2): 增加查询类型加权（aggregation/join/cte/window）
func (g *Generator) matchAndFormatExamples(examples []*modelai.SQLExample, question string, maxExamples int) string {

	type scoredExample struct {
		example *modelai.SQLExample
		score   float64
	}
	var scored []scoredExample
	questionLower := strings.ToLower(question)
	keywords := extractKeywords(question)
	questionType := ClassifyQuery(question)

	for _, ex := range examples {
		score := 0.0
		exLower := strings.ToLower(ex.Question)

		for _, kw := range keywords {
			if strings.Contains(exLower, kw) {
				score += 2.0
			}
		}

		if strings.Contains(exLower, questionLower) || strings.Contains(questionLower, exLower) {
			score += 5.0
		}

		for _, kw := range keywords {
			if strings.Contains(strings.ToLower(ex.SQL), kw) {
				score += 1.0
			}
		}

		// P1-9: 查询类型匹配加权（问题类型与示例 SQL 类型一致时 +3.0）
		if questionType != QueryTypeBasic && questionType != QueryTypeUnknown {
			exType := inferSQLQueryType(ex.SQL)
			if exType == questionType {
				score += 3.0
			}
		}

		if score > 0 {
			scored = append(scored, scoredExample{example: ex, score: score})
		}
	}

	if len(scored) == 0 {
		if len(examples) > maxExamples {
			examples = examples[:maxExamples]
		}
		return g.formatFewShotExamples(examples)
	}

	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	if len(scored) > maxExamples {
		scored = scored[:maxExamples]
	}
	var topExamples []*modelai.SQLExample
	for _, s := range scored {
		topExamples = append(topExamples, s.example)
	}

	return g.formatFewShotExamples(topExamples)
}

// formatVectorExamples 格式化向量检索结果为 Few-shot 示例文本
func (g *Generator) formatVectorExamples(results []SQLExampleSearchResult) string {
	if len(results) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Few-Shot 示例（基于语义相似度检索）\n\n")

	for i, res := range results {
		sb.WriteString(fmt.Sprintf("### 示例 %d (相似度: %.2f)\n", i+1, res.Score))
		sb.WriteString(fmt.Sprintf("**问题**: %s\n\n", res.Question))
		sb.WriteString("**SQL**:\n```sql\n")
		sb.WriteString(res.SQL)
		sb.WriteString("\n```\n\n")
	}

	return sb.String()
}

func (g *Generator) SaveExample(question, sql string, spaceID int, userID int) error {
	if g.exampleRepo == nil {
		return fmt.Errorf("example repo not configured")
	}
	return g.exampleRepo.Create(&modelai.SQLExample{
		SpaceID:   spaceID,
		Question:  question,
		SQL:       sql,
		Verified:  0,
		CreatedBy: userID,
	})
}

// formatFewShotExamples 将示例格式化为 Few-shot Prompt 段落
func (g *Generator) formatFewShotExamples(examples []*modelai.SQLExample) string {
	if len(examples) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## 参考示例（相似问题的历史SQL，注意字段名映射方式）\n\n")
	for i, ex := range examples {
		sb.WriteString(fmt.Sprintf("### 示例 %d\n", i+1))
		sb.WriteString(fmt.Sprintf("**问题**: %s\n", ex.Question))
		// 截断过长的 SQL
		sql := ex.SQL
		fewShotMaxChars := GetText2SQLConfig().FewShotSQLMaxChars
		if len(sql) > fewShotMaxChars {
			sql = sql[:fewShotMaxChars] + "\n-- ..."
		}
		sb.WriteString(fmt.Sprintf("**SQL**:\n```sql\n%s\n```\n\n", sql))
	}
	return sb.String()
}

// SetExampleRepo 设置 Few-shot 示例仓库
func (g *Generator) SetExampleRepo(repo *airepo.SQLExampleRepo) {
	g.exampleRepo = repo
}

func (g *Generator) GetExampleRepo() *airepo.SQLExampleRepo {
	return g.exampleRepo
}

// ============================================================
// P0: 全局默认 FewShot 示例
// ============================================================

// defaultExample 预置通用示例（新空间零示例时的兜底）
type defaultExample struct {
	keywords    []string  // 匹配关键词（任一命中即匹配）
	queryType   QueryType // P1-9: 查询类型分类
	question    string
	sql         string
	explanation string
}

// getDefaultFewShotExamples 基于问题关键词返回匹配的默认示例
// P1-9 (§4.3.2): 增加查询类型过滤，优先返回与问题类型一致的示例
func getDefaultFewShotExamples(question string, maxExamples int) string {
	lowerQ := strings.ToLower(question)
	questionType := ClassifyQuery(question)

	defaults := []defaultExample{
		{
			keywords:    []string{"开头", "start", "like", "前缀", "prefix"},
			queryType:   QueryTypeBasic,
			question:    "查找字段值以某前缀开头的数据",
			sql:         "SELECT * FROM table_name WHERE field_name LIKE 'prefix%'",
			explanation: "数值类型字段直接用 LIKE（MySQL 自动隐式转换，无需 CAST）",
		},
		{
			keywords:    []string{"三天", "3天", "最近", "recent", "近几天"},
			queryType:   QueryTypeBasic,
			question:    "查找最近3天的数据",
			sql:         "SELECT * FROM table_name WHERE update_time >= DATE_SUB(NOW(), INTERVAL 3 DAY)",
			explanation: "使用 DATE_SUB + INTERVAL 做时间范围筛选",
		},
		{
			keywords:    []string{"统计", "count", "数量", "多少个", "sum", "avg", "聚合", "分组", "group"},
			queryType:   QueryTypeAggregation,
			question:    "统计某字段的分组数量",
			sql:         "SELECT field_name, COUNT(*) AS cnt FROM table_name GROUP BY field_name ORDER BY cnt DESC",
			explanation: "COUNT + GROUP BY 做分组统计，ORDER BY 排序",
		},
		{
			keywords:    []string{"关联", "join", "连接", "两个表", "多表"},
			queryType:   QueryTypeMultiJoin,
			question:    "关联两个表查询",
			sql:         "SELECT a.*, b.field FROM table_a a JOIN table_b b ON a.id = b.a_id",
			explanation: "JOIN 关联条件必须使用 ON 子句明确指定",
		},
		{
			keywords:    []string{"排序", "order", "最大", "最小", "前几", "top"},
			queryType:   QueryTypeBasic,
			question:    "按某字段排序取前N条",
			sql:         "SELECT * FROM table_name ORDER BY field_name DESC LIMIT 10",
			explanation: "ORDER BY + LIMIT 实现排序取前N条",
		},
		{
			keywords:    []string{"字段名", "列名", "映射", "实际", "DDL", "叫什么"},
			queryType:   QueryTypeBasic,
			question:    "字段名映射示例：用户说的'名称'可能是DDL中的'NAME'或'PARAM_NAME'",
			sql:         "SELECT t.PARAM_NAME AS 参数名称 FROM bm_config t WHERE t.STATUS = '1'",
			explanation: "关键：用户说的字段名可能与DDL不一致，必须对照DDL中的实际列名。用户说'名称'->DDL中实际是'PARAM_NAME'，用DDL中的名称。字符串值用单引号，数字不需要。",
		},
	}

	// 匹配关键词
	type scored struct {
		example defaultExample
		score   int
	}
	var matched []scored
	for _, d := range defaults {
		score := 0
		for _, kw := range d.keywords {
			if strings.Contains(lowerQ, kw) {
				score++
			}
		}
		// P1-9: 查询类型匹配加权（同类型 +2，强信号优先）
		if questionType != QueryTypeBasic && questionType != QueryTypeUnknown &&
			d.queryType == questionType {
			score += 2
		}
		if score > 0 {
			matched = append(matched, scored{example: d, score: score})
		}
	}

	// 按匹配分排序
	sort.Slice(matched, func(i, j int) bool { return matched[i].score > matched[j].score })

	if len(matched) == 0 {
		// P1-9: 无关键词命中时，若问题有明确类型，优先返回同类型示例
		if questionType != QueryTypeBasic && questionType != QueryTypeUnknown {
			for _, d := range defaults {
				if d.queryType == questionType {
					matched = append(matched, scored{example: d, score: 0})
				}
			}
		}
		// 仍无匹配：返回前 3 条通用示例
		if len(matched) == 0 {
			max := maxExamples
			if max > len(defaults) {
				max = len(defaults)
			}
			matched = make([]scored, max)
			for i := 0; i < max; i++ {
				matched[i] = scored{example: defaults[i], score: 0}
			}
		}
	}

	if len(matched) > maxExamples {
		matched = matched[:maxExamples]
	}

	// 格式化
	var sb strings.Builder
	sb.WriteString("## Few-Shot 示例（内置通用示例，学习字段映射和 SQL 写法）\n\n")
	sb.WriteString("**注意**：以下示例仅为 SQL 写法参考，请根据上方 DDL 中的**实际表名和字段名**替换。\n\n")
	for i, m := range matched {
		sb.WriteString(fmt.Sprintf("### 示例 %d\n", i+1))
		sb.WriteString(fmt.Sprintf("**问题**: %s\n", m.example.question))
		sb.WriteString(fmt.Sprintf("**SQL**:\n```sql\n%s\n```\n", m.example.sql))
		sb.WriteString(fmt.Sprintf("**说明**: %s\n\n", m.example.explanation))
	}
	return sb.String()
}
