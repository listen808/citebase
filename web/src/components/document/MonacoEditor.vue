<template>
  <div ref="containerRef" :style="containerStyle">
    <!-- 右键上下文菜单：格式化 + AI -->
    <div
      v-if="showContextMenu"
      ref="contextMenuRef"
      class="apple-glass-layer"
      :style="contextMenuStyle"
    >
      <template v-if="hasSelection">
        <ApButton variant="ghost" size="small" @click.stop="wrapSelection('bold')"><ApIcon name="Bold" :size="16" /><span :style="ctxLabelStyle">{{ $t('docUi1.k55') }}</span></ApButton>
        <ApButton variant="ghost" size="small" @click.stop="wrapSelection('italic')"><ApIcon name="Italic" :size="16" /><span :style="ctxLabelStyle">{{ $t('docUi1.k56') }}</span></ApButton>
        <ApButton variant="ghost" size="small" @click.stop="wrapSelection('heading')"><ApIcon name="Heading" :size="16" /><span :style="ctxLabelStyle">{{ $t('docUi1.k57') }}</span></ApButton>
        <ApButton variant="ghost" size="small" @click.stop="wrapSelection('link')"><ApIcon name="Link" :size="16" /><span :style="ctxLabelStyle">{{ $t('docUi1.k58') }}</span></ApButton>
        <ApButton variant="ghost" size="small" @click.stop="wrapSelection('code')"><ApIcon name="Code" :size="16" /><span :style="ctxLabelStyle">{{ $t('docUi1.k59') }}</span></ApButton>
        <template v-if="aiStore.enabled">
          <div :style="ctxDividerStyle" />
          <ApButton variant="ghost" size="small" @click.stop="executeAiAction('rewrite')"><ApIcon name="RefreshCw" :size="16" /><span :style="ctxLabelStyle">{{ $t('docUi1.k30') }}</span></ApButton>
          <ApButton variant="ghost" size="small" @click.stop="executeAiAction('continue')"><ApIcon name="ArrowRight" :size="16" /><span :style="ctxLabelStyle">{{ $t('docUi1.k31') }}</span></ApButton>
          <ApButton variant="ghost" size="small" @click.stop="executeAiAction('translate')"><ApIcon name="Languages" :size="16" /><span :style="ctxLabelStyle">{{ $t('docUi1.k32') }}</span></ApButton>
          <ApButton variant="ghost" size="small" @click.stop="executeAiAction('summarize')"><ApIcon name="FileText" :size="16" /><span :style="ctxLabelStyle">{{ $t('docUi1.k33') }}</span></ApButton>
          <ApButton variant="ghost" size="small" @click.stop="executeAiAction('explain')"><ApIcon name="Code" :size="16" /><span :style="ctxLabelStyle">{{ $t('docUi1.k60') }}</span></ApButton>
          <div :style="ctxDividerStyle" />
        </template>
      </template>
      <ApButton variant="ghost" size="small" @click.stop="handleCtxCopy"><ApIcon name="Copy" :size="16" /><span :style="ctxLabelStyle">{{ $t('docUi1.copy') }}</span></ApButton>
      <ApButton variant="ghost" size="small" @click.stop="handleCtxCut"><ApIcon name="Scissors" :size="16" /><span :style="ctxLabelStyle">{{ $t('docUi1.k62') }}</span></ApButton>
      <ApButton variant="ghost" size="small" @click.stop="handleCtxPaste"><ApIcon name="ClipboardPaste" :size="16" /><span :style="ctxLabelStyle">{{ $t('docUi1.k63') }}</span></ApButton>
      <ApButton variant="ghost" size="small" @click.stop="handleCtxPasteImage"><ApIcon name="ImagePlus" :size="16" /><span :style="ctxLabelStyle">{{ $t('docUi1.k64') }}</span></ApButton>
    </div>

    <!-- AI自动补全提示 -->
    <div
      v-if="aiGhostText"
      :style="aiGhostHintStyle"
    >
      <span :style="resultCountStyle">{{ $t('docUi1.k65') }}</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, onMounted, onUnmounted, watch } from 'vue';
