package connector

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// FileParser parses uploaded files into Markdown content.
type FileParser struct{}

// NewFileParser creates a new FileParser.
func NewFileParser() *FileParser {
	return &FileParser{}
}

// SupportedExtensions lists file extensions the parser can handle.
var SupportedExtensions = []string{".txt", ".md", ".html", ".htm", ".csv", ".json", ".pdf", ".docx", ".xlsx", ".pptx", ".epub"}

// IsSupported returns true if the file extension is supported.
func IsSupported(filename string) bool {
	ext := strings.ToLower(filepath.Ext(filename))
	for _, s := range SupportedExtensions {
		if ext == s {
			return true
		}
	}
	return false
}

// ParseFile parses file content into a Markdown string.
func (p *FileParser) ParseFile(filename string, data []byte) (string, error) {
	ext := strings.ToLower(filepath.Ext(filename))

	switch ext {
	case ".txt":
		return string(data), nil

	case ".md":
		return string(data), nil

	case ".html", ".htm":
		return p.parseHTML(data)

	case ".csv":
		return p.parseCSV(data)

	case ".json":
		return p.parseJSON(data)

	case ".pdf":
		// P5：MinerU 可选后端（复杂版面保留表格/结构），未配置或失败回退内置解析器
		if endpoint := mineruEndpoint(); endpoint != "" {
			if md, mErr := parsePDFWithMinerU(endpoint, filename, data); mErr == nil && strings.TrimSpace(md) != "" {
				return md, nil
			}
		}
		return parsePDF(data)

	case ".docx":
		return parseDOCX(data)

	case ".xlsx":
		return parseXLSX(data)

	case ".pptx":
		return parsePPTX(data)

	case ".epub":
		return parseEPUB(data)

	default:
		return "", fmt.Errorf("unsupported file type: %s", ext)
	}
}

// parseHTML converts HTML content to Markdown.
func (p *FileParser) parseHTML(data []byte) (string, error) {
	doc, err := html.Parse(strings.NewReader(string(data)))
	if err != nil {
		return "", fmt.Errorf("parse HTML: %w", err)
	}

	title := extractTitle(doc)
	body := findFirst(doc, atom.Body)
	if body == nil {
		body = doc
	}

	md := htmlToMarkdown(body)

	if title != "" {
		return "# " + title + "\n\n" + md, nil
	}
	return md, nil
}

// parseCSV converts CSV data to a Markdown table.
func (p *FileParser) parseCSV(data []byte) (string, error) {
	reader := csv.NewReader(strings.NewReader(string(data)))
	reader.LazyQuotes = true
	reader.TrimLeadingSpace = true

	records, err := reader.ReadAll()
	if err != nil {
		return "", fmt.Errorf("parse CSV: %w", err)
	}

	if len(records) == 0 {
		return "", nil
	}

	var sb strings.Builder

	// Header row
	sb.WriteString("| " + strings.Join(records[0], " | ") + " |")
	sb.WriteString("\n")

	// Separator
	sb.WriteString("|")
	for range records[0] {
		sb.WriteString(" --- |")
	}
	sb.WriteString("\n")

	// Data rows
	for _, row := range records[1:] {
		// Pad row to match header length
		for len(row) < len(records[0]) {
			row = append(row, "")
		}
		sb.WriteString("| " + strings.Join(row, " | ") + " |")
		sb.WriteString("\n")
	}

	return sb.String(), nil
}

// parseJSON formats JSON data as a readable Markdown block.
func (p *FileParser) parseJSON(data []byte) (string, error) {
	var raw interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return "", fmt.Errorf("parse JSON: %w", err)
	}

	formatted, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return "", fmt.Errorf("format JSON: %w", err)
	}

	var sb strings.Builder
	sb.WriteString("```json\n")
	sb.Write(formatted)
	sb.WriteString("\n```\n")

	return sb.String(), nil
}
