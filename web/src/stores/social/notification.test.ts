import { beforeEach, describe, expect, it, vi } from 'vitest';
import { createPinia, setActivePinia } from 'pinia';

// API 层整体换成可控的替身：这里要验的是 store 在失败时留下什么痕迹，不是请求怎么发。
const api = vi.hoisted(() => ({
  getNotifications: vi.fn(),
  getUnreadCount: vi.fn(),
  markNotificationRead: vi.fn(),
  markAllNotificationsRead: vi.fn(),
  deleteNotification: vi.fn(),
}));

vi.mock('@/api/modules/social/notification', () => api);

import { useNotificationStore } from './notification';
import { useUIStore } from '@/stores/app/ui';

describe('notification store 的失败呈现', () => {
  beforeEach(() => {
    setActivePinia(createPinia());
    vi.clearAllMocks();
  });

  it('列表加载失败时记下原因，而不是把「挂了」报成「没有通知」', async () => {
    api.getNotifications.mockRejectedValue(new Error('network down'));
    api.getUnreadCount.mockResolvedValue({ count: 0 });

    const store = useNotificationStore();
    await store.loadList();

    expect(store.items).toEqual([]);
    expect(store.error).not.toBe('');
    expect(store.loading).toBe(false);
  });

  it('重新加载成功后清掉失败原因', async () => {
    api.getNotifications.mockRejectedValueOnce(new Error('boom'));
    const store = useNotificationStore();
    await store.loadList();
    expect(store.error).not.toBe('');

    api.getNotifications.mockResolvedValue({ data: [{ id: 1 }], total: 1 });
    await store.loadList();
    expect(store.error).toBe('');
    expect(store.items.length).toBe(1);
  });

  it('未读数拉取失败时保留上一次的计数且不打扰用户', async () => {
    api.getUnreadCount.mockResolvedValueOnce({ count: 3 });
    const store = useNotificationStore();
    await store.loadUnreadCount();
    expect(store.unreadCount).toBe(3);

    const ui = useUIStore();
    const toast = vi.spyOn(ui, 'showToast');
    api.getUnreadCount.mockRejectedValueOnce(new Error('boom'));
    await store.loadUnreadCount();

    // 归零等于让角标凭空消失 —— 那是把「不知道」当成「没有」
    expect(store.unreadCount).toBe(3);
    // 被动轮询失败不弹 toast：用户没做操作，浮层报错会盖住正在看的内容
    expect(toast).not.toHaveBeenCalled();
  });

  it('三个写操作把错误上抛，交给调用方弹一次提示', async () => {
    const store = useNotificationStore();
    api.markNotificationRead.mockRejectedValue(new Error('boom'));
    await expect(store.markRead(1)).rejects.toThrow('boom');

    api.markAllNotificationsRead.mockRejectedValue(new Error('boom'));
    await expect(store.markAllRead()).rejects.toThrow('boom');

    api.deleteNotification.mockRejectedValue(new Error('boom'));
    await expect(store.remove(1)).rejects.toThrow('boom');

    // 失败时不该顺手把列表刷空（那会让「删除失败」看起来像「删掉了」）
    expect(api.getNotifications).not.toHaveBeenCalled();
  });

  it('写操作成功后才重拉列表与未读数', async () => {
    const store = useNotificationStore();
    api.deleteNotification.mockResolvedValue({});
    api.getNotifications.mockResolvedValue({ data: [], total: 0 });
    api.getUnreadCount.mockResolvedValue({ count: 0 });

    await store.remove(7);

    expect(api.getNotifications).toHaveBeenCalled();
    expect(api.getUnreadCount).toHaveBeenCalled();
  });
});
