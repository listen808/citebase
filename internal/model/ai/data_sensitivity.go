package ai

import "time"

// DataSensitivityLevel 数据敏感度级别
type DataSensitivityLevel string

const (
	SensitivityLow      DataSensitivityLevel = "low"
	SensitivityMedium   DataSensitivityLevel = "medium"
	SensitivityHigh     DataSensitivityLevel = "high"
	SensitivityCritical DataSensitivityLevel = "critical"
)

// DataSensitivityRule 字段级敏感度规则
type DataSensitivityRule struct {
	ID          uint64               `json:"id" gorm:"primaryKey;autoIncrement"`
	SpaceID     uint64               `json:"space_id" gorm:"index:idx_space_table_field"`
	Table       string               `json:"table_name" gorm:"size:128;column:table_name;index:idx_space_table_field"`
	Field       string               `json:"field_name" gorm:"size:128;column:field_name;index:idx_space_table_field"`
	Level       DataSensitivityLevel `json:"level" gorm:"size:32"`
	MaskType    string               `json:"mask_type" gorm:"size:32"`   // full, partial, hash, regex
	MaskRule    string               `json:"mask_rule" gorm:"type:text"` // 脱敏规则 JSON 或正则
	Description string               `json:"description" gorm:"type:text"`
	CreatedAt   time.Time            `json:"created_at"`
	UpdatedAt   time.Time            `json:"updated_at"`
}

func (DataSensitivityRule) TableName() string {
	return "aw_ai_data_sensitivity_rules"
}
