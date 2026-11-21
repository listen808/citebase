package search

import (
	"context"
	"strconv"

	"github.com/blevesearch/bleve/v2"
	logx "github.com/listen808/citebase/internal/log"
)

// BleveAdapter wraps the existing Bleve search to implement FullTextSearcher.
type BleveAdapter struct{}

// NewBleveAdapter creates a new BleveAdapter.
func NewBleveAdapter() *BleveAdapter {
	return &BleveAdapter{}
}

// Search executes a Bleve full-text search and returns SearchHit results.
func (a *BleveAdapter) Search(ctx context.Context, query string, opts FullTextSearchOptions) ([]SearchHit, error) {
	if index == nil {
		ensureIndex()
	}
	if index == nil {
		return nil, nil
	}

	topK := opts.TopK
	if topK <= 0 {
		topK = 20
	}

	searchOpts := &SearchOptions{
		Query:   query,
		Type:    SearchTypeAll,
		SpaceID: 0,
		Limit:   topK,
	}

	// 用户请求的空间与用户可访问权限取交集，确保只返回有权访问的结果。
	// AllowedSpaceIDs 由 API 层注入，未设置时按 SpaceIDs 原样执行（向后兼容）。
	spaceIDs := opts.SpaceIDs
	if len(opts.AllowedSpaceIDs) > 0 {
		spaceIDs = intersectUint(spaceIDs, opts.AllowedSpaceIDs)
	}

	if len(spaceIDs) == 1 {
		searchOpts.SpaceID = int(spaceIDs[0])
	} else if len(spaceIDs) > 1 {
		sids := make([]int, 0, len(spaceIDs))
		for _, sid := range spaceIDs {
			sids = append(sids, int(sid))
		}
		searchOpts.SpaceIDs = sids
	}

	result, err := SearchDocumentsWithOptions(searchOpts)
	if err != nil {
		logx.Infof("[BleveAdapter] Search error: %v", err)
		return nil, err
	}

	return convertBleveResults(result), nil
}

// intersectUint returns elements that appear in both a and b.
func intersectUint(a, b []uint) []uint {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	// b 作为 lookup set（通常更小）
	bb := make(map[uint]struct{}, len(b))
	for _, v := range b {
		bb[v] = struct{}{}
	}
	var result []uint
	for _, v := range a {
		if _, ok := bb[v]; ok {
			result = append(result, v)
		}
	}
	return result
}

// convertBleveResults converts Bleve SearchResult to []SearchHit.
func convertBleveResults(result *bleve.SearchResult) []SearchHit {
	if result == nil || result.Hits == nil {
		return nil
	}

	hits := make([]SearchHit, 0, len(result.Hits))
	for _, hit := range result.Hits {
		if hit == nil {
			continue
		}

		var docID uint
		// Try to extract from fields first
		if hit.Fields != nil {
			if id, ok := hit.Fields["id"].(float64); ok {
				docID = uint(id)
			}
		}
		// Fallback to hit ID
		if docID == 0 {
			if parsed, err := strconv.Atoi(hit.ID); err == nil {
				docID = uint(parsed)
			}
		}
		if docID == 0 {
			continue
		}

		sh := SearchHit{
			DocumentID: docID,
			Score:      hit.Score,
		}

		// Extract snippet from content fragments
		if hit.Fragments != nil {
			if frags, ok := hit.Fragments["content"]; ok && len(frags) > 0 {
				sh.Snippet = frags[0]
			}
		}

		// Collect highlight strings
		if hit.Fragments != nil {
			if nameFrags, ok := hit.Fragments["name"]; ok {
				for _, f := range nameFrags {
					sh.Highlights = append(sh.Highlights, f)
				}
			}
			if contentFrags, ok := hit.Fragments["content"]; ok {
				for _, f := range contentFrags {
					sh.Highlights = append(sh.Highlights, f)
				}
			}
		}

		hits = append(hits, sh)
	}

	return hits
}
