import { get, post, put, del } from '../../client';

/**
 * 保存的搜索 API
 * 归属 document 模块（与 search.ts 同域：保存的是检索条件）
 * 对应后端 internal/api/workspace/saved_search.go（路由前缀 /api/v1 已在 client baseURL 中）
 */

export type SavedSearchType = 'all' | 'title' | 'content';
export type SavedSearchSort = 'relevance' | 'time_desc' | 'time_asc';

export interface SavedSearch {
  id: number;
  user_id: number;
  name: string;
  query: string;
  type: SavedSearchType;
  /** 0 表示不限空间 */
  space_id: number;
  tag_filter: string;
  sort_by: SavedSearchSort;
  /** 1=公开（进入公共热门列表），0=私有 */
  is_public: 0 | 1;
  use_count: number;
  create_time: number;
  update_time: number;
}

export interface CreateSavedSearchRequest {
  /** 必填，最长 100 */
  name: string;
  /** 必填，最长 500 */
  query: string;
  type?: SavedSearchType;
  space_id?: number;
  /** 最长 100 */
  tag_filter?: string;
  sort_by?: SavedSearchSort;
  is_public?: 0 | 1;
}

/**
 * 更新为增量语义：空字符串 / 0 的字段后端会跳过（无法清空 space_id 与 tag_filter），
 * 但 is_public 每次都会被覆盖，因此必须始终传值。
 */
export interface UpdateSavedSearchRequest {
  name?: string;
  query?: string;
  type?: SavedSearchType;
  space_id?: number;
  tag_filter?: string;
  sort_by?: SavedSearchSort;
  is_public: 0 | 1;
}

/** 创建保存的搜索 */
export function createSavedSearch(data: CreateSavedSearchRequest) {
  return post<SavedSearch>('/saved-searches', data);
}

/** 我的保存搜索列表（后端直接返回数组） */
export function getMySavedSearches() {
  return get<SavedSearch[]>('/saved-searches');
}

/** 公开热门列表，limit 后端限制 1-50，默认 10 */
export function getPublicSavedSearches(limit = 10) {
  return get<SavedSearch[]>('/saved-searches/public', { params: { limit } });
}

/** 更新（仅本人） */
export function updateSavedSearch(id: number, data: UpdateSavedSearchRequest) {
  return put<SavedSearch>(`/saved-searches/${id}`, data);
}

/** 记录一次「使用」并返回自增后的对象 */
export function useSavedSearch(id: number) {
  return post<SavedSearch>(`/saved-searches/${id}/use`);
}

/** 删除（仅本人） */
export function deleteSavedSearch(id: number) {
  return del<void>(`/saved-searches/${id}`);
}
