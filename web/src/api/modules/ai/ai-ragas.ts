import { get, post, del } from '../../client';

/**
 * RAGAS 检索质量评估
 * 路由：internal/api/ai/routes.go:140-143（handler internal/api/ai/config.go RAGASEvaluate/RAGASResults/RAGASStats/RAGASDeleteResult）
 * 字段对齐 internal/ai/rag/ragas_evaluator.go RAGASEvalResult / RAGASMetrics / Stats()
 * 注意：列表与统计读的是评估器的内存 map（进程重启即清空），
 *      aw_ai_ragas_eval 表只写不读；DELETE 只删内存条目，不落库。
 */

/** internal/ai/rag/ragas_evaluator.go RAGASMetrics，四项取值均为 0~1 */
export interface RAGASMetrics {
  faithfulness: number;
  answer_relevancy: number;
  context_precision: number;
  context_recall: number;
}

export interface RAGASEvalResult {
  /** 内存自增 ID，与 aw_ai_ragas_eval.id 无关 */
  id: number;
  question: string;
  answer: string;
  /** 后端 Evaluate() 从不写入，恒为空串 */
  expected_answer: string;
  contexts: string[] | null;
  ground_truth: string;
  metrics: RAGASMetrics;
  avg_score: number;
  /** Go time.Duration 字符串，如 "3.412s" */
  duration: string;
  created_at: string;
}

/**
 * RAGASEvaluator.Stats()（gin.H 等价 map）。
 * total 为 0 时后端只返回 {"total":0}，其余均值键缺省；
 * avg_context_recall 仅当存在带 ground_truth 的样本时才返回。
 */
export interface RAGASStats {
  total: number;
  avg_faithfulness?: number;
  avg_answer_relevancy?: number;
  avg_context_precision?: number;
  avg_context_recall?: number;
}

/** POST /ai/ragas/evaluate 请求体（question/answer 必填，contexts 与 ground_truth 可空） */
export interface RAGASEvaluateRequest {
  question: string;
  answer: string;
  contexts?: string[];
  ground_truth?: string;
}

export const RAGASAPI = {
  stats() {
    return get<RAGASStats>('/ai/ragas/stats');
  },

  /** offset 超出条数时后端返回 results: null */
  results(params: { limit?: number; offset?: number } = {}) {
    return get<{ results: RAGASEvalResult[] | null }>('/ai/ragas/results', { params });
  },

  /** 同步执行多次 LLM 调用，耗时可达数十秒（client timeout 30s） */
  evaluate(data: RAGASEvaluateRequest) {
    return post<{ result: RAGASEvalResult }>('/ai/ragas/evaluate', data);
  },

  deleteResult(id: number) {
    return del<{ deleted: boolean }>(`/ai/ragas/results/${id}`);
  },
};
