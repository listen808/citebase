<template>
  <div
    :style="suggestionItemStyle(isSelected)"
    :tabindex="isSelected ? 0 : -1"
    role="option"
    :aria-selected="isSelected"
    @click="$emit('click', result)"
    @keydown.enter="$emit('click', result)"
    @keydown.space.prevent="$emit('click', result)"
    @mouseenter="$emit('hover')"
  >
    <FileText v-if="result.document_id" :size="18" :style="itemIconStyle" />
    <Folder v-else-if="result.space_id" :size="18" :style="itemIconStyle" />
    <div :style="itemContentStyle">
      <!-- eslint-disable-next-line vue/no-v-html -- highlightText 先 escapeHtml 再插 <mark>，不含原始 HTML -->
      <span :style="itemTitleStyle" :title="result.name" v-html="highlightText(result.name, kw)"></span>
      <span v-if="result.space_name" :style="itemMetaStyle">{{ result.space_name }}</span>
      <!-- eslint-disable-next-line vue/no-v-html -- snippet 由 getHighlightSnippet 经 sanitizeAndHighlightHighlight 转义加亮 -->
      <span v-if="snippet" :style="itemSnippetStyle" v-html="snippet"></span>
    </div>
    <span v-if="result.document_id" :style="itemTypeStyle">{{ $t('apUi1.k50') }}</span>
    <span v-else-if="result.space_id" :style="itemTypeStyle">{{ $t('apUi1.k51') }}</span>
  </div>
</template>

<script setup lang="ts">
import { FileText, Folder } from 'lucide-vue-next';
import { AppleFont, AppleRadius, AppleSpace, AppleTransition, AppleType, cv } from '@/utils/core/apple-tokens';
import { highlightText } from '@/utils/domain/search-highlight';
import { itemIconStyle } from '@/styles/component/spotlight-search-styles';
import type { SearchResult } from '@/types/models';

defineProps<{
  result: SearchResult;
  isSelected: boolean;
  kw: string;
  snippet: string;
}>();

defineEmits<{
  (e: 'click', result: SearchResult): void;
  (e: 'hover'): void;
}>();

function suggestionItemStyle(selected: boolean) {
  return {
    display: 'flex',
    alignItems: 'center',
    gap: AppleSpace.sm,
    padding: `${AppleSpace.sm} ${AppleSpace.md}`,
    background: selected ? cv('blue-selection-strong') : 'transparent',
    cursor: 'pointer',
    transition: AppleTransition.smooth,
  };
}

const itemContentStyle = {
  display: 'flex' as const,
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
  flex: '1',
  overflow: 'hidden',
};

const itemTitleStyle = {
  fontFamily: AppleType.sectionTitle.font,
  fontSize: AppleType.sectionTitle.size,
  letterSpacing: AppleType.sectionTitle.letterSpacing,
  color: cv('text-primary'),
  whiteSpace: 'nowrap' as const,
  overflow: 'hidden',
  textOverflow: 'ellipsis',
};

const itemMetaStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  whiteSpace: 'nowrap' as const,
  overflow: 'hidden',
  textOverflow: 'ellipsis',
};

const itemSnippetStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const itemTypeStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  background: cv('bg-secondary'),
  padding: `3px ${AppleSpace.sm}`,
  borderRadius: AppleRadius.control,
  flexShrink: '0',
};
</script>
