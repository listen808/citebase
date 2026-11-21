package analyzer

import (
	"archive/zip"
	"bytes"
	"testing"
)

// newZip 创建一个写入给定条目的 ZIP 写入器（测试夹具构造辅助）。
func newZip(buf *bytes.Buffer, entries map[string]string) *zip.Writer {
	zw := zip.NewWriter(buf)
	for name, content := range entries {
		w, err := zw.Create(name)
		if err != nil {
			// 测试辅助，错误直接 panic 以便快速定位。
			panic(err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			panic(err)
		}
	}
	return zw
}

// buildTestXlsx 构造最小合法 xlsx 测试夹具。
func buildTestXlsx(t *testing.T) []byte {
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
	zw := newZip(&buf, map[string]string{
		"xl/sharedStrings.xml":     shared,
		"xl/worksheets/sheet1.xml": sheet,
	})
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
