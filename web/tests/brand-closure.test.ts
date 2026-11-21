import { describe, it, expect } from 'vitest';
import fs from 'node:fs';
import path from 'node:path';
import { APP_NAME } from '@/utils/core/app-info';
import { PROVENANCE_KEYWORD, brandGeometryStamp, parseIco, pngDims, readPngText } from '../scripts/lib/brand-icon-bytes.mjs';
import { sourceFiles, stripComments } from './source-scan';

// 这台的射程：**产品名在前端只准写一处，且不许被当成译文的一部分**。
//
// 为什么需要它：改名之后 "Citebase" 在前端被抄回六处（app-info.ts 的 APP_NAME、
// router/guards.ts 的 APP_TITLE、ApBrand 的 wordmark 默认值、LoginView 的 title、
// SpaceMobileHeader 的默认值、ShareView 与邮件预览里那句 "Powered by Citebase"），
// 而语言包还把品牌名写在译文里（en 的 'Citebase knowledge base' / zh 的 'Citebase 知识库管理系统'）——
// 结果是"改一次名要动七个文件 + 两套语言包"，而漏掉哪一处没有任何东西会发现。
//
// 三条判据分别拦三种复发：
//   ① 代码里不许再出现独立的产品名（组件名 CitebaseXxxView 这种标识符不算，那是命名法不是文案）；
//   ② 语言包的译文里不许内嵌产品名，必须走 {app} 参数；
//   ③ 带 {app} 的键在每个调用点都必须真的传了 app —— 漏传的表现是界面上出现字面量 "{app}"，
//      这一类我这次改动最容易亲手造出来（消息里插值、调用点忘传参）。
//
// 反向验证：往任一 .vue 塞一句 `const label = 'Citebase'` → 第一条红；
// 把 chrome.tagline 改回带产品名的写法 → 第二条红；
// 删掉某个调用点的 `{ app: ... }` → 第三条红。

/** 唯一允许写产品名的文件：默认名与读取路径本身 */
const BRAND_ALLOWLIST = ['src/utils/core/app-info.ts', 'src/utils/core/site-brand.ts'];

const BRAND = 'Citebase';

// 独立单词形态：前面不是路径分隔符、后面不是标识符字符。
// 这样 `./CitebaseGovernanceView.vue`（组件命名法）不会被误判，
// 而 `'Citebase'`、`Powered by Citebase`、`: 'Citebase 知识库'` 一个都跑不掉。
const STANDALONE_BRAND = new RegExp(`(?<![./\\w])${BRAND}(?![\\w])`);

function lineOf(src: string, index: number): number {
  return src.slice(0, index).split('\n').length;
}

