import { shallowRef, computed } from 'vue';
import { useCollaboration } from '@/composables/collab/useCollaboration';
import { useDocumentStore } from '@/stores/workspace/document';
import { useUIStore } from '@/stores/app/ui';
import { getCollabState } from '@/api/modules/document/document';
import i18n from '@/i18n';

const { t } = i18n.global;

export type EditMode = 'exclusive' | 'collab';

/**
 * 编辑模式状态机：独占模式（排他锁）与协作模式（Yjs）互斥切换。
 * 进入时依据协作会话数自动判定；协作模式不持锁、独占模式不建 WebSocket。
 */
export function useEditMode(docId: number) {
  const docStore = useDocumentStore();
  const ui = useUIStore();
  const collab = useCollaboration(docId);

  const mode = shallowRef<EditMode>('exclusive');
  const isCollab = computed(() => mode.value === 'collab');
  const isExclusive = computed(() => mode.value === 'exclusive');

  async function enter(): Promise<boolean> {
    try {
      const state = await getCollabState(docId);
      if (state && state.clients > 0) {
        enterCollab();
        return true;
      }
    } catch {
      // 查询失败回退到独占模式
    }
    return await enterExclusive();
  }

  async function enterExclusive(): Promise<boolean> {
    const ok = await docStore.acquireLock(docId);
    if (ok) {
      collab.disconnect();
      mode.value = 'exclusive';
    }
    return ok;
  }

  function enterCollab(): void {
    if (mode.value === 'exclusive') {
      void docStore.releaseLock(docId);
    }
    mode.value = 'collab';
    collab.connect();
  }

  async function toggle(): Promise<void> {
    if (mode.value === 'exclusive') {
      enterCollab();
      return;
    }
    collab.disconnect();
    const ok = await docStore.acquireLock(docId);
    if (ok) {
      mode.value = 'exclusive';
    } else {
      ui.showToast(t('tsUi.k11'), 'warning');
      enterCollab();
    }
  }

  function dispose(): void {
    collab.disconnect();
    if (mode.value === 'exclusive') {
      void docStore.releaseLock(docId);
    }
  }

  return {
    mode,
    isCollab,
    isExclusive,
    enter,
    enterExclusive,
    enterCollab,
    toggle,
    dispose,
    collab,
  };
}