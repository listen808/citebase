<template>
  <div :style="bubbleStyle(message.role)">
    <div :style="avatarStyle(message.role)">
      <ApIcon :name="message.role === 'user' ? 'User' : 'Bot'" :size="18" />
    </div>
    <div :style="contentWrapperStyle">
      <div :style="contentStyle">
      <div v-if="formattedTime" :style="timeStyle(message.role)">
        {{ formattedTime }}
      </div>
      <!-- ChatGPT 风格 Thinking 折叠区（推理链内联展示） -->
      <div v-if="message.role === 'assistant' && message.reasoningChain?.length && message.status === 'success'" :style="thinkingBlockStyle">
        <div :style="thinkingHeaderStyle" role="button" :aria-expanded="showThinking" @click="showThinking = !showThinking">
          <ApIcon name="Brain" :size="14" :style="{ color: cv('text-tertiary') }" />
          <span :style="thinkingTitleStyle">思考过程 · {{ message.reasoningChain.length }} 步</span>
          <span :style="thinkingBadgeStyle">{{ showThinking ? $t('aiUi.k12') : $t('aiUi.k13') }}</span>
          <ApIcon name="ChevronDown" :size="14" :style="{ color: cv('text-quaternary'), transform: showThinking ? 'rotate(180deg)' : 'none', transition: 'transform var(--apple-dur-interactive)', marginLeft: 'auto' }" />
        </div>
        <div v-if="showThinking" :style="thinkingBodyStyle">
          <div v-for="(step, i) in message.reasoningChain" :key="i" :style="thinkingStepStyle">
            <span :style="thinkingStepNumStyle">{{ i + 1 }}</span>
            <div :style="thinkingStepContentStyle">
              <div :style="thinkingStepTitleStyle">{{ step.title }}</div>
              <div v-if="step.detail" :style="thinkingStepDetailStyle">{{ step.detail }}</div>
            </div>
          </div>
        </div>
      </div>

      <ChatThinkBlock v-if="message.role === 'assistant' && message.think" :content="message.think" />

      <template v-if="message.status === 'loading'">
        <div :style="loadingContainerStyle">
          <div :style="typingStyle">
            <span :style="dotStyle(0)" /><span :style="dotStyle(1)" /><span :style="dotStyle(2)" />
          </div>
          <div :style="loadingTextStyle">
            {{ currentLoadingStep || $t('fallback.k85') }}
          </div>
        </div>
      </template>

      <template v-else>
        <div ref="textContentRef" class="chat-md-content" :style="textStyle" @click="handleTextClick" @mouseover="handleTextMouseOver" @mouseout="handleTextMouseOut" v-html="renderMarkdown(message.content)" />
      </template>

      <!-- Citation hover tooltip -->
      <Teleport to="body">
        <div v-if="tooltipVisible && tooltipSource" class="apple-glass-layer" :style="tooltipStyle">
          <div :style="{ display: 'flex', alignItems: 'center', gap: AppleSpace.xs, marginBottom: AppleSpace.xs }">
            <span :style="{ display:'inline-flex',alignItems:'center',justifyContent:'center',minWidth:'20px',height:'18px',padding:`0 ${AppleSpace.xs}`,borderRadius:AppleRadius['5px'],background:cv('blue'),color: cv('on-accent'),fontSize:AppleType.microUI.size,letterSpacing:AppleType.microUI.letterSpacing,fontWeight:String(AppleFontWeight.semibold) }">
              [{{ tooltipSource.citation_index }}]
            </span>
            <span style="font-weight:500">{{ tooltipSource.document_name }}</span>
            <span v-if="tooltipSource.verify_status === 'cited'" :style="{ display:'inline-flex', alignItems:'center', gap:AppleSpace.xxs, fontSize:AppleType.microUI.size,letterSpacing:AppleType.microUI.letterSpacing,color:cv('green') }"><ApIcon name="Check" :size="12" />{{ $t('aiChat.cited') }}</span>
          </div>
          <div v-if="tooltipSource.heading_path" :style="{ fontSize:AppleType.microUI.size,letterSpacing:AppleType.microUI.letterSpacing,color:cv('text-tertiary'),marginBottom:AppleSpace.xs }">
            {{ tooltipSource.heading_path }}
          </div>
          <div :style="{ color:cv('text-secondary'),display:'-webkit-box',WebkitLineClamp:'3',WebkitBoxOrient:'vertical',overflow:'hidden' }">
            {{ tooltipSource.snippet }}
          </div>
          <div v-if="tooltipSource.score" :style="{ fontSize:AppleType.microUI.size,letterSpacing:AppleType.microUI.letterSpacing,color:cv('text-tertiary'),marginTop:AppleSpace.xs }">
            相关度 {{ Math.round(tooltipSource.score * 100) }}%
          </div>
          <div v-if="tooltipSource.paragraph_id" :style="{ fontSize:AppleType.microUI.size,letterSpacing:AppleType.microUI.letterSpacing,color:cv('text-tertiary'),marginTop:AppleSpace.xxs }">
            ¶{{ tooltipSource.paragraph_id }}
          </div>
        </div>
      </Teleport>

      <!-- 澄清选项按钮（内联渲染，避免插槽嵌套问题） -->
      <div
        v-if="message.clarify?.options?.length"
        :style="clarifyOptionsStyle"
        role="group"
        :aria-label="'澄清选项'"
      >
        <ApButton
          v-for="opt in message.clarify.options"
          :key="opt"
          variant="outline"
          size="small"
          @click="emit('clarify-select', opt)"
        >
          {{ opt }}
        </ApButton>
      </div>

      <slot />

      <GeneratedComponentRenderer
        v-if="message.components?.length"
        :components="message.components as GenComponent[]"
        :style="{ marginTop: AppleSpace.xs }"
      />

      <StreamRenderer
        v-if="message.streamComponents?.length"
        :components="message.streamComponents as StreamComponent[]"
        :style="{ marginTop: AppleSpace.xs }"
        @action="handleStreamAction"
      />

      <!-- 底部推理链（备用，默认隐藏；主力展示在正文上方 ChatGPT 风格 thinking 区块） -->
      <div v-if="false && message.reasoningChain?.length" :style="chainStyle">
        <div :style="chainHeaderStyle" @click="showChain = !showChain">
          <ApIcon name="Brain" :size="14" />
          <span>思考过程 · {{ message.reasoningChain!.length }} 步</span>
          <ApIcon name="ChevronRight" :size="12" :style="{ transform: showChain ? 'rotate(90deg)' : 'none', transition: 'transform var(--apple-dur-fast)' }" />
        </div>
        <div v-if="showChain" :style="chainBodyStyle">
          <div v-for="(step, i) in message.reasoningChain" :key="i" :style="chainStepStyle">
            <span :style="chainDotStyle(step.status)"></span>
            <div :style="chainStepContentStyle">
              <div :style="chainStepTitleStyle">
                <span :style="stepNumStyle">{{ i + 1 }}</span>
                {{ step.title }}
              </div>
              <div v-if="step.detail" :style="chainStepDetailStyle">{{ step.detail }}</div>
              <div v-if="(step as any).action" :style="chainStepActionStyle">{{ (step as any).action }}</div>
            </div>
          </div>
        </div>
      </div>

      <div v-if="message.sources?.length" :style="sourcesStyle">
        <ApButton variant="secondary" size="small" @click="$emit('show-sources', message.sources!)">
          <ApIcon name="BookOpen" :size="16" /> {{ message.sources.length }} {{ $t('cExtraUi.k2') }}
        </ApButton>
      </div>

      <div v-if="message.role === 'assistant' && message.status === 'success'" :style="speakRowStyle">
        <ApButton variant="ghost" size="small" @click="copyMessageContent">
          <ApIcon :name="copied ? 'Check' : 'Copy'" :size="16" /> {{ copied ? $t('aiUi.k14') : $t('aiUi.copy') }}
        </ApButton>
        <ApButton variant="ghost" size="small" :disabled="speaking" @click="speakAnswer">
          <ApIcon name="Volume2" :size="16" /> {{ speaking ? $t('aiUi.k16') : $t('aiUi.k17') }}
        </ApButton>
        <!-- P0-4：点赞点踩回流失败样本库。此前 submitFeedback 是孤儿函数，问答链路没有任何反馈入口 -->
        <ApButton
          variant="ghost" size="small" :disabled="feedbackSent !== null"
          :aria-pressed="feedbackSent === 'positive'"
          @click="sendFeedback(1)"
        >
          <ApIcon name="ThumbsUp" :size="16" :style="{ color: feedbackSent === 'positive' ? cv('green') : undefined }" />
          {{ feedbackSent === 'positive' ? $t('aiUi.k18') : $t('aiUi.k19') }}
        </ApButton>
        <ApButton
          variant="ghost" size="small" :disabled="feedbackSent !== null"
          :aria-pressed="feedbackSent === 'negative'"
          @click="sendFeedback(-1)"
        >
          <ApIcon name="ThumbsDown" :size="16" :style="{ color: feedbackSent === 'negative' ? cv('red') : undefined }" />
          {{ feedbackSent === 'negative' ? $t('aiUi.k20') : $t('aiUi.k21') }}
        </ApButton>
        <!-- P1-4 好答案回填：把这条回答沉淀为 Wiki 知识页（走审核流） -->
        <ApButton variant="ghost" size="small" :disabled="saving" @click="emit('propose-save', props.message)">
          <ApIcon name="BookmarkPlus" :size="16" /> {{ $t('aiUi.proposeSave') }}
        </ApButton>
      </div>

      <div v-if="message.status === 'failed' || message.status === 'aborted'" :style="errorStyle">
        <ApButton variant="danger" size="small" @click="$emit('retry')">
          <ApIcon name="RefreshCw" :size="16" /> {{ $t('aiChat.k52') }}
        </ApButton>
      </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { copyText } from '@/utils/clipboard';
