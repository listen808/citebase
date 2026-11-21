#!/usr/bin/env node
// 前端 API 层死导出检测 + 分流。
//
// 为什么要分流：一个"从未被调用"的导出不等于"该删"，也不等于"该接线"。
//   · 同 URL 已有一个**在用**的导出覆盖 → 它是重复包装，该删；硬接上去只会做出第二套实现
//   · 没有任何在用的同 URL 导出 → 后端这条能力在 UI 上真的没有入口，该接
// 早期版本只按"没人引用"就开清单，差点删掉 `reindexVectors`——它是规范实现，
// 被同文件的 3 个 @deprecated 别名调用着，而那 3 个别名正在被页面使用。
// 所以：同文件内的引用必须算数，且匹配前要先剥掉注释，否则
// `/** @deprecated 请使用 foo */` 里的 foo 会被当成一次调用。
//
// 用法：
//   node web/scripts/check-dead-api-exports.mjs            # 人读的分流报告
//   node web/scripts/check-dead-api-exports.mjs --json     # 机器可读
//   node web/scripts/check-dead-api-exports.mjs --deletable # 只列可删的 name@file
import { readdirSync, readFileSync, statSync } from 'fs';
import { join, sep } from 'path';

const files = [];
(function walk(d) {
  for (const e of readdirSync(d)) {
    const p = join(d, e);
    const s = statSync(p);
    if (s.isDirectory()) walk(p);
    else if (/\.(ts|vue)$/.test(e) && !/[\\/](node_modules|dist)[\\/]/.test(p)) files.push(p.split(sep).join('/'));
  }
})('src');

// 去掉注释，否则 @deprecated 提示里提到的函数名会被算成一次调用
function stripComments(src) {
  return src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|\s)\/\/[^\n]*/g, '$1');
}

const isApi = (f) => f.includes('src/api/modules/');
const texts = new Map();
for (const f of files) {
  const raw = readFileSync(f, 'utf8');
  // 只对 .ts 剥注释。.vue 里有 `<template>` 文本与内联样式，`/* … */` 会跨过整段
  // 模板把真实调用一起吃掉（AIImportView.vue 17799 → 8038 字符，`importFile` 的两个
  // 引用全被吞掉，于是被误报成「无 UI 入口」）。.vue 的注释里几乎不会出现 API 名，
  // 保留原文的误判代价远小于误删。
  texts.set(f, f.endsWith('.vue') ? raw : stripComments(raw));
}
const apiFiles = files.filter((f) => isApi(f) && !f.endsWith('.test.ts') && !f.endsWith('.d.ts'));

// 提取一个导出函数实际打的请求：method + 归一化路径。
// 不能用 `<[^()]*>` 去跨泛型：`get<PaginationResponse<SchemaSpace>>('/x')` 会停在
// 第一个 `>` 上，端点整个提不出来（第一版就这样把 schemaApi 的成员全判成"无入口"）。
// 改成：定位动词 → 取它后面第一个以 / 开头的字符串字面量。
const VERB_RE = /\b(get|post|put|del|patch)\b/;
const URL_RE = /['`]([^'`]*\/[^'`]*)['`]/;
function endpointOf(body) {
  const vm = body.match(VERB_RE);
  if (!vm) return null;
  const rest = body.slice(vm.index + vm[0].length, vm.index + 600);
  const sm = rest.match(URL_RE);
  if (!sm) return null;
  let path = sm[1].replace(/\$\{[^}]*\}/g, '*');
  if (path.endsWith('/')) path += '*';
  return vm[1].toUpperCase() + ' ' + path;
}

// 逐个导出：收集引用数（同文件内 −1 为自身声明）与它打的请求。
// 一律在**剥掉注释**的文本上切片，否则注释里的示例 URL 会被当成真实请求。
const entries = [];
for (const af of apiFiles) {
  const clean = texts.get(af);
  const declRe = /export\s+(?:async\s+)?(function|const)\s+(\w+)/g;
  const seen = new Set();
  for (const m of [...clean.matchAll(declRe)]) {
    const name = m[2];
    if (seen.has(name)) continue;
    seen.add(name);
    const at = m.index;
    const rest = clean.slice(at + m[0].length);
    const nextDecl = rest.search(/\nexport\s+(?:async\s+)?(?:function|const|interface|type|class)\s/);
    const body = rest.slice(0, nextDecl < 0 ? rest.length : nextDecl);
    const selfUses = [...clean.matchAll(new RegExp('\\b' + name + '\\b', 'g'))].length - 1;
    const usedElsewhere = files.some((o) => o !== af && new RegExp('\\b' + name + '\\b').test(texts.get(o)));
    entries.push({ name, file: af, used: selfUses > 0 || usedElsewhere, ep: endpointOf(body), indirect: !endpointOf(body) });
  }
}

const dead = entries.filter((e) => !e.used);

