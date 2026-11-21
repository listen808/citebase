package workspace

// DocumentApproval 文档审批流模型 (对应 aw_document_approval 表)
// 支持"提交审批 → 多级审批 → 通过/驳回"的简单线性审批流。
type DocumentApproval struct {
	ID int `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	// idx_approval_status (status, approver) 由 repository.allIndexes 统一定义；
	// 此处不能再挂同名标签，否则 GORM 先建出 (document_id,status)/(status)，
	// ensureAllIndexes 再建同名索引会撞 Duplicate key name，期望的复合索引反而永远建不上。
	DocumentID int    `gorm:"column:document_id;index:idx_approval_doc" json:"document_id"`
	SpaceID    int    `gorm:"column:space_id;index:idx_approval_space" json:"space_id"`
	Submitter  int    `gorm:"column:submitter;index:idx_submitter" json:"submitter"`            // 提交人
	Approver   int    `gorm:"column:approver;index:idx_approver" json:"approver"`               // 当前待审批人
	Status     string `gorm:"column:status;size:20" json:"status"`                              // pending/approved/rejected
	ChainType  string `gorm:"column:chain_type;size:20;default:'sequential'" json:"chain_type"` // sequential/parallel
	Comment    string `gorm:"column:comment;size:1000" json:"comment"`                          // 审批意见
	SubmitTime int    `gorm:"column:submit_time" json:"submit_time"`
	HandleTime int    `gorm:"column:handle_time" json:"handle_time"`
}

func (DocumentApproval) TableName() string {
	return "aw_document_approval"
}

// 审批状态常量
const (
	ApprovalStatusPending  = "pending"
	ApprovalStatusApproved = "approved"
	ApprovalStatusRejected = "rejected"
)

// ApprovalRequest 提交审批请求
type ApprovalRequest struct {
	DocumentID int    `json:"document_id" binding:"required"`
	Approver   int    `json:"approver" binding:"required"` // 指定审批人
	Comment    string `json:"comment"`
}

// ApprovalHandleRequest 审批处理请求
type ApprovalHandleRequest struct {
	Decision string `json:"decision" binding:"required,oneof=approve reject"` // approve/reject
	Comment  string `json:"comment"`
}

// ApprovalResponse 审批记录响应
type ApprovalResponse struct {
	ID         int    `json:"id"`
	DocumentID int    `json:"document_id"`
	SpaceID    int    `json:"space_id"`
	Submitter  int    `json:"submitter"`
	Approver   int    `json:"approver"`
	Status     string `json:"status"`
	Comment    string `json:"comment"`
	SubmitTime int    `json:"submit_time"`
	HandleTime int    `json:"handle_time"`
}

// ToResponse 转换为响应
func (a *DocumentApproval) ToResponse() ApprovalResponse {
	return ApprovalResponse{
		ID:         a.ID,
		DocumentID: a.DocumentID,
		SpaceID:    a.SpaceID,
		Submitter:  a.Submitter,
		Approver:   a.Approver,
		Status:     a.Status,
		Comment:    a.Comment,
		SubmitTime: a.SubmitTime,
		HandleTime: a.HandleTime,
	}
}
