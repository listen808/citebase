// 逐组件几何普查 + 覆盖清单：不是"抓几条违例"，而是把**每一个组件、每一个控件**
// 在真实页面上渲染出来的位置与尺寸都记一遍账，然后回答两个问题：
//   ① 有没有组件根本没被量到？（B5 覆盖清单 —— 弹层里的、需交互才渲染的、
//      被父组件身份吃掉的，全都算"没量到"，必须显式列出来而不是假装全站绿）
//   ② 量到的这些，尺寸/位置是不是落在同一套尺子上？
//      B1 控件高度不在控制天阶（24/28/32/40/44）
//      B2 盒模型漂移：box-sizing 不是 border-box，声明高度和实测外盒对不上
//      B3 同容器左边缘不齐（差值不落尺：1/3/5/6/7px 这种"看着偏一点"）
//      B4 兄弟控件纵向间距不落尺
//      B5 覆盖清单（组件 → 实例数 → 出现页）
//      B6 元素跑到视口外（横向被裁 / 定位算错）
//      B7 一行里混用「label 在上」与「label 同行」两种字段形态
//      B8 同形态一行里 label 没落在同一条水平线上（全同行比 label 中线，全在上比控件底边）
// 会**主动打开** Modal / Drawer（沿用弹层巡检那套只点叶子按钮、破坏性文案一律不点的口径），
// 否则表单控件这一整类永远量不到。
// 用法（前端 dev :3000、后端 :3001 需在跑；口令只走环境变量）：
//   SNAP_USER=… SNAP_PASS=… node scripts/check-box-census.mjs [--only=<路由名子串>] [--max=18]
//   node scripts/check-box-census.mjs --selftest   # 反向验证：B1–B4、B6、B7、B8、B9 各自必须抓到注入的违例
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
const MAX = Number(argOf('max') || 18);
const TARGETS = await pickTargets(ONLY, here);

// 组件身份清单从源码算：src/components 下所有 Ap*.vue，去掉前缀转 kebab = mixin 打的 data-ap
const kebab = (n) => n.replace(/^Ap/, '').replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase();
const AP_COMPONENTS = (() => {
  const out = new Map();
  const walk = (d) => {
    for (const f of fs.readdirSync(d, { withFileTypes: true })) {
      if (f.isDirectory()) { if (!/node_modules/.test(f.name)) walk(path.join(d, f.name)); }
      else if (/^Ap[A-Z]\w*\.vue$/.test(f.name)) out.set(kebab(f.name.replace('.vue', '')), f.name);
    }
  };
  walk(path.join(here, '..', 'src', 'components'));
  return out;
})();

// mixin 的守卫是 `!el.hasAttribute('data-ap')` —— **子组件先 mounted 会赢**。
// 所以 `<ApFieldWrapper>` 当模板根的那些组件（ApInput/ApSelect/ApTextarea/ApFormItem…）
// 永远不会有自己的 data-ap：它们归并在宿主名下。这不是"没渲染"，
// 上一版把它们算进"没量到"是误报，24 个里一大半是这么来的。
const ABSORBED = (() => {
  const map = new Map();
  for (const [slug, file] of AP_COMPONENTS) {
    const dir = ['components/apple', 'components'].find((d) => fs.existsSync(path.join(here, '..', 'src', d, file)));
    if (!dir) continue;
    const text = fs.readFileSync(path.join(here, '..', 'src', dir, file), 'utf8');
    const tpl = text.slice(text.indexOf('<template'));
    const root = (tpl.match(/<template>\s*<([A-Za-z][\w-]*)/) || [])[1] || '';
    if (/^Ap[A-Z]/.test(root) && root !== file.replace('.vue', '')) {
      map.set(slug, root.replace(/^Ap/, '').replace(/([a-z0-9])([A-Z])/g, '$1-$2').toLowerCase());
    }
  }
  return map;
})();

const LADDER_H = [24, 28, 32, 40, 44];
const SPACE = [2, 4, 8, 12, 16, 20, 24, 32, 40, 48, 64, 80];

