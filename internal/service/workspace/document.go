package workspace

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	logx "github.com/listen808/citebase/internal/log"
	socialmodel "github.com/listen808/citebase/internal/model/social"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	socialrepo "github.com/listen808/citebase/internal/repository/social"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	adminsvc "github.com/listen808/citebase/internal/service/admin"
	"github.com/listen808/citebase/internal/service/doccontent"
	searchsvc "github.com/listen808/citebase/internal/service/search"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/config"
	"github.com/listen808/citebase/internal/utils/safego"
	"gorm.io/gorm"
)

var (
	ErrDocumentNameExists   = errors.New("document name already exists in this space")
	ErrDocumentNotFound     = workspacerepo.ErrDocumentNotFound
	ErrInvalidParent        = errors.New("invalid parent document")
	ErrNoDocumentPermission = errors.New("no permission to access this document")
	ErrDocumentConflict     = errors.New("document has been modified by another user")
)

var VectorSyncFunc func(docID, spaceID int, docName, content, opType string)

// VectorSyncReindexFunc P0：重建索引用的强制通道（绕过内容指纹判重）。
// 为 nil 时重建回退到普通通道。
var VectorSyncReindexFunc func(docID, spaceID int, docName, content, opType string)

// WikiRecompileFunc 源文档内容变更后触发 Wiki 重编译。
// 在 AI 模块初始化时由 app/server 注入；为 nil 时跳过（AI 未启用）。
var WikiRecompileFunc func(docID, spaceID int)

// WikiRemoveSourceFunc 源文档下架（入回收站 / 彻底删除）后触发 Wiki 级联清理：
// 独占页删除、共享页摘除溯源、死链清理。
// 与 WikiRecompileFunc 分开注入——两者语义不同（重编译 vs 摘除），共用一个变量会误触发。
var WikiRemoveSourceFunc func(spaceID, docID int)

type DocumentService struct {
	docRepo        *workspacerepo.DocumentRepository
	colRepo        *socialrepo.CollectionRepository
	spaceService   *SpaceService
	userRepo       *userrepo.UserRepository
	spaceRepo      *workspacerepo.SpaceRepository
	docStorage     *utils.DocumentStorage
	searchService  *searchsvc.SearchService
	emailService   *adminsvc.EmailService
	followRepo     *socialrepo.FollowRepository
	configService  *adminsvc.ConfigService
	docLinkRepo    *workspacerepo.DocumentLinkRepository
	editLockRepo   *workspacerepo.DocumentEditLockRepository
	trashRepo      *workspacerepo.DocumentTrashRepository
	mentionService *MentionService
	viewLogRepo    *workspacerepo.DocumentViewLogRepository
}

func NewDocumentService() *DocumentService {
	docDir := config.AppConfig.Storage.DocumentDir
	if docDir == "" {
		docDir = "./data/markdowns"
	}

	svc := &DocumentService{
		docRepo:        workspacerepo.NewDocumentRepository(),
		colRepo:        socialrepo.NewCollectionRepository(),
		spaceService:   NewSpaceService(),
		userRepo:       userrepo.NewUserRepository(),
		spaceRepo:      workspacerepo.NewSpaceRepository(),
		docStorage:     utils.NewDocumentStorageWithDir(docDir),
		searchService:  searchsvc.NewSearchService(),
		emailService:   adminsvc.NewEmailService(),
		followRepo:     socialrepo.NewFollowRepository(),
		configService:  adminsvc.NewConfigService(),
		docLinkRepo:    workspacerepo.NewDocumentLinkRepository(),
		editLockRepo:   workspacerepo.NewDocumentEditLockRepository(),
		trashRepo:      workspacerepo.NewDocumentTrashRepository(),
		mentionService: NewMentionService(),
		viewLogRepo:    workspacerepo.NewDocumentViewLogRepository(),
	}
	if err := svc.docStorage.EnsureBaseDirs(); err != nil {
		logx.Warnf("failed to ensure base dirs: %v", err)
	}
	return svc
}

func (s *DocumentService) CreateDocument(userID int, req *workspacemodel.CreateDocumentRequest) (*workspacemodel.Document, error) {
	privilege, err := s.spaceService.GetUserSpacePrivilege(req.SpaceID, userID)
	if err != nil {
		return nil, err
	}
	if privilege < 1 {
		return nil, ErrNoDocumentPermission
	}

	if req.ParentID > 0 {
		parentDoc, err := s.docRepo.FindByID(req.ParentID)
		if err != nil {
			return nil, err
		}
		if parentDoc.SpaceID != req.SpaceID {
			return nil, ErrInvalidParent
		}
		if parentDoc.Type != workspacemodel.DocumentTypeDir {
			return nil, errors.New("parent must be a directory")
		}
	}

	existingDoc, _ := s.docRepo.FindByNameAndParentID(req.Name, req.ParentID, req.SpaceID)
	if existingDoc != nil {
		return nil, ErrDocumentNameExists
	}

	path := "0"
	if req.ParentID > 0 {
		parentDoc, _ := s.docRepo.FindByID(req.ParentID)
		if parentDoc != nil {
			path = parentDoc.Path + fmt.Sprintf(",%d", req.ParentID)
		}
	}

	if req.Sequence <= 0 {
		maxSeq, err := s.docRepo.MaxSequenceByParent(req.ParentID, req.SpaceID)
		if err == nil {
			req.Sequence = maxSeq + 1
		} else {
			req.Sequence = 1
		}
	}

	doc := &workspacemodel.Document{
		ParentID:     req.ParentID,
		SpaceID:      req.SpaceID,
		Name:         req.Name,
		Type:         req.Type,
		Path:         path,
		Sequence:     req.Sequence,
		CreateUserID: userID,
		EditUserID:   userID,
		IsDelete:     0,
	}

	// 先写文件（不可逆操作），避免 DB 写入后文件失败导致残留
	// db 模式下正文随文档行写入同一事务，文件分支保持原样（回滚只需切回 file）
	var pageFile string
	var contentForVersion string
	if req.Type == workspacemodel.DocumentTypePage {
		contentForVersion = req.Content
		if contentForVersion == "" {
			contentForVersion = "# " + req.Name + "\n\n"
		}
		if !doccontent.UseDB() {
			pageFile = s.getPageFile(doc)
			if err := s.docStorage.SetContent(pageFile, contentForVersion); err != nil {
				logx.Errorf("Failed to set document content for %s: %v", pageFile, err)
				return nil, errors.New("failed to save document content")
			}
		}
	}

	// DB 事务：创建文档 + 正文 + 版本，保证原子性
	err = s.docRepo.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(doc).Error; err != nil {
			return err
		}
		if req.Type == workspacemodel.DocumentTypePage {
			if err := doccontent.SetTx(tx, doc.ID, contentForVersion); err != nil {
				return err
			}
			version := &workspacemodel.DocumentVersion{
				DocumentID: doc.ID,
				Version:    1,
				Content:    contentForVersion,
				ChangeLog:  "初始版本",
				CreatedBy:  userID,
			}
			if err := tx.Create(version).Error; err != nil {
				return err
			}
		}
		return nil
	})

	if err != nil {
		// 回滚：删除已创建的文件（db 模式由事务自身回滚）
		if req.Type == workspacemodel.DocumentTypePage && pageFile != "" {
			if delErr := s.docStorage.Delete(pageFile, req.Type); delErr != nil {
				logx.Errorf("Failed to rollback file %s: %v", pageFile, delErr)
			}
		}
		return nil, err
	}

	safego.Go("triggerIndex", func() { s.triggerIndex(doc) })

	if req.Type == workspacemodel.DocumentTypePage && req.Content != "" {
		s.parseDocumentLinksAsync(doc.ID, doc.SpaceID, req.Content)
		safego.Go("ResolveAndSave", func() { _ = s.mentionService.ResolveAndSave(doc.ID, userID, req.Content) })
	}

	FireWebhook(workspacemodel.WebhookEventDocCreated, int64(doc.SpaceID), func(p *WebhookEventPayload) {
		p.DocumentID = int64(doc.ID)
		p.DocumentName = doc.Name
		p.ActorID = int64(userID)
	})

	return doc, nil
}

func (s *DocumentService) GetDocumentByID(id int) (*workspacemodel.DocumentResponse, error) {
	doc, err := s.docRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if doc.IsDelete == 1 {
		return nil, workspacerepo.ErrDocumentNotFound
	}

	spaceName := "根目录"
	if space, err := s.spaceRepo.FindByIDCached(doc.SpaceID); err == nil && space != nil {
		spaceName = space.Name
	}

	response := doc.ToResponse(spaceName)

	userIDs := make([]int, 0, 2)
	if doc.CreateUserID > 0 {
		userIDs = append(userIDs, doc.CreateUserID)
	}
	if doc.EditUserID > 0 && doc.EditUserID != doc.CreateUserID {
		userIDs = append(userIDs, doc.EditUserID)
	}
	if len(userIDs) > 0 {
		users, _ := s.userRepo.FindByIDs(userIDs)
		for _, u := range users {
			if u.ID == doc.CreateUserID {
				response.CreatorName = u.GivenName
			}
			if u.ID == doc.EditUserID {
				response.EditorName = u.GivenName
			}
		}
	}

	if doc.Type == workspacemodel.DocumentTypePage {
		latestVersion, _ := s.docRepo.GetLatestVersion(doc.ID)
		response.Version = latestVersion
	}

	return &response, nil
}

func (s *DocumentService) GetDocumentRawByID(id int) (*workspacemodel.Document, error) {
	return s.docRepo.FindByID(id)
}

