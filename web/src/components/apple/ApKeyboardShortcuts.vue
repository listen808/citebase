<template>
  <!-- 使用 v-show 而非仅依赖 v-if，确保内容在生产构建中不会意外显示 -->
  <ApModal v-show="isOpen" v-model="isOpen" :title="$t('apUi1.k9')" size="medium">
    <div :style="contentStyle">
      <div v-for="group in shortcutGroups" :key="group.title" :style="groupStyle">
        <h3 :style="groupTitleStyle">{{ group.title }}</h3>
        <div v-for="item in group.items" :key="item.keys" :style="rowStyle">
          <span :style="descStyle">{{ item.desc }}</span>
          <span :style="keysStyle">
            <kbd v-for="key in item.keys.split('+')" :key="key" :style="kbdStyle">{{ key }}</kbd>
          </span>
        </div>
      </div>
    </div>
  </ApModal>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { ref, onMounted } from 'vue';
import { cv, AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleType } from '@/utils/core/apple-tokens';

// 确保初始状态为 false，防止生产构建中意外显示
const isOpen = ref(false);

// 防御性检查：确保组件挂载时模态框是关闭的
onMounted(() => {
  isOpen.value = false;
});

const { t } = useI18n();
const shortcutGroups = [
  {
    title: t('apUi1.k10'),
    items: [
      { keys: 'Ctrl+K', desc: t('apUi1.k11') },
      { keys: 'Ctrl+/', desc: t('apUi1.k12') },
    ],
  },
  {
    title: t('apUi1.k13'),
    items: [
      { keys: 'Ctrl+E', desc: t('apUi1.k14') },
      { keys: 'Ctrl+S', desc: t('apUi1.k15') },
      { keys: 'Ctrl+Shift+B', desc: t('apUi1.k16') },
      { keys: 'Ctrl+Shift+H', desc: t('apUi1.k17') },
      { keys: 'E', desc: t('apUi1.k18') },
      { keys: 'Esc', desc: t('apUi1.k19') },
    ],
  },
  {
    title: t('apUi1.k20'),
    items: [
      { keys: 'Ctrl+S', desc: t('apUi1.k21') },
      { keys: 'Ctrl+Shift+S', desc: t('apUi1.k22') },
      { keys: 'Ctrl+Shift+E', desc: t('apUi1.k23') },
    ],
  },
  {
    title: t('apUi1.k24'),
    items: [
      { keys: 'Alt+←', desc: t('apUi1.k25') },
      { keys: 'Alt+→', desc: t('apUi1.k26') },
    ],
  },
];

function open() {
  isOpen.value = true;
}

function close() {
  isOpen.value = false;
}

defineExpose({ open, close });

const contentStyle = {
  maxHeight: '60vh',
  overflowY: 'auto' as const,
};

const groupStyle = {
  marginBottom: AppleSpace.lg,
};

const groupTitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  margin: `0 0 ${AppleSpace.sm} 0`,
  paddingBottom: AppleSpace.xs,
  borderBottom: `1px solid ${cv('border-light')}`,
};

const rowStyle = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  padding: `${AppleSpace.sm} ${AppleSpace.xs}`,
  borderRadius: AppleRadius.small,
};

const descStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-primary'),
};

const keysStyle = {
  display: 'flex',
  gap: AppleSpace.xs,
  alignItems: 'center',
  flexShrink: 0,
};

const kbdStyle = {
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  padding: `${AppleSpace.xxs} ${AppleSpace.sm}`,
  minWidth: '28px',
  height: '26px',
  fontFamily: AppleFont.mono,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-secondary'),
  background: cv('bg-tertiary'),
  borderRadius: AppleRadius.micro,
  border: `1px solid ${cv('border')}`,
  borderBottom: `2px solid ${cv('border')}`,
  lineHeight: '1',
};
</script>