import { get, post, put, del, ensureValidToken, apiUrl } from '../../client';
import type { ChatMessage, AIConversation } from '@/types/ai';

// StreamMeta 流式末帧携带的元信息（引用来源、推理链、生成式组件等）。
export interface StreamMeta {
  tokenUsage: number;
  durationMs: number;
  sources?: unknown[];
  reasoningChain?: unknown[];
  components?: unknown[];
}

export interface StreamCallbacks {
  onChunk: (text: string) => void;
  onDone: () => void;
  onError: (err: string) => void;
  onFirstToken?: (latencyMs: number) => void;
  onMeta?: (meta: StreamMeta) => void;
}

export function streamAsk(
  data: { question: string; space_id?: number; limit?: number; history?: ChatMessage[]; images?: string[] },
  onChunk: (text: string) => void,
  onDone: () => void,
  onError: (err: string) => void,
  onFirstToken?: (latencyMs: number) => void,
  onMeta?: (meta: StreamMeta) => void,
): AbortController {
  const controller = new AbortController();

  const body = JSON.stringify(data);
  const url = apiUrl('/ai/stream/ask');

  // P0-8: 主动确保 token 有效（避免 fetch 绕过 axios 拦截器导致 401 后无自动刷新）
  ensureValidToken()
    .then((token) => {
      if (!token) {
        onError('登录已过期，请重新登录');
        return;
      }
      return fetch(url, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body,
        signal: controller.signal,
      });
    })
    .then(async (response) => {
      if (!response) return;
      if (!response.ok) {
        onError(`HTTP ${response.status}`);
        return;
      }
      const reader = response.body?.getReader();
      if (!reader) {
        onError('No readable stream');
        return;
      }
      const decoder = new TextDecoder();
      let buffer = '';

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() || '';

        for (const line of lines) {
          let raw: string;
          if (line.startsWith('data: ')) {
            raw = line.slice(6).trim();
          } else if (line.startsWith('data:')) {
            raw = line.slice(5).trim();
          } else {
            continue;
          }
          if (raw === '[DONE]') {
            onDone();
            return;
          }
          try {
            const parsed = JSON.parse(raw);
            if (parsed.error) {
              onError(parsed.error);
              return;
            }
            if (parsed.first_token && parsed.latency_ms != null) {
              onFirstToken?.(parsed.latency_ms);
            }
            if (parsed.meta && parsed.token_usage != null) {
              onMeta?.({
                tokenUsage: parsed.token_usage,
                durationMs: parsed.duration_ms,
                sources: parsed.sources,
                reasoningChain: parsed.reasoning_chain,
                components: parsed.components,
              });
            }
            if (parsed.content) {
              onChunk(parsed.content);
            }
          } catch {
            // skip malformed lines
          }
        }
      }
      onDone();
    })
    .catch((e) => {
      if (e?.name !== 'AbortError') {
        onError(e?.message || 'Stream error');
      }
    });

  return controller;
}

export function listConversations(page = 1, pageSize = 20, convType?: string) {
  const params: Record<string, number | string> = { page, page_size: pageSize };
  if (convType) params.type = convType;
  return get<{ conversations: AIConversation[]; total: number }>('/ai/conversations', { params });
}

export function createConversation(data: { title?: string; space_id?: number; type?: string }) {
  return post<{ conversation: AIConversation }>('/ai/conversations', data);
}

export function getConversation(id: number) {
  return get<{ conversation: AIConversation }>(`/ai/conversations/${id}`);
}

export function deleteConversation(id: number) {
  return del<{ deleted: boolean }>(`/ai/conversations/${id}`);
}

export function updateConversation(id: number, data: { title?: string; messages?: { role: string; content: string }[] }) {
  return put<{ conversation: AIConversation }>(`/ai/conversations/${id}`, data);
}

export function askAI(data: { question: string; space_id?: number; limit?: number; history?: ChatMessage[]; mode?: string; images?: string[] }) {
  return post<{ answer: string; sources: unknown[] }>('/ai/ask', data, { timeout: 120000 });
}

export function submitFeedback(data: { conversation_id?: number; question?: string; answer?: string; rating: 1 | -1; reason?: string }) {
  return post<{ feedback_id: number }>('/ai/feedback', data);
}
