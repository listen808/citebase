<template>
  <div :style="welcomeStyle">
    <div :style="titleStyle">{{ title }}</div>
    <div :style="subtitleStyle">{{ subtitle }}</div>
    <div :style="cardsStyle">
      <div
        v-for="(card, index) in promptCards"
        :key="index"
        :style="cardStyle"
        role="button"
        tabindex="0"
        @click="$emit('select-prompt', card.prompt)"
        @keydown.enter="$emit('select-prompt', card.prompt)"
      >
        <div :style="cardContentStyle">
          <div :style="cardTitleStyle">{{ card.title }}</div>
          <div v-if="card.description" :style="cardDescStyle">{{ card.description }}</div>
        </div>
        <ApIcon name="ChevronRight" :size="16" :style="{ color: cv('text-quaternary'), flexShrink: '0' }" />
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { computed } from 'vue';
import { cv, AppleSpace, AppleType, AppleTransition } from '@/utils/core/apple-tokens';

const props = defineProps<{
  title: string;
  subtitle: string;
  prompts: string[];
}>();

defineEmits<{
  'select-prompt': [prompt: string];
}>();
const { t } = useI18n();

const defaultCards = [
  { icon: 'FileText', title: t('aiChat.k60'), description: t('aiChat.k61'), prompt: '帮我写一篇文档', color: 'blue' as const },
  { icon: 'Search', title: t('aiChat.k40'), description: t('aiChat.k62'), prompt: '搜索知识库', color: 'purple' as const },
  { icon: 'FileSpreadsheet', title: t('aiChat.k63'), description: t('aiChat.k64'), prompt: '总结当前文档', color: 'green' as const },
  { icon: 'BarChart3', title: t('aiChat.k65'), description: t('aiChat.k66'), prompt: '生成数据分析', color: 'orange' as const },
];

const promptCards = computed(() => {
  if (props.prompts?.length) {
    return props.prompts.map((p, i) => {
      const card = defaultCards[i % defaultCards.length]!;
      // 传入的 prompt 本身就是完整问题，不再同文重复一遍作描述（审计 B13）
      return {
        icon: card.icon,
        title: p,
        description: '',
        prompt: p,
        color: card.color,
      };
    });
  }
  return defaultCards;
});

const welcomeStyle = { display: 'flex', flexDirection: 'column' as const, alignItems: 'flex-start', justifyContent: 'center', flex: '1', gap: AppleSpace.xs, padding: `${AppleSpace.lg} ${AppleSpace.md}` };
const titleStyle = { fontFamily: AppleType.bodyEmphasis.font, fontSize: AppleType.bodyEmphasis.size, fontWeight: String(AppleType.bodyEmphasis.weight), color: cv('text-primary') };
const subtitleStyle = { fontSize: AppleType.microUI.size, color: cv('text-secondary') };
const cardsStyle = { display: 'flex', flexDirection: 'column' as const, gap: 0, width: '100%', marginTop: AppleSpace.sm };
const cardStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  padding: `${AppleSpace.sm} 0`,
  background: 'transparent',
  border: 'none',
  borderBottom: `1px solid ${cv('border-light')}`,
  cursor: 'pointer',
  transition: AppleTransition.fast,
};
const cardContentStyle = { flex: '1', minWidth: '0', display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xxs };
const cardTitleStyle = { fontSize: AppleType.controlLabel.size, fontWeight: String(AppleType.bodyEmphasis.weight), color: cv('text-primary') };
const cardDescStyle = { fontSize: AppleType.microUI.size, color: cv('text-tertiary'), overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' as const };
</script>

<style scoped>
div[role="button"]:hover {
  background: var(--apple-bg-hover);
}
div[role="button"]:active {
  background: var(--apple-bg-hover);
}
</style>
