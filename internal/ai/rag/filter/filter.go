package filter

import "context"

type ChatMessage struct {
	Question string `json:"question"`
	Answer   string `json:"answer"`
}

type ConversationFilter interface {
	Name() string
	Filter(ctx context.Context, messages []ChatMessage) []ChatMessage
}
