// 路由清单的唯一来源仍是 visual-snapshot.mjs 里那份字面量（它带 public 标记，
// 巡检要按登录态分批）。这里只负责"把它读出来并展开成待测列表"——
// 否则每加一个工装就再抄一遍正则，两份慢慢量的就不是同一批页了。
//
// 夹具 id 一律现场解析，不许写死：写死的那三个（/space/1/doc/356、/user/9、/roles/1/users）
// 早就随一轮清库变成 404，而 parity 把这三页报成"整页没有可见页头 h1"——
// 判据没错，错在被测对象根本不存在。**假夹具比没有夹具更坏：它会指挥人去修一个不存在的问题。**
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));

/** 夹具清单：路径里出现这些占位符，loadTargets / resolveTargets 会用真实 id 替换 */
const FIXTURE_KEYS = ['space', 'doc', 'user', 'role'];

function apiBase() {
  return (process.env.SNAP_BASE || 'http://localhost:3000').replace(/\/$/, '');
}

const TOKEN_CACHE = path.join(os.tmpdir(), 'citebase-inspect-token.json');

// 登录那一档是 10 次/分钟/IP 的严格限流（internal/middleware/rateLimit 里 strictLimiter）。
// 12 台工装每台既要在浏览器里登录、又要在这里登录一次，串着跑没事、并排跑就会把自己锁在门外 ——
// 撞上的表现还是一片 429，看不出是限流还是应用坏了。所以 token 复用五分钟。
async function cachedToken(base) {
  let token = '';
  try {
    const cached = JSON.parse(fs.readFileSync(TOKEN_CACHE, 'utf8'));
    if (cached && cached.token && Date.now() - cached.at < 5 * 60 * 1000) token = cached.token;
  } catch {
    // 缓存读不出来就当没有：首次运行、被别的用户跑过、或者文件被清 —— 都不该拦下巡检
  }
  if (token && (await ping(base, token))) return token;
  return login(base);
}

async function ping(base, token) {
  try {
    const res = await fetchWithRetry(`${base}/api/v1/spaces/my?page=1&page_size=1`, { headers: auth(token) });
    // 429 不能当成"token 失效"：那会把一个好 token 换掉，还顺手多打一次登录
    if (res.status === 429) return true;
    return res.ok;
  } catch {
    return false;
  }
}

function auth(token) {
  return { Authorization: `Bearer ${token}`, 'Content-Type': 'application/json' };
}

// Node 24 + Windows 上 fetch 偶发直接抛（进程还会在退出时撞 libuv 的 async.c 断言）。
// 后端正在重启或瞬时拒连时，工装该看到一句"连不上谁、怎么办"，不是一个 abort 堆栈。
async function fetchWithRetry(url, init, tries = 3) {
  let last;
  for (let i = 0; i < tries; i++) {
    try {
      return await fetch(url, init);
    } catch (e) {
      last = e;
      await new Promise((res) => setTimeout(res, 400 * (i + 1)));
    }
  }
  throw new Error(`连不上 ${url}：${String(last?.message || last)} —— 前端 :3000 与后端 :3001 都在跑吗？`);
}

async function login(base) {
  const user = process.env.SNAP_USER;
  const pass = process.env.SNAP_PASS;
  if (!user || !pass) throw new Error('解析夹具需要 SNAP_USER / SNAP_PASS（口令只走环境变量，不写进仓库）');
  const res = await fetchWithRetry(`${base}/api/v1/auth/login`, { method: 'POST', headers: auth(''), body: JSON.stringify({ username: user, password: pass }) });
  const body = await res.json().catch(() => null);
  const token = body?.data?.access_token;
  if (!token) throw new Error(`工装登录失败（${res.status}）：${JSON.stringify(body)?.slice(0, 160) ?? '响应不是 JSON'}`);
  try {
    fs.writeFileSync(TOKEN_CACHE, JSON.stringify({ token, at: Date.now() }), 'utf8');
  } catch {
    // 缓存写不进去只影响下一次要重新登录，不值得让巡检失败
  }
  return token;
}

async function getJson(base, token, url, tries = 3) {
  for (let i = 0; i < tries; i++) {
    const res = await fetchWithRetry(`${base}/api/v1${url}`, { headers: auth(token) });
    // 429 不是"后端坏了"，是前面几台工装攒下来的限流 —— 等一个窗口再来。
    // 不退避的话 pickTargets 会抛，而抛在 Windows/Node 24 上会撞上还没关的 undici socket，
    // libuv 断言把退出码改成 127，人看到的就只是一段 abort 堆栈。
    if (res.status === 429 && i + 1 < tries) {
      console.log(`↻ ${url} 被限流（429），等 20 秒后重试 ${i + 1}/${tries - 1}`);
      await new Promise((res2) => setTimeout(res2, 20_000));
      continue;
    }
    if (!res.ok) throw new Error(`GET ${url} → ${res.status}`);
    const body = await res.json();
    return body?.data;
  }
  throw new Error(`GET ${url} 一直被限流`);
}

/** 叶子文档（type 1）才算数：目录节点能打开但没有正文，截出来的图不是"文档页" */
function pickDoc(nodes, spaceId) {
  const doc = (nodes ?? []).find((n) => n.type === 1 && !n.is_delete);
  return doc ? { spaceId, docId: doc.id } : null;
}

/**
 * 现场解析出待测页需要的真实 id。
 * 挑"文档最多的空间里第一篇真文档"，而不是固定第 1 空间第 356 篇 ——
 * 后者换一台机器就不存在了。取不到文档就明确抛错：宁可巡检跑不起来，
 * 也不要静默拿一个 404 页去量"这页有没有页头"。
 */
