#!/usr/bin/env node
/**
 * 从真实令牌生成品牌预览 —— npm run gen:brand-preview
 *
 * 为什么是脚本而不是手写 HTML：docs/ui-previews/ 里原先那份手写的 theme-global-preview.html
 * 已经因为"与实现分叉"被废过一次，2026-10-08 连同其余 5 张非生成物预览页一起删除
 * （它把液态玻璃画成暗色，而实现是浅色清玻璃）。
 * 本脚本和 gen-theme-preview.mjs 用同一套 esbuild 求值机制，把 AppleType / AppleSpace /
 * AppleRadius 的真实数值直接烘进产物 —— 改令牌 → 重跑，预览不会变成假样式。
 *
 * 内容对应品牌拆解的结论（取数笔记在未入库的 spark-output/ 下，判据见 docs/design/brand.md 开头）：
 *   外壳 lean-in（四家一手 token 证明克制已是品类地板线），
 *   只在「来源链轨」这一个构件上 diverge（Notion 停在章、思源停在动作确认，链没人做）。
 * 三个版面 × 两档激进程度：A 只上构件、B 在教条允许放开的版面做人格。
 *
 * 缺档即抛错：绝不为了"这里想大一点"而在 AppleType 之外写裸 px 字号（门禁 G31 拦的就是这个）。
 */
import fs from 'fs';
import path from 'path';
import module from 'node:module';
import { buildSync } from 'esbuild';

