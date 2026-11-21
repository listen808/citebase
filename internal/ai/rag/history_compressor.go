package rag

import (
	"context"
	"fmt"
	"strings"

	logx "github.com/listen808/citebase/internal/log"

	"github.com/listen808/citebase/internal/ai/provider"
)

type HistoryCompressor struct {
	provider provider.Provider
}

func NewHistoryCompressor(p provider.Provider) *HistoryCompressor {
	return &HistoryCompressor{provider: p}
}

// SmartCrushRule defines a rule for preserving important messages during compression.
type SmartCrushRule struct {
	Name     string
	Match    func(msg ChatMessage) bool
	Priority int // higher = more important, always kept
}

// DefaultSmartCrushRules returns the standard set of preservation rules.
func DefaultSmartCrushRules() []SmartCrushRule {
	return []SmartCrushRule{
		{
			Name:     "code_block",
			Priority: 100,
			Match: func(msg ChatMessage) bool {
				return strings.Contains(msg.Answer, "```") || strings.Contains(msg.Question, "```")
			},
		},
		{
			Name:     "decision_or_conclusion",
			Priority: 90,
			Match: func(msg ChatMessage) bool {
				lowerA := strings.ToLower(msg.Answer)
				for _, kw := range []string{"结论是", "决定", "最终", "conclusion", "decision", "the answer is"} {
					if strings.Contains(lowerA, kw) {
						return true
					}
				}
				return false
			},
		},
		{
			Name:     "error_or_fix",
			Priority: 85,
			Match: func(msg ChatMessage) bool {
				lowerA := strings.ToLower(msg.Answer)
				for _, kw := range []string{"错误", "修复", "error", "fixed", "bug", "问题已解决"} {
					if strings.Contains(lowerA, kw) {
						return true
					}
				}
				return false
			},
		},
		{
			Name:     "action_item",
			Priority: 80,
			Match: func(msg ChatMessage) bool {
				lowerA := strings.ToLower(msg.Answer)
				for _, kw := range []string{"下一步", "待办", "action", "todo", "需要做", "follow up"} {
					if strings.Contains(lowerA, kw) {
						return true
					}
				}
				return false
			},
		},
		{
			Name:     "user_correction",
			Priority: 75,
			Match: func(msg ChatMessage) bool {
				lowerQ := strings.ToLower(msg.Question)
				for _, kw := range []string{"不对", "错了", "应该是", "不准确", "wrong", "incorrect", "not right"} {
					if strings.Contains(lowerQ, kw) {
						return true
					}
				}
				return false
			},
		},
		{
			Name:     "numbers_or_metrics",
			Priority: 70,
			Match: func(msg ChatMessage) bool {
				count := 0
				for _, ch := range msg.Answer {
					if ch >= '0' && ch <= '9' {
						count++
					}
				}
				return count > 10 // contains significant numeric data
			},
		},
		{
			Name:     "question_answer_pair",
			Priority: 50,
			Match: func(msg ChatMessage) bool {
				return len([]rune(msg.Question)) > 5 && len([]rune(msg.Answer)) > 20
			},
		},
	}
}

// SmartCrushResult is the output of the SmartCrush compression.
type SmartCrushResult struct {
	Preserved  []ChatMessage   `json:"preserved"`
	Compressed []ChatMessage   `json:"compressed"`
	Summary    string          `json:"summary,omitempty"`
	Stats      SmartCrushStats `json:"stats"`
}

// SmartCrushStats holds compression statistics.
type SmartCrushStats struct {
	TotalMessages   int     `json:"total_messages"`
	PreservedCount  int     `json:"preserved_count"`
	CompressedCount int     `json:"compressed_count"`
	PreserveRatio   float64 `json:"preserve_ratio"`
}

