// 逐页对照 apple-design 判据做审计。这台只管**运行时能量到、且现有九台没覆盖**的那几条：
//   A1 动画属性必须合成器专属（§11：只动 transform/opacity）。
//      transition-property 里出现 width/height/top/left/margin/padding，动画期间就在触发布局重排；
//      `all` 单列一档 —— 它把布局属性也一起接管了，是"看着没事、改一行就抖"的那种。
//   A2 功能层材质一致性（§12）：sticky/fixed 的条（导航/工具栏/吸底操作条）该是同一档材质。
//      不判"必须半透明"（五套主题里实心面是刻意的），判**同一页里混了几档材质**、
//      以及**全站同类 chrome 是不是各写一套**。
//   A3 半透明叠半透明（§12：never stack a light translucent surface on another）。
//   A4 只有 hover 才现的操作（§16 Agency/Flexibility）：opacity≈0 的容器里装着可交互控件，
//      键盘聚焦进去之后仍然不显形 —— 键盘用户既看不见也够不着。判法是**真去 focus 一个后代再读一次**。
//   A5 触控命中区实测最小边（登记，不当门禁：44 只对触屏档成立，那条归 check:geom）。
//   A6 吸顶条下面用 1px 硬线分隔、却没有 scroll-edge 遮罩（§12 登记项）。
// 另外 `--table` 把九台已有产物按页合并成一张记分卡，回答"哪几页有问题"这句。
// 用法（dev :3000、后端 :3001 在跑；口令只走环境变量）：
//   SNAP_USER=… SNAP_PASS=… node scripts/check-apple-design.mjs [--only=<路由名子串>] [--max=18]
//   node scripts/check-apple-design.mjs --selftest   # 反向验证：A1–A4 必须各自抓到注入的违例
//   node scripts/check-apple-design.mjs --table      # 只合并产物出记分卡，不开浏览器
import { chromium } from 'playwright';
import fs from 'node:fs';
import path from 'node:path';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { findChromium } from './lib/chromium-path.mjs';
import { pickTargets } from './lib/route-list.mjs';
import { loginForm } from './lib/page-state.mjs';

const BASE = process.env.SNAP_BASE || 'http://localhost:3000';
const here = path.dirname(fileURLToPath(import.meta.url));
const argv = process.argv.slice(2);
const argOf = (k) => (argv.find((a) => a.startsWith(`--${k}=`)) || '').split('=')[1] || '';
const ONLY = argOf('only');
const MAX = Number(argOf('max') || 18);
const ROOT = path.join(here, '..');

const LAYOUT_PROPS = ['width', 'height', 'top', 'left', 'right', 'bottom', 'margin', 'padding',
  'min-width', 'max-width', 'min-height', 'max-height', 'line-height', 'font-size'];

