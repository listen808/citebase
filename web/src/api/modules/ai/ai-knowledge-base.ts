import { get, post } from '../../client';

export interface KnowledgeBase {
  id: number;
  name: string;
  description: string;
  doc_count: number;
  total_words: number;
  last_updated: string;
  status: number;
}

export interface DocChunkInfo {
  doc_id: number;
  doc_name: string;
  type: number;
  update_time: string;
  has_vector: boolean;
  vector_status: string;
}

export interface RetrievalResult {
  index: number;
  document_id: number;
  document_name: string;
  score: number;
  snippet: string;
  heading_path: string;
  source_type: string;
}

export interface KnowledgeBaseStats {
  space_id: number;
  space_name: string;
  total_docs: number;
  total_pages: number;
  total_dirs: number;
  vectorized_count: number;
  pending_vectors: number;
  failed_vectors: number;
  vector_completion: number;
  last_vector_sync: string;
}

export function listKnowledgeBases() {
  return get<{ knowledge_bases: KnowledgeBase[]; total: number }>('/ai/knowledge-bases');
}

export function listChunks(spaceId: number) {
  return get<{ documents: DocChunkInfo[]; total_docs: number; vectorized_count: number }>(
    `/ai/knowledge-bases/${spaceId}/chunks`,
  );
}

export function testRetrieval(spaceId: number, data: { query: string; top_k?: number }) {
  return post<{
    query: string;
    space_id: number;
    top_k: number;
    results: RetrievalResult[];
    result_count: number;
    method: string;
  }>(`/ai/knowledge-bases/${spaceId}/test-retrieval`, data);
}

export function getKnowledgeBaseStats(spaceId: number) {
  return get<KnowledgeBaseStats>(`/ai/knowledge-bases/${spaceId}/stats`);
}
