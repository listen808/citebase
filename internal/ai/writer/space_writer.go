package writer

import (
	"fmt"
	"strings"
	"sync"
	"time"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/analyzer"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/service/doccontent"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/config"
)

type SpaceWriter struct {
	spaceRepo   *workspacerepo.SpaceRepository
	docRepo     *workspacerepo.DocumentRepository
	spaceSvc    *workspacesvc.SpaceService
	docSvc      *workspacesvc.DocumentService
	docStorage  *utils.DocumentStorage
	aiSpaceID   int
	aiSpaceName string
	docIDMap    map[string]int
	docIDMu     sync.RWMutex                          // 并发分析任务会同时写 docIDMap，无锁会触发 fatal error: concurrent map writes
	mappingRepo *knowledgerepo.AIDocMappingRepository // P1: 用于增量合并检查
}

// setDir / getDir 是 docIDMap 的唯一入口，锁收敛在这里而不是散落到每个调用点。
func (w *SpaceWriter) setDir(key string, id int) {
	w.docIDMu.Lock()
	w.docIDMap[key] = id
	w.docIDMu.Unlock()
}

func (w *SpaceWriter) getDir(key string) (int, bool) {
	w.docIDMu.RLock()
	defer w.docIDMu.RUnlock()
	id, ok := w.docIDMap[key]
	return id, ok
}

func NewSpaceWriter(spaceName string) *SpaceWriter {
	docDir := config.AppConfig.Storage.DocumentDir
	if docDir == "" {
		docDir = "./data/markdowns"
	}
	return &SpaceWriter{
		spaceRepo:   workspacerepo.NewSpaceRepository(),
		docRepo:     workspacerepo.NewDocumentRepository(),
		spaceSvc:    workspacesvc.NewSpaceService(),
		docSvc:      workspacesvc.NewDocumentService(),
		docStorage:  utils.NewDocumentStorageWithDir(docDir),
		aiSpaceName: spaceName,
		docIDMap:    make(map[string]int),
		mappingRepo: knowledgerepo.NewAIDocMappingRepository(),
	}
}

func (w *SpaceWriter) EnsureAISpace(systemUserID int) (int, error) {
	if w.aiSpaceID > 0 {
		return w.aiSpaceID, nil
	}

	spaces, _, err := w.spaceRepo.FindAll(0, 10000, "", "", "")
	if err != nil {
		return 0, fmt.Errorf("query spaces: %w", err)
	}

	for _, space := range spaces {
		if space.Name == w.aiSpaceName {
			// 历史版本曾以 visit_level=1（登录用户可见）创建 AI 空间：分析范围覆盖全部空间
			// （含私密空间），摘要里的正文概要/访问地址/配置项会构成跨空间信息泄露，
			// 统一收紧为仅成员/管理员可见。
			if space.IsPublic() || space.IsLoggedInVisible() {
				space.VisitLevel = workspacemodel.VisitLevelMemberOnly
				if err := w.spaceRepo.Update(space); err != nil {
					return 0, fmt.Errorf("tighten AI space visit level: %w", err)
				}
				logx.Infof("[SpaceWriter] AI space %s visit_level tightened to member-only", w.aiSpaceName)
			}
			w.aiSpaceID = space.ID
			return w.aiSpaceID, nil
		}
	}

	space, err := w.spaceSvc.CreateSpace(systemUserID, &workspacemodel.CreateSpaceRequest{
		Name:        w.aiSpaceName,
		Description: "AI 自动归纳梳理的知识空间",
		// AI 摘要覆盖全库（含私密空间）内容，必须仅成员/管理员可见（2），
		// 否则任何登录用户都能读到私密空间的文档摘要与配置项。
		VisitLevel: "2",
	})
	if err != nil {
		return 0, fmt.Errorf("create AI space: %w", err)
	}

	w.aiSpaceID = space.ID
	logx.Infof("[SpaceWriter] AI space created: %s (ID: %d)", w.aiSpaceName, w.aiSpaceID)
	return w.aiSpaceID, nil
}

func (w *SpaceWriter) GetAISpaceID() int {
	return w.aiSpaceID
}

