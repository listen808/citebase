package rag

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
)

type WorkflowNodeType string

const (
	NodeTypeLLM       WorkflowNodeType = "llm"
	NodeTypeTool      WorkflowNodeType = "tool"
	NodeTypeCondition WorkflowNodeType = "condition"
	NodeTypeParallel  WorkflowNodeType = "parallel"
	NodeTypeStart     WorkflowNodeType = "start"
	NodeTypeEnd       WorkflowNodeType = "end"
	// NodeTypeRetrieval 真实检索节点。
	// 存在理由：此前 deep_research 的"并行多源检索"是三个不同 system prompt 的纯 LLM
	// 生成，不查任何数据源 —— 产出的研究报告没有任何真实来源支撑。
	// 这里刻意新增节点类型而不是放开 workflowSafeTools 白名单：工具节点的参数可由
	// 上一步模型输出插值得到，而工作流执行链没有用户身份维度，放行读文档类工具会让
	// 提示注入直接变成越权读文档。检索节点的检索函数由调用方按自身权限范围注入。
	NodeTypeRetrieval WorkflowNodeType = "retrieval"
)

type WorkflowNode struct {
	ID          string                 `json:"id"`
	Type        WorkflowNodeType       `json:"type"`
	Name        string                 `json:"name"`
	Config      map[string]interface{} `json:"config"`
	NextNodes   []string               `json:"next_nodes"`
	ConditionFn string                 `json:"condition_fn,omitempty"`
}

type WorkflowEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type WorkflowDefinition struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Nodes       []WorkflowNode `json:"nodes"`
	Edges       []WorkflowEdge `json:"edges"`
	Active      bool           `json:"active"`
	CreatedAt   time.Time      `json:"created_at"`
}

type WorkflowExecution struct {
	ID          string                 `json:"id"`
	WorkflowID  string                 `json:"workflow_id"`
	Status      string                 `json:"status"`
	CurrentNode string                 `json:"current_node"`
	Results     map[string]interface{} `json:"results"`
	Error       string                 `json:"error,omitempty"`
	StartedAt   time.Time              `json:"started_at"`
	CompletedAt time.Time              `json:"completed_at,omitempty"`
}

type WorkflowEngine struct {
	mu        sync.RWMutex
	provider  provider.Provider
	workflows map[string]*WorkflowDefinition
	running   map[string]*WorkflowExecution
}

// 工作流执行安全上限：Nodes/NextNodes 由管理员请求体原样写入（UpdateWorkflow 无图校验），
// 成环定义会让主循环与 parallel 递归无界执行，每轮都调用 LLM（费用耗尽）并派生 goroutine；
// 嵌套 parallel 的多个子 goroutine 此前还会并发写同一 results map，触发
// fatal error: concurrent map writes（无法 recover，整进程退出）。
const (
	maxWorkflowNodeExecutions = 100
	maxParallelDepth          = 8
)

// RetrievedSnippet 检索节点返回的一个片段。
type RetrievedSnippet struct {
	DocumentID int     `json:"document_id"`
	Title      string  `json:"title"`
	Content    string  `json:"content"`
	Score      float64 `json:"score"`
}

// RetrievalFunc 按调用方权限范围执行一次真实检索。
type RetrievalFunc func(ctx context.Context, query string, topK int) ([]RetrievedSnippet, error)

// execScope 为单次执行提供互斥保护的 results 访问与节点执行计数。
type execScope struct {
	mu         sync.Mutex
	results    map[string]interface{}
	executions int
	retriever  RetrievalFunc
}

// reserve 预占一次节点执行名额，超限返回 false。
func (s *execScope) reserve() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.executions >= maxWorkflowNodeExecutions {
		return false
	}
	s.executions++
	return true
}

// snapshot 返回 results 的一致性拷贝，供插值只读使用（避免与并发写迭代冲突）。
func (s *execScope) snapshot() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	cp := make(map[string]interface{}, len(s.results))
	for k, v := range s.results {
		cp[k] = v
	}
	return cp
}

// set 在锁保护下写回单个结果。
func (s *execScope) set(k string, v interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.results[k] = v
}

var workflowEngine *WorkflowEngine
var workflowEngineOnce sync.Once

func InitWorkflowEngine(p provider.Provider) {
	workflowEngineOnce.Do(func() {
		workflowEngine = &WorkflowEngine{
			provider:  p,
			workflows: make(map[string]*WorkflowDefinition),
			running:   make(map[string]*WorkflowExecution),
		}
	})
}

func GetWorkflowEngine() *WorkflowEngine {
	if workflowEngine == nil {
		workflowEngineOnce.Do(func() {
			workflowEngine = &WorkflowEngine{
				workflows: make(map[string]*WorkflowDefinition),
				running:   make(map[string]*WorkflowExecution),
			}
		})
	}
	return workflowEngine
}

func (we *WorkflowEngine) RegisterWorkflow(wf *WorkflowDefinition) {
	we.mu.Lock()
	defer we.mu.Unlock()
	wf.CreatedAt = time.Now()
	we.workflows[wf.ID] = wf
}

func (we *WorkflowEngine) GetWorkflow(id string) *WorkflowDefinition {
	we.mu.RLock()
	defer we.mu.RUnlock()
	return we.workflows[id]
}

