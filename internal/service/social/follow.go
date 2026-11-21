package social

import (
	"fmt"
	"time"

	logx "github.com/listen808/citebase/internal/log"
	socialmodel "github.com/listen808/citebase/internal/model/social"
	usermodel "github.com/listen808/citebase/internal/model/user"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	repos "github.com/listen808/citebase/internal/repository"
	socialrepo "github.com/listen808/citebase/internal/repository/social"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/utils"
	"github.com/listen808/citebase/internal/utils/safego"
)

type FollowService struct {
	followRepo          *socialrepo.FollowRepository
	userRepo            *userrepo.UserRepository
	docRepo             *workspacerepo.DocumentRepository
	spaceRepo           *workspacerepo.SpaceRepository
	notificationService *NotificationService
}

func NewFollowService() *FollowService {
	return &FollowService{
		followRepo:          socialrepo.NewFollowRepository(),
		userRepo:            userrepo.NewUserRepository(),
		docRepo:             workspacerepo.NewDocumentRepository(),
		spaceRepo:           workspacerepo.NewSpaceRepository(),
		notificationService: NewNotificationService(repos.DB),
	}
}

func (s *FollowService) ToggleFollowUser(followerID int, targetUserID int) (bool, *socialmodel.FollowResponse, error) {
	if followerID == targetUserID {
		return false, nil, ErrCannotFollowSelf
	}

	targetUser, err := s.userRepo.FindByID(targetUserID)
	if err != nil {
		return false, nil, userrepo.ErrUserNotFound
	}
	if targetUser.IsDelete == 1 {
		return false, nil, userrepo.ErrUserNotFound
	}

	existing, err := s.followRepo.FindByUserAndObject(followerID, socialmodel.FollowTypeUser, targetUserID)
	if err != nil && err != socialrepo.ErrFollowNotFound {
		return false, nil, err
	}

	if existing != nil {
		if err := s.followRepo.DeleteByUserAndObject(existing.UserID, existing.Type, existing.ObjectID); err != nil {
			return false, nil, err
		}
		resp := existing.ToResponse()
		resp.ObjectName = targetUser.GivenName
		if resp.ObjectName == "" {
			resp.ObjectName = targetUser.Username
		}
		resp.TargetUser = &socialmodel.FollowUserInfo{
			UserID:    targetUser.ID,
			Username:  targetUser.Username,
			GivenName: targetUser.GivenName,
			Email:     targetUser.Email,
		}
		return false, &resp, nil
	}

	follow := &socialmodel.Follow{
		UserID:   followerID,
		Type:     socialmodel.FollowTypeUser,
		ObjectID: targetUserID,
	}
	if err := s.followRepo.Create(follow); err != nil {
		// 并发双击会撞 idx_user_object_type 的 1062 —— 那是"已经关注了"，不是失败。
		if socialrepo.IsDuplicateKey(err) {
			if existing, findErr := s.followRepo.FindByUserAndObject(followerID, socialmodel.FollowTypeUser, targetUserID); findErr == nil && existing != nil {
				resp := existing.ToResponse()
				resp.ObjectName = targetUser.GivenName
				if resp.ObjectName == "" {
					resp.ObjectName = targetUser.Username
				}
				return true, &resp, nil
			}
		}
		return false, nil, err
	}
	resp := follow.ToResponse()
	resp.ObjectName = targetUser.GivenName
	if resp.ObjectName == "" {
		resp.ObjectName = targetUser.Username
	}
	resp.TargetUser = &socialmodel.FollowUserInfo{
		UserID:    targetUser.ID,
		Username:  targetUser.Username,
		GivenName: targetUser.GivenName,
		Email:     targetUser.Email,
	}

	// 异步通知被关注用户
	safego.Go("notifyUserFollowed", func() { s.notifyUserFollowed(followerID, targetUserID) })

	return true, &resp, nil
}

// notifyUserFollowed 通知被关注用户
func (s *FollowService) notifyUserFollowed(followerID, targetUserID int) {
	followerName := "某用户"
	if u, err := s.userRepo.FindByID(followerID); err == nil && u != nil {
		if u.GivenName != "" {
			followerName = u.GivenName
		} else {
			followerName = u.Username
		}
	}
	title := fmt.Sprintf("%s 关注了你", followerName)
	contentMsg := fmt.Sprintf("用户 %s 开始关注你", followerName)
	if err := s.notificationService.CreateNotification(targetUserID, socialmodel.NotifTypeFollow, title, contentMsg, followerID); err != nil {
		logx.Warnf("[follow] notify target user %d failed: %v", targetUserID, err)
	}
}

