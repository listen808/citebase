package rag

import (
	"encoding/json"
	"regexp"
	"strings"
)

// Generative UI：AI 回答可携带结构化组件描述，前端按类型渲染为图表/表格/卡片/代码，
// 而非仅纯文本。这是"生成式 UI"的核心——模型输出机器可消费的语义块。

// ComponentKind 结构化组件类型。
type ComponentKind string

const (
	ComponentChart ComponentKind = "chart" // ECharts 配置（由 generate_chart_tool 产出）
	ComponentTable ComponentKind = "table" // 表格数据
	ComponentCard  ComponentKind = "card"  // 信息卡片
	ComponentCode  ComponentKind = "code"  // 代码片段
)

// GeneratedComponent 一个可渲染的结构化组件。
type GeneratedComponent struct {
	Kind  ComponentKind `json:"kind"`
	Title string        `json:"title,omitempty"`
	// ChartOption 当 Kind=chart 时，为 ECharts option（map 结构）。
	ChartOption map[string]interface{} `json:"chart_option,omitempty"`
	// TableColumns / TableRows 当 Kind=table 时。
	TableColumns []string        `json:"table_columns,omitempty"`
	TableRows    [][]interface{} `json:"table_rows,omitempty"`
	// CardFields 当 Kind=card 时，键值对。
	CardFields map[string]string `json:"card_fields,omitempty"`
	// Code / Language 当 Kind=code 时。
	Code     string `json:"code,omitempty"`
	Language string `json:"language,omitempty"`
}

// fencedBlockRe 匹配 ```lang\n...\n``` 围栏块，捕获语言与内容。
var fencedBlockRe = regexp.MustCompile("(?s)```([a-zA-Z0-9_+-]*)\\n(.*?)```")

// ExtractComponents 从回答文本中提取结构化组件。
// 约定：模型在回答中嵌入如下围栏块即可被识别：
//
//	```chart\n<ECharts option JSON>\n```
//	```table\n<JSON: {"columns":[...],"rows":[[...]]}>\n```
//	```card\n<JSON: {"title":..,"fields":{..}}>\n```
//	```code\n<源码>\n```  (language 为围栏语言)
//
// 纯文本中没有这些块时返回 nil。提取后从原文本中移除这些块（由调用方决定是否替换）。
func ExtractComponents(answer string) []GeneratedComponent {
	var comps []GeneratedComponent
	for _, m := range fencedBlockRe.FindAllStringSubmatch(answer, -1) {
		lang := strings.ToLower(strings.TrimSpace(m[1]))
		body := strings.TrimSpace(m[2])
		if body == "" {
			continue
		}
		switch lang {
		case "chart":
			var opt map[string]interface{}
			if err := json.Unmarshal([]byte(body), &opt); err == nil {
				comps = append(comps, GeneratedComponent{Kind: ComponentChart, ChartOption: opt})
			}
		case "table":
			var t struct {
				Columns []string        `json:"columns"`
				Rows    [][]interface{} `json:"rows"`
			}
			if err := json.Unmarshal([]byte(body), &t); err == nil && len(t.Columns) > 0 {
				comps = append(comps, GeneratedComponent{Kind: ComponentTable, TableColumns: t.Columns, TableRows: t.Rows})
			}
		case "card":
			var c struct {
				Title  string            `json:"title"`
				Fields map[string]string `json:"fields"`
			}
			if err := json.Unmarshal([]byte(body), &c); err == nil && len(c.Fields) > 0 {
				comps = append(comps, GeneratedComponent{Kind: ComponentCard, Title: c.Title, CardFields: c.Fields})
			}
		case "code", "":
			// 普通代码块不作为 Generative 组件（避免与答案中的说明性代码冲突），
			// 仅当显式 lang=code 时收集。
			if lang == "code" {
				comps = append(comps, GeneratedComponent{Kind: ComponentCode, Code: body})
			}
		}
	}
	return comps
}

// StripComponentBlocks 从回答中移除已提取的围栏块，返回干净文本。
func StripComponentBlocks(answer string) string {
	return fencedBlockRe.ReplaceAllString(answer, "")
}
