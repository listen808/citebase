package search

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	logx "github.com/listen808/citebase/internal/log"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	searchrepo "github.com/listen808/citebase/internal/repository/search"
)

// FullTextSearcher abstracts full-text (Bleve) search.
type FullTextSearcher interface {
	Search(ctx context.Context, query string, opts FullTextSearchOptions) ([]SearchHit, error)
}

// VectorSearcher abstracts vector (semantic) search.
type VectorSearcher interface {
	Search(ctx context.Context, queryEmbedding []float64, topK int) ([]VectorHit, error)
	Embed(ctx context.Context, text string) ([]float64, error)
}

// FullTextSearchOptions configures a full-text search request.
type FullTextSearchOptions struct {
	SpaceIDs         []uint
	AllowedSpaceIDs  []uint // 用户可访问的空间ID列表（用于前置过滤）
	CategoryIDs      []uint
	Tags             []string
	TopK             int
}

// SearchHit represents a single full-text search result.
type SearchHit struct {
	DocumentID uint
	Score      float64
	Snippet    string
	Highlights []string
}

// VectorHit represents a single vector search result.
type VectorHit struct {
	DocumentID uint
	SpaceID    uint
	Score      float64
}

// HybridSearchConfig holds configuration for hybrid search.
type HybridSearchConfig struct {
	FullTextWeight float64 `yaml:"fulltext_weight"` // default 0.5
	VectorWeight   float64 `yaml:"vector_weight"`   // default 0.5
	TopK           int     `yaml:"top_k"`           // default 20
	MinScore       float64 `yaml:"min_score"`       // default 0.1
	RRFRankK       int     `yaml:"rrf_rank_k"`      // default 60
}

// DefaultHybridSearchConfig returns sensible defaults.
func DefaultHybridSearchConfig() HybridSearchConfig {
	return HybridSearchConfig{
		FullTextWeight: 0.5,
		VectorWeight:   0.5,
		TopK:           20,
		MinScore:       0.0,
		RRFRankK:       60,
	}
}

// HybridSearchRequest is the inbound request for hybrid search.
type HybridSearchRequest struct {
	Query       string   `json:"query" binding:"required"`
	SpaceIDs    []uint   `json:"space_ids,omitempty"`    // 用户请求的空间ID列表（API层已与权限取交集）
	AllowedSpaceIDs []uint `json:"allowed_space_ids,omitempty"` // 用户的有效权限空间ID列表（API层计算后注入）
	CategoryIDs []uint   `json:"category_ids,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	DateFrom    string   `json:"date_from,omitempty"`
	DateTo      string   `json:"date_to,omitempty"`
	SearchMode  string   `json:"search_mode"` // "hybrid", "fulltext", "semantic"
	Page        int      `json:"page"`
	PageSize    int      `json:"page_size"`
}

// HybridSearchResult is a single result in the hybrid search response.
type HybridSearchResult struct {
	DocumentID   uint        `json:"document_id"`
	Title        string      `json:"title"`
	Snippet      string      `json:"snippet"`
	Score        float64     `json:"score"`
	ScoreDetail  ScoreDetail `json:"score_detail"`
	Highlights   []string    `json:"highlights"`
	Author       string      `json:"author"`
	SpaceName    string      `json:"space_name"`
	CategoryName string      `json:"category_name"`
	UpdatedAt    string      `json:"updated_at"`
	SpaceID      int         `json:"space_id"`
	Type         int         `json:"type"`
	Path         string      `json:"path"`
}

// ScoreDetail provides transparency into how the final score was computed.
type ScoreDetail struct {
	FullTextScore float64 `json:"fulltext_score"`
	VectorScore   float64 `json:"vector_score"`
	FusionMethod  string  `json:"fusion_method"`
}

// HybridSearchService combines full-text and vector search.
type HybridSearchService struct {
	fullTextSearcher FullTextSearcher
	vectorSearcher   VectorSearcher
	config           HybridSearchConfig
	searchRepo       *searchrepo.SearchRepository
}

// NewHybridSearchService creates a HybridSearchService.
// vectorSearcher may be nil (in which case only fulltext search is used).
// searchRepo may be nil (enrichment will be skipped).
func NewHybridSearchService(ft FullTextSearcher, vs VectorSearcher, cfg HybridSearchConfig, repo *searchrepo.SearchRepository) *HybridSearchService {
	return &HybridSearchService{
		fullTextSearcher: ft,
		vectorSearcher:   vs,
		config:           cfg,
		searchRepo:       repo,
	}
}

// Search executes a hybrid search request and returns enriched results plus the actual mode executed.
func (s *HybridSearchService) Search(ctx context.Context, req *HybridSearchRequest) ([]HybridSearchResult, int, string, error) {
	if req.Query == "" {
		return nil, 0, "", fmt.Errorf("empty query")
	}

	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = s.config.TopK
	}
	if pageSize > 100 {
		pageSize = 100
	}
	page := req.Page
	if page <= 0 {
		page = 1
	}

	// Determine search mode
	intent := DetectIntent(req.Query)
	mode := SelectSearchMode(intent, req.SearchMode)

	logx.Infof("[HybridSearch] query=%q intent=%s mode=%s", req.Query, intent, mode)

	// Fetch candidates from both search systems
	topK := pageSize * 3 // over-fetch for fusion
	if topK < 30 {
		topK = 30
	}

	var ftResults []SearchHit
	var vecResults []VectorHit
	var ftErr, vecErr error

	ftOpts := FullTextSearchOptions{
		SpaceIDs:    req.SpaceIDs,
		Tags:        req.Tags,
		TopK:        topK,
		AllowedSpaceIDs: req.AllowedSpaceIDs,
	}

	switch mode {
	case ModeFulltext:
		ftResults, _ = s.fullTextSearcher.Search(ctx, req.Query, ftOpts)

	case ModeSemantic:
		if s.vectorSearcher == nil {
			// Fallback to fulltext if vector search unavailable
			logx.Infof("[HybridSearch] Vector searcher unavailable, falling back to fulltext")
			ftResults, _ = s.fullTextSearcher.Search(ctx, req.Query, ftOpts)
			mode = ModeFulltext
		} else {
			var queryVec []float64
			queryVec, vecErr = s.vectorSearcher.Embed(ctx, req.Query)
			if vecErr == nil {
				vecResults, vecErr = s.vectorSearcher.Search(ctx, queryVec, topK)
			}
			if vecErr != nil {
				logx.Infof("[HybridSearch] Vector search error, falling back to fulltext: %v", vecErr)
				ftResults, _ = s.fullTextSearcher.Search(ctx, req.Query, ftOpts)
				mode = ModeFulltext
			}
		}

	case ModeHybrid:
		// Run both searches
		ftResults, ftErr = s.fullTextSearcher.Search(ctx, req.Query, ftOpts)
		if ftErr != nil {
			logx.Infof("[HybridSearch] Fulltext search error: %v", ftErr)
		}

		if s.vectorSearcher != nil {
			var queryVec []float64
			queryVec, vecErr = s.vectorSearcher.Embed(ctx, req.Query)
			if vecErr == nil {
				vecResults, vecErr = s.vectorSearcher.Search(ctx, queryVec, topK)
			}
			if vecErr != nil {
				logx.Infof("[HybridSearch] Vector search error (continuing with fulltext only): %v", vecErr)
			}
		}
	}

	// 前置过滤：向量结果必须落入用户可访问的空间列表。
	if len(req.AllowedSpaceIDs) > 0 {
		vecResults = filterVectorHitsByAllowedSpaceIDs(vecResults, req.AllowedSpaceIDs)
	}

	// Fuse results
	fused := s.fuseResults(ftResults, vecResults, mode)

	// Apply minimum score filter
	if s.config.MinScore > 0 {
		var filtered []RankedResult
		for _, r := range fused {
			if r.Score >= s.config.MinScore {
				filtered = append(filtered, r)
			}
		}
		fused = filtered
	}

	totalCount := len(fused)

	// Paginate
	startIdx := (page - 1) * pageSize
	if startIdx >= len(fused) {
		return []HybridSearchResult{}, totalCount, mode, nil
	}
	endIdx := startIdx + pageSize
	if endIdx > len(fused) {
		endIdx = len(fused)
	}
	paged := fused[startIdx:endIdx]

	// Enrich results with metadata from database
	enriched := s.enrichResults(ctx, paged, ftResults, vecResults, mode)

	return enriched, totalCount, mode, nil
}

// fuseResults combines fulltext and vector results using the appropriate strategy.
func (s *HybridSearchService) fuseResults(ftResults []SearchHit, vecResults []VectorHit, mode string) []RankedResult {
	switch mode {
	case ModeFulltext:
		return searchHitsToRanked(ftResults)

	case ModeSemantic:
		return vectorHitsToRanked(vecResults)

	case ModeHybrid:
		ftRanked := searchHitsToRanked(ftResults)
		vecRanked := vectorHitsToRanked(vecResults)

		if len(ftRanked) == 0 {
			return vecRanked
		}
		if len(vecRanked) == 0 {
			return ftRanked
		}

		// Use RRF as the primary fusion strategy
		return ReciprocalRankFusion([][]RankedResult{ftRanked, vecRanked}, s.config.RRFRankK)

	default:
		return searchHitsToRanked(ftResults)
	}
}

// searchHitsToRanked converts fulltext SearchHit results to RankedResult.
func searchHitsToRanked(hits []SearchHit) []RankedResult {
	ranked := make([]RankedResult, len(hits))
	for i, h := range hits {
		ranked[i] = RankedResult{
			DocumentID: h.DocumentID,
			Score:      h.Score,
			Source:     "fulltext",
		}
	}
	return ranked
}

// vectorHitsToRanked converts vector VectorHit results to RankedResult.
func vectorHitsToRanked(hits []VectorHit) []RankedResult {
	ranked := make([]RankedResult, len(hits))
	for i, h := range hits {
		ranked[i] = RankedResult{
			DocumentID: h.DocumentID,
			Score:      h.Score,
			Source:     "vector",
		}
	}
	return ranked
}

// filterVectorHitsByAllowedSpaceIDs keeps only hits whose SpaceID is in allowedSet.
// If allowedSet is empty, returns hits unchanged (no filtering).
func filterVectorHitsByAllowedSpaceIDs(hits []VectorHit, allowedSet []uint) []VectorHit {
	if len(allowedSet) == 0 {
		return hits
	}
	allowed := make(map[uint]struct{}, len(allowedSet))
	for _, sid := range allowedSet {
		allowed[sid] = struct{}{}
	}
	filtered := make([]VectorHit, 0, len(hits))
	for _, h := range hits {
		if _, ok := allowed[h.SpaceID]; ok {
			filtered = append(filtered, h)
		}
	}
	return filtered
}

// enrichResults populates document metadata from the database for each result.
func (s *HybridSearchService) enrichResults(
	ctx context.Context,
	ranked []RankedResult,
	ftResults []SearchHit,
	vecResults []VectorHit,
	mode string,
) []HybridSearchResult {
	if len(ranked) == 0 {
		return []HybridSearchResult{}
	}

	// Build lookup maps for score details
	ftScoreMap := make(map[uint]float64, len(ftResults))
	ftSnippetMap := make(map[uint]string)
	ftHighlightMap := make(map[uint][]string)
	for _, h := range ftResults {
		ftScoreMap[h.DocumentID] = h.Score
		if h.Snippet != "" {
			ftSnippetMap[h.DocumentID] = h.Snippet
		}
		if len(h.Highlights) > 0 {
			ftHighlightMap[h.DocumentID] = h.Highlights
		}
	}

	vecScoreMap := make(map[uint]float64, len(vecResults))
	for _, h := range vecResults {
		vecScoreMap[h.DocumentID] = h.Score
	}

	// Batch fetch document metadata (skip if no repo)
	docMap := make(map[int]*workspacemodel.Document)
	if s.searchRepo != nil {
		docIDs := make([]int, len(ranked))
		for i, r := range ranked {
			docIDs[i] = int(r.DocumentID)
		}
		if docs, err := s.searchRepo.FindDocumentsByIDs(docIDs); err == nil {
			for _, doc := range docs {
				docMap[doc.ID] = doc
			}
		}
	}

	// Fetch space names
	spaceNames := make(map[int]string)

	results := make([]HybridSearchResult, 0, len(ranked))
	for _, r := range ranked {
		docID := int(r.DocumentID)
		doc := docMap[docID]

		hr := HybridSearchResult{
			DocumentID: r.DocumentID,
			Score:      r.Score,
			ScoreDetail: ScoreDetail{
				FullTextScore: ftScoreMap[r.DocumentID],
				VectorScore:   vecScoreMap[r.DocumentID],
				FusionMethod:  mode,
			},
		}

		if doc != nil {
			hr.Title = doc.Name
			hr.SpaceID = doc.SpaceID
			hr.Type = doc.Type
			hr.Path = doc.Path

			if doc.CreateTime > 0 {
				hr.UpdatedAt = strconv.FormatInt(int64(doc.CreateTime), 10)
			}
		}

		// Use snippet from fulltext search
		if snippet, ok := ftSnippetMap[r.DocumentID]; ok {
			hr.Snippet = snippet
		}
		if highlights, ok := ftHighlightMap[r.DocumentID]; ok {
			hr.Highlights = highlights
		}

		// Resolve space name (skip if no repo)
		if s.searchRepo != nil && hr.SpaceID > 0 {
			if name, ok := spaceNames[hr.SpaceID]; ok {
				hr.SpaceName = name
			} else if space, err := s.searchRepo.FindSpaceByID(hr.SpaceID); err == nil && space != nil {
				spaceNames[hr.SpaceID] = space.Name
				hr.SpaceName = space.Name
			}
		}

		// Resolve path to readable format (skip if no repo)
		if s.searchRepo != nil && hr.Path != "" && hr.Path != "0" {
			hr.Path = resolveDocPath(hr.Path, s.searchRepo)
		}

		results = append(results, hr)
	}

	// Sort by score descending
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	return results
}

// resolveDocPath converts "0,1,2" path format to "Parent/Child" readable format.
func resolveDocPath(rawPath string, repo *searchrepo.SearchRepository) string {
	if rawPath == "" || rawPath == "0" {
		return ""
	}
	parts := strings.Split(rawPath, ",")
	var pathNames []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "0" {
			continue
		}
		id := 0
		fmt.Sscanf(part, "%d", &id)
		if id > 0 {
			if doc, err := repo.FindDocumentByID(id); err == nil && doc != nil {
				pathNames = append(pathNames, doc.Name)
			}
		}
	}
	return strings.Join(pathNames, "/")
}

// filterDrafts removes draft documents from hybrid search results.
func filterDrafts(results []HybridSearchResult, docMap map[int]*workspacemodel.Document) []HybridSearchResult {
	filtered := make([]HybridSearchResult, 0, len(results))
	for _, r := range results {
		if doc, ok := docMap[int(r.DocumentID)]; ok {
			if doc.Status == workspacemodel.DocumentStatusDraft {
				continue
			}
		}
		filtered = append(filtered, r)
	}
	return filtered
}

// FilterBySpaceIDs filters hybrid results to only include documents from specified spaces.
func FilterBySpaceIDs(results []HybridSearchResult, spaceIDs []uint) []HybridSearchResult {
	if len(spaceIDs) == 0 {
		return results
	}
	allowed := make(map[int]bool, len(spaceIDs))
	for _, id := range spaceIDs {
		allowed[int(id)] = true
	}
	filtered := make([]HybridSearchResult, 0, len(results))
	for _, r := range results {
		if allowed[r.SpaceID] {
			filtered = append(filtered, r)
		}
	}
	return filtered
}
