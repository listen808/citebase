<template>
  <div ref="containerRef" :style="containerStyle" />
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted, watch, nextTick } from 'vue';
import * as monaco from 'monaco-editor';
import { setupMonacoWorkers } from '@/utils/domain/monaco-setup';
import { getCommonEditorOptions } from '@/utils/domain/monaco-config';
import { defineMonacoThemes, toAppleMonacoTheme } from '@/styles/theme/highlight-tokens';

const props = defineProps<{
  original: string;
  modified: string;
  language?: string;
  theme?: string;
}>();

const containerRef = ref<HTMLElement | null>(null);
let diffEditor: monaco.editor.IStandaloneDiffEditor | null = null;
let originalModel: monaco.editor.ITextModel | null = null;
let modifiedModel: monaco.editor.ITextModel | null = null;

const containerStyle = {
  width: '100%',
  overflow: 'hidden',
};

onMounted(() => {
  setupMonacoWorkers();
  if (!containerRef.value) return;

  defineMonacoThemes(monaco);
  diffEditor = monaco.editor.createDiffEditor(containerRef.value, {
    ...getCommonEditorOptions(),
    theme: toAppleMonacoTheme(props.theme),
    folding: true,
    selectOnLineNumbers: true,
    roundedSelection: false,
    cursorBlinking: 'blink',
    smoothScrolling: true,
    renderSideBySide: true,
    renderIndicators: true,
    originalEditable: false,
    readOnly: true,
    scrollbar: {
      useShadows: false,
      verticalHasArrows: false,
      horizontalHasArrows: false,
      vertical: 'visible',
      horizontal: 'auto',
      verticalScrollbarSize: 8,
      horizontalScrollbarSize: 8,
    },
  });

  originalModel = monaco.editor.createModel(
    props.original,
    props.language || 'markdown'
  );
  modifiedModel = monaco.editor.createModel(
    props.modified,
    props.language || 'markdown'
  );

  diffEditor.setModel({ original: originalModel, modified: modifiedModel });

  nextTick(() => {
    setupScrollSync();
  });
});

function setupScrollSync() {
  if (!diffEditor) return;

  const originalEditor = diffEditor.getOriginalEditor();
  const modifiedEditor = diffEditor.getModifiedEditor();

  if (originalEditor && modifiedEditor) {
    let syncLock = false;

    originalEditor.onDidScrollChange((e) => {
      if (syncLock) return;
      syncLock = true;
      try {
        modifiedEditor.setScrollTop(e.scrollTop);
        modifiedEditor.setScrollLeft(e.scrollLeft);
      } finally {
        setTimeout(() => syncLock = false, 0);
      }
    });

    modifiedEditor.onDidScrollChange((e) => {
      if (syncLock) return;
      syncLock = true;
      try {
        originalEditor.setScrollTop(e.scrollTop);
        originalEditor.setScrollLeft(e.scrollLeft);
      } finally {
        setTimeout(() => syncLock = false, 0);
      }
    });
  }
}

onUnmounted(() => {
  diffEditor?.dispose();
  diffEditor = null;
  originalModel?.dispose();
  originalModel = null;
  modifiedModel?.dispose();
  modifiedModel = null;
});

watch(
  () => props.original,
  (val) => {
    const model = diffEditor?.getModel();
    if (model?.original) {
      model.original.setValue(val);
    }
  }
);

watch(
  () => props.modified,
  (val) => {
    const model = diffEditor?.getModel();
    if (model?.modified) {
      model.modified.setValue(val);
    }
  }
);
</script>
