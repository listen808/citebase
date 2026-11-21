package text2sql

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"sync/atomic"
	"time"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/observability"
	modelai "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/text2sql/dialect"
	"github.com/listen808/citebase/internal/text2sql/trust"
)

type Generator struct {
	kb            *KnowledgeBase
	recognizer    *EntityRecognizer
	inferer       *RelationInferer
	assembler     *SQLAssembler
	validator     *SQLValidator
	repo          *airepo.TextToSQLRepo
	promptTplRepo *airepo.PromptTemplateRepo
	llmProvider   LLMProvider
	ragAdapter    *SQLRAGAdapter
	executor      *SQLExecutor
	schemaLinker  *SchemaLinker
	schemaRepo    SchemaRepoInterface
	promptTpl     string
	exampleRepo   *airepo.SQLExampleRepo
	glossaryRepo  *airepo.BusinessGlossaryRepo
	optimizer     *SchemaOptimizer       // Schema优化器，用于精简Schema上下文
	exampleVS     *SQLExampleVectorStore // SQL示例向量存储，用于语义检索

	// 新增：准确率提升子系统
	conversationState *ConversationState  // 对话状态机
	schemaCache       *SchemaCachePool    // Schema缓存池
	sqlTemplateLib    *SQLTemplateLibrary // SQL模板库
	astCorrector      *ASTCorrector       // AST修正器
	feedbackLoop      *FeedbackLoop       // 反馈闭环

	// 新增：多轮对话持久化记忆系统
	memoryManager *ConversationMemoryManager // 对话记忆管理器（数据库+向量+记忆三层）
	// P0-2: currentConvID/currentUserID 在并发请求中被读写，
	// 改为 atomic.Uint64 避免竞态（atomic 类型不可在首次使用后复制，但 Generator 始终通过指针共享）
	currentConvID atomic.Uint64 // 当前对话ID
	currentUserID atomic.Uint64 // 当前用户ID

	// Agent 模式
	agent *Agent // ReAct Agent（可选，启用后替代传统生成）

	// 可信闭环
	explainer *trust.SQLExplainer
	auditor   *trust.SQLAuditRecorder

	// 阶段三：Trace 与语义层
	traceCollector *TraceCollector
	traceRepo      *airepo.SQLTraceRepo
	metricRepo     *airepo.SemanticMetricRepo
	dimensionRepo  *airepo.SemanticDimensionRepo
	statsRepo      *airepo.ColumnStatsRepo

	// 阶段四：Prompt 版本与 A/B 实验
	promptVersionSelector *PromptVersionSelector

	// P0-3: CoT 提示词构建器（可配置 CoT）
	coTPromptBuilder *CoTPromptBuilder

	// P0-4: 执行反馈循环（LLM 驱动的 SQL 自动修正）
	executionFeedbackLoop *ExecutionFeedbackLoop

	// P0-5: 多候选生成器（并行生成多个候选 SQL 并选优）
	multiCandidateGenerator *MultiCandidateGenerator

	// P0-6: 多跳 SQL 推理器（处理复杂跨表查询）
	multiHopReasoner *MultiHopSQLReasoner

	// P1-2: 执行计划分析器（EXPLAIN 评分）
	executionPlanAnalyzer *ExecutionPlanAnalyzer

	// P1-3: 意图-结果一致性校验器
	intentChecker *IntentChecker

	// P1-8: LLM Refiner（AST 修正失败时的 LLM 修正）
	llmRefiner *LLMRefiner

	// P2-1: 跨方言 SQL 改写器（按目标 DBType 改写非 MySQL 语法）
	dialectRewriter *DialectRewriter

	// P2-5: 多模型路由器（复杂查询走 reasoning tier）
	modelRouter ModelRouterAdapter

	// P1-9: 结构化方言（标识符引用/分页/类型映射/占位符等）
	// 为空时默认 mysql；与 dialectRewriter 互补：dialect 提供结构化能力，dialectRewriter 做字符串改写
	sqlDialect dialect.Dialect
}

// GetPromptTemplate 获取当前使用的 prompt 模板
// EstimateComplexity 估算自然语言查询的 SQL 复杂度（0-100）
func EstimateComplexity(question string) int {
	q := strings.ToLower(question)
	score := 0

	// 多表 / JOIN
	if strings.Contains(q, "join") || strings.Contains(q, "关联") || strings.Contains(q, "连接") {
		score += 20
	}
	// 子查询 / CTE
	if strings.Contains(q, "子查询") || strings.Contains(q, "嵌套") || strings.Contains(q, "cte") || strings.Contains(q, "with") {
		score += 20
	}
	// 聚合 + 分组
	if strings.Contains(q, "分组") || strings.Contains(q, "group by") || strings.Contains(q, "聚合") || strings.Contains(q, "统计") {
		score += 15
	}
	// 窗口函数
	if strings.Contains(q, "窗口") || strings.Contains(q, "over") || strings.Contains(q, "row_number") || strings.Contains(q, "rank") {
		score += 25
	}
	// 同比/环比/时间序列
	if strings.Contains(q, "同比") || strings.Contains(q, "环比") || strings.Contains(q, "趋势") || strings.Contains(q, "时间序列") {
		score += 20
	}
	// 多条件 / 多步骤
	if strings.Count(q, "和") > 2 || strings.Count(q, ",") > 2 || strings.Count(q, "、") > 2 || strings.Count(q, " and ") > 2 {
		score += 10
	}
	// 问题长度（长问题通常更复杂）
	if len(q) > 80 {
		score += 10
	}

	if score > 100 {
		return 100
	}
	return score
}

func (g *Generator) GetPromptTemplate() string {
	if g.promptTpl != "" {
		return g.promptTpl
	}
	return g.getDefaultPromptTemplate()
}

// SetPromptTemplate 设置自定义 prompt 模板
func (g *Generator) SetPromptTemplate(tpl string) {
	g.promptTpl = tpl
}

// getDefaultPromptTemplate 获取默认的 prompt 模板
func (g *Generator) getDefaultPromptTemplate() string {
	return `你是一位资深数据库专家。请根据以下表结构信息，生成准确的SQL来回答用户的问题。

## 用户问题
%s

## 表结构信息
%s

## 生成要求
1. 使用标准SQL语法
2. 仔细分析用户的业务需求，理解表之间的关联关系
3. 当用户说"两个字段值一样"时，这意味着这两个字段可以作为 JOIN 条件
4. 当用户说"包含xxx内容"时，使用 LIKE '%%xxx%%' 条件
5. 如果需要关联多个表，请使用合适的 JOIN 语法
6. 只返回SELECT查询，禁止INSERT/UPDATE/DELETE
7. **避免冗余WHERE条件**：JOIN ON条件已确保两列相等时，WHERE中只需对其中一列写过滤条件，不要重复

## 重要规则（必须严格遵守）
1. **字段归属**：每个字段必须明确指定所属表，使用 "表别名.字段名" 格式（如 p.param_val, d.PARAM_VALUE）
2. **不要混淆表**：不同表的字段不能混在一起，每个字段只能来自一个表
3. **表别名**：为每个表定义简短的别名（如 p, d），并在整个SQL中使用
4. **SELECT字段**：只选择用户需要的字段，不要列出所有字段
5. **JOIN条件**：当用户说"两个字段值一样"时，使用这两个字段作为JOIN条件
6. **WHERE条件**：当用户说"包含xxx"时，在WHERE子句中使用LIKE条件
7. **JOIN类型选择（极其重要）**：
   - 当用户要求"获取两表匹配的数据"、"值相同的数据"、"关联数据" → 使用 **INNER JOIN**（只返回两表都匹配的数据）
   - 当用户要求"列出所有数据"、"包含没有匹配的数据" → 使用 **LEFT JOIN**（保留左表所有数据）
   - 默认使用 **INNER JOIN**，除非用户明确要求保留所有数据
8. **JOIN条件简洁（极其重要）**：ON条件只包含表关联的字段（如 ON p.id = d.id），不要把数据筛选条件放到ON中，筛选条件放在WHERE中
9. **不要过度关联**：用户只提到两个表时，只JOIN这两个表，不要在ON条件中添加额外的关联条件

## JOIN类型示例
- 用户说"获取值相同的数据" → 使用 INNER JOIN，ON只包含关联字段
- 用户说"列出所有数据" → 使用 LEFT JOIN，保留左表所有数据
- 错误示例：LEFT JOIN ... ON p.a = d.a AND p.b = d.b（除非用户明确要求两个字段都关联）
- 正确示例：INNER JOIN ... ON p.a = d.a（简洁，筛选条件放WHERE）

## 思考过程（请按步骤分析）
在生成SQL前，请先完成以下分析步骤：
1. **理解需求**：用户想查询什么？需要哪些字段？筛选条件是什么？
2. **识别表**：哪些表包含所需数据？如何关联？
3. **字段映射**：用户说的字段名对应DDL中的哪个实际字段名？
4. **JOIN类型判断**：用户是要匹配的数据（INNER）还是所有数据（LEFT）？
5. **JOIN条件设计**：ON条件只包含关联字段，筛选条件放WHERE
6. **生成SQL**：基于以上分析，生成正确的SQL
7. **自检**：验证SQL是否符合所有规则，特别是JOIN类型和JOIN条件

## 输出格式
请严格按以下格式输出：

SQL:
` + "```sql" + `

SELECT 表别名1.字段1, 表别名2.字段2
FROM 表1 表别名1
INNER JOIN 表2 表别名2 ON 表别名1.关联字段 = 表别名2.关联字段
WHERE 表别名1.字段 LIKE '%%关键词%%'
` + "```" + `

解释:
...`
}

