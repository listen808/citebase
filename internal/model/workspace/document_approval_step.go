package workspace

import (
	"time"

	"gorm.io/gorm"
)

// DocumentApprovalStep represents a single step in a multi-level approval chain.
type DocumentApprovalStep struct {
	ID         int64  `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	ApprovalID int64  `gorm:"column:approval_id;not null;index:idx_step_approval" json:"approval_id"`
	StepOrder  int    `gorm:"column:step_order;not null" json:"step_order"`
	ApproverID int64  `gorm:"column:approver_id;not null;index:idx_step_approver" json:"approver_id"`
	Status     string `gorm:"column:status;size:20;not null;default:'pending'" json:"status"` // pending/approved/rejected/skipped
	Comment    string `gorm:"column:comment;size:1000;default:''" json:"comment"`
	HandleTime int64  `gorm:"column:handle_time;not null;default:0" json:"handle_time"`
	CreateTime int64  `gorm:"column:create_time;not null;default:0" json:"create_time"`
}

func (DocumentApprovalStep) TableName() string {
	return "aw_document_approval_step"
}

func (s *DocumentApprovalStep) BeforeCreate(tx *gorm.DB) error {
	if s.CreateTime == 0 {
		s.CreateTime = time.Now().Unix()
	}
	return nil
}

// ApprovalStepStatus constants
const (
	ApprovalStepPending  = "pending"
	ApprovalStepApproved = "approved"
	ApprovalStepRejected = "rejected"
	ApprovalStepSkipped  = "skipped"
)

// ApprovalChainType constants
const (
	ApprovalChainSequential = "sequential"
	ApprovalChainParallel   = "parallel"
)

// ApprovalStepResponse is the API response for approval steps.
type ApprovalStepResponse struct {
	ID         int64  `json:"id"`
	ApprovalID int64  `json:"approval_id"`
	StepOrder  int    `json:"step_order"`
	ApproverID int64  `json:"approver_id"`
	Status     string `json:"status"`
	Comment    string `json:"comment"`
	HandleTime int64  `json:"handle_time"`
	CreateTime int64  `json:"create_time"`
}

// MultiLevelApprovalRequest extends the basic approval request for multi-level chains.
type MultiLevelApprovalRequest struct {
	DocumentID int                   `json:"document_id" binding:"required"`
	ChainType  string                `json:"chain_type" binding:"required,oneof=sequential parallel"`
	Steps      []ApprovalStepRequest `json:"steps" binding:"required,min=1"`
	Comment    string                `json:"comment"`
}

// ApprovalStepRequest defines a single step in the chain.
type ApprovalStepRequest struct {
	StepOrder  int   `json:"step_order" binding:"required"`
	ApproverID int64 `json:"approver_id" binding:"required"`
}

// ApprovalChainResponse includes step information.
type ApprovalChainResponse struct {
	ID         int                    `json:"id"`
	DocumentID int                    `json:"document_id"`
	SpaceID    int                    `json:"space_id"`
	Submitter  int                    `json:"submitter"`
	ChainType  string                 `json:"chain_type"`
	Status     string                 `json:"status"`
	Comment    string                 `json:"comment"`
	SubmitTime int                    `json:"submit_time"`
	HandleTime int                    `json:"handle_time"`
	Steps      []ApprovalStepResponse `json:"steps,omitempty"`
}

// ToChainResponse converts to chain response.
func (a *DocumentApproval) ToChainResponse() ApprovalChainResponse {
	return ApprovalChainResponse{
		ID:         a.ID,
		DocumentID: a.DocumentID,
		SpaceID:    a.SpaceID,
		Submitter:  a.Submitter,
		Status:     a.Status,
		Comment:    a.Comment,
		SubmitTime: a.SubmitTime,
		HandleTime: a.HandleTime,
	}
}
