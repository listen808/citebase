import { get, post, put, del } from '../../client';
import type { Link } from '@/types/models';

export type { Link };

export function getLinks(page = 1, pageSize = 20, keyword = '') {
  return get<{ data: Link[]; total: number; page: number; page_size: number }>('/links', {
    params: { page, page_size: pageSize, keyword },
  });
}

export function saveLink(name: string, url: string, sort = 0) {
  return post<Link>('/links', { name, url, sort });
}

export function updateLink(id: number, name: string, url: string, sort = 0) {
  return put<Link>(`/links/${id}`, { name, url, sort });
}

export function deleteLink(id: number) {
  return del<void>(`/links/${id}`);
}

/**
 * 记一次"有人打开了这条链接"。任何登录用户都能报（后端 MinRole 3），
 * 失败不该打断打开动作 —— 所以调用方一律 fire-and-forget。
 */
export function reportLinkClick(id: number) {
  return post<{ id: number; click_count: number }>(`/links/${id}/click`);
}
