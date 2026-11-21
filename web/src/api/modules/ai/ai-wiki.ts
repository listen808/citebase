import { get, post, del } from '../../client'

// LLM Wiki 知识编译与治理接口。
// 所有接口都要求 space_id：编译产物按空间隔离。

export interface WikiPage {
  id: number
  space_id: number
  slug: string
  type: string
  title: string
  summary: string
  content: string
  sources: number[]
  status: 'planned' | 'draft' | 'published' | 'orphan'
  version: number
  base_version: number
  source_count: number
  provenance: 'llm' | 'human'
  confidence: number
  superseded_by: string
  compiled_at?: string
  created_at: string
  updated_at: string
}

export interface WikiLink {
  id: number
  space_id: number
  from_slug: string
  to_slug: string
  link_type: string
  weight: number
}

export interface WikiIndexEntry {
  slug: string
  type: string
  title: string
  summary: string
  source_count: number
}

export interface WikiHit {
  slug: string
  title: string
  summary: string
  content: string
  score: number
  via: 'directory' | 'fulltext' | 'both'
}

export interface WikiRelated {
  slug: string
  title: string
  type: string
  score: number
  reason: string
}

export interface WikiInsight {
  kind: 'surprising' | 'gap' | 'bridge'
  slugs: string[]
  score: number
  detail: string
  action: string
}

export interface WikiLintIssue {
  id: number
  space_id: number
  page_slug: string
  category: string
  severity: string
  detail: string
  status: string
}

export interface WikiPin {
  id: number
  space_id: number
  page_slug: string
  kind: string
  claim: string
  anchor: string
  provenance: string
  status: string
  created_by: number
}

export interface WikiReview {
  id: number
  space_id: number
  page_slug: string
  kind: string
  action: string
  reason: string
  base_version: number
  status: string
}

export interface WikiPinCheck {
  pin_id: number
  claim: string
  status: string
  note: string
}

export interface CompileResult {
  space_id: number
  created: string[]
  updated: string[]
  skipped: number
}

export interface WikiSection {
  title: string
  key_points: string[]
  content: string
}

export interface CitationIssue {
  section: string
  kind: 'missing' | 'out_of_range'
  detail: string
}

export interface ArticleResult {
  topic: string
  sections: WikiSection[]
  markdown: string
  source_count: number
  citation_issues: CitationIssue[]
}

interface PageListResponse {
  pages: WikiPage[]
  total: number
}

export function compileWiki(data: {
  space_id: number
  doc_ids: number[]
  page_type?: string
  concurrency?: number
}) {
  return post<CompileResult>('/ai/wiki/compile', data)
}

export function listWikiPages(spaceId: number, status = '') {
  return get<PageListResponse>('/ai/wiki/pages', { params: { space_id: spaceId, status } })
}

export function getWikiPage(spaceId: number, slug: string) {
  return get<{ page: WikiPage }>(`/ai/wiki/pages/${encodeURIComponent(slug)}`, {
    params: { space_id: spaceId },
  })
}

export function publishWikiPage(spaceId: number, slug: string, baseVersion: number) {
  return post<{ page: WikiPage }>(
    `/ai/wiki/pages/${encodeURIComponent(slug)}/publish`,
    { base_version: baseVersion },
    { params: { space_id: spaceId } },
  )
}

export function markWikiPageHumanEdited(spaceId: number, slug: string) {
  return post<{ slug: string; provenance: string }>(
    `/ai/wiki/pages/${encodeURIComponent(slug)}/human-edit`,
    {},
    { params: { space_id: spaceId } },
  )
}

export function deleteWikiPage(spaceId: number, slug: string) {
  return del<unknown>(`/ai/wiki/pages/${encodeURIComponent(slug)}`, { params: { space_id: spaceId } })
}

export function supersedeWikiPage(spaceId: number, slug: string, supersededBy: string) {
  return post<{ slug: string; superseded_by: string }>(
    `/ai/wiki/pages/${encodeURIComponent(slug)}/supersede`,
    { superseded_by: supersededBy },
    { params: { space_id: spaceId } },
  )
}

export function searchWiki(spaceId: number, question: string, fulltextTopK = 2) {
  return post<{ hits: WikiHit[]; total: number }>(
    '/ai/wiki/search',
    { question, fulltext_top_k: fulltextTopK },
    { params: { space_id: spaceId } },
  )
}

