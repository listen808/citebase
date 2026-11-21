import { get } from '../../client';

/**
 * AI 能力成熟度 + AI 模块健康检查
 * 路由：internal/api/ai/routes.go:148（GET /ai/features/maturity，handler internal/api/ai/config.go:886）
 *      internal/api/ai/routes.go:29 （GET /ai/health，handler internal/api/ai/health.go:10）
 * 字段对齐 internal/ai/maturity.go
 *
 * 后端行为注意：
 *   - FeatureMaturity 已补 json tag，与 tools 数组一致输出小写 name / category / maturity / description。
 *   - min 只会过滤，非法值（不在 experimental/beta/stable 中）等价于不过滤。
 *   - 过滤后为空时 Go 返回 nil slice，JSON 为 null，需按空数组兜底。
 *   - /ai/health 挂在 OptionalAuthMiddleware 上，未登录也可访问。
 */

export type MaturityLevel = 'experimental' | 'beta' | 'stable';

/** internal/ai/maturity.go FeatureMaturity */
export interface FeatureMaturity {
  name: string;
  category: string;
  maturity: string;
  description: string;
}

/** internal/ai/maturity.go ToolMaturity 打平成数组后的元素 */
export interface ToolMaturityEntry {
  name: string;
  maturity: string;
}

export interface FeatureMaturityReport {
  rag: FeatureMaturity[] | null;
  text2sql: FeatureMaturity[] | null;
  analyzer: FeatureMaturity[] | null;
  tools: ToolMaturityEntry[] | null;
  min_maturity: string;
}

/** internal/api/ai/health.go：enabled 与 primary_provider 任一为 false 即 DOWN，
 *  其余三项任一为 false 则 DEGRADED */
export interface AIHealthStatus {
  enabled: boolean;
  primary_provider: boolean;
  embedding_configured: boolean;
  retriever_ready: boolean;
  vector_store_ready: boolean;
  overall: string;
}

export const AICapabilityAPI = {
  maturity(min?: MaturityLevel) {
    return get<FeatureMaturityReport>('/ai/features/maturity', { params: min ? { min } : {} });
  },

  health() {
    return get<AIHealthStatus>('/ai/health');
  },
};
