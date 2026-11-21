package search

import (
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

// SearchRepository 搜索领域数据访问层
type SearchRepository struct {
	docRepo   *workspacerepo.DocumentRepository
	spaceRepo *workspacerepo.SpaceRepository
	tagRepo   *workspacerepo.DocumentTagRepository
}

func NewSearchRepository() *SearchRepository {
	return &SearchRepository{
		docRepo:   workspacerepo.NewDocumentRepository(),
		spaceRepo: workspacerepo.NewSpaceRepository(),
		tagRepo:   workspacerepo.NewDocumentTagRepository(repos.DB),
	}
}

func (r *SearchRepository) FindDocumentByID(id int) (*workspacemodel.Document, error) {
	return r.docRepo.FindByID(id)
}

func (r *SearchRepository) FindDocumentsByIDs(ids []int) ([]*workspacemodel.Document, error) {
	return r.docRepo.FindByIDs(ids)
}

func (r *SearchRepository) FindDocumentsBySpaceID(spaceID int) ([]*workspacemodel.Document, error) {
	return r.docRepo.FindBySpaceID(spaceID)
}

func (r *SearchRepository) FindAllDocumentsIncludeDirs() ([]*workspacemodel.Document, error) {
	return r.docRepo.FindAllDocumentsIncludeDirs()
}

func (r *SearchRepository) GetLatestVersion(docID int) (int, error) {
	return r.docRepo.GetLatestVersion(docID)
}

func (r *SearchRepository) GetVersionContent(docID int, version int) (string, error) {
	return r.docRepo.GetVersionContent(docID, version)
}

func (r *SearchRepository) FindSpaceByID(id int) (*workspacemodel.Space, error) {
	return r.spaceRepo.FindByIDCached(id)
}

func (r *SearchRepository) FindAllSpaces() ([]*workspacemodel.Space, error) {
	spaces, _, err := r.spaceRepo.FindAll(0, 1000, "", "", "")
	return spaces, err
}

func (r *SearchRepository) FindAccessibleSpaceIDs(userID int, isSuperAdmin bool) ([]int, error) {
	return r.spaceRepo.FindAccessibleSpaceIDs(userID, isSuperAdmin)
}

func (r *SearchRepository) CountAll() (int64, error) {
	return r.docRepo.CountAll()
}

// FindDocumentIDsByAuthor 按作者 ID 查找文档 ID 列表
func (r *SearchRepository) FindDocumentIDsByAuthor(authorID int) (map[int]bool, error) {
	if authorID <= 0 {
		return nil, nil
	}
	docs, err := r.docRepo.FindByCreateUserID(authorID)
	if err != nil {
		return nil, err
	}
	idSet := make(map[int]bool, len(docs))
	for _, doc := range docs {
		idSet[doc.ID] = true
	}
	return idSet, nil
}

// FindDocumentIDsByTag 按标签名查找文档 ID 集合
func (r *SearchRepository) FindDocumentIDsByTag(tagName string) (map[int]bool, error) {
	if tagName == "" {
		return nil, nil
	}
	docIDs, err := r.tagRepo.GetDocumentsByTag(tagName, 0)
	if err != nil {
		return nil, err
	}
	idSet := make(map[int]bool, len(docIDs))
	for _, id := range docIDs {
		idSet[id] = true
	}
	return idSet, nil
}

// GetAllTags 获取所有标签列表
func (r *SearchRepository) GetAllTags(keyword string, limit int) ([]string, error) {
	return r.tagRepo.GetAllTags(keyword, limit)
}
