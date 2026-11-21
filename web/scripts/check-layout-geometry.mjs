// 元素级几何审计：字号天阶由 check-ui-consistency 管，这台管"摆得正不正"。
// 十条断言全部读真实渲染盒，不读源码：
//   R1 同一行里的控件高度不一致（"按钮和框不对齐"）
//   R1b 同一行控件垂直中线不齐
//   R2 网格里同一行的卡片不等高
//   R3 文字被截断却没有 title/tooltip（信息就此丢失）
//   R4 控件内文字折行（nowrap 失守，按钮变成两行块）
//   R5 纵向间距不落尺（声明的 row-gap / margin 冒出 10/13/26px 这种凑出来的数）
//   R6 表格数值列的表头与单元格对齐方向相反
//   R7 整页可以横向滚动（窄视口下最常见的布局事故）
//   R8 控件被带 overflow:hidden 的祖先纵向切掉（固定高度工具栏装不下控件）
//   R9 有粘住条却没给滚动容器留 scroll-padding-top（锚点跳转会停在条底下）
//   R10 同一工具条带里并排的分段控件档位不一（32/13px 与 40/17px 同排 = 同一角色两种重量）
// 用法（前端 dev :3000、后端 :3001 需在跑；口令只走环境变量，不落仓库）：
//   SNAP_USER=… SNAP_PASS=… node scripts/check-layout-geometry.mjs [--only=<路由名子串>] [--max=8] [--widths=1440,1024,768,390]
//   SNAP_USER=… SNAP_PASS=… node scripts/check-layout-geometry.mjs --selftest   # 反向验证十条探针
import { chromium } from 'playwright';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { findChromium } from './lib/chromium-path.mjs';
import { pickTargets } from './lib/route-list.mjs';
import { installErroredProbe, loginForm} from './lib/page-state.mjs';

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
const MAX = Number(argOf('max') || 6);
// 多档视口：1440 是主设计稿，1024/768 是常见窄屏，390 触发站点的 isMobile(<641px) 分支
const WIDTHS = (argOf('widths') || '1440').split(',').map((s) => Number(s)).filter(Boolean);
// 默认只采顶部：站内的量测都读布局盒，视口外的元素同样已经排好版，滚下去再量一遍不会多发现问题。
// 留这个开关是给 sticky/滚动相关断言复现用的（--scrolls=0,0.5,1）。
const SCROLLS = (argOf('scrolls') || '0').split(',').map(Number).filter((n) => !Number.isNaN(n));
const TARGETS = await pickTargets(ONLY, here);

