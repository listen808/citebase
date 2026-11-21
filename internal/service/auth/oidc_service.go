package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	logx "github.com/listen808/citebase/internal/log"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	usermodel "github.com/listen808/citebase/internal/model/user"
	authrepo "github.com/listen808/citebase/internal/repository/auth"
	userrepo "github.com/listen808/citebase/internal/repository/user"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

var (
	ErrProviderNotFound    = errors.New("OIDC provider not found")
	ErrProviderNotEnabled  = errors.New("OIDC provider not enabled")
	ErrOIDCInitFailed      = errors.New("OIDC provider initialization failed")
	ErrTokenExchangeFailed = errors.New("OIDC token exchange failed")
	ErrUserInfoFailed      = errors.New("failed to get user info from OIDC provider")
	ErrAutoCreateDisabled  = errors.New("user not found and auto-create is disabled")
)

// oidcProviderState holds the initialized OIDC provider and its OAuth2 config
type oidcProviderState struct {
	provider    *oidc.Provider
	oauth2Cfg   *oauth2.Config
	verifier    *oidc.IDTokenVerifier
	providerCfg *adminmodel.OIDCProvider
}

// OIDCService OIDC 认证服务
type OIDCService struct {
	providerRepo *authrepo.OIDCProviderRepository
	mappingRepo  *authrepo.OIDCUserMappingRepository
	userRepo     *userrepo.UserRepository

	mu        sync.RWMutex
	providers map[uint]*oidcProviderState // keyed by provider ID
	// initErrors 记录"已启用但初始化失败"的原因。
	// 建/改 provider 时初始化失败只 log warning 并照样返回成功，管理员于是会以为
	// 配好了，而登录页根本不出现入口、点了报 500——这个失败必须能被管理端读到。
	initErrors map[uint]string
}

func NewOIDCService() *OIDCService {
	return &OIDCService{
		providerRepo: authrepo.NewOIDCProviderRepository(),
		mappingRepo:  authrepo.NewOIDCUserMappingRepository(),
		userRepo:     userrepo.NewUserRepository(),
		providers:    make(map[uint]*oidcProviderState),
		initErrors:   make(map[uint]string),
	}
}

// InitError 返回该 provider 最近一次初始化失败的原因；成功或从未尝试则为空串。
func (s *OIDCService) InitError(id uint) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.initErrors[id]
}

// InitAllProviders initializes all enabled OIDC providers from the database
func (s *OIDCService) InitAllProviders() {
	providers, err := s.providerRepo.FindEnabled()
	if err != nil {
		logx.Warnf("[OIDC] Failed to load enabled providers: %v", err)
		return
	}
	for _, p := range providers {
		if err := s.initProvider(p); err != nil {
			logx.Warnf("[OIDC] Failed to init provider %q (id=%d): %v", p.Name, p.ID, err)
		}
	}
}

