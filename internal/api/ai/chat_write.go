package ai

import (
	"context"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"time"

	"github.com/gin-gonic/gin"
	aimod "github.com/listen808/citebase/internal/ai"
	provider "github.com/listen808/citebase/internal/ai/provider"
	rag "github.com/listen808/citebase/internal/ai/rag"
	"github.com/listen808/citebase/internal/middleware"
	"github.com/listen808/citebase/internal/response"
	config "github.com/listen808/citebase/internal/utils/config"
)

// AI 写作与内容生成：写作辅助 / 表格 / 报告 / 邮件 / 翻译 / 推荐 / 质量巡检 / 模板。

type writeAssistRequest struct {
	Text        string `json:"text" binding:"required"`
	Action      string `json:"action" binding:"required"`
	Instruction string `json:"instruction"`
}

func validWriteAssistActions() map[string]bool {
	return rag.ValidWriteActions()
}

func (api *ChatAPI) WriteAssist(c *gin.Context) {
	var req writeAssistRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "text and action are required")
		return
	}

	if !validWriteAssistActions()[req.Action] {
		response.BadRequest(c, fmt.Sprintf("不支持的 action: %s", req.Action))
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	summarizer := m.Summarizer()
	if summarizer == nil {
		response.ServerErrorKey(c, response.MsgAIServiceUnconfigured)
		return
	}

	startTime := time.Now()
	result, tokenUsage, err := summarizer.WriteAssist(c.Request.Context(), req.Text, rag.AIAction(req.Action), req.Instruction)
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

	questionText := fmt.Sprintf("动作: %s", req.Action)
	if req.Instruction != "" {
		questionText = fmt.Sprintf("动作: %s, 指令: %s", req.Action, req.Instruction)
	}
	if len(req.Text) > 200 {
		questionText = questionText + ", 文本: " + req.Text[:200] + "..."
	} else {
		questionText = questionText + ", 文本: " + req.Text
	}

	targetID := ""
	resultStr := "success"
	reasonStr := ""
	if err != nil {
		resultStr = "failed"
		reasonStr = err.Error()
	}

	RecordAIAudit(c, userID, "write_assist", targetID, questionText, result, modelName, providerName, resultStr, reasonStr, tokenUsage.TotalTokens, 0, durationMs)

	if err != nil {
		logx.Infof("[ChatAPI] WriteAssist error: %v", err)
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{"result": result})
}

func (api *ChatAPI) GenerateTable(c *gin.Context) {
	var req struct {
		Description string `json:"description" binding:"required"`
		SourceText  string `json:"source_text"`
		TableType   string `json:"table_type"`
		MaxRows     int    `json:"max_rows"`
		MaxColumns  int    `json:"max_columns"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "description is required")
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	tg := m.TableGenerator()
	if tg == nil {
		response.ServerErrorKey(c, response.MsgAITableWriterUnconfigured)
		return
	}

	startTime := time.Now()
	result, tokenUsage, err := tg.Generate(c.Request.Context(), rag.TableRequest{
		Description: req.Description,
		SourceText:  req.SourceText,
		TableType:   req.TableType,
		MaxRows:     req.MaxRows,
		MaxColumns:  req.MaxColumns,
	})
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

	resultStr := "success"
	reason := ""
	if err != nil {
		resultStr = "failed"
		reason = err.Error()
	}

	RecordAIAudit(c, userID, "table_generate", "", req.Description, "", modelName, providerName, resultStr, reason, tokenUsage.TotalTokens, 0, durationMs)

	if err != nil {
		logx.Infof("[ChatAPI] GenerateTable error: %v", err)
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"markdown":     result.Markdown,
		"headers":      result.Headers,
		"rows":         result.Rows,
		"chart_config": result.ChartConfig,
	})
}

func (api *ChatAPI) GenerateReport(c *gin.Context) {
	var req struct {
		Type     string   `json:"type" binding:"required"`
		Title    string   `json:"title" binding:"required"`
		Sources  []string `json:"sources"`
		Extra    string   `json:"extra"`
		Sections []string `json:"sections"`
		Language string   `json:"language"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "type and title are required")
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	rg := m.ReportGenerator()
	if rg == nil {
		response.ServerErrorKey(c, response.MsgAIReportWriterUnconfigured)
		return
	}

	startTime := time.Now()
	result, tokenUsage, err := rg.Generate(c.Request.Context(), rag.ReportRequest{
		Type:     rag.ReportType(req.Type),
		Title:    req.Title,
		Sources:  req.Sources,
		Extra:    req.Extra,
		Sections: req.Sections,
		Language: req.Language,
	})
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

	resultStr := "success"
	reason := ""
	if err != nil {
		resultStr = "failed"
		reason = err.Error()
	}

	RecordAIAudit(c, userID, "report_generate", "", req.Title, "", modelName, providerName, resultStr, reason, tokenUsage.TotalTokens, 0, durationMs)

	if err != nil {
		logx.Infof("[ChatAPI] GenerateReport error: %v", err)
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"title":    result.Title,
		"content":  result.Content,
		"sections": result.Sections,
	})
}