func NewGenerator(repo *airepo.TextToSQLRepo) (*Generator, error) {
	kb := NewKnowledgeBase(repo)

	if err := kb.LoadRules(); err != nil {
		return nil, fmt.Errorf("load rules: %w", err)
	}

	caches, err := repo.GetAllTableCaches()
	if err != nil {
		return nil, fmt.Errorf("load table caches: %w", err)
	}
	for i := range caches {
		kb.tables[caches[i].TableNam] = &caches[i]
	}

	relations, err := repo.GetAllRelations()
	if err != nil {
		return nil, fmt.Errorf("load relations: %w", err)
	}
	for _, rel := range relations {
		kb.rels[rel.SourceTable] = append(kb.rels[rel.SourceTable], rel)
	}

	g := &Generator{
		kb:                kb,
		recognizer:        NewEntityRecognizer(kb),
		inferer:           NewRelationInferer(kb),
		assembler:         NewSQLAssembler(kb),
		validator:         NewSQLValidator(kb),
		repo:              repo,
		llmProvider:       globalLLMProvider,
		exampleRepo:       airepo.NewSQLExampleRepo(),
		glossaryRepo:      airepo.NewBusinessGlossaryRepo(),
		optimizer:         NewSchemaOptimizer(),
		conversationState: NewConversationState(0, 0),
		schemaCache:       NewSchemaCachePool(0, 0),
		sqlTemplateLib:    NewSQLTemplateLibrary(),
		astCorrector:      NewASTCorrector(),
		feedbackLoop:      NewFeedbackLoop(),
		coTPromptBuilder:  NewCoTPromptBuilder(),
	}

	// P0-11: 当真实 provider 可用时，自动创建 Agent（支持流式输出）
	if globalRealProvider != nil {
		g.agent = NewAgent(globalRealProvider, g)
		logx.Info("[Text2SQL] Agent initialized with streaming support")
	}

	// P2-5: 注入全局模型路由器（复杂查询走 reasoning tier）
	if globalModelRouter != nil {
		g.modelRouter = globalModelRouter
		logx.Info("[Text2SQL] Model router injected (P2-5 reasoning routing)")
	}

	return g, nil
}

// GetAgent 获取 Agent 实例
func (g *Generator) GetAgent() *Agent {
	return g.agent
}

// GetStreamHandler 获取流式处理器（P0-11: Agent SSE 流式输出）
// 当 Agent 可用时返回 StreamHandler，否则返回 nil
func (g *Generator) GetStreamHandler() *StreamHandler {
	if g.agent == nil {
		return nil
	}
	return NewStreamHandler(g.agent)
}

// getExecutionFeedbackLoop 懒初始化执行反馈循环
// P0-4: 当配置启用且 provider/executor 就绪时创建
func (g *Generator) getExecutionFeedbackLoop() *ExecutionFeedbackLoop {
	if g.executionFeedbackLoop == nil && g.llmProvider != nil && g.executor != nil {
		g.executionFeedbackLoop = NewExecutionFeedbackLoop(g, g.executor, g.llmProvider)
	}
	return g.executionFeedbackLoop
}

// getMultiCandidateGenerator 懒初始化多候选生成器
// P0-5: 当配置启用且 provider 就绪时创建
func (g *Generator) getMultiCandidateGenerator() *MultiCandidateGenerator {
	if g.multiCandidateGenerator == nil && g.llmProvider != nil {
		g.multiCandidateGenerator = NewMultiCandidateGenerator(g.llmProvider, g.executor)
	}
	return g.multiCandidateGenerator
}

// getMultiHopReasoner 懒初始化多跳 SQL 推理器
// P0-6: 当 provider 就绪时创建（使用当前 kb）
func (g *Generator) getMultiHopReasoner() *MultiHopSQLReasoner {
	if g.multiHopReasoner == nil && g.llmProvider != nil {
		g.multiHopReasoner = NewMultiHopSQLReasoner(g.llmProvider, g.ragAdapter, g.kb)
	}
	return g.multiHopReasoner
}

// getExecutionPlanAnalyzer 懒初始化执行计划分析器
// P1-2: 当 executor 就绪时创建
func (g *Generator) getExecutionPlanAnalyzer() *ExecutionPlanAnalyzer {
	if g.executionPlanAnalyzer == nil && g.executor != nil {
		g.executionPlanAnalyzer = NewExecutionPlanAnalyzer(g.executor)
	}
	return g.executionPlanAnalyzer
}

// getIntentChecker 懒初始化意图校验器
// P1-3: 当 provider 就绪时创建
func (g *Generator) getIntentChecker() *IntentChecker {
	if g.intentChecker == nil && g.llmProvider != nil {
		g.intentChecker = NewIntentChecker(g.llmProvider)
	}
	return g.intentChecker
}

// getLLMRefiner 懒初始化 LLM 修正器
// P1-8: 当 provider 就绪时创建
func (g *Generator) getLLMRefiner() *LLMRefiner {
	if g.llmRefiner == nil && g.llmProvider != nil {
		g.llmRefiner = NewLLMRefiner(g.llmProvider)
	}
	return g.llmRefiner
}

// SetDialect P2-1: 设置目标方言，触发方言改写器初始化
// dialect 可选：mysql（默认）/ postgres / oracle / sqlite
// P1-9: 同时初始化结构化方言 sqlDialect，提供 QuoteIdentifier/Paginate/TypeMapping 等能力
func (g *Generator) SetDialect(dialectName string) {
	g.dialectRewriter = NewDialectRewriter(dialectName)
	g.sqlDialect = dialect.GetDialect(dialectName)
}

// GetDialectRewriter 返回当前方言改写器（可能为 nil）
func (g *Generator) GetDialectRewriter() *DialectRewriter {
	return g.dialectRewriter
}

// SetModelRouter P2-5: 注入多模型路由器（复杂查询走 reasoning tier）
func (g *Generator) SetModelRouter(router ModelRouterAdapter) {
	g.modelRouter = router
}

// GetModelRouter 返回当前模型路由器（可能为 nil）
func (g *Generator) GetModelRouter() ModelRouterAdapter {
	return g.modelRouter
}

// selectProviderForQuery P2-5: 根据查询复杂度选择 LLM provider
// 复杂查询（complexity >= 50）走 reasoning tier，否则用默认 llmProvider
func (g *Generator) selectProviderForQuery(text string) LLMProvider {
	if g.modelRouter == nil {
		return g.llmProvider
	}
	complexity := EstimateComplexity(text)
	if complexity >= 50 {
		p, _ := g.modelRouter.SelectReasoning()
		if p != nil {
			logx.Infof("[Text2SQL] P2-5: Complex query (complexity=%d), routing to reasoning provider", complexity)
			return p
		}
	}
	return g.llmProvider
}

