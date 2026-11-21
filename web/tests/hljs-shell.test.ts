import { describe, it, expect } from 'vitest';
import { useMarkdownRenderer } from '@/composables/editor/useMarkdownRenderer';

const { render, highlightCodeBlocks } = useMarkdownRenderer();

const script = [
  '#!/bin/bash -x',
  '# 指定应用的搜索关键字',
  'keyword="app"',
  'coreSize=`grep processor /proc/cpuinfo|wc -l`',
  'if [ -n "$1" ];then',
  '    keyword=$1',
  'fi',
].join('\n');

async function highlightSpanCount(lang: string, code: string): Promise<{ spans: number; text: string }> {
  const { html } = render('```' + lang + '\n' + code + '\n```\n');
  const container = document.createElement('div');
  container.innerHTML = html;
  document.body.appendChild(container);
  try {
    await highlightCodeBlocks(container);
    const spans = container.querySelectorAll('pre code span[class*="hljs-"]');
    return { spans: spans.length, text: container.querySelector('pre code')?.textContent || '' };
  } finally {
    container.remove();
  }
}

describe('hljs shell 语法着色', () => {
  it('```shell（脚本内容）应有 bash 级语法着色', async () => {
    const { spans, text } = await highlightSpanCount('shell', script);
    // eslint-disable-next-line no-console
    console.log('SHELL SPANS:', spans);
    expect(text).toContain('coreSize=`grep');
    // hljs 的 shell 语法是终端会话高亮，脚本只有 2 个 span；映射到 bash 后应 ≥5
    expect(spans, 'shell 脚本未被按 bash 语法着色').toBeGreaterThanOrEqual(5);
  });

  it('```bash 应有着色（对照组）', async () => {
    const { spans } = await highlightSpanCount('bash', script);
    // eslint-disable-next-line no-console
    console.log('BASH SPANS:', spans);
    expect(spans).toBeGreaterThan(0);
  });

  it('$ 提示符的终端会话内容保持原语义', async () => {
    const { text } = await highlightSpanCount('shell', '$ ls -la\n$ cd /tmp\ntotal 0');
    expect(text).toContain('$ ls -la');
  });
});
