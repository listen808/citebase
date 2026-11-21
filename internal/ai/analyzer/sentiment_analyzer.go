package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/listen808/citebase/internal/log"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

// SentimentAnalyzer 情感分析器 - 判断文档/段落的情感倾向
type SentimentAnalyzer struct {
	provider provider.Provider
}

// NewSentimentAnalyzer 创建情感分析器
func NewSentimentAnalyzer(p provider.Provider) *SentimentAnalyzer {
	return &SentimentAnalyzer{provider: p}
}

// SentimentResult 情感分析结果
type SentimentResult struct {
	OverallSentiment string  `json:"overall_sentiment"` // positive/negative/neutral/mixed
	Confidence       float64 `json:"confidence"`        // 置信度 0-1
	Score            float64 `json:"score"`             // 情感得分 -1.0 到 1.0
	KeySentiments    []struct {
		Text      string  `json:"text"`
		Sentiment string  `json:"sentiment"`
		Score     float64 `json:"score"`
	} `json:"key_sentiments"` // 关键段落的情感
	Emotions []string `json:"emotions"` // 识别到的情绪（如：满意、担忧、紧急等）
}

// AnalyzeSentiment 分析文档情感
func (s *SentimentAnalyzer) AnalyzeSentiment(ctx context.Context, content string) (*SentimentResult, error) {
	if s.provider == nil {
		return nil, fmt.Errorf("provider not configured")
	}

	if len(content) > 4000 {
		content = utils.TruncateRunes(content, 4000)
	}

	prompt := `请分析以下文档的情感倾向和情绪特征。

情感类别：
- positive: 积极正面（如：成功、满意、进展顺利）
- negative: 消极负面（如：失败、问题、风险）
- neutral: 中性客观（如：说明、描述、记录）
- mixed: 混合情感（同时包含正面和负面）

情绪类型（可多选）：
- satisfied: 满意
- concerned: 担忧
- urgent: 紧急
- confident: 自信
- uncertain: 不确定
- enthusiastic: 热情
- frustrated: 沮丧
- neutral: 中性

文本内容：
%s

请以JSON格式输出：
{
  "overall_sentiment": "positive/negative/neutral/mixed",
  "confidence": 0.9,
  "score": 0.5,
  "key_sentiments": [
    {
      "text": "关键段落文本",
      "sentiment": "positive/negative/neutral",
      "score": 0.8
    }
  ],
  "emotions": ["情绪1", "情绪2"]
}

只输出JSON，不要其他内容。`

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := s.provider.Chat(ctx, "你是一个情感分析专家，擅长判断文档的情感倾向和情绪特征。只输出JSON。", []provider.Message{
		provider.NewTextMessage("user", fmt.Sprintf(prompt, content)),
	}, provider.WithMaxTokens(1024), provider.WithTemperature(0.1))
	if err != nil {
		log.Errorf("[SentimentAnalyzer] LLM analyze error: %v", err)
		return nil, err
	}

	result := &SentimentResult{}

	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), result); err != nil {
		log.Errorf("[SentimentAnalyzer] JSON parse error: %v", err)
		return nil, err
	}

	return result, nil
}