describe('品牌名在前端只有一处落点', () => {
  it('除默认名与读取路径两个文件外，代码里不许出现独立的产品名', () => {
    const offenders: string[] = [];
    for (const rel of sourceFiles()) {
      if (BRAND_ALLOWLIST.includes(rel)) continue;
      const cleaned = stripComments(fs.readFileSync(path.resolve(rel), 'utf8'));
      const m = STANDALONE_BRAND.exec(cleaned);
      if (m) offenders.push(`${rel}:${lineOf(cleaned, m.index)}: ${cleaned.slice(m.index, m.index + 60).trim()}`);
    }
    expect(offenders).toEqual([]);
  }, 30000); // 扫全仓：默认 5s 在钩子并发满载时不够（实测 9s+），给显式预算

  it('两套语言包都不许把产品名写进译文', () => {
    // 品牌名进了译文，改名就得重译两套语言包；而且 zh 用户看到的"英文品牌"和
    // en 用户看到的其实是两个字符串，没法保证一致。
    const offenders: string[] = [];
    for (const pack of ['src/i18n/locales/en-US.ts', 'src/i18n/locales/zh-CN.ts']) {
      const src = stripComments(fs.readFileSync(path.resolve(pack), 'utf8'));
      src.split('\n').forEach((line, i) => {
        if (STANDALONE_BRAND.test(line)) offenders.push(`${pack}:${i + 1}: ${line.trim().slice(0, 70)}`);
      });
    }
    expect(offenders).toEqual([]);
  });

  it('每个用到 {app} 的语言包键，在所有调用点都拿到了 app 参数', () => {
    const packs = ['src/i18n/locales/en-US.ts', 'src/i18n/locales/zh-CN.ts'];
    const appKeys = new Set<string>();
    for (const pack of packs) {
      const src = fs.readFileSync(path.resolve(pack), 'utf8');
      let ns = '';
      src.split('\n').forEach((line) => {
        const nsMatch = /^ {2}(\w+): \{/.exec(line);
        if (nsMatch) {
          ns = nsMatch[1] as string;
          return;
        }
        const keyMatch = /^ {4}(\w+):/.exec(line);
        if (keyMatch && line.includes('{app}')) appKeys.add(`${ns}.${keyMatch[1]}`);
      });
    }
    expect(appKeys.size).toBeGreaterThan(0); // 判据空转比不过更糟：一个键都没有就是它自己坏了

    const unsupplied: string[] = [];
    const used: string[] = [];
    for (const rel of sourceFiles()) {
      const src = stripComments(fs.readFileSync(path.resolve(rel), 'utf8'));
      for (const m of src.matchAll(/(?:\$t|\bt)\(\s*'([\w.]+)'([^)]*)\)/g)) {
        const key = m[1] as string;
        if (!appKeys.has(key)) continue;
        used.push(key);
        // 认两种等价写法：`{ app: x }` 与对象简写 `{ app }`。只认前者会把简写
        // 判成漏传，逼着人写 `{ app: app }` 这种空转形式去迁就正则 ——
        // brand_hygiene 认 LEGACY 表体时写下过同一条规矩：为过门禁把代码写弯是本末倒置。
        if (!/[,{]\s*app\s*[:,}]|\bapp\s*:/.test(m[2] ?? '')) unsupplied.push(`${rel}: ${key}`);
      }
    }
    expect(unsupplied).toEqual([]);

    // 反向：语言包里有 {app} 却全站没人调，等于我改了消息忘了改调用点（或删了页面）。
    const neverCalled = [...appKeys].filter((k) => !used.includes(k));
    expect(neverCalled).toEqual([]);
  }, 30000); // 同上：第二遍扫全仓
});

/**
 * 静态外壳：构建期就定死、JS 起来之前（甚至没有 JS 时）被人看到的那几个文件。
 *
 * 上面三条判据的射程是 sourceFiles()，它只走 src/** —— 所以 index.html 的
 * `<title>` 与 meta description、vite.config.ts 的 PWA manifest、两枚图标的
 * aria-label 全在射程之外：前三条全绿时，它们照样能留着改名前的名字。
 * 2026-10-16 的品牌全面检查就是这么数出六份副本的。
 *
 * 判据是"等于 APP_NAME"而不是再抄一份拼写：改名时唯一落点一动，
 * 这四条立刻逐个点名，把人领到该改的文件前；而本文件自己的那份拼写
 * （上面的 BRAND）由第一条单独钉住，改名必须是一次有意的、被看到的编辑。
 */
const SHELL_FILES = ['index.html', 'vite.config.ts', 'public/favicon.svg', 'public/pwa-icon.svg'];

function shellText(rel: string): string {
  return stripComments(fs.readFileSync(path.resolve(rel), 'utf8'));
}

