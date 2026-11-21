/**
 * 上传约束的唯一来源：编辑器粘贴、附件面板、媒体管理、AI 聊天共用。
 * 之前分散在 useEditorUpload / MediaManageView / AIChatView 三处，后两者完全没有校验。
 */
export const MAX_IMAGE_SIZE = 10 * 1024 * 1024; // 10MB
export const MAX_ATTACHMENT_SIZE = 20 * 1024 * 1024; // 20MB
export const IMAGE_ACCEPT = 'image/png,image/jpeg,image/gif,image/webp,image/svg+xml';
/** AI 聊天上传更严格（历史行为，保留） */
export const MAX_CHAT_IMAGE_SIZE = 5 * 1024 * 1024; // 5MB
export const CHAT_IMAGE_TYPES = ['image/jpeg', 'image/jpg', 'image/png', 'image/gif', 'image/webp'];

export const ATTACHMENT_ACCEPT = 'image/*,.pdf,.doc,.docx,.xls,.xlsx,.ppt,.pptx,.txt,.md,.zip,.csv,.json';

/** 超限返回提示文案，未超限返回 null */
export function overSizeMessage(file: { name: string; size: number }, kind: 'image' | 'attachment'): string | null {
  const limit = kind === 'image' ? MAX_IMAGE_SIZE : MAX_ATTACHMENT_SIZE;
  if (file.size <= limit) return null;
  return `${file.name} 超过 ${limit / 1024 / 1024}MB 限制`;
}
