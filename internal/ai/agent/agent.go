// Package agent 提供轻量 Multi-agent 编排框架。
//
// 背景：复杂问题（研究、对比、多步分析）单 agent 难以兼顾检索、计算、总结等
// 不同能力。本框架把任务拆给多个具有不同角色与工具集的子 Agent，按编排模式协作：
//   - Sequential：依次执行，前序输出作为后序上下文（流水线）。
//   - Parallel：并发执行，最后聚合各 agent 输出（独立视角汇总）。
//   - Router：用 LLM 选择最合适的单个 agent 处理（单专家路由）。
//
// 工具集复用 internal/ai/rag 的 ToolExecutor，Agent 本身是 provider + 系统提示 + 工具的薄封装。
package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/ai/rag"
)

// Mode 编排模式。
type Mode string

const (
	ModeSequential Mode = "sequential"
	ModeParallel   Mode = "parallel"
	ModeRouter     Mode = "router"
)

// Agent 单个子 Agent：角色 + 系统提示 + 可选工具。
type Agent struct {
	Name         string
	Role         string
	SystemPrompt string
	Tools        []rag.ToolExecutor
}

// Result 单个 agent 的执行结果。
type Result struct {
	Agent  string `json:"agent"`
	Role   string `json:"role"`
	Output string `json:"output"`
	Error  string `json:"error,omitempty"`
}

// LLMClient 编排器所需的最小 LLM 接口（provider.Provider 自动满足）。
type LLMClient interface {
	Chat(ctx context.Context, system string, messages []provider.Message, opts ...provider.Option) (*provider.LLMResponse, error)
}

// Orchestrator 多 agent 编排器。
type Orchestrator struct {
	Provider LLMClient
	Mode     Mode
	Agents   []*Agent
	// RouterPrompt 仅 Mode=Router 时使用，指导 LLM 选择 agent。
	RouterPrompt string
}

// NewOrchestrator 创建编排器。mode 非法时退化为 Sequential。
func NewOrchestrator(p LLMClient, mode Mode, agents []*Agent) *Orchestrator {
	if p == nil {
		panic("agent: provider must not be nil")
	}
	if mode == "" {
		mode = ModeSequential
	}
	return &Orchestrator{Provider: p, Mode: mode, Agents: agents}
}

// Run 执行编排，返回各 agent 结果（顺序：Sequential 按 agent 顺序，Parallel 同序，Router 仅被选中的）。
func (o *Orchestrator) Run(ctx context.Context, task string, history []rag.ChatMessage) ([]Result, error) {
	if len(o.Agents) == 0 {
		return nil, fmt.Errorf("agent: no agents registered")
	}
	switch o.Mode {
	case ModeParallel:
		return o.runParallel(ctx, task, history)
	case ModeRouter:
		return o.runRouter(ctx, task, history)
	default:
		return o.runSequential(ctx, task, history)
	}
}

// runSequential 流水线：每个 agent 看到前面所有 agent 的输出。
func (o *Orchestrator) runSequential(ctx context.Context, task string, history []rag.ChatMessage) ([]Result, error) {
	results := make([]Result, 0, len(o.Agents))
	contextSoFar := task
	for _, a := range o.Agents {
		out, err := o.callAgent(ctx, a, contextSoFar, history)
		r := Result{Agent: a.Name, Role: a.Role, Output: out}
		if err != nil {
			r.Error = err.Error()
			results = append(results, r)
			return results, err // 任一失败即中断流水线
		}
		results = append(results, r)
		contextSoFar = fmt.Sprintf("%s\n\n[%s 的输出]:\n%s", contextSoFar, a.Name, out)
	}
	return results, nil
}

// runParallel 并发执行所有 agent，聚合输出。
func (o *Orchestrator) runParallel(ctx context.Context, task string, history []rag.ChatMessage) ([]Result, error) {
	results := make([]Result, len(o.Agents))
	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error
	for i, a := range o.Agents {
		wg.Add(1)
		go func(i int, a *Agent) {
			defer wg.Done()
			out, err := o.callAgent(ctx, a, task, history)
			mu.Lock()
			defer mu.Unlock()
			r := Result{Agent: a.Name, Role: a.Role, Output: out}
			if err != nil {
				r.Error = err.Error()
				if firstErr == nil {
					firstErr = err
				}
			}
			results[i] = r
		}(i, a)
	}
	wg.Wait()
	return results, firstErr
}

// runRouter 用 LLM 选择单个最合适的 agent 处理任务。
func (o *Orchestrator) runRouter(ctx context.Context, task string, history []rag.ChatMessage) ([]Result, error) {
	idx, err := o.selectAgent(ctx, task)
	if err != nil {
		return nil, err
	}
	a := o.Agents[idx]
	out, err := o.callAgent(ctx, a, task, history)
	r := Result{Agent: a.Name, Role: a.Role, Output: out}
	if err != nil {
		r.Error = err.Error()
		return []Result{r}, err
	}
	return []Result{r}, nil
}