// GenerateWithAgent 使用 Agent 模式生成 SQL（ReAct + Function Calling）
func (g *Generator) GenerateWithAgent(ctx context.Context, text string, spaceID uint64, cfg *AgentReActConfig) (*AgentResult, error) {
	if g.agent == nil {
		return nil, fmt.Errorf("Agent 未初始化，请先调用 SetAgent")
	}

	// 加载空间知识库
	kb := NewKnowledgeBase(g.repo)
	if g.schemaRepo != nil {
		kb.SetSchemaRepo(g.schemaRepo)
	}
	if err := kb.LoadFromSpace(spaceID); err != nil {
		return nil, fmt.Errorf("加载空间知识库失败: %w", err)
	}

	// 补充全局表缓存
	if len(kb.GetAllTables()) == 0 {
		caches, err := g.repo.GetAllTableCaches()
		if err == nil {
			for i := range caches {
				kb.tables[caches[i].TableNam] = &caches[i]
			}
		}
	}

	// LoadFromSpace 已统一从 aw_schema_* 加载（通过 KnowledgeBase.schemaRepo）

	// 更新 Agent 使用的知识库 + 该请求专属的会话状态
	// P1 首要任务 2：请求级上下文改经 ctx 显式传递，不再加锁替换 Generator.kb /
	// Agent.conversationState 这类进程级字段（旧 BindRequestScope 通道会让并发
	// 请求 B 在生成中途读到请求 A 刚换入的 kb/state，构成跨请求借用）。
	ctx = WithRequestScope(ctx, &RequestScope{
		UserID:  UserIDFromContext(ctx),
		SpaceID: spaceID,
		KB:      kb,
		State:   ConversationStateFor(int(g.currentConvID.Load()), UserIDFromContext(ctx), int(spaceID)),
	})

	return g.agent.Generate(ctx, text, spaceID, cfg)
}

// GenerateContext 带请求上下文版本：ctx 携带真实调用者身份，
// 供 provider 层做 Token 配额预检。
func (g *Generator) GenerateContext(ctx context.Context, text string) (*modelai.SQLResult, error) {
	// P2-2: 记录生成指标
	genStart := time.Now()
	var genSuccess bool
	var genRetries int
	defer func() {
		RecordGeneration(0, time.Since(genStart), genSuccess, genRetries, 0)
	}()

	// 阶段三：创建 Trace 收集器
	var traceCollector *TraceCollector
	if g.traceRepo != nil && GetText2SQLConfig().EnableTrace {
		traceCollector = NewTraceCollector(0, int(g.currentConvID.Load()), g.traceRepo)
		g.traceCollector = traceCollector
	}

	// 阶段一：统一自然语言入口记录对话轮次
	g.conversationState.RecordTurn(text, "", nil)

	// 检测用户意图
	intent := g.conversationState.DetectIntent(text)

	// 关键修复：先记录意图，再调用 generateWithLLM
	// 这样 generateWithLLM 内部获取的就是当前轮的意图
	g.conversationState.RecordIntent(intent)

	// 解析引用
	resolvedText := g.conversationState.ResolveReference(text)

	// 阶段二：SQL 模板优先生成（简单意图直接走模板，降低 LLM 幻觉）
	if templateSQL, score := g.tryTemplateGenerate(resolvedText, intent.IntentType, g.kb, 0); templateSQL != "" {
		var knownTables []string
		for name := range g.kb.GetAllTables() {
			knownTables = append(knownTables, name)
		}
		result := &modelai.SQLResult{
			SQL:      templateSQL,
			Tables:   extractTablesFromSQL(templateSQL, knownTables),
			Warnings: []string{fmt.Sprintf("模板生成（匹配度 %.0f%%）", score*100)},
		}
		if len(g.conversationState.SQLHistory) > 0 {
			g.conversationState.SQLHistory[len(g.conversationState.SQLHistory)-1].GeneratedSQL = result.SQL
			g.conversationState.CurrentSQL = result.SQL
		}
		// 阶段三：记录 Trace
		if traceCollector != nil {
			traceCollector.Flush(text, intent.IntentType, result.SQL, false)
			result.TraceID = traceCollector.TraceID()
		}
		return result, nil
	}

	// 如果有 LLM provider，优先使用 LLM 生成（使用全部表知识库）
	if g.llmProvider != nil {
		result, err := g.generateWithLLM(ctx, resolvedText, g.kb, 0)
		if err != nil {
			return nil, err
		}

		// 记录生成的SQL到对话状态
		if len(g.conversationState.SQLHistory) > 0 {
			g.conversationState.SQLHistory[len(g.conversationState.SQLHistory)-1].GeneratedSQL = result.SQL
			g.conversationState.CurrentSQL = result.SQL
		}

		// 保存当前轮次到数据库和向量库（强制持久化）
		if g.memoryManager != nil && g.currentConvID.Load() > 0 {
			_, saveErr := g.SaveCurrentTurn(
				text,         // userQuery
				resolvedText, // resolvedQuery
				intent.IntentType,
				result.SQL,
				"", // explanation（可从result中提取）
				result.Tables,
				nil,   // fields（可从result中提取）
				false, // isSuccess（执行后更新）
				"",    // executionError
			)
			if saveErr != nil {
				logx.Infof("[Generator] Save turn failed: %v", saveErr)
			}
		}

		// 缓存使用的表结构
		for _, table := range result.Tables {
			if tableCache := g.kb.GetTable(table); tableCache != nil {
				columns := make([]ColumnInfo, len(tableCache.ColumnsJSON))
				for i, col := range tableCache.ColumnsJSON {
					columns[i] = ColumnInfo{
						Name:     col.Name,
						Type:     col.Type,
						Nullable: col.IsNullable,
						Comment:  col.Comment,
					}
				}
				g.conversationState.RecordTableUsage(table, columns)
				g.schemaCache.CacheTable(tableRowSpaceID(tableCache.SpaceID), table, columns, false)
			}
		}

		// 阶段三：记录 Trace
		if traceCollector != nil {
			execOK := result.ExecutionResult != nil
			traceCollector.Flush(text, intent.IntentType, result.SQL, execOK)
			result.TraceID = traceCollector.TraceID()
		}
		genSuccess = true
		return result, nil
	}

	// 规则路径 (fallback)
	input := g.recognizer.Recognize(resolvedText)

	if len(input.Tables) == 0 {
		return nil, fmt.Errorf("未能识别出任何表，请检查输入是否包含表名或业务术语")
	}

	joins := g.inferer.InferJoins(input.Tables)

	sql := g.assembler.Assemble(input, joins)

	validation := g.validator.Validate(sql, input)
	if !validation.Valid {
		return nil, fmt.Errorf("SQL验证失败: %v", validation.Errors)
	}

	result := &modelai.SQLResult{
		SQL:      sql,
		Tables:   input.Tables,
		Entities: g.convertEntities(input),
		Warnings: validation.Warnings,
	}

	for _, join := range joins {
		result.Relations = append(result.Relations, modelai.TableRelation{
			SourceTable: join.SourceTable,
			SourceField: join.SourceField,
			TargetTable: join.TargetTable,
			TargetField: join.TargetField,
		})
	}

	result.Explanations = g.generateExplanations(input, joins)

	// 记录生成的SQL到对话状态
	if len(g.conversationState.SQLHistory) > 0 {
		g.conversationState.SQLHistory[len(g.conversationState.SQLHistory)-1].GeneratedSQL = result.SQL
		g.conversationState.CurrentSQL = result.SQL
	}

	// 记录意图
	g.conversationState.RecordIntent(intent)

	// 缓存使用的表结构
	for _, table := range result.Tables {
		if tableCache := g.kb.GetTable(table); tableCache != nil {
			columns := make([]ColumnInfo, len(tableCache.ColumnsJSON))
			for i, col := range tableCache.ColumnsJSON {
				columns[i] = ColumnInfo{
					Name:     col.Name,
					Type:     col.Type,
					Nullable: col.IsNullable,
					Comment:  col.Comment,
				}
			}
			g.conversationState.RecordTableUsage(table, columns)
			g.schemaCache.CacheTable(tableRowSpaceID(tableCache.SpaceID), table, columns, false)
		}
	}

	// 阶段三：记录 Trace
	if traceCollector != nil {
		traceCollector.Flush(text, intent.IntentType, result.SQL, false)
		result.TraceID = traceCollector.TraceID()
	}

	return result, nil
}

// Generate 兼容包装：无请求上下文，等价于 GenerateContext(context.Background(), text)。
// 保留此签名是因为部分内部调用方（TextToSQLGenerateSQL 等）尚未接上请求 ctx；
// 新调用方应使用 GenerateContext。
func (g *Generator) Generate(text string) (*modelai.SQLResult, error) {
	return g.GenerateContext(context.Background(), text)
}

