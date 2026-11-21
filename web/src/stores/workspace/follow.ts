import { defineStore } from 'pinia';
import { ref, shallowRef } from 'vue';
import { getFollowingList, getFollowersList, toggleFollowUser, toggleFollowDocument, getFollowStats } from '@/api/modules/space/follow';
import type { FollowUser } from '@/api/modules/space/follow';
import { useUIStore } from '@/stores/app/ui';
import i18n from '@/i18n';

const { t } = i18n.global;

export const useFollowStore = defineStore('follow', () => {
  const followingList = ref<FollowUser[]>([]);
  const followersList = ref<FollowUser[]>([]);
  const stats = ref({ following: 0, followers: 0 });
  const loading = shallowRef(false);

  let loadAllAbort: AbortController | null = null;

  async function loadAll(userId?: number) {
    loadAllAbort?.abort();
    loadAllAbort = new AbortController();
    const signal = loadAllAbort.signal;
    loading.value = true;
    try {
      const results = await Promise.allSettled([
        getFollowingList(userId),
        getFollowersList(userId),
      ]);
      if (signal.aborted) return;
      
      if (results[0].status === 'fulfilled') {
        followingList.value = results[0].value.data || [];
        stats.value = { ...stats.value, following: results[0].value.following || 0 };
      }
      if (results[1].status === 'fulfilled') {
        followersList.value = results[1].value.data || [];
        stats.value = { ...stats.value, followers: results[1].value.followers || 0 };
      }
      
      const failed = results.filter(r => r.status === 'rejected');
      if (failed.length > 0) {
        const ui = useUIStore();
        ui.showToast(t('sentUi.k78', { p1: failed.length }), 'warning');
      }
    } catch (e) {
      if (signal.aborted || (e instanceof Error && e.name === 'AbortError')) return;
      const ui = useUIStore();
      ui.showToast(t('tsUi.k82'), 'error');
    } finally {
      loading.value = false;
    }
  }

  // Toggle pattern: call API first, then update list on success; splice rollback on failure
  // (Same pattern as collection.ts toggle, but rollback differs: splice vs filter+map restore)
  async function toggle(type: 'user' | 'doc', id: number) {
    const prevFollowing = stats.value.following;
    let removedItem: FollowUser | null = null;
    let removedIndex = -1;

    const isUser = type === 'user';
    const apiCall = isUser ? toggleFollowUser(id) : toggleFollowDocument(id);
    const matchItem = isUser
      ? (item: FollowUser) => item.type === 2 && item.target_user?.user_id === id
      : (item: FollowUser) => item.type === 1 && item.target_doc?.document_id === id;

    try {
      const res = await apiCall;

      if (!res.followed) {
        removedIndex = followingList.value.findIndex(matchItem);
        if (removedIndex !== -1) {
          removedItem = followingList.value[removedIndex] ?? null;
          followingList.value.splice(removedIndex, 1);
          if (isUser) stats.value.following--;
        }
      }

      return res.followed;
    } catch {
      if (removedIndex !== -1 && removedItem) {
        followingList.value.splice(removedIndex, 0, removedItem);
        if (isUser) stats.value.following = prevFollowing;
      }
      const ui = useUIStore();
      ui.showToast(t('tsUi.k81'), 'error');
      return null;
    }
  }

  async function refreshStats(userId?: number) {
    try {
      const res = await getFollowStats(userId);
      stats.value = { following: res.following, followers: res.followers };
    } catch {
      // ignore
    }
  }

  return {
    followingList,
    followersList,
    stats,
    loading,
    loadAll,
    toggle,
    refreshStats,
  };
});