func (s *FollowService) ToggleFollowDocument(userID int, documentID int) (bool, *socialmodel.FollowResponse, error) {
	doc, err := s.docRepo.FindByID(documentID)
	if err != nil {
		return false, nil, workspacerepo.ErrDocumentNotFound
	}
	if doc.IsDelete == 1 {
		return false, nil, workspacerepo.ErrDocumentNotFound
	}

	existing, err := s.followRepo.FindByUserAndObject(userID, socialmodel.FollowTypeDocument, documentID)
	if err != nil && err != socialrepo.ErrFollowNotFound {
		return false, nil, err
	}

	if existing != nil {
		if err := s.followRepo.DeleteByUserAndObject(existing.UserID, existing.Type, existing.ObjectID); err != nil {
			return false, nil, err
		}
		resp := existing.ToResponse()
		resp.ObjectName = doc.Name
		resp.TargetDoc = s.buildTargetDoc(doc)
		return false, &resp, nil
	}

	follow := &socialmodel.Follow{
		UserID:   userID,
		Type:     socialmodel.FollowTypeDocument,
		ObjectID: documentID,
	}
	if err := s.followRepo.Create(follow); err != nil {
		if socialrepo.IsDuplicateKey(err) {
			if existing, findErr := s.followRepo.FindByUserAndObject(userID, socialmodel.FollowTypeDocument, documentID); findErr == nil && existing != nil {
				resp := existing.ToResponse()
				resp.ObjectName = doc.Name
				resp.TargetDoc = s.buildTargetDoc(doc)
				return true, &resp, nil
			}
		}
		return false, nil, err
	}
	resp := follow.ToResponse()
	resp.ObjectName = doc.Name
	resp.TargetDoc = s.buildTargetDoc(doc)
	return true, &resp, nil
}

func (s *FollowService) GetFollowingList(userID int) ([]*socialmodel.FollowResponse, error) {
	userFollows, err := s.followRepo.GetUserFollowing(userID)
	if err != nil {
		return nil, err
	}
	docFollows, err := s.followRepo.GetUserFollowDocs(userID)
	if err != nil {
		return nil, err
	}

	follows := append(userFollows, docFollows...)

	userIDs, docIDs := s.collectFollowIDs(follows)
	userMap, docMap := s.batchFetchFollowRelations(userIDs, docIDs)

	responses := make([]*socialmodel.FollowResponse, len(follows))
	for i, f := range follows {
		resp := f.ToResponse()
		if f.Type == socialmodel.FollowTypeUser {
			if u, ok := userMap[f.ObjectID]; ok {
				resp.ObjectName = u.GivenName
				if resp.ObjectName == "" {
					resp.ObjectName = u.Username
				}
				resp.TargetUser = &socialmodel.FollowUserInfo{
					UserID:    u.ID,
					Username:  u.Username,
					GivenName: u.GivenName,
					Email:     u.Email,
				}
			}
		} else if f.Type == socialmodel.FollowTypeDocument {
			if d, ok := docMap[f.ObjectID]; ok {
				resp.ObjectName = d.Name
				resp.TargetDoc = s.buildTargetDoc(d)
			}
		}
		responses[i] = &resp
	}
	return responses, nil
}

