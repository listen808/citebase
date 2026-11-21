#!/usr/bin/env node
/**
 * 对比度守卫 —— 用法：npm run check:contrast（已串进 npm run check:style）
 *
 * 为什么不用浏览器量：color-mix 的计算值会被序列化成 color(srgb …)/oklab，
 * 脚本里解一次就出错（我就被坑过一次）。这里直接在 Node 里按 CSS 规则求值：
 * 把主题差异表里的 color-mix / var 表达式解析成真实 sRGB，再按 alpha 合成到
 * 它实际会落在的表面上，算 WCAG 相对亮度比。
 *
 * 断言的是「文字压在它实际会出现的背景上」的对比度：
 *   正文 ≥ 7（AAA）  次级/三级 ≥ 4.5（AA）  四级 ≥ 3（辅助/图标，WCAG 1.4.11）
 *   强调色上的文字 ≥ 3（AA 大字，按钮都是 14px+ 半粗）
 *   色相文字压同色浅染（徽章 / 选中行 / chip）≥ 4.5
 */
import { loadTsExports } from './lib/ts-source.mjs';

// 判据值一律从 src 的 TS 源码现编译取回，工装里不写第二份（见 lib/ts-source.mjs）。
const {
  cssVariables, readableForegroundVars, contrastInkOn, THEME_REGISTRY, themeOverrideMapFor,
  GLASS_OVERLAY, GLASS_CLEAR_OVERLAY, GLASS_ENV_BASE_COLOR,
  GLASS_FLOAT_ALPHA_FLOOR, GLASS_FLOAT_ALPHA_CEIL, glassContrast,
  ACCENT_REGISTRY,
} = loadTsExports([
  { from: './src/utils/core/apple-tokens', names: ['cssVariables', 'readableForegroundVars', 'contrastInkOn'] },
  { from: './src/styles/theme/themes', names: ['THEME_REGISTRY', 'themeOverrideMapFor', 'GLASS_OVERLAY', 'GLASS_CLEAR_OVERLAY', 'GLASS_ENV_BASE_COLOR'] },
  { from: './src/utils/core/glass-contrast', names: ['GLASS_FLOAT_ALPHA_FLOOR', 'GLASS_FLOAT_ALPHA_CEIL', 'glassContrast'] },
  { from: './src/styles/theme/accents', names: ['ACCENT_REGISTRY'] },
]);

/* ---------- 颜色求值 ---------- */
const HUE_KEYS = ['blue', 'green', 'red', 'orange', 'yellow', 'purple', 'teal', 'indigo', 'pink', 'gray'];

function parseColor(str) {
  const s = String(str).trim();
  let m = s.match(/^#([0-9a-f]{3,8})$/i);
  if (m) {
    let h = m[1];
    if (h.length === 3 || h.length === 4) h = h.split('').slice(0, 3).map((c) => c + c).join('');
    h = h.slice(0, 6);
    return [parseInt(h.slice(0, 2), 16), parseInt(h.slice(2, 4), 16), parseInt(h.slice(4, 6), 16), 1];
  }
  m = s.match(/^rgba?\(([^)]+)\)$/i);
  if (m) {
    const a = m[1].replace(/\//g, ' ').split(/[\s,]+/).filter(Boolean).map(Number);
    return [a[0], a[1], a[2], a.length > 3 ? a[3] : 1];
  }
  return null;
}

/** 递归求值：支持 var(--apple-x) 与 color-mix(in srgb, C N%, T) */
function resolve(name, map, seen = new Set()) {
  const raw = map[name] ?? cssVariables[name];
  if (raw === undefined) return null;
  if (seen.has(name)) return null;
  seen.add(name);
  const s = String(raw).trim();
  if (s.startsWith('var(')) {
    return resolve(s.slice(4, -1).trim(), map, seen);
  }
  const mix = s.match(/^color-mix\(\s*in srgb,\s*(.+?)\s+([\d.]+)%\s*,\s*(.+?)\s*\)$/i);
  if (mix) {
    const c1 = resolveExpr(mix[1], map, new Set(seen));
    const p = Number(mix[2]) / 100;
    const c2 = resolveExpr(mix[3], map, new Set(seen));
    if (!c1 || !c2) return null;
    return [
      c1[0] * p + c2[0] * (1 - p),
      c1[1] * p + c2[1] * (1 - p),
      c1[2] * p + c2[2] * (1 - p),
      c1[3] * p + c2[3] * (1 - p),
    ];
  }
  return parseColor(s);
}
function resolveExpr(expr, map, seen) {
  const e = String(expr).trim();
  if (e.startsWith('var(')) return resolve(e.slice(4, -1).trim(), map, seen);
  if (e === 'transparent') return [0, 0, 0, 0];
  return parseColor(e);
}

/** 前景合成到不透明背景上 */
const over = (fg, bg) => (fg ? [0, 1, 2].map((i) => fg[3] * fg[i] + (1 - fg[3]) * bg[i]) : null);
function lum(c) {
  const f = c.slice(0, 3).map((v) => {
    const s = v / 255;
    return s <= 0.03928 ? s / 12.92 : ((s + 0.055) / 1.055) ** 2.4;
  });
  return 0.2126 * f[0] + 0.7152 * f[1] + 0.0722 * f[2];
}
function ratio(a, b) {
  if (!a || !b) return null;
  const l1 = lum(a);
  const l2 = lum(b);
  return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05);
}

