#!/usr/bin/env node
/**
 * 主题轮巡检 —— 用法：
 *   SNAP_USER=… SNAP_PASS=… npm run check:theme-rounds [-- --routes=/,/users --themes=all]
 *
 * 这台的存在理由是一条**以前没法回答**的问题：八档外观×材质组合里，到底有哪几档
 * 真的被自动化看过？答案是几乎没有：25 台 playwright 工装里只有 3 台轮主题，
 * 而且各自写死一个默认档（check:elements 只跑 light、check:glass:runtime 只跑一档玻璃、
 * snap 只拍 light）—— editorial、warm-paper、整个 dark 系、任何 solid 档从没被跑过，
 * 而清单上的话说的是"运行时已按主题巡检"。
 *
 * 解法不是把每台工装都改成 ×8（51 条路由 × 8 档 = 408 次加载，还要撞 300/分钟的限流），
 * 而是这一台专门的轮：**每档都跑，每档只看少数几页**，判的是只有"换了主题"才成立的四条不变量：
 *
 *  ① **这一轮真的切过去了** —— DOM 上的 data-theme / data-material / data-glass 必须等于
 *     本轮请求的那一档。这条是这台的地基：2026-10-16 实测 check:glass:runtime 把主题键写成
 *     `citebase:citebase:ui:theme`（前缀拼了两次），于是八轮全在默认档上量、报表却按 --themes
 *     标成玻璃档。**没有①，其余三条测的都是同一档四次。**
 *  ② 材质轴真的改变渲染结果 —— 同一外观的 solid 轮与 glass 轮，顶栏算出来的背景必须不同；
 *     玻璃轮还得真带 backdrop-filter。否则"接了材质令牌"只是 JS 里多存了一个值。
 *  ③ 浮层档在每个主题都落在自适应区间内（alpha ∈ [0.40, 0.94]）—— 这是液态玻璃的地板与天花板，
 *     某档主题把它写出 1.0 就等于该主题下没有玻璃。
 *  ④ 每档主题的正文/壳面对比度过线 —— 静态 check:contrast 按令牌算，这台按**浏览器算出来的值**算，
 *     两者不一致说明有东西在运行时覆盖了令牌（自适应加厚就是这样一个覆盖者）。
 *
 * 红线全部来自代码里的既有判据，不在这里另立一套口径。
 */
import { chromium } from 'playwright';
import { findChromium } from './lib/chromium-path.mjs';
import { loginForm } from './lib/page-state.mjs';
import {
  parseThemeList,
  parseThemeTarget,
  themeStorageEntries,
  ALL_THEME_TARGETS,
  BASELINE_PERSONALIZATION,
  PERSONA_DOM_ATTRS,
  PERSONALIZATION_SWEEP,
} from './lib/theme-target.mjs';
import { loadTsExports } from './lib/ts-source.mjs';

// 地板与天花板取自 src 的那份实现（同一文件已被单测钉住），不在工装里抄第二个数
const { GLASS_FLOAT_ALPHA_FLOOR, GLASS_FLOAT_ALPHA_CEIL } = loadTsExports([
  { from: './src/utils/core/glass-contrast', names: ['GLASS_FLOAT_ALPHA_FLOOR', 'GLASS_FLOAT_ALPHA_CEIL'] },
]);

// 档位取值也取自 src：工装手抄一份 hex / 倍数，色板一改就变成"按旧值判" ——
// 与 E1 那次"合法集自己抄成空集，把 1164 处玻璃圆角全报成违例"是同一类错。
const { MOTION_SCALE, ACCENT_REGISTRY } = loadTsExports([
  { from: './src/styles/theme/personalization', names: ['MOTION_SCALE'] },
  { from: './src/styles/theme/accents', names: ['ACCENT_REGISTRY'] },
]);
const ACCENT_HEX = new Map(ACCENT_REGISTRY.map((a) => [a.id, { light: a.light, dark: a.dark }]));
// 属性名 → 探针字段名（浏览器里那份闭包只回字段值，判据留在 Node 侧，别把期望值抄进闭包）
const AXIS_FIELD = {
  motion: 'domMotion',
  contrast: 'domContrast',
  transparency: 'domTransparency',
  accent: 'domAccent',
  persona: 'domPersona',
};

