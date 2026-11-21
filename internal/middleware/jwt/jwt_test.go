package jwt

import (
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestGenerateAndParseToken(t *testing.T) {
	token, err := GenerateToken(42, "testuser", 1)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	if token == "" {
		t.Fatal("Expected non-empty token")
	}

	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}

	if claims.UserID != 42 {
		t.Errorf("Expected UserID=42, got %d", claims.UserID)
	}
	if claims.Username != "testuser" {
		t.Errorf("Expected Username='testuser', got '%s'", claims.Username)
	}
	if claims.RoleID != 1 {
		t.Errorf("Expected RoleID=1, got %d", claims.RoleID)
	}
}

func TestParseToken_Invalid(t *testing.T) {
	_, err := ParseToken("invalid-token-string")
	if err == nil {
		t.Error("Expected error for invalid token")
	}
}

func TestParseToken_Empty(t *testing.T) {
	_, err := ParseToken("")
	if err == nil {
		t.Error("Expected error for empty token")
	}
}

func TestGenerateRefreshToken(t *testing.T) {
	token, err := GenerateRefreshToken(99)
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}

	if token == "" {
		t.Fatal("Expected non-empty refresh token")
	}

	userID, remaining, err := ParseRefreshToken(token)
	if err != nil {
		t.Fatalf("ParseRefreshToken failed: %v", err)
	}

	if userID != 99 {
		t.Errorf("Expected userID=99, got %d", userID)
	}

	// 刷新令牌有效期为 7 天（见 GenerateRefreshToken）。
	// 旧断言写的是「剩余有效期 > 99 年」，与实现的 7 天契约不符，属于长期失败的陈旧测试。
	if remaining < 6*24*time.Hour || remaining > 7*24*time.Hour+time.Minute {
		t.Errorf("Expected remaining ≈7 days, got %v", remaining)
	}
}

func TestGenerateRefreshToken_WithExtend(t *testing.T) {
	token1, err := GenerateRefreshToken(1)
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}
	token2, err := GenerateRefreshToken(1, true)
	if err != nil {
		t.Fatalf("GenerateRefreshToken with extend failed: %v", err)
	}

	// Both should be valid non-empty tokens
	if token1 == "" || token2 == "" {
		t.Error("Expected non-empty tokens")
	}

	// Both should parse successfully
	uid1, _, err := ParseRefreshToken(token1)
	if err != nil || uid1 != 1 {
		t.Errorf("Token1 parse failed: uid=%d, err=%v", uid1, err)
	}
	uid2, _, err := ParseRefreshToken(token2)
	if err != nil || uid2 != 1 {
		t.Errorf("Token2 parse failed: uid=%d, err=%v", uid2, err)
	}
}

func TestParseRefreshToken_Invalid(t *testing.T) {
	_, _, err := ParseRefreshToken("garbage")
	if err == nil {
		t.Error("Expected error for invalid refresh token")
	}
}

func TestValidateSecret(t *testing.T) {
	tests := []struct {
		name   string
		secret string
		want   bool
	}{
		{"too short", "short", false},
		{"exactly 31 chars", strings.Repeat("a", 31), false},
		{"exactly 32 chars", strings.Repeat("a", 32), true},
		{"long enough", strings.Repeat("x", 64), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ValidateSecret(tt.secret)
			if got != tt.want {
				t.Errorf("ValidateSecret(%q) = %v, want %v", tt.secret, got, tt.want)
			}
		})
	}
}

func TestGenerateRandomKey(t *testing.T) {
	key1 := generateRandomKey(32)
	key2 := generateRandomKey(32)

	if key1 == "" {
		t.Error("Expected non-empty key")
	}
	if key2 == "" {
		t.Error("Expected non-empty key")
	}
	// Two random keys should be different
	if key1 == key2 {
		t.Error("Expected different random keys")
	}
}

