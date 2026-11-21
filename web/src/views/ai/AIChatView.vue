<template>
  <ApPageContainer :title="$t('aiChat.k6')" :subtitle="$t('aiChat.k10')" :show-breadcrumb="true" :max-width="AppleLayout.maxWideContentWidth" compact no-scroll>
    <template #actions>
      <ApButton v-if="isMobile" variant="secondary" size="small" :aria-label="$t('aiChat.k14')" @click="sidebarOpen = true">
        <ApIcon name="Menu" :size="AppleIconSize.control" /> {{ $t('aiChat.k1') }}
      </ApButton>
      <ApButton variant="secondary" size="small" @click="router.push('/ai/graph')">
        <ApIcon name="Share2" :size="AppleIconSize.control" /> {{ $t('aiChat.k2') }}
      </ApButton>
      <ApButton variant="secondary" size="small" @click="router.push('/ai/settings')">
        <ApIcon name="Settings" :size="AppleIconSize.control" /> {{ $t('aiChat.k3') }}
      </ApButton>
    </template>
    <div :style="chatContainerStyle">
      <ChatSidebar
        v-if="!isMobile"
        :conversations="conversations"
        :active-id="activeConversationId"
        :error="conversationError"
        @new-chat="onStartNewChat"
        @select="loadConv"
        @delete="onDeleteConversation"
        @retry="fetchConversations"
      />
      <!-- 移动端把固定 220px 的会话列表收进抽屉，否则消息列会被压到不可用 -->
      <ApDrawer v-else v-model="sidebarOpen" placement="left" width="260px">
        <template #title>{{ $t('aiChat.k4') }}</template>
        <ChatSidebar
          :conversations="conversations"
          :active-id="activeConversationId"
          :error="conversationError"
          @new-chat="onDrawerNewChat"
          @select="onDrawerSelect"
          @delete="onDeleteConversation"
          @retry="fetchConversations"
        />
      </ApDrawer>

      <div :style="chatMainAreaStyle" @paste="handleImagePaste">
        <!-- role=log：追加式对话流的标准角色，读屏会把新到达的增量念出来；
             不给的话，流式回答在屏幕阅读器里等于没发生。 -->
        <div
          ref="messagesContainer"
          :style="chatMessagesContainerStyle"
          role="log"
          aria-live="polite"
          aria-busy="false"
          :aria-label="$t('aiChat.k15')"
          @scroll="onMessagesScroll"
        >
          <ChatWelcome
            v-if="messages.length === 0"
            :title="$t('aiChat.k7')"
            :subtitle="$t('aiChat.k11')"
            :prompts="quickPrompts"
            @select-prompt="sendChatMessage"
          />

          <ChatMessageBubble
            v-for="msg in messages"
            :key="msg.id"
            :message="msg"
            @retry="retryMessage(msg.id)"
            @show-sources="openSourcesModal"
            @source-click="openSourceByIndex"
            @clarify-select="(opt) => handleClarifyOption(opt, msg.id)"
            @stream-action="handleStreamAction"
            @propose-save="openProposeSave"
          />
        </div>

        <!-- 跳至最新：用户上翻后出现，点击回到底部并恢复自动跟随 -->
        <ApButton
          v-if="messages.length > 0 && !isPinnedToBottom"
          variant="secondary"
          size="small"
          :style="jumpLatestStyle"
          :aria-label="$t('aiChat.k67')"
          @click="jumpToLatest"
        >
          <ApIcon name="ArrowDown" :size="AppleIconSize.control" /> {{ $t('aiChat.k67') }}
        </ApButton>

        <div :style="voiceBarStyle">
          <ApButton :variant="recording ? 'danger' : 'secondary'" size="small" :disabled="streaming" @click="toggleVoice">
            <ApIcon name="Mic" :size="AppleIconSize.control" /> {{ recording ? $t('vAiUi.k9') : $t('vAiUi.k10') }}
          </ApButton>
          <ApButton variant="secondary" size="small" :disabled="streaming || messages.length === 0" @click="speakLastAnswer">
            <ApIcon name="Volume2" :size="AppleIconSize.control" /> {{ speaking ? $t('vAiUi.k11') : $t('vAiUi.k12') }}
          </ApButton>
          <ApButton variant="secondary" size="small" :disabled="streaming" @click="imageInputRef?.click()">
            <ApIcon name="Image" :size="AppleIconSize.control" /> {{ $t('aiChat.k5') }}
          </ApButton>
          <input ref="imageInputRef" type="file" :aria-label="$t('aiChat.k16')" accept="image/*" multiple :style="{ display: 'none' }" @change="handleImageUpload">
          <span v-if="voiceError" :style="voiceErrStyle">{{ voiceError }}</span>
        </div>

        <!-- 图片预览区 -->
        <div v-if="pendingImages.length > 0" :style="imagePreviewContainerStyle">
          <div v-for="img in pendingImages" :key="img.id" :style="imagePreviewItemStyle">
            <img :src="img.url" :style="imagePreviewImgStyle" :alt="$t('aiChat.k18')" />
            <ApButton variant="danger" icon-only size="mini" :aria-label="$t('aiChat.k17')" @click="removeImage(img.id)"><ApIcon name="X" :size="AppleIconSize.control" /></ApButton>
          </div>
        </div>

        <ChatInputBar
          v-model="chatQuestion"
          :disabled="streaming"
          :is-streaming="streaming"
          :effective-intent="effectiveIntent"
          :selected-space-id="selectedSpaceId"
          :spaces="(spaceStore.mySpaces || []).map(s => ({ id: s.id, name: s.name }))"
          @send="handleChatSend"
          @stop="handleStopStream"
          @intent-change="overrideIntent"
          @space-change="selectedSpaceId = $event"
        />
      </div>
    </div>

    <ApConfirmModal
      v-model="showNewChatConfirm"
      :title="$t('aiChat.k8')"
      :message="$t('vAiUi.k7')"
      :confirm-text="$t('aiChat.confirm')"
      @confirm="startNewChat"
    />
    <ApConfirmModal
      v-model="showDeleteConvConfirm"
      :title="$t('aiChat.k9')"
      :message="$t('vAiUi.k8')"
      :confirm-text="$t('aiChat.delete')"
      type="danger"
      @confirm="confirmDeleteConversation"
    />

    <ChatSourcesModal
      :visible="showSourcesModal"
      :sources="activeSources"
      :highlight-citation-index="activeCitationIndex"
      @update:visible="showSourcesModal = $event"
    />

    <!-- P1-4 好答案回填：把这条回答沉淀为 Wiki 知识页 -->
    <ApModal v-model="showProposeSave" :title="$t('aiUi.proposeSaveTitle')" size="medium">
      <div :style="{ display: 'flex', flexDirection: 'column', gap: cv('space-12px') }">
        <ApFormItem :label="$t('aiUi.proposeSaveHint')" required>
          <ApInput v-model="proposeTitle" :aria-label="$t('aiUi.proposeSaveHint')" />
        </ApFormItem>
        <div :style="{ fontSize: AppleType.caption.size, opacity: 0.7, lineHeight: 1.6 }">{{ proposePreview }}</div>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showProposeSave = false">{{ $t('aiSet2.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :disabled="!proposeTitle.trim() || savingProposal" @click="submitProposal">
          {{ savingProposal ? $t('aiUi.proposeSaving') : $t('aiUi.proposeSave') }}
        </ApButton>
      </template>
    </ApModal>
  </ApPageContainer>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { MAX_CHAT_IMAGE_SIZE, CHAT_IMAGE_TYPES } from '@/constants/upload-limits';
