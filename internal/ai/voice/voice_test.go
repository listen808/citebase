package voice

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/listen808/citebase/internal/utils/config"
)

// 临时覆盖 loadConfig 不便，这里直接用构造器并打桩 config。
// 为隔离外部配置，本测试用 setenv 注入 AI_API_KEY / AI_BASE_URL。

func TestTranscribeAndSynthesize(t *testing.T) {
	// 启动假 audio 服务器
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/audio/transcriptions":
			_ = r.ParseMultipartForm(10 << 20)
			// 回显一个 JSON 文本
			w.Header().Set("Content-Type", "application/json")
			io.WriteString(w, `{"text":"你好世界"}`)
		case "/audio/speech":
			w.Header().Set("Content-Type", "audio/mpeg")
			w.Write([]byte("FAKE_MP3_BYTES"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()

	os.Setenv("AI_API_KEY", "test-key")
	os.Setenv("AI_BASE_URL", srv.URL)
	defer os.Unsetenv("AI_API_KEY")
	defer os.Unsetenv("AI_BASE_URL")

	tr, err := NewTranscriber()
	if err != nil {
		t.Fatalf("NewTranscriber: %v", err)
	}
	text, err := tr.Transcribe(context.Background(), []byte("dummy audio"), "test.webm")
	if err != nil {
		t.Fatalf("Transcribe error: %v", err)
	}
	if text != "你好世界" {
		t.Fatalf("unexpected transcript: %q", text)
	}

	syn, err := NewSynthesizer()
	if err != nil {
		t.Fatalf("NewSynthesizer: %v", err)
	}
	audio, err := syn.Synthesize(context.Background(), "朗读这段", "alloy")
	if err != nil {
		t.Fatalf("Synthesize error: %v", err)
	}
	if string(audio) != "FAKE_MP3_BYTES" {
		t.Fatalf("unexpected audio: %q", string(audio))
	}
}

func TestNewTranscriberNoKey(t *testing.T) {
	old, had := os.LookupEnv("AI_API_KEY")
	os.Unsetenv("AI_API_KEY")
	// 同时清空全局 config 的 key（test 可能继承了别处设置的 key）
	prevKey := config.AppConfig.AI.APIKey
	config.AppConfig.AI.APIKey = ""
	defer func() {
		if had {
			os.Setenv("AI_API_KEY", old)
		}
		config.AppConfig.AI.APIKey = prevKey
	}()

	if _, err := NewTranscriber(); err == nil {
		t.Fatal("expected error when API key missing")
	}
}

func TestAudioExt(t *testing.T) {
	cases := map[string]string{
		"a.mp3":     "audio/mpeg",
		"b.wav":     "audio/wav",
		"c.ogg":     "audio/ogg",
		"d.m4a":     "audio/mp4",
		"e.unknown": "application/octet-stream",
	}
	for f, want := range cases {
		if got := AudioExt(f); got != want {
			t.Fatalf("AudioExt(%s)=%s, want %s", f, got, want)
		}
	}
}
