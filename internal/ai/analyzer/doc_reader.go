package analyzer

import (
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"path/filepath"
	"strings"
	"time"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/service/doccontent"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/config"
)

type DocumentContent struct {
	DocID      int
	SpaceID    int
	SpaceName  string
	SpaceDesc  string
	DocName    string
	ParentID   int
	Path       string
	BreadCrumb string
	Sequence   int
	Content    string
	CreateTime int // 添加创建时间，用于快速扫描
	UpdateTime int
}

type ReadDiagnostics struct {
	TotalSpaces       int
	AISpaceID         int
	TargetSpaceIDs    []int
	ModifiedSince     int
	ModifiedSinceTime string
	SpaceDetails      []SpaceDiagDetail
	TotalDocs         int
	SkippedByType     int
	SkippedByDelete   int
	SkippedByTime     int
}

type SpaceDiagDetail struct {
	SpaceID   int
	SpaceName string
	Action    string // "included", "skipped-ai", "skipped-target"
	DocCount  int
	DocsAdded int
}

type DocReader struct {
	docRepo    *workspacerepo.DocumentRepository
	spaceRepo  *workspacerepo.SpaceRepository
	docStorage *utils.DocumentStorage
	aiSpaceID  int
}

func NewDocReader(aiSpaceID int) *DocReader {
	docDir := config.AppConfig.Storage.DocumentDir
	if docDir == "" {
		docDir = "./data/markdowns"
	}
	return &DocReader{
		docRepo:    workspacerepo.NewDocumentRepository(),
		spaceRepo:  workspacerepo.NewSpaceRepository(),
		docStorage: utils.NewDocumentStorageWithDir(docDir),
		aiSpaceID:  aiSpaceID,
	}
}

func (r *DocReader) ReadAllDocuments(targetSpaceIDs []int, modifiedSince int) ([]*DocumentContent, *ReadDiagnostics, error) {
	spaces, _, err := r.spaceRepo.FindAll(0, 10000, "", "", "")
	if err != nil {
		return nil, nil, fmt.Errorf("query spaces: %w", err)
	}

	diag := &ReadDiagnostics{
		TotalSpaces:       len(spaces),
		AISpaceID:         r.aiSpaceID,
		TargetSpaceIDs:    targetSpaceIDs,
		ModifiedSince:     modifiedSince,
		ModifiedSinceTime: time.Unix(int64(modifiedSince), 0).Format("2006-01-02 15:04:05"),
		SpaceDetails:      []SpaceDiagDetail{},
	}

	docNameCache := r.buildDocNameCache(spaces)

	var allDocs []*DocumentContent

	for _, space := range spaces {
		if space.ID == r.aiSpaceID {
			diag.SpaceDetails = append(diag.SpaceDetails, SpaceDiagDetail{
				SpaceID:   space.ID,
				SpaceName: space.Name,
				Action:    "skipped-ai",
			})
			continue
		}

		if len(targetSpaceIDs) > 0 {
			found := false
			for _, id := range targetSpaceIDs {
				if id == space.ID {
					found = true
					break
				}
			}
			if !found {
				diag.SpaceDetails = append(diag.SpaceDetails, SpaceDiagDetail{
					SpaceID:   space.ID,
					SpaceName: space.Name,
					Action:    "skipped-target",
				})
				continue
			}
		}

		docs, err := r.docRepo.FindBySpaceID(space.ID)
		if err != nil {
			continue
		}

		detail := SpaceDiagDetail{
			SpaceID:   space.ID,
			SpaceName: space.Name,
			Action:    "included",
			DocCount:  len(docs),
		}

		for _, doc := range docs {
			if doc.Type != workspacemodel.DocumentTypePage {
				diag.SkippedByType++
				continue
			}
			if doc.IsDelete == 1 {
				diag.SkippedByDelete++
				continue
			}
			// 修复BUG：快速扫描应该检查 CreateTime 或 UpdateTime
			if modifiedSince > 0 && doc.UpdateTime < modifiedSince && doc.CreateTime < modifiedSince {
				diag.SkippedByTime++
				continue
			}

			content := r.readContent(doc)
			breadCrumb := r.buildBreadCrumb(doc, space.Name, docNameCache)

			allDocs = append(allDocs, &DocumentContent{
				DocID:      doc.ID,
				SpaceID:    space.ID,
				SpaceName:  space.Name,
				SpaceDesc:  space.Description,
				DocName:    doc.Name,
				ParentID:   doc.ParentID,
				Path:       doc.Path,
				BreadCrumb: breadCrumb,
				Sequence:   doc.Sequence,
				Content:    content,
				CreateTime: doc.CreateTime,
				UpdateTime: doc.UpdateTime,
			})
			detail.DocsAdded++
		}
		diag.SpaceDetails = append(diag.SpaceDetails, detail)
	}

	diag.TotalDocs = len(allDocs)

	// 将诊断信息写入文件
	diagStr := fmt.Sprintf(
		"=== AI Doc Reader Diagnostics ===\nTime: %s\nTotal Spaces: %d\nAI Space ID: %d\nTarget Space IDs: %v\nModified Since: %d (%s)\nTotal Docs: %d\nSkipped by Type: %d\nSkipped by Delete: %d\nSkipped by Time: %d\n",
		time.Now().Format("2006-01-02 15:04:05"),
		diag.TotalSpaces,
		diag.AISpaceID,
		diag.TargetSpaceIDs,
		diag.ModifiedSince,
		diag.ModifiedSinceTime,
		diag.TotalDocs,
		diag.SkippedByType,
		diag.SkippedByDelete,
		diag.SkippedByTime,
	)
	logx.Infof("[DocReader] Diagnostics: %s", diagStr)

	return allDocs, diag, nil
}

