package ai

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/gin-gonic/gin"
	aimod "github.com/listen808/citebase/internal/ai"
	agentpkg "github.com/listen808/citebase/internal/ai/agent"
	provider "github.com/listen808/citebase/internal/ai/provider"
	rag "github.com/listen808/citebase/internal/ai/rag"
	"github.com/listen808/citebase/internal/middleware"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
	config "github.com/listen808/citebase/internal/utils/config"
)

// Agent 问答与工作流编排。

type agentAskRequest struct {
	Question     string            `json:"question" binding:"required"`
	SpaceID      int               `json:"space_id"`
	Limit        int               `json:"limit"`
	History      []rag.ChatMessage `json:"history"`
	EnabledTools []string          `json:"enabled_tools"`
	Images       []string          `json:"images"`
}

func (api *ChatAPI) AgentAsk(c *gin.Context) {
	var req agentAskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "question is required")
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	retriever := m.Retriever()
	if retriever == nil {
		response.ServerErrorKey(c, response.MsgAIRetrieverUnconfigured)
		return
	}

	agentChat := rag.NewAgentChatWithTools(m.PrimaryProvider(), retriever, req.EnabledTools, workspacesvc.NewSpaceService(), m.AdaptiveRetriever(), m.CorrectiveRAG())
	if m.HyDERetriever() != nil {
		agentChat.SetHyDERetriever(m.HyDERetriever())
	}

	agentUserID, _ := middleware.GetUserID(c)
	agentCtx := provider.ContextWithUserID(c.Request.Context(), agentUserID)

	startTime := time.Now()
	var answer *rag.AIAnswer
	var err error
	if req.SpaceID > 0 {
		// 使用 Agentic RAG: 根据查询类型选择检索策略
		if len(req.Images) > 0 {
			answer, err = agentChat.ChatWithSpace(agentCtx, req.Question, req.History, req.SpaceID, req.Limit, req.Images[0])
		} else {
			answer, err = agentChat.ChatWithSpace(agentCtx, req.Question, req.History, req.SpaceID, req.Limit)
		}
	} else if len(req.Images) > 0 {
		answer, err = agentChat.Chat(agentCtx, req.Question, req.History, req.Images[0])
	} else {
		answer, err = agentChat.Chat(agentCtx, req.Question, req.History)
	}
	durationMs := time.Since(startTime).Milliseconds()

	providerName := ""
	modelName := ""
	if p := m.PrimaryProvider(); p != nil {
		providerName = p.Name()
	}
	if cfg := config.AppConfig.AI.Primary; cfg != nil {
		modelName = cfg.Model
	} else if config.AppConfig.AI.Model != "" {
		modelName = config.AppConfig.AI.Model
	}

	sourceCount := 0
	answerText := ""
	if answer != nil {
		sourceCount = len(answer.Sources)
		answerText = answer.Answer
	}

	result := "success"
	reason := ""
	if err != nil {
		result = "failed"
		if err != nil {
			reason = err.Error()
		}
	}

	targetID := ""
	if req.SpaceID > 0 {
		targetID = fmt.Sprintf("%d", req.SpaceID)
	}

	RecordAIAudit(
		c,
		agentUserID,
		"agent_ask",
		targetID,
		req.Question,
		answerText,
		modelName,
		providerName,
		result,
		reason,
		0,
		sourceCount,
		durationMs,
	)

	if err != nil {
		logx.Infof("[ChatAPI] AgentAsk error: %v", err)
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"answer":  answer.Answer,
		"sources": answer.Sources,
	})
}

// orchestrateAgentRequest 多 agent 编排请求。
type orchestrateAgentRequest struct {
	Question string `json:"question" binding:"required"`
	SpaceID  int    `json:"space_id"`
	Mode     string `json:"mode"` // sequential | parallel | router
	// AgentSpecs 可选：自定义 agent 角色与系统提示；为空时使用内置预设（检索+总结）。
	AgentSpecs []struct {
		Name         string `json:"name"`
		Role         string `json:"role"`
		SystemPrompt string `json:"system_prompt"`
	} `json:"agents"`
}

