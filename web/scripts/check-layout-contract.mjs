#!/usr/bin/env node
// 布局契约门禁：校验 docs/engineering/frontend-conventions.md「Layout Contract / Page Container」里那套约定。
// check-style-tokens.mjs 只管 token 注册与暗色覆盖，管不到"页面用没用容器、
// 版心有没有显式声明、留白叠了几层"这类结构问题——那些过去只能靠 review，于是持续漂移。
// 规则全部按当前代码基线写，任何新增违例都会让 npm run check:style 失败。
import fs from 'fs';
import path from 'path';

const WEB = path.resolve(process.argv[2] || '.');
const SRC = path.join(WEB, 'src');
const ROUTER = ['routes-main.ts', 'routes-admin.ts', 'routes-public.ts'];

const problems = [];
const add = (file, hint, why) => problems.push({ file, hint, why });

const read = p => fs.readFileSync(p, 'utf8');
const rel = p => path.relative(SRC, p).replace(/\\/g, '/');

/** 遍历 src 下的 .vue/.ts（跳过测试与 .d.ts），供跨文件结构规则使用 */
function* walkFiles(dir) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) yield* walkFiles(p);
    else if (/\.(vue|ts)$/.test(e.name) && !p.endsWith('.d.ts')) yield p;
  }
}

