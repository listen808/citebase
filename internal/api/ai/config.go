package ai

import (
	"context"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	aimod "github.com/listen808/citebase/internal/ai"
	analyzer "github.com/listen808/citebase/internal/ai/analyzer"
	mcp "github.com/listen808/citebase/internal/ai/mcp"
	"github.com/listen808/citebase/internal/ai/provider"
	rag "github.com/listen808/citebase/internal/ai/rag"
	"github.com/listen808/citebase/internal/middleware"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/response"
	aiSvc "github.com/listen808/citebase/internal/service/ai"
	config "github.com/listen808/citebase/internal/utils/config"
	"github.com/spf13/viper"
)

type AIConfigAPI struct {
	auditSvc *aiSvc.AuditService
}

func NewAIConfigAPI() *AIConfigAPI {
	return &AIConfigAPI{
		auditSvc: aiSvc.NewAuditService(airepo.NewAIAuditLogRepository()),
	}
}

type updateAIConfigRequest struct {
	Provider       string  `json:"provider_type"`
	BaseURL        string  `json:"base_url"`
	APIKey         string  `json:"api_key"`
	Model          string  `json:"model"`
	FastModel      string  `json:"fast_model"`
	ReasoningModel string  `json:"reasoning_model"`
	Temperature    float64 `json:"temperature"`
	MaxTokens      int     `json:"max_tokens"`
}

type updateAnalyzerConfigRequest struct {
	BatchSize       int `json:"batch_size" binding:"required,min=1,max=50"`
	MaxConcurrent   int `json:"max_concurrent" binding:"required,min=1,max=10"`
	RateLimitRPM    int `json:"rate_limit_rpm" binding:"required,min=1,max=1000"`
	DocMaxChars     int `json:"doc_max_chars" binding:"required,min=1000,max=100000"`
	ContextMaxChars int `json:"context_max_chars" binding:"required,min=4000,max=200000"`
}

type ragConfigRequest struct {
	VectorWeight        *float64 `json:"vector_weight,omitempty"`
	KeywordWeight       *float64 `json:"keyword_weight,omitempty"`
	SimilarityThreshold *float64 `json:"similarity_threshold,omitempty"`
	SearchTopK          *int     `json:"search_top_k,omitempty"`
	MaxContextChars     *int     `json:"max_context_chars,omitempty"`
	EnableRerank        *bool    `json:"enable_rerank,omitempty"`
	EnableQueryRewrite  *bool    `json:"enable_query_rewrite,omitempty"`
	EnableSubQuestion   *bool    `json:"enable_sub_question,omitempty"`
	RerankEndpoint      *string  `json:"rerank_endpoint,omitempty"`
	RerankApiKey        *string  `json:"rerank_api_key,omitempty"`
}

type ragConfigResponse struct {
	Config ragConfigRequest `json:"config"`
}

func (api *AIConfigAPI) GetAIConfig(c *gin.Context) {
	m := aimod.GetModule()
	cfg := &config.AppConfig.AI

	provider := cfg.Provider
	baseURL := cfg.BaseURL
	apiKey := cfg.APIKey
	model := cfg.Model
	maxTokens := cfg.MaxTokens
	temperature := cfg.Temperature

	if cfg.Primary != nil {
		provider = cfg.Primary.Provider
		baseURL = cfg.Primary.BaseURL
		apiKey = cfg.Primary.APIKey
		model = cfg.Primary.Model
		maxTokens = cfg.Primary.MaxTokens
		temperature = cfg.Primary.Temperature
	}

	// fast/reasoning 层信息
	var fastModel, fastProvider, fastBaseURL string
	if cfg.Fast != nil {
		fastModel = cfg.Fast.Model
		fastProvider = cfg.Fast.Provider
		fastBaseURL = cfg.Fast.BaseURL
	}
	var reasoningModel, reasoningProvider, reasoningBaseURL string
	if cfg.Reasoning != nil {
		reasoningModel = cfg.Reasoning.Model
		reasoningProvider = cfg.Reasoning.Provider
		reasoningBaseURL = cfg.Reasoning.BaseURL
	}

	response.Success(c, gin.H{
		"enabled":            m.Enabled(),
		"ai_space_name":      m.AISpaceName(),
		"provider":           provider,
		"base_url":           baseURL,
		"api_key":            maskAPIKey(apiKey),
		"model":              model,
		"max_tokens":         maxTokens,
		"temperature":        temperature,
		"fast_model":         fastModel,
		"fast_provider":      fastProvider,
		"fast_base_url":      fastBaseURL,
		"reasoning_model":    reasoningModel,
		"reasoning_provider": reasoningProvider,
		"reasoning_base_url": reasoningBaseURL,
	})
}