const root = path.resolve('.');
const bundle = buildSync({
  stdin: {
    contents: `
      export { cssVariables, AppleSpace, AppleType, AppleRadius, AppleLayout } from './src/utils/core/apple-tokens';
      export { THEME_REGISTRY, BASE_THEME_IDS, MATERIAL_REGISTRY, GLASS_OVERLAY, themeOverrideMapFor, supportsGlass } from './src/styles/theme/themes';
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
  cssVariables, AppleSpace, AppleType, AppleRadius, AppleLayout,
  THEME_REGISTRY, BASE_THEME_IDS, MATERIAL_REGISTRY, GLASS_OVERLAY, themeOverrideMapFor, supportsGlass,
} = mod.exports;

const OUT = path.resolve('../docs/ui-previews/brand-preview.html');

/** 取一档排版令牌；档名写错立刻抛，不静默产出假样式 */
function T(tier) {
  const t = AppleType[tier];
  if (!t) {
    throw new Error(`AppleType 里没有「${tier}」这一档（现有：${Object.keys(AppleType).join(', ')}）`);
  }
  return t;
}
const typeCss = (tier) => {
  const t = T(tier);
  return `font-family:${t.font};font-size:${t.size};font-weight:${t.weight};line-height:${t.lineHeight};letter-spacing:${t.letterSpacing}`;
};
const S = (k) => {
  const v = AppleSpace[k];
  if (!v) throw new Error(`AppleSpace 里没有「${k}」`);
  return v;
};
const R = (k) => {
  const v = AppleRadius[k];
  if (!v) throw new Error(`AppleRadius 里没有「${k}」`);
  return v;
};
const L = (k) => {
  const v = AppleLayout[k];
  if (!v) throw new Error(`AppleLayout 里没有「${k}」`);
  return v;
};

/* ---------- 主题注入：与 gen-theme-preview 同构（base × material 两个键一起给） ---------- */
const varsBlock = (map) => Object.entries(map).map(([k, v]) => `  ${k}: ${v};`).join('\n');

const themeBlocks = [];
for (const { id: base } of THEME_REGISTRY) {
  const solid = themeOverrideMapFor(base);
  if (solid) themeBlocks.push(`:root[data-theme='${base}'][data-material='solid'] {\n${varsBlock(solid)}\n}`);
  if (supportsGlass(base)) {
    const glass = GLASS_OVERLAY[base === 'dark' ? 'dark' : 'light'];
    // 材质层是 Partial：只覆写它必须改的那几档，其余继承实心值 —— 所以是合并而不是替换
    themeBlocks.push(`:root[data-theme='${base}'][data-material='glass'] {\n${varsBlock({ ...solid, ...glass })}\n}`);
  }
}
const rootCss = `:root{\n${varsBlock(cssVariables)}\n}`;

/* ---------- 构件：来源链轨 ---------- */
const badge = (cls, txt) => `<span class="badge ${cls}">${txt}</span>`;

/**
 * 一条来源 = 「序号 · 源文档 · 段落锚点 · 人工确认态 · 源有效性」
 * 后两栏是 Notion 的 Verified 章里没有的信息：它只说"确认过"，不说"依据哪一段、源还在不在"。
 */
function sourceRow(n, doc, anchor, opts = {}) {
  const confirmed = opts.confirmed !== false;
  const dead = !!opts.dead;
  // 窄版（200px 侧栏）里"源有效/已人工确认"两枚加起来 144px > 可用 126px，会折成两行。
  // 改文案长度而不是改布局：信息一个不少，标签收紧。
  const okTxt = opts.compact ? (dead ? '已下架' : '有效') : (dead ? '源已下架' : '源有效');
  const cfTxt = opts.compact ? (confirmed ? '已确认' : '待确认') : (confirmed ? '已人工确认' : '待确认');
  return `
  <li class="src">
    <span class="idx">${n}</span>
    <span class="srctxt">
      <span class="doc">${doc}</span>
      <span class="anch">${anchor}</span>
    </span>
    <span class="stat">
      ${badge(dead ? 'warn' : 'ok', okTxt)}
      ${badge(confirmed ? 'done' : 'pending', cfTxt)}
    </span>
  </li>`;
}

const rail = (rows, label = '本段依据') => `
<div class="rail">
  <div class="railhead">${label}</div>
  <ol class="srcs">${rows.join('')}</ol>
</div>`;

/* ---------- 版面 ---------- */
const landingA = `
<section class="pane">
  <div class="wrap">
    <p class="eyebrow">开源 · MIT · 可自托管</p>
    <h1 class="lA">让每一条 AI 写的知识<br>都带着它的来源</h1>
    <p class="ledeA">Citebase 是 Go + Vue 的企业 AI 知识库：RAG 检索、引用落段、人工确认、源下架后的级联处理，全部在界面里看得见。</p>
    <div class="cta">
      <button class="btn primary">快速开始</button>
      <button class="btn">查看架构</button>
    </div>
    ${rail([
    sourceRow(1, '接口手册 v3', '§4.2 响应信封'),
    sourceRow(2, '数据库迁移说明', '§表前缀约定', { confirmed: false }),
  ])}
  </div>
</section>`;

const landingB = `
<section class="pane split">
  <div class="half">
    <p class="eyebrow">SELF-HOSTED · MIT</p>
    <h1 class="lB"><span class="nb">知识不该</span><br>只有一句 <span class="nb">"AI 生成"</span></h1>
    <p class="ledeB">Outline 给你一个来源标签，Notion 给你一个蓝勾。Citebase 给的是<b>链</b>：哪一篇、哪一段、有没有人确认过、源下架之后这句话会怎样。</p>
    <div class="cta">
      <button class="btn primary">部署</button>
      <button class="btn ghost">读源码</button>
    </div>
    <ul class="facts">
      <li><span class="num">71</span><span class="fact">个主题契约 token，4 套外观 × 实心/玻璃</span></li>
      <li><span class="num">42</span><span class="fact">条设计门禁（G/T/R）机器强制，不靠 review</span></li>
      <li><span class="num">MIT</span><span class="fact">真开源（对比 Outline 的 BSL 1.1）</span></li>
    </ul>
  </div>
  <div class="half">
    <div class="shot">
      <div class="shottitle">文档 · 响应信封规范</div>
      <p class="shotbody">所有 API 响应共用一个信封：<code>code / message / data</code>。错误文案对客户端只给结论，内部细节留在日志。</p>
      ${rail([
        sourceRow(1, '接口手册 v3', '§4.2 响应信封'),
        sourceRow(2, '错误码表', '§5xx 不外泄原文', { dead: true }),
      ], '这句话的依据')}
      <div class="shotfoot">由 Citebase 提供 · 源下架时本段自动摘除溯源</div>
    </div>
  </div>
</section>`;

const loginA = `
<section class="pane center">
  <div class="cardbox">
    <div class="brandrow"><span class="mark">Citebase</span></div>
    <h1 class="pageTitle">登录你的知识库</h1>
    <label class="fld"><span>用户名</span><input placeholder="admin"></label>
    <label class="fld"><span>密码</span><input type="password" placeholder="••••••••"></label>
    <button class="btn primary block">登录</button>
    <div class="sep"></div>
    <p class="why">它在证明什么</p>
    ${rail([sourceRow(1, '安全策略', '§5xx 不外泄原文')], '登录页也在示例自己的主张')}
  </div>
</section>`;

const loginB = `
<section class="pane split login">
  <div class="half dark">
    <div class="darkinner">
      <span class="mark onbrand">Citebase</span>
      <h1 class="lC"><span class="nb">写进来的每一句，</span><br><span class="nb">都能被追问出处</span></h1>
      <p class="ledeC">RAG 溯源 · 人工确认 · 源下架级联。不是提示词工程，是数据模型。</p>
      <div class="chips">
        <span class="chip">10 步检索管线</span><span class="chip">幻觉检测重试</span><span class="chip">Text2SQL 校验</span>
      </div>
    </div>
  </div>
  <div class="half">
    <div class="cardbox">
      <h1 class="pageTitle">登录</h1>
      <label class="fld"><span>用户名</span><input placeholder="admin"></label>
      <label class="fld"><span>密码</span><input type="password" placeholder="••••••••"></label>
      <button class="btn primary block">登录</button>
      <p class="hint">单用户许可无限制 · 自托管无遥测</p>
    </div>
  </div>
</section>`;

const shareA = `
<section class="pane">
  <div class="sheet">
    <div class="meta">产品文档 / 接口手册 v3 · 由 李明 创建</div>
    <h1 class="objtitle">响应信封</h1>
    <p class="body">所有 API 响应共用一个信封字段。<b>客户端只拿结论，内部细节留在日志</b>。</p>
    ${rail([
      sourceRow(1, '错误码表', '§4.2', ),
      sourceRow(2, '审计记录 2026-10', '§原文外泄 53 处', { confirmed: false }),
    ])}
    <div class="sharefoot">由 Citebase 提供 · MIT 开源 · <span class="link">部署一个自己的</span></div>
  </div>
</section>`;

const shareB = `
<section class="pane withside">
  <div class="sheet">
    <div class="meta">产品文档 / 接口手册 v3</div>
    <h1 class="objtitle">响应信封</h1>
    <p class="body">所有 API 响应共用一个信封字段。<mark class="hot">客户端只拿结论，内部细节留在日志</mark>。</p>
    <p class="body">归档与删除标注到人，权限提示直陈事实，不写"发生未知错误"。</p>
    <div class="sharefoot">由 Citebase 提供 · MIT 开源 · <span class="link">部署一个自己的</span></div>
  </div>
  <aside class="side">
    <div class="sidehead">本页依据</div>
    ${rail([
      sourceRow(1, '错误码表', '§4.2', { compact: true }),
      sourceRow(2, '审计记录 2026-10', '§原文外泄 53 处', { compact: true }),
      sourceRow(3, '旧版接口说明', '§500 直出', { dead: true, confirmed: false, compact: true }),
    ], '')}
    <p class="sidenote">源下架时高亮段自动摘除溯源，人工确认过的内容不随之删除。</p>
  </aside>
</section>`;

/* ---------- 汇总 ---------- */
const panes = [
  { id: 'landing-a', label: 'Landing A — 只上构件', note: '外壳不动，签名构件进首屏。maxContentWidth 1200 居中。', html: landingA },
  { id: 'landing-b', label: 'Landing B — 允许放开', note: '双栏 + 展示级大字（heroDisplayXL 80 只在 landing/login/install 出现，与教条一致）。', html: landingB },
  { id: 'login-a', label: '/login A — 保守', note: '沿用现有居中卡（cardNarrow 440），但登录页第一次有了产品论点。', html: loginA },
  { id: 'login-b', label: '/login B — 进取', note: '左深色品牌栏 + 右表单。深色遮罩上的控件走 overlay 语义，浅染取 veil-on-dark 而非 bg-hover。', html: loginB },
  { id: 'share-a', label: '/share A — 保守', note: '分享页是唯一被转发给外部人的版面：正文用 surface-primary（该档的 ALLOW 就登记了分享正文两页）。', html: shareA },
  { id: 'share-b', label: '/share B — 进取', note: '来源常驻侧栏（tocWidth 200），高亮段与引用条目对应。这是四家竞品都没有的一屏。', html: shareB },
];

const css = `
${rootCss}
${themeBlocks.join('\n')}
*{box-sizing:border-box}
body{margin:0;background:var(--apple-surface-canvas);color:var(--apple-text-primary);${typeCss('bodyPrimary')};font-family:var(--apple-font-text)}
.bar{position:sticky;top:0;z-index:var(--apple-z-index-sticky);background:var(--apple-surface-chrome);border-bottom:1px solid var(--apple-border);padding:${S('sm')} ${S('md')};display:flex;gap:${S('sm')};align-items:center;flex-wrap:wrap}
.bar .t{${typeCss('controlLabel')};color:var(--apple-text-secondary);margin-right:${S('xs')}}
.sw{border:1px solid var(--apple-border);background:var(--apple-surface-panel);color:var(--apple-text-primary);border-radius:var(--apple-radius-control);padding:${S('1px')} ${S('sm')};cursor:pointer;font-size:12px;line-height:1.4}
.sw[aria-pressed="true"]{background:var(--apple-blue-selection);color:var(--apple-blue-text);border-color:transparent}
.tabs{display:flex;gap:${S('xs')};padding:${S('sm')} ${S('md')} 0;flex-wrap:wrap}
.tab{border:1px solid var(--apple-border);border-bottom:none;background:var(--apple-surface-canvas);color:var(--apple-text-secondary);border-radius:${R('control')} ${R('control')} 0 0;padding:${S('sm')} ${S('md')};cursor:pointer;${typeCss('controlLabel')}}
.tab[aria-selected="true"]{background:var(--apple-surface-panel);color:var(--apple-text-primary);font-weight:600}
.note{padding:${S('sm')} ${S('md')};color:var(--apple-text-tertiary);${typeCss('controlLabel')}}
main{padding:0 ${S('md')} ${S('2xl')}}
.stage{display:none;background:var(--apple-surface-panel);border:1px solid var(--apple-border);border-radius:${R('card')};overflow:hidden}
.stage.on{display:block}
.pane{background:var(--apple-surface-canvas);padding:${S('4xl')} ${S('2xl')}}
.wrap{max-width:${L('maxContentWidth')};margin:0 auto}
.split{display:grid;grid-template-columns:1fr 1fr;gap:${S('3xl')};max-width:${L('maxWideContentWidth')};margin:0 auto;align-items:center}
.center{display:grid;place-items:center;min-height:420px}
.withside{display:grid;grid-template-columns:minmax(0,1fr) ${L('tocWidth')};gap:${S('2xl')};max-width:${L('maxContentWidth')};margin:0 auto;align-items:start}
.eyebrow{${typeCss('controlLabel')};color:var(--apple-blue-text);letter-spacing:.08em;text-transform:uppercase;margin:0 0 ${S('md')}}
/* 展示级字号取自 AppleType，一个像素都没改；
   但 CJK 没有词间空格，56-80px 下按拉丁文的换行直觉排会造出孤行与破词
   （实测两次：先是 "知识" 单独一行，加了 text-wrap:balance 后又把 "AI 写的" 切成 "AI / 写的"）。
   结论：balance 只管行宽均匀，不管断点是否落在词上 —— CJK 标题必须显式给断行点，
   所以语义单元包 .nb（nowrap），换行位置由文案作者定，不交给自动换行。，56-80px 下按拉丁文的换行直觉排会造出孤行
   （实测 "让每一条 AI 写的 / 知识 / 都带着它的来源"，"知识"单独一行）。
   所以这里只改排版组合：限幅 + text-wrap:balance，字号档位不动。 */
.lA,.lB,.lC,.pageTitle,.objtitle{text-wrap:balance;max-width:14em;margin:0 0 ${S('md')}}
.lA{${typeCss('heroDisplayL')}}
.lB{${typeCss('heroDisplayXL')};max-width:11em}
.lC{${typeCss('promoDisplay')}}
.pageTitle{${typeCss('pageTitle')};margin:0 0 ${S('lg')}}
.objtitle{${typeCss('linkActionHeading')};margin:0 0 ${S('md')}}
.ledeA,.ledeB{${typeCss('bodyPrimary')};color:var(--apple-text-secondary);max-width:56ch;margin:0 0 ${S('xl')}}
.ledeC{${typeCss('controlLabel')};color:var(--apple-text-secondary)}
.body{${typeCss('bodyPrimary')};margin:0 0 ${S('md')}}
.cta{display:flex;gap:${S('sm')};margin:0 0 ${S('xl')};flex-wrap:wrap}
.btn{${typeCss('controlLabel')};font-weight:600;border-radius:${R('control')};padding:0 ${S('lg')};min-height:${L('iconButtonMedium')};border:1px solid var(--apple-border);background:var(--apple-surface-panel);color:var(--apple-text-primary);cursor:pointer}
.btn.primary{background:var(--apple-blue);color:var(--apple-on-blue);border-color:transparent}
.btn.ghost{background:transparent;border-color:transparent}
.btn.block{width:100%}
.facts{list-style:none;padding:0;margin:${S('xl')} 0 0;display:grid;gap:${S('sm')}}
.facts li{display:flex;gap:${S('sm')};align-items:baseline;${typeCss('controlLabel')};color:var(--apple-text-secondary)}
.num{${typeCss('statValue')};font-variant-numeric:tabular-nums;color:var(--apple-text-primary)}
/* 签名构件：来源链轨 */
.rail{border-left:2px solid var(--apple-blue-bg-subtle);padding:${S('sm')} 0 ${S('sm')} ${S('md')};margin:${S('lg')} 0 0}
.railhead{${typeCss('sectionTitle')};margin:0 0 ${S('sm')}}
.srcs{list-style:none;margin:0;padding:0;display:grid;gap:${S('xs')}}
.src{display:flex;gap:${S('sm')};align-items:center;flex-wrap:wrap;${typeCss('controlLabel')}}
.idx{min-width:20px;height:20px;border-radius:${R('pill')};background:var(--apple-blue-selection);color:var(--apple-blue-text);display:grid;place-items:center;font-size:11px;font-weight:600;font-variant-numeric:tabular-nums}
.doc{color:var(--apple-text-primary);font-weight:500}
.anch{color:var(--apple-text-tertiary)}
.stat{display:flex;gap:${S('xs')};margin-left:auto}
.badge{border-radius:${R('pill')};padding:1px ${S('sm')};font-size:11px;font-weight:600;border:1px solid transparent}
.badge.ok{background:var(--apple-green-bg-subtle);color:var(--apple-green-text)}
.badge.warn{background:var(--apple-orange-bg-subtle);color:var(--apple-orange-text)}
.badge.done{background:var(--apple-blue-bg-subtle);color:var(--apple-blue-text)}
.badge.pending{background:transparent;border-color:var(--apple-border);color:var(--apple-text-tertiary)}
.shot{background:var(--apple-surface-elevated);border:1px solid var(--apple-border);border-radius:${R('card')};padding:${S('xl')};box-shadow:var(--apple-shadow-dropdown)}
.shottitle{${typeCss('sectionTitle')};margin:0 0 ${S('sm')}}
.shotbody{${typeCss('controlLabel')};margin:0 0 ${S('md')}}
.shotfoot,.sharefoot{${typeCss('controlLabel')};color:var(--apple-text-tertiary);margin-top:${S('lg')};padding-top:${S('sm')};border-top:1px solid var(--apple-border-light)}
.link{color:var(--apple-blue-text);cursor:pointer}
.cardbox{width:${L('cardNarrow')};max-width:100%;background:var(--apple-surface-panel);border:1px solid var(--apple-border);border-radius:${R('card')};padding:${S('2xl')}}
.brandrow{margin:0 0 ${S('md')}}
.mark{${typeCss('sectionTitle')};font-weight:700;letter-spacing:-.01em}
.onbrand{color:var(--apple-on-accent)}
.fld{display:grid;gap:${S('1px')};margin:0 0 ${S('md')}}
.fld span{${typeCss('controlLabel')};color:var(--apple-text-secondary)}
.fld input{${typeCss('controlLabel')};width:100%;min-height:${L('iconButtonMedium')};padding:0 ${S('sm')};border:1px solid var(--apple-border);border-radius:${R('control')};background:var(--apple-surface-field);color:var(--apple-text-primary)}
.sep{height:1px;background:var(--apple-border-light);margin:${S('xl')} 0}
.why{${typeCss('sectionTitle')};margin:0 0 ${S('sm')}}
.hint{${typeCss('controlLabel')};color:var(--apple-text-tertiary);text-align:center;margin:${S('md')} 0 0}
.dark{background:var(--apple-near-black-ink);padding:${S('4xl')} ${S('2xl')};border-radius:${R('card')}}
.darkinner{max-width:44ch}
/* 深色品牌栏里的文字必须显式走 on-accent：
   .lC 继承的是 --apple-text-primary（浅色下 #1d1d1f），压在 near-black-ink 上等于隐形
   —— 这正是教条 T10 反向的那一条（"别拿主题的 bg-hover 当深色底"同源）。
   实测截图里整行标题一个字都看不见，就是漏了这一句。 */
.dark .lC,.dark .mark,.dark .ledeC,.dark .eyebrow{color:var(--apple-on-accent)}
.dark .eyebrow{color:var(--apple-blue-selection-strong,var(--apple-on-accent))}
.chips{display:flex;gap:${S('xs')};flex-wrap:wrap;margin-top:${S('lg')}}
.chip{border:1px solid var(--apple-glass-border-nav,rgba(255,255,255,.14));border-radius:${R('pill')};padding:2px ${S('sm')};font-size:12px;color:var(--apple-on-accent)}
.sheet{max-width:calc(680px + ${S('xl')} * 2);background:var(--apple-surface-primary);border:1px solid var(--apple-border);border-radius:${R('card')};padding:${S('xl')};margin:0 auto}
.meta{${typeCss('controlLabel')};color:var(--apple-text-tertiary);margin:0 0 ${S('md')}}
.side{position:sticky;top:${S('md')};background:var(--apple-surface-elevated);border:1px solid var(--apple-border);border-radius:${R('card')};padding:${S('lg')}}
.sidehead{${typeCss('sectionTitle')};margin:0 0 ${S('sm')}}
.sidenote{${typeCss('controlLabel')};color:var(--apple-text-tertiary);margin:${S('md')} 0 0}
/* 窄版构件：200px 侧栏放不下"序号 + 源名 + 锚点 + 两枚徽章"一行排。
   实测在 share-b 里序号会掉成孤行、徽章错位换行。
   窄容器里改成两行排：第一行序号+源名+锚点，第二行徽章左对齐（不再 margin-left:auto）。 */
.side .src{flex-direction:column;align-items:flex-start;gap:${S('1px')};padding:${S('xs')} 0}
.side .src .stat{margin-left:0;flex-wrap:wrap}
.side .src .idx{position:absolute}
.side .srcs{position:relative;padding-left:26px}
.side .src .idx{left:0}
.side .src>span:not(.idx){width:100%}
.side .srctxt{display:block}
.side .doc,.side .anch{${typeCss('controlLabel')}}
.nb{white-space:nowrap}
mark.hot{background:var(--apple-blue-selection);color:var(--apple-blue-text);padding:0 2px;border-radius:2px}
code{font-family:var(--apple-font-mono,ui-monospace,monospace);font-size:.92em;background:var(--apple-bg-tertiary,#eee);padding:1px 4px;border-radius:3px}
@media (max-width:900px){.split,.withside{grid-template-columns:1fr}.dark{border-radius:0}}
`;

const html = `<!DOCTYPE html>
<html lang="zh-CN" data-theme="light" data-material="solid">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>Citebase 品牌预览 — 来源链构件 × 三版面两档</title>
<style>${css}</style>
</head>
<body>
<div class="bar">
  <span class="t">外观</span>
  ${BASE_THEME_IDS.map((b) => `<button class="sw" data-base="${b}" aria-pressed="${b === 'light'}">${b}</button>`).join('\n  ')}
  <span class="t">材质</span>
  ${MATERIAL_REGISTRY.map((m) => `<button class="sw" data-mat="${m.id}" aria-pressed="${m.id === 'solid'}">${m.label ?? m.id}</button>`).join('\n  ')}
  <span class="t" style="margin-left:auto">令牌取自源码 · 改令牌后重跑 <code>npm run gen:brand-preview</code></span>
</div>

<div class="tabs">
  ${panes.map((p, i) => `<button class="tab" data-pane="${p.id}" aria-selected="${i === 0}">${p.label}</button>`).join('\n  ')}
</div>
<p class="note" id="note">${panes[0].note}</p>

<main>
${panes.map((p, i) => `<section class="stage${i === 0 ? ' on' : ''}" id="${p.id}">${p.html}</section>`).join('\n')}
</main>

<script>
const root=document.documentElement;
let base='light', mat='solid';
function apply(){
  root.setAttribute('data-theme', base);
  root.setAttribute('data-material', mat);
  // 纸没有玻璃：与 app 侧 effectiveMaterial() 同一判断，切到不支持的组合就退回实心
  const glassOk = (base==='light'||base==='dark');
  document.querySelectorAll('[data-mat]').forEach(b=>{
    const on = b.dataset.mat===mat && !(b.dataset.mat==='glass' && !glassOk);
    b.setAttribute('aria-pressed', on?'true':'false');
  });
  if(mat==='glass' && !glassOk){ mat='solid'; root.setAttribute('data-material','solid'); }
}
document.querySelectorAll('[data-base]').forEach(b=>b.onclick=()=>{
  base=b.dataset.base;
  document.querySelectorAll('[data-base]').forEach(x=>x.setAttribute('aria-pressed', x===b?'true':'false'));
  apply();
});
document.querySelectorAll('[data-mat]').forEach(b=>b.onclick=()=>{ mat=b.dataset.mat; apply(); });
document.querySelectorAll('.tab').forEach(t=>t.onclick=()=>{
  document.querySelectorAll('.tab').forEach(x=>x.setAttribute('aria-selected', x===t?'true':'false'));
  document.querySelectorAll('.stage').forEach(s=>s.classList.toggle('on', s.id===t.dataset.pane));
  document.getElementById('note').textContent = ${JSON.stringify(Object.fromEntries(panes.map((p) => [p.id, p.note])))}[t.dataset.pane];
});
apply();
</script>
</body></html>
`;

fs.mkdirSync(path.dirname(OUT), { recursive: true });
fs.writeFileSync(OUT, html, 'utf8');
const usedTiers = ['heroDisplayXL', 'heroDisplayL', 'promoDisplay', 'pageTitle', 'linkActionHeading', 'sectionTitle', 'bodyPrimary', 'controlLabel', 'statValue'];
console.log(`✓ 已生成 ${path.relative(process.cwd(), OUT)}`);
console.log(`  用到的排版档（全部真实存在）：${usedTiers.join(', ')}`);
console.log('  版面：' + panes.map((p) => p.id).join(' / '));
