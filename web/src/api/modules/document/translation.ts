import { get, post, del } from '../../client';

/**
 * 文档多语言翻译关联 API（后端 internal/api/workspace/translation.go）
 * 翻译关联只做「原文档 ↔ 译文文档」的挂接，不生成内容。
 */

/** 翻译状态：draft / published */
export type TranslationStatus = 'draft' | 'published';

/** 翻译关联（对应 workspacemodel.DocumentTranslation） */
export interface DocumentTranslation {
  id: number;
  /** 原文档 ID */
  document_id: number;
  /** 语言代码，如 zh-CN / en-US */
  lang: string;
  /** 译文文档 ID */
  translated_document_id: number;
  translation_status: TranslationStatus;
  created_at: number;
}

/** 语言信息（对应 workspacemodel.LanguageInfo） */
export interface LanguageInfo {
  code: string;
  name: string;
  native_name: string;
}

/** 创建翻译关联请求 */
export interface CreateTranslationRequest {
  lang: string;
  translated_document_id: number;
  translation_status?: TranslationStatus;
}

/** 支持的语言列表：GET /translations/languages */
export function getSupportedLanguages() {
  return get<LanguageInfo[]>('/translations/languages');
}

/** 文档翻译组（含反向关联）：GET /documents/:id/translations */
export function getDocumentTranslations(documentId: number) {
  return get<DocumentTranslation[]>(`/documents/${documentId}/translations`);
}

/** 创建翻译关联：POST /documents/:id/translations */
export function createTranslation(documentId: number, data: CreateTranslationRequest) {
  return post<DocumentTranslation>(`/documents/${documentId}/translations`, data);
}

/** 删除翻译关联：DELETE /translations/:id */
export function deleteTranslation(translationId: number) {
  return del<void>(`/translations/${translationId}`);
}
