import { get, post, put, del } from '../../client';
import type { CommentItem } from '@/types/models';

export type { CommentItem };

// 评论 API：归属 social 模块（对应后端 internal/api/social）
// 迁移自 @/api/modules/document/document，document.ts 中不再定义

export function createComment(
  documentId: number,
  content: string,
  selectionText?: string,
  selectionRange?: string,
  parentId?: number,
) {
  return post<CommentItem>('/comments', {
    document_id: documentId,
    content,
    selection_text: selectionText,
    selection_range: selectionRange,
    parent_id: parentId || 0,
  });
}

export function getComments(documentId: number) {
  return get<CommentItem[]>(`/comments/${documentId}`);
}

export function deleteComment(id: number) {
  return del<void>(`/comments/${id}`);
}

export function resolveComment(id: number, resolved: boolean) {
  return put<void>(`/comments/${id}/resolve`, { resolved });
}