const BASE = process.env.SNAP_BASE || 'http://localhost:3000';
const USER = process.env.SNAP_USER;
const PASS = process.env.SNAP_PASS;
const argv = process.argv.slice(2);
const argOf = (name) => {
  const hit = argv.find((a) => a.startsWith(`--${name}=`));
  return hit ? hit.slice(name.length + 3) : '';
};

// 少数几页，但每页都换一遍全部八档 —— 与"一页不换主题跑八遍"是两回事。
const ROUTES = (argOf('routes') || '/,/dashboard,/users,/spaces').split(',').map((s) => s.trim()).filter(Boolean);
const themesArg = argOf('themes') || 'all';
const parsed = parseThemeList(themesArg);
if (!parsed.ok) {
  console.error(`✗ --themes=${themesArg} 解析失败：${parsed.error}`);
  console.error(`  可给：all 或逗号分隔，例如 light,dark+glass+clear；全量共 ${ALL_THEME_TARGETS.length} 档`);
  process.exit(1);
}
const TARGETS = parsed.targets;
// --personalization=1：把轮换成"每档相对基准档的单因子差异 + 一个固定极端档"（§7 的 C3）。
// 为什么不全乘：外观 4 × 材质 3 × 强调色 9 × 动效 4 × 人格 2 = 864 轮，而 51 条路由 × 8 档
// 就已经会撞 300 次/分钟的限流。报表必须写清算法，不许写"已覆盖全组合"。
const PERSONALITY = argOf('personalization') === '1' || argOf('personalization') === 'all';
// 个性化扫描模式：单一外观（light+glass）上跑各档差异，标签把档位组合写全，报表里能看出跑的是哪一套
const ROUNDS = PERSONALITY
? PERSONALIZATION_SWEEP.map((personalization) => {
const t = parseThemeTarget('light+glass');
if (!t.ok) throw new Error(`个性化轮需要 light+glass：${t.error}`);
return {
...t,
personalization,
label: `${t.label}@${Object.entries(personalization).map(([k, v]) => k + '=' + v).join(',')}`,
};
})
: TARGETS;

if (!USER || !PASS) {
  console.error('✗ 需要 SNAP_USER / SNAP_PASS：拿登录失败兜底页当主题样本，报表会绿得毫无意义');
  process.exit(1);
}

/**
 * 在页面里取本轮要判的东西。只读计算样式与 DOM 属性，不改任何东西。
 * 对比度用 WCAG 相对亮度，与 web/src/utils/core/glass-contrast.ts 同一公式（那台是单测过的实现，
 * 这里内联是为了能在 page.evaluate 里跑 —— 内联的**判据值**由上面的 import 提供，不重写）。
 */
