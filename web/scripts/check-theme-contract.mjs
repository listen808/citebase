#!/usr/bin/env node
/**
 * 主题契约守卫 —— 用法：npm run check:theme（已串进 npm run check:style）
 *
 * 为什么要有这个文件：主题系统的失效方式从来不是「代码写错」，而是「某套主题漏了一个 token」
 * 和「某处绕过了 token 写死字面值」。这两类问题在肉眼比对下几乎不可见，
 * 所以把它们变成断言。
 *
 * 校验项：
 *  T1 契约键必须都在 CSS 变量注册表里（写错变量名 = 门禁 + 编译双重拦截）
 *  T2 每套主题（含 dark）必须覆盖全部契约键，且不得写未注册的键
 *  T3 THEME_REGISTRY / THEME_IDS / EXTENDED_THEME_IDS 三者必须一致
 *  T4 themeExtraCss 的键必须是已注册主题
 *  T5 基色为字面色值（#hex / rgba）的令牌，要么在契约里，要么 dark 有覆盖
 *  T6 每套主题都要登记 hljs 调色板与 Monaco 主题（渲染器不许漏主题）
 *  T7 JS 里不许再出现主题 id 字面量比较（isDarkThemeId / supportsGlassClear 是入口）
 *  T8 材质与阴影不许绕过 token（AppleShadow.* / AppleGlassBlur.* 只在 apple-tokens.ts 里定义）
 *  T10 强调色/遮罩上的文字必须用 on-accent（pure-white-canvas 在暗色下是黑的）
 *  T11 黑/白原色只准出现在令牌层（组件写死 #fff / rgba(0,0,0,..) 即绕过主题）
 *  T9 CSS 里的 [data-theme='x'] 选择器必须是已注册主题（防拼出never-matching 的死样式）
 *  T10 文字/图标压在强调色或图片遮罩上时必须用 on-accent，不许用 pure-white-canvas（暗色里它是黑的）
 */
import fs from 'fs';
import path from 'path';

const SRC = path.resolve('src');
const TOKENS = path.join(SRC, 'utils/core/apple-tokens.ts');
const THEMES = path.join(SRC, 'styles/theme/themes.ts');
const HIGHLIGHT = path.join(SRC, 'styles/theme/highlight-tokens.ts');

const read = (f) => fs.readFileSync(f, 'utf8').replace(/\r\n/g, '\n');
const rel = (f) => path.relative(SRC, f).replace(/\\/g, '/');

const walk = (d, out = []) => {
  for (const e of fs.readdirSync(d, { withFileTypes: true })) {
    const p = path.join(d, e.name);
    if (e.isDirectory()) walk(p, out);
    else if (/\.(vue|ts)$/.test(e.name) && !p.endsWith('.d.ts')) out.push(p);
  }
  return out;
};

const errors = [];

/* ---------- 解析注册表与主题差异表 ---------- */
const tokens = read(TOKENS);
const registered = new Set(
  [...tokens.matchAll(/^\s*'(--apple-[a-z0-9-]+)'\s*:/gm)].map((m) => m[1])
);

/** AppleColor / AppleNeutralColor 里的原始色值，用来把 `AppleColor.xxx` 形式的注册值解析成真实颜色 */
const primitiveColors = new Map(
  [...tokens.matchAll(/^\s{2}([A-Za-z]\w*)\s*:\s*'(#[0-9a-fA-F]{3,8}|rgba?\([^']*\))',/gm)].map(
    (m) => [m[1], m[2]]
  )
);
const resolvePrimitive = (value) => {
  const ref = value.match(/^Apple(?:Color|NeutralColor|StatusColor)\.(\w+)$/);
  return ref ? (primitiveColors.get(ref[1]) ?? value) : value;
};

const baseValues = new Map(
  [...tokens.matchAll(/^\s*'(--apple-[a-z0-9-]+)'\s*:\s*(.+?),\s*$/gm)]
    .map((m) => [m[1], resolvePrimitive(m[2].trim().replace(/^'(.*)'$/, '$1'))])
);

const themesSrc = read(THEMES);
const contract = new Set(
  [
    ...(
      themesSrc.match(/THEME_CONTRACT_KEYS = \[([\s\S]*?)\] as const/)?.[1] ?? ''
    ).matchAll(/'(--apple-[a-z0-9-]+)'/g),
  ].map((m) => m[1])
);

/** 取一段对象字面量里的 `--apple-x`: 键（只认顶层键行，注释里的不算） */
const objectKeys = (block) =>
  new Set([...block.matchAll(/^\s+'(--apple-[a-z0-9-]+)'\s*:/gm)].map((m) => m[1]));