func maskAPIKey(key string) string {
	if key == "" {
		return ""
	}
	if len(key) <= 8 {
		return "****"
	}
	return key[:4] + strings.Repeat("*", len(key)-8) + key[len(key)-4:]
}

// ensureFastConfig 确保 AI.Fast 不为 nil，若为 nil 则初始化并同步到 viper
func ensureFastConfig() {
	if config.AppConfig.AI.Fast == nil {
		config.AppConfig.AI.Fast = &config.AIProviderConfig{}
		viper.Set("ai.fast", map[string]interface{}{})
	}
}

// ensureReasoningConfig 确保 AI.Reasoning 不为 nil，若为 nil 则初始化并同步到 viper
func ensureReasoningConfig() {
	if config.AppConfig.AI.Reasoning == nil {
		config.AppConfig.AI.Reasoning = &config.AIProviderConfig{}
		viper.Set("ai.reasoning", map[string]interface{}{})
	}
}

// truncateStr 截断字符串到指定长度
func truncateStr(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}

func (api *AIConfigAPI) UpdateAIConfig(c *gin.Context) {
	var req updateAIConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}

	validProviders := map[string]bool{
		"openai": true, "deepseek": true, "qwen": true,
		"zhipu": true, "ernie": true, "gemini": true,
		"ollama": true, "anthropic": true,
	}
	if req.Provider != "" && !validProviders[req.Provider] {
		response.BadRequest(c, fmt.Sprintf("unsupported provider: %s", req.Provider))
		return
	}
	if req.Temperature < 0 || req.Temperature > 2 {
		response.BadRequest(c, "temperature must be between 0 and 2")
		return
	}
	if req.MaxTokens < 0 || req.MaxTokens > 128000 {
		response.BadRequest(c, "max_tokens must be between 0 and 128000")
		return
	}

	if req.Provider != "" {
		viper.Set("ai.primary.provider", req.Provider)
		if config.AppConfig.AI.Primary != nil {
			config.AppConfig.AI.Primary.Provider = req.Provider
		}
	}
	if req.BaseURL != "" {
		viper.Set("ai.primary.base_url", req.BaseURL)
		if config.AppConfig.AI.Primary != nil {
			config.AppConfig.AI.Primary.BaseURL = req.BaseURL
		}
	}
	if req.APIKey != "" {
		viper.Set("ai.primary.api_key", req.APIKey)
		if config.AppConfig.AI.Primary != nil {
			config.AppConfig.AI.Primary.APIKey = req.APIKey
		}
	}
	if req.Model != "" {
		viper.Set("ai.primary.model", req.Model)
		if config.AppConfig.AI.Primary != nil {
			config.AppConfig.AI.Primary.Model = req.Model
		}
	}
	if req.Temperature > 0 {
		viper.Set("ai.primary.temperature", req.Temperature)
		if config.AppConfig.AI.Primary != nil {
			config.AppConfig.AI.Primary.Temperature = req.Temperature
		}
	}
	if req.MaxTokens > 0 {
		viper.Set("ai.primary.max_tokens", req.MaxTokens)
		if config.AppConfig.AI.Primary != nil {
			config.AppConfig.AI.Primary.MaxTokens = req.MaxTokens
		}
	}

	// fast 层：继承 primary 的 provider/base_url/api_key，仅覆盖 model
	if req.FastModel != "" {
		ensureFastConfig()
		viper.Set("ai.fast.model", req.FastModel)
		config.AppConfig.AI.Fast.Model = req.FastModel
		// 如果 fast 层没有自己的 provider/base_url/api_key，从 primary 继承
		if config.AppConfig.AI.Fast.Provider == "" {
			fastProv := req.Provider
			if fastProv == "" && config.AppConfig.AI.Primary != nil {
				fastProv = config.AppConfig.AI.Primary.Provider
			}
			viper.Set("ai.fast.provider", fastProv)
			config.AppConfig.AI.Fast.Provider = fastProv
		}
		if config.AppConfig.AI.Fast.BaseURL == "" {
			fastURL := req.BaseURL
			if fastURL == "" && config.AppConfig.AI.Primary != nil {
				fastURL = config.AppConfig.AI.Primary.BaseURL
			}
			viper.Set("ai.fast.base_url", fastURL)
			config.AppConfig.AI.Fast.BaseURL = fastURL
		}
		if config.AppConfig.AI.Fast.APIKey == "" {
			fastKey := req.APIKey
			if fastKey == "" && config.AppConfig.AI.Primary != nil {
				fastKey = config.AppConfig.AI.Primary.APIKey
			}
			viper.Set("ai.fast.api_key", fastKey)
			config.AppConfig.AI.Fast.APIKey = fastKey
		}
	}

	// reasoning 层：继承 primary 的 provider/base_url/api_key，仅覆盖 model
	if req.ReasoningModel != "" {
		ensureReasoningConfig()
		viper.Set("ai.reasoning.model", req.ReasoningModel)
		config.AppConfig.AI.Reasoning.Model = req.ReasoningModel
		if config.AppConfig.AI.Reasoning.Provider == "" {
			reasonProv := req.Provider
			if reasonProv == "" && config.AppConfig.AI.Primary != nil {
				reasonProv = config.AppConfig.AI.Primary.Provider
			}
			viper.Set("ai.reasoning.provider", reasonProv)
			config.AppConfig.AI.Reasoning.Provider = reasonProv
		}
		if config.AppConfig.AI.Reasoning.BaseURL == "" {
			reasonURL := req.BaseURL
			if reasonURL == "" && config.AppConfig.AI.Primary != nil {
				reasonURL = config.AppConfig.AI.Primary.BaseURL
			}
			viper.Set("ai.reasoning.base_url", reasonURL)
			config.AppConfig.AI.Reasoning.BaseURL = reasonURL
		}
		if config.AppConfig.AI.Reasoning.APIKey == "" {
			reasonKey := req.APIKey
			if reasonKey == "" && config.AppConfig.AI.Primary != nil {
				reasonKey = config.AppConfig.AI.Primary.APIKey
			}
			viper.Set("ai.reasoning.api_key", reasonKey)
			config.AppConfig.AI.Reasoning.APIKey = reasonKey
		}
	}

	if err := viper.WriteConfig(); err != nil {
		if cfgFile := viper.ConfigFileUsed(); cfgFile != "" {
			if writeErr := writeConfigToFile(cfgFile); writeErr != nil {
				logx.Infof("[AIConfigAPI] WriteConfig failed: %v, manual write also failed: %v", err, writeErr)
				response.ServerError(c, fmt.Sprintf("配置写入失败: %v", err))
				return
			}
		} else {
			logx.Infof("[AIConfigAPI] WriteConfig failed: %v", err)
			response.ServerError(c, fmt.Sprintf("配置写入失败: %v", err))
			return
		}
	}

	aiCfg := aimod.BuildAIConfig(
		config.AppConfig.AI.Enabled,
		config.AppConfig.AI.Provider,
		config.AppConfig.AI.BaseURL,
		config.AppConfig.AI.APIKey,
		config.AppConfig.AI.Model,
		config.AppConfig.AI.MaxTokens,
		config.AppConfig.AI.Temperature,
		config.AppConfig.AI.EmbeddingURL,
		config.AppConfig.AI.EmbeddingKey,
		config.AppConfig.AI.EmbeddingModel,
	)

	if cfg := config.AppConfig.AI.Primary; cfg != nil {
		aiCfg.Primary = cfg
	}
	if cfg := config.AppConfig.AI.Fast; cfg != nil {
		aiCfg.Fast = cfg
	}
	if cfg := config.AppConfig.AI.Reasoning; cfg != nil {
		aiCfg.Reasoning = cfg
	}
	aimod.InitModule(aiCfg)

	m := aimod.GetModule()
	primary := config.AppConfig.AI.Primary
	providerName := config.AppConfig.AI.Provider
	modelName := config.AppConfig.AI.Model
	maxTokens := config.AppConfig.AI.MaxTokens
	temperature := config.AppConfig.AI.Temperature
	if primary != nil {
		providerName = primary.Provider
		modelName = primary.Model
		maxTokens = primary.MaxTokens
		temperature = primary.Temperature
	}

	response.Success(c, gin.H{
		"enabled":        m.Enabled(),
		"provider":       providerName,
		"model":          modelName,
		"temperature":    temperature,
		"max_tokens":     maxTokens,
		"restart_needed": false,
	})
}

