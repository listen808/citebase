import { get, post, put, del } from '../../client'

/**
 * SQL 执行历史
 * 路由注册：internal/api/schema/routes.go:55-61（handler：internal/api/ai/texttosql/texttosql.go SQLHistoryAPI）
 * 字段对齐 internal/model/ai/datasource.go SQLHistory
 * 归属：仅当前登录用户自己的历史可见（Service 层按 user_id 强制过滤，越权返回 403）
 */
export interface SQLHistory {
  id: number
  created_at: string
  updated_at: string
  space_id: number
  user_id: number
  /** 自然语言问题 */
  query_text: string
  generated_sql: string
  executed_sql: string
  execution_result: string
  execution_error: string
  is_favorite: boolean
  /** 0 = 未评分，1-5 */
  rating: number
  feedback: string
  /** 逗号分隔的表名 */
  tables_used: string
  execution_time_ms: number
  datasource_id: number
  duration_ms: number
  /** success / error 等 */
  status: string
  explanation_json: string
  /** 关联 aw_ai_sql_trace.trace_id，可跳转 Text2SQL 步骤追踪 */
  trace_id: string
  generator_tier: string
}

/** POST /sql-history/save 请求体（internal/service/ai/sql_history_service.go SaveSQLHistoryRequest） */
export interface SaveSQLHistoryRequest {
  space_id: number
  query_text: string
  generated_sql?: string
  executed_sql?: string
  tables_used?: string
  duration_ms?: number
  status?: string
  feedback?: string
}

/** response.Paginated 解包后的分页体（internal/response/response.go PaginatedData） */
export interface SQLHistoryPage {
  data: SQLHistory[]
  total: number
  page: number
  page_size: number
}

export interface SQLHistoryListParams {
  /** 0 / 不传 = 全部空间 */
  space_id?: number
  page?: number
  page_size?: number
}

export const SQLHistoryAPI = {
  list(params: SQLHistoryListParams = {}) {
    return get<SQLHistoryPage>('/sql-history', { params })
  },

  listFavorites(params: SQLHistoryListParams = {}) {
    return get<SQLHistoryPage>('/sql-history/favorites', { params })
  },

  detail(id: number) {
    return get<SQLHistory>(`/sql-history/${id}`)
  },

  save(data: SaveSQLHistoryRequest) {
    return post<SQLHistory>('/sql-history/save', data)
  },

  /** 切换收藏（后端为 toggle，不回传新状态，调用方需重新拉取详情） */
  toggleFavorite(id: number) {
    return put<{ message: string }>(`/sql-history/${id}/favorite`)
  },

  /** rating 取值 1-5，越界后端 400 */
  rate(id: number, rating: number) {
    return put<{ message: string }>(`/sql-history/${id}/rate`, { rating })
  },

  remove(id: number) {
    return del<null>(`/sql-history/${id}`)
  },
}
