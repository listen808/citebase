package text2sql

import (
	"sync"

	"github.com/listen808/citebase/internal/utils/config"
)

// text2sqlConfig 全局 Text2SQL 配置（线程安全）
var (
	text2sqlConfig     config.Text2SQLConfig
	text2sqlConfigOnce sync.Once
)

// InitText2SQLConfig 初始化 Text2SQL 配置（幂等，仅首次调用生效）
// 在 ai.go 装配时调用
func InitText2SQLConfig(cfg config.Text2SQLConfig) {
	text2sqlConfigOnce.Do(func() {
		text2sqlConfig = cfg
	})
}

// GetText2SQLConfig 获取 Text2SQL 配置
// 如果未初始化，返回默认值
func GetText2SQLConfig() config.Text2SQLConfig {
	text2sqlConfigOnce.Do(func() {
		text2sqlConfig = config.DefaultText2SQLConfig()
	})
	return text2sqlConfig
}
