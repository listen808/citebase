import { post } from '../../client'

export interface SearchSource {
  id?: number
  title?: string
  content?: string
  score?: number
}

export interface SearchOverviewRequest {
  query: string
  results?: SearchSource[]
  space_id?: number
}

export interface SearchOverviewResponse {
  overview: {
    overview: string
    [key: string]: unknown
  }
}

export async function searchOverview(data: SearchOverviewRequest): Promise<SearchOverviewResponse> {
  return post('/ai/search/overview', data)
}
