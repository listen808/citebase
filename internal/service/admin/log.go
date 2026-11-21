package admin

import (
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	usermodel "github.com/listen808/citebase/internal/model/user"
	workspacemodel "github.com/listen808/citebase/internal/model/workspace"
	adminrepo "github.com/listen808/citebase/internal/repository/admin"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"
	"github.com/listen808/citebase/internal/utils"
)

type LogService struct {
	logRepo   *adminrepo.LogRepository
	userRepo  *userrepo.UserRepository
	docRepo   *workspacerepo.DocumentRepository
	spaceRepo *workspacerepo.SpaceRepository
}

func NewLogService() *LogService {
	return &LogService{
		logRepo:   adminrepo.NewLogRepository(),
		userRepo:  userrepo.NewUserRepository(),
		docRepo:   workspacerepo.NewDocumentRepository(),
		spaceRepo: workspacerepo.NewSpaceRepository(),
	}
}

func (s *LogService) LogDocumentAction(documentID, spaceID, userID, action int, comment string) error {
	docLog := &adminmodel.DocumentLog{
		DocumentID: documentID,
		SpaceID:    spaceID,
		UserID:     userID,
		Action:     action,
		Comment:    comment,
	}
	return s.logRepo.CreateDocumentLog(docLog)
}

func (s *LogService) GetDocumentHistory(documentID int, limit int) ([]*adminmodel.DocumentLogResponse, error) {
	if limit <= 0 {
		limit = 20
	}

	logs, err := s.logRepo.GetDocumentLogs(documentID, limit)
	if err != nil {
		return nil, err
	}

	// 批量取用户（与同文件 batchFetchLogRelations 口径一致）：旧实现对每条日志单查一次用户，limit=200 时就是 200 条 SQL
	userIDs := make([]int, 0, len(logs))
	for _, l := range logs {
		if l.UserID > 0 {
			userIDs = append(userIDs, l.UserID)
		}
	}
	userMap := make(map[int]*usermodel.User)
	if len(userIDs) > 0 {
		if users, uerr := s.userRepo.FindByIDs(utils.UniqueInts(userIDs)); uerr == nil {
			for _, u := range users {
				userMap[u.ID] = u
			}
		}
	}

	responses := make([]*adminmodel.DocumentLogResponse, len(logs))
	for i, log := range logs {
		resp := log.ToResponse()
		if user, ok := userMap[log.UserID]; ok && user != nil {
			resp.Username = user.Username
			resp.GivenName = user.GivenName
		}
		responses[i] = &resp
	}

	return responses, nil
}

func (s *LogService) GetUserActivity(userID, page, pageSize int) ([]*adminmodel.DocumentLogResponse, int64, error) {
	offset := (page - 1) * pageSize

	logs, total, err := s.logRepo.GetUserDocumentLogs(userID, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}

	userMap, docMap, spaceMap := s.batchFetchLogRelations(logs)

	responses := make([]*adminmodel.DocumentLogResponse, len(logs))
	for i, log := range logs {
		resp := log.ToResponse()
		item := adminmodel.DocumentLogResponse{
			ID:         resp.ID,
			DocumentID: resp.DocumentID,
			SpaceID:    resp.SpaceID,
			UserID:     resp.UserID,
			Username:   resp.Username,
			Action:     resp.Action,
			ActionName: resp.ActionName,
			Comment:    resp.Comment,
			CreateTime: resp.CreateTime,
		}
		if u, ok := userMap[log.UserID]; ok {
			item.Username = u.Username
			item.GivenName = u.GivenName
		}
		if d, ok := docMap[log.DocumentID]; ok {
			item.DocumentName = d.Name
		}
		if sp, ok := spaceMap[log.SpaceID]; ok {
			item.SpaceName = sp.Name
		}
		responses[i] = &item
	}

	return responses, total, nil
}

func (s *LogService) GetUserActivityFiltered(userID int, actionFilter string, page, pageSize int) ([]*adminmodel.DocumentLogResponse, int64, error) {
	offset := (page - 1) * pageSize

	logs, total, err := s.logRepo.GetUserDocumentLogsFiltered(userID, actionFilter, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}

	userMap, docMap, spaceMap := s.batchFetchLogRelations(logs)

	responses := make([]*adminmodel.DocumentLogResponse, len(logs))
	for i, log := range logs {
		resp := log.ToResponse()
		item := adminmodel.DocumentLogResponse{
			ID:         resp.ID,
			DocumentID: resp.DocumentID,
			SpaceID:    resp.SpaceID,
			UserID:     resp.UserID,
			Username:   resp.Username,
			Action:     resp.Action,
			ActionName: resp.ActionName,
			Comment:    resp.Comment,
			CreateTime: resp.CreateTime,
		}
		if u, ok := userMap[log.UserID]; ok {
			item.Username = u.Username
			item.GivenName = u.GivenName
		}
		if d, ok := docMap[log.DocumentID]; ok {
			item.DocumentName = d.Name
		}
		if sp, ok := spaceMap[log.SpaceID]; ok {
			item.SpaceName = sp.Name
		}
		responses[i] = &item
	}

	return responses, total, nil
}

