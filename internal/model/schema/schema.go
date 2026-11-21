package schema

import "time"

// SchemaSpace Schema空间
type SchemaSpace struct {
	ID         int        `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SpaceID    int        `gorm:"column:space_id;not null;uniqueIndex:uk_space_id" json:"space_id"`
	Name       string     `gorm:"column:name;size:255;not null" json:"name"`
	DBType     string     `gorm:"column:db_type;size:50;default:mysql" json:"db_type"`
	Status     int        `gorm:"column:status;default:1" json:"status"`
	LastScanAt *time.Time `gorm:"column:last_scan_at" json:"last_scan_at"`
	DocCount   int        `gorm:"column:doc_count;default:0" json:"doc_count"`
	TableCount int        `gorm:"column:table_count;default:0" json:"table_count"`
	CreatedAt  time.Time  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at" json:"updated_at"`
}

func (SchemaSpace) TableName() string {
	return "aw_schema_space"
}

// SchemaTable 表元数据
type SchemaTable struct {
	ID            int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SchemaSpaceID int       `gorm:"column:schema_space_id;not null" json:"schema_space_id"`
	Name          string    `gorm:"column:table_name;size:128;not null" json:"table_name"`
	TableComment  string    `gorm:"column:table_comment;size:500" json:"table_comment"`
	Engine        string    `gorm:"column:engine;size:50" json:"engine"`
	Charset       string    `gorm:"column:charset;size:50" json:"charset"`
	PrimaryKey    string    `gorm:"column:primary_key;size:255" json:"primary_key"`
	DocID         int       `gorm:"column:doc_id" json:"doc_id"`
	DocPath       string    `gorm:"column:doc_path;size:500" json:"doc_path"`
	RawDDL        string    `gorm:"column:raw_ddl;type:text" json:"raw_ddl"`
	Status        int       `gorm:"column:status;default:1" json:"status"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
	UpdatedAt     time.Time `gorm:"column:updated_at" json:"updated_at"`
}

func (SchemaTable) TableName() string {
	return "aw_schema_table"
}

// SchemaColumn 字段元数据
type SchemaColumn struct {
	ID              int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SchemaSpaceID   int       `gorm:"column:schema_space_id;not null" json:"schema_space_id"`
	TableID         int       `gorm:"column:table_id;not null" json:"table_id"`
	Table           string    `gorm:"column:table_name;size:128;not null" json:"table_name"`
	ColumnName      string    `gorm:"column:column_name;size:128;not null" json:"column_name"`
	DataType        string    `gorm:"column:data_type;size:50;not null" json:"data_type"`
	Length          int       `gorm:"column:length" json:"length"`
	Nullable        int       `gorm:"column:nullable;default:1" json:"nullable"`
	DefaultValue    string    `gorm:"column:default_value;size:255" json:"default_value"`
	ColumnComment   string    `gorm:"column:column_comment;size:500" json:"column_comment"`
	IsPrimaryKey    int       `gorm:"column:is_primary_key;default:0" json:"is_primary_key"`
	IsForeignKey    int       `gorm:"column:is_foreign_key;default:0" json:"is_foreign_key"`
	IsIndex         int       `gorm:"column:is_index;default:0" json:"is_index"`
	OrdinalPosition int       `gorm:"column:ordinal_position" json:"ordinal_position"`
	CreatedAt       time.Time `gorm:"column:created_at" json:"created_at"`
}

func (SchemaColumn) TableName() string {
	return "aw_schema_column"
}

// SchemaRelation 表关系
type SchemaRelation struct {
	ID            int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SchemaSpaceID int       `gorm:"column:schema_space_id;not null" json:"schema_space_id"`
	SourceTable   string    `gorm:"column:source_table;size:128;not null" json:"source_table"`
	SourceColumn  string    `gorm:"column:source_column;size:128;not null" json:"source_column"`
	TargetTable   string    `gorm:"column:target_table;size:128;not null" json:"target_table"`
	TargetColumn  string    `gorm:"column:target_column;size:128;not null" json:"target_column"`
	RelationType  string    `gorm:"column:relation_type;size:50;default:one_to_many" json:"relation_type"`
	Confidence    float32   `gorm:"column:confidence;default:1.0" json:"confidence"`
	Source        string    `gorm:"column:source;size:20;default:ddl" json:"source"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
}

func (SchemaRelation) TableName() string {
	return "aw_schema_relation"
}

// SchemaBusinessTag 业务语义标签
type SchemaBusinessTag struct {
	ID            int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SchemaSpaceID int       `gorm:"column:schema_space_id;not null" json:"schema_space_id"`
	TableID       int       `gorm:"column:table_id" json:"table_id"`
	Table         string    `gorm:"column:table_name;size:128" json:"table_name"`
	TagName       string    `gorm:"column:tag_name;size:100;not null" json:"tag_name"`
	TagType       string    `gorm:"column:tag_type;size:50;default:auto" json:"tag_type"`
	Confidence    float32   `gorm:"column:confidence;default:1.0" json:"confidence"`
	CreatedAt     time.Time `gorm:"column:created_at" json:"created_at"`
}

func (SchemaBusinessTag) TableName() string {
	return "aw_schema_business_tag"
}

// SchemaScanTask 扫描任务
// 状态流转: pending(等待) -> running(运行中) -> paused(已暂停)/completed(完成)/failed(失败)/cancelled(已取消)
type SchemaScanTask struct {
	ID            int        `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	SchemaSpaceID int        `gorm:"column:schema_space_id;not null" json:"schema_space_id"`
	Status        string     `gorm:"column:status;size:20;default:pending" json:"status"`
	TotalDocs     int        `gorm:"column:total_docs;default:0" json:"total_docs"`
	ProcessedDocs int        `gorm:"column:processed_docs;default:0" json:"processed_docs"`
	FoundTables   int        `gorm:"column:found_tables;default:0" json:"found_tables"`
	ErrorMessage  string     `gorm:"column:error_message;type:text" json:"error_message"`
	StartedAt     *time.Time `gorm:"column:started_at" json:"started_at"`
	CompletedAt   *time.Time `gorm:"column:completed_at" json:"completed_at"`
	CreatedAt     time.Time  `gorm:"column:created_at" json:"created_at"`
}

func (SchemaScanTask) TableName() string {
	return "aw_schema_scan_task"
}
