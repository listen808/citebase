#!/usr/bin/env node
// 跨页一致性量测：在 1440×900 下逐页取真实渲染值，再跨页比对。
//
// 为什么需要它：五道静态门禁查的是"源码里有没有用令牌"，visual-snapshot 查的是
// "单页内部有没有溢出/离档/不等高"。但一致性是**跨页**属性——同一档版心的两页
// 左边界差了 24px、两个页标题一个是 21px 一个是 24px、同类空态一页画钥匙一页画
// 文件夹，这些在单页视角里全都"正常"。历史上"空间卡比文档卡还小"就是这么漏过去的。
//
// 用法（前后端需在跑：:3000 与 :3001）：
//   SNAP_USER=xxx SNAP_PASS=yyy node web/scripts/check-ui-consistency.mjs [--routes=N]
import { chromium } from 'playwright';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { findChromium } from './lib/chromium-path.mjs';
import { pickTargets } from './lib/route-list.mjs';
import { installErroredProbe, loginForm, retryCrashedPages} from './lib/page-state.mjs';

const BASE = process.env.SNAP_BASE || 'http://localhost:3000';
const USER = process.env.SNAP_USER;
const PASS = process.env.SNAP_PASS;
if (!USER || !PASS) {
  console.error('需要 SNAP_USER / SNAP_PASS（口令只走环境变量，不写进仓库）');
  process.exit(2);
}

// 路由清单复用 visual-snapshot.mjs（经 lib/route-list.mjs），避免两处各维护一份而慢慢长不一样
const here = path.dirname(fileURLToPath(import.meta.url));
const ROUTES = await pickTargets('', here);
const LIMIT = Number((process.argv.find((a) => a.startsWith('--routes=')) || '').split('=')[1] || 0);
const TARGETS = LIMIT > 0 ? ROUTES.slice(0, LIMIT) : ROUTES;

