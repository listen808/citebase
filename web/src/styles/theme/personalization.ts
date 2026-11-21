/**
 * 个性化档位（动效强度 / 对比度 / 透明度）的**唯一真相源**。
 *
 * 为什么单开一个文件而不是塞进 `themes.ts`：`themes.ts` 管的是「调色板」——
 * 外观 × 材质两轴，值都是颜色与模糊；这一层管的是「行为」——时长倍数、描边是否画实、
 * 表面是否退回不透明。两层的门禁完全不同（前者由 `check:theme` 的 72 个必填 token 管，
 * 后者由 `check:glass` / `check:motion` 管），混在一个文件里迟早会让"加一档颜色"的人
 * 顺手以为自己在加一档动效。
 *
 * **三条档位都有 `auto`，且 `auto` 是默认值**：默认路径的行为必须由系统信号决定，
 * 和今天一模一样。应用内的显式选择盖在系统信号之上 —— 这是本仓唯一一处「应用设置覆盖系统设置」，
 * 理由写在下面的 `resolveMotionMode` 注释里，改之前先读那段。
 *
 * 清单同时被服务端读走一份做校验（`internal/model/user/preferences.go`），
 * 两边分叉的后果不是"新档位不能用"，而是**用户在另一台设备上的选择被当非法值丢掉**，
 * 由 `tools/repocheck/preference_contract_test.go` 逐条比对。
 */

/** 动效强度：full 保持设计时长，soft 全体降速降幅，reduced 走「不位移、只淡入淡出」那一档 */
export const MOTION_MODES = ['auto', 'full', 'soft', 'reduced'] as const;
export type MotionMode = (typeof MOTION_MODES)[number];

/**
 * 时长倍数。`reduced` 给的不是 0 而是 0.0001 —— 因为 `calc(150ms * 0)` 会让一部分
 * 浏览器把 transition 判成非法值而回落到 0s，两者行为相同但前者是"猜"，后者是"没有过渡"。
 * 真正的减弱靠 `:root[data-motion='reduced']` 那组 `!important` 通配（把无限循环动画的
 * iteration-count 也压成 1，这是倍数做不到的），倍数只负责"量不到的那条路径也别跑得欢"。
 */
export const MOTION_SCALE: Record<Exclude<MotionMode, 'auto'>, number> = {
  full: 1,
  soft: 0.6,
  reduced: 0.0001,
};

/** 对比度：standard = 今天的描边，more = 近实心 + 把描边一族画实 */
export const CONTRAST_MODES = ['auto', 'standard', 'more'] as const;
export type ContrastMode = (typeof CONTRAST_MODES)[number];

/** 透明度：full = 材质说什么就是什么，reduce = 玻璃退回该外观的不透明表面 */
export const TRANSPARENCY_MODES = ['auto', 'full', 'reduce'] as const;
export type TransparencyMode = (typeof TRANSPARENCY_MODES)[number];

/**
 * 版面人格（批4）：公开版面的入场编排。`plain` = 今天的样子，静置无入场。
 *
 * **这一档没有 `auto`，因为它不映射到任何系统信号** —— 系统里没有"要不要入场动画"这个开关。
 * 默认必须是 `plain`：HIG 的原话是"不要在高频交互上加动效"，而新访客的第一眼如果就带编排，
 * 那等于替所有用户做了这个决定。
 *
 * 生效射程由 `check:design` 的 G34 钉住：编排只准出现在登记的公开版面
 * （登录 / 授权登录 / 安装向导 / 分享页 / 错误页）。产品内页出现即红 ——
 * 这条边界是这一档存在的前提，不是建议。
 */
export const PERSONA_MODES = ['plain', 'signature'] as const;
export type PersonaMode = (typeof PERSONA_MODES)[number];

/** 三条档位的默认值：全 auto = "这台设备没表过态"，行为完全由系统信号决定 */
export const DEFAULT_MOTION_MODE: MotionMode = 'auto';
export const DEFAULT_CONTRAST_MODE: ContrastMode = 'auto';
export const DEFAULT_TRANSPARENCY_MODE: TransparencyMode = 'auto';
/** 默认静置：新访客看到的还是今天这个样子（改这一行请先读 PERSONA_MODES 上面那段） */
export const DEFAULT_PERSONA_MODE: PersonaMode = 'plain';

