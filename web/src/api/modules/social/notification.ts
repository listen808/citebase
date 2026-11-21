import { get, put, del } from '../../client';

/**
 * 通知 API
 */

export interface NotificationItem {
  id: number;
  user_id: number;
  type: string;
  title: string;
  content: string;
  is_read: number;
  related_id: number;
  create_time: number;
}

/** 获取通知列表 */
export function getNotifications(params?: { type?: 'all' | 'unread'; page?: number; page_size?: number }) {
  return get<{ data: NotificationItem[]; total: number; page: number; page_size: number }>('/notifications', { params });
}

/** 获取未读通知数 */
export function getUnreadCount() {
  return get<{ count: number }>('/notifications/unread-count');
}

/** 标记通知已读 */
export function markNotificationRead(id: number) {
  return put<void>(`/notifications/${id}/read`);
}

/** 标记所有通知已读 */
export function markAllNotificationsRead() {
  return put<void>('/notifications/read-all');
}

/** 删除通知 */
export function deleteNotification(id: number) {
  return del<void>(`/notifications/${id}`);
}
