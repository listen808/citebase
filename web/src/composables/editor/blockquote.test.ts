import { describe, it, expect } from 'vitest';
import { useMarkdownRenderer } from './useMarkdownRenderer';

/**
 * 引用块内的行内语法必须被解析（marked v12+ 之后 renderer 收到的是 token，
 * 早期实现直接用了未解析的 token.text，导致 `**粗体**` 按字面量显示）。
 */
describe('blockquote 渲染', () => {
  const { render } = useMarkdownRenderer();
  const htmlOf = (md: string) => render(md).html;

  it('引用块内解析粗体、行内代码与链接', () => {
    const html = htmlOf('> 状态：**设计规范**，配套 `theme-live-preview.html`，见 [规范](/docs)\n');
    expect(html).toContain('<blockquote>');
    expect(html).toContain('<strong>设计规范</strong>');
    expect(html).toContain('<code>theme-live-preview.html</code>');
    expect(html).toContain('<a href="/docs"');
    expect(html).not.toContain('**设计规范**');
  });

  it('引用块内多行合并为一个段落并保留行内标记', () => {
    const html = htmlOf('> 日期：2026-09-28\n> 约束：**两套基础主题原样保留**\n');
    expect(html).toContain('<strong>两套基础主题原样保留</strong>');
    expect(html).not.toMatch(/\*\*/);
  });

  it('[!note] 等提示块语法仍然生效', () => {
    const html = htmlOf('> [!warning] 这里要**注意**\n');
    expect(html).toContain('callout-warning');
    expect(html).toContain('callout-title');
    expect(html).toContain('<strong>注意</strong>');
  });

  it('[!toggle] 渲染为可折叠块', () => {
    const html = htmlOf('> [!toggle] 折叠标题\n> 折叠内容\n');
    expect(html).toContain('<details');
    expect(html).toContain('toggle-block');
  });
});
