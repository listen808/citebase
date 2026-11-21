package eval

import (
	"encoding/json"
	"fmt"

	evalmodel "github.com/listen808/citebase/internal/model/eval"
	repos "github.com/listen808/citebase/internal/repository"
	"gorm.io/gorm"
)

// DatasetRepository 评估数据集与结果的仓库
type DatasetRepository struct {
	db *gorm.DB
}

// NewDatasetRepository 创建仓库实例
func NewDatasetRepository() *DatasetRepository {
	return &DatasetRepository{db: repos.DB}
}

// CreateDataset 创建评估数据集
func (r *DatasetRepository) CreateDataset(name string, spaceID int, questions []EvalQuestion) (*DatasetWithQuestions, error) {
	questionsJSON, err := json.Marshal(questions)
	if err != nil {
		return nil, fmt.Errorf("marshal questions: %w", err)
	}

	ds := &evalmodel.EvalDataset{
		Name:      name,
		SpaceID:   spaceID,
		Questions: string(questionsJSON),
	}
	if err := r.db.Create(ds).Error; err != nil {
		return nil, err
	}

	return &DatasetWithQuestions{
		ID:        ds.ID,
		Name:      ds.Name,
		SpaceID:   ds.SpaceID,
		Questions: questions,
		CreatedAt: ds.CreatedAt,
		UpdatedAt: ds.UpdatedAt,
	}, nil
}

// GetDataset 获取数据集详情
func (r *DatasetRepository) GetDataset(id int64) (*DatasetWithQuestions, error) {
	var ds evalmodel.EvalDataset
	if err := r.db.First(&ds, id).Error; err != nil {
		return nil, err
	}

	var questions []EvalQuestion
	if ds.Questions != "" {
		_ = json.Unmarshal([]byte(ds.Questions), &questions)
	}

	return &DatasetWithQuestions{
		ID:        ds.ID,
		Name:      ds.Name,
		SpaceID:   ds.SpaceID,
		Questions: questions,
		CreatedAt: ds.CreatedAt,
		UpdatedAt: ds.UpdatedAt,
	}, nil
}

// ListDatasets 列出所有数据集
func (r *DatasetRepository) ListDatasets(limit, offset int) ([]*DatasetWithQuestions, int64, error) {
	var datasets []evalmodel.EvalDataset
	var total int64

	if err := r.db.Model(&evalmodel.EvalDataset{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := r.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&datasets).Error; err != nil {
		return nil, 0, err
	}

	result := make([]*DatasetWithQuestions, 0, len(datasets))
	for _, ds := range datasets {
		var questions []EvalQuestion
		if ds.Questions != "" {
			_ = json.Unmarshal([]byte(ds.Questions), &questions)
		}
		result = append(result, &DatasetWithQuestions{
			ID:        ds.ID,
			Name:      ds.Name,
			SpaceID:   ds.SpaceID,
			Questions: questions,
			CreatedAt: ds.CreatedAt,
			UpdatedAt: ds.UpdatedAt,
		})
	}

	return result, total, nil
}

// DeleteDataset 删除数据集
func (r *DatasetRepository) DeleteDataset(id int64) error {
	return r.db.Delete(&evalmodel.EvalDataset{}, id).Error
}

// SaveResult 保存评估结果
func (r *DatasetRepository) SaveResult(result *evalmodel.EvalResult) error {
	return r.db.Create(result).Error
}

// GetResult 获取评估结果详情
func (r *DatasetRepository) GetResult(id int64) (*ResultWithDetails, error) {
	var dbResult evalmodel.EvalResult
	if err := r.db.First(&dbResult, id).Error; err != nil {
		return nil, err
	}
	return r.fromDBModel(&dbResult), nil
}

// ListResults 列出评估结果
func (r *DatasetRepository) ListResults(limit, offset int) ([]*ResultWithDetails, int64, error) {
	var dbResults []evalmodel.EvalResult
	var total int64

	if err := r.db.Model(&evalmodel.EvalResult{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	if err := r.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&dbResults).Error; err != nil {
		return nil, 0, err
	}

	results := make([]*ResultWithDetails, 0, len(dbResults))
	for i := range dbResults {
		results = append(results, r.fromDBModel(&dbResults[i]))
	}

	return results, total, nil
}

// DeleteResult 删除评估结果
func (r *DatasetRepository) DeleteResult(id int64) error {
	return r.db.Delete(&evalmodel.EvalResult{}, id).Error
}

// fromDBModel 从数据库模型转换为 API 传输对象
func (r *DatasetRepository) fromDBModel(dbResult *evalmodel.EvalResult) *ResultWithDetails {
	result := &ResultWithDetails{
		ID:           dbResult.ID,
		DatasetID:    dbResult.DatasetID,
		DatasetName:  dbResult.DatasetName,
		SpaceID:      dbResult.SpaceID,
		OverallScore: dbResult.OverallScore,
		Duration:     dbResult.Duration,
		Status:       dbResult.Status,
		CreatedAt:    dbResult.CreatedAt,
	}
	if dbResult.MetricsJSON != "" {
		var metrics RAGASMetrics
		if err := json.Unmarshal([]byte(dbResult.MetricsJSON), &metrics); err == nil {
			result.Metrics = &metrics
		}
	}
	if dbResult.DetailsJSON != "" {
		var details []QuestionEval
		if err := json.Unmarshal([]byte(dbResult.DetailsJSON), &details); err == nil {
			result.Details = details
		}
	}
	return result
}
