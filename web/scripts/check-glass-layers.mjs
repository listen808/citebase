#!/usr/bin/env node
/**
 * 玻璃层普查 —— 用法：npm run check:glass（已串进 npm run check:style）
 *
 * 它管两件只有把 src 扫一遍才看得见的事：
 *
 * 1. **每一处 backdrop-filter 都必须走材质令牌**。写死 `blur(20px)` 的那一处不会跟随材质：
 *    切到 solid / 纸感时它照样付一次 GPU 模糊，而外观上什么都看不出来。
 *    （ApMarkdownPreview 里那条「玻璃层有预算」原先只活在注释里，就是这台门禁补上执行。
 *    预算本身是**声明站点数**，同屏层数与嵌套由 check-glass-runtime.mjs 按实测基线量。）
 *
 * 2. **每一处半透明壳面都必须交代它压在谁上面**。三种可接受的答案：
 *      a) 用浮层档（surface-elevated / overlay / surface-field）—— 这一档的可读性地板
 *         由 check-contrast 的「浮层面 × 极值外部内容」断言量过（正文 ≥7、次级/三级 ≥4.5）；
 *      b) 走 composables/ui/useScrollEdge.ts 的 shellSurfaceKey() —— 静止时通透，
 *         内容滚到底下时升到浮层档；
 *      c) 在 SHELL_EXEMPT 里带理由登记（典型是流水面：卡片、正文块，没有内容从它底下过）。
 *    为什么值得拦：浅外观下静止档的 bg-secondary 0.74 压着滚动的数据行，三级文字只剩 3.6:1，
 *    而这件事**在截图上完全看不出来**（截图里那一带通常没有内容压着）。
 *
 * 普查为 0 处一律判失败：扫不到东西不等于没有问题。
 */
import fs from 'node:fs';
import path from 'node:path';

const PRINT = process.argv.includes('--print');
const SRC = path.join(process.cwd(), 'src');

/** 每文件允许的逻辑 backdrop-filter 处数（配对写 -webkit- 只算一处；只准降不准升） */
const GLASS_SITES = {
  'components/ai/AIWriterPanel.vue': 1,
  // 2026-10-26 收敛：**只有壳层与真浮层付玻璃**，流水面一律不付。
  // ApButton / ApEntityCard / ApStatCard / ApSegmentedControl / ApMarkdownPreview 原先铺的
  // 内容档模糊已收掉 —— 它们是流水面，数量随列表放大（运行时曾实测首页同屏 24 层），
  // 且侧栏按钮压在侧栏那层玻璃上形成嵌套。材质层级改由半透明底色表达，理由写在各组件里。
  // 2026-10-26 二次调整：ApButton 的控件档模糊**恢复**（用户拍板「按钮也是液态玻璃」）——
  // 只给底本身半透明的变体付（secondary/tint 恒付，ghost/overlay 悬停浅染时付，实心不付），
  // 其余流水面（ApEntityCard / ApStatCard / ApSegmentedControl / ApMarkdownPreview）维持不付。
  // 同屏层数不再是固定红线，而是运行时实测基线 MAX_BLUR_LAYERS=16 / MAX_NESTING=1
  // （见 check-glass-runtime.mjs）；静态这边只管声明站点数，别把两者混成一个数。
  // 2026-10-26 全面普查二次修正：ghost 图标钮 idle 不再逐颗付 blur（稠密流水面——权限树全展开
  // 实测同屏 66 层），玻璃观感由 AppleGlass.dropdown 的高光/斜面/描边承担，悬停浅染时才付。
  'components/apple/ApButton.vue': 1,
  'components/apple/ApCard.vue': 1, // 只剩 loading 遮罩（AppleGlassBlur.mask，按需出现）
  // ApPopover 不再有站点：它原来的 backdropFilter 写在"实心分支"里（与半透明分支二选一），
  // 那个分支连同 glass prop 一起删了 —— 模糊现在只由 .apple-glass-layer 伪元素承载（见 MATERIALIZED）
  'components/apple/ApSidebar.vue': 1,
  'components/apple/ApTable.vue': 1,
  // 文档树侧栏 2026-11-01 并入壳层档（与应用侧栏同一档：chrome + blur-panel + 玻璃发丝边）。
  // 净账仍是降：本轮遮罩与聚光各减一处（15 → 14），这里 +1 是"两处侧栏一种材质"的代价。
  'styles/layout/doc-sidebar-styles.ts': 1,
  // 登录页/作者登录页的主题触发器（glass prop）：浮在环境渐变上的真浮层语言。
  // 顶栏那处不付（压在 .apple-glass-nav 上会玻璃压玻璃）—— 由 prop 分流，静态只此一处站点。
  'components/apple/ApThemeMenu.vue': 1,
  // AI 设置页的 sticky 导航列原先铺半透明面板却不铺模糊（全仓唯一真假玻璃）
  'views/ai/AISettingsLayout.vue': 1,
  // overlay.ts 的遮罩模糊 2026-11-01 归零：遮罩只染色，模糊交回浮层自己（见 getOverlayStyle 注释）。
  // spotlight-search-styles.ts 的 2026-11-01 归零：聚光面板的模糊从行内挪到 .apple-glass-layer
  // 伪元素（与 ApModal / 菜单同一套），顺带把那处硬编码白色高光一起收掉 —— 站点数只降不升。
  // 令牌层：.apple-glass-layer::before 1、AppleGlass.standard / heavy / nav 各 1 = 4。
  // 折射位移 url(#apple-liquid-glass) 不进 backdrop-filter —— Chromium 整条声明不渲染
  // （2026-10-27 实测），见 docs/engineering/liquid-glass.md
  'utils/core/apple-tokens.ts': 4,
  // 顶栏 blur 条件付费（2026-10-27 性能收口）：从令牌层移到组件行内，仅 scroll-edge='under'
  // 才挂 —— 静止态壳下只有静态环境层，blur 纯白付；滚动中全宽逐帧重滤波才是真实成本。
  // 与 ApTable 吸顶表头同一模式（under 才付），总站点数不变（令牌层 5→4、组件 +1）
  'components/apple/ApNavBar.vue': 1,
  // 底栏 2026-11-01 浮层化（用户判词「底栏还没穿透效果」）：负 margin 叠进滚区下沿后，
  // 内容真的从它底下滚过，按顶栏同款接 shellSurfaceKey + 两档模糊（静止 blur-panel、under 全档）。
  // 预算因此 +1（14 → 15）：这是用户点名的材质缺口，不是流水面回潮 —— 底栏是第三条壳层横边。
  'components/apple/ApFooter.vue': 1,
  'views/ai/AIKnowledgeGraphView.vue': 1,
};

