// 反向验证工装：给"壳层标尺 + 令牌别名"这两组门禁断言逐条注入违例，确认每条都抓得到。
// 为什么要有这个文件：门禁只要有一条正则写错，就会永远绿——上一轮 check-layout-contract
// 的 minmax 正则就是这类"假绿"（写成 min?\(?\)? 只认 min(max()) 一种形态，纯数字地板全放行）。
// 用法：npm run check:gates:rv（路径按脚本位置算，从仓库根或 web/ 跑都一样）
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';
import { execFileSync } from 'child_process';

const WEB = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
// 临时副本放在 web/ 下而不是 scripts/ 里：工装要把 scripts/ 也复制一份（R14 有跨文件对齐检查），
// 副本嵌在源目录里 cpSync 会报"copy into self"。
const TMP = path.join(WEB, '.rv-tmp');
const SRC = path.join(TMP, 'src');
const GATES = {
  layout: path.join(WEB, 'scripts', 'check-layout-contract.mjs'),
  theme: path.join(WEB, 'scripts', 'check-theme-contract.mjs'),
  doctrine: path.join(WEB, 'scripts', 'check-design-doctrine.mjs'),
  glass: path.join(WEB, 'scripts', 'check-glass-layers.mjs'),
  contrast: path.join(WEB, 'scripts', 'check-contrast.mjs'),
  style: path.join(WEB, 'scripts', 'check-style-tokens.mjs'),
};

const TOKENS = path.join(SRC, 'utils/core/apple-tokens.ts');
const FOOTER = path.join(SRC, 'components/apple/ApFooter.vue');
const SIDEBAR = path.join(SRC, 'components/apple/ApSidebar.vue');
const NAVBAR = path.join(SRC, 'components/apple/ApNavBar.vue');
const CARD = path.join(SRC, 'components/apple/ApCard.vue');
const HOME = path.join(SRC, 'views/workspace/home/HomeView.vue');
const CRUMB = path.join(SRC, 'components/apple/ApBreadcrumb.vue');
const CONTAINER = path.join(SRC, 'components/apple/ApPageContainer.vue');
const SNAP = path.join(TMP, 'scripts', 'visual-snapshot.mjs');
const THEMES = path.join(SRC, 'styles/theme/themes.ts');
const PERS = path.join(SRC, 'styles/theme/personalization.ts');
const ACCENTS = path.join(SRC, 'styles/theme/accents.ts');
const SPOTLIGHT = path.join(SRC, 'styles/component/spotlight-search-styles.ts');
const MODAL = path.join(SRC, 'components/apple/ApModal.vue');
const MOCTOC = path.join(SRC, 'components/space/reader/MobileTocFab.vue');
const GLASSMATH = path.join(SRC, 'utils/core/glass-contrast.ts');

