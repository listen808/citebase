// 把两套语言包的每条消息喂给 vue-i18n 底层的消息编译器，报出编译不过的那几条。
//
// 为什么要单独有一台：`@` 与 `{` 在消息格式里是语法（`@:key` 链接、`{name}` 插值），
// 而"该文档还没有 @提及""可填 JSON，如 {"keep":4}"这类文案长得完全正常。
// 消息编译不过时 vue-i18n 回退返回原文，所以**页面看不出坏**，只在渲染那一刻刷控制台；
// type-check / eslint / 构建都不会红。转义写法是字面量模式：`{'@'}`、`{'{'}`、`{'}'}`。
//
// 用法（web/ 下）：node scripts/i18n-msg-audit.mjs
import fs from 'node:fs';
import { baseCompile } from '@intlify/message-compiler';

const LEAF = /^ {4}([A-Za-z0-9_]+): (['`])((?:\\.|(?!\2).)*)\2,$/;
const BLOCK = /^ {2}([A-Za-z0-9_]+): \{/;

function leaves(p) {
  const out = [];
  let cur = '';
  fs
    .readFileSync(p, 'utf8')
    .split('\n')
    .forEach((line, i) => {
      const bm = line.match(BLOCK);
      if (bm) {
        cur = bm[1];
        return;
      }
      const e = line.match(LEAF);
      if (e && cur) out.push({ n: i + 1, key: `${cur}.${e[1]}`, raw: e[3] });
    });
  return out;
}

// 语言包源码里是转义形态（`\n` 两个字符），编译器要的是成品形态。
const parsed = (s) => s.replace(/\\([\s\S])/g, (m, c) => (c === 'n' ? '\n' : c === 't' ? '\t' : c === 'r' ? '\r' : c));

let total = 0;
for (const p of ['src/i18n/locales/zh-CN.ts', 'src/i18n/locales/en-US.ts']) {
  const bad = [];
  for (const l of leaves(p)) {
    const codes = [];
    try {
      baseCompile(parsed(l.raw), { onError: (t) => codes.push(String(t.code)), onWarn: (t) => codes.push('warn:' + t.code) });
    } catch (e) {
      codes.push('throw:' + String(e.message).slice(0, 40));
    }
    if (codes.length) bad.push({ ...l, codes });
  }
  total += bad.length;
  console.log(`${p} → ${bad.length} 条消息编译不过`);
  for (const b of bad) console.log(`  ${b.n} ${b.key} [${[...new Set(b.codes)].join(',')}] ${b.raw.slice(0, 90)}`);
}
if (total > 0) process.exit(1);

/* ---- 引用到的键必须真的存在于两套语言包 ----
 * 为什么单独有这一条：vue-i18n 找不到键时**不报错、不崩、不刷 error**，
 * 它把键名原样返回给你 —— 于是界面上会出现 `fallback.k12` 这种字符串，
 * 而 type-check / eslint / 构建 / 运行时错误巡检（check:runtime:pw 只看 console 与 5xx）
 * **没有一台会红**。批量搬运（codemod）最容易造出这种键，所以它必须有一台判据。
 * 只判字面量键：动态拼出来的键静态判不了，跳过并如实说明，不去猜。
 * `server.` 那一族不住在语言包里（它是后端话术的译文，见 locales/server-messages.ts），排除。 */
const KEY_REF = /(?:^|[^.\w])\$?t\(\s*'([a-zA-Z0-9_]+\.[a-zA-Z0-9_]+)'/g;
const localeKeys = (p) => new Set(leaves(p).map((l) => l.key));
const zhKeys = localeKeys('src/i18n/locales/zh-CN.ts');
const enKeys = localeKeys('src/i18n/locales/en-US.ts');
const walkSrc = (d, out = []) => {
  for (const e of fs.readdirSync(d, { withFileTypes: true })) {
    const p = `${d}/${e.name}`;
    if (e.isDirectory()) walkSrc(p, out);
    else if (/\.(vue|ts)$/.test(e.name) && !/\.test\.ts$/.test(e.name) && !p.includes('i18n/locales')) out.push(p);
  }
  return out;
};
const dangling = [];
let refs = 0;
for (const f of walkSrc('src')) {
  const t = fs.readFileSync(f, 'utf8');
  for (const m of t.matchAll(KEY_REF)) {
    const k = m[1];
    if (k.startsWith('server.')) continue;
    refs++;
    if (!zhKeys.has(k) || !enKeys.has(k)) {
      const side = [!zhKeys.has(k) && 'zh-CN', !enKeys.has(k) && 'en-US'].filter(Boolean).join('/');
      dangling.push(`${f}: ${k} 在 ${side} 里没有词条`);
    }
  }
}
if (dangling.length) {
  console.error(`✗ 引用了不存在的中文字典键 ${dangling.length} 处（界面会直接把键名印出来）：`);
  for (const d of dangling.slice(0, 20)) console.error('  ' + d);
  if (dangling.length > 20) console.error(`  … 另有 ${dangling.length - 20} 处`);
  process.exit(1);
}
const parity = [...zhKeys].filter((k) => !enKeys.has(k)).concat([...enKeys].filter((k) => !zhKeys.has(k)));
if (parity.length) {
  console.error(`✗ 两套语言包键集不一致（${parity.length} 个）：${parity.slice(0, 10).join(', ')}`);
  process.exit(1);
}
console.log(`ℹ i18n 键引用：静态引用 ${refs} 处全部有词条，zh/en 键集一致（各 ${zhKeys.size} 键）`);

/* ---- 模板裸文案：.vue 模板里看得见的中文必须来自语言包 ----
 * 为什么单独有这一台：i18n-msg 门禁原先只校验语言包编译与 $t() 键存在性，
 * 模板里直接写死的「允许/禁止」「您好，」没有任何一台会红，英文界面照样漏中文。
 * 只扫模板：script 块里的字符串有日志、类型名等大量合法中文，静态判不了；
 * 模板里的中文（含 :attr="…'中文'…" 表达式与原生属性值）几乎都是 UI 文案。
 * 英文裸文案噪音太大判不了，这台只钉中文。
 *
 * 棘轮模式（--update-baseline 重建）：存量在 scripts/i18n-bare-baseline.json 登记在册，
 * 门禁只红「不在基线里的新增」。每清掉一批存量就重建一次基线让它只减不增，
 * 存量清完后把本段改成硬判即可（基线为空时行为与硬判等价）。 */
const CJK_RE = /[\u3400-\u9fff\uf900-\ufaff]/;
const { createHash } = await import('node:crypto');
const hashOf = (s) => createHash('sha1').update(s).digest('hex').slice(0, 12);
const BASELINE_PATH = 'scripts/i18n-bare-baseline.json';
const UPDATE_BASELINE = process.argv.includes('--update-baseline');

const bareText = [];
for (const f of walkSrc('src').filter((p) => p.endsWith('.vue'))) {
  // 注释先删掉，用等量换行占位保持行号不漂移
  const src = fs.readFileSync(f, 'utf8').replace(/<!--[\s\S]*?-->/g, (m) => m.replace(/[^\n]/g, ''));
  const lines = src.split('\n');
  let block = null; // 当前所在的 <script>/<style> 块
  lines.forEach((line, i) => {
    if (block) {
      if (new RegExp(`</${block}\\s*>`).test(line)) block = null;
      return;
    }
    const open = line.match(/<(script|style)\b/);
    if (open) {
      const before = line.slice(0, open.index);
      if (CJK_RE.test(before)) bareText.push(`${f}:${i + 1} ${before.trim().slice(0, 60)}`);
      if (!/\/>\s*$/.test(line)) block = open[1];
      return;
    }
    if (CJK_RE.test(line)) bareText.push(`${f}:${i + 1} ${line.trim().slice(0, 60)}`);
  });
}

if (UPDATE_BASELINE) {
  const baseline = {};
  for (const b of bareText) baseline[hashOf(b)] = b;
  fs.writeFileSync(BASELINE_PATH, JSON.stringify(baseline, null, 2) + '\n');
  console.log(`ℹ 模板裸文案基线已重建：${Object.keys(baseline).length} 处 → ${BASELINE_PATH}`);
} else {
  const baseline = fs.existsSync(BASELINE_PATH) ? JSON.parse(fs.readFileSync(BASELINE_PATH, 'utf8')) : {};
  const fresh = bareText.filter((b) => !(hashOf(b) in baseline));
  if (fresh.length) {
    console.error(`✗ 模板裸文案新增（中文必须走语言包 $t，存量 ${bareText.length - fresh.length} 处已登记在基线）${fresh.length} 处：`);
    for (const b of fresh) console.error('  ' + b);
    process.exit(1);
  }
  console.log(`ℹ 模板裸文案：新增 0 处（基线存量 ${bareText.length} 处待清，清后用 --update-baseline 缩基线）`);
}