const CENSUS = (_cfg) => {
  const rows = [];
  const own = (el) => el.getAttribute('data-ap') || null;
  const rectOf = (el) => el.getBoundingClientRect();
  // 组身份优先用**元素自己**的 data-ap：只按最近祖先分组，会把嵌套组件并进父类
  // （ApInput 的根被 ApFieldWrapper 抢先标掉，input/select/textarea 整类就"消失"了）
  const groupOf = (el) => own(el) || (el.closest('[data-ap]')?.getAttribute('data-ap') || '?') + '/' + el.tagName.toLowerCase();
  // 兄弟关系必须按**父元素身份**认，不能按"组名#标签"：同一张卡片里几十个 <div>
  // 会被并成一桶，于是两列网格里相邻字段的输入框成了"兄弟"，比出 9px 的假"左边缘不落尺"
  // （B3 的 8 类里 6 类是这个）。组名留着给人看，桶键换成 WeakMap 发号。
  const pids = new WeakMap();
  let pidSeq = 0;
  const pidOf = (n) => {
    if (!n) return '';
    if (!pids.has(n)) pids.set(n, `#${++pidSeq}`);
    return pids.get(n);
  };
  for (const el of document.querySelectorAll('body [data-ap], body [data-ap] *, body input, body select, body textarea, body button, body a[href], body [role="tab"], body [role="button"]')) {
    const r = rectOf(el);
    if (r.width < 3 || r.height < 2) continue;
    if (el.closest('.monaco-editor, .cm-editor, [data-mode-id], [class*="devtools"]')) continue;
    const cs = getComputedStyle(el);
    rows.push({
      g: groupOf(el),
      tag: el.tagName.toLowerCase(),
      ta: cs.textAlign,
      // 只给叶子节点取文本：清单必须能说出"是哪一段"，否则复核的人还得自己回页面上找一遍。
      // 非叶子取 textContent 会拼出整页文本，既慢又没意义。
      text: el.children.length === 0 ? (el.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 22) : '',
      w: Math.round(r.width * 10) / 10,
      h: Math.round(r.height * 10) / 10,
      x: Math.round(r.left),
      y: Math.round(r.top),
      right: Math.round(r.right),
      box: cs.boxSizing,
      declared: cs.height,
      border: parseFloat(cs.borderTopWidth) + parseFloat(cs.borderBottomWidth),
      pad: parseFloat(cs.paddingTop) + parseFloat(cs.paddingBottom),
      mt: parseFloat(cs.marginTop) || 0,
      mb: parseFloat(cs.marginBottom) || 0,
      svg: /^(svg|path|circle|ellipse|rect|polyline|line|polygon|g|text)$/.test(el.tagName.toLowerCase()),
      // 有没有横向滚动/裁切的祖先：表格放在 overflow-x:auto 的壳里，元素 right
      // 超过视口是**正常滚动**，不是跑出画面（check:geom 的 R7 同口径）。
      clipX: (() => {
        for (let n = el.parentElement; n; n = n.parentElement) {
          const o = getComputedStyle(n).overflowX;
          if (o === 'auto' || o === 'scroll' || o === 'hidden' || o === 'clip') return 1;
        }
        return 0;
      })(),
      parent: el.parentElement ? pidOf(el.parentElement) + '#' + (el.parentElement.tagName || '') : '',
      parentLabel: el.parentElement ? groupOf(el.parentElement) + '#' + (el.parentElement.tagName || '') : '',
      gap: el.parentElement ? (parseFloat(getComputedStyle(el.parentElement).rowGap) || 0) : 0,
      ptop: el.parentElement ? Math.round(rectOf(el.parentElement).top) : 0,
    });
  }
  // B7 字段形态：一行里每项的 label 要么"与控件同行"、要么"在控件上方"。
  // 混用就是用户说的"都不在一条水平上"：label 高低不齐、控件基线也不齐。
  // 弹层巡检早就有这条（D4），页面级从来没量过。
  // B8 同形态行的 label 落线：全是"同行"形态的行，各 label 的**纵向中线**必须重合；
  // 全是"在上"形态的行，各控件的**底边**必须重合（这就是底对齐存在的唯一理由）。
  // 文档协作中心「空间 / 文档 / 当前审批」三组都是同行形态，B7 抓不到它 ——
  // 下拉 40px、纯文字 19px，行按 flex-end 对齐就把矮组整条拽下去，label 低 10px。
  const fieldRows = [];
  const alignRows = [];
  // 真正比过的行数。没有这个数，"0 命中"就分不清"全站干净"和"探针没接到东西上"
  // —— B7 已经犯过一次（采了 fieldRows 却从没落桶，恒为 0）。
  let judged = 0;
  const analyze = (list, rowId, where, mode) => {
    const groups = [];
    for (const g of list) {
      const gr = rectOf(g);
      if (gr.width < 8 || gr.height < 8) continue;
      const kids = [...g.children];
      if (kids.length < 2) continue;
      // label 只认组内**第一个**子元素且必须是文本元素：否则"值"会被当成 label
      const lab = kids[0];
      if (!/^(SPAN|LABEL|P|B|STRONG)$/.test(lab.tagName)) continue;
      const text = (lab.textContent || '').trim();
      if (!text || text.length > 12) continue;
      const ctlBox = (() => {
        const r = { top: Infinity, bottom: -Infinity };
        for (const n of kids.slice(1)) {
          const q = rectOf(n);
          if (q.width < 2 || q.height < 2) continue;
          r.top = Math.min(r.top, q.top);
          r.bottom = Math.max(r.bottom, q.bottom);
        }
        return r.bottom === -Infinity ? null : r;
      })();
      if (!ctlBox) continue;
      const lr = rectOf(lab);
      const beside = lr.top + lr.height / 2 > ctlBox.top - 1 && lr.top + lr.height / 2 < ctlBox.bottom + 1;
      const above = lr.bottom <= ctlBox.top + 1;
      if (!beside && !above) continue;
      const lcs = getComputedStyle(lab);
      groups.push({
        shape: beside ? 'inline' : 'stacked',
        lab: lr.top + lr.height / 2,
        labTop: lr.top,
        vtop: ctlBox.top,
        vbot: ctlBox.bottom,
        gh: gr.height,
        // 只有**同一套 label 样式**才谈得上"该落在同一条线上"。角色成员页每行是
        // 「头像字母 + 姓名/工号列」，头像是大号居中字母、姓名是正文 —— 样式不同，
        // 拿它们比 label 中线会报出 8px 假阳性。
        sig: `${lcs.fontSize}|${lcs.fontWeight}|${lcs.color}`,
      });
    }
    if (groups.length < 2) return;
    const shapes = new Set(groups.map((x) => x.shape));
    if (shapes.size > 1) {
      judged++;
      fieldRows.push(rowId + ' → ' + [...shapes].join('+') + ' ' + where);
      return;
    }
    const sameSig = {};
    for (const g of groups) (sameSig[g.sig] = sameSig[g.sig] || []).push(g);
    for (const l of Object.values(sameSig)) {
      if (l.length < 2) continue;
      judged++;
      if (l[0].shape === 'inline') {
        const mids = l.map((x) => x.lab);
        const d = Math.round(Math.max(...mids) - Math.min(...mids));
        if (d > 2) alignRows.push(rowId + ' → 同行 label 中线差 ' + d + 'px（组高 ' + l.map((x) => Math.round(x.gh)).join('/') + 'px，行 ' + mode + '） ' + where);
      } else {
        // "在上"形态比什么，由行的对齐方式决定：底对齐比控件底边（共享基线就是它存在的
        // 理由），center 比中线，stretch/start（网格默认）比顶边 —— 三种都比"label 顶边"
        // 会在底对齐行里误报（一个字段多一行 hint 就把它的 label 顶上去，那是设计意图）。
        const key = mode === 'end' ? 'vbot' : mode === 'center' ? 'vmid' : 'vtop';
        const vals = l.map((x) => (key === 'vmid' ? (x.vtop + x.vbot) / 2 : x[key]));
        const d = Math.round(Math.max(...vals) - Math.min(...vals));
        if (d > 2) alignRows.push(rowId + ' → 控件' + (key === 'vbot' ? '底边' : key === 'vtop' ? '顶边' : '中线') + '差 ' + d + 'px（行 ' + mode + '） ' + where);
      }
    }
  };

  // 一行 = 一个比较单元。flex 行直接取 children；**网格按视觉行切**（顶边相同的
  // 算同一行）—— 表单用 display:grid 一行摆两个字段，上一版只认 flex，整类没看见。
  for (const row of document.querySelectorAll('body [data-ap] *, body [data-ap]')) {
    const cs = getComputedStyle(row);
    const isFlex = cs.display === 'flex' && cs.flexDirection === 'row';
    const isGrid = cs.display === 'grid' || cs.display === 'inline-grid';
    if (!isFlex && !isGrid) continue;
    if (row.childElementCount < 2 || row.childElementCount > 12) continue;
    const rr = rectOf(row);
    if (rr.width < 120 || rr.height < 16 || rr.height > (isGrid ? 420 : 160)) continue;
    if (row.closest('[data-ap=markdown-preview], .md-preview, .chat-md-content, table')) continue;
    const rowId = (row.closest('[data-ap]')?.getAttribute('data-ap') || '无') + '/' + row.tagName.toLowerCase();
    const where = '@' + Math.round(rr.x) + ',' + Math.round(rr.y);
    const am = /end/.test(cs.alignItems) ? 'end' : /center|baseline/.test(cs.alignItems) ? 'center' : 'start';
    if (isFlex) { analyze([...row.children], rowId, where, am); continue; }
    const bands = [];
    for (const g of row.children) {
      const t = rectOf(g).top;
      const band = bands.find((x) => Math.abs(x.t - t) <= 24);
      if (band) { band.list.push(g); band.t = Math.min(band.t, t); }
      else bands.push({ t, list: [g] });
    }
    for (const x of bands) analyze(x.list, rowId, where, am);
  }
  const byParent = {};
  for (const row of rows) if (!row.svg) (byParent[row.parent] = byParent[row.parent] || []).push(row);

  // 只有**纵向堆叠**的兄弟才谈得上"对齐"和"间距"。横排（一行工具栏、表格一行、
  // 网格一列列）的 x 差是列宽，不是错位 —— 上一版按 x 分组比，报出 392px/1062px
  // 这种荒唐"左边缘不落尺"，还把行高 half-leading 的 0.6px/8.6px 当成间距违例。
  const stacked = (a, b) => b.y >= a.y + a.h - 1 && a.x < b.x + b.w && b.x < a.x + a.w;
  const gaps = [];
  const edges = [];
  for (const [parent, list] of Object.entries(byParent)) {
    if (list.length < 2 || list.length > 40) continue;
    const sorted = [...list].sort((a, b) => (a.y - b.y) || (a.x - b.x));
    for (let i = 1; i < sorted.length; i++) {
      const a = sorted[i - 1];
      const b = sorted[i];
      if (!stacked(a, b)) continue;
      // 间距量**声明值**（margin-top + 父容器 row-gap），不量矩形距离
      const declared = b.gap + b.mt;
      // 亚 2px 的"间距"不是间距：那是 1px 发丝分隔或 half-leading 的取整余数。
      // 布局尺最细一档是 2px，把它计进清单的结果是 961 条 button 1px 淹没真正待看的 30/26/28px。
      if (declared >= 2 && declared < 200) gaps.push({ g: b.g, gap: declared, text: b.text, x: b.x, y: b.y });
      // 左边缘只关心"偏一点点"（1–11px 的抖动）；更大的差值是两列，不是没对齐。
      // 居中对齐的文本要跳过：ApEmpty 那种"标题 + 说明"整块居中，左边缘由**字数**决定，
      // 比它得到 1–9px 的差是必然的，不是错位（实测 empty/p 三条全是这么来的）。
      const dx = Math.abs(b.x - a.x);
      const centered = (n) => /center|right/.test(n.ta || '');
      if (dx > 0 && dx < 12 && !centered(a) && !centered(b)) edges.push({ g: b.g, parent, diff: dx });
    }
  }
  // B9 工具条换行：一条 flex-wrap 的控件行，在 1440 下必须只有一行。
  // 这条是第十五批逼出来的：把筛选并进工具条之后，媒体页仍是三行 —— 而 R7/R8/B6 全 0，
  // 因为换行既不横向滚动也不裁切。"并进去了"和"看起来是一行"是两件事，只有数行才分得出。
  const toolbars = [];
  let tbJudged = 0;
  for (const el of document.querySelectorAll('body div')) {
    const cs = getComputedStyle(el);
    if (cs.display !== 'flex' || cs.flexWrap !== 'wrap' || cs.flexDirection.startsWith('column')) continue;
    const kids = [...el.children].filter((c) => {
      const r = c.getBoundingClientRect();
      return r.width > 10 && r.height > 8;
    });
    if (kids.length < 3) continue;
    // 只认「控件行」：至少两枚可点/可输入的控件，避免把普通卡片容器当工具条
    const controls = kids.filter((c) => c.matches('button, input, select, [role="tab"], [role="button"]')
      || c.querySelector('button, input, select, [role="tab"]'));
    if (controls.length < 2) continue;
    tbJudged++;
    const ai = cs.alignItems;
    const key = (c) => {
      const r = c.getBoundingClientRect();
      if (ai === 'flex-end') return r.bottom;
      if (ai === 'center') return (r.top + r.bottom) / 2;
      return r.top;
    };
    const ks = kids.map(key).sort((a, b) => a - b);
    let lines = 1;
    for (let i = 1; i < ks.length; i++) if (ks[i] - ks[i - 1] > 6) lines++;
    if (lines > 1) {
      const w = Math.round(el.getBoundingClientRect().width);
      toolbars.push(`${groupOf(el)}/${el.tagName.toLowerCase()} → 折成 ${lines} 行（行宽 ${w}px、${kids.length} 项，首项「${(kids[0].textContent || kids[0].tagName).trim().slice(0, 14)}」）`);
    }
  }
  return { rows, gaps, edges, fieldRows, alignRows, toolbars, judged, tbJudged, vw: innerWidth };
};

