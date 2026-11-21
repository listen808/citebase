package rag

import (
	"context"
	"fmt"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
)

type Translator struct {
	provider provider.Provider
}

func NewTranslator(p provider.Provider) *Translator {
	return &Translator{provider: p}
}

type TranslateRequest struct {
	Text           string            `json:"text"`
	SourceLanguage string            `json:"source_language"`
	TargetLanguage string            `json:"target_language"`
	PreserveFormat bool              `json:"preserve_format"`
	Glossary       map[string]string `json:"glossary"`
}

type TranslateResult struct {
	TranslatedText string `json:"translated_text"`
	SourceLanguage string `json:"source_language"`
	TargetLanguage string `json:"target_language"`
}

func (t *Translator) Translate(ctx context.Context, req TranslateRequest) (*TranslateResult, provider.TokenUsage, error) {
	if t.provider == nil {
		return nil, provider.TokenUsage{}, fmt.Errorf("AI服务未启用")
	}

	if req.Text == "" {
		return nil, provider.TokenUsage{}, fmt.Errorf("文本内容为空")
	}

	if req.TargetLanguage == "" {
		req.TargetLanguage = "en"
	}

	if len(req.Text) > 10000 {
		req.Text = req.Text[:10000] + "..."
	}

	sourceHint := "自动检测"
	if req.SourceLanguage != "" {
		sourceHint = req.SourceLanguage
	}

	glossaryPart := ""
	if len(req.Glossary) > 0 {
		glossaryPart = "\n\n术语表（必须严格使用以下翻译）：\n"
		for k, v := range req.Glossary {
			glossaryPart += fmt.Sprintf("- %s → %s\n", k, v)
		}
	}

	formatHint := ""
	if req.PreserveFormat {
		formatHint = "\n请保留原文的 Markdown 格式、标题层级、列表、代码块等格式。"
	}

	systemPrompt := fmt.Sprintf(`你是一个专业的翻译助手。请将文本从 %s 翻译为 %s。

要求：
1. 翻译准确、自然、流畅
2. 保持专业术语的一致性
3. 不要添加额外解释%s%s
4. 直接输出翻译结果，不要添加前缀或说明`, sourceHint, req.TargetLanguage, formatHint, glossaryPart)

	timeoutCtx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()

	resp, err := t.provider.Chat(timeoutCtx, systemPrompt, []provider.Message{
		provider.NewTextMessage("user", req.Text),
	}, provider.WithMaxTokens(8192), provider.WithTemperature(0.3))
	if err != nil {
		return nil, provider.TokenUsage{}, err
	}

	return &TranslateResult{
		TranslatedText: resp.Content,
		SourceLanguage: req.SourceLanguage,
		TargetLanguage: req.TargetLanguage,
	}, resp.TokenUsage, nil
}

func (t *Translator) TranslateDocument(ctx context.Context, docContent string, targetLang string) (*TranslateResult, provider.TokenUsage, error) {
	return t.Translate(ctx, TranslateRequest{
		Text:           docContent,
		TargetLanguage: targetLang,
		PreserveFormat: true,
	})
}
