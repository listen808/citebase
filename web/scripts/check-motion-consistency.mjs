// 动效与交互反馈一致性巡检（按 Apple「Designing Fluid Interfaces」的三条可量判据）：
//   M1 按下必须有反馈 —— 响应要发生在 pointer-**down**，不是 release。
//      量法：静置 → 悬停 → 按住，三个状态各读一次计算值。
//      "按住时和悬停时一模一样"= 键盘/鼠标用户按下去什么都不知道，这条是 §1 的第一原则。
//      注意必须先量"无悬停"的静置态：只看"按下前后"会把 hover 变化误当成 press 变化。
//   M2 过渡时长/缓动不落注册表 —— AppleTransition / AppleEasing 是唯一来源，
//      合法值直接从 apple-tokens.ts 源码解析，不在工装里另抄一份"我以为的合法值"。
//      同一类控件（同 data-ap）出现两种时长，就是"看得到但不该存在"的漂移。
//   M3 系统「减弱动态效果」必须真的减弱 —— emulateMedia(reduce) 后重量一遍，
//      任何元素还留着 > 20ms 的过渡/动画都算没接住（§14：响应三个独立信号）。
//      这条同时反向证明"没开 reduce 时确实有长过渡"，否则 M3 的 0 是空的。
// 已知口径边界：M1 只覆盖**视口内**控件（鼠标要真的移得过去），每页最多 PRESS_NODES 个，
// 且"按下去本身会改变页面"——导航、聚焦滚动、展开菜单都会让后面的坐标失效，
// 那些会被记成 skipped（实测一轮 616 按过 / 104 真判定 / 512 被挡住）。
// 所以 M1 的 0 读作"被真正按到的那批都有反馈"，不是"全站都有反馈"。
// 用法（前端 dev :3000、后端 :3001 需在跑；口令只走环境变量）：
//   SNAP_USER=… SNAP_PASS=… node scripts/check-motion-consistency.mjs [--only=<路由名子串>] [--max=14]
//   node scripts/check-motion-consistency.mjs --selftest   # 反向验证：三条都必须抓到自造违例
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
const MAX = Number(argOf('max') || 14);
const TARGETS = await pickTargets(ONLY, here);
const PER_PAGE = Number(process.env.PRESS_NODES || 12);

// 合法值从令牌源码解析
const tokensSrc = fs.readFileSync(path.join(here, '..', 'src', 'utils', 'core', 'apple-tokens.ts'), 'utf8');
const section = (name) => tokensSrc.match(new RegExp(`export const ${name} = \\{([\\s\\S]*?)\\n\\}`, 'm'))?.[1] || '';
const normEase = (s) => (s || '').replace(/\s+/g, ' ').trim().toLowerCase();
// 时长档现在住在 `BASE_DURATION_MS`（毫秒），CSS 侧是 `calc(<ms>ms * var(--apple-motion-scale))`。
// **不能再从 AppleTiming / AppleTransition 的源码里抓秒数** —— 那两个对象已经改成引用 var 令牌，
// 抓出来是空集，而空合法集正是这个仓库最熟悉的那类假绿（E1 圆角那次把 1164 处全报成违例，
// 反方向就是"什么都不判"）。合法值 = 基准表 × 每个档位倍数；
// 减弱档那条 `!important` 通配会把值压到 0.01ms，所以 <20ms 一律算合法（那是减弱档生效的样子，不是漂移）。
const baseMs = [...section('BASE_DURATION_MS').matchAll(/^\s*([A-Za-z][\w]*):\s*(\d+(?:\.\d+)?),/gm)].map((m) => ({
  key: m[1],
  ms: Number(m[2]),
}));
if (baseMs.length < 10) {
  console.error(`BASE_DURATION_MS 只解析出 ${baseMs.length} 档 —— 判据引用的结构已变了，先修判据再说代码（不修就等于 M2 拿空合法集运行，什么都判不出来）`);
  process.exit(2);
}
const MOTION_SCALES = [1, 0.6];
const REDUCE_CEILING_MS = 20;
const DUR_OK = [...new Set([
  ...baseMs.flatMap((b) => MOTION_SCALES.map((s) => `${(b.ms * s) / 1000}s`)),
  '0s',
])];
const EASE_OK = [...new Set([
  ...[...section('AppleEasing').matchAll(/cubic-bezier\([^)]*\)/g)].map((m) => normEase(m[0])),
  ...[...section('AppleTransition').matchAll(/cubic-bezier\([^)]*\)/g)].map((m) => normEase(m[0])),
  ...[...section('AppleTiming').matchAll(/cubic-bezier\([^)]*\)/g)].map((m) => normEase(m[0])),
])];

