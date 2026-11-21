import { get, post } from '../../client';
import type { PaginatedResponse } from '../common';
import type { CollectionItem } from '@/types/models';

export type { CollectionItem };

export interface ToggleResult {
  collected: boolean;
  collection_id: number;
}

export function getMyCollections(params?: { type?: number; page?: number; page_size?: number; keyword?: string }, config?: { signal?: AbortSignal }) {
  return get<PaginatedResponse<CollectionItem>>('/collections', { params, ...config });
}

export function toggleCollection(type_: number, resourceId: number) {
  return post<ToggleResult>('/collections/toggle', { type: type_, resource_id: resourceId });
}

export function checkCollected(type_: number, resourceId: number) {
  return get<{ collected: boolean; collection_id: number }>('/collections/check', {
    params: { type: type_, resource_id: resourceId },
  });
}
