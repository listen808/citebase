package rag

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"runtime/debug"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
)

// AgentStreamEvent Agent 流式事件
// 事件类型：thinking / tool_call / tool_result / content / done / error / component
type AgentStreamEvent struct {
	Type       string             `json:"type"`                  // thinking | tool_call | tool_result | content | done | error | component
	Content    string             `json:"content,omitempty"`     // thinking/content 时有效
	Round      int                `json:"round,omitempty"`       // 当前 ReAct 轮次
	ToolCall   *provider.ToolCall `json:"tool_call,omitempty"`   // tool_call 时有效
	ToolResult *ToolResult        `json:"tool_result,omitempty"` // tool_result 时有效
	Sources    []Source           `json:"sources,omitempty"`     // done 时有效
	Error      string             `json:"error,omitempty"`       // error 时有效
	DurationMs int64              `json:"duration_ms,omitempty"` // done 时有效
	Component  json.RawMessage    `json:"component,omitempty"`   // component 事件时有效
}

// sendStreamEvent 向 outCh 投递事件。客户端断开后没有人再消费 outCh，
// 裸发送在写满容量 64 的缓冲后会永久阻塞并泄漏生产协程，因此必须同时监听 ctx.Done()。
// 返回 false 表示上下文已结束，调用方应立即退出协程。
func sendStreamEvent(ctx context.Context, outCh chan<- AgentStreamEvent, ev AgentStreamEvent) bool {
	select {
	case outCh <- ev:
		return true
	case <-ctx.Done():
		return false
	}
}

// recoverStreamProducer 捕获生产协程里的 panic（工具实现、LLM 返回结构异常都可能触发），
// 避免一次异常直接打崩整个进程；需在 defer close(outCh) 之后注册，使日志先于关闭执行。
func recoverStreamProducer(label string) {
	if r := recover(); r != nil {
		logx.Errorf("[AgentChat-Stream] %s producer panic recovered: %v\n%s", label, r, debug.Stack())
	}
}

// ChatWithSpaceStream 带空间上下文的流式 Agent 对话
// 返回 channel，消费方按事件类型转发 SSE
func (a *AgentChat) ChatWithSpaceStream(ctx context.Context, question string, history []ChatMessage, spaceID int, limit int, images ...string) (<-chan AgentStreamEvent, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("AI服务未启用")
	}

	// P1-5: 复杂查询跳过预检索，让 Agent 自主多轮检索
	complexity := EstimateQueryComplexity(question)
	skipPreRetrieval := complexity > 50 && a.provider.Capabilities().FunctionCalling && len(a.tools) > 0
	if skipPreRetrieval {
		logx.Infof("[AgentRAG-Stream] Complex query (complexity=%d), skipping pre-retrieval — Agent will search autonomously via ReAct", complexity)
	}

	// 1. Agentic RAG 预检索（同步执行，复用 retrieveWithStrategy）
	var ragContext string
	var preSources []Source
	if !skipPreRetrieval && a.retriever != nil && spaceID > 0 {
		if limit <= 0 {
			limit = 5
		}
		sources, strategyUsed := a.retrieveWithStrategy(ctx, question, spaceID, limit)
		preSources = sources
		if len(sources) > 0 {
			ragContext = buildCitedContext(sources)
			logx.Infof("[AgentRAG-Stream] Pre-retrieved %d sources with strategy %s", len(sources), strategyUsed)
		}
	}

	// 2. 构建系统提示。
	// Prompt Caching 友好：system 只放**静态**内容（系统指令 + 复杂查询提示），
	// 每轮都变的 ragContext 不拼进 system —— 否则前缀每轮都失效，缓存永远不命中。
	// 检索结果改放第一条 user 消息（见下方），既保住前缀稳定性，又让模型看到它。
	systemPrompt := a.getSystemPrompt()
	if skipPreRetrieval {
		systemPrompt += "\n\n## 重要提示\n这是一个复杂查询，你可以使用 search_documents 工具进行多轮检索。请先分析需要哪些信息，然后分步检索，每次检索后评估是否需要进一步检索。"
	}

	// P1-5: 上下文预算 + 超预算压缩历史（compaction）。
	// agent 链路此前对前端传进来的全量历史零截断，长会话会把 prompt 撑爆或吃掉预算。
	// 这里在组装前用压缩层统一过一遍：未超预算原样返回，超了才 LLM 摘要旧消息。
	var docs []string
	if ragContext != "" {
		docs = append(docs, ragContext)
	}
	if a.compression != nil {
		cr := a.compression.Compress(ctx, question, docs, history)
		if cr.Compressed {
			logx.Infof("[AgentRAG-Stream] Context compaction triggered: %d tokens, strategy=%s", cr.TotalTokens, cr.Strategy)
		}
		history = cr.History
	}

	// 3. 构建消息历史
	messages := make([]provider.Message, 0, len(history)*2+1)
	for _, h := range history {
		messages = append(messages, provider.NewTextMessage("user", h.Question))
		messages = append(messages, provider.NewTextMessage("assistant", h.Answer))
	}
	// 检索结果作为独立的 user 上下文块，紧贴当前问题之前（动态内容放前缀之后）。
	if ragContext != "" {
		messages = append(messages, provider.NewTextMessage("user",
			"以下是从知识库中检索到的相关内容，请优先参考这些内容回答问题：\n\n"+ragContext))
	}
	if len(images) > 0 && images[0] != "" {
		messages = append(messages, provider.NewImageMessage("user", question, images[0]))
	} else {
		messages = append(messages, provider.NewTextMessage("user", question))
	}

	// 4. 选择路径
	if a.provider.Capabilities().FunctionCalling && len(a.tools) > 0 {
		return a.chatWithFunctionCallingStream(ctx, systemPrompt, messages, preSources)
	}
	return a.chatWithPromptInjectionStream(ctx, systemPrompt, messages, preSources)
}