// SmartCrush performs rule-based message preservation before LLM summarization.
// It identifies important messages (code, decisions, errors, etc.) and preserves
// them intact, only summarizing the remaining lower-priority messages.
func (hc *HistoryCompressor) SmartCrush(ctx context.Context, history []ChatMessage, maxPreserve int) *SmartCrushResult {
	if maxPreserve <= 0 {
		maxPreserve = 4
	}
	rules := DefaultSmartCrushRules()

	result := &SmartCrushResult{
		Stats: SmartCrushStats{
			TotalMessages: len(history),
		},
	}

	// Always keep the last N messages
	recentCount := 2
	if len(history) <= recentCount {
		result.Preserved = history
		result.Stats.PreservedCount = len(history)
		result.Stats.PreserveRatio = 1.0
		return result
	}

	recent := history[len(history)-recentCount:]
	older := history[:len(history)-recentCount]

	// Score and sort older messages by rule priority
	type scored struct {
		msg   ChatMessage
		score int
		index int
	}
	var scoredMsgs []scored
	for i, msg := range older {
		bestScore := 0
		for _, rule := range rules {
			if rule.Match(msg) && rule.Priority > bestScore {
				bestScore = rule.Priority
			}
		}
		scoredMsgs = append(scoredMsgs, scored{msg: msg, score: bestScore, index: i})
	}

	// Sort by score descending
	for i := 0; i < len(scoredMsgs); i++ {
		for j := i + 1; j < len(scoredMsgs); j++ {
			if scoredMsgs[j].score > scoredMsgs[i].score {
				scoredMsgs[i], scoredMsgs[j] = scoredMsgs[j], scoredMsgs[i]
			}
		}
	}

	// Pick top preserved messages
	preserveCount := maxPreserve
	if preserveCount > len(scoredMsgs) {
		preserveCount = len(scoredMsgs)
	}

	preservedSet := make(map[int]bool)
	var preserved []ChatMessage
	for i := 0; i < preserveCount; i++ {
		if scoredMsgs[i].score > 0 {
			preserved = append(preserved, scoredMsgs[i].msg)
			preservedSet[scoredMsgs[i].index] = true
		}
	}

	// Collect remaining for LLM compression
	var toCompress []ChatMessage
	for i, msg := range older {
		if !preservedSet[i] {
			toCompress = append(toCompress, msg)
		}
	}

	// LLM compression of remaining messages
	var summary string
	if len(toCompress) > 0 && hc.provider != nil {
		summary = hc.summarizeOlderMessages(ctx, toCompress)
	}

	// Build final result: preserved + summary + recent
	result.Preserved = append(preserved, recent...)
	result.Compressed = toCompress
	result.Summary = summary
	result.Stats.PreservedCount = len(preserved) + recentCount
	result.Stats.CompressedCount = len(toCompress)
	if len(history) > 0 {
		result.Stats.PreserveRatio = float64(result.Stats.PreservedCount) / float64(len(history))
	}

	return result
}

// CompressHistory compresses conversation history, using SmartCrush when history is long.
func (hc *HistoryCompressor) CompressHistory(ctx context.Context, history []ChatMessage) []ChatMessage {
	if len(history) <= 8 {
		return history
	}

	if hc.provider == nil {
		if len(history) > 10 {
			return history[len(history)-10:]
		}
		return history
	}

	// Use SmartCrush for intelligent compression
	result := hc.SmartCrush(ctx, history, 4)
	if result == nil || len(result.Preserved) == 0 {
		if len(history) > 10 {
			return history[len(history)-10:]
		}
		return history
	}

	// If SmartCrush produced a summary, include it
	if result.Summary != "" {
		compressed := make([]ChatMessage, 0, len(result.Preserved)+1)
		compressed = append(compressed, ChatMessage{
			Question: "[对话摘要]",
			Answer:   result.Summary,
		})
		compressed = append(compressed, result.Preserved...)
		return compressed
	}

	return result.Preserved
}

// legacy fallback — kept for backward compatibility
func (hc *HistoryCompressor) summarizeOlderMessages(ctx context.Context, messages []ChatMessage) string {
	var sb strings.Builder
	sb.WriteString("以下是之前的对话历史摘要：\n\n")
	for i, msg := range messages {
		sb.WriteString(fmt.Sprintf("Q%d: %s\nA%d: %s\n\n", i+1, msg.Question, i+1, msg.Answer))
	}

	input := sb.String()
	if len(input) > 4000 {
		input = input[:4000]
	}

	prompt := fmt.Sprintf(`请将以下对话历史压缩为一段简洁的摘要（不超过300字），保留关键信息、结论和上下文：

%s`, input)

	resp, err := hc.provider.Chat(ctx, "你是一个对话摘要助手。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(512), provider.WithTemperature(0.3))
	if err != nil {
		logx.Infof("[HistoryCompressor] summarize error: %v", err)
		return ""
	}

	return strings.TrimSpace(resp.Content)
}
