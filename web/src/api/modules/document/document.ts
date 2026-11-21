import { get, post, put, del } from '../../client';
import type { Document, DocumentVersion } from '@/types/models';

export interface CreateDocumentRequest {
  parent_id: number;
  space_id: number;
  name: string;
  type: number; // 1 page, 2 dir
  sequence?: number;
  content?: string;
}

export interface UpdateDocumentRequest {
  name?: string;
  content?: string;
  sequence?: number;
  icon?: string;
  cover_image?: string;
  change_log?: string;
  last_update_time?: number;
  // is_follow_doc 已于 P2-5 从后端请求中移除（避免跨模块副作用），
  // 如需关注文档请直接调用 POST /api/v1/follow/document
}

export interface MoveDocumentRequest {
  parent_id: number;
  space_id: number;
}

export interface SortDocumentItem {
  id: number;
  sequence: number;
}

export function getDocumentTree(spaceId?: number) {
  const params = spaceId ? { params: { space_id: spaceId } } : undefined;
  return get<Document[]>('/documents/tree', params);
}

export function getRootDocuments(spaceId: number) {
  return get<Document[]>('/documents/roots', { params: { space_id: spaceId } });
}

export function getDocumentChildren(parentId: number) {
  return get<Document[]>(`/documents/${parentId}/children`);
}

export function getDocumentByID(id: number, config?: import('axios').AxiosRequestConfig) {
  return get<Document>(`/documents/${id}`, config);
}

export function getDocumentContent(id: number, config?: import('axios').AxiosRequestConfig) {
  return get<{ document_id: number; content: string }>(`/documents/${id}/content`, config);
}

// 合并接口：一次性返回文档元数据+内容+路径，减少 3 次 HTTP 往返为 1 次
export function getDocumentFull(id: number, config?: import('axios').AxiosRequestConfig) {
  return get<{
    document: Document;
    content: string;
    path: { id: number; name: string }[];
  }>(`/documents/${id}/full`, config);
}

export function createDocument(data: CreateDocumentRequest) {
  return post<Document>('/documents', data);
}

export function updateDocument(id: number, data: UpdateDocumentRequest) {
  return put<Document>(`/documents/${id}`, data);
}

export function deleteDocument(id: number) {
  return del<void>(`/documents/${id}`);
}

export function moveDocument(id: number, data: MoveDocumentRequest) {
  return put<void>(`/documents/${id}/move`, data);
}

export function copyDocument(id: number, data: MoveDocumentRequest) {
  return post<void>(`/documents/${id}/copy`, data);
}

export function sortDocuments(orders: SortDocumentItem[]) {
  return put<void>('/documents/sort', { orders });
}

export function getDocumentVersions(id: number, page = 1, pageSize = 10) {
  return get<{ data: DocumentVersion[]; total: number; page: number; page_size: number }>(`/documents/${id}/versions`, { params: { page, page_size: pageSize } });
}

export function getVersionContent(id: number, version: number) {
  return get<{ document_id: number; version: number; content: string }>(`/documents/${id}/versions/${version}`);
}

export interface DiffBlock {
  type: 'context' | 'added' | 'removed';
  lines: string[];
}

export interface DiffResult {
  document_id: number;
  from_version: number;
  to_version: number;
  has_changes: boolean;
  diff_blocks: DiffBlock[];
  added_count: number;
  removed_count: number;
}

export function diffDocumentVersions(id: number, fromVersion: number, toVersion: number) {
  return get<DiffResult>(`/documents/${id}/diff`, { params: { from: fromVersion, to: toVersion } });
}

export interface DailyViewCount {
  date: string;
  count: number;
}

export interface DocumentViewStats {
  document_id: number;
  total_views: number;
  unique_visitors: number;
  today_views: number;
  week_views: number;
  daily_views: DailyViewCount[];
}

export function getDocumentStats(id: number) {
  return get<DocumentViewStats>(`/documents/${id}/stats`);
}

export function rollbackDocument(id: number, version: number) {
  return post<Document>(`/documents/${id}/rollback`, { version });
}

export function exportDocument(id: number) {
  return get<Blob>(`/documents/${id}/export`, { responseType: 'blob' });
}

export function exportDocumentHTML(id: number) {
  return get<Blob>(`/documents/${id}/export-html`, { responseType: 'blob' });
}

export function exportDocumentWord(id: number) {
  return get<Blob>(`/documents/${id}/export-word`, { responseType: 'blob' });
}

export function exportDocumentPDF(id: number) {
  return get<Blob>(`/documents/${id}/export-pdf`, { responseType: 'blob' });
}

export function exportDocumentMarkdown(id: number) {
  return get<Blob>(`/documents/${id}/export-md`, { responseType: 'blob' });
}

export function getBacklinks(id: number) {
  return get<import('@/types/models').BacklinkResult[]>(`/documents/${id}/backlinks`);
}

export function getForwardLinks(id: number) {
  return get<import('@/types/models').ForwardLinkResult[]>(`/documents/${id}/forward-links`);
}

export function getDocumentPath(id: number) {
  return get<Document[]>(`/documents/${id}/path`);
}

