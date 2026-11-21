// Package multiagent 实现多智能体团队编排与「规划-执行-批判-修订」闭环
// （P0-2 多智能体 + P0-3 规划反思闭环）。
//
// 架构：
//   - Orchestrator：总指挥，负责 Plan（任务分解）→ 调度 Worker → Critique（批判门控）→ Revise（修订）。
//   - Supervisor：在 Orchestrator 内部做子任务的角色分配与结果汇总。
//   - Worker：专业化角色（researcher / executor / writer），每个 Worker 是一个独立的 AgentChat（含工具集）。
//   - Critic：对草稿进行批判审查，输出是否通过及修订建议，构成验证门（verification gate）。
package multiagent

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"
	"sync"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/ai/rag"
	"github.com/listen808/citebase/internal/ai/router"
)

// Role 定义 Worker 的专业化角色。
type Role string

const (
	RoleResearcher Role = "researcher" // 检索/调研：使用 search_documents 等检索类工具
	RoleExecutor   Role = "executor"   // 执行/操作：创建文档、执行代码、系统命令等
	RoleWriter     Role = "writer"     // 撰写/综合：负责最终答案与文档起草
	RoleCritic     Role = "critic"     // 批判审查（特殊角色，由 Critic 组件承担）
)

// Worker 一个专业化智能体，包装一个 AgentChat 实例。
type Worker struct {
	Role        Role
	Agent       *rag.AgentChat
	Description string
}

// OrchestratorConfig 编排器配置。
type OrchestratorConfig struct {
	MaxPlanSteps    int  // 最大子任务数
	MaxReviseRounds int  // 最大批判-修订轮数
	EnableCritique  bool // 是否启用批判门控
}

// DefaultConfig 返回默认配置。
func DefaultConfig() OrchestratorConfig {
	return OrchestratorConfig{
		MaxPlanSteps:    6,
		MaxReviseRounds: 2,
		EnableCritique:  true,
	}
}

// Orchestrator 多智能体编排器。
type Orchestrator struct {
	router    *router.Router
	provider  provider.Provider // 用于 Supervisor/Critic 推理的模型（通常 reasoning/primary）
	retriever *rag.Retriever
	spaceSvc  rag.SpacePrivilegeChecker
	tools     map[string]rag.ToolExecutor
	cfg       OrchestratorConfig

	mu       sync.Mutex
	lastPlan []SubTask
	history  []RoundLog
}

// SubTask 计划中的一个子任务。
type SubTask struct {
	ID        int    `json:"id"`
	Role      Role   `json:"role"`
	Objective string `json:"objective"`
	DependsOn []int  `json:"depends_on"`
	Done      bool   `json:"done"`
	Result    string `json:"result"`
}

// RoundLog 记录一轮「执行-批判-修订」过程，供前端可视化推理链。
type RoundLog struct {
	Round    int       `json:"round"`
	Plan     []SubTask `json:"plan"`
	Draft    string    `json:"draft"`
	Critique string    `json:"critique"`
	Passed   bool      `json:"passed"`
	Revised  string    `json:"revised"`
}

// NewOrchestrator 创建编排器。
func NewOrchestrator(r *router.Router, p provider.Provider, retriever *rag.Retriever, spaceSvc rag.SpacePrivilegeChecker, cfg OrchestratorConfig) *Orchestrator {
	if cfg.MaxPlanSteps <= 0 {
		cfg = DefaultConfig()
	}
	o := &Orchestrator{
		router:    r,
		provider:  p,
		retriever: retriever,
		spaceSvc:  spaceSvc,
		tools:     make(map[string]rag.ToolExecutor),
		cfg:       cfg,
	}
	o.registerDefaultTools()
	return o
}

// registerDefaultTools 注册可被 Worker 使用的工具集。
func (o *Orchestrator) registerDefaultTools() {
	registry := rag.GetToolRegistry()
	for name, tool := range registry.GetAll() {
		o.tools[name] = tool
	}
	if o.retriever != nil {
		o.tools["search_documents"] = rag.NewSearchDocumentsTool(o.retriever)
	}
}

// SetTools 覆盖可用工具集（用于按空间权限裁剪）。
func (o *Orchestrator) SetTools(tools map[string]rag.ToolExecutor) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.tools = tools
}

// newWorker 创建一个指定角色的 Worker。
func (o *Orchestrator) newWorker(role Role) *Worker {
	ac := rag.NewAgentChatWithTools(o.provider, o.retriever, o.roleTools(role), o.spaceSvc, nil, nil)
	return &Worker{
		Role:        role,
		Agent:       ac,
		Description: roleDescription(role),
	}
}

