#!/usr/bin/env node
// audit-api.mjs — deterministic backend route <-> frontend API call inventory + diff
import fs from 'node:fs';
import path from 'node:path';

const ROOT = process.cwd();
const OUT = path.join(ROOT, 'tools/audit-out');
fs.mkdirSync(OUT, { recursive: true });
const METHODS = ['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS', 'Any'];
const REL = (p) => path.relative(ROOT, p).replace(/\\/g, '/');

function walk(dir, ok) {
  const out = [];
  if (!fs.existsSync(dir)) return out;
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    if (['node_modules', '.git', 'dist', '.nuxt'].includes(e.name)) continue;
    const p = path.join(dir, e.name);
    if (e.isDirectory()) out.push(...walk(p, ok));
    else if (ok(e.name)) out.push(p);
  }
  return out;
}

/* ---------------- backend ---------------- */
const goFiles = walk(path.join(ROOT, 'internal', 'api'), (n) => n.endsWith('.go') && !n.endsWith('_test.go'))
  .concat([path.join(ROOT, 'app', 'server', 'server.go')]);

// var -> prefix, scoped by file; '*' = defaults for well-known param names
const prefixes = new Map();
const put = (file, v, p) => prefixes.set(`${file}::${v}`, p);
const get = (file, v) => prefixes.get(`${file}::${v}`) ?? prefixes.get(`*::${v}`) ?? null;

put('*', 'r', ''); put('*', 'engine', ''); put('*', 'v1', '/api/v1');
put(path.join(ROOT, 'internal/api/admin/api_key.go'), 'open', '/api/v1/open');

const groupRe = /([A-Za-z_]\w*)\s*(?::?=|,)\s*([A-Za-z_]\w*)\.Group\(\s*"([^"]*)"/g;
const routeRe = new RegExp(`([A-Za-z_]\\w*)\\.(${METHODS.join('|')})\\(\\s*"([^"]*)"`, 'g');

for (const file of goFiles) {
  const src = fs.readFileSync(file, 'utf8');
  let m;
  groupRe.lastIndex = 0;
  while ((m = groupRe.exec(src))) {
    const [, child, parent, pfx] = m;
    const base = get(file, parent) ?? get('*', parent);
    put(file, child, base === null ? `UNRESOLVED(${parent})` : base + pfx);
  }
}

const backend = [];
const unresolvedReceivers = new Set();
for (const file of goFiles) {
  const lines = fs.readFileSync(file, 'utf8').split('\n');
  for (let i = 0; i < lines.length; i++) {
    const line = lines[i];
    if (/^\s*(\/\/|\*)/.test(line)) continue;
    let m;
    routeRe.lastIndex = 0;
    while ((m = routeRe.exec(line))) {
      const [, recv, method, p] = m;
      const base = get(file, recv) ?? get('*', recv);
      if (base === null) { unresolvedReceivers.add(`${REL(file)}:${recv}`); continue; }
      const esc = p.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
      const handler = (line.match(new RegExp(`${method}\\(\\s*"${esc}"[^,]*,\\s*(?:[A-Za-z_]\\w*\\.)*([A-Za-z_]\\w*)`)) || [])[1] || '';
      backend.push({ method, path: base + p, file: REL(file), line: i + 1, handler });
    }
  }
}

