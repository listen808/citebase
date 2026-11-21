import { describe, it, expect, afterEach, vi } from 'vitest';
import { storageGet, storageSet, storageRemove, storageGetJSON, storageSetJSON, storageClearAll, migrateLegacyRawKeys } from '@/utils/core/storage';

/** Safari 隐私模式 / 配额写满时，localStorage 的每个方法都会抛 */
const throwingStorage = {
  getItem() { throw new Error('SecurityError'); },
  setItem() { throw new Error('QuotaExceededError'); },
  removeItem() { throw new Error('SecurityError'); },
  key() { throw new Error('SecurityError'); },
  length: 0,
};

function fakeStorage(entries: Record<string, string> = {}) {
  const store = new Map(Object.entries(entries));
  return {
    store,
    getItem: (k: string) => (store.has(k) ? store.get(k)! : null),
    setItem: (k: string, v: string) => { store.set(k, v); },
    removeItem: (k: string) => { store.delete(k); },
    key: (i: number) => [...store.keys()][i] ?? null,
    get length() { return store.size; },
  };
}

afterEach(() => {
  vi.unstubAllGlobals();
});

describe('storage 在 localStorage 不可用时降级', () => {
  it('写入不抛错，且同会话内可读回', () => {
    vi.stubGlobal('localStorage', throwingStorage);
    expect(() => storageSet('test:fallback', 'abc')).not.toThrow();
    expect(storageGet('test:fallback')).toBe('abc');
  });

  it('移除后读不到值', () => {
    vi.stubGlobal('localStorage', throwingStorage);
    storageSet('test:removed', 'x');
    storageRemove('test:removed');
    expect(storageGet('test:removed')).toBe(null);
  });

  it('JSON 读写同样不抛错', () => {
    vi.stubGlobal('localStorage', throwingStorage);
    expect(() => storageSetJSON('test:json', { a: 1 })).not.toThrow();
    expect(storageGetJSON('test:json', null)).toEqual({ a: 1 });
  });
});

describe('storage 正常路径', () => {
  it('优先读 localStorage 而不是内存兜底', () => {
    const fs = fakeStorage({ 'citebase:ui:theme': 'dark' });
    vi.stubGlobal('localStorage', fs);
    expect(storageGet('ui:theme')).toBe('dark');
  });

  it('写入走 citebase: 前缀', () => {
    const fs = fakeStorage();
    vi.stubGlobal('localStorage', fs);
    storageSet('recent-docs:list', '[]');
    expect(fs.store.get('citebase:recent-docs:list')).toBe('[]');
  });

  it('未写入的键返回 null，不做任何隐式回退', () => {
    const fs = fakeStorage({ 'other-app:ui:theme': 'dark' });
    vi.stubGlobal('localStorage', fs);
    expect(storageGet('ui:theme')).toBeNull();
  });

  it('storageRemove 只删除当前前缀下的键', () => {
    const fs = fakeStorage({ 'citebase:ui:theme': 'dark' });
    vi.stubGlobal('localStorage', fs);
    storageRemove('ui:theme');
    expect(fs.store.has('citebase:ui:theme')).toBe(false);
  });

  it('storageClearAll 清除全部 citebase: 前缀键', () => {
    const fs = fakeStorage({
      'citebase:ui:theme': 'dark',
      'citebase:auth:token': 't',
      'unrelated:key': 'keep',
    });
    vi.stubGlobal('localStorage', fs);
    storageClearAll();
    expect(fs.store.has('citebase:ui:theme')).toBe(false);
    expect(fs.store.has('citebase:auth:token')).toBe(false);
    expect(fs.store.has('unrelated:key')).toBe(true);
  });
});

