package workspace

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"image"
	_ "image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestBuildDocxFromMarkdown 验证 buildDocxFromMarkdown 生成合规的 docx
// 使用真实图片文件验证图片嵌入功能，结构对齐 python-docx 标准
func TestBuildDocxFromMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	imgDir := filepath.Join(tmpDir, "data", "images")
	os.MkdirAll(imgDir, 0755)

	pngData := createTestPNG(2, 2)
	imgPath := filepath.Join(imgDir, "test.png")
	if err := os.WriteFile(imgPath, pngData, 0644); err != nil {
		t.Fatalf("无法创建测试图片: %v", err)
	}

	wd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(wd)

	md := `# 标题一

这是一段正文。

## 标题二

![测试图片](/images/test.png)

更多正文。`

	// 磁盘版图片加载器（对齐 loadImageForEmbed 的语义：存储路径 → 本地 data/images）
	load := func(src string) ([]byte, string, bool) {
		stored := storedAttachmentPath(src)
		if stored == "" || !strings.HasPrefix(stored, "images/") {
			return nil, "", false
		}
		data, err := os.ReadFile(filepath.Join("data", stored))
		if err != nil {
			return nil, "", false
		}
		return data, filepath.Ext(stored), true
	}
	docx, err := buildDocxFromMarkdown("测试文档", md, load)
	if err != nil {
		t.Fatalf("buildDocxFromMarkdown 失败: %v", err)
	}

	zr, err := zip.NewReader(bytes.NewReader(docx), int64(len(docx)))
	if err != nil {
		t.Fatalf("生成的 docx 不是有效 zip: %v", err)
	}

	// 必要文件：对齐 python-docx 标准结构
	requiredFiles := map[string]bool{
		"[Content_Types].xml":          false,
		"_rels/.rels":                  false,
		"word/_rels/document.xml.rels": false,
		"word/styles.xml":              false,
		"word/settings.xml":            false,
		"word/fontTable.xml":           false,
		"word/document.xml":            false,
		"word/media/image1.png":        false,
	}
	for _, f := range zr.File {
		if _, ok := requiredFiles[f.Name]; ok {
			requiredFiles[f.Name] = true
		}
	}
	for name, found := range requiredFiles {
		if !found {
			t.Errorf("缺少必要文件: %s", name)
		}
	}

	// [Content_Types].xml
	ct := readZipFile(t, zr, "[Content_Types].xml")
	if !bytes.Contains(ct, []byte(`Extension="png"`)) {
		t.Errorf("[Content_Types].xml 未声明 png 扩展名")
	}
	if !bytes.Contains(ct, []byte(`settings.xml`)) {
		t.Errorf("[Content_Types].xml 未声明 settings.xml")
	}
	if !bytes.Contains(ct, []byte(`fontTable.xml`)) {
		t.Errorf("[Content_Types].xml 未声明 fontTable.xml")
	}

	// document.xml.rels
	rels := readZipFile(t, zr, "word/_rels/document.xml.rels")
	if !bytes.Contains(rels, []byte(`Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/image"`)) {
		t.Errorf("document.xml.rels 未包含 image 关系")
	}
	if !bytes.Contains(rels, []byte(`Target="media/image1.png"`)) {
		t.Errorf("document.xml.rels 未指向 image1.png")
	}
	if !bytes.Contains(rels, []byte(`settings`)) {
		t.Errorf("document.xml.rels 未包含 settings 关系")
	}
	if !bytes.Contains(rels, []byte(`fontTable`)) {
		t.Errorf("document.xml.rels 未包含 fontTable 关系")
	}

	// document.xml — 关键 OOXML 节点
	docXML := readZipFile(t, zr, "word/document.xml")
	checks := []struct {
		label string
		sub   string
	}{
		{"<w:drawing>", "<w:drawing"},
		{"wp 命名空间声明", `xmlns:wp="http://schemas.openxmlformats.org/drawingml/2006/wordprocessingDrawing"`},
		{"wp:docPr 数字 id", `wp:docPr id="2"`},
		{"wp:cNvGraphicFramePr", "cNvGraphicFramePr"},
		{"pic:nvPicPr", "nvPicPr"},
		{"pic:cNvPr", "cNvPr"},
		{"pic:cNvPicPr", "cNvPicPr"},
		{"a:stretch", "stretch"},
		{"a:fillRect", "fillRect"},
		{"a:prstGeom", "prstGeom"},
		{"a:graphicFrameLocks", "graphicFrameLocks"},
		{"r:embed 引用", `r:embed="rId2"`},
	}
	for _, c := range checks {
		if !bytes.Contains(docXML, []byte(c.sub)) {
			t.Errorf("document.xml 未包含 %s (子串: %s)", c.label, c.sub)
		}
	}

	// 关键：验证 document.xml 是合法 XML（Word 严格解析 XML，非法 XML 会导致打不开）
	var xmlDoc struct{}
	if err := xml.Unmarshal(docXML, &xmlDoc); err != nil {
		t.Errorf("document.xml 不是合法 XML: %v", err)
	}

	// word/media/image1.png
	imgFileData := readZipFile(t, zr, "word/media/image1.png")
	if !bytes.Equal(imgFileData, pngData) {
		t.Errorf("word/media/image1.png 与源文件不匹配")
	}

	cfg, _, err := image.DecodeConfig(bytes.NewReader(pngData))
	if err != nil {
		t.Fatalf("无法解码测试图片: %v", err)
	}
	t.Logf("测试图片尺寸: %dx%d, docx 大小: %d 字节", cfg.Width, cfg.Height, len(docx))
}

func readZipFile(t *testing.T, zr *zip.Reader, name string) []byte {
	for _, f := range zr.File {
		if f.Name == name {
			rc, err := f.Open()
			if err != nil {
				t.Fatalf("无法打开 %s: %v", name, err)
			}
			defer rc.Close()
			buf := new(bytes.Buffer)
			buf.ReadFrom(rc)
			return buf.Bytes()
		}
	}
	t.Fatalf("文件 %s 不存在", name)
	return nil
}

func createTestPNG(w, h int) []byte {
	return []byte{
		0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1F, 0x15, 0xC4,
		0x89, 0x00, 0x00, 0x00, 0x0D, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9C, 0x62, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0D, 0x0A, 0x2D, 0xB4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4E, 0x44, 0xAE,
		0x42, 0x60, 0x82,
	}
}
