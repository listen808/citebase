package rag

import (
	logx "github.com/listen808/citebase/internal/log"

	adminsvc "github.com/listen808/citebase/internal/service/admin"
	"github.com/listen808/citebase/internal/utils/config"
)

// RegisterBuiltinTools 注册 Agent 内置工具。
//
// 能力边界由 ai.agent_tools 配置决定：execute_code / execute_command / file_operation
// 的输入直接来自模型输出，默认不注册（工具不存在时模型无法调用），
// 只有显式打开并给出白名单时才注册，避免「任何登录用户 = 服务器 shell」。
func RegisterBuiltinTools(r *ToolRegistry, emailService *adminsvc.EmailService) {
	policy := config.AppConfig.AI.AgentTools

	r.Register("edit_document", NewEditDocumentTool())
	r.Register("search_user", NewSearchUserTool())
	r.Register("send_email", NewSendEmailTool(emailService))
	r.Register("calculate", NewCalculateTool())
	r.Register("fetch_web", NewFetchWebToolWithPolicy(policy.AllowedDomains))
	r.Register("generate_chart", NewGenerateChartTool())
	r.Register("query_stats", NewQueryStatsTool())
	r.Register("get_current_time", NewGetCurrentTimeTool())
	r.Register("list_spaces", NewListSpacesTool())
	r.Register("get_document", NewGetDocumentTool())
	r.Register("list_documents", NewListDocumentsTool())
	r.Register("string_manipulate", NewStringManipulateTool())
	r.Register("json_formatter", NewJSONFormatterTool())
	r.Register("random_number", NewRandomNumberTool())
	r.Register("web_search", NewWebSearchTool())
	r.Register("api_call", NewAPICallToolWithDomains(policy.AllowedDomains))

	registered := 16
	skipped := make([]string, 0, 4)

	if policy.EnableCodeExecutor {
		r.Register("execute_code", NewCodeExecutorTool())
		registered++
	} else {
		skipped = append(skipped, "execute_code")
	}

	if policy.EnableFileOperation {
		// 未配置 allowed_dirs 时构造函数会保持空集合，工具内部按 fail-closed 拒绝所有路径
		r.Register("file_operation", NewFileOperationTool(policy.AllowedDirs...))
		registered++
	} else {
		skipped = append(skipped, "file_operation")
	}

	if policy.EnableSystemCommand {
		r.Register("execute_command", NewSystemCommandToolWithPolicy(policy.AllowedCommands, policy.TimeoutSeconds))
		registered++
	} else {
		skipped = append(skipped, "execute_command")
	}

	if policy.EnableDatabaseQuery {
		// query_database 由 ai.go 在拿到业务库连接后注册，这里只提示开关状态
		logx.Info("[AI] query_database 已按配置启用（请确保使用只读账号并配置行数上限）")
	} else {
		skipped = append(skipped, "query_database")
	}

	if len(skipped) > 0 {
		logx.Infof("[AI] Agent 工具注册完成：%d 个可用，%d 个高危工具因未开启而跳过：%v",
			registered, len(skipped), skipped)
	} else {
		logx.Infof("[AI] Agent 工具注册完成：%d 个可用", registered)
	}
}