import { logger } from '@/utils/logger';
import { computed, ref, watch, onMounted, onBeforeUnmount } from 'vue';
import { useRouter } from 'vue-router';

const { t } = useI18n();
import ApButton from '@/components/apple/ApButton.vue';
import ApDrawer from '@/components/apple/ApDrawer.vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import { cv, AppleRadius, AppleSpace, AppleType, AppleLayout, AppleIconSize } from '@/utils/core/apple-tokens';
import { chatContainerStyle, chatMainAreaStyle, chatMessagesContainerStyle, chatJumpLatestStyle } from '@/styles/ai/chat-styles';
import { streamAsk, askAI } from '@/api/modules/ai/ai-chat';
import { proposeWikiAnalysis } from '@/api/modules/ai/ai-wiki';
import { recallMemories, extractMemories } from '@/api/modules/ai/ai-memory';
import { textToSQLApi } from '@/api/modules/ai/ai-data';
import type { AIAskSource } from '@/types/ai';
import { useUIStore } from '@/stores/app/ui';
import { useSpaceStore } from '@/stores/workspace/space';

import { useConversation } from '@/composables/ai/useConversation';
import { useChatMessages, type ChatMessage, type ChatSource, type ReasoningStep, type GenComponent } from '@/composables/ai/useChatMessages';
import { useIntentDetection } from '@/composables/ai/useIntentDetection';
import { useVoice } from '@/composables/ai/useVoice';
import { useMobile } from '@/composables/ui/useMobile';

import ChatSidebar from '@/components/ai/chat/ChatSidebar.vue';
import ChatWelcome from '@/components/ai/chat/ChatWelcome.vue';
import ChatMessageBubble from '@/components/ai/chat/ChatMessageBubble.vue';
import ChatInputBar from '@/components/ai/chat/ChatInputBar.vue';
import ChatSourcesModal from '@/components/ai/chat/ChatSourcesModal.vue';

// Composables
const {
  conversations, activeConversationId,
  error: conversationError,
  fetchConversations, createNewConversation, loadConversation,
  saveConversation, removeConversation, clearActive,
} = useConversation('ai');

const {
  messages, streaming, messagesContainer, isPinnedToBottom,
  addMessage, removeMessage, clearMessages,
  onMessagesScroll, scrollToBottom, jumpToLatest, parseThinkBlocks,
} = useChatMessages();

