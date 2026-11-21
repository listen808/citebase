import { describe, it, expect } from 'vitest';
import {
  GLASS_FLOAT_ALPHA_CEIL,
  GLASS_FLOAT_ALPHA_FLOOR,
  GLASS_TEXT_FLOOR,
  compositeOver,
  contrastRatio,
  glassContrast,
  minAlphaForFloor,
  parseCssColor,
  srgbLuminance,
  type Rgb,
} from './glass-contrast';

const WHITE_PLATE: Rgb = [250, 252, 255];
const DARK_TEXT: Rgb = [0x58, 0x58, 0x5e];
const BLACK: Rgb = [0, 0, 0];
const WHITE: Rgb = [255, 255, 255];

describe('glass-contrast 基础数学', () => {
  it('纯黑亮度 0、纯白亮度 1、中灰按规范线性化', () => {
    expect(srgbLuminance(BLACK)).toBe(0);
    expect(srgbLuminance(WHITE)).toBeCloseTo(1, 6);
    expect(srgbLuminance([128, 128, 128])).toBeCloseTo(0.2159, 3);
  });

  it('alpha=1 时完全取前景，alpha=0 时完全取背景', () => {
    expect(compositeOver([10, 20, 30, 1], [200, 200, 200])).toEqual([10, 20, 30]);
    expect(compositeOver([10, 20, 30, 0], [200, 200, 200])).toEqual([200, 200, 200]);
  });

  it('黑白对比 21，同色对比 1', () => {
    expect(contrastRatio(BLACK, WHITE)).toBeCloseTo(21, 1);
    expect(contrastRatio(DARK_TEXT, DARK_TEXT)).toBeCloseTo(1, 6);
  });

  it('parseCssColor 认三种写法，解不出的给 null 而不是猜', () => {
    expect(parseCssColor('rgba(250, 252, 255, 0.78)')).toEqual([250, 252, 255, 0.78]);
    expect(parseCssColor('rgb(250 252 255)')).toEqual([250, 252, 255, 1]);
    expect(parseCssColor('#fafcff')).toEqual([250, 252, 255, 1]);
    expect(parseCssColor('var(--apple-surface-float)')).toBeNull();
    expect(parseCssColor('')).toBeNull();
  });
});

describe('minAlphaForFloor：浮层自适应的唯一解算器', () => {
  it('背景越暗，需要的 alpha 越大（单调，且严格在区间内）', () => {
    const alphas = [0, 0.1, 0.25, 0.5, 0.75, 1].map((l) =>
      minAlphaForFloor(WHITE_PLATE, DARK_TEXT, [l * 255, l * 255, l * 255]));
    for (let i = 1; i < alphas.length; i++) {
      expect(alphas[i] ?? 0).toBeLessThanOrEqual(alphas[i - 1] ?? 1);
    }
    expect(alphas[0]).toBeGreaterThan(0.8);
    expect(alphas[alphas.length - 1]).toBe(GLASS_FLOAT_ALPHA_FLOOR);
  });

  it('解出来的每一个 alpha 都真的过地板 —— 这条不成立，运行时那一层就是假的安全', () => {
    for (let l = 0; l <= 1.0001; l += 0.05) {
      const bg: Rgb = [l * 255, l * 255, l * 255];
      const a = minAlphaForFloor(WHITE_PLATE, DARK_TEXT, bg);
      const got = glassContrast(WHITE_PLATE, a, bg, DARK_TEXT);
      // 上限那一档是"解不出来"的出口，不是地板承诺
      if (a < GLASS_FLOAT_ALPHA_CEIL) expect(got).toBeGreaterThanOrEqual(GLASS_TEXT_FLOOR - 0.01);
    }
  });

  it('浅字压深底同样解得出来（方向反过来时不能退回同一个数）', () => {
    const darkPlate: Rgb = [40, 40, 42];
    const lightText: Rgb = [0xb8, 0xb8, 0xbe];
    const onWhite = minAlphaForFloor(darkPlate, lightText, WHITE);
    const onMid = minAlphaForFloor(darkPlate, lightText, [181, 181, 181]);
    expect(onWhite).toBeGreaterThan(onMid);
    expect(glassContrast(darkPlate, onMid, [181, 181, 181], lightText)).toBeGreaterThanOrEqual(GLASS_TEXT_FLOOR - 0.01);
  });
});
