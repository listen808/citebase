import { onBeforeUnmount, watch } from 'vue';

/**
 * 浮层的 Esc 关闭。
 * 存在的理由：全站有 4 处自写浮层（图片灯箱两份、文档目录抽屉、AI 写作面板）
 * 既没有 `role=dialog` 也接不住 Esc —— 键盘用户能 Tab 进去却出不来。
 * ApModal / ApDrawer 自带这套（useFocusTrap），但它们是完整组件，
 * 那 4 处的形态各不相同，只能把"Esc 关掉"这一条抽出来共用。
 */
export function useEscapeClose(isOpen: () => boolean, close: () => void): void {
  function onKeydown(e: KeyboardEvent) {
    if (e.key !== 'Escape' || !isOpen()) return;
    // 有真弹窗在上面时让 ApModal 自己处理，别抢它的 Esc
    if (document.querySelector('[role="dialog"][aria-modal="true"]')) return;
    e.stopPropagation();
    close();
  }

  // 只由 isOpen 驱动：初始关着就不挂监听，开着才挂 —— 少一层"挂载时先加、关闭时删掉、
  // 再打开就没了"的顺序风险。
  watch(isOpen, (open) => {
    if (open) window.addEventListener('keydown', onKeydown, true);
    else window.removeEventListener('keydown', onKeydown, true);
  }, { immediate: true });

  onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown, true));
}
