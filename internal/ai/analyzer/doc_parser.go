package analyzer

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// DocumentFormat 文档格式枚举
type DocumentFormat string

const (
	FormatMarkdown DocumentFormat = "markdown"
	FormatPDF      DocumentFormat = "pdf"
	FormatWord     DocumentFormat = "docx"
	FormatExcel    DocumentFormat = "xlsx"
	FormatPPT      DocumentFormat = "pptx"
	FormatHTML     DocumentFormat = "html"
	FormatCSV      DocumentFormat = "csv"
	FormatText     DocumentFormat = "txt"
	FormatUnknown  DocumentFormat = "unknown"
)

// ParsedDocument 解析后的文档结构
type ParsedDocument struct {
	Format   DocumentFormat `json:"format"`
	Title    string         `json:"title"`
	Content  string         `json:"content"`  // 主要文本内容
	Tables   []ParsedTable  `json:"tables"`   // 提取的表格
	Images   []ParsedImage  `json:"images"`   // 提取的图片路径
	Metadata DocMetadata    `json:"metadata"` // 文档元数据
	RawBytes []byte         `json:"-"`        // 原始字节
}

// ParsedTable 解析后的表格
type ParsedTable struct {
	Caption string     `json:"caption"`
	Headers []string   `json:"headers"`
	Rows    [][]string `json:"rows"`
}

// ParsedImage 解析后的图片信息
type ParsedImage struct {
	FileName string `json:"file_name"`
	Path     string `json:"path"`
	AltText  string `json:"alt_text"`
	Format   string `json:"format"`
}

// DocMetadata 文档元数据
type DocMetadata struct {
	Title      string            `json:"title"`
	Author     string            `json:"author"`
	CreatedAt  string            `json:"created_at"`
	ModifiedAt string            `json:"modified_at"`
	PageCount  int               `json:"page_count"`
	WordCount  int               `json:"word_count"`
	Keywords   []string          `json:"keywords"`
	Language   string            `json:"language"`
	Custom     map[string]string `json:"custom"`
}

// MultiFormatParser 多格式文档解析器接口
type MultiFormatParser interface {
	Parse(r io.Reader, fileName string) (*ParsedDocument, error)
	SupportedFormats() []DocumentFormat
}

// UniversalDocParser 通用文档解析器（自动识别格式并选择合适的解析器）
type UniversalDocParser struct {
	parsers map[DocumentFormat]MultiFormatParser
}

// NewUniversalDocParser 创建通用文档解析器
func NewUniversalDocParser() *UniversalDocParser {
	p := &UniversalDocParser{
		parsers: make(map[DocumentFormat]MultiFormatParser),
	}

	// 注册内置解析器
	p.RegisterParser(NewMarkdownDocParser())
	p.RegisterParser(NewTextDocParser())
	p.RegisterParser(NewCSVDocParser())
	p.RegisterParser(NewHTMLDocParser())

	return p
}

// RegisterParser 注册格式解析器
func (p *UniversalDocParser) RegisterParser(parser MultiFormatParser) {
	for _, format := range parser.SupportedFormats() {
		p.parsers[format] = parser
	}
}

// Parse 解析文档（自动检测格式）
func (p *UniversalDocParser) Parse(filePath string) (*ParsedDocument, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", filePath, err)
	}

	format := detectFormat(filePath, data)
	parser, ok := p.parsers[format]
	if !ok {
		// 回退到纯文本解析
		parser = p.parsers[FormatText]
	}

	doc, err := parser.Parse(bytes.NewReader(data), filepath.Base(filePath))
	if err != nil {
		return nil, fmt.Errorf("parse %s as %s: %w", filePath, format, err)
	}

	// 补充文件级元数据
	if doc.Metadata.ModifiedAt == "" {
		if info, err := os.Stat(filePath); err == nil {
			doc.Metadata.ModifiedAt = info.ModTime().Format("2006-01-02 15:04:05")
		}
	}
	doc.RawBytes = data

	return doc, nil
}

// ParseBytes 从字节数据解析文档
func (p *UniversalDocParser) ParseBytes(data []byte, fileName string) (*ParsedDocument, error) {
	format := detectFormat(fileName, data)
	parser, ok := p.parsers[format]
	if !ok {
		parser = p.parsers[FormatText]
	}

	doc, err := parser.Parse(bytes.NewReader(data), fileName)
	if err != nil {
		return nil, fmt.Errorf("parse as %s: %w", format, err)
	}
	doc.RawBytes = data
	return doc, nil
}

// SupportedFormats 返回所有支持的格式
func (p *UniversalDocParser) SupportedFormats() []DocumentFormat {
	formats := make([]DocumentFormat, 0, len(p.parsers))
	for f := range p.parsers {
		formats = append(formats, f)
	}
	return formats
}

