// 视觉签名一致性巡检：把全站**每一个可见元素**按组件身份（data-ap）分组，
// 比较它算出来的视觉外观（描边粗细/描边色/底色/文字色/字号/字重/内边距/圆角/有无阴影）。
// 静态门禁拦的是"源码写了什么令牌"，跨页天阶工装量的是"标题字号一不一致"，
// 而这台管的是人眼真正看到的：**同一个组件在 A 页和 B 页是不是长成一个样**。
//
// 判据是"离群值"而不是"多值即违例"：同一类元素本来会有合法的变体
// （primary / ghost / 选中态 / 错误态…），它们各自都是一大簇；
// 真正的漂移是那种**只出现一两次**的写法 —— 一张卡描边离群、一个标签蹭错字重。
// 所以报的是：某类某属性上，出现次数 ≤ OUTLIER_MAX 且占比 < OUTLIER_PCT 的值。
//
// **V1 是登记表，不是门禁**（这是全站跑完 11455 个元素之后的结论）：
// 稀有 ≠ 错。60 条离群里绝大多数是"语义本来就只出现一两次"——选中态的 tint、
// 品牌图标的底色、错误态的红、压在实心色上的白字。把它当门禁，等于逼着
// 全站把所有语义色磨成同一个颜色。这台的价值是**照着名单人工过一遍**：
// 2026-10-01 那轮从里面挑出的真漂移是 WorkflowEditor 两个标签手写 `fontWeight: '500'`
// （既不在字重天阶上，又跟它借用的字号档不配对），已改成整档令牌。
//
// 用法（前端 dev :3000、后端 :3001 需在跑；口令只走环境变量）：
//   SNAP_USER=… SNAP_PASS=… node scripts/check-visual-signature.mjs [--only=<路由名子串>] [--max=16]
//   node scripts/check-visual-signature.mjs --selftest   # 反向验证：注入两种离群值都必须抓到
import { chromium } from 'playwright';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { findChromium } from './lib/chromium-path.mjs';
import { pickTargets } from './lib/route-list.mjs';
import { loginForm } from './lib/page-state.mjs';

const BASE = process.env.SNAP_BASE || 'http://localhost:3000';
const USER = process.env.SNAP_USER;
const PASS = process.env.SNAP_PASS;
if (!USER || !PASS) {
  console.error('需要 SNAP_USER / SNAP_PASS（口令只走环境变量，不写进仓库）');
  process.exit(2);
}

const here = path.dirname(fileURLToPath(import.meta.url));
const argv = process.argv.slice(2);
const argOf = (k) => (argv.find((a) => a.startsWith(`--${k}=`)) || '').split('=')[1] || '';
const ONLY = argOf('only');
const MAX = Number(argOf('max') || 16);
const TARGETS = await pickTargets(ONLY, here);
const OUTLIER_MAX = Number(process.env.OUTLIER_MAX || 2);
const OUTLIER_PCT = Number(process.env.OUTLIER_PCT || 10);

// 参与比对的视觉属性（都是"看一眼就能看出不一样"的那些）
const PROPS = ['borderTopWidth', 'borderTopColor', 'backgroundColor', 'color', 'fontSize',
  'fontWeight', 'paddingTop', 'paddingLeft', 'borderTopLeftRadius', 'boxShadow'];

