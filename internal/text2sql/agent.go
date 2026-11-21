package text2sql

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
	modelai "github.com/listen808/citebase/internal/model/ai"
	"github.com/listen808/citebase/internal/utils"
)

// AgentStep 记录 ReAct 循环中的一步
type AgentStep struct {
	Step        int                    `json:"step"`
	Thought     string                 `json:"thought"`
	Action      string                 `json:"action"`
	ActionInput map[string]interface{} `json:"action_input,omitempty"`
	Observation string                 `json:"observation"`
	DurationMs  int64                  `json:"duration_ms"`
}

// AgentResult Agent 最终输出
type AgentResult struct {
	SQL            string             `json:"sql"`
	Explanation    string             `json:"explanation"`
	Confidence     float64            `json:"confidence"`
	Tables         []string           `json:"tables"`
	Result         *modelai.SQLResult `json:"-"`
	Steps          []AgentStep        `json:"steps"`
	Interpretation string             `json:"interpretation,omitempty"`
}

// StreamEvent 流式输出事件
type StreamEvent struct {
	Type    string `json:"type"` // "thought", "action", "observation", "sql", "explanation", "done", "error", "clarify"
	Content string `json:"content"`
	Step    int    `json:"step"`
	Done    bool   `json:"done"`
}

// AgentReActConfig Agent 配置
type AgentReActConfig struct {
	MaxSteps        int               // 最大 ReAct 循环步数
	StreamCallback  func(StreamEvent) // 流式回调
	EnableClarify   bool              // 是否启用主动澄清
	EnableInterpret bool              // 是否启用结果解读
}

// DefaultAgentConfig 默认配置
func DefaultAgentConfig() *AgentReActConfig {
	return &AgentReActConfig{
		MaxSteps:        8,
		EnableClarify:   true,
		EnableInterpret: true,
	}
}

// Agent Text2SQL ReAct Agent
type Agent struct {
	provider          provider.Provider
	generator         *Generator
	tools             map[string]*AgentTool
	conversationState *ConversationState
	feedbackLoop      *FeedbackLoop
	resultInterpreter *ResultInterpreter
}

// NewAgent 创建 Agent
func NewAgent(p provider.Provider, gen *Generator) *Agent {
	a := &Agent{
		provider:          p,
		generator:         gen,
		conversationState: gen.conversationState,
		feedbackLoop:      gen.feedbackLoop,
		tools:             make(map[string]*AgentTool),
		resultInterpreter: NewResultInterpreter(p),
	}
	a.registerTools()
	return a
}

