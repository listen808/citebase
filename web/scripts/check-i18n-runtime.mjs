// 运行时 i18n 巡检：以 en-US 档访问路由表里的每一条，量"用户看得见的中文"还剩多少。
//
// 为什么源码级清单（i18n-inventory.mjs）不够：它数的是**源码里的形状**，
// 而用户看见的中文有三类它数不到 ——
//   ① 运行时算出来的（日期、Intl、随机问候语）；
//   ② 后端返回的数据（空间名、文档标题）—— 这些**本来就不该翻**；
//   ③ 消息语法没转义时界面上印出的 `{'@'}` 这类碎片。
// 所以这台工具的职责不是替代源码清单，而是给出"英文用户实际看到什么"，
// 并把 ② 排除掉：只有**公开路由**（无需登录、无用户数据）才设硬门槛。
//
// 用法（前端 dev server 需在 :3000，后端 :3001）：
//   SNAP_USER=xxx SNAP_PASS=yyy node scripts/check-i18n-runtime.mjs [--budget=40] [--only=/ai] [--dump]
// 口令只从环境变量取，理由与 visual-snapshot.mjs 相同：脚本会进仓库。
//
// --dump（以及默认输出的 code/data 拆分）解决"红了没法行动"：可见中文里绝大多数是
// 用户自己写的数据（空间名、文档标题、用户名），它们**本来就不该翻**；判据是机械的 ——
// 访问这条路由时拦下所有 /api/ 响应体，节点文本若原样出现在响应体里就是数据，
// 否则才是代码产的文案。以前这一步靠人眼一条条比对（临时写了个 probe-cjk.mjs），
// 而人眼会在 51 条路由上骗自己。
import { chromium } from 'playwright';
import { findChromium } from './lib/chromium-path.mjs';
import { resolveFixtures } from './lib/route-list.mjs';