// SetWorkflowActive 更新工作流的启用状态
func (we *WorkflowEngine) SetWorkflowActive(id string, active bool) error {
	we.mu.Lock()
	defer we.mu.Unlock()
	wf, ok := we.workflows[id]
	if !ok {
		return fmt.Errorf("workflow not found: %s", id)
	}
	wf.Active = active
	return nil
}

// UpdateWorkflow 更新工作流定义（nodes/edges 及其它字段）
func (we *WorkflowEngine) UpdateWorkflow(id string, updated *WorkflowDefinition) (*WorkflowDefinition, error) {
	we.mu.Lock()
	defer we.mu.Unlock()
	wf, ok := we.workflows[id]
	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", id)
	}
	if updated.Name != "" {
		wf.Name = updated.Name
	}
	if updated.Description != "" {
		wf.Description = updated.Description
	}
	if updated.Nodes != nil {
		wf.Nodes = updated.Nodes
	}
	if updated.Edges != nil {
		wf.Edges = updated.Edges
	}
	return wf, nil
}

func (we *WorkflowEngine) ListWorkflows() []*WorkflowDefinition {
	we.mu.RLock()
	defer we.mu.RUnlock()
	result := make([]*WorkflowDefinition, 0, len(we.workflows))
	for _, wf := range we.workflows {
		result = append(result, wf)
	}
	return result
}

// DeleteWorkflow 删除工作流
func (we *WorkflowEngine) DeleteWorkflow(id string) error {
	we.mu.Lock()
	defer we.mu.Unlock()
	if _, ok := we.workflows[id]; !ok {
		return fmt.Errorf("workflow not found: %s", id)
	}
	delete(we.workflows, id)
	return nil
}

// StopExecution 停止正在运行的工作流执行
func (we *WorkflowEngine) StopExecution(execID string) error {
	we.mu.Lock()
	defer we.mu.Unlock()
	exec, ok := we.running[execID]
	if !ok {
		return fmt.Errorf("execution not found: %s", execID)
	}
	exec.Status = "stopped"
	exec.CompletedAt = time.Now()
	return nil
}

// GetExecution 获取执行状态
func (we *WorkflowEngine) GetExecution(execID string) *WorkflowExecution {
	we.mu.RLock()
	defer we.mu.RUnlock()
	return we.running[execID]
}

// ListRunningExecutions 列出所有正在运行的执行
func (we *WorkflowEngine) ListRunningExecutions() []*WorkflowExecution {
	we.mu.RLock()
	defer we.mu.RUnlock()
	result := make([]*WorkflowExecution, 0, len(we.running))
	for _, exec := range we.running {
		result = append(result, exec)
	}
	return result
}

// Execute 执行工作流（不带检索能力，检索节点会显式声明"无来源"）。
func (we *WorkflowEngine) Execute(ctx context.Context, workflowID string, inputs map[string]interface{}) (*WorkflowExecution, error) {
	return we.ExecuteWithRetriever(ctx, workflowID, inputs, nil)
}

// ExecuteWithRetriever 执行工作流，并注入按调用方权限范围限定的检索函数。
// 检索函数是 per-execution 的，不放在引擎单例上：不同用户的执行并发进行时，
// 单例字段会互相覆盖，造成跨用户的数据可见性串味。
func (we *WorkflowEngine) ExecuteWithRetriever(ctx context.Context, workflowID string, inputs map[string]interface{}, retrieve RetrievalFunc) (*WorkflowExecution, error) {
	we.mu.RLock()
	wf, ok := we.workflows[workflowID]
	we.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("workflow not found: %s", workflowID)
	}

	exec := &WorkflowExecution{
		ID:         fmt.Sprintf("exec-%d", time.Now().UnixNano()),
		WorkflowID: workflowID,
		Status:     "running",
		Results:    make(map[string]interface{}),
		StartedAt:  time.Now(),
	}
	for k, v := range inputs {
		exec.Results[k] = v
	}

	we.mu.Lock()
	we.running[exec.ID] = exec
	we.mu.Unlock()

	defer func() {
		we.mu.Lock()
		delete(we.running, exec.ID)
		we.mu.Unlock()
	}()

	nodeMap := make(map[string]*WorkflowNode)
	var startNode *WorkflowNode
	for i := range wf.Nodes {
		nodeMap[wf.Nodes[i].ID] = &wf.Nodes[i]
		if wf.Nodes[i].Type == NodeTypeStart {
			startNode = &wf.Nodes[i]
		}
	}

	if startNode == nil {
		exec.Status = "error"
		exec.Error = "no start node found"
		return exec, nil
	}

	currentNode := startNode
	scope := &execScope{results: exec.Results, retriever: retrieve}
	for currentNode != nil {
		exec.CurrentNode = currentNode.ID

		if ctx.Err() != nil {
			exec.Status = "cancelled"
			return exec, nil
		}

		result, err := we.executeNode(ctx, currentNode, scope, 0)
		if err != nil {
			exec.Status = "error"
			exec.Error = fmt.Sprintf("node %s error: %v", currentNode.ID, err)
			return exec, nil
		}

		scope.set(currentNode.ID, result)

		nextNodeID := we.resolveNextNode(currentNode, scope.snapshot())
		if nextNodeID == "" {
			break
		}
		currentNode = nodeMap[nextNodeID]
	}

	exec.Status = "completed"
	exec.CompletedAt = time.Now()
	return exec, nil
}

