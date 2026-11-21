import { describe, it, expect, beforeEach } from 'vitest';
import {
  EXTENDED_THEME_IDS,
  isDarkThemeId,
  isThemeId,
  isThemePreference,
  supportsGlass,
  supportsGlassClear,
  resolveThemeTokens,
  GLASS_OVERLAY,
  THEME_CONTRACT_KEYS,
  THEME_IDS,
  THEME_REGISTRY,
  themeMetaFor,
  themeOverrideMapFor,
  themeSwatch,
} from './themes';
import { cssVariables, injectCSSVariables } from '@/utils/core/apple-tokens';
import {
  HIGHLIGHT_PALETTE_BY_THEME,
  HighlightColors,
  injectHighlightCSSVariables,
  MONACO_APPLE_THEMES,
  monacoThemeForUi,
} from './highlight-tokens';

/**
 * 主题契约的运行时断言。编译期的 Record<ThemeId, …> 已经拦得住漏键，
 * 这里守的是编译期拦不到的那一半：注入结果的形状、渲染器映射、以及「每套主题都被测到」。
 */
describe('主题契约', () => {
  it('注册表与 THEME_IDS 是同一批主题', () => {
    expect(THEME_REGISTRY.map((t) => t.id)).toEqual([...THEME_IDS]);
    expect(THEME_IDS).toEqual(['light', 'dark', ...EXTENDED_THEME_IDS]);
  });

  it('除浅色基线外，每套主题都覆盖全部契约 token', () => {
    for (const { id } of THEME_REGISTRY) {
      const map = themeOverrideMapFor(id);
      if (!map) {
        expect(id).toBe('light');
        continue;
      }
      const missing = THEME_CONTRACT_KEYS.filter((key) => !map[key]);
      expect(missing, `主题 ${id} 缺失 token`).toEqual([]);
    }
  });

  it('契约 token 都存在于 CSS 变量注册表', () => {
    const registered = new Set(Object.keys(cssVariables));
    for (const key of THEME_CONTRACT_KEYS) expect(registered.has(key), key).toBe(true);
  });

  it('面板/阴影/材质这些「全元素」令牌不许留空，也不许带 CSS 变量自引用', () => {
    for (const { id } of THEME_REGISTRY) {
      const map = themeOverrideMapFor(id);
      if (!map) continue;
      for (const key of ['--apple-surface-panel', '--apple-shadow-card', '--apple-glass-blur'] as const) {
        const value = map[key];
        expect(value, `${id} 的 ${key}`).toBeTruthy();
        expect(value, `${id} 的 ${key} 引用了未注册变量`).not.toMatch(/--apple-unknown/);
      }
    }
  });

  it('主题 id 守卫只认注册过的值', () => {
    expect(isThemeId('nope')).toBe(false);
    expect(isThemeId(THEME_IDS[0])).toBe(true);
    expect(isThemePreference('auto')).toBe(true);
    expect(isThemePreference('nope')).toBe(false);
    expect(themeMetaFor('nope')).toBeUndefined();
  });

  it('明暗与玻璃能力由注册表元数据决定，而非主题 id 字面量', () => {
    for (const meta of THEME_REGISTRY) {
      expect(isDarkThemeId(meta.id)).toBe(meta.appearance === 'dark');
      // 玻璃是材质轴：只有 supportsGlass 的外观在 material=glass 时才有 Clear 变体
      expect(supportsGlass(meta.id)).toBe(meta.supportsGlass === true);
      expect(supportsGlassClear(meta.id, 'glass')).toBe(meta.supportsGlass === true);
      expect(supportsGlassClear(meta.id, 'solid')).toBe(false);
    }
    expect(THEME_REGISTRY.some((m) => m.appearance === 'dark')).toBe(true);
  });

  it('玻璃叠加层只补材质 token，键都已注册；与 base 合并后覆盖契约', () => {
    const registered = new Set(Object.keys(cssVariables));
    for (const base of ['light', 'dark'] as const) {
      const overlay = GLASS_OVERLAY[base];
      expect(overlay, `${base} 缺玻璃叠加层`).toBeTruthy();
      // 叠加层不许写野键（写错变量名这里就拦下）
      const wild = Object.keys(overlay).filter((k) => !registered.has(k));
      expect(wild, `${base} 玻璃叠加层写了未注册键`).toEqual([]);
      // 合并 base（light 走 :root 默认，dark 走 DARK_THEME）+ 玻璃后，契约键都有值。
      // 玻璃块只覆写材质键，其余靠层叠回落到 base，所以这里按「base ∪ overlay」判覆盖。
      const palette = base === 'dark' ? (themeOverrideMapFor('dark') ?? {}) : cssVariables;
      const merged = { ...palette, ...overlay };
      const missing = THEME_CONTRACT_KEYS.filter((key) => !merged[key]);
      expect(missing, `${base}+glass 合并后缺 token`).toEqual([]);
    }
    // 纸感主题没有玻璃：material=glass 时叠加层为空，解析结果等同 solid
    for (const base of EXTENDED_THEME_IDS) {
      expect(supportsGlass(base)).toBe(false);
      expect(resolveThemeTokens(base, 'glass')).toEqual(resolveThemeTokens(base, 'solid'));
      // 而且共享的玻璃语言（AppleGlass 常量 / .apple-glass-layer）读这三枚令牌时，
      // 解析结果必须是"没有玻璃"：不带模糊、不带镜面高光、棱线透明。
      const solid = resolveThemeTokens(base, 'solid') as Record<string, string | undefined>;
      const fallback = cssVariables as Record<string, string | undefined>;
      const pick = (key: string) => solid[key] ?? fallback[key];
      expect(pick('--apple-glass-blur'), `${base} 不该有玻璃模糊`).toBe('none');
      expect(pick('--apple-glass-specular'), `${base} 不该有镜面高光`).toBe('none');
      expect(pick('--apple-glass-rim-specular'), `${base} 不该有镜面环`).toBe('none');
      expect(pick('--apple-glass-edge'), `${base} 不该有玻璃棱线`).toMatch(/rgba\(0,\s*0,\s*0,\s*0\)/);
    }
  });
});

