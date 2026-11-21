package workspace

import (
	"fmt"

	"github.com/listen808/citebase/internal/api/websocket"
	logx "github.com/listen808/citebase/internal/log"
	socialmodel "github.com/listen808/citebase/internal/model/social"
	usermodel "github.com/listen808/citebase/internal/model/user"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	socialrepo "github.com/listen808/citebase/internal/repository/social"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/utils/safego"
)

// MentionService @提及 服务
type MentionService struct {
	mentionRepo  *workspacerepo.DocumentMentionRepository
	userRepo     *userrepo.UserRepository
	docRepo      *workspacerepo.DocumentRepository
	notifRepo    *socialrepo.NotificationRepository
	spaceService *SpaceService
}

// NewMentionService 创建服务实例
func NewMentionService() *MentionService {
	return &MentionService{
		mentionRepo:  workspacerepo.NewDocumentMentionRepository(),
		userRepo:     userrepo.NewUserRepository(),
		docRepo:      workspacerepo.NewDocumentRepository(),
		notifRepo:    socialrepo.NewNotificationRepository(repos.DB),
		spaceService: NewSpaceService(),
	}
}

// ResolveAndSave 解析文本中的 @ 标识，落库提及关系，并对新增被提及用户发送通知。
// editorUserID 为编辑者，避免通知自己。
func (s *MentionService) ResolveAndSave(docID, editorUserID int, content string) error {
	names := workspacemodel.ExtractMentions(content)
	if len(names) == 0 {
		// 无提及也需清空旧记录，保证与内容一致
		return s.mentionRepo.ReplaceByDocument(docID, nil)
	}

	userIDs := make([]int, 0, len(names))
	for _, name := range names {
		u, err := s.userRepo.FindByUsername(name)
		if err != nil {
			// given_name 兜底必须精确匹配：旧实现取模糊搜索结果的第一条，
			// 会把通知发给同名不同人的错误用户。
			users, _, ferr := s.userRepo.FindAll(0, 10, 0, name)
			if ferr == nil {
				for _, cand := range users {
					if cand.GivenName == name {
						u = cand
						break
					}
				}
			}
			if u == nil {
				logx.Warnf("[mention] user not found for @%s, skipped", name)
				continue
			}
		}
		userIDs = append(userIDs, u.ID)
	}

	if err := s.mentionRepo.ReplaceByDocument(docID, userIDs); err != nil {
		return err
	}

	// 发送通知（异步，避免阻塞主流程）
	safego.Go("notifyNewMentions", func() { s.notifyNewMentions(docID, editorUserID, userIDs) })
	return nil
}

func (s *MentionService) notifyNewMentions(docID, editorUserID int, userIDs []int) {
	// 文档解析失败时不发通知（旧实现退化成用文档 ID 当标题继续广播）
	doc, err := s.docRepo.FindByID(docID)
	if err != nil || doc == nil {
		return
	}
	docName := doc.Name
	editorName := "某用户"
	if u, eerr := s.userRepo.FindByID(editorUserID); eerr == nil && u != nil {
		if u.GivenName != "" {
			editorName = u.GivenName
		} else {
			editorName = u.Username
		}
	}
	hub := websocket.GetHub()
	for _, uid := range userIDs {
		if uid == editorUserID {
			continue
		}
		// 只通知对该文档所在空间有读权的用户：否则私有文档标题与「谁被提及」
		// 会被广播给任意陌生人（信息泄露 + 点击通知后 403 的权限模型不一致）。
		if priv, perr := s.spaceService.GetUserSpacePrivilege(doc.SpaceID, uid); perr != nil || priv < 1 {
			continue
		}
		notif := &socialmodel.Notification{
			UserID:    uid,
			Type:      socialmodel.NotifTypeDocument,
			Title:     fmt.Sprintf("%s 在文档《%s》中提到了你", editorName, docName),
			Content:   fmt.Sprintf("你在文档《%s》中被 @ 提及", docName),
			RelatedID: docID,
		}
		if nerr := s.notifRepo.Create(notif); nerr != nil {
			logx.Warnf("[mention] failed to notify user %d: %v", uid, nerr)
			continue
		}
		// 实时 WebSocket 推送
		hub.NotifyUser(uid, notif.Title, notif.Content, notif.Type, docID)
		if unreadCount, cerr := s.notifRepo.GetUnreadCount(uid); cerr == nil {
			hub.NotifyUnreadCount(uid, unreadCount)
		}
	}
}

// GetMentions 获取文档的被提及用户列表
func (s *MentionService) GetMentions(docID int) ([]workspacemodel.MentionUser, error) {
	userIDs, err := s.mentionRepo.FindByDocument(docID)
	if err != nil {
		return nil, err
	}
	// 一条 IN 批量取，而不是逐个 FindByID：原先一篇被 @ 了 30 个人的文档要发 30 次查询。
	// 更要紧的是原先写的是 `u, err := FindByID(uid); if err != nil { continue }` ——
	// 数据库真出错时那几个人就从列表里**安静地消失**，界面上看不出任何异常，
	// 而被 @ 的人其实收到了通知。批量取只有一个错误点，出错就整体返回错误。
	users, err := s.userRepo.FindByIDs(userIDs)
	if err != nil {
		return nil, err
	}
	// 结果顺序按 userIDs（@ 出现的先后）走，而不是按数据库返回的顺序 ——
	// IN 查询不保证次序，而界面上这个列表是"@ 过谁"的列表。
	byID := make(map[int]*usermodel.User, len(users))
	for _, u := range users {
		if u == nil {
			continue
		}
		byID[u.ID] = u
	}
	out := make([]workspacemodel.MentionUser, 0, len(userIDs))
	for _, uid := range userIDs {
		u, ok := byID[uid]
		if !ok {
			continue // 账号已删除：提及记录还在，但人已经不可见了
		}
		out = append(out, workspacemodel.MentionUser{
			UserID:    u.ID,
			Username:  u.Username,
			GivenName: u.GivenName,
		})
	}
	return out, nil
}
