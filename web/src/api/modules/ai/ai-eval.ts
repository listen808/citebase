import { get, post, del } from '../../client';

/** 后端 /run、/testcases/run、/ab-compare 是同步长任务（ctx 10~20 分钟），
 *  client 默认 30s 会先超时，这三个请求单独放宽超时。 */
const LONG_RUN_TIMEOUT = 21 * 60 * 1000;

/**
 * AI 评估控制台：RAGAS 数据集评估 + 端到端 TestCase + A/B 对比
 * 路由：internal/api/ai/routes.go:271-283（handler internal/api/ai/eval.go EvalAPI）
 * 字段对齐 internal/ai/eval/dataset.go 与 internal/ai/eval/enhanced/evaluator.go
 *
 * 与 /ai/ragas/*（见 ai-ragas.ts）是两套独立接口：
 *   - /ai/ragas 走 rag.RAGASEvaluator，指标四项，落库 aw_ai_ragas_eval；
 *   - /ai/eval 走 eval.Evaluator，指标五项（多 answer_correctness），数据集与结果落库
 *     aw_eval_dataset / aw_eval_result。
 *
 * 后端行为注意：
 *   - TestCase 只存在 EvalAPI 的进程内 map（无落库、无 DELETE 接口），重启即清空；
 *     同 id 重新 POST 会覆盖，编辑即复用创建接口。
 *   - EvalQuestion.difficulty 会被持久化但 eval.Evaluator 全程不参与打分。
 *   - /run、/testcases/run、/ab-compare 都是同步长任务（后端 ctx 10~20 分钟），
 *     本模块已为这三个请求单独放宽 timeout（见 LONG_RUN_TIMEOUT）。
 */

/** internal/ai/eval/dataset.go EvalQuestion */
export interface EvalQuestion {
  question: string;
  expected_answer: string;
  /** 上下文精确率/召回率的基准，留空则两项记 0 */
  expected_context: string[] | null;
  /** easy/medium/hard，后端仅存储不使用 */
  difficulty: string;
}

/**
 * internal/ai/eval/dataset.go RAGASMetrics（五项，取值 0~1）。
 * 注意与 ai-ragas.ts 的同名类型不同：这里是 answer_relevance + answer_correctness。
 */
export interface EvalRAGASMetrics {
  faithfulness: number;
  answer_relevance: number;
  context_precision: number;
  context_recall: number;
  answer_correctness: number;
  retrieval_hit_rate?: number;
  /** W4 四段式指标（无标注时为 0） */
  routing_accuracy?: number;
  retrieval_hit_at_k?: number;
  rerank_ndcg?: number;
  noise_robustness?: number;
}

/** 三层归因聚合（检索/增强/生成），后端 LayerScoresFrom 计算 */
export interface EvalLayerScores {
  retrieval: number;
  enrich: number;
  generation: number;
  /** W4 扩展层 */
  routing?: number;
  rerank?: number;
  noise_res?: number;
}

/** 从指标算三层归因（前端兜底，后端已聚合则直接取） */
export function evalLayerScores(m?: EvalRAGASMetrics | null): EvalLayerScores | null {
  if (!m) return null;
  const avg = (vals: number[]) => (vals.length ? vals.reduce((a, b) => a + b, 0) / vals.length : 0);
  return {
    retrieval: m.retrieval_hit_rate ?? 0,
    enrich: avg([m.context_precision, m.context_recall]),
    generation: avg([m.faithfulness, m.answer_relevance, m.answer_correctness]),
    routing: m.routing_accuracy ?? 0,
    rerank: m.rerank_ndcg ?? 0,
    noise_res: m.noise_robustness ?? 0,
  };
}

/** internal/ai/eval/dataset.go QuestionEval */
export interface EvalQuestionDetail {
  question: string;
  generated_answer: string;
  retrieved_context: string[] | null;
  expected_answer: string;
  expected_context: string[] | null;
  metrics: EvalRAGASMetrics;
}

/** internal/ai/eval/dataset.go DatasetWithQuestions */
export interface EvalDataset {
  id: number;
  name: string;
  space_id: number;
  questions: EvalQuestion[] | null;
  created_at: string;
  updated_at: string;
}

/** internal/ai/eval/dataset.go ResultWithDetails（metrics/details 为 omitempty，可能缺省） */
export interface EvalResult {
  id: number;
  dataset_id: number;
  dataset_name: string;
  space_id: number;
  /** 五项指标均值，0~1 */
  overall_score: number;
  metrics?: EvalRAGASMetrics;
  details?: EvalQuestionDetail[];
  /** Go time.Duration 字符串，如 "1m12.345s" */
  duration: string;
  status: string;
  created_at: string;
}

