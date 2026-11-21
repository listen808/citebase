// 页面对等性巡检：把每一页**画出来**（截图）并抽出**版面指纹**，然后按模板聚类逐页对比。
//
// 为什么需要它：前面九台工装都是"从一条判据出发扫全站"（字号、圆角、间距、按压…），
// 它们能证明"每个值都落在尺子上"，却回答不了用户真正会问的那句 ——
// 「这两页看起来是不是同一套版式？」。两个页面每个单独量都合规，
// 并排放时页头高低差 20px、正文起点左缩进差一档，照样一眼看出不一致。
//
//   P1 版面指纹：顶栏高 / 页头块顶边与高 / h1 左缩进 / 首个内容块顶边 / 页脚顶边
//      —— 同一模板簇里这些值应当一致，离群即"看起来不一样"。
//   P2 模板簇：按 (版心宽度, 有无面包屑, 有无侧栏) 自动分簇，只在同簇内比。
//      跨簇比是错的：设置子页有左导航、列表页没有，页头位置本就该不同。
//   P3 contact sheet：把每页缩略图拼成一张大图写盘，供人眼逐页对（脚本判不了的东西）。
//
// 用法（前端 dev :3000、后端 :3001 需在跑；口令只走环境变量）：
//   SNAP_USER=… SNAP_PASS=… node scripts/check-page-parity.mjs [--only=<路由名子串>] [--sheet=1]
//   node scripts/check-page-parity.mjs --selftest   # 反向验证：注入一个页头偏移的页面，P1 必须抓到
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

const here = path.dirname(fileURLToPath(import.meta.url));
const argv = process.argv.slice(2);
const argOf = (k) => (argv.find((a) => a.startsWith(`--${k}=`)) || '').split('=')[1] || '';
const ONLY = argOf('only');
const TARGETS = await pickTargets(ONLY, here);
const SHEET_DIR = path.join(here, '..', '..', 'docs', 'ui-snapshots', 'parity');
const TOL = Number(process.env.PARITY_TOL || 2);

