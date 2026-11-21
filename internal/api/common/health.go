package common

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/brand"
	repos "github.com/listen808/citebase/internal/repository"
	"github.com/listen808/citebase/internal/response"
	"github.com/listen808/citebase/internal/version"
	"gorm.io/gorm"
)

// HealthCheck liveness：进程活着就返回 UP，不探依赖。
// K8s 的 liveness 探针如果依赖 MySQL，一次数据库抖动会把所有副本重启一遍。
//
// 这里带 version 不带 commit：运维要能在不登录的情况下确认线上跑的是哪个版本
// （以前 /health 什么版本信息都没有，只能进容器看二进制），
// 而 commit / build date 属于构建细节，放在管理端的服务器状态页。
func HealthCheck(c *gin.Context) {
	response.Success(c, gin.H{
		"status":  "UP",
		"version": version.Version,
		"message": brand.DefaultName + " API is running",
	})
}

// ReadyCheck readiness：真正探依赖，失败时返回 503 让负载均衡把流量摘走。
// P1-5：原先 health 无条件返回 UP，MySQL 挂了编排层也认为服务健康。
func ReadyCheck(c *gin.Context) {
	checks := make(map[string]string)
	ready := true

	if repos.DB == nil {
		checks["mysql"] = "not initialized"
		ready = false
	} else if err := pingDB(repos.DB); err != nil {
		checks["mysql"] = "down: " + err.Error()
		ready = false
	} else {
		checks["mysql"] = "up"
	}

	if repos.AppCache != nil {
		if err := repos.AppCache.Ping(c.Request.Context()); err != nil {
			// 缓存挂了不摘流量：Redis 是可选依赖，降级到内存模式仍可服务
			checks["cache"] = "degraded: " + err.Error()
		} else {
			checks["cache"] = "up"
		}
	} else {
		checks["cache"] = "not initialized"
	}

	status := http.StatusOK
	msg := "READY"
	if !ready {
		status = http.StatusServiceUnavailable
		msg = "NOT_READY"
	}
	response.JSON(c, status, 0, msg, gin.H{"checks": checks, "version": version.Version})
}

func pingDB(db *gorm.DB) error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Ping()
}
