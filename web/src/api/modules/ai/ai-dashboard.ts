import { get, post } from '../../client';

export function getAIDashboard() {
  return get<{
    enabled: boolean;
    token_usage: number;
    token_by_user: Record<string, number>;
    cache_size: number;
    vector_docs: number;
    analysis_task_count: number;
    provider: { name?: string };
    feedback: { positive: number; negative: number; total: number; recent_reasons: string[] };
  }>('/ai/dashboard');
}



export function getTokenUsage() {
  return get<{ total_tokens_today: number; by_user: Record<string, number> }>('/ai/token/usage');
}

export function getTokenUsageByModel(params?: { days?: number; limit?: number }) {
  const searchParams = new URLSearchParams();
  if (params?.days) searchParams.set('days', params.days.toString());
  if (params?.limit) searchParams.set('limit', params.limit.toString());
  const query = searchParams.toString() ? `?${searchParams.toString()}` : '';
  return get<{
    stats: Array<{
      model: string;
      provider: string;
      total_usage: number;
      request_count: number;
      avg_usage: number;
      percent: number;
    }>;
    total: number;
    days: number;
  }>(`/ai/token/usage/by-model${query}`);
}

export function getTokenUsageByUser(params?: { days?: number; limit?: number }) {
  const searchParams = new URLSearchParams();
  if (params?.days) searchParams.set('days', params.days.toString());
  if (params?.limit) searchParams.set('limit', params.limit.toString());
  const query = searchParams.toString() ? `?${searchParams.toString()}` : '';
  return get<{
    stats: Array<{
      user_id: number;
      username: string;
      total_usage: number;
      request_count: number;
      avg_usage: number;
      last_used: string;
      percent: number;
    }>;
    total: number;
    days: number;
  }>(`/ai/token/usage/by-user${query}`);
}

export function getTokenUsageDailyTrend(params?: { days?: number }) {
  const searchParams = new URLSearchParams();
  if (params?.days) searchParams.set('days', params.days.toString());
  const query = searchParams.toString() ? `?${searchParams.toString()}` : '';
  return get<{
    trend: Array<{
      date: string;
      total_usage: number;
      request_count: number;
      unique_users: number;
      percent: number;
    }>;
    max: number;
    days: number;
  }>(`/ai/token/usage/daily${query}`);
}

export function getQueryCacheStats() {
  return get<{ enabled: boolean; size: number }>('/ai/cache/stats');
}

export function clearQueryCache() {
  return post<{ cleared: boolean }>('/ai/cache/clear');
}

export function getEmbedCode() {
  return get<{ js_embed: string; iframe_embed: string; base_url: string }>('/ai/embed-code');
}

export function generateSummary(documentId: number) {
  return post<{ summary: string }>(`/ai/summary/${documentId}`, {}, { timeout: 120000 });
}

export function getAIAuditLogs(params?: { limit?: number; offset?: number; action?: string }) {
  const searchParams = new URLSearchParams();
  if (params?.limit) searchParams.set('limit', params.limit.toString());
  if (params?.offset) searchParams.set('offset', params.offset.toString());
  if (params?.action) searchParams.set('action', params.action);
  const query = searchParams.toString() ? `?${searchParams.toString()}` : '';
  return get<{
    logs: Array<{
      id: number;
      user_id: number;
      action: string;
      target_id?: string;
      detail?: string;
      result?: string;
      reason?: string;
      ip_address?: string;
      created_at: string;
    }>;
    total: number;
    limit: number;
    offset: number;
  }>(`/ai/audit-logs${query}`);
}
