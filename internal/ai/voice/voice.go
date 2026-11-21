// Package voice 提供语音交互能力：语音转文本（STT）与文本转语音（TTS）。
//
// 背景：知识库问答的"语音入口"能显著降低使用门槛（口语提问、朗读回答）。
// 本包基于 OpenAI 兼容的 audio 接口实现（/audio/transcriptions 与 /audio/speech），
// 复用 config.AppConfig.AI 中的 BaseURL / APIKey 配置。未配置时返回明确错误，不崩溃。
package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/utils/config"
)

// Transcriber 语音转文本。
type Transcriber interface {
	Transcribe(ctx context.Context, audio []byte, filename string) (string, error)
}

// Synthesizer 文本转语音。
type Synthesizer interface {
	Synthesize(ctx context.Context, text, voice string) ([]byte, error)
}

// openAIConfig 从全局配置解析出的 audio 端点配置。
type openAIConfig struct {
	BaseURL string
	APIKey  string
	// TranscribeModel STT 模型名（如 whisper-1 / gpt-4o-transcribe）。
	TranscribeModel string
	// SynthesizeModel TTS 模型名（如 tts-1 / gpt-4o-mini-tts）。
	SynthesizeModel string
}

func loadConfig() (openAIConfig, error) {
	// 环境变量优先，回退到全局配置（与项目其他处 env 覆盖文件的惯例一致）。
	apiKey := envOr("AI_API_KEY", config.AppConfig.AI.APIKey)
	base := envOr("AI_BASE_URL", config.AppConfig.AI.BaseURL)
	if apiKey == "" {
		return openAIConfig{}, fmt.Errorf("voice: AI API key 未配置，无法使用语音能力")
	}
	// 拒绝示例/占位符 key（如 sk-xxxx...），避免向本地模型服务误发音频请求。
	if isPlaceholderKey(apiKey) {
		return openAIConfig{}, fmt.Errorf("voice: AI API key 仍为占位符（%q），请在配置中填写真实 key 才能使用语音能力", maskKey(apiKey))
	}
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	return openAIConfig{
		BaseURL:         strings.TrimRight(base, "/"),
		APIKey:          apiKey,
		TranscribeModel: envOr("AI_STT_MODEL", "whisper-1"),
		SynthesizeModel: envOr("AI_TTS_MODEL", "tts-1"),
	}, nil
}

// isPlaceholderKey 判断是否为未填写的示例 key。
func isPlaceholderKey(key string) bool {
	k := strings.TrimSpace(key)
	if strings.HasPrefix(k, "sk-") {
		rest := strings.TrimPrefix(k, "sk-")
		// 占位符通常全为 x / 0 等重复字符。
		if len(rest) > 0 && strings.Trim(rest, "xX0") == "" {
			return true
		}
	}
	return false
}

// maskKey 脱敏显示 key 前后缀，便于日志定位而不泄露完整凭证。
func maskKey(key string) string {
	if len(key) <= 8 {
		return "***"
	}
	return key[:4] + "..." + key[len(key)-4:]
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// OpenAITranscriber 基于 OpenAI 兼容接口的 STT 实现。
type OpenAITranscriber struct {
	cfg openAIConfig
}

// NewTranscriber 用全局配置构造 STT（配置缺失时返回错误）。
func NewTranscriber() (*OpenAITranscriber, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	return &OpenAITranscriber{cfg: cfg}, nil
}

// Transcribe 将音频转写为文本。
func (t *OpenAITranscriber) Transcribe(ctx context.Context, audio []byte, filename string) (string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audio); err != nil {
		return "", err
	}
	_ = writer.WriteField("model", t.cfg.TranscribeModel)
	if err := writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.cfg.BaseURL+"/audio/transcriptions", body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+t.cfg.APIKey)

	return doJSONText(req, "text")
}

// OpenAITTSynthesizer 基于 OpenAI 兼容接口的 TTS 实现。
type OpenAITTSynthesizer struct {
	cfg openAIConfig
}

// NewSynthesizer 用全局配置构造 TTS（配置缺失时返回错误）。
func NewSynthesizer() (*OpenAITTSynthesizer, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}
	return &OpenAITTSynthesizer{cfg: cfg}, nil
}

