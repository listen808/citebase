package admin

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	adminmodel "github.com/listen808/citebase/internal/model/admin"
	repos "github.com/listen808/citebase/internal/repository"
	"github.com/listen808/citebase/internal/utils"
	"gorm.io/gorm"
)

// ErrApiKeyNotFound 密钥不存在
var ErrApiKeyNotFound = errors.New("api key not found")

// ApiKeyRepository 开放平台密钥仓储（G029）
type ApiKeyRepository struct {
	db *gorm.DB
}

func NewApiKeyRepository() *ApiKeyRepository {
	return &ApiKeyRepository{db: repos.DB}
}

// generateRawKey 生成形如 "mwk_<32hex>" 的明文密钥。
func generateRawKey() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return "mwk_" + hex.EncodeToString(b), nil
}

func hashKey(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// Create 生成新密钥，返回 (记录, 明文密钥, error)。明文仅此一次返回。
func (r *ApiKeyRepository) Create(appName string, ownerID, rateLimit int, expiresAt *time.Time) (*adminmodel.ApiKey, string, error) {
	raw, err := generateRawKey()
	if err != nil {
		return nil, "", err
	}
	enc, err := utils.Encrypt(raw)
	if err != nil {
		return nil, "", err
	}
	rec := &adminmodel.ApiKey{
		AppName:   appName,
		OwnerID:   ownerID,
		KeyHash:   hashKey(raw),
		KeyEnc:    enc,
		Status:    adminmodel.ApiKeyStatusActive,
		RateLimit: rateLimit,
	}
	if expiresAt != nil {
		rec.ExpiresAt = *expiresAt
	}
	if err := r.db.Create(rec).Error; err != nil {
		return nil, "", err
	}
	return rec, raw, nil
}

// FindByHash 按密钥哈希查找（用于鉴权）。
func (r *ApiKeyRepository) FindByHash(hash string) (*adminmodel.ApiKey, error) {
	var k adminmodel.ApiKey
	if err := r.db.Where("key_hash = ?", hash).First(&k).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrApiKeyNotFound
		}
		return nil, err
	}
	return &k, nil
}

// FindByOwner 列出某用户持有的密钥。
func (r *ApiKeyRepository) FindByOwner(ownerID int) ([]*adminmodel.ApiKey, error) {
	var list []*adminmodel.ApiKey
	if err := r.db.Where("owner_id = ?", ownerID).Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// FindAll 列出全部密钥（管理员）。
func (r *ApiKeyRepository) FindAll() ([]*adminmodel.ApiKey, error) {
	var list []*adminmodel.ApiKey
	if err := r.db.Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, err
	}
	return list, nil
}

// Revoke 吊销密钥。
func (r *ApiKeyRepository) Revoke(id int) error {
	return r.db.Model(&adminmodel.ApiKey{}).Where("id = ?", id).
		Update("status", adminmodel.ApiKeyStatusRevoked).Error
}

// Touch 更新最后使用时间。
func (r *ApiKeyRepository) Touch(id int) error {
	return r.db.Model(&adminmodel.ApiKey{}).Where("id = ?", id).
		Update("last_used_at", time.Now()).Error
}
