<template>
  <!-- eslint-disable-next-line vue/no-v-html -- 内容已由 DOMPurify 深度消毒（白名单标签/属性/协议/CSS），markdown 渲染必须注入 HTML -->
  <div ref="previewRef" class="md-preview" v-html="renderedHtml" />
  <ApImageLightbox v-model="imagePreviewOpen" :items="previewItems" />
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { ref, computed, watch, nextTick, shallowRef, onMounted, onUnmounted } from 'vue';
import { useRouter } from 'vue-router';
import { useMarkdownRenderer, clearRenderCache, extractAndApplyThemeColors, setWikiLinkResolver, repaintMermaid, bumpThemeGeneration } from '@/composables/editor/useMarkdownRenderer';
import { AppleType, AppleSpace, AppleRadius } from '@/utils/core/apple-tokens';
import ApImageLightbox from '@/components/apple/ApImageLightbox.vue';
import { useDocumentStore } from '@/stores/workspace/document';
import { useUIStore } from '@/stores/app/ui';
import { copyText } from '@/utils/clipboard';

const props = defineProps<{
  content: string;
}>();
const { t } = useI18n();

const router = useRouter();
const ui = useUIStore();
const docStore = useDocumentStore();
const previewRef = ref<HTMLElement | null>(null);
const { render, highlightCodeBlocks } = useMarkdownRenderer();

const renderedHtml = shallowRef('');
const headings = shallowRef<Array<{ id: string; text: string; level: number }>>([]);
const previewSrc = shallowRef('');

// 添加渲染结果缓存
const renderCache = new Map<string, { html: string; headings: Array<{ id: string; text: string; level: number }>; pending: boolean }>();

// token 取消机制：快速连续切换内容时，只有最新一次高亮会执行
let highlightToken = 0;

function handleImageClick(e: MouseEvent) {
  const target = e.target as HTMLElement;
  if (target.tagName === 'IMG') {
    const src = (target as HTMLImageElement).src;
    if (src) {
      e.preventDefault();
      previewSrc.value = src;
    }
  }
}

// 文内目录/锚点跳转：# 开头链接平滑滚动到对应标题；
// 标题锚点按钮点击复制带 hash 的页面链接；
// 站内路由链接（/space/...）交给 SPA 导航（附件/图片直链除外，保持下载行为）
function handleLinkClick(e: MouseEvent) {
  const anchor = (e.target as HTMLElement).closest('a');
  if (!anchor) return;
  const href = anchor.getAttribute('href') || '';
  if (anchor.classList.contains('heading-anchor')) {
    e.preventDefault();
    const id = decodeURIComponent(href.slice(1));
    const url = `${location.origin}${location.pathname}${location.search}#${encodeURIComponent(id)}`;
    copyText(url);
    ui.showToast(t('apUi1.k31'), 'success');
    return;
  }
  if (href.startsWith('#')) {
    e.preventDefault();
    let id = href.slice(1);
    try {
      id = decodeURIComponent(id);
    } catch {
      /* 非法编码则按原值查找 */
    }
    const el = (previewRef.value?.querySelector(`#${CSS.escape(id)}`)
      || document.getElementById(id)) as HTMLElement | null;
    el?.scrollIntoView({ behavior: 'smooth', block: 'start' });
    return;
  }
  if (href.startsWith('/') && !href.startsWith('/images/') && !href.startsWith('/attachments/')) {
    e.preventDefault();
    void router.push(href);
  }
}