const CONTROL_TAGS = new Set(['button', 'input', 'select', 'textarea']);

const run = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  page.on('dialog', (d) => (d.type() === 'beforeunload' ? d.accept() : d.dismiss()).catch(() => {}));
  await loginForm(page, BASE, USER, PASS);

  const allRows = [];
  const allGaps = [];
  const allEdges = [];
  const b7 = {};
  const b8 = {};
  const b9 = {};
  let pages = 0;
  let dialogs = 0;
  let judged = 0;
  let tbJudged = 0;
  const failed = [];
  let vw = 1440;

  // B7/B8 必须在这里落桶：上一版 CENSUS 采了 fieldRows，却从没写进 b7 ——
  // 那条门禁恒为 0，"绿"来自没接线，不是来自没有违例。
  const bucket = (dst, label, list) => {
    for (const line of list || []) {
      const i = line.indexOf(' → ');
      const k = `${label} ${i < 0 ? line : line.slice(0, i)}`;
      const v = i < 0 ? '命中' : line.slice(i + 3);
      (dst[k] = dst[k] || { values: {}, pages: [] }).values[v] = (dst[k].values[v] || 0) + 1;
      if (!dst[k].pages.includes(label)) dst[k].pages.push(label);
    }
  };

  // 量之前先把页面弄"静"：移开鼠标（弹层是被点开的，指针正停在触发按钮上，
  // hover 过渡还在跑）再等所有 CSS 动画进入非 running 态。
  // B2 的判据是"声明高度 vs 实测外盒"，过渡帧里量到 ±2px 是工装自己的影子，不是页面的缺陷
  // —— 2026-10-02 document-views»编辑 报过 3 个按钮各 -2px，同一份代码隔一次再跑就 0 命中。
  const settle = async (ms) => {
    await page.mouse.move(1, 1);
    await page.waitForTimeout(ms);
    await page.evaluate(() => new Promise((res) => {
      const t0 = Date.now();
      const tick = () => {
        const running = (document.getAnimations ? document.getAnimations() : [])
          .filter((a) => a.playState === 'running').length
          // JS 弹簧不进 getAnimations() 的账：不等它，就会量到"飞行途中"的抽屉，
          // 报出"抽屉跑出视口 9px"这种只有工装自己造成的违例。运动中的层自己挂 data-spring。
          + document.querySelectorAll('[data-spring="running"]').length;
        if (!running || Date.now() - t0 > 3000) return res();
        return requestAnimationFrame(tick);
      };
      return tick();
    })).catch(() => {});
  };

  // coverageOnly：这一趟只用于"组件有没有被渲染出来"的覆盖统计，不参与任何违例判定。
  // 加载态快照必然抓到过渡中的元素（按钮高度还在动、卡片还在淡入），
  // 拿它比"声明高度 vs 实测外盒"会造出只有工装自己造成的假阳性
  // —— 实测把 SNAP_GAP_MS 压到 500 时，document-edit 就凭这条红过一次。
  const take = async (label, coverageOnly) => {
    const res = await page.evaluate(CENSUS);
    vw = res.vw;
    // vw 必须跟着每一行进桶：上面那个全局变量会被最后一次 take() 覆盖，
    // 于是「最后一趟量在 390」会让 1440 页面上的元素全被判成跑出视口（实测造出 5 类假阳性）。
    for (const r of res.rows) allRows.push({ ...r, page: label, vw: res.vw, cov: coverageOnly ? 1 : 0 });
    if (coverageOnly) return;
    for (const g of res.gaps) allGaps.push({ ...g, page: label });
    for (const e of res.edges) allEdges.push({ ...e, page: label });
    bucket(b7, label, res.fieldRows);
    bucket(b8, label, res.alignRows);
    bucket(b9, label, res.toolbars);
    judged += res.judged || 0;
    tbJudged += res.tbJudged || 0;
  };

  const TRIGGER = /新建|新增|添加|编辑|重命名|移动|复制|成员|权限|详情|配置|设置|邀请|上传/;
  const NEVER = /删除|移除|清空|重置|执行|运行|触发|重跑|同步|保存|提交|发送|导出|下载|导入|重建|恢复|应用|退出|登出|索引|升级|开始|停止|启用|禁用/;

  for (const r of TARGETS) {
    try {
      await page.goto(`${BASE}${r.path}`, { waitUntil: 'load', timeout: 30000 });
      // 先量一次"刚落地"的状态：骨架屏与 spinner 只在数据回来之前存在，
      // 等 900ms 再量就永远量不到（ApSkeleton 52 个文件引用却被判"零渲染"就是这么来的）。
      await take(r.name + '»加载态', true);
      await page.waitForTimeout(Number(process.env.SNAP_GAP_MS || 900));
      await page.mouse.move(1, 1);
      await page.waitForTimeout(120);
      pages++;
      await take(r.name);

      // 打开本页会弹的东西，把弹层里的控件也计入普查（否则表单控件整类量不到）
      // 正则要用**源码字符串**传进页面：page.evaluate 只搬函数体，
      // 直接引用 Node 侧的 TRIGGER/NEVER 会在页面里 ReferenceError，
      // 而外层 try 把它吞成"这页没弹层"—— 上一版"打开弹层 0 个"就是这么来的。
      const btns = await page.evaluate((rx) => {
        const T = new RegExp(rx.trigger);
        const N = new RegExp(rx.never);
        return [...document.querySelectorAll('main button, [data-ap-scroll] button')]
          .filter((b) => b.offsetParent && b.querySelectorAll('button').length === 0)
          .map((b) => ({ t: (b.textContent || '').trim().slice(0, 24) }))
          .filter((x) => x.t && T.test(x.t) && !N.test(x.t)).slice(0, 2);
      }, { trigger: TRIGGER.source, never: NEVER.source });
      for (const b of btns) {
        try {
          const handle = page.getByRole('button', { name: b.t }).first();
          await handle.click({ timeout: 3000 });
          await settle(700);
          const opened = await page.evaluate(() => !!document.querySelector('[role=dialog], [data-ap=modal], [data-ap=drawer]'));
          if (!opened) continue;
          dialogs++;
          await take(r.name + '»' + b.t);
          await page.keyboard.press('Escape');
          await page.waitForTimeout(400);
        } catch (e) { console.log(`   （${r.name}»${b.t} 没打开：${String(e).slice(0, 40)}）`); await page.keyboard.press('Escape').catch(() => {}); }
      }
    } catch (e) {
      failed.push(`${r.name}: ${String(e).slice(0, 50)}`);
    }
  }
  // ---- 四个"要走到那个状态才渲染"的补量入口 ----
  // B5 报"零渲染"里有相当一部分不是死码，而是巡检从没到过那个态：
  // 错误页要跳不存在的路由、⌘K 与 ? 要按键、抽屉要窄视口、正文要等文档加载完。
  // 不补这几步，"这个组件没量到"就会被当成"这个组件是死的"去删。
  const extras = [];
  // after：量完再做的事（关弹层、把视口切回桌面档）。
  // 放在 go() 里就会在量之前执行 —— 窄视口抽屉那条以前是先按 Esc 再采，
  // 采到的正是"正在飞出去"的抽屉（x=-9），报成"元素跑出视口"。
  const probe = async (label, go, after) => {
    try {
      await go();
      // 合成探针也必须先 settle 再量。主循环每页都等动画停，这里以前不等，
      // 于是量到的是入场过渡中途的帧 —— 实测快捷键面板里一个 28px 的按钮量出 25.8px
      // （28 × 0.92，正是弹层 scale 入场跑到一半），B2 把它报成盒模型违例。
      // 干净复现（同样等 900ms，但前一个弹层没有叠着收）0 命中、0 动画在跑，
      // 说明这是工装自己造出来的影子，不是页面缺陷。
      await settle(200);
      await take(label);
      if (after) await after();
    } catch (e) {
      extras.push(`${label}: ${String(e).slice(0, 60)}`);
    }
  };
  await probe('错误页(404)', async () => {
    await page.goto(`${BASE}/route-that-does-not-exist-404`, { waitUntil: 'load', timeout: 30000 });
    await page.waitForTimeout(1200);
  });
  await probe('全局搜索⌘K', async () => {
    await page.goto(`${BASE}/dashboard`, { waitUntil: 'load', timeout: 30000 });
    await page.waitForTimeout(1500);
    await page.keyboard.press('Control+k');
    await page.waitForTimeout(900);
  });
  await probe('快捷键面板?', async () => {
    await page.keyboard.press('Escape');
    await page.waitForTimeout(400);
    await page.keyboard.press('Control+/');
    await page.waitForTimeout(900);
  });
  await probe('文档正文', async () => {
    await page.keyboard.press('Escape');
    await page.waitForTimeout(400);
    await page.goto(`${BASE}/space/1/doc/356`, { waitUntil: 'load', timeout: 30000 });
    await page.waitForTimeout(2600);
  });
  await probe('窄视口抽屉', async () => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`${BASE}/dashboard`, { waitUntil: 'load', timeout: 30000 });
    await page.waitForTimeout(1500);
    await page.locator('[data-ap="nav-bar"] button').first().click({ timeout: 3000 });
    await settle(200);   // 抽屉是 JS 弹簧推到位的，等它落定再量（getAnimations 看不见它）
  }, async () => {
    await page.keyboard.press('Escape');
    await page.setViewportSize({ width: 1440, height: 900 });
  });
  if (extras.length) console.log(`   （补量入口有 ${extras.length} 个没走到：${extras.slice(0, 3).join(' | ')}）`);

  await browser.close();

  const byGroup = {};
  for (const row of allRows) (byGroup[row.g] = byGroup[row.g] || []).push(row);

  // B5 覆盖清单
  const measured = new Set(Object.keys(byGroup).map((g) => g.split('/')[0]));
  const absorbedHit = [...ABSORBED.entries()].filter(([, host]) => measured.has(host));
  const never = [...AP_COMPONENTS.keys()].filter((k) => !measured.has(k) && !ABSORBED.has(k));
  // "零渲染"必须分三类，一锅端会把死码和数据依赖混成同一句话：
  //   无模板消费者 = 真死码；挂在 v-if/v-show 上 = 数据或状态依赖（当前库里没行就是看不见）；
  //   两者都不是 = 需要特定交互才出现（键位、抽屉、路由态）——那是巡检该继续补入口，不是组件该删。
  const vueSources = [];
  const walkVue = (d) => {
    for (const f of fs.readdirSync(d, { withFileTypes: true })) {
      const fp = path.join(d, f.name);
      if (f.isDirectory()) { if (!/node_modules/.test(f.name)) walkVue(fp); }
      else if (f.name.endsWith('.vue')) vueSources.push([f.name, fs.readFileSync(fp, 'utf8')]);
    }
  };
  walkVue(path.join(here, '..', 'src'));
  const neverReason = (slug) => {
    const tag = AP_COMPONENTS.get(slug).replace('.vue', '');
    const hits = [];
    for (const [file, text] of vueSources) {
      if (file === tag + '.vue') continue;
      for (const m of text.matchAll(new RegExp(`<${tag}\\b([^>]*)>`, 'g'))) {
        hits.push([file, m[1] || '']);
      }
    }
    if (!hits.length) return { kind: '死码（模板里 0 个消费者）', where: '' };
    const conditional = hits.filter(([, attrs]) => /\bv-if=|\bv-show=/.test(attrs));
    if (conditional.length === hits.length) {
      return { kind: `条件渲染（${hits.length} 处全挂 v-if/v-show，数据或状态为空时不出现）`, where: hits.map(([f]) => f).slice(0, 2).join(' ') };
    }
    return { kind: `需特定交互（${hits.length} 处消费者、其中 ${hits.length - conditional.length} 处无条件）`, where: hits.map(([f]) => f).slice(0, 2).join(' ') };
  };

  const b1 = {};
  const b2 = {};
  const b6 = {};
  for (const [g, list] of Object.entries(byGroup)) {
    const tag = list[0].tag;
    // textarea 不参与比高：它的高度由 rows/内容决定（实测 65px 与 254px 都正常），
    // 跨类比会在每个带描述框的页面上报假阳性 —— 与 check:dialog 的 D2 同口径。
    if (CONTROL_TAGS.has(tag) && tag !== 'textarea') {
      for (const row of list) {
        if (row.cov) continue;
        const h = Math.round(row.h);
        // 容差 ±2px：ApButton 带 transform/height 过渡，量到 33/35 多半是过渡帧而不是
        // 设计值离档（schema 页那个 33px 复测就不存在了）。
        const onLadder = LADDER_H.some((x) => Math.abs(h - x) <= 2);
        if (!onLadder) {
          const k = g;
          (b1[k] = b1[k] || { values: {}, pages: [] }).values[h + 'px'] = (b1[k].values[h + 'px'] || 0) + 1;
          if (b1[k].pages.length < 4 && !b1[k].pages.includes(row.page)) b1[k].pages.push(row.page);
        }
        // B2：border-box 下，声明 height 应等于实测外盒；content-box 时 border/padding 会额外撑出
        const declaredPx = parseFloat(row.declared);
        if (!Number.isNaN(declaredPx) && row.declared.endsWith('px')) {
          const drift = Math.round(row.h) - Math.round(declaredPx);
          if (Math.abs(drift) > 1) {
            const kk = g + (row.box === 'border-box' ? ' (border-box)' : ' (content-box)');
            (b2[kk] = b2[kk] || { values: {}, pages: [] }).values[`实测比声明${drift > 0 ? '+' : ''}${drift}px`] = (b2[kk].values[`实测比声明${drift > 0 ? '+' : ''}${drift}px`] || 0) + 1;
            if (b2[kk].pages.length < 4 && !b2[kk].pages.includes(row.page)) b2[kk].pages.push(row.page);
          }
        }
      }
    }
    for (const row of list) {
      if (row.cov) continue;
      const limit = row.vw || vw;
      if ((row.right > limit + 2 || row.x < -2) && !row.clipX) {
        const kk = g;
        (b6[kk] = b6[kk] || { values: {}, pages: [] }).values[`x=${row.x} right=${row.right}`] = (b6[kk].values[`x=${row.x} right=${row.right}`] || 0) + 1;
        if (b6[kk].pages.length < 4 && !b6[kk].pages.includes(row.page)) b6[kk].pages.push(row.page);
      }
    }
  }

  const b3 = {};
  for (const e of allEdges) {
    if (SPACE.includes(Math.round(e.diff))) continue;
    const k = e.g;
    const val = `左边缘偏 ${e.diff}px`;
    (b3[k] = b3[k] || { values: {}, pages: [] }).values[val] = (b3[k].values[val] || 0) + 1;
    if (b3[k].pages.length < 4 && !b3[k].pages.includes(e.page)) b3[k].pages.push(e.page);
  }
  const b4 = {};
  for (const g of allGaps) {
    if (!SPACE.includes(Math.round(g.gap))) {
      const k = g.g;
      (b4[k] = b4[k] || { values: {}, pages: [], samples: [] }).values[`${g.gap}px`] = (b4[k].values[`${g.gap}px`] || 0) + 1;
      if (b4[k].pages.length < 4 && !b4[k].pages.includes(g.page)) b4[k].pages.push(g.page);
      // 带上"是哪一段文字"：复核清单如果只给数字，人还得回页面上一处处找，等于没清单
      if (b4[k].samples.length < 3 && g.text) b4[k].samples.push(`${g.page}·${g.text}`);
    }
  }

  console.log(`量测页面 = ${pages} / ${TARGETS.length}，打开弹层 = ${dialogs} 个`);
  console.log(`量到元素 = ${allRows.length}，分组 = ${Object.keys(byGroup).length} 类，B7/B8 真正比过 = ${judged} 行\n`);

  console.log(`== B5 覆盖清单：Ap* 组件 ${AP_COMPONENTS.size} 个 = 自己名下量到 ${measured.size - absorbedHit.length === 0 ? 0 : [...AP_COMPONENTS.keys()].filter((k) => measured.has(k)).length} + 归并在宿主名下 ${absorbedHit.length} + 真没渲染 ${never.length} ==`);
  console.log(`   （mixin 是"子组件先 mounted 会赢"，所以模板根是另一个 Ap* 的组件永远没有自己的 data-ap，见下）`);
  for (const [slug, host] of absorbedHit) console.log(`   ≈ ${AP_COMPONENTS.get(slug)} 归并在 data-ap=${host} 名下${measured.has(slug) ? '（自己也有标记）' : ''}`);
  for (const [slug, host] of ABSORBED) if (!measured.has(host)) console.log(`   ? ${AP_COMPONENTS.get(slug)} 归并在 data-ap=${host} 名下，但宿主本身没量到 —— 这条是真缺口`);
  for (const k of never) {
    const why = neverReason(k);
    console.log(`   ✗ ${AP_COMPONENTS.get(k)} (data-ap=${k}) —— 巡检没量到：${why.kind}${why.where ? ` 于 ${why.where}` : ''}`);
  }
  const counts = Object.entries(byGroup).map(([g, l]) => [g, l.length]).sort((a, b) => b[1] - a[1]);
  console.log(`   量到的类（前 ${MAX}，按实例数）：`);
  for (const [g, n] of counts.slice(0, MAX)) console.log(`   ✓ ${g.padEnd(28)} ${n} 个实例`);

  const show = (title, bucket, note) => {
    const keys = Object.keys(bucket);
    console.log(`\n== ${title}：${keys.length} 类命中 ==`);
    for (const k of keys.sort((a, b) => Object.keys(bucket[b].values).length - Object.keys(bucket[a].values).length).slice(0, MAX)) {
      const vs = Object.entries(bucket[k].values).sort((a, b) => b[1] - a[1]).slice(0, 5);
      console.log(`   ${k.padEnd(26)} ` + vs.map(([v, n]) => `${v}×${n}`).join('  ').slice(0, 92) + `   页:${bucket[k].pages.slice(0, 3).join(',')}`);
      // 命中要能自己说出是哪一段（B2 同一条教训：只给数字，人得回页面复现三次才找到）
      if (bucket[k].samples?.length) console.log(`      ↳ ${bucket[k].samples.join('   ')}`.slice(0, 150));
    }
    if (note) console.log(`   ${note}`);
  };
  show('B1 控件实测高度不在控制天阶 24/28/32/40/44', b1);
  show('B2 盒模型：声明高度与实测外盒对不上', b2, '—— content-box 下 border/padding 会额外撑高，全站必须 border-box');
  // 命中要能自己说清是哪个元素。以前只有"button (border-box) 实测比声明-2px×1 页:某某"，
  // 拿着这行去手工复现三次都量不到（元素只在巡检那一刻的页面状态里存在），
  // 等于把一条无法追查的违例丢给人。现在把声明值、实测值与坐标一起打出来。
  // 过滤条件必须和上面落桶的口径一致（只算控件标签、且分组键就是那个桶名）——
  // 第一版没加这两条，把 SVG <rect>（按 viewBox 缩放，声明 6px 实渲 3.5px）全倒了出来，
  // 几千行噪音里找不到真凶，等于没有这条详情。
  if (Object.keys(b2).length) {
    const bucketKeys = new Set(Object.keys(b2));
    for (const row of allRows) {
      if (row.cov || !CONTROL_TAGS.has(row.tag) || row.tag === 'textarea') continue;
      const declaredPx = parseFloat(row.declared);
      if (Number.isNaN(declaredPx) || !String(row.declared).endsWith('px')) continue;
      const drift = Math.round(row.h) - Math.round(declaredPx);
      if (Math.abs(drift) <= 1) continue;
      const key = row.g + (row.box === 'border-box' ? ' (border-box)' : ' (content-box)');
      if (!bucketKeys.has(key)) continue;
      console.log(`   ↳ ${row.page} · ${key} <${row.tag}> 声明 ${row.declared} 实测 ${row.h}px（差 ${drift > 0 ? '+' : ''}${drift}）`
        + ` @${row.x},${row.y} 父=${row.parentLabel}`);
    }
  }
  show('B3 同容器左边缘差值不落尺（人工复核清单，不计入门禁）', b3);
  show('B4 兄弟纵向声明间距不落尺（人工复核清单，不计入门禁）', b4);
  show('B6 元素横向跑出视口', b6);
  show('B7 同一行里混用「label 在上」与「label 同行」两种字段形态', b7,
    '—— 混用就是"都不在一条水平上"：label 高低不齐、控件基线也不齐。弹层侧同名断言是 D4');
  show('B8 同形态行里 label/控件没落在同一条水平线上（flex 行 + 网格视觉行）', b8,
    '—— 全同行形态比 label 中线；全在上形态按行的对齐方式比（底对齐比控件底边、center 比中线、网格 stretch 比顶边）。B7 只管混形态，抓不到"三组都同行但一组的值只有 19px 高"');
  show('B9 工具条在 1440 下折成多行', b9,
    `—— 判过的控件行 ${tbJudged} 条；行内出现 100% 宽的元素（横分隔线、width:100% 的下拉）必然把行切断，静态门一条都不报`);

  if (failed.length || pages === 0 || allRows.length === 0) {
    console.log(`\n✗ 有页面没量到（失败 ${failed.length}）—— 上面所有清单与"0 命中"都不作数：${failed.slice(0, 3).join(' | ')}`);
    process.exitCode = 1;
  }
  // 只有 B1/B2/B6 是硬门禁。B3/B4 是**人工复核清单**：1–9px 的左边缘差与"非档位"
  // 边距里，绝大多数是嵌套容器的边框+内边距、或圆点为对齐文本的光学偏移
  // （ApTimelineItem 紧凑档的 marginTop: 3px 就是 —— xxs 是 2px、xs 是 4px，
  //  取任一头都会让圆点偏心，它本来就不该落在布局尺上）。
  // 把它们计入退出码，等于把工装永久调成红色，那比不量更糟。
  const gate = Object.keys(b1).length + Object.keys(b2).length + Object.keys(b6).length + Object.keys(b7).length + Object.keys(b8).length + Object.keys(b9).length;
  console.log(`\n几何门禁合计 ${gate} 类；覆盖缺口 ${never.length} 个组件（B5 是清单，不当门禁，但必须看见）`);
  if (gate) process.exitCode = 1;
  fs.writeFileSync(path.join(here, '..', 'box-census.json'), JSON.stringify(
    { total: allRows.length, groups: Object.fromEntries(counts), never, b1, b2, b3, b4, b6, b7, b8, b9 }, null, 1));
  console.log('明细已写入 web/box-census.json');
};

