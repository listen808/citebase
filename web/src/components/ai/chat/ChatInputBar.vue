<template>
  <div :style="inputAreaStyle">
    <div :style="intentRowStyle">
      <!-- 四个意图是互斥选项，不是四个动作按钮：并回 ApSegmentedControl 之后
           选中态、role=tab、aria-selected 与方向键都由那一处负责 -->
      <ApSegmentedControl
        :model-value="effectiveIntent"
        :options="intentOptions"
        size="small"
        wrap
        :aria-label="$t('aiChat.k30')"
        @update:model-value="(v: string | number) => $emit('intent-change', String(v) as AIIntent)"
      />
      <ApSelect
        v-if="effectiveIntent === 'data_query'"
        :model-value="selectedSpaceId"
        :options="spaceOptions"
        size="small"
        :style="{ marginLeft: 'auto', maxWidth: '160px' }"
        @update:model-value="$emit('space-change', Number($event))"
      />
    </div>
    <!-- 斜杠命令菜单 -->
    <div v-if="showSlashMenu" ref="slashMenuRef" class="apple-glass-layer" :style="slashMenuStyle">
      <div
        v-for="(cmd, i) in filteredSlashCommands"
        :key="cmd.label"
        :style="slashItemStyle(i === activeSlashIdx)"
        @click="selectSlash(cmd)"
        @mouseenter="activeSlashIdx = i"
      >
        <ApIcon :name="cmd.icon" :size="14" :style="slashIconStyle" />
        <div>
          <div :style="slashLabelStyle">{{ cmd.label }}</div>
          <div :style="slashDescStyle">{{ cmd.desc }}</div>
        </div>
      </div>
      <div v-if="filteredSlashCommands.length === 0" :style="slashEmptyStyle">{{ $t('aiChat.k27') }}</div>
      <div v-if="filteredSlashCommands.length > 0" :style="slashHintStyle">
        {{ $t('aiChat.k28') }}
      </div>
    </div>
    <div :style="inputRowStyle">
      <div ref="textareaWrapRef" :style="textareaWrapperStyle">
        <ApTextarea
          :model-value="modelValue"
          :style="textareaStyle"
          :aria-label="$t('aiChat.k31')"
          :placeholder="$t('aiChat.k29')"
          :rows="1"
          :disabled="disabled || isStreaming"
          :maxlength="8000"
          @update:model-value="onInput"
          @keydown.enter.exact.prevent="handleEnter"
          @compositionend="onCompositionEnd"
          @keydown.escape.prevent="showSlashMenu = false"
          @keydown.up="onArrowUp"
          @keydown.down="onArrowDown"
        />
        <div :style="charCountStyle">{{ modelValue.length }}/8000</div>
      </div>
      <ApButton
        v-if="isStreaming"
        variant="danger"
        size="small"
        icon-only
        :style="sendButtonStyle"
        :aria-label="$t('ai.stopGeneration')"
        @click="$emit('stop')"
      >
        <ApIcon name="Square" :size="16" />
      </ApButton>
      <ApButton
        v-else
        variant="primary"
        :disabled="disabled || !modelValue.trim()"
        size="small"
        icon-only
        :style="sendButtonStyle"
        :aria-label="$t('ai.send')"
        @click="$emit('send')"
      >
        <ApIcon name="Send" :size="16" />
      </ApButton>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import type { Component } from 'vue';
import type { IconName } from '@/utils/icons';
import { icons } from '@/utils/icons';
import { ref, computed, shallowRef } from 'vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApTextarea from '@/components/apple/ApTextarea.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import { useClickOutsideClose } from '@/composables/ui/useClickOutsideClose';
import { AppleEasing, AppleFontWeight, AppleInputSize, AppleSpace, AppleType, cv, menuSurfaceStyle } from '@/utils/core/apple-tokens';
import { scrollSingleAxis } from '@/styles/layout/page-styles';
import type { AIIntent } from '@/api/modules/ai/ai-intent';

