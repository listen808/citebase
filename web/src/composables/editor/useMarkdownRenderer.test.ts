import { describe, it, expect, vi, afterAll } from 'vitest';
import { useMarkdownRenderer, setWikiLinkResolver } from './useMarkdownRenderer';

const { render, highlightCodeBlocks } = useMarkdownRenderer();

function htmlOf(md: string): { html: string; headings: Array<{ id: string; text: string; level: number }> } {
  return render(md);
}

describe('复现：文档内目录（锚点链接）点击跳转', () => {
  it('正文锚点链接不应在新标签页打开，且 href 应能命中标题 id', () => {
    const md = [
      '## 目录',
      '',
      '1. [全景流程图](#1-全景流程图)',
      '2. [角色与账号](#2-角色与账号)',
      '',
      '## 1. 全景流程图',
      '正文A',
      '',
      '## 2. 角色与账号',
      '正文B',
    ].join('\n');
    const { html, headings } = htmlOf(md);
    const anchors = html.match(/<a [^>]*>/g) || [];
    // eslint-disable-next-line no-console
    console.log('ANCHORS:', JSON.stringify(anchors, null, 1));
    // eslint-disable-next-line no-console
    console.log('HEADINGS:', JSON.stringify(headings.map((h) => ({ id: h.id, text: h.text })), null, 1));

    const div = document.createElement('div');
    div.innerHTML = html;
    const headingIds = Array.from(div.querySelectorAll('h2')).map((h) => h.id);
    // 期望1：锚点链接 target=_blank 不出现（页内跳转）
    const blankAnchors = anchors.filter((a) => a.includes('target="_blank"') && a.includes('href="#'));
    expect(blankAnchors, '锚点链接被强制 target=_blank').toEqual([]);
    // 期望2：作者可手写的锚点 href 能对应到标题 id
    const hrefs = anchors.map((a) => a.match(/href="([^"]*)"/)?.[1] || '');
    for (const href of hrefs) {
      if (href.startsWith('#')) {
        const id = decodeURIComponent(href.slice(1));
        expect(headingIds, `锚点 ${href} 应命中标题 id（实际 ids: ${headingIds.join(', ')}）`).toContain(id);
      }
    }
  });
});

