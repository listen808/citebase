import { get, post, put, del } from '../../client'
import type { WorkflowEdge, WorkflowNode } from '@/types/ai'

// ============================================================
// Workflow
// ============================================================

export interface WorkflowDefinition {
  id: string
  name: string
  description?: string
  type?: string
  cron_expr?: string
  active?: boolean
  nodes?: WorkflowNode[]
  edges?: WorkflowEdge[]
  created_at?: string
  updated_at?: string
}

export interface WorkflowExecution {
  id: string
  workflow_id: string
  status: string
  started_at?: string
  completed_at?: string
}

export interface CreateWorkflowRequest {
  name: string
  description?: string
  type?: string
  cron_expr?: string
}

export interface UpdateWorkflowRequest {
  name?: string
  description?: string
  nodes?: unknown[]
  edges?: unknown[]
}

export async function listWorkflows(): Promise<{ workflows: WorkflowDefinition[] }> {
  return get('/ai/workflows')
}

export async function createWorkflow(data: CreateWorkflowRequest): Promise<{ workflow: WorkflowDefinition }> {
  return post('/ai/workflows', data)
}

export async function updateWorkflow(
  id: string,
  data: UpdateWorkflowRequest
): Promise<{ workflow: WorkflowDefinition; message?: string }> {
  return put(`/ai/workflows/${id}`, data)
}

export async function executeWorkflow(
  id: string,
  inputs?: Record<string, unknown>
): Promise<{ execution: WorkflowExecution; message?: string }> {
  return post(`/ai/workflows/${id}/execute`, { inputs })
}

export async function toggleWorkflow(
  id: string,
  active: boolean
): Promise<{ workflow: WorkflowDefinition; active: boolean; message?: string }> {
  return post(`/ai/workflows/${id}/toggle`, { active })
}

export async function deleteWorkflow(id: string): Promise<{ deleted: boolean; message?: string }> {
  return del(`/ai/workflows/${id}`)
}

/**
 * W6：自然语言 → 工作流 DAG。
 * 后端返回已注册的草稿（Active=false），需管理员在界面确认后调用 toggleWorkflow 激活。
 * 超时给足：LLM 生成 + 结构校验，通常 10–40s。
 */
export async function generateWorkflow(
  description: string
): Promise<{ workflow: WorkflowDefinition; active: boolean; message?: string }> {
  return post('/ai/workflows/generate', { description }, { timeout: 120000 })
}

// ============================================================
// Schema Space
// ============================================================

export interface SchemaSpace {
  id: number
  space_id: number
  name: string
  db_type: string
  status: number
  last_scan_at: string | null
  doc_count: number
  table_count: number
  created_at: string
  updated_at: string
}

// schema-spaces 这一组能力由 `@/api/modules/schema-sql` 的 schemaApi 提供并在
// AISchemaView 实际使用；此处曾有 8 个同名不同形的包装（listSchemaSpaces /
// createSchemaSpace / deleteSchemaSpace / getSchemaStats / startSchemaScan /
// listSchemaTables / getSchemaTableDetail / askSchema），从未被调用，已删——
// 留着只会让人在两套实现之间选错。
export interface ConversationMessage {
  role: string
  content: string
  sql?: string
}

// ============================================================
// SQL / Data Query
// ============================================================

export interface SQLExecResult {
  success: boolean
  columns: string[]
  rows: Record<string, unknown>[]
  error?: string
  row_count: number
  duration_ms: number
}

export interface SQLGenerationResponse {
  sql: string
  explanations: string[]
  tables: string[]
  warnings: string[]
  explanation?: string
  trace_id: string
  history_id?: number
  generator_tier: string
}

export interface SQLSecurityPolicy {
  id?: number
  space_id: number
  max_rows: number
  timeout_sec: number
  max_cost_tokens: number
  allow_patterns: string
  deny_patterns: string
  allow_dml: boolean
  allow_ddl: boolean
}

export function executeSQL(sql: string, space_id: number, trace_id?: string) {
  return post<SQLExecResult>('/ai/data-query/execute-sql', { sql, space_id, trace_id }, { timeout: 30000 })
}

export function generateSQL(space_id: number, question: string, history?: ConversationMessage[]) {
  return post<SQLGenerationResponse>(
    '/ai/data-query/generate-sql',
    { space_id, question, history: history || [] },
    { timeout: 300000 }
  )
}