const AUDIT = async (cfg) => {
  const rectOf = (el) => el.getBoundingClientRect();
  const ap = (el) => el.getAttribute('data-ap') || el.closest('[data-ap]')?.getAttribute('data-ap') || '?';
  const alphaOf = (c) => {
    if (!c || c === 'transparent') return 0;
    const slash = c.match(/\/\s*([\d.]+)(%)?\s*\)/);
    if (slash) return Math.min(1, parseFloat(slash[1]) * (slash[2] ? 0.01 : 1));
    const fn = c.match(/^rgba?\(([^)]+)\)$/);
    if (fn) {
      const p = fn[1].split(/[\s,]+/).filter(Boolean);
      if (p.length >= 4) return Math.min(1, parseFloat(p[3]) * (p[3].endsWith('%') ? 0.01 : 1));
      return 1;
    }
    const hex8 = c.match(/^#([0-9a-f]{8})$/i);
    if (hex8) return parseInt(hex8[1].slice(6, 8), 16) / 255;
    return 1;
  };
  const out = { a1: [], a1b: [], a1e: [], a2: [], a3: [], a4: [], a5: [], a6: [], chrome: 0, hoverable: 0, vw: innerWidth };

  // ---- A1 动画属性（A1 = 明确在动布局属性；A1b = transition:all，把布局属性也一并接管了）----
  for (const el of document.querySelectorAll('body [data-ap] *, body [data-ap]')) {
    const cs = getComputedStyle(el);
    if (cs.transitionProperty === 'none' || cs.transitionDuration === '0s') continue;
    const props = cs.transitionProperty.split(',').map((x) => x.trim());
    const bad = props.filter((x) => cfg.layout.includes(x));
    if (bad.length === 1 && bad[0] === 'max-width') {
      // 天生绑布局的那一个：阅读栏在「阅读宽度 ↔ 宽版」之间切换时正文列要平滑变宽。
      // 文字列不能 scaleX（字会被拉扁），只能动 max-width —— 这是设计意图不是漏网。
      // 与 check:geom 的 R9 同一个做法：判据留着、命中带理由登记，不为凑一条修复去改对的代码。
      out.a1e.push(`${ap(el)}/${el.tagName.toLowerCase()} 只动 max-width（正文列宽切换，绑布局）`);
    } else if (bad.length) out.a1.push(`${ap(el)}/${el.tagName.toLowerCase()} 动布局属性 ${[...new Set(bad)].join('+')}`);
    else if (props.includes('all')) out.a1b.push(`${ap(el)}/${el.tagName.toLowerCase()} transition:all`);
  }

  // ---- A2/A6 功能层 chrome ----
  for (const el of document.querySelectorAll('body [data-ap] *, body [data-ap]')) {
    const cs = getComputedStyle(el);
    if (cs.position !== 'sticky' && cs.position !== 'fixed') continue;
    const r = rectOf(el);
    if (r.width < 280 || r.height < 20 || r.height > 96) continue;
    const cs2 = getComputedStyle(el);
    const bf = cs2.backdropFilter === 'none' ? '' : cs2.backdropFilter;
    const a = alphaOf(cs2.backgroundColor);
    // 完全不透明的定位壳（toast 容器那类）不是一种材质 —— 它什么都不画，
    // 拿它和导航条比"同页混了几档材质"是假阳性。
    if (a <= 0.02) continue;
    out.chrome++;
    const mat = `${a >= 0.98 ? '实心' : '半透' + Math.round(a * 100)}+${bf ? 'blur' : '无blur'}`;
    /**
     * 带 `data-scroll-edge` 的条**不参与"同页混档材质"这条判断**。
     * 它不是"随手选了另一个档位"，而是**按身后有没有内容滚过**在两档之间切换：
     * 顶栏静止态走 chrome（通透），ApTable 表头静止态走 bg-secondary（底下有行要压），
     * 两者档位不同恰恰是 scroll-edge 设计要求的。把它们算进同一只集合，
     * 21 个 ai-settings 子页就会一起报"混档"—— 判据把正确的分工当成了不一致。
     * A6（硬线无溶解）仍然看它们：那条判的是"有没有接遮罩"，与档位无关。
     */
    if (!el.hasAttribute('data-scroll-edge')) out.a2.push(`${ap(el)}/${el.tagName.toLowerCase()} ${mat}`);
    const bl = parseFloat(cs2.borderBottomWidth);
    /**
     * A6 原来只认 `mask-image`，而本站的 scroll-edge 溶解**不是 mask**：
     * 它是主题层发的一条 `::after` 渐变（`linear-gradient(to bottom, 壳面色, transparent)`），
     * 外加元素上的 `data-scroll-edge` 状态位。于是这条判据在 73 页上全亮 ——
     * 不是 73 页都缺溶解，是探针找的是一条不存在的实现。
     * 现在按真实机制认：状态位在场，或 `::after` 里有一条渐隐到 transparent 的渐变，都算已接。
     */
    const after = getComputedStyle(el, '::after');
    const afterFade = /gradient\([^)]*transparent/i.test(after.backgroundImage || '');
    const hasEdge = el.hasAttribute('data-scroll-edge') || afterFade;
    /**
     * A6 的触发条件从"有没有溶解遮罩"改成"**这条底下到底有没有可滚内容经过**"。
     * 理由有两条，都是这一轮现场学到的：
     *  ① 原判据猜的是实现（`mask-image`），而本站的溶解是 `::after` 渐变 + 状态位，
     *    于是它在 73 页全亮、且永远不可能被满足 —— 判据写错方向比缺判据更坏。
     *  ② 就算认对了机制，它仍然分不清"忘了接"和"不该接"：编辑器的 sticky 工具条
     *    与滚动容器是**兄弟**，内容从不从它底下经过，给它接 scroll-edge 就是死代码
     *    （这条已经写进 composables/ui/useScrollEdge.ts 的文件头）。
     * 现在判的是前提本身：取这条 bar 覆盖的那块矩形，看它下面是否压着一个
     * 真的还能滚的容器（scrollHeight > clientHeight）。压着 → 该有溶解；没压着 → 与它无关。
     */
    const passesUnder = (() => {
      const cx = Math.round(r.left + r.width / 2);
      const cy = Math.round(r.top + Math.min(r.height / 2, 8));
      if (cx < 0 || cy < 0 || cx > innerWidth || cy > innerHeight) return false;
      for (const below of document.elementsFromPoint(cx, cy)) {
        // 只跳过"这条自己"和"画在它上面的后代"。**祖先不能跳** ——
        // sticky 条的滚动容器恰恰就是它的祖先，第一版把 `below.contains(el)` 也排除了，
        // 于是 selftest 自造的违例都抓不到（这条判据当场是瞎的，靠 --selftest 才露出来）。
        if (below === el || el.contains(below)) continue;
        const bs = getComputedStyle(below);
        if (!['auto', 'scroll'].includes(bs.overflowY)) continue;
        if (below.scrollHeight > below.clientHeight + 1) return true;
        const doc = document.scrollingElement;
        if (doc && doc.scrollHeight > doc.clientHeight + 1 && doc.contains(below)) return true;
      }
      return false;
    })();
    if (bl > 0 && bl <= 2 && !hasEdge && passesUnder) {
      out.a6.push(`${ap(el)}/${el.tagName.toLowerCase()} 硬线 ${bl}px 无 scroll-edge（底下确有可滚内容经过）`);
    }
  }

  // ---- A3 半透明叠半透明 ----
  for (const el of document.querySelectorAll('body [data-ap] *, body [data-ap]')) {
    const cs = getComputedStyle(el);
    if (cs.backdropFilter === 'none') continue;
    if (alphaOf(cs.backgroundColor) >= 0.98) continue;
    for (let a = el.parentElement; a; a = a.parentElement) {
      const acs = getComputedStyle(a);
      if (acs.backdropFilter !== 'none' && alphaOf(acs.backgroundColor) < 0.98) {
        out.a3.push(`${ap(el)}/${el.tagName.toLowerCase()} 叠在 ${ap(a)}/${a.tagName.toLowerCase()} 上`);
        break;
      }
    }
  }

  // ---- A4 只有 hover 才现的操作：真去 focus 一个后代再读一次 ----
  const INTERACTIVE = 'button, a[href], input, select, textarea, [role=button], [role=menuitem], [role=tab]';
  for (const el of document.querySelectorAll('body *')) {
    const cs = getComputedStyle(el);
    if (parseFloat(cs.opacity) > 0.05) continue;
    if (cs.display === 'none' || cs.visibility === 'hidden') continue;
    const ctl = el.querySelector(INTERACTIVE);
    if (!ctl) continue;
    // toast 正在离场时也是 opacity 0 且仍挂载（`TransitionGroup` 留 250ms 淡出），
    // 那期间它带着一个可聚焦的关闭按钮 —— A4 认的是"只有 hover 才现的常驻操作条"，
    // 不是"正在消失的东西"。限流那阵一次退三四条 toast，这一档就会凭空地响。
    if (el.closest('[data-ap="toast"]')) continue;
    const r = rectOf(el);
    if (r.width < 8 || r.height < 8) continue;
    out.hoverable++;
    if (out.hoverable > cfg.cap) continue;
    const prev = document.activeElement;
    let shownByFocus;
    try {
      ctl.focus({ preventScroll: true });
      // 必须等一帧 + 过渡走完再读：focusin 触发的是 Vue 的响应式改样式（下一帧才落地），
      // 而显形本身是 300ms 过渡 —— 立刻读会量到 0.03 这种中间值，把已修好的页面报成没修
      // （contacts 就这么被冤枉过一次）。
      await new Promise((r) => requestAnimationFrame(() => requestAnimationFrame(r)));
      await new Promise((r) => setTimeout(r, cfg.settleMs));
      shownByFocus = parseFloat(getComputedStyle(el).opacity) > 0.5;
    } catch { shownByFocus = false; }
    if (prev && prev.focus) { try { prev.focus({ preventScroll: true }); } catch { /* 回不去就不算 */ } }
    if (!shownByFocus) {
      const txt = (el.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 18);
      out.a4.push(`${ap(el)}/${el.tagName.toLowerCase()} 内含 ${ctl.tagName.toLowerCase()}「${txt}」聚焦后仍 opacity 0`);
    }
  }

  // ---- A5 命中区：两档视口判两条不同的轴 ----
  // 桌面看最小边（<24 就是点不中的小目标）；触屏只看**高度** —— 宽度由文字决定，
  // 仓库既定的补偿规则就是"触屏下高度一起补、宽度只给图标控件补方形"（ApButton 同一条）。
  const touch = innerWidth < 641;
  for (const el of document.querySelectorAll(INTERACTIVE)) {
    if (!el.closest('main, [data-ap-scroll]')) continue;
    const r = rectOf(el);
    if (r.width < 4 || r.height < 4) continue;
    const cs = getComputedStyle(el);
    if (cs.visibility === 'hidden' || parseFloat(cs.opacity) < 0.05) continue;
    const short = touch ? 44 - r.height : 24 - Math.min(r.width, r.height);
    if (short > 0) out.a5.push(`${ap(el)}/${el.tagName.toLowerCase()} 命中 ${Math.round(r.width)}×${Math.round(r.height)}${touch ? '（触屏档高度不足 44）' : ''}`);
  }
  return out;
};