// detectFormat 根据文件名和后缀检测文档格式
func detectFormat(fileName string, data []byte) DocumentFormat {
	ext := strings.ToLower(filepath.Ext(fileName))
	switch ext {
	case ".md", ".markdown":
		return FormatMarkdown
	case ".pdf":
		return FormatPDF
	case ".docx", ".doc":
		return FormatWord
	case ".xlsx", ".xls":
		return FormatExcel
	case ".pptx", ".ppt":
		return FormatPPT
	case ".html", ".htm":
		return FormatHTML
	case ".csv":
		return FormatCSV
	case ".txt", ".text", ".log":
		return FormatText
	default:
		// 尝试通过MIME类型检测
		if isPDFData(data) {
			return FormatPDF
		}
		if isDOCXData(data) {
			return FormatWord
		}
		if isXLSXData(data) {
			return FormatExcel
		}
		return FormatText // 默认作为纯文本处理
	}
}

// isPDFData 检测是否为PDF数据
func isPDFData(data []byte) bool {
	return len(data) >= 4 && string(data[:4]) == "%PDF"
}

// isDOCXData 检测是否为DOCX数据（ZIP格式，含word/document.xml）
func isDOCXData(data []byte) bool {
	return len(data) >= 4 && data[0] == 0x50 && data[1] == 0x4B && data[2] == 0x03 && data[3] == 0x04
}

// isXLSXData 检测是否为XLSX数据（ZIP格式，含xl/workbook.xml）
func isXLSXData(data []byte) bool {
	return isDOCXData(data) // XLSX也是ZIP格式，进一步检查需要在解析器中处理
}

// MarkdownDocParser Markdown文档解析器
type MarkdownDocParser struct{}

func NewMarkdownDocParser() *MarkdownDocParser {
	return &MarkdownDocParser{}
}

func (p *MarkdownDocParser) SupportedFormats() []DocumentFormat {
	return []DocumentFormat{FormatMarkdown}
}

func (p *MarkdownDocParser) Parse(r io.Reader, fileName string) (*ParsedDocument, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	content := string(data)

	doc := &ParsedDocument{
		Format:  FormatMarkdown,
		Title:   extractMarkdownTitle(content, fileName),
		Content: content,
		Metadata: DocMetadata{
			WordCount: len(strings.Fields(content)),
			Language:  detectLanguage(content),
			Keywords:  extractSimpleKeywords(content, 10),
		},
	}

	// 提取Markdown表格
	doc.Tables = extractMarkdownTables(content)

	return doc, nil
}

// extractMarkdownTitle 从Markdown内容中提取标题
func extractMarkdownTitle(content, fallback string) string {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
		if strings.HasPrefix(trimmed, "#") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "#"))
		}
	}
	// fallback: 使用文件名（去掉后缀）
	return strings.TrimSuffix(fallback, filepath.Ext(fallback))
}

// extractMarkdownTables 提取Markdown表格
func extractMarkdownTables(content string) []ParsedTable {
	var tables []ParsedTable
	lines := strings.Split(content, "\n")
	inTable := false
	var currentTable ParsedTable

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if !strings.HasPrefix(line, "|") || !strings.HasSuffix(line, "|") {
			if inTable {
				tables = append(tables, currentTable)
				currentTable = ParsedTable{}
				inTable = false
			}
			continue
		}

		// 跳过分隔行
		if strings.Contains(line, "---") && strings.Contains(line, "|") {
			continue
		}

		cells := splitTableRow(line)
		if !inTable {
			inTable = true
			currentTable.Headers = cells
		} else {
			currentTable.Rows = append(currentTable.Rows, cells)
		}
	}

	if inTable && len(currentTable.Rows) > 0 {
		tables = append(tables, currentTable)
	}

	return tables
}

// splitTableRow 分割表格行
func splitTableRow(line string) []string {
	trimmed := strings.Trim(line, "|")
	parts := strings.Split(trimmed, "|")
	cells := make([]string, len(parts))
	for i, p := range parts {
		cells[i] = strings.TrimSpace(p)
	}
	return cells
}

// TextDocParser 纯文本文档解析器
type TextDocParser struct{}

func NewTextDocParser() *TextDocParser {
	return &TextDocParser{}
}

func (p *TextDocParser) SupportedFormats() []DocumentFormat {
	return []DocumentFormat{FormatText, FormatPDF, FormatWord, FormatPPT}
}

func (p *TextDocParser) Parse(r io.Reader, fileName string) (*ParsedDocument, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	content := string(data)
	format := detectFormat(fileName, data)

	// PDF / Word 使用真正的原生解析（P0-1）。
	if format == FormatPDF {
		if extracted, eerr := parsePdfText(data); eerr == nil && strings.TrimSpace(extracted) != "" {
			content = extracted
		}
	} else if format == FormatWord {
		if extracted, eerr := parseDocxText(data); eerr == nil && strings.TrimSpace(extracted) != "" {
			content = extracted
		}
	}

	doc := &ParsedDocument{
		Format:  format,
		Title:   strings.TrimSuffix(fileName, filepath.Ext(fileName)),
		Content: content,
		Metadata: DocMetadata{
			WordCount: len(strings.Fields(content)),
			Language:  detectLanguage(content),
			Keywords:  extractSimpleKeywords(content, 10),
		},
	}

	return doc, nil
}

// CSVDocParser CSV文档解析器
type CSVDocParser struct{}

