package analyzer

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

type BatchProcessor struct {
	primaryProvider   provider.Provider
	fastProvider      provider.Provider
	reasoningProvider provider.Provider
	batchSize         int
	maxConcurrent     int
	rateLimitRPM      int
	docMaxChars       int
	contextMaxChars   int
	mu                sync.Mutex
	lastCallTime      time.Time
	tokenUsage        int64
}

func NewBatchProcessor(primary, fast, reasoning provider.Provider, batchSize, maxConcurrent, rateLimitRPM, docMaxChars, contextMaxChars int) *BatchProcessor {
	if batchSize <= 0 {
		batchSize = 20
	}
	if maxConcurrent <= 0 {
		maxConcurrent = 3
	}
	if rateLimitRPM <= 0 {
		rateLimitRPM = 60
	}
	if docMaxChars <= 0 {
		docMaxChars = 12000
	}
	if contextMaxChars <= 0 {
		contextMaxChars = 32000
	}
	return &BatchProcessor{
		primaryProvider:   primary,
		fastProvider:      fast,
		reasoningProvider: reasoning,
		batchSize:         batchSize,
		maxConcurrent:     maxConcurrent,
		rateLimitRPM:      rateLimitRPM,
		docMaxChars:       docMaxChars,
		contextMaxChars:   contextMaxChars,
	}
}

func (p *BatchProcessor) BatchDocuments(docs []*DocumentContent) [][]*DocumentContent {
	groups := groupBySpace(docs)

	var batches [][]*DocumentContent
	for _, group := range groups {
		// P1: 先按面包屑主题聚类，同主题文档放入同一批次
		topicGroups := groupByTopic(group)
		for _, topicGroup := range topicGroups {
			spaceBatches := p.dynamicBatch(topicGroup)
			batches = append(batches, spaceBatches...)
		}
	}
	return batches
}

func (p *BatchProcessor) dynamicBatch(docs []*DocumentContent) [][]*DocumentContent {
	var batches [][]*DocumentContent
	var current []*DocumentContent
	currentChars := 0

	for _, doc := range docs {
		docChars := len(doc.Content)
		if docChars > p.docMaxChars {
			docChars = p.docMaxChars
		}

		if len(current) > 0 && (currentChars+docChars > p.contextMaxChars || len(current) >= p.batchSize) {
			batches = append(batches, current)
			current = nil
			currentChars = 0
		}

		current = append(current, doc)
		currentChars += docChars
	}

	if len(current) > 0 {
		batches = append(batches, current)
	}
	return batches
}

func groupBySpace(docs []*DocumentContent) [][]*DocumentContent {
	spaceMap := make(map[int][]*DocumentContent)
	spaceOrder := make([]int, 0)

	for _, doc := range docs {
		if _, ok := spaceMap[doc.SpaceID]; !ok {
			spaceOrder = append(spaceOrder, doc.SpaceID)
		}
		spaceMap[doc.SpaceID] = append(spaceMap[doc.SpaceID], doc)
	}

	var groups [][]*DocumentContent
	for _, spaceID := range spaceOrder {
		groups = append(groups, spaceMap[spaceID])
	}
	return groups
}

// groupByTopic 将同一空间的文档按面包屑主题聚类，确保同类文档被分析在同一个批次中
func groupByTopic(docs []*DocumentContent) [][]*DocumentContent {
	topicMap := make(map[string][]*DocumentContent)
	topicOrder := make([]string, 0)

	for _, doc := range docs {
		topic := extractTopicFromBreadcrumb(doc.BreadCrumb, doc.SpaceName)
		if _, ok := topicMap[topic]; !ok {
			topicOrder = append(topicOrder, topic)
		}
		topicMap[topic] = append(topicMap[topic], doc)
	}

	var groups [][]*DocumentContent
	for _, topic := range topicOrder {
		groups = append(groups, topicMap[topic])
	}
	return groups
}

