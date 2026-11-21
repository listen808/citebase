package common

// PageQuery 统一分页请求参数
type PageQuery struct {
	PageNum  int `form:"page_num" json:"page_num"`
	PageSize int `form:"page_size" json:"page_size"`
}

// Offset 返回数据库查询偏移量，同时规范化页码/每页数量
func (p PageQuery) Offset() int {
	if p.PageNum < 1 {
		p.PageNum = 1
	}
	if p.PageSize < 1 {
		p.PageSize = 10
	}
	return (p.PageNum - 1) * p.PageSize
}

// Limit 返回数据库查询限制数量，同时规范化每页数量
func (p PageQuery) Limit() int {
	if p.PageSize < 1 {
		p.PageSize = 10
	}
	if p.PageSize > 1000 {
		p.PageSize = 1000
	}
	return p.PageSize
}

// PageResult 统一分页响应结构
type PageResult[T any] struct {
	List     []T   `json:"list"`
	Total    int64 `json:"total"`
	PageNum  int   `json:"page_num"`
	PageSize int   `json:"page_size"`
}
