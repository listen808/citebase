// 页面细节巡检：check:ui 管字号天阶、check:geom 管几何，这台管"内容与结构对不对得上"。
// 全部读真实渲染结果，不读源码：
//   P1 一个页面出现多个 h1（文档大纲被打破，读屏器的"标题列表"会出现两个同名顶层）
//   P2 标题层级跳跃（h2 直接跳 h4，中间没有 h3）
//   P3 可见按钮没有可访问名字（只有图标，既无 aria-label 也无文本）
//   P4 同一页里千分位写法不一致（1,234 与 1234 并存 → 一眼看出是两处代码各写各的）
//   P5 同一页里时间格式混用（绝对时间与"N 分钟前"并存）
//   P6 同一页里数量单位/量词混用（KB/MB 与 万/千，或同一列"次/条/个"混排）
//   P7 按钮文案脏字符（全角空格、emoji 当图标）—— 不查 textContent 的首尾空白：
//      模板里图标与文字之间的换行缩进必然进 textContent，渲染时会被折叠，那是噪声不是缺陷
//   P8 术语漂移登记表（新建/添加/新增、删除/移除、编辑/修改… 各出现在哪些页）——只登记不自动改，
//      文案归一是产品决定，不是机械修复
// 用法（前端 dev :3000、后端 :3001 需在跑；口令只走环境变量）：
//   SNAP_USER=… SNAP_PASS=… node scripts/check-page-detail.mjs --selftest   # 反向验证七条探针
//   SNAP_USER=… SNAP_PASS=… node scripts/check-page-detail.mjs [--only=<路由名子串>] [--max=10]
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
const MAX = Number(argOf('max') || 10);
const TARGETS = await pickTargets(ONLY, here);

const COLLECT = () => {
  const rect = (el) => el.getBoundingClientRect();
  const vis = (el) => { const r = rect(el); return r.width > 2 && r.height > 2; };
  const txt = (el) => (el.textContent || '').replace(/\s+/g, ' ').trim();
  const out = { h1: 0, h1Text: [], jumps: [], noName: [], groupMixed: null, timeMixed: null, unitMixed: null, dirty: [], labels: [], errored: false };

  const hs = [...document.querySelectorAll('main h1, [data-ap-scroll] h1, main h2, main h3, main h4, main h5, main h6, [data-ap-scroll] h2, [data-ap-scroll] h3, [data-ap-scroll] h4')]
    .filter(vis);
  out.h1 = hs.filter((h) => h.tagName === 'H1').length;
  out.h1Text = hs.filter((h) => h.tagName === 'H1').map(txt).slice(0, 3);
  let prev = 0;
  for (const h of hs) {
    const lv = +h.tagName[1];
    if (prev && lv > prev + 1) out.jumps.push(`h${prev}「${txt(hs[hs.indexOf(h) - 1] || h).slice(0, 10)}」→ h${lv}「${txt(h).slice(0, 10)}」`);
    prev = lv;
  }

  const accName = (b) => {
    const t = txt(b);
    if (t) return t;
    const al = b.getAttribute('aria-label');
    if (al) return al;
    const lb = b.getAttribute('aria-labelledby');
    if (lb) { const el = document.getElementById(lb.split(/\s+/)[0]); if (el && txt(el)) return txt(el); }
    const img = b.querySelector('img[alt]');
    if (img && img.getAttribute('alt')) return img.getAttribute('alt');
    const ti = b.getAttribute('title');
    if (ti) return ti;
    return '';
  };
  for (const b of document.querySelectorAll('main button, [data-ap-scroll] button')) {
    if (!vis(b) || b.disabled) continue;
    const n = accName(b);
    if (!n) {
      const p = b.closest('[data-ap=table]') ? '表格行内' : '';
      out.noName.push({ near: txt(b.parentElement || b).slice(0, 16), where: p, svg: !!b.querySelector('svg') });
    }
  }

  // 数字/时间/单位一律**按文本节点**采，不要把整页 textContent 拼成一串：元素之间没有
  // 分隔符，"9.0 GB" 紧接 "1,234" 会让 \b 失效（实测漏判），父元素还会把子元素的文本重复数一遍。
  const nodeTexts = [];
  for (const el of document.querySelectorAll('main *, [data-ap-scroll] *')) {
    if (!vis(el)) continue;
    for (const n of el.childNodes) if (n.nodeType === 3 && (n.textContent || '').trim()) nodeTexts.push(n.textContent);
  }
  const bodyText = nodeTexts.join('\n');

  // P4 千分位：只看 ≥1000 的整数，比较"带分隔符"与"不带"两种写法各自出现几次
  const withSep = (bodyText.match(/\b\d{1,3}(?:,\d{3})+\b/g) || []).length;
  const plain = (bodyText.match(/\b\d{4,}\b/g) || []).filter((s) => !s.includes(',')).length;
  if (withSep && plain) out.groupMixed = { withSep, plain };

  // P5 时间格式：绝对 vs 相对
  const abs = (bodyText.match(/\b\d{4}-\d{2}-\d{2}\b|\b\d{2}:\d{2}(:\d{2})?\b/g) || []).length;
  const rel = (bodyText.match(/\d+\s*(秒|分钟|小时|天|周|个月|年)前|刚刚/g) || []).length;
  if (abs && rel) out.timeMixed = { abs, rel };

  // P6 单位：字节单位与中文数量单位并存（"1.2 MB" 与 "3.4 万"）
  const bytes = (bodyText.match(/\b\d+(?:\.\d+)?\s*(B|KB|MB|GB)\b/g) || []).length;
  const cn = (bodyText.match(/\d+(?:\.\d+)?\s*[万千亿]/g) || []).length;
  if (bytes && cn) out.unitMixed = { bytes, cn };

  for (const b of document.querySelectorAll('main button, [data-ap-scroll] button')) {
    if (!vis(b)) continue;
    const label = txt(b);
    if (label) out.labels.push(label);
    // 判原始 textContent：txt() 用 \s+ 折叠空白，而全角空格本身就属于 \s，
    // 在折叠后的 label 上永远测不到 —— 第一版就是一条永不响的死判据。
    if (/[\u3000]/.test(b.textContent || '')) out.dirty.push({ why: '全角空格', label });
    if (/[\u{1F300}-\u{1FAFF}\u{2600}-\u{27BF}]/u.test(label)) out.dirty.push({ why: 'emoji 当图标', label });
  }

  out.errored = (window.__apErrored ? window.__apErrored() : false);
  return out;
};

