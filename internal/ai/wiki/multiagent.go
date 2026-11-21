package wiki

import (
	"context"
	"strconv"
	"strings"

	logx "github.com/listen808/citebase/internal/log"

	adminrepo "github.com/listen808/citebase/internal/repository/admin"
)

// MultiagentRunner 多智能体编排器的最小接口（避免 wiki → multiagent → rag → mcp → wiki 导入循环）。
// 实现方是 API 层包装的适配器，调用 multiagent.Orchestrator.Run 时传 nil history。
type MultiagentRunner interface {
	// Run 执行一次多智能体研究闭环（规划→检索→综合→批判→修订），返回最终答案。
	Run(ctx context.Context, question string) (string, error)
}

// MultiagentEnabled 读取 wiki.multiagent.enabled 配置开关。
// 未设置或读取失败返回 false（安全默认：关闭）。
func MultiagentEnabled() bool {
	v, err := adminrepo.NewConfigRepository().GetValueByKey("wiki.multiagent.enabled")
	if err != nil {
		return false
	}
	b, _ := strconv.ParseBool(v)
	return b
}

// generateQueriesViaMultiagent 使用多智能体编排器生成研究查询。
// researcher 角色拥有检索类工具，能结合知识库上下文产出更精准的研究查询。
// 失败时退化为单轮 LLM 路径（generateResearchQueries）。
func (c *Compiler) generateQueriesViaMultiagent(ctx context.Context, ins Insight, runner MultiagentRunner) []string {
	fallback := make([]string, 0, len(ins.Slugs))
	for _, s := range ins.Slugs {
		fallback = append(fallback, strings.ReplaceAll(s, "-", " "))
	}

	question := "知识图谱体检发现：[" + ins.Kind + "] " + ins.Detail +
		"（相关页面：" + strings.Join(ins.Slugs, "、") + "）\n" +
		"请生成 " + strconv.Itoa(deepResearchMaxQueries) + " 条用于补充研究该缺口的检索查询，每行一条，不要编号和解释。"

	answer, err := runner.Run(ctx, question)
	if err != nil {
		logx.Infof("[WikiMultiagent] multiagent query gen failed: %v, falling back to single LLM", err)
		return c.generateResearchQueries(ctx, ins)
	}

	var out []string
	for _, line := range strings.Split(answer, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && len(line) > 2 {
			out = append(out, line)
		}
		if len(out) >= deepResearchMaxQueries {
			break
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

// generateContentViaMultiagent 使用多智能体编排器生成研究纪要。
// researcher 检索 + writer 综合 + critic 审查的完整闭环，
// 比单轮 LLM 产出质量更高。失败时退化为单轮 LLM 路径。
func (c *Compiler) generateContentViaMultiagent(ctx context.Context, queries []string, material string, runner MultiagentRunner) string {
	question := "研究问题：\n" + strings.Join(queries, "\n") +
		"\n\n已有材料：\n" + truncate(material, 8000) +
		"\n\n请基于以上材料撰写研究纪要（已知综合 + 缺口分析 + 下一步），只使用材料中存在的信息，绝不编造。"

	answer, err := runner.Run(ctx, question)
	if err != nil {
		logx.Infof("[WikiMultiagent] multiagent content gen failed: %v, falling back to single LLM", err)
		return c.generateResearchContent(ctx, queries, material)
	}
	return strings.TrimSpace(answer)
}

// SetMultiagentOrchestrator 注入多智能体编排器（由 API 层在配置开启时调用）。
// 未注入时 DeepResearch 走单轮 LLM 路径。
func (c *Compiler) SetMultiagentOrchestrator(runner MultiagentRunner) {
	c.multiagentOrch = runner
}