// GenerateWithSpaceContext 带请求上下文版本：ctx 携带真实调用者身份
// （provider.ContextWithUserID），供 provider 层做 Token 配额预检。
func (g *Generator) GenerateWithSpaceContext(ctx context.Context, text string, spaceID uint64) (*modelai.SQLResult, error) {
	// 阶段三：创建 Trace 收集器
	var traceCollector *TraceCollector
	if g.traceRepo != nil && GetText2SQLConfig().EnableTrace {
		traceCollector = NewTraceCollector(spaceID, int(g.currentConvID.Load()), g.traceRepo)
		g.traceCollector = traceCollector
	}

	// 记录对话轮次
	g.conversationState.RecordTurn(text, "", nil)

	// 检测用户意图
	intent := g.conversationState.DetectIntent(text)

	// 关键修复：先记录意图，再调用 generateWithLLM
	g.conversationState.RecordIntent(intent)

	// 解析引用
	resolvedText := g.conversationState.ResolveReference(text)

	kb := NewKnowledgeBase(g.repo)
	if g.schemaRepo != nil {
		kb.SetSchemaRepo(g.schemaRepo)
	}
	if err := kb.LoadFromSpace(spaceID); err != nil {
		return nil, fmt.Errorf("加载空间知识库失败: %w", err)
	}

	spaceTableCount := len(kb.GetAllTables())

	if spaceTableCount == 0 {
		schemaCount := g.LoadFromSchemaSpace(kb, int(spaceID))
		if schemaCount > 0 {
			spaceTableCount = schemaCount
		}
	}

	if spaceTableCount == 0 {
		caches, err := g.repo.GetAllTableCaches()
		if err != nil {
			return nil, fmt.Errorf("加载全部表缓存失败: %w", err)
		}
		for i := range caches {
			kb.tables[caches[i].TableNam] = &caches[i]
		}
	} else {
		keywords := extractKeywords(resolvedText)
		questionTables := g.recognizer.ExtractTableNames(resolvedText)
		missingTables := make(map[string]bool)
		for _, t := range questionTables {
			if kb.GetTable(t) == nil {
				lowerT := strings.ToLower(t)
				found := false
				for name := range kb.GetAllTables() {
					if strings.ToLower(name) == lowerT {
						found = true
						break
					}
				}
				if !found {
					missingTables[t] = true
				}
			}
		}
		for _, kw := range keywords {
			if len(kw) < 3 {
				continue
			}
			kwLower := strings.ToLower(kw)
			found := false
			for name := range kb.GetAllTables() {
				if strings.Contains(strings.ToLower(name), kwLower) || strings.Contains(kwLower, strings.ToLower(name)) {
					found = true
					break
				}
			}
			if !found {
				missingTables[kw] = true
			}
		}

		if len(missingTables) > 0 {
			allCaches, err := g.repo.GetAllTableCaches()
			if err == nil {
				for i := range allCaches {
					nameLower := strings.ToLower(allCaches[i].TableNam)
					for mt := range missingTables {
						mtLower := strings.ToLower(mt)
						if strings.Contains(nameLower, mtLower) || strings.Contains(mtLower, nameLower) {
							if kb.GetTable(allCaches[i].TableNam) == nil {
								kb.tables[allCaches[i].TableNam] = &allCaches[i]
							}
						}
					}
				}
			}
		}
	}

	if err := kb.LoadRules(); err != nil {
		return nil, fmt.Errorf("加载规则失败: %w", err)
	}

	resolvedText = sanitizeQuestionFields(resolvedText, kb)
	resolvedText = g.applyGlossaryMapping(resolvedText, int(spaceID))

	// 阶段二：SQL 模板优先生成（简单意图直接走模板，降低 LLM 幻觉）
	if templateSQL, score := g.tryTemplateGenerate(resolvedText, intent.IntentType, kb, int(spaceID)); templateSQL != "" {
		var knownTables []string
		for name := range kb.GetAllTables() {
			knownTables = append(knownTables, name)
		}
		result := &modelai.SQLResult{
			SQL:      templateSQL,
			Tables:   extractTablesFromSQL(templateSQL, knownTables),
			Warnings: []string{fmt.Sprintf("模板生成（匹配度 %.0f%%）", score*100)},
		}
		if len(g.conversationState.SQLHistory) > 0 {
			g.conversationState.SQLHistory[len(g.conversationState.SQLHistory)-1].GeneratedSQL = result.SQL
			g.conversationState.CurrentSQL = result.SQL
		}
		for _, table := range result.Tables {
			if tableCache := kb.GetTable(table); tableCache != nil {
				columns := make([]ColumnInfo, len(tableCache.ColumnsJSON))
				for i, col := range tableCache.ColumnsJSON {
					columns[i] = ColumnInfo{
						Name:     col.Name,
						Type:     col.Type,
						Nullable: col.IsNullable,
						Comment:  col.Comment,
					}
				}
				g.conversationState.RecordTableUsage(table, columns)
				g.schemaCache.CacheTable(tableRowSpaceID(tableCache.SpaceID), table, columns, false)
			}
		}
		// 阶段三：记录 Trace
		if traceCollector != nil {
			traceCollector.Flush(text, intent.IntentType, result.SQL, false)
			result.TraceID = traceCollector.TraceID()
		}
		return result, nil
	}

	var result *modelai.SQLResult
	var err error

	if g.llmProvider != nil {
		result, err = g.generateWithLLM(ctx, resolvedText, kb, int(spaceID))
	} else {
		result, err = g.generateWithRules(resolvedText, kb)
	}

	if err != nil {
		return nil, err
	}

	// 记录生成的SQL到对话状态
	if len(g.conversationState.SQLHistory) > 0 {
		g.conversationState.SQLHistory[len(g.conversationState.SQLHistory)-1].GeneratedSQL = result.SQL
		g.conversationState.CurrentSQL = result.SQL
	}

	// 记录意图
	g.conversationState.RecordIntent(intent)

	// 缓存使用的表结构
	for _, table := range result.Tables {
		if tableCache := kb.GetTable(table); tableCache != nil {
			columns := make([]ColumnInfo, len(tableCache.ColumnsJSON))
			for i, col := range tableCache.ColumnsJSON {
				columns[i] = ColumnInfo{
					Name:     col.Name,
					Type:     col.Type,
					Nullable: col.IsNullable,
					Comment:  col.Comment,
				}
			}
			g.conversationState.RecordTableUsage(table, columns)
			g.schemaCache.CacheTable(tableRowSpaceID(tableCache.SpaceID), table, columns, false)
		}
	}

	// 阶段三：记录 Trace
	if traceCollector != nil {
		execOK := result.ExecutionResult != nil
		traceCollector.Flush(text, intent.IntentType, result.SQL, execOK)
		result.TraceID = traceCollector.TraceID()
	}

	return result, nil
}

// GenerateWithSpace 兼容包装：无请求上下文，等价于
// GenerateWithSpaceContext(context.Background(), text, spaceID)。
// 保留此签名是因为部分内部调用方（TextToSQLGenerateSQL 等）尚未接上请求 ctx；
// 新调用方应使用 GenerateWithSpaceContext。
func (g *Generator) GenerateWithSpace(text string, spaceID uint64) (*modelai.SQLResult, error) {
	return g.GenerateWithSpaceContext(context.Background(), text, spaceID)
}

// tryTemplateGenerate 尝试用 SQL 模板库直接生成 SQL
// spaceID：只允许消费该空间分片内的 schema 缓存，跨空间表名/字段不得进入模板上下文。
func (g *Generator) tryTemplateGenerate(text string, intent string, kb *KnowledgeBase, spaceID int) (string, float64) {
	if g.sqlTemplateLib == nil || kb == nil {
		return "", 0
	}

	var tables []string
	if g.schemaCache != nil {
		// P0 安全修复：必须走带锁访问器取表名。直接 `range g.schemaCache.Tables`
		// 与并发的 CacheTable/AddRelation 写操作构成 concurrent map read/write，
		// 会让进程直接 fatal（runtime throw），而不是 panic 可恢复。
		tables = g.schemaCache.GetCachedTables(spaceID)
	}
	if len(tables) == 0 {
		for name := range kb.GetAllTables() {
			tables = append(tables, name)
		}
	}
	if len(tables) == 0 {
		return "", 0
	}

	var fields []string
	seenFields := make(map[string]bool)
	for _, table := range tables {
		if cached, ok := g.schemaCache.GetCachedTable(spaceID, table); ok && len(cached.Columns) > 0 {
			for _, col := range cached.Columns {
				if !seenFields[col.Name] {
					fields = append(fields, col.Name)
					seenFields[col.Name] = true
				}
			}
		} else if tc := kb.GetTable(table); tc != nil && len(tc.ColumnsJSON) > 0 {
			for _, col := range tc.ColumnsJSON {
				if !seenFields[col.Name] {
					fields = append(fields, col.Name)
					seenFields[col.Name] = true
				}
			}
		}
	}

	return g.sqlTemplateLib.ApplyTemplate(text, intent, tables, fields, kb)
}