describe('主题注入', () => {
  beforeEach(() => {
    document.head.querySelectorAll('style[id^="apple-"], style[id="highlight-colors"]').forEach((n) => n.remove());
  });

  it('每套主题都产出自己的 style 块，且带齐契约变量', () => {
    injectCSSVariables();
    // 浅色是 :root 默认块，没有 data-theme 选择器
    expect(document.getElementById('apple-light-theme')?.textContent).toContain('--apple-text-primary:');
    for (const { id } of THEME_REGISTRY) {
      if (id === 'light') continue;
      const node = document.getElementById(`apple-${id}-theme`);
      expect(node, `缺少 apple-${id}-theme`).not.toBeNull();
      const css = node!.textContent ?? '';
      expect(css, `apple-${id}-theme 选择器不对`).toContain(`[data-theme='${id}']`);
      for (const key of THEME_CONTRACT_KEYS) {
        expect(css, `apple-${id}-theme 缺 ${key}`).toContain(`${key}:`);
      }
    }
  });

  it('语法高亮与 Monaco 都按主题分派，且明暗调色板不共用', () => {
    injectHighlightCSSVariables();
    const css = document.getElementById('highlight-colors')?.textContent ?? '';
    expect(css).toContain(':root[data-theme=');
    expect(HighlightColors.dark.keyword).not.toBe(HighlightColors.light.keyword);

    const palettes = new Set(THEME_REGISTRY.map((m) => HIGHLIGHT_PALETTE_BY_THEME[m.id]));
    const darkPalettes = new Set(
      THEME_REGISTRY.filter((m) => m.appearance === 'dark').map((m) => HIGHLIGHT_PALETTE_BY_THEME[m.id])
    );
    expect(darkPalettes.size).toBe(1);
    expect(darkPalettes.has('light')).toBe(false);
    expect(palettes.size).toBeGreaterThan(1);

    const monacoThemes = THEME_REGISTRY.map((m) => monacoThemeForUi(m.id));
    expect(new Set(monacoThemes).size).toBe(THEME_REGISTRY.length);

    // 材质轴：玻璃给浅/深各一套透明底 Monaco 主题，且与实心不同
    expect(monacoThemeForUi('light', 'glass')).toBe(MONACO_APPLE_THEMES.glass);
    expect(monacoThemeForUi('dark', 'glass')).toBe(MONACO_APPLE_THEMES.glassDark);
    expect(monacoThemeForUi('light', 'glass')).not.toBe(monacoThemeForUi('light', 'solid'));
    expect(monacoThemeForUi('dark', 'glass')).not.toBe(monacoThemeForUi('dark', 'solid'));
    // 纸感主题不支持玻璃：material=glass 也回落到该外观自己的 Monaco 主题
    for (const base of EXTENDED_THEME_IDS) {
      expect(monacoThemeForUi(base, 'glass')).toBe(monacoThemeForUi(base, 'solid'));
    }
  });

  it('支持玻璃的外观额外产出 glass 材质块，选择器带 data-material', () => {
    injectCSSVariables();
    const MATERIAL_KEYS = ['--apple-surface-panel:', '--apple-glass-blur:', '--apple-shadow-card:'];
    for (const { id } of THEME_REGISTRY) {
      if (!supportsGlass(id)) {
        expect(document.getElementById(`apple-${id}-glass-theme`), `${id} 不该有玻璃块`).toBeNull();
        continue;
      }
      const node = document.getElementById(`apple-${id}-glass-theme`);
      expect(node, `缺少 apple-${id}-glass-theme`).not.toBeNull();
      const css = node!.textContent ?? '';
      expect(css, `玻璃块选择器不对`).toContain(`[data-theme='${id}'][data-material='glass']`);
      // 玻璃块只覆写材质键（其余靠层叠回落 base），但材质签名键必须在场
      for (const key of MATERIAL_KEYS) {
        expect(css, `apple-${id}-glass-theme 缺材质键 ${key}`).toContain(key);
      }
    }
  });

  it('缩略色块从主题自身派生，且不出现空色', () => {
    for (const { id } of THEME_REGISTRY) {
      for (const material of supportsGlass(id) ? (['solid', 'glass'] as const) : (['solid'] as const)) {
        const swatch = themeSwatch(id, cssVariables, material);
        for (const value of Object.values(swatch)) {
          expect(value, `${id}/${material} 的缩略字段为空`).toBeTruthy();
        }
      }
    }
  });
});
