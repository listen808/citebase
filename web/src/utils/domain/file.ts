import { formatBytes } from '@/utils/format/format-bytes';
import { storageGet, StorageKeys } from '@/utils/core/storage';

const API_BASE_URL = import.meta.env.VITE_API_BASE_URL || (typeof window !== 'undefined' ? window.location.origin + '/api/v1' : '/api/v1');

/**
 * @deprecated 使用 `formatBytes` 替代，本函数保留仅为向后兼容
 */
export function formatSize(bytes: number) {
  return formatBytes(bytes);
}

export function resolveFileUrl(url: string | undefined): string {
  if (!url) return '';
  if (url.startsWith('http://') || url.startsWith('https://') || url.startsWith('//')) {
    return url;
  }
  const baseUrl = API_BASE_URL.replace(/\/api\/v1$/, '');
  const path = url.startsWith('/') ? url : '/' + url;
  let fullUrl = baseUrl + path;
  // 为图片/附件路径附加 token，支持后端 OptionalAuthMiddleware 的 ?token= 认证
  if (path.startsWith('/images/') || path.startsWith('/attachments/')) {
    const token = storageGet(StorageKeys.AUTH_TOKEN);
    if (token) {
      const sep = fullUrl.includes('?') ? '&' : '?';
      fullUrl += sep + 'token=' + encodeURIComponent(token);
    }
  }
  return fullUrl;
}

export function getDownloadUrl(url: string | undefined): string {
  if (!url) return '';
  let path = url.startsWith('/') ? url : '/' + url;
  // 为附件路径附加 token
  if (path.startsWith('/images/') || path.startsWith('/attachments/')) {
    const token = storageGet(StorageKeys.AUTH_TOKEN);
    if (token) {
      const sep = path.includes('?') ? '&' : '?';
      path += sep + 'token=' + encodeURIComponent(token);
    }
  }
  return path;
}