// P1-3: 后端 Get/UpdateSQLSecurityPolicy 用 response.Success(c, policy) 直接返回策略对象
// （client 拦截器已拆掉 {code,data} 信封），此前多包一层 { data } 导致消费方恒为 undefined，
// 页面用默认值渲染并在保存时静默覆盖线上安全策略。
export function getSQLSecurityPolicy(space_id: number) {
  return get<SQLSecurityPolicy>(`/ai/sql-security-policy?space_id=${space_id}`)
}

export function updateSQLSecurityPolicy(data: SQLSecurityPolicy) {
  return put<SQLSecurityPolicy>('/ai/sql-security-policy', data)
}

export interface GenerateSQLRequest {
  question: string
  space_id?: number
  history?: { role: string; content: string; sql: string }[]
}

export interface AgentStepInfo {
  step: number
  thought: string
  action: string
  observation: string
  duration_ms: number
}

export interface SQLResult {
  sql: string
  tables: string[]
  entities: RecognizedEntity[]
  relations: TableRelation[]
  explanations: string[]
  warnings?: string[]
  explanation?: string
  clarify?: {
    needed: boolean
    options: string[]
  }
  /** CoT 思考步骤（后端 buildCoTSteps 构建） */
  steps?: AgentStepInfo[]
  /** LLM 原始 CoT 输出（含思考流程+字段映射+SQL） */
  raw_cot?: string
  /** selfReflect 修正前的 SQL */
  self_reflect_before?: string
  /** selfReflect 修正后的 SQL */
  self_reflect_after?: string
  /** 字段映射列表 */
  field_mappings?: string[]
}

export interface RecognizedEntity {
  type: string
  value: string
  field?: string
}

export interface TableRelation {
  source_table: string
  source_field: string
  target_table: string
  target_field: string
}

export interface BusinessRule {
  id: number
  name: string
  type: 'enum' | 'relation' | 'formula' | 'condition'
  table_name: string
  field_name?: string
  content: string
  description?: string
  status: number
  created_at: string
  updated_at: string
}

export interface TableCache {
  id: number
  table_name: string
  table_comment?: string
  ddl_content: string
  columns_json: ColumnInfo[]
  space_id?: number
  doc_id?: number
  created_at: string
  updated_at: string
}

export interface ColumnInfo {
  name: string
  type: string
  comment?: string
  is_pk: boolean
}

export interface CreateRuleRequest {
  name: string
  type: 'enum' | 'relation' | 'formula' | 'condition'
  table_name: string
  field_name?: string
  content: string
  description?: string
}

export interface CreateRelationRequest {
  source_table: string
  source_field: string
  target_table: string
  target_field: string
  confidence?: number
}

export interface ScanResult {
  total_docs: number
  scanned_docs: number
  new_tables: number
  updated_tables: number
  deleted_tables: number
  tables: string[]
  errors?: string[]
}

export interface SQLCandidate {
  sql: string
  explanation: string
  exec_success: boolean
  exec_error?: string
  struct_issues: number
  confidence: number
}

export interface SQLEvalResult {
  question: string
  generated_sql: string
  execution_ok: boolean
  exec_error?: string
  struct_valid: boolean
  struct_issues: number
  tables_correct: boolean
  execution_time_ms: number
  score: number
}

export interface PromptTemplateItem {
  id: number | string
  name: string
  query_type: string
  system_prompt: string
  user_template: string
  description: string
  is_active: boolean
  is_default: boolean
  is_built_in?: boolean
  sort: number
  space_id: number | null
  created_at: string
  updated_at: string
}

export interface CreatePromptTemplateRequest {
  name: string
  query_type: string
  system_prompt: string
  user_template: string
  description: string
  is_active: boolean
  is_default: boolean
  sort: number
  space_id: number | null
}

