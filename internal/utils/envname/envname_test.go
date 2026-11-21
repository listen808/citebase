package envname

import (
	"os"
	"testing"
)

func TestGetenvPrecedence(t *testing.T) {
	t.Setenv("CITEBASE_PROBE_DSN", "新名给的")
	t.Setenv("AIWIKI_PROBE_DSN", "旧名给的")

	if got := Getenv("CITEBASE_PROBE_DSN", "AIWIKI_PROBE_DSN"); got != "新名给的" {
		t.Errorf("两个都设时应当新名优先，实得 %q", got)
	}
}

func TestGetenvLegacyFallback(t *testing.T) {
	// 这条是双读存在的全部理由：现存部署只有旧名，改名后必须继续工作。
	os.Unsetenv("CITEBASE_PROBE_DSN")
	t.Setenv("AIWIKI_PROBE_DSN", "旧名给的")

	if got := Getenv("CITEBASE_PROBE_DSN", "AIWIKI_PROBE_DSN"); got != "旧名给的" {
		t.Errorf("只设旧名时应当兜底，实得 %q", got)
	}
}

func TestGetenvEmptyIsNotAFallback(t *testing.T) {
	// 空串不算"设过"。否则 `.env` 里一行 CITEBASE_ENCRYPT_KEY=（留空等人工填）
	// 会把旧名里已经配好的值盖掉，表现是"密钥忽然不生效"。
	t.Setenv("CITEBASE_PROBE_KEY", "")
	t.Setenv("AIWIKI_PROBE_KEY", "旧名给的")

	if got := Getenv("CITEBASE_PROBE_KEY", "AIWIKI_PROBE_KEY"); got != "旧名给的" {
		t.Errorf("新名是空值时不应截断兜底，实得 %q", got)
	}
}

func TestGetenvNoArgs(t *testing.T) {
	if got := Getenv(); got != "" {
		t.Errorf("不传名字应当返回空串而不是 panic，实得 %q", got)
	}
}