// GetDocumentFull 一次性返回文档元数据、内容、路径，减少前端 3 次 HTTP 往返为 1 次
func (s *DocumentService) GetDocumentFull(id int) (*workspacemodel.DocumentFullResponse, error) {
	doc, err := s.docRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if doc.IsDelete == 1 {
		return nil, workspacerepo.ErrDocumentNotFound
	}

	spaceName := "根目录"
	if space, err := s.spaceRepo.FindByIDCached(doc.SpaceID); err == nil && space != nil {
		spaceName = space.Name
	}

	resp := doc.ToResponse(spaceName)

	// 批量查询用户名
	userIDs := make([]int, 0, 2)
	if doc.CreateUserID > 0 {
		userIDs = append(userIDs, doc.CreateUserID)
	}
	if doc.EditUserID > 0 && doc.EditUserID != doc.CreateUserID {
		userIDs = append(userIDs, doc.EditUserID)
	}
	if len(userIDs) > 0 {
		users, _ := s.userRepo.FindByIDs(userIDs)
		for _, u := range users {
			if u.ID == doc.CreateUserID {
				resp.CreatorName = u.GivenName
			}
			if u.ID == doc.EditUserID {
				resp.EditorName = u.GivenName
			}
		}
	}

	if doc.Type == workspacemodel.DocumentTypePage {
		latestVersion, _ := s.docRepo.GetLatestVersion(doc.ID)
		resp.Version = latestVersion
	}

	// 读取内容
	// 目录（type=2）本身也是 md 文档，其内容存储在「目录名/README.md」中，
	// 点击目录查看时应展示其内容，而非仅作为容器返回空内容
	content := ""
	pageFile := s.getPageFile(doc)
	content, _ = doccontent.Get(doc.ID, s.docStorage, pageFile)

	// 查询路径
	path := make([]map[string]interface{}, 0)
	pathParts := strings.Split(doc.Path, ",")
	nodeIDs := make([]int, 0, len(pathParts))
	for _, part := range pathParts {
		part = strings.TrimSpace(part)
		if part == "" || part == "0" {
			continue
		}
		nodeID := 0
		fmt.Sscanf(part, "%d", &nodeID)
		if nodeID > 0 {
			nodeIDs = append(nodeIDs, nodeID)
		}
	}
	if len(nodeIDs) > 0 {
		nodes, _ := s.docRepo.FindByIDs(nodeIDs)
		nodeMap := make(map[int]*workspacemodel.Document, len(nodes))
		for _, n := range nodes {
			nodeMap[n.ID] = n
		}
		for _, nodeID := range nodeIDs {
			if node, ok := nodeMap[nodeID]; ok {
				path = append(path, map[string]interface{}{
					"id":   node.ID,
					"name": node.Name,
				})
			}
		}
	}
	path = append(path, map[string]interface{}{
		"id":   doc.ID,
		"name": doc.Name,
	})

	return &workspacemodel.DocumentFullResponse{
		Document: resp,
		Content:  content,
		Path:     path,
	}, nil
}

// #7 草稿/发布分离

// PublishDocument 发布文档（draft → published）
func (s *DocumentService) PublishDocument(docID, userID int) error {
	doc, err := s.docRepo.FindByID(docID)
	if err != nil {
		return err
	}
	if doc.Status == workspacemodel.DocumentStatusPublished {
		return nil // 已发布，幂等
	}
	return s.docRepo.SetStatus(docID, workspacemodel.DocumentStatusPublished, userID)
}

// UnpublishDocument 转为草稿（published → draft）
func (s *DocumentService) UnpublishDocument(docID, userID int) error {
	doc, err := s.docRepo.FindByID(docID)
	if err != nil {
		return err
	}
	if doc.Status == workspacemodel.DocumentStatusDraft {
		return nil // 已是草稿，幂等
	}
	return s.docRepo.SetStatus(docID, workspacemodel.DocumentStatusDraft, userID)
}

// GetDraftsByAuthor 获取用户创建的草稿列表
func (s *DocumentService) GetDraftsByAuthor(userID int) ([]*workspacemodel.Document, error) {
	return s.docRepo.FindDraftsByAuthor(userID)
}

func (s *DocumentService) GetTrashByDocumentID(docID int) (*workspacemodel.DocumentTrash, error) {
	return s.trashRepo.FindByDocumentID(docID)
}

func (s *DocumentService) GetDocumentTree(spaceID int) ([]*workspacemodel.DocumentResponse, error) {
	allDocs, err := s.docRepo.FindBySpaceID(spaceID)
	if err != nil {
		return nil, err
	}

	respMap := make(map[int]*workspacemodel.DocumentResponse, len(allDocs))
	childrenMap := make(map[int][]*workspacemodel.DocumentResponse)

	for _, doc := range allDocs {
		resp := doc.ToResponse()
		respMap[doc.ID] = &resp
		childrenMap[doc.ParentID] = append(childrenMap[doc.ParentID], &resp)
	}

	var buildChildren func(parentID int)
	buildChildren = func(parentID int) {
		children := childrenMap[parentID]
		for _, child := range children {
			if child.Type == workspacemodel.DocumentTypeDir {
				child.HasChildren = len(childrenMap[child.ID]) > 0
				child.Children = childrenMap[child.ID]
				buildChildren(child.ID)
			}
		}
	}

	roots := childrenMap[0]
	for _, root := range roots {
		if root.Type == workspacemodel.DocumentTypeDir {
			root.HasChildren = len(childrenMap[root.ID]) > 0
			root.Children = childrenMap[root.ID]
			buildChildren(root.ID)
		}
	}

	return roots, nil
}

func (s *DocumentService) GetDocumentChildren(parentID int) ([]*workspacemodel.DocumentResponse, error) {
	children, err := s.docRepo.FindByParentID(parentID)
	if err != nil {
		logx.Warnf("GetDocumentChildren FindByParentID error: %v", err)
		return nil, err
	}

	dirIDs := make([]int, 0)
	for _, child := range children {
		if child.Type == workspacemodel.DocumentTypeDir {
			dirIDs = append(dirIDs, child.ID)
		}
	}

	hasChildrenMap := make(map[int]bool, len(dirIDs))
	if len(dirIDs) > 0 {
		childCounts, _ := s.docRepo.CountChildrenByParents(dirIDs)
		for _, cc := range childCounts {
			hasChildrenMap[cc.ParentID] = cc.Count > 0
		}
	}

	responses := make([]*workspacemodel.DocumentResponse, len(children))
	for i, child := range children {
		resp := child.ToResponse()
		if child.Type == workspacemodel.DocumentTypeDir {
			resp.HasChildren = hasChildrenMap[child.ID]
		} else {
			resp.HasChildren = false
		}
		responses[i] = &resp
	}
	return responses, nil
}

func (s *DocumentService) GetRootDocuments(spaceID int) ([]*workspacemodel.DocumentResponse, error) {
	roots, err := s.docRepo.FindRootDocuments(spaceID)
	if err != nil {
		return nil, err
	}

	dirIDs := make([]int, 0)
	for _, root := range roots {
		if root.Type == workspacemodel.DocumentTypeDir {
			dirIDs = append(dirIDs, root.ID)
		}
	}

	hasChildrenMap := make(map[int]bool, len(dirIDs))
	if len(dirIDs) > 0 {
		childCounts, _ := s.docRepo.CountChildrenByParents(dirIDs)
		for _, cc := range childCounts {
			hasChildrenMap[cc.ParentID] = cc.Count > 0
		}
	}

	responses := make([]*workspacemodel.DocumentResponse, len(roots))
	for i, root := range roots {
		resp := root.ToResponse()
		if root.Type == workspacemodel.DocumentTypeDir {
			resp.HasChildren = hasChildrenMap[root.ID]
		} else {
			resp.HasChildren = false
		}
		responses[i] = &resp
	}
	return responses, nil
}

func (s *DocumentService) buildDocumentTree(parentID int) ([]*workspacemodel.DocumentResponse, error) {
	children, err := s.docRepo.FindByParentID(parentID)
	if err != nil {
		return nil, err
	}

	responses := make([]*workspacemodel.DocumentResponse, len(children))
	for i, child := range children {
		response := child.ToResponse()
		if child.Type == workspacemodel.DocumentTypeDir {
			grandChildren, err := s.buildDocumentTree(child.ID)
			if err != nil {
				return nil, err
			}
			response.Children = grandChildren
		}
		responses[i] = &response
	}

	return responses, nil
}

