#!/usr/bin/env node
/**
 * 样式一致性守卫（禁止 CSS 文件体系下的静态校验）
 * 用法：npm run check:style
 * 校验项：
 *  1. cv('x') / var(--apple-x) 引用的变量必须存在于 apple-tokens.ts 注册表
 *  2. 禁止 var()/cv() 之后拼接 hex alpha（`var(--apple-blue)15`）
 *  3. 禁止裸 zIndex 数字字面量（必须走 AppleZIndex）
 *  4. 禁止新增 AppleSpace['Npx'] / AppleRadius['Npx'] 像素键（存量见 BASELINE）
 *  5. .vue 中 <style> 必须 scoped（App.vue 全局基线例外）
 *  6.（已迁出）颜色变量的各主题覆盖完整性由 scripts/check-theme-contract.mjs 的 T5 负责
 *  7. 毛玻璃模糊值必须走 cv('glass-blur')，材质由主题决定
 */
import fs from 'fs';
import path from 'path';
import { eachJsObject } from './lib/js-objects.mjs';

const SRC = path.resolve('src');
const TOKENS = path.join(SRC, 'utils/core/apple-tokens.ts');

// 存量像素键基线（只降不升；整改时同步下调这里的数字）
// 使用点基线: 统计 AppleSpace['Npx'] / AppleRadius['Npx'] 方括号取用次数
// 注册表基线: 统计 apple-tokens.ts 中 'Npx': 'Npx' 像素键定义数量
const BASELINE = {
  spacePixelKeys: 22,        // 使用点（2026-10-02 进度条并档 25→23：两条 6px 轨道改走 AppleSpace 档位；2026-10-26 面板头并档 23→22：DocCommentPanel 的 ['6px'] 改走 xs，与 DocProperties 统一）
  radiusPixelKeys: 15,       // 2026-10-15：引文预览并回唯一浮层表面后，它自己那份 ['medium'] 圆角随副本消失；2026-10-19 一致性批次：ApThemeMenu 色块的 '0.3125rem' 旁路改走已注册的 ['5px']（合法用键 +1）
       // 使用点（2026-10-03 图片灯箱并档 16→15：预览图的 ['22px'] 随四份副本一起消失；2026-09-28 重绘 26→22；2026-10-02 标签/徽章圆角并档 22→17：['5px'] 与两处 ['8px'] 改走 AppleRadius.control；同日进度条并档 17→16：上传条的 ['3px'] 随轨道改走 pill）
  // 2026-10-02 再由 742 降 10：10 份手写浮层表面并成 menuSurfaceStyle() 一份（'6px' 外边距、'1px'/'0.5px' 描边、'9.25rem' 一类随之消失）
  spaceRegistryKeys: 31,     // 注册表定义
  radiusRegistryKeys: 20,    // 注册表定义
  // .vue 本地样式对象与 styles/** 导出逐字节相同。
  // 2026-09-29 计数口径变严（不是新增债务）：norm() 不再把尾逗号当成不同的值（+17），
  // 且开始统计 `const X = computed(() => ({…}))` 形态（+3，含设计系统自己的 ApSkeleton 三条）。
  // 旧基线 112 对应旧口径下的 107；新口径实测 127，从这里继续只降不升。
  // 2026-09-30 AITokenView 删掉一整块没有模板引用的遗留样式（其中两处是共享样式的逐字节克隆）后为 124。
  localStyleClones: 108,     // 2026-11-15 P2-4：横排/竖排三行对象并回 rowStyle()/listStyle()（-11），
                             // 聚光搜索结果图标样式并回 spotlight-search-styles，119→108
  // 2026-10-02：批 C 并档 124→122，随后 AIWriterPanel 的两处本地空态并回 <ApEmpty> 再降一条
  // 2026-10-01 由 757 降 3：ApSlider 的 4px 视觉轨道改由 `::-webkit-slider-runnable-track` 画
  // （input 本身换成 `AppleInputSize.small` 令牌），实体卡片元信息槽与面包屑改走 overflow/ellipsis 不再补 px。
  pagePxLiterals: 694,       // 2026-10-15 一致性批次：spinner 时长并成 --apple-spinner-duration（7 处手抄消失）、6 处裸 repeat(N, 1fr) 并回 appleFixedColumns，随之少掉裸 px；2026-10-19 一致性批次：三处登录/安装卡 clamp padding 提为共享 authCardPadding（-5）；2026-10-26 走查修复 695→693：搜索框 flex-basis 改纯弹性（'1 1 0'）、侧栏收起按钮圆角改 AppleRadius.small；2026-11-15 手动取代弹窗 +1（CitebaseCompileView supersede modal）
       // .vue 样式属性里的裸 px 字面量（2026-10-03 工具条并轨：弹窗宽度 3 处旁路并回 AppleModalSize，min(640px,90vw)/min(900px,95vw) 里那两枚裸 px 随之消失 729→727）（2026-10-03 图片灯箱并档 732→730：90vh/80vh 那两份预览尺寸随副本消失）（重绘去掉 hero 字号/徽章 px；cv('space-12px') 等令牌引用不再计数。2026-10-02 批 C：目录 FAB 48→触控档、回到顶部偏移 24/20→AppleSpace.xl、卡片内边距 xl→lg）
  /**
   * `.vue` 里 `|| '中文'` / `?? '中文'` 这种**兜底位硬编码中文**。
   * 为什么运行时那台巡检看不见：`check:i18n-runtime` 量的是"渲染出来的中文"，
   * 而兜底**只在数据缺失时渲染** —— 库里文档都有名字，`|| '文档'` 就永远不出现在 DOM 里，
   * 于是它 51 条路由全绿的同时，这 154 处一直躺在源码里。英文界面一旦碰上无名字的记录，
   * 吐出来的就是中文。
   * 2026-10-15 全部收进 `fallback` 命名空间（117 条词条，zh/en 各一份），基线钉在 0：
   * 这一族从此**不得复活**。新加一处兜底标签，请同时加两份词条。
   */
  hardcodedCjkFallback: 0,
                             // 2026-10-01 由 758 降 1：ApSwitch 的 minHeight '44px' 改成 AppleLayout.touchTargetMin
};
// 允许非 scoped <style> 的历史文件（选择器命中 v-html 渲染内容，scoped 后会失效，需配 :deep 迁移）
const SCOPED_ALLOWLIST = ['components/ai/chat/ChatMessageBubble.vue'];

