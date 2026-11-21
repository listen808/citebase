package auth

import (
	"strings"
	"testing"

	adminmodel "github.com/listen808/citebase/internal/model/admin"
)

// initProvider 失败时原先只 log 一条 warning，CreateProvider/UpdateProvider 照样返回
// 成功——管理员看到"保存成功"，实际登录页根本不会出现该入口。
// 这里验证失败原因确实被记进运行态、能被管理端读到，且修好后会清除。
//
// 用"手工端点且缺 auth_url"这条分支制造失败：不触网，所以本机没有外部 OIDC
// 服务也能跑；成功分支同理（走手工端点就不做服务发现）。
func TestInitProviderRecordsFailureReason(t *testing.T) {
	s := NewOIDCService()
	cfg := &adminmodel.OIDCProvider{ID: 7, Name: "broken-idp", ClientID: "cid"}

	err := s.initProvider(cfg)
	if err == nil {
		t.Fatal("缺少 issuer_url 与 auth_url/token_url 时初始化应当失败")
	}

	got := s.InitError(7)
	if got == "" {
		t.Fatal("初始化失败必须被记录，否则管理端只能看到「保存成功」")
	}
	if !strings.Contains(got, "broken-idp") {
		t.Errorf("记录的原因应指明是哪个 provider, got %q", got)
	}
	if got != err.Error() {
		t.Errorf("记录的原因应与返回给调用方的错误一致: got %q, want %q", got, err.Error())
	}

	// 补齐手工端点后成功，旧失败原因必须清除，不能一直挂着"初始化失败"
	cfg.AuthURL = "https://idp.example.com/authorize"
	cfg.TokenURL = "https://idp.example.com/token"
	if err2 := s.initProvider(cfg); err2 != nil {
		t.Fatalf("手工端点齐备时不应失败: %v", err2)
	}
	if still := s.InitError(7); still != "" {
		t.Errorf("初始化成功后应清除失败原因, got %q", still)
	}
}