/**
 * 2026-10-07 改名把前缀从 `aiwiki:` 换成 `citebase:`，当时没做回落 —— 老用户的主题、材质、
 * 透明增强、搜索历史、未保存草稿在升级当天全部读不到。下面钉的是回落的**边界**而不只是"能读到"：
 * 覆盖优先级（升级后又设过一次不能被旧值回滚）、采纳即删、清库连旧前缀一起清，
 * 以及"正文与时间戳两类草稿键都要搬" —— 漏一类，时间戳留在旧名下，
 * 表现是草稿明明还在、却被判定成已过期。
 */
describe('改名前 aiwiki: 键的回落与迁移', () => {
  it('新键缺席时采纳旧前缀的值', () => {
    const fs = fakeStorage({ 'aiwiki:ui:material': 'glass' });
    vi.stubGlobal('localStorage', fs);
    expect(storageGet('ui:material')).toBe('glass');
  });

  it('新键已有值时不被旧值覆盖', () => {
    const fs = fakeStorage({ 'citebase:ui:glass-clear': 'false', 'aiwiki:ui:glass-clear': 'true' });
    vi.stubGlobal('localStorage', fs);
    expect(storageGet('ui:glass-clear')).toBe('false');
  });

  it('采纳后删掉旧键，不留两份', () => {
    const fs = fakeStorage({ 'aiwiki:search:history': '["a"]' });
    vi.stubGlobal('localStorage', fs);
    expect(storageGet('search:history')).toBe('["a"]');
    expect(fs.store.has('aiwiki:search:history')).toBe(false);
    expect(fs.store.get('citebase:search:history')).toBe('["a"]');
  });

  it('storageClearAll 连旧前缀一起清 —— 否则刷新后旧值会被重新采纳，"清除"等于没生效', () => {
    const fs = fakeStorage({ 'aiwiki:reader:wide-mode': 'true' });
    vi.stubGlobal('localStorage', fs);
    storageClearAll();
    expect(fs.store.has('aiwiki:reader:wide-mode')).toBe(false);
    expect(storageGet('reader:wide-mode')).toBeNull();
  });

  it('localStorage 抛错时回落不炸，只是读不到', () => {
    vi.stubGlobal('localStorage', throwingStorage);
    expect(() => storageGet('ui:material')).not.toThrow();
    expect(storageGet('ui:material')).toBeNull();
  });

  it('migrateLegacyRawKeys 搬草稿（正文与时间戳两类）与面板宽度', () => {
    const fs = fakeStorage({
      'aiwiki_doc_draft_7_9': '正文',
      'aiwiki_doc_draft_time_7_9': '1700000000000',
      'aiwiki-sidebar-width': '320',
      'something-unrelated': 'x',
    });
    vi.stubGlobal('localStorage', fs);
    expect(migrateLegacyRawKeys()).toBe(3);
    expect(fs.store.get('citebase_doc_draft_7_9')).toBe('正文');
    expect(fs.store.get('citebase_doc_draft_time_7_9')).toBe('1700000000000');
    expect(fs.store.get('citebase-sidebar-width')).toBe('320');
    expect(fs.store.has('aiwiki_doc_draft_7_9')).toBe(false);
    expect(fs.store.has('aiwiki_doc_draft_time_7_9')).toBe(false);
    expect(fs.store.get('something-unrelated')).toBe('x');
  });

  it('新名已有值时不搬（不覆盖用户升级后写的草稿）', () => {
    const fs = fakeStorage({ 'aiwiki_doc_draft_7_9': '旧的', 'citebase_doc_draft_7_9': '新的' });
    vi.stubGlobal('localStorage', fs);
    expect(migrateLegacyRawKeys()).toBe(0);
    expect(fs.store.get('citebase_doc_draft_7_9')).toBe('新的');
  });

  it('再跑一次是零成本（幂等）', () => {
    const fs = fakeStorage({ 'aiwiki_doc_draft_1_2': 'a' });
    vi.stubGlobal('localStorage', fs);
    expect(migrateLegacyRawKeys()).toBe(1);
    expect(migrateLegacyRawKeys()).toBe(0);
  });
});
