#!/usr/bin/env node
/**
 * 生成四枚主题签名图标—— npm run gen:theme-icons
 *
 * 复用主流开源图标库（lucide，GitHub/VS Code/macOS 设置同款）的几何：
 *   light      lucide `sun`        —— 圆形核心 + 8 根 45° 等分光芒
 *   dark       lucide `moon`       —— 单一贝塞尔月牙
 *   editorial  lucide `newspaper`  —— 报纸轮廓 + 标题块 + 两行正文
 *   warm-paper lucide `book-open`  —— 摊开的书本 + 中缝
 *
 * 形状直接沿用 lucide 官方 24×24 viewBox 的 path，等比放大到 208×144 画布。
 * 颜色取 themes.ts 的 CSS token，与运行时主题单一来源；
 * 描边线宽、端点、圆角全部沿用 lucide 的 stroke 规范，
 * 保证与生态内其它 lucide 图标视觉一致，不重画、不"手绘"。
 */
import fs from 'fs';
import path from 'path';
import module from 'node:module';
import { fileURLToPath } from 'node:url';
import { buildSync } from 'esbuild';
import { chromium } from 'playwright';
import { findChromium } from './lib/chromium-path.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(HERE, '..');
const OUT_DIR = path.join(root, 'src', 'assets', 'theme-icons');

const bundle = buildSync({
  stdin: {
    contents: `
      export { cssVariables } from './src/utils/core/apple-tokens';
      export { THEME_CONTRACT_KEYS, THEME_REGISTRY, themeExtraCss, themeOverrideMapFor } from './src/styles/theme/themes';
      export { HighlightColors, HIGHLIGHT_PALETTE_BY_THEME } from './src/styles/theme/highlight-tokens';
    `,
    resolveDir: root,
    loader: 'ts',
  },
  bundle: true,
  format: 'cjs',
  platform: 'node',
  write: false,
  absWorkingDir: root,
});

const req = module.createRequire(path.join(root, 'noop.js'));
const mod = { exports: {} };
new Function('module', 'exports', 'require', bundle.outputFiles[0].text)(mod, mod.exports, req);
const { cssVariables, THEME_REGISTRY, themeExtraCss, themeOverrideMapFor, HighlightColors, HIGHLIGHT_PALETTE_BY_THEME } = mod.exports;

const varsBlock = (map) => Object.entries(map).map(([k, v]) => `  ${k}: ${v};`).join('\n');
const hlVars = (palette) => Object.fromEntries(Object.entries(HighlightColors[palette]).map(([k, v]) => [`--hl-${k}`, v]));

// 每套主题一段 CSS：覆写表 + 高亮族 + extraCss（与 gen-theme-preview 同一口径）
const themeCss = THEME_REGISTRY.map(({ id }) => {
  const map = themeOverrideMapFor(id);
  const hl = `html[data-theme='${id}'] {\n${varsBlock(hlVars(HIGHLIGHT_PALETTE_BY_THEME[id]))}\n}`;
  const vars = map ? `html[data-theme='${id}'] {\n${varsBlock(map)}\n}\n\n` : '';
  return vars + hl + (themeExtraCss[id] ?? '');
}).join('\n\n');

/**
 * lucide 官方 SVG 几何（24×24 viewBox），内联复用。
 * 描边线宽 2、端点 round、拐角 round —— 与 lucide 文档与 GitHub 图标规范一致。
 *
 * 画布布局：208×144，lucide 24×24 几何居中放置：
 *   scale = 5   （24 * 5 = 120，留出两侧 44px 与上下 12px 的呼吸空间）
 *   中心 (104, 72)，几何中心 (12, 12)
 * 因此 transform = translate(44, 12) scale(5)，即 translate(cx - 12*s, cy - 12*s) scale(s)。
 * 为保持描边比例不变，stroke-width 也乘 5 → 10，
 * 视觉上等价于"在 24px 图标里用 2px 描边"，放大后锐度一致。
 */
const LUCIDE = {
  sun: [
    { tag: 'circle', attrs: { cx: 12, cy: 12, r: 4 } },
    { tag: 'path', attrs: { d: 'M12 2v2' } },
    { tag: 'path', attrs: { d: 'M12 20v2' } },
    { tag: 'path', attrs: { d: 'm4.93 4.93 1.41 1.41' } },
    { tag: 'path', attrs: { d: 'm17.66 17.66 1.41 1.41' } },
    { tag: 'path', attrs: { d: 'M2 12h2' } },
    { tag: 'path', attrs: { d: 'M20 12h2' } },
    { tag: 'path', attrs: { d: 'm6.34 17.66-1.41 1.41' } },
    { tag: 'path', attrs: { d: 'm19.07 4.93-1.41 1.41' } },
  ],
  moon: [
    { tag: 'path', attrs: { d: 'M20.985 12.486a9 9 0 1 1-9.473-9.472c.405-.022.617.46.402.803a6 6 0 0 0 8.268 8.268c.344-.215.825-.004.803.401' } },
  ],
  newspaper: [
    { tag: 'path', attrs: { d: 'M15 18h-5' } },
    { tag: 'path', attrs: { d: 'M18 14h-8' } },
    { tag: 'path', attrs: { d: 'M4 22h16a2 2 0 0 0 2-2V4a2 2 0 0 0-2-2H8a2 2 0 0 0-2 2v16a2 2 0 0 1-4 0v-9a2 2 0 0 1 2-2h2' } },
    { tag: 'rect', attrs: { width: 8, height: 4, x: 10, y: 6, rx: 1 } },
  ],
  'book-open': [
    { tag: 'path', attrs: { d: 'M12 5v16' } },
    { tag: 'path', attrs: { d: 'M20.001 19A2 2 0 0022 17V5a2 2 0 00-1.999-2L16 3.002A5 5 0 0012 5a5 5 0 00-4-2H4a2 2 0 00-2 2v12a2 2 0 001.999 2H8a5 5 0 014 2 5 5 0 014-2z' } },
  ],
};

