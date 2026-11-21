import { get } from '../../client';

export interface Span {
  id: string;
  trace_id: string;
  parent_id?: string;
  name: string;
  service: string;
  status: 'ok' | 'error' | 'running';
  start_time: string;
  end_time?: string;
  duration_ms: number;
  attributes?: Record<string, any>;
  events?: SpanEvent[];
  children?: Span[];
  error_msg?: string;
}

export interface SpanEvent {
  name: string;
  time: string;
  attributes?: Record<string, any>;
}

export interface MetricValue {
  name: string;
  type: 'counter' | 'gauge' | 'histogram';
  value: number;
  labels?: Record<string, string>;
  updated_at: string;
}

export interface DashboardSummary {
  total_requests: number;
  total_tokens: number;
  avg_duration_ms: number;
  active_requests: number;
  cache_hit_ratio: number;
  cache_hits: number;
  cache_misses: number;
}

export interface DashboardData {
  summary: DashboardSummary;
  metrics: MetricValue[];
  recent_traces: Span[];
}

export function getObservabilityTraces(limit?: number) {
  const query = limit ? `?limit=${limit}` : '';
  return get<{ traces: Span[]; total: number }>(`/ai/observability/traces${query}`);
}

export function getObservabilityDashboard() {
  return get<DashboardData>('/ai/observability/dashboard');
}
