package workspace

import "time"

// DocumentPermission 文档级 ACL。
//
// P1-3：此前权限只到空间成员级（aw_space_user），任何空间成员都能看到空间内
// 全部文档。企业场景下"这份薪酬制度只有 HR 能看"这类需求无法表达，
// AI 问答还会把受限文档的内容带进上下文——一旦进了上下文，风险就已经发生，
// 所以要在检索**前**过滤、检索**后**再校验一次。
type DocumentPermission struct {
	ID      int `gorm:"primaryKey;autoIncrement" json:"id"`
	DocID   int `gorm:"column:doc_id;not null;uniqueIndex:uk_doc_subject" json:"doc_id"`
	SpaceID int `gorm:"column:space_id;not null;index:idx_space_id" json:"space_id"`
	// SubjectType: user | role
	SubjectType string `gorm:"column:subject_type;type:varchar(16);not null;uniqueIndex:uk_doc_subject" json:"subject_type"`
	SubjectID   int    `gorm:"column:subject_id;not null;uniqueIndex:uk_doc_subject" json:"subject_id"`
	// Effect: deny 优先于 allow。同一 (doc, subject) 只保留一条记录。
	Effect    string    `gorm:"column:effect;type:varchar(8);not null" json:"effect"`
	CreatedBy int       `gorm:"column:created_by" json:"created_by"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (DocumentPermission) TableName() string {
	return "aw_document_permission"
}

const (
	ACLEffectAllow = "allow"
	ACLEffectDeny  = "deny"

	ACLSubjectUser = "user"
	ACLSubjectRole = "role"
)