const PROBE = () => {
  const parse = (s) => {
    const str = String(s).trim();
    const m = str.match(/rgba?\(([^)]+)\)/);
    if (m) {
      const p = m[1].split(',').map((x) => Number.parseFloat(x.trim()));
      return { r: p[0], g: p[1], b: p[2], a: p.length > 3 ? p[3] : 1 };
    }
    // 实心档的浮层令牌本来就是 #ffffff / #2c2c2e 这种十六进制（2026-10-16 实测四档实心全如此），
    // 解不出来就报"取不到"是工装的错，不是主题的错。
    const h = str.match(/^#([0-9a-f]{3}|[0-9a-f]{6}|[0-9a-f]{8})$/i);
    if (!h) return null;
    const hex = h[1].length === 3 ? h[1].split('').map((c) => c + c).join('') : h[1].slice(0, 6);
    return {
      r: Number.parseInt(hex.slice(0, 2), 16),
      g: Number.parseInt(hex.slice(2, 4), 16),
      b: Number.parseInt(hex.slice(4, 6), 16),
      a: h[1].length === 8 ? Number.parseInt(h[1].slice(6, 8), 16) / 255 : 1,
    };
  };
  const lum = ({ r, g, b }) => {
    const f = (v) => {
      const c = v / 255;
      return c <= 0.03928 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
    };
    return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b);
  };
  const over = (fg, bg) => {
    if (fg.a >= 1) return fg;
    const mix = (k) => fg[k] * fg.a + bg[k] * (1 - fg.a);
    return { r: mix('r'), g: mix('g'), b: mix('b'), a: 1 };
  };
  const ratio = (a, b) => {
    const [x, y] = [lum(a), lum(b)];
    const [hi, lo] = x >= y ? [x, y] : [y, x];
    return (hi + 0.05) / (lo + 0.05);
  };
  const root = document.documentElement;
  const nav = document.querySelector('[data-ap="nav-bar"]') || document.querySelector('header');
  const body = document.body;
  const navBgRaw = nav ? getComputedStyle(nav).backgroundColor : 'transparent';
  const navBg = over(parse(navBgRaw) || { r: 255, g: 255, b: 255, a: 1 }, parse(getComputedStyle(body).backgroundColor) || { r: 255, g: 255, b: 255, a: 1 });
  const token = getComputedStyle(root).getPropertyValue('--apple-surface-float').trim();
  const float = parse(token);
  // 找一个真正渲染出来的正文节点：拿最靠前的一个有文字的可见元素
  const text = [...document.querySelectorAll('h1, h2, p, span, a')]
    .find((e) => (e.textContent || '').trim().length > 1 && e.offsetParent !== null);
  const textEl = text || nav || body;
  const fg = parse(getComputedStyle(textEl).color) || { r: 0, g: 0, b: 0, a: 1 };
  return {
    domTheme: root.getAttribute('data-theme'),
    domMaterial: root.getAttribute('data-material'),
    domGlass: root.getAttribute('data-glass'),
    domMotion: root.getAttribute('data-motion'),
    domContrast: root.getAttribute('data-a11y-contrast'),
    domTransparency: root.getAttribute('data-a11y-transparency'),
    domAccent: root.getAttribute('data-accent'),
    domPersona: root.getAttribute('data-persona'),
    // 生效证据：属性写了不等于真的生效（这两枚是被 JS 算出来再写到 DOM 上的）
    blueToken: getComputedStyle(root).getPropertyValue('--apple-blue').trim(),
    motionScale: getComputedStyle(root).getPropertyValue('--apple-motion-scale').trim(),
    navBlur: nav ? String(getComputedStyle(nav).backdropFilter || 'none') : 'none',
    navBgRgba: [navBg.r, navBg.g, navBg.b, Number(navBg.a.toFixed(3))].join(','),
    floatToken: token,
    floatAlpha: float ? float.a : null,
    floatRgba: /^rgba?\(/.test(token),
    contrast: Number(ratio(over(fg, navBg), navBg).toFixed(2)),
    textSample: (textEl.textContent || '').trim().slice(0, 24),
  };
};

const browser = await chromium.launch({ executablePath: findChromium() });
const errors = [];
const rows = [];
// 每个外观记下 solid 轮的顶栏背景，供 glass 轮比对"材质轴到底改变了什么"
const solidNavBg = new Map();
// 每档之间留一点间隔：连续 4 页 × 8 档 = 32 次导航，别让限流器把后面的轮全变成 429 页
const GAP_MS = 250;

