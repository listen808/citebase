package rag

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"
	"sync"
	"time"

	"github.com/listen808/citebase/internal/ai/mcp"
	"github.com/listen808/citebase/internal/ai/observability"
	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/ctxkeys"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

type ToolDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

type ToolResult struct {
	ToolName string `json:"tool_name"`
	Success  bool   `json:"success"`
	Result   string `json:"result"`
	Error    string `json:"error,omitempty"`
}

type FunctionDefinition struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Parameters  map[string]interface{} `json:"parameters"`
}

func convertToFunctions(tools map[string]ToolExecutor) []FunctionDefinition {
	funcs := make([]FunctionDefinition, 0, len(tools))
	for _, tool := range tools {
		d := tool.Definition()
		funcs = append(funcs, FunctionDefinition{
			Name:        d.Name,
			Description: d.Description,
			Parameters:  d.Parameters,
		})
	}
	return funcs
}

type ToolExecutor interface {
	Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error)
	Definition() ToolDefinition
}

// SpacePrivilegeChecker 检查用户在空间中的权限
type SpacePrivilegeChecker interface {
	GetUserSpacePrivilege(spaceID, userID int) (int, error)
}

type SearchDocumentsTool struct {
	retriever *Retriever
}

// NewSearchDocumentsTool 构造检索文档工具（供多 agent 编排等场景复用）。
func NewSearchDocumentsTool(retriever *Retriever) *SearchDocumentsTool {
	return &SearchDocumentsTool{retriever: retriever}
}

func (t *SearchDocumentsTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	query, _ := params["query"].(string)
	if query == "" {
		return &ToolResult{ToolName: "search_documents", Success: false, Error: "query is required"}, nil
	}

	spaceID := 0
	if sid, ok := params["space_id"].(float64); ok {
		spaceID = int(sid)
	}

	// space_id 由模型给出（0=全部），hybridRetrieve 本身不做归属过滤，
	// 因此必须在返回前按调用者可见空间裁剪，否则可枚举任意私有空间的正文片段。
	sources, _ := t.retriever.hybridRetrieve(ctx, query, query, spaceID, 5)
	// P0-2 修复：统一走 Retriever.filterSourcesForCtx（空间过滤 + 文档 ACL 二次校验），
	// 与 AgentChat 其他出口保持一致，防止枚举任意私有空间正文片段。
	sources = t.retriever.filterSourcesForCtx(ctx, sources)
	var result strings.Builder
	for i, src := range sources {
		result.WriteString(fmt.Sprintf("%d. %s (相似度: %.2f)\n", i+1, src.DocumentName, src.Score))
		if src.Snippet != "" {
			snippet := src.Snippet
			if len(snippet) > 200 {
				snippet = snippet[:200] + "..."
			}
			result.WriteString(fmt.Sprintf("   %s\n", snippet))
		}
	}
	if len(sources) == 0 {
		return &ToolResult{ToolName: "search_documents", Success: true, Result: "没有可访问的匹配文档"}, nil
	}
	return &ToolResult{ToolName: "search_documents", Success: true, Result: result.String()}, nil
}

func (t *SearchDocumentsTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "search_documents",
		Description: "搜索知识库文档，返回与查询最相关的文档列表",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"query":    map[string]interface{}{"type": "string", "description": "搜索查询词"},
				"space_id": map[string]interface{}{"type": "integer", "description": "限定搜索空间ID，0表示全部"},
			},
			"required": []string{"query"},
		},
	}
}

type CreateDocumentTool struct {
	docRepo  *workspacerepo.DocumentRepository
	spaceSvc SpacePrivilegeChecker
}

func (t *CreateDocumentTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	name, _ := params["name"].(string)
	spaceIDF, _ := params["space_id"].(float64)
	spaceID := int(spaceIDF)
	content, _ := params["content"].(string)
	parentIDF, _ := params["parent_id"].(float64)
	parentID := int(parentIDF)

	if name == "" || spaceID == 0 {
		return &ToolResult{ToolName: "create_document", Success: false, Error: "name and space_id are required"}, nil
	}
	// 权限检查：用户需有空间编辑权限(priv >= 1)
	userID := ctxkeys.UserID(ctx)
	if userID > 0 && spaceID > 0 && t.spaceSvc != nil {
		priv, err := t.spaceSvc.GetUserSpacePrivilege(spaceID, userID)
		if err != nil || priv < 1 {
			return &ToolResult{ToolName: "create_document", Success: false, Error: "无该空间编辑权限"}, nil
		}
	}
	doc := &workspacemodel.Document{
		Name:     name,
		SpaceID:  spaceID,
		Content:  content,
		ParentID: parentID,
	}

	err := t.docRepo.Create(doc)
	if err != nil {
		return &ToolResult{ToolName: "create_document", Success: false, Error: err.Error()}, nil
	}

	return &ToolResult{
		ToolName: "create_document",
		Success:  true,
		Result:   fmt.Sprintf("文档创建成功: ID=%d, 名称=%s, 空间ID=%d", doc.ID, name, spaceID),
	}, nil
}