// Synthesize 将文本合成为音频字节（默认 mp3）。
func (s *OpenAITTSynthesizer) Synthesize(ctx context.Context, text, voice string) ([]byte, error) {
	if voice == "" {
		voice = "alloy"
	}
	payload := fmt.Sprintf(`{"model":%q,"input":%q,"voice":%q,"response_format":"mp3"}`,
		s.cfg.SynthesizeModel, text, voice)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.BaseURL+"/audio/speech", bytes.NewBufferString(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.cfg.APIKey)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("TTS request to %s failed (%d): %s (请确认该 BaseURL 提供 /audio/speech 兼容接口，本地 Ollama 通常不支持 TTS)",
			s.cfg.BaseURL+"/audio/speech", resp.StatusCode, string(b))
	}
	return io.ReadAll(resp.Body)
}

// doJSONText 发送 multipart 请求并解析返回的 JSON 中的指定文本字段。
func doJSONText(req *http.Request, field string) (string, error) {
	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("STT request failed (%d): %s", resp.StatusCode, string(b))
	}
	// 兼容两种返回：{"text":"..."} 或纯文本
	if bytes.HasPrefix(bytes.TrimSpace(b), []byte("{")) {
		var out map[string]interface{}
		if err := json.Unmarshal(b, &out); err != nil {
			return "", fmt.Errorf("parse STT response failed: %w", err)
		}
		if v, ok := out[field].(string); ok {
			return v, nil
		}
		return "", fmt.Errorf("STT response missing %q field", field)
	}
	return string(bytes.TrimSpace(b)), nil
}

// AudioExt 根据文件名返回音频内容类型。
func AudioExt(filename string) string {
	ext := filepath.Ext(filename)
	switch ext {
	case ".mp3":
		return "audio/mpeg"
	case ".wav":
		return "audio/wav"
	case ".ogg":
		return "audio/ogg"
	case ".m4a":
		return "audio/mp4"
	case ".webm":
		return "audio/webm"
	default:
		return "application/octet-stream"
	}
}

// ---------------------------------------------------------------------------
// VoiceRAG — STT → RAG → TTS pipeline
// ---------------------------------------------------------------------------

// VoiceRAGQuestioner defines the interface for asking questions via RAG.
// This is satisfied by rag.Asker or any wrapper that provides Ask/StreamAsk.
type VoiceRAGQuestioner interface {
	Ask(ctx context.Context, question string, spaceID int, limit int) (*VoiceRAGAnswer, error)
}

// VoiceRAGAnswer mirrors the RAG answer structure for the voice pipeline.
type VoiceRAGAnswer struct {
	Answer  string
	Sources []VoiceRAGSource
}

// VoiceRAGSource is a simplified source for voice output.
type VoiceRAGSource struct {
	DocumentName string
	Snippet      string
}

// VoiceRAG bridges speech input → text RAG query → speech output.
// It uses the existing OpenAI STT and TTS implementations, and accepts any
// VoiceRAGQuestioner for the retrieval step.
type VoiceRAG struct {
	transcriber    *OpenAITranscriber
	synthesizer    *OpenAITTSynthesizer
	questioner     VoiceRAGQuestioner
	voice          string // TTS voice name (alloy, echo, fable, onyx, nova, shimmer)
	responseFormat string // "text" or "verbose_json"
}

// VoiceRAGConfig configures the VoiceRAG pipeline.
type VoiceRAGConfig struct {
	Voice          string // TTS voice (default: "alloy")
	ResponseFormat string // STT response format (default: "text")
}

// NewVoiceRAG creates a VoiceRAG pipeline with the given RAG questioner.
// Returns error if STT or TTS cannot be initialized (e.g., missing API key).
func NewVoiceRAG(questioner VoiceRAGQuestioner, cfg VoiceRAGConfig) (*VoiceRAG, error) {
	stt, err := NewTranscriber()
	if err != nil {
		return nil, fmt.Errorf("voice RAG: STT init failed: %w", err)
	}
	tts, err := NewSynthesizer()
	if err != nil {
		return nil, fmt.Errorf("voice RAG: TTS init failed: %w", err)
	}
	if cfg.Voice == "" {
		cfg.Voice = "alloy"
	}
	if cfg.ResponseFormat == "" {
		cfg.ResponseFormat = "text"
	}
	return &VoiceRAG{
		transcriber:    stt,
		synthesizer:    tts,
		questioner:     questioner,
		voice:          cfg.Voice,
		responseFormat: cfg.ResponseFormat,
	}, nil
}

