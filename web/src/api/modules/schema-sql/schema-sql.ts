import { get, post, del } from '@/api/client'

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

export interface SchemaTable {
  id: number
  schema_space_id: number
  table_name: string
  table_comment: string
  engine: string
  charset: string
  primary_key: string
  doc_id: number
  doc_path: string
  raw_ddl: string
  status: number
  created_at: string
  updated_at: string
}

export interface SchemaColumn {
  id: number
  schema_space_id: number
  table_id: number
  table_name: string
  column_name: string
  data_type: string
  length: number
  nullable: number
  default_value: string
  column_comment: string
  is_primary_key: number
  is_foreign_key: number
  is_index: number
  ordinal_position: number
  created_at: string
}

export interface SchemaRelation {
  id: number
  schema_space_id: number
  source_table: string
  source_column: string
  target_table: string
  target_column: string
  relation_type: string
  confidence: number
  source: string
  created_at: string
}

export interface SchemaScanTask {
  id: number
  schema_space_id: number
  status: string
  total_docs: number
  processed_docs: number
  found_tables: number
  error_message: string
  started_at: string | null
  completed_at: string | null
  created_at: string
}

export interface SQLResult {
  sql: string
  explanation: string
  sources: Array<{
    table_name: string
    confidence: number
  }>
  confidence: number
  warnings: string[]
}

export interface TableCache {
  id: number
  table_name: string
  table_comment: string
  ddl_content: string
  columns_json: Array<{
    name: string
    type: string
    comment: string
    is_pk: boolean
    is_nullable: boolean
    default_value: string
    is_auto_inc: boolean
    charset: string
    collation: string
  }>
  constraints_json: Array<{
    table_name: string
    constraint_type: string
    constraint_name: string
    columns: string[]
    ref_table: string
    ref_columns: string[]
    index_type: string
    comment: string
  }>
  space_id: number | null
  schema_space_id: number | null
  doc_id: number | null
  db_type: string
  row_count_estimate: number
  version: number
  created_at: string
  updated_at: string
}

export interface BusinessRule {
  id: number
  schema_space_id: number
  name: string
  type: string
  table_name: string
  field_name: string
  content: string
  description: string
  status: number
  created_at: string
  updated_at: string
}

export interface PaginationResponse<T> {
  data: T[]
  total: number
  page: number
  page_size: number
}

export const schemaApi = {
  // Schema Space
  createSpace(data: { space_id: number; name: string; db_type?: string }) {
    return post<SchemaSpace>('/schema-spaces', data)
  },

  getSpace(id: number) {
    return get<SchemaSpace>(`/schema-spaces/${id}`)
  },

  listSpaces(page = 1, pageSize = 20) {
    return get<PaginationResponse<SchemaSpace>>('/schema-spaces', {
      params: { page, page_size: pageSize }
    })
  },

  deleteSpace(id: number) {
    return del(`/schema-spaces/${id}`)
  },

  getStats(id: number) {
    return get<{
      table_count: number
      column_count: number
      relation_count: number
      scan_task_count: number
    }>(`/schema-spaces/${id}/stats`)
  },

  // Scan
  startScan(id: number) {
    return post<SchemaScanTask>(`/schema-spaces/${id}/scan`)
  },

  getScanTask(taskId: number) {
    return get<SchemaScanTask>(`/schema-scan-tasks/${taskId}`)
  },

  listScanTasks(id: number) {
    return get<SchemaScanTask[]>(`/schema-spaces/${id}/scan-tasks`)
  },

  deleteScanTask(taskId: number) {
    return del(`/schema-scan-tasks/${taskId}`)
  },

  deleteAllScanTasks(id: number) {
    return del(`/schema-spaces/${id}/scan-tasks`)
  },

  pauseScanTask(taskId: number) {
    return post(`/schema-scan-tasks/${taskId}/pause`)
  },

  resumeScanTask(taskId: number) {
    return post(`/schema-scan-tasks/${taskId}/resume`)
  },

  cancelScanTask(taskId: number) {
    return post(`/schema-scan-tasks/${taskId}/cancel`)
  },

  // Tables
  listTables(id: number, keyword?: string, page = 1, pageSize = 20) {
    return get<PaginationResponse<SchemaTable>>(`/schema-spaces/${id}/tables`, {
      params: { keyword, page, page_size: pageSize }
    })
  },

  getTableDetail(id: number, tableName: string) {
    return get<{
      table: SchemaTable
      columns: SchemaColumn[]
      relations: SchemaRelation[]
    }>(`/schema-spaces/${id}/tables/${tableName}`)
  },

  // Ask (LLM generation may take >30s, use 5min timeout)
  askQuestion(id: number, question: string) {
    return post<SQLResult>(`/schema-spaces/${id}/ask`, { question }, { timeout: 300000 })
  },

  // TableCache
  listTableCache(id: number, keyword?: string, page = 1, pageSize = 100) {
    return get<PaginationResponse<TableCache>>(`/schema-spaces/${id}/table-cache`, {
      params: { keyword, page, page_size: pageSize }
    })
  },

  // RefreshKnowledge
  refreshKnowledge(id: number) {
    return post(`/schema-spaces/${id}/refresh-knowledge`)
  },

  // BusinessRules
  listBusinessRules(id: number, page = 1, pageSize = 20) {
    return get<PaginationResponse<BusinessRule>>(`/schema-spaces/${id}/rules`, {
      params: { page, page_size: pageSize }
    })
  },

  createBusinessRule(id: number, data: { name: string; type: string; table_name: string; field_name?: string; content: string; description?: string }) {
    return post<BusinessRule>(`/schema-spaces/${id}/rules`, data)
  },

  deleteBusinessRule(id: number, ruleId: number) {
    return del(`/schema-spaces/${id}/rules/${ruleId}`)
  },

  // Export CSV
  exportCSV(columns: string[], rows: Record<string, unknown>[]) {
    return post<Blob>('/schema-export-csv', { columns, rows }, { responseType: 'blob' })
  }
}