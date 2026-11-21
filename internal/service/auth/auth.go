package auth

import (
	"errors"
	jwt "github.com/listen808/citebase/internal/middleware/jwt"
	usermodel "github.com/listen808/citebase/internal/model/user"
	usersvc "github.com/listen808/citebase/internal/service/user"
	"time"

	"github.com/listen808/citebase/internal/utils/safego"
	"sync"
)

var (
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrAccountLocked      = errors.New("account locked") // 句子在 internal/response/messages.go，这里只管日志
)

type loginAttempt struct {
	mu          sync.Mutex
	count       int
	lastAttempt time.Time
	lockedUntil time.Time
}

var (
	loginAttempts sync.Map
	maxAttempts   = 5
	lockDuration  = 15 * time.Minute
)

// recordFailedLogin 累加某次「用户名+来源IP」的失败计数，达到阈值即锁定。
// 首次失败也要计数：旧实现只在 LoadOrStore 命中已存在条目时才 +1，等于阈值永远多一次。
func recordFailedLogin(key string) {
	attempt, _ := loginAttempts.LoadOrStore(key, &loginAttempt{lastAttempt: time.Now()})
	a := attempt.(*loginAttempt)
	a.mu.Lock()
	defer a.mu.Unlock()
	if !a.lastAttempt.IsZero() && time.Since(a.lastAttempt) > loginAttemptsTTL {
		// 太久没有活动，视为新一轮尝试
		a.count = 0
		a.lockedUntil = time.Time{}
	}
	a.count++
	a.lastAttempt = time.Now()
	if a.count >= maxAttempts {
		a.lockedUntil = time.Now().Add(lockDuration)
	}
}

var tokenBlacklist sync.Map

const (
	// tokenBlacklistTTL: JWT access token 2 小时有效期 + 5 分钟缓冲
	tokenBlacklistTTL = 125 * time.Minute
	// loginAttemptsTTL: 远超 15 分钟锁定时长，确保解锁后清理
	loginAttemptsTTL = 1 * time.Hour
	// cleanupInterval: 清理扫描周期
	cleanupInterval = 10 * time.Minute
)

func init() {
	// P0-14: 后台定期清理过期的 tokenBlacklist 和 loginAttempts
	// 避免 sync.Map 无限累积导致内存泄漏
	go func() {
		ticker := time.NewTicker(cleanupInterval)
		defer ticker.Stop()
		for range ticker.C {
			cleanupExpiredBlacklist()
			cleanupExpiredLoginAttempts()
		}
	}()
}

// cleanupExpiredBlacklist 清理 tokenBlacklist 中超过 TTL 的条目
func cleanupExpiredBlacklist() {
	now := time.Now()
	tokenBlacklist.Range(func(k, v interface{}) bool {
		storedAt, ok := v.(time.Time)
		if !ok {
			tokenBlacklist.Delete(k)
			return true
		}
		if now.Sub(storedAt) > tokenBlacklistTTL {
			tokenBlacklist.Delete(k)
		}
		return true
	})
}

// cleanupExpiredLoginAttempts 清理 loginAttempts 中已解锁且超过 TTL 的条目
func cleanupExpiredLoginAttempts() {
	now := time.Now()
	loginAttempts.Range(func(k, v interface{}) bool {
		a, ok := v.(*loginAttempt)
		if !ok {
			loginAttempts.Delete(k)
			return true
		}
		a.mu.Lock()
		locked := now.Before(a.lockedUntil)
		last := a.lastAttempt
		a.mu.Unlock()
		// 已解锁且最后一次尝试距今超过 TTL 才清理
		if !locked && !last.IsZero() && now.Sub(last) > loginAttemptsTTL {
			loginAttempts.Delete(k)
		}
		return true
	})
}

type AuthService struct {
	userService *usersvc.UserService
}

func NewAuthService() *AuthService {
	return &AuthService{
		userService: usersvc.NewUserService(),
	}
}

type LoginRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type LoginResponse struct {
	AccessToken  string   `json:"access_token"`
	RefreshToken string   `json:"refresh_token"`
	ExpiresIn    int      `json:"expires_in"`
	UserInfo     UserInfo `json:"user_info"`
}

type UserInfo struct {
	UserID     int    `json:"id"` // P0 修复：对齐前端 User.id（原为 user_id，前端 LoginResponse.user_info 无兜底转换）
	Username   string `json:"username"`
	GivenName  string `json:"given_name"`
	RoleID     int    `json:"role_id"`
	Email      string `json:"email"`
	Mobile     string `json:"mobile"`
	Phone      string `json:"phone"`
	Department string `json:"department"`
	Position   string `json:"position"`
	Location   string `json:"location"`
	IM         string `json:"im"`
	LastIP     string `json:"last_ip"`
	LastTime   int    `json:"last_time"`
	CreateTime int    `json:"create_time"`
	// Preferences 是账号级个性化偏好；nil = 这个账号从没同步过。
	// **必须是可选**：前端拿它决定"沿用本地还是照服务器切"，
	// 后端把空值合成成默认浅色实心，就等于每次登录都把用户在自己机器上调好的档位盖掉。
	Preferences *usermodel.Preferences `json:"preferences,omitempty"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type RefreshTokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in"`
}

