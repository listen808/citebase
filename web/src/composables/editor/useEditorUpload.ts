import { MAX_IMAGE_SIZE, MAX_ATTACHMENT_SIZE } from '@/constants/upload-limits';
import { shallowRef } from 'vue';
import { uploadAttachment } from '@/api/modules/document/attachment';
import { useUIStore } from '@/stores/app/ui';
import i18n from '@/i18n';

const { t } = i18n.global;

export function useEditorUpload(documentId: number, insertAtCursor: (text: string) => void) {
  const uploading = shallowRef(false);
  const uploadProgress = shallowRef(0);
  const currentUploadFile = shallowRef('');
  const ui = useUIStore();

  async function uploadSingleImage(file: File): Promise<string> {
    if (file.size > MAX_IMAGE_SIZE) {
      ui.showToast(t('sentUi.k71', { p1: file.name }), 'error');
      return '';
    }
    currentUploadFile.value = file.name;
    uploadProgress.value = 0;
    const result = await uploadAttachment(documentId, file, (progress) => {
      uploadProgress.value = progress;
    });
    const markdown = `![${file.name}](${result.url})`;
    insertAtCursor('\n' + markdown + '\n');
    uploadProgress.value = 100;
    return result.url;
  }

  async function uploadSingleAttachment(file: File): Promise<void> {
    if (file.size > MAX_ATTACHMENT_SIZE) {
      ui.showToast(t('sentUi.k72', { p1: file.name }), 'error');
      return;
    }
    currentUploadFile.value = file.name;
    uploadProgress.value = 0;
    const result = await uploadAttachment(documentId, file, (progress) => {
      uploadProgress.value = progress;
    });
    const isImage = file.type.startsWith('image/');
    const markdown = isImage
      ? `![${file.name}](${result.url})`
      : `[${file.name}](${result.url})`;
    insertAtCursor('\n' + markdown + '\n');
    uploadProgress.value = 100;
  }

  async function handleImageSelected(event: Event) {
    const input = event.target as HTMLInputElement;
    if (!input.files?.length) return;
    uploading.value = true;
    try {
      await Promise.all(Array.from(input.files).map((file) => uploadSingleImage(file)));
    } finally {
      uploading.value = false;
      input.value = '';
    }
  }

  async function handleAttachmentSelected(event: Event) {
    const input = event.target as HTMLInputElement;
    if (!input.files?.length) return;
    uploading.value = true;
    try {
      await Promise.all(Array.from(input.files).map((file) => uploadSingleAttachment(file)));
    } finally {
      uploading.value = false;
      input.value = '';
    }
  }

  async function handleImagePaste(file: File) {
    uploading.value = true;
    try {
      await uploadSingleImage(file);
    } finally {
      uploading.value = false;
    }
  }

  async function processImageFile(file: File): Promise<boolean> {
    if (!file.type.startsWith('image/')) return false;
    uploading.value = true;
    try {
      await uploadSingleImage(file);
    } finally {
      uploading.value = false;
    }
    return true;
  }

  async function handleParentPaste(event: ClipboardEvent) {
    const items = event.clipboardData?.items;
    if (items && items.length > 0) {
      for (let i = 0; i < items.length; i++) {
        const item = items[i];
        if (item && item.type.startsWith('image/')) {
          event.preventDefault();
          event.stopPropagation();
          const file = item.getAsFile();
          if (file) {
            await processImageFile(file);
          }
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
          await processImageFile(file);
          return;
        }
      }
    }
  }

  return {
    uploading,
    uploadProgress,
    currentUploadFile,
    handleImageSelected,
    handleAttachmentSelected,
    handleImagePaste,
    handleParentPaste,
  };
}