import ApButton from '@/components/apple/ApButton.vue';
import ChatThinkBlock from './ChatThinkBlock.vue';
import GeneratedComponentRenderer from './GeneratedComponentRenderer.vue';
import StreamRenderer from '../stream/StreamRenderer.vue';
import { AppleZIndex, cv, AppleSpace, AppleType, AppleRadius, AppleFont, AppleFontWeight, AppleLineHeight, menuSurfaceStyle } from '@/utils/core/apple-tokens';
import { ref, watch, nextTick, onMounted, onUnmounted, computed } from 'vue';
import { useVoice } from '@/composables/ai/useVoice';
import { useMarkdownRenderer } from '@/composables/editor/useMarkdownRenderer';
import { submitFeedback } from '@/api/modules/ai/ai-chat';
import { useToast } from '@/utils/toast';
import type { ChatMessage, ChatSource, StreamComponent, GenComponent } from '@/composables/ai/useChatMessages';

import { formatDateTime } from '@/utils/format/format-time';
import { sanitizeMarkdownHtml } from '@/utils/core/markdown-sanitize';

const props = defineProps<{
  message: ChatMessage;
}>();

const emit = defineEmits<{
  'retry': [];
  'show-sources': [sources: ChatSource[]];
  'source-click': [sources: ChatSource[], citationIndex: number];
  'clarify-select': [option: string];
  'stream-action': [action: { action: string; payload: Record<string, any>; compId: string }];
  'propose-save': [message: ChatMessage];
}>();