// chatWithFunctionCallingStream 流式 Function Calling ReAct 循环
func (a *AgentChat) chatWithFunctionCallingStream(ctx context.Context, systemPrompt string, messages []provider.Message, preSources []Source) (<-chan AgentStreamEvent, error) {
	outCh := make(chan AgentStreamEvent, 64)
	tools := a.getToolDefinitions()

	go func() {
		defer close(outCh)
		defer recoverStreamProducer("function-calling")
		var allResults []ToolResult
		startTime := time.Now()

		for round := 0; round < a.maxRounds; round++ {
			if ctx.Err() != nil {
				return
			}
			// 发送 thinking 事件通知前端开始新一轮推理
			if !sendStreamEvent(ctx, outCh, AgentStreamEvent{Type: "thinking", Round: round, Content: fmt.Sprintf("正在思考（第 %d 轮）...", round+1)}) {
				return
			}

			// 调用 StreamChatWithTools
			ch, err := a.provider.StreamChatWithTools(ctx, systemPrompt, messages, tools,
				provider.WithMaxTokens(4096),
				provider.WithTemperature(0.3),
			)
			if err != nil {
				sendStreamEvent(ctx, outCh, AgentStreamEvent{Type: "error", Error: fmt.Sprintf("AI 调用失败: %v", err), Round: round})
				return
			}

			// 消费 channel：区分 content 和 tool_call
			var toolCalls []provider.ToolCall
			var roundContent strings.Builder
			for chunk := range ch {
				if ctx.Err() != nil {
					return
				}
				if chunk.Error != "" {
					sendStreamEvent(ctx, outCh, AgentStreamEvent{Type: "error", Error: chunk.Error, Round: round})
					return
				}
				if chunk.ToolCall != nil {
					toolCalls = append(toolCalls, *chunk.ToolCall)
					if !sendStreamEvent(ctx, outCh, AgentStreamEvent{Type: "tool_call", ToolCall: chunk.ToolCall, Round: round}) {
						return
					}
					continue
				}
				if chunk.Content != "" {
					roundContent.WriteString(chunk.Content)
					// 只在最终轮（无工具调用）转发 content，避免中间轮噪音
					// 但此处无法预知是否有 tool_call，先缓冲，循环结束后判断
				}
				if chunk.Done {
					break
				}
			}

			// 无工具调用 → 当前轮 content 即为最终答案
			if len(toolCalls) == 0 {
				if roundContent.Len() > 0 {
					if !sendStreamEvent(ctx, outCh, AgentStreamEvent{Type: "content", Content: roundContent.String(), Round: round}) {
						return
					}
				}
				allSources := a.extractSources(allResults)
				if len(allSources) == 0 {
					allSources = preSources
				}
				sendStreamEvent(ctx, outCh, AgentStreamEvent{
					Type:       "done",
					Sources:    allSources,
					DurationMs: time.Since(startTime).Milliseconds(),
				})
				return
			}

			// 有工具调用 → 执行工具，回填 messages，发送 tool_result 事件
			results := a.executeToolCallsParallel(ctx, toolCalls)
			allResults = append(allResults, results...)
			messages = append(messages, provider.NewToolCallMessage("", toolCalls))
			for i, tc := range toolCalls {
				resultJSON, _ := json.Marshal(results[i])
				messages = append(messages, provider.NewToolResultMessage(tc.ID, string(resultJSON)))
				if !sendStreamEvent(ctx, outCh, AgentStreamEvent{Type: "tool_result", ToolResult: &results[i], Round: round}) {
					return
				}
			}
			// 继续下一轮
		}

		// 超过最大轮次 → 汇总
		finalAnswer := a.summarizeToolResults(allResults)
		if finalAnswer != "" {
			sendStreamEvent(ctx, outCh, AgentStreamEvent{Type: "content", Content: finalAnswer, Round: a.maxRounds})
		}
		allSources := a.extractSources(allResults)
		if len(allSources) == 0 {
			allSources = preSources
		}
		sendStreamEvent(ctx, outCh, AgentStreamEvent{
			Type:       "done",
			Sources:    allSources,
			DurationMs: time.Since(startTime).Milliseconds(),
		})
	}()

	return outCh, nil
}

