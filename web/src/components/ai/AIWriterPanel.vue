<template>
  <div v-if="modelValue" :style="panelContainerStyle" role="dialog" aria-modal="true" :aria-label="$t('aiUi.k4')">
    <div class="apple-glass-layer" :style="panelStyle">
      <div :style="headerStyle">
        <span :style="headerTitleStyle">{{ $t('aiUi.k4') }}</span>
        <ApButton variant="ghost" size="mini" icon-only :aria-label="$t('aiUi.k9')" @click="emit('update:modelValue', false)">
          <ApIcon name="X" :size="16" />
        </ApButton>
      </div>
      
      <div :style="tabsStyle">
        <ApSegmentedControl
          :model-value="activeTab"
          :options="tabOptions"
          size="small"
          :aria-label="$t('aiUi.k10')"
          @update:model-value="(v: string | number) => setActiveTab(String(v) as AIWriterTab)"
        />
      </div>
      
      <div :style="contentStyle">
        <div v-if="activeTab === 'chat'" :style="chatPaneStyle">
          <div :style="messagesStyle">
            <ApEmpty
              v-if="messages.length === 0"
              icon="MessageSquare"
              :title="$t('aiUi.k7')"
              :description="$t('cExtraUi.k1')"
            />
            <div v-else>
              <div
                v-for="msg in messages"
                :key="msg.id"
                :style="messageBubbleStyle(msg.role)"
              >
                <div :style="messageContentStyle">
                  <ApMarkdownPreview :content="msg.content" />
                </div>
              </div>
            </div>
          </div>
          <div :style="inputAreaStyle">
            <ApTextarea
              v-model="inputMessage"
              :placeholder="$t('aiUi.k6')"
              :disabled="loading"
              @keydown="handleInputKeydown"
            />
            <div :style="inputActionsStyle">
              <ApButton
                variant="primary"
                size="small"
                :loading="loading"
                @click="handleSend"
              >
                {{ $t('aiUi.k5') }}
              </ApButton>
            </div>
          </div>
        </div>
        
        <div v-else-if="activeTab === 'templates'" :style="templatesPaneStyle">
          <AITemplatePicker
            :templates="templates"
            @generate="handleGenerateFromTemplate"
          />
        </div>
        
        <div v-else-if="activeTab === 'history'" :style="historyPaneStyle">
          <ApEmpty :title="$t('aiUi.k8')" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { ref } from 'vue';
import { useAIWriter } from '@/composables/useAIWriter';
import { subtleBg, AppleSpace, AppleRadius, AppleType, AppleFontWeight, AppleLineHeight, cv, AppleZIndex } from '@/utils/core/apple-tokens';
import { scrollSingleAxis } from '@/styles/layout/page-styles';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import ApTextarea from '@/components/apple/ApTextarea.vue';
import ApMarkdownPreview from '@/components/apple/ApMarkdownPreview.vue';
import AITemplatePicker from '@/components/ai/AITemplatePicker.vue';
import type { AIWriterTab } from '@/types/ai-writer';
import { logger } from '@/utils/logger';
import { useUIStore } from '@/stores/app/ui';
import { useEscapeClose } from '@/composables/ui/useEscapeClose';

const props = defineProps<{
  modelValue: boolean;
  documentId?: number;
  spaceId?: number;
}>();

const emit = defineEmits<{
  'update:modelValue': [value: boolean];
  'insert-text': [text: string];
  'replace-selection': [text: string];
}>();
const { t } = useI18n();

// 写作面板是覆盖全页的半模态层，原先只能用右上角那颗按钮关
useEscapeClose(() => props.modelValue, () => emit('update:modelValue', false));


const {
  activeTab,
  messages,
  loading,
  templates,
  setActiveTab,
  sendMessage,
  applyAI,
  insertText
} = useAIWriter({
  onInsertText: (text) => emit('insert-text', text),
  onReplaceSelection: (text) => emit('replace-selection', text)
});

const ui = useUIStore();

const inputMessage = ref('');

