import { get, post, put, del } from '../../client'

/**
 * Text2SQL 少样本示例（SQL Examples / few-shot）
 * 路由：internal/api/ai/routes.go:203-209（handler internal/api/ai/facade.go AIFacadeAPI）
 * 字段对齐 internal/model/ai/sql_example.go SQLExample
 * 权限：list/get 任意登录用户；create/update/verify/delete 需管理员（AutoRBAC）
 */
export interface SQLExample {
  id: number
  space_id: number
  /** 自然语言问题 */
  question: string
  /** 期望生成的 SQL */
  sql: string
  /** mysql 等方言，默认 mysql */
  db_type: string
  /** 0 = 未验证，1 = 已验证；只有 1 会参与 verified=1 过滤 */
  verified: number
  created_by: number
  /** 后端以字符串返回 */
  created_at: string
}

/** POST /ai/data-query/examples 请求体（question、sql 必填） */
export interface CreateSQLExampleRequest {
  question: string
  sql: string
  space_id?: number
  verified?: number
}

/** PUT /ai/data-query/examples/:id 请求体：空字符串字段会被后端忽略，verified 仅接受 0/1 */
export interface UpdateSQLExampleRequest {
  question?: string
  sql?: string
  verified?: number
}

/** response.Paginated 解包后的分页体 */
export interface SQLExamplePage {
  data: SQLExample[]
  total: number
  page: number
  page_size: number
}

export const SQLExampleAPI = {
  /** verified=1 只看已验证示例；space_id 是精确匹配（0 表示全局示例） */
  list(params: { space_id?: number; verified?: number; page?: number; page_size?: number } = {}) {
    return get<SQLExamplePage>('/ai/data-query/examples', { params })
  },

  detail(id: number) {
    return get<SQLExample>(`/ai/data-query/examples/${id}`)
  },

  create(data: CreateSQLExampleRequest) {
    return post<{ message: string }>('/ai/data-query/examples', data)
  },

  update(id: number, data: UpdateSQLExampleRequest) {
    return put<SQLExample>(`/ai/data-query/examples/${id}`, data)
  },

  /** 标记示例为已验证（后端直接置 verified=1，无请求体） */
  verify(id: number) {
    return put<{ message: string }>(`/ai/data-query/examples/${id}/verify`)
  },

  remove(id: number) {
    return del<{ message: string }>(`/ai/data-query/examples/${id}`)
  },
}