const cases = [
  // ── R14：壳层一把尺 ──
  {
    gate: 'layout',
    name: '标尺被写了第二个数（shellBarHeight 直接写死）',
    file: TOKENS, from: 'shellBarHeight: SHELL_BAR_HEIGHT,', to: "shellBarHeight: '52px',",
    expect: 'AppleLayout.shellBarHeight 写了死数',
  },
  {
    gate: 'layout',
    name: '历史别名 navHeight 复活',
    file: TOKENS, from: 'shellBarHeight: SHELL_BAR_HEIGHT,', to: 'shellBarHeight: SHELL_BAR_HEIGHT,\n  navHeight: SHELL_BAR_HEIGHT,',
    expect: 'navHeight 这个历史别名',
  },
  {
    gate: 'layout',
    name: '壳层 CSS 变量与标尺分叉',
    file: TOKENS, from: "'--apple-shell-bar-height': AppleLayout.shellBarHeight,", to: "'--apple-shell-bar-height': '60px',",
    expect: '--apple-shell-bar-height 不再取自壳层标尺',
  },
  {
    gate: 'layout',
    name: '吸顶样式的变量兜底写死（藏第二把尺）',
    // 原先这条注入改的是 .apple-sticky 的兜底值，而那个工具类已作为"零消费者死码"删除
    // （G19 抓的）。判据本身仍然要防住，所以改成**注入一条新的**带死兜底的吸顶规则 ——
    // 目标从"改旧代码"变成"造新违例"，与这条规则的适用面一致。
    file: TOKENS, from: '.apple-glass-nav {',
    to: '.apple-sticky-probe { position: sticky; top: var(--apple-shell-bar-height, 60px); }\n.apple-glass-nav {',
    expect: '--apple-shell-bar-height 的兜底值写了',
  },
  {
    gate: 'layout',
    name: '页脚条退回裸 px 高度',
    file: FOOTER, from: 'height: AppleLayout.shellBarHeight,', to: "height: '60px',",
    expect: 'footerStyle 里写了裸 px 高度',
  },
  {
    gate: 'layout',
    name: '顶栏改用别的名字引用同一把尺',
    file: NAVBAR, from: 'height: AppleLayout.shellBarHeight,', to: 'height: AppleLayout.sidebarWidth,',
    expect: 'navStyle 的高度没走 AppleLayout.shellBarHeight',
  },
  {
    gate: 'layout',
    name: '侧栏用户行的运行时锚点被改名',
    file: SIDEBAR, from: 'data-ap-shell="user-row"', to: 'data-ap-shell="usr"',
    expect: '缺 data-ap-shell="user-row"',
  },
  {
    gate: 'layout',
    name: '变量只在一头改名（运行时期望值会读成 NaN 而静默全绿）',
    file: SNAP, from: "getPropertyValue('--apple-shell-bar-height')", to: "getPropertyValue('--apple-nav-height')",
    expect: '壳层期望值仍在读别的变量',
  },
  {
    // 合成场景（重复声明本身编译不过），只为证明"标尺被定义多次"这一支不是死代码
    gate: 'layout',
    name: '标尺被重复定义',
    file: TOKENS, from: "const SHELL_BAR_HEIGHT = '48px';", to: "const SHELL_BAR_HEIGHT = '48px';\nconst SHELL_BAR_HEIGHT = '44px';",
    expect: 'SHELL_BAR_HEIGHT 定义了 2 次',
  },

  // ── T12：一个概念一个名字 ──
  {
    gate: 'theme',
    name: '已删除的表面别名被写回 :root',
    file: TOKENS, from: "  '--apple-surface-knob': AppleColor.pureWhiteCanvas,",
    to: "  '--apple-bg-card': 'var(--apple-surface-panel)',\n  '--apple-surface-knob': AppleColor.pureWhiteCanvas,",
    expect: '是已删除的表面别名',
  },
  {
    gate: 'theme',
    name: '新造一条同层转发别名（泛化那一支）',
    file: TOKENS, from: "  '--apple-surface-knob': AppleColor.pureWhiteCanvas,",
    to: "  '--apple-panel-alias': 'var(--apple-surface-panel)',\n  '--apple-surface-knob': AppleColor.pureWhiteCanvas,",
    expect: '的转发别名',
  },

  // ── G10：组件不许再踩别名（<style> 块里 TS 类型看不见）──
  {
    gate: 'doctrine',
    name: '组件样式块里写已删除的别名变量',
    file: CARD, from: '  background: cv(\'surface-panel\'),',
    to: '  background: cv(\'surface-panel\'),\n  /* eslint-disable */\n  border: 1px solid var(--apple-glass-bg);',
    expect: '用了已删除的表面别名',
  },

  // ── G14：收藏开关只有一处实现 ──
  {
    gate: 'doctrine',
    name: '页面又手搓收藏星标（回归成首页那版写法）',
    file: HOME, from: '<ApFavoriteButton',
    to: '<ApButton aria-label="收藏空间"><Star :size="14" /></ApButton><ApFavoriteButton',
    expect: '自己画星标又自己写"收藏"文案',
  },
  {
    gate: 'doctrine',
    name: '斜线星 StarOff 复活（回归成收藏页那版写法）',
    file: HOME, from: '<ApFavoriteButton',
    to: '<ApButton aria-label="取消收藏"><StarOff :size="14" /></ApButton><ApFavoriteButton',
    expect: '渲染了 StarOff',
  },

  // ── G17–G21：把"运行时量不到"的四种源码级漂移钉住 ──
  {
    gate: 'doctrine',
    name: 'G17 页面样式里写 transition: all（运行时那台看不见未渲染的元素）',
    file: HOME, from: '<ApFavoriteButton',
    to: "<div :style=\"{ transition: 'all 0.2s ease' }\">z</div><ApFavoriteButton",
    expect: 'G17',
  },
  {
    gate: 'doctrine',
    name: 'G18 裸文件上传控件没有可访问名字',
    file: HOME, from: '<ApFavoriteButton',
    to: '<input type="file" accept=".png" /><ApFavoriteButton',
    expect: 'G18',
  },
  {
    gate: 'doctrine',
    name: 'G19 注入一个没人用的 .apple-* 工具类（第二套并行样式体系）',
    file: TOKENS, from: '.apple-glass-nav {',
    to: '.apple-dead-probe { color: red; }\n.apple-glass-nav {',
    expect: '工具类 .apple-dead-probe 零消费者',
  },
  {
    gate: 'doctrine',
    name: 'G19 注入一个没人用的关键帧',
    file: TOKENS, from: '@keyframes apple-pulse-soft {',
    to: '@keyframes zz-dead-kf { 0% { opacity: 1; } }\n@keyframes apple-pulse-soft {',
    expect: '关键帧 zz-dead-kf 零消费者',
  },
  {
    gate: 'doctrine',
    name: 'G20 图标用 13px 这种介于两档之间的奇数值',
    file: HOME, from: '<ApFavoriteButton',
    to: '<Star :size="13" /><ApFavoriteButton',
    expect: 'G20',
  },
  {
    gate: 'doctrine',
    name: 'G21 阴影注册表里塞一个与已有键逐字节同值的第二个名字',
    file: TOKENS, from: "  subtle: '0 1px 4px rgba(0,0,0,0.06)',",
    to: "  subtle: '0 1px 4px rgba(0,0,0,0.06)',\n  subtleDup: '0 1px 4px rgba(0, 0, 0, 0.06)',",
    expect: 'subtleDup 与 subtle 值逐字节相同',
  },
  {
    gate: 'doctrine',
    name: 'G22 页面自己声明一个 progressbar 角色（第二套进度条的身份）',
    file: HOME, from: '<ApFavoriteButton',
    to: '<div role="progressbar" aria-valuenow="3">z</div><ApFavoriteButton',
    expect: 'G22',
  },
  {
    gate: 'doctrine',
    name: 'G22 页面手搓横向 scaleX 填充（left center 是进度条签名）',
    file: HOME, from: '<ApFavoriteButton',
    to: `<div :style="{ transformOrigin: 'left center' }">z</div><ApFavoriteButton`,
    expect: 'G22',
  },
  {
    gate: 'doctrine',
    name: 'G22 页面用模板字面量的百分比宽度自造填充',
    file: HOME, from: '<ApFavoriteButton',
    to: '<div :style="{ width: `${p}%` }">z</div><ApFavoriteButton',
    expect: 'G22',
  },
  {
    gate: 'doctrine',
    name: 'G23 页面手搓一份浮层表面（下拉阴影是它的签名）',
    file: HOME, from: '<ApFavoriteButton',
    to: `<div :style="{ boxShadow: cv('shadow-dropdown'), borderRadius: AppleRadius.medium }">z</div><ApFavoriteButton`,
    expect: 'G23',
  },
  {
    gate: 'layout',
    name: 'R16 白名单外的文件蹭主对象档（阴影例外一旦扩散就回到"靠阴影喊"）',
    file: HOME, from: '<ApFavoriteButton',
    to: "<div :style=\"primarySurfaceStyle()\">z</div><ApFavoriteButton",
    expect: 'surface-primary',
  },
  {
    gate: 'doctrine',
    name: 'G28 用按钮颜色表达选中态（第二套选中语言，没有 aria 状态也没有键盘）',
    file: HOME, from: '<ApFavoriteButton',
    to: `<ApButton v-for="o in opts" :key="o" :variant="pick === o ? 'primary' : 'ghost'">{{ o }}</ApButton><ApFavoriteButton`,
    expect: 'G28',
  },
  {
    gate: 'doctrine',
    name: 'G27 过渡点到布局属性（运行时那台看不见没渲染的元素）',
    file: HOME, from: '<ApFavoriteButton',
    to: "<div :style=\"{ transition: 'width 0.3s ease' }\">z</div><ApFavoriteButton",
    expect: 'G27',
  },
  {
    gate: 'doctrine',
    name: 'G26 自己再写一份全屏遮罩（fixed + overlay 令牌，z 档与 blur 从此各走各的）',
    file: HOME, from: '<ApFavoriteButton',
    to: "<div :style=\"{ position: 'fixed', inset: '0', background: cv('overlay') }\">z</div><ApFavoriteButton",
    expect: 'G26',
  },
  {
    gate: 'doctrine',
    name: 'G25① 组件里命令式改 .style.transform（在页面里重写一套物理）',
    file: HOME, from: '<ApFavoriteButton',
    to: '<div @click="() => { (0, eval)(\'x\'); }" @pointerdown="el => el && (el.style.transform = \'translateX(1px)\')"></div><ApFavoriteButton',
    expect: 'G25',
  },
  {
    gate: 'doctrine',
    name: 'G25② 自己排 rAF 拼 transform（第二套逐帧动画）',
    file: HOME, from: '<ApFavoriteButton',
    to: "<div @click=\"() => requestAnimationFrame(() => {})\" :style=\"{ transform: 'translate3d(0, 1px, 0)' }\">z</div><ApFavoriteButton",
    expect: 'G25',
  },
  {
    gate: 'doctrine',
    name: 'G25③ fixed 层回到 translate(-100%) + transition 的 keyframe 重放',
    file: HOME, from: '<ApFavoriteButton',
    to: "<div :style=\"{ position: 'fixed', transform: 'translateX(-100%)', transition: 'transform 0.3s ease' }\"></div><ApFavoriteButton",
    expect: 'G25',
  },
  {
    gate: 'layout',
    name: 'R15 页头 h1 漏挂锚点（运行时页头量测会查无元素、静默不量）',
    file: HOME, from: '<ApFavoriteButton',
    to: '<h1 class="zz-title-probe">标题</h1><ApFavoriteButton',
    expect: '没挂 data-ap-page-title',
  },
  {
    gate: 'doctrine',
    name: 'G24 又一个下拉面板没接共用退路（用户报的"点别处不收"）',
    file: HOME, from: '<ApFavoriteButton',
    to: '<div v-if="showZzMenu" role="menu">z</div><ApFavoriteButton',
    expect: 'G24',
  },
  {
    gate: 'doctrine',
    name: 'G24 自己再写一份 document 级 outside 监听（第 N 份实现）',
    file: HOME, from: '<ApFavoriteButton',
    to: "<span @click=\"document.addEventListener('mousedown', onGlobalMouseZz, true)\">z</span><ApFavoriteButton",
    expect: 'G24',
  },
  {
    gate: 'doctrine',
    name: 'G29 弹窗宽度绕过 AppleModalSize（组件已带 maxWidth:90vw，页面再包一层 min() 就是尺外再加一档）',
    file: HOME, from: '<ApFavoriteButton',
    to: '<ApModal v-model="showZz" title="t" width="min(640px, 90vw)"><i /></ApModal><ApFavoriteButton',
    expect: 'G29',
  },
  {
    gate: 'layout',
    name: 'R8 面包屑锚点被改名（parity/check:ui 会静默退回历史选择器，"没量到"看起来就是"没问题"）',
    file: CRUMB, from: 'data-ap="breadcrumb"',
    to: 'data-ap="zz-crumb-renamed"',
    expect: '没挂 data-ap="breadcrumb" 锚点',
  },
  {
    gate: 'layout',
    name: 'R8 容器又自己引面包屑样式（两套 DOM 从此各长各的：一份有 ol/li，一份没有）',
    file: CONTAINER, from: '<ApBreadcrumb',
    to: '<nav :style="{ ...breadcrumbRowStyle }">z</nav><ApBreadcrumb',
    expect: '容器又自己引了面包屑样式',
  },
  {
    gate: 'doctrine',
    name: 'G30 手抄缓动曲线（值一样也是第二处定义，改一次要动 8 个文件）',
    file: HOME, from: '<ApFavoriteButton',
    to: "<div :style=\"{ transition: 'opacity 0.3s cubic-bezier(0.4, 0, 0.2, 1)' }\">z</div><ApFavoriteButton",
    expect: 'G30',
  },
  {
    gate: 'doctrine',
    name: 'G31 第二张字号表复活（同名异值 + 天阶外字号就是这么来的）',
    file: HOME, from: '<ApFavoriteButton',
    to: '<div :style="{ fontSize: AppleFontSize.micro }">z</div><ApFavoriteButton',
    expect: 'G31',
  },
  {
    gate: 'doctrine',
    name: 'G31 控件尺寸表里写裸 px 字号（字号只准住在 AppleType）',
    file: TOKENS, from: "mini: { height: '24px', padding: '0 8px', fontSize: AppleType.microUI.size }",
    to: "mini: { height: '24px', padding: '0 8px', fontSize: '12px' }",
    expect: 'G31',
  },
  {
    gate: 'layout',
    name: 'R18 <ApTable> 又写 size=（没写 inheritAttrs:false，vue-tsc 会当 fallthrough 放行，表格静默退回默认密度）',
    file: HOME, from: '<ApFavoriteButton',
    to: '<ApTable size="compact" :columns="[]" :data="[]" /><ApFavoriteButton',
    expect: 'R18',
  },
  {
    gate: 'layout',
    name: 'R17 手写固定列网格（R10 只认 minmax(<数字>px，这一族整类从它眼下走过）',
    file: HOME, from: '<ApFavoriteButton',
    to: "<div :style=\"{ display: 'grid', gridTemplateColumns: 'repeat(2, minmax(0, 1fr))' }\">z</div><ApFavoriteButton",
    expect: '手写固定列网格（R17）',
  },
  // ↓ 五条专打"判据自己骗自己"：老写法在这几条上永远绿，因为它们数的是子串/窗口文本。
  {
    gate: 'doctrine',
    name: 'G19 关键帧名字是别人名字的一段（pulse ⊂ apple-pulse-soft，\\b 把连字符当词边界就数到 2 次"有人用"）',
    file: TOKENS, from: '@keyframes apple-pulse-soft {',
    to: '@keyframes pulse { 0% { opacity: 1 } }\n@keyframes apple-pulse-soft {',
    expect: '关键帧 pulse 零消费者',
  },
  {
    gate: 'doctrine',
    name: 'G19 工具类被更长的类名和 Monaco 主题 id 冒充成有人用（.apple-glass ⊂ .apple-glass-nav）',
    file: TOKENS, from: '.apple-glass-nav {',
    to: '.apple-glass { color: red; }\n.apple-glass-nav {',
    expect: '工具类 .apple-glass 零消费者',
  },
  {
    gate: 'layout',
    name: 'R10 手写 minmax(230px,1fr) 当卡片地板（四档之外即漂移）',
    file: HOME, from: '<ApFavoriteButton',
    to: "<div :style=\"{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(230px, 1fr))' }\">z</div><ApFavoriteButton",
    expect: '手写 minmax(230px',
  },
  {
    gate: 'layout',
    name: 'R11 entity 网格没设等高（同一对象里确实没有 gridAutoRows）',
    file: HOME, from: '<ApFavoriteButton',
    to: "<div :style=\"{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, ' + AppleGridFloor.entity + ')' }\">z</div><ApFavoriteButton",
    expect: 'entity 地板却没设 gridAutoRows',
  },
  {
    gate: 'layout',
    name: 'R11 等高只写在注释里也算违例（老判据取上下 15 行文本窗口，注释能把它骗过去）',
    file: HOME, from: '<ApFavoriteButton',
    to: "<div :style=\"{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, ' + AppleGridFloor.entity + ')' /* gridAutoRows 这里只是注释 */ }\">z</div><ApFavoriteButton",
    expect: 'entity 地板却没设 gridAutoRows',
  },
  // ── 材质层：玻璃档的地板、站点账本与 materialize 接线 ──
  {
    gate: 'contrast',
    name: '浮层壳面透明度被调低（顶栏 under 态压在任意内容上，实心档的门槛量不到它）',
    file: THEMES, from: "'--apple-surface-elevated': 'rgba(255,255,255,0.88)'",
    to: "'--apple-surface-elevated': 'rgba(255,255,255,0.30)'",
    expect: 'elevated/次级/浮层壳面 对比度',
  },
  {
    // 这一条是 2026-10-15 拆档之后才存在的判据：拆档前菜单和顶栏共用 elevated，
    // 谁把菜单调透就会连带把顶栏的地板拆掉，判据只有一条 elevated 断言、也就能拦住它。
    // 拆成 float 之后顶栏管不着菜单了，所以浮层面板档必须**自己**有一组断言，
    // 否则"把 float 调到 0.3"会一路绿灯到肉眼才发现文字糊了。
    gate: 'contrast',
    name: '浮层面板档被调到没有地板（菜单/弹窗这一档的最坏背景是暗图而非纯黑，但它也得有地板）',
    file: THEMES, from: "'--apple-surface-float': 'rgba(250,252,255,0.78)'",
    to: "'--apple-surface-float': 'rgba(250,252,255,0.30)'",
    expect: 'float/三级/浮层面板 对比度',
  },
  {
    // 自适应那一层能走到的两端由门禁钉住；解算器住在 glass-contrast.ts，
    // 运行时与判据共用同一份 —— 所以把上限改小，必须在这一节就红，
    // 而不是等到某个菜单真的压上一张暗图。
    gate: 'contrast',
    name: '浮层自适应的上限被调低（身后是读不出亮度的位图时，加厚那一档必须扛得住极值内容）',
    file: GLASSMATH, from: 'export const GLASS_FLOAT_ALPHA_CEIL = 0.94;',
    to: 'export const GLASS_FLOAT_ALPHA_CEIL = 0.50;',
    expect: '自适应浮层',
  },
  {
    gate: 'glass',
    name: '一处写死的 blur 数值（不跟随材质切换，切到 solid/纸感时白付一次 GPU）',
    file: CARD, from: "backdropFilter: cv('blur-panel'),",
    to: "backdropFilter: 'blur(20px)',",
    expect: '不是材质令牌',
  },
  {
    gate: 'glass',
    name: '浮层用静止档铺底（聚光灯面板浮在正在滚动的列表上，却拿卡片那一档）',
    file: SPOTLIGHT, from: "background: cv('surface-float'),",
    to: "background: cv('surface-panel'),",
    expect: '用了静止档 surface-panel',
  },
  {
    gate: 'glass',
    name: 'materialize 的类名被删（模糊不在元素自己身上，backdrop-filter 站点数看不出这种丢失）',
    file: MODAL, from: 'class="apple-glass-layer" data-ap="modal"',
    to: 'data-ap="modal"',
    expect: '类名已经不在这份文件里',
  },
  {
    gate: 'glass',
    name: '多加一处玻璃却没进登记表（玻璃层有预算 ≤3）',
    file: CARD, from: "backdropFilter: cv('blur-panel'),",
    to: "backdropFilter: cv('blur-panel'),\n    WebkitBackdropFilter: cv('blur-panel'),\n    backdropFilter: cv('blur-panel'),",
    expect: '的逻辑 backdrop-filter 站点数 = 3',
  },
  {
    gate: 'layout',
    name: 'R12 走工具类这条间接，但工具类指向了别的档（间接层不能变成新的绕过路径）',
    file: TOKENS, from: '.apple-glass-nav {\n  background: var(--apple-surface-chrome);',
    to: '.apple-glass-nav {\n  background: var(--apple-surface-panel);',
    expect: '在令牌层里并不取 surface-chrome',
  },
  {
    gate: 'layout',
    name: 'R19 导航滚动容器又开始只写一轴（横轴被 CSS 补成 auto → 横向条与竖向条互相挤，两条一起闪）',
    // 锚点跟着 navScrollStyle 的三元结构走：展开态那一支从 scrollSingleAxis 换成裸单轴对象
    file: SIDEBAR, from: '    : scrollSingleAxis),',
    to: "    : { overflowY: 'auto' as const }),",
    expect: '单轴滚动容器只声明了 overflowY',
  },
  {
    gate: 'glass',
    name: '材质层少了无障碍信号 prefers-contrast（只实现 reduced-transparency 看着像做完了，两条在系统里各开各的）',
    file: THEMES, from: '@media (prefers-contrast: more)',
    to: '@media (prefers-contrast-harder: more)',
    expect: '缺无障碍信号 @media (prefers-contrast: more)',
  },
  {
    gate: 'glass',
    name: 'materialize 的债账只按登记表遍历 —— 新消费者恰恰不在任何表里，按表循环等于这条规则永不命中',
    file: CARD, from: '<script setup lang="ts">',
    to: "<script setup lang=\"ts\">\nconst rvNewPanel = menuSurfaceStyle({ position: 'fixed' });",
    expect: '却没登记进 MATERIALIZED',
  },
  {
    gate: 'glass',
    name: '浮层档表面没铺玻璃（用户报的"很多弹窗不是液态玻璃"：表面自己写 surface-float，却没有任何模糊）',
    // 目标从 FloatingToc 换成 MobileTocFab：前者这一轮改成走 menuSurfaceStyle() 取表面，
    // 文件里不再有 surface-* 字面量，这条判据（按字面量认浮层档）就扫不到它了 ——
    // 它改由 MATERIALIZED 那条判据管。留在这里会让本条断言静默失去对象，所以换个还自己写表面的。
    file: MOCTOC, from: 'class="apple-glass-layer"',
    to: '',
    expect: '铺浮层表面却没有玻璃',
  },
  {
    gate: 'style',
    name: 'padding 简写盖掉长写：键序即声明序，长写被静默重置而界面不报错（搜索框就是这么跑到顶栏背后的）',
    // 锚点跟着 sidebarStyle 的三元写法走：在 padding 简写之前塞一个 paddingTop 长写
    file: SIDEBAR,
    from: '  padding: collapsed.value\n    ? `calc(var(--apple-shell-bar-height) + ${AppleSpace.md}) 0 0 0`',
    to: "  paddingTop: '64px',\n  padding: collapsed.value\n    ? `calc(var(--apple-shell-bar-height) + ${AppleSpace.md}) 0 0 0`",
    expect: '长写会被静默重置',
  },
  {
    // G23 的旧签名只认 `shadow-dropdown` 一个字面量，于是 2026-10-15 有两处手搓浮层表面
    // 从它眼下走过（引文预览用 level3、移动端抽屉用 shadow-card）。
    // 这条注入用**同样的 level3 组合**，证明加宽后的结构签名真能抓到旧判据漏掉的那一类。
    gate: 'doctrine',
    name: '手搓浮层表面但绕开 shadow-dropdown 签名（G23 旧判据对这一类永远绿）',
    file: CARD, from: '<script setup lang="ts">',
    to: "<script setup lang=\"ts\">\nconst rvHandMenu = { position: 'fixed', zIndex: 1000, background: cv('surface-elevated'), boxShadow: cv('shadow-level3') };",
    expect: 'G23b',
  },
  {
    gate: 'theme',
    name: '组件里写死非黑白的字面颜色（T11 只拦黑/白，这一族整类从它眼下走过）',
    file: CARD, from: '<script setup lang="ts">',
    to: "<script setup lang=\"ts\">\nconst rvVendorHex = '#10A37F';",
    expect: 'T13',
  },
  {
    // T6 只保证"每套主题都登记了渲染器调色板"，看不见**新画布消费点根本没去取调色板**：
    // echarts 三处 init 全绿地把服务端给的 option 直接画进 canvas，切深色时静态门禁一条都不红。
    gate: 'theme',
    name: '新增一处 echarts.init 而不接 echarts-theme（T6 对这类永远绿）',
    file: CARD, from: '<script setup lang="ts">',
    to: "<script setup lang=\"ts\">\nfunction rvInitChart(el) { return echarts.init(el); }",
    expect: 'T14',
  },
  {
    // R19 原先只遍历 3 个文件的登记表：新写一个裸 overflowY 容器恰恰不会进那张表，
    // 于是这条判据对新消费者永远绿。R19b 改成全 src 扫描 + 只降不升基线，这条注入证明它真能涨红。
    gate: 'layout',
    name: '新写一个只声明 overflowY 的滚动容器（R19 按登记表遍历对这类永远绿）',
    file: CARD, from: '<script setup lang="ts">',
    to: "<script setup lang=\"ts\">\nconst rvScroller = { overflowY: 'auto', maxHeight: '200px' };",
    expect: 'R19b',
  },
  {
    // 兜底位硬编码中文是**运行时看不见**的一族：`|| '文档'` 只在数据缺失时才渲染，
    // 库里记录都有名字，check:i18n-runtime 51 条路由全绿，它一处也不会暴露。
    gate: 'style',
    name: '新增一处兜底位硬编码中文（运行时 i18n 巡检永远量不到，因为兜底不渲染）',
    file: CARD, from: '<script setup lang="ts">',
    to: "<script setup lang=\"ts\">\nconst rvFallback = props.name || '未命名条目';",
    expect: '兜底位硬编码中文 数量',
  },
  // ── 个性化批1/批2：动效时长、行为档入口、护栏 ──
  // 这四条是这一批自己造的判据，所以按同一批的规矩立刻自证一次：
  // 断言写错的门禁会永远绿，而"时长没乘倍数"这件事肉眼看不出、单测也不跑浏览器。
  {
    gate: 'doctrine',
    name: 'G32：动效时长写回裸字面量（个性化调不到它）',
    file: MODAL, from: 'transition: opacity var(--apple-dur-default) var(--apple-easing-standard);',
    to: 'transition: opacity 0.27s var(--apple-easing-standard);',
    expect: 'G32',
  },
  {
    gate: 'theme',
    name: 'T15：动效档有清单但没倍数（undefined 会被写进 CSS）',
    file: PERS, from: '  reduced: 0.0001,', to: '  // rv 探针：删掉这一档的倍数',
    expect: '没有倍数',
  },
  {
    gate: 'theme',
    name: 'T15：时长表里加了档但没发令牌（引用它的 transition 整条失效）',
    file: TOKENS, from: "  '--apple-dur-fast': `calc(${BASE_DURATION_MS.fast}ms * var(--apple-motion-scale))`,\n", to: '',
    expect: '没发令牌',
  },
  {
    gate: 'glass',
    name: '材质层缺应用内入口：偏好页的对比度档没有对应 CSS 块',
    file: THEMES, from: "${g}[data-a11y-contrast='more'] {", to: "${g}[data-a11y-contrastRV='more'] {",
    expect: '缺应用内/护栏入口',
  },
  {
    gate: 'glass',
    name: '护栏被拆：应用内选了 full 也会被系统的减弱动效通配压掉',
    file: PERS, from: `export const MOTION_GUARD = ":root:not([data-motion='full']):not([data-motion='soft'])";`,
    to: 'export const MOTION_GUARD = ":root";',
    expect: '缺应用内/护栏入口',
  },
  // ── 批3：强调色 ──
  {
    gate: 'theme',
    name: 'T16：色板里少一枚 hex（运行时会把 --apple-blue 覆写成 "undefined"）',
    file: ACCENTS,
    from: "{ id: 'purple', labelKey: 'chrome.accentPurple', light: '#7b3697', dark: '#a161bd' },",
    to: "{ id: 'purple', labelKey: 'chrome.accentPurple', light: '#7b3697' },",
    expect: 'T16',
  },
  {
    gate: 'doctrine',
    name: 'G33：实心强调色上又用回恒白 on-accent',
    file: HOME, from: "    background: cv('blue'),\n    color: cv('on-blue'),",
    to: "    background: cv('blue'),\n    color: cv('on-accent'),",
    expect: 'G33',
  },
  // ── 批4：版面人格的入场编排 ──
  {
    gate: 'doctrine',
    name: 'G34：产品内页（面包屑）偷用入场编排',
    file: CRUMB, from: '    <ol :style="listStyle">',
    to: '    <ol :style="listStyle" class="apple-persona-stack">',
    expect: 'G34',
  },
  {
    gate: 'theme',
    name: 'T15：persona 清单里没有默认档 plain',
    file: PERS, from: "PERSONA_MODES = ['plain', 'signature']",
    to: "PERSONA_MODES = ['signature']",
    expect: "里没有 'plain'",
  },
  {
    gate: 'theme',
    name: 'T17：有一条编排规则没被档位套住（默认档也会动）',
    file: TOKENS, from: ":root[data-persona='signature'] .apple-persona-stack > :nth-child(-n + 6) {",
    to: ".apple-persona-stack > :nth-child(-n + 6) {",
    expect: "没被 :root[data-persona='signature'] 套住",
  },
  {
    gate: 'theme',
    name: 'T17：关键帧里动了布局属性（G17/G27 管不到 @keyframes）',
    file: TOKENS, from: '  from { opacity: 0; transform: translateY(0.6rem); }',
    to: '  from { opacity: 0; width: 0.6rem; }',
    expect: '动了布局属性',
  },
];