// 术语漂移：同一意图的多种说法各自出现在哪些页。只登记，不自动改文案。
const GROUPS = {
  '创建': ['新建', '添加', '新增', '创建'],
  '删除': ['删除', '移除', '解除', '清空'],
  '编辑': ['编辑', '修改', '重命名'],
  '刷新': ['刷新', '重新加载', '重试'],
  '导出': ['导出', '下载'],
  '关闭': ['关闭', '收起', '取消'],
};


// 反向验证：每条规则都得能在"人为造出的违例"上响，否则 0 只是探针没量到。
const INJECT = {
  P1() { const h = document.createElement('h1'); h.textContent = '第二个页标题'; document.querySelector('main, [data-ap-scroll]')?.appendChild(h); return '再插一个 h1'; },
  P2() { const h = document.createElement('h4'); h.textContent = '跳级小节'; document.querySelector('main h2, main h3')?.after(h); return '在 h2 后面插一个 h4'; },
  // 注入不能改 Vue 管理的节点：仪表盘有轮询，重渲染会把"清空文本/塞全角空格"还原，
  // 于是探针看起来"抓不到"。改成插入一个自有节点，宿主归我。
  P3() {
    const host = document.querySelector('main, [data-ap-scroll]');
    if (!host) return null;
    const b = document.createElement('button');
    b.setAttribute('data-ap', 'button');
    b.innerHTML = '<svg width="14" height="14" viewBox="0 0 24 24"></svg>';
    host.appendChild(b);
    return '插入一个只有图标、没有任何可访问名字的按钮';
  },
  P7() {
    const host = document.querySelector('main, [data-ap-scroll]');
    if (!host) return null;
    const b = document.createElement('button');
    b.textContent = '保　存';
    host.appendChild(b);
    return '插入一个文案里带全角空格的按钮';
  },
  P4() { const d = document.createElement('div'); d.textContent = '1,234 与 5678'; document.querySelector('main, [data-ap-scroll]')?.appendChild(d); return '写入带分隔符与不带分隔符各一个'; },
  P5() { const d = document.createElement('div'); d.textContent = '2026-01-01 与 3 天前'; document.querySelector('main, [data-ap-scroll]')?.appendChild(d); return '写入绝对时间与相对时间各一个'; },
  P6() { const d = document.createElement('div'); d.textContent = '1.2 MB 与 3 万'; document.querySelector('main, [data-ap-scroll]')?.appendChild(d); return '写入字节单位与中文数量单位各一个'; },
};