import * as monaco from 'monaco-editor';
import { setupMonacoWorkers } from '@/utils/domain/monaco-setup';
import { getCommonEditorOptions } from '@/utils/domain/monaco-config';
import { searchDocuments } from '@/api/modules/document/document';
import { logger } from '@/utils/logger';
import { copyText } from '@/utils/clipboard';
import { defineMonacoThemes, toAppleMonacoTheme } from '@/styles/theme/highlight-tokens';

import { useAIStore } from '@/stores/ai/ai';
import { useUIStore } from '@/stores/app/ui';
import { useClickOutsideClose } from '@/composables/ui/useClickOutsideClose';
import { useEscapeClose } from '@/composables/ui/useEscapeClose';
import { useMarkdownInsert } from '@/composables/editor/useMarkdownInsert';
import { useAIWrite, type EditorAIAction } from '@/composables/ai/useAIWrite';
import { CONTENT_SLASH_COMMANDS, AI_SLASH_COMMANDS, SLASH_TRIGGER_REGEX } from '@/constants/editor-slash';
import { AppleSpace, AppleType, AppleZIndex, cv, menuSurfaceStyle } from '@/utils/core/apple-tokens';
import { resultCountStyle } from '@/styles/component/spotlight-search-styles';
import { monacoThemeForUi } from '@/styles/theme/highlight-tokens';

const props = defineProps<{
  modelValue: string;
  language?: string;
  theme?: string;
  spaceId?: number;
  enableAutoComplete?: boolean;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void;
  (e: 'imagePaste', file: File): void;
  (e: 'cycleViewMode'): void;
  (e: 'toggleSplitMode'): void;
  (e: 'showShortcuts'): void;
  (e: 'openAiWriter'): void;
}>();
const { t } = useI18n();

const containerRef = ref<HTMLElement | null>(null);
const contextMenuRef = ref<HTMLElement | null>(null);
let editor: monaco.editor.IStandaloneCodeEditor | null = null;

const aiStore = useAIStore();
const ui = useUIStore();

// P1-10: AI 请求代次 + 文档快照校验。await 前抓的 selection/range 只是坐标快照，
// 请求期间用户继续编辑或切换文档（model 被整体替换）时旧 range 指向完全不同内容，
// 直接 executeEdits 会把 AI 结果写到错误区域。回写前必须确认：
// 仍是最新一次请求、model 未更换、文档版本未变化；否则丢弃结果并提示。
let aiWriteGeneration = 0;

const showContextMenu = shallowRef(false);
const contextMenuPos = shallowRef({ top: 0, left: 0 });
const hasSelection = shallowRef(false);
let lastSelection: monaco.IRange | null = null;

const { wrapSelection, applyMarkdown, getInsertText } = useMarkdownInsert(() => editor);
const { run: runAI, isRunning } = useAIWrite();

// ==================== AI自动补全 ====================
const aiGhostText = shallowRef('');
let ghostDecorationIds: string[] = [];
let ghostRange: monaco.IRange | null = null;

const containerStyle = {
  width: '100%',
  height: '100%',
  overflow: 'hidden',
  position: 'relative' as const,
};

const contextMenuStyle = computed(() => ({
  position: 'fixed' as const,
  top: `${contextMenuPos.value.top}px`,
  left: `${contextMenuPos.value.left}px`,
  display: 'flex',
  flexDirection: 'column' as const,
  ...menuSurfaceStyle(),
  minWidth: '150px',
  padding: `${AppleSpace.xs} 0`,
}));

const ctxLabelStyle = {
  flex: '1',
  lineHeight: '1',
};

const ctxDividerStyle = {
  height: '1px',
  background: cv('border-light'),
  margin: `${AppleSpace.xs} 0`,
};

function showContextMenuAt(x: number, y: number) {
  if (!editor) return;
  const selection = editor.getSelection();
  hasSelection.value = !!(selection && !selection.isEmpty());
  if (hasSelection.value) {
    lastSelection = {
      startLineNumber: selection!.startLineNumber,
      startColumn: selection!.startColumn,
      endLineNumber: selection!.endLineNumber,
      endColumn: selection!.endColumn,
    };
  } else {
    lastSelection = null;
  }
  const vw = window.innerWidth;
  const vh = window.innerHeight;
  contextMenuPos.value = {
    top: y + 160 > vh ? Math.max(0, y - 160) : y,
    left: x + 170 > vw ? Math.max(0, x - 170) : x,
  };
  showContextMenu.value = true;
}