const darkMatch = themesSrc.match(/export const DARK_THEME: ThemeOverrideMap = \{\n([\s\S]*?)\n\};/);
const themeBlocks = new Map();
if (darkMatch) themeBlocks.set('dark', objectKeys(darkMatch[1]));
const overridesMatch = themesSrc.match(/export const themeOverrides[\s\S]*?\{\n([\s\S]*?)\n\};\n/);
if (overridesMatch) {
  const re = /^ {2}'?([a-z-]+)'?: \{\n([\s\S]*?)^ {2}\},$/gm;
  for (const m of overridesMatch[1].matchAll(re)) themeBlocks.set(m[1], objectKeys(m[2]));
}

/* ---------- 解析材质轴：MATERIALS / 玻璃 overlay / 支持玻璃的 base ---------- */
const materials = [
  ...(themesSrc.match(/export const MATERIALS = \[([^\]]*)\]/)?.[1] ?? '').matchAll(/'([a-z-]+)'/g),
].map((m) => m[1]);
const glassMatch = themesSrc.match(/export const GLASS_OVERLAY:[\s\S]*?= \{\n([\s\S]*?)\n\};/);
const glassBlocks = new Map(); // 'light' | 'dark' -> Set(keys)
if (glassMatch) {
  const re = /^ {2}(light|dark): \{\n([\s\S]*?)^ {2}\},?$/gm;
  for (const m of glassMatch[1].matchAll(re)) glassBlocks.set(m[1], objectKeys(m[2]));
}

