// 运行时错误巡检：九台设计门禁查的是"排版/材质/动效是否一致"，查不到"这一页跑起来会不会炸"。
// 这台专捕现有工装盲区里的四类**运行时错误**，逐页登录态过一遍：
//   R1 未捕获异常（page.on('pageerror')）—— Vue 渲染抛错、undefined 解引用等，最硬的 bug。
//   R2 控制台 error（page.on('console', type==='error')）—— 含 Vue warn 升级为 error 的、三方库报错。
//   R3 网络失败（page.on('requestfailed')）—— 请求根本没发出去/被拦（CORS、DNS、aborted 除外见降噪）。
//   R4 非 2xx/3xx 响应（page.on('response')）—— 5xx 计入门禁；4xx 只登记（预认证 401 探测是合法的）。
// 门禁 = R1 + R2 + R3 + 5xx；warn 与 4xx 是登记项，需要人工分真伪，不一刀切判红。
// 用法（dev :3000、后端 :3001 在跑；口令只走环境变量，绝不写进仓库）：
//   SNAP_USER=… SNAP_PASS=… node scripts/check-runtime-errors.mjs [--only=<路由名子串>] [--max=20] [--settle=1200]
import { chromium } from 'playwright';
import path from 'node:path';
import fs from 'node:fs';
import { fileURLToPath } from 'node:url';
import { findChromium } from './lib/chromium-path.mjs';
import { pickTargets } from './lib/route-list.mjs';
import { loginForm } from './lib/page-state.mjs';

const BASE = process.env.SNAP_BASE || 'http://localhost:3000';
const ROOT = path.join(path.dirname(fileURLToPath(import.meta.url)), '..');
const HERE = path.join(ROOT, 'scripts');
const argv = process.argv.slice(2);
const argOf = (k) => { const m = argv.find((a) => a.startsWith(`--${k}=`)); return m && m.split('=')[1]; };
const ONLY = argOf('only');
const MAX = Number(argOf('max') || 20);
const SETTLE = Number(argOf('settle') || 1200);
const GAP = Number(argOf('gap') || 2500);
const SELFTEST = argv.includes('--selftest');

const login = async (page, USER, PASS) => {
  await loginForm(page, BASE, USER, PASS);
};

// 降噪：这些是环境/框架的已知无害噪声，不算 bug（但要能看见，故仍打印总数）。
const IGNORE_CONSOLE = [
  /ResizeObserver loop/i,
  /Download the Vue Devtools/i,
  /\[vite\]/i,
  /favicon/i,
  /Third-party cookie/i,
  /prefers-reduced-motion/i,
  // 无人势的自动导航下 Chrome 会拦 beforeunload 弹层并记一条 error——真实用户必有手势，属工装假象。
  /beforeunload.*user gesture|Blocked attempt to show a 'beforeunload'/i,
];
const IGNORE_REQUEST = [
  /favicon/i,
  /\/(apple-touch-icon|manifest)/i,
  /\.ico(\?|$)/i,
];
const ignored = (list, s) => list.some((re) => re.test(s));