// extractTopicFromBreadcrumb 从面包屑路径中提取主题（空间名后的第一个目录层级）
// 示例: "后端开发 > 技术架构 > API设计 > 鉴权方案" → "技术架构"
func extractTopicFromBreadcrumb(breadcrumb, spaceName string) string {
	trimmed := strings.TrimPrefix(breadcrumb, spaceName+" > ")
	if idx := strings.Index(trimmed, " > "); idx >= 0 {
		return trimmed[:idx]
	}
	return trimmed
}

func (p *BatchProcessor) waitRateLimit() {
	p.mu.Lock()
	defer p.mu.Unlock()

	minInterval := time.Minute / time.Duration(p.rateLimitRPM)
	elapsed := time.Since(p.lastCallTime)
	if elapsed < minInterval {
		time.Sleep(minInterval - elapsed)
	}
	p.lastCallTime = time.Now()
}

func (p *BatchProcessor) ProcessBatch(ctx context.Context, batch []*DocumentContent, globalCategories string, cumulativeSummary string) (*AnalysisResult, int, error) {
	p.waitRateLimit()

	prompt := p.buildAnalysisPrompt(batch, globalCategories, cumulativeSummary)

	systemPrompt := `你是一个知识库深度分析专家。请按以下步骤对文档集合进行全面归纳分析，并以严格的JSON格式输出结果。

分析流程：先深入理解每篇文档的所有章节内容，再进行跨文档归纳。

分析步骤（Chain-of-Thought）：
1. 深入理解每个文档的完整内容，不遗漏任何章节
2. 提取每篇文档的核心概念、关键信息和详细要点（不少于5个）
3. 从全局分类体系中选择最合适的分类，或创建新分类
4. 发现文档间的知识关系（包括层级关系和语义关系）
5. 归纳跨文档的共同主题

分类规则：
- 优先从"全局分类体系"中选择已有分类
- 面包屑路径中父级目录名可作为分类依据
- 同一空间下相似文档应归入同一分类

关系类型（relation_type）：
- references: A引用了B的内容
- depends_on: A依赖B才能成立
- related_to: A和B语义相关
- contradicts: A与B观点矛盾
- parent_of: A是B的上层概念
- child_of: A是B的下层概念
- prerequisite_of: 学习A是理解B的前置条件
- alternative_of: A和B是替代方案

输出格式（严格JSON）：
{
  "categories": [
    {"name": "分类名称", "description": "分类说明", "doc_ids": [文档ID列表]}
  ],
  "summaries": [
    {
      "doc_id": 文档ID,
      "category": "所属分类",
      "summary": "至少200字的文档摘要，覆盖全部关键内容",
      "key_points": ["要点1", "要点2", "要点3", "要点4", "要点5"],
      "tags": ["标签1", "标签2"],
      "confidence": "high/medium/low"
    }
  ],
  "relations": [
    {
      "source_doc_id": 源文档ID,
      "target_doc_id": 目标文档ID,
      "type": "关系类型",
      "description": "关系描述",
      "confidence": "high/medium/low"
    }
  ],
  "themes": [
    {
      "theme": "主题名称",
      "summary": "主题归纳说明",
      "related_doc_ids": [相关文档ID列表]
    }
  ]
}

重要要求：
1. 必须输出严格的JSON格式，不要有任何额外文字说明
2. 每篇文档的summary至少200字，覆盖文档全部关键内容，不要遗漏技术细节
3. key_points不少于5个，要包含具体的配置参数、接口地址、操作步骤等技术细节
4. 准确识别文档的技术主题和业务领域
5. 发现文档间的关系并归纳跨文档主题`

	providerToUse := p.primaryProvider
	if providerToUse == nil {
		providerToUse = p.fastProvider
	}
	if providerToUse == nil {
		return nil, 0, fmt.Errorf("no LLM provider configured")
	}

	resp, err := providerToUse.Chat(ctx, systemPrompt, []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(16384), provider.WithTemperature(0.3))
	if err != nil {
		return nil, 0, fmt.Errorf("LLM call: %w", err)
	}

	tokenUsed := resp.TokenUsage.TotalTokens
	p.mu.Lock()
	p.tokenUsage += int64(tokenUsed)
	p.mu.Unlock()

	result, _ := p.parseResult(resp.Content, batch)

	// 如果解析结果质量差（半数以上文档无摘要），降温度重试一次
	if result != nil && len(batch) > 1 && len(result.Summaries) < len(batch)/2 {
		logx.Infof("[BatchProcessor] Low quality parse (%d/%d), retrying with temperature=0.1", len(result.Summaries), len(batch))
		retryResp, retryErr := providerToUse.Chat(ctx, systemPrompt, []provider.Message{
			provider.NewTextMessage("user", prompt),
		}, provider.WithMaxTokens(16384), provider.WithTemperature(0.1))
		if retryErr == nil {
			retryResult, _ := p.parseResult(retryResp.Content, batch)
			if retryResult != nil && len(retryResult.Summaries) > len(result.Summaries) {
				result = retryResult
			}
		}
	}

	// Phase2: 将源文档完整内容写入分析结果，确保信息无损
	if result != nil {
		p.populateFullContent(result, batch)
	}

	return result, tokenUsed, err
}