// 反向验证：每条判据注入一处明显违例，四条都必须抓到。
const INJECT = () => {
  const host = document.querySelector('main') || document.body;
  const box = document.createElement('div');
  box.setAttribute('data-ap', 'zzapple');
  box.style.cssText = 'position:relative;display:block;width:600px;height:420px;margin:0;padding:0';
  box.innerHTML = ''
    // A1：把 width 放进 transition-property
    + '<div data-ap="zzw" style="transition: width 0.3s ease, opacity 0.2s;width:120px;height:20px;background:#888">a</div>'
    // A2：两条 sticky chrome，一实心一半透 —— 同页混档材质
    + '<div data-ap="zzbar1" style="position:sticky;top:0;width:600px;height:44px;background:#ffffff">b</div>'
    + '<div data-ap="zzbar2" style="position:sticky;top:0;width:600px;height:44px;background:rgba(255,255,255,0.6);backdrop-filter:blur(10px)">c</div>'
    // A3：半透明叠半透明
    + '<div data-ap="zzout" style="position:static;width:400px;height:120px;background:rgba(10,10,10,0.5);backdrop-filter:blur(8px)">'
    + '<div data-ap="zzin" style="width:300px;height:60px;background:rgba(10,10,10,0.4);backdrop-filter:blur(6px)">d</div></div>'
    // A4：opacity 0 的容器里装着按钮，且没有 focus 显形路径
    + '<div data-ap="zzhide" style="opacity:0;width:200px;height:40px"><button>只有hover</button></div>'
    // A6：一条 sticky 硬线，压在**真的还能滚**的容器上，且没接 scroll-edge。
    // 这条存在的理由很具体：A6 的触发条件刚从"有没有 mask"改成"底下有没有可滚内容经过"，
    // 改完必须能红一次，否则新条件是永远绿的那一种 —— 而它替代的旧判据恰恰是因为
    // 在 73 页上永远亮才被换掉的。
    + '<div data-ap="zzscroll" style="position:relative;width:600px;height:160px;overflow-y:auto;background:#eee">'
    + '<div data-ap="zzedge" style="position:sticky;top:0;width:600px;height:44px;background:rgba(255,255,255,0.6);border-bottom:1px solid #999">e</div>'
    + '<div style="height:600px"></div></div>';
  host.appendChild(box);
  return true;
};

