<template>
  <ApModal v-model="showModal" :title="$t('docUi1.k70')" size="medium" density="compact">
    <div :style="containerStyle">
      <div v-for="group in shortcutGroups" :key="group.title" :style="groupStyle">
        <h3 :style="groupTitleStyle">{{ group.title }}</h3>
        <div :style="listStyle">
          <div v-for="item in group.items" :key="item.keys" :style="itemStyle">
            <span :style="keysStyle">
              <kbd v-for="key in item.keys.split('+')" :key="key" :style="kbdStyle">{{ key }}</kbd>
            </span>
            <span :style="totalStyle">{{ item.desc }}</span>
          </div>
        </div>
      </div>
    </div>
  </ApModal>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { computed } from 'vue';
import ApModal from '@/components/apple/ApModal.vue';
import { AppleSpace, AppleFont, AppleType, AppleFontWeight, AppleRadius, cv } from '@/utils/core/apple-tokens';
import { listStyle as stackListStyle, totalStyle } from '@/styles/layout/page-styles';

const props = defineProps<{
  modelValue: boolean;
}>();

const emit = defineEmits<{
  (e: 'update:modelValue', value: boolean): void;
}>();
const { t } = useI18n();

const showModal = computed({
  get: () => props.modelValue,
  set: (v: boolean) => emit('update:modelValue', v),
});

const shortcutGroups = [
  {
    title: t('docUi1.k71'),
    items: [
      { keys: 'Ctrl+S', desc: t('docUi1.k52') },
      { keys: 'Esc', desc: t('docUi1.k51') },
    ],
  },
  {
    title: t('docUi1.k72'),
    items: [
      { keys: 'Ctrl+Z', desc: t('docUi1.k73') },
      { keys: 'Ctrl+Shift+Z', desc: t('docUi1.k74') },
      { keys: 'Ctrl+B', desc: t('docUi1.k55') },
      { keys: 'Ctrl+I', desc: t('docUi1.k56') },
      { keys: 'Ctrl+K', desc: t('docUi1.k75') },
      { keys: 'Ctrl+H', desc: t('docUi1.k76') },
      { keys: 'Ctrl+L', desc: t('docUi1.k77') },
      { keys: 'Ctrl+Q', desc: t('docUi1.k78') },
      { keys: 'Ctrl+T', desc: t('docUi1.k6') },
    ],
  },
  {
    title: t('docUi1.k79'),
    items: [
      { keys: 'Ctrl+E', desc: t('docUi1.k80') },
      { keys: 'Ctrl+Shift+E', desc: t('docUi1.k81') },
      { keys: 'Ctrl+/', desc: t('docUi1.k50') },
    ],
  },
];

const containerStyle = stackListStyle(AppleSpace.lg);

const groupStyle = {};

const groupTitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  margin: `0 0 ${AppleSpace.sm} 0`,
};

const listStyle = stackListStyle(AppleSpace.xs);

const itemStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: `${AppleSpace.xs} 0`,
};

const keysStyle = {
  display: 'flex',
  gap: AppleSpace.xxs,
};

const kbdStyle = {
  display: 'inline-flex',
  alignItems: 'center',
  justifyContent: 'center',
  minWidth: '24px',
  height: '24px',
  padding: `0 ${AppleSpace.xs}`,
  fontFamily: AppleFont.mono,
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-secondary'),
  background: cv('bg-secondary'),
  border: `1px solid ${cv('border-light')}`,
  borderRadius: AppleRadius.small,
};


</script>