// selectAgent 让 LLM 从 agent 列表中选择最合适的一个，返回其索引。
func (o *Orchestrator) selectAgent(ctx context.Context, task string) (int, error) {
	var b strings.Builder
	b.WriteString("你是一个任务路由器。请从以下 Agent 中选择最合适处理用户任务的一个，只输出其编号（1-based）：\n\n")
	for i, a := range o.Agents {
		fmt.Fprintf(&b, "%d. %s —— %s\n", i+1, a.Name, a.Role)
	}
	b.WriteString("\n用户任务：")
	b.WriteString(task)
	b.WriteString("\n\n只回复一个数字。")

	resp, err := o.Provider.Chat(ctx, o.RouterPrompt, []provider.Message{provider.NewTextMessage("user", b.String())}, provider.WithMaxTokens(8), provider.WithTemperature(0))
	if err != nil {
		return 0, fmt.Errorf("router llm failed: %w", err)
	}
	var n int
	if _, err := fmt.Sscanf(strings.TrimSpace(resp.Content), "%d", &n); err != nil || n < 1 || n > len(o.Agents) {
		return 0, fmt.Errorf("router returned invalid selection: %q", resp.Content)
	}
	return n - 1, nil
}

// callAgent 调用单个 agent 完成一轮 LLM 生成（含可选工具循环）。
func (o *Orchestrator) callAgent(ctx context.Context, a *Agent, task string, history []rag.ChatMessage) (string, error) {
	messages := make([]provider.Message, 0, len(history)+1)
	for _, h := range history {
		messages = append(messages, provider.NewTextMessage("user", h.Question))
		messages = append(messages, provider.NewTextMessage("assistant", h.Answer))
	}
	messages = append(messages, provider.NewTextMessage("user", task))

	// 若有工具，走工具调用循环（复用 agent_chat 的 ToolExecutor 机制）。
	if len(a.Tools) > 0 {
		return o.callWithTools(ctx, a, messages)
	}
	resp, err := o.Provider.Chat(ctx, a.SystemPrompt, messages)
	if err != nil {
		return "", err
	}
	return resp.Content, nil
}

// callWithTools 带工具调用的 agent 循环（最多 maxToolRounds 轮）。
func (o *Orchestrator) callWithTools(ctx context.Context, a *Agent, messages []provider.Message) (string, error) {
	const maxToolRounds = 4
	funcs := toToolDefs(a.Tools)
	cur := messages
	for round := 0; round < maxToolRounds; round++ {
		resp, err := o.Provider.Chat(ctx, a.SystemPrompt, cur, provider.WithTools(funcs))
		if err != nil {
			return "", err
		}
		if len(resp.ToolCalls) == 0 {
			return resp.Content, nil
		}
		for _, call := range resp.ToolCalls {
			var args map[string]interface{}
			_ = json.Unmarshal(call.Arguments, &args)
			res := executeTool(a.Tools, call.Name, args)
			cur = append(cur, provider.NewToolCallMessage("", []provider.ToolCall{call}))
			cur = append(cur, provider.NewToolResultMessage(call.ID, res.Result))
		}
	}
	// 工具轮次耗尽，做最终总结。
	final, err := o.Provider.Chat(ctx, a.SystemPrompt, cur)
	if err != nil {
		return "", err
	}
	return final.Content, nil
}

// toToolDefs 把 rag.ToolExecutor 列表转为 provider 工具定义。
func toToolDefs(tools []rag.ToolExecutor) []provider.ToolDefinition {
	defs := make([]provider.ToolDefinition, 0, len(tools))
	for _, t := range tools {
		d := t.Definition()
		defs = append(defs, provider.ToolDefinition{
			Name:        d.Name,
			Description: d.Description,
			Parameters:  d.Parameters,
		})
	}
	return defs
}

func toolsMap(tools []rag.ToolExecutor) map[string]rag.ToolExecutor {
	m := make(map[string]rag.ToolExecutor, len(tools))
	for _, t := range tools {
		m[t.Definition().Name] = t
	}
	return m
}

func executeTool(tools []rag.ToolExecutor, name string, args map[string]interface{}) *rag.ToolResult {
	for _, t := range tools {
		if t.Definition().Name == name {
			res, err := t.Execute(context.Background(), args)
			if err != nil {
				return &rag.ToolResult{ToolName: name, Success: false, Error: err.Error()}
			}
			return res
		}
	}
	return &rag.ToolResult{ToolName: name, Success: false, Error: "tool not found: " + name}
}

// Aggregate 把多个 agent 结果聚合为一段可读总结文本（用于 Parallel 模式最终呈现）。
func Aggregate(results []Result) string {
	var b strings.Builder
	for _, r := range results {
		b.WriteString(fmt.Sprintf("### %s（%s）\n", r.Agent, r.Role))
		if r.Error != "" {
			b.WriteString("（执行失败：")
			b.WriteString(r.Error)
			b.WriteString("）\n")
			continue
		}
		b.WriteString(r.Output)
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}
