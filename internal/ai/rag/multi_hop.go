package rag

import (
	"context"
	"encoding/json"
	"fmt"
	logx "github.com/listen808/citebase/internal/log"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

type MultiHopReasoner struct {
	provider  provider.Provider
	retriever *Retriever
	maxHops   int
}

func NewMultiHopReasoner(p provider.Provider, r *Retriever) *MultiHopReasoner {
	return &MultiHopReasoner{
		provider:  p,
		retriever: r,
		maxHops:   3,
	}
}

type ReasoningResult struct {
	Answer    string   `json:"answer"`
	Reasoning []string `json:"reasoning"`
	Sources   []Source `json:"sources"`
	Hops      int      `json:"hops"`
}

func (m *MultiHopReasoner) Reason(ctx context.Context, question string, spaceID int, limit int) (*ReasoningResult, error) {
	if m.provider == nil {
		return nil, fmt.Errorf("Multi-hop provider not initialized")
	}

	_ = limit // 保留参数以维持 API 兼容；当前跳数由 m.maxHops 控制

	var allSources []Source
	var reasoning []string
	seenDocs := make(map[int]bool)
	currentQuestion := question

	for hop := 0; hop < m.maxHops; hop++ {
		logx.Infof("[MultiHop] Hop %d: %s", hop+1, currentQuestion)

		sources, _ := m.retriever.RetrieveContextWithRAG(ctx, currentQuestion, spaceID, 3)

		newSourceCount := 0
		for _, s := range sources {
			if !seenDocs[s.DocumentID] {
				seenDocs[s.DocumentID] = true
				allSources = append(allSources, s)
				newSourceCount++
			}
		}

		reasoning = append(reasoning, fmt.Sprintf("Hop %d: 检索了 %d 个新文档", hop+1, newSourceCount))

		shouldContinue, nextQuestion, err := m.evaluateAndPlanNext(ctx, question, allSources, reasoning)
		if err != nil {
			logx.Infof("[MultiHop] Hop %d evaluation error: %v", hop+1, err)
			break
		}
		if !shouldContinue {
			break
		}

		currentQuestion = nextQuestion
	}

	if len(allSources) == 0 {
		return nil, fmt.Errorf("no sources found after %d hops", m.maxHops)
	}

	answer, err := m.generateFinalAnswer(ctx, question, allSources, reasoning)
	if err != nil {
		return nil, fmt.Errorf("generate final answer: %w", err)
	}

	return &ReasoningResult{
		Answer:    answer,
		Reasoning: reasoning,
		Sources:   allSources,
		Hops:      len(reasoning),
	}, nil
}

func (m *MultiHopReasoner) evaluateAndPlanNext(ctx context.Context, originalQuestion string, sources []Source, reasoning []string) (bool, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	var sourceSummaries []string
	for i, s := range sources {
		snippet := s.Snippet
		runes := []rune(snippet)
		if len(runes) > 200 {
			snippet = string(runes[:200]) + "..."
		}
		sourceSummaries = append(sourceSummaries, fmt.Sprintf("[Source %d] %s: %s", i+1, s.DocumentName, snippet))
	}

	prompt := `Based on the collected information, determine if you can fully answer the original question.
If information is insufficient, generate a supplementary retrieval query.

Original question: ` + originalQuestion + `

Collected information:
` + strings.Join(sourceSummaries, "\n") + `

Reasoning steps performed: ` + strings.Join(reasoning, " -> ") + `

Output JSON format:
{
  "should_continue": true/false,
  "next_query": "supplementary retrieval query (if needed)",
  "reason": "reason for judgment"
}`

	resp, err := m.provider.Chat(ctx, "You are a reasoning planning expert. Output only JSON.", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(512), provider.WithTemperature(0.2))
	if err != nil {
		return false, "", err
	}

	var result struct {
		ShouldContinue bool   `json:"should_continue"`
		NextQuery      string `json:"next_query"`
		Reason         string `json:"reason"`
	}

	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return false, "", err
	}

	return result.ShouldContinue, result.NextQuery, nil
}

func (m *MultiHopReasoner) generateFinalAnswer(ctx context.Context, question string, sources []Source, reasoning []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var sourceTexts []string
	for i, s := range sources {
		snippet := s.Snippet
		runes := []rune(snippet)
		if len(runes) > 500 {
			snippet = string(runes[:500]) + "..."
		}
		sourceTexts = append(sourceTexts, fmt.Sprintf("[Source %d] %s: %s", i+1, s.DocumentName, snippet))
	}

	prompt := `Please provide a complete answer based on the information collected through multiple rounds of retrieval.

Question: ` + question + `

Collected information:
` + strings.Join(sourceTexts, "\n\n") + `

Reasoning process: ` + strings.Join(reasoning, " -> ") + `

Please provide a complete and accurate answer, citing sources where appropriate.`

	resp, err := m.provider.Chat(ctx, "You are a knowledge base AI assistant skilled at synthesizing information from multiple sources to answer complex questions.", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(2048), provider.WithTemperature(0.3))
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

// combineSources 合并多个来源