// 版面指纹：全部取"真实渲染盒"，不读源码。
const FINGERPRINT = () => {
  const r = (el) => el.getBoundingClientRect();
  const nav = document.querySelector('[data-ap=nav-bar], header');
  const foot = document.querySelector('[data-ap=footer], footer');
  // 页头优先认 ApPageContainer 页头槽那枚 h1（锚点 data-ap-page-title）；取不到再退回
  // "第一个**看得见**的 h1"。退回这一档是必须的：原先直接取第一个 h1，于是编辑器页那种
  // **故意做成视觉隐藏**的标题（整页没有可见页头，文档名只在面包屑里）会被当成页头，
  // 量到一个 1px 的盒子 —— 2026-10-02 的 4 条假"排版离群"全部来自这里。
  const anchor = document.querySelector('[data-ap-page-title]');
  // 退回档只用于分辨"另写了一份页头"与"没有可见页头"，不参与量测；
  // 且必须跳过 markdown 正文子树 —— 编辑器里文档自己的 `# 标题` 会渲染成 h1，
  // 那是内容不是页头（与 check:geom 对 .md-preview / [data-ap=markdown-preview] 的豁免同一条口径）。
  const inContent = (el) => !!el.closest('[data-ap=markdown-preview], .md-preview, .chat-md-content, .monaco-editor, [data-ap=doc-content]');
  const loose = [...document.querySelectorAll('main h1, h1')]
    .find((el) => { const b = el.getBoundingClientRect(); return b.height > 4 && b.width > 4 && !inContent(el); }) || null;
  // 页头指标只认锚点。退回"任意可见 h1"会把编辑器页正文里的 markdown 大标题当页头量
  // （document-edit 的"演练"就是），那又是一条假离群 —— 所以 loose 只用来分辨
  // "这页另写了一份页头" 与 "这页根本没有可见页头"，不参与量测。
  // 页头指标只认容器页头锚点。退回"任意可见 h1"会把正文里的 markdown 大标题当页头量
  // （document-edit 的"演练"就是），那又是一条假离群 —— 所以 objectTitle / loose 只用来
  // 把"没量到"分成三种事实说清楚，不参与量测。
  const h1 = anchor;
  const h1Source = anchor ? 'page'
    : document.querySelector('[data-ap-object-title]') ? 'object'
    : loose ? 'loose' : 'none';
  // 锚点优先。原先只认 `main nav[aria-label]`，靠文档顺序才没撞上 SettingsView 的
  // 「设置分组导航」—— 多一个 nav 或调一下顺序，crumbY 就会量到别的元素上（假离群）。
  const crumb = document.querySelector('[data-ap="breadcrumb"]') || document.querySelector('main nav[aria-label]');
  const side = document.querySelector('[data-ap=sidebar]');
  const pc = document.querySelector('[data-ap=page-container]');
  const out = {
    navH: nav ? Math.round(r(nav).height) : -1,
    navTop: nav ? Math.round(r(nav).top) : -1,
    footH: foot ? Math.round(r(foot).height) : -1,
    footTop: foot ? Math.round(r(foot).top) : -1,
    h1x: h1 ? Math.round(r(h1).left) : -1,
    h1y: h1 ? Math.round(r(h1).top) : -1,
    h1h: h1 ? Math.round(r(h1).height) : -1,
    hasCrumb: !!crumb,
    crumbY: crumb ? Math.round(r(crumb).top) : -1,
    hasSide: !!side,
    // 页头来自哪里：1 = ApPageContainer 的页头槽；0 = 页面自己另写的可见 h1；-1 = 整页没有可见页头
    h1Source,
    // 首个内容块：页头之后第一个有描边或面板底的卡片/表格，量它的顶边与左边
    firstBlockX: -1, firstBlockY: -1,
  };
  if (pc && h1) {
    const hr = r(h1);
    const cands = [...pc.querySelectorAll('[data-ap=card], [data-ap=stat-card], [data-ap=table], [data-ap=entity-card], [data-ap=page-content-card]')];
    const below = cands.map((el) => r(el)).filter((b) => b.top > hr.bottom - 1 && b.width > 40)
      .sort((a, b) => a.top - b.top || a.left - b.left)[0];
    if (below) { out.firstBlockX = Math.round(below.left); out.firstBlockY = Math.round(below.top); }
  }
  out.contentW = pc ? Math.round(r(pc).width) : -1;
  // h1 右边如果有**同行控件**（返回按钮、页头操作按钮），它的左边缘本来就该右移。
  // check:ui 早就按这个口径扣过（"4 个带返回按钮的页会报多缩进 36px"），
  // 这里不扣就会把那 4 页报成版式离群 —— 假阳性。
  if (h1) {
    const hr = r(h1);
    let lead = 0;
    for (const el of document.querySelectorAll('main button, main a')) {
      const b = r(el);
      if (b.bottom <= hr.top + 2 || b.top >= hr.bottom - 2) continue;
      if (b.right <= hr.left + 1 && b.right > hr.left - 200 && hr.left - b.left < 120) lead = Math.max(lead, hr.left - b.left);
    }
    out.h1Lead = lead;
    out.h1xNet = Math.round(hr.left) - lead;
  }
  // 内层子导航（AI 设置那种"页中页"左栏）：有它就是另一套模板，不能和列表页同簇比。
  // 必须按**几何形状**认而不是按组件名：AISettingsLayout 的那条左栏是 <main> 里
  // 一个裸 <nav>（没有 aria-label、也不是 ApSidebar），按名字找会漏掉 30 个 AI 子页。
  // 参照边用页头左缘；这页没有可见页头时退回**版心左缘**，不能用页头的存在与否来开关这一档
  // —— 否则"页头取不到"会连带把页面换到另一个模板簇里，跨模板比出假离群（2026-10-02 踩过）。
  const refLeft = h1 ? Math.round(r(h1).left) : (pc ? Math.round(r(pc).left) : 0);
  const inColumn = (b) => b.width >= 160 && b.width <= 320 && Math.abs(b.left - refLeft) <= 8;
  const geom = [...document.querySelectorAll('main *')].some((el) => {
    const b = r(el);
    return b.height > 200 && inColumn(b);
  });
  // 几何档漏掉的是"矮栏"：/settings 的分组导航只有四个按钮、约 136px 高，于是整页被塞进
  // 单列簇，再跟 36 个列表页比 firstBlockX，报出一个假离群（480 vs 256）。
  // 语义档补这个：<nav> 本身就是导航，只要它在内容左缘起一条列、在页头之下、并且高到有内容
  // （面包屑也是 <nav>，但它在页头之上；分页条也在下面却只有 32px 高 —— 都不算一条列）。
  const belowHeader = (h1 ? r(h1).bottom : (pc ? r(pc).bottom : 0)) - 4;
  const semantic = [...document.querySelectorAll('main nav, main [role="navigation"]')].some((el) => {
    const b = r(el);
    return b.height > 80 && b.top >= belowHeader && inColumn(b);
  });
  out.innerNav = geom || semantic ? 1 : 0;
  return out;
};