/* ---------- 每个主题的断言 ---------- */
const FLOOR = { body: 7, secondary: 4.5, tertiary: 4.5, quaternary: 3, nonText: 3, onAccent: 3, tintText: 4.5 };
const errors = [];
const rows = [];

for (const meta of THEME_REGISTRY) {
  // 与 injectCSSVariables 同构：先合并差异表，再按该主题的纯色派生 on-* 前景
  const base = { ...cssVariables, ...(themeOverrideMapFor(meta.id) ?? {}) };
  const map = { ...base, ...readableForegroundVars(base) };
  // 页面底：canvas 档合成到环境底色。
  // 原先这里有一支 `meta.id === 'liquid-glass' ? parseColor('#eef3fc')` —— 那是给
  // 「玻璃当第四种配色」那版写的，别名降级成 light+glass 之后它永远不会命中，是死码。
  // 玻璃档现在在下面的「材质」小节里单独量（它必须合成到环境底色与极值内容上，而不是实心页面底）。
  const pageBase =
    resolve('--apple-surface-canvas', map) ?? resolve('--apple-bg-secondary', map);
  const page = over(resolve('--apple-bg-secondary', map), pageBase ?? [128, 128, 128, 1]) ?? pageBase;
  const panel = over(resolve('--apple-surface-panel', map), page);
  const nav = over(resolve('--apple-surface-chrome', map), page);
  const cardBg = over(resolve('--apple-surface-panel', map), page);

  const checks = [
    ['正文/面板', ratio(resolve('--apple-text-primary', map), panel), FLOOR.body],
    ['次级文字/面板', ratio(resolve('--apple-text-secondary', map), panel), FLOOR.secondary],
    ['三级文字/导航', ratio(resolve('--apple-text-tertiary', map), nav), FLOOR.tertiary],
    ...HUE_KEYS.map((h) => ['实心 ' + h + ' 上前景', ratio(resolve('--apple-on-' + h, map) ?? resolve('--apple-on-accent', map), resolve('--apple-' + h, map)), 4.5]),
    ['正文/卡片', ratio(resolve('--apple-text-primary', map), cardBg), FLOOR.body],
  ];
  for (const h of HUE_KEYS) {
    const tint = over(resolve(`--apple-${h}-bg-subtle`, map), panel);
    const r = ratio(resolve(`--apple-${h}-text`, map), tint);
    if (r !== null) checks.push([`${h} 文字/浅染`, r, FLOOR.tintText]);
  }

  const worst = checks.reduce((a, c) => (c[1] !== null && c[1] < a[1] ? c : a), checks[0]);
  rows.push(`${meta.id.padEnd(13)} 最低 ${worst[1].toFixed(1)}（${worst[0]}）`);
  for (const [label, value, floor] of checks) {
    if (value === null) { errors.push(`${meta.id} ${label}：令牌无法解析为颜色（检查 color-mix 写法）`); continue; }
    if (value < floor) errors.push(`${meta.id} ${label} 对比度 ${value.toFixed(2)} < ${floor}`);
  }
}

