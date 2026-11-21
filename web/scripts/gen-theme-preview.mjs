#!/usr/bin/env node
/**
 * 从真实主题数据生成预览页 —— npm run gen:theme-preview
 *
 * 手写的那份 docs/ui-previews/theme-global-preview.html（11 套蓝图）会与实现分叉
 * （它把液态玻璃画成暗色，而实现是浅色清玻璃），2026-10-08 已删除 —— 双轴重构后"11 套"这个口径本身就作废了。
 * 本脚本把已落地主题的实际注入结果
 * 导出成一个可直接用浏览器打开的切换页：改主题 → 重跑脚本，预览不会与实现分叉。
 *
 * 注意：主题必须作用在 <html data-theme> 上。派生染色族（*-bg-subtle / callout-*）
 * 是 :root 上的 color-mix(var(--apple-x))，只有与主题覆写同一元素时才会重新解析。
 */
import fs from 'fs';
import path from 'path';
import module from 'node:module';
import { buildSync } from 'esbuild';

const root = path.resolve('.');
const bundle = buildSync({
  stdin: {
    contents: `
      export { cssVariables, AppleSpace, AppleType, AppleFont } from './src/utils/core/apple-tokens';
      export { THEME_CONTRACT_KEYS, THEME_REGISTRY, themeExtraCss, themeOverrideMapFor } from './src/styles/theme/themes';
      export { HighlightColors, HIGHLIGHT_PALETTE_BY_THEME } from './src/styles/theme/highlight-tokens';
    `,
    resolveDir: root,
    loader: 'ts',
  },
  bundle: true,
  format: 'cjs',
  platform: 'node',
  write: false,
  absWorkingDir: root,
});

const req = module.createRequire(path.join(root, 'noop.js'));
const mod = { exports: {} };
new Function('module', 'exports', 'require', bundle.outputFiles[0].text)(mod, mod.exports, req);
const {
  cssVariables,
  AppleSpace,
  AppleType,
  AppleFont,
  THEME_CONTRACT_KEYS,
  THEME_REGISTRY,
  themeExtraCss,
  themeOverrideMapFor,
  HighlightColors,
  HIGHLIGHT_PALETTE_BY_THEME,
} = mod.exports;

const OUT = path.resolve('../docs/ui-previews/theme-live-preview.html');

const varsBlock = (map) =>
  Object.entries(map)
    .map(([k, v]) => `  ${k}: ${v};`)
    .join('\n');

const hlVars = (palette) =>
  Object.fromEntries(Object.entries(HighlightColors[palette]).map(([k, v]) => [`--hl-${k}`, v]));

/** 与 injectCSSVariables() / injectHighlightCSSVariables() 产出同构的块 */
const themeCss = THEME_REGISTRY.map(({ id }) => {
  const map = themeOverrideMapFor(id);
  const hl = `:root[data-theme='${id}'] {\n${varsBlock(hlVars(HIGHLIGHT_PALETTE_BY_THEME[id]))}\n}`;
  const vars = map ? `:root[data-theme='${id}'] {\n${varsBlock(map)}\n}\n\n` : '';
  return vars + hl + (themeExtraCss[id] ?? '');
}).join('\n\n');

const rootCss = `:root {\n${varsBlock(cssVariables)}\n}\n:root {\n${varsBlock(hlVars('light'))}\n}`;

