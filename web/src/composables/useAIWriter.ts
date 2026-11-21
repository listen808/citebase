import { ref, computed } from 'vue';
import type {
  AIWriterState,
  AIWriterTab,
  AITemplate,
  AIChatMessage,
} from '@/types/ai-writer';
import type { AIWriteAction } from '@/types/ai';
import { writeAssist, applyTemplate, chatWithDoc } from '@/api/modules/ai/ai-write';
import { useUIStore } from '@/stores/app/ui';
import { logger } from '@/utils/logger';
import i18n from '@/i18n';

const { t } = i18n.global;

// 函数而不是常量数组：常量在 import 那一刻就把语言钉死了，
// 之后切界面语言不会重算 —— 与 withDefaults 的默认值是同一类坑（没有任何门禁会红）。
const defaultTemplates = (): AITemplate[] => [
  {
    id: 'project_intro',
    name: t('tsUi.tplName1'),
    description: t('tsUi.k49'),
    variables: ['projectName', 'goal', 'teamSize', 'timeline'],
    icon: 'Folder'
  },
  {
    id: 'meeting_minutes',
    name: t('tsUi.tplName2'),
    description: t('tsUi.k50'),
    variables: ['date', 'attendees', 'topics', 'actionItems'],
    icon: 'Calendar'
  },
  {
    id: 'tech_doc',
    name: t('tsUi.tplName3'),
    description: t('tsUi.k51'),
    variables: ['featureName', 'purpose', 'architecture', 'usage'],
    icon: 'FileText'
  },
  {
    id: 'api_doc',
    name: t('tsUi.tplName4'),
    description: t('tsUi.k52'),
    variables: ['endpoint', 'method', 'params', 'response'],
    icon: 'Code2'
  }
];

export function useAIWriter(options?: {
  onInsertText?: (text: string) => void;
  onReplaceSelection?: (text: string) => void;
}) {
  const ui = useUIStore();

  const state = ref<AIWriterState>({
    open: false,
    activeTab: 'chat',
    messages: [],
    loading: false,
    error: '',
    streamingContent: '',
    selectedText: '',
    templates: defaultTemplates()
  });

  // 基础操作
  const open = () => {
    state.value.open = true;
  };

  const close = () => {
    state.value.open = false;
  };

  const setActiveTab = (tab: AIWriterTab) => {
    state.value.activeTab = tab;
  };

  const setSelectedText = (text: string) => {
    state.value.selectedText = text;
  };

  // 聊天功能
  const sendMessage = async (content: string, ctx?: { docId?: number; spaceId?: number }) => {
    if (!content.trim()) return;

    const userMsg: AIChatMessage = {
      id: Date.now().toString(),
      role: 'user',
      content,
      timestamp: Date.now()
    };

    state.value.messages.push(userMsg);
    state.value.loading = true;
    state.value.error = '';
    state.value.streamingContent = '';

    try {
      const res = await chatWithDoc({
        messages: state.value.messages.map(m => ({ role: m.role, content: m.content })),
        spaceId: ctx?.spaceId,
        docId: ctx?.docId,
        selectedText: state.value.selectedText || undefined
      });

      const assistantMsg: AIChatMessage = {
        id: (Date.now() + 1).toString(),
        role: 'assistant',
        content: res.content,
        timestamp: Date.now(),
        sources: res.sources
      };

      state.value.messages.push(assistantMsg);
    } catch (error) {
      logger.error('Chat error', error);
      // 原先只 log：spinner 停转、无回复、无提示，失败被呈现成「AI 没话说」。
      // 失败原因只写进 state.error 并弹一次提示，不回写进 messages ——
      // 那一条会在下一轮被整段映射进请求体，把错误文本当历史发给模型。
      state.value.error = t('tsUi.k53');
      ui.showToast(state.value.error, 'error');
    } finally {
      state.value.loading = false;
    }
  };

  const clearMessages = () => {
    state.value.messages = [];
    state.value.error = '';
  };

  // 模板功能
  const applyAI = async (templateId: string, variables: Record<string, string>) => {
    state.value.loading = true;
    try {
      const res = await applyTemplate({ templateId, variables });
      if (options?.onInsertText && res.result) {
        options.onInsertText(res.result);
      }
      return res.result;
    } catch (error) {
      logger.error('Apply template error', error);
      throw error;
    } finally {
      state.value.loading = false;
    }
  };

  // 文本优化
  const optimizeSelection = async (action: AIWriteAction, text: string) => {
    state.value.loading = true;
    try {
      const res = await writeAssist({ text, action });
      if (options?.onReplaceSelection && res.result) {
        options.onReplaceSelection(res.result);
      }
      return res.result;
    } catch (error) {
      logger.error('Optimize error', error);
      throw error;
    } finally {
      state.value.loading = false;
    }
  };

  // 插入/替换文本
  const insertText = (text: string) => {
    options?.onInsertText?.(text);
  };

  const replaceSelection = (text: string) => {
    options?.onReplaceSelection?.(text);
  };

  return {
    state,
    isOpen: computed(() => state.value.open),
    activeTab: computed(() => state.value.activeTab),
    messages: computed(() => state.value.messages),
    loading: computed(() => state.value.loading),
    /** 最近一次发送失败的原因（空串 = 没有失败）；面板目前靠 toast 呈现，这一档留给渲染态 */
    error: computed(() => state.value.error),
    templates: computed(() => state.value.templates),

    // Actions
    open,
    close,
    setActiveTab,
    setSelectedText,
    sendMessage,
    clearMessages,
    applyAI,
    optimizeSelection,
    insertText,
    replaceSelection
  };
}
