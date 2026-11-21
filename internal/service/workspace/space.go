package workspace

import (
	"archive/zip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	logx "github.com/listen808/citebase/internal/log"
	usermodel "github.com/listen808/citebase/internal/model/user"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/service/doccontent"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/config"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrSpaceAlreadyExists  = errors.New("space name already exists")
	ErrSpaceNameInvalid    = errors.New("space name contains invalid characters")
	ErrNoPermission        = errors.New("no permission to access this space")
	ErrNotSpaceMember      = errors.New("user is not a member of this space")
	ErrSpaceHasDocuments   = errors.New("space still has documents, please delete them first")
	ErrSpaceExportDisabled = errors.New("space export is disabled")
	ErrSpaceShareDisabled  = errors.New("space share is disabled")
)

var spaceNameInvalidChars = regexp.MustCompile(`[\\/:*?"<>、|]`)

type SpaceService struct {
	spaceRepo  *workspacerepo.SpaceRepository
	userRepo   *userrepo.UserRepository
	docRepo    *workspacerepo.DocumentRepository
	docStorage *utils.DocumentStorage
}

func NewSpaceService() *SpaceService {
	docDir := config.AppConfig.Storage.DocumentDir
	if docDir == "" {
		docDir = "./data/markdowns"
	}
	return &SpaceService{
		spaceRepo:  workspacerepo.NewSpaceRepository(),
		userRepo:   userrepo.NewUserRepository(),
		docRepo:    workspacerepo.NewDocumentRepository(),
		docStorage: utils.NewDocumentStorageWithDir(docDir),
	}
}

func validateSpaceName(name string) error {
	if name == "" {
		return errors.New("space name cannot be empty")
	}
	if spaceNameInvalidChars.MatchString(name) {
		return ErrSpaceNameInvalid
	}
	return nil
}

