package jwt

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"

	logx "github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/utils/envname"

	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

var (
	ErrInvalidToken = errors.New("invalid token")
	ErrExpiredToken = errors.New("token has expired")
)

// Claims JWT 声明
type Claims struct {
	UserID   int    `json:"user_id"`
	Username string `json:"username"`
	RoleID   int    `json:"role_id"`
	jwt.RegisteredClaims
}

var jwtKey = []byte(getJWTSecret())

func getJWTSecret() string {
	// 1. 优先从环境变量读取
	secret := envname.Getenv("CITEBASE_JWT_SECRET", "AIWIKI_JWT_SECRET")
	if secret != "" {
		validateSecret(secret)
		return secret
	}

	// 2. 尝试从密钥文件读取（安装时生成）
	dataDir := envname.Getenv("CITEBASE_DATA_DIR", "AIWIKI_DATA_DIR")
	if dataDir == "" {
		dataDir = "./data"
	}
	keyFile := filepath.Join(dataDir, ".jwt_secret")
	if data, err := os.ReadFile(keyFile); err == nil && len(data) > 0 {
		secret = string(data)
		validateSecret(secret)
		return secret
	}

	// 3. 生成随机密钥并保存（首次启动时）
	secret = generateRandomKey(32)
	validateSecret(secret)
	_ = os.MkdirAll(filepath.Dir(keyFile), 0700)
	// 如果写入失败，至少使用生成的密钥
	_ = os.WriteFile(keyFile, []byte(secret), 0600)
	return secret
}

// ValidateSecret 检查 JWT Secret 强度（至少 32 字节）
func ValidateSecret(secret string) bool {
	return len(secret) >= 32
}

func validateSecret(secret string) {
	if !ValidateSecret(secret) {
		msg := "JWT secret is too weak (length < 32). Please set a stronger CITEBASE_JWT_SECRET (the pre-rename variable name is still accepted) or delete .jwt_secret to regenerate."
		if envname.Getenv("CITEBASE_ENV", "AIWIKI_ENV") == "production" {
			logx.Panic(msg)
		} else {
			logx.Warn(msg)
		}
	}
}

func generateRandomKey(length int) string {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		// 降级方案：使用更长的随机数据
		fallback := make([]byte, length+16)
		_, _ = rand.Read(fallback)
		return base64.StdEncoding.EncodeToString(fallback)
	}
	return base64.StdEncoding.EncodeToString(b)
}

// GenerateToken 生成访问令牌（2 小时有效期）
func GenerateToken(userID int, username string, roleID int) (string, error) {
	claims := Claims{
		UserID:   userID,
		Username: username,
		RoleID:   roleID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(2 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Issuer:    "citebase",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

// GenerateRefreshToken 生成刷新令牌（7 天有效期）
// 当 extend 为 true 时，基于当前时间重新计算有效期（用于滑动续期）
func GenerateRefreshToken(userID int, extend ...bool) (string, error) {
	claims := jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour)),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Issuer:    "citebase",
		Subject:   strconv.Itoa(userID),
	}

	if len(extend) > 0 && extend[0] {
		claims.ExpiresAt = jwt.NewNumericDate(time.Now().Add(7 * 24 * time.Hour))
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtKey)
}

// ParseToken 解析令牌
// P2：用 WithValidMethods 限制只接受 HS256，避免 alg=none 攻击
func ParseToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		// P2：校验 UserID > 0，避免空 claims 被当作有效身份
		if claims.UserID <= 0 {
			return nil, ErrInvalidToken
		}
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// ParseRefreshToken 解析刷新令牌，返回 userID 和剩余有效期
//
// 三条与 ParseToken 同规格的约束，原先只有第一条有：
//   - WithValidMethods(["HS256"])：不设的话签名算法取自令牌自己声明的 header，
//     而"用同一把 HMAC 密钥去验对方指定算法"正是算法混淆那一类攻击的入口。
//     ParseToken 在 P2 批次里加了这条并写了理由，刷新令牌漏了 ——
//     同一个库里两个验签函数强度不一样，比两个都松更危险，因为读代码的人会以为都覆盖了。
//   - Subject 必须是正整数：strconv.Atoi("0") 与 Atoi("-5") 都会成功，
//     于是 sub=0 的令牌能换出一个 user_id=0 的会话。
//   - 必须有 ExpiresAt：GenerateRefreshToken 恒写这一项，所以缺 exp 的令牌一定不是本系统签发的。
//     不拦的话它会被验签通过且 remaining=0，调用方若把 0 当"不限期"，就等于永久令牌。
func ParseRefreshToken(tokenString string) (int, time.Duration, error) {
	token, err := jwt.ParseWithClaims(tokenString, &jwt.RegisteredClaims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtKey, nil
	}, jwt.WithValidMethods([]string{"HS256"}))

	if err != nil {
		return 0, 0, err
	}

	if claims, ok := token.Claims.(*jwt.RegisteredClaims); ok && token.Valid {
		userID, err := strconv.Atoi(claims.Subject)
		if err != nil {
			return 0, 0, err
		}
		if userID <= 0 {
			return 0, 0, ErrInvalidToken
		}
		if claims.ExpiresAt == nil {
			return 0, 0, ErrInvalidToken
		}
		return userID, time.Until(claims.ExpiresAt.Time), nil
	}

	return 0, 0, ErrInvalidToken
}
