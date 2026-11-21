package repository

import (
	searchmodel "github.com/listen808/citebase/internal/model/search"
)

// 搜索领域表：全文检索的运行底账。
// aw_search_log 是索引监控页趋势/热词/无结果率/平均耗时的唯一数据来源，
// Bleve 自身的统计不含时间维度，无法回答这几个问题。
func init() {
	RegisterModels(
		&searchmodel.SearchLog{},
	)
}
