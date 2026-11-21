import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import {
  decideSync,
  parseServerPreferences,
  readLocalSyncStamp,
  toPayload,
  writeLocalSyncStamp,
} from '@/utils/core/preference-sync';
import { storageRemove, StorageKeys } from '@/utils/core/storage';

/**
 * 跨设备同步的判据。这里测的全是"会不会静默吃掉用户的选择"那类问题，
 * 而不是字段有没有拼对 —— 后者 type-check 就管了。
 */
describe('parseServerPreferences', () => {
  it('正常形状照单收下', () => {
    const p = parseServerPreferences({ theme: 'dark', material: 'glass', glass_clear: true, updated_at: 10 });
    expect(p).toEqual({
      theme: 'dark',
      material: 'glass',
      glass_clear: true,
      // 三条行为档在老账号那一行里根本没有，缺席时补本机值（这里没传 local → 补 auto）
      motion: 'auto',
      contrast: 'auto',
      transparency: 'auto',
      persona: 'plain',
      accent: 'auto',
      updated_at: 10,
    });
  });

  it('行为三档带值时收下，非法时补本机值而**不是整份判 null**', () => {
    const full = parseServerPreferences({
      theme: 'dark', material: 'solid', glass_clear: false,
      motion: 'soft', contrast: 'more', transparency: 'reduce', updated_at: 7,
    });
    expect(full).toMatchObject({ motion: 'soft', contrast: 'more', transparency: 'reduce' });
    // 2026-10-16 之前落库的行没有这三格；把它们当非法 = 升级后连主题都不同步
    const withLocal = parseServerPreferences(
      { theme: 'dark', material: 'solid', glass_clear: false, motion: 'neon', updated_at: 7 },
      { motion: 'full', contrast: 'more', transparency: 'auto', accent: 'teal', persona: 'signature' },
    );
    expect(withLocal).toMatchObject({ theme: 'dark', motion: 'full', contrast: 'more', transparency: 'auto', persona: 'signature' });
  });

  // 前后端清单万一分叉（有一档外观只在一边登记），回退默认值会**静默改掉用户的界面**；
  // 当"账号上没配过"处理则只是不同步，是可逆的那种失败。
  it('不认识的外观当账号上没配过，不回退默认值', () => {
    expect(parseServerPreferences({ theme: 'neon', material: 'solid', glass_clear: false, updated_at: 3 })).toBeNull();
  });

  it('缺字段、类型不对、null 一律当没配过', () => {
    expect(parseServerPreferences(null)).toBeNull();
    expect(parseServerPreferences(undefined)).toBeNull();
    expect(parseServerPreferences({ theme: 'light' })).toBeNull();
    expect(parseServerPreferences({ theme: 'light', material: 'glass', glass_clear: 'yes', updated_at: 1 })).toBeNull();
    expect(parseServerPreferences('light')).toBeNull();
  });

  it('updated_at 缺失按 0（最旧）而不是今天 —— 否则这条记录永远赢', () => {
    const p = parseServerPreferences({ theme: 'light', material: 'solid', glass_clear: false, updated_at: undefined });
    expect(p?.updated_at).toBe(0);
  });
});

describe('decideSync', () => {
  const server = parseServerPreferences({ theme: 'dark', material: 'solid', glass_clear: false, updated_at: 100 })!;

  it('账号没配过 + 本地有选择 → 把本地推上去（seed）', () => {
    expect(decideSync(null, 0, true)).toBe('seed');
  });

  it('账号没配过 + 本地也没写过 → 什么都不做（不给新用户写一份"偏好"）', () => {
    expect(decideSync(null, 0, false)).toBe('none');
  });

  it('服务器更新 → 采纳', () => {
    expect(decideSync(server, 50, true)).toBe('adopt');
  });

  it('本地更新 → 推送', () => {
    expect(decideSync(server, 200, true)).toBe('push');
  });

  it('相等 → 什么都不做（回声就是在这里产生的）', () => {
    expect(decideSync(server, 100, true)).toBe('keep');
  });
});

describe('toPayload', () => {
  const axes = { motion: 'auto', contrast: 'auto', transparency: 'auto', accent: 'auto', persona: 'plain' } as const;

  it('solid 下的 clear 被压成 false，与服务端 NormalizePreferences 同一条规则', () => {
    expect(toPayload({ theme: 'light', material: 'solid', glassClear: true, ...axes })).toEqual({
      theme: 'light',
      material: 'solid',
      glass_clear: false,
      motion: 'auto',
      contrast: 'auto',
      transparency: 'auto',
      persona: 'plain',
      accent: 'auto',
    });
  });

  it('不带 updated_at：那个时钟归服务端', () => {
    expect(Object.keys(toPayload({ theme: 'auto', material: 'glass', glassClear: true, ...axes })).sort()).toEqual(
      ['accent', 'contrast', 'glass_clear', 'material', 'motion', 'persona', 'theme', 'transparency'],
    );
  });

  it('参数是对象而不是六个位置参数 —— 顺序写错在类型上合法，只有对象形状能拦', () => {
    // @ts-expect-error 故意少给一档：位置参数版本这种错误根本传得进去
    expect(typeof toPayload({ theme: 'light', material: 'solid', glassClear: false, motion: 'soft' })).toBe('object');
  });
});

describe('本地同步时刻', () => {
  const store = new Map<string, string>();
  beforeEach(() => {
    store.clear();
    // storage 层在 localStorage 抛错/缺席时会退到模块级内存 Map，那份内存跨测试留着 ——
    // 不清它的话"没写过读成 0"这条会读到上一个用例写的 1234（实测就是这样红的）。
    storageRemove(StorageKeys.UI_PREF_SYNCED_AT);
    vi.stubGlobal('localStorage', {
      getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
      setItem: (k: string, v: string) => store.set(k, v),
      removeItem: (k: string) => store.delete(k),
      key: (i: number) => [...store.keys()][i] ?? null,
      get length() { return store.size; },
    });
  });
  afterEach(() => vi.unstubAllGlobals());

  it('写下去能读回来', () => {
    writeLocalSyncStamp(1234);
    expect(readLocalSyncStamp()).toBe(1234);
  });

  it('没写过读成 0，不是 NaN —— NaN 参与比较会让所有判据都走"什么都不做"那一支', () => {
    expect(readLocalSyncStamp()).toBe(0);
  });

  it('脏值也读成 0', () => {
    store.set('citebase:ui:pref-synced-at', 'abc');
    expect(readLocalSyncStamp()).toBe(0);
  });
});
