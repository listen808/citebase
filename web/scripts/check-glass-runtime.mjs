#!/usr/bin/env node
/**
 * 玻璃层运行时普查 —— 用法：
 *   SNAP_USER=… SNAP_PASS=… npm run check:glass:runtime [-- --pages=/,/users --shots]
 *
 * 静态普查（check:glass）数的是**声明站点**：哪几个文件写了 backdrop-filter、走没走令牌、
 * 登没登记。它回答不了运行时才有的三个问题，而这三个恰恰是"看着没问题"的重灾区：
 *   1. 一屏**同时**叠了几层实时模糊（基线 MAX_BLUR_LAYERS=16。声明站点 17 处不等于同屏 17 层，
 *      也不等于 2 层 —— v-for 出来的卡片会把一处声明放大成二十层，2026-10-15 实测过 23 层）；
 *   2. 有没有玻璃压玻璃（嵌套深度 >0。静态上两个选择器谁套谁看不出来）；
 *   3. 滚动边缘效应真的在触发吗（内容滚到浮层壳底下时壳有没有加厚）。
 * 所以这台工装直接开浏览器量渲染结果，并且每轮截图存证。
 *
 * 没登录凭据时它起不来（loginForm 需要 SNAP_USER/SNAP_PASS）—— 这是故意的：
 * 量登录后的页面却拿一个"登录失败兜底页"当样本，报表会绿得毫无意义。
 */
import { chromium } from 'playwright';
import fs from 'node:fs';
import path from 'node:path';
import { findChromium } from './lib/chromium-path.mjs';
import { loginForm } from './lib/page-state.mjs';
import { parseThemeTarget, themeStorageEntries } from './lib/theme-target.mjs';

const BASE = process.env.SNAP_BASE || 'http://localhost:3000';
const USER = process.env.SNAP_USER;
const PASS = process.env.SNAP_PASS;
const argv = process.argv.slice(2);
const pagesArg = (argv.find((a) => a.startsWith('--pages=')) || '--pages=/,/users,/spaces').slice(8);
const PAGES = pagesArg.split(',').map((s) => s.trim()).filter(Boolean);
// 默认跑**四档玻璃轮**而不是原来那一档：这台只判玻璃，而玻璃有 light/dark × std/clear 四个形态，
// 少跑一档 = 那一档的同屏层数与嵌套从没被看过（报表上看不出来）。
const THEMES = (argv.find((a) => a.startsWith('--themes=')) || '--themes=light+glass,light+glass+clear,dark+glass,dark+glass+clear').slice(9).split(',');
const SHOTS = argv.includes('--shots');
const OUT = path.resolve('.glass-runtime');

/** 红线：同屏实时模糊层数。2026-10-26 用户拍板「按钮也是液态玻璃」——控件档（ApButton
 * 半透明变体 + ghost 图标钮）回归后流水面数量随页面放大，≤3 那条红线作废；改为**实测基线钉住**：
 * 两轮实测（26 页，图标钮玻璃化后再测 10 页），典型 8~11 层、最大 13（/ai/workspace），
 * 基线钉 16（最大值 + 3 层余量）——防再次暴涨，而不是防按钮本身。 */
const MAX_BLUR_LAYERS = 16;
/** 嵌套红线：控件档回归后，壳层（顶栏/侧栏）玻璃上的按钮天然形成一层嵌套，
 * 这是拍板接受的形态；两层以上的「玻璃压玻璃」仍然判红。 */
const MAX_NESTING = 1;