// ReadSingleDocument P0-1: 直接按 docID 查询单篇文档，避免全量扫描
// 返回的 DocumentContent 包含 SpaceName / BreadCrumb 等完整上下文
func (r *DocReader) ReadSingleDocument(docID int) (*DocumentContent, error) {
	doc, err := r.docRepo.FindByID(docID)
	if err != nil || doc == nil {
		return nil, fmt.Errorf("document %d not found: %w", docID, err)
	}
	if doc.IsDelete == 1 {
		return nil, fmt.Errorf("document %d is deleted", docID)
	}

	space, err := r.spaceRepo.FindByID(doc.SpaceID)
	if err != nil || space == nil {
		return nil, fmt.Errorf("space %d not found for doc %d: %w", doc.SpaceID, docID, err)
	}

	// 构造与 ReadAllDocuments 一致的 nameCache（仅当前空间）
	spaces := []*workspacemodel.Space{space}
	docNameCache := r.buildDocNameCache(spaces)

	content := r.readContent(doc)
	breadCrumb := r.buildBreadCrumb(doc, space.Name, docNameCache)

	return &DocumentContent{
		DocID:      doc.ID,
		SpaceID:    space.ID,
		SpaceName:  space.Name,
		SpaceDesc:  space.Description,
		DocName:    doc.Name,
		ParentID:   doc.ParentID,
		Path:       doc.Path,
		BreadCrumb: breadCrumb,
		Sequence:   doc.Sequence,
		Content:    content,
		CreateTime: doc.CreateTime,
		UpdateTime: doc.UpdateTime,
	}, nil
}

// ReadDocumentsByIDs P0-1: 按 docID 列表批量查询文档（多文档分析）
// 相比 ReadSingleDocument 减少多次查询开销，复用空间缓存
func (r *DocReader) ReadDocumentsByIDs(docIDs []int) ([]*DocumentContent, error) {
	if len(docIDs) == 0 {
		return nil, nil
	}
	docs, err := r.docRepo.FindByIDs(docIDs)
	if err != nil {
		return nil, fmt.Errorf("query documents by ids: %w", err)
	}

	// 缓存空间元数据（避免重复查询）
	spaceCache := make(map[int]*workspacemodel.Space)
	docNameCache := make(map[int]string)

	var result []*DocumentContent
	for _, doc := range docs {
		if doc.IsDelete == 1 {
			continue
		}
		space, ok := spaceCache[doc.SpaceID]
		if !ok {
			space, err = r.spaceRepo.FindByID(doc.SpaceID)
			if err != nil || space == nil {
				continue
			}
			spaceCache[doc.SpaceID] = space
		}
		if _, ok := docNameCache[doc.ID]; !ok {
			docNameCache[doc.ID] = doc.Name
		}

		content := r.readContent(doc)
		breadCrumb := r.buildBreadCrumb(doc, space.Name, docNameCache)

		result = append(result, &DocumentContent{
			DocID:      doc.ID,
			SpaceID:    space.ID,
			SpaceName:  space.Name,
			SpaceDesc:  space.Description,
			DocName:    doc.Name,
			ParentID:   doc.ParentID,
			Path:       doc.Path,
			BreadCrumb: breadCrumb,
			Sequence:   doc.Sequence,
			Content:    content,
			CreateTime: doc.CreateTime,
			UpdateTime: doc.UpdateTime,
		})
	}
	return result, nil
}

