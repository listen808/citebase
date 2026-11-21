// 长内容抗压巡检：把页面上真实存在的短文案（标题 / 卡片题 / 表格单元 / 按钮 / 链接）
// 逐段换成超长文本，然后当场量布局有没有被撑坏，量完立刻还原。
// 为什么单独做一台：check:geom 那十条断言量的是**当前库里正好不长的内容**。
// "标题够长会不会顶破卡片""按钮文案一长就折成两行""单元格里一个长 URL 把整页拉出横向滚动"
// ——这些缺陷只有在内容变长时才现形，而内容什么时候变长不由前端决定（用户起的文档名、
// 别人贴的链接）。所以这台跑的是同一批几何判据，只是把内容当成"一定会变长"来压。
//   S1 撑破：注入后出现横向溢出（documentElement.scrollWidth > 视口）
//   S2 顶出组件盒：被压的文本越过它最近的 [data-ap] 组件边界
//   S3 控件文案折行：按钮/链接里的标签被压成两行及以上
//   S4 截断无提示（登记表）：压出 ellipsis 的节点自身和祖先都没有 title / aria-label
//      —— 记下来不当门禁：真内容未必有这么长，但"截断处没有全名"这件事值得有一张表。
// 注入只在一次 evaluate 里完成（改→量→还原，全程同步），Vue 来不及插手重渲染。
// 用法（前端 dev :3000、后端 :3001 需在跑；口令只走环境变量）：
//   SNAP_USER=… SNAP_PASS=… node scripts/check-long-content.mjs [--only=<路由名子串>] [--max=15]
//   node scripts/check-long-content.mjs --selftest   # 反向验证：S1/S2/S3 必须各自抓到违例
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
const MAX = Number(argOf('max') || 15);
const TARGETS = await pickTargets(ONLY, here);
const PER_PAGE = Number(process.env.STRESS_NODES || 26);

// 两种长度都要压：CJK 能任意折行（考的是行数和组件盒），
// 无空格的拉丁长串（URL / 哈希 / 英文长词）不能折行（考的是 overflow-wrap）。
const CJK = '重命名与批量归档的交互回归验证覆盖全站所有弹层与表格单元以及侧栏导航条目若干条记录';
const LATIN = 'https://citebase.example.internal/documents/very-long-slug-abcdefghij0123456789';

