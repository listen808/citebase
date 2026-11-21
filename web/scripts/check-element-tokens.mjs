// 元素级令牌一致性巡检：把全站渲染出来的元素**按组件身份（data-ap）分组**，
// 比较同一组元素的圆角 / 阴影 / 字体 / 控件高度 / 图标尺寸 / 内边距是不是同一个值。
// 静态门禁（check:style）拦的是"源码里写了什么令牌"，这台拦的是"浏览器最终算出来什么"——
// :style 拼字符串、clamp()、被上层覆写，都只有运行时看得见。
//   E1 圆角落不在 AppleRadius 注册表里
//   E2 静置就该没阴影的类带了阴影（卡片静置不投影是设计教条）
//   E3 字体不是设计字体（text / mono 两种之外）
//   E4 表单控件高度不落在 32/40/44 阶梯上
//   E5 同一角色的图标有好几种尺寸（按上下文分组：按钮内 / 卡片头 / 表格单元 / 侧栏 / 标题行）。
//      读这条要先知道：图标"计算宽度"不等于声明尺寸 —— svg 是 flex 列的默认拉伸项，
//      容器多宽它就多宽（实测 /space 侧栏里 44 个声明 13px 的图标被拉成 15px）。
//      所以 E5 是登记表不是门禁：先按 owner 找出"同一个组件里两种尺寸"那种真漂移，
//      跨组件的尺寸差要先排除拉伸假象。
//   E6 组件根上的 padding / gap 不落 AppleSpace 注册表
//   E7 键盘 Tab 走过去焦点看不见（静置态与聚焦态的 outline/阴影/描边/底色完全没变）
// 用法（前端 dev :3000、后端 :3001 需在跑；口令只走环境变量）：
//   SNAP_USER=… SNAP_PASS=… node scripts/check-element-tokens.mjs [--only=<路由名子串>] [--max=12]
//   …[--themes=light,dark,light+glass,dark+glass,editorial,warm-paper]  默认只跑 light
//     外观 × 材质是两条轴：不写 +glass 就是 solid。liquid-glass 仍可用作 light+glass 的别名。
//   FOCUS_TABS=22 每页最多按几次 Tab
import { chromium } from 'playwright';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { findChromium } from './lib/chromium-path.mjs';
import { parseThemeTarget, themeStorageEntries } from './lib/theme-target.mjs';
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
const MAX = Number(argOf('max') || 12);
const TARGETS = await pickTargets(ONLY, here);

// 注册表从令牌源码里读，避免在工装里再抄一份"我以为的合法值"
const tokensSrc = fs.readFileSync(path.join(here, '..', 'src', 'utils', 'core', 'apple-tokens.ts'), 'utf8');
const section = (name) => tokensSrc.match(new RegExp(`export const ${name} = \\{([\\s\\S]*?)\\n\\}`, 'm'))?.[1] || '';
const pxValues = (body) => new Set([...body.matchAll(/'(-?\d+(?:\.\d+)?)px'/g)].map((m) => `${m[1]}px`));
const RADIUS_OK = pxValues(section('AppleRadius'));
RADIUS_OK.add('0px'); RADIUS_OK.add('50%'); RADIUS_OK.add('9999px');
const SPACE_OK = pxValues(section('AppleSpace'));
SPACE_OK.add('0px');
const CONTROL_H = new Set(['32px', '40px', '44px', '24px', '28px']);

// 主题有权重映射半径变量：玻璃把 --apple-radius-card 定成 14px、warm-paper 定成 10px、
// editorial 干脆全 0。这些是该主题的**本意值**，拿共享注册表去判就会把 1164 处玻璃圆角报成违例
// （首轮五主题巡检的 liquid-glass E1 就是这么响的）。所以合法集 = 注册表 ∪ 该外观自己声明的值
// ∪（材质是 glass 时）玻璃声明的值。值仍然从 themes.ts 源码解析，不在工装里手写。
//
// 判据要从主题**现读**这件事，重构后多了一条：外观与材质是两条轴，玻璃的半径住在
// `GLASS_OVERLAY` 而不住在任何以主题命名的块里。原先这里按 'liquid-glass' 找块，而那个键
// 现在只剩 `LEGACY_THEME_ALIAS` 里的一行（`{ base: 'light', material: 'glass' }`，没有半径），
// 于是玻璃的合法半径被解析成空集 —— 换个写法把它按轴取回，别让"没取到"冒充"没有覆盖值"。
const themeSrc = fs.readFileSync(path.join(here, '..', 'src', 'styles', 'theme', 'themes.ts'), 'utf8');
const radiusVarsIn = (t) => [...t.matchAll(/'--apple-radius-[\w-]+':\s*'(-?[\d.]+)px'/g)].map((m) => `${m[1]}px`);