const MEASURE = () => {
  const isBlur = (e) => {
    const b = getComputedStyle(e).backdropFilter;
    return b && b !== 'none';
  };
  // 浏览器装了 Vue devtools 扩展时，它的面板自己带 backdrop-filter，会被算进"同屏层数"
  // —— 那不是应用画的玻璃（实测首页因此多报 1 层）。按类名排除，报表才等于应用自己铺的层。
  const clsOf = (e) => (typeof e.className === 'string' ? e.className : '');
  const isDevtools = (e) => /vue-devtools/.test(clsOf(e)) || !!e.closest('.vue-devtools__panel');
  const all = [...document.querySelectorAll('*')].filter((e) => isBlur(e) && !isDevtools(e));
  const depthOf = (e) => {
    let d = 0;
    let p = e.parentElement;
    while (p) {
      if (isBlur(p)) d++;
      p = p.parentElement;
    }
    return d;
  };
  const nav = document.querySelector('[data-ap="nav-bar"]');
  const sc = document.querySelector('[data-ap-scroll]');
  /**
   * 凹陷档可见性（语法表 F1 的判据，2026-11-01）：`surface-field` 是"压进去"的面，
   * 它必须比**自己所在的那块板**明显暗（或亮）一点，否则读成贴上去的另一块板 ——
   * /space/1 文档树里那个搜索框实测就是 field 0.18 白压在 chrome 0.24 上，只亮不凹。
   * 判据：|Δ亮度| ≥ 12（亮度按审计用的加权 RGB，0-255 单位，与剖面数字同一口径）。
   * 令牌值取解析后的实际颜色（color-mix 在自定义属性里是原文，量不出来的）。
   */
  const probe = document.createElement('div');
  document.body.appendChild(probe);
  probe.style.backgroundColor = 'var(--apple-surface-field)';
  const fieldCss = getComputedStyle(probe).backgroundColor;
  probe.remove();
  const parse = (c) => {
    const m = String(c).match(/rgba?\(([^)]+)\)/);
    if (!m) return c === 'transparent' ? [0, 0, 0, 0] : [0, 0, 0, 1];
    const p = m[1].split(',').map(Number);
    return [p[0], p[1], p[2], p.length > 3 ? p[3] : 1];
  };
  const over = (fg, bg) => [0, 1, 2].map((i) => fg[3] * fg[i] + (1 - fg[3]) * bg[i]);
  const lum = (c) => 0.2126 * c[0] + 0.7152 * c[1] + 0.0722 * c[2];
  let worstDent = null;
  for (const el of document.querySelectorAll('body *')) {
    const cs = getComputedStyle(el);
    if (cs.backgroundColor !== fieldCss) continue;
    const r = el.getBoundingClientRect();
    if (r.width < 40 || r.height < 16 || r.bottom < 0 || r.top > innerHeight) continue;
    const chain = [];
    for (let p = el.parentElement; p && p !== document.documentElement; p = p.parentElement) chain.unshift(p);
    let plate = [8, 10, 22];
    for (const p of chain) {
      const c = parse(getComputedStyle(p).backgroundColor);
      if (c[3] > 0) plate = over(c, plate);
    }
    const delta = Math.abs(lum(over(parse(fieldCss), plate)) - lum(plate));
    // 凹陷有两种画法：底色差，或描边/内阴影。轨道这类面坐在哪块板上是未知的（canvas 本来就暗），
    // 只许第一种就会逼着作者去调一个"在哪都刚好"的数 —— 那不存在，所以两种都认。
    const bordered = parseFloat(cs.borderTopWidth) >= 0.5 || /inset/.test(cs.boxShadow || '');
    if (bordered && delta < 12) continue;
    if (worstDent === null || delta < worstDent.delta) {
      worstDent = { delta: Math.round(delta), id: `${el.tagName.toLowerCase()}.${String(el.className).split(' ')[0]}` };
    }
  }
  return {
    blurLayers: all.length,
    maxNesting: all.length ? Math.max(...all.map(depthOf)) : 0,
    navEdge: nav ? nav.getAttribute('data-scroll-edge') : null,
    navBg: nav ? getComputedStyle(nav).backgroundColor : null,
    scrollable: sc ? sc.scrollHeight > sc.clientHeight : false,
    worstDent,
  };
};

if (!USER || !PASS) {
  console.error('✗ 需要 SNAP_USER / SNAP_PASS：量登录后的页面却拿登录失败兜底页当样本，报表会绿得毫无意义');
  console.error('  本机跑法（前端 :3000 与后端 :3001 都得在跑）：');
  console.error('    SNAP_USER=<账号> SNAP_PASS=<密码> npm run check:glass:runtime');
  console.error('  这台工装**不在 CI**：它要登录态与真实服务，CI 两样都没有。');
  console.error('  所以改任何玻璃材质之后，必须有人在本地跑一次 —— 静态门禁数的是声明站点，看不见同屏层数。');
  process.exit(1);
}

