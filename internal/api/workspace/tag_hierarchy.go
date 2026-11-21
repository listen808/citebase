package workspace

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
)

// TagHierarchyAPI handles tag hierarchy API requests.
type TagHierarchyAPI struct {
	service *workspacesvc.TagHierarchyService
}

func NewTagHierarchyAPI() *TagHierarchyAPI {
	return &TagHierarchyAPI{
		service: workspacesvc.NewTagHierarchyService(),
	}
}

// CreateHierarchy creates a parent-child tag relationship.
func (api *TagHierarchyAPI) CreateHierarchy(c *gin.Context) {
	var req workspace.CreateTagHierarchyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	result, err := api.service.CreateHierarchy(req.ParentTag, req.ChildTag, req.SpaceID)
	if err != nil {
		if err == workspacesvc.ErrTagCircularReference {
			response.BadRequest(c, "circular tag reference detected")
			return
		}
		if err == workspacesvc.ErrTagHierarchyExists {
			response.ErrorWithErr(c, err, 409)
			return
		}
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, result)
}

// GetChildren returns all children of a tag.
func (api *TagHierarchyAPI) GetChildren(c *gin.Context) {
	parentTag := c.Param("tag")
	if parentTag == "" {
		response.BadRequest(c, "tag name required")
		return
	}

	spaceIDStr := c.DefaultQuery("space_id", "0")
	spaceID, _ := strconv.ParseInt(spaceIDStr, 10, 64)

	children, err := api.service.GetChildren(parentTag, spaceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, children)
}

// GetParent returns the parent of a tag.
func (api *TagHierarchyAPI) GetParent(c *gin.Context) {
	childTag := c.Param("tag")
	if childTag == "" {
		response.BadRequest(c, "tag name required")
		return
	}

	parent, err := api.service.GetParent(childTag)
	if err != nil {
		response.NotFound(c, "no parent found")
		return
	}

	response.Success(c, parent)
}

// GetTagTree returns the full tag hierarchy as a tree.
func (api *TagHierarchyAPI) GetTagTree(c *gin.Context) {
	spaceIDStr := c.DefaultQuery("space_id", "0")
	spaceID, _ := strconv.ParseInt(spaceIDStr, 10, 64)

	tree, err := api.service.GetTagTree(spaceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, tree)
}

// DeleteHierarchy removes a tag hierarchy entry.
func (api *TagHierarchyAPI) DeleteHierarchy(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	if err := api.service.DeleteHierarchy(id); err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	response.Success(c, nil)
}