const selftest = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  await installErroredProbe(page);
  await loginForm(page, BASE, USER, PASS);
  const read = (k, o) => (k === 'P1' ? (o.h1 > 1 ? 1 : 0)
    : k === 'P4' ? (o.groupMixed ? 1 : 0)
    : k === 'P5' ? (o.timeMixed ? 1 : 0)
    : k === 'P6' ? (o.unitMixed ? 1 : 0)
    : o[{ P2: 'jumps', P3: 'noName', P7: 'dirty' }[k]].length);
  let bad = 0;
  for (const k of ['P1', 'P2', 'P3', 'P4', 'P5', 'P6', 'P7']) {
    await page.goto(`${BASE}/dashboard`, { waitUntil: 'load', timeout: 30000 });
    await page.waitForTimeout(1500);
    const before = read(k, await page.evaluate(COLLECT));
    const how = await page.evaluate(INJECT[k]);
    const after = read(k, await page.evaluate(COLLECT));
    const fired = !!how && after > before;
    if (!fired) bad++;
    console.log(`${fired ? '✓' : '✗'} ${k}  ${before} → ${after}   ${how || '(注入失败：找不到宿主)'}`);
  }
  await browser.close();
  if (bad) console.log(`✗ ${bad} 条规则注入违例后没有变红——别信它的"0 页"`);
  else console.log('七条探针全部抓得到注入的违例');
  process.exitCode = bad ? 1 : 0;
};

const run = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  await installErroredProbe(page);
  page.on('dialog', (d) => (d.type() === 'beforeunload' ? d.accept() : d.dismiss()).catch(() => {}));
  await loginForm(page, BASE, USER, PASS);

  const rows = [];
  for (const r of TARGETS) {
    try {
      await page.goto(`${BASE}${r.path}`, { waitUntil: 'load', timeout: 30000 });
      await page.waitForTimeout(Number(process.env.SNAP_GAP_MS || 1000));
      rows.push({ name: r.name, path: r.path, ...(await page.evaluate(COLLECT)) });
    } catch (e) {
      rows.push({ name: r.name, failed: String(e).slice(0, 70) });
    }
  }
  // 兜底页先自动复跑一次：多半是巡检自己打满限流（300 次/分钟/IP），不是页面坏了
  await retryCrashedPages(page, BASE, rows, async (p, row) => Object.assign(row, await p.evaluate(COLLECT)));
  await browser.close();

  const errored = rows.filter((x) => x.errored);
  const ok = rows.filter((x) => !x.failed && !x.errored);
  console.log(`量测页数 = ${ok.length} / ${rows.length}${errored.length ? '（兜底页：' + errored.map((x) => x.name).join(', ') + '）' : ''}\n`);

  const bad = (title, pick, fmt) => {
    const hits = ok.map((x) => ({ p: x.name, v: pick(x) })).filter((x) => x.v && (Array.isArray(x.v) ? x.v.length : true));
    console.log(`== ${title}：${hits.length} 页 ==`);
    for (const h of hits.slice(0, MAX)) console.log(`   ${h.p}  ` + fmt(h.v));
    console.log('');
  };

  bad('P1 多个 h1', (x) => (x.h1 > 1 ? [`h1×${x.h1}: ${x.h1Text.join(' / ')}`] : []), (v) => v.join(' ; '));
  bad('P2 标题层级跳跃', (x) => x.jumps, (v) => v.slice(0, 3).join(' ; '));
  bad('P3 按钮没有可访问名字', (x) => x.noName.map((n) => `${n.where || n.near}${n.svg ? '(只有图标)' : ''}`), (v) => `${v.length} 个：` + v.slice(0, 4).join(' ; '));
  bad('P4 千分位写法混用', (x) => (x.groupMixed ? [`${x.groupMixed.withSep} 个带分隔符 / ${x.groupMixed.plain} 个不带`] : []), (v) => v.join(''));
  bad('P5 时间格式混用（绝对 vs 相对）', (x) => (x.timeMixed ? [`${x.timeMixed.abs} 处绝对 / ${x.timeMixed.rel} 处相对`] : []), (v) => v.join(''));
  bad('P6 数量单位混用（字节 vs 万/千）', (x) => (x.unitMixed ? [`${x.unitMixed.bytes} 处 B/KB/MB / ${x.unitMixed.cn} 处 万/千/亿`] : []), (v) => v.join(''));
  bad('P7 按钮文案脏字符', (x) => x.dirty.map((d) => `${d.why} ${JSON.stringify(d.label)}`), (v) => v.slice(0, 4).join(' ; '));

  const labels = ok.flatMap((x) => x.labels.map((l) => ({ p: x.name, l })));
  console.log(`== P8 术语漂移登记（同一意图的多种说法，共 ${labels.length} 个按钮文案）==`);
  for (const [intent, words] of Object.entries(GROUPS)) {
    const found = words.map((w) => [w, labels.filter((x) => x.l.startsWith(w)).map((x) => x.p)]).filter(([, ps]) => ps.length);
    if (found.length > 1) {
      console.log(`   「${intent}」有 ${found.length} 种说法：` + found.map(([w, ps]) => `${w}×${ps.length}(${[...new Set(ps)].slice(0, 4).join(',')})`).join('  '));
    }
  }

  fs.writeFileSync(path.join(here, '..', 'page-detail.json'), JSON.stringify(rows, null, 1));
  console.log('\n明细已写入 web/page-detail.json');
  if (errored.length) process.exitCode = 1;
};

if (argv.includes('--selftest')) selftest(); else run();
