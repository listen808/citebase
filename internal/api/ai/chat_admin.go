package ai

import (
	"fmt"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	aimod "github.com/listen808/citebase/internal/ai"
	provider "github.com/listen808/citebase/internal/ai/provider"
	rag "github.com/listen808/citebase/internal/ai/rag"
	"github.com/listen808/citebase/internal/middleware"
	aimodel "github.com/listen808/citebase/internal/model/ai"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	airepo "github.com/listen808/citebase/internal/repository/ai"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
	"github.com/listen808/citebase/internal/response"
	config "github.com/listen808/citebase/internal/utils/config"
)

// AI 管理与辅助：Token 用量 / 缓存 / 图像 / 导入 / 概览 / 反馈 / 仪表盘 / 自动标签 / 嵌入 Widget / 审计日志。

func (api *ChatAPI) GetTokenUsage(c *gin.Context) {
	total, byUser := provider.GetTokenUsage()
	response.Success(c, gin.H{
		"total_tokens_today": total,
		"by_user":            byUser,
	})
}

// GetTokenUsageByModel 按模型分组的使用统计
func (api *ChatAPI) GetTokenUsageByModel(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if days < 1 || days > 365 {
		days = 30
	}

	auditLogRepo := airepo.NewAIAuditLogRepository()
	stats, err := auditLogRepo.UsageByModel(days, limit)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	// Calculate totals for percentage
	var grandTotal int64
	for _, s := range stats {
		grandTotal += s.TotalUsage
	}

	type modelStatResp struct {
		Model        string  `json:"model"`
		Provider     string  `json:"provider"`
		TotalUsage   int64   `json:"total_usage"`
		RequestCount int64   `json:"request_count"`
		AvgUsage     int64   `json:"avg_usage"`
		Percent      float64 `json:"percent"`
	}
	result := make([]modelStatResp, 0, len(stats))
	for _, s := range stats {
		pct := 0.0
		if grandTotal > 0 {
			pct = float64(s.TotalUsage) / float64(grandTotal) * 100
		}
		result = append(result, modelStatResp{
			Model:        s.Model,
			Provider:     s.Provider,
			TotalUsage:   s.TotalUsage,
			RequestCount: s.RequestCount,
			AvgUsage:     int64(s.AvgUsage),
			Percent:      pct,
		})
	}

	response.Success(c, gin.H{
		"stats": result,
		"total": grandTotal,
		"days":  days,
	})
}

// GetTokenUsageByUser 按用户分组的使用统计
func (api *ChatAPI) GetTokenUsageByUser(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	if days < 1 || days > 365 {
		days = 30
	}

	auditLogRepo := airepo.NewAIAuditLogRepository()
	stats, err := auditLogRepo.UsageByUser(days, limit)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	var grandTotal int64
	for _, s := range stats {
		grandTotal += s.TotalUsage
	}

	type userStatResp struct {
		UserID       int     `json:"user_id"`
		Username     string  `json:"username"`
		GivenName    string  `json:"given_name"`
		TotalUsage   int64   `json:"total_usage"`
		RequestCount int64   `json:"request_count"`
		AvgUsage     int64   `json:"avg_usage"`
		LastUsed     string  `json:"last_used"`
		Percent      float64 `json:"percent"`
	}
	result := make([]userStatResp, 0, len(stats))
	for _, s := range stats {
		pct := 0.0
		if grandTotal > 0 {
			pct = float64(s.TotalUsage) / float64(grandTotal) * 100
		}
		result = append(result, userStatResp{
			UserID:       s.UserID,
			Username:     s.Username,
			GivenName:    s.GivenName,
			TotalUsage:   s.TotalUsage,
			RequestCount: s.RequestCount,
			AvgUsage:     int64(s.AvgUsage),
			LastUsed:     s.LastUsed,
			Percent:      pct,
		})
	}

	response.Success(c, gin.H{
		"stats": result,
		"total": grandTotal,
		"days":  days,
	})
}