func (api *ChatAPI) GenerateEmail(c *gin.Context) {
	var req struct {
		Type       string `json:"type" binding:"required"`
		Subject    string `json:"subject"`
		DocContent string `json:"doc_content"`
		DocTitle   string `json:"doc_title"`
		Tone       string `json:"tone"`
		Extra      string `json:"extra"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "type is required")
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	eg := m.EmailGenerator()
	if eg == nil {
		response.ServerErrorKey(c, response.MsgAIEmailWriterUnconfigured)
		return
	}

	startTime := time.Now()
	result, tokenUsage, err := eg.Generate(c.Request.Context(), rag.EmailRequest{
		Type:       rag.EmailType(req.Type),
		Subject:    req.Subject,
		DocContent: req.DocContent,
		DocTitle:   req.DocTitle,
		Tone:       req.Tone,
		Extra:      req.Extra,
	})
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

	resultStr := "success"
	reason := ""
	if err != nil {
		resultStr = "failed"
		reason = err.Error()
	}

	RecordAIAudit(c, userID, "email_generate", "", req.Subject, "", modelName, providerName, resultStr, reason, tokenUsage.TotalTokens, 0, durationMs)

	if err != nil {
		logx.Infof("[ChatAPI] GenerateEmail error: %v", err)
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"subject": result.Subject,
		"body":    result.Body,
	})
}

func (api *ChatAPI) TranslateText(c *gin.Context) {
	var req struct {
		Text           string            `json:"text" binding:"required"`
		TargetLanguage string            `json:"target_language" binding:"required"`
		SourceLanguage string            `json:"source_language"`
		PreserveFormat bool              `json:"preserve_format"`
		Glossary       map[string]string `json:"glossary"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "text and target_language are required")
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	t := m.Translator()
	if t == nil {
		response.ServerErrorKey(c, response.MsgAITranslatorUnconfigured)
		return
	}

	startTime := time.Now()
	result, tokenUsage, err := t.Translate(c.Request.Context(), rag.TranslateRequest{
		Text:           req.Text,
		SourceLanguage: req.SourceLanguage,
		TargetLanguage: req.TargetLanguage,
		PreserveFormat: req.PreserveFormat,
		Glossary:       req.Glossary,
	})
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

	resultStr := "success"
	reason := ""
	if err != nil {
		resultStr = "failed"
		reason = err.Error()
	}

	RecordAIAudit(c, userID, "translate", "", req.Text[:min(100, len(req.Text))], "", modelName, providerName, resultStr, reason, tokenUsage.TotalTokens, 0, durationMs)

	if err != nil {
		logx.Infof("[ChatAPI] TranslateText error: %v", err)
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{
		"translated_text": result.TranslatedText,
		"source_language": result.SourceLanguage,
		"target_language": result.TargetLanguage,
	})
}

func (api *ChatAPI) GetRecommendations(c *gin.Context) {
	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	rec := m.Recommender()
	if rec == nil {
		response.ServerErrorKey(c, response.MsgAIRecommendUnconfigured)
		return
	}

	userID, _ := middleware.GetUserID(c)

	limitStr := c.DefaultQuery("limit", "5")
	limit := 5
	fmt.Sscanf(limitStr, "%d", &limit)

	recommendations, err := rec.RecommendForUser(userCtx(c), userID, limit)
	if err != nil {
		logx.Infof("[ChatAPI] GetRecommendations error: %v", err)
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, gin.H{"recommendations": recommendations})
}