function hideContextMenu() {
  showContextMenu.value = false;
}

// P1-10: 回写前校验代次/model/版本，任一不满足即丢弃（版本变化时提示用户）
function canApplyAiResult(gen: number, model: monaco.editor.ITextModel, versionBefore: number): boolean {
  if (gen !== aiWriteGeneration || !editor) return false;
  if (editor.getModel() !== model) return false; // 文档已被切换（model 整体更换）
  if (model.getVersionId() !== versionBefore) {
    ui.showToast(t('docUi1.k11'), 'warning');
    return false;
  }
  return true;
}

async function executeAiAction(action: EditorAIAction) {
  if (!editor || isRunning()) return;
  const selection = lastSelection
    ? new monaco.Selection(
        lastSelection.startLineNumber, lastSelection.startColumn,
        lastSelection.endLineNumber, lastSelection.endColumn,
      )
    : editor.getSelection();
  if (!selection) return;
  const model = editor.getModel();
  if (!model) return;
  const selectedText = model.getValueInRange(selection) || '';
  if (!selectedText.trim()) return;
  hideContextMenu();
  const gen = ++aiWriteGeneration;
  const versionBefore = model.getVersionId();
  const result = await runAI(action, selectedText);
  // P1-10: 不再用 await 前的旧 range 盲写
  if (result && editor && canApplyAiResult(gen, model, versionBefore)) {
    editor.executeEdits('ai-ctx-action', [{ range: selection, text: result }]);
    editor.focus();
  }
}

function getSelectionRange(): monaco.IRange | null {
  if (!editor) return null;
  const selection = editor.getSelection();
  if (!selection || selection.isEmpty()) return null;
  return {
    startLineNumber: selection.startLineNumber,
    startColumn: selection.startColumn,
    endLineNumber: selection.endLineNumber,
    endColumn: selection.endColumn,
  };
}

function handleCtxCopy() {
  const model = editor?.getModel();
  const range = getSelectionRange();
  hideContextMenu();
  if (!model || !range) return;
  const text = model.getValueInRange(range);
  if (text) void copyText(text);
  editor?.focus();
}

function handleCtxCut() {
  const model = editor?.getModel();
  const range = getSelectionRange();
  hideContextMenu();
  if (!model || !range) return;
  const text = model.getValueInRange(range);
  if (!text) return;
  void copyText(text).then((ok) => {
    if (ok && editor) {
      editor.executeEdits('ctx-cut', [{ range, text: '' }]);
      editor.focus();
    }
  });
}

async function handleCtxPaste() {
  hideContextMenu();
  if (!editor) return;
  let text = '';
  if (navigator.clipboard && typeof navigator.clipboard.readText === 'function') {
    try {
      text = await navigator.clipboard.readText();
    } catch (err) {
      logger.warn('Clipboard readText failed', err);
    }
  }
  if (text) {
    const selection = editor.getSelection();
    const pos = editor.getPosition();
    const range: monaco.IRange | null = selection && !selection.isEmpty()
      ? selection
      : pos
        ? new monaco.Range(pos.lineNumber, pos.column, pos.lineNumber, pos.column)
        : null;
    if (range) {
      editor.executeEdits('ctx-paste', [{ range, text }]);
    }
  }
  editor.focus();
}

async function handleCtxPasteImage() {
  hideContextMenu();
  if (!navigator.clipboard || typeof navigator.clipboard.read !== 'function') {
    ui.showToast(t('docUi1.k67'), 'warning');
    return;
  }
  try {
    const items = await navigator.clipboard.read();
    for (const item of items) {
      const imageType = item.types.find((t) => t.startsWith('image/'));
      if (!imageType) continue;
      const blob = await item.getType(imageType);
      const ext = imageType.split('/')[1] || 'png';
      const file = new File([blob], `pasted-image.${ext}`, { type: imageType });
      emit('imagePaste', file);
      return;
    }
    ui.showToast(t('docUi1.k68'), 'info');
  } catch (err) {
    logger.warn('Read clipboard image failed', err);
    ui.showToast(t('docUi1.k69'), 'warning');
  }
}

