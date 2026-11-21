import { ref, shallowRef } from 'vue';
import { useUIStore } from '@/stores/app/ui';
import { uploadAttachment, getDocumentImages, getDocumentAttachments, type Attachment } from '@/api/modules/document/attachment';
import i18n from '@/i18n';

const { t } = i18n.global;

export { formatSize, resolveFileUrl, getDownloadUrl } from '@/utils/domain/file';

export function useAttachmentUpload(
  documentId: () => number,
  type: () => 'images' | 'attachments',
  isEditor: () => boolean,
  emit: {
    (e: 'insert-image', url: string): void;
    (e: 'insert-attachment', url: string, name: string): void;
  }
) {
  const ui = useUIStore();

  const attachments = ref<Attachment[]>([]);
  const loading = shallowRef(false);
  const uploadProgress = shallowRef(0);
  const isDragging = shallowRef(false);
  const fileInputRef = ref<HTMLInputElement | null>(null);

  async function load() {
    const docId = documentId();
    if (!docId) return;
    loading.value = true;
    try {
      if (type() === 'images') {
        const images = await getDocumentImages(docId);
        attachments.value = images;
      } else {
        const allAttachments = await getDocumentAttachments(docId, 0);
        attachments.value = allAttachments;
      }
    } catch {
      ui.showToast(type() === 'images' ? '加载图片失败' : '加载附件失败', 'error');
    } finally {
      loading.value = false;
    }
  }

  function triggerUpload() {
    fileInputRef.value?.click();
  }

  function handleDragOver(e: DragEvent) {
    e.preventDefault();
  }

  function handleDragEnter(e: DragEvent) {
    e.preventDefault();
    isDragging.value = true;
  }

  function handleDragLeave(e: DragEvent) {
    e.preventDefault();
    isDragging.value = false;
  }

  async function handleDrop(e: DragEvent) {
    e.preventDefault();
    isDragging.value = false;

    const files = Array.from(e.dataTransfer?.files || []);
    if (files.length === 0) return;

    const validFiles = files.filter(file => {
      if (type() === 'images') {
        return file.type.startsWith('image/');
      }
      return true;
    });

    if (validFiles.length === 0) {
      ui.showToast(type() === 'images' ? '请拖拽图片文件' : '请拖拽有效文件', 'warning');
      return;
    }

    for (const file of validFiles) {
      await uploadFile(file);
    }
  }

  async function handleFileSelect(event: Event) {
    const target = event.target as HTMLInputElement;
    const files = Array.from(target.files || []);
    if (files.length === 0) return;

    const validFiles = files.filter(file => {
      if (type() === 'images') {
        return file.type.startsWith('image/');
      }
      return true;
    });

    if (validFiles.length === 0) {
      ui.showToast(type() === 'images' ? '请选择图片文件' : '请选择有效文件', 'warning');
      return;
    }

    for (const file of validFiles) {
      await uploadFile(file);
    }

    if (fileInputRef.value) {
      fileInputRef.value.value = '';
    }
  }

  async function uploadFile(file: File) {
    const docId = documentId();
    if (!docId) return;
    uploadProgress.value = 0;

    try {
      const result = await uploadAttachment(docId, file, (progress) => {
        uploadProgress.value = progress;
      });
      if (type() === 'images' && isEditor()) {
        emit('insert-image', result.url);
      } else if (type() === 'attachments' && isEditor()) {
        emit('insert-attachment', result.url, result.name);
      }
      ui.showToast(type() === 'images' ? '图片上传成功' : '附件上传成功', 'success');
      await load();
    } catch {
      ui.showToast(t('tsUi.k8'), 'error');
    } finally {
      uploadProgress.value = 0;
    }
  }

  async function deleteAttachmentById(id: number) {
    try {
      const { deleteAttachment } = await import('@/api/modules/document/attachment');
      await deleteAttachment(id);
      ui.showToast(t('tsUi.k9'), 'success');
      await load();
    } catch {
      ui.showToast(t('tsUi.k10'), 'error');
    }
  }

  return {
    attachments,
    loading,
    uploadProgress,
    isDragging,
    fileInputRef,
    load,
    triggerUpload,
    handleDragOver,
    handleDragEnter,
    handleDragLeave,
    handleDrop,
    handleFileSelect,
    deleteAttachmentById,
  };
}
