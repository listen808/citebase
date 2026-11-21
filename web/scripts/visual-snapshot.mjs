// 前端视觉 UI 快照测试：登录后遍历全部路由，逐页全页截图 + 采集控制台/网络异常。
// 用法（前端 dev server 需已在 :3000 运行，后端 :3001 需在运行）：
//   node web/scripts/visual-snapshot.mjs [--themes=light,dark] [--only=<name子串>]
import { chromium } from 'playwright';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { findChromium } from './lib/chromium-path.mjs';
import { fillFixtures, resolveFixtures } from './lib/route-list.mjs';
import { parseThemeTarget, themeStorageEntries } from './lib/theme-target.mjs';
import { loginForm } from './lib/page-state.mjs';

const BASE = process.env.SNAP_BASE || 'http://localhost:3000';
// 口令只从环境变量取，不给默认值：这个脚本会被提交进仓库，
// 写死账号口令等于把凭据推到远端。缺变量就直接退出，让人去设。
const USER = process.env.SNAP_USER;
const PASS = process.env.SNAP_PASS;
if (!USER || !PASS) {
  console.error('需要 SNAP_USER / SNAP_PASS 环境变量（例：SNAP_USER=xxx SNAP_PASS=yyy node web/scripts/visual-snapshot.mjs）');
  process.exit(2);
}
// 产物路径按脚本位置算，不按 cwd：原先按 process.cwd() 解析，在 web/ 下跑就把 4.7MB 截图
// 写进了 web/docs/ui-snapshots（.gitignore 只挡仓库根那份），git 看到的是未忽略的垃圾目录。
const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const OUT_ROOT = path.join(ROOT, 'docs', 'ui-snapshots');

const argv = Object.fromEntries(process.argv.slice(2).map((a) => {
  const [k, v = ''] = a.replace(/^--/, '').split('=');
  return [k, v];
}));
const THEME_TARGETS = [];
for (const spec of (argv.themes || 'light').split(',').filter(Boolean)) {
  // 外观 × 材质两条轴。认不出来的规格直接退出而不是跳过：少刷一套主题、
  // 却在报表里按"跑过了"打印，是最容易放过去的一种假结果。
  const t = parseThemeTarget(spec);
  if (!t.ok) {
    process.stdout.write(`✗ --themes 里的「${spec}」无法解析：${t.error}\n`);
    process.exit(1);
  }
  THEME_TARGETS.push(t);
}
const LIGHT_TARGET = parseThemeTarget('light');
const ONLY = argv.only || '';

// AI 设置 31 个子页（与 AISettingsLayout.vue 的 groups 保持一致）
const AI_SECTIONS = [
  'overview', 'analysis', 'search-overview',
  'model', 'rag', 'prompts',
  'schema', 'knowledge-base', 'import', 'auto-tag', 'sql-history', 'sql-audit-trace',
  'sql-example', 'advanced-ask',
  'mcp', 'workflows', 'widget', 'write', 'wiki-compile', 'wiki-write', 'wiki-governance',
  'observability', 'token', 'operations', 'sql-security', 'ragas-eval',
  'data-sensitivity', 'eval-console', 'capability-health', 'memory',
  'ab-experiment',
];

