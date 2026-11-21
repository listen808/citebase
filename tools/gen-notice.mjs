#!/usr/bin/env node
/**
 * 生成根目录 NOTICE.md —— 第三方依赖的许可清单。
 *
 * 为什么要有这台：LICENSE 是本产品自己的 MIT，读者据此以为"全仓都是 MIT"。
 * 实际链接进产物的 Go 模块里有 Apache-2.0 与 BSD，它们各自要求"分发时保留版权声明"，
 * 而手抄的清单一定会过期 —— 加一个依赖没人会想起来改 NOTICE。所以清单由现场重算：
 * `node tools/gen-notice.mjs`（生成）+ `tools/repocheck/notice_test.go`（比对，缺一条就红）。
 *
 * 射程与口径（写在这里是因为它们会被追问）：
 *  - Go 侧只收**真的链接进产物**的模块（`go list -deps`），不收 module graph 里那些
 *    从未 import 的间接声明 —— 后者在 `go list -m all` 里有 219 条，前者 102 条，
 *    抄全部等于把没用的东西也声明成"我们分发了它"。
 *  - JS 侧只收 `web/package.json` 的**直接**依赖：前端产物由用户 `npm install` 现场解析，
 *    传递依赖由 npm 自己带 LICENSE 分发，不在本仓的声明义务面上。
 *  - 许可名由模块缓存里那份 LICENSE 文件的签名判出，不查网络、不猜 SPDX 数据库。
 *    判不出就写 `UNKNOWN` 并在末尾点名 —— 让"没读过许可证"这件事在文档上看得见，
 *    而不是被省略成一个看起来像合规的空位。
 */
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const ROOT = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const out = [];
const w = (s = '') => out.push(s);

/**
 * 读 LICENSE 正文判许可。
 *
 * **标题优先，正文兜底**，这条顺序是被实测教出来的：MPL-2.0 全文的 §10.2
 * "Copyleft License Compatibility" 逐条点名 "GNU Affero General Public License, version 3.0"
 * 与 "GNU General Public License, version 2.0" —— 按全文关键字匹配会把两个 MPL 模块
 * 误判成 AGPL，而 AGPL 在 MIT 产品里等于发布阻塞，误报的代价是把人吓去换依赖。
 * 所以 GPL/AGPL/LGPL 只认文档开头；MIT/BSD/Apache 这类靠惯用句式认，正文兜底。
 * BSD-3 与 BSD-2 只差一条 "Neither the name" 从句，必须在同一步里分。
 */
function classify(text) {
  const norm = text.replace(/\s+/g, ' ');
  const head = norm.slice(0, 300);
  const starts = (re) => re.test(head);
  const has = (re) => re.test(norm);

  if (starts(/GNU AFFERO GENERAL PUBLIC/i)) return 'AGPL-3.0';
  if (starts(/GNU LESSER GENERAL PUBLIC/i)) return 'LGPL';
  if (starts(/GNU GENERAL PUBLIC LICENSE/i)) return 'GPL';
  if (starts(/Mozilla Public License/i) || has(/^Mozilla Public License Version/m)) return 'MPL-2.0';
  if (starts(/Apache License Version 2\.0/i) || has(/Apache License\s+Version 2\.0/)) return 'Apache-2.0';
  if (starts(/MIT License|The MIT License/i)) return 'MIT';
  if (starts(/BSD/i)) return /Neither the name/i.test(norm) ? 'BSD-3-Clause' : 'BSD-2-Clause';
  if (starts(/ISC License/i)) return 'ISC';
  if (starts(/Boost Software License/i)) return 'BSL-1.0';
  if (starts(/Unlicense/i) || has(/unlicense\.org/i)) return 'Unlicense';

  // 兜底：没有标准标题的文件（很多仓库直接写"Copyright (c) X. Permission is hereby granted…"）
  if (has(/Permission is hereby granted, free of charge/i)) return 'MIT';
  if (has(/Redistribution and use in source and binary forms/i)) {
    return /Neither the name/i.test(norm) ? 'BSD-3-Clause' : 'BSD-2-Clause';
  }
  if (has(/Permission to use, copy, modify, and\/?or distribute/i)) return 'ISC';
  return 'UNKNOWN';
}