// 采集：只取"这个页面上真实渲染出来的值"，不读源码
const COLLECT = () => {
  const px = (v) => Math.round(parseFloat(v) * 10) / 10;
  const vis = (el) => {
    const r = el.getBoundingClientRect();
    return r.width > 2 && r.height > 2;
  };
  // 从 el 逐级向上到 stop，累加"与标题同一行的前置控件"占掉的横向空间。
  // 只认 flex 行内、纵向与标题重叠的前置兄弟：面包屑也是标题容器的兄弟节点，
  // 但它在上一行，按兄弟减掉就会量出 -1144px 这种荒唐缩进。
  const leadInset = (el, stop) => {
    const line = el.getBoundingClientRect();
    let lead = 0;
    for (let cur = el; cur && cur !== stop; cur = cur.parentElement) {
      const parent = cur.parentElement;
      if (!parent) break;
      const pcs = getComputedStyle(parent);
      if (!/flex/.test(pcs.display) || !/^row/.test(pcs.flexDirection)) continue;
      const gap = parseFloat(pcs.columnGap) || 0;
      for (let sib = cur.previousElementSibling; sib; sib = sib.previousElementSibling) {
        const r = sib.getBoundingClientRect();
        if (r.width <= 2 || r.bottom <= line.top || r.top >= line.bottom) continue;
        lead += r.width + gap;
        break;
      }
    }
    return Math.round(lead);
  };
  const out = { h1: [], objTitle: [], section: [], stat: [], buttons: {}, buttonWho: {}, empties: [], alerts: [], crumb: false, footer: false, scrollLeft: null, errored: false };

  // 页头只认锚点 data-ap-page-title（ApPageContainer 页头槽 / PageHeader 覆盖槽都带）。
  // 原先取"第一个 h1"，于是 /space/* 那种**对象自己的标题**（空间名、文档标题，住在内容卡里，
  // 缩进由居中阅读栏决定）会被当页头量，只能再写一份页名白名单去排除它。
  // 现在两拨各量各的：页头比 size+weight+inset，对象标题只比 size+weight。
  const h1 = document.querySelector('[data-ap-page-title]');
  for (const el of document.querySelectorAll('[data-ap-object-title]')) {
    if (!vis(el)) continue;
    const cs = getComputedStyle(el);
    out.objTitle.push({ text: el.textContent.trim().slice(0, 18), size: px(cs.fontSize), weight: cs.fontWeight });
  }
  if (h1 && vis(h1)) {
    const cs = getComputedStyle(h1);
    const sc = document.querySelector('[data-ap-scroll]');
    const scLeft = sc ? Math.round(sc.getBoundingClientRect().left) : 0;
    // 找声明版心的那一层（ApPageContainer 的内容盒）：绝对 left 会被侧栏宽度、
    // /space/* 隐藏侧栏等规则干扰，只有"相对内容区的左缩进"才是可跨页比的量
    let box = h1.parentElement; let maxW = 'none'; let padL = null;
    while (box && box !== document.body) {
      const bcs = getComputedStyle(box);
      if (bcs.maxWidth && bcs.maxWidth !== 'none') { maxW = bcs.maxWidth; const pl = parseFloat(bcs.paddingLeft); if (!Number.isNaN(pl)) padL = Math.round(pl); break; }
      box = box.parentElement;
    }
    out.h1.push({
      text: h1.textContent.trim().slice(0, 20), size: px(cs.fontSize), weight: cs.fontWeight,
      left: Math.round(h1.getBoundingClientRect().left),
      // 相对内容滚动区的左缩进：侧栏宽度、/space/* 隐藏侧栏等规则都被它抵消，
      // 这才是可跨页直接比的量。标题前的返回按钮/图标是合法控件（实测让 4 个页
      // 面看起来"多缩进 36px"），逐级扣掉它们占的位，剩下的才是真的版心偏差。
      inset: Math.round(h1.getBoundingClientRect().left) - (scLeft || 0) - leadInset(h1, box),
      maxW, padL,
    });
  }
  for (const el of document.querySelectorAll('main h2, main h3, [data-ap-scroll] h2, [data-ap-scroll] h3')) {
    if (!vis(el)) continue;
    const cs = getComputedStyle(el);
    out.section.push({ size: px(cs.fontSize), weight: cs.fontWeight, text: el.textContent.trim().slice(0, 18) });
  }
  // 统计数值：ApStatCard 的值。判据是"独立成块、纯数字、字号不小"
  for (const el of document.querySelectorAll('main *, [data-ap-scroll] *')) {
    if (el.children.length) continue;
    const t = (el.textContent || '').trim();
    if (!/^[\d.,]+\s*[%％]?$|^\d[\d.,]*\s*(B|KB|MB|GB|ms|s)$/.test(t)) continue;
    if (!vis(el)) continue;
    const cs = getComputedStyle(el);
    if (px(cs.fontSize) < 16) continue;
    out.stat.push({ size: px(cs.fontSize), weight: cs.fontWeight, fvn: cs.fontVariantNumeric, text: t.slice(0, 12) });
  }
  // 同类控件的渲染高度：按 variant 主色区分不了，这里按 (字号, 上下内边距) 归组
  for (const b of document.querySelectorAll('main button, [data-ap-scroll] button')) {
    if (!vis(b)) continue;
    const cs = getComputedStyle(b);
    // 悬停缩放中的按钮不参与高度普查：ApButton 悬停是 scale(1.02)，32px 会被量成 32.64 →
    // 报出"1334 个 32px 里混进一个 33px"的假离群。
    // 注意**不能**按"transform 有值"过滤：ApButton 静置就写着 scale(1)，那样会把 1382 颗
    // 按钮全滤掉，普查只剩几十颗还看起来"全绿"（第一版就是这么错的）。只滤掉非 1 的缩放。
    const tf = /matrix\(([-\d.]+)/.exec(cs.transform || '');
    if (tf && Math.abs(Number(tf[1]) - 1) > 0.001) continue;
    const h = Math.round(b.getBoundingClientRect().height);
    const key = `${px(cs.fontSize)}/${cs.fontWeight}`;
    out.buttons[key] = out.buttons[key] || {};
    out.buttons[key][h] = (out.buttons[key][h] || 0) + 1;
    // 记下"是哪一颗"：只有直方图的话，"33px×1"这种离群要人再手工去页面上找一遍，
    // 而找的过程比修还长（i18n 巡检也是同一课：红了必须能直接行动）。
    const wkey = `${key}@${h}`;
    const who = (b.getAttribute('aria-label') || b.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 20) || b.className.split(/\s+/).filter(Boolean)[0] || '(无名)';
    out.buttonWho[wkey] = out.buttonWho[wkey] || [];
    if (out.buttonWho[wkey].length < 3 && !out.buttonWho[wkey].includes(who)) out.buttonWho[wkey].push(who);
  }
  // 空态与"就地报错"共用 role，混在一格里会把「仓库地址必填」这种校验错误
  // 当成空态统计（图标直方图因此凭空多出一档 x-icon）。按 role 分开采。
  for (const e of document.querySelectorAll('[role=status]')) {
    if (!vis(e)) continue;
    const svg = e.querySelector('svg');
    const cls = svg ? [...svg.classList].find((c) => c.startsWith('lucide-')) : '';
    out.empties.push({ icon: (cls || '').replace('lucide-', ''), title: e.textContent.trim().slice(0, 24) });
  }
  for (const e of document.querySelectorAll('[role=alert]')) {
    if (!vis(e)) continue;
    const svg = e.querySelector('svg');
    const cls = svg ? [...svg.classList].find((c) => c.startsWith('lucide-')) : '';
    out.alerts.push({ icon: (cls || '').replace('lucide-', ''), title: e.textContent.trim().slice(0, 24) });
  }
  // [data-ap-breadcrumb] 是条死选择器（真锚点是 data-ap="breadcrumb"），合并实现后换成能命中的那条。
  // 顺序要紧：锚点优先，`nav[aria-label*="面包屑"]` 与 `nav ol` 只作历史兜底。
  out.crumb = !!document.querySelector('[data-ap="breadcrumb"], nav[aria-label*="面包屑"], nav ol');
  out.footer = !!document.querySelector('footer');
  // 错误兜底页自带 role=alert 与 h2，会被下面的空态/区块标题统计当成页面内容读进去；
  // 单独标出来，避免把"这页崩了"误读成"这页排版不一致"。
  // 判据在 scripts/lib/page-state.mjs：SPA + KeepAlive 下兜底节点会留在 DOM 里但不可见，
  // 所以必须判可见，否则"上一跳崩过"会算到当前页头上。
  out.errored = window.__apErrored ? window.__apErrored() : false;
  if (out.errored) out.errorText = (document.querySelector('[role=alert]')?.textContent || '').trim().slice(0, 200);
  const sc = document.querySelector('[data-ap-scroll]');
  if (sc) out.scrollLeft = Math.round(sc.getBoundingClientRect().left);
  return out;
};

const mode = (arr) => {
  const c = {};
  for (const v of arr) c[v] = (c[v] || 0) + 1;
  return Object.entries(c).sort((a, b) => b[1] - a[1]);
};

const run = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  await installErroredProbe(page);
  await loginForm(page, BASE, USER, PASS);
  if (page.url().includes('/login')) {
    console.error('登录失败，无法量测受保护页');
    await browser.close();
    process.exit(2);
  }

  const rows = [];
  for (const r of TARGETS) {
    try {
      await page.goto(`${BASE}${r.path}`, { waitUntil: 'load', timeout: 30000 });
      await page.waitForTimeout(Number(process.env.SNAP_GAP_MS || 1500));
      const m = await page.evaluate(COLLECT);
      rows.push({ name: r.name, path: r.path, ...m });
    } catch (e) {
      rows.push({ name: r.name, failed: String(e).slice(0, 90) });
    }
  }

  // 被判进兜底页的先自动复跑一次（多半是巡检自己打满限流），复跑仍崩才让工具变红
  await retryCrashedPages(page, BASE, rows, async (p, row) => Object.assign(row, await p.evaluate(COLLECT)));
  await browser.close();

  const errored = rows.filter((x) => x.errored);
  const ok = rows.filter((x) => !x.failed && !x.errored);
  console.log(`量测页数 = ${ok.length} / ${rows.length}${rows.filter((x) => x.failed).length ? '（导航失败：' + rows.filter((x) => x.failed).map((x) => x.name).join(', ') + '）' : ''}`);
  if (errored.length) {
    console.log(`✗ 渲染兜底页（ErrorBoundary 接住了异常，下面的统计不含这几页）：${errored.map((x) => x.name).join(', ')}`);
    for (const x of errored) console.log(`      ${x.name}: ${JSON.stringify(x.errorText || '')}`);
    // 全量硬刷新 70+ 页 ≈ 每分钟上千个请求，先撞的是通用限流（300 次/分钟/IP）。
    // 客户端把所有 429 都改写成"操作过于频繁"，所以这句文案不代表认证入口被限流。
    // 处理办法是单页复跑确认，不是把这页当缺陷修掉。
    console.log('      （多为巡检自身打满通用限流 300 次/分钟/IP —— 单页复跑确认再下结论）');
  }
  console.log(`视口 = 1440×900\n`);

  // 1. 页标题字号
  const h1sizes = mode(ok.flatMap((x) => x.h1.map((h) => h.size)));
  console.log('== 页标题 (h1) 字号分布 ==');
  for (const [v, n] of h1sizes) console.log(`   ${v}px × ${n}`);
  if (h1sizes.length > 1) {
    for (const [v] of h1sizes.slice(1)) {
      console.log(`   ! 偏离主值 ${h1sizes[0][0]}px 的页：`);
      for (const x of ok) for (const h of x.h1) if (h.size === +v) console.log(`      ${x.name}  "${h.text}" = ${h.size}px`);
    }
  }

  // 2. 正文左缩进：按声明的版心档分组比较，跨档比是伪结论（1200 与 1600 居中后本就不等）
  console.log('\n== 正文左缩进（相对内容区）按版心档分组 ==');
  const byMax = {};
  for (const x of ok) for (const h of x.h1) {
    const k = h.maxW || 'none'; (byMax[k] = byMax[k] || []).push({ name: x.name, inset: h.inset });
  }
  for (const [k, list] of Object.entries(byMax).sort((a, b) => b[1].length - a[1].length)) {
    const dist = mode(list.map((l) => l.inset));
    console.log(`   版心 ${k}  (${list.length} 页): ` + dist.map(([v, n]) => `inset=${v}px×${n}`).join('  '));
    if (dist.length > 1) {
      const main = Number(dist[0][0]);
      for (const l of list.filter((x) => x.inset !== main)) console.log(`      ! 偏离 ${main}px：${l.name} inset=${l.inset}`);
    }
  }
  const objSizes = mode(ok.flatMap((x) => (x.objTitle || []).map((h) => h.size)));
  if (objSizes.length) {
    console.log(`   对象标题（data-ap-object-title，${ok.flatMap((x) => x.objTitle || []).length} 枚）: `
      + objSizes.map(([v, n]) => `${v}px×${n}`).join('  '));
    if (objSizes.length > 1) console.log('      ! 对象标题之间也不该各写一档');
  }

  // 3. 区块标题
  console.log('\n== 区块标题 (h2/h3) 字号/字重分布 ==');
  const secByRole = {};
  for (const x of ok) for (const s of x.section) {
    const k = `${s.size}px/${s.weight}`;
    (secByRole[k] = secByRole[k] || []).push(`${x.name}:${s.text}`);
  }
  for (const [v, list] of Object.entries(secByRole).sort((a, b) => b[1].length - a[1].length)) {
    console.log(`   ${v} × ${list.length}`);
    // 少数派必须点名到页与标题：h2 有两种身份（页面区块标题 19px / 文档正文标题 24px），
    // 只看直方图会把"SSO 文档页的正文标题"读成一处排版不一致 —— 那是两种角色，不是一处漂移。
    if (list.length <= 2) console.log(`      例: ${list.slice(0, 2).join('  ')}`);
  }

  // 4. 统计数值：字号一档 + 必须 tabular-nums
  console.log('\n== 统计数值字号/数字等宽 ==');
  for (const [v, n] of mode(ok.flatMap((x) => x.stat.map((s) => `${s.size}px/${s.weight}/fvn=${s.fvn}`)))) console.log(`   ${v} × ${n}`);
  const noTab = ok.filter((x) => x.stat.some((s) => !/tabular-nums/.test(s.fvn)));
  if (noTab.length) { console.log('   ! 数值未用 tabular-nums（数位会跳动、上下行对不齐）：'); for (const x of noTab.slice(0, 8)) console.log(`      ${x.name}  ${JSON.stringify(x.stat.filter((s) => !/tabular-nums/.test(s.fvn)).map((s) => s.text))}`); }

  // 5. 同类控件高度
  console.log('\n== 同字号/字重下按钮渲染高度（同一档应当只有一个高度）==');
  const perKey = {};
  const whoKey = {};
  for (const x of ok) {
    for (const [k, hs] of Object.entries(x.buttons)) {
      perKey[k] = perKey[k] || {};
      for (const [h, n] of Object.entries(hs)) perKey[k][h] = (perKey[k][h] || 0) + n;
    }
    for (const [wk, list] of Object.entries(x.buttonWho || {})) {
      whoKey[wk] = whoKey[wk] || new Set();
      for (const w of list) if (whoKey[wk].size < 4) whoKey[wk].add(`${x.name}:${w}`);
    }
  }
  for (const [k, hs] of Object.entries(perKey).sort((a, b) => Object.values(b[1]).reduce((s, v) => s + v, 0) - Object.values(a[1]).reduce((s, v) => s + v, 0)).slice(0, 8)) {
    const list = Object.entries(hs).sort((a, b) => b[1] - a[1]);
    const total = list.reduce((s, [, n]) => s + n, 0);
    console.log(`   ${k}: ` + list.map(([h, n]) => `${h}px×${n}`).join('  ') + (list.length > 2 ? '   ! 同档多高度' : ''));
    // 少数派高度要点名到具体按钮：1334 个 32px 里混一个 33px，只有"是哪一颗"能让人去修。
    // 占比 >2% 的不点名 —— 那是 mini/small 两档共存，属于合法。
    for (const [h, n] of list.slice(1)) {
      if (n / total > 0.02) continue;
      console.log(`      ! ${k} 档里的 ${h}px×${n}：${[...(whoKey[`${k}@${h}`] || [])].slice(0, 3).join('  ')}`);
    }
  }

  // 6. 空态图标语义（就地报错的 role=alert 单列，不混进空态直方图）
  console.log('\n== 空态图标使用 ==');
  const icons = mode(ok.flatMap((x) => x.empties.map((e) => e.icon || '(无)')));
  for (const [v, n] of icons) console.log(`   ${v} × ${n}`);
  const alertIcons = mode(ok.flatMap((x) => (x.alerts || []).map((e) => e.icon || '(无)')));
  console.log(`   —— 就地报错 (role=alert) 图标：` + (alertIcons.length ? alertIcons.map(([v, n]) => `${v}×${n}`).join('  ') : '无'));
  const folderish = ok.flatMap((x) => x.empties.filter((e) => /folder/.test(e.icon)).map((e) => ({ page: x.name, title: e.title })));
  const wrong = folderish.filter((f) => !/空间|文件|目录|夹|文档/.test(f.title));
  if (wrong.length) { console.log('   ! 文件夹图标配在非文件夹语义的空态上：'); for (const w of wrong) console.log(`      ${w.page}  "${w.title}"`); }

  // 7. 面包屑 / 页脚覆盖
  const noCrumb = ok.filter((x) => !x.crumb).map((x) => x.name);
  const noFooter = ok.filter((x) => !x.footer).map((x) => x.name);
  console.log(`\n== 面包屑缺失 ${noCrumb.length} 页 ==\n   ` + noCrumb.slice(0, 20).join(', '));
  console.log(`== 页脚缺失 ${noFooter.length} 页 ==\n   ` + noFooter.slice(0, 20).join(', '));

  fs.writeFileSync(path.join(here, '..', 'ui-consistency.json'), JSON.stringify(rows, null, 1));
  console.log('\n明细已写入 web/ui-consistency.json');
  if (errored.length) process.exitCode = 1;
};

run();
