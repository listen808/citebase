import { ref, shallowRef, reactive, watch } from 'vue';
import { useUIStore } from '@/stores/app/ui';
import { scrollContentToTop } from '@/utils/core/scroll';
import { storageGet, storageSet, StorageKeys } from '@/utils/core/storage';
import i18n from '@/i18n';

const { t } = i18n.global;

export interface FetchParams {
  page: number;
  pageSize: number;
  keyword: string;
}

export interface PaginatedResult<T> {
  data: T[];
  total: number;
  page: number;
  page_size: number;
}

export function useAdminCrud<T extends { id: number }>(options: {
  fetchFn: (params: FetchParams) => Promise<PaginatedResult<T>>;
  createFn?: (data: Partial<T>) => Promise<unknown>;
  updateFn?: (data: Partial<T>) => Promise<unknown>;
  deleteFn?: (id: number) => Promise<unknown>;
  itemName?: string;
  searchFields?: string[];
  resetForm?: () => void;
}) {
  const ui = useUIStore();
  const itemName = options.itemName || '项目';

  const loading = shallowRef(false);
  const loadError = shallowRef<string | null>(null);
  const submitting = shallowRef(false);
  const items = ref<T[]>([]);
  const total = shallowRef(0);
  const page = shallowRef(1);
  const pageSize = shallowRef(20);
  const keyword = shallowRef('');

  const showCreateModal = shallowRef(false);
  const showEditModal = shallowRef(false);
  const showDeleteModal = shallowRef(false);
  const editingItem = shallowRef<T | null>(null);

  const formErrors = reactive<Record<string, string>>({});
  const viewMode = shallowRef<'card' | 'table'>(
    storageGet(StorageKeys.UI_COLLECTION_VIEW_MODE) === 'table' ? 'table' : 'card',
  );

  // 卡片↔列表只是同一批数据的两种排布，不该重新请求；档位记到全站那一个键上，切页与刷新都跟手
  watch(viewMode, (mode) => {
    storageSet(StorageKeys.UI_COLLECTION_VIEW_MODE, mode);
  });

  function clearFormErrors() {
    Object.keys(formErrors).forEach(key => { formErrors[key] = ''; });
  }

  async function fetchList() {
    loading.value = true;
    loadError.value = null;
    try {
      const res = await options.fetchFn({
        page: page.value,
        pageSize: pageSize.value,
        keyword: keyword.value,
      });
      items.value = res.data || [];
      total.value = res.total || 0;
      if (items.value.length === 0 && page.value > 1) {
        page.value = Math.max(1, page.value - 1);
        await fetchList();
        return;
      }
    } catch (e: unknown) {
      // 失败不再伪装成"空列表"：置错误态供页面渲染错误块 + 重试
      loadError.value = e instanceof Error && e.message ? e.message : t('sentUi.k73', { p1: itemName });
      ui.showToast(loadError.value, 'error');
    } finally {
      loading.value = false;
    }
  }

  /** 防抖由 ApSearch 统一负责（AppleDuration.debounceSearch），此处不再叠加第二层 */
  function handleSearch() {
    page.value = 1;
    fetchList();
  }

  function changePage(newPage: number) {
    page.value = newPage;
    fetchList();
    scrollContentToTop();
  }

  /** 错误态下的手动重试入口 */
  function retryLoad() {
    loadError.value = null;
    fetchList();
  }

  function openCreate() {
    editingItem.value = null;
    clearFormErrors();
    showCreateModal.value = true;
  }

  function openEdit(item: T) {
    editingItem.value = { ...item };
    clearFormErrors();
    showEditModal.value = true;
  }

  function openDelete(item: T) {
    editingItem.value = { ...item };
    showDeleteModal.value = true;
  }

  function closeModals() {
    showCreateModal.value = false;
    showEditModal.value = false;
    showDeleteModal.value = false;
    editingItem.value = null;
    options.resetForm?.();
  }

  async function handleCreate(data: Partial<T>) {
    if (!options.createFn || submitting.value) return;
    submitting.value = true;
    try {
      await options.createFn(data);
      ui.showToast(t('sentUi.k74', { p1: itemName }), 'success');
      closeModals();
      fetchList();
    } catch (e: unknown) {
      ui.showToast((e instanceof Error ? e.message : '') || `添加${itemName}失败`, 'error');
    } finally {
      submitting.value = false;
    }
  }

  async function handleUpdate(data: Partial<T>) {
    if (!options.updateFn || submitting.value) return;
    submitting.value = true;
    try {
      await options.updateFn(data);
      ui.showToast(t('sentUi.k75', { p1: itemName }), 'success');
      closeModals();
      fetchList();
    } catch (e: unknown) {
      ui.showToast((e instanceof Error ? e.message : '') || `更新${itemName}失败`, 'error');
    } finally {
      submitting.value = false;
    }
  }

  async function handleDelete() {
    if (!options.deleteFn || !editingItem.value || submitting.value) return;
    submitting.value = true;
    try {
      await options.deleteFn(editingItem.value.id);
      ui.showToast(t('tsUi.k19'), 'success');
      closeModals();
      fetchList();
    } catch (e: unknown) {
      ui.showToast((e instanceof Error ? e.message : '') || `删除${itemName}失败`, 'error');
    } finally {
      submitting.value = false;
    }
  }

  async function handleSubmit(data: Partial<T>, validateFn?: (data: Partial<T>) => Record<string, string> | null) {
    clearFormErrors();
    if (validateFn) {
      const errors = validateFn(data);
      if (errors) {
        Object.assign(formErrors, errors);
        return;
      }
    }
    if (editingItem.value) {
      await handleUpdate({ ...data, id: editingItem.value.id });
    } else {
      await handleCreate(data);
    }
  }

  return {
    loading,
    loadError,
    submitting,
    items,
    total,
    page,
    pageSize,
    keyword,
    showCreateModal,
    showEditModal,
    showDeleteModal,
    editingItem,
    formErrors,
    viewMode,
    fetchList,
    retryLoad,
    handleSearch,
    changePage,
    openCreate,
    openEdit,
    openDelete,
    closeModals,
    clearFormErrors,
    handleCreate,
    handleUpdate,
    handleDelete,
    handleSubmit,
  };
}
