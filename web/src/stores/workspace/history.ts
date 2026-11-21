import { defineStore } from 'pinia';
import { ref, shallowRef } from 'vue';
import { storageGet, storageSet, storageRemove, StorageKeys } from '@/utils/core/storage';
import { debounce } from '@/utils/core/shared';

const MAX_STACK_SIZE = 50;
const PERSIST_DEBOUNCE_MS = 500;

export interface PageState {
  path: string;
  scrollTop: number;
  expandedNodes: number[];
  selectedDocumentId?: number;
  searchKeyword?: string;

  sidebarCollapsed?: boolean;
}

export const useHistoryStore = defineStore('history', () => {
  const stack = ref<PageState[]>([]);
  const currentIndex = shallowRef(-1);
  
  function initFromStorage() {
    try {
      const saved = storageGet(StorageKeys.HISTORY_STACK);
      if (saved) {
        const data = JSON.parse(saved);
        if (Array.isArray(data.stack) && typeof data.currentIndex === 'number') {
          stack.value = data.stack.slice(-MAX_STACK_SIZE);
          currentIndex.value = Math.min(data.currentIndex, stack.value.length - 1);
        }
      }
    } catch { /* ignore corrupt data */ }
  }

  function persist() {
    try {
      storageSet(StorageKeys.HISTORY_STACK, JSON.stringify({
        stack: stack.value,
        currentIndex: currentIndex.value,
      }));
    } catch { /* ignore storage errors */ }
  }

  const debouncedPersist = debounce(persist, PERSIST_DEBOUNCE_MS);

  function push(state: PageState) {
    if (currentIndex.value < stack.value.length - 1) {
      stack.value = stack.value.slice(0, currentIndex.value + 1);
    }
    stack.value.push(state);
    if (stack.value.length > MAX_STACK_SIZE) {
      stack.value.shift();
    }
    currentIndex.value = stack.value.length - 1;
    debouncedPersist();
  }

  function canGoBack() {
    return currentIndex.value > 0;
  }

  function canGoForward() {
    return currentIndex.value < stack.value.length - 1;
  }

  function goBack(): PageState | null {
    if (!canGoBack()) return null;
    currentIndex.value--;
    debouncedPersist();
    return stack.value[currentIndex.value] || null;
  }

  function goForward(): PageState | null {
    if (!canGoForward()) return null;
    currentIndex.value++;
    debouncedPersist();
    return stack.value[currentIndex.value] || null;
  }

  function getCurrentState(): PageState | null {
    if (currentIndex.value < 0 || currentIndex.value >= stack.value.length) {
      return null;
    }
    return stack.value[currentIndex.value] || null;
  }

  function clear() {
    stack.value = [];
    currentIndex.value = -1;
    debouncedPersist.cancel();
    storageRemove(StorageKeys.HISTORY_STACK);
  }

  function flushPersist() {
    debouncedPersist.cancel();
    persist();
  }

  return {
    stack,
    currentIndex,
    push,
    canGoBack,
    canGoForward,
    goBack,
    goForward,
    getCurrentState,
    clear,
    initFromStorage,
    flushPersist,
  };
});