// initProvider initializes a single OIDC provider using discovery or manual endpoints
//
// 命名返回值 + defer：失败原因统一在这里记账，成功就清除。函数体里有多个返回错误的
// 分支，逐个手写记录会漏；调用方（CreateProvider/UpdateProvider/InitAllProviders）
// 依旧只看返回的 error，行为不变。
func (s *OIDCService) initProvider(cfg *adminmodel.OIDCProvider) (err error) {
	defer func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if err != nil {
			s.initErrors[cfg.ID] = err.Error()
		} else {
			delete(s.initErrors, cfg.ID)
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	var scopes []string
	if cfg.Scopes != "" {
		scopes = strings.Split(cfg.Scopes, ",")
	} else {
		scopes = []string{oidc.ScopeOpenID, "profile", "email"}
	}

	// Build the callback URL placeholder (will be set dynamically per request)
	oauth2Cfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		Scopes:       scopes,
	}

	var provider *oidc.Provider

	if cfg.IssuerURL != "" {
		// Use OIDC discovery
		var discErr error
		provider, discErr = oidc.NewProvider(ctx, cfg.IssuerURL)
		if discErr != nil {
			return fmt.Errorf("OIDC discovery failed for %s: %w", cfg.IssuerURL, discErr)
		}

		endpoint := provider.Endpoint()
		oauth2Cfg.Endpoint = endpoint
	} else {
		// Manual endpoint configuration
		if cfg.AuthURL == "" || cfg.TokenURL == "" {
			return fmt.Errorf("issuer_url or auth_url+token_url required for provider %s", cfg.Name)
		}
		oauth2Cfg.Endpoint = oauth2.Endpoint{
			AuthURL:  cfg.AuthURL,
			TokenURL: cfg.TokenURL,
		}
	}

	// Create ID token verifier
	var verifier *oidc.IDTokenVerifier
	if provider != nil {
		verifier = provider.Verifier(&oidc.Config{
			ClientID: cfg.ClientID,
		})
	}

	s.mu.Lock()
	s.providers[cfg.ID] = &oidcProviderState{
		provider:    provider,
		oauth2Cfg:   oauth2Cfg,
		verifier:    verifier,
		providerCfg: cfg,
	}
	s.mu.Unlock()

	logx.Infof("[OIDC] Provider %q (id=%d, type=%s) initialized successfully", cfg.Name, cfg.ID, cfg.Type)
	return nil
}

// GetEnabledProviders returns a list of enabled providers for the login page
func (s *OIDCService) GetEnabledProviders() ([]adminmodel.OIDCProviderPublicInfo, error) {
	providers, err := s.providerRepo.FindEnabled()
	if err != nil {
		return nil, err
	}
	result := make([]adminmodel.OIDCProviderPublicInfo, len(providers))
	for i, p := range providers {
		result[i] = p.ToPublicInfo()
	}
	return result, nil
}

// GetAuthURL generates the authorization URL for the given provider
func (s *OIDCService) GetAuthURL(providerID uint, redirectURI, state, nonce string) (string, error) {
	s.mu.RLock()
	state_, ok := s.providers[providerID]
	s.mu.RUnlock()

	if !ok {
		// Try to initialize on demand
		cfg, err := s.providerRepo.FindByID(providerID)
		if err != nil {
			return "", ErrProviderNotFound
		}
		if !cfg.Enabled {
			return "", ErrProviderNotEnabled
		}
		if err := s.initProvider(cfg); err != nil {
			return "", fmt.Errorf("%w: %v", ErrOIDCInitFailed, err)
		}
		s.mu.RLock()
		state_, ok = s.providers[providerID]
		s.mu.RUnlock()
		if !ok {
			return "", ErrOIDCInitFailed
		}
	}

	// 复制配置后再改 RedirectURL：缓存里的 *oauth2.Config 是跨请求共享对象，
	// 直接赋值会让并发登录互相污染对方的 redirect_uri。
	cfg := *state_.oauth2Cfg
	cfg.RedirectURL = redirectURI

	authOpts := []oauth2.AuthCodeOption{}
	if nonce != "" {
		authOpts = append(authOpts, oauth2.SetAuthURLParam("nonce", nonce))
	}

	url := cfg.AuthCodeURL(state, authOpts...)
	return url, nil
}

// OIDCCallbackResult holds the result of a successful OIDC callback
type OIDCCallbackResult struct {
	User         *usermodel.User
	AccessToken  string
	RefreshToken string
	ExpiresIn    int
	UserInfo     OIDCUserInfo
}

// OIDCUserInfo basic user info extracted from OIDC token
type OIDCUserInfo struct {
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	GivenName string `json:"given_name"`
	Email     string `json:"email"`
	RoleID    int    `json:"role_id"`
}