// auth: false = 无需登录
// __SPACE__ / __DOC__ / __USER__ / __ROLE__ 由 scripts/lib/route-list.mjs 现场解析：
// 这里以前写死 356 与 9，一轮清库之后它们成了 404，而巡检把这三种页报成"没有可见页头"——
// 被测对象不存在时，量出来的一切都不是设计问题。
const ROUTES = [
  // ── 公开页 ──
  { path: '/login', name: 'login', auth: false, public: true },
  { path: '/author-login', name: 'author-login', auth: false, public: true },
  { path: '/install', name: 'install', auth: false, public: true },
  { path: '/auth/sso/callback', name: 'sso-callback', auth: false, public: true },
  { path: '/share/__DOC__', name: 'share', auth: false, public: true },
  { path: '/error', name: 'error-page', auth: false, public: true },
  // ── 主站 ──
  { path: '/', name: 'home' },
  { path: '/search', name: 'search' },
  { path: '/space/__SPACE__', name: 'space' },
  { path: '/space/__SPACE__/doc/__DOC__', name: 'document' },
  { path: '/space/__SPACE__/doc/__DOC__/edit', name: 'document-edit' },
  // P2-18：原 /space/:id/views 已并入独立页 /doc-views（router 无兼容重定向），
  // 旧路径留在清单里只会让巡检"跑过=没问题"。
  { path: '/doc-views', name: 'document-views' },
  { path: '/collections', name: 'collections' },
  { path: '/follow', name: 'follow' },
  { path: '/activities', name: 'activities' },
  { path: '/notifications', name: 'notifications' },
  { path: '/saved-searches', name: 'saved-searches' },
  { path: '/webhooks', name: 'webhooks' },
  { path: '/tags', name: 'tags' },
  { path: '/templates', name: 'templates' },
  { path: '/doc-collab', name: 'doc-collab' },
  { path: '/profile', name: 'profile' },
  { path: '/user/__USER__', name: 'user-profile' },
  { path: '/about', name: 'about' },
  // ── AI ──
  { path: '/ai/workspace', name: 'ai-workspace' },
  { path: '/ai/graph', name: 'ai-graph' },
  ...AI_SECTIONS.map((s) => ({ path: `/ai/settings?section=${s}`, name: `ai-settings__${s}` })),
  // ── 管理后台 ──
  { path: '/dashboard', name: 'dashboard' },
  { path: '/users', name: 'users' },
  { path: '/roles', name: 'roles' },
  { path: '/roles/__ROLE__/users', name: 'role-users' },
  { path: '/privileges', name: 'privileges' },
  { path: '/spaces', name: 'spaces' },
  { path: '/logs', name: 'logs' },
  { path: '/links', name: 'links' },
  { path: '/contacts', name: 'contacts' },
  { path: '/media', name: 'media' },
  { path: '/settings', name: 'settings' },
  { path: '/settings/email', name: 'settings-email' },
  { path: '/settings/sso', name: 'settings-sso' },
  { path: '/settings/sso/doc', name: 'settings-sso-doc' },
  { path: '/settings/oidc', name: 'settings-oidc' },
  { path: '/settings/api-keys', name: 'settings-api-keys' },
  { path: '/plugins', name: 'plugins' },
  { path: '/search-monitor', name: 'search-monitor' },
  { path: '/backups', name: 'backups' },
  { path: '/data-export', name: 'data-export' },
  { path: '/search-ops', name: 'search-ops' },
];

// 主题切换需要已登录，且 5 套主题都要过一遍的核心页（占位符同 ROUTES，由夹具解析）
const THEME_PAGES = ['/', '/space/__SPACE__', '/ai/workspace', '/dashboard', '/profile', '/settings'];

const themeSlug = (t) => t.replace(/[^a-z0-9-]/gi, '');

// dev server 会注入 Vue DevTools 浮层（画面底部那条 "V ◎"）。它不是页面缺陷，
// 但会跟着截图一路进 README —— 公开仓库里印出开发工装，而且没人会注意到。
// 所以出图前一律摘掉并断言摘干净：摘不掉就当工装故障抛错，不要产出一张带浮层的图。
async function stripDevOverlays(page) {
  return page.evaluate(() => {
    let removed = 0;
    const el = document.getElementById('__vue-devtools-container__');
    if (el) { el.remove(); removed++; }
    document.querySelectorAll('vue-devtools, __vue-devtools__app__, vite-error-overlay').forEach((e) => { e.remove(); removed++; });
    return { removed, still: !!document.getElementById('__vue-devtools-container__') };
  });
}

// Windows 上目标 PNG 可能正被资源管理器预览 / 索引 / 杀软握着，写入直接抛
// UNKNOWN: unknown error —— 那是工装故障不是页面故障，别让它冒充成页面缺陷。
async function shotWithRetry(page, file, attempts = 3) {
  const dev = await stripDevOverlays(page);
  if (dev.still) throw new Error(`${file}：dev 浮层没摘干净，拒绝出图`);
  for (let i = 1; ; i++) {
    try {
      await page.screenshot({ path: file, fullPage: true });
      return;
    } catch (e) {
      if (i >= attempts || !/unknown error|EBUSY|EPERM/.test(String(e))) throw e;
      await page.waitForTimeout(400 * i);
    }
  }
}

// 本机 chromium 定位逻辑与 check-ui-consistency.mjs 共用一份实现

