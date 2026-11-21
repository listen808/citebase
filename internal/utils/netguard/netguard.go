// Package netguard 提供"出网前先判定目标地址"的共用实现。
//
// 仓里有两处地方需要它：AI 的网页抓取器（用户给什么 URL 就抓什么）和
// webhook 投递（用户配什么 URL 服务端就往外 POST 什么）。两处曾经各写一份判据，
// 而且都写成"先解析、后连接"—— 那是 TOCTOU：攻击者把域名 TTL 设成 1 秒，
// 校验那次解析回公网 IP（放行），Go 的 http.Transport 随后自己再解析一次，
// 这回回 127.0.0.1 或 169.254.169.254，于是内网与云元数据照打不误。
//
// 这里的 DialContext 把"解析、判定、连接"合成一步：**连的就是刚判定过的那个 IP**，
// 不再让标准库拿主机名去解析第二遍。TLS 的 SNI 与证书校验仍按请求 URL 里的主机名走
// （http.Transport 从 req.URL 取 ServerName，不看连接地址），所以钉 IP 不影响证书。
package netguard

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// BlockedIP 判定禁止访问的地址：环回、私网、链路本地、组播、未指定，以及云元数据端点。
// 元数据那两个地址（169.254.169.254 / 169.254.169.253）落在 link-local 段里，
// 但按段判会连带把正常的链路本地用法一起禁掉，所以单独点名，读代码的人知道这是防谁。
func BlockedIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() || ip.IsMulticast() {
		return true
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip.Equal(net.IPv4(169, 254, 169, 254)) || ip.Equal(net.IPv4(169, 254, 169, 253)) {
			return true
		}
	}
	return false
}

// CheckURL 做请求前的静态判定：scheme 白名单 + 主机名形式的本地地址。
// 它不看 IP —— IP 那一半必须在连接时判（见 DialContext），因为解析结果是可以变的。
func CheckURL(rawURL string, allowedSchemes ...string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		schemes := allowedSchemes
		if len(schemes) == 0 {
			schemes = []string{"http", "https"}
		}
		ok := false
		for _, s := range schemes {
			if strings.EqualFold(u.Scheme, s) {
				ok = true
				break
			}
		}
		if !ok {
			return fmt.Errorf("scheme %q 不被允许（仅 http/https）", u.Scheme)
		}
	}
	host := u.Hostname()
	if host == "" {
		return fmt.Errorf("empty host")
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") || lower == "metadata.google.internal" {
		return fmt.Errorf("localhost access denied")
	}
	// 主机名本身就写成 IP 字面量时，这里就能当场拒掉，不必等到连接。
	if ip := net.ParseIP(host); ip != nil && BlockedIP(ip) {
		return fmt.Errorf("internal address access denied: %s", ip.String())
	}
	return nil
}

// DialContext 返回一个"解析→判定→连接同一个 IP"的拨接函数。
// 解析出的**每一个** IP 都必须通过判定；任一被拒就整次拒绝（而不是挑一个能连的），
// 否则一个同时有公网 A 记录与内网 AAAA 记录的域名就能绕过检查。
func DialContext(dialer *net.Dialer) func(ctx context.Context, network, addr string) (net.Conn, error) {
	resolver := net.DefaultResolver
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, err
		}
		// 主机名已经是 IP 字面量时不必解析。
		ips := []net.IP{net.ParseIP(host)}
		if ips[0] == nil {
			resolved, err := resolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("目标主机解析失败: %s", host)
			}
			if len(resolved) == 0 {
				return nil, fmt.Errorf("目标主机无解析结果: %s", host)
			}
			ips = make([]net.IP, 0, len(resolved))
			for _, r := range resolved {
				ips = append(ips, r.IP)
			}
		}
		for _, ip := range ips {
			if BlockedIP(ip) {
				return nil, fmt.Errorf("internal address access denied: %s", ip.String())
			}
		}
		// 钉住第一个通过判定的 IP：不回头用 host 再解析一次，就是把 TOCTOU 关掉的那一步。
		return dialer.DialContext(ctx, network, net.JoinHostPort(ips[0].String(), port))
	}
}

// Client 组装一个带完整防护的出网客户端：
// 请求前查 scheme/主机名、连接时查 IP 并钉住、每一跳重定向复查。
// 重定向必须复查 —— 公网站点 302 到 127.0.0.1 是最省事的一条打内网的路线。
func Client(timeout time.Duration, maxRedirects int) *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		DialContext: DialContext(dialer),
	}
	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if maxRedirects > 0 && len(via) >= maxRedirects {
				return fmt.Errorf("重定向次数过多（上限 %d）", maxRedirects)
			}
			return CheckURL(req.URL.String())
		},
	}
}
