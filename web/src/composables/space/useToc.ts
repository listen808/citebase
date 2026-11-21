import { ref, shallowRef, watch, nextTick, onMounted, onBeforeUnmount, type Ref, type ComputedRef } from 'vue';

export interface TocItem {
  id: string;
  text: string;
  level: number;
}

type AnyRef<T> = Ref<T> | ComputedRef<T> | (() => T);

function unwrap<T>(val: AnyRef<T>): T {
  return typeof val === 'function' ? (val as () => T)() : val.value;
}

export function useToc(
  contentRef: AnyRef<HTMLElement | null>,
  previewRef: AnyRef<{ headings?: TocItem[] } | null>,
  currentContent: AnyRef<string>,
) {
  const tocItems = ref<TocItem[]>([]);
  const activeHeadingId = shallowRef('');

  function scrollToHeading(id: string) {
    const container = unwrap(contentRef);
    if (!container) return;

    const elementById = container.querySelector(`#${CSS.escape(id)}`) as HTMLElement;
    if (elementById) {
      elementById.scrollIntoView({ behavior: 'smooth', block: 'start' });
      return;
    }

    const tocItem = tocItems.value.find(item => item.id === id);
    if (!tocItem) return;

    const allHeadings = container.querySelectorAll('h1, h2, h3, h4, h5, h6');

    for (const h of Array.from(allHeadings)) {
      const headingEl = h as HTMLElement;
      const headingText = headingEl.textContent?.trim() || '';
      const tocText = tocItem.text.trim();

      if (headingText.includes(tocText) || tocText.includes(headingText)) {
        headingEl.scrollIntoView({ behavior: 'smooth', block: 'start' });
        return;
      }
    }
  }

  function updateActiveHeading() {
    const container = unwrap(contentRef);
    if (!container) return;
    const headings = container.querySelectorAll('h1[id], h2[id], h3[id], h4[id], h5[id], h6[id]');
    if (!headings.length) return;
    const containerRect = container.getBoundingClientRect();
    const offset = 20;
    let current = '';
    for (const h of Array.from(headings)) {
      const rect = (h as HTMLElement).getBoundingClientRect();
      if (rect.top - containerRect.top <= offset) current = h.id;
      else break;
    }
    if (current && current !== activeHeadingId.value) activeHeadingId.value = current;
  }

  let scrollListener: (() => void) | null = null;
  let lastContainer: HTMLElement | null = null;
  let scrollRafId: number | null = null;

  function setupToc() {
    const previewHeadings = unwrap(previewRef)?.headings;
    if (previewHeadings && previewHeadings.length > 0) {
      tocItems.value = previewHeadings.map(h => ({ id: h.id, text: h.text, level: h.level }));
    } else {
      tocItems.value = [];
    }
    activeHeadingId.value = '';
    nextTick(() => {
      const container = unwrap(contentRef);
      if (container) {
        cleanupToc();
        lastContainer = container;
        scrollListener = () => {
          if (scrollRafId !== null) return;
          scrollRafId = requestAnimationFrame(() => {
            updateActiveHeading();
            scrollRafId = null;
          });
        };
        container.addEventListener('scroll', scrollListener, { passive: true });
      }
    });
  }

  function cleanupToc() {
    if (scrollRafId !== null) {
      cancelAnimationFrame(scrollRafId);
      scrollRafId = null;
    }
    if (scrollListener) {
      (lastContainer || unwrap(contentRef))?.removeEventListener('scroll', scrollListener);
      scrollListener = null;
      lastContainer = null;
    }
  }

  onBeforeUnmount(cleanupToc);

  onMounted(() => {
    nextTick(() => { setupToc(); });
  });

  watch(() => unwrap(currentContent), () => {
    cleanupToc();
    nextTick(() => { setupToc(); });
  });

  watch(() => unwrap(previewRef)?.headings, () => {
    const h = unwrap(previewRef)?.headings;
    if (h && h.length > 0) {
      tocItems.value = h.map(item => ({ id: item.id, text: item.text, level: item.level }));
      // 如果滚动监听器尚未附加（例如从非空间路由进入时 contentRef 在 mount 时为 null），重新设置
      if (!scrollListener) {
        nextTick(() => { setupToc(); });
      }
    }
  });

  return {
    tocItems,
    activeHeadingId,
    scrollToHeading,
  };
}
