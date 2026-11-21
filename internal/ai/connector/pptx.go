package connector

import (
	"archive/zip"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// parsePPTX 逐页抽取 PPTX 的文本，转 Markdown。
//
// PPTX 是 ZIP 容器：ppt/slides/slideN.xml 每页一个 XML，正文都在
// DrawingML 的 <a:t> 文本 run 里，<a:p> 是段落边界。按页码顺序拼接，
// 页与页之间用分隔线，方便后续按标题分块。
func parsePPTX(data []byte) (string, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", fmt.Errorf("open pptx: %w", err)
	}

	type slide struct {
		num  int
		text string
	}
	var slideRe = regexp.MustCompile(`^ppt/slides/slide(\d+)\.xml$`)
	var slides []slide

	for _, f := range zr.File {
		m := slideRe.FindStringSubmatch(f.Name)
		if m == nil {
			continue
		}
		num, _ := strconv.Atoi(m[1])
		rc, openErr := f.Open()
		if openErr != nil {
			continue
		}
		raw, readErr := io.ReadAll(rc)
		rc.Close()
		if readErr != nil {
			continue
		}
		text := extractSlideText(raw)
		if text != "" {
			slides = append(slides, slide{num: num, text: text})
		}
	}
	if len(slides) == 0 {
		return "", fmt.Errorf("pptx has no extractable slide text")
	}
	sort.Slice(slides, func(i, j int) bool { return slides[i].num < slides[j].num })

	var b strings.Builder
	for i, s := range slides {
		if i > 0 {
			b.WriteString("\n\n---\n\n")
		}
		fmt.Fprintf(&b, "## 第 %d 页\n\n%s", s.num, s.text)
	}
	return b.String(), nil
}

// extractSlideText 用流式 XML 解析收集 <a:t> 文本，<a:p> 边界换行。
func extractSlideText(xmlData []byte) string {
	dec := xml.NewDecoder(bytes.NewReader(xmlData))
	dec.Strict = false

	var b strings.Builder
	var line strings.Builder
	inP := false
	flush := func() {
		lineStr := strings.TrimSpace(line.String())
		if lineStr != "" {
			b.WriteString(lineStr)
			b.WriteString("\n")
		}
		line.Reset()
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "p": // a:p 段落
				inP = true
			case "t": // a:t 文本 run
				var txt string
				if dec.DecodeElement(&txt, &t) == nil {
					line.WriteString(txt)
				}
			}
		case xml.EndElement:
			if t.Name.Local == "p" && inP {
				inP = false
				flush()
			}
		}
	}
	flush()
	return strings.TrimSpace(b.String())
}
