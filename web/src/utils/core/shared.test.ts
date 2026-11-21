import { describe, it, expect } from 'vitest';
import {
  escapeHtml,
  hexToRgba,
  withAlpha,
  isSafeRedirect,
} from '@/utils/core/shared';

// 注：slugify 不在这里——useMarkdownRenderer.ts 的本地版是 Unicode 增强实现（\p{L} 任意字母、
// 去 HTML 标签、空标题回退），与曾经 shared.ts 的 \w+CJK 版语义不同且更强，故删 shared 版保留本地版。

describe('escapeHtml', () => {
  it('先转义 & 再转其余字符，避免二次转义', () => {
    expect(escapeHtml('<a href="x">&\'</a>')).toBe('&lt;a href=&quot;x&quot;&gt;&amp;&#039;&lt;/a&gt;');
  });

  it('普通文本原样返回', () => {
    expect(escapeHtml('纯文本 123')).toBe('纯文本 123');
  });
});

describe('hexToRgba', () => {
  it('十六进制转 rgba', () => {
    expect(hexToRgba('#0071e3', 0.5)).toBe('rgba(0,113,227,0.5)');
  });

  it('三位缩写补全通道', () => {
    expect(hexToRgba('#fff', 1)).toBe('rgba(255,255,255,1)');
  });

  it('非十六进制输入退化为 color-mix 以保持主题联动', () => {
    expect(hexToRgba('var(--apple-blue)', 0.12)).toBe('color-mix(in srgb, var(--apple-blue) 12%, transparent)');
  });
});
describe('withAlpha', () => {
  it('追加透明度', () => {
    expect(withAlpha('#0071e3', 0.8)).toBe('rgba(0,113,227,0.8)');
  });

  it('替换已有 rgba 的 alpha', () => {
    expect(withAlpha('rgba(1,2,3,0.2)', 0.9)).toBe('rgba(1,2,3,0.9)');
  });

  it('透明度夹在 0-1', () => {
    expect(withAlpha('#ffffff', 5)).toBe('rgba(255,255,255,1)');
    expect(withAlpha('#ffffff', -1)).toBe('rgba(255,255,255,0)');
  });

  it('非颜色值原样返回，交由 CSS 自行解析', () => {
    expect(withAlpha('var(--apple-blue)', 0.5)).toBe('var(--apple-blue)');
  });
});

describe('isSafeRedirect', () => {
  it('站内相对路径与 http(s) 判为安全', () => {
    expect(isSafeRedirect('/dashboard')).toBe(true);
    expect(isSafeRedirect('https://example.com/x')).toBe(true);
  });

  it('javascript: 等危险协议判为不安全', () => {
    expect(isSafeRedirect('javascript:alert(1)')).toBe(false);
    expect(isSafeRedirect('data:text/html,<script>')).toBe(false);
  });
});
