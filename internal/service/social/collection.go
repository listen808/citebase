package social

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	socialmodel "github.com/listen808/citebase/internal/model/social"
	socialrepo "github.com/listen808/citebase/internal/repository/social"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

var (
	ErrDocumentNotFound = errors.New("document not found")
	ErrSpaceNotFound    = errors.New("space not found")
)

// CollectionService 收藏业务逻辑层
type CollectionService struct {
	colRepo   *socialrepo.CollectionRepository
	docRepo   *workspacerepo.DocumentRepository
	spaceRepo *workspacerepo.SpaceRepository
}

func NewCollectionService() *CollectionService {
	return &CollectionService{
		colRepo:   socialrepo.NewCollectionRepository(),
		docRepo:   workspacerepo.NewDocumentRepository(),
		spaceRepo: workspacerepo.NewSpaceRepository(),
	}
}

// ToggleCollection 切换收藏状态，返回 (是否已收藏, 收藏ID, 错误)
func (s *CollectionService) ToggleCollection(userID, resourceType, resourceID int) (bool, int, error) {
	if resourceType == socialmodel.CollectionTypeDocument {
		doc, err := s.docRepo.FindByID(resourceID)
		if err != nil || doc == nil || doc.IsDelete == 1 {
			return false, 0, ErrDocumentNotFound
		}
	} else if resourceType == socialmodel.CollectionTypeSpace {
		space, err := s.spaceRepo.FindByID(resourceID)
		if err != nil || space == nil || space.IsDelete == 1 {
			return false, 0, ErrSpaceNotFound
		}
	}

	existing, err := s.colRepo.FindByUserAndResource(userID, resourceType, resourceID)
	if err != nil {
		return false, 0, err
	}

	if existing != nil {
		if err := s.colRepo.Delete(existing.CollectionID); err != nil {
			return false, 0, err
		}
		return false, existing.CollectionID, nil
	}

	col := &socialmodel.Collection{
		UserID:     userID,
		Type:       resourceType,
		ResourceID: resourceID,
	}
	if err := s.colRepo.Create(col); err != nil {
		// 上面「查一次没有再插一条」之间的窗口里，另一个请求（双击 / 两个标签页）
		// 可能已经插进去了，于是这条撞 uk_user_resource 的 1062。
		// 那是"已经收藏了"，不是失败 —— 把状态读回来按正常结果返回。
		if socialrepo.IsDuplicateKey(err) {
			won, findErr := s.colRepo.FindByUserAndResource(userID, resourceType, resourceID)
			if findErr == nil && won != nil {
				return true, won.CollectionID, nil
			}
		}
		return false, 0, err
	}
	return true, col.CollectionID, nil
}

// GetMyCollections 获取我的收藏列表
func (s *CollectionService) GetMyCollections(userID, colType int, keyword string, page, pageSize int) ([]gin.H, int, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// keyword 非空时必须「先过滤再分页」：旧实现在 DB 分页之后才在内存里按关键词丢弃条目，
	// total 仍是未过滤总数，前端按 total 算页数会出现大量空页/漏条目。
	if keyword != "" {
		const keywordScanLimit = 2000
		all, _, err := s.colRepo.GetUserCollections(userID, colType, keywordScanLimit, 0)
		if err != nil {
			return nil, 0, err
		}
		filtered := s.buildCollectionItems(all, keyword)
		total := len(filtered)
		start := (page - 1) * pageSize
		if start > total {
			start = total
		}
		end := start + pageSize
		if end > total {
			end = total
		}
		return filtered[start:end], total, nil
	}

	offset := (page - 1) * pageSize
	collections, total, err := s.colRepo.GetUserCollections(userID, colType, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}
	return s.buildCollectionItems(collections, ""), int(total), nil
}

// buildCollectionItems 组装收藏条目：按类型批量取文档/空间名（旧实现每行单独 FindByID，N+1）。
// keyword 非空时在构造阶段过滤，返回值即最终可见集合。
func (s *CollectionService) buildCollectionItems(collections []*socialmodel.Collection, keyword string) []gin.H {
	var docIDs, spaceIDs []int
	for _, col := range collections {
		if col.Type == socialmodel.CollectionTypeDocument {
			docIDs = append(docIDs, col.ResourceID)
		} else if col.Type == socialmodel.CollectionTypeSpace {
			spaceIDs = append(spaceIDs, col.ResourceID)
		}
	}

	type refInfo struct {
		name      string
		spaceID   int
		spaceName string
	}
	refs := make(map[[2]int]refInfo, len(docIDs)+len(spaceIDs))
	lowerKW := strings.ToLower(keyword)

	if len(spaceIDs) > 0 {
		if spaces, err := s.spaceRepo.FindByIDs(spaceIDs); err == nil {
			for _, sp := range spaces {
				refs[[2]int{socialmodel.CollectionTypeSpace, sp.ID}] = refInfo{name: sp.Name, spaceID: sp.ID, spaceName: sp.Name}
			}
		}
	}
	if len(docIDs) > 0 {
		docs, err := s.docRepo.FindByIDs(docIDs)
		if err != nil {
			docs = nil
		}
		// 文档的空间名批量补齐
		var docSpaceIDs []int
		for _, d := range docs {
			if d.SpaceID > 0 {
				docSpaceIDs = append(docSpaceIDs, d.SpaceID)
			}
		}
		spaceNames := make(map[int]string, len(docSpaceIDs))
		if len(docSpaceIDs) > 0 {
			if sps, serr := s.spaceRepo.FindByIDs(docSpaceIDs); serr == nil {
				for _, sp := range sps {
					spaceNames[sp.ID] = sp.Name
				}
			}
		}
		for _, d := range docs {
			refs[[2]int{socialmodel.CollectionTypeDocument, d.ID}] = refInfo{
				name: d.Name, spaceID: d.SpaceID, spaceName: spaceNames[d.SpaceID],
			}
		}
	}

	items := make([]gin.H, 0, len(collections))
	for _, col := range collections {
		info := refs[[2]int{col.Type, col.ResourceID}]
		if keyword != "" && !strings.Contains(strings.ToLower(info.name), lowerKW) {
			continue
		}
		item := gin.H{
			"id":          col.CollectionID,
			"type":        col.Type,
			"resource_id": col.ResourceID,
			"create_time": col.CreateTime,
			"name":        info.name,
		}
		if info.spaceID > 0 {
			item["space_id"] = info.spaceID
		}
		if info.spaceName != "" {
			item["space_name"] = info.spaceName
		}
		items = append(items, item)
	}
	return items
}

// IsCollected 检查是否已收藏
func (s *CollectionService) IsCollected(userID, colType, resourceID int) (bool, int, error) {
	existing, err := s.colRepo.FindByUserAndResource(userID, colType, resourceID)
	if err != nil {
		return false, 0, err
	}
	if existing != nil {
		return true, existing.CollectionID, nil
	}
	return false, 0, nil
}
