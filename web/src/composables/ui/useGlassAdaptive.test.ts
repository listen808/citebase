import { describe, it, expect, vi } from 'vitest';
import { installGlassAdaptive, liftAlphaFor, sampleBackdrop, shellAlphaFor, shellPlateFor } from './useGlassAdaptive';
import {
  compositeOver,
  GLASS_FLOAT_ALPHA_CEIL,
  GLASS_FLOAT_ALPHA_FLOOR,
  GLASS_TEXT_FLOOR,
  glassContrast,
  mixRgba,
  srgbLuminance,
  type Rgb,
  type Rgba,
} from '@/utils/core/glass-contrast';

const WHITE_PLATE: Rgb = [250, 252, 255];
const DARK_TEXT: Rgb = [0x58, 0x58, 0x5e];
const rect = (x: number, y: number, w: number, h: number) =>
  ({ left: x, top: y, right: x + w, bottom: y + h, width: w, height: h, x, y }) as DOMRect;

/**
 * 造一棵**嵌套**的层：`backs[0]` 是最外面（最底）那层，依次往里，最里是那块面板。
 * 必须嵌套，不能写成并列兄弟 —— 真实页面就是 `body > 遮罩 > 面板`，
 * 而"自底向上合成"这个顺序只在祖先链上才有意义。
 * （兄弟节点那版测试放走过一个真 bug：顺着往上合成时，最外层不透明的页面底
 * 会反过来盖掉遮罩，于是弹窗和菜单解到同一个下限。）
 */
function nest(panelStyle: string, backs: string[]): HTMLElement {
  document.body.innerHTML = '';
  let parent: HTMLElement = document.body;
  for (const css of backs) {
    const el = document.createElement('div');
    el.setAttribute('style', css);
    parent.appendChild(el);
    parent = el;
  }
  const panel = document.createElement('div');
  panel.setAttribute('style', panelStyle);
  parent.appendChild(panel);
  return panel;
}

/** 运行时看到的那条采样栈：面板在最上，页面底在最下 */
function ancestorsOf(from: Element): Element[] {
  const out: Element[] = [];
  for (let n: Element | null = from; n; n = n.parentElement) out.push(n);
  return out;
}

function withStack<T>(stack: Element[], run: () => T): T {
  const original = document.elementsFromPoint;
  document.elementsFromPoint = (() => stack) as typeof document.elementsFromPoint;
  try {
    return run();
  } finally {
    document.elementsFromPoint = original;
  }
}

describe('liftAlphaFor：一次测量到一档透明度', () => {
  it('身后是位图（读不出亮度）→ 抬到上限，不许赌它浅', () => {
    expect(liftAlphaFor(WHITE_PLATE, DARK_TEXT, { luminance: 0.9, unknown: true })).toBe(GLASS_FLOAT_ALPHA_CEIL);
  });

  it('身后是页面自己的浅色环境 → 落到下限附近，这才是"看着像玻璃"那一档', () => {
    const a = liftAlphaFor(WHITE_PLATE, DARK_TEXT, { luminance: 0.86, unknown: false });
    expect(a).toBeLessThanOrEqual(GLASS_FLOAT_ALPHA_FLOOR + 0.06);
  });

  it('无论落在哪一档，对**量到的那个背景**都必须过地板（区间内逐点扫）', () => {
    for (let l = 0; l <= 1.0001; l += 0.04) {
      const a = liftAlphaFor(WHITE_PLATE, DARK_TEXT, { luminance: l, unknown: false });
      const bg: Rgb = [l * 255, l * 255, l * 255];
      const got = glassContrast(WHITE_PLATE, a, bg, DARK_TEXT);
      // 撞上下限说明背景本来就浅；撞到上限说明够不着，由门禁那一端兜
      if (a > GLASS_FLOAT_ALPHA_FLOOR && a < GLASS_FLOAT_ALPHA_CEIL) {
        expect(got).toBeGreaterThanOrEqual(GLASS_TEXT_FLOOR - 0.02);
      }
    }
  });

  it('遮罩那一档亮度（0.29）必须解回保守厚度，不是下限', () => {
    const a = liftAlphaFor(WHITE_PLATE, DARK_TEXT, { luminance: 0.29, unknown: false });
    expect(a).toBeGreaterThan(0.7);
  });
});