/**
 * 靠 .apple-glass-layer 伪元素拿模糊的消费者（materialize 入场）。
 * 它们**没有** backdrop-filter 站点，所以必须单独钉住：谁把类名删了，
 * 那个浮层就静默变回实心板 —— 站点计数看不出这种丢失，因为模糊本来就不在它自己身上。
 */
const MATERIALIZED = new Set([
  // 2026-11-01：聚光面板此前自己写行内 blur + 硬编码高光，现并入浮层家族（与 ApModal 同源）。
  'components/apple/ApSpotlightSearch.vue',
  'components/apple/ApModal.vue',
  'components/apple/ApDrawer.vue',
  // 2026-10-15 还账：共享表面 menuSurfaceStyle 的模糊挪到 .apple-glass-layer 之后，
  // 这 10 个消费点逐个接上。漏一个 = 那个菜单静默没有模糊，而站点计数看不出来。
  'components/ai/chat/ChatInputBar.vue',
  // ApNavBar 原先靠帮助下拉用这个类；下拉从顶栏收掉后它不再有玻璃站点（2026-10-25）
  'components/apple/ApPopover.vue',
  'components/apple/ApSearch.vue',
  'components/apple/ApSelect.vue',
  'components/apple/ApSidebar.vue',
  'components/apple/ApThemeMenu.vue',
  'components/document/MonacoEditor.vue',
  'components/space/reader/DocContentPanel.vue',
  'components/space/sidebar/DocContextMenu.vue',
  // 2026-10-15 清点"哪些弹窗不是液态玻璃"时补上的四处浮层面板：
  // 它们铺浮层档却从没铺模糊 —— 浮层档按定义就是压在内容上的壳，实心等于材质在这里断了。
  'components/space/reader/FloatingToc.vue',
  'components/space/reader/MobileTocFab.vue',
  'views/workspace/SpaceView.vue',
  'views/workspace/editor/DocumentEditView.vue',
  // 引文预览原先自己写了第二张浮层表面（elevated + border-light + level3），既绕开 menuSurfaceStyle
  // 又被 FLOAT_TIER_EXEMPT 用一句错到身上的理由盖住（"内容气泡"说的是气泡，不是这块 fixed 预览）。
  // 并进唯一表面后它就是一个真浮层消费者，按规矩接 materialize。
  'components/ai/chat/ChatMessageBubble.vue',
  // 2026-10-26 全面普查补接：AI 写作面板本体原先只有遮罩模糊、自己没有玻璃语言（高光/斜面/亮边）。
  // v-if 挂载（关闭不占 DOM），打开时遮罩+面板各一层模糊 = 嵌套 1，运行时工装按基线量。
  'components/ai/AIWriterPanel.vue',
]);

/**
 * 已经量过地板的浮层档。
 * surface-float 是 2026-10-15 从 elevated 里拆出来的那一档（菜单/弹窗/抽屉）：
 * 拆档之后两档各有各的最坏背景，check-contrast 里也是两组断言，所以两档都算"量过地板"。
 */
const FLOORED = new Set(['surface-elevated', 'surface-float', 'overlay', 'surface-field']);
/** 静止档：出现在壳面上就必须走 b) 或 c) */
const REST_KEYS = new Set([
  'surface-chrome', 'surface-panel', 'surface-primary', 'surface-canvas',
  'bg-secondary', 'bg-tertiary', 'bg-hover', 'glass-bg', 'glass-bg-nav',
]);

/**
 * 交代清单。**每条理由都必须能指着一段代码讲清楚**，"它不是浮层"正是要被证明的命题。
 * 键是相对 src 的路径，值是 { surface: 该文件用的静止档, why: 为什么没有内容从它底下过 }
 */
