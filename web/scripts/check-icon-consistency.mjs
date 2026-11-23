#!/usr/bin/env node
/**
 * 图标一致性守卫 —— 用法：npm run check:icons（已串进 npm run check:style）
 *
 * 起因：全站有 5 条图标路径并存（lucide-vue-next 直接 import / utils/icons 注册表 /
 * lucide 数据 + morphicons / 数据驱动 <component :is> / 文本 glyph），门禁几乎不管图标，
 * 于是出现「ApIcon name 不在注册表里 → 静默渲染空白」这类肉眼看不见的 bug。
 *
 * 分两类：
 *  HARD（必须 0，否则红）：
 *   I1 注册表完整性：每个 <ApIcon name="X"> / :name="'X'" 字面量都必须是 icons.ts 的键
 *   I3 手绘图标：.vue 模板里不许出现 <svg>（图标一律走 lucide；例外见 INLINE_SVG_ALLOW）
 *   I5 按钮里的图标只有一档尺寸（BUTTON_ICON_SIZE=16；2026-10-13 拍板"所有 <button> 都 16"）
 *   I6 图标尺寸只准落 AppleIconSize 注册档（档位从 apple-tokens.ts 读，门禁不抄第二份数字）
 *   I7 图标位画 emoji —— 它没有尺寸档也没有描边，等于第三套图标族
 *   I8 ≥24px 的图标必须解出描边宽度（走 ApIcon 的强制点，或自己显式传 stroke-width）
 *   I9 模板里的裸图标标签（<Send /> 这类）必须 import 或全局注册，否则渲染成空元素
 *  RATCHET（只准降不准升，超过基线即红）：
 *   I2 glyph-as-icon：把 ×→←↑↓ 当图标用的元素文本
 *   I4 一文件多路径：同一 .vue 里混用 {直接 lucide-vue-next} 与 {ApIcon 注册表}
 *   I6b 注册表外仍写死的尺寸（12/14 这些密集档，逐处收编前先进基线）
 *
 * 基线是当前实测值；每修一处就把基线调低，直到归零。
 */
import fs from 'fs';
import path from 'path';

const SRC = path.resolve('src');
const ICONS = path.join(SRC, 'utils/icons.ts');
const TOKENS = path.join(SRC, 'utils/core/apple-tokens.ts');

const read = (f) => fs.readFileSync(f, 'utf8').replace(/\r\n/g, '\n');
/** 按钮里的图标尺寸只准这一档 */
const BUTTON_ICON_SIZE = 16;
/** 只看模板部分：脚本里的字符串常量不该被当成"按钮里的图标" */
const tplOf = (text) => {
  const i = text.indexOf('<script');
  return i > 0 ? text.slice(0, i) : text;
};
/** 只取 <template> 本体：<script setup> 写在文件头时 tplOf 切不到，I7/I9 会误扫脚本里的泛型 */
const templateOf = (text) => /<template>[\s\S]*<\/template>/.exec(text)?.[0] ?? '';

/* ---------- 图标尺寸档：从 token 里读，门禁不抄第二份数字 ---------- */
// 档位表搬进 apple-tokens 的 AppleIconSize 之前，check-design-doctrine 的 G20 自己抄了
// 一份 11 个数的宽表（12/14/22/28/32 全放行），于是"唯一标尺"有两把尺、且松的那把先绿。
// 现在判据跟着表走：改表即改判据，两处不可能再分叉。
const iconSizeTiers = new Set(
  [...((/export const AppleIconSize\s*=\s*\{([\s\S]*?)\n\}/.exec(read(TOKENS))?.[1] ?? '').matchAll(/^\s+\w+:\s*(\d+)/gm))].map((m) => Number(m[1])),
);
if (!iconSizeTiers.size) {
  console.error('✗ 无法解析 apple-tokens.ts 的 AppleIconSize（结构变了？同步改 check-icon-consistency.mjs）');
  process.exit(1);
}

/* ---------- 图标元素判定 ---------- */
/** 除注册表组件外，全站"图标"只有三种写法：lucide 直接 import 的名字、ApIcon、component :is */
const ICON_COMPONENT_TAGS = new Set(['ApIcon', 'component', 'ApMorphIcon']);
const lucideTagsOf = (text) =>
  new Set(
    [...text.matchAll(/import\s*\{([^}]*)\}\s*from\s*['"]lucide-vue-next['"]/g)].flatMap((m) =>
      m[1].split(',').map((s) => s.trim().split(/\s+as\s+/).pop()).filter(Boolean),
    ),
  );
const isIconTag = (tag, lucide) => ICON_COMPONENT_TAGS.has(tag) || lucide.has(tag);
/** 行号由匹配起点前的换行数算出（违例报告要能直接跳过去） */
const lineOf = (text, index) => text.slice(0, index).split('\n').length;
const SIZE_ATTR = /(?:^|[\s:])size="([^"]*)"/;

