import { post } from '../../client';

/**
 * 高级问答试跑通道（多跳推理 / Self-RAG）
 * 路由：internal/api/ai/routes.go:97-98（handler internal/api/ai/chat_advanced.go MultiHopAsk/SelfRAGAsk）
 * 请求体 = internal/api/ai/chat.go askRequest；响应 = rag.AIAnswer 的子集 {answer, sources}
 * 字段对齐 internal/ai/rag/retriever.go Source
 * 注意：history / images 后端会解析但这两个通道不使用；limit 缺省 5、上限 20。
 */

export type AdvancedAskChannel = 'multi-hop' | 'self-rag';

export interface AdvancedAskRequest {
  question: string;
  /** 0 表示不限定空间 */
  space_id?: number;
  limit?: number;
}

/** rag.Source（internal/ai/rag/retriever.go:26） */
export interface AskSource {
  document_id: number;
  document_name: string;
  space_id: number;
  score: number;
  snippet: string;
  start_line: number;
  end_line: number;
  heading_path: string;
  keywords: string[] | null;
  citation_index: number;
  updated_at: number;
  source_type?: string;
  /** 可验证引用增强：cited / unverified / not_found */
  cited?: boolean;
  verify_status?: string;
  doc_url?: string;
}

export interface AdvancedAskResult {
  answer: string;
  sources: AskSource[] | null;
}

export const AdvancedAskAPI = {
  /** POST /ai/multi-hop-ask */
  multiHop(data: AdvancedAskRequest) {
    return post<AdvancedAskResult>('/ai/multi-hop-ask', data);
  },

  /** POST /ai/self-rag-ask */
  selfRag(data: AdvancedAskRequest) {
    return post<AdvancedAskResult>('/ai/self-rag-ask', data);
  },

  run(channel: AdvancedAskChannel, data: AdvancedAskRequest) {
    return channel === 'self-rag' ? AdvancedAskAPI.selfRag(data) : AdvancedAskAPI.multiHop(data);
  },
};