// 对象式导出（`export const schemaApi = { listSpaces(){...}, askQuestion(){...} }`）
// 必须逐个成员解析：第一版只认 `export function`，于是 AISchemaView 明明在用
// `schemaApi.listSpaces`，这堆 URL 却全部显示成"后端能力无 UI 入口"——
// 照那份清单去接线，就会在既有实现旁边做出第二套。
// 成员是否在用，看 `NAME.member` 这个整体有没有出现在任何文件里（对象字面量自身
// 不含这种写法，所以无需排除声明处）。
const memberEndpoints = [];
for (const af of apiFiles) {
  const raw = texts.get(af);
  for (const m of raw.matchAll(/export\s+const\s+(\w+)\s*=\s*\{/g)) {
    const objName = m[1];
    const start = m.index + m[0].length;
    let depth = 1;
    let i = start;
    while (i < raw.length && depth > 0) {
      if (raw[i] === '{') depth++;
      else if (raw[i] === '}') depth--;
      i++;
    }
    const body = raw.slice(start, i);
    // 只认对象**顶层**成员：早先按正则在整段 body 上扫，`listSpaces(page = 1, pageSize = 20)`
    // 签名里的 `page =` 也被当成一个成员，于是把上一个成员的内容切在 return 之前，
    // 端点整个丢失。这里先算出每个字符处的括号深度，只接受深度 0 的匹配。
    const depthAt = new Int32Array(body.length + 1);
    let d = 0;
    for (let k = 0; k < body.length; k++) {
      const ch = body[k];
      if (ch === '{' || ch === '(' || ch === '[') d++;
      else if (ch === '}' || ch === ')' || ch === ']') d--;
      depthAt[k] = d;
    }
    const marks = [];
    for (const mm of body.matchAll(/(?:^|[,{\n])\s*(\w+)\s*(?::\s*)?(?:async\s*)?(?:\(|=)/g)) {
      if (depthAt[mm.index] !== 0) continue;
      marks.push({ member: mm[1], start: mm.index, from: mm.index + mm[0].length });
    }
    marks.forEach((mk, idx) => {
      const next = marks[idx + 1];
      const memberBody = body.slice(mk.from, next ? next.start : body.length);
      const ep = endpointOf(memberBody);
      if (!ep) return;
      const used = files.some((o) => new RegExp('\\b' + objName + '\\.' + mk.member + '\\b').test(texts.get(o)));
      memberEndpoints.push({ objName, member: mk.member, ep, used });
    });
  }
}

// 同一 endpoint 是否有"在用"的导出或对象成员覆盖
const liveByEndpoint = new Map();
const addLive = (ep, label) => {
  if (!liveByEndpoint.has(ep)) liveByEndpoint.set(ep, []);
  liveByEndpoint.get(ep).push(label);
};
for (const e of entries) if (e.used && e.ep) addLive(e.ep, e.name + ' @' + e.file);
for (const me of memberEndpoints) if (me.used) addLive(me.ep, me.objName + '.' + me.member + ' @api-object');

const duplicates = [];
const unreached = [];
for (const d of dead) {
  const twins = d.ep ? liveByEndpoint.get(d.ep) : null;
  if (twins && twins.length) duplicates.push({ ...d, twins });
  else unreached.push(d);
}

if (process.argv.includes('--deletable')) {
  for (const d of duplicates) console.log(`${d.name}@${d.file}`);
  process.exit(0);
}

const byFile = (arr) => {
  const o = {};
  for (const d of arr) o[d.file] = (o[d.file] || 0) + 1;
  return Object.entries(o).sort((a, b) => b[1] - a[1]);
};

if (process.argv.includes('--json')) {
  console.log(JSON.stringify({ total: entries.length, dead: dead.length, duplicates, unreached }, null, 1));
  process.exit(0);
}

console.log('api module files :', apiFiles.length);
console.log('value exports    :', entries.length);
console.log('never called     :', dead.length, '(' + ((dead.length / entries.length) * 100).toFixed(1) + '%)');
console.log('');
console.log(`A. 重复包装 —— 同 URL 已有在用导出覆盖，应删：${duplicates.length}`);
for (const [f, n] of byFile(duplicates)) console.log(`   ${String(n).padStart(2)}  ${f}`);
console.log('');
console.log(`B. 后端能力无 UI 入口 —— 应接线：${unreached.length}`);
for (const [f, n] of byFile(unreached)) console.log(`   ${String(n).padStart(2)}  ${f}`);
console.log('');
console.log('--- A 明细 ---');
for (const d of duplicates) console.log(`  [${d.ep}] ${d.name}  (${d.file})  ← 在用的: ${d.twins.join(', ')}`);
console.log('');
console.log('--- B 明细 ---');
for (const d of unreached) console.log(`  [${d.ep || '间接调用/未解析'}] ${d.name}  (${d.file})`);