export function isMotionMode(v: unknown): v is MotionMode {
  return typeof v === 'string' && (MOTION_MODES as readonly string[]).includes(v);
}
export function isContrastMode(v: unknown): v is ContrastMode {
  return typeof v === 'string' && (CONTRAST_MODES as readonly string[]).includes(v);
}
export function isTransparencyMode(v: unknown): v is TransparencyMode {
  return typeof v === 'string' && (TRANSPARENCY_MODES as readonly string[]).includes(v);
}
export function isPersonaMode(v: unknown): v is PersonaMode {
  return typeof v === 'string' && (PERSONA_MODES as readonly string[]).includes(v);
}

/**
 * 系统信号读取。**每条都是独立信号**（系统里可以只开「提高对比度」而不开「减弱动态效果」），
 * 合并成一个函数就是漏掉一批用户 —— 材质层那三条 @media 已经是分开的，这里保持同样的粒度。
 * 取不到 matchMedia（SSR / 单测环境）时一律返回 false，即"按默认档走"。
 */
function systemSignal(query: string): boolean {
  if (typeof window === 'undefined' || !window.matchMedia) return false;
  try {
    return window.matchMedia(query).matches;
  } catch {
    return false;
  }
}

export function systemReduceMotion(): boolean {
  return systemSignal('(prefers-reduced-motion: reduce)');
}
export function systemPrefersMoreContrast(): boolean {
  return systemSignal('(prefers-contrast: more)');
}
export function systemReduceTransparency(): boolean {
  return systemSignal('(prefers-reduced-transparency: reduce)');
}

/**
 * 偏好 + 系统信号 → 实际生效的动效档。
 *
 * **为什么应用内可以选 `full` 覆盖系统的「减弱动态效果」**：这一档的默认值是 `auto`，
 * 也就是"没表过态的人继续尊重系统"——实测新用户 100% 走这条路径。而显式选了 `full`
 * 是用户在**这个应用的上下文里**更新的一次表态（且随时能改回去），
 * 比一次全局开关更能代表他现在想要什么。Apple 的原话是 "Make motion optional"，
 * 可选性本来就是双向的：不许关是另一种强制。
 * 代价写在这里：账号同步会把这台的选择带到那台，所以跨设备时系统开关可能与应用档位相反 ——
 * 界面上这一档必须显示"当前生效的是哪一档"，不能只显示所选档位（`ApThemeMenu` 的标签负责这件事）。
 */
export function resolveMotionMode(pref: MotionMode): Exclude<MotionMode, 'auto'> {
  if (pref === 'auto') return systemReduceMotion() ? 'reduced' : 'full';
  return pref;
}

export function resolveContrastMode(pref: ContrastMode): Exclude<ContrastMode, 'auto'> {
  if (pref === 'auto') return systemPrefersMoreContrast() ? 'more' : 'standard';
  return pref;
}

export function resolveTransparencyMode(pref: TransparencyMode): Exclude<TransparencyMode, 'auto'> {
  if (pref === 'auto') return systemReduceTransparency() ? 'reduce' : 'full';
  return pref;
}

/** 实际生效的时长倍数：DOM 上的 `--apple-motion-scale` 由它决定 */
export function motionScaleFor(mode: Exclude<MotionMode, 'auto'>): number {
  return MOTION_SCALE[mode];
}

/**
 * **CSS 侧的"这台设备没表过态"护栏**（`data-motion` 缺席或等于 reduced 时，系统信号说话）。
 *
 * 住在这一层而不是 apple-tokens.ts，是因为注入层与主题层两边都要用它：
 * 材质层那条「减弱动效时环境层不再跟随视口」在 `themes.ts`，全局通配在 `apple-tokens.ts`。
 * 一条判断写成两处，就会出现"系统开了减弱动效，卡片背景还在跟着滚"这类半接线的洞。
 */
export const MOTION_GUARD = ":root:not([data-motion='full']):not([data-motion='soft'])";

/** 提高对比度的护栏（同一条理由：显式选了 standard 就是压过系统的 more） */
export const CONTRAST_GUARD = ":root:not([data-a11y-contrast='standard'])";