async function settle(page, prevUrl) {
  // SPA 懒加载 chunk：等 URL 稳定 + 网络静默（WebSocket 会挂着，所以按请求数收敛而非 networkidle）
  const deadline = Date.now() + 12000;
  let stable = 0;
  while (Date.now() < deadline) {
    await page.waitForTimeout(250);
    const url = page.url();
    if (url === prevUrl) stable += 1; else { stable = 0; prevUrl = url; }
    if (stable >= 2) break;
  }
  await page.waitForTimeout(400);
}

async function setTheme(page, target) {
  // 主题偏好持久化在 localStorage（storage.ts 的 'citebase:' 前缀），写 key 后重载最稳。
  // 四个键一律显式写：外观 × 材质是两条轴，只写 theme 的话上一轮的 glass 会一路跟到下一轮，
  // 而"这张是 solid 的截图"和"这张其实是被上一轮材质污染的截图"在文件名上分不出来。
  await page.evaluate((entries) => {
    try {
      for (const [k, v] of entries) localStorage.setItem(k, v);
    } catch { /* ignore */ }
  }, themeStorageEntries(target)).catch(() => {});
}

// 页面"活着"不等于"排版对"：按钮文字竖排、控件三种高度、同行卡片一高一低，
// 全都以 ok 通过。这里补一组几何断言，把本轮手工修过的问题变成每次自动拦。
const LAYOUT_AUDIT = () => {
  const ALLOWED_H = [24, 28, 32, 40, 44];
  const near = (h) => ALLOWED_H.some((a) => Math.abs(h - a) <= 1);
  const out = { textOverflow: [], offLadder: [], unequalCards: [], clippedTable: [], docOverflow: 0, escapedAbsolute: [], shellBars: [], shellClipped: [] };

  out.docOverflow = document.documentElement.scrollWidth - document.documentElement.clientWidth;

  for (const e of document.querySelectorAll('button,[role=tab],[role=combobox],a,span,div')) {
    const r = e.getBoundingClientRect();
    if (!r.height || r.width < 12) continue;
    const cs = getComputedStyle(e);
    if (cs.display === 'none' || cs.visibility === 'hidden') continue;
    const label = (e.textContent || e.getAttribute('aria-label') || '').trim().slice(0, 10);
    if (!label) continue;
    if ((e.tagName === 'BUTTON' || e.getAttribute('role') === 'tab') && e.scrollWidth > Math.ceil(r.width) + 1) {
      out.textOverflow.push(`${label} ${e.scrollWidth}>${Math.round(r.width)}`);
    }
    if ((e.tagName === 'BUTTON' || e.tagName === 'INPUT' || e.getAttribute('role') === 'combobox') && cs.position !== 'fixed' && !near(Math.round(r.height))) {
      out.offLadder.push(`${e.tagName}:${label} h${Math.round(r.height)}`);
    }
  }

  for (const g of document.querySelectorAll('div,section')) {
    const cs = getComputedStyle(g);
    if (cs.display !== 'grid' || g.children.length < 2) continue;
    const rows = {};
    for (const k of g.children) {
      const r = k.getBoundingClientRect();
      if (r.height < 40 || r.width < 40) continue;
      // 只比"卡片"：有描边或面板底色的才算。主从布局里侧栏与正文高度不同是正常的，
      // 之前把 200px 侧栏(818) 和正文(444) 当成"同行卡片不等高"报了 33 页假阳性。
      const kcs = getComputedStyle(k);
      const isCard = (kcs.borderTopWidth !== '0px' && kcs.borderTopStyle !== 'none')
        || /surface-panel/.test(kcs.backgroundColor);
      if (!isCard) continue;
      (rows[Math.round(r.top)] = rows[Math.round(r.top)] || []).push(Math.round(r.height));
    }
    for (const hs of Object.values(rows)) {
      if (hs.length > 1 && Math.max(...hs) - Math.min(...hs) > 12) {
        out.unequalCards.push(`${Math.max(...hs)}/${Math.min(...hs)}`);
        break;
      }
    }
  }

  for (const t of document.querySelectorAll('table')) {
    let anc = t.parentElement, sc = null;
    while (anc) {
      const cs = getComputedStyle(anc);
      if (cs.overflowX === 'auto' || cs.overflowX === 'scroll') { sc = anc; break; }
      anc = anc.parentElement;
    }
    const limit = sc ? sc.clientWidth : (t.parentElement ? t.parentElement.clientWidth : 1e9);
    if (t.scrollWidth > limit + 4 && !sc) out.clippedTable.push(`t${t.scrollWidth}>${limit}`);
  }

  for (const e of document.querySelectorAll('div,span')) {
    const cls = String(e.className || '');
    if (/vue-devtools/.test(cls)) continue;
    // Monaco 的视图层（.view-lines / 光标 / 装饰）天生就是绝对定位叠层，
    // 它的内部排布不属于我们的布局契约，量它只会造出假阳性。
    if (e.closest('.monaco-editor')) continue;
    const cs = getComputedStyle(e);
    if (cs.position !== 'absolute') continue;
    const r = e.getBoundingClientRect();
    if (r.width < 8 || r.height < 8) continue;
    let anc = e.parentElement, f = null;
    while (anc) { if (getComputedStyle(anc).position !== 'static') { f = anc; break; } anc = anc.parentElement; }
    if (!f) continue;
    const ar = f.getBoundingClientRect();
    if (r.left < ar.left - 16 || r.right > ar.right + 16 || r.top < ar.top - 16 || r.bottom > ar.bottom + 16) {
      out.escapedAbsolute.push(`${(e.textContent || '').trim().slice(0, 8)}@${Math.round(r.left)},${Math.round(r.top)}`);
    }
  }

  // 壳层三条横边实测同高：顶栏、页脚、侧栏底部用户行。
  // 静态门禁（R14）只保证"三处引用同一把尺"，量不到被 flex 压扁、被边框加高、
  // 被页面自己覆盖这些情况——历史上 60/47/49 就是三个各自凑数攒出来的。
  // 期望值取自 --apple-shell-bar-height（与 shellBarHeight 同源），所以标尺改动这里不用跟着改。
  const SHELL_BARS = { nav: '顶栏', footer: '页脚', 'user-row': '用户行' };
  const want = Math.round(parseFloat(getComputedStyle(document.documentElement).getPropertyValue('--apple-shell-bar-height')));
  // 变量被改名或删掉时 parseFloat 给 NaN，而 Math.abs(h - NaN) > 1 恒为 false ——
  // 三条边就全都"通过"。这是最安静的一种假绿，必须显式量一次期望值本身。
  if (!Number.isFinite(want) || want <= 0) {
    out.shellBars.push('--apple-shell-bar-height 取不到数值（变量被改名或缺失），同高断言无法判定');
  }
  // 一条都没量到 = 要么探针跑在壳层挂载之前，要么三个锚点全被改名。
  // 静默返回会让这组断言永远绿（实测 /settings 在 settle=1.8s 下就是这个状态），所以显式报。
  if (!document.querySelector('[data-ap-shell]') && document.querySelector('main')) {
    out.shellBars.push('整页无 data-ap-shell 锚点（壳层未挂载或锚点全被改名）');
  }
  // 地标在、锚点没了 = 有人改了 data-ap-shell 的值：下面的逐条比对会一条都量不到，
  // 门禁于是"全绿"，但那是不检查结果造成的绿。
  for (const [sel, anchor, cn] of [['header', 'nav', '顶栏'], ['footer', 'footer', '页脚']]) {
    if (document.querySelector(sel) && !document.querySelector(`[data-ap-shell="${anchor}"]`)) {
      out.shellBars.push(`${cn}地标在但 data-ap-shell="${anchor}" 锚点丢失`);
    }
  }
  // 逐条各比各的：移动端抽屉收着、公开页走 BlankLayout 时本来就凑不齐三条，
  // 若要求"三条都在"才判等，这些页面就永远量不到。
  for (const [key, cn] of Object.entries(SHELL_BARS)) {
    const bar = document.querySelector(`[data-ap-shell="${key}"]`);
    if (!bar) continue;
    const br = bar.getBoundingClientRect();
    const h = Math.round(br.height);
    if (!h) continue; // 这条边本页没渲染（display:none），量不到就等于不该判
    if (Math.abs(h - want) > 1) out.shellBars.push(`${cn}${h}≠标尺${want}`);
    // 定高把内容压住时，越界的是子元素而不是盒子自己：align-items:center 的 flex 子项
    // 向上也会溢出，而 scrollHeight 只累计下/右侧，量不到 —— 所以直接比子元素矩形与条矩形。
    for (const kid of bar.querySelectorAll('*')) {
      const cs = getComputedStyle(kid);
      if (cs.display === 'contents' || cs.position === 'absolute' || cs.position === 'fixed') continue;
      const kr = kid.getBoundingClientRect();
      if (!kr.width || !kr.height) continue;
      if (kr.top < br.top - 1 || kr.bottom > br.bottom + 1) {
        out.shellClipped.push(`${cn}内${Math.round(kr.height)}越出${h}高的条`);
        break;
      }
    }
  }

  for (const k of ['textOverflow', 'offLadder', 'unequalCards', 'clippedTable', 'escapedAbsolute', 'shellBars', 'shellClipped']) {
    out[k] = [...new Set(out[k])].slice(0, 6);
  }
  return out;
};