func (we *WorkflowEngine) executeNode(ctx context.Context, node *WorkflowNode, scope *execScope, depth int) (interface{}, error) {
	// 步数/深度上限：成环或自指的节点图在旧实现里可无限执行，耗尽 LLM 费用与 goroutine
	if depth > maxParallelDepth {
		return nil, fmt.Errorf("parallel 嵌套深度超过上限 %d，疑似环状工作流定义", maxParallelDepth)
	}
	if !scope.reserve() {
		return nil, fmt.Errorf("工作流节点执行数超过上限 %d，疑似环状定义，终止执行", maxWorkflowNodeExecutions)
	}
	// 快照读取上游结果，避免与并行子任务的写回并发访问同一 map
	vars := scope.snapshot()

	switch node.Type {
	case NodeTypeStart, NodeTypeEnd:
		return nil, nil

	case NodeTypeLLM:
		return we.executeLLMNode(ctx, node, vars)

	case NodeTypeTool:
		return we.executeToolNode(ctx, node, vars)

	case NodeTypeCondition:
		return we.executeConditionNode(node, vars)

	case NodeTypeParallel:
		return we.executeParallelNode(ctx, node, scope, depth)

	case NodeTypeRetrieval:
		return we.executeRetrievalNode(ctx, node, scope)

	default:
		return nil, fmt.Errorf("unknown node type: %s", node.Type)
	}
}

func (we *WorkflowEngine) executeLLMNode(ctx context.Context, node *WorkflowNode, results map[string]interface{}) (interface{}, error) {
	if we.provider == nil {
		return nil, fmt.Errorf("AI provider not available")
	}

	promptTmpl, _ := node.Config["prompt"].(string)
	system, _ := node.Config["system"].(string)

	prompt := we.interpolateTemplate(promptTmpl, results)

	resp, err := we.provider.Chat(ctx, system, []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(1024), provider.WithTemperature(0.3))
	if err != nil {
		return nil, err
	}

	return resp.Content, nil
}

// workflowSafeTools 工作流工具节点白名单：工具参数可由上一步模型输出插值得到
// （检索文档/用户输入触发提示注入即可操控参数），且 workflow 执行链没有用户身份维度，
// 因此只允许无外部副作用的纯计算类工具；出网、发邮件、命令/代码执行、读写文档与
// 读取用户/空间数据的工具一律拒绝（与 Agent 链路的高危工具开关分离，fail-closed）。
var workflowSafeTools = map[string]bool{
	"calculate":         true,
	"get_current_time":  true,
	"string_manipulate": true,
	"json_formatter":    true,
	"random_number":     true,
}

func (we *WorkflowEngine) executeToolNode(ctx context.Context, node *WorkflowNode, results map[string]interface{}) (interface{}, error) {
	toolName, _ := node.Config["tool"].(string)
	paramsConfig, _ := node.Config["params"].(map[string]interface{})

	if !workflowSafeTools[toolName] {
		return map[string]interface{}{"tool": toolName, "status": "denied", "error": "工具不在工作流安全白名单内，已拒绝执行"}, nil
	}

	params := make(map[string]interface{})
	for k, v := range paramsConfig {
		if str, ok := v.(string); ok {
			params[k] = we.interpolateTemplate(str, results)
		} else {
			params[k] = v
		}
	}

	registry := GetToolRegistry()
	if registry == nil {
		return map[string]interface{}{"tool": toolName, "status": "skipped", "error": "tool registry not initialized"}, nil
	}

	tool, ok := registry.Get(toolName)
	if !ok {
		return map[string]interface{}{"tool": toolName, "status": "skipped", "error": "tool not found"}, nil
	}

	// 插值后的参数含模型文本，按工具声明的 JSON Schema 做类型/必填校验后再执行
	if err := ValidateParams(tool.Definition(), params); err != nil {
		return map[string]interface{}{"tool": toolName, "status": "error", "error": "参数校验失败: " + err.Error()}, nil
	}

	result, err := tool.Execute(ctx, params)
	if err != nil {
		return map[string]interface{}{"tool": toolName, "status": "error", "error": err.Error()}, nil
	}

	return map[string]interface{}{
		"tool":   toolName,
		"params": params,
		"result": result,
		"status": "executed",
	}, nil
}