// 流式增量跟随：贴底时才追新内容（一往上翻就松手）
watch(() => messages.value[messages.value.length - 1]?.content, () => scrollToBottom());
const jumpLatestStyle = chatJumpLatestStyle;

const {
  effectiveIntent, detect, overrideIntent,
} = useIntentDetection();

const ui = useUIStore();
const spaceStore = useSpaceStore();
const router = useRouter();

const { isMobile } = useMobile();
const sidebarOpen = ref(false);

// P2-5: 语音交互（录音转写填入输入框 / TTS 朗读回答）
const { recording, speaking, transcript, error: voiceError, startRecording, stopRecording, speak } = useVoice();
function toggleVoice() {
  if (recording.value) {
    stopRecording();
  } else {
    startRecording();
  }
}
// 转写完成后自动填入输入框
watch(transcript, (t) => {
  if (t) {
    chatQuestion.value = chatQuestion.value ? chatQuestion.value + ' ' + t : t;
  }
});
// 朗读最后一条 AI 回答
function speakLastAnswer() {
  const lastMsg = [...messages.value].reverse().find(m => m.role === 'assistant' && m.content);
  if (lastMsg) {
    // 去除 markdown 标记后朗读纯文本
    const plainText = lastMsg.content.replace(/```[\s\S]*?```/g, t('uiTextMain.k13')).replace(/[*#`>_~[\]()]/g, '');
    speak(plainText.slice(0, 4000)); // 限制长度防止超时
  }
}

// Local state
const chatQuestion = ref('');
const selectedSpaceId = ref(0);
const showSourcesModal = ref(false);
const activeSources = ref<AIAskSource[]>([]);
const activeCitationIndex = ref(0);
// P1-4 好答案回填
const showProposeSave = ref(false);
const proposeTitle = ref('');
const savingProposal = ref(false);
const proposeTarget = ref<ChatMessage | null>(null);
const proposePreview = computed(() => {
  const c = proposeTarget.value?.content || '';
  return c.length > 200 ? c.slice(0, 200) + '…' : c;
});
function openProposeSave(msg: ChatMessage) {
  if (!selectedSpaceId.value) {
    ui.showToast(t('aiUi.proposeNeedSpace'), 'warning');
    return;
  }
  proposeTarget.value = msg;
  proposeTitle.value = msg.question || '';
  showProposeSave.value = true;
}
async function submitProposal() {
  const sid = selectedSpaceId.value;
  const msg = proposeTarget.value;
  const title = proposeTitle.value.trim();
  if (!sid || !msg || !title) return;
  savingProposal.value = true;
  try {
    await proposeWikiAnalysis({
      space_id: sid,
      title,
      question: msg.question || '',
      answer: msg.content,
    });
    ui.showToast(t('aiUi.proposeSaved'), 'success');
    showProposeSave.value = false;
  } catch (e) {
    logger.error('Failed to propose wiki analysis:', e);
    ui.showToast(t('aiUi.proposeSaveFailed'), 'error');
  } finally {
    savingProposal.value = false;
  }
}
const pendingImages = ref<Array<{ id: string; url: string; file?: File }>>([]);
const imageInputRef = ref<HTMLInputElement | null>(null);
const pendingClarify = ref<{ options: string[]; question: string; history: { role: string; content: string; sql: string }[] } | null>(null);
const sqlHistory = ref<{ role: string; content: string; sql: string }[]>([]);
const streamAbortController = ref<AbortController | null>(null);
const showNewChatConfirm = ref(false);
const showDeleteConvConfirm = ref(false);
const pendingDeleteConvId = ref<number | null>(null);

// 必须是 computed 而不是常量数组：数组里的字面量只求值一次，
// 切了语言这四条推荐问题还会留着原文（语言包已经换了，界面上却看不见）。
const quickPrompts = computed(() => [t('aiChat.qp1'), t('aiChat.qp2'), t('aiChat.qp3'), t('aiChat.qp4')]);

// 将 ChatMessage 序列化为可持久化的 {role, content} 格式
// 推理链、思考过程、引用来源编码到 content 中（用 JSON 元数据）
function serializeMessage(m: ChatMessage): { role: string; content: string } {
  let content = m.think ? `<think>${m.think}</think>${m.content}` : m.content;
  // 将 reasoningChain + sources 编码为隐藏的 JSON 块
  const meta: Record<string, unknown> = {};
  if (m.reasoningChain?.length) meta.rc = m.reasoningChain;
  if (m.sources?.length) meta.src = m.sources;
  if (Object.keys(meta).length > 0) {
    content = `\u200B${JSON.stringify(meta)}\u200B` + content; // 零宽空格包裹 JSON
  }
  return { role: m.role, content };
}

// 从持久化数据还原 ChatMessage
function deserializeMessage(m: { role: string; content: string }): ChatMessage {
  const text = m.content || '';
  // 尝试提取零宽空格包裹的 JSON 元数据
  const metaRe = /\u200B(\{.*?(?:"rc"|"src")\s*:.*?\})\u200B/;
  const metaMatch = text.match(metaRe);
  let reasoningChain: ReasoningStep[] | undefined;
  let sources: ChatSource[] | undefined;
  let cleanContent = text;
  if (metaMatch && metaMatch[1]) {
    try {
      const meta = JSON.parse(metaMatch[1]);
      reasoningChain = meta.rc;
      sources = meta.src;
      cleanContent = text.replace(metaRe, '');
    } catch { /* ignore */ }
  }
  const parsed = parseThinkBlocks(cleanContent);
  return {
    id: '', // id 将由 addMessage 自动生成
    role: m.role as 'user' | 'assistant',
    content: parsed.answer || cleanContent,
    think: parsed.think || '',
    status: 'success',
    reasoningChain,
    sources,
  };
}

function abortActiveStream() {
  if (streamAbortController.value) {
    streamAbortController.value.abort();
    streamAbortController.value = null;
  }
}

function handleStopStream() {
  if (!streaming.value) return;
  abortActiveStream();
  const lastAssistantIdx = [...messages.value].reverse().findIndex(m => m.role === 'assistant');
  if (lastAssistantIdx !== -1) {
    const idx = messages.value.length - 1 - lastAssistantIdx;
    const msg = messages.value[idx];
    if (msg && msg.status === 'loading') {
      const parsed = parseThinkBlocks(msg.content);
      const abortedContent = parsed.answer || msg.content;
      msg.content = abortedContent + '\n\n' + t('uiTextMain.k1');
      msg.status = 'aborted';
      messages.value = [...messages.value];
      scrollToBottom();
    }
  }
  streaming.value = false;
}

function resetConversationState() {
  pendingClarify.value = null;
  sqlHistory.value = [];
  pendingImages.value = [];
  chatQuestion.value = '';
  activeSources.value = [];
  activeCitationIndex.value = 0;
  showSourcesModal.value = false;
}

function startNewChat() {
  abortActiveStream();
  clearMessages();
  clearActive();
  resetConversationState();
  ui.showToast(t('aiChat.k20'), 'success');
}

function onStartNewChat() {
  showNewChatConfirm.value = true;
}

function onDrawerNewChat() {
  sidebarOpen.value = false;
  onStartNewChat();
}

async function onDrawerSelect(id: number) {
  sidebarOpen.value = false;
  await loadConv(id);
}

async function finalizeAssistantMessage(assistantMsg: ChatMessage, rawContent: string) {
  const parsed = parseThinkBlocks(rawContent);
  assistantMsg.think = parsed.think;
  assistantMsg.content = parsed.answer || rawContent;
  assistantMsg.status = 'success';
  if (activeConversationId.value) {
    try {
      await saveConversation(activeConversationId.value, messages.value.map(serializeMessage));
      await fetchConversations();
      if (messages.value.filter(m => m.role === 'user').length % 5 === 0) {
        extractMemories({
          conversation_id: activeConversationId.value,
          messages: messages.value.map(m => ({ role: m.role, content: m.content })),
        }).catch((e) => { logger.error('Failed to extract memories:', e); });
      }
    } catch (e) {
      logger.error('Failed to update conversation:', e);
      ui.showToast(t('aiChat.k21'), 'error');
    }
  }
  streaming.value = false;
  scrollToBottom();
}

async function failAssistantMessage(assistantMsg: ChatMessage, err: string) {
  console.error('AI reply failed:', err);
  let friendlyMsg = t('sentUi.k33');
  if (err.includes('network') || err.includes('Network') || err.includes('网络') || err.includes('timeout') || err.includes('Timeout') || err.includes('ECONNABORTED')) {
    friendlyMsg = t('sentUi.k34');
  } else if (err.includes('401') || err.includes('403') || err.includes('未授权') || err.includes('权限')) {
    friendlyMsg = t('sentUi.k35');
  } else if (err.includes('500') || err.includes('502') || err.includes('503') || err.includes('服务器')) {
    friendlyMsg = t('sentUi.k33');
  } else if (err.includes('rate limit') || err.includes('RateLimit') || err.includes('限流') || err.includes('频率')) {
    friendlyMsg = t('sentUi.k36');
  }
  assistantMsg.content = t('aiChat.k71', { p1: friendlyMsg });
  assistantMsg.status = 'failed';
  if (activeConversationId.value) {
    try {
      await saveConversation(activeConversationId.value, messages.value.map(serializeMessage));
      await fetchConversations();
    } catch (e) { logger.error('Failed to save failed conversation:', e); ui.showToast(t('aiChat.k21'), 'error'); }
  }
  streaming.value = false;
  ui.showToast(friendlyMsg, 'error');
}

// ===== Helpers =====
function findMessageIndexById(msgId: string): number {
  return messages.value.findIndex(m => m.id === msgId);
}

function buildHistory(assistantMsgId: string) {
  const idx = findMessageIndexById(assistantMsgId);
  return messages.value.slice(0, idx).map(m => ({ id: m.id, role: m.role, content: m.content }));
}

function startStream(question: string, assistantMsg: ChatMessage, history: ChatMessage[], prefix = '', images?: string[]) {
  let inThinkBlock = false;
  let thinkContent = '';
  let answerContent = '';
  let buffer = '';
  const THINK_OPEN = '<think>';
  const THINK_CLOSE = '</think>';

  function processChunk(chunk: string) {
    buffer += chunk;
    let changed = false;

    while (buffer.length > 0) {
      if (!inThinkBlock) {
        const idx = buffer.indexOf(THINK_OPEN);
        if (idx !== -1) {
          answerContent += buffer.slice(0, idx);
          buffer = buffer.slice(idx + THINK_OPEN.length);
          inThinkBlock = true;
          changed = true;
        } else {
          const keep = Math.min(buffer.length, THINK_OPEN.length - 1);
          const processLen = buffer.length - keep;
          if (processLen > 0) {
            answerContent += buffer.slice(0, processLen);
            buffer = buffer.slice(processLen);
            changed = true;
          }
          break;
        }
      } else {
        const idx = buffer.indexOf(THINK_CLOSE);
        if (idx !== -1) {
          thinkContent += buffer.slice(0, idx);
          buffer = buffer.slice(idx + THINK_CLOSE.length);
          inThinkBlock = false;
          changed = true;
        } else {
          const keep = Math.min(buffer.length, THINK_CLOSE.length - 1);
          const processLen = buffer.length - keep;
          if (processLen > 0) {
            thinkContent += buffer.slice(0, processLen);
            buffer = buffer.slice(processLen);
            changed = true;
          }
          break;
        }
      }
    }

    if (changed) {
      assistantMsg.think = thinkContent.trim();
      assistantMsg.content = prefix + answerContent;
    }
  }

  abortActiveStream();
  streamAbortController.value = streamAsk(
    { question, limit: 10, history, images },
    (chunk) => {
      processChunk(chunk);
    },
    async () => {
      streamAbortController.value = null;
      if (inThinkBlock) {
        thinkContent += buffer;
      } else {
        answerContent += buffer;
      }
      const fullThink = thinkContent.trim();
      const fullAnswer = answerContent;
      const rawContent = fullThink ? `<think>${fullThink}</think>${fullAnswer}` : fullAnswer;
      finalizeAssistantMessage(assistantMsg, prefix + rawContent);
    },
    (err) => { streamAbortController.value = null; failAssistantMessage(assistantMsg, err); },
    undefined,
    (meta: {
      tokenUsage: number;
      durationMs: number;
      sources?: unknown[];
      reasoningChain?: unknown[];
      components?: unknown[];
    }) => {
      if (meta.sources?.length) assistantMsg.sources = meta.sources as ChatSource[];
      if (meta.reasoningChain?.length) {
        Object.assign(assistantMsg, { reasoningChain: meta.reasoningChain as ReasoningStep[] });
      }
      if (meta.components?.length) assistantMsg.components = meta.components as GenComponent[];
    },
  );
}

function formatSQLResult(result: { sql?: string; explanations?: string[]; warnings?: string[]; explanation?: string }) {
  const parts: string[] = [];
  if (result.sql) parts.push(`\`\`\`sql\n${result.sql}\n\`\`\``);
  if (result.explanations?.length) parts.push(result.explanations.join('\n'));
  if (result.warnings?.length) parts.push(t('aiChat.k72') + result.warnings.join('；'));
  return parts.join('\n\n') || t('fallback.k55');
}

async function executeSQLQuery(question: string, assistantMsg: ChatMessage, history: { role: string; content: string; sql: string }[]) {
  const result = await textToSQLApi.generateSQL({ question, history, space_id: selectedSpaceId.value || undefined });
  // 仅在确实返回了非空 options 时才走澄清分支，否则一律生成 SQL
  if (result?.clarify?.needed && result.clarify.options && result.clarify.options.length > 0) {
    const clarify = { options: result.clarify.options, question, history: [...history] };
    pendingClarify.value = clarify;
    assistantMsg.clarify = clarify;
    assistantMsg.content = t('aiChat.k73');
    assistantMsg.status = 'success';
    assistantMsg.question = question;
    streaming.value = false;
    scrollToBottom();
    return true;
  }
  sqlHistory.value.push({ role: 'user', content: question, sql: '' });
  if (result?.sql) sqlHistory.value.push({ role: 'assistant', content: result.explanation || '', sql: result.sql });
  assistantMsg.content = formatSQLResult(result);
  // P0: 将后端的 CoT Steps + 原始CoT + 修正对比映射为 reasoningChain 展示
  const reasoningSteps: ReasoningStep[] = [];
  if (result?.steps && result.steps.length > 0) {
    for (const s of result.steps) {
      reasoningSteps.push({
        stage: s.action,
        title: s.observation || s.thought,
        detail: s.thought,
        status: 'success' as const,
      });
    }
  }
  // 追加 selfReflect 修正对比步骤
  if (result?.self_reflect_before && result?.self_reflect_after &&
      result.self_reflect_before !== result.self_reflect_after) {
    reasoningSteps.push({
      stage: 'self_reflect',
      title: t('aiChat.k19'),
      detail: t('uiTextMain.k2', { p1: result.self_reflect_before.slice(0, 80), p2: result.self_reflect_after.slice(0, 80) }),
      status: 'success' as const,
    });
  }
  if (reasoningSteps.length > 0) {
    Object.assign(assistantMsg, { reasoningChain: reasoningSteps });
  }
  assistantMsg.status = 'success';
  return false;
}

// ===== Intent dispatch =====
async function executeByIntent(question: string, assistantMsgId: string, intent: string, images?: string[]) {
  const assistantMsg = messages.value.find(m => m.id === assistantMsgId);
  if (!assistantMsg) return;
  assistantMsg.status = 'loading';
  assistantMsg.content = '';
  scrollToBottom();

  if (!activeConversationId.value) {
    const id = await createNewConversation(question);
    if (!id) {
      ui.showToast(t('aiChat.k22'), 'error');
      assistantMsg.status = 'failed';
      assistantMsg.content = t('aiChat.k22');
      streaming.value = false;
      return;
    }
  }

  if (intent === 'data_query') {
    try { await executeSQLQuery(question, assistantMsg, sqlHistory.value); } catch (e: unknown) {
      assistantMsg.content = t('aiChat.k74', { p1: (e as { message?: string }).message || String(e) });
      assistantMsg.status = 'failed';
    }
    if (activeConversationId.value) {
      try {
        await saveConversation(activeConversationId.value, messages.value.map(serializeMessage));
        await fetchConversations();
      } catch (e) { logger.error('Failed to save SQL conversation:', e); }
    }
    streaming.value = false;
    scrollToBottom();
  } else if (intent === 'agent') {
    // P0-2/P0-3：多智能体编排 + 规划反思闭环（非流式，后端返回完整答案）
    assistantMsg.content = '> ' + t('aiChat.k68') + '\n\n';
    try {
      const res = await askAI({ question, space_id: selectedSpaceId.value || undefined, limit: 10, history: buildHistory(assistantMsgId), mode: 'agent', images });
      assistantMsg.content = res.answer || t('fallback.k1');
      if (res.sources) assistantMsg.sources = res.sources as ChatSource[];
      assistantMsg.status = 'success';
    } catch (e: unknown) {
      assistantMsg.content = t('aiChat.k70', { p1: (e as { message?: string }).message || String(e) });
      assistantMsg.status = 'failed';
    }
    streaming.value = false;
    scrollToBottom();
    if (activeConversationId.value) {
      try { await saveConversation(activeConversationId.value, messages.value.map(serializeMessage)); } catch (e) { logger.error('save agent conv:', e); }
    }
    return;
  } else if (intent === 'research') {
    assistantMsg.content = '> ' + t('aiChat.k69') + '\n\n';
    startStream(question, assistantMsg, buildHistory(assistantMsgId), assistantMsg.content, images);
  } else if (intent === 'wiki_query') {
    // Wiki 知识库页面查询：优先检索已编译的 Wiki 页面
    assistantMsg.content = '> ' + t('aiChat.k75') + '\n\n';
    startStream(question, assistantMsg, buildHistory(assistantMsgId), assistantMsg.content, images);
  } else {
    // P2-6: 个性化记忆由后端在 StreamAsk 中自动召回并注入 prompt，
    // 前端仅做召回调用以触发记忆统计/可观测，不再伪装成历史消息。
    try { await recallMemories({ query: question, limit: 5 }); } catch (e) { logger.error('Failed to recall memories:', e); }
    const history = buildHistory(assistantMsgId);
    startStream(question, assistantMsg, history, '', images);
  }
}

async function handleClarifyOption(option: string, _msgId: string) {
  if (!pendingClarify.value) return;
  const clarify = pendingClarify.value;
  addMessage({ role: 'user', content: t('uiTextMain.k3', { p1: option }), status: 'success' });
  // 将已选选项累积到原始问题中，避免后端每次都看到原始模糊问题而重复澄清
  const enrichedQuestion = t('uiTextMain.k4', { p1: clarify.question, p2: option });
  const assistantMsg = addMessage({ role: 'assistant', content: '', think: '', status: 'loading', question: enrichedQuestion });
  streaming.value = true;

  const history = [...clarify.history];
  history.push({ role: 'user', content: clarify.question, sql: '' });
  history.push({ role: 'assistant', content: t('uiTextMain.k5', { p1: clarify.options.join(' / ') }), sql: '' });
  history.push({ role: 'user', content: t('uiTextMain.k3', { p1: option }), sql: '' });
  pendingClarify.value = null;

  try {
    const result = await textToSQLApi.generateSQL({ question: enrichedQuestion, history });
    // 仅在确实返回了非空 options 时才走澄清分支
    if (result?.clarify?.needed && result.clarify.options && result.clarify.options.length > 0) {
      const nextClarify = { options: result.clarify.options, question: enrichedQuestion, history };
      pendingClarify.value = nextClarify;
      assistantMsg.clarify = nextClarify;
      assistantMsg.content = t('uiTextMain.k6');
      assistantMsg.status = 'success';
    } else {
      // 后端没要求澄清，或返回了空 options：直接走 SQL 生成
      sqlHistory.value.push({ role: 'user', content: clarify.question, sql: '' });
      if (result?.sql) sqlHistory.value.push({ role: 'assistant', content: result.explanation || '', sql: result.sql });
      assistantMsg.content = formatSQLResult(result);
      const reasoningSteps2: ReasoningStep[] = [];
      if (result?.steps && result.steps.length > 0) {
        for (const s of result.steps) {
          reasoningSteps2.push({
            stage: s.action,
            title: s.observation || s.thought,
            detail: s.thought,
            status: 'success' as const,
          });
        }
      }
      if (result?.self_reflect_before && result?.self_reflect_after &&
          result.self_reflect_before !== result.self_reflect_after) {
        reasoningSteps2.push({
          stage: 'self_reflect',
          title: t('aiChat.k19'),
          detail: t('uiTextMain.k2', { p1: result.self_reflect_before.slice(0, 80), p2: result.self_reflect_after.slice(0, 80) }),
          status: 'success' as const,
        });
      }
      if (reasoningSteps2.length > 0) {
        Object.assign(assistantMsg, { reasoningChain: reasoningSteps2 });
      }
      assistantMsg.status = 'success';
    }
  } catch (e: unknown) {
    assistantMsg.content = t('uiTextMain.k7', { p1: (e as { message?: string }).message || String(e) });
    assistantMsg.status = 'failed';
  }
  streaming.value = false;
  scrollToBottom();
  // 保存对话：每次澄清交互后持久化 messages
  if (activeConversationId.value) {
    try {
      await saveConversation(activeConversationId.value, messages.value.map(serializeMessage));
      await fetchConversations();
    } catch (e) { logger.error('Failed to save clarify conversation:', e); }
  }
}

// ===== Send =====
async function sendChatMessage(text?: string) {
  const q = (text || chatQuestion.value).trim();
  const images = pendingImages.value.map(img => img.url);
  if ((!q && images.length === 0) || streaming.value) return;
  pendingImages.value = [];
  addMessage({ role: 'user', content: q, status: 'success' });
  jumpToLatest(); // 用户自己发的消息，无论翻到哪儿都回到底部看回答
  if (!text) chatQuestion.value = '';
  const intent = await detect(q);
  const assistantMsg = addMessage({ role: 'assistant', content: '', think: '', status: 'loading', question: q, intent });
  streaming.value = true;
  await executeByIntent(q, assistantMsg.id, intent, images);
}

function handleChatSend() { sendChatMessage(); }

// ===== Retry =====
const MAX_RETRY_COUNT = 3;

async function retryMessage(msgId: string) {
  if (streaming.value) return;
  const msgIndex = findMessageIndexById(msgId);
  if (msgIndex === -1) return;
  const failedMsg = messages.value[msgIndex];
  if (!failedMsg) return;
  const retryCount = failedMsg.retryCount || 0;
  if (retryCount >= MAX_RETRY_COUNT) {
    ui.showToast(t('uiTextMain.k8', { p1: MAX_RETRY_COUNT }), 'error');
    return;
  }
  const userMsg = messages.value.slice(0, msgIndex).reverse().find(m => m.role === 'user');
  const question = userMsg?.content || failedMsg.question || t('fallback.k101');
  const intent = failedMsg.intent || effectiveIntent.value;
  removeMessage(msgIndex);
  const assistantMsg = addMessage({
    role: 'assistant',
    content: '',
    think: '',
    status: 'loading',
    question,
    intent,
    retryCount: retryCount + 1,
  });
  streaming.value = true;
  await executeByIntent(question, assistantMsg.id, intent);
}

// ===== Conversation management =====
async function loadConv(id: number) {
  abortActiveStream();
  streaming.value = false;
  const conv = await loadConversation(id);
  if (!conv) {
    ui.showToast(t('aiChat.k23'), 'error');
    return;
  }
  clearMessages();
  resetConversationState();
  const msgs = (conv.messages && Array.isArray(conv.messages)) ? conv.messages : [];
  logger.info(`[loadConv] id=${id} messages_count=${msgs.length}`);

  // 防止对话历史出现连续 0 条：若后端返回空，提示用户
  if (msgs.length === 0) {
    ui.showToast(t('aiChat.k24'), 'info');
  }

  msgs.forEach((m: { role: string; content: string }) => {
    if (!m || typeof m.role !== 'string') return;
    addMessage(deserializeMessage(m));
  });
  activeConversationId.value = id;
  // 等 DOM 更新后滚动到底部
  setTimeout(() => scrollToBottom(), 50);
}

function onDeleteConversation(id: number) {
  pendingDeleteConvId.value = id;
  showDeleteConvConfirm.value = true;
}

async function confirmDeleteConversation() {
  const id = pendingDeleteConvId.value;
  if (id === null) return;
  abortActiveStream();
  try {
    await removeConversation(id);
    if (activeConversationId.value === null) startNewChat();
    ui.showToast(t('aiChat.k25'), 'success');
  } catch {
    // removeConversation 原先把错误吞掉只 log，这里就无条件弹成功 —— 删除失败被报成「已删除对话」。
    // 弹窗收起来放 finally：失败后它还留着的话，用户会以为是自己没点中确认按钮。
    ui.showToast(t('aiChat.k26'), 'error');
  } finally {
    showDeleteConvConfirm.value = false;
  }
}

function openSourcesModal(sources: unknown[]) {
  activeSources.value = sources as AIAskSource[];
  activeCitationIndex.value = 0;
  showSourcesModal.value = true;
}

function openSourceByIndex(sources: unknown[], citationIndex: number) {
  activeSources.value = sources as AIAskSource[];
  activeCitationIndex.value = citationIndex;
  showSourcesModal.value = true;
}

// ===== Image Upload =====

const MAX_IMAGE_COUNT = 9;


function generatePendingImageId(): string {
  return Date.now().toString(36) + Math.random().toString(36).slice(2, 8);
}

function validateImageFile(file: File): boolean {
  if (!CHAT_IMAGE_TYPES.includes(file.type)) {
    ui.showToast(t('uiTextMain.k9', { p1: file.name }), 'error');
    return false;
  }
  if (file.size > MAX_CHAT_IMAGE_SIZE) {
    ui.showToast(t('uiTextMain.k10', { p1: file.name, p2: MAX_CHAT_IMAGE_SIZE / 1024 / 1024 }), 'error');
    return false;
  }
  if (pendingImages.value.length >= MAX_IMAGE_COUNT) {
    ui.showToast(t('uiTextMain.k11', { p1: MAX_IMAGE_COUNT }), 'error');
    return false;
  }
  return true;
}

function addPendingImage(file: File) {
  if (!validateImageFile(file)) return;
  const reader = new FileReader();
  reader.onload = () => {
    pendingImages.value.push({ id: generatePendingImageId(), url: reader.result as string, file });
  };
  reader.readAsDataURL(file);
}

function handleImageUpload(e: Event) {
  const files = (e.target as HTMLInputElement).files;
  if (!files) return;
  const fileArray = Array.from(files);
  const remainingSlots = MAX_IMAGE_COUNT - pendingImages.value.length;
  if (remainingSlots <= 0) {
    ui.showToast(t('uiTextMain.k11', { p1: MAX_IMAGE_COUNT }), 'warning');
    (e.target as HTMLInputElement).value = '';
    return;
  }
  if (fileArray.length > remainingSlots) {
    ui.showToast(t('uiTextMain.k12', { p1: MAX_IMAGE_COUNT, p2: remainingSlots }), 'info');
  }
  for (const file of fileArray.slice(0, remainingSlots)) {
    if (!file.type.startsWith('image/')) continue;
    addPendingImage(file);
  }
  (e.target as HTMLInputElement).value = '';
}

function removeImage(id: string) {
  pendingImages.value = pendingImages.value.filter(img => img.id !== id);
}

function handleImagePaste(e: ClipboardEvent) {
  const items = e.clipboardData?.items;
  if (!items) return;
  for (const item of Array.from(items)) {
    if (!item.type.startsWith('image/')) continue;
    const file = item.getAsFile();
    if (file) {
      addPendingImage(file);
    }
  }
}

// ===== Stream Components =====
function handleStreamAction(action: { action: string; payload: Record<string, unknown>; compId: string }) {
  logger.info('[StreamComponent] action:', action);
}

// ===== Lifecycle =====
onMounted(() => {
  fetchConversations();
  spaceStore.loadMySpaces();
});
onBeforeUnmount(() => { abortActiveStream(); });

// ===== Styles =====

// P2-5: 语音交互栏样式
const voiceBarStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace.xs,
  marginTop: AppleSpace.xs, padding: AppleSpace.xxs,
};

const voiceErrStyle = {
  fontSize: AppleType.legalMeta.size, color: cv('red'),
};

const imagePreviewContainerStyle = {
  display: 'flex', flexWrap: 'wrap' as const, gap: AppleSpace.xxs,
  padding: `${AppleSpace.xxs} ${AppleSpace.sm}`, borderTop: `1px solid ${cv('border-light')}`,
};
const imagePreviewItemStyle = {
  width: '56px', height: '56px', borderRadius: AppleRadius.small,
  overflow: 'hidden' as const, position: 'relative' as const,
  border: `1px solid ${cv('border-light')}`,
};
const imagePreviewImgStyle = {
  width: '100%', height: '100%', objectFit: 'cover' as const,
};
</script>
