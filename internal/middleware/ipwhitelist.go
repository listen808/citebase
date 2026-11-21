package middleware

import (
	"net"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/response"
	"github.com/listen808/citebase/internal/utils/config"
)

// ipRange 表示一个已解析的白名单条目（单 IP 或 CIDR）
type ipRange struct {
	network *net.IPNet
	ip      net.IP
}

var (
	parsedIPRanges  []ipRange
	ipWhitelistOnce sync.Once
)

// loadIPWhitelist 解析配置中的 IP 白名单（进程生命周期内只解析一次）
func loadIPWhitelist() {
	cfg := config.AppConfig.Security
	parsedIPRanges = nil
	for _, item := range cfg.IPWhitelist {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		if strings.Contains(item, "/") {
			_, network, err := net.ParseCIDR(item)
			if err != nil {
				continue
			}
			parsedIPRanges = append(parsedIPRanges, ipRange{network: network})
		} else {
			if ip := net.ParseIP(item); ip != nil {
				parsedIPRanges = append(parsedIPRanges, ipRange{ip: ip})
			}
		}
	}
}

// ipAllowed 判断客户端 IP 是否放行
func ipAllowed(clientIP, mode string, ranges []ipRange) bool {
	ip := net.ParseIP(clientIP)
	if ip == nil {
		// 无法解析的 IP 按不信任处理
		return false
	}
	matched := false
	for _, r := range ranges {
		if r.network != nil {
			if r.network.Contains(ip) {
				matched = true
				break
			}
		} else if r.ip != nil && r.ip.Equal(ip) {
			matched = true
			break
		}
	}
	if mode == "deny" {
		return !matched
	}
	return matched
}

// IPWhitelistMiddleware IP 白名单中间件（P1-G115）
//   - mode=allow（默认）：仅放行白名单内的 IP，其余返回 403
//   - mode=deny：封锁白名单内的 IP，其余放行
func IPWhitelistMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg := config.AppConfig.Security
		if !cfg.IPWhitelistEnabled {
			c.Next()
			return
		}
		ipWhitelistOnce.Do(loadIPWhitelist)

		if !ipAllowed(c.ClientIP(), cfg.IPWhitelistMode, parsedIPRanges) {
			response.Forbidden(c, "Access denied by IP whitelist")
			c.Abort()
			return
		}
		c.Next()
	}
}
