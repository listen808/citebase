import { computed, shallowRef } from 'vue';
import { AppleSpring, type SpringSpec } from '@/utils/core/apple-tokens';
import { project, rubberBand, useSpring } from './useSpring';

/**
 * 可拖走的浮层（底部 sheet、左右抽屉）共用的手势通路。
 * 三条判据都来自《Designing Fluid Interfaces》，缺一条手感就散：
 *   1. **1:1 跟手** —— 指针位移直接写在呈现值上，不等动画跑完；用 `setPointerCapture`，
 *      手指越出把手也不丢事件。
 *   2. **越界橡皮筋** —— 拉过边界不硬钉死，而是越拉越沉（`rubberBand`），松手弹回，
 *      用户立刻知道到头了。
 *   3. **按速度投影落点** —— 轻轻一小甩也要走、慢吞吞拖过半屏反而回弹，
 *      判据是投影位置而不是"拖到哪算哪"。
 * 打断是自动的：`onPointerdown` 先停弹簧再接管，落位从**此刻的呈现值**继续（速度一并接力），
 * 不会瞬移回起点重放一遍。
 *
 * 坐标约定：`offset` 是"朝 dismiss 方向走了多少 px"，恒 ≥0；屏幕上的正负由 `transform` 折一下，
 * 组件只管读 `offset` 判进度，不用关心自己朝哪边关。
 */
