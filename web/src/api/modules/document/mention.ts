import { get, post } from '../../client';

/**
 * 文档 @提及 API（后端 internal/api/workspace/mention.go）
 * 提及关系在文档保存时由后端从正文解析落库，前端只做查询与预览。
 */

/** 被提及用户（对应 workspacemodel.MentionUser） */
export interface MentionUser {
  user_id: number;
  username: string;
  given_name?: string;
}

/** 提及标识预览结果（对应后端 gin.H{"mentions": [...]}） */
export interface MentionPreviewResult {
  mentions: string[];
}

/** 文档的被提及用户名单：GET /documents/:id/mentions */
export function getDocumentMentions(documentId: number) {
  return get<MentionUser[]>(`/documents/${documentId}/mentions`);
}

/** 预览文本中会被解析出的 @标识：POST /mentions/preview */
export function previewMentions(content: string) {
  return post<MentionPreviewResult>('/mentions/preview', { content });
}
