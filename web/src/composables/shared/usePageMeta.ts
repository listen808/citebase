import { onBeforeUnmount, watch } from 'vue';
import { applySiteMetaDescription } from '@/utils/core/site-brand';

type MetaValue = string | undefined | null;

function upsertMeta(selector: string, attr: 'name' | 'property', key: string, value: MetaValue) {
  let el = document.head.querySelector<HTMLMetaElement>(selector);
  if (!value) {
    el?.remove();
    return;
  }
  if (!el) {
    el = document.createElement('meta');
    el.setAttribute(attr, key);
    document.head.appendChild(el);
  }
  el.setAttribute('content', value);
}

/**
 * 分享元信息（Open Graph + canonical）。
 *
 * 说清楚它能做到哪一步：它服务的是**会执行 JS 的客户端**（站内"复制链接"后的预览、
 * 部分支持渲染的抓取器、以及浏览器标签页本身）。而 Facebook/LinkedIn 这类抓取器
 * 不执行 JS，它们要的是服务端把 meta 直接写进 /share/:id 的 HTML —— 那一步在 Go 侧
 * 的 SPA 兜底 handler 里，不在这个 composable 的能力范围内。
 * 在此之前，全站是彻底的 0 实现：分享链接连标题都读不到。
 */
export function usePageMeta(source: () => {
  title?: MetaValue;
  description?: MetaValue;
  image?: MetaValue;
  url?: MetaValue;
}): void {
  const applied: Array<{ selector: string; attr: 'name' | 'property'; key: string }> = [];

  function apply() {
    const { title, description, image, url } = source() || {};
    const pairs: Array<[string, 'name' | 'property', string, MetaValue]> = [
      ['meta[property="og:title"]', 'property', 'og:title', title],
      ['meta[property="og:description"]', 'property', 'og:description', description],
      ['meta[property="og:type"]', 'property', 'og:type', title ? 'article' : undefined],
      ['meta[property="og:image"]', 'property', 'og:image', image],
      ['meta[property="og:url"]', 'property', 'og:url', url],
      ['meta[name="twitter:card"]', 'name', 'twitter:card', title ? 'summary_large_image' : undefined],
      ['meta[name="description"]', 'name', 'description', description],
    ];
    for (const [selector, attr, key, value] of pairs) {
      if (!applied.some((x) => x.selector === selector)) applied.push({ selector, attr, key });
      upsertMeta(selector, attr, key, value);
    }
    // canonical 必须是绝对 URL：相对路径在抓取器眼里等于没有
    const link = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]');
    if (url) {
      const abs = new URL(url, window.location.origin).href;
      if (!link) {
        const created = document.createElement('link');
        created.rel = 'canonical';
        document.head.appendChild(created);
        created.href = abs;
      } else {
        link.href = abs;
      }
    } else {
      link?.remove();
    }
  }

  apply();
  const stop = watch(source, apply, { deep: true });

  onBeforeUnmount(() => {
    stop();
    // 分享页之外不该留着上一篇文章的 og 标签：SPA 不刷新，不清就是过期数据。
    // `description` 不在此列 —— 一并清掉的结果是这个页面此后再也没有描述
    // （比"上一篇的"更糟），所以清完落回站点自己的那一句。
    for (const { selector } of applied) document.head.querySelector(selector)?.remove();
    document.head.querySelector('link[rel="canonical"]')?.remove();
    applySiteMetaDescription();
  });
}