// populateFullContent 将源文档完整内容填入分析结果
func (p *BatchProcessor) populateFullContent(result *AnalysisResult, batch []*DocumentContent) {
	docMap := make(map[int]*DocumentContent)
	for _, doc := range batch {
		docMap[doc.DocID] = doc
	}
	for i := range result.Summaries {
		doc, ok := docMap[result.Summaries[i].SourceDocID]
		if !ok {
			continue
		}
		result.Summaries[i].FullContent = doc.Content
		result.Summaries[i].ContentSize = len([]rune(doc.Content))
		result.Summaries[i].HasFullText = true
		result.Summaries[i].UpdateTime = doc.UpdateTime
		// 按标题分段，保留结构化章节
		result.Summaries[i].Sections = splitDocIntoSections(doc.Content)
		// M-5: 计算内容哈希，用于增量差异比对
		h := sha256.Sum256([]byte(doc.Content))
		result.Summaries[i].ContentHash = fmt.Sprintf("%x", h)
	}
}

// processSingleDoc 对单篇文档调用LLM进行深度分析，提取结构化章节和深度要点
func (p *BatchProcessor) processSingleDoc(ctx context.Context, doc *DocumentContent) (*DocSummary, error) {
	sections := splitDocIntoSections(doc.Content)

	prompt := fmt.Sprintf("请深入分析以下文档，提取结构化章节信息和深度要点。\n\n文档ID: %d\n文档名称: %s\n面包屑: %s\n\n", doc.DocID, doc.DocName, doc.BreadCrumb)
	if len(sections) > 0 {
		prompt += "章节结构：\n"
		for i, sec := range sections {
			prompt += fmt.Sprintf("  [章节%d] %s (层级%d, %d字)\n", i+1, sec.Heading, sec.Level, sec.Size)
		}
	}
	prompt += fmt.Sprintf("\n文档全文：\n%s", doc.Content)

	systemPrompt := `你是一个文档深度分析专家。请对单篇文档进行结构化分析：
1. 识别所有章节及其层次关系
2. 提取每个章节的核心要点（不少于5个）
3. 归纳文档的核心主题和技术领域
4. 生成文档标签

输出JSON格式：
{
  "summary": "至少200字的文档摘要，覆盖全部关键内容",
  "key_points": ["要点1", "要点2", "要点3"],
  "tags": ["标签1", "标签2"],
  "confidence": "high/medium/low"
}
只输出JSON，不要额外文字。`

	providerToUse := p.fastProvider
	if providerToUse == nil {
		providerToUse = p.primaryProvider
	}
	if providerToUse == nil {
		return nil, fmt.Errorf("no LLM provider configured")
	}

	resp, err := providerToUse.Chat(ctx, systemPrompt, []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(8192), provider.WithTemperature(0.3))
	if err != nil {
		return nil, fmt.Errorf("single doc analysis LLM call: %w", err)
	}

	tokenUsed := resp.TokenUsage.TotalTokens
	p.mu.Lock()
	p.tokenUsage += int64(tokenUsed)
	p.mu.Unlock()

	jsonStr := utils.ExtractJSON(resp.Content)
	var raw struct {
		Summary    string   `json:"summary"`
		KeyPoints  []string `json:"key_points"`
		Tags       []string `json:"tags"`
		Confidence string   `json:"confidence"`
	}
	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("parse single doc result: %w", err)
	}

	confidence := raw.Confidence
	if confidence == "" {
		confidence = "medium"
	}

	return &DocSummary{
		SourceDocID:   doc.DocID,
		SourceSpaceID: doc.SpaceID,
		SourceName:    doc.DocName,
		SpaceName:     doc.SpaceName,
		Summary:       raw.Summary,
		KeyPoints:     raw.KeyPoints,
		Tags:          raw.Tags,
		Confidence:    confidence,
		FullContent:   doc.Content,
		Sections:      sections,
		HasFullText:   true,
		ContentSize:   len([]rune(doc.Content)),
		UpdateTime:    doc.UpdateTime,
	}, nil
}

