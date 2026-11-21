import { AppleZIndex } from '@/utils/core/apple-tokens';
import { computed, type Ref } from 'vue';

interface BoundaryOptions {
  margin?: number;
}

export function useViewportBoundary(
  triggerRef: Ref<HTMLElement | null>,
  contentRef: Ref<HTMLElement | null>,
  options: BoundaryOptions = {}
) {
  const { margin = 8 } = options;

  const style = computed(() => {
    const trigger = triggerRef.value;
    const content = contentRef.value;
    if (!trigger || !content) return {};

    const rect = trigger.getBoundingClientRect();
    const contentRect = content.getBoundingClientRect();
    const vw = window.innerWidth;
    const vh = window.innerHeight;

    let top = rect.bottom + margin;
    let left = rect.left;

    if (top + contentRect.height > vh - margin) {
      top = rect.top - contentRect.height - margin;
    }
    if (left + contentRect.width > vw - margin) {
      left = vw - contentRect.width - margin;
    }
    if (left < margin) {
      left = margin;
    }

    return { position: 'fixed' as const, top: `${top}px`, left: `${left}px`, zIndex: AppleZIndex.dropdown };
  });

  return { style };
}

export function clampPosition(x: number, y: number, width: number, height: number, margin = 8) {
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  // 右键菜单默认指针作左上角向下展开；下方放不下时向上翻——让菜单"底边"落在指针处（top = y - height），
  // 而不是把 top 抬到指针上方（那样需要比 height 还多的上方空间，短视口仍会裁）。
  // 只有菜单比视口还高（y - height < margin 且 vh < height + 2*margin）才退化为顶部对齐保顶部。
  let top = y;
  if (y + height > vh - margin) {
    if (y - height >= margin) top = y - height;
    else top = Math.max(margin, vh - height - margin);
  }
  return {
    x: Math.min(Math.max(x, margin), vw - width - margin),
    y: top,
  };
}