const SHELL_EXEMPT = {
  // 顶栏 ApNavBar **不在**这张表里，但它**有**一处站点（GLASS_SITES 里那 1）：
  // 浮层壳，内容真的从它底下滚过，走 shellSurfaceKey + useShellGlassLift —— 它不需要豁免，
  // 需要的是"交代它压着谁"，那一行写在组件样式注释与 composables/ui/useScrollEdge.ts 里。
  // 没用的豁免会被下面那条检查直接判失败：空着的豁免迟早顺手盖掉下一个真站点。
  'components/apple/ApSidebar.vue': '侧栏在 bodyStyle 横排里独占一列，正文在它右侧滚动而不是它下面',
  'styles/layout/doc-sidebar-styles.ts': '文档树侧栏与应用侧栏同为"独占一列"的壳层，正文在它右侧滚动而不是它下面',
  // 控件档玻璃（2026-10-26 恢复）：按钮在页面流里随内容一起滚，没有内容从它底下滚过
  'components/apple/ApButton.vue': '按钮随页面流滚动，无内容从它底下过；壳层上的按钮嵌套由运行时工装按基线量',
  // 顶栏那颗主题触发钮：控件档（bg-secondary + AppleGlass.control），随壳层静止，
  // 没有内容从它底下滚过；它展开的下拉菜单是另一块面，走 .apple-glass-layer。
  'components/apple/ApThemeMenu.vue': '顶栏主题触发钮是控件档，随壳层静止、无内容从它底下过；下拉菜单另走浮层家族',
  // 2026-10-26：SpaceView 的 printStyle 是打印导出的 CSS 字符串（th/code 的 bg-secondary 是打印页代码块底色），
  // 不是应用壳面；全屏阅读面板 fixed 罩住整页（2026-11-01 核对：它把底色覆写成 surface-float，
  // 不是当初登记时写的 surface-panel —— 理由仍成立：整页罩死，没有内容从它底下滚过）
  'views/workspace/SpaceView.vue': 'printStyle 是打印导出 CSS 字符串；全屏面板 fixed 罩整页、无内容从底下滚过',
  // 2026-10-26：ApEntityCard / ApStatCard / ApMarkdownPreview 的模糊站点已收掉
  // （流水面不付玻璃，见 GLASS_SITES 那条注释），它们的豁免一并删除 ——
  // 没人用的豁免会一直"通过"，直到下一个真站点被它顺手盖掉。
  // ApButton 的控件档模糊 2026-10-26 用户拍板恢复（见 GLASS_SITES）：按钮随页面流滚动，
  // 没有内容从它底下滚过；侧栏/顶栏内的按钮压在壳层玻璃上是拍板接受的形态（嵌套 ≤1 由运行时工装量）。
  // 同一形状的第二处：AI 设置页把导航做成 grid 的左列并 sticky，正文在右列滚。
  // 它压的只有页面 canvas（正文与它并排），没有内容从它底下过。
  'views/ai/AISettingsLayout.vue': '设置页导航是与正文并排的 sticky 列，正文在它右侧滚动而不是它下面',
  // 卡片本体不再付模糊，只剩 loading 遮罩这一处站点（AppleGlassBlur.mask，按需出现）。
  'components/apple/ApCard.vue': 'loading 遮罩压的是卡片自己的内容（局部遮罩），不是从它底下滚过的页面内容',
  // 2026-11-01：图谱 tooltip 改走浮层档（surface-float + glass-rim + 浮层圆角）之后，
  // 它已经是"铺了浮层档且带玻璃"的真表面，不再需要豁免 —— 留着这条只会顺手盖掉下一个真站点。
  // 这三张卡铺的是材质常量 AppleGlass.standard（静止档 surface-panel）。
  // 判据核过：三个文件里既没有 overflow 容器也没有 sticky 元素，卡片就在页面流里，
  // 背后是页面自己的环境层 —— 没有内容从它底下滚过，所以不需要升档。
  'views/auth/LoginView.vue': '整屏一张登录卡，页内无滚动容器，内容不从卡片底下过',
  'views/auth/AuthorLoginView.vue': '整屏一张授权登录卡，同上',
  'views/auth/InstallView.vue': '安装向导整屏一张卡，同上',
};

function walk(dir, out = []) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) walk(p, out);
    else if (/\.(vue|ts)$/.test(e.name) && !e.name.endsWith('.d.ts') && !e.name.endsWith('.test.ts')) out.push(p);
  }
  return out;
}

/**
 * 令牌层特殊处理：apple-tokens.ts 里的 AppleGlass.* 是**材质常量的定义处**，
 * 它们本身不定位、不压内容 —— 交代"压在谁上面"是消费方的事。
 * 所以这里不判它，但下面会单独普查每个 `...AppleGlass.` 的消费点并要求登记。
 */
const TOKEN_LAYER = 'utils/core/apple-tokens.ts';

/**
 * 只数逻辑站点。`(?<![-\w])` 一站就把两种配对写法排除掉：
 * CSS 的 `-webkit-backdrop-filter` 前面是 `-`，JS 的 `WebkitBackdropFilter` 前面是字母 t，
 * 都不允许出现在 "backdrop-filter" 之前。
 * （正则对象带 /g 时 .test() 会推进 lastIndex，跨行复用会漏判，所以每次新建。）
 */