func (s *AuthService) Login(username, password, clientIP string) (*LoginResponse, error) {
	key := username + ":" + clientIP
	if attempt, ok := loginAttempts.Load(key); ok {
		a := attempt.(*loginAttempt)
		a.mu.Lock()
		locked := time.Now().Before(a.lockedUntil)
		if !locked && !a.lockedUntil.IsZero() {
			// 锁定期已过：重置计数，给一次新的窗口
			a.count = 0
			a.lockedUntil = time.Time{}
		}
		a.mu.Unlock()
		if locked {
			return nil, ErrAccountLocked
		}
		// 注意：此处绝不能 Delete —— 未锁定时删掉计数会让每次登录都从 0 开始，
		// 锁定阈值永远达不到（历史实现即如此，导致暴破防护形同虚设）。
	}

	user, err := s.userService.AuthenticateUser(username, password)
	if err != nil {
		if err == usersvc.ErrUserNotFound || err == usersvc.ErrInvalidPassword {
			recordFailedLogin(key)
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}

	loginAttempts.Delete(key)

	accessToken, err := jwt.GenerateToken(user.ID, user.Username, user.RoleID)
	if err != nil {
		return nil, err
	}

	refreshToken, err := jwt.GenerateRefreshToken(user.ID)
	if err != nil {
		return nil, err
	}

	safego.Go("UpdateLastLogin", func() { _ = s.userService.UpdateLastLogin(user.ID, clientIP) })

	return &LoginResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    7200,
		UserInfo: UserInfo{
			UserID:    user.ID,
			Username:  user.Username,
			GivenName: user.GivenName,
			RoleID:    user.RoleID,
		},
	}, nil
}

func (s *AuthService) RefreshToken(refreshToken string) (*RefreshTokenResponse, error) {
	userID, remaining, err := jwt.ParseRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}

	user, err := s.userService.GetUserByID(userID)
	if err != nil {
		return nil, err
	}

	if user.IsForbidden == 1 {
		return nil, errors.New("用户已被禁用")
	}
	if user.IsDelete == 1 {
		return nil, errors.New("用户已被删除")
	}

	accessToken, err := jwt.GenerateToken(user.ID, user.Username, user.RoleID)
	if err != nil {
		return nil, err
	}

	resp := &RefreshTokenResponse{
		AccessToken: accessToken,
		ExpiresIn:   7200,
	}

	if remaining < 24*time.Hour {
		newRefreshToken, err := jwt.GenerateRefreshToken(user.ID, true)
		if err != nil {
			return nil, err
		}
		resp.RefreshToken = newRefreshToken
	}

	return resp, nil
}

func (s *AuthService) Logout(token string) error {
	tokenBlacklist.Store(token, time.Now())
	return nil
}

func IsTokenBlacklisted(token string) bool {
	_, ok := tokenBlacklist.Load(token)
	return ok
}

func (s *AuthService) FindOrCreateSSOUser(username string, ssoID int) (*usermodel.User, error) {
	user, err := s.userService.FindByUsername(username)
	if err == nil && user.IsDelete != 1 {
		return user, nil
	}

	req := &usermodel.CreateUserRequest{
		Username:  username,
		Password:  "",
		GivenName: username,
	}
	newUser, err := s.userService.CreateUser(req)
	if err != nil {
		return nil, err
	}
	return newUser, nil
}

func (s *AuthService) GetFullUserInfo(userID int) (*UserInfo, error) {
	userResp, err := s.userService.GetUserByID(userID)
	if err != nil {
		return nil, err
	}
	info := &UserInfo{
		UserID:     userResp.ID,
		Username:   userResp.Username,
		GivenName:  userResp.GivenName,
		RoleID:     userResp.RoleID,
		Email:      userResp.Email,
		Mobile:     userResp.Mobile,
		Phone:      userResp.Phone,
		Department: userResp.Department,
		Position:   userResp.Position,
		Location:   userResp.Location,
		IM:         userResp.IM,
		LastIP:     userResp.LastIP,
		LastTime:   userResp.LastTime,
		CreateTime: userResp.CreateTime,
	}
	// 偏好单独取一趟：资料面（UserResponse）上那一列是 json:"-"，
	// 因为管理员的用户列表不该带着每个人的界面配置。
	// 读失败就留 nil 而不报错 —— 这次没同步上，比整个 /auth/me 变成 500 好得多。
	if prefs, err := s.userService.GetPreferences(userID); err == nil {
		info.Preferences = prefs
	}
	return info, nil
}

// UpdateOwnProfile 当前用户更新自己的资料（不允许修改 RoleID）
func (s *AuthService) UpdateOwnProfile(userID int, req *usermodel.UpdateUserRequest) (*UserInfo, error) {
	user, err := s.userService.UpdateOwnProfile(userID, req)
	if err != nil {
		return nil, err
	}
	return &UserInfo{
		UserID:     user.ID,
		Username:   user.Username,
		GivenName:  user.GivenName,
		RoleID:     user.RoleID,
		Email:      user.Email,
		Mobile:     user.Mobile,
		Phone:      user.Phone,
		Department: user.Department,
		Position:   user.Position,
		Location:   user.Location,
		IM:         user.IM,
		LastIP:     user.LastIP,
		LastTime:   user.LastTime,
		CreateTime: user.CreateTime,
	}, nil
}

// ChangeOwnPassword 当前用户修改自己的密码（需校验旧密码）
func (s *AuthService) ChangeOwnPassword(userID int, oldPassword, newPassword string) error {
	return s.userService.ChangePassword(userID, oldPassword, newPassword)
}

// SaveOwnPreferences 保存当前用户的个性化偏好并回一份完整的 UserInfo。
//
// 回整份而不是只回 updated_at：前端 PUT 之后要把它写进 user store，
// 否则下一次 /auth/me 之前的任何读取都还是旧偏好，"另一台设备"看到的顺序会依赖竞态。
func (s *AuthService) SaveOwnPreferences(userID int, prefs *usermodel.Preferences) (*UserInfo, error) {
	if _, err := s.userService.SavePreferences(userID, prefs); err != nil {
		return nil, err
	}
	return s.GetFullUserInfo(userID)
}