const COLLECT = (cfg) => {
  const out = {};
  const rect = (el) => el.getBoundingClientRect();
  // SVG 内部节点不比：lucide 图标没有 font-size/font-weight 概念，继承值会冒充成
  // "某个按钮的 fontWeight 离群"；第三方编辑器内部（Monaco 的查找框、gutter）同理。
  const skip = (el) => {
    // HTML 文档里的 SVG 元素 tagName 是**小写**（'path' 而不是 'PATH'），
    // 上一版用大写正则匹配，豁免静默失效还照样报出 5 条 SVG 离群。
    if (el.tagName && /^(svg|path|circle|ellipse|rect|polyline|line|polygon|g|text)$/.test(el.tagName)) return true;
    return !!el.closest('.monaco-editor, .cm-editor, [data-mode-id], [class*="devtools"], .monaco-findWidget');
  };
  for (const el of document.querySelectorAll('body [data-ap] *, body [data-ap]')) {
    if (skip(el)) continue;
    const r = rect(el);
    if (r.width < 3 || r.height < 3) continue;
    if (r.bottom < 0 || r.top > innerHeight) continue;
    const host = el.closest('[data-ap]');
    if (!host) continue;
    // 组身份 = 组件类 + 它在组件里的角色标签。只按 data-ap 分组会把卡片里的标题和
    // 卡片本身混成一桶；只按标签分组又会让每张卡的 h2 都散开。
    const cls = (host.getAttribute('data-ap') || '?') + '/' + el.tagName.toLowerCase();
    const cs = getComputedStyle(el);
    const bucket = (out[cls] = out[cls] || { n: 0, props: {} });
    bucket.n++;
    for (const p of cfg.props) {
      let v = cs[p];
      if (p === 'boxShadow') v = v === 'none' ? 'none' : '有阴影';
      if (p === 'borderTopColor' && cs.borderTopWidth === '0px') continue;
      if (p === 'paddingTop' && v === '0px' && cs.paddingLeft === '0px') continue;
      const pb = (bucket.props[p] = bucket.props[p] || { values: {}, pages: {} });
      pb.values[v] = (pb.values[v] || 0) + 1;
      pb.pages[v] = pb.pages[v] || [];
      // 违例必须带得出处，否则报"某类某值离群"没人能照着修
      if (pb.pages[v].length < 4 && !pb.pages[v].includes(cfg.page)) pb.pages[v].push(cfg.page);
    }
  }
  return out;
};

const merge = (a, b) => {
  for (const [cls, bucket] of Object.entries(b)) {
    a[cls] = a[cls] || { n: 0, props: {} };
    a[cls].n += bucket.n;
    for (const [p, d] of Object.entries(bucket.props)) {
      const t = (a[cls].props[p] = a[cls].props[p] || { values: {}, pages: {} });
      for (const [v, n] of Object.entries(d.values)) {
        t.values[v] = (t.values[v] || 0) + n;
        t.pages[v] = t.pages[v] || [];
        for (const pg of d.pages[v] || []) if (!t.pages[v].includes(pg)) t.pages[v].push(pg);
      }
    }
  }
  return a;
};

const run = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  page.on('dialog', (d) => (d.type() === 'beforeunload' ? d.accept() : d.dismiss()).catch(() => {}));
  await loginForm(page, BASE, USER, PASS);

  const acc = {};
  let pages = 0;
  let elements = 0;
  const failed = [];
  for (const r of TARGETS) {
    try {
      await page.goto(`${BASE}${r.path}`, { waitUntil: 'load', timeout: 30000 });
      await page.waitForTimeout(Number(process.env.SNAP_GAP_MS || 900));
      await page.mouse.move(1, 1);
      await page.waitForTimeout(120);
      const got = await page.evaluate(COLLECT, { props: PROPS, page: r.name });
      merge(acc, got);
      pages++;
      elements += Object.values(got).reduce((s, b) => s + b.n, 0);
    } catch (e) {
      failed.push(`${r.name}: ${String(e).slice(0, 50)}`);
    }
  }
  await browser.close();

  console.log(`量测页数 = ${pages} / ${TARGETS.length}，量到元素 = ${elements}，分组 = ${Object.keys(acc).length} 类`);
  console.log(`离群判据：某类某属性上，出现 ≤ ${OUTLIER_MAX} 次且占比 < ${OUTLIER_PCT}% 的值\n`);

  const findings = [];
  for (const [cls, bucket] of Object.entries(acc)) {
    for (const [p, d] of Object.entries(bucket.props)) {
      const total = Object.values(d.values).reduce((s, n) => s + n, 0);
      for (const [v, n] of Object.entries(d.values)) {
        if (n <= OUTLIER_MAX && (n / total) * 100 < OUTLIER_PCT) {
          findings.push({ cls, prop: p, value: v, n, total, pages: (d.pages[v] || []).slice(0, 3) });
        }
      }
    }
  }
  findings.sort((a, b) => (b.total - a.total) || a.cls.localeCompare(b.cls));
  console.log(`== V1 视觉签名离群（登记表，不当门禁）：${findings.length} 条 ==`);
  for (const f of findings.slice(0, MAX)) {
    console.log(`   ${f.cls}  ${f.prop} = ${String(f.value).slice(0, 44)}  —— ${f.n}/${f.total}  页:${(f.pages || []).join(',')}  `);
  }
  if (findings.length > MAX) console.log(`   … 另有 ${findings.length - MAX} 条（全量见 web/visual-signature.json）`);

  if (failed.length || pages === 0 || elements === 0) {
    console.log(`\n✗ 有页面没量到（失败 ${failed.length}）—— 上面的"0 条"不作数：${failed.slice(0, 3).join(' | ')}`);
    process.exitCode = 1;
  }
  fs.writeFileSync(path.join(here, '..', 'visual-signature.json'), JSON.stringify({ acc, findings }, null, 1));
  console.log(`\n明细已写入 web/visual-signature.json`);
};