const PREPARE = (cfg) => {
  const rect = (el) => el.getBoundingClientRect();
  const vis = (el) => { const r = rect(el); return r.width > 6 && r.height > 6 && r.top >= 0 && r.bottom <= innerHeight && r.left >= 0 && r.right <= innerWidth; };
  const snap = (el) => {
    const cs = getComputedStyle(el);
    return [cs.transform, cs.boxShadow, cs.backgroundColor, cs.borderColor, cs.opacity, cs.filter].join('|');
  };
  // 归因串必须带**能照着修的出处**：只有"标签 @宿主"时，报出 43 个 a 也不知道是哪个链接
  // （和术语巡检那条"无名按钮@坐标 没法照着修"是同一个教训）。
  const meta = (el) => el.tagName.toLowerCase()
    + (el.getAttribute('href') ? `[${el.getAttribute('href').slice(0, 26)}]` : '')
    + (el.id ? `#${el.id}` : '')
    + ' @' + (el.closest('[data-ap]')?.getAttribute('data-ap') || '无')
    + ' 「' + (el.textContent || el.getAttribute('aria-label') || '').trim().slice(0, 10) + '」';
  const PARTS = ['button', 'a[href]', '[role="button"]', '[role="tab"]', 'input[type=checkbox]', '[data-ap=switch]'];
  // 前缀要逐条挂：写成 'main ' + SEL 只有第一个选择器带 main，其余是全局的 ——
  // 于是顶栏那些"按下去就导航/折叠"的链接混进样本，一按就把后面的测量全打断
  // （实测 390/864 个因此丢失）。采样收在正文与滚动壳层里。
  const SEL = ['main', '[data-ap-scroll]'].flatMap((h) => PARTS.map((p) => `${h} ${p}`)).join(',');
  // 破坏性文案的控件一律不按住（跑在真实库上）。选择器与正则在这里各写一遍：
  // page.evaluate 只搬函数体，Node 侧的常量到页面里是 undefined。
  const NEVER = /删除|移除|清空|重置|执行|运行|触发|重跑|同步|保存|提交|发送|导出|下载|导入|重建|恢复|退出|登出/;
  const out = [];
  const counts = new Map();
  let i = 0;
  for (const el of document.querySelectorAll(SEL)) {
    if (out.length >= cfg.per) break;
    if (el.disabled || el.getAttribute('aria-disabled') === 'true' || !vis(el)) continue;
    if (NEVER.test(el.textContent || '') || NEVER.test(el.getAttribute('aria-label') || '')) continue;
    el.setAttribute('data-zzm', String(i));
    // 图标按钮既没 href 也没文字，key 会整片撞在一起（/logs 上 6 个 BUTTON|| 全同键，
    // find() 只会回到第一个），所以再带上"这是第几个同键元素"。
    const baseKey = el.tagName + '|' + (el.getAttribute('href') || '') + '|' + (el.textContent || el.getAttribute('aria-label') || '').trim().slice(0, 12);
    const key = baseKey + '#' + (counts.get(baseKey) || 0);
    counts.set(baseKey, (counts.get(baseKey) || 0) + 1);
    out.push({ i: String(i), key, meta: meta(el), x: Math.round(rect(el).left + rect(el).width / 2), y: Math.round(rect(el).top + rect(el).height / 2), rest: snap(el) });
    i++;
  }
  return out;
};