func (w *SpaceWriter) WriteResult(result *analyzer.AnalysisResult, systemUserID int) error {
	spaceID, err := w.EnsureAISpace(systemUserID)
	if err != nil {
		return err
	}

	// P1: 增量合并策略 - 查询已被用户人工编辑或审核通过的文档，写入时跳过
	protectedDocIDs := make(map[int]bool)
	if w.mappingRepo != nil {
		manualMappings, _ := w.mappingRepo.FindByReviewStatus(knowledgemodel.ReviewStatusManualEdit)
		for _, m := range manualMappings {
			protectedDocIDs[m.SourceDocID] = true
		}
		acceptedMappings, _ := w.mappingRepo.FindByReviewStatus(knowledgemodel.ReviewStatusAccepted)
		for _, m := range acceptedMappings {
			protectedDocIDs[m.SourceDocID] = true
		}
		if len(protectedDocIDs) > 0 {
			logx.Infof("[SpaceWriter] Protecting %d manually-edited docs from overwrite", len(protectedDocIDs))
		}
	}

	categoryDirIDs := w.createCategoryDirs(spaceID, result.Categories, systemUserID)
	spaceDirIDs := w.createSpaceDirs(spaceID, result.Summaries, systemUserID)

	if len(result.Summaries) > 0 {
		if err := w.writeSummaries(spaceID, result.Summaries, categoryDirIDs, spaceDirIDs, systemUserID, protectedDocIDs); err != nil {
			logx.Infof("[SpaceWriter] Write summaries error: %v", err)
		}
	}

	if len(result.Categories) > 0 {
		if err := w.writeCategoryIndex(spaceID, result.Categories, categoryDirIDs, result.Summaries, systemUserID); err != nil {
			logx.Infof("[SpaceWriter] Write category index error: %v", err)
		}
	}

	if len(result.Relations) > 0 {
		if err := w.writeRelations(spaceID, result.Relations, result.Summaries, systemUserID); err != nil {
			logx.Infof("[SpaceWriter] Write relations error: %v", err)
		}
	}

	if len(result.Themes) > 0 {
		if err := w.writeThemes(spaceID, result.Themes, result.Summaries, systemUserID); err != nil {
			logx.Infof("[SpaceWriter] Write themes error: %v", err)
		}
	}

	if len(result.Gaps) > 0 {
		if err := w.writeGaps(spaceID, result.Gaps, systemUserID); err != nil {
			logx.Infof("[SpaceWriter] Write gaps error: %v", err)
		}
	}

	if err := w.writeKnowledgeGraph(result, spaceID); err != nil {
		logx.Infof("[SpaceWriter] Write knowledge graph error: %v", err)
	}

	if err := w.writeAnalysisReport(spaceID, result, systemUserID); err != nil {
		logx.Infof("[SpaceWriter] Write analysis report error: %v", err)
	}

	return w.writeOverview(spaceID, result, systemUserID)
}

func (w *SpaceWriter) createCategoryDirs(spaceID int, categories []analyzer.Category, userID int) map[string]int {
	dirIDs := make(map[string]int)

	for _, cat := range categories {
		parts := strings.SplitN(cat.Name, "/", 2)
		topCat := parts[0]

		if _, ok := dirIDs[topCat]; ok {
			continue
		}

		dirName := topCat
		dirID, err := w.ensureDir(spaceID, 0, dirName, userID)
		if err != nil {
			logx.Infof("[SpaceWriter] Create category dir %s error: %v", dirName, err)
			continue
		}
		dirIDs[topCat] = dirID
		w.setDir("cat:"+topCat, dirID)

		if len(parts) > 1 {
			subCat := parts[1]
			subKey := topCat + "/" + subCat
			if _, ok := dirIDs[subKey]; ok {
				continue
			}
			subDirID, err := w.ensureDir(spaceID, dirID, subCat, userID)
			if err != nil {
				logx.Infof("[SpaceWriter] Create sub-category dir %s/%s error: %v", topCat, subCat, err)
				continue
			}
			dirIDs[subKey] = subDirID
			w.setDir("cat:"+subKey, subDirID)
		}
	}

	return dirIDs
}

func (w *SpaceWriter) createSpaceDirs(spaceID int, summaries []analyzer.DocSummary, userID int) map[int]int {
	dirIDs := make(map[int]int)
	summaryDirID, err := w.ensureDir(spaceID, 0, "文档摘要", userID)
	if err != nil {
		logx.Infof("[SpaceWriter] Create summary dir error: %v", err)
		return dirIDs
	}
	w.setDir("dir:summaries", summaryDirID)

	spaceSet := make(map[int]string)
	for _, s := range summaries {
		if _, ok := spaceSet[s.SourceSpaceID]; !ok {
			spaceSet[s.SourceSpaceID] = s.SpaceName
		}
	}

	for spaceID, spaceName := range spaceSet {
		dirID, err := w.ensureDir(spaceID, summaryDirID, spaceName, userID)
		if err != nil {
			logx.Infof("[SpaceWriter] Create space dir %s error: %v", spaceName, err)
			continue
		}
		dirIDs[spaceID] = dirID
	}

	return dirIDs
}

