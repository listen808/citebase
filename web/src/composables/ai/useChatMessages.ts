import { ref, nextTick } from 'vue';

export interface ChatSource {
  document_id: number;
  document_name: string;
  space_id: number;
  score: number;
  snippet: string;
  start_line: number;
  end_line: number;
  heading_path: string;
  keywords: string[];
  citation_index?: number;
  updated_at?: number;
  cited?: boolean;
  verify_status?: 'cited' | 'unverified' | 'not_found';
  doc_url?: string;
  paragraph_id?: number;
}

export interface StreamComponent {
  id: string;
  type: string;
  title?: string;
  updated_at?: number;
  [key: string]: unknown;
}

export interface ChatMessage {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  think?: string;
  status?: 'loading' | 'success' | 'failed' | 'aborted';
  sources?: ChatSource[];
  reasoningChain?: ReasoningStep[];
  components?: GenComponent[];
  streamComponents?: StreamComponent[];
  sql?: string;
  sqlResult?: unknown;
  sqlExplanation?: unknown;
  feedback?: { rating: number; comment: string; submitted: boolean };
  traceId?: string;
  clarify?: { options: string[]; question: string; history: unknown[] };
  question?: string;
  intent?: string;
  retryCount?: number;
  timestamp?: number;
}

export interface ReasoningStep {
  stage: string;
  title: string;
  detail?: string;
  count?: number;
  status?: string;
}

export interface GenComponent {
  kind: 'chart' | 'table' | 'card' | 'code';
  title?: string;
  chart_option?: Record<string, unknown>;
  table_columns?: string[];
  table_rows?: unknown[][];
  card_fields?: Record<string, string>;
  code?: string;
  language?: string;
}

export function useChatMessages() {
  const messages = ref<ChatMessage[]>([]);
  const streaming = ref(false);
  const messagesContainer = ref<HTMLElement | null>(null);
  // 贴底才自动跟随；用户一往上翻就松手（记录 onMessagesScroll 的判定结果）
  const isPinnedToBottom = ref(true);

  function generateMessageId(): string {
    return Date.now().toString(36) + Math.random().toString(36).slice(2, 8);
  }

  function addMessage(message: Omit<ChatMessage, 'id'> & { id?: string }): ChatMessage {
    const msgWithId: ChatMessage = {
      ...message,
      id: message.id || generateMessageId(),
    };
    messages.value = [...messages.value, msgWithId];
    scrollToBottom();
    return msgWithId;
  }

  function updateMessage(index: number, updates: Partial<ChatMessage>) {
    if (messages.value[index]) {
      // 替换整个对象引用以触发响应式更新
      messages.value = messages.value.map((m, i) =>
        i === index ? { ...m, ...updates } : m
      );
    }
  }

  function removeMessage(index: number) {
    messages.value = messages.value.filter((_, i) => i !== index);
  }

  function clearMessages() {
    messages.value = [];
  }

  function onMessagesScroll() {
    const el = messagesContainer.value;
    if (!el) return;
    isPinnedToBottom.value = el.scrollHeight - el.scrollTop - el.clientHeight < 48;
  }

  function scrollToBottom(force = false) {
    nextTick(() => {
      const el = messagesContainer.value;
      if (!el) return;
      // 一往上翻就松手：不贴底时不拽用户回去，force（用户主动跳最新/发送消息）除外
      if (!force && !isPinnedToBottom.value) return;
      el.scrollTop = el.scrollHeight;
    });
  }

  function jumpToLatest() {
    isPinnedToBottom.value = true;
    scrollToBottom(true);
  }

  function parseThinkBlocks(content: string): { think: string; answer: string } {
    const match = content.match(/<think>([\s\S]*?)<\/think>/);
    if (match && match[1]) {
      return {
        think: match[1].trim(),
        answer: content.replace(/<think>[\s\S]*?<\/think>/, '').trim(),
      };
    }
    return { think: '', answer: content };
  }

  return {
    messages,
    streaming,
    messagesContainer,
    isPinnedToBottom,
    addMessage,
    updateMessage,
    removeMessage,
    clearMessages,
    onMessagesScroll,
    scrollToBottom,
    jumpToLatest,
    parseThinkBlocks,
  };
}