const aiGhostHintStyle = computed(() => ({
  position: 'absolute' as const,
  bottom: '8px',
  right: '12px',
  zIndex: String(AppleZIndex.raised),
  pointerEvents: 'none' as const,
}));



function handleDocumentPaste(event: ClipboardEvent) {
  const target = event.target as Node;
  if (!containerRef.value) return;

  const isEditorRelated = containerRef.value.contains(target);

  if (!isEditorRelated) return;

  const items = event.clipboardData?.items;
  if (items && items.length > 0) {
    for (let i = 0; i < items.length; i++) {
      const item = items[i];
      if (item && item.type.startsWith('image/')) {
        event.preventDefault();
        event.stopPropagation();
        const file = item.getAsFile();
        if (file) emit('imagePaste', file);
        return;
      }
    }
  }

  const files = event.clipboardData?.files;
  if (files && files.length > 0) {
    for (let i = 0; i < files.length; i++) {
      const file = files[i];
      if (file && file.type.startsWith('image/')) {
        event.preventDefault();
        event.stopPropagation();
        emit('imagePaste', file);
        return;
      }
    }
  }
}

// 拖拽图片上传：与粘贴共用 imagePaste 通道；非图片拖拽（文本/附件）不拦截
function handleDrop(event: DragEvent) {
  const files = Array.from(event.dataTransfer?.files || []);
  const image = files.find((f) => f.type.startsWith('image/'));
  if (!image) return;
  event.preventDefault();
  event.stopPropagation();
  emit('imagePaste', image);
}

// ==================== AI自动补全 ====================
async function triggerAiAutocomplete() {
  if (!aiStore.enabled || !editor || isRunning()) return;
  const model = editor.getModel();
  if (!model) return;

  const position = editor.getPosition();
  if (!position) return;

  const startLine = Math.max(1, position.lineNumber - 50);
  const startCol = startLine === position.lineNumber ? Math.max(1, position.column - 2000) : 1;
  const contextText = model.getValueInRange({
    startLineNumber: startLine,
    startColumn: startCol,
    endLineNumber: position.lineNumber,
    endColumn: position.column,
  });

  if (!contextText.trim()) return;

  const result = await runAI('continue', contextText);
  if (result && editor) {
    const pos = editor.getPosition();
    if (!pos) return;

    aiGhostText.value = result;
    ghostRange = {
      startLineNumber: pos.lineNumber,
      startColumn: pos.column,
      endLineNumber: pos.lineNumber,
      endColumn: pos.column,
    };

    ghostDecorationIds = editor.deltaDecorations(ghostDecorationIds, [{
      range: new monaco.Range(pos.lineNumber, pos.column, pos.lineNumber, pos.column),
      options: {
        after: { content: result, inlineClassName: 'ai-ghost-text' },
        inlineClassName: undefined,
        isWholeLine: false,
      },
    }]);
  }
}

function acceptGhostText() {
  if (!editor || !aiGhostText.value || !ghostRange) return;
  const text = aiGhostText.value;
  editor.executeEdits('ai-ghost-accept', [{
    range: new monaco.Range(
      ghostRange.startLineNumber, ghostRange.startColumn,
      ghostRange.endLineNumber, ghostRange.endColumn,
    ),
    text,
  }]);
  clearGhostText();
  editor.focus();
}

function clearGhostText() {
  if (editor) {
    ghostDecorationIds = editor.deltaDecorations(ghostDecorationIds, []);
  }
  aiGhostText.value = '';
  ghostRange = null;
}

