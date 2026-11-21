import { get, post, put, del } from '../../client';
import type { PaginatedResponse } from '../common';
import type { User } from '@/types/models';

export interface CreateUserRequest {
  username: string;
  password: string;
  given_name?: string;
  email?: string;
  mobile?: string;
  phone?: string;
  department?: string;
  position?: string;
  location?: string;
  im?: string;
  role_id: number;
}

export interface UpdateUserRequest {
  given_name?: string;
  mobile?: string;
  phone?: string;
  email?: string;
  department?: string;
  position?: string;
  location?: string;
  im?: string;
  role_id?: number;
}

export function getUsers(params?: { page?: number; page_size?: number; role_id?: number; keyword?: string }) {
  return get<PaginatedResponse<User>>('/users', { params });
}

export function getUserByID(id: number) {
  return get<User>(`/users/${id}`);
}

export function createUser(data: CreateUserRequest) {
  return post<User>('/users', data);
}

export function updateUser(id: number, data: UpdateUserRequest) {
  return put<User>(`/users/${id}`, data);
}

export function deleteUser(id: number) {
  return del<void>(`/users/${id}`);
}

export function changePassword(id: number, old_password?: string, new_password?: string) {
  return put<void>(`/users/${id}/password`, { old_password, new_password });
}

export function toggleUserForbidden(id: number) {
  return put<void>(`/users/${id}/forbidden`);
}

export function exportUsersCSV() {
  return get<Blob>('/users/export', { responseType: 'blob' });
}