func (t *CreateDocumentTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "create_document",
		Description: "在指定空间创建新文档",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"name":      map[string]interface{}{"type": "string", "description": "文档名称"},
				"space_id":  map[string]interface{}{"type": "integer", "description": "目标空间ID"},
				"content":   map[string]interface{}{"type": "string", "description": "文档内容(Markdown)"},
				"parent_id": map[string]interface{}{"type": "integer", "description": "父文档ID，0为顶层"},
			},
			"required": []string{"name", "space_id"},
		},
	}
}

type MoveDocumentTool struct {
	docRepo  *workspacerepo.DocumentRepository
	spaceSvc SpacePrivilegeChecker
}

func (t *MoveDocumentTool) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	docIDF, _ := params["document_id"].(float64)
	docID := int(docIDF)
	targetSpaceIDF, _ := params["target_space_id"].(float64)
	targetSpaceID := int(targetSpaceIDF)
	parentIDF, _ := params["parent_id"].(float64)
	parentID := int(parentIDF)

	if docID == 0 {
		return &ToolResult{ToolName: "move_document", Success: false, Error: "document_id is required"}, nil
	}

	doc, err := t.docRepo.FindByID(docID)
	if err != nil || doc == nil {
		return &ToolResult{ToolName: "move_document", Success: false, Error: "文档不存在"}, nil
	}
	// 权限检查：源空间和目标空间
	userID := ctxkeys.UserID(ctx)
	if userID > 0 && t.spaceSvc != nil {
		// 检查源空间权限
		priv, err := t.spaceSvc.GetUserSpacePrivilege(doc.SpaceID, userID)
		if err != nil || priv < 1 {
			return &ToolResult{ToolName: "move_document", Success: false, Error: "无源空间编辑权限"}, nil
		}
		// 检查目标空间权限
		if targetSpaceID > 0 {
			priv, err := t.spaceSvc.GetUserSpacePrivilege(targetSpaceID, userID)
			if err != nil || priv < 1 {
				return &ToolResult{ToolName: "move_document", Success: false, Error: "无目标空间编辑权限"}, nil
			}
		}
	}
	if targetSpaceID > 0 {
		doc.SpaceID = targetSpaceID
	}
	if parentID > 0 {
		doc.ParentID = parentID
	}

	if err := t.docRepo.Update(doc); err != nil {
		return &ToolResult{ToolName: "move_document", Success: false, Error: err.Error()}, nil
	}

	return &ToolResult{
		ToolName: "move_document",
		Success:  true,
		Result:   fmt.Sprintf("文档已移动: ID=%d → 空间ID=%d, 父ID=%d", docID, doc.SpaceID, doc.ParentID),
	}, nil
}

func (t *MoveDocumentTool) Definition() ToolDefinition {
	return ToolDefinition{
		Name:        "move_document",
		Description: "移动文档到其他空间或父文档下",
		Parameters: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"document_id":     map[string]interface{}{"type": "integer", "description": "要移动的文档ID"},
				"target_space_id": map[string]interface{}{"type": "integer", "description": "目标空间ID"},
				"parent_id":       map[string]interface{}{"type": "integer", "description": "目标父文档ID"},
			},
			"required": []string{"document_id"},
		},
	}
}

// MCPToolAdapter adapts an MCP tool to the ToolExecutor interface.
type MCPToolAdapter struct {
	client   *mcp.Client
	fullName string // "serverName:toolName"
	toolDef  ToolDefinition
}

// NewMCPToolAdapter creates a new adapter for an MCP tool.
func NewMCPToolAdapter(client *mcp.Client, fullName string) *MCPToolAdapter {
	serverName, toolName, _ := mcp.ParseToolName(fullName)
	entry, _ := client.GetServer(serverName)

	def := ToolDefinition{
		Name:        fullName,
		Description: fmt.Sprintf("[MCP:%s] %s", serverName, toolName),
		Parameters:  map[string]interface{}{},
	}

	if entry != nil {
		for _, t := range entry.Tools {
			if t.Name == toolName {
				def.Description = fmt.Sprintf("[MCP:%s] %s", serverName, t.Description)
				def.Parameters = t.Parameters
				break
			}
		}
	}

	return &MCPToolAdapter{
		client:   client,
		fullName: fullName,
		toolDef:  def,
	}
}

func (a *MCPToolAdapter) Execute(ctx context.Context, params map[string]interface{}) (*ToolResult, error) {
	result, err := a.client.CallToolByFullName(ctx, a.fullName, params)
	if err != nil {
		return &ToolResult{
			ToolName: a.fullName,
			Success:  false,
			Error:    err.Error(),
		}, nil
	}

	if result.IsError {
		var errText string
		for _, c := range result.Content {
			if c.Type == "text" {
				errText += c.Text
			}
		}
		return &ToolResult{
			ToolName: a.fullName,
			Success:  false,
			Error:    errText,
		}, nil
	}

	var text string
	for _, c := range result.Content {
		if c.Type == "text" {
			text += c.Text
		}
	}

	return &ToolResult{
		ToolName: a.fullName,
		Success:  true,
		Result:   text,
	}, nil
}