// 反向验证：注入 B1/B2/B3/B4/B6 各一条明显违例，五条都必须抓到。
const PROBE = () => {
  const box = document.createElement('div');
  box.setAttribute('data-ap', 'zzbox');
  box.style.cssText = 'position:absolute;left:40px;top:120px;width:300px;padding:0';
  // 必须 display:block：按钮默认 inline-block，并排放就没有"纵向兄弟间距"可量，
  // 上一版探针因此让 B3/B4 两条断言看着像失灵。
  box.innerHTML = '<button style="display:block;box-sizing:content-box;height:37px;padding:6px;border:3px solid #999;width:80px">a</button>'
    + '<button style="display:block;height:32px;width:80px;margin-top:7px;margin-left:3px">b</button>'
    + '<button style="display:block;height:32px;width:80px;margin-top:7px">c</button>'
    + '<div style="position:absolute;left:1300px;top:0;width:400px;height:20px;background:#ccc"></div>'
    // B7 探针：一行两组，一组 label 与控件同行、一组 label 在控件上方
    + '<div data-ap="zzshape" style="display:flex;align-items:center;gap:8px;width:520px;height:48px;background:#ddd">'
    + '<div style="display:flex;align-items:center;gap:4px"><span>甲</span><input style="width:90px;height:32px"></div>'
    + '<div style="display:flex;flex-direction:column;gap:2px"><span>乙</span><input style="width:90px;height:20px"></div>'
    + '</div>'
    // B8 探针：两组**都是**「label 与控件同行」，B7 因此抓不到它；差别在值的高度
    // —— 一个 40px 一个 19px，行按 flex-end 底对齐就把矮组整条拽下去，
    // label 中线错开 ~10px。文档协作中心「空间/文档/当前审批」那行正是这个形态。
    + '<div data-ap="zzline" style="display:flex;align-items:flex-end;gap:8px;width:520px;height:60px;background:#eee">'
    + '<div style="display:flex;align-items:center;gap:4px"><span>甲</span><input style="width:90px;height:40px"></div>'
    + '<div style="display:flex;align-items:center;gap:4px"><span>乙丙丁</span><span style="display:inline-block;height:19px">值</span></div>'
    + '</div>'
    // B8 网格探针：display:grid 一行两个「label 在上」字段，第二个被 margin-top 推下 8px。
    // 网格默认 stretch，判据是控件**顶边**；上一版只认 flex 行，表单网格整类没看见。
    + '<div data-ap="zzgrid" style="display:grid;grid-template-columns:1fr 1fr;gap:8px;width:520px;background:#efd">'
    + '<div style="display:flex;flex-direction:column;gap:2px"><span>丙丁戊</span><input style="width:120px;height:40px"></div>'
    + '<div style="display:flex;flex-direction:column;gap:2px;margin-top:8px"><span>己庚辛</span><input style="width:120px;height:40px"></div>'
    + '</div>'
    // B9 探针：三枚按钮本来一行放得下（520px 里 180px），中间插一条 100% 宽的横分隔线
    // 就把行切断了 —— 媒体页当时的真实成因。分隔线只有 1px 高，会被"子项高度 > 8"筛掉，
    // 但它造成的错位仍在按钮底边上，所以这条探针同时证明"筛掉细线不影响判据"。
    + '<div data-ap="zzwrap" style="display:flex;align-items:flex-end;flex-wrap:wrap;gap:8px;width:520px;background:#fee">'
    + '<button style="height:32px;width:60px">甲</button>'
    + '<button style="height:32px;width:60px">乙</button>'
    + '<div style="width:100%;height:1px;background:#999"></div>'
    + '<button style="height:32px;width:60px">丙</button>'
    + '</div>';
  document.body.appendChild(box);
  // 反向探针一：**不同父容器**里的同名组，纵向堆叠、x 差 9px。
  // 兄弟桶按"组名#标签"合并的那版会把它们当兄弟，报出假的"左边缘不落尺"
  // （B3 原来 8 类里 6 类就是这么来的）。两个容器本身 x 相同，不构成真错位。
  const crossA = document.createElement('div');
  crossA.setAttribute('data-ap', 'zzcross');
  crossA.style.cssText = 'position:absolute;left:40px;top:520px;width:200px';
  crossA.innerHTML = '<div style="height:20px;width:120px;background:#cdc">甲</div>';
  const crossB = document.createElement('div');
  crossB.setAttribute('data-ap', 'zzcross');
  crossB.style.cssText = 'position:absolute;left:40px;top:560px;width:200px';
  crossB.innerHTML = '<div style="height:20px;width:120px;margin-left:9px;background:#cdc">乙</div>';
  document.body.appendChild(crossA);
  document.body.appendChild(crossB);
  // 反向探针二：1px 是发丝分隔/half-leading 取整，不是间距（布局尺最细一档 2px）。
  const hair = document.createElement('div');
  hair.setAttribute('data-ap', 'zzhair');
  hair.style.cssText = 'position:absolute;left:40px;top:640px;width:200px';
  hair.innerHTML = '<button style="display:block;height:24px;width:80px">a</button>'
    + '<button style="display:block;height:24px;width:80px;margin-top:1px">b</button>';
  document.body.appendChild(hair);
  // 反向探针三：整块居中的两段文字，左边缘差由字数决定，不是错位。
  const center = document.createElement('div');
  center.setAttribute('data-ap', 'zzcenter');
  center.style.cssText = 'position:absolute;left:40px;top:700px;width:300px;text-align:center';
  center.innerHTML = '<p style="margin:0 0 4px">短标题</p><p style="margin:0">这是一段明显更长的说明文字</p>';
  document.body.appendChild(center);
  return { crossParents: [crossA, crossB].map((n) => n.getAttribute('data-ap')), hair };
};

