package response

// 这张表是"后端说给人听的那句话"的唯一目录。
//
// 为什么要它：实测有 245 处把中文直接塞进响应封套的 message 字段（37 个文件、去重后 123 条），
// 而前端手里只有 err.message 这一根管子 —— ErrorBoundary 的报错页、toast、登录页的错误位
// 显示的都是它。于是英文界面的用户读到一句中文，且**前端修不了**：那句话不是前端产的字符串，
// 换任何语言包都盖不住。这是 i18n 十四批之外唯一一类"前端够不着"的欠账。
//
// 形状：响应里多带一个 message_key（稳定、机器可读），message 仍是中文原文 ——
// curl、CLI、邮件模板与不认识键的旧部署还要读它。前端在 ApiError 构造那一刻
// 按界面语言换成译文（web/src/api/client.ts），所以现存所有显示 err.message 的地方
// 自动跟着变，不必改两百个调用点。
//
// 三条规矩由 tools/repocheck/server_message_test.go 与 web/tests/server-messages.test.ts 钉住：
//   1. 键名是公开契约：只增不改不删（改了旧前端读不到译文，会退回中文，不是崩但语言就错了）；
//   2. 每种语言都必须有非空文案，且不许出现 %s/%d/{x} 这类插值 —— 要带参数就另开键，
//      因为封套里没有参数位，插值会原样印到界面上；
//   3. 前端语言包是这张表的生成物（node web/scripts/gen-server-messages.mjs），
//      生成物没跟上就是红。