export function getWikiLinks(spaceId: number) {
  return get<{ links: WikiLink[]; total: number }>('/ai/wiki/links', { params: { space_id: spaceId } })
}

export function getWikiRelated(spaceId: number, slug: string, limit = 10) {
  return get<{ related: WikiRelated[]; total: number }>(`/ai/wiki/related/${encodeURIComponent(slug)}`, {
    params: { space_id: spaceId, limit },
  })
}

export function generateArticle(data: { topic: string; doc_ids?: number[]; skip_polish?: boolean }) {
  return post<ArticleResult>('/ai/wiki/article/generate', data)
}

export function createWikiPin(data: {
  space_id: number
  page_slug: string
  claim: string
  kind?: string
  anchor?: string
}) {
  return post<{ pin: WikiPin }>('/ai/wiki/pins', data)
}

export function listWikiPins(spaceId: number, status = '') {
  return get<{ pins: WikiPin[]; total: number }>('/ai/wiki/pins', { params: { space_id: spaceId, status } })
}

export function resolveWikiPin(id: number, status = 'resolved') {
  return post<{ id: number; status: string }>(`/ai/wiki/pins/${id}/resolve`, {}, { params: { status } })
}

export function checkWikiPins(spaceId: number, slug: string) {
  return post<{ checks: WikiPinCheck[]; total: number }>(
    `/ai/wiki/pages/${encodeURIComponent(slug)}/check-pins`,
    {},
    { params: { space_id: spaceId } },
  )
}

export function listWikiReviews(spaceId: number, status = '') {
  return get<{ reviews: WikiReview[]; total: number }>('/ai/wiki/reviews', {
    params: { space_id: spaceId, status },
  })
}

export function resolveWikiReview(id: number, action = 'accepted') {
  return post<{ id: number; status: string }>(`/ai/wiki/reviews/${id}/resolve`, { action })
}

export function runWikiLint(spaceId: number, staleDays = 90) {
  return post<{ issues: WikiLintIssue[]; total: number }>(
    '/ai/wiki/lint',
    { space_id: spaceId, stale_days: staleDays },
    { params: { space_id: spaceId } },
  )
}

export function listWikiLint(spaceId: number, status = 'open') {
  return get<{ issues: WikiLintIssue[]; total: number }>('/ai/wiki/lint', {
    params: { space_id: spaceId, status },
  })
}

export function getWikiInsights(spaceId: number) {
  return get<{ insights: WikiInsight[]; total: number }>('/ai/wiki/insights', {
    params: { space_id: spaceId },
  })
}

export interface WikiPageHistory {
  id: number
  space_id: number
  slug: string
  version: number
  title: string
  summary: string
  content: string
  sources: number[]
  provenance: string
  content_hash: string
  created_at: string
}

export function listWikiPageHistory(spaceId: number, slug: string) {
  return get<{ slug: string; history: WikiPageHistory[]; total: number }>(
    `/ai/wiki/pages/${encodeURIComponent(slug)}/history`,
    { params: { space_id: spaceId } },
  )
}

export function rollbackWikiPage(spaceId: number, slug: string, version: number) {
  return post<{ page: WikiPage }>(
    `/ai/wiki/pages/${encodeURIComponent(slug)}/rollback`,
    { version },
    { params: { space_id: spaceId } },
  )
}

export function markWikiContradictions(spaceId: number, slug: string, items: string[]) {
  return post<{ slug: string; marked: number }>(
    `/ai/wiki/pages/${encodeURIComponent(slug)}/contradictions`,
    { items },
    { params: { space_id: spaceId } },
  )
}

export function proposeWikiAnalysis(data: {
  space_id: number
  title: string
  question?: string
  answer: string
}) {
  return post<{ page: WikiPage }>('/ai/wiki/propose-analysis', data)
}

// OKF/Markdown 导出：zip（全部已发布页 + links.md）或 md（单页）
export function exportWiki(spaceId: number, format: 'zip' | 'md' = 'zip', slug?: string) {
  return get<Blob>('/ai/wiki/export', {
    params: { space_id: spaceId, format, slug },
    responseType: 'blob',
  })
}
