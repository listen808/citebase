package workspace

import (
	"strconv"

	"github.com/gin-gonic/gin"
	workspace "github.com/listen808/citebase/internal/model/workspace"
	"github.com/listen808/citebase/internal/response"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
	"github.com/listen808/citebase/internal/utils"
)

// DocumentViewAPI handles document view mode API requests.
type DocumentViewAPI struct {
	docService   *workspacesvc.DocumentService
	spaceService *workspacesvc.SpaceService
}

func NewDocumentViewAPI() *DocumentViewAPI {
	return &DocumentViewAPI{
		docService:   workspacesvc.NewDocumentService(),
		spaceService: workspacesvc.NewSpaceService(),
	}
}

// DocumentListItem represents a document in list/gallery view.
type DocumentListItem struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	Type        int    `json:"type"`
	SpaceID     int    `json:"space_id"`
	SpaceName   string `json:"space_name"`
	Status      int    `json:"status"`
	ParentID    int    `json:"parent_id"`
	Sequence    int    `json:"sequence"`
	CreatedBy   int    `json:"created_by"`
	CreatorName string `json:"creator_name,omitempty"`
	UpdateTime  int    `json:"update_time"`
	PublishTime int    `json:"publish_time"`
}

// GetDocumentListView returns documents in flat list format for kanban/gallery views.
func (api *DocumentViewAPI) GetDocumentListView(c *gin.Context) {
	spaceIDStr := c.DefaultQuery("space_id", "0")
	spaceID, err := strconv.Atoi(spaceIDStr)
	if err != nil || spaceID == 0 {
		response.BadRequest(c, "invalid space_id")
		return
	}

	statusStr := c.DefaultQuery("status", "-1")
	status, _ := strconv.Atoi(statusStr)

	// 视图接口返回整棵文档树（标题/层级/父子），不校验空间访问权限等于让任意
	// 登录用户枚举他人私有空间的文档结构
	if !requireSpaceReadable(c, spaceID) {
		return
	}

	// 走共用的钳制入口：原先自己写 "page_size>100 就退回 20"，
	// 那和全站"超过 MaxPageSize 就钳到 MaxPageSize"是两种行为 ——
	// 传 500 的人拿到 20 条，而传 250 的人在别处拿到 200 条。
	page, pageSize, _ := utils.ParsePagination(c, 20)

	tree, err := api.docService.GetDocumentTree(spaceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	var items []DocumentListItem
	flattenTree(tree, &items, status)

	space, _ := api.spaceService.GetSpaceByID(spaceID)
	spaceName := ""
	if space != nil {
		spaceName = space.Name
	}

	for i := range items {
		items[i].SpaceName = spaceName
	}

	total := len(items)
	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	response.Paginated(c, items[start:end], int64(total), page, pageSize)
}

// GetDocumentKanbanView returns documents grouped by status for kanban view.
func (api *DocumentViewAPI) GetDocumentKanbanView(c *gin.Context) {
	spaceIDStr := c.DefaultQuery("space_id", "0")
	spaceID, err := strconv.Atoi(spaceIDStr)
	if err != nil || spaceID == 0 {
		response.BadRequest(c, "invalid space_id")
		return
	}

	if !requireSpaceReadable(c, spaceID) {
		return
	}

	tree, err := api.docService.GetDocumentTree(spaceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	kanban := map[string][]DocumentListItem{
		"draft":     {},
		"published": {},
	}

	var items []DocumentListItem
	flattenTree(tree, &items, -1)

	for _, item := range items {
		if item.Type == 2 {
			continue
		}
		if item.Status == 1 {
			kanban["published"] = append(kanban["published"], item)
		} else {
			kanban["draft"] = append(kanban["draft"], item)
		}
	}

	response.Success(c, kanban)
}

// GetDocumentGalleryView returns documents with cover images for gallery view.
func (api *DocumentViewAPI) GetDocumentGalleryView(c *gin.Context) {
	spaceIDStr := c.DefaultQuery("space_id", "0")
	spaceID, err := strconv.Atoi(spaceIDStr)
	if err != nil || spaceID == 0 {
		response.BadRequest(c, "invalid space_id")
		return
	}

	if !requireSpaceReadable(c, spaceID) {
		return
	}

	tree, err := api.docService.GetDocumentTree(spaceID)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}

	var items []DocumentListItem
	flattenTree(tree, &items, -1)

	var gallery []DocumentListItem
	for _, item := range items {
		if item.Type == 2 {
			continue
		}
		gallery = append(gallery, item)
	}

	if len(gallery) == 0 {
		for _, item := range items {
			if item.Type == 2 {
				continue
			}
			gallery = append(gallery, item)
		}
	}

	response.Success(c, gallery)
}

// flattenTree recursively flattens the document tree into a list.
func flattenTree(nodes []*workspace.DocumentResponse, items *[]DocumentListItem, statusFilter int) {
	for _, node := range nodes {
		status := 0
		if node.PublishedAt > 0 {
			status = 1
		}

		if statusFilter == -1 || status == statusFilter {
			*items = append(*items, DocumentListItem{
				ID:         node.ID,
				Name:       node.Name,
				Type:       node.Type,
				SpaceID:    node.SpaceID,
				Status:     status,
				ParentID:   node.ParentID,
				Sequence:   node.Sequence,
				UpdateTime: node.UpdateTime,
			})
		}

		if node.Children != nil {
			flattenTree(node.Children, items, statusFilter)
		}
	}
}
