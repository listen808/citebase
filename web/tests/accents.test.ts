import { describe, it, expect, beforeEach } from 'vitest';
import {
  ACCENT_AUTO,
  ACCENT_IDS,
  ACCENT_REGISTRY,
  DEFAULT_ACCENT,
  accentColorFor,
  applyAccent,
  isAccentId,
} from '@/styles/theme/accents';
import { contrastInkOn } from '@/utils/core/apple-tokens';

/**
 * 强调色档位（批3）的行为判据。
 *
 * **色值本身的合格性不在这里测** —— 那是 `check:contrast` 的职责（它把 accent × 外观 × 材质
 * 全笛卡尔跑一遍，且用的是与运行时同一份 `contrastInkOn`）。这里只测两件门禁测不到的事：
 * 清单自洽，以及覆写/撤销覆写真的落到 DOM 上。
 * "少一枚 hex"这类问题在界面上的表现是主题像没切过去，而静态代码看起来完全正常。
 */

const root = () => document.documentElement;

beforeEach(() => {
  root().removeAttribute('data-accent');
  root().style.removeProperty('--apple-blue');
  root().style.removeProperty('--apple-on-blue');
});

describe('清单', () => {
  it('auto 在清单里，而且是第一个（默认档 = 不覆盖）', () => {
    expect(ACCENT_IDS[0]).toBe(ACCENT_AUTO);
    expect(isAccentId('auto')).toBe(true);
    expect(DEFAULT_ACCENT).toBe(ACCENT_AUTO);
  });

  it('每一档都有 id / 名字键 / 浅深两枚 hex，且 id 不重复', () => {
    const ids = new Set<string>();
    for (const a of ACCENT_REGISTRY) {
      expect(a.id).toMatch(/^[a-z]+$/);
      expect(a.labelKey.startsWith('chrome.')).toBe(true);
      expect(a.light).toMatch(/^#[0-9a-f]{6}$/);
      expect(a.dark).toMatch(/^#[0-9a-f]{6}$/);
      expect(ids.has(a.id)).toBe(false);
      ids.add(a.id);
    }
    // 清单与注册表必须一一对应：多一档少一档都是"另一台设备上的选择被判非法值"那一类
    expect(ACCENT_IDS.slice(1).sort()).toEqual([...ids].sort());
  });

  it('认不得的取值不被当成合法档（服务端会拒绝，前端不许照写）', () => {
    expect(isAccentId('chartreuse')).toBe(false);
    expect(isAccentId(undefined)).toBe(false);
  });
});

describe('accentColorFor', () => {
  it('auto 返回 null，而不是某个具体颜色', () => {
    // 返回 null 与返回"外观自己的 hex"不是一回事：后者会把 --apple-blue 钉死成静态值，
    // 于是从纸感（橙）切回浅色时颜色不跟着外观走了。
    expect(accentColorFor(ACCENT_AUTO, 'light')).toBeNull();
    expect(accentColorFor(ACCENT_AUTO, 'dark')).toBeNull();
  });

  it('浅/深两列分别取到', () => {
    const teal = ACCENT_REGISTRY.find((a) => a.id === 'teal')!;
    expect(accentColorFor('teal', 'light')).toBe(teal.light);
    expect(accentColorFor('teal', 'dark')).toBe(teal.dark);
  });

  it('清单里有但注册表里没有的 id 返回 null（不许回退成第一档）', () => {
    expect(accentColorFor('auto-but-wrong' as never, 'light')).toBeNull();
  });
});

describe('applyAccent 写 DOM', () => {
  it('选定档位：覆写 --apple-blue，并同时重算 --apple-on-blue', () => {
    const purple = ACCENT_REGISTRY.find((a) => a.id === 'purple')!;
    applyAccent('purple', 'dark');
    expect(root().getAttribute('data-accent')).toBe('purple');
    expect(root().style.getPropertyValue('--apple-blue')).toBe(purple.dark);
    // 这一行是本批最容易写错的地方：on-blue 是 JS 按亮度算的，不跟着 var() 走
    expect(root().style.getPropertyValue('--apple-on-blue')).toBe(contrastInkOn(purple.dark));
  });

  it('切回 auto：两枚覆写都被撤掉，颜色交回外观自己的调色板', () => {
    applyAccent('green', 'light');
    applyAccent(ACCENT_AUTO, 'light');
    expect(root().style.getPropertyValue('--apple-blue')).toBe('');
    expect(root().style.getPropertyValue('--apple-on-blue')).toBe('');
    // data-accent 恒写（含 auto）：缺席只该意味着"JS 没跑"
    expect(root().getAttribute('data-accent')).toBe('auto');
  });

  it('外观从浅切深时重新取列（否则深色外观带着浅色那枚 hex，白字压不住）', () => {
    applyAccent('blue', 'light');
    const lightHex = root().style.getPropertyValue('--apple-blue');
    applyAccent('blue', 'dark');
    expect(root().style.getPropertyValue('--apple-blue')).not.toBe(lightHex);
  });
});
