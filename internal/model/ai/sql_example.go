package ai

type SQLExample struct {
	ID        int    `json:"id" gorm:"primaryKey;autoIncrement"`
	SpaceID   int    `json:"space_id" gorm:"column:space_id;index:idx_space_verified"`
	Question  string `json:"question" gorm:"type:varchar(500);not null"`
	SQL       string `json:"sql" gorm:"type:text;not null;column:sql"`
	DBType    string `json:"db_type" gorm:"type:varchar(20);default:mysql"`
	Verified  int    `json:"verified" gorm:"default:0;index:idx_space_verified"`
	CreatedBy int    `json:"created_by" gorm:"default:0"`
	CreatedAt string `json:"created_at" gorm:"autoCreateTime"`
}

func (SQLExample) TableName() string { return "aw_ai_sql_example" }
