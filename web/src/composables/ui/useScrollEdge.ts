import { onMounted, onUnmounted, shallowRef, type Ref } from 'vue';

/**
 * 滚动边缘效应（scroll edge）的状态探针。
 *
 * 适用范围是**几何事实**，不是材质：凡是 `position: sticky|fixed` 且底色是半透明表面的东西，
 * 内容都会从它底下滚过去。全仓实测**只有一个真站点**：ApTable 的 sticky 表头
 * —— 它住在 tableWrapperStyle（overflow-y:auto）里，行真的会从它底下滚过。
 * 两个看起来像但其实不是的：顶栏 ApNavBar 与 EditorToolbar 都是各自滚动容器的**兄弟**，
 * 在 flex 列里独占一行，sticky 在没有滚动祖先的情况下不做任何事，内容从不从它们底下过。
 *
 * 为什么需要它：静止态保持通透很好看；可一旦有内容压在下面，半透明壳 + 次级/三级文字就会掉到
 * AA 线以下（浅外观下表头静止档 bg-secondary 0.74，三级文字实测 3.6:1）。这正是 Apple 用
 * scroll edge effect 解决的那件事 —— 壳在内容滚进来时加厚，而不是永远是一块白板。
 *
 * 两个刻意的设计决定：
 * 1. **加厚必须由状态触发**。常态加厚等于放弃招牌的通透；而且状态切换在 solid 材质下天然惰性
 *    （静止档与浮层档同色），组件不需要任何材质判断。
 * 2. **由 JS 挑档位，不用 CSS 属性选择器**。组件的底色是行内样式（`cv()`），属性选择器压不过行内，
 *    那种写法会"看起来装了、其实从没生效"。
 *
 * 反例也要记着：ApNavBar 与 EditorToolbar **不属于**这一类。顶栏住在 MainLayout 的 flex 列里独占一行
 * （`<ApNavBar />` 是 contentStyle 那个滚动容器的兄弟，不是它的后代），EditorToolbar 同理是
 * DocumentEditor 容器与滚动主体之间的兄弟 —— 给它们接探针就是死代码，而"装了但从不触发"
 * 正是这类效应唯一也最隐蔽的失败方式。
 */

export type ScrollEdgeState = 'under' | 'free';

/**
 * 「滚动位置 > 0」就是「有内容已经滚到壳层底下」。
 * 单独抽出来是因为这条判据值得被测：写成 `> 8` 之类的"防抖余量"会红在测试上，
 * 而那 8px 恰恰是最不可读的一瞬间。
 */
export function scrollEdgeState(scrollTop: number): ScrollEdgeState {
  return scrollTop > 0 ? 'under' : 'free';
}

/**
 * 压在内容上的壳面一律换到浮层档（surface-elevated）。
 * 这条规则只有这一个落点 —— 组件不许自己写 `'surface-elevated' : 别的` 的三元判断。
 * 泛型是为了把静止档的字面量类型带到 cv() 那里去（cv 收的是 AppleCssVarName 联合，
 * 返回宽 string 会在调用点编译不过），而不是为了在这里引令牌层的类型。
 */
export function shellSurfaceKey<K extends string>(
  state: ScrollEdgeState,
  restKey: K,
): 'surface-elevated' | K {
  return state === 'under' ? 'surface-elevated' : restKey;
}

/** 该元素在当前视口下真的能滚吗（内容比可视区高） */
function isScrollableY(node: Element): boolean {
  const y = getComputedStyle(node).overflowY;
  if (y !== 'auto' && y !== 'scroll') return false;
  return node.scrollHeight > node.clientHeight;
}

/**
 * 往上找那个**真的在滚**的容器：壳层 sticky 在谁的可视区里，内容就从谁那里滚过来。
 * 一路找到能滚的那一层才返回 —— 中间可能路过若干 overflow:auto 但内容并不溢出的壳，
 * 认了它们就等于把监听挂在一个永远不会发 scroll 的元素上（表现是"装了但从不触发"）。
 */
export function nearestScroller(el: Element | null): Element | Window | null {
  let node = el ? el.parentElement : null;
  while (node) {
    if (isScrollableY(node)) return node;
    node = node.parentElement;
  }
  // 文档本身在滚（没有内层滚动区的页面，以及窄视口）
  if (typeof document !== 'undefined' && document.scrollingElement) {
    const doc = document.scrollingElement as HTMLElement;
    if (doc.scrollHeight > doc.clientHeight) return window;
  }
  return null;
}

function readScrollTop(scroller: Element | Window): number {
  return scroller === window ? window.scrollY : (scroller as Element).scrollTop;
}

export interface ScrollEdgeOptions {
  attr?: string;
  /**
   * 显式指定要观察的滚动容器。顶栏这类**浮层壳**不在内容滚动区的祖先链上
   * （它钉在视口顶，滚动区是它的兄弟子树），向上找永远找不到，必须指名。
   * 用 `[data-ap-scroll]` 这个运行时锚点找：它本来就是为了巡检而挂的锚，
   * 锚点没了这里会退回 nearestScroller，而不是静默地什么都不观察。
   */
  scroller?: () => Element | Window | null;
}

/**
 * 返回当前状态（初始 'free'），并把状态同时写到元素属性上（默认 data-scroll-edge）——
 * 属性是为了可调试、可被工装核对；样式不依赖它。
 * 只在状态真的变化时写属性：滚动是高频事件，每帧 setAttribute 会把样式重算挂在滚动上。
 */
export function useScrollEdge(
  target: Ref<HTMLElement | null>,
  options: ScrollEdgeOptions = {},
): Readonly<Ref<ScrollEdgeState>> {
  const { attr = 'data-scroll-edge', scroller: resolveScroller } = options;
  const state = shallowRef<ScrollEdgeState>('free');
  let scroller: Element | Window | null = null;
  // 属性写入与状态变化分开记：state 初值就是 'free'，若用 state 自己判重复，
  // 静止态的元素会**永远没有 data-scroll-edge 属性**（表现为"探针装了但属性从没出现"，
  // 而界面看起来完全正常 —— 依赖这个属性做任何核对的工装都会静默失效）。
  let written: ScrollEdgeState | null = null;

  function apply(next: ScrollEdgeState) {
    if (next !== state.value) state.value = next;
    if (written === next) return;
    written = next;
    target.value?.setAttribute(attr, next);
  }

  function onScroll() {
    if (scroller) apply(scrollEdgeState(readScrollTop(scroller)));
  }

  onMounted(() => {
    scroller = (resolveScroller ? resolveScroller() : null) ?? nearestScroller(target.value);
    if (!scroller) {
      // 没有可滚的祖先 = 这一屏不存在"内容从壳层底下滚过"，钉在 free，不挂监听。
      apply('free');
      return;
    }
    apply(scrollEdgeState(readScrollTop(scroller)));
    scroller.addEventListener('scroll', onScroll, { passive: true });
  });

  onUnmounted(() => {
    if (scroller) scroller.removeEventListener('scroll', onScroll);
    scroller = null;
  });

  return state;
}
