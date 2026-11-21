import { shallowRef } from 'vue';
import { useDocumentStore } from '@/stores/workspace/document';
import { useUIStore } from '@/stores/app/ui';
import { useCrudModal } from '@/composables/shared/useCrudModal';
import i18n from '@/i18n';

const { t } = i18n.global;

interface CreateDocForm {
  [key: string]: unknown;
  name: string;
  parentId: number;
  spaceId: number;
  type: 1 | 2;
}

interface RenameDocForm {
  [key: string]: unknown;
  name: string;
  docId: number | null;
}

interface DeleteDocForm {
  [key: string]: unknown;
  docId: number | null;
  message: string;
  docName: string;
}

export function useDocCRUD() {
  const docStore = useDocumentStore();
  const ui = useUIStore();

  const createModal = useCrudModal<CreateDocForm>({
    name: '',
    parentId: 0,
    spaceId: 0,
    type: 1,
  });

  const renameModal = useCrudModal<RenameDocForm>({
    name: '',
    docId: null,
  });

  const deleteModal = useCrudModal<DeleteDocForm>({
    docId: null,
    message: '',
    docName: '',
  });

  const deleting = shallowRef(false);
  const moving = shallowRef(false);
  const copying = shallowRef(false);

  const showMoveModal = shallowRef(false);
  const movingDocId = shallowRef<number | null>(null);

  const showCopyModal = shallowRef(false);
  const copyingDocId = shallowRef<number | null>(null);

  const showCreateModal = createModal.show;
  const createDocName = shallowRef('');
  const createDocParentId = shallowRef(0);
  const createDocSpaceId = shallowRef(0);
  const createDocType = shallowRef<1 | 2>(1);
  const creatingDoc = createModal.submitting;

  const showRenameModal = renameModal.show;
  const renameDocName = shallowRef('');
  const renamingDoc = renameModal.submitting;
  const renamingDocId = shallowRef<number | null>(null);

  const showDeleteModal = deleteModal.show;
  const deletingDocId = shallowRef<number | null>(null);
  const deleteMessage = shallowRef('');
  const deletingDocName = shallowRef('');

  function openCreateModal(parentId: number, sId: number, type: 1 | 2 = 1, currentSpaceId?: number) {
    createDocName.value = '';
    createDocParentId.value = parentId;
    createDocSpaceId.value = sId || currentSpaceId || 0;
    createDocType.value = type;
    createModal.openCreate();
  }

  async function handleCreateDoc(onSuccess?: (doc: { id: number }) => void, templateContent?: string) {
    if (creatingDoc.value) return;
    // 原先空名与"正在提交"混在一个条件里静默 return：弹窗里按回车什么也不会发生，
    // 用户只能猜。重命名那条（handleRenameDoc）也是 return，但它的按钮按名字是否为空置灰，
    // 这里同理 —— 按钮灰掉时看 tooltip，回车时看这条提示。
    if (!createDocName.value.trim()) {
      ui.showToast(t('tsUi.k24'), 'warning');
      return;
    }
    createModal.submitting.value = true;
    try {
      const doc = await docStore.create({
        space_id: createDocSpaceId.value,
        parent_id: createDocParentId.value,
        name: createDocName.value.trim(),
        type: createDocType.value,
        content: templateContent || '',
      });
      createModal.close();
      ui.showToast(t('tsUi.k25'));
      if (doc && onSuccess) onSuccess(doc);
    } catch {
      ui.showToast(t('tsUi.k26'));
    } finally {
      createModal.submitting.value = false;
    }
  }

  function handleRename(doc: { id: number; name: string }) {
    renamingDocId.value = doc.id;
    renameDocName.value = doc.name;
    renameModal.openCreate();
  }

  async function handleRenameDoc(onSuccess?: (id: number, newName: string) => void) {
    if (!renamingDocId.value || !renameDocName.value.trim()) return;
    renameModal.submitting.value = true;
    try {
      await docStore.update(renamingDocId.value, { name: renameDocName.value.trim() });
      ui.showToast(t('tsUi.k27'), 'success');
      if (onSuccess) onSuccess(renamingDocId.value, renameDocName.value.trim());
      renameModal.close();
    } catch {
      ui.showToast(t('tsUi.k28'), 'error');
    } finally {
      renameModal.submitting.value = false;
    }
  }

  function handleDelete(doc: { id: number; name: string; type?: number }) {
    deletingDocId.value = doc.id;
    deletingDocName.value = doc.name;
    const isDir = doc.type === 2;
    const base = `确定删除${isDir ? '目录' : '文档'}「${doc.name}」吗？`;
    const suffix = isDir ? '目录及其下所有子文档将移入回收站，可在回收站中恢复。' : '文档将移入回收站，可在回收站中恢复。';
    deleteMessage.value = `${base}${suffix}`;
    deleteModal.openCreate();
  }

  async function confirmDelete(onDeleted?: () => void) {
    if (!deletingDocId.value || deleting.value) return;
    deleting.value = true;
    const spaceId = docStore.currentSpaceId || 0;
    try {
      await docStore.remove(deletingDocId.value, spaceId);
      ui.showToast(t('tsUi.k19'), 'success');
      if (onDeleted) onDeleted();
    } catch {
      ui.showToast(t('tsUi.k10'), 'error');
    } finally {
      deleting.value = false;
    }
    deleteModal.close();
    deletingDocId.value = null;
  }

  function handleMove(doc: { id: number; name: string; type: number; space_id: number }) {
    movingDocId.value = doc.id;
    showMoveModal.value = true;
  }

  function handleCopy(doc: { id: number; name: string; type: number; space_id: number }) {
    copyingDocId.value = doc.id;
    showCopyModal.value = true;
  }

  async function confirmMove(targetId: number, spaceId: number) {
    if (!movingDocId.value || moving.value) return;
    moving.value = true;
    try {
      await docStore.move(movingDocId.value, {
        parent_id: targetId,
        space_id: spaceId,
      });
      ui.showToast(t('tsUi.k29'), 'success');
    } catch {
      ui.showToast(t('tsUi.k30'), 'error');
    } finally {
      moving.value = false;
    }
    movingDocId.value = null;
  }

  async function confirmCopy(targetId: number, spaceId: number) {
    if (!copyingDocId.value || copying.value) return;
    copying.value = true;
    try {
      await docStore.copy(copyingDocId.value, {
        parent_id: targetId,
        space_id: spaceId,
      });
      ui.showToast(t('tsUi.k31'), 'success');
    } catch {
      ui.showToast(t('tsUi.k22'), 'error');
    } finally {
      copying.value = false;
    }
    copyingDocId.value = null;
  }

  return {
    showCreateModal,
    createDocName,
    createDocParentId,
    createDocSpaceId,
    createDocType,
    creatingDoc,
    showRenameModal,
    renameDocName,
    renamingDoc,
    renamingDocId,
    showDeleteModal,
    deletingDocId,
    deleteMessage,
    deletingDocName,
    showMoveModal,
    movingDocId,
    showCopyModal,
    copyingDocId,
    openCreateModal,
    handleCreateDoc,
    handleRename,
    handleRenameDoc,
    handleDelete,
    confirmDelete,
    handleMove,
    handleCopy,
    confirmMove,
    confirmCopy,
  };
}