// OrchestrateAgents 多 agent 协作编排入口（P2-4）。
// 默认内置「检索 Agent」+「总结 Agent」顺序流水线；可通过 AgentSpecs 自定义。
func (api *ChatAPI) OrchestrateAgents(c *gin.Context) {
	var req orchestrateAgentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "question is required")
		return
	}

	m := aimod.GetModule()
	p := m.PrimaryProvider()
	if p == nil {
		response.ServerErrorKey(c, response.MsgAIServiceUnconfigured)
		return
	}
	retriever := m.Retriever()
	if retriever == nil {
		response.ServerErrorKey(c, response.MsgAIRetrieverUnconfigured)
		return
	}

	var agents []*agentpkg.Agent
	if len(req.AgentSpecs) > 0 {
		for _, spec := range req.AgentSpecs {
			agents = append(agents, &agentpkg.Agent{
				Name:         spec.Name,
				Role:         spec.Role,
				SystemPrompt: spec.SystemPrompt,
			})
		}
	} else {
		// 内置预设：检索 Agent（带检索工具）→ 总结 Agent。
		searchTool := rag.NewSearchDocumentsTool(retriever)
		agents = []*agentpkg.Agent{
			{Name: "retriever", Role: "信息检索", SystemPrompt: "你是检索专家，基于知识库检索与工具获取与问题相关的事实。", Tools: []rag.ToolExecutor{searchTool}},
			{Name: "summarizer", Role: "综合分析", SystemPrompt: "你是分析总结专家，基于前面检索到的信息，给出结构化、准确的回答。"},
		}
	}

	mode := agentpkg.Mode(req.Mode)
	orch := agentpkg.NewOrchestrator(p, mode, agents)

	startTime := time.Now()
	results, err := orch.Run(userCtx(c), req.Question, nil)
	durationMs := time.Since(startTime).Milliseconds()
	if err != nil {
		logx.Infof("[ChatAPI] OrchestrateAgents error: %v", err)
		response.ServerErrorSafe(c, err)
		return
	}

	answer := agentpkg.Aggregate(results)
	response.Success(c, gin.H{
		"mode":       string(mode),
		"answer":     answer,
		"agents":     results,
		"elapsed_ms": durationMs,
	})
}

// buildScopedRetriever 构造按调用方权限范围限定的检索函数。
// 空间范围取请求体里的 space_id；检索结果再按 ctx 中的用户身份做权限过滤。
func buildScopedRetriever(c *gin.Context, inputs map[string]interface{}) rag.RetrievalFunc {
	m := aimod.GetModule()
	r := m.Retriever()
	if r == nil {
		return nil
	}

	spaceID := 0
	if inputs != nil {
		switch v := inputs["space_id"].(type) {
		case float64:
			spaceID = int(v)
		case int:
			spaceID = v
		case string:
			spaceID, _ = strconv.Atoi(v)
		}
	}

	return func(ctx context.Context, query string, topK int) ([]rag.RetrievedSnippet, error) {
		if topK <= 0 || topK > 20 {
			topK = 5
		}
		sources, err := r.RetrieveScoped(ctx, query, spaceID, topK)
		if err != nil {
			return nil, err
		}
		out := make([]rag.RetrievedSnippet, 0, len(sources))
		for _, s := range sources {
			out = append(out, rag.RetrievedSnippet{
				DocumentID: s.DocumentID,
				Title:      s.DocumentName,
				Content:    s.Snippet,
				Score:      s.Score,
			})
		}
		return out, nil
	}
}

func (api *ChatAPI) ListWorkflows(c *gin.Context) {
	engine := rag.GetWorkflowEngine()
	workflows := engine.ListWorkflows()
	response.Success(c, gin.H{"workflows": workflows})
}

