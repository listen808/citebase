import { defineStore } from 'pinia';
import { ref, shallowRef, onScopeDispose } from 'vue';
import { getAllSpaces, getMySpaces, getSpaceByID } from '@/api/modules/space/space';
import type { Space } from '@/types/models';
import { mergeMySpacesToAll } from '@/utils/domain/space';
import type { CacheEntry } from '@/utils/core/cache';
import { isCacheValid } from '@/utils/core/cache';

export const useSpaceStore = defineStore('space', () => {
  const spaces = ref<Space[]>([]);
  const mySpaces = ref<Space[]>([]);
  const currentSpace = ref<Space | null>(null);
  const loading = shallowRef(false);

  const spacesCache = ref<CacheEntry<Space[]> | null>(null);
  // 缓存 key 包含参数 hash，参数变化时清缓存复用
  const mySpacesCache = ref<{ data: Space[]; timestamp: number; paramsHash: string } | null>(null);
  const spaceCacheMap = shallowRef<Map<number, CacheEntry<Space>>>(new Map());

  function hashParams(params?: { page_size?: number }): string {
    if (!params) return 'default';
    return Object.entries(params).sort().map(([k, v]) => `${k}:${v}`).join('|');
  }

  async function loadSpaces() {
    if (isCacheValid(spacesCache.value)) {
      spaces.value = spacesCache.value.data;
      return;
    }

    loading.value = true;
    try {
      const res = await getAllSpaces();
      spaces.value = res.data;
      spacesCache.value = { data: res.data, timestamp: Date.now() };
    } finally {
      loading.value = false;
    }
  }

  async function loadMySpaces(params?: { visit_level?: string; tags?: string; page?: number; page_size?: number; keyword?: string }) {
    const paramsHash = hashParams(params);
    if (mySpacesCache.value && mySpacesCache.value.paramsHash === paramsHash && isCacheValid(mySpacesCache.value)) {
      mySpaces.value = mySpacesCache.value.data;
      return;
    }

    loading.value = true;
    try {
      const res = await getMySpaces(params);
      mySpaces.value = res.data;
      mySpacesCache.value = { data: res.data, timestamp: Date.now(), paramsHash };
      spaces.value = mergeMySpacesToAll(res.data, spaces.value, spacesCache);
    } finally {
      loading.value = false;
    }
  }

  async function loadSpace(id: number) {
    const cached = spaceCacheMap.value.get(id);
    if (isCacheValid(cached)) {
      currentSpace.value = cached.data;
    }

    loading.value = true;
    try {
      const res = await getSpaceByID(id);
      currentSpace.value = res;
      const newMap = new Map(spaceCacheMap.value);
      newMap.set(id, { data: res, timestamp: Date.now() });
      spaceCacheMap.value = newMap;
    } finally {
      loading.value = false;
    }
  }

  function invalidateSpacesCache() {
    spacesCache.value = null;
    mySpacesCache.value = null;
    spaceCacheMap.value = new Map();
  }

  const scheduleTimers = new Map<() => Promise<void>, ReturnType<typeof setTimeout>>();

  function scheduleLoad(loadFn: () => Promise<void>, delay = 300): () => void {
    const existing = scheduleTimers.get(loadFn);
    if (existing) clearTimeout(existing);
    let cancelled = false;
    const timer = setTimeout(() => {
      scheduleTimers.delete(loadFn);
      if (!cancelled) loadFn();
    }, delay);
    scheduleTimers.set(loadFn, timer);
    return () => {
      cancelled = true;
      const t = scheduleTimers.get(loadFn);
      if (t) {
        clearTimeout(t);
        scheduleTimers.delete(loadFn);
      }
    };
  }

  function scheduleLoadSpaces(delay = 300): () => void {
    return scheduleLoad(loadSpaces, delay);
  }

  function scheduleLoadMySpaces(delay = 300): () => void {
    return scheduleLoad(loadMySpaces, delay);
  }

  function cleanup() {
    for (const t of scheduleTimers.values()) clearTimeout(t);
    scheduleTimers.clear();
  }

  onScopeDispose(() => {
    cleanup();
  });

  return {
    spaces,
    mySpaces,
    currentSpace,
    loading,
    loadSpaces,
    loadMySpaces,
    loadSpace,
    invalidateSpacesCache,
    scheduleLoadSpaces,
    scheduleLoadMySpaces,
    cleanup,
  };
});
