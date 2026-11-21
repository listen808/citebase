import { get, post } from '../../client';
import type { KnowledgeGraphData } from '@/types/ai';

export function getKnowledgeGraph(spaceId?: number, format: 'echarts' | 'mermaid' = 'echarts') {
  const params = new URLSearchParams();
  if (spaceId) params.set('space_id', String(spaceId));
  params.set('format', format);
  return get<{ option?: Record<string, unknown>; mermaid?: string; data: KnowledgeGraphData }>(`/ai/knowledge-graph?${params}`);
}

export function runQualityPatrol(spaceId = 0) {
  return get<{
    total_docs: number;
    checked_docs: number;
    issue_count: number;
    health_score: number;
    issues: Array<{ document_id: number; document_name: string; issue_type: string; description: string; severity: string }>;
    suggestions: string[];
  }>(`/ai/quality-patrol?space_id=${spaceId}`);
}

export function getRecommendations(limit = 5) {
  return get<{ recommendations: Array<{ id: number; title: string; score: number; reason: string; space_id: number }> }>(`/ai/recommendations?limit=${limit}`);
}

export function importDocument(data: { content: string; format: string }) {
  return post<{ document: { title: string; content: string; format: string } }>('/ai/import', data);
}

export function searchOverview(data: { query: string; results?: Record<string, unknown>[]; space_id?: number }) {
  return post<{ overview: { overview: string; key_points: string[] } }>('/ai/search/overview', data);
}

export function autoTag(data: { content: string }) {
  return post<{ tags: { tags: string[]; category: string; confidence: number } }>('/ai/auto-tag', data);
}
