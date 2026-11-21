import { marked } from 'marked';
import DOMPurify from 'dompurify';
import { AppleColor, AppleRadius, AppleSpace, AppleType } from '@/utils/core/apple-tokens';
import { storageGet, StorageKeys } from '@/utils/core/storage';
import { defineMonacoThemes, mermaidThemeVariables, monacoThemeForUi } from '@/styles/theme/highlight-tokens';

let katexModule: typeof import('katex').default | null = null;
let katexLoading: Promise<void> | null = null;

let monacoModule: typeof import('monaco-editor') | null = null;
let monacoLoading: Promise<typeof import('monaco-editor') | null> | null = null;

function ensureKatex(): void {
  if (katexModule) return;
  if (!katexLoading) {
    katexLoading = import('katex')
      .then((mod) => { katexModule = mod.default; })
      .catch(() => { katexModule = null; katexLoading = null; });
    import('katex/dist/katex.min.css');
  }
}

function ensureMonaco(): Promise<typeof import('monaco-editor') | null> {
  if (monacoModule) return Promise.resolve(monacoModule);
  if (!monacoLoading) {
    monacoLoading = import('monaco-editor')
      .then((mod) => { monacoModule = mod; return mod; })
      .catch(() => { monacoModule = null; monacoLoading = null; return null; });
  }
  return monacoLoading;
}

// 配置 DOMPurify：只允许安全的链接协议
const ALLOWED_PROTOCOLS = new Set(['http:', 'https:', 'mailto:']);

function isAllowedUrl(url: string): boolean {
  if (!url) return true;
  if (url.startsWith('/') || url.startsWith('#')) return true;
  try {
    const parsed = new URL(url);
    return ALLOWED_PROTOCOLS.has(parsed.protocol);
  } catch {
    return false;
  }
}

DOMPurify.addHook('uponSanitizeAttribute', (node, data) => {
  if (data.attrName === 'href' || data.attrName === 'src') {
    if (!isAllowedUrl(data.attrValue)) {
      data.keepAttr = false;
    }
  }
});

let mermaidInitialized = false;

const MAX_CACHE_SIZE = 50;
const renderCache = new Map<string, { html: string; headings: Array<{ id: string; text: string; level: number }>; pending: boolean }>();

function hashContent(str: string): string {
  let h = 0;
  for (let i = 0; i < str.length; i++) {
    h = ((h << 5) - h + str.charCodeAt(i)) | 0;
  }
  return str.length + ':' + h;
}

export function clearRenderCache(): void {  renderCache.clear();
  // 重置主题相关缓存，确保切换主题后代码块颜色重新提取
  cssInjected = false;
  currentTheme = null;
  mermaidInitialized = false;

  // 主题切换时，不清空已高亮的代码块内容，避免闪屏。
  // Monaco 高亮使用的 CSS 类（如 .mtk7/.mtk8/.mtk9）颜色已由 extractAndApplyThemeColors()
  // 注入的样式表通过 !important 覆盖，随 CSS 变量自动过渡。
  // 只清除标记，让 highlightCodeBlocks() 在需要时重新提取主题色。
  if (typeof document !== 'undefined') {
    document.querySelectorAll('pre[data-highlighted]').forEach((pre) => {
      const htmlPre = pre as HTMLElement;
      htmlPre.removeAttribute('data-highlighted');
    });
  }
}

marked.setOptions({
  gfm: true,
  breaks: true,
});

/** mermaid 产出的是模型可控的字符串，进 innerHTML 前必须过白名单（两处共用同一条） */
function sanitizeMermaidSvg(svg: string): string {
  return DOMPurify.sanitize(svg, {
    ALLOWED_TAGS: ['svg', 'path', 'circle', 'rect', 'line', 'polyline', 'polygon', 'ellipse', 'g', 'text', 'tspan', 'defs', 'marker', 'title', 'desc', 'foreignObject', 'div', 'span', 'p', 'br', 'table', 'tr', 'td', 'th', 'thead', 'tbody', 'style'],
    ALLOWED_ATTR: ['id', 'class', 'style', 'd', 'x', 'y', 'width', 'height', 'viewBox', 'fill', 'stroke', 'stroke-width', 'transform', 'rx', 'ry', 'cx', 'cy', 'r', 'x1', 'y1', 'x2', 'y2', 'points', 'font-size', 'font-family', 'text-anchor', 'dominant-baseline', 'marker-end', 'marker-start', 'markerWidth', 'markerHeight', 'refX', 'refY', 'orient', 'xmlns', 'version', 'preserveAspectRatio', 'tabindex', 'role', 'aria-roledescription'],
  }) as string;
}

/**
 * 主题版本号。首渲染那张图是异步的 —— 从 `mermaid.render()` 发出到 SVG 进 DOM 之间，
 * 用户完全可能已经换了主题。这时候换主题那一趟的重绘**找不到节点**（图还没进 DOM），
 * 而这次切换也就永远丢了：现象是"深色下流程图还是浅底，翻一页才正常"。
 * 所以每次主题变化 +1，渲染中的那一张画完时发现自己跨过一次版本，就立刻按新主题重画。
 */
let themeGeneration = 0;

/** 由 ApMarkdownPreview 的主题 observer 调用；必须在 repaintMermaid 之前 */
export function bumpThemeGeneration(): void {
  themeGeneration++;
}

/** 按当前主题重画一张图（`node` 上必须带着 `data-mermaid-source`） */
async function repaintNode(node: HTMLElement, mermaid: typeof import('mermaid').default): Promise<void> {
  await mermaid
    .render(`mermaid-repaint-${Date.now()}-${Math.random().toString(36).slice(2)}`, node.dataset.mermaidSource || '')
    .then(({ svg }) => {
      node.innerHTML = sanitizeMermaidSvg(svg);
    })
    .catch(() => { /* 单张图重画失败就留着上一张：比擦成空白好 */ });
}

/**
 * 按**当前主题**原地重画容器里已有的 mermaid 图，返回重画的张数。
 *
 * 为什么需要它而不是"重排整篇 markdown"：`mermaid.render()` 出的 SVG 里 fill/stroke 是
 * 渲染当时对 CSS 变量的快照；换主题后那些字符串不会跟着变。而整篇重排走不通 ——
 * 同一段内容渲染出的 HTML 字符串一模一样，`v-html` 判定"没变化"就一次 patch 都不做，
 * DOM 里留着的还是那张旧图（这一条是实测出来的，不是推的）。
 * 所以首渲染时把**围栏源码**存进 `data-mermaid-source`，换主题时只重画这几张图。
 */
export async function repaintMermaid(container: HTMLElement): Promise<number> {
  const nodes = [...container.querySelectorAll<HTMLElement>('.mermaid-diagram[data-mermaid-source]')];
  if (!nodes.length) return 0;
  const { default: mermaid } = await import('mermaid');
  // themeVariables 是 initialize 时的一次快照，重画前必须用当前主题重新拍一次
  mermaid.initialize({ startOnLoad: false, theme: 'base', themeVariables: mermaidThemeVariables() });
  mermaidInitialized = true;
  await Promise.all(nodes.map((node) => repaintNode(node, mermaid)));
  return nodes.length;
}

const apiBaseUrl = import.meta.env.VITE_API_BASE_URL || (typeof window !== 'undefined' ? window.location.origin + '/api/v1' : '/api/v1');
const serverBaseUrl = apiBaseUrl.replace(/\/api\/v\d+$/, '');