// generateWithLLM 使用 LLM 生成 SQL（含 RAG 向量检索、多模板、Self-Reflection、结构校验、重试）
// P1 首要任务 1：接收请求 ctx，使自动试执行/自纠错能沿 ctx 拿到真实调用者身份
// （旧实现内部一律 context.Background()，执行入口只能以 userID=0 被 fail-closed 拒绝）。
func (g *Generator) generateWithLLM(ctx context.Context, text string, kb *KnowledgeBase, spaceID int) (*modelai.SQLResult, error) {
	startTime := time.Now()
	if ctx == nil {
		ctx = context.Background()
	}
	obsProvider := observability.DefaultProvider()
	ctx, rootSpan := obsProvider.StartSpan(ctx, "text2sql.generate",
		observability.WithService("text2sql"),
		observability.WithAttributes(map[string]interface{}{
			"question": text,
			"space_id": spaceID,
		}),
	)
	defer func() {
		obsProvider.ObserveHistogram(observability.MetricAIRequestDuration, map[string]string{"service": "text2sql"}, float64(time.Since(startTime).Milliseconds()))
		obsProvider.AddGauge(observability.MetricAIActiveRequests, map[string]string{"service": "text2sql"}, -1)
	}()
	obsProvider.Inc(observability.MetricAIRequestsTotal, map[string]string{"service": "text2sql"}, 1)
	obsProvider.AddGauge(observability.MetricAIActiveRequests, map[string]string{"service": "text2sql"}, 1)

	// 入口统一预清理字段名
	text = sanitizeQuestionFields(text, kb)
	// 业务术语确定性替换（Glossary 映射）
	text = g.applyGlossaryMapping(text, spaceID)
	if len(kb.GetAllTables()) == 0 {
		obsProvider.EndSpan(rootSpan, observability.SpanStatusError)
		rootSpan.ErrorMsg = "知识库中没有任何表结构"
		return nil, fmt.Errorf("知识库中没有任何表结构，请先通过「Schema导入」功能导入DDL，或调用 POST /api/v1/text2sql/scan-docs 扫描文档中的建表语句")
	}
	qType := ClassifyQuery(text)

	var schemaContext string
	var relationsContext string

	_, schemaLinkSpan := obsProvider.StartSpan(ctx, "schema_link", observability.WithService("text2sql"))
	// 优先使用Schema优化器构建精简上下文
	if g.optimizer != nil {
		keywords := extractKeywords(text)
		// P0增强：提取用户显式提到的表名，强制纳入Schema上下文
		mentionedTables := extractMentionedTables(text, kb)
		mentionedNames := make([]string, 0, len(mentionedTables))
		for _, mt := range mentionedTables {
			mentionedNames = append(mentionedNames, mt.TableNam)
		}
		schemaContext = g.optimizer.BuildOptimizedContext(text, kb.GetAllTables(), keywords, mentionedNames...)
		relationsContext = g.buildRelationsContext(kb)
	} else if g.schemaLinker != nil && spaceID > 0 {
		// 保留原有逻辑作为fallback
		schemaSpaceID := g.resolveSchemaSpaceID(spaceID)
		linked, err := g.schemaLinker.Link(ctx, text, schemaSpaceID)
		if err == nil && len(linked.Tables) > 0 {
			schemaContext = g.buildLinkedSchemaContext(linked)
			relationsContext = g.buildLinkedRelationsContext(linked)
		} else {
			schemaContext = g.buildSchemaContext(text, kb)
			relationsContext = g.buildRelationsContext(kb)
		}
	} else if g.ragAdapter != nil {
		// 次优先：RAG 向量检索构建 Schema 上下文
		ragSources := g.ragAdapter.RetrieveRelevantSchemas(text, spaceID, 10)
		schemaContext = g.ragAdapter.BuildSchemaContext(kb, ragSources, text)
		relationsContext = g.buildRelationsContext(kb)
	} else {
		// 无 RAG 时使用关键词匹配 + 硬截断
		schemaContext = g.buildSchemaContext(text, kb)
		relationsContext = g.buildRelationsContext(kb)
	}

	// 阶段三：注入语义层与列统计上下文
	semanticContext := ""
	if spaceID > 0 {
		semanticContext = g.buildSemanticContext(uint64(spaceID), kb)
	}
	obsProvider.EndSpan(schemaLinkSpan, observability.SpanStatusOK)
	obsProvider.SetAttribute(schemaLinkSpan, "schema_context_len", len(schemaContext))

	// Trace: Schema Linking 完成
	g.traceStep("schema_linking", text, schemaContext, "ok", "")

	var schemaTables []string
	for name := range kb.GetAllTables() {
		schemaTables = append(schemaTables, name)
	}

	// P0-6/P0-5: 预生成 SQL（多跳推理优先，多候选作为备选）
	var preGeneratedSQL string
	var preGeneratedWarnings []string

	// P0-6: 多跳推理（当配置启用且复杂度达标时，优先使用多跳推理处理复杂跨表查询）
	if GetText2SQLConfig().EnableMultiHop && EstimateComplexity(text) >= GetText2SQLConfig().MultiHopComplexityThreshold {
		mhr := g.getMultiHopReasoner()
		if mhr != nil {
			mhResult, mhErr := mhr.Reason(ctx, text, spaceID)
			if mhErr == nil && mhResult != nil && mhResult.FinalSQL != "" {
				preGeneratedSQL = mhResult.FinalSQL
				preGeneratedWarnings = []string{fmt.Sprintf("多跳推理（%d跳）", mhResult.TotalHops)}
				g.traceStep("multi_hop", text, fmt.Sprintf("hops=%d final_sql_len=%d", mhResult.TotalHops, len(mhResult.FinalSQL)), "ok", "")
			} else if mhErr != nil {
				logx.Infof("[Text2SQL] Multi-hop reasoning failed, falling back: %v", mhErr)
			}
		}
	}

	// P0-5: 多候选并行生成（当多跳未生成 SQL 且配置启用时，并行生成多个候选选优）
	// P1-1: 当候选数 > 1 时启用 Self-Consistency 投票
	if preGeneratedSQL == "" && GetText2SQLConfig().EnableMultiCandidate {
		mcg := g.getMultiCandidateGenerator()
		if mcg != nil {
			best, candidates, mcErr := mcg.GenerateBestWithCandidates(ctx, text, schemaContext, kb.tables)
			if mcErr == nil && best != nil && best.SQL != "" {
				// P1-1: Self-Consistency 投票（候选数 > 1 时）
				if len(candidates) > 1 {
					voter := NewSelfConsistencyVoter()
					winner, groups := voter.Vote(candidates)
					if winner != nil && winner.Best != nil {
						best = winner.Best
						preGeneratedWarnings = []string{fmt.Sprintf("多候选投票（%d候选→%d组，胜出票数%d，置信度%.0f%%）", len(candidates), len(groups), winner.VoteCount, best.Confidence*100)}
						g.traceStep("self_consistency_vote", text, fmt.Sprintf("candidates=%d groups=%d winner_votes=%d", len(candidates), len(groups), winner.VoteCount), "ok", "")
					} else {
						preGeneratedWarnings = []string{fmt.Sprintf("多候选生成（%d个候选，置信度%.0f%%，策略：%s）", len(candidates), best.Confidence*100, best.Strategy)}
					}
				} else {
					preGeneratedWarnings = []string{fmt.Sprintf("多候选生成（%d个候选，置信度%.0f%%，策略：%s）", len(candidates), best.Confidence*100, best.Strategy)}
				}
				preGeneratedSQL = best.SQL
				g.traceStep("multi_candidate", text, fmt.Sprintf("candidates=%d best_confidence=%.2f strategy=%s", len(candidates), best.Confidence, best.Strategy), "ok", "")
			} else if mcErr != nil {
				logx.Infof("[Text2SQL] Multi-candidate generation failed, falling back to single LLM call: %v", mcErr)
			}
		}
	}

	maxRetries := GetText2SQLConfig().MaxRetries
	var lastSQL string
	var lastExecError string

	for retry := 0; retry < maxRetries; retry++ {
		tpl, ok := g.getPromptTemplateFromDB(qType, spaceID)
		if !ok {
			tpl = GetPromptTemplate(qType)
		}

		// 阶段四：Prompt 版本/A/B 实验选择器覆盖模板
		if g.promptVersionSelector != nil && spaceID > 0 {
			if pv, err := g.promptVersionSelector.Select(uint64(spaceID), qType.String(), fmt.Sprintf("%d", g.currentUserID.Load())); err == nil && pv != nil {
				if pv.SystemPrompt != "" {
					tpl.SystemPrompt = pv.SystemPrompt
				}
				if pv.UserTemplate != "" {
					tpl.UserTemplate = pv.UserTemplate
				}
			}
		}

		prompt := strings.ReplaceAll(strings.ReplaceAll(tpl.UserTemplate, "{question}", text), "{schema_context}", schemaContext)
		prompt = strings.ReplaceAll(prompt, "{relations}", relationsContext)
		if semanticContext != "" {
			prompt = strings.ReplaceAll(prompt, "{semantic_context}", semanticContext)
		}

		// 注入对话上下文（多轮对话记忆）
		// 增强：根据意图类型选择上下文注入方式
		intentType := "new_query"
		if len(g.conversationState.IntentHistory) > 0 {
			lastIntent := g.conversationState.IntentHistory[len(g.conversationState.IntentHistory)-1]
			intentType = lastIntent.IntentType
		}
		if convContext := g.conversationState.GetContextForLLM(intentType); convContext != "" {
			prompt = convContext + "\n\n" + prompt
		}

		// 如果意图是修改/跟进，额外注入修改指令上下文
		if intentType == "follow_up" || intentType == "modify" {
			if modContext := g.conversationState.BuildModificationContext(text); modContext != "" {
				prompt = modContext + "\n\n" + prompt
			}

			// 强制约束：当上一轮SQL存在时，在prompt末尾添加强制保留指令
			lastTurn := g.conversationState.GetLastTurn()
			if lastTurn != nil && lastTurn.GeneratedSQL != "" {
				forceKeep := fmt.Sprintf(`

## 【强制约束】基于上一轮SQL修改

上一轮SQL：
%s

**绝对规则**：
1. 你必须基于上述SQL进行修改，不是重新生成
2. 用户没有要求修改的部分，必须一字不差保留原样
3. 只修改用户明确要求的部分
4. 如果用户说"列出所有数据"，SELECT改为 `+"`SELECT *`"+` 或列出所有字段
5. 如果用户说"保持where"，WHERE子句完全保留，不要删除或修改
6. 最终SQL必须包含上一轮SQL的所有部分（FROM、JOIN、WHERE等），除非用户明确要求删除某部分
`, lastTurn.GeneratedSQL)
				prompt = prompt + forceKeep
			}
		}

		// 注入 Few-shot 示例（Vanna 核心策略：相似问题的历史SQL作为参考）
		if retry == 0 {
			fewShotBlock := g.retrieveFewShotExamples(text, spaceID, GetText2SQLConfig().FewShotCount)
			if fewShotBlock != "" {
				prompt = fewShotBlock + "\n\n" + prompt
			}
		}

		if retry > 0 && lastSQL != "" {
			issues := ValidateSQLStructure(lastSQL, schemaTables)
			if len(issues) > 0 {
				var feedback []string
				for _, iss := range issues {
					feedback = append(feedback, fmt.Sprintf("- %s: %s", iss.Type, iss.Message))
				}
				prompt += "\n\n## Previous SQL Issues\nPrevious attempt generated invalid SQL:\n" +
					strings.Join(feedback, "\n") + "\n\nPlease fix ALL issues above.\n"
			}
			if lastExecError != "" {
				prompt += fmt.Sprintf("\n\n## SQL Execution Error\nThe previous SQL failed when executed on the database:\nError: %s\n\nPrevious SQL:\n```sql\n%s\n```\n\nPlease fix the SQL based on the execution error above.\n", lastExecError, lastSQL)
				lastExecError = ""
			}
		}

		// 注入字段名对照（如果检测到不匹配）
		fieldMappings := detectMissingFields(text, kb)
		if fieldMappings != "" {
			prompt = fieldMappings + prompt
		}

		messages := []Message{
			NewTextMessage("user", prompt),
		}

		// 增强系统提示词：加入分析表结构的指引
		enhancedSystemPrompt := tpl.SystemPrompt
		if tpl.SystemPrompt != "" {
			enhancedSystemPrompt += "\n\n先仔细分析表结构（字段类型、主键、约束、注释、表间关系），理解字段含义后再生成准确高效的SQL。如果用户提到了字段名，务必从DDL中查找实际字段名，用户提到的字段名可能不准确。"
		}
		// P1（清单 117）：这里原先重新 `ctx := context.Background()` 遮蔽了外层请求 ctx，
		// 导致 LLM 调用与试执行无法随客户端断开/超时取消；现直接沿用语义正确的 ctx。

		// P0-5: 首轮优先使用多候选生成的 SQL（跳过 LLM 调用）
		var result *modelai.SQLResult
		if retry == 0 && preGeneratedSQL != "" {
			result = &modelai.SQLResult{
				SQL:      preGeneratedSQL,
				Tables:   extractTablesFromSQL(preGeneratedSQL, schemaTables),
				Warnings: preGeneratedWarnings,
			}
			preGeneratedSQL = "" // 只用一次，失败后走正常 LLM 重试
			g.traceStep("llm_call", "multi_candidate", result.SQL, "ok", "")
		} else {
			var response *ChatResponse
			var err error

			// 尝试使用结构化输出（JSON Schema 约束）
			if g.supportsStructuredOutput() {
				response, err = g.chatWithStructuredOutput(ctx, enhancedSystemPrompt, messages)
				if err != nil {
					logx.Infof("[Text2SQL] Structured output failed, falling back to plain chat: %v", err)
					response, err = g.llmProvider.Chat(ctx, enhancedSystemPrompt, messages)
				}
			} else {
				response, err = g.llmProvider.Chat(ctx, enhancedSystemPrompt, messages)
			}

			if err != nil {
				g.traceStep("llm_call", fmt.Sprintf("retry=%d", retry), "", "error", err.Error())
				if retry < maxRetries-1 {
					continue
				}
				return nil, fmt.Errorf("LLM 调用失败: %w", err)
			}

			result = g.parseLLMResponse(response.Content, kb)
			logx.Infof("[Text2SQL-DIAG] Step1 parseLLMResponse: sql=%q, len=%d", result.SQL, len(result.SQL))
			g.traceStep("llm_call", fmt.Sprintf("retry=%d", retry), result.SQL, "ok", "")
		}

		if result.SQL != "" {
			// 统一后处理：Self-Reflection + 后向字段校验 + 确定性字段验证
			processedSQL, procWarnings, hasError := g.postProcessSQL(ctx, text, result.SQL, schemaContext, relationsContext, kb, schemaTables)
			result.SQL = processedSQL
			g.traceStep("post_process", result.SQL, fmt.Sprintf("hasError=%v warnings=%d", hasError, len(procWarnings)), "ok", "")
			lastSQL = processedSQL
			result.Warnings = append(result.Warnings, procWarnings...)

			if !hasError && g.executor != nil && spaceID > 0 {
				efl := g.getExecutionFeedbackLoop()
				if GetText2SQLConfig().EnableExecutionFeedbackLoop && efl != nil {
					// P0-4: ExecutionFeedbackLoop 自动执行+LLM修正+重试
					execResult, feedback := efl.ExecuteAndFix(ctx, result.SQL, text, schemaContext, uint64(spaceID))
					execStatus := "ok"
					if !execResult.Success {
						execStatus = "error"
					}
					g.traceStep("execute", result.SQL, fmt.Sprintf("level=%d retry=%d rows=%d", feedback.Level, feedback.RetryCount, execResult.RowCount), execStatus, execResult.Error)
					if execResult.Success {
						result.ExecutionResult = execResult
						if feedback.FixedSQL != "" && feedback.FixedSQL != result.SQL {
							result.SQL = feedback.FixedSQL
							result.Warnings = append(result.Warnings, "ExecutionFeedbackLoop修正: "+feedback.Message)
						}
						// P1-2: 执行计划分析（EXPLAIN 评分）
						if GetText2SQLConfig().EnableExecutionPlanAnalysis && spaceID > 0 {
							epa := g.getExecutionPlanAnalyzer()
							if epa != nil {
								scoreDelta, epWarnings := epa.ScoreForCandidate(result.SQL, uint64(spaceID))
								if len(epWarnings) > 0 {
									result.Warnings = append(result.Warnings, epWarnings...)
								}
								if scoreDelta < -0.1 {
									result.Warnings = append(result.Warnings, fmt.Sprintf("执行计划评分: %.2f（建议优化索引）", scoreDelta))
								}
								g.traceStep("exec_plan", result.SQL, fmt.Sprintf("score_delta=%.2f warnings=%d", scoreDelta, len(epWarnings)), "ok", "")
							}
						}
						// P1-3: 意图-结果一致性校验（LLM 判断结果是否回答了问题）
						if GetText2SQLConfig().EnableIntentCheck {
							ic := g.getIntentChecker()
							if ic != nil {
								icResult := ic.Check(ctx, text, result.SQL, execResult)
								if icResult.Status != "answered" {
									result.Warnings = append(result.Warnings, fmt.Sprintf("意图校验[%s]: %s", icResult.Status, icResult.Reason))
								}
								g.traceStep("intent_check", result.SQL, fmt.Sprintf("status=%s should_retry=%v", icResult.Status, icResult.ShouldRetry), "ok", "")
								// not_answered 时建议重试（但当前循环已结束，仅在 warning 中提示）
								if icResult.ShouldRetry {
									result.Warnings = append(result.Warnings, "建议重新提问或细化查询条件")
								}
							}
						}
						g.feedbackLoop.RecordSuccess(FeedbackRecord{
							SpaceID:      spaceID,
							UserQuery:    text,
							GeneratedSQL: result.SQL,
							Duration:     time.Duration(execResult.Duration) * time.Millisecond,
						})
					} else {
						hasError = true
						lastExecError = execResult.Error
						result.Warnings = append(result.Warnings, "SQL执行错误: "+execResult.Error)
						g.feedbackLoop.RecordFailure(FeedbackRecord{
							SpaceID:      spaceID,
							UserQuery:    text,
							GeneratedSQL: result.SQL,
							ErrorType:    classifyErrorType(execResult.Error),
							ErrorMessage: execResult.Error,
							Duration:     time.Duration(execResult.Duration) * time.Millisecond,
						})
						if retry < maxRetries-1 {
							lastSQL = result.SQL
							continue
						}
					}
				} else {
					// 原有内联执行+AST修正逻辑
					// P1 首要任务 1：试执行沿 ctx 携带真实调用者身份（无身份仍被拒绝）
					execResult := g.executor.ExecuteWithContext(ctx, result.SQL, uint64(spaceID))
					execStatus := "ok"
					if !execResult.Success {
						execStatus = "error"
					}
					g.traceStep("execute", result.SQL, fmt.Sprintf("success=%v rows=%d", execResult.Success, execResult.RowCount), execStatus, execResult.Error)
					if !execResult.Success {
						hasError = true
						lastExecError = execResult.Error
						result.Warnings = append(result.Warnings, "SQL执行错误: "+execResult.Error)

						// 反馈闭环：记录失败
						g.feedbackLoop.RecordFailure(FeedbackRecord{
							SpaceID:      spaceID,
							UserQuery:    text,
							GeneratedSQL: result.SQL,
							ErrorType:    classifyErrorType(execResult.Error),
							ErrorMessage: execResult.Error,
							Duration:     time.Duration(execResult.Duration) * time.Millisecond,
						})

						// 尝试AST修正
						availableFields := g.buildAvailableFieldsMap(kb)
						correction := g.astCorrector.Correct(result.SQL, execResult.Error, availableFields)
						if correction.Corrected {
							result.SQL = correction.CorrectedSQL
							result.Warnings = append(result.Warnings, "AST自动修正: "+correction.Suggestion)

							// 重新执行修正后的SQL
							execResult2 := g.executor.ExecuteWithContext(ctx, result.SQL, uint64(spaceID))
							if execResult2.Success {
								result.ExecutionResult = execResult2
								hasError = false

								// 反馈闭环：记录修正成功
								g.feedbackLoop.RecordCorrection(FeedbackRecord{
									SpaceID:       spaceID,
									UserQuery:     text,
									GeneratedSQL:  lastSQL,
									CorrectedSQL:  result.SQL,
									CorrectionSrc: "ast",
									Duration:      time.Duration(execResult2.Duration) * time.Millisecond,
								})
							}
						}

						// P1-8: AST 修正失败时尝试 LLM 修正
						if hasError && GetText2SQLConfig().EnableLLMRefiner {
							refiner := g.getLLMRefiner()
							if refiner != nil {
								refinedSQL, refineErr := refiner.Refine(ctx, result.SQL, text, schemaContext, execResult.Error)
								if refineErr == nil && refinedSQL != "" {
									result.SQL = refinedSQL
									result.Warnings = append(result.Warnings, "LLM修正: AST修正失败后LLM修正")

									// 重新执行 LLM 修正后的 SQL
									execResult3 := g.executor.ExecuteWithContext(ctx, result.SQL, uint64(spaceID))
									if execResult3.Success {
										result.ExecutionResult = execResult3
										hasError = false
										g.feedbackLoop.RecordCorrection(FeedbackRecord{
											SpaceID:       spaceID,
											UserQuery:     text,
											GeneratedSQL:  lastSQL,
											CorrectedSQL:  result.SQL,
											CorrectionSrc: "llm",
											Duration:      time.Duration(execResult3.Duration) * time.Millisecond,
										})
										g.traceStep("llm_refine", result.SQL, fmt.Sprintf("error=%s", execResult.Error), "ok", "")
									}
								}
							}
						}

						if hasError && retry < maxRetries-1 {
							lastSQL = result.SQL
							continue
						}
					} else {
						result.ExecutionResult = execResult

						// 反馈闭环：记录成功
						g.feedbackLoop.RecordSuccess(FeedbackRecord{
							SpaceID:      spaceID,
							UserQuery:    text,
							GeneratedSQL: result.SQL,
							Duration:     time.Duration(execResult.Duration) * time.Millisecond,
						})
					}
				}
			}

			if !hasError || retry >= maxRetries-1 {
				return result, nil
			}

			continue
		}
		return result, nil
	}

	return nil, fmt.Errorf("多次生成 SQL 均存在结构性问题，请优化提示词或检查表结构")
}

