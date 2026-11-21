import { get, post, put, del } from '../../client';

/**
 * 字段级数据敏感与脱敏规则 + PII 检测
 * 路由：internal/api/ai/routes.go:233-237（handler internal/api/ai/data_sensitivity.go）
 * 字段对齐 internal/model/ai/data_sensitivity.go DataSensitivityRule
 *      internal/service/ai/data_sensitivity_service.go CreateSensitivityRuleRequest / DetectPIIResult
 * 注意：GET 的 space_id 为必填（ParseUint 失败即 400 "space_id 无效"），无分页、无默认值。
 */

export type SensitivityLevel = 'low' | 'medium' | 'high' | 'critical';

/** full / partial / hash / regex（后端仅存字符串，未做枚举校验） */
export type SensitivityMaskType = 'full' | 'partial' | 'hash' | 'regex';

export interface DataSensitivityRule {
  id: number;
  space_id: number;
  /** JSON 键为 table_name / field_name，模型字段名是 Table / Field */
  table_name: string;
  field_name: string;
  level: SensitivityLevel;
  mask_type: SensitivityMaskType | '';
  mask_rule: string;
  description: string;
  created_at: string;
  updated_at: string;
}

/** 创建与更新共用同一请求体（PUT 时 space_id 被后端忽略，不会改归属空间） */
export interface DataSensitivityRuleRequest {
  space_id: number;
  table_name: string;
  field_name: string;
  level: SensitivityLevel;
  mask_type?: string;
  mask_rule?: string;
  description?: string;
}

/** 命中项格式为 "类型:掩码后片段"，如 mobile:13****78 */
export interface DetectPIIResult {
  pii_found: string[] | null;
  count: number;
}

export const DataSensitivityAPI = {
  listRules(spaceId: number) {
    return get<DataSensitivityRule[] | null>('/ai/data-sensitivity', { params: { space_id: spaceId } });
  },

  createRule(data: DataSensitivityRuleRequest) {
    return post<DataSensitivityRule>('/ai/data-sensitivity', data);
  },

  updateRule(id: number, data: DataSensitivityRuleRequest) {
    return put<DataSensitivityRule>(`/ai/data-sensitivity/${id}`, data);
  },

  deleteRule(id: number) {
    return del<{ deleted: boolean }>(`/ai/data-sensitivity/${id}`);
  },

  /** 正则匹配手机号 / 身份证 / 邮箱 / 银行卡，与规则表无关（不读 space_id） */
  detectPII(text: string) {
    return post<DetectPIIResult>('/ai/detect-pii', { text });
  },
};