func (api *ChatAPI) ExecuteWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	var req struct {
		Inputs map[string]interface{} `json:"inputs"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Inputs = make(map[string]interface{})
	}

	// 检索函数按调用方身份与空间范围注入，供 NodeTypeRetrieval 节点使用。
	// 不注入时检索节点会显式返回"无来源"，而不是让 LLM 凭空生成研究报告。
	retrieve := buildScopedRetriever(c, req.Inputs)

	startTime := time.Now()
	engine := rag.GetWorkflowEngine()
	exec, err := engine.ExecuteWithRetriever(userCtx(c), workflowID, req.Inputs, retrieve)
	durationMs := time.Since(startTime).Milliseconds()

	userID, _ := middleware.GetUserID(c)

	m := aimod.GetModule()
	providerName := ""
	modelName := ""
	if p := m.PrimaryProvider(); p != nil {
		providerName = p.Name()
	}
	if cfg := config.AppConfig.AI.Primary; cfg != nil {
		modelName = cfg.Model
	} else if config.AppConfig.AI.Model != "" {
		modelName = config.AppConfig.AI.Model
	}

	resultStr := "success"
	reasonStr := ""
	if err != nil {
		resultStr = "failed"
		reasonStr = err.Error()
	}

	questionText := fmt.Sprintf("工作流: %s", workflowID)
	answerText := ""
	if exec != nil {
		answerText = fmt.Sprintf("工作流执行完成, 状态: %s", exec.Status)
	}

	RecordAIAudit(c, userID, "workflow", workflowID, questionText, answerText, modelName, providerName, resultStr, reasonStr, 0, 0, durationMs)

	if err != nil {
		response.ServerError(c, fmt.Sprintf("工作流执行失败: %v", err))
		return
	}

	response.Success(c, gin.H{"execution": exec})
}

func (api *ChatAPI) DeleteWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	engine := rag.GetWorkflowEngine()
	if err := engine.DeleteWorkflow(workflowID); err != nil {
		response.ServerError(c, fmt.Sprintf("删除工作流失败: %v", err))
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (api *ChatAPI) StopWorkflow(c *gin.Context) {
	execID := c.Param("execId")
	engine := rag.GetWorkflowEngine()
	if err := engine.StopExecution(execID); err != nil {
		response.ServerError(c, fmt.Sprintf("停止工作流失败: %v", err))
		return
	}
	response.Success(c, gin.H{"stopped": true})
}

func (api *ChatAPI) CreateWorkflow(c *gin.Context) {
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Type        string `json:"type"`
		CronExpr    string `json:"cron_expr"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKey(c, response.MsgParameterError)
		return
	}
	if req.Name == "" {
		response.BadRequestKey(c, response.MsgWorkflowNameEmpty)
		return
	}
	engine := rag.GetWorkflowEngine()
	wf := &rag.WorkflowDefinition{
		ID:          fmt.Sprintf("wf_%d", time.Now().UnixNano()),
		Name:        req.Name,
		Description: req.Description,
		Nodes:       []rag.WorkflowNode{},
		Edges:       []rag.WorkflowEdge{},
	}
	wfType := req.Type
	if wfType == "" {
		wfType = "quality"
	}
	startNode := rag.WorkflowNode{
		ID:        "start",
		Type:      rag.NodeTypeStart,
		Name:      "开始",
		NextNodes: []string{"quality_check"},
	}
	llmNode := rag.WorkflowNode{
		ID:   "quality_check",
		Type: rag.NodeTypeLLM,
		Name: "质量巡检",
		Config: map[string]interface{}{
			"prompt": fmt.Sprintf("请对文档执行%s类型的质量分析，找出内容缺陷、格式问题和需要改进的地方。", wfType),
			"type":   wfType,
		},
		NextNodes: []string{"end"},
	}
	endNode := rag.WorkflowNode{
		ID:   "end",
		Type: rag.NodeTypeEnd,
		Name: "结束",
	}
	wf.Nodes = []rag.WorkflowNode{startNode, llmNode, endNode}
	engine.RegisterWorkflow(wf)
	response.Success(c, gin.H{"workflow": wf})
}

