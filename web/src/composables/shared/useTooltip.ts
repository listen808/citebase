import { computed, nextTick, onScopeDispose, shallowRef, useId, type Ref } from 'vue';
import { calcPosition } from '@/utils/core/position';
import {
  AppleLineHeight, AppleRadius, AppleSpace, AppleType, AppleZIndex, cv,
} from '@/utils/core/apple-tokens';

export type TooltipPlacement = 'top' | 'bottom' | 'left' | 'right';

/**
 * 提示气泡的唯一实现：定位、延迟、role=tooltip、样式都只在这里有一份。
 *
 * 为什么不是「包装组件套一层 div」给按钮用（原来那个 ApTooltip 就是这么写的，已删）：ApButton 的 :style 与属性靠默认透传落在
 * <button> 本身，外面套一层 wrapper 会把透传接走，一排按钮的高度与间距当场散掉。
 * 所以按钮把自己当触发元素传进来，ApTooltip 把 wrapper 当触发元素传进来，共用同一套逻辑。
 */
export function useTooltip(
  triggerRef: Ref<HTMLElement | null>,
  bubbleRef: Ref<HTMLElement | null>,
  options: {
    content: () => string | undefined | null;
    placement?: () => TooltipPlacement;
    delay?: () => number;
  },
) {
  const visible = shallowRef(false);
  // 位置没算出来之前先 visibility:hidden，否则气泡会在左上角闪一帧
  const ready = shallowRef(false);
  const position = shallowRef({ top: '-9999px', left: '-9999px' });
  // 气泡与触发元素靠这个 id 连上 aria-describedby，读屏才会把提示当作描述文字念出来
  const bubbleId = useId();
  let timer: ReturnType<typeof setTimeout> | null = null;

  onScopeDispose(() => {
    if (timer) clearTimeout(timer);
    timer = null;
  });

  async function updatePosition() {
    const trigger = triggerRef.value;
    const bubble = bubbleRef.value;
    if (!trigger || !bubble) return;
    position.value = calcPosition(
      trigger.getBoundingClientRect(),
      bubble.getBoundingClientRect(),
      options.placement?.() ?? 'top',
      6,
      4,
    );
  }

  function show() {
    if (!options.content()) return;
    if (timer) clearTimeout(timer);
    timer = setTimeout(() => {
      visible.value = true;
      ready.value = false;
      void nextTick(async () => {
        await updatePosition();
        ready.value = true;
      });
    }, options.delay?.() ?? 200);
  }

  function hide() {
    if (timer) clearTimeout(timer);
    timer = null;
    visible.value = false;
    ready.value = false;
  }

  return { visible, ready, position, bubbleId, show, hide, updatePosition };
}

/** 气泡外观。底色是 cv('text-primary')，会随主题反转，所以文字必须跟着反转：
    这里用 pure-white-canvas（浅色主题=白、暗色主题=黑）而不是 on-accent（恒浅色），
    否则暗色下「浅底浅字」。已在 check-theme-contract 的 INVERSE_SURFACE_ALLOWLIST 登记。 */
export function tooltipBubbleStyle(ready: boolean, position: { top: string; left: string }) {
  return {
    position: 'fixed' as const,
    top: position.top,
    left: position.left,
    zIndex: String(AppleZIndex.tooltip),
    background: cv('text-primary'),
    color: cv('pure-white-canvas'),
    borderRadius: AppleRadius.small,
    padding: `${AppleSpace.sm} ${AppleSpace.md}`,
    fontFamily: AppleType.controlLabel.font,
    fontSize: AppleType.controlLabel.size,
    fontWeight: String(AppleType.controlLabel.weight),
    lineHeight: AppleLineHeight.default,
    boxShadow: cv('shadow-level2'),
    whiteSpace: 'nowrap' as const,
    pointerEvents: 'none' as const,
    visibility: (ready ? 'visible' : 'hidden') as 'visible' | 'hidden',
  };
}