func (s *DocumentService) UpdateDocument(id int, userID int, req *workspacemodel.UpdateDocumentRequest) (*workspacemodel.Document, error) {
	doc, err := s.docRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if req.LastUpdateTime > 0 && doc.UpdateTime > req.LastUpdateTime {
		return nil, ErrDocumentConflict
	}

	privilege, err := s.spaceService.GetUserSpacePrivilege(doc.SpaceID, userID)
	if err != nil {
		return nil, err
	}
	if privilege < 1 {
		return nil, ErrNoDocumentPermission
	}

	if req.Name != "" && req.Name != doc.Name {
		existingDoc, _ := s.docRepo.FindByNameAndParentID(req.Name, doc.ParentID, doc.SpaceID)
		if existingDoc != nil && existingDoc.ID != id {
			return nil, ErrDocumentNameExists
		}

		oldPageFile := s.getPageFile(doc)

		oldName := doc.Name
		doc.Name = req.Name
		newPageFile := s.getPageFile(doc)
		doc.Name = oldName

		if oldPageFile != newPageFile {
			if err := doccontent.Rename(s.docStorage, oldPageFile, newPageFile, doc.Type); err != nil {
				logx.Warnf("Failed to rename document file from %s to %s: %v (file may not exist)", oldPageFile, newPageFile, err)
			}
		}

		doc.Name = req.Name
	}
	if req.Sequence > 0 {
		doc.Sequence = req.Sequence
	}
	if req.Icon != "" || req.CoverImage != "" {
		if req.Icon != "" {
			doc.Icon = req.Icon
		}
		if req.CoverImage != "" {
			doc.CoverImage = req.CoverImage
		}
	}
	doc.EditUserID = userID

	var oldContent string
	var contentChanged bool
	// oldContentKnown：能不能把文件回滚回去，取决于我们是否真读到了旧正文。
	// 原先写的是 `oldContent, _ = GetContent(...)`，读失败时 oldContent 恒为 ""，
	// 而下面那句 `oldContent != ""` 就成了"回滚补偿永远不触发"的开关 ——
	// 数据库没改、文件已经改了，两边各说各话。
	var oldContentKnown bool
	var pageFile string
	var pendingVersion *workspacemodel.DocumentVersion
	if req.Content != "" {
		pageFile = s.getPageFile(doc)
		prev, readErr := doccontent.Get(id, s.docStorage, pageFile)
		if readErr != nil {
			logx.Warnf("Failed to read previous content for %s（文档行写入失败时将无法回滚文件）: %v", pageFile, readErr)
		} else {
			oldContent = prev
			oldContentKnown = true
		}
		contentChanged = true

		if err := doccontent.Set(id, s.docStorage, pageFile, req.Content); err != nil {
			logx.Errorf("Failed to update document content for %s: %v", pageFile, err)
			return nil, errors.New("failed to save document content")
		}

		if req.ChangeLog != "" {
			latestVersion, vErr := s.docRepo.GetLatestVersion(id)
			if vErr != nil {
				// 拿不到最新版本号就不能接着编号：从 1 重新开始会和已有版本撞号，
				// 而"回滚到版本 N"取的是第一条命中 —— 撞了号就会滚错内容。
				if rollbackErr := s.restorePageFile(pageFile, oldContent, oldContentKnown, id); rollbackErr != nil {
					logx.Errorf("Failed to rollback document content for %s: %v", pageFile, rollbackErr)
				}
				return nil, fmt.Errorf("failed to resolve latest version for doc %d: %w", id, vErr)
			}
			pendingVersion = &workspacemodel.DocumentVersion{
				DocumentID: id,
				Version:    latestVersion + 1,
				Content:    req.Content,
				ChangeLog:  req.ChangeLog,
				CreatedBy:  userID,
			}
		}
	}

	if err := s.docRepo.UpdateWithVersion(doc, pendingVersion); err != nil {
		if contentChanged {
			if rollbackErr := s.restorePageFile(pageFile, oldContent, oldContentKnown, id); rollbackErr != nil {
				logx.Errorf("Failed to rollback document content: %v", rollbackErr)
			}
		}
		return nil, err
	}

	safego.Go("triggerIndex", func() { s.triggerIndex(doc) })

	// 向量索引同步已禁用：仅允许定时任务和手动触发
	// go s.triggerVectorSync(doc, "update")

	if req.Content != "" {
		s.parseDocumentLinksAsync(doc.ID, doc.SpaceID, req.Content)
		safego.Go("ResolveAndSave", func() { _ = s.mentionService.ResolveAndSave(doc.ID, userID, req.Content) })

		// P1 #3: 源文档内容变更 → 触发 Wiki 重编译（如果该文档是 wiki 页面来源）
		if doc.IsImmutable == 1 && WikiRecompileFunc != nil {
			safego.Go("wikiRecompile", func() { WikiRecompileFunc(doc.ID, doc.SpaceID) })
		}
	}

	if req.ChangeLog != "" {
		safego.Go("notifyDocumentFollowers", func() { s.notifyDocumentFollowers(doc, userID, req.ChangeLog) })
	}

	FireWebhook(workspacemodel.WebhookEventDocUpdated, int64(doc.SpaceID), func(p *WebhookEventPayload) {
		p.DocumentID = int64(doc.ID)
		p.DocumentName = doc.Name
		p.ActorID = int64(userID)
	})

	return doc, nil
}

// restorePageFile 把正文文件写回改动前的内容。
//
// oldContentKnown 是这条函数的全部意义：没读到旧正文时**不要**用空串去覆盖文件
// （那等于把用户的新内容连同旧内容一起清掉，比不回滚更糟），
// 但也不能像原先那样用 `oldContent != ""` 一句话把这事糊过去 ——
// 不回滚就意味着磁盘与数据库分了家，必须说出来，否则下一次读文件的人无从知道。
func (s *DocumentService) restorePageFile(pageFile, oldContent string, known bool, docID int) error {
	if !known {
		logx.Errorf("文档 %d 的正文文件 %s 无法回滚：改动前的内容没读到，数据库与文件可能已不一致（请重新保存一次该文档）", docID, pageFile)
		return nil
	}
	if pageFile == "" {
		return nil
	}
	return doccontent.Set(docID, s.docStorage, pageFile, oldContent)
}

func (s *DocumentService) DeleteDocument(id int, userID int) error {
	return s.MoveToTrash(id, userID)
}

