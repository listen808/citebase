package rag

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/connector"
)

// 本文件统一约束「由模型给定 URL」的出网请求（fetch_web / api_call）。
// 这些 URL 完全不可信：不加约束时模型（或被检索文档里的 prompt 注入）可以让服务端
// 去访问内网与云元数据地址，把凭据读回来。

var errBlockedOutbound = errors.New("目标地址不允许访问")

// allowedDomainMatch 判断 host 是否命中白名单（精确或子域）。
func allowedDomainMatch(host string, allowed []string) bool {
	host = strings.ToLower(strings.TrimSuffix(host, "."))
	for _, a := range allowed {
		a = strings.ToLower(strings.TrimSpace(a))
		if a == "" {
			continue
		}
		a = strings.TrimSuffix(strings.TrimPrefix(a, "."), ".")
		if host == a || strings.HasSuffix(host, "."+a) {
			return true
		}
	}
	return false
}

// validateOutboundURL 校验目标 URL：仅允许 http/https，且
// 配置了白名单时必须命中；未配置白名单时禁止内网/回环/链路本地与元数据地址。
func validateOutboundURL(rawURL string, allowedDomains []string) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("URL 无法解析: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("仅允许 http/https，当前 scheme=%q", parsed.Scheme)
	}
	if parsed.Hostname() == "" {
		return errors.New("URL 缺少主机名")
	}
	if len(allowedDomains) > 0 {
		if !allowedDomainMatch(parsed.Hostname(), allowedDomains) {
			return errBlockedOutbound
		}
		return nil
	}
	// 无白名单：至少挡住内网与元数据端点
	if err := connector.IsBlockedHost(parsed.String()); err != nil {
		return err
	}
	return nil
}

// newOutboundClient 构造受 SSRF 约束的 HTTP 客户端：
// 每一跳重定向都重新校验，避免「先过检再 302 到 169.254.169.254」的绕过。
func newOutboundClient(timeout time.Duration, allowedDomains []string) *http.Client {
	if timeout <= 0 {
		timeout = 15 * time.Second
	}
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("重定向次数过多")
			}
			return validateOutboundURL(req.URL.String(), allowedDomains)
		},
	}
}