// Generate 使用 Agent 生成 SQL（ReAct 模式）
func (a *Agent) Generate(ctx context.Context, text string, spaceID uint64, cfg *AgentReActConfig) (*AgentResult, error) {
	if cfg == nil {
		cfg = DefaultAgentConfig()
	}

	startTime := time.Now()

	// P1 首要任务 1/2：会话状态与知识库从请求 ctx 解析（无绑定时回退到实例字段），
	// 进程级单例字段不再被并发请求互相覆盖/借用
	state := a.convStateFrom(ctx)
	kb := a.generator.kbFromContext(ctx)

	// 记录对话
	state.RecordTurn(text, "", nil)
	intent := state.DetectIntent(text)
	resolvedText := state.ResolveReference(text)

	// 构建系统提示词
	systemPrompt := a.buildAgentSystemPrompt(spaceID)
	convCtx := state.GetContextForLLM(intent.IntentType)

	// 构建消息
	messages := []provider.Message{
		provider.NewTextMessage("user", resolvedText),
	}

	// 注入对话上下文
	if convCtx != "" {
		contextMsg := provider.NewTextMessage("system", "以下是之前的对话历史，请参考：\n\n"+convCtx)
		messages = append([]provider.Message{contextMsg}, messages...)
	}

	// 转换工具定义
	toolDefs := a.buildToolDefinitions()

	// ReAct 循环
	steps := make([]AgentStep, 0, cfg.MaxSteps)
	var finalSQL string
	var finalExplanation string
	var finalTables []string

	for step := 0; step < cfg.MaxSteps; step++ {
		stepStart := time.Now()

		// 调用 LLM（带工具）
		resp, err := a.provider.ChatWithTools(ctx, systemPrompt, messages, toolDefs)
		if err != nil {
			a.emitStream(cfg, StreamEvent{Type: "error", Content: fmt.Sprintf("LLM调用失败: %v", err), Done: true})
			return nil, fmt.Errorf("agent LLM call failed: %w", err)
		}

		// 解析 LLM 响应
		content := resp.Content
		thought, action, actionInput := a.parseReActResponse(content)

		if cfg.StreamCallback != nil {
			cfg.StreamCallback(StreamEvent{
				Type:    "thought",
				Content: thought,
				Step:    step + 1,
			})
		}

		// 如果 LLM 返回了 tool calls（原生 Function Calling 模式）
		if len(resp.ToolCalls) > 0 {
			for _, tc := range resp.ToolCalls {
				var args map[string]interface{}
				if err := json.Unmarshal(tc.Arguments, &args); err != nil {
					args = map[string]interface{}{"raw": string(tc.Arguments)}
				}

				obs := a.executeTool(ctx, tc.Name, args, spaceID, cfg)

				if cfg.StreamCallback != nil {
					cfg.StreamCallback(StreamEvent{
						Type:    "action",
						Content: fmt.Sprintf("调用工具: %s", tc.Name),
						Step:    step + 1,
					})
					cfg.StreamCallback(StreamEvent{
						Type:    "observation",
						Content: utils.TruncateWithEllipsis(obs, 200),
						Step:    step + 1,
					})
				}

				steps = append(steps, AgentStep{
					Step:        step + 1,
					Thought:     thought,
					Action:      tc.Name,
					ActionInput: args,
					Observation: utils.TruncateWithEllipsis(obs, 500),
					DurationMs:  time.Since(stepStart).Milliseconds(),
				})

				// 添加 assistant 消息（含 tool call）
				messages = append(messages, provider.NewToolCallMessage(content, resp.ToolCalls))
				// 添加 tool 结果
				messages = append(messages, provider.NewToolResultMessage(tc.ID, obs))

				// 检查是否是 clarify 工具
				if tc.Name == "clarify" && cfg.EnableClarify {
					if cfg.StreamCallback != nil {
						cfg.StreamCallback(StreamEvent{
							Type:    "clarify",
							Content: obs,
							Step:    step + 1,
							Done:    true,
						})
					}
					return &AgentResult{
						Explanation: obs,
						Steps:       steps,
						Confidence:  0,
					}, nil
				}

				// 检查是否已生成最终 SQL
				if tc.Name == "final_answer" {
					if sqlStr, ok := args["sql"].(string); ok && sqlStr != "" {
						finalSQL = sqlStr
					}
					if expl, ok := args["explanation"].(string); ok {
						finalExplanation = expl
					}
					if tablesRaw, ok := args["tables"]; ok {
						switch t := tablesRaw.(type) {
						case []interface{}:
							for _, tbl := range t {
								if s, ok := tbl.(string); ok {
									finalTables = append(finalTables, s)
								}
							}
						}
					}
				}
			}
		} else if action != "" {
			// 文本解析模式（降级）：从 LLM 文本响应中提取 Thought/Action
			obs := a.executeTool(ctx, action, actionInput, spaceID, cfg)

			if cfg.StreamCallback != nil {
				cfg.StreamCallback(StreamEvent{
					Type:    "action",
					Content: fmt.Sprintf("执行: %s", action),
					Step:    step + 1,
				})
				cfg.StreamCallback(StreamEvent{
					Type:    "observation",
					Content: utils.TruncateWithEllipsis(obs, 200),
					Step:    step + 1,
				})
			}

			steps = append(steps, AgentStep{
				Step:        step + 1,
				Thought:     thought,
				Action:      action,
				ActionInput: actionInput,
				Observation: utils.TruncateWithEllipsis(obs, 500),
				DurationMs:  time.Since(stepStart).Milliseconds(),
			})

			// 添加 assistant 消息和 tool 结果
			messages = append(messages, provider.NewTextMessage("assistant", content))
			messages = append(messages, provider.NewTextMessage("user",
				fmt.Sprintf("工具 %s 的执行结果：\n%s\n\n请继续思考，或使用 final_answer 工具提交最终 SQL。", action, obs)))

			// 检查是否是 clarify 工具
			if action == "clarify" && cfg.EnableClarify {
				if cfg.StreamCallback != nil {
					cfg.StreamCallback(StreamEvent{
						Type:    "clarify",
						Content: obs,
						Step:    step + 1,
						Done:    true,
					})
				}
				return &AgentResult{
					Explanation: obs,
					Steps:       steps,
					Confidence:  0,
				}, nil
			}
		} else {
			// 没有工具调用也没有 action → 可能是最终答案
			// 尝试从文本中提取 SQL
			sql := extractSQLFromText(content)
			if sql != "" {
				finalSQL = sql
				finalExplanation = content
				steps = append(steps, AgentStep{
					Step:        step + 1,
					Thought:     thought,
					Action:      "final_answer",
					Observation: "从文本中提取SQL",
					DurationMs:  time.Since(stepStart).Milliseconds(),
				})
				break
			}

			// 如果无法提取 SQL，提示 LLM 继续
			messages = append(messages, provider.NewTextMessage("assistant", content))
			messages = append(messages, provider.NewTextMessage("user",
				"请使用工具来完成任务。你需要先调用 search_schema 查看表结构，然后生成 SQL，最后用 execute_sql 验证。如果确认无误，用 final_answer 提交。"))
		}

		// 如果已有 final_answer，退出循环
		if finalSQL != "" {
			break
		}
	}

	// 如果没有生成 SQL，尝试最后一次 LLM 调用
	if finalSQL == "" {
		messages = append(messages, provider.NewTextMessage("user",
			"请基于以上所有信息，生成最终的 SQL 查询。使用 final_answer 工具提交，包含 sql、explanation 和 tables 字段。"))
		resp, err := a.provider.ChatWithTools(ctx, systemPrompt, messages, toolDefs)
		if err == nil {
			for _, tc := range resp.ToolCalls {
				if tc.Name == "final_answer" {
					var args map[string]interface{}
					_ = json.Unmarshal(tc.Arguments, &args)
					if sqlStr, ok := args["sql"].(string); ok {
						finalSQL = sqlStr
					}
					if expl, ok := args["explanation"].(string); ok {
						finalExplanation = expl
					}
				}
			}
			// 降级：从文本提取
			if finalSQL == "" {
				finalSQL = extractSQLFromText(resp.Content)
				finalExplanation = resp.Content
			}
		}
	}

	if finalSQL == "" {
		return nil, fmt.Errorf("Agent 未能生成有效 SQL，已尝试 %d 步", len(steps))
	}

	// 执行 SQL 验证
	result := &modelai.SQLResult{
		SQL:          finalSQL,
		Explanations: []string{finalExplanation},
		Tables:       finalTables,
	}

	// 尝试执行
	confidence := 0.85
	if a.generator.executor != nil && spaceID > 0 {
		// P1 首要任务 1：试执行沿 ctx 携带真实调用者身份，恢复自动纠错闭环
		execResult := a.generator.executor.ExecuteWithContext(ctx, finalSQL, spaceID)
		if execResult.Success {
			result.ExecutionResult = execResult
			confidence = 0.95

			// 反馈闭环：记录成功
			a.feedbackLoop.RecordSuccess(FeedbackRecord{
				SpaceID:      int(spaceID),
				UserQuery:    text,
				GeneratedSQL: finalSQL,
				Duration:     time.Since(startTime),
			})
		} else {
			// 尝试 AST 修正
			availableFields := a.generator.buildAvailableFieldsMap(kb)
			correction := a.generator.astCorrector.Correct(finalSQL, execResult.Error, availableFields)
			if correction.Corrected {
				finalSQL = correction.CorrectedSQL
				result.SQL = finalSQL

				execResult2 := a.generator.executor.ExecuteWithContext(ctx, finalSQL, spaceID)
				if execResult2.Success {
					result.ExecutionResult = execResult2
					confidence = 0.85
					result.Warnings = append(result.Warnings, "AST自动修正: "+correction.Suggestion)

					a.feedbackLoop.RecordCorrection(FeedbackRecord{
						SpaceID:       int(spaceID),
						UserQuery:     text,
						GeneratedSQL:  result.SQL,
						CorrectedSQL:  finalSQL,
						CorrectionSrc: "ast",
						Duration:      time.Since(startTime),
					})
				} else {
					result.ExecutionResult = execResult2
					a.feedbackLoop.RecordFailure(FeedbackRecord{
						SpaceID:      int(spaceID),
						UserQuery:    text,
						GeneratedSQL: finalSQL,
						ErrorType:    classifyErrorType(execResult2.Error),
						ErrorMessage: execResult2.Error,
						Duration:     time.Since(startTime),
					})
				}
			} else {
				result.ExecutionResult = execResult
				a.feedbackLoop.RecordFailure(FeedbackRecord{
					SpaceID:      int(spaceID),
					UserQuery:    text,
					GeneratedSQL: finalSQL,
					ErrorType:    classifyErrorType(execResult.Error),
					ErrorMessage: execResult.Error,
					Duration:     time.Since(startTime),
				})
			}
		}
	}

	// 记录意图
	state.RecordIntent(intent)

	// 缓存使用的表结构
	for _, table := range finalTables {
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
			state.RecordTableUsage(table, columns)
			a.generator.schemaCache.CacheTable(tableRowSpaceID(tableCache.SpaceID), table, columns, false)
		}
	}

	// 结果解读
	var interpretation string
	if cfg.EnableInterpret && result.ExecutionResult != nil {
		if execRes, ok := result.ExecutionResult.(*ExecResult); ok && execRes.Success {
			interp, err := a.resultInterpreter.Interpret(ctx, text, finalSQL, execRes)
			if err == nil {
				interpretation = interp
				if cfg.StreamCallback != nil {
					cfg.StreamCallback(StreamEvent{
						Type:    "interpretation",
						Content: interp,
						Step:    len(steps) + 1,
					})
				}
			}
		}
	}

	agentResult := &AgentResult{
		SQL:            finalSQL,
		Explanation:    finalExplanation,
		Confidence:     confidence,
		Tables:         finalTables,
		Result:         result,
		Steps:          steps,
		Interpretation: interpretation,
	}

	if cfg.StreamCallback != nil {
		cfg.StreamCallback(StreamEvent{
			Type:    "sql",
			Content: finalSQL,
			Step:    len(steps) + 1,
		})
		cfg.StreamCallback(StreamEvent{
			Type:    "explanation",
			Content: finalExplanation,
			Step:    len(steps) + 1,
		})
		cfg.StreamCallback(StreamEvent{Type: "done", Content: "", Done: true})
	}

	return agentResult, nil
}

