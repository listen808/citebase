import { ref, shallowRef } from 'vue';
import { getUserActivity } from '@/api/modules/system/log';
import type { UserActivity } from '@/api/modules/system/log';
import { logger } from '@/utils/logger';
import { storageGet, storageSet, storageRemove, StorageKeys } from '@/utils/core/storage';

const MAX_RECENT = 20;
// 缓存只负责秒开，超过这个时长后台回源刷新一次，避免「最近文档」永远停在快照上。
const REFETCH_TTL = 5 * 60 * 1000;

export interface RecentDoc {
  id: number;
  name: string;
  space_id: number;
  space_name: string;
  update_time: number;
  action?: string;
  creator_user_id?: number;
  creator_name?: string;
  isDeleted?: boolean;
}

const sharedDocs = ref<RecentDoc[]>([]);
const sharedLoading = shallowRef(false);
// 加载失败态：给消费方区分「加载完但空」与「请求挂了」（原先静默吞掉两者长得一样）。
const sharedError = ref(false);
let loadSeq = 0;
let lastFetchAt = 0;

export function useRecentDocs() {
  const docs = sharedDocs;
  const loading = sharedLoading;
  const error = sharedError;

  function loadFromStorage(): RecentDoc[] {
    try {
      const raw = storageGet(StorageKeys.RECENT_DOCS);
      if (raw) {
        const list = JSON.parse(raw);
        return list.sort((a: RecentDoc, b: RecentDoc) => b.update_time - a.update_time);
      }
    } catch { /* noop */ }
    return [];
  }

  function persist(list: RecentDoc[]) {
    try {
      storageSet(StorageKeys.RECENT_DOCS, JSON.stringify(list.slice(0, MAX_RECENT)));
    } catch { /* noop */ }
  }

  function recordVisit(doc: RecentDoc) {
    const list = loadFromStorage();
    const idx = list.findIndex(d => d.id === doc.id);
    if (idx >= 0) list.splice(idx, 1);
    list.unshift(doc);
    persist(list);
    docs.value = list.slice(0, 10);
  }

  async function loadRecentDocs() {
    const seq = ++loadSeq;
    loading.value = true;
    error.value = false;
    try {
      const cached = loadFromStorage();
      if (cached.length > 0) docs.value = cached.slice(0, 10);
      if (cached.length >= 3 && Date.now() - lastFetchAt < REFETCH_TTL) return;
      const res = await getUserActivity({ page: 1, page_size: 20 });
      // 过期响应直接丢弃：侧栏与最近页并发调用时，慢的那次不能覆盖新数据。
      if (seq !== loadSeq) return;
      const activities: UserActivity[] = res?.data ?? [];
      const seen = new Set<number>();
      const recent: RecentDoc[] = [];
      for (const act of activities) {
        const docId = act.document_id || act.document?.document_id;
        if (!docId || seen.has(docId)) continue;
        seen.add(docId);
        recent.push({
          id: docId,
          name: act.document_name || act.document?.name || '',
          space_id: act.space_id || act.document?.space_id || 0,
          space_name: act.space_name || '',
          update_time: act.create_time,
          action: act.action || undefined,
          creator_user_id: act.operator?.user_id || act.user_id,
          creator_name: act.given_name || act.operator?.given_name || act.operator?.username || act.username,
          isDeleted: act.action === 'delete' || act.action === '3',
        });
        if (recent.length >= 10) break;
      }
      recent.sort((a, b) => b.update_time - a.update_time);
      docs.value = recent;
      if (recent.length > 0) persist(recent);
      lastFetchAt = Date.now();
    } catch (e) {
      logger.error('Failed to load recent docs:', e);
      // 侧栏/首页是被动展示位，不弹 toast 盖掉用户正在看的内容；只在无缓存可显示时标错误态。
      if (docs.value.length === 0 && seq === loadSeq) error.value = true;
    } finally {
      if (seq === loadSeq) loading.value = false;
    }
  }

  function clear() {
    docs.value = [];
    storageRemove(StorageKeys.RECENT_DOCS);
  }

  return { docs, loading, error, recordVisit, loadRecentDocs, clear };
}
