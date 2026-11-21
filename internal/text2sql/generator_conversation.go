package text2sql

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	logx "github.com/listen808/citebase/internal/log"
	modelai "github.com/listen808/citebase/internal/model/ai"
	"github.com/listen808/citebase/internal/utils"
)

// Text2SQL 生成器：多轮会话上下文、澄清、会话式生成。

// SetAgent 设置 Agent 实例
func (g *Generator) SetMemoryManager(mm *ConversationMemoryManager) {
	g.memoryManager = mm
}

// SetConversationContext 设置当前对话上下文（强制使用对话历史）
// 每次生成前必须调用，从数据库加载历史对话状态
func (g *Generator) SetConversationContext(userID, convID uint64, spaceID uint64) error {
	g.currentUserID.Store(userID)
	g.currentConvID.Store(convID)

	// 如果记忆管理器未初始化，只更新内存状态
	if g.memoryManager == nil {
		g.conversationState = NewConversationState(int(convID), int(spaceID))
		logx.Infof("[Generator] MemoryManager not initialized, using in-memory state only")
		return nil
	}

	// 从数据库加载对话历史（强制加载）
	if err := g.memoryManager.LoadConversationHistory(convID, g.conversationState); err != nil {
		logx.Infof("[Generator] Failed to load conversation history from db: %v", err)
		// 加载失败时创建新状态，但不报错（允许新对话开始）
		g.conversationState = NewConversationState(int(convID), int(spaceID))
	} else {
		logx.Infof("[Generator] Loaded conversation %d history from db, %d turns", convID, len(g.conversationState.SQLHistory))
	}

	// 更新会话最后访问时间
	if err := g.memoryManager.SaveSessionState(userID, convID, spaceID, g.conversationState); err != nil {
		logx.Infof("[Generator] Failed to save session state: %v", err)
	}

	return nil
}

// SaveCurrentTurn 保存当前轮次到数据库和向量库
// 在SQL生成完成后调用，确保每轮对话都被持久化
func (g *Generator) SaveCurrentTurn(
	userQuery, resolvedQuery, intentType, generatedSQL, explanation string,
	tables, fields []string,
	isSuccess bool,
	executionError string,
) (*modelai.SQLConversationTurn, error) {
	convID := g.currentConvID.Load()
	userID := g.currentUserID.Load()
	if g.memoryManager == nil || convID == 0 {
		logx.Infof("[Generator] MemoryManager not initialized or convID is 0, skip saving turn")
		return nil, nil
	}

	// 获取当前轮次号
	maxTurn, err := g.memoryManager.repo.GetMaxTurnNumber(convID)
	if err != nil {
		maxTurn = len(g.conversationState.SQLHistory)
	}
	turnNumber := maxTurn + 1

	// 保存到数据库和向量库
	turn, err := g.memoryManager.SaveTurn(
		context.Background(),
		userID,
		convID,
		uint64(g.conversationState.SpaceID),
		turnNumber,
		userQuery,
		resolvedQuery,
		intentType,
		generatedSQL,
		explanation,
		tables,
		fields,
		isSuccess,
		executionError,
	)
	if err != nil {
		return nil, fmt.Errorf("save turn failed: %w", err)
	}

	// 更新对话摘要
	_ = g.memoryManager.UpdateConversationSummary(
		convID,
		userID,
		uint64(g.conversationState.SpaceID),
		generatedSQL,
		tables,
		fields,
		turnNumber,
		0, // successCount会在UpdateTurnResult时更新
	)

	// 保存会话状态
	_ = g.memoryManager.SaveSessionState(userID, convID, uint64(g.conversationState.SpaceID), g.conversationState)

	logx.Infof("[Generator] Saved turn %d for conversation %d", turnNumber, convID)
	return turn, nil
}

type ClarificationResult struct {
	Needed  bool     `json:"needed"`
	Options []string `json:"options"`
}

type ConversationMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
	SQL     string `json:"sql,omitempty"`
}

