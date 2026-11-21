#!/usr/bin/env node
/**
 * 生成品牌图标的落盘资产集 —— npm run gen:brand-icons
 *
 * 为什么需要它（2026-11-01 那次取证）：`public/favicon.ico` 是 `d0a45c1 chore: initial commit`
 * 带进来的 AIWiki 旧标记，2026-10-07 改名之后没人重生成过，而浏览器偏偏就用它画
 * 固定标签页、地址栏与 Windows 快捷方式；同时 `vite.config.ts` 把它声明成 64×64，
 * 文件里其实只有一条 32×32。也就是说唯一几何源（ApBrand.vue → favicon.svg / pwa-icon.svg）
 * 是好的，坏的是**从它派生出来的那批位图和贴在它们身上的声明**——而派生物一旦落成二进制，
 * 就再没有任何东西回头看它跟源还像不像。
 *
 * 从此这条链只有一条路：改几何 → 跑本脚本 → 门禁核对。
 *   1. 每枚 PNG（含 ICO 里每条 entry 的 PNG 载荷）都带一条 tEXt `BrandSrc`，
 *      值是几何的 sha256（由 lib/brand-icon-bytes.mjs 从文本里抽，和 ApBrand.vue 同一套判据）。
 *      于是"改了 SVG 却没重跑生成"是**可判**的，不用拿图片做相似度比对。
 *   2. 尺寸只由本脚本决定，声明处（index.html 与 vite manifest）照着它写，
 *      门禁再把声明逐条对回文件真实像素。
 *
 * 用法：cd web && npm run gen:brand-icons
 * 浏览器：Playwright 自带的那份 Chromium（和 visual-snapshot / check-glass-runtime 同一个定位函数，
 * 因此也共享它那条"找不到就退到系统浏览器并在 stderr 公告"的退路）。
 * 输出可重复：同一份源 + 同一份 Chromium 两次跑出来的字节逐位相同（PNG 里没有时间和路径，
 * ICO 目录按固定顺序拼）。
 */
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { chromium } from 'playwright';
import { findChromium } from './lib/chromium-path.mjs';
import { PROVENANCE_KEYWORD, addPngText, brandGeometryStamp, buildIco, pngDims } from './lib/brand-icon-bytes.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const publicDir = path.resolve(here, '..', 'public');

/** 裸标记（摊开的书 + 脊 + 节点）：标签页与 ICO 用它——透明底，压在浅色或深色 tab 上都读得清 */
const MARK_FILE = 'favicon.svg';
/** accent 瓦片（同一套几何缩进 maskable 安全区）：桌面/主屏图标用它 */
const TILE_FILE = 'pwa-icon.svg';

/**
 * 光栅清单。sizes 那三个小尺寸只进 ICO（16 是标签页、32 是高清标签页、48 是控制面板/快捷方式），
 * 192/512 才进 PWA manifest —— Android 的安装门槛是 192+，这条写在 vite.config.ts 里也有账。
 */
const PNG_TARGETS = [
  { file: 'pwa-icon-192.png', source: TILE_FILE, px: 192, fullBleed: false },
  { file: 'pwa-icon-512.png', source: TILE_FILE, px: 512, fullBleed: false },
  // maskable 版必须满幅：系统会自己套圆形/圆角方遮罩，源里留圆角 = 遮罩外一圈透明缺口。
  // 瓦片的几何本来就在安全区（约 60%）里，所以只需要把 rx 抹平，不必重画。
  { file: 'pwa-icon-maskable-512.png', source: TILE_FILE, px: 512, fullBleed: true },
];
const ICO_SIZES = [16, 32, 48];

/** SVG 光栅化要有确定的内在尺寸：源里只有 viewBox，这里按目标像素把 width/height 钉上去 */
const withPixelSize = (svg, px) => {
  const tag = /<svg[^>]*>/.exec(svg);
  if (!tag) throw new Error(`${MARK_FILE} / ${TILE_FILE} 里找不到 <svg> 根标签`);
  const opened = tag[0].replace(/\s(width|height)="[^"]*"/g, '');
  const sized = `${opened.replace(/^<svg/, `<svg width="${px}" height="${px}"`)}${svg.slice(tag.index + tag[0].length)}`;
  return svg.slice(0, tag.index) + sized;
};

