package workspace

// ImportDocumentItem 批量导入单项
type ImportDocumentItem struct {
	Path    string `json:"path" binding:"required"`
	Content string `json:"content" binding:"required"`
	Format  string `json:"format"` // md / confluence / notion
}

// ImportDocumentsRequest 批量导入请求
type ImportDocumentsRequest struct {
	Items []ImportDocumentItem `json:"items" binding:"required,dive"`
}

// ImportDocumentsResult 批量导入结果
type ImportDocumentsResult struct {
	Created int      `json:"created"`
	Skipped int      `json:"skipped"`
	Errors  []string `json:"errors"`
}
