package analyzer

import (
	"strings"

	pdf "github.com/ledongthuc/pdf"
)

// parsePdfText 从 PDF 字节数据中提取纯文本（P0-1 原生解析）。
// 使用纯 Go 的 ledongthuc/pdf 库（无 CGO，跨平台沙箱安全）。
func parsePdfText(data []byte) (string, error) {
	reader, err := pdf.NewReader(strings.NewReader(string(data)), int64(len(data)))
	if err != nil {
		// PDF 损坏或结构不完整时不阻断解析，返回空文本。
		return "", nil
	}
	total := reader.NumPage()
	if total == 0 {
		return "", nil
	}
	var sb strings.Builder
	for i := 1; i <= total; i++ {
		page := reader.Page(i)
		text, eerr := page.GetPlainText(nil)
		if eerr != nil {
			continue
		}
		if strings.TrimSpace(text) != "" {
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	}
	return strings.TrimSpace(sb.String()), nil
}
