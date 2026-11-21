package workspace

// DocumentContent 文档正文（P1 落库）
// 正文原先存放在 data/markdowns 的 md 文件里；source=db 时以 document_id 为主键读取本表。
type DocumentContent struct {
	DocumentID  int    `gorm:"column:document_id;primaryKey;autoIncrement:false" json:"document_id"`
	Content     string `gorm:"column:content;type:mediumtext;not null" json:"content"`
	ContentHash string `gorm:"column:content_hash;size:64;not null;default:''" json:"content_hash"`
	ContentSize int    `gorm:"column:content_size;not null;default:0" json:"content_size"`
	UpdateTime  int    `gorm:"column:update_time;not null;default:0" json:"update_time"`
}

// TableName 指定表名
func (DocumentContent) TableName() string {
	return "aw_document_content"
}
