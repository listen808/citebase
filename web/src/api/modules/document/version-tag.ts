import { get, post, del } from '../../client';

/**
 * 文档版本标签 / 里程碑 API（后端 internal/api/workspace/document_version_tag.go）
 * version = 0 表示文档当前版本。
 */

/** 版本标签（对应 workspacemodel.DocumentVersionTagResponse） */
export interface VersionTag {
  id: number;
  document_id: number;
  version: number;
  tag_name: string;
  /** 后端原样存储的短色值/色名（max 20），本前端存语义色名 */
  color: string;
  note: string;
  created_by: number;
  create_time: number;
  /** 后端批量解析的创建者名称 */
  creator_name?: string;
}

/** 新建版本标签请求（对应 workspacemodel.CreateVersionTagRequest） */
export interface CreateVersionTagRequest {
  version: number;
  /** 必填，最长 100 */
  tag_name: string;
  /** 可选，最长 20 */
  color?: string;
  /** 可选，最长 500 */
  note?: string;
}

/** 创建版本标签：POST /documents/:id/version-tags（同名同版本返回 409） */
export function createVersionTag(documentId: number, data: CreateVersionTagRequest) {
  return post<VersionTag>(`/documents/${documentId}/version-tags`, data);
}

/** 文档全部版本标签：GET /documents/:id/version-tags */
export function getDocumentVersionTags(documentId: number) {
  return get<VersionTag[]>(`/documents/${documentId}/version-tags`);
}

/** 指定版本的标签：GET /documents/:id/versions/:version/tags */
export function getVersionTags(documentId: number, version: number) {
  return get<VersionTag[]>(`/documents/${documentId}/versions/${version}/tags`);
}

/** 删除版本标签：DELETE /version-tags/:tagId */
export function deleteVersionTag(tagId: number) {
  return del<void>(`/version-tags/${tagId}`);
}

/** 空间全部版本标签：GET /spaces/:id/version-tags */
export function getSpaceVersionTags(spaceId: number) {
  return get<VersionTag[]>(`/spaces/${spaceId}/version-tags`);
}