// GetTokenUsageDailyTrend 每日使用量趋势
func (api *ChatAPI) GetTokenUsageDailyTrend(c *gin.Context) {
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))
	if days < 1 || days > 365 {
		days = 30
	}

	auditLogRepo := airepo.NewAIAuditLogRepository()
	stats, err := auditLogRepo.UsageDailyTrend(days)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	var maxUsage int64
	for _, s := range stats {
		if s.TotalUsage > maxUsage {
			maxUsage = s.TotalUsage
		}
	}

	type trendResp struct {
		Date         string  `json:"date"`
		TotalUsage   int64   `json:"total_usage"`
		RequestCount int64   `json:"request_count"`
		UniqueUsers  int64   `json:"unique_users"`
		Percent      float64 `json:"percent"`
	}
	result := make([]trendResp, 0, len(stats))
	for _, s := range stats {
		pct := 0.0
		if maxUsage > 0 {
			pct = float64(s.TotalUsage) / float64(maxUsage) * 100
		}
		result = append(result, trendResp{
			Date:         s.Date,
			TotalUsage:   s.TotalUsage,
			RequestCount: s.RequestCount,
			UniqueUsers:  s.UniqueUsers,
			Percent:      pct,
		})
	}

	response.Success(c, gin.H{
		"trend": result,
		"max":   maxUsage,
		"days":  days,
	})
}

func (api *ChatAPI) GetQueryCacheStats(c *gin.Context) {
	cache := rag.GetQueryCache()
	if cache == nil {
		response.Success(c, gin.H{"enabled": false, "size": 0})
		return
	}
	response.Success(c, gin.H{
		"enabled": true,
		"size":    cache.Size(),
	})
}

func (api *ChatAPI) ClearQueryCache(c *gin.Context) {
	cache := rag.GetQueryCache()
	if cache != nil {
		cache.Clear()
	}
	response.Success(c, gin.H{"cleared": true})
}

func (api *ChatAPI) DescribeImage(c *gin.Context) {
	var req struct {
		ImageURL string `json:"image_url" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "image_url is required")
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

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

	startTime := time.Now()
	processor := rag.NewMultimodalProcessor(m.PrimaryProvider())
	description, tokenUsage, err := processor.DescribeImage(c.Request.Context(), req.ImageURL)
	durationMs := time.Since(startTime).Milliseconds()

	userID, _ := middleware.GetUserID(c)

	resultStr := "success"
	reasonStr := ""
	if err != nil {
		resultStr = "failed"
		reasonStr = err.Error()
	}

	RecordAIAudit(c, userID, "describe_image", req.ImageURL, fmt.Sprintf("描述图片: %s", req.ImageURL), description, modelName, providerName, resultStr, reasonStr, tokenUsage.TotalTokens, 0, durationMs)

	if err != nil {
		response.ServerError(c, fmt.Sprintf("图片描述失败: %v", err))
		return
	}

	response.Success(c, gin.H{"description": description})
}

func (api *ChatAPI) ImportDocument(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
		Format  string `json:"format" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "content and format are required")
		return
	}

	m := aimod.GetModule()
	startTime := time.Now()
	importer := rag.NewDocumentImporter(m.PrimaryProvider())

	doc, err := importer.ImportFromText(c.Request.Context(), req.Content, req.Format)
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

	questionText := fmt.Sprintf("文档导入, 格式: %s, 长度: %d", req.Format, len(req.Content))
	answerText := ""
	if doc != nil {
		answerText = fmt.Sprintf("文档导入成功: %s", doc.Title)
	}

	RecordAIAudit(c, userID, "import_doc", targetID, questionText, answerText, modelName, providerName, resultStr, reasonStr, 0, 0, durationMs)

	if err != nil {
		response.ServerError(c, fmt.Sprintf("导入失败: %v", err))
		return
	}

	response.Success(c, gin.H{"document": doc})
}