const COLLECT = () => {
  const rect = (el) => el.getBoundingClientRect();
  const vis = (el) => {
    const r = rect(el);
    return r.width > 2 && r.height > 2;
  };
  const desc = (el) => {
    const t = (el.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 16);
    const ap = el.getAttribute && el.getAttribute('data-ap');
    const role = el.getAttribute && el.getAttribute('role');
    return `${(el.tagName || '').toLowerCase()}${ap ? `[${ap}]` : role ? `[role=${role}]` : ''} ${JSON.stringify(t)}`;
  };
  const inScope = (el) => !!el.closest('main, [data-ap-scroll]');
  // 渲染出来的 markdown 正文（文档正文、聊天气泡里的 md）是"内容"，不是 UI 布局：
  // 它的段距按 em 走（17px × 1.25 ≈ 21px 就是实测那条"不落尺"），拿 UI 的尺子量必然假阳性。
  // ApMarkdownPreview 的根由全局 mixin 打了 data-ap=markdown-preview，正好当锚点。
  const inProse = (el) => !!el.closest('[data-ap=markdown-preview], .md-preview, .chat-md-content');
  const CONTROL = 'button, input, select, textarea, [role=combobox], [role=spinbutton], [data-ap=button], [data-ap=switch], [data-ap=segmented]';

  const out = { rowHeights: [], rowCenters: [], cardHeights: [], clipped: [], wrapped: [], gaps: [], tableAlign: [], overflowX: [], clippedBy: [], coveredBy: [], segTiers: [], pageScroll: 0, errored: false };

  // ---- R8 控件被会裁剪的祖先切掉（固定高度的工具栏装不下控件）----
  // 只认纵向被切：overflow:hidden 的圆角卡片天天有，但控件正常都在里面待得好好的；
  // 真的被切掉说明"容器高度"和"控件高度"打架了。滚动容器（auto/scroll）是有意的，跳过。
  for (const c of document.querySelectorAll(CONTROL)) {
    if (!vis(c) || !inScope(c) || inProse(c)) continue;
    const r = rect(c);
    // 出流豁免：卡片右上角那类"hover 才现"的绝对定位悬浮层，它的包含块在更外面，
    // 中间那层 overflow:hidden（实体卡 meta 行为了截断长文本而加的）根本不裁它 ——
    // 量出来"24px 按钮被 14px 容器切掉"是假阳性，联系人页 6 条全是这么来的。
    let outOfFlow = false;
    for (let a = c.parentElement; a && a !== document.body; a = a.parentElement) {
      const s = getComputedStyle(a);
      if (outOfFlow) continue;
      if (/auto|scroll/.test(s.overflowY)) break;
      if (s.position === 'absolute' || s.position === 'fixed') { outOfFlow = true; continue; }
      if (!/hidden|clip/.test(s.overflowY)) continue;
      const ar = rect(a);
      // 溢出只有两个方向：控件底边探到容器底边之下，或顶边探到容器顶边之上。
      // 写成 ar.bottom - r.bottom 会把"控件比容器高"算成负数，规则永远不响。
      const lost = Math.max(r.bottom - ar.bottom, ar.top - r.top);
      if (lost > 2) {
        out.clippedBy.push({ ctl: desc(c), by: desc(a), lost: Math.round(lost), ctlH: Math.round(r.height), boxH: Math.round(ar.height) });
      }
      break;
    }
  }

  // ---- R9 有"钉在容器顶边的粘住条"却没给该容器留 scroll-padding-top ----
  // 锚点跳转（文档目录、标题 #anchor）会把目标停在滚动容器顶端；容器顶上压着一条
  // sticky 工具栏时，目标就永远藏在条底下。判据收窄到三件事同时成立：
  // 容器真的会滚、里面有钉在顶边（top<=8）的粘住条、该条不在表格里
  // （ApTable 的 sticky 表头也满足"粘住"，但锚点永远不会跳进表格，算进来就是满屏假阳性）。
  for (const sc of document.querySelectorAll('main *, [data-ap-scroll], main')) {
    const scs = getComputedStyle(sc);
    if (!/auto|scroll/.test(scs.overflowY)) continue;
    if (sc.scrollHeight <= sc.clientHeight + 8) continue;
    let barH = 0;
    for (const el of sc.querySelectorAll('*')) {
      const s2 = getComputedStyle(el);
      if (s2.position !== 'sticky' && s2.position !== 'fixed') continue;
      if (parseFloat(s2.top) > 8) continue;
      if (el.closest('table')) continue;
      const r = rect(el);
      if (r.height < 8 || r.height > innerHeight * 0.5 || r.width < 40) continue;
      if (s2.visibility === 'hidden' || +s2.opacity === 0) continue;
      barH = Math.max(barH, Math.round(r.height));
    }
    if (!barH) continue;
    const pad = parseFloat(scs.scrollPaddingTop) || 0;
    if (pad + 1 < barH) out.coveredBy.push({ bar: barH, sc: desc(sc), pad: Math.round(pad) });
  }

  // ---- R7 横向溢出：整页能左右滚就是问题，容器自己横向滚不是 ----
  // 只认 documentElement 的溢出量：真的横向滚动容器（表格、走马灯）自己消化了宽度，
  // 页面不会跟着左右滚。归因取"父节点没越界、自己越界"的那一层，才是肇事元素；
  // 不去猜某个 overflow-x:auto 是"有意的滚动区"还是"顺带算出来的 auto"。
  const de = document.documentElement;
  out.pageScroll = Math.max(0, de.scrollWidth - de.clientWidth - 1);
  if (out.pageScroll > 0) {
    for (const el of document.querySelectorAll('body *')) {
      const r = el.getBoundingClientRect();
      if (r.width < 2 || r.height < 2 || r.right <= innerWidth + 1) continue;
      const pr = el.parentElement && el.parentElement.getBoundingClientRect();
      if (pr && pr.right > innerWidth + 1) continue;
      out.overflowX.push({ el: desc(el), over: Math.round(r.right - innerWidth) });
      if (out.overflowX.length >= 3) break;
    }
    out.overflowX.push({ el: `(整页横向可滚 ${out.pageScroll}px)`, over: out.pageScroll });
  }

  // ---- R1 同一行控件：高度 / 垂直中线 ----
  // 高度只比"受控件高度阶梯约束"的那些（按钮/输入/下拉/文本域）：开关、分段项、
  // 复选框天生比按钮矮，把它们混进同一组会比出一堆"按钮 32 vs 开关 24"的假阳性。
  // 中线不齐则对所有可交互控件成立，所以两条用不同的集合。
  // "同一行"必须按矩形纵向重叠判定，不能只按共同父节点：工具栏在窄视口会 flex-wrap，
  // 两行按钮同属一个父节点，按父节点分组会比出"中线偏 72px"的假阳性。
  const LADDER_CTRL = 'button, input:not([type=checkbox]):not([type=radio]), select, textarea, [data-ap=button]';
  const rows = new Map();
  for (const c of document.querySelectorAll(CONTROL)) {
    if (!vis(c) || !inScope(c) || inProse(c)) continue;
    if (c.closest('td')) continue; // 表格行内控件由 R6 管，且列宽压缩会假报高度
    const p = c.parentElement;
    if (!p) continue;
    const pcs = getComputedStyle(p);
    const isRow = (pcs.display === 'flex' || pcs.display === 'inline-flex') ? /^row/.test(pcs.flexDirection) : pcs.display === 'grid';
    if (!isRow) continue;
    if (!rows.has(p)) rows.set(p, []);
    rows.get(p).push(c);
  }
  const splitLines = (list) => {
    const lines = [];
    for (const c of [...list].sort((a, b) => rect(a).top - rect(b).top)) {
      const r = rect(c);
      const line = lines.find((l) => {
        const a = rect(l[0]);
        return Math.min(a.bottom, r.bottom) - Math.max(a.top, r.top) > 0.5 * Math.min(a.height, r.height);
      });
      if (line) line.push(c); else lines.push([c]);
    }
    return lines;
  };
  for (const [p, list] of rows) {
    for (const line of splitLines(list)) {
      if (line.length < 2) continue;
      const sized = line.filter((c) => c.matches(LADDER_CTRL));
      const hs = [...new Set(sized.map((c) => Math.round(rect(c).height)))].sort((a, b) => a - b);
      const centers = line.map((c) => { const r = rect(c); return (r.top + r.bottom) / 2; });
      const spread = Math.max(...centers) - Math.min(...centers);
      if (hs.length > 1 && hs[hs.length - 1] - hs[0] > 2) {
        out.rowHeights.push({ row: desc(p), items: sized.map((c) => `${Math.round(rect(c).height)}:${desc(c)}`).slice(0, 4).join(' | ') });
      } else if (spread > 2) {
        out.rowCenters.push({ row: desc(p), spread: Math.round(spread), items: line.map((c) => desc(c)).slice(0, 4).join(' | ') });
      }
    }
  }

  // 同一"横排"的判定用矩形纵向重叠，不用 top 相等：
  // 只按 top 分组会把侧栏 nav 的两个分组（820 / 1415 高）当成"同一行卡片"报出来。
  const inSameRow = (a, b) => {
    const ov = Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top);
    const sideBySide = Math.abs(a.left - b.left) > 8;
    return ov > 0.5 * Math.min(a.height, b.height) && sideBySide;
  };
  const rowGroups = (kids) => {
    const groups = [];
    for (const k of kids) {
      const r = rect(k);
      const g = groups.find((arr) => inSameRow(rect(arr[0]), r));
      if (g) g.push(k); else groups.push([k]);
    }
    return groups;
  };

  // ---- R2 网格同一行卡片等高 ----
  for (const g of document.querySelectorAll('main *, [data-ap-scroll] *')) {
    const cs = getComputedStyle(g);
    if (!/^grid/.test(cs.display) || !vis(g) || inProse(g)) continue;
    const kids = [...g.children].filter(vis);
    if (kids.length < 2) continue;
    for (const arr of rowGroups(kids)) {
      if (arr.length < 2) continue;
      // 只比"等宽的一排卡片"。侧栏 + 内容区也是 grid 的一行（240 / 1160），
      // 那本来就不是等高关系，混进来只会报出 595px 的荒唐差值。
      const ws = arr.map((k) => Math.round(rect(k).width));
      if (Math.max(...ws) - Math.min(...ws) > 2) continue;
      const hs = arr.map((k) => Math.round(rect(k).height));
      const d = Math.max(...hs) - Math.min(...hs);
      if (d > 2) out.cardHeights.push({ grid: desc(g), diff: d, heights: hs.slice(0, 5).join('/'), first: desc(arr[0]) });
    }
  }

  // ---- R10 同一工具条带里的分段控件必须同档 ----
  // 一条工具条上并排两组互斥选项（左「看哪些」+ 右「怎么看」），档位不同就并排出
  // 32/13px 与 40/17px —— 同一个角色两种视觉重量（实测 /collections 就是这样）。
  // 这条判据只能落在这台：R1 量的是 button/input 的控件档，分段控件的外盒是容器 div，
  // 不在它射程内；check:ui 只在"同字号同字重"里比高度，两组字号不同就永远不相遇。
  {
    const segs = [...document.querySelectorAll('[data-ap=segmented-control]')].filter(vis)
      .sort((a, b) => rect(a).top - rect(b).top);
    const bands = [];
    for (const s of segs) {
      const r = rect(s);
      const band = bands.find((bd) => bd.some((x) => {
        const q = rect(x);
        return q.bottom > r.top - 6 && q.top < r.bottom + 6;
      }));
      if (band) band.push(s);
      else bands.push([s]);
    }
    const fontOf = (s) => {
      const a = s.querySelector('[aria-selected=true], [aria-checked=true]');
      return a ? getComputedStyle(a).fontSize : '';
    };
    for (const band of bands) {
      if (band.length < 2) continue;
      const hs = [...new Set(band.map((s) => Math.round(rect(s).height)))];
      const fs = [...new Set(band.map(fontOf))].filter(Boolean);
      if (hs.length > 1 || fs.length > 1) {
        out.segTiers.push({
          heights: hs.join('/'),
          fonts: fs.join('/'),
          items: band.map((s) => `${desc(s).slice(0, 30)}=${Math.round(rect(s).height)}px/${fontOf(s)}`).join('  |  '),
        });
      }
    }
  }

  // ---- R3 截断无提示 ----
  const hasHint = (el) => {
    for (let n = el, i = 0; n && i < 3; n = n.parentElement, i++) {
      if (n.getAttribute && (n.getAttribute('title') || n.getAttribute('aria-label') || n.hasAttribute('data-ap-tooltip'))) return true;
    }
    return false;
  };
  for (const el of document.querySelectorAll('main *, [data-ap-scroll] *')) {
    if (el.children.length > 0) continue;
    const t = (el.textContent || '').trim();
    if (t.length < 4) continue;
    const cs = getComputedStyle(el);
    if (!/hidden|clip/.test(cs.overflow) && cs.textOverflow !== 'ellipsis') continue;
    if (el.scrollWidth <= el.clientWidth + 1) continue;
    if (!vis(el) || hasHint(el) || inProse(el)) continue;
    out.clipped.push({ text: t.slice(0, 20), lost: el.scrollWidth - el.clientWidth });
  }

  // ---- R4 控件文字折行 ----
  // 不能用 clientHeight / line-height 判：32px 的 small 按钮行高 13px 是正常值，
  // 那样会把每个按钮都报成折行。直接量文本自己占了几行（只取文本节点，绕开图标）。
  const textLines = (el) => {
    const tops = new Set();
    for (const n of el.childNodes) {
      if (n.nodeType !== 3 || !(n.textContent || '').trim()) continue;
      const r = document.createRange();
      r.selectNodeContents(n);
      for (const box of r.getClientRects()) if (box.height > 1) tops.add(Math.round(box.top));
    }
    return tops.size;
  };
  for (const b of document.querySelectorAll(CONTROL)) {
    if (!vis(b) || !inScope(b) || inProse(b)) continue;
    if (!(b.textContent || '').trim()) continue;
    const lines = textLines(b);
    if (lines > 1) out.wrapped.push({ h: Math.round(rect(b).height), lines, text: (b.textContent || '').trim().replace(/\s+/g, ' ').slice(0, 16) });
  }

  // ---- R5 纵向间距落尺 ----
  // 量"声明的间距"而不是两个矩形的距离：矩形距离里混着行高的 half-leading，
  // 实测同一张 ApEntityCard 声明 24px 却量出 26px，照单去改就会把对的改成错的。
  // 尺子取语义档（不是 AppleSpace 的像素键表——那张表本身就有 9/14/17/18px 这类
  // 历史凑数，拿它当尺等于不判）。40px 是注册过的整档，算落尺。
  const LADDER = [2, 4, 8, 12, 16, 20, 24, 32, 40, 48, 64, 80];
  const offLadder = (g) => LADDER.every((v) => Math.abs(g - v) > 0.5);
  for (const p of document.querySelectorAll('main *, [data-ap-scroll] *')) {
    const cs = getComputedStyle(p);
    const flexCol = /^flex/.test(cs.display) && /^col/.test(cs.flexDirection);
    const blocky = flexCol || cs.display === 'grid' || cs.display === 'block' || cs.display === 'flow-root';
    if (!blocky || !vis(p) || inProse(p)) continue;
    const declaredGap = parseFloat(cs.rowGap) || 0;
    const kids = [...p.children].filter((k) => {
      if (!(k instanceof HTMLElement)) return false; // SVG 内部的 path 不算布局节点
      const s = getComputedStyle(k);
      return vis(k) && s.position !== 'absolute' && s.position !== 'fixed';
    });
    for (let i = 1; i < kids.length; i++) {
      const own = Math.max(
        parseFloat(getComputedStyle(kids[i - 1]).marginBottom) || 0,
        parseFloat(getComputedStyle(kids[i]).marginTop) || 0,
      );
      const declared = Math.round(flexCol || cs.display === 'grid' ? declaredGap || own : own);
      if (declared < 2 || declared > 80) continue;
      if (offLadder(declared)) out.gaps.push({ gap: declared, parent: desc(p), above: desc(kids[i - 1]), below: desc(kids[i]) });
    }
  }

  // ---- R6 表格数值列对齐方向 ----
  for (const tb of document.querySelectorAll('[data-ap=table] table, main table')) {
    const head = [...tb.querySelectorAll('thead th')];
    const rowsAll = [...tb.querySelectorAll('tbody tr')].slice(0, 12);
    if (!head.length || rowsAll.length < 2) continue;
    head.forEach((th, i) => {
      const hAlign = getComputedStyle(th).textAlign;
      const cells = rowsAll.map((r) => r.children[i]).filter(Boolean);
      const nums = cells.filter((c) => /^[\d.,]+\s*[%％]?$|^\d[\d.,]*\s*(B|KB|MB|GB|ms|s|次|条|人|天)?$/.test((c.textContent || '').trim()));
      if (nums.length < 2) return;
      const aligns = [...new Set(nums.map((c) => {
        const a = getComputedStyle(c).textAlign;
        return a === 'start' ? (hAlign === 'right' ? 'right' : 'left') : a;
      }))];
      if (aligns.length > 1 || (nums.length >= 2 && !['right', 'end'].includes(hAlign) && aligns[0] === 'left' && /数|量|率|次|额|度|比|时间|大小/.test(th.textContent || ''))) {
        out.tableAlign.push({ col: (th.textContent || '').trim().slice(0, 10), head: hAlign, cells: aligns.join('/') });
      }
    });
  }

  out.errored = (window.__apErrored ? window.__apErrored() : false);
  return out;
};

