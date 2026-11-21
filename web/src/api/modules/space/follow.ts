import { get, post } from '../../client';
import type { FollowTarget } from '@/types/models';

export type { FollowTarget };
export type FollowUser = FollowTarget;

export function toggleFollowUser(userId: number) {
  return post<{ followed: boolean; follow: FollowTarget }>('/follow/user', { user_id: userId });
}

export function toggleFollowDocument(documentId: number) {
  return post<{ followed: boolean; follow: FollowTarget }>('/follow/document', { document_id: documentId });
}

export function getFollowingList(userId?: number) {
  const params: Record<string, unknown> = {};
  if (userId) params.user_id = userId;
  return get<{ data: FollowTarget[]; following: number; followers: number }>('/follow/following', { params });
}

export function getFollowersList(userId?: number) {
  const params: Record<string, unknown> = {};
  if (userId) params.user_id = userId;
  return get<{ data: FollowTarget[]; followers: number }>('/follow/followers', { params });
}

export function getFollowStats(userId?: number) {
  const params: Record<string, unknown> = {};
  if (userId) params.user_id = userId;
  return get<{ following: number; followers: number }>('/follow/stats', { params });
}