/* ---------- emoji：图标位的判据 ---------- */
// emoji 没有尺寸档、没有描边、不随主题变，把它当 UI 图标就等于引入第三套图标族
// （docTemplates 曾经的 📋📝🚀 就是这么混进模板卡的，前四条判据一条都数不到它）。
/**
 * emoji 判据只看基字符区。`\uFE0F`（变体选择符）是**组合符**，放进字符类里
 * ESLint 会按 no-misleading-character-class 判错（它会让 `[a\uFE0F]` 看起来像
 * 匹配两个码位，实际匹配的是三个独立码位），而且我们本来也不需要单独认出 VS16 ——
 * 带不带它，前面的基字符区已经先命中了。
 */
const EMOJI_RE = /[\u{1F000}-\u{1FAFF}\u{2600}-\u{27BF}\u{2B00}-\u{2BFF}]/u;
// 例外：emoji 是**用户数据**本身时不算图标位。每条都要写清为什么，新增必须先想好这句。
const EMOJI_DATA_ALLOW = new Map([
  // 每篇文档自己的 emoji 图标（存库的 document.icon，可在属性面板编辑），
  // 回落值 📄 是"这条数据为空"的默认，不是 UI 的图标槽 —— 它是被展示的内容，不是展示它的那只手。
  ['components/space/sidebar/DocProperties.vue', 'document.icon 是用户数据（每篇文档的 emoji），不是 UI 图标位'],
]);


