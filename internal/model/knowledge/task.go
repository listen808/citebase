package knowledge

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type AITask struct {
	ID             int        `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	TaskID         string     `gorm:"column:task_id;uniqueIndex:uk_task_id;size:64;not null" json:"task_id"`
	Type           string     `gorm:"column:type;size:32;not null" json:"type"`
	Status         string     `gorm:"column:status;size:32;not null;index:idx_status" json:"status"`
	Progress       float64    `gorm:"column:progress;type:decimal(5,4);default:0" json:"progress"`
	TargetSpaceIDs IntArray   `gorm:"column:target_space_ids;type:text" json:"target_space_ids"`
	TargetDocID    int        `gorm:"column:target_doc_id;default:0" json:"target_doc_id"`
	TotalDocs      int        `gorm:"column:total_docs;default:0" json:"total_docs"`
	ProcessedDocs  int        `gorm:"column:processed_docs;default:0" json:"processed_docs"`
	Result         string     `gorm:"column:result;type:longtext" json:"result"`
	ErrorMessage   string     `gorm:"column:error_message;type:text" json:"error_message"`
	TokenUsage     int        `gorm:"column:token_usage;default:0" json:"token_usage"`
	Checkpoint     string     `gorm:"column:checkpoint;type:text" json:"checkpoint"` // S8: 断点JSON
	StartedAt      *time.Time `gorm:"column:started_at" json:"started_at"`
	CompletedAt    *time.Time `gorm:"column:completed_at" json:"completed_at"`
	CreatedAt      time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (AITask) TableName() string {
	return "aw_ai_task"
}

type IntArray []int

func (a IntArray) Value() (driver.Value, error) {
	if a == nil {
		return nil, nil
	}
	return json.Marshal(a)
}

func (a *IntArray) Scan(value interface{}) error {
	if value == nil {
		*a = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, a)
}

type AIDocMapping struct {
	ID            int        `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AIDocID       int        `gorm:"column:ai_doc_id;uniqueIndex:uk_ai_doc_id;not null" json:"ai_doc_id"`
	SourceDocID   int        `gorm:"column:source_doc_id;not null;index:idx_source_doc_id" json:"source_doc_id"`
	SourceSpaceID int        `gorm:"column:source_space_id;not null" json:"source_space_id"`
	MappingType   string     `gorm:"column:mapping_type;size:32;not null;index:idx_mapping_type" json:"mapping_type"`
	ReviewStatus  string     `gorm:"column:review_status;size:32;default:'pending'" json:"review_status"`
	LastSyncedAt  *time.Time `gorm:"column:last_synced_at" json:"last_synced_at"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (AIDocMapping) TableName() string {
	return "aw_ai_doc_mapping"
}

const (
	ReviewStatusPending    = "pending"
	ReviewStatusAccepted   = "accepted"
	ReviewStatusRejected   = "rejected"
	ReviewStatusManualEdit = "manual_edit"
)
