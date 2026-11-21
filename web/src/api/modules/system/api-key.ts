import { get, post } from '../../client';

/**
 * 开放平台 API Key 管理（G029）
 * 对应后端 internal/api/admin/api_key.go
 * 路由 internal/api/admin/routes.go:54-56，走 AuthMiddleware + AutoRBAC（管理端会话）。
 * 说明：后端未提供 DELETE / 更新端点，密钥只能吊销（status=revoked），不可删除。
 */

/** active | revoked（internal/model/admin.ApiKeyStatusActive / Revoked） */
export type ApiKeyStatus = 'active' | 'revoked';

/**
 * internal/model/admin.ApiKey 的列表投影（api_key.go 内联 out 结构体）。
 * ExpiresAt / LastUsedAt 是 time.Time 值类型而非指针，未设置时序列化为
 * "0001-01-01T00:00:00Z"，展示需按零值处理。列表不含任何密钥材料。
 */
export interface ApiKeyRecord {
  id: number;
  app_name: string;
  status: ApiKeyStatus | string;
  /** 每分钟请求上限 */
  rate_limit: number;
  expires_at: string;
  last_used_at: string;
  created_at: string;
}

export interface ApiKeyListResult {
  api_keys: ApiKeyRecord[];
  total: number;
}

export interface CreateApiKeyRequest {
  /** 必填 */
  app_name: string;
  /** 可选，<=0 时后端回落为 60 */
  rate_limit?: number;
  /** 有效期天数，0 或不传表示不过期 */
  expires_in_days?: number;
}

export interface CreateApiKeyResponse {
  id: number;
  app_name: string;
  status: ApiKeyStatus | string;
  rate_limit: number;
  expires_at: string;
  /** 明文密钥（形如 mwk_<32hex>），后端仅在此响应中出现一次 */
  api_key: string;
  created_at: string;
}

/** POST /open/api-keys */
export function createApiKey(data: CreateApiKeyRequest) {
  return post<CreateApiKeyResponse>('/open/api-keys', data);
}

/** GET /open/api-keys —— 无分页参数，一次返回全部 */
export function listApiKeys() {
  return get<ApiKeyListResult>('/open/api-keys');
}

/** POST /open/api-keys/:id/revoke —— 后端返回 message「密钥已吊销」 */
export function revokeApiKey(id: number) {
  return post<null>(`/open/api-keys/${id}/revoke`);
}