const saving = ref(false);
const { t } = useI18n();

const formattedTime = computed(() => {
  if (!props.message.timestamp) return '';
  return formatDateTime(props.message.timestamp, 'HH:mm');
});

const currentLoadingStep = computed(() => {
  if (props.message.status !== 'loading') return '';
  if (props.message.reasoningChain && props.message.reasoningChain.length > 0) {
    const lastStep = props.message.reasoningChain.at(-1);
    return lastStep?.title || lastStep?.stage || '';
  }
  return '';
});

function handleStreamAction(action: { action: string; payload: Record<string, any>; compId: string }) {
  emit('stream-action', action);
}

const showChain = ref(false);
const showThinking = ref(false); // ChatGPT 风格 thinking 折叠

const textContentRef = ref<HTMLElement | null>(null);
const copied = ref(false);

function copyMessageContent() {
  const text = (props.message.content || '').replace(/<think>[\s\S]*?<\/think>/g, '').trim();
  copyText(text).then((ok) => {
    if (!ok) return;
    copied.value = true;
    setTimeout(() => { copied.value = false; }, 1500);
  });
}

// Citation hover tooltip
const tooltipVisible = ref(false);
const tooltipStyle = ref<Record<string, string>>({});
const tooltipSource = ref<ChatSource | null>(null);
let tooltipTimer: ReturnType<typeof setTimeout> | null = null;

