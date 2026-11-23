/**
 * 液态玻璃的镜面高光随光源移动 —— Apple「陀螺仪技巧」的网页等价物。
 *
 * 为什么值得做：Liquid Glass 的物理感一大半来自**高光跟着设备动**（1ar labs 对
 * WWDC25 材质的拆解：镜面高光与陀螺仪同步，倾斜设备光就换角度 —— 这也是为什么
 * 截图/录屏里的玻璃看起来扁平：静态帧丢掉了"光源在动"这件事）。
 * 网页上没有系统级的陀螺仪信号可以白拿，等价物是两条：
 *  · 桌面（hover:hover + pointer:fine）→ 指针位置。光在指针那边，玻璃迎着它亮。
 *  · 移动 → deviceorientation。gamma（左右倾）→ x、beta（前后倾）→ y。
 *    iOS 13+ 需要 DeviceOrientationEvent.requestPermission()，**这里不主动要权限**：
 *    未授权时事件根本不来，令牌停在默认值 = 回到静态高光，功能无损。
 *
 * 写的是 `--apple-light-x / --apple-light-y` 两个根级令牌，高光渐变（径向）引用它们：
 *  · .apple-glass-layer::before（17 个浮层消费者）
 *  · 下拉浮层材质常量（`AppleGlass` 的 dropdown 档）的镜面高光
 * 所以这里没有任何组件接线 —— 一处改写，全站玻璃一起转向。
 *
 * 护栏与成本（写根级自定义属性会让引用它的渐变全部重绘，所以闸门有三道）：
 *  · 动效档：指针/倾斜追踪只在 full 档跑（soft/reduced 停在静态高光），运行时改档即时生效；
 *  · 减弱动效（系统 prefers-reduced-motion 或应用内 data-motion='reduced'）→ 不更新，
 *    令牌保持默认值 18%/0%（与旧静态 135° 渐变观感一致）；应用内显式选 full 时
 *    压过系统信号，与 CSS/时长两处的口径一致；
 *  · 滚动静默窗：scroll 后 120ms 内不追 —— 滚动中 backdrop 逐帧变化、blur 在重滤波，
 *    高光重绘再叠上去是两笔开销并发；页面隐藏时同样跳过；
 *  · rAF 惯性合帧（每帧向目标靠 12%，贴住即停）+ 0.4% 位移阈值兜底。
 */

import { currentMotionMode } from '@/styles/theme/personalization';

const ROOT = typeof document !== 'undefined' ? document.documentElement : null;

/**
 * 惯性平滑（2026-11-01「跟随光太粗糙」返工）：指针事件把目标写进 target，rAF 循环每帧
 * 只向目标靠拢 EASE 比例，近了就贴住停表。此前令牌 1:1 硬跟指针 —— 高光段跟着光标逐帧
 * 平移，读出来是"一条白线在边上滑"而不是"光换了个方向"；惯性让它像有质量的光源。
 */
const EASE = 0.12;
/** 距目标小于这个百分比就贴住并停表：0.05% ≈ 半像素，再追是白烧 rAF */
const SETTLE_PCT = 0.05;

/** 小于这个百分比位移就不写令牌：光在毫米级晃动不值得让全站渐变重绘 */
const THRESHOLD_PCT = 0.4;
/** 设备倾斜的满偏角度：±30° 内线性映射到 0~100%，再大就钳到边缘 */
const TILT_RANGE_DEG = 30;
/** 滚动静默窗：滚动中 backdrop 在逐帧变化，此时再让高光渐变重绘是两笔开销叠着烧 */
const SCROLL_QUIET_MS = 120;

let installed = false;
/** 与令牌默认值（18%/0%）同源的起点：第一次动效从这里滑出去，而不是瞬移 */
let curX = 18;
let curY = 0;
let targetX = 18;
let targetY = 0;
let running = false;
let scrollQuietUntil = 0;

