import { onBeforeUnmount, watch } from 'vue';
import { useEscapeClose } from './useEscapeClose';

/**
 * 浮层的"点别处就收"——全站唯一实现（与 useEscapeClose 配成一套：一个管出去，一个管回来）。
 *
 * 为什么用 pointerdown 而不是 click（原先三处自写实现都用 click，各有其害）：
 *  - 触屏上 click 是合成事件，"点一下别处关掉"会晚一拍，滚动一下列表常常根本不触发；
 *  - 用 click 冒泡时，**点另一个触发按钮**会先被这里关掉、再被那个按钮自己的 @click 打开，
 *    用户看到的就是"这个菜单怎么关不掉"。capture 阶段先关，随后那个按钮再开，才是想要的切换。
 *
 * 判定范围是"一串自己人元素"而不是"触发器的子树"：面板可以 Teleport 到 body
 * （ApSelect / ApPopover 就是），只看触发器会把面板内的一次点击当成"点了别处"。
 *
 * own() 写成函数而不是数组：面板常常是 v-if 出来的，取引用时它可能还不存在。
 */
export function useClickOutsideClose(
  isOpen: () => boolean,
  close: () => void,
  own: () => Array<HTMLElement | null | undefined>,
): void {
  function onPointerDown(e: PointerEvent) {
    if (!isOpen()) return;
    const target = e.target as Node | null;
    if (!target) return;
    for (const el of own()) {
      if (el && (el === target || el.contains(target))) return;
    }
    close();
  }

  // 只在开着的时候挂监听：全站顶栏 + 每个 ApSelect 实例都常驻一份 document 监听的话，
  // 一次点击要跑几十遍回调，白付。
  watch(isOpen, (open) => {
    if (open) document.addEventListener('pointerdown', onPointerDown, true);
    else document.removeEventListener('pointerdown', onPointerDown, true);
  }, { immediate: true });

  onBeforeUnmount(() => document.removeEventListener('pointerdown', onPointerDown, true));
}

/**
 * 一个浮层的两种退路：点别处 + 按 Esc。
 * 顶栏那两个菜单原先两样都没有——它们常驻在页面最上层（z=dropdown），
 * 点开主题菜单再点正文，菜单还挂在那儿挡住"设置"和"通知"两个按钮。
 */
export function useDismiss(
  open: { value: boolean },
  own: () => Array<HTMLElement | null | undefined>,
): void {
  const isOpen = () => open.value;
  const close = () => { open.value = false; };
  useClickOutsideClose(isOpen, close, own);
  useEscapeClose(isOpen, close);
}