func (s *FollowService) GetFollowersList(userID int) ([]*socialmodel.FollowResponse, error) {
	follows, err := s.followRepo.GetUserFollowers(userID)
	if err != nil {
		return nil, err
	}

	followerUserIDs := make([]int, 0, len(follows))
	for _, f := range follows {
		followerUserIDs = append(followerUserIDs, f.UserID)
	}
	followerUserIDs = utils.UniqueInts(followerUserIDs)

	userMap := make(map[int]*usermodel.User)
	if len(followerUserIDs) > 0 {
		users, uerr := s.userRepo.FindByIDs(followerUserIDs)
		if uerr != nil {
			logx.Errorf("[Follow] 批量查询粉丝用户失败: %v", uerr)
		}
		for _, u := range users {
			userMap[u.ID] = u
		}
	}

	responses := make([]*socialmodel.FollowResponse, len(follows))
	for i, f := range follows {
		resp := f.ToResponse()
		if u, ok := userMap[f.UserID]; ok {
			resp.ObjectName = u.GivenName
			if resp.ObjectName == "" {
				resp.ObjectName = u.Username
			}
			resp.TargetUser = &socialmodel.FollowUserInfo{
				UserID:    u.ID,
				Username:  u.Username,
				GivenName: u.GivenName,
				Email:     u.Email,
			}
		}
		responses[i] = &resp
	}
	return responses, nil
}

func (s *FollowService) GetFollowStats(userID int) (following int64, followers int64, err error) {
	following, err = s.followRepo.CountFollowing(userID, socialmodel.FollowTypeUser)
	if err != nil {
		return 0, 0, err
	}
	followers, err = s.followRepo.CountFollowers(userID, socialmodel.FollowTypeUser)
	if err != nil {
		return 0, 0, err
	}
	return following, followers, nil
}

func (s *FollowService) IsFollowingUser(followerID int, targetUserID int) bool {
	return s.followRepo.IsFollowing(followerID, socialmodel.FollowTypeUser, targetUserID)
}

func (s *FollowService) IsFollowingDocument(userID int, documentID int) bool {
	return s.followRepo.IsFollowing(userID, socialmodel.FollowTypeDocument, documentID)
}

func (s *FollowService) buildTargetDoc(doc *workspacemodel.Document) *socialmodel.FollowDocInfo {
	info := &socialmodel.FollowDocInfo{
		DocumentID: doc.ID,
		Name:       doc.Name,
		SpaceID:    doc.SpaceID,
	}
	if space, err := s.spaceRepo.FindByID(doc.SpaceID); err == nil {
		info.SpaceName = space.Name
	}
	return info
}

func (s *FollowService) collectFollowIDs(follows []*socialmodel.Follow) (userIDs []int, docIDs []int) {
	for _, f := range follows {
		if f.Type == socialmodel.FollowTypeUser && f.ObjectID > 0 {
			userIDs = append(userIDs, f.ObjectID)
		} else if f.Type == socialmodel.FollowTypeDocument && f.ObjectID > 0 {
			docIDs = append(docIDs, f.ObjectID)
		}
	}
	userIDs = utils.UniqueInts(userIDs)
	docIDs = utils.UniqueInts(docIDs)
	return
}

func (s *FollowService) batchFetchFollowRelations(userIDs []int, docIDs []int) (
	userMap map[int]*usermodel.User,
	docMap map[int]*workspacemodel.Document,
) {
	userMap = make(map[int]*usermodel.User)
	docMap = make(map[int]*workspacemodel.Document)
	if len(userIDs) > 0 {
		// 旧实现丢弃错误、静默返回空 map，前端表现为「关注对象不存在」但无人知晓是 DB 故障
		users, err := s.userRepo.FindByIDs(userIDs)
		if err != nil {
			logx.Errorf("[Follow] 批量查询关注用户失败: %v", err)
		}
		for _, u := range users {
			userMap[u.ID] = u
		}
	}
	if len(docIDs) > 0 {
		docs, err := s.docRepo.FindByIDs(docIDs)
		if err != nil {
			logx.Errorf("[Follow] 批量查询关注文档失败: %v", err)
		}
		for _, d := range docs {
			docMap[d.ID] = d
		}
	}
	return
}

func (s *FollowService) FollowDocument(userID int, documentID int) error {
	if s.followRepo.IsFollowing(userID, socialmodel.FollowTypeDocument, documentID) {
		return nil
	}

	follow := &socialmodel.Follow{
		UserID:     userID,
		Type:       socialmodel.FollowTypeDocument,
		ObjectID:   documentID,
		CreateTime: int(time.Now().Unix()),
	}
	if err := s.followRepo.Create(follow); err != nil {
		// 这条同样是「先 IsFollowing 再 Insert」，并发下撞 1062 就是"已经关注了"。
		if socialrepo.IsDuplicateKey(err) {
			return nil
		}
		return err
	}
	return nil
}
