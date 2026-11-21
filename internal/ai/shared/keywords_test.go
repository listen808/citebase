package shared

import "testing"

func TestExtractKeywords(t *testing.T) {
	text := "SELECT user_name, email FROM users WHERE status = 'active'"
	keywords := ExtractKeywords(text, 10, 2, 20)

	if len(keywords) == 0 {
		t.Error("expected some keywords")
	}

	found := make(map[string]bool)
	for _, k := range keywords {
		found[k] = true
	}

	// from 被停用词过滤，其余都应出现
	for _, want := range []string{"user_name", "email", "select", "users", "where", "status", "active"} {
		if !found[want] {
			t.Errorf("expected %q in keywords, got %v", want, keywords)
		}
	}
}

func TestExtractChineseKeywords(t *testing.T) {
	text := "查询所有活跃用户的姓名和邮箱"
	keywords := ExtractKeywords(text, 10, 2, 20)

	if len(keywords) == 0 {
		t.Error("expected some Chinese keywords")
	}
}

func TestExtractChineseNgrams(t *testing.T) {
	text := "查询用户信息"
	words := extractChineseNgrams(text, 2, 3)
	if len(words) == 0 {
		t.Error("expected some Chinese n-grams")
	}

	found := make(map[string]bool)
	for _, w := range words {
		found[w] = true
	}

	for _, want := range []string{"查询", "用户", "信息"} {
		if !found[want] {
			t.Errorf("expected %q in n-grams, got %v", want, words)
		}
	}
}

func TestExtractEnglishWords(t *testing.T) {
	text := "SELECT user_name FROM users"
	words := extractEnglishWords(text, 2, 20)
	if len(words) < 2 {
		t.Errorf("expected at least 2 English words, got %d: %v", len(words), words)
	}

	found := make(map[string]bool)
	for _, w := range words {
		found[w] = true
	}
	for _, want := range []string{"SELECT", "user_name", "FROM", "users"} {
		if !found[want] {
			t.Errorf("expected %q in words, got %v", want, words)
		}
	}
}

func TestExtractKeywords_EmptyText(t *testing.T) {
	keywords := ExtractKeywords("", 10, 2, 20)
	if len(keywords) != 0 {
		t.Errorf("expected 0 keywords for empty text, got %d", len(keywords))
	}
}

func TestExtractKeywords_MaxKeywords(t *testing.T) {
	text := "一 二 三 四 五 六 七 八 九 十"
	keywords := ExtractKeywords(text, 3, 1, 20)
	if len(keywords) > 3 {
		t.Errorf("expected at most 3 keywords, got %d: %v", len(keywords), keywords)
	}
}

func TestExtractKeywords_StopWordsFiltered(t *testing.T) {
	text := "的 是 在 有 用户 查询"
	keywords := ExtractKeywords(text, 10, 1, 20)
	for _, k := range keywords {
		if StopWords[k] {
			t.Errorf("stop word %q should be filtered", k)
		}
	}

	found := make(map[string]bool)
	for _, k := range keywords {
		found[k] = true
	}
	for _, want := range []string{"用户", "查询"} {
		if !found[want] {
			t.Errorf("expected %q in keywords, got %v", want, keywords)
		}
	}
}

func TestExtractKeywords_MinGreaterThanMax(t *testing.T) {
	text := "查询用户信息"
	keywords := ExtractKeywords(text, 10, 10, 2)
	if len(keywords) != 0 {
		t.Errorf("expected 0 keywords when minLen > maxLen, got %d", len(keywords))
	}
}

func TestExtractKeywords_MixedChineseEnglish(t *testing.T) {
	text := "查询 SELECT 语句用于获取用户 user_name 数据"
	keywords := ExtractKeywords(text, 50, 2, 20)
	if len(keywords) == 0 {
		t.Error("expected some keywords from mixed text")
	}

	found := make(map[string]bool)
	for _, k := range keywords {
		found[k] = true
	}
	if !found["查询"] {
		t.Errorf("expected Chinese keyword '查询', got %v", keywords)
	}
	if !found["user_name"] {
		t.Errorf("expected English keyword 'user_name', got %v", keywords)
	}
}
