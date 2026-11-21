package text2sql

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"regexp"
	"strings"
	"time"

	modelai "github.com/listen808/citebase/internal/model/ai"
)

// Text2SQL 生成器：RAG 增强生成、自反思、实体转换。

// SetRAGAdapter 设置 RAG 适配器
func (g *Generator) SetRAGAdapter(adapter *SQLRAGAdapter) {
	g.ragAdapter = adapter
}

// GenerateWithRAG 使用 RAG 增强生成 SQL
// 兼容旧签名：无调用者身份时，生成期试执行仍会被执行链 fail-closed 拒绝。
// 需要恢复自动验证/自纠错的调用方请改用 GenerateWithRAGContext 传入请求 ctx。
func (g *Generator) GenerateWithRAG(text string, spaceID uint64) (*modelai.SQLResult, []RAGSchemaSource, error) {
	return g.GenerateWithRAGContext(context.Background(), text, spaceID)
}

// GenerateWithRAGContext 带请求上下文版本：ctx 携带真实调用者身份
// （provider.ContextWithUserID / WithRequestScope），供清单 35 的试执行使用。
func (g *Generator) GenerateWithRAGContext(ctx context.Context, text string, spaceID uint64) (*modelai.SQLResult, []RAGSchemaSource, error) {
	kb := NewKnowledgeBase(g.repo)
	if err := kb.LoadFromSpace(spaceID); err != nil {
		return nil, nil, fmt.Errorf("加载空间知识库失败: %w", err)
	}

	// 如果指定空间没有表缓存，回退加载全部表缓存
	if len(kb.GetAllTables()) == 0 {
		caches, err := g.repo.GetAllTableCaches()
		if err != nil {
			return nil, nil, fmt.Errorf("加载全部表缓存失败: %w", err)
		}
		for i := range caches {
			kb.tables[caches[i].TableNam] = &caches[i]
		}
	}

	if err := kb.LoadRules(); err != nil {
		return nil, nil, fmt.Errorf("加载规则失败: %w", err)
	}

	// 如果有 LLM provider，使用 RAG + LLM 生成 SQL
	if g.llmProvider != nil {
		return g.generateWithRAGAndLLM(ctx, text, kb, int(spaceID))
	}

	// 否则使用基于规则的生成
	result, err := g.generateWithRules(text, kb)
	return result, nil, err
}

// generateWithRAGAndLLM 使用 RAG 检索 + LLM 生成 SQL
// P1（清单 113/35）：沿调用链透传 ctx，LLM 调用与试执行不再一律 context.Background()。
func (g *Generator) generateWithRAGAndLLM(ctx context.Context, text string, kb *KnowledgeBase, spaceID int) (*modelai.SQLResult, []RAGSchemaSource, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var ragSources []RAGSchemaSource

	if g.ragAdapter != nil {
		ragSources = g.ragAdapter.RetrieveRelevantSchemas(text, spaceID, 10)
	}

	var schemaContext string
	if g.ragAdapter != nil {
		schemaContext = g.ragAdapter.BuildSchemaContext(kb, ragSources, text)
	} else {
		schemaContext = g.buildSchemaContext(text, kb)
	}

	relationsContext := g.buildRelationsContext(kb)

	qType := ClassifyQuery(text)

	// 收集所有表名供结构校验
	var schemaTables []string
	for name := range kb.GetAllTables() {
		schemaTables = append(schemaTables, name)
	}

	// 最多重试 3 次
	maxRetries := 3
	var lastSQL string
	var lastExecError string

	for retry := 0; retry < maxRetries; retry++ {
		tpl, ok := g.getPromptTemplateFromDB(qType, spaceID)
		if !ok {
			tpl = GetPromptTemplate(qType)
		}

		prompt := strings.ReplaceAll(strings.ReplaceAll(tpl.UserTemplate, "{question}", text), "{schema_context}", schemaContext)
		prompt = strings.ReplaceAll(prompt, "{relations}", relationsContext)
		prompt = strings.ReplaceAll(prompt, "{schemaContext}", schemaContext)
		fieldMappings := detectMissingFields(text, kb)
		if fieldMappings != "" {
			prompt = fieldMappings + prompt
		}
		// 注入 Few-shot 示例
		if retry == 0 {
			fewShotBlock := g.retrieveFewShotExamples(text, spaceID, 3)
			if fewShotBlock != "" {
				prompt = fewShotBlock + "\n\n" + prompt
			}
		}

		// 重试时附加前一次的错误信息
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

		messages := []Message{
			NewTextMessage("user", prompt),
		}

		resp, err := g.llmProvider.Chat(ctx, tpl.SystemPrompt, messages)
		if err != nil {
			if retry < maxRetries-1 {
				continue
			}
			return nil, ragSources, fmt.Errorf("LLM 调用失败: %w", err)
		}

		result := g.parseLLMResponse(resp.Content, kb)

		if result.SQL != "" {
			// 统一后处理：Self-Reflection + 后向字段校验 + 确定性字段验证
			processedSQL, procWarnings, hasError := g.postProcessSQL(ctx, text, result.SQL, schemaContext, relationsContext, kb, schemaTables)
			result.SQL = processedSQL
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
				return result, ragSources, nil
			}
			continue
		}
		return result, ragSources, nil
	}

	return nil, ragSources, fmt.Errorf("多次生成 SQL 均存在结构性问题，请优化提示词或检查表结构")
}