func writeConfigToFile(path string) error {
	return viper.WriteConfigAs(path)
}

// ListAIModels 按当前 primary 配置调上游 GET /models，把模型清单带回前端做自动带入。
// 只发现不保存：换模型生效走 /ai/config/update。
func (api *AIConfigAPI) ListAIModels(c *gin.Context) {
	m := aimod.GetModule()
	if !m.Enabled() {
		response.Success(c, gin.H{"success": false, "models": []string{}, "message": "AI 服务未启用"})
		return
	}
	p := m.PrimaryProvider()
	if p == nil {
		response.Success(c, gin.H{"success": false, "models": []string{}, "message": "AI Provider 未配置"})
		return
	}
	inner := p
	if pp, ok := inner.(*provider.PooledProvider); ok {
		inner = pp.Inner()
	}
	lister, ok := inner.(provider.ModelLister)
	if !ok {
		response.Success(c, gin.H{"success": false, "models": []string{}, "message": "当前 Provider 类型不支持模型发现"})
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
	defer cancel()
	models, err := lister.ListModels(ctx)
	if err != nil {
		response.Success(c, gin.H{"success": false, "models": []string{}, "message": fmt.Sprintf("模型发现失败: %v", err)})
		return
	}
	response.Success(c, gin.H{"success": true, "models": models, "message": fmt.Sprintf("发现 %d 个模型", len(models))})
}

func (api *AIConfigAPI) TestAIConnection(c *gin.Context) {
	m := aimod.GetModule()
	if !m.Enabled() {
		response.Success(c, gin.H{"success": false, "message": "AI 服务未启用"})
		return
	}
	p := m.PrimaryProvider()
	if p == nil {
		response.Success(c, gin.H{"success": false, "message": "AI Provider 未配置"})
		return
	}
	// 真正发送一个轻量级请求验证连通性
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	resp, err := p.Chat(ctx, "You are a test assistant.", []provider.Message{
		provider.NewTextMessage("user", "hi"),
	}, provider.WithMaxTokens(5))
	if err != nil {
		response.Success(c, gin.H{
			"success": false,
			"message": fmt.Sprintf("连接失败: %v", err),
		})
		return
	}
	response.Success(c, gin.H{
		"success": true,
		"message": fmt.Sprintf("连接成功，Provider: %s，模型响应: %s", p.Name(), truncateStr(resp.Content, 50)),
	})
}

func (api *AIConfigAPI) UpdateAISchedule(c *gin.Context) {
	var req struct {
		Schedule string `json:"schedule" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "schedule is required")
		return
	}

	m := aimod.GetModule()
	cs := m.CronScheduler()
	if cs == nil {
		response.ServerErrorKey(c, response.MsgAICronUnconfigured)
		return
	}

	if err := cs.UpdateSchedule(req.Schedule); err != nil {
		response.ServerError(c, fmt.Sprintf("更新调度失败: %v", err))
		return
	}

	response.Success(c, gin.H{"schedule": req.Schedule})
}

func (api *AIConfigAPI) GetAISchedule(c *gin.Context) {
	m := aimod.GetModule()
	cs := m.CronScheduler()
	if cs == nil {
		response.Success(c, gin.H{"schedule": "", "running": false})
		return
	}
	response.Success(c, gin.H{
		"schedule": cs.GetSchedule(),
		"running":  cs.IsRunning(),
	})
}

func (api *AIConfigAPI) GetAnalyzerConfig(c *gin.Context) {
	m := aimod.GetModule()
	if !m.Enabled() {
		response.BadRequestKey(c, response.MsgAIServiceDisabled)
		return
	}
	cfg := m.GetAnalyzerConfig()
	response.Success(c, cfg)
}

func (api *AIConfigAPI) UpdateAnalyzerConfig(c *gin.Context) {
	m := aimod.GetModule()
	if !m.Enabled() {
		response.BadRequestKey(c, response.MsgAIServiceDisabled)
		return
	}

	var req updateAnalyzerConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequestKeyDetail(c, response.MsgParameterError, err.Error())
		return
	}

	cfg := &analyzer.AnalyzerConfig{
		BatchSize:       req.BatchSize,
		MaxConcurrent:   req.MaxConcurrent,
		RateLimitRPM:    req.RateLimitRPM,
		DocMaxChars:     req.DocMaxChars,
		ContextMaxChars: req.ContextMaxChars,
	}
	m.UpdateAnalyzerConfig(cfg)
	response.Success(c, cfg)
}

func (api *AIConfigAPI) GetRAGConfig(c *gin.Context) {
	response.Success(c, gin.H{
		"config": gin.H{
			"vector_weight":        config.AppConfig.AI.VectorWeight,
			"keyword_weight":       config.AppConfig.AI.KeywordWeight,
			"similarity_threshold": config.AppConfig.AI.SimilarityThreshold,
			"search_top_k":         config.AppConfig.AI.SearchTopK,
			"max_context_chars":    config.AppConfig.AI.MaxContextChars,
			"enable_rerank":        config.AppConfig.AI.RerankEnabled,
			"enable_query_rewrite": true,
			"enable_sub_question":  true,
			"rerank_endpoint":      config.AppConfig.AI.RerankEndpoint,
			"rerank_api_key":       config.AppConfig.AI.RerankApiKey,
		},
	})
}

func (api *AIConfigAPI) UpdateRAGConfig(c *gin.Context) {
	var req ragConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request")
		return
	}

	if req.VectorWeight != nil {
		viper.Set("ai.vector_weight", *req.VectorWeight)
		config.AppConfig.AI.VectorWeight = *req.VectorWeight
	}
	if req.KeywordWeight != nil {
		viper.Set("ai.keyword_weight", *req.KeywordWeight)
		config.AppConfig.AI.KeywordWeight = *req.KeywordWeight
	}
	if req.SimilarityThreshold != nil {
		viper.Set("ai.similarity_threshold", *req.SimilarityThreshold)
		config.AppConfig.AI.SimilarityThreshold = *req.SimilarityThreshold
	}
	if req.SearchTopK != nil {
		viper.Set("ai.search_top_k", *req.SearchTopK)
		config.AppConfig.AI.SearchTopK = *req.SearchTopK
	}
	if req.MaxContextChars != nil {
		viper.Set("ai.max_context_chars", *req.MaxContextChars)
		config.AppConfig.AI.MaxContextChars = *req.MaxContextChars
	}
	if req.EnableRerank != nil {
		viper.Set("ai.rerank_enabled", *req.EnableRerank)
		config.AppConfig.AI.RerankEnabled = *req.EnableRerank
	}
	if req.RerankEndpoint != nil {
		viper.Set("ai.rerank_endpoint", *req.RerankEndpoint)
		config.AppConfig.AI.RerankEndpoint = *req.RerankEndpoint
	}
	if req.RerankApiKey != nil {
		viper.Set("ai.rerank_api_key", *req.RerankApiKey)
		config.AppConfig.AI.RerankApiKey = *req.RerankApiKey
	}

	if err := viper.WriteConfig(); err != nil {
		logx.Infof("[AIConfigAPI] Failed to write config: %v", err)
	}

	api.GetRAGConfig(c)
}

func (api *AIConfigAPI) TestRAGRetrieval(c *gin.Context) {
	var req askRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "question is required")
		return
	}

	if req.Limit <= 0 {
		req.Limit = 5
	}
	if req.Limit > 20 {
		req.Limit = 20
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	retriever := m.Retriever()
	if retriever == nil {
		response.ServerErrorKey(c, response.MsgAIServiceUnconfigured)
		return
	}

	startTime := time.Now()
	_, debug, err := retriever.AskWithDebug(userCtx(c), req.Question, req.SpaceID, req.Limit)
	durationMs := time.Since(startTime).Milliseconds()

	userID, _ := middleware.GetUserID(c)

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
	if debug != nil && debug.FinalSources != nil {
		sourceCount = len(debug.FinalSources)
	}

	targetID := ""
	if req.SpaceID > 0 {
		targetID = fmt.Sprintf("%d", req.SpaceID)
	}

	result := "success"
	reason := ""
	if err != nil {
		result = "failed"
		reason = err.Error()
	}

	RecordAIAudit(c, userID, "test_rag", targetID, req.Question, fmt.Sprintf("RAG测试, 来源: %d", sourceCount), modelName, providerName, result, reason, 0, sourceCount, durationMs)

	if err != nil {
		logx.Infof("[AIConfigAPI] TestRAGRetrieval error: %v", err)
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{"debug": debug})
}

func (api *AIConfigAPI) RAGASEvaluate(c *gin.Context) {
	var req struct {
		Question    string   `json:"question" binding:"required"`
		Answer      string   `json:"answer" binding:"required"`
		Contexts    []string `json:"contexts"`
		GroundTruth string   `json:"ground_truth"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "question and answer are required")
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	startTime := time.Now()
	evaluator := rag.GetRAGASEvaluator()
	if evaluator == nil {
		response.ServerErrorKey(c, response.MsgAIRAGASUninitialized)
		return
	}

	result, err := evaluator.Evaluate(req.Question, req.Answer, req.Contexts, req.GroundTruth)
	durationMs := time.Since(startTime).Milliseconds()

	userID, _ := middleware.GetUserID(c)

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

	targetID := ""
	resultStr := "success"
	reasonStr := ""
	if err != nil {
		resultStr = "failed"
		reasonStr = err.Error()
	}

	answerText := ""
	if result != nil {
		answerText = fmt.Sprintf("评估完成, 平均分数: %.2f", result.AvgScore)
	}

	RecordAIAudit(c, userID, "ragas_eval", targetID, req.Question, answerText, modelName, providerName, resultStr, reasonStr, 0, len(req.Contexts), durationMs)

	if err != nil {
		response.ServerError(c, fmt.Sprintf("评估失败: %v", err))
		return
	}

	response.Success(c, gin.H{"result": result})
}

func (api *AIConfigAPI) RAGASResults(c *gin.Context) {
	evaluator := rag.GetRAGASEvaluator()
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	results, total, err := evaluator.GetHistory(limit, offset)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"results": results, "total": total})
}