// 稳定键名。前端的 web/src/i18n/locales/server-messages.ts 里出现的就是这些字符串。
const (
	MsgTooManyRequests             = "too_many_requests"
	MsgActionTooFrequent           = "action_too_frequent"
	MsgRefreshTooFrequent          = "refresh_too_frequent"
	MsgInvalidCredentials          = "invalid_credentials" //nolint:gosec // G101: i18n message key, not a credential
	MsgCredentialsEmpty            = "credentials_empty"
	MsgAuthFailed                  = "auth_failed"
	MsgLoginFailed                 = "login_failed"
	MsgSSOAuthCodeInvalid          = "sso_auth_code_invalid"
	MsgSSONameTaken                = "sso_name_taken"
	MsgSSONotFound                 = "sso_not_found"
	MsgSSOPrefixTaken              = "sso_prefix_taken"
	MsgAlreadyInitialized          = "already_initialized"
	MsgConcurrentInit              = "concurrent_init"
	MsgNotSignedIn                 = "not_signed_in"
	MsgNoFileUploaded              = "no_file_uploaded"
	MsgInvalidDocID                = "invalid_doc_id"
	MsgInvalidAttachmentID         = "invalid_attachment_id"
	MsgInvalidSourceKind           = "invalid_source_kind"
	MsgAttachmentNotFound          = "attachment_not_found"
	MsgDocNotFound                 = "doc_not_found"
	MsgNoPermissionDeleteAttach    = "no_permission_delete_attachment"
	MsgNoPermissionUploadAttach    = "no_permission_upload_attachment"
	MsgNoPermissionReadAttach      = "no_permission_read_attachment"
	MsgWrongSharePassword          = "wrong_share_password"
	MsgSpaceDisallowsShare         = "space_disallows_share"
	MsgSpaceDisallowsExport        = "space_disallows_export"
	MsgDocModifiedByOther          = "doc_modified_by_other"
	MsgNoPermissionExportDoc       = "no_permission_export_document"
	MsgNoPermissionVersionHistory  = "no_permission_version_history"
	MsgNoPermissionHistoricalDoc   = "no_permission_historical_document"
	MsgNoPermissionCompareVersions = "no_permission_compare_versions"
	MsgNoPermissionDocComments     = "no_permission_document_comments"
	MsgNoPermissionSpace           = "no_permission_space"
	MsgNoPermissionSpaceContent    = "no_permission_space_content"
	MsgNoPermissionSpaceGraph      = "no_permission_space_graph"
	MsgNoPermissionAnalyzeSpace    = "no_permission_analyze_space"
	MsgNoPermissionDocSpace        = "no_permission_document_space"
	MsgNoPermissionKBSSpace        = "no_permission_kbs_space"
	MsgSpaceNameTaken              = "space_name_taken"
	MsgSpaceNameIllegalChars       = "space_name_illegal_chars"
	MsgTemplateNameEditDenied      = "no_permission_template_edit"
	MsgContactNotFound             = "contact_not_found"
	MsgContactFieldsEmpty          = "contact_fields_empty"
	MsgNoContactChosen             = "no_contact_chosen"
	MsgNoLinkChosen                = "no_link_chosen"
	MsgLinkNotFound                = "link_not_found"
	MsgLinkIDInvalid               = "link_id_invalid"
	MsgLinkNameTaken               = "link_name_taken"
	MsgLinkFieldsEmpty             = "link_fields_empty"
	MsgEmailServerNameTaken        = "email_server_name_taken"
	MsgPluginKeyTaken              = "plugin_key_taken"
	MsgOpenAPIUsersModuleDenied    = "openapi_users_module_denied"
	MsgRestoreNeedsConfirm         = "restore_needs_confirm"
	MsgRequestBodyUnreadable       = "request_body_unreadable"
	MsgInvalidParameter            = "invalid_parameter"
	MsgParameterError              = "parameter_error"
	MsgIDInvalid                   = "id_invalid"
	MsgIDParamInvalid              = "id_param_invalid"
	MsgHistoryIDInvalid            = "history_id_invalid"
	MsgSpaceIDInvalid              = "space_id_invalid"
	MsgTextEmpty                   = "text_empty"
	MsgInvalidNumericID            = "invalid_numeric_id"
	MsgInvalidDataSourceID         = "invalid_data_source_id"
	MsgFileTooLarge                = "file_too_large"
	MsgNoFileChosen                = "no_file_chosen"
	MsgURLRequired                 = "url_required"
	MsgActiveFieldRequired         = "active_field_required"
	MsgDocIDsRequired              = "doc_ids_required"
	MsgQuestionRequired            = "question_required"
	MsgSpaceIDRequired             = "space_id_required"
	MsgSpaceIDAndSlugRequired      = "space_id_and_slug_required"
	MsgSpaceIDAndSourceDocRequired = "space_id_and_source_doc_required"
	MsgSpacePageClaimRequired      = "space_page_claim_required"
	MsgTopicRequired               = "topic_required"
	MsgConversationNotFound        = "conversation_not_found"
	MsgNoPermissionConversation    = "no_permission_conversation"
	MsgNoPermissionConversationUpd = "no_permission_conversation_update"
	MsgNoPermissionConversationDel = "no_permission_conversation_delete"
	MsgWorkflowNotFound            = "workflow_not_found"
	MsgWorkflowNameEmpty           = "workflow_name_empty"
	MsgNoPermissionSQLHistory      = "no_permission_sql_history"
	MsgNoPermissionSQLHistoryRead  = "no_permission_sql_history_read"
	MsgNoPermissionKBChunks        = "no_permission_kb_chunks"
	MsgNoPermissionKBRecallTest    = "no_permission_kb_recall_test"
	MsgNoPermissionKBStats         = "no_permission_kb_stats"
	MsgPageNotFound                = "page_not_found"
	MsgPageUpdated                 = "page_updated"
	MsgPageVersionNotFound         = "page_version_not_found"
	MsgWikiNoCompilableContent     = "wiki_no_compilable_content"
	MsgHostHeaderUnsafeScript      = "host_header_unsafe_script"
	MsgHostHeaderUnsafePage        = "host_header_unsafe_page"
	MsgHostHeaderUnsafeEmbed       = "host_header_unsafe_embed"
	MsgAIServiceDisabled           = "ai_service_disabled"
	MsgAIServiceUnconfigured       = "ai_service_unconfigured"
	MsgAIRetrieverUnconfigured     = "ai_retriever_unconfigured"
	MsgAIPrimaryModelUnconfigured  = "ai_primary_model_unconfigured"
	MsgAIModelUnconfigured         = "ai_model_unconfigured"
	MsgAIAnalyzerUnconfigured      = "ai_analyzer_unconfigured"
	MsgAIRecommendUnconfigured     = "ai_recommend_unconfigured"
	MsgAIReportWriterUnconfigured  = "ai_report_writer_unconfigured"
	MsgAITranslatorUnconfigured    = "ai_translator_unconfigured"
	MsgAITableWriterUnconfigured   = "ai_table_writer_unconfigured"
	MsgAIEmailWriterUnconfigured   = "ai_email_writer_unconfigured"
	MsgAIQualityUnconfigured       = "ai_quality_unconfigured"
	MsgAIAdaptiveUnconfigured      = "ai_adaptive_retriever_unconfigured"
	MsgAIRAGASUninitialized        = "ai_ragas_uninitialized"
	MsgAICronUnconfigured          = "ai_cron_unconfigured"
	MsgAIEmbeddingUnconfigured     = "ai_embedding_unconfigured"
	MsgAIChatFailed                = "ai_chat_failed"
	MsgAIGenerateFailed            = "ai_generate_failed"
	MsgAISafetyBlocked             = "ai_safety_blocked"
	MsgMemoryCreateFailed          = "memory_create_failed"
	MsgMemoryDeleteFailed          = "memory_delete_failed"
	MsgMemorySearchFailed          = "memory_search_failed"
	MsgMemoryRecallFailed          = "memory_recall_failed"
	MsgMemoryListFailed            = "memory_list_failed"
	MsgMemoryUpdateFailed          = "memory_update_failed"
	MsgMemoryStatsFailed           = "memory_stats_failed"
	MsgAccountLocked               = "account_locked"
	MsgHistoryNotFound             = "history_not_found"
	MsgHistoryQueryFailed          = "history_query_failed"
	MsgExplanationParseFailed      = "explanation_parse_failed"
	MsgAuditRecordNotFound         = "audit_record_not_found"
	MsgRecordNotFound              = "record_not_found"
	MsgDataSourceNotFound          = "data_source_not_found"
	MsgDataSourceConnectFailed     = "data_source_connect_failed"
	MsgConfluenceConvertFailed     = "confluence_convert_failed"
	MsgNotionConvertFailed         = "notion_convert_failed"
	MsgSpaceNameIllegalCharsHint   = "space_name_illegal_chars_hint"
	MsgAudioMissingField           = "audio_field_missing"
	MsgAudioOpenFailed             = "audio_open_failed"
	MsgAudioReadFailed             = "audio_read_failed"
	// 系统设置开关拒绝（搜索组）
	MsgFulltextSearchDisabled = "fulltext_search_disabled"
	// 向量重建的异步受理（数据字段里的中文迁到键：两个 handler 发同一句）
	MsgVectorReindexStarted = "vector_reindex_started"
	// 封套级兜底（internal/response 的共享 sentinel 与 500 那一档）
	MsgSignedOutOrTokenInvalid = "signed_out_or_token_invalid"
	MsgResourceNotFound        = "resource_not_found"
	MsgRequestParameterError   = "request_parameter_error"
	MsgInternalError           = "internal_error"
)

