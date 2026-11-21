package utils

import (
	"math"
	"strconv"

	"github.com/gin-gonic/gin"
)

// MaxPageSize 全局分页上限：调用方传入超大 page_size 时钳制，
// 避免「忘钳制即全表扫」被放大（如通知链路 api→service→repo 全程透传）。
const MaxPageSize = 200

// ParsePagination parses page/page_size query parameters and returns
// page, pageSize and offset. Negative or zero values are normalized.
func ParsePagination(c *gin.Context, defaultPageSize int) (page, pageSize, offset int) {
	if defaultPageSize < 1 {
		defaultPageSize = 20
	}
	if defaultPageSize > MaxPageSize {
		defaultPageSize = MaxPageSize
	}
	page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", strconv.Itoa(defaultPageSize)))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = defaultPageSize
	}
	if pageSize > MaxPageSize {
		pageSize = MaxPageSize
	}
	// (page-1)*pageSize 在 int 下可溢出为负（极大 page/page_size 组合），
	// 用 int64 计算并钉到安全上界。
	off := (int64(page) - 1) * int64(pageSize)
	if off < 0 {
		off = 0
	}
	if off > math.MaxInt32 {
		off = math.MaxInt32
	}
	offset = int(off)
	return
}