const STRESS = (cfg) => {
  const main = document.querySelector('main') || document.body;
  const isProse = (el) => !!el.closest('article, .markdown-body, pre, code, [data-prose], [data-ap-markdown]');
  const words = (el) => {
    let n = 0;
    for (const node of el.childNodes) {
      if (node.nodeType === 3) n += (node.textContent.match(/[^\s，,、；;：:/|]+/g) || []).length;
      else if (node.nodeType === 1 && !/^(SCRIPT|STYLE)$/.test(node.tagName)) n += words(node);
    }
    return n;
  };
  const vis = (el) => { const r = el.getBoundingClientRect(); return r.width > 4 && r.height > 4 && r.top < innerHeight && r.bottom > 0; };

  // 只挑"现在很短"的题/单元/按钮/链接：它们正是真内容一变长就出事的那批
  const SEL = 'h1,h2,h3,h4,td,th,a,button,span[class],div[class]';
  const marks = [];
  let k = 0;
  for (const el of main.querySelectorAll(SEL)) {
    if (marks.length >= cfg.per) break;
    if (isProse(el) || !vis(el)) continue;
    if (el.matches('td,th') && el.cellIndex === 0) continue;
    if (el.querySelectorAll('h1,h2,h3,h4,td,th,a,button').length) continue;
    const t = (el.textContent || '').trim();
    if (!t || t.length > 24) continue;
    if (words(el) < 1 || words(el) > 12) continue;
    el.setAttribute('data-zzs', String(k));
    marks.push({ id: String(k), tag: el.tagName.toLowerCase(), owner: el.closest('[data-ap]')?.getAttribute('data-ap') || '无', text: t });
    k++;
  }

  const out = { boxLost: [], wrapped: [], noHint: [] };
  for (const round of [cfg.cjk, cfg.latin]) {
    const victims = [];
    for (const m of marks) {
      const el = main.querySelector(`[data-zzs="${m.id}"]`);
      if (!el) continue;
      // 只压文本：改第一个文本节点，保住结构不被破坏
      const tn = [...el.childNodes].find((n) => n.nodeType === 3 && n.textContent.trim());
      if (!tn) continue;
      victims.push({ el, tn, m, orig: tn.textContent });
      tn.textContent = round + round.slice(0, 12);
    }
    for (const { el, tn, m, orig } of victims) {
      // 行数与墨迹都只量**那个文本节点**：拿整元素的 Range 会把图标 svg、文字前后的
      // 其它内联盒各算成一个矩形，于是 nowrap 的按钮也被数成"两行"（实测 1500 条假阳性）。
      const range = document.createRange();
      range.selectNodeContents(tn);
      const rects = [...range.getClientRects()];
      const host = el.closest('[data-ap]');
      if (host && rects.length) {
        // 量的是"字的墨迹"越过组件盒，不是元素盒：nowrap 的标题元素盒永远等于父宽，
        // 真正越界的只有文字本身 —— 只比元素盒的话这条断言就是死的。
        const hr = host.getBoundingClientRect();
        const inkRight = Math.max(...rects.map((x) => x.right));
        const inkLeft = Math.min(...rects.map((x) => x.left));
        const lost = Math.max(inkRight - hr.right, hr.left - inkLeft, 0);
        // 从自己往上走到组件盒为止，途中遇到任何横向裁切/滚动容器都算"它自己消化宽度"
        // （表格里 4911px 那种就是横向滚动容器在吸收，check:geom 的 R7 同口径）。
        let exempt = getComputedStyle(el).textOverflow === 'ellipsis';
        for (let n = el; n && !exempt; n = n.parentElement) {
          const o = getComputedStyle(n).overflowX;
          if (o === 'hidden' || o === 'clip' || o === 'auto' || o === 'scroll') exempt = true;
          if (n === host) break;
        }
        if (lost > 2 && !exempt) out.boxLost.push(`${m.tag}@${m.owner} 「${m.text}」越界 ${Math.round(lost)}px`);
      }
      const lines = new Set(rects.map((x) => Math.round(x.top))).size;
      if (el.matches('a,button') && lines > 1) {
        out.wrapped.push(`${m.tag}@${m.owner} 「${m.text}」压成 ${lines} 行`);
      }
      if (el.scrollWidth > el.clientWidth + 2) {
        const hinted = el.hasAttribute('title') || !!el.closest('[title]') || el.hasAttribute('aria-label');
        if (!hinted) out.noHint.push(`${m.tag}@${m.owner}`);
      }
      tn.textContent = orig;
    }
  }
  for (const el of main.querySelectorAll('[data-zzs]')) el.removeAttribute('data-zzs');
  out.docOverflow = document.documentElement.scrollWidth - document.documentElement.clientWidth;
  out.sampled = marks.length;
  return out;
};

const uniq = (a) => [...new Set(a)];

// 已知违例：文案是源码写死的标签，唯一现实的变长来源是 i18n，而英文长度仍在盒内。
// 列在这里是**待复核的债**，不是免罪金牌 —— 门禁只拦这份清单之外的新违例
// （与静态门禁的 BASELINE 同一口径）。要加条目必须同时写清"为什么不可能变长"。
const KNOWN = [
  { page: 'ai-workspace', cls: 'button@button', text: '新对话',
    why: 'ChatSidebar 的固定标签；按钮 203px，zh 3 字 / en "New conversation" 15 字符都在盒内' },
  { page: 'space', cls: 'button@page-container', text: '复制',
    why: 'DocContentPanel 划词菜单 / SpaceModals 分享里源码写死的 "复制"（2 字固定标签，i18n 也不会更长），真内容不会变长' },
];
const isKnown = (row) => KNOWN.some((k) => row.startsWith(`${k.page} `) && row.includes(k.cls) && row.includes(`「${k.text}」`));