const tally = (arr) => {
  const c = {};
  for (const v of arr) c[v] = (c[v] || 0) + 1;
  return Object.entries(c).sort((a, b) => b[1] - a[1]);
};

// 反向验证：--selftest 在真实页面上逐条注入违例，确认每条规则都抓得到。
// 没有这一步，"73 页 0 处"完全可能只是"探针没量到东西"——本项目门禁最常见的失效方式。
// 注入必须自己把"探针要求的宿主条件"造出来（例如 R1 只认 flex 行里的控件），
// 否则"没变红"到底是探针瞎还是宿主不成立，分不清。宿主页按顺序试，第一个能注入的用。
// 注意：page.evaluate 只序列化被传入的那个函数，所以每个注入体都是自包含的，
// 找不到共享 helper —— 抽出去就会在页面里 ReferenceError。
const INJECT = {
  R1() {
    const btns = [...document.querySelectorAll('main button, [data-ap-scroll] button')]
      .filter((b) => b.getBoundingClientRect().width > 2);
    for (const b of btns) {
      const p = b.parentElement;
      if (!p) continue;
      const sibs = [...p.children].filter((k) => k.tagName === 'BUTTON' && k.getBoundingClientRect().width > 2);
      if (sibs.length < 2) continue;
      p.style.display = 'flex';
      p.style.flexDirection = 'row';
      sibs[0].style.height = '64px';
      return `flex 行里把 ${JSON.stringify(sibs[0].textContent.trim().slice(0, 6))} 抬到 64px（同行还有 ${sibs.length - 1} 个）`;
    }
    return null;
  },
  R1b() {
    const btns = [...document.querySelectorAll('main button, [data-ap-scroll] button')]
      .filter((b) => b.getBoundingClientRect().width > 2);
    for (const b of btns) {
      const p = b.parentElement;
      if (!p) continue;
      const sibs = [...p.children].filter((k) => k.tagName === 'BUTTON' && k.getBoundingClientRect().width > 2);
      if (sibs.length < 2) continue;
      p.style.display = 'flex';
      p.style.flexDirection = 'row';
      p.style.alignItems = 'flex-start';
      sibs[0].style.marginTop = '18px';
      return `同一 flex 行里把 ${JSON.stringify(sibs[0].textContent.trim().slice(0, 6))} 下推 18px（等高但中线错开）`;
    }
    return null;
  },
  R2() {
    for (const g of document.querySelectorAll('main *, [data-ap-scroll] *')) {
      if (!/^grid/.test(getComputedStyle(g).display)) continue;
      const kids = [...g.children].filter((k) => k.getBoundingClientRect().width > 2);
      if (kids.length < 2) continue;
      // 必须是"真的并排"：单列堆叠的网格宽度也相等，但探针按矩形重叠判同行，
      // 拿它当宿主注入 48px 差值是不会变红的（分不清是探针瞎还是宿主不成立）。
      let pair = null;
      for (let i = 0; i < kids.length && !pair; i++) {
        for (let j = i + 1; j < kids.length; j++) {
          const a = kids[i].getBoundingClientRect();
          const b = kids[j].getBoundingClientRect();
          const ov = Math.min(a.bottom, b.bottom) - Math.max(a.top, b.top);
          // 探针还要求"等宽才算同一排卡片"（侧栏+内容区也是并排），注入侧必须同样成立
          if (Math.abs(a.width - b.width) > 2) continue;
          if (Math.abs(a.left - b.left) > 8 && ov > 0.5 * Math.min(a.height, b.height)) { pair = [kids[i], kids[j]]; break; }
        }
      }
      if (!pair) continue;
      // 网格项默认 align-items:stretch —— 只给一张卡写 height，另一张会被行轨拉到同高，
      // 差值当场被"自愈"（这正是 R11 靠 stretch/1fr 保等高的机制）。要造出不等高，
      // 必须先把拉伸关掉，否则测的是"注入没生效"而不是探针。
      g.style.gridAutoRows = 'auto';
      g.style.alignItems = 'start';
      pair[0].style.height = `${Math.round(pair[1].getBoundingClientRect().height) + 48}px`;
      return '关掉拉伸后把并排两张卡里的一张拉高 48px';
    }
    return null;
  },
  R3() {
    for (const el of document.querySelectorAll('main *, [data-ap-scroll] *')) {
      if (el.children.length || (el.textContent || '').trim().length < 12) continue;
      el.style.cssText += ';overflow:hidden;text-overflow:ellipsis;white-space:nowrap;width:60px';
      el.removeAttribute('title');
      return `截断 ${JSON.stringify(el.textContent.trim().slice(0, 10))} 且不给提示`;
    }
    return null;
  },
  R4() {
    for (const b of document.querySelectorAll('main button, [data-ap-scroll] button')) {
      if (!(b.textContent || '').trim()) continue;
      b.style.cssText += ';width:28px;min-width:0;flex-shrink:1;white-space:normal';
      return `把 ${JSON.stringify(b.textContent.trim().slice(0, 8))} 挤到折行`;
    }
    return null;
  },
  R5() {
    for (const p of document.querySelectorAll('main *, [data-ap-scroll] *')) {
      const cs = getComputedStyle(p);
      if (cs.display !== 'block' && cs.display !== 'flex') continue;
      const kids = [...p.children].filter((k) => k.getBoundingClientRect().height > 2);
      if (kids.length < 2) continue;
      kids[1].style.marginTop = '10px';
      return '给块容器第二个子元素写 10px 上边距（不落尺）';
    }
    return null;
  },
  R6() {
    // 不挑"已有数值列"的表：多数表在这台库上是空的，等不到宿主。
    // 直接找一张有 ≥2 行的表，把某一列的两格写成数值并给相反的对齐，
    // 让探针的前置条件由注入自己保证。
    for (const tb of document.querySelectorAll('main table, [data-ap-scroll] table')) {
      const body = [...tb.querySelectorAll('tbody tr')].filter((r) => r.children.length >= 2);
      const head = [...tb.querySelectorAll('thead th')];
      if (body.length < 2 || !head.length) continue;
      const i = Math.min(1, head.length - 1);
      const a = body[0].children[i];
      const b = body[1].children[i];
      a.textContent = '12.50';
      b.textContent = '7.50';
      a.style.textAlign = 'left';
      b.style.textAlign = 'right';
      return `把「${head[i].textContent.trim().slice(0, 6)}」列两格写成数值并给相反对齐`;
    }
    return null;
  },
  R10() {
    // 两颗同带、不同档的分段控件：外盒 48/56、选中项 13px/17px —— 正是 /collections 实测到的那个差
    const mk = (left, h, fs, label) => {
      const d = document.createElement('div');
      d.setAttribute('data-ap', 'segmented-control');
      d.style.cssText = `position:fixed;top:120px;left:${left}px;height:${h}px;width:200px;z-index:9999;background:#777`;
      const a = document.createElement('span');
      a.setAttribute('aria-selected', 'true');
      a.style.fontSize = fs;
      a.textContent = label;
      d.appendChild(a);
      document.body.appendChild(d);
    };
    mk(20, 48, '13px', 'ZZA');
    mk(260, 56, '17px', 'ZZB');
    return '并排放两颗分段控件（48/13px 与 56/17px）';
  },
  R9() {
    const sc = [...document.querySelectorAll('[data-ap-scroll] *, main *')]
      .find((e) => /auto|scroll/.test(getComputedStyle(e).overflowY) && e.scrollHeight > e.clientHeight + 20);
    if (!sc) return null;
    const bar = document.createElement('div');
    bar.style.cssText = 'position:sticky;top:0;height:64px;background:#000';
    sc.insertBefore(bar, sc.firstChild);
    sc.style.scrollPaddingTop = '0px';
    return '在真会滚的容器里插一条 64px、钉在顶边的 sticky 条（scroll-padding 清零）';
  },
  R8() {
    // 造一个固定 24px 高、overflow:hidden 的条，把里面的按钮撑到 44px —— 纵向必被切
    const b = [...document.querySelectorAll('main button, [data-ap-scroll] button')]
      .find((x) => x.getBoundingClientRect().height > 2);
    if (!b) return null;
    const box = document.createElement('div');
    box.style.cssText = 'height:24px;overflow:hidden;display:flex;align-items:flex-start';
    b.parentNode.insertBefore(box, b);
    box.appendChild(b);
    b.style.height = '44px';
    return '把按钮塞进 24px 高、overflow:hidden 的条里（纵向被切 20px）';
  },
  R7() {
    document.body.style.width = 'calc(100vw + 320px)';
    document.body.style.minWidth = 'calc(100vw + 320px)';
    return '把 body 撑到 100vw+320px（整页可横向滚动）';
  },
};