const props = defineProps<{
  modelValue: string;
  disabled?: boolean;
  isStreaming?: boolean;
  effectiveIntent: AIIntent;
  selectedSpaceId: number;
  spaces: Array<{ id: number; name: string }>;
}>();

const emit = defineEmits<{
  'update:modelValue': [value: string];
  'send': [];
  'stop': [];
  'intent-change': [intent: AIIntent];
  'space-change': [id: number];
}>();
const { t } = useI18n();

const intentOptions: { value: AIIntent; label: string; icon: Component }[] = [
  { value: 'chat', label: t('aiChat.k32'), icon: icons.MessageSquare },
  { value: 'data_query', label: t('aiChat.k33'), icon: icons.Database },
  { value: 'agent', label: t('aiChat.k34'), icon: icons.Zap },
  { value: 'research', label: t('aiChat.k35'), icon: icons.Search },
];

const spaceOptions = computed(() => [
  { label: t('aiChat.k36'), value: 0 },
  ...props.spaces.map(s => ({ label: s.name, value: s.id })),
]);

// 斜杠命令
const showSlashMenu = ref(false);
const slashMenuRef = shallowRef<HTMLElement | null>(null);
const textareaWrapRef = shallowRef<HTMLElement | null>(null);
// 原先只有 Esc 与"选中一条"会关：点聊天区别处（比如翻历史消息）菜单就一直挂着，
// 盖在输入框上方。Esc 那半条本来就在，这里补"点别处"这半条。
useClickOutsideClose(
  () => showSlashMenu.value,
  () => { showSlashMenu.value = false; },
  () => [slashMenuRef.value, textareaWrapRef.value],
);
const activeSlashIdx = ref(0);
const slashQuery = ref('');

interface SlashCommand { icon: IconName; label: string; desc: string; template: string; intent?: AIIntent }
const slashCommands: SlashCommand[] = [
  { icon: 'FileText', label: t('aiChat.k37'), desc: t('aiChat.k38'), template: '请帮我撰写一篇关于 ' },
  { icon: 'BarChart3', label: t('aiChat.k33'), desc: t('aiChat.k39'), template: '查询 ', intent: 'data_query' },
  { icon: 'Search', label: t('aiChat.k40'), desc: t('aiChat.k41'), template: '搜索：' },
  { icon: 'Globe', label: t('aiChat.k35'), desc: t('aiChat.k42'), template: '深入研究：', intent: 'research' },
  { icon: 'ClipboardList', label: t('aiChat.k43'), desc: t('aiChat.k44'), template: '请总结以下内容：\n' },
  { icon: 'RefreshCw', label: t('aiChat.k45'), desc: t('aiChat.k46'), template: '请将以下内容翻译为英文：\n' },
  { icon: 'Bug', label: t('aiChat.k47'), desc: t('aiChat.k48'), template: '请审查以下代码：\n```\n\n```' },
  { icon: 'Lightbulb', label: t('aiChat.k49'), desc: t('aiChat.k50'), template: '请对以下话题进行头脑风暴：' },
];

const filteredSlashCommands = computed(() => {
  if (!slashQuery.value) return slashCommands;
  const q = slashQuery.value.toLowerCase();
  return slashCommands.filter(c => c.label.toLowerCase().includes(q) || c.desc.toLowerCase().includes(q));
});

function onInput(val: string | number) {
  const v = String(val);
  emit('update:modelValue', v);
  // 检测斜杠触发
  if (v === '/' || v.endsWith('\n/')) {
    showSlashMenu.value = true;
    slashQuery.value = '';
    activeSlashIdx.value = 0;
    return;
  }
  if (showSlashMenu.value) {
    const lastSlash = v.lastIndexOf('/');
    if (lastSlash === -1) {
      showSlashMenu.value = false;
      return;
    }
    slashQuery.value = v.slice(lastSlash + 1);
    activeSlashIdx.value = 0;
  }
}

function selectSlash(cmd: SlashCommand) {
  const v = props.modelValue;
  const lastSlash = v.lastIndexOf('/');
  const prefix = lastSlash >= 0 ? v.slice(0, lastSlash) : v;
  emit('update:modelValue', prefix + cmd.template);
  if (cmd.intent) emit('intent-change', cmd.intent);
  showSlashMenu.value = false;
}

