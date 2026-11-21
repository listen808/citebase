#!/usr/bin/env node
/**
 * 运行时冒烟门禁（ego-lite 版）—— 与 Playwright 那套**并存**，不替代：
 *   · Playwright 套（snap / check:theme-rounds / check:glass:runtime / check:i18n-runtime）
 *     自带 Chromium、能进自动化流水线，但每次冷启动 + 登录，跑一轮很重；
 *   · 这套用本机已登录的 ego-lite，省掉冷启动与登录，适合"改完界面立刻看一眼"。
 *
 * 它只判三件事（重活仍归 Playwright 那套）：
 *   ① 路由打得开（不白屏）；
 *   ② 没有落到 ErrorBoundary（「渲染内容时发生了未知错误」）；
 *   ③ 顺带记录英文档下的可见中文条数 —— 只报告、不设门槛，
 *      因为用户数据（空间名、文档标题）本来就不该翻，硬门槛会天天误报。
 *
 * 用法（前端 :3000、后端 :3001 需在跑；ego-lite 需已登录）：
 *   SNAP_USER=… SNAP_PASS=… npm run check:runtime [-- --only=/ai] [-- --max=10]
 *
 * `check:runtime` 默认就是这一台；Playwright 那台（console/5xx）改名 `check:runtime:pw`。
 *
 * 口令仍然只走环境变量：路由夹具（空间/文档 id）由本进程解析后写成 routes.json，
 * ego 里那个 Node 运行时拿不到 shell 环境变量，也不需要拿到。
 */
import { spawn } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { loadTargets } from './lib/route-list.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(here, '..', '..');
const OUT = path.join(ROOT, 'docs', 'ui-ego');

const argv = process.argv.slice(2);
const only = (argv.find((a) => a.startsWith('--only=')) || '').slice(7);
const max = Number((argv.find((a) => a.startsWith('--max=')) || '--max=0').slice(6)) || 0;

if (!process.env.SNAP_USER || !process.env.SNAP_PASS) {
  console.error('需要 SNAP_USER / SNAP_PASS 环境变量（路由夹具现场解析要用它取真实 id）');
  process.exit(2);
}

let targets;
try {
  targets = await loadTargets(here);
} catch (e) {
  console.error(`路由清单/夹具取不出来：${String(e?.message || e)}`);
  console.error('  这不是产品缺陷 —— 先确认前端 :3000 与后端 :3001 在跑，再重跑这一台。');
  process.exit(2);
}
let routes = only ? targets.filter((r) => r.name.includes(only) || r.path.includes(only)) : targets;
if (max > 0) routes = routes.slice(0, max);

fs.mkdirSync(path.join(OUT, 'shots'), { recursive: true });
const input = path.join(OUT, 'routes.json');
fs.writeFileSync(input, JSON.stringify(routes.map((r) => ({ path: r.path, name: r.name })), null, 2), 'utf8');

// ego-browser 只认文件模块（PowerShell 下不能 heredoc），参数从 globalThis 递进去：
// 那个 Node 运行时不是本进程的子作用域，process.env 里什么都没有。
const toUrl = (p) => pathToFileURL(p).href;
const payload = { routes: input.replace(/\\/g, '/'), out: OUT.replace(/\\/g, '/') };
const code = `globalThis.__EGO_RUN = ${JSON.stringify(payload)}; await import(${JSON.stringify(toUrl(path.join(here, 'ego', 'runtime-sweep.mjs')))})`;
const child = spawn('ego-browser', ['nodejs', '-e', code], { stdio: 'inherit' });
child.on('error', (e) => {
  console.error(`ego-browser 起不来：${String(e?.message || e)}`);
  console.error('  Playwright 那套仍可跑：npm run check:runtime:pw');
  process.exit(2);
});
child.on('exit', (c) => process.exit(c ?? 1));
