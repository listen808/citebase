import { watch, onUnmounted, type Ref } from 'vue';
import type { editor as EditorType } from 'monaco-editor';

// Monaco scroll event 所需字段（避免直接引用可能不存在的 IScrollEvent）
interface MonacoScrollEvent {
  scrollTop: number;
  scrollLeft: number;
  scrollHeight: number;
  scrollWidth: number;
}

export function useScrollSync(
  monacoEditor: Ref<EditorType.IStandaloneCodeEditor | null>,
  previewEl: Ref<HTMLElement | null>
) {
  let isMonacoScrolling = false;
  let isPreviewScrolling = false;
  let monacoDisposable: { dispose: () => void } | null = null;

  // Monaco 滚动 → 同步预览
  function handleMonacoScroll(e: MonacoScrollEvent) {
    const preview = previewEl.value;
    if (!preview || isPreviewScrolling) return;
    isMonacoScrolling = true;

    const editor = monacoEditor.value;
    if (!editor) return;
    const scrollHeight = editor.getScrollHeight();
    const clientHeight = editor.getLayoutInfo().height;
    const ratio = e.scrollTop / (scrollHeight - clientHeight || 1);
    preview.scrollTop = ratio * (preview.scrollHeight - preview.clientHeight);

    requestAnimationFrame(() => { isMonacoScrolling = false; });
  }

  // 预览滚动 → 同步 Monaco
  function handlePreviewScroll() {
    const editor = monacoEditor.value;
    if (!editor || isMonacoScrolling) return;
    isPreviewScrolling = true;

    const preview = previewEl.value;
    if (!preview) return;
    const scrollHeight = editor.getScrollHeight();
    const clientHeight = editor.getLayoutInfo().height;
    const ratio = preview.scrollTop / (preview.scrollHeight - preview.clientHeight || 1);
    editor.setScrollTop(ratio * (scrollHeight - clientHeight));

    requestAnimationFrame(() => { isPreviewScrolling = false; });
  }

  function unbind() {
    monacoDisposable?.dispose();
    monacoDisposable = null;
    previewEl.value?.removeEventListener('scroll', handlePreviewScroll);
  }

  function bind() {
    const editor = monacoEditor.value;
    const preview = previewEl.value;

    if (editor) {
      monacoDisposable = editor.onDidScrollChange(handleMonacoScroll as unknown as (e: unknown) => void);
    }
    if (preview) {
      preview.addEventListener('scroll', handlePreviewScroll, { passive: true });
    }
  }

  watch([monacoEditor, previewEl], () => {
    unbind();
    bind();
  }, { immediate: true });

  onUnmounted(() => {
    unbind();
  });
}