const BASE = process.env.SNAP_BASE || 'http://localhost:3000';
const USER = process.env.SNAP_USER;
const PASS = process.env.SNAP_PASS;
if (!USER || !PASS) {
  console.error('需要 SNAP_USER / SNAP_PASS 环境变量');
  process.exit(2);
}
const argv = Object.fromEntries(process.argv.slice(2).map((a) => {
  const [k, v = ''] = a.replace(/^--/, '').split('=');
  return [k, v];
}));
const BUDGET = Number(argv.budget ?? 40);
// 代码文案（不是后端数据）在英文界面出现中文 = 待译。这一档实测归零，所以默认就是 0，
// 加一条硬拦：谁再往模板里写死中文，只要它出现在某条被巡检的路由上就红。
const CODE_BUDGET = Number(argv['code-budget'] ?? argv.codeBudget ?? 0);
// Git Bash 会把 --only=/dashboard 改写成 C:/Program Files/Git/dashboard（MSYS 的路径转换），
// 于是"筛一条路由"变成"筛出 0 条 = 全绿"。这里剥掉那层前缀并补回前导斜杠；
// 筛完为空另有下文一条硬拦，两者是不同的失败面。
const ONLY = '/' + String(argv.only || '').replace(/^.*\/Git\//, '').replace(/^\/+/, '');
const DUMP = 'dump' in argv;
// 还有一类既不是文案也不是用户数据：**代码里的语言数据**。同义词词库就是词典本身，
// 检索运维页把它显示出来是在管理词典，翻译它等于改检索行为
// （与 i18n-inventory 那条"数据值 vs 显示值"的三型判断同源）。
// 判据不许静默放行：命中的串照样印出来，只是不计入待译预算。
const CODE_DATA_PHRASES = new Set([
  '空间 / 目录 / 文件夹 / 区域', // utils/domain/search-enhanced.ts 的同义词族
]);
const CJK = /[\u3400-\u9fff]/;
// 消息语法碎片与未解析插值：这两类是"翻译做坏了"，比没翻译更糟。
const BROKEN = /\{'@'\}|\{'\{'|\{'\}'|\{\{\s*\w+\s*\}\}\}|(?<![{\w])\{[a-zA-Z_]\w*\}(?![}\w])/;

const browser = await chromium.launch({
  executablePath: findChromium(),
  args: ['--no-sandbox', '--disable-dev-shm-usage'],
});
const ctx = await browser.newContext({ viewport: { width: 1600, height: 1000 } });
const page = await ctx.newPage();

async function login() {
  await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' });
  await page.fill('input[autocomplete="username"]', USER);
  await page.fill('input[autocomplete="current-password"]', PASS);
  // 按位置点而不是按名字点：这台工具跑在 en-US 档，按"登录"这个中文名找按钮，
  // 翻译一落地选择器就自己失效了（选择器依赖被测文本 = 假阴性）。
  await page.locator('form button').last().click();
  await page.waitForURL((u) => !u.pathname.includes('login'), { timeout: 20000 });
  await page.waitForTimeout(1200);
}

await page.goto(BASE, { waitUntil: 'domcontentloaded' });
await page.evaluate(() => localStorage.setItem('app-locale', 'en-US'));
await login();

// 路由表从**跑着的应用**里取，不另抄一份清单：抄的那份一定会和 router 漂移，
// 而漂移的表现是"巡检全绿、新页面整片中文"。
const paths = await page.evaluate(() => {
  // 挂载点是 #app-root（main.ts），不是惯常的 #app —— 找错元素的表现是"路由表为空"，
  // 而不是报错，所以这里显式两个候选并在拿不到时退出，绝不静默跑成"0 条路由 = 全绿"。
  const el = document.querySelector('#app-root') || document.querySelector('#app');
  const app = el?.__vue_app__;
  const router = app?.config?.globalProperties?.$router;
  if (!router) return [];
  const seen = new Set();
  const out = [];
  for (const r of router.getRoutes()) {
    if (r.path.includes(':')) continue; // 参数路由用样例值访问，见下
    if (seen.has(r.path)) continue;
    seen.add(r.path);
    out.push({ path: r.path, isPublic: r.meta?.public === true });
  }
  // 参数路由（router 里带 :id 的那些）不在这张表里，用现场解析出来的夹具补三条：
  // 写死 /document/1 的旧写法在清过库的机器上量的是 404 页，那不是文档页。
  return out;
});

const fx = await resolveFixtures();
paths.push(
  { path: `/space/${fx.space}`, isPublic: false },
  { path: `/space/${fx.space}/doc/${fx.doc}`, isPublic: false },
  { path: `/share/${fx.doc}`, isPublic: true },
);

if (paths.length === 0) {
  console.error('拿不到路由表（$router 未挂载？）');
  await browser.close();
  process.exit(2);
}

const rows = [];
// 每次导航重装一次"这页从后端拿到的字符串"，用于区分代码文案与用户数据。
let apiValues = new Set();
let apiRaw = [];

// JSON 里取叶子字符串值：判"这条文本是数据"要的是**整串相等**，
// 而不是"在某处出现过" —— 子串判据会把 '用户' 这类既出现在数据里、又出现在代码文案里的
// 短词洗成数据，等于把门禁读瞎。解析失败的响应（流式、非 JSON）退化成子串判据。
function collectStrings(value, out) {
  if (typeof value === 'string') {
    if (CJK.test(value)) out.add(value.trim());
    return;
  }
  if (Array.isArray(value)) {
    for (const v of value) collectStrings(v, out);
    return;
  }
  if (value && typeof value === 'object') {
    for (const v of Object.values(value)) collectStrings(v, out);
  }
}

page.on('response', async (res) => {
  const url = res.url();
  if (!url.includes('/api/')) return;
  const type = res.headers()['content-type'] || '';
  if (!type.includes('json') && !type.startsWith('text/')) return;
  let body;
  try {
    body = await res.text();
  } catch {
    return; // 导航把响应体丢了：宁可少判一条数据，也不要巡检中途炸掉
  }
  if (body.length > 2_000_000) return;
  try {
    collectStrings(JSON.parse(body), apiValues);
  } catch {
    apiRaw.push(body);
  }
});

const targets = paths.filter((x) => x.path.startsWith(ONLY || '/'));
if (DUMP) console.log('路由表: ' + paths.map((x) => x.path).join(' '));
if (targets.length === 0) {
  // --only 打错一个字母就会筛出 0 条路由，而"0 条"在这台工具里天然等于全绿。
  // 上面已经拦了"路由表本身为空"，这里拦的是"过滤后为空"——同一个假绿，另一个入口。
  console.error(`--only=${ONLY} 没匹配到任何路由（共 ${paths.length} 条），拒绝以"全绿"收场`);
  await browser.close();
  process.exit(2);
}

for (const p of targets) {
  apiValues = new Set();
  apiRaw = [];
  await page.goto(BASE + p.path, { waitUntil: 'domcontentloaded' }).catch(() => {});
  await page.waitForTimeout(700);
  /**
   * 用户数据有**两条来路**：这一趟的 /api/ 响应体，以及浏览器自己的缓存。
   * 最近文档、空间列表这类 store 会落 localStorage，首屏直接由缓存渲染 —— 于是
   * "整串相等"永远等不到，用户写的标题被判成代码文案。实测首页三张文档卡片
   * （集客 / 个人工作台 / 上线流程）就是这个形状，而它们确实不是前端写的字。
   * 这里只多接一条**来路**，不放宽任何口径：仍然走同一个 collectStrings，
   * 仍然按整串相等判，所以"代码文案恰好是某条数据的子串"那种洗白路径没有打开。
   */
  const cached = await page.evaluate(() => {
    const out = [];
    for (const store of [window.localStorage, window.sessionStorage]) {
      for (let i = 0; i < store.length; i++) {
        const k = store.key(i);
        const v = k ? store.getItem(k) : null;
        if (v && /[\u3400-\u9fff\uf900-\ufaff]/.test(v)) out.push(v);
      }
    }
    return out;
  }).catch(() => []);
  for (const blob of cached) {
    try {
      collectStrings(JSON.parse(blob), apiValues);
    } catch {
      apiRaw.push(blob);
    }
  }
  // 取**文本节点**而不是 innerText：整页 innerText 的中文数里分不清谁产的串，
  // 而按节点看，"这条是代码文案"与"这条是后端给的数据"才有可判的边界。
  const texts = await page.evaluate(() => {
    const out = [];
    const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
    for (let n = walker.nextNode(); n; n = walker.nextNode()) {
      const v = (n.nodeValue || '').trim();
      if (v) out.push(v);
    }
    return out;
  }).catch(() => []);
  // 响应体可能在读文本之后才落地，再等一拍并复用同一批节点文本重判一次。
  await page.waitForTimeout(150);
  const fromApi = (text) => apiValues.has(text) || apiRaw.some((b) => b.includes(text));
  // 混合节点（"管理员 created 3 documents"）：先把落在 API 值里的那段摘掉，再看余下还有没有中文。
  // 只做整串相等会把这类误判成代码文案，而它其实是数据嵌进了本地化句子里。
  const leftOverIsCode = (text) => {
    let rest = text;
    for (const v of apiValues) {
      if (v.length > 1 && rest.includes(v)) rest = rest.split(v).join('');
    }
    return CJK.test(rest);
  };
  const cjkTexts = texts.filter((t) => CJK.test(t));
  // 单字节点且它正是某条数据的第一个字：那是头像占位字母（'管理员' → '管'），不是代码文案。
  // 判据只收单字，多字仍按代码算 —— 宁可窄，也不要把手上的单位词（'条'）洗白。
  const isDataInitial = (text) => text.length === 1 && [...apiValues].some((v) => v[0] === text);
  const codeData = cjkTexts.filter((t) => CODE_DATA_PHRASES.has(t));
  const code = cjkTexts.filter((t) => !fromApi(t) && !isDataInitial(t) && leftOverIsCode(t) && !CODE_DATA_PHRASES.has(t));
  const dataCount = cjkTexts.length - code.length - codeData.length;
  const cjk = cjkTexts.reduce((n, t) => n + (t.match(new RegExp(CJK.source), 'g') || []).length, 0);
  const pageText = cjkTexts.join('\n');
  const broken = pageText.match(new RegExp(BROKEN.source, 'g')) || [];
  const undefinedish = /\bundefined\b|\bNaN\b/.test(pageText);
  rows.push({ path: p.path, isPublic: p.isPublic, cjk, code, codeData, data: dataCount, broken: [...new Set(broken)], undefinedish });
}
await browser.close();

rows.sort((a, b) => b.cjk - a.cjk);
const allCode = new Set(rows.flatMap((r) => r.code));
const hard = rows.filter((r) => r.broken.length > 0 || r.undefinedish);
// 总量门槛只管"公开且无后端数据"的页：登录/安装这类页出现中文必然是代码文案。
// 用户数据页（文档详情、文档列表）中文多是因为用户写了中文，不是待译 —— 报告里
// /doc-views 与 /space/:id/doc/:id 就栽在这里。公开分享页虽标 public，但渲染的是文档数据，
// 靠 data 数量一并排除。代码文案由下面的 CODE_BUDGET 独立硬拦，两档互不替代。
const over = rows.filter((r) => r.isPublic && r.data === 0 && r.cjk > BUDGET);
const codeOver = rows.filter((r) => r.code.length > CODE_BUDGET);
console.log(`路由 ${rows.length} 条 · 总量门槛 仅公开无数据页 ≤${BUDGET}；代码文案 全路由 ≤${CODE_BUDGET}`);
// 门槛仍按"可见中文总量"，因为那条基线是历史量出来的；真正能行动的是这一行：
// 代码文案（去掉跨页重复的串）才是待译清单，其余是用户自己写的数据。
console.log(`代码文案 ${rows.reduce((n, r) => n + r.code.length, 0)} 处 · 跨页去重 ${allCode.size} 条 · 其余为后端数据`);
// 词典那一类照样要看得见，只是不计入待译预算
const dictPhrases = [...new Set(rows.flatMap((r) => r.codeData))];
if (dictPhrases.length > 0) console.log(`语言数据（词典本身，翻译它等于改检索行为）：${dictPhrases.join(' | ')}`);
for (const r of rows.slice(0, 18)) {
  console.log(
    `  ${String(r.cjk).padStart(5)}  ${r.path}  代码 ${r.code.length} / 数据 ${r.data}` +
      `${r.broken.length ? '  ⚠ 碎片 ' + r.broken.join(' ') : ''}${r.undefinedish ? '  ⚠ undefined/NaN' : ''}`,
  );
}
if (DUMP) {
  // 只有 code 会被印出来：数据不值得人看（那是用户自己写的字）。
  for (const r of rows.filter((x) => x.code.length > 0)) {
    console.log(`\n── ${r.path} (${r.code.length} 条代码文案)`);
    for (const t of [...new Set(r.code)].slice(0, 40)) console.log(`   ${t.replace(/\s+/g, ' ').slice(0, 90)}`);
  }
}
if (hard.length > 0) {
  console.error(`✗ ${hard.length} 条路由把消息语法或 undefined 印在了界面上：`);
  hard.forEach((r) => console.error(`   ${r.path}: ${r.broken.join(' ')}${r.undefinedish ? ' undefined/NaN' : ''}`));
  process.exit(1);
}
if (over.length > 0) {
  console.error(`✗ ${over.length} 条公开无数据路由可见中文超过 ${BUDGET}（最多 5 条）：`);
  over.slice(0, 5).forEach((r) => console.error(`   ${r.path} = ${r.cjk}`));
  process.exit(1);
}
if (codeOver.length > 0) {
  console.error(`✗ ${codeOver.length} 条路由渲染出代码产的中文字符串（每页门槛 ${CODE_BUDGET}，跨页去重 ${allCode.size} 条）：`);
  for (const r of codeOver.slice(0, 8)) console.error(`   ${r.path}: ${[...new Set(r.code)].slice(0, 6).join(' / ')}`);
  console.error('  这些不是后端给的数据，是写死在模板或脚本里的中文 —— 加 --dump=1 看全表，或 --code-budget=N 临时放宽（会留下待还的债）');
  process.exit(1);
}
console.log('✓ 运行时 i18n 巡检通过');