func (api *AIConfigAPI) RAGASStats(c *gin.Context) {
	evaluator := rag.GetRAGASEvaluator()
	// GetStats 优先聚合 aw_ai_ragas_eval，无库时才回落内存
	response.Success(c, evaluator.GetStats())
}

func (api *AIConfigAPI) RAGASDeleteResult(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}
	evaluator := rag.GetRAGASEvaluator()
	if err := evaluator.DeleteResult(id); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"deleted": true})
}

func (api *AIConfigAPI) ListAIAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	action := c.Query("action")

	roleID, _ := c.Get("role_id")
	isAdmin := false
	if roleID != nil {
		if rid, ok := roleID.(int); ok && (rid == 1 || rid == 2) {
			isAdmin = true
		}
	}

	userID, _ := middleware.GetUserID(c)

	logs, total, err := api.auditSvc.ListAuditLogs(action, limit, offset, userID, isAdmin)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"logs":   logs,
		"total":  total,
		"limit":  limit,
		"offset": offset,
	})
}

func (api *AIConfigAPI) ListMCPServers(c *gin.Context) {
	m := aimod.GetModule()
	client := m.MCPClient()
	if client == nil {
		response.Success(c, gin.H{"servers": []interface{}{}})
		return
	}

	servers := client.ListServers()
	result := make([]gin.H, 0, len(servers))
	for _, s := range servers {
		result = append(result, gin.H{
			"name":       s.Config.Name,
			"transport":  s.Config.Transport,
			"command":    s.Config.Command,
			"args":       s.Config.Args,
			"endpoint":   s.Config.Endpoint,
			"connected":  s.Connected,
			"error":      s.Error,
			"tool_count": len(s.Tools),
			"tools":      s.Tools,
		})
	}
	response.Success(c, gin.H{"servers": result})
}

