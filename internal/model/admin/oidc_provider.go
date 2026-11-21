package admin

import "time"

// OIDCProvider 企业 OIDC 身份提供方配置（对应 aw_oidc_provider 表）
type OIDCProvider struct {
	ID           uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	Name         string    `json:"name" gorm:"uniqueIndex:uk_name;size:50;not null"`
	DisplayName  string    `json:"display_name" gorm:"size:100"`
	Type         string    `json:"type" gorm:"size:20;not null;default:oidc"` // oidc, google, github, azure, keycloak
	ClientID     string    `json:"client_id" gorm:"size:255;not null"`
	ClientSecret string    `json:"-" gorm:"size:512;not null"`
	IssuerURL    string    `json:"issuer_url" gorm:"size:512"`
	AuthURL      string    `json:"auth_url" gorm:"size:512"`
	TokenURL     string    `json:"token_url" gorm:"size:512"`
	UserInfoURL  string    `json:"user_info_url" gorm:"size:512"`
	Scopes       string    `json:"scopes" gorm:"size:512;default:openid,profile,email"`
	Enabled      bool      `json:"enabled" gorm:"default:true"`
	AutoCreate   bool      `json:"auto_create" gorm:"default:true"`
	DefaultRole  string    `json:"default_role" gorm:"size:20;default:viewer"`
	IconURL      string    `json:"icon_url" gorm:"size:512"`
	SortOrder    int       `json:"sort_order" gorm:"default:0"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 指定表名
func (OIDCProvider) TableName() string {
	return "aw_oidc_provider"
}

// OIDCProviderResponse OIDC 提供方响应（隐藏 ClientSecret）
type OIDCProviderResponse struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
	ClientID    string `json:"client_id"`
	IssuerURL   string `json:"issuer_url"`
	AuthURL     string `json:"auth_url"`
	TokenURL    string `json:"token_url"`
	UserInfoURL string `json:"user_info_url"`
	Scopes      string `json:"scopes"`
	Enabled     bool   `json:"enabled"`
	AutoCreate  bool   `json:"auto_create"`
	DefaultRole string `json:"default_role"`
	IconURL     string `json:"icon_url"`
	SortOrder   int    `json:"sort_order"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
	// InitError 是最近一次初始化（OIDC 服务发现 / 端点校验）失败的原因，空表示正常。
	// 由 API 层从运行态填入，不落库：它是进程内状态，重启后会按最新一次尝试重算。
	InitError string `json:"init_error,omitempty"`
}

// ToResponse 转换为响应对象
func (p *OIDCProvider) ToResponse() OIDCProviderResponse {
	return OIDCProviderResponse{
		ID:          p.ID,
		Name:        p.Name,
		DisplayName: p.DisplayName,
		Type:        p.Type,
		ClientID:    p.ClientID,
		IssuerURL:   p.IssuerURL,
		AuthURL:     p.AuthURL,
		TokenURL:    p.TokenURL,
		UserInfoURL: p.UserInfoURL,
		Scopes:      p.Scopes,
		Enabled:     p.Enabled,
		AutoCreate:  p.AutoCreate,
		DefaultRole: p.DefaultRole,
		IconURL:     p.IconURL,
		SortOrder:   p.SortOrder,
		CreatedAt:   p.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   p.UpdatedAt.Format(time.RFC3339),
	}
}

// OIDCProviderPublicInfo 登录页展示的公开信息
type OIDCProviderPublicInfo struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
	Type        string `json:"type"`
	IconURL     string `json:"icon_url"`
}

// ToPublicInfo 转换为公开信息
func (p *OIDCProvider) ToPublicInfo() OIDCProviderPublicInfo {
	displayName := p.DisplayName
	if displayName == "" {
		displayName = p.Name
	}
	return OIDCProviderPublicInfo{
		ID:          p.ID,
		Name:        p.Name,
		DisplayName: displayName,
		Type:        p.Type,
		IconURL:     p.IconURL,
	}
}

// CreateOIDCProviderRequest 创建 OIDC 提供方请求
type CreateOIDCProviderRequest struct {
	Name         string `json:"name" binding:"required,min=1,max=50"`
	DisplayName  string `json:"display_name"`
	Type         string `json:"type" binding:"required,oneof=oidc google github azure keycloak"`
	ClientID     string `json:"client_id" binding:"required"`
	ClientSecret string `json:"client_secret" binding:"required"`
	IssuerURL    string `json:"issuer_url"`
	AuthURL      string `json:"auth_url"`
	TokenURL     string `json:"token_url"`
	UserInfoURL  string `json:"user_info_url"`
	Scopes       string `json:"scopes"`
	Enabled      *bool  `json:"enabled"`
	AutoCreate   *bool  `json:"auto_create"`
	DefaultRole  string `json:"default_role"`
	IconURL      string `json:"icon_url"`
	SortOrder    int    `json:"sort_order"`
}

// UpdateOIDCProviderRequest 更新 OIDC 提供方请求
type UpdateOIDCProviderRequest struct {
	Name         string `json:"name" binding:"required,min=1,max=50"`
	DisplayName  string `json:"display_name"`
	Type         string `json:"type" binding:"required,oneof=oidc google github azure keycloak"`
	ClientID     string `json:"client_id" binding:"required"`
	ClientSecret string `json:"client_secret"` // 空则不更新
	IssuerURL    string `json:"issuer_url"`
	AuthURL      string `json:"auth_url"`
	TokenURL     string `json:"token_url"`
	UserInfoURL  string `json:"user_info_url"`
	Scopes       string `json:"scopes"`
	Enabled      *bool  `json:"enabled"`
	AutoCreate   *bool  `json:"auto_create"`
	DefaultRole  string `json:"default_role"`
	IconURL      string `json:"icon_url"`
	SortOrder    int    `json:"sort_order"`
}

// OIDCUserMapping 用户 OIDC 身份映射（对应 aw_oidc_user_mapping 表）
type OIDCUserMapping struct {
	ID         uint      `json:"id" gorm:"primaryKey;autoIncrement"`
	UserID     int       `json:"user_id" gorm:"uniqueIndex:idx_oidc_user;not null"`
	ProviderID uint      `json:"provider_id" gorm:"uniqueIndex:idx_oidc_user;not null"`
	Subject    string    `json:"subject" gorm:"size:255;not null"` // OIDC subject identifier
	Email      string    `json:"email" gorm:"size:255"`
	RawInfo    string    `json:"-" gorm:"type:text"` // JSON raw user info from IdP
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// TableName 指定表名
func (OIDCUserMapping) TableName() string {
	return "aw_oidc_user_mapping"
}