function showCitationTooltip(e: MouseEvent, citationIdx: number) {
  if (tooltipTimer) { clearTimeout(tooltipTimer); tooltipTimer = null; }
  const sources = props.message.sources;
  if (!sources?.length) return;
  const source = sources.find(s => s.citation_index === citationIdx) || sources[citationIdx - 1];
  if (!source) return;
  tooltipSource.value = source;
  const rect = (e.target as HTMLElement).getBoundingClientRect();
  tooltipStyle.value = menuSurfaceStyle({
    position: 'fixed',
    // 引文预览是一块浮在页面之上的面板，所以走全站唯一那张浮层表面。原先它自己写了
    // elevated + border-light + level3 + medium 圆角一份 —— 那是第二张浮层表面，
    // 而 dark 实心下 `--apple-border-light` 与 `--apple-surface-elevated` 同为 #2c2c2e，
    // 这块预览在暗色里根本没有可见边缘（同一个缺陷在 10 个菜单上收过一次，漏了这里）。
    // zIndex 保留 tooltip：弹窗打开时鼠标不会在遮罩后面悬停引文，所以它压在 modal 之上无害，
    // 而压在 dropdown 之下会让"从下拉里点开引文"这条路看不到预览。
    zIndex: String(AppleZIndex.tooltip),
    left: `${Math.min(rect.left, window.innerWidth - 340)}px`,
    top: `${rect.bottom + 8}px`,
    maxWidth: '320px',
    padding: `${AppleSpace.sm} ${AppleSpace.md}`,
    fontSize: AppleType.microUI.size,
    color: cv('text-primary'),
    pointerEvents: 'none',
    lineHeight: AppleLineHeight.loose,
  });
  tooltipVisible.value = true;
}

function hideCitationTooltip() {
  tooltipTimer = setTimeout(() => {
    tooltipVisible.value = false;
    tooltipSource.value = null;
  }, 300);
}

onUnmounted(() => {
  if (tooltipTimer) clearTimeout(tooltipTimer);
});

// P2-5: 朗读回答
const { speak, speaking, stopSpeaking } = useVoice();
function speakAnswer() {
  if (speaking.value) { stopSpeaking(); return; }
  const text = (props.message.content || '').replace(/<think>[\s\S]*?<\/think>/g, '').trim();
  if (text) speak(text);
}

// ─── P0-4: 点赞点踩 → 失败样本库 ───────────────────────────────
const feedbackSent = ref<'positive' | 'negative' | null>(null);
const feedbackSending = ref(false);
const { success, error } = useToast();

async function sendFeedback(rating: 1 | -1) {
  if (feedbackSending.value || feedbackSent.value !== null) return;
  feedbackSending.value = true;
  try {
    await submitFeedback({
      question: stripThink(props.message.content),
      answer: props.message.content,
      rating,
      reason: '',
    });
    feedbackSent.value = rating === 1 ? 'positive' : 'negative';
    success(rating === 1 ? '感谢反馈' : '已收到反馈，我们会改进');
  } catch {
    error('反馈提交失败');
  } finally {
    feedbackSending.value = false;
  }
}

function stripThink(text?: string) {
  return (text || '').replace(/<think>[\s\S]*?<\/think>/g, '').trim();
}

function handleTextClick(e: MouseEvent) {
  const copyBtn = (e.target as HTMLElement).closest('.code-copy-btn') as HTMLElement | null;
  if (copyBtn) {
    e.preventDefault();
    e.stopPropagation();
    const wrapper = copyBtn.closest('.code-block-wrapper');
    const codeLines = wrapper?.querySelectorAll('.code-content');
    let text: string;
    if (codeLines?.length) {
      text = Array.from(codeLines).map(el => el.textContent || '').join('\n');
    } else {
      text = wrapper?.querySelector('code')?.textContent || '';
    }
    copyText(text).then((ok) => {
      if (!ok) return;
      copyBtn.textContent = t('aiUi.k14');
      copyBtn.classList.add('copied');
      setTimeout(() => { copyBtn.textContent = t('aiUi.copy'); copyBtn.classList.remove('copied'); }, 1500);
    });
    return;
  }
  const link = (e.target as HTMLElement).closest('.chat-citation-link') as HTMLElement | null;
  if (!link) return;
  e.preventDefault();
  e.stopPropagation();
  const idx = parseInt(link.dataset.citationIdx || '0');
  if (idx > 0 && props.message.sources?.length) {
    emit('source-click', props.message.sources, idx);
  }
}

