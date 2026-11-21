package user

import (
	"crypto/md5" //nolint:gosec // G501: legacy md5 password compat
	"errors"

	usermodel "github.com/listen808/citebase/internal/model/user"
	userrepo "github.com/listen808/citebase/internal/repository/user"
	workspacerepo "github.com/listen808/citebase/internal/repository/workspace"

	"fmt"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/log"
	"golang.org/x/crypto/bcrypt"
)

var (
	ErrUserAlreadyExists = errors.New("username already exists")
	ErrInvalidPassword   = errors.New("invalid password")
	ErrUserNotFound      = userrepo.ErrUserNotFound
	ErrDocumentNotFound  = workspacerepo.ErrDocumentNotFound
	ErrSpaceNotFound     = workspacerepo.ErrSpaceNotFound
	ErrCannotFollowSelf  = errors.New("cannot follow yourself")
	ErrCannotOperateRoot = errors.New("cannot operate on super administrator")
	ErrRootCannotChange  = errors.New("root user role cannot be changed")
)

type UserService struct {
	userRepo *userrepo.UserRepository
	roleRepo *userrepo.RoleRepository
}

func NewUserService() *UserService {
	return &UserService{
		userRepo: userrepo.NewUserRepository(),
		roleRepo: userrepo.NewRoleRepository(),
	}
}

func (s *UserService) CreateUser(req *usermodel.CreateUserRequest) (*usermodel.User, error) {
	existingUser, _ := s.userRepo.FindByUsername(req.Username)
	if existingUser != nil {
		return nil, ErrUserAlreadyExists
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, err
	}

	user := &usermodel.User{
		Username:   req.Username,
		Password:   string(hashedPassword),
		GivenName:  req.GivenName,
		Email:      req.Email,
		Mobile:     req.Mobile,
		Department: req.Department,
		Position:   req.Position,
		RoleID:     req.RoleID,
		IsDelete:   0,
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}

	return user, nil
}

func (s *UserService) GetUserByID(id int) (*usermodel.UserResponse, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if user.IsDelete == 1 {
		return nil, userrepo.ErrUserNotFound
	}

	response := user.ToResponse()

	if role, err := s.roleRepo.FindRoleByID(user.RoleID); err == nil {
		response.RoleName = role.Name
	}

	return &response, nil
}

func (s *UserService) GetUserByUsername(username string) (*usermodel.UserResponse, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return nil, err
	}

	if user.IsDelete == 1 {
		return nil, userrepo.ErrUserNotFound
	}

	response := user.ToResponse()
	return &response, nil
}

func (s *UserService) FindByUsername(username string) (*usermodel.User, error) {
	return s.userRepo.FindByUsername(username)
}

func (s *UserService) UpdateUser(id int, req *usermodel.UpdateUserRequest) (*usermodel.User, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if user.RoleID == 1 {
		return nil, ErrCannotOperateRoot
	}

	if req.GivenName != "" {
		user.GivenName = req.GivenName
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Mobile != "" {
		user.Mobile = req.Mobile
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.Department != "" {
		user.Department = req.Department
	}
	if req.Position != "" {
		user.Position = req.Position
	}
	if req.Location != "" {
		user.Location = req.Location
	}
	if req.IM != "" {
		user.IM = req.IM
	}
	if req.RoleID > 0 {
		if req.RoleID == 1 {
			return nil, ErrRootCannotChange
		}
		user.RoleID = req.RoleID
	}

	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}

	return user, nil
}

// UpdateOwnProfile 普通用户自助更新个人资料
// 与 UpdateUser 的差异：
//   - 不校验 ErrCannotOperateRoot（根用户也应能修改自己的资料）
//   - 不允许修改 RoleID（防止权限提升）
func (s *UserService) UpdateOwnProfile(id int, req *usermodel.UpdateUserRequest) (*usermodel.User, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, err
	}

	if req.GivenName != "" {
		user.GivenName = req.GivenName
	}
	if req.Email != "" {
		user.Email = req.Email
	}
	if req.Mobile != "" {
		user.Mobile = req.Mobile
	}
	if req.Phone != "" {
		user.Phone = req.Phone
	}
	if req.Department != "" {
		user.Department = req.Department
	}
	if req.Position != "" {
		user.Position = req.Position
	}
	if req.Location != "" {
		user.Location = req.Location
	}
	if req.IM != "" {
		user.IM = req.IM
	}

	if err := s.userRepo.Update(user); err != nil {
		return nil, err
	}
	return user, nil
}

func (s *UserService) DeleteUser(id int) error {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return err
	}

	if user.RoleID == 1 {
		return ErrCannotOperateRoot
	}

	if err := s.userRepo.Delete(id); err != nil {
		return err
	}

	return nil
}

