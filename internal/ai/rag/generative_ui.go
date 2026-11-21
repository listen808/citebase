package rag

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ============================================================
// 增强的生成式UI组件
// ============================================================

func init() {
	// 注册新的组件类型
}

// StreamingDataTable 流式数据表组件 - 支持分页、排序、流式更新
type StreamingDataTable struct {
	BaseComponent
	Columns    []DataTableColumn `json:"columns"`
	Rows       [][]interface{}   `json:"rows"`
	Total      int               `json:"total,omitempty"`
	Page       int               `json:"page,omitempty"`
	PageSize   int               `json:"page_size,omitempty"`
	SortBy     string            `json:"sort_by,omitempty"`
	SortOrder  string            `json:"sort_order,omitempty"` // asc / desc
	Streaming  bool              `json:"streaming"`            // 是否正在流式接收
	IsComplete bool              `json:"is_complete"`          // 数据是否接收完毕
	Actions    []TableAction     `json:"actions,omitempty"`    // 行操作按钮
}

type TableAction struct {
	Label   string `json:"label"`
	Icon    string `json:"icon,omitempty"`
	Action  string `json:"action"`            // action type: "view", "edit", "delete", "link"
	Field   string `json:"field,omitempty"`   // 关联的字段
	URL     string `json:"url,omitempty"`     // 链接地址
	Variant string `json:"variant,omitempty"` // button variant
}

func (c *StreamingDataTable) Type() UIComponentType          { return CompTypeDataTable }
func (c *StreamingDataTable) JSON() (json.RawMessage, error) { return json.Marshal(c) }

// AddRow 流式添加一行数据
func (c *StreamingDataTable) AddRow(row []interface{}) {
	c.Rows = append(c.Rows, row)
	c.Total = len(c.Rows)
}

// SetComplete 标记数据接收完毕
func (c *StreamingDataTable) SetComplete() {
	c.Streaming = false
	c.IsComplete = true
}

// SetStreaming 标记正在流式接收
func (c *StreamingDataTable) SetStreaming() {
	c.Streaming = true
	c.IsComplete = false
}

// InteractiveChart 交互式图表组件 - 支持多种图表类型和交互
type InteractiveChart struct {
	BaseComponent
	ChartType   string          `json:"chart_type"` // bar, line, pie, scatter, heatmap, radar, funnel
	Option      json.RawMessage `json:"option"`     // ECharts/Plotly option
	Height      int             `json:"height,omitempty"`
	Width       int             `json:"width,omitempty"`
	Interactive bool            `json:"interactive"`       // 是否支持交互
	DrillDown   bool            `json:"drill_down"`        // 是否支持下钻
	Exportable  bool            `json:"exportable"`        // 是否支持导出
	Actions     []ChartAction   `json:"actions,omitempty"` // 图表操作
}

type ChartAction struct {
	Label   string                 `json:"label"`
	Icon    string                 `json:"icon,omitempty"`
	Action  string                 `json:"action"` // "export_png", "export_svg", "drill_down", "toggle_legend"
	Options map[string]interface{} `json:"options,omitempty"`
}

func (c *InteractiveChart) Type() UIComponentType          { return CompTypeChart }
func (c *InteractiveChart) JSON() (json.RawMessage, error) { return json.Marshal(c) }

// MarkdownCard 富Markdown内容卡片
type MarkdownCard struct {
	BaseComponent
	Content     string            `json:"content"`               // Markdown内容
	Markdown    bool              `json:"markdown"`              // 是否渲染为Markdown
	Collapsible bool              `json:"collapsible,omitempty"` // 是否可折叠
	Collapsed   bool              `json:"collapsed,omitempty"`   // 默认是否折叠
	MaxHeight   int               `json:"max_height,omitempty"`  // 最大高度(px)
	Actions     []CardAction      `json:"actions,omitempty"`     // 卡片操作
	Metadata    map[string]string `json:"metadata,omitempty"`    // 元数据
}

type CardAction struct {
	Label   string `json:"label"`
	Icon    string `json:"icon,omitempty"`
	Action  string `json:"action"`
	Variant string `json:"variant,omitempty"` // primary, secondary, ghost, danger
}

func (c *MarkdownCard) Type() UIComponentType          { return "markdown_card" }
func (c *MarkdownCard) JSON() (json.RawMessage, error) { return json.Marshal(c) }

// KeyValueCard 键值对展示卡片
type KeyValueCard struct {
	BaseComponent
	Items   []KeyValueItem `json:"items"`
	Layout  string         `json:"layout,omitempty"`  // "grid", "list", "table"
	Columns int            `json:"columns,omitempty"` // grid布局列数
}