const imageRenderer = {
  image({ href, title, text }: { href: string | null; title: string | null; text: string }): string {
    if (!href) return `<span class="img-missing">[图片缺失]</span>`;
    let src = href;
    if (src.startsWith('/images/') || src.startsWith('/attachments/')) {
      src = serverBaseUrl + src;
      // 附加 token 用于后端 OptionalAuthMiddleware 的 ?token= 认证
      const token = storageGet(StorageKeys.AUTH_TOKEN);
      if (token) {
        const sep = src.includes('?') ? '&' : '?';
        src += sep + 'token=' + encodeURIComponent(token);
      }
    }
    src = encodeURI(src).replace(/\(/g, '%28').replace(/\)/g, '%29');
    const titleAttr = title ? ` title="${escapeHtml(title)}"` : '';
    // 添加 loading="lazy"（懒加载，视口外图片不立即下载）和 decoding="async"（异步解码，不阻塞主线程）
    const img = `<img src="${src}" alt="${escapeHtml(text)}"${titleAttr} loading="lazy" decoding="async" style="max-width:100%;" />`;
    if (title) {
      return `<figure>${img}<figcaption>${escapeHtml(title)}</figcaption></figure>`;
    }
    return img;
  },
  link({ href, title, text }: { href: string | null; title?: string | null; text: string }): string {
    if (!href) return text;
    let url = href;
    // 为附件/图片链接附加 token（支持私有空间访问）
    if (url.startsWith('/images/') || url.startsWith('/attachments/')) {
      const token = storageGet(StorageKeys.AUTH_TOKEN);
      if (token) {
        const sep = url.includes('?') ? '&' : '?';
        url += sep + 'token=' + encodeURIComponent(token);
      }
    }
    url = encodeURI(url).replace(/\(/g, '%28').replace(/\)/g, '%29');
    const titleAttr = title ? ` title="${escapeHtml(title)}"` : '';
    // 非图片的附件链接添加 download 属性触发下载
    const isImageLink = href.startsWith('/images/') || /\.(png|jpe?g|gif|webp|svg|bmp)$/i.test(href);
    const downloadAttr = !isImageLink && href.startsWith('/attachments/') ? ' download' : '';
    // 修复：链接文本中若包含 ![alt](imgurl) 图片语法，手动将其渲染为 <img>，
    // 解决 marked 解析嵌套图片链接 [![alt](img)](link) 时内层图片无法显示的问题
    const processedText = text.replace(/!\[([^\]]*)\]\(([^)]+)\)/g, (_, alt, imgUrl) => {
      let src = imgUrl;
      if (src.startsWith('/images/') || src.startsWith('/attachments/')) {
        src = serverBaseUrl + src;
        const token = storageGet(StorageKeys.AUTH_TOKEN);
        if (token) {
          const sep = src.includes('?') ? '&' : '?';
          src += sep + 'token=' + encodeURIComponent(token);
        }
      }
      src = encodeURI(src).replace(/\(/g, '%28').replace(/\)/g, '%29');
      return `<img src="${src}" alt="${escapeHtml(alt)}" loading="lazy" decoding="async" style="max-width:100%;" />`;
    });
    // 页内锚点（#开头）与站内链接（/开头）不再强制新标签打开：
    // 锚点由预览组件点击委托做平滑滚动，站内路由交给 SPA 接管；
    // 仅外部 http(s)/mailto 链接保持 target=_blank
    const isAnchor = href.startsWith('#');
    const isInternal = !isAnchor && url.startsWith('/');
    const targetAttr = isAnchor || isInternal ? '' : ' target="_blank" rel="noopener noreferrer"';
    return `<a href="${url}"${targetAttr}${titleAttr}${downloadAttr}>${processedText}</a>`;
  },
};

const blockquoteRenderer = {
  /**
   * marked v12+ 把 renderer 的入参改成了 token：`token.text` 是**未解析的原始 markdown**，
   * 必须走 parser 才能拿到行内 HTML（之前直接拼 text，导致引用块里 `**粗体**`、
   * `` `代码` ``、`[!note]` 全部按字面量显示）。
   */
  blockquote(
    this: { parser: { parse: (tokens: never[]) => string } },
    token: { tokens?: unknown; text?: string },
  ): string {
    // marked v18：block-level 子节点要走 parser.parse（parseInline 只吃 inline token，
    // 传 paragraph 会抛 "Token with paragraph type was not found"）
    const raw = token?.tokens as never[] | undefined;
    let parsed = '';
    try {
      if (raw?.length && typeof this?.parser?.parse === 'function') parsed = String(this.parser.parse(raw));
    } catch {
      parsed = '';
    }
    const text = parsed || String(token?.text ?? '');
    const calloutMatch = text.match(/^<p>\[!(note|warning|tip|danger|info)\]\s*/i);
    if (calloutMatch) {
      const type = calloutMatch[1]!.toLowerCase();
      const content = text.replace(calloutMatch[0], '<p>');
      // 标签而非 emoji：与 GitHub 的 [!NOTE] 渲染一致，且颜色取自主题 callout 令牌
      const labels: Record<string, string> = {
        note: '说明', warning: '警告', tip: '提示', danger: '危险', info: '信息'
      };
      return `<div class="callout callout-${type}"><div class="callout-title">${labels[type]}</div><div class="callout-content">${content}</div></div>`;
    }
    const toggleMatch = text.match(/^<p>\[!toggle\]\s*(.*?)<\/p>(?:<p>([\s\S]*)<\/p>)?/i);
    if (toggleMatch) {
      const title = toggleMatch[1] || '折叠内容';
      const content = toggleMatch[2] || '';
      return `<details class="toggle-block"><summary>${title}</summary><div class="toggle-content">${content}</div></details>`;
    }
    return `<blockquote>${text}</blockquote>`;
  }
};

marked.use({ renderer: { ...imageRenderer, ...blockquoteRenderer } });

// 通过 postprocess hook 为表格包裹滚动容器
marked.use({
  hooks: {
    postprocess(html: string): string {
      return html.replace(/<table>/g, '<div class="table-wrapper"><table>').replace(/<\/table>/g, '</table></div>');
    }
  }
});

import { escapeHtml } from '@/utils/core/shared';
import { copyText } from '@/utils/clipboard';
import i18n from '@/i18n';

const { t } = i18n.global;

function slugify(text: string): string {
  // GitHub 风格 slug：保留中文等 Unicode 字母，空格/标点转 -，
  // 使用户可以手写 [标题](#标题slug) 锚点直接命中（query 时经 CSS.escape，无选择器问题）
  const result = text
    .toLowerCase()
    .replace(/<[^>]+>/g, '')
    .replace(/[^\p{L}\p{N}_-]+/gu, '-')
    .replace(/-+/g, '-')
    .replace(/^-|-$/g, '') || 'heading';
  return result;
}

export function makeHeadingId(text: string, index: number): string {
  // GitHub 约定：首次出现无后缀，重复出现追加 -1/-2…（index 即重复序号）
  const slug = slugify(text);
  return index === 0 ? slug : `${slug}-${index}`;
}

// ============================================================
// 内容嵌入(Embed)：支持 YouTube、B站、Vimeo、Figma、CodePen、通用 iframe
// ============================================================

// 嵌入白名单：域名 → 转换函数
const EMBED_WHITELIST: Array<{ pattern: RegExp; render: (url: URL) => { src: string; title: string } }> = [
  // YouTube: https://www.youtube.com/watch?v=VIDEO_ID 或 https://youtu.be/VIDEO_ID
  {
    pattern: /(?:youtube\.com\/watch\?v=|youtu\.be\/)([\w-]{11})/,
    render: (url) => {
      const match = url.href.match(/(?:youtube\.com\/watch\?v=|youtu\.be\/)([\w-]{11})/);
      const videoId = match ? match[1] : '';
      return { src: `https://www.youtube.com/embed/${videoId}`, title: t('tsUi.k14') };
    },
  },
  // Bilibili: https://www.bilibili.com/video/BVxxxx
  {
    pattern: /bilibili\.com\/video\/(BV[\w]+)/,
    render: (url) => {
      const match = url.href.match(/bilibili\.com\/video\/(BV[\w]+)/);
      const bvid = match ? match[1] : '';
      return { src: `https://player.bilibili.com/player.html?bvid=${bvid}&high_quality=1`, title: t('tsUi.k15') };
    },
  },
  // Vimeo
  {
    pattern: /vimeo\.com\/(\d+)/,
    render: (url) => {
      const match = url.href.match(/vimeo\.com\/(\d+)/);
      const videoId = match ? match[1] : '';
      return { src: `https://player.vimeo.com/video/${videoId}`, title: t('tsUi.k16') };
    },
  },
  // CodePen: https://codepen.io/pen/xxx
  {
    pattern: /codepen\.io\/([\w]+)\/pen\/([\w]+)/,
    render: (url) => {
      const match = url.href.match(/codepen\.io\/([\w]+)\/pen\/([\w]+)/);
      if (match) {
        return { src: `https://codepen.io/${match[1]}/embed/${match[2]}`, title: 'CodePen' };
      }
      return { src: url.href, title: 'CodePen' };
    },
  },
  // Figma
  {
    pattern: /figma\.com/,
    render: (url) => {
      return { src: `https://www.figma.com/embed?embed_host=wiki&url=${encodeURIComponent(url.href)}`, title: t('tsUi.k17') };
    },
  },
];