// executeRetrievalNode 执行一次真实检索。
//
// 关键：检索不可用时返回显式的"无来源"标记，而不是让下游 LLM 节点凭空生成。
// 这正是改造前 deep_research 的问题 —— 三个"检索源"节点全是纯 LLM 生成，
// 产出的报告看起来有模有样，实际没有任何来源支撑。
func (we *WorkflowEngine) executeRetrievalNode(ctx context.Context, node *WorkflowNode, scope *execScope) (interface{}, error) {
	vars := scope.snapshot()

	queryTmpl, _ := node.Config["query"].(string)
	query := we.interpolateTemplate(queryTmpl, vars)
	if query == "" {
		query, _ = vars["question"].(string)
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return "[无检索结果] 检索节点未获得有效查询语句", nil
	}

	topK := 5
	if v, ok := node.Config["top_k"].(float64); ok && v > 0 {
		topK = int(v)
	}

	retrieve := scope.retriever
	if retrieve == nil {
		return "[无检索结果] 本次执行未注入检索源，无法获取真实资料，下游不得编造来源", nil
	}

	snippets, err := retrieve(ctx, query, topK)
	if err != nil {
		return fmt.Sprintf("[检索失败] %v", err), nil
	}
	if len(snippets) == 0 {
		return fmt.Sprintf("[无检索结果] 未检索到与“%s”相关的资料，下游不得编造来源", query), nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "检索式：%s\n命中 %d 条：\n", query, len(snippets))
	for i, s := range snippets {
		title := s.Title
		if title == "" {
			title = fmt.Sprintf("文档#%d", s.DocumentID)
		}
		fmt.Fprintf(&b, "\n[%d] %s (doc_id=%d)\n%s\n", i+1, title, s.DocumentID, truncateRunes(s.Content, 1200))
	}
	return b.String(), nil
}

func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

func (we *WorkflowEngine) executeConditionNode(node *WorkflowNode, results map[string]interface{}) (interface{}, error) {
	conditionFn, _ := node.Config["condition"].(string)
	conditionResult := we.interpolateTemplate(conditionFn, results)

	// 评估比较表达式: >, <, >=, <=, ==, !=
	evaluated := we.evaluateExpression(conditionResult)
	return fmt.Sprintf("%v", evaluated), nil
}

// evaluateExpression 简单的表达式求值器，支持 >, <, >=, <=, ==, !=
func (we *WorkflowEngine) evaluateExpression(expr string) bool {
	expr = strings.TrimSpace(expr)

	operators := []string{">=", "<=", "!=", "==", ">", "<"}
	for _, op := range operators {
		parts := strings.SplitN(expr, op, 2)
		if len(parts) == 2 {
			left := strings.TrimSpace(parts[0])
			right := strings.TrimSpace(parts[1])
			leftIsNum := we.isNumeric(left)
			rightIsNum := we.isNumeric(right)
			// 两边均为数字字面量时按数值比较，否则按字符串比较（保持 == 与 != 对称）
			if leftIsNum && rightIsNum {
				leftVal := we.parseNumber(left)
				rightVal := we.parseNumber(right)
				switch op {
				case ">":
					return leftVal > rightVal
				case "<":
					return leftVal < rightVal
				case ">=":
					return leftVal >= rightVal
				case "<=":
					return leftVal <= rightVal
				case "==":
					return leftVal == rightVal
				case "!=":
					return leftVal != rightVal
				}
			}
			switch op {
			case ">":
				return left > right
			case "<":
				return left < right
			case ">=":
				return left >= right
			case "<=":
				return left <= right
			case "==":
				return left == right
			case "!=":
				return left != right
			}
		}
	}

	// 如果不是比较表达式，检查是否为字面量 "true"/"false"
	lower := strings.ToLower(expr)
	return lower == "true" || lower == "1" || lower == "yes"
}

// isNumeric 判断字符串是否可解析为 float64
func (we *WorkflowEngine) isNumeric(s string) bool {
	_, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	return err == nil
}

func (we *WorkflowEngine) parseNumber(s string) float64 {
	var v float64
	fmt.Sscanf(s, "%f", &v)
	return v
}

func (we *WorkflowEngine) executeParallelNode(ctx context.Context, node *WorkflowNode, scope *execScope, depth int) (interface{}, error) {
	children, ok := node.Config["children"].([]interface{})
	if !ok || len(children) == 0 {
		return nil, fmt.Errorf("parallel node has no children")
	}

	childIDs := make([]string, 0, len(children))
	for _, c := range children {
		if id, ok := c.(string); ok {
			childIDs = append(childIDs, id)
		}
	}

	var wg sync.WaitGroup
	var mu sync.Mutex
	var firstErr error

	childResults := make(map[string]interface{}, len(childIDs))
	nodeMap := make(map[string]*WorkflowNode)

	we.mu.RLock()
	for _, wf := range we.workflows {
		for i := range wf.Nodes {
			nodeMap[wf.Nodes[i].ID] = &wf.Nodes[i]
		}
	}
	we.mu.RUnlock()

	for _, childID := range childIDs {
		childNode, exists := nodeMap[childID]
		if !exists {
			continue
		}

		wg.Add(1)
		go func(child *WorkflowNode) {
			defer wg.Done()

			select {
			case <-ctx.Done():
				mu.Lock()
				if firstErr == nil {
					firstErr = ctx.Err()
				}
				mu.Unlock()
				return
			default:
			}

			result, err := we.executeNode(ctx, child, scope, depth+1)
			if err != nil {
				mu.Lock()
				if firstErr == nil {
					firstErr = fmt.Errorf("child node %s: %w", child.ID, err)
				}
				mu.Unlock()
				return
			}

			mu.Lock()
			childResults[child.ID] = result
			mu.Unlock()
		}(childNode)
	}

	wg.Wait()

	if firstErr != nil {
		return childResults, firstErr
	}

	// 嵌套 parallel 时多个父 goroutine 会走到这里，写回必须经过 scope 的互斥锁，
	// 否则并发写同一个 results map 触发 fatal error: concurrent map writes
	for k, v := range childResults {
		scope.set(k, v)
	}

	return childResults, nil
}