const UNINJECT = () => {
  const el = document.querySelector('[data-ap="zzapple"]');
  if (el) el.remove();
};

const selftest = async () => {
  const USER = process.env.SNAP_USER;
  const PASS = process.env.SNAP_PASS;
  if (!USER || !PASS) { console.error('需要 SNAP_USER / SNAP_PASS'); process.exit(2); }
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  await login(page, USER, PASS);
  await page.goto(`${BASE}/dashboard`, { waitUntil: 'load', timeout: 30000 });
  await page.waitForTimeout(1500);
  await page.evaluate(INJECT);
  const res = await page.evaluate(AUDIT, { layout: LAYOUT_PROPS, cap: 40, settleMs: 450 });
  await page.evaluate(UNINJECT);
  const after = await page.evaluate(AUDIT, { layout: LAYOUT_PROPS, cap: 40, settleMs: 450 });
  await browser.close();
  const has = (list, key) => (list || []).some((x) => x.includes(key));
  const checks = [
    [`A1 抓到动布局属性（${(res.a1 || []).filter((x) => x.includes('zzw')).join(' | ') || '没抓到'}）`, has(res.a1, 'zzw')],
    [`A2 抓到同页混档材质（chrome ${(res.a2 || []).filter((x) => /zzbar/.test(x)).map((x) => x.split(' ')[1]).join('/')}）`,
      has(res.a2, 'zzbar1') && has(res.a2, 'zzbar2') && new Set((res.a2 || []).filter((x) => x.includes('zzbar')).map((x) => x.split(' ')[1])).size > 1],
    [`A3 抓到半透明叠半透明（${(res.a3 || []).filter((x) => x.includes('zzin')).join(' | ') || '没抓到'}）`, has(res.a3, 'zzin')],
    [`A4 抓到只有 hover 的操作（${(res.a4 || []).filter((x) => x.includes('zzhide')).join(' | ') || '没抓到'}）`, has(res.a4, 'zzhide')],
    [`A6 抓到压在可滚容器上的 sticky 硬线（${(res.a6 || []).filter((x) => x.includes('zzedge')).join(' | ') || '没抓到'}）`, has(res.a6, 'zzedge')],
    [`撤掉注入后四条归零（A1 ${(after.a1 || []).filter((x) => x.includes('zz')).length} A3 ${(after.a3 || []).filter((x) => x.includes('zz')).length} A4 ${(after.a4 || []).filter((x) => x.includes('zz')).length}）`,
      !has(after.a1, 'zzw') && !has(after.a3, 'zzin') && !has(after.a4, 'zzhide') && !has(after.a6, 'zzedge')],
  ];
  let bad = 0;
  for (const [name, ok] of checks) { console.log(`${ok ? '✓' : '✗'} ${name}`); if (!ok) bad++; }
  console.log(bad ? `\n${bad} 条判据没抓到自造违例 —— 这台是瞎的` : '四条判据都抓到了自造违例');
  process.exitCode = bad ? 1 : 0;
};

