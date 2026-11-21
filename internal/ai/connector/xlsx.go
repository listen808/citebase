package connector

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// xlsxSharedStrings 对应 xl/sharedStrings.xml。
type xlsxSharedStrings struct {
	XMLName xml.Name `xml:"sst"`
	Items   []struct {
		Text string `xml:"t"`
	} `xml:"si"`
}

// xlsxWorksheet 对应 xl/worksheets/sheetN.xml（仅需关心的字段）。
type xlsxWorksheet struct {
	XMLName   xml.Name      `xml:"worksheet"`
	SheetData xlsxSheetData `xml:"sheetData"`
}

type xlsxSheetData struct {
	XMLName xml.Name  `xml:"sheetData"`
	Rows    []xlsxRow `xml:"row"`
}

type xlsxRow struct {
	XMLName xml.Name   `xml:"row"`
	Cells   []xlsxCell `xml:"c"`
}

type xlsxCell struct {
	XMLName xml.Name `xml:"c"`
	Ref     string   `xml:"r,attr"`
	Type    string   `xml:"t,attr"`
	Value   string   `xml:"v"`
}

// parseXLSX 从 xlsx 字节数据中提取所有工作表为 Markdown 表格。
// xlsx 是 ZIP 包：共享字符串在 xl/sharedStrings.xml，各工作表在 xl/worksheets/sheetN.xml。
func parseXLSX(data []byte) (string, error) {
	zr, err := zip.NewReader(strings.NewReader(string(data)), int64(len(data)))
	if err != nil {
		return "", err
	}

	shared := loadSharedStrings(zr)

	var sheets []*xlsxWorksheet
	for _, f := range zr.File {
		// 限制工作表数量：防御恶意 zip 中海量 sheet 条目造成的资源放大
		if len(sheets) >= maxSheetCount {
			return "", fmt.Errorf("xlsx 工作表数量超过上限 %d，拒绝解析", maxSheetCount)
		}
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			raw, err := readZipEntryLimited(rc)
			rc.Close()
			if err != nil {
				return "", err
			}
			var ws xlsxWorksheet
			if err := xml.Unmarshal(raw, &ws); err != nil {
				return "", err
			}
			sheets = append(sheets, &ws)
		}
	}
	if len(sheets) == 0 {
		return "", fmt.Errorf("xlsx 中未找到任何工作表")
	}

	var sb strings.Builder
	for i, ws := range sheets {
		if len(sheets) > 1 {
			sb.WriteString(fmt.Sprintf("## Sheet %d\n\n", i+1))
		}
		sb.WriteString(worksheetMarkdown(ws, shared))
		sb.WriteString("\n\n")
	}

	return strings.TrimSpace(sb.String()), nil
}

// loadSharedStrings 读取共享字符串表；缺失时返回空映射。
func loadSharedStrings(zr *zip.Reader) []string {
	for _, f := range zr.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, err := f.Open()
			if err != nil {
				return nil
			}
			raw, err := readZipEntryLimited(rc)
			rc.Close()
			if err != nil {
				return nil
			}
			var sst xlsxSharedStrings
			if err := xml.Unmarshal(raw, &sst); err != nil {
				return nil
			}
			out := make([]string, 0, len(sst.Items))
			for _, it := range sst.Items {
				out = append(out, it.Text)
			}
			return out
		}
	}
	return nil
}

func worksheetMarkdown(ws *xlsxWorksheet, shared []string) string {
	rows := ws.SheetData.Rows
	if len(rows) == 0 {
		return ""
	}
	var sb strings.Builder
	for i, row := range rows {
		cells := make([]string, 0, len(row.Cells))
		for _, c := range row.Cells {
			cells = append(cells, cellValue(c, shared))
		}
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

// cellValue 解析单元格值：共享字符串(s)按索引查表，否则直接用内联值。
func cellValue(c xlsxCell, shared []string) string {
	if c.Type == "s" {
		if idx, err := strconv.Atoi(c.Value); err == nil && idx >= 0 && idx < len(shared) {
			return shared[idx]
		}
		return ""
	}
	return c.Value
}