func (we *WorkflowEngine) resolveNextNode(node *WorkflowNode, results map[string]interface{}) string {
	if len(node.NextNodes) == 0 {
		return ""
	}

	if node.Type == NodeTypeCondition {
		conditionResult, ok := results[node.ID].(string)
		if ok && conditionResult == "true" && len(node.NextNodes) > 0 {
			return node.NextNodes[0]
		}
		if len(node.NextNodes) > 1 {
			return node.NextNodes[1]
		}
	}

	return node.NextNodes[0]
}

func (we *WorkflowEngine) interpolateTemplate(tmpl string, vars map[string]interface{}) string {
	result := tmpl
	for k, v := range vars {
		placeholder := "{{" + k + "}}"
		var valStr string
		switch val := v.(type) {
		case string:
			valStr = val
		default:
			bytes, err := json.Marshal(val)
			if err != nil {
				valStr = fmt.Sprintf("%v", val)
			} else {
				valStr = string(bytes)
			}
		}
		result = replaceAll(result, placeholder, valStr)
	}
	return result
}

func (we *WorkflowEngine) LoadBuiltinWorkflows() {
	we.RegisterWorkflow(&WorkflowDefinition{
		ID:          "doc_quality_check",
		Name:        "文档质量检查工作流",
		Description: "自动检查文档质量并生成改进建议",
		Nodes: []WorkflowNode{
			{ID: "start", Type: NodeTypeStart, Name: "开始", NextNodes: []string{"analyze"}},
			{ID: "analyze", Type: NodeTypeLLM, Name: "分析文档质量", Config: map[string]interface{}{
				"system": "你是一个文档质量评估专家。",
				"prompt": "请评估以下文档的质量，从完整性、清晰度、结构性三个维度打分(0-100)，并给出改进建议：\n\n{{document_content}}",
			}, NextNodes: []string{"generate_report"}},
			{ID: "generate_report", Type: NodeTypeLLM, Name: "生成改进报告", Config: map[string]interface{}{
				"system": "你是一个技术写作专家。",
				"prompt": "基于以下质量评估，生成结构化的改进报告：\n\n{{analyze}}",
			}, NextNodes: []string{"end"}},
			{ID: "end", Type: NodeTypeEnd, Name: "结束"},
		},
	})

	we.RegisterWorkflow(&WorkflowDefinition{
		ID:          "doc_summarize_and_tag",
		Name:        "文档摘要与标签生成",
		Description: "自动生成文档摘要和标签",
		Nodes: []WorkflowNode{
			{ID: "start", Type: NodeTypeStart, Name: "开始", NextNodes: []string{"summarize"}},
			{ID: "summarize", Type: NodeTypeLLM, Name: "生成摘要", Config: map[string]interface{}{
				"system": "你是一个知识库AI助手。",
				"prompt": "请为以下文档生成200字以内的摘要：\n\n{{document_content}}",
			}, NextNodes: []string{"tag"}},
			{ID: "tag", Type: NodeTypeLLM, Name: "提取标签", Config: map[string]interface{}{
				"system": "你是一个标签提取助手。",
				"prompt": "请为以下文档提取5-10个标签（JSON数组格式）：\n\n{{document_content}}",
			}, NextNodes: []string{"end"}},
			{ID: "end", Type: NodeTypeEnd, Name: "结束"},
		},
	})

	// P2-8: Deep Research workflow — multi-step research with plan → parallel search → synthesis → fact-check
	we.RegisterWorkflow(&WorkflowDefinition{
		ID:          "deep_research",
		Name:        "深度研究工作流",
		Description: "多步骤深度研究：制定研究计划 → 并行检索多源信息 → 综合分析 → 事实核查 → 生成研究报告",
		Nodes: []WorkflowNode{
			{ID: "start", Type: NodeTypeStart, Name: "开始", NextNodes: []string{"plan"}},

			// Step 1: Generate research plan
			{ID: "plan", Type: NodeTypeLLM, Name: "制定研究计划", Config: map[string]interface{}{
				"system": `你是一个深度研究规划专家。根据用户问题制定结构化的研究计划。
输出JSON格式：
{"sub_questions": ["子问题1", "子问题2", "子问题3"], "search_keywords": ["关键词1", "关键词2"], "research_angles": ["角度1", "角度2"]}`,
				"prompt": "请为以下问题制定深度研究计划：\n\n{{question}}",
			}, NextNodes: []string{"parallel_search"}},

			// Step 2: 并行多源检索（真实检索节点，检索函数由调用方按权限范围注入）
			{ID: "parallel_search", Type: NodeTypeParallel, Name: "并行多源检索", Config: map[string]interface{}{
				"children": []interface{}{"search_a", "search_b", "search_c"},
			}, NextNodes: []string{"synthesize"}},

			{ID: "search_a", Type: NodeTypeRetrieval, Name: "检索源A", Config: map[string]interface{}{
				"query": "{{question}} 背景信息 关键事实",
				"top_k": 5,
			}},
			{ID: "search_b", Type: NodeTypeRetrieval, Name: "检索源B", Config: map[string]interface{}{
				"query": "{{question}} 统计数据 案例 实证",
				"top_k": 5,
			}},
			{ID: "search_c", Type: NodeTypeRetrieval, Name: "检索源C", Config: map[string]interface{}{
				"query": "{{question}} 争议 对比 趋势",
				"top_k": 5,
			}},

			// Step 3: Synthesize all sources
			{ID: "synthesize", Type: NodeTypeLLM, Name: "综合分析", Config: map[string]interface{}{
				"system": `你是一个研究综合分析专家。将多个检索源的信息整合为连贯的分析报告。
要求：
1. 只使用检索结果中确实出现的信息，绝不补充检索结果之外的"常识"
2. 若某个检索源显示"无检索结果"，如实说明该角度缺乏资料，不得用想象填补
3. 消除信息冲突，保留最可靠的信息
4. 每条关键论断后标注其来源编号 [n] 或 doc_id
5. 生成结构化的综合报告`,
				"prompt": "请综合以下检索结果，生成分析报告：\n\n源A：{{search_a}}\n\n源B：{{search_b}}\n\n源C：{{search_c}}",
			}, NextNodes: []string{"fact_check"}},

			// Step 4: Fact-check
			{ID: "fact_check", Type: NodeTypeLLM, Name: "事实核查", Config: map[string]interface{}{
				"system": `你是一个事实核查专家。检查综合报告中的关键声明，标注可信度等级：
- HIGH: 有多个独立来源支持
- MEDIUM: 有单一来源支持
- LOW: 无法验证
- CONFLICTING: 来源之间存在矛盾

输出格式：
{"verified_claims": [{"claim": "...", "confidence": "HIGH", "source": "..."}], "unverified_claims": ["..."], "overall_reliability": 0.85}`,
				"prompt": "请对以下综合报告进行事实核查：\n\n{{synthesize}}",
			}, NextNodes: []string{"final_report"}},

			// Step 5: Generate final research report
			{ID: "final_report", Type: NodeTypeLLM, Name: "生成研究报告", Config: map[string]interface{}{
				"system": `你是一个高级研究报告撰写专家。基于研究计划、多源检索结果、综合分析和事实核查，生成一份专业的深度研究报告。

报告结构：
1. 执行摘要
2. 研究背景与方法
3. 主要发现
4. 详细分析
5. 事实核查摘要
6. 结论与建议
7. 参考来源

硬性要求：
- 每条关键论断必须附带来源（[n] 编号或 doc_id），无来源的论断不得写入
- 所有检索源均为"无检索结果"时，报告必须明确声明"当前知识库无相关资料"，不得生成内容
- 不得出现检索结果中不存在的数字、名称或引述`,
				"prompt": "请生成深度研究报告：\n\n研究计划：{{plan}}\n\n综合分析：{{synthesize}}\n\n事实核查：{{fact_check}}\n\n问题：{{question}}",
			}, NextNodes: []string{"end"}},

			{ID: "end", Type: NodeTypeEnd, Name: "结束"},
		},
	})

	logx.Infof("[WorkflowEngine] Loaded %d builtin workflows", len(we.workflows))
}

