/**
 * P1-7: AI 流式图表/生成式组件把模型返回的整份 echarts option 直传 setOption，
 * 而 echarts tooltip 默认 renderMode:'html' 会走 innerHTML 渲染（TooltipHTMLContent），
 * formatter 字符串模板的字面量部分不做 encodeHTML，模型可控文本可造成 DOM 注入。
 * 这里做“结构白名单 + 危险字符串中和”：
 *  - 丢弃函数类型值（防止 callback 绕过）；
 *  - 含 HTML 标签样式的字符串（`<a`、`</`、`<!--`、`javascript:` 等）统一转义，
 *    echarts 的 `{a}/{b}/{c}` 模板占位符不含尖括号，正常图表配置不受影响；
 *  - 顶层键白名单：只保留已知安全的图表配置段，模型夹带的未知键直接丢弃。
 */
import { escapeHtml } from './shared';

const ALLOWED_TOP_KEYS = new Set([
  'title', 'legend', 'grid', 'series', 'xAxis', 'yAxis', 'tooltip', 'dataset',
  'color', 'backgroundColor', 'radar', 'visualMap', 'dataZoom', 'polar',
  'angleAxis', 'radiusAxis', 'geo', 'toolbox', 'graphic', 'animation',
  'animationDuration', 'smooth', 'name', 'type', 'data', 'itemStyle', 'lineStyle',
  'areaStyle', 'label', 'emphasis', 'axisLabel', 'axisLine', 'splitLine', 'encode',
]);

const HTML_LIKE = /(<\s*[a-zA-Z/!]|javascript\s*:|on\w+\s*=)/;

function sanitizeValue(value: unknown): unknown {
  if (typeof value === 'function') return undefined; // 函数一律丢弃
  if (typeof value === 'string') {
    return HTML_LIKE.test(value) ? escapeHtml(value) : value;
  }
  if (Array.isArray(value)) {
    return value.map(v => sanitizeValue(v)).filter(v => v !== undefined);
  }
  if (value && typeof value === 'object') {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      const sv = sanitizeValue(v);
      if (sv !== undefined) out[k] = sv;
    }
    return out;
  }
  // number/boolean/null 原样保留
  return value;
}

export function sanitizeEChartsOption(option: unknown): Record<string, unknown> | null {
  if (!option || typeof option !== 'object' || Array.isArray(option)) return null;
  const src = option as Record<string, unknown>;
  const out: Record<string, unknown> = {};
  for (const [k, v] of Object.entries(src)) {
    if (!ALLOWED_TOP_KEYS.has(k)) continue; // 未知顶层键丢弃
    const sv = sanitizeValue(v);
    if (sv !== undefined) out[k] = sv;
  }
  return Object.keys(out).length ? out : null;
}
