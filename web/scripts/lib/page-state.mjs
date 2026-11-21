// "这页是不是被 ErrorBoundary 接住了" —— 四台工装都要判，判据只准有一份。
//
// 为什么必须判可见：这是 SPA，KeepAlive 的页面切走之后兜底节点还留在 DOM 里（不可见）。
// 只按 #error-boundary-title 存在与否来判，表现是"上一跳崩过的页面会把脏名分传给下一跳"——
// 实测 home 单独量是干净的，连跑才被别的工装判成崩页，然后它的统计被整页丢掉。
//
// 用法：每台工装在拿到 page 之后 `await page.addInitScript(ERRORED_INIT_SCRIPT)`
// （addInitScript 每次文档加载都会跑，page.goto 换页也不会丢），
// 采集函数里调 `window.__apErrored()`。
export function ERRORED_INIT_SCRIPT() {
  window.__apErrored = () => {
    const eb = document.querySelector('#error-boundary-title');
    if (!eb) return false;
    const r = eb.getBoundingClientRect();
    return r.width > 0 && r.height > 0;
  };
}

/** 给已经在跑的 page 补装探针（addInitScript 只影响之后的导航，所以当前页也要执行一次） */
export async function installErroredProbe(page) {
  await page.addInitScript(ERRORED_INIT_SCRIPT);
  await page.evaluate(ERRORED_INIT_SCRIPT).catch(() => {});
}

/**
 * 用登录表单真登录一次，失败就等冷却再试。
 *
 * 为什么要重试：`/auth/login` 挂的是 10 次/分钟/IP 的严格限流，而一整轮巡检要开八九台工装、
 * 每台都登录一次 —— 串着跑就会把自己锁在门外。表现是"登录失败，无法量测受保护页"（exit 2），
 * 而不是任何产品缺陷。按名字点按钮也不行（英文档下"登录"这个中文名会找不到），所以按位置点。
 */
export async function loginForm(page, base, user, pass, { attempts = 3, cooldownMs = 65_000 } = {}) {
  for (let i = 0; i < attempts; i++) {
    await page.goto(`${base}/login`, { waitUntil: 'load', timeout: 30000 });
    await page.fill('input[autocomplete="username"]', user);
    await page.fill('input[autocomplete="current-password"]', pass);
    await page.locator('form button').last().click();
    await page.waitForTimeout(2500);
    if (!page.url().includes('/login')) return true;
    if (i + 1 < attempts) {
      console.log(`↻ 登录没成（多半是 /auth/login 的 10 次/分钟/IP 限流），等 ${Math.round(cooldownMs / 1000)} 秒后重试 ${i + 1}/${attempts - 1}`);
      await new Promise((res) => setTimeout(res, cooldownMs));
    }
  }
  return false;
}

/**
 * 巡检自己撞限流时，"这页崩了"不是产品事实：73 页 × 每页八九个请求必然穿过 300 次/分钟/IP。
 * 所以等一个滑动窗口（限流 60 秒）把这几种页复跑一次 —— 复跑仍崩才让工具变红。
 * 以前这些工具只打印一句"请单页复跑确认"，等于把工装的一半工作推给记性不好的人。
 *
 * rows 里每项要有 path；remeasure(page, row) 由调用方决定怎么重采（含重新截图的就把图也重截）。
 */
export async function retryCrashedPages(page, base, rows, remeasure, { cooldownMs = 65_000, settleMs = 2000 } = {}) {
  const bad = rows.filter((r) => r.crashed || r.errored);
  if (bad.length === 0) return 0;
  console.log(`↻ ${bad.length} 页被判进兜底页，等 ${Math.round(cooldownMs / 1000)} 秒（限流窗口 60 秒）后复跑：${bad.map((r) => r.name).join(', ')}`);
  await new Promise((res) => setTimeout(res, cooldownMs));
  for (const row of bad) {
    if (!row.path) continue;
    try {
      await page.goto(`${base}${row.path}`, { waitUntil: 'load', timeout: 30000 });
      await new Promise((res) => setTimeout(res, settleMs));
      await remeasure(page, row);
    } catch {
      // 复跑仍失败就保留原判读，让它继续出现在红名单里 —— 掩盖比报出来更糟
    }
  }
  return bad.length;
}
