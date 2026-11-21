import { get, post } from '../../client';
import type { AnalysisTask, StartAnalysisRequest } from '@/types/ai';

export function startAnalysis(data?: StartAnalysisRequest) {
  return post<{ message: string }>('/ai/analysis/start', data || {});
}

export function listAnalysisTasks() {
  return get<{ list: AnalysisTask[]; tasks?: AnalysisTask[]; total: number }>('/ai/analysis/tasks');
}

export function pauseAnalysis(taskId: string) {
  return post<{ task_id: string; status: string }>(`/ai/analysis/pause/${taskId}`);
}

export function resumeAnalysis(taskId: string) {
  return post<{ task_id: string; status: string; message: string }>(`/ai/analysis/resume/${taskId}`);
}

export interface MergePlan {
  target_doc_id: number;
  source_doc_id: number;
  sections_to_keep: string[];
  gaps_to_fill: string[];
  reason: string;
}

/**
 * 为一对重复文档生成合并建议。后端契约是 body 必填 `doc_ids`（至少 2 个），
 * 且**只取前两篇**、返回 plan —— 它不合并任何文档。
 * 此前这里声明成 `mergeCluster(clusterId)` 且不带 body，照它调用必然 400
 * 「doc_ids are required」，这也正是这个能力一直没被接上的原因。
 * clusterIndex 仅用于后端在理由里标注是第几组，系统里没有持久化的数字簇 ID。
 */
export function mergeCluster(clusterIndex: number, docIds: number[]) {
  return post<{ plan: MergePlan; message: string }>(`/ai/analysis/merge/${clusterIndex}`, { doc_ids: docIds });
}

export function detectDuplicates(data: { space_id?: number; threshold?: number }) {
  return post<{ total_duplicates: number; groups: Array<Record<string, unknown>>; message: string }>('/ai/analysis/detect-duplicates', data);
}

export function getClusters(spaceId?: number) {
  return get<{ clusters: Array<Record<string, unknown>>; message?: string }>('/ai/analysis/clusters', { params: spaceId ? { space_id: spaceId } : undefined });
}
