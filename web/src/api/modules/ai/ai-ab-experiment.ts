import client from '@/api/client'

export interface ABExperiment {
  id: number
  space_id: number
  name: string
  experiment_id: string
  description: string
  traffic_a: number
  traffic_b: number
  metric_a: number
  metric_b: number
  status: number
  created_at: string
  updated_at: string
}

export interface ABExperimentRequest {
  space_id: number
  name: string
  experiment_id: string
  description: string
  traffic_a: number
  traffic_b: number
  status: number
}

export const ABExperimentAPI = {
  list(params: { space_id: number }) {
    return client.get<ABExperiment[]>('/ai/data-query/ab-experiments', { params })
  },
  get(id: number) {
    return client.get<ABExperiment>(`/ai/data-query/ab-experiments/${id}`)
  },
  create(data: ABExperimentRequest) {
    return client.post<ABExperiment>('/ai/data-query/ab-experiments', data)
  },
  update(id: number, data: ABExperimentRequest) {
    return client.put<ABExperiment>(`/ai/data-query/ab-experiments/${id}`, data)
  },
  delete(id: number) {
    return client.delete(`/ai/data-query/ab-experiments/${id}`)
  },
}