// 页面 URL 带 #锚点 时（如复制的标题链接被重新打开），渲染后自动滚动定位
function scrollToHash() {
  const hash = window.location.hash;
  if (!hash || hash.length < 2) return;
  const raw = hash.slice(1);
  let id = raw;
  try {
    id = decodeURIComponent(raw);
  } catch {
    /* 非法编码则按原值查找 */
  }
  const el = (previewRef.value?.querySelector(`#${CSS.escape(id)}`)
    || document.getElementById(id)) as HTMLElement | null;
  el?.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

// 灯箱的 Esc / 焦点收口 / 遮罩都在 ApImageLightbox 里，这里只管"有没有图在看"
const imagePreviewOpen = computed({
  get: () => !!previewSrc.value,
  set: (open: boolean) => { if (!open) previewSrc.value = ''; },
});
const previewItems = computed(() => (previewSrc.value ? [{ src: previewSrc.value }] : []));

async function applyHighlighting() {
  const token = ++highlightToken;
  await nextTick();
  // 如果期间有更新的内容到达，放弃本次高亮
  if (token !== highlightToken) return;
  if (!previewRef.value) return;
  try {
    await highlightCodeBlocks(previewRef.value!);
  } catch {
    // 忽略高亮错误
  }
  // 异步等待结束后再次检查 token
  if (token !== highlightToken) return;
  // #5 表格排序/筛选增强
  enhanceTables(previewRef.value!);
}

// ============================================================
// #5 表格排序与筛选增强
// ============================================================

/** 表格增强状态记录（用于清理） */
const enhancedTables: Array<{ cleanup: () => void }> = [];

/** 为容器内所有 table 添加排序和筛选能力 */
function enhanceTables(container: HTMLElement) {
  // 清理之前的增强
  for (const t of enhancedTables) t.cleanup();
  enhancedTables.length = 0;

  const tables = container.querySelectorAll('table');
  tables.forEach((tableEl) => {
    const table = tableEl as HTMLTableElement;
    const wrapper = table.closest('.table-wrapper') as HTMLElement | null;
    if (!wrapper) return; // 没有包装器则跳过
    if (wrapper.dataset.enhanced) return; // 已增强
    wrapper.dataset.enhanced = '1';

    const thead = table.querySelector('thead');
    const headerCells = thead?.querySelectorAll('th') ?? table.querySelectorAll('tr:first-child th');
    if (headerCells.length === 0) return;

    // 排序状态
    let sortCol = -1;
    let sortAsc = true;

    // 创建工具栏
    const toolbar = document.createElement('div');
    toolbar.style.cssText = `display:flex;gap:${AppleSpace.sm};align-items:center;margin-bottom:6px;font-size:${AppleType.microUI.size};`;

    // 筛选输入
    const filterInput = document.createElement('input');
    filterInput.type = 'text';
    filterInput.placeholder = t('apUi1.k27');
    filterInput.style.cssText = `flex:1;max-width:240px;padding:${AppleSpace.xs} ${AppleSpace.sm};font-size:${AppleType.microUI.size};border:1px solid var(--apple-border-light, var(--apple-soft-border-gray));border-radius:${AppleRadius['6px']};outline:none;background:transparent;color:inherit;`;
    filterInput.style.display = 'none';

    // 筛选切换按钮
    const filterBtn = document.createElement('button');
    filterBtn.textContent = t('apUi1.filter');
    filterBtn.style.cssText = `padding:2px 10px;font-size:${AppleType.microUI.size};border:1px solid var(--apple-border-light, var(--apple-soft-border-gray));border-radius:${AppleRadius['6px']};background:transparent;color:inherit;cursor:pointer;`;
    let filterVisible = false;
    // handler 先存变量再注册，清理时才能 removeEventListener（用新箭头函数去删等于没删）。
    const filterBtnHandler = () => {
      filterVisible = !filterVisible;
      filterInput.style.display = filterVisible ? 'block' : 'none';
      if (!filterVisible) {
        filterInput.value = '';
        applyFilter('');
      }
    };

    // 重置按钮
    const resetBtn = document.createElement('button');
    resetBtn.textContent = t('apUi1.reset');
    resetBtn.style.cssText = `padding:2px 10px;font-size:${AppleType.microUI.size};border:1px solid var(--apple-border-light, var(--apple-soft-border-gray));border-radius:${AppleRadius['6px']};background:transparent;color:inherit;cursor:pointer;`;
    const resetBtnHandler = () => {
      sortCol = -1;
      applySort();
      filterInput.value = '';
      applyFilter('');
    };
    const filterInputHandler = () => applyFilter(filterInput.value);

    filterBtn.addEventListener('click', filterBtnHandler);
    resetBtn.addEventListener('click', resetBtnHandler);

    toolbar.appendChild(filterBtn);
    toolbar.appendChild(filterInput);
    toolbar.appendChild(resetBtn);
    wrapper.insertBefore(toolbar, table);

    // 排序：点击表头
    const thClickHandlers: Array<{ th: Element; handler: () => void }> = [];
    headerCells.forEach((th, idx) => {
      const thEl = th as HTMLElement;
      thEl.style.cursor = 'pointer';
      thEl.style.userSelect = 'none';
      thEl.title = t('apUi1.k30');

      const handler = () => {
        if (sortCol === idx) {
          sortAsc = !sortAsc;
        } else {
          sortCol = idx;
          sortAsc = true;
        }
        applySort();
        updateSortIndicators();
      };
      thEl.addEventListener('click', handler);
      thClickHandlers.push({ th: thEl, handler });
    });

    // 排序指示器
    function updateSortIndicators() {
      headerCells.forEach((th, idx) => {
        const thEl = th as HTMLElement;
        // 移除旧指示器
        const old = thEl.querySelector('.sort-indicator');
        if (old) old.remove();
        if (idx === sortCol) {
          const indicator = document.createElement('span');
          indicator.className = 'sort-indicator';
          indicator.textContent = sortAsc ? ' ▲' : ' ▼';
          indicator.style.cssText = `font-size:${AppleType.legalMeta.size};opacity:0.6;`;
          thEl.appendChild(indicator);
        }
      });
    }

    // 执行排序
    function applySort() {
      const tbody = table.querySelector('tbody');
      if (!tbody || sortCol < 0) return;
      const rows = Array.from(tbody.querySelectorAll('tr'));
      // 考虑 rowspan 的影响：被合并的行跳过该列
      const getValue = (row: HTMLTableRowElement, col: number): string => {
        const cells = row.querySelectorAll('td');
        if (col < cells.length) {
          return cells[col]!.textContent?.trim() ?? '';
        }
        return '';
      };
      rows.sort((a, b) => {
        const va = getValue(a, sortCol);
        const vb = getValue(b, sortCol);
        // 尝试数值比较
        const na = parseFloat(va);
        const nb = parseFloat(vb);
        if (!isNaN(na) && !isNaN(nb) && va !== '' && vb !== '') {
          return sortAsc ? na - nb : nb - na;
        }
        return sortAsc ? va.localeCompare(vb, 'zh') : vb.localeCompare(va, 'zh');
      });
      rows.forEach((r) => tbody.appendChild(r));
    }

    // 执行筛选
    function applyFilter(keyword: string) {
      const tbody = table.querySelector('tbody');
      if (!tbody) return;
      const kw = keyword.toLowerCase().trim();
      tbody.querySelectorAll('tr').forEach((row) => {
        if (!kw) {
          (row as HTMLElement).style.display = '';
          return;
        }
        const text = row.textContent?.toLowerCase() ?? '';
        (row as HTMLElement).style.display = text.includes(kw) ? '' : 'none';
      });
    }

    filterInput.addEventListener('input', filterInputHandler);

    // 记录清理函数
    enhancedTables.push({
      cleanup: () => {
        thClickHandlers.forEach(({ th, handler }) => th.removeEventListener('click', handler));
        filterBtn.removeEventListener('click', filterBtnHandler);
        filterInput.removeEventListener('input', filterInputHandler);
        resetBtn.removeEventListener('click', resetBtnHandler);
        if (toolbar.parentNode) toolbar.parentNode.removeChild(toolbar);
        delete wrapper.dataset.enhanced;
      },
    });
  });
}

function updateRenderedContent(content: string) {
  const trimmedContent = content || '';
  
  // 检查缓存
  if (renderCache.has(trimmedContent)) {
    const cached = renderCache.get(trimmedContent)!;
    renderedHtml.value = cached.html;
    headings.value = cached.headings;
    applyHighlighting();
    window.setTimeout(scrollToHash, 400);
    return;
  }
  
  // 渲染新内容
  const result = render(trimmedContent);
  renderedHtml.value = result.html;
  headings.value = result.headings;

  // 缓存结果（保持缓存大小合理）；KaTeX 未就绪的降级渲染不缓存
  if (!result.pending) {
    if (renderCache.size >= 50) {
      const firstKey = renderCache.keys().next().value;
      if (firstKey) {
        renderCache.delete(firstKey);
      }
    }
    renderCache.set(trimmedContent, result);
  }

  applyHighlighting();
  window.setTimeout(scrollToHash, 400);
}


// 立即渲染，不再有 300ms 延迟
watch(() => props.content, (newVal) => {
  updateRenderedContent(newVal);
}, { immediate: true, flush: 'sync' });

// 监听主题变化：重新注入 Monaco 代码块主题色样式表，不重新高亮代码块内容。
// 代码块颜色通过 CSS 变量过渡自动平滑切换，避免闪屏。
let themeObserver: MutationObserver | null = null;

onMounted(() => {
  // wiki 双链解析：在已加载的文档树索引中按名称查找
  setWikiLinkResolver((name: string): string | null => {
    const node = docStore.findNodeByName(name);
    if (!node) return null;
    const sid = (node as { space_id?: number }).space_id || Number(docStore.currentSpaceId) || 0;
    if (!sid) return null;
    return `/space/${sid}/doc/${node.id}`;
  });
  /**
   * 主题变了必须**重画** mermaid：SVG 的 fill/stroke 是 `mermaid.render()` 当时烤进去的，
   * 而 `mermaidThemeVariables()` 读的是 CSS 变量的一次快照 —— 原先这个 observer 只清缓存 +
   * 重注入代码块配色，DOM 里那张图会一直停在旧配色，直到内容变一次才被动更新
   * （切到深色，流程图还是浅底；用户看到的是"主题没生效"）。
   *
   * 走 `repaintMermaid` 而不是"重排整篇"：同一段内容渲染出的 HTML 一模一样，
   * `v-html` 判定无变化就一次 patch 都不做 —— 重排等于白付一次整篇渲染的钱，图上还是一个像素没变
   * （这一条是打点打出来的：第二次进 updateRenderedContent 时容器里 `pre` 是 0 个、旧图还在）。
   *
   * **合并判据取的是"当前三个属性的签名"，不是"回调来了几次"**：一次主题操作会连着写
   * 外观 / 材质 / 透明变体三个属性，按回调次数去重就等于依赖 MutationObserver 的派发时机 ——
   * 规范是同一微任务里的多条记录合并成一次回调，happy-dom 是一条记录派一次，
   * 于是"改三个属性只重画一次"这条判据在测试环境里测的是环境而不是代码（实测两种写法各红一次）。
   * 改成队列尾部比对签名，两种派发节奏下都恰好重画一次。
   */
  const themeSignature = () => {
    const at = document.documentElement;
    return `${at.getAttribute('data-theme')}|${at.getAttribute('data-material')}|${at.getAttribute('data-glass')}`;
  };
  let redrawQueued = false;
  let lastDrawnSignature = themeSignature();
  const redrawMermaid = () => {
    if (redrawQueued) return;
    redrawQueued = true;
    queueMicrotask(async () => {
      redrawQueued = false;
      const sig = themeSignature();
      if (sig === lastDrawnSignature) return;
      lastDrawnSignature = sig;
      // 先 bump：那张还在飞的图发现自己跨了版本，画完会自己按新主题补一次
      // （没有这一步，"首渲染未完成时换主题"就是一次静默丢失 —— 组件测试跑红抓到的）
      bumpThemeGeneration();
      // 没有图的文档返回 0 张，一次 markdown 渲染都不掏 —— 判据不用再看围栏
      if (previewRef.value) await repaintMermaid(previewRef.value);
    });
  };
  themeObserver = new MutationObserver(() => {
    clearRenderCache();
    renderCache.clear();
    extractAndApplyThemeColors();
    redrawMermaid();
  });
  themeObserver.observe(document.documentElement, {
    attributes: true,
    attributeFilter: ['data-theme', 'data-material', 'data-glass'],
  });
  if (previewRef.value) {
    previewRef.value.addEventListener('click', handleImageClick);
    previewRef.value.addEventListener('click', handleLinkClick);
  }
});

onUnmounted(() => {
  setWikiLinkResolver(null);
  themeObserver?.disconnect();
  // #5 清理表格增强
  for (const t of enhancedTables) t.cleanup();
  enhancedTables.length = 0;
  if (previewRef.value) {
    previewRef.value.removeEventListener('click', handleImageClick);
    previewRef.value.removeEventListener('click', handleLinkClick);
  }
  renderCache.clear();
});

defineExpose({ headings });


</script>

<style scoped>
.md-preview {
  font-family: var(--apple-font-text);
  font-size: var(--apple-font-size-body, 16px);
  line-height: 1.7;
  color: var(--apple-text-primary);
  /* 正文里的长 URL 在窄屏会把段落撑宽：overflow-wrap 可继承，一处覆盖所有块级内容 */
  overflow-wrap: anywhere;
  padding: var(--apple-space-16px) var(--apple-space-24px);
  /* 正文阅读面**不铺自己的底色**：它就是那张纸（surface-primary）本身。
     上一轮把它归到"从属面"改成 surface-field，在 32px 的目录空态条上看着对，
     到这页却是把整篇正文压进一块比纸暗得多的板 —— 纸变成双层，字写在板里。
     凹陷档只给控件与小面（搜索框、分段轨、开关槽），不给阅读面。 */
  background: transparent;
  flex: 1;
  min-height: 0;
}

.md-preview :deep(h1) { font-size: 1.875em; margin: 2em 0 0.67em; border-bottom: 1px solid var(--apple-border-light); padding-bottom: 0.3em; color: var(--apple-text-primary); }
/* 首块不该带上边距：h1 的 2em 顶边距原本会把每篇文档的开头整段往下推，
   阅读区顶部因此凭空多出一条空白带（截图里标题上方那块灰带）。
   v-html 注入的节点没有 scoped 属性，所以必须把 :deep() 放在子组合符之后。 */
.md-preview > :deep(:first-child) { margin-top: 0; }
.md-preview > :deep(:last-child) { margin-bottom: 0; }
.md-preview :deep(h2) { font-size: 1.5em; margin: 1.5em 0 0.5em; border-bottom: 1px solid var(--apple-border-light); padding-bottom: 0.2em; color: var(--apple-text-primary); }
.md-preview :deep(h3) { font-size: 1.25em; margin: 1.2em 0 0.4em; color: var(--apple-text-primary); }
.md-preview :deep(h4) { font-size: 1em; margin: 1em 0 0.3em; color: var(--apple-text-primary); }
.md-preview :deep(h5) { font-size: 0.875em; margin: 1em 0; color: var(--apple-text-secondary); }
.md-preview :deep(h6) { font-size: 0.85em; margin: 1em 0; color: var(--apple-text-secondary); }

/* Editorial 主题排版签名：衬线大标题 + 首段首字下沉（正文保持无衬线，仅阅读装饰） */
:root[data-theme='editorial'] .md-preview :deep(h1),
:root[data-theme='editorial'] .md-preview :deep(h2) {
  font-family: var(--apple-font-display);
}
/* 刊头（masthead）：标题上方一条重墨线，题名下收一条细线——杂志封面的签名，
   层级靠线不靠影（主题 desc 的同一条判据） */
:root[data-theme='editorial'] .md-preview :deep(h1) {
  font-size: 2.3em;
  letter-spacing: -0.02em;
  border-bottom: 0.0625rem solid var(--apple-border);
  border-top: 0.25rem solid var(--apple-text-primary);
  padding-top: 0.35em;
}
:root[data-theme='editorial'] .md-preview :deep(h2) {
  border-bottom: none;
}
:root[data-theme='editorial'] .md-preview :deep(p:first-of-type)::first-letter {
  float: left;
  font-family: var(--apple-font-display);
  font-size: 3.2em;
  line-height: 0.85;
  padding-right: 0.12em;
  padding-top: 0.05em;
  font-weight: 700;
  color: var(--apple-blue);
}

/* ---- G7：媒体圆角与标题断行跟随主题 ---- */
.md-preview :deep(img) { border-radius: var(--apple-radius-medium); max-width: 100%; }
.md-preview :deep(h1), .md-preview :deep(h2), .md-preview :deep(h3) { text-wrap: balance; }

/* ---- glass 材质签名（正文里的内容块是浮在环境上的小玻璃片） ----
   材质轴：light-glass 与 dark-glass 都吃这套签名。
   只对块级容器做玻璃，不嵌套 backdrop-filter（红线：玻璃层 ≤3） */
:root[data-material='glass'] .md-preview :deep(.code-block-wrapper),
:root[data-material='glass'] .md-preview :deep(table),
:root[data-material='glass'] .md-preview :deep(blockquote),
:root[data-material='glass'] .md-preview :deep(img) {
  background: var(--apple-bg-secondary);
  border-color: var(--apple-border);
  border-radius: var(--apple-radius-card);
  box-shadow: var(--apple-shadow-card);
  /* 正文块（代码块/表格/引用/图片）是流水面里的块级内容：一篇长文一屏能出十几个，
     2026-10-26 起不付模糊（红线：同屏实时模糊 ≤3，模糊只留给壳层与真浮层）。
     「小玻璃片」的观感由上面的 bg-secondary + 描边 + 阴影表达，玻璃材质下依然成立。 */
}
:root[data-material='glass'] .md-preview :deep(table) { border-collapse: separate; border-spacing: 0; overflow: hidden; }
:root[data-material='glass'] .md-preview :deep(th) { background: var(--apple-bg-tertiary); }
:root[data-material='glass'] .md-preview :deep(code:not(pre code)) {
  background: var(--apple-bg-tertiary);
  border-radius: var(--apple-radius-control);
}
:root[data-material='glass'] .md-preview :deep(blockquote) { border-left-color: var(--apple-blue); }

/* ---- editorial：杂志细节（悬挂大引号 / 横线表格 + tabular-nums / h2 章节符 / 墨色图框） ---- */
:root[data-theme='editorial'] .md-preview :deep(blockquote) {
  border-left: none;
  padding-left: 2.2em;
  position: relative;
  font-style: italic;
}
:root[data-theme='editorial'] .md-preview :deep(blockquote)::before {
  content: '\201C';
  position: absolute;
  left: 0;
  top: -0.12em;
  font-family: var(--apple-font-display);
  font-size: 3em;
  line-height: 1;
  color: var(--apple-blue);
}
:root[data-theme='editorial'] .md-preview :deep(table th),
:root[data-theme='editorial'] .md-preview :deep(table td) {
  border: none;
  border-bottom: 0.0625rem solid var(--apple-border);
}
:root[data-theme='editorial'] .md-preview :deep(table) { border: none; }
:root[data-theme='editorial'] .md-preview :deep(table th) {
  font-size: 0.78em;
  background: transparent;
}
:root[data-theme='editorial'] .md-preview :deep(td) { font-variant-numeric: tabular-nums; }
/* 章节题走 kicker 语言：小体大写 + 加宽字距（Monocle / 经济学人的栏目题），
   § 保留在放大写之前做栏目符 */
:root[data-theme='editorial'] .md-preview :deep(h2) {
  font-size: 1em;
  font-weight: 700;
  text-transform: uppercase;
  letter-spacing: 0.08em;
  margin-top: 2.2em;
}
:root[data-theme='editorial'] .md-preview :deep(h2)::before {
  content: '§ ';
  color: var(--apple-blue);
}
:root[data-theme='editorial'] .md-preview :deep(img) {
  border: 0.0625rem solid var(--apple-text-primary);
  border-radius: 0;
}
/* 图注：小体 + 次级墨，与图框同族（杂志图注语言） */
:root[data-theme='editorial'] .md-preview :deep(figcaption) {
  font-size: 0.8em;
  color: var(--apple-text-secondary);
  margin-top: 0.4em;
}

.md-preview :deep(p) { margin: 0 0 var(--apple-space-16px); color: var(--apple-text-primary); }

.md-preview :deep(a) { color: var(--apple-apple-action-blue); text-decoration: none; }
.md-preview :deep(a:hover) { text-decoration: underline; }

.md-preview :deep(strong) { font-weight: 600; }
.md-preview :deep(em) { font-style: italic; }

.md-preview :deep(ul), .md-preview :deep(ol) { padding-left: 2em; margin: 0 0 var(--apple-space-16px); }
.md-preview :deep(li) { margin: 0.25em 0; }
.md-preview :deep(li > p) { margin-top: var(--apple-space-8px); }

/* GFM 任务列表：隐藏圆点，勾选框与文本对齐 */
.md-preview :deep(li.task-item) { list-style: none; margin-left: -1.4em; }
.md-preview :deep(li.task-item input[type='checkbox']) { margin-right: 0.5em; vertical-align: middle; accent-color: var(--apple-apple-action-blue); }

/* 标题 hover 锚点：默认透明，hover 标题时浮现 */
.md-preview :deep(.doc-heading) { position: relative; }
.md-preview :deep(.heading-anchor) {
  position: absolute;
  left: -1.1em;
  top: 50%;
  transform: translateY(-50%);
  color: var(--apple-text-tertiary);
  text-decoration: none;
  opacity: 0;
  transition: opacity var(--apple-dur-fast) ease;
  font-weight: 400;
  user-select: none;
}
.md-preview :deep(.doc-heading:hover .heading-anchor) { opacity: 1; }
.md-preview :deep(.heading-anchor:hover) { color: var(--apple-apple-action-blue); }

/* wiki 双链：未解析为虚线+橙色，提示待创建 */
.md-preview :deep(a.wikilink) { color: var(--apple-purple, var(--apple-apple-action-blue)); text-decoration: none; border-bottom: 1px dashed currentColor; }
.md-preview :deep(a.wikilink:hover) { text-decoration: none; filter: brightness(1.1); }
.md-preview :deep(a.wikilink-missing) { color: var(--apple-orange); }

/* 脚注 */
.md-preview :deep(.footnotes) { margin-top: var(--apple-space-24px); font-size: 0.9em; color: var(--apple-text-secondary); }
.md-preview :deep(.footnotes hr) { margin-top: var(--apple-space-24px); }
.md-preview :deep(.footnote-ref) { padding-left: 2px; }
.md-preview :deep(.footnote-ref a) { text-decoration: none; }
.md-preview :deep(.footnote-backref) { margin-left: 6px; text-decoration: none; color: var(--apple-apple-action-blue); }

.md-preview :deep(blockquote) {
  margin: 0 0 var(--apple-space-16px);
  padding: 0 1em;
  color: var(--apple-text-secondary);
  border-left: 0.25em solid var(--apple-border);
}
.md-preview :deep(blockquote > :first-child) { margin-top: 0; }
.md-preview :deep(blockquote > :last-child) { margin-bottom: 0; }

.md-preview :deep(code) {
  font-family: var(--apple-font-mono);
  font-size: 85%;
  padding: 0.2em 0.4em;
  margin: 0;
  background: var(--apple-bg-hover);
  border-radius: var(--apple-radius-small);
  color: var(--apple-text-primary);
}

/* 代码块容器 */
.md-preview :deep(.code-block-wrapper) {
  position: relative;
  margin: 0 0 var(--apple-space-16px);
  border-radius: var(--apple-radius-small);
  overflow: hidden;
  border: 1px solid var(--apple-border);
  background: var(--apple-bg-secondary);
}

/* 代码块头部（语言标签 + 复制按钮） */
.md-preview :deep(.code-block-header) {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: var(--apple-space-6px) var(--apple-space-16px);
  background: var(--apple-bg-tertiary);
  border-bottom: 1px solid var(--apple-border);
  font-size: var(--apple-font-size-caption);
  color: var(--apple-text-secondary);
}

.md-preview :deep(.code-block-lang) {
  font-weight: 500;
  font-family: var(--apple-font-text);
}

.md-preview :deep(.code-copy-btn) {
  padding: 2px 8px;
  border: 1px solid var(--apple-border);
  border-radius: var(--apple-radius-small);
  background: var(--apple-surface-field);
  color: var(--apple-text-secondary);
  font-size: var(--apple-font-size-caption);
  cursor: pointer;
  transition: var(--apple-transition-fast);
  line-height: 1.4;
}

.md-preview :deep(.code-copy-btn:hover) {
  background: var(--apple-bg-hover);
  color: var(--apple-text-primary);
}

.md-preview :deep(.code-copy-btn.copied) {
  color: var(--apple-green);
  border-color: var(--apple-green);
}

.md-preview :deep(.monaco-code-block),
.md-preview :deep(.code-block) {
  background: var(--apple-bg-secondary);
  border: none;
  border-radius: 0;
  padding: var(--apple-space-14px) 0;
  overflow-x: auto;
  font-size: var(--apple-font-size-caption);
  line-height: 1.6;
  margin: 0;
}

.md-preview :deep(.monaco-code-block code),
.md-preview :deep(.monaco-code-block span),
.md-preview :deep(.code-block code),
.md-preview :deep(.code-block span) {
  background: transparent !important;
  background-color: transparent !important;
}

.md-preview :deep(.monaco-code-block .code-content),
.md-preview :deep(.code-block .code-content) {
  font-family: var(--apple-font-mono);
  background: transparent;
  padding: 0;
  font-size: 100%;
  border-radius: 0;
  display: block;
  white-space: pre;
  tab-size: 4;
}

.md-preview :deep(table) {
  display: table;
  width: auto;
  min-width: 100%;
  table-layout: auto;
  border-collapse: collapse;
  margin: 0;
  border: 1px solid var(--apple-border-light);
  border-radius: var(--apple-radius-small);
  font-size: var(--apple-font-size-small, 14px);
}

/* 表格水平滚动容器：超出宽度时可滚动，列宽保持自然 */
.md-preview :deep(.table-wrapper),
.md-preview :deep(table) {
  max-width: 100%;
}

.md-preview :deep(.table-wrapper) {
  overflow: auto;
  max-height: 70vh;
  margin: 0 0 var(--apple-space-16px);
  border: 1px solid var(--apple-border-light);
  border-radius: var(--apple-radius-small);
}

.md-preview :deep(.table-wrapper table) {
  border: none;
  border-radius: 0;
  margin: 0;
}

/* 滚动条始终可见：方便长表格横向滚动 */
.md-preview :deep(.table-wrapper::-webkit-scrollbar) {
  height: 8px;
  background: transparent;
}

.md-preview :deep(.table-wrapper::-webkit-scrollbar-track) {
  background: var(--apple-bg-tertiary);
  border-radius: var(--apple-radius-micro, 4px);
}

.md-preview :deep(.table-wrapper::-webkit-scrollbar-thumb) {
  background: var(--apple-border-light);
  border-radius: var(--apple-radius-micro, 4px);
}

.md-preview :deep(.table-wrapper::-webkit-scrollbar-thumb:hover) {
  background: var(--apple-text-tertiary);
}

/* 支持原生 HTML 表格的 caption */
.md-preview :deep(caption) {
  padding: var(--apple-space-8px) var(--apple-space-12px);
  color: var(--apple-text-secondary);
  font-size: 0.875em;
  text-align: center;
  background: var(--apple-bg-tertiary);
  border-bottom: 1px solid var(--apple-border-light);
}

.md-preview :deep(table th),
.md-preview :deep(table td) {
  padding: 10px 14px;
  border: 1px solid var(--apple-border-light);
  vertical-align: middle;
  word-break: keep-all;
  white-space: nowrap;
  line-height: 1.5;
}

/* 支持 align 属性 */
.md-preview :deep(table th[align="center"]),
.md-preview :deep(table td[align="center"]) {
  text-align: center;
}

.md-preview :deep(table th[align="right"]),
.md-preview :deep(table td[align="right"]) {
  text-align: right;
}

.md-preview :deep(table th[align="left"]),
.md-preview :deep(table td[align="left"]) {
  text-align: left;
}

/* 支持 valign 属性 */
.md-preview :deep(table th[valign="top"]),
.md-preview :deep(table td[valign="top"]) {
  vertical-align: top;
}

.md-preview :deep(table th[valign="middle"]),
.md-preview :deep(table td[valign="middle"]) {
  vertical-align: middle;
}

.md-preview :deep(table th[valign="bottom"]),
.md-preview :deep(table td[valign="bottom"]) {
  vertical-align: bottom;
}

/* 支持 colspan 和 rowspan */
.md-preview :deep(table th[colspan]),
.md-preview :deep(table td[colspan]) {
  text-align: center;
  font-weight: 600;
}

.md-preview :deep(table th[rowspan]),
.md-preview :deep(table td[rowspan]) {
  vertical-align: middle;
}

.md-preview :deep(table th:last-child),
.md-preview :deep(table td:last-child) {
  border-right: 1px solid var(--apple-border-light);
}

.md-preview :deep(table tr:last-child th),
.md-preview :deep(table tr:last-child td) {
  border-bottom: 1px solid var(--apple-border-light);
}

.md-preview :deep(table th) {
  background: var(--apple-bg-tertiary);
  font-weight: 600;
  text-align: left;
  white-space: nowrap;
}

/* 支持 thead/tfoot 的背景色 */
.md-preview :deep(thead th) {
  background: var(--apple-bg-tertiary);
  position: sticky;
  top: 0;
  z-index: var(--apple-z-index-base);
}

.md-preview :deep(tfoot td) {
  background: var(--apple-bg-tertiary);
  font-weight: 500;
}

.md-preview :deep(table tr) {
  background: var(--apple-surface-panel);
  transition: background var(--apple-dur-fast) ease;
}

.md-preview :deep(table tr:nth-child(2n)) {
  background: var(--apple-bg-secondary);
}

.md-preview :deep(table tr:hover) {
  background: var(--apple-bg-hover);
}

/* 支持 col 和 colgroup */
.md-preview :deep(col) {
  /* col 元素本身不渲染，但可以通过 style 影响列 */
}

.md-preview :deep(colgroup) {
  /* colgroup 容器 */
}

.md-preview :deep(img) {
  max-width: 100%;
  box-sizing: border-box;
  margin: 0 0 var(--apple-space-16px);
}

.md-preview :deep(hr) {
  height: 0.25em;
  padding: 0;
  margin: var(--apple-space-24px) 0;
  background: var(--apple-border);
  border: 0;
}

/* KaTeX 数学公式 */
.md-preview :deep(.katex-block) {
  overflow-x: auto;
  padding: var(--apple-space-8px) 0;
  text-align: center;
}

.md-preview :deep(.katex) {
  font-size: 1.1em;
}

.md-preview :deep(.katex-error) {
  color: var(--apple-red);
  background: var(--apple-red-light);
  padding: var(--apple-space-2px) var(--apple-space-6px);
  border-radius: var(--apple-radius-small);
  font-family: var(--apple-font-mono);
}

/* Mermaid 图表 */
.md-preview :deep(.mermaid-diagram) {
  display: flex;
  justify-content: center;
  padding: var(--apple-space-16px) 0;
  background: var(--apple-bg-secondary);
  border-radius: var(--apple-radius-small);
  margin: 0 0 var(--apple-space-16px);
}

.md-preview :deep(.mermaid-diagram svg) {
  max-width: 100%;
  height: auto;
}

/* 内容嵌入(Embed) */
.md-preview :deep(.embed-widget) {
  margin: 0 0 var(--apple-space-16px);
}

.md-preview :deep(.embed-error) {
  padding: var(--apple-space-12px) var(--apple-space-16px);
  background: var(--apple-red-light);
  color: var(--apple-red);
  border-radius: var(--apple-radius-small);
  font-size: 0.875em;
}

.md-preview :deep(.embed-card) {
  padding: var(--apple-space-12px) var(--apple-space-16px);
  background: var(--apple-bg-secondary);
  border: 1px solid var(--apple-border-light);
  border-radius: var(--apple-radius-small);
  margin: 0 0 var(--apple-space-16px);
}



/* Callout/Admonition */
.md-preview :deep(.callout) {
  display: flex;
  gap: var(--apple-space-12px);
  padding: var(--apple-space-16px);
  border-radius: var(--apple-radius-small);
  margin: 0 0 var(--apple-space-16px);
  border-left: 3px solid;
}
.md-preview :deep(.callout-title) { font-size: 12px; font-weight: 600; letter-spacing: 0; line-height: 1.4; flex-shrink: 0; margin-bottom: 2px; }
.md-preview :deep(.callout-note .callout-title) { color: var(--apple-blue); }
.md-preview :deep(.callout-warning .callout-title) { color: var(--apple-orange); }
.md-preview :deep(.callout-tip .callout-title) { color: var(--apple-green); }
.md-preview :deep(.callout-danger .callout-title) { color: var(--apple-red); }
.md-preview :deep(.callout-info .callout-title) { color: var(--apple-text-secondary); }
.md-preview :deep(.callout-content) { flex: 1; min-width: 0; }
.md-preview :deep(.callout-content p) { margin: 0; }
.md-preview :deep(.callout-content p + p) { margin-top: var(--apple-space-8px); }
.md-preview :deep(.callout-note) { background: var(--apple-callout-note-bg); border-left-color: var(--apple-callout-note-border); }
.md-preview :deep(.callout-warning) { background: var(--apple-callout-warning-bg); border-left-color: var(--apple-callout-warning-border); }
.md-preview :deep(.callout-tip) { background: var(--apple-callout-tip-bg); border-left-color: var(--apple-callout-tip-border); }
.md-preview :deep(.callout-danger) { background: var(--apple-callout-danger-bg); border-left-color: var(--apple-callout-danger-border); }
.md-preview :deep(.callout-info) { background: var(--apple-callout-info-bg); border-left-color: var(--apple-callout-info-border); }

/* Toggle / Details block */
.md-preview :deep(.toggle-block) {
  margin: 0 0 var(--apple-space-16px);
  border: 1px solid var(--apple-border-light);
  border-radius: var(--apple-radius-small);
  overflow: hidden;
}
.md-preview :deep(.toggle-block summary) {
  padding: var(--apple-space-10px) var(--apple-space-16px);
  background: var(--apple-bg-secondary);
  cursor: pointer;
  font-weight: 600;
  font-size: 0.95em;
  color: var(--apple-text-primary);
  user-select: none;
  list-style: none;
  display: flex;
  align-items: center;
  gap: var(--apple-space-8px);
}
.md-preview :deep(.toggle-block summary::before) {
  content: '▶';
  font-size: var(--apple-font-size-nano, 10px);
  transition: transform var(--apple-dur-interactive) ease;
  flex-shrink: 0;
}
.md-preview :deep(.toggle-block[open] summary::before) {
  transform: rotate(90deg);
}
.md-preview :deep(.toggle-block summary::-webkit-details-marker) {
  display: none;
}
.md-preview :deep(.toggle-content) {
  padding: var(--apple-space-12px) var(--apple-space-16px);
  border-top: 1px solid var(--apple-border-light);
}
.md-preview :deep(.toggle-content p:first-child) {
  margin-top: 0;
}
.md-preview :deep(.toggle-content p:last-child) {
  margin-bottom: 0;
}

.md-preview :deep(.code-line) {
  display: flex;
  align-items: flex-start;
  line-height: 1.6;
  min-height: 1.6em;
}

/* Line numbers */
.md-preview :deep(.line-number) {
  display: inline-block;
  min-width: 2em;
  text-align: right;
  padding: 0 0.6em 0 0.4em;
  margin-right: 0.6em;
  color: var(--apple-text-quaternary);
  user-select: none;
  font-size: 0.9em;
  vertical-align: top;
  border-right: 1px solid var(--apple-border-light);
  position: relative;
}

/* Figure/Figcaption */
.md-preview :deep(figure) { margin: 0 0 var(--apple-space-16px); }
.md-preview :deep(figcaption) {
  text-align: center;
  font-size: 0.875em;
  color: var(--apple-text-tertiary);
  margin-top: var(--apple-space-8px);
}
</style>