func (s *LogService) batchFetchLogRelations(logs []*adminmodel.DocumentLog) (
	userMap map[int]*usermodel.User,
	docMap map[int]*workspacemodel.Document,
	spaceMap map[int]*workspacemodel.Space,
) {
	userMap = make(map[int]*usermodel.User)
	docMap = make(map[int]*workspacemodel.Document)
	spaceMap = make(map[int]*workspacemodel.Space)

	userIDs := make([]int, 0)
	docIDs := make([]int, 0)
	spaceIDs := make([]int, 0)
	for _, log := range logs {
		if log.UserID > 0 {
			userIDs = append(userIDs, log.UserID)
		}
		if log.DocumentID > 0 {
			docIDs = append(docIDs, log.DocumentID)
		}
		if log.SpaceID > 0 {
			spaceIDs = append(spaceIDs, log.SpaceID)
		}
	}

	userIDs = utils.UniqueInts(userIDs)
	docIDs = utils.UniqueInts(docIDs)
	spaceIDs = utils.UniqueInts(spaceIDs)

	if len(userIDs) > 0 {
		users, _ := s.userRepo.FindByIDs(userIDs)
		for _, u := range users {
			userMap[u.ID] = u
		}
	}
	if len(docIDs) > 0 {
		docs, _ := s.docRepo.FindByIDs(docIDs)
		for _, d := range docs {
			docMap[d.ID] = d
		}
	}
	if len(spaceIDs) > 0 {
		spaces, _ := s.spaceRepo.FindByIDs(spaceIDs)
		for _, sp := range spaces {
			spaceMap[sp.ID] = sp
		}
	}
	return
}

func (s *LogService) LogSystemAction(level int, path, get, post, message, ip, userAgent, referer string, userID int, username string) error {
	post = maskPasswords(post)

	log := &adminmodel.SystemLog{
		Level:      level,
		Path:       path,
		GetParams:  get,
		PostParams: post,
		Message:    message,
		IP:         ip,
		UserAgent:  userAgent,
		Referer:    referer,
		UserID:     userID,
		Username:   username,
	}
	return s.logRepo.CreateSystemLog(log)
}

func maskPasswords(post string) string {
	if post == "" {
		return ""
	}

	if len(post) > 200 {
		post = post[:200] + "...(truncated)"
	}

	return utils.MaskSensitiveInfo(post)
}

type SystemLogResponse struct {
	LogID      int64  `json:"log_id"`           // P0 修复：对齐前端 SystemLog.log_id
	Level      string `json:"level"`            // P0 修复：对齐前端 SystemLog.level（string）
	Method     string `json:"method,omitempty"` // P0 修复：补齐前端期望字段
	Path       string `json:"path"`
	Message    string `json:"message"`
	IP         string `json:"ip"`
	UserAgent  string `json:"user_agent"`
	UserID     int    `json:"user_id"`
	Username   string `json:"username"`
	GivenName  string `json:"given_name,omitempty"`
	GetParams  string `json:"get_params,omitempty"`  // P0 修复：对齐前端 get_params
	PostParams string `json:"post_params,omitempty"` // P0 修复：对齐前端 post_params
	Referer    string `json:"referer,omitempty"`     // P0 修复：对齐前端 referer
	CreateTime int    `json:"create_time"`
}

func (s *LogService) GetSystemLogs(level, page, pageSize int) ([]*SystemLogResponse, int64, error) {
	offset := (page - 1) * pageSize

	logs, total, err := s.logRepo.GetSystemLogs(level, pageSize, offset)
	if err != nil {
		return nil, 0, err
	}

	// 批量解析用户名，消除循环内逐条 FindByID 的 N+1
	userIDs := make([]int, 0, len(logs))
	for _, l := range logs {
		if l.UserID > 0 {
			userIDs = append(userIDs, l.UserID)
		}
	}
	userMap := make(map[int]*usermodel.User)
	if len(userIDs) > 0 {
		if users, uerr := s.userRepo.FindByIDs(utils.UniqueInts(userIDs)); uerr == nil {
			for _, u := range users {
				userMap[u.ID] = u
			}
		}
	}

	responses := make([]*SystemLogResponse, len(logs))
	for i, l := range logs {
		resp := &SystemLogResponse{
			LogID:      l.LogID,
			Level:      adminmodel.SystemLogLevelName(l.Level),
			Method:     l.Method,
			Path:       l.Path,
			Message:    l.Message,
			IP:         l.IP,
			UserAgent:  l.UserAgent,
			UserID:     l.UserID,
			Username:   l.Username,
			GetParams:  l.GetParams,
			PostParams: l.PostParams,
			Referer:    l.Referer,
			CreateTime: l.CreateTime,
		}
		if l.UserID > 0 {
			if user, ok := userMap[l.UserID]; ok && user != nil {
				resp.GivenName = user.GivenName
			}
		}
		responses[i] = resp
	}

	return responses, total, nil
}

func (s *LogService) GetRecentErrorLogs(limit int) ([]*adminmodel.SystemLog, error) {
	return s.logRepo.GetRecentErrorLogs(limit)
}

func (s *LogService) GetLogDetail(logID int) (*adminmodel.SystemLog, error) {
	return s.logRepo.GetLogDetail(logID)
}
