package ai

import (
	"time"

	"gorm.io/gorm"
)

type DataSource struct {
	ID         uint64         `gorm:"primaryKey" json:"id"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index:idx_deleted_at" json:"-"`
	Name       string         `gorm:"size:200;not null" json:"name"`
	SpaceID    uint64         `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	DBType     string         `gorm:"size:50;not null" json:"db_type"`
	Host       string         `gorm:"size:200;not null" json:"host"`
	Port       int            `gorm:"default:3306" json:"port"`
	DBName     string         `gorm:"size:200;not null" json:"db_name"`
	Username   string         `gorm:"size:200;not null" json:"username"`
	Password   string         `gorm:"size:500;not null" json:"-"`
	SyncCron   string         `gorm:"size:100" json:"sync_cron"`
	LastSyncAt *time.Time     `json:"last_sync_at"`
	IsActive   bool           `gorm:"default:true" json:"is_active"`
}

func (DataSource) TableName() string {
	return "aw_ai_datasource"
}

type DataSourceList []DataSource

func (ds DataSourceList) Value() (interface{}, error) {
	return ds, nil
}

type SQLHistory struct {
	ID              uint64         `gorm:"primaryKey" json:"id"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index:idx_deleted_at" json:"-"`
	SpaceID         uint64         `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	UserID          uint64         `gorm:"column:user_id;not null;index:idx_user_id" json:"user_id"`
	QueryText       string         `gorm:"type:text;not null" json:"query_text"`
	GeneratedSQL    string         `gorm:"type:text" json:"generated_sql"`
	ExecutedSQL     string         `gorm:"type:text" json:"executed_sql"`
	ExecutionResult string         `gorm:"type:longtext" json:"execution_result"`
	ExecutionError  string         `gorm:"type:text" json:"execution_error"`
	IsFavorite      bool           `gorm:"default:false" json:"is_favorite"`
	Rating          int8           `gorm:"default:0" json:"rating"`
	Feedback        string         `gorm:"type:text" json:"feedback"`
	TablesUsed      string         `gorm:"type:text" json:"tables_used"`
	ExecutionTimeMs int            `gorm:"default:0" json:"execution_time_ms"`
	DatasourceID    uint64         `gorm:"column:datasource_id" json:"datasource_id"`
	DurationMs      int            `gorm:"default:0" json:"duration_ms"`
	Status          string         `gorm:"size:20;default:'success'" json:"status"`
	ExplanationJSON string         `gorm:"type:longtext" json:"explanation_json"`
	TraceID         string         `gorm:"size:64;index:idx_trace_id" json:"trace_id"`
	GeneratorTier   string         `gorm:"size:20" json:"generator_tier"`
}

func (SQLHistory) TableName() string {
	return "aw_ai_sql_history"
}
