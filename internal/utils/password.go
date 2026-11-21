package utils

import (
	"errors"
	"regexp"
)

// ErrWeakPassword 密码不满足强度要求。规则与前端 validators.password 一致：
// 至少 8 位，且包含小写、大写、数字、特殊字符中的至少 3 类。
var ErrWeakPassword = errors.New("password must be at least 8 characters and contain at least 3 of: lowercase, uppercase, digits, special characters")

var passwordCharClasses = []*regexp.Regexp{
	regexp.MustCompile(`[a-z]`),
	regexp.MustCompile(`[A-Z]`),
	regexp.MustCompile(`[0-9]`),
	regexp.MustCompile(`[!@#$%^&*()_+\-=\[\]{};':"\\|,.<>/?]`),
}

// ValidatePasswordStrength 校验密码强度，不满足时返回 ErrWeakPassword。
func ValidatePasswordStrength(password string) error {
	if len(password) < 8 || len(password) > 60 {
		return ErrWeakPassword
	}
	types := 0
	for _, re := range passwordCharClasses {
		if re.MatchString(password) {
			types++
		}
	}
	if types < 3 {
		return ErrWeakPassword
	}
	return nil
}