const layoutIssues = (l) => !l
  ? 0
  : l.textOverflow.length + l.offLadder.length + l.unequalCards.length
    + l.clippedTable.length + l.escapedAbsolute.length + (l.shellBars || []).length
    + (l.shellClipped || []).length + (l.docOverflow > 4 ? 1 : 0);

async function capture(page, route, theme, results) {
  // 文件名只用路由名：早先带序号，而序号是按路由清单位置算的——清单里插一个新页，
  // 后面每一页都换名字，旧图留在原地变成"看着像基线其实过期"的孤儿。
  const rel = `${themeSlug(theme)}/${route.name}.png`;
  const file = path.join(OUT_ROOT, rel);
  fs.mkdirSync(path.dirname(file), { recursive: true });

  const consoleErrors = [];
  const pageErrors = [];
  const badResponses = [];
  const onConsole = (m) => { if (m.type() === 'error') consoleErrors.push(m.text().slice(0, 300)); };
  const onPageError = (e) => pageErrors.push(String(e).slice(0, 300));
  const onResponse = (r) => {
    const u = r.url();
    if (r.status() >= 400 && !u.includes('favicon') && !u.includes('/api/v1/collab/ws')) {
      badResponses.push(`${r.status()} ${u.replace(BASE, '').slice(0, 160)}`);
    }
  };
  page.on('console', onConsole);
  page.on('pageerror', onPageError);
  page.on('response', onResponse);

  const requested = route.path;
  let finalUrl = '';
  let textLen = 0;
  let err = '';
  let layout = null;
  // 逐页之间留一点间隔：连打 78 页会把后端限流器打满（实测 14 个端点回 429），
  // 截图拍到的是降级/空态而不是页面本身，且 auth/me 429 会让守卫弹回登录页。
  await page.waitForTimeout(Number(process.env.SNAP_GAP_MS || 700));
  try {
    await page.goto(BASE + requested, { waitUntil: 'load', timeout: 25000 });
    await settle(page, requested);
    // 挂载闸门贴在测量前面：壳层没渲染就量，三条边查无元素、断言静默通过。
    // 每条路由都必有 ApFooter（公开页走 BlankLayout 也带 pinned 页脚），所以等它出现是通用判据。
    await page.waitForFunction(() => !!document.querySelector('[data-ap-shell]'), { timeout: 8000 }).catch(() => {});
    finalUrl = new URL(page.url()).pathname + new URL(page.url()).search;
    textLen = await page.evaluate(() => (document.body.innerText || '').trim().length);
    layout = await page.evaluate(LAYOUT_AUDIT);
    await shotWithRetry(page, file);
  } catch (e) {
    err = String(e).slice(0, 300);
    try { await shotWithRetry(page, file, 1); } catch { /* ignore */ }
  }

  page.off('console', onConsole);
  page.off('pageerror', onPageError);
  page.off('response', onResponse);

  const redirected = !err && finalUrl !== requested;
  const nIssues = layoutIssues(layout);
  results.push({
    name: route.name, requested, finalUrl, theme, screenshot: `docs/ui-snapshots/${rel}`,
    blank: !err && textLen < 30, textLen, redirected, error: err, layoutIssues: nIssues, layout,
    consoleErrors: [...new Set(consoleErrors)].slice(0, 8),
    pageErrors: [...new Set(pageErrors)].slice(0, 8),
    badResponses: [...new Set(badResponses)].slice(0, 10),
  });
  const flag = err ? 'ERR' : redirected ? 'REDIR' : results.at(-1).blank ? 'BLANK' : nIssues ? `LAYOUT(${nIssues})` : 'ok';
  process.stdout.write(`${flag.padEnd(11)} ${theme}/${route.name}\n`);
}