// --selftest：判据本身要能红。不改真源码，直接喂夹具模板 ——
// "全站 0 违例"既可能是真干净，也可能是正则根本没匹配上，只有反向夹具分得出来。
if (process.argv.includes('--selftest')) {
  const fired = buttonIconSizeIssues(`<template><ApButton><Zap :size="18" /></ApButton><button><ChevronDown size="14" /></button></template>`, 'fixture.vue');
  const quiet = buttonIconSizeIssues(`<template><ApButton><Zap :size="16" /></ApButton><Zap :size="18" /></template>`, 'fixture.vue');
  // I6：注册表外的写死尺寸要抓到；走 AppleIconSize 的表达式不能误伤
  const offFixture = `<template><Folder :size="13" /><ApIcon name="Star" :size="15" /></template>`;
  const onRegistry = `<template><ApIcon name="Star" :size="AppleIconSize.hero" /><Folder :size="16" /></template>`;
  const off = iconSizeRegistryIssues(offFixture, offFixture, 'fixture.vue', lucideTagsOf(`import { Folder } from 'lucide-vue-next'`), iconSizeTiers);
  const on = iconSizeRegistryIssues(onRegistry, onRegistry, 'fixture.vue', lucideTagsOf(onRegistry), iconSizeTiers);
  // I6：数据对象里的 iconSize: 13 同样算写死（ApStatCard 那类档位表就长这样）
  const dataOff = iconSizeRegistryIssues('const c = { iconSize: 13 };', 'const c = { iconSize: 13 };', 'fixture.ts', new Set(), iconSizeTiers);
  // I7：图标位的 emoji 红；已登记为"用户数据"的那一处不报
  const emojiBad = emojiIconIssues(`<template><span :style="iconStyle">📋</span></template>`, 'fixture.vue');
  const emojiOk = emojiIconIssues(`<template><span :style="iconStyle">${'📄'}</span></template>`, 'components/space/sidebar/DocProperties.vue');
  const emojiField = emojiIconIssues(`const t = [{ icon: '🚀' }];`, 'fixture.ts');
  // I8：≥24px 却没解描边要红；走 ApIcon 或显式 stroke-width 都不报
  const strokeBad = strokeWidthIssues(`<template><Folder :size="48" /><ApIcon name="Star" :size="40" /><XCircle :size="32" :stroke-width="iconStrokeFor(32)" /></template>`, 'fixture.vue', lucideTagsOf(`import { Folder, XCircle } from 'lucide-vue-next'`));
  // I9：裸图标标签没 import 要红；ApIcon 写法与已 import 的组件都不报
  const bareRegistry = new Set(['Send', 'Square', 'Zap']);
  const bareBad = bareIconTagIssues(`<template><ApButton><Send :size="16" /></ApButton></template>`, `import ApButton from '@/components/apple/ApButton.vue'`, 'fixture.vue', bareRegistry);
  const bareOk = bareIconTagIssues(`<template><ApButton><ApIcon name="Send" :size="16" /></ApButton></template>`, '', 'fixture.vue', bareRegistry);
  const bareImported = bareIconTagIssues(`<template><Send :size="16" /></template>`, `import { Send } from 'lucide-vue-next'`, 'fixture.vue', bareRegistry);
  const cases = [
    [`I5 抓到按钮里的 18px / 14px 图标（实得 ${fired.length} 条）`, fired.length === 2],
    [`I5 不误报按钮外的 18px（实得 ${quiet.length} 条）`, quiet.length === 0],
    ['I5 全站源码当前无违例', buttonIconSizeIssues(tplOf(read(path.join(SRC, 'components/apple/ApSidebar.vue'))), 'apple/ApSidebar.vue').length === 0],
    [`I6 抓到注册表外的 13px / 15px（实得 ${off.length} 条）`, off.length === 2],
    [`I6 不误报 AppleIconSize 与在档的 16px（实得 ${on.length} 条）`, on.length === 0],
    [`I6 抓到数据对象里写死的 iconSize（实得 ${dataOff.length} 条）`, dataOff.length === 1],
    [`I7 抓到图标位的 emoji（实得 ${emojiBad.length} 条）`, emojiBad.length === 1],
    [`I7 不误报登记为用户数据的那处（实得 ${emojiOk.length} 条）`, emojiOk.length === 0],
    [`I7 抓到数据里 icon: '🚀' 这种把 emoji 当图标的写法（实得 ${emojiField.length} 条）`, emojiField.length === 1],
    [`I8 只漏下没描边的 48px 裸图标（实得 ${strokeBad.length} 条）`, strokeBad.length === 1 && /48/.test(strokeBad[0])],
    ['I8 全站源码当前无违例', strokeWidthIssues(read(path.join(SRC, 'components/apple/ApEmpty.vue')), 'apple/ApEmpty.vue', lucideTagsOf(read(path.join(SRC, 'components/apple/ApEmpty.vue')))).length === 0],
    [`I9 抓到未 import 的裸图标标签（实得 ${bareBad.length} 条）`, bareBad.length === 1],
    [`I9 不误报 ApIcon 写法（实得 ${bareOk.length} 条）`, bareOk.length === 0],
    [`I9 不误报已 import 的图标组件（实得 ${bareImported.length} 条）`, bareImported.length === 0],
  ];
  let bad = 0;
  for (const [name, ok] of cases) { if (!ok) bad++; console.log(`${ok ? '✓' : '✗'} ${name}`); }
  console.log(bad ? `✗ ${bad} 条没通过 —— 判据写错了` : 'I5/I6/I7/I8/I9 判据正反都成立');
  process.exit(bad ? 1 : 0);
}
const rel = (f) => path.relative(SRC, f).replace(/\\/g, '/');
const walk = (d, re = /\.vue$/, out = []) => {
  for (const e of fs.readdirSync(d, { withFileTypes: true })) {
    const p = path.join(d, e.name);
    if (e.isDirectory()) walk(p, re, out);
    else if (re.test(e.name)) out.push(p);
  }
  return out;
};

/* ---------- 解析注册表键 ---------- */
const iconsSrc = read(ICONS);
const objBody = iconsSrc.match(/export const icons\s*=\s*\{([\s\S]*?)\n\};/)?.[1] ?? '';
const registry = new Set([...objBody.matchAll(/^\s+([A-Za-z]\w*)\s*,\s*$/gm)].map((m) => m[1]));
if (!registry.size) {
  console.error('✗ 无法解析 icons.ts 注册表（结构变了？同步改 check-icon-consistency.mjs）');
  process.exit(1);
}

/* ---------- 基线（只准降不准升）---------- */
// 当前实测：glyph-as-icon 已归零（× 等已换成 lucide）；混路径文件剩 8 个待烧：
//   4 个 AI/SSO 视图（views/ai/*, views/auth/SSOCallbackView.vue）+
//   4 个 admin 视图（MediaManageView / SSOConfigView / PrivilegeListView / RoleManageView），
//   后者保留 lucide 直接 import 仅因 <component :is> 动态渲染走注册表外的组件引用（规则 4 不可转），
//   非可消除的双轨，待 <component :is> 站点改用数据驱动 ApIcon 后再降基线。
// offRegistrySizes 是 12/14 这类「密集控件里 16 的复制品」：收编要逐处看版面，
// 先按 2026-11 实测数进基线，只准降不准升 —— 新写一处不在档上的尺寸立刻红。
const BASELINE = {
  glyphAsIcon: 0,
  mixedPathFiles: 8,
  offRegistrySizes: 0,
};

