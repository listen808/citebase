package ai

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/listen808/citebase/internal/ai/voice"
	"github.com/listen808/citebase/internal/log"
	"github.com/listen808/citebase/internal/response"
)

// maxVoiceFileSize 语音文件最大 25MB（与 OpenAI Whisper API 限制一致）
const maxVoiceFileSize = 25 * 1024 * 1024

// allowedAudioExtensions 允许的音频文件扩展名
var allowedAudioExtensions = map[string]bool{
	".mp3": true, ".wav": true, ".m4a": true, ".webm": true,
	".ogg": true, ".flac": true, ".aac": true,
}

// VoiceTranscribe 语音转文本：接收 multipart 音频，返回转写文本。
func (api *ChatAPI) VoiceTranscribe(c *gin.Context) {
	file, err := c.FormFile("audio")
	if err != nil {
		response.BadRequestKey(c, response.MsgAudioMissingField)
		return
	}
	// 文件大小限制
	if file.Size > maxVoiceFileSize {
		response.BadRequest(c, fmt.Sprintf("音频文件过大，最大支持 %d MB", maxVoiceFileSize/1024/1024))
		return
	}
	// 文件格式校验
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if !allowedAudioExtensions[ext] {
		response.BadRequest(c, fmt.Sprintf("不支持的音频格式 %s，支持: mp3/wav/m4a/webm/ogg/flac/aac", ext))
		return
	}

	f, err := file.Open()
	if err != nil {
		response.ServerErrorKey(c, response.MsgAudioOpenFailed)
		return
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		response.ServerErrorKey(c, response.MsgAudioReadFailed)
		return
	}

	tr, err := voice.NewTranscriber()
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	text, err := tr.Transcribe(c.Request.Context(), data, file.Filename)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	response.Success(c, gin.H{"text": text})
}

// VoiceSynthesize 文本转语音：接收 JSON {text, voice}，返回音频字节（mp3）。
func (api *ChatAPI) VoiceSynthesize(c *gin.Context) {
	var req struct {
		Text  string `json:"text" binding:"required"`
		Voice string `json:"voice"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "text is required")
		return
	}

	syn, err := voice.NewSynthesizer()
	if err != nil {
		log.Errorf("[voice] synthesize init failed: %v", err)
		response.ServerErrorSafe(c, err)
		return
	}
	audio, err := syn.Synthesize(c.Request.Context(), req.Text, req.Voice)
	if err != nil {
		response.ServerErrorSafe(c, err)
		return
	}
	c.Header("Content-Type", "audio/mpeg")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"speech.mp3\""))
	c.Data(http.StatusOK, "audio/mpeg", audio)
}