// VoiceRAGResult holds the full pipeline result.
type VoiceRAGResult struct {
	TranscribedText string           `json:"transcribed_text"`
	Answer          string           `json:"answer"`
	Sources         []VoiceRAGSource `json:"sources,omitempty"`
	AudioBytes      []byte           `json:"-"`
	AudioFormat     string           `json:"audio_format,omitempty"`
	Duration        string           `json:"duration,omitempty"`
}

// AskAudio runs the full STT → RAG → TTS pipeline.
// It transcribes the input audio, queries the knowledge base, and synthesizes
// the text answer into speech output.
func (vr *VoiceRAG) AskAudio(ctx context.Context, audio []byte, filename string, spaceID int, limit int) (*VoiceRAGResult, error) {
	start := time.Now()

	// Step 1: STT — transcribe audio to text
	text, err := vr.transcriber.Transcribe(ctx, audio, filename)
	if err != nil {
		return nil, fmt.Errorf("STT failed: %w", err)
	}
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("STT returned empty text")
	}

	// Step 2: RAG — query knowledge base
	if vr.questioner == nil {
		return &VoiceRAGResult{
			TranscribedText: text,
			Answer:          "RAG 服务未配置",
		}, nil
	}

	ragAnswer, err := vr.questioner.Ask(ctx, text, spaceID, limit)
	if err != nil {
		return &VoiceRAGResult{
			TranscribedText: text,
			Answer:          fmt.Sprintf("查询失败: %v", err),
		}, nil
	}

	// Step 3: TTS — synthesize answer to speech
	var audioBytes []byte
	var audioFormat string
	ttsText := ragAnswer.Answer
	// Truncate very long answers for TTS (limit ~4000 chars for TTS)
	runes := []rune(ttsText)
	if len(runes) > 4000 {
		ttsText = string(runes[:4000]) + "，以上为部分内容。"
	}

	audioBytes, err = vr.synthesizer.Synthesize(ctx, ttsText, vr.voice)
	if err != nil {
		// TTS failure is non-fatal — return text answer without audio
		audioFormat = ""
	} else {
		audioFormat = "audio/mpeg"
	}

	result := &VoiceRAGResult{
		TranscribedText: text,
		Answer:          ragAnswer.Answer,
		Sources:         ragAnswer.Sources,
		AudioBytes:      audioBytes,
		AudioFormat:     audioFormat,
		Duration:        time.Since(start).Round(time.Millisecond).String(),
	}

	return result, nil
}

// AskText skips the STT step and runs RAG → TTS directly.
// Useful when the user's text query is already available (e.g., from keyboard input).
func (vr *VoiceRAG) AskText(ctx context.Context, question string, spaceID int, limit int) (*VoiceRAGResult, error) {
	start := time.Now()

	if vr.questioner == nil {
		return &VoiceRAGResult{
			TranscribedText: question,
			Answer:          "RAG 服务未配置",
		}, nil
	}

	ragAnswer, err := vr.questioner.Ask(ctx, question, spaceID, limit)
	if err != nil {
		return &VoiceRAGResult{
			TranscribedText: question,
			Answer:          fmt.Sprintf("查询失败: %v", err),
		}, nil
	}

	// TTS
	var audioBytes []byte
	var audioFormat string
	ttsText := ragAnswer.Answer
	runes := []rune(ttsText)
	if len(runes) > 4000 {
		ttsText = string(runes[:4000]) + "，以上为部分内容。"
	}

	audioBytes, err = vr.synthesizer.Synthesize(ctx, ttsText, vr.voice)
	if err == nil {
		audioFormat = "audio/mpeg"
	}

	return &VoiceRAGResult{
		TranscribedText: question,
		Answer:          ragAnswer.Answer,
		Sources:         ragAnswer.Sources,
		AudioBytes:      audioBytes,
		AudioFormat:     audioFormat,
		Duration:        time.Since(start).Round(time.Millisecond).String(),
	}, nil
}
