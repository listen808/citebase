/**
 * 文本高亮工具函数
 */

import { escapeHtml } from '@/utils/core/shared';

/**
 * 在文本中高亮显示关键词
 * @param text 原始文本
 * @param keyword 要高亮的关键词
 * @returns HTML 字符串（包含高亮标记）
 */
export function highlightText(text: string, keyword: string): string {
  if (!keyword || !text) return escapeHtml(text);

  const escapedText = escapeHtml(text);
  const escapedKeyword = escapeRegExp(keyword);
  const regex = new RegExp(`(${escapedKeyword})`, 'gi');
  return escapedText.replace(regex, '<mark class="highlight">$1</mark>');
}

/**
 * 净化并高亮搜索结果中的高亮片段
 * 确保来自后端的高亮内容是安全的
 */
export function sanitizeAndHighlightHighlight(highlight: string, keyword: string): string {
  if (!highlight) return '';
  
  // 先完全转义
  const escaped = escapeHtml(highlight);
  
  // 如果有关键词,再高亮
  if (keyword) {
    const escapedKeyword = escapeRegExp(keyword);
    const regex = new RegExp(`(${escapedKeyword})`, 'gi');
    return escaped.replace(regex, '<mark class="highlight">$1</mark>');
  }
  
  return escaped;
}

/**
 * 转义正则表达式特殊字符
 */
function escapeRegExp(string: string): string {
  return string.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
}

/**
 * 净化后端返回的高亮片段
 * 只允许 <mark> 标签通过，其他 HTML 标签全部转义
 */
export function sanitizeHighlight(highlight: string): string {
  if (!highlight) return '';

  // 先将 <mark> 和 </mark> 替换为占位符
  const MARK_OPEN = '\x00MARK_OPEN\x00';
  const MARK_CLOSE = '\x00MARK_CLOSE\x00';

  let temp = highlight
    .replace(/<mark\b[^>]*>/gi, MARK_OPEN)
    .replace(/<\/mark>/gi, MARK_CLOSE);

  // 转义所有剩余 HTML
  temp = temp
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&#039;');

  // 恢复 <mark> 标签
  temp = temp
    .replace(new RegExp(MARK_OPEN, 'g'), '<mark class="highlight">')
    .replace(new RegExp(MARK_CLOSE, 'g'), '</mark>');

  return temp;
}

/**
 * 获取搜索结果的高亮片段（优先使用新的多片段，fallback 到旧的单片段）
 */
export function getHighlightFragments(r: { highlights?: Record<string, string>; highlight_fragments?: Record<string, import('@/types/models').HighlightFragment[]> }): import('@/types/models').HighlightFragment[] {
  // 优先使用新的多片段
  if (r.highlight_fragments?.content && r.highlight_fragments.content.length > 0) {
    return r.highlight_fragments.content;
  }
  // fallback 到旧的单片段
  if (r.highlights?.content) {
    return [{ text: r.highlights.content, html: sanitizeHighlight(r.highlights.content), score: 1 }];
  }
  return [];
}

/**
 * 净化片段 HTML（仅允许 <mark> 标签）
 */
export function sanitizeFragmentHTML(html: string): string {
  return sanitizeHighlight(html);
}

/**
 * 在 Markdown 内容中高亮关键词（保护代码块、行内代码、链接、图片等）
 */
export function highlightMarkdownContent(markdown: string, keywords: string[]): string {
  if (!markdown || !keywords || keywords.length === 0) return escapeHtml(markdown);

  // 提取并保护各种区块
  const protectedBlocks: string[] = [];
  let processed = markdown;

  // 保护围栏代码块 (```...```)
  processed = processed.replace(/```[\s\S]*?```/g, (match) => {
    protectedBlocks.push(match);
    return `\uE000BLOCK${protectedBlocks.length - 1}\uE000`;
  });

  // 保护行内代码 (`...`)
  processed = processed.replace(/`[^`]+`/g, (match) => {
    protectedBlocks.push(match);
    return `\uE000BLOCK${protectedBlocks.length - 1}\uE000`;
  });

  // 保护图片 ![alt](url)
  processed = processed.replace(/!\[[^\]]*\]\([^)]+\)/g, (match) => {
    protectedBlocks.push(match);
    return `\uE000BLOCK${protectedBlocks.length - 1}\uE000`;
  });

  // 保护链接 URL 部分 [text](url) - 只保护 url
  processed = processed.replace(/\]\(([^)]+)\)/g, (match, url) => {
    protectedBlocks.push(url);
    return `](\uE000BLOCK${protectedBlocks.length - 1}\uE000)`;
  });

  // 在剩余文本中高亮关键词
  const sortedKeywords = [...keywords].sort((a, b) => b.length - a.length);
  const escapedKeywords = sortedKeywords.map(k => escapeRegExp(escapeHtml(k))).join('|');
  if (escapedKeywords) {
    const regex = new RegExp(`(${escapedKeywords})`, 'gi');
    processed = escapeHtml(processed).replace(regex, '<mark class="highlight">$1</mark>');
  } else {
    processed = escapeHtml(processed);
  }

  // 恢复所有保护的区块
  processed = processed.replace(/\uE000BLOCK(\d+)\uE000/g, (_, idx) => {
    const block = protectedBlocks[parseInt(idx)];
    return block ? escapeHtml(block) : '';
  });

  return processed;
}
