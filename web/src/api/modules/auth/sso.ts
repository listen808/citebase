import { get, post, put, del } from '../../client';
import type { PaginatedResponse, PaginatedParams } from '../common';
import type { SSOConfig } from '@/types/models';

export interface CreateSSORequest {
  name: string;
  url: string;
  username_prefix: string;
  ext_data?: string;
}

export interface SSOLoginResponse {
  redirect_url: string;
  state: string;
}

export interface SSOCallbackResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  user_info: {
    user_id: number;
    username: string;
    given_name: string;
    role_id: number;
  };
}

export function getSSOConfigs(params?: PaginatedParams) {
  return get<PaginatedResponse<SSOConfig>>('/auth-sso', { params });
}

export function createSSO(data: CreateSSORequest) {
  return post<SSOConfig>('/auth-sso', data);
}

export function updateSSO(id: number, data: CreateSSORequest) {
  return put<SSOConfig>(`/auth-sso/${id}`, data);
}

export function deleteSSO(id: number) {
  return del<void>(`/auth-sso/${id}`);
}

export function setDefaultSSO(id: number) {
  return put<void>(`/auth-sso/${id}/default`);
}

export function initiateSSOLogin(id: number) {
  return get<SSOLoginResponse>(`/auth/sso/${id}/login`);
}

export function handleSSOCallback(code: string, state: string, sso_id: number) {
  return get<SSOCallbackResponse>('/auth/sso/callback', {
    params: { code, state, sso_id },
  });
}

export function getEnabledSSOList() {
  return get<Pick<SSOConfig, 'id' | 'name' | 'username_prefix'>[]>('/auth-sso/used');
}

// --- OIDC SSO API ---

export interface OIDCProviderPublicInfo {
  id: number;
  name: string;
  display_name: string;
  type: string;
  icon_url: string;
}

export interface OIDCLoginResponse {
  redirect_url: string;
  state: string;
}

export interface OIDCCallbackResponse {
  access_token: string;
  refresh_token: string;
  expires_in: number;
  user_info: {
    user_id: number;
    username: string;
    given_name: string;
    email: string;
    role_id: number;
  };
}

export interface OIDCProviderConfig {
  id: number;
  name: string;
  display_name: string;
  type: string;
  client_id: string;
  issuer_url: string;
  auth_url: string;
  token_url: string;
  user_info_url: string;
  scopes: string;
  enabled: boolean;
  auto_create: boolean;
  default_role: string;
  icon_url: string;
  sort_order: number;
  created_at: string;
  updated_at: string;
  /**
   * 最近一次初始化失败的原因（OIDC 服务发现 / 端点校验）。空表示正常。
   * 后端在创建/更新失败时仍返回成功，只看状态码发现不了配错了，必须读这个字段。
   */
  init_error?: string;
}

export interface CreateOIDCProviderRequest {
  name: string;
  display_name?: string;
  type: 'oidc' | 'google' | 'github' | 'azure' | 'keycloak';
  client_id: string;
  client_secret: string;
  issuer_url?: string;
  auth_url?: string;
  token_url?: string;
  user_info_url?: string;
  scopes?: string;
  enabled?: boolean;
  auto_create?: boolean;
  default_role?: string;
  icon_url?: string;
  sort_order?: number;
}

export interface UpdateOIDCProviderRequest {
  name: string;
  display_name?: string;
  type: 'oidc' | 'google' | 'github' | 'azure' | 'keycloak';
  client_id: string;
  client_secret?: string;
  issuer_url?: string;
  auth_url?: string;
  token_url?: string;
  user_info_url?: string;
  scopes?: string;
  enabled?: boolean;
  auto_create?: boolean;
  default_role?: string;
  icon_url?: string;
  sort_order?: number;
}

// Public: Get enabled OIDC providers for login page
export function getOIDCProviders() {
  return get<OIDCProviderPublicInfo[]>('/auth/oidc/providers');
}

// Public: Initiate OIDC login (returns redirect URL)
export function initiateOIDCLogin(providerId: number) {
  return get<OIDCLoginResponse>(`/auth/oidc/${providerId}/login`);
}

// Public: Handle OIDC callback
export function handleOIDCCallback(code: string, state: string, provider_id: number) {
  return get<OIDCCallbackResponse>('/auth/oidc/callback', {
    params: { code, state, provider_id },
  });
}

// Admin: Get all OIDC providers
export function getOIDCProviderAdmin() {
  return get<OIDCProviderConfig[]>('/auth/oidc/admin/providers');
}

// Admin: Create OIDC provider
export function createOIDCProvider(data: CreateOIDCProviderRequest) {
  return post<OIDCProviderConfig>('/auth/oidc/admin/providers', data);
}

// Admin: Update OIDC provider
export function updateOIDCProvider(id: number, data: UpdateOIDCProviderRequest) {
  return put<OIDCProviderConfig>(`/auth/oidc/admin/providers/${id}`, data);
}

// Admin: Delete OIDC provider
export function deleteOIDCProvider(id: number) {
  return del<void>(`/auth/oidc/admin/providers/${id}`);
}
