package tokenizer

// Tokenizer 定义 token 计数接口
type Tokenizer interface {
	CountTokens(text string) int
	CountMessagesTokens(messages []MessageForCount) int
	FitsInContext(messages []MessageForCount, maxTokens int) (bool, int)
}

// MessageForCount 用于 token 计数的消息结构
type MessageForCount struct {
	Role    string
	Content string
}

// SimpleTokenizer 基于字符估算的简单 tokenizer
// 规则：ASCII 字符 1 token/4 chars，非 ASCII（中文等）2 runes/1 token
type SimpleTokenizer struct{}

// NewSimpleTokenizer 创建简单 tokenizer 实例
func NewSimpleTokenizer() *SimpleTokenizer {
	return &SimpleTokenizer{}
}

// CountTokens 估算文本的 token 数
func (t *SimpleTokenizer) CountTokens(text string) int {
	count := 0
	for _, r := range text {
		if r > 0x7F {
			count += 2
		} else {
			count += 1
		}
	}
	return count / 4
}

// CountMessagesTokens 估算消息列表的 token 数（每条消息额外 +4 for role/separator overhead）
func (t *SimpleTokenizer) CountMessagesTokens(messages []MessageForCount) int {
	total := 0
	for _, msg := range messages {
		total += t.CountTokens(msg.Content) + 4
	}
	return total
}

// FitsInContext 检查消息列表是否在上下文窗口内
func (t *SimpleTokenizer) FitsInContext(messages []MessageForCount, maxTokens int) (bool, int) {
	used := t.CountMessagesTokens(messages)
	return used < maxTokens, used
}
