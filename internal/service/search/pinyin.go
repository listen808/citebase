package search

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/mozillazg/go-pinyin"
)

var cjkRegexp = regexp.MustCompile(`[\p{Han}]+`)

func ToPinyinInitials(text string) string {
	if text == "" {
		return ""
	}

	matches := cjkRegexp.FindAllString(text, -1)
	if len(matches) == 0 {
		return ""
	}

	var result strings.Builder
	for _, match := range matches {
		a := pinyin.NewArgs()
		a.Style = pinyin.FirstLetter
		a.Heteronym = false

		pyList := pinyin.Pinyin(match, a)
		for _, py := range pyList {
			if len(py) > 0 {
				result.WriteString(py[0])
			}
		}
	}

	return result.String()
}

func ToPinyinFull(text string) string {
	if text == "" {
		return ""
	}

	matches := cjkRegexp.FindAllString(text, -1)
	if len(matches) == 0 {
		return ""
	}

	var result strings.Builder
	for _, match := range matches {
		a := pinyin.NewArgs()
		a.Style = pinyin.Normal
		a.Heteronym = false

		pyList := pinyin.Pinyin(match, a)
		for i, py := range pyList {
			if len(py) > 0 {
				result.WriteString(py[0])
			}
			if i < len(pyList)-1 {
				result.WriteString(" ")
			}
		}
	}

	return result.String()
}

func isCJK(r rune) bool {
	return unicode.Is(unicode.Han, r)
}

// PinyinIndexText 生成索引用的拼音文本：首字母单字符空格分隔 + 全拼音节空格分隔。
// 例："微服务架构" → "w j f w j g fu wu jia gou"
// 单字母 token 使首字母搜索（"wjfwjg" → 查询 "w j"）能通过 MatchQuery 命中，
// 无需依赖 bleve 自定义 ngram analyzer（其在当前版本注册表中不可用）。
func PinyinIndexText(text string) string {
	initials := ToPinyinInitials(text)
	full := ToPinyinFull(text)

	var spacedInitials strings.Builder
	for i, r := range initials {
		if i > 0 {
			spacedInitials.WriteByte(' ')
		}
		spacedInitials.WriteRune(r)
	}

	parts := make([]string, 0, 2)
	if spacedInitials.Len() > 0 {
		parts = append(parts, spacedInitials.String())
	}
	if full != "" {
		parts = append(parts, full)
	}
	return strings.Join(parts, " ")
}