func (a *MCPToolAdapter) Definition() ToolDefinition {
	return a.toolDef
}

type AgentChat struct {
	provider          provider.Provider
	tools             map[string]ToolExecutor
	maxRounds         int
	retriever         *Retriever
	hydeRetriever     *HyDERetriever
	adaptiveRetriever *AdaptiveRetriever
	correctiveRAG     *CorrectiveRAG
	// P1-7: 前置工作流拦截器（斜杠命令 / 模式匹配），在 LLM 调用前尝试确定性处理
	workflowHandler *WorkflowHandler
	// P1-5: 上下文预算 + 超预算压缩历史（compaction），agent 链路此前对历史零截断
	compression *ContextCompressionLayer
}

func NewAgentChat(p provider.Provider, retriever *Retriever, spaceSvc SpacePrivilegeChecker, adaptiveRetriever *AdaptiveRetriever, correctiveRAG *CorrectiveRAG) *AgentChat {
	agent := &AgentChat{
		provider:          p,
		tools:             make(map[string]ToolExecutor),
		maxRounds:         10,
		retriever:         retriever,
		adaptiveRetriever: adaptiveRetriever,
		correctiveRAG:     correctiveRAG,
		compression:       NewContextCompressionLayer(p, DefaultCompressionConfig()),
	}

	agent.tools["search_documents"] = &SearchDocumentsTool{retriever: retriever}
	agent.tools["create_document"] = &CreateDocumentTool{
		docRepo:  workspacerepo.NewDocumentRepository(),
		spaceSvc: spaceSvc,
	}
	agent.tools["move_document"] = &MoveDocumentTool{
		docRepo:  workspacerepo.NewDocumentRepository(),
		spaceSvc: spaceSvc,
	}

	// 注册全局工具注册中心中的工具
	registry := GetToolRegistry()
	for name, tool := range registry.GetAll() {
		agent.tools[name] = tool
	}

	return agent
}

// NewAgentChatWithTools 创建一个只包含指定工具的 AgentChat 实例
func NewAgentChatWithTools(p provider.Provider, retriever *Retriever, enabledTools []string, spaceSvc SpacePrivilegeChecker, adaptiveRetriever *AdaptiveRetriever, correctiveRAG *CorrectiveRAG) *AgentChat {
	ac := &AgentChat{
		provider:          p,
		tools:             make(map[string]ToolExecutor),
		maxRounds:         10,
		retriever:         retriever,
		adaptiveRetriever: adaptiveRetriever,
		correctiveRAG:     correctiveRAG,
		compression:       NewContextCompressionLayer(p, DefaultCompressionConfig()),
	}

	if len(enabledTools) > 0 {
		enabledSet := make(map[string]bool)
		for _, name := range enabledTools {
			enabledSet[name] = true
		}

		// 注册匹配的内置工具
		builtinTools := map[string]ToolExecutor{
			"search_documents": &SearchDocumentsTool{retriever: retriever},
			"create_document":  &CreateDocumentTool{docRepo: workspacerepo.NewDocumentRepository(), spaceSvc: spaceSvc},
			"move_document":    &MoveDocumentTool{docRepo: workspacerepo.NewDocumentRepository(), spaceSvc: spaceSvc},
		}
		for name, tool := range builtinTools {
			if enabledSet[name] {
				ac.tools[name] = tool
			}
		}

		// 注册匹配的全局注册表工具
		registry := GetToolRegistry()
		for name, tool := range registry.GetAll() {
			if enabledSet[name] {
				ac.tools[name] = tool
			}
		}
	} else {
		// 未指定工具列表时，注册所有工具（保持原有行为）
		ac.tools["search_documents"] = &SearchDocumentsTool{retriever: retriever}
		ac.tools["create_document"] = &CreateDocumentTool{docRepo: workspacerepo.NewDocumentRepository(), spaceSvc: spaceSvc}
		ac.tools["move_document"] = &MoveDocumentTool{docRepo: workspacerepo.NewDocumentRepository(), spaceSvc: spaceSvc}
		registry := GetToolRegistry()
		for name, tool := range registry.GetAll() {
			ac.tools[name] = tool
		}
	}

	return ac
}

func (a *AgentChat) RegisterTool(name string, tool ToolExecutor) {
	a.tools[name] = tool
}

// SetWorkflowHandler 设置前置工作流拦截器（P1-7）
// 设置后 Chat / ChatWithSpace 会在 LLM 调用前尝试斜杠命令与模式匹配，
// 命中则直接返回确定性响应，节省 token 并提供一致的快捷指令体验。
func (a *AgentChat) SetWorkflowHandler(h *WorkflowHandler) {
	a.workflowHandler = h
}