// GenerateWorkflow 自然语言 → 工作流 DAG（W6）。
//
// 模型产出的定义先过引擎侧结构校验（validateGeneratedWorkflow），
// 通过后才以草稿（Active=false）注册。管理员在界面确认无误后再激活，
// 避免一次描述错误直接变成一次无界 LLM 执行。
func (api *ChatAPI) GenerateWorkflow(c *gin.Context) {
	var req struct {
		Description string `json:"description"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKey(c, response.MsgParameterError)
		return
	}
	if strings.TrimSpace(req.Description) == "" {
		response.BadRequestKey(c, response.MsgParameterError)
		return
	}

	startTime := time.Now()
	engine := rag.GetWorkflowEngine()
	wf, err := engine.GenerateWorkflow(userCtx(c), req.Description)
	durationMs := time.Since(startTime).Milliseconds()

	userID, _ := middleware.GetUserID(c)
	m := aimod.GetModule()
	providerName := ""
	modelName := ""
	if p := m.PrimaryProvider(); p != nil {
		providerName = p.Name()
	}
	if cfg := config.AppConfig.AI.Primary; cfg != nil {
		modelName = cfg.Model
	} else if config.AppConfig.AI.Model != "" {
		modelName = config.AppConfig.AI.Model
	}

	resultStr := "success"
	reasonStr := ""
	if err != nil {
		resultStr = "failed"
		reasonStr = err.Error()
	}
	questionText := fmt.Sprintf("AI 生成工作流: %s", truncateForAudit(req.Description, 80))
	RecordAIAudit(c, userID, "workflow", "generate", questionText, "", modelName, providerName, resultStr, reasonStr, 0, 0, durationMs)

	if err != nil {
		response.ServerError(c, fmt.Sprintf("生成工作流失败: %v", err))
		return
	}

	// 校验已通过，注册为草稿。
	engine.RegisterWorkflow(wf)
	response.Success(c, gin.H{"workflow": wf, "active": false, "message": "工作流已生成为草稿，请在确认无误后启用"})
}

// truncateForAudit 把审计用的问题文本截到 n 个 rune，避免超长描述灌进审计表。
func truncateForAudit(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func (api *ChatAPI) ToggleWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	var req struct {
		Active bool `json:"active"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKey(c, response.MsgActiveFieldRequired)
		return
	}
	engine := rag.GetWorkflowEngine()
	if err := engine.SetWorkflowActive(workflowID, req.Active); err != nil {
		response.NotFoundKey(c, response.MsgWorkflowNotFound)
		return
	}
	wf := engine.GetWorkflow(workflowID)
	response.Success(c, gin.H{"workflow": wf, "active": req.Active})
}

// UpdateWorkflow 更新工作流定义（节点/边等）
func (api *ChatAPI) UpdateWorkflow(c *gin.Context) {
	workflowID := c.Param("id")
	var req struct {
		Name        string             `json:"name"`
		Description string             `json:"description"`
		Nodes       []rag.WorkflowNode `json:"nodes"`
		Edges       []rag.WorkflowEdge `json:"edges"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKey(c, response.MsgParameterError)
		return
	}
	engine := rag.GetWorkflowEngine()
	updated := &rag.WorkflowDefinition{
		Name:        req.Name,
		Description: req.Description,
		Nodes:       req.Nodes,
		Edges:       req.Edges,
	}
	wf, err := engine.UpdateWorkflow(workflowID, updated)
	if err != nil {
		response.NotFoundKey(c, response.MsgWorkflowNotFound)
		return
	}
	response.Success(c, gin.H{"workflow": wf, "message": "工作流已更新"})
}
