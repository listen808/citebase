import { describe, expect, it } from 'vitest';
import i18n from '@/i18n';
import { formatDuration, formatTimeAgo } from './format-time';

// 时长与"多久前"曾在调用点各拼一份中文（`${days}天 ${hours}小时`、`${diff/3600}小时前`），
// 于是界面切成英文、这些格子却还是中文 —— 静态 CJK 门禁只能逐个源码扫，扫不到"运行时拼出来的串"
// 这一类，除非它先落在源码里。收进 time.* 之后，这里钉的是"每种语言各说各的话"这条判据。
const withLocale = <T>(locale: 'zh-CN' | 'en-US', run: () => T): T => {
  const before = i18n.global.locale.value;
  i18n.global.locale.value = locale;
  try {
    return run();
  } finally {
    i18n.global.locale.value = before;
  }
};

describe('formatDuration', () => {
  it('中文：按天/小时/分钟分档', () => {
    withLocale('zh-CN', () => {
      expect(formatDuration(9 * 86400 + 8 * 3600)).toBe('9天 8小时');
      expect(formatDuration(2 * 3600 + 5 * 60)).toBe('2小时 5分');
      expect(formatDuration(7 * 60)).toBe('7分钟');
    });
  });

  it('英文：不夹中文（这几句以前是写死的"9天 8小时"）', () => {
    withLocale('en-US', () => {
      expect(formatDuration(9 * 86400 + 8 * 3600)).toBe('9d 8h');
      expect(formatDuration(2 * 3600 + 5 * 60)).toBe('2h 5m');
      expect(formatDuration(7 * 60)).toBe('7m');
      for (const text of [formatDuration(0), formatDuration(-1), formatDuration(NaN)]) {
        expect(text).not.toMatch(/[\u4e00-\u9fff]/);
      }
    });
  });

  it('无效值仍是占位符，不是 0天 0小时', () => {
    withLocale('zh-CN', () => {
      expect(formatDuration(0)).toBe('--');
      expect(formatDuration(-5)).toBe('--');
      expect(formatDuration(Number.NaN)).toBe('--');
    });
  });
});

describe('formatTimeAgo', () => {
  it('英文单复数由语言包的 | 形式决定，不在代码里拼', () => {
    withLocale('en-US', () => {
      const now = Date.now();
      expect(formatTimeAgo(now - 3 * 3600 * 1000)).toBe('3 hours ago');
      expect(formatTimeAgo(now - 1 * 3600 * 1000 - 30 * 1000)).toBe('1 hour ago');
      expect(formatTimeAgo(now - 2 * 86400 * 1000)).toBe('2 days ago');
    });
  });

  it('中文不分单复数', () => {
    withLocale('zh-CN', () => {
      const now = Date.now();
      expect(formatTimeAgo(now - 1 * 3600 * 1000 - 30 * 1000)).toBe('1小时前');
      expect(formatTimeAgo(now - 30 * 1000)).toBe('刚刚');
    });
  });

  it('取不到时间给的是本地化的"未知"，不是空串或键名', () => {
    withLocale('en-US', () => expect(formatTimeAgo(0)).toBe('Unknown'));
    withLocale('zh-CN', () => expect(formatTimeAgo(0)).toBe('未知'));
  });
});
