package rag

import "encoding/json"

type DataTableColumn struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Width string `json:"width,omitempty"`
	Align string `json:"align,omitempty"`
}

type DataTableComponent struct {
	BaseComponent
	Columns []DataTableColumn `json:"columns"`
	Rows    [][]interface{}   `json:"rows"`
	Total   int               `json:"total,omitempty"`
}

func (c *DataTableComponent) Type() UIComponentType { return CompTypeDataTable }
func (c *DataTableComponent) JSON() (json.RawMessage, error) {
	return json.Marshal(c)
}

type ChartComponent struct {
	BaseComponent
	ChartType string          `json:"chart_type"`
	Option    json.RawMessage `json:"option"`
	Height    int             `json:"height,omitempty"`
}

func (c *ChartComponent) Type() UIComponentType          { return CompTypeChart }
func (c *ChartComponent) JSON() (json.RawMessage, error) { return json.Marshal(c) }

type ButtonAction struct {
	Action  string                 `json:"action"`
	Payload map[string]interface{} `json:"payload"`
}

type ButtonItem struct {
	Label    string       `json:"label"`
	Variant  string       `json:"variant"`
	Action   ButtonAction `json:"action"`
	Disabled bool         `json:"disabled,omitempty"`
}

type ButtonGroupComponent struct {
	BaseComponent
	Buttons []ButtonItem `json:"buttons"`
}

func (c *ButtonGroupComponent) Type() UIComponentType          { return CompTypeButtonGroup }
func (c *ButtonGroupComponent) JSON() (json.RawMessage, error) { return json.Marshal(c) }

type StatusItem struct {
	Label string `json:"label"`
	Value string `json:"value"`
	Icon  string `json:"icon,omitempty"`
	Color string `json:"color,omitempty"`
}

type StatusCardComponent struct {
	BaseComponent
	Items []StatusItem `json:"items"`
}

func (c *StatusCardComponent) Type() UIComponentType          { return CompTypeStatusCard }
func (c *StatusCardComponent) JSON() (json.RawMessage, error) { return json.Marshal(c) }

type TaskStatus string

const (
	TaskPending TaskStatus = "pending"
	TaskRunning TaskStatus = "running"
	TaskSuccess TaskStatus = "success"
	TaskFailed  TaskStatus = "failed"
	TaskSkipped TaskStatus = "skipped"
)

type TaskStep struct {
	StepID     string     `json:"id"`
	Name       string     `json:"name"`
	Status     TaskStatus `json:"status"`
	Detail     string     `json:"detail,omitempty"`
	DurationMs int64      `json:"duration_ms,omitempty"`
}

type TaskTrackerComponent struct {
	BaseComponent
	Steps      []TaskStep `json:"steps"`
	CurrentIdx int        `json:"current_idx"`
	TotalMs    int64      `json:"total_ms,omitempty"`
}

func (c *TaskTrackerComponent) Type() UIComponentType          { return CompTypeTaskTracker }
func (c *TaskTrackerComponent) JSON() (json.RawMessage, error) { return json.Marshal(c) }

type ProgressBarComponent struct {
	BaseComponent
	Percent   float64 `json:"percent"`
	Status    string  `json:"status,omitempty"`
	ShowLabel bool    `json:"show_label"`
}

func (c *ProgressBarComponent) Type() UIComponentType          { return CompTypeProgressBar }
func (c *ProgressBarComponent) JSON() (json.RawMessage, error) { return json.Marshal(c) }
