package shared

import (
	"context"
	"testing"
)

func TestCompressHistory_NoCompressionNeeded(t *testing.T) {
	history := []ChatMessage{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "hi"},
	}

	// keepRecent >= len(history)，不应压缩
	result, err := CompressHistory(context.Background(), nil, history, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 messages, got %d", len(result))
	}
}

func TestCompressHistory_EmptyHistory(t *testing.T) {
	result, err := CompressHistory(context.Background(), nil, nil, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(result) != 0 {
		t.Errorf("expected 0 messages, got %d", len(result))
	}
}
