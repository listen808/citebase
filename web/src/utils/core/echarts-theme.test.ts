import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { applyChartTheme, chartPalette, subscribeTheme } from '@/utils/core/echarts-theme';

/**
 * 画布类渲染器的主题接线。测的不是"配色好不好看"，而是三条会静默失效的机制：
 *  ① 默认值只在**对方没写**的时候补 —— 模型/服务端显式给的颜色是数据，不能被主题盖掉；
 *  ② 坐标轴"对象或数组"两种形状都要过（漏了数组那一支，多轴图就是死的）；
 *  ③ 订阅者退订之后 observer 必须断开，否则聊天气泡每出一条图表就永久挂住一个重绘回调。
 *
 * 配色本身来自 CSS 变量，happy-dom 里没有真的样式表，所以这里 stub `getComputedStyle`
 * 喂一组可辨认的值 —— 判的是"取的是令牌、不是字面色"，不是具体某个 hex。
 */
const TOKENS: Record<string, string> = {
  '--apple-text-primary': '#111111',
  '--apple-text-secondary': '#222222',
  '--apple-border-light': '#333333',
  '--apple-border': '#444444',
  '--apple-surface-elevated': 'rgba(250, 250, 250, 0.9)',
  '--apple-blue': '#0000ff',
  '--apple-green': '#00ff00',
  '--apple-orange': '#ffa500',
  '--apple-purple': '#800080',
  '--apple-teal': '#008080',
  '--apple-red': '#ff0000',
  '--apple-indigo': '#4b0082',
  '--apple-pink': '#ffc0cb',
};

const original = window.getComputedStyle;
beforeEach(() => {
  vi.spyOn(window, 'getComputedStyle').mockImplementation(((_el: Element) => ({
    getPropertyValue: (name: string) => TOKENS[name] ?? '',
  }) as CSSStyleDeclaration) satisfies typeof window.getComputedStyle);
});
afterEach(() => {
  vi.restoreAllMocks();
  window.getComputedStyle = original;
});

describe('chartPalette', () => {
  it('全部取自令牌，一个色面都不自己写', () => {
    const p = chartPalette();
    expect(p.text).toBe('#111111');
    expect(p.surface).toBe('rgba(250, 250, 250, 0.9)');
    expect(p.series).toEqual(['#0000ff', '#00ff00', '#ffa500', '#800080', '#008080', '#ff0000', '#4b0082', '#ffc0cb']);
  });
});

describe('applyChartTheme', () => {
  it('没写颜色的地方补上主题值', () => {
    const out = applyChartTheme({ xAxis: { type: 'category' }, series: [{ type: 'bar' }] });
    expect(out.textStyle).toEqual({ color: '#111111' });
    expect((out.xAxis as Record<string, never>).axisLabel).toEqual({ color: '#222222' });
    expect(out.color).toEqual(chartPalette().series);
  });

  it('显式给的颜色赢 —— 这是数据不是主题', () => {
    const out = applyChartTheme({
      color: ['#abcdef'],
      textStyle: { color: '#000001' },
      xAxis: [{ axisLabel: { color: '#000002' } }, { type: 'value' }],
    });
    expect(out.color).toEqual(['#abcdef']);
    expect((out.textStyle as Record<string, never>).color).toBe('#000001');
    const [first, second] = out.xAxis as Array<Record<string, never>>;
    expect(first?.axisLabel).toEqual({ color: '#000002' });
    expect(second?.axisLabel).toEqual({ color: '#222222' });
  });

  it('不修改传进来的那份 option（调用方还要拿它做别的）', () => {
    const src = { xAxis: { type: 'category' } };
    applyChartTheme(src);
    expect(src).toEqual({ xAxis: { type: 'category' } });
  });

  it('series.data 原样保留', () => {
    const out = applyChartTheme({ series: [{ type: 'line', data: [1, 2, 3], itemStyle: { color: '#ffee00' } }] });
    const s = (out.series as Array<Record<string, unknown>>)[0] as Record<string, unknown>;
    expect(s.data).toEqual([1, 2, 3]);
    expect((s.itemStyle as Record<string, never>).color).toBe('#ffee00');
  });
});

describe('subscribeTheme', () => {
  it('改 data-material / data-glass 也要回调（材质轴以前全站没人 watch）', async () => {
    const seen = vi.fn();
    const off = subscribeTheme(seen);
    const root = document.documentElement;
    root.setAttribute('data-material', 'glass');
    await new Promise((r) => setTimeout(r, 0));
    expect(seen).toHaveBeenCalled();
    root.setAttribute('data-glass', 'clear');
    await new Promise((r) => setTimeout(r, 0));
    expect(seen.mock.calls.length).toBeGreaterThanOrEqual(2);
    off();
  });

  it('退订后不再回调，且最后一个退订会把 observer 断开', async () => {
    const a = vi.fn();
    const b = vi.fn();
    const offA = subscribeTheme(a);
    const offB = subscribeTheme(b);
    offA();
    document.documentElement.setAttribute('data-theme', 'dark');
    await new Promise((r) => setTimeout(r, 0));
    expect(a).not.toHaveBeenCalled();
    expect(b).toHaveBeenCalled();
    offB();
    const spy = vi.spyOn(MutationObserver.prototype, 'disconnect');
    document.documentElement.setAttribute('data-theme', 'light');
    await new Promise((r) => setTimeout(r, 0));
    expect(spy).not.toHaveBeenCalled(); // 已经断开过，不该再收到回调
    offB(); // 幂等
  });
});
