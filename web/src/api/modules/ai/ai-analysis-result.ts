import { get } from '../../client';

export interface AIAnalysisResult {
  id: number;
  space_id: number;
  doc_id: number;
  summary: string;
  key_points: string[];
  category: string;
  tags: string[];
  quality_score: number;
  quality_issues: QualityIssue[];
  entities: ExtractedEntities;
  content_hash: string;
  created_at: string;
  updated_at: string;
}

export interface QualityIssue {
  type: string;
  severity: string;
  description: string;
  location: string;
}

export interface ExtractedEntities {
  urls: string[];
  config_items: string[];
  aliases: string[];
  action_items: string[];
  technologies?: Technology[];
  concepts?: Concept[];
}

export interface Technology {
  name: string;
  category: string;
  confidence: number;
}

export interface Concept {
  name: string;
  definition: string;
  aliases: string[];
}

export interface CategoryNode {
  name: string;
  count: number;
  children?: CategoryNode[];
}

export interface QualityStats {
  avg_score: number;
  total: number;
  low_quality: number;
  high_quality: number;
}

export interface KnowledgeGap {
  category: string;
  description: string;
  suggested_title: string;
  priority: string;
}

export interface AnalysisResultPage {
  list: AIAnalysisResult[];
  total: number;
  page_num: number;
  page_size: number;
}

export function getAnalysisResults(spaceId: number, page = 1, pageSize = 20) {
  return get<AnalysisResultPage>(`/ai/analysis-results/${spaceId}?page=${page}&page_size=${pageSize}`);
}

export function getCategories(spaceId: number) {
  return get<CategoryNode[]>(`/ai/analysis-results/${spaceId}/categories`);
}

export function getQualityStats(spaceId: number) {
  return get<QualityStats>(`/ai/analysis-results/${spaceId}/quality`);
}

export function getKnowledgeGaps(spaceId: number) {
  return get<KnowledgeGap[]>(`/ai/analysis-results/${spaceId}/gaps`);
}
