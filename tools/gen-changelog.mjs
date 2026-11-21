#!/usr/bin/env node
// 从 git log 生成 CHANGELOG.md。
//
// 为什么是生成而不是手写：这个仓库在此之前**根本没有 CHANGELOG**，
// 而手写 154 个提交的历史会在下一次发布时立刻过期（过期的一次性文档比没有更糟，
// 因为它读起来像权威）。所以这里只解决"从 v1.0.0 起的历史"这一段，
// 之后的 release 仍然按 Conventional Commits 的既有约定重新生成。
//
// 判据说明（写在文件里，免得下一个人以为是随手写的）：
// - 只认 Conventional Commits 的 type(scope)!: subject 头部；不符合的提交归进
//   "其他"而不是丢掉 —— 丢掉会让"这份清单是完整的"变成假话。
// 判据要求冒号：Conventional Commits 的正文标记是 `BREAKING CHANGE: <说明>` 这一行。
// 只匹配 "BREAKING CHANGE" 四个字会被**谈到它的文字**冒充 —— 实测本仓库第一条就是这么来的：
// 生成器自己的提交说明写了"正文里的 BREAKING CHANGE"，于是那条提交被列进"破坏性变更"一节。
// 必须是**行首**的 git trailer：`BREAKING CHANGE: <说明>` 单独成行。
// 只要求冒号还不够 —— 本仓库第二次踩同一坑：提交说明里写了一句
// "没一条按约定打了 `!` / `BREAKING CHANGE:`"，于是那条提交自己被列进"破坏性变更"。
// 谈到这个标记 ≠ 使用这个标记。
const breakingBodyRe = /^BREAKING[ -]CHANGE:[ 	]*\S/m;
// - merge 提交不进清单（它们是"什么时候合的"，不是"改了什么"）。

import { execFileSync } from 'node:child_process';
import { writeFileSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const OUT = path.join(ROOT, 'CHANGELOG.md');

const RANGE = process.argv[2] || 'v1.0.0..HEAD';
const SECTIONS = [
  ['feat', '新增'],
  ['fix', '修复'],
  ['perf', '性能'],
  ['refactor', '重构'],
  ['revert', '回退'],
  ['docs', '文档'],
  ['style', '格式化（ESLint / gofmt / prettier，不改行为）'],
  ['test', '测试'],
  ['build', '构建'],
  ['ci', 'CI'],
  ['chore', '杂项'],
];

const log = execFileSync(
  'git',
  ['log', RANGE, '--no-merges', '--date=short', '--pretty=format:%H\x1f%ad\x1f%s\x1f%b\x1e'],
  { cwd: ROOT, encoding: 'utf8' }
)
  .split('\x1e')
  .map((raw) => raw.trim())
  .filter(Boolean)
  .map((raw) => {
    const [hash, date, subject = '', body = ''] = raw.split('\x1f');
    return { hash: (hash || '').slice(0, 8), date, subject: subject.trim(), body: body.trim() };
  });

// 判据的类型表只有一份：从 SECTIONS 派生。手写第二份枚举就是"两张平行的表"，
// 加一档时必然漏改一处，漏掉的那档会安静地落进"其他"。
const header = new RegExp(
  '^(' + SECTIONS.map(([k]) => k).join('|') + ')(\\([^)]*\\))?!?:\\s*'
);
const groups = new Map(SECTIONS.map(([k, v]) => [k, { title: v, items: [] }]));
const other = { title: '其他（提交标题不符合 Conventional Commits）', items: [] };
const breaking = { title: '破坏性变更', items: [] };

for (const c of log) {
  const m = header.exec(c.subject);
  if (!m) {
    other.items.push(c);
    if (breakingBodyRe.test(c.body)) breaking.items.push({ ...c, why: '正文标注 BREAKING CHANGE:' });
    continue;
  }
  const type = m[1];
  (groups.get(type) || other).items.push(c);
  // `!` 在标题上，或正文里写了 BREAKING CHANGE，两条都算
  if (c.subject[m[0].length - 1] === '!' || breakingBodyRe.test(c.body)) {
    breaking.items.push({ ...c, why: m[0].includes('!') ? '标题带 `!`' : '正文标注 BREAKING CHANGE' });
  }
}

const fmt = (c) => `- ${c.subject}（${c.date} · \`${c.hash}\`）`;

const lines = [
  '# 更新日志',
  '',
  '> 本文件由 `node tools/gen-changelog.mjs` 从 git log 生成，**不要手工编辑**——',
  '> 手写的条目会在下一次发布时被整体覆盖。要解释"为什么这么改"请写进提交信息或',
  '> [`docs/decision-log/`](docs/decision-log) 的决策记录，那里才是不会过期的地方。',
  '>',
  `> 覆盖范围：\`${RANGE}\`，共 ${log.length} 个非 merge 提交。`,
  '',
];

if (breaking.items.length) {
  lines.push('## 破坏性变更', '');
  lines.push('先单列一节：升级前必须先看这里，而不是在功能清单里找。', '');
  for (const c of breaking.items) lines.push(`- ${c.subject} — ${c.why}（${c.date} · \`${c.hash}\`）`);
  lines.push('');
}

for (const [, group] of groups) {
  if (!group.items.length) continue;
  lines.push(`## ${group.title}`, '');
  for (const c of group.items) lines.push(fmt(c));
  lines.push('');
}

if (other.items.length) {
  lines.push(`## ${other.title}`, '');
  for (const c of other.items) lines.push(fmt(c));
  lines.push('');
}

writeFileSync(OUT, lines.join('\n'), 'utf8');

const total = [...groups.values()].reduce((a, g) => a + g.items.length, 0) + other.items.length;
console.log(`CHANGELOG.md 已生成：${total} 条（破坏性变更 ${breaking.items.length} 条，其他 ${other.items.length} 条）`);
if (total !== log.length) {
  console.error(`✗ 分类后条数 ${total} 与提交数 ${log.length} 不一致 —— 有提交在归类时丢了`);
  process.exit(1);
}
