import { get, post } from '../../client';
import type { SearchResult } from '@/types/models';

export interface SearchRequest {
  query: string;
  space_id?: number;
  limit?: number;
  offset?: number;
  type?: 'all' | 'title' | 'content';
  author_id?: number;
  tag_name?: string;
  time_from?: number;
  time_to?: number;
  sort_by?: 'relevance' | 'time_desc' | 'time_asc';
}

export function fulltextSearch(params: SearchRequest, config?: { signal?: AbortSignal }) {
  return get<{ results: SearchResult[]; total: number }>('/search/fulltext', { params, ...config });
}

// === Hybrid (Semantic) Search ===

export type HybridSearchMode = 'hybrid' | 'fulltext' | 'semantic';

export interface HybridSearchRequest {
  query: string;
  space_ids?: number[];
  category_ids?: number[];
  tags?: string[];
  date_from?: string;
  date_to?: string;
  search_mode?: HybridSearchMode;
  page?: number;
  page_size?: number;
}

export interface HybridScoreDetail {
  fulltext_score: number;
  vector_score: number;
  fusion_method: string;
}

export interface HybridSearchResult {
  document_id: number;
  title: string;
  snippet: string;
  score: number;
  score_detail: HybridScoreDetail;
  highlights: string[];
  author: string;
  space_name: string;
  category_name: string;
  updated_at: string;
  space_id: number;
  type: number;
  path: string;
}

export function hybridSearch(data: HybridSearchRequest, config?: { signal?: AbortSignal }) {
  return post<{ results: HybridSearchResult[]; total: number }>('/search/hybrid', data, config);
}

export function getSearchTags(keyword?: string, limit?: number) {
  return get<{ tags: string[] }>('/search/tags', { params: { keyword, limit } });
}

export function searchSuggest(query: string) {
  return get<string[]>('/search/suggest', { params: { query } });
}

export interface SearchTrendPoint {
  date: string;
  count: number;
}

export interface HotSearchWord {
  term: string;
  count: number;
}

export interface SearchMonitorStatistics {
  range_days: number;
  total_searches: number;
  today_searches: number;
  no_result_rate: number;
  avg_latency_ms: number;
  unique_users: number;
  trend: SearchTrendPoint[];
  hot_words: HotSearchWord[];
}

export interface SearchMonitorData {
  index: {
    is_open: boolean;
    doc_count: number;
    index_size: number;
    index_path: string;
    last_updated: number;
    indexing: boolean;
  };
  statistics: SearchMonitorStatistics;
}

/**
 * 索引监控页数据。与公开的 /search/stats 分开：管理端才拿得到索引目录绝对路径
 * 与 aw_search_log 的聚合（查询原文和 user_id 属于行为数据）。
 */
export function searchMonitor(days = 7, hotLimit = 10) {
  return get<SearchMonitorData>('/search/monitor', { params: { days, hot_limit: hotLimit } });
}

export function searchDebug(documentId: number) {
  return get<{ index_stats: unknown; document: unknown; debug_tip: string }>(`/search/debug/${documentId}`);
}

export function reindexAll() {
  // 重建索引可能需要较长时间，设置5分钟超时
  return post<{ message: string }>('/search/reindex', undefined, { timeout: 300000 });
}

export interface ReindexStatus {
  is_running: boolean;
  progress: number;
  total: number;
  current: number;
  message: string;
  start_time: number;
}

export function getReindexStatus() {
  return get<ReindexStatus>('/search/reindex/status');
}

/**
 * 后端 config.SynonymGroup 只有 mapstructure:"words"、没有 json 标签，
 * 因此 /search/synonyms 里 synonyms 数组的键实际是 Go 字段名 `Words`。
 * `words` 分支保留给后端补上 json 标签之后的兼容读取。
 */
export interface SynonymGroupPayload {
  Words?: string[];
  words?: string[];
}

export function getSynonyms() {
  return get<{
    synonyms: SynonymGroupPayload[];
    synonym_map: Record<string, string[]>;
  }>('/search/synonyms');
}

// === 词库与索引运维 ===

/** 同义词扩展：后端把 configs 里的同义词组逐个替换到 query 上，原句永远在首位 */
export function expandQueryWithSynonyms(query: string) {
  return get<{ original: string; expanded: string[] }>('/search/expand', { params: { query } });
}

/** 索引初始化诊断：索引未打开时强制初始化并回报状态 */
export interface IndexInitStats {
  is_open?: boolean;
  doc_count?: number;
  actual_doc_count?: number;
  error?: string;
  [key: string]: unknown;
}

export interface IndexInitDiagnostic {
  index_path: string;
  parent_dir_exists: boolean;
  index_dir_exists: boolean;
  /** already_initialized | init_succeeded | init_failed: <err> | attempting_init */
  init_result: string;
  index_stats: IndexInitStats;
}

export function initIndexDiagnostic() {
  return get<IndexInitDiagnostic>('/search/init');
}

/** 单文档重建索引（新增或更新该文档的全文索引条目） */
export function indexSingleDocument(id: number) {
  return post<{ id: number }>(`/search/index/${id}`);
}
