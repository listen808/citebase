package connector

import (
	"archive/zip"
	"bytes"
	"testing"
)

// 内存构造一个最小 PPTX：单页 slide，含两个段落
func buildTestPPTX(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("ppt/slides/slide1.xml")
	if err != nil {
		t.Fatal(err)
	}
	slideXML := `<?xml version="1.0"?>
<p:sld xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" xmlns:p="x">
  <p:cSld><p:spTree>
    <p:sp><p:txBody>
      <a:p><a:r><a:t>页标题</a:t></a:r></a:p>
      <a:p><a:r><a:t>第一行内容</a:t></a:r></a:p>
    </p:txBody></p:sp>
  </p:spTree></p:cSld>
</p:sld>`
	if _, err := f.Write([]byte(slideXML)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestParsePPTX(t *testing.T) {
	md, err := parsePPTX(buildTestPPTX(t))
	if err != nil {
		t.Fatalf("parse pptx: %v", err)
	}
	for _, want := range []string{"第 1 页", "页标题", "第一行内容"} {
		if !bytes.Contains([]byte(md), []byte(want)) {
			t.Fatalf("pptx markdown missing %q, got:\n%s", want, md)
		}
	}
}

func TestParsePPTXRejectsNonZip(t *testing.T) {
	if _, err := parsePPTX([]byte("not a zip")); err == nil {
		t.Fatalf("expected error for non-zip input")
	}
}
