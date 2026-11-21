export interface PositionResult {
  top: string;
  left: string;
}

export interface Viewport {
  width: number;
  height: number;
}

function defaultViewport(): Viewport {
  return { width: window.innerWidth, height: window.innerHeight };
}

/** 夹取到 [minBound, viewport - size - minBound]；内容比视口大时退化为只保住下限 */
function clamp(value: number, size: number, bound: number, extent: number): number {
  const max = extent - size - bound;
  if (max < bound) return bound;
  return Math.min(Math.max(value, bound), max);
}

export function calcPosition(
  triggerRect: DOMRect,
  contentRect: DOMRect,
  placement: 'top' | 'bottom' | 'left' | 'right',
  gap: number = 8,
  minBound: number = 8,
  viewport: Viewport = defaultViewport(),
): PositionResult {
  let top = 0;
  let left = 0;

  switch (placement) {
    case 'top':
      top = triggerRect.top - contentRect.height - gap;
      left = triggerRect.left + (triggerRect.width - contentRect.width) / 2;
      break;
    case 'bottom':
      top = triggerRect.bottom + gap;
      left = triggerRect.left + (triggerRect.width - contentRect.width) / 2;
      break;
    case 'left':
      top = triggerRect.top + (triggerRect.height - contentRect.height) / 2;
      left = triggerRect.left - contentRect.width - gap;
      break;
    case 'right':
      top = triggerRect.top + (triggerRect.height - contentRect.height) / 2;
      left = triggerRect.right + gap;
      break;
  }

  // 只夹下限会让靠近右/下边缘的浮层溢出视口且无法取回，这里两侧都夹
  return {
    top: `${clamp(top, contentRect.height, minBound, viewport.height)}px`,
    left: `${clamp(left, contentRect.width, minBound, viewport.width)}px`,
  };
}