// 按坐标现定位，不往 DOM 里写任何标记：这类页面内容会自己重渲染，
// 写进去的标记一刷新就查无元素（实测 390/864 个因此丢失）。
const AT = (pt) => {
  let el = document.elementFromPoint(pt.x, pt.y);
  if (!el) return null;
  // 命中的常常是按钮里的 <span>/<svg>，它的样式当然不随按压变化 ——
  // 不归到可交互祖先就会把"读错了元素"报成"按下无反馈"（「发布」按钮那条假阳性就是这么来的）。
  el = el.closest('button,a[href],[role="button"],[role="tab"],input,[data-ap=switch]') || el;
  const cs = getComputedStyle(el);
  // 带上 :active —— 这是"这一下到底按到它没有"的唯一自证。
  // 悬停会展开下拉的控件（顶栏分组链接）在按下那一刻已经被自己的菜单接走指针，
  // 不判这个就会把"探针没按到"报成"按下无反馈"（实测顶栏「AI」链接 ×40 假阳性）。
  return [cs.transform, cs.boxShadow, cs.backgroundColor, cs.borderColor, cs.opacity, cs.filter].join('|') + '#' + (el.matches(':active') ? 'A' : '-');
};
// 按之前必须确认这个点上的最上层元素**就是它**：前面某个控件被按开后留下的
// 下拉/菜单会盖住后面的控件，鼠标实际按在浮层上，被测元素永远不会进 :active
// —— 第一版就是这么把顶栏「AI」链接报成"按下无反馈"的假阳性。
// 命中者必须就是当初采样到的那个控件（按 key 认），否则这一发不算数
const TOP_AT = (c) => {
  const el = document.elementFromPoint(c.x, c.y);
  if (!el) return 'none';
  const keyOf = (e) => e.tagName + '|' + (e.getAttribute('href') || '') + '|' + (e.textContent || e.getAttribute('aria-label') || '').trim().slice(0, 12);
  // 命中者常常是按钮里的 <svg>/<span>，先归到最近的可交互祖先再比 ——
  // 上一版直接比祖先链，514/638 个被误判成"被挡住"，覆盖率掉到 19%。
  const host = el.closest('button,a[href],[role="button"],[role="tab"],input,[data-ap=switch]') || el;
  return keyOf(host) === c.key.split('#')[0] ? 'ok' : 'covered';
};

// 逗号切分必须跳过括号内部：`cubic-bezier(0.25, 0.1, 0.25, 1)` 直接 split(',')
// 会碎成 4 段"非法曲线"，第一版就是这样把 3 条合法缓动报成 13 类漂移的。
const TRANSITIONS = () => {
  const split = (s) => {
    const parts = [];
    let depth = 0;
    let cur = '';
    for (const ch of s) {
      if (ch === '(') depth++;
      else if (ch === ')') depth--;
      if (ch === ',' && depth === 0) { parts.push(cur); cur = ''; continue; }
      cur += ch;
    }
    parts.push(cur);
    return parts.map((x) => x.trim()).filter(Boolean);
  };
  const out = { dur: {}, ease: {} };
  const seen = new Set();
  // 第三方 DOM 一律不判：devtools 的锚点带 `transition: all 1s`，
  // 第一版没豁免它，73 页每页都报一条"时长离档"假漂移（其余工装早就跳了）。
  const thirdParty = (e) => !!e.closest('.vue-devtools__anchor, .monaco-editor, .cm-editor, [data-mode-id], [class*="devtools"]');
  for (const el of document.querySelectorAll('body *')) {
    const r = el.getBoundingClientRect();
    if (r.width < 2 || r.height < 2) continue;
    if (thirdParty(el)) continue;
    const cs = getComputedStyle(el);
    if (cs.transitionDuration === '0s' && cs.animationDuration === '0s') continue;
    const cls = el.closest('[data-ap]')?.getAttribute('data-ap') || el.tagName.toLowerCase();
    const push = (bucket, val) => {
      const k = `${bucket}::${cls}::${val}`;
      if (seen.has(k)) return;
      seen.add(k);
      (out[bucket][cls] = out[bucket][cls] || { values: {}, pages: [] });
      out[bucket][cls].values[val] = (out[bucket][cls].values[val] || 0) + 1;
    };
    for (const d of split(cs.transitionDuration)) if (d && d !== '0s') push('dur', d);
    for (const e of split(cs.transitionTimingFunction)) if (e) push('ease', e.toLowerCase());
  }
  return out;
};

