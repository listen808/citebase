package text2sql

import (
	airepo "github.com/listen808/citebase/internal/repository/ai"
	"github.com/listen808/citebase/internal/text2sql/dialect"
	"github.com/listen808/citebase/internal/text2sql/trust"
)

// Note: SetDialect(string) 已在 generator.go 中声明（P2-1），
// 此处仅补充 P1-9 的结构化方言访问器。

// Text2SQL 生成器：执行器 / 解释器 / 审计器 / 反馈仓储等依赖注入 setter。

// SetExecutor 设置 SQL 执行器
func (g *Generator) SetExecutor(executor *SQLExecutor) {
	g.executor = executor
}

// GetExecutor 获取 SQL 执行器
func (g *Generator) GetExecutor() *SQLExecutor {
	return g.executor
}

// SetExplainer 设置 SQL 解释器
func (g *Generator) SetExplainer(explainer *trust.SQLExplainer) {
	g.explainer = explainer
}

// SetAuditor 设置审计记录器
func (g *Generator) SetAuditor(auditor *trust.SQLAuditRecorder) {
	g.auditor = auditor
}

// GetAuditor 获取审计记录器
func (g *Generator) GetAuditor() *trust.SQLAuditRecorder {
	return g.auditor
}

// SetFeedbackRepo 设置 Text2SQL 反馈记录仓库，启用执行反馈持久化
func (g *Generator) SetFeedbackRepo(repo *airepo.Text2SQLFeedbackRepo) {
	if g.feedbackLoop == nil {
		g.feedbackLoop = NewFeedbackLoop()
	}
	g.feedbackLoop.SetRepo(repo)
}

// SetPromptTemplateRepo 设置提示词模板仓库
func (g *Generator) SetPromptTemplateRepo(repo *airepo.PromptTemplateRepo) {
	g.promptTplRepo = repo
}

// SetPromptVersionSelector 设置 Prompt 版本选择器
func (g *Generator) SetPromptVersionSelector(selector *PromptVersionSelector) {
	g.promptVersionSelector = selector
}

// GetStructuredDialect 获取当前结构化数据库方言（P1-9）
// 未设置时返回默认 MySQL 方言；与 SetDialect(string) 配合使用，
// 后者会同时初始化 dialectRewriter 与 sqlDialect 字段
func (g *Generator) GetStructuredDialect() dialect.Dialect {
	if g.sqlDialect == nil {
		return dialect.GetDialect("mysql")
	}
	return g.sqlDialect
}
