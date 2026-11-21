package ai

import (
	"fmt"
	"regexp"

	aimodel "github.com/listen808/citebase/internal/model/ai"
	airepo "github.com/listen808/citebase/internal/repository/ai"
)

// DataSensitivityService 数据脱敏业务逻辑层
type DataSensitivityService struct {
	repo *airepo.DataSensitivityRepo
}

// NewDataSensitivityService 创建 DataSensitivityService 实例
func NewDataSensitivityService(repo *airepo.DataSensitivityRepo) *DataSensitivityService {
	return &DataSensitivityService{repo: repo}
}

// CreateSensitivityRuleRequest 创建规则请求
type CreateSensitivityRuleRequest struct {
	SpaceID     uint64                       `json:"space_id" binding:"required"`
	TableName   string                       `json:"table_name" binding:"required"`
	FieldName   string                       `json:"field_name" binding:"required"`
	Level       aimodel.DataSensitivityLevel `json:"level" binding:"required"`
	MaskType    string                       `json:"mask_type"`
	MaskRule    string                       `json:"mask_rule"`
	Description string                       `json:"description"`
}

// CreateRule 创建规则
func (s *DataSensitivityService) CreateRule(req *CreateSensitivityRuleRequest) (*aimodel.DataSensitivityRule, error) {
	rule := &aimodel.DataSensitivityRule{
		SpaceID:     req.SpaceID,
		Table:       req.TableName,
		Field:       req.FieldName,
		Level:       req.Level,
		MaskType:    req.MaskType,
		MaskRule:    req.MaskRule,
		Description: req.Description,
	}
	if err := s.repo.Create(rule); err != nil {
		return nil, fmt.Errorf("创建规则失败: %w", err)
	}
	return rule, nil
}

// UpdateRule 更新规则
func (s *DataSensitivityService) UpdateRule(id uint64, req *CreateSensitivityRuleRequest) (*aimodel.DataSensitivityRule, error) {
	rule, err := s.repo.FindByID(id)
	if err != nil {
		return nil, fmt.Errorf("规则不存在: %w", err)
	}
	rule.Table = req.TableName
	rule.Field = req.FieldName
	rule.Level = req.Level
	rule.MaskType = req.MaskType
	rule.MaskRule = req.MaskRule
	rule.Description = req.Description
	if err := s.repo.Update(rule); err != nil {
		return nil, fmt.Errorf("更新规则失败: %w", err)
	}
	return rule, nil
}

// DeleteRule 删除规则
func (s *DataSensitivityService) DeleteRule(id uint64) error {
	return s.repo.Delete(id)
}

// ListRules 列出规则
func (s *DataSensitivityService) ListRules(spaceID uint64) ([]*aimodel.DataSensitivityRule, error) {
	return s.repo.ListBySpace(spaceID)
}

// DetectPIIResult PII 检测结果
type DetectPIIResult struct {
	PIIFound []string `json:"pii_found"`
	Count    int      `json:"count"`
}

// DetectPII 检测 PII
func (s *DataSensitivityService) DetectPII(text string) *DetectPIIResult {
	found := simpleDetectPII(text)
	return &DetectPIIResult{PIIFound: found, Count: len(found)}
}

// simpleDetectPII 简单 PII 检测：手机号、身份证、邮箱、银行卡
func simpleDetectPII(text string) []string {
	patterns := []struct {
		name    string
		pattern string
	}{
		{"mobile", `1[3-9]\d{9}`},
		{"id_card", `\d{17}[\dXx]|\d{15}`},
		{"email", `[\w.+-]+@[\w-]+\.[\w.-]+`},
		{"bank_card", `\d{16,19}`},
	}
	found := []string{}
	for _, p := range patterns {
		re := regexp.MustCompile(p.pattern)
		matches := re.FindAllString(text, -1)
		for _, m := range matches {
			if p.name == "id_card" && !(len(m) == 15 || len(m) == 18) {
				continue
			}
			if p.name == "bank_card" && (len(m) < 16 || len(m) > 19) {
				continue
			}
			found = append(found, fmt.Sprintf("%s:%s", p.name, maskPIIPartial(m)))
		}
	}
	return found
}

// maskPIIPartial 保留首尾，中间用 * 替代
func maskPIIPartial(s string) string {
	runes := []rune(s)
	n := len(runes)
	if n <= 2 {
		return "*"
	}
	show := n / 4
	if show < 1 {
		show = 1
	}
	if show*2 >= n {
		return string(runes[:1]) + "***" + string(runes[n-1:])
	}
	return string(runes[:show]) + "****" + string(runes[n-show:])
}
