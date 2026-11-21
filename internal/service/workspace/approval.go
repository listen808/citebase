package workspace

import (
	"errors"
	"fmt"

	logx "github.com/listen808/citebase/internal/log"
	socialmodel "github.com/listen808/citebase/internal/model/social"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	socialrepo "github.com/listen808/citebase/internal/repository/social"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/utils/safego"
)

// 审批相关错误
var (
	ErrApprovalExists   = errors.New("document already has a pending approval")
	ErrApprovalConflict = errors.New("no pending approval for this document")
	ErrNotApprover      = errors.New("current user is not the approver")
)

// ApprovalService 文档审批流服务
type ApprovalService struct {
	approvalRepo *workspacerepo.DocumentApprovalRepository
	docRepo      *workspacerepo.DocumentRepository
	userRepo     *userrepo.UserRepository
	spaceService *SpaceService
	// notifRepo 通过构造函数注入数据库依赖（P0：之前 notifySubmitter 用 nil *gorm.DB
	// 创建仓库，异步协程内立刻 panic 并击穿整个进程）
	notifRepo *socialrepo.NotificationRepository
}

// NewApprovalService 创建服务实例
func NewApprovalService() *ApprovalService {
	s := &ApprovalService{
		approvalRepo: workspacerepo.NewDocumentApprovalRepository(),
		docRepo:      workspacerepo.NewDocumentRepository(),
		userRepo:     userrepo.NewUserRepository(),
		spaceService: NewSpaceService(),
	}
	if repos.DB != nil {
		s.notifRepo = socialrepo.NewNotificationRepository(repos.DB)
	}
	return s
}

// Submit 提交审批：校验权限、避免重复 pending、创建审批记录。
func (s *ApprovalService) Submit(submitter int, req *workspacemodel.ApprovalRequest) (*workspacemodel.DocumentApproval, error) {
	if req.Approver == submitter {
		return nil, errors.New("cannot approve your own submission")
	}
	doc, err := s.docRepo.FindByID(req.DocumentID)
	if err != nil {
		return nil, err
	}
	// 提交人需有文档编辑权限
	priv, err := s.spaceService.GetUserSpacePrivilege(doc.SpaceID, submitter)
	if err != nil {
		return nil, err
	}
	if priv < 1 {
		return nil, errors.New("no permission to submit approval for this document")
	}
	// 审批人需存在
	if _, err := s.userRepo.FindByID(req.Approver); err != nil {
		return nil, errors.New("approver not found")
	}
	// 避免重复 pending
	existing, eerr := s.approvalRepo.FindByDocument(req.DocumentID)
	if eerr == nil && existing.Status == workspacemodel.ApprovalStatusPending {
		return nil, ErrApprovalExists
	}

	a := &workspacemodel.DocumentApproval{
		DocumentID: req.DocumentID,
		SpaceID:    doc.SpaceID,
		Submitter:  submitter,
		Approver:   req.Approver,
		Status:     workspacemodel.ApprovalStatusPending,
		Comment:    req.Comment,
	}
	if err := s.approvalRepo.Create(a); err != nil {
		return nil, err
	}
	FireWebhook(workspacemodel.WebhookEventApprovalSubmitted, int64(a.SpaceID), func(p *WebhookEventPayload) {
		p.ApprovalID = int64(a.ID)
		p.DocumentID = int64(a.DocumentID)
		p.ActorID = int64(submitter)
	})
	return a, nil
}

// Handle 处理审批：通过/驳回，并通知提交人。
func (s *ApprovalService) Handle(approvalID, approver int, req *workspacemodel.ApprovalHandleRequest) (*workspacemodel.DocumentApproval, error) {
	a, err := s.approvalRepo.FindByID(approvalID)
	if err != nil {
		return nil, err
	}
	rec := *a
	if rec.Status != workspacemodel.ApprovalStatusPending {
		return nil, ErrApprovalConflict
	}
	if rec.Approver != approver {
		return nil, ErrNotApprover
	}
	status := workspacemodel.ApprovalStatusApproved
	if req.Decision == "reject" {
		status = workspacemodel.ApprovalStatusRejected
	}
	if err := s.approvalRepo.UpdateStatus(rec.ID, approver, status, req.Comment); err != nil {
		return nil, err
	}
	rec.Status = status
	rec.Comment = req.Comment

	// 通知提交人（异步）。协程必须自带 recover：这里的 panic 发生在 gin Recovery 之外，
	// 未捕获会直接终止进程。
	safego.Go("notifySubmitterSafe", func() { s.notifySubmitterSafe(rec) })
	FireWebhook(workspacemodel.WebhookEventApprovalHandled, int64(rec.SpaceID), func(p *WebhookEventPayload) {
		p.ApprovalID = int64(rec.ID)
		p.DocumentID = int64(rec.DocumentID)
		p.ActorID = int64(approver)
	})
	return &rec, nil
}

// notifySubmitterSafe 包裹 notifySubmitter，捕获 panic 并记录日志。
func (s *ApprovalService) notifySubmitterSafe(a workspacemodel.DocumentApproval) {
	defer func() {
		if r := recover(); r != nil {
			logx.Errorf("[approval] notify submitter %d panicked: %v", a.Submitter, r)
		}
	}()
	s.notifySubmitter(a)
}

func (s *ApprovalService) notifySubmitter(a workspacemodel.DocumentApproval) {
	repo := s.notifRepo
	if repo == nil {
		// 服务构造时数据库可能尚未初始化，这里再取一次全局 DB（不缓存，避免与并发读产生竞争）；
		// 仍然为空则放弃通知而不是 panic
		if repos.DB == nil {
			logx.Errorf("[approval] notification repository unavailable (db nil), skip notifying submitter %d", a.Submitter)
			return
		}
		repo = socialrepo.NewNotificationRepository(repos.DB)
	}
	title := "审批通过"
	if a.Status == workspacemodel.ApprovalStatusRejected {
		title = "审批驳回"
	}
	notif := &socialmodel.Notification{
		UserID:    a.Submitter,
		Type:      socialmodel.NotifTypeDocument,
		Title:     fmt.Sprintf("你的文档审批已%s", title),
		Content:   a.Comment,
		RelatedID: a.DocumentID,
	}
	if nerr := repo.Create(notif); nerr != nil {
		logx.Warnf("[approval] failed to notify submitter %d: %v", a.Submitter, nerr)
	}
}

// GetByDocument 获取文档审批记录
func (s *ApprovalService) GetByDocument(docID int) (*workspacemodel.DocumentApproval, error) {
	return s.approvalRepo.FindByDocument(docID)
}

// ListPending 待我审批列表
func (s *ApprovalService) ListPending(approver, page, pageSize int) ([]workspacemodel.ApprovalResponse, int64, error) {
	list, total, err := s.approvalRepo.FindByApprover(approver, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}
	resp := make([]workspacemodel.ApprovalResponse, len(list))
	for i, a := range list {
		resp[i] = a.ToResponse()
	}
	return resp, total, nil
}

// ListSubmitted 我提交的列表
func (s *ApprovalService) ListSubmitted(submitter, page, pageSize int) ([]workspacemodel.ApprovalResponse, int64, error) {
	list, total, err := s.approvalRepo.FindBySubmitter(submitter, (page-1)*pageSize, pageSize)
	if err != nil {
		return nil, 0, err
	}
	resp := make([]workspacemodel.ApprovalResponse, len(list))
	for i, a := range list {
		resp[i] = a.ToResponse()
	}
	return resp, total, nil
}