function createEmbedWidget(rawUrl: string): HTMLElement {
  const wrapper = document.createElement('div');
  wrapper.className = 'embed-widget';

  if (!rawUrl) {
    wrapper.innerHTML = '<div class="embed-error">嵌入链接为空</div>';
    return wrapper;
  }

  let url: URL;
  try {
    url = new URL(rawUrl);
  } catch {
    wrapper.innerHTML = '<div class="embed-error">无效的链接</div>';
    return wrapper;
  }

  if (url.protocol !== 'https:' && url.protocol !== 'http:') {
    wrapper.innerHTML = '<div class="embed-error">仅支持 http/https 链接</div>';
    return wrapper;
  }

  // 匹配白名单
  const matched = EMBED_WHITELIST.find(e => e.pattern.test(url.href));
  if (!matched) {
    // 非白名单域名，显示链接卡片而非直接嵌入
    wrapper.className = 'embed-card';
    const link = document.createElement('a');
    link.href = url.href;
    link.target = '_blank';
    link.rel = 'noopener noreferrer';
    link.textContent = url.href;
    link.style.cssText = 'color: var(--apple-apple-action-blue); text-decoration: none; word-break: break-all;';
    const label = document.createElement('div');
    label.style.cssText = `font-size: ${AppleType.microUI.size}; color: var(--apple-text-tertiary); margin-bottom: 4px;`;
    label.textContent = t('tsUi.k18');
    wrapper.appendChild(label);
    wrapper.appendChild(link);
    return wrapper;
  }

  const { src, title } = matched.render(url);
  const iframe = document.createElement('iframe');
  iframe.src = src;
  iframe.title = title;
  iframe.setAttribute('loading', 'lazy');
  iframe.setAttribute('referrerpolicy', 'no-referrer');
  iframe.setAttribute('sandbox', 'allow-scripts allow-same-origin allow-popups allow-forms');
  iframe.style.cssText = `width: 100%; height: 400px; border: none; border-radius: ${AppleRadius.small};`;
  wrapper.appendChild(iframe);
  return wrapper;
}



let cssInjected = false;
let currentTheme: string | null = null;

export function extractAndApplyThemeColors(): void {
  const themeKey = document.documentElement.getAttribute('data-theme') || 'light';
  const material = document.documentElement.getAttribute('data-material') || 'solid';
  // 缓存键含材质：base 不变但 solid↔glass 切换时也要重刷（玻璃有专属透明编辑器底）
  const cacheKey = `${themeKey}|${material}`;

  if (cssInjected && currentTheme === cacheKey) return;

  // 同步切换 Monaco 主题（与预览 --hl-* 同一调色板；glass/paper 为专属 UI 色）
  ensureMonaco().then((m) => {
    try {
      if (!m) return;
      defineMonacoThemes(m);
      m.editor.setTheme(monacoThemeForUi(themeKey, material));
    } catch {
      /* ignore */
    }
  });

  const existingStyle = document.getElementById('monaco-preview-colors');
  if (existingStyle) {
    existingStyle.remove();
    cssInjected = false;
  }

  const codeBg = getComputedStyle(document.documentElement).getPropertyValue('--apple-code-bg').trim() || AppleColor.codeBg;

  let previewCss = '';
  const styleEls = document.head.querySelectorAll('style');
  for (const style of styleEls) {
    const css = style.textContent || '';
    if (css.includes('.mtk') && css.includes('.monaco-editor')) {
      previewCss = css
        .replace(/\.monaco-editor\b/g, '.monaco-code-block')
        .replace(/\.monaco-editor\s+/g, '.monaco-code-block ')
        .replace(/background-color:[^;]+;?/g, '')
        .replace(/background:[^;]+;?/g, '');
      break;
    }
  }

  const overrideCss = `
/* VSCode 风格代码块样式覆盖（token 颜色由 apple-* Monaco 主题提供，不再硬编码覆盖） */
.monaco-code-block {
  background: ${codeBg} !important;
  border: none !important;
  border-radius: 0 !important;
  padding: ${AppleSpace.md} ${AppleSpace.md} !important;
  font-size: ${AppleType.caption.size} !important;
  line-height: 1.6 !important;
  margin: 0 !important;
}
.monaco-code-block .code-content {
  background: transparent !important;
}
.monaco-code-block *,
.monaco-code-block code {
  background: transparent !important;
  background-color: transparent !important;
}
`;
  const newStyle = document.createElement('style');
  newStyle.id = 'monaco-preview-colors';
  newStyle.textContent = previewCss + overrideCss;
  document.head.appendChild(newStyle);
  cssInjected = true;
  currentTheme = cacheKey;
}

// 公式占位符：使用 Unicode 私有区字符包裹，避免与用户正文中的
// "§KI0§" 之类字面量撞车（html.replace 按首次出现替换，撞车会破坏先出现的代码块）
const MATH_BLOCK_PH = (i: number): string => `\uE000KB${i}\uE001`;
const MATH_INLINE_PH = (i: number): string => `\uE000KI${i}\uE001`;

// ============================================================
// wiki 双链 [[页面名]] / [[页面名|别名]]
// ============================================================

// 由宿主组件注册的解析器：页面名 → 站内路由（如 /space/1/doc/12）；
// 未注册或解析不到时降级为"待创建"样式并链接到全文检索
type WikiLinkResolver = (name: string) => string | null;
let wikiLinkResolver: WikiLinkResolver | null = null;

export function setWikiLinkResolver(fn: WikiLinkResolver | null): void {
  wikiLinkResolver = fn;
}

function renderWikiLink(name: string, alias: string): string {
  let url: string | null;
  try {
    url = wikiLinkResolver ? wikiLinkResolver(name) : null;
  } catch {
    url = null;
  }
  if (url) {
    return `<a class="wikilink" href="${escapeHtml(url)}">${escapeHtml(alias)}</a>`;
  }
  return `<a class="wikilink wikilink-missing" href="/search?q=${encodeURIComponent(name)}" title="未找到文档「${escapeHtml(name)}」，点击搜索">${escapeHtml(alias)}</a>`;
}