// getToolDefinitions 将当前工具集转换为 provider.ToolDefinition 列表
func (a *AgentChat) getToolDefinitions() []provider.ToolDefinition {
	defs := make([]provider.ToolDefinition, 0, len(a.tools))
	for _, tool := range a.tools {
		td := tool.Definition()
		defs = append(defs, provider.ToolDefinition{
			Name:        td.Name,
			Description: td.Description,
			Parameters:  td.Parameters,
		})
	}
	return defs
}

// executeTool 执行指定工具并返回结果
func (a *AgentChat) executeTool(ctx context.Context, name string, args json.RawMessage) (ToolResult, error) {
	tool, ok := a.tools[name]
	if !ok {
		return ToolResult{ToolName: name, Success: false, Error: fmt.Sprintf("工具不存在: %s", name)}, nil
	}

	var params map[string]interface{}
	if err := json.Unmarshal(args, &params); err != nil {
		return ToolResult{ToolName: name, Success: false, Error: fmt.Sprintf("参数解析失败: %v", err)}, nil
	}

	// 执行前按工具声明的 schema 校验参数：脏类型会让工具内部静默拿到 0，绕过权限守卫。
	if err := ValidateParams(tool.Definition(), params); err != nil {
		return ToolResult{ToolName: name, Success: false, Error: "参数校验失败: " + err.Error()}, nil
	}

	// P1 修复（清单 13 附带）：工具的入参全部来自模型输出（数组长度、下标、边界），
	// 越界 panic 若发生在 executeToolCallsParallel / 流式生产协程这类无 recover 的
	// goroutine 中会直接击穿整个进程（gin Recovery 只覆盖 HTTP handler 协程）。
	// 统一在此 recover，把 panic 转为该次工具调用的错误结果。
	result, err := a.runToolSafely(ctx, tool, name, params)
	if err != nil {
		return ToolResult{ToolName: name, Success: false, Error: err.Error()}, nil
	}
	return *result, nil
}

// runToolSafely 执行工具并把 panic 转成 error。
func (a *AgentChat) runToolSafely(ctx context.Context, tool ToolExecutor, name string, params map[string]interface{}) (*ToolResult, error) {
	var result *ToolResult
	err := func() (err error) {
		defer func() {
			if r := recover(); r != nil {
				err = fmt.Errorf("工具 %s 执行panic: %v", name, r)
			}
		}()
		result, err = tool.Execute(ctx, params)
		return err
	}()
	if err != nil {
		return nil, err
	}
	return result, nil
}

// executeToolCallsParallel 并行执行多个工具调用
func (a *AgentChat) executeToolCallsParallel(ctx context.Context, calls []provider.ToolCall) []ToolResult {
	results := make([]ToolResult, len(calls))
	var wg sync.WaitGroup
	for i, call := range calls {
		wg.Add(1)
		go func(idx int, c provider.ToolCall) {
			defer wg.Done()
			result, _ := a.executeTool(ctx, c.Name, c.Arguments)
			results[idx] = result
		}(i, call)
	}
	wg.Wait()
	return results
}

func (a *AgentChat) Chat(ctx context.Context, question string, history []ChatMessage, images ...string) (*AIAnswer, error) {
	if a.provider == nil {
		return nil, fmt.Errorf("AI服务未启用")
	}

	// P1-7: 前置工作流拦截（斜杠命令 / 模式匹配），命中则直接返回，跳过 LLM 调用
	if a.workflowHandler != nil {
		chatCtx := &ChatContext{Question: question, History: history}
		if result := a.workflowHandler.TryHandle(ctx, chatCtx); result.Handled {
			if result.Err != nil {
				return nil, result.Err
			}
			return &AIAnswer{Answer: result.Response, Sources: []Source{}}, nil
		}
	}

	// 构建消息历史
	messages := make([]provider.Message, 0, len(history)*2+1)
	for _, h := range history {
		messages = append(messages, provider.NewTextMessage("user", h.Question))
		messages = append(messages, provider.NewTextMessage("assistant", h.Answer))
	}
	// 如果有图片，创建多模态消息
	if len(images) > 0 && images[0] != "" {
		messages = append(messages, provider.NewImageMessage("user", question, images[0]))
	} else {
		messages = append(messages, provider.NewTextMessage("user", question))
	}

	// 根据 provider 能力选择调用路径
	if a.provider.Capabilities().FunctionCalling && len(a.tools) > 0 {
		return a.chatWithFunctionCalling(ctx, messages)
	}
	return a.chatWithPromptInjection(ctx, messages)
}

