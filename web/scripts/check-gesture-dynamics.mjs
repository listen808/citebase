// 手势动力学巡检：静态门禁能证明"只有一处实现"，证明不了"手感真的是跟手的"。
// 这台在 390 宽的触屏视口里真的按下、真的拖、真的甩，读的是元素的渲染盒。
// 四条判据（都来自《Designing Fluid Interfaces》）：
//   G1 位移由弹簧驱动，不是 keyframe 重放 —— transform 的 transition-duration 必须是 0s。
//      只要还挂着 CSS 过渡，拖到一半松手就要等动画跑完，"打断"就是假的。
//   G2 1:1 跟手 —— 手指走 40px，渲染盒就要走 40px（容差 3px）。
//   G3 越界橡皮筋 —— 越过静止位置继续拖，画面要跟得动但明显沉（0 < 实际 < 手指）。
//   G4 按速度投影落点 —— 快甩一下要走完；慢拖一点点停住再松手必须回弹。
//   G5 减弱动态效果 —— 开了 reduce 之后开抽屉应当直接落位，而不是一帧一帧挪。
// 只做"开—拖—甩—关"，绝不点任何有副作用的按钮（脚本跑在真实库上）。
// 用法（前端 dev :3000、后端 :3001 需在跑；口令只走环境变量）：
//   SNAP_USER=… SNAP_PASS=… node scripts/check-gesture-dynamics.mjs
//   node scripts/check-gesture-dynamics.mjs --selftest   # 反向验证：拿一个 keyframe 抽屉喂给同一批判据，必须全报红
import { chromium } from 'playwright';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { findChromium } from './lib/chromium-path.mjs';

const BASE = process.env.SNAP_BASE || 'http://localhost:3000';
const here = path.dirname(fileURLToPath(import.meta.url));
const OUT = path.join(here, '..', 'gesture-dynamics.json');
const SELFTEST = process.argv.includes('--selftest');
const USER = process.env.SNAP_USER;
const PASS = process.env.SNAP_PASS;
if (!SELFTEST && (!USER || !PASS)) {
  console.error('需要 SNAP_USER / SNAP_PASS（口令只走环境变量，不写进仓库）');
  process.exit(2);
}

/** 读一个元素的渲染盒左边缘（弹簧和 keyframe 都改这里，比读 style 字符串稳） */
const leftOf = (page, sel) => page.$eval(sel, (el) => el.getBoundingClientRect().left);

const transformDuration = (page, sel) => page.$eval(sel, (el) => {
  const cs = getComputedStyle(el);
  const props = (cs.transitionProperty || '').split(',').map((s) => s.trim());
  const i = props.findIndex((p) => p === 'transform' || p === 'all');
  return i === -1 ? '0s' : (cs.transitionDuration.split(',')[i] || cs.transitionDuration).trim();
});

/** steps=1 + dwell=0 就是"一甩"；steps 多且每步等一会儿就是"慢慢拖" */
async function drag(page, x, y, dx, { steps = 4, dwell = 0 } = {}) {
  await page.mouse.move(x, y);
  await page.mouse.down();
  for (let i = 1; i <= steps; i++) {
    await page.mouse.move(x + (dx * i) / steps, y);
    if (dwell) await page.waitForTimeout(dwell);
  }
  await page.mouse.up();
}