// HandleCallback processes the OIDC callback, exchanges the code for tokens, and finds/creates the user.
// expectedNonce 来自服务端签发并登记的一次性 state（见 IssueOIDCState）；
// redirectURI 必须与发起授权时登记的完全一致，否则令牌换取会被 IdP 拒绝。
func (s *OIDCService) HandleCallback(providerID uint, code, redirectURI, expectedNonce string) (*OIDCCallbackResult, error) {
	s.mu.RLock()
	state, ok := s.providers[providerID]
	s.mu.RUnlock()

	if !ok {
		// Try to init on demand
		cfg, err := s.providerRepo.FindByID(providerID)
		if err != nil {
			return nil, ErrProviderNotFound
		}
		if err := s.initProvider(cfg); err != nil {
			return nil, fmt.Errorf("%w: %v", ErrOIDCInitFailed, err)
		}
		s.mu.RLock()
		state, ok = s.providers[providerID]
		s.mu.RUnlock()
		if !ok {
			return nil, ErrOIDCInitFailed
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 复制共享配置，避免并发登录互相改写 RedirectURL
	oauth2Cfg := *state.oauth2Cfg
	oauth2Cfg.RedirectURL = redirectURI

	// Exchange authorization code for token
	token, err := oauth2Cfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrTokenExchangeFailed, err)
	}

	// Extract claims from the ID token or UserInfo endpoint
	claims, err := s.extractClaims(ctx, state, token)
	if err != nil {
		return nil, err
	}
	// nonce 校验：签发过 nonce 就必须原样回到 ID Token 中，否则说明这份断言不是发给本次会话的
	if expectedNonce != "" {
		if !claims.fromIDToken {
			return nil, fmt.Errorf("无法校验 nonce（未获得可验证的 ID Token），已拒绝登录")
		}
		if claims.Nonce != expectedNonce {
			return nil, fmt.Errorf("nonce 不匹配，已拒绝登录")
		}
	}

	// Find or create user
	user, err := s.findOrCreateUser(state.providerCfg, claims)
	if err != nil {
		return nil, err
	}

	return &OIDCCallbackResult{
		User: user,
		UserInfo: OIDCUserInfo{
			UserID:    user.ID,
			Username:  user.Username,
			GivenName: user.GivenName,
			Email:     user.Email,
			RoleID:    user.RoleID,
		},
	}, nil
}

// oidcClaims holds the claims extracted from the OIDC token
type oidcClaims struct {
	Subject           string `json:"sub"`
	Email             string `json:"email"`
	EmailVerified     bool   `json:"email_verified"`
	Name              string `json:"name"`
	PreferredUsername string `json:"preferred_username"`
	GivenName         string `json:"given_name"`
	FamilyName        string `json:"family_name"`
	Picture           string `json:"picture"`
	Nonce             string `json:"nonce"`

	// fromIDToken 标记断言是否来自通过签名校验的 ID Token（UserInfo 回退路径拿不到可信 nonce）
	fromIDToken bool
}

// extractClaims extracts user claims from the OIDC ID token or UserInfo endpoint
func (s *OIDCService) extractClaims(ctx context.Context, state *oidcProviderState, token *oauth2.Token) (*oidcClaims, error) {
	claims := &oidcClaims{}

	// Try to extract from ID token first
	rawIDToken, ok := token.Extra("id_token").(string)
	if ok && rawIDToken != "" && state.verifier != nil {
		idToken, err := state.verifier.Verify(ctx, rawIDToken)
		if err != nil {
			logx.Warnf("[OIDC] ID token verification failed: %v, falling back to UserInfo", err)
		} else {
			if err := idToken.Claims(claims); err != nil {
				logx.Warnf("[OIDC] Failed to parse ID token claims: %v, falling back to UserInfo", err)
			} else if claims.Subject != "" {
				claims.fromIDToken = true
				return claims, nil
			}
		}
	}

	// Fallback: use UserInfo endpoint
	if state.provider != nil {
		userInfo, err := state.provider.UserInfo(ctx, oauth2.StaticTokenSource(token))
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrUserInfoFailed, err)
		}
		claims.Subject = userInfo.Subject
		claims.Email = userInfo.Email
		claims.EmailVerified = userInfo.EmailVerified
		if err := userInfo.Claims(claims); err != nil {
			logx.Warnf("[OIDC] Failed to parse UserInfo claims: %v", err)
		}
	}

	if claims.Subject == "" {
		return nil, fmt.Errorf("OIDC provider did not return a subject identifier")
	}

	return claims, nil
}