func (s *SpaceService) CreateSpace(userID int, req *workspacemodel.CreateSpaceRequest) (*workspacemodel.Space, error) {
	if err := validateSpaceName(req.Name); err != nil {
		return nil, err
	}

	exists, err := s.spaceRepo.HasSpaceName(req.Name)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrSpaceAlreadyExists
	}

	visitLevel := req.VisitLevel
	if visitLevel == "" {
		visitLevel = workspacemodel.VisitLevelPublic
	}

	// 默认允许分享和导出
	isShare := 1
	if req.IsShare != nil && !*req.IsShare {
		isShare = 0
	}
	isExport := 1
	if req.IsExport != nil && !*req.IsExport {
		isExport = 0
	}
	// 分享密码 hash
	sharePassword := ""
	if req.SharePassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.SharePassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash share password: %w", err)
		}
		sharePassword = string(hash)
	}

	var space *workspacemodel.Space
	var spaceDoc *workspacemodel.Document
	err = s.spaceRepo.Transaction(func(tx *gorm.DB) error {
		txSpaceRepo := workspacerepo.NewSpaceRepositoryWithDB(tx)
		txDocRepo := workspacerepo.NewDocumentRepositoryWithDB(tx)

		maxSeq, _ := s.spaceRepo.GetMaxSequence()
		space = &workspacemodel.Space{
			Name:          req.Name,
			Description:   req.Description,
			Tags:          req.Tags,
			Status:        1,
			VisitLevel:    visitLevel,
			IsShare:       isShare,
			IsExport:      isExport,
			SharePassword: sharePassword,
			CreatedBy:     userID,
			CreateUserID:  userID,
			EditUserID:    userID,
			IsDelete:      0,
			Sequence:      maxSeq + 1,
		}
		if err := txSpaceRepo.Create(space); err != nil {
			return err
		}

		spaceUser := &workspacemodel.SpaceUser{
			UserID:     userID,
			SpaceID:    space.ID,
			Privilege:  2,
			CreateTime: int(time.Now().Unix()),
		}
		if err := txSpaceRepo.AddSpaceUser(spaceUser); err != nil {
			return err
		}

		now := int(time.Now().Unix())
		spaceDoc = &workspacemodel.Document{
			ParentID:     0,
			SpaceID:      space.ID,
			Name:         space.Name,
			Type:         workspacemodel.DocumentTypeDir,
			Path:         "0",
			Sequence:     0,
			CreateUserID: userID,
			EditUserID:   userID,
			IsDelete:     0,
			CreateTime:   now,
			UpdateTime:   now,
		}
		if err := txDocRepo.Create(spaceDoc); err != nil {
			return fmt.Errorf("failed to create space default document: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	pageFile := s.docStorage.GetPageFile(space.Name, space.Name, workspacemodel.DocumentTypeDir)
	logx.Infof("Creating space index file: pageFile=%s, space.Name=%s", pageFile, space.Name)
	if err := doccontent.Set(spaceDoc.ID, s.docStorage, pageFile, fmt.Sprintf("# %s\n\n欢迎来到 %s 空间\n", space.Name, space.Name)); err != nil {
		logx.Warnf("failed to create space index content: pageFile=%s, error=%v", pageFile, err)
	} else {
		logx.Infof("Successfully created space index content: %s", pageFile)
	}

	return space, nil
}

func (s *SpaceService) GetSpaceByID(id int) (*workspacemodel.SpaceResponse, error) {
	// P3 修复：走缓存，避免每次请求都查 DB
	space, err := s.spaceRepo.FindByIDCached(id)
	if err != nil {
		return nil, err
	}

	if space.IsDelete == 1 {
		return nil, workspacerepo.ErrSpaceNotFound
	}

	memberCount, _ := s.spaceRepo.GetSpaceMemberCount(id)
	creatorID, _ := s.spaceRepo.GetSpaceCreator(id)

	docCount, _ := s.docRepo.CountBySpaceID(id)

	response := space.ToResponse()
	response.MemberCount = memberCount
	response.CreatorID = creatorID
	response.DocCount = docCount
	return &response, nil
}

func (s *SpaceService) GetSpacesByUserID(userID int, page, pageSize int, keyword, tags string) ([]*workspacemodel.SpaceResponse, int64, error) {
	offset := (page - 1) * pageSize

	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return nil, 0, err
	}
	isAdmin := user.RoleID == 1 || user.RoleID == 2

	var spaces []*workspacemodel.Space
	var total int64
	if isAdmin {
		spaces, total, err = s.spaceRepo.FindAll(offset, pageSize, keyword, tags, "")
	} else {
		spaces, total, err = s.spaceRepo.FindByUserID(userID, offset, pageSize, keyword, tags)
	}
	if err != nil {
		return nil, 0, err
	}

	// P1+P2 修复：批量聚合，消除 N+1 查询（62→4 次 DB 调用）
	spaceIDs := make([]int, len(spaces))
	for i, sp := range spaces {
		spaceIDs[i] = sp.ID
	}
	memberCounts, _ := s.spaceRepo.BatchSpaceMemberCount(spaceIDs)
	docCounts, _ := s.spaceRepo.BatchSpaceDocCount(spaceIDs)
	privileges, _ := s.spaceRepo.BatchUserSpacePrivileges(userID, spaceIDs)

	responses := make([]*workspacemodel.SpaceResponse, len(spaces))
	for i, space := range spaces {
		response := space.ToResponse()
		if p, ok := privileges[space.ID]; ok {
			response.Privilege = p
		}
		response.MemberCount = memberCounts[space.ID]
		response.DocCount = docCounts[space.ID]
		responses[i] = &response
	}

	return responses, total, nil
}

// GetAllSpaces 分页浏览空间。必须按调用者可见范围裁剪：
// 旧实现直接查全表且不问访问者是谁，任何（含未登录）访问者都能枚举私有空间的名称与描述。
func (s *SpaceService) GetAllSpaces(page, pageSize int, keyword, tags, visitLevel string, userID int, isSuperAdmin bool) ([]*workspacemodel.SpaceResponse, int64, error) {
	offset := (page - 1) * pageSize

	visibleIDs, err := s.spaceRepo.FindAccessibleSpaceIDs(userID, isSuperAdmin)
	if err != nil {
		return nil, 0, err
	}

	spaces, total, err := s.spaceRepo.FindAllVisible(offset, pageSize, keyword, tags, visitLevel, visibleIDs, isSuperAdmin)
	if err != nil {
		return nil, 0, err
	}

	// P1+P2 修复：批量聚合，消除 N+1 查询（42→3 次 DB 调用）
	spaceIDs := make([]int, len(spaces))
	for i, sp := range spaces {
		spaceIDs[i] = sp.ID
	}
	memberCounts, _ := s.spaceRepo.BatchSpaceMemberCount(spaceIDs)
	docCounts, _ := s.spaceRepo.BatchSpaceDocCount(spaceIDs)

	responses := make([]*workspacemodel.SpaceResponse, len(spaces))
	for i, space := range spaces {
		response := space.ToResponse()
		response.MemberCount = memberCounts[space.ID]
		response.DocCount = docCounts[space.ID]
		responses[i] = &response
	}

	return responses, total, nil
}

func (s *SpaceService) UpdateSpace(spaceID int, req *workspacemodel.UpdateSpaceRequest) (*workspacemodel.Space, error) {
	space, err := s.spaceRepo.FindByID(spaceID)
	if err != nil {
		return nil, err
	}

	if req.Name != "" && req.Name != space.Name {
		if err := validateSpaceName(req.Name); err != nil {
			return nil, err
		}
		sameName, err := s.spaceRepo.HasSameName(spaceID, req.Name)
		if err != nil {
			return nil, err
		}
		if sameName {
			return nil, ErrSpaceAlreadyExists
		}

		oldPageFile := s.docStorage.GetPageFile(space.Name, space.Name, workspacemodel.DocumentTypeDir)
		if !doccontent.UseDB() && s.docStorage.Exists(oldPageFile) {
			if err := s.docStorage.Rename(oldPageFile, req.Name, req.Name, workspacemodel.DocumentTypeDir); err != nil {
				logx.Warnf("failed to rename space directory: %v", err)
			} else {
				// Rename 只搬 {旧}/{旧} → {新}/{新}，旧一级目录会留成空壳；os.Remove 只删空目录，非空时安全失败
				oldRoot := filepath.Join(s.docStorage.DocDir, utils.SafePathSegment(space.Name))
				if rmErr := os.Remove(oldRoot); rmErr != nil && !os.IsNotExist(rmErr) {
					logx.Warnf("failed to remove empty old space dir %s: %v", oldRoot, rmErr)
				}
			}
		}

		space.Name = req.Name
	}

	if req.Description != "" {
		space.Description = req.Description
	}
	if req.Tags != "" {
		space.Tags = req.Tags
	}
	if req.VisitLevel != "" {
		space.VisitLevel = req.VisitLevel
	}
	if req.IsShare != nil {
		if *req.IsShare {
			space.IsShare = 1
		} else {
			space.IsShare = 0
		}
	}
	if req.IsExport != nil {
		if *req.IsExport {
			space.IsExport = 1
		} else {
			space.IsExport = 0
		}
	}
	if req.SharePassword != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.SharePassword), bcrypt.DefaultCost)
		if err != nil {
			return nil, fmt.Errorf("failed to hash share password: %w", err)
		}
		space.SharePassword = string(hash)
	}
	if req.Sequence != nil {
		space.Sequence = *req.Sequence
	}
	if err := s.spaceRepo.Update(space); err != nil {
		return nil, err
	}

	return space, nil
}

