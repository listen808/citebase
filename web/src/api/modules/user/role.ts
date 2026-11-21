import { get, post, put, del } from '../../client';
import type { Role } from '@/types/models';

export interface PrivilegeNode {
  id: number;
  name: string;
  parent_id: number;
  type: string;
  controller: string;
  action: string;
  icon: string;
  target?: string;
  is_display: number;
  is_menu?: number;
  sequence?: number;
  list_order?: number;
  is_delete?: number;
  create_time?: number;
  update_time?: number;
  children?: PrivilegeNode[];
}

export interface RoleUser {
  id: number;
  username: string;
  given_name: string;
  email: string;
  mobile: string;
  department: string;
  role_id: number;
  is_forbidden: number;
  last_time: number;
  create_time: number;
}

export interface RoleUserListResponse {
  data: RoleUser[];
  total: number;
  page: number;
  page_size: number;
  role_name: string;
}

export function getRoles() {
  return get<Role[]>('/roles');
}

export function createRole(name: string) {
  return post<Role>('/roles', { name });
}

export function updateRole(id: number, name: string) {
  return put<Role>(`/roles/${id}`, { name });
}

export function deleteRole(id: number) {
  return del<void>(`/roles/${id}`);
}

export function getPrivileges() {
  return get<PrivilegeNode[]>('/privileges');
}

export function getRolePrivileges(id: number) {
  return get<number[]>(`/roles/${id}/privileges`);
}

export function assignPrivileges(id: number, privilege_ids: number[]) {
  return post<void>(`/roles/${id}/privileges`, { privilege_ids });
}

export function getRoleUsers(roleId: number, page: number = 1, pageSize: number = 20) {
  return get<RoleUserListResponse>(`/roles/${roleId}/users`, { params: { page, page_size: pageSize } });
}

export function createPrivilege(data: {
  name: string;
  type: string;
  parent_id?: number;
  controller?: string;
  action?: string;
  icon?: string;
  target?: string;
  is_display?: number;
  sequence?: number;
}) {
  return post<PrivilegeNode>('/privileges', data);
}

export function updatePrivilege(id: number, data: {
  name?: string;
  type?: string;
  parent_id?: number;
  controller?: string;
  action?: string;
  icon?: string;
  target?: string;
  is_display?: number;
  sequence?: number;
}) {
  return put<PrivilegeNode>(`/privileges/${id}`, data);
}

export function deletePrivilege(id: number) {
  return del<void>(`/privileges/${id}`);
}

export function resetRoleUsers(roleId: number) {
  return post<void>(`/roles/${roleId}/reset-users`);
}

export function addUserToRole(roleId: number, username: string) {
  return post<void>(`/roles/${roleId}/users`, { username });
}