/** 四条判据跑在"任何一个可拖走的层"上：真页面和 selftest 的假抽屉共用这一段 */
async function auditSheet(page, { panelSel, edgeSel, restLeft, dismissLeft, label }) {
  const f = [];
  const dur = await transformDuration(page, panelSel);
  if (dur !== '0s') f.push(`G1 ${label}：transform 还挂着 ${dur} 的 CSS 过渡 —— 拖到一半松手要等动画跑完，打断是假的`);

  const edge = await page.$eval(edgeSel, (el) => {
    const r = el.getBoundingClientRect();
    return { x: Math.round(r.left + r.width / 2), y: Math.round(r.top + Math.min(r.height / 2, 200)) };
  });

  // G2 跟手
  const a0 = await leftOf(page, panelSel);
  const dirSign = Math.sign(dismissLeft - restLeft);
  const dx = 40 * dirSign;
  await page.mouse.move(edge.x, edge.y);
  await page.mouse.down();
  await page.mouse.move(edge.x + dx / 2, edge.y);
  await page.waitForTimeout(70);   // 没有这一等，两步之间的间隔是毫秒级，"慢慢拖"会被读成一甩
  await page.mouse.move(edge.x + dx, edge.y);
  const a1 = await leftOf(page, panelSel);
  const tracked = a1 - a0;
  if (Math.abs(tracked - dx) > 3) f.push(`G2 ${label}：手指走了 ${dx}px，画面只走了 ${Math.round(tracked)}px —— 没跟手`);
  // 松手让它回到静止位，橡皮筋单独量：指针位置是相对上一次算的，
  // 接着上一次拖的坐标算增量会把"回弹了一段"混进来，量出来的就不是橡皮筋
  await page.mouse.up();
  await page.waitForTimeout(700);

  // G3 橡皮筋：从静止往反方向（越过边界）拖 60px
  const bx = -60 * dirSign;
  const b0 = await leftOf(page, panelSel);
  await page.mouse.move(edge.x, edge.y);
  await page.mouse.down();
  await page.mouse.move(edge.x + bx / 2, edge.y);
  await page.mouse.move(edge.x + bx, edge.y);
  const b1 = await leftOf(page, panelSel);
  const moved = b1 - b0;
  if (Math.sign(moved) !== Math.sign(bx) || Math.abs(moved) > Math.abs(bx) || Math.abs(moved) < 3) {
    f.push(`G3 ${label}：越过静止位置再拖 ${bx}px，画面只走了 ${Math.round(moved)}px —— 应当"跟得动但明显沉"（同号、且小于手指）`);
  }
  await page.mouse.up();
  await page.waitForTimeout(700);

  // G4a 慢拖一点就松手：必须回弹
  const c0 = await leftOf(page, panelSel);
  await drag(page, edge.x, edge.y, 40 * Math.sign(dismissLeft - restLeft), { steps: 3, dwell: 120 });
  await page.waitForTimeout(900);
  const c1 = await leftOf(page, panelSel);
  if (Math.abs(c1 - c0) > 6) f.push(`G4 ${label}：慢拖 40px 停住松手，却落到了 ${Math.round(c1 - c0)}px 外 —— 回弹判据丢了`);

  // G4b 一甩：必须走完（dismiss）
  const d0 = await leftOf(page, panelSel);
  await drag(page, edge.x, edge.y, 60 * Math.sign(dismissLeft - restLeft), { steps: 2, dwell: 0 });
  await page.waitForTimeout(900);
  const d1 = await leftOf(page, panelSel).catch(() => null);
  const gone = d1 === null || Math.abs(d1 - d0) > Math.abs(dismissLeft - restLeft) * 0.6;
  if (!gone) f.push(`G4 ${label}：60px 的一甩只走了 ${Math.round((d1 || d0) - d0)}px —— 没按速度投影落点，松手就该滑走`);
  return f;
}

const login = async (page) => {
  await page.goto(`${BASE}/login`, { waitUntil: 'networkidle' });
  await page.fill('input[autocomplete="username"]', USER);
  await page.fill('input[autocomplete="current-password"]', PASS);
  await page.getByRole('button', { name: '登录' }).first().click();
  await page.waitForTimeout(2500);
};

const openNavDrawer = async (page) => {
  // 只认这个精确名字：`[aria-label*="导航"]` 会先命中外层 <nav aria-label="主导航">，
  // 点它什么也不会发生，于是后面"没量到"被读成"没违例"
  const burger = page.locator('[aria-label="打开菜单"]').first();
  await burger.click();
  await page.waitForTimeout(600);
};