// ---------- 收集路由页面（只有路由挂的才算"页面"） ----------
function routedPages() {
  const pages = new Map();
  for (const f of ROUTER) {
    const t = read(path.join(SRC, 'router', f));
    for (const m of t.matchAll(/component:\s*\(\)\s*=>\s*import\(\s*['"]([^'"]+)['"]\s*\)/g)) {
      const abs = path.join(SRC, m[1].replace(/^@\//, ''));
      if (!fs.existsSync(abs)) continue;
      const before = t.slice(Math.max(0, m.index - 700), m.index);
      const rt = [...before.matchAll(/path:\s*'([^']*)'/g)].pop();
      if (!pages.has(rel(abs))) pages.set(rel(abs), rt ? rt[1] : '?');
    }
  }
  return pages;
}

// 表格主导页留在窄版心的显式白名单（页面主体是工作台，表格只是其中一块）
// （GitSyncView 白名单随 Git 同步下线一并移除）
const TABLE_NARROW_ALLOW = new Map([
  ['views/search/SearchOpsView.vue', '词库/试跑/索引运维三段工作台，非全表 CRUD'],
]);

const pages = routedPages();
let checked = 0;

for (const [file, route] of pages) {
  const text = read(path.join(SRC, file));
  const open = /<ApPageContainer([^>]*)>/s.exec(text);
  checked++;

  // R1 页面必须使用 ApPageContainer
  if (!open) {
    add(file, `${route} 没有用 <ApPageContainer> 作根容器`, '版心/页头/面包屑都由容器统一负责');
    continue;
  }
  const tag = open[1];
  // 裸属性（compact / no-scroll）与绑定属性（:max-width）都要认到；
  // 只认冒号前缀会把裸属性全漏掉
  const has = name => new RegExp('(?:^|[\\s:@])' + name + '(?:\\s*=|[\\s>]|$)').test(tag);
  const valOf = name => {
    const m = new RegExp('[:@]?' + name + '\\s*=\\s*"([^"]*)"').exec(tag);
    return m ? m[1] : undefined;
  };

  const fullBleed = has('no-scroll');

  // R2 版心必须显式（全屏页例外：容器对 no-scroll 且未声明者按全宽处理）
  const mw = valOf('max-width');
  if (!has('max-width')) {
    if (!fullBleed) add(file, `${route} 未声明 :max-width，会静默继承容器默认 1600`, '显式写 AppleLayout.maxContentWidth / maxWideContentWidth / none');
  } else if (!/AppleLayout\.(maxContentWidth|maxWideContentWidth)$|^none$/.test(mw || '')) {
    add(file, `${route} 的 :max-width="${mw}" 不在三档之内`, '只允许 1200 / 1600 / none 三档，自造宽度会绕过版心约定');
  }

  // R3 表格主导页必须 1600
  const tableLed = /<ApTable|<AdminCrudPage/.test(text);
  const narrow = /maxContentWidth/.test(mw || '');
  if (tableLed && narrow && !TABLE_NARROW_ALLOW.has(file)) {
    add(file, `${route} 是表格/CRUD 页却限 1200`, '列表页用 maxWideContentWidth；确属工作台请加入 TABLE_NARROW_ALLOW 并写明理由');
  }

  // R4 非全屏页必须 compact（左右留白 16px，与非 compact 的 20~40px 不混用）
  if (!fullBleed && !has('compact')) add(file, `${route} 缺 compact`, '同组列表页的留白要靠它对齐');

  // R6 视口高度只允许 100dvh
  const vh = text.match(/[^d]100vh|calc\(\s*100vh/g);
  if (vh) add(file, `${route} 出现 ${vh.join(' / ')}`, '用 100dvh，避免移动端地址栏抖动');

  // R7 允许 <ApBreadcrumb>/<DocBreadcrumb（两者共用同一套样式），只禁"页面手搓面包屑标记"
  // R7 页面级面包屑只能有一套实现；结果列表里的"空间 / 路径"片段（用 / 分隔）不算面包屑
  const own = text.replace(/:show-breadcrumb|breadcrumb-parent|breadcrumbParent|ApBreadcrumb|DocBreadcrumb/g, '');
  if (/›/.test(own) && /const\s+\w*[Bb]readcrumb\w*Style\s*=/.test(own)) {
    add(file, `${route} 自己画了面包屑`, '用 :show-breadcrumb（必要时配 breadcrumb-parent）或 <ApBreadcrumb>，样式走 page-styles 的共享来源');
  }
}

// R8 面包屑只准有一处实现，而且必须带着量测锚点。
// 合并前容器另写了一份扁平 `<nav>`（没有 ol/li、没有 aria-current），ApBreadcrumb 的链接又都没有
// href（不进 Tab 序）—— 两边各对一半，所以现在：样式与渲染都在 ApBreadcrumb，容器只派生层级。
const CRUMB = 'components/apple/ApBreadcrumb.vue';
const CONTAINER = 'components/apple/ApPageContainer.vue';
const crumbText = read(path.join(SRC, CRUMB));
for (const n of ['breadcrumbRowStyle', 'breadcrumbLinkStyle', 'breadcrumbCurrentStyle', 'breadcrumbSepStyle']) {
  if (!crumbText.includes(n)) add(CRUMB, `没有引用共享面包屑样式 ${n}`, '样式集中在 styles/layout/page-styles.ts，两处导航才长得一样');
}
if (!/data-ap="breadcrumb"/.test(crumbText)) {
  add(CRUMB, '面包屑 nav 没挂 data-ap="breadcrumb" 锚点', 'parity 与 check:ui 优先按这个锚点找它；锚点没了会静默退回历史选择器，"没量到"和"没问题"打印出来一模一样（R15 同一条教训）');
}
const shellText = read(path.join(SRC, CONTAINER));
if (!/<ApBreadcrumb\b/.test(shellText)) {
  add(CONTAINER, '容器不再用 <ApBreadcrumb> 渲染面包屑', '容器自己写 nav 就等于回到两套 DOM');
}
if (/breadcrumbRowStyle/.test(shellText)) {
  add(CONTAINER, '容器又自己引了面包屑样式', '节奏/字重/gap 只准由 ApBreadcrumb 负责，容器只给 items');
}

// R9 正文页必须套 680 行度量
for (const f of ['views/share/ShareView.vue', 'views/admin/system/SSODocView.vue']) {
  if (!read(path.join(SRC, f)).includes('readingAreaStyle')) add(f, '正文没有复用 readingAreaStyle', '长文行宽用共享度量，不要随版心铺满');
}

// R10 网格地板只准取自 AppleGridFloor 四档（tile/stat/entity/panel）。
// 历史上 src 里散落着 15 种 minmax 地板（120…520），同类卡片在不同页面列数不同、宽窄不同，
// 这就是"内容区排版不和谐、空间卡比文档卡还小"的直接来源。
const FLOOR_FILE = 'utils/core/apple-tokens.ts';
// 非卡片网格的合法 minmax 首参：主从布局的侧列宽度、固定列数里的 0（让列可缩到内容宽以下）。
const FLOOR_EXEMPT = new Map([
  ['views/admin/system/SettingsView.vue', '主从布局左列 minmax(160px,200px)，不是卡片地板'],
]);
for (const f of walkFiles(SRC)) {
  const r = rel(f);
  if (r === FLOOR_FILE || r.endsWith('.test.ts')) continue;
  const text = read(f).replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '));
  // 旧正则写成 minmax\(\s*min?\(?\s*(\d+)px，其中 min? 是「mi」+可选 n，于是只匹配
  // minmax(min(200px,…) 这一种形态，minmax(230px,1fr) 这类纯数字地板全部静默放行。
  const raw = [...text.matchAll(/minmax\(\s*(?:min\(\s*)?(\d+)px/g)]
    .map((m) => Number(m[1]))
    .filter((n) => n > 0);
  if (raw.length && !FLOOR_EXEMPT.has(r)) {
    add(r, `手写 minmax(${raw.join('/')}px) 当地板`, '改用 AppleGridFloor.tile|stat|entity|panel（四档之外即漂移）；确属非卡片网格请进 FLOOR_EXEMPT 写理由');
  }
}

// R17 固定列数网格必须走 appleFixedColumns()/appleFixedGrid()（R10 的补集）。
// R10 的正则要 `minmax(` 后面紧跟数字+px，于是 `repeat(2, minmax(0, 1fr))` 这一族**整类看不见**
// （0 被 n>0 滤掉，`0,` 也不匹配 `(\d+)px`）—— 七处里六处是这一族，全靠人眼发现。
// 编号叫 R17 而不是 R10b：条数由 `^// R<数字>␣` 数出来，带字母前缀的标题不会被数到，
// 那样"10 条约定"的打印就会少报一条真实存在的门禁。
// 现在按引号里的字面量抓：写了 'repeat(N, minmax(' 却没走工厂即违例。
const FIXED_GRID_ALLOW = new Map([
  ['views/admin/dashboard/components/DashboardHeatmap.vue', '贡献热力图：7 列固定（一周七天）+ 单元格最小 44px，不是等分列'],
]);
for (const f of walkFiles(SRC)) {
  const r = rel(f);
  if (r === FLOOR_FILE || r.endsWith('.test.ts') || FIXED_GRID_ALLOW.has(r)) continue;
  const text = read(f);
  for (const m of text.matchAll(/["'`]repeat\(\s*\d+\s*,\s*minmax\(/g)) {
    add(r, `第 ${text.slice(0, m.index).split('\n').length} 行手写固定列网格（R17）`, '改用 appleFixedColumns(n) / appleFixedGrid(n, gap)；确属非等分列请进 FIXED_GRID_ALLOW 写理由');
  }
}

// R11 entity 地板的卡片网格必须等高：grid-auto-rows:1fr（或走 appleCardGrid()）。
// 卡片自己是 flex 列（ApCard/ApEntityCard），等高之后同行元信息才落在同一基线。
// 判据必须落在**同一个样式对象里**：上一版取"上下 ±6/+8 行的文本窗口"，于是隔壁行的注释
// 里出现 gridAutoRows 三个字就算通过 —— 那是文本假绿，不是结构判据。注释也先抹掉。
const stripComments = (t) => t
  .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
  .replace(/(^|[^:])\/\/[^\n]*/g, (_m, p) => p + ' '.repeat(_m.length - p.length));
const enclosingObject = (text, idx) => {
  let depth = 0;
  let start = -1;
  for (let i = idx; i >= 0; i--) {
    const c = text[i];
    if (c === '}') depth++;
    else if (c === '{') { if (depth === 0) { start = i; break; } depth--; }
  }
  if (start < 0) return '';
  let open = 0;
  for (let i = start; i < text.length; i++) {
    const c = text[i];
    if (c === '{') open++;
    else if (c === '}' && --open === 0) return text.slice(start, i + 1);
  }
  return text.slice(start);
};
// 模板插值 `${...}` 的花括号不是对象：不掩掉的话，`repeat(auto-fill, ${AppleGridFloor.entity})`
// 里那个 `${` 会被当成"最近的未闭合 {"，取到的"对象"就只有 `${…}` 那 20 个字符，
// 于是把 8 处**已经写了 gridAutoRows** 的网格全判成违例（等长替换，行号与下标都不动）。
const maskInterp = (t) => t.replace(/\$\{[^{}]*\}/g, (m) => '$$' + m.slice(2, -1) + ' ');

// R5 布局层不得再加左右留白。
// 判据取**同一个样式对象**（花括号配对），不取文本窗口：原先是"contentStyle 之后 600 字符里
// 必须出现 padding: '0'"，2026-10-15 顶栏浮层化往 contentStyle 里加了几行注释与
// scroll-padding-top，窗口被撑破，门禁立刻假红 —— 而它想管的其实只有**横向**留白。
// 纵向让位（padding-top 给浮层顶栏）是另一件事，这里显式放行并说明为什么。
const ml = read(path.join(SRC, 'components/layout/MainLayout.vue'));
const mlObj = enclosingObject(ml, ml.indexOf('{', ml.indexOf('const contentStyle')) + 1);
if (/padding:\s*[^'"\n]*AppleSpace/.test(mlObj) || /padding:\s*`/.test(mlObj)) {
  add('components/layout/MainLayout.vue', '布局层的 contentStyle 又加了左右留白', '留白只允许一层负责：交回 ApPageContainer');
}
if (!/padding:\s*'0'/.test(mlObj)) {
  add('components/layout/MainLayout.vue', 'contentStyle 的 padding 简写不再恒为 0', '左右留白单层负责（见布局契约）');
}
if (/paddingLeft|paddingRight/.test(mlObj)) {
  add('components/layout/MainLayout.vue', 'contentStyle 出现了横向 padding', '左右留白单层负责：交回 ApPageContainer');
}
// 纵向只允许"给浮层顶栏让位"这一种：值必须是壳高变量，别的数一律是新的第二层留白
for (const m of mlObj.matchAll(/paddingTop:\s*([^,\n}]+)/g)) {
  if (!/apple-shell-bar-height/.test(m[1]) && !/'0'|"0"|0\b/.test(m[1])) {
    add('components/layout/MainLayout.vue', `contentStyle 的 paddingTop 不是壳高让位：${m[1].trim()}`,
      '纵向让位只准等于浮层顶栏的高度，否则就是第二层留白');
  }
}

for (const f of walkFiles(SRC)) {
  const r = rel(f);
  if (r === FLOOR_FILE || r.endsWith('.test.ts')) continue;
  const text = maskInterp(stripComments(read(f)));
  for (const m of text.matchAll(/AppleGridFloor\.entity/g)) {
    const obj = enclosingObject(text, m.index);
    if (!/gridAutoRows|appleCardGrid\(/.test(obj)) {
      add(r, `第 ${text.slice(0, m.index).split('\n').length} 行用了 entity 地板却没设 gridAutoRows`, '卡片网格要等高：同一个样式对象里补 gridAutoRows: 1fr，或直接 appleCardGrid()');
    }
  }
}

// R12 壳层表面必须走阶梯：顶栏/侧栏 = surface-chrome，页面底 = surface-canvas。
// 之前 nav 用 glass-bg-nav、侧栏用 surface-panel、内容用 bg-primary，三个互不相干的角色，
// 主题改一档不会传导到另外两档，于是"内容区和上栏、侧栏颜色不一致"。
const SHELL = {
  'components/apple/ApNavBar.vue': ['surface-chrome'],
  'components/apple/ApSidebar.vue': ['surface-chrome'],
  'components/layout/MainLayout.vue': ['surface-canvas'],
  'components/apple/ApPageContainer.vue': ['surface-canvas'],
};
// 顶栏可以走共享工具类 .apple-glass-nav（底色在令牌层里取 surface-chrome）。
// 判据必须认得这层间接：否则唯一正确的写法（同一档颜色只有一个落点）会被判成违规，
// 逼人把 cv('surface-chrome') 再抄回组件里 —— 那正是 R12 想消灭的"同一层颜色两条命名路径"。
// 但只认"文件里出现了类名"不够：那类名指向什么都没人管，所以还要在令牌层里核到它确实取这一档。
const tokenLayer = read(path.join(SRC, 'utils/core/apple-tokens.ts'));
const navClassIsChrome = /\.apple-glass-nav\s*\{[^}]*var\(--apple-surface-chrome\)/.test(tokenLayer);
for (const [f, tokens] of Object.entries(SHELL)) {
  const text = read(path.join(SRC, f));
  for (const t of tokens) {
    const viaClass = t === 'surface-chrome' && /apple-glass-nav/.test(text);
    if (viaClass && !navClassIsChrome) {
      add(f, '顶栏挂的 .apple-glass-nav 在令牌层里并不取 surface-chrome',
        '工具类必须是壳层档的唯一落点，否则这条间接就是新的绕过路径');
      continue;
    }
    if (!text.includes(`cv('${t}')`) && !viaClass) {
      add(f, `壳层没有使用 cv('${t}')`, '壳层表面是主题契约的一档，绕开它主题就无法整体调配浅深（走 .apple-glass-nav 也算）');
    }
  }
  if (/cv\('glass-bg-nav'\)|cv\('surface-panel'\)|cv\('bg-primary'\)/.test(text) && f.includes('NavBar')) {
    add(f, '顶栏背景仍取旧角色', '顶栏走 surface-chrome（模糊强度另走 cv(\'glass-blur\')）');
  }
}

// R13 阶梯角色不许被壳层之外的文件复述：旧版 R12 只白名单读 4 个文件，
// 于是内容区里再画一条 chrome（移动端二级顶栏）、或再铺一层 canvas（父级已是 canvas）
// 都扫不到，主题调浅深时这几层不跟手。
const ROLE_OWNER = {
  // R14 早就把这三条横边钉在同一把尺上（顶栏 / 页脚 / 侧栏用户行共用 shellBarHeight），
  // 材质上它们也必须同档 —— 页脚此前是"壳层几何 + 零材质"，实测跨过它上沿亮度差 Δ0~1。
  'surface-chrome': new Set(['components/apple/ApNavBar.vue', 'components/apple/ApSidebar.vue', 'components/apple/ApFooter.vue']),
  'surface-canvas': new Set(['components/layout/MainLayout.vue', 'components/layout/BlankLayout.vue', 'components/apple/ApPageContainer.vue', 'components/common/ErrorBoundary.vue']),
};
// 受控例外：必须写理由，否则等于把规则又关回去。
const SURFACE_EXEMPT = new Map([
  ['views/workspace/SpaceView.vue', '阅读全屏时正文 position:fixed 脱离 ApPageContainer 的绘制框，父级 canvas 盖不到，只在 isFullscreen 分支补一层同角色底'],
  ['styles/layout/doc-sidebar-styles.ts', '文档树侧栏是阅读版自己的**壳层列**（与应用侧栏同一角色：独占一列、正文在它右侧滚），' +
    '按表面语法表它必须走 chrome 档 —— 否则同一屏里"侧栏"有两种材质（普查点名的头号不统一）。' +
    '它不是内容区里再造一条 chrome：全站壳层那三块仍只由 ROLE_OWNER 里的文件铺。'],
]);
for (const f of walkFiles(SRC)) {
  const r = rel(f);
  if (r === 'utils/core/apple-tokens.ts' || r === 'styles/theme/themes.ts' || r.endsWith('.test.ts')) continue;
  const text = read(f);
  for (const [role, owners] of Object.entries(ROLE_OWNER)) {
    if (owners.has(r) || SURFACE_EXEMPT.has(r)) continue;
    if (new RegExp(`cv\\('${role}'\\)`).test(text)) {
      add(r, `壳层之外复述了 ${role}`, `${role} 只属于全站壳层；内容区里的条/面板走 surface-panel，父级已是 canvas 就不要重复铺底；确需自铺请进 SURFACE_EXEMPT 写理由`);
    }
  }
}

// R14 壳层三条横边共用一把尺：顶栏 / 页脚 / 侧栏底部用户行必须引用 AppleLayout.shellBarHeight。
// 这三条边原先各自凑数（顶栏 60、用户行 49、页脚 47），谁改谁都不传导，边线对不齐就是"看着散"。
// 静态这层拦"有人又写了个高度"，运行时那层（visual-snapshot 的 shellBars）拦"写了 48 但渲染成别的"
// ——被 flex 压扁、被边框加高、被页面覆盖，只有量得出来。
const BAR_OWNER = {
  nav: ['components/apple/ApNavBar.vue', 'navStyle'],
  footer: ['components/apple/ApFooter.vue', 'footerStyle'],
  'user-row': ['components/apple/ApSidebar.vue', 'userSectionStyle'],
};
const TOKENS_FILE = 'utils/core/apple-tokens.ts';
const tokensText = read(path.join(SRC, TOKENS_FILE));
// 标尺只准定义一次，而且只准有一个名字。
// 原先 AppleLayout 里 navHeight 与 shellBarHeight 并存在（navHeight 是 60px 时代留下的历史名），
// 两条名字都"对"，于是顶栏引用一个、页脚引用另一个，改其一不改其二——这正是别名会养出的 drift。
const rulers = [...tokensText.matchAll(/const SHELL_BAR_HEIGHT = '([^']*)'/g)].map((m) => m[1]);
if (rulers.length !== 1) {
  add(TOKENS_FILE, `SHELL_BAR_HEIGHT 定义了 ${rulers.length} 次`, '一把尺才谈得上同高；重复定义等于各写各的');
} else {
  const m = /shellBarHeight:\s*(SHELL_BAR_HEIGHT|'[^']*')/.exec(tokensText);
  if (!m) add(TOKENS_FILE, 'AppleLayout.shellBarHeight 不见了', '三条横边共用的那把尺必须存在');
  else if (m[1] !== 'SHELL_BAR_HEIGHT') add(TOKENS_FILE, `AppleLayout.shellBarHeight 写了死数 ${m[1]}`, `必须复用 SHELL_BAR_HEIGHT（当前 ${rulers[0]}）`);
  if (/navHeight/.test(tokensText)) {
    add(TOKENS_FILE, 'AppleLayout/CSS 变量里又出现了 navHeight 这个历史别名', '一把尺一个名字：请统一用 shellBarHeight / --apple-shell-bar-height');
  }
  const barVar = /'(--apple-shell-bar-height[^']*)':\s*([^,\n}]+)/.exec(tokensText);
  if (!barVar || !/AppleLayout\.shellBarHeight/.test(barVar[2])) {
    add(TOKENS_FILE, '--apple-shell-bar-height 不再取自壳层标尺', 'CSS 变量与内联样式必须同源，否则 calc(100dvh - …) 排出来的高度跟量到的高度不是一回事');
  }
  // 运行时那条断言靠这个变量名取"期望高度"。只改一处名字，parseFloat 就给 NaN，
  // 而 Math.abs(h - NaN) > 1 恒为 false → 三条边全部静默通过。所以变量名要两头对齐校验。
  if (barVar) {
    const snapPath = path.join(WEB, 'scripts', 'visual-snapshot.mjs');
    if (fs.existsSync(snapPath)) {
      const snap = read(snapPath);
      if (!snap.includes(`getPropertyValue('${barVar[1]}')`)) {
        add('scripts/visual-snapshot.mjs', `壳层期望值仍在读别的变量（令牌里是 ${barVar[1]}）`, '改名要两头一起改；单头改名会让运行时断言拿到 NaN 而静默全绿');
      }
    }
  }
  // 兜底值是第二把尺的藏身处：<style> 块与注入样式串取不到 TS 常量，只能靠 CSS 变量，
  // 而 var(--x, 60px) 里的 60px 一旦是死数，变量缺失时就静默回到旧高度（这里曾经就是 60px）。
  for (const f of walkFiles(SRC)) {
    const r = rel(f);
    if (r.endsWith('.test.ts')) continue;
    const text = read(f).replace(/\/\*[\s\S]*?\*\//g, (c) => c.replace(/[^\n]/g, ' '));
    for (const c of text.matchAll(/var\(\s*--apple-shell-bar-height\s*,\s*([^)]*)\)/g)) {
      if (!/\$\{AppleLayout\.shellBarHeight\}/.test(c[1])) {
        add(r, `--apple-shell-bar-height 的兜底值写了 "${c[1]}"`, '兜底必须插值取自壳层标尺；写死数字等于藏第二把尺，变量缺失时三条边又错开');
      }
    }
  }
}
for (const [anchor, [file, styleName]] of Object.entries(BAR_OWNER)) {
  const text = read(path.join(SRC, file));
  // 运行时探针靠 data-ap-shell 找这三条边；锚点被改名，探针就查无此元素、静默假绿
  if (!text.includes(`data-ap-shell="${anchor}"`)) {
    add(file, `缺 data-ap-shell="${anchor}"`, 'visual-snapshot 的壳层同高断言用这个锚点定位，改名会让断言空跑');
  }
  // 三种写法都要认到：普通对象字面量以 `};` 结束，computed 样式以 `}));` 结束
  const block = new RegExp(`const ${styleName}[\\s\\S]*?\\n(?:\\};|\\}\\)\\);)`).exec(text);
  if (!block) {
    add(file, `找不到 ${styleName}`, '壳层横边的高度声明改名了，请同步更新本规则');
    continue;
  }
  const body = block[0];
  if (!new RegExp(`height:\\s*AppleLayout\\.shellBarHeight`).test(body)) {
    add(file, `${styleName} 的高度没走 AppleLayout.shellBarHeight`, '三条横边共尺：改这里就等于把这条边挪出同一水平线');
  }
  const raw = [...body.matchAll(/height:\s*`?'?(\d+(?:\.\d+)?)px/g)].map((m) => m[1] + 'px');
  if (raw.length) add(file, `${styleName} 里写了裸 px 高度 ${raw.join('/')}`, '用 AppleLayout.shellBarHeight');
}

// R15 页头/对象标题必须挂锚点。运行时工装（check:parity）只认 `[data-ap-page-title]` 找页头，
// 锚点一旦被改名或漏写，那台不是"报错"而是**查无元素、静默不量**——本项目门禁最常见的失效方式。
// 两种锚点二选一：page = 容器页头（或 PageHeader 那种页头槽覆盖）；object = 对象头部卡
// （空间详情头、阅读区正文标题），它的位置由内容决定，不该按页头尺子比，但同样要能被认出来。
const TITLE_ANCHOR_ALLOW = {
  'views/workspace/editor/DocumentEditView.vue': '编辑页故意没有可见页头：文档名只出现在面包屑，这枚 h1 是 sr-only 给辅助技术的',
  'views/workspace/SpaceView.vue': '这处 <h1> 在导出 HTML 的 JS 模板字符串里，不是应用 DOM',
};
for (const f of walkFiles(SRC)) {
  if (!f.endsWith('.vue')) continue;
  const r = rel(f);
  if (TITLE_ANCHOR_ALLOW[r]) continue;
  const text = read(f);
  for (const m of text.matchAll(/<h1\b([\s\S]{0,400}?>)/g)) {
    if (!/data-ap-(page|object)-title/.test(m[1])) {
      add(r, `第 ${text.slice(0, m.index).split('\n').length} 行的 <h1> 没挂 data-ap-page-title / data-ap-object-title`,
        '运行时页头量测靠这两个锚点找元素；漏写不会报错，只会静默量不到（"没量到"看起来和"没问题"一样）');
    }
  }
}

// R16 主对象档只准发给"这一页真正的主角"。静置卡片不投影是这套设计的地板，
// surface-primary + shadow-primary 是它唯一的例外；一旦谁都能用，它就成了第二个默认卡片底，
// 层级重新回到"靠阴影喊"，而这是前面九批费力拆掉的东西。
const PRIMARY_SURFACE_OWNER = 'utils/core/apple-tokens.ts';
const PRIMARY_SURFACE_ALLOW = {
  'components/space/reader/DocContentPanel.vue': '文档正文是阅读页唯一的主对象（编辑/预览两态共用这一张纸）',
  'views/share/ShareView.vue': '分享出去的那张纸就是本页的主角，没有别的内容',
};
for (const f of walkFiles(SRC)) {
  const r = rel(f);
  if (r === PRIMARY_SURFACE_OWNER) continue;
  const text = read(f);
  const uses = /primarySurfaceStyle\(/.test(text) || /cv\('surface-primary'\)|var\(--apple-surface-primary\)/.test(text);
  if (!uses) continue;
  if (!PRIMARY_SURFACE_ALLOW[r] && !r.startsWith('styles/theme/')) {
    add(r, '用了主对象档 surface-primary', '这一档只发给白名单里的"页面主对象"；新地方要用就先在这里登记理由，别默默扩散');
  }
}

// R18 `<ApTable>` 不许再写 `size=`。密度轴的 prop 已改名 `density`（与 `ApModal density` 对齐，
// 也让 `AppleDensity` 这条轴在组件库里只有一个名字）。
// 为什么要把"改完"这件事变成门禁：`ApTable` 没写 `inheritAttrs: false`，vue-tsc 会把认不出来的
// prop 当 fallthrough attribute 放行 —— 于是 `size="compact"` 残留时 **type-check 仍然 0 error**，
// 而那张表已经静默退回 default 密度。"编译通过"在这里不等于"接上了"，只能靠静态抓一把。
// 取标签本体时要按引号状态扫到第一个不在引号里的 `>`：直接按行数取窗口会把
// 表格里嵌套的 `<ApButton size="small">` 算进来（实测会造出几十条假违例）。
for (const f of walkFiles(SRC)) {
  const r = rel(f);
  if (!f.endsWith('.vue')) continue;
  const text = read(f);
  let from = 0;
  for (;;) {
    const start = text.indexOf('<ApTable', from);
    if (start < 0) break;
    let inQuote = null;
    let end = -1;
    for (let i = start + 1; i < text.length; i++) {
      const c = text[i];
      if (inQuote) { if (c === inQuote) inQuote = null; continue; }
      if (c === '"' || c === "'") { inQuote = c; continue; }
      if (c === '>') { end = i; break; }
    }
    if (end < 0) break;
    const tag = text.slice(start, end + 1);
    const m = /(^|[\s:])size=/.exec(tag);
    if (m) {
      add(r, `第 ${text.slice(0, start + m.index).split('\n').length} 行给 <ApTable> 传了 size=（R18）`,
        '密度轴的叫 density：<ApTable density="compact">。写 size 会被当成 fallthrough attr 静默忽略，表格退回默认密度');
    }
    from = end;
  }
}

// R19 单轴滚动容器必须走 scrollSingleAxis（两轴显式 + 竖向槽位常驻）。
// 只写 overflowY:'auto' 时，CSS 不允许一对轴里一个 visible 一个非 visible，横轴按 auto 处理；
// 于是条目文字顶出宽度就冒**横向滚动条**，横向条抢走约 15px 可视高度 → 竖向条出现/消失 →
// 内容宽度回弹 → 两条互相把对方挤出来，用户看到的就是"上下左右一起闪"（2026-10-15 首页侧栏报的正是这个）。
// 导航/菜单这类列表没有"横向平移"这个动作：溢出只能截断，不能平移。
// 判据落在**同一个样式对象里**并先抹注释 —— R11 那条文本窗口的教训（注释能把它骗过去）。
const SINGLE_AXIS_SCROLL = [
  'components/apple/ApSidebar.vue',
  'components/ai/chat/ChatSidebar.vue',
  'components/apple/ApSelect.vue',
];
for (const f of SINGLE_AXIS_SCROLL) {
  const abs = path.join(SRC, f);
  if (!fs.existsSync(abs)) { add(f, '登记在 R19 名单里的文件已不存在', '从名单里删掉：空登记会让下一个手搓滚动容器的文件顺手过关'); continue; }
  const raw = read(abs);
  if (!raw.includes('scrollSingleAxis')) {
    add(f, 'R19 名单里的文件不再引用 scrollSingleAxis',
      '要么并回共享写法，要么把它从名单里摘掉并说明为什么这个列表允许横向平移');
  }
  const text = maskInterp(stripComments(raw));
  for (const m of text.matchAll(/overflowY:\s*'?(?:auto|scroll)'?/g)) {
    const obj = enclosingObject(text, m.index);
    if (!obj.includes('scrollSingleAxis') && !/overflowX/.test(obj)) {
      add(f, '单轴滚动容器只声明了 overflowY（横轴会被 CSS 补成 auto）',
        '走 scrollSingleAxis，或显式写出 overflowX 与 scrollbarGutter');
    }
  }
}

/**
 * R19b：把上面那条从**登记表**改成**全 src 扫描**。
 * 按表遍历是这仓库第三次踩的同一个坑（MATERIALIZE_TODO 那次实测注入新消费者毫无反应，
 * 判据"永远绿"）：需要交代的对象恰恰是**还没进表**的那一个。
 * 实测全 src 有 37 处对象级 `overflowY` 裸写，一次判红不现实，所以走**只降不升基线**：
 * 新增一处即失败，存量随修随降；修完后把基线改小，改大就是欠新债。
 * 登记表那一段仍然留着 —— 它管的是"导航类列表必须走共享写法"这条更强的要求，
 * 与本条"任何单轴滚动容器都得交代横轴"是两件事，不要合并。
 */
const SINGLE_AXIS_BASELINE = 37;
let singleAxisCount = 0;
const singleAxisSamples = [];
for (const f of walkFiles(SRC)) {
  const r = path.relative(SRC, f).split(path.sep).join('/');
  if (r === 'utils/core/apple-tokens.ts' || r.endsWith('.test.ts') || r.endsWith('.d.ts') || r.startsWith('styles/theme/')) continue;
  const text = maskInterp(stripComments(fs.readFileSync(f, 'utf8')));
  for (const m of text.matchAll(/overflowY\s*[:=]\s*['"]?(?:auto|scroll)['"]?/g)) {
    const obj = enclosingObject(text, m.index);
    if (!obj || obj.includes('scrollSingleAxis') || /overflowX/.test(obj)) continue;
    singleAxisCount++;
    if (singleAxisSamples.length < 6) {
      singleAxisSamples.push(`${r}:${text.slice(0, m.index).split('\n').length}`);
    }
  }
}
if (singleAxisCount > SINGLE_AXIS_BASELINE) {
  problems.push({
    file: `R19b 单轴滚动容器（全 src）`,
    why: `对象级 \`overflowY: auto\` 裸写从基线 ${SINGLE_AXIS_BASELINE} 涨到 ${singleAxisCount} 处`,
    hint: '新写一个滚动容器请走 scrollSingleAxis 或显式写出 overflowX —— '
      + `只降不升。当前样例：${singleAxisSamples.join('、')}`,
  });
} else if (singleAxisCount < SINGLE_AXIS_BASELINE) {
  console.log(`ℹ R19b 单轴滚动容器裸写 ${singleAxisCount} 处 < 基线 ${SINGLE_AXIS_BASELINE} —— 把基线降到 ${singleAxisCount}，别让余量闲置`);
}

// 计数别写死：这条打印写的是"5 项"，而 R15/R16 加进来之后就一直在撒谎，
// 门禁的汇总行一旦过期，读它的人会把旧口径当现状。
const RULE_IDS = [...new Set((fs.readFileSync(new URL(import.meta.url), 'utf8').match(/^\/\/ R(\d+) /gm) || []).map(s => Number(s.slice(4))))].sort((a, b) => a - b);
console.log(`布局契约检查：${checked} 个路由页面 + ${RULE_IDS.length} 条编号约定（R${RULE_IDS[0]}–R${RULE_IDS[RULE_IDS.length - 1]}）+ 网格/壳层尺度`);if (!problems.length) {
  console.log('✓ 布局契约检查通过');
  process.exit(0);
}
for (const p of problems) console.log(`✗ ${p.file}\n    ${p.hint}\n    → ${p.why}`);
console.log(`\n✗ 布局契约未通过：${problems.length} 处`);
process.exit(1);