// findOrCreateUser finds an existing user by OIDC mapping or creates a new one
func (s *OIDCService) findOrCreateUser(providerCfg *adminmodel.OIDCProvider, claims *oidcClaims) (*usermodel.User, error) {
	// 1. Look up existing mapping by subject
	mapping, err := s.mappingRepo.FindByProviderAndSubject(providerCfg.ID, claims.Subject)
	if err == nil && mapping != nil {
		// Found existing mapping, look up user
		user, err := s.userRepo.FindByID(mapping.UserID)
		if err == nil && user != nil && user.IsDelete == 0 {
			// Update raw info
			rawJSON, _ := json.Marshal(claims)
			mapping.RawInfo = string(rawJSON)
			if claims.Email != "" {
				mapping.Email = claims.Email
			}
			// 刷新 RawInfo 失败不影响这次登录（身份已经认出、用户已经取到），
			// 但静默丢掉的话，IdP 侧改了资料而本地映射一直是旧的，没人看得出来。
			if uerr := s.mappingRepo.Update(mapping); uerr != nil {
				logx.Warnf("[OIDC] 刷新 provider %d 的映射 %d 失败: %v", providerCfg.ID, mapping.ID, uerr)
			}
			// Sync user info from IdP
			s.syncUserInfo(user, claims)
			return user, nil
		}
	}

	// 2. Try to match by email if available
	// 只有 IdP 明确告知邮箱已验证时才允许按邮箱关联既有账号：否则任何人都能在 IdP 里
	// 把邮箱改成他人地址，从而在首次登录时接管那个本地账号。
	if claims.Email != "" && claims.EmailVerified {
		mapping, err := s.mappingRepo.FindByProviderAndEmail(providerCfg.ID, claims.Email)
		if err == nil && mapping != nil {
			user, err := s.userRepo.FindByID(mapping.UserID)
			if err == nil && user != nil && user.IsDelete == 0 {
				// Update mapping with subject
				mapping.Subject = claims.Subject
				rawJSON, _ := json.Marshal(claims)
				mapping.RawInfo = string(rawJSON)
				// 这次写失败不阻断登录（邮箱那一路下次还能认出来），但必须留下痕迹：
				// 补不上 subject 的话，第 1 步「按 subject 找」永远落空，
				// 每次登录都要多绕一趟邮箱查询，而且邮箱一旦不再 verified 就彻底认不出。
				if uerr := s.mappingRepo.Update(mapping); uerr != nil {
					logx.Warnf("[OIDC] 为映射 %d 补 subject %q 失败: %v", mapping.ID, claims.Subject, uerr)
				}
				s.syncUserInfo(user, claims)
				return user, nil
			}
		}

		// Try to find user by email in the user table
		user, err := s.userRepo.FindByEmail(claims.Email)
		if err == nil && user != nil && user.IsDelete == 0 {
			// Create mapping for existing user
			rawJSON, _ := json.Marshal(claims)
			newMapping := &adminmodel.OIDCUserMapping{
				UserID:     user.ID,
				ProviderID: providerCfg.ID,
				Subject:    claims.Subject,
				Email:      claims.Email,
				RawInfo:    string(rawJSON),
			}
			if err := s.mappingRepo.Create(newMapping); err != nil {
				return nil, fmt.Errorf("failed to create OIDC mapping: %w", err)
			}
			s.syncUserInfo(user, claims)
			return user, nil
		}
	}

	// 3. Auto-create user if enabled
	if !providerCfg.AutoCreate {
		return nil, ErrAutoCreateDisabled
	}

	username := s.buildUsername(claims)
	givenName := claims.GivenName
	if givenName == "" {
		givenName = claims.Name
	}
	if givenName == "" {
		givenName = username
	}

	roleID := s.resolveRoleID(providerCfg.DefaultRole)

	user := &usermodel.User{
		Username:   username,
		GivenName:  givenName,
		Email:      claims.Email,
		RoleID:     roleID,
		CreateTime: int(time.Now().Unix()),
		UpdateTime: int(time.Now().Unix()),
	}

	if err := s.userRepo.Create(user); err != nil {
		return nil, fmt.Errorf("failed to create user from OIDC: %w", err)
	}

	// Create OIDC mapping
	rawJSON, _ := json.Marshal(claims)
	newMapping := &adminmodel.OIDCUserMapping{
		UserID:     user.ID,
		ProviderID: providerCfg.ID,
		Subject:    claims.Subject,
		Email:      claims.Email,
		RawInfo:    string(rawJSON),
	}
	if err := s.mappingRepo.Create(newMapping); err != nil {
		// 这一步不能只记日志：用户行已经写进去了，映射却没建成 ——
		// 于是这个本地账号和 IdP 身份之间没有任何绑定，下次登录第 1 步（按 subject）
		// 必然落空，只能靠「邮箱已验证」那条路兜回来；邮箱为空或未验证时，
		// 第 3 步会再建一个同名账号并被唯一键拒掉，从此这个 sub 再也登不进来。
		// 宁可让这次登录明确失败，也不要把一个孤儿账号留在库里装作登录成功。
		logx.Errorf("[OIDC] 自动创建的用户 %d 映射写入失败（成为孤儿账号，需人工处理）: %v", user.ID, err)
		return nil, fmt.Errorf("failed to create OIDC mapping for auto-created user %d: %w", user.ID, err)
	}

	logx.Infof("[OIDC] Auto-created user %q (id=%d) from provider %q", username, user.ID, providerCfg.Name)
	return user, nil
}