const run = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  page.on('dialog', (d) => (d.type() === 'beforeunload' ? d.accept() : d.dismiss()).catch(() => {}));
  await loginForm(page, BASE, USER, PASS);

  const cfg = { cjk: CJK, latin: LATIN, per: PER_PAGE };
  const s1 = [];
  const s2 = [];
  const s3 = [];
  const s4 = {};
  let pages = 0;
  let sampled = 0;
  const failed = [];

  // 浮层那一趟是补的盲区：⌘K 面板"按下才存在"，页面侧那趟 STRESS 只看得到打开页面时就
  // 渲染出来的东西。而结果行、表单几乎全住在浮层里 —— 不补这一趟，
  // "73 页 S1/S3 全 0"读起来就像"全站长内容全 0"，其实只是"73 页初始态全 0"。
  // 只跑前 SPOTLIGHT_PAGES 页：每跑一页会发一次真实搜索请求，全站跑会把限流打满。
  const SPOTLIGHT_PAGES = Number(process.env.STRESS_SPOTLIGHT_PAGES || 3);
  let overlayPages = 0;
  let overlaySampled = 0;
  const overlayMissing = [];
  async function stressSpotlight(name) {
    if (overlayPages + overlayMissing.length >= SPOTLIGHT_PAGES) return;
    await page.keyboard.press('Control+k');
    const opened = await page
      .waitForSelector('[data-ap="spotlight-search"]', { timeout: 3000 })
      .then(() => true)
      .catch(() => false);
    if (!opened) {
      overlayMissing.push(name);
      return;
    }
    // 打一个字，让"实时建议"那一族行真的渲染出来（空面板里只有提示语）
    await page.keyboard.type('长标题').catch(() => {});
    await page.waitForTimeout(500);
    const res = await page.evaluate(STRESS, { ...cfg, per: 12 });
    overlayPages++;
    overlaySampled += res.sampled;
    if (res.docOverflow > 2) s1.push(`${name}·⌘K 横向溢出 ${Math.round(res.docOverflow)}px`);
    for (const x of res.boxLost) s2.push(`${name}·⌘K ${x}`);
    for (const x of res.wrapped) s3.push(`${name}·⌘K ${x}`);
    for (const x of uniq(res.noHint)) s4[x] = (s4[x] || 0) + 1;
    await page.keyboard.press('Escape');
    await page.waitForTimeout(150);
  }

  for (const r of TARGETS) {
    try {
      await page.goto(`${BASE}${r.path}`, { waitUntil: 'load', timeout: 30000 });
      await page.waitForTimeout(Number(process.env.SNAP_GAP_MS || 900));
      await page.mouse.move(1, 1);
      const res = await page.evaluate(STRESS, cfg);
      pages++;
      sampled += res.sampled;
      if (res.docOverflow > 2) s1.push(`${r.name} 横向溢出 ${Math.round(res.docOverflow)}px`);
      for (const x of res.boxLost) s2.push(`${r.name} ${x}`);
      for (const x of res.wrapped) s3.push(`${r.name} ${x}`);
      for (const x of uniq(res.noHint)) s4[x] = (s4[x] || 0) + 1;
      await stressSpotlight(r.name);
    } catch (e) {
      failed.push(`${r.name}: ${String(e).slice(0, 50)}`);
    }
  }
  await browser.close();

  console.log(`压测页数 = ${pages} / ${TARGETS.length}，共压 ${sampled} 个文本节点（每页最多 ${PER_PAGE} 个，两轮：CJK + 无空格长串）`);
  console.log(
    `浮层压测 = ⌘K 面板开在 ${overlayPages} / 计划 ${SPOTLIGHT_PAGES} 页，共压 ${overlaySampled} 个节点` +
      `${overlayMissing.length ? '（打不开：' + overlayMissing.join(', ') + '）' : ''}`,
  );
  // 判定覆盖数必须印出来：没有这个数，"0 命中"就分不清"浮层真没问题"和"探针根本没进浮层"。
  if (pages && !overlayPages) {
    console.log('✗ ⌘K 面板一次都没打开 —— 浮层那一档等于没量，上面所有"0 命中"都不含浮层');
    process.exitCode = 1;
  }
  console.log(`${failed.length ? '✗ 失败页：' + failed.join(', ') + '\n' : ''}`);
  const dump = (title, rows, limit) => {
    console.log(`\n== ${title}：${rows.length} 条 ==`);
    for (const x of uniq(rows).slice(0, limit)) console.log('   ' + x);
  };
  const all = [...s1, ...uniq(s2), ...uniq(s3)];
  const fresh = uniq(all.filter((x) => !isKnown(x)));
  const known = uniq(all.filter(isKnown));
  dump('S1 长内容撑破横向滚动（门禁）', s1, MAX);
  dump('S2 文本顶出组件盒（门禁）', s2, MAX);
  dump('S3 控件文案被压成多行（门禁）', s3, MAX);
  if (known.length) {
    console.log(`\n== 已知违例（源码写死的标签，不计入门禁）：${known.length} 条 ==`);
    for (const x of known) console.log('   ' + x);
    for (const k of KNOWN) console.log(`   理由 [${k.page} ${k.cls}「${k.text}」] ${k.why}`);
  }
  console.log(`\n== S4 截断无全名提示（登记表，不当门禁）：${Object.keys(s4).length} 类 ==`);
  for (const [k, n] of Object.entries(s4).sort((a, b) => b[1] - a[1]).slice(0, MAX)) console.log(`   ${k} ×${n}`);

  // 一个节点都没压到 = 探针没跑，四条"0 条"全是假的
  if (failed.length || sampled === 0) {
    console.log('\n✗ 有页面没压到内容 —— 上面所有"0 条"都不作数');
    process.exitCode = 1;
  }
  if (fresh.length) process.exitCode = 1;
  fs.writeFileSync(path.join(here, '..', 'long-content.json'), JSON.stringify(
    { s1: uniq(s1), s2: uniq(s2), s3: uniq(s3), s4, known }, null, 1));
  console.log(`\n门禁新违例 ${fresh.length} 条（另有已知 ${known.length} 条）；明细已写入 web/long-content.json`);
};