const SAMPLE = `
  <div class="nav">
    <span class="brand">Citebase</span>
    <span class="crumb">知识库 / 设计规范 / 主题一致性</span>
    <span class="pill">⌘K 搜索</span>
  </div>
  <div class="side">
    <div class="side-item">全部文档</div>
    <div class="side-item on">设计规范</div>
    <div class="side-item">接口手册</div>
  </div>
  <div class="ladder">
    <span class="lad chrome">surface-chrome 顶栏/侧栏</span>
    <span class="lad canvas">surface-canvas 页面底</span>
    <span class="lad panel">surface-panel 卡片</span>
    <span class="lad elevated">surface-elevated 浮层</span>
    <span class="lad field">surface-field 输入面</span>
    <span class="lad primary">surface-primary 主对象</span>
  </div>
  <div class="grid">
    <div class="card">
      <h2>主题一致性</h2>
      <p class="lead">面板、阴影、材质、代码块、状态色与滚动条都取自同一套 token；切换主题时整体跟随。</p>
      <p>正文示例：<code>cv('surface-panel')</code> 决定弹层表面，<code>cv('glass-blur')</code> 决定材质，
      <code>cv('green-bg-subtle')</code> 由纯色 <code>--apple-green</code> 派生。</p>
      <div class="row">
        <button class="btn primary">主按钮</button>
        <button class="btn">次按钮</button>
        <span class="badge ok">成功</span>
        <span class="badge warn">警告</span>
        <span class="badge err">错误</span>
        <span class="badge info">中性</span>
      </div>
      <div class="callout">提示块（callout）：底色与描边同源于主题强调色，不需要每套主题手写 rgba。</div>
    </div>
    <div class="card">
      <h3>数据表</h3>
      <table>
        <thead><tr><th>主题</th><th>契约 token</th><th>门禁</th></tr></thead>
        <tbody>
          <tr><td>light+glass</td><td>${THEME_CONTRACT_KEYS.length}</td><td>通过</td></tr>
          <tr><td>editorial</td><td>${THEME_CONTRACT_KEYS.length}</td><td>通过</td></tr>
          <tr class="sel"><td>warm-paper</td><td>${THEME_CONTRACT_KEYS.length}</td><td>通过</td></tr>
        </tbody>
      </table>
      <blockquote>引用块：层级由主题决定——玻璃靠镜面内高光，纸感靠发丝横线。</blockquote>
       <pre class="code"><span class="k">const</span> <span class="f">theme</span> = <span class="s">'warm-paper'</span>; <span class="c">// 暖纸调色板</span>
<span class="t">applyTheme</span>(theme); <span class="n">// 面板 / 阴影 / 代码块跟随</span></pre>
      <div class="field"><input placeholder="输入框（focus 走主题焦点环）"></div>
    </div>
  </div>
`;

const switcher = THEME_REGISTRY.map(
  (m) => `<button class="sw" data-pick="${m.id}">${m.label}</button>`
).join('\n      ');

