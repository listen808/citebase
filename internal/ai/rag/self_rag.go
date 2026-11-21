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

type SelfRAG struct {
	provider  provider.Provider
	retriever *Retriever
}

func NewSelfRAG(p provider.Provider, r *Retriever) *SelfRAG {
	return &SelfRAG{
		provider:  p,
		retriever: r,
	}
}

type ReflectionResult struct {
	NeedsMoreEvidence bool     `json:"needs_more_evidence"`
	GapQuery          string   `json:"gap_query,omitempty"`
	Confidence        float64  `json:"confidence"`
	Issues            []string `json:"issues,omitempty"`
}

func (s *SelfRAG) GenerateWithReflection(ctx context.Context, question string, spaceID int, limit int) (*AIAnswer, error) {
	if s.provider == nil {
		return nil, fmt.Errorf("Self-RAG provider not initialized")
	}

	if limit <= 0 {
		limit = 5
	}

	// 1. 初始检索
	sources, _ := s.retriever.RetrieveContextWithRAG(ctx, question, spaceID, limit)
	if len(sources) == 0 {
		return &AIAnswer{
			Answer:  "未找到相关文档，无法回答该问题。",
			Sources: []Source{},
		}, nil
	}

	// 2. 初始生成
	initialAnswer, err := s.generateInitialAnswer(ctx, question, sources)
	if err != nil {
		return nil, fmt.Errorf("generate initial answer: %w", err)
	}

	// 3. 自我反思
	reflection, err := s.reflectOnAnswer(ctx, question, initialAnswer, sources)
	if err != nil {
		logx.Infof("[Self-RAG] Reflection failed: %v, returning initial answer", err)
		return &AIAnswer{Answer: initialAnswer, Sources: sources}, nil
	}

	// 4. 如果需要更多证据，重新检索并生成
	if reflection.NeedsMoreEvidence && reflection.GapQuery != "" {
		logx.Infof("[Self-RAG] Need more evidence, gap query: %s", reflection.GapQuery)

		additionalSources, _ := s.retriever.RetrieveContextWithRAG(ctx, reflection.GapQuery, spaceID, 3)
		if len(additionalSources) > 0 {
			// 创建新切片避免修改原始数据
			allSources := make([]Source, 0, len(sources)+len(additionalSources))
			allSources = append(allSources, sources...)
			allSources = append(allSources, additionalSources...)

			improvedAnswer, err := s.generateImprovedAnswer(ctx, question, allSources, reflection.Issues)
			if err == nil {
				return &AIAnswer{Answer: improvedAnswer, Sources: allSources}, nil
			}
			logx.Infof("[Self-RAG] Generate improved answer failed: %v, returning initial answer", err)
		}
	}

	return &AIAnswer{Answer: initialAnswer, Sources: sources}, nil
}

func (s *SelfRAG) generateInitialAnswer(ctx context.Context, question string, sources []Source) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	contextText := buildCitedContext(sources)
	systemPrompt := "你是一个知识库AI助手。请基于提供的文档内容回答问题。"

	resp, err := s.provider.Chat(ctx, systemPrompt, []provider.Message{
		provider.NewTextMessage("user", "文档内容：\n"+contextText+"\n\n问题："+question),
	}, provider.WithMaxTokens(2048), provider.WithTemperature(0.3))
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}

func (s *SelfRAG) reflectOnAnswer(ctx context.Context, question, answer string, sources []Source) (*ReflectionResult, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()

	contextText := buildCitedContext(sources)
	prompt := `请评估以下回答是否完整、准确，并判断是否需要更多证据。

问题：` + question + `

回答：` + answer + `

参考文档：` + contextText + `

输出JSON格式：
{
  "needs_more_evidence": true/false,
  "gap_query": "如果需要更多证据，生成补充检索查询",
  "confidence": 0.8,
  "issues": ["问题1", "问题2"]
}`

	resp, err := s.provider.Chat(ctx, "你是一个回答质量评估专家。只输出JSON。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(512), provider.WithTemperature(0.1))
	if err != nil {
		return nil, err
	}

	var result ReflectionResult
	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (s *SelfRAG) generateImprovedAnswer(ctx context.Context, question string, sources []Source, issues []string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	contextText := buildCitedContext(sources)
	issuesText := strings.Join(issues, "、")

	systemPrompt := "你是一个知识库AI助手。请基于提供的文档内容回答问题，注意避免以下问题：" + issuesText

	resp, err := s.provider.Chat(ctx, systemPrompt, []provider.Message{
		provider.NewTextMessage("user", "文档内容：\n"+contextText+"\n\n问题："+question),
	}, provider.WithMaxTokens(2048), provider.WithTemperature(0.3))
	if err != nil {
		return "", err
	}

	return resp.Content, nil
}
