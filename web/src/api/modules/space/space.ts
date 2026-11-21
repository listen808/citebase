import { get, post, put, del } from '../../client';
import type { Space, SpaceMember } from '@/types/models';
import type { PaginatedResponse } from '../common';

export interface CreateSpaceRequest {
  name: string;
  description?: string;
  tags?: string;
  visit_level?: string;
  is_share?: boolean;
  is_export?: boolean;
}

export interface UpdateSpaceRequest {
  name?: string;
  description?: string;
  tags?: string;
  visit_level?: string;
  is_share?: boolean;
  is_export?: boolean;
  sequence?: number;
}

export interface SpaceOrder {
  id: number;
  sequence: number;
}

export function getAllSpaces(params?: { visit_level?: string; tags?: string; page?: number; page_size?: number; keyword?: string }) {
  return get<PaginatedResponse<Space>>('/spaces', { params });
}

export function getSpaceByID(id: number) {
  return get<Space>(`/spaces/${id}`);
}

export function getMySpaces(params?: { visit_level?: string; tags?: string; page?: number; page_size?: number; keyword?: string }) {
  return get<PaginatedResponse<Space>>('/spaces/my', { params });
}

export function createSpace(data: CreateSpaceRequest) {
  return post<Space>('/spaces', data);
}

export function updateSpace(id: number, data: UpdateSpaceRequest) {
  return put<Space>(`/spaces/${id}`, data);
}

export function deleteSpace(id: number) {
  return del<void>(`/spaces/${id}`);
}

export interface SpaceDeleteCheck {
  document_count: number;
  member_count: number;
}

export function preCheckDeleteSpace(id: number) {
  return get<SpaceDeleteCheck>(`/spaces/${id}/pre-delete-check`);
}

export function getSpaceMembers(id: number) {
  return get<SpaceMember[]>(`/spaces/${id}/members`);
}

export function addSpaceMember(id: number, userId: number, privilege: number) {
  return post<void>(`/spaces/${id}/members`, { user_id: userId, privilege });
}

export function removeSpaceMember(id: number, userId: number) {
  return del<void>(`/spaces/${id}/members`, { data: { user_id: userId } });
}

export function updateMemberPrivilege(id: number, userId: number, privilege: number) {
  return put<void>(`/spaces/${id}/members/privilege`, { user_id: userId, privilege });
}

export function downloadSpace(spaceId: number) {
  return get<Blob>(`/spaces/${spaceId}/download`, { responseType: 'blob' });
}
export function sortSpaces(orders: SpaceOrder[]) {
  return put<void>('/spaces/sort', { orders });
}
