package knowledge

import "time"

type KnowledgeEntity struct {
	ID          int       `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string    `gorm:"type:varchar(255);not null" json:"name"`
	Type        string    `gorm:"type:varchar(50);not null" json:"type"`
	Description string    `gorm:"type:text" json:"description"`
	DocID       int       `gorm:"column:doc_id;not null;index:idx_doc_id" json:"doc_id"`
	SpaceID     int       `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	Aliases     string    `gorm:"type:text" json:"aliases"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (KnowledgeEntity) TableName() string {
	return "aw_knowledge_entity"
}

type KnowledgeRelation struct {
	ID             int       `gorm:"primaryKey;autoIncrement" json:"id"`
	SourceEntityID int       `gorm:"column:source_entity_id;not null;index:idx_source_entity_id" json:"source_entity_id"`
	TargetEntityID int       `gorm:"column:target_entity_id;not null;index:idx_target_entity_id" json:"target_entity_id"`
	RelationType   string    `gorm:"type:varchar(50);not null" json:"relation_type"`
	Description    string    `gorm:"type:text" json:"description"`
	Confidence     float64   `gorm:"type:decimal(3,2);default:0.8" json:"confidence"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (KnowledgeRelation) TableName() string {
	return "aw_knowledge_relation"
}
