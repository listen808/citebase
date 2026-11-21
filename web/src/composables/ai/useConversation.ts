import { ref } from 'vue';
import { listConversations, createConversation, getConversation, updateConversation, deleteConversation } from '@/api/modules/ai/ai-chat';
import type { AIConversation } from '@/types/ai';
import { logger } from '@/utils/logger';
import i18n from '@/i18n';

const { t } = i18n.global;

export function useConversation(type: 'ai' | 'sql' = 'ai') {
  const conversations = ref<AIConversation[]>([]);
  const activeConversationId = ref<number | null>(null);
  const loading = ref(false);
  /** 列表加载失败的可见原因；空串 = 没有失败。没有它时「请求挂了」在侧栏里就是「暂无对话记录」 */
  const error = ref('');

  async function fetchConversations() {
    loading.value = true;
    error.value = '';
    try {
      const res = await listConversations(1, 50, type);
      conversations.value = res.conversations || [];
    } catch (e) {
      logger.error('Failed to fetch conversations:', e);
      conversations.value = [];
      error.value = t('tsUi.k2');
    } finally {
      loading.value = false;
    }
  }

  async function createNewConversation(title: string) {
    try {
      const res = await createConversation({ title: title.slice(0, 50), type });
      activeConversationId.value = res.conversation.id;
      await fetchConversations();
      return res.conversation.id;
    } catch (e) {
      logger.error('Failed to create conversation:', e);
      return null;
    }
  }

  async function loadConversation(id: number) {
    try {
      const res = await getConversation(id);
      return res.conversation;
    } catch (e) {
      logger.error('Failed to load conversation:', e);
      return null;
    }
  }

  async function saveConversation(id: number, messages: { role: string; content: string }[]) {
    try {
      await updateConversation(id, { messages });
      return true;
    } catch (e) {
      logger.error('Failed to save conversation:', e);
      throw e; // rethrow so callers can show user feedback
    }
  }

  async function removeConversation(id: number) {
    try {
      await deleteConversation(id);
      if (activeConversationId.value === id) {
        activeConversationId.value = null;
      }
      await fetchConversations();
    } catch (e) {
      logger.error('Failed to delete conversation:', e);
      throw e; // rethrow so callers can show user feedback
    }
  }

  function clearActive() {
    activeConversationId.value = null;
  }

  return {
    conversations,
    activeConversationId,
    loading,
    error,
    fetchConversations,
    createNewConversation,
    loadConversation,
    saveConversation,
    removeConversation,
    clearActive,
  };
}
