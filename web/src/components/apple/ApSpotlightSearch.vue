<template>
  <Teleport to="body">
    <transition name="spotlight">
      <div v-if="visible" :style="overlayStyle" @click.self="close">
        <div data-ap="spotlight-search" class="apple-glass-layer" :style="containerStyle">
          <div role="search" :style="searchBoxStyle">
            <Search :size="20" :style="searchIconStyle" />
            <input
              ref="inputRef"
              v-model="keyword"
              :style="inputStyle"
              :placeholder="$t('apUi1.k44')"
              :aria-label="$t('apUi1.search')"
              @keydown="handleKeydown"
            />
            <span v-if="loading" :style="loadingIconStyle">
              <Loader2 :size="18" class="spinner" />
            </span>
          </div>

          <div v-if="showResults" :style="resultsStyle">
            <div v-if="loading && results.length === 0" :style="loadingStyle">
              <ApSkeleton variant="card" :lines="3" />
            </div>

            <div v-else-if="!hasSearched && suggestions.length > 0">
              <div :style="suggestionHeaderStyle">
                <span :style="suggestionTitleStyle">{{ $t('apUi1.k33') }}</span>
              </div>
              <div :style="suggestionListStyle">
                <div
                  v-for="(item, idx) in suggestions"
                  :key="item.id"
                  :style="suggestionItemStyle(selectedIndex === idx)"
                  :tabindex="selectedIndex === idx ? 0 : -1"
                  role="option"
                  :aria-selected="selectedIndex === idx"
                  @click="handleSuggestionClick(item)"
                  @keydown.enter="handleSuggestionClick(item)"
                  @keydown.space.prevent="handleSuggestionClick(item)"
                  @mouseenter="selectedIndex = idx"
                >
                  <component :is="item.icon" :size="18" :style="itemIconStyle" />
                  <span :style="itemTextStyle" :title="item.label">{{ item.label }}</span>
                  <span v-if="item.shortcut" :style="itemShortcutStyle">{{ item.shortcut }}</span>
                </div>
              </div>
            </div>

            <div v-if="!hasSearched && recentDocs.length > 0" :style="historySectionStyle">
              <div :style="suggestionHeaderStyle">
                <span :style="suggestionTitleStyle">{{ $t('apUi1.k34') }}</span>
              </div>
              <div :style="suggestionListStyle">
                <div
                  v-for="(doc, idx) in recentDocs.slice(0, 5)"
                  :key="doc.id"
                  :style="suggestionItemStyle(recentStartIndex + idx === selectedIndex)"
                  :tabindex="recentStartIndex + idx === selectedIndex ? 0 : -1"
                  role="option"
                  :aria-selected="recentStartIndex + idx === selectedIndex"
                  @click="handleRecentDocClick(doc)"
                  @keydown.enter="handleRecentDocClick(doc)"
                  @mouseenter="selectedIndex = recentStartIndex + idx"
                >
                  <FileText :size="18" :style="itemIconStyle" />
                  <span :style="itemTextStyle" :title="doc.name">{{ doc.name }}</span>
                  <span :style="recentSpaceStyle">{{ doc.space_name }}</span>
                </div>
              </div>
            </div>

            <div v-else-if="results.length > 0">
              <div :style="suggestionHeaderStyle">
                <span :style="suggestionTitleStyle">{{ $t('apUi1.k35') }}</span>
                <span :style="resultCountStyle">{{ results.length }} {{ $t('cExtraUi.k6') }}</span>
              </div>
              <div :style="suggestionListStyle">
                <SpotlightSearchResult
                  v-for="(result, idx) in results"
                  :key="result.document_id || result.space_id"
                  :result="result"
                  :is-selected="selectedIndex === idx"
                  :kw="keyword"
                  :snippet="getHighlightSnippet(result)"
                  @click="handleResultClick"
                  @hover="selectedIndex = idx"
                />
                <div v-if="results.length >= 20" :style="viewAllStyle" @click="goToSearchPage">
                  <span>{{ $t('apUi1.k36') }}</span>
                  <ExternalLink :size="14" />
                </div>
              </div>
            </div>

            <div v-else-if="hasSearched && results.length === 0" :style="emptyStyle">
              <ApEmpty icon="Search">{{ $t('apUi1.k37') }}</ApEmpty>
            </div>

            <div v-if="hasSearched && liveSuggestTerms.length > 0" :style="historySectionStyle">
              <div :style="suggestionHeaderStyle">
                <span :style="suggestionTitleStyle">{{ $t('apUi1.k38') }}</span>
              </div>
              <div :style="historyListStyle">
                <div
                  v-for="(term, idx) in liveSuggestTerms"
                  :key="term"
                  :style="suggestionItemStyle(suggestStartIndex + idx === selectedIndex)"
                  @click="handleLiveSuggestionClick(term)"
                  @mouseenter="selectedIndex = suggestStartIndex + idx"
                >
                  <FileText :size="16" :style="itemIconStyle" />
                  <span :style="itemTextStyle" :title="term">{{ term }}</span>
                </div>
              </div>
            </div>

            <div v-if="!hasSearched && suggestTerms.length > 0" :style="historySectionStyle">
              <div :style="suggestionHeaderStyle">
                <span :style="suggestionTitleStyle">{{ $t('apUi1.k39') }}</span>
              </div>
              <div :style="historyListStyle">
                <div
                  v-for="(term, idx) in suggestTerms"
                  :key="term"
                  :style="suggestionItemStyle(suggestStartIndex + idx === selectedIndex)"
                  @click="keyword = term; handleHistoryClick(term)"
                  @mouseenter="selectedIndex = suggestStartIndex + idx"
                >
                  <Search :size="16" :style="itemIconStyle" />
                  <span :style="itemTextStyle" :title="term">{{ term }}</span>
                </div>
              </div>
            </div>

            <div v-if="!hasSearched && searchHistory.length > 0" :style="historySectionStyle">
              <div :style="suggestionHeaderStyle">
                <span :style="suggestionTitleStyle">{{ $t('apUi1.k40') }}</span>
                <ApButton variant="ghost" size="small" @click="clearHistory">{{ $t('apUi1.k41') }}</ApButton>
              </div>
              <div :style="historyListStyle">
                <div
                  v-for="(item, idx) in searchHistory"
                  :key="item"
                  :style="historyItemStyle(historyStartIndex + idx === selectedIndex)"
                  :tabindex="historyStartIndex + idx === selectedIndex ? 0 : -1"
                  role="option"
                  :aria-selected="historyStartIndex + idx === selectedIndex"
                  @click="handleHistoryClick(item)"
                  @keydown.enter="handleHistoryClick(item)"
                  @keydown.space.prevent="handleHistoryClick(item)"
                  @mouseenter="selectedIndex = historyStartIndex + idx"
                >
                  <Clock :size="16" :style="itemIconStyle" />
                  <span :style="itemTextStyle" :title="item">{{ item }}</span>
                </div>
              </div>
            </div>
          </div>
        </div>
        <div :style="hintStyle">
          <kbd :style="kbdStyle">Enter</kbd> {{ $t('apUi1.confirm2') }}
          <span :style="hintDividerStyle">·</span>
          <kbd :style="kbdStyle">↑↓</kbd> {{ $t('apUi1.k24') }}
          <span :style="hintDividerStyle">·</span>
          <kbd :style="kbdStyle">Esc</kbd> {{ $t('apUi1.close') }}
        </div>
      </div>
    </transition>
  </Teleport>