describe('复现：shell 代码块渲染', () => {
  const shellScript = [
    '#!/bin/bash',
    'set -euo pipefail',
    'NAME="demo"',
    'echo "hello $NAME"',
    'FILES=$(ls /var/log | grep -c ".log")',
    "sed -i 's/old/new/g' config.yaml",
    'for f in *.txt; do',
    '  mv "$f" "backup/$f"',
    'done',
  ].join('\n');

  it('```shell 围栏：代码内容完整保留（含 $ 变量）', () => {
    const { html } = htmlOf('```shell\n' + shellScript + '\n```\n');
    const div = document.createElement('div');
    div.innerHTML = html;
    const codeEl = div.querySelector('pre code');
    const text = codeEl?.textContent || '';
    // eslint-disable-next-line no-console
    console.log('SHELL TEXT:', JSON.stringify(text));
    expect(text).toContain('#!/bin/bash');
    expect(text).toContain('echo "hello $NAME"');
    expect(text).toContain('FILES=$(ls /var/log | grep -c ".log")');
    expect(text).toContain('done');
    expect(codeEl?.className).toContain('language-shell');
  });

  it('```shell 围栏：高亮后行内容完整、无吞行', async () => {
    const { html } = htmlOf('```shell\n' + shellScript + '\n```\n');
    const container = document.createElement('div');
    container.innerHTML = html;
    document.body.appendChild(container);
    try {
      await highlightCodeBlocks(container);
      const lines = container.querySelectorAll('.code-line');
      // eslint-disable-next-line no-console
      console.log('HIGHLIGHTED LINE COUNT:', lines.length, '/ expect 9');
      expect(lines.length).toBe(9);
      const joined = Array.from(lines).map((l) => l.textContent || '').join('\n');
      expect(joined).toContain('echo "hello $NAME"');
    } finally {
      container.remove();
    }
  });

  it('~~~shell 波浪围栏（AI 生成文档常见）：$ 变量不被误当公式', () => {
    const { html } = htmlOf('~~~shell\necho "hello $NAME and $HOME"\n~~~\n');
    const div = document.createElement('div');
    div.innerHTML = html;
    const text = div.querySelector('pre code')?.textContent || '';
    // eslint-disable-next-line no-console
    console.log('TILDE FENCE TEXT:', JSON.stringify(text));
    expect(text).toContain('echo "hello $NAME and $HOME"');
  });

  it('列表项内的 ```bash 代码块正常渲染', () => {
    const md = [
      '1. 第一步',
      '',
      '   ```bash',
      '   export PATH=$PATH:/usr/local/bin',
      '   echo $HOME',
      '   ```',
      '',
      '2. 第二步',
    ].join('\n');
    const { html } = htmlOf(md);
    const div = document.createElement('div');
    div.innerHTML = html;
    const text = div.querySelector('pre code')?.textContent || '';
    // eslint-disable-next-line no-console
    console.log('LIST FENCE TEXT:', JSON.stringify(text));
    expect(text).toContain('export PATH=$PATH:/usr/local/bin');
    expect(text).toContain('echo $HOME');
  });

  it('CRLF 行尾的 shell 代码块正常渲染且无前导空行', () => {
    const md = '```shell\r\nline1 $A $B\r\nline2\r\n```';
    const { html } = htmlOf(md);
    const div = document.createElement('div');
    div.innerHTML = html;
    const text = div.querySelector('pre code')?.textContent || '';
    expect(text).toContain('line1 $A $B');
    expect(text).toContain('line2');
    // \r 残留在围栏信息串里会让代码内容多出前导空行（预览第一行显示为空）
    expect(text.startsWith('\n'), `代码内容不应以空行开头: ${JSON.stringify(text.slice(0, 20))}`).toBe(false);
  });

  it('BOM 前缀不破坏围栏识别', () => {
    const { html } = htmlOf('\uFEFF```shell\nline1 `cmd`\n```\n');
    expect(html).toContain('language-shell');
    expect(html).toContain('line1 `cmd`');
    expect(html).not.toContain('<h1');
  });

  it('代码内容含 $ 公式占位符 § 不受影响，且与真实公式互不干扰', () => {
    const { html } = htmlOf('```shell\necho "§KI0§"\n```\n\n$E=mc^2$ 与 $x+y$ 行内公式\n');
    const div = document.createElement('div');
    div.innerHTML = html;
    const codeText = div.querySelector('pre code')?.textContent || '';
    // eslint-disable-next-line no-console
    console.log('PLACEHOLDER COLLISION CODE:', JSON.stringify(codeText));
    // 代码块里的字面量 §KI0§ 原样保留，不被真实公式的占位符替换波及
    expect(codeText).toContain('§KI0§');
    // 真实行内公式正常渲染
    expect(html).toContain('katex');
  });
});

describe('复现：其他常见语法', () => {
  it('任务列表 - [ ] 渲染出可识别的勾选框', () => {
    const { html } = htmlOf('- [ ] 待办一\n- [x] 已办二\n');
    const div = document.createElement('div');
    div.innerHTML = html;
    // eslint-disable-next-line no-console
    console.log('TASK LIST HTML:', JSON.stringify(div.querySelector('ul')?.innerHTML));
    const checkbox = div.querySelector('input[type="checkbox"], .task-list-item');
    expect(checkbox, '任务列表勾选框丢失（被 DOMPurify 剥离）').toBeTruthy();
  });

  it('多行块级公式 $$\\n…\\n$$ 渲染为 KaTeX 而非原文', async () => {
    const md = '$$\n\\int_0^1 x^2 dx = \\frac{1}{3}\n$$\n';
    // KaTeX 为动态 import 预加载，等待其就绪后重试渲染（pending 渲染不缓存）
    await vi.waitFor(() => {
      const { html } = htmlOf(md);
      expect(html).toContain('katex');
    }, { timeout: 5000 });
    const { html } = htmlOf(md);
    expect(html).not.toContain('§K');
  });

  it('行内代码中的 $不成对内容不被误当公式', () => {
    const { html } = htmlOf('运行 `cost $5 和 $6` 命令\n');
    const div = document.createElement('div');
    div.innerHTML = html;
    const codeText = div.querySelector('code')?.textContent || '';
    // eslint-disable-next-line no-console
    console.log('INLINE CODE TEXT:', JSON.stringify(codeText));
    expect(codeText).toBe('cost $5 和 $6');
  });
});

