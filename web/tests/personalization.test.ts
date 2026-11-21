import { describe, it, expect, beforeEach, afterEach, vi } from 'vitest';
import {
  CONTRAST_MODES,
  MOTION_MODES,
  MOTION_SCALE,
  TRANSPARENCY_MODES,
  applyPersonalizationAttrs,
  currentMotionMode,
  motionScaleValue,
  resolveContrastMode,
  resolveMotionMode,
  resolveTransparencyMode,
  scaledMs,
  scaledResponse,
} from '@/styles/theme/personalization';

/**
 * 个性化档位（动效强度 / 对比度 / 透明度）的判据。
 *
 * 这里测的两件事是静态门禁量不到的：
 *  ① **默认路径必须与今天完全一致** —— auto + 系统没开任何开关 ⇒ 倍数正好 1。
 *     这条一旦漂，症状是"没人设置过的新用户动效变快/变慢"，而没有任何人会去开设置页核对。
 *  ② 档位 → DOM 的写法必须真的落到 :root 上（CSS 那两层选择器是按属性判的）。
 * `check:theme` 的 T15 管的是"清单少登记一半"，这里管"解算与写盘的行为"。
 */

/** 只负责"把系统信号伪装成某个样子"；还原交给 afterEach 的 unstubAllGlobals */
function stubSystemSignal(matches: Partial<Record<string, boolean>>) {
  vi.stubGlobal('matchMedia', (query: string) => ({
    matches: !!matches[query],
    media: query,
    addEventListener: () => {},
    removeEventListener: () => {},
  }));
}

const ALL_OFF = {
  '(prefers-reduced-motion: reduce)': false,
  '(prefers-contrast: more)': false,
  '(prefers-reduced-transparency: reduce)': false,
};

beforeEach(() => {
  document.documentElement.removeAttribute('data-motion');
  document.documentElement.removeAttribute('data-a11y-contrast');
  document.documentElement.removeAttribute('data-a11y-transparency');
  document.documentElement.style.removeProperty('--apple-motion-scale');
});
afterEach(() => vi.unstubAllGlobals());

describe('档位清单', () => {
  it('三条档位的默认值都是 auto（= 这台设备没表过态，由系统信号决定）', () => {
    for (const list of [MOTION_MODES, CONTRAST_MODES, TRANSPARENCY_MODES]) {
      expect(list).toContain('auto');
    }
  });

  it('每个非 auto 的动效档都有倍数 —— 少一个就是把 undefined 写进 CSS', () => {
    for (const mode of MOTION_MODES.filter((m) => m !== 'auto')) {
      expect(typeof MOTION_SCALE[mode]).toBe('number');
      expect(MOTION_SCALE[mode]).toBeGreaterThan(0);
    }
  });
});

describe('resolve*', () => {
  it('auto + 系统全关 → 设计档（full / standard / full）', () => {
    stubSystemSignal(ALL_OFF);
    expect(resolveMotionMode('auto')).toBe('full');
    expect(resolveContrastMode('auto')).toBe('standard');
    expect(resolveTransparencyMode('auto')).toBe('full');
  });

  it('auto + 系统开关逐个打开 → 各自那一档（三条是独立信号，不能合并判断）', () => {
    stubSystemSignal({
      ...ALL_OFF,
      '(prefers-contrast: more)': true,
      '(prefers-reduced-transparency: reduce)': true,
    });
    expect(resolveMotionMode('auto')).toBe('full');
    expect(resolveContrastMode('auto')).toBe('more');
    expect(resolveTransparencyMode('auto')).toBe('reduce');
  });

  it('应用内显式选择盖过系统信号（这是"可选性双向"那条决定的行为，改动需要重新点头）', () => {
    stubSystemSignal({ ...ALL_OFF, '(prefers-reduced-motion: reduce)': true });
    expect(resolveMotionMode('auto')).toBe('reduced');
    expect(resolveMotionMode('full')).toBe('full');
    expect(resolveMotionMode('soft')).toBe('soft');
  });
});

describe('写 DOM', () => {
  it('三档恒写，倍数以行内覆写令牌本身的方式给', () => {
    applyPersonalizationAttrs({ motion: 'soft', contrast: 'more', transparency: 'reduce', persona: 'signature' });
    const root = document.documentElement;
    expect(root.getAttribute('data-motion')).toBe('soft');
    expect(root.getAttribute('data-a11y-contrast')).toBe('more');
    expect(root.getAttribute('data-a11y-transparency')).toBe('reduce');
    expect(root.getAttribute('data-persona')).toBe('signature');
    expect(root.style.getPropertyValue('--apple-motion-scale')).toBe(String(MOTION_SCALE.soft));
  });

  it('默认路径（full/standard/full）写出来的倍数正好是 1 —— 首屏不许被个性化改快或改慢', () => {
    applyPersonalizationAttrs({ motion: 'full', contrast: 'standard', transparency: 'full', persona: 'plain' });
    expect(document.documentElement.style.getPropertyValue('--apple-motion-scale')).toBe('1');
  });

  it('选了 standard 时属性仍然写上：缺席只能意味着"JS 没跑"，否则 CSS 护栏分不清这两种', () => {
    applyPersonalizationAttrs({ motion: 'full', contrast: 'standard', transparency: 'full', persona: 'plain' });
    expect(document.documentElement.getAttribute('data-a11y-contrast')).toBe('standard');
  });

  it('persona 恒写，且默认档就是 plain（编排整体被 :root[data-persona] 套住，T17 判那一侧）', () => {
    applyPersonalizationAttrs({ motion: 'full', contrast: 'standard', transparency: 'full', persona: 'plain' });
    expect(document.documentElement.getAttribute('data-persona')).toBe('plain');
  });
});

describe('取值与缩放', () => {
  it('data-motion 缺席时回落系统信号（与 CSS 侧 MOTION_GUARD 同一口径）', () => {
    stubSystemSignal({ ...ALL_OFF, '(prefers-reduced-motion: reduce)': true });
    expect(currentMotionMode()).toBe('reduced');
    stubSystemSignal(ALL_OFF);
    expect(currentMotionMode()).toBe('full');
  });

  it('DOM 上的倍数优先于查表（行内覆写才是实际生效的那个数）', () => {
    applyPersonalizationAttrs({ motion: 'reduced', contrast: 'standard', transparency: 'full', persona: 'plain' });
    expect(motionScaleValue()).toBeCloseTo(MOTION_SCALE.reduced, 6);
    expect(scaledMs(1000)).toBe(Math.round(1000 * MOTION_SCALE.reduced));
  });

  it('soft 档：时长变短为 0.6 倍，而弹簧 response 变长（除号写反的症状是"选柔和档反而更冲"）', () => {
    applyPersonalizationAttrs({ motion: 'soft', contrast: 'standard', transparency: 'full', persona: 'plain' });
    expect(scaledMs(300)).toBe(180);
    expect(scaledResponse(0.4)).toBeCloseTo(0.4 / MOTION_SCALE.soft, 6);
    expect(scaledResponse(0.4)).toBeGreaterThan(0.4);
  });

  it('读不到倍数时不猜：按当前生效档查表，绝不返回 NaN', () => {
    document.documentElement.style.removeProperty('--apple-motion-scale');
    applyPersonalizationAttrs({ motion: 'full', contrast: 'standard', transparency: 'full', persona: 'plain' });
    document.documentElement.style.removeProperty('--apple-motion-scale');
    expect(Number.isFinite(motionScaleValue())).toBe(true);
    expect(Number.isFinite(scaledMs(250))).toBe(true);
  });
});