// W6：自然语言 → 工作流 DAG 生成。
//
// 生成的工作流刻意注册为草稿（Active=false）：DAG 由模型从一句话描述推出来，
// 节点数、prompt、并行度都不可控，直接激活会让一次描述错误变成一次无界执行。
// 校验器（validateGeneratedWorkflow）先把结构缺陷拦在注册之前，
// 管理员在界面上确认无误后再用 ToggleWorkflow 激活 —— 这是刻意的审批流。

// 允许生成的节点类型。tool/condition/parallel 刻意排除：
// tool 的参数字段 schema 复杂，模型生成的 JSON 极易与白名单/参数校验错位；
// condition/parallel 引入了多分支与嵌套递归，校验与可预测性都显著更难。
// 一条「线性 LLM 流水线 + 可选真实检索」已经能覆盖绝大多数「写段处理流程」的需求。
var generateAllowedNodeTypes = map[WorkflowNodeType]bool{
	NodeTypeStart:     true,
	NodeTypeEnd:       true,
	NodeTypeLLM:       true,
	NodeTypeRetrieval: true,
}

// GenerateWorkflow 让模型把一句自然语言描述转成可执行的工作流定义。
//
// 返回的 *WorkflowDefinition 是未注册、Active=false 的草稿；
// 调用方（API 层）负责注册与返回，注册前必须先过 validateGeneratedWorkflow。
// 模型只输出 JSON，由本方法解析、补默认值、跑结构校验；任何一步失败都返回 error，
// 绝不把半成品注册进引擎（那会污染 ListWorkflows 并被后续执行撞上）。
func (we *WorkflowEngine) GenerateWorkflow(ctx context.Context, description string) (*WorkflowDefinition, error) {
	if we.provider == nil {
		return nil, fmt.Errorf("AI provider not available")
	}
	description = strings.TrimSpace(description)
	if description == "" {
		return nil, fmt.Errorf("description is empty")
	}

	prompt := buildGenerateWorkflowPrompt(description)
	resp, err := we.provider.Chat(ctx,
		"你是一个工作流编排引擎。把用户的自然语言流程描述翻译成符合指定 schema 的 DAG JSON。"+
			"只输出 JSON 本体，不要 markdown 代码块、不要解释、不要多余文本。",
		[]provider.Message{provider.NewTextMessage("user", prompt)},
		provider.WithMaxTokens(4096), provider.WithTemperature(0))
	if err != nil {
		return nil, err
	}

	wf, err := parseGeneratedWorkflow(resp.Content, description)
	if err != nil {
		return nil, err
	}
	if err := validateGeneratedWorkflow(wf); err != nil {
		return nil, err
	}
	return wf, nil
}