const merge = (a, b) => {
  for (const [k, buckets] of Object.entries(b)) {
    a[k] = a[k] || {};
    for (const [cls, v] of Object.entries(buckets)) {
      a[k][cls] = a[k][cls] || { values: {}, pages: [] };
      for (const [val, n] of Object.entries(v.values)) a[k][cls].values[val] = (a[k][cls].values[val] || 0) + n;
      if (v.pages) for (const pg of v.pages) if (!a[k][cls].pages.includes(pg)) a[k][cls].pages.push(pg);
    }
  }
  return a;
};

const login = async (page) => {
  await loginForm(page, BASE, USER, PASS);
};

const run = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  page.on('dialog', (d) => (d.type() === 'beforeunload' ? d.accept() : d.dismiss()).catch(() => {}));
  await login(page);

  const acc = {};
  const press = {};
  let pages = 0;
  let pressed = 0;
  let judged = 0;
  let skipped = 0;
  let lost = 0;
  let hoverReactive = 0;
  const failed = [];

  for (const r of TARGETS) {
    try {
      await page.goto(`${BASE}${r.path}`, { waitUntil: 'load', timeout: 30000 });
      await page.waitForTimeout(Number(process.env.SNAP_GAP_MS || 900));
      await page.mouse.move(1, 1);
      await page.waitForTimeout(120);
      pages++;

      merge(acc, TRANSITIONS_TO_BUCKETS(await page.evaluate(TRANSITIONS), r.name));

      // M1：静置 → 悬停 → 按住，三段各读一次
      const cands = await page.evaluate(PREPARE, { per: PER_PAGE });
      for (const c of cands) {
        pressed++;
        const pt = c;  // TOP_AT 要拿 key 核对命中者，整条候选一起传
        if (await page.evaluate(TOP_AT, pt) !== 'ok') { skipped++; continue; }
        await page.mouse.move(c.x, c.y);
        await page.waitForTimeout(90);
        if (await page.evaluate(TOP_AT, pt) !== 'ok') { skipped++; continue; }
        const urlBefore = page.url();
        const hover = await page.evaluate(AT, pt);
        await page.mouse.down();
        await page.waitForTimeout(120);
        const down = await page.evaluate(AT, pt);
        await page.mouse.up();
        // 剥掉 :active 标记再比样式；没按到的一律不计入判定
        const pressedIt = down ? down.endsWith('#A') : false;
        const hoverStyle = hover ? hover.slice(0, hover.lastIndexOf('#')) : null;
        const downStyle = down ? down.slice(0, down.lastIndexOf('#')) : null;
        if (process.env.MOTION_DEBUG) {
          console.log(`   [dbg] ${c.key.slice(0, 26)} 按到=${pressedIt ? 'Y' : 'N'} hover=${String(hoverStyle).slice(0, 70)} down=${String(downStyle).slice(0, 70)}`);
        }
        if (!pressedIt) { skipped++; await page.keyboard.press('Escape').catch(() => {}); await page.mouse.move(1, 1); await page.waitForTimeout(200); continue; }
        judged++;
        // 按的是链接就会真的跳页 —— 不回跳，后面每个坐标都量在另一张页面上
        if (page.url() !== urlBefore) {
          await page.goto(urlBefore, { waitUntil: 'load', timeout: 30000 }).catch(() => {});
          await page.waitForTimeout(700);
        }
        if (hoverStyle === null || downStyle === null) { lost++; continue; }
        if (downStyle === hoverStyle) {
          const b = (press[c.meta] = press[c.meta] || { values: {}, pages: [] });
          b.values['按下与悬停完全相同'] = (b.values['按下与悬停完全相同'] || 0) + 1;
          if (!b.pages.includes(r.name)) b.pages.push(r.name);
        }
        if (hoverStyle !== c.rest) hoverReactive++;
        // 按完把可能弹出的下拉/菜单关掉再走下一个，否则它会盖住后面的控件
        await page.keyboard.press('Escape').catch(() => {});
        await page.mouse.move(1, 1);
        await page.waitForTimeout(220);
      }
    } catch (e) {
      failed.push(`${r.name}: ${String(e).slice(0, 60)}`);
    }
  }

  // M3：开系统「减弱动态效果」重量
  await page.emulateMedia({ reducedMotion: 'reduce' });
  const stillMoving = {};
  let m3Pages = 0;
  for (const r of TARGETS.slice(0, Number(process.env.M3_PAGES || 14))) {
    try {
      await page.goto(`${BASE}${r.path}`, { waitUntil: 'load', timeout: 30000 });
      await page.waitForTimeout(Number(process.env.SNAP_GAP_MS || 900));
      m3Pages++;
      const t = await page.evaluate(TRANSITIONS);
      for (const [cls, v] of Object.entries(t.dur)) {
        for (const val of Object.keys(v.values)) {
          const s = val.endsWith('ms') ? Number(val.slice(0, -2)) / 1000 : parseFloat(val);
          if (s > 0.02) {
            (stillMoving[cls] = stillMoving[cls] || { values: {}, pages: [] });
            stillMoving[cls].values[val] = (stillMoving[cls].values[val] || 0) + 1;
            if (!stillMoving[cls].pages.includes(r.name)) stillMoving[cls].pages.push(r.name);
          }
        }
      }
    } catch (e) { failed.push(`[reduce] ${r.name}: ${String(e).slice(0, 50)}`); }
  }
  await page.emulateMedia({ reducedMotion: null });
  await browser.close();

  console.log(`量测页数 = ${pages} / ${TARGETS.length}，按过 ${pressed} 个控件（真判定 ${judged} / 被浮层或视口挡住 ${skipped} / 中途被重渲染弄丢 ${lost}），reduce 模式复量页 = ${m3Pages}`);
  const show = (title, buckets, note) => {
    const cls = Object.keys(buckets);
    console.log(`\n== ${title}：${cls.length} 类命中 ==`);
    for (const c of cls.sort((a, b) => Object.keys(buckets[b].values).length - Object.keys(buckets[a].values).length).slice(0, MAX)) {
      const vs = Object.entries(buckets[c].values).sort((a, b) => b[1] - a[1]);
      console.log(`   ${c}  ` + vs.map(([v, n]) => `${v}×${n}`).join('  ').slice(0, 118) + ((buckets[c].pages || []).length ? `   页:${buckets[c].pages.slice(0, 3).join(',')}` : ''));
    }
    if (note) console.log(`   ${note}`);
  };

  show('M1 按下没有反馈（hover 态与 press 态计算值完全相同）', press, '—— 判据来自 §1：反馈要发生在 pointer-down，悬停变化不算');

  const durBad = {}; const easeBad = {};
  for (const [cls, v] of Object.entries(acc.dur || {})) {
    for (const [val, n] of Object.entries(v.values)) {
      const s = val.endsWith('ms') ? Number(val.slice(0, -2)) / 1000 : parseFloat(val);
      if (DUR_OK.includes(`${s}s`) || DUR_OK.includes(`${val}`)) continue;
      // 减弱动效那组 !important 会把值压到 0.01ms：那不是漂移，是那一档该有的样子
      if (Number.isFinite(s) && s * 1000 < REDUCE_CEILING_MS) continue;
      (durBad[cls] = durBad[cls] || { values: {}, pages: [] }).values[val] = n;
    }
  }
  for (const [cls, v] of Object.entries(acc.ease || {})) {
    for (const [val, n] of Object.entries(v.values)) {
      if (EASE_OK.includes(normEase(val)) || val === 'linear' || val === 'ease' || val.startsWith('cubic-bezier(0, 0, 1, 1)')) continue;
      (easeBad[cls] = easeBad[cls] || { values: {}, pages: [] }).values[val.slice(0, 40)] = n;
    }
  }
  show('M2 过渡时长不落 AppleTransition（按类）', durBad, `合法档：${DUR_OK.join(' ')}`);
  show('M2 缓动曲线不落 AppleEasing（按类）', easeBad, `合法曲线：${EASE_OK.length} 条 + linear/ease`);
  show('M3 开了「减弱动态效果」仍在动的元素（按类）', stillMoving, '—— 令牌层有全局 `*{transition-duration:.01ms !important}`，命中说明某处用了 !important 或写了内联长过渡');

  if (failed.length || pages === 0 || judged === 0) {
    console.log(`\n✗ 有页面/控件没量到（失败 ${failed.length} 条：${failed.slice(0, 3).join(' | ')}）—— 上面所有"0 命中"都不作数`);
    process.exitCode = 1;
  }
  const gate = Object.keys(press).length + Object.keys(durBad).length + Object.keys(easeBad).length + Object.keys(stillMoving).length;
  console.log(`\n门禁合计 ${gate} 类；hover 态与静置不同的控件 ${hoverReactive} 个（悬停有反馈，正常）`);
  // 这条以前只打印不置码：M1/M2/M3 有违例时 exit 仍是 0，"跑过了"和"跑绿了"分不出来。
  // 上面那句 failed/pages/judged 的闸门管的是"有没有量到"，管不到"量到了什么"。
  if (gate) process.exitCode = 1;
  fs.writeFileSync(path.join(here, '..', 'motion-consistency.json'), JSON.stringify(
    { press, dur: acc.dur, ease: acc.ease, m3: stillMoving }, null, 1));
  console.log('明细已写入 web/motion-consistency.json');
};