fs.rmSync(OUT, { recursive: true, force: true });
if (SHOTS) fs.mkdirSync(OUT, { recursive: true });

const browser = await chromium.launch({ executablePath: findChromium() });
const errors = [];
const rows = [];
try {
  for (const spec of THEMES) {
    const target = parseThemeTarget(spec);
    if (!target.ok) { console.error(`✗ 主题规格不认：${target.error}`); process.exit(1); }
    const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const page = await ctx.newPage();
    // themeStorageEntries 返回的键**已经自带前缀**（'citebase:ui:theme'）。
    // 这里以前写的是 localStorage.setItem('citebase:' + kk, vv)，拼成 citebase:citebase:ui:theme ——
    // 于是每一轮都在应用默认档上量，报表却按 --themes 标成 light+glass+clear：一台假绿的运行时工装。
    for (const [k, v] of themeStorageEntries(target)) await page.addInitScript(([kk, vv]) => localStorage.setItem(kk, vv), [k, v]);
    await page.goto(BASE + '/login', { waitUntil: 'domcontentloaded' });
    await loginForm(page, BASE, USER, PASS);
    for (const route of PAGES) {
      await page.goto(BASE + route, { waitUntil: 'domcontentloaded' });
      await page.waitForTimeout(900);
      const atTop = await page.evaluate(MEASURE);
      let under = null;
      if (atTop.scrollable) {
        await page.evaluate(() => { document.querySelector('[data-ap-scroll]').scrollTop = 320; });
        await page.waitForTimeout(300);
        under = await page.evaluate(MEASURE);
        await page.evaluate(() => { document.querySelector('[data-ap-scroll]').scrollTop = 0; });
        await page.waitForTimeout(200);
      }
      if (atTop.blurLayers > MAX_BLUR_LAYERS) {
        errors.push(`${spec} ${route} 同屏 ${atTop.blurLayers} 层实时模糊 > 红线 ${MAX_BLUR_LAYERS}`);
      }
      if (atTop.maxNesting > MAX_NESTING) errors.push(`${spec} ${route} 玻璃嵌套深度 ${atTop.maxNesting}（红线 ≤${MAX_NESTING}）`);
      if (under && under.navEdge !== 'under') errors.push(`${spec} ${route} 内容已滚到顶栏底下但壳没加厚（data-scroll-edge=${under.navEdge}）`);
      if (atTop.scrollable && atTop.navEdge !== 'free') errors.push(`${spec} ${route} 页面顶部壳却报 under（探针初值错）`);
      if (atTop.worstDent && atTop.worstDent.delta < 12) {
        errors.push(`${spec} ${route} 凹陷档读不出来：${atTop.worstDent.id} 与它所在那块板只差 ${atTop.worstDent.delta} 个亮度单位（判据 ≥12）`);
      }
      rows.push(`${spec.padEnd(16)} ${route.padEnd(14)} 层数 ${String(atTop.blurLayers).padStart(2)} 嵌套 ${atTop.maxNesting} 壳 ${atTop.navEdge}→${under ? under.navEdge : '-'}`);
      if (SHOTS) {
        await page.evaluate(() => { const sc = document.querySelector('[data-ap-scroll]'); if (sc) sc.scrollTop = 240; });
        await page.waitForTimeout(250);
        await page.screenshot({ path: path.join(OUT, `${spec.replace(/\W/g, '-')}${route.replace(/\W/g, '-') || '-home'}.png`) });
        await page.evaluate(() => { const sc = document.querySelector('[data-ap-scroll]'); if (sc) sc.scrollTop = 0; });
      }
    }
    await ctx.close();
  }
} finally {
  await browser.close();
}

console.log(rows.join('\n'));
if (SHOTS) console.log(`\n截图在 ${OUT}`);
if (errors.length) {
  console.error(`\n✗ 玻璃运行时普查未通过（${errors.length} 项）：`);
  for (const e of errors) console.error('  ' + e);
  process.exit(1);
}
console.log(`\nℹ 玻璃运行时普查：${rows.length} 轮量测，同屏模糊层数均 ≤${MAX_BLUR_LAYERS}、嵌套 ≤${MAX_NESTING}、滚动边缘双向触发`);