func (w *SpaceWriter) ensureDir(spaceID, parentID int, name string, userID int) (int, error) {
	docs, err := w.docRepo.FindBySpaceID(spaceID)
	if err != nil {
		return 0, err
	}

	for _, doc := range docs {
		if doc.Name == name && doc.Type == workspacemodel.DocumentTypeDir && doc.ParentID == parentID {
			return doc.ID, nil
		}
	}

	doc, err := w.docSvc.CreateDocument(userID, &workspacemodel.CreateDocumentRequest{
		SpaceID:  spaceID,
		ParentID: parentID,
		Name:     name,
		Type:     workspacemodel.DocumentTypeDir,
	})
	if err != nil {
		return 0, fmt.Errorf("create dir: %w", err)
	}
	logx.Infof("[SpaceWriter] Created dir: %s (ID: %d, Parent: %d)", name, doc.ID, parentID)
	return doc.ID, nil
}

func (w *SpaceWriter) writeSummaries(spaceID int, summaries []analyzer.DocSummary, categoryDirIDs map[string]int, spaceDirIDs map[int]int, userID int, protectedDocIDs map[int]bool) error {
	skippedCount := 0
	for _, s := range summaries {
		// P1: 跳过已人工编辑或审核的文档，不覆盖用户修改
		if protectedDocIDs[s.SourceDocID] {
			skippedCount++
			continue
		}
		title := s.SourceName
		content := renderSummaryTemplate(s)
		parentID := w.resolveSummaryParent(s, categoryDirIDs, spaceDirIDs)
		if err := w.upsertDocument(spaceID, parentID, title, content, userID); err != nil {
			logx.Infof("[SpaceWriter] Write summary for doc %d error: %v", s.SourceDocID, err)
		}
	}
	if skippedCount > 0 {
		logx.Infof("[SpaceWriter] Skipped %d protected docs from overwrite", skippedCount)
	}
	return nil
}

func (w *SpaceWriter) resolveSummaryParent(s analyzer.DocSummary, categoryDirIDs map[string]int, spaceDirIDs map[int]int) int {
	if dirID, ok := categoryDirIDs[s.Category]; ok {
		return dirID
	}
	parts := strings.SplitN(s.Category, "/", 2)
	if dirID, ok := categoryDirIDs[parts[0]]; ok {
		return dirID
	}
	if dirID, ok := spaceDirIDs[s.SourceSpaceID]; ok {
		return dirID
	}
	if dirID, ok := w.getDir("dir:summaries"); ok {
		return dirID
	}
	return 0
}

func (w *SpaceWriter) writeCategoryIndex(spaceID int, categories []analyzer.Category, categoryDirIDs map[string]int, summaries []analyzer.DocSummary, userID int) error {
	summaryMap := make(map[int]analyzer.DocSummary)
	for _, s := range summaries {
		summaryMap[s.SourceDocID] = s
	}

	for _, cat := range categories {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("# %s\n\n%s\n\n", cat.Name, cat.Description))
		sb.WriteString(fmt.Sprintf("> 包含文档数：%d | 生成时间：%s\n\n", len(cat.DocIDs), time.Now().Format("2006-01-02 15:04:05")))

		sb.WriteString("## 文档列表\n\n")
		for _, docID := range cat.DocIDs {
			docName := fmt.Sprintf("文档ID:%d", docID)
			if s, ok := summaryMap[docID]; ok && s.SourceName != "" {
				docName = s.SourceName
			}
			sb.WriteString(fmt.Sprintf("- [%s](/space/%d/doc/%d)", docName, spaceID, docID))
			if s, ok := summaryMap[docID]; ok {
				if s.Summary != "" {
					runes := []rune(s.Summary)
					if len(runes) > 50 {
						sb.WriteString(fmt.Sprintf(" — %s...", string(runes[:50])))
					} else {
						sb.WriteString(fmt.Sprintf(" — %s", s.Summary))
					}
				}
			}
			sb.WriteString("\n")
		}

		dirID := categoryDirIDs[cat.Name]
		if dirID == 0 {
			parts := strings.SplitN(cat.Name, "/", 2)
			dirID = categoryDirIDs[parts[0]]
		}

		title := cat.Name + " - 分类索引"
		if err := w.upsertDocument(spaceID, dirID, title, sb.String(), userID); err != nil {
			logx.Infof("[SpaceWriter] Write category index for %s error: %v", cat.Name, err)
		}
	}
	return nil
}