describe('壳层加厚档：浅色只调厚度', () => {
  // 浅色玻璃那一套：加厚档 0.88、静止档 0.58、三级文字 #58585e（与 check-contrast 同口径）
  const ELEVATED: Rgba = [255, 255, 255, 0.88];
  const CHROME: Rgba = [252, 253, 255, 0.58];
  const CHROME_A = CHROME[3];
  const LIGHT_ENV = srgbLuminance([238, 243, 252]);

  it('浅色：身后是页面自己的环境底色 → 停在静止档，不再一滚就跳到令牌那一档', () => {
    // 这就是「topbar 滚动时发白」那一格：0.58 → 0.88 的跳变原本无条件发生
    const a = shellAlphaFor(ELEVATED, CHROME, DARK_TEXT, { luminance: LIGHT_ENV, unknown: false });
    expect(a).toBeCloseTo(CHROME_A, 3);
  });

  it('撞上位图（量不出亮度）→ 退回令牌值：这一层绝不会把顶栏变得比现在更透', () => {
    expect(shellAlphaFor(ELEVATED, CHROME, DARK_TEXT, { luminance: 0, unknown: true })).toBe(ELEVATED[3]);
    expect(shellPlateFor(CHROME, ELEVATED, DARK_TEXT, { luminance: 0, unknown: true })).toEqual(ELEVATED);
  });

  it('浅色：加厚只能往上，任何背景下都在 [静止档, 令牌值] 区间内', () => {
    for (let l = 0; l <= 1.0001; l += 0.05) {
      const a = shellAlphaFor(ELEVATED, CHROME, DARK_TEXT, { luminance: l, unknown: false });
      expect(a).toBeGreaterThanOrEqual(CHROME_A);
      expect(a).toBeLessThanOrEqual(ELEVATED[3]);
    }
  });

  it('浅色：极值内容（纯黑）那一格仍要过 AA —— 令牌那枚保守值就是为它准备的', () => {
    const a = shellAlphaFor(ELEVATED, CHROME, DARK_TEXT, { luminance: 0, unknown: false });
    const plate: Rgb = [ELEVATED[0], ELEVATED[1], ELEVATED[2]];
    expect(glassContrast(plate, a, [0, 0, 0], DARK_TEXT)).toBeGreaterThanOrEqual(GLASS_TEXT_FLOOR);
  });

  // 深色玻璃那一套（2026-11-01 九轮降灰后）：加厚档 44,45,50 / 0.90、静止档 114,116,122 / 0.28、环境底 #0b0b0e
  const DARK_ELEVATED: Rgba = [44, 45, 50, 0.90];
  const DARK_CHROME: Rgba = [114, 116, 122, 0.28];
  const DARK_TEXT_TIER: Rgb = [0xb8, 0xb8, 0xbe];
  const DARK_ENV: Rgb = [11, 11, 14];
  const asRgb = (c: Rgba): Rgb => [c[0], c[1], c[2]];

  it('深色：身后是页面自己的底色 → 原样用静止档，滚动前后是同一块板', () => {
    // 八轮判词「一滚动，顶栏就变了，不和谐」：静止 70~80、加厚 46，内容一滚顶栏比身下还暗。
    // 滚过去的本来就是这个底色（画布透明，光还在），这一屏不需要加厚。
    const plate = shellPlateFor(
      DARK_CHROME, DARK_ELEVATED, DARK_TEXT_TIER,
      { luminance: srgbLuminance(DARK_ENV), unknown: false },
    );
    expect(plate).toEqual(DARK_CHROME);
  });

  it('深色：白卡片滑到身下 → 走到加厚档那一端，且过 AA', () => {
    const plate = shellPlateFor(DARK_CHROME, DARK_ELEVATED, DARK_TEXT_TIER, { luminance: 1, unknown: false });
    // 二分给的是"刚好过 AA 的最小厚度"，纯白上它落在令牌值附近而不是恰好那枚 0.90
    // （令牌是按纯白极值取的上界，本身带一点点余量；九轮压深后余量更大，落点约 0.875）。
    expect(plate[3]).toBeGreaterThan(0.86);
    expect(glassContrast(asRgb(plate), plate[3], [255, 255, 255], DARK_TEXT_TIER))
      .toBeGreaterThanOrEqual(GLASS_TEXT_FLOOR);
  });

  it('深色：任何背景亮度下都过 AA（二分解不出时才停在令牌值）', () => {
    for (let l = 0; l <= 1.0001; l += 0.05) {
      const plate = shellPlateFor(DARK_CHROME, DARK_ELEVATED, DARK_TEXT_TIER, { luminance: l, unknown: false });
      expect(glassContrast(asRgb(plate), plate[3], [l * 255, l * 255, l * 255], DARK_TEXT_TIER))
        .toBeGreaterThanOrEqual(GLASS_TEXT_FLOOR - 0.02);
    }
  });

  it('深色：加厚是连续的 —— 背景越亮，板越深越实，不会一步跳档', () => {
    const lums = [0.02, 0.1, 0.3, 0.6, 0.9];
    const plates = lums.map((l) => shellPlateFor(DARK_CHROME, DARK_ELEVATED, DARK_TEXT_TIER, { luminance: l, unknown: false }));
    for (let i = 1; i < plates.length; i++) {
      const prev = plates[i - 1] as Rgba;
      const cur = plates[i] as Rgba;
      expect(cur[3]).toBeGreaterThanOrEqual(prev[3] - 1e-6);
      expect(srgbLuminance(asRgb(cur))).toBeLessThanOrEqual(srgbLuminance(asRgb(prev)) + 1e-6);
    }
    // 落在两块板之间：不是任何一格都等于某一个端点
    const mid = shellPlateFor(DARK_CHROME, DARK_ELEVATED, DARK_TEXT_TIER, { luminance: 0.45, unknown: false });
    expect(mid[3]).toBeGreaterThan(DARK_CHROME[3]);
    expect(mid[3]).toBeLessThan(DARK_ELEVATED[3]);
    // 颜色与厚度必须是同一个 t 插出来的：只改 alpha 会让中间那一档变成"灰得发白"的另一块板。
    const expected = mixRgba(DARK_CHROME, DARK_ELEVATED, (mid[3] - DARK_CHROME[3]) / (DARK_ELEVATED[3] - DARK_CHROME[3]));
    for (let i = 0; i < 4; i++) expect(mid[i]).toBeCloseTo(expected[i] as number, 6);
  });

  it('深色：加厚档与静止档压在**同一块底色**上必须合成同一个明度（Δ ≤ 6 级）', () => {
    // 这条才是「三条壳边是一种材质」的判据：换档只该改变"透多少"，不该改变"是什么颜色"。
    // 底色要用环境底自己的 rgb 去合成，不能把它的**亮度**当灰阶通道用 —— 近黑处的亮度
    // （0.0037）与 11/255 差一个量级，那样算出来的差异是假的。
    const rest = compositeOver(DARK_CHROME, DARK_ENV);
    const under = compositeOver(DARK_ELEVATED, DARK_ENV);
    for (let i = 0; i < 3; i++) {
      expect(Math.abs((rest[i] ?? 0) - (under[i] ?? 0))).toBeLessThanOrEqual(6);
    }
  });
});