function TRANSITIONS_TO_BUCKETS(t, pageName) {
  const out = {};
  for (const bucket of ['dur', 'ease']) {
    out[bucket] = {};
    for (const [cls, v] of Object.entries(t[bucket])) {
      out[bucket][cls] = { values: v.values, pages: [pageName] };
    }
  }
  return out;
}

// 反向验证：造一个"按下毫无反应 + 时长缓动都不落令牌"的控件，三条断言必须各自抓到。
const INJECT = () => {
  const host = document.querySelector('main') || document.body;
  const st = document.createElement('style');
  st.textContent = '#zzpress{width:120px;height:32px;background:#444;border:1px solid #444;color:#fff;'
    + 'transition-duration:0.222s;transition-timing-function:cubic-bezier(0.9,0.1,0.2,0.7)}'
    + '#zzpress:hover{background:#444;box-shadow:none;transform:none;border-color:#444}'
    + '#zzpress:active{background:#444;box-shadow:none;transform:none;border-color:#444}'
    + '#zzlong{transition:width 3s cubic-bezier(0.25,0.1,0.25,1);width:40px;height:8px;background:#888}';
  document.head.appendChild(st);
  const b = document.createElement('button');
  b.id = 'zzpress';
  b.textContent = 'zzpress';
  const l = document.createElement('div');
  l.id = 'zzlong';
  l.setAttribute('data-ap', 'zzlong');
  host.appendChild(b);
  host.appendChild(l);
  // 采样只覆盖视口内控件（鼠标要真的移得过去），所以探针要滚进视野
  b.scrollIntoView({ block: 'center' });
  return true;
};