// ============================================================
// emoji 短代码 :tada:（常用子集，覆盖 GitHub 常用前 70 个）
// ============================================================
const EMOJI_MAP: Record<string, string> = {
  '+1': '👍', '-1': '👎', '100': '💯', '1234': '🔢', '8ball': '🎱',
  'angry': '😠', 'angel': '👼', 'art': '🎨', 'astonished': '😲', 'beer': '🍺',
  'bell': '🔔', 'bike': '🚲', 'bomb': '💣', 'book': '📖', 'bug': '🐛',
  'bulb': '💡', 'cake': '🎂', 'calendar': '📆', 'calling': '📲', 'camera': '📷',
  'cat': '🐱', 'chart_with_upwards_trend': '📈', 'checkered_flag': '🏁',
  'clap': '👏', 'clock': '🕐', 'coffee': '☕', 'computer': '💻',
  'confounded': '😖', 'construction': '🚧', 'corn': '🌽', 'cow': '🐮',
  'cry': '😭', 'dancer': '💃', 'dart': '🎯', 'dog': '🐶', 'dollar': '💵',
  'droplet': '💧', 'email': '✉️', 'fire': '🔥', 'first_place_medal': '🥇',
  'fish': '🐟', 'fist': '✊', 'flag': '🏴', 'ghost': '👻', 'gift': '🎁',
  'golf': '⛳', 'grapes': '🍇', 'grin': '😁', 'heart': '❤️', 'heart_eyes': '😍',
  'helicopter': '🚁', 'herb': '🌿', 'horse': '🐴', 'hourglass': '⌛',
  'house': '🏠', 'joy': '😂', 'key': '🔑', 'kiss': '💋', 'lamp': '💡',
  'leaves': '🍃', 'ledger': '📒', 'lips': '👄', 'lock': '🔒', 'mag': '🔍',
  'mahjong': '🀄', 'man': '👨', 'memo': '📝', 'money_with_wings': '💸',
  'monkey_face': '🐵', 'moon': '🌙', 'mortar_board': '🎓', 'mountain': '⛰️',
  'mouse': '🐭', 'muscle': '💪', 'mouse2': '🖱️', 'nail_care': '💅',
  'necktie': '👔', 'new': '🆕', 'no_entry': '⛔', 'notes': '🎶',
  'ok_hand': '👌', 'ok_woman': '🙆‍♀️', 'package': '📦', 'page_facing_up': '📄',
  'pencil': '✏️', 'pencil2': '✏️', 'penguin': '🐧', 'pig': '🐷',
  'point_down': '👇', 'point_left': '👈', 'point_right': '👉', 'point_up': '👆',
  'postbox': '📮', 'pray': '🙏', 'question': '❓', 'rabbit': '🐰',
  'rainbow': '🌈', 'raised_hands': '🙌', 'recycle': '♻️', 'rocket': '🚀',
  'rosette': '🏵️', 'rotating_light': '🚨', 'smile': '😄', 'smiley': '😃',
  'smirk': '😏', 'sob': '😭', 'sparkles': '✨', 'sparkling_heart': '💖',
  'speaker': '🔊', 'star': '⭐', 'star2': '🌟', 'stars': '🌠',
  'sun_with_face': '🌞', 'sunflower': '🌻', 'sunny': '☀️', 'tada': '🎉',
  'tea': '🍵', 'thought_balloon': '💭', 'thumbsdown': '👎', 'thumbsup': '👍',
  'tiger': '🐯', 'tm': '™️', 'trophy': '🏆', 'triumph': '😤',
  'trolleybus': '🚎', 'trophy2': '🏆', 'umbrella': '☂️', 'unamused': '😒',
  'v': '✌️', 'warning': '⚠️', 'wave': '👋', 'watermelon': '🍉',
  'wheelchair': '♿', 'white_check_mark': '✅', 'wind_bell': '🎐',
  'wink': '😉', 'wolf': '🐺', 'womans_clothes': '👚', 'x': '❌',
  'yum': '😋', 'zap': '⚡', 'zero': '0️⃣', 'zzz': '💤',
};

function replaceEmojiToken(whole: string, pre: string, code: string): string {
  const emoji = EMOJI_MAP[code.toLowerCase()];
  return emoji ? pre + emoji : whole;
}

// ============================================================
// front-matter：剥离文档开头的 --- 元数据块，避免被当作 hr/正文渲染
// ============================================================
function stripFrontMatter(md: string): string {
  if (!md.startsWith('---')) return md;
  const firstNewline = md.indexOf('\n');
  if (firstNewline === -1) return md;
  // 闭合 --- 需独占一行（允许行尾空白）；找不到则按普通 hr 处理
  const closeMatch = md.slice(firstNewline + 1).match(/^(---)[ \t]*(\n|$)/m);
  if (!closeMatch || closeMatch.index === undefined) return md;
  const end = firstNewline + 1 + closeMatch.index + closeMatch[0].length;
  return md.slice(end).replace(/^\n+/, '');
}

// ============================================================
// 脚注 [^key] 引用 + [^key]: 定义
// ============================================================
const FN_REF_PH = (key: string): string => `\uE000FNREF${key}\uE001`;
const FN_LIST_MARK = '\uE000FNLIST\uE001';

interface FootnoteDef { key: string; number: number; text: string }