// parseLLMResponse 解析 LLM 响应
func (g *Generator) parseLLMResponse(content string, kb *KnowledgeBase) *modelai.SQLResult {
	logx.Infof("[Text2SQL-DIAG] parseLLMResponse input: content=%q, len=%d", content, len(content))
	result := &modelai.SQLResult{
		Tables:       []string{},
		Entities:     []modelai.RecognizedEntity{},
		Relations:    []modelai.TableRelation{},
		Explanations: []string{},
		Warnings:     []string{},
	}

	// P1-4: 优先尝试 JSON 结构化输出解析（当启用 WithJSONSchema 时 LLM 返回 JSON）
	if parsed, ok := tryParseStructuredJSON(content); ok {
		result.SQL = parsed.SQL
		if parsed.Explanation != "" {
			result.Explanations = append(result.Explanations, parsed.Explanation)
		}
		if len(parsed.Tables) > 0 {
			result.Tables = parsed.Tables
		}
		// 结构化输出成功，跳过正则路径
		logx.Infof("[Text2SQL] Parsed structured JSON output: sql_len=%d confidence=%.2f", len(result.SQL), parsed.Confidence)
		return result
	}

	// P0: 保存 LLM 原始 CoT 输出（截取 SQL 之前的部分作为思考过程）
	if idx := strings.Index(content, "```sql"); idx > 0 {
		result.RawCoT = strings.TrimSpace(content[:idx])
	} else if idx := strings.Index(content, "SQL:"); idx > 0 {
		result.RawCoT = strings.TrimSpace(content[:idx])
	}

	// P0: 提取字段映射（用户说的X → DDL中的Y）
	result.FieldMappings = extractFieldMappings(content)

	// 提取 SQL 代码块 — 三级回退策略
	sqlRegex := regexp.MustCompile("(?s)```sql\\s*(.*?)\\s*```")
	if m := sqlRegex.FindStringSubmatch(content); len(m) > 1 {
		result.SQL = strings.TrimSpace(m[1])
	} else {
		// 回退1：匹配 SELECT ... 到分号结束
		selectRegex := regexp.MustCompile(`(?is)(SELECT\s+.+;)`)
		if m := selectRegex.FindStringSubmatch(content); len(m) > 1 {
			result.SQL = strings.TrimSpace(m[1])
		} else {
			// 回退2：提取 SELECT 到代码块结束或文本结束
			selectRegex2 := regexp.MustCompile("(?is)(SELECT\\s+.*?)(?:```|$)")
			if m := selectRegex2.FindStringSubmatch(content); len(m) > 1 {
				result.SQL = strings.TrimSpace(m[1])
			} else {
				// 回退3：任何以 SELECT 开头的行直到文本结束
				lines := strings.Split(content, "\n")
				var sqlLines []string
				capturing := false
				for _, line := range lines {
					trimmed := strings.TrimSpace(line)
					if !capturing && strings.HasPrefix(strings.ToUpper(trimmed), "SELECT") {
						capturing = true
					}
					if capturing {
						if trimmed == "```" || trimmed == "---" {
							break
						}
						sqlLines = append(sqlLines, line)
					}
				}
				if len(sqlLines) > 0 {
					result.SQL = strings.TrimSpace(strings.Join(sqlLines, "\n"))
				}
			}
		}
	}

	// 提取解释
	expRegex := regexp.MustCompile(`(?i)解释[:：]\s*(.*?)(?:\n\n|$)`)
	if m := expRegex.FindStringSubmatch(content); len(m) > 1 {
		result.Explanations = append(result.Explanations, strings.TrimSpace(m[1]))
	}

	// 从 SQL 中提取涉及的表名
	if result.SQL != "" {
		tables := g.extractTablesFromSQL(result.SQL, kb)
		result.Tables = tables
		for _, t := range tables {
			result.Entities = append(result.Entities, modelai.RecognizedEntity{
				Type:  "table",
				Value: t,
			})
		}
	}

	// 添加 LLM 生成的说明
	result.Explanations = append(result.Explanations, "SQL 由 LLM 根据表结构自动生成")

	return result
}