// TestParseRefreshTokenRejectsNonHS256 钉住 WithValidMethods：
// 不带这条时，令牌自己 header 里声明的算法就是验签用的算法。
// 本系统只有 HMAC 一把密钥，所以今天还打不穿；但"两个验签函数强度不一样"
// 本身就是要拦的东西 —— ParseToken 加了、刷新令牌没加，读代码的人会以为都覆盖了。
func TestParseRefreshTokenRejectsNonHS256(t *testing.T) {
	for _, method := range []string{"HS512", "HS384"} {
		claims := jwtClaimsForSubject(t, "42", time.Hour)
		token := jwtNewWithClaims(method, claims)
		signed, err := token.SignedString(jwtKey)
		if err != nil {
			t.Fatalf("用 %s 造令牌失败: %v", method, err)
		}
		if _, _, err := ParseRefreshToken(signed); err == nil {
			t.Errorf("alg=%s 的刷新令牌被接受了 —— 只准 HS256", method)
		}
	}

	// 正面：本库自己签的 HS256 必须仍然可用，否则这条判据就成了把功能关掉。
	signed, err := GenerateRefreshToken(42)
	if err != nil {
		t.Fatal(err)
	}
	uid, remaining, err := ParseRefreshToken(signed)
	if err != nil {
		t.Fatalf("自家签发的 HS256 刷新令牌应当通过: %v", err)
	}
	if uid != 42 {
		t.Errorf("userID = %d, want 42", uid)
	}
	if remaining <= 0 {
		t.Errorf("剩余有效期应为正，实得 %v", remaining)
	}
}

// TestParseRefreshTokenRejectsNonPositiveSubject 钉住 sub=0 / sub=-5：
// Atoi 对它们都成功，不拦就等于允许一个 user_id=0 的会话被换出来。
func TestParseRefreshTokenRejectsNonPositiveSubject(t *testing.T) {
	for _, sub := range []string{"0", "-5", "", "abc"} {
		signed := signRefreshTokenWithSubject(t, sub, "HS256", time.Hour)
		if _, _, err := ParseRefreshToken(signed); err == nil {
			t.Errorf("subject=%q 的刷新令牌被接受了", sub)
		}
	}
}

// TestParseRefreshTokenRequiresExpiry 钉住缺 exp 的令牌。
// GenerateRefreshToken 恒写 exp，所以缺它就是"不是本系统签发的"；
// 原先缺 exp 会验签通过并返回 remaining=0，调用方一旦把 0 当"不限期"就是永久令牌。
func TestParseRefreshTokenRequiresExpiry(t *testing.T) {
	signed := signRefreshTokenWithSubject(t, "42", "HS256", 0)
	if _, _, err := ParseRefreshToken(signed); err == nil {
		t.Error("没有 exp 的刷新令牌被接受了（等于签发了一枚不过期令牌）")
	}
}

// ---- 造非法令牌用的小工具：走签名同款密钥，但算法/exp 由测试指定 ----

func jwtClaimsForSubject(_ *testing.T, sub string, ttl time.Duration) jwt.RegisteredClaims {
	c := jwt.RegisteredClaims{Issuer: "citebase", Subject: sub, IssuedAt: jwt.NewNumericDate(time.Now())}
	if ttl > 0 {
		c.ExpiresAt = jwt.NewNumericDate(time.Now().Add(ttl))
	}
	return c
}

func jwtNewWithClaims(method string, claims jwt.RegisteredClaims) *jwt.Token {
	return jwt.NewWithClaims(jwt.GetSigningMethod(method), claims)
}

func signRefreshTokenWithSubject(t *testing.T, sub, method string, ttl time.Duration) string {
	t.Helper()
	tok := jwtNewWithClaims(method, jwtClaimsForSubject(t, sub, ttl))
	signed, err := tok.SignedString(jwtKey)
	if err != nil {
		t.Fatalf("签名失败: %v", err)
	}
	return signed
}
