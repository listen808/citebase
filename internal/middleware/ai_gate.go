package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/response"
	"github.com/listen808/citebase/internal/utils/config"
)

// AIEnabledGate 部署级 AI 总开关门禁：
// AI 未启用（配置文件 ai.enabled=false 或 AI_ENABLED 环境变量关闭）时，
// 所有 /ai/* 能力端点直接 503——前端隐藏入口只是"不显示"，
// 这里保证"不调用"：即使有人绕过 UI 直接打接口也拿不到 AI 能力。
// 例外（在路由注册处不挂本中间件）：
//   - GET /ai/config：前端的能力探测通道（enabled 字段决定是否展示 AI UI）
//   - GET /ai/health：轻量探针，不发真实 LLM 请求
func AIEnabledGate() gin.HandlerFunc {
	return func(c *gin.Context) {
		if config.AppConfig.AI.Enabled {
			c.Next()
			return
		}
		response.JSON(c, http.StatusServiceUnavailable, 503, "AI 功能未启用", nil)
		c.Abort()
	}
}