function handleTextMouseOver(e: MouseEvent) {
  const link = (e.target as HTMLElement).closest('.chat-citation-link') as HTMLElement | null;
  if (!link) return;
  const idx = parseInt(link.dataset.citationIdx || '0');
  if (idx > 0) showCitationTooltip(e, idx);
}

function handleTextMouseOut(e: MouseEvent) {
  const related = e.relatedTarget as HTMLElement | null;
  if (related?.closest('.chat-citation-link')) return;
  hideCitationTooltip();
}


const { render: renderMd, highlightCodeBlocks, removeCodeBlockListeners } = useMarkdownRenderer();

// P1-25: 流式期间每个 delta 都会整体替换 v-html，旧一轮异步高亮若在替换后才完成，
// 会在已脱离文档的 pre 上操作（parentElement 为 null 抛错）并做无谓的 O(n²) 重高亮。
// 参照 ApMarkdownPreview 的 highlightToken 取消模式 + try/catch。
let highlightToken = 0;

watch(() => props.message.content, async () => {
  const token = ++highlightToken;
  await nextTick();
  if (token !== highlightToken) return; // 期间又来了新内容，放弃本轮
  if (textContentRef.value) {
    try {
      await highlightCodeBlocks(textContentRef.value);
    } catch {
      // 忽略高亮错误（流式竞态下 DOM 可能已被替换）
    }
  }
});

onMounted(() => {
  if (textContentRef.value) {
    highlightCodeBlocks(textContentRef.value);
  }
});

onUnmounted(() => {
  if (textContentRef.value) {
    removeCodeBlockListeners(textContentRef.value);
  }
});

function renderMarkdown(text: string): string {
  if (!text) return '';
  const citations: string[] = [];
  const protectedText = text.replace(/\[(\d+)\]/g, (_m, idx: string) => {
    citations.push(idx);
    return `<span class="chat-citation-link" data-citation-idx="${idx}">[${idx}]</span>`;
  });
  const html = renderMd(protectedText).html;
  // 白名单统一走 markdown-sanitize.ts（此前自持一份且与其它管线漂移）
  return sanitizeMarkdownHtml(html);
}


const bubbleStyle = (role: string) => ({
  display: 'flex', gap: AppleSpace.xs, marginBottom: AppleSpace.sm,
  flexDirection: role === 'user' ? ('row-reverse' as const) : ('row' as const),
});
const avatarStyle = (role: string) => ({
  width: '32px', height: '32px', borderRadius: AppleRadius.circle, flexShrink: '0',
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  background: role === 'user' ? cv('blue') : cv('bg-secondary'),
  color: role === 'user' ? cv('on-accent') : cv('text-primary'),
});
const contentStyle = { maxWidth: '70%', minWidth: '0' };
const timeStyle = (role: string) => ({
  fontSize: AppleType.microUI.size,
  letterSpacing: AppleType.microUI.letterSpacing,
  color: cv('text-quaternary'),
  textAlign: role === 'user' ? ('right' as const) : ('left' as const),
  marginBottom: '1px',
});
const contentWrapperStyle = { position: 'relative' as const, minWidth: '0' };
const textStyle = { fontSize: AppleType.bodyPrimary.size, lineHeight: AppleLineHeight.spacious, color: cv('text-primary') };
const typingStyle = { display: 'flex', gap: AppleSpace.xs, padding: `${AppleSpace.sm} 0` };
const loadingContainerStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  padding: `${AppleSpace.sm} 0`,
};
const loadingTextStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};
const dotStyle = (index: number) => ({
  width: '8px', height: '8px', borderRadius: AppleRadius.circle, background: cv('text-tertiary'),
  // 时长与逐颗延迟都走令牌：原先这里是硬写的 1.4s 与 0.16s×序号，
  // 个性化（动效强度档）调不动行内的裸秒数 —— 见 apple-tokens.ts 的 BASE_DURATION_MS。
  // reduced 档由注入层那组 !important 把 iteration-count 压成 1（无限跳动直接停住）。
  animation: 'chatTypingBounce var(--apple-dur-typingBounce) infinite ease-in-out both',
  animationDelay: `calc(${index * 0.16}s * var(--apple-motion-scale))`,
});
const sourcesStyle = { marginTop: AppleSpace.xs };
const speakRowStyle = { marginTop: AppleSpace.xs };
const errorStyle = { marginTop: AppleSpace.xs };
const chainStyle = { marginTop: AppleSpace.xs, borderTop: `1px solid ${cv('border-light')}`, paddingTop: AppleSpace.xs };
const chainHeaderStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace.xs,
  fontSize: AppleType.microUI.size, color: cv('text-tertiary'),
  cursor: 'pointer', userSelect: 'none' as const,
};
const chainBodyStyle = { marginTop: AppleSpace.xs, display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.sm };
const chainStepStyle = { display: 'flex', gap: AppleSpace.sm, alignItems: 'flex-start' };
const chainDotStyle = (status?: string) => ({
  width: '8px', height: '8px', borderRadius: AppleRadius.circle, marginTop: AppleSpace.xs, flexShrink: '0',
  background: status === 'warn' ? cv('orange') : cv('green'),
});
const chainStepTitleStyle = { fontSize: AppleType.microUI.size, fontWeight: String(AppleFontWeight.medium), color: cv('text-primary') };
const chainStepContentStyle = { flex: '1', minWidth: '0' };

