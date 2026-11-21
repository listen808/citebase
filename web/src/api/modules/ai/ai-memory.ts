import { get, post, del } from '../../client';

export type MemoryType = 'preference' | 'knowledge' | 'session' | 'entity' | 'user_feedback';

// 记忆层级：short 默认 24h，importance>=0.7 升 long，90 天未命中降 stale，180 天删除
export type MemoryTier = 'short' | 'long' | 'stale';

// 记忆确认状态：LLM 推断的记忆先 pending，用户点头后 confirmed 才进召回
export type MemoryStatus = 'pending' | 'confirmed';

export interface AIMemory {
  id: number;
  user_id: number;
  memory_type: MemoryType;
  content: string;
  summary: string;
  source_conv_id: number;
  tags: string[];
  importance: number;
  access_count: number;
  memory_tier?: MemoryTier;
  status?: MemoryStatus;
  expires_at?: string;
  last_accessed_at?: string;
  created_at: string;
  updated_at: string;
}

export interface MemoryStats {
  total: number;
  preferences: number;
  knowledge: number;
  sessions: number;
  entities: number;
  user_feedback: number;
}

export interface MemoryRecallResult {
  memories: AIMemory[];
  recall_count: number;
  context: string;
}

// 获取记忆列表
export function listMemories(params?: { type?: string; page?: number; page_size?: number }) {
  return get<{ memories: AIMemory[]; total: number; page: number; page_size: number }>('/ai/memory', { params });
}

// 确认一条记忆，使其进入召回
export function confirmMemory(id: number) {
  return post<{ id: number; status: string }>(`/ai/memory/${id}/confirm`, {});
}

// 创建记忆
export function createMemory(data: {
  content: string;
  memory_type?: string;
  summary?: string;
  source_conv_id?: number;
  tags?: string[];
  importance?: number;
}) {
  return post<{ memory: AIMemory }>('/ai/memory', data);
}

// 删除记忆
export function deleteMemory(id: number) {
  return del<{ deleted: boolean }>(`/ai/memory/${id}`);
}

// 搜索记忆
export function searchMemories(data: { query: string; memory_type?: string; limit?: number }) {
  return post<{ memories: AIMemory[]; total: number }>('/ai/memory/search', data);
}

// 提取记忆（从对话中）
export function extractMemories(data: {
  conversation_id: number;
  messages: Array<{ role: string; content: string }>;
}) {
  return post<{ extracted: number; memories: AIMemory[] }>('/ai/memory/extract', data);
}

// 召回记忆（注入 prompt）
export function recallMemories(data: { query: string; memory_type?: string; limit?: number }) {
  return post<MemoryRecallResult>('/ai/memory/recall', data);
}

// 记忆统计
export function getMemoryStats() {
  return get<{ stats: MemoryStats }>('/ai/memory/stats');
}
