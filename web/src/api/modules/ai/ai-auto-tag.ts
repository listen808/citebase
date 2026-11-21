import { post } from '../../client'

export interface AutoTagRequest {
  content: string
}

// P1-22: 后端返回 gin.H{"tags": TagResult}，TagResult 内含 tags/category/confidence，
// 此前扁平类型与真实结构不符，AIAutoTagView 在对象上 v-for 渲染出错乱 chip。
// 与 ai-knowledge.ts 的 autoTag（DocProperties 已在用）保持同一结构。
export interface AutoTagResult {
  tags: string[]
  category: string
  confidence: number
}

export interface AutoTagResponse {
  tags: AutoTagResult
}

export async function autoTag(data: AutoTagRequest): Promise<AutoTagResponse> {
  return post('/ai/auto-tag', data)
}
