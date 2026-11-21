import { get, post, put, del } from '../../client';

/**
 * 文档标签 API
 * 对应后端 internal/api/workspace/document_tag.go + tag_hierarchy.go
 * （路由 internal/api/workspace/routes.go:80-83、115-124；前缀 /api/v1 已在 client baseURL 中）
 */

/** 标签使用统计（GET /tags/stats 单项） */
export interface TagStat {
  tag_name: string;
  count: number;
}

/** 层级关系条目（parent_tag -> child_tag 的一条边） */
export interface TagHierarchyItem {
  id: number;
  parent_tag: string;
  child_tag: string;
  /** 0 表示全局关系 */
  space_id: number;
}

/** 层级树节点（GET /tags/hierarchy/tree） */
export interface TagTreeNode {
  tag_name: string;
  /** 后端 GetTagTree 未填充该字段，恒为 0；引用数请用 getTagStats 结果映射 */
  count: number;
  children?: TagTreeNode[];
}

export interface CreateTagHierarchyRequest {
  /** 必填，最长 100 */
  parent_tag: string;
  /** 必填，最长 100；全局唯一约束 uk_tag_child，一个子标签只能有一个父标签 */
  child_tag: string;
  /** 0 表示全局层级关系 */
  space_id?: number;
}

/** 获取文档标签 */
export function getDocumentTags(documentId: number) {
  return get<{ tags: string[] }>(`/documents/${documentId}/tags`);
}

/** 设置文档标签（全量替换，tags 为逗号分隔字符串） */
export function setDocumentTags(documentId: number, tags: string) {
  return put<{ tags: string[] }>(`/documents/${documentId}/tags`, { tags });
}

/** 追加文档标签（增量，tags 为逗号分隔字符串；重复标签后端忽略） */
export function addDocumentTags(documentId: number, tags: string) {
  return post<{ tags: string[] }>(`/documents/${documentId}/tags`, { tags });
}

/** 移除文档标签（tags 为逗号分隔字符串，后端要求非空） */
export function removeDocumentTags(documentId: number, tags: string) {
  return del<{ tags: string[] }>(`/documents/${documentId}/tags`, { data: { tags } });
}

/** 获取全局标签列表（按名称去重、升序） */
export function getAllTags(params?: { keyword?: string; limit?: number }) {
  return get<string[]>('/tags', { params });
}

/** 获取标签使用统计（按引用文档数倒序，Top N） */
export function getTagStats(limit = 20) {
  return get<TagStat[]>('/tags/stats', { params: { limit } });
}

/** 获取打了某标签的文档 ID 列表（只返回 ID，不含标题/空间） */
export function getDocumentsByTag(tag: string, limit = 50) {
  return get<{ document_ids: number[] }>(`/tags/${encodeURIComponent(tag)}/documents`, {
    params: { limit },
  });
}

/** 新建父子标签关系 */
export function createTagHierarchy(data: CreateTagHierarchyRequest) {
  return post<TagHierarchyItem>('/tags/hierarchy', data);
}

/** 完整层级树（space_id>0 时返回该空间 + 全局关系；=0 时返回全部） */
export function getTagHierarchyTree(spaceId = 0) {
  return get<TagTreeNode[]>('/tags/hierarchy/tree', { params: { space_id: spaceId } });
}

/** 某标签的直接子标签关系 */
export function getTagChildren(tag: string, spaceId = 0) {
  return get<TagHierarchyItem[]>(`/tags/${encodeURIComponent(tag)}/children`, {
    params: { space_id: spaceId },
  });
}

/** 某标签的父标签关系（无父标签时后端返回 404，需按 status 分支） */
export function getTagParent(tag: string) {
  return get<TagHierarchyItem>(`/tags/${encodeURIComponent(tag)}/parent`);
}

/** 删除一条层级关系（按关系条目 ID，非标签名） */
export function deleteTagHierarchy(id: number) {
  return del<void>(`/tags/hierarchy/${id}`);
}