func (api *ChatAPI) SearchOverview(c *gin.Context) {
	var req struct {
		Query   string       `json:"query" binding:"required"`
		Results []rag.Source `json:"results"`
		SpaceID int          `json:"space_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "query is required")
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() || m.FastProvider() == nil {
		response.Success(c, gin.H{"overview": nil, "message": "AI服务未启用"})
		return
	}

	startTime := time.Now()
	tagging := rag.NewAutoTagger(m.FastProvider())
	overview, err := tagging.GenerateSearchOverview(c.Request.Context(), req.Query, req.Results)
	durationMs := time.Since(startTime).Milliseconds()

	userID, _ := middleware.GetUserID(c)

	providerName := ""
	modelName := ""
	if p := m.FastProvider(); p != nil {
		providerName = p.Name()
	}
	if cfg := config.AppConfig.AI.Primary; cfg != nil {
		modelName = cfg.Model
	} else if config.AppConfig.AI.Model != "" {
		modelName = config.AppConfig.AI.Model
	}

	targetID := ""
	if req.SpaceID > 0 {
		targetID = fmt.Sprintf("%d", req.SpaceID)
	}

	resultStr := "success"
	reasonStr := ""
	if err != nil {
		resultStr = "failed"
		reasonStr = err.Error()
	}

	overviewText := ""
	if overview != nil {
		overviewText = overview.Overview
	}
	RecordAIAudit(c, userID, "search_overview", targetID, req.Query, overviewText, modelName, providerName, resultStr, reasonStr, 0, len(req.Results), durationMs)

	if err != nil {
		response.Success(c, gin.H{"overview": nil, "message": fmt.Sprintf("生成概览失败: %v", err)})
		return
	}

	response.Success(c, gin.H{"overview": overview})
}

func (api *ChatAPI) SubmitFeedback(c *gin.Context) {
	uid, err := middleware.GetUserID(c)
	if err != nil || uid == 0 {
		response.Unauthorized(c, "user not authenticated")
		return
	}

	var req struct {
		ConversationID int    `json:"conversation_id"`
		Question       string `json:"question"`
		Answer         string `json:"answer"`
		Rating         int    `json:"rating" binding:"required"`
		Reason         string `json:"reason"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "rating is required")
		return
	}

	if req.Rating != 1 && req.Rating != -1 {
		response.BadRequest(c, "rating must be 1 (positive) or -1 (negative)")
		return
	}

	feedback := &knowledgemodel.AIFeedback{
		UserID:         uid,
		ConversationID: req.ConversationID,
		Question:       req.Question,
		Answer:         req.Answer,
		Rating:         req.Rating,
		Reason:         req.Reason,
	}

	feedbackRepo := knowledgerepo.DefaultAIFeedbackRepository()
	if err := feedbackRepo.Create(feedback); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{"feedback_id": feedback.ID})
}

func (api *ChatAPI) GetFeedbackStats(c *gin.Context) {
	feedbackRepo := knowledgerepo.DefaultAIFeedbackRepository()
	positive, negative, err := feedbackRepo.GetStats()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	reasons, _ := feedbackRepo.GetRecentReasons(10)

	response.Success(c, gin.H{
		"positive":       positive,
		"negative":       negative,
		"total":          positive + negative,
		"recent_reasons": reasons,
	})
}

func (api *ChatAPI) GetAIDashboard(c *gin.Context) {
	m := aimod.GetModule()
	if !m.Enabled() {
		response.Success(c, gin.H{"enabled": false})
		return
	}

	total, byUser := provider.GetTokenUsage()
	cache := rag.GetQueryCache()

	cacheSize := 0
	if cache != nil {
		cacheSize = cache.Size()
	}

	feedbackRepo := knowledgerepo.DefaultAIFeedbackRepository()
	positive, negative, _ := feedbackRepo.GetStats()
	recentReasons, _ := feedbackRepo.GetRecentReasons(5)

	vs := m.VectorStore()
	docCount := 0
	if vs != nil {
		docCount = vs.DocCount()
	}

	providerInfo := gin.H{}
	if pp := m.PrimaryProvider(); pp != nil {
		providerInfo["name"] = pp.Name()
	}

	analysisTaskCount := 0
	if sched := m.Scheduler(); sched != nil {
		analysisTaskCount = sched.TaskCount()
	}

	response.Success(c, gin.H{
		"enabled":             true,
		"token_usage":         total,
		"token_by_user":       byUser,
		"cache_size":          cacheSize,
		"vector_docs":         docCount,
		"analysis_task_count": analysisTaskCount,
		"provider":            providerInfo,
		"feedback": gin.H{
			"positive":       positive,
			"negative":       negative,
			"total":          positive + negative,
			"recent_reasons": recentReasons,
		},
	})
}