/** 减弱透明度的护栏（显式选了 full 就是压过系统的 reduce） */
export const TRANSPARENCY_GUARD = ":root:not([data-a11y-transparency='full'])";

/**
 * JS 侧的时长换算（`AppleDuration` 那些毫秒数：等待动画结束、动画关闭等）。
 * CSS 侧由 `--apple-motion-scale` 乘，JS 侧必须乘同一个数，否则会出现
 * "CSS 还在动、JS 已经认为动完了"的那类竞态。
 * 取不到 DOM（单测）时按原值返回，不猜。
 */
export function scaledMs(ms: number): number {
  return Math.round(ms * motionScaleValue());
}

/**
 * 把三条档位写到 DOM 上。**三条都是恒写**（不是"只在加强档时写"）：
 * 缺席必须只意味着一件事 —— "这台设备的 JS 还没跑 / 没表过态"，于是 CSS 里那些
 * `:not([data-a11y-...])` 护栏可以放心地让系统信号接管。
 * 如果缺席同时表示"用户选了默认档"，那"没跑 JS"和"跑过 JS 且选了 standard"
 * 就成了同一个状态，系统开关与用户选择冲突时谁赢就没法判了。
 *
 * 倍数以**行内覆写令牌本身**的方式给（`--apple-motion-scale`），不在令牌里嵌一层：
 * 自定义属性的 `var()` 在声明它的那个元素上就替换完了，后代覆写追不回来，
 * 这条是 `docs/engineering/theme-system.md` 里「一个 CSS 事实」那条记的学费。
 */
export function applyPersonalizationAttrs(input: {
  motion: Exclude<MotionMode, 'auto'>;
  contrast: Exclude<ContrastMode, 'auto'>;
  transparency: Exclude<TransparencyMode, 'auto'>;
  persona: PersonaMode;
}): void {
  if (typeof document === 'undefined') return;
  const root = document.documentElement;
  root.setAttribute('data-motion', input.motion);
  root.style.setProperty('--apple-motion-scale', String(motionScaleFor(input.motion)));
  root.setAttribute('data-a11y-contrast', input.contrast);
  root.setAttribute('data-a11y-transparency', input.transparency);
  // persona 恒写：CSS 那一批入场规则全部挂在 `:root[data-persona='signature']` 下，
  // 缺席（JS 没跑）就等于 plain —— 不许出现"没跑 JS 反而看到编排"的那种反向默认。
  root.setAttribute('data-persona', input.persona);
}

/**
 * 当前生效的动效档（JS 侧动效实现用它判断，不再各自 matchMedia）。
 * **`data-motion` 缺席时回落系统信号** —— 与 CSS 侧 `MOTION_GUARD` 那条护栏同一个口径：
 * 没表过态 = 尊重系统。两处判断口径不一样的话，JS 弹簧和 CSS 过渡会对"现在是不是减弱档"
 * 各说一遍，症状是"页面一半动一半不动"。
 */
export function currentMotionMode(): Exclude<MotionMode, 'auto'> {
  if (typeof document === 'undefined') return systemReduceMotion() ? 'reduced' : 'full';
  const attr = document.documentElement.getAttribute('data-motion');
  if (attr === 'soft' || attr === 'reduced' || attr === 'full') return attr;
  return systemReduceMotion() ? 'reduced' : 'full';
}

/** 当前生效的时长倍数（读行内覆写，读不到就按生效档位查表；永不返回 NaN） */
export function motionScaleValue(): number {
  if (typeof document !== 'undefined') {
    const raw = document.documentElement.style.getPropertyValue('--apple-motion-scale');
    const scale = Number(raw);
    if (Number.isFinite(scale) && scale > 0) return scale;
  }
  return MOTION_SCALE[currentMotionMode()];
}

/**
 * 弹簧的 `response`（秒）缩放：**除以**倍数 —— 倍数 < 1 表示"动得更慢"，
 * 而 response 越小到位越快，所以这里必须是除号。写反的症状是"选了柔和档，手势反而更冲"。
 */
export function scaledResponse(seconds: number): number {
  return seconds / motionScaleValue();
}