func (s *DocumentService) MoveToTrash(id int, userID int) error {
	doc, err := s.docRepo.FindByID(id)
	if err != nil {
		return err
	}

	privilege, err := s.spaceService.GetUserSpacePrivilege(doc.SpaceID, userID)
	if err != nil {
		return err
	}
	if privilege < 2 && doc.CreateUserID != userID {
		return ErrNoDocumentPermission
	}

	var docsToTrash []*workspacemodel.Document
	docsToTrash = append(docsToTrash, doc)
	if doc.Type == workspacemodel.DocumentTypeDir {
		s.collectDocumentsRecursive(id, &docsToTrash)
	}

	err = s.docRepo.Transaction(func(tx *gorm.DB) error {
		now := int(time.Now().Unix())
		for _, d := range docsToTrash {
			contentPath := ""
			if d.Type == workspacemodel.DocumentTypePage {
				contentPath = s.getPageFile(d)
			}
			trash := &workspacemodel.DocumentTrash{
				DocumentID:  d.ID,
				ParentID:    d.ParentID,
				SpaceID:     d.SpaceID,
				Name:        d.Name,
				Type:        d.Type,
				Path:        d.Path,
				Sequence:    d.Sequence,
				ContentPath: contentPath,
				DeletedBy:   userID,
				DeletedAt:   now,
				DeleteLog:   "",
			}
			if err := tx.Create(trash).Error; err != nil {
				return err
			}
			// 同时改写墓碑名：uk_space_parent_name 对已删除行仍生效，保留原名会让
			// 同父新建同名文档撞 1062（名称原文保存在 DocumentTrash 行，恢复时写回）
			if err := tx.Model(&workspacemodel.Document{}).Where("document_id = ?", d.ID).Updates(map[string]interface{}{
				"is_delete": 1,
				"name":      workspacerepo.TombstoneName(d.Name, d.ID),
			}).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}

	// 目录入回收站时全部后代都要移除检索索引：旧实现只删顶层 id 的索引，
	// 已软删的后代正文仍会被全文检索命中（内容泄露 + 点击 404）。
	go func(docs []*workspacemodel.Document) {
		defer func() { _ = recover() }()
		for _, d := range docs {
			if err := s.searchService.DeleteDocumentIndex(d.ID); err != nil {
				logx.Warnf("failed to delete search index for document %d: %v", d.ID, err)
			}
		}
	}(docsToTrash)

	FireWebhook(workspacemodel.WebhookEventDocDeleted, int64(doc.SpaceID), func(p *WebhookEventPayload) {
		p.DocumentID = int64(doc.ID)
		p.DocumentName = doc.Name
		p.ActorID = int64(userID)
	})

	// 文档入回收站即从可检索语料里消失，基于它编译出的知识页不能再对查询可见：
	// 独占页删除、共享页摘除该源、死链清理（治理规则见 internal/ai/wiki/cascade.go）。
	if WikiRemoveSourceFunc != nil {
		for _, d := range docsToTrash {
			if d.Type != workspacemodel.DocumentTypePage {
				continue
			}
			safego.Go("wikiRemoveSource", func() { WikiRemoveSourceFunc(d.SpaceID, d.ID) })
		}
	}

	return nil
}

func (s *DocumentService) RestoreDocument(id int, userID int) error {
	trash, err := s.trashRepo.FindByDocumentID(id)
	if err != nil {
		return err
	}

	privilege, err := s.spaceService.GetUserSpacePrivilege(trash.SpaceID, userID)
	if err != nil {
		return err
	}
	if privilege < 2 && trash.DeletedBy != userID {
		return ErrNoDocumentPermission
	}

	parentID := trash.ParentID
	parentPath := "0"
	if parentID > 0 {
		parentDoc, err := s.docRepo.FindByID(parentID)
		if err != nil {
			// 父文档已被删除，恢复到根目录
			parentID = 0
			parentPath = "0"
		} else {
			parentPath = parentDoc.Path + "," + strconv.Itoa(parentID)
		}
	}

	// 获取新父节点下的最大 sequence，避免排序冲突
	maxSeq, err := s.docRepo.MaxSequenceByParent(parentID, trash.SpaceID)
	if err != nil {
		maxSeq = 0
	}
	newSeq := maxSeq + 1

	err = s.docRepo.Transaction(func(tx *gorm.DB) error {
		// 恢复时写回回收站保存的原始名称（入回收站时已改为墓碑名以让出唯一键）。
		// 若同名文档已被重建，这里会撞 1062 并整体回滚，文档留在回收站而不是半恢复状态。
		if err := tx.Model(&workspacemodel.Document{}).Where("document_id = ?", id).Updates(map[string]interface{}{
			"is_delete": 0,
			"name":      trash.Name,
			"parent_id": parentID,
			"path":      parentPath,
			"sequence":  newSeq,
		}).Error; err != nil {
			return err
		}
		if err := tx.Where("document_id = ?", id).Delete(&workspacemodel.DocumentTrash{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	// 恢复文档后触发重新索引（回收站删除了索引，恢复后需重新添加）
	doc, _ := s.docRepo.FindByID(id)
	if doc != nil {
		safego.Go("triggerIndex", func() { s.triggerIndex(doc) })
	}

	return nil
}

func (s *DocumentService) PermanentDeleteDocument(id int, userID int) error {
	doc, err := s.docRepo.FindByID(id)
	if err != nil {
		// 允许从回收站彻底删除，文档本身已被软删除
		if err != ErrDocumentNotFound {
			return err
		}
		doc = nil
	}

	trash, err := s.trashRepo.FindByDocumentID(id)
	if err != nil {
		return err
	}

	spaceID := trash.SpaceID
	if doc != nil {
		spaceID = doc.SpaceID
	}

	privilege, err := s.spaceService.GetUserSpacePrivilege(spaceID, userID)
	if err != nil {
		return err
	}
	if privilege < 2 && trash.DeletedBy != userID {
		return ErrNoDocumentPermission
	}

	var docsToDelete []*workspacemodel.Document
	if doc != nil {
		docsToDelete = append(docsToDelete, doc)
		if doc.Type == workspacemodel.DocumentTypeDir {
			s.collectDocumentsRecursive(id, &docsToDelete)
		}
	}

	// 先快照各文档入回收站时记录的正文路径：软删后文档 name 已改为墓碑名，
	// 再按 getPageFile 现算会得到错误路径、导致磁盘文件删不掉（泄露残留）。
	contentPaths := make(map[int]string, len(docsToDelete))
	for _, d := range docsToDelete {
		contentPaths[d.ID] = ""
		if d.Type == workspacemodel.DocumentTypePage {
			if t, terr := s.trashRepo.FindByDocumentID(d.ID); terr == nil && t != nil && t.ContentPath != "" {
				contentPaths[d.ID] = t.ContentPath
			} else {
				contentPaths[d.ID] = s.getPageFile(d)
			}
		}
	}

	// 墓碑行（DocumentTrash）按"每个入过回收站的文档一行"写（见 MoveToTrash），
	// 所以彻底删除也必须按同一批文档清干净。原先只删顶层 id 那一行：
	// 目录被彻底删除后，它全部后代的回收站条目还留在列表里，而正文文件已经没了 ——
	// 用户看得见、点恢复拿到的是一个空文档，且回收站里再也清不掉（记录已不是"可恢复"状态）。
	trashIDs := make([]int, 0, len(docsToDelete))
	trashIDs = append(trashIDs, id)
	for _, d := range docsToDelete {
		if d.ID != id {
			trashIDs = append(trashIDs, d.ID)
		}
	}

	err = s.docRepo.Transaction(func(tx *gorm.DB) error {
		if err := s.docRepo.DeleteDocumentRecursive(tx, id); err != nil {
			return err
		}
		if err := tx.Where("document_id IN ?", trashIDs).Delete(&workspacemodel.DocumentTrash{}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return err
	}

	for _, d := range docsToDelete {
		pageFile := contentPaths[d.ID]
		if pageFile == "" {
			continue
		}
		if err := doccontent.Delete(d.ID, s.docStorage, pageFile, d.Type); err != nil {
			logx.Warnf("failed to delete document content for %s: %v", pageFile, err)
		}
	}

	// db 模式：上面只覆盖了还能查到的文档；回收站里已软删的文档（FindByID 返回 NotFound）
	// 同样要清掉正文行，否则彻底删除后 aw_document_content 永久残留。
	if doccontent.UseDB() {
		for _, docID := range trashIDs {
			if err := doccontent.Delete(docID, s.docStorage, "", 0); err != nil {
				logx.Warnf("failed to delete content row of document %d: %v", docID, err)
			}
		}
	}

	// 附件/图片不跟正文一起删：旧实现只清了正文与文档行，aw_attachment 记录
	// 和磁盘文件永久残留（DeleteByDocumentID 一直无人调用）。这里按同一批文档清理。
	attachmentRepo := workspacerepo.NewAttachmentRepository()
	for _, docID := range trashIDs {
		atts, attErr := attachmentRepo.FindByDocumentID(docID)
		if attErr != nil {
			logx.Warnf("failed to load attachments of document %d: %v", docID, attErr)
			continue
		}
		for _, att := range atts {
			if fullPath := ResolveAttachmentPath(att.Path); fullPath != "" {
				if rmErr := os.Remove(fullPath); rmErr != nil && !os.IsNotExist(rmErr) {
					logx.Warnf("failed to delete attachment file %s: %v", fullPath, rmErr)
				}
			}
			if delErr := attachmentRepo.Delete(att.ID); delErr != nil {
				logx.Warnf("failed to delete attachment record %d: %v", att.ID, delErr)
			}
		}
	}

	if err := s.colRepo.DeleteByResourceID(socialmodel.CollectionTypeDocument, id); err != nil {
		logx.Warnf("failed to delete collections for document %d: %v", id, err)
	}

	for _, d := range docsToDelete {
		_ = s.docLinkRepo.DeleteBySource(d.ID)
		_ = s.docLinkRepo.DeleteByTarget(d.ID)
	}

	// 彻底删除后同一套 wiki 级联（trashIDs 覆盖目录下的全部后代，包括此前已软删的）。
	if WikiRemoveSourceFunc != nil {
		drainedSpaceID := spaceID
		for _, docID := range trashIDs {
			docID := docID
			safego.Go("wikiRemoveSource", func() { WikiRemoveSourceFunc(drainedSpaceID, docID) })
		}
	}

	// 彻底删除同样要清理全部后代的检索索引（旧实现只删顶层 id，残留私有正文可被搜到）
	go func(docs []*workspacemodel.Document) {
		defer func() { _ = recover() }()
		for _, d := range docs {
			if err := s.searchService.DeleteDocumentIndex(d.ID); err != nil {
				logx.Warnf("failed to delete search index for document %d: %v", d.ID, err)
			}
		}
	}(docsToDelete)

	return nil
}

func (s *DocumentService) GetSpaceTrash(spaceID int, userID int) ([]*workspacemodel.DocumentTrash, error) {
	privilege, err := s.spaceService.GetUserSpacePrivilege(spaceID, userID)
	if err != nil {
		return nil, err
	}
	if privilege < 1 {
		return nil, ErrNoDocumentPermission
	}
	return s.trashRepo.FindBySpaceID(spaceID)
}

func (s *DocumentService) collectDocumentsRecursive(parentID int, docs *[]*workspacemodel.Document) {
	// 批量加载：先获取该空间所有文档，在内存中构建树，避免 N+1 查询
	if len(*docs) == 0 {
		return
	}
	spaceID := (*docs)[0].SpaceID
	allDocs, err := s.docRepo.FindBySpaceID(spaceID)
	if err != nil || len(allDocs) == 0 {
		return
	}
	// 构建 parentID → children 映射
	childrenMap := make(map[int][]*workspacemodel.Document)
	for _, doc := range allDocs {
		childrenMap[doc.ParentID] = append(childrenMap[doc.ParentID], doc)
	}
	// BFS 收集所有后代
	queue := []int{parentID}
	visited := make(map[int]bool)
	visited[parentID] = true
	for len(queue) > 0 {
		pid := queue[0]
		queue = queue[1:]
		for _, child := range childrenMap[pid] {
			if !visited[child.ID] {
				visited[child.ID] = true
				*docs = append(*docs, child)
				if child.Type == workspacemodel.DocumentTypeDir {
					queue = append(queue, child.ID)
				}
			}
		}
	}
}

func (s *DocumentService) MoveDocument(id int, userID int, req *workspacemodel.MoveDocumentRequest) error {
	doc, err := s.docRepo.FindByID(id)
	if err != nil {
		return err
	}

	privilege, err := s.spaceService.GetUserSpacePrivilege(req.SpaceID, userID)
	if err != nil {
		return err
	}
	if privilege < 1 {
		return ErrNoDocumentPermission
	}

	// 目标空间有权限还不够：源文档所在空间同样必须可访问，
	// 否则任何人都能把别人的文档移动或复制到自己的目标空间（跨空间窃取）。
	sourcePrivilege, err := s.spaceService.GetUserSpacePrivilege(doc.SpaceID, userID)
	if err != nil {
		return err
	}
	if sourcePrivilege < 1 {
		return ErrNoDocumentPermission
	}

	if req.ParentID > 0 {
		parentDoc, err := s.docRepo.FindByID(req.ParentID)
		if err != nil {
			return err
		}
		if parentDoc.SpaceID != req.SpaceID {
			return ErrInvalidParent
		}
		if parentDoc.Type != workspacemodel.DocumentTypeDir {
			return errors.New("parent must be a directory")
		}

		if doc.Type == workspacemodel.DocumentTypeDir {
			descendants, err := s.docRepo.FindDescendants(id)
			if err != nil {
				return errors.New("failed to check descendants for circular reference")
			}
			for _, desc := range descendants {
				if desc.ID == req.ParentID {
					return errors.New("cannot move document into its own descendant")
				}
			}
		}
	}

	newPath := "0"
	if req.ParentID > 0 {
		parentDoc, _ := s.docRepo.FindByID(req.ParentID)
		if parentDoc != nil {
			newPath = parentDoc.Path + fmt.Sprintf(",%d", req.ParentID)
		}
	}

	// 记录迁移前的文件路径（基于旧 SpaceID 和旧 Path 计算）
	oldPageFile := s.getPageFile(doc)

	oldPath := doc.Path
	doc.ParentID = req.ParentID
	doc.SpaceID = req.SpaceID
	doc.Path = newPath

	// 文档自身更新与后代 Path 批量更新必须在同一事务：
	// 旧实现是两次独立写且失败仅 Warnf，会留下「父文档已移动、目录后代 Path 仍是旧前缀」的半移动状态，
	// getPageFilePath 据此拼出的正文路径错位（树里可见但读不出正文，只能重刷索引自愈）。
	if err := s.docRepo.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&workspacemodel.Document{}).Where("document_id = ?", doc.ID).Updates(map[string]interface{}{
			"parent_id":    doc.ParentID,
			"space_id":     doc.SpaceID,
			"path":         doc.Path,
			"edit_user_id": doc.EditUserID,
			"update_time":  int(time.Now().Unix()),
		}).Error; err != nil {
			return err
		}
		if doc.Type == workspacemodel.DocumentTypeDir {
			descendants, err := s.docRepo.FindDescendants(id)
			if err != nil {
				return err
			}
			if len(descendants) > 0 {
				newPrefix := fmt.Sprintf("%s,%d", newPath, id)
				oldPrefix := fmt.Sprintf("%s,%d", oldPath, id)
				for _, desc := range descendants {
					updatedPath := replacePathPrefix(desc.Path, oldPrefix, newPrefix)
					if updatedPath == desc.Path {
						continue
					}
					if err := tx.Model(&workspacemodel.Document{}).Where("document_id = ?", desc.ID).
						Update("path", updatedPath).Error; err != nil {
						return err
					}
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// 跨空间/跨目录移动时迁移文件系统中的 markdown 文件
	// 目录类型：os.Rename 会递归移动整个目录（含子文件），无需逐个迁移后代
	newPageFile := s.getPageFile(doc)
	if oldPageFile != newPageFile {
		// 文件迁移失败必须上抛：DB 已提交而文件未移动，静默返回成功会让用户以为一切正常，
		// 直到读正文才炸。显式报错让管理员感知不一致并处理（重命名/重试）。
		if err := doccontent.Rename(s.docStorage, oldPageFile, newPageFile, doc.Type); err != nil {
			logx.Errorf("move document %d: DB 已更新但文件迁移失败 %s -> %s: %v", id, oldPageFile, newPageFile, err)
			return fmt.Errorf("文档已移动但正文文件迁移失败，请联系管理员处理存储不一致: %w", err)
		}
	}

	// 移动文档后触发重新索引（Bleve 索引中的 space_id 和 path 已过期）
	safego.Go("triggerIndex", func() { s.triggerIndex(doc) })
	if doc.Type == workspacemodel.DocumentTypeDir {
		// 目录移动后，所有子文档的 space_id/path 也变了，需要重建索引
		descendants, _ := s.docRepo.FindDescendants(id)
		for _, desc := range descendants {
			safego.Go("triggerIndex", func() { s.triggerIndex(desc) })
		}
	}

	return nil
}

func replacePathPrefix(path, oldPrefix, newPrefix string) string {
	if !strings.HasPrefix(path, oldPrefix) {
		return path
	}
	rest := path[len(oldPrefix):]
	if len(rest) == 0 || rest[0] == ',' {
		return newPrefix + rest
	}
	return path
}

func (s *DocumentService) SortDocuments(userID int, req *workspacemodel.SortDocumentsRequest) error {
	if len(req.Orders) == 0 {
		return nil
	}

	// 批量查询所有待排序文档，校验权限与父节点一致性
	docIDs := make([]int, len(req.Orders))
	for i, order := range req.Orders {
		docIDs[i] = order.ID
	}
	docs, err := s.docRepo.FindByIDs(docIDs)
	if err != nil {
		return err
	}
	if len(docs) != len(req.Orders) {
		return errors.New("some documents not found")
	}

	docMap := make(map[int]*workspacemodel.Document, len(docs))
	var refParentID int
	var refSpaceID int
	for i, doc := range docs {
		docMap[doc.ID] = docs[i]
		if i == 0 {
			refParentID = doc.ParentID
			refSpaceID = doc.SpaceID
		}
	}
	for _, doc := range docs {
		if doc.ParentID != refParentID || doc.SpaceID != refSpaceID {
			return errors.New("all documents must share the same parent and space")
		}
	}

	privilege, err := s.spaceService.GetUserSpacePrivilege(refSpaceID, userID)
	if err != nil {
		return err
	}
	if privilege < 1 {
		return ErrNoDocumentPermission
	}

	return s.docRepo.Transaction(func(tx *gorm.DB) error {
		for _, order := range req.Orders {
			if _, ok := docMap[order.ID]; !ok {
				continue
			}
			if err := tx.Model(&workspacemodel.Document{}).Where("document_id = ?", order.ID).Update("sequence", order.Sequence).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *DocumentService) CopyDocument(id int, userID int, req *workspacemodel.CopyDocumentRequest) error {
	// 入口先校验源文档所在空间可见：只校验目标空间等于任何人都能把别人的文档复制进自己空间。
	srcDoc, err := s.docRepo.FindByID(id)
	if err != nil {
		return err
	}
	sourcePrivilege, err := s.spaceService.GetUserSpacePrivilege(srcDoc.SpaceID, userID)
	if err != nil {
		return err
	}
	if sourcePrivilege < 1 {
		return ErrNoDocumentPermission
	}
	return s.copyDocumentRecursive(id, userID, req, true)
}

func (s *DocumentService) copyDocumentRecursive(id int, userID int, req *workspacemodel.CopyDocumentRequest, addSuffix bool) error {
	srcDoc, err := s.docRepo.FindByID(id)
	if err != nil {
		return err
	}

	privilege, err := s.spaceService.GetUserSpacePrivilege(req.SpaceID, userID)
	if err != nil {
		return err
	}
	if privilege < 1 {
		return ErrNoDocumentPermission
	}

	// 源空间可见性由 CopyDocument 入口统一校验，递归子节点不重复查库。

	if req.ParentID > 0 {
		parentDoc, err := s.docRepo.FindByID(req.ParentID)
		if err != nil {
			return err
		}
		if parentDoc.SpaceID != req.SpaceID {
			return ErrInvalidParent
		}
		if parentDoc.Type != workspacemodel.DocumentTypeDir {
			return errors.New("parent must be a directory")
		}
	}

	// 顶层副本加 " (复制)" 后缀；子文档保持原名。冲突时自动递增。
	newName := srcDoc.Name
	if addSuffix {
		newName = srcDoc.Name + " (复制)"
	}
	existingDoc, _ := s.docRepo.FindByNameAndParentID(newName, req.ParentID, req.SpaceID)
	if existingDoc != nil {
		counter := 2
		if !addSuffix {
			newName = srcDoc.Name + " (复制)"
			existingDoc, _ = s.docRepo.FindByNameAndParentID(newName, req.ParentID, req.SpaceID)
		}
		for existingDoc != nil {
			newName = fmt.Sprintf("%s (复制 %d)", srcDoc.Name, counter)
			existingDoc, _ = s.docRepo.FindByNameAndParentID(newName, req.ParentID, req.SpaceID)
			counter++
			if counter > 100 {
				return ErrDocumentNameExists
			}
		}
	}

	path := "0"
	if req.ParentID > 0 {
		parentDoc, _ := s.docRepo.FindByID(req.ParentID)
		if parentDoc != nil {
			path = parentDoc.Path + fmt.Sprintf(",%d", req.ParentID)
		}
	}

	// 获取目标父节点下的最大 sequence
	maxSeq, _ := s.docRepo.MaxSequenceByParent(req.ParentID, req.SpaceID)

	// 对于页面类型，先读取源文档内容（在创建 DB 记录前失败快速返回）
	var content string
	if srcDoc.Type == workspacemodel.DocumentTypePage {
		content, err = s.GetDocumentContent(id)
		if err != nil {
			return fmt.Errorf("failed to read source document content: %w", err)
		}
	}

	newDoc := &workspacemodel.Document{
		ParentID:     req.ParentID,
		SpaceID:      req.SpaceID,
		Name:         newName,
		Type:         srcDoc.Type,
		Path:         path,
		Sequence:     maxSeq + 1,
		CreateUserID: userID,
		EditUserID:   userID,
	}

	err = s.docRepo.Transaction(func(tx *gorm.DB) error {
		return workspacerepo.NewDocumentRepositoryWithDB(tx).Create(newDoc)
	})
	if err != nil {
		return err
	}

	// 写入内容文件（DB 记录已创建，文件写入失败仅记录警告）
	if srcDoc.Type == workspacemodel.DocumentTypePage && content != "" {
		pageFile := s.getPageFile(newDoc)
		if err := doccontent.Set(newDoc.ID, s.docStorage, pageFile, content); err != nil {
			logx.Warnf("failed to set content for copied document %d: %v", newDoc.ID, err)
		}
	}

	if srcDoc.Type == workspacemodel.DocumentTypeDir {
		children, err := s.docRepo.FindByParentID(id)
		if err == nil {
			for _, child := range children {
				childReq := &workspacemodel.CopyDocumentRequest{
					ParentID: newDoc.ID,
					SpaceID:  req.SpaceID,
				}
				if err := s.copyDocumentRecursive(child.ID, userID, childReq, false); err != nil {
					logx.Warnf("failed to copy child document %d: %v", child.ID, err)
				}
			}
		}
	}

	return nil
}

func (s *DocumentService) GetDocumentVersions(documentID int) ([]*workspacemodel.DocumentVersionResponse, error) {
	versions, err := s.docRepo.GetVersions(documentID)
	if err != nil {
		return nil, err
	}

	userIDs := make([]int, 0)
	seen := make(map[int]bool)
	for _, v := range versions {
		if v.CreatedBy > 0 && !seen[v.CreatedBy] {
			seen[v.CreatedBy] = true
			userIDs = append(userIDs, v.CreatedBy)
		}
	}

	userMap := make(map[int]string, len(userIDs))
	if len(userIDs) > 0 {
		users, _ := s.userRepo.FindByIDs(userIDs)
		for _, u := range users {
			userMap[u.ID] = u.GivenName
		}
	}

	responses := make([]*workspacemodel.DocumentVersionResponse, len(versions))
	for i, v := range versions {
		resp := v.ToVersionResponse()
		if name, ok := userMap[v.CreatedBy]; ok {
			resp.CreatorName = name
		}
		responses[i] = &resp
	}

	return responses, nil
}

func (s *DocumentService) GetDocumentVersionsPaginated(documentID int, page int, pageSize int) ([]*workspacemodel.DocumentVersionResponse, int64, error) {
	versions, total, err := s.docRepo.GetVersionsPaginated(documentID, page, pageSize)
	if err != nil {
		return nil, 0, err
	}

	userIDs := make([]int, 0)
	seen := make(map[int]bool)
	for _, v := range versions {
		if v.CreatedBy > 0 && !seen[v.CreatedBy] {
			seen[v.CreatedBy] = true
			userIDs = append(userIDs, v.CreatedBy)
		}
	}

	userMap := make(map[int]string, len(userIDs))
	if len(userIDs) > 0 {
		users, _ := s.userRepo.FindByIDs(userIDs)
		for _, u := range users {
			userMap[u.ID] = u.GivenName
		}
	}

	responses := make([]*workspacemodel.DocumentVersionResponse, len(versions))
	for i, v := range versions {
		resp := v.ToVersionResponse()
		if name, ok := userMap[v.CreatedBy]; ok {
			resp.CreatorName = name
		}
		responses[i] = &resp
	}

	return responses, total, nil
}

// RecordView 异步记录文档访问
func (s *DocumentService) RecordView(documentID int, spaceID int, userID int, ip string, userAgent string) {
	if s.viewLogRepo == nil {
		return
	}
	logEntry := &workspacemodel.DocumentViewLog{
		DocumentID: documentID,
		SpaceID:    spaceID,
		UserID:     userID,
		IP:         ip,
		UserAgent:  userAgent,
		ViewedAt:   int(time.Now().Unix()),
	}
	go func() {
		_ = s.viewLogRepo.Create(logEntry)
	}()
}

// GetViewStats 获取文档访问统计
func (s *DocumentService) GetViewStats(documentID int) (*workspacemodel.DocumentViewStats, error) {
	return s.viewLogRepo.GetViewStats(documentID)
}

// ============================================================
// 多语言翻译管理 (P2-2)
// ============================================================

// GetTranslationGroup 获取文档的翻译组
func (s *DocumentService) GetTranslationGroup(documentID int) ([]*workspacemodel.DocumentTranslation, error) {
	// 使用内联查询避免引入新 repo 字段
	repo := workspacerepo.NewDocumentTranslationRepository()
	return repo.GetTranslationGroup(documentID)
}

// CreateTranslation 创建翻译关联
func (s *DocumentService) CreateTranslation(documentID int, lang string, translatedDocID int, status string) (*workspacemodel.DocumentTranslation, error) {
	// 校验文档存在
	_, err := s.docRepo.FindByID(documentID)
	if err != nil {
		return nil, err
	}
	_, err = s.docRepo.FindByID(translatedDocID)
	if err != nil {
		return nil, errors.New("translated document not found")
	}

	translation := &workspacemodel.DocumentTranslation{
		DocumentID:        documentID,
		Lang:              lang,
		TranslatedDocID:   translatedDocID,
		TranslationStatus: status,
	}
	repo := workspacerepo.NewDocumentTranslationRepository()
	if err := repo.Create(translation); err != nil {
		return nil, err
	}
	return translation, nil
}

// DeleteTranslation 删除翻译关联
func (s *DocumentService) DeleteTranslation(id int) error {
	repo := workspacerepo.NewDocumentTranslationRepository()
	return repo.Delete(id)
}

// EnsureDocReadable 校验用户是否有权读取该文档（含其历史版本与导出内容）。
//
// 版本列表 / 版本内容 / diff / 导出这几条读路径此前只校验「已登录」，不校验文档所属空间，
// 任何登录用户都可以用别人的 document_id 读出任意私有文档正文。内部索引与检索链路
// 不走这个方法（它们按空间批量取内容，由各自的可见性过滤负责）。
func (s *DocumentService) EnsureDocReadable(documentID, userID int) error {
	if userID <= 0 {
		return ErrNoDocumentPermission
	}
	doc, err := s.docRepo.FindByID(documentID)
	if err != nil {
		return err
	}
	privilege, err := s.spaceService.GetUserSpacePrivilege(doc.SpaceID, userID)
	if err != nil {
		return err
	}
	if privilege < 1 {
		return ErrNoDocumentPermission
	}
	return nil
}

// GetVersionContentForUser 读取指定版本内容，带空间可见性校验。
func (s *DocumentService) GetVersionContentForUser(documentID, version, userID int) (string, error) {
	if err := s.EnsureDocReadable(documentID, userID); err != nil {
		return "", err
	}
	return s.GetVersionContent(documentID, version)
}

// DiffVersionsForUser 对比两个版本，带空间可见性校验。
func (s *DocumentService) DiffVersionsForUser(documentID, fromVersion, toVersion, userID int) (*workspacemodel.DiffResponse, error) {
	if err := s.EnsureDocReadable(documentID, userID); err != nil {
		return nil, err
	}
	return s.DiffVersions(documentID, fromVersion, toVersion)
}

// GetDocumentVersionsPaginatedForUser 分页读取版本列表，带空间可见性校验。
func (s *DocumentService) GetDocumentVersionsPaginatedForUser(documentID, page, pageSize, userID int) ([]*workspacemodel.DocumentVersionResponse, int64, error) {
	if err := s.EnsureDocReadable(documentID, userID); err != nil {
		return nil, 0, err
	}
	return s.GetDocumentVersionsPaginated(documentID, page, pageSize)
}

func (s *DocumentService) GetVersionContent(documentID int, version int) (string, error) {
	if version == 0 {
		return s.GetDocumentContent(documentID)
	}
	return s.docRepo.GetVersionContent(documentID, version)
}

// DiffVersions 对比两个版本的内容差异
// fromVersion=0 表示当前内容，toVersion=0 表示当前内容
func (s *DocumentService) DiffVersions(documentID int, fromVersion int, toVersion int) (*workspacemodel.DiffResponse, error) {
	fromContent, err := s.GetVersionContent(documentID, fromVersion)
	if err != nil {
		return nil, err
	}
	toContent, err := s.GetVersionContent(documentID, toVersion)
	if err != nil {
		return nil, err
	}

	blocks := computeLineDiff(fromContent, toContent)

	added := 0
	removed := 0
	hasChanges := false
	for _, b := range blocks {
		switch b.Type {
		case workspacemodel.DiffBlockAdded:
			added += len(b.Lines)
			hasChanges = true
		case workspacemodel.DiffBlockRemoved:
			removed += len(b.Lines)
			hasChanges = true
		}
	}

	return &workspacemodel.DiffResponse{
		DocumentID:   documentID,
		FromVersion:  fromVersion,
		ToVersion:    toVersion,
		HasChanges:   hasChanges,
		DiffBlocks:   blocks,
		AddedCount:   added,
		RemovedCount: removed,
	}, nil
}

// computeLineDiff 基于 LCS 的行级 diff，无需外部依赖
func computeLineDiff(oldText string, newText string) []workspacemodel.DiffBlock {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)

	// LCS 动态规划表
	m, n := len(oldLines), len(newLines)
	// 对于超大文档，降级为简单对比避免 OOM
	if m > 5000 || n > 5000 {
		return simpleDiff(oldLines, newLines)
	}

	lcs := make([][]int, m+1)
	for i := range lcs {
		lcs[i] = make([]int, n+1)
	}
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if oldLines[i-1] == newLines[j-1] {
				lcs[i][j] = lcs[i-1][j-1] + 1
			} else if lcs[i-1][j] >= lcs[i][j-1] {
				lcs[i][j] = lcs[i-1][j]
			} else {
				lcs[i][j] = lcs[i][j-1]
			}
		}
	}

	// 回溯生成 diff 块
	type op struct {
		typ   string
		lines []string
	}
	var ops []op
	i, j := m, n
	for i > 0 || j > 0 {
		if i > 0 && j > 0 && oldLines[i-1] == newLines[j-1] {
			ops = append(ops, op{typ: workspacemodel.DiffBlockContext, lines: []string{oldLines[i-1]}})
			i--
			j--
		} else if j > 0 && (i == 0 || lcs[i][j-1] >= lcs[i-1][j]) {
			ops = append(ops, op{typ: workspacemodel.DiffBlockAdded, lines: []string{newLines[j-1]}})
			j--
		} else if i > 0 {
			ops = append(ops, op{typ: workspacemodel.DiffBlockRemoved, lines: []string{oldLines[i-1]}})
			i--
		}
	}

	// 反转并合并连续同类型块
	var blocks []workspacemodel.DiffBlock
	for k := len(ops) - 1; k >= 0; k-- {
		if len(blocks) > 0 && blocks[len(blocks)-1].Type == ops[k].typ {
			blocks[len(blocks)-1].Lines = append(blocks[len(blocks)-1].Lines, ops[k].lines...)
		} else {
			blocks = append(blocks, workspacemodel.DiffBlock{
				Type:  ops[k].typ,
				Lines: ops[k].lines,
			})
		}
	}
	return blocks
}

// simpleDiff 超大文档降级方案：整体标记为删除+新增
func simpleDiff(oldLines []string, newLines []string) []workspacemodel.DiffBlock {
	var blocks []workspacemodel.DiffBlock
	if len(oldLines) > 0 {
		blocks = append(blocks, workspacemodel.DiffBlock{Type: workspacemodel.DiffBlockRemoved, Lines: oldLines})
	}
	if len(newLines) > 0 {
		blocks = append(blocks, workspacemodel.DiffBlock{Type: workspacemodel.DiffBlockAdded, Lines: newLines})
	}
	return blocks
}

// splitLines 按行分割，保留空行，不包含换行符
func splitLines(text string) []string {
	if text == "" {
		return []string{}
	}
	lines := strings.Split(text, "\n")
	// 处理 \r\n
	for i, l := range lines {
		lines[i] = strings.TrimSuffix(l, "\r")
	}
	// 移除末尾空行（由末尾换行符产生）
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func (s *DocumentService) RollbackToVersion(documentID int, targetVersion int, userID int) error {
	content, err := s.docRepo.GetVersionContent(documentID, targetVersion)
	if err != nil {
		return err
	}

	doc, err := s.docRepo.FindByID(documentID)
	if err != nil {
		return err
	}

	privilege, err := s.spaceService.GetUserSpacePrivilege(doc.SpaceID, userID)
	if err != nil {
		return err
	}
	if privilege < 1 {
		return ErrNoDocumentPermission
	}

	pageFile := s.getPageFile(doc)
	if err := doccontent.Set(documentID, s.docStorage, pageFile, content); err != nil {
		return err
	}

	doc.EditUserID = userID
	if err := s.docRepo.Update(doc); err != nil {
		return err
	}

	latestVersion, _ := s.docRepo.GetLatestVersion(documentID)
	newVersion := &workspacemodel.DocumentVersion{
		DocumentID: documentID,
		Version:    latestVersion + 1,
		Content:    content,
		ChangeLog:  fmt.Sprintf("回滚到版本 %d", targetVersion),
		CreatedBy:  userID,
	}
	if err := s.docRepo.CreateVersion(newVersion); err != nil {
		return err
	}

	safego.Go("triggerIndex", func() { s.triggerIndex(doc) })

	return nil
}

// SearchDocuments 已于 P2-1 移除（与 /search/fulltext 重复）

func (s *DocumentService) GetDocumentContent(id int) (string, error) {
	doc, err := s.docRepo.FindByID(id)
	if err != nil {
		return "", err
	}
	return s.GetDocumentContentByDoc(doc)
}

// GetDocumentContentByDoc 根据已查询的 doc 对象读取内容，避免重复 DB 查询
func (s *DocumentService) GetDocumentContentByDoc(doc *workspacemodel.Document) (string, error) {
	if doc == nil {
		return "", workspacerepo.ErrDocumentNotFound
	}
	pageFile := s.getPageFile(doc)
	return doccontent.Get(doc.ID, s.docStorage, pageFile)
}

func (s *DocumentService) UpdateDocumentContent(doc *workspacemodel.Document, content string) error {
	pageFile := s.getPageFile(doc)
	return doccontent.Set(doc.ID, s.docStorage, pageFile, content)
}

// MarkWikiSource 标记文档为 wiki 源（raw 层不可变），后续内容变更将触发 wiki 重编译。
func (s *DocumentService) MarkWikiSource(docID int, immutable bool) error {
	return s.docRepo.MarkImmutable(docID, immutable)
}

func (s *DocumentService) GetSpaceDocumentCount(spaceID int) (int64, error) {
	return s.docRepo.CountBySpaceID(spaceID)
}

func (s *DocumentService) GetDocumentPathNames(id int) ([]map[string]interface{}, error) {
	doc, err := s.docRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	result := make([]map[string]interface{}, 0)

	// 收集所有路径节点 ID，一次批量查询（替代循环 FindByID 的 N+1）
	pathParts := strings.Split(doc.Path, ",")
	nodeIDs := make([]int, 0, len(pathParts))
	for _, part := range pathParts {
		part = strings.TrimSpace(part)
		if part == "" || part == "0" {
			continue
		}
		nodeID := 0
		fmt.Sscanf(part, "%d", &nodeID)
		if nodeID > 0 {
			nodeIDs = append(nodeIDs, nodeID)
		}
	}

	nodeMap := make(map[int]*workspacemodel.Document, len(nodeIDs))
	if len(nodeIDs) > 0 {
		nodes, _ := s.docRepo.FindByIDs(nodeIDs)
		for _, n := range nodes {
			nodeMap[n.ID] = n
		}
	}

	for _, nodeID := range nodeIDs {
		if node, ok := nodeMap[nodeID]; ok {
			result = append(result, map[string]interface{}{
				"id":   node.ID,
				"name": node.Name,
			})
		}
	}

	result = append(result, map[string]interface{}{
		"id":   doc.ID,
		"name": doc.Name,
	})

	return result, nil
}

func (s *DocumentService) ImportDocuments(spaceID, userID int, items []workspacemodel.ImportDocumentItem) (*workspacemodel.ImportDocumentsResult, error) {
	privilege, err := s.spaceService.GetUserSpacePrivilege(spaceID, userID)
	if err != nil {
		return nil, err
	}
	if privilege < 1 {
		return nil, ErrNoDocumentPermission
	}

	result := &workspacemodel.ImportDocumentsResult{}
	nameMap := make(map[string]int)

	for _, item := range items {
		parts := strings.Split(strings.Trim(item.Path, "/"), "/")
		if len(parts) == 0 || parts[0] == "" {
			result.Skipped++
			continue
		}

		parentID := 0
		ok := true
		// 记录本轮新创建的目录 ID，失败时回滚，避免"提示跳过但实际导入"
		createdDirIDs := make([]int, 0, len(parts))

		for i := 0; i < len(parts)-1; i++ {
			dirName := parts[i]
			existing, _ := s.docRepo.FindByNameAndParentID(dirName, parentID, spaceID)
			if existing != nil {
				// 路径冲突：父级已存在但不是目录，整个文件无法放置在合法目录中
				if existing.Type != workspacemodel.DocumentTypeDir {
					result.Errors = append(result.Errors, fmt.Sprintf("跳过 %s: 路径 %s 已存在但不是目录", item.Path, dirName))
					ok = false
					break
				}
				parentID = existing.ID
				continue
			}
			dir, err := s.CreateDocument(userID, &workspacemodel.CreateDocumentRequest{
				SpaceID:  spaceID,
				ParentID: parentID,
				Name:     dirName,
				Type:     workspacemodel.DocumentTypeDir,
			})
			if err != nil {
				// 回滚本次已成功创建的目录，避免残留
				for _, dirID := range createdDirIDs {
					if delErr := s.docRepo.Delete(dirID); delErr != nil {
						logx.Errorf("rollback directory %d failed: %v", dirID, delErr)
					}
				}
				result.Errors = append(result.Errors, fmt.Sprintf("创建目录 %s 失败: %v", dirName, err))
				ok = false
				break
			}
			createdDirIDs = append(createdDirIDs, dir.ID)
			parentID = dir.ID
		}
		if !ok {
			result.Skipped++
			continue
		}

		fileName := parts[len(parts)-1]
		baseName := strings.TrimSuffix(fileName, filepath.Ext(fileName))
		uniqueName := s.uniqueName(parentID, baseName, nameMap)

		_, err := s.CreateDocument(userID, &workspacemodel.CreateDocumentRequest{
			SpaceID:  spaceID,
			ParentID: parentID,
			Name:     uniqueName,
			Type:     workspacemodel.DocumentTypePage,
			Content:  item.Content,
		})
		if err != nil {
			// 文档创建失败时回滚本轮新创建的目录，避免"提示跳过但实际导入"
			for _, dirID := range createdDirIDs {
				if delErr := s.docRepo.Delete(dirID); delErr != nil {
					logx.Errorf("rollback directory %d failed: %v", dirID, delErr)
				}
			}
			result.Errors = append(result.Errors, fmt.Sprintf("导入 %s 失败: %v", item.Path, err))
			result.Skipped++
			continue
		}
		result.Created++
	}

	return result, nil
}

func (s *DocumentService) uniqueName(parentID int, base string, nameMap map[string]int) string {
	candidate := base
	for {
		key := fmt.Sprintf("%d/%s", parentID, candidate)
		if _, exists := nameMap[key]; !exists {
			nameMap[key] = 1
			return candidate
		}
		nameMap[key]++
		candidate = fmt.Sprintf("%s (%d)", base, nameMap[key]-1)
	}
}

func (s *DocumentService) getPageFile(doc *workspacemodel.Document) string {
	return getPageFilePath(doc, s.docRepo, s.spaceRepo, s.docStorage)
}

// ComputePageFilePath 供迁移/诊断工具复用同一套正文路径算法（file 模式）。
func ComputePageFilePath(doc *workspacemodel.Document) string {
	return getPageFilePath(doc, workspacerepo.NewDocumentRepository(), workspacerepo.NewSpaceRepository(),
		utils.NewDocumentStorageWithDir(config.AppConfig.Storage.DocumentDir))
}

func getPageFilePath(doc *workspacemodel.Document, docRepo *workspacerepo.DocumentRepository, spaceRepo *workspacerepo.SpaceRepository, docStorage *utils.DocumentStorage) string {
	space, _ := spaceRepo.FindByIDCached(doc.SpaceID)
	spaceName := fmt.Sprintf("space_%d", doc.SpaceID)
	if space != nil {
		spaceName = space.Name
	}
	// GetPageFile 只净化自己拼接的路径；本函数 else 分支用原始名拼接，
	// 空间名/父目录名/文档名可能来自用户输入或 AI 生成内容，这里统一净化每一段，
	// 杜绝 ".."、分隔符（Windows 下反斜杠同样可逃逸）拼出存储根之外的路径。
	safeSpaceName := utils.SafePathSegment(spaceName)

	if doc.Path == "0" || doc.Path == "" {
		if doc.Name == spaceName && doc.Type == workspacemodel.DocumentTypeDir {
			return filepath.Join(safeSpaceName, utils.DocumentDefaultName+utils.DocumentPageSuffix)
		}
		result := docStorage.GetPageFile(spaceName, doc.Name, doc.Type)
		return result
	}

	pathParts := strings.Split(doc.Path, ",")

	// 收集所有父节点 ID，一次批量查询（替代循环 FindByID 的 N+1）
	parentIDs := make([]int, 0, len(pathParts))
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
			parentIDs = append(parentIDs, parentID)
		}
	}

	parentMap := make(map[int]*workspacemodel.Document, len(parentIDs))
	if len(parentIDs) > 0 {
		parents, _ := docRepo.FindByIDs(parentIDs)
		for _, p := range parents {
			parentMap[p.ID] = p
		}
	}

	var parentPath string
	for _, pid := range parentIDs {
		if p, ok := parentMap[pid]; ok {
			parentPath = parentPath + "/" + utils.SafePathSegment(p.Name)
		}
	}

	var pageFile string
	if parentPath == "" {
		pageFile = docStorage.GetPageFile(spaceName, doc.Name, doc.Type)
	} else {
		fullPath := safeSpaceName + parentPath
		safeDocName := utils.SafePathSegment(doc.Name)

		if doc.Type == workspacemodel.DocumentTypePage {
			pageFile = fullPath + "/" + safeDocName + utils.DocumentPageSuffix
		} else {
			pageFile = fullPath + "/" + safeDocName + "/" + utils.DocumentDefaultName + utils.DocumentPageSuffix
		}
	}

	return pageFile
}

func (s *DocumentService) triggerIndex(doc *workspacemodel.Document) {
	if err := s.searchService.IndexDocument(doc); err != nil {
		logx.Warnf("failed to index document %d: %v", doc.ID, err)
	}
}

func (s *DocumentService) triggerVectorSync(doc *workspacemodel.Document, opType string) {
	if VectorSyncFunc == nil {
		return
	}

	content := ""
	if opType != "delete" {
		pageFile := s.getPageFile(doc)
		content, _ = doccontent.Get(doc.ID, s.docStorage, pageFile)
	}

	go VectorSyncFunc(doc.ID, doc.SpaceID, doc.Name, content, opType)
}

func (s *DocumentService) notifyDocumentFollowers(doc *workspacemodel.Document, editorUserID int, changeLog string) {
	// 系统设置「发送邮件通知」总开关（basic 组）：显式配为 "1" 才发通知邮件，缺键/其他值都视为关闭。
	// 只拦通知这一档；AI 工具的 send_email 是用户显式请求，不受此开关约束。
	if v, err := s.configService.GetConfigValue("send_email_open"); err != nil || v != "1" {
		return
	}

	followers, err := s.followRepo.FindFollowers(doc.ID, socialmodel.FollowTypeDocument)
	if err != nil || len(followers) == 0 {
		return
	}

	userIDs := make([]int, 0)
	for _, f := range followers {
		if f.UserID != editorUserID {
			userIDs = append(userIDs, f.UserID)
		}
	}
	if len(userIDs) == 0 {
		return
	}

	users, err := s.userRepo.FindByIDs(userIDs)
	if err != nil || len(users) == 0 {
		return
	}

	emails := make([]string, 0)
	for _, u := range users {
		if u.Email != "" {
			emails = append(emails, u.Email)
		}
	}
	if len(emails) == 0 {
		return
	}

	editorName := ""
	if editor, err := s.userRepo.FindByID(editorUserID); err == nil {
		editorName = editor.GivenName
		if editorName == "" {
			editorName = editor.Username
		}
	}

	docURL := "/"
	if baseURL, err := s.configService.GetConfigValue("system.base_url"); err == nil && baseURL != "" {
		docURL = fmt.Sprintf("%s/document/%d", baseURL, doc.ID)
	}

	changeLogText := changeLog
	if changeLogText == "" {
		changeLogText = "无"
	}
	if err := s.emailService.SendDocumentNotification(doc.Name, editorName, changeLogText, docURL, emails); err != nil {
		logx.Warnf("failed to send document notification: %v", err)
	}
}

func ReindexAllVectors() (int, error) {
	if VectorSyncFunc == nil {
		return 0, fmt.Errorf("vector sync not configured")
	}
	// P0：重建走强制通道，不做指纹判重
	syncFn := VectorSyncReindexFunc
	if syncFn == nil {
		syncFn = VectorSyncFunc
	}

	docRepo := workspacerepo.NewDocumentRepository()
	docs, err := docRepo.FindAllDocuments()
	if err != nil {
		return 0, err
	}

	docStorage := utils.NewDocumentStorageWithDir(config.AppConfig.Storage.DocumentDir)
	spaceRepo := workspacerepo.NewSpaceRepository()
	var count int32
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup

	for _, doc := range docs {
		if doc.Type != workspacemodel.DocumentTypePage {
			continue
		}

		pageFile := getPageFilePath(doc, docRepo, spaceRepo, docStorage)
		content, err := doccontent.Get(doc.ID, docStorage, pageFile)
		if err != nil || content == "" {
			continue
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(docID, spaceID int, docName, content string) {
			defer wg.Done()
			defer func() { <-sem }()
			syncFn(docID, spaceID, docName, content, "store")
			atomic.AddInt32(&count, 1)
		}(doc.ID, doc.SpaceID, doc.Name, content)
	}

	wg.Wait()
	return int(count), nil
}

func ReindexSpaceVectors(spaceID int) (int, error) {
	if VectorSyncFunc == nil {
		return 0, fmt.Errorf("vector sync not configured")
	}
	// P0：重建走强制通道，不做指纹判重
	syncFn := VectorSyncReindexFunc
	if syncFn == nil {
		syncFn = VectorSyncFunc
	}

	docRepo := workspacerepo.NewDocumentRepository()
	docs, err := docRepo.FindBySpaceID(spaceID)
	if err != nil {
		return 0, err
	}

	docStorage := utils.NewDocumentStorageWithDir(config.AppConfig.Storage.DocumentDir)
	spaceRepo := workspacerepo.NewSpaceRepository()
	var count int32
	sem := make(chan struct{}, 5)
	var wg sync.WaitGroup

	for _, doc := range docs {
		if doc.Type != workspacemodel.DocumentTypePage {
			continue
		}

		d := doc
		pageFile := getPageFilePath(d, docRepo, spaceRepo, docStorage)
		content, err := doccontent.Get(d.ID, docStorage, pageFile)
		if err != nil || content == "" {
			continue
		}

		sem <- struct{}{}
		wg.Add(1)
		go func(docID, spaceID int, docName, content string) {
			defer wg.Done()
			defer func() { <-sem }()
			syncFn(docID, spaceID, docName, content, "store")
			atomic.AddInt32(&count, 1)
		}(d.ID, d.SpaceID, d.Name, content)
	}

	wg.Wait()
	return int(count), nil
}

func ReindexDocVector(docID int) error {
	if VectorSyncFunc == nil {
		return fmt.Errorf("vector sync not configured")
	}
	// P0：重建走强制通道，不做指纹判重
	syncFn := VectorSyncReindexFunc
	if syncFn == nil {
		syncFn = VectorSyncFunc
	}

	docRepo := workspacerepo.NewDocumentRepository()
	doc, err := docRepo.FindByID(docID)
	if err != nil {
		return err
	}
	if doc.Type != workspacemodel.DocumentTypePage {
		return fmt.Errorf("document is not a page")
	}

	docStorage := utils.NewDocumentStorageWithDir(config.AppConfig.Storage.DocumentDir)
	spaceRepo := workspacerepo.NewSpaceRepository()
	pageFile := getPageFilePath(doc, docRepo, spaceRepo, docStorage)
	content, err := doccontent.Get(doc.ID, docStorage, pageFile)
	if err != nil || content == "" {
		return fmt.Errorf("document content is empty")
	}

	syncFn(doc.ID, doc.SpaceID, doc.Name, content, "store")
	return nil
}

var linkPattern = regexp.MustCompile(`\[([^\]]*)\]\(/space/(\d+)/doc/(\d+)\)`)

func (s *DocumentService) parseDocumentLinks(docID int, spaceID int, content string) {
	if content == "" {
		return
	}

	matches := linkPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return
	}

	seen := make(map[int]bool)
	var links []*workspacemodel.DocumentLink
	for _, m := range matches {
		targetDocID, err := strconv.Atoi(m[2])
		if err != nil || targetDocID == 0 {
			continue
		}
		if targetDocID == docID {
			continue
		}
		if seen[targetDocID] {
			continue
		}
		seen[targetDocID] = true

		links = append(links, &workspacemodel.DocumentLink{
			TargetDocID: targetDocID,
			LinkText:    m[1],
		})
	}

	if err := s.docLinkRepo.ReplaceLinks(docID, spaceID, links); err != nil {
		logx.Warnf("failed to save document links for doc %d: %v", docID, err)
	}
}

// GetBacklinks 获取文档的反向链接（哪些文档引用了该文档）
func (s *DocumentService) GetBacklinks(docID int) ([]*workspacerepo.BacklinkResult, error) {
	return s.docLinkRepo.GetBacklinks(docID)
}

// GetForwardLinks 获取文档的正向链接（该文档引用了哪些文档）
func (s *DocumentService) GetForwardLinks(docID int) ([]*workspacerepo.ForwardLinkResult, error) {
	return s.docLinkRepo.GetForwardLinks(docID)
}

func (s *DocumentService) parseDocumentLinksAsync(docID int, spaceID int, content string) {
	go func() {
		defer func() { _ = recover() }()
		s.parseDocumentLinks(docID, spaceID, content)
	}()
}

type EditConflictInfo struct {
	HasConflict   bool   `json:"has_conflict"`
	EditingUser   string `json:"editing_user"`
	EditingUserID int    `json:"editing_user_id"`
	LockedAt      string `json:"locked_at"`
}

func (s *DocumentService) LockForEdit(docID int, userID int, permanent bool) error {
	username := ""
	if u, err := s.userRepo.FindByID(userID); err == nil {
		username = u.Username
	}
	return s.editLockRepo.Lock(docID, userID, username, permanent)
}

func (s *DocumentService) UnlockEdit(docID int, userID int) error {
	return s.editLockRepo.Unlock(docID, userID)
}

// HeartbeatEdit 刷新编辑锁，防止长编辑会话被强制解锁
func (s *DocumentService) HeartbeatEdit(docID int, userID int) error {
	return s.editLockRepo.Heartbeat(docID, userID)
}

func (s *DocumentService) CheckEditConflict(docID int, userID int) EditConflictInfo {
	lock, err := s.editLockRepo.Get(docID)
	if err != nil {
		return EditConflictInfo{HasConflict: false}
	}
	if lock.UserID == userID {
		return EditConflictInfo{HasConflict: false}
	}
	return EditConflictInfo{
		HasConflict:   true,
		EditingUser:   lock.Username,
		EditingUserID: lock.UserID,
		LockedAt:      lock.LockedAt.Format("2006-01-02 15:04:05"),
	}
}