func (p *BatchProcessor) GetTokenUsage() int64 {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.tokenUsage
}

func (p *BatchProcessor) buildAnalysisPrompt(batch []*DocumentContent, globalCategories string, cumulativeSummary string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("以下是 %d 个文档的内容，请进行归纳分析：\n\n", len(batch)))

	if globalCategories != "" {
		sb.WriteString("## 全局分类体系（请优先从中选择分类，也可新增）\n")
		sb.WriteString(globalCategories)
		sb.WriteString("\n\n")
	}

	if cumulativeSummary != "" {
		sb.WriteString("## 累积批次上下文（之前批次的归纳摘要，用于保持一致性）\n")
		sb.WriteString(cumulativeSummary)
		sb.WriteString("\n\n")
	}

	sb.WriteString("## 文档内容\n\n")
	for i, doc := range batch {
		content := doc.Content
		docSize := len([]rune(content))

		docSections := splitDocIntoSections(doc.Content)
		var sectionHeadings []string
		for _, sec := range docSections {
			sectionHeadings = append(sectionHeadings, fmt.Sprintf("%s (层级%d)", sec.Heading, sec.Level))
		}

		sb.WriteString(fmt.Sprintf("### 文档%d（共%d字）\n", i+1, docSize))
		sb.WriteString(fmt.Sprintf("- ID: %d\n- 面包屑: %s\n- 空间: %s (%s)\n- 父文档ID: %d\n- 排序: %d\n",
			doc.DocID, doc.BreadCrumb, doc.SpaceName, doc.SpaceDesc, doc.ParentID, doc.Sequence))
		if len(sectionHeadings) > 0 {
			sb.WriteString(fmt.Sprintf("- 章节结构: %s\n\n", strings.Join(sectionHeadings, " → ")))
		} else {
			sb.WriteString("\n")
		}

		sb.WriteString(content)
		sb.WriteString("\n\n---\n\n")
	}
	return sb.String()
}

func splitBySections(content string, maxChars int) []string {
	if len([]rune(content)) <= maxChars {
		return []string{content}
	}
	// Phase2: 对于超长文档，不再截断丢弃，而是按标题分段
	// 让LLM能看到完整的章节结构
	sections := splitDocIntoSections(content)
	if len(sections) <= 1 {
		// 没有标题结构的纯文本，返回完整内容（忽略maxChars限制）
		return []string{content}
	}
	// 如果文档可以分段，返回完整内容
	return []string{content}
}

