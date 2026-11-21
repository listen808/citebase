import { defineStore } from 'pinia';
import { ref, shallowRef } from 'vue';
import { getMyCollections, toggleCollection, checkCollected } from '@/api/modules/space/collection';
import type { CollectionItem } from '@/api/modules/space/collection';
import { useUIStore } from '@/stores/app/ui';
import i18n from '@/i18n';

const { t } = i18n.global;

export const useCollectionStore = defineStore('collection', () => {
  const items = ref<CollectionItem[]>([]);
  const total = shallowRef(0);
  const loading = shallowRef(false);
  const collectedMap = ref<Map<string, boolean>>(new Map());

  async function loadCollections(params?: { type?: number; page?: number; page_size?: number; keyword?: string }, signal?: AbortSignal) {
    loading.value = true;
    try {
      const res = await getMyCollections(params, { signal });
      items.value = res.data;
      total.value = res.total;
      return res;
    } catch (e) {
      if (signal?.aborted || (e instanceof Error && e.name === 'AbortError')) return null;
      // 不在 store 里弹 toast：被动加载（侧栏/首页）失败应静默降级，
      // 收藏页主体失败由调用方 catch 决定呈现。原先 store 吞错返回 null 会让
      // CollectionsView 的外层 catch 变死代码、页面只能显示「加载完了但空的」假成功。
      throw e;
    } finally {
      loading.value = false;
    }
  }

  // Toggle pattern: call API first, then update map+list on success; map rollback on failure
  // (Same pattern as follow.ts toggle, but rollback differs: filter+map restore vs splice)
  async function toggle(type: number, resourceId: number) {
    const key = `${type}-${resourceId}`;
    const wasCollected = collectedMap.value.get(key) ?? false;

    try {
      const res = await toggleCollection(type, resourceId);
      collectedMap.value.set(key, res.collected);
      
      if (!res.collected) {
        items.value = items.value.filter(
          item => !(item.type === type && item.resource_id === resourceId)
        );
        total.value--;
      }
      
      return res.collected;
    } catch {
      collectedMap.value.set(key, wasCollected);
      const ui = useUIStore();
      ui.showToast(t('tsUi.k81'), 'error');
      return null;
    }
  }

  async function checkStatus(type: number, resourceId: number) {
    try {
      const res = await checkCollected(type, resourceId);
      const key = `${type}-${resourceId}`;
      collectedMap.value.set(key, res.collected);
      return res.collected;
    } catch {
      return false;
    }
  }

  function isCollected(type: number, resourceId: number): boolean {
    return collectedMap.value.get(`${type}-${resourceId}`) ?? false;
  }

  return {
    items,
    total,
    loading,
    collectedMap,
    loadCollections,
    toggle,
    checkStatus,
    isCollected,
  };
});
