package search

// SearchDocumentRequest 搜索请求
type SearchDocumentRequest struct {
	Query           string `json:"query" binding:"required"`
	SpaceID         int    `json:"space_id"`
	Limit           int    `json:"limit"`
	Type            string `json:"type"`
	AuthorID        int    `json:"author_id"`         // 按作者过滤（create_user_id）
	TagName         string `json:"tag_name"`          // 按标签过滤
	TimeFrom        int64  `json:"time_from"`         // 时间范围起始（Unix 时间戳）
	TimeTo          int64  `json:"time_to"`           // 时间范围结束（Unix 时间戳）
	SortBy          string `json:"sort_by"`           // 排序方式: relevance(默认) / time_desc / time_asc
	AllowedSpaceIDs []int  `json:"allowed_space_ids"` // 权限过滤：允许搜索的空间 ID 列表，nil 表示不过滤
	Offset          int    `json:"offset"`            // 分页偏移量
}

// SearchDocumentResult 搜索结果
type SearchDocumentResult struct {
	DocumentID         int                            `json:"document_id"`
	SpaceID            int                            `json:"space_id"`
	SpaceName          string                         `json:"space_name,omitempty"`
	Name               string                         `json:"name"`
	Type               int                            `json:"type"`
	Path               string                         `json:"path"`
	ParentID           int                            `json:"parent_id,omitempty"` // 直接父节点 ID
	PathIDs            []int                          `json:"path_ids,omitempty"`  // 完整路径 ID 列表（不含自身）
	Score              float64                        `json:"score"`
	Highlights         map[string]string              `json:"highlights"`                    // 向后兼容
	HighlightFragments map[string][]HighlightFragment `json:"highlight_fragments,omitempty"` // 多片段高亮
	MatchCount         int                            `json:"match_count,omitempty"`         // 匹配总数
	CreateTime         int64                          `json:"create_time"`
}

// SearchDocument 可搜索的文档结构
type SearchDocument struct {
	ID         int    `json:"id"`
	SpaceID    int    `json:"space_id"`
	Name       string `json:"name"`
	Content    string `json:"content"`
	NamePinyin string `json:"name_pinyin"`
	Type       int    `json:"type"`
	Path       string `json:"path"`
}

// IndexableDocument 用于批量索引
type IndexableDocument struct {
	ID      int
	SpaceID int
	Name    string
	Content string
	Type    int
	Path    string
}

// HighlightFragment 高亮片段结构
type HighlightFragment struct {
	Text  string  `json:"text"`  // 纯文本片段
	HTML  string  `json:"html"`  // 带 <mark> 标签的 HTML
	Score float64 `json:"score"` // 关键词密度分
}