/**
 * 与 CSS 侧（MOTION_GUARD）和 JS 时长（scaledMs）共用同一个口径：
 * 应用内显式选档压过系统信号，`data-motion` 缺席才回落 prefers-reduced-motion。
 * 自己再 matchMedia 一次的话，系统开了减弱、应用内选了 full 的用户会遇到
 * "过渡在动、镜面高光不动"的半接线状态。
 */
function motionReduced(): boolean {
  if (!ROOT) return true;
  return currentMotionMode() === 'reduced';
}

function clampPct(v: number): number {
  return Math.min(100, Math.max(0, v));
}

function writeTokens(): void {
  if (!ROOT) return;
  ROOT.style.setProperty('--apple-light-x', curX.toFixed(2) + '%');
  ROOT.style.setProperty('--apple-light-y', curY.toFixed(2) + '%');
}

function tick(): void {
  // 页面隐藏或动效被关掉就停表：下一次 pointermove 会重新拉起
  if (document.hidden || currentMotionMode() !== 'full' || motionReduced()) {
    running = false;
    return;
  }
  const dx = targetX - curX;
  const dy = targetY - curY;
  if (Math.abs(dx) < SETTLE_PCT && Math.abs(dy) < SETTLE_PCT) {
    curX = targetX;
    curY = targetY;
    writeTokens();
    running = false;
    return;
  }
  curX += dx * EASE;
  curY += dy * EASE;
  writeTokens();
  requestAnimationFrame(tick);
}

function schedule(x: number, y: number): void {
  // 与当前值差得极小就不追：毫米级晃动不值得让全站渐变逐帧重绘
  if (Math.abs(targetX - x) < THRESHOLD_PCT && Math.abs(targetY - y) < THRESHOLD_PCT) return;
  targetX = x;
  targetY = y;
  if (!running) {
    running = true;
    requestAnimationFrame(tick);
  }
}

function onPointerMove(e: PointerEvent): void {
  if (motionReduced()) return;
  // 档位闸门：指针追踪只在 full 档跑。写根级令牌会让引用它的渐变全部重绘，
  // 这份开销不该落在 soft/reduced 用户身上（运行时改档即时生效，不用重装）。
  if (currentMotionMode() !== 'full') return;
  // 滚动静默窗：滚动 + 高光重绘并发是最差组合，滚动后 120ms 内不追
  if (Date.now() < scrollQuietUntil) return;
  const w = window.innerWidth || 1;
  const h = window.innerHeight || 1;
  schedule((e.clientX / w) * 100, (e.clientY / h) * 100);
}

function onOrient(e: DeviceOrientationEvent): void {
  const gamma = e.gamma;
  const beta = e.beta;
  if (gamma === null || beta === null) return;
  if (motionReduced()) return;
  if (currentMotionMode() !== 'full') return;
  // 光随倾斜反向移动：设备右倾（gamma>0）相当于光源相对屏幕向左移
  schedule(clampPct(50 - (gamma / TILT_RANGE_DEG) * 50), clampPct(50 + (beta / TILT_RANGE_DEG) * 50));
}

/**
 * 全站唯一接线处（App.vue onMounted 调一次）。幂等：重复调用只生效一次。
 * 精确指针设备走 pointermove，触屏设备走 deviceorientation（未授权则静默停在默认值），
 * 两者都不满足 = 没有光源信号，什么都不挂。
 */
export function installSpecularLight(): void {
  if (installed || typeof window === 'undefined' || !ROOT) return;
  installed = true;

  if (typeof matchMedia === 'function' &&
      matchMedia('(hover: hover) and (pointer: fine)').matches) {
    window.addEventListener('pointermove', onPointerMove, { passive: true });
  } else if ('ondeviceorientation' in window) {
    window.addEventListener('deviceorientation', onOrient, { passive: true });
  } else {
    return;
  }
  // 滚动静默窗：scroll 不冒泡，capture 才能在 window 上接住任意内层滚动容器
  window.addEventListener('scroll', () => { scrollQuietUntil = Date.now() + SCROLL_QUIET_MS; }, { capture: true, passive: true });
}
