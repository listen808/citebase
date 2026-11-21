// i18n 原子批次工装：抽取 → 用词典自动补英文 → 全部有解才写盘，否则整批回退。
//
// 为什么要有它：手工一批一批推，一旦中途断在"抽了 130 处但语言包还没合并"上，
// 仓库就是红的（i18n-sync 与 closure 两台账都拦着）。而这个脚本把"抽取 + 合并 + 校验"
// 绑成一次事务：**词典覆盖不到任何一条新串就 git checkout 回退全部改动**，
// 于是可以被反复调用，永远不会留下半成品。
//
// 词典从哪来：仓库里已经入库的 2000+ 个键本身就是最好的中英对照表 ——
// 反读 zh-CN / en-US 两套语言包，按"中文原文 → 英文译文"建表，跨文件复用的 UI 词
// （保存/删除/加载中/操作失败…）自动命中，只有真正的新句子会被挡下来要人写。
// 人写过的那部分进 scripts/i18n-glossary-extra.json，下一批直接复用，不必从头再译。
//
// 用法（web/ 下）：
//   node scripts/i18n-batch.mjs --ns=docUi src/components/apple/ApTable.vue ...
//   node scripts/i18n-batch.mjs --ns=docUi --glossary-only <files>   # 只报哪些串没词典
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';

const argv = process.argv.slice(2);
const nsArg = argv.find((a) => a.startsWith('--ns='));
const NS = nsArg ? nsArg.split('=')[1] : '';
const DRY = argv.includes('--glossary-only');
const files = argv.filter((a) => !a.startsWith('--'));
if (!NS || files.length === 0) {
  console.error('用法：node scripts/i18n-extract/batch --ns=<命名空间> <文件...> [--glossary-only]');
  process.exit(2);
}

const ENTITY = { '&#10;': '\n', '&#13;': '\r', '&nbsp;': ' ', '&#10003;': '✓', '&amp;': '&', '&lt;': '<', '&gt;': '>', '&quot;': '"' };
const decode = (s) => Object.entries(ENTITY).reduce((a, [k, v]) => a.split(k).join(v), s);
const short = (k) => k.split('.')[1];

// 源码里的字面量是**转义形态**（`\n` 是两个字符），语言包读出来的是**成品形态**（真换行）。
// 两头不归一就会：① 词典永远命中不了带换行的串（表现为"没译文"，其实是没查到）；
// ② 写盘时 q() 把反斜杠再转一次，locale 里出现 `\\n`，渲染出来就是一行字面的 \n ——
// 文档正文里的 AI 摘要前缀正是这种串，错了不会被任何类型检查发现。
const unesc = (s) => s.replace(/\\([\s\S])/g, (m, c) => (c === 'n' ? '\n' : c === 't' ? '\t' : c === 'r' ? '\r' : c));
const toValue = (v) => unesc(decode(v));