// ChatWithSpace 带空间上下文的对话，支持 Agentic RAG 策略选择
func (a *AgentChat) ChatWithSpace(ctx context.Context, question string, history []ChatMessage, spaceID int, limit int, images ...string) (*AIAnswer, error) {
	startTime := time.Now()
	obsProvider := observability.DefaultProvider()
	ctx, rootSpan := obsProvider.StartSpan(ctx, "agent.chat",
		observability.WithService("agent"),
		observability.WithAttributes(map[string]interface{}{
			"question": question,
			"space_id": spaceID,
			"limit":    limit,
		}),
	)
	defer func() {
		obsProvider.ObserveHistogram(observability.MetricAIRequestDuration, map[string]string{"service": "agent"}, float64(time.Since(startTime).Milliseconds()))
		obsProvider.AddGauge(observability.MetricAIActiveRequests, map[string]string{"service": "agent"}, -1)
	}()
	obsProvider.Inc(observability.MetricAIRequestsTotal, map[string]string{"service": "agent"}, 1)
	obsProvider.AddGauge(observability.MetricAIActiveRequests, map[string]string{"service": "agent"}, 1)

	if a.provider == nil {
		obsProvider.EndSpan(rootSpan, observability.SpanStatusError)
		rootSpan.ErrorMsg = "AI服务未启用"
		return nil, fmt.Errorf("AI服务未启用")
	}

	// P1-7: 前置工作流拦截（斜杠命令 / 模式匹配），命中则直接返回，跳过 LLM 与检索
	if a.workflowHandler != nil {
		chatCtx := &ChatContext{Question: question, History: history, SpaceID: spaceID}
		if result := a.workflowHandler.TryHandle(ctx, chatCtx); result.Handled {
			if result.Err != nil {
				obsProvider.EndSpan(rootSpan, observability.SpanStatusError)
				return nil, result.Err
			}
			obsProvider.EndSpan(rootSpan, observability.SpanStatusOK)
			return &AIAnswer{Answer: result.Response, Sources: []Source{}}, nil
		}
	}

	// P1-5 (§3.2.1): ReAct 循环驱动检索
	// 复杂查询（复杂度 > 50）跳过预检索，让 Agent 在 ReAct 循环中自主决定检索策略
	// 简单查询保持预检索 + ReAct 的现有行为
	complexity := EstimateQueryComplexity(question)
	skipPreRetrieval := complexity > 50 && a.provider.Capabilities().FunctionCalling && len(a.tools) > 0
	if skipPreRetrieval {
		logx.Infof("[AgentRAG] Complex query (complexity=%d), skipping pre-retrieval — Agent will search autonomously via ReAct", complexity)
	}

	// Agentic RAG: 根据查询类型选择检索策略并预检索
	var ragContext string
	if !skipPreRetrieval && a.retriever != nil && spaceID > 0 {
		_, preRetrievalSpan := obsProvider.StartSpan(ctx, "pre_retrieval", observability.WithService("agent"))
		if limit <= 0 {
			limit = 5
		}
		sources, strategyUsed := a.retrieveWithStrategy(ctx, question, spaceID, limit)
		if len(sources) > 0 {
			ragContext = buildCitedContext(sources)
			logx.Infof("[AgentRAG] Pre-retrieved %d sources with strategy %s", len(sources), strategyUsed)
		}
		obsProvider.EndSpan(preRetrievalSpan, observability.SpanStatusOK)
		obsProvider.SetAttribute(preRetrievalSpan, "source_count", len(sources))
		obsProvider.SetAttribute(preRetrievalSpan, "strategy", strategyUsed)
	}

	// 构建系统提示。
	// Prompt Caching 友好：system 只放静态内容；每轮变化的 ragContext 移出 system
	// 改放到当前问题之前的独立 user 块，保住稳定前缀（见流式链路的同款注释）。
	systemPrompt := a.getSystemPrompt()
	// P1-5: 复杂查询时在系统提示中告知 Agent 可以自主检索
	if skipPreRetrieval {
		systemPrompt += "\n\n## 重要提示\n这是一个复杂查询，你可以使用 search_documents 工具进行多轮检索。请先分析需要哪些信息，然后分步检索，每次检索后评估是否需要进一步检索。"
	}

	// P1-5: 上下文预算 + 超预算压缩历史（与流式链路同源）。
	var docs []string
	if ragContext != "" {
		docs = append(docs, ragContext)
	}
	if a.compression != nil {
		cr := a.compression.Compress(ctx, question, docs, history)
		history = cr.History
	}

	// 构建消息历史
	messages := make([]provider.Message, 0, len(history)*2+1)
	for _, h := range history {
		messages = append(messages, provider.NewTextMessage("user", h.Question))
		messages = append(messages, provider.NewTextMessage("assistant", h.Answer))
	}
	if ragContext != "" {
		messages = append(messages, provider.NewTextMessage("user",
			"以下是从知识库中检索到的相关内容，请优先参考这些内容回答问题：\n\n"+ragContext))
	}
	if len(images) > 0 && images[0] != "" {
		messages = append(messages, provider.NewImageMessage("user", question, images[0]))
	} else {
		messages = append(messages, provider.NewTextMessage("user", question))
	}

	if a.provider.Capabilities().FunctionCalling && len(a.tools) > 0 {
		return a.chatWithFunctionCallingWithSystem(ctx, systemPrompt, messages)
	}
	return a.chatWithPromptInjectionWithSystem(ctx, systemPrompt, messages)
}