const KEYS = ['navH', 'navTop', 'footH', 'h1xNet', 'h1y', 'h1h', 'crumbY', 'firstBlockX'];
// firstBlockY（页头到首个内容块的纵向距离）**只登记不当门禁**：它由页面自己在中间插没插
// 引导条/提示条决定（ai-settings__overview 顶部那条「配置引导」就有 278px 高），
// 那是内容不是版式。横向的 firstBlockX 才是列对齐，留在门禁里。
const REGISTER_KEYS = ['firstBlockY'];

const median = (a) => { const s = [...a].sort((x, y) => x - y); return s[Math.floor(s.length / 2)]; };
const mode = (a) => { const c = {}; for (const v of a) c[v] = (c[v] || 0) + 1; return Object.entries(c).sort((x, y) => y[1] - x[1])[0]?.[0]; };

const run = async () => {
  fs.mkdirSync(SHEET_DIR, { recursive: true });
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  await installErroredProbe(page);
  page.on('dialog', (d) => (d.type() === 'beforeunload' ? d.accept() : d.dismiss()).catch(() => {}));
  await loginForm(page, BASE, USER, PASS);

  const shots = [];
  const rows = [];
  const failed = [];
  for (const r of TARGETS) {
    try {
      await page.goto(`${BASE}${r.path}`, { waitUntil: 'load', timeout: 30000 });
      await page.waitForTimeout(Number(process.env.SNAP_GAP_MS || 1000));
      await page.mouse.move(1, 1);
      await page.waitForTimeout(150);
      const fp = await page.evaluate(FINGERPRINT);
      const crashed = await page.evaluate(() => (window.__apErrored ? window.__apErrored() : false));
      const buf = await page.screenshot({ path: path.join(SHEET_DIR, `${r.name}.png`) });
      rows.push({ name: r.name, path: r.path, ...fp, crashed, bytes: buf.length });
      shots.push({ name: r.name, file: path.join(SHEET_DIR, `${r.name}.png`) });
    } catch (e) {
      failed.push(`${r.name}: ${String(e).slice(0, 50)}`);
    }
  }
  // 被判进兜底页的先自动复跑一次（多半是巡检自己打满限流），并且把缩略图重截 ——
  // 否则 contact sheet 里会留一张错误页，人眼那一遍就看到假缺陷。
  await retryCrashedPages(page, BASE, rows, async (p, row) => {
    const fp = await p.evaluate(FINGERPRINT);
    const crashed = await p.evaluate(() => (window.__apErrored ? window.__apErrored() : false));
    const buf = await p.screenshot({ path: path.join(SHEET_DIR, `${row.name}.png`) });
    Object.assign(row, fp, { crashed, bytes: buf.length });
  });

  // P2 分簇：只在同版式内比。
  // 错误态页（ErrorBoundary）不进统计：它的指纹全是垃圾值，算进簇里就是把工装问题当产品问题卖。
  // "没有可见页头"不走这条路——那是页面版式的真实事实，由 -1 哨兵在单个指标上缺席即可。
  const clusters = {};
  const crashedPages = [];
  for (const row of rows) {
    if (row.crashed) { crashedPages.push(row.name); continue; }
    const key = `${row.contentW}px${row.hasCrumb ? '+crumb' : ''}${row.hasSide ? '+side' : ''}${row.innerNav ? '+subnav' : ''}`;
    (clusters[key] = clusters[key] || []).push(row);
  }
  if (crashedPages.length) console.log(`   （${crashedPages.length} 页落在错误态，已排除且不算离群——这是"没量到"不是"没排版"：${crashedPages.slice(0, 6).join(', ')}${crashedPages.length > 6 ? ' …' : ''}）`);

  const report = [];
  const register = [];
  for (const [key, list] of Object.entries(clusters)) {
    if (list.length < 3) continue;
    for (const k of [...KEYS, ...REGISTER_KEYS]) {
      const vals = list.map((x) => x[k]).filter((v) => v >= 0);
      if (vals.length < 3) continue;
      const m = Number(mode(vals));
      const bad = list.filter((x) => x[k] >= 0 && Math.abs(x[k] - m) > TOL);
      if (bad.length) (KEYS.includes(k) ? report : register).push({ key, metric: k, expect: m, actual: median(vals), bad: bad.map((x) => [x.name, x[k]]) });
    }
  }

  console.log(`截图页数 = ${rows.length} / ${TARGETS.length}，模板簇 = ${Object.keys(clusters).length} 个`);
  // "页头没量到"要分成三种事实说清楚，混成一桶就会把设计意图报成缺陷：
  // object = 这页的标题是**对象头部卡**（空间详情头 / 阅读区正文标题），位置由内容决定，本就不该按页头尺子比；
  // loose  = 另写了一份页头 h1 而没走容器槽 —— 只有这一类是要修的偏离；
  // none   = 整页没有可见页头（编辑页故意把文档名只放进面包屑）。
  const bySource = (k) => rows.filter((x) => x.h1Source === k).map((x) => x.name);
  const objectPages = bySource('object');
  const loosePages = bySource('loose');
  const noTitle = bySource('none');
  if (objectPages.length) console.log(`   （${objectPages.length} 页的标题是对象头部卡（data-ap-object-title），位置由内容决定，页头类指标不比：${objectPages.join(', ')}）`);
  if (loosePages.length) console.log(`   （${loosePages.length} 页另写了一份页头 h1、没走容器槽 —— 这才是偏离：${loosePages.join(', ')}）`);
  if (noTitle.length) console.log(`   （${noTitle.length} 页整页没有可见页头 h1（不是"离群"，是"没有这个东西"）：${noTitle.join(', ')}）`);
  for (const [k, list] of Object.entries(clusters).sort((a, b) => b[1].length - a[1].length)) {
    console.log(`   簇 ${k.padEnd(28)} ${String(list.length).padStart(3)} 页  例:${list.slice(0, 4).map((x) => x.name).join(', ')}`);
  }
  const crashed = rows.filter((x) => x.crashed);
  if (crashed.length) console.log(`\n✗ ${crashed.length} 页被 ErrorBoundary 接住：${crashed.map((x) => x.name).join(', ')} —— 先当限流复跑，别读成版式问题`);

  console.log(`\n== P1 同簇版面指纹离群（容差 ±${TOL}px）：${report.length} 项 ==`);
  for (const item of report.slice(0, 40)) {
    console.log(`   [${item.key}] ${item.metric} 众数=${item.expect}px，${item.bad.length} 页偏离：${item.bad.slice(0, 6).map(([n, v]) => `${n}=${v}`).join(' ')}`);
  }
  if (report.length > 40) console.log(`   … 另有 ${report.length - 40} 项`);
  if (register.length) {
    console.log(`\n== 纵向节奏登记表（由页面中间插的内容条决定，不计入门禁）：${register.length} 项 ==`);
    for (const item of register.slice(0, 8)) console.log(`   [${item.key}] ${item.metric} 众数=${item.expect}px，${item.bad.length} 页不同：${item.bad.slice(0, 6).map(([n, v]) => `${n}=${v}`).join(' ')}`);
  }

  // P3 contact sheet：拼一张大图，供人眼逐页对
  if (argOf('sheet') !== '0' && shots.length) {
    // 拼图页写成截图**同目录**的 HTML，再用 file:// 打开。前两版都在这上面翻过车：
    // base64 内嵌时页面本身是 data: 源，图片解不出来，产出 78KB 的"全白" sheet ——
    // 而全白看起来很像"这些页本来就是白的"，是最容易放过去的一种假结果；
    // 从 setContent 页里引 file:/// 绝对路径则被 Chromium 直接拒绝加载。
    const cols = 4;
    const cellW = 360;
    const cellH = 240;
    const html = `<!doctype html><meta charset=utf-8><body style="margin:0;background:#111;font:11px system-ui">`
      + `<div style="display:grid;grid-template-columns:repeat(${cols},${cellW}px)">`
      + shots.map((x) => `<div style="background:#fff;overflow:hidden"><div style="background:#000;color:#fff;padding:2px 4px">${x.name}</div>`
        + `<img src="${path.basename(x.file)}" style="width:${cellW}px;height:${cellH}px;object-fit:cover;object-position:left top;display:block"></div>`).join('')
      + `</div></body>`;
    const sheetHtml = path.join(SHEET_DIR, 'sheet.html');
    fs.writeFileSync(sheetHtml, html);
    const sheetPage = await browser.newPage({ viewport: { width: cols * cellW + 20, height: 1200 } });
    await sheetPage.goto('file:///' + sheetHtml.replace(/\\/g, '/'), { waitUntil: 'load' });
    const loaded = await sheetPage.evaluate(async () => {
      await Promise.all([...document.images].map((i) => i.decode().catch(() => {})));
      return { n: document.images.length, ok: [...document.images].filter((i) => i.naturalWidth > 0).length };
    });
    const rowsN = Math.ceil(shots.length / cols);
    await sheetPage.setViewportSize({ width: cols * cellW + 20, height: Math.min(rowsN * (cellH + 18) + 40, 15000) });
    const out = path.join(SHEET_DIR, 'contact-sheet.png');
    await sheetPage.screenshot({ path: out, fullPage: true });
    if (!loaded.ok) console.log(`✗ contact sheet 一张图都没加载成功（${loaded.n} 个 img）—— 这张 sheet 不能当"看过了"用`);
    else console.log(`contact sheet 加载 ${loaded.ok}/${loaded.n} 页`);
    await sheetPage.close();
    console.log(`\ncontact sheet（人眼逐页对）：${out}`);
  }

  await browser.close();
  if (failed.length || rows.length === 0) {
    console.log(`\n✗ 有页面没画出来（失败 ${failed.length}）：${failed.slice(0, 3).join(' | ')}`);
    process.exitCode = 1;
  }
  if (crashed.length) process.exitCode = 1;
  const gate = report.length;
  console.log(`\nP1 离群项 ${gate}；明细已写入 web/page-parity.json`);
  fs.writeFileSync(path.join(here, '..', 'page-parity.json'), JSON.stringify({ rows, clusters: Object.fromEntries(Object.entries(clusters).map(([k, v]) => [k, v.map((x) => x.name)])), report }, null, 1));
  if (gate) process.exitCode = 1;
};