/** 满幅版：去掉瓦片的圆角。源里本来就没有 rx 就说明这条转换多余了，抛出来让人删掉它而不是静默跳过 */
const squareTile = (svg, file) => {
  const squared = svg.replace(/\s+rx="[^"]*"/, '');
  if (squared === svg) throw new Error(`${file} 里没有 rx 了 —— maskable 那步"抹平圆角"应当连同本函数一起删`);
  return squared;
};

const rasterize = async (page, svg, px, label) => {
  const dataUrl = `data:image/svg+xml;base64,${Buffer.from(withPixelSize(svg, px), 'utf8').toString('base64')}`;
  const base64 = await page.evaluate(async ({ url, size }) => {
    const img = new Image();
    img.src = url;
    await img.decode();
    const canvas = document.createElement('canvas');
    canvas.width = size;
    canvas.height = size;
    const ctx = canvas.getContext('2d');
    ctx.clearRect(0, 0, size, size);
    ctx.drawImage(img, 0, 0, size, size);
    return canvas.toDataURL('image/png').split(',')[1];
  }, { url: dataUrl, size: px });
  const png = Buffer.from(base64, 'base64');
  const dims = pngDims(png, label);
  // 尺寸不符就说明光栅化那步被什么改变了（浏览器版本、DPR、canvas 行为），
  // 静默写出一枚 190×190 比抛错糟得多——声明处还在写 192。
  if (dims.width !== px || dims.height !== px) throw new Error(`${label}：要 ${px}×${px}，实到 ${dims.width}×${dims.height}`);
  return png;
};

const run = async () => {
  const markSvg = fs.readFileSync(path.join(publicDir, MARK_FILE), 'utf8');
  const tileSvg = fs.readFileSync(path.join(publicDir, TILE_FILE), 'utf8');

  const markStamp = brandGeometryStamp(markSvg, MARK_FILE);
  const tileStamp = brandGeometryStamp(tileSvg, TILE_FILE);
  if (markStamp !== tileStamp) {
    throw new Error(
      `${MARK_FILE} 与 ${TILE_FILE} 的几何已经分叉，生成器不知道该按哪一份出图。\n` +
        `  真相源是 src/components/apple/ApBrand.vue，请把两枚 SVG 改回与它同一套 path/圆，再重跑。`,
    );
  }

  const browser = await chromium.launch({ executablePath: findChromium() });
  try {
    // DPR 钉在 1：canvas 尺寸由像素显式给定，别让系统缩放把 512 变成 640 之类的整数倍
    const page = await browser.newPage({ viewport: { width: 600, height: 600 }, deviceScaleFactor: 1 });
    const written = [];

    for (const target of PNG_TARGETS) {
      const source = target.source === TILE_FILE ? tileSvg : markSvg;
      const svg = target.fullBleed ? squareTile(source, target.source) : source;
      const png = addPngText(await rasterize(page, svg, target.px, target.file), PROVENANCE_KEYWORD, markStamp, target.file);
      fs.writeFileSync(path.join(publicDir, target.file), png);
      written.push([target.file, png.length, `${target.px}×${target.px}`]);
    }

    const entries = [];
    for (const size of ICO_SIZES) {
      const png = addPngText(await rasterize(page, markSvg, size, `favicon.ico ${size}`), PROVENANCE_KEYWORD, markStamp, `favicon.ico ${size}`);
      entries.push({ size, png });
    }
    const ico = buildIco(entries);
    fs.writeFileSync(path.join(publicDir, 'favicon.ico'), ico);
    written.push(['favicon.ico', ico.length, ICO_SIZES.map((s) => `${s}×${s}`).join(' + ')]);

    for (const [file, bytes, dims] of written) {
      console.log(`✓ public/${file}  ${String(bytes).padStart(6)} 字节  ${dims}`);
    }
    console.log(`  几何戳（写进每枚位图的 tEXt ${PROVENANCE_KEYWORD}）：${markStamp.slice(0, 16)}…`);
    console.log('  声明处：index.html 的 alternate icon、vite.config.ts 的 manifest.icons；两处都由门禁对回真实尺寸。');
  } finally {
    await browser.close();
  }
};

run().catch((err) => {
  console.error(`✗ 生成失败：${err instanceof Error ? err.message : String(err)}`);
  process.exit(1);
});