type KeyValueItem struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	KeyIcon   string `json:"key_icon,omitempty"`
	ValueIcon string `json:"value_icon,omitempty"`
	Link      string `json:"link,omitempty"`  // 可点击链接
	Color     string `json:"color,omitempty"` // 值的颜色
}

func (c *KeyValueCard) Type() UIComponentType          { return "key_value_card" }
func (c *KeyValueCard) JSON() (json.RawMessage, error) { return json.Marshal(c) }

// CollapsibleSection 可折叠内容区
type CollapsibleSection struct {
	BaseComponent
	Content  string `json:"content"`
	Markdown bool   `json:"markdown"`
	Expanded bool   `json:"expanded"`
	Icon     string `json:"icon,omitempty"`
	Badge    string `json:"badge,omitempty"` // 折叠时显示的摘要
}

func (c *CollapsibleSection) Type() UIComponentType          { return "collapsible_section" }
func (c *CollapsibleSection) JSON() (json.RawMessage, error) { return json.Marshal(c) }

// SearchResultsCard 搜索结果展示卡片
type SearchResultsCard struct {
	BaseComponent
	Query   string             `json:"query"`
	Results []SearchResultItem `json:"results"`
	Total   int                `json:"total"`
	Page    int                `json:"page"`
	HasMore bool               `json:"has_more"`
	TimeMs  int64              `json:"time_ms"`
}

type SearchResultItem struct {
	Title       string            `json:"title"`
	Snippet     string            `json:"snippet"`
	Source      string            `json:"source,omitempty"`
	DocURL      string            `json:"doc_url,omitempty"`
	Score       float64           `json:"score"`
	Highlighted string            `json:"highlighted,omitempty"` // 高亮的匹配文本
	Metadata    map[string]string `json:"metadata,omitempty"`
}

func (c *SearchResultsCard) Type() UIComponentType          { return "search_results" }
func (c *SearchResultsCard) JSON() (json.RawMessage, error) { return json.Marshal(c) }

// ComparisonTable 对比表格组件
type ComparisonTable struct {
	BaseComponent
	Headers    []string        `json:"headers"`
	Rows       []ComparisonRow `json:"rows"`
	Highlights map[int]string  `json:"highlights,omitempty"` // column index -> color
}

type ComparisonRow struct {
	Label    string        `json:"label"`
	Values   []interface{} `json:"values"`
	IsHeader bool          `json:"is_header,omitempty"`
}

func (c *ComparisonTable) Type() UIComponentType          { return "comparison_table" }
func (c *ComparisonTable) JSON() (json.RawMessage, error) { return json.Marshal(c) }

// CodeBlock 代码块组件
type CodeBlock struct {
	BaseComponent
	Code        string `json:"code"`
	Language    string `json:"language"`
	ShowLineNum bool   `json:"show_line_num"`
	Highlight   string `json:"highlight,omitempty"` // 高亮的行号范围，如 "1-5,8"
	Copyable    bool   `json:"copyable"`
}

func (c *CodeBlock) Type() UIComponentType          { return "code_block" }
func (c *CodeBlock) JSON() (json.RawMessage, error) { return json.Marshal(c) }

// ============================================================
// 组件构建器 - 方便创建常用组件
// ============================================================

// BuildStreamingTable 构建流式数据表
func BuildStreamingTable(title string, columns []DataTableColumn) *StreamingDataTable {
	return &StreamingDataTable{
		BaseComponent: BaseComponent{
			CompType: CompTypeDataTable,
			Title:    title,
		},
		Columns:    columns,
		Rows:       make([][]interface{}, 0),
		Streaming:  true,
		IsComplete: false,
		PageSize:   20,
	}
}

// BuildChart 构建交互式图表
func BuildChart(title, chartType string, option json.RawMessage) *InteractiveChart {
	return &InteractiveChart{
		BaseComponent: BaseComponent{
			CompType: CompTypeChart,
			Title:    title,
		},
		ChartType:   chartType,
		Option:      option,
		Height:      300,
		Interactive: true,
		Exportable:  true,
	}
}

// BuildMarkdownCard 构建Markdown卡片
func BuildMarkdownCard(title, content string) *MarkdownCard {
	return &MarkdownCard{
		BaseComponent: BaseComponent{
			CompType: "markdown_card",
			Title:    title,
		},
		Content:  content,
		Markdown: true,
	}
}

// BuildKeyValueCard 构建键值对卡片
func BuildKeyValueCard(title string, items []KeyValueItem) *KeyValueCard {
	return &KeyValueCard{
		BaseComponent: BaseComponent{
			CompType: "key_value_card",
			Title:    title,
		},
		Items:  items,
		Layout: "list",
	}
}

