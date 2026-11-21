import type { editor } from 'monaco-editor';
import i18n from '@/i18n';

const { t } = i18n.global;

type Editor = editor.IStandaloneCodeEditor;

interface InsertSpec {
  text: string;
  cursorOffset: number;
}

/**
 * 统一的 Markdown 插入能力层：工具栏按钮、右键菜单、Slash 命令全部复用。
 */
export function useMarkdownInsert(getEditor: () => Editor | null) {
  function wrapSelection(type: string): void {
    const ed = getEditor();
    if (!ed) return;
    const selection = ed.getSelection();
    const model = ed.getModel();
    if (!selection || !model) return;
    const selectedText = model.getValueInRange(selection) || '';
    if (!selectedText) return;

    let wrapped: string;
    switch (type) {
      case 'bold':
        wrapped = `**${selectedText}**`;
        break;
      case 'italic':
        wrapped = `*${selectedText}*`;
        break;
      case 'heading':
        wrapped = `## ${selectedText}`;
        break;
      case 'link':
        wrapped = `[${selectedText}](url)`;
        break;
      case 'code':
      case 'inline-code':
        wrapped = `\`${selectedText}\``;
        break;
      default:
        return;
    }
    ed.executeEdits('markdown-wrap', [{ range: selection, text: wrapped }]);
    ed.focus();
  }

  function applyMarkdown(type: string, opts?: { lang?: string }): boolean {
    const ed = getEditor();
    if (!ed) return false;
    const selection = ed.getSelection();
    const model = ed.getModel();
    if (!selection || !model) return false;

    if (type === 'undo') {
      ed.trigger('markdown-toolbar', 'undo', null);
      ed.focus();
      return true;
    }
    if (type === 'redo') {
      ed.trigger('markdown-toolbar', 'redo', null);
      ed.focus();
      return true;
    }
    // 表格需要弹窗选择行列，由调用方处理
    if (type === 'table') return true;

    const selectedText = model.getValueInRange(selection) || '';
    const spec = buildInsert(type, selectedText, opts?.lang);
    if (!spec) return false;

    ed.executeEdits('markdown-toolbar', [{ range: selection, text: spec.text }]);
    if (!selectedText && spec.cursorOffset !== 0) {
      const pos = ed.getPosition();
      if (pos) {
        ed.setPosition({ lineNumber: pos.lineNumber, column: Math.max(1, pos.column + spec.cursorOffset) });
      }
    }
    ed.focus();
    return true;
  }

  // Slash / 内容类命令使用的纯文本（无选区、无光标调整）
  function getInsertText(type: string): string {
    const spec = buildInsert(type, '');
    return spec ? spec.text : '';
  }

  function buildTableMarkdown(rows: number, cols: number): string {
    const r = Math.max(1, Math.min(20, rows));
    const c = Math.max(1, Math.min(10, cols));
    const header = '| ' + Array.from({ length: c }, (_, i) => `列${i + 1}`).join(' | ') + ' |';
    const separator = '| ' + Array.from({ length: c }, () => '---').join(' | ') + ' |';
    const body = Array.from({ length: r - 1 }, () => '| ' + Array.from({ length: c }, () => '内容').join(' | ') + ' |').join('\n');
    return `\n${header}\n${separator}\n${body}\n`;
  }

  return { wrapSelection, applyMarkdown, getInsertText, buildTableMarkdown };
}

function buildInsert(type: string, selectedText: string, _lang?: string): InsertSpec | null {
  if (type.startsWith('code:')) {
    const l = type.slice(5);
    return { text: `\n\`\`\`${l}\n${selectedText || '代码'}\n\`\`\`\n`, cursorOffset: selectedText ? 0 : -5 };
  }
  switch (type) {
    case 'heading':
      return { text: `\n## ${selectedText || '标题'}\n`, cursorOffset: selectedText ? 0 : -1 };
    case 'bold':
      return { text: `**${selectedText || '粗体文本'}**`, cursorOffset: selectedText ? 0 : -4 };
    case 'italic':
      return { text: `*${selectedText || '斜体文本'}*`, cursorOffset: selectedText ? 0 : -4 };
    case 'link':
      return { text: `[${selectedText || '链接文本'}](url)`, cursorOffset: selectedText ? 0 : -5 };
    case 'code':
      return { text: `\n\`\`\`\n${selectedText || '代码'}\n\`\`\`\n`, cursorOffset: selectedText ? 0 : -5 };
    case 'inline-code':
      return { text: `\`${selectedText || '代码'}\``, cursorOffset: selectedText ? 0 : -1 };
    case 'list':
      return { text: `\n- ${selectedText || '列表项'}\n`, cursorOffset: selectedText ? 0 : -1 };
    case 'ordered-list':
      return { text: `\n1. ${selectedText || '列表项'}\n`, cursorOffset: selectedText ? 0 : -1 };
    case 'task':
      return { text: `\n- [ ] ${selectedText || '待办事项'}\n- [ ] \n`, cursorOffset: selectedText ? 0 : -1 };
    case 'quote':
      return { text: `\n> ${selectedText || '引用文本'}\n`, cursorOffset: selectedText ? 0 : -1 };
    case 'table':
      return { text: '\n| 列1 | 列2 | 列3 |\n| --- | --- | --- |\n| 内容 | 内容 | 内容 |\n', cursorOffset: 0 };
    case 'callout':
      return { text: `\n> [!note]\n> ${selectedText || '提示内容'}\n`, cursorOffset: selectedText ? 0 : -1 };
    case 'toggle':
      return { text: `\n> [!toggle] 标题\n> ${selectedText || '内容'}\n`, cursorOffset: selectedText ? 0 : -1 };
    case 'mermaid':
    case 'mermaid-flow':
      return {
        text: '\n```mermaid\nflowchart TD\n    A[开始] --> B{判断}\n    B -->|是| C[结果1]\n    B -->|否| D[结果2]\n```\n',
        cursorOffset: -5,
      };
    case 'mermaid-sequence':
      return {
        text: t('tsUi.k12'),
        cursorOffset: -5,
      };
    case 'mermaid-gantt':
      return {
        text: t('tsUi.k13'),
        cursorOffset: -5,
      };
    case 'embed':
      return { text: '\n```embed\nhttps://www.youtube.com/watch?v=dQw4w9WgXcQ\n```\n', cursorOffset: -5 };
    case 'math':
      return {
        text: `\n$$\n${selectedText || "x = \\frac{-b \\pm \\sqrt{b^2-4ac}}{2a}"}\n$$\n`,
        cursorOffset: selectedText ? 0 : -3,
      };
    case 'hr':
      return { text: '\n---\n', cursorOffset: 0 };
    case 'image':
      return { text: `![${selectedText || '图片描述'}](url)`, cursorOffset: 0 };
    case 'doc-link':
      return { text: '[[', cursorOffset: 0 };
    default:
      return null;
  }
}