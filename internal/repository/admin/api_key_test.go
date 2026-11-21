package admin

import (
	"strings"
	"testing"

	"github.com/listen808/citebase/internal/utils"
)

func TestGenerateRawKeyFormat(t *testing.T) {
	raw, err := generateRawKey()
	if err != nil {
		t.Fatalf("generateRawKey: %v", err)
	}
	if !strings.HasPrefix(raw, "mwk_") {
		t.Fatalf("key should start with mwk_, got %q", raw)
	}
	if len(raw) != 4+48 { // "mwk_" + 24 bytes hex
		t.Fatalf("unexpected key length: %d (%q)", len(raw), raw)
	}
	// 两次生成应不同
	raw2, _ := generateRawKey()
	if raw == raw2 {
		t.Fatal("generated keys should be unique")
	}
}

func TestHashKeyStable(t *testing.T) {
	a := hashKey("mwk_abc")
	b := hashKey("mwk_abc")
	if a != b {
		t.Fatal("hashKey not stable")
	}
	if hashKey("mwk_abc") == hashKey("mwk_other") {
		t.Fatal("different keys should hash differently")
	}
}

func TestKeyEncryptRoundtrip(t *testing.T) {
	raw, err := generateRawKey()
	if err != nil {
		t.Fatalf("generateRawKey: %v", err)
	}
	enc, err := utils.Encrypt(raw)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	dec, err := utils.Decrypt(enc)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if dec != raw {
		t.Fatalf("roundtrip mismatch: %q != %q", dec, raw)
	}
}