const isBlurLine = (line) => {
  const t = line.trim();
  // 注释里写着"这里原先挂着 backdropFilter: cv('glass-blur')"是**历史说明**，不是站点。
  // 不排掉它就会把一段解释当成一处玻璃，然后逼着作者去删注释 —— 判据不该有这种副作用。
  if (t.startsWith('//') || t.startsWith('*') || t.startsWith('/*')) return false;
  return /(?<![-\w])backdrop-?Filter\s*[:=]/i.test(line);
};

/**
 * 抹掉注释但保住行号。下面"浮层档必须铺玻璃"那条判据也靠它：
 * 组件里"（surface-elevated）"这类解释性注释到处都是，不抹就会把注释当表面。
 */
const stripComments = (t) => t
  .replace(/\/\*[\s\S]*?\*\//g, (m) => m.replace(/[^\n]/g, ' '))
  .replace(/(^|[^:])\/\/[^\n]*/g, (_m, p) => p + ' '.repeat(_m.length - p.length));

/** 取这处壳面真正铺的底色键：只看设置 background 的那些行，别把注释里的旧名字算进来 */
function surfaceKeysNear(lines, i) {
  const window_ = lines.slice(Math.max(0, i - 20), i + 20);
  const found = new Set();
  window_.forEach((l) => {
    // 注释行（REM /// // * 以及 CSS 注释里的历史说明）不算：
    // 那段"原先玻璃分支写 cv('glass-bg')"就在 ApModal 里，按行取会把它当成当前表面档
    const t = l.trim();
    if (t.startsWith('//') || t.startsWith('*') || t.startsWith('/*') || t.includes('原先')) return;
    if (!/background/.test(l)) return;
    for (const m of l.matchAll(/cv\(\s*'([a-z0-9-]+)'\s*\)/g)) found.add(m[1]);
    for (const m of l.matchAll(/var\(--apple-([a-z0-9-]+)\)/g)) found.add(m[1]);
  });
  return [...found];
}

const files = walk(SRC);
const errors = [];
const rows = [];
let blurTotal = 0;
const glassConsumers = [];
/** 豁免必须被真的用上：没人用的豁免会烂在那儿，下一任读不出它防的是哪块代码 */
const usedExemptions = new Set();

for (const abs of files) {
  const rel = path.relative(SRC, abs).split(path.sep).join('/');
  const text = fs.readFileSync(abs, 'utf8');
  const lines = text.split('\n');

  // ---- 0. AppleGlass.* 的消费点也要登记 ----
  if (rel !== TOKEN_LAYER) {
    lines.forEach((line, i) => {
      // 注释里提到 AppleGlass.* 是**说明**（例如"与曾经的 AppleGlass.dropdown 合并"），
      // 不是消费点；不排掉就会把一段解释当成一处表面，逼作者删注释。
      const t = line.trim();
      if (t.startsWith('//') || t.startsWith('*') || t.startsWith('/*')) return;
      if (!/\.\.\.AppleGlass\.|AppleGlass\.[a-z]/.test(line)) return;
      const m = line.match(/AppleGlass\.([a-z]+)/);
      glassConsumers.push(`${rel}:${i + 1} → ${m ? m[1] : '?'}`);
      if (!SHELL_EXEMPT[rel]) {
        errors.push(`${rel}:${i + 1} 铺了材质常量 AppleGlass.${m ? m[1] : '?'}（静止档表面），` +
          `但没在 SHELL_EXEMPT 交代这块表面有没有内容从它底下滚过`);
      } else {
        usedExemptions.add(rel);
      }
    });
  }

  let blurCount = 0;
  lines.forEach((line, i) => {
    if (!isBlurLine(line)) return;
    blurCount++;
    blurTotal++;
    const value = (line.split(/backdrop-?Filter\s*[:=]/i)[1] || '').trim();
    /**
     * 可接受的写法：材质令牌（cv('glass-blur') / var(--apple-glass-blur) / 令牌层里的 GLASS_FILTER）、
     * 令牌层的命名档（AppleGlassBlur.*，遮罩那类固定用途走它，T8 已另行管着组件不许自己抄），
     * 以及显式 'none'。
     * 判失败的是**写死数值**：blur(20px) 不会跟随材质，切到 solid/纸感时照样付一次 GPU 模糊。
     */
    const hardcoded = /blur\(\s*\d/.test(value);
    const knownForm = /glass-blur|blur-panel|GlassBlur|AppleGlass\.|GLASS_FILTER|liquid-glass|['"]none['"]/.test(line) || value === '';
    if (hardcoded || !knownForm) {
      errors.push(`${rel}:${i + 1} backdrop-filter 不是材质令牌（写的是 ${value.slice(0, 60) || '(空)'}）` +
        (hardcoded ? ' —— 写死数值不会跟随材质切换，切到 solid/纸感时它照样付一次 GPU 模糊' : ''));
    }

    if (rel === TOKEN_LAYER) { rows.push(`${rel}:${i + 1}  令牌层定义处`); return; }

    /**
     * 这一支把玻璃**关掉**（solid 分支回到实心面板）：它不铺半透明表面，
     * 所以不需要交代压在谁上面。数站点仍然数它 —— 它是材质分支的证据，不是漏网。
     */
    if (/['"]none['"]/.test(value)) { rows.push(`${rel}:${i + 1}  显式不铺玻璃`); return; }

    // ---- 这处壳面交代了吗 ----
    const near = lines.slice(Math.max(0, i - 20), i + 20).join('\n');
    const surf = surfaceKeysNear(lines, i);
    const rest = surf.filter((k) => REST_KEYS.has(k));
    const floored = surf.some((k) => FLOORED.has(k));
    const stateful = /shellSurfaceKey/.test(near);
    let verdict;
    if (stateful) verdict = 'shellSurfaceKey';
    else if (floored && rest.length === 0) verdict = '浮层档';
    else if (SHELL_EXEMPT[rel]) { usedExemptions.add(rel); verdict = 'exempt: ' + SHELL_EXEMPT[rel]; }
    else if (rest.length === 0 && surf.length === 0) {
      verdict = '未交代（取不到表面键）';
      errors.push(`${rel}:${i + 1} 这处半透明壳面取不到它铺的表面档，也无法证明它不压内容 —— ` +
        `要么让它显式用浮层档，要么在 SHELL_EXEMPT 带理由登记`);
    } else {
      verdict = '未交代';
      errors.push(`${rel}:${i + 1} 这处半透明壳面用了静止档 ${rest.join('/')}，` +
        `既不是浮层档、也没走 shellSurfaceKey()、也没在 SHELL_EXEMPT 带理由登记`);
    }
    rows.push(`${rel}:${i + 1}  ${verdict}`);
  });

  const expected = GLASS_SITES[rel] ?? 0;
  if (blurCount !== expected) {
    errors.push(`${rel} 的逻辑 backdrop-filter 站点数 = ${blurCount}，登记表里是 ${expected}。` +
      `新增一处要在 GLASS_SITES 登记并写理由（玻璃层有预算：声明站点数只准降不准升；` +
      `同屏层数基线见 check-glass-runtime.mjs）`);
  }

  // ---- 3. materialize 消费者：模糊不在自己身上，站点计数看不见它丢没丢 ----
  // 只看类绑定，不看正文提及：这些文件里都有解释 .apple-glass-layer 的注释，
  // 拿 /apple-glass-layer/ 去判"接没接"，注释自己就会把类名被删掉这件事盖住（实测盖住过一次）。
  const usesMaterialize = rel !== TOKEN_LAYER &&
    /class="[^"]*apple-glass-layer/.test(text);
  if (usesMaterialize && !MATERIALIZED.has(rel)) {
    errors.push(`${rel} 用了 .apple-glass-layer 却没在 MATERIALIZED 登记`);
  }
  if (MATERIALIZED.has(rel) && !usesMaterialize) {
    errors.push(`${rel} 登记在 MATERIALIZED 里，但类名已经不在这份文件里了 —— ` +
      `这个浮层现在拿不到模糊，而 backdrop-filter 站点计数看不出这种丢失`);
  }
}

// 登记表里不许留已经不存在的文件：那会让下一任误以为那里还有玻璃
for (const rel of Object.keys(GLASS_SITES)) {
  if (!files.includes(path.join(SRC, rel))) errors.push(`登记表里的 ${rel} 已经不存在了，从 GLASS_SITES 删掉`);
}
for (const rel of Object.keys(SHELL_EXEMPT)) {
  if (!fs.existsSync(path.join(SRC, rel))) errors.push(`SHELL_EXEMPT 里的 ${rel} 已经不存在了，删掉这条豁免`);
  else if (!usedExemptions.has(rel)) {
    errors.push(`SHELL_EXEMPT 里的 ${rel} 这一轮没被任何站点用上 —— 删掉它：` +
      `没人用的豁免会一直"通过"，直到下一个真站点被它顺手盖掉`);
  }
}

if (blurTotal === 0) errors.push('一处 backdrop-filter 都没扫到 —— 判据在空转');

/**
 * 材质层的无障碍三信号。**prefers-contrast 早先整条是缺的**：
 * 只实现 reduced-transparency 看着像"无障碍做过了"，但系统里这两条是可以各开各的，
 * 提高对比度的用户拿到的仍然是半透明面。缺哪条都会在报表里显示成"这块做完了"。
 * 顺带核三张数据表都真的给了两套外观 —— 空表会让 @media 分支变成一条什么也不做的规则。
 */
const themesText = fs.readFileSync(path.join(SRC, 'styles/theme/themes.ts'), 'utf8');
const A11Y_SIGNALS = [
  ['@media (prefers-reduced-transparency: reduce)', '减弱透明度：玻璃退回不透明表面'],
  ['@media (prefers-contrast: more)', '提高对比度：近实心 + 把描边一族画实'],
  ['@media (prefers-reduced-motion: reduce)', '减弱动效：环境层不再跟随视口固定'],
];
for (const [needle, what] of A11Y_SIGNALS) {
  if (!themesText.includes(needle)) {
    errors.push(`材质层缺无障碍信号 ${needle}（${what}） —— 玻璃是默认材质，缺这一条等于默认路径少一档退化`);
  }
}

/**
 * 同一批信号的**应用内入口**（2026-10-16 个性化批1/批2）。
 * 只核 @media 不够：加了偏好页的三档开关、而 CSS 里只有媒体查询那条路径时，
 * 从偏好页进来的用户拿到的是默认材质，而静态门禁与单测都看不见（它们不跑浏览器）。
 *
 * **判据按出现次数判，不按"有没有这个串"判**。第一版就是后者的写法，于是探针把
 * `:root[data-motion='reduced'] *` 三选择器里的两个改名，门禁照样绿 ——
 * 因为剩下那个 `*::after` 仍然含同一个子串。数一下就抓得住"删掉一整块"和"只删一半"这两种，
 * 而后者正是这类入口最容易犯的错（一处漏接 = 那个入口的动效还在跑）。
 */
const count = (text, needle) => text.split(needle).length - 1;
const tokensText = fs.readFileSync(path.join(SRC, 'utils/core/apple-tokens.ts'), 'utf8');
// 护栏常量住 personalization.ts（材质层与注入层都要用它），所以判"护栏在不在"要两个文件一起看
const persText = fs.readFileSync(path.join(SRC, 'styles/theme/personalization.ts'), 'utf8');
const APP_ENTRIES = [
  [themesText, "[data-a11y-transparency='reduce']", 2, '应用内「减弱透明度」的玻璃退化块（表面 + 撤掉下沿渐隐）'],
  [themesText, ":not([data-a11y-transparency='full'])", 2, '系统减弱透明度信号的应用内护栏（两处：表面块与 nav-bar 渐隐）'],
  [themesText, "[data-a11y-contrast='more']", 1, '应用内「提高对比度」的描边加强块'],
  [themesText, ":not([data-a11y-contrast='standard'])", 1, '系统提高对比度信号的应用内护栏：选了 standard 也会被 @media 压住'],
  [themesText, "[data-motion='reduced']", 1, '应用内「减弱动效」档的环境层退路（background-attachment）'],
  [persText, ":not([data-motion='full'])", 1, 'MOTION_GUARD 护栏本体：没有它，应用内选了 full 也会被系统通配压成 0.01ms'],
  [tokensText, ":root[data-motion='reduced']", 6, '注入层减弱动效的应用内入口（通配三选择器 + scroll-behavior + 页过渡两行 + 玻璃入场）'],
  [tokensText, '${MOTION_GUARD}', 3, '注入里按护栏判断的三处（通配两行 + 页过渡一行）'],
];
for (const [text, needle, min, what] of APP_ENTRIES) {
  const got = count(text, needle);
  if (got < min) {
    errors.push(`缺应用内/护栏入口 —— ${what}：期望至少 ${min} 处，实得 ${got} 处（"${needle}"）`);
  }
}
for (const table of ['GLASS_OVERLAY', 'GLASS_CLEAR_OVERLAY', 'GLASS_OPAQUE_OVERLAY', 'GLASS_CONTRAST_OVERLAY']) {
  const at = themesText.indexOf(`export const ${table}`);
  if (at < 0) { errors.push(`${table} 找不到了 —— 判据引用的结构已变了，先修判据再说代码`); continue; }
  const body = themesText.slice(at, themesText.indexOf('};', at));
  for (const appearance of ['light', 'dark']) {
    const block = body.split(`${appearance}:`)[1] ?? '';
    const keys = (block.slice(0, block.indexOf('},') + 2).match(/'--apple-/g) || []).length;
    if (keys < 3) errors.push(`${table}.${appearance} 只有 ${keys} 个键 —— 空表会让对应的 @media 分支变成一条什么也不做的规则`);
  }
}

/**
 * 材质门（2026-10-27）：玻璃的**视觉语言**（模糊、镜面高光、亮边、方向性棱线、浮起投影）
 * 必须挂在 `:root[data-material='glass']` 上，基础规则只留定位与层序。
 *
 * 为什么令牌判据挡不住这件事：纸感/杂志把 `--apple-glass-blur` 解析成 none、
 * 高光解析成 none、棱线解析成透明 —— 令牌全对；但只要装饰写在基类 `.apple-glass-layer::before`
 * 里，那条 `0 10px 30px` 的黑投影和白色斜面仍会被绘制。令牌层「对」而观感仍残留玻璃，
 * 正是用户报的"纸感里还有液态玻璃"。themes.test.ts 只看令牌值，看不见 CSS 规则体，所以在这里判。
 */
const cssRuleBody = (selector) => {
  const at = tokensText.indexOf(selector + ' {');
  if (at < 0) return null;
  const end = tokensText.indexOf('}', at);
  return end < 0 ? null : tokensText.slice(at + selector.length + 2, end);
};
const baseLayer = cssRuleBody('.apple-glass-layer::before');
if (baseLayer === null) {
  errors.push('找不到 `.apple-glass-layer::before` 基础规则 —— 判据引用的结构已变了');
} else {
  for (const [what, needle] of [
    ['模糊', 'backdrop-filter'],
    ['浮起投影', 'box-shadow'],
    ['镜面高光', '--apple-glass-specular'],
    ['方向性棱线', '--apple-glass-edge'],
    ['亮边', '--apple-glass-rim'],
  ]) {
    if (baseLayer.includes(needle)) {
      errors.push(`.apple-glass-layer::before 基础规则里还有${what}（${needle}）—— ` +
        `它不在 data-material='glass' 门上，solid / 纸感 / 杂志也会画出来。装饰应挪进 ` +
        `:root[data-material='glass'] .apple-glass-layer::before`);
    }
  }
}
const glassLayer = cssRuleBody(":root[data-material='glass'] .apple-glass-layer::before");
if (glassLayer === null) {
  errors.push("找不到 `:root[data-material='glass'] .apple-glass-layer::before` 规则 —— " +
    `玻璃材质现在没有视觉语言（只剩半透底色），或者材质门被拆掉了`);
} else {
  for (const [what, needle] of [
    ['模糊', 'backdrop-filter: var(--apple-glass-blur)'],
    ['镜面高光', 'background: var(--apple-glass-specular)'],
    ['方向性棱线', 'var(--apple-glass-edge)'],
    ['亮边', 'var(--apple-glass-rim)'],
  ]) {
    if (!glassLayer.includes(needle)) {
      errors.push(`:root[data-material='glass'] .apple-glass-layer::before 缺${what}（${needle}）—— ` +
        `玻璃材质下浮层会退化成一块半透实板`);
    }
  }
}
/** 顶栏同理：玻璃投影/亮边写在基类可以，但必须有一条非玻璃材质退回主题自己导航面的规则 */
const baseNav = cssRuleBody('.apple-glass-nav');
const solidNav = cssRuleBody(":root:not([data-material='glass']) .apple-glass-nav");
if (baseNav === null) errors.push('找不到 `.apple-glass-nav` 基础规则 —— 判据引用的结构已变了');
else if (/backdrop-filter/.test(baseNav)) {
  errors.push('.apple-glass-nav 基础规则里又挂上了 backdrop-filter —— ' +
    '模糊只该在 ApNavBar 行内、scroll-edge=under 时付（2026-10-27 性能收口）');
} else if (!baseNav.includes('var(--apple-glass-rim)') || !baseNav.includes('var(--apple-glass-edge)')) {
  errors.push('.apple-glass-nav 缺玻璃亮边/棱线 —— 玻璃材质下顶栏没有材质语言');
}
if (solidNav === null) {
  errors.push("找不到 `:root:not([data-material='glass']) .apple-glass-nav` —— " +
    `非玻璃材质下顶栏仍会画玻璃投影与白色斜面（纸感/杂志残留液态玻璃）`);
} else if (!solidNav.includes('--apple-border') || !solidNav.includes('--apple-shadow-nav')) {
  errors.push(`:root:not([data-material='glass']) .apple-glass-nav 没有退回主题自己的导航边与阴影`);
}

/**
 * 行内材质门（2026-11-01）：上面那条材质门只看**令牌层的规则体**，组件行内样式不在射程内。
 * 实测漏网：`dark + solid`（这台根本没有玻璃）下，聚光面板仍画一道 135° 白色斜光 ——
 * 那串 `rgba(255,255,255,.30)` 是写在 containerStyle 里的字面值，切材质时没人管。
 * 判据收窄到"只会是玻璃装饰"的写法：值里出现字面白/黑，且带 linear-gradient 或 inset。
 * （组件阴影里的 rgba(0,0,0,.x) 很常见，但那不带 inset/gradient 就不是材质高光，不算。）
 */
for (const abs of files) {
  const rel = path.relative(SRC, abs).split(path.sep).join('/');
  if (rel === TOKEN_LAYER || rel === 'styles/theme/themes.ts' || rel.endsWith('.test.ts')) continue;
  const lines = stripComments(fs.readFileSync(abs, 'utf8')).split('\n');
  lines.forEach((line, i) => {
    if (!/(backgroundImage|boxShadow|extraCssText)\s*[:=]/.test(line)) return;
    const literal = /rgba\(\s*255\s*,\s*255\s*,\s*255/.test(line) || /rgba\(\s*0\s*,\s*0\s*,\s*0/.test(line);
    const decor = /linear-gradient\(/.test(line) || /\binset\b/.test(line);
    if (literal && decor) {
      errors.push(`${rel}:${i + 1} 把玻璃装饰（镜面高光 / 内棱线）写成字面颜色 —— 行内样式不受材质门管辖，` +
        `非玻璃材质照样会画出来。改用 cv('glass-specular') / var(--apple-glass-edge) 这一族`);
    }
  });
}

if (PRINT) {
  console.log(`逻辑 backdrop-filter 站点 ${blurTotal} 处：\n  ` + rows.join('\n  '));
}

/**
 * materialize 的**债账**（只准缩，不准长）。
 * 已接线：见上面 MATERIALIZED（ApModal / ApDrawer）。
 * 下面这些是共用 `menuSurfaceStyle()` 那张表面的浮层，还没接 —— 它们现在**有**模糊（模糊在共享样式里），
 * 所以漏接不会立刻显形；要挪走那份共享模糊，必须先把这里的每一处都接上类名，
 * 这也是本轮没有顺手挪它的原因：接错一个元素就是静默丢模糊，而那次改动没有任何肉眼证据可核。
 * （2026-10-15 债已还清：共享表面 menuSurfaceStyle 的模糊挪到 .apple-glass-layer，
 * 10 个消费点全部接上并进了 MATERIALIZED。下面这条规则留着，是为了让下一个新消费者
 * 必须显式登记 —— 漏接的菜单会静默没有模糊，而站点计数看不出来。）
 *
 * **粒度要说清楚，别把它读成更大的东西**：这张表记的是**文件**，不是浮层个数。
 * 同一文件里再加第二个用 menuSurfaceStyle 的面板，这条判据看不出债又厚了一层 ——
 * 静态判据分不清哪个 style 对象最终落在哪个元素上，那需要真的把页面渲染出来看。
 */
// 遍历**所有**文件而不是登记表：新加一个用 menuSurfaceStyle 的浮层，它恰恰不在任何表里，
// 按表遍历就等于这条规则永远不会命中（第一版就是这么写的，实测注入一个新消费者毫无反应）。
for (const abs of files) {
  const rel = path.relative(SRC, abs).split(path.sep).join('/');
  if (rel === TOKEN_LAYER) continue;
  const t = fs.readFileSync(abs, 'utf8');
  if (/menuSurfaceStyle\(/.test(t) && !MATERIALIZED.has(rel)) {
    errors.push(`${rel} 用 menuSurfaceStyle 铺浮层表面，却没登记进 MATERIALIZED —— ` +
      `共享表面的模糊已挪到 .apple-glass-layer，漏接的菜单会静默没有模糊`);
  }
}

/**
 * 反向判据：**铺了浮层档表面就必须真的铺玻璃**（或带理由豁免）。
 * 上面三条判据都在数"已有的 backdrop-filter 站点"，所以"该有玻璃却没有"的浮层
 * 从它们眼下走过 —— 用户报的"很多弹窗不是液态玻璃"正是这一类：
 * 表面档走的是浮层那一档，却没有任何模糊，读起来就是一块实心板。
 * 两档都要查：2026-10-15 把菜单/弹窗从 elevated 拆到 float 之后，只认 elevated 的话
 * 这条判据会跟着表面档搬家一起瞎掉（拆档当天就瞎了一次，靠 rv 抓回来）。
 */
const FLOAT_TIER_EXEMPT = {
  // 那处 surface-elevated 是 <img> 本体的衬底色，不是浮层面板：给图片铺模糊只会糊掉用户要看的东西
  'components/apple/ApImageLightbox.vue': 'img 本体衬底，非浮层面板',
  // 2026-10-26：侧栏收起/展开的骑缝把手（20×44 小胶囊，贴着侧栏自身那层玻璃的边缘）——
  // 再付一层模糊就是"玻璃压玻璃"的微缩版（嵌套基线 1、同屏基线 16，见 check-glass-runtime.mjs），
  // 实心浮层底 + 描边足够表达材质层级
  'components/layout/MainLayout.vue': '骑缝把手是贴着侧栏玻璃边缘的小浮标，实心即可，不再付模糊',
};
const FLOAT_TIER = /background(-color)?\s*[:=][^\n]*surface-(elevated|float)/;
for (const abs of files) {
  const rel = path.relative(SRC, abs).split(path.sep).join('/');
  if (rel === TOKEN_LAYER || rel === 'styles/theme/themes.ts' || rel.endsWith('.test.ts')) continue;
  const t = stripComments(fs.readFileSync(abs, 'utf8'));
  const hit = FLOAT_TIER.exec(t);
  if (!hit) continue;
  const hasGlass = /backdrop-?Filter|backdrop-filter|apple-glass-layer/.test(t);
  if (hasGlass || FLOAT_TIER_EXEMPT[rel]) continue;
  errors.push(`${rel} 用 surface-${hit[1]} 铺浮层表面却没有玻璃 —— 要么接 .apple-glass-layer，` +
    `要么在 FLOAT_TIER_EXEMPT 写清为什么这块浮层该是实心`);
}
for (const rel of Object.keys(FLOAT_TIER_EXEMPT)) {
  const abs = path.join(SRC, rel);
  if (!fs.existsSync(abs)) { errors.push(`FLOAT_TIER_EXEMPT 里的 ${rel} 已不存在，删掉`); continue; }
  const t = stripComments(fs.readFileSync(abs, 'utf8'));
  if (!FLOAT_TIER.test(t)) {
    errors.push(`FLOAT_TIER_EXEMPT 里的 ${rel} 这一轮没用到浮层档表面 —— 删掉这条豁免（空豁免会顺手盖掉下一个真违例）`);
  }
}

if (errors.length) {
  console.error(`\n✗ 玻璃层普查未通过（${errors.length} 项）：`);
  for (const e of errors) console.error('  ' + e);
  process.exit(1);
}
console.log(`ℹ 玻璃层普查：${blurTotal} 处 backdrop-filter 全部走材质令牌并登记在册，` +
  `逐条交代了压在谁上面（浮层档 / shellSurfaceKey / 带理由的豁免）`);
// 这台门禁数的是**声明站点**，回答不了"一屏同时叠了几层"（v-for 会把一处声明放大成十几层，
// 2026-10-26 实测首页 24 层，当时静态那边全绿）。改完玻璃材质要再跑一次运行时工装。
console.log(`ℹ 提醒：同屏层数与玻璃嵌套只有运行时量得到 —— ` +
  `改玻璃后请跑 SNAP_USER=… SNAP_PASS=… npm run check:glass:runtime（不在 CI，需本机登录态与服务）`);
