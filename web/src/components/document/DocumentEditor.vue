<template>
  <div :style="editorContainerStyle">
    <EditorToolbar
      :view-mode="viewMode"
      :view-mode-label="viewModeLabel"

      :uploading="uploading"
      @save="emit('save')"
      @cancel="emit('cancel')"
      @show-versions="showVersions = true"
      @show-shortcuts="showShortcuts = true"
      @toggle-view-mode="cycleViewMode"
      @insert-markdown="insertMarkdown"
      @trigger-image-upload="imageInputRef?.click()"
      @trigger-attachment-upload="attachmentInputRef?.click()"
      @ai-write="handleAIWrite"
    />

    <input ref="imageInputRef" type="file" :aria-label="$t('docUi1.k7')" accept="image/*" multiple :style="{ display: 'none' }" @change="handleImageSelected" />
    <input ref="attachmentInputRef" type="file" :aria-label="$t('docUi1.k8')" multiple :style="{ display: 'none' }" @change="handleAttachmentSelected" />

    <div :style="splitContainerStyle">
      <div v-show="viewMode !== 'preview'" :style="editorPaneStyle">
        <MonacoEditor
          ref="monacoRef"
          :model-value="modelValue"
          language="markdown"
          theme="vs"
          :space-id="spaceId"
          :enable-auto-complete="aiStore.enabled"
          @update:model-value="emit('update:modelValue', $event)"
          @image-paste="handleImagePaste"
          @cycle-view-mode="cycleViewMode"
          @toggle-split-mode="toggleSplitMode"
          @show-shortcuts="showShortcuts = true"
          @open-ai-writer="showAIWriter = true"
        />
      </div>

      <div v-show="viewMode === 'split'" :style="resizeHandleStyle" @mousedown="startResize" @wheel="handleResizeWheel" />

      <div v-show="viewMode !== 'edit'" :style="previewPaneStyle">
        <div ref="previewRef" :style="previewContentStyle">
          <ApMarkdownPreview :content="modelValue" />
        </div>
      </div>
    </div>

    <ApModal v-model="showVersions" :title="$t('docUi1.k5')" size="large">
      <VersionHistory :document-id="documentId" @rollback="handleRollback" />
    </ApModal>

    <ApModal v-model="showTableModal" :title="$t('docUi1.k6')" size="small" density="compact">
      <div :style="tableModalStyle">
        <div :style="tableFieldStyle">
          <label :style="tableLabelStyle">{{ $t('docUi1.k1') }}</label>
          <ApInput
            type="number"
            :model-value="String(tableRows)"
            :min="1"
            :max="20"
            :style="tableInputStyle"
            @update:model-value="(v: string) => tableRows = parseInt(v) || 1"
          />
        </div>
        <div :style="tableFieldStyle">
          <label :style="tableLabelStyle">{{ $t('docUi1.k2') }}</label>
          <ApInput
            type="number"
            :model-value="String(tableCols)"
            :min="1"
            :max="10"
            :style="tableInputStyle"
            @update:model-value="(v: string) => tableCols = parseInt(v) || 1"
          />
        </div>
        <div :style="tableActionsStyle">
          <ApButton variant="ghost" size="small" @click="showTableModal = false">{{ $t('docUi1.cancel') }}</ApButton>
          <ApButton variant="primary" size="small" @click="insertTable">{{ $t('docUi1.k4') }}</ApButton>
        </div>
      </div>
    </ApModal>

    <AIWriterPanel
      v-if="aiStore.enabled"
      v-model="showAIWriter"
      :document-id="documentId"
      :space-id="spaceId"
      @insert-text="handleInsertText"
      @replace-selection="handleReplaceSelection"
    />

    <ShortcutsModal v-model="showShortcuts" />
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, nextTick, watch, onBeforeUnmount } from 'vue';
import type { editor } from 'monaco-editor';
import { AppleFont, AppleSpace, AppleTransition, AppleType, cv } from '@/utils/core/apple-tokens';
import ApModal from '@/components/apple/ApModal.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApMarkdownPreview from '@/components/apple/ApMarkdownPreview.vue';
import MonacoEditor from './MonacoEditor.vue';
import VersionHistory from './VersionHistory.vue';
import EditorToolbar from './EditorToolbar.vue';
import ShortcutsModal from './ShortcutsModal.vue';
import AIWriterPanel from '@/components/ai/AIWriterPanel.vue';
import { useEditorUpload } from '@/composables/editor/useEditorUpload';
import { useMarkdownInsert } from '@/composables/editor/useMarkdownInsert';
import { useAIWrite } from '@/composables/ai/useAIWrite';
import { useScrollSync } from '@/composables/ui/useScrollSync';
import { useUIStore } from '@/stores/app/ui';
import { useAIStore } from '@/stores/ai/ai';
import { rowStyle } from '@/styles/layout/page-styles';

