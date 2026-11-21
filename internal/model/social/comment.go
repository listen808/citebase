package social

type Comment struct {
	ID             int        `gorm:"column:id;primaryKey" json:"id"`
	DocumentID     int        `gorm:"column:document_id;index:idx_document_id" json:"document_id"`
	ParentID       int        `gorm:"column:parent_id;default:0;index:idx_parent_id" json:"parent_id"`
	Resolved       int        `gorm:"column:resolved;default:0" json:"resolved"`
	Content        string     `gorm:"column:content;type:text" json:"content"`
	SelectionText  string     `gorm:"column:selection_text;type:text" json:"selection_text"`
	SelectionRange string     `gorm:"column:selection_range;type:varchar(100)" json:"selection_range"`
	CreatorID      int        `gorm:"column:creator_id;index:idx_creator_id" json:"creator_id"`
	CreatorName    string     `gorm:"-" json:"creator_name"`
	CreatedAt      int        `gorm:"column:create_time" json:"created_at"`
	UpdatedAt      int        `gorm:"column:update_time" json:"updated_at"`
	Replies        []*Comment `gorm:"-" json:"replies,omitempty"`
}

func (Comment) TableName() string {
	return "aw_comment"
}