// ─── ChatGPT 风格 Thinking 内联展示 ──────────────────────────────
const thinkingBlockStyle = {
  marginBottom: AppleSpace.xs,
  borderRadius: AppleRadius.small,
  border: `1px solid ${cv('border-light')}`,
  background: cv('bg-tertiary'),
  overflow: 'hidden',
};
const thinkingHeaderStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace.xxs,
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  cursor: 'pointer', userSelect: 'none' as const,
  transition: 'background var(--apple-dur-fast)',
};
const thinkingTitleStyle = {
  fontSize: AppleType.microUI.size, fontWeight: String(AppleFontWeight.medium), color: cv('text-secondary'),
};
const thinkingBadgeStyle = {
  fontSize: AppleType.microUI.size, letterSpacing: AppleType.microUI.letterSpacing, color: cv('text-quaternary'),
  marginLeft: AppleSpace.xxs,
};
const thinkingBodyStyle = {
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  borderTop: `1px solid ${cv('border-light')}`,
  display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.sm,
};
const thinkingStepStyle = {
  display: 'flex', gap: AppleSpace.xxs, alignItems: 'flex-start',
};
const thinkingStepNumStyle = {
  display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
  width: '18px', height: '18px', borderRadius: AppleRadius.circle,
  background: cv('bg-secondary'), color: cv('text-secondary'),
  fontSize: AppleType.microUI.size, letterSpacing: AppleType.microUI.letterSpacing, fontWeight: String(AppleFontWeight.semibold),
  flexShrink: '0', marginTop: '1px',
};
const thinkingStepContentStyle = { flex: '1', minWidth: '0' };
const thinkingStepTitleStyle = {
  fontSize: AppleType.microUI.size, fontWeight: String(AppleFontWeight.medium), color: cv('text-primary'),
};
const thinkingStepDetailStyle = {
  fontSize: AppleType.microUI.size, color: cv('text-tertiary'), lineHeight: AppleLineHeight.default, marginTop: '1px',
  whiteSpace: 'pre-wrap' as const, wordBreak: 'break-word' as const,
};

const stepNumStyle = {
  display: 'inline-flex', alignItems: 'center', justifyContent: 'center',
  width: '18px', height: '18px', borderRadius: AppleRadius.circle,
  background: cv('blue'), color: cv('on-accent'),
  fontSize: AppleType.microUI.size, letterSpacing: AppleType.microUI.letterSpacing, fontWeight: String(AppleFontWeight.semibold),
  marginRight: AppleSpace.xs, flexShrink: '0',
};
const chainStepActionStyle = {
  fontSize: AppleType.microUI.size, letterSpacing: AppleType.microUI.letterSpacing, color: cv('text-tertiary'),
  fontFamily: AppleFont.mono, marginTop: AppleSpace.xxs,
};
const chainStepDetailStyle = { fontSize: AppleType.microUI.size, color: cv('text-tertiary'), lineHeight: AppleLineHeight.loose, marginTop: AppleSpace.xxs };
const clarifyOptionsStyle = { display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xs, marginTop: AppleSpace.xs };
</script>