const tabs = [
  { id: 'chat' as const, name: '对话' },
  { id: 'templates' as const, name: '模板' },
  { id: 'history' as const, name: '历史' }
];
const tabOptions = tabs.map((t) => ({ label: t.name, value: t.id }));

const handleSend = () => {
  if (inputMessage.value.trim()) {
    sendMessage(inputMessage.value, { docId: props.documentId, spaceId: props.spaceId });
    inputMessage.value = '';
  }
};

const handleInputKeydown = (e: KeyboardEvent) => {
  if (e.key === 'Enter' && !e.shiftKey) {
    // IME 守卫：还在选字时回车只管选字，不发送
    if (e.isComposing || e.keyCode === 229) return;
    e.preventDefault();
    handleSend();
  }
};

const handleGenerateFromTemplate = async (templateId: string, variables: Record<string, string>) => {
  try {
    const result = await applyAI(templateId, variables);
    if (result) {
      insertText(result);
    }
  } catch (error) {
    logger.error('Error applying template:', error);
    ui.showToast(t('aiUi.k11'), 'error');
  }
};

const panelContainerStyle = {
  position: 'fixed' as const,
  top: '0',
  right: '0',
  bottom: '0',
  // 满屏高 + 半透明遮罩 + 从右侧滑出 = 抽屉语义，与 ApDrawer 同档；原先取 dropdown(1000)，
  // 于是任何下拉菜单都能盖在它上面
  zIndex: String(AppleZIndex.drawer),
  background: subtleBg(cv('text-primary'), 50, 'gray-bg-subtle'),
  backdropFilter: cv('glass-blur'),
  display: 'flex',
  justifyContent: 'flex-end',
  alignItems: 'stretch'
};

const panelStyle = {
  width: '400px',
  flex: '1',
  minHeight: '0',
  // 浮层本体是 surface-float 档（不是卡片档），阴影同理取 modal 档（与 ApDrawer/ApModal 一致）
  background: cv('surface-float'),
  boxShadow: cv('shadow-modal'),
  display: 'flex',
  flexDirection: 'column' as const
};

const headerStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: `${AppleSpace.md} ${AppleSpace.lg}`,
  borderBottom: `1px solid ${cv('border-light')}`
};

const headerTitleStyle = {
  fontWeight: String(AppleFontWeight.semibold),
  fontSize: AppleType.bodyPrimary.size
};



const tabsStyle = {
  display: 'flex',
  padding: `${AppleSpace.xs} ${AppleSpace.md}`,
  gap: AppleSpace.xs,
  borderBottom: `1px solid ${cv('border-light')}`
};

const contentStyle = {

  flex: '1',
  display: 'flex',
  flexDirection: 'column' as const,
  overflow: 'hidden'
};

const chatPaneStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  flex: '1',
  minHeight: '0'
};

const messagesStyle = {
  flex: '1',
  padding: AppleSpace.md,
  ...scrollSingleAxis,
};

const messageBubbleStyle = (role: string) => ({
  marginBottom: AppleSpace.md,
  padding: AppleSpace.md,
  borderRadius: AppleRadius.medium,
  background: role === 'user' ? cv('blue') : cv('bg-secondary'),
  color: role === 'user' ? cv('on-accent') : cv('text-primary'),
  maxWidth: '85%',
  alignSelf: role === 'user' ? 'flex-end' : 'flex-start'
});

const messageContentStyle = {
  fontSize: AppleType.controlLabel.size,
  lineHeight: AppleLineHeight.spacious
};

const inputAreaStyle = {
  padding: AppleSpace.md,
  borderTop: `1px solid ${cv('border-light')}`
};

const inputActionsStyle = {
  display: 'flex',
  justifyContent: 'flex-end',
  marginTop: AppleSpace.sm
};





const templatesPaneStyle = {
  padding: AppleSpace.md,
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.md,
  ...scrollSingleAxis,
};

const historyPaneStyle = {
  padding: AppleSpace.md,
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.md,
  ...scrollSingleAxis,
};
</script>
