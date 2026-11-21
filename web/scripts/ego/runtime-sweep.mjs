/* global taskSpace -- ego-browser 注入的运行时全局，不在模块作用域里 */
/**
 * 运行时冒烟巡检的 ego-lite 任务体 —— 由 scripts/check-runtime-ego.mjs 拉起：
 *   ego-browser nodejs -e "globalThis.__EGO_RUN = {...}; await import('file:///…/runtime-sweep.mjs')"
 *
 * 这里跑在 ego 的 Node 运行时里，不是页面环境：浏览器侧代码一律进 page.evaluate()。
 * 入参只有 globalThis.__EGO_RUN = { routes, out }，路径都是绝对路径、正斜杠。
 */
const run = globalThis.__EGO_RUN;
if (!run?.routes || !run?.out) throw new Error('缺少 __EGO_RUN（请通过 npm run check:runtime 启动）');

const fs = await import('node:fs/promises');
const routes = JSON.parse(await fs.readFile(run.routes, 'utf8'));
const BASE = 'http://localhost:3000';

/** 页面上的两套 ErrorBoundary 文案（中/英各一），命中即判失败 */
const BROKEN = ['渲染内容时发生了未知错误', 'Something went wrong rendering this page'];

const task = await taskSpace(`runtime-ego ${new Date().toISOString().slice(0, 19)}`);
const page = (await task.pages())[0] ?? (await task.newPage());

// 英文档下巡检：坑位文案最容易漏翻，且与 Playwright 的 check:i18n-runtime 同一档位。
// 语言是写进用户浏览器 profile 的，跑完必须还原 —— 巡检不该顺手把人的界面改成英文。
await page.goto(BASE, { timeout: 30000 });
const prevLocale = await page.evaluate(() => localStorage.getItem('app-locale'));
await page.evaluate(() => localStorage.setItem('app-locale', 'en-US'));
await page.reload({ timeout: 30000 });
await page.waitForLoadState('load', { timeout: 15000 }).catch(() => {});

const results = [];
for (const route of routes) {
  const started = Date.now();
  const row = { name: route.name, path: route.path, ok: true, problems: [], chinese: [], ms: 0 };
  try {
    await page.goto(BASE + route.path, { timeout: 25000 });
    await page.waitForLoadState('load', { timeout: 15000 }).catch(() => {});
    // 首屏是异步 chunk + 后端数据，立刻读 DOM 会把"还没渲染"当成"白屏"
    await page.waitForTimeout(700);
    const probe = await page.evaluate((broken) => {
      const text = (document.body?.innerText || '').replace(/\s+/g, ' ').trim();
      return {
        len: text.length,
        title: document.title,
        broken: broken.some((b) => text.includes(b)),
        chinese: (text.match(/[\u4e00-\u9fa5]+/g) || []).slice(0, 5),
      };
    }, BROKEN);
    if (probe.broken) row.problems.push('落到 ErrorBoundary');
    if (probe.len < 20) row.problems.push(`页面几乎为空（${probe.len} 字符）`);
    row.chinese = probe.chinese;
    row.title = probe.title;
    const shot = `${run.out}/shots/${route.name.replace(/[^\w.-]+/g, '_')}.png`;
    await page.screenshot({ path: shot }).catch(() => {});
    row.shot = shot;
  } catch (e) {
    row.problems.push(`打开失败：${String(e?.message || e).slice(0, 160)}`);
  }
  row.ms = Date.now() - started;
  if (row.problems.length) row.ok = false;
  results.push(row);
  console.log(`${row.ok ? '✓' : '✗'} ${row.name} ${row.path} ${row.ms}ms${row.problems.length ? ' — ' + row.problems.join('；') : ''}`);
}

const failed = results.filter((r) => !r.ok);
const withCjk = results.filter((r) => r.chinese.length > 0);
const report = { at: new Date().toISOString(), total: results.length, failed: failed.length, results };
await fs.writeFile(`${run.out}/report.json`, JSON.stringify(report, null, 2), 'utf8');

console.log(`\n合计 ${results.length} 条路由，失败 ${failed.length} 条；英文档下仍有可见中文的 ${withCjk.length} 条（仅报告，不判失败）`);
if (withCjk.length) {
  console.log('可见中文样例（多半是用户数据，如空间名/文档标题）：');
  for (const r of withCjk.slice(0, 10)) console.log(`  · ${r.name}：${r.chinese.join(' / ')}`);
}
console.log(`报告：${run.out}/report.json`);

await page
  .evaluate((p) => (p === null ? localStorage.removeItem('app-locale') : localStorage.setItem('app-locale', p)), prevLocale)
  .catch(() => {});
await task.finish({ keep: [] });
process.exitCode = failed.length ? 1 : 0;