func (g *Generator) selfReflect(ctx context.Context, question, sql, schemaContext, relationsContext string) (string, error) {
	refPrompt := strings.ReplaceAll(reflectionPromptTemplate, "{question}", question)
	refPrompt = strings.ReplaceAll(refPrompt, "{sql}", sql)
	refPrompt = strings.ReplaceAll(refPrompt, "{schema_context}", schemaContext)
	refPrompt = strings.ReplaceAll(refPrompt, "{relations}", relationsContext)

	resp, err := g.llmProvider.Chat(ctx, "你是一位严格的 MySQL SQL 代码审查专家。只使用 MySQL 兼容语法，禁止使用 Oracle 专有函数（如 DBMS_LOB、VARCHAR2、CAST(...AS VARCHAR2)、ROWNUM 等）。DDL 中的 CLOB 类型在 MySQL 中对应 TEXT 类型，直接使用 = 和 LIKE 即可，无需类型转换。", []Message{
		NewTextMessage("user", refPrompt),
	})
	if err != nil {
		return "", err
	}
	logx.Infof("[Text2SQL-DIAG] selfReflect LLM response: content=%q, len=%d", resp.Content, len(resp.Content))

	// 支持 BRACKETjson 和标准 ```json 两种格式
	jsonRe := regexp.MustCompile("(?s)(?:BRACKETjson|```json)\\s*(.*?)\\s*(?:BRACKET|```)")
	matches := jsonRe.FindStringSubmatch(resp.Content)
	if len(matches) < 2 {
		return "", nil
	}

	jsonStr := strings.TrimSpace(matches[1])

	type reflectResult struct {
		Passed       bool   `json:"passed"`
		CorrectedSQL string `json:"corrected_sql"`
	}
	var result reflectResult
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		sqlRe := regexp.MustCompile(`(?s)"corrected_sql"\s*:\s*"((?:[^"\\]|\\.)*)"`)
		sqlMatches := sqlRe.FindStringSubmatch(jsonStr)
		if len(sqlMatches) > 1 {
			corrected := sqlMatches[1]
			corrected = strings.ReplaceAll(corrected, `\"`, `"`)
			corrected = strings.ReplaceAll(corrected, `\\n`, "\n")
			corrected = strings.ReplaceAll(corrected, `\n`, "\n")
			return corrected, nil
		}
		return "", nil
	}

	if !result.Passed && result.CorrectedSQL != "" {
		return result.CorrectedSQL, nil
	}

	return "", nil
}

// convertEntities 转换实体
func (g *Generator) convertEntities(input *RecognizedInput) []modelai.RecognizedEntity {
	var entities []modelai.RecognizedEntity

	for _, table := range input.Tables {
		entities = append(entities, modelai.RecognizedEntity{
			Type:  "table",
			Value: table,
		})
	}

	for table, fields := range input.Fields {
		for _, field := range fields {
			entities = append(entities, modelai.RecognizedEntity{
				Type:  "field",
				Value: field,
				Field: table,
			})
		}
	}

	return entities
}
