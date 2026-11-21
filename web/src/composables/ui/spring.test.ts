import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { effectScope } from 'vue';
import { prefersReducedMotion, project, rubberBand, useSpring } from './useSpring';
import { useSheetGesture } from './useSheetGesture';

/**
 * 这一份测的是"物理"，不是快照：弹簧的手感没法用眼睛验收，只能用断言钉住——
 * 落点、越界不越权、打断不瞬移、减弱动效真的不动、甩一下的投影距离。
 * rAF 与 performance.now 全部自己驱动，所以每一帧的 dt 是确定的 16ms。
 */

type Frame = (now: number) => void;
let queue: Frame[] = [];
let clock = 0;
let active = effectScope();

beforeEach(() => {
  clock = 0;
  queue = [];
  active = effectScope();
  vi.stubGlobal('requestAnimationFrame', (cb: Frame) => { queue.push(cb); return queue.length; });
  vi.stubGlobal('cancelAnimationFrame', (id: number) => { queue[id - 1] = () => {}; });
  vi.stubGlobal('performance', { now: () => clock });
});

afterEach(() => {
  active.stop();
  vi.unstubAllGlobals();
});

/** composable 里用了 onScopeDispose，必须有作用域才不报警 */
function inScope<T>(fn: () => T): T {
  let out!: T;
  active.run(() => { out = fn(); });
  return out;
}

/** 推进 n 帧，每帧 16ms */
function pump(frames = 1) {
  for (let i = 0; i < frames; i++) {
    clock += 16;
    const pending = queue.splice(0, queue.length);
    for (const cb of pending) cb(clock);
  }
}

function settle() {
  pump(120);
}

/** resolve 排的是微任务：不同步等一手，dismiss 回调根本还没跑 */
async function flush() {
  await Promise.resolve();
  await Promise.resolve();
}

describe('rubberBand', () => {
  it('小位移按常数压缩，且永远小于位移本身', () => {
    expect(rubberBand(10, 400)).toBeCloseTo(10 * 0.55 / (1 + (10 * 0.55) / 400), 6);
    expect(Math.abs(rubberBand(10, 400))).toBeLessThan(10);
  });

  it('单调但封顶：越拉越沉，拉不到头', () => {
    let prev = 0;
    for (let x = 0; x <= 4000; x += 100) {
      const y = rubberBand(x, 400);
      expect(y).toBeGreaterThanOrEqual(prev);
      expect(y).toBeLessThan(400);
      prev = y;
    }
  });

  it('保留方向：往回拽是负的', () => {
    expect(rubberBand(-80, 400)).toBeLessThan(0);
    expect(Math.abs(rubberBand(-80, 400))).toBeCloseTo(Math.abs(rubberBand(80, 400)), 6);
  });
});

describe('project', () => {
  it('1200px/s 的一甩投影约 119px，够判"要走"', () => {
    expect(project(0, 1200)).toBeCloseTo(118.8, 1);
  });

  it('速度为 0 就停在原地', () => {
    expect(project(60, 0)).toBe(60);
  });

  it('反向速度会把落点拉回来', () => {
    expect(project(100, -500)).toBeLessThan(100);
  });
});

describe('useSpring', () => {
  it('临界阻尼不越过目标', () => {
    const spring = inScope(() => useSpring(0, { response: 0.34, damping: 1 }));
    spring.to(100);
    let over = 0;
    for (let i = 0; i < 120; i++) {
      pump(1);
      if (spring.value.value > 100.0001) over++;
    }
    expect(over).toBe(0);
    expect(spring.value.value).toBe(100);
  });

  it('落定后停得干净：不再排新帧', () => {
    const spring = inScope(() => useSpring(0));
    spring.to(50);
    settle();
    expect(spring.running.value).toBe(false);
    expect(spring.value.value).toBe(50);
    const before = queue.length;
    pump(3);
    expect(queue.length).toBe(before);
  });

  it('打断是从当前呈现值继续，不是瞬移回起点重放', () => {
    const spring = inScope(() => useSpring(0));
    spring.to(200);
    pump(6);
    const midway = spring.value.value;
    expect(midway).toBeGreaterThan(0);
    expect(midway).toBeLessThan(200);
    // 手指重新抓住：位置保持、目标改成 0
    spring.jump(midway);
    spring.to(0, -300);
    pump(1);
    // 第一帧只能比中途更靠上（继续往 0 走），绝不会跳到 200 那一侧
    expect(spring.value.value).toBeLessThan(midway + 0.001);
    expect(spring.value.value).toBeGreaterThan(0);
  });

  it('切后台回来那帧的巨大 dt 被钳住，不会把值甩飞', () => {
    const spring = inScope(() => useSpring(0));
    spring.to(100);
    pump(1);
    clock += 5000;
    const pending = queue.splice(0, queue.length);
    for (const cb of pending) cb(clock);
    expect(spring.value.value).toBeGreaterThan(0);
    expect(spring.value.value).toBeLessThan(100);
  });

  it('开了「减弱动态效果」就直接落位，一帧都不动', () => {
    vi.stubGlobal('matchMedia', (q: string) => ({ matches: q.includes('reduce'), addEventListener() {}, removeEventListener() {} }));
    const spring = inScope(() => useSpring(0));
    expect(prefersReducedMotion()).toBe(true);
    spring.to(120);
    expect(spring.value.value).toBe(120);
    expect(queue.length).toBe(0);
  });
});