const chrome = `
  body { margin:0; font-family:var(--apple-font-text); background:var(--apple-surface-canvas); color:var(--apple-text-primary); }
  .bar { display:flex; align-items:center; gap:8px; padding:10px 16px; position:sticky; top:0; z-index:10;
    background:var(--apple-surface-chrome); border-bottom:1px solid var(--apple-border); }
  .bar b { font-size:14px; margin-right:8px; }
  .sw { padding:5px 12px; font-size:13px; cursor:pointer; border-radius:980px;
    border:1px solid var(--apple-border); background:var(--apple-bg-hover); color:var(--apple-text-secondary); }
  .sw.on { background:var(--apple-blue); border-color:transparent; color:var(--apple-on-accent); }
  .meta { margin-left:auto; font-size:12px; color:var(--apple-text-tertiary); }
  .nav { display:flex; align-items:center; gap:${AppleSpace.md}; padding:${AppleSpace.sm} ${AppleSpace.md};
    background:var(--apple-surface-chrome); backdrop-filter:var(--apple-glass-blur);
    border-bottom:1px solid var(--apple-glass-border-nav); color:var(--apple-text-primary); }
  .brand { font-weight:600; }
  .crumb { color:var(--apple-text-tertiary); font-size:${AppleType.microUI.size}; }
  .pill { margin-left:auto; padding:2px ${AppleSpace.sm}; border-radius:980px; font-size:${AppleType.microUI.size};
    background:var(--apple-bg-hover); border:1px solid var(--apple-border); color:var(--apple-text-secondary); }
  .side { display:flex; gap:6px; padding:8px ${AppleSpace.md} 0; font-size:13px; color:var(--apple-text-secondary); }
  .side-item { padding:4px 10px; border-radius:var(--apple-radius-control); }
  /* 选中项语言与组件一致：同色浅染 + 压暗后的同色文字（ApButton variant="tint"） */
  .side-item.on { background:var(--apple-blue-selection); color:var(--apple-blue-text); font-weight:600; }
  .grid { display:grid; grid-template-columns:1fr 1fr; gap:${AppleSpace.md}; padding:${AppleSpace.md}; }
  /* 表面阶梯：四档角色并排显示，切换主题时应当整体跟随（同档同色才算一致） */
  .ladder { display:grid; grid-template-columns:repeat(auto-fit, minmax(160px, 1fr)); gap:${AppleSpace.md}; padding:${AppleSpace.md} ${AppleSpace.md} 0; }
  .lad { padding:${AppleSpace.md}; border-radius:var(--apple-radius-card); border:1px solid var(--apple-border);
    font-size:${AppleType.microUI.size}; color:var(--apple-text-secondary); }
  .lad.chrome { background:var(--apple-surface-chrome); }
  .lad.canvas { background:var(--apple-surface-canvas); }
  .lad.panel { background:var(--apple-surface-panel); }
  .lad.elevated { background:var(--apple-surface-elevated); box-shadow:var(--apple-shadow-level3); }
  .lad.field { background:var(--apple-surface-field); }
  .lad.primary { background:var(--apple-surface-primary); box-shadow:var(--apple-shadow-primary); }
  .card { background:var(--apple-surface-panel); border:1px solid var(--apple-border);
    border-radius:var(--apple-radius-card); box-shadow:var(--apple-shadow-card); padding:${AppleSpace.md};
    color:var(--apple-text-primary); backdrop-filter:var(--apple-blur-panel); }
  .card h2, .card h3 { font-family:var(--apple-font-display); margin:0 0 ${AppleSpace.sm}; }
  .lead { color:var(--apple-text-secondary); }
  p { line-height:1.65; }
  code { background:var(--apple-code-bg); color:var(--apple-code-fg); padding:1px 5px;
    border-radius:var(--apple-radius-control); font-family:${AppleFont.mono}; font-size:12px; }
  .row { display:flex; align-items:center; gap:${AppleSpace.sm}; flex-wrap:wrap; margin-top:${AppleSpace.md}; }
  .btn { padding:6px 14px; border-radius:var(--apple-radius-control); border:1px solid var(--apple-border);
    background:var(--apple-surface-panel); color:var(--apple-text-primary); cursor:pointer; font-size:13px; }
  .btn.primary { background:var(--apple-blue); border-color:transparent; color:var(--apple-on-accent); }
  .badge { padding:3px 9px; border-radius:980px; font-size:12px; }
  .badge.ok { background:var(--apple-green-bg-subtle); color:var(--apple-green); }
  .badge.warn { background:var(--apple-orange-bg-subtle); color:var(--apple-orange); }
  .badge.err { background:var(--apple-red-bg-subtle); color:var(--apple-red); }
  .badge.info { background:var(--apple-gray-bg-subtle); color:var(--apple-text-secondary); }
  .callout { margin-top:${AppleSpace.md}; padding:${AppleSpace.sm}; font-size:13px;
    background:var(--apple-callout-note-bg); border:1px solid var(--apple-callout-note-border);
    border-radius:var(--apple-radius-control); }
  table { width:100%; border-collapse:collapse; font-size:13px; font-variant-numeric:tabular-nums; }
  th, td { text-align:left; padding:6px 8px; border-bottom:1px solid var(--apple-border-light); }
  th { color:var(--apple-text-secondary); font-weight:600; }
  tr.sel td { background:var(--apple-blue-selection); color:var(--apple-blue-text); }
  blockquote { margin:${AppleSpace.md} 0 0; padding-left:${AppleSpace.md}; border-left:3px solid var(--apple-border);
    color:var(--apple-text-secondary); }
  .code { margin:${AppleSpace.md} 0 0; padding:${AppleSpace.sm}; background:var(--apple-code-bg);
    border-radius:var(--apple-radius-control); color:var(--apple-code-fg); overflow:auto; }
  .field { margin-top:${AppleSpace.md}; }
  .field input { width:100%; padding:8px 10px; font-size:13px; background:var(--apple-surface-panel);
    border:1px solid var(--apple-border); border-radius:var(--apple-radius-control); color:var(--apple-text-primary); }
  .field input:focus { outline:none; box-shadow:var(--apple-blue-focus-ring); border-color:var(--apple-blue); }
  .k { color:var(--hl-keyword); } .s { color:var(--hl-string); } .c { color:var(--hl-comment); }
  .t { color:var(--hl-title); } .f { color:var(--hl-function); } .n { color:var(--hl-name); }
`;

const html = `<!DOCTYPE html>
<html lang="zh-CN" data-theme="light">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Citebase 主题实况预览（由代码生成）</title>
<style>
${rootCss}

${themeCss}

${chrome}
</style>
</head>
<body>
  <div class="bar">
    <b>Citebase 主题实况预览</b>
      ${switcher}
    <span class="meta">${THEME_REGISTRY.length} 套主题 × ${THEME_CONTRACT_KEYS.length} 个契约 token · 由 npm run gen:theme-preview 生成</span>
  </div>
${SAMPLE}
<script>
  const pick = (id) => {
    document.documentElement.setAttribute('data-theme', id);
    document.querySelectorAll('.sw').forEach((b) => b.classList.toggle('on', b.dataset.pick === id));
  };
  document.querySelectorAll('.sw').forEach((b) => b.addEventListener('click', () => pick(b.dataset.pick)));
  pick(new URLSearchParams(location.search).get('theme') || 'light');
</script>
</body>
</html>
`;

fs.mkdirSync(path.dirname(OUT), { recursive: true });
fs.writeFileSync(OUT, html, 'utf8');
console.log(`✓ 已生成 ${path.relative(process.cwd(), OUT)}（${THEME_REGISTRY.length} 套主题，?theme=<id> 可直达）`);
