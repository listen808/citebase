package netguard

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestBlockedIPTiers(t *testing.T) {
	cases := []struct {
		ip   string
		want bool
		why  string
	}{
		{"127.0.0.1", true, "IPv4 环回"},
		{"::1", true, "IPv6 环回"},
		{"10.0.0.1", true, "私网 A 段"},
		{"172.16.0.1", true, "私网 B 段"},
		{"192.168.1.1", true, "私网 C 段"},
		{"169.254.169.254", true, "AWS/GCP 元数据"},
		{"169.254.169.253", true, "GCP 元数据备用"},
		{"169.254.1.1", true, "链路本地整段"},
		{"0.0.0.0", true, "未指定地址"},
		{"224.0.0.1", true, "组播"},
		{"fc00::1", true, "IPv6 唯一本地地址"},
		{"93.184.216.34", false, "公网地址必须放行，否则抓取器全废"},
		{"2606:4700:4700::1111", false, "公网 IPv6 同上"},
	}
	for _, c := range cases {
		ip := net.ParseIP(c.ip)
		if ip == nil {
			t.Fatalf("%s 不是合法 IP 字面量（测试写错了）", c.ip)
		}
		if got := BlockedIP(ip); got != c.want {
			t.Errorf("BlockedIP(%s) = %v，期望 %v（%s）", c.ip, got, c.want, c.why)
		}
	}
	if !BlockedIP(nil) {
		t.Error("BlockedIP(nil) 必须为 true：拿不到地址不能算放行")
	}
}

func TestCheckURLSchemes(t *testing.T) {
	for _, raw := range []string{
		"file:///etc/passwd",
		"gopher://127.0.0.1:6379/_SET",
		"http://localhost:3001/api/v1/users",
		"http://127.0.0.1/x",
		"http://169.254.169.254/latest/meta-data/",
		"http://[::1]/x",
		"http://10.1.2.3/x",
		"http:///nopath",
		"http://metadata.google.internal/x",
		"HTTP://LOCALHOST/x",
	} {
		err := CheckURL(raw)
		if err == nil {
			t.Errorf("CheckURL(%q) 应当拒绝，却放行了", raw)
			continue
		}
		// 大小写不能改变结论，但错误文案里应当带上真实输入，便于排障。
		if strings.Contains(err.Error(), "TODO") {
			t.Errorf("错误文案含 TODO：%v", err)
		}
	}

	for _, raw := range []string{
		"https://example.com/doc",
		"http://93.184.216.34/doc",
	} {
		if err := CheckURL(raw); err != nil {
			t.Errorf("CheckURL(%q) 不该拒绝公网地址: %v", raw, err)
		}
	}
}

// TestDialContextRefusesLiteralPrivateTargetsWithoutDNS 钉住"钉 IP"这条路最直接的性质：
// 目标写成 IP 字面量时不必解析、也连不上，且整个过程没有第二次解析机会。
func TestDialContextRefusesLiteralPrivateTargetsWithoutDNS(t *testing.T) {
	dialer := &net.Dialer{Timeout: 200 * time.Millisecond}
	dial := DialContext(dialer)
	ctx := context.Background()

	for _, addr := range []string{"127.0.0.1:80", "10.0.0.1:80", "169.254.169.254:80", "[::1]:80"} {
		conn, err := dial(ctx, "tcp", addr)
		if err == nil {
			if conn != nil {
				_ = conn.Close()
			}
			t.Errorf("拨 %s 竟然成功 —— 内网地址必须在连接前就拒掉", addr)
			continue
		}
		if !strings.Contains(err.Error(), "internal address access denied") {
			t.Errorf("拨 %s 的失败原因不对，期望地址拒绝，实得 %v", addr, err)
		}
	}
}

// TestClientHasRedirectGuard 是"每一跳复查"这条性质的最小可证形式：
// 3xx 打到内网时，CheckRedirect 必须报错而不是继续跟。
func TestClientHasRedirectGuard(t *testing.T) {
	c := Client(2*time.Second, 5)
	if c.Transport == nil {
		t.Fatal("Client 必须自带 Transport：裸 http.Client 没有连接期地址校验")
	}
	if c.CheckRedirect == nil {
		t.Fatal("Client 必须自带 CheckRedirect：不复查跳转就能被公网站点 302 到 127.0.0.1")
	}

	redirectTargets := []string{
		"http://127.0.0.1:9/x",
		"http://169.254.169.254/latest/meta-data/",
		"file:///etc/passwd",
		"http://localhost:3001/x",
	}
	for _, target := range redirectTargets {
		req, err := http.NewRequest(http.MethodGet, target, nil)
		if err != nil {
			t.Fatalf("构造 %s 的请求失败: %v", target, err)
		}
		if err := c.CheckRedirect(req, []*http.Request{req}); err == nil {
			t.Errorf("跳转到 %s 应当被拒绝，却放行了", target)
		}
	}

	// 跳转次数上限同样要有：没上限就是无限跟跳。
	req, err := http.NewRequest(http.MethodGet, "https://example.com/x", nil)
	if err != nil {
		t.Fatal(err)
	}
	via := make([]*http.Request, 5)
	for i := range via {
		via[i] = req
	}
	if err := c.CheckRedirect(req, via); err == nil {
		t.Error("达到 maxRedirects 后应当拒绝继续跳转")
	}
}
