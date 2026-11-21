package workspace

import (
	"regexp"
	"strings"
)

// DocumentMention 文档 @提及 模型 (对应 aw_document_mention 表)
// 记录文档内容中 @ 到的用户，用于通知与订阅。
type DocumentMention struct {
	ID         int `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	DocumentID int `gorm:"column:document_id;index:idx_mention_doc;uniqueIndex:uk_doc_user" json:"document_id"`
	UserID     int `gorm:"column:user_id;index:idx_mention_user;uniqueIndex:uk_doc_user" json:"user_id"`
	CreateTime int `gorm:"column:create_time" json:"create_time"`
}

func (DocumentMention) TableName() string {
	return "aw_document_mention"
}

// MentionUser 被提及用户简要信息
type MentionUser struct {
	UserID    int    `json:"user_id"`
	Username  string `json:"username"`
	GivenName string `json:"given_name,omitempty"`
}

// mentionPattern 匹配 @username 或 @给定名（不含空白，且 @ 前为行首或空白，避免误匹配邮箱）
var mentionPattern = regexp.MustCompile(`(^|\s)@([^\s@,，。;；:：<>()\[\]"']+)`)

// ExtractMentions 从文本中提取被 @ 的用户标识（去重，保留原始顺序）。
func ExtractMentions(text string) []string {
	matches := mentionPattern.FindAllStringSubmatch(text, -1)
	seen := make(map[string]bool)
	var out []string
	for _, m := range matches {
		name := strings.TrimSpace(m[2])
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}