// 只在 THEME_REGISTRY 块里取 id，否则 MATERIAL_REGISTRY 的 solid/glass 会混进来
const themeRegistryBlock = themesSrc.match(/export const THEME_REGISTRY:[\s\S]*?\n\];/)?.[0] ?? '';
const registryIds = [...themeRegistryBlock.matchAll(/\{ id: '([a-z-]+)'/g)].map((m) => m[1]);
const glassBases = [...themeRegistryBlock.matchAll(/\{ id: '([a-z-]+)'[^\n]*?supportsGlass: true/g)].map((m) => m[1]);
const extendedIds = [
  ...(themesSrc.match(/EXTENDED_THEME_IDS = \[([^\]]*)\]/)?.[1] ?? '').matchAll(/'([a-z-]+)'/g),
].map((m) => m[1]);
const extraCssIds = [
  ...themesSrc.matchAll(/^ {2}'?([a-z-]+)'?: `\n:root\[data-theme='[a-z-]+'\]/gm),
].map((m) => m[1]);

if (!registered.size || !contract.size || !themeBlocks.size || registryIds.length < 2 || !glassBlocks.size || !materials.length) {
  console.error('✗ 主题契约守卫无法解析源文件（结构变了？同步改 check-theme-contract.mjs）');
  console.error({ registered: registered.size, contract: contract.size, themes: themeBlocks.size, registryIds, materials, glass: [...glassBlocks.keys()] });
  process.exit(1);
}

/* ---------- T1 契约键必须已注册 ---------- */
for (const key of contract) {
  if (!registered.has(key)) errors.push(`T1 契约键 ${key} 不在 CSS 变量注册表里`);
}

/* ---------- T2 每套主题覆盖完整且无野键 ---------- */
for (const [id, keys] of themeBlocks) {
  const missing = [...contract].filter((k) => !keys.has(k));
  if (missing.length) errors.push(`T2 主题 ${id} 漏了 ${missing.length} 个契约键：${missing.join(', ')}`);
  const unknown = [...keys].filter((k) => !registered.has(k));
  if (unknown.length) errors.push(`T2 主题 ${id} 写了未注册的键：${unknown.join(', ')}`);
}

/* ---------- T2g 玻璃 overlay：材质轴的差异表 ----------
   玻璃只覆写材质键，中性 palette 靠层叠回落 base，所以不要求覆盖全契约；
   但写进来的键必须已注册，且材质签名键必须在场，否则玻璃块形同虚设。 */
const GLASS_MATERIAL_KEYS = [
  '--apple-surface-panel',
  '--apple-surface-canvas',
  '--apple-glass-blur',
  '--apple-shadow-card',
  // 玻璃语言三件套：棱线 / 亮边 / 镜面高光。它们必须由材质层提供，
  // 非玻璃主题在 :root 里解析成透明 inset / 中性发丝边 / none（纸感、杂志因此不会"像玻璃"）。
  '--apple-glass-edge',
  '--apple-glass-rim',
  '--apple-glass-specular',
];
for (const [appearance, keys] of glassBlocks) {
  const unknown = [...keys].filter((k) => !registered.has(k));
  if (unknown.length) errors.push(`T2g 玻璃 overlay(${appearance}) 写了未注册的键：${unknown.join(', ')}`);
  const missing = GLASS_MATERIAL_KEYS.filter((k) => !keys.has(k));
  if (missing.length) errors.push(`T2g 玻璃 overlay(${appearance}) 缺材质签名键：${missing.join(', ')}`);
}
// 每个支持玻璃的 base，按其明暗必须能找到对应 overlay
for (const id of glassBases) {
  const appearance = id === 'dark' ? 'dark' : 'light';
  if (!glassBlocks.has(appearance)) errors.push(`T2g base ${id} 支持玻璃但缺 ${appearance} 玻璃 overlay`);
}
// 反过来：不支持玻璃的 base 不该出现在 glassBases 之外还声明 supportsGlass
for (const id of registryIds) {
  const declaresGlass = new RegExp(`\\{ id: '${id}'[^\\n]*?supportsGlass: true`).test(themeRegistryBlock);
  if (declaresGlass !== glassBases.includes(id)) {
    errors.push(`T2g base ${id} 的 supportsGlass 声明与解析结果不一致`);
  }
}
// 纸感 / 杂志是产品定义上的"非玻璃外观"：即使有人给它们加上 supportsGlass，
// 上面的循环也会因为 light overlay 存在而放行 —— 所以这一条要单独钉住。
for (const id of extendedIds) {
  if (glassBases.includes(id)) {
    errors.push(`T2g 扩展主题 ${id} 声明了 supportsGlass —— 纸感/杂志不提供液态玻璃材质`);
  }
}

/* ---------- T3 主题清单三处一致 ---------- */
const expected = ['light', 'dark', ...extendedIds];
if (registryIds.join(',') !== expected.join(',')) {
  errors.push(`T3 THEME_REGISTRY(${registryIds.join(',')}) 与 THEME_IDS(${expected.join(',')}) 不一致`);
}
for (const id of registryIds) {
  if (id !== 'light' && !themeBlocks.has(id)) errors.push(`T3 主题 ${id} 在注册表里但没有差异表`);
}
for (const id of themeBlocks.keys()) {
  if (!registryIds.includes(id)) errors.push(`T3 差异表 ${id} 没有登记进 THEME_REGISTRY`);
}

/* ---------- T4 额外部样式的键 ---------- */
for (const id of extraCssIds) {
  if (!registryIds.includes(id)) errors.push(`T4 themeExtraCss 里的 ${id} 不是已注册主题`);
}

/* ---------- T5 字面色值要么在契约里，要么 dark 有覆盖 ---------- */
const darkKeys = themeBlocks.get('dark') ?? new Set();
for (const [key, value] of baseValues) {
  const isLiteralColor = /^(#[0-9a-f]{3,8}|rgba?\()/i.test(value);
  const structural = /^--apple-(font-size|space|radius|z-index|breakpoint|slider)/.test(key);
  // --apple-on-* 由 injectCSSVariables 按各主题纯色派生（contrastInkOn），不需要逐主题写值
  const derivedAtInject = /^--apple-on-/.test(key);
  if (isLiteralColor && !structural && !derivedAtInject && !contract.has(key) && !darkKeys.has(key)) {
    errors.push(`T5 ${key} 是字面色值（${value}）但既不在契约里、dark 也没覆盖 → 暗色下会不匹配`);
  }
}

/* ---------- T6 渲染器不许漏主题 ----------
   hljs 调色板仍是 Record<ThemeId,…>，逐 base 登记，静态可查。
   Monaco 改成 monacoThemeForUi(base, material) 分派函数（玻璃要透明底，明/暗各一套），
   不再是逐 base 的 Record；这里只校验它存在、且 glass/paper/editorial 分支都在，
   逐组合的映射唯一性交给 themes.test.ts 的运行时断言。 */
const highlightSrc = read(HIGHLIGHT);
const hlBlock = highlightSrc.match(/HIGHLIGHT_PALETTE_BY_THEME: Record<ThemeId, HighlightPalette> = \{([\s\S]*?)\n\};/);
const mappedIn = (block) =>
  new Set([...(block?.[1] ?? '').matchAll(/^\s+'?([a-z-]+)'?:/gm)].map((m) => m[1]));
const hlMapped = mappedIn(hlBlock);
for (const id of registryIds) {
  if (!hlMapped.has(id)) errors.push(`T6 主题 ${id} 没有登记 hljs 调色板（HIGHLIGHT_PALETTE_BY_THEME）`);
}
const monacoFn = highlightSrc.match(/export function monacoThemeForUi\(([\s\S]*?)\n\}/);
if (!monacoFn) {
  errors.push('T6 找不到 monacoThemeForUi(base, material) 分派函数');
} else {
  const body = monacoFn[0];
  if (!/material === 'glass'/.test(body)) errors.push('T6 monacoThemeForUi 没有 glass 材质分支');
  if (!/MONACO_APPLE_THEMES\.glassDark/.test(body)) errors.push('T6 monacoThemeForUi 缺深色玻璃主题（glassDark）');
  for (const id of extendedIds) {
    if (!body.includes(`'${id}'`)) errors.push(`T6 monacoThemeForUi 没有处理扩展主题 ${id}`);
  }
}

/* ---------- T7 / T8 组件侧的字面量与绕过 ---------- */
const THEME_IDS_FOR_TEST = registryIds.map((id) => `'${id}'`).join('|');
const glassShadowBypass = [];
for (const f of walk(SRC)) {
  const r = rel(f);
  if (r === 'utils/core/apple-tokens.ts' || r === 'styles/theme/themes.ts' || r === 'scripts/check-theme-contract.mjs') continue;
  const text = read(f)
    .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
    .replace(/(^|[^:])\/\/[^\n]*/g, (m, p1) => p1 + ' '.repeat(m.length - p1.length));

  text.split('\n').forEach((line, i) => {
    // T7：主题 id 只准出现在 CSS 选择器 / 调色板数据里，不准参与 JS 判断
    if (!isSelectorLine(line) && new RegExp(`(theme|Theme)\\s*[=!]==?\\s*(${THEME_IDS_FOR_TEST})`).test(line)) {
      errors.push(`T7 ${r}:${i + 1} 用主题 id 字面量做判断，改用 isDarkThemeId / supportsGlassClear / themeMetaFor`);
    }
    if (/AppleShadow\.(modal|dropdown|card|cardHover|cardHoverRefined|cardRefined|subtle|level[123]|nav|innerFocus)/.test(line)) {
      glassShadowBypass.push(`${r}:${i + 1}`);
    }
    if (/AppleGlassBlur\.(standard|nav|dropdown|heavy)/.test(line)) {
      glassShadowBypass.push(`${r}:${i + 1}`);
    }
  });

  // T9：CSS 选择器里的主题 id / 材质必须存在
  for (const m of text.matchAll(/\[data-theme='([a-z-]+)'\]/g)) {
    if (!registryIds.includes(m[1])) errors.push(`T9 ${r} 的 [data-theme='${m[1]}'] 不是已注册主题（死样式）`);
  }
  for (const m of text.matchAll(/\[data-material='([a-z-]+)'\]/g)) {
    if (!materials.includes(m[1])) errors.push(`T9 ${r} 的 [data-material='${m[1]}'] 不是已注册材质（死样式）`);
  }
}
function isSelectorLine(line) {
  return /:root\[|data-theme=|data-material=|['"]apple-[a-z-]+-(?:glass-)?theme['"]|monacoThemeForUi|MONACO_APPLE_THEMES|HIGHLIGHT_PALETTE_BY_THEME|THEME_REGISTRY|MATERIAL_REGISTRY|themeExtraCss|glassExtraCss/.test(line);
}
// T8 只拦新增：遮罩层的轻模糊 AppleGlassBlur.mask 允许（它是环境效果，不随主题变）
for (const site of glassShadowBypass) {
  errors.push(`T8 ${site} 绕过主题令牌（改用 cv('shadow-*') / cv('glass-blur')）`);
}

// T10：反色文字的两种角色要分开——压在强调色/图片上恒为浅色（on-accent），
// 跟随表面反转的（tooltip chip、品牌渐变卡）才允许 pure-white-canvas，在这里显式登记。
const INVERSE_SURFACE_ALLOWLIST = new Set([
  // 气泡样式已从 ApTooltip 抽到 useTooltip（按钮与包装组件共用一份实现），
  // 登记的是「底随主题反转、文字必须跟着反转」这一处，不是某个组件文件。
  'composables/shared/useTooltip.ts',
  'views/workspace/components/SpaceRootContent.vue',
  'utils/core/apple-tokens.ts',
  'styles/theme/themes.ts',
]);
for (const f of walk(SRC)) {
  const r = rel(f);
  if (INVERSE_SURFACE_ALLOWLIST.has(r) || r.endsWith('.test.ts')) continue;
  read(f)
    .split('\n')
    .forEach((line, i) => {
      // 旧正则要求 cv('pure-white-canvas') 紧跟在 color: 之后，于是三元写法
      // color: active ? cv('pure-white-canvas') : item.color 全部放行——实测暗色下这些位置渲染成 rgb(0,0,0)。
      if (/(?:^|[^\w-])color\s*[:=][^\n]*cv\('pure-white-canvas'\)/.test(line)) {
        errors.push(`T10 ${r}:${i + 1} 强调色/遮罩上的文字应使用 cv('on-accent')（pure-white-canvas 在暗色下是黑色）`);
      }
    });
}

// T11：黑白原色只准出现在令牌层。组件写死 #fff / rgba(0,0,0,..) 就等于绕过主题，
// 需要新的浅/深语义时，先在 apple-tokens.ts 注册角色令牌（on-accent / surface-knob 就是这样来的）。
// 例外必须写理由：目前只有邮件模板（客户端会剥掉 CSS 变量）与第三方厂商身份色。
const RAW_COLOR_EXEMPT = new Map([
  ['views/admin/system/EmailConfigView.vue', 'HTML 邮件模板：客户端不支持 CSS 变量，只能内联十六进制色值'],
  ['views/ai/AIConfigView.vue', '第三方 LLM 厂商的品牌身份色（OpenAI 绿 / Anthropic 橙…）：这些色**不该**跟随主题，'
    + '跟着主题变就等于把别人的 logo 色染成我们的强调色'],
  // 2026-10-26：聚光玻璃面的镜面高光（白）与边缘斜面暗线（黑）是材质无关的固定叠加层，
  // 与令牌层 .apple-glass-layer::before 的同源液态玻璃语言；高光=白、暗线=黑，不存在对应的浅/深语义令牌
  ['styles/component/spotlight-search-styles.ts', '液态玻璃的镜面高光（白）与边缘暗线（黑）固定叠加，与令牌层同源，无浅/深语义令牌'],
]);
const stripComments = (text) =>
  text
    .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
    .replace(/(^|[^:])\/\/[^\n]*/g, (m, p1) => p1 + ' '.repeat(m.length - p1.length))
    .replace(/<!--[\s\S]*?-->/g, (m) => m.replace(/[^\n]/g, ' '));
for (const f of walk(SRC)) {
  const r = rel(f);
  if (r === 'utils/core/apple-tokens.ts' || r.startsWith('styles/theme/') || r.endsWith('.test.ts')) continue;
  if (RAW_COLOR_EXEMPT.has(r)) continue;
  stripComments(read(f))
    .split('\n')
    .forEach((line, i) => {
      if (
        /#(?:fff|ffffff|000|000000)(?![0-9a-f])/i.test(line) ||
        /rgba?\(\s*(?:255,\s*255,\s*255|0,\s*0,\s*0)\s*[,/)]/i.test(line)
      ) {
        errors.push(`T11 ${r}:${i + 1} 写死了黑/白原色，改用主题令牌或 withAlpha(resolvedColor('…'))`);
      }
    });
}

/**
 * T13：**非黑白的字面 hex** 也只准住在令牌层。
 * T11 只拦黑/白，于是"品牌色 / 图表色 / 状态色"这一族整类从它眼下走过 —— 组件里写死
 * `#10A37F` 与写死 `#fff` 是同一件事（绕过主题），只是 T11 的射程没覆盖到。
 * 实测全 src 只有两处真需要字面 hex（邮件模板、LLM 厂商身份色），所以这里用
 * **带理由的登记表**而不是"只降不升"的基线：27 处能逐条说清，基线反而把话说糊。
 * 三条反假阳，每条都是这一版实测踩出来的：
 *  ① `&#039;`（HTML 实体）里的 `#039` 不是颜色 —— 前面有 `&` 就排除；
 *  ② `<template #default>` 的 `#defa` 会被当成 4 位十六进制 —— 后面紧跟字母数字就不算颜色；
 *  ③ 长度只认 3/4/6/8，`#12345` 这种半截串不当颜色处理。
 */
const HEX_COLOR = /(?<!&)#([0-9a-fA-F]{8}|[0-9a-fA-F]{6}|[0-9a-fA-F]{4}|[0-9a-fA-F]{3})(?![0-9a-zA-Z])/g;
for (const f of walk(SRC)) {
  const r = rel(f);
  if (r === 'utils/core/apple-tokens.ts' || r.startsWith('styles/theme/') || r.endsWith('.test.ts')) continue;
  if (RAW_COLOR_EXEMPT.has(r)) continue;
  stripComments(read(f))
    .split('\n')
    .forEach((line, i) => {
      const hits = line.match(HEX_COLOR);
      if (hits) {
        errors.push(`T13 ${r}:${i + 1} 写死了字面颜色 ${hits.join(' ')} —— 注册进令牌层（或主题差异表），`
          + `确需字面量（第三方品牌色 / 邮件模板）就在 RAW_COLOR_EXEMPT 带理由登记`);
      }
    });
}

/* ---------- T14 画布类渲染器不许漏接线 ----------
   T6 管"每套主题都登记了渲染器调色板"，但它看不见**新的画布消费点根本没去取调色板**。
   echarts 就是活例：三处 `echarts.init()` 直接把服务端/模型给的 option 画进 canvas，
   既不在主题切换时重绘，也从不读令牌 —— 静态门禁全绿（源码里确实没写死颜色），
   用户切到深色却看到一套浅底图表。判据只能是结构性的：
   **谁 init 了画布，谁就必须从 utils/core/echarts-theme 取东西**（applyChartTheme 或 subscribeTheme），
   或经由 useChartLifecycle（2026-10-19 收口后的共享生命周期出口，内部走 subscribeTheme）。
   以后换/加图表库（leaflet、自绘 canvas）要把这条一起扩，别等下一次"切主题没反应"再补。 */
const CANVAS_INIT = /\becharts\.init\(|new Chart\(/;
for (const f of walk(SRC)) {
  const r = rel(f);
  if (r === 'utils/core/echarts-theme.ts' || r === 'composables/ui/useChartLifecycle.ts' || r.endsWith('.test.ts')) continue;
  const text = read(f);
  if (!CANVAS_INIT.test(text)) continue;
  if (!/from ['"]@\/utils\/core\/echarts-theme['"]|useChartLifecycle/.test(text)) {
    errors.push(`T14 ${r} 直接 init 了画布却没接 echarts-theme —— canvas 不跟 CSS 变量，切主题不会重画`);
  }
}

// T12：一种颜色只有一个名字。
// 历史上表面阶梯有五档，另外还挂着五个转发别名（bg-primary / bg-card / bg-elevated /
// glass-bg / glass-bg-nav）。别名的害处不是"多五个名字"，而是它把阶梯值二次绑定：
// 主题只要覆写别名就反向遮蔽阶梯（浅深改一半 → "内容区和上栏颜色不一致"），
// 而组件用别名时看不出自己踩的是哪一档，玻璃主题下 canvas 是 transparent，
// 蹭 bg-primary 的输入框直接变成一个洞。五个别名已全删，这一段是"不得复活"的门。
const SURFACE_ALIASES = [
  '--apple-bg-primary',
  '--apple-bg-card',
  '--apple-bg-elevated',
  '--apple-glass-bg',
  '--apple-glass-bg-nav',
];
for (const [id, keys] of themeBlocks) {
  const shadowed = SURFACE_ALIASES.filter((k) => keys.has(k));
  if (shadowed.length) errors.push(`T12 主题 ${id} 又写回了表面别名 ${shadowed.join(', ')}；阶梯只准写 surface-*`);
}
for (const [appearance, keys] of glassBlocks) {
  const shadowed = SURFACE_ALIASES.filter((k) => keys.has(k));
  if (shadowed.length) errors.push(`T12 玻璃 overlay(${appearance}) 又写回了表面别名 ${shadowed.join(', ')}；阶梯只准写 surface-*`);
}
for (const key of SURFACE_ALIASES) {
  if (baseValues.has(key)) errors.push(`T12 ${key} 是已删除的表面别名，不得回到 :root；要用就写它原先指向的 surface-* 角色`);
  if (contract.has(key)) errors.push(`T12 ${key} 是别名，不该进主题契约（契约里应写 surface-* 角色）`);
}
// 泛化：:root 里任何"值恰好是另一个 var(--apple-x)"的条目都是同层重名。
// 唯一的例外是语义状态色——它不是同一个概念的第二个名字，而是「状态 → 纯色」的两层。
const FORWARD_ALLOW = new Map([
  ['--apple-success', 'green'],
  ['--apple-warning', 'orange'],
  ['--apple-error', 'red'],
]);
for (const [key, value] of baseValues) {
  const ref = /^var\(--apple-([a-z0-9-]+)\)$/.exec(value);
  if (!ref) continue;
  if (FORWARD_ALLOW.get(key) === ref[1]) continue;
  errors.push(`T12 :root 里 ${key} 只是 ${value} 的转发别名；同一个概念只留一个名字，消费者请直接写正名`);
}

/* ---------- T15 个性化档位与时长令牌必须自洽（2026-10-16 批2）----------
   这一层的失效方式全部是"少登记一半"，而且没有任何一处会报错：
   - 加一档动效强度忘了给倍数 → resolveMotionMode 交出 undefined，`String(undefined)` 写进
     `--apple-motion-scale`，CSS 拿到非法值，**全站时长回落到令牌里的 1**：选了档位等于没选；
   - 时长表加了一档但没发令牌 → 组件引用 var(--apple-dur-xxx) 解析成空，那条 transition 整条失效
     （和"AppleTransition 拼出 `opacity all 0.15s` 被浏览器丢掉"是同一类静默失效）；
   - DOM 属性名与 CSS 选择器对不上 → 档位写了、样式从没命中过。
   所以这里判结构，不判观感。 */
const PERS = path.join(SRC, 'styles/theme/personalization.ts');
if (fs.existsSync(PERS)) {
  const pers = read(PERS);
  const list = (name) => {
    const m = pers.match(new RegExp(`${name}\\s*=\\s*\\[([^\\]]*)\\]`));
    return m ? [...m[1].matchAll(/'([^']+)'/g)].map((x) => x[1]) : [];
  };
  const axes = [
    ['MOTION_MODES', 'MOTION_SCALE'],
    ['CONTRAST_MODES', null],
    ['TRANSPARENCY_MODES', null],
    // persona 没有 auto（系统里没有“要不要入场编排”这个开关），默认档是 plain。
    // 判据按每条自己的默认值核，不许一刀切要求含 auto —— Go 侧那条同一批也踩过这个坑。
    ['PERSONA_MODES', null],
  ];
  for (const [listName, scaleName] of axes) {
    const modes = list(listName);
    if (!modes.length) {
      errors.push(`T15 ${listName} 解析不出档位 —— 判据引用的结构已变了，先修判据再说代码`);
      continue;
    }
    const wantDefault = listName === 'PERSONA_MODES' ? 'plain' : 'auto';
    if (!modes.includes(wantDefault)) errors.push(`T15 ${listName} 里没有 '${wantDefault}'：默认档不许缺席`);
    if (scaleName) {
      const scaleBlock = pers.match(new RegExp(`${scaleName}\\s*[:=][\\s\\S]*?\\};`))?.[0] || '';
      for (const mode of modes.filter((m) => m !== 'auto')) {
        if (!new RegExp(`${mode}:`).test(scaleBlock)) {
          errors.push(`T15 动效档 ${mode} 在 ${scaleName} 里没有倍数 —— 选了它等于写进一个 undefined，全站时长回落到 1`);
        }
      }
    }
  }
  // DOM 属性必须真的在样式里有对应选择器
  const tokensText = read(TOKENS);
  for (const attr of ['data-motion', 'data-a11y-contrast', 'data-a11y-transparency']) {
    if (!new RegExp(`setAttribute\\('${attr}'`).test(pers)) {
      errors.push(`T15 personalization.ts 没再写 ${attr}，但样式层还按它判断 —— 两处要一起搬家`);
      continue;
    }
    const inCss = new RegExp(`\\[${attr}=`).test(read(THEMES) + tokensText);
    if (!inCss) errors.push(`T15 样式层没有 [${attr}=…] 分支 —— 这一档写了 DOM 也没人读（接线全对、效果为零）`);
  }
}
// 时长表 ↔ 时长令牌必须一一对应
const tokensSrc = read(TOKENS);
const baseBlock = tokensSrc.match(/BASE_DURATION_MS\s*=\s*\{([\s\S]*?)\n\}/)?.[1] || '';
const durKeys = [...baseBlock.matchAll(/^\s*([A-Za-z][\w]*):\s*\d+/gm)].map((m) => m[1]);
const durVarBlock = tokensSrc.match(/durationVariables\s*=\s*\{([\s\S]*?)\n\}/)?.[1] || '';
const emitted = new Set([...durVarBlock.matchAll(/'--apple-dur-([A-Za-z][\w]*)'/g)].map((m) => m[1]));
if (!durKeys.length) errors.push('T15 解析不出 BASE_DURATION_MS —— 判据引用的结构已变了');
for (const k of durKeys) if (!emitted.has(k)) errors.push(`T15 时长档 ${k} 没发令牌 --apple-dur-${k}：引用它的 transition 会整条失效`);
for (const k of emitted) if (!durKeys.includes(k)) errors.push(`T15 --apple-dur-${k} 在 BASE_DURATION_MS 里没有这一档（多余的名字会被当成合法令牌用掉，值却是未定义的）`);

/* ---------- T16 强调色色板必须自洽（批3）----------
   色板是这一层唯一"错了也看不出来"的数据结构：少一枚 hex，运行时把 `--apple-blue`
   覆写成 "undefined"，整条染色族（选中态 / 焦点环 / 浅染 / callout）一起失效，
   而界面上的表现是"主题好像没切过去"。`check:contrast` 按注册表算，看不见 DOM 上的覆写值，
   所以这一格只能在清单侧拦。labelKey 也在这里核：注册表里的名字键是**数据**，
   `i18n-msg-audit` 只扫 `$t('a.b')` 那种静态引用，扫不到它。 */
const ACCENTS = path.join(SRC, 'styles/theme/accents.ts');
if (fs.existsSync(ACCENTS)) {
  const accents = read(ACCENTS);
  const entries = [...accents.matchAll(/\{\s*id:\s*'([a-z]+)',[^]*?labelKey:\s*'([^']+)',[^]*?light:\s*'(#[0-9a-fA-F]{6})',\s*dark:\s*'(#[0-9a-fA-F]{6})'\s*\}/g)];
  // 两条判据合起来才拦得住"少一枚 hex"：只数条目的话，删掉 dark 会让那一档**解析不出来**，
  // 于是条目数从 8 掉到 7 —— 依然 ≥4，看起来什么都没发生。按 id 出现次数与成功解析次数对账，
  // 才是"每一档都完整"的那个意思（本仓吃过好几次"合法集自己抄成空集/半集"的亏）。
  const idCount = (accents.match(/\bid: '/g) || []).length;
  if (entries.length !== idCount) {
    errors.push(`T16 ACCENT_REGISTRY 里有 ${idCount} 个 id 但只有 ${entries.length} 档解析成功 —— 有档位缺 labelKey 或缺 6 位 hex（少一枚 hex 的症状是运行时把 --apple-blue 覆写成 "undefined"，整条染色族一起失效）`);
  }
  if (entries.length < 4) {
    errors.push(`T16 只解析出 ${entries.length} 档强调色 —— 判据引用的结构已变了，先修判据再说结论`);
  }
  const seenIds = new Set();
  const localeFiles = ['i18n/locales/zh-CN.ts', 'i18n/locales/en-US.ts'].map((p) => ({ p, text: read(path.join(SRC, p)) }));
  for (const m of entries) {
    const [, id, labelKey] = m;
    if (seenIds.has(id)) errors.push(`T16 强调色 ${id} 登记了两次 —— 服务端清单只能有一条`);
    seenIds.add(id);
    if (labelKey.startsWith('chrome.')) {
      for (const { p, text } of localeFiles) {
        const leaf = labelKey.split('.').pop();
        if (!new RegExp(`\\n\\s*${leaf}:`).test(text)) {
          errors.push(`T16 强调色 ${id} 的名字键 ${labelKey} 不在 ${p} 里 —— 界面上会印成键名，而 $t 是运行时才知道的（静态 i18n 巡检扫不到数据里的键）`);
        }
      }
    }
  }
  if (!/ACCENT_AUTO\s*=\s*'auto'/.test(accents)) errors.push("T16 强调色清单里没有 auto：默认档必须是不覆盖，而不是替用户选一枚颜色");
  if (!/export const ACCENT_IDS = \[ACCENT_AUTO,/.test(accents)) {
    errors.push('T16 ACCENT_IDS 没有把 auto 写在清单里（上一批的坑：伪取值住在另一个常量，跨语言判据只比主清单就会漏）');
  }
}

/* ---------- T17 入场编排必须整体被档位关住（批4）----------
   这一段判的是"缺席即无效果"：默认档 plain 下，DOM 上有 `.apple-persona-stack` 类名，
   但**没有任何一条规则应该命中它**。只要有一条编排规则没被 `[data-persona='signature']` 套住，
   全站新访客第一眼就带动画 —— 而那是一个没人会在设置页里找得到开关的行为改变。
   顺带核关键帧只动 transform/opacity：G17/G27 管的是 transition 声明，@keyframes 里的
   `width`/`top` 它们看不见（动画每帧重排比过渡更贵）。 */
const PERSONA_KEYFRAMES = ['apple-persona-rise', 'apple-brand-rise', 'apple-node-arrive'];
if (!tokensSrc.includes('function personaCss')) {
  errors.push('T17 找不到 personaCss() —— 判据引用的结构已变了，先修判据再说代码');
} else {
  for (const kf of PERSONA_KEYFRAMES) {
    if (!tokensSrc.includes(`@keyframes ${kf}`)) {
      errors.push(`T17 关键帧 ${kf} 没了，但还有选择器在引用它（G19 那类"定义了没人用"的反向问题）`);
    }
    // 逐个关键帧自己取体：整段抽取会被模板字符串里"顶格的 }"提前截断（第一版就这么误判过两次）
    const body = tokensSrc.match(new RegExp(`@keyframes ${kf}\\s*\\{([\\s\\S]*?)\\n\\s*\\}`))?.[1] || '';
    const bad = body.match(/\b(width|height|top|left|right|bottom|margin|padding|font-size)\s*:/g) || [];
    if (bad.length) errors.push(`T17 关键帧 ${kf} 动了布局属性 ${[...new Set(bad)].join(', ')} —— 只准 transform/opacity（G17/G27 管不到 @keyframes）`);
  }
  // 选择器行逐条判：凡引用这两个类名的规则，都必须在 :root[data-persona='signature'] 之下
  const selectorLines = tokensSrc
    .split('\n')
    .map((l) => l.trim())
    .filter((l) => (l.includes('.apple-persona-stack') || l.includes('.ap-brand-enter')) && l.endsWith('{'));
  for (const sel of selectorLines) {
    if (!sel.includes("[data-persona='signature']")) {
      errors.push(`T17 编排规则 "${sel.slice(0, 70)}" 没被 :root[data-persona='signature'] 套住 —— 默认档 plain 的用户也会看到入场动画`);
    }
  }
  if (selectorLines.length === 0) errors.push('T17 一条编排选择器都没扫到 —— 判据在空转，先确认 personaCss 还在不在');
}

if (errors.length) {
  console.error(`\n✗ 主题契约检查未通过（${errors.length} 项）：`);
  for (const e of errors.slice(0, 40)) console.error('  ' + e);
  if (errors.length > 40) console.error(`  … 另有 ${errors.length - 40} 项`);
  process.exit(1);
}
console.log(
  `ℹ 主题契约：${contract.size} 个必填 token × ${registryIds.length} 套主题全覆盖；` +
    `渲染器调色板已登记；组件侧无令牌绕过`
);
