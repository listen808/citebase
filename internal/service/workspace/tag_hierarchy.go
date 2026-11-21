package workspace

import (
	"errors"

	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

var ErrTagCircularReference = errors.New("circular tag reference detected")
var ErrTagHierarchyExists = errors.New("tag hierarchy already exists")

// TagHierarchyService handles tag hierarchy business logic.
type TagHierarchyService struct {
	hierarchyRepo *workspacerepo.TagHierarchyRepository
}

func NewTagHierarchyService() *TagHierarchyService {
	return &TagHierarchyService{
		hierarchyRepo: workspacerepo.NewTagHierarchyRepository(),
	}
}

// CreateHierarchy creates a parent-child relationship between tags.
func (s *TagHierarchyService) CreateHierarchy(parentTag, childTag string, spaceID int64) (*workspacemodel.TagHierarchyResponse, error) {
	if parentTag == childTag {
		return nil, ErrTagCircularReference
	}

	// Check if child already has a parent
	existing, _ := s.hierarchyRepo.FindByChildTag(childTag)
	if existing != nil {
		return nil, ErrTagHierarchyExists
	}

	// Check for circular reference
	if s.wouldCreateCycle(parentTag, childTag) {
		return nil, ErrTagCircularReference
	}

	hierarchy := &workspacemodel.TagHierarchy{
		ParentTag: parentTag,
		ChildTag:  childTag,
		SpaceID:   spaceID,
	}

	if err := s.hierarchyRepo.Create(hierarchy); err != nil {
		return nil, err
	}

	return &workspacemodel.TagHierarchyResponse{
		ID:        hierarchy.ID,
		ParentTag: hierarchy.ParentTag,
		ChildTag:  hierarchy.ChildTag,
		SpaceID:   hierarchy.SpaceID,
	}, nil
}

// GetChildren returns all children of a tag.
func (s *TagHierarchyService) GetChildren(parentTag string, spaceID int64) ([]workspacemodel.TagHierarchyResponse, error) {
	items, err := s.hierarchyRepo.FindByParentTag(parentTag, spaceID)
	if err != nil {
		return nil, err
	}

	result := make([]workspacemodel.TagHierarchyResponse, 0, len(items))
	for _, item := range items {
		result = append(result, workspacemodel.TagHierarchyResponse{
			ID:        item.ID,
			ParentTag: item.ParentTag,
			ChildTag:  item.ChildTag,
			SpaceID:   item.SpaceID,
		})
	}
	return result, nil
}

// GetParent returns the parent of a tag.
func (s *TagHierarchyService) GetParent(childTag string) (*workspacemodel.TagHierarchyResponse, error) {
	item, err := s.hierarchyRepo.FindByChildTag(childTag)
	if err != nil {
		return nil, err
	}

	return &workspacemodel.TagHierarchyResponse{
		ID:        item.ID,
		ParentTag: item.ParentTag,
		ChildTag:  item.ChildTag,
		SpaceID:   item.SpaceID,
	}, nil
}

// GetTagTree returns the full tag hierarchy as a tree structure.
func (s *TagHierarchyService) GetTagTree(spaceID int64) ([]*workspacemodel.TagTreeNode, error) {
	allItems, err := s.hierarchyRepo.GetAll(spaceID)
	if err != nil {
		return nil, err
	}

	// Build parent -> children map
	childrenMap := make(map[string][]string)
	for _, item := range allItems {
		childrenMap[item.ParentTag] = append(childrenMap[item.ParentTag], item.ChildTag)
	}

	// Find root tags (not children of any other tag)
	childSet := make(map[string]bool)
	for _, item := range allItems {
		childSet[item.ChildTag] = true
	}

	var rootTags []string
	seen := make(map[string]bool)
	for _, item := range allItems {
		if !childSet[item.ParentTag] && !seen[item.ParentTag] {
			rootTags = append(rootTags, item.ParentTag)
			seen[item.ParentTag] = true
		}
	}

	// Build tree recursively（visited 防脏数据环导致无限递归）
	var buildNode func(tag string, visited map[string]bool) *workspacemodel.TagTreeNode
	buildNode = func(tag string, visited map[string]bool) *workspacemodel.TagTreeNode {
		node := &workspacemodel.TagTreeNode{TagName: tag}
		if visited[tag] {
			return node
		}
		visited[tag] = true
		if children, ok := childrenMap[tag]; ok {
			for _, child := range children {
				node.Children = append(node.Children, buildNode(child, visited))
			}
		}
		return node
	}

	var tree []*workspacemodel.TagTreeNode
	visited := make(map[string]bool)
	for _, root := range rootTags {
		tree = append(tree, buildNode(root, visited))
	}

	// 环内节点选不出根（每个节点都是某人之子）：作为孤儿根补进树，避免静默丢数据
	for _, item := range allItems {
		if !visited[item.ParentTag] {
			visited[item.ParentTag] = true
			tree = append(tree, buildNode(item.ParentTag, visited))
		}
	}

	return tree, nil
}

// DeleteHierarchy removes a parent-child relationship.
func (s *TagHierarchyService) DeleteHierarchy(id int64) error {
	return s.hierarchyRepo.Delete(id)
}

// DeleteByParent removes all children of a tag.
func (s *TagHierarchyService) DeleteByParent(parentTag string) error {
	return s.hierarchyRepo.DeleteByParent(parentTag)
}

// wouldCreateCycle checks if adding parent->child would create a cycle.
// 方向必须从 parent 沿父链上溯：child 在此之前一定没有父节点（上面已拒绝），
// 旧实现从 child 向上 BFS 立即结束、恒返回 false，形如 C→X→P 再加 P→C 的环会被写入，
// 环内节点在 GetTagTree 中永远选不出根、整支静默消失。
func (s *TagHierarchyService) wouldCreateCycle(parent, child string) bool {
	visited := make(map[string]bool)
	current := parent
	for current != "" {
		if current == child {
			return true
		}
		if visited[current] {
			// 已有脏数据成环：直接判定为环，阻止继续加深
			return true
		}
		visited[current] = true

		hierarchy, err := s.hierarchyRepo.FindByChildTag(current)
		if err != nil || hierarchy == nil {
			break
		}
		current = hierarchy.ParentTag
	}

	return false
}