// 宿主页：管理端 CRUD 默认是卡片视图（没有 <table>），所以 R6 必须靠带数据的 AI 子页。
const HOSTS = [
  '/settings', '/spaces', '/ai/settings?section=memory', '/dashboard', '/contacts',
  '/ai/settings?section=ragas-eval', '/ai/settings?section=sql-history', '/users',
];

const selftest = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  await installErroredProbe(page);
  await loginForm(page, BASE, USER, PASS);
  const rules = [
    ['R1', 'rowHeights'], ['R1b', 'rowCenters'], ['R2', 'cardHeights'],
    ['R3', 'clipped'], ['R4', 'wrapped'], ['R5', 'gaps'], ['R6', 'tableAlign'], ['R7', 'overflowX'], ['R8', 'clippedBy'], ['R9', 'coveredBy'],
    ['R10', 'segTiers'],
  ];
  let bad = 0;
  for (const [key, field] of rules) {
    let fired = false;
    let line = '';
    for (const host of HOSTS) {
      await page.goto(`${BASE}${host}`, { waitUntil: 'load', timeout: 30000 });
      await page.waitForTimeout(1200);
      const before = (await page.evaluate(COLLECT))[field].length;
      const how = await page.evaluate(INJECT[key]);
      if (!how) continue;
      const after = (await page.evaluate(COLLECT))[field].length;
      line = `${key.padEnd(4)} ${host}  ${before} → ${after}   ${how}`;
      fired = after > before;
      if (fired) break;
    }
    if (!fired) bad++;
    console.log(`${fired ? '✓' : '✗'} ${line || `${key}  没有任一宿主页可注入`}`);
  }
  await browser.close();
  // 条数不写死：加一条规则却忘了改这句，报表就会一直报"十条"，而实际跑了十一条
  console.log(bad ? `\n✗ ${bad} 条规则注入违例后没有变红——探针写错了，别信它的"0 处"` : `\n${rules.length} 条探针全部抓得到注入的违例`);
  process.exitCode = bad ? 1 : 0;
};

