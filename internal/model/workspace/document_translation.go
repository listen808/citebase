package workspace

// DocumentTranslation 文档多语言翻译关联
// 用于关联同一文档的不同语言版本
type DocumentTranslation struct {
	ID                int    `gorm:"column:id;primaryKey;autoIncrement" json:"id"`
	DocumentID        int    `gorm:"column:document_id;index:idx_document_id" json:"document_id"`
	Lang              string `gorm:"column:lang;size:10;not null" json:"lang"` // 语言代码: zh-CN, en-US, ja-JP 等
	TranslatedDocID   int    `gorm:"column:translated_document_id;index:idx_translated_document_id" json:"translated_document_id"`
	TranslationStatus string `gorm:"column:translation_status;size:20;default:'draft'" json:"translation_status"` // draft / published
	CreatedAt         int    `gorm:"column:created_at" json:"created_at"`
}

func (DocumentTranslation) TableName() string {
	return "aw_document_translation"
}

// 支持的语言列表（可扩展）
var SupportedLanguages = []LanguageInfo{
	{Code: "zh-CN", Name: "简体中文", NativeName: "简体中文"},
	{Code: "en-US", Name: "English", NativeName: "English"},
	{Code: "ja-JP", Name: "Japanese", NativeName: "日本語"},
	{Code: "ko-KR", Name: "Korean", NativeName: "한국어"},
	{Code: "fr-FR", Name: "French", NativeName: "Français"},
	{Code: "de-DE", Name: "German", NativeName: "Deutsch"},
	{Code: "es-ES", Name: "Spanish", NativeName: "Español"},
	{Code: "ru-RU", Name: "Russian", NativeName: "Русский"},
}

// LanguageInfo 语言信息
type LanguageInfo struct {
	Code       string `json:"code"`
	Name       string `json:"name"`
	NativeName string `json:"native_name"`
}

// TranslationStatus 常量
const (
	TranslationStatusDraft     = "draft"
	TranslationStatusPublished = "published"
)
