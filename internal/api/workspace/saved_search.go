package workspace

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/middleware"
	"github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
)

// SavedSearchAPI handles saved search API requests.
type SavedSearchAPI struct {
	service *workspacesvc.SavedSearchService
}

func NewSavedSearchAPI() *SavedSearchAPI {
	return &SavedSearchAPI{
		service: workspacesvc.NewSavedSearchService(),
	}
}

// CreateSavedSearch creates a new saved search.
func (api *SavedSearchAPI) CreateSavedSearch(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)

	var req workspace.CreateSavedSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := api.service.CreateSavedSearch(int64(userID), &req)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, result)
}

// GetUserSavedSearches returns all saved searches for the current user.
func (api *SavedSearchAPI) GetUserSavedSearches(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)

	searches, err := api.service.GetUserSavedSearches(int64(userID))
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, searches)
}

// GetPublicSavedSearches returns popular public saved searches.
func (api *SavedSearchAPI) GetPublicSavedSearches(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	limit, _ := strconv.Atoi(limitStr)
	if limit < 1 || limit > 50 {
		limit = 10
	}

	searches, err := api.service.GetPublicSavedSearches(limit)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, searches)
}

// GetSavedSearch returns a saved search by ID.
func (api *SavedSearchAPI) GetSavedSearch(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	result, err := api.service.GetSavedSearch(id)
	if err != nil {
		response.NotFound(c, "saved search not found")
		return
	}

	response.Success(c, result)
}

// UpdateSavedSearch updates a saved search.
func (api *SavedSearchAPI) UpdateSavedSearch(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	var req workspace.UpdateSavedSearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := api.service.UpdateSavedSearch(int64(userID), id, &req)
	if err != nil {
		if err == workspacesvc.ErrSavedSearchNotFound {
			response.NotFound(c, "saved search not found")
			return
		}
		if err == workspacesvc.ErrSavedSearchNotOwner {
			response.Forbidden(c, "not your saved search")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, result)
}

// UseSavedSearch increments use count and returns the search.
func (api *SavedSearchAPI) UseSavedSearch(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	result, err := api.service.UseSavedSearch(id)
	if err != nil {
		response.NotFound(c, "saved search not found")
		return
	}

	response.Success(c, result)
}

// DeleteSavedSearch deletes a saved search.
func (api *SavedSearchAPI) DeleteSavedSearch(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if err := api.service.DeleteSavedSearch(int64(userID), id); err != nil {
		if err == workspacesvc.ErrSavedSearchNotFound {
			response.NotFound(c, "saved search not found")
			return
		}
		if err == workspacesvc.ErrSavedSearchNotOwner {
			response.Forbidden(c, "not your saved search")
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, nil)
}