const props = defineProps<{
  modelValue: string;
  documentId: number;
  spaceId?: number;
  saveStatus?: string;
  hasUnsaved?: boolean;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', value: string): void;
  (e: 'save'): void;
  (e: 'rollback', version: number): void;
  (e: 'cancel'): void;
  (e: 'image-paste', file: File): void;
}>();
const { t } = useI18n();

const aiStore = useAIStore();

type ViewMode = 'edit' | 'preview' | 'split';

const viewMode = ref<ViewMode>('split');
const showVersions = shallowRef(false);
const showShortcuts = shallowRef(false);
const showTableModal = shallowRef(false);

const tableRows = ref(3);
const tableCols = ref(3);
const previewRef = ref<HTMLElement | null>(null);
const monacoEditorRef = ref<editor.IStandaloneCodeEditor | null>(null);
const splitRatio = shallowRef(0.5);
const imageInputRef = ref<HTMLInputElement | null>(null);
const attachmentInputRef = ref<HTMLInputElement | null>(null);
const monacoRef = ref<InstanceType<typeof MonacoEditor> | null>(null);



// 当 Monaco 组件挂载后，获取 editor 实例
watch(monacoRef, (instance) => {
  if (instance) {
    monacoEditorRef.value = instance.getEditor() ?? null;
  }
}, { immediate: true });

// 分栏切换时重新绑定编辑器实例（切换显示/隐藏后确保关联有效）
watch(viewMode, async (newMode) => {
  if (newMode === 'split') {
    await nextTick();
    const instance = monacoRef.value;
    if (instance) {
      monacoEditorRef.value = instance.getEditor() ?? null;
    }
  }
});

useScrollSync(monacoEditorRef, previewRef);

const viewModeLabel = computed(() => {
  switch (viewMode.value) {
    case 'edit': return '仅编辑';
    case 'preview': return '仅预览';
    case 'split': return '分栏';
    default: return '分栏';
  }
});


const { uploading, handleImageSelected, handleAttachmentSelected, handleImagePaste } = useEditorUpload(
  props.documentId,
  (text: string) => monacoRef.value?.insertAtCursor(text),
);

const { applyMarkdown, buildTableMarkdown } = useMarkdownInsert(() => monacoRef.value?.getEditor() ?? null);
const { run: runAIWrite, running: aiWriteLoading } = useAIWrite();

const ui = useUIStore();
const showAIWriter = ref(false);

function cycleViewMode() {
  viewMode.value = viewMode.value === 'edit' ? 'preview' : viewMode.value === 'preview' ? 'split' : 'edit';
}

function toggleSplitMode() {
  viewMode.value = viewMode.value === 'split' ? 'edit' : 'split';
}

async function handleAIWrite(action: string) {
  if (action === 'open') {
    showAIWriter.value = true;
    return;
  }

  if (aiWriteLoading.value) return;
  const ed = monacoRef.value?.getEditor();
  if (!ed) return;
  const selection = ed.getSelection();
  const model = ed.getModel();
  if (!model || !selection) return;
  const selectedText = model.getValueInRange(selection) || '';
  const text = selectedText || props.modelValue;
  if (!text.trim()) {
    ui.showToast(t('docUi1.k9'), 'warning');
    return;
  }
  // P1-9: await 前的 selection 只是坐标快照，请求期间文档被继续编辑（或整体替换）时
  // 旧 range 指向已不是原内容；回写前校验 model 未被更换、版本未变化，否则丢弃并提示，
  // 绝不静默覆盖用户新输入
  const versionBefore = model.getVersionId();
  const result = await runAIWrite(action as never, text);
  if (!result) return;
  if (ed.getModel() !== model) {
    ui.showToast(t('docUi1.k10'), 'warning');
    return;
  }
  if (model.getVersionId() !== versionBefore) {
    ui.showToast(t('docUi1.k11'), 'warning');
    return;
  }
  const insertText = selectedText ? result : '\n\n' + result;
  ed.executeEdits('ai-write', [{ range: selection, text: insertText }]);
  ed.focus();
  ui.showToast(t('docUi1.k12'), 'success');
}

function handleInsertText(text: string) {
  if (!monacoRef.value) return;
  const editor = monacoRef.value.getEditor();
  if (!editor) return;
  const position = editor.getPosition();
  if (position) {
    editor.executeEdits('ai-insert', [{
      range: {
        startLineNumber: position.lineNumber,
        startColumn: position.column,
        endLineNumber: position.lineNumber,
        endColumn: position.column
      },
      text: text
    }]);
  }
}

function handleReplaceSelection(text: string) {
  if (!monacoRef.value) return;
  const editor = monacoRef.value.getEditor();
  if (!editor) return;
  const selection = editor.getSelection();
  if (selection) {
    editor.executeEdits('ai-replace', [{
      range: selection,
      text: text
    }]);
  }
}