// chatWithPromptInjectionStream 流式 Prompt Injection 降级路径
// provider 不支持 Function Calling 时，用 StreamChat + 文本解析工具调用
func (a *AgentChat) chatWithPromptInjectionStream(ctx context.Context, systemPrompt string, messages []provider.Message, preSources []Source) (<-chan AgentStreamEvent, error) {
	outCh := make(chan AgentStreamEvent, 64)

	go func() {
		defer close(outCh)
		defer recoverStreamProducer("prompt-injection")
		var allResults []ToolResult
		startTime := time.Now()

		for round := 0; round < a.maxRounds; round++ {
			if ctx.Err() != nil {
				return
			}
			if !sendStreamEvent(ctx, outCh, AgentStreamEvent{Type: "thinking", Round: round, Content: fmt.Sprintf("正在思考（第 %d 轮）...", round+1)}) {
				return
			}

			ch, err := a.provider.StreamChat(ctx, systemPrompt, messages,
				provider.WithMaxTokens(4096),
				provider.WithTemperature(0.3),
			)
			if err != nil {
				sendStreamEvent(ctx, outCh, AgentStreamEvent{Type: "error", Error: fmt.Sprintf("AI 调用失败: %v", err), Round: round})
				return
			}

			var roundContent strings.Builder
			for chunk := range ch {
				if ctx.Err() != nil {
					return
				}
				if chunk.Error != "" {
					sendStreamEvent(ctx, outCh, AgentStreamEvent{Type: "error", Error: chunk.Error, Round: round})
					return
				}
				if chunk.Content != "" {
					roundContent.WriteString(chunk.Content)
				}
				if chunk.Done {
					break
				}
			}

			// 文本解析工具调用
			fullContent := roundContent.String()
			tc := a.extractToolCall(fullContent)
			if tc == nil {
				// 无工具调用 → 最终答案
				if !sendStreamEvent(ctx, outCh, AgentStreamEvent{Type: "content", Content: fullContent, Round: round}) {
					return
				}
				allSources := a.extractSources(allResults)
				if len(allSources) == 0 {
					allSources = preSources
				}
				sendStreamEvent(ctx, outCh, AgentStreamEvent{
					Type:       "done",
					Sources:    allSources,
					DurationMs: time.Since(startTime).Milliseconds(),
				})
				return
			}

			// 有工具调用
			tool, ok := a.tools[tc.Tool]
			if !ok {
				messages = append(messages,
					provider.NewTextMessage("assistant", fullContent),
					provider.NewTextMessage("user", fmt.Sprintf("工具 %s 不存在，请使用可用工具或直接回答。", tc.Tool)),
				)
				continue
			}

			providerTC := &provider.ToolCall{
				ID:        fmt.Sprintf("call_%d_%d", round, time.Now().UnixNano()),
				Name:      tc.Tool,
				Arguments: tc.Arguments,
			}
			if !sendStreamEvent(ctx, outCh, AgentStreamEvent{Type: "tool_call", ToolCall: providerTC, Round: round}) {
				return
			}

			// 参数需先按工具声明的 schema 校验：模型可以编造任意类型（如把 document_id 写成字符串
			// "12" 或对象），未校验的脏参数会绕过工具内部的 space_id / document_id 守卫。
			if verr := ValidateParams(tool.Definition(), tc.Params); verr != nil {
				failed := &ToolResult{ToolName: tc.Tool, Success: false, Error: "参数校验失败: " + verr.Error()}
				allResults = append(allResults, *failed)
				messages = append(messages,
					provider.NewTextMessage("assistant", fullContent),
					provider.NewTextMessage("user", fmt.Sprintf("工具参数校验失败: %v", verr)),
				)
				sendStreamEvent(ctx, outCh, AgentStreamEvent{Type: "tool_result", ToolResult: failed, Round: round})
				continue
			}

			result, execErr := tool.Execute(ctx, tc.Params)
			if execErr != nil {
				logx.Infof("[AgentChat-Stream] Tool %s error: %v", tc.Tool, execErr)
				messages = append(messages,
					provider.NewTextMessage("assistant", fullContent),
					provider.NewTextMessage("user", fmt.Sprintf("工具执行失败: %v", execErr)),
				)
				continue
			}
			allResults = append(allResults, *result)
			messages = append(messages,
				provider.NewTextMessage("assistant", fullContent),
				provider.NewTextMessage("user", fmt.Sprintf("工具 %s 执行结果: %s", result.ToolName, result.Result)),
			)
			if !sendStreamEvent(ctx, outCh, AgentStreamEvent{Type: "tool_result", ToolResult: result, Round: round}) {
				return
			}
		}

		finalAnswer := a.summarizeToolResults(allResults)
		if finalAnswer != "" {
			sendStreamEvent(ctx, outCh, AgentStreamEvent{Type: "content", Content: finalAnswer, Round: a.maxRounds})
		}
		allSources := a.extractSources(allResults)
		if len(allSources) == 0 {
			allSources = preSources
		}
		sendStreamEvent(ctx, outCh, AgentStreamEvent{
			Type:       "done",
			Sources:    allSources,
			DurationMs: time.Since(startTime).Milliseconds(),
		})
	}()

	return outCh, nil
}

// ChatStream 无空间上下文的流式 Agent 对话
func (a *AgentChat) ChatStream(ctx context.Context, question string, history []ChatMessage, images ...string) (<-chan AgentStreamEvent, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("AI服务未启用")
	}

	messages := make([]provider.Message, 0, len(history)*2+1)
	for _, h := range history {
		messages = append(messages, provider.NewTextMessage("user", h.Question))
		messages = append(messages, provider.NewTextMessage("assistant", h.Answer))
	}
	if len(images) > 0 && images[0] != "" {
		messages = append(messages, provider.NewImageMessage("user", question, images[0]))
	} else {
		messages = append(messages, provider.NewTextMessage("user", question))
	}

	if a.provider.Capabilities().FunctionCalling && len(a.tools) > 0 {
		return a.chatWithFunctionCallingStream(ctx, a.getSystemPrompt(), messages, nil)
	}
	return a.chatWithPromptInjectionStream(ctx, a.getSystemPrompt(), messages, nil)
}
