package knowledge

import (
	"fmt"
	"time"

	logx "github.com/listen808/citebase/internal/log"
	knowledgemodel "github.com/listen808/citebase/internal/model/knowledge"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

type AIAnalysisResultRepository struct {
	db *gorm.DB
}

func NewAIAnalysisResultRepository() *AIAnalysisResultRepository {
	return &AIAnalysisResultRepository{db: repos.DB}
}

func NewAIAnalysisResultRepositoryWithDB(db *gorm.DB) *AIAnalysisResultRepository {
	return &AIAnalysisResultRepository{db: db}
}

func (r *AIAnalysisResultRepository) FindBySpaceID(spaceID int, page, pageSize int) ([]*knowledgemodel.AIAnalysisResult, int64, error) {
	var results []*knowledgemodel.AIAnalysisResult
	var total int64

	logx.Infof("[AnalysisResultRepo] FindBySpaceID: spaceID=%d, page=%d, pageSize=%d, db=%v", spaceID, page, pageSize, r.db)

	if r.db == nil {
		logx.Error("[AnalysisResultRepo] DB is nil!")
		return nil, 0, fmt.Errorf("database connection is nil")
	}

	query := r.db.Model(&knowledgemodel.AIAnalysisResult{}).Where("space_id = ?", spaceID)

	if err := query.Count(&total).Error; err != nil {
		logx.Errorf("[AnalysisResultRepo] Count failed: spaceID=%d, error=%v", spaceID, err)
		return nil, 0, err
	}

	logx.Infof("[AnalysisResultRepo] Count: spaceID=%d, total=%d", spaceID, total)

	if err := query.Offset((page - 1) * pageSize).Limit(pageSize).
		Order("updated_at DESC").
		Find(&results).Error; err != nil {
		logx.Errorf("[AnalysisResultRepo] Find failed: spaceID=%d, error=%v", spaceID, err)
		return nil, 0, err
	}

	logx.Infof("[AnalysisResultRepo] FindBySpaceID success: spaceID=%d, total=%d, returned=%d", spaceID, total, len(results))

	return results, total, nil
}

func (r *AIAnalysisResultRepository) FindByDocID(docID int) (*knowledgemodel.AIAnalysisResult, error) {
	var result knowledgemodel.AIAnalysisResult
	if err := r.db.Where("doc_id = ?", docID).First(&result).Error; err != nil {
		return nil, err
	}
	return &result, nil
}

// QualityScoresByDocIDs P1-2：批量取文档质量分（0-100），一次 IN 查询。
// 检索侧原先完全不知道分析结果——质量分只躺在 ai_analysis_result 里，
// 从未参与排序，analysis 与 retrieval 是两条平行链路。
func (r *AIAnalysisResultRepository) QualityScoresByDocIDs(docIDs []int) (map[int]float64, error) {
	out := make(map[int]float64, len(docIDs))
	if len(docIDs) == 0 {
		return out, nil
	}
	var rows []struct {
		DocID     int
		Score     float64
		UpdatedAt time.Time
	}
	if err := r.db.Model(&knowledgemodel.AIAnalysisResult{}).
		Select("doc_id, quality_score AS score, updated_at").
		Where("doc_id IN ?", docIDs).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	// 同一文档可能有多条分析记录（多次分析），取最近一条
	latest := make(map[int]time.Time, len(rows))
	for _, row := range rows {
		if prev, ok := latest[row.DocID]; !ok || row.UpdatedAt.After(prev) {
			latest[row.DocID] = row.UpdatedAt
			out[row.DocID] = row.Score
		}
	}
	return out, nil
}

func (r *AIAnalysisResultRepository) FindBySpaceIDAndCategory(spaceID int, category string) ([]*knowledgemodel.AIAnalysisResult, error) {
	var results []*knowledgemodel.AIAnalysisResult
	if err := r.db.Where("space_id = ? AND category = ?", spaceID, category).
		Order("updated_at DESC").
		Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

func (r *AIAnalysisResultRepository) GetCategoriesBySpaceID(spaceID int) ([]string, error) {
	var categories []string
	if err := r.db.Model(&knowledgemodel.AIAnalysisResult{}).
		Where("space_id = ?", spaceID).
		Distinct("category").
		Pluck("category", &categories).Error; err != nil {
		return nil, err
	}
	return categories, nil
}

func (r *AIAnalysisResultRepository) GetQualityStats(spaceID int) (map[string]interface{}, error) {
	var avgScore float64
	var count int64

	if err := r.db.Model(&knowledgemodel.AIAnalysisResult{}).
		Where("space_id = ?", spaceID).
		Select("AVG(quality_score)").
		Scan(&avgScore).Error; err != nil {
		return nil, err
	}

	if err := r.db.Model(&knowledgemodel.AIAnalysisResult{}).
		Where("space_id = ?", spaceID).
		Count(&count).Error; err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"avg_score": avgScore,
		"total":     count,
	}, nil
}

func (r *AIAnalysisResultRepository) Save(result *knowledgemodel.AIAnalysisResult) error {
	if result.ID > 0 {
		return r.db.Save(result).Error
	}
	return r.db.Create(result).Error
}

func (r *AIAnalysisResultRepository) BatchSave(results []*knowledgemodel.AIAnalysisResult) error {
	return r.db.CreateInBatches(results, 100).Error
}

func (r *AIAnalysisResultRepository) DeleteBySpaceID(spaceID int) error {
	return r.db.Where("space_id = ?", spaceID).Delete(&knowledgemodel.AIAnalysisResult{}).Error
}

func (r *AIAnalysisResultRepository) FindLowQuality(spaceID int, threshold float64) ([]*knowledgemodel.AIAnalysisResult, error) {
	var results []*knowledgemodel.AIAnalysisResult
	if err := r.db.Where("space_id = ? AND quality_score < ?", spaceID, threshold).
		Order("quality_score ASC").
		Find(&results).Error; err != nil {
		return nil, err
	}
	return results, nil
}

// CountByCategory 一次 GROUP BY 统计空间内各分类的分析结果数量。
// 旧调用方式为「取去重分类 → 每分类全量加载整行（含正文/要点大字段）只为 len()」，
// 一次请求放大为分类数 × 全表行 的读取。
func (r *AIAnalysisResultRepository) CountByCategory(spaceID int) (map[string]int64, error) {
	type row struct {
		Category string
		Cnt      int64
	}
	var rows []row
	err := r.db.Model(&knowledgemodel.AIAnalysisResult{}).
		Select("category, COUNT(*) as cnt").
		Where("space_id = ?", spaceID).
		Group("category").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make(map[string]int64, len(rows))
	for _, x := range rows {
		out[x.Category] = x.Cnt
	}
	return out, nil
}

// CountLowQuality 只统计低质量数量（COUNT 下推），替代「全量取行再 len()」。
func (r *AIAnalysisResultRepository) CountLowQuality(spaceID int, threshold float64) (int64, error) {
	var count int64
	err := r.db.Model(&knowledgemodel.AIAnalysisResult{}).
		Where("space_id = ? AND quality_score < ?", spaceID, threshold).
		Count(&count).Error
	return count, err
}
