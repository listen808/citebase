import { get } from '../../client';

// 对应 internal/api/admin/export.go（G037 数据导出中心）
// 与 internal/api/admin/compliance.go（G038 合规报告）。
// 注意：三个端点中只有 GET /stats/compliance-report 走统一响应信封；
// 两个 export 端点直接回写文件流（text/csv / application/json + Content-Disposition），
// 必须以 responseType: 'blob' 接收。

/** 与 internal/service/admin.ExportService.SupportedModules() 严格一致 */
export const EXPORT_MODULES = ['documents', 'spaces', 'users'] as const;

export type ExportModule = (typeof EXPORT_MODULES)[number];

/** 后端 doExport 仅接受 csv | json，其它值返回 400 */
export type ExportFormat = 'csv' | 'json';

/** 导出模块中文名，用于界面展示 */
export const EXPORT_MODULE_LABELS: Record<ExportModule, string> = {
  documents: '文档',
  spaces: '空间',
  users: '用户（含 PII）',
};

/**
 * GET /stats/export/:module?format=
 * users 模块含全量用户 PII，后端注释明确禁止走开放平台通道，
 * 管理端 session + RBAC 是唯一允许路径。
 */
export function exportModuleData(module: ExportModule, format: ExportFormat = 'csv') {
  return get<Blob>(`/stats/export/${module}`, { params: { format }, responseType: 'blob' });
}

// ==================== 合规报告 ====================

/** internal/service/admin.ActionStat */
export interface ComplianceActionStat {
  action: string;
  count: number;
  success_count: number;
  fail_count: number;
}

/** internal/repository/ai.UserUsageStat */
export interface ComplianceTopUser {
  user_id: number;
  username: string;
  given_name: string;
  total_usage: number;
  request_count: number;
  avg_usage: number;
  last_used: string;
}

/** internal/repository/ai.DailyUsageStat */
export interface ComplianceDailyTrend {
  date: string;
  total_usage: number;
  request_count: number;
  unique_users: number;
}

/**
 * internal/service/admin.ComplianceReport
 * 三个列表字段由 Go 侧 append 到 nil slice，无数据时序列化为 null。
 */
export interface ComplianceReport {
  /** RFC3339 字符串（time.Time） */
  generated_at: string;
  period_days: number;
  total_events: number;
  success_count: number;
  fail_count: number;
  /** 百分比数值（0-100），后端已乘 100 */
  success_rate: number;
  by_action: ComplianceActionStat[] | null;
  top_users: ComplianceTopUser[] | null;
  daily_trend: ComplianceDailyTrend[] | null;
}

/** GET /stats/compliance-report?days= —— days 取值 (0,365]，越界后端回落到 30 */
export function getComplianceReport(days: number = 30) {
  return get<ComplianceReport>('/stats/compliance-report', { params: { days } });
}

/** GET /stats/compliance-report/export?days=&format= —— 返回 csv/json 文件流 */
export function exportComplianceReport(days: number = 30, format: ExportFormat = 'csv') {
  return get<Blob>('/stats/compliance-report/export', {
    params: { days, format },
    responseType: 'blob',
  });
}