// chatWithFunctionCalling 使用标准化 Function Calling 路径
func (a *AgentChat) chatWithFunctionCalling(ctx context.Context, messages []provider.Message) (*AIAnswer, error) {
	return a.chatWithFunctionCallingWithSystem(ctx, a.getSystemPrompt(), messages)
}

// chatWithFunctionCallingWithSystem 使用自定义系统提示的 Function Calling 路径
func (a *AgentChat) chatWithFunctionCallingWithSystem(ctx context.Context, systemPrompt string, messages []provider.Message) (*AIAnswer, error) {
	obsProvider := observability.DefaultProvider()
	tools := a.getToolDefinitions()
	var allResults []ToolResult

	for round := 0; round < a.maxRounds; round++ {
		var resp *provider.LLMResponse
		var err error

		_, llmSpan := obsProvider.StartSpan(ctx, "llm_call", observability.WithService("agent"))
		obsProvider.SetAttribute(llmSpan, "round", round)
		resp, err = a.provider.ChatWithTools(ctx, systemPrompt, messages, tools,
			provider.WithMaxTokens(4096),
			provider.WithTemperature(0.3),
		)
		if err != nil {
			obsProvider.EndSpan(llmSpan, observability.SpanStatusError)
			llmSpan.ErrorMsg = err.Error()
			return nil, fmt.Errorf("AI 调用失败: %w", err)
		}
		obsProvider.EndSpan(llmSpan, observability.SpanStatusOK)
		obsProvider.SetAttribute(llmSpan, "has_tool_calls", len(resp.ToolCalls) > 0)
		obsProvider.SetAttribute(llmSpan, "tool_call_count", len(resp.ToolCalls))

		// 检查是否有工具调用
		if len(resp.ToolCalls) > 0 {
			_, toolSpan := obsProvider.StartSpan(ctx, "tool_call", observability.WithService("agent"))
			obsProvider.SetAttribute(toolSpan, "round", round)
			obsProvider.SetAttribute(toolSpan, "tool_count", len(resp.ToolCalls))
			// 并行执行所有工具调用
			results := a.executeToolCallsParallel(ctx, resp.ToolCalls)
			allResults = append(allResults, results...)
			obsProvider.EndSpan(toolSpan, observability.SpanStatusOK)

			// 将 assistant 消息（含 tool_use）加入历史
			messages = append(messages, provider.NewToolCallMessage(resp.Content, resp.ToolCalls))

			// 为每个工具调用添加结果消息
			for i, tc := range resp.ToolCalls {
				resultJSON, _ := json.Marshal(results[i])
				messages = append(messages, provider.NewToolResultMessage(tc.ID, string(resultJSON)))
			}
			continue
		}

		// 无工具调用，返回最终回答
		_, genSpan := obsProvider.StartSpan(ctx, "generation", observability.WithService("agent"))
		result := &AIAnswer{
			Answer:  resp.Content,
			Sources: a.extractSources(allResults),
		}
		obsProvider.EndSpan(genSpan, observability.SpanStatusOK)
		obsProvider.SetAttribute(genSpan, "source_count", len(result.Sources))

		if rootSpan := observability.SpanFromContext(ctx); rootSpan != nil {
			obsProvider.EndSpan(rootSpan, observability.SpanStatusOK)
		}

		return result, nil
	}

	// 超过最大轮次，汇总工具结果
	if len(allResults) > 0 {
		return &AIAnswer{
			Answer:  a.summarizeToolResults(allResults),
			Sources: a.extractSources(allResults),
		}, nil
	}

	return &AIAnswer{Answer: "已达到最大工具调用轮数，未能生成回答。", Sources: nil}, nil
}

// chatWithPromptInjection 降级方案：通过 prompt 注入 + 文本解析实现工具调用
func (a *AgentChat) chatWithPromptInjection(ctx context.Context, messages []provider.Message) (*AIAnswer, error) {
	return a.chatWithPromptInjectionWithSystem(ctx, a.getPromptInjectionSystemPrompt(), messages)
}

// getPromptInjectionSystemPrompt 获取 prompt injection 模式的系统提示
func (a *AgentChat) getPromptInjectionSystemPrompt() string {
	_ = a.buildToolDefinitions() // 预留：未来可用于动态生成工具描述

	return `你是一个知识库AI助手，拥有以下工具能力：

文档操作：
- search_documents: 搜索知识库文档
- create_document: 创建新文档
- move_document: 移动文档
- edit_document: 编辑文档内容（替换章节、追加、插入）
- get_document: 获取指定文档的详细信息和内容
- list_documents: 列出指定空间下的文档（支持平铺列表或树形结构）
- list_spaces: 列出知识库中的所有空间，支持关键词搜索

用户与通信：
- search_user: 搜索系统用户
- send_email: 发送邮件通知

数据与计算：
- calculate: 数学计算（加减乘除、百分比、聚合）
- query_stats: 查询知识库统计数据（文档数、空间数、用户数）
- generate_chart: 生成图表配置（柱状图、折线图、饼图）

信息获取：
- fetch_web: 抓取网页内容
- get_current_time: 获取当前时间和日期

工具类：
- string_manipulate: 字符串处理（长度、大小写、截取、替换、查找、分割、反转）
- json_formatter: JSON格式化（美化、压缩、验证）
- random_number: 随机数生成（整数、浮点数、批量、洗牌、随机选择）

当你需要执行操作时，请使用以下JSON格式之一调用工具：

格式1（标准格式）：
{"tool": "工具名", "params": {"参数名": "参数值"}}

格式2（Function Calling格式）：
{"function": "工具名", "arguments": {"参数名": "参数值"}}

两种格式均可识别，选择你偏好的即可。
如果只需要回答问题，直接回答即可。你可以连续调用多个工具完成复杂任务。
请用中文回答。`
}

