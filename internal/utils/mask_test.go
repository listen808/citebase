package utils

import (
	"testing"
)

func TestMaskSensitiveInfo_Password(t *testing.T) {
	input := `{"username":"admin","password":"secret123"}`
	result := MaskSensitiveInfo(input)
	if result == input {
		t.Error("Expected password to be masked")
	}
	expected := `{"username":"admin","password":"****"}`
	if result != expected {
		t.Errorf("Expected %s, got %s", expected, result)
	}
}

func TestMaskSensitiveInfo_Token(t *testing.T) {
	input := `{"token":"eyJhbGciOiJIUzI1NiJ9.xxx"}`
	result := MaskSensitiveInfo(input)
	if result == input {
		t.Error("Expected token to be masked")
	}
}

func TestMaskSensitiveInfo_MultipleFields(t *testing.T) {
	input := `{"password":"abc","secret":"xyz","username":"admin"}`
	result := MaskSensitiveInfo(input)
	if result == input {
		t.Error("Expected sensitive fields to be masked")
	}
	// username should remain
	if !contains(result, "admin") {
		t.Error("Non-sensitive field 'username' should not be masked")
	}
}

func TestMaskSensitiveInfo_Empty(t *testing.T) {
	result := MaskSensitiveInfo("")
	if result != "" {
		t.Errorf("Expected empty string, got %s", result)
	}
}

func TestMaskSensitiveInfo_NoSensitiveFields(t *testing.T) {
	input := `{"name":"test","value":123}`
	result := MaskSensitiveInfo(input)
	if result != input {
		t.Errorf("Expected unchanged, got %s", result)
	}
}

func TestMaskSensitiveInfo_CaseInsensitive(t *testing.T) {
	input := `{"Password":"secret","TOKEN":"abc"}`
	result := MaskSensitiveInfo(input)
	if result == input {
		t.Error("Expected case-insensitive masking")
	}
}

func TestMaskPII_PhoneNumber(t *testing.T) {
	result := MaskPII("联系电话13812345678")
	if result == "联系电话13812345678" {
		t.Error("Expected phone number to be masked")
	}
}

func TestMaskPII_Email(t *testing.T) {
	result := MaskPII("邮箱test@example.com请回复")
	if result == "邮箱test@example.com请回复" {
		t.Error("Expected email to be masked")
	}
}

func TestMaskPII_Empty(t *testing.T) {
	result := MaskPII("")
	if result != "" {
		t.Errorf("Expected empty string, got %s", result)
	}
}

func TestMaskPII_NoPII(t *testing.T) {
	input := "这是一段普通文本"
	result := MaskPII(input)
	if result != input {
		t.Errorf("Expected unchanged, got %s", result)
	}
}

func TestTruncateRunes(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello", 3, "hel"},
		{"你好世界", 2, "你好"},
		{"你好世界", 10, "你好世界"},
		{"", 5, ""},
	}

	for _, tt := range tests {
		result := TruncateRunes(tt.input, tt.max)
		if result != tt.expected {
			t.Errorf("TruncateRunes(%q, %d) = %q, want %q", tt.input, tt.max, result, tt.expected)
		}
	}
}

func TestTruncateWithEllipsis(t *testing.T) {
	tests := []struct {
		input    string
		max      int
		expected string
	}{
		{"hello", 10, "hello"},
		{"hello world", 5, "hello..."},
		{"你好世界啊", 3, "你好世..."},
		{"short", 10, "short"},
	}

	for _, tt := range tests {
		result := TruncateWithEllipsis(tt.input, tt.max)
		if result != tt.expected {
			t.Errorf("TruncateWithEllipsis(%q, %d) = %q, want %q", tt.input, tt.max, result, tt.expected)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