/** 假把手：只需要 addEventListener 与 pointer capture 这几个动作 */
function fakeHandle() {
  const listeners: Record<string, Array<(e: never) => void>> = {};
  return {
    addEventListener(type: string, fn: (e: never) => void) { (listeners[type] ||= []).push(fn); },
    removeEventListener(type: string, fn: (e: never) => void) {
      listeners[type] = (listeners[type] || []).filter((f) => f !== fn);
    },
    setPointerCapture() {},
    releasePointerCapture() {},
    hasPointerCapture: () => true,
    fire(type: string, e: unknown) { (listeners[type] || []).forEach((f) => f(e as never)); },
  };
}

function pointerEvent(el: unknown, x: number, y: number, timeStamp: number) {
  return {
    pointerId: 1,
    pointerType: 'touch',
    button: 0,
    clientX: x,
    clientY: y,
    timeStamp,
    currentTarget: el,
    preventDefault() {},
  };
}

/** 纵向 sheet：满程 400px，把手是那个假元素。时间由 clock 推，弹簧和测速读的是同一个时钟 */
function makeSheet(travel = 400) {
  let dismissed = 0;
  const sheet = inScope(() => useSheetGesture({ axis: 'y', travel: () => travel, onDismiss: () => { dismissed++; } }));
  const handle = fakeHandle();
  return {
    sheet,
    dismissed: () => dismissed,
    park: (px: number) => sheet.park(px),
    grab: () => { clock += 16; sheet.onPointerdown(pointerEvent(handle, 0, 0, clock) as unknown as PointerEvent); },
    move: (x: number, y: number, t: number) => { clock = t; handle.fire('pointermove', pointerEvent(handle, x, y, t)); },
    up: (x: number, y: number, t: number) => { clock = t; handle.fire('pointerup', pointerEvent(handle, x, y, t)); },
  };
}

describe('useSheetGesture', () => {
  it('1:1 跟手：手指走 40px，画面就走 40px', () => {
    const g = makeSheet();
    g.grab();
    g.move(0, 40, clock + 16);
    expect(g.sheet.offset.value).toBe(40);
    g.move(0, 137.5, clock + 32);
    expect(g.sheet.offset.value).toBe(137.5);
    g.up(0, 137.5, clock + 48);
  });

  it('越过静止边界是橡皮筋，不是硬钉死、也不是照单全收', () => {
    const g = makeSheet();
    g.grab();
    g.move(0, -60, clock + 16);
    const shown = g.sheet.offset.value;
    expect(shown).toBeLessThan(0);             // 允许往回拽一点
    expect(Math.abs(shown)).toBeLessThan(60);  // 但远小于手指
    expect(Math.abs(shown)).toBeGreaterThan(20);
    g.up(0, -60, clock + 32);
    settle();
    expect(g.sheet.offset.value).toBe(0);
  });

  it('拖到不到一半就松手：回弹，不关', () => {
    const g = makeSheet();
    g.grab();
    g.move(0, 120, clock + 16);
    g.move(0, 120, clock + 900);   // 停住：速度归零
    g.up(0, 120, clock + 916);
    settle();
    return flush().then(() => {
      expect(g.dismissed()).toBe(0);
      expect(g.sheet.offset.value).toBe(0);
    });
  });

  it('轻轻一甩：按速度投影划走，并真的通知 dismiss', async () => {
    const g = makeSheet();
    g.grab();
    g.move(0, 60, clock + 16);
    g.move(0, 100, clock + 32);   // 40px / 16ms ≈ 2500px/s
    g.up(0, 100, clock + 48);
    settle();
    await flush();
    expect(g.dismissed()).toBe(1);
    expect(g.sheet.offset.value).toBe(400);
  });

  it('停在"关"那一头的抽屉被抓住，第一帧不会瞬跳开', () => {
    const g = makeSheet();
    g.park(400);                       // 常驻 DOM 的抽屉：关着 = 停在满程外
    g.grab();
    g.move(0, 20, clock + 16);         // 再往外推 20px
    const shown = g.sheet.offset.value;
    // 只算手指增量的话这里会是 20（= 抽屉被凭空拉出来），正确值是越过满程后被橡皮筋压住的 410 左右
    expect(shown).toBeGreaterThan(400);
    expect(shown).toBeLessThan(420);
    g.up(0, 20, clock + 32);
    settle();
  });

  it('运动期间挂着 data-spring="running"，落定就撤', () => {
    const g = makeSheet();
    expect(g.sheet.springAttrs.value['data-spring']).toBeUndefined();
    g.sheet.enter();
    pump(3);
    expect(g.sheet.springAttrs.value['data-spring']).toBe('running');
    settle();
    expect(g.sheet.springAttrs.value['data-spring']).toBeUndefined();
  });

  it('主轴锁：纵向拖一个横向抽屉算滚动，手势交还给内容', () => {
    let dismissed = 0;
    const sheet = inScope(() => useSheetGesture({ axis: 'x', travel: () => 280, onDismiss: () => { dismissed++; } }));
    const handle = fakeHandle();
    sheet.onPointerdown(pointerEvent(handle, 0, 0, clock) as unknown as PointerEvent);
    handle.fire('pointermove', pointerEvent(handle, 2, 90, clock + 16));
    expect(sheet.dragging.value).toBe(false);
    settle();
    expect(sheet.offset.value).toBe(0);
    expect(dismissed).toBe(0);
  });

  it('半路抓住正在关闭的 sheet：接着拖，不会自己关掉', async () => {
    const g = makeSheet();
    g.grab();
    g.move(0, 200, clock + 16);
    g.move(0, 260, clock + 32);
    g.up(0, 260, clock + 48);      // 判成划走，弹簧正往 400 跑
    pump(4);
    expect(g.sheet.offset.value).toBeGreaterThan(260);
    g.grab();                      // 用户又抓住了它
    await flush();                 // 让被接管那次 resolve 的回调先跑完
    expect(g.dismissed()).toBe(0);
    expect(g.sheet.dragging.value).toBe(true);
  });
});