// splitDocIntoSections 将文档内容按Markdown标题分词成结构化章节
func splitDocIntoSections(content string) []Section {
	var sections []Section
	lines := strings.Split(content, "\n")
	var currentHeading string
	var currentLevel int
	var currentContent strings.Builder

	flushSection := func() {
		if currentContent.Len() > 0 && currentHeading != "" {
			sections = append(sections, Section{
				Heading: currentHeading,
				Level:   currentLevel,
				Content: currentContent.String(),
				Size:    len([]rune(currentContent.String())),
			})
		}
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		var heading string
		var level int
		if strings.HasPrefix(trimmed, "# ") {
			heading = strings.TrimPrefix(trimmed, "# ")
			level = 1
		} else if strings.HasPrefix(trimmed, "## ") {
			heading = strings.TrimPrefix(trimmed, "## ")
			level = 2
		} else if strings.HasPrefix(trimmed, "### ") {
			heading = strings.TrimPrefix(trimmed, "### ")
			level = 3
		} else if strings.HasPrefix(trimmed, "#### ") {
			heading = strings.TrimPrefix(trimmed, "#### ")
			level = 4
		}

		if heading != "" {
			flushSection()
			currentHeading = heading
			currentLevel = level
			currentContent.Reset()
		} else {
			if currentContent.Len() > 0 || currentHeading == "" {
				currentContent.WriteString(line)
				currentContent.WriteString("\n")
			}
		}
	}
	flushSection()

	return sections
}