// buildAgentSystemPrompt 构建 Agent 系统提示词
func (a *Agent) buildAgentSystemPrompt(spaceID uint64) string {
	return `你是一个数据库查询专家 Agent。你使用 ReAct（推理-行动）模式工作。

## 工作流程
1. 理解用户问题，确定需要查询哪些表
2. 使用 search_schema 工具查看表结构
3. 如需更多信息，使用 get_columns 或 get_relations 工具
4. 使用 search_examples 查找相似的历史查询作为参考
5. 生成 SQL 并用 execute_sql 验证
6. 如 SQL 执行失败，分析错误并修正
7. 确认无误后，用 final_answer 提交最终 SQL

## 核心规则
- 必须先用 search_schema 查看表结构，不要猜测字段名
- 生成 SQL 后必须用 execute_sql 验证
- 如果字段名不确定，使用 clarify 工具向用户确认
- 所有字段名必须从 search_schema 返回的 DDL 中查找
- 使用 MySQL 标准语法，字段使用 表别名.字段名 格式
- 只生成 SELECT 查询

## 可用工具
- search_schema: 搜索表结构，输入关键词或表名
- get_columns: 获取指定表的详细列信息
- get_relations: 获取表之间的关联关系
- search_examples: 搜索相似的历史 SQL 查询示例
- execute_sql: 执行 SQL 验证（仅限 SELECT）
- validate_sql: 校验 SQL 结构合法性
- clarify: 当字段名或查询意图有歧义时，向用户确认
- final_answer: 提交最终 SQL 结果

## 输出规范
最终答案使用 final_answer 工具，必须包含：
- sql: 完整的 SQL 语句
- explanation: 生成逻辑的中文解释
- tables: 涉及的表名列表`
}