// chatWithPromptInjectionWithSystem 使用自定义系统提示的 prompt injection 路径
func (a *AgentChat) chatWithPromptInjectionWithSystem(ctx context.Context, systemPrompt string, messages []provider.Message) (*AIAnswer, error) {
	var allSources []Source
	var toolResults []ToolResult

	for round := 0; round < a.maxRounds; round++ {
		resp, err := a.provider.Chat(ctx, systemPrompt, messages,
			provider.WithMaxTokens(2048), provider.WithTemperature(0.3))
		if err != nil {
			return nil, fmt.Errorf("LLM call error: %w", err)
		}

		answer := resp.Content
		toolCall := a.extractToolCall(answer)
		if toolCall == nil {
			return &AIAnswer{Answer: answer, Sources: allSources}, nil
		}

		tool, ok := a.tools[toolCall.Tool]
		if !ok {
			messages = append(messages,
				provider.NewTextMessage("assistant", answer),
				provider.NewTextMessage("user", fmt.Sprintf("工具 %s 不存在，请使用可用工具或直接回答。", toolCall.Tool)),
			)
			continue
		}

		// 与 Function Calling 路径一致：先校验参数类型/必填，再交给工具执行。
		if verr := ValidateParams(tool.Definition(), toolCall.Params); verr != nil {
			toolResults = append(toolResults, ToolResult{ToolName: toolCall.Tool, Success: false, Error: "参数校验失败: " + verr.Error()})
			messages = append(messages,
				provider.NewTextMessage("assistant", answer),
				provider.NewTextMessage("user", fmt.Sprintf("工具参数校验失败: %v", verr)),
			)
			continue
		}

		result, err := tool.Execute(ctx, toolCall.Params)
		if err != nil {
			logx.Infof("[AgentChat] Tool %s error: %v", toolCall.Tool, err)
			messages = append(messages,
				provider.NewTextMessage("assistant", answer),
				provider.NewTextMessage("user", fmt.Sprintf("工具执行失败: %v", err)),
			)
			continue
		}

		toolResults = append(toolResults, *result)
		messages = append(messages,
			provider.NewTextMessage("assistant", answer),
			provider.NewTextMessage("user", fmt.Sprintf("工具 %s 执行结果: %s", result.ToolName, result.Result)),
		)
	}

	var finalAnswer strings.Builder
	finalAnswer.WriteString("我执行了以下操作：\n\n")
	for i, tr := range toolResults {
		status := "成功"
		if !tr.Success {
			status = "失败"
		}
		finalAnswer.WriteString(fmt.Sprintf("%d. **%s** (%s): %s\n", i+1, tr.ToolName, status, tr.Result))
	}

	return &AIAnswer{Answer: finalAnswer.String(), Sources: allSources}, nil
}

// getSystemPrompt 返回 Function Calling 路径的系统提示
func (a *AgentChat) getSystemPrompt() string {
	return `你是一个知识库AI助手，可以通过工具调用来帮助用户完成文档操作。

你可以使用提供的工具来搜索、创建、移动和编辑文档，以及搜索用户、发送邮件、进行计算、查询统计、生成图表、抓取网页等操作。

请根据用户的问题，合理使用工具来完成任务。如果只需要回答问题，直接回答即可。
你可以连续调用多个工具完成复杂任务。
请用中文回答。`
}

// extractSources 从工具结果中提取文档来源
func (a *AgentChat) extractSources(results []ToolResult) []Source {
	var sources []Source
	for _, r := range results {
		if r.ToolName == "search_documents" && r.Success && r.Result != "" {
			// 从搜索结果中解析来源信息（简化处理）
			sources = append(sources, Source{
				DocumentName: r.ToolName,
				Snippet:      r.Result,
			})
		}
	}
	return sources
}

// summarizeToolResults 汇总多个工具执行结果为文本
func (a *AgentChat) summarizeToolResults(results []ToolResult) string {
	var sb strings.Builder
	sb.WriteString("我执行了以下操作：\n\n")
	for i, tr := range results {
		status := "成功"
		if !tr.Success {
			status = "失败"
		}
		sb.WriteString(fmt.Sprintf("%d. **%s** (%s): %s\n", i+1, tr.ToolName, status, tr.Result))
	}
	return sb.String()
}

