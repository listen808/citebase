/**
 * Markdown 渲染产物的统一 DOMPurify 消毒层。
 *
 * 此前三处渲染管线各自维护白名单且已漂移（ChatMessageBubble 允许 button/details、
 * SearchView 轻量管线与 SpaceView 打印导出各一份更窄的），XSS 面不统一。
 * 这里收敛为唯一事实源：白名单取三处的并集，新增渲染管线一律走 sanitizeMarkdownHtml，
 * 需要额外标签时用 extraTags 显式扩展并写明理由。
 */
import DOMPurify from 'dompurify';

export const SANITIZE_ALLOWED_TAGS: string[] = [
  'p', 'br', 'hr', 'div', 'span', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
  'ul', 'ol', 'li', 'blockquote', 'pre', 'code', 'strong', 'em', 'a', 'img',
  'table', 'thead', 'tbody', 'tfoot', 'tr', 'th', 'td', 'caption', 'col', 'colgroup',
  'sup', 'sub', 'del', 'ins', 'kbd', 's', 'b', 'i', 'u', 'mark', 'small', 'dl', 'dt', 'dd',
  'figure', 'figcaption',
  'details', 'summary',
  'button', // 代码块复制按钮（useMarkdownRenderer 注入）
];

export const SANITIZE_ALLOWED_ATTR: string[] = [
  'href', 'title', 'src', 'alt', 'id', 'class', 'data-lang', 'data-citation-idx', 'data-num',
  'target', 'rel', 'width', 'height', 'type',
  'colspan', 'rowspan', 'align', 'valign', 'scope', 'headers',
  'abbr', 'axis', 'bgcolor', 'border', 'cellpadding', 'cellspacing',
  'frame', 'rules', 'summary',
  'style',
  'open',
];

export const SANITIZE_ALLOWED_STYLES: string[] = [
  'width', 'height', 'max-width', 'border', 'border-collapse', 'border-spacing',
  'text-align', 'vertical-align', 'background-color', 'color',
  'font-weight', 'font-style', 'padding', 'margin',
];

/** 消毒 Markdown 渲染出的 HTML。extraTags：极少数管线需要的额外标签（如 katex 输出）。 */
export function sanitizeMarkdownHtml(html: string, extraTags?: string[]): string {
  return DOMPurify.sanitize(html, {
    ALLOWED_TAGS: extraTags?.length ? [...SANITIZE_ALLOWED_TAGS, ...extraTags] : SANITIZE_ALLOWED_TAGS,
    ALLOWED_ATTR: SANITIZE_ALLOWED_ATTR,
    // @ts-expect-error ALLOWED_STYLES exists at runtime but not in DOMPurify types
    ALLOWED_STYLES: SANITIZE_ALLOWED_STYLES,
  });
}