// roleTools 返回某角色应启用的工具子集。
func (o *Orchestrator) roleTools(role Role) []string {
	all := make([]string, 0, len(o.tools))
	for name := range o.tools {
		all = append(all, name)
	}
	switch role {
	case RoleResearcher:
		filtered := make([]string, 0)
		for _, n := range all {
			if strings.Contains(n, "search") || strings.Contains(n, "fetch") || strings.Contains(n, "web") || n == "get_document" || n == "list_documents" || n == "list_spaces" {
				filtered = append(filtered, n)
			}
		}
		if len(filtered) > 0 {
			return filtered
		}
		return all
	case RoleExecutor:
		filtered := make([]string, 0)
		for _, n := range all {
			if n == "create_document" || n == "move_document" || n == "edit_document" || n == "execute_code" || n == "execute_command" || n == "calculate" || n == "generate_chart" {
				filtered = append(filtered, n)
			}
		}
		if len(filtered) > 0 {
			return filtered
		}
		return all
	default: // writer / 其它：使用全部工具
		return all
	}
}

// Run 执行完整的「规划-执行-批判-修订」闭环，返回最终答案。
func (o *Orchestrator) Run(ctx context.Context, question string, history []rag.ChatMessage, images ...string) (string, error) {
	if o.provider == nil {
		return "", fmt.Errorf("AI服务未启用")
	}

	o.mu.Lock()
	o.history = nil
	o.mu.Unlock()

	// 1) Plan：分解任务。
	plan, err := o.plan(ctx, question, history)
	if err != nil {
		return "", fmt.Errorf("plan failed: %w", err)
	}
	o.mu.Lock()
	o.lastPlan = plan
	o.mu.Unlock()

	// 2) 执行：按依赖顺序调度 Worker。
	execResults, err := o.executePlan(ctx, plan, question, history, images...)
	if err != nil {
		logx.Infof("[multiagent] execute plan partial error: %v", err)
	}

	// 3) 汇总草稿。
	draft := o.synthesizeDraft(question, execResults)

	// 4) 批判-修订闭环。
	finalAnswer := draft
	if o.cfg.EnableCritique {
		for round := 0; round < o.cfg.MaxReviseRounds; round++ {
			critique, passed := o.critique(ctx, question, draft, execResults)
			revised := draft
			if !passed {
				revised = o.revise(ctx, question, draft, critique)
			}
			o.mu.Lock()
			o.history = append(o.history, RoundLog{
				Round:    round + 1,
				Plan:     plan,
				Draft:    draft,
				Critique: critique,
				Passed:   passed,
				Revised:  revised,
			})
			o.mu.Unlock()
			finalAnswer = revised
			if passed {
				break
			}
			draft = revised
		}
	}

	return finalAnswer, nil
}

// plan 调用模型生成子任务计划（JSON）。
func (o *Orchestrator) plan(ctx context.Context, question string, history []rag.ChatMessage) ([]SubTask, error) {
	prompt := fmt.Sprintf(`你是一个任务规划器。请将用户问题分解为可由专业化智能体执行的子任务。
可用角色：researcher（检索调研）、executor（执行操作/代码）、writer（撰写综合）。

用户问题：%s

请以 JSON 数组返回，每个元素包含：
{"id": 序号, "role": "researcher|executor|writer", "objective": "子任务目标", "depends_on": [依赖的子任务id]}

要求：
- 子任务数量不超过 %d。
- writer 角色应放在最后，负责综合前面结果形成最终答案。
- 仅返回 JSON 数组，不要额外解释。`, question, o.cfg.MaxPlanSteps)

	sysMsg := "你是严谨的任务规划器，只输出结构化计划。"
	resp, err := o.provider.Chat(ctx, sysMsg, []provider.Message{provider.NewTextMessage("user", prompt)},
		provider.WithMaxTokens(1024), provider.WithTemperature(0.2))
	if err != nil {
		return nil, err
	}

	return parsePlan(resp.Content)
}

// parsePlan 从模型输出解析子任务列表（容错提取 JSON 数组）。
func parsePlan(text string) ([]SubTask, error) {
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start == -1 || end <= start {
		// 退化：整体作为一个 researcher 子任务
		return []SubTask{{ID: 1, Role: RoleResearcher, Objective: strings.TrimSpace(text), DependsOn: nil, Done: false}}, nil
	}
	raw := text[start : end+1]
	var tasks []SubTask
	if err := json.Unmarshal([]byte(raw), &tasks); err != nil {
		return nil, fmt.Errorf("parse plan json: %w", err)
	}
	return tasks, nil
}