const run = async () => {
  fs.mkdirSync(OUT_ROOT, { recursive: true });
  const browser = await chromium.launch({ executablePath: findChromium() });
  const ctx = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 1,
    locale: 'zh-CN',
  });
  const page = await ctx.newPage();
  // beforeunload 必须 accept：dismiss 的语义是「留在本页」，会把这次导航整个中止，
  // 于是从文档编辑页之后每一跳都变成 net::ERR_ABORTED（实测 89/100 页因此作废）。
  // 其余对话框（confirm/alert）仍按 dismiss 处理，避免误触发删除等动作。
  page.on('dialog', (d) => (d.type() === 'beforeunload' ? d.accept() : d.dismiss()).catch(() => {}));

  const results = [];
  // 夹具在开机时就解析一次（公开页里的 /share/__DOC__ 也要用到），后面三处都用解析后的表
  const TARGETS = fillFixtures(ROUTES, await resolveFixtures());
  const onlyMatch = (r) => !ONLY || r.name.includes(ONLY);

  // 1) 公开页（未登录）
  for (const r of TARGETS.filter((r) => r.public)) {
    if (onlyMatch(r)) await capture(page, r, 'light', results);
  }

  // 2) 登录
  await loginForm(page, BASE, USER, PASS);
  const loggedIn = !page.url().includes('/login');
  process.stdout.write(`\nlogin: ${loggedIn ? 'ok' : 'FAILED'} (${page.url()})\n\n`);

  // 3) 全部受保护路由（默认主题 = light + solid）
  for (const r of TARGETS.filter((r) => !r.public)) {
    if (onlyMatch(r)) await capture(page, r, LIGHT_TARGET.label, results);
  }

  // 4) 其它外观/材质组合抽检核心页
  for (const target of THEME_TARGETS.filter((t) => t.label !== LIGHT_TARGET.label)) {
    await setTheme(page, target);
    for (const p of THEME_PAGES) {
      const r = TARGETS.find((x) => x.path === p) || { path: p, name: p.replace(/\W+/g, '-') };
      if (onlyMatch(r)) await capture(page, r, target.label, results);
    }
  }
  await setTheme(page, LIGHT_TARGET);

  await browser.close();
  fs.writeFileSync(path.join(OUT_ROOT, 'report.json'), JSON.stringify(results, null, 2), 'utf8');
  const bad = results.filter((x) => x.layoutIssues);
  const tally = {};
  for (const x of bad) {
    if (!x.layout) continue;
    if (x.layout.textOverflow.length) tally.textOverflow = (tally.textOverflow || 0) + 1;
    if (x.layout.offLadder.length) tally.offLadder = (tally.offLadder || 0) + 1;
    if (x.layout.unequalCards.length) tally.unequalCards = (tally.unequalCards || 0) + 1;
    if (x.layout.clippedTable.length) tally.clippedTable = (tally.clippedTable || 0) + 1;
    if (x.layout.escapedAbsolute.length) tally.escapedAbsolute = (tally.escapedAbsolute || 0) + 1;
    if (x.layout.docOverflow > 4) tally.docOverflow = (tally.docOverflow || 0) + 1;
  }
  process.stdout.write(`\n共 ${results.length} 张快照 → ${OUT_ROOT}\n`);
  process.stdout.write(`硬故障: ERR ${results.filter((x) => x.error).length} / REDIR ${results.filter((x) => x.redirected).length} / BLANK ${results.filter((x) => x.blank).length}\n`);
  process.stdout.write(`排版可疑页 ${bad.length} 张: ${JSON.stringify(tally)}\n`);
  for (const x of bad.slice(0, 25)) {
    process.stdout.write(`  ${x.theme}/${x.name}: ${JSON.stringify(x.layout).replace(/[{}"]/g, '').slice(0, 150)}\n`);
  }
};

run().catch((e) => { console.error(e); process.exit(1); });
