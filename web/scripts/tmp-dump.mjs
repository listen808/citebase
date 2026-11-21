// 一次性诊断：以 en-US 档登录后访问给定路由，打印界面上还剩的中文行（去重）。
import { chromium } from 'playwright';
import { findChromium } from './lib/chromium-path.mjs';

const BASE = 'http://localhost:3000';
const CJK = '[\\u3400-\\u9fff]';
const routes = process.argv.slice(2);
const b = await chromium.launch({ executablePath: findChromium(), args: ['--no-sandbox', '--disable-dev-shm-usage'] });
const page = await (await b.newContext({ viewport: { width: 1600, height: 1000 } })).newPage();
await page.goto(BASE, { waitUntil: 'domcontentloaded' });
await page.evaluate(() => localStorage.setItem('app-locale', 'en-US'));
await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' });
await page.fill('input[autocomplete="username"]', process.env.SNAP_USER);
await page.fill('input[autocomplete="current-password"]', process.env.SNAP_PASS);
await page.locator('form button').last().click();
await page.waitForURL((u) => !u.pathname.includes('login'), { timeout: 25000 });
for (const r of routes) {
  await page.goto(BASE + r, { waitUntil: 'domcontentloaded' }).catch(() => {});
  await page.waitForTimeout(1000);
  const t = await page.evaluate(() => document.body.innerText).catch(() => '');
  const hits = [...new Set(t.match(new RegExp('[^\\n]*' + CJK + '+[^\\n]*', 'g')) || [])];
  console.log(`=== ${r}  cjk=${(t.match(new RegExp(CJK, 'g')) || []).length}`);
  hits.slice(0, 16).forEach((h) => console.log('    ' + h.slice(0, 70)));
}
await b.close();