export const textToSQLApi = {
  generateSQL(data: GenerateSQLRequest): Promise<SQLResult> {
    return post<SQLResult>('/ai/data-query/generate-sql', data, { timeout: 300000 })
  },

  generateSQLWithRAG(data: GenerateSQLRequest): Promise<SQLResult> {
    return post<SQLResult>('/ai/data-query/generate-sql-rag', data)
  },

  generateMultiCandidateSQL(
    data: GenerateSQLRequest & { num_candidates?: number }
  ): Promise<{ best: SQLResult; candidates: SQLCandidate[]; total: number }> {
    return post('/ai/data-query/generate-sql-multi', data, { timeout: 300000 })
  },

  evaluateSQL(data: { question: string; sql: string; space_id?: number }): Promise<SQLEvalResult> {
    return post('/ai/data-query/evaluate-sql', data)
  },

  executeSQL(data: {
    sql: string
    datasource_id?: number
    space_id?: number
    max_rows?: number
  }): Promise<{
    columns: Array<{ name: string; data_type: string }>
    rows: Record<string, unknown>[]
    row_count: number
    execution_time_ms: number
  }> {
    return post('/ai/data-query/execute-sql', data)
  },

  getRules(params?: { page?: number; page_size?: number; type?: string; table_name?: string }) {
    return get<{ data: BusinessRule[]; total: number }>('/ai/data-query/rules', { params })
  },

  getRule(id: number): Promise<BusinessRule> {
    return get<BusinessRule>(`/ai/data-query/rules/${id}`)
  },

  createRule(data: CreateRuleRequest): Promise<BusinessRule> {
    return post<BusinessRule>('/ai/data-query/rules', data)
  },

  updateRule(id: number, data: CreateRuleRequest): Promise<BusinessRule> {
    return put<BusinessRule>(`/ai/data-query/rules/${id}`, data)
  },

  deleteRule(id: number): Promise<null> {
    return del<null>(`/ai/data-query/rules/${id}`)
  },

  getTables(params?: { page?: number; page_size?: number; keyword?: string }) {
    return get<{ data: TableCache[]; total: number }>('/ai/data-query/tables', { params })
  },

  getTableDetail(name: string): Promise<TableCache> {
    return get<TableCache>(`/ai/data-query/tables/${name}`)
  },

  getTableRelations(name: string): Promise<TableRelation[]> {
    return get<TableRelation[]>(`/ai/data-query/tables/${name}/relations`)
  },

  createRelation(data: CreateRelationRequest): Promise<TableRelation> {
    return post<TableRelation>('/ai/data-query/relations', data)
  },

  deleteRelation(id: number): Promise<null> {
    return del<null>(`/ai/data-query/relations/${id}`)
  },

  refreshKnowledge(): Promise<{ message: string }> {
    return post<{ message: string }>('/ai/data-query/refresh')
  },

  getSpaceTables(spaceId: number): Promise<TableCache[]> {
    return get<TableCache[]>(`/ai/data-query/spaces/${spaceId}/tables`)
  },

  scanSpace(spaceId: number): Promise<ScanResult> {
    return post<ScanResult>(`/ai/data-query/spaces/${spaceId}/scan`)
  },

  scanSpaceWithRAG(spaceId: number): Promise<ScanResult> {
    return post<ScanResult>(`/ai/data-query/spaces/${spaceId}/scan-rag`)
  },

  listPromptTemplates(params?: { space_id?: number; include_global?: boolean }) {
    return get<PromptTemplateItem[]>('/ai/data-query/prompts', { params })
  },

  listBuiltInPromptTemplates() {
    return get<PromptTemplateItem[]>('/ai/data-query/prompts/builtin')
  },

  createPromptTemplate(data: CreatePromptTemplateRequest): Promise<PromptTemplateItem> {
    return post<PromptTemplateItem>('/ai/data-query/prompts', data)
  },

  getPromptTemplateById(id: number | string): Promise<PromptTemplateItem> {
    return get<PromptTemplateItem>(`/ai/data-query/prompts/${id}`)
  },

  updatePromptTemplate(id: number | string, data: CreatePromptTemplateRequest): Promise<PromptTemplateItem> {
    return put<PromptTemplateItem>(`/ai/data-query/prompts/${id}`, data)
  },

  deletePromptTemplate(id: number | string): Promise<null> {
    return del<null>(`/ai/data-query/prompts/${id}`)
  },

  setDefaultPromptTemplate(id: number | string, space_id?: number): Promise<{ message: string }> {
    return put<{ message: string }>(`/ai/data-query/prompts/${id}/default`, { space_id: space_id || 0 })
  },

  togglePromptTemplate(id: number | string, is_active: boolean): Promise<{ message: string }> {
    return put<{ message: string }>(`/ai/data-query/prompts/${id}/toggle`, { is_active })
  }
}