const errors = [];
const stripComments = (t) =>
  t.replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
    .replace(/(^|[^:])\/\/[^\n]*/g, (m, p1) => p1 + ' '.repeat(m.length - p1.length))
    .replace(/<!--[\s\S]*?-->/g, (m) => m.replace(/[^\n]/g, ' '));

const INLINE_SVG_ALLOW = new Set([
  // 品牌标记本身（全站唯一 logo 源，不是图标）——见 components/apple/ApBrand.vue
  'components/apple/ApBrand.vue',
]);

let glyphCount = 0;
let mixedCount = 0;
const mixedFiles = [];
let offRegistryCount = 0;
const offRegistrySites = [];

for (const f of walk(SRC)) {
  const r = rel(f);
  if (r === 'utils/icons.ts') continue;
  const raw = read(f);
  const text = stripComments(raw);

  // I1 注册表完整性：静态 ApIcon name
  for (const m of text.matchAll(/<ApIcon\b[^>]*?\sname="([A-Za-z]\w*)"/g)) {
    if (!registry.has(m[1])) errors.push(`I1 ${r} <ApIcon name="${m[1]}"> 不在注册表里 → 渲染空白`);
  }
  for (const m of text.matchAll(/<ApIcon\b[^>]*?:name="'([A-Za-z]\w*)'"/g)) {
    if (!registry.has(m[1])) errors.push(`I1 ${r} <ApIcon :name="'${m[1]}'"> 不在注册表里 → 渲染空白`);
  }

  // I3 手绘图标：.vue 模板里的 <svg>
  if (/<svg[\s>]/.test(text) && !INLINE_SVG_ALLOW.has(r)) {
    errors.push(`I3 ${r} 出现手绘 <svg> 图标（改用 lucide 组件；确需例外请登记 INLINE_SVG_ALLOW）`);
  }

  // I2 glyph-as-icon：×→←↑↓ 作为元素文本
  glyphCount += [...text.matchAll(/>\s*[×→←↑↓]\s*(?:<\/|$)/g)].length;

  // I4 一文件多路径：直接 lucide-vue-next 组件 与 ApIcon 注册表混用
  const usesApIcon = /<ApIcon\b/.test(text);
  const usesDirectLucide = /from ['"]lucide-vue-next['"]/.test(text);
  if (usesApIcon && usesDirectLucide) {
    mixedCount++;
    mixedFiles.push(r);
  }

  // I5 按钮里的图标只有一档尺寸（16）。
  // 尺寸归控件管，不归每个调用点各写一遍：2026-10-13 实测同一档 32px 高的按钮里混着
  // 18（侧边栏导航项）/ 14（顶栏图标按钮、箭头）/ 16（工具栏）三种，运行时按实例数算是 961/226/20。
  // 用户拍板"所有 <button> 都 16"，这条负责让它不再漂回去。
  for (const msg of buttonIconSizeIssues(tplOf(text), r)) errors.push(msg);

  // I6 / I7 / I8 / I9：尺寸档、emoji、大尺寸描边、裸图标标签
  const lucide = lucideTagsOf(text);
  const off = iconSizeRegistryIssues(tplOf(text), text, r, lucide, iconSizeTiers);
  offRegistryCount += off.length;
  offRegistrySites.push(...off);
  for (const msg of emojiIconIssues(text, r)) errors.push(msg);
  for (const msg of strokeWidthIssues(text, r, lucide)) errors.push(msg);
  for (const msg of bareIconTagIssues(templateOf(text), text, r, registry)) errors.push(msg);
}

// 数据文件里的「图标字段」同样在射程内：docTemplates 那 16 条 📋📝🚀 就是从这里混进 UI 的。
// .ts 只判 icon: '<emoji>' 这一种形态，别把 markdown 的 :tada: → 🎉 shortcode 表当违例（那是渲染内容的词表）。
for (const f of walk(SRC, /\.ts$/)) {
  const r = rel(f);
  if (r === 'utils/icons.ts' || /\.test\.ts$/.test(r)) continue;
  const text = stripComments(read(f));
  for (const msg of emojiIconIssues(text, r)) errors.push(msg);
  for (const msg of iconSizeRegistryIssues('', text, r, new Set(), iconSizeTiers)) {
    offRegistryCount++;
    offRegistrySites.push(msg);
  }
}

/** 按钮块内图标尺寸判据（单独成函数是为了能被 --selftest 直接喂夹具，不必改真源码） */
function buttonIconSizeIssues(tpl, rel) {
  const out = [];
  for (const block of tpl.matchAll(/<(ApButton|button)\b[\s\S]*?<\/\1>/g)) {
    for (const attr of block[0].matchAll(/<(?:[A-Z][A-Za-z0-9]*|ApIcon|component|svg|i)\b[^>]*?(?::|\b)size="(\d+)"/g)) {
      if (attr[1] !== String(BUTTON_ICON_SIZE)) {
        out.push(`I5 ${rel} 按钮里的图标是 ${attr[1]}px（统一 ${BUTTON_ICON_SIZE}px；确需例外要写清为什么）`);
      }
    }
  }
  return out;
}

/**
 * I6：图标尺寸必须落 AppleIconSize 的档。
 * 只看图标元素（lucide 直接 import 的名字 / ApIcon / component / ApMorphIcon），
 * 别的组件的 size= 是它自己的控件档，不该被这条误伤。
 * 返回的是"注册表外的写死数"清单 —— 数量进基线、只准降不准升，
 * 所以加一处新漂移就红，而存量 12/14 不必在这一批里全烧完。
 */
function iconSizeRegistryIssues(tpl, full, rel, lucide, tiers) {
  const out = [];
  for (const m of tpl.matchAll(/<([A-Z][A-Za-z0-9]*)\b([^>]*?)\/?>/g)) {
    if (!isIconTag(m[1], lucide)) continue;
    const size = SIZE_ATTR.exec(m[2])?.[1];
    if (size == null) continue;
    for (const n of size.matchAll(/\d+/g)) {
      if (!tiers.has(Number(n[0]))) {
        out.push(`${rel}:${lineOf(tpl, m.index)} 图标 :size="${size}" 里的 ${n[0]} 不在 AppleIconSize 档（${[...tiers].sort((a, b) => a - b).join('/')}）`);
      }
    }
  }
  // 数据对象里的图标档（ApStatCard 的 sizeConfig 那一族）同样是尺寸，别让它绕过尺子
  for (const m of full.matchAll(/\biconSize:\s*(\d+)/g)) {
    if (!tiers.has(Number(m[1]))) out.push(`${rel}:${lineOf(full, m.index)} iconSize: ${m[1]} 不在 AppleIconSize 档`);
  }
  return out;
}

/**
 * I7：图标位不许画 emoji。
 * .vue 的模板里出现 emoji 就算（模板是 UI，不是数据）；.ts 只认 `icon: '🚀'` 这种
 * "把 emoji 存成图标字段"的形态 —— markdown 的 :tada: shortcode 表是渲染内容，不在射程内。
 */
function emojiIconIssues(text, rel) {
  const out = [];
  if (EMOJI_DATA_ALLOW.has(rel)) return out;
  if (rel.endsWith('.vue')) {
    // 只判模板：脚本里的 emoji 大多是**内容**（聊天进度句「> 🤖 正在编排…」那一族），
    // 不是 UI 图标位。tplOf 那种「切到 <script 为止」在 <script setup> 写在文件头部的
    // 页面里等于没切，所以这里取 <template>…</template> 本体。
    const tpl = /<template>[\s\S]*<\/template>/.exec(text)?.[0] ?? '';
    for (const m of tpl.matchAll(new RegExp(EMOJI_RE.source, 'gu'))) {
      out.push(`${rel}:${lineOf(tpl, m.index)} 图标位画了 emoji「${m[0]}」（emoji 没有尺寸档与描边，等于第三套图标族；改用 ApIcon + 注册表名）`);
    }
    return out;
  }
  for (const m of text.matchAll(/\bicon:\s*(['"`])([\s\S]*?)\1/g)) {
    if (!EMOJI_RE.test(m[2])) continue;
    out.push(`${rel}:${lineOf(text, m.index)} 图标字段存了 emoji「${m[2]}」（改用 ApIcon + 注册表名）`);
  }
  return out;
}

/**
 * I8：≥24px 的图标必须解出描边宽度。
 * 尺寸越大、lucide 默认 stroke-width: 2 画出来的线越粗（viewBox 单位随尺寸等比放大），
 * 40px 不接管就是 3.3px 的粗线。ApIcon 自己按 iconStrokeFor 反解，所以走 ApIcon 即算合规；
 * 直接画 lucide 组件的必须显式传 stroke-width，否则红。
 */
function strokeWidthIssues(text, rel, lucide) {
  const out = [];
  for (const m of text.matchAll(/<([A-Z][A-Za-z0-9]*)\b([^>]*?)\/?>/g)) {
    if (!isIconTag(m[1], lucide)) continue;
    const attrs = m[2];
    const size = SIZE_ATTR.exec(attrs)?.[1];
    if (size == null) continue;
    const max = Math.max(...[...size.matchAll(/\d+/g)].map((n) => Number(n[0])), 0);
    if (max < 24) continue;
    if (m[1] === 'ApIcon') continue;
    if (/(?:stroke-width|strokeWidth)=/.test(attrs)) continue;
    out.push(`${rel}:${lineOf(text, m.index)} ${m[1]} 画到 ${max}px 却没解描边（走 ApIcon，或显式 :stroke-width="iconStrokeFor(${max})"）`);
  }
  return out;
}

/**
 * I9：模板里写 `<Send />` 这种裸图标标签，既没 import 也不是全局 Ap* 组件时，
 * Vue 把它当未知元素渲染（DOM 里是 <send>，里面没有 svg）——按钮就是一个空白方块，
 * 而尺寸档、描边、ApIcon name 那几条判据全都够不着它。
 * 只认 icons.ts 注册表里的名字：普通业务组件不在表里，不受这条约束。
 */
function bareIconTagIssues(tpl, full, rel, registrySet) {
  const imported = new Set();
  for (const im of full.matchAll(/import\s+([\s\S]*?)\s+from\s+['"][^'"]+['"]/g)) {
    for (const part of im[1].split(',')) {
      const clause = part.trim().replace(/^type\s+/, '');
      const name = (clause.split(/\s+as\s+/).pop() || '').replace(/[{}*]/g, '').trim();
      if (/^[A-Za-z_$][\w$]*$/.test(name)) imported.add(name);
    }
  }
  const out = [];
  for (const m of tpl.matchAll(/<([A-Z][A-Za-z0-9]*)\b/g)) {
    const tag = m[1];
    if (!registrySet.has(tag)) continue;
    if (/^Ap[A-Z]/.test(tag)) continue; // main.ts 全局只注册 Ap*
    if (imported.has(tag)) continue;
    out.push(`${rel}:${lineOf(tpl, m.index)} 裸图标标签 <${tag}> 未 import 也未全局注册 → 渲染空白（改用 <ApIcon name="${tag}">）`);
  }
  return out;
}

if (glyphCount > BASELINE.glyphAsIcon) {
  errors.push(`I2 glyph-as-icon 从基线 ${BASELINE.glyphAsIcon} 升到 ${glyphCount}（×→←↑↓ 当图标用，改用 lucide）`);
}
if (mixedCount > BASELINE.mixedPathFiles) {
  errors.push(`I4 混用图标路径的文件从基线 ${BASELINE.mixedPathFiles} 升到 ${mixedCount}：${mixedFiles.join(', ')}`);
}
if (offRegistryCount > BASELINE.offRegistrySizes) {
  errors.push(
    `I6 注册表外的写死图标尺寸从基线 ${BASELINE.offRegistrySizes} 升到 ${offRegistryCount}：\n  ` +
    offRegistrySites.slice(0, 20).join('\n  '),
  );
}

if (errors.length) {
  console.error(`\n✗ 图标一致性检查未通过（${errors.length} 项）：`);
  for (const e of errors.slice(0, 40)) console.error('  ' + e);
  if (errors.length > 40) console.error(`  … 另有 ${errors.length - 40} 项`);
  process.exit(1);
}
console.log(
  `ℹ 图标一致性：注册表 ${registry.size} 键；尺寸档 ${[...iconSizeTiers].sort((a, b) => a - b).join('/')}；` +
  `ApIcon 静态 name 全部可解析；无手绘 <svg>；无图标位 emoji；≥24px 描边全部有解；裸图标标签均已注册；` +
  `glyph-as-icon ${glyphCount}/${BASELINE.glyphAsIcon}、混路径文件 ${mixedCount}/${BASELINE.mixedPathFiles}、` +
  `注册表外尺寸 ${offRegistryCount}/${BASELINE.offRegistrySizes}（基线只降不升）`,
);
