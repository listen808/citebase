package rag

import (
	"context"
	"fmt"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/service/doccontent"
	"github.com/listen808/citebase/internal/utils"
)

type AIAction string

const (
	AIActionRewrite   AIAction = "rewrite"
	AIActionContinue  AIAction = "continue"
	AIActionTranslate AIAction = "translate"
	AIActionSummarize AIAction = "summarize"
	AIActionTemplate  AIAction = "template"  // S10: 文档模板生成
	AIActionDiffDesc  AIAction = "diff_desc" // S11: 变更摘要

	// 写作辅助 action
	AIActionExplain      AIAction = "explain"
	AIActionShorten      AIAction = "shorten"
	AIActionFormalize    AIAction = "formalize"
	AIActionSimplify     AIAction = "simplify"
	AIActionBulletize    AIAction = "bulletize"
	AIActionExpand       AIAction = "expand"
	AIActionCheckQuality AIAction = "check_quality"
	AIActionAddComments  AIAction = "add_comments"
	AIActionFixGrammar   AIAction = "fix_grammar"
)

type Summarizer struct {
	provider   provider.Provider
	docRepo    *workspacerepo.DocumentRepository
	spaceRepo  *workspacerepo.SpaceRepository
	docStorage *utils.DocumentStorage
}

func NewSummarizer(p provider.Provider) *Summarizer {
	docDir := "./data/markdowns"
	return &Summarizer{
		provider:   p,
		docRepo:    workspacerepo.NewDocumentRepository(),
		spaceRepo:  workspacerepo.NewSpaceRepository(),
		docStorage: utils.NewDocumentStorageWithDir(docDir),
	}
}