func (api *AIConfigAPI) AddMCPServer(c *gin.Context) {
	var req mcp.ServerConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "invalid request body")
		return
	}
	if req.Name == "" {
		response.BadRequest(c, "name is required")
		return
	}
	if req.Transport == "" {
		req.Transport = "stdio"
	}
	if req.Transport != "stdio" && req.Transport != "sse" {
		response.BadRequest(c, "transport must be 'stdio' or 'sse'")
		return
	}
	if req.Transport == "stdio" && req.Command == "" {
		response.BadRequest(c, "command is required for stdio transport")
		return
	}

	m := aimod.GetModule()
	client := m.MCPClient()
	if client == nil {
		client = mcp.NewClient()
		m.SetMCPClient(client)
	}

	ctx := c.Request.Context()
	if err := client.AddServer(ctx, req); err != nil {
		response.ServerError(c, fmt.Sprintf("Failed to connect to MCP server: %v", err))
		return
	}

	entry, _ := client.GetServer(req.Name)
	if entry != nil {
		toolRegistry := rag.GetToolRegistry()
		for _, tool := range entry.Tools {
			fullName := req.Name + ":" + tool.Name
			toolRegistry.Register(fullName, rag.NewMCPToolAdapter(client, fullName))
		}
	}

	newCfg := config.AIMCPServerConfig{
		Name:      req.Name,
		Transport: req.Transport,
		Command:   req.Command,
		Args:      req.Args,
		Env:       req.Env,
		Endpoint:  req.Endpoint,
	}
	var persistTarget *[]config.AIMCPServerConfig
	if config.AppConfig.AI.MCP != nil {
		persistTarget = &config.AppConfig.AI.MCP.Servers
	} else {
		persistTarget = &config.AppConfig.AI.MCPServers
	}
	cfgServers := *persistTarget
	found := false
	for i, s := range cfgServers {
		if s.Name == req.Name {
			cfgServers[i] = newCfg
			found = true
			break
		}
	}
	if !found {
		cfgServers = append(cfgServers, newCfg)
	}
	*persistTarget = cfgServers
	viper.Set("ai.mcp.servers", cfgServers)
	viper.Set("ai.mcp_servers", cfgServers)
	if err := viper.WriteConfig(); err != nil {
		logx.Infof("[AIConfigAPI] Failed to persist MCP config: %v", err)
	}

	response.Success(c, gin.H{
		"name":       req.Name,
		"connected":  true,
		"tool_count": len(entry.Tools),
	})
}

