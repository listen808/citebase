package user

import (
	"time"
)

// Contact 联系人
type Contact struct {
	ContactID  uint      `gorm:"primaryKey" json:"contact_id"`
	Name       string    `gorm:"size:50;not null" json:"name"`
	Mobile     string    `gorm:"size:13;not null" json:"mobile"`
	Email      string    `gorm:"size:50;not null" json:"email"`
	Position   string    `gorm:"size:100" json:"position"`
	CreateTime int64     `json:"create_time"`
	UpdateTime int64     `json:"update_time"`
	CreatedAt  time.Time `json:"-"`
	UpdatedAt  time.Time `json:"-"`
}

func (Contact) TableName() string {
	return "aw_contact"
}