/* ---------------- frontend ---------------- */
const feFiles = walk(path.join(ROOT, 'web', 'src'), (n) => (n.endsWith('.ts') || n.endsWith('.vue')) && !n.endsWith('.d.ts'));
const helperRe = /(?<![\w.$])(get|post|put|del|patch)\s*(?:<[^<>]*(?:<[^<>]*>[^<>]*)*>|<[^<>]*>)?\s*\(\s*['"`]([^'"`]+)['"`]/g;
const clientRe = /\b(?:client|axios|instance)\.(get|post|put|delete|patch)\s*(?:<[^<>]*(?:<[^<>]*>[^<>]*)*>|<[^<>]*>)?\s*\(\s*['"`]([^'"`]+)['"`]/g;
const fetchRe = /\bfetch\s*\(\s*['"`]([^'"`]+)['"`]/g;
const litRe = /['"`](\/[A-Za-z0-9_\-/{}.$]+)['"`]/g;

function normUrl(raw) {
  return urlVariants(raw)[0];
}
// A template like `/metrics${query}` (placeholder glued to a segment, i.e. a query string)
// must also be probed as the bare `/metrics`, else it reads as an unmatched path.
function urlVariants(raw) {
  let u = raw.split('#')[0];
  const qi = u.indexOf('?');
  const cut = u.indexOf('${');
  const glued = cut > 0 && u[cut - 1] !== '/';
  u = qi >= 0 ? u.slice(0, qi) : u;
  const full = fix(u.replace(/\\\$\{[^}]*\}/g, '*').replace(/\$\{[^}]*\}/g, '*').replace(/\/+$/, ''));
  const out = [full];
  if (glued) {
    const base = fix(u.slice(0, cut).replace(/\/+$/, ''));
    if (base && base !== full) out.push(base);
  }
  return out;
}
function fix(u) {
  if (u.startsWith('/api/v1')) u = u.slice(7);
  if (u.startsWith('/api/')) return 'ABS:' + u;
  if (/^https?:/.test(u)) return 'EXT:' + u;
  return u.startsWith('/') ? u : '/' + u;
}

const fe = [];
const feLiterals = new Set();
const lineOf = (src, idx) => src.slice(0, idx).split('\n').length;
for (const file of feFiles) {
  const src = fs.readFileSync(file, 'utf8');
  // blank out comments (keep line offsets) so commented-out calls don't count as live
  const code = src
    .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
    .replace(/(^|[^:\\'"])\/\/[^\n]*/g, '$1//');
  let m;
  for (const [re, methKey] of [[helperRe, 1], [clientRe, 1], [fetchRe, 0]]) {
    re.lastIndex = 0;
    while ((m = re.exec(code))) {
      const method = methKey ? { GET: 'GET', POST: 'POST', PUT: 'PUT', DEL: 'DELETE', DELETE: 'DELETE', PATCH: 'PATCH' }[m[1].toUpperCase()] : 'GET';
      if (!method) continue;
      const urlRaw = methKey ? m[2] : m[1];
      const urls = urlVariants(urlRaw);
      fe.push({ method, url: urls[0], urls, raw: urlRaw, file: REL(file), line: lineOf(code, m.index) });
    }
  }
  litRe.lastIndex = 0;
  while ((m = litRe.exec(code))) for (const u of urlVariants(m[1])) feLiterals.add(u);
}

/* ---------------- match ---------------- */
function toRegex(ginPath) {
  const clean = ('/' + ginPath.replace(/^\/+/, '').replace(/\/+$/, '')) || '/';
  let esc = '';
  for (const seg of clean.slice(1).split('/')) {
    if (!seg) continue;
    esc += '/' + (seg.startsWith(':') ? '[^/]+' : seg.replace(/[.*+?^${}()|[\]\\]/g, '\\$&'));
  }
  // wildcards in gin paths (*filepath)
  esc = esc.replace(/\/\*/g, '/.*');
  return new RegExp(`^${esc || '/'}/?$`);
}
for (const b of backend) { b.rel = b.path.replace(/^\/api\/v1/, '') || '/'; b.re = toRegex(b.rel); }

const strip = (u) => (u.startsWith('ABS:') ? u.slice(4).replace(/^\/api\/v1/, '') : u);
const unmatched = [], matched = [];
for (const c of fe) {
  if (c.url.startsWith('EXT:')) continue;
  const hits = backend.filter((b) => b.method === c.method && c.urls.some((p) => b.re.test(strip(p))));
  if (hits.length) matched.push({ ...c, hits: [...new Set(hits.map((h) => `${h.method} ${h.path}`))] });
  else unmatched.push(c);
}

const key = (x) => `${x.method} ${x.url ?? x.path}`;
const uniq = (arr) => { const s = new Map(); for (const x of arr) if (!s.has(key(x))) s.set(key(x), x); return [...s.values()].sort((a, b) => key(a).localeCompare(key(b))); };

const feUnique = uniq(fe);
const bePaths = new Set(backend.map((b) => `${b.method} ${b.path}`));

// reverse: backend endpoints never called by frontend
const called = (b) => fe.some((c) => c.method === b.method && c.urls.some((u) => b.re.test(strip(u))));
const feShapes = new Set([...feLiterals].map((u) => u.replace(/:[^/]+/g, '*')));
const feText = feFiles.map((f) => fs.readFileSync(f, 'utf8')).join('\n');
// soft == the path's distinctive segment does appear somewhere in web/src, so the call may be
// built dynamically; hard == no shape and no trace of the segment name at all.
const weakRef = (b) => {
  if (feShapes.has(b.rel.replace(/:[^/]+/g, '*'))) return true;
  const seg = b.rel.replace(/^\/+/, '').split('/').filter((s) => s && !s.startsWith(':'));
  const distinctive = seg.find((s) => /[a-z]/.test(s) && !['api', 'v1', 'ai', 'id'].includes(s)) ?? seg[0];
  return distinctive ? feText.includes(distinctive) : false;
};
const neverCalled = backend.filter((b) => !called(b));
const hard = neverCalled.filter((b) => !weakRef(b));

const soft = neverCalled.filter((b) => weakRef(b));
const report = {
  counts: { backend_routes: backend.length, backend_unique: bePaths.size, frontend_calls: fe.length, frontend_unique: feUnique.length, unmatched_frontend: uniq(unmatched).length, never_called_backend: uniq(neverCalled).length, unreached_hard: uniq(hard).length, unreached_soft: uniq(soft).length },
  unresolvedReceivers: [...unresolvedReceivers],
  unmatched_frontend: uniq(unmatched),
  never_called_backend: uniq(neverCalled),
  unreached_hard: uniq(hard),
  unreached_soft: uniq(soft),
};
fs.writeFileSync(path.join(OUT, 'api-report.json'), JSON.stringify(report, null, 2));
fs.writeFileSync(path.join(OUT, 'backend-routes.txt'), [...bePaths].sort().join('\n'));
fs.writeFileSync(path.join(OUT, 'frontend-calls.txt'), feUnique.map((c) => `${c.method} ${c.url}\t${c.file}:${c.line}`).join('\n'));

console.log(`backend routes : ${backend.length} (${bePaths.size} unique)`);
console.log(`frontend calls : ${fe.length} (${feUnique.length} unique)`);
console.log(`unresolved receivers: ${report.unresolvedReceivers.join(', ') || 'none'}`);
console.log(`\nFRONTEND -> NO BACKEND MATCH (${report.counts.unmatched_frontend}):`);
for (const u of report.unmatched_frontend) console.log(`  ${u.method.padEnd(6)} ${u.url}\t${u.file}:${u.line}`);
console.log(`\nBACKEND NOT CALLED BY FRONTEND (${report.counts.never_called_backend}) = hard ${report.counts.unreached_hard} + soft ${report.counts.unreached_soft}`);
const grp = {};
for (const n of report.unreached_hard) { const s = '/' + (n.path.replace(/^\/api\/v1\/?/, '').split('/')[0] || ''); grp[s] = (grp[s] || 0) + 1; }
console.log('hard 按一级路径: ' + Object.entries(grp).sort((a, b) => b[1] - a[1]).map(([k, v]) => `${k}(${v})`).join(' '));
for (const n of report.unreached_hard) console.log(`  ${n.method.padEnd(6)} ${n.path}\t${n.file}:${n.line} ${n.handler}`);
