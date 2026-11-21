import { get, post, put } from '../../client';
import type { User } from '@/types/models';

export interface LoginRequest {
  username: string;
  password: string;
}

export interface LoginResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  user_info?: User;
}

export function login(data: LoginRequest) {
  return post<LoginResponse>('/auth/login', data);
}

export function refreshToken(refreshToken: string) {
  return post<LoginResponse>('/auth/refresh', { refresh_token: refreshToken });
}

export function logout() {
  return post<void>('/auth/logout');
}

export function getCurrentUser() {
  // P0 修复：后端 UserInfo.UserID 已改为 json:"id"，无需前端兜底转换
  return get<User>('/auth/me');
}

/**
 * PUT /auth/me 请求体，与后端 usermodel.UpdateUserRequest 对齐。
 * 后端在此接口强制 role_id = 0，因此不接受角色变更。
 * 服务端对空字符串按「不修改」处理，所以清空字段无法通过该接口完成。
 */
export interface UpdateProfileRequest {
  given_name?: string;
  mobile?: string;
  phone?: string;
  email?: string;
  department?: string;
  position?: string;
  location?: string;
  im?: string;
}

/**
 * 自助更新个人资料（任意登录用户，无需 user:modify 权限）。
 * 返回 UserInfo，不含 phone / location / im，保存后需再 fetchUser() 回显。
 */
export function updateCurrentUser(data: UpdateProfileRequest) {
  return put<User>('/auth/me', data);
}

/**
 * 自助保存个性化偏好（外观 × 材质 × 透明变体）。
 * 返回整份 UserInfo —— 里面的 `preferences.updated_at` 是服务端盖写的，
 * 前端把它记下来当"这台设备与账号一致的时刻"，否则采纳之后会立刻判定本地更新再推回去。
 */
export function updateOwnPreferences(data: { theme: string; material: string; glass_clear: boolean }) {
  return put<User>('/auth/me/preferences', data);
}

/** 自助修改密码（校验旧密码，new_password 后端约束 6-60 位） */
export function changeOwnPassword(oldPassword: string, newPassword: string) {  return put<void>('/auth/me/password', {
    old_password: oldPassword,
    new_password: newPassword,
  });
}