// 反向验证：注入"一个离群元素"和"一簇合法变体"，前者必须被抓、后者必须不被抓。
// 只验前者会漏掉判据过严的情况 —— 那样满屏假阳性，也会被当成"全站漂移"交出去。
const PROBE = () => {
  const host = document.querySelector('main [data-ap=card]') || document.querySelector('main') || document.body;
  // 探针的离群维度用**描边色**而不是描边粗细：Chrome 会把 border-width 归到整设备像素
  // （1.5px 在 DPR 1.25 下算出来就是 1px），拿粗细做判据在有的机器上根本量不出差别。
  const mk = (c) => {
    const box = document.createElement('div');
    box.setAttribute('data-ap', 'zzsig');
    box.style.cssText = `width:120px;height:12px;border-top:1px solid ${c}`;
    host.appendChild(box);
  };
  mk('rgb(250, 0, 0)');
  for (let i = 0; i < 40; i++) mk('rgb(0, 250, 0)');
  return true;
};

const selftest = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  await loginForm(page, BASE, USER, PASS);
  await page.goto(`${BASE}/dashboard`, { waitUntil: 'load', timeout: 30000 });
  await page.waitForTimeout(1500);
  await page.evaluate(PROBE);
  const got = await page.evaluate(COLLECT, { props: PROPS, page: 'selftest' });
  await browser.close();
  const b = got['zzsig/div'];  // 组身份是拼起来的字符串键，不是嵌套对象
  const bw = b?.props?.borderTopColor?.values || {};
  const total = Object.values(bw).reduce((s, n) => s + n, 0);
  const pct = (v) => (bw[v] ? (bw[v] / total) * 100 : 100);
  const checks = [
    [`离群描边色被采到（${bw['rgb(250, 0, 0)'] || 0}/${total}）`, (bw['rgb(250, 0, 0)'] || 0) > 0],
    [`离群值落在判据内（占比 ${pct('rgb(250, 0, 0)').toFixed(1)}% < ${OUTLIER_PCT}%）`, (bw['rgb(250, 0, 0)'] || 0) <= OUTLIER_MAX && pct('rgb(250, 0, 0)') < OUTLIER_PCT],
    [`成簇的合法值（${bw['rgb(0, 250, 0)'] || 0}/${total}）不被报成漂移`, (bw['rgb(0, 250, 0)'] || 0) > OUTLIER_MAX || pct('rgb(0, 250, 0)') >= OUTLIER_PCT],
  ];
  let bad = 0;
  for (const [k, ok] of checks) { if (!ok) bad++; console.log(`${ok ? '✓' : '✗'} ${k}`); }
  console.log(bad ? `✗ ${bad} 条判据不成立——离群口径写错了` : '离群判据两头都成立：抓得到孤例，也不误伤合法变体簇');
  process.exitCode = bad ? 1 : 0;
};

if (argv.includes('--selftest')) selftest(); else run();
