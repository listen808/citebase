package ai

import (
	"database/sql/driver"
	"encoding/json"
	"time"
)

type AIConversation struct {
	ID         int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	UserID     int       `gorm:"column:user_id;index:idx_user_id;not null" json:"user_id"`
	SpaceID    int       `gorm:"column:space_id;default:0" json:"space_id"`
	Type       string    `gorm:"column:type;size:20;default:ai;index:idx_type" json:"type"`
	Title      string    `gorm:"column:title;size:255" json:"title"`
	Messages   ConvMsgs  `gorm:"column:messages;type:json;not null" json:"messages"`
	TokenUsage int       `gorm:"column:token_usage;default:0" json:"token_usage"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (AIConversation) TableName() string {
	return "aw_ai_conversation"
}

type ConvMsgs []map[string]interface{}

func (m ConvMsgs) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	return json.Marshal(m)
}

func (m *ConvMsgs) Scan(value interface{}) error {
	if value == nil {
		*m = nil
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, m)
}