try {
  for (const target of ROUNDS) {
    const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
    const page = await ctx.newPage();
    for (const [k, v] of themeStorageEntries(target)) {
      await page.addInitScript(([kk, vv]) => localStorage.setItem(kk, vv), [k, v]);
    }
    await page.goto(`${BASE}/login`, { waitUntil: 'domcontentloaded' });
    await loginForm(page, BASE, USER, PASS);

    for (const route of ROUTES) {
      await page.goto(`${BASE}${route}`, { waitUntil: 'domcontentloaded', timeout: 30000 });
      await page.waitForTimeout(700);
      // PROBE 不带参数：上下限只在 Node 侧比，避免把判据值抄进浏览器里那份闭包（闭包会被序列化，抄进去就成了第二份真相）
      const p = await page.evaluate(PROBE);
      const where = `${target.label} ${route}`;

      // ① 本轮真的生效了吗
      if (p.domTheme !== target.base) errors.push(`${where} data-theme=${p.domTheme}（本轮要的是 ${target.base}）—— 主题没切过去，这一档等于没量`);
      if (p.domMaterial !== target.material) errors.push(`${where} data-material=${p.domMaterial}（要 ${target.material}）`);
      const wantGlass = target.clear ? 'clear' : null;
      if ((p.domGlass || null) !== wantGlass) errors.push(`${where} data-glass=${p.domGlass}（要 ${wantGlass ?? '不设'}）`);

      // ①b 五条个性化档逐个对账（批1–批4）。存在理由与历史那条一模一样：
      //    工装曾把主题键拼成 citebase:citebase:ui:theme，八轮全在默认档上量、报表却按玻璃档标注。
      //    "属性写了"与"真的生效"是两件事，所以 accent 与 motion 各再量一次实际取值。
      const want = { ...BASELINE_PERSONALIZATION, ...(target.personalization || {}) };
      for (const [axis, field] of Object.entries(AXIS_FIELD)) {
        if (p[field] !== want[axis]) {
          errors.push(`${where} ${PERSONA_DOM_ATTRS[axis]}=${p[field]}（本轮要的是 ${want[axis]}）—— 这一档没切过去，等于没量`);
        }
      }
      if (want.accent !== 'auto') {
        const hex = ACCENT_HEX.get(want.accent)?.[target.base === 'dark' ? 'dark' : 'light'];
        if (hex && p.blueToken !== hex) {
          errors.push(`${where} 强调色选了 ${want.accent}，但 --apple-blue 实际是 ${p.blueToken}（要 ${hex}）—— 覆写没落到 DOM`);
        }
      }
      const scale = MOTION_SCALE[want.motion];
      if (scale !== undefined && Number(p.motionScale) !== scale) {
        errors.push(`${where} 动效档 ${want.motion} 的倍数实际是 ${p.motionScale}（要 ${scale}）—— 档位写了但时长没跟着走`);
      }

      // ② 材质轴真的改变渲染结果
      if (target.material === 'glass') {
        if (p.navBlur === 'none') errors.push(`${where} 玻璃档但顶栏没有 backdrop-filter —— 材质只活在 JS 里`);
        const base = solidNavBg.get(target.base);
        if (base && base === p.navBgRgba) errors.push(`${where} 顶栏背景与 ${target.base}+solid 完全相同（${base}）—— 材质轴对这一档没有效果`);
      } else if (!solidNavBg.has(target.base)) {
        solidNavBg.set(target.base, p.navBgRgba);
      }

      // ③ 浮层档在每一档主题都有值，且形态与材质轴一致
      if (p.floatAlpha === null) errors.push(`${where} --apple-surface-float 取不到（${p.floatToken || '空'}）`);
      else if (target.material === 'glass') {
        if (!p.floatRgba) errors.push(`${where} 玻璃档的浮层令牌不是 rgba（${p.floatToken}）—— 该档没有半透明壳面`);
        else if (p.floatAlpha < GLASS_FLOAT_ALPHA_FLOOR - 1e-6 || p.floatAlpha > GLASS_FLOAT_ALPHA_CEIL + 1e-6) {
          errors.push(`${where} 浮层档 alpha=${p.floatAlpha} 越出自适应区间 [${GLASS_FLOAT_ALPHA_FLOOR}, ${GLASS_FLOAT_ALPHA_CEIL}]`);
        }
      } else if (p.floatAlpha < 0.999) {
        // 实心档的浮层本来就该是实心（实测四档实心都回 #ffffff / #2c2c2e 这类十六进制）。
        // 它要是变成半透明，说明材质轴被谁绕过了 —— 那正是"切了 solid 却还是玻璃脸"的表现。
        errors.push(`${where} 实心档的浮层令牌是半透明（α=${p.floatAlpha} ${p.floatToken}）—— 材质轴没生效`);
      }

      // ④ 运行时算出来的对比度过线（次要文字口径 4.5）
      if (p.contrast < 4.5) errors.push(`${where} 顶栏上的文字对比度 ${p.contrast} < 4.5（取样节点："${p.textSample}"）`);

      rows.push(`${target.label.padEnd(18)} ${route.padEnd(12)} ${p.domTheme}/${p.domMaterial}/${p.domGlass || '-'}  模糊 ${p.navBlur === 'none' ? '无' : '有'}  浮层 α ${p.floatAlpha ?? '-'}  对比 ${p.contrast}`);
      await page.waitForTimeout(GAP_MS);
    }
    await ctx.close();
  }
} finally {
  await browser.close();
}

console.log(rows.join('\n'));
if (errors.length) {
  console.error(`\n✗ 主题轮巡检失败 ${errors.length} 条：`);
  errors.forEach((e) => console.error(`  - ${e}`));
  process.exit(1);
}
console.log(`
✓ ${ROUNDS.length} 轮 × ${ROUTES.length} 页全部生效且判据过线（共 ${rows.length} 次采样）` + (PERSONALITY ? `
  本轮口径：light+glass 上的个性化档位扫描（单因子 + 一个极端组合，共 ${PERSONALIZATION_SWEEP.length} 轮），不是外观×材质的 ${ALL_THEME_TARGETS.length} 档全量` : ''));