func (api *AIConfigAPI) RemoveMCPServer(c *gin.Context) {
	name := c.Param("name")
	if name == "" {
		response.BadRequest(c, "server name is required")
		return
	}

	m := aimod.GetModule()
	client := m.MCPClient()
	if client == nil {
		response.NotFound(c, "MCP client not initialized")
		return
	}

	entry, exists := client.GetServer(name)
	if !exists {
		response.NotFound(c, fmt.Sprintf("MCP server %q not found", name))
		return
	}

	toolRegistry := rag.GetToolRegistry()
	for _, tool := range entry.Tools {
		fullName := name + ":" + tool.Name
		toolRegistry.Unregister(fullName)
	}

	if err := client.RemoveServer(name); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	var persistTarget *[]config.AIMCPServerConfig
	if config.AppConfig.AI.MCP != nil {
		persistTarget = &config.AppConfig.AI.MCP.Servers
	} else {
		persistTarget = &config.AppConfig.AI.MCPServers
	}
	cfgServers := *persistTarget
	for i, s := range cfgServers {
		if s.Name == name {
			*persistTarget = append(cfgServers[:i], cfgServers[i+1:]...)
			break
		}
	}
	viper.Set("ai.mcp.servers", *persistTarget)
	viper.Set("ai.mcp_servers", *persistTarget)
	if err := viper.WriteConfig(); err != nil {
		logx.Infof("[AIConfigAPI] Failed to persist MCP config after removal: %v", err)
	}

	response.Success(c, gin.H{"removed": name})
}