const login = async (page, USER, PASS) => {
  await loginForm(page, BASE, USER, PASS);
};

const bucketize = (dst, page, list) => {
  for (const line of list || []) {
    const k = `${page} ${line}`;
    dst[k] = (dst[k] || 0) + 1;
  }
};

const show = (title, dst, note) => {
  const keys = Object.keys(dst);
  console.log(`\n== ${title}：${keys.length} 处 ==`);
  for (const k of keys.slice(0, MAX)) console.log(`   ${k}${dst[k] > 1 ? ` ×${dst[k]}` : ''}`);
  if (keys.length > MAX) console.log(`   …另 ${keys.length - MAX} 处`);
  if (note) console.log(`   ${note}`);
};

const run = async () => {
  const USER = process.env.SNAP_USER;
  const PASS = process.env.SNAP_PASS;
  if (!USER || !PASS) { console.error('需要 SNAP_USER / SNAP_PASS（口令只走环境变量，不写进仓库）'); process.exit(2); }
  const TARGETS = await pickTargets(ONLY, here);
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: Number(argOf('w')) || 1440, height: 900 }, locale: 'zh-CN' });
  page.on('dialog', (d) => (d.type() === 'beforeunload' ? d.accept() : d.dismiss()).catch(() => {}));
  await login(page, USER, PASS);

  const acc = { a1: {}, a1b: {}, a1e: {}, a2: {}, a3: {}, a4: {}, a5: {}, a6: {} };
  const perPage = [];
  let pages = 0;
  const failed = [];
  for (const r of TARGETS) {
    try {
      await page.goto(`${BASE}${r.path}`, { waitUntil: 'load', timeout: 30000 });
      await page.waitForTimeout(Number(process.env.SNAP_GAP_MS || 900));
      await page.mouse.move(1, 1);
      await page.waitForTimeout(120);
      const res = await page.evaluate(AUDIT, { layout: LAYOUT_PROPS, cap: Number(argOf('cap')) || 8, settleMs: 450 });
      pages++;
      for (const k of Object.keys(acc)) bucketize(acc[k], r.name, res[k]);
      const mats = new Set((res.a2 || []).map((x) => x.split(' ')[1]));
      perPage.push({ name: r.name, a1: res.a1.length, a1b: res.a1b.length, chrome: res.chrome, mats: [...mats], a3: res.a3.length, a4: res.a4.length, a5: res.a5.length, a6: res.a6.length });
    } catch (e) {
      failed.push(`${r.name}: ${String(e).slice(0, 60)}`);
    }
  }
  await browser.close();

  console.log(`量测页面 = ${pages} / ${TARGETS.length}，视口 = ${argOf('w') || 1440}`);
  const badPages = perPage.filter((p) => p.a1 || p.a3 || p.a4 || p.mats.length > 1);
  console.log(`有硬违例的页（A1/A3/A4 或同页混档材质）= ${badPages.length} 页`);
  for (const p of badPages.slice(0, MAX)) {
    console.log(`   ${p.name.padEnd(26)} A1=${p.a1} A1b=${p.a1b} A3=${p.a3} A4=${p.a4} 材质档=${p.mats.join('|') || '—'}（chrome ${p.chrome}）`);
  }
  show('A1 动画在动布局属性（§11 只该动 transform/opacity）', acc.a1);
  show('A1e 只动 max-width 的正文列宽切换（登记：天生绑布局，见代码注释）', acc.a1e);
  show('A1b transition:all —— 布局属性被一并接管（登记，修共享组件即可整批改掉）', acc.a1b);
  show('A2 每页 chrome 材质档位（>1 档即同页混材质）', acc.a2);
  show('A3 半透明叠半透明（§12 可读性会塌）', acc.a3);
  show('A4 只有 hover 才现、聚焦不显形的操作（§16 键盘用户够不到）', acc.a4);
  show('A5 命中区小于 24px（登记）', acc.a5);
  show('A6 吸顶条用 1px 硬线而无 scroll-edge 遮罩（登记）', acc.a6);

  if (failed.length) {
    console.log(`\n✗ 有页面没量到（失败 ${failed.length}）—— 上面所有清单与"0 处"都不作数：${failed.slice(0, 3).join(' | ')}`);
    process.exitCode = 1;
  }
  const gate = Object.keys(acc.a1).length + Object.keys(acc.a3).length + Object.keys(acc.a4).length
    + perPage.filter((p) => p.mats.length > 1).length;
  console.log(`\napple 门禁合计 ${gate} 处（A1+A3+A4+同页混档材质）；A5/A6 是登记项，不计入门禁`);
  fs.writeFileSync(path.join(ROOT, 'apple-design.json'), JSON.stringify({ perPage, acc }, null, 1));
  console.log('明细已写入 web/apple-design.json');
  if (gate) process.exitCode = 1;
};

