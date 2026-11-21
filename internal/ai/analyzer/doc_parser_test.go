package analyzer

import (
	"bytes"
	"strings"
	"testing"
)

func buildTestDocx(t *testing.T) []byte {
	t.Helper()
	documentXML := `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
  <w:body>
    <w:p><w:r><w:t>第一章</w:t></w:r></w:p>
    <w:p><w:r><w:t>这是测试内容</w:t></w:r></w:p>
  </w:body>
</w:document>`
	var buf bytes.Buffer
	zw := newZip(&buf, map[string]string{"word/document.xml": documentXML})
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestUniversalDocParser_DOCX(t *testing.T) {
	data := buildTestDocx(t)
	p := NewUniversalDocParser()
	doc, err := p.ParseBytes(data, "report.docx")
	if err != nil {
		t.Fatalf("ParseBytes docx error: %v", err)
	}
	if doc.Format != FormatWord {
		t.Errorf("格式识别错误: %s", doc.Format)
	}
	if !strings.Contains(doc.Content, "第一章") || !strings.Contains(doc.Content, "测试内容") {
		t.Errorf("DOCX 文本内容缺失: %q", doc.Content)
	}
}

func TestUniversalDocParser_XLSX(t *testing.T) {
	data := buildTestXlsx(t)
	p := NewUniversalDocParser()
	doc, err := p.ParseBytes(data, "data.xlsx")
	if err != nil {
		t.Fatalf("ParseBytes xlsx error: %v", err)
	}
	if doc.Format != FormatExcel {
		t.Errorf("格式识别错误: %s", doc.Format)
	}
	if !strings.Contains(doc.Content, "Alice") {
		t.Errorf("XLSX 内容缺失: %q", doc.Content)
	}
	if len(doc.Tables) == 0 || len(doc.Tables[0].Headers) == 0 {
		t.Errorf("XLSX 表格结构未提取: %+v", doc.Tables)
	}
}
