package connector

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	config "github.com/listen808/citebase/internal/utils/config"
)

// MinerU 是可选的复杂版面 PDF 解析后端（表格/多栏/公式保留为结构化 Markdown）。
// 未配置 endpoint 时完全不启用；调用失败时自动回退内置解析器 ——
// 解析是导入链路的一环，绝不能因为外部服务抖动而让导入失败。

const mineruTimeout = 120 * time.Second
const mineruMaxBytes = 50 << 20 // 50MB

// mineruEndpoint 返回配置的 MinerU 服务地址；空串 = 未启用。
func mineruEndpoint() string {
	cfg := config.AppConfig
	if cfg.AI.MinerUEndpoint == "" {
		return ""
	}
	return strings.TrimRight(cfg.AI.MinerUEndpoint, "/")
}

// parsePDFWithMinerU 调用 MinerU 兼容的 /file_parse 接口解析 PDF。
// 兼容多种部署形态的响应：JSON 里的 markdown / md_content 字段，
// 或纯文本响应。
func parsePDFWithMinerU(endpoint, filename string, data []byte) (string, error) {
	if len(data) > mineruMaxBytes {
		return "", fmt.Errorf("pdf too large for mineru: %d bytes", len(data))
	}

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, err := mw.CreateFormFile("files", filename)
	if err != nil {
		return "", err
	}
	if _, err := fw.Write(data); err != nil {
		return "", err
	}
	if err := mw.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint+"/file_parse", &buf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	client := &http.Client{Timeout: mineruTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("mineru request: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, mineruMaxBytes))
	if err != nil {
		return "", fmt.Errorf("mineru read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("mineru status %d", resp.StatusCode)
	}

	// 先尝试 JSON（files → 数组，每项含 md_content / markdown）
	var parsed struct {
		Files []struct {
			MDContent string `json:"md_content"`
			Markdown  string `json:"markdown"`
		} `json:"files"`
		MDContent string `json:"md_content"`
		Markdown  string `json:"markdown"`
		Content   string `json:"content"`
	}
	if json.Unmarshal(body, &parsed) == nil {
		if len(parsed.Files) > 0 {
			if s := strings.TrimSpace(parsed.Files[0].MDContent); s != "" {
				return s, nil
			}
			if s := strings.TrimSpace(parsed.Files[0].Markdown); s != "" {
				return s, nil
			}
		}
		for _, s := range []string{parsed.MDContent, parsed.Markdown, parsed.Content} {
			if s = strings.TrimSpace(s); s != "" {
				return s, nil
			}
		}
	}
	// 纯文本响应
	if s := strings.TrimSpace(string(body)); s != "" {
		return s, nil
	}
	return "", fmt.Errorf("mineru returned empty content")
}
