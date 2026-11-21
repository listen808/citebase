package admin

import "time"

// ApiKey 开放平台应用密钥（G029）。
// 明文 key 仅创建时返回一次，库内以 AES-GCM 加密存储。
type ApiKey struct {
	ID         int       `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	AppName    string    `gorm:"column:app_name;size:128;not null" json:"app_name"`
	OwnerID    int       `gorm:"column:owner_id;not null;default:0;index:idx_owner_id" json:"owner_id"`
	KeyHash    string    `gorm:"column:key_hash;size:128;not null;uniqueIndex:uk_key_hash" json:"-"` // 用于查找的哈希
	KeyEnc     string    `gorm:"column:key_enc;size:512;not null" json:"-"`                          // AES-GCM 加密的明文
	Status     string    `gorm:"column:status;size:16;not null;default:'active'" json:"status"`      // active/revoked
	RateLimit  int       `gorm:"column:rate_limit;not null;default:60" json:"rate_limit"`            // 每分钟请求上限
	ExpiresAt  time.Time `gorm:"column:expires_at;default:NULL" json:"expires_at"`
	LastUsedAt time.Time `gorm:"column:last_used_at;default:NULL" json:"last_used_at"`
	CreatedAt  time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (ApiKey) TableName() string {
	return "aw_api_key"
}

// ApiKey 状态常量
const (
	ApiKeyStatusActive  = "active"
	ApiKeyStatusRevoked = "revoked"
)
