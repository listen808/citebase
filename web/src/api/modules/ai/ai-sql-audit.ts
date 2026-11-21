import { get } from '../../client'

/**
 * SQL 审计追踪 + Text2SQL 步骤追踪
 * 路由：internal/api/ai/routes.go:244-245（handler internal/api/ai/sql_trust.go）
 *      internal/api/ai/routes.go:211-212（handler internal/api/ai/sql_trace.go）
 * 字段对齐 internal/model/ai/audit_log.go AIAuditLog、internal/model/ai/sql_trace.go SQLTrace
 * 说明：审计列表只返回 action = text2sql 的记录，分页用 limit/offset（limit 上限 100，非法回落 20）
 */

/** aw_ai_audit_log 一行 = 一次 Text2SQL 调用 */
export interface SQLAuditTrace {
  id: number
  user_id: number
  /** 固定为 text2sql */
  action: string
  /** 关联对象 ID（字符串，通常为 sql history id） */
  target_id: string
  question: string
  answer: string
  detail: string
  token_usage: number
  source_count: number
  model: string
  provider: string
  /** success / failure */
  result: string
  reason: string
  ip_address: string
  user_agent: string
  duration_ms: number
  created_at: string
}

/** {list, total}（sql_trust.go ListSQLAuditTraces 用 gin.H 返回，非 PaginatedData） */
export interface SQLAuditTracePage {
  list: SQLAuditTrace[]
  total: number
}

export const SQLAuditTraceAPI = {
  list(params: { limit?: number; offset?: number } = {}) {
    return get<SQLAuditTracePage>('/ai/sql-audit-traces', { params })
  },

  detail(id: number) {
    return get<SQLAuditTrace>(`/ai/sql-audit-traces/${id}`)
  },
}

/** 单个生成步骤（internal/model/ai/sql_trace.go SQLTraceStep） */
export interface SQLTraceStep {
  name: string
  input: string
  output: string
  duration_ms: number
  status: string
  error_msg?: string
}

/**
 * Text2SQL 生成链路追踪。
 * 注意：模型只有上述字段，没有 token_usage / total_time_ms / generator_tier，
 * 旧 ai-trace.ts 的接口定义与后端不一致，已合并进本模块并纠正。
 */
export interface SQLTrace {
  id: number
  trace_id: string
  space_id: number
  conversation_id: number
  user_query: string
  intent: string
  steps: SQLTraceStep[]
  final_sql: string
  execution_ok: boolean
  duration_ms: number
  created_at: string
}

/** commonmodel.PageResult（internal/model/common/page.go）解包后的分页体 */
export interface SQLTraceListPage {
  list: SQLTrace[]
  total: number
  page_num: number
  page_size: number
}

export const SQLTraceAPI = {
  list(params: { space_id?: number; page?: number; page_size?: number; since?: string } = {}) {
    return get<SQLTraceListPage>('/ai/data-query/traces', { params })
  },

  /** trace_id 为字符串唯一键，非主键；since 需 RFC3339 */
  detail(traceId: string) {
    return get<SQLTrace>(`/ai/data-query/traces/${encodeURIComponent(traceId)}`)
  },
}
