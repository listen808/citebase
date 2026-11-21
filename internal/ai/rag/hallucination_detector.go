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

type HallucinationDetector struct {
	provider            provider.Provider
	confidenceThreshold float64
}

// NewHallucinationDetector 创建幻觉检测器
// threshold: 高置信度幻觉判定阈值（0-1），<=0 时使用默认 0.7
func NewHallucinationDetector(p provider.Provider) *HallucinationDetector {
	return &HallucinationDetector{
		provider:            p,
		confidenceThreshold: 0.7,
	}
}

// SetConfidenceThreshold 设置高置信度幻觉判定阈值
func (d *HallucinationDetector) SetConfidenceThreshold(t float64) {
	if t > 0 && t <= 1 {
		d.confidenceThreshold = t
	}
}

type HallucinationReport struct {
	HasHallucination   bool     `json:"has_hallucination"`
	HallucinatedClaims []string `json:"hallucinated_claims,omitempty"`
	SupportedClaims    []string `json:"supported_claims,omitempty"`
	Confidence         float64  `json:"confidence"`
	Explanation        string   `json:"explanation,omitempty"`
}

func (d *HallucinationDetector) DetectHallucination(ctx context.Context, answer string, sources []Source) (*HallucinationReport, error) {
	if d.provider == nil {
		return nil, fmt.Errorf("hallucination detector not initialized")
	}

	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()

	var sourceTexts []string
	for i, src := range sources {
		snippet := src.Snippet
		runes := []rune(snippet)
		if len(runes) > 300 {
			snippet = string(runes[:300]) + "..."
		}
		sourceTexts = append(sourceTexts, fmt.Sprintf("[来源%d] %s: %s", i+1, src.DocumentName, snippet))
	}
	sourcesContext := strings.Join(sourceTexts, "\n\n")

	prompt := `请检查以下回答是否存在幻觉（与源文档不一致的内容）。

回答内容：
` + answer + `

参考文档：
` + sourcesContext + `

请分析：
1. 回答中的每个主要声明是否都能在源文档中找到支持
2. 是否有添加了源文档中不存在的信息
3. 是否有曲解或错误引用源文档的内容

输出JSON格式：
{
  "has_hallucination": true/false,
  "hallucinated_claims": ["声明1", "声明2"],
  "supported_claims": ["声明3"],
  "confidence": 0.9,
  "explanation": "检测说明"
}`

	resp, err := d.provider.Chat(ctx, "你是一个事实核查专家，擅长检测AI生成内容中的幻觉。只输出JSON。", []provider.Message{
		provider.NewTextMessage("user", prompt),
	}, provider.WithMaxTokens(1024), provider.WithTemperature(0.1))
	if err != nil {
		return nil, fmt.Errorf("hallucination detection failed: %w", err)
	}

	var report HallucinationReport
	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), &report); err != nil {
		return nil, fmt.Errorf("parse hallucination report failed: %w", err)
	}

	return &report, nil
}

func (d *HallucinationDetector) ValidateAnswer(ctx context.Context, answer string, sources []Source) (string, *HallucinationReport) {
	report, err := d.DetectHallucination(ctx, answer, sources)
	if err != nil {
		logx.Infof("[HallucinationDetector] Detection failed: %v, skipping validation", err)
		return answer, nil
	}

	if report.HasHallucination && len(report.HallucinatedClaims) > 0 {
		logx.Infof("[HallucinationDetector] Detected %d hallucinated claims", len(report.HallucinatedClaims))

		if len(report.HallucinatedClaims) > 2 {
			warning := "\n\n⚠️ 注意：以上回答中部分内容可能不完全准确，建议参考原始文档确认。"
			return answer + warning, report
		}
	}

	return answer, report
}

// ValidateAnswerWithRetry 检测幻觉并在高置信度时返回重试指引
// 返回值：最终答案, 重试指引（非空时需用该指引重新生成）, report
func (d *HallucinationDetector) ValidateAnswerWithRetry(ctx context.Context, answer string, sources []Source) (string, string, *HallucinationReport) {
	report, err := d.DetectHallucination(ctx, answer, sources)
	if err != nil {
		logx.Infof("[HallucinationDetector] Detection failed: %v, skipping validation", err)
		return answer, "", nil
	}

	if report.HasHallucination && len(report.HallucinatedClaims) > 0 {
		logx.Infof("[HallucinationDetector] Detected %d hallucinated claims (confidence=%.2f)", len(report.HallucinatedClaims), report.Confidence)

		// 高置信度幻觉：返回重试指引，由调用方重新生成
		if report.Confidence > d.confidenceThreshold {
			var claimLines []string
			for _, claim := range report.HallucinatedClaims {
				claimLines = append(claimLines, "- "+claim)
			}
			retryGuidance := "## 避免以下幻觉声明\n以下内容在源文档中找不到支持，请重新生成时避免：\n" +
				strings.Join(claimLines, "\n") +
				"\n\n请严格基于源文档重新回答。"
			return answer, retryGuidance, report
		}

		// 低置信度但声明较多：仅追加警告，不重试
		if len(report.HallucinatedClaims) > 2 {
			warning := "\n\n⚠️ 注意：以上回答中部分内容可能不完全准确，建议参考原始文档确认。"
			return answer + warning, "", report
		}
	}

	return answer, "", report
}