describe('静态外壳跟着唯一落点走', () => {
  it('判据自己那份拼写与 APP_NAME 一致', () => {
    expect(BRAND).toBe(APP_NAME);
  });

  it('index.html 的标签页标题等于默认名，meta description 以它开头', () => {
    const html = shellText('index.html');
    expect(html).toMatch(/<title>([^<]*)<\/title>/);
    expect(/<title>([^<]*)<\/title>/.exec(html)?.[1]).toBe(APP_NAME);
    const desc = /<meta name="description" content="([^"]*)"/.exec(html)?.[1];
    expect(desc, 'index.html 缺 meta description').toBeDefined();
    // 描述正文是站点自己的语言（lang=zh-CN），这里只钉名字那一段同源 ——
    // 后缀怎么写是文案，名字写错是品牌。
    expect(desc?.startsWith(`${APP_NAME} `)).toBe(true);
  });

  it('PWA manifest 的 name / short_name 等于默认名', () => {
    const src = shellText('vite.config.ts');
    expect(/(?<![A-Za-z_])name:\s*'([^']*)'/.exec(src)?.[1], 'manifest.name').toBe(APP_NAME);
    expect(/short_name:\s*'([^']*)'/.exec(src)?.[1], 'manifest.short_name').toBe(APP_NAME);
  });

  it('两枚图标的无障碍名等于默认名', () => {
    for (const rel of ['public/favicon.svg', 'public/pwa-icon.svg']) {
      const aria = /aria-label="([^"]*)"/.exec(shellText(rel))?.[1];
      expect(aria, `${rel} 缺 aria-label`).toBe(APP_NAME);
    }
  });

  it('静态外壳里不许出现旧品牌名', () => {
    const offenders = SHELL_FILES.filter((rel) => shellText(rel).toLowerCase().includes('aiwiki'));
    expect(offenders).toEqual([]);
  });
});

/**
 * 图标链：`ApBrand.vue` 是唯一几何源，位图与声明都只是它的派生物。
 *
 * 为什么单独一组判据（2026-11-01）：上面那组的射程是**文本**，而图标链坏掉的恰恰是
 * 二进制那一头 —— `public/favicon.ico` 是 initial commit 带进来的改名前旧标记，改名后
 * 没人重生成（浏览器却正好用它画固定标签页、地址栏与 Windows 快捷方式），
 * 而 `vite.config.ts` 还把它声明成 64×64（文件里实际只有一条 32×32）。
 * 两件事都没有任何文本判据数得到：SVG 侧全绿，标签页上仍是旧图标。
 *
 * 四条判据分别拦四种复发：
 *   ① 声明引用了不存在的文件 / public 下留着没人引用的位图（改名改一半）；
 *   ② `sizes:` 与文件真实像素不符（**这次的原始故障**：32×32 的文件被声明成 64×64）；
 *   ③ `.ico` 目录里声明的宽高与 PNG 载荷不符（Windows 按目录挑尺寸，谎在文件内部）；
 *   ④ 位图内嵌的几何戳与 ApBrand.vue 对不上（改了 SVG 却没重跑生成器）。
 * ④ 靠的是生成器往每枚 PNG 的 tEXt 里写一条几何 sha256（ICO 的每条载荷同样带）——
 * 于是"派生物过期"这件事在不开浏览器、不做图像相似度比对的前提下也能判。
 *
 * 反向验证：把 favicon.ico 换回 git 里那份旧的 → ②③ 三条一起红；
 * 手改 manifest 里任一条 sizes → ② 红；改 ApBrand.vue 的 path 不重跑生成 → ④ 红。
 */
const ICONS_DIR = 'public';
/** 生成器该产出的那三条 entry（多尺寸是 .ico 的意义本身，只剩一条就是又退回旧状态了） */
const ICO_ENTRY_SIZES = ['16x16', '32x32', '48x48'];

type IconRef = { src: string; sizes?: string; type?: string; purpose?: string };