/* ---------- 材质层：玻璃（含 Clear）必须先合成再量 ----------
   上面那圈只量得到四套**实心**外观。玻璃档的 canvas 是 transparent，而它现在是全新访客的
   默认材质 —— 于是"最高频的那条路径没人量"。Apple 用 adaptive tint + scroll edge 消掉这一片；
   web 上取不到身后内容的亮度，所以这里改用「按面档加厚 + 玻璃上的文字档压深/提亮」把地板钉死。
   值本身一律从 GLASS_OVERLAY / GLASS_CLEAR_OVERLAY / GLASS_ENV_BASE_COLOR 读，不抄第二份。 */
const TEXT_TIERS = [
  ['--apple-text-primary', FLOOR.body, '正文'],
  ['--apple-text-secondary', FLOOR.secondary, '次级'],
  ['--apple-text-tertiary', FLOOR.tertiary, '三级'],
  // 四级文字大量用于浮层/壳层的小号元信息（时间、计数、折叠箭头）与图标。
  // 它曾长期不在判定表里，而根级 #74747a 压在最坏照片上只有 ≈4.1。
  // 这里按 3（WCAG 1.4.11 非文本对比度）判：自适应浮层的真实落点（CEIL/FLOOR）另有
  // 4.5 的断言，而壳层（elevated）在半透明白板压纯黑的极值上，任何比三级更浅的档
  // 都到不了 4.5 —— 要么与三级合并（层级消失），要么明确降级为辅助/图标档。
  ['--apple-text-quaternary', FLOOR.nonText, '四级'],
];

/**
 * 每档面**身后是什么**，就按什么量。这张表是这节判据全部的裁量权所在，所以写成数据而不是
 * 散在 if 里 —— 要动这节的结论，动的就是"什么算这档面的最坏背景"这一列。
 *   · 流水面（panel / primary / chrome 静止态）：背景是页面自己的环境底色，可控。
 *   · 浮层壳面（elevated；顶栏在内容滚过来时由 CSS 换到这一档）：压在**任意外部内容**上 ——
 *     上传的图片、代码块都能到极值，所以最坏情况取纯黑（浅色外观）/ 纯白（深色外观）。
 *   · 浮层面板（float；菜单 / 下拉 / 弹窗 / 抽屉）：身后是本应用自己的版面，最坏是一张
 *     偏暗 / 偏亮的图（一张暗照片在菜单那块尺寸上的均值），不是纯黑纯白 —— 全站没有任何一处
 *     会在浮层正后方铺一整块极值色。
 * 最后一条是拿"浮层像玻璃"换来的，说清楚代价：elevated 那个极值把 alpha 钉在 0.84 以上，
 * 而它原先同时是菜单的底色，于是菜单成了带发丝边的白板（用户判词：根本就不是液态玻璃）。
 * 分成两档之后，顶栏继续扛极值，菜单按自己的最坏背景走 —— 两边的地板各自成立。
 */
const SURFACE_KINDS = [
  ['--apple-surface-panel', '流水面', 'env'],
  ['--apple-surface-primary', '流水面', 'env'],
  ['--apple-surface-chrome', '流水面', 'env'],
  ['--apple-surface-elevated', '浮层壳面', 'extreme'],
  // 浮层面板按**运行时上限**量：身后是最坏照片时自适应会把 alpha 抬到 CEIL。
  // 令牌里的 0.78 只是"还没测量"时的回落值（下面两端断言单独判它）。
  ['--apple-surface-float', '浮层面板', 'photo', GLASS_FLOAT_ALPHA_CEIL],
];
const WORST_PHOTO = { light: '#4a4a4a', dark: '#b5b5b5' };

if (Object.keys(GLASS_OVERLAY || {}).length === 0) {
  errors.push('读不到 GLASS_OVERLAY —— 材质层的判据在空转');
}