// ==================== Slash AI 命令执行 ====================
async function handleSlashAICommand(action: string) {
  if (!editor) return;
  if (action === 'open') {
    emit('openAiWriter');
    return;
  }
  const model = editor.getModel();
  if (!model) return;
  const selection = editor.getSelection();
  const pos = editor.getPosition();
  const selectedText = selection && !selection.isEmpty() ? model.getValueInRange(selection) : '';

  if (action === 'summarize') {
    const fullText = model.getValue();
    // P1-10: await 前记录代次与版本，回写前校验，防止覆盖等待期间的新输入
    const gen = ++aiWriteGeneration;
    const versionBefore = model.getVersionId();
    const result = await runAI('summarize', fullText);
    if (result && pos && editor && canApplyAiResult(gen, model, versionBefore)) {
      editor.executeEdits('slash-ai-result', [{
        range: new monaco.Range(pos.lineNumber, pos.column, pos.lineNumber, pos.column),
        text: t('docUi1.k66') + result + '\n',
      }]);
    }
    editor?.focus();
    return;
  }

  if (action === 'translate' || action === 'explain' || action === 'rewrite') {
    if (!selection || selection.isEmpty() || !selectedText.trim()) {
      editor.focus();
      return;
    }
    const gen = ++aiWriteGeneration;
    const versionBefore = model.getVersionId();
    const result = await runAI(action as EditorAIAction, selectedText);
    if (result && editor && canApplyAiResult(gen, model, versionBefore)) {
      editor.executeEdits('slash-ai-result', [{ range: selection, text: result }]);
    }
    editor?.focus();
    return;
  }

  if (action === 'report') {
    const title = selectedText || t('fallback.k51');
    const fullText = model.getValue();
    const sources = fullText.length > 100 ? [fullText.substring(0, 10000)] : [];
    const gen = ++aiWriteGeneration;
    const versionBefore = model.getVersionId();
    const result = await runAI('report', title, { reportTitle: title, reportSources: sources.length > 0 ? sources : undefined });
    if (result && pos && editor && canApplyAiResult(gen, model, versionBefore)) {
      editor.executeEdits('slash-ai-result', [{
        range: new monaco.Range(pos.lineNumber, pos.column, pos.lineNumber, pos.column),
        text: '\n' + result + '\n',
      }]);
    }
    editor?.focus();
    return;
  }

  editor.focus();
}

let disposables: (monaco.IDisposable | string | null)[] = [];

// 右键菜单的"点别处收起"走全站唯一实现（原先这里自己挂了一份常驻 mousedown capture）
useClickOutsideClose(() => showContextMenu.value, () => hideContextMenu(), () => [contextMenuRef.value]);
// 右键菜单也要能 Esc 关掉（capture 阶段，先于 monaco 清 ghost text 的 Esc；有真弹窗在上时让位）
useEscapeClose(() => showContextMenu.value, () => hideContextMenu());

