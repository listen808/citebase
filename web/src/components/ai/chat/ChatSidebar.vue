<template>
  <div :style="sidebarStyle">
    <div :style="headerStyle">
      <ApButton :style="newChatBtnStyle" size="small" @click="$emit('new-chat')">
        <ApIcon name="Plus" :size="16" /> {{ $t('aiChat.k53') }}
      </ApButton>
    </div>
    <div :style="listStyle">
      <!-- 加载失败与「确实没有对话」必须是两样（同 DocumentViewsView 的 loadError 形态） -->
      <ApEmpty
        v-if="error"
        type="error"
        :title="$t('aiChat.k54')"
        :description="error"
        :action-text="$t('lastAttrsUi.retry')"
        @action="$emit('retry')"
      />
      <ApEmpty v-else-if="conversations.length === 0" icon="MessageSquare" :title="$t('aiChat.k55')" />
      <div v-for="conv in conversations" :key="conv.id" :style="itemStyle(conv.id === activeId)" @click="$emit('select', conv.id)">
        <ApIcon name="MessageSquare" :size="14" :color="cv('text-tertiary')" />
        <div :style="infoStyle">
          <div :style="titleStyle" :title="conv.title || $t('fallback.k66')">{{ conv.title || $t('fallback.k66') }}</div>
          <div :style="timeStyle">{{ formatTime(conv.updated_at) }}</div>
        </div>
        <ApButton variant="ghost" icon-only size="mini" :aria-label="$t('aiChat.k56')" @click.stop="$emit('delete', conv.id)">
          <ApIcon name="Trash2" :size="16" />
        </ApButton>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import ApButton from '@/components/apple/ApButton.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import { cv, AppleSpace, AppleRadius, AppleType, AppleTransition } from '@/utils/core/apple-tokens';
import { scrollSingleAxis } from '@/styles/layout/page-styles';
import { formatShortMonthDay as formatTime } from '@/utils/format/format-time';
import type { AIConversation } from '@/types/ai';

defineProps<{
  conversations: AIConversation[];
  activeId: number | null;
  /** 由 useConversation 的 error 透传：空串表示加载没失败 */
  error?: string;
}>();

defineEmits<{
  'new-chat': [];
  'select': [id: number];
  'delete': [id: number];
  'retry': [];
}>();

const sidebarStyle = { width: '220px', flexShrink: '0', display: 'flex', flexDirection: 'column' as const, borderRight: `1px solid ${cv('border-light')}` };
const headerStyle = { padding: `${AppleSpace.xs} ${AppleSpace.sm}` };
const newChatBtnStyle = { width: '100%', justifyContent: 'center' };
// 会话标题已经会截断（infoStyle minWidth:0 + titleStyle ellipsis + :title），这里补的是容器那一侧：
// 两轴显式交代 + 槽位常驻，否则条目顶宽就冒横向条，与竖向条互相把对方挤出来（抖）
const listStyle = { flex: '1', ...scrollSingleAxis, padding: `0 ${AppleSpace.sm}` };
const itemStyle = (active: boolean) => ({
  display: 'flex', alignItems: 'center', gap: AppleSpace.xs,
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`, borderRadius: AppleRadius.small,
  cursor: 'pointer', transition: AppleTransition.default,
  background: active ? cv('bg-secondary') : 'transparent',
});
const infoStyle = { flex: '1', minWidth: '0' };
const titleStyle = { fontSize: AppleType.controlLabel.size, color: cv('text-primary'), overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const };
const timeStyle = { fontSize: AppleType.microUI.size, color: cv('text-tertiary') };
</script>