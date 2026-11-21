// 弹层几何审计：页面巡检只量"打开页面时看到的东西"，而表单几乎全住在 Modal/Drawer 里，
// 那些地方从来没被测过。这台把每个页面上"会开一个对话框"的按钮点开，量里面的对齐与可达性。
// 五条断言（读真实渲染盒）：
//   D1 对话框超出视口（顶部被顶出去 / 底部够不着）
//   D2 单行字段自己分档，或同一行按钮自己分档（textarea 高度按行数走、页脚按钮统一 small，都不算混档）
//   D3 控件没有可访问名字（无 label / aria-label / placeholder）
//   D4 同一个表单里 label 位置混用（有的在控件上方、有的同行）
//   D5 对话框内文字截断无提示 / 控件文字折行
//   D6 对话框内控件被 overflow:hidden 的祖先纵向切掉
// 只做"打开—量—关掉"，绝不提交：删除/执行/保存类按钮一律不点。
// 用法（前端 dev :3000、后端 :3001 需在跑；口令只走环境变量）：
//   SNAP_USER=… SNAP_PASS=… node scripts/check-dialog-layout.mjs [--only=<路由名子串>] [--max=12] [--widths=1440]
import { chromium } from 'playwright';
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
const MAX = Number(argOf('max') || 8);
const WIDTHS = (argOf('widths') || '1440').split(',').map(Number).filter(Boolean);
const TARGETS = await pickTargets(ONLY, here);

// 只点"会开一个对话框"的动作。NEVER 保守取全集：任何有副作用的词都排除，
// 因为脚本会在真实库上跑，误点一次「删除」就是一次真删除。
const TRIGGER = /新建|新增|添加|编辑|重命名|移动|复制|成员|权限|详情|配置|设置|邀请|上传/;
const NEVER = /删除|移除|清空|重置|执行|运行|触发|重跑|同步|保存|提交|发送|导出|下载|导入|重建|恢复|回滚|应用|退出|登出|索引|升级|开始|停止|启用|禁用/;

const DIALOG_COLLECT = () => {
  const rect = (el) => el.getBoundingClientRect();
  const vis = (el) => { const r = rect(el); return r.width > 2 && r.height > 2; };
  const textOf = (el) => (el.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 18);
  const dlg = document.querySelector('[role=dialog][aria-modal="true"]');
  if (!dlg) return null;
  const dr = rect(dlg);
  const out = {
    title: textOf(dlg.querySelector('h1,h2,h3,[id$="-title"],[id$="title"]') || dlg) || '(无标题)',
    size: `${Math.round(dr.width)}×${Math.round(dr.height)}`,
    overflow: [], wide: [], contentOverflowX: [], mixedHeights: [], noName: [], labelPlacement: [], clipped: [], wrapped: [], clippedBy: [],
  };
  if (dr.top < -1 || dr.bottom > innerHeight + 1) {
    out.overflow.push({ top: Math.round(dr.top), bottom: Math.round(dr.bottom), vh: innerHeight });
  }
  // D2 只在同类之间比：全站 123 处弹层页脚按钮都是 small(32)，字段是 medium(40)，
  // 把"字段 vs 页脚按钮"算混档会在每个带表单的弹层上报假阳性。
  // 真正的缺陷是：同一个表单里字段自己分档，或同一行按钮自己分档。
  const nameOf = (c) => c.getAttribute('aria-label') || c.getAttribute('placeholder') || c.getAttribute('title')
    || ((c.id && document.querySelector(`label[for="${CSS.escape(c.id)}"]`) || {}).textContent || '').trim()
    || textOf(c);
  // textarea 不进这条比较：它的高度由 rows/内容决定（实测 65px、254px 都是正常的
  // 多行框），拿单行控件的高度阶梯去比必然假阳性。
  const fields = [...dlg.querySelectorAll('input:not([type=hidden]):not([type=checkbox]):not([type=radio]), select')]
    .filter((c) => vis(c));
  const fh = [...new Set(fields.map((c) => Math.round(rect(c).height)))].sort((a, b) => a - b);
  if (fh.length > 1) {
    out.mixedHeights.push({
      kind: '字段', heights: fh.join('/'),
      items: fields.map((c) => `${Math.round(rect(c).height)}:${c.tagName.toLowerCase()} ${JSON.stringify(nameOf(c).slice(0, 12))}`),
    });
  }
  const byRow = new Map();
  for (const b of [...dlg.querySelectorAll('button')].filter((x) => vis(x))) {
    const p = b.parentElement;
    if (!p) continue;
    if (!byRow.has(p)) byRow.set(p, []);
    byRow.get(p).push(b);
  }
  for (const [, list] of byRow) {
    if (list.length < 2) continue;
    const hs = [...new Set(list.map((c) => Math.round(rect(c).height)))].sort((a, b) => a - b);
    if (hs[hs.length - 1] - hs[0] > 2) {
      out.mixedHeights.push({
        kind: '同行按钮', heights: hs.join('/'),
        items: list.map((c) => `${Math.round(rect(c).height)}:${JSON.stringify(nameOf(c).slice(0, 10))}`),
      });
    }
  }
  for (const c of [...dlg.querySelectorAll('input, select, textarea')]) {
    if (!vis(c)) continue;
    if (c.type === 'hidden' || c.type === 'checkbox' || c.type === 'radio') continue;
    const named = c.getAttribute('aria-label') || c.getAttribute('placeholder') || c.getAttribute('title')
      || (c.id && document.querySelector(`label[for="${CSS.escape(c.id)}"]`))
      || c.closest('label');
    if (!named) out.noName.push({ tag: c.tagName.toLowerCase(), near: textOf(c.parentElement || c) });
  }
  const above = [];
  const inline = [];
  for (const lab of dlg.querySelectorAll('label')) {
    if (!vis(lab)) continue;
    const lr = rect(lab);
    const target = (lab.htmlFor && document.getElementById(lab.htmlFor))
      || lab.querySelector('input, select, textarea');
    if (!target || !vis(target)) continue;
    const tr = rect(target);
    (tr.top >= lr.bottom - 2 ? above : inline).push(textOf(lab));
  }
  if (above.length && inline.length) out.labelPlacement.push({ above: above.length, inline: inline.length, 例: `${above[0]} / ${inline[0]}` });
  for (const el of dlg.querySelectorAll('*')) {
    if (el.children.length || !vis(el)) continue;
    const t = (el.textContent || '').trim();
    if (t.length < 6) continue;
    const cs = getComputedStyle(el);
    if ((/hidden|clip/.test(cs.overflow) || cs.textOverflow === 'ellipsis') && el.scrollWidth > el.clientWidth + 1) {
      let hint = false;
      for (let n = el, i = 0; n && i < 3; n = n.parentElement, i++) {
        if (n.getAttribute && (n.getAttribute('title') || n.getAttribute('aria-label'))) { hint = true; break; }
      }
      if (!hint) out.clipped.push({ text: t.slice(0, 18), lost: el.scrollWidth - el.clientWidth });
    }
  }
  const lines = (el) => {
    const tops = new Set();
    for (const n of el.childNodes) {
      if (n.nodeType !== 3 || !(n.textContent || '').trim()) continue;
      const r = document.createRange();
      r.selectNodeContents(n);
      for (const box of r.getClientRects()) if (box.height > 1) tops.add(Math.round(box.top));
    }
    return tops.size;
  };
  for (const b of dlg.querySelectorAll('button')) {
    if (!vis(b) || !(b.textContent || '').trim()) continue;
    if (lines(b) > 1) out.wrapped.push({ text: textOf(b), h: Math.round(rect(b).height) });
  }
  // D7 弹层横向出界：遮罩自带 24px padding，而 90vw 的上限在窄屏会大于
  // 遮罩的内容盒（390 - 48 = 342 < 351 = 90vw），于是"有 max-width 却仍然溢出"。
  if (dr.left < 3 || dr.right > innerWidth - 3) {
    out.wide.push({ left: Math.round(dr.left), right: Math.round(dr.right), vw: innerWidth });
  }
  // D8 弹层内容比弹层还宽（横向被切或需要左右拖着看）
  if (dlg.scrollWidth > dlg.clientWidth + 2) {
    out.contentOverflowX.push({ lost: dlg.scrollWidth - dlg.clientWidth });
  }

  // D6 弹层里的控件被 overflow:hidden 的祖先纵向切掉（页面侧同名断言是 check:geom 的 R8；
  // 弹层里的固定高度条不在页面巡检的覆盖范围内）
  for (const c of dlg.querySelectorAll('button, input:not([type=hidden]), select, textarea')) {
    if (!vis(c)) continue;
    const r = rect(c);
    for (let a = c.parentElement; a && a !== dlg; a = a.parentElement) {
      const s = getComputedStyle(a);
      if (/auto|scroll/.test(s.overflowY)) break;
      if (!/hidden|clip/.test(s.overflowY)) continue;
      const ar = rect(a);
      const lost = Math.max(r.bottom - ar.bottom, ar.top - r.top);
      if (lost > 2) out.clippedBy.push({ ctl: textOf(c) || c.tagName.toLowerCase(), by: textOf(a).slice(0, 12), lost: Math.round(lost) });
      break;
    }
  }
  return out;
};

const run = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  page.on('dialog', (d) => (d.type() === 'beforeunload' ? d.accept() : d.dismiss()).catch(() => {}));
  await loginForm(page, BASE, USER, PASS);

  const seen = [];
  for (const w of WIDTHS) {
    await page.setViewportSize({ width: w, height: 900 });
    for (const r of TARGETS) {
      await page.goto(`${BASE}${r.path}`, { waitUntil: 'load', timeout: 30000 }).catch(() => {});
      await page.waitForTimeout(900);
      const labels = await page.evaluate(([TRIGGER_SRC, NEVER_SRC]) => {
        const T = new RegExp(TRIGGER_SRC);
        const N = new RegExp(NEVER_SRC);
        const hits = [];
        for (const b of document.querySelectorAll('main button, [data-ap-scroll] button')) {
          const t = (b.textContent || '').trim();
          if (!t || t.length > 12) continue;
          if (N.test(t) || !T.test(t)) continue;
          // 只点叶子按钮：容器按钮（如 ApSelect 的触发器）点下去是开下拉，
          // 会把"下一个候选"变成下拉里的一个选项，量出来的就不是对话框了
          if (b.querySelector('button, [role=listbox], [role=combobox]')) continue;
          hits.push(t);
        }
        return [...new Set(hits)].slice(0, 3);
      }, [TRIGGER.source, NEVER.source]);
      for (const label of labels) {
        const btn = page.locator(`main button:has-text("${label}"), [data-ap-scroll] button:has-text("${label}")`).first();
        const opened = await btn.click({ timeout: 2500 }).then(() => true).catch(() => false);
        if (!opened) continue;
        await page.waitForTimeout(700);
        const d = await page.evaluate(DIALOG_COLLECT);
        if (d) {
          seen.push({ page: r.name, w, trigger: label, ...d });
          await page.keyboard.press('Escape').catch(() => {});
          await page.waitForTimeout(350);
          const still = await page.evaluate(() => !!document.querySelector('[role=dialog][aria-modal="true"]'));
          if (still) {
            await page.evaluate(() => {
              const x = document.querySelector('[role=dialog] button[aria-label*="关闭"], [role=dialog] [data-ap=button]');
              if (x) x.dispatchEvent(new MouseEvent('click', { bubbles: true }));
            });
            await page.keyboard.press('Escape');
            await page.waitForTimeout(300);
          }
          const open2 = await page.evaluate(() => !!document.querySelector('[role=dialog][aria-modal="true"]'));
          if (open2) { await page.goto(`${BASE}${r.path}`, { waitUntil: 'load' }); await page.waitForTimeout(700); }
        }
      }
    }
  }
  await browser.close();

  const byW = {};
  for (const d of seen) (byW[d.w] = byW[d.w] || []).push(d);
  for (const w of Object.keys(byW).sort((a, b) => b - a)) {
    const list = byW[w];
    console.log(`\n########## 视口 ${w}×900：打开 ${list.length} 个对话框 ##########`);
    console.log('打开的弹层：' + list.map((d) => `${d.page}:${d.title}`).slice(0, 40).join(', '));
    const bad = (name, pick) => {
      const hits = list.filter((d) => pick(d).length);
      console.log(`\n== ${name}：${hits.length} 个 ==`);
      for (const d of hits.slice(0, MAX)) console.log(`   ${d.page} 「${d.trigger}」→ ${d.title} (${d.size})  ` + pick(d).map((v) => JSON.stringify(v)).join(' ; '));
    };
    bad('D1 弹层超出视口', (d) => d.overflow);
    bad('D2 同一表单控件高度混档', (d) => d.mixedHeights);
    bad('D3 控件没有可访问名字', (d) => d.noName);
    bad('D4 label 位置混用（上置与同行并存）', (d) => d.labelPlacement);
    bad('D5a 文字截断无提示', (d) => d.clipped);
    bad('D5b 按钮文字折行', (d) => d.wrapped);
    bad('D6 控件被 overflow:hidden 的祖先切掉', (d) => d.clippedBy);
    bad('D7 弹层横向出界', (d) => d.wide);
    bad('D8 弹层内容比弹层还宽', (d) => d.contentOverflowX);
  }
  if (!seen.length) console.log('一个对话框都没打开——先怀疑选择器或触发词表，不要当成"弹层没问题"。');
};

run();