<style>
.code-wrapper {
  position: relative;
  margin: 8px 0;
  border-radius: var(--apple-radius-medium, 8px);
  overflow: hidden;
  border: 1px solid var(--apple-border);
  background: var(--apple-bg-secondary);
}
.code-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 12px;
  background: var(--apple-bg-tertiary);
  border-bottom: 1px solid var(--apple-border);
}
.code-lang {
  font-size: var(--apple-font-size-caption, 12px);
  font-weight: 500;
  color: var(--apple-text-secondary);
  font-family: -apple-system, sans-serif;
}
.code-copy-btn {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 2px 8px;
  border-radius: var(--apple-radius-small, 5px);
  border: 1px solid var(--apple-border);
  background: var(--apple-surface-field);
  color: var(--apple-text-secondary);
  font-size: var(--apple-font-size-caption, 12px);
  cursor: pointer;
  transition: background var(--apple-dur-fast), color var(--apple-dur-fast), border-color var(--apple-dur-fast);
  font-family: -apple-system, sans-serif;
  line-height: 1.4;
}
.code-copy-btn:hover {
  background: var(--apple-bg-hover);
  color: var(--apple-text-primary);
}
.code-copy-btn.copied {
  color: var(--apple-green);
  border-color: var(--apple-green);
}
.chat-code-block {
  background: var(--apple-code-bg);
  color: var(--apple-code-fg);
  border-radius: 0;
  padding: 12px 16px;
  margin: 0;
  overflow-x: auto;
  font-family: 'SF Mono', 'Fira Code', 'Cascadia Code', Consolas, monospace;
  font-size: var(--apple-font-size-caption, 13px);
  line-height: 1.5;
}
.chat-code-block code {
  background: none;
  padding: 0;
  border-radius: 0;
  font-size: inherit;
}

@keyframes chatTypingBounce {
  0%, 80%, 100% {
    transform: scale(0.6);
    opacity: 0.6;
  }
  40% {
    transform: scale(1);
    opacity: 1;
  }
}

/* 内联引用标记样式 */
.chat-citation-link {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 18px;
  height: 18px;
  padding: 0 5px;
  border-radius: var(--apple-radius-small, 5px);
  background: var(--apple-blue);
  color: var(--apple-white);
  font-size: var(--apple-font-size-micro, 11px);
  font-weight: 600;
  text-decoration: none;
  vertical-align: middle;
  margin: 0 1px;
  cursor: pointer;
  transition: opacity var(--apple-dur-fast);
}
.chat-citation-link:hover {
  opacity: 0.8;
}

/* ── Chat Markdown Content Styles ── */
.chat-md-content h1 { font-size: 1.5em; margin: 1em 0 0.5em; font-weight: 600; color: var(--apple-text-primary); }
.chat-md-content h2 { font-size: 1.25em; margin: 0.8em 0 0.4em; font-weight: 600; color: var(--apple-text-primary); }
.chat-md-content h3 { font-size: 1.1em; margin: 0.6em 0 0.3em; font-weight: 600; color: var(--apple-text-primary); }
.chat-md-content h4 { font-size: 1em; margin: 0.5em 0 0.25em; font-weight: 600; color: var(--apple-text-primary); }
.chat-md-content h5, .chat-md-content h6 { font-size: 0.9em; margin: 0.5em 0; font-weight: 600; color: var(--apple-text-secondary); }

.chat-md-content p { margin: 0 0 0.6em; }

.chat-md-content a { color: var(--apple-apple-action-blue); text-decoration: none; }
.chat-md-content a:hover { text-decoration: underline; }

.chat-md-content strong { font-weight: 600; }
.chat-md-content em { font-style: italic; }

