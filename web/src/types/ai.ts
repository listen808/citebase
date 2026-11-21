export interface AnalysisTask {
  id: string;
  type: 'full' | 'incremental' | 'space_only' | 'quick' | 'single_doc';
  target_space_ids?: number[];
  target_doc_id?: number;
  status: 'pending' | 'running' | 'paused' | 'completed' | 'failed';
  progress: number;
  message: string;
  total_docs: number;
  processed_docs: number;
  skipped_docs?: number;
  token_usage: number;
  created_at: number;
  started_at: number;
  completed_at: number;
  result?: AnalysisResult;
}

export interface AnalysisResult {
  total_docs: number;
  processed_docs: number;
  categories: AnalysisCategory[];
  summaries: AnalysisDocSummary[];
  relations: AnalysisRelation[];
  themes: AnalysisTheme[];
  generated_at: number;
}

export interface AnalysisCategory {
  name: string;
  description: string;
  doc_ids: number[];
  space_ids?: number[];
}

export interface AnalysisDocSummary {
  source_doc_id: number;
  source_space_id: number;
  source_name: string;
  space_name: string;
  category: string;
  summary: string;
  key_points: string[];
  tags: string[];
  confidence: 'high' | 'medium' | 'low';
  review_status?: 'pending' | 'accepted' | 'rejected' | 'manual_edit';
}

export interface AnalysisRelation {
  source_doc_id: number;
  target_doc_id: number;
  relation_type: string;
  description: string;
  confidence?: 'high' | 'medium' | 'low';
}

export interface AnalysisTheme {
  theme: string;
  summary: string;
  related_doc_ids: number[];
}

export interface StartAnalysisRequest {
  type?: 'full' | 'incremental' | 'space_only' | 'quick' | 'single_doc';
  target_space_ids?: number[];
  target_doc_id?: number;
  date_from?: string;
  date_to?: string;
}

export interface ChatMessage {
  role: 'user' | 'assistant';
  content: string;
}

export interface AIConversation {
  id: number;
  user_id: number;
  space_id: number;
  type: string;
  title: string;
  messages: { role: string; content: string }[];
  localMessages?: { role: string; content: string; sql?: string; explanations?: string[]; tables?: string[]; warnings?: string[] }[];
  created_at: string;
  updated_at: string;
}

export interface AIAskRequest {
  question: string;
  space_id?: number;
  limit?: number;
  history?: ChatMessage[];
  // mode: "rag"（默认检索增强）或 "agent"（多智能体编排+反思闭环）
  mode?: string;
}

export interface AIAskResponse {
  answer: string;
  sources: AIAskSource[];
}

export interface AIAskSource {
  document_id: number;
  document_name: string;
  space_id: number;
  score: number;
  snippet: string;
  start_line: number;
  end_line: number;
  heading_path: string;
  keywords: string[];
  citation_index?: number;
  updated_at?: number;
  paragraph_id?: number;
}

// RAG 检索过程追踪类型
export interface RAGRetrievalStep {
  step: 'query_rewrite' | 'vector_search' | 'keyword_search' | 'rerank' | 'filter' | 'final';
  name: string;
  description: string;
  duration_ms: number;
  result_count?: number;
  details?: Record<string, unknown>;
}

export interface RAGDebugInfo {
  original_question: string;
  rewritten_question?: string;
  sub_questions?: string[];
  retrieval_steps: RAGRetrievalStep[];
  vector_results?: Array<{ doc_id: number; score: number; name: string }>;
  keyword_results?: Array<{ doc_id: number; score: number; name: string }>;
  combined_results?: Array<{ doc_id: number; score: number; name: string; vector_score: number; keyword_score: number }>;
  final_sources?: AIAskSource[];
  cache_hit?: boolean;
  total_duration_ms: number;
}

// RAG 配置类型
export interface RAGConfig {
  vector_weight: number;
  keyword_weight: number;
  similarity_threshold: number;
  search_top_k: number;
  max_context_chars: number;
  enable_rerank: boolean;
  enable_query_rewrite: boolean;
  enable_sub_question: boolean;
  rerank_endpoint?: string;
  rerank_api_key?: string;
}

