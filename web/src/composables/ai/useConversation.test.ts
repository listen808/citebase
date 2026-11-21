import { beforeEach, describe, expect, it, vi } from 'vitest';

const api = vi.hoisted(() => ({
  listConversations: vi.fn(),
  createConversation: vi.fn(),
  getConversation: vi.fn(),
  updateConversation: vi.fn(),
  deleteConversation: vi.fn(),
}));

vi.mock('@/api/modules/ai/ai-chat', () => api);

import { useConversation } from './useConversation';

describe('useConversation 的列表失败呈现', () => {
  beforeEach(() => vi.clearAllMocks());

  it('拉取失败时记下原因，侧栏才有办法不说「暂无对话记录」', async () => {
    api.listConversations.mockRejectedValue(new Error('network down'));
    const conv = useConversation('ai');

    await conv.fetchConversations();

    expect(conv.conversations.value).toEqual([]);
    expect(conv.error.value).not.toBe('');
    expect(conv.loading.value).toBe(false);
  });

  it('重试成功后清掉失败原因', async () => {
    api.listConversations.mockRejectedValueOnce(new Error('boom'));
    const conv = useConversation('ai');
    await conv.fetchConversations();
    expect(conv.error.value).not.toBe('');

    api.listConversations.mockResolvedValue({ conversations: [{ id: 1 }] as never });
    await conv.fetchConversations();
    expect(conv.error.value).toBe('');
    expect(conv.conversations.value.length).toBe(1);
  });

  it('删除失败仍然上抛（调用方据此收起确认弹窗并弹一次提示）', async () => {
    const conv = useConversation('ai');
    api.deleteConversation.mockRejectedValue(new Error('boom'));
    await expect(conv.removeConversation(3)).rejects.toThrow('boom');
  });
});
