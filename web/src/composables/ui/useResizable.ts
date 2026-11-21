import { ref } from 'vue';

/**
 * 侧栏/分栏拖拽。
 * 原先只挂 `mousemove`/`mouseup`：触屏与触控笔按下去没有任何反应，
 * 6px 的把手移出后还会丢事件。现在走 Pointer Events + `setPointerCapture`，
 * 鼠标、触屏、触控笔同一条通路，且指针越出把手仍然 1:1 跟手。
 */
export function useResizable(options: {
  initialWidth: number;
  minWidth: number;
  maxWidth: number;
  storageKey?: string;
}) {
  const stored = options.storageKey ? localStorage.getItem(options.storageKey) : null;
  const width = ref(stored ? parseInt(stored, 10) : options.initialWidth);
  let startX = 0;
  let startWidth = 0;
  let handle: HTMLElement | null = null;
  let pointerId = -1;

  function onDragMove(e: PointerEvent) {
    const delta = e.clientX - startX;
    width.value = Math.max(options.minWidth, Math.min(options.maxWidth, startWidth + delta));
  }

  function onDragEnd() {
    if (handle) {
      if (pointerId !== -1 && handle.hasPointerCapture?.(pointerId)) {
        try { handle.releasePointerCapture(pointerId); } catch { /* 已经释放了就算成功 */ }
      }
      handle.removeEventListener('pointermove', onDragMove);
      handle.removeEventListener('pointerup', onDragEnd);
      handle.removeEventListener('pointercancel', onDragEnd);
    }
    handle = null;
    pointerId = -1;
    document.body.style.cursor = '';
    document.body.style.userSelect = '';
    if (options.storageKey) {
      localStorage.setItem(options.storageKey, String(width.value));
    }
  }

  function onDragStart(e: PointerEvent) {
    handle = e.currentTarget as HTMLElement;
    e.preventDefault();
    startX = e.clientX;
    startWidth = width.value;
    pointerId = e.pointerId;
    try {
      handle.setPointerCapture(pointerId);
    } catch {
      // 捕获失败（极少数旧内核）就退回元素级监听，仍然能拖，只是越界会断
    }
    handle.addEventListener('pointermove', onDragMove);
    handle.addEventListener('pointerup', onDragEnd);
    handle.addEventListener('pointercancel', onDragEnd);
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
  }

  return { width, onDragStart };
}