func (s *UserService) GetUserList(page, pageSize int, roleID int, keyword string) ([]*usermodel.UserResponse, int64, error) {
	offset := (page - 1) * pageSize

	users, total, err := s.userRepo.FindAll(offset, pageSize, roleID, keyword)
	if err != nil {
		return nil, 0, err
	}

	roleIDSet := make(map[int]bool)
	for _, u := range users {
		if u.RoleID > 0 {
			roleIDSet[u.RoleID] = true
		}
	}
	roleIDs := make([]int, 0, len(roleIDSet))
	for roleID := range roleIDSet {
		roleIDs = append(roleIDs, roleID)
	}

	roleMap := make(map[int]string)
	for _, rid := range roleIDs {
		if role, err := s.roleRepo.FindRoleByID(rid); err == nil {
			roleMap[rid] = role.Name
		}
	}

	responses := make([]*usermodel.UserResponse, len(users))
	for i, user := range users {
		roleName := roleMap[user.RoleID]
		resp := user.ToResponseWithRole(roleName)
		responses[i] = &resp
	}

	return responses, total, nil
}

func (s *UserService) ChangePassword(id int, oldPassword, newPassword string) error {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(oldPassword)); err != nil {
		return ErrInvalidPassword
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.userRepo.UpdatePassword(id, string(hashedPassword))
}

func (s *UserService) ResetPassword(id int, newPassword string) error {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return err
	}

	if user.RoleID == 1 {
		return ErrCannotOperateRoot
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.userRepo.UpdatePassword(id, string(hashedPassword))
}

func (s *UserService) ToggleUserForbidden(id int) error {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return err
	}

	if user.RoleID == 1 {
		return ErrCannotOperateRoot
	}

	newStatus := 0
	if user.IsForbidden == 0 {
		newStatus = 1
	}

	return s.userRepo.ToggleForbidden(id, newStatus)
}

func (s *UserService) AuthenticateUser(username, password string) (*usermodel.User, error) {
	user, err := s.userRepo.FindByUsername(username)
	if err != nil {
		return nil, err
	}

	if user.IsDelete == 1 {
		return nil, userrepo.ErrUserNotFound
	}

	if user.IsForbidden == 1 {
		return nil, errors.New("user is forbidden")
	}

	if !verifyPassword(user.Password, password) {
		return nil, ErrInvalidPassword
	}

	if isMD5Hash(user.Password) {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			log.Errorf("[ERROR] Failed to upgrade password from MD5 to bcrypt for user %d: %v", user.ID, err)
		} else {
			if err := s.userRepo.UpdatePassword(user.ID, string(hashedPassword)); err != nil {
				log.Errorf("[ERROR] Failed to save upgraded bcrypt password for user %d: %v", user.ID, err)
			}
		}
	}

	return user, nil
}

func isMD5Hash(password string) bool {
	if len(password) != 32 {
		return false
	}
	for _, c := range password {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func verifyPassword(hashedPassword, password string) bool {
	if strings.HasPrefix(hashedPassword, "$2") {
		return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password)) == nil
	}

	if isMD5Hash(hashedPassword) {
		//nolint:gosec // G501,G401: legacy md5 password compat — new users get bcrypt
		return hashedPassword == fmt.Sprintf("%x", md5.Sum([]byte(password)))
	}

	return false
}

func (s *UserService) UpdateLastLogin(userID int, clientIP string) error {
	return s.userRepo.UpdateLastLogin(userID, clientIP)
}

func (s *UserService) GetAllUsers() ([]*usermodel.User, error) {
	return s.userRepo.FindAllForExport()
}

func (s *UserService) GetAllUsersBasic() ([]map[string]interface{}, error) {
	users, err := s.userRepo.FindAllForExport()
	if err != nil {
		return nil, err
	}
	result := make([]map[string]interface{}, 0, len(users))
	for _, u := range users {
		result = append(result, map[string]interface{}{
			"user_id":    u.ID,
			"username":   u.Username,
			"given_name": u.GivenName,
			"email":      u.Email,
			"mobile":     u.Mobile,
			"position":   u.Position,
			"department": u.Department,
		})
	}
	return result, nil
}

// SavePreferences 校验并保存当前用户的个性化偏好，返回服务端盖写的更新时间。
//
// 为什么服务端也要校验而不是只存：这一列会被另一台设备读回去直接生效。
// 写进去的脏值（拼错的外观名、solid 配 clear）在写入那台设备上可能"看着没事"——
// 前端自己会兜底 —— 换设备时才暴露，而那时已经查不到是谁写的了。
// `updated_at` 由服务端盖写：多设备同步靠它判"哪份更新"，那个时钟不能放在客户端手里。
func (s *UserService) SavePreferences(id int, prefs *usermodel.Preferences) (int64, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return 0, err
	}
	if user == nil {
		return 0, ErrUserNotFound
	}
	now := time.Now().Unix()
	prefs.UpdatedAt = now
	raw, err := usermodel.MarshalPreferences(prefs)
	if err != nil {
		return 0, err
	}
	if err := s.userRepo.UpdatePreferences(id, raw, int(now)); err != nil {
		return 0, err
	}
	return now, nil
}

// GetPreferences 读回偏好；从没同步过返回 (nil, nil)。
// 返回 nil 而不是零值结构体是关键判据：前端要能区分"账号上没配过"（沿用本地）
// 与"账号上配的就是浅色实心"（照它切）。合成一个默认值就等于每次登录都把本地选择判给服务器。
func (s *UserService) GetPreferences(id int) (*usermodel.Preferences, error) {
	user, err := s.userRepo.FindByID(id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, nil
	}
	return usermodel.ParsePreferences(user.Preferences)
}