const glassRows = [];
for (const appearance of ['light', 'dark']) {
  for (const variant of ['glass', 'glass+clear']) {
    const merged = { ...cssVariables, ...(themeOverrideMapFor(appearance) ?? {}), ...GLASS_OVERLAY[appearance] };
    if (variant === 'glass+clear') Object.assign(merged, GLASS_CLEAR_OVERLAY[appearance]);
    const map = { ...merged, ...readableForegroundVars(merged) };
    const env = parseColor(GLASS_ENV_BASE_COLOR[appearance]);
    const basis = {
      env,
      extreme: parseColor(appearance === 'light' ? '#000000' : '#ffffff'),
      photo: parseColor(WORST_PHOTO[appearance]),
    };
    let worst = null;
    let worstNote = '';

    for (const [tierKey, floor, tierName] of TEXT_TIERS) {
      const fg = resolve(tierKey, map);
      for (const [surfKey, kind, basisKey, alphaOverride] of SURFACE_KINDS) {
        const raw = resolve(surfKey, map);
        // 自适应浮层按运行时能落到的那一档量（上限），而不是令牌里的回落 alpha
        const surf = alphaOverride && raw ? [raw[0], raw[1], raw[2], alphaOverride] : raw;
        const bg = over(surf, basis[basisKey]);
        const value = ratio(fg, bg);
        const label = `${surfKey.replace('--apple-surface-', '')}/${tierName}/${kind}`;
        if (value === null) { errors.push(`${variant}(${appearance}) ${label}：令牌解不出颜色`); continue; }
        if (value < floor) errors.push(`${variant}(${appearance}) ${label} 对比度 ${value.toFixed(2)} < ${floor}`);
        if (worst === null || value < worst) {
          worst = value;
          worstNote = `${tierName}/${kind}，门槛 ${floor}`;
        }
      }
    }
    glassRows.push(`${appearance}+${variant.replace('glass', '') || 'std'}`.padEnd(18) + ` 最低 ${(worst ?? NaN).toFixed(2)}（${worstNote}）`);
  }
}

/* ---------- 自适应浮层：它能走到的两端也必须在地板之上 ----------
   运行时（composables/ui/useGlassAdaptive.ts）按面板身后**被画出来的底色**，
   在元素上把 `--apple-surface-float` 覆写成 [FLOOR, CEIL] 之间的某个 alpha。
   中间那些值不在这里判 ——
   它们由 `minAlphaForFloor` 按同一个 floor 解出来，而解算器与本文件共用
   `utils/core/glass-contrast.ts` 这一份实现（判据不许抄第二份）。
   这里只钉两端：
   · CEIL：身后撞上位图（读不出亮度）时退回的那一档。必须扛**极值**内容，
     因为"读不出"不等于"浅" —— 那是这一层唯一不能赌的地方。
   · FLOOR：身后就是页面自己的环境底色时允许的最透一档。
   外加一条：令牌里那个"没量过"的回落值必须落在区间内，否则 JS 没跑的用户拿到的
   是一个这一节从没判过的 alpha。 */
for (const appearance of ['light', 'dark']) {
  const merged = { ...cssVariables, ...(themeOverrideMapFor(appearance) ?? {}), ...GLASS_OVERLAY[appearance] };
  const map = { ...merged, ...readableForegroundVars(merged) };
  const plate = resolve('--apple-surface-float', map);
  if (!plate) { errors.push(`glass(${appearance}) 自适应浮层：板令牌解不出颜色`); continue; }
  const rgb = [plate[0], plate[1], plate[2]];
  const extreme = parseColor(appearance === 'light' ? '#000000' : '#ffffff');
  const env = parseColor(GLASS_ENV_BASE_COLOR[appearance]);
  for (const [tierKey, tierName] of [
    ['--apple-text-tertiary', '三级'],
    ['--apple-text-quaternary', '四级'],
  ]) {
    const text = resolve(tierKey, map);
    if (!text) { errors.push(`glass(${appearance}) 自适应浮层：${tierName}文字令牌解不出颜色`); continue; }
    const twoEnds = [
      [`CEIL×极值内容（${tierName}）`, glassContrast(rgb, GLASS_FLOAT_ALPHA_CEIL, extreme, text)],
      [`FLOOR×环境底色（${tierName}）`, glassContrast(rgb, GLASS_FLOAT_ALPHA_FLOOR, env, text)],
    ];
    // 令牌回落值只判三级：四级在半透明白板 + 最坏照片上即便 0.78 也够不着 4.5，
    // 而它只在"组件已挂载、rAF 还没跑"的那一两帧里生效，运行时随即被 CEIL 那端接管。
    // 三级留着这条是因为它是壳层小字的主力，回落值不允许有可见的不可读窗口。
    if (tierKey === '--apple-text-tertiary') {
      twoEnds.push(['令牌回落×最坏照片（三级）', glassContrast(rgb, plate[3], parseColor(WORST_PHOTO[appearance]), text)]);
    }
    for (const [label, value] of twoEnds) {
      if (value < FLOOR.tertiary) {
        errors.push(`glass(${appearance}) 自适应浮层 ${label} 对比度 ${value.toFixed(2)} < ${FLOOR.tertiary}`);
      }
    }
  }
  if (plate[3] < GLASS_FLOAT_ALPHA_FLOOR || plate[3] > GLASS_FLOAT_ALPHA_CEIL) {
    errors.push(`glass(${appearance}) 的 surface-float 回落 alpha ${plate[3]} 不在运行时区间 ` +
      `[${GLASS_FLOAT_ALPHA_FLOOR}, ${GLASS_FLOAT_ALPHA_CEIL}] 内 —— JS 没跑时用户拿到的是这一节没判过的值`);
  }
}