// executePlan 按依赖拓扑顺序执行子任务，返回 id->结果 映射。
func (o *Orchestrator) executePlan(ctx context.Context, plan []SubTask, question string, history []rag.ChatMessage, images ...string) (map[int]string, error) {
	results := make(map[int]string)
	var mu sync.Mutex
	var wg sync.WaitGroup

	// 按 depends_on 做简单层级调度：反复扫描未完成且依赖已满足的任务。
	remaining := make(map[int]SubTask)
	for _, t := range plan {
		remaining[t.ID] = t
	}

	for len(remaining) > 0 {
		var ready []SubTask
		for _, t := range remaining {
			depsReady := true
			for _, dep := range t.DependsOn {
				if _, ok := results[dep]; !ok {
					depsReady = false
					break
				}
			}
			if depsReady {
				ready = append(ready, t)
			}
		}
		if len(ready) == 0 {
			// 存在无法解析的依赖环，直接执行剩余任务
			for _, t := range remaining {
				ready = append(ready, t)
			}
		}

		for _, task := range ready {
			delete(remaining, task.ID)
			wg.Add(1)
			go func(t SubTask) {
				defer wg.Done()
				worker := o.newWorker(t.Role)
				context := fmt.Sprintf("原始问题：%s\n你的子任务：%s", question, t.Objective)
				if len(t.DependsOn) > 0 {
					var depText strings.Builder
					for _, dep := range t.DependsOn {
						mu.Lock()
						r := results[dep]
						mu.Unlock()
						depText.WriteString(fmt.Sprintf("\n[前置任务%d结果]\n%s\n", dep, r))
					}
					context += depText.String()
				}
				ans, err := worker.Agent.Chat(ctx, context, history, images...)
				out := ""
				if err != nil {
					out = fmt.Sprintf("[子任务执行失败: %v]", err)
				} else if ans != nil {
					out = ans.Answer
				}
				mu.Lock()
				results[t.ID] = out
				mu.Unlock()
			}(task)
		}
		wg.Wait()
	}
	return results, nil
}

// synthesizeDraft 汇总各子任务结果，形成草稿答案。
func (o *Orchestrator) synthesizeDraft(question string, results map[int]string) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("问题：%s\n\n子任务执行结果汇总：\n", question))
	// 按 id 排序输出（简单稳定顺序）
	ids := sortedKeys(results)
	for _, id := range ids {
		sb.WriteString(fmt.Sprintf("\n[子任务 %d]\n%s\n", id, results[id]))
	}
	return sb.String()
}

// critique 批判审查草稿，返回审查意见与是否通过。
func (o *Orchestrator) critique(ctx context.Context, question, draft string, results map[int]string) (string, bool) {
	prompt := fmt.Sprintf(`请作为严格的质量审查员，评审下面针对用户问题的草稿答案。

用户问题：%s

草稿：
%s

请评估：
1. 是否准确回答了用户问题？
2. 是否遗漏关键信息或存在事实错误？
3. 是否需要补检索或修正？

以如下 JSON 返回：
{"passed": true/false, "comment": "审查意见与修订建议"}

仅返回 JSON。`, question, draft)

	resp, err := o.provider.Chat(ctx, "你是严谨的审查员，只输出 JSON 评审结论。",
		[]provider.Message{provider.NewTextMessage("user", prompt)},
		provider.WithMaxTokens(1024), provider.WithTemperature(0.2), provider.WithJSONMode())
	if err != nil {
		logx.Infof("[multiagent] critique failed: %v", err)
		return "审查器不可用，默认通过", true
	}

	var cr struct {
		Passed  bool   `json:"passed"`
		Comment string `json:"comment"`
	}
	if err := json.Unmarshal([]byte(extractJSON(resp.Content)), &cr); err != nil {
		return resp.Content, false
	}
	return cr.Comment, cr.Passed
}

// revise 根据批判意见修订草稿。
func (o *Orchestrator) revise(ctx context.Context, question, draft, critique string) string {
	prompt := fmt.Sprintf(`根据审查意见修订下方草稿，使其更准确、完整地回答用户问题。

用户问题：%s

原草稿：
%s

审查意见：
%s

请直接输出修订后的最终答案（中文，清晰简洁）。`, question, draft, critique)

	resp, err := o.provider.Chat(ctx, "你是专业的答案修订者。",
		[]provider.Message{provider.NewTextMessage("user", prompt)},
		provider.WithMaxTokens(2048), provider.WithTemperature(0.3))
	if err != nil {
		logx.Infof("[multiagent] revise failed: %v", err)
		return draft
	}
	return resp.Content
}

// Plan 返回最近一次计划（供前端可视化）。
func (o *Orchestrator) Plan() []SubTask {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.lastPlan
}

// History 返回批判-修订闭环历史（供前端推理链可视化）。
func (o *Orchestrator) History() []RoundLog {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.history
}

// ---- 工具函数 ----

func roleDescription(role Role) string {
	switch role {
	case RoleResearcher:
		return "检索调研专家，负责从知识库与外部来源收集信息"
	case RoleExecutor:
		return "执行操作专家，负责创建文档、运行代码、执行命令"
	case RoleWriter:
		return "撰写综合专家，负责整合信息形成最终答案"
	default:
		return "通用智能体"
	}
}

func extractJSON(text string) string {
	start := strings.Index(text, "{")
	end := strings.LastIndex(text, "}")
	if start == -1 || end <= start {
		return text
	}
	return text[start : end+1]
}

func sortedKeys(m map[int]string) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 简单插入排序（不引入 sort 依赖以保持轻量）
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j-1] > keys[j]; j-- {
			keys[j-1], keys[j] = keys[j], keys[j-1]
		}
	}
	return keys
}
