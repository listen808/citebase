import { type Ref } from 'vue';
import { storageGet, storageSet, StorageKeys } from '@/utils/core/storage';

export function useExpandedState(expandedNodes: Ref<Set<number>>) {
  function persistExpanded(spaceId: number) {
    try {
      const key = `${StorageKeys.EXPANDED_NODES}-${spaceId}`;
      storageSet(key, JSON.stringify([...expandedNodes.value]));
    } catch { /* noop */ }
  }

  function restoreExpanded(spaceId: number): Set<number> {
    try {
      const key = `${StorageKeys.EXPANDED_NODES}-${spaceId}`;
      const saved = storageGet(key);
      if (saved) {
        const ids: number[] = JSON.parse(saved);
        return new Set(ids);
      }
    } catch { /* noop */ }
    return new Set();
  }

  return { persistExpanded, restoreExpanded };
}
