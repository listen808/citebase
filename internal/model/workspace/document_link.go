package workspace

type DocumentLink struct {
	ID          int    `gorm:"column:id;primaryKey" json:"id"`
	SourceDocID int    `gorm:"column:source_doc_id;index:idx_source" json:"source_doc_id"`
	TargetDocID int    `gorm:"column:target_doc_id;index:idx_target" json:"target_doc_id"`
	LinkText    string `gorm:"column:link_text;type:varchar(500)" json:"link_text"`
	SpaceID     int    `gorm:"column:space_id" json:"space_id"`
	CreatedAt   int    `gorm:"column:create_time" json:"created_at"`
}

func (DocumentLink) TableName() string {
	return "aw_document_link"
}
