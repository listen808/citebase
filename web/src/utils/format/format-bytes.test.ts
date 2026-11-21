import { describe, it, expect } from 'vitest';
import { formatBytes } from '@/utils/format/format-bytes';

describe('formatBytes', () => {
  it('0 与负数都归零', () => {
    expect(formatBytes(0)).toBe('0 B');
    expect(formatBytes(-1)).toBe('0 B');
  });

  it('小于 1KB 不带小数', () => {
    expect(formatBytes(500)).toBe('500 B');
    expect(formatBytes(1023)).toBe('1023 B');
  });

  it('进位后保留一位小数', () => {
    expect(formatBytes(1024)).toBe('1.0 KB');
    expect(formatBytes(1536)).toBe('1.5 KB');
    expect(formatBytes(1048576)).toBe('1.0 MB');
    expect(formatBytes(1073741824)).toBe('1.0 GB');
  });
});