/* ---------- 纸感 / 杂志：不支持玻璃的 base 不得继承任何玻璃装饰 ----------
   材质轴在运行时会把 glass 折叠成 solid（ui.ts 的 effectiveMaterial），但共享常量里
   若还留着镜面高光 / 斜面棱线 / 模糊，这些外观会"看起来像玻璃"，与产品定义冲突。
   判定物是**令牌值**：玻璃语言三件套在 :root 里必须解析成非玻璃装饰。 */
const nonGlassBases = THEME_REGISTRY.filter((m) => m.supportsGlass !== true);
if (!nonGlassBases.length) {
  errors.push('找不到任何不支持玻璃的主题 —— 这条判据在空转');
}
for (const meta of nonGlassBases) {
  const merged = { ...cssVariables, ...(themeOverrideMapFor(meta.id) ?? {}) };
  const pick = (key) => merged[key] ?? cssVariables[key];
  if (pick('--apple-glass-blur') !== 'none') {
    errors.push(`${meta.id} 不支持玻璃，但 --apple-glass-blur = ${pick('--apple-glass-blur')}（应为 none）`);
  }
  if (pick('--apple-glass-specular') !== 'none') {
    errors.push(`${meta.id} 不支持玻璃，但 --apple-glass-specular = ${pick('--apple-glass-specular')}（镜面高光应为 none）`);
  }
  if (!/rgba\(0,\s*0,\s*0,\s*0\)/.test(String(pick('--apple-glass-edge')))) {
    errors.push(`${meta.id} 不支持玻璃，但 --apple-glass-edge = ${pick('--apple-glass-edge')}（棱线应为透明 inset）`);
  }
}

/* ---------- 强调色档位（个性化批3）：accent × 外观 × 材质 全笛卡尔 ----------
   运行时做的事就两件（styles/theme/accents.ts 的 applyAccent）：把 `--apple-blue` 覆写成
   色板里的 hex，再用**同一个** `contrastInkOn()` 重算 `--apple-on-blue`。
   这里必须照这两件复现一遍 —— 只覆写前者、不重算后者，是这一层最容易写错的地方，
   而它的症状是"某个色相的按钮上白字看不见"，静态看代码完全正常。
   派生族（blue-text / blue-bg-subtle / selection / focus-ring）都是 color-mix 引用 var(--apple-blue)，
   覆写发生在同一个元素（:root 就是 documentElement）上，所以 resolve() 跟着走，与浏览器一致。

   **这一圈最初还有一条"恒白(on-accent)压在实心强调色上 ≥3"，2026-10-16 当场改掉了一处接线**。
   跑第一版时 8 档里 4 档在深色外观只有 2.3~2.6:1 —— 因为 `ApButton` 的 primary/danger 和
   `ApCheckbox` 的勾都写着 `cv('on-accent')`（恒白）落在 `cv('blue')` 上。
   那条断言的前提本身就是错的：教条早就规定**实心色块的前景由色值决定**（`on-{hue}` / `onSolid`），
   `on-accent` 是给图片与深色遮罩用的。内置强调色恰好够深，所以这个捷径一直没露馅，
   而"用户能自己选强调色"第一次把它照出来。三处接线已改成 `on-blue` / `on-red`，
   回归由设计教条 **G33** 拦（on-accent 出现在实心 hue 背景的同一个对象里即红，存量有棘轮）。
   留在这里的那三条才是这一圈真正判的东西：实心前景 ≥4.5、同色浅染上的文字 ≥4.5、
   强调色对面板 ≥3（焦点环与描边是非文本对比度，WCAG 1.4.11）。 */