const LICENSE_RE = /^(license|licence|copying|ofl|patents?)([-._][a-z0-9]+)?$/i;

function readLicense(dir) {
  let names = [];
  try {
    names = fs.readdirSync(dir);
  } catch {
    return { file: null, kind: 'NO_SOURCE' };
  }
  const hit = names
    .filter((n) => LICENSE_RE.test(n) && fs.statSync(path.join(dir, n)).isFile())
    .sort((a, b) => a.length - b.length)[0];
  if (!hit) return { file: null, kind: 'UNKNOWN' };
  const text = fs.readFileSync(path.join(dir, hit), 'utf8').slice(0, 20000);
  return { file: hit, kind: classify(text) };
}

function go(cmd, args) {
  return execFileSync(cmd, args, { cwd: ROOT, encoding: 'utf8', maxBuffer: 64 << 20 });
}

// 1) 链接进产物的模块清单
const linked = new Map();
for (const line of go('go', ['list', '-deps', '-f', '{{if .Module}}{{.Module.Path}} {{.Module.Version}}{{end}}', './app/server', './tools/...'])
  .split('\n')) {
  const [p, v] = line.trim().split(/\s+/);
  if (!p || p === 'github.com/listen808/citebase' || !v) continue;
  linked.set(p, v);
}

// 2) 模块 → 缓存目录（同一趟 go list，避免逐个 fork 进程）
const dirs = new Map();
for (const line of go('go', ['list', '-m', '-f', '{{.Path}}|{{.Dir}}', 'all']).split('\n')) {
  const i = line.indexOf('|');
  if (i > 0) dirs.set(line.slice(0, i), line.slice(i + 1).trim());
}

const byLicense = new Map();
const unknown = [];
for (const [mod, ver] of [...linked].sort()) {
  const dir = dirs.get(mod);
  const { kind } = dir ? readLicense(dir) : { kind: 'NO_SOURCE' };
  const label = dir ? kind : 'NO_SOURCE';
  if (label === 'UNKNOWN' || label === 'NO_SOURCE') unknown.push(`${mod}@${ver} (${label})`);
  if (!byLicense.has(label)) byLicense.set(label, []);
  byLicense.get(label).push({ mod, ver });
}

// 3) 前端直接依赖：许可取各自 node_modules 里 package.json 的 license 字段
const pkgPath = path.join(ROOT, 'web', 'package.json');
const webPkg = JSON.parse(fs.readFileSync(pkgPath, 'utf8'));
const npmByLicense = new Map();
const npmUnknown = [];
for (const name of Object.keys(webPkg.dependencies || {}).sort()) {
  const manifest = path.join(ROOT, 'web', 'node_modules', name, 'package.json');
  let kind = 'NO_SOURCE';
  if (fs.existsSync(manifest)) {
    const j = JSON.parse(fs.readFileSync(manifest, 'utf8'));
    const field = typeof j.license === 'string' ? j.license : j.license && j.license.type;
    // `SEE LICENSE IN <file>` 不是许可名，是一句"自己去看文件" —— 去看。
    const see = typeof field === 'string' && field.match(/^SEE LICENSE IN (.+)$/i);
    if (see) {
      const target = path.join(ROOT, 'web', 'node_modules', name, see[1].trim());
      kind = fs.existsSync(target) ? classify(fs.readFileSync(target, 'utf8')) : 'UNKNOWN';
    } else if (typeof field === 'string') {
      kind = field;
    } else {
      kind = 'UNKNOWN';
    }
  }
  if (kind === 'UNKNOWN' || kind === 'NO_SOURCE') npmUnknown.push(`${name} (${kind})`);
  if (!npmByLicense.has(kind)) npmByLicense.set(kind, []);
  npmByLicense.get(kind).push(name);
}