// 反向验证：把某一页的页头整体下移 20px，P1 必须在同簇里把它报出来。
// 没有这一步，"0 项离群"完全可能只是指纹取不到值（比如 h1 选择器没命中）。
const selftest = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  await installErroredProbe(page);
  await loginForm(page, BASE, USER, PASS);
  const clean = [];
  for (const p of ['/collections', '/links', '/contacts', '/spaces', '/media']) {
    await page.goto(`${BASE}${p}`, { waitUntil: 'load', timeout: 30000 });
    await page.waitForTimeout(1400);
    clean.push(await page.evaluate(FINGERPRINT));
  }
  await page.goto(`${BASE}/collections`, { waitUntil: 'load', timeout: 30000 });
  await page.waitForTimeout(1400);
  await page.evaluate(() => {
    const h1 = document.querySelector('main h1');
    h1.style.marginTop = '20px';
    const nav = document.querySelector('[data-ap=nav-bar], header');
    if (nav) nav.style.height = '60px';
  });
  await page.waitForTimeout(250);
  const broken = await page.evaluate(FINGERPRINT);

  const medH1 = median(clean.map((x) => x.h1y));
  const medNav = Number(mode(clean.map((x) => x.navH)));

  // 内层左栏的判据要能认出"矮栏"：/settings 那条分组导航只有四个按钮（约 136px 高），
  // 以前只认 height>200，于是它被塞进单列簇、跟列表页比 firstBlockX，报出一个假离群。
  // 同时不能把分页条那种"在下面但很矮"的 <nav> 当成一条列。
  const railProbe = async (heightPx) => {
    await page.goto(`${BASE}/collections`, { waitUntil: 'load', timeout: 30000 });
    await page.waitForTimeout(1400);
    await page.evaluate((h) => {
      const h1 = document.querySelector('main h1');
      const b = h1.getBoundingClientRect();
      const n = document.createElement('nav');
      n.setAttribute('aria-label', '自造左栏');
      n.style.cssText = `position:absolute;left:${Math.round(b.left)}px;top:${Math.round(b.bottom) + 2}px;width:220px;height:${h}px`;
      document.querySelector('main').appendChild(n);
    }, heightPx);
    await page.waitForTimeout(200);
    return await page.evaluate(FINGERPRINT);
  };
  const shortRail = await railProbe(136);
  const thinBar = await railProbe(32);
  await browser.close();

  const checks = [
    [`指纹取得到值（h1y 中位 ${medH1}，顶栏众数 ${medNav}）`, medH1 > 0 && medNav > 0],
    [`页头下移 20px 被抓到（${broken.h1y} vs ${medH1}）`, Math.abs(broken.h1y - medH1) > TOL],
    [`顶栏变高被抓到（${broken.navH} vs ${medNav}）`, Math.abs(broken.navH - medNav) > TOL],
    ['136px 高的左栏认成"页中页"（innerNav=1）', shortRail.innerNav === 1],
    ['32px 高的条（分页条那种）不算一条列（innerNav=0）', thinBar.innerNav === 0],
  ];
  let bad = 0;
  for (const [k, ok] of checks) { if (!ok) bad++; console.log(`${ok ? '✓' : '✗'} ${k}`); }
  console.log(bad ? `✗ ${bad} 条没抓到自造违例——指纹取值或分簇判据写错了` : '版面指纹抓得到自造的页头偏移');
  process.exitCode = bad ? 1 : 0;
};

if (argv.includes('--selftest')) selftest(); else run();