.chat-md-content ul, .chat-md-content ol { padding-left: 1.8em; margin: 0 0 0.6em; }
.chat-md-content li { margin: 0.2em 0; }
.chat-md-content li > p { margin-top: 0.3em; }

.chat-md-content blockquote {
  margin: 0 0 0.6em;
  padding: 0.4em 1em;
  color: var(--apple-text-secondary);
  border-left: 0.25em solid var(--apple-border);
  background: var(--apple-bg-hover);
  border-radius: 0 var(--apple-radius-small, 8px) var(--apple-radius-small, 8px) 0;
}
.chat-md-content blockquote > :first-child { margin-top: 0; }
.chat-md-content blockquote > :last-child { margin-bottom: 0; }

.chat-md-content code {
  font-family: var(--apple-font-mono, 'SF Mono', 'Menlo', 'Consolas', monospace);
  font-size: 0.875em;
  padding: 0.15em 0.4em;
  background: var(--apple-bg-hover);
  border-radius: var(--apple-radius-small, 4px);
  color: var(--apple-text-primary);
}

.chat-md-content pre {
  margin: 0 0 0.8em;
  padding: 0;
  background: transparent;
}

.chat-md-content .code-block-wrapper {
  position: relative;
  margin: 0 0 0.8em;
  border-radius: var(--apple-radius-small, 8px);
  overflow: hidden;
  border: 1px solid var(--apple-border);
  background: var(--apple-bg-secondary);
}

.chat-md-content .code-block-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 12px;
  background: var(--apple-bg-tertiary);
  border-bottom: 1px solid var(--apple-border);
  font-size: 0.8em;
  color: var(--apple-text-secondary);
}

.chat-md-content .code-block-lang {
  font-weight: 500;
  font-family: var(--apple-font-text, -apple-system, sans-serif);
}

.chat-md-content .code-copy-btn {
  padding: 2px 8px;
  border: 1px solid var(--apple-border);
  border-radius: var(--apple-radius-small, 4px);
  background: var(--apple-surface-field);
  color: var(--apple-text-secondary);
  font-size: 0.8em;
  cursor: pointer;
  /* 点名要动的属性：all 会把 width/height 一起动（每帧重排），
     而且省略 easing 会落到浏览器默认 ease，不在 AppleEasing 三档上 */
  transition: background var(--apple-dur-fast) var(--apple-easing-standard), border-color var(--apple-dur-fast) var(--apple-easing-standard), color var(--apple-dur-fast) var(--apple-easing-standard);
  line-height: 1.4;
}
.chat-md-content .code-copy-btn:hover {
  background: var(--apple-bg-hover);
  color: var(--apple-text-primary);
}
.chat-md-content .code-copy-btn.copied {
  color: var(--apple-green);
  border-color: var(--apple-green);
}

.chat-md-content .code-block,
.chat-md-content .monaco-code-block {
  background: var(--apple-bg-secondary);
  border: none;
  border-radius: 0;
  padding: 12px 16px;
  overflow-x: auto;
  font-family: var(--apple-font-mono, 'SF Mono', 'Menlo', 'Consolas', monospace);
  font-size: 0.85em;
  line-height: 1.6;
  margin: 0;
}
.chat-md-content .code-line {
  display: flex;
  min-height: 1.6em;
}
.chat-md-content .line-number {
  display: inline-block;
  min-width: 2.5em;
  padding-right: 1.2em;
  text-align: right;
  color: var(--apple-text-quaternary);
  user-select: none;
  flex-shrink: 0;
}
.chat-md-content .code-content {
  flex: 1;
  min-width: 0;
}

.chat-md-content table {
  border-collapse: collapse;
  width: 100%;
  margin: 0 0 0.8em;
  font-size: 0.9em;
}
.chat-md-content th, .chat-md-content td {
  border: 1px solid var(--apple-border);
  padding: 6px 12px;
  text-align: left;
}
.chat-md-content th {
  background: var(--apple-bg-secondary);
  font-weight: 600;
}
.chat-md-content tr:nth-child(even) {
  background: var(--apple-bg-hover);
}

.chat-md-content hr {
  border: none;
  border-top: 1px solid var(--apple-border);
  margin: 1em 0;
}

.chat-md-content img {
  max-width: 100%;
  border-radius: var(--apple-radius-small, 8px);
}
</style>
