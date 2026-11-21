package rag

import "encoding/json"

type UIComponentType string

const (
	CompTypeDataTable       UIComponentType = "data_table"
	CompTypeChart           UIComponentType = "chart"
	CompTypeButtonGroup     UIComponentType = "button_group"
	CompTypeStatusCard      UIComponentType = "status_card"
	CompTypeTaskTracker     UIComponentType = "task_tracker"
	CompTypeProgressBar     UIComponentType = "progress_bar"
	CompTypeMarkdownCard    UIComponentType = "markdown_card"
	CompTypeKeyValueCard    UIComponentType = "key_value_card"
	CompTypeCollapsible     UIComponentType = "collapsible_section"
	CompTypeSearchResults   UIComponentType = "search_results"
	CompTypeComparisonTable UIComponentType = "comparison_table"
	CompTypeCodeBlock       UIComponentType = "code_block"
)

type UIComponent interface {
	Type() UIComponentType
	JSON() (json.RawMessage, error)
}

type BaseComponent struct {
	CompID    string          `json:"id"`
	CompType  UIComponentType `json:"type"`
	Title     string          `json:"title,omitempty"`
	UpdatedAt int64           `json:"updated_at"`
}
