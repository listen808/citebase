package knowledge

import "time"

type AIFeedback struct {
	ID             int       `gorm:"primaryKey;autoIncrement" json:"id"`
	UserID         int       `gorm:"column:user_id;not null;index:idx_user_id" json:"user_id"`
	ConversationID int       `gorm:"column:conversation_id;index:idx_conversation_id" json:"conversation_id"`
	Question       string    `gorm:"type:text" json:"question"`
	Answer         string    `gorm:"type:text" json:"answer"`
	Rating         int       `gorm:"type:tinyint;not null" json:"rating"`
	Reason         string    `gorm:"type:varchar(500)" json:"reason"`
	SourceDocIDs   string    `gorm:"type:text" json:"source_doc_ids"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (AIFeedback) TableName() string {
	return "aw_ai_feedback"
}