const selftest = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 }, locale: 'zh-CN' });
  page.on('dialog', (d) => (d.type() === 'beforeunload' ? d.accept() : d.dismiss()).catch(() => {}));
  await login(page);
  await page.goto(`${BASE}/dashboard`, { waitUntil: 'load', timeout: 30000 });
  await page.waitForTimeout(1500);
  await page.mouse.move(1, 1);
  await page.evaluate(INJECT);
  await page.evaluate(PREPARE, { per: 400 });
  const target = await page.evaluate(() => {
    const el = document.getElementById('zzpress');
    if (!el) return null;
    const r = el.getBoundingClientRect();
    return { x: Math.round(r.x + r.width / 2), y: Math.round(r.y + r.height / 2) };
  });
  let m1 = false;
  if (target) {
    await page.mouse.move(target.x, target.y);
    await page.waitForTimeout(90);
    const hover = await page.evaluate(AT, target);
    await page.mouse.down();
    await page.waitForTimeout(120);
    const down = await page.evaluate(AT, target);
    await page.mouse.up();
    const strip = (x) => (x ? x.slice(0, x.lastIndexOf('#')) : null);
    // 探针必须"真的被按到"（down 带 #A）且样式一模一样，才算"按下无反馈"
    m1 = !!hover && !!down && down.endsWith('#A') && strip(down) === strip(hover);
  }
  const t = await page.evaluate(TRANSITIONS);
  const durVals = new Set(Object.values(t.dur).flatMap((v) => Object.keys(v.values)));
  const easeVals = new Set(Object.values(t.ease).flatMap((v) => Object.keys(v.values)));
  await page.emulateMedia({ reducedMotion: 'reduce' });
  await page.waitForTimeout(300);
  const t2 = await page.evaluate(TRANSITIONS);
  const dur2 = new Set(Object.values(t2.dur).flatMap((v) => Object.keys(v.values)));
  await browser.close();

  const checks = [
    [`M1 按下无反馈（探针 ${target ? '已找到' : '没采到'}）`, m1 && !!target],
    ['M2 时长 0.222s 被抓到', durVals.has('0.222s')],
    // 断言要认**整条曲线**，不能只认 '0.9' 这个子串 —— 按逗号切碎括号的 bug
    // 用子串断言照样通过，这条判据就白写了。
    [`M2 非法曲线整条被抓到（${[...easeVals].find((x) => x.includes('0.9')) || '没量到'}）`,
      easeVals.has(normEase('cubic-bezier(0.9, 0.1, 0.2, 0.7)'))],
    [`M2 合法曲线以整条形态存在、没被切碎（${[...easeVals].filter((x) => x.startsWith('cubic-bezier(0.25, 0.1')).length} 处）`,
      [...easeVals].some((x) => x.startsWith('cubic-bezier(0.25, 0.1'))],
    // M3 要证明的是"这条量测路径两边都通"：不开 reduce 时 3s 量得到，开了之后必须塌到 0。
    // 只断言"reduce 下还量得到"是反的 —— 全局 !important 本来就该把它吞掉，
    // 那样这条断言永远绿，而它其实什么都没测。
    [`M3 3s 长过渡：未开 reduce ${durVals.has('3s') ? '在' : '不在'} / 开了 ${dur2.has('3s') ? '仍在' : '已塌'}`,
      durVals.has('3s') && !dur2.has('3s')],
  ];
  let bad = 0;
  for (const [k, ok] of checks) { if (!ok) bad++; console.log(`${ok ? '✓' : '✗'} ${k}`); }
  console.log(bad ? `✗ ${bad} 条没抓到明显违例——判据或采集写错了` : '三条判据都抓到了自造违例');
  process.exitCode = bad ? 1 : 0;
};

if (argv.includes('--selftest')) selftest(); else run();