// 读语言包里所有叶子字符串对，建 zh → en 表。同一句中文出现多种译法时保留第一个并计数，
// 因为"同一个按钮在英文里有两种拼法"本身就是要暴露的问题。
function buildGlossary() {
  const read = (p) => fs.readFileSync(p, 'utf8');
  const zh = read('src/i18n/locales/zh-CN.ts').split('\n');
  const en = read('src/i18n/locales/en-US.ts').split('\n');
  const unq = (l) => {
    const m = l.match(/^ {4}([A-Za-z0-9_]+): (['`])((?:\\.|(?!\2).)*)\2,$/);
    return m ? { k: m[1], v: m[3].replace(/\\'/g, "'").replace(/\\n/g, '\n').replace(/\\\\/g, '\\') } : null;
  };
  const g = new Map();
  // 两套文件结构不同（各自一个文件），所以按"块名 + 键名"配对：这里用键名在同名块内配对。
  const enByBlock = new Map();
  let cur = '';
  for (const line of en) {
    const bm = line.match(/^ {2}([A-Za-z0-9_]+): \{/);
    if (bm) { cur = bm[1]; enByBlock.set(cur, new Map()); continue; }
    const e = unq(line);
    if (e && cur) enByBlock.get(cur).set(e.k, e.v);
  }
  cur = '';
  let collisions = 0;
  for (const line of zh) {
    const bm = line.match(/^ {2}([A-Za-z0-9_]+): \{/);
    if (bm) { cur = bm[1]; continue; }
    const e = unq(line);
    if (!e || !cur) continue;
    const tr = enByBlock.get(cur)?.get(e.k);
    if (!tr) continue;
    const key = decode(e.v);
    if (g.has(key) && g.get(key) !== tr) collisions++;
    else if (!g.has(key)) g.set(key, decode(tr));
  }
  return { glossary: g, collisions };
}

const { glossary, collisions } = buildGlossary();
// 人工补的对照表优先级更高：反读出来的第一版译法可能不如后来想清楚的措辞。
// 分片存（extra.json / extra2.json / …）：一批几百条，整文件重写容易在中间断掉。
const extras = fs
  .readdirSync('scripts')
  .filter((f) => /^i18n-glossary-extra\d*\.json$/.test(f))
  .sort();
for (const f of extras) {
  for (const [zh, en] of Object.entries(JSON.parse(fs.readFileSync('scripts/' + f, 'utf8')))) {
    glossary.set(decode(zh), decode(en));
  }
}

// 抽取：直接复用 i18n-extract.mjs（它会就地改文件并打印 pendingKeys）
// 回退靠**动手前的内容快照**，不靠 git checkout：新文件没进过索引，
// `git checkout -- <未跟踪文件>` 直接 pathspec 报错，于是"整批回退"变成"崩在半路"（实测踩到）。
const snapshot = new Map(files.map((f) => [f, fs.readFileSync(f, 'utf8')]));
const revert = () => {
  for (const [f, text] of snapshot) fs.writeFileSync(f, text, 'utf8');
};

let out;
try {
  out = execFileSync('node', ['scripts/i18n-extract.mjs', `--ns=${NS}`, ...files], { encoding: 'utf8' });
} catch (e) {
  console.error('抽取失败：', String(e).slice(0, 300));
  process.exit(1);
}
const result = JSON.parse(out);
const pending = result.pendingKeys;

const need = Object.entries(pending).filter(([, zh]) => !glossary.has(toValue(zh)));
// 消息格式违例：`@` 与 `{ }` 在 vue-i18n 的消息里是语法（`@:链接`、`{name}` 插值），
// 未转义的那条会在**渲染那一刻**编译失败、靠回退返回原文 —— 页面看不出坏、type-check 也不红。
// 这种串一律拦下来整批回退，交给人写成 `{'@'}` / `{'{'}`，或改成带参数的插值。
const UNSAFE_MSG = /[@{}]/;
// {p1} 这类具名插值是**合法消息**（抽取器自己生成的），判违例前先摘掉，
// 否则"整句带插值"这一大类会被自己的守卫全数拦死。
const stripOwnParams = (s) => s.replace(/\{p\d+\}/g, '');
const unsafe = Object.entries(pending).filter(([, zh]) => UNSAFE_MSG.test(stripOwnParams(zh)));
if (unsafe.length > 0) {
  revert();
  // 报出落在哪个文件，否则"整批回退了"没法继续。取串中间 8 个字面搜 ——
  // 抽出来的值可能带转义或首尾空白，整串 includes 经常匹配不上。
  const where = (v) => {
    const probe = v.replace(/\\[\s\S]/g, ' ').trim().slice(2, 10);
    return files.filter((f) => (probe ? fs.readFileSync(f, 'utf8').includes(probe) : false)).slice(0, 3);
  };
  console.log(JSON.stringify({
    ok: false, namespace: NS, reverted: files.length, reason: '消息语法违例（@ 或 {}），需人工转义成 {\'@\'} / \'{\'}\'',
    unsafe: unsafe.map(([k, v]) => ({ key: short(k), zh: v, files: where(v) })),
  }, null, 2));
  process.exit(4);
}
if (need.length > 0) {
  // 回退：git checkout 这批文件。语言包还没动过，所以只需要还原源码。
  revert();
  console.log(JSON.stringify({
    ok: false, namespace: NS, reverted: files.length,
    replaced_would_be: result.replaced, keys: Object.keys(pending).length,
    untranslated: need.length,
    glossary_size: glossary.size, glossary_zh_en_collisions: collisions,
    // 一次报全：这批清单的用途就是喂给 i18n-glossary-extra.json，截断会让人以为已经补完。
    // 带参消息附上 {pN} 对应的原表达式：不看它就没法把英文语序写对。
    sample_untranslated: need.map(([k, v]) => {
      const ps = result.pendingParams?.[k];
      return `${short(k)}=${decode(v)}${ps && ps.length ? `   [p: ${ps.join(' | ')}]` : ''}`;
    }),
  }, null, 2));
  process.exit(3);
}

if (DRY) {
  revert();
  console.log(JSON.stringify({ ok: true, dryRun: true, keys: Object.keys(pending).length, glossary_size: glossary.size }, null, 2));
  process.exit(0);
}

const q = (s) => `'${s.replace(/\\/g, '\\\\').replace(/'/g, "\\'").replace(/\n/g, '\\n')}'`;
for (const [p, which] of [['src/i18n/locales/zh-CN.ts', 'zh'], ['src/i18n/locales/en-US.ts', 'en']]) {
  let src = fs.readFileSync(p, 'utf8');
  if (src.includes(`\n  ${NS}: {`)) throw new Error(`${p} 里已经有 ${NS}`);
  const lines = Object.entries(pending).map(([k, v]) => {
    const val = which === 'zh' ? toValue(v) : glossary.get(toValue(v));
    if (val === undefined) throw new Error(`${p}: ${short(k)} 没有英文译文，词典校验被绕过`);
    return `    ${short(k)}: ${q(val)},`;
  });
  const block = `\n  ${NS}: {\n` + lines.join('\n') + '\n  },';
  const anchor = src.lastIndexOf('\n} as const;');
  src = src.slice(0, anchor) + block + src.slice(anchor);
  // 落盘后再验一次：`\\n` 在语言包里是"字面反斜杠 + n"，渲染出来就是一行 \n，
  // 而 type-check / eslint / 单测都不会发现 —— 只有人的眼睛在页面上才看得见。
  if (/\\\\n/.test(src)) throw new Error(`${p} 出现了双重转义的 \\n，写入路径有 bug`);
  fs.writeFileSync(p, src, 'utf8');
}
console.log(JSON.stringify({
  ok: true, namespace: NS, replaced: result.replaced, keys: Object.keys(pending).length,
  fromGlossary: Object.keys(pending).length, files: files.length,
  needsHumanEyes: result.needsHumanEyes.length,
}, null, 2));
