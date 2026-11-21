import { defineStore } from 'pinia';
import { ref, shallowRef } from 'vue';
import { getNotifications, getUnreadCount, markNotificationRead, markAllNotificationsRead, deleteNotification, type NotificationItem } from '@/api/modules/social/notification';

import { logger } from '@/utils/logger';
import i18n from '@/i18n';

const { t } = i18n.global;

export const useNotificationStore = defineStore('notification', () => {
  const items = ref<NotificationItem[]>([]);
  const total = shallowRef(0);
  const unreadCount = shallowRef(0);
  const loading = shallowRef(false);
  /** 列表加载失败的可见原因；空串 = 没有失败。view 侧靠它把「加载失败」和「确实没有通知」分开 */
  const error = shallowRef('');

  async function loadList(params: { type?: 'all' | 'unread'; page?: number; page_size?: number } = {}) {
    loading.value = true;
    error.value = '';
    try {
      const res = await getNotifications({
        type: params.type ?? 'all',
        page: params.page ?? 1,
        page_size: params.page_size ?? 20,
      });
      const data = res as unknown as { data: NotificationItem[]; total: number };
      items.value = data?.data ?? [];
      total.value = data?.total ?? 0;
    } catch (e) {
      logger.error('Failed to fetch notifications:', e);
      // 原先只 log 并把列表清成 []，于是「请求挂了」在页面上长得和「你一条通知都没有」一样。
      items.value = [];
      error.value = t('tsUi.k79');
    } finally {
      loading.value = false;
    }
  }

  async function loadUnreadCount() {
    try {
      const res = await getUnreadCount();
      const data = res as unknown as { count: number };
      unreadCount.value = data?.count ?? 0;
    } catch (e) {
      // 未读数是顶栏角标的被动轮询：失败只记日志并保留上一次计数，
      // 不弹 toast —— 用户没做操作，浮层报错会盖掉正在看的内容（首页曾三条同源错误条堆叠）。
      logger.error('Failed to fetch unread count:', e);
    }
  }

  /**
   * 三个写操作把错误上抛（同 useConversation.saveConversation 的口径），
   * 由调用方弹一次 toast —— 原先在 store 里吞掉，页面只能显示「已标记已读」的假成功。
   */
  async function markRead(id: number) {
    try {
      await markNotificationRead(id);
    } catch (e) {
      logger.error('Failed to mark notification read:', e);
      throw e;
    }
    await Promise.all([loadList(), loadUnreadCount()]);
  }

  async function markAllRead() {
    try {
      await markAllNotificationsRead();
    } catch (e) {
      logger.error('Failed to mark all notifications read:', e);
      throw e;
    }
    await Promise.all([loadList(), loadUnreadCount()]);
  }

  async function remove(id: number) {
    try {
      await deleteNotification(id);
    } catch (e) {
      logger.error('Failed to delete notification:', e);
      throw e;
    }
    await Promise.all([loadList(), loadUnreadCount()]);
  }

  return {
    items,
    total,
    unreadCount,
    loading,
    error,
    loadList,
    loadUnreadCount,
    markRead,
    markAllRead,
    remove,
  };
});
