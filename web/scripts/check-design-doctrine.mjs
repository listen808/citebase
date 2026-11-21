#!/usr/bin/env node
/**
 * 设计教条守卫 —— 用法：npm run check:design（已串进 npm run check:style）
 *
 * 目的：把「去掉通用 AI 观感」这件事从一次性的手工整改，变成不可回退的断言。
 * 每条规则都对应一次真实的整改，写死数字而不是靠 review 记仇。
 *
 *  G1 装饰机制不得复活：ApGradientCard / format/gradient / Sparkles / 交错入场
 *  G2 营销级 display 字号只准出现在白名单（错误页 / 登录页），产品页一律走层级令牌
 *  G3 UI chrome 禁用 emoji 作图标（白名单=内容型文件：文档模板 / emoji 图标 / 模拟回答文本）
 *  G4 表面必须用阶梯角色（surface-panel / surface-elevated）；pure-white-canvas 在暗色下是黑色，不得当背景
 *  G5 渐变必须由主题令牌拼接（gradient(...) 里出现 #hex 即违规）
 *  G6 组件/视图不得写死 ≥16px 圆角或裸 px 字号（圆角是主题语言，不是页面私有审美）
 *  G7 英文全大写 kicker 标签禁用（首字母缩写的头像除外）
 *  G8/G9 文字色必须与它实际落到的背景搭配（white-alpha 当文字、白字压浅染都等于隐形）
 *  G10 表面必须用阶梯角色（surface-chrome/canvas/panel/elevated/field），不许再用 bg-primary 等降级别名
 *  G11 AppleType 不得整条展开进 :style（size/weight/font 不是 CSS 属性名，会静默失效）
 *  G12 控件高度只准一条阶梯（同档的 button / input / segment 必须同高，并排才不错基线）
 *  G13 按钮体里只有一个图标时必须声明 icon-only（否则按文本按钮 padding 渲染，大一圈）
 *  G14 收藏开关只有一处实现（ApFavoriteButton）；不许手搓星标或斜线星表达收藏
 *  G32 动效时长只准走 var(--apple-dur-*)（唯一事实源是 apple-tokens.ts 的 BASE_DURATION_MS；
 *      写死秒数的话个性化的「动效强度」那一档根本调不到它，而肉眼与单测都看不出）
 *  G33 实心强调色上的前景必须用 on-blue（= contrastInkOn(blue)），不许用恒白的 on-accent ——
 *      强调色改成用户可选之后，恒白压不住浅色档；本仓八处存量已在批3 迁完，零容忍不留棘轮
 */
import fs from 'fs';
import path from 'path';
import { eachJsObject, stripCommentsKeepLines, lineAt } from './lib/js-objects.mjs';

const SRC = path.resolve('src');
const walk = (d, out = []) => {
  for (const e of fs.readdirSync(d, { withFileTypes: true })) {
    const p = path.join(d, e.name);
    if (e.isDirectory()) walk(p, out);
    else if (/\.(vue|ts)$/.test(e.name) && !p.endsWith('.d.ts')) out.push(p);
  }
  return out;
};
const rel = (f) => path.relative(SRC, f).replace(/\\/g, '/');
const read = (f) => fs.readFileSync(f, 'utf8').replace(/\r\n/g, '\n');
const strip = (t) =>
  t
    .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
    .replace(/(^|[^:])\/\/[^\n]*/g, (m, p1) => p1 + ' '.repeat(m.length - p1.length))
    .replace(/<!--[\s\S]*?-->/g, (m) => m.replace(/[^\n]/g, ' '));

/** 营销级字号（80/56/48/40/38/32/28px display 家族） */
const DISPLAY_TOKENS = /AppleType\.(heroDisplayXL|heroDisplayL|sectionDisplay|productHeading|featureDisplay|promoDisplay|cardProductTitle)\b/;
const DISPLAY_ALLOW = new Set([
  'components/apple/ApErrorPage.vue', // 错误页允许大字号
  'components/auth/LoginForm.vue',
  'views/auth/InstallView.vue',
  'utils/core/apple-tokens.ts',
  'styles/theme/themes.ts',
]);
/** 内容型文件：文档本身可以带 emoji（模板、文档图标、模拟回答文本） */
const EMOJI_ALLOW = new Set([
  'config/docTemplates.ts',
  'composables/editor/useMarkdownRenderer.ts',
  'components/space/sidebar/DocProperties.vue',
  'views/ai/AIChatView.vue',
]);
const EMOJI_RE = /[\u{1F300}-\u{1FAFF}\u{2600}-\u{27BF}]|\u{FE0F}/u;
/** 时长事实源所在文件：BASE_DURATION_MS / durationVariables / 减弱档那组 0.01ms 通配都在这一处 */
const DURATION_ALLOW = new Set(['utils/core/apple-tokens.ts']);
const UPPERCASE_ALLOW = new Set(['components/apple/ApAvatar.vue']);

const errors = [];
const files = walk(SRC);