// BuildSearchResults 构建搜索结果卡片
func BuildSearchResults(title, query string, results []SearchResultItem, timeMs int64) *SearchResultsCard {
	return &SearchResultsCard{
		BaseComponent: BaseComponent{
			CompType: "search_results",
			Title:    title,
		},
		Query:   query,
		Results: results,
		Total:   len(results),
		TimeMs:  timeMs,
	}
}

// BuildCodeBlock 构建代码块
func BuildCodeBlock(title, code, language string) *CodeBlock {
	return &CodeBlock{
		BaseComponent: BaseComponent{
			CompType: "code_block",
			Title:    title,
		},
		Code:     code,
		Language: language,
		Copyable: true,
	}
}

// BuildComparisonTable 构建对比表格
func BuildComparisonTable(title string, headers []string, rows []ComparisonRow) *ComparisonTable {
	return &ComparisonTable{
		BaseComponent: BaseComponent{
			CompType: "comparison_table",
			Title:    title,
		},
		Headers: headers,
		Rows:    rows,
	}
}

// ============================================================
// RAG答案的结构化输出 - 结合多种组件
// ============================================================

// RAGAnswerWithUI 带生成式UI的RAG答案
type RAGAnswerWithUI struct {
	Answer     string        `json:"answer"`
	Sources    []Source      `json:"sources"`
	Components []UIComponent `json:"components,omitempty"` // 生成式UI组件
	Citations  []Citation    `json:"citations,omitempty"`
}

// BuildRAGAnswerWithTable 为RAG答案添加数据表组件
func BuildRAGAnswerWithTable(answer string, columns []DataTableColumn, rows [][]interface{}) *RAGAnswerWithUI {
	table := BuildStreamingTable("查询结果", columns)
	table.Rows = rows
	table.Total = len(rows)
	table.Streaming = false
	table.IsComplete = true

	return &RAGAnswerWithUI{
		Answer:     answer,
		Components: []UIComponent{table},
	}
}

// BuildRAGAnswerWithChart 为RAG答案添加图表组件
func BuildRAGAnswerWithChart(answer string, chartType string, option json.RawMessage) *RAGAnswerWithUI {
	chart := BuildChart("数据可视化", chartType, option)

	return &RAGAnswerWithUI{
		Answer:     answer,
		Components: []UIComponent{chart},
	}
}

// BuildRAGAnswerWithEverything 为RAG答案添加所有组件
func BuildRAGAnswerWithEverything(answer string, sources []Source, columns []DataTableColumn, rows [][]interface{}, chartOption json.RawMessage) *RAGAnswerWithUI {
	result := &RAGAnswerWithUI{
		Answer:  answer,
		Sources: sources,
	}

	// 添加数据表
	if len(columns) > 0 && len(rows) > 0 {
		table := BuildStreamingTable("查询结果", columns)
		table.Rows = rows
		table.Total = len(rows)
		table.Streaming = false
		table.IsComplete = true
		result.Components = append(result.Components, table)
	}

	// 添加图表
	if chartOption != nil {
		chart := BuildChart("数据可视化", "bar", chartOption)
		result.Components = append(result.Components, chart)
	}

	// 添加来源卡片
	if len(sources) > 0 {
		items := make([]KeyValueItem, 0, len(sources))
		for i, src := range sources {
			items = append(items, KeyValueItem{
				Key:   fmt.Sprintf("来源 %d", i+1),
				Value: src.DocumentName,
				Link:  src.DocURL,
			})
		}
		sourcesCard := BuildKeyValueCard("参考来源", items)
		result.Components = append(result.Components, sourcesCard)
	}

	return result
}

// FormatSourcesAsSearchResults 将来源格式化为搜索结果卡片
func FormatSourcesAsSearchResults(query string, sources []Source, timeMs int64) *SearchResultsCard {
	results := make([]SearchResultItem, 0, len(sources))
	for _, src := range sources {
		results = append(results, SearchResultItem{
			Title:   src.DocumentName,
			Snippet: src.Snippet,
			DocURL:  src.DocURL,
			Score:   src.Score,
		})
	}
	return BuildSearchResults("搜索结果", query, results, timeMs)
}

// FormatAnswerAsMarkdownCard 将答案格式化为Markdown卡片
func FormatAnswerAsMarkdownCard(answer string, sources []Source) *MarkdownCard {
	var content strings.Builder
	content.WriteString(answer)

	if len(sources) > 0 {
		content.WriteString("\n\n---\n\n**参考来源:**\n\n")
		for i, src := range sources {
			content.WriteString(fmt.Sprintf("%d. [%s](%s) (相关度: %.0f%%)\n",
				i+1, src.DocumentName, src.DocURL, src.Score*100))
		}
	}

	return BuildMarkdownCard("AI 回答", content.String())
}