func (s *Summarizer) GenerateSummary(ctx context.Context, docID int) (string, provider.TokenUsage, error) {
	if s.provider == nil {
		return "", provider.TokenUsage{}, fmt.Errorf("AI服务未启用")
	}
	content := s.readDocumentContent(ctx, docID)
	if content == "" {
		return "", provider.TokenUsage{}, fmt.Errorf("文档内容为空或无权访问")
	}
	if len(content) > 12000 {
		content = utils.TruncateRunes(content, 12000)
	}

	prompt := fmt.Sprintf(`请对以下文档内容生成结构化的中文摘要，包含以下部分：
1. 概要（100字以内，概括文档核心主题）
2. 要点（3-5个关键要点，每个不超过30字）
3. 关键词（3-5个标签词，用逗号分隔）

示例输出格式：
概要：本文描述了基于JWT的微服务鉴权方案设计
要点：
- 采用RS256非对称签名算法
- Token刷新机制支持7天有效期
- 网关层统一验证并传递用户信息
关键词：JWT,鉴权,微服务,RS256,网关

文档内容：
%s`, content)

	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := s.provider.Chat(timeoutCtx, "你是一个知识库AI助手，擅长总结文档要点。请严格按照指定格式输出摘要。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	})
	if err != nil {
		return "", provider.TokenUsage{}, err
	}
	return resp.Content, resp.TokenUsage, nil
}

func (s *Summarizer) WriteAssist(ctx context.Context, text string, action AIAction, instruction string) (string, provider.TokenUsage, error) {
	if s.provider == nil {
		return "", provider.TokenUsage{}, fmt.Errorf("AI服务未启用")
	}
	if text == "" {
		return "", provider.TokenUsage{}, fmt.Errorf("文本内容为空")
	}
	if len(text) > 8000 {
		text = utils.TruncateRunes(text, 8000)
	}

	var systemPrompt string
	var userPrompt string
	var temperature float64

	switch action {
	case AIActionRewrite:
		systemPrompt = "你是一个专业的写作助手，请按要求处理文本。"
		userPrompt = fmt.Sprintf("请改写以下文本，使其更加清晰、专业。%s\n\n%s", addInstruction(instruction), text)
		temperature = 0.7
	case AIActionContinue:
		systemPrompt = "你是一个专业的写作助手，请按要求处理文本。"
		userPrompt = fmt.Sprintf("请续写以下文本，保持风格一致，续写100-200字：\n\n%s", text)
		temperature = 0.7
	case AIActionTranslate:
		lang := "英文"
		if instruction != "" {
			lang = instruction
		}
		systemPrompt = "你是一个专业的写作助手，请按要求处理文本。"
		userPrompt = fmt.Sprintf("请将以下文本翻译为%s：\n\n%s", lang, text)
		temperature = 0.7
	case AIActionSummarize:
		systemPrompt = "你是一个专业的写作助手，请按要求处理文本。"
		userPrompt = fmt.Sprintf("请总结以下文本的核心要点，不超过150字：\n\n%s", text)
		temperature = 0.7
	case AIActionTemplate:
		// S10: 文档模板生成
		templateType := "通用文档"
		if instruction != "" {
			templateType = instruction
		}
		systemPrompt = "你是一个专业的写作助手，请按要求处理文本。"
		userPrompt = fmt.Sprintf(`请根据以下描述生成一份%s的Markdown文档模板。
要求：
- 包含清晰的标题层级（#、##、###）
- 在需要填充内容的地方使用 [待填写] 占位符
- 包含适当的示例内容
- 模板应实用且可直接使用

描述：%s`, templateType, text)
		temperature = 0.7
	case AIActionDiffDesc:
		// S11: 变更摘要 - text为旧内容，instruction为新内容
		if instruction == "" {
			return "", provider.TokenUsage{}, fmt.Errorf("变更摘要需要提供新内容作为instruction")
		}
		systemPrompt = "你是一个专业的写作助手，请按要求处理文本。"
		userPrompt = fmt.Sprintf(`请用中文描述以下文档变更的核心内容（不超过100字）：

旧内容（前1000字）：
%s

新内容（前1000字）：
%s`, utils.TruncateWithEllipsis(text, 1000), utils.TruncateWithEllipsis(instruction, 1000))
		temperature = 0.7
	case AIActionExplain:
		systemPrompt = "你是一位善于用通俗语言解释复杂概念的专家。请用简单易懂的方式解释以下内容，适合非技术人员理解。保持原意的同时使用更平实的语言。"
		userPrompt = fmt.Sprintf("请解释以下内容：\n\n%s", text)
		temperature = 0.5
	case AIActionShorten:
		systemPrompt = "你是一位精炼文字的专家。请在保留核心信息的前提下，将以下文本精简到最短。去除冗余表述，保留关键信息。输出精简后的文本，不要添加额外说明。"
		userPrompt = fmt.Sprintf("请精简以下文本：\n\n%s", text)
		temperature = 0.3
	case AIActionFormalize:
		systemPrompt = "你是一位专业的文字润色专家。请将以下文本转换为正式、专业的书面语气。保持原意，使用更正式的措辞和句式。输出正式化后的文本，不要添加额外说明。"
		userPrompt = fmt.Sprintf("请将以下文本正式化：\n\n%s", text)
		temperature = 0.4
	case AIActionSimplify:
		systemPrompt = "你是一位简化表达的专家。请使用简单易懂的词汇和句式重写以下内容。避免专业术语和复杂句式，让所有人都能理解。输出简化后的文本，不要添加额外说明。"
		userPrompt = fmt.Sprintf("请简化以下文本：\n\n%s", text)
		temperature = 0.4
	case AIActionBulletize:
		systemPrompt = "你是一位格式化专家。请将以下内容转换为清晰的要点列表格式。每个要点一行，以 '- ' 开头。保留所有关键信息，去除冗余表述。输出要点列表，不要添加额外说明。"
		userPrompt = fmt.Sprintf("请将以下文本转换为要点列表：\n\n%s", text)
		temperature = 0.3
	case AIActionExpand:
		systemPrompt = "你是一位内容扩展专家。请在不改变原意的前提下，丰富和扩展以下内容的细节。补充必要的背景信息、示例或解释，使内容更加完整和详尽。输出扩展后的文本，不要添加额外说明。"
		userPrompt = fmt.Sprintf("请扩展以下文本：\n\n%s", text)
		temperature = 0.7
	case AIActionCheckQuality:
		systemPrompt = "你是一位文字质量审查专家。请仔细检查以下文本的质量问题，包括：语法错误、逻辑不一致、表述不清、用词不当、格式问题等。请列出发现的问题和改进建议，按严重程度排序。"
		userPrompt = fmt.Sprintf("请检查以下文本的质量：\n\n%s", text)
		temperature = 0.3
	case AIActionAddComments:
		systemPrompt = "你是一位文档批注专家。请为以下内容添加解释性批注和注释。在关键概念、专业术语、重要逻辑处添加注释说明。使用 Markdown 引用格式（> 注：...）标注批注。"
		userPrompt = fmt.Sprintf("请为以下文本添加批注：\n\n%s", text)
		temperature = 0.5
	case AIActionFixGrammar:
		systemPrompt = "你是一位语法修正专家。请修正以下文本中的语法错误、拼写错误和标点问题。只修正错误，不改变原文风格和表述方式。输出修正后的完整文本，不要列出修改清单。"
		userPrompt = fmt.Sprintf("请修正以下文本的语法错误：\n\n%s", text)
		temperature = 0.2
	default:
		return "", provider.TokenUsage{}, fmt.Errorf("不支持的操作: %s", action)
	}

	resp, err := s.provider.Chat(ctx, systemPrompt, []provider.Message{
		provider.NewTextMessage("user", userPrompt),
	}, provider.WithMaxTokens(2048), provider.WithTemperature(temperature))
	if err != nil {
		return "", provider.TokenUsage{}, err
	}
	return resp.Content, resp.TokenUsage, nil
}

func addInstruction(instruction string) string {
	if instruction != "" {
		return fmt.Sprintf("额外要求：%s", instruction)
	}
	return ""
}

// ValidWriteActions 返回所有有效的写辅助 action 集合
func ValidWriteActions() map[string]bool {
	return map[string]bool{
		string(AIActionRewrite):      true,
		string(AIActionContinue):     true,
		string(AIActionTranslate):    true,
		string(AIActionSummarize):    true,
		string(AIActionTemplate):     true,
		string(AIActionDiffDesc):     true,
		string(AIActionExplain):      true,
		string(AIActionShorten):      true,
		string(AIActionFormalize):    true,
		string(AIActionSimplify):     true,
		string(AIActionBulletize):    true,
		string(AIActionExpand):       true,
		string(AIActionCheckQuality): true,
		string(AIActionAddComments):  true,
		string(AIActionFixGrammar):   true,
	}
}

// readDocumentContent 读取文档正文。docID 可能来自越权请求，因此读取前必须校验
// 文档所属空间对当前调用者可见，不可见时返回空内容（调用方据此报错，绝不返回摘要）。
func (s *Summarizer) readDocumentContent(ctx context.Context, docID int) string {
	doc, err := s.docRepo.FindByID(docID)
	if err != nil || doc == nil {
		return ""
	}
	if err := ensureSpaceVisible(ctx, doc.SpaceID); err != nil {
		return ""
	}
	if doc.Type != 1 {
		return ""
	}

	pageFile := getPageFilePath(doc, s.docRepo, s.spaceRepo, s.docStorage)
	content, err := doccontent.Get(doc.ID, s.docStorage, pageFile)
	if err != nil || content == "" {
		latestVersion, verErr := s.docRepo.GetLatestVersion(docID)
		if verErr == nil && latestVersion > 0 {
			versionContent, verErr := s.docRepo.GetVersionContent(docID, latestVersion)
			if verErr == nil {
				content = versionContent
			}
		}
	}
	return content
}
