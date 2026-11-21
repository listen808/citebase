package auth

import (
	logx "github.com/listen808/citebase/internal/log"
	adminmodel "github.com/listen808/citebase/internal/model/admin"
	usermodel "github.com/listen808/citebase/internal/model/user"
	authrepo "github.com/listen808/citebase/internal/repository/auth"
	userrepo "github.com/listen808/citebase/internal/repository/user"

	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// newAuthHTTPClient 认证请求专用客户端：禁止跟随重定向，避免把凭据带到被重定向到的第三方地址。
func newAuthHTTPClient() *http.Client {
	return &http.Client{
		Timeout: 10 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

type UnifiedLoginService struct {
	userRepo *userrepo.UserRepository
	authRepo *authrepo.AuthSSORepository
}

func NewUnifiedLoginService() *UnifiedLoginService {
	return &UnifiedLoginService{
		userRepo: userrepo.NewUserRepository(),
		authRepo: authrepo.NewAuthSSORepository(),
	}
}

type ssoExtConfig struct {
	Protocol     string `json:"protocol"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	BaseDN       string `json:"base_dn"`
	UserFilter   string `json:"user_filter"`
	TokenURL     string `json:"token_url"`
	HTTPMethod   string `json:"http_method"`
	// AuthURL 显式指定认证代理地址（推荐 https://），避免把口令走明文 HTTP 发送
	AuthURL string `json:"auth_url"`
	// UserField 身份字段名（IdP 不用 username 时配置，如 preferred_username）
	UserField string `json:"user_field"`
	// CallbackURL 授权回调地址，换取令牌时随 code 一起提交给 IdP
	CallbackURL string `json:"callback_url"`
	// UserInfoURL 身份接口地址；为空时从令牌响应体读取身份字段
	UserInfoURL string `json:"userinfo_url"`
}

func parseExtConfig(extData string) (*ssoExtConfig, error) {
	cfg := &ssoExtConfig{}
	if extData != "" {
		if err := json.Unmarshal([]byte(extData), cfg); err != nil {
			// 旧实现在这里 `return cfg, nil` 吞掉错误，返回一份 protocol 默认 http 的空配置：
			// ExtData 写坏时本应走 ldap/自定义协议的认证被悄悄降级为对 cfg.URL 的 http 认证。
			// 解析失败必须上抛，由调用方跳过该配置并告警。
			return nil, fmt.Errorf("解析 SSO 扩展配置失败: %w", err)
		}
	}
	if cfg.Protocol == "" {
		cfg.Protocol = "http"
	}
	return cfg, nil
}

func (s *UnifiedLoginService) AuthLogin(username, password string) (*usermodel.UserResponse, error) {
	configs, err := s.authRepo.GetUsedAuthSSOs()
	if err != nil || len(configs) == 0 {
		return nil, fmt.Errorf("no auth login config found")
	}

	for _, cfg := range configs {
		extCfg, err := parseExtConfig(cfg.ExtData)
		if err != nil {
			// 配置解析失败时跳过该 SSO 源，绝不带着「全空配置」继续降级成 http 认证
			logx.Warnf("[SSO] auth_sso id=%d ExtData 解析失败，已跳过该配置: %v", cfg.ID, err)
			continue
		}
		info, err := s.tryAuth(cfg, extCfg, username, password)
		if err != nil {
			continue
		}
		info.AuthID = cfg.ID
		return s.findOrCreateUser(info)
	}
	return nil, fmt.Errorf("authentication failed")
}

func (s *UnifiedLoginService) tryAuth(cfg *adminmodel.AuthSSO, extCfg *ssoExtConfig, username, password string) (*ssoUserInfo, error) {
	switch extCfg.Protocol {
	case "ldap", "ldaps":
		return s.ldapAuth(cfg, extCfg, username, password)
	default:
		return s.httpAuth(cfg, extCfg, username, password)
	}
}

func (s *UnifiedLoginService) ldapAuth(cfg *adminmodel.AuthSSO, extCfg *ssoExtConfig, username, password string) (*ssoUserInfo, error) {
	host := extCfg.Host
	port := extCfg.Port
	if host == "" {
		u, _ := url.Parse(cfg.URL)
		host = u.Hostname()
		if port == 0 {
			p, _ := parsePort(u.Port())
			port = p
		}
	}
	if port == 0 {
		if extCfg.Protocol == "ldaps" {
			port = 636
		} else {
			port = 389
		}
	}

	client := newAuthHTTPClient()
	authURL := extCfg.AuthURL
	if authURL == "" {
		scheme := "http"
		if extCfg.Protocol == "ldaps" {
			// ldaps 的语义就是 TLS 通道：旧配置兜底地址也必须强制 https，
			// 否则域账号口令与 bind_password 会以明文在内网链路上被截获。
			scheme = "https"
		} else {
			logx.Warnf("[SSO] auth_sso id=%d 未配置 auth_url，口令将以明文 HTTP 传输，请改为 https 认证代理地址", cfg.ID)
		}
		authURL = fmt.Sprintf("%s://%s:%d/ldap-auth", scheme, host, port)
	} else if extCfg.Protocol == "ldaps" && strings.HasPrefix(strings.ToLower(authURL), "http://") {
		// 声明了 ldaps 却配了 http 代理地址：禁止回退明文，fail-closed
		return nil, fmt.Errorf("protocol=ldaps 要求使用 https 认证地址，拒绝以明文传输凭据")
	}
	data := url.Values{}
	data.Set("username", username)
	data.Set("password", password)
	if extCfg.ClientID != "" {
		data.Set("bind_dn", extCfg.ClientID)
	}
	if extCfg.ClientSecret != "" {
		data.Set("bind_password", extCfg.ClientSecret)
	}
	if extCfg.BaseDN != "" {
		data.Set("base_dn", extCfg.BaseDN)
	}
	if extCfg.UserFilter != "" {
		data.Set("user_filter", extCfg.UserFilter)
	}

	resp, err := client.PostForm(authURL, data) //nolint:noctx // bounded auth client, no request ctx in signature
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("ldap auth failed: %d", resp.StatusCode)
	}

	return &ssoUserInfo{
		Username:  s.buildUsername(cfg, username),
		GivenName: username,
	}, nil
}

func (s *UnifiedLoginService) httpAuth(cfg *adminmodel.AuthSSO, extCfg *ssoExtConfig, username, password string) (*ssoUserInfo, error) {
	apiURL := cfg.URL
	if apiURL == "" {
		if extCfg.Host != "" {
			scheme := "http"
			if extCfg.Protocol == "https" {
				scheme = "https"
			}
			port := extCfg.Port
			if port == 0 {
				port = 80
			}
			apiURL = fmt.Sprintf("%s://%s:%d%s", scheme, extCfg.Host, port, extCfg.TokenURL)
		} else {
			return nil, fmt.Errorf("no auth url configured")
		}
	}

	data := url.Values{}
	data.Set("username", username)
	data.Set("password", password)

	client := newAuthHTTPClient()
	var resp *http.Response
	var err error

	method := strings.ToUpper(extCfg.HTTPMethod)
	if method == "" {
		method = "GET"
	}

	if method == "POST" {
		resp, err = client.PostForm(apiURL, data) //nolint:noctx // bounded auth client
	} else {
		reqURL := fmt.Sprintf("%s?%s", apiURL, data.Encode())
		resp, err = client.Get(reqURL) //nolint:noctx // bounded auth client
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	// 2xx 只是传输层成功，不等于认证通过；进一步要求响应体给出可验证的身份或成功语义。
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("http auth returned %d", resp.StatusCode)
	}

	body, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if readErr != nil {
		return nil, fmt.Errorf("读取认证响应失败: %w", readErr)
	}
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		// 旧实现在这里直接判定登录成功（任何非 JSON 响应都放行），属认证绕过；改为必须拿到身份信息才通过。
		return nil, fmt.Errorf("认证响应不是合法 JSON，无法确认身份")
	}
	if authRejected(result) {
		return nil, fmt.Errorf("统一认证拒绝登录: %s", authRejectReason(result))
	}

	// 身份来源优先级：data.username > username > 配置的 user_field；全部缺失即失败（fail-closed）
	dataMap, _ := result["data"].(map[string]interface{})
	idName := ssoFirstStr(dataMap, result, "username")
	if idName == "" {
		if extCfg.UserField != "" {
			idName = ssoFirstStr(dataMap, result, extCfg.UserField)
		}
	}
	if idName == "" {
		return nil, fmt.Errorf("认证响应未包含用户身份字段，拒绝登录")
	}

	return &ssoUserInfo{
		Username:   s.buildUsername(cfg, idName),
		GivenName:  ssoGetStr(dataMap, "given_name", idName),
		Email:      ssoGetStr(dataMap, "email", ""),
		Mobile:     ssoGetStr(dataMap, "mobile", ""),
		Phone:      ssoGetStr(dataMap, "phone", ""),
		Department: ssoGetStr(dataMap, "department", ""),
		Position:   ssoGetStr(dataMap, "position", ""),
		Location:   ssoGetStr(dataMap, "location", ""),
		IM:         ssoGetStr(dataMap, "im", ""),
	}, nil
}

// authRejected 识别 IdP 用 JSON 信封表达的失败（HTTP 仍是 200）。
func authRejected(result map[string]interface{}) bool {
	if v, ok := result["success"]; ok {
		if b, isBool := v.(bool); isBool && !b {
			return true
		}
	}
	if v, ok := result["code"]; ok {
		switch n := v.(type) {
		case float64:
			if n != 0 {
				return true
			}
		case string:
			if n != "" && n != "0" {
				return true
			}
		}
	}
	if v, ok := result["result"]; ok {
		if s, isStr := v.(string); isStr {
			ls := strings.ToLower(s)
			if ls == "fail" || ls == "false" || ls == "error" {
				return true
			}
		}
	}
	if v, ok := result["error"]; ok {
		switch e := v.(type) {
		case string:
			if e != "" {
				return true
			}
		default:
			if e != nil {
				return true
			}
		}
	}
	return false
}

func authRejectReason(result map[string]interface{}) string {
	for _, k := range []string{"msg", "message", "error", "error_description"} {
		if s, ok := result[k].(string); ok && s != "" {
			r := []rune(s)
			if len(r) > 120 {
				r = r[:120]
			}
			return string(r)
		}
	}
	return "响应中无成功标识"
}

// ssoFirstStr 依次在 data 与顶层查找字符串字段。
func ssoFirstStr(dataMap, root map[string]interface{}, key string) string {
	if v := ssoGetStr(dataMap, key, ""); v != "" {
		return v
	}
	if s, ok := root[key].(string); ok {
		return strings.TrimSpace(s)
	}
	return ""
}

func (s *UnifiedLoginService) buildUsername(cfg *adminmodel.AuthSSO, username string) string {
	if cfg.UsernamePrefix != "" && !strings.HasPrefix(username, cfg.UsernamePrefix+"_") {
		return cfg.UsernamePrefix + "_" + username
	}
	return username
}

type ssoUserInfo struct {
	Username   string
	GivenName  string
	Email      string
	Mobile     string
	Phone      string
	Department string
	Position   string
	Location   string
	IM         string
	AuthID     int
}

func (s *UnifiedLoginService) findOrCreateUser(info *ssoUserInfo) (*usermodel.UserResponse, error) {
	existing, err := s.userRepo.FindByUsername(info.Username)
	if err == nil && existing != nil && existing.IsDelete == 0 {
		return s.syncUserInfo(existing, info)
	}

	user := &usermodel.User{
		Username:   info.Username,
		GivenName:  info.GivenName,
		Email:      info.Email,
		Mobile:     info.Mobile,
		Phone:      info.Phone,
		Department: info.Department,
		Position:   info.Position,
		Location:   info.Location,
		IM:         info.IM,
		RoleID:     3,
	}
	if err := s.userRepo.Create(user); err != nil {
		return nil, err
	}
	resp := user.ToResponse()
	return &resp, nil
}

func (s *UnifiedLoginService) syncUserInfo(user *usermodel.User, info *ssoUserInfo) (*usermodel.UserResponse, error) {
	changed := false
	if info.GivenName != "" && user.GivenName != info.GivenName {
		user.GivenName = info.GivenName
		changed = true
	}
	if info.Email != "" && user.Email != info.Email {
		user.Email = info.Email
		changed = true
	}
	if info.Mobile != "" && user.Mobile != info.Mobile {
		user.Mobile = info.Mobile
		changed = true
	}
	if info.Phone != "" && user.Phone != info.Phone {
		user.Phone = info.Phone
		changed = true
	}
	if info.Department != "" && user.Department != info.Department {
		user.Department = info.Department
		changed = true
	}
	if info.Position != "" && user.Position != info.Position {
		user.Position = info.Position
		changed = true
	}
	if info.Location != "" && user.Location != info.Location {
		user.Location = info.Location
		changed = true
	}
	if info.IM != "" && user.IM != info.IM {
		user.IM = info.IM
		changed = true
	}
	if changed {
		_ = s.userRepo.Update(user)
	}
	resp := user.ToResponse()
	return &resp, nil
}

func ssoGetStr(m map[string]interface{}, key, defaultVal string) string {
	if m == nil {
		return defaultVal
	}
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return defaultVal
}

func parsePort(s string) (int, bool) {
	if s == "" {
		return 0, false
	}
	var p int
	if _, err := fmt.Sscanf(s, "%d", &p); err != nil {
		return 0, false
	}
	return p, true
}