// buildUsername builds a unique username from OIDC claims
func (s *OIDCService) buildUsername(claims *oidcClaims) string {
	// Prefer preferred_username, then email prefix, then subject
	if claims.PreferredUsername != "" {
		return claims.PreferredUsername
	}
	if claims.Email != "" {
		parts := strings.SplitN(claims.Email, "@", 2)
		if len(parts) > 0 {
			return parts[0]
		}
	}
	// Use a prefix + subject as fallback
	return "oidc_" + claims.Subject
}

// resolveRoleID converts a role name to a role ID
//
// "admin" 必须落 role 2（管理员），不是 role 1。role 1 在 aw_role 里的描述是
// 「系统内置超级管理员，拥有所有权限」，而 AutoRBAC 认到 role 1 就直接 c.Next()
// 跳过全部权限判定 —— 等于 IdP 里给某个 IdP 填了 default_role=admin，
// 从这个 IdP 首次登录的人全都拿到超级管理员。管理页那个下拉写的也是「管理员（admin）」，
// 对应的是 role 2 那一档。
func (s *OIDCService) resolveRoleID(roleName string) int {
	switch roleName {
	case "admin":
		return 2
	case "editor":
		return 3 // 编辑者：本系统没有独立的 editor 角色，落到"能写文档的普通用户"这一档
	default:
		return 3 // viewer
	}
}

// syncUserInfo updates user info from OIDC claims if changed
func (s *OIDCService) syncUserInfo(user *usermodel.User, claims *oidcClaims) {
	changed := false

	if claims.GivenName != "" && user.GivenName != claims.GivenName {
		user.GivenName = claims.GivenName
		changed = true
	}
	if claims.Email != "" && user.Email != claims.Email {
		user.Email = claims.Email
		changed = true
	}

	if changed {
		user.UpdateTime = int(time.Now().Unix())
		// 同步 IdP 侧改过的姓名/邮箱失败不影响登录，但必须留痕：
		// 静默丢掉的话本地档案会一直停在旧值，而用户以为自己改过了。
		if uerr := s.userRepo.Update(user); uerr != nil {
			logx.Warnf("[OIDC] 同步用户 %d 的资料失败: %v", user.ID, uerr)
		}
	}
}

// --- Admin CRUD operations ---

