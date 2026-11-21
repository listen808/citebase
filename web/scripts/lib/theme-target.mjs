/**
 * 运行时工装的主题轮：把「这一轮要量哪个外观 × 哪个材质」解析成可写进 localStorage 的形状。
 *
 * 为什么单独成文件：主题系统早就从「5 套主题」重构成了**两条正交轴**
 * （base = light|dark|editorial|warm-paper，material = solid|glass），
 * 而 `check:elements` 与 `snap` 这两个会轮主题的工装只写 `ui:theme`，从不写 `ui:material`。
 * 于是 `--themes=...,liquid-glass,...` 那一轮被 `LEGACY_THEME_ALIAS` 解成 base=light，
 * 材质却留在默认 solid —— 看着量了五套，实际把 light 量了两遍，而**唯一真正需要量的玻璃材质
 * 一次都没量过**（玻璃是全站唯一一个表面 rgba 0.58~0.86 + backdrop-filter 的形态，
 * 也是历史上"输入框蹭页面底变成一个洞"翻过车的那一档）。
 * "没量到"和"没问题"打印出来一模一样，这条判据就是为了让前者不再冒充后者。
 */

export const BASE_IDS = ['light', 'dark', 'editorial', 'warm-paper'];
export const MATERIAL_IDS = ['solid', 'glass'];
/** 纸感/编辑部是纸，没有玻璃（与 themes.ts 的 supportsGlass 同一口径） */
export const GLASS_CAPABLE_BASES = ['light', 'dark'];

export const THEME_KEY = 'citebase:ui:theme';
export const THEME_PREF_KEY = 'citebase:ui:theme-pref';
export const MATERIAL_KEY = 'citebase:ui:material';
export const GLASS_CLEAR_KEY = 'citebase:ui:glass-clear';

/**
 * 行为与个性化五档的键（2026-10-16 批1–批4）。前缀照同一个常量写 ——
 * `check-glass-runtime.mjs` 那次把 `themeStorageEntries()` 交出来的键又拼了一次 `citebase:`，
 * 落盘成 `citebase:citebase:ui:theme`，八轮全在默认档上量而报表按玻璃档标注。
 * 那条"消费方不许二次拼前缀"的源码扫描（web/tests/theme-target.test.ts）射程就是这些导出常量。
 */
export const MOTION_KEY = 'citebase:ui:motion';
export const CONTRAST_KEY = 'citebase:ui:contrast';
export const TRANSPARENCY_KEY = 'citebase:ui:transparency';
export const ACCENT_KEY = 'citebase:ui:accent';
export const PERSONA_KEY = 'citebase:ui:persona';

/**
 * 工装写入的**基准档**。全部取显式值、一个 `auto` 都不用，是刻意的：
 * `auto` 的生效结果取决于跑巡检那台机器的系统开关，同一份脚本在两台机器上会量出两套东西，
 * 而"这台 CI 机器恰好开着减弱动效"这种事在报表里完全看不出来。
 * 基准档 = 应用默认观感（full / standard / full / auto / plain）——
 * 只有 accent 例外：它的 auto 就是"不覆写、用外观自己的强调色"，没有更确定的写法。
 */
export const BASELINE_PERSONALIZATION = {
  motion: 'full',
  contrast: 'standard',
  transparency: 'full',
  accent: 'auto',
  persona: 'plain',
};

/** 生效档 → DOM 属性名（运行时不变量按这张表核，别各写各的字符串） */
export const PERSONA_DOM_ATTRS = {
  motion: 'data-motion',
  contrast: 'data-a11y-contrast',
  transparency: 'data-a11y-transparency',
  accent: 'data-accent',
  persona: 'data-persona',
};

/**
 * 个性化档位的全量口径（§7 的 C3：跑"每档相对基准档的单因子差异"+ 一个固定组合子集，
 * 而不是外观×材质×强调色×动效×人格的 576 个笛卡尔积 —— 51 条路由 × 8 档已经会撞限流）。
 * 报表必须写清算法，不许写"已覆盖全组合"。
 */
export const PERSONALIZATION_SWEEP = [
  { ...BASELINE_PERSONALIZATION },
  { ...BASELINE_PERSONALIZATION, motion: 'soft' },
  { ...BASELINE_PERSONALIZATION, motion: 'reduced' },
  { ...BASELINE_PERSONALIZATION, contrast: 'more' },
  { ...BASELINE_PERSONALIZATION, transparency: 'reduce' },
  { ...BASELINE_PERSONALIZATION, accent: 'teal' },
  { ...BASELINE_PERSONALIZATION, persona: 'signature' },
  // 固定笛卡尔子集：一个"深色 + 玻璃 + 柔和 + 提高对比 + 非默认强调色 + 有编排"的极端档
  { motion: 'soft', contrast: 'more', transparency: 'full', accent: 'purple', persona: 'signature' },
];

/** 历史存储里的独立玻璃主题 → 双轴组合（与 src 侧 LEGACY_THEME_ALIAS 保持一致） */
const LEGACY = { 'liquid-glass': { base: 'light', material: 'glass' } };

/**
 * @returns {{ ok: true, base: string, material: string, clear: boolean, label: string }
 *          | { ok: false, error: string }}
 */