// SortSpaces 批量更新空间排序序号
func (s *SpaceService) SortSpaces(userID int, req *workspacemodel.SortSpacesRequest) error {
	if len(req.Orders) == 0 {
		return nil
	}
	return s.spaceRepo.BatchUpdateSequence(req.Orders)
}

func (s *SpaceService) DeleteSpace(spaceID int) error {
	space, err := s.spaceRepo.FindByID(spaceID)
	if err != nil {
		return err
	}

	// P3：用事务包裹删除操作，避免部分删除导致数据不一致
	if err := s.spaceRepo.Transaction(func(tx *gorm.DB) error {
		if err := s.spaceRepo.DeleteSpaceDocuments(spaceID); err != nil {
			return fmt.Errorf("failed to delete space documents: %w", err)
		}
		if err := s.spaceRepo.DeleteSpaceAttachments(spaceID); err != nil {
			return fmt.Errorf("failed to delete space attachments: %w", err)
		}
		if err := s.spaceRepo.DeleteSpaceUsersBySpaceID(spaceID); err != nil {
			return fmt.Errorf("failed to delete space members: %w", err)
		}
		if err := s.spaceRepo.Delete(spaceID); err != nil {
			return fmt.Errorf("failed to delete space: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	spaceDirPath := filepath.Join(s.docStorage.DocDir, space.Name)
	if _, err := os.Stat(spaceDirPath); err == nil {
		if err := os.RemoveAll(spaceDirPath); err != nil {
			logx.Warnf("failed to remove space directory: %v", err)
		}
	}

	return nil
}

// SpaceDeleteCheck 空间删除预检查结果
type SpaceDeleteCheck struct {
	DocumentCount int64 `json:"document_count"`
	MemberCount   int64 `json:"member_count"`
}

// PreCheckDeleteSpace 删除空间前预检查，返回文档和成员数量供前端确认
func (s *SpaceService) PreCheckDeleteSpace(spaceID int) (*SpaceDeleteCheck, error) {
	docCount, err := s.spaceRepo.CountDocumentsBySpaceID(spaceID)
	if err != nil {
		return nil, err
	}
	memberCount, err := s.spaceRepo.CountMembersBySpaceID(spaceID)
	if err != nil {
		return nil, err
	}
	return &SpaceDeleteCheck{
		DocumentCount: docCount,
		MemberCount:   memberCount,
	}, nil
}

func (s *SpaceService) AddSpaceMember(spaceID, operatorID, targetUserID int, privilege int) error {
	exists, operatorPrivilege, err := s.spaceRepo.CheckSpacePrivilege(spaceID, operatorID)
	if err != nil {
		return err
	}
	if !exists || operatorPrivilege < 2 {
		return ErrNoPermission
	}

	_, err = s.userRepo.FindByID(targetUserID)
	if err != nil {
		return userrepo.ErrUserNotFound
	}

	_, err = s.spaceRepo.FindSpaceUser(spaceID, targetUserID)
	if err == nil {
		return errors.New("user is already a member of this space")
	}

	spaceUser := &workspacemodel.SpaceUser{
		UserID:    targetUserID,
		SpaceID:   spaceID,
		Privilege: privilege,
	}
	return s.spaceRepo.AddSpaceUser(spaceUser)
}

func (s *SpaceService) RemoveSpaceMember(spaceID, operatorID, targetUserID int) error {
	exists, operatorPrivilege, err := s.spaceRepo.CheckSpacePrivilege(spaceID, operatorID)
	if err != nil {
		return err
	}
	if !exists || operatorPrivilege < 2 {
		return ErrNoPermission
	}

	if operatorID == targetUserID {
		return errors.New("cannot remove yourself")
	}

	creatorID, _ := s.spaceRepo.GetSpaceCreator(spaceID)
	if creatorID == targetUserID {
		return errors.New("cannot remove the space creator")
	}

	return s.spaceRepo.RemoveSpaceUser(spaceID, targetUserID)
}

func (s *SpaceService) UpdateMemberPrivilege(spaceID, operatorID, targetUserID, privilege int) error {
	exists, operatorPrivilege, err := s.spaceRepo.CheckSpacePrivilege(spaceID, operatorID)
	if err != nil {
		return err
	}
	if !exists || operatorPrivilege < 2 {
		return ErrNoPermission
	}

	if operatorID == targetUserID {
		return errors.New("cannot modify your own privilege")
	}

	creatorID, _ := s.spaceRepo.GetSpaceCreator(spaceID)
	if creatorID == targetUserID {
		return errors.New("cannot modify the space creator's privilege")
	}

	return s.spaceRepo.UpdateSpaceUserPrivilege(spaceID, targetUserID, privilege)
}

func (s *SpaceService) GetSpaceMembers(spaceID int) (map[string]interface{}, error) {
	spaceUsers, err := s.spaceRepo.GetSpaceUsers(spaceID)
	if err != nil {
		return nil, err
	}

	userIDs := make([]int, len(spaceUsers))
	memberIDSet := make(map[int]bool, len(spaceUsers))
	for i, su := range spaceUsers {
		userIDs[i] = su.UserID
		memberIDSet[su.UserID] = true
	}

	users, err := s.userRepo.FindByIDs(userIDs)
	if err != nil {
		return nil, err
	}

	userMap := make(map[int]*usermodel.User, len(users))
	for _, u := range users {
		userMap[u.ID] = u
	}

	responses := make([]*workspacemodel.SpaceUserResponse, len(spaceUsers))
	for i, su := range spaceUsers {
		resp := &workspacemodel.SpaceUserResponse{
			ID:         su.SpaceUserID,
			UserID:     su.UserID,
			SpaceID:    su.SpaceID,
			Privilege:  su.Privilege,
			CreateTime: su.CreateTime,
			UpdateTime: su.UpdateTime,
		}
		if user, ok := userMap[su.UserID]; ok {
			resp.Username = user.Username
			resp.GivenName = user.GivenName
			resp.Email = user.Email
			resp.Department = user.Department
		}
		responses[i] = resp
	}

	allUsers, err := s.userRepo.FindAllForExport()
	if err != nil {
		return nil, err
	}

	// other_users 仅服务于「添加成员」选择器：只回传定位所需的精简字段。
	// 旧实现直接回传完整 User 模型（email/mobile/department/position/last_ip 等 PII），
	// 既放大了敏感信息暴露面，也让响应体随用户量线性膨胀。
	type userBrief struct {
		ID        int    `json:"id"`
		Username  string `json:"username"`
		GivenName string `json:"given_name"`
	}
	otherUsers := make([]userBrief, 0, len(allUsers))
	for _, u := range allUsers {
		if !memberIDSet[u.ID] {
			otherUsers = append(otherUsers, userBrief{ID: u.ID, Username: u.Username, GivenName: u.GivenName})
		}
	}

	return map[string]interface{}{
		"members":     responses,
		"other_users": otherUsers,
	}, nil
}

func (s *SpaceService) CheckSpaceAccess(spaceID, userID int) (bool, error) {
	space, err := s.spaceRepo.FindByID(spaceID)
	if err != nil {
		return false, err
	}

	// admin/super_admin 有所有空间的访问权
	if userID > 0 {
		user, err := s.userRepo.FindByID(userID)
		if err == nil && (user.RoleID == 1 || user.RoleID == 2) {
			return true, nil
		}
	}

	// visit_level: 0/public=所有人可见, 1/login=登录用户可见, 2/member=仅空间成员可见
	if space.IsPublic() {
		return true, nil
	}
	if space.IsLoggedInVisible() {
		if userID == 0 {
			return false, ErrNotSpaceMember
		}
		return true, nil
	}
	// 其余（"2"/"member"/"private" 及未知值）：仅空间成员可见

	exists, _, err := s.spaceRepo.CheckSpacePrivilege(spaceID, userID)
	if err != nil {
		return false, err
	}
	if !exists {
		return false, ErrNotSpaceMember
	}
	return true, nil
}

func (s *SpaceService) GetUserSpacePrivilege(spaceID, userID int) (int, error) {
	user, err := s.userRepo.FindByID(userID)
	if err != nil {
		return 0, err
	}
	if user.RoleID == 1 {
		return 2, nil
	}

	exists, privilege, err := s.spaceRepo.CheckSpacePrivilege(spaceID, userID)
	if err != nil {
		return 0, err
	}
	if !exists {
		return 0, ErrNotSpaceMember
	}
	return privilege, nil
}

func (s *SpaceService) SearchTags(keyword string, limit int) ([]string, error) {
	if limit <= 0 {
		limit = 10
	}
	return s.spaceRepo.SearchByTag(keyword, limit)
}

// packMarkdownsFromDB db 模式导出：正文来自 aw_document_content，zip 内路径与 file 模式一致。
func (s *SpaceService) packMarkdownsFromDB(space *workspacemodel.Space, zw *zip.Writer) error {
	docs, err := s.docRepo.FindBySpaceID(space.ID)
	if err != nil {
		return err
	}
	for _, doc := range docs {
		if doc.IsDelete == 1 {
			continue
		}
		pageFile := getPageFilePath(doc, s.docRepo, s.spaceRepo, s.docStorage)
		content, err := doccontent.Get(doc.ID, s.docStorage, pageFile)
		if err != nil {
			return fmt.Errorf("read document %d: %w", doc.ID, err)
		}
		if content == "" {
			continue
		}
		w, err := zw.Create(pageFile)
		if err != nil {
			return err
		}
		if _, err := w.Write([]byte(content)); err != nil {
			return err
		}
	}
	return nil
}

func (s *SpaceService) DownloadSpace(spaceID int) (string, string, error) {
	space, err := s.spaceRepo.FindByID(spaceID)
	if err != nil {
		return "", "", err
	}
	if space.IsExport == 0 {
		return "", "", ErrSpaceExportDisabled
	}
	spaceDirPath := filepath.Join(s.docStorage.DocDir, space.Name)

	tmpDir := os.TempDir()
	// 临时文件必须唯一化（spaceID + 纳秒时间戳）：旧实现固定 <spaceName>.zip，
	// 并发导出同一空间时两个 zip.Writer 交替写同一文件，产出中心目录损坏的压缩包。
	zipPath := filepath.Join(tmpDir, "citebase", fmt.Sprintf("space_%d_export_%d.zip", spaceID, time.Now().UnixNano()))

	if err := os.MkdirAll(filepath.Dir(zipPath), 0755); err != nil {
		return "", "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	zipFile, err := os.Create(zipPath)
	if err != nil {
		return "", "", fmt.Errorf("failed to create zip file: %w", err)
	}

	zipWriter := zip.NewWriter(zipFile)

	if doccontent.UseDB() {
		if dbErr := s.packMarkdownsFromDB(space, zipWriter); dbErr != nil {
			zipWriter.Close()
			zipFile.Close()
			os.Remove(zipPath)
			return "", "", fmt.Errorf("failed to pack space content: %w", dbErr)
		}
	} else {
		err = filepath.Walk(spaceDirPath, func(filePath string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(spaceDirPath, filePath)
			if err != nil {
				return err
			}

			zipEntryPath := filepath.Join(space.Name, relPath)

			w, err := zipWriter.Create(zipEntryPath)
			if err != nil {
				return err
			}

			f, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer f.Close()

			_, err = io.Copy(w, f)
			return err
		})
	}

	if err != nil {
		zipWriter.Close()
		zipFile.Close()
		os.Remove(zipPath)
		return "", "", fmt.Errorf("failed to pack space files: %w", err)
	}

	docs, _ := s.docRepo.FindBySpaceID(spaceID)
	attachRepo := workspacerepo.NewAttachmentRepository()
	docIDs := make([]int, 0, len(docs))
	for _, doc := range docs {
		docIDs = append(docIDs, doc.ID)
	}
	// 一次批量取回全部附件，替代逐文档 FindByDocumentID 的 N+1
	attachments, _ := attachRepo.FindByDocIDs(docIDs)
	usedAttNames := make(map[string]bool, len(attachments))
	for _, att := range attachments {
		if att.Path == "" {
			continue
		}
		// P3：db 模式且已入库（kind=1）直接流式读块表，不再依赖磁盘文件；
		// file 模式与未迁移记录仍按 path 前缀解析磁盘路径（旧图片在 images/、其余在 attachment/）
		dbChunk := workspacerepo.UseDB() && att.StorageKind == workspacemodel.AttachmentStorageDB
		attFilePath := ""
		if !dbChunk {
			attFilePath = ResolveAttachmentPath(att.Path)
			if attFilePath == "" {
				continue
			}
			if _, err := os.Stat(attFilePath); err != nil {
				continue
			}
		}
		attName := att.Name
		if attName == "" {
			attName = filepath.Base(att.Path)
		}
		zipAttPath := filepath.Join(space.Name, "attachments", attName)
		if usedAttNames[zipAttPath] {
			zipAttPath = filepath.Join(space.Name, "attachments", fmt.Sprintf("%d_%s", att.ID, attName))
		}
		usedAttNames[zipAttPath] = true
		w, err := zipWriter.Create(zipAttPath)
		if err != nil {
			continue
		}
		if dbChunk {
			if err := attachRepo.StreamAttachmentChunks(att.ID, w); err != nil {
				continue
			}
			continue
		}
		f, err := os.Open(attFilePath)
		if err != nil {
			continue
		}
		_, _ = io.Copy(w, f)
		f.Close()
	}

	// zip 的中央目录由 Close 写入：Close/文件句柄失败必须报错并删除半成品，
	// 否则返回一个「物理存在但不可解压」的 zip，管理员却以为导出成功。
	if err := zipWriter.Close(); err != nil {
		zipFile.Close()
		os.Remove(zipPath)
		return "", "", fmt.Errorf("failed to finalize zip: %w", err)
	}
	if err := zipFile.Close(); err != nil {
		os.Remove(zipPath)
		return "", "", fmt.Errorf("failed to close zip file: %w", err)
	}
	// handler 发送完成后无人清理临时文件，目录会随导出次数无限增长：延迟删除兜底
	// （保留 30 分钟覆盖慢速下载；彻底方案见待办——handler 发送后即删）
	time.AfterFunc(30*time.Minute, func() { _ = os.Remove(zipPath) })

	return zipPath, space.Name + ".zip", nil
}

func ParseTags(tags string) []string {
	if tags == "" {
		return []string{}
	}
	parts := strings.Split(tags, ",")
	result := make([]string, 0)
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}