// extractFieldMappings 从 LLM 输出中提取字段映射
// 格式: 用户提到的 bm_attr_spec → DDL中的 BM_ATTR_SPEC
func extractFieldMappings(content string) []string {
	// 匹配 "字段映射" 段落
	mappingRe := regexp.MustCompile(`(?s)字段映射[:：]\s*(.*?)(?:\n\n|\n###|\nSQL:)`)
	m := mappingRe.FindStringSubmatch(content)
	if len(m) < 2 {
		return nil
	}
	// 逐行提取映射
	lines := strings.Split(strings.TrimSpace(m[1]), "\n")
	var mappings []string
	arrowRe := regexp.MustCompile(`→`)
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || !arrowRe.MatchString(line) {
			continue
		}
		// 去除列表标记
		line = strings.TrimLeft(line, "-*• ")
		mappings = append(mappings, strings.TrimSpace(line))
	}
	return mappings
}

// generateWithRules 使用规则生成 SQL（回退方案）
func (g *Generator) generateWithRules(text string, kb *KnowledgeBase) (*modelai.SQLResult, error) {
	recognizer := NewEntityRecognizer(kb)
	input := recognizer.Recognize(text)

	if len(input.Tables) == 0 {
		return nil, fmt.Errorf("未能识别出任何表，请检查输入是否包含表名或业务术语")
	}

	inferer := NewRelationInferer(kb)
	joins := inferer.InferJoins(input.Tables)

	assembler := NewSQLAssembler(kb)
	sql := assembler.Assemble(input, joins)

	validator := NewSQLValidator(kb)
	validation := validator.Validate(sql, input)
	if !validation.Valid {
		return nil, fmt.Errorf("SQL验证失败: %v", validation.Errors)
	}

	result := &modelai.SQLResult{
		SQL:      sql,
		Tables:   input.Tables,
		Entities: g.convertEntities(input),
		Warnings: validation.Warnings,
	}

	for _, join := range joins {
		result.Relations = append(result.Relations, modelai.TableRelation{
			SourceTable: join.SourceTable,
			SourceField: join.SourceField,
			TargetTable: join.TargetTable,
			TargetField: join.TargetField,
		})
	}

	result.Explanations = g.generateExplanations(input, joins)

	return result, nil
}