export function parseThemeTarget(spec) {
  const raw = String(spec || '').trim();
  if (!raw) return { ok: false, error: '主题名为空' };

  let base = raw;
  let material = null;
  let clear = false;

  const legacy = LEGACY[raw];
  if (legacy) {
    base = legacy.base;
    material = legacy.material;
  } else if (raw.includes('+')) {
    // 形如 light / light+glass / light+glass+clear；clear 只能跟着 glass 走
    const [head, ...rest] = raw.split('+');
    base = head;
    // 拼错的段落要当场说破，不能让 `light+frosted` 悄悄当成 `light+solid` 量一遍还报"量过了"
    const unknown = rest.filter((p) => !MATERIAL_IDS.includes(p) && p !== 'clear');
    if (unknown.length) {
      return {
        ok: false,
        error: `${raw}：不认识「${unknown.join(',')}」，可用材质 ${MATERIAL_IDS.join(' | ')}，变体 clear`,
      };
    }
    if (rest.includes('clear')) clear = true;
    material = rest.find((p) => MATERIAL_IDS.includes(p)) || null;
    // dark+clear 这种简写：clear 隐含 glass，不然它没有别的宿主
    if (clear && !material) material = 'glass';
  }

  if (clear && material !== 'glass') {
    return { ok: false, error: `${raw}：clear 是玻璃的变体，必须和 glass 一起给` };
  }
  if (!BASE_IDS.includes(base)) {
    return {
      ok: false,
      error: `不认识的外观「${base}」，可用：${BASE_IDS.join(' | ')}（玻璃请写 ${base}+glass）`,
    };
  }
  const effMaterial = material || 'solid';
  if (effMaterial === 'glass' && !GLASS_CAPABLE_BASES.includes(base)) {
    // 纸感+玻璃会被 app 侧 effectiveMaterial 悄悄塌回 solid。
    // 工装若照单跑，报表就会把"量了 solid"记成"量了 glass"—— 宁可在这里拒绝。
    return { ok: false, error: `${base} 没有玻璃形态（纸没有玻璃），要 glass 请换 ${GLASS_CAPABLE_BASES.join('/')}` };
  }

  const label = `${base}+${effMaterial}${clear ? '+clear' : ''}`;
  return { ok: true, base, material: effMaterial, clear, label };
}

/**
 * 一轮主题开始前要写的 localStorage 键。
 * **三个键一律显式写**，包括 solid 和 false —— 沿用上一轮的材质/变体去"量下一套主题"，
 * 是 check:elements 那条"每轮显式写 localStorage（含 light，否则上一轮 dark 污染下一轮）"的同一个坑。
 */
export function themeStorageEntries(target) {
  return [
    [THEME_KEY, target.base],
    [THEME_PREF_KEY, target.base],
    [MATERIAL_KEY, target.material],
    [GLASS_CLEAR_KEY, String(target.clear)],
    // 五条行为/个性化档同样**逐键显式写**：沿用上一轮的去量下一轮，
    // 与"三个键里少写一个"是同一个坑（只不过现在有八个键，症状更难猜）。
    // target.personalization 缺省时用基准档。
    ...Object.entries({ ...BASELINE_PERSONALIZATION, ...(target.personalization || {}) }).map(
      ([axis, value]) => [
        { motion: MOTION_KEY, contrast: CONTRAST_KEY, transparency: TRANSPARENCY_KEY, accent: ACCENT_KEY, persona: PERSONA_KEY }[axis],
        String(value),
      ],
    ),
  ];
}

/**
 * 全量组合：把两条轴的所有**合法**搭配一次列全（纸/编辑部没有玻璃，所以不是 4×2×2=16）。
 *
 * 这个数组存在的理由是"覆盖"这件事以前没法回答：每台工装各自写死一个默认档
 * （check:elements 只跑 light、check:glass:runtime 只跑 light+glass+clear、snap 只拍 light），
 * 于是 editorial / warm-paper / 整个 dark 系 / 任何 solid 档从未被任何自动化跑过，
 * 而 README 与清单上的话说的是"运行时已按主题巡检"。有了全量枚举，
 * `--themes=all` 是一个能真的答"跑过哪几档"的口径。
 *
 * 由 parseThemeTarget 生成而不是手抄字符串：判"什么组合合法"的规则只该有一份
 * （玻璃能力、clear 依附 glass），手抄一份就等于给这两条规则开了第二个真相源。
 */
export const ALL_THEME_TARGETS = (() => {
  const list = [];
  for (const base of BASE_IDS) {
    for (const material of MATERIAL_IDS) {
      if (material === 'glass' && !GLASS_CAPABLE_BASES.includes(base)) continue;
      for (const clear of material === 'glass' ? [false, true] : [false]) {
        const spec = `${base}${material === 'solid' ? '' : '+' + material}${clear ? '+clear' : ''}`;
        const t = parseThemeTarget(spec);
        if (!t.ok) throw new Error(`全量枚举产出了非法组合 ${spec}：${t.error}`);
        list.push(t);
      }
    }
  }
  return list;
})();

/**
 * 解析 `--themes=` 的参数值：逗号分隔的组合名，或 `all` 取全量。
 * @returns {{ ok: true, targets: Array } | { ok: false, error: string }}
 */
export function parseThemeList(value) {
  const parts = String(value || '')
    .split(',')
    .map((s) => s.trim())
    .filter(Boolean);
  if (!parts.length) return { ok: false, error: '--themes 是空的' };
  if (parts.length === 1 && parts[0].toLowerCase() === 'all') return { ok: true, targets: ALL_THEME_TARGETS };
  const targets = [];
  for (const spec of parts) {
    const t = parseThemeTarget(spec);
    if (!t.ok) return { ok: false, error: t.error };
    targets.push(t);
  }
  return { ok: true, targets };
}
