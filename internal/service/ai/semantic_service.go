package ai

import (
	"fmt"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

// SemanticService 语义层业务逻辑层
// 从 internal/api/ai/semantic_layer.go 中提取的业务逻辑
type SemanticService struct {
	metricRepo    *airepo.SemanticMetricRepo
	dimensionRepo *airepo.SemanticDimensionRepo
}

// NewSemanticService 创建 SemanticService 实例
func NewSemanticService(metricRepo *airepo.SemanticMetricRepo, dimensionRepo *airepo.SemanticDimensionRepo) *SemanticService {
	return &SemanticService{metricRepo: metricRepo, dimensionRepo: dimensionRepo}
}

// ListMetrics 列出语义指标
func (s *SemanticService) ListMetrics(spaceID uint64, page, pageSize int) ([]aimodel.SemanticMetric, int64, error) {
	return s.metricRepo.ListBySpace(spaceID, page, pageSize)
}

// CreateMetric 创建语义指标
func (s *SemanticService) CreateMetric(m *aimodel.SemanticMetric) error {
	if err := s.metricRepo.Create(m); err != nil {
		return fmt.Errorf("create failed: %w", err)
	}
	return nil
}

// UpdateMetric 更新语义指标
func (s *SemanticService) UpdateMetric(id uint64, m *aimodel.SemanticMetric) error {
	m.ID = id
	if err := s.metricRepo.Update(m); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}
	return nil
}

// DeleteMetric 删除语义指标
func (s *SemanticService) DeleteMetric(id uint64) error {
	if err := s.metricRepo.Delete(id); err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}
	return nil
}

// ListDimensions 列出语义维度
func (s *SemanticService) ListDimensions(spaceID uint64, page, pageSize int) ([]aimodel.SemanticDimension, int64, error) {
	return s.dimensionRepo.ListBySpace(spaceID, page, pageSize)
}

// CreateDimension 创建语义维度
func (s *SemanticService) CreateDimension(d *aimodel.SemanticDimension) error {
	if err := s.dimensionRepo.Create(d); err != nil {
		return fmt.Errorf("create failed: %w", err)
	}
	return nil
}

// DeleteDimension 删除语义维度
func (s *SemanticService) DeleteDimension(id uint64) error {
	if err := s.dimensionRepo.Delete(id); err != nil {
		return fmt.Errorf("delete failed: %w", err)
	}
	return nil
}