const accentRows = [];
if (!Array.isArray(ACCENT_REGISTRY) || ACCENT_REGISTRY.length === 0) {
  errors.push('读不到 ACCENT_REGISTRY —— 强调色那一圈判据在空转');
} else {
  for (const accent of ACCENT_REGISTRY) {
    for (const meta of THEME_REGISTRY) {
      // 色板按外观的明暗档取值：editorial / warm-paper 都是浅色外观
      const appearance = meta.appearance === 'dark' ? 'dark' : 'light';
      const hex = accent[appearance];
      if (!hex) { errors.push(`强调色 ${accent.id} 缺 ${appearance} 档的取值`); continue; }
      for (const material of ['solid', 'glass']) {
        const merged = {
          ...cssVariables,
          ...(themeOverrideMapFor(meta.id) ?? {}),
          ...(material === 'glass' && GLASS_OVERLAY[appearance] ? GLASS_OVERLAY[appearance] : {}),
          '--apple-blue': hex,
        };
        const map = { ...merged, ...readableForegroundVars(merged) };
        // 背景按这一档实际落在的面上合成（玻璃档的 panel 是半透明的，必须合成后再量）
        const env = parseColor(GLASS_ENV_BASE_COLOR[appearance]);
        const panelRaw = resolve('--apple-surface-panel', map);
        const panel = over(panelRaw, panelRaw && panelRaw[3] < 1 ? env : panelRaw ?? [255, 255, 255, 1]);
        const onBlue = resolve('--apple-on-blue', map);
        const solidBlue = resolve('--apple-blue', map);
        const tint = over(resolve('--apple-blue-bg-subtle', map), panel);
        const checks = [
          ['实心强调色上的前景', ratio(onBlue, solidBlue), 4.5],
          ['同色浅染上的文字', ratio(resolve('--apple-blue-text', map), tint), 4.5],
          ['强调色本身对面板（焦点环/描边）', ratio(solidBlue, panel), 3],
        ];
        let worst = null;
        let worstLabel = '';
        for (const [label, value, floor] of checks) {
          if (value === null) { errors.push(`${accent.id}/${meta.id}+${material} ${label}：令牌解不出颜色`); continue; }
          if (value < floor) errors.push(`${accent.id}/${meta.id}+${material} ${label} 对比度 ${value.toFixed(2)} < ${floor}`);
          if (worst === null || value < worst) { worst = value; worstLabel = label; }
        }
        if (worst !== null && (accentRows[accent.id] === undefined || worst < accentRows[accent.id][0])) {
          accentRows[accent.id] = [worst, `${meta.id}+${material} ${worstLabel}`];
        }
        // 运行时覆写用的算法必须与门禁这一份是同一个函数（不是"各写一遍恰好相等"）
        if (map['--apple-on-blue'] !== contrastInkOn(hex)) {
          errors.push(`${accent.id} 的 on-blue 与 contrastInkOn(${hex}) 不一致 —— 判据与实现分叉了`);
        }
      }
    }
  }
}

if (errors.length) {
  console.error(`\n✗ 对比度检查未通过（${errors.length} 项）：`);
  for (const e of errors.slice(0, 40)) console.error('  ' + e);
  if (errors.length > 40) console.error(`  … 另有 ${errors.length - 40} 项`);
  process.exit(1);
}
console.log('ℹ 对比度：' + rows.join(' ｜ ') + `（门槛 正文${FLOOR.body} 次级${FLOOR.secondary} 按钮${FLOOR.onAccent} 浅染文字${FLOOR.tintText}）`);
console.log('ℹ 对比度（材质=玻璃，已按 alpha 合成）：' + glassRows.join(' ｜ ') + '｜ 浮层壳面压极值内容（浅外观纯黑 / 深外观纯白），浮层面板按运行时上限压最坏照片底色（' + WORST_PHOTO.light + ' / ' + WORST_PHOTO.dark + '）；四档文字全判');
if (Object.keys(accentRows).length) {
  console.log('ℹ 对比度（强调色档位，accent × 外观 × 材质 全笛卡尔）：' + Object.entries(accentRows)
    .map(([id, [v, where]]) => `${id} ${v.toFixed(2)}(${where})`).join(' ｜ ')
    + '（每档取最差那一格；门槛 实心前景4.5 浅染文字4.5 描边3 —— 恒白那一格已于批3 换成接线修正，见上面的注释）');
}