const run = async () => {
  const USER = process.env.SNAP_USER;
  const PASS = process.env.SNAP_PASS;
  if (!USER || !PASS) { console.error('需要 SNAP_USER / SNAP_PASS（口令只走环境变量，不写进仓库）'); process.exit(2); }

  const TARGETS = await pickTargets(ONLY, HERE, { matchPath: true });
  const browser = await chromium.launch({ executablePath: findChromium() });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  const page = await ctx.newPage();

  const r1 = {}, r2 = {}, r3 = {}, r5xx = {}, r4xx = {}, r2b = {};
  const note = (dst, key) => { dst[key] = (dst[key] || 0) + 1; };
  const first = (s) => (s || '').split('\n')[0].trim().slice(0, 160);
  // 这台工装自己就是限流的主要制造者：73 页 × 每页八九个请求，300/分钟/IP 一定被穿过。
  // 把它算进 R2（"应用的控制台错误"）的表现是这台永远红 —— 而一直红的门禁等于没有门禁。
  // 所以 429 相关的降为登记项 R2b，不静默丢弃；限流口径本身另有 TestLimiterTiers
  // 与前端 429 呈现的处理测试在管，不靠这台兜。
  const RATE_LIMIT = /429|Too Many Requests|rate_limit|请求过于频繁/i;
  let cur = 'boot';

  page.on('pageerror', (e) => { if (cur !== 'login') note(r1, `${cur} | ${first(e.message || String(e))}`); });
  page.on('console', (m) => {
    if (cur === 'login') return;
    const t = m.type(), txt = m.text();
    if (ignored(IGNORE_CONSOLE, txt)) return;
    if (t !== 'error') return;
    note(RATE_LIMIT.test(txt) ? r2b : r2, `${cur} | ${first(txt)}`);
  });
  page.on('requestfailed', (req) => {
    if (cur === 'login') return;
    const u = req.url();
    if (ignored(IGNORE_REQUEST, u)) return;
    const f = req.failure()?.errorText || 'failed';
    if (/aborted/i.test(f)) return; // 页面切换取消在途请求，属正常
    note(r3, `${cur} | ${f} ${first(u)}`);
  });
  page.on('response', (res) => {
    if (cur === 'login') return;
    const s = res.status(), u = res.url();
    if (ignored(IGNORE_REQUEST, u)) return;
    if (s >= 500) note(r5xx, `${cur} | ${s} ${first(u)}`);
    else if (s >= 400) note(r4xx, `${cur} | ${s} ${first(u)}`);
  });

  await login(page, USER, PASS);

  // 反向验证：先自造一条未捕获异常 + 一条 console.error，确认探针不是瞎的（否则"0 处"不算数）。
  if (SELFTEST) {
    cur = 'selftest';
    await page.goto(`${BASE}/dashboard`, { waitUntil: 'load' });
    await page.evaluate((b) => {
      console.error('SELFTEST_R2_console_error');
      setTimeout(() => { throw new Error('SELFTEST_R1_uncaught'); }, 10);
      fetch(`${b.replace(/^http/, 'ws')}/selftest-bad-origin`).catch(() => {});
    }, BASE);
    await page.waitForTimeout(800);
    const gotR1 = Object.keys(r1).some((k) => k.includes('SELFTEST_R1'));
    const gotR2 = Object.keys(r2).some((k) => k.includes('SELFTEST_R2'));
    console.log(`${gotR1 ? '✓' : '✗'} R1 抓到自造未捕获异常（pageerror）`);
    console.log(`${gotR2 ? '✓' : '✗'} R2 抓到自造 console.error`);
    // R3（requestfailed）/R5（4xx）/R4（5xx）已接线，但同源环境难以自造干净的违例，故不在此自测。
    await browser.close();
    const blind = !gotR1 || !gotR2;
    console.log(blind ? '\n崩溃级判据（R1/R2）没全抓到自造违例 —— 这台是瞎的' : '\n崩溃级判据（R1/R2）都能抓到自造违例');
    process.exitCode = blind ? 1 : 0;
    return;
  }

  let visited = 0;
  const failedNav = [];
  // 本机 socket 层面的偶发错（ERR_NO_BUFFER_SPACE 就是 Windows 直说"缓冲用完了"，
  // 快速 localhost 爬取的典型产物；ERR_CONNECTION_TIMED_OUT 同一类）。
  // 实测过：/api/v1/ai/config 直连 11ms、走 vite 代理 219ms —— 慢的不是应用，是爬取自己。
  // 处理办法不是豁免掉，而是**整轮爬完之后单独复访那些页**：安静状态下仍复现才算违例。
  // （放在"刚量完这一页"那一刻判是无效的：请求的失败通常落在页间 GAP 里，那时已经换页了。）
  const TRANSPORT = /ERR_NO_BUFFER_SPACE|ERR_CONNECTION_RESET|ERR_NETWORK_CHANGED|ERR_EMPTY_RESPONSE|ERR_ADDRESS_UNREACHABLE|ERR_INSUFFICIENT_RESOURCES|ERR_[A-Z_]*TIMED_OUT|Network Error|ERR_NETWORK|ECONNREFUSED|ECONNRESET|socket hang up/i;
  const transportPages = () => [...new Set(
    [...Object.keys(r2), ...Object.keys(r3)]
      .filter((k) => TRANSPORT.test(k))
      .map((k) => k.split(' | ')[0]),
  )];
  const dropTransport = (names) => {
    for (const bucket of [r2, r3]) {
      for (const k of Object.keys(bucket)) {
        if (TRANSPORT.test(k) && names.includes(k.split(' | ')[0])) delete bucket[k];
      }
    }
  };

  for (const t of TARGETS) {
    cur = t.name;
    try {
      await page.goto(`${BASE}${t.path}`, { waitUntil: 'load', timeout: 30000 });
      await page.waitForTimeout(SETTLE);
      visited++;
    } catch (e) {
      failedNav.push(`${t.name}: ${first(String(e.message || e))}`);
    }
    // 限速：apiLimiter 300/分钟/IP、refreshLimiter 30/分钟/IP，逐页突发会自造 429 风暴（噪声）。
    // 页间留 gap 让滑动窗口回落到真实用户节奏，剩下的才是真 bug。
    await page.waitForTimeout(GAP);
  }

  const flaky = transportPages();
  if (flaky.length) {
    console.log(`↻ ${flaky.length} 页出现本机传输错（socket 耗尽那一类），安静复访一次再判：${flaky.join(', ')}`);
    dropTransport(flaky);
    await new Promise((res) => setTimeout(res, 5000));
    for (const name of flaky) {
      const t = TARGETS.find((x) => x.name === name);
      if (!t) continue;
      cur = name;
      try {
        await page.goto(`${BASE}${t.path}`, { waitUntil: 'load', timeout: 30000 });
        await page.waitForTimeout(SETTLE + 2000);
      } catch (e) {
        failedNav.push(`${name}（传输错复访）: ${first(String(e.message || e))}`);
      }
    }
  }

  await browser.close();

  const show = (title, dst, hard) => {
    const keys = Object.keys(dst);
    console.log(`\n== ${title}：${keys.length} 处${hard ? '（计入门禁）' : '（登记，需分真伪）'} ==`);
    for (const k of keys.slice(0, MAX)) console.log(`   ${k}${dst[k] > 1 ? ` ×${dst[k]}` : ''}`);
    if (keys.length > MAX) console.log(`   …另 ${keys.length - MAX} 处`);
  };
  show('R1 未捕获异常（pageerror）', r1, true);
  show('R2 控制台 error', r2, true);
  show('R2b 控制台 error（限流相关，多为工装自己打出来的）', r2b, false);
  show('R3 网络请求失败', r3, true);
  show('R4 服务端 5xx', r5xx, true);
  show('R5 客户端 4xx', r4xx, false);
  if (failedNav.length) console.log(`\n✗ 有页面没量到（导航失败 ${failedNav.length}）—— 上面"0 处"不作数：\n   ${failedNav.slice(0, 5).join('\n   ')}`);

  const gate = Object.keys(r1).length + Object.keys(r2).length + Object.keys(r3).length + Object.keys(r5xx).length;
  console.log(`\n量测页面 = ${visited} / ${TARGETS.length}，视口 1440`);
  console.log(`运行时门禁合计 ${gate} 处（R1+R2+R3+5xx）；4xx 与限流控制台（R2b）为登记项，不计入门禁`);
  const out = { visited, total: TARGETS.length, r1, r2, r2b, r3, r5xx, r4xx, failedNav, gate };
  fs.writeFileSync(path.join(ROOT, 'runtime-errors.json'), JSON.stringify(out, null, 2));
  console.log('明细已写入 web/runtime-errors.json');
  process.exitCode = gate ? 1 : 0;
};

run().catch((e) => { console.error('巡检自身崩了：', e); process.exit(2); });