func (g *Generator) RefreshKnowledge() error {
	if err := g.kb.LoadRules(); err != nil {
		return err
	}

	caches, err := g.kb.tableCachesForRefresh()
	if err != nil {
		return err
	}
	g.kb.ReloadTablesFromCaches(caches)

	return nil
}

func (g *Generator) GetKnowledgeBase() *KnowledgeBase {
	return g.kb
}

// supportsStructuredOutput 检查是否支持结构化输出
func (g *Generator) supportsStructuredOutput() bool {
	// 检查 llmProvider 是否实现了 ChatWithOptions 方法
	_, ok := g.llmProvider.(interface {
		ChatWithOptions(ctx context.Context, systemPrompt string, messages []Message, opts ...CallOption) (*ChatResponse, error)
	})
	return ok
}

// chatWithStructuredOutput 使用结构化输出调用 LLM
func (g *Generator) chatWithStructuredOutput(ctx context.Context, systemPrompt string, messages []Message) (*ChatResponse, error) {
	providerWithOpts, ok := g.llmProvider.(interface {
		ChatWithOptions(ctx context.Context, systemPrompt string, messages []Message, opts ...CallOption) (*ChatResponse, error)
	})
	if !ok {
		return nil, fmt.Errorf("LLM provider does not support structured output")
	}

	// 使用 JSON Schema 约束输出
	schema := GetSQLGenerationJSONSchema()
	return providerWithOpts.ChatWithOptions(ctx, systemPrompt, messages, WithJSONSchema(schema))
}

// getPromptTemplateFromDB 从数据库加载指定类型的提示词模板
func (g *Generator) getPromptTemplateFromDB(qType QueryType, spaceID int) (PromptTemplate, bool) {
	if g.promptTplRepo == nil {
		return PromptTemplate{}, false
	}

	queryTypeStr := ""
	switch qType {
	case QueryTypeBasic:
		queryTypeStr = "basic"
	case QueryTypeMultiJoin:
		queryTypeStr = "multi_join"
	case QueryTypeAggregation:
		queryTypeStr = "aggregation"
	case QueryTypeSubquery:
		queryTypeStr = "subquery"
	case QueryTypeCTE:
		queryTypeStr = "cte"
	case QueryTypeComplexReport:
		queryTypeStr = "complex_report"
	default:
		queryTypeStr = "basic"
	}

	var spaceIDPtr uint64
	if spaceID > 0 {
		spaceIDPtr = uint64(spaceID)
	}

	tpl, err := g.promptTplRepo.FindByQueryType(spaceIDPtr, queryTypeStr)
	if err != nil {
		return PromptTemplate{}, false
	}

	return PromptTemplate{
		Type:         qType,
		Name:         tpl.Name,
		SystemPrompt: tpl.SystemPrompt,
		UserTemplate: tpl.UserTemplate,
	}, true
}

// generateExplanations 生成解释
func (g *Generator) generateExplanations(input *RecognizedInput, joins []JoinPath) []string {
	var explanations []string

	explanations = append(explanations, fmt.Sprintf("识别到 %d 张表: %v", len(input.Tables), input.Tables))

	if len(joins) > 0 {
		explanations = append(explanations, fmt.Sprintf("推断出 %d 个关联关系", len(joins)))
	}

	if len(input.Conditions) > 0 {
		explanations = append(explanations, fmt.Sprintf("提取了 %d 个筛选条件", len(input.Conditions)))
	}

	return explanations
}
