import { get } from '../../client'

export interface EmbedCodeResponse {
  js_embed: string
  iframe_embed: string
  base_url: string
}

export async function getEmbedCode(): Promise<EmbedCodeResponse> {
  return get('/ai/embed-code')
}

export function getWidgetUrl(): string {
  // 预览 iframe 必须走已代理的 /api/v1 前缀：dev 里 Vite 只代理 /api/v1、/images、/attachments，
  // 裸 /ai/widget 不代理 → 命中 SPA fallback → 在 sandbox(opaque origin) 里整站重启 → main.ts 跨源被 CORS 拦。
  // /api/v1/ai/widget 在 dev（经代理）与生产（后端同名路由，实测 200）都能取到自包含的 widget HTML。
  return '/api/v1/ai/widget'
}