func (p *BatchProcessor) parseResult(content string, batch []*DocumentContent) (*AnalysisResult, error) {
	trimmed := utils.ExtractJSON(content)

	var raw struct {
		Categories []struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			DocIDs      []int  `json:"doc_ids"`
		} `json:"categories"`
		Summaries []struct {
			DocID      int      `json:"doc_id"`
			Category   string   `json:"category"`
			Summary    string   `json:"summary"`
			KeyPoints  []string `json:"key_points"`
			Tags       []string `json:"tags"`
			Confidence string   `json:"confidence"`
			URLs       []struct {
				URL       string `json:"url"`
				Label     string `json:"label"`
				Type      string `json:"type"`
				IsPrimary bool   `json:"is_primary"`
			} `json:"urls"`
			ConfigItems []struct {
				Name  string `json:"name"`
				Value string `json:"value"`
				Type  string `json:"type"`
			} `json:"config_items"`
			Aliases     []string `json:"aliases"`
			ActionItems []string `json:"action_items"`
		} `json:"summaries"`
		Relations []struct {
			SourceDocID int    `json:"source_doc_id"`
			TargetDocID int    `json:"target_doc_id"`
			Type        string `json:"type"`
			Description string `json:"description"`
			Confidence  string `json:"confidence"`
		} `json:"relations"`
		Themes []struct {
			Theme         string `json:"theme"`
			Summary       string `json:"summary"`
			RelatedDocIDs []int  `json:"related_doc_ids"`
		} `json:"themes"`
	}

	err := json.Unmarshal([]byte(trimmed), &raw)
	if err != nil {
		logx.Infof("[BatchProcessor] JSON parse error: %v, attempting LLM self-repair...", err)

		if p.fastProvider != nil {
			repaired := p.llmSelfRepair(content)
			if repaired != "" {
				if err2 := json.Unmarshal([]byte(repaired), &raw); err2 == nil {
					logx.Infof("[BatchProcessor] LLM self-repair succeeded")
					err = nil
				}
			}
		}

		if err != nil {
			return p.regexFallbackResult(content, batch), nil
		}
	}

	batchMap := make(map[int]*DocumentContent)
	for _, doc := range batch {
		batchMap[doc.DocID] = doc
	}

	result := &AnalysisResult{
		TotalDocs:   len(batch),
		GeneratedAt: time.Now().Unix(),
	}

	for _, cat := range raw.Categories {
		var spaceIDs []int
		validDocIDs := make([]int, 0, len(cat.DocIDs))
		for _, docID := range cat.DocIDs {
			if _, ok := batchMap[docID]; ok {
				validDocIDs = append(validDocIDs, docID)
				spaceIDs = append(spaceIDs, batchMap[docID].SpaceID)
			}
		}
		if cat.Name == "" || len(validDocIDs) == 0 {
			continue
		}
		result.Categories = append(result.Categories, Category{
			Name:        cat.Name,
			Description: cat.Description,
			DocIDs:      validDocIDs,
			SpaceIDs:    spaceIDs,
		})
	}

	for _, s := range raw.Summaries {
		var spaceName, sourceName string
		var spaceID int
		if doc, ok := batchMap[s.DocID]; ok {
			spaceName = doc.SpaceName
			sourceName = doc.DocName
			spaceID = doc.SpaceID
		} else {
			continue
		}
		confidence := s.Confidence
		if confidence == "" {
			confidence = "medium"
		}
		result.Summaries = append(result.Summaries, DocSummary{
			SourceDocID:   s.DocID,
			SourceSpaceID: spaceID,
			SourceName:    sourceName,
			SpaceName:     spaceName,
			Category:      s.Category,
			Summary:       s.Summary,
			KeyPoints:     s.KeyPoints,
			Tags:          s.Tags,
			Confidence:    confidence,
		})

		// 填充实体提取字段
		idx := len(result.Summaries) - 1
		for _, u := range s.URLs {
			result.Summaries[idx].URLs = append(result.Summaries[idx].URLs, URLInfo{
				URL: u.URL, Label: u.Label, Type: u.Type, IsPrimary: u.IsPrimary,
			})
		}
		for _, c := range s.ConfigItems {
			result.Summaries[idx].ConfigItems = append(result.Summaries[idx].ConfigItems, ConfigItem{
				Key: c.Name, Value: c.Value, Description: c.Type,
			})
		}
		result.Summaries[idx].Aliases = s.Aliases
		result.Summaries[idx].ActionItems = s.ActionItems
	}

	for _, r := range raw.Relations {
		_, srcOK := batchMap[r.SourceDocID]
		_, tgtOK := batchMap[r.TargetDocID]
		if !srcOK || !tgtOK {
			continue
		}
		confidence := r.Confidence
		if confidence == "" {
			confidence = "medium"
		}
		result.Relations = append(result.Relations, KnowledgeRelation{
			SourceDocID:  r.SourceDocID,
			TargetDocID:  r.TargetDocID,
			RelationType: r.Type,
			Description:  r.Description,
			Confidence:   confidence,
		})
	}

	for _, t := range raw.Themes {
		result.Themes = append(result.Themes, ThemeSummary{
			Theme:         t.Theme,
			Summary:       t.Summary,
			RelatedDocIDs: t.RelatedDocIDs,
		})
	}

	return result, nil
}

func (p *BatchProcessor) llmSelfRepair(badJSON string) string {
	llm := p.fastProvider
	if llm == nil {
		llm = p.primaryProvider
	}
	if llm == nil {
		return ""
	}

	preview := utils.TruncateRunes(badJSON, 2000)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	resp, err := llm.Chat(ctx, "以下JSON格式有误，请修复并返回正确的JSON。只输出修复后的JSON，不要解释。", []provider.Message{
		provider.NewTextMessage("user", preview),
	}, provider.WithMaxTokens(4096), provider.WithTemperature(0.1))
	if err != nil {
		return ""
	}

	return utils.ExtractJSON(resp.Content)
}