// 4) 渲染
const COPYLEFT = ['GPL', 'AGPL-3.0', 'LGPL'];
const fmt = (s) => (s.length <= 60 ? s.join(', ') : s.map((x) => `- \`${x}\``).join('\n'));

w('# NOTICE');
w();
w('Third-party software components used by Citebase.');
w('Citebase itself is MIT-licensed (see [LICENSE](LICENSE)); that covers **our** code only.');
w('The components below keep their own licenses, which are reproduced by name here and shipped');
w('inside each dependency. **This file is generated — do not edit it by hand.**');
w();
w('```bash');
w('node tools/gen-notice.mjs   # rewrites NOTICE.md from the current build list');
w('go test ./tools/repocheck/ -run TestNoticeCoversBuildList   # fails if it drifts');
w('```');
w();
w(`Measured against HEAD on ${new Date().toISOString().slice(0, 10)}:`);
w(`${linked.size} Go modules actually linked into the binary (not the \`${dirs.size}\` entries in the`);
w('module graph), and the direct npm dependencies of the web app.');
w();
w('## Go dependencies');
w();
w('Scope: modules providing at least one package imported by `./app/server` or `./tools/...`.');
w('License is read from the dependency’s own LICENSE file in the module cache.');
w();
for (const kind of [...byLicense.keys()].sort()) {
  const list = byLicense.get(kind);
  w(`### ${kind} — ${list.length}`);
  w();
  w(fmt(list.map((x) => `${x.mod}@${x.ver}`)));
  w();
}

const flags = [...byLicense.keys()].filter((k) => COPYLEFT.includes(k));
if (flags.length) {
  w('## ⚠ Copyleft components present');
  w();
  w(`These are linked into the shipped binary: ${flags.join(', ')}. Distributing the combined`);
  w('work may require more than a notice — read each license before release.');
  w();
}

if (byLicense.has('MPL-2.0')) {
  w('## Note on MPL-2.0');
  w();
  w('MPL-2.0 is **file-level** copyleft: it can ship inside an MIT project, but any change to');
  w('those specific library files must itself be published under MPL-2.0, and the MPL text');
  w('must stay with them. Its own §10.2 additionally names GNU Affero GPL v3 and GPL v2 as');
  w('compatible licenses for those files — that text is why a keyword scan reports MPL as');
  w('AGPL, so treat anything in this repo that says otherwise as a scanner bug.');
  w();
}

if (unknown.length) {
  w('## Unresolved license names');
  w();
  w('No license file was read for these; they are listed rather than silently omitted:');
  w();
  w(fmt(unknown));
  w();
}

w('## Web (npm) direct dependencies');
w();
w('Scope: `dependencies` in `web/package.json`. Transitive npm packages are fetched and');
w('distributed by `npm install` itself, each with its own license file, and are therefore not');
w('declared here. License names come from each package manifest’s `license` field.');
w();
for (const kind of [...npmByLicense.keys()].sort()) {
  const list = npmByLicense.get(kind);
  w(`### ${kind} — ${list.length}`);
  w();
  w(fmt(list));
  w();
}

const machine = [...linked.entries()].sort().map(([p, v]) => `go\t${p}@${v}`);
const npmMachine = Object.keys(webPkg.dependencies || {}).sort().map((n) => `npm\t${n}`);
w('<!-- machine-readable coverage list; tools/repocheck/notice_test.go compares this block');
w('     against `go list -deps` and web/package.json. Keep it last. -->');
w('```');
w([...machine, ...npmMachine].join('\n'));
w('```');
w();

fs.writeFileSync(path.join(ROOT, 'NOTICE.md'), out.join('\n'), 'utf8');
console.log(`NOTICE.md 已生成：Go ${linked.size} 个模块 / npm ${npmMachine.length} 个直接依赖`);
if (flags.length) console.log(`注意：存在 copyleft 依赖 ${flags.join(', ')}`);
if (unknown.length) console.log(`未判出许可名：${unknown.length} 个`);
