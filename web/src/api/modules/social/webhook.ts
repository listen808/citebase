import { get, post, put, del } from '../../client';

/**
 * Webhook 通知 API
 * 归属 social 模块（与 notification.ts 同域：notification 是站内信，webhook 是出站回调投递）
 * 对应后端 internal/api/workspace/webhook.go（路由前缀 /api/v1 已在 client baseURL 中）
 */

/** Webhook 配置：events 为逗号分隔的事件字符串（后端 aw_webhook_config.events） */
export interface WebhookConfig {
  id: number;
  user_id: number;
  space_id: number;
  name: string;
  url: string;
  /** 后端仅在非空时返回（json omitempty） */
  secret?: string;
  events: string;
  /** 1=启用，0=停用 */
  is_active: number;
  create_time: number;
  update_time: number;
}

/** 单次投递记录（后端 aw_webhook_delivery，不含 payload 正文） */
export interface WebhookDelivery {
  id: number;
  webhook_id: number;
  event: string;
  status_code: number;
  response: string;
  /** 1=成功，0=失败 */
  success: number;
  retry_count: number;
  create_time: number;
}

export interface CreateWebhookRequest {
  /** 必填，最长 100 */
  name: string;
  /** 必填，需为合法 URL，最长 500；后端会做 SSRF 校验（内网地址会被拒绝） */
  url: string;
  /** 可选，最长 100，用于 HMAC 签名 */
  secret?: string;
  /** 必填，逗号分隔的事件名 */
  events: string;
  /** 0 表示不绑定空间（全局） */
  space_id?: number;
}

/** 更新为增量语义：空字符串字段后端会跳过，is_active 仅在传入时生效 */
export interface UpdateWebhookRequest {
  name?: string;
  url?: string;
  secret?: string;
  events?: string;
  is_active?: 0 | 1;
}

/** 创建 Webhook */
export function createWebhook(data: CreateWebhookRequest) {
  return post<WebhookConfig>('/webhooks', data);
}

/** 当前用户的 Webhook 列表（后端直接返回数组） */
export function getWebhooks() {
  return get<WebhookConfig[]>('/webhooks');
}

/** 支持的事件类型，如 doc.created / comment.created */
export function getWebhookEvents() {
  return get<string[]>('/webhooks/events');
}

/** 更新 Webhook */
export function updateWebhook(id: number, data: UpdateWebhookRequest) {
  return put<WebhookConfig>(`/webhooks/${id}`, data);
}

/** 删除 Webhook */
export function deleteWebhook(id: number) {
  return del<void>(`/webhooks/${id}`);
}

/** 最近投递记录，limit 后端限制 1-100，默认 20 */
export function getWebhookDeliveries(id: number, limit = 20) {
  return get<WebhookDelivery[]>(`/webhooks/${id}/deliveries`, { params: { limit } });
}
