package workspace

// DocumentTag 文档-标签关联模型 (对应 aw_document_tag 表)
type DocumentTag struct {
	ID         int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	DocumentID int    `gorm:"column:document_id;uniqueIndex:uk_doc_tag;index:idx_document_id" json:"document_id"`
	TagName    string `gorm:"column:tag_name;size:100;uniqueIndex:uk_doc_tag" json:"tag_name"`
	CreateTime int    `gorm:"column:create_time" json:"create_time"`
}

func (DocumentTag) TableName() string {
	return "aw_document_tag"
}

// TagStats 标签统计
type TagStats struct {
	TagName string `json:"tag_name"`
	Count   int    `json:"count"`
}