/**
 * 文档名称快搜 - 基于 /search/fulltext (Bleve, type=title)
 * 历史：早期后端提供 /documents/search (DB LIKE)，已于 P2-1 下线。
 * 本函数保留入口名以兼容现有调用点（MonacoEditor / useSpaceTree），
 * 内部调用 fulltextSearch 并将 SearchResult 映射为 Document。
 * 目录树内搜索语义对齐主流方案（语雀/Notion/VS Code）：仅按名称过滤导航，
 * 全文内容搜索请使用 search.ts 中的 fulltextSearch()（全局搜索页）。
 */
export async function searchDocuments(keyword: string, spaceId: number): Promise<Document[]> {
  const { fulltextSearch } = await import('./search');
  const trimmed = (keyword || '').trim();
  const res = await fulltextSearch({
    query: trimmed || ' ',
    space_id: spaceId,
    type: 'title',
    limit: 500,
  });
  const items = res?.results ?? [];
  return items.map((r) => ({
    id: r.document_id ?? 0,
    parent_id: r.parent_id ?? 0,
    space_id: r.space_id,
    name: r.name,
    type: r.type,
    path: r.path ?? '',
    path_ids: r.path_ids ?? [],
    sequence: 0,
    icon: '',
    creator_id: 0,
    editor_id: 0,
    create_time: r.create_time,
    update_time: r.updated_at ?? r.create_time,
  })) as unknown as Document[];
}

/**
 * 分享端点的响应有**两种形状**，取哪一种由"这道门开没开"决定：
 * - 带分享密码且还没输密码：服务端只回 `{ has_password: true }`。标题、空间名、创建者、
 *   整条面包屑路径都不给 —— 密码保护的是内容，而链接被转发出去时，光凭标题和路径就够认出来是什么。
 * - 其余（公开分享 / 密码已验证）：字段齐全。
 * 所以除 has_password 外一律可选，逼调用方处理"还没解锁"这一态。
 */
export interface ShareDocumentResponse {
  has_password?: boolean;
  id?: number;
  name?: string;
  type?: number;
  content?: string;
  space_name?: string;
  space_id?: number;
  creator_name?: string;
  creator_id?: number;
  editor_name?: string;
  editor_id?: number;
  create_time?: number;
  update_time?: number;
  breadcrumb?: { id: number; name: string }[];
}

export function shareDocument(id: number, password?: string) {
  const params: Record<string, unknown> = {};
  if (password) params.password = password;
  return get<ShareDocumentResponse>(`/documents/${id}/share`, { params });
}

// 评论 API 已迁移到 @/api/modules/social/comment（归属 social 模块，对应后端 internal/api/social）
// 历史兼容：如需使用请直接引入新路径，本文件不再 re-export

export function lockEdit(id: number) {
  return post<{ locked: boolean; locked_by?: number }>(`/documents/${id}/lock`);
}

export function unlockEdit(id: number) {
  return del<void>(`/documents/${id}/lock`);
}

export function heartbeatEdit(id: number) {
  return put<void>(`/documents/${id}/lock`);
}

export interface CollabState {
  document_id: number;
  active: boolean;
  state_length: number;
  version: number;
  clients: number;
}

export function getCollabState(id: number) {
  return get<CollabState>(`/collab/state/${id}`);
}

/** 服务端协作会话（internal/collab/hub.go ClientInfo） */
export interface CollabSession {
  user_id: number;
  user_name: string;
  /** 服务端游标颜色，形如 #rrggbb */
  color: string;
  /** RFC3339 字符串；当前实现返回的是查询时刻，非真实加入时间 */
  joined_at: string;
}

export function getCollabSessions(id: number) {
  return get<{ document_id: number; sessions: CollabSession[]; count: number }>(
    `/collab/sessions/${id}`,
  );
}

// #7 草稿/发布分离：published_at>0 即已发布，unpublish 将其清零回到草稿
export function publishDocument(id: number) {
  return post<null>(`/documents/${id}/publish`);
}

export function unpublishDocument(id: number) {
  return post<null>(`/documents/${id}/unpublish`);
}

export interface TrashItem {
  id: number;
  document_id: number;
  parent_id: number;
  space_id: number;
  name: string;
  type: number;
  path: string;
  sequence: number;
  content_path: string;
  deleted_by: number;
  deleted_at: number;
  delete_log: string;
}

export function restoreDocument(id: number) {
  return post<void>(`/documents/${id}/restore`);
}

export function permanentDeleteDocument(id: number) {
  return del<void>(`/documents/${id}/permanent`);
}

export function getSpaceTrash(spaceId: number) {
  return get<TrashItem[]>(`/spaces/${spaceId}/trash`);
}

export interface ImportDocumentItem {
  path: string;
  content: string;
  /** 后端 model/workspace.ImportDocumentItem.Format：md / confluence / notion */
  format?: string;
}

export interface ImportDocumentsResult {
  created: number;
  skipped: number;
  errors: string[];
}

export function importDocuments(spaceId: number, items: ImportDocumentItem[]) {
  return post<ImportDocumentsResult>(`/spaces/${spaceId}/import`, { items });
}
