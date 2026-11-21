package utils

import "testing"

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plain := "secret-value-123"
	ct, err := Encrypt(plain)
	if err != nil {
		t.Fatalf("Encrypt err: %v", err)
	}
	if ct == plain {
		t.Fatal("ciphertext should not equal plaintext")
	}
	pt, err := Decrypt(ct)
	if err != nil {
		t.Fatalf("Decrypt err: %v", err)
	}
	if pt != plain {
		t.Fatalf("roundtrip mismatch: %q != %q", pt, plain)
	}
}

func TestEncryptEmpty(t *testing.T) {
	ct, err := Encrypt("")
	if err != nil {
		t.Fatalf("Encrypt empty err: %v", err)
	}
	if ct != "" {
		t.Fatalf("expected empty ciphertext, got %q", ct)
	}
	pt, err := Decrypt("")
	if err != nil {
		t.Fatalf("Decrypt empty err: %v", err)
	}
	if pt != "" {
		t.Fatalf("expected empty plaintext, got %q", pt)
	}
}

func TestDecryptInvalid(t *testing.T) {
	if _, err := Decrypt("not-valid-base64!!!"); err == nil {
		t.Fatal("expected error for invalid base64 ciphertext")
	}
}