for (const f of files) {
  const r = rel(f);
  const t = strip(read(f));
  t.split('\n').forEach((line, i) => {
    const at = `${r}:${i + 1}`;

    // G1 装饰机制复活
    if (/ApGradientCard|format\/gradient|useStaggerEnter|v-stagger|\bSparkles\b|WandSparkles/.test(line))
      errors.push(`G1 ${at} 复活了已删除的装饰机制（渐变卡 / 随机渐变 / 交错入场 / Sparkles）`);

    // G2 营销字号
    if (DISPLAY_TOKENS.test(line) && !DISPLAY_ALLOW.has(r))
      errors.push(`G2 ${at} 产品页使用营销级 display 字号，请改用 pageTitle / sectionTitle / statValue`);

    // G32 动效时长必须走 --apple-dur-* 令牌（个性化批2）。
    // 理由是功能性的，不是审美：时长写死在声明处，「动效强度」那一档就调不到它 ——
    // 而这类漏接线在静态侧看不出来（页面照样动、门禁照样绿），只有把倍数改掉之后
    // 拿"两处该不一样"去量才发现。合法落点只有 BASE_DURATION_MS 那一张表。
    if (!DURATION_ALLOW.has(r) && /\b(transition|animation)\b/.test(line) && /[\w'"`-]\s+[\d.]+m?s\b/.test(line))
      errors.push(`G32 ${at} 动效时长写成了裸字面量，请改用 var(--apple-dur-*)（时长档见 apple-tokens.ts 的 BASE_DURATION_MS）`);

    // G3 chrome emoji
    if (EMOJI_RE.test(line) && !EMOJI_ALLOW.has(r) && !/.test.ts$/.test(r))
      errors.push(`G3 ${at} UI chrome 里用 emoji 当图标，请改用 lucide 图标组件`);

    // G4 画布令牌当背景
    if (/background(?:Color)?\s*[:=]\s*["']?\s*cv\('pure-white-canvas'\)/.test(line))
      errors.push(`G4 ${at} 表面请用阶梯角色 cv('surface-panel'|'surface-elevated')；pure-white-canvas 在暗色下是黑色`);

    // G5 未主题化的渐变
    if (/(linear|radial|conic)-gradient\([^)]*#[0-9a-fA-F]{3,8}/.test(line))
      errors.push(`G5 ${at} 渐变里写死 hex，主题无法跟随；请用 cv() 拼接或收进主题令牌`);

    // G6 圆角与裸 px 字号
    const radiusLit = line.match(/borderR(?:adius|ounding)\s*[:=]\s*['"](\d+(?:\.\d+)?)px['"]/);
    if (radiusLit && Number(radiusLit[1]) >= 16 && r !== 'utils/core/apple-tokens.ts')
      errors.push(`G6 ${at} 写死 ${radiusLit[1]}px 圆角，请走 AppleRadius / 主题圆角令牌`);
    if (/fontSize\s*[:=]\s*['"]\d+(?:\.\d+)?px['"]/.test(line) && r !== 'utils/core/apple-tokens.ts')
      errors.push(`G6 ${at} 写死字号 px，请走 AppleType / AppleFontSize`);

    // G7 全大写 kicker
    if (/textTransform:\s*['"]uppercase['"]/.test(line) && !UPPERCASE_ALLOW.has(r))
      errors.push(`G7 ${at} 全大写 kicker 标签是通用 AI 版面套路，请改用字重与字号做层级`);
  });
}

/* G1 补充：这些文件必须不存在（防止半路重新加回来） */
for (const gone of ['components/apple/ApGradientCard.vue', 'utils/format/gradient.ts']) {
  if (fs.existsSync(path.join(SRC, gone))) errors.push(`G1 ${gone} 已废除，不应重新出现`);
}

/* ---------- G8/G9：文字色与它落到的背景必须搭配（截图里「卡片元信息隐形」就是这类） ---------- */
const TINT_BG = /background(?:Color)?\s*[:=][^;\n]*(?:-bg-subtle|-light\x27\)|cv\(\x27bg-(secondary|tertiary|card)\x27\)|surface-panel)/;
const SOLID_BG = /background(?:Color)?\s*[:=][^;\n]*(?:cv\(\x27(blue|green|red|orange|yellow|purple|teal|indigo|pink|gray|success|warning|error)\x27\)|onSolid\(|gradient|bg\x27?\))/;
for (const f of walk('src')) {
  const r = rel(f);
  const lines = strip(read(f)).split('\n');
  lines.forEach((line, i) => {
    const at = `${r}:${i + 1}`;
    // G8：white-alpha-* 是给「深色遮罩上的描边/阴影」用的，当文字色会在浅色主题下隐形
    if (/color\s*[:=]\s*["']?\s*cv\('white-alpha-/.test(line))
      errors.push(`G8 ${at} 文字用 white-alpha-* 在浅色主题下等于隐形；遮罩上的文字用 cv('on-accent')，面板上用 text-secondary/tertiary`);
    // G9：浅染背景（*-bg-subtle / *-light / 面板底）上不能用 on-accent，要用 {hue}-text
    if (/color\s*[:=]\s*["']?\s*cv\('on-accent'\)/.test(line)) {
      const win = lines.slice(Math.max(0, i - 10), i + 4).join('\n');
      if (TINT_BG.test(win) && !SOLID_BG.test(win))
        errors.push(`G9 ${at} on-accent（白字）压在浅染/面板底上会隐形，改用 cv('{hue}-text') 或 text-secondary`);
    }
  });
}

/* ---------- G10：表面必须用阶梯角色，不许再用降级别名 ---------- */
// 阶梯别名（bg-primary/bg-card/bg-elevated/glass-bg/glass-bg-nav）已从 :root 删除，
// 这条门禁因此从"劝退"升级成"必然失败"：写 cv('bg-card') 会被 AppleCssVarName 类型拦下，
// 而 <style> 块里的 var(--apple-bg-card) 编译期看不见，只能靠这里拦。
const SURFACE_ALIAS =
  /(?:cv\('((?:bg|glass-bg)(?:-(?:primary|card|elevated|nav))?)'\)|var\(--apple-(bg-(?:primary|card|elevated)|glass-bg(?:-nav)?)\)|resolvedColor\('bg-(?:primary|card|elevated)'\))/;
for (const f of walk('src')) {
  const r = rel(f);
  if (r === 'utils/core/apple-tokens.ts' || r.startsWith('styles/theme/')) continue;
  strip(read(f))
    .split('\n')
    .forEach((line, i) => {
      if (!SURFACE_ALIAS.test(line)) return;
      errors.push(
        `G10 ${r}:${i + 1} 用了已删除的表面别名 ${line.trim().slice(0, 40)}；按意图改取 cv('surface-canvas'|'surface-panel'|'surface-elevated'|'surface-chrome'|'surface-field')`,
      );
    });
}

// G11：AppleType.* 的字段名是设计稿口径（size / weight / font），不是 CSS 属性名。
// `...AppleType.microUI` 展开进 :style 时，size 与 weight 被浏览器丢弃、font 被当作
// font shorthand 整条失效 —— 结果是「以为设了 12px 小标签，实际继承 17px 正文」。
for (const f of files) {
  const r = rel(f);
  // 令牌层自己的注释里会举这个反例，不参与检查
  if (r === 'utils/core/apple-tokens.ts') continue;
  const lines = read(f).split('\n');
  lines.forEach((line, i) => {
    if (!/\.\.\.\s*AppleType\.[A-Za-z]+/.test(line)) return;
    errors.push(`G11 ${r}:${i + 1} 直接展开 AppleType，字号与字重会静默失效；改用 ...cssType(AppleType.x)`);
  });
}

// G12：控件高度只允许一条阶梯。历史上 AppleButtonSize.medium=40 而 AppleInputSize.medium=38，
// 分段控件又自带 padding 算出的 27/35 —— 同一行里三种高度，工具栏基线必然对不上。
{
  const src = read(path.join(SRC, 'utils/core/apple-tokens.ts'));
  const ladder = (name) => {
    const blk = new RegExp(`export const ${name} = \\{([\\s\\S]*?)\\n\\} as const`).exec(src);
    if (!blk) return null;
    const out = {};
    for (const m of blk[1].matchAll(/(\w+):\s*\{([^}]*)\}/g)) {
      const h = /height: '([^']+)'/.exec(m[2]);
      if (h) out[m[1]] = h[1];
    }
    return out;
  };
  const ladders = {
    AppleButtonSize: ladder('AppleButtonSize'),
    AppleInputSize: ladder('AppleInputSize'),
    AppleSegmentSize: ladder('AppleSegmentSize'),
  };
  for (const [k, v] of Object.entries(ladders)) {
    if (!v) errors.push(`G12 apple-tokens.ts 里找不到 ${k} 的 height 阶梯，控件高度失去唯一标尺`);
  }
  if (ladders.AppleButtonSize && ladders.AppleInputSize && ladders.AppleSegmentSize) {
    for (const size of ['small', 'medium', 'large']) {
      const vals = [ladders.AppleButtonSize[size], ladders.AppleInputSize[size], ladders.AppleSegmentSize[size]];
      if (vals.some((v) => !v)) continue;
      if (new Set(vals).size > 1) {
        errors.push(`G12 ${size} 档三套控件高度不一致：button ${vals[0]} / input ${vals[1]} / segment ${vals[2]}；并排必错基线`);
      }
    }
  }
}

// G13：按钮体里只有一个图标时，必须声明 icon-only。
// 漏声明时它按文本按钮的 padding 渲染，同一行里比邻居高大一圈（权限页实测删除键成了大红块）。
for (const f of files) {
  const r = rel(f);
  const text = read(f);
  const line = (idx) => text.slice(0, idx).split('\n').length;
  for (const m of text.matchAll(/<ApButton([^>]*?)>(\s*<[A-Z][A-Za-z0-9]*\s+:size="?\d+"?[^>]*\/>\s*)<\/ApButton>/g)) {
    if (/\bicon-only\b/.test(m[1])) continue;
    errors.push(`G13 ${r}:${line(m.index)} 按钮体里只有图标却没声明 icon-only，会按文本按钮的 padding 渲染`);
  }
}

// G14：收藏开关只有一处实现（ApFavoriteButton）。
// 全站曾有六处各画一颗星：fill 有的有有的没有、颜色在 orange / warning / text-secondary /
// text-quaternary 之间各挑一个，还有红色 StarOff 与 variant="danger" 的「取消」文字按钮——
// 同一次收藏在不同页面长得完全不一样，可访问名也不随状态变。
// 斜线星 StarOff 一律不许再出现；<Star 与"收藏"同时出现在一个文件里，就是要手搓开关。
const STAR_USAGE_ALLOW = new Map([
  ['components/home/FavoriteDocs.vue', '「收藏文档」空态的装饰星形图标，不可点击，不是开关'],
  ['views/ai/settings/AISQLHistoryView.vue', '详情里的评分星级（实心/描边表达 1–5 分），与收藏无关'],
]);
for (const f of walk('src')) {
  const r = rel(f);
  if (r === 'components/apple/ApFavoriteButton.vue' || STAR_USAGE_ALLOW.has(r)) continue;
  const text = strip(read(f));
  if (/<StarOff\b/.test(text)) {
    errors.push(`G14 ${r} 渲染了 StarOff（斜线星）；收藏状态请用 <ApFavoriteButton :active> 表达，别换图标`);
  } else if (/<Star\b/.test(text) && /收藏/.test(text)) {
    errors.push(`G14 ${r} 自己画星标又自己写"收藏"文案；请改用 <ApFavoriteButton>（其它星形用途进 STAR_USAGE_ALLOW 写理由）`);
  }
}

// G15：字号天阶里的字距必须由 trackingFor() 派生，不许逐条目手写。
// 原先 22 处手写值的实测结果是两头反了：17px 正文 -0.374px 是全站最紧，
// 21px 页头反而 +0.231px 最松，20px 与 21px 两个相邻档符号相反。跨页看每个尺寸都是
// 单值（运行时排版巡检因此一直绿），但"一致"指的是**随字号单调**，不是"每处都抄一个数"。
// 这条门禁保证以后不会有人再往条目里回填字面量。
{
  const f = 'src/utils/core/apple-tokens.ts';
  const text = read(f);
  const start = text.indexOf('export const AppleType = {');
  const end = text.indexOf('\n} as const;', start);
  const block = start >= 0 && end > start ? text.slice(start, end) : '';
  const bad = (block.match(/letterSpacing: '(?!normal)/g) || []).length;
  if (bad) errors.push(`G15 ${rel(f)} AppleType 里有 ${bad} 处手写字距；请改用 trackingFor(<字号px>)`);
  if (block && !/letterSpacing: trackingFor\(/.test(block)) {
    errors.push(`G15 ${rel(f)} AppleType 没有一处走 trackingFor()——派生入口被绕过了`);
  }
}

// G16：静态属性里不许出现表达式。`aria-label="foo ? 'a' : 'b'"` 少了绑定冒号，
// 编译后就是字面量 —— 读屏会把源码三元表达式整串念出来。这类写法 P3 抓不到
// （它只判"有没有可访问名字"，而这四个按钮名字"在"，只是内容是代码）。
const EXPR_IN_STATIC_ATTR = /(?<![:\w-])(aria-label|title|alt|placeholder|tooltip)="[^"]*\?\s*'[^']*'\s*:/g;
for (const f of walk('src')) {
  const r = rel(f);
  if (!r.endsWith('.vue')) continue;
  const text = read(f);
  for (const m of text.matchAll(EXPR_IN_STATIC_ATTR)) {
    errors.push(`G16 ${r}:${text.slice(0, m.index).split('\n').length} 静态 ${m[1]} 里写了三元表达式（少了绑定冒号）——读屏会念出源码`);
  }
}

// G17：过渡只准点名「不触发布局」的属性（§11 合成器专属）。
// 运行时那台（check:apple A1/A1b）只能量"当场渲染出来的元素"，所以源码里
// 那些要特定状态才出现的 `transition: all` / `transition: width` 它永远看不见 ——
// 这条把同一条规则搬到静态侧。
const TRANSITION_LAYOUT_PROPS = ['all', 'width', 'height', 'max-width', 'max-height', 'min-width', 'min-height',
  'top', 'left', 'right', 'bottom', 'inset', 'margin', 'padding', 'gap', 'row-gap', 'column-gap',
  'line-height', 'font-size', 'border-width', 'grid-template-columns'];
// 简写省略 property 等价于 all，单独判
const TIME_RE = /^\d+(?:\.\d+)?m?s$/;
const CURVE_RE = /^(cubic-bezier|linear|ease|ease-in|ease-out|ease-in-out|steps)/;
const TRANSITION_DECL = /transition(?:Property)?\s*[:=]\s*(['"`])([\s\S]*?)\1/g;
// 天生绑布局的例外：进表必须写理由（与运行时 A1e 同一条口径）
const LAYOUT_TRANSITION_ALLOW = new Map([
  ['components/space/reader/DocContentPanel.vue',
    '正文列在「阅读宽度 ↔ 宽版」之间切换：文字列不能 scaleX（字会被拉扁），只能动 max-width'],
]);
const splitLayers = (s) => {
  const out = []; let depth = 0; let cur = '';
  for (const ch of s) {
    if (ch === '(') depth++;
    if (ch === ')') depth--;
    if (ch === ',' && depth === 0) { out.push(cur); cur = ''; } else cur += ch;
  }
  if (cur.trim()) out.push(cur);
  return out;
};
for (const f of files) {
  const r = rel(f);
  if (!/\.(vue|ts)$/.test(r) || r.startsWith('utils/core/apple-tokens.ts')) continue;
  const text = read(f);
  for (const m of text.matchAll(TRANSITION_DECL)) {
    const val = m[2].trim();
    // 令牌与 CSS 变量提供的属性表由令牌文件自己负责（qualify() 单一来源）
    if (/AppleTransition\.|AppleTiming\.|var\(--apple-/.test(val) && !TRANSITION_LAYOUT_PROPS.some((p) => new RegExp(`(^|[\\s,])${p}(?![\\w-])`).test(val))) continue;
    const bad = [];
    for (const layer of splitLayers(val)) {
      const tokens = layer.replace(/\$\{[^}]*\}/g, 'X').trim().split(/\s+/).filter(Boolean);
      if (!tokens.length) continue;
      const first = tokens[0];
      if (TRANSITION_LAYOUT_PROPS.includes(first)) bad.push(first);
      else if (TIME_RE.test(first) || CURVE_RE.test(first)) bad.push('all(简写省略 property)');
    }
    if (!bad.length) continue;
    const why = LAYOUT_TRANSITION_ALLOW.get(r);
    if (why && bad.every((p) => /max-width|width/.test(p))) continue;
    errors.push(`G17 ${r}:${text.slice(0, m.index).split('\n').length} transition 动了布局属性 ${[...new Set(bad)].join('+')}（值 ${val.slice(0, 46)}）——只准 transform/opacity/颜色族，要动尺寸请用 transform${why ? `（该文件例外理由只覆盖 max-width，这条不在例外内）` : ''}`);
  }
}

// G18：裸 `<input type="file">` 必须有可访问名字。
// 文件选择框的"选择文件"是浏览器写死的，读屏念不出这是选什么；
// check:dialog 的 D3 只覆盖弹层里当场渲染的控件，这条补静态侧。
for (const f of files) {
  const r = rel(f);
  if (!r.endsWith('.vue')) continue;
  const text = read(f);
  for (const m of text.matchAll(/<input\b[^>]*>/g)) {
    const tag = m[0];
    if (!/type=["']file["']/.test(tag)) continue;
    if (/aria-label=|:aria-label=/.test(tag)) continue;
    const idm = tag.match(/\s(?::?id)="([^"]+)"/);
    if (idm && new RegExp(`for=["']${idm[1]}["']|:for="[^"]*${idm[1]}`).test(text)) continue;
    // 配对也可能由**子组件**渲染：`<ApFormItem id="x">` 会把 id 下发给字段并把
    // `label for="x"` 画在 ApFieldWrapper 里（AIImportView 那处就是这么做的，实测成立）。
    // 只在本文件找字面 for= 会把这种合法配对报成缺陷，而补 aria-label 反而会让读屏
    // 忽略可见 label 文本 —— 那是无障碍回退，不是修复。
    if (idm && new RegExp(`<(ApFormItem|ApFieldWrapper)\\b[\\s\\S]{0,400}?id="${idm[1]}"`).test(text)) continue;
    errors.push(`G18 ${r}:${text.slice(0, m.index).split('\n').length} 裸 <input type="file"> 没有 aria-label 也没有 label[for] —— 读屏只念得出"选择文件"`);
  }
}

// G19：注入的关键帧与 .apple-* 工具类必须有消费者。
// 定义即死码是这套体系特有的失效方式：工具类是"第二套并行样式系统"，
// 没人用还留着，下一个人就会以为它是推荐写法。
const tokensFile = read(path.join(SRC, 'utils/core/apple-tokens.ts'));
const allText = files.map((f) => read(f)).join('\n');
const ANIM_KEY_MAP = Object.fromEntries([...tokensFile.matchAll(/(\w+):\s*'((?:apple-|spin)[\w-]*)'/g)]
  .filter((m) => /AppleAnimation/.test(tokensFile.slice(0, m.index).split('export const AppleAnimation').pop() || '')).map((m) => [m[2], m[1]]));
// 名字边界必须把连字符算进标识符：`\b` 认为 `-` 是词边界，于是 `apple-pulse-soft`
// 会给 `pulse` 记两次"有人用"（实测：现判据下 pulse 命中 2、soft 命中 20，全部假绿）。
const nameIsUsed = (body, name) => new RegExp(`(?<![-\\w])${name}(?![-\\w])`).test(body);
const KDEF_STRIP = /@keyframes\s+[\w-]+/g;
// 全局关键帧只有一个注入源（apple-tokens 的 #apple-keyframes 块）—— 原先还有第二块，已并入。
for (const src of [['utils/core/apple-tokens.ts', tokensFile]]) {
  const body = allText.replace(KDEF_STRIP, '');
  for (const m of src[1].matchAll(/@keyframes\s+([\w-]+)/g)) {
    const name = m[1];
    const viaToken = !!(ANIM_KEY_MAP[name] && allText.includes(`AppleAnimation.${ANIM_KEY_MAP[name]}`));
    if (!nameIsUsed(body, name) && !viaToken) {
      errors.push(`G19 ${src[0]} 关键帧 ${name} 零消费者（全站只出现在定义处）—— 删掉，别留第二套并行体系`);
    }
  }
}
for (const m of tokensFile.matchAll(/^(\.apple-[\w-]+)\s*\{/gm)) {
  const cls = m[1].slice(1);
  // 必须落在 class 上下文里才算消费者：`includes('apple-glass')` 会被 `.apple-glass-nav`
  // 和 Monaco 主题 id `glass:'apple-glass'` 双双冒充成"有人用"（实测假绿 5 个文件）。
  const BOUND = '(?<![-\\w])' + cls + '(?![-\\w])';
  // 消费者必须**用到这个名字**，但允许三种真实写法：class 字面量、`:class="'x'"` 表达式、
  // `classes.push('x')` 数组拼装、以及 `:class="{ 'x': cond }"` 对象键。
  // 反过来要排除"对象属性值"：`glass:'apple-glass'` 是 Monaco 的主题 id，不是 class
  // —— 老判据 `text.includes(cls)` 正是被它和 `.apple-glass-nav` 的子串关系骗过（假绿 5 个文件）。
  const OBJ_VALUE = new RegExp('[\\w$]\\s*:\\s*["\'`]' + cls + '["\'`]', 'g');
  const OCC = new RegExp(BOUND);
  const used = files.some((f) => {
    if (rel(f).startsWith('utils/core/apple-tokens.ts')) return false;
    return OCC.test(read(f).replace(OBJ_VALUE, ''));
  });
  if (!used) errors.push(`G19 apple-tokens.ts 工具类 .${cls} 零消费者 —— 要么接线要么删`);
}

// G20：图标尺寸只能落档。13/11/15 这类"介于两档之间"的奇数值是肉眼最难发现、
// 又最典型的漂移（同一屏两个 1 像素之差的图标并排就是"看着散"）。
// 48 是展示档（图片占位符、登录页大图标），与 40 的 ApEmpty 状态档各自有明确用途，不是离群值。
const ICON_SIZES = [12, 14, 16, 18, 20, 22, 24, 28, 32, 40, 48];
for (const f of files) {
  const r = rel(f);
  if (!r.endsWith('.vue')) continue;
  const text = read(f);
  if (!/lucide-vue-next/.test(text)) continue;
  for (const m of text.matchAll(/(?<![\w:-]):?size="(\d+)"/g)) {
    const n = Number(m[1]);
    if (!ICON_SIZES.includes(n)) {
      errors.push(`G20 ${r}:${text.slice(0, m.index).split('\n').length} 图标 size=${n} 不在档位 ${ICON_SIZES.join('/')} —— 就近升档或降档，别开新标尺`);
    }
  }
}

// G22：进度条只准有一个实现 —— components/apple/ApProgress.vue。
// 这条拦的是"自造轨道"的三个签名，每个都曾经是全站并存的第 N 套进度条：
// role=progressbar 是这类控件的可访问身份；`transformOrigin: 'left center'` 是横条 scaleX 填充的签名；
// 模板字面量的百分比宽度是"动 width 的填充"的签名（G17 拦的是它的过渡，这里拦的是它的存在）。
const PROGRESS_OWNER = 'components/apple/ApProgress.vue';
const PROGRESS_SIGNS = [
  ['role="progressbar"', /role="progressbar"/],
  ["transformOrigin: 'left center'", /transformOrigin:\s*'left center'/],
  ['width: `${…}%` 手搓填充', /width:\s*`[^`]*\$\{[^`]*%[^`]*`/],
];
for (const f of files) {
  const r = rel(f);
  if (!r.endsWith('.vue') || r === PROGRESS_OWNER) continue;
  const text = read(f);
  for (const [name, re] of PROGRESS_SIGNS) {
    const m = re.exec(text);
    if (m) errors.push(`G22 ${r}:${text.slice(0, m.index).split('\n').length} 出现 ${name} —— 进度条只认 <ApProgress>，别再手搓轨道`);
  }
}

// G23：浮层菜单的表面只准由 menuSurfaceStyle() 发一份。
// 判据取"下拉阴影"这个签名：一份菜单要偏离标准，得同时改底色/圆角/边/阴影/z 五件，
// 而阴影是它一定留着的那件 —— 手写一份就必然带上它，抓这一条足够拦住"再开一套"。
const SHADOW_DROPDOWN_OWNER = 'utils/core/apple-tokens.ts';
// 不是菜单、但确实要那团投影的地方（灯箱里的预览图：悬在遮罩上的一块内容，没有菜单的其它属性）
const SHADOW_DROPDOWN_ALLOW = ['styles/layout/page-styles.ts'];
for (const f of files) {
  const r = rel(f);
  if (r === SHADOW_DROPDOWN_OWNER || SHADOW_DROPDOWN_ALLOW.includes(r)) continue;
  const text = read(f);
  const m = /boxShadow:\s*cv\('shadow-dropdown'\)/.exec(text);
  if (m) errors.push(`G23 ${r}:${text.slice(0, m.index).split('\n').length} 手写了浮层阴影 —— 菜单表面请改用 menuSurfaceStyle()（要偏离标准就用 extra 覆盖并写理由）`);
}

/**
 * G23b：G23 的**签名太窄**，2026-10-15 被这件事实证了两次。
 * 原判据只认 `boxShadow: cv('shadow-dropdown')` 一个字面量，于是
 * `ChatMessageBubble` 的引文预览用 `shadow-level3` + `surface-elevated` + `border-light`
 * 自己拼了一张浮层表面，G23 一声不响；`DocumentEditView` 的移动端抽屉用 `shadow-card`
 * 也躲过同一族不一致。"手写一份就必然带上那团下拉阴影"这句前提**是错的**。
 *
 * 现在的签名按结构取：**同一个样式对象里**同时出现
 *   ① position: fixed|absolute ② zIndex ③ boxShadow ④ 底色取自**表面令牌**
 * 四条。④ 是关键那条排除：拖拽插入指示线（`background: cv('blue')` + 发光阴影 + zIndex）
 * 长得像浮层但不是表面，实测它占了旧签名的 2 个假阳。
 * 取对象而不是取文本窗口：R11 的教训 —— 注释能把行窗口骗过去。
 */
const FLOAT_SURFACE_ALLOW = {
  'components/apple/ApDrawer.vue': '抽屉族：走 surface-float + shadow-modal 这条正典路径，不是下拉菜单',
  'components/space/reader/MobileTocFab.vue': '移动端目录抽屉：同上（底部抽屉）',
  'views/workspace/SpaceView.vue': '空间移动端抽屉：同上',
  'views/workspace/editor/DocumentEditView.vue': '编辑器移动端属性/评论抽屉：同上',
};
const SURFACE_TOKEN_BG = /(?:background|backgroundColor)\s*[:=][^,\n]*(?:surface-|bg-secondary|bg-tertiary|glass-bg|pure-white|near-black)/;
for (const f of files) {
  const r = rel(f);
  if (r === SHADOW_DROPDOWN_OWNER || SHADOW_DROPDOWN_ALLOW.includes(r) || FLOAT_SURFACE_ALLOW[r]) continue;
  const text = stripCommentsKeepLines(read(f));
  eachJsObject(text, (obj, start) => {
    if (!/position:\s*['"](?:fixed|absolute)['"]/.test(obj)) return;
    if (!/zIndex/.test(obj) || !/boxShadow/.test(obj) || !SURFACE_TOKEN_BG.test(obj)) return;
    errors.push(`G23b ${r}:${lineAt(text, start)} 手搓了一张浮层表面（position+zIndex+阴影+表面令牌底色四样齐）`
      + ` —— 浮层菜单请用 menuSurfaceStyle()，抽屉族请沿用 shadow-modal 并在此登记理由`);
  }, 1600);
}

// G24：浮层的"点别处收起"只准有一处实现（composables/ui/useClickOutsideClose）。
// 两个签名：① 有下拉/菜单面板（v-if="show…Menu|Dropdown" 或 role="menu"）却没接共用版 ——
// 这就是用户报的"点了别处不收"，因为共用版之前那三处菜单两样退路都没有；
// ② 文件里自己写了一份 document 级 click/mousedown 的 outside 处理 —— 那是在造第 N 份实现，
// 而各份的时机不一样（click 冒泡 vs pointerdown capture），冒泡版有"点另一个触发器会先关再开"的竞态。
const DISMISS_OWNER = 'composables/ui/useClickOutsideClose.ts';
// ApSearch 的建议面板由**焦点**驱动：blur 即收、Esc 也接（第 126 行），再叠一份 pointerdown
// 只会和 blur 那 150ms 的延时抢时序。它确实有退路，只是不是共用版那一条。
const DISMISS_ALLOW = ['components/apple/ApSearch.vue'];
for (const f of files) {
  const r = rel(f);
  if (r === DISMISS_OWNER || DISMISS_ALLOW.includes(r)) continue;
  if (!r.endsWith('.vue')) continue;
  const text = read(f);
  const hasPanel = /v-if="show[A-Za-z]*(Menu|Dropdown|Picker)\b/.test(text) || /role="menu"/.test(text);
  const usesShared = /use(Dismiss|ClickOutsideClose)\(/.test(text) || /useContextMenu\(/.test(text);
  if (hasPanel && !usesShared) {
    errors.push(`G24 ${r} 有下拉/菜单面板却没接 useDismiss/useClickOutsideClose —— 它多半没有"点别处收起"这条退路`);
  }
  const handRolled = /document\.addEventListener\('(click|mousedown|pointerdown)',\s*(handle\w*Outside|onGlobalMouse\w+)/.exec(text);
  if (handRolled) {
    errors.push(`G24 ${r}:${text.slice(0, handRolled.index).split('\n').length} 自己写了一份 document 级 outside 监听 —— 改用 useClickOutsideClose（时机与竞态处理只有一处）`);
  }
}

// G25：手势浮层的位移只准由那一份弹簧驱动（composables/ui/useSpring.ts + useSheetGesture.ts）。
// CSS transition 画不出"从当前呈现值接着动"：它只会从 keyframe 0% 重放，所以拖到一半松手的层
// 要么瞬移、要么等跑完才响应 —— 三个签名分别抓住三种"又回去用 keyframe"的写法：
//   ① `.style.transform =`：命令式逐帧推 transform，等于在组件里手写一遍积分器；
//   ② 同文件同时出现 `requestAnimationFrame` 与 `translate*(` 且没引共用弹簧：自己排帧做动画；
//   ③ `position: fixed` 的层用 `translateX(-100%) + transition: transform`：off-canvas keyframe 重放
//      （SpaceView 的移动端侧栏以前就是这个样子）。
const SPRING_OWNERS = ['composables/ui/useSpring.ts', 'composables/ui/useSheetGesture.ts'];
for (const f of files) {
  const r = rel(f);
  if (SPRING_OWNERS.includes(r)) continue;
  const text = read(f);
  const imperative = /\.style\.transform\s*=[^=]/.exec(text);
  if (imperative) {
    errors.push(`G25 ${r}:${text.slice(0, imperative.index).split('\n').length} 命令式改 .style.transform —— 位移请走 useSpring/useSheetGesture，别在组件里再写一套物理`);
  }
  if (/requestAnimationFrame/.test(text) && /translate(3d|X|Y)\(/.test(text) && !/use(Spring|SheetGesture)\(/.test(text)) {
    errors.push(`G25 ${r} 自己排 rAF 拼 transform —— 逐帧动画请复用 useSpring（打断与减弱动效它都处理了）`);
  }
  if (/position:\s*['"]?fixed/.test(text)
    && /translate[XY]\(-?100%?\)/.test(text)
    && /transition:[^;]*transform/.test(text)) {
    errors.push(`G25 ${r} 的 fixed 层还在用 translate(±100%) + transition 重放 —— 拖不动也打不断，请换成 useSheetGesture`);
  }
}

// G26：全屏遮罩只准有一处实现（styles/component/overlay.ts 的 getOverlayStyle）。
// 判据取 `position:'fixed'` 与 `background: cv('overlay')` **成对出现**（同一块样式里，中间隔不到 160 字）：
// 卡片内部那类 absolute 覆盖层（缩略图 hover、网格选中态）不是遮罩，不该被这条误伤。
// 并档前全站五份各写一遍：ApModal 内联、两份图片灯箱、SpaceView 抽屉、目录 sheet ——
// z 档取 modal / drawer / modal-1 三种，blur 有的有一份没有，透明度一份跟着状态跳、一份等 CSS 动画。
const OVERLAY_OWNER = 'styles/component/overlay.ts';
const OVERLAY_SIGN = /position:\s*'fixed'[\s\S]{0,160}background:\s*cv\('overlay'\)/;
for (const f of files) {
  const r = rel(f);
  if (r === OVERLAY_OWNER || !r.endsWith('.vue')) continue;
  const text = read(f);
  if (OVERLAY_SIGN.test(text) && !/getOverlayStyle\(/.test(text)) {
    errors.push(`G26 ${r}:${text.slice(0, OVERLAY_SIGN.exec(text).index).split('\n').length} 自己写了一份全屏遮罩 —— 请走 getOverlayStyle()（z 档、blur、透明度都能用参数覆盖）`);
  }
}

// G27：源码里不许出现"过渡布局属性"。运行时那台（check:apple A1）只量得到**渲染出来的**元素，
// 所以"没渲染的 transition: width"（上传进度、路由进度条、hover 才现的层）它永远报 0 ——
// 这条把那个盲区从静态侧堵上。
// `max-*` / `min-*` 是豁免档：正文阅读列宽切换天生要动尺寸（A1e 已带理由登记为受控例外），
// 而 `width`/`height`/`top`/`left`/`margin`/`padding` 没有这种"必须动它"的场景。
const LAYOUT_TRANSITION_PROPS = /\b(width|height|top|left|right|bottom|margin|padding)\b/;
for (const f of files) {
  const r = rel(f);
  const text = read(f);
  for (const m of text.matchAll(/transition(?:Property)?\s*[:=]\s*['"`]?([^;{}'"`\n]*)/g)) {
    const decl = m[1].replace(/\b(max|min)-[a-z-]+/g, '');
    if (!LAYOUT_TRANSITION_PROPS.test(decl)) continue;
    errors.push(`G27 ${r}:${text.slice(0, m.index).split('\n').length} 过渡点到了布局属性（${m[1].trim()}）—— 动画只准落在 transform/opacity 等合成器属性上，§11`);
  }
}

// G28：互斥选项组只准 `ApSegmentedControl` 一处实现。
// 签名是「实心档当当前项、ghost/secondary 当未选」这一对三元 —— 认的就是"用按钮颜色表达选中"，
// 不是任何动态 variant：分页（primary/outline）、侧栏当前项（tint/ghost）、
// 按决定换动作色（primary/danger）都不在射程内。
// 只抓"一组"：同一文件里 ≥2 处这种三元，或那一个按钮挂在 `v-for` 上（一个循环就是一组）。
// 单个开关按钮（`DocumentEditView` 的协作模式，1 处、无 v-for）不算——它不是选项组。
// 并档前这类选项组有 9 处（搜索页 6 组、评论筛选、写作面板、意图行、批量导入、模板分类、
// 媒体/日志/活动筛选），其中 `DocCommentPanel` 三颗全是实心 pill，根本看不出当前筛的是哪一档。
const SELECTED_BY_COLOR = /:variant="[^"]*\? 'primary' : '(?:ghost|secondary)'"/g;
for (const f of files) {
  const r = rel(f);
  const text = read(f);
  const sites = [...text.matchAll(SELECTED_BY_COLOR)];
  if (!sites.length) continue;
  const isGroup = sites.length >= 2
    || /v-for=/.test(text.slice(Math.max(0, sites[0].index - 200), sites[0].index));
  if (!isGroup) continue;
  errors.push(`G28 ${r}:${text.slice(0, sites[0].index).split('\n').length} 用按钮颜色表达选中态（${sites.length} 处 primary↔ghost/secondary）—— 互斥选项组请换 <ApSegmentedControl>（自带 role=tab / aria-selected / 方向键）`);
}

// G29：弹窗宽度只准走 `AppleModalSize` 那一把尺（`size="small|compact|regular|medium|large|xlarge"`）。
// ApModal 的面板本来就带 `maxWidth: 90vw`，所以页面再写 `width="min(640px, 90vw)"` 不是"更保险"，
// 而是在尺子外面又加一档 —— 并档前 3 处旁路（版本预览 640、版本对比 900、目标文件夹 500）
// 正好都卡在两档之间，谁宽谁窄没法比。`width` prop 仍留给"确实要非档位宽"的极少数，
// 但出现即报，逼人来解释。
const MODAL_WIDTH_BYPASS = /<ApModal\b[^>]*?\s(?::width|width)=/g;
for (const f of files) {
  const r = rel(f);
  const text = read(f);
  for (const m of text.matchAll(MODAL_WIDTH_BYPASS)) {
    errors.push(`G29 ${r}:${text.slice(0, m.index).split('\n').length} 给 <ApModal> 传了 width（${m[0].slice(-24)}）—— 请用 size= 走 AppleModalSize，窄屏封顶组件已带 maxWidth:90vw`);
  }
}

// G30：缓动曲线只准住在 `AppleEasing` 那一处。
// `<style>` 块读不到 JS 常量，所以之前 34 处各自手抄同一条 `cubic-bezier(0.25, 0.1, 0.25, 1)` ——
// 值是一样的，但改一次要动 8 个文件，而且没人会去改。现在曲线注入成 `--apple-easing-*`
// （style 段用 var()，脚本/内联用 AppleEasing.*），字面量只准出现在定义处。
const EASING_SOURCE = /export const AppleEasing = \{[\s\S]*?\n\} as const;/;
for (const f of files) {
  const r = rel(f);
  let text = read(f);
  if (r === 'utils/core/apple-tokens.ts') text = text.replace(EASING_SOURCE, '');
  const m = /cubic-bezier\(/.exec(text);
  if (m) {
    errors.push(`G30 ${r}:${text.slice(0, m.index).split('\n').length} 手抄缓动曲线 —— 用 var(--apple-easing-standard|emphasized|spring) 或 AppleEasing.*，曲线只有一处定义`);
  }
}

// G31：字号只准住在 `AppleType` 那一处。
// 之前有三张平行的字号表：`AppleFontSize`（14 键，注释自称只给 SVG 用、实际 28 处全是 DOM :style）、
// `AppleButtonSize`、`AppleInputSize`。后果是实测到的两类具体错误：
//   · 同名异值 —— `AppleFontSize.subhead`=15px 而 `AppleType.subhead`=19px（区块标题那一档），
//     写的人以为自己在用区块标题档；而 T12/G21 管的是 CSS 变量与单表内重复，管不到跨表的值轴碰撞。
//   · 天阶外字号 —— 11 / 16 / 18px 在 AppleType 的阶梯上不存在，`check:ui` 按页量的是 ladder 上的档，
//     所以跨页看每个尺寸都像单值。
// 判据两条：① `AppleFontSize` 这个标识符不许再出现（防复活）；② 剥掉 AppleType 定义块之后，
// 令牌层里再出现任何 `fontSize: '<N>px'` 裸字面量即失败 —— 要加一档就加进 AppleType（带派生字距）。
// 判据必须先把注释抹掉再数标识符：这条 G31 第一次跑就把自己写在 apple-tokens.ts 里那段
// "原先挂着 AppleFontSize 表"的说明文字抓成了违例。与 check:layout 的 R11 同一条教训。
const stripCommentsG31 = (t) => t
  .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
  .replace(/(^|[^:])\/\/[^\n]*/g, (_m, p) => p + ' '.repeat(_m.length - p.length));
for (const f of files) {
  const r = rel(f);
  const code = stripCommentsG31(read(f));
  const m = /\bAppleFontSize\b/.exec(code);
  if (m) {
    errors.push(`G31 ${r}:${code.slice(0, m.index).split('\n').length} 引用了 AppleFontSize —— 字号只准取 AppleType.<档>.size`);
  }
}
{
  const stripped = stripCommentsG31(tokensFile).replace(/export const AppleType = \{[\s\S]*?\n\} as const;/, '');
  const m = /fontSize:\s*['"`]-?[\d.]+px['"`]/.exec(stripped);
  if (m) {
    errors.push(
      `G31 utils/core/apple-tokens.ts:${stripped.slice(0, m.index).split('\n').length} ` +
        `字号表里写了裸 px（${m[0]}）—— 请引 AppleType.<档>.size，别再造第二张字号表`,
    );
  }
}

// G21：注册表里不许有两个键逐字节同值（"一个概念只有一个名字"在令牌层的延伸）。
for (const reg of ['AppleShadow', 'AppleGlassBlur']) {
  const block = tokensFile.match(new RegExp(`export const ${reg} = \\{([\\s\\S]*?)\\n\\}`, 'm'))?.[1] || '';
  const byNorm = new Map();
  for (const m of block.matchAll(/(\w+):\s*['"`]([^'"`]*)['"`]/g)) {
    const norm = m[2].replace(/\s+/g, '');
    if (!norm) continue;
    if (byNorm.has(norm)) errors.push(`G21 ${reg}.${m[1]} 与 ${byNorm.get(norm)} 值逐字节相同 —— 删掉一个，别留两个名字`);
    else byNorm.set(norm, m[1]);
  }
}

// G33：实心强调色上的前景必须由色值决定（on-blue / on-{hue}），不许用恒白的 on-accent。
// 动机不是洁癖，是批3 加「强调色可由用户选」之后暴露的真问题：`--apple-on-accent` 是"恒浅色"，
// 它只在内置那几枚足够深的强调色上才读得出来。门禁量到的原始数字：
// notion-paper 的 on-accent 是深墨 #2b1a05（因为它自己的强调色是橙），而白字压在新加的深色档位上只有 2.3~2.6:1。
// 头像首字母、计数徽章、步骤号、复选勾、主按钮 —— 八处都踩在同一条捷径上。
// 判法按窗口而不是逐行：一个对象字面量里 `background: cv('blue')` 与 `color: cv('on-accent')`
// 是同一种东西的两半，逐行判就等于看不见（这正是本仓"同一条规则别按单行写"的那类教训）。
// **零容忍，不留棘轮**：存量 8 处已在批3 全部迁到 on-blue，留一个基线数只会让下一处混过去。
const G33_BG = /background(?:Color)?\s*[:=]\s*["'`]*\s*cv\('blue'\)/;
const G33_FG = /color\s*[:=]\s*["'`]*\s*cv\('on-accent'\)/;
const G33_WINDOW = 8;
for (const f of files) {
  const r = rel(f);
  if (r === 'utils/core/apple-tokens.ts') continue; // 令牌层定义 on-accent 本身，不在此列
  const lines = strip(read(f)).split('\n');
  lines.forEach((line, i) => {
    if (!G33_FG.test(line)) return;
    if (!lines.slice(Math.max(0, i - G33_WINDOW), i).some((w) => G33_BG.test(w))) return;
    errors.push(`G33 ${r}:${i + 1} 实心强调色上用恒白 on-accent 当前景，请改用 cv('on-blue')（由 contrastInkOn 按实际色值算）—— 用户能选强调色之后，恒白压不住浅档`);
  });
}

// G34：入场编排（版面人格，批4）只准出现在登记的公开版面。
// 为什么不是"禁掉 stagger"而是"登记射程"：G1 禁的是被删掉的通用装饰机制复活（渐变卡 / Sparkles /
// 交错入场那套实现），这条管的是新加的一种**有边界的能力** —— 登录 / 安装 / 分享 / 错误页是低频、
// 会被转发、不承担操作效率的版面，在那里做人格是 brand.md 早就开过的「B 进取」档；
// 而产品内页 48 个路由每天都滚进滚出，Apple 的原话是"不要在高频交互上加动效"。
// 所以判据是结构性的：**用没用这套类名/标记**，不在允许清单里即红。
// 新增公开版面必须在这里写一行理由，否则就是一处"没登记的能力扩张"。
const PERSONA_ALLOW = new Map([
  ['utils/core/apple-tokens.ts', '编排本体的定义处（关键帧与 :root[data-persona] 那批规则）'],
  ['styles/theme/personalization.ts', '档位清单与 DOM 属性的定义处'],
  ['components/apple/ApBrand.vue', '品牌标记自己只打 data-brand-part 与 enter 类，是否动由版面人格决定'],
  ['views/auth/LoginView.vue', '公开版面：登录页'],
  ['views/auth/AuthorLoginView.vue', '公开版面：作者登录'],
  ['views/auth/InstallView.vue', '公开版面：安装向导'],
  ['views/share/ShareView.vue', '公开版面：分享页（唯一会被转发给外部人的版面）'],
  ['components/apple/ApErrorPage.vue', '公开版面：整页错误'],
]);
const PERSONA_USE = /apple-persona-stack|ap-brand-enter|data-brand-part|data-persona/;
for (const f of files) {
  const r = rel(f);
  if (PERSONA_ALLOW.has(r)) continue;
  const text = strip(read(f));
  const m = PERSONA_USE.exec(text);
  if (!m) continue;
  errors.push(`G34 ${r}:${text.slice(0, m.index).split('\n').length} 用了入场编排标记 ${m[0]}，但它不在公开版面登记清单里 —— 产品内页不许加入场动效（HIG：别在高频交互上加动效）；确实新增了一个公开版面，就在 PERSONA_ALLOW 写一行理由`);
}

if (errors.length) {
  console.error(`\n✗ 设计教条检查未通过（${errors.length} 项）：`);
  for (const e of errors.slice(0, 60)) console.error('  ' + e);
  if (errors.length > 60) console.error(`  … 另有 ${errors.length - 60} 项`);
  process.exit(1);
}
console.log(`ℹ 设计教条：${files.length} 个文件，无装饰机制复活 / 无 chrome emoji / 表面与圆角均走令牌`);
