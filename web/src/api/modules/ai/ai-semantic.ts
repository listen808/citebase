import { get, post, put, del } from '../../client';

export interface SemanticMetric {
  id?: number
  space_id: number
  name: string
  code: string
  description?: string
  formula: string
  default_table: string
  dimensions?: string
  time_field?: string
  time_granularity?: string
  created_at?: string
  updated_at?: string
}

export interface SemanticDimension {
  id?: number
  space_id: number
  name: string
  code: string
  field: string
  table: string
  value_type?: string
  description?: string
  created_at?: string
  updated_at?: string
}

// P1-5: 后端 ListMetrics/ListDimensions 返回 gin.H{"list","total"}（非 PaginatedResponse 的 data 键），
// CreateMetric/UpdateMetric/CreateDimension 直接返回裸对象（无 data 包装），此前类型与真实信封不符导致列表恒为空。
export const SemanticAPI = {
  listMetrics(params: { space_id: number; page?: number; page_size?: number }) {
    return get<{ list: SemanticMetric[]; total: number }>('/ai/semantic-metrics', { params });
  },
  createMetric(data: SemanticMetric) {
    return post<SemanticMetric>('/ai/semantic-metrics', data);
  },
  updateMetric(id: number, data: SemanticMetric) {
    return put<SemanticMetric>(`/ai/semantic-metrics/${id}`, data);
  },
  deleteMetric(id: number) {
    return del<void>(`/ai/semantic-metrics/${id}`);
  },
  listDimensions(params: { space_id: number; page?: number; page_size?: number }) {
    return get<{ list: SemanticDimension[]; total: number }>('/ai/semantic-dimensions', { params });
  },
  createDimension(data: SemanticDimension) {
    return post<SemanticDimension>('/ai/semantic-dimensions', data);
  },
  deleteDimension(id: number) {
    return del<void>(`/ai/semantic-dimensions/${id}`);
  },
};