const run = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  await installErroredProbe(page);
  page.on('dialog', (d) => (d.type() === 'beforeunload' ? d.accept() : d.dismiss()).catch(() => {}));
  await loginForm(page, BASE, USER, PASS);

  const all = [];
  for (const w of WIDTHS) {
    await page.setViewportSize({ width: w, height: 900 });
    const rows = [];
    for (const r of TARGETS) {
      try {
        await page.goto(`${BASE}${r.path}`, { waitUntil: 'load', timeout: 30000 });
        await page.waitForTimeout(Number(process.env.SNAP_GAP_MS || 1000));
        for (const frac of SCROLLS) {
          await page.evaluate((f) => {
            const sc = document.querySelector('[data-ap-scroll]') || document.scrollingElement || document.documentElement;
            sc.scrollTop = Math.round((sc.scrollHeight - sc.clientHeight) * f);
          }, frac);
          await page.waitForTimeout(260);
          rows.push({ name: r.name, w, s: frac, ...(await page.evaluate(COLLECT)) });
        }
      } catch (e) {
        rows.push({ name: r.name, w, failed: String(e).slice(0, 80) });
      }
    }
    all.push(...rows);

    // 兜底页十有八九是这趟巡检自己把通用限流（300 次/分钟/IP）打满，不是页面坏了。
    // 等一个窗口过去再复测一遍，否则报告上永远挂着"N 页没量到"，分不清是节奏问题还是缺陷。
    const retry = rows.filter((x) => x.errored);
    if (retry.length) {
      process.stdout.write(`… 等 65s 让限流窗口过去，复测 ${retry.length} 页\n`);
      await page.waitForTimeout(65000);
      for (const x of retry) {
        const t = TARGETS.find((r) => r.name === x.name);
        if (!t) continue;
        await page.goto(`${BASE}${t.path}`, { waitUntil: 'load', timeout: 30000 }).catch(() => {});
        await page.waitForTimeout(Number(process.env.SNAP_GAP_MS || 1000));
        const m = await page.evaluate(COLLECT);
        if (!m.errored) Object.assign(x, m);
      }
    }

    const errored = rows.filter((x) => x.errored && !x.s);
    const allOk = rows.filter((x) => !x.failed && !x.errored);
    // 每页采两个滚动点，但只有 R9 需要全部样本；其余断言取顶部样本，免得一条问题被数两遍
    const ok = allOk.filter((x) => !x.s);
    console.log(`\n########## 视口 ${w}×900 ##########`);
    console.log(`量测页数 = ${ok.length} / ${rows.length}（清单 ${TARGETS.length} 页${ONLY ? `，--only=${ONLY}` : ''}）`);
    if (errored.length) console.log(`✗ 兜底页：${errored.map((x) => x.name).join(', ')}`);

    const show = (title, list, fmt) => {
      console.log(`== ${title}：${list.length} 处 ==`);
      for (const [desc, n] of tally(list.map(fmt)).slice(0, MAX)) console.log(`   ×${String(n).padStart(3)}  ${desc}`);
      console.log('');
    };

    show('R1 同一行控件高度不一致', ok.flatMap((x) => x.rowHeights.map((v) => ({ p: x.name, ...v }))), (v) => `${v.p}  ${v.items}`);
    show('R1b 同一行控件垂直中线不齐（同高但没对齐）', ok.flatMap((x) => x.rowCenters.map((v) => ({ p: x.name, ...v }))), (v) => `${v.p}  偏${v.spread}px  ${v.items}`);
    show('R2 网格同行卡片不等高', ok.flatMap((x) => x.cardHeights.map((v) => ({ p: x.name, ...v }))), (v) => `${v.p}  差${v.diff}px (${v.heights})  ${v.first}`);
    show('R3 文字截断且无提示', ok.flatMap((x) => x.clipped.map((v) => ({ p: x.name, ...v }))), (v) => `${v.p}  少${v.lost}px  ${JSON.stringify(v.text)}`);
    show('R4 控件文字折行', ok.flatMap((x) => x.wrapped.map((v) => ({ p: x.name, ...v }))), (v) => `${v.p}  h=${v.h} ${JSON.stringify(v.text)}`);
    const allGaps = ok.flatMap((x) => x.gaps.map((v) => ({ p: x.name, ...v })));
    const gapHist = tally(allGaps.map((g) => `${g.gap}px`));
    console.log(`== R5 纵向间距不落尺：${allGaps.length} 处（尺子 = ${[2, 4, 8, 12, 16, 20, 24, 32, 40, 48, 64, 80].join('/')}）==`);
    console.log('   ' + (gapHist.map(([v, n]) => `${v}×${n}`).join('  ') || '无'));
    for (const [desc, n] of tally(allGaps.map((g) => `${g.p}  ${g.gap}px：${g.above} ↑ ${g.below}  (容器 ${g.parent})`)).slice(0, MAX)) console.log(`   ×${String(n).padStart(3)}  ${desc}`);
    console.log('');
    show('R6 表格数值列对齐可疑', ok.flatMap((x) => x.tableAlign.map((v) => ({ p: x.name, ...v }))), (v) => `${v.p}  「${v.col}」表头=${v.head} 单元=${v.cells}`);
    show('R9 粘住条未预留 scroll-padding-top', ok.flatMap((x) => (x.coveredBy || []).map((v) => ({ p: x.name, s: x.s, ...v }))), (v) => `${v.p}  粘住条高 ${v.bar}px，${v.sc} 的 scroll-padding-top 只有 ${v.pad}px → 锚点会停在条底下`);
    show('R8 控件被 overflow:hidden 的祖先切掉', ok.flatMap((x) => x.clippedBy.map((v) => ({ p: x.name, ...v }))), (v) => `${v.p}  ${v.ctl} 被 ${v.by} 切 ${v.lost}px（控件 ${v.ctlH} / 容器 ${v.boxH}）`);
    const over = ok.filter((x) => x.pageScroll > 0);
    console.log(`== R7 整页可横向滚动：${over.length} 页 ==`);
    for (const x of over.slice(0, MAX)) {
      console.log(`   ${x.name}  溢出 ${x.pageScroll}px  ` + x.overflowX.map((o) => `${o.el}(+${o.over})`).join(' ; '));
    }
    console.log('');
    show('R10 同一工具条里分段控件档位不一', ok.flatMap((x) => (x.segTiers || []).map((v) => ({ p: x.name, ...v }))), (v) => `${v.p}  高${v.heights} 字${v.fonts}  ${v.items}`);

    // 这台以前只打印不置码：`十条全 0` 是靠人读报表读出来的，不是门禁。
    // 与 check:motion 那条"算出了 gate 却只打印"是同一种失效 —— 违例涨上去，CI 里没人会知道。
    const gateCount =
      ok.reduce((n, x) => n + (x.rowHeights?.length || 0) + (x.rowCenters?.length || 0)
        + (x.cardHeights?.length || 0) + (x.clipped?.length || 0) + (x.wrapped?.length || 0)
        + (x.gaps?.length || 0) + (x.tableAlign?.length || 0) + (x.clippedBy?.length || 0)
        + (x.coveredBy?.length || 0) + (x.segTiers?.length || 0), 0) + over.length;
    console.log(`几何门禁合计 ${gateCount} 处（R1–R10，含 R7 横向滚动）`);
    if (gateCount) process.exitCode = 1;
  }
  await browser.close();

  fs.writeFileSync(path.join(here, '..', 'layout-geometry.json'), JSON.stringify(all, null, 1));
  console.log('明细已写入 web/layout-geometry.json');
  const badRows = all.filter((x) => x.errored || x.failed);
  if (badRows.length) {
    console.log(`✗ ${badRows.length} 条量测没跑成（兜底页或导航失败）：`
      + [...new Set(badRows.map((x) => `${x.name}@${x.w}`))].join(', '));
    process.exitCode = 1;
  }
};

if (argv.includes('--selftest')) selftest(); else run();