describe('sampleBackdrop：量的是面板身后被画出来的底色，不是文字', () => {
  it('遮罩压在**不透明的页面底**上：取的是合成结果，不是页面底本身', () => {
    const panel = nest('background-color:rgb(255,255,255)', [
      'background-color:rgb(255,255,255)',
      'background-color:rgba(20,30,60,0.45)',
    ]);
    const s = withStack(ancestorsOf(panel), () => sampleBackdrop(rect(0, 0, 40, 40), panel));
    expect(s.unknown).toBe(false);
    // 45% 的深遮罩压在白页上 = rgb(149,154,167)。不猜"大概多暗"，直接和同一次合成比。
    expect(s.luminance).toBeCloseTo(srgbLuminance([149.25, 153.75, 167.25]), 3);
    expect(s.luminance).toBeLessThan(0.5);
  });

  it('撞上 <img> 就整块判未知 —— 位图的亮度量不出来，只能按最坏处理', () => {
    const panel = nest('background-color:rgb(255,255,255)', ['background-color:rgb(255,255,255)']);
    const page = panel.parentElement as HTMLElement;
    const img = document.createElement('img');
    img.alt = '';
    page.insertBefore(img, panel);
    const s = withStack([panel, img, page], () => sampleBackdrop(rect(0, 0, 40, 40), panel));
    expect(s).toEqual({ luminance: 0, unknown: true });
  });

  it('环境层（body）的渐变算已知，并把最暗 stop 合成进来（不能只量 background-color）', () => {
    document.body.style.backgroundColor = 'rgb(238,243,252)';
    document.body.style.backgroundImage =
      'radial-gradient(circle, rgba(96,140,255,0.34) 0%, rgba(255,255,255,0) 64%)';
    try {
      const panel = nest('background-color:rgb(255,255,255)', []);
      const s = withStack(ancestorsOf(panel), () => sampleBackdrop(rect(0, 0, 40, 40), panel));
      expect(s.unknown).toBe(false);
      // 0.89 的底色 + 0.34 的蓝色 stop ≈ 0.63 —— 旧实现读到的是 0.89
      expect(s.luminance).toBeCloseTo(srgbLuminance([189.72, 207.98, 253.02]), 5);
      expect(s.luminance).toBeLessThan(srgbLuminance([238, 243, 252]));
    } finally {
      document.body.style.backgroundColor = '';
      document.body.style.backgroundImage = '';
    }
  });

  it('非环境层的 CSS 渐变可能是用户内容（封面/主题色块）→ 判未知', () => {
    const panel = nest('background-color:rgb(255,255,255)', [
      'background-color:rgb(238,243,252);background-image:linear-gradient(rgb(0,0,0), rgb(255,255,255))',
    ]);
    const s = withStack(ancestorsOf(panel), () => sampleBackdrop(rect(0, 0, 40, 40), panel));
    expect(s).toEqual({ luminance: 0, unknown: true });
  });

  it('壳面/浮层自己的镜面高光不是未知内容 —— 否则嵌在壳里的菜单会被抬到近实心', () => {
    document.body.style.backgroundColor = 'rgb(10,10,23)';
    try {
      const panel = nest('background-color:rgb(255,255,255)', [
        'background-color:rgba(28,28,30,0.42);' +
          'background-image:radial-gradient(120% 90% at 18% 0%, rgba(255,255,255,0.10), rgba(255,255,255,0) 62%)',
      ]);
      const shell = panel.parentElement as HTMLElement;
      shell.setAttribute('data-glass-surface', 'sidebar');
      const s = withStack(ancestorsOf(panel), () => sampleBackdrop(rect(0, 0, 40, 40), panel));
      expect(s.unknown).toBe(false);
      // 镜面高光被当作材质层跳过，只按壳面底色合成到页面底
      expect(s.luminance).toBeCloseTo(srgbLuminance(compositeOver([28, 28, 30, 0.42], [10, 10, 23])), 5);
    } finally {
      document.body.style.backgroundColor = '';
    }
  });

  it('渐变里出现读不出的颜色（命名色）时整层判未知，不许只合成认得的那几个', () => {
    document.body.style.backgroundColor = 'rgb(238,243,252)';
    document.body.style.backgroundImage = 'linear-gradient(red, transparent)';
    try {
      const panel = nest('background-color:rgb(255,255,255)', []);
      const s = withStack(ancestorsOf(panel), () => sampleBackdrop(rect(0, 0, 40, 40), panel));
      expect(s.unknown).toBe(true);
    } finally {
      document.body.style.backgroundColor = '';
      document.body.style.backgroundImage = '';
    }
  });

  it('`url()` 背景是真位图，按未知处理', () => {
    const panel = nest('background-color:rgb(255,255,255)', [
      'background-color:rgb(255,255,255);background-image:url(/images/hero.png)',
    ]);
    const s = withStack(ancestorsOf(panel), () => sampleBackdrop(rect(0, 0, 40, 40), panel));
    expect(s.unknown).toBe(true);
  });

  it('面板自己与它的后代一律跳过，量到的是它身后那一层', () => {
    document.body.innerHTML = '<div id="page" style="background-color:rgb(0,0,0)"></div>'
      + '<div id="panel" style="background-color:rgb(255,255,255)"><span id="kid">x</span></div>';
    const panel = document.getElementById('panel') as HTMLElement;
    const kid = document.getElementById('kid') as HTMLElement;
    const page = document.getElementById('page') as HTMLElement;
    const s = withStack([kid, panel, page], () => sampleBackdrop(rect(0, 0, 40, 40), panel));
    expect(s.luminance).toBe(0);
    expect(s.unknown).toBe(false);
  });

  it('全局接线观察 documentElement：主题属性写在 html 上，挂在 body 上永远收不到', () => {
    const observed: Array<{ target: Element; init: MutationObserverInit }> = [];
    class FakeObserver {
      observe(target: Element, init: MutationObserverInit) { observed.push({ target, init }); }
      disconnect() {}
      takeRecords() { return []; }
    }
    const original = globalThis.MutationObserver;
    // 安装前就存在的浮层必须被补扫：安装发生在 app.mount() 之后，首帧浮层走不到 childList
    document.body.innerHTML = '';
    const existing = document.createElement('div');
    existing.className = 'apple-glass-layer';
    document.body.appendChild(existing);
    const qsa = vi.spyOn(document, 'querySelectorAll');
    // installGlassAdaptive 有模块级 installed 闩：这个用例必须是全套里第一次调用它
    globalThis.MutationObserver = FakeObserver as unknown as typeof MutationObserver;
    try {
      installGlassAdaptive();
    } finally {
      globalThis.MutationObserver = original;
    }
    expect(observed).toHaveLength(1);
    expect(observed[0]?.target).toBe(document.documentElement);
    expect(observed[0]?.init.subtree).toBe(true);
    expect(observed[0]?.init.attributeFilter).toEqual(['data-theme', 'data-material', 'data-glass']);
    expect(qsa).toHaveBeenCalledWith('.apple-glass-layer');
    qsa.mockRestore();
    existing.remove();
  });

  it('两层遮罩逐层合成：越压越暗', () => {
    // 必须测完一层再建下一层：nest 会清空 body，先建的那块面板就成了脱离文档树的节点，
    // 祖先链取不到 → 采样落进"量不到"那一支，两条都返回 0，看着像通过其实什么都没比。
    const one = nest('background-color:rgb(255,255,255)', [
      'background-color:rgb(255,255,255)', 'background-color:rgba(0,0,0,0.2)',
    ]);
    const l1 = withStack(ancestorsOf(one), () => sampleBackdrop(rect(0, 0, 40, 40), one));
    const two = nest('background-color:rgb(255,255,255)', [
      'background-color:rgb(255,255,255)', 'background-color:rgba(0,0,0,0.6)',
    ]);
    const l2 = withStack(ancestorsOf(two), () => sampleBackdrop(rect(0, 0, 40, 40), two));
    expect(l1.unknown).toBe(false);
    expect(l2.unknown).toBe(false);
    expect(l2.luminance).toBeLessThan(l1.luminance);
  });
});