func (w *SpaceWriter) writeRelations(spaceID int, relations []analyzer.KnowledgeRelation, summaries []analyzer.DocSummary, userID int) error {
	docNameMap := make(map[int]string)
	docSpaceMap := make(map[int]int)
	for _, s := range summaries {
		docNameMap[s.SourceDocID] = s.SourceName
		docSpaceMap[s.SourceDocID] = s.SourceSpaceID
	}

	groupedByType := make(map[string][]analyzer.KnowledgeRelation)
	for _, r := range relations {
		groupedByType[r.RelationType] = append(groupedByType[r.RelationType], r)
	}

	typeNames := map[string]string{
		"references":      "引用关系",
		"depends_on":      "依赖关系",
		"related_to":      "关联关系",
		"contradicts":     "矛盾关系",
		"parent_of":       "父子关系",
		"child_of":        "子父关系",
		"prerequisite_of": "前置条件",
		"alternative_of":  "替代方案",
	}

	var sb strings.Builder
	sb.WriteString("# 知识关系\n\n")
	sb.WriteString(fmt.Sprintf("> 生成时间：%s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	for relType, rels := range groupedByType {
		name := typeNames[relType]
		if name == "" {
			name = relType
		}
		sb.WriteString(fmt.Sprintf("## %s\n\n", name))
		for _, r := range rels {
			srcName := docNameMap[r.SourceDocID]
			tgtName := docNameMap[r.TargetDocID]
			srcSpaceID := docSpaceMap[r.SourceDocID]
			tgtSpaceID := docSpaceMap[r.TargetDocID]

			if srcName != "" && tgtName != "" {
				sb.WriteString(fmt.Sprintf("- [%s](/space/%d/doc/%d) → [%s](/space/%d/doc/%d)：%s\n",
					srcName, srcSpaceID, r.SourceDocID, tgtName, tgtSpaceID, r.TargetDocID, r.Description))
			} else {
				sb.WriteString(fmt.Sprintf("- 文档 %d → 文档 %d：%s\n", r.SourceDocID, r.TargetDocID, r.Description))
			}
		}
		sb.WriteString("\n")
	}

	sb.WriteString("## 知识图谱\n\n")
	sb.WriteString("```mermaid\ngraph LR\n")
	nodeIndex := 0
	docToNode := make(map[int]string)
	for _, r := range relations {
		if _, ok := docToNode[r.SourceDocID]; !ok {
			docToNode[r.SourceDocID] = fmt.Sprintf("n%d", nodeIndex)
			nodeIndex++
		}
		if _, ok := docToNode[r.TargetDocID]; !ok {
			docToNode[r.TargetDocID] = fmt.Sprintf("n%d", nodeIndex)
			nodeIndex++
		}
	}
	for docID, nodeID := range docToNode {
		name := docNameMap[docID]
		if name == "" {
			name = fmt.Sprintf("Doc%d", docID)
		}
		// P3：转义 mermaid label 中的特殊字符，避免破坏图表语法
		name = strings.ReplaceAll(name, "\"", "#quot;")
		name = strings.ReplaceAll(name, "]", "#93;")
		name = strings.ReplaceAll(name, "[", "#91;")
		sb.WriteString(fmt.Sprintf("  %s[\"%s\"]\n", nodeID, name))
	}
	for _, r := range relations {
		srcID := docToNode[r.SourceDocID]
		tgtID := docToNode[r.TargetDocID]
		relLabel := r.RelationType
		if relLabel == "" {
			relLabel = "关联"
		}
		sb.WriteString(fmt.Sprintf("  %s -->|%s| %s\n", srcID, relLabel, tgtID))
	}
	sb.WriteString("```\n")

	// 按文档视角分组展示
	sb.WriteString("## 按文档视角\n\n")
	groupedByDoc := make(map[int][]analyzer.KnowledgeRelation)
	for _, r := range relations {
		groupedByDoc[r.SourceDocID] = append(groupedByDoc[r.SourceDocID], r)
	}
	for docID, rels := range groupedByDoc {
		name := docNameMap[docID]
		if name == "" {
			name = fmt.Sprintf("文档 %d", docID)
		}
		sb.WriteString(fmt.Sprintf("### [%s](/space/%d/doc/%d)\n\n", name, docSpaceMap[docID], docID))
		for _, r := range rels {
			tgtName := docNameMap[r.TargetDocID]
			if tgtName == "" {
				tgtName = fmt.Sprintf("文档 %d", r.TargetDocID)
			}
			sb.WriteString(fmt.Sprintf("- → [%s](/space/%d/doc/%d)（%s）\n",
				tgtName, docSpaceMap[r.TargetDocID], r.TargetDocID, r.Description))
		}
		sb.WriteString("\n")
	}

	summaryDirID, _ := w.getDir("dir:summaries")
	return w.upsertDocument(spaceID, summaryDirID, "知识关系", sb.String(), userID)
}

func (w *SpaceWriter) writeThemes(spaceID int, themes []analyzer.ThemeSummary, summaries []analyzer.DocSummary, userID int) error {
	docNameMap := make(map[int]string)
	docSpaceMap := make(map[int]int)
	for _, s := range summaries {
		docNameMap[s.SourceDocID] = s.SourceName
		docSpaceMap[s.SourceDocID] = s.SourceSpaceID
	}

	var sb strings.Builder
	sb.WriteString("# 主题归纳\n\n")
	sb.WriteString(fmt.Sprintf("> 生成时间：%s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	for _, t := range themes {
		weight := ""
		if len(t.RelatedDocIDs) >= 5 {
			weight = " 🔥"
		} else if len(t.RelatedDocIDs) >= 3 {
			weight = " ⭐"
		}
		sb.WriteString(fmt.Sprintf("## %s%s\n\n%s\n\n**相关文档（%d篇）：**\n", t.Theme, weight, t.Summary, len(t.RelatedDocIDs)))
		for _, docID := range t.RelatedDocIDs {
			name := docNameMap[docID]
			spaceID := docSpaceMap[docID]
			if name != "" {
				sb.WriteString(fmt.Sprintf("- [%s](/space/%d/doc/%d)\n", name, spaceID, docID))
			} else {
				sb.WriteString(fmt.Sprintf("- 文档 ID: %d\n", docID))
			}
		}
		sb.WriteString("\n")
	}

	summaryDirID, _ := w.getDir("dir:summaries")
	return w.upsertDocument(spaceID, summaryDirID, "主题归纳", sb.String(), userID)
}

func (w *SpaceWriter) writeGaps(spaceID int, gaps []analyzer.KnowledgeGap, userID int) error {
	var sb strings.Builder
	sb.WriteString("# 知识缺口\n\n")
	sb.WriteString("> AI 识别出的知识盲区和薄弱环节，建议补充相关文档\n\n")
	sb.WriteString(fmt.Sprintf("> 生成时间：%s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	sb.WriteString("| 优先级 | 分类 | 缺口描述 | 建议补充内容 |\n")
	sb.WriteString("|--------|------|----------|-------------|\n")

	for _, g := range gaps {
		priorityLabel := "中"
		switch g.Priority {
		case "high":
			priorityLabel = "🔴 高"
		case "medium":
			priorityLabel = "🟡 中"
		case "low":
			priorityLabel = "🟢 低"
		}
		sb.WriteString(fmt.Sprintf("| %s | %s | %s | %s |\n",
			priorityLabel, g.Category, g.Description, g.SuggestedTitle))
	}

	summaryDirID, _ := w.getDir("dir:summaries")
	return w.upsertDocument(spaceID, summaryDirID, "知识缺口", sb.String(), userID)
}

func (w *SpaceWriter) writeOverview(spaceID int, result *analyzer.AnalysisResult, userID int) error {
	var sb strings.Builder
	sb.WriteString("# 知识库总览\n\n")
	sb.WriteString(fmt.Sprintf("> 由 AI 于 %s 自动生成\n\n", time.Now().Format("2006-01-02 15:04:05")))

	sb.WriteString("## 统计概览\n\n")
	sb.WriteString("| 指标 | 数值 |\n")
	sb.WriteString("|------|------|\n")
	sb.WriteString(fmt.Sprintf("| 分析文档总数 | %d |\n", result.TotalDocs))
	sb.WriteString(fmt.Sprintf("| 已处理文档 | %d |\n", result.ProcessedDocs))
	sb.WriteString(fmt.Sprintf("| 知识分类数 | %d |\n", len(result.Categories)))
	sb.WriteString(fmt.Sprintf("| 识别关系数 | %d |\n", len(result.Relations)))
	sb.WriteString(fmt.Sprintf("| 主题归纳数 | %d |\n", len(result.Themes)))
	if len(result.Gaps) > 0 {
		sb.WriteString(fmt.Sprintf("| 知识缺口数 | %d |\n", len(result.Gaps)))
	}

	lowQualityCount := 0
	for _, s := range result.Summaries {
		if s.Quality != nil && s.Quality.OverallScore < 60 {
			lowQualityCount++
		}
	}
	if lowQualityCount > 0 {
		sb.WriteString(fmt.Sprintf("| 低质量文档 | %d |\n", lowQualityCount))
	}
	sb.WriteString("\n")

	if len(result.Categories) > 0 {
		sb.WriteString("## 分类概览\n\n")
		for _, cat := range result.Categories {
			sb.WriteString(fmt.Sprintf("### [%s](/space/%d/doc/%d)\n%s\n\n包含文档数：%d\n\n", cat.Name, spaceID, 0, cat.Description, len(cat.DocIDs)))
		}
	}

	if len(result.Themes) > 0 {
		sb.WriteString("## 主题归纳\n\n")
		for _, t := range result.Themes {
			sb.WriteString(fmt.Sprintf("### %s\n%s\n\n", t.Theme, t.Summary))
		}
	}

	// P0: 低质量/低置信度文档提示
	lowQualityDocs := 0
	lowConfidenceDocs := 0
	for _, s := range result.Summaries {
		if s.Quality != nil && s.Quality.OverallScore < 60 {
			lowQualityDocs++
		}
		if s.Confidence == "low" {
			lowConfidenceDocs++
		}
	}
	if lowQualityDocs > 0 || lowConfidenceDocs > 0 {
		sb.WriteString("## 需关注文档\n\n")
		sb.WriteString("> 以下文档建议人工审核：\n\n")
		for _, s := range result.Summaries {
			reasons := []string{}
			if s.Confidence == "low" {
				reasons = append(reasons, "置信度低")
			}
			if s.Quality != nil && s.Quality.OverallScore < 60 {
				reasons = append(reasons, fmt.Sprintf("质量评分%.0f", s.Quality.OverallScore))
				for _, issue := range s.Quality.Issues {
					reasons = append(reasons, issue.Description)
				}
			}
			if len(reasons) > 0 {
				sb.WriteString(fmt.Sprintf("- [%s](/space/%d/doc/%d) — %s\n",
					s.SourceName, s.SourceSpaceID, s.SourceDocID, strings.Join(reasons, "、")))
			}
		}
		sb.WriteString("\n")
	}

	return w.upsertDocument(spaceID, 0, "知识总览", sb.String(), userID)
}

func (w *SpaceWriter) writeAnalysisReport(spaceID int, result *analyzer.AnalysisResult, userID int) error {
	var sb strings.Builder
	sb.WriteString("# 分析报告\n\n")
	sb.WriteString(fmt.Sprintf("> 生成时间：%s\n\n", time.Now().Format("2006-01-02 15:04:05")))

	sb.WriteString("## 分析概况\n\n")
	sb.WriteString(fmt.Sprintf("| 指标 | 数值 |\n|---|---|\n"))
	sb.WriteString(fmt.Sprintf("| 分析文档数 | %d |\n", len(result.Summaries)))
	sb.WriteString(fmt.Sprintf("| 分类数 | %d |\n", len(result.Categories)))
	sb.WriteString(fmt.Sprintf("| 关系数 | %d |\n", len(result.Relations)))
	sb.WriteString(fmt.Sprintf("| 主题数 | %d |\n", len(result.Themes)))
	gapCount := len(result.Gaps)
	sb.WriteString(fmt.Sprintf("| 知识缺口数 | %d |\n", gapCount))
	sb.WriteString("\n")

	if len(result.Categories) > 0 {
		sb.WriteString("## 分类分布\n\n")
		for _, cat := range result.Categories {
			sb.WriteString(fmt.Sprintf("- **%s**：%d篇文档 — %s\n", cat.Name, len(cat.DocIDs), cat.Description))
		}
		sb.WriteString("\n")
	}

	if len(result.Themes) > 0 {
		sb.WriteString("## 核心主题\n\n")
		for _, t := range result.Themes {
			sb.WriteString(fmt.Sprintf("- **%s**：%s（涉及%d篇文档）\n", t.Theme, t.Summary, len(t.RelatedDocIDs)))
		}
		sb.WriteString("\n")
	}

	if len(result.Gaps) > 0 {
		sb.WriteString("## 知识缺口建议\n\n")
		sb.WriteString("| 优先级 | 分类 | 缺口描述 |\n")
		sb.WriteString("|--------|------|----------|\n")
		for _, g := range result.Gaps {
			priorityLabel := g.Priority
			switch g.Priority {
			case "high":
				priorityLabel = "🔴 高"
			case "medium":
				priorityLabel = "🟡 中"
			case "low":
				priorityLabel = "🟢 低"
			}
			sb.WriteString(fmt.Sprintf("| %s | %s | %s |\n", priorityLabel, g.Category, g.Description))
		}
		sb.WriteString("\n")
	}

	dirID := w.docIDMap["dir:summaries"]
	return w.upsertDocument(spaceID, dirID, "分析报告", sb.String(), userID)
}

func (w *SpaceWriter) upsertDocument(spaceID int, parentID int, name string, content string, userID int) error {
	docs, err := w.docRepo.FindBySpaceID(spaceID)
	if err != nil {
		return fmt.Errorf("query docs: %w", err)
	}

	for _, doc := range docs {
		if doc.Name == name && doc.Type == workspacemodel.DocumentTypePage && doc.ParentID == parentID {
			space, _ := w.spaceRepo.FindByID(doc.SpaceID)
			spaceName := fmt.Sprintf("space_%d", doc.SpaceID)
			if space != nil {
				spaceName = space.Name
			}
			pageFile := w.docStorage.GetPageFile(spaceName, doc.Name, doc.Type)
			return doccontent.Set(doc.ID, w.docStorage, pageFile, content)
		}
	}

	_, err = w.docSvc.CreateDocument(userID, &workspacemodel.CreateDocumentRequest{
		SpaceID:  spaceID,
		ParentID: parentID,
		Name:     name,
		Type:     workspacemodel.DocumentTypePage,
		Content:  content,
	})
	if err != nil {
		return fmt.Errorf("create doc: %w", err)
	}
	logx.Infof("[SpaceWriter] Created document: %s in space %d (parent: %d)", name, spaceID, parentID)
	return nil
}

func renderSummaryTemplate(s analyzer.DocSummary) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("# %s\n\n", s.SourceName))
	sb.WriteString(fmt.Sprintf("> 源文档：[%s](/space/%d/doc/%d) | 空间：%s | 归类：%s\n", s.SourceName, s.SourceSpaceID, s.SourceDocID, s.SpaceName, s.Category))
	sb.WriteString(fmt.Sprintf("> 置信度：%s | 生成时间：%s\n\n", formatConfidence(s.Confidence), time.Now().Format("2006-01-02 15:04:05")))

	sb.WriteString("## 概述\n\n")
	sb.WriteString(s.Summary)
	sb.WriteString("\n\n")

	if len(s.KeyPoints) > 0 {
		sb.WriteString("## 核心要点\n\n")
		for _, p := range s.KeyPoints {
			sb.WriteString(fmt.Sprintf("- %s\n", p))
		}
		sb.WriteString("\n")
	}

	if len(s.URLs) > 0 {
		sb.WriteString("## 访问地址\n\n")
		sb.WriteString("| 名称 | 地址 | 类型 | 是否主地址 |\n")
		sb.WriteString("|------|------|------|-----------|\n")
		for _, u := range s.URLs {
			isPrimary := "否"
			if u.IsPrimary {
				isPrimary = "✅ 是"
			}
			sb.WriteString(fmt.Sprintf("| %s | [%s](%s) | %s | %s |\n", u.Label, u.URL, u.URL, u.Type, isPrimary))
		}
		sb.WriteString("\n")
	}

	if len(s.ConfigItems) > 0 {
		sb.WriteString("## 配置项\n\n")
		sb.WriteString("| 配置键 | 值 | 说明 |\n")
		sb.WriteString("|--------|-----|------|\n")
		for _, c := range s.ConfigItems {
			sb.WriteString(fmt.Sprintf("| `%s` | `%s` | %s |\n", c.Key, c.Value, c.Description))
		}
		sb.WriteString("\n")
	}

	if len(s.Aliases) > 0 {
		sb.WriteString("## 别名/别称\n\n")
		for _, a := range s.Aliases {
			sb.WriteString(fmt.Sprintf("- %s\n", a))
		}
		sb.WriteString("\n")
	}

	if len(s.ActionItems) > 0 {
		sb.WriteString("## 待办事项\n\n")
		for _, a := range s.ActionItems {
			sb.WriteString(fmt.Sprintf("- [ ] %s\n", a))
		}
		sb.WriteString("\n")
	}

	if len(s.Tags) > 0 {
		sb.WriteString("## 标签\n\n")
		sb.WriteString(strings.Join(s.Tags, "、"))
		sb.WriteString("\n")
	}

	return sb.String()
}

func formatConfidence(c string) string {
	switch c {
	case "high":
		return "高"
	case "medium":
		return "中"
	case "low":
		return "⚠️ 低（建议人工审核）"
	default:
		return "中"
	}
}

func (w *SpaceWriter) writeKnowledgeGraph(result *analyzer.AnalysisResult, spaceID int) error {
	kgRepo := knowledgerepo.NewKnowledgeGraphRepository(repos.DB)

	// 先查询数据库中已存在的实体（按 doc_id），用于去重
	existingEntities, _ := kgRepo.FindEntitiesBySpaceID(spaceID)
	existingDocMap := make(map[int]int) // doc_id -> entity_id
	for _, e := range existingEntities {
		existingDocMap[e.DocID] = e.ID
	}

	var newEntities []knowledgemodel.KnowledgeEntity
	for _, s := range result.Summaries {
		if _, exists := existingDocMap[s.SourceDocID]; exists {
			continue // 已存在，跳过
		}
		newEntities = append(newEntities, knowledgemodel.KnowledgeEntity{
			Name:    s.SourceName,
			Type:    "document",
			DocID:   s.SourceDocID,
			SpaceID: s.SourceSpaceID,
		})
	}

	// 插入新实体
	if len(newEntities) > 0 {
		if err := kgRepo.BatchCreateEntities(newEntities); err != nil {
			logx.Infof("[SpaceWriter] BatchCreateEntities error: %v", err)
		}
	}

	// 重新加载所有实体（包含新插入的）
	allEntities, _ := kgRepo.FindEntitiesBySpaceID(spaceID)
	docToEntityID := make(map[int]int)
	for _, e := range allEntities {
		docToEntityID[e.DocID] = e.ID
	}

	// 查询已存在的关系，用于去重
	existingRelations, _ := kgRepo.FindRelationsBySpaceID(spaceID)
	existingRelSet := make(map[string]bool)
	for _, r := range existingRelations {
		key := fmt.Sprintf("%d|%d|%s", r.SourceEntityID, r.TargetEntityID, r.RelationType)
		existingRelSet[key] = true
	}

	var newRelations []knowledgemodel.KnowledgeRelation
	for _, r := range result.Relations {
		sourceEntityID := docToEntityID[r.SourceDocID]
		targetEntityID := docToEntityID[r.TargetDocID]
		if sourceEntityID == 0 || targetEntityID == 0 {
			continue
		}
		relKey := fmt.Sprintf("%d|%d|%s", sourceEntityID, targetEntityID, r.RelationType)
		if existingRelSet[relKey] {
			continue // 关系已存在，跳过
		}
		newRelations = append(newRelations, knowledgemodel.KnowledgeRelation{
			SourceEntityID: sourceEntityID,
			TargetEntityID: targetEntityID,
			RelationType:   r.RelationType,
			Description:    r.Description,
			Confidence:     0.8,
		})
		existingRelSet[relKey] = true // 防止同一次分析内重复
	}

	if len(newRelations) > 0 {
		if err := kgRepo.BatchCreateRelations(newRelations); err != nil {
			logx.Infof("[SpaceWriter] BatchCreateRelations error: %v", err)
		}
	}

	logx.Infof("[SpaceWriter] Knowledge graph written: %d new entities, %d new relations (skipped %d existing entities, %d existing relations)",
		len(newEntities), len(newRelations), len(result.Summaries)-len(newEntities), len(result.Relations)-len(newRelations))
	return nil
}