onMounted(() => {
  setupMonacoWorkers();
  if (!containerRef.value) return;
  containerRef.value.addEventListener('drop', handleDrop, true);

  monaco.languages.setLanguageConfiguration('markdown', {
    wordPattern: /(-?\d*\.\d\w*)|([^`~!@#$%^&*()\-=+[\]{}\\|;:'",.<>/?\s]+)/g,
    autoClosingPairs: [
      { open: '{', close: '}' },
      { open: '[', close: ']' },
      { open: '(', close: ')' },
      { open: '"', close: '"' },
      { open: "'", close: "'" },
      { open: '`', close: '`' },
      { open: '**', close: '**' },
      { open: '*', close: '*' },
    ],
    surroundingPairs: [
      { open: '{', close: '}' },
      { open: '[', close: ']' },
      { open: '(', close: ')' },
      { open: '"', close: '"' },
      { open: "'", close: "'" },
      { open: '`', close: '`' },
    ],
  });

  // 统一色系：用与预览 --hl-* 同源的调色板注册 apple-light/apple-dark 主题
  defineMonacoThemes(monaco);

  editor = monaco.editor.create(containerRef.value, {
    ...getCommonEditorOptions(),
    value: props.modelValue,
    language: props.language || 'markdown',
    theme: toAppleMonacoTheme(props.theme),
    folding: true,
    selectOnLineNumbers: true,
    roundedSelection: false,
    cursorStyle: 'line' as const,
    cursorBlinking: 'blink' as const,
    smoothScrolling: true,
    contextmenu: false,
    scrollbar: {
      useShadows: false,
      verticalHasArrows: false,
      horizontalHasArrows: false,
      vertical: 'visible' as const,
      horizontal: 'visible' as const,
      verticalScrollbarSize: 8,
      horizontalScrollbarSize: 8,
    },
  });

  const ghostStyle = document.createElement('style');
  ghostStyle.textContent = `.ai-ghost-text { color: color-mix(in srgb, var(--apple-text-tertiary) 50%, transparent); font-style: italic; }`;
  document.head.appendChild(ghostStyle);

  // 回车续列表/引用：Monaco 的 onEnterRules appendText 不支持捕获组插值
  //（无法递增有序列表序号、复制变化的缩进），用手动插入实现
  editor.addCommand(monaco.KeyCode.Enter, () => {
    if (!editor) return;
    const model = editor.getModel();
    const selection = editor.getSelection();
    const pos = editor.getPosition();
    if (!model || !selection || !pos) return;
    const lineContent = model.getLineContent(pos.lineNumber);
    const before = lineContent.slice(0, pos.column - 1);
    const listMatch = before.match(/^(\s*)(?:([-*+])\s+|(\d+)([.)])\s+|(>)\s?)/);
    // 非列表行或存在选区：走默认换行
    if (!listMatch || !selection.isEmpty()) {
      editor.trigger('keyboard', 'type', { text: '\n' });
      return;
    }
    const indent = listMatch[1] || '';
    const bullet = listMatch[2];
    const num = listMatch[3];
    const numDelim = listMatch[4];
    const quote = listMatch[5];
    // 空列表项（只有标记没有内容）：回车退出列表，清掉标记
    const rest = before.slice(listMatch[0].length);
    if (!rest.trim()) {
      editor.executeEdits('list-exit', [
        { range: new monaco.Range(pos.lineNumber, 1, pos.lineNumber, pos.column), text: indent },
      ]);
      return;
    }
    let prefix: string;
    if (num && numDelim) prefix = `${indent}${Number(num) + 1}${numDelim} `;
    else if (quote) prefix = `${indent}> `;
    else prefix = `${indent}${bullet} `;
    editor.executeEdits('list-continue', [{ range: selection, text: `\n${prefix}` }]);
  });

  // Slash `/` 命令：触发判定仅在块级起始位置，避免 URL/路径误触发
  disposables.push(monaco.languages.registerCompletionItemProvider('markdown', {
    triggerCharacters: ['/'],
    provideCompletionItems: (model, position) => {
      const lineContent = model.getLineContent(position.lineNumber);
      const textBefore = lineContent.slice(0, position.column - 1);
      const match = textBefore.match(SLASH_TRIGGER_REGEX);
      if (!match) return { suggestions: [] };

      const typedLen = match[2] ? match[2].length : 0;
      const slashStartCol = Math.max(1, position.column - 1 - typedLen);
      const range = new monaco.Range(position.lineNumber, slashStartCol, position.lineNumber, position.column);

      // AI 未启用时不下发 AI slash 命令（防止点开后发起 AI 请求）
      const aiCommands = aiStore.enabled ? AI_SLASH_COMMANDS.map((cmd) => ({
        label: cmd.label,
        kind: monaco.languages.CompletionItemKind.Function,
        insertText: '',
        detail: t(cmd.detailKey),
        range,
        command: { id: 'ai.slashCommand', title: cmd.label, arguments: [cmd.action] },
      })) : [];

      const contentCommands = CONTENT_SLASH_COMMANDS.map((cmd) => ({
        label: cmd.label,
        kind: monaco.languages.CompletionItemKind.Snippet,
        insertText: getInsertText(cmd.type || ''),
        detail: t(cmd.detailKey),
        range,
      }));

      return { suggestions: [...aiCommands, ...contentCommands] };
    },
  }));

  // `[[` 文档链接补全（无 `/`，不受 slash 触发规则影响）
  disposables.push(monaco.languages.registerCompletionItemProvider('markdown', {
    triggerCharacters: ['['],
    provideCompletionItems: async (model, position) => {
      const lineContent = model.getLineContent(position.lineNumber);
      const textBefore = lineContent.slice(0, position.column - 1);
      const match = textBefore.match(/\[\[([^\]]*)$/);
      if (!match) return { suggestions: [] };

      const query = (match[1] ?? '').trim();
      let docs: { id: number; name: string; space_id: number }[] = [];
      if (props.spaceId) {
        try {
          const res = await searchDocuments(query || ' ', props.spaceId);
          docs = Array.isArray(res) ? res.slice(0, 20) : [];
        } catch { /* ignore */ }
      }
      if (docs.length === 0) return { suggestions: [] };

      const spaceId = props.spaceId;
      return {
        suggestions: docs.map(doc => ({
          label: doc.name,
          kind: monaco.languages.CompletionItemKind.Reference,
          insertText: `[${doc.name}](/space/${spaceId}/doc/${doc.id})`,
          detail: `文档 #${doc.id}`,
          range: {
            startLineNumber: position.lineNumber,
            startColumn: position.column - 2 - query.length,
            endLineNumber: position.lineNumber,
            endColumn: position.column,
          },
        })),
      };
    },
  }));

  // 注册 slash AI 命令（completion 选中后执行）
  disposables.push(
    editor.addAction({
      id: 'ai.slashCommand',
      label: 'AI Slash Command',
      run: (_ed, ...args: unknown[]) => {
        void handleSlashAICommand(String(args[0] ?? ''));
      },
    }),
  );

  document.addEventListener('paste', handleDocumentPaste, true);

  let autoCompleteTimer: ReturnType<typeof setTimeout> | null = null;
  editor.onDidChangeModelContent(() => {
    if (editor) {
      emit('update:modelValue', editor.getValue());
      if (autoCompleteTimer) clearTimeout(autoCompleteTimer);
      if (aiGhostText.value) clearGhostText();
      // 自动续写默认关闭，仅在显式开启时触发
      if (props.enableAutoComplete) {
        autoCompleteTimer = setTimeout(() => {
          triggerAiAutocomplete();
        }, 600);
      }
    }
  });

  editor.onContextMenu((e) => {
    e.event.preventDefault();
    e.event.stopPropagation();
    // posx/posy 是相对编辑器视口的坐标，而右键菜单用 position:fixed 定位到浏览器视口，
    // 必须改用 browserEvent 的 clientX/clientY，否则菜单偏离鼠标甚至跑到可视区外
    const be = e.event.browserEvent ?? (e.event as unknown as { clientX?: number; clientY?: number });
    const x = typeof be.clientX === 'number' ? be.clientX : e.event.posx;
    const y = typeof be.clientY === 'number' ? be.clientY : e.event.posy;
    showContextMenuAt(x, y);
  });

  // Alt+A 手动触发 AI 续写
  disposables.push(
    editor.addCommand(
      monaco.KeyMod.Alt | monaco.KeyCode.KeyA,
      () => { triggerAiAutocomplete(); },
    ),
  );

  // Tab 接受 / Esc 取消 ghost text
  disposables.push(
    editor.addCommand(
      monaco.KeyCode.Tab,
      () => {
        if (aiGhostText.value) {
          acceptGhostText();
          return;
        }
        const suggest = editor?.getContribution('editor.contrib.suggestController') as { model?: { state?: number } } | null;
        if (suggest?.model && suggest.model.state && suggest.model.state > 0) {
          editor?.trigger('keyboard', 'acceptSelectedSuggestion', null);
        } else {
          editor?.trigger('keyboard', 'tab', null);
        }
      },
    ),
  );

  disposables.push(
    editor.addCommand(
      monaco.KeyCode.Escape,
      () => {
        if (aiGhostText.value) {
          clearGhostText();
        }
      },
    ),
  );

  // 工具栏快捷键绑定
  disposables.push(
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyB, () => { wrapSelection('bold'); }),
  );
  disposables.push(
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyI, () => { wrapSelection('italic'); }),
  );
  disposables.push(
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyH, () => { wrapSelection('heading'); }),
  );
  disposables.push(
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyK, () => { wrapSelection('link'); }),
  );
  disposables.push(
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyL, () => { applyMarkdown('list'); }),
  );
  disposables.push(
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyQ, () => { applyMarkdown('quote'); }),
  );
  disposables.push(
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyT, () => { applyMarkdown('table'); }),
  );
  disposables.push(
    editor.addCommand(
      monaco.KeyMod.CtrlCmd | monaco.KeyMod.Shift | monaco.KeyCode.KeyZ,
      () => { editor?.trigger('keyboard', 'redo', null); },
    ),
  );
  disposables.push(
    editor.addCommand(monaco.KeyMod.CtrlCmd | monaco.KeyCode.KeyE, () => { emit('cycleViewMode'); }),
  );
  disposables.push(
    editor.addCommand(
      monaco.KeyMod.CtrlCmd | monaco.KeyMod.Shift | monaco.KeyCode.KeyE,
      () => { emit('toggleSplitMode'); },
    ),
  );
  disposables.push(
    editor.addCommand(
      monaco.KeyMod.CtrlCmd | monaco.KeyCode.Slash,
      () => { emit('showShortcuts'); },
    ),
  );

  // 监听全局主题切换，同步 Monaco 主题。主题现在是 base(data-theme) × 材质(data-material) 两轴，
  // 玻璃有独立的透明编辑器底，所以两个属性都要读、都要观察。
  const syncMonacoTheme = () => {
    const root = document.documentElement;
    monaco.editor.setTheme(
      monacoThemeForUi(root.getAttribute('data-theme'), root.getAttribute('data-material')),
    );
  };
  const themeObserver = new MutationObserver(() => {
    if (!editor) return;
    syncMonacoTheme();
  });
  themeObserver.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme', 'data-material'] });
  syncMonacoTheme();

  (editor as { __themeObserver?: unknown; __ghostStyle?: unknown; __resizeObserver?: unknown }).__themeObserver = themeObserver;
  (editor as { __themeObserver?: unknown; __ghostStyle?: unknown; __resizeObserver?: unknown }).__ghostStyle = ghostStyle;

  const resizeObserver = new ResizeObserver(() => {
    if (editor) {
      editor.layout();
    }
  });
  if (containerRef.value) {
    resizeObserver.observe(containerRef.value);
  }
  (editor as { __themeObserver?: unknown; __ghostStyle?: unknown; __resizeObserver?: ResizeObserver }).__resizeObserver = resizeObserver;
});

