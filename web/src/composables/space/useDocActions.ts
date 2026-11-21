import { ref, shallowRef, type ShallowRef } from 'vue';
import { useUIStore } from '@/stores/app/ui';
import { copyText } from '@/utils/clipboard';
import { toggleCollection, checkCollected } from '@/api/modules/space/collection';
import { exportDocument, exportDocumentHTML, exportDocumentWord, exportDocumentPDF, exportDocumentMarkdown } from '@/api/modules/document/document';
import { getDocumentHistory } from '@/api/modules/system/log';
import type { DocumentHistory } from '@/api/modules/system/log';
import i18n from '@/i18n';

const { t } = i18n.global;

async function downloadBlob(
  apiFn: () => Promise<unknown>,
  filename: string,
  label: string,
  loading: ShallowRef<boolean>,
  showToast: (msg: string, type?: 'success' | 'error' | 'warning' | 'info') => void,
) {
  if (loading.value) return;
  loading.value = true;
  try {
    const blob = await apiFn();
    const url = window.URL.createObjectURL(blob as Blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = filename;
    a.click();
    window.URL.revokeObjectURL(url);
    showToast(t('sentUi.k76', { p1: label }), 'success');
  } catch {
    showToast(t('sentUi.k77', { p1: label }), 'error');
  } finally {
    loading.value = false;
  }
}

export function useDocActions() {
  const ui = useUIStore();

  const isCollected = shallowRef(false);
  const collectLoading = shallowRef(false);
  const shareUrl = shallowRef('');
  const docHistoryList = ref<DocumentHistory[]>([]);
  const docHistoryLoading = shallowRef(false);
  const exportLoading = shallowRef(false);

  async function checkCollectStatus(targetType: number, targetId: number) {
    try {
      const res = await checkCollected(targetType, targetId);
      isCollected.value = res.collected;
    } catch {
      isCollected.value = false;
    }
  }

  async function handleCollect(targetType: number, targetId: number) {
    if (collectLoading.value) return;
    collectLoading.value = true;
    const previousState = isCollected.value;
    isCollected.value = !isCollected.value;
    try {
      await toggleCollection(targetType, targetId);
      ui.showToast(isCollected.value ? '收藏成功' : '取消收藏', 'success');
    } catch {
      isCollected.value = previousState;
      ui.showToast(t('tsUi.k20'), 'error');
    } finally {
      collectLoading.value = false;
    }
  }

  function handleExport(docId: number, docName: string) {
    const ts = new Date().toISOString().replace(/[:.]/g, '-').slice(0, -5);
    return downloadBlob(() => exportDocument(docId), `${docName}_${ts}.zip`, '', exportLoading, ui.showToast);
  }

  function handleExportHTML(docId: number, docName: string) {
    return downloadBlob(() => exportDocumentHTML(docId), `${docName}.html`, 'HTML', exportLoading, ui.showToast);
  }

  function handleExportWord(docId: number, docName: string) {
    return downloadBlob(() => exportDocumentWord(docId), `${docName}.docx`, 'Word', exportLoading, ui.showToast);
  }

  function handleExportPDF(docId: number, docName: string) {
    return downloadBlob(() => exportDocumentPDF(docId), `${docName}.pdf`, 'PDF', exportLoading, ui.showToast);
  }

  function handleExportMarkdown(docId: number, docName: string) {
    return downloadBlob(() => exportDocumentMarkdown(docId), `${docName}.md`, 'Markdown', exportLoading, ui.showToast);
  }

  function buildShareUrl(_spaceId: number, docId: number) {
    shareUrl.value = `${window.location.origin}/share/${docId}`;
  }

  function copyShareUrl() {
    copyText(shareUrl.value).then((ok) =>
      ui.showToast(ok ? t('tsUi.k21') : t('tsUi.k22'), ok ? 'success' : 'error')
    );
  }

  async function loadHistory(docId: number) {
    docHistoryLoading.value = true;
    try {
      docHistoryList.value = await getDocumentHistory(docId, 20);
    } catch {
      ui.showToast(t('tsUi.k23'), 'error');
    } finally {
      docHistoryLoading.value = false;
    }
  }

  return {
    isCollected,
    collectLoading,
    shareUrl,
    docHistoryList,
    docHistoryLoading,
    exportLoading,
    checkCollectStatus,
    handleCollect,
    handleExport,
    handleExportHTML,
    handleExportWord,
    handleExportPDF,
    handleExportMarkdown,
    buildShareUrl,
    copyShareUrl,
    loadHistory,
  };
}
