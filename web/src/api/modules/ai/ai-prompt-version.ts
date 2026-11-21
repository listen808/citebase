import client from '@/api/client'

export interface PromptVersion {
  id: number
  space_id: number
  name: string
  query_type: string
  version: number
  system_prompt: string
  user_template: string
  active: boolean
  experiment_id?: string
  created_at: string
  updated_at: string
}

export interface PromptVersionRequest {
  space_id: number
  name: string
  query_type: string
  version: number
  system_prompt: string
  user_template: string
  active: boolean
  experiment_id?: string
}

export const PromptVersionAPI = {
  list(params: { space_id: number; page?: number; page_size?: number }) {
    return client.get<PromptVersion[]>('/ai/data-query/prompt-versions', { params })
  },
  create(data: PromptVersionRequest) {
    return client.post<PromptVersion>('/ai/data-query/prompt-versions', data)
  },
  update(id: number, data: PromptVersionRequest) {
    return client.put<PromptVersion>(`/ai/data-query/prompt-versions/${id}`, data)
  },
  delete(id: number) {
    return client.delete(`/ai/data-query/prompt-versions/${id}`)
  },
  activate(id: number) {
    return client.post<{ message: string }>(`/ai/data-query/prompt-versions/${id}/activate`)
  },
}
