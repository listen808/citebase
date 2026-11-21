import { onScopeDispose, shallowRef } from 'vue';
import { AppleSpring, type SpringSpec } from '@/utils/core/apple-tokens';
import { currentMotionMode, scaledResponse } from '@/styles/theme/personalization';

/**
 * 弹簧物理只在这一处。
 * 存在的理由：CSS transition 画不出"从当前呈现值接着动"——它只会从 keyframe 0% 重放，
 * 所以拖到一半松手的层要么瞬移、要么等动画跑完才响应。Apple《Designing Fluid Interfaces》
 * 把这条列为第一性要求（打断后必须从此刻的位置和速度继续），只有积分器能做到。
 * 手势层（底部目录 sheet、侧栏抽屉）共用这一份，别再各处写第二套 rAF lerp。
 */

/**
 * 是否处于减弱动效档。**不再直接查 matchMedia**：个性化之后"减弱"有两个来源
 * （系统开关 / 应用内选了 `reduced`），口径必须与 CSS 侧 `MOTION_GUARD` 一致
 * （见 personalization.ts 的 currentMotionMode），否则 JS 弹簧与 CSS 过渡会各判各的。
 * 函数名与导出保持不变 —— 消费方与 `check:gesture` 的 G5 判据都按这个名字找它。
 */
export function prefersReducedMotion(): boolean {
  return currentMotionMode() === 'reduced';
}

/**
 * 越过边界的那截按橡皮筋压缩：越拉越沉，但永远拉不到头（`dimension` 是渐近上限）。
 * 小位移时约等于 `constant × overshoot`，所以手指走 1px 画面动 0.55px —— 跟手但不越权。
 */
export function rubberBand(overshoot: number, dimension: number, constant = 0.55): number {
  if (dimension <= 0 || overshoot === 0) return 0;
  const sign = overshoot < 0 ? -1 : 1;
  const x = Math.abs(overshoot);
  return sign * (1 - 1 / ((x * constant) / dimension + 1)) * dimension;
}

/**
 * 松手后的惯性落点：`v` 用 px/s。取 iOS `normal` 减速率（0.99），
 * 1200px/s 的一甩投影约 119px —— 够判"这是要划走"，又不至于轻轻一动就飞出屏幕。
 */
export function project(current: number, velocity: number, deceleration = 0.99): number {
  return current + (velocity / 1000) * (deceleration / (1 - deceleration));
}

/**
 * 临界阻尼弹簧积分器。`response` 是到达快慢（秒），`damping` 是回弹比例（1 = 不弹）。
 * 固定子步积分：帧率抖到 20fps 落点也和 120Hz 一致，否则同一手势在不同机器上走不同轨迹。
 */
export function useSpring(initial: number, spec: SpringSpec = AppleSpring.sheet) {
  const value = shallowRef(initial);
  /** 弹簧在跑 = true。运行时工装要靠它判断"这一帧的位置不是静止位"，见 `data-spring` */
  const running = shallowRef(false);
  const damping = spec.damping;
  /**
   * 每帧按当前动效倍数重算角频率。为什么不在构造时把 `omega/stiffness/friction` 算成常量：
   * 档位是可以在弹簧正跑着的时候改的（偏好页就在抽屉旁边），闭包里钉死一个 response
   * 会让"改了动效强度"这件事要到组件重挂才生效 —— 又是那种"接线全对、效果为零"的洞。
   */
  const omegaNow = () => (2 * Math.PI) / scaledResponse(spec.response);

  let x = initial;
  let v = 0;
  let target = initial;
  let last = 0;
  let raf = 0;
  let landed: Array<() => void> = [];

  /** 一帧：返回是否已经落定 */
  function advance(now: number): boolean {
    // 切后台再回来会有几秒的 dt，钳住：否则积分直接发散成一条直线甩出去
    const dt = Math.min((now - last) / 1000, 1 / 30);
    last = now;
    const sub = Math.max(1, Math.ceil(dt / (1 / 240)));
    const h = dt / sub;
    const w = omegaNow();
    const stiffness = w * w;
    const friction = 2 * damping * w;
    for (let i = 0; i < sub; i++) {
      v += (stiffness * (target - x) - friction * v) * h;
      x += v * h;
    }
    value.value = x;
    if (Math.abs(target - x) < 0.4 && Math.abs(v) < 0.4) {
      x = target;
      v = 0;
      value.value = target;
      return true;
    }
    return false;
  }

  function tick(now: number) {
    if (advance(now)) {
      raf = 0;
      running.value = false;
      const waiters = landed;
      landed = [];
      for (const done of waiters) done();
      return;
    }
    raf = requestAnimationFrame(tick);
  }

  /** 朝 target 走。`velocity` 是手势松手那一刻的 px/s，交给弹簧继续跑（速度接力） */
  function to(next: number, velocity = 0): Promise<void> {
    target = next;
    v = velocity;
    if (prefersReducedMotion()) {
      stop();
      x = next;
      v = 0;
      value.value = next;
      return Promise.resolve();
    }
    if (!raf) {
      last = performance.now();
      running.value = true;
      raf = requestAnimationFrame(tick);
    }
    return new Promise<void>((resolve) => { landed.push(resolve); });
  }

  /** 瞬移：用于打开前的摆位、减弱动效、以及卸载后的复位 */
  function jump(next: number) {
    stop();
    target = next;
    x = next;
    v = 0;
    value.value = next;
  }

  function stop() {
    running.value = false;
    if (raf) { cancelAnimationFrame(raf); raf = 0; }
    const waiters = landed;
    landed = [];
    for (const done of waiters) done();
  }

  onScopeDispose(stop);

  return { value, to, jump, stop, isRunning: () => raf !== 0, running, position: () => x };
}

export type Spring = ReturnType<typeof useSpring>;