const publicPath = (href: string) => path.resolve(ICONS_DIR, href.replace(/^\//, ''));

/** vite.config.ts 里 manifest.icons 的每一项（注释已被 shellText 剥掉，数不到说理文字） */
function manifestIcons(): IconRef[] {
  const arr = /icons:\s*\[([\s\S]*?)\]/.exec(shellText('vite.config.ts'))?.[1];
  expect(arr, 'vite.config.ts 里找不到 manifest.icons 数组 —— 数组写法变了就要同步这台门禁的解析').toBeTruthy();
  return [...(arr as string).matchAll(/\{[^{}]*\}/g)].map((m) => {
    const field = (name: string) => new RegExp(`${name}:\\s*'([^']*)'`).exec(m[0])?.[1];
    return { src: field('src') ?? '', sizes: field('sizes'), type: field('type'), purpose: field('purpose') };
  });
}

/** index.html 里的 <link rel="…icon…"> */
function htmlIconLinks(): IconRef[] {
  const html = shellText('index.html');
  return [...html.matchAll(/<link rel="[^"]*\bicon\b[^"]*"\s+href="([^"]*)"([^>]*)>/g)].map((m) => {
    const field = (name: string) => new RegExp(`${name}="([^"]*)"`).exec(m[2] ?? '')?.[1];
    return { src: m[1] ?? '', sizes: field('sizes'), type: field('type') };
  });
}

