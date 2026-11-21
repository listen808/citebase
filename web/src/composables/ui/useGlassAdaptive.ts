import { onMounted, onUnmounted, type Ref } from 'vue';
import {
  GLASS_FLOAT_ALPHA_CEIL,
  GLASS_FLOAT_ALPHA_FLOOR,
  GLASS_TEXT_FLOOR,
  compositeOver,
  glassContrast,
  minAlphaForFloor,
  mixRgba,
  parseCssColor,
  srgbLuminance,
  type Rgb,
  type Rgba,
} from '@/utils/core/glass-contrast';
import { nearestScroller, type ScrollEdgeState } from './useScrollEdge';

/**
 * 玻璃的 **adaptive tint**（Apple 那条 "玻璃自己决定该多实" 的机制）。
 * 两个落点共用同一套测量、各写各的令牌：
 *   · 浮层面板（float；菜单 / 弹窗 / 抽屉）→ applyGlassLift；
 *   · 壳层加厚档（elevated；顶栏 under 态）→ applyShellLift + useShellGlassLift。
 *
 * 为什么需要它：一块玻璃该多透完全取决于**它身后是什么**。
 * 压在页面自己的白卡片 / 浅色渐变上时，0.5 的板就是玻璃；压在一用户上传的暗图上时，
 * 同样的 0.5 就是不可读。原先只有两个静态答案（0.78 = 白板、0.62 = 玻璃），
 * 选哪个都有一半的场景是错的 —— 用户两轮判词「根本就不是液态玻璃」量的正是那一半。
 *
 * 怎么知道身后是什么：不去截图（拿不到、也不该在浏览器里读合成结果），
 * 而是**量被画出来的底色**：在面板那块矩形上撒格子，逐点用 `elementsFromPoint`
 * 从最上层往下合成各元素的 `background-color`，取最暗的一点。
 * 这条口径是有意为之的两件事：
 *  1. 只量**底色**，不量文字 —— 文字正是 blur 会抹平的高频细节，它不该决定板的厚度；
 *     背景色才是"这块区域整体有多暗"的真答案，而 WCAG 判的就是它。
 *  2. 撞上**位图**（img / canvas / video / iframe / `url()` 背景）就整块判为「未知」，
 *     退回保守档。**只有 html/body 的环境层渐变算已知** —— 那是我们自己画的壁纸；
 *     其余元素上的背景图任意（用户内容可以是深色渐变、封面虚化），一律判未知。
 *     环境层本身也要把渐变 stop 合成进来：只量 `background-color` 会读到 0.89，
 *     而浅色环境层最暗的渐变热点实测 ≈0.63，四级文字正是被这个差值挤下 4.5。
 *
 * 兜底方向：JS 没跑、量不到、解不出，一律**保留令牌里那个保守 alpha**（壳面则是 0.88/0.96）。
 * 也就是说这一层只会让玻璃更透（在能证明安全的时候），绝不会把谁变得比现状更看不见。
 */

/**
 * 覆写的是**令牌本身**挂在这个元素上，不是另起一枚内层属性：
 * 自定义属性在声明处（:root）就把嵌套的 var() 解完了，元素上再改内层追不上 ——
 * 实测过一版 `rgba(…, var(--apple-glass-float-a, 0.78))`，运行时写了值，计算值一动不动。
 */
const LIFT_PROP = '--apple-surface-float';
/** 壳层加厚档：顶栏在内容滚到它底下时换的那一档（见 shellSurfaceKey） */
const SHELL_LIFT_PROP = '--apple-surface-elevated';
/** 静止档：加厚档的下限 = 它停着时候的那档（加厚只能往上） */
const CHROME_PROP = '--apple-surface-chrome';
/**
 * 顶栏又不厚又全宽，滚动时它身后每帧都在换，所以必须节流量。
 * 120ms（≈8Hz）是「先保证不掉帧」的那一侧：再快，撒点那几次 getComputedStyle 会把
 * 样式重算挂在滚动上；再慢，暗图滑到顶栏底下那一瞬间的可读性就追不上。
 */