// GenerateWithConversation 多轮对话SQL生成
// 支持：①澄清模糊问题 ②基于历史SQL修改 ③在历史上下文基础上继续查询
func (g *Generator) GenerateWithConversation(ctx context.Context, question string, spaceID uint64, history []ConversationMessage) (*modelai.SQLResult, *ClarificationResult, error) {
	if g.llmProvider == nil {
		return nil, nil, fmt.Errorf("LLM provider not configured")
	}

	// 阶段三：创建 Trace 收集器
	var traceCollector *TraceCollector
	if g.traceRepo != nil {
		traceCollector = NewTraceCollector(spaceID, int(g.currentConvID.Load()), g.traceRepo)
	}

	kb := NewKnowledgeBase(g.repo)
	// 注入 Schema 仓库到新 kb（g.kb 已在 Generator 初始化时注入，但此处创建了新实例）
	if g.schemaRepo != nil {
		kb.SetSchemaRepo(g.schemaRepo)
	}
	if err := kb.LoadFromSpace(spaceID); err != nil {
		return nil, nil, fmt.Errorf("加载空间知识库失败: %w", err)
	}
	// 无条件合并全局表缓存：确保 kb 包含所有空间的表，
	// 避免因 space_id 不匹配导致"表不在知识库"的误报
	if err := kb.MergeAllTables(); err != nil {
		// MergeAllTables 失败时回退到 LoadAll
		if err := kb.LoadAll(); err != nil {
			return nil, nil, fmt.Errorf("加载全局知识库失败: %w", err)
		}
	}
	// LoadFromSpace 已统一从 aw_schema_* 加载（通过 KnowledgeBase.schemaRepo），
	// 不再需要单独的 LoadFromSchemaSpace 桥接调用
	// 0. 最关键：将用户问题中的错误字段名替换为DDL实际字段名
	question = sanitizeQuestionFields(question, kb)
	// 业务术语确定性替换（Glossary 映射）
	question = g.applyGlossaryMapping(question, int(spaceID))

	// ① 判断是否需要澄清
	// 【关键】先用规则做"已澄清放行"判断，避免 LLM 反复澄清
	// 然后再调用 LLM 判定，LLM 能看到 Schema 摘要（通过 kb 参数）
	if shouldBypassClarification(question, history, kb) {
		logx.Infof("[Clarify] 规则判定放行：用户已通过澄清回答或上下文充分明确")
	} else {
		clarification := g.checkClarificationWithKB(ctx, question, history, kb)
		if clarification != nil && clarification.Needed {
			// 阶段三：记录 Trace（澄清模式）
			if traceCollector != nil {
				traceCollector.Flush(question, "clarification", "", false)
			}
			return nil, clarification, nil
		}
	}

	// ② 判断是否是修改模式（历史中有SQL，且当前问题语义是修改/调整）
	modifyMode := false
	if len(history) > 0 {
		lastMsg := history[len(history)-1]
		modifyMode = lastMsg.SQL != "" && isModifyIntent(question)
	}

	// ③ 构建历史SQL上下文
	var historySQL string
	for _, m := range history {
		if m.SQL != "" {
			historySQL = m.SQL
		}
	}
	if len(kb.GetAllTables()) == 0 {
		caches, err := g.repo.GetAllTableCaches()
		if err != nil {
			return nil, nil, fmt.Errorf("加载表缓存失败: %w", err)
		}
		for i := range caches {
			kb.tables[caches[i].TableNam] = &caches[i]
		}
	}
	// 注意：不再依赖 g.recognizer（它绑定 Generator 构造时的旧 kb），
	// 因为 GenerateWithConversation 创建了新的 kb，g.recognizer 无法感知新加载的表。
	// MergeAllTables 已无条件合并全局表，kb 现在包含所有空间的表。
	if err := kb.LoadRules(); err != nil {
		return nil, nil, fmt.Errorf("加载规则失败: %w", err)
	}

	// ④ 构建对话历史字符串（使用智能压缩）
	conversationCtx := g.CompressConversationContext(history)

	// ⑤ 生成SQL（传入历史SQL和对话上下文）
	result, err := g.generateWithConversationContext(ctx, question, kb, int(spaceID), historySQL, conversationCtx, modifyMode)
	if err != nil {
		return nil, nil, err
	}

	// 补充解释与 TraceID + CoT 思考步骤
	if result != nil {
		// 阶段三：优先使用 traceCollector 的 TraceID
		if traceCollector != nil {
			execOK := result.ExecutionResult != nil
			traceCollector.Flush(question, "conversation", result.SQL, execOK)
			result.TraceID = traceCollector.TraceID()
		} else {
			result.TraceID = traceIDFromContext(ctx)
		}
		if g.explainer != nil && result.SQL != "" {
			if explanation, err := g.explainer.Explain(ctx, question, result.SQL); err == nil {
				result.Explanation = explanation
			}
		}
		// P0: 构建 CoT 思考步骤供前端展示
		result.Steps = g.buildCoTSteps(question, kb, result)

		// P0: 自动收集成功 SQL 为 FewShot 示例
		// P1（清单 110）：新样本默认 Verified=0（未验证），不再"直接标记已验证、立即
		// 生效"——否则被注入的问答只要恰好执行成功，就成了对同空间所有用户 prompt 的
		// 可信示范（提示注入放大通道）；仅人工 Verify 或离线复核后才进入 few-shot。
		// 后台 goroutine 补 recover：exampleRepo 异常不得击穿进程。
		if result.SQL != "" && len(result.Warnings) == 0 && g.exampleRepo != nil {
			go func(q, sql string) {
				defer func() {
					if r := recover(); r != nil {
						logx.Infof("[FewShot] auto-save example panicked: %v", r)
					}
				}()
				example := &modelai.SQLExample{
					SpaceID:   int(spaceID),
					Question:  q,
					SQL:       sql,
					Verified:  0,
					CreatedBy: 0,
				}
				if err := g.exampleRepo.Create(example); err != nil {
					logx.Infof("[FewShot] Failed to auto-save example: %v", err)
				} else {
					logx.Infof("[FewShot] Auto-saved unverified example for space %d (待人工验证)", spaceID)
				}
			}(question, result.SQL)
		}
	}

	return result, nil, nil
}

