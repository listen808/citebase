package analyzer

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

type adocxDocument struct {
	XMLName xml.Name  `xml:"document"`
	Body    adocxBody `xml:"body"`
}

type adocxBody struct {
	XMLName    xml.Name         `xml:"body"`
	Paragraphs []adocxParagraph `xml:"p"`
	Tables     []adocxTable     `xml:"tbl"`
}

type adocxParagraph struct {
	XMLName xml.Name   `xml:"p"`
	Runs    []adocxRun `xml:"r"`
}

type adocxRun struct {
	XMLName xml.Name    `xml:"r"`
	Text    []adocxText `xml:"t"`
}

type adocxText struct {
	XMLName xml.Name `xml:"t"`
	Content string   `xml:",chardata"`
}

type adocxTable struct {
	XMLName xml.Name        `xml:"tbl"`
	Rows    []adocxTableRow `xml:"tr"`
}

type adocxTableRow struct {
	XMLName xml.Name         `xml:"tr"`
	Cells   []adocxTableCell `xml:"tc"`
}

type adocxTableCell struct {
	XMLName    xml.Name         `xml:"tc"`
	Paragraphs []adocxParagraph `xml:"p"`
}

// parseDocxText 从 docx 字节提取纯文本（保留段落与表格），用于 analyzer 链路。
func parseDocxText(data []byte) (string, error) {
	zr, err := zip.NewReader(strings.NewReader(string(data)), int64(len(data)))
	if err != nil {
		return "", err
	}
	docXML, err := readZipEntry(zr, "word/document.xml")
	if err != nil {
		return "", err
	}
	var doc adocxDocument
	if err := xml.Unmarshal(docXML, &doc); err != nil {
		return "", err
	}
	var sb strings.Builder
	for _, p := range doc.Body.Paragraphs {
		sb.WriteString(adocxParagraphText(p))
		sb.WriteString("\n\n")
	}
	for _, t := range doc.Body.Tables {
		sb.WriteString(adocxTableMarkdown(t))
		sb.WriteString("\n\n")
	}
	return strings.TrimSpace(sb.String()), nil
}

func adocxParagraphText(p adocxParagraph) string {
	var sb strings.Builder
	for _, r := range p.Runs {
		for _, t := range r.Text {
			sb.WriteString(t.Content)
		}
	}
	return sb.String()
}

func adocxTableMarkdown(t adocxTable) string {
	if len(t.Rows) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, row := range t.Rows {
		cells := adocxCellTexts(row)
		sb.WriteString("| " + strings.Join(cells, " | ") + " |")
		sb.WriteString("\n")
		if i == 0 {
			sb.WriteString("|")
			for range cells {
				sb.WriteString(" --- |")
			}
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func adocxCellTexts(row adocxTableRow) []string {
	cells := make([]string, 0, len(row.Cells))
	for _, c := range row.Cells {
		var sb strings.Builder
		for _, p := range c.Paragraphs {
			sb.WriteString(adocxParagraphText(p))
		}
		cells = append(cells, strings.TrimSpace(sb.String()))
	}
	return cells
}

// maxZipEntryBytes 单个 zip 条目的解压上限：docx/xlsx 是不可信上传内容，
// 几 KB 的“解压炸弹”zip 可膨胀出数百 MB~GB 的 XML，io.ReadAll 无上限会导致 OOM 拖垮进程。
const maxZipEntryBytes = 64 << 20 // 64MB

// maxSheetCount xlsx 工作表数量上限，防御海量 sheet 条目的资源放大。
const maxSheetCount = 100

// readZipEntryLimited 带解压上限地读取 zip 条目，超限即判定为解压炸弹并报错。
func readZipEntryLimited(rc io.Reader) ([]byte, error) {
	raw, err := io.ReadAll(io.LimitReader(rc, maxZipEntryBytes+1))
	if err != nil {
		return nil, err
	}
	if int64(len(raw)) > maxZipEntryBytes {
		return nil, fmt.Errorf("zip 条目解压超过 %d 字节上限，疑似解压炸弹，拒绝解析", maxZipEntryBytes)
	}
	return raw, nil
}

// readZipEntry 读取 zip 中指定名称的条目内容。
func readZipEntry(zr *zip.Reader, name string) ([]byte, error) {
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				return nil, err
			}
			defer rc.Close()
			return readZipEntryLimited(rc)
		}
	}
	return nil, fmt.Errorf("zip 条目未找到: %s", name)
}