function preProcessFootnotes(md: string): { body: string; defs: FootnoteDef[] } {
  if (!/\[\^([^\]\s]+)\]/.test(md)) return { body: md, defs: [] };

  const lines = md.split('\n');
  const out: string[] = [];
  const defs: FootnoteDef[] = [];
  const defIndex = new Map<string, FootnoteDef>();
  let fenceChar: string | null = null;
  let fenceLen = 0;
  let lastDef: FootnoteDef | null = null;

  for (const line of lines) {
    const fenceMatch = line.match(/^\s{0,3}(`{3,}|~{3,})/);
    if (fenceMatch) {
      const marker = fenceMatch[1]!;
      const ch = marker[0]!;
      if (!fenceChar) {
        fenceChar = ch;
        fenceLen = marker.length;
      } else if (ch === fenceChar && marker.length >= fenceLen && line.trim() === marker) {
        fenceChar = null;
      }
      out.push(line);
      continue;
    }
    if (fenceChar) {
      out.push(line);
      continue;
    }

    // 定义行：从正文移除，集中到文末列表
    const defMatch = line.match(/^ {0,3}\[\^([^\]\s]+)\]:\s?(.*)$/);
    if (defMatch) {
      const key = defMatch[1]!;
      const existing = defIndex.get(key);
      if (existing) {
        existing.text += ' ' + (defMatch[2] || '');
      } else {
        const def: FootnoteDef = { key, number: defs.length + 1, text: defMatch[2] || '' };
        defs.push(def);
        defIndex.set(key, def);
      }
      lastDef = defIndex.get(key)!;
      continue;
    }
    // 定义的多行续文（缩进 4 空格/Tab 起始）
    if (lastDef && /^(?:\t| {4})\S/.test(line)) {
      lastDef.text += ' ' + line.trim();
      continue;
    }
    if (line.trim() !== '') {
      lastDef = null; // 非空普通行打断续文
    }

    // 引用替换为占位符（解析后再转 <sup> 链接）
    out.push(line.replace(/\[\^([^\]\s]+)\]/g, (_, key: string) => FN_REF_PH(key)));
  }

  let body = out.join('\n');
  if (defs.length > 0) {
    body += `\n${FN_LIST_MARK}\n` + defs.map((d) => `${d.number}. ${d.text}`).join('\n');
  }
  return { body, defs };
}

function postProcessFootnotes(html: string, defs: FootnoteDef[]): string {
  if (defs.length === 0) {
    // 只有引用没有定义：还原为字面量
    return html.replace(/\uE000FNREF([^\uE001]*)\uE001/g, (_, key: string) => `[^${key}]`);
  }
  const numberByKey = new Map(defs.map((d) => [d.key, d.number] as const));
  html = html.replace(/\uE000FNREF([^\uE001]*)\uE001/g, (whole, key: string) => {
    const num = numberByKey.get(key);
    if (!num) return `[^${key}]`;
    return `<sup class="footnote-ref" id="fnref-${num}"><a href="#fn-${num}">[${num}]</a></sup>`;
  });
  // 定义列表：标记行渲染为 <p>MARKER</p>，其后是 <ol>，包上 .footnotes 容器
  html = html.replace(new RegExp(`<p>${FN_LIST_MARK}</p>\\s*<ol>`), '<div class="footnotes"><ol>');
  if (html.includes('<div class="footnotes"><ol>')) {
    // 定义列表是文档末尾追加的，其 </ol> 即文档最后一个元素
    html = html.replace(/<\/ol>\s*$/, '</ol></div>');
  }
  return html;
}

function preProcessMath(md: string): { text: string; mathBlocks: string[]; inlineMaths: string[] } {
  const mathBlocks: string[] = [];
  const inlineMaths: string[] = [];
  const lines = md.split('\n');
  const out: string[] = [];

  // 围栏状态：记录开栏字符（` 或 ~）与长度，闭合行必须是"仅围栏标记"
  // （旧实现只认 ``` 开头且见 ``` 就翻转，导致 ~~~ 围栏、行内代码、缩进代码里的
  //   $xx$ 被误当公式替换成占位符，shell 脚本等内容被破坏）
  let fenceChar: string | null = null;
  let fenceLen = 0;
  let inBlockMath = false;
  let blockMathBuf: string[] = [];

  const pushBlockMath = (raw: string): string => {
    mathBlocks.push(raw);
    return MATH_BLOCK_PH(mathBlocks.length - 1);
  };
  const pushInlineMath = (raw: string): string => {
    inlineMaths.push(raw);
    return MATH_INLINE_PH(inlineMaths.length - 1);
  };

  // 单段文本（不含行内代码）的公式替换：
  // 先整段 $$..$$（块公式），再 $..$（行内公式）。
  // 行内公式沿用 GitHub/Pandoc 启发式：开 $ 后、闭 $ 前不能是空白，
  // 避免 "价格 $5，成本 $6" 这类普通文本被误判。
  const processSegment = (seg: string): string =>
    seg
      .replace(/\$\$([^$\n]+?)\$\$/g, (_, m: string) => pushBlockMath(`$$${m.trim()}$$`))
      .replace(/\$([^\s$][^$\n]*?)\$/g, (whole, m: string) => {
        const t = m.trim();
        if (!t) return whole;
        return pushInlineMath(m);
      })
      // wiki 双链
      .replace(/\[\[([^\]|\n]+)(?:\|([^\]\n]+))?\]\]/g, (_, name: string, alias?: string) =>
        renderWikiLink(name.trim(), (alias || name).trim()))
      // emoji 短代码
      .replace(/(^|[^:\w]):([a-zA-Z0-9_+-]{2,30}):/g, (whole, pre: string, code: string) => replaceEmojiToken(whole, pre, code));

  // 按行内代码 span 切分：`...` 之间的内容原样保留，其余才做公式替换
  const processLine = (line: string): string => {
    let result = '';
    let i = 0;
    const n = line.length;
    while (i < n) {
      if (line[i] === '`') {
        let run = 1;
        while (i + run < n && line[i + run] === '`') run++;
        const marker = '`'.repeat(run);
        const close = line.indexOf(marker, i + run);
        if (close !== -1) {
          result += line.slice(i, close + run);
          i = close + run;
        } else {
          result += line.slice(i);
          i = n;
        }
        continue;
      }
      let j = i;
      while (j < n && line[j] !== '`') j++;
      result += processSegment(line.slice(i, j));
      i = j;
    }
    return result;
  };

  for (const line of lines) {
    // 1) 围栏行：``` 或 ~~~ 开栏；闭合行必须是同一字符、长度足够且行内仅有围栏标记
    const fenceMatch = line.match(/^\s{0,3}(`{3,}|~{3,})/);
    if (fenceMatch) {
      const marker = fenceMatch[1]!;
      const ch = marker[0]!;
      if (!fenceChar) {
        fenceChar = ch;
        fenceLen = marker.length;
      } else if (ch === fenceChar && marker.length >= fenceLen && line.trim() === marker) {
        fenceChar = null;
      }
      out.push(line);
      continue;
    }
    if (fenceChar) {
      out.push(line);
      continue;
    }

    // 2) 缩进代码块（4 空格 / Tab）：整行保护，终端粘贴的脚本不受公式替换影响
    if (/^(?:\t| {4})/.test(line)) {
      out.push(line);
      continue;
    }

    // 3) 跨行块公式 $$\n...\n$$
    if (inBlockMath) {
      const closeIdx = line.indexOf('$$');
      if (closeIdx !== -1) {
        blockMathBuf.push(line.slice(0, closeIdx));
        out.push(pushBlockMath(`$$\n${blockMathBuf.join('\n').trim()}\n$$`));
        blockMathBuf = [];
        inBlockMath = false;
        const rest = line.slice(closeIdx + 2);
        if (rest.trim()) out.push(processLine(rest));
      } else {
        blockMathBuf.push(line);
      }
      continue;
    }

    const trimmed = line.trim();
    if (trimmed.startsWith('$$')) {
      const rest = trimmed.slice(2);
      const closeIdx = rest.indexOf('$$');
      if (closeIdx !== -1 && rest.slice(0, closeIdx).trim()) {
        // 单行块公式 $$x^2$$
        out.push(pushBlockMath(`$$${rest.slice(0, closeIdx).trim()}$$`));
        const tail = rest.slice(closeIdx + 2);
        if (tail.trim()) out.push(processLine(tail));
        continue;
      }
      if (closeIdx === -1) {
        // 块公式开始，等待闭合 $$
        inBlockMath = true;
        blockMathBuf = rest ? [rest] : [];
        continue;
      }
      out.push(line);
      continue;
    }

    out.push(processLine(line));
  }

  // 未闭合的块公式：还原为普通文本，避免占位符悬空
  if (inBlockMath) {
    out.push('$$', ...blockMathBuf);
  }

  return { text: out.join('\n'), mathBlocks, inlineMaths };
}

function postProcessMath(html: string, mathBlocks: string[], inlineMaths: string[]): { html: string; pending: boolean } {
  if (!katexModule) {
    // KaTeX 尚未加载完成：先以原文降级展示，避免内部占位符泄漏给用户。
    // 调用方据此跳过渲染缓存，待加载完成后重新渲染。
    let degraded = html;
    for (let i = 0; i < mathBlocks.length; i++) {
      const math = (mathBlocks[i] || '').slice(2, -2).trim();
      degraded = degraded.replace(MATH_BLOCK_PH(i), `<div class="katex-block">${escapeHtml(math)}</div>`);
    }
    for (let i = 0; i < inlineMaths.length; i++) {
      degraded = degraded.replace(MATH_INLINE_PH(i), escapeHtml(inlineMaths[i] || ''));
    }
    return { html: degraded, pending: true };
  }
  for (let i = 0; i < mathBlocks.length; i++) {
    const math = (mathBlocks[i] || '').slice(2, -2).trim();
    try {
      const rendered = katexModule.renderToString(math, { displayMode: true, throwOnError: false });
      html = html.replace(MATH_BLOCK_PH(i), `<div class="katex-block">${rendered}</div>`);
    } catch {
      html = html.replace(MATH_BLOCK_PH(i), `<div class="katex-block katex-error">${escapeHtml(math)}</div>`);
    }
  }
  for (let i = 0; i < inlineMaths.length; i++) {
    try {
      const rendered = katexModule.renderToString(inlineMaths[i] || '', { displayMode: false, throwOnError: false });
      html = html.replace(MATH_INLINE_PH(i), rendered);
    } catch {
      html = html.replace(MATH_INLINE_PH(i), `<span class="katex-error">${escapeHtml(inlineMaths[i] || '')}</span>`);
    }
  }
  return { html, pending: false };
}

// ============================================================
// #5 表格高级功能：Grid Tables 预处理 + 单元格合并后处理
// ============================================================

/**
 * Grid Tables 预处理：将 +---+---+ 格式的 Grid Table 转换为标准 pipe table
 *
 * 支持的语法：
 *   +-------+-------+-------+
 *   | H1    | H2    | H3    |
 *   +=======+=======+=======+
 *   | A     | B     | C     |
 *   +-------+-------+-------+
 *   | 合并两列               |
 *   +-------+-------+-------+
 *
 * 单元格合并标记（在 pipe table 中使用）：
 *   >  = 向右合并（colspan），该单元格内容合并到左侧单元格
 *   ^^ = 向上合并（rowspan），该单元格内容合并到上方单元格
 */
function preProcessGridTables(text: string): string {
  const lines = text.split('\n');
  const result: string[] = [];
  let i = 0;

  while (i < lines.length) {
    const line = lines[i];

    // 检测 Grid Table 起始行：+---+---+---+
    if (line && /^\+[-=+]+\+$/.test(line.trim())) {
      const gridBlock = collectGridBlock(lines, i);
      if (gridBlock.lines.length >= 3) {
        const pipeTable = convertGridToPipe(gridBlock.lines);
        if (pipeTable) {
          result.push(pipeTable);
          i = gridBlock.endIndex + 1;
          continue;
        }
      }
    }

    result.push(line || '');
    i++;
  }

  return result.join('\n');
}

/** 收集连续的 Grid Table 行 */
function collectGridBlock(lines: string[], start: number): { lines: string[]; endIndex: number } {
  const block: string[] = [];
  let i = start;
  while (i < lines.length) {
    const l = lines[i];
    if (!l) break;
    const trimmed = l.trim();
    // Grid Table 行：分隔符行(+---+) 或数据行(| ... |)
    if (/^\+[-=+]+\+$/.test(trimmed) || /^\|.*\|$/.test(trimmed)) {
      block.push(l);
      i++;
    } else {
      break;
    }
  }
  return { lines: block, endIndex: i - 1 };
}

/** 将 Grid Table 转换为 pipe table */
function convertGridToPipe(gridLines: string[]): string | null {
  // 从第一个分隔符行提取列边界位置
  const firstSep = gridLines[0]!.trim();
  const colBoundaries: number[] = [];
  for (let i = 0; i < firstSep.length; i++) {
    if (firstSep[i] === '+') colBoundaries.push(i);
  }
  if (colBoundaries.length < 3) return null;

  const pipeRows: string[] = [];
  let hasHeader = false;

  for (let i = 1; i < gridLines.length; i++) {
    const line = gridLines[i]!.trim();

    // 跳过分隔符行
    if (/^\+[-=+]+\+$/.test(line)) {
      // 检测 header 分隔符 (+=+)
      if (/^\+[=+]+\+$/.test(line)) {
        hasHeader = true;
      }
      continue;
    }

    // 数据行：| ... | ... |
    if (!line.startsWith('|') || !line.endsWith('|')) continue;

    // 按列边界提取单元格
    const cells: string[] = [];
    for (let c = 0; c < colBoundaries.length - 1; c++) {
      const start = colBoundaries[c]! + 1;
      const end = colBoundaries[c + 1]!;
      // 原始行可能比第一个分隔符短，需做边界检查
      const rawLine = gridLines[i]!;
      if (end > rawLine.length) {
        cells.push(rawLine.slice(start).trim());
      } else {
        cells.push(rawLine.slice(start, end).trim());
      }
    }

    // 检测跨列：如果某个单元格内容跨越了多个列边界位置
    // 简化处理：直接使用提取的单元格，对于 Grid Table 中内容超过一个列宽的单元格，
    // 上面的 slice 已经正确提取了内容
    pipeRows.push('| ' + cells.join(' | ') + ' |');
  }

  if (pipeRows.length === 0) return null;

  // 构建 pipe table
  const result: string[] = [];
  if (hasHeader && pipeRows.length >= 1) {
    result.push(pipeRows[0]!);
    // 生成对齐分隔行
    const firstCells = pipeRows[0]!.split('|').filter((_, idx, arr) => idx > 0 && idx < arr.length - 1);
    const sepCells = firstCells.map(() => '------');
    result.push('| ' + sepCells.join(' | ') + ' |');
    result.push(...pipeRows.slice(1));
  } else {
    // 无表头，全部作为数据行
    result.push(...pipeRows);
  }

  return result.join('\n');
}

/**
 * 表格合并后处理：将 > 和 ^^ 标记转换为 colspan/rowspan 属性
 *
 * 在 pipe table 中：
 *   | 文本 | >  | >  |  →  <td colspan="3">文本</td>
 *   | 文本 | ^^ |     →  上方单元格 rowspan="2"
 */
function postProcessTableMerging(html: string): string {
  // 匹配 <table>...</table> 块
  return html.replace(/<table>([\s\S]*?)<\/table>/g, (_, tableContent: string) => {
    return '<table>' + processTableMerging(tableContent) + '</table>';
  });
}

/** 处理单个表格的合并标记 */
function processTableMerging(tableContent: string): string {
  // 按 <tr> 分割行
  const rows: string[] = [];
  const trRegex = /<tr[^>]*>([\s\S]*?)<\/tr>/g;
  let match: RegExpExecArray | null;
  while ((match = trRegex.exec(tableContent)) !== null) {
    rows.push(match[0]);
  }

  if (rows.length === 0) return tableContent;

  // 解析每行的单元格
  const parsedRows = rows.map((row) => {
    const cellRegex = /<(td|th)[^>]*>([\s\S]*?)<\/\1>/g;
    const cells: Array<{ tag: string; attrs: string; content: string; isMergeRight: boolean; isMergeUp: boolean }> = [];
    let m: RegExpExecArray | null;
    while ((m = cellRegex.exec(row)) !== null) {
      const tag = m[1]!;
      const content = m[2]!;
      const trimmed = content.trim();

      // 检测合并标记（允许 < 或 &lt; 作为"向右合并"标记）
      const isMergeRight = trimmed === '>' || trimmed === '&gt;' || trimmed === '<' || trimmed === '&lt;';
      const isMergeUp = trimmed === '^^' || trimmed === '^';

      // 提取现有属性
      const tagMatch = row.match(new RegExp(`<(?:${tag})([^>]*)>`));
      const attrs = tagMatch ? tagMatch[1] || '' : '';

      cells.push({ tag, attrs, content, isMergeRight, isMergeUp });
    }
    return { raw: row, cells };
  });

  // 处理 colspan（向右合并）
  for (const row of parsedRows) {
    for (let i = row.cells.length - 1; i >= 0; i--) {
      const cell = row.cells[i]!;
      if (cell.isMergeRight && i > 0) {
        // 找到左侧第一个非合并单元格，增加 colspan
        let targetIdx = i - 1;
        while (targetIdx >= 0 && row.cells[targetIdx]!.isMergeRight) {
          targetIdx--;
        }
        if (targetIdx >= 0) {
          const target = row.cells[targetIdx]!;
          const existingColspan = target.attrs.match(/colspan="(\d+)"/);
          const currentSpan = existingColspan ? parseInt(existingColspan[1]!, 10) : 1;
          const newSpan = currentSpan + 1;
          if (existingColspan) {
            target.attrs = target.attrs.replace(/colspan="\d+"/, `colspan="${newSpan}"`);
          } else {
            target.attrs += ` colspan="${newSpan}"`;
          }
        }
        // 移除合并标记单元格
        row.cells.splice(i, 1);
      }
    }
  }

  // 处理 rowspan（向上合并）
  for (let r = 1; r < parsedRows.length; r++) {
    const row = parsedRows[r]!;
    for (let i = 0; i < row.cells.length; i++) {
      const cell = row.cells[i]!;
      if (cell.isMergeUp) {
        // 在上方行中找到同列的单元格，增加 rowspan
        for (let up = r - 1; up >= 0; up--) {
          const upRow = parsedRows[up]!;
          if (i < upRow.cells.length) {
            const upCell = upRow.cells[i]!;
            if (upCell.isMergeUp) continue; // 跳过也是合并标记的单元格
            const existingRowspan = upCell.attrs.match(/rowspan="(\d+)"/);
            const currentSpan = existingRowspan ? parseInt(existingRowspan[1]!, 10) : 1;
            const newSpan = currentSpan + 1;
            if (existingRowspan) {
              upCell.attrs = upCell.attrs.replace(/rowspan="\d+"/, `rowspan="${newSpan}"`);
            } else {
              upCell.attrs += ` rowspan="${newSpan}"`;
            }
            // 移除合并标记单元格
            row.cells.splice(i, 1);
            i--; // 调整索引
            break;
          }
        }
      }
    }
  }

  // 重建表格行
  return parsedRows
    .map((row) => {
      // 提取 <tr> 的属性
      const trAttrMatch = row.raw.match(/<tr([^>]*)>/);
      const trAttr = trAttrMatch ? trAttrMatch[1] || '' : '';
      const cellsHtml = row.cells
        .map((c) => `<${c.tag}${c.attrs}>${c.content}</${c.tag}>`)
        .join('');
      return `<tr${trAttr}>${cellsHtml}</tr>`;
    })
    .join('');
}

// ============================================================
// #6 交互式表格：点击表头排序
// ============================================================

export function useMarkdownRenderer() {
  function render(md: string): { html: string; headings: Array<{ id: string; text: string; level: number }>; pending: boolean } {
    if (typeof md !== 'string') {
      if (md && typeof md === 'object' && 'content' in md) {
        md = String((md as { content: unknown }).content);
      } else {
        return { html: String(md), headings: [], pending: false };
      }
    }
    if (!md) return { html: '', headings: [], pending: false };

    // 行尾/BOM 归一化：CRLF 会让 marked 的 fence 解析在代码内容开头多出一个空行
    //（\r 残留在围栏信息串里），BOM 会让围栏完全不被识别（整篇按段落解析）
    md = md.replace(/^\uFEFF/, '').replace(/\r\n?/g, '\n');

    ensureKatex();

    const cacheKey = hashContent(md);
    const cached = renderCache.get(cacheKey);
    if (cached) return cached;

    // front-matter 剥离（--- 包裹的元数据不参与渲染）
    md = stripFrontMatter(md);
    if (!md) return { html: '', headings: [], pending: false };

    // 脚注提取：定义行移出正文，引用替换为占位符（需在公式/表格预处理前完成）
    const footnotes = preProcessFootnotes(md);

    const { text: mathProcessed, mathBlocks, inlineMaths } = preProcessMath(footnotes.body);

    // #5 Grid Tables 预处理（在 marked 解析前转换）
    const gridProcessed = preProcessGridTables(mathProcessed);

    let result: string;
    try {
      result = marked.parse(gridProcessed) as string;
    } catch {
      return { html: `<pre>${escapeHtml(md)}</pre>`, headings: [], pending: false };
    }

    const mathResult = postProcessMath(result, mathBlocks, inlineMaths);
    result = mathResult.html;
    const mathPending = mathResult.pending;
    // 脚注：占位符转 <sup> 链接，定义列表包 .footnotes 容器
    result = postProcessFootnotes(result, footnotes.defs);
    // #5 表格合并后处理（> 和 ^^ 标记转换为 colspan/rowspan）
    result = postProcessTableMerging(result);

    // XSS 过滤：先清理恶意脚本，再提取 headings
    const sanitizedHtml = DOMPurify.sanitize(result, {
      ALLOWED_TAGS: [
        'p', 'br', 'hr', 'div', 'span', 'h1', 'h2', 'h3', 'h4', 'h5', 'h6',
        'ul', 'ol', 'li', 'blockquote', 'pre', 'code', 'strong', 'em', 'a', 'img',
        'table', 'thead', 'tbody', 'tfoot', 'tr', 'th', 'td', 'caption', 'col', 'colgroup',
        'sup', 'sub', 'del', 'ins', 'kbd', 's', 'b', 'i', 'u', 'mark', 'small',         'dl', 'dt', 'dd',
        'figure', 'figcaption',
        'details', 'summary',
        // GFM 任务列表勾选框
        'input',
      ],
      ALLOWED_ATTR: [
        'href', 'title', 'src', 'alt', 'id', 'class', 'data-lang', 'data-citation-idx',
        'target', 'rel', 'width', 'height',
        // 任务列表勾选框属性
        'type', 'checked', 'disabled',
        // 图片懒加载/异步解码属性，避免切换文档时所有图片同时下载阻塞
        'loading', 'decoding',
        // 表格相关属性
        'colspan', 'rowspan', 'align', 'valign', 'scope', 'headers',
        'abbr', 'axis', 'bgcolor', 'border', 'cellpadding', 'cellspacing',
        'frame', 'rules', 'summary',
        // 允许安全的样式属性
        'style',
        // details/summary
        'open',
      ],
      // @ts-expect-error ALLOWED_STYLES exists at runtime but not in DOMPurify types
      ALLOWED_STYLES: [
        'width', 'height', 'max-width', 'border', 'border-collapse', 'border-spacing',
        'text-align', 'vertical-align', 'background-color', 'color',
        'font-weight', 'font-style', 'padding', 'margin',
      ],
    });

    // 在 sanitize 之后提取 headings，确保 ID 与渲染结果一致
    const div = document.createElement('div');
    div.innerHTML = sanitizedHtml;

    const DANGEROUS_CSS_VALUES = /url\s*\(|expression\s*\(|javascript\s*:|behavior\s*:/i;
    div.querySelectorAll('[style]').forEach(el => {
      const style = el.getAttribute('style') ?? '';
      if (DANGEROUS_CSS_VALUES.test(style)) {
        el.removeAttribute('style');
      }
    });
    const headingEls = div.querySelectorAll('h1, h2, h3, h4, h5, h6');
    const headings: Array<{ id: string; text: string; level: number }> = [];
    const slugCount = new Map<string, number>();

    headingEls.forEach((h) => {
      const text = h.textContent || '';
      // 同名标题去重：首次无后缀，之后 -1/-2…（与 GitHub 锚点约定一致，
      // 使作者手写的 [标题](#slug) 可以直接命中）
      const base = slugify(text);
      const seen = slugCount.get(base) ?? 0;
      slugCount.set(base, seen + 1);
      const id = makeHeadingId(text, seen);
      h.setAttribute('id', id);
      h.setAttribute('class', 'doc-heading');
      // 标题锚点：hover 显示链接图标，点击复制带锚点的页面链接（由预览组件委托处理）。
      // 这里绝不能用字面 '#'——它和「没解析掉的 markdown」长得一模一样，用户只会以为渲染坏了。
      const anchorEl = document.createElement('a');
      anchorEl.className = 'heading-anchor';
      anchorEl.href = `#${id}`;
      anchorEl.innerHTML =
        '<svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">' +
        '<path d="M10 13a5 5 0 0 0 7.54.54l3-3a5 5 0 0 0-7.07-7.07l-1.72 1.71"/>' +
        '<path d="M14 11a5 5 0 0 0-7.54-.54l-3 3a5 5 0 0 0 7.07 7.07l1.71-1.71"/></svg>';
      anchorEl.setAttribute('aria-label', '复制标题链接');
      anchorEl.setAttribute('title', '复制标题链接');
      h.appendChild(anchorEl);
      headings.push({
        id,
        text,
        level: parseInt(h.tagName.charAt(1)),
      });
    });

    // 任务列表：含勾选框的 li 加 task-item 类（样式上隐藏列表圆点）
    div.querySelectorAll('li').forEach((li) => {
      const first = li.firstElementChild;
      if (first && first.tagName === 'INPUT' && (first as HTMLInputElement).type === 'checkbox') {
        li.classList.add('task-item');
      }
    });

    // 脚注列表：按定义顺序补 li id 与回链
    if (footnotes.defs.length > 0) {
      div.querySelectorAll('.footnotes ol > li').forEach((li, i) => {
        const num = i + 1;
        (li as HTMLElement).id = `fn-${num}`;
        const back = document.createElement('a');
        back.className = 'footnote-backref';
        back.href = `#fnref-${num}`;
        back.textContent = '↩';
        li.appendChild(back);
      });
    }

    const sanitized = {
      html: div.innerHTML,
      headings,
      pending: mathPending
    };

    // KaTeX 未就绪的降级渲染不缓存，待加载完成后重新渲染出完整公式
    if (!mathPending) {
      if (renderCache.size >= MAX_CACHE_SIZE) {
        const firstKey = renderCache.keys().next().value;
        if (firstKey !== undefined) renderCache.delete(firstKey);
      }
      renderCache.set(cacheKey, sanitized);
    }

    return sanitized;
  }

let hljsModule: typeof import('highlight.js').default | null = null;
let hljsLoading: Promise<typeof import('highlight.js').default | null> | null = null;

function ensureHLJS(): Promise<typeof import('highlight.js').default | null> {
  if (hljsModule) return Promise.resolve(hljsModule);
  if (!hljsLoading) {
    hljsLoading = import('highlight.js')
      .then((mod) => {
        hljsModule = mod.default;
        // 代码配色不再用 github.css：统一走 highlight-tokens.ts 的 --hl-* 体系
        // （与 Monaco apple-* 主题同一调色板，自动跟随明暗主题）
        return hljsModule;
      })
      .catch(() => { hljsModule = null; hljsLoading = null; return null; });
  }
  return hljsLoading;
}

// 预加载 hljs：在模块加载时立即开始异步加载，消除首次代码块渲染的等待
if (typeof window !== 'undefined') {
  ensureHLJS();
  // KaTeX 同样预加载：否则首次渲染公式时 katexModule 尚未就绪，
  // postProcessMath 只能把占位符还原为原文且该结果会被缓存
  ensureKatex();
}

  async function highlightCodeBlocks(container: HTMLElement): Promise<void> {
    const codeBlocks = container.querySelectorAll('pre code');
    if (!codeBlocks.length) return;

    // 预热 hljs（异步非阻塞）
    const hljsReady = ensureHLJS();

    const promises: Promise<void>[] = [];

    for (const codeEl of Array.from(codeBlocks)) {
      const code = codeEl as HTMLElement;
      const pre = code.parentElement;
      if (!pre) continue;
      if (pre.dataset.highlighted) continue;
      if (code.querySelector('span.mtk') || code.querySelector('span.hljs')) {
        pre.dataset.highlighted = 'true';
        continue;
      }

      let lang = 'plaintext';
      if (pre.dataset.lang) {
        lang = pre.dataset.lang;
      } else {
        const match = code.className.match(/language-(\S+)/);
        if (match && match[1]) lang = match[1];
      }

      if (lang === 'mermaid') {
        // ... mermaid handling kept unchanged
        promises.push(
          import('mermaid').then(({ default: mermaid }) => {
            if (!mermaidInitialized) {
              // 一律用 base + 主题变量：扩展主题（暖纸 / 杂志）不该长出 macOS 蓝的流程图
              mermaid.initialize({ startOnLoad: false, theme: 'base', themeVariables: mermaidThemeVariables() });
              mermaidInitialized = true;
            }
            const id = `mermaid-${Date.now()}-${Math.random().toString(36).slice(2)}`;
            const source = code.textContent || '';
            const genAtStart = themeGeneration;
            return mermaid.render(id, source).then(async ({ svg }) => {
              const wrapper = document.createElement('div');
              wrapper.className = 'mermaid-diagram';
              // 存**源码**而不是存结果：SVG 里的 fill/stroke 是渲染当时按 CSS 变量快照烤死的。
              // 换主题后既改不动那张图，也不能靠"重排整篇"绕过去 ——
              // 同一段内容产出的 HTML 一模一样，v-html 判定无变化就完全不 patch（实测踩过）。
              wrapper.dataset.mermaidSource = source;
              wrapper.innerHTML = sanitizeMermaidSvg(svg);
              pre.replaceWith(wrapper);
              // 这张图在飞的期间主题换过：换主题那一趟的重绘找不到节点（那时图还没进 DOM），
              // 所以由画完的这里补一次 —— 否则那次切换就永远丢了（组件测试跑红暴露的竞态）。
              if (genAtStart !== themeGeneration) await repaintNode(wrapper, mermaid);
            }).catch(() => {});
          }).catch(() => {})
        );
        continue;
      }

      // 内容嵌入(Embed)：```embed\nurl\n```
      if (lang === 'embed') {
        const rawUrl = (code.textContent || '').trim();
        const embedWrapper = createEmbedWidget(rawUrl);
        pre.replaceWith(embedWrapper);
        continue;
      }



      const rawCode = code.dataset.rawCode || code.textContent || '';
      pre.dataset.rawCode = rawCode;

      // hljs 的 shell/console 语法是"终端会话"高亮（只认 $ 提示符行，其余按输出零着色），
      // 普通 shell 脚本用它几乎全黑——内容不是会话形态时改用 bash 语法（语言标签仍显示原样）
      let highlightLang = lang;
      if ((lang === 'shell' || lang === 'console' || lang === 'shellsession') && !/^\s*\$\s/m.test(rawCode)) {
        highlightLang = 'bash';
      }

      const promise = (async () => {
        // 直接使用 highlight.js（优先保证有高亮），Monaco colorize 仅作增强备选
        const hljs = await hljsReady;
        const highlightedHtml = hljs
          ? (hljs.getLanguage(highlightLang) ? hljs.highlight(rawCode, { language: highlightLang }).value : hljs.highlightAuto(rawCode).value)
          : rawCode.replace(/[<>&]/g, c => ({ '<': '&lt;', '>': '&gt;', '&': '&amp;' }[c] || c));

        const processedLines = highlightedHtml.replace(/<br\s*\/?>/g, '\n').split('\n');
        while (processedLines.length > 0 && processedLines[processedLines.length - 1] === '') {
          processedLines.pop();
        }
        const totalLines = processedLines.length;
        const numWidth = String(totalLines).length;

        const numberedLines = processedLines.map((line, i) => {
          const num = String(i + 1).padStart(numWidth, '\u00a0');
          return `<div class="code-line"><span class="line-number" data-num="${num}">${num}</span><span class="code-content">${line || '\u00a0'}</span></div>`;
        }).join('');

        code.innerHTML = DOMPurify.sanitize(numberedLines, {
          ALLOWED_TAGS: ['div', 'span', 'br', 'a'],
          ALLOWED_ATTR: ['class', 'style', 'data-num'],
        });
        pre.dataset.highlighted = 'true';
        pre.className = 'code-block';
        wrapCodeBlock(pre, lang, rawCode);
      })();
      promises.push(promise);
    }

    await Promise.all(promises);
  }

  function removeCodeBlockListeners(container: HTMLElement): void {
    container.querySelectorAll('.code-copy-btn').forEach((btn) => {
      const clone = btn.cloneNode(true);
      btn.parentNode?.replaceChild(clone, btn);
    });
  }

  return { render, highlightCodeBlocks, removeCodeBlockListeners };
}


