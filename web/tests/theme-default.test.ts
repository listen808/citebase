import { describe, it, expect, beforeEach } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';
import { useUIStore } from '@/stores/app/ui';
import { StorageKeys, storageClearAll, storageSet } from '@/utils/core/storage';

/**
 * 钉住「全新访客第一眼看到什么」这条产品默认值：浅色 + 液态玻璃 + 透明增强。
 *
 * 为什么要有这台门禁：三档默认值散在 `ui.ts` 的 ref 初始化和 `initTheme()` 的兜底分支里，
 * 后者会覆盖前者 —— 只改一处（比如把 ref 改成 glass 却忘了 pref 的 'auto' 兜底）
 * 界面照样好看，但换一台机器/系统深色时第一眼就不是那个东西了，而这种漂移没有任何静态检查看得见。
 *
 * 另一半判据同样重要：**用户显式选过的值必须继续赢过默认值**。
 * 默认值只是"没写过的时候给什么"，一旦它开始盖掉用户的选择，就是回归。
 */
const k = StorageKeys;

function freshStore() {
  setActivePinia(createPinia());
  return useUIStore();
}

describe('主题默认配置', () => {
  beforeEach(() => {
    // storageClearAll 而不是 localStorage.clear()：
    // storage.ts 在 Safari 隐私模式/配额满时会退到模块内的内存 Map，
    // 只清 localStorage 会让上一条用例 storageSet 的值漏进下一条用例里。
    storageClearAll();
    const root = document.documentElement;
    root.removeAttribute('data-theme');
    root.removeAttribute('data-material');
    root.removeAttribute('data-glass');
  });

  it('空存储下 = 浅色 + glass + 透明增强', () => {
    const ui = freshStore();
    ui.initTheme();

    expect(ui.themePreference).toBe('light');
    expect(ui.theme).toBe('light');
    expect(ui.material).toBe('glass');
    expect(ui.glassClear).toBe(true);

    const root = document.documentElement;
    expect(root.getAttribute('data-theme')).toBe('light');
    expect(root.getAttribute('data-material')).toBe('glass');
    // 透明增强要真的落到 DOM 上，否则只是个没接线的状态值
    expect(root.getAttribute('data-glass')).toBe('clear');
  });

  it('系统深色不会把默认改成深色（默认是钉住的浅色，不是跟随系统）', () => {
    const mq = window.matchMedia;
    window.matchMedia = ((query: string) => ({
      matches: query.includes('dark'),
      media: query,
      addEventListener: () => {},
      removeEventListener: () => {},
    })) as unknown as typeof window.matchMedia;
    try {
      const ui = freshStore();
      ui.initTheme();
      expect(ui.theme).toBe('light');
    } finally {
      window.matchMedia = mq;
    }
  });

  it('用户显式选过的三档都赢过默认值', () => {
    storageSet(k.UI_THEME_PREF, 'dark');
    storageSet(k.UI_MATERIAL, 'solid');
    storageSet(k.UI_GLASS_CLEAR, 'false');

    const ui = freshStore();
    ui.initTheme();

    expect(ui.themePreference).toBe('dark');
    expect(ui.material).toBe('solid');
    expect(ui.glassClear).toBe(false);
    expect(document.documentElement.getAttribute('data-material')).toBe('solid');
    expect(document.documentElement.getAttribute('data-glass')).toBeNull();
  });

  it("显式选过『跟随系统』的用户继续跟随系统", () => {
    storageSet(k.UI_THEME_PREF, 'auto');
    const ui = freshStore();
    ui.initTheme();
    expect(ui.themePreference).toBe('auto');
  });

  it('历史独立主题 liquid-glass 仍解成 浅色 + glass（升级不丢主题）', () => {
    storageSet(k.UI_THEME_PREF, 'liquid-glass');
    const ui = freshStore();
    ui.initTheme();
    expect(ui.theme).toBe('light');
    expect(ui.material).toBe('glass');
  });

  it('纸面外观下 glass 塌回 solid，透明增强不上 DOM', () => {
    storageSet(k.UI_THEME_PREF, 'warm-paper');
    const ui = freshStore();
    ui.initTheme();
    // 偏好仍然是 glass（纸面塌回只发生在应用时），用户切回浅色玻璃不用重选
    expect(ui.material).toBe('glass');
    expect(ui.theme).toBe('warm-paper');
    const root = document.documentElement;
    expect(root.getAttribute('data-material')).toBe('solid');
    expect(root.getAttribute('data-glass')).toBeNull();
  });
});