const runGate = (gate) => {
  const argv = gate === 'layout' ? [GATES.layout, TMP] : [GATES[gate]];
  try {
    return { code: 0, out: execFileSync(process.execPath, argv, { encoding: 'utf8', cwd: gate === 'layout' ? WEB : TMP }) };
  } catch (e) {
    return { code: e.status ?? 1, out: String(e.stdout || '') + String(e.stderr || '') };
  }
};

fs.rmSync(TMP, { recursive: true, force: true });
fs.cpSync(path.join(WEB, 'src'), SRC, { recursive: true });
// 也带上 scripts：R14 有一条"令牌里的变量名 ↔ 运行时读的变量名"要对齐的检查，
// 探针脚本本身不在副本里就没法验证那一支。
fs.cpSync(path.join(WEB, 'scripts'), path.join(TMP, 'scripts'), { recursive: true });

let bad = 0;
try {
  const base = runGate('layout');
  if (base.code !== 0) { console.log('FAIL  副本基线就不绿，后面的注入结果都不可信\n' + base.out.split('\n').slice(0, 8).join('\n')); process.exit(1); }

  for (const c of cases) {
    const orig = fs.readFileSync(c.file, 'utf8');
    if (!orig.includes(c.from)) {
      console.log(`FAIL  ${c.name}\n      锚文本在源码里找不到：${c.from}（断言可能已与代码脱节）`);
      bad++;
      continue;
    }
    fs.writeFileSync(c.file, orig.replace(c.from, c.to));
    const { code, out } = runGate(c.gate);
    fs.writeFileSync(c.file, orig);
    if (code !== 0 && out.includes(c.expect)) console.log(`PASS  [${c.gate}] ${c.name}`);
    else {
      console.log(`FAIL  [${c.gate}] ${c.name}\n      期望命中"${c.expect}"，实际 exit=${code}`);
      console.log(out.split('\n').filter((l) => l.startsWith('✗') || l.startsWith('G1') || l.startsWith('T1') || l.startsWith('R1')).slice(0, 5).join('\n'));
      bad++;
    }
    // 每条之间确认还原干净：上一处违例串进下一轮会让后面的 PASS 变成假 PASS
    if (runGate(c.gate).code !== 0) { console.log(`FAIL  ${c.name}：还原后门禁未恢复绿色`); bad++; }
  }
} finally {
  fs.rmSync(TMP, { recursive: true, force: true });
}

console.log(bad ? `\n✗ 反向验证未通过：${bad} 条断言抓不到` : `\n✓ ${cases.length} 条断言都能被抓到，且还原后门禁回绿（无误伤）`);
process.exit(bad ? 1 : 0);