// buildGenerateWorkflowPrompt 生成 prompt。schema 说明必须与 WorkflowNode/WorkflowDefinition
// 的 JSON tag 完全一致，否则解析会丢字段；节点类型只列允许生成的那四种。
func buildGenerateWorkflowPrompt(description string) string {
	return "请把下面的流程描述翻译成工作流 DAG，并只输出一个 JSON 对象，结构如下：\n" +
		"{\n" +
		"  \"name\": \"简短的工作流名称（中文，20 字以内）\",\n" +
		"  \"description\": \"一句话说明这个工作流做什么（中文，50 字以内）\",\n" +
		"  \"nodes\": [\n" +
		"    { \"id\": \"唯一短 id，如 start/step1/step2/end\", \"type\": \"start|llm|retrieval|end\", \"name\": \"节点中文名\", \"config\": {...}, \"next_nodes\": [\"下一个节点 id\"] }\n" +
		"  ]\n" +
		"}\n\n" +
		"硬性规则：\n" +
		"1. 必须恰好一个 type=start 的节点，它的 next_nodes 指向第一个处理节点；必须恰好一个 type=end 的节点，且 end 的 next_nodes 留空数组。\n" +
		"2. 除 start 与 end 外，只允许 type=llm 或 type=retrieval；不要生成 tool/condition/parallel。\n" +
		"3. type=llm 节点的 config 必须含两个字符串字段：\"system\"（该步骤的角色设定）与 \"prompt\"（该步骤的提示词，可用 {{上游节点id}} 插值引用上游结果，例如 {{step1}}；引用用户输入用 {{question}} 或 {{document_content}}）。\n" +
		"4. type=retrieval 节点的 config 必须含字符串字段 \"query\"（检索式，可用 {{question}} 等插值）与数字字段 \"top_k\"（1-10 的整数）。\n" +
		"5. 每个节点的 next_nodes 里列出的 id 必须真实存在于 nodes 中；整张图必须从 start 出发能走到 end，且不能有环（每个节点最多被访问一次）。\n" +
		"6. 节点 id 用小写字母/数字/下划线，不要中文、不要空格。\n" +
		"7. 流程尽量精简：2-5 个处理节点足够；不要为了显得复杂而堆叠同义步骤。\n\n" +
		"示例（描述：把一段文档先检索背景、再总结、再翻译成英文）：\n" +
		"{\n" +
		"  \"name\": \"文档总结与翻译\",\n" +
		"  \"description\": \"先检索相关背景，再总结原文，最后翻译成英文\",\n" +
		"  \"nodes\": [\n" +
		"    {\"id\": \"start\", \"type\": \"start\", \"name\": \"开始\", \"next_nodes\": [\"retrieve\"]},\n" +
		"    {\"id\": \"retrieve\", \"type\": \"retrieval\", \"name\": \"检索背景\", \"config\": {\"query\": \"{{question}} 相关背景\", \"top_k\": 5}, \"next_nodes\": [\"summarize\"]},\n" +
		"    {\"id\": \"summarize\", \"type\": \"llm\", \"name\": \"总结原文\", \"config\": {\"system\": \"你是一个文档总结专家。\", \"prompt\": \"请总结以下文档并结合背景：\\n\\n文档：{{document_content}}\\n\\n背景：{{retrieve}}\"}, \"next_nodes\": [\"translate\"]},\n" +
		"    {\"id\": \"translate\", \"type\": \"llm\", \"name\": \"翻译成英文\", \"config\": {\"system\": \"你是一个专业翻译。\", \"prompt\": \"请把以下总结翻译成英文：\\n\\n{{summarize}}\"}, \"next_nodes\": [\"end\"]},\n" +
		"    {\"id\": \"end\", \"type\": \"end\", \"name\": \"结束\", \"next_nodes\": []}\n" +
		"  ]\n" +
		"}\n\n" +
		"用户的流程描述：\n" + description + "\n\n" +
		"现在只输出 JSON 对象：\n"
}

