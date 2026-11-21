import { get, post, del } from '../../client';
import type { Attachment, UploadResult } from '@/types/models';

export type { Attachment, UploadResult };

export function getDocumentImages(documentId: number) {
  return get<Attachment[]>(`/images/${documentId}`);
}

export function getDocumentAttachments(documentId: number, source = -1) {
  return get<Attachment[]>(`/attachments/${documentId}`, { params: { source } });
}

// 统一上传入口：图片和附件都用同一个接口
export function uploadAttachment(document_id: number, file: File, onProgress?: (progress: number) => void) {
  const form = new FormData();
  form.append('file', file);
  return post<UploadResult>(`/attachments/upload/${document_id}`, form, {
    onUploadProgress: (progressEvent) => {
      if (progressEvent.total && onProgress) {
        const progress = Math.round((progressEvent.loaded / progressEvent.total) * 100);
        onProgress(progress);
      }
    },
  });
}

export function deleteAttachment(id: number) {
  return del<void>(`/attachments/${id}`);
}
