package utils

import (
	"regexp"
	"strings"
)

var sensitiveKeys = []string{"password", "token", "secret", "authorization", "old_password", "new_password", "refresh_token"}
var sensitiveRe *regexp.Regexp

// S6: PII脱敏正则 - 手机号、身份证、邮箱、银行卡、IP地址
// 两个关键点（旧实现都踩过）：
//  1. MaskPII 按序就地替换：更长、更特定的模式（身份证、银行卡）必须排在手机号之前，
//     否则手机号规则会先把身份证中间数字改写为星号，后续身份证模式永远无法整体命中，残留敏感片段；
//  2. 数字类模式用 \b 锚定，避免命中 18 位身份证 / 16-19 位银行卡长数字串中的 11 位子串。
var piiPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\b[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`), // 身份证
	regexp.MustCompile(`\b(?:\d{16}|\d{19})\b`),                                                           // 银行卡
	regexp.MustCompile(`\b1[3-9]\d{9}\b`),                                                                 // 手机号
	regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`),                                // 邮箱
	regexp.MustCompile(`\b(?:\d{1,3}\.){3}\d{1,3}\b`),                                                     // IP地址
	regexp.MustCompile(`(?:sk|pk|ak|key|api[_-]?key)[_-][\w\-]{20,}`),                                     // API密钥
}

func init() {
	pattern := `("(` + strings.Join(sensitiveKeys, "|") + `)"\s*:\s*")([^"]+)(")`
	sensitiveRe = regexp.MustCompile("(?i)" + pattern)
}

// MaskSensitiveInfo 脱敏敏感字段（JSON格式）
func MaskSensitiveInfo(s string) string {
	if s == "" {
		return ""
	}
	return sensitiveRe.ReplaceAllString(s, "${1}****${4}")
}

// MaskPII 脱敏文本中的个人身份信息（PII）
// S6: AI回答后处理过滤器
func MaskPII(text string) string {
	if text == "" {
		return ""
	}
	result := text
	for _, re := range piiPatterns {
		result = re.ReplaceAllStringFunc(result, func(match string) string {
			runes := []rune(match)
			if len(runes) <= 4 {
				return strings.Repeat("*", len(runes))
			}
			return string(runes[:3]) + strings.Repeat("*", len(runes)-6) + string(runes[len(runes)-3:])
		})
	}
	return result
}

func TruncateRunes(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes])
}

// TruncateWithEllipsis 截断字符串并添加省略号（rune安全，正确处理中文）
func TruncateWithEllipsis(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) <= maxRunes {
		return s
	}
	return string(runes[:maxRunes]) + "..."
}