// parseReActResponse 从 LLM 文本响应中解析 Thought/Action
func (a *Agent) parseReActResponse(content string) (thought, action string, actionInput map[string]interface{}) {
	lines := strings.Split(content, "\n")
	var lastInputStr string

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "Thought:") || strings.HasPrefix(trimmed, "思考：") {
			thought = strings.TrimPrefix(strings.TrimPrefix(trimmed, "Thought:"), "思考：")
			thought = strings.TrimSpace(thought)
		}

		if strings.HasPrefix(trimmed, "Action:") || strings.HasPrefix(trimmed, "行动：") {
			action = strings.TrimPrefix(strings.TrimPrefix(trimmed, "Action:"), "行动：")
			action = strings.TrimSpace(action)
		}

		if strings.HasPrefix(trimmed, "Action Input:") || strings.HasPrefix(trimmed, "行动输入：") {
			lastInputStr = strings.TrimPrefix(strings.TrimPrefix(trimmed, "Action Input:"), "行动输入：")
			lastInputStr = strings.TrimSpace(lastInputStr)

			// 尝试解析 JSON
			actionInput = make(map[string]interface{})
			if err := json.Unmarshal([]byte(lastInputStr), &actionInput); err != nil {
				// 不是 JSON，尝试 key=value 格式
				actionInput["query"] = lastInputStr
			}
		}

		// 检查是否有连续的 Action Input 行（JSON 多行）
		if actionInput != nil && i+1 < len(lines) {
			nextLine := strings.TrimSpace(lines[i+1])
			if strings.HasPrefix(nextLine, "{") || strings.HasPrefix(nextLine, "[") {
				// 尝试合并多行 JSON
				fullJSON := lastInputStr
				for j := i + 1; j < len(lines); j++ {
					fullJSON += "\n" + strings.TrimSpace(lines[j])
				}
				var parsed map[string]interface{}
				if err := json.Unmarshal([]byte(fullJSON), &parsed); err == nil {
					actionInput = parsed
				}
			}
		}
	}

	return
}