export async function resolveFixtures() {
  const base = apiBase();
  const token = await cachedToken(base);

  const spaces = (await getJson(base, token, '/spaces/my?page=1&page_size=50'))?.data ?? [];
  const ordered = [...spaces].sort((a, b) => (b.doc_count ?? 0) - (a.doc_count ?? 0));
  let spaceId = 0;
  let docId = 0;
  for (const space of ordered.slice(0, 8)) {
    const roots = (await getJson(base, token, `/documents/tree?space_id=${space.id}`)) ?? [];
    let found = pickDoc(roots, space.id);
    // 根上全是目录时再下一层，而不是把整棵树拉平：夹具只要一篇真文档
    for (const root of found ? [] : roots.filter((n) => n.has_children).slice(0, 5)) {
      found ??= pickDoc((await getJson(base, token, `/documents/${root.id}/children`)) ?? [], space.id);
    }
    if (found) {
      ({ spaceId, docId } = found);
      break;
    }
  }
  if (!docId) throw new Error(`跑着的应用里找不到任何文档（扫了 ${Math.min(ordered.length, 8)} 个空间）—— 夹具无法解析，拒绝用 404 页当被测对象`);

  const users = (await getJson(base, token, '/users?page=1&page_size=10'))?.data ?? [];
  const userId = users.find((u) => !u.is_delete && !u.is_forbidden)?.id ?? users[0]?.id;
  if (!userId) throw new Error('夹具解析失败：一个用户都取不到');

  const roles = (await getJson(base, token, '/roles')) ?? [];
  const roleId = (Array.isArray(roles) ? roles : roles?.data ?? []) [0]?.role_id;

  return { space: spaceId, doc: docId, user: userId, role: roleId };
}

/** 把 __SPACE__ / __DOC__ / __USER__ / __ROLE__ 换成真实 id；缺一个就宁可报错也不留占位符 */
export function fillFixtures(list, fx) {
  return list.map((item) => {
    let p = item.path;
    for (const key of FIXTURE_KEYS) {
      if (p.includes(`__${key.toUpperCase()}__`)) {
        const value = fx[key];
        if (value === undefined || value === null || value === '') {
          throw new Error(`夹具 ${key} 没解析出来，但 ${item.path} 用到它`);
        }
        p = p.replaceAll(`__${key.toUpperCase()}__`, String(value));
      }
    }
    return { ...item, path: p };
  });
}

export async function resolveTargets(list) {
  return fillFixtures(list, await resolveFixtures());
}

/**
 * 工装通用的"取待测页 + 按 --only 过滤"。
 * 收成一处是因为 12 台各自写过一遍同样的三元表达式，而且都写成 `loadTargets(here) ? ... : loadTargets(here)`
 * —— 一次调用要读两遍文件、解析两遍夹具；夹具改成现场解析之后，那就是每次多打一轮 API。
 */
export async function pickTargets(only, scriptsDir, { matchPath = false } = {}) {
  let all;
  try {
    all = await loadTargets(scriptsDir);
  } catch (e) {
    // 取不出清单就干净退出，别让 Node 在 Windows 上抛一个 libuv 断言（exit=127 + abort 堆栈，
    // 看不出是谁挂了）。这不是产品缺陷：先确认 :3000 与 :3001 在跑，再重跑这一台。
    console.error(`路由清单/夹具取不出来：${String(e?.message || e)}`);
    console.error('  这不是产品缺陷 —— 先确认前端 :3000 与后端 :3001 在跑（连跑多台时本机 socket 可能被打满），再重跑这一台。');
    process.exit(2);
  }
  if (!only) return all;
  return all.filter((r) => r.name.includes(only) || (matchPath && r.path.includes(only)));
}

export async function loadTargets(scriptsDir = path.join(here, '..')) {
  const snap = fs.readFileSync(path.join(scriptsDir, 'visual-snapshot.mjs'), 'utf8');
  // 静态拦一道：路由字面量里不许再出现 /space/1、/user/9、/doc/356 这类魔法 id。
  // 写死的那一刻它是真的，一轮清库之后它就是 404 —— 而巡检不会报"夹具没了"，
  // 它会把这页当普通页量下去，产出一堆"这页没有页头/这页节奏不对"的假设计问题。
  const magic = [...snap.matchAll(/\{ path: '([^']*)'/g)]
    .map((m) => m[1])
    .filter((p) => /\/(space|user|roles?|doc|share)\/\d/.test(p));
  if (magic.length > 0) {
    throw new Error(`visual-snapshot.mjs 里有写死的夹具 id：${magic.join(', ')} —— 换成 __SPACE__/__DOC__/__USER__/__ROLE__，由这里现场解析`);
  }
  const aiSections = [...(snap.match(/const AI_SECTIONS = \[([\s\S]*?)\];/)?.[1] || '').matchAll(/'([\w-]+)'/g)]
    .map((m) => m[1]);
  const explicit = [...snap.matchAll(/\{ path: '([^']+)', name: '([^']+)'([^}]*)\}/g)]
    .map((m) => ({ path: m[1], name: m[2], public: /public: true/.test(m[3]) }))
    .filter((r) => !r.public);
  const raw = [
    ...aiSections.map((s) => ({ path: `/ai/settings?section=${s}`, name: `ai-settings__${s}` })),
    ...explicit,
  ];
  // 占位符必须被换掉：残留一个就意味着夹具没解析，量出来的东西不可信
  return (await resolveTargets(raw)).map((r) => {
    if (/__(SPACE|DOC|USER|ROLE)__/.test(r.path)) throw new Error(`夹具没替换干净：${r.path}`);
    return r;
  });
}
