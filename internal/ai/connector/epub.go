package connector

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"path"
	"strings"
)

// parseEPUB 抽取 EPUB 电子书的章节正文，转 Markdown。
//
// EPUB 是 ZIP 容器：META-INF/container.xml 指向 OPF 包描述文件，
// OPF 的 manifest 声明文档、spine 规定阅读顺序。逐章解析 XHTML
// （复用 HTML→Markdown 管线），章节标题用书内 h1/h2，缺失时用文件序号。
func parseEPUB(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open epub: %w", err)
	}

	// 1) container.xml → OPF 路径
	opfPath := ""
	for _, f := range zr.File {
		if f.Name != "META-INF/container.xml" {
			continue
		}
		rc, openErr := f.Open()
		if openErr != nil {
			break
		}
		raw, _ := io.ReadAll(rc)
		rc.Close()
		var container struct {
			Rootfiles []struct {
				FullPath string `xml:"full-path,attr"`
			} `xml:"rootfiles>rootfile"`
		}
		if xml.Unmarshal(raw, &container) == nil && len(container.Rootfiles) > 0 {
			opfPath = container.Rootfiles[0].FullPath
		}
		break
	}
	if opfPath == "" {
		return "", fmt.Errorf("epub container.xml missing or invalid")
	}

	// 2) OPF → manifest + spine
	var opfRaw []byte
	for _, f := range zr.File {
		if f.Name == opfPath {
			rc, openErr := f.Open()
			if openErr != nil {
				return "", fmt.Errorf("open opf: %w", openErr)
			}
			opfRaw, _ = io.ReadAll(rc)
			rc.Close()
			break
		}
	}
	if opfRaw == nil {
		return "", fmt.Errorf("epub opf not found: %s", opfPath)
	}

	var opf struct {
		Manifest struct {
			Items []struct {
				ID        string `xml:"id,attr"`
				Href      string `xml:"href,attr"`
				MediaType string `xml:"media-type,attr"`
			} `xml:"manifest>item"`
		} `xml:"manifest"`
		Spine struct {
			ItemRefs []struct {
				IDRef string `xml:"idref,attr"`
			} `xml:"spine>itemref"`
		} `xml:"spine"`
	}
	if err := xml.Unmarshal(opfRaw, &opf); err != nil {
		return "", fmt.Errorf("parse opf: %w", err)
	}
	hrefByID := make(map[string]string)
	for _, item := range opf.Manifest.Items {
		if item.MediaType == "application/xhtml+xml" || item.MediaType == "text/html" {
			hrefByID[item.ID] = item.Href
		}
	}

	opfDir := path.Dir(opfPath)
	var b strings.Builder
	chapter := 0
	for _, ref := range opf.Spine.ItemRefs {
		href, ok := hrefByID[ref.IDRef]
		if !ok {
			continue
		}
		full := href
		if opfDir != "." {
			full = opfDir + "/" + href
		}
		var raw []byte
		for _, f := range zr.File {
			if f.Name == full {
				rc, openErr := f.Open()
				if openErr != nil {
					break
				}
				raw, _ = io.ReadAll(rc)
				rc.Close()
				break
			}
		}
		if len(raw) == 0 {
			continue
		}
		md, convErr := NewFileParser().parseHTML(raw)
		if convErr != nil || strings.TrimSpace(md) == "" {
			continue
		}
		chapter++
		b.WriteString(fmt.Sprintf("\n\n# 第 %d 章\n\n", chapter))
		b.WriteString(strings.TrimSpace(md))
		b.WriteString("\n")
	}
	if chapter == 0 {
		return "", fmt.Errorf("epub has no readable chapters")
	}
	return strings.TrimSpace(b.String()), nil
}
