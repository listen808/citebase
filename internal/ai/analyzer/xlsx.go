package analyzer

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

type axlsxSharedStrings struct {
	XMLName xml.Name `xml:"sst"`
	Items   []struct {
		Text string `xml:"t"`
	} `xml:"si"`
}

type axlsxWorksheet struct {
	XMLName   xml.Name       `xml:"worksheet"`
	SheetData axlsxSheetData `xml:"sheetData"`
}

type axlsxSheetData struct {
	XMLName xml.Name   `xml:"sheetData"`
	Rows    []axlsxRow `xml:"row"`
}

type axlsxRow struct {
	XMLName xml.Name    `xml:"row"`
	Cells   []axlsxCell `xml:"c"`
}

type axlsxCell struct {
	XMLName xml.Name `xml:"c"`
	Ref     string   `xml:"r,attr"`
	Type    string   `xml:"t,attr"`
	Value   string   `xml:"v"`
}

// parseXlsx 解析 xlsx 为文本（Markdown 表格）与结构化表格切片，用于 analyzer 链路。
func parseXlsx(data []byte) (text string, tables []ParsedTable, err error) {
	zr, err := zip.NewReader(strings.NewReader(string(data)), int64(len(data)))
	if err != nil {
		return "", nil, err
	}

	shared := loadAXlsxSharedStrings(zr)

	var sheets []*axlsxWorksheet
	for _, f := range zr.File {
		// 限制工作表数量：防御恶意 zip 中海量 sheet 条目造成的资源放大
		if len(sheets) >= maxSheetCount {
			return "", nil, fmt.Errorf("xlsx 工作表数量超过上限 %d，拒绝解析", maxSheetCount)
		}
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			rc, oerr := f.Open()
			if oerr != nil {
				return "", nil, oerr
			}
			raw, rerr := readZipEntryLimited(rc)
			rc.Close()
			if rerr != nil {
				return "", nil, rerr
			}
			var ws axlsxWorksheet
			if xerr := xml.Unmarshal(raw, &ws); xerr != nil {
				return "", nil, xerr
			}
			sheets = append(sheets, &ws)
		}
	}
	if len(sheets) == 0 {
		return "", nil, fmt.Errorf("xlsx 中未找到任何工作表")
	}

	var sb strings.Builder
	for i, ws := range sheets {
		if len(sheets) > 1 {
			sb.WriteString(fmt.Sprintf("## Sheet %d\n\n", i+1))
		}
		md, tbl := axlsxSheetMarkdown(ws, shared)
		sb.WriteString(md)
		sb.WriteString("\n\n")
		if len(tbl.Rows) > 0 {
			tables = append(tables, tbl)
		}
	}

	return strings.TrimSpace(sb.String()), tables, nil
}

func loadAXlsxSharedStrings(zr *zip.Reader) []string {
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
			var sst axlsxSharedStrings
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

func axlsxSheetMarkdown(ws *axlsxWorksheet, shared []string) (string, ParsedTable) {
	rows := ws.SheetData.Rows
	if len(rows) == 0 {
		return "", ParsedTable{}
	}
	var sb strings.Builder
	tbl := ParsedTable{Rows: make([][]string, 0, len(rows)-1)}
	for i, row := range rows {
		cells := make([]string, 0, len(row.Cells))
		for _, c := range row.Cells {
			cells = append(cells, axlsxCellValue(c, shared))
		}
		sb.WriteString("| " + strings.Join(cells, " | ") + " |")
		sb.WriteString("\n")
		if i == 0 {
			sb.WriteString("|")
			for range cells {
				sb.WriteString(" --- |")
			}
			sb.WriteString("\n")
			tbl.Headers = cells
		} else {
			tbl.Rows = append(tbl.Rows, cells)
		}
	}
	return sb.String(), tbl
}

func axlsxCellValue(c axlsxCell, shared []string) string {
	if c.Type == "s" {
		if idx, err := strconv.Atoi(c.Value); err == nil && idx >= 0 && idx < len(shared) {
			return shared[idx]
		}
		return ""
	}
	return c.Value
}
