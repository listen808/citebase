package tokenizer

import "testing"

func TestNewSimpleTokenizer(t *testing.T) {
	tk := NewSimpleTokenizer()
	if tk == nil {
		t.Fatal("expected non-nil SimpleTokenizer")
	}
}

func TestSimpleTokenizer_CountTokens_Empty(t *testing.T) {
	tk := NewSimpleTokenizer()
	count := tk.CountTokens("")
	if count != 0 {
		t.Errorf("expected 0 tokens for empty string, got %d", count)
	}
}

func TestSimpleTokenizer_CountTokens_ASCII(t *testing.T) {
	tk := NewSimpleTokenizer()
	// 4 ASCII chars = 1 token
	tests := []struct {
		input    string
		expected int
	}{
		{"abcd", 1},
		{"abcdefgh", 2},
		{"hello world", 2}, // 11 chars / 4 = 2
		{"a", 0},           // 1 char / 4 = 0
		{"123", 0},         // 3 chars / 4 = 0
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := tk.CountTokens(tt.input)
			if result != tt.expected {
				t.Errorf("CountTokens(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSimpleTokenizer_CountTokens_Chinese(t *testing.T) {
	tk := NewSimpleTokenizer()
	// 中文：每个 rune 计 2，然后 /4
	// 1 个中文字符 = 2 / 4 = 0
	// 2 个中文字符 = 4 / 4 = 1
	tests := []struct {
		input    string
		expected int
	}{
		{"你", 0},      // 2 / 4 = 0
		{"你好", 1},     // 4 / 4 = 1
		{"你好世界", 2},   // 8 / 4 = 2
		{"中文测试内容", 3}, // 12 / 4 = 3
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := tk.CountTokens(tt.input)
			if result != tt.expected {
				t.Errorf("CountTokens(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

func TestSimpleTokenizer_CountTokens_Mixed(t *testing.T) {
	tk := NewSimpleTokenizer()
	// "hello你好" = 5 ASCII(5) + 2 中文(4) = 9 / 4 = 2
	result := tk.CountTokens("hello你好")
	if result != 2 {
		t.Errorf("expected 2 tokens for mixed, got %d", result)
	}
}

func TestSimpleTokenizer_CountMessagesTokens_Empty(t *testing.T) {
	tk := NewSimpleTokenizer()
	result := tk.CountMessagesTokens(nil)
	if result != 0 {
		t.Errorf("expected 0 for nil messages, got %d", result)
	}

	result = tk.CountMessagesTokens([]MessageForCount{})
	if result != 0 {
		t.Errorf("expected 0 for empty slice, got %d", result)
	}
}

func TestSimpleTokenizer_CountMessagesTokens_Single(t *testing.T) {
	tk := NewSimpleTokenizer()
	msgs := []MessageForCount{
		{Role: "user", Content: "hello"},
	}
	// hello = 5 chars = 1 token + 4 overhead = 5
	result := tk.CountMessagesTokens(msgs)
	if result != 5 {
		t.Errorf("expected 5 tokens, got %d", result)
	}
}

func TestSimpleTokenizer_CountMessagesTokens_Multiple(t *testing.T) {
	tk := NewSimpleTokenizer()
	msgs := []MessageForCount{
		{Role: "system", Content: "you are helpful"}, // 14 chars = 3 + 4 = 7
		{Role: "user", Content: "hi"},                // 2 chars = 0 + 4 = 4
		{Role: "assistant", Content: "hello!"},       // 6 chars = 1 + 4 = 5
	}
	result := tk.CountMessagesTokens(msgs)
	expected := 7 + 4 + 5
	if result != expected {
		t.Errorf("expected %d tokens, got %d", expected, result)
	}
}

func TestSimpleTokenizer_FitsInContext_Empty(t *testing.T) {
	tk := NewSimpleTokenizer()
	ok, used := tk.FitsInContext(nil, 100)
	if !ok {
		t.Error("expected empty messages to fit")
	}
	if used != 0 {
		t.Errorf("expected 0 used, got %d", used)
	}
}

func TestSimpleTokenizer_FitsInContext_Yes(t *testing.T) {
	tk := NewSimpleTokenizer()
	msgs := []MessageForCount{
		{Role: "user", Content: "hi"},
	}
	ok, used := tk.FitsInContext(msgs, 100)
	if !ok {
		t.Error("expected short message to fit")
	}
	if used != 4 {
		// "hi" = 2 chars = 0 + 4 overhead = 4
		t.Errorf("expected 4 used, got %d", used)
	}
}

func TestSimpleTokenizer_FitsInContext_No(t *testing.T) {
	tk := NewSimpleTokenizer()
	msgs := []MessageForCount{
		{Role: "user", Content: "hello world this is a long message"},
	}
	// 33 chars = 8 tokens + 4 overhead = 12
	ok, _ := tk.FitsInContext(msgs, 5)
	if ok {
		t.Error("expected long message to not fit in 5 tokens")
	}
}

func TestSimpleTokenizer_FitsInContext_Edge(t *testing.T) {
	tk := NewSimpleTokenizer()
	msgs := []MessageForCount{
		{Role: "user", Content: "abcd"}, // 4 chars = 1 + 4 = 5
	}
	ok, used := tk.FitsInContext(msgs, 5)
	if ok {
		t.Error("5 < 5 is false, so should not fit exactly at boundary")
	}
	if used != 5 {
		t.Errorf("expected 5 used, got %d", used)
	}
}

func TestMessageForCount_Struct(t *testing.T) {
	msg := MessageForCount{
		Role:    "system",
		Content: "test content",
	}
	if msg.Role != "system" {
		t.Errorf("expected Role='system', got %s", msg.Role)
	}
	if msg.Content != "test content" {
		t.Errorf("expected Content='test content', got %s", msg.Content)
	}
}

func TestSimpleTokenizer_InterfaceCheck(t *testing.T) {
	var tk Tokenizer = NewSimpleTokenizer()
	if tk == nil {
		t.Fatal("expected Tokenizer interface implementation")
	}
	// 验证三个方法都可调用
	count := tk.CountTokens("test")
	if count < 0 {
		t.Error("CountTokens should return non-negative")
	}

	msgs := []MessageForCount{{Role: "user", Content: "hi"}}
	msgCount := tk.CountMessagesTokens(msgs)
	if msgCount <= 0 {
		t.Error("CountMessagesTokens should return positive for non-empty messages")
	}

	ok, used := tk.FitsInContext(msgs, 1000)
	if !ok {
		t.Error("short message should fit in large context")
	}
	if used <= 0 {
		t.Error("used should be positive")
	}
}
