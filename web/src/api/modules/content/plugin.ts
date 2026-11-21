import { get, post, put, del } from '../../client';
import type { PaginatedResponse, PaginatedParams } from '../common';
import type { Plugin } from '@/types/models';

export type { Plugin };

export interface CreatePluginRequest {
  plugin_key: string;
  title: string;
  description?: string;
  cover_url?: string;
  conf_value?: string;
  is_used?: number;
}

export interface UpdatePluginRequest {
  title?: string;
  description?: string;
  cover_url?: string;
  is_used?: number;
}

export interface UpdatePluginConfigRequest {
  conf_value: string;
}

export function getPlugins(params?: PaginatedParams) {
  return get<PaginatedResponse<Plugin>>('/plugins', { params });
}

export function createPlugin(data: CreatePluginRequest) {
  return post<Plugin>('/plugins', data);
}

export function updatePlugin(id: number, data: UpdatePluginRequest) {
  return put<Plugin>(`/plugins/${id}`, data);
}

export function updatePluginConfig(id: number, conf_value: string) {
  return put<void>(`/plugins/${id}/config`, { conf_value });
}

export function togglePlugin(id: number) {
  return put<void>(`/plugins/${id}/toggle`);
}

export function deletePlugin(id: number) {
  return del<void>(`/plugins/${id}`);
}