// 反向验证：造一个"注定撑破/顶出/折行"的样本，S1/S2/S3 必须各自抓到。
// 全站有 :focus-visible 和 overflow 兜底时，"0 条"和"探针失灵"打印出来一模一样，
// 所以这台必须先跑红再信它的绿。
const INJECT = () => {
  const main = document.querySelector('main') || document.body;
  const host = main.querySelector('[data-ap=card],[data-ap=stat-card],[data-ap=entity-card]') || main;
  // h3 用 nowrap 且不截断 → 文字墨迹必须顶出组件盒（S2）；
  // a 允许折行 → 标签必须被压成两行以上（S3）。
  // 二者都不预设 data-zzs：让 STRESS 自己采样打号，预设会和它分配的号撞车。
  host.innerHTML = `
    <div data-ap="zzstress" style="width:220px;display:inline-block;background:#eee;padding:8px">
      <h3 style="margin:0;white-space:nowrap">短标题</h3>
      <a href="#zz" style="display:inline-block">短链接</a>
    </div>`;
  // S1 要的是"整页横向溢出"，所以宽块挂在 body 上：塞进 main 会被它自己的
  // overflow:auto 吞掉，页面不滚动，探针就以"S1 全 0"的姿态漏判这一整类。
  const wide = document.createElement('div');
  wide.id = 'zzwide';
  wide.style.cssText = 'position:absolute;left:0;top:120px;width:2600px;height:6px;background:#333';
  document.body.appendChild(wide);
  return { host: !!host.querySelector('[data-ap=zzstress]') };
};

const EXPECT = (res) => {
  const lost = res.boxLost.filter((x) => x.includes('zzstress'));
  const wrapped = res.wrapped.filter((x) => x.includes('zzstress'));
  return { lost: lost.length, wrapped: wrapped.length, docOverflow: res.docOverflow };
};

const selftest = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  await loginForm(page, BASE, USER, PASS);
  await page.goto(`${BASE}/dashboard`, { waitUntil: 'load', timeout: 30000 });
  await page.waitForTimeout(1500);
  const clean = await page.evaluate(STRESS, { cjk: CJK, latin: LATIN, per: 999 });
  await page.evaluate(INJECT);
  const res = await page.evaluate(STRESS, { cjk: CJK, latin: LATIN, per: 999 });
  await browser.close();
  const e = EXPECT(res);
  const checks = [
    // S1 必须"注入前不溢出、注入后溢出"才算抓到：只看注入后 >2 的话，
    // 页面本来就有横向滚动时探针永远绿，而它其实什么都没测。
    [`S1 撑破横向（注入前 ${Math.round(clean.docOverflow)}px → 注入后 ${Math.round(res.docOverflow)}px）`,
      res.docOverflow > 2 && clean.docOverflow <= 2],
    [`S2 顶出组件盒（${e.lost} 条命中 zzstress）`, e.lost > 0],
    [`S3 控件文案折行（${e.wrapped} 条命中 zzstress）`, e.wrapped > 0],
  ];
  let bad = 0;
  for (const [k, ok] of checks) { if (!ok) bad++; console.log(`${ok ? '✓' : '✗'} ${k}`); }
  console.log(bad ? `✗ ${bad} 条没抓到明显违例——判据或采样写错了` : '三条门禁都抓到了自造违例');
  process.exitCode = bad ? 1 : 0;
};

if (argv.includes('--selftest')) selftest(); else run();
