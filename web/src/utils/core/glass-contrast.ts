/**
 * 玻璃浮层的对比度数学 —— **唯一一份**。
 *
 * 为什么单独成模块：浮层的自适应加厚要在浏览器里解「这块板压在这个背景上，
 * 要多大 alpha 才够 AA」，而同一条不等式 `check:contrast` 也要判。
 * 两边各抄一份，就是本仓库反复付过学费的那件事（E1 圆角抄成空集、
 * clear 变体抄成第二份色值）。所以门禁与运行时都从这里取函数与常数。
 *
 * 只做纯函数：不碰 DOM、不读令牌。取值由调用方负责。
 */

export type Rgb = [number, number, number];
export type Rgba = [number, number, number, number];

/** 浮层上的文字按 AA 判（正文那一档在浮层上基本不出现，出现了只会更容易过） */
export const GLASS_TEXT_FLOOR = 4.5;

/**
 * 自适应能解出的上下限。
 * 下限不是为对比度（背景越浅越不需要板），而是为了「这块东西是浮起来的」这个读法，
 * 并且它必须**高于内容板那一档的实心度**：内容卡是 0.44~0.50 的半透板，浮层若解到 0.40
 * 就比它身下的卡片更透，屏幕上读出来是"卡片浮在菜单上"，层级整个反掉
 * （2026-11-01 运行时普查实测：菜单 0.43 压在 0.44 的卡片上，肉眼分不出谁在前）。
 * 上限不给 1.0：身后是一张真暗图时，加厚到 0.94 已经够 AA，再多就是回到实心，
 * 而材质一旦实心，materialize 与折射就都没意义了。
 */
export const GLASS_FLOAT_ALPHA_FLOOR = 0.55;
export const GLASS_FLOAT_ALPHA_CEIL = 0.94;

/** WCAG 相对亮度（sRGB，线性化按规范那条分段函数） */
export function srgbLuminance(c: Rgb | Rgba): number {
  const lin = (v: number) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  };
  return 0.2126 * lin(c[0]) + 0.7152 * lin(c[1]) + 0.0722 * lin(c[2]);
}

/** 半透明前景合成到不透明底色上（sRGB 逐通道，玻璃这层不需要线性空间） */
export function compositeOver(fg: Rgba, bg: Rgb): Rgb {
  const a = fg[3];
  return [a * fg[0] + (1 - a) * bg[0], a * fg[1] + (1 - a) * bg[1], a * fg[2] + (1 - a) * bg[2]];
}

/** 两个颜色之间的 WCAG 对比度 */
export function contrastRatio(a: Rgb | Rgba, b: Rgb | Rgba): number {
  const l1 = srgbLuminance(a);
  const l2 = srgbLuminance(b);
  return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
}

/** 文字压在一块 alpha 可变的玻璃板上，板压在背景上 */
export function glassContrast(plate: Rgb, alpha: number, bg: Rgb, text: Rgb): number {
  return contrastRatio(text, compositeOver([plate[0], plate[1], plate[2], alpha], bg));
}

/**
 * 解「要多大 alpha 才让文字达到 floor」。
 *
 * 单调性：alpha 越大，浅色外观下的板越亮、深字对比越高（深色外观方向相反），
 * 所以二分够用，不需要闭式解 —— 闭式解要按「文字比背景亮还是暗」分支，
 * 那正是抄两份数学最容易抄错的地方。
 * 解不出来（任何 alpha 都不够，例如浅外观下深字压纯白背景）时返回上限：
 * 加厚是这里唯一能做的事，但调用方不该把它当成"已经安全了"。
 *
 * alpha 的两个端点默认是**浮层**那一档（0.55~0.94）。壳面调用要换成自己的端点：
 * 顶栏的地板是它的静止档 chrome，不是浮层的地板 —— 用浮层那对端点会把"身后不需要加厚"
 * 的顶栏也抬到 0.594，实测就是"一滚就比停着厚"（useScrollEdge 文件头写的那条反例）。
 */
export function minAlphaForFloor(
  plate: Rgb,
  text: Rgb,
  bg: Rgb,
  floor = GLASS_TEXT_FLOOR,
  alphaFloor = GLASS_FLOAT_ALPHA_FLOOR,
  alphaCeil = GLASS_FLOAT_ALPHA_CEIL,
): number {
  if (glassContrast(plate, alphaCeil, bg, text) < floor) return alphaCeil;
  if (glassContrast(plate, alphaFloor, bg, text) >= floor) return alphaFloor;
  let lo = alphaFloor;
  let hi = alphaCeil;
  for (let i = 0; i < 24; i++) {
    const mid = (lo + hi) / 2;
    if (glassContrast(plate, mid, bg, text) >= floor) hi = mid;
    else lo = mid;
  }
  return hi;
}

/** 两块板之间按 t 插值（含 alpha）：深色壳的静止档与加厚档颜色不同，厚度要在两块板之间走 */
export function mixRgba(a: Rgba, b: Rgba, t: number): Rgba {
  return [
    a[0] + (b[0] - a[0]) * t,
    a[1] + (b[1] - a[1]) * t,
    a[2] + (b[2] - a[2]) * t,
    a[3] + (b[3] - a[3]) * t,
  ];
}

/** `rgb(250, 252, 255)` / `rgba(250,252,255,0.78)` / `#fafcff` → Rgba；解不出来返回 null */export function parseCssColor(s: string | null | undefined): Rgba | null {
  const t = (s ?? '').trim();
  if (!t) return null;
  const hex = /^#([0-9a-f]{3,8})$/i.exec(t);
  if (hex) {
    const raw = hex[1];
    if (!raw) return null;
    let h = raw;
    if (h.length === 3 || h.length === 4) h = h.split('').slice(0, 3).map((c) => c + c).join('');
    h = h.slice(0, 6);
    const a = raw.length === 8 ? parseInt(raw.slice(6, 8), 16) / 255 : 1;
    return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16), a];
  }
  const fn = /^rgba?\(([^)]+)\)$/i.exec(t);
  if (fn) {
    const body = fn[1];
    if (!body) return null;
    const n = body.replace(/\//g, ' ').split(/[\s,]+/).filter(Boolean).map(Number);
    const r = n[0];
    const g = n[1];
    const b = n[2];
    if (r === undefined || g === undefined || b === undefined) return null;
    if ([r, g, b].some((v) => Number.isNaN(v))) return null;
    return [r, g, b, n.length > 3 ? (n[3] ?? 1) : 1];
  }
  return null;
}
