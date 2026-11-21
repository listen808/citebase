package analyzer

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/listen808/citebase/internal/log"
	"strings"
	"time"

	"github.com/listen808/citebase/internal/ai/provider"
	"github.com/listen808/citebase/internal/utils"
)

// EventExtractor 事件抽取器 - 从文本中识别事件及其参与者、时间、地点
type EventExtractor struct {
	provider provider.Provider
}

// NewEventExtractor 创建事件抽取器
func NewEventExtractor(p provider.Provider) *EventExtractor {
	return &EventExtractor{provider: p}
}

// ExtractedEvent 抽取的事件
type ExtractedEvent struct {
	EventName    string   `json:"event_name"`   // 事件名称
	EventType    string   `json:"event_type"`   // 事件类型
	Trigger      string   `json:"trigger"`      // 触发词
	Participants []string `json:"participants"` // 参与者
	Time         string   `json:"time"`         // 时间
	Location     string   `json:"location"`     // 地点
	Description  string   `json:"description"`  // 事件描述
	Impact       string   `json:"impact"`       // 影响
	Confidence   float64  `json:"confidence"`   // 置信度 0-1
}

// EventExtractionResult 事件抽取结果
type EventExtractionResult struct {
	Events []ExtractedEvent `json:"events"`
}

// ExtractEvents 从文本中抽取事件
func (e *EventExtractor) ExtractEvents(ctx context.Context, content string) (*EventExtractionResult, error) {
	if e.provider == nil {
		return nil, fmt.Errorf("provider not configured")
	}

	if len(content) > 8000 {
		content = utils.TruncateRunes(content, 8000)
	}

	prompt := `请从以下文本中识别并抽取所有事件。

事件类型包括：
- deployment: 部署/发布事件
- incident: 故障/事故事件
- meeting: 会议事件
- decision: 决策事件
- change: 变更事件
- release: 版本发布事件
- migration: 迁移事件
- integration: 集成事件
- approval: 审批事件
- other: 其他事件

对于每个事件，请抽取：
- 事件名称
- 事件类型
- 触发词（表示事件发生的动词或名词）
- 参与者（人、团队、系统等）
- 时间（如果提到）
- 地点（如果提到）
- 事件描述
- 影响（事件的影响或结果）
- 置信度（0-1）

文本内容：
%s

请以JSON格式输出：
{
  "events": [
    {
      "event_name": "事件名称",
      "event_type": "事件类型",
      "trigger": "触发词",
      "participants": ["参与者1", "参与者2"],
      "time": "时间",
      "location": "地点",
      "description": "事件描述",
      "impact": "影响",
      "confidence": 0.9
    }
  ]
}

只输出JSON，不要其他内容。`

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := e.provider.Chat(ctx, "你是一个事件抽取专家，擅长从文本中识别事件及其要素。只输出JSON。", []provider.Message{
		provider.NewTextMessage("user", fmt.Sprintf(prompt, content)),
	}, provider.WithMaxTokens(2048), provider.WithTemperature(0.1))
	if err != nil {
		log.Errorf("[EventExtractor] LLM extract error: %v", err)
		return nil, err
	}

	result := &EventExtractionResult{
		Events: []ExtractedEvent{},
	}

	jsonStr := utils.ExtractJSON(resp.Content)
	if err := json.Unmarshal([]byte(jsonStr), result); err != nil {
		// 模型有时返回裸数组而非 {"events": [...]}，归一化后重试
		var events []ExtractedEvent
		if arrErr := json.Unmarshal([]byte(jsonStr), &events); arrErr == nil {
			result.Events = events
			return result, nil
		}
		log.Errorf("[EventExtractor] JSON parse error: %v", err)
		return nil, err
	}

	return result, nil
}

// ExtractEventsFromSummaries 从多个文档摘要中抽取事件
func (e *EventExtractor) ExtractEventsFromSummaries(ctx context.Context, summaries []DocSummary) ([]ExtractedEvent, error) {
	if e.provider == nil || len(summaries) == 0 {
		return nil, nil
	}

	var sb strings.Builder
	sb.WriteString("以下是多个文档的摘要，请识别其中的事件：\n\n")

	for i, s := range summaries {
		if i >= 10 { // 限制最多10个文档
			break
		}
		sb.WriteString(fmt.Sprintf("文档%d (%s):\n", s.SourceDocID, s.SourceName))
		sb.WriteString(fmt.Sprintf("摘要: %s\n", s.Summary))
		if len(s.KeyPoints) > 0 {
			sb.WriteString(fmt.Sprintf("要点: %s\n", strings.Join(s.KeyPoints, "; ")))
		}
		sb.WriteString("\n")
	}

	prompt := `请从以上文档信息中识别所有事件，并按类型分类。

输出JSON数组格式：
[
  {
    "event_name": "事件名称",
    "event_type": "事件类型",
    "trigger": "触发词",
    "participants": ["参与者"],
    "time": "时间",
    "location": "地点",
    "description": "描述",
    "impact": "影响",
    "confidence": 0.9
  }
]

只输出JSON数组，不要其他内容。`

	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := e.provider.Chat(ctx, "你是一个事件抽取专家。只输出JSON数组。", []provider.Message{
		provider.NewTextMessage("user", sb.String()+"\n"+prompt),
	}, provider.WithMaxTokens(2048), provider.WithTemperature(0.1))
	if err != nil {
		log.Errorf("[EventExtractor] Cross-doc extract error: %v", err)
		return nil, err
	}

	events, err := parseJSONAsEventArray(utils.ExtractJSON(resp.Content))
	if err != nil {
		log.Errorf("[EventExtractor] JSON parse error: %v", err)
		return nil, err
	}

	return events, nil
}

// parseJSONAsEventArray 将 JSON 字符串解析为 []ExtractedEvent。
// 模型有时返回单个对象而非数组，自动包一层数组归一化。
func parseJSONAsEventArray(jsonStr string) ([]ExtractedEvent, error) {
	var events []ExtractedEvent
	if err := json.Unmarshal([]byte(jsonStr), &events); err == nil {
		return events, nil
	}
	var single ExtractedEvent
	if err := json.Unmarshal([]byte(jsonStr), &single); err == nil {
		return []ExtractedEvent{single}, nil
	}
	preview := jsonStr
	if len(preview) > 200 {
		preview = preview[:200] + "..."
	}
	return nil, fmt.Errorf("expect JSON array of events, got: %s", preview)
}