func NewCSVDocParser() *CSVDocParser {
	return &CSVDocParser{}
}

func (p *CSVDocParser) SupportedFormats() []DocumentFormat {
	return []DocumentFormat{FormatCSV, FormatExcel}
}

func (p *CSVDocParser) Parse(r io.Reader, fileName string) (*ParsedDocument, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}

	// XLSX 走真正的原生解析（P0-1），而非当作 CSV。
	if isXLSXData(data) {
		text, tables, xerr := parseXlsx(data)
		if xerr != nil {
			return nil, xerr
		}
		doc := &ParsedDocument{
			Format:  FormatExcel,
			Title:   strings.TrimSuffix(fileName, filepath.Ext(fileName)),
			Content: text,
			Tables:  tables,
			Metadata: DocMetadata{
				WordCount: len(strings.Fields(text)),
				Language:  detectLanguage(text),
				Keywords:  extractSimpleKeywords(text, 10),
			},
		}
		return doc, nil
	}

	content := string(data)

	// 简单CSV解析
	lines := strings.Split(content, "\n")
	var table ParsedTable
	table.Caption = strings.TrimSuffix(fileName, filepath.Ext(fileName))

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		cells := strings.Split(line, ",")
		if i == 0 {
			table.Headers = cells
		} else {
			table.Rows = append(table.Rows, cells)
		}
	}

	doc := &ParsedDocument{
		Format:  FormatCSV,
		Title:   strings.TrimSuffix(fileName, filepath.Ext(fileName)),
		Content: content,
		Tables:  []ParsedTable{table},
		Metadata: DocMetadata{
			WordCount: len(strings.Fields(content)),
			Language:  detectLanguage(content),
			Keywords:  extractSimpleKeywords(content, 10),
		},
	}

	return doc, nil
}

// HTMLDocParser HTML文档解析器
type HTMLDocParser struct{}

func NewHTMLDocParser() *HTMLDocParser {
	return &HTMLDocParser{}
}

func (p *HTMLDocParser) SupportedFormats() []DocumentFormat {
	return []DocumentFormat{FormatHTML}
}

func (p *HTMLDocParser) Parse(r io.Reader, fileName string) (*ParsedDocument, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	content := string(data)

	// 简单HTML文本提取（去除标签）
	plainText := stripHTMLTags(content)

	doc := &ParsedDocument{
		Format:  FormatHTML,
		Title:   extractHTMLTitle(content, fileName),
		Content: plainText,
		Metadata: DocMetadata{
			WordCount: len(strings.Fields(plainText)),
			Language:  detectLanguage(plainText),
			Keywords:  extractSimpleKeywords(plainText, 10),
		},
	}

	return doc, nil
}

// stripHTMLTags 去除HTML标签
func stripHTMLTags(html string) string {
	var result strings.Builder
	inTag := false
	for _, r := range html {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			result.WriteRune(' ')
			continue
		}
		if !inTag {
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

// extractHTMLTitle 从HTML中提取标题
func extractHTMLTitle(html, fallback string) string {
	lower := strings.ToLower(html)
	start := strings.Index(lower, "<title>")
	end := strings.Index(lower, "</title>")
	if start >= 0 && end > start {
		return strings.TrimSpace(html[start+7 : end])
	}
	return strings.TrimSuffix(fallback, filepath.Ext(fallback))
}

// detectLanguage 简单语言检测（基于中文字符比例）
func detectLanguage(text string) string {
	chineseCount := 0
	totalCount := 0
	for _, r := range text {
		if r >= 32 && r < 127 || r >= 0x4e00 {
			totalCount++
		}
		if r >= 0x4e00 && r <= 0x9fff {
			chineseCount++
		}
	}
	if totalCount > 0 && float64(chineseCount)/float64(totalCount) > 0.3 {
		return "zh"
	}
	return "en"
}

// extractSimpleKeywords 简单关键词提取（基于词频）
func extractSimpleKeywords(text string, maxCount int) []string {
	words := strings.Fields(text)
	freq := make(map[string]int)

	// 停用词
	stopWords := map[string]bool{
		"the": true, "a": true, "an": true, "is": true, "are": true,
		"in": true, "on": true, "at": true, "to": true, "for": true,
		"of": true, "and": true, "or": true, "with": true, "from": true,
		"的": true, "了": true, "是": true, "在": true, "有": true,
		"和": true, "与": true, "或": true, "把": true, "被": true,
	}

	for _, w := range words {
		w = strings.ToLower(strings.Trim(w, ".,!?;:()[]{}\"'"))
		if len(w) < 3 || stopWords[w] {
			continue
		}
		freq[w]++
	}

	// 排序取前N个
	type wordFreq struct {
		word  string
		count int
	}
	sorted := make([]wordFreq, 0, len(freq))
	for w, c := range freq {
		sorted = append(sorted, wordFreq{w, c})
	}

	// 简单排序（冒泡，因为数据量小）
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[j].count > sorted[i].count {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	keywords := make([]string, 0, maxCount)
	for i := 0; i < len(sorted) && i < maxCount; i++ {
		keywords = append(keywords, sorted[i].word)
	}

	return keywords
}