// CreateProvider creates a new OIDC provider and initializes it if enabled
func (s *OIDCService) CreateProvider(req *adminmodel.CreateOIDCProviderRequest) (*adminmodel.OIDCProvider, error) {
	provider := &adminmodel.OIDCProvider{
		Name:         req.Name,
		DisplayName:  req.DisplayName,
		Type:         req.Type,
		ClientID:     req.ClientID,
		ClientSecret: req.ClientSecret,
		IssuerURL:    req.IssuerURL,
		AuthURL:      req.AuthURL,
		TokenURL:     req.TokenURL,
		UserInfoURL:  req.UserInfoURL,
		Scopes:       req.Scopes,
		AutoCreate:   true,
		DefaultRole:  "viewer",
		IconURL:      req.IconURL,
		SortOrder:    req.SortOrder,
		Enabled:      true,
	}

	if req.Scopes == "" {
		provider.Scopes = "openid,profile,email"
	}
	if req.Enabled != nil {
		provider.Enabled = *req.Enabled
	}
	if req.AutoCreate != nil {
		provider.AutoCreate = *req.AutoCreate
	}
	if req.DefaultRole != "" {
		provider.DefaultRole = req.DefaultRole
	}

	if err := s.providerRepo.Create(provider); err != nil {
		return nil, err
	}

	// Initialize if enabled
	if provider.Enabled {
		if err := s.initProvider(provider); err != nil {
			logx.Warnf("[OIDC] Provider %q created but initialization failed: %v", provider.Name, err)
		}
	}

	return provider, nil
}

// UpdateProvider updates an existing OIDC provider
func (s *OIDCService) UpdateProvider(id uint, req *adminmodel.UpdateOIDCProviderRequest) (*adminmodel.OIDCProvider, error) {
	provider, err := s.providerRepo.FindByID(id)
	if err != nil {
		return nil, ErrProviderNotFound
	}

	provider.Name = req.Name
	provider.DisplayName = req.DisplayName
	provider.Type = req.Type
	provider.ClientID = req.ClientID
	if req.ClientSecret != "" {
		provider.ClientSecret = req.ClientSecret
	}
	provider.IssuerURL = req.IssuerURL
	provider.AuthURL = req.AuthURL
	provider.TokenURL = req.TokenURL
	provider.UserInfoURL = req.UserInfoURL
	provider.Scopes = req.Scopes
	if req.Scopes == "" {
		provider.Scopes = "openid,profile,email"
	}
	if req.Enabled != nil {
		provider.Enabled = *req.Enabled
	}
	if req.AutoCreate != nil {
		provider.AutoCreate = *req.AutoCreate
	}
	if req.DefaultRole != "" {
		provider.DefaultRole = req.DefaultRole
	}
	provider.IconURL = req.IconURL
	provider.SortOrder = req.SortOrder

	if err := s.providerRepo.Update(provider); err != nil {
		return nil, err
	}

	// Re-initialize if enabled, remove from cache if disabled
	s.mu.Lock()
	delete(s.providers, id)
	delete(s.initErrors, id)
	s.mu.Unlock()

	if provider.Enabled {
		if err := s.initProvider(provider); err != nil {
			logx.Warnf("[OIDC] Provider %q updated but re-initialization failed: %v", provider.Name, err)
		}
	}

	return provider, nil
}

// DeleteProvider deletes an OIDC provider and cleans up its cache
func (s *OIDCService) DeleteProvider(id uint) error {
	if err := s.providerRepo.Delete(id); err != nil {
		return err
	}
	s.mu.Lock()
	delete(s.providers, id)
	delete(s.initErrors, id)
	s.mu.Unlock()
	return nil
}

// GetProvider returns a provider by ID
func (s *OIDCService) GetProvider(id uint) (*adminmodel.OIDCProvider, error) {
	return s.providerRepo.FindByID(id)
}

// GetAllProviders returns all providers (admin)
func (s *OIDCService) GetAllProviders() ([]*adminmodel.OIDCProvider, error) {
	return s.providerRepo.FindAll()
}
