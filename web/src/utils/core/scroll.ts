/**
 * 列表/页面内容滚动容器回到顶部。
 * 真实滚动容器是 MainLayout 的 [data-ap-scroll]（页面内 overflowY:auto），window 本身不滚动，
 * 因此翻页后必须显式回顶，否则用户停留在长列表底部看不到第一页。
 */
export function scrollContentToTop(behavior: ScrollBehavior = 'smooth'): void {
  if (typeof document === 'undefined') return;
  document.querySelectorAll('[data-ap-scroll]').forEach((el) => el.scrollTo({ top: 0, behavior }));
  window.scrollTo({ top: 0, behavior });
}