function wrapCodeBlock(pre: HTMLElement, lang: string, rawCode: string): void {
  if (pre.parentElement?.classList.contains('code-block-wrapper')) return;

  const wrapper = document.createElement('div');
  wrapper.className = 'code-block-wrapper';

  const header = document.createElement('div');
  header.className = 'code-block-header';

  const langSpan = document.createElement('span');
  langSpan.className = 'code-block-lang';
  langSpan.textContent = lang;

  const copyBtn = document.createElement('button');
  copyBtn.className = 'code-copy-btn';
  copyBtn.textContent = '\u590d\u5236';
  copyBtn.addEventListener('click', () => {
    const onSuccess = () => {
      copyBtn.textContent = '\u5df2\u590d\u5236';
      copyBtn.classList.add('copied');
      setTimeout(() => {
        copyBtn.textContent = '\u590d\u5236';
        copyBtn.classList.remove('copied');
      }, 1500);
    };
    copyText(rawCode).then((ok) => {
      if (ok) onSuccess();
    });
  });

  header.appendChild(langSpan);
  header.appendChild(copyBtn);

  // P1-25: 异步高亮完成时 DOM 可能已被 v-html 整体替换，pre 已脱离文档则跳过，避免空断言抛 TypeError
  if (!pre.parentElement) return;
  pre.parentElement.insertBefore(wrapper, pre);
  wrapper.appendChild(header);
  wrapper.appendChild(pre);
}