func (api *ChatAPI) RunQualityPatrol(c *gin.Context) {
	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	qp := m.QualityPatrol()
	if qp == nil {
		response.ServerErrorKey(c, response.MsgAIQualityUnconfigured)
		return
	}

	spaceIDStr := c.DefaultQuery("space_id", "0")
	spaceID := 0
	fmt.Sscanf(spaceIDStr, "%d", &spaceID)

	result, err := qp.RunPatrol(userCtx(c), spaceID)
	if err != nil {
		logx.Infof("[ChatAPI] RunQualityPatrol error: %v", err)
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, result)
}

type aiTemplate struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Variables   []string `json:"variables"`
	Icon        string   `json:"icon,omitempty"`
}

func builtinTemplates() []aiTemplate {
	return []aiTemplate{
		{ID: "report", Name: "工作报告", Description: "结构化工作报告，包含内容、成果、问题和计划", Variables: []string{"period", "achievements", "challenges", "plan"}, Icon: "file-text"},
		{ID: "email", Name: "邮件撰写", Description: "根据要点生成专业邮件", Variables: []string{"recipient", "subject", "key_points", "tone"}, Icon: "mail"},
		{ID: "weekly", Name: "周报", Description: "汇总本周工作进展与下周计划", Variables: []string{"week", "completed", "in_progress", "next_week"}, Icon: "calendar"},
		{ID: "meeting_minutes", Name: "会议纪要", Description: "议题、决议和待办事项", Variables: []string{"meeting_title", "date", "attendees", "topics", "decisions", "action_items"}, Icon: "clipboard"},
		{ID: "tech_doc", Name: "技术文档", Description: "技术方案或API文档", Variables: []string{"title", "overview", "details", "examples"}, Icon: "code"},
		{ID: "project_proposal", Name: "项目方案", Description: "项目提案，背景、目标、计划、风险", Variables: []string{"project_name", "background", "objectives", "timeline", "risks"}, Icon: "lightbulb"},
		{ID: "knowledge_card", Name: "知识卡片", Description: "将知识点整理为知识卡片", Variables: []string{"topic", "definition", "key_points", "examples"}, Icon: "book-open"},
		{ID: "release_notes", Name: "发布说明", Description: "版本发布新功能、修复和改进", Variables: []string{"version", "new_features", "fixes", "improvements"}, Icon: "package"},
	}
}

// ListTemplates 返回 AI 写作模板列表
func (api *ChatAPI) ListTemplates(c *gin.Context) {
	response.Success(c, gin.H{"templates": builtinTemplates()})
}

type applyTemplateRequest struct {
	TemplateID string            `json:"template_id" binding:"required"`
	Inputs     map[string]string `json:"inputs"`
	DocID      int               `json:"doc_id"`
	SpaceID    int               `json:"space_id"`
}

// ApplyTemplate 应用模板，使用 AI 生成内容
func (api *ChatAPI) ApplyTemplate(c *gin.Context) {
	var req applyTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "template_id is required")
		return
	}

	m := aimod.GetModule()
	if !m.Enabled() {
		response.ServerErrorKey(c, response.MsgAIServiceDisabled)
		return
	}

	// 查找模板
	templates := builtinTemplates()
	var tmpl *aiTemplate
	for i := range templates {
		if templates[i].ID == req.TemplateID {
			tmpl = &templates[i]
			break
		}
	}
	if tmpl == nil {
		response.BadRequest(c, "template not found: "+req.TemplateID)
		return
	}

	// 构建 prompt
	prompt := fmt.Sprintf("你是一个专业的文档撰写助手。请根据以下模板生成内容。\n\n模板类型: %s\n描述: %s\n", tmpl.Name, tmpl.Description)
	for k, v := range req.Inputs {
		prompt += fmt.Sprintf("%s: %s\n", k, v)
	}
	prompt += "\n请直接输出生成的内容，不需要额外解释。"

	// 调用 AI 生成
	prim := m.PrimaryProvider()
	if prim == nil {
		response.ServerErrorKey(c, response.MsgAIPrimaryModelUnconfigured)
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 120*time.Second)
	defer cancel()

	textMsg := provider.NewTextMessage("user", prompt)
	result, err := prim.Chat(ctx, "", []provider.Message{textMsg})
	if err != nil {
		logx.Infof("[Template] AI generate error: %v", err)
		response.ServerErrorKey(c, response.MsgAIGenerateFailed)
		return
	}

	response.Success(c, gin.H{"result": result.Content})
}