// RAG 评估指标
export interface RAGEvalMetrics {
  faithfulness?: number;
  answer_relevance?: number;
  context_precision?: number;
  context_recall?: number;
  answer_correctness?: number;
}

export type AIWriteAction = 
  | 'rewrite' 
  | 'continue' 
  | 'translate' 
  | 'summarize' 
  | 'explain' 
  | 'shorten' 
  | 'table'
  | 'report'
  | 'formalize'
  | 'simplify'
  | 'bulletize'
  | 'expand'
  | 'check_quality'
  | 'add_comments'
  | 'fix_grammar';

export interface AIWriteRequest {
  text: string;
  action: AIWriteAction;
  instruction?: string;
}

export interface UpdateAIConfigRequest {
  provider_type?: string;
  base_url?: string;
  api_key?: string;
  model?: string;
  fast_model?: string;
  reasoning_model?: string;
  temperature?: number;
  max_tokens?: number;
}

export interface PromptTemplate {
  id: number;
  name: string;
  type: string;
  query_type: string;
  system_prompt: string;
  user_template: string;
  description: string;
  version?: string;
  variables?: string[];
  is_active: boolean;
  is_default: boolean;
  sort: number;
  space_id: number | null;
  created_at: string;
  updated_at: string;
}

export interface KnowledgeGraphData {
  nodes: { id: string; label: string; type: string; group: string }[];
  edges: { source: string; target: string; label: string; confidence: number }[];
}

export interface WorkflowNode {
  id: string;
  type: string;
  name: string;
  config?: Record<string, unknown>;
}

export interface WorkflowEdge {
  source: string;
  target: string;
  condition?: string;
}

// 持久化的工作流定义见 api/modules/ai/ai-data.ts 的 WorkflowDefinition（此处不再重复声明）

// AI 审计日志类型
export interface AIAuditLog {
  id: number;
  user_id: number;
  action: string;
  target_id?: string;
  question?: string;
  answer?: string;
  detail?: string;
  token_usage?: number;
  source_count?: number;
  model?: string;
  provider?: string;
  result?: string;
  reason?: string;
  ip_address?: string;
  duration_ms?: number;
  created_at: string;
}

// 表格生成
export interface AITableRequest {
  description: string;
  source_text?: string;
  table_type?: 'comparison' | 'data' | 'matrix' | 'timeline';
  max_rows?: number;
  max_columns?: number;
}

export interface AITableResult {
  markdown: string;
  headers: string[];
  rows: string[][];
  chart_config?: string;
}

// 汇报生成
export interface AIReportRequest {
  type: 'weekly' | 'monthly' | 'tech' | 'meeting' | 'project' | 'custom';
  title: string;
  sources?: string[];
  extra?: string;
  sections?: string[];
  language?: string;
}

export interface AIReportSection {
  title: string;
  content: string;
}

export interface AIReportResult {
  title: string;
  content: string;
  sections: AIReportSection[];
}

// 邮件生成
export interface AIEmailRequest {
  type: 'notify' | 'share' | 'meeting' | 'weekly_digest' | 'custom';
  subject?: string;
  doc_content?: string;
  doc_title?: string;
  tone?: 'formal' | 'semi-formal' | 'casual';
  extra?: string;
}

export interface AIEmailResult {
  subject: string;
  body: string;
}

// 翻译
export interface AITranslateRequest {
  text: string;
  target_language: string;
  source_language?: string;
  preserve_format?: boolean;
  glossary?: Record<string, string>;
}

export interface AITranslateResult {
  translated_text: string;
  source_language: string;
  target_language: string;
}

// 推荐
export interface AIRecommendation {
  document_id: number;
  document_name: string;
  space_id: number;
  space_name: string;
  score: number;
  reason: string;
}

// Analyzer 配置
export interface AnalyzerConfig {
  batch_size: number;
  max_concurrent: number;
  rate_limit_rpm: number;
  doc_max_chars: number;
  context_max_chars: number;
}

// 质量巡检
export interface AIPatrolResult {
  total_docs: number;
  checked_docs: number;
  issue_count: number;
  health_score: number;
  issues: AIPatrolIssue[];
  suggestions: string[];
}

export interface AIPatrolIssue {
  document_id: number;
  document_name: string;
  issue_type: string;
  description: string;
  severity: string;
}