// 记分卡：把十台工装的产物按页合并，回答「哪几页有问题」
const table = () => {
  const nodeRequire = createRequire(import.meta.url);
  const read = (f) => { try { return nodeRequire(path.join(ROOT, f)); } catch { console.log(`  （读不到 ${f}，先跑对应工装）`); return null; } };
  const rows = new Map();
  const touch = (name) => { if (!name) return null; if (!rows.has(name)) rows.set(name, {}); return rows.get(name); };

  const ui = read('ui-consistency.json');
  if (Array.isArray(ui)) {
    for (const p of ui) {
      const r = touch(p.name);
      if (!r) continue;
      if (p.errored) r.页面崩 = 'Y';
      if (p.crumb === false) r.缺面包屑 = 'Y';
      if (p.footer === false) r.缺页脚 = 'Y';
      const h1s = [...new Set((p.h1 || []).map((x) => x.size))];
      if (h1s.length > 1) r.h1多档 = h1s.join('/');
      const st = [...new Set((p.stat || []).map((x) => `${x.size}/${x.weight}`))];
      if (st.length > 1) r.数值多档 = st.join(' ');
    }
  }
  // layout-geometry 里 rowHeights/rowCenters/cardHeights 是**分布登记**不是违例，别算进记分卡
  const VIOL = ['clipped', 'wrapped', 'gaps', 'tableAlign', 'overflowX', 'clippedBy', 'coveredBy'];
  const CODE = { clipped: 'R3', wrapped: 'R4', gaps: 'R5', tableAlign: 'R6', overflowX: 'R7', clippedBy: 'R8', coveredBy: 'R9' };
  const geo = read('layout-geometry.json');
  if (Array.isArray(geo)) {
    for (const p of geo) {
      const r = touch(p.name);
      if (!r) continue;
      if (p.errored) r.页面崩 = 'Y';
      for (const k of VIOL) if ((p[k] || []).length) r[CODE[k]] = p[k].length;
      for (const k of ['rowHeights', 'rowCenters']) {
        const list = p[k] || [];
        const off = list.filter((x) => x && (x.diff ?? 0) > 2 || x?.off).length;
        if (off) r[CODE[k] || k] = off;
      }
    }
  }
  const par = read('page-parity.json');
  if (par) {
    for (const p of par.rows || []) { const r = touch(p.name); if (r && p.crashed) r.页面崩 = 'Y'; }
    for (const o of par.report || []) { const r = touch(o.name || o.page); if (r) r.版面离群 = (r.版面离群 || 0) + 1; }
  }
  const mo = read('motion-consistency.json');
  if (mo) {
    // dur/ease 是**分布登记**（每个类见过哪些时长），不是违例 —— 违例在打印时才现算，
    // 没落盘。把它们当记账项会让 73 页全部"有罪"。只有 press / m3 是违例桶。
    const MCODE = { press: '按下无反馈', m3: '减动效未降级' };
    for (const [axis, bucket] of Object.entries(mo)) {
      if (!MCODE[axis] || !bucket || typeof bucket !== 'object') continue;
      for (const v of Object.values(bucket)) {
        for (const pg of (v?.pages || [])) { const r = touch(pg); if (r) r[MCODE[axis]] = (r[MCODE[axis]] || 0) + 1; }
      }
    }
  }
  const vs = read('visual-signature.json');
  if (vs) for (const f of vs.findings || []) for (const pg of f.pages || []) { const r = touch(pg); if (r) r.签名离群 = (r.签名离群 || 0) + 1; }
  const ap = read('apple-design.json');
  if (ap?.perPage) for (const p of ap.perPage) {
    const r = touch(p.name);
    if (!r) continue;
    if (p.a1) r.A1动布局属性 = p.a1;
    if (p.a1b) r.A1b全接管 = p.a1b;
    if (p.a3) r.A3叠半透明 = p.a3;
    if (p.a4) r.A4仅hover = p.a4;
    if (p.mats.length > 1) r.A2混材质 = p.mats.join('|');
    if (p.a5) r.A5命中区小 = p.a5;
  }
  const bx = read('box-census.json');
  if (bx) {
    const BCODE = { b1: '控件离档', b2: '盒模型漂移', b3: '左边缘差', b4: '间距不落尺', b6: '跑出视口', b7: '混字段形态', b8: 'label不落线' };
    for (const axis of Object.keys(BCODE)) {
      for (const v of Object.values(bx[axis] || {})) {
        for (const pg of (v?.pages || [])) { const r = touch(pg); if (r) r[BCODE[axis]] = (r[BCODE[axis]] || 0) + 1; }
      }
    }
  }
  const dirty = [...rows.entries()].filter(([, r]) => Object.keys(r).length);
  const keys = [...new Set(dirty.flatMap(([, r]) => Object.keys(r)))];
  console.log(`记分卡：${rows.size} 页有条目，其中 ${dirty.length} 页有记账项。列 = ${keys.join(' ')}`);
  for (const [name, r] of dirty.sort((a, b) => Object.keys(b[1]).length - Object.keys(a[1]).length)) {
    console.log(`   ${name.padEnd(28)} ` + Object.entries(r).map(([k, v]) => `${k}=${v}`).join(' ').slice(0, 150));
  }
  const clean = [...rows.keys()].filter((n) => !rows.get(n).length);
  console.log(`\n零记账项的页 ${clean.length} 个：${clean.slice(0, 44).join(', ')}${clean.length > 44 ? ' …' : ''}`);
};

if (argv.includes('--selftest')) selftest();
else if (argv.includes('--table')) table();
else run();