const run = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const ctx = await browser.newContext({
    viewport: { width: 390, height: 844 },
    hasTouch: true,
    isMobile: true,
    locale: 'zh-CN',
  });
  const page = await ctx.newPage();
  const findings = [];
  try {
    await login(page);
    await page.goto(BASE, { waitUntil: 'networkidle' });
    await openNavDrawer(page);
    const panelSel = '[data-ap="drawer"]';
    if (!(await page.$(panelSel))) {
      findings.push('G0 移动端没找到抽屉面板（`[data-ap="drawer"]`）—— 后面四条都没量到，别读成"全绿"');
    } else {
      const restLeft = await leftOf(page, panelSel);
      const width = await page.$eval(panelSel, (el) => el.getBoundingClientRect().width);
      findings.push(...await auditSheet(page, {
        panelSel,
        edgeSel: '[data-ap="drawer-edge"]',
        restLeft,
        dismissLeft: restLeft - width,
        label: '移动端导航抽屉',
      }));
      // G5 减弱动效：重开一次，开了 reduce 之后应当直接落位
      await page.emulateMedia({ reducedMotion: 'reduce' });
      await page.keyboard.press('Escape').catch(() => {});
      await page.waitForTimeout(300);
      if (await page.$(panelSel)) { await page.mouse.click(370, 400); await page.waitForTimeout(400); }
      await openNavDrawer(page);
      const settled = await leftOf(page, panelSel);
      if (Math.abs(settled) > 2) findings.push(`G5 开了「减弱动态效果」抽屉还停在 ${Math.round(settled)}px —— 弹簧没接住这条系统设置`);
      await page.emulateMedia({ reducedMotion: null });
    }
  } finally {
    await browser.close();
  }
  fs.writeFileSync(OUT, JSON.stringify({ at: new Date().toISOString(), findings }, null, 2));
  for (const line of findings) console.log(`   ${line}`);
  console.log(findings.length ? `\n✗ 手势动力学未通过：${findings.length} 项` : '\n✓ 跟手 / 橡皮筋 / 动量投影 / 减弱动效 四项都成立');
  process.exit(findings.length ? 1 : 0);
};

/**
 * 反向验证：不碰真页面，喂一个"keyframe 重放 + 不跟手"的假抽屉给同一批判据。
 * 判据如果写错了（比如 selector 取不到、或者只看 style 不看渲染盒），在假货上也会报绿，
 * 那它的 0 就是空的。这里要求 G1/G2/G4 至少各报一条。
 */
const selftest = async () => {
  const browser = await chromium.launch({ executablePath: findChromium() });
  const page = await browser.newPage({ viewport: { width: 390, height: 844 } });
  await page.setContent(`<!doctype html><style>
    html,body{margin:0;height:100%}
    #panel{position:fixed;top:0;left:0;width:280px;height:100%;background:#eee;
      transform:translateX(-100%);transition:transform .3s cubic-bezier(.32,.72,0,1)}
    #panel.open{transform:translateX(0)}
    #edge{position:absolute;top:0;right:0;width:20px;height:100%}
  </style><div id="panel" class="open"><div id="edge"></div></div>`);
  const findings = await auditSheet(page, {
    panelSel: '#panel',
    edgeSel: '#edge',
    restLeft: 0,
    dismissLeft: -280,
    label: '假抽屉（keyframe 重放，不跟手）',
  });
  await browser.close();
  const hit = {
    G1: findings.some((f) => f.startsWith('G1')),
    G2: findings.some((f) => f.startsWith('G2')),
    G4: findings.some((f) => f.startsWith('G4')),
  };
  console.log(`   注入的假抽屉被抓到：${findings.length} 条`);
  for (const f of findings) console.log(`     ${f}`);
  const missing = Object.entries(hit).filter(([, ok]) => !ok).map(([k]) => k);
  if (missing.length) {
    console.error(`✗ ${missing.join('/')} 没抓到明显违例 —— 判据写错了，真页面的 0 不可信`);
    process.exit(1);
  }
  console.log('✓ 三条判据都能抓到自造违例');
};

if (SELFTEST) selftest(); else run();
