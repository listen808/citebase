// AI 审计动作枚举 → i18n 键（唯一事实源）。
// 后端写入点见 internal/api/ai/chat*.go 的 RecordAIAudit 与 internal/text2sql/trust/audit.go。
// 消费方：AIOperationsView（筛选器 + 表格徽章）、DataExportView（合规报告动作列）。
export const AI_AUDIT_ACTION_LABEL_KEYS: Record<string, string> = {
  ask: 'aiView2.k66',
  ask_debug: 'aiView2.k137',
  multi_hop_ask: 'aiView2.k138',
  self_rag_ask: 'aiView2.k139',
  summary: 'aiView2.k140',
  write_assist: 'aiView2.k141',
  table_generate: 'aiView2.k142',
  report_generate: 'aiView2.k143',
  email_generate: 'aiView2.k144',
  translate: 'aiView2.k145',
  describe_image: 'aiView2.k146',
  import_doc: 'aiView2.k147',
  search_overview: 'aiView2.k148',
  auto_tag: 'aiView2.k149',
  workflow: 'aiView2.k67',
  test_rag: 'aiView2.k150',
  ragas_eval: 'aiView2.k151',
  text2sql: 'aiView2.k152',
};

/** 认不出的动作回退原值（后端加了新枚举时界面不崩，只是暂时直出） */
export function auditActionLabel(t: (key: string) => string, action: string): string {
  const key = AI_AUDIT_ACTION_LABEL_KEYS[action];
  return key ? t(key) : action;
}
