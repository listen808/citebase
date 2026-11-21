import { post } from '../../client'

export interface ImportDocumentRequest {
  content: string
  format: string
}

/**
 * /ai/import（handler internal/api/ai/chat_admin.go:258）只做 AI 文本转换，
 * 返回 rag.ImportedDocument{title,content,format}——既不落库，也没有 id/space_id。
 * 真正写入空间要再调 POST /spaces/:id/import（见 document.importDocuments）。
 */
export interface ImportDocumentResponse {
  document: {
    title: string
    content: string
    format: string
  }
}

export async function importDocument(data: ImportDocumentRequest): Promise<ImportDocumentResponse> {
  return post('/ai/import', data)
}

/** pdf/docx/xlsx 必须由后端 connector 解析，浏览器直读会得到二进制乱码 */
const BINARY_EXT = ['pdf', 'docx', 'xlsx']

export async function importFile(file: File, spaceId?: number): Promise<ImportDocumentResponse> {
  // P1-24: 后端 /ai/import 只接受 JSON {content, format}（ShouldBindJSON），multipart 永远 400。
  const ext = (file.name.split('.').pop() || '').toLowerCase()
  if (BINARY_EXT.includes(ext)) {
    const parsed = await parseConnectorFile(file, spaceId)
    return { document: { title: parsed.filename, content: parsed.content, format: 'markdown' } }
  }
  const content = await file.text()
  const format = ['md', 'markdown', 'txt', 'text', 'json', 'html'].includes(ext) ? ext : 'text'
  return importDocument({ content, format })
}

// ============================================================
// 外部数据源连接器（internal/api/ai/datasource.go，路由 internal/api/ai/routes.go:266-268）
// 三个端点只做「解析」，不落库、不建文档：返回解析后的 Markdown 正文，
// 是否入库由调用方决定是否再走 /ai/import。
// space_id：scrape-* 走 JSON 字段、upload 走 PostForm 表单项；后端原样回显，不参与任何查询逻辑。
// ============================================================

/** 站点抓取要遍历多页（后端上限 50 页），默认 30s 的 axios 超时不够用 */
const CONNECTOR_TIMEOUT_MS = 120000

/** 单页抓取成功返回体；失败走 400（SSRF / 参数缺失）或 500（爬取异常） */
export interface ScrapeUrlResponse {
  url: string
  title: string
  content: string
  word_count: number
  /** 后端 links 为 []string，无链接时序列化为 null */
  links: string[] | null
  /** time.Time，RFC3339 字符串 */
  crawl_time: string
  space_id: number
}

/** 站点抓取的单页摘要（后端 pageSummary，字段比单页抓取少） */
export interface ScrapeSitePage {
  url: string
  title: string
  word_count: number
  content: string
}

export interface ScrapeSiteResponse {
  /** 一页都没抓到时为 null（后端 var pages []pageSummary 未初始化） */
  pages: ScrapeSitePage[] | null
  total: number
  space_id: number
}

/** 文件解析成功返回体；上限 10MB，扩展名须在 connector.SupportedExtensions 内 */
export interface ConnectorUploadResponse {
  filename: string
  content: string
  word_count: number
  space_id: number
}

export function scrapeUrl(url: string, spaceId?: number) {
  return post<ScrapeUrlResponse>(
    '/ai/datasources/scrape-url',
    { url, space_id: spaceId || 0 },
    { timeout: CONNECTOR_TIMEOUT_MS },
  )
}

export function scrapeSite(url: string, spaceId?: number, maxPages?: number) {
  return post<ScrapeSiteResponse>(
    '/ai/datasources/scrape-site',
    { url, space_id: spaceId || 0, max_pages: maxPages || 0 },
    { timeout: CONNECTOR_TIMEOUT_MS },
  )
}

export function parseConnectorFile(file: File, spaceId?: number) {
  // 与 attachments 上传一致：交给 client 请求拦截器清掉 Content-Type，由浏览器补 multipart boundary
  const form = new FormData()
  form.append('file', file)
  form.append('space_id', String(spaceId || 0))
  return post<ConnectorUploadResponse>('/ai/datasources/upload', form, { timeout: CONNECTOR_TIMEOUT_MS })
}
