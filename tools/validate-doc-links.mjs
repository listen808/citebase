#!/usr/bin/env node
/**
 * 检查 markdown 里的相对链接是否指向真实存在的文件。
 *
 * 为什么需要它：commit b0a5e8aa（"清理"，2026-09-29）一次删掉 docs/ 下 12 份文档，
 * 而 README.md 与 docs/README.md 的索引照旧指着其中 6 份 —— 直到 10-06 才发现。
 * 这不是"文档写得不好"，是**文档指路指到不存在的文件**：
 * AGENTS.md 当时还写着"读这两份再动代码"（read both before non-trivial changes），
 * 于是每个按说明办事的人和 AI 协作者都在找一个扑空的路径，
 * 而那 1,300 行内容看起来就像"仓库里少了东西"而不是"索引没跟着删"。
 *
 * 判据只有一条：链接去掉 #锚点 后，相对本文件的位置必须存在。
 * 外部链接（http/mailto）与纯锚点跳过；docs/swagger/ 是 `make swagger` 的产出物，
 * 里面是生成文件，不纳入检查（也不该由这里管）。
 *
 * 用法：make check-docs   或   node tools/validate-doc-links.mjs
 */
import { readdirSync, readFileSync, statSync, existsSync } from 'node:fs';
import { join, dirname, relative } from 'node:path';

const ROOT = process.cwd();
const SKIP_DIRS = new Set(['node_modules', '.git', 'dist', 'data', '.rv-tmp']);
// 生成物目录：内容不是手工维护的，里面的链接由生成器负责。
const SKIP_PATHS = ['docs/swagger'];

function walk(dir, out = []) {
  for (const name of readdirSync(dir)) {
    const p = join(dir, name);
    const rel = relative(ROOT, p).replace(/\\/g, '/');
    if (SKIP_DIRS.has(name)) continue;
    if (SKIP_PATHS.some((sp) => rel === sp || rel.startsWith(sp + '/'))) continue;
    const st = statSync(p);
    if (st.isDirectory()) walk(p, out);
    else if (name.endsWith('.md')) out.push(p);
  }
  return out;
}

// [text](target "可选 title")：先切掉尾部 title，再剥 #锚点与 ?查询。
const LINK_RE = /\[[^\]]*]\(([^)\s]+)(?:\s+"[^"]*")?\)/g;

const files = walk(ROOT);
const broken = [];
let checked = 0;

for (const file of files) {
  const rel = relative(ROOT, file).replace(/\\/g, '/');
  const text = readFileSync(file, 'utf8');
  for (const m of text.matchAll(LINK_RE)) {
    let target = m[1];
    if (/^(https?:|mailto:|data:)/.test(target)) continue;
    target = target.split('#')[0].split('?')[0];
    if (!target) continue; // 纯页内锚点
    checked++;
    // 相对当前文件解析；以 / 开头视为仓库根（本仓没这么写，但别误报）
    const abs = target.startsWith('/')
      ? join(ROOT, target.slice(1))
      : join(dirname(file), target);
    if (!existsSync(abs)) broken.push({ rel, target });
  }
}

console.log(`文档链接检查：${files.length} 个 markdown 文件，${checked} 条相对链接`);
if (broken.length) {
  console.error(`\n${broken.length} 条链接指向不存在的文件：`);
  for (const b of broken) console.error(`  ${b.rel} -> ${b.target}`);
  console.error('\n删文档要连着改索引：要么把文件补回来，要么把这条链接改掉。');
  process.exit(1);
}
console.log('全部相对链接都能落到真实文件。');
