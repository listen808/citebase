package social

import (
	"errors"
	"fmt"
	"time"

	logx "github.com/listen808/citebase/internal/log"
	socialmodel "github.com/listen808/citebase/internal/model/social"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	socialrepo "github.com/listen808/citebase/internal/repository/social"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	workspacesvc "github.com/listen808/citebase/internal/service/workspace"
	"github.com/listen808/citebase/internal/utils/safego"
)

var (
	ErrCommentNotOwner  = errors.New("comment not owned by user")
	ErrCannotFollowSelf = errors.New("cannot follow yourself")
	ErrCannotResolve    = errors.New("no permission to resolve this comment")
)

type CommentService struct {
	commentRepo         *socialrepo.CommentRepository
	docRepo             *workspacerepo.DocumentRepository
	spaceService        *workspacesvc.SpaceService
	userRepo            *userrepo.UserRepository
	notificationService *NotificationService
}

func NewCommentService() *CommentService {
	return &CommentService{
		commentRepo:         socialrepo.NewCommentRepository(),
		docRepo:             workspacerepo.NewDocumentRepository(),
		spaceService:        workspacesvc.NewSpaceService(),
		userRepo:            userrepo.NewUserRepository(),
		notificationService: NewNotificationService(repos.DB),
	}
}

func (s *CommentService) CreateComment(documentID, creatorID int, content, selectionText, selectionRange string, parentID int) (*socialmodel.Comment, error) {
	doc, err := s.docRepo.FindByID(documentID)
	if err != nil {
		return nil, workspacerepo.ErrDocumentNotFound
	}
	if doc.IsDelete == 1 {
		return nil, workspacerepo.ErrDocumentNotFound
	}

	// 写评论同样要求对文档可见，否则任何人可向私有文档注入内容并触发成员通知
	if err := s.ensureDocPrivilege(doc, creatorID, 1); err != nil {
		return nil, err
	}

	// If replying, verify parent exists
	var parentComment *socialmodel.Comment
	if parentID > 0 {
		parentComment, err = s.commentRepo.FindByID(parentID)
		if err != nil {
			return nil, err
		}
		if parentComment.DocumentID != documentID {
			return nil, errors.New("parent comment does not belong to this document")
		}
	}

	now := int(time.Now().Unix())
	comment := &socialmodel.Comment{
		DocumentID:     documentID,
		ParentID:       parentID,
		Content:        content,
		SelectionText:  selectionText,
		SelectionRange: selectionRange,
		CreatorID:      creatorID,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	if err := s.commentRepo.Create(comment); err != nil {
		return nil, err
	}

	// 异步触发通知（不阻塞主流程，失败仅记录日志）
	safego.Go("notifyCommentCreated", func() { s.notifyCommentCreated(comment, doc, parentComment) })

	workspacesvc.FireWebhook(workspacemodel.WebhookEventCommentCreated, int64(doc.SpaceID), func(p *workspacesvc.WebhookEventPayload) {
		p.CommentID = int64(comment.ID)
		p.DocumentID = int64(comment.DocumentID)
		p.DocumentName = doc.Name
		p.ActorID = int64(comment.CreatorID)
	})

	return comment, nil
}

// notifyCommentCreated 评论创建后通知相关用户
func (s *CommentService) notifyCommentCreated(comment *socialmodel.Comment, doc *workspacemodel.Document, parent *socialmodel.Comment) {
	creatorName := s.resolveUserName(comment.CreatorID)
	if parent == nil {
		// 顶级评论：通知文档作者（自己评论自己的文档不通知）
		if doc.CreateUserID != comment.CreatorID {
			title := fmt.Sprintf("%s 评论了文档《%s》", creatorName, doc.Name)
			contentMsg := fmt.Sprintf("%s 在文档《%s》中发表了评论", creatorName, doc.Name)
			if err := s.notificationService.CreateNotification(doc.CreateUserID, socialmodel.NotifTypeComment, title, contentMsg, comment.DocumentID); err != nil {
				logx.Warnf("[comment] notify doc author %d failed: %v", doc.CreateUserID, err)
			}
		}
		return
	}
	// 回复评论：通知父评论作者（自己回复自己不通知）
	if parent.CreatorID != comment.CreatorID {
		title := fmt.Sprintf("%s 回复了你的评论", creatorName)
		contentMsg := fmt.Sprintf("%s 在文档《%s》中回复了你的评论", creatorName, doc.Name)
		if err := s.notificationService.CreateNotification(parent.CreatorID, socialmodel.NotifTypeComment, title, contentMsg, comment.ID); err != nil {
			logx.Warnf("[comment] notify parent author %d failed: %v", parent.CreatorID, err)
		}
	}
}

// resolveUserName 获取用户显示名
func (s *CommentService) resolveUserName(userID int) string {
	u, err := s.userRepo.FindByID(userID)
	if err != nil || u == nil {
		return fmt.Sprintf("用户#%d", userID)
	}
	if u.GivenName != "" {
		return u.GivenName
	}
	return u.Username
}

// ensureDocPrivilege 校验调用者对文档所属空间的最低权限（1=可见，2=可编辑）。
// 评论读写此前完全不校验：任何登录用户都能读出私有文档的评论线程（含引用正文片段），
// 也能在无权访问的文档上写评论并触发对真实成员的通知我。
func (s *CommentService) ensureDocPrivilege(doc *workspacemodel.Document, userID, minPrivilege int) error {
	if userID <= 0 || doc == nil {
		return workspacesvc.ErrNoDocumentPermission
	}
	privilege, err := s.spaceService.GetUserSpacePrivilege(doc.SpaceID, userID)
	if err != nil {
		return err
	}
	if privilege < minPrivilege {
		return workspacesvc.ErrNoDocumentPermission
	}
	return nil
}

// GetDocumentComments 读取文档评论，要求调用者对该文档可见。
func (s *CommentService) GetDocumentComments(documentID, userID int) ([]*socialmodel.Comment, error) {
	doc, err := s.docRepo.FindByID(documentID)
	if err != nil {
		return nil, err
	}
	if err := s.ensureDocPrivilege(doc, userID, 1); err != nil {
		return nil, err
	}
	return s.commentRepo.GetByDocumentID(documentID)
}

func (s *CommentService) DeleteComment(commentID, userID int) error {
	comment, err := s.commentRepo.FindByID(commentID)
	if err != nil {
		return err
	}

	if comment.CreatorID != userID {
		return ErrCommentNotOwner
	}

	return s.commentRepo.Delete(commentID)
}

func (s *CommentService) ResolveComment(commentID, userID int, resolved bool) error {
	comment, err := s.commentRepo.FindByID(commentID)
	if err != nil {
		return err
	}

	// 鉴权：仅评论作者或文档所在空间的编辑者（privilege >= 1）可 resolve
	if comment.CreatorID != userID {
		doc, err := s.docRepo.FindByID(comment.DocumentID)
		if err != nil || doc == nil {
			return ErrCannotResolve
		}
		privilege, err := s.spaceService.GetUserSpacePrivilege(doc.SpaceID, userID)
		if err != nil || privilege < 1 {
			return ErrCannotResolve
		}
	}

	val := 0
	if resolved {
		val = 1
	}
	return s.commentRepo.Resolve(commentID, val)
}