const selftest = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  await loginForm(page, BASE, USER, PASS);
  await page.goto(`${BASE}/dashboard`, { waitUntil: 'load', timeout: 30000 });
  await page.waitForTimeout(1500);
  await page.evaluate(PROBE);
  const res = await page.evaluate(CENSUS);
  await browser.close();
  const L = [2, 4, 8, 12, 16, 20, 24, 32, 40, 48, 64, 80];
  const rows = res.rows.filter((r) => r.g.includes('zzbox'));
  const hs = rows.map((r) => Math.round(r.h));
  const bw = rows.find((r) => r.box === 'content-box');
  // 断言按**实测值**判，不写死像素坐标：写死就要跟着探针的位置一起改，
  // 探针一动就误报成"判据失灵"。
  const btns = rows.filter((r) => r.tag === 'button');
  const xDiffs = btns.slice(1).map((b, i) => Math.abs(b.x - btns[i].x));
  const gaps = btns.slice(1).map((b) => b.gap + b.mt);
  const checks = [
    [`B1 抓到离档控件高度（实测 ${[...new Set(hs)].join('/')}）`, hs.some((h) => ![24, 28, 32, 40, 44].includes(h))],
    [`B2 抓到 content-box 的盒模型漂移（声明 37px，实测 ${bw ? Math.round(bw.h) : '—'}px）`, !!bw && Math.abs(bw.h - 37) > 1],
    [`B3 抓到不落尺的左边缘错位（堆叠兄弟 x 差 ${xDiffs.join('/')}）`, xDiffs.some((d) => d > 0 && d < 12 && !L.includes(d))],
    // 反向：不同父元素的同名组不许被当兄弟（把父桶键改回"组名#标签"，这条立刻红）
    [`B3 不把跨父容器的同名组当兄弟`, !(res.edges || []).some((e) => String(e.g).includes('zzcross'))],
    // 反向：1px 发丝不算间距
    [`B4 不把 1px 发丝当间距`, !(res.gaps || []).some((g) => String(g.g).includes('zzhair'))],
    // 反向：居中排版的文本不比左边缘（ApEmpty 那种）
    [`B3 不把居中文本的左边缘差当错位`, !(res.edges || []).some((e) => String(e.g).includes('zzcenter'))],
    [`B4 抓到不落尺的声明间距（margin+rowGap ${[...new Set(gaps)].join('/')}px）`, gaps.some((g) => g > 0 && !L.includes(Math.round(g)))],
    [`B6 抓到跑出视口的元素（right=${Math.max(0, ...rows.map((r) => r.right))} > ${res.vw}）`, rows.some((r) => r.right > res.vw + 2)],
    [`B7 抓到混用字段形态的行（${(res.fieldRows || []).filter((x) => x.includes('zzshape')).join(' | ') || '没抓到'}）`,
      (res.fieldRows || []).some((x) => x.includes('zzshape') && x.includes('inline') && x.includes('stacked'))],
    [`B8 抓到同形态行里 label 不落线（${(res.alignRows || []).filter((x) => x.includes('zzline')).join(' | ') || '没抓到'}）`,
      (res.alignRows || []).some((x) => x.includes('zzline'))],
    [`B8 抓到网格视觉行里控件顶边不齐（${(res.alignRows || []).filter((x) => x.includes('zzgrid')).join(' | ') || '没抓到'}）`,
      (res.alignRows || []).some((x) => x.includes('zzgrid'))],
    [`B9 抓到工具条被 100% 宽元素切成多行（${(res.toolbars || []).filter((x) => x.includes('zzwrap')).join(' | ') || '没抓到'}）`,
      (res.toolbars || []).some((x) => x.includes('zzwrap'))],
  ];
  let bad = 0;
  for (const [k, ok] of checks) { if (!ok) bad++; console.log(`${ok ? '✓' : '✗'} ${k}`); }
  console.log(bad ? `✗ ${bad} 条没抓到自造违例——判据或采集写错了` : `${checks.length} 条几何判据都抓到了自造违例`);
  process.exitCode = bad ? 1 : 0;
};

if (argv.includes('--selftest')) selftest(); else run();