export function useSheetGesture(options: {
  axis: 'x' | 'y';
  /** dismiss 方向的满程（px）：sheet 高 / 抽屉宽。必须现读，旋转和内容变化都会改它 */
  travel: () => number;
  onDismiss: () => void;
  /** x 轴默认朝屏幕右划是 dismiss（右侧抽屉）；左侧抽屉传 -1 */
  direction?: 1 | -1;
  spec?: SpringSpec;
  /** 投影超过满程的这个比例就算"要划走" */
  dismissRatio?: number;
}) {
  const dismissRatio = options.dismissRatio ?? 0.42;
  const dir = options.direction ?? 1;
  const spring = useSpring(0, options.spec ?? AppleSpring.sheet);
  const dragging = shallowRef(false);
  const dragOffset = shallowRef(0);
  let startX = 0;
  let startY = 0;
  /** 抓住之前在静止的哪一头（0 = 开着，travel = 关着）；判成滚动时要还回这里 */
  let startOffset = 0;
  let locked: 'main' | 'cross' | null = null;
  let pointerId = -1;
  let handle: HTMLElement | null = null;
  const samples: Array<{ t: number; p: number }> = [];

  /** 拖拽期间的呈现值优先于弹簧：手指在哪，画面就在哪 */
  const offset = computed(() => (dragging.value ? dragOffset.value : spring.value.value));

  /** 满程之外的那截交给橡皮筋，之内的是原样的 1:1 */
  function present(raw: number) {
    const t = options.travel();
    if (raw < 0) return rubberBand(raw, t);
    if (raw > t) return t + rubberBand(raw - t, t);
    return raw;
  }

  /** 只看最近 ~90ms：更早的样本是"拖到一半停下来想会儿"，不该算进甩的速度 */
  function velocity(): number {
    const first = samples[0];
    const last = samples[samples.length - 1];
    if (!first || !last || samples.length < 2) return 0;
    let prev = first;
    for (let i = samples.length - 2; i >= 0; i--) {
      const s = samples[i];
      if (s && last.t - s.t > 90) { prev = s; break; }
    }
    const dt = (last.t - prev.t) / 1000;
    return dt > 0 ? (last.p - prev.p) / dt : 0;
  }

  function onPointermove(e: PointerEvent) {
    if (!dragging.value || e.pointerId !== pointerId) return;
    const dx = e.clientX - startX;
    const dy = e.clientY - startY;
    if (!locked) {
      if (Math.max(Math.abs(dx), Math.abs(dy)) < 8) return;
      const main = options.axis === 'x' ? Math.abs(dx) : Math.abs(dy);
      const cross = options.axis === 'x' ? Math.abs(dy) : Math.abs(dx);
      locked = main > cross ? 'main' : 'cross';
      // 判成滚动就别抢手势：内容里的纵向滚动比划走重要
      if (locked === 'cross') { release(e, { abort: true }); return; }
    }
    // 位移是"抓住之前所在那一头 + 手指增量"，不是增量本身：常驻 DOM 的抽屉关着时停在满程外，
    // 只算增量会让它第一帧从 travel 瞬跳回 0
    const raw = startOffset + (options.axis === 'x' ? dx : dy) * dir;
    dragOffset.value = present(raw);
    // 用本地时钟而不是 e.timeStamp：合成器把两次 pointermove 打进同一帧时 timeStamp 会相同，
    // 那样算出的速度恒为 0 —— 表现就是"怎么甩都关不掉"，而这条缺陷在真机上很难复现。
    samples.push({ t: performance.now(), p: raw });
    if (samples.length > 12) samples.shift();
    e.preventDefault();
  }

  function release(e: PointerEvent, opts: { abort?: boolean } = {}) {
    if (!dragging.value || (e.pointerId !== undefined && e.pointerId !== pointerId)) return;
    dragging.value = false;
    if (handle && handle.hasPointerCapture?.(pointerId)) {
      try { handle.releasePointerCapture(pointerId); } catch { /* 已经释放了就算成功 */ }
    }
    handle?.removeEventListener('pointermove', onPointermove);
    handle?.removeEventListener('pointerup', onPointerup);
    handle?.removeEventListener('pointercancel', onPointerup);
    pointerId = -1;
    const t = options.travel();
    if (opts.abort) {
      // 判成滚动就把位置交还给"抓住之前那一头"，不是硬回 0：
      // 常驻 DOM 的抽屉（SpaceView）关着时停在满程外，回 0 等于把它凭空推出来
      spring.jump(startOffset);
      samples.length = 0;
      locked = null;
      return;
    }
    const v = velocity();
    spring.jump(dragOffset.value);
    const willDismiss = t > 0 && project(dragOffset.value, v) > t * dismissRatio;
    // 落位动画的 resolve 不等于"该关"：中途被再次抓住也会 resolve 一次，
    // 所以关的判据只看**当前的呈现值是不是真的停在满程**，不看 promise。
    if (willDismiss) {
      spring.to(t, v).then(() => {
        if (!dragging.value && offset.value >= t - 1) options.onDismiss();
      });
    } else {
      spring.to(0, v);
    }
    samples.length = 0;
    locked = null;
  }

  function onPointerup(e: PointerEvent) {
    release(e);
  }

  function onPointerdown(e: PointerEvent) {
    if (e.pointerType === 'mouse' && e.button !== 0) return;
    const from = offset.value;
    startOffset = from;
    // 打断：先停弹簧，从当前呈现值接着来
    spring.stop();
    spring.jump(from);
    dragOffset.value = from;
    startX = e.clientX;
    startY = e.clientY;
    locked = null;
    samples.length = 0;
    dragging.value = true;
    pointerId = e.pointerId;
    handle = e.currentTarget as HTMLElement;
    try {
      handle.setPointerCapture(pointerId);
    } catch {
      // 极少数内核捕获失败：仍能拖，只是指针越出把手会断
    }
    handle.addEventListener('pointermove', onPointermove);
    handle.addEventListener('pointerup', onPointerup);
    handle.addEventListener('pointercancel', onPointerup);
  }

  /** 打开：首帧就摆到满程外，再回到 0 —— 不会看见"从中间冒出来" */
  function enter() {
    spring.jump(options.travel());
    return spring.to(0);
  }

  /** 关闭：交给 onDismiss 收尾（组件在那儿把 v-if 落下来） */
  function leave() {
    const t = options.travel();
    return spring.to(t).then(() => {
      if (!dragging.value) options.onDismiss();
    });
  }

  return {
    offset,
    dragging,
    enter,
    leave,
    /** 停放在某个位置不动：常驻 DOM 的浮层（不是 v-if 出来的）初始化时要摆到"关"那一头 */
    park: (next: number) => spring.jump(next),
    /** 拖拽区的 pointerdown 直接绑到元素上；`currentTarget` 就是把手本身 */
    onPointerdown,
    /**
     * 挂在会动的元素上：`data-spring="running"`。
     * 运行时工装（`check:boxes` 的 settle 等）原本靠 `document.getAnimations()` 等动画跑完再量，
     * 而 JS 弹簧不在那份账上 —— 于是它量到"飞行途中"的位置，报出"抽屉跑出视口 9px"这种假违例。
     * 让运动本身可查，比给判据加容差诚实。
     */
    springAttrs: computed(() => ({
      'data-spring': dragging.value || spring.running.value ? 'running' : undefined,
    })),
    /** 拼进 :style —— 只动 transform，落在合成器上（§11） */
    transformStyle: computed(() => {
      // 停在满程（offset 0）时**不写 transform**，也不写 willChange。
      // `translate3d(0,0,0)` 视觉上是恒等，但它照样把这一层变成内部 `position: fixed` 后代的
      // **包含块**，并给它开一个层叠上下文 —— 于是抽屉里那个右键菜单（DocContextMenu）的坐标
      // 全靠"抽屉正好在原点"这个巧合成立，它的 z 档也被关在抽屉那一层里出不来。
      // willChange 静置时同理：白占一层合成层。
      if (Math.abs(offset.value) < 0.01 && !dragging.value && !spring.running.value) return {};
      return {
        transform: options.axis === 'x'
          ? `translate3d(${offset.value * dir}px, 0, 0)`
          : `translate3d(0, ${offset.value}px, 0)`,
        willChange: 'transform',
      };
    }),
  };
}