/** internal/ai/eval/enhanced/evaluator.go ExpectedOutcome，全部字段 omitempty */
export interface ExpectedOutcome {
  contains_any?: string[];
  contains_all?: string[];
  expected_doc_ids?: number[];
  expected_tools?: string[];
  expected_tables?: string[];
  not_contains?: string[];
  max_latency_ms?: number;
}

/** internal/ai/eval/enhanced/evaluator.go TestCase */
export interface EvalTestCase {
  id: string;
  name: string;
  question: string;
  space_id: number;
  expected: ExpectedOutcome;
  tags?: string[];
}

/** internal/ai/eval/enhanced/evaluator.go TestFailure */
export interface EvalTestFailure {
  type: string;
  expected: string;
  actual: string;
}

/** internal/ai/eval/enhanced/evaluator.go TestResult */
export interface EvalTestResult {
  test_case_id: string;
  passed: boolean;
  actual_answer: string;
  actual_sources?: { document_id: number }[];
  actual_tools?: string[];
  actual_tables?: string[];
  duration_ms: number;
  failures?: EvalTestFailure[];
}

/** POST /ai/eval/testcases/run 响应 */
export interface EvalSuiteRun {
  results: EvalTestResult[];
  total: number;
  passed: number;
  failed: number;
  /** 0~1 */
  pass_rate: number;
  /** 所有用例耗时之和（毫秒） */
  total_latency: number;
}

/** internal/ai/eval/enhanced/evaluator.go AgentVariant / VariantConfig */
export interface EvalAgentVariant {
  id: string;
  name: string;
  config: {
    provider?: string;
    model?: string;
    temperature?: number;
    top_k?: number;
  };
}

/** internal/ai/eval/enhanced/evaluator.go ABResult，variants 以 AgentVariant.id 为键 */
export interface EvalABResult {
  test_case_id: string;
  variants: Record<string, EvalTestResult>;
  /** 通过且延迟最低的变体 id，全失败时为空串 */
  winner: string;
  summary: string;
}

/** POST /ai/eval/datasets 请求体（name 与 questions 后端 binding:required，至少一题） */
export interface CreateEvalDatasetRequest {
  name: string;
  space_id: number;
  questions: EvalQuestion[];
}

export const EvalAPI = {
  datasets(params: { limit?: number; offset?: number } = {}) {
    return get<{ datasets: EvalDataset[]; total: number }>('/ai/eval/datasets', { params });
  },

  dataset(id: number) {
    return get<{ dataset: EvalDataset }>(`/ai/eval/datasets/${id}`);
  },

  createDataset(data: CreateEvalDatasetRequest) {
    return post<{ dataset: EvalDataset }>('/ai/eval/datasets', data);
  },

  deleteDataset(id: number) {
    return del<{ deleted: boolean }>(`/ai/eval/datasets/${id}`);
  },

  /** 同步跑完整数据集，逐题多次调用 LLM，耗时远超 client timeout */
  run(data: { dataset_id: number }) {
    // 后端逐题调用 LLM，同步返回，实测远超 client 默认 30s
    return post<{ result: EvalResult }>('/ai/eval/run', data, { timeout: LONG_RUN_TIMEOUT });
  },

  /** 列表接口与详情接口共用 fromDBModel，每行都会带上完整 details（longtext），条数多时响应很大 */
  results(params: { limit?: number; offset?: number } = {}) {
    return get<{ results: EvalResult[]; total: number }>('/ai/eval/results', { params });
  },

  result(id: number) {
    return get<{ result: EvalResult }>(`/ai/eval/results/${id}`);
  },

  deleteResult(id: number) {
    return del<{ deleted: boolean }>(`/ai/eval/results/${id}`);
  },

  testCases() {
    return get<{ test_cases: EvalTestCase[]; total: number }>('/ai/eval/testcases');
  },

  /** 创建即 upsert：同 id 再 POST 会覆盖进程内旧用例 */
  createTestCase(data: EvalTestCase) {
    return post<{ test_case: EvalTestCase }>('/ai/eval/testcases', data);
  },

  /** test_case_ids 留空则跑全部用例 */
  runTestCases(data: { test_case_ids?: string[] } = {}) {
    return post<EvalSuiteRun>('/ai/eval/testcases/run', data, { timeout: LONG_RUN_TIMEOUT });
  },

  /** variants 至少 2 个，否则后端 400 */
  abCompare(data: { test_case_ids?: string[]; variants: EvalAgentVariant[] }) {
    return post<{ results: EvalABResult[]; total: number }>('/ai/eval/ab-compare', data, { timeout: LONG_RUN_TIMEOUT });
  },
};