const SHELL_LIFT_INTERVAL = 120;
/** 44px 高的长条：横撒 4 点、竖只要一行，再密是白付（一条 48px 网格都跨不过它） */
const SHELL_SAMPLE_MAX = 4;
const UNKNOWN_TAGS = new Set(['IMG', 'CANVAS', 'VIDEO', 'IFRAME', 'OBJECT', 'EMBED']);
/** 撒格子的间距与内缩：48px 已经远大于一条发丝线，再密只是白付 getComputedStyle */
const SAMPLE_STEP = 48;
const SAMPLE_INSET = 6;
/** 面板比这更大时最多撒 6×6 —— 上限锁死每次测量的成本 */
const SAMPLE_MAX = 6;

let probe: HTMLDivElement | null = null;

/**
 * 把 `--apple-surface-float` 解析成真实 rgb。
 * 直接 `getPropertyValue()` 拿到的可能是带 `var()` 的原始 token 流（浏览器对自定义属性的
 * 计算值处理各家不一），所以借一个一次性元素让**用它的那条属性**去解析 —— 得到的一定是
 * 参与绘制的那个值，而不是声明里的那个字符串。
 * 挂在 body 上而不是面板里：面板自己会被覆写 `--apple-surface-float`，挂里面就量到改后的了。
 */
function resolvePlate(prop: string): Rgba | null {
  if (typeof document === 'undefined') return null;
  if (!probe) {
    probe = document.createElement('div');
    probe.setAttribute('aria-hidden', 'true');
    probe.style.cssText = 'position:fixed;left:-10000px;top:0;width:1px;height:1px;pointer-events:none';
    document.body.appendChild(probe);
  }
  probe.style.background = `var(${prop})`;
  const c = parseCssColor(getComputedStyle(probe).backgroundColor);
  probe.style.background = '';
  return c;
}

function resolveFloatPlate(): Rgb | null {
  const c = resolvePlate(LIFT_PROP);
  return c ? [c[0], c[1], c[2]] : null;
}

function resolveTextTier(name: string): Rgb | null {
  const c = parseCssColor(getComputedStyle(document.documentElement).getPropertyValue(name));
  return c ? [c[0], c[1], c[2]] : null;
}

/** 壳面/浮层的现成标记：这几处的背景图是我们自己的材质，不是用户内容 */
function isOwnGlassSurface(node: Element): boolean {
  return (
    node.hasAttribute('data-glass-surface') ||
    node.hasAttribute('data-ap-shell') ||
    node.classList.contains('apple-glass-layer') ||
    node.classList.contains('apple-glass-nav')
  );
}

/**
 * 这一层的背景图给我们什么。
 *   · `null` —— 没有背景图，只按 background-color 合成；
 *   · `'unknown'` —— 读不出亮度（位图，或非环境层上任意渐变）；
 *   · `Rgba` —— 环境层渐变里**压在自身底色上最暗的那个 stop**。
 *
 * 为什么不是把所有 stop 依次合成：四层径向渐变铺在屏幕不同位置，合成到一起等于虚构一个
 * 「四层同时取最暗值」的点（≈0.48），比实测最暗热点（≈0.63）厚一档，浮层会整体发白。
 * 最暗 stop 压在环境底色上，正是实测峰值那一格；而漏掉渐变（旧行为）会读到 0.89，
 * 把四级文字在最热点上的真实对比度从 ≈3.6 高估成 ≈4.3。
 */