function moveSlash(dir: number) {
  if (!showSlashMenu.value) return;
  const max = filteredSlashCommands.value.length - 1;
  activeSlashIdx.value = Math.max(0, Math.min(max, activeSlashIdx.value + dir));
}

function onArrowUp(e: KeyboardEvent) {
  if (!showSlashMenu.value) return;
  e.preventDefault();
  moveSlash(-1);
}

function onArrowDown(e: KeyboardEvent) {
  if (!showSlashMenu.value) return;
  e.preventDefault();
  moveSlash(1);
}

// IME 守卫：还在选字时回车只管选字；Safari 选完字半秒内会补发一次回车，一并拦下
let lastCompositionEndAt = 0;
function onCompositionEnd() { lastCompositionEndAt = Date.now(); }

function handleEnter(e?: KeyboardEvent) {
  if (e && (e.isComposing || e.keyCode === 229)) return;
  if (Date.now() - lastCompositionEndAt < 500) return;
  if (showSlashMenu.value && filteredSlashCommands.value.length > 0) {
    const cmd = filteredSlashCommands.value[activeSlashIdx.value];
    if (cmd) selectSlash(cmd);
    return;
  }
  emit('send');
}

const inputAreaStyle = { padding: `${AppleSpace.xs} ${AppleSpace.sm}`, borderTop: `1px solid ${cv('border')}` };
const intentRowStyle = { display: 'flex', gap: AppleSpace.xs, marginBottom: AppleSpace.xs, alignItems: 'center', flexWrap: 'wrap' as const };

const inputRowStyle = { display: 'flex', gap: AppleSpace.xs, alignItems: 'flex-end' };
// P1-7：输入框下方还有一行字符计数（12px × 1.33 + 2px 间距）。flex-end 原本把发送按钮
// 拉到计数行的底线，视觉上吊在输入框下方；补上计数行高度，按钮才与输入框下沿对齐。
const sendButtonStyle = { marginBottom: `calc(${AppleType.microUI.size} * ${AppleType.microUI.lineHeight} + ${AppleSpace.xxs})` };
const textareaWrapperStyle = { flex: '1', minWidth: 0, display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xxs };
const textareaStyle = { minHeight: AppleInputSize.medium.height, maxHeight: '140px', width: '100%' };
const charCountStyle = { fontSize: AppleType.microUI.size, color: cv('text-tertiary'), textAlign: 'right' as const, paddingRight: AppleSpace.xxs };

// 斜杠菜单样式
const slashMenuStyle = menuSurfaceStyle({
  maxHeight: '240px',
  marginBottom: AppleSpace.xs,
  ...scrollSingleAxis,
});
const slashItemStyle = (active: boolean) => ({
  display: 'flex',
  gap: AppleSpace.sm,
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  cursor: 'pointer',
  background: active ? cv('blue-selection') : 'transparent',
  color: active ? cv('blue-text') : cv('text-primary'),
  transition: `background var(--apple-dur-fast) ${AppleEasing.standard}`,
});
const slashIconStyle = { fontSize: AppleType.subhead.size, lineHeight: '1', flexShrink: '0' };
const slashLabelStyle = { fontSize: AppleType.controlLabel.size, fontWeight: String(AppleFontWeight.medium), color: 'inherit' };
const slashDescStyle = { fontSize: AppleType.microUI.size, color: cv('text-tertiary'), marginTop: AppleSpace.xxs };
const slashEmptyStyle = { padding: `${AppleSpace.sm} ${AppleSpace.md}`, fontSize: AppleType.microUIBold.size, color: cv('text-tertiary'), textAlign: 'center' as const };
const slashHintStyle = {
  padding: `${AppleSpace.xs} ${AppleSpace.md}`,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
  textAlign: 'center' as const,
  borderTop: `1px solid ${cv('border-light')}`,
  background: cv('bg-secondary'),
};
</script>
