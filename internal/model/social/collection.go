package social

type Collection struct {
	CollectionID int `gorm:"column:collection_id;primaryKey;autoIncrement" json:"collection_id"`
	UserID       int `gorm:"column:user_id;index:idx_user_id" json:"user_id"`
	Type         int `gorm:"column:type" json:"type"`
	ResourceID   int `gorm:"column:resource_id" json:"resource_id"`
	CreateTime   int `gorm:"column:create_time" json:"create_time"`
	UpdateTime   int `gorm:"column:update_time" json:"update_time"`
}

func (Collection) TableName() string {
	return "aw_collection"
}

const (
	CollectionTypeDocument = 1
	CollectionTypeSpace    = 2
)

type CollectionResponse struct {
	ID         int    `json:"id"`
	UserID     int    `json:"user_id"`
	Type       int    `json:"type"`
	ResourceID int    `json:"resource_id"`
	Name       string `json:"name,omitempty"`
	CreateTime int    `json:"create_time"`
}

func (c *Collection) ToResponse() CollectionResponse {
	return CollectionResponse{
		ID:         c.CollectionID,
		UserID:     c.UserID,
		Type:       c.Type,
		ResourceID: c.ResourceID,
		CreateTime: c.CreateTime,
	}
}