// isModifyIntent 判断用户输入是否是修改/延续已有SQL的意图
func isModifyIntent(question string) bool {
	modifyKeywords := []string{
		"改成", "修改", "加上", "去掉", "增加", "删除", "调整",
		"换个", "替换", "改为", "改成", "加上条件", "去掉条件",
		"排序", "分组", "加个", "再加", "也查", "补上", "追加",
		"modify", "change", "update this", "add to", "remove from",
		"改成按", "换一个", "在这个基础上",
		// 延续性查询（基于上一轮结果的追问）
		"查询全部", "查全部", "全部数据", "所有数据", "全部记录",
		"再查", "继续查", "接着查", "然后查",
		"统计", "汇总", "合计",
		"去掉条件", "不加条件", "不限制",
		"限制", "筛选", "过滤",
	}
	lower := strings.ToLower(question)
	for _, kw := range modifyKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

// buildConversationContext 构建多轮对话上下文
func buildConversationContext(history []ConversationMessage) string {
	if len(history) == 0 {
		return ""
	}
	var parts []string
	for _, m := range history {
		role := "用户"
		if m.Role == "assistant" {
			role = "助手"
		}
		line := fmt.Sprintf("%s: %s", role, m.Content)
		if m.SQL != "" {
			line += fmt.Sprintf("\n[生成的SQL: %s]", m.SQL)
		}
		parts = append(parts, line)
	}
	return strings.Join(parts, "\n")
}

// generateWithConversationContext 带对话上下文的SQL生成 —— LLM驱动：向量检索表结构 → think → 生成SQL
func (g *Generator) generateWithConversationContext(ctx context.Context, text string, kb *KnowledgeBase, spaceID int, historySQL string, conversationCtx string, modifyMode bool) (*modelai.SQLResult, error) {
	// 1. 从历史SQL中提取表名，确保这些表的DDL被包含
	historyTables := extractTableNamesFromSQL(historySQL)
	if len(historyTables) > 0 {
		for _, t := range historyTables {
			if cache, ok := kb.tables[t]; ok {
				kb.tables[t] = cache // 确保在kb中
			}
		}
	}

	// 2. Schema Linker 语义检索：基于用户问题+对话历史从向量库查找最相关的表
	searchText := text
	if historySQL != "" {
		// 将历史表名也加入搜索文本，确保向量检索能找到它们
		tableNames := strings.Join(historyTables, " ")
		searchText = text + "\n" + conversationCtx + "\n上一轮SQL: " + historySQL + "\n涉及的表: " + tableNames
	}

	// 【关键】从对话历史中提取"已选择"字段（如 update_time）并注入到 searchText
	// 避免新一轮检索时丢失用户在澄清中明确指定过的字段名
	if conversationCtx != "" {
		chosenFieldRe := regexp.MustCompile(`已选择[：:]([^（()）]+)`)
		matches := chosenFieldRe.FindAllStringSubmatch(conversationCtx, -1)
		if len(matches) > 0 {
			chosenFields := make([]string, 0, len(matches))
			for _, m := range matches {
				if len(m) > 1 {
					chosenFields = append(chosenFields, strings.TrimSpace(m[1]))
				}
			}
			searchText = searchText + "\n[User-chosen fields: " + strings.Join(chosenFields, ", ") + "]"
		}
	}

	schemaContext, relationsContext := g.retrieveSchemaContext(ctx, searchText, kb, spaceID)

	// 3. 确保历史SQL中的表在schema上下文中（兜底：如果向量检索没找到，强制从kb补充）
	if historySQL != "" && len(historyTables) > 0 {
		for _, t := range historyTables {
			if !strings.Contains(schemaContext, t) {
				if cache, ok := kb.tables[t]; ok && cache.DDLContent != "" {
					schemaContext += "\n\n" + cache.DDLContent
				}
			}
		}
	}

	// 2. 字段名安全检测：用户提到的字段名如果在DDL中不存在，生成对照表
	fieldMappings := detectMissingFields(text, kb)

	// 2.5 Few-shot 示例检索
	fewShotBlock := g.retrieveFewShotExamples(text, spaceID, 3)

	var schemaTables []string
	for name := range kb.GetAllTables() {
		schemaTables = append(schemaTables, name)
	}

	maxRetries := 3
	var lastSQL string
	var lastExecError string

	for retry := 0; retry < maxRetries; retry++ {
		// 3. 构建Chain-of-Thought提示词
		prompt := g.buildConversationPrompt(text, schemaContext, relationsContext, historySQL, conversationCtx, modifyMode, lastSQL, lastExecError, fieldMappings, fewShotBlock)

		messages := []Message{NewTextMessage("user", prompt)}

		// 3. LLM思考+生成（使用增强的CoT系统提示词）
		// 根据查询复杂度自动选择模型档位：复杂查询使用 reasoning provider
		tier := TierPrimary
		if EstimateComplexity(text) >= 50 {
			tier = TierReasoning
		}
		resp, err := g.llmProvider.ChatWithOptions(
			ctx,
			g.buildCoTSystemPrompt(schemaContext, relationsContext, text),
			messages,
			WithTier(tier),
			WithTemperature(GetText2SQLConfig().TemperatureGeneration),
		)
		if err != nil {
			if retry < maxRetries-1 {
				continue
			}
			return nil, fmt.Errorf("LLM 调用失败: %w", err)
		}

		result := g.parseLLMResponse(resp.Content, kb)
		result.GeneratorTier = string(tier)

		if result.SQL != "" {
			// P0: 保存 postProcess 前的原始 SQL，供前端展示修正对比
			result.SelfReflectBefore = result.SQL

			// 统一后处理：Self-Reflection + 后向字段校验 + 确定性字段验证
			processedSQL, procWarnings, hasError := g.postProcessSQL(ctx, text, result.SQL, schemaContext, relationsContext, kb, schemaTables)
			result.SQL = processedSQL
			// P0: 保存 postProcess 后的最终 SQL
			result.SelfReflectAfter = processedSQL
			lastSQL = processedSQL
			result.Warnings = append(result.Warnings, procWarnings...)

			if !hasError && g.executor != nil && spaceID > 0 {
				// P1 首要任务 1：试执行沿请求 ctx 携带真实调用者身份（清单 35）
				execResult := g.executor.ExecuteWithContext(ctx, result.SQL, uint64(spaceID))
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

			if !hasError || retry >= maxRetries-1 {
				return result, nil
			}
			continue
		}
		return result, nil
	}

	return nil, fmt.Errorf("多次生成 SQL 均存在结构性问题")
}

// buildConversationPrompt 构建带对话上下文的提示词
func (g *Generator) buildConversationPrompt(question, schemaCtx, relationsCtx, historySQL, convCtx string, modifyMode bool, prevSQL, prevError string, fieldMappings string, fewShotBlock string) string {
	var prompt strings.Builder

	if modifyMode && historySQL != "" {
		// 修改模式：在已有SQL基础上修改
		prompt.WriteString("你是一个数据库专家。请基于以下已有的SQL进行修改，满足用户的新需求。\n\n")
		prompt.WriteString(fmt.Sprintf("## 用户新需求\n%s\n\n", question))
		prompt.WriteString(fmt.Sprintf("## 已有SQL（请在此SQL基础上修改，不要完全重写）\n```sql\n%s\n```\n\n", historySQL))
		prompt.WriteString(fmt.Sprintf("## 表结构参考\n%s\n\n", schemaCtx))
		prompt.WriteString(fmt.Sprintf("## 表关联关系\n%s\n\n", relationsCtx))
		prompt.WriteString("## 修改要求\n")
		prompt.WriteString("1. 尽量保留已有SQL的结构和逻辑\n")
		prompt.WriteString("2. 根据用户新需求精确修改\n")
		prompt.WriteString("3. 只输出修改后的完整SQL\n")
		prompt.WriteString("4. 使用 `表别名.字段名` 格式\n")
	} else {
		// 新查询模式 — DDL最优先，对话历史用于上下文理解
		prompt.WriteString("你是数据库专家。以下DDL是字段名的唯一权威来源。\n\n")
		// 1. DDL先行（最权威）
		prompt.WriteString(fmt.Sprintf("## 完整表结构（唯一权威来源，所有字段名以此为准）\n%s\n\n", schemaCtx))
		prompt.WriteString(fmt.Sprintf("## 表关联关系\n%s\n\n", relationsCtx))
		// 2. 字段名对照（如检测到不匹配）
		if fieldMappings != "" {
			prompt.WriteString(fieldMappings)
		}
		// 3. Few-Shot 示例（动态检索 + 内置示例）
		if fewShotBlock != "" {
			prompt.WriteString(fewShotBlock)
			prompt.WriteString("\n")
		}
		// 内置硬编码示例（始终提供，帮助 LLM 学习正确的字段映射模式）
		prompt.WriteString("## Few-Shot 示例（学习正确的字段映射方式）\n")
		prompt.WriteString("问题：查询bm_dc_param表中val包含caftp的参数值和BM_DATA_DICT表中的字典值，两个表val值相同\n")
		prompt.WriteString("字段映射：val → bm_dc_param.param_val, VALUE → BM_DATA_DICT.PARAM_VALUE\n")
		prompt.WriteString("```sql\nSELECT p.param_val AS 参数值, d.PARAM_VALUE AS 字典值 FROM bm_dc_param p INNER JOIN BM_DATA_DICT d ON p.param_val = d.PARAM_VALUE WHERE p.param_val LIKE '%caftp%';\n```\n\n")
		// 4. 对话历史 + 上轮SQL（重要上下文）
		if convCtx != "" {
			prompt.WriteString(fmt.Sprintf("## 对话历史\n%s\n\n", convCtx))
		}
		if historySQL != "" {
			prompt.WriteString(fmt.Sprintf("## 上一轮生成的SQL\n```sql\n%s\n```\n\n", historySQL))
			prompt.WriteString("⚠️ 如果用户的问题是基于上一轮查询的延续（如\"查询全部数据\"、\"再加个条件\"、\"按时间排序\"等），请在上一轮SQL基础上修改，而不是重新生成。\n\n")
		}
		// 5. 用户问题（最末）
		prompt.WriteString(fmt.Sprintf("## 用户当前问题\n%s\n\n", question))
		prompt.WriteString("## 生成要求\n")
		prompt.WriteString("🚨 表名必须来自上方DDL！用户说的表名就是DDL中的表名，不允许替换为其他表！\n")
		prompt.WriteString("🚨 所有字段名必须从上方DDL中查找！\n")
		prompt.WriteString("🚨 字段详情表中有\"可能被称作\"列，帮助你将用户的称呼映射到实际字段名！\n")
		prompt.WriteString("🚨 用户说\"val\"但DDL中是\"param_val\"→必须用param_val！\n")
		prompt.WriteString("🚨 用户说\"VALUE\"但DDL中是\"PARAM_VALUE\"→必须用PARAM_VALUE！\n")
		prompt.WriteString("1. 在\"解释\"中先列出字段映射：用户说的X → DDL中的Y\n")
		prompt.WriteString("2. 使用MySQL标准SQL语法，所有字段使用 `表别名.字段名` 格式\n")
		prompt.WriteString("3. 多表查询使用 JOIN 语法，明确关联条件\n")
		prompt.WriteString("4. 参考对话历史和上一轮SQL理解用户上下文\n")
		prompt.WriteString("5. 使用 `--` 注释说明关键查询逻辑\n")
		prompt.WriteString("6. 只返回SELECT查询\n")
		prompt.WriteString("7. **避免冗余WHERE**：JOIN ON已确保两列相等，WHERE中只对其中一列写LIKE/%条件\n\n")
		prompt.WriteString("🔍 生成后必须逐字段核对DDL，确保SQL中没有DDL中不存在的字段名\n")
	}

	if prevSQL != "" {
		prompt.WriteString(fmt.Sprintf("\n## 上一轮失败信息\n上次生成的SQL: ```sql\n%s\n```\n", prevSQL))
		issues := ValidateSQLStructure(prevSQL, nil)
		if len(issues) > 0 {
			for _, iss := range issues {
				prompt.WriteString(fmt.Sprintf("- %s: %s\n", iss.Type, iss.Message))
			}
		}
		if prevError != "" {
			prompt.WriteString(fmt.Sprintf("执行错误: %s\n", prevError))
		}
		prompt.WriteString("\n请修复以上问题。\n")
	}

	prompt.WriteString("\n## 输出格式\nSQL:\n```sql\n-- 查询说明\nSELECT ...\n```\n")
	prompt.WriteString("解释:\n...")

	return prompt.String()
}

// buildSystemPrompt 构建带表结构分析指引的系统提示词
func (g *Generator) buildSystemPrompt(schemaCtx, relationsCtx string) string {
	var sb strings.Builder
	sb.WriteString("你是一个资深数据库专家和SQL分析师。你的工作流程是：先仔细分析表结构，理解字段含义和表间关系，再生成准确高效的SQL。\n\n")

	sb.WriteString("## 分析步骤\n")
	sb.WriteString("1. **理解需求**：解析用户的自然语言问题，识别查询意图\n")
	sb.WriteString("2. **分析结构**：检查相关表的字段、主键、外键、索引和约束，确定需要的表和字段\n")
	sb.WriteString("3. **确定关系**：分析表间关联关系（JOIN方式），选择合适的连接类型\n")
	sb.WriteString("4. **生成SQL**：基于以上分析，生成MySQL兼容的SQL语句\n")
	sb.WriteString("5. **优化检查**：确保使用了正确的表别名、字段前缀、WHERE条件\n\n")

	sb.WriteString("## 关键表概要\n")
	tables := extractTableSummary(schemaCtx)
	if len(tables) > 0 {
		for _, t := range tables {
			sb.WriteString(fmt.Sprintf("- `%s`: %s\n", t.name, t.summary))
		}
	} else {
		sb.WriteString("参见提示词中的完整表结构定义\n")
	}
	sb.WriteString("\n")

	sb.WriteString("## 🚨 最高优先级规则（违反将导致SQL完全不可用）\n")
	sb.WriteString("- 用户问题中的字段名仅作语义参考，绝对不能直接使用！必须在DDL中查找对应的实际字段名\n")
	sb.WriteString("- 查看\"可能被称作\"列：字段详情表中有\"可能被称作\"列，帮助你将用户的称呼映射到实际字段名\n")
	sb.WriteString("- 示例：用户说'val字段'但DDL中实际列名是param_val → 必须用param_val，绝对不能用val\n")
	sb.WriteString("- 示例：用户说'VALUE字段'但DDL中无此列 → 查找最接近的列（如PARAM_VALUE），绝对不能用VALUE\n")
	sb.WriteString("- 生成SQL前先列出你从DDL中找到的实际字段名对比用户提到的字段名\n")
	sb.WriteString("- 必须使用 `表别名.字段名` 格式引用所有字段\n")
	sb.WriteString("- 多表查询时必须使用 JOIN 语法，并指定关联条件\n")
	sb.WriteString("- 字符串比较区分大小写，除非使用 LIKE 或显式转换\n")
	sb.WriteString("- 日期字段使用标准MySQL日期函数\n")
	sb.WriteString("- 使用 `--` 注释说明关键查询逻辑\n")
	sb.WriteString("- 对话历史仅用于理解语义上下文（如用户说的'那个表'指代什么），字段名必须从DDL获取\n\n")

	sb.WriteString("## 🔍 生成后必须自检（逐一核对）\n")
	sb.WriteString("1. SQL中用到的每个字段名，在DDL中都能找到吗？（用SELECT/WHERE/JOIN ON中的字段名逐一比对DDL）\n")
	sb.WriteString("2. 有没有使用用户提到的字段名而非DDL中的实际字段名？（这是最常见的错误）\n")
	sb.WriteString("3. 如果发现使用了DDL中不存在的字段名，必须修正后再输出\n")

	return sb.String()
}

// buildCoTSystemPrompt 构建Chain-of-Thought系统提示词：指导LLM分步思考
// P0-3: 当 coTPromptBuilder 可用时，根据问题类型追加特定指导
func (g *Generator) buildCoTSystemPrompt(schemaCtx, relationsCtx, question string) string {
	var sb strings.Builder

	sb.WriteString("你是一个资深数据库专家和SQL分析师。请按照以下步骤进行深度思考：\n\n")

	sb.WriteString("## 思考流程（Chain of Thought）\n\n")
	sb.WriteString("### 第1步：理解需求\n")
	sb.WriteString("- 解析用户问题的核心意图\n")
	sb.WriteString("- 识别用户想要查询的数据维度、筛选条件、排序方式\n")
	sb.WriteString("- 如果对话历史中有上下文，结合历史理解用户指代\n\n")

	sb.WriteString("### 第2步：分析表结构\n")
	sb.WriteString("- **只能使用DDL中出现的表名！禁止编造或使用DDL中不存在的表！**\n")
	sb.WriteString("- 用户问题中明确提到了表名（如 BM_DATA_DICT），必须使用该表\n")
	sb.WriteString("- 逐表查看提供的DDL和字段详情\n")
	sb.WriteString("- 确认每个表的主键、外键、索引\n")
	sb.WriteString("- 理解每个字段的含义（通过注释和字段名）\n")
	sb.WriteString("- 判断哪些字段适合作为查询条件、关联字段、排序字段\n\n")

	sb.WriteString("### 第3步：确定关联关系\n")
	sb.WriteString("- 如果需要多表查询，确定表间的关联方式\n")
	sb.WriteString("- 选择合适的JOIN类型（INNER/LEFT/RIGHT）\n")
	sb.WriteString("- 确认关联条件使用的字段\n\n")

	sb.WriteString("### 第4步：设计查询逻辑\n")
	sb.WriteString("- 确定SELECT的字段列表\n")
	sb.WriteString("- 确定FROM子句和JOIN子句\n")
	sb.WriteString("- 确定WHERE筛选条件\n")
	sb.WriteString("- 确定GROUP BY分组字段和聚合函数\n")
	sb.WriteString("- 确定ORDER BY排序字段\n")
	sb.WriteString("- 考虑是否需要LIMIT限制\n\n")

	sb.WriteString("### 第5步：优化检查\n")
	sb.WriteString("- 🚨 逐字段核对DDL：SQL中用到的每个字段名是否在DDL中存在？\n")
	sb.WriteString("- 🚨 是否使用了用户提到的字段名而非DDL中的实际字段名？（这是最常见的致命错误）\n")
	sb.WriteString("- 🚨 如果用户说\"val\"但DDL中是\"param_val\"，必须用param_val\n")
	sb.WriteString("- 检查JOIN条件是否正确\n")
	sb.WriteString("- 检查聚合函数与GROUP BY是否匹配\n")
	sb.WriteString("- 检查字段引用是否使用了表别名前缀\n")
	sb.WriteString("- 检查是否有性能优化的空间\n\n")

	sb.WriteString("## 关键表概要\n")
	tables := extractTableSummary(schemaCtx)
	if len(tables) > 0 {
		for _, t := range tables {
			sb.WriteString(fmt.Sprintf("- `%s`: %s\n", t.name, t.summary))
		}
	}
	sb.WriteString("\n")

	sb.WriteString("## 输出要求\n")
	sb.WriteString("- **表名必须是DDL中的表名，禁止替换或编造表名！**\n")
	sb.WriteString("- 在\"解释\"部分详细说明思考过程\n")
	sb.WriteString("- SQL使用MySQL标准语法\n")
	sb.WriteString("- 所有字段使用 `表别名.字段名` 格式\n")
	sb.WriteString("- 添加中文注释说明关键逻辑\n")
	sb.WriteString("- 如果多次调用生成，根据上轮反馈改进SQL\n")

	// P0-3: 追加查询类型特定指导（CoTPromptBuilder）
	if g.coTPromptBuilder != nil && question != "" {
		queryType := g.coTPromptBuilder.DetectQueryType(question)
		sb.WriteString("\n## 查询类型特定指导\n")
		sb.WriteString(fmt.Sprintf("- 识别到的查询类型: **%s**\n", queryType))
		sb.WriteString(g.coTPromptBuilder.getTypeSpecificGuidance(queryType))
	}

	return sb.String()
}

// buildCoTSteps 构建 CoT 思考步骤供前端展示
// 基于 LLM 原始 CoT 输出 + 后处理修正结果，重建完整推理过程
func (g *Generator) buildCoTSteps(question string, kb *KnowledgeBase, result *modelai.SQLResult) []modelai.AgentStepInfo {
	var steps []modelai.AgentStepInfo
	stepNum := 1

	// Step 1: 理解需求
	steps = append(steps, modelai.AgentStepInfo{
		Step: stepNum, Thought: fmt.Sprintf("理解用户需求: %s", question),
		Action: "parse_intent", Observation: "已识别查询意图",
		DurationMs: 0,
	})
	stepNum++

	// Step 2: 分析表结构（基于实际使用的表）
	if len(result.Tables) > 0 {
		tableNames := strings.Join(result.Tables, ", ")
		steps = append(steps, modelai.AgentStepInfo{
			Step: stepNum, Thought: "分析表结构与字段映射",
			Action: "analyze_schema", Observation: fmt.Sprintf("已定位 %d 张表: %s", len(result.Tables), tableNames),
			DurationMs: 0,
		})
		stepNum++
	}

	// Step 3: Schema Linking（表间关系）
	if len(result.Relations) > 0 {
		relCount := len(result.Relations)
		relDetails := make([]string, 0, relCount)
		for _, r := range result.Relations {
			relDetails = append(relDetails, fmt.Sprintf("%s.%s→%s.%s", r.SourceTable, r.SourceField, r.TargetTable, r.TargetField))
		}
		steps = append(steps, modelai.AgentStepInfo{
			Step: stepNum, Thought: "确定表间关联关系",
			Action: "schema_linking", Observation: fmt.Sprintf("已识别 %d 条关联: %s", relCount, strings.Join(relDetails, ", ")),
			DurationMs: 0,
		})
		stepNum++
	}

	// Step 4: LLM 生成 SQL（原始 CoT）
	steps = append(steps, modelai.AgentStepInfo{
		Step: stepNum, Thought: "基于表结构 + CoT 思维链生成 SQL",
		Action:      "generate_sql",
		Observation: fmt.Sprintf("LLM 已生成 SQL（含 CoT 推理 %d 字符）", len(result.RawCoT)),
		DurationMs:  0,
	})
	stepNum++

	// Step 5: 后处理修正（展示修正前后对比）
	correctDetail := "验证通过"
	if result.SelfReflectBefore != "" && result.SelfReflectAfter != "" &&
		result.SelfReflectBefore != result.SelfReflectAfter {
		correctDetail = fmt.Sprintf("修正前: %s → 修正后: %s",
			truncateSQL(result.SelfReflectBefore, 80),
			truncateSQL(result.SelfReflectAfter, 80))
	}
	if len(result.Warnings) > 0 {
		correctDetail += fmt.Sprintf("（%d 条警告）", len(result.Warnings))
	}
	steps = append(steps, modelai.AgentStepInfo{
		Step: stepNum, Thought: "后处理：规则修正 + 字段校验",
		Action:      "validate_and_fix",
		Observation: correctDetail,
		DurationMs:  0,
	})
	stepNum++

	// Step 6: 最终 SQL
	steps = append(steps, modelai.AgentStepInfo{
		Step: stepNum, Thought: "输出最终 SQL",
		Action:      "final_output",
		Observation: truncateSQL(result.SQL, 120),
		DurationMs:  0,
	})

	return steps
}

// truncateSQL 截断 SQL 用于展示
func truncateSQL(sql string, maxLen int) string {
	sql = strings.TrimSpace(sql)
	if len(sql) <= maxLen {
		return sql
	}
	return sql[:maxLen] + "..."
}

func (g *Generator) checkClarification(ctx context.Context, question string, history []ConversationMessage) *ClarificationResult {
	return g.checkClarificationWithKB(ctx, question, history, nil)
}

// shouldBypassClarification 规则化判定：以下情况直接放行不再澄清
// 1. 用户最近的消息含"我选择："前缀（已经回答了澄清问题）
// 2. 历史中上一轮 assistant 消息已包含澄清选项，本轮是用户的回答
// 3. 问题中包含"（已选择：xxx）"标记（前端包装）
// 4. 包含"基于update_time/创建时间"等明确字段名
// 5. 是延续/修改型问题（"再查"/"加上"等关键词 + 已有SQL历史）
// 6. 用户明确确认表名（如"xxx表正确"、"xxx表确认"）
// 7. 用户已经澄清了3次以上（强制不再澄清）
// 8. 用户问题中明确包含 kb 中存在的表名和字段名/查询意图
func shouldBypassClarification(question string, history []ConversationMessage, kb *KnowledgeBase) bool {
	if len(question) == 0 {
		return false
	}

	// 规则 1：用户刚刚回答了澄清
	lowerQ := strings.ToLower(question)
	if strings.HasPrefix(lowerQ, "我选择") || strings.HasPrefix(lowerQ, "我选") {
		return true
	}

	// 规则 2：前端包装的"已选择"标记
	if strings.Contains(question, "已选择：") || strings.Contains(question, "已选择:") {
		return true
	}

	// 规则 3：明确的时间字段引用（基于 update_time、基于创建时间等）
	fieldBypassPatterns := []string{
		"基于update_time", "基于update time", "基于 update_time", "基于 update time",
		"基于create_time", "基于create time", "基于 create_time", "基于 create time",
		"基于创建时间", "基于 更新时间", "基于修改时间", "基于  创建时间",
		"update_time 三天内", "update_time三天内", "update time 三天内",
		"创建时间三天内", "更新时间三天内", "修改时间三天内",
		"创建时间三天的", "更新时间三天的", "修改时间三天的",
		"创建时间往前", "更新时间往前", "修改时间往前",
	}
	for _, p := range fieldBypassPatterns {
		if strings.Contains(lowerQ, strings.ToLower(p)) {
			return true
		}
	}

	// 规则 4：明确包含"时间"+"三天"组合 + 历史有澄清（说明用户在做澄清回答）
	if strings.Contains(question, "三天") || strings.Contains(question, "3天") {
		// 如果历史中有过澄清，本轮基本是用户在做选择
		for _, m := range history {
			if m.Role == "assistant" && (strings.Contains(m.Content, "需要先确认") || strings.Contains(m.Content, "请从以下选项中选择")) {
				return true
			}
		}
	}

	// 规则 5：历史中有 SQL + 当前问题含延续/修改关键词
	if len(history) > 0 {
		hasHistorySQL := false
		for _, m := range history {
			if m.SQL != "" {
				hasHistorySQL = true
				break
			}
		}
		if hasHistorySQL {
			modifyKeywords := []string{"再", "继续", "然后", "接着", "换成", "改为", "改成", "加上", "去掉", "增加", "减少", "替换", "更新"}
			for _, kw := range modifyKeywords {
				if strings.Contains(lowerQ, kw) {
					return true
				}
			}
		}
	}

	// 规则 6：用户明确确认表名（如"xxx表正确"、"xxx表确认"、"xxx表在空间里"）
	// 检测用户回答中包含表名确认关键词
	tableConfirmPatterns := []string{"表正确", "表确认", "表在空间", "表存在", "表没问题", "表对的", "表是对的"}
	for _, p := range tableConfirmPatterns {
		if strings.Contains(question, p) {
			return true
		}
	}

	// 规则 7：用户已经澄清了3次以上（强制不再澄清）
	clarifyTurnCount := 0
	for _, m := range history {
		if m.Role == "assistant" && strings.Contains(m.Content, "需要先确认几个信息") {
			clarifyTurnCount++
		}
	}
	if clarifyTurnCount >= 3 {
		return true
	}

	// 规则 8：用户问题中明确包含 kb 中存在的表名，且包含字段名或查询意图
	if kb != nil {
		mentionedTables := extractMentionedTables(question, kb)
		if len(mentionedTables) > 0 {
			// 检查是否同时提到了字段或明确的查询意图
			hasFieldOrIntent := strings.Contains(lowerQ, "字段") || strings.Contains(lowerQ, "列") ||
				strings.Contains(lowerQ, "相差") || strings.Contains(lowerQ, "统计") ||
				strings.Contains(lowerQ, "查询") || strings.Contains(lowerQ, "查找") ||
				strings.Contains(lowerQ, "计算") || strings.Contains(lowerQ, "平均") ||
				strings.Contains(lowerQ, "最大") || strings.Contains(lowerQ, "最小") ||
				strings.Contains(lowerQ, "总和") || strings.Contains(lowerQ, "计数")
			// 或检查是否包含下划线命名字段（如 create_date, status_date）
			fieldRe := regexp.MustCompile(`\b[a-zA-Z_][a-zA-Z0-9_]*_[a-zA-Z_][a-zA-Z0-9_]*\b`)
			fields := fieldRe.FindAllString(question, -1)
			if hasFieldOrIntent || len(fields) >= 2 {
				return true
			}
		}
	}

	return false
}

func (g *Generator) checkClarificationWithKB(ctx context.Context, question string, history []ConversationMessage, kb *KnowledgeBase) *ClarificationResult {
	if g.llmProvider == nil {
		return nil
	}

	historyStr := buildConversationContext(history)

	// 构建包含历史SQL的上下文（取最后一条SQL，与主生成逻辑一致）
	var historySQLContext string
	for _, m := range history {
		if m.SQL != "" {
			historySQLContext = fmt.Sprintf("\n最近生成的SQL:\n```sql\n%s\n```", m.SQL)
		}
	}

	// 检查澄清轮数：超过 3 轮强制不再澄清
	clarifyTurnCount := 0
	for _, m := range history {
		if m.Role == "assistant" && strings.Contains(m.Content, "需要先确认几个信息") {
			clarifyTurnCount++
		}
	}
	if clarifyTurnCount >= 3 {
		logx.Infof("[Clarify] 澄清已达 %d 轮，跳过澄清直接生成", clarifyTurnCount)
		return nil
	}

	// 注入Schema摘要：让LLM能自主从表结构推断时间字段等，无需反复问用户
	schemaSummary := buildSchemaSummaryForClarification(kb, question)

	prompt := fmt.Sprintf(`判断以下用户输入是否足够明确以生成SQL查询。

## 用户输入
%s

## 对话历史
%s%s

## 当前数据库可用表结构摘要（LLM可自主据此推断）
%s

## 判断标准（按优先级）
1. **有明确上下文**：如果历史对话中已有SQL，用户是在该SQL基础上的修改/追问（包含"改成/加上/去掉/再查"等关键词）→ **不模糊**
2. **可从Schema推断**：如果用户问"查找X天内的数据"，但Schema中只有1个时间字段（如 update_time）→ **不模糊**，让SQL生成时从schema自动选择
3. **可从Schema推断**：如果Schema中有多张表都含相似字段，但用户的问题指向明确表名（即使是简写/别名）→ 优先按用户原话中提到的表名匹配 → **不模糊**
4. **可从对话历史推断**：用户说"当前实践往前推三天"等口语化表达，结合历史SQL可推断 → **不模糊**
5. **代词上下文明确**：用户使用"这个/那个/它"等代词，但历史SQL/对话中有明确指代对象 → **不模糊**
6. **解释性问题**：用户问"什么意思/为什么"等 → **不模糊**（返回解释而非SQL）
7. **真正模糊**：用户输入确实无法从Schema、历史或上下文推断出表名/字段/条件 → 才返回澄清选项

## 反例（这些情况应直接生成SQL，不应要求澄清）
- 用户问"查找三天内的数据"，Schema中只有 update_time 字段 → 直接生成 WHERE update_time >= NOW() - INTERVAL 3 DAY
- 用户说"BM_DATA_DICT 查找数据"，Schema中存在 BM_DATA_DICT 表 → 直接使用该表
- 用户在历史中选了"基于 update_time"，后续问题中含"已选择：基于 update_time"标记 → 直接使用 update_time
- 用户明确说"统计表twb_send_sms 字段create_date和status_date相差的时间"，Schema中存在该表和字段 → 直接生成SQL计算时间差
- 用户明确指定了完整的表名和字段名 → 直接生成SQL，不要澄清

## 澄清选项生成要求
- 只有在"真正模糊"时返回 needed: true
- 给出 3 个**真正不明确的元素**的澄清选项（如时间字段、具体表名）
- 避免问"您想用哪个时间字段"这种 LLM 自己能从 Schema 推断的问题

请严格按JSON格式输出（必须是合法JSON）：
{"needed": true/false, "options": ["选项1", "选项2", "选项3"]}`, question, historyStr, historySQLContext, schemaSummary)

	resp, err := g.llmProvider.Chat(ctx, "你是一个意图理解助手。", []Message{
		{Role: "user", Content: prompt},
	})
	if err != nil {
		return nil
	}
	return parseClarification(resp.Content)
}

// buildSchemaSummaryForClarification 构建Schema摘要用于澄清判定
// 让 LLM 看到表名列表和关键时间字段，避免因不知道Schema而反复要求澄清
// 用户明确提到的表会被优先包含，不受20张表限制截断
func buildSchemaSummaryForClarification(kb *KnowledgeBase, question string) string {
	if kb == nil {
		return "（Schema摘要不可用）"
	}

	tables := kb.GetAllTables()
	if len(tables) == 0 {
		return "（当前知识库无表）"
	}

	// 提取用户提到的表名
	mentionedSet := make(map[string]bool)
	if question != "" {
		mentionedTables := extractMentionedTables(question, kb)
		for _, mt := range mentionedTables {
			mentionedSet[mt.TableNam] = true
		}
	}

	// 限制最多展示 20 张表，避免 prompt 过长
	// 但用户提到的表优先展示，不受限制
	maxTables := 20
	tableList := make([]*modelai.TableCache, 0, len(tables))
	for _, t := range tables {
		tableList = append(tableList, t)
	}

	// 排序：用户提到的表优先，其余按字母顺序
	sort.Slice(tableList, func(i, j int) bool {
		iMentioned := mentionedSet[tableList[i].TableNam]
		jMentioned := mentionedSet[tableList[j].TableNam]
		if iMentioned && !jMentioned {
			return true
		}
		if !iMentioned && jMentioned {
			return false
		}
		return tableList[i].TableNam < tableList[j].TableNam
	})

	// 截断时保留用户提到的表
	mentionedCount := 0
	for _, t := range tableList {
		if mentionedSet[t.TableNam] {
			mentionedCount++
		}
	}
	if len(tableList) > maxTables {
		// 至少保留用户提到的表，再加其他表直到 maxTables
		tablesToKeep := mentionedCount
		if tablesToKeep < maxTables {
			tablesToKeep = maxTables
		}
		if tablesToKeep > len(tableList) {
			tablesToKeep = len(tableList)
		}
		tableList = tableList[:tablesToKeep]
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("共 %d 张表（含时间字段）：\n", len(tables)))

	timeFieldPatterns := []string{"time", "date", "create", "update", "modified", "_at", "时间", "日期"}
	for _, t := range tableList {
		// 提取时间字段
		var timeFields []string
		var allFields []string
		for _, col := range t.ColumnsJSON {
			allFields = append(allFields, col.Name)
			colLower := strings.ToLower(col.Name)
			for _, p := range timeFieldPatterns {
				if strings.Contains(colLower, p) {
					timeFields = append(timeFields, col.Name)
					break
				}
			}
		}
		if len(timeFields) > 0 {
			sb.WriteString(fmt.Sprintf("- %s: 时间字段=[%s]\n", t.TableNam, strings.Join(timeFields, ", ")))
		} else {
			sb.WriteString(fmt.Sprintf("- %s: 字段=[%s]\n", t.TableNam, strings.Join(allFields, ", ")))
		}
	}
	return sb.String()
}

func parseClarification(content string) *ClarificationResult {
	result := &ClarificationResult{}
	jsonStr := utils.ExtractJSON(content)
	if jsonStr == "" {
		return nil
	}

	// 严格的 JSON 解析：先尝试反序列化到结构体
	type clarifyJSON struct {
		Needed  bool     `json:"needed"`
		Options []string `json:"options"`
	}
	var parsed clarifyJSON
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err == nil {
		// 解析成功：使用真实字段值
		result.Needed = parsed.Needed
		result.Options = parsed.Options
		return result
	}

	// 降级：正则提取关键字段（兜底）
	result.Needed = strings.Contains(jsonStr, `"needed":true`) || strings.Contains(jsonStr, `"needed": true`)
	// 不再用模糊的 strings.Contains("true")，避免误判

	if strings.Contains(jsonStr, `"options"`) {
		// 尝试用正则提取 options 数组中的字符串
		re := regexp.MustCompile(`"options"\s*:\s*\[(.*?)\]`)
		if m := re.FindStringSubmatch(jsonStr); len(m) > 1 {
			itemRe := regexp.MustCompile(`"([^"\\]*(?:\\.[^"\\]*)*)"`)
			for _, item := range itemRe.FindAllStringSubmatch(m[1], -1) {
				if len(item) > 1 {
					result.Options = append(result.Options, item[1])
				}
			}
		}
	}
	return result
}
