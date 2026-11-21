package connector

import (
	"errors"
	"strings"

	pdf "github.com/ledongthuc/pdf"
)

// ErrPDFNoTextLayer P1-4：PDF 结构正常但没有文本层——扫描件、图片型 PDF、
// 或文字以曲线/位图形式嵌入。这类文件原先被静默当成"空文档"导入成功，
// 用户以为入库了，检索时什么也搜不到。
var ErrPDFNoTextLayer = errors.New("pdf has no extractable text layer (scanned/image-only pdf, OCR required)")

// parsePDF 从 PDF 字节数据中提取纯文本。
// 使用纯 Go 的 ledongthuc/pdf 库（无 CGO，跨平台沙箱安全），
// 按页读取并合并为可检索文本。
func parsePDF(data []byte) (string, error) {
	reader, err := pdf.NewReader(strings.NewReader(string(data)), int64(len(data)))
	if err != nil {
		// 结构损坏时保持原有容错：返回空文本，不阻断批量导入。
		// P1-4 修的是下面"有页却零文本"那种真正会骗人的情况。
		return "", nil
	}

	total := reader.NumPage()
	if total == 0 {
		return "", nil
	}

	var sb strings.Builder
	for i := 1; i <= total; i++ {
		page := reader.Page(i)
		text, extractErr := page.GetPlainText(nil)
		if extractErr != nil {
			continue
		}
		if strings.TrimSpace(text) != "" {
			sb.WriteString(text)
			sb.WriteString("\n\n")
		}
	}

	out := strings.TrimSpace(sb.String())
	if out == "" {
		return "", ErrPDFNoTextLayer
	}
	return out, nil
}