/** 把一条 lucide 节点渲染成 SVG 元素字符串 */
const toEl = (n) => {
  const attrs = Object.entries(n.attrs).map(([k, v]) => `${k}="${v}"`).join(' ');
  return `    <${n.tag} ${attrs} />`;
};

/**
 * 渲染一枚图标：lucide 几何 + CSS 变量颜色 + 居中等比缩放。
 * 背景取自该主题的画布色，保证与菜单里 1.25rem 方块的 paper 底色同色（不露缝）。
 */
const renderIcon = (id, iconKey) => {
  const nodes = LUCIDE[iconKey].map(toEl).join('\n');
  return `
  <svg viewBox="0 0 208 144" xmlns="http://www.w3.org/2000/svg">
    <rect x="0" y="0" width="208" height="144" fill="var(--apple-surface-canvas)"/>
    <g transform="translate(44, 12) scale(5)"
       fill="none" stroke="var(--apple-blue)" stroke-width="2"
       stroke-linecap="round" stroke-linejoin="round">
${nodes}
    </g>
  </svg>`;
};

const ICONS = {
  light: renderIcon('light', 'sun'),
  dark: renderIcon('dark', 'moon'),
  editorial: renderIcon('editorial', 'newspaper'),
  'warm-paper': renderIcon('warm-paper', 'book-open'),
};

const rootCss = `:root {\n${varsBlock(cssVariables)}\n}\n:root {\n${varsBlock(hlVars('light'))}\n}`;

const pageFor = (id) => `<!DOCTYPE html>
<html lang="zh-CN" data-theme="${id}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=208, initial-scale=1">
<style>
${rootCss}
${themeCss}
* { box-sizing: border-box; margin: 0; padding: 0; }
html, body { width: 208px; height: 144px; display: flex; align-items: center; justify-content: center;
  background: var(--apple-surface-canvas); }
svg { width: 208px; height: 144px; display: block; }
</style>
</head>
<body>
${ICONS[id]}
</body>
</html>
`;

// 视口精确等于图标画布，DPR 4 → 832×576 位图
const VIEWPORT = { width: 208, height: 144 };
const DPR = 4;

async function main() {
  const exe = findChromium();
  if (!exe) {
    console.error('✗ 找不到 Chromium（ms-playwright 或系统 Edge/Chrome），无法截图');
    process.exit(1);
  }
  const browser = await chromium.launch({ executablePath: exe, args: ['--force-color-profile=srgb'] });
  fs.mkdirSync(OUT_DIR, { recursive: true });
  for (const { id } of THEME_REGISTRY) {
    const page = await browser.newPage({
      viewport: VIEWPORT,
      deviceScaleFactor: DPR,
    });
    await page.setContent(pageFor(id), { waitUntil: 'load' });
    // 等字体与布局稳定一拍（系统字体，这里主要等布局稳定）
    await page.waitForTimeout(300);
    const out = path.join(OUT_DIR, `${id}.png`);
    await page.screenshot({ path: out, omitBackground: false, clip: { x: 0, y: 0, width: VIEWPORT.width, height: VIEWPORT.height } });
    await page.close();
    console.log(`✓ ${id} → ${path.relative(root, out)}`);
  }
  await browser.close();

  // 生成一张 @4x 对照预览（四枚横排），供肉眼检查
  const { execFileSync } = await import('node:child_process');
  const preview = path.join(OUT_DIR, 'preview.png');
  const ids = THEME_REGISTRY.map(({ id }) => id);
  const imgMagick = ['magick', 'convert'].find((bin) => {
    try { execFileSync(bin, ['-version'], { stdio: 'ignore' }); return true; } catch { return false; }
  });
  if (imgMagick) {
    execFileSync(imgMagick, [...ids.map((id) => path.join(OUT_DIR, `${id}.png`)), '+append', '-resize', '416x288', preview], { stdio: 'ignore' });
    console.log(`✓ 预览 ${path.relative(root, preview)}`);
  } else {
    console.log('ℹ 未找到 ImageMagick，跳过对照预览（四张 PNG 已就绪）');
  }
  console.log(`\n完成。四枚主题图标已落 src/assets/theme-icons/（${VIEWPORT.width}×${VIEWPORT.height} @${DPR}x）。`);
  console.log('复用 lucide 开源图标：light=sun · dark=moon · editorial=newspaper · warm-paper=book-open。');
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
