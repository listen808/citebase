package wiki

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/listen808/citebase/internal/ai/provider"
)

// Section 文章的一个章节。
type Section struct {
	Title     string   `json:"title"`
	KeyPoints []string `json:"key_points"`
	Content   string   `json:"content"`
}

// CitationIssue 写作期引用校验发现的问题。
type CitationIssue struct {
	Section string `json:"section"`
	Kind    string `json:"kind"` // missing / out_of_range
	Detail  string `json:"detail"`
}

var citationRefPattern = regexp.MustCompile(`\[(\d+)\]`)

// PlanOutline 生成分层大纲（STORM 流水线的第 2 步）。
// 本项目此前完全没有大纲生成能力，报告章节只能由调用方传入 —— 这里补上 AI 自规划。
func (c *Compiler) PlanOutline(ctx context.Context, topic, materials string) ([]Section, error) {
	if c.provider == nil {
		return nil, fmt.Errorf("AI 模型未配置，无法生成大纲")
	}
	prompt := fmt.Sprintf("主题：%s\n\n可用资料：\n%s\n\n"+
		"请设计一份分层大纲。输出 JSON 数组，每项形如 "+
		`{"title":"章节标题","key_points":["要点1","要点2"]}，章节数 4-8 个，不要解释。`,
		topic, truncate(materials, 8000))

	resp, err := c.provider.Chat(ctx,
		"你是结构化写作规划助手，负责把资料组织成逻辑清晰的分层大纲。",
		[]provider.Message{provider.NewTextMessage("user", prompt)},
		provider.WithMaxTokens(1024), provider.WithTemperature(0.3))
	if err != nil {
		return nil, err
	}

	var sections []Section
	if err := json.Unmarshal([]byte(extractJSONArray(resp.Content)), &sections); err != nil {
		return nil, fmt.Errorf("大纲解析失败: %w", err)
	}
	return sections, nil
}

// WriteSections 按大纲逐节填充正文（STORM 流水线的第 3 步）。
// 每个章节独立生成并强制引用来源，避免长文一次性生成时后半段丢引用。
func (c *Compiler) WriteSections(ctx context.Context, topic string, sections []Section, materials string) ([]Section, error) {
	if c.provider == nil {
		return nil, fmt.Errorf("AI 模型未配置，无法生成正文")
	}
	out := make([]Section, 0, len(sections))
	for i := range sections {
		s := sections[i]
		prompt := fmt.Sprintf("主题：%s\n\n本章节：%s\n要点：%s\n\n可用资料（引用时用 [n] 标注来源编号）：\n%s\n\n"+
			"请撰写该章节正文。要求：只使用资料中的信息；每个关键论断后标注 [n]；"+
			"资料中没有的信息宁可不写，不得编造。",
			topic, s.Title, strings.Join(s.KeyPoints, "、"), truncate(materials, 6000))

		resp, err := c.provider.Chat(ctx,
			"你是严谨的知识写作助手，只基于给定资料写作，并逐条标注来源。",
			[]provider.Message{provider.NewTextMessage("user", prompt)},
			provider.WithMaxTokens(2048), provider.WithTemperature(0.3))
		if err != nil {
			return out, fmt.Errorf("章节 %q 生成失败: %w", s.Title, err)
		}
		s.Content = resp.Content
		out = append(out, s)
	}
	return out, nil
}

// Polish 润色：加摘要段并去除跨章节重复（STORM 流水线的第 4 步）。
func (c *Compiler) Polish(ctx context.Context, topic string, sections []Section) (string, error) {
	var body strings.Builder
	for _, s := range sections {
		fmt.Fprintf(&body, "## %s\n\n%s\n\n", s.Title, s.Content)
	}
	if c.provider == nil {
		return body.String(), nil
	}
	prompt := fmt.Sprintf("主题：%s\n\n正文：\n%s\n\n请做两件事：1) 开头补一段 100 字以内的执行摘要；"+
		"2) 删除跨章节重复的表述。直接输出最终 Markdown，保留 [n] 引用编号。",
		topic, truncate(body.String(), 12000))

	resp, err := c.provider.Chat(ctx,
		"你是文稿编辑，负责压缩冗余并补齐摘要，不改变事实与引用编号。",
		[]provider.Message{provider.NewTextMessage("user", prompt)},
		provider.WithMaxTokens(4096), provider.WithTemperature(0.2))
	if err != nil {
		return body.String(), nil
	}
	return resp.Content, nil
}

// VerifyCitations 写作期引用校验：章节无引用 / 引用编号越界都会被标记。
// 这是确定性检查，不需要模型；它拦不住"引用了但内容与来源不符"，
// 那种语义级校验需要交给 RAGAS 或幻觉检测链路。
func VerifyCitations(sections []Section, sourceCount int) []CitationIssue {
	var issues []CitationIssue
	for _, s := range sections {
		refs := citationRefPattern.FindAllStringSubmatch(s.Content, -1)
		if len(refs) == 0 {
			issues = append(issues, CitationIssue{
				Section: s.Title, Kind: "missing",
				Detail: "该章节没有任何来源引用，论断无法溯源",
			})
			continue
		}
		for _, r := range refs {
			n := 0
			fmt.Sscanf(r[1], "%d", &n)
			if n <= 0 || n > sourceCount {
				issues = append(issues, CitationIssue{
					Section: s.Title, Kind: "out_of_range",
					Detail: fmt.Sprintf("引用编号 [%s] 超出来源数量 %d", r[1], sourceCount),
				})
			}
		}
	}
	return issues
}
