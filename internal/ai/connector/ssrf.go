package connector

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/listen808/citebase/internal/utils/netguard"
)

// IsBlockedHost 判断主机是否属于被禁止访问的内部/元数据地址。
//
// 这一层只做"请求前"的静态判定（scheme、主机名形式的本地地址、写成 IP 字面量的内网地址）。
// 它**不能**单独构成 SSRF 防护：原先这里用 net.LookupIP 解析后逐个判定，
// 而 Go 的 http.Transport 随后会拿主机名再解析一次去连接 —— 攻击者把 TTL 设成 1 秒，
// 两次解析给出不同答案，校验那次给公网 IP、连接那次给 169.254.169.254，防线整条形同虚设。
// 真正的连接期判定与"钉住刚校验过的 IP"在 netguard.DialContext 里，
// 由 NewWebScraper 装进 transport。
func IsBlockedHost(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty host")
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") {
		return fmt.Errorf("localhost access denied")
	}
	if err := netguard.CheckURL(rawURL); err != nil {
		return err
	}
	return nil
}