func (a *AgentChat) buildToolDefinitions() string {
	var defs []string
	for _, tool := range a.tools {
		d := tool.Definition()
		paramsJSON, _ := json.Marshal(d.Parameters)
		defs = append(defs, fmt.Sprintf("- %s: %s\n  参数: %s", d.Name, d.Description, string(paramsJSON)))
	}
	return strings.Join(defs, "\n\n")
}

type toolCall struct {
	Tool      string                 `json:"tool"`
	Params    map[string]interface{} `json:"params"`
	Function  string                 `json:"function"`
	Arguments json.RawMessage        `json:"arguments"`
}

func (a *AgentChat) extractToolCall(text string) *toolCall {
	start := 0
	for {
		relIdx := strings.Index(text[start:], "{")
		if relIdx == -1 {
			return nil
		}
		pos := start + relIdx

		lastBrace := strings.LastIndex(text, "}")
		if lastBrace <= pos {
			start = pos + 1
			if start >= len(text) {
				return nil
			}
			continue
		}

		candidate := text[pos : lastBrace+1]

		var tc toolCall
		if err := json.Unmarshal([]byte(candidate), &tc); err != nil {
			start = pos + 1
			if start >= len(text) {
				return nil
			}
			continue
		}

		if tc.Tool != "" {
			if _, ok := a.tools[tc.Tool]; ok {
				return &tc
			}
		}

		if tc.Function != "" {
			if _, ok := a.tools[tc.Function]; ok {
				params, err := parseArguments(tc.Arguments)
				if err == nil {
					tc.Tool = tc.Function
					tc.Params = params
					return &tc
				}
			}
		}

		start = pos + 1
		if start >= len(text) {
			return nil
		}
	}
}

func parseArguments(raw json.RawMessage) (map[string]interface{}, error) {
	if raw == nil || len(raw) == 0 {
		return map[string]interface{}{}, nil
	}

	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" {
		return map[string]interface{}{}, nil
	}

	var params map[string]interface{}
	if trimmed[0] == '{' {
		if err := json.Unmarshal(raw, &params); err == nil {
			return params, nil
		}
	}

	var argsStr string
	if err := json.Unmarshal(raw, &argsStr); err == nil {
		if err := json.Unmarshal([]byte(argsStr), &params); err == nil {
			return params, nil
		}
	}

	return nil, fmt.Errorf("failed to parse arguments")
}

// SetHyDERetriever 设置 HyDE 检索器
func (a *AgentChat) SetHyDERetriever(h *HyDERetriever) {
	a.hydeRetriever = h
}

// retrieveWithStrategy 根据查询类型动态选择检索策略。
//
// 所有出口统一走 filterSourcesForCtx：HyDE / Corrective 分支此前直接回吐原始结果，
// 绕过了 FilterSourcesByPermission（RetrieveContextWithRAG 的 default 出口内部有过滤，
// 这两个旁路没有）—— 预检索上下文里会混入调用者无权访问空间的正文片段。
func (a *AgentChat) retrieveWithStrategy(ctx context.Context, question string, spaceID int, limit int) ([]Source, string) {
	if a.adaptiveRetriever == nil {
		sources, _ := a.retriever.RetrieveContextWithRAG(ctx, question, spaceID, limit)
		return a.retriever.filterSourcesForCtx(ctx, sources), "basic_rag"
	}
	strategy := a.adaptiveRetriever.SelectStrategy(ctx, question)
	queryType := a.adaptiveRetriever.ClassifyQuery(ctx, question)
	logx.Infof("[AgentRAG] Strategy: %s, QueryType: %s for: %s", strategy, queryType, question)
	switch strategy {
	case StrategyHyDE:
		if a.hydeRetriever != nil {
			results, err := a.hydeRetriever.RetrieveWithHyDE(ctx, question, spaceID, limit)
			if err == nil && len(results) > 0 {
				sources := make([]Source, 0, len(results))
				for _, r := range results {
					sources = append(sources, Source{DocumentID: r.DocID, DocumentName: r.Name, Score: r.Score})
				}
				return a.retriever.filterSourcesForCtx(ctx, sources), "hyde"
			}
		}
		sources, _ := a.retriever.RetrieveContextWithRAG(ctx, question, spaceID, limit)
		return a.retriever.filterSourcesForCtx(ctx, sources), "basic_rag"
	case StrategyCorrective:
		if a.correctiveRAG != nil {
			a.correctiveRAG.SetQueryType(queryType)
			answer, err := a.correctiveRAG.RetrieveAndAnswer(ctx, question, spaceID, limit)
			if err == nil && answer != nil {
				return a.retriever.filterSourcesForCtx(ctx, answer.Sources), "corrective"
			}
		}
		sources, _ := a.retriever.RetrieveContextWithRAG(ctx, question, spaceID, limit)
		return a.retriever.filterSourcesForCtx(ctx, sources), "basic_rag"
	default:
		sources, _ := a.retriever.RetrieveContextWithRAG(ctx, question, spaceID, limit)
		return a.retriever.filterSourcesForCtx(ctx, sources), string(strategy)
	}
}