describe('图标链：位图与声明都是 ApBrand.vue 那套几何的派生物', () => {
  it('两处声明引用的图标文件都在 public/ 下真实存在，public/ 里也不留没人引用的位图', () => {
    const refs = [...new Set([...htmlIconLinks(), ...manifestIcons()].map((i) => i.src).filter(Boolean))];
    // 判据空转比不过更糟：一处声明都数不到就说明解析坏了，下面两条会一起假绿。
    expect(refs.length, 'index.html 与 manifest 里一条图标声明都没数到').toBeGreaterThanOrEqual(4);

    const missing = refs.filter((src) => !fs.existsSync(publicPath(src)));
    expect(
      missing,
      `声明引用了不存在的图标：${missing.join(', ')}。位图请跑 cd web && npm run gen:brand-icons 生成，` +
        '文本图标放 web/public/ 下，或把声明改回真实文件（声明处：index.html 的 <link> 与 vite.config.ts 的 manifest.icons）',
    ).toEqual([]);

    const referenced = new Set(refs.map((src) => src.replace(/^\//, '')));
    const orphans = fs
      .readdirSync(path.resolve(ICONS_DIR))
      .filter((f) => /\.(png|ico)$/.test(f) && !referenced.has(f));
    expect(
      orphans,
      `public/ 里留着没被任何声明引用的位图：${orphans.join(', ')}。` +
        '要么它该进 manifest / index.html，要么它就该删 —— 留着的下一位接手的人会以为它还在用（这次的旧 favicon.ico 就是这么活下来的）',
    ).toEqual([]);
  });

  it('manifest 每条 PNG 的 sizes 等于文件真实像素，且装得上主屏（192+ / maskable 512+）', () => {
    const sized = manifestIcons()
      .filter((i) => i.src.endsWith('.png'))
      .map((i) => ({ ...i, dims: pngDims(fs.readFileSync(publicPath(i.src)), i.src) }));
    expect(sized.length, 'manifest 里一条位图都没有：Android 装不上（它不认 SVG），这条链等于没接').toBeGreaterThan(0);

    const lying = sized.filter((i) => i.sizes !== `${i.dims.width}x${i.dims.height}`).map((i) => `${i.src} 声明 ${i.sizes}，实为 ${i.dims.width}x${i.dims.height}`);
    expect(
      lying,
      `sizes 与真实像素不符：${lying.join('；')}。位图是 scripts/gen-brand-icons.mjs 的产物 —— ` +
        '要改尺寸就改那份清单重跑生成，别只改 vite.config.ts 里的声明（那正是这次的故障形态）',
    ).toEqual([]);

    const opaque192 = sized.some((i) => i.purpose !== 'maskable' && i.dims.width >= 192 && i.dims.height >= 192);
    expect(opaque192, 'manifest 里没有 ≥192 的常规位图：Android 的安装门槛是 192+，低于它就装不出图标').toBe(true);
    const maskable512 = sized.some((i) => i.purpose === 'maskable' && i.dims.width >= 512 && i.dims.height >= 512);
    expect(maskable512, 'manifest 里没有 ≥512 的 maskable 位图：系统遮罩会把它放大裁切，桌面图标就糊了').toBe(true);
  });

  it('favicon.ico 的目录、载荷与两处声明三方一致', () => {
    const ico = parseIco(fs.readFileSync(publicPath('/favicon.ico')), 'favicon.ico');
    const inside = ico.entries.map((e) => `${e.width}x${e.height}`).sort();
    expect(inside, `favicon.ico 里的 entry 尺寸集是 ${inside.join(', ')}，应为 ${ICO_ENTRY_SIZES.join(', ')}（生成器 scripts/gen-brand-icons.mjs 的 ICO_SIZES）`).toEqual(
      ICO_ENTRY_SIZES,
    );

    // 目录段与载荷不符 = Windows 按目录去挑尺寸，挑到的却是一枚别的像素的图
    const dirLied = ico.entries
      .filter((e) => e.declaredWidth !== e.width || e.declaredHeight !== e.height)
      .map((e) => `目录 ${e.declaredWidth}x${e.declaredHeight}，载荷 ${e.width}x${e.height}`);
    expect(dirLied, `favicon.ico 内部自相矛盾：${dirLied.join('；')} —— 重跑 npm run gen:brand-icons`).toEqual([]);

    const declared = [
      ['vite.config.ts manifest.icons', manifestIcons().find((i) => i.src.endsWith('.ico'))],
      ['index.html <link rel="alternate icon">', htmlIconLinks().find((l) => l.src.endsWith('.ico'))],
    ] as const;
    for (const [where, ref] of declared) {
      expect(ref, `${where} 里没有 .ico 那一条声明了（alternate icon 是 SVG 进不去那几个面的唯一入口）`).toBeDefined();
      const sizes = String(ref?.sizes ?? '').split(/\s+/).filter(Boolean).sort();
      expect(sizes, `${where} 给 favicon.ico 写的 sizes 是「${ref?.sizes}」，文件里真实是 ${ICO_ENTRY_SIZES.join(' ')}` ).toEqual(ICO_ENTRY_SIZES);
    }
  });

  it('每枚位图（含 .ico 每条载荷）内嵌的几何戳等于 ApBrand.vue 那套几何', () => {
    const stamp = brandGeometryStamp(fs.readFileSync(path.resolve('src/components/apple/ApBrand.vue'), 'utf8'), 'ApBrand.vue');
    for (const rel of ['public/favicon.svg', 'public/pwa-icon.svg']) {
      expect(
        brandGeometryStamp(fs.readFileSync(path.resolve(rel), 'utf8'), rel),
        `${rel} 与 src/components/apple/ApBrand.vue 的几何已经分叉 —— 组件是唯一真相源，先把它对齐到 SVG（或反之），再跑 npm run gen:brand-icons`,
      ).toBe(stamp);
    }

    const rasters: [string, string | undefined][] = [];
    for (const src of [...new Set([...htmlIconLinks(), ...manifestIcons()].map((i) => i.src).filter(Boolean))]) {
      if (!fs.existsSync(publicPath(src))) continue; // 存在性由第一条点名，这里不重复报
      const buf = fs.readFileSync(publicPath(src));
      if (src.endsWith('.png')) rasters.push([src, readPngText(buf, PROVENANCE_KEYWORD, src)]);
      if (src.endsWith('.ico'))
        parseIco(buf, src).entries.forEach((e, i) => rasters.push([`${src} 第 ${i + 1} 条 ${e.width}x${e.height}`, e.provenance]));
    }
    expect(rasters.length, '一枚位图都没数到：生成物全没了或被改名了').toBeGreaterThanOrEqual(6);

    const drifted = rasters.filter(([, got]) => got !== stamp).map(([file, got]) => `${file}：${got ?? '没有几何戳'}`);
    expect(
      drifted,
      `这些位图不是当前几何生成的：${drifted.join('；')}。` +
        '跑 cd web && npm run gen:brand-icons 重生成（几何戳由 scripts/lib/brand-icon-bytes.mjs 从 path/圆参数算出，与 ApBrand.vue 同一套判据）',
    ).toEqual([]);
  });
});