// executeTool 执行工具调用
func (a *Agent) executeTool(ctx context.Context, toolName string, args map[string]interface{}, spaceID uint64, cfg *AgentReActConfig) string {
	tool, ok := a.tools[toolName]
	if !ok {
		return fmt.Sprintf("未知工具: %s", toolName)
	}

	result, err := tool.Handler(ctx, args)
	if err != nil {
		logx.Infof("[Agent] Tool %s failed: %v", toolName, err)
		return fmt.Sprintf("工具执行失败: %v", err)
	}

	return result
}

// buildToolDefinitions 构建工具定义列表
func (a *Agent) buildToolDefinitions() []provider.ToolDefinition {
	defs := make([]provider.ToolDefinition, 0, len(a.tools))
	for _, tool := range a.tools {
		defs = append(defs, tool.Definition)
	}
	return defs
}

// emitStream 发送流式事件
func (a *Agent) emitStream(cfg *AgentReActConfig, event StreamEvent) {
	if cfg.StreamCallback != nil {
		cfg.StreamCallback(event)
	}
}

// extractSQLFromText 从文本中提取 SQL
func extractSQLFromText(content string) string {
	// 尝试提取 ```sql ... ``` 代码块
	if idx := strings.Index(content, "```sql"); idx != -1 {
		start := idx + 6
		if end := strings.Index(content[start:], "```"); end != -1 {
			return strings.TrimSpace(content[start : start+end])
		}
	}
	// 尝试提取 ``` ... ``` 代码块
	if idx := strings.Index(content, "```"); idx != -1 {
		start := idx + 3
		// 跳过可能的语言标识
		if nl := strings.Index(content[start:], "\n"); nl != -1 && nl < 20 {
			start += nl + 1
		}
		if end := strings.Index(content[start:], "```"); end != -1 {
			sql := strings.TrimSpace(content[start : start+end])
			if strings.HasPrefix(strings.ToUpper(sql), "SELECT") {
				return sql
			}
		}
	}
	// 尝试直接查找 SELECT 语句
	upper := strings.ToUpper(content)
	if idx := strings.Index(upper, "SELECT"); idx != -1 {
		// 从 SELECT 开始截取到分号或结尾
		remainder := content[idx:]
		if semiIdx := strings.Index(remainder, ";"); semiIdx != -1 {
			return strings.TrimSpace(remainder[:semiIdx+1])
		}
		// 没有分号，取到结尾
		return strings.TrimSpace(remainder)
	}
	return ""
}
