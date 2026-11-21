package workspace

import (
	"errors"

	logx "github.com/listen808/citebase/internal/log"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

var ErrVersionTagExists = errors.New("version tag already exists for this name on this version")

// DocumentVersionTagService handles version tag business logic.
type DocumentVersionTagService struct {
	tagRepo    *workspacerepo.DocumentVersionTagRepository
	docService *DocumentService
}

func NewDocumentVersionTagService() *DocumentVersionTagService {
	return &DocumentVersionTagService{
		tagRepo:    workspacerepo.NewDocumentVersionTagRepository(),
		docService: NewDocumentService(),
	}
}

func NewDocumentVersionTagServiceWithDeps(
	tagRepo *workspacerepo.DocumentVersionTagRepository,
	docService *DocumentService,
) *DocumentVersionTagService {
	return &DocumentVersionTagService{
		tagRepo:    tagRepo,
		docService: docService,
	}
}

// CreateTag creates a new version tag for a specific document version.
func (s *DocumentVersionTagService) CreateTag(userID int64, documentID int64, req *workspacemodel.CreateVersionTagRequest) (*workspacemodel.DocumentVersionTagResponse, error) {
	doc, err := s.docService.GetDocumentRawByID(int(documentID))
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, errors.New("document not found")
	}

	// 必须校验调用者对文档所属空间有编辑权：否则任意登录用户可向他人文档添加标签（数据污染）
	privilege, err := s.docService.spaceService.GetUserSpacePrivilege(doc.SpaceID, int(userID))
	if err != nil {
		return nil, err
	}
	if privilege < 1 {
		return nil, ErrNoEditPermission
	}

	// Verify version exists (version 0 = current)
	tags, err := s.tagRepo.FindByDocumentAndVersion(documentID, req.Version)
	if err != nil {
		return nil, err
	}

	// Check duplicate tag name on same version
	for _, existing := range tags {
		if existing.TagName == req.TagName {
			return nil, ErrVersionTagExists
		}
	}

	tag := &workspacemodel.DocumentVersionTag{
		DocumentID: documentID,
		Version:    req.Version,
		TagName:    req.TagName,
		Color:      req.Color,
		Note:       req.Note,
		CreatedBy:  userID,
	}

	if err := s.tagRepo.Create(tag); err != nil {
		return nil, err
	}

	resp := tag.ToResponse()
	resp.CreatorName = s.resolveUserName(userID)
	return resp, nil
}

// GetDocumentVersionTags returns all version tags for a document.
func (s *DocumentVersionTagService) GetDocumentVersionTags(documentID int64) ([]workspacemodel.DocumentVersionTagResponse, error) {
	tags, err := s.tagRepo.FindByDocumentID(documentID)
	if err != nil {
		return nil, err
	}

	result := make([]workspacemodel.DocumentVersionTagResponse, 0, len(tags))
	// Batch resolve creator names
	userIDs := make([]int64, 0, len(tags))
	for _, t := range tags {
		userIDs = append(userIDs, t.CreatedBy)
	}
	nameMap := s.resolveUserNames(userIDs)

	for i := range tags {
		resp := tags[i].ToResponse()
		resp.CreatorName = nameMap[tags[i].CreatedBy]
		result = append(result, *resp)
	}
	return result, nil
}

// GetVersionTags returns tags for a specific version.
func (s *DocumentVersionTagService) GetVersionTags(documentID int64, version int) ([]workspacemodel.DocumentVersionTagResponse, error) {
	tags, err := s.tagRepo.FindByDocumentAndVersion(documentID, version)
	if err != nil {
		return nil, err
	}

	// 批量解析创建者名（旧实现循环内逐条查用户，N+1）
	userIDs := make([]int64, 0, len(tags))
	for _, t := range tags {
		userIDs = append(userIDs, t.CreatedBy)
	}
	nameMap := s.resolveUserNames(userIDs)

	result := make([]workspacemodel.DocumentVersionTagResponse, 0, len(tags))
	for i := range tags {
		resp := tags[i].ToResponse()
		resp.CreatorName = nameMap[tags[i].CreatedBy]
		result = append(result, *resp)
	}
	return result, nil
}

// GetSpaceVersionTags returns all version tags in a space (via document join).
func (s *DocumentVersionTagService) GetSpaceVersionTags(spaceID int64) ([]workspacemodel.DocumentVersionTagResponse, error) {
	// 旧实现：先取去重标签名，再对每个名字单独查询，循环内每条标签又单查一次用户（M+N+ΣK 次查询）。
	// 现在一次 JOIN 查询取回全部标签，创建者名统一批量解析。
	tags, err := s.tagRepo.FindBySpaceID(spaceID)
	if err != nil {
		return nil, err
	}

	userIDs := make([]int64, 0, len(tags))
	for _, t := range tags {
		userIDs = append(userIDs, t.CreatedBy)
	}
	nameMap := s.resolveUserNames(userIDs)

	result := make([]workspacemodel.DocumentVersionTagResponse, 0, len(tags))
	for i := range tags {
		resp := tags[i].ToResponse()
		resp.CreatorName = nameMap[tags[i].CreatedBy]
		result = append(result, *resp)
	}
	return result, nil
}

// DeleteTag removes a version tag.
func (s *DocumentVersionTagService) DeleteTag(tagID int64) error {
	return s.tagRepo.Delete(tagID)
}

// DeleteVersionTags removes all tags for a specific document version.
func (s *DocumentVersionTagService) DeleteVersionTags(documentID int64, version int) error {
	return s.tagRepo.DeleteByDocumentAndVersion(documentID, version)
}

func (s *DocumentVersionTagService) resolveUserName(userID int64) string {
	if userID == 0 {
		return ""
	}
	user, err := userrepo.NewUserRepository().FindByID(int(userID))
	if err != nil || user == nil {
		return ""
	}
	if user.GivenName != "" {
		return user.GivenName
	}
	return user.Username
}

func (s *DocumentVersionTagService) resolveUserNames(userIDs []int64) map[int64]string {
	result := make(map[int64]string, len(userIDs))
	if len(userIDs) == 0 {
		return result
	}
	// 去重后一次批量查询（旧实现虽名为 batch，内部仍逐条 FindByID，本质还是 N+1）
	seen := make(map[int64]bool, len(userIDs))
	ids := make([]int, 0, len(userIDs))
	for _, uid := range userIDs {
		if uid == 0 || seen[uid] {
			continue
		}
		seen[uid] = true
		ids = append(ids, int(uid))
	}
	if len(ids) == 0 {
		return result
	}
	users, err := userrepo.NewUserRepository().FindByIDs(ids)
	if err != nil {
		logx.Warnf("批量解析版本标签创建者失败: %v", err)
		return result
	}
	for _, u := range users {
		if u.GivenName != "" {
			result[int64(u.ID)] = u.GivenName
		} else {
			result[int64(u.ID)] = u.Username
		}
	}
	return result
}
