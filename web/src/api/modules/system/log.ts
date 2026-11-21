import { get } from '../../client';
import type { PaginatedResponse } from '../common';

export interface DocumentHistory {
  id: number;
  document_id: number;
  space_id: number;
  action: number;
  user_id: number;
  change_log: string;
  create_time: number;
  operator?: {
    user_id: number;
    username: string;
    given_name: string;
  };
}

export interface UserActivity {
  id: number;
  document_id: number;
  space_id: number;
  user_id: number;
  username?: string;
  given_name?: string;
  document_name?: string;
  space_name?: string;
  change_log?: string;
  action: string;
  target_type: string;
  target_id: number;
  detail?: string;
  create_time: number;
  operator?: {
    user_id: number;
    username: string;
    given_name: string;
  };
  document?: {
    document_id: number;
    name: string;
    space_id: number;
  };
}

export interface SystemLog {
  log_id: number;
  user_id?: number;
  username: string;
  given_name?: string;
  path: string;
  method: string;
  level: string;
  ip: string;
  user_agent: string;
  create_time: number;
  get_params?: string;
  post_params?: string;
  referer?: string;
}

export interface RecentError {
  id: number;
  message: string;
  level: string;
  create_time: number;
}

export function getDocumentHistory(docId: number, limit = 20) {
  return get<DocumentHistory[]>(`/documents/${docId}/history`, { params: { limit } });
}

export function getUserActivity(params?: { page?: number; page_size?: number; action?: string }) {
  const queryParams: Record<string, unknown> = {
    page: params?.page || 1,
    page_size: params?.page_size || 20
  };
  if (params?.action && params.action !== 'all') {
    queryParams.action = params.action;
  }
  return get<PaginatedResponse<UserActivity>>('/logs/activity', { params: queryParams });
}

export function getUserActivityById(userId: number, page = 1, pageSize = 20) {
  return get<{ data: UserActivity[]; total: number; page: number; page_size: number }>(`/logs/users/${userId}/activity`, {
    params: { page, page_size: pageSize },
  });
}

export function getRecentErrors(limit = 5) {
  return get<RecentError[]>('/logs/recent-errors', { params: { limit } });
}

export function getSystemLogs(level = 0, page = 1, pageSize = 50, username?: string, message?: string) {
  const params: Record<string, unknown> = { level, page, page_size: pageSize };
  if (username) params.username = username;
  if (message) params.message = message;
  return get<{ data: SystemLog[]; total: number; page: number; page_size: number }>('/logs/system', { params });
}

export function getLogDetail(id: number) {
  return get<SystemLog>(`/logs/${id}`);
}