func (api *ChatAPI) AutoTag(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "content is required")
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	startTime := time.Now()
	tagging := rag.NewAutoTagger(m.FastProvider())
	result, err := tagging.GenerateTags(c.Request.Context(), req.Content)
	durationMs := time.Since(startTime).Milliseconds()

	userID, _ := middleware.GetUserID(c)

	providerName := ""
	modelName := ""
	if p := m.FastProvider(); p != nil {
		providerName = p.Name()
	}
	if cfg := config.AppConfig.AI.Primary; cfg != nil {
		modelName = cfg.Model
	} else if config.AppConfig.AI.Model != "" {
		modelName = config.AppConfig.AI.Model
	}

	answerText := ""
	if result != nil {
		answerText = fmt.Sprintf("标签: %v", result)
	}

	targetID := ""
	resultStr := "success"
	reasonStr := ""
	if err != nil {
		resultStr = "failed"
		reasonStr = err.Error()
	}

	questionText := fmt.Sprintf("自动标签, 内容长度: %d", len(req.Content))
	if len(req.Content) > 100 {
		questionText = fmt.Sprintf("自动标签, 内容长度: %d, 内容: %s...", len(req.Content), req.Content[:100])
	}

	RecordAIAudit(c, userID, "auto_tag", targetID, questionText, answerText, modelName, providerName, resultStr, reasonStr, 0, 0, durationMs)

	if err != nil {
		response.ServerError(c, fmt.Sprintf("标签生成失败: %v", err))
		return
	}

	response.Success(c, gin.H{"tags": result})
}

// widgetBaseFromRequest 由请求推导挂件用的绝对基址。
// Host 是客户端可控的头，所以必须过 SafeBaseURL；返回 false 时调用方要拒绝出页，
// 而不是退回一个默认值 —— 挂件页要 fetch 同源接口，猜错基址就是跨域请求失败，
// 那种"看起来配好了其实一直报错"的状态最难查。
func widgetBaseFromRequest(c *gin.Context) (string, bool) {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return rag.SafeBaseURL(scheme + "://" + c.Request.Host)
}

func (api *ChatAPI) ServeWidget(c *gin.Context) {
	baseURL, ok := widgetBaseFromRequest(c)
	if !ok {
		response.BadRequestKey(c, response.MsgHostHeaderUnsafePage)
		return
	}

	widget := rag.NewEmbedWidget(baseURL)
	widget.ServeWidgetPage(c.Writer, c.Request)
}

func (api *ChatAPI) ServeWidgetJS(c *gin.Context) {
	baseURL, ok := widgetBaseFromRequest(c)
	if !ok {
		response.BadRequestKey(c, response.MsgHostHeaderUnsafeScript)
		return
	}

	widget := rag.NewEmbedWidget(baseURL)
	widget.ServeWidgetJS(c.Writer, c.Request)
}

func (api *ChatAPI) GetEmbedCode(c *gin.Context) {
	baseURL, ok := widgetBaseFromRequest(c)
	if !ok {
		response.BadRequestKey(c, response.MsgHostHeaderUnsafeEmbed)
		return
	}

	widget := rag.NewEmbedWidget(baseURL)
	jsCode := widget.GenerateEmbedJS()
	iframeCode := fmt.Sprintf(`<iframe src="%s/ai/widget" width="400" height="550" style="border:none;border-radius:12px;box-shadow:0 4px 16px rgba(0,0,0,0.1);"></iframe>`, baseURL)

	response.Success(c, gin.H{
		"js_embed":     jsCode,
		"iframe_embed": iframeCode,
		"base_url":     baseURL,
	})
}

// ListAIAuditLogs 获取AI审计日志列表
func (api *ChatAPI) ListAIAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))
	action := c.Query("action")

	auditLogRepo := airepo.NewAIAuditLogRepository()

	var logs []*aimodel.AIAuditLog
	var total int64
	var err error

	roleID, _ := c.Get("role_id")
	isAdmin := false
	if roleID != nil {
		if rid, ok := roleID.(int); ok && (rid == 1 || rid == 2) {
			isAdmin = true
		}
	}

	if action != "" {
		if isAdmin {
			logs, total, err = auditLogRepo.FindByAction(action, limit, offset)
		} else {
			userID, _ := middleware.GetUserID(c)
			if userID > 0 {
				var userLogs []*aimodel.AIAuditLog
				userLogs, total, err = auditLogRepo.FindByUserID(userID, limit, offset)
				for _, log := range userLogs {
					if log.Action == action {
						logs = append(logs, log)
					}
				}
			} else {
				logs, err = auditLogRepo.FindRecent(limit)
				total = int64(len(logs))
			}
		}
	} else if isAdmin {
		logs, total, err = auditLogRepo.FindAll(limit, offset)
	} else {
		userID, _ := middleware.GetUserID(c)
		if userID > 0 {
			logs, total, err = auditLogRepo.FindByUserID(userID, limit, offset)
		} else {
			logs, err = auditLogRepo.FindRecent(limit)
			total = int64(len(logs))
		}
	}

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
