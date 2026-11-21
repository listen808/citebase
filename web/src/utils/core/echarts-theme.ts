/**
 * echarts 的主题接线 —— 画布类渲染器读令牌的唯一出口。
 *
 * 为什么必须有这台：hljs 有 `HIGHLIGHT_PALETTE_BY_THEME`、Monaco 有 `MONACO_APPLE_THEMES`、
 * mermaid 有 `mermaidThemeVariables()`，**唯独 echarts 什么都没有**：三处 `echarts.init()`
 * 都是拿服务端/模型给的 option 直接画，既不在主题切换时重绘，也不从令牌取过任何颜色。
 * 表现是切到深色后图表还是那套浅底配色，轴标签压在深面上几乎看不见 —— 而静态门禁全绿，
 * 因为它只保证"源码里没写死颜色"，管不了"画布根本没读令牌"。
 *
 * 与那三处的口径一致：**不抄第二份调色板**，全部现读 CSS 变量（`resolvedColor`），
 * 于是主题层改任何值这里自动跟；也不会出现"注册表漏了一档主题"这种 T6 拦得住的错。
 */
import { resolvedColor } from './apple-tokens';

export interface ChartPalette {
  /** 轴标签 / 图例文字 */
  text: string;
  /** 次级说明文字 */
  subtext: string;
  /** 网格线与轴线 */
  line: string;
  /** tooltip 的壳面（浮层档，含 alpha） */
  surface: string;
  /** 描边 */
  border: string;
  /** 系列取色：全站强调色的固定一族，顺序即优先级 */
  series: string[];
}

/** 系列取色的顺序 —— 从令牌取，不在这里写字面色值（T13 会拦，也避免与主题分叉） */
const SERIES_HUES = ['blue', 'green', 'orange', 'purple', 'teal', 'red', 'indigo', 'pink'] as const;

export function chartPalette(): ChartPalette {
  return {
    text: resolvedColor('text-primary'),
    subtext: resolvedColor('text-secondary'),
    line: resolvedColor('border-light'),
    surface: resolvedColor('surface-elevated'),
    border: resolvedColor('border'),
    series: SERIES_HUES.map((h) => resolvedColor(h)),
  };
}

type Rec = Record<string, unknown>;
const isRec = (v: unknown): v is Rec => !!v && typeof v === 'object' && !Array.isArray(v);

/** 只在对方没写的时候补默认值 —— 模型/服务端显式给的颜色必须赢，这是数据不是主题 */
function fill(target: Rec, patch: Rec): Rec {
  for (const [k, v] of Object.entries(patch)) {
    if (v === undefined || v === null || v === '') continue;
    const cur = target[k];
    if (cur === undefined || cur === null || cur === '') target[k] = v;
    else if (isRec(cur) && isRec(v)) target[k] = fill(cur, v);
  }
  return target;
}

/** 坐标轴可能是单个对象也可能是数组，两种形状都要过一遍 */
function axisPatch(target: unknown, patch: Rec): unknown {
  if (Array.isArray(target)) return target.map((a) => (isRec(a) ? fill(a, patch) : a));
  if (isRec(target)) return fill(target, patch);
  return target;
}

/**
 * 把主题默认值并进一份**已净化**的 option，返回新对象（不改调用方传进来的那份）。
 * 补的全是"文字与线"这一类主题色，不碰 `series[].data`、不碰 `series[].itemStyle.color`
 * 里模型自己挑的颜色。
 */
export function applyChartTheme(option: Rec, palette = chartPalette()): Rec {
  const next: Rec = JSON.parse(JSON.stringify(option));
  const axis = {
    axisLabel: { color: palette.subtext },
    axisLine: { lineStyle: { color: palette.line } },
    splitLine: { lineStyle: { color: palette.line } },
  };
  fill(next, {
    textStyle: { color: palette.text },
    title: { textStyle: { color: palette.text }, subtextStyle: { color: palette.subtext } },
    legend: { textStyle: { color: palette.subtext } },
    tooltip: {
      backgroundColor: palette.surface,
      borderColor: palette.border,
      textStyle: { color: palette.text },
    },
    xAxis: axis,
    yAxis: axis,
  });
  // 轴那种"对象或数组"的形状 fill() 处理不了（它只递归同形对象），单独再过一遍
  for (const key of ['xAxis', 'yAxis'] as const) {
    if (next[key] !== undefined) next[key] = axisPatch(next[key], axis);
  }
  if (!Array.isArray(next.color) && !next.color) next.color = palette.series;
  return next;
}

/**
 * 主题/材质/透明变体变化时回调。**画布类渲染器必须订阅这个**，
 * 因为 `setOption` 是一次性的：切主题不会让已经画好的 canvas 重画。
 *
 * 用 MutationObserver 而不是 `watch(() => ui.theme)`：
 *  ① 材质轴与 clear 变体以前全站没有一处 watch（Monaco 那条 `MonacoEditor.vue:770` 是唯一
 *     同时看两个属性的范本，也只看了 theme/material，没看 glass）；
 *  ② DOM 上的那三个属性就是**生效中的**主题，比 store 状态更接近真相（回落、别名、
 *     `effectiveMaterial()` 把纸感+玻璃塌回实心，都已经在 DOM 上算完了）。
 * 观察者共用一个，回调按订阅者计数增删，免得每个图表各挂一个。
 */
const listeners = new Set<() => void>();
let observer: MutationObserver | null = null;

export function subscribeTheme(cb: () => void): () => void {
  if (typeof document === 'undefined') return () => {};
  if (!observer) {
    observer = new MutationObserver(() => {
      for (const fn of listeners) fn();
    });
    observer.observe(document.documentElement, {
      attributes: true,
      attributeFilter: ['data-theme', 'data-material', 'data-glass'],
    });
  }
  listeners.add(cb);
  return () => {
    listeners.delete(cb);
    if (!listeners.size && observer) {
      observer.disconnect();
      observer = null;
    }
  };
}