func (api *AIConfigAPI) ListMCPServerTools(c *gin.Context) {
	m := aimod.GetModule()
	client := m.MCPClient()
	if client == nil {
		response.Success(c, gin.H{"tools": []interface{}{}})
		return
	}

	tools := client.GetAllTools()
	result := make([]gin.H, 0, len(tools))
	for _, t := range tools {
		result = append(result, gin.H{
			"name":        t.Name,
			"description": t.Description,
			"parameters":  t.Parameters,
		})
	}
	response.Success(c, gin.H{"tools": result})
}

func (api *AIConfigAPI) GetFeatureMaturity(c *gin.Context) {
	minLevel := c.DefaultQuery("min", "experimental")

	ragFeatures := aimod.RAGFeatures
	if minLevel != "" && minLevel != "experimental" {
		ragFeatures = aimod.FilterFeaturesByMaturity(ragFeatures, aimod.MaturityLevel(minLevel))
	}

	text2sqlFeatures := aimod.Text2SQLFeatures
	if minLevel != "" && minLevel != "experimental" {
		text2sqlFeatures = aimod.FilterFeaturesByMaturity(text2sqlFeatures, aimod.MaturityLevel(minLevel))
	}

	analyzerFeatures := aimod.AnalyzerFeatures
	if minLevel != "" && minLevel != "experimental" {
		analyzerFeatures = aimod.FilterFeaturesByMaturity(analyzerFeatures, aimod.MaturityLevel(minLevel))
	}

	toolMaturity := make([]gin.H, 0)
	for name, level := range aimod.ToolMaturity {
		if minLevel != "" && minLevel != "experimental" {
			if !level.Gte(aimod.MaturityLevel(minLevel)) {
				continue
			}
		}
		toolMaturity = append(toolMaturity, gin.H{
			"name":     name,
			"maturity": level.String(),
		})
	}

	response.Success(c, gin.H{
		"rag":          ragFeatures,
		"text2sql":     text2sqlFeatures,
		"analyzer":     analyzerFeatures,
		"tools":        toolMaturity,
		"min_maturity": minLevel,
	})
}