</template>

<script setup lang="ts">
import { ref, shallowRef, nextTick } from 'vue';
import { Search, Loader2, Clock, ExternalLink, FileText } from 'lucide-vue-next';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApButton from '@/components/apple/ApButton.vue';
import SpotlightSearchResult from '@/components/apple/SpotlightSearchResult.vue';
import { useSpotlightSearch } from '@/composables/ui/useSpotlightSearch';
import { overlayStyle, containerStyle, searchBoxStyle, searchIconStyle, inputStyle, loadingIconStyle, resultsStyle, loadingStyle, suggestionHeaderStyle, suggestionTitleStyle, resultCountStyle, suggestionListStyle, suggestionItemStyle, itemIconStyle, itemTextStyle, itemShortcutStyle, emptyStyle, historySectionStyle, historyListStyle, historyItemStyle, viewAllStyle, hintStyle, kbdStyle, hintDividerStyle, recentSpaceStyle } from '@/styles/component/spotlight-search-styles';

const emit = defineEmits<{
  (e: 'close'): void;
}>();

const inputRef = ref<HTMLInputElement | null>(null);
const visible = shallowRef(false);

function close() {
  visible.value = false;
  emit('close');
}

const {
  keyword,
  selectedIndex,
  results,
  hasSearched,
  loading,
  showResults,
  suggestTerms,
  liveSuggestTerms,
  searchHistory,
  suggestions,
  recentDocs,
  recentStartIndex,
  historyStartIndex,
  suggestStartIndex,
  handleKeydown,
  handleResultClick,
  handleSuggestionClick,
  handleRecentDocClick,
  handleHistoryClick,
  handleLiveSuggestionClick,
  getHighlightSnippet,
  goToSearchPage,
  clearHistory,
  resetState,
} = useSpotlightSearch(close);

function open() {
  visible.value = true;
  resetState();
  nextTick(() => {
    inputRef.value?.focus();
  });
}

defineExpose({ open, close });
</script>

<style scoped>
.spotlight-enter-active {
  transition: opacity var(--apple-dur-interactive) ease;
}
.spotlight-leave-active {
  transition: opacity var(--apple-dur-fast) ease;
}
.spotlight-enter-from,
.spotlight-leave-to {
  opacity: 0;
}
.spotlight-enter-active > div,
.spotlight-leave-active > div {
  transition: transform var(--apple-dur-interactive) var(--apple-easing-emphasized), opacity var(--apple-dur-interactive) ease;
}
.spotlight-enter-from > div,
.spotlight-leave-to > div {
  transform: scale(0.98) translateY(-8px);
  opacity: 0;
}

.spinner {
  animation: spin var(--apple-spinner-duration) linear infinite;
}
</style>