package knowledge

import (
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	commonmodel "github.com/listen808/citebase/internal/model/common"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	knowledgerepo "github.com/listen808/citebase/internal/repository/knowledge"
)

type AIAnalysisService struct {
	resultRepo *knowledgerepo.AIAnalysisResultRepository
}

func NewAIAnalysisService() *AIAnalysisService {
	return &AIAnalysisService{
		resultRepo: knowledgerepo.NewAIAnalysisResultRepository(),
	}
}

type CategoryNode struct {
	Name     string          `json:"name"`
	Count    int             `json:"count"`
	Children []*CategoryNode `json:"children,omitempty"`
}

type QualityStats struct {
	AvgScore    float64 `json:"avg_score"`
	Total       int64   `json:"total"`
	LowQuality  int64   `json:"low_quality"`
	HighQuality int64   `json:"high_quality"`
}

func (s *AIAnalysisService) GetResultsBySpaceID(spaceID int, pq commonmodel.PageQuery) (*commonmodel.PageResult[*knowledgemodel.AIAnalysisResult], error) {
	nq := pq
	_ = nq.Offset()
	limit := nq.Limit()
	pageNum := nq.PageNum

	logx.Infof("[AnalysisService] GetResultsBySpaceID: spaceID=%d, page=%d, limit=%d", spaceID, pageNum, limit)

	results, total, err := s.resultRepo.FindBySpaceID(spaceID, pageNum, limit)
	if err != nil {
		logx.Errorf("[AnalysisService] FindBySpaceID failed: spaceID=%d, page=%d, limit=%d, error=%v", spaceID, pageNum, limit, err)
		return nil, fmt.Errorf("query analysis results: %w", err)
	}

	logx.Infof("[AnalysisService] GetResultsBySpaceID success: spaceID=%d, total=%d, returned=%d", spaceID, total, len(results))

	return &commonmodel.PageResult[*knowledgemodel.AIAnalysisResult]{
		List:     results,
		Total:    total,
		PageNum:  pageNum,
		PageSize: limit,
	}, nil
}

func (s *AIAnalysisService) GetResultByDocID(docID int) (*knowledgemodel.AIAnalysisResult, error) {
	return s.resultRepo.FindByDocID(docID)
}

func (s *AIAnalysisService) GetCategoryTree(spaceID int) ([]*CategoryNode, error) {
	categories, err := s.resultRepo.GetCategoriesBySpaceID(spaceID)
	if err != nil {
		return nil, fmt.Errorf("get categories: %w", err)
	}

	// 一次 GROUP BY 拿到各分类计数（旧实现对每个分类全量加载整行只为 len(results)）
	counts, err := s.resultRepo.CountByCategory(spaceID)
	if err != nil {
		return nil, fmt.Errorf("count categories: %w", err)
	}

	var nodes []*CategoryNode
	for _, cat := range categories {
		nodes = append(nodes, &CategoryNode{
			Name:  cat,
			Count: int(counts[cat]),
		})
	}

	return nodes, nil
}

func (s *AIAnalysisService) GetQualityStats(spaceID int) (*QualityStats, error) {
	stats, err := s.resultRepo.GetQualityStats(spaceID)
	if err != nil {
		return nil, fmt.Errorf("get quality stats: %w", err)
	}

	avgScore, _ := stats["avg_score"].(float64)
	total, _ := stats["total"].(int64)

	// COUNT 下推，不再把低质量结果整行读进内存只为计数
	lowQuality, _ := s.resultRepo.CountLowQuality(spaceID, 60)

	return &QualityStats{
		AvgScore:    avgScore,
		Total:       total,
		LowQuality:  lowQuality,
		HighQuality: total - lowQuality,
	}, nil
}

func (s *AIAnalysisService) GetKnowledgeGaps(spaceID int) ([]map[string]interface{}, error) {
	categories, err := s.resultRepo.GetCategoriesBySpaceID(spaceID)
	if err != nil {
		return nil, err
	}

	counts, err := s.resultRepo.CountByCategory(spaceID)
	if err != nil {
		return nil, err
	}

	var gaps []map[string]interface{}
	for _, cat := range categories {
		cnt := int(counts[cat])
		if cnt < 2 {
			gaps = append(gaps, map[string]interface{}{
				"category":        cat,
				"description":     fmt.Sprintf("分类「%s」下仅有 %d 篇文档，内容较少", cat, cnt),
				"suggested_title": fmt.Sprintf("补充%s相关文档", cat),
				"priority":        "medium",
			})
		}
	}

	return gaps, nil
}

func (s *AIAnalysisService) SaveAnalysisResult(result *knowledgemodel.AIAnalysisResult) error {
	return s.resultRepo.Save(result)
}

func (s *AIAnalysisService) BatchSaveResults(results []*knowledgemodel.AIAnalysisResult) error {
	return s.resultRepo.BatchSave(results)
}

func parseKeyPoints(keyPointsJSON string) []string {
	var points []string
	if err := json.Unmarshal([]byte(keyPointsJSON), &points); err != nil {
		return nil
	}
	return points
}

func parseTags(tagsJSON string) []string {
	var tags []string
	if err := json.Unmarshal([]byte(tagsJSON), &tags); err != nil {
		return nil
	}
	return tags
}
