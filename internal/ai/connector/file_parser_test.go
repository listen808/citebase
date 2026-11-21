package connector

import (
	"archive/zip"
	"bytes"
	"strings"
	"testing"
)

// buildDocx 构造一个最小合法 docx（ZIP 包）用于测试。
func buildDocx(t *testing.T) []byte {
	t.Helper()
	documentXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>Hello</w:t></w:r></w:p>
    <w:p><w:r><w:t>World</w:t></w:r></w:p>
    <w:tbl>
      <w:tr><w:tc><w:p><w:r><w:t>Name</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>Age</w:t></w:r></w:p></w:tc></w:tr>
      <w:tr><w:tc><w:p><w:r><w:t>Alice</w:t></w:r></w:p></w:tc><w:tc><w:p><w:r><w:t>30</w:t></w:r></w:p></w:tc></w:tr>
    </w:tbl>
  </w:body>
</w:document>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("word/document.xml")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte(documentXML)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// buildXlsx 构造一个最小合法 xlsx（含共享字符串与单工作表）用于测试。
func buildXlsx(t *testing.T) []byte {
	t.Helper()
	shared := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<sst xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main" count="4" uniqueCount="4">
  <si><t>Name</t></si>
  <si><t>Age</t></si>
  <si><t>Alice</t></si>
  <si><t>30</t></si>
</sst>`
	sheet := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main">
  <sheetData>
    <row r="1"><c r="A1" t="s"><v>0</v></c><c r="B1" t="s"><v>1</v></c></row>
    <row r="2"><c r="A2" t="s"><v>2</v></c><c r="B2" t="s"><v>3</v></c></row>
  </sheetData>
</worksheet>`

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	entries := map[string]string{
		"xl/sharedStrings.xml":     shared,
		"xl/worksheets/sheet1.xml": sheet,
	}
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParseDOCX(t *testing.T) {
	data := buildDocx(t)
	out, err := parseDOCX(data)
	if err != nil {
		t.Fatalf("parseDOCX error: %v", err)
	}
	if !strings.Contains(out, "Hello") || !strings.Contains(out, "World") {
		t.Errorf("DOCX 文本缺失: %q", out)
	}
	if !strings.Contains(out, "Name") || !strings.Contains(out, "Alice") {
		t.Errorf("DOCX 表格未解析: %q", out)
	}
}

func TestParseXLSX(t *testing.T) {
	data := buildXlsx(t)
	out, err := parseXLSX(data)
	if err != nil {
		t.Fatalf("parseXLSX error: %v", err)
	}
	if !strings.Contains(out, "Name") || !strings.Contains(out, "Alice") {
		t.Errorf("XLSX 内容未解析: %q", out)
	}
	if !strings.Contains(out, "|") {
		t.Errorf("XLSX 未转为 Markdown 表格: %q", out)
	}
}

func TestFileParser_ParseFile_NewFormats(t *testing.T) {
	p := NewFileParser()
	if !IsSupported("a.pdf") || !IsSupported("a.docx") || !IsSupported("a.xlsx") {
		t.Fatal("SupportedExtensions 未包含新格式")
	}

	// DOCX
	out, err := p.ParseFile("doc.docx", buildDocx(t))
	if err != nil || !strings.Contains(out, "Hello") {
		t.Errorf("ParseFile docx 失败: err=%v out=%q", err, out)
	}
	// XLSX
	out, err = p.ParseFile("data.xlsx", buildXlsx(t))
	if err != nil || !strings.Contains(out, "Alice") {
		t.Errorf("ParseFile xlsx 失败: err=%v out=%q", err, out)
	}
}

func TestParsePDF_Malformed(t *testing.T) {
	// 损坏的 PDF 字节：缺失 startxref，解析应容错返回空而非报错。
	pdfData := []byte("%PDF-1.4\n1 0 obj<</Type/Catalog>>endobj\n%%EOF\n")
	out, err := parsePDF(pdfData)
	if err != nil {
		t.Fatalf("parsePDF 对损坏 PDF 不应报错: %v", err)
	}
	if out != "" {
		t.Errorf("损坏 PDF 应返回空文本，得到 %q", out)
	}
}