function envGradientStop(node: Element, style: CSSStyleDeclaration): Rgba | null | 'unknown' {
  const bi = style.backgroundImage;
  if (!bi || bi === 'none') return null;
  // 我们自己壳面/浮层上的背景图就是镜面高光，不是用户内容。
  // 把它判成未知，所有嵌在壳里的浮层（侧栏用户菜单、搜索面板）都会退到 0.94 近实心 ——
  // 实测侧栏用户菜单就是这样变成一块黑板的，而它身后其实只是侧栏自己的 chrome。
  // 忽略这层高光会少算 ≤0.26 的白，取点又内缩 6px，留给 liftAlphaFor 的 1.08 余量兜住。
  if (isOwnGlassSurface(node)) return null;
  // 环境层是我们自己画的壁纸；其余元素的背景图可能来自用户内容
  if (node !== document.documentElement && node !== document.body) return 'unknown';
  if (/url\(/i.test(bi)) return 'unknown';
  const tokens = bi.match(/rgba?\([^)]*\)|#[0-9a-f]{3,8}\b/gi);
  if (!tokens?.length) return 'unknown';
  const base = parseCssColor(style.backgroundColor);
  const ground: Rgb = base && base[3] >= 0.999 ? [base[0], base[1], base[2]] : [255, 255, 255];
  let worst: Rgba | null = null;
  let worstL = Infinity;
  for (const token of tokens) {
    const c = parseCssColor(token);
    // 采集到一半发现读不出的颜色（hsl / 命名色）就整层判未知，不许只合成认得的那几个
    if (!c) return 'unknown';
    if (c[3] <= 0) continue;
    const l = srgbLuminance(compositeOver(c, ground));
    if (l < worstL) {
      worstL = l;
      worst = c;
    }
  }
  return worst;
}

export type BackdropSample = { luminance: number; unknown: boolean };

function samplePoints(rect: DOMRect, max = SAMPLE_MAX): Array<{ x: number; y: number }> {
  const x0 = Math.max(rect.left + SAMPLE_INSET, 0);
  const x1 = Math.min(rect.right - SAMPLE_INSET, window.innerWidth);
  const y0 = Math.max(rect.top + SAMPLE_INSET, 0);
  const y1 = Math.min(rect.bottom - SAMPLE_INSET, window.innerHeight);
  if (x1 <= x0 || y1 <= y0) return [];
  const cols = Math.min(max, Math.max(1, Math.ceil((x1 - x0) / SAMPLE_STEP)));
  const rows = Math.min(max, Math.max(1, Math.ceil((y1 - y0) / SAMPLE_STEP)));
  const pts: Array<{ x: number; y: number }> = [];
  for (let r = 0; r < rows; r++) {
    for (let c = 0; c < cols; c++) {
      pts.push({
        x: cols === 1 ? (x0 + x1) / 2 : x0 + ((x1 - x0) * c) / (cols - 1),
        y: rows === 1 ? (y0 + y1) / 2 : y0 + ((y1 - y0) * r) / (rows - 1),
      });
    }
  }
  return pts;
}

/**
 * 量面板身后**被画出来的底色**。
 * 取哪一格不是固定的：板上是深字（浅色浮层）时最暗的那格最不利；板上是浅字
 * （深色壳/浮层）时反过来是最亮的那格 —— 深色壳固定取 `min` 会把一块压在白卡片上的
 * 顶栏算成"身后很暗"，于是它永远不肯加厚。默认 `min` 保持浮层那一支的既有口径。
 * 每点从最上层不是自己/自己后代的元素开始往上合成，撞到不透明即停。
 */
export function sampleBackdrop(
  rect: DOMRect,
  self: Element,
  max = SAMPLE_MAX,
  pick: 'min' | 'max' = 'min',
): BackdropSample {
  let worst = pick === 'min' ? 1 : -1;
  let any = false;
  for (const p of samplePoints(rect, max)) {
    const stack = document.elementsFromPoint(p.x, p.y);
    let top: Element | undefined;
    for (const el of stack) {
      if (el !== self && !self.contains(el)) { top = el; break; }
    }
    if (!top) continue;
    /**
     * 先**从下往上**收集，再按这个顺序自底向上合成。
     * 顺序是这条判据的全部难点：`elementsFromPoint` 给的是从上往下，
     * 而 alpha 合成的语义是"后画的压在前画的上面"。第一版顺着往上走、每层都
     * `compositeOver(这一层, 已累计结果)`，于是走到 `body`（不透明、浅色）时，
     * 页面底被算成了压在遮罩**上面**，弹窗身后那块 0.45 的暗遮罩整个被抹掉 ——
     * 表现是弹窗和菜单都解到同一个下限，看着像"自适应没生效"。
     */
    const layers: Array<[number, number, number, number]> = [];
    let unknown = false;
    for (let n: Element | null = top; n; n = n.parentElement) {
      const s = getComputedStyle(n);
      // 元素本身就是一张位图（img / canvas / video / iframe…）：它的亮度读不出来
      if (UNKNOWN_TAGS.has(n.tagName)) { unknown = true; break; }
      const stop = envGradientStop(n, s);
      if (stop === 'unknown') { unknown = true; break; }
      const c = parseCssColor(s.backgroundColor);
      // 同层绘制顺序是「先底色、再背景图」；layers 从上往下 push、最后反向合成，
      // 所以这里先 push 背景图那一层、再 push 底色，反向遍历时顺序才对。
      if (stop) layers.push(stop);
      if (!c || c[3] === 0) continue;
      layers.push(c);
      if (c[3] >= 0.999) break;
    }
    if (unknown) return { luminance: 0, unknown: true };
    if (!layers.length) continue;
    let acc: Rgb = [255, 255, 255];
    for (let i = layers.length - 1; i >= 0; i--) {
      const layer = layers[i];
      if (layer) acc = compositeOver(layer, acc);
    }
    const l = srgbLuminance(acc);
    if (pick === 'min' ? l < worst : l > worst) worst = l;
    any = true;
  }
  return any ? { luminance: worst, unknown: false } : { luminance: 0, unknown: true };
}

/**
 * 由一次测量得出这一档该用的 alpha。单独抽出来是因为它是这一层全部的判断，
 * 而 DOM 那半边（撒格子、往上合成）不值得为了测它再造一个假浏览器。
 * 位图 = 读不出亮度 = 抬到上限：这一层唯一不许赌的地方。
 */
export function liftAlphaFor(plate: Rgb, text: Rgb, sample: BackdropSample): number {
  if (sample.unknown) return GLASS_FLOAT_ALPHA_CEIL;
  const v = sample.luminance * 255;
  const bg: Rgb = [v, v, v];
  const need = minAlphaForFloor(plate, text, bg);
  // 留一档余量：测量取的是格点上的底色，板子边缘还会折射进更大范围的内容
  return Math.min(GLASS_FLOAT_ALPHA_CEIL, Math.max(GLASS_FLOAT_ALPHA_FLOOR, need * 1.08));
}

/**
 * 壳面在 under 态该画成什么（**颜色 + alpha**），由身后最不利的那一格决定。
 *
 * 2026-11-01 八轮（判词「一滚动，顶栏就变了，不和谐」）把这一档从"二值换板"改成"两块板之间走"：
 * 六轮那条「滚不滚只差 1~2 级」的前提是环境底近平黑（静止合成 45、加厚合成 46），
 * 七轮把环境光挪回画面内之后，静止态在受光处合成到 70~80，而加厚档仍然是 46 ——
 * 内容一开始滚，顶栏从 70 掉到 50，比它身下的内容还暗，读起来就是"换了块板"。
 * 二值那一版还有一个更根本的毛病：加厚档是**近实心的深石墨**，它把身后的环境光一起挡掉了，
 * 而滚过去的往往就是页面自己的底色（画布是透明的，光还在）。
 *
 * 所以深色壳不再固定用 elevated 那块板：
 *   · 静止档（chrome，浅石墨 0.28）压在这块背景上如果能过 AA → 原样用它，**这一屏根本不需要加厚**。
 *     滚过去的是页面自己的底色时正好落在这一支 —— 顶栏与静止态同一个颜色，变化只剩清晰度。
 *   · 不够（白卡片/图片滑到身下）→ 在 chrome → elevated 之间按 t 插值，**颜色和厚度一起走**：
 *     越亮的内容越把它推向"深而实"的那一端，而不是一步跳到近实心。
 * 浅色壳两块板同色（255 白），只有 alpha 在动，仍走 shellAlphaFor 那条原路 —— 那条已被四轮验收过，别顺手动。
 *
 * 为什么 t 用二分：`glassContrast` 对 t 单调（t 越大板越深越实，浅字对比越高），
 * 而且 t=1 那一格就是令牌本身（按纯白极值取的，门禁已钉住），所以必然有解。
 */
export function shellPlateFor(
  chrome: Rgba,
  elevated: Rgba,
  text: Rgb,
  sample: BackdropSample,
): Rgba {
  const chromeRgb: Rgb = [chrome[0], chrome[1], chrome[2]];
  const elevatedRgb: Rgb = [elevated[0], elevated[1], elevated[2]];
  // 浅色外观：加厚板比静止板亮/同色 → 厚度独立自适应（原逻辑）
  if (srgbLuminance(elevatedRgb) >= srgbLuminance(chromeRgb)) {
    return [elevated[0], elevated[1], elevated[2], shellAlphaFor(elevated, chrome, text, sample)];
  }
  if (sample.unknown) return elevated;
  const v = sample.luminance * 255;
  const bg: Rgb = [v, v, v];
  // 身后是页面自己的底色：静止档本来就够读，回到它 = 滚动前后同一块板
  if (glassContrast(chromeRgb, chrome[3], bg, text) >= GLASS_TEXT_FLOOR) return chrome;
  let lo = 0;
  let hi = 1;
  for (let i = 0; i < 12; i++) {
    const mid = (lo + hi) / 2;
    const p = mixRgba(chrome, elevated, mid);
    if (glassContrast([p[0], p[1], p[2]], p[3], bg, text) >= GLASS_TEXT_FLOOR) hi = mid;
    else lo = mid;
  }
  return mixRgba(chrome, elevated, hi);
}

/**
 * 浅色外观的壳面加厚档该用多大 alpha（= 旧 liftAlphaForShell）。
 *
 * 地板不是写死的"静止档 alpha"，而是由**两块板的深浅关系**决定 —— 这是全站三条壳边
 * 读起来是不是同一种材质的判据：
 *   · 加厚板比静止板**亮**（浅色外观：255 白 vs 252 白）→ 地板 = 静止档 alpha。
 *     薄一档只是更透，不会把壳读成另一块板；「一滚就发白」要消灭的是 0.58 → 0.88 那个跳变。
 *   · 加厚板比静止板**深**的那一支（深色）不再走这里 —— 见 shellPlateFor。
 * 上限 = 令牌里那枚保守值（0.88）：量不出来（撞上位图）就退回它 ——
 * 这一层只会让壳更透，绝不会比"没装它"的时候更看不见。
 */
export function shellAlphaFor(
  elevated: Rgba,
  chrome: Rgba,
  text: Rgb,
  sample: BackdropSample,
): number {
  if (sample.unknown) return elevated[3];
  const plate: Rgb = [elevated[0], elevated[1], elevated[2]];
  const v = sample.luminance * 255;
  const bg: Rgb = [v, v, v];
  // 停在地板上时**不乘余量**：那 8% 是给"确实要加厚"的情况留的，
  // 乘在地板上就变成"一滚就比停着厚一点"，正是这条自适应要消灭的跳变。
  const need = minAlphaForFloor(plate, text, bg, GLASS_TEXT_FLOOR, chrome[3], elevated[3]);
  if (need <= chrome[3]) return chrome[3];
  return Math.min(elevated[3], Math.max(chrome[3], need * 1.08));
}

/**
 * 顶栏/底栏：算出它 under 态该画成什么并把**颜色 + alpha**写到元素上（返回写进去的值，没写则 null）。
 * 只认「背景确实由 `--apple-surface-elevated` 决定」的元素。
 * 实心材质（含"减弱透明度"退化档）直接不量 —— 那一档是实心 hex，加厚与静止同色，
 * 没有可透的东西，撒点测量纯属白付。
 */
export function applyShellLift(el: HTMLElement): Rgba | null {
  if (typeof document === 'undefined' || !el.isConnected) return null;
  const elevated = resolvePlate(SHELL_LIFT_PROP);
  const chrome = resolvePlate(CHROME_PROP);
  const text = resolveTextTier('--apple-text-tertiary');
  if (!elevated || !chrome || !text) return null;
  if (elevated[3] >= 0.999) return null;

  const own = parseCssColor(getComputedStyle(el).backgroundColor);
  // 上一次写进去的是两块板之间的插值，所以"是不是我们自己写的"不能拿 elevated 单点比，
  // 要整条线段比（任何 t 都落在这条线段上）；对不上就是上一套主题留下的旧值，清掉。
  if (!own || !onPlateSegment(own, chrome, elevated)) {
    el.style.removeProperty(SHELL_LIFT_PROP);
    return null;
  }

  const rect = el.getBoundingClientRect();
  if (rect.width < 1 || rect.height < 1) return null;

  // 深色壳上是浅字，最亮的那格才是不利的那格（压在白卡片上必须加厚）；浅色壳反过来。
  const deepens =
    srgbLuminance([elevated[0], elevated[1], elevated[2]]) <
    srgbLuminance([chrome[0], chrome[1], chrome[2]]);
  const applied = shellPlateFor(
    chrome,
    elevated,
    [text[0], text[1], text[2]],
    sampleBackdrop(rect, el, SHELL_SAMPLE_MAX, deepens ? 'max' : 'min'),
  );
  el.style.setProperty(
    SHELL_LIFT_PROP,
    `rgba(${Math.round(applied[0])},${Math.round(applied[1])},${Math.round(applied[2])},${applied[3].toFixed(3)})`,
  );
  return applied;
}

/** `c` 是否落在 chrome→elevated 这条线段上（逐通道，容差 1 级） */
function onPlateSegment(c: Rgba, a: Rgba, b: Rgba): boolean {
  for (let i = 0; i < 3; i++) {
    const lo = Math.min(a[i] ?? 0, b[i] ?? 0) - 1;
    const hi = Math.max(a[i] ?? 0, b[i] ?? 0) + 1;
    if ((c[i] ?? 0) < lo || (c[i] ?? 0) > hi) return false;
  }
  return true;
}

export interface ShellGlassLiftOptions {
  /** 与 useScrollEdge 同一个滚动容器（顶栏钉在视口，不在它的祖先链上，必须指名） */
  scroller: () => Element | Window | null;
  /** 只有 under 态才加厚：静止态顶栏铺的是 chrome 那一档，覆写 elevated 没人读 */
  edge: Readonly<Ref<ScrollEdgeState>>;
}

/**
 * 顶栏的加厚跟着**它身后正滚过去的东西**走。
 *
 * 为什么顶栏也要这一层：elevated 那一档为了扛「任意外部内容」钉在 0.88/0.96，
 * 于是内容一开始滚，顶栏就从 0.58 直接跳到近实心 —— 浅色外观读作一条白板、
 * 深色外观读作一条灰板（用户判词「topbar 发灰 / 发白」）。
 * 而"顶栏身后是纯黑"是极值假设：实测绝大多数时候它身后是页面自己的环境底色，
 * 那一屏根本不需要 0.88 —— 这正是浮层面板那档已经用自适应解决掉的同一件事。
 */
export function useShellGlassLift(
  target: Ref<HTMLElement | null>,
  options: ShellGlassLiftOptions,
): void {
  let scroller: Element | Window | null = null;
  let last = 0;
  let pending = 0;

  function measure() {
    const el = target.value;
    if (!el) return;
    if (options.edge.value === 'under') applyShellLift(el);
    else el.style.removeProperty(SHELL_LIFT_PROP);
  }

  function onScroll() {
    const now = Date.now();
    const wait = SHELL_LIFT_INTERVAL - (now - last);
    if (wait > 0) {
      // 尾部补一次：停手那一刻的最终厚度必须由最后一次滚动决定，不是由节流窗口决定的
      if (!pending) {
        pending = window.setTimeout(() => {
          pending = 0;
          last = Date.now();
          measure();
        }, wait);
      }
      return;
    }
    last = now;
    measure();
  }

  onMounted(() => {
    scroller = options.scroller() ?? (target.value ? nearestScroller(target.value) : null);
    // 进场也可能已经带着滚动位置（浏览器恢复 / 锚点跳转），那时没有任何 scroll 事件会来
    requestAnimationFrame(measure);
    if (scroller) scroller.addEventListener('scroll', onScroll, { passive: true });
  });

  onUnmounted(() => {
    if (pending) {
      clearTimeout(pending);
      pending = 0;
    }
    if (scroller) scroller.removeEventListener('scroll', onScroll);
    scroller = null;
  });
}

/**
 * 给一块浮层面板算出它该用的 alpha 并写到元素上；返回写进去的值（没写则 null）。
 * 只认「背景确实由 `--apple-surface-float` 决定」的元素 —— 顶栏那层壳面走的是
 * chrome / elevated 两档，由 applyShellLift 单独负责（共用同一套测量，落点是另一枚令牌）。
 */
export function applyGlassLift(el: HTMLElement): number | null {
  if (typeof document === 'undefined' || !el.isConnected) return null;
  const plate = resolveFloatPlate();
  const text = resolveTextTier('--apple-text-tertiary');
  if (!plate || !text) return null;

  const own = parseCssColor(getComputedStyle(el).backgroundColor);
  if (!own || own[0] !== plate[0] || own[1] !== plate[1] || own[2] !== plate[2]) {
    // 与壳面同理：这条内联覆写若来自上一套主题，它会让这块板一直停在旧颜色上
    el.style.removeProperty(LIFT_PROP);
    return null;
  }

  const rect = el.getBoundingClientRect();
  if (rect.width < 1 || rect.height < 1) return null;

  const applied = liftAlphaFor(plate, text, sampleBackdrop(rect, el));
  el.style.setProperty(LIFT_PROP, `rgba(${plate[0]},${plate[1]},${plate[2]},${applied.toFixed(3)})`);
  return applied;
}

const tracked = new WeakSet<Element>();
let installed = false;

/** 只有这个类是"浮层家族"的现成标记；精确判定留给 applyGlassLift（它会拒掉壳面） */
const FLOAT_MARK = 'apple-glass-layer';

function schedule(el: HTMLElement) {
  if (!el.classList?.contains(FLOAT_MARK) || tracked.has(el)) return;
  tracked.add(el);
  requestAnimationFrame(() => { applyGlassLift(el); });
}

/**
 * 全站唯一接线处：谁铺了浮层面板档，就在它进 DOM 的那一帧量一次；
 * 安装时先补扫一遍已存在的（首帧渲染的浮层走不到 childList）。
 * 用 MutationObserver 而不是让 16 个组件各自 onMounted 调一遍 ——
 * 少一处接线就少一处"忘了接而静态看不出来"（materialize 那笔债正是这么欠下的）。
 * 外观/材质一换就重算：换主题会同时换掉板的 rgb 与文字档，留着旧 alpha 等于把上一套主题的
 * 测量结果压在新一套的背景上 —— 那正是这一层要消灭的那类错。
 */
export function installGlassAdaptive(): void {
  if (installed || typeof MutationObserver === 'undefined') return;
  installed = true;

  new MutationObserver((records) => {
    let themeChanged = false;
    for (const rec of records) {
      if (rec.type === 'attributes') { themeChanged = true; continue; }
      for (const node of rec.addedNodes) {
        if (!(node instanceof HTMLElement)) continue;
        schedule(node);
        if (node.classList?.contains(FLOAT_MARK)) continue;
        for (const child of node.querySelectorAll<HTMLElement>('.apple-glass-layer')) schedule(child);
      }
    }
    if (!themeChanged) return;
    // 壳面那枚也一起清：顶栏是常驻元素，不靠这里的 schedule 重量（它自己在滚动时量），
    // 但换主题后旧的 alpha 必须当场作废，否则下次滚动前一直压着上一套主题的厚度。
    for (const el of document.querySelectorAll<HTMLElement>('[style]')) {
      const lifted = el.style?.getPropertyValue(SHELL_LIFT_PROP);
      const float = el.style?.getPropertyValue(LIFT_PROP);
      if (!lifted && !float) continue;
      el.style.removeProperty(SHELL_LIFT_PROP);
      el.style.removeProperty(LIFT_PROP);
      tracked.delete(el);
      if (float) schedule(el);
    }
    // 观察 documentElement 而不是 body：主题/材质属性写在 html 上，html 不是 body 的后代，
    // 挂在 body 上时 themeChanged 永远为 false —— 那圈“换主题即作废旧厚度”的 sweep 会成死代码。
  }).observe(document.documentElement, {
    childList: true,
    subtree: true,
    attributes: true,
    attributeFilter: ['data-theme', 'data-material', 'data-glass'],
  });

  // 安装发生在 app.mount() 之后（见 main.ts）：首帧就渲染出来的浮层不会产生 childList 记录，
  // 于是永远停在令牌默认厚度。阅读页右侧目录就是这一类的现成例子 —— 打开文档不点任何东西，
  // 它一直是 0.80 那块深色板；"玻璃只在浮层上成立"的观感一半坏在这里。补一次存量扫描。
  document.querySelectorAll<HTMLElement>('.apple-glass-layer').forEach((el) => schedule(el));
}
