import { get, post, del } from '../../client';
import type { UpdateAIConfigRequest, RAGConfig, AnalyzerConfig, RAGDebugInfo } from '@/types/ai';

export function getAIConfig() {
  return get<{
    enabled: boolean;
    ai_space_name: string;
    provider?: string;
    base_url?: string;
    api_key?: string;
    model?: string;
    max_tokens?: number;
    temperature?: number;
    fast_model?: string;
    fast_provider?: string;
    fast_base_url?: string;
    reasoning_model?: string;
    reasoning_provider?: string;
    reasoning_base_url?: string;
  }>('/ai/config');
}

export function updateAIConfig(data: UpdateAIConfigRequest) {
  return post<{
    enabled: boolean;
    provider: string;
    model: string;
    temperature: number;
    max_tokens: number;
    restart_needed: boolean;
  }>('/ai/config/update', data);
}

export function testAIConnection() {
  return post<{ success: boolean; message: string }>('/ai/config/test');
}

// 按当前已保存的 primary 配置向上游发现模型（GET /v1/models），只发现不保存
export function discoverAIModels() {
  return post<{ success: boolean; models: string[]; message: string }>('/ai/config/models');
}

export function getVectorStats() {
  return get<{
    enabled: boolean;
    doc_count: number;
    embedder_ready: boolean;
    syncer_ready: boolean;
    embedding_model: string;
    vector_store_type: string;
    tasks: {
      pending: number; processing: number; done: number; failed: number; dead: number };
    recent_errors: Array<{ doc_id: number; doc_name: string; status: string; last_error: string; retry: number }>;
  }>('/ai/vector/stats');
}

// 向量重建 API
// P2-3：后端已提供统一入口 POST /ai/vector/reindex/unified（body: {scope, target_id}）
// 以下三个旧函数已改为封装统一入口，旧后端路弱仍保留作为 deprecated 兼容。

export type ReindexScope = 'all' | 'space' | 'doc';

export function reindexVectors(scope: ReindexScope, targetId?: number) {
  return post<{ message: string }>('/ai/vector/reindex/unified', { scope, target_id: targetId });
}

/** @deprecated 请使用 reindexVectors('all') */
export function reindexAllVectors() {
  return reindexVectors('all');
}

/** @deprecated 请使用 reindexVectors('space', spaceId) */
export function reindexSpaceVectors(spaceId: number) {
  return reindexVectors('space', spaceId);
}

/** @deprecated 请使用 reindexVectors('doc', docId) */
export function reindexDocVector(docId: number) {
  return reindexVectors('doc', docId);
}

// RAG 配置相关 API
export function getRAGConfig() {
  return get<{ config: RAGConfig }>('/ai/rag/config');
}

export function updateRAGConfig(data: Partial<RAGConfig>) {
  return post<{ config: RAGConfig }>('/ai/rag/config', data);
}

export function testRAGRetrieval(data: { question: string; space_id?: number; config?: Partial<RAGConfig> }) {
  // 后端 handler 返回 gin.H{"debug": RAGDebugInfo}（internal/api/ai/config.go TestRAGRetrieval）
  return post<{ debug: RAGDebugInfo | null }>('/ai/rag/test', data);
}

// Analyzer 配置
export function getAnalyzerConfig() {
  return get<AnalyzerConfig>('/ai/analyzer/config');
}

export function updateAnalyzerConfig(data: AnalyzerConfig) {
  return post<AnalyzerConfig>('/ai/analyzer/config', data);
}

// MCP 服务器管理
export interface MCPServerConfig {
  name: string;
  transport: string;
  command: string;
  args: string[];
  env?: Record<string, string>;
  endpoint?: string;
}

export interface MCPServerInfo {
  name: string;
  transport: string;
  command: string;
  args: string[];
  endpoint: string;
  connected: boolean;
  error: string;
  tool_count: number;
  tools: Array<{ name: string; description: string; parameters: unknown }>;
}

export function listMCPServers() {
  return get<{ servers: MCPServerInfo[] }>('/ai/mcp/servers');
}

export function addMCPServer(data: MCPServerConfig) {
  return post<{ name: string; connected: boolean; tool_count: number }>('/ai/mcp/servers', data);
}

export function removeMCPServer(name: string) {
  return del<{ removed: string }>(`/ai/mcp/servers/${encodeURIComponent(name)}`);
}

export function listMCPTools() {
  return get<{ tools: Array<{ name: string; description: string; parameters: unknown }> }>('/ai/mcp/tools');
}

/**
 * 自动分析调度。后端契约是**一条 cron 表达式 + 是否在跑**：
 * GET  /ai/config/schedule → {schedule: string, running: bool}
 * POST /ai/config/schedule ← {schedule: string}
 * 这里原先声明成 {enabled, cron, type, space_ids}，与后端完全不同——照着那个类型
 * 接上去只会读到 undefined，也正是这两个函数一直没人接的原因。
 */
export interface AIScheduleStatus {
  schedule: string;
  running: boolean;
}

export function getAISchedule() {
  return get<AIScheduleStatus>('/ai/config/schedule');
}

export function updateAISchedule(schedule: string) {
  return post<{ schedule: string }>('/ai/config/schedule', { schedule });
}