onUnmounted(() => {
  document.removeEventListener('paste', handleDocumentPaste, true);
  containerRef.value?.removeEventListener('drop', handleDrop, true);
  disposables.forEach(d => {
    if (d && typeof d === 'object' && typeof d.dispose === 'function') {
      d.dispose();
    }
  });
  disposables = [];
  if (editor) {
    const obs = (editor as { __themeObserver?: { disconnect: () => void } }).__themeObserver;
    if (obs) obs.disconnect();
    const ghostStyle = (editor as { __ghostStyle?: { remove: () => void } }).__ghostStyle;
    if (ghostStyle) ghostStyle.remove();
    const resizeObserver = (editor as { __resizeObserver?: ResizeObserver }).__resizeObserver;
    if (resizeObserver) resizeObserver.disconnect();
  }
  editor?.dispose();
  editor = null;
});

watch(
  () => props.modelValue,
  (val) => {
    if (editor && editor.getValue() !== val) {
      const model = editor.getModel();
      if (model) {
        // 用 executeEdits 替换而非 setValue，保留 undo 栈、避免光标重置
        editor.executeEdits('external-update', [{ range: model.getFullModelRange(), text: val }]);
      }
    }
  }
);


function insertAtCursor(text: string) {
  if (!editor) return;
  const position = editor.getPosition();
  if (!position) return;
  editor.executeEdits('paste-image', [{
    range: new monaco.Range(position.lineNumber, position.column, position.lineNumber, position.column),
    text,
  }]);
  editor.focus();
}

defineExpose({ insertAtCursor, getEditor: () => editor });
</script>

<style scoped>
button:hover {
  background: var(--apple-bg-hover);
}
</style>