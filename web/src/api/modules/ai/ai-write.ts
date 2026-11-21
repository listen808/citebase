import { post, get } from '../../client';
import type {
  AIWriteRequest, AIAskSource,
  AITableRequest, AITableResult,
  AIReportRequest, AIReportResult,
  AITranslateRequest, AITranslateResult,
  AIRecommendation, AIPatrolResult,
} from '@/types/ai';
import type { ApplyTemplateRequest, AITemplate } from '@/types/ai-writer';

export function writeAssist(data: AIWriteRequest) {
  return post<{ result: string }>('/ai/write', data, { timeout: 120000 });
}

export function listTemplates() {
  return get<{ templates: AITemplate[] }>('/ai/templates');
}

export function applyTemplate(data: ApplyTemplateRequest) {
  return post<{ result: string }>('/ai/templates/apply', data, { timeout: 120000 });
}

export function chatWithDoc(data: { 
  messages: Array<{ role: string; content: string }>; 
  spaceId?: number;
  docId?: number;
  selectedText?: string;
}) {
  return post<{ content: string; sources?: AIAskSource[] }>('/ai/chat', data, { timeout: 120000 });
}

export function generateTable(data: AITableRequest) {
  return post<AITableResult>('/ai/table/generate', data, { timeout: 120000 });
}

export function generateReport(data: AIReportRequest) {
  return post<AIReportResult>('/ai/report/generate', data, { timeout: 120000 });
}

export function translateText(data: AITranslateRequest) {
  return post<AITranslateResult>('/ai/translate', data, { timeout: 120000 });
}

export function getRecommendations(limit = 5) {
  return get<{ recommendations: AIRecommendation[] }>(`/ai/recommendations?limit=${limit}`);
}

export function runQualityPatrol(spaceId = 0) {
  return get<AIPatrolResult>(`/ai/quality-patrol?space_id=${spaceId}`);
}