describe('新增特性', () => {
  afterAll(() => {
    setWikiLinkResolver(null);
  });

  it('标题带 hover 锚点，锚点 href 指向标题 id', () => {
    const { html } = htmlOf('## 概述\n\n正文\n');
    const div = document.createElement('div');
    div.innerHTML = html;
    const anchor = div.querySelector('.doc-heading a.heading-anchor') as HTMLAnchorElement | null;
    expect(anchor, '标题锚点缺失').toBeTruthy();
    expect(anchor!.getAttribute('href')).toBe(`#${div.querySelector('.doc-heading')!.id}`);
  });

  it('[[wiki 双链]] 未解析时链接到全文检索并带 missing 样式', () => {
    const { html } = htmlOf('参见 [[不存在的页面]] 与 [[页面名|别名]]\n');
    const div = document.createElement('div');
    div.innerHTML = html;
    const links = Array.from(div.querySelectorAll('a.wikilink')) as HTMLAnchorElement[];
    expect(links.length).toBe(2);
    expect(links[0]!.classList.contains('wikilink-missing')).toBe(true);
    expect(links[0]!.getAttribute('href')).toBe(`/search?q=${encodeURIComponent('不存在的页面')}`);
    expect(links[1]!.textContent).toBe('别名');
  });

  it('[[wiki 双链]] 有解析器时解析为站内路由', () => {
    setWikiLinkResolver((name) => (name === '集客' ? '/space/1/doc/12' : null));
    const { html } = htmlOf('参见 [[集客]]\n');
    const div = document.createElement('div');
    div.innerHTML = html;
    const link = div.querySelector('a.wikilink:not(.wikilink-missing)') as HTMLAnchorElement | null;
    expect(link, '双链未解析').toBeTruthy();
    expect(link!.getAttribute('href')).toBe('/space/1/doc/12');
  });

  it('代码块内的 [[x]] 与 :tada: 不被替换', () => {
    const { html } = htmlOf('```\n[[raw]] :tada: [[a|b]]\n```\n');
    const div = document.createElement('div');
    div.innerHTML = html;
    const text = div.querySelector('pre code')?.textContent || '';
    expect(text).toContain('[[raw]] :tada: [[a|b]]');
  });

  it('front-matter 被剥离不渲染', () => {
    const { html } = htmlOf('---\ntitle: 测试\nstatus: draft\n---\n\n# 正文标题\n');
    expect(html).not.toContain('title:');
    expect(html).not.toContain('status:');
    expect(html).toContain('正文标题');
  });

  it('脚注引用与定义渲染为 sup 链接 + 文末列表', () => {
    const { html } = htmlOf('正文有脚注[^1] 和第二个[^note]。\n\n[^1]: 第一个脚注内容\n[^note]: 第二个脚注\n');
    expect(html).toContain('footnote-ref');
    expect(html).toContain('footnotes');
    expect(html).toContain('第一个脚注内容');
    const div = document.createElement('div');
    div.innerHTML = html;
    // 引用顺序按定义编号，回链指向 fnref
    expect(div.querySelector('sup.footnote-ref a')?.getAttribute('href')).toBe('#fn-1');
    expect(div.querySelector('.footnotes li#fn-1')).toBeTruthy();
    expect(div.querySelector('.footnotes li#fn-1 a.footnote-backref')?.getAttribute('href')).toBe('#fnref-1');
  });

  it('emoji 短代码渲染为 emoji', () => {
    const { html } = htmlOf('发布成功 :tada: 修好了 :bug:，时间 12:30 不受影响\n');
    expect(html).toContain('🎉');
    expect(html).toContain('🐛');
    expect(html).toContain('12:30');
  });
});