func (r *DocReader) buildDocNameCache(spaces []*workspacemodel.Space) map[int]string {
	cache := make(map[int]string)
	for _, space := range spaces {
		docs, err := r.docRepo.FindBySpaceID(space.ID)
		if err != nil {
			continue
		}
		for _, doc := range docs {
			cache[doc.ID] = doc.Name
		}
	}
	return cache
}

func (r *DocReader) buildBreadCrumb(doc *workspacemodel.Document, spaceName string, nameCache map[int]string) string {
	parts := []string{spaceName}

	if doc.Path != "" && doc.Path != "0" {
		pathIDs := strings.Split(doc.Path, ",")
		for _, idStr := range pathIDs {
			if idStr == "" || idStr == "0" {
				continue
			}
			id := 0
			fmt.Sscanf(idStr, "%d", &id)
			if id > 0 {
				if name, ok := nameCache[id]; ok {
					parts = append(parts, name)
				}
			}
		}
	}

	parts = append(parts, doc.Name)
	return strings.Join(parts, " > ")
}

func (r *DocReader) readContent(doc *workspacemodel.Document) string {
	pageFile := getPageFilePath(doc, r.docRepo, r.spaceRepo, r.docStorage)
	content, err := doccontent.Get(doc.ID, r.docStorage, pageFile)
	if err != nil || content == "" {
		latestVersion, verErr := r.docRepo.GetLatestVersion(doc.ID)
		if verErr == nil && latestVersion > 0 {
			versionContent, verErr := r.docRepo.GetVersionContent(doc.ID, latestVersion)
			if verErr == nil {
				content = versionContent
			}
		}
	}
	return content
}

func getPageFilePath(doc *workspacemodel.Document, docRepo *workspacerepo.DocumentRepository, spaceRepo *workspacerepo.SpaceRepository, docStorage *utils.DocumentStorage) string {
	space, _ := spaceRepo.FindByID(doc.SpaceID)
	spaceName := fmt.Sprintf("space_%d", doc.SpaceID)
	if space != nil {
		spaceName = space.Name
	}

	if doc.Path == "0" || doc.Path == "" {
		if doc.Name == spaceName && doc.Type == workspacemodel.DocumentTypeDir {
			return filepath.Join(spaceName, utils.DocumentDefaultName+utils.DocumentPageSuffix)
		}
		return docStorage.GetPageFile(spaceName, doc.Name, doc.Type)
	}

	pathParts := strings.Split(doc.Path, ",")
	var parentPath string
	skipFirst := true
	for _, part := range pathParts {
		if part == "" || part == "0" {
			continue
		}
		parentID := 0
		fmt.Sscanf(part, "%d", &parentID)
		if parentID > 0 {
			if skipFirst {
				skipFirst = false
				continue
			}
			parentDoc, err := docRepo.FindByID(parentID)
			if err == nil {
				parentPath = parentPath + "/" + parentDoc.Name
			}
		}
	}

	if parentPath == "" {
		return docStorage.GetPageFile(spaceName, doc.Name, doc.Type)
	}
	fullPath := spaceName + parentPath
	if doc.Type == workspacemodel.DocumentTypePage {
		return fullPath + "/" + doc.Name + utils.DocumentPageSuffix
	}
	return fullPath + "/" + doc.Name + "/" + utils.DocumentDefaultName + utils.DocumentPageSuffix
}
