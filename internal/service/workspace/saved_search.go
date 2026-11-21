package workspace

import (
	"errors"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

var ErrSavedSearchNotFound = errors.New("saved search not found")
var ErrSavedSearchNotOwner = errors.New("not the owner of this saved search")

// SavedSearchService handles saved search business logic.
type SavedSearchService struct {
	repo *workspacerepo.SavedSearchRepository
}

func NewSavedSearchService() *SavedSearchService {
	return &SavedSearchService{
		repo: workspacerepo.NewSavedSearchRepository(),
	}
}

// CreateSavedSearch creates a new saved search.
func (s *SavedSearchService) CreateSavedSearch(userID int64, req *workspacemodel.CreateSavedSearchRequest) (*workspacemodel.SavedSearchResponse, error) {
	search := &workspacemodel.SavedSearch{
		UserID:    userID,
		Name:      req.Name,
		Query:     req.Query,
		Type:      req.Type,
		SpaceID:   req.SpaceID,
		TagFilter: req.TagFilter,
		SortBy:    req.SortBy,
		IsPublic:  req.IsPublic,
	}

	if search.Type == "" {
		search.Type = "all"
	}
	if search.SortBy == "" {
		search.SortBy = "relevance"
	}

	if err := s.repo.Create(search); err != nil {
		return nil, err
	}

	return s.toResponse(search), nil
}

// GetUserSavedSearches returns all saved searches for a user.
func (s *SavedSearchService) GetUserSavedSearches(userID int64) ([]workspacemodel.SavedSearchResponse, error) {
	searches, err := s.repo.FindByUserID(userID)
	if err != nil {
		return nil, err
	}

	result := make([]workspacemodel.SavedSearchResponse, 0, len(searches))
	for i := range searches {
		result = append(result, *s.toResponse(&searches[i]))
	}
	return result, nil
}

// GetPublicSavedSearches returns popular public saved searches.
func (s *SavedSearchService) GetPublicSavedSearches(limit int) ([]workspacemodel.SavedSearchResponse, error) {
	searches, err := s.repo.FindPublic(limit)
	if err != nil {
		return nil, err
	}

	result := make([]workspacemodel.SavedSearchResponse, 0, len(searches))
	for i := range searches {
		result = append(result, *s.toResponse(&searches[i]))
	}
	return result, nil
}

// GetSavedSearch returns a saved search by ID.
func (s *SavedSearchService) GetSavedSearch(id int64) (*workspacemodel.SavedSearchResponse, error) {
	search, err := s.repo.FindByID(id)
	if err != nil {
		return nil, ErrSavedSearchNotFound
	}
	return s.toResponse(search), nil
}

// UpdateSavedSearch updates a saved search.
func (s *SavedSearchService) UpdateSavedSearch(userID, id int64, req *workspacemodel.UpdateSavedSearchRequest) (*workspacemodel.SavedSearchResponse, error) {
	search, err := s.repo.FindByID(id)
	if err != nil {
		return nil, ErrSavedSearchNotFound
	}

	if search.UserID != userID {
		return nil, ErrSavedSearchNotOwner
	}

	if req.Name != "" {
		search.Name = req.Name
	}
	if req.Query != "" {
		search.Query = req.Query
	}
	if req.Type != "" {
		search.Type = req.Type
	}
	if req.SpaceID != 0 {
		search.SpaceID = req.SpaceID
	}
	if req.TagFilter != "" {
		search.TagFilter = req.TagFilter
	}
	if req.SortBy != "" {
		search.SortBy = req.SortBy
	}
	search.IsPublic = req.IsPublic

	if err := s.repo.Update(search); err != nil {
		return nil, err
	}

	return s.toResponse(search), nil
}

// UseSavedSearch increments the use count and returns the search.
func (s *SavedSearchService) UseSavedSearch(id int64) (*workspacemodel.SavedSearchResponse, error) {
	search, err := s.repo.FindByID(id)
	if err != nil {
		return nil, ErrSavedSearchNotFound
	}

	_ = s.repo.IncrementUseCount(id)
	search.UseCount++

	return s.toResponse(search), nil
}

// DeleteSavedSearch deletes a saved search.
func (s *SavedSearchService) DeleteSavedSearch(userID, id int64) error {
	search, err := s.repo.FindByID(id)
	if err != nil {
		return ErrSavedSearchNotFound
	}

	if search.UserID != userID {
		return ErrSavedSearchNotOwner
	}

	return s.repo.Delete(id)
}

func (s *SavedSearchService) toResponse(search *workspacemodel.SavedSearch) *workspacemodel.SavedSearchResponse {
	return &workspacemodel.SavedSearchResponse{
		ID:         search.ID,
		UserID:     search.UserID,
		Name:       search.Name,
		Query:      search.Query,
		Type:       search.Type,
		SpaceID:    search.SpaceID,
		TagFilter:  search.TagFilter,
		SortBy:     search.SortBy,
		IsPublic:   search.IsPublic,
		UseCount:   search.UseCount,
		CreateTime: search.CreateTime,
		UpdateTime: search.UpdateTime,
	}
}
