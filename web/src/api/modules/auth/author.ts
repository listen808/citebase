import { get, post } from '../../client';
import type { LoginRequest, LoginResponse } from './auth';

export { initiateSSOLogin } from './sso';

export interface AuthorConfig {
  sso_enabled: boolean;
  sso_list: { id: number; name: string; username_prefix: string }[];
}

export function getAuthorConfig() {
  return get<AuthorConfig>('/author/config');
}

export function authorLogin(data: LoginRequest) {
  return post<LoginResponse>('/author/login', data);
}