// parseGeneratedWorkflow 解析模型输出为草稿定义。
//
// 模型即便被叮嘱也只输出 JSON，仍可能夹带 ```json 围栏或前后杂文，
// 这里先抠出第一段平衡花括号的 {...} 再反序列化；id/name 缺失时补默认值，
// 但不做语义校验 —— 那是 validateGeneratedWorkflow 的职责。
func parseGeneratedWorkflow(content, description string) (*WorkflowDefinition, error) {
	seg := extractJSONSegment(content)
	if seg == "" {
		return nil, fmt.Errorf("model output contains no JSON object")
	}
	var raw struct {
		Name        string         `json:"name"`
		Description string         `json:"description"`
		Nodes       []WorkflowNode `json:"nodes"`
	}
	if err := json.Unmarshal([]byte(seg), &raw); err != nil {
		return nil, fmt.Errorf("parse generated workflow JSON: %v", err)
	}
	if len(raw.Nodes) == 0 {
		return nil, fmt.Errorf("generated workflow has no nodes")
	}
	name := strings.TrimSpace(raw.Name)
	if name == "" {
		name = "AI 生成工作流"
	}
	desc := strings.TrimSpace(raw.Description)
	if desc == "" {
		desc = description
	}
	// 给缺失的 id/name 兜底，避免后续校验/执行因空 id 报错。
	for i := range raw.Nodes {
		if strings.TrimSpace(raw.Nodes[i].ID) == "" {
			raw.Nodes[i].ID = fmt.Sprintf("node_%d", i+1)
		}
		if strings.TrimSpace(raw.Nodes[i].Name) == "" {
			raw.Nodes[i].Name = raw.Nodes[i].ID
		}
		if raw.Nodes[i].Config == nil {
			raw.Nodes[i].Config = map[string]interface{}{}
		}
		if raw.Nodes[i].NextNodes == nil {
			raw.Nodes[i].NextNodes = []string{}
		}
	}
	return &WorkflowDefinition{
		ID:          fmt.Sprintf("wf_gen_%d", time.Now().UnixNano()),
		Name:        name,
		Description: desc,
		Nodes:       raw.Nodes,
		Edges:       []WorkflowEdge{},
		Active:      false,
	}, nil
}

// validateGeneratedWorkflow 结构校验：只接受能确定性执行的线性 DAG。
// 任何一条不满足都返回带具体节点 id 的 error，便于前端把原因回显给管理员。
//
// 分两遍：第一遍收全 id 集合并校验单节点字段（类型/config），
// 第二遍再查 next_nodes 是否都指向已存在的 id —— 单遍做这件事会把
// 「前向引用」（start 先声明、指向后面才出现的 a）误判成未知 id。
func validateGeneratedWorkflow(wf *WorkflowDefinition) error {
	if len(wf.Nodes) == 0 {
		return fmt.Errorf("no nodes")
	}
	idSet := make(map[string]bool, len(wf.Nodes))
	var startIDs, endIDs int
	for i := range wf.Nodes {
		n := &wf.Nodes[i]
		if n.ID == "" {
			return fmt.Errorf("node #%d has empty id", i)
		}
		if idSet[n.ID] {
			return fmt.Errorf("duplicate node id: %s", n.ID)
		}
		idSet[n.ID] = true
		if !generateAllowedNodeTypes[n.Type] {
			return fmt.Errorf("node %s: unsupported type %q (only start/llm/retrieval/end allowed)", n.ID, n.Type)
		}
		switch n.Type {
		case NodeTypeStart:
			startIDs++
		case NodeTypeEnd:
			endIDs++
		case NodeTypeLLM:
			if p, ok := n.Config["prompt"].(string); !ok || p == "" {
				return fmt.Errorf("node %s: llm node requires non-empty config.prompt", n.ID)
			}
		case NodeTypeRetrieval:
			if q, ok := n.Config["query"].(string); !ok || q == "" {
				return fmt.Errorf("node %s: retrieval node requires non-empty config.query", n.ID)
			}
		}
	}
	if startIDs != 1 {
		return fmt.Errorf("exactly one start node required, got %d", startIDs)
	}
	if endIDs != 1 {
		return fmt.Errorf("exactly one end node required, got %d", endIDs)
	}
	// 第二遍：next_nodes 引用必须指向已存在的 id。
	for _, n := range wf.Nodes {
		for _, nx := range n.NextNodes {
			if !idSet[nx] {
				return fmt.Errorf("node %s: next_nodes references unknown id %q", n.ID, nx)
			}
		}
	}

	// 拓扑可达性 + 无环：从 start 做 BFS，每个节点最多入队一次，
	// 最后必须恰好访问到全部节点（否则有环或存在不可达节点，都拒）。
	var startID string
	for _, n := range wf.Nodes {
		if n.Type == NodeTypeStart {
			startID = n.ID
			break
		}
	}
	adj := make(map[string][]string, len(wf.Nodes))
	for _, n := range wf.Nodes {
		adj[n.ID] = n.NextNodes
	}
	visited := make(map[string]bool)
	queue := []string{startID}
	visited[startID] = true
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		for _, nx := range adj[cur] {
			if !visited[nx] {
				visited[nx] = true
				queue = append(queue, nx)
			}
		}
	}
	if len(visited) != len(wf.Nodes) {
		return fmt.Errorf("graph is not a single start->end DAG: %d of %d nodes unreachable from start", len(visited), len(wf.Nodes))
	}
	return nil
}

// extractJSONSegment 从模型输出里抠出第一段平衡花括号的 {...}。
// 从第一个 '{' 开始做花括号配对计数（跳过字符串字面量内的花括号），
// 找到与之闭合的 '}' 为止；花括号不平衡（模型截断）时返回空串，
// 让上层报「no JSON object」。不做完整 JSON 解析 —— 那是 Unmarshal 的事。
func extractJSONSegment(s string) string {
	s = strings.TrimSpace(s)
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	inStr := false
	esc := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inStr {
			if esc {
				esc = false
			} else if c == '\\' {
				esc = true
			} else if c == '"' {
				inStr = false
			}
			continue
		}
		switch c {
		case '"':
			inStr = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}