// zhMessages 是中文原文（也是响应封套里 message 字段的值）。
// 英文侧住在下面的 enMessages；两份必须同键，前端语言包由这两张表生成。
var zhMessages = map[string]string{
	MsgTooManyRequests:             "请求过于频繁，请稍后再试",
	MsgActionTooFrequent:           "操作过于频繁，请稍后再试",
	MsgRefreshTooFrequent:          "刷新过于频繁，请稍后再试",
	MsgInvalidCredentials:          "用户名或密码错误",
	MsgCredentialsEmpty:            "用户名和密码不能为空",
	MsgAuthFailed:                  "认证失败",
	MsgLoginFailed:                 "登录失败",
	MsgSSOAuthCodeInvalid:          "SSO 授权码校验失败",
	MsgSSONameTaken:                "SSO 认证名称已存在",
	MsgSSONotFound:                 "SSO 认证配置不存在",
	MsgSSOPrefixTaken:              "用户名前缀已存在",
	MsgAlreadyInitialized:          "系统已初始化，不能重复执行",
	MsgConcurrentInit:              "系统已被并发初始化，请刷新后重试",
	MsgNotSignedIn:                 "未登录",
	MsgNoFileUploaded:              "没有上传文件",
	MsgInvalidDocID:                "无效的文档ID",
	MsgInvalidAttachmentID:         "无效的附件ID",
	MsgInvalidSourceKind:           "无效的来源类型参数",
	MsgAttachmentNotFound:          "附件不存在",
	MsgDocNotFound:                 "文档不存在",
	MsgNoPermissionDeleteAttach:    "您没有权限删除该附件",
	MsgNoPermissionUploadAttach:    "您没有权限在该空间上传附件",
	MsgNoPermissionReadAttach:      "您没有权限访问该空间的附件",
	MsgWrongSharePassword:          "分享密码错误",
	MsgSpaceDisallowsShare:         "该空间不允许分享文档",
	MsgSpaceDisallowsExport:        "该空间已禁止导出",
	MsgDocModifiedByOther:          "文档已被其他用户修改，请刷新后重试",
	MsgNoPermissionExportDoc:       "无权导出该文档",
	MsgNoPermissionVersionHistory:  "无权查看该文档的版本历史",
	MsgNoPermissionHistoricalDoc:   "无权查看该文档的历史版本",
	MsgNoPermissionCompareVersions: "无权对比该文档的版本差异",
	MsgNoPermissionDocComments:     "无权查看该文档的评论",
	MsgNoPermissionSpace:           "无权访问该空间",
	MsgNoPermissionSpaceContent:    "无权操作该空间的内容",
	MsgNoPermissionSpaceGraph:      "无权查看该空间的知识图谱",
	MsgNoPermissionAnalyzeSpace:    "无权分析该空间",
	MsgNoPermissionDocSpace:        "无权访问该文档所属空间",
	MsgNoPermissionKBSSpace:        "无权访问该 Schema 空间所属的知识库空间",
	MsgSpaceNameTaken:              "空间名已存在",
	MsgSpaceNameIllegalChars:       "空间名称包含非法字符",
	MsgTemplateNameEditDenied:      "无权修改该模板",
	MsgContactNotFound:             "联系人不存在",
	MsgContactFieldsEmpty:          "联系人姓名和邮箱不能为空",
	MsgNoContactChosen:             "没有选择联系人",
	MsgNoLinkChosen:                "没有选择链接",
	MsgLinkNotFound:                "链接不存在",
	MsgLinkIDInvalid:               "链接 ID 无效",
	MsgLinkNameTaken:               "链接名已存在",
	MsgLinkFieldsEmpty:             "链接名称和地址不能为空",
	MsgEmailServerNameTaken:        "邮件服务器名称已存在",
	MsgPluginKeyTaken:              "插件标识已存在",
	MsgOpenAPIUsersModuleDenied:    "开放平台导出不允许 users 模块（用户 PII 仅可通过管理端导出）",
	MsgRestoreNeedsConfirm:         "请确认恢复操作: 需添加 ?confirm=1 参数以确认",
	MsgRequestBodyUnreadable:       "读取请求体失败",
	MsgInvalidParameter:            "参数无效",
	MsgParameterError:              "参数错误",
	MsgIDInvalid:                   "ID 无效",
	MsgIDParamInvalid:              "id 无效",
	MsgHistoryIDInvalid:            "history_id 无效",
	MsgSpaceIDInvalid:              "space_id 无效",
	MsgTextEmpty:                   "text 不能为空",
	MsgInvalidNumericID:            "无效的ID",
	MsgInvalidDataSourceID:         "无效的数据源ID",
	MsgFileTooLarge:                "文件大小不能超过 10MB",
	MsgNoFileChosen:                "请选择要上传的文件",
	MsgURLRequired:                 "请提供有效的 URL 地址",
	MsgActiveFieldRequired:         "请提供 active 字段",
	MsgDocIDsRequired:              "请提供 doc_ids",
	MsgQuestionRequired:            "请提供 question",
	MsgSpaceIDRequired:             "请提供 space_id",
	MsgSpaceIDAndSlugRequired:      "请提供 space_id 与 slug",
	MsgSpaceIDAndSourceDocRequired: "请提供 space_id 与 source_doc_id",
	MsgSpacePageClaimRequired:      "请提供 space_id、page_slug 与 claim",
	MsgTopicRequired:               "请提供 topic",
	MsgConversationNotFound:        "对话不存在",
	MsgNoPermissionConversation:    "无权访问该对话",
	MsgNoPermissionConversationUpd: "无权修改该对话",
	MsgNoPermissionConversationDel: "无权删除该对话",
	MsgWorkflowNotFound:            "工作流不存在",
	MsgWorkflowNameEmpty:           "工作流名称不能为空",
	MsgNoPermissionSQLHistory:      "无权操作他人的 SQL 历史",
	MsgNoPermissionSQLHistoryRead:  "无权查看他人的 SQL 历史",
	MsgNoPermissionKBChunks:        "无权查看该知识库的分块",
	MsgNoPermissionKBRecallTest:    "无权对该知识库做召回测试",
	MsgNoPermissionKBStats:         "无权查看该知识库的统计",
	MsgPageNotFound:                "页面不存在",
	MsgPageUpdated:                 "页面已被更新，请基于最新版本重新生成草稿",
	MsgPageVersionNotFound:         "该页面没有这个历史版本",
	MsgWikiNoCompilableContent:     "未读取到可编译的文档内容",
	MsgHostHeaderUnsafeScript:      "请求的 Host 头不是合法的主机名/端口形式，拒绝生成挂件脚本",
	MsgHostHeaderUnsafePage:        "请求的 Host 头不是合法的主机名/端口形式，拒绝生成挂件页",
	MsgHostHeaderUnsafeEmbed:       "请求的 Host 头不是合法的主机名/端口形式，无法生成嵌入代码",
	MsgAIServiceDisabled:           "AI服务未启用",
	MsgAIServiceUnconfigured:       "AI服务未配置",
	MsgAIRetrieverUnconfigured:     "AI检索器未配置",
	MsgAIPrimaryModelUnconfigured:  "AI主模型未配置",
	MsgAIModelUnconfigured:         "AI模型未配置",
	MsgAIAnalyzerUnconfigured:      "AI分析器未配置",
	MsgAIRecommendUnconfigured:     "AI推荐引擎未配置",
	MsgAIReportWriterUnconfigured:  "AI汇报生成器未配置",
	MsgAITranslatorUnconfigured:    "AI翻译器未配置",
	MsgAITableWriterUnconfigured:   "AI表格生成器未配置",
	MsgAIEmailWriterUnconfigured:   "AI邮件生成器未配置",
	MsgAIQualityUnconfigured:       "AI质量巡检未配置",
	MsgAIAdaptiveUnconfigured:      "自适应检索器未配置",
	MsgAIRAGASUninitialized:        "RAGAS评估器未初始化",
	MsgAICronUnconfigured:          "Cron调度未配置",
	MsgAIEmbeddingUnconfigured:     "Embedding 未配置，无法构建向量索引",
	MsgAIChatFailed:                "AI对话失败",
	MsgAIGenerateFailed:            "AI生成失败",
	MsgAISafetyBlocked:             "查询被安全策略拦截",
	MsgMemoryCreateFailed:          "创建记忆失败",
	MsgMemoryDeleteFailed:          "删除记忆失败",
	MsgMemorySearchFailed:          "搜索记忆失败",
	MsgMemoryRecallFailed:          "召回记忆失败",
	MsgMemoryListFailed:            "获取记忆列表失败",
	MsgMemoryUpdateFailed:          "更新记忆失败",
	MsgMemoryStatsFailed:           "获取统计失败",
	// 批次十六B：这些原句此前是**服务层 errors 的文本被处理器原样回出**（英文界面必读到中文）。
	// 收进来的判据是"它真的会走到用户屏幕"，不是"它是中文" —— 服务层还有一百多句中文只进日志，
	// 那些不该进这张表（表是界面契约）。
	MsgAccountLocked:             "账户已被锁定，请稍后再试",
	MsgHistoryNotFound:           "历史记录不存在",
	MsgHistoryQueryFailed:        "查询历史失败",
	MsgExplanationParseFailed:    "解释解析失败",
	MsgAuditRecordNotFound:       "审计记录不存在",
	MsgRecordNotFound:            "记录不存在",
	MsgDataSourceNotFound:        "数据源不存在",
	MsgDataSourceConnectFailed:   "连接失败",
	MsgConfluenceConvertFailed:   "Confluence HTML 转换失败",
	MsgNotionConvertFailed:       "Notion CSV 转换失败",
	MsgSpaceNameIllegalCharsHint: `空间名称包含非法字符（不能包含 \ / : * ? " < > | ）`,
	MsgAudioMissingField:         "缺少 audio 文件字段",
	MsgAudioOpenFailed:           "打开音频失败",
	MsgAudioReadFailed:           "读取音频失败",
	MsgSignedOutOrTokenInvalid:   "未登录或 token 无效",
	MsgResourceNotFound:          "资源不存在",
	MsgRequestParameterError:     "请求参数错误",
	MsgInternalError:             "服务器内部错误",
	MsgFulltextSearchDisabled:    "全文搜索未开启",
	MsgVectorReindexStarted:      "向量索引重建已启动，请稍后查看 /ai/vector/stats",
}