func (p *BatchProcessor) regexFallbackResult(content string, batch []*DocumentContent) *AnalysisResult {
	result := &AnalysisResult{
		TotalDocs:   len(batch),
		GeneratedAt: time.Now().Unix(),
	}

	summmaryRe := regexp.MustCompile(`(?i)"summary"\s*:\s*"([^"]{10,})"`)
	categoryRe := regexp.MustCompile(`(?i)"category"\s*:\s*"([^"]+)"`)
	keyPointsRe := regexp.MustCompile(`(?i)"key_points"\s*:\s*\[([^\]]+)\]`)
	tagsRe := regexp.MustCompile(`(?i)"tags"\s*:\s*\[([^\]]+)\]`)
	confidenceRe := regexp.MustCompile(`(?i)"confidence"\s*:\s*"([^"]+)"`)
	docIDRe := regexp.MustCompile(`(?i)"doc_id"\s*:\s*(\d+)`)

	matches := docIDRe.FindAllStringSubmatch(content, -1)
	summaries := summmaryRe.FindAllStringSubmatch(content, -1)
	categories := categoryRe.FindAllStringSubmatch(content, -1)
	confidences := confidenceRe.FindAllStringSubmatch(content, -1)

	maxLen := len(batch)
	if len(matches) < maxLen {
		maxLen = len(matches)
	}
	if len(summaries) < maxLen {
		maxLen = len(summaries)
	}

	for i := 0; i < maxLen; i++ {
		docID := 0
		fmt.Sscanf(matches[i][1], "%d", &docID)

		var doc *DocumentContent
		for _, d := range batch {
			if d.DocID == docID {
				doc = d
				break
			}
		}
		if doc == nil {
			if i < len(batch) {
				doc = batch[i]
			} else {
				continue
			}
		}

		summary := summaries[i][1]
		category := doc.SpaceName
		if i < len(categories) {
			category = categories[i][1]
		}
		confidence := "low"
		if i < len(confidences) {
			confidence = confidences[i][1]
		}

		var keyPoints []string
		if i < len(keyPointsRe.FindAllStringSubmatch(content, -1)) {
			kpMatches := keyPointsRe.FindAllStringSubmatch(content, -1)
			if i < len(kpMatches) {
				for _, kp := range strings.Split(kpMatches[i][1], ",") {
					kp = strings.TrimSpace(kp)
					kp = strings.Trim(kp, "\"")
					if kp != "" {
						keyPoints = append(keyPoints, kp)
					}
				}
			}
		}

		var tags []string
		if i < len(tagsRe.FindAllStringSubmatch(content, -1)) {
			tagMatches := tagsRe.FindAllStringSubmatch(content, -1)
			if i < len(tagMatches) {
				for _, t := range strings.Split(tagMatches[i][1], ",") {
					t = strings.TrimSpace(t)
					t = strings.Trim(t, "\"")
					if t != "" {
						tags = append(tags, t)
					}
				}
			}
		}

		result.Summaries = append(result.Summaries, DocSummary{
			SourceDocID:   doc.DocID,
			SourceSpaceID: doc.SpaceID,
			SourceName:    doc.DocName,
			SpaceName:     doc.SpaceName,
			Category:      category,
			Summary:       summary,
			KeyPoints:     keyPoints,
			Tags:          tags,
			Confidence:    confidence,
		})
	}

	if len(result.Summaries) == 0 {
		return p.fallbackResult(batch)
	}
	return result
}

func (p *BatchProcessor) fallbackResult(batch []*DocumentContent) *AnalysisResult {
	result := &AnalysisResult{
		TotalDocs:   len(batch),
		GeneratedAt: time.Now().Unix(),
	}
	for _, doc := range batch {
		summary := utils.TruncateWithEllipsis(doc.Content, 500)
		result.Summaries = append(result.Summaries, DocSummary{
			SourceDocID:   doc.DocID,
			SourceSpaceID: doc.SpaceID,
			SourceName:    doc.DocName,
			SpaceName:     doc.SpaceName,
			Category:      doc.SpaceName,
			Summary:       summary,
			FullContent:   doc.Content,
			Sections:      splitDocIntoSections(doc.Content),
			HasFullText:   true,
			ContentSize:   len([]rune(doc.Content)),
			KeyPoints:     []string{},
			Tags:          []string{},
		})
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