/** 从 keyRe 命中处的第一个 `{` 开始做花括号配对，取出整块源码 */
function blockOf(src, keyRe) {
  const m = src.match(keyRe);
  if (!m) return '';
  const start = src.indexOf('{', m.index);
  if (start < 0) return '';
  let depth = 0;
  for (let j = start; j < src.length; j++) {
    if (src[j] === '{') depth += 1;
    else if (src[j] === '}') {
      depth -= 1;
      if (depth === 0) return src.slice(start, j + 1);
    }
  }
  return '';
}

const overridesSrc = themeSrc.slice(themeSrc.indexOf('export const themeOverrides'));
/** 外观轴：dark 住在单独的 DARK_THEME，其余在 themeOverrides 的对应键下 */
const BASE_RADII = {
  dark: radiusVarsIn(blockOf(themeSrc, /export const DARK_THEME/)),
  ...Object.fromEntries(
    ['editorial', 'warm-paper'].map((b) => [
      b,
      radiusVarsIn(blockOf(overridesSrc, new RegExp(`['"]?${b}['"]?\\s*:\\s*\\{`))),
    ]),
  ),
  light: [],
};
const GLASS_RADII = radiusVarsIn(blockOf(themeSrc, /export const GLASS_OVERLAY/));

const COLLECT = (cfg) => {
  const PAGE = cfg.page;
  const rect = (el) => el.getBoundingClientRect();
  const vis = (el) => { const r = rect(el); return r.width > 2 && r.height > 2; };
  // 主题自己声明的字体栈（含等宽），逐页现读——换主题就换白名单，不在工装里写死。
  // 原先写死一份 ['SF','PingFang',…]，而 editorial 的 --apple-font-text
  // 是 Georgia 系衬线，全体元素都会被判成"非设计字体"。
  const firstFam = (s) => (s || '').replace(/["']/g, '').replace(/\s+/g, ' ').trim().toLowerCase().split(',')[0].trim();
  const rs = getComputedStyle(document.documentElement);
  const allowedFirst = [
    firstFam(rs.getPropertyValue('--apple-font-text')),
    firstFam(rs.getPropertyValue('--apple-font-display')),
    ...cfg.monoFirst,
  ].filter(Boolean);
  const out = {};
  const push = (key, cls, val, where) => {
    (out[key] = out[key] || {});
    const bucket = (out[key][cls] = out[key][cls] || { values: {}, pages: [] });
    bucket.values[val] = (bucket.values[val] || 0) + 1;
    if (where && !bucket.pages.includes(where)) bucket.pages.push(where);
  };

  for (const el of document.querySelectorAll('body [data-ap]')) {
    if (!vis(el)) continue;
    const cls = el.getAttribute('data-ap');
    const cs = getComputedStyle(el);
    // E1 圆角
    for (const side of ['borderTopLeftRadius', 'borderTopRightRadius', 'borderBottomRightRadius', 'borderBottomLeftRadius']) {
      const v = cs[side];
      if (v && v !== '0px' && !v.includes('%') && !cfg.radiusOk.includes(v)) push('radius', cls, v, null);
    }
    // E2 阴影
    if (cs.boxShadow && cs.boxShadow !== 'none') push('shadow', cls, cs.boxShadow.replace(/\s+/g, ' ').slice(0, 46), null);
    // E3 字体：与**当前主题自己声明的那串**比，不在工装里抄白名单。
    // 原先写死一份 ['SF','PingFang',…]，serif 主题（editorial 的
    // --apple-font-text 是 Georgia 系）必然全体假阳性。
    const famFirst = firstFam(cs.fontFamily);
    if (!allowedFirst.includes(famFirst)) push('font', cls, cs.fontFamily.replace(/["']/g, '').split(',')[0].trim(), null);
    // E6 内边距 / 间距
    for (const p of [cs.paddingTop, cs.paddingBottom, cs.paddingLeft, cs.paddingRight]) {
      if (p && p !== '0px' && !cfg.spaceOk.includes(p)) push('space', `${cls}.padding`, p, null);
    }
    for (const g of [cs.rowGap, cs.columnGap]) {
      if (g && g !== 'normal' && g !== '0px' && !cfg.spaceOk.includes(g)) push('space', `${cls}.gap`, g, null);
    }
  }

  // E4 表单控件高度
  for (const el of document.querySelectorAll('main input:not([type=hidden]):not([type=checkbox]):not([type=radio]), main select, [data-ap-scroll] input:not([type=hidden]):not([type=checkbox]):not([type=radio]), [data-ap-scroll] select')) {
    if (!vis(el)) continue;
    // 控件高度：控件自身或它 3 层内任一层落在阶梯上就算合规。
    // 不能只量 input 自己（ApInput 的视觉盒是它外面那层 div），也不能只量最近
    // [data-ap] 宿主（ApInput 的根就是 ApFieldWrapper，量到的是 label+控件整体 65px）。
    const cands = [];
    for (let n = el, k = 0; n && k < 4; n = n.parentElement, k++) {
      const h = Math.round(rect(n).height);
      if (h > 2) cands.push(h);
    }
    if (!cands.some((h) => cfg.controlH.includes(`${h}px`))) {
      const host = el.closest('[data-ap]')?.getAttribute('data-ap') || '无宿主';
      const kind = `${el.tagName.toLowerCase()}[${el.type || 'select'}]`;
      push('controlH', `${kind} @${host}`, cands.map((h) => `${h}px`).join('>'), PAGE);
    }
  }

  // E5 图标尺寸，按上下文分角色
  for (const svg of document.querySelectorAll('main svg, [data-ap-scroll] svg')) {
    if (!vis(svg)) continue;
    const w = Math.round(rect(svg).width);
    if (w < 8 || w > 64) continue;
    const ctx = svg.closest('button') ? '按钮内'
      : svg.closest('th') ? '表头'
      : svg.closest('td') ? '表格单元'
      : svg.closest('[data-ap=card] h1, [data-ap=card] h2, [data-ap=card] h3') ? '卡片标题'
      : svg.closest('[data-ap=sidebar], nav') ? '侧栏/导航'
      : '其它';
    // 归属到"角色 @最近组件"，否则"其它 15px×44"这种数字没法下手。
    // 按钮里的图标再按**按钮实测高度**分层：小按钮(28)配 14px、中按钮(32)配 18px 是档位，
    // 把它们并成一桶就会报出"同一角色四种尺寸"的假漂移 —— 登记表自己写着"先排 owner 内的尺寸漂移"，
    // 分层之前它做不到。
    const hostBtn = svg.closest('button');
    const owner = svg.closest('[data-ap]')?.getAttribute('data-ap') || '无';
    const tier = hostBtn ? ` ${Math.round(rect(hostBtn).height)}px 档` : '';
    push('icon', `${ctx}${tier} @${owner}`, `${w}px`, PAGE);
  }
  return out;
};

const cfg = {
  radiusOk: [...RADIUS_OK],
  spaceOk: [...SPACE_OK],
  controlH: [...CONTROL_H],
  page: '',
  // 等宽字体是代码/数字场景的合法第二栈（不随主题变，取自 AppleFont.mono 的首项）
  monoFirst: ['sf mono', 'monaco', 'consolas', 'ui-monospace', 'menlo'],
};
const THEME_SPECS = (argOf('themes') || 'light').split(',').filter(Boolean);
// 主题写成两条轴（外观 × 材质）。认不出的规格**直接退出**，不能跳过那一轮继续报"全 0" ——
// 少量一套和量到一套干净的结果，在报表上长得一模一样。
const THEME_TARGETS = [];
for (const spec of THEME_SPECS) {
  const t = parseThemeTarget(spec);
  if (!t.ok) {
    console.error(`✗ --themes 里的「${spec}」无法解析：${t.error}`);
    process.exitCode = 1;
  } else THEME_TARGETS.push(t);
}
if (!THEME_TARGETS.length) process.exit(1);
const TABS = Number(process.env.FOCUS_TABS || 30);

// ---- E7 键盘焦点可见性 ----
// 静置态量不出"按 Tab 能不能看见自己在哪"。这里真的用键盘 Tab 走一遍：
// 先给所有可聚焦元素打上编号并记下它"没被聚焦时"的样子，然后每按一次 Tab
// 拿当前 activeElement 的同一条链去比——链完全没变 = 焦点看不见。
// 为什么不静态读 CSS：焦点环多半挂在 :focus-visible 上，还常被上层容器用
// :focus-within 承接，读源码要么漏判要么假阳性。
// page.evaluate 只序列化那一个函数，所以下面两个函数各自带一份同样的小工具和选择器。
const FOCUS_TAG = () => {
  const vis = (el) => { const r = el.getBoundingClientRect(); return r.width > 2 && r.height > 2 && r.bottom > 0 && r.top < innerHeight; };
  const one = (el) => { const cs = getComputedStyle(el); return [cs.outlineStyle, cs.outlineWidth, cs.outlineColor, cs.boxShadow, cs.borderTopColor, cs.backgroundColor].join('|'); };
  const chain = (el) => { const a = []; for (let n = el, k = 0; n && k < 4; n = n.parentElement, k++) a.push(one(n)); return a.join('#'); };
  const meta = (el) => el.tagName.toLowerCase() + (el.type ? `[${el.type}]` : '') + ' @' + (el.closest('[data-ap]')?.getAttribute('data-ap') || '无');
  // 选择器在这里写第二遍：page.evaluate 只搬函数体，Node 侧的 FOCUS_SEL 常量到浏览器里是 undefined
  const SEL = 'a[href],button:not([disabled]),input:not([type=hidden]):not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';
  const rest = {};
  let n = 0;
  for (const el of document.querySelectorAll(SEL)) {
    if (!vis(el)) continue;
    el.setAttribute('data-zzf', String(n));
    rest[n] = { chain: chain(el), meta: meta(el) };
    n++;
  }
  return { rest, count: n };
};
const FOCUS_ACTIVE = () => {
  const one = (el) => { const cs = getComputedStyle(el); return [cs.outlineStyle, cs.outlineWidth, cs.outlineColor, cs.boxShadow, cs.borderTopColor, cs.backgroundColor].join('|'); };
  const chain = (el) => { const a = []; for (let n = el, k = 0; n && k < 4; n = n.parentElement, k++) a.push(one(n)); return a.join('#'); };
  const meta = (el) => el.tagName.toLowerCase() + (el.type ? `[${el.type}]` : '') + ' @' + (el.closest('[data-ap]')?.getAttribute('data-ap') || '无');
  const el = document.activeElement;
  if (!el || el === document.body || el === document.documentElement) return null;
  return { id: el.getAttribute('data-zzf') || '', chain: chain(el), meta: meta(el) };
};
// 焦点环"绝对存在"的判据（outline 实线且非透明）；不满足时再看焦点前后样式链有没有变。
const ringPresent = (cur) => {
  const self = (cur.chain.split('#')[0] || '').split('|');
  return self[0] !== 'none' && parseFloat(self[1] || '0') >= 1 && self[2] !== 'rgba(0, 0, 0, 0)';
};
const judgeFocus = (rest, visited, pageName) => {
  const focus = {};
  let seen = 0;
  for (const cur of visited) {
    const base = rest[cur.id];
    if (!base) continue;
    seen++;
    if (ringPresent(cur) || cur.chain !== base.chain) continue;
    const b = (focus[cur.meta] = focus[cur.meta] || { values: {}, pages: [] });
    b.values['无可见焦点态'] = (b.values['无可见焦点态'] || 0) + 1;
    if (!b.pages.includes(pageName)) b.pages.push(pageName);
  }
  return { focus, seen };
};

const merge = (a, b) => {
  for (const [k, buckets] of Object.entries(b)) {
    a[k] = a[k] || {};
    for (const [cls, v] of Object.entries(buckets)) {
      a[k][cls] = a[k][cls] || { values: {}, pages: [] };
      for (const [val, n] of Object.entries(v.values)) a[k][cls].values[val] = (a[k][cls].values[val] || 0) + n;
      for (const pg of v.pages || []) if (!a[k][cls].pages.includes(pg)) a[k][cls].pages.push(pg);
    }
  }
  return a;
};

const run = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  page.on('dialog', (d) => (d.type() === 'beforeunload' ? d.accept() : d.dismiss()).catch(() => {}));
  await loginForm(page, BASE, USER, PASS);

  const themeAcc = {};
  const shadowBad = [];
  for (const target of THEME_TARGETS) {
    // 每一轮都把四个键显式写一遍（含 light / solid / false）：上一轮留下的偏好会污染下一轮，
    // 而"light 主题全 0"和"根本没切成 dark 所以量的还是 light"打印出来一模一样。
    // 材质与玻璃变体同理 —— 原先这里只写 theme，玻璃那一轮实际量的还是 solid。
    await page.evaluate((entries) => {
      try {
        for (const [k, v] of entries) localStorage.setItem(k, v);
      } catch { /* ignore */ }
    }, themeStorageEntries(target)).catch(() => {});
    cfg.radiusOk = [
      ...RADIUS_OK,
      ...(BASE_RADII[target.base] || []),
      ...(target.material === 'glass' ? GLASS_RADII : []),
    ];
    const acc = {};
    let pages = 0;
    const failed = [];
    let applied = '';
    let appliedMaterial = '';
    let focusSeen = 0;
    let focusPages = 0;
    let taggedPages = 0;
    const walkDead = [];
    for (const r of TARGETS) {
      try {
        await page.goto(`${BASE}${r.path}`, { waitUntil: 'load', timeout: 30000 });
        await page.waitForTimeout(Number(process.env.SNAP_GAP_MS || 900));
        // 量"静置态"：登录点击后指针停在视口中间，导航回来时 Chrome 会给指针下的卡片
        // 补发 mouseenter（实测 27 张卡带着 hover 阴影被记成静置违例）。先把指针挪走。
        cfg.page = r.name;
        await page.mouse.move(1, 1);
        await page.waitForTimeout(120);
        if (!applied) {
          const got = await page.evaluate(() => ({
            theme: document.documentElement.dataset.theme || '(unset)',
            material: document.documentElement.dataset.material || '(unset)',
          }));
          applied = got.theme;
          appliedMaterial = got.material;
        }
        merge(acc, await page.evaluate(COLLECT, cfg));
        pages++;
        // E7：真的按 Tab。Tab 不产生请求，不会把限流打爆。
        const tagged = await page.evaluate(FOCUS_TAG);
        if (tagged.count) {
          taggedPages++;
          const visited = [];
          let last = null;
          let skipped = 0;
          // 必须把焦点显式放回文档里再按 Tab：上一页走到末尾时焦点已经跳到浏览器界面，
          // 这时 keyboard.press('Tab') 落在页面外，activeElement 一直是 body —— 实测 72/73 页
          // "走过 0 个"，E7 于是以"0 命中"的姿态假绿。
          await page.evaluate(() => {
            document.body.tabIndex = -1;
            document.body.focus();
          });
          for (let i = 0; i < TABS; i++) {
            await page.keyboard.press('Tab');
            const cur = await page.evaluate(FOCUS_ACTIVE);
            if (!cur) break;
            if (cur.id && cur.id === last) break;
            if (!cur.id) {
              // 采到焦点但没被采样：它被 vis() 挡掉了（1px 的跳转链接、视口外的表单项）。
              // 不能因此中止 —— 从 body 起按 Tab 第一个落点往往就是它，
              // 一中止整页一个都没走过，实测 73/73 页全 0。
              skipped++;
              continue;
            }
            last = cur.id;
            visited.push(cur);
          }
          const judged = judgeFocus(tagged.rest, visited, r.name);
          merge(acc, judged.focus);
          if (judged.seen) { focusSeen += judged.seen; focusPages++; }
          if (!judged.seen && !skipped) walkDead.push(r.name);
        }
      } catch (e) {
        failed.push(`${r.name}: ${String(e).slice(0, 50)}`);
      }
    }

    console.log(
      `\n########## 主题 ${target.label}（实际 data-theme=${applied || '未量到'} ` +
        `data-material=${appliedMaterial || '未量到'}）##########`,
    );
    console.log(`量测页数 = ${pages} / ${TARGETS.length}${failed.length ? '（失败：' + failed.join(', ') + '）' : ''}`);
    console.log(`E7 Tab 走过 ${focusSeen} 个焦点元素 / 覆盖 ${focusPages} 页`);
    if (pages && (applied !== target.base || appliedMaterial !== target.material)) {
      console.log(
        `✗ 主题没切过去：请求 ${target.label} 但页面是 ${applied}+${appliedMaterial} ` +
          `—— 本主题所有"0 命中"都不作数`,
      );
      process.exitCode = 1;
    }
    if (taggedPages && focusPages * 2 < taggedPages) {
      console.log(`✗ 大半有焦点元素的页面没走过 Tab（${focusPages}/${taggedPages}）—— E7 的"0 命中"是探针没跑，不是全站合规`);
      console.log(`   走过 0 个的页：${walkDead.slice(0, 8).join(', ')}${walkDead.length > 8 ? ' …' : ''}`);
      process.exitCode = 1;
    }

    const show = (key, title, note) => {
      const buckets = acc[key] || {};
      const cls = Object.keys(buckets);
      console.log(`== ${title}：${cls.length} 类命中 ==`);
      for (const c of cls.sort((a, b) => Object.keys(buckets[b].values).length - Object.keys(buckets[a].values).length).slice(0, MAX)) {
        const vs = Object.entries(buckets[c].values).sort((a, b) => b[1] - a[1]);
        const ps = (buckets[c].pages || []).slice(0, 4).join(',');
        console.log(`   ${c}  ` + vs.map(([v, n]) => `${v}×${n}`).join('  ').slice(0, 110) + (ps ? `   页:${ps}` : ''));
      }
      if (note) console.log(`   ${note}`);
      console.log('');
    };

    show('radius', 'E1 圆角落不在注册表（按类）', '—— 空=全部落在 AppleRadius 上');
    show('shadow', 'E2 静置带阴影的类（按类）', '—— 浮层/弹窗有阴影是对的，卡片静置不该有');
    // E2 是唯一从普查升级成门禁的一条：其余几条还在登记阶段（值本身要靠人判真伪），
    // 但"卡片在装浮层"这件事不需要判 —— 组件身份不在白名单里就是违例。
    for (const v of shadowViolations(acc.shadow)) shadowBad.push(`${target.label} ${v}`);
    show('font', 'E3 非设计字体（按类）');
    show('controlH', 'E4 表单控件高度不在阶梯（按标签）');
    show('icon', 'E5 图标尺寸分布（按角色）', '—— 登记表：先排 owner 内的尺寸漂移，跨组件差值先怀疑 flex 拉伸');
    show('space', 'E6 padding/gap 不落 AppleSpace（按类）');
    show('focus', 'E7 键盘焦点看不见（按控件）', '—— 判据：按 Tab 后 outline/阴影/描边/底色一项都没变 = 键盘用户看不出焦点在哪');

    // 量测失败必须让进程非零：否则一页都没量到时，六条全都会打印"0 命中"，
    // 看着像"全站合规"，其实是探针根本没跑起来。
    if (failed.length) {
      console.log(`✗ ${failed.length} 页没量到 —— ${target.label} 主题下所有"0 命中"都不作数`);
      process.exitCode = 1;
    }

    themeAcc[target.label] = Object.fromEntries(Object.entries(acc).map(([k, v]) => [k, Object.fromEntries(
      Object.entries(v).map(([c, b]) => [c, { values: b.values, pages: b.pages }]))]));
  }

  await browser.close();

  console.log(`合法取值集合来自令牌源码：AppleRadius ${RADIUS_OK.size} 档 / AppleSpace ${SPACE_OK.size} 档 / 控件高度 ${[...CONTROL_H].join(',')}`);
  fs.writeFileSync(path.join(here, '..', 'element-tokens.json'), JSON.stringify(
    Object.fromEntries(Object.entries(themeAcc).map(([t, acc]) => [t, Object.fromEntries(
      Object.entries(acc).map(([k, v]) => [k, Object.fromEntries(Object.entries(v).map(([c, b]) => [c, b.values]))]))])), null, 1));
  console.log('明细已写入 web/element-tokens.json');
  if (shadowBad.length) {
    console.error(`✗ E2 静置阴影：${shadowBad.length} 个组件身份不在白名单里（白名单逐条写着理由，加一条要先想清楚它是不是真的浮起来）`);
    shadowBad.slice(0, 10).forEach((v) => console.error(`   ${v}`));
    process.exitCode = 1;
  }
};

// 反向验证：插一个"什么都不合规"的探针元素，六条断言必须各自抓到它。
// 没有这一步，"全部落在注册表上"完全可能只是探针没采到元素（判据是集合成员关系，
// 集合来自令牌源码——源码一改判据就跟着变，更需要跑红来证明它还在工作）。
const PROBE = () => {
  const host = document.querySelector('main, [data-ap-scroll]') || document.body;
  const box = document.createElement('div');
  box.setAttribute('data-ap', 'zzprobe');
  box.style.cssText = 'border-radius:7.5px;box-shadow:0 1px 2px rgba(9,9,9,.5);'
    + 'padding:11px;gap:13px;display:flex;font-family:ZZFakeFont, sans-serif;width:120px;height:41px';
  box.innerHTML = '<svg width="13" height="13" style="display:block"></svg>';
  host.appendChild(box);
  const inp = document.createElement('input');
  inp.style.cssText = 'height:37px;width:80px;display:block';
  host.appendChild(inp);
  return true;
};

// E7 的反向验证要注入"两个"探针：一个用默认焦点环（不该被标记），一个把
// 焦点态全部抹平（必须被标记）。只验"抓到坏的"不够——全站有 :focus-visible 兜底，
// 好的那个若也被标记，说明判据在瞎报。坏的探针用 id 选择器 + !important 压过兜底。
const PROBE_FOCUS = () => {
  const SEL = 'a[href],button:not([disabled]),input:not([type=hidden]):not([disabled]),select:not([disabled]),textarea:not([disabled]),[tabindex]:not([tabindex="-1"])';
  const first = document.querySelector(SEL);
  const host = first?.parentElement || document.body;
  const st = document.createElement('style');
  st.textContent = '#zzprobe-bad{background-color:transparent;border:1px solid transparent;box-shadow:none;outline:none;transition:none}'
    + '#zzprobe-bad:focus,#zzprobe-bad:focus-visible{outline:none!important;box-shadow:none!important;'
    + 'background-color:transparent!important;border-color:transparent!important}';
  document.head.appendChild(st);
  const mk = (id, ap) => {
    const b = document.createElement('button');
    b.id = id;
    b.setAttribute('data-ap', ap);
    b.textContent = 'x';
    b.style.cssText = 'width:24px;height:24px;padding:0';
    return b;
  };
  const good = mk('zzprobe-good', 'zzgood');
  const bad = mk('zzprobe-bad', 'zzbad');
  host.insertBefore(good, first || null);
  host.insertBefore(bad, first || null);
  return { insertedBefore: first ? first.tagName.toLowerCase() : '(none)' };
};

// 静置阴影只准出现在"本来就浮起来的东西"上。这里逐条写理由，是因为没有理由的白名单
// 会在下一次改样式时被顺手加一行 —— 那等于把判据擦掉。
// 组件身份来自 main.ts 的全局 mixin（Ap* 根节点自动打 data-ap=<slug>）。
const SHADOW_ALLOWED = {
  'button': 'ApButton 主按钮的同色柔光是既有决定（组件源码里写着理由），焦点环优先级更高',
  'search': 'ApSearch 用令牌 shadow.subtle（0 1px 4px），是输入框的浮起感不是卡片投影',
  'sidebar': 'ApSidebar 玻璃材质棱线（glass-edge + glass-side-shadow），是壳层玻璃边不是投影',
  'nav-bar': 'ApNavBar 玻璃材质内阴影（glass 材质 token 派生），是壳层玻璃边不是投影',
  // stat-card 的 1px 环是 shadow-level1（上缘发丝高光），不是投影：shadow-card 令牌本身就是
  // 'none'（"卡片静置不投影"教条在令牌层已兑现），ApStatCard 源码注释也写明"不接 shadow-card"。
  // 这条白名单放行的是那根 1px 环，若哪天 stat-card 冒出真外投影（level2+）这里就该红。
  'stat-card': 'ApStatCard 的 shadow-level1（0 0 0 1px）上缘发丝高光，非外投影；shadow-card 令牌本身为 none',
  'modal': '浮层', 'confirm-modal': '浮层', 'drawer': '浮层', 'toast': '浮层',
  'image-lightbox': '浮层', 'spotlight-search': '浮层（⌘K 面板）', 'popover': '浮层',
  'select': '下拉面板', 'theme-menu': '下拉面板', 'pagination': '浮层',
};

/** 纯函数：从 shadow 桶里挑出"不该带静置阴影"的组件。单独提出来是为了能在自检里直接判 */
function shadowViolations(buckets) {
  return Object.entries(buckets || {})
    .filter(([cls]) => !SHADOW_ALLOWED[cls] && !cls.startsWith('zz'))
    .map(([cls, b]) => `${cls} ${Object.entries(b.values || {}).slice(0, 2).map(([v, n]) => `${v}×${n}`).join(' ')}`);
}

const selftest = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  await loginForm(page, BASE, USER, PASS);
  await page.goto(`${BASE}/dashboard`, { waitUntil: 'load', timeout: 30000 });
  await page.waitForTimeout(1500);
  await page.evaluate(PROBE);
  const got = await page.evaluate(COLLECT, cfg);

  // E7：把探针插到 Tab 序列最前面，然后真按键盘走
  await page.evaluate(PROBE_FOCUS);
  const tagged = await page.evaluate(FOCUS_TAG);
  const visited = [];
  await page.evaluate(() => { const a = document.activeElement; if (a && a.blur) a.blur(); });
  let lastId = null;
  for (let i = 0; i < 6; i++) {
    await page.keyboard.press('Tab');
    const cur = await page.evaluate(FOCUS_ACTIVE);
    if (!cur || !cur.id || cur.id === lastId) break;
    lastId = cur.id;
    visited.push(cur);
  }
  const judged = judgeFocus(tagged.rest, visited, 'selftest');
  const flagged = Object.keys(judged.focus || {});
  const checks = [
    // 桶名会随归因方式变（早先按标签、后来按"类型 @宿主"），所以断言只认"哪个桶里
    // 出现了探针的那个取值"，不绑桶名——否则改一次归因就要跟着改一次自检，还会误报成探针失灵。
    ['E1 圆角', Object.values(got.radius || {}).some((b) => b.values?.['7.5px'])],
    ['E2 阴影', (got.shadow?.zzprobe && Object.keys(got.shadow.zzprobe.values).length) > 0],
    // E2 的门禁半边：白名单判据本身要能红。探针类（zz*）永远不算违例，
    // 所以这里直接喂真组件身份测"卡片带阴影必须报、浮层带阴影不许报"。
    ['E2 门禁抓到卡片投影', shadowViolations({ card: { values: { '0 1px 2px rgba(0,0,0,.5)': 3 } } }).length === 1],
    ['E2 门禁不误报浮层与主按钮', shadowViolations({ modal: { values: { 'x': 1 } }, button: { values: { 'y': 90 } }, zzprobe: { values: { 'z': 1 } } }).length === 0],
    ['E3 字体', Object.values(got.font || {}).some((b) => b.values?.ZZFakeFont)],
    ['E4 控件高度', Object.values(got.controlH || {}).some((x) => Object.keys(x.values || {}).some((v) => v.startsWith('37px')))],
    ['E5 图标尺寸', Object.values(got.icon || {}).some((b) => b.values?.['13px'])],
    ['E6 间距', Object.values(got.space || {}).some((b) => b.values?.['11px']) && Object.values(got.space || {}).some((b) => b.values?.['13px'])],
    [`E7 焦点看不见（Tab 走了 ${visited.length} 个：${flagged.join('|') || '无'}）`,
      judged.seen >= 2 && flagged.some((k) => k.includes('zzbad')) && !flagged.some((k) => k.includes('zzgood'))],
  ];
  await browser.close();
  let badCount = 0;
  for (const [k, ok] of checks) { if (!ok) badCount++; console.log(`${ok ? '✓' : '✗'} ${k} 抓到探针`); }
  if (badCount) console.log('✗ ' + badCount + ' 条断言没抓到明显不合规的探针——判据或采集范围写错了');
  else console.log(`${checks.length} 条断言全部抓到了探针`);
  process.exitCode = badCount ? 1 : 0;
};

if (argv.includes('--selftest')) selftest(); else run();