function insertMarkdown(type: string) {
  if (type === 'table') {
    showTableModal.value = true;
    return;
  }
  applyMarkdown(type);
}

function handleRollback(version: number) {
  emit('rollback', version);
  showVersions.value = false;
}

function insertTable() {
  const tableText = buildTableMarkdown(tableRows.value, tableCols.value);
  monacoRef.value?.insertAtCursor(tableText);
  showTableModal.value = false;
}

let resizeListeners: { mousemove: (e: MouseEvent) => void; mouseup: () => void; mouseleave: () => void } | null = null;

function startResize(e: MouseEvent) {
  const container = (e.target as HTMLElement).parentElement;
  if (!container) return;
  const startX = e.clientX;
  const startWidth = container.clientWidth;

  function onMouseMove(ev: MouseEvent) {
    const delta = ev.clientX - startX;
    splitRatio.value = Math.max(0.2, Math.min(0.8, (startWidth * splitRatio.value + delta) / startWidth));
  }

  function stopResize() {
    if (resizeListeners) {
      document.removeEventListener('mousemove', resizeListeners.mousemove);
      document.removeEventListener('mouseup', resizeListeners.mouseup);
      document.removeEventListener('mouseleave', resizeListeners.mouseleave);
      resizeListeners = null;
    }
  }

  resizeListeners = { mousemove: onMouseMove, mouseup: stopResize, mouseleave: stopResize };
  document.addEventListener('mousemove', onMouseMove);
  document.addEventListener('mouseup', stopResize);
  document.addEventListener('mouseleave', stopResize);
}

onBeforeUnmount(() => {
  cleanupResizeListeners();
});

function cleanupResizeListeners() {
  if (resizeListeners) {
    document.removeEventListener('mousemove', resizeListeners.mousemove);
    document.removeEventListener('mouseup', resizeListeners.mouseup);
    document.removeEventListener('mouseleave', resizeListeners.mouseleave);
    resizeListeners = null;
  }
}

// 在分割线（resize handle）上滚动时，同步滚动左右两屏
function handleResizeWheel(e: WheelEvent) {
  e.preventDefault();
  const editor = monacoEditorRef.value;
  const preview = previewRef.value;
  if (!editor || !preview) return;

  // 同步滚动 Monaco 编辑器
  const editorScrollHeight = editor.getScrollHeight();
  const editorClientHeight = editor.getLayoutInfo().height;
  const currentEditorScroll = editor.getScrollTop();
  const maxEditorScroll = Math.max(0, editorScrollHeight - editorClientHeight);
  const newEditorScroll = Math.max(0, Math.min(maxEditorScroll, currentEditorScroll + e.deltaY));
  editor.setScrollTop(newEditorScroll);

  // 同步滚动预览面板
  const maxPreviewScroll = Math.max(0, preview.scrollHeight - preview.clientHeight);
  const newPreviewScroll = Math.max(0, Math.min(maxPreviewScroll, preview.scrollTop + e.deltaY));
  preview.scrollTop = newPreviewScroll;
}

const editorContainerStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  flex: '1',
  minHeight: '0',
  overflow: 'hidden' as const,
  background: cv('surface-panel') };

const splitContainerStyle = computed(() => ({
  display: 'flex',
  flex: '1',
  overflow: 'hidden',
  position: 'relative' as const,
  minHeight: '0' }));

const editorPaneStyle = computed(() => ({
  width: viewMode.value === 'split' ? `${splitRatio.value * 100}%` : '100%',
  minWidth: viewMode.value === 'split' ? '200px' : '0',
  overflow: 'hidden',
  display: viewMode.value === 'preview' ? 'none' : 'flex',
  flexDirection: 'column' as const,
  minHeight: '0', }));

const resizeHandleStyle = {
  width: '5px',
  cursor: 'col-resize',
  flexShrink: 0,
  transition: AppleTransition.fast,
  minHeight: '100%',
  backgroundColor: cv('border-light'),
};

const previewPaneStyle = computed(() => ({
  display: 'flex',
  flexDirection: 'column' as const,
  flex: viewMode.value === 'split' ? '1' : '1',
  width: viewMode.value === 'split' ? `${(1 - splitRatio.value) * 100}%` : '100%',
  background: cv('surface-panel'),
  borderLeft: `1px solid ${cv('border')}` }));

const previewContentStyle = {
  flex: '1',
  overflow: 'auto',
  background: cv('surface-panel') };

const tableModalStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.md,
  padding: AppleSpace.md,
};

const tableFieldStyle = rowStyle(AppleSpace.md);

const tableLabelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-primary'),
  minWidth: '40px',
};

const tableInputStyle = {
  width: '80px',
};

const tableActionsStyle = {
  display: 'flex',
  justifyContent: 'flex-end' as const,
  gap: AppleSpace.sm,
  marginTop: AppleSpace.sm,
};
</script>
