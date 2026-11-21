package rag

import (
	"context"
	"fmt"
	"math"
	"sort"

	"github.com/listen808/citebase/internal/ai/provider"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
)

type Recommender struct {
	provider  provider.Provider
	docRepo   *workspacerepo.DocumentRepository
	spaceRepo *workspacerepo.SpaceRepository
}

func NewRecommender(p provider.Provider) *Recommender {
	return &Recommender{
		provider:  p,
		docRepo:   workspacerepo.NewDocumentRepository(),
		spaceRepo: workspacerepo.NewSpaceRepository(),
	}
}

type Recommendation struct {
	DocumentID   int     `json:"document_id"`
	DocumentName string  `json:"document_name"`
	SpaceID      int     `json:"space_id"`
	SpaceName    string  `json:"space_name"`
	Score        float64 `json:"score"`
	Reason       string  `json:"reason"`
}

func (r *Recommender) RecommendForUser(ctx context.Context, userID int, limit int) ([]Recommendation, error) {
	if limit <= 0 {
		limit = 5
	}

	recentDocs, err := r.docRepo.FindAllDocuments()
	if err != nil {
		return nil, err
	}

	// FindAllDocuments 是全库口径（含私有空间），必须先用 userID 的可见空间集合过滤，
	// 否则推荐会把他人私有文档的标题直接送进会话。
	visibleIDs, err := r.spaceRepo.FindAccessibleSpaceIDs(userID, false)
	if err != nil {
		return nil, err
	}
	visible := make(map[int]bool, len(visibleIDs))
	for _, id := range visibleIDs {
		visible[id] = true
	}
	accessibleDocs := make([]*workspacemodel.Document, 0, len(recentDocs))
	for _, doc := range recentDocs {
		if visible[doc.SpaceID] {
			accessibleDocs = append(accessibleDocs, doc)
		}
	}
	recentDocs = accessibleDocs

	if len(recentDocs) == 0 {
		return []Recommendation{}, nil
	}

	sort.Slice(recentDocs, func(i, j int) bool {
		return recentDocs[i].UpdateTime > recentDocs[j].UpdateTime
	})

	if len(recentDocs) > 20 {
		recentDocs = recentDocs[:20]
	}

	// 一条 IN 批量取名下空间，而不是每个文档查一次空间：
	// 这一屏推荐最多 20 条，原先就是 20 次空间查询（RecommendSimilar 同理，
	// 而它遍历的是"同空间全部文档"，大空间下能到几百次）。
	// 空间查不到时返回的是空 map，那条记录的 SpaceName 就是空串 ——
	// 与原实现 `space, _ := FindByID(...)` 忽略错误的表现一致，不新增失败面。
	spaceNames := r.spaceNamesByDocs(recentDocs)

	var recs []Recommendation
	for i, doc := range recentDocs {
		if i >= limit {
			break
		}
		score := 1.0 - float64(i)*0.1
		if score < 0.1 {
			score = 0.1
		}

		recs = append(recs, Recommendation{
			DocumentID:   doc.ID,
			DocumentName: doc.Name,
			SpaceID:      doc.SpaceID,
			SpaceName:    spaceNames[doc.SpaceID],
			Score:        math.Round(score*100) / 100,
			Reason:       "最近更新的文档",
		})
	}

	return recs, nil
}

// spaceNamesByDocs 按文档列表去重后批量取空间名。
func (r *Recommender) spaceNamesByDocs(docs []*workspacemodel.Document) map[int]string {
	ids := make([]int, 0, len(docs))
	seen := make(map[int]bool, len(docs))
	for _, d := range docs {
		if d == nil || seen[d.SpaceID] {
			continue
		}
		seen[d.SpaceID] = true
		ids = append(ids, d.SpaceID)
	}
	names := make(map[int]string, len(ids))
	if len(ids) == 0 {
		return names
	}
	spaces, err := r.spaceRepo.FindByIDs(ids)
	if err != nil {
		return names
	}
	for _, s := range spaces {
		if s != nil {
			names[s.ID] = s.Name
		}
	}
	return names
}

func (r *Recommender) RecommendSimilar(ctx context.Context, docID int, limit int) ([]Recommendation, error) {
	if limit <= 0 {
		limit = 5
	}

	doc, err := r.docRepo.FindByID(docID)
	if err != nil || doc == nil {
		return nil, fmt.Errorf("文档不存在")
	}

	sameSpaceDocs, err := r.docRepo.FindBySpaceID(doc.SpaceID)
	if err != nil {
		return nil, err
	}

	// 这个循环遍历的是同一个空间下的全部文档，空间名查一次就够 ——
	// 原先每条文档各查一次，大空间里一次推荐能打出几百次相同的空间查询。
	relatedSpaceName := ""
	if space, serr := r.spaceRepo.FindByID(doc.SpaceID); serr == nil && space != nil {
		relatedSpaceName = space.Name
	}

	var recs []Recommendation
	for _, d := range sameSpaceDocs {
		if d.ID == docID {
			continue
		}

		score := 0.8
		if d.ParentID == doc.ParentID {
			score = 0.95
		}

		recs = append(recs, Recommendation{
			DocumentID:   d.ID,
			DocumentName: d.Name,
			SpaceID:      d.SpaceID,
			SpaceName:    relatedSpaceName,
			Score:        score,
			Reason:       "同空间相关文档",
		})
	}

	sort.Slice(recs, func(i, j int) bool {
		return recs[i].Score > recs[j].Score
	})

	if len(recs) > limit {
		recs = recs[:limit]
	}

	return recs, nil
}