const walk = (d, out = []) => {
  for (const e of fs.readdirSync(d, { withFileTypes: true })) {
    const p = path.join(d, e.name);
    if (e.isDirectory()) walk(p, out);
    else if (/\.(vue|ts)$/.test(e.name) && !e.name.endsWith('.d.ts')) out.push(p);
  }
  return out;
};

const files = walk(SRC);
const read = (f) => fs.readFileSync(f, 'utf8').replace(/\r\n/g, '\n');
/** 去注释（保留行号），避免文档里的示例值被当成引用 */
const stripComments = (t) =>
  t
    .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
    .replace(/(^|[^:])\/\/[^\n]*/g, (m, p1) => p1 + ' '.repeat(m.length - p1.length));
const rel = (f) => path.relative(SRC, f).replace(/\\/g, '/');
const errors = [];
const warn = [];

/* ---- 注册表 ---- */
const tok = read(TOKENS);
const registered = new Set([...tok.matchAll(/^\s*'(--apple-[a-z0-9-]+)'\s*:/gm)].map((m) => m[1]));
if (!registered.size) {
  console.error('✗ 无法解析 apple-tokens.ts 的 CSS 变量注册表，守卫失效');
  process.exit(1);
}

/* ---- 逐文件扫描 ---- */
let spaceKeys = 0;
let radiusKeys = 0;
for (const f of files) {
  const isTokens = f === TOKENS;
  const raw = read(f);
  const t = stripComments(raw);
  t.split('\n').forEach((line, i) => {
    const at = `${rel(f)}:${i + 1}`;
    // 含正则字面量的行: var() 不是真实引用，跳过规则 1，但规则 2/3/4 仍需检查
    const hasRegexLit = /\.match\(|new RegExp|RegExp\(/.test(line);
    if (!isTokens) {
      // 1. 变量必须注册（正则字面量里的 var() 不是引用，跳过）
      if (!hasRegexLit) {
        for (const m of line.matchAll(/var\(--apple-([a-z0-9-]+)(?:\s*,[^)]*)?\)/g)) {
          if (!registered.has(`--apple-${m[1]}`)) errors.push(`${at} 引用未注册变量 var(--apple-${m[1]})`);
        }
        for (const m of line.matchAll(/cv\(\s*['"]([a-z0-9_-]+)['"]/g)) {
          if (!registered.has(`--apple-${m[1]}`)) errors.push(`${at} 引用未注册变量 cv('${m[1]}') → --apple-${m[1]}`);
        }
      }
      // 2. 颜色拼接（补 var(--apple-x) 直接拼接 hex alpha 形态）
      if (/(?:\$\{cv\('[a-z0-9-]+'\)|cv\('[a-z0-9-]+'\) *\+ *)['"`]?[0-9a-fA-F]{2}/.test(line))
        errors.push(`${at} var()/cv() 拼接 hex alpha（非法 CSS），改用 *-bg-subtle / *-selection / withAlpha()`);
      // color-mix() 里的 `var(--apple-x) 12%` 是合法语法，百分号结尾不算拼接
      if (/var\(--apple-[a-z0-9-]+\)[\s'"`]*[0-9a-fA-F]{2,8}(?![\w%])/.test(line))
        errors.push(`${at} var() 后拼接 hex alpha（非法 CSS），改用 *-bg-subtle / *-selection / withAlpha()`);
      // 3. 裸 z-index（同时匹配驼峰 zIndex 和 kebab z-index）
      if (/(?:zIndex|z-index)\s*[:=]\s*['"]?\d/.test(line) && !/AppleZIndex/.test(line)) errors.push(`${at} 裸 z-index 字面量，请改用 AppleZIndex`);
    }
    // 4. 像素键计数（只统计真正的 px 字面量键，'2xl'/'3xl' 等语义键不计）
    spaceKeys += (line.match(/AppleSpace\[\s*['"]\d+px['"]\s*\]/g) || []).length;
    radiusKeys += (line.match(/AppleRadius\[\s*['"]\d+px['"]\s*\]/g) || []).length;
  });

  // 5. <style> 必须 scoped（只看 SFC 顶层块，忽略 JS 模板字符串里为导出 HTML 写的 style）
  if (f.endsWith('.vue') && rel(f) !== 'App.vue' && !SCOPED_ALLOWLIST.includes(rel(f))) {
    const outsideScript = raw.replace(/<script[\s\S]*?<\/script>/g, '');
    for (const m of outsideScript.matchAll(/<style([^>]*)>/g)) if (!/\bscoped\b/.test(m[1])) errors.push(`${rel(f)} 存在非 scoped <style> 块（规范要求 <style scoped>）`);
  }
}

/* ---- 6. 颜色变量的主题覆盖完整性：已迁到 scripts/check-theme-contract.mjs 的 T5 ----
 * 原先这里只认 apple-tokens.ts 里手写的 [data-theme='dark'] CSS 串；暗色现在与扩展主题
 * 同为 themes.ts 里的数据表，由主题守卫视觉解析，不再重复实现一遍。 */

/* ---- 7. 毛玻璃模糊值必须走主题令牌 ---- */
for (const f of files) {
  if (f === TOKENS) continue;
  const t = stripComments(read(f));
  t.split('\n').forEach((line, i) => {
    if (/(?:webkit-)?[Bb]ackdropFilter\s*[:=]\s*['"`]blur\(/.test(line) || /backdrop-filter\s*:\s*blur\(/.test(line))
      errors.push(`${rel(f)}:${i + 1} 硬编码 blur 值，请改用 cv('glass-blur')（材质由主题决定）`);
  });
}

/* ---- 10. padding 简写不得出现在 padding* 长写之后（同一对象内） ----
 * CSS-in-JS 里对象键序就是声明序：`paddingTop` 写在 `padding` 简写前面时，
 * 简写会把 paddingTop 重置成它自己的值，而**界面上什么都不报错**。
 * 2026-10-15 这个坑一轮里咬了两次：contentStyle 的顶栏让位被 `padding:'0'` 重置；
 * sidebarStyle 的让位被重置后，搜索框跑到浮层顶栏背后，用户报的是"搜索框没了"。
 * 反向顺序（简写在前、长写在后）是有意覆盖，放行。 */
const eachObject = (text, cb) => eachJsObject(text, cb);
for (const f of files) {
  const t = stripComments(read(f));
  eachObject(t, (obj, start) => {
    if (obj.length > 4000) return;
    const longs = [...obj.matchAll(/padding(?:Top|Bottom|Left|Right)\s*:/g)].map((m) => m.index);
    const shorts = [...obj.matchAll(/(?<![-\w])padding\s*:/g)].map((m) => m.index);
    if (longs.length && shorts.some((s) => longs.some((l) => s > l))) {
      const line = t.slice(0, start).split('\n').length;
      errors.push(`${rel(f)}:${line} 同一对象里 padding 简写出现在 padding* 长写之后，长写会被静默重置（键序即声明序）`);
    }
  });
}

/* ---- 8/9. 关键帧：重名检测 + JS 引用可解析性 ---- */
const GLOBAL_SOURCES = ['utils/core/apple-tokens.ts'];
// 全局关键帧只住一处：apple-tokens 注入的那一个 <style id="apple-keyframes">。
// （原先 styles/global-animations.ts 另注入了 spin，两块 <style> 两套
//   “谁负责全局动画”；那个文件已删。）
const globalKf = new Set();
const kfOwner = new Map();
for (const f of files) {
  const r = rel(f);
  const t = read(f);
  for (const m of t.matchAll(/@keyframes\s+([\w-]+)/g)) {
    if (GLOBAL_SOURCES.includes(r)) globalKf.add(m[1]);
    if (!kfOwner.has(m[1])) kfOwner.set(m[1], []);
    kfOwner.get(m[1]).push(r);
  }
}
for (const [name, owners] of kfOwner) {
  const uniq = [...new Set(owners)];
  if (uniq.length > 1) errors.push(`@keyframes ${name} 在多个文件重复定义：${uniq.join(', ')}`);
}
for (const f of files) {
  if (f === TOKENS) continue;
  const r = rel(f);
  const t = stripComments(read(f));
  const own = new Set([...t.matchAll(/@keyframes\s+([\w-]+)/g)].map((m) => m[1]));
  t.split('\n').forEach((line, i) => {
    const m = line.match(/animation(?:-name)?\s*[:=]\s*['"`]?([\w-]+)[\s,;'"]/);
    if (!m) return;
    const name = m[1];
    if (name === 'none' || name === 'inherit') return;
    if (!globalKf.has(name) && !own.has(name))
      errors.push(`${r}:${i + 1} animation 引用了无法解析的关键帧 "${name}"（既非全局注入，也不在本文件定义；scoped keyframes 会被哈希，JS 内联串无法引用）`);
  });
}

/* ---- 10. 星号再导出被本地同名定义遮蔽（静默降级，消费方拿不到完整样式） ---- */
const STYLE_DIR = path.resolve(SRC, 'styles');

function resolveSpec(spec, fromFile) {
  if (!spec.startsWith('@/') && !spec.startsWith('.')) return null;
  const base = spec.startsWith('@/') ? path.join(SRC, spec.slice(2)) : path.resolve(path.dirname(fromFile), spec);
  for (const cand of [base, `${base}.ts`, `${base}.vue`, path.join(base, 'index.ts')]) {
    if (fs.existsSync(cand) && fs.statSync(cand).isFile()) return cand;
  }
  return null;
}

/** 从 '{' 处起做括号配平，取回对象字面量原文 */
function objectLiteralAt(text, braceIdx) {
  let depth = 0;
  for (let i = braceIdx; i < text.length; i++) {
    if (text[i] === '{') depth += 1;
    else if (text[i] === '}') { depth -= 1; if (!depth) return text.slice(braceIdx, i + 1); }
  }
  return null;
}

// 尾逗号不参与比较：`{a:1,}` 与 `{a:1}` 是同一份样式，旧版把它们当成两种值，
// 于是 17 条逐值相同的克隆数不出来（改共享版时它们照样不跟）。
const norm = (s) => s.replace(/\s+/g, '').replace(/,}/g, '}');

function objectConsts(text) {
  const out = new Map();
  for (const m of text.matchAll(/^\s*(?:export\s+)?const\s+([A-Za-z0-9_$]+)\s*(?::[^=]+)?=\s*\{/gm)) {
    const body = objectLiteralAt(text, m.index + m[0].length - 1);
    if (body) out.set(m[1], body);
  }
  // 响应式样式对象（const X = computed(() => ({…}))）同样是克隆源，旧版只认 `= {` 所以全漏，
  // 设计系统自己的 ApSkeleton 三条就是这样藏住的。
  for (const m of text.matchAll(/^\s*(?:export\s+)?const\s+([A-Za-z0-9_$]+)\s*(?::[^=]+)?=\s*computed\s*\(\s*\(\)\s*=>\s*\(\s*\{/gm)) {
    const body = objectLiteralAt(text, m.index + m[0].length - 1);
    if (body) out.set(m[1], body);
  }
  return out;
}

const styleFiles = files.filter((f) => f.startsWith(STYLE_DIR));
const styleOwnNames = new Map();
const styleOwnBodies = new Map();
const starExports = new Map();
const sharedBodies = new Map();
for (const f of styleFiles) {
  const text = stripComments(read(f));
  const consts = objectConsts(text);
  const ownNames = new Set([...text.matchAll(/^\s*export\s+(?:const|function|type|interface)\s+([A-Za-z0-9_$]+)/gm)].map((m) => m[1]));
  // `export { a as b } from '…'` 同样往本模块的导出面上放一个符号名（这里取别名 b）。
  // 不采集它，就看不见 ai-page-styles 用 form-field-styles 的 formGridStyle 顶掉
  // page-styles 里同名的 aiFormGridStyle —— 消费方以为拿到 auto-fit，实际拿到写死的两列。
  for (const blk of text.matchAll(/^\s*export\s*\{([^}]*)\}\s*from/gm)) {
    for (const part of blk[1].split(',')) {
      const seg = part.trim();
      if (!seg || seg === 'type') continue;
      const alias = /\bas\s+([A-Za-z0-9_$]+)/.exec(seg);
      ownNames.add(alias ? alias[1] : seg.replace(/^type\s+/, '').trim());
    }
  }
  styleOwnNames.set(f, ownNames);
  styleOwnBodies.set(f, consts);
  for (const [name, body] of consts) {
    const nb = norm(body);
    if (nb.length >= 40 && !sharedBodies.has(nb)) sharedBodies.set(nb, `${rel(f)}/${name}`);
  }
  starExports.set(f, [...text.matchAll(/^export\s+\*\s+from\s+['"]([^'"]+)['"]/gm)].map((m) => resolveSpec(m[1], f)));
}

function starClosure(f, seen = new Set()) {
  for (const t of starExports.get(f) || []) {
    if (!t || seen.has(t)) continue;
    seen.add(t);
    starClosure(t, seen);
  }
  return seen;
}

for (const [f, own] of styleOwnNames) {
  for (const src of starClosure(f)) {
    for (const name of styleOwnNames.get(src) || []) {
      if (own.has(name)) errors.push(`${rel(f)} 重复导出 ${name}，本地定义会遮蔽来自 ${rel(src)} 的星号再导出（消费方静默拿到降级版）`);
    }
  }
}

/* ---- 11. 页面本地克隆共享样式对象（逐字节相同） ---- */
let styleClones = 0;
const cloneExamples = [];
for (const f of files) {
  if (!f.endsWith('.vue') || f.startsWith(STYLE_DIR)) continue;
  for (const [name, body] of objectConsts(stripComments(read(f)))) {
    const nb = norm(body);
    if (nb.length >= 40 && sharedBodies.has(nb)) {
      styleClones += 1;
      if (cloneExamples.length < 12) cloneExamples.push(`${rel(f)} 的 ${name} ≡ ${sharedBodies.get(nb)}`);
    }
  }
}

/* ---- 11b. 页面本地样式与"本文件导入的样式模块"导出面重名（遮蔽报警） ----
   只查真正危险的撞名：本地定义的名字存在于该文件 import 的样式模块（含星号再导出）里 ——
   未来把本地 const 改成 import 时会静默拿到共享版，或反之。泛名如 headerStyle 与
   未导入模块的撞名不在此列（那是本文件自己的命名空间，不构成遮蔽）。
   纯别名转发（const x = sharedY;）不算。 */
function exportSurfaceOf(f, seen = new Set()) {
  if (!f || seen.has(f)) return [];
  seen.add(f);
  const names = [...(styleOwnNames.get(f) || [])];
  for (const t of starClosure(f)) names.push(...(styleOwnNames.get(t) || []));
  return names;
}
for (const f of files) {
  if (!f.endsWith('.vue') || f.startsWith(STYLE_DIR)) continue;
  const text = stripComments(read(f));
  const surface = new Set();
  for (const m of text.matchAll(/from\s+['"](@\/styles\/[^'"]+|(\.\.?\/[^'"]*styles[^'"]*))['"]/g)) {
    const spec = m[1];
    const target = resolveSpec(spec.replace(/^@\//, ''), f);
    for (const n of exportSurfaceOf(target)) surface.add(n);
  }
  if (surface.size === 0) continue;
  for (const m of text.matchAll(/^\s*(?:const|function)\s+([A-Za-z0-9_$]+)\b([^\n]*)/gm)) {
    const name = m[1];
    if (!surface.has(name)) continue;
    // 纯别名转发：const x = sharedY; —— 有意的本地改名，不是遮蔽
    if (/^\s*=\s*[A-Za-z0-9_$]+\s*;?\s*$/.test(m[2])) continue;
    errors.push(`${rel(f)} 本地定义 ${name} 与其导入的样式模块导出重名（遮蔽/混淆，请重命名本地名或改用共享版）`);
  }
}

/* ---- 12. 页面内裸 px 字面量（样式属性上的魔法数字） ---- */
let pagePx = 0;
for (const f of files) {
  if (!f.endsWith('.vue')) continue;
  stripComments(read(f)).split('\n').forEach((line) => {
    // 只数真正的魔法数字：cv('space-12px') / var(--apple-space-12px) 属于令牌引用，先摘掉
    const bare = line.replace(/cv\('[^']*px'\)/g, '').replace(/var\(--apple-[a-z0-9-]*px\)/g, '');
    if (/[A-Za-z][A-Za-z0-9]*\s*[:=]\s*[^;\n]*\b\d+px\b/.test(bare)) pagePx += (bare.match(/\b\d+(\.\d+)?px\b/g) || []).length;
  });
}

/* ---- 13. 度量注册表零引用键（存在动态取值风险，只做只降不升的提示） ---- */
const allSrc = files.map((f) => stripComments(read(f))).join('\n');
const deadKeys = [];
for (const obj of ['AppleSpace', 'AppleRadius', 'AppleLayout', 'AppleFontSize', 'AppleTransition', 'AppleShadow', 'AppleZIndex', 'AppleEasing', 'AppleGlassBlur', 'AppleModalSize', 'AppleInputSize', 'AppleButtonSize', 'AppleStatusBadgeSize', 'AppleBreakpoint']) {
  const block = tok.match(new RegExp(`export const ${obj}\\s*=\\s*\\{([\\s\\S]*?)\\n\\}`))?.[1];
  if (!block) continue;
  if (new RegExp(`${obj}\\[\\s*[^'\\"]`, 'm').test(allSrc)) continue; // 动态索引，键名无法静态判定
  for (const m of block.matchAll(/^\s*(?:'([A-Za-z0-9_-]+)'|([A-Za-z0-9_]+))\s*:/gm)) {
    const key = m[1] || m[2];
    if (!key) continue;
    const used = new RegExp(`${obj}\\.${key}\\b|${obj}\\[\\s*['"]${key}['"]\\s*\\]`).test(allSrc);
    if (!used) deadKeys.push(`${obj}.${key}`);
  }
}

/* ---- 输出 ---- */
const report = (label, n, max) => (n > max ? errors.push(`${label} 数量 ${n} 超过基线 ${max}（只降不升）`) : warn.push(`${label} ${n} / 基线 ${max}`));
report('AppleSpace 像素键', spaceKeys, BASELINE.spacePixelKeys);
report('AppleRadius 像素键', radiusKeys, BASELINE.radiusPixelKeys);
// 同时统计注册表里的像素键定义数量，防止新增 'Npx': 'Npx' 键不受约束
const spaceSection = tok.match(/AppleSpace\s*=\s*\{([\s\S]*?)\}/)?.[1] || '';
const spaceRegistryKeysActual = (spaceSection.match(/^\s*'\d+px'\s*:\s*'\d+px'/gm) || []).length;
const radiusSection = tok.match(/AppleRadius\s*=\s*\{([\s\S]*?)\}/)?.[1] || '';
const radiusRegistryKeys = (radiusSection.match(/^\s*'\d+px'\s*:\s*'\d+px'/gm) || []).length;
report('AppleSpace 注册表像素键', spaceRegistryKeysActual, BASELINE.spaceRegistryKeys);
report('AppleRadius 注册表像素键', radiusRegistryKeys, BASELINE.radiusRegistryKeys);
report('页面本地克隆共享样式', styleClones, BASELINE.localStyleClones);
report('页面裸 px 字面量', pagePx, BASELINE.pagePxLiterals);
// 兜底位硬编码中文：`|| '中文'` / `?? '中文'`。运行时那台巡检看不见这一族
// （兜底只在数据缺失时渲染，库里记录都有名字时它压根不进 DOM）。
let cjkFallback = 0;
const cjkFallbackSamples = [];
for (const f of files) {
  if (!f.endsWith('.vue')) continue;
  const t = stripComments(read(f));
  const m = t.match(/(?:\|\||\?\?)\s*'[^']*[一-鿿][^']*'/g) || [];
  if (!m.length) continue;
  cjkFallback += m.length;
  if (cjkFallbackSamples.length < 6) cjkFallbackSamples.push(`${path.relative(SRC, f).split(path.sep).join('/')} ×${m.length}`);
}
report('兜底位硬编码中文', cjkFallback, BASELINE.hardcodedCjkFallback);
if (cjkFallbackSamples.length) warn.push(`  样例（按文件计处数）：${cjkFallbackSamples.join('、')} …`);
if (cloneExamples.length) warn.push(`克隆样例：\n  - ${cloneExamples.join('\n  - ')}`);
if (deadKeys.length) warn.push(`度量注册表零引用键 ${deadKeys.length} 个（动态取值无法静态判定，只提示）：${deadKeys.slice(0, 15).join(', ')}${deadKeys.length > 15 ? ' …' : ''}`);

if (warn.length) console.log('ℹ ' + warn.slice(0, 20).join('\nℹ ') + (warn.length > 20 ? `\nℹ …共 ${warn.length} 条提示` : ''));
if (errors.length) {
  console.error(`\n✗ 样式一致性检查未通过（${errors.length} 项）：\n` + errors.slice(0, 40).join('\n') + (errors.length > 40 ? `\n…共 ${errors.length} 项` : ''));
  process.exit(1);
}
console.log(`✓ 样式一致性检查通过（扫描 ${files.length} 个文件；注册变量 ${registered.size}）`);
