<template>
  <ApPageContainer
    :title="$t('search.search')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <SearchHeader
      v-model:keyword="keyword"
      v-model:mode="mode"
      v-model:active-tab="activeTab"
      v-model:sort-by="sortBy"
      v-model:search-type="searchType"
      v-model:search-mode="searchMode"
      v-model:selected-space-id="selectedSpaceId"
      v-model:selected-tag="selectedTag"
      v-model:time-range="timeRange"
      :loading="loading"
      :searched="searched"
      :search-history="searchHistory"
      :space-options="spaceOptions"
      :tag-options="tagOptions"
      :tab-options="tabOptions"
      :has-results="rawResults.length > 0"
      @search="onApSearch"
      @select-suggestion="onSuggestionSelect"
      @clear-history="showClearHistoryModal = true"
      @space-change="debouncedSearch"
      @search-type-change="debouncedSearch"
      @search-mode-change="debouncedSearch"
      @tag-change="debouncedSearch"
      @time-range-change="debouncedSearch"
    />

    <template v-if="mode === 'ai'">
      <div v-if="!searched" :style="aiHintStyle">
        <p>{{ $t('search.k77') }}</p>
      </div>
      <div v-else-if="aiLoading && !aiAnswer" :style="loadingStyle">
        <ApSkeleton variant="card" :lines="4" />
      </div>
      <ApError v-else-if="aiError" :message="aiError" show-retry @retry="runAIMode(keyword)" />
      <div v-else-if="aiAnswer" :style="aiAnswerStyle">
        <div :style="aiAnswerContentStyle" v-html="renderedAiHtml" />
      </div>
      <ApEmpty v-else :title="$t('search.k80')" :description="$t('vSearchUi.k4')" />
    </template>

    <template v-else-if="mode === 'search'">
      <div v-if="loading && rawResults.length === 0" :style="loadingStyle">
        <ApSkeleton variant="card" :lines="3" />
        <ApSkeleton variant="card" :lines="3" />
        <ApSkeleton variant="card" :lines="3" />
      </div>

      <template v-else-if="searched">
        <ApError
          v-if="searchError"
          :message="searchError"
          show-retry
          @retry="handleSearch"
        />
        <div v-else-if="filteredResults.length > 0">
          <div :style="statsStyle">
            {{ $t('uiTextMain.k46', { p1: filteredResults.length }) }}
          </div>
          <div :style="resultsListStyle">
            <div :style="listStyle(AppleSpace.sm)">
              <div
                v-for="(r, idx) in filteredResults"
                :key="r.document_id || r.space_id"
                v-bind="keyboardClick(() => handleResultClick(r))"
                :style="{ ...resultCardStyle(activeResultIndex === idx) }"
                :data-result-index="idx"
                @mouseenter="activeResultIndex = idx"
                @mouseleave="activeResultIndex = -1"
                @touchstart="activeResultIndex = idx"
                @focusin="activeResultIndex = idx"
                @focusout="activeResultIndex = -1"
              >
                <!-- 头部：面包屑路径 + 元信息 -->
                <div :style="resultHeaderStyle">
                  <div :style="breadcrumbStyle">
                    <ApIcon :name="r.document_id ? 'FileText' : 'Folder'" :size="14" :color="cv('text-tertiary')" />
                    <span :style="breadcrumbItemStyle">{{ r.space_name || $t('fallback.k79') }}</span>
                    <template v-if="r.document_id && r.path">
                      <span :style="breadcrumbSepStyle">/</span>
                      <span :style="breadcrumbItemStyle">{{ r.path }}</span>
                    </template>
                  </div>
                  <div :style="metaInfoStyle">
                    <span :style="relativeTimeStyle">{{ formatRelativeTime(r.updated_at || r.create_time) }}</span>
                    <span v-if="r.match_count && r.match_count > 0" :style="matchBadgeStyle">
                      {{ r.match_count }} {{ $t('vSearchUi.k9') }}
                    </span>
                    <span v-if="r.score_detail && r.score_detail.fusion_method !== 'fulltext'" :style="scoreDetailBadgeStyle">
                      {{ r.score_detail.fusion_method === 'semantic' ? $t('vSearchUi.k7') : $t('vSearchUi.k8') }}
                    </span>
                  </div>
                </div>

                <!-- 标题：独立一行，更醒目 -->
                <div :style="resultTitleStyle" v-html="highlightedName(r.name)" />

                <!-- 片段展示：卡片样式 + 上下文 -->
                <template v-if="getFragments(r).length > 0">
                  <div :style="fragmentsContainerStyle">
                    <div
                      v-for="(frag, fi) in getFragments(r).slice(0, isResultExpanded(r) ? getFragments(r).length : 2)"
                      :key="fi"
                      :style="fragmentCardStyle"
                    >
                      <div :style="fragmentHeaderStyle">
                        <ApIcon name="FileText" :size="12" :color="cv('text-tertiary')" />
                        <span :style="fragmentLabelStyle">{{ $t('uiTextMain.k47', { p1: fi + 1 }) }}</span>
                        <span :style="fragmentScoreStyle">{{ $t('uiTextMain.k48', { p1: Math.round(frag.score * 100) }) }}</span>
                      </div>
                      <div :style="fragmentContentStyle">
                        <span v-if="fi > 0" :style="ellipsisStyle">...</span>
                        <span v-html="highlightedFragment(frag.html)" />
                        <span v-if="fi < getFragments(r).length - 1" :style="ellipsisStyle">...</span>
                      </div>
                    </div>
                    <ApButton
                      v-if="getFragments(r).length > 2"
                      size="small"
                      variant="ghost"
                      @click.stop="toggleResultExpand(r)"
                    >
                      <ApIcon :name="isResultExpanded(r) ? 'ChevronUp' : 'ChevronDown'" :size="16" :color="cv('blue')" />
                      <span>{{ isResultExpanded(r) ? $t('uiTextMain.k49') : $t('uiTextMain.k50', { p1: getFragments(r).length - 2 }) }}</span>
                    </ApButton>
                  </div>
                </template>
                <div
                  v-else-if="r.path"
                  :style="resultPathMetaStyle"
                  v-html="highlightedName(r.path)"
                />

                <!-- 快速操作按钮：hover 时显示 -->
                <div :style="resultQuickActionsStyle(activeResultIndex === idx)">
                  <ApButton
                    size="small"
                    variant="ghost"
                    icon="ExternalLink"
                    :tooltip="$t('search.k84')"
                    :aria-label="$t('search.k84')"
                    @click.stop="openInNewTab(r)"
                  />
                </div>
              </div>
            </div>
          </div>
          <!-- 无限滚动 sentinel -->
          <div v-if="hasMore" ref="sentinelRef" :style="{ height: '1px' }" />
          <div v-if="loadingMore" :style="loadingStyle">
            <ApSkeleton variant="card" :lines="2" />
          </div>
        </div>
        <div v-else :style="emptyStyle">
          <div v-if="spellSuggestions.length > 0" :style="{ marginBottom: AppleSpace.md, textAlign: 'center' as const }">
            <span :style="{ color: cv('text-secondary') }">{{ $t('search.k78') }}</span>
            <span
              v-for="s in spellSuggestions"
              :key="s"
              :style="{ color: cv('blue'), cursor: 'pointer', marginLeft: AppleSpace.sm, fontWeight: String(AppleFontWeight.medium) }"
              @click="keyword = s; handleSearch()"
            >{{ s }}</span>
          </div>
          <ApEmpty type="search" :title="$t('search.k81')" :description="$t('vSearchUi.k5')" />
        </div>
      </template>
    </template>

    <ApConfirmModal
      v-model="showClearHistoryModal"
      :title="$t('search.k82')"
      :message="$t('vSearchUi.k6')"
      :confirm-text="$t('search.k83')"
      type="danger"
      @confirm="clearHistory"
    />
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, onMounted, onUnmounted, computed, watch } from 'vue';
import { logger } from '@/utils/logger';
import { debounce } from '@/utils/core/shared';
import { sanitizeMarkdownHtml } from '@/utils/core/markdown-sanitize';
import { useRouter, useRoute } from 'vue-router';
import ApIcon from '@/components/apple/ApIcon.vue';
import { fulltextSearch, hybridSearch, getSearchTags, searchSuggest } from '@/api/modules/document/search';
import type { HybridSearchMode } from '@/api/modules/document/search';
import { streamAsk } from '@/api/modules/ai/ai-chat';
import type { SearchResult } from '@/types/models';
import { AppleLayout, AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleType, cv, AppleLineHeight, AppleTransition } from '@/utils/core/apple-tokens';
import { keyboardClick } from '@/composables/ui/useKeyboardClick';
import { loadingStyle, emptyStateStyle as emptyStyle, listStyle, searchResultCardStyle } from '@/styles/layout/page-styles';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApError from '@/components/apple/ApError.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApButton from '@/components/apple/ApButton.vue';
import { highlightText, getHighlightFragments, sanitizeFragmentHTML } from '@/utils/domain/search-highlight';
import { sortResults, expandQuery, spellCheck, type SortType } from '@/utils/domain/search-enhanced';
import { useSpaceStore } from '@/stores/workspace/space';
import { useUIStore } from '@/stores/app/ui';
import { storageGet, storageSet, storageRemove, StorageKeys } from '@/utils/core/storage';
import SearchHeader from '@/components/search/SearchHeader.vue';
import { useSearchKeyboard } from '@/composables/ui/useSearchKeyboard';
import { useAIStore } from '@/stores/ai/ai';
import { formatDate, formatTimeAgo } from '@/utils/format/format-time';
import { formatRelativeTime as formatRelativeTimeDefault } from '@/utils/format/format-time';

const router = useRouter();
const route = useRoute();
const spaceStore = useSpaceStore();
const ui = useUIStore();
const aiStore = useAIStore();

const keyword = shallowRef('');
const rawResults = ref<SearchResult[]>([]);
const sortBy = ref<SortType>('relevance');
const searched = shallowRef(false);
const activeTab = ref<'all' | 'document' | 'space'>('all');
const mode = ref<'search' | 'ai'>('search');

// === AI 问答模式 ===
const aiAnswer = ref('');
const aiLoading = shallowRef(false);
const aiError = shallowRef('');
let aiAbortController: AbortController | null = null;

function runAIMode(query: string) {
  if (!query.trim()) return;
  // 双保险：入口按钮已按 aiStore.enabled 隐藏，这里拦住调用本身
  if (!aiStore.enabled) return;
  aiLoading.value = true;
  aiError.value = '';
  aiAnswer.value = '';
  aiAbortController?.abort();
  aiAbortController = streamAsk(
    { question: query, limit: 10, history: [] },
    (chunk) => { aiAnswer.value += chunk; },
    () => { aiAbortController = null; aiLoading.value = false; },
    (err) => { aiAbortController = null; aiLoading.value = false; aiError.value = err; },
  );
}
const isInitialLoad = shallowRef(true);
const searchType = ref<'all' | 'title' | 'content'>('all');
const searchMode = ref<HybridSearchMode>('hybrid');
const selectedSpaceId = shallowRef<number | null>(null);
const selectedTag = ref<string>('');
const timeRange = ref<string>('all');
const tagOptions = ref<Array<{ label: string; value: string }>>([]);

const selectedIndex = ref(-1);
const expandedResults = ref<Set<number>>(new Set());
// 快捷操作位：指针悬停 **或** 键盘聚焦进卡片都要显形（apple-design §16 Agency ——
// 只挂 hover 时键盘用户看不见这几个按钮，也不知道结果卡上还有操作）。
const activeResultIndex = ref(-1);

let searchAbortController: AbortController | null = null;

const loading = shallowRef(false);
const loadingMore = shallowRef(false);
const hasMore = shallowRef(false);
const spellSuggestions = ref<string[]>([]);
const searchError = shallowRef('');

const PAGE_SIZE = 50;
const sentinelRef = ref<HTMLElement | null>(null);
let intersectionObserver: IntersectionObserver | null = null;

const MAX_HISTORY = 10;
const searchHistory = ref<string[]>([]);
const showClearHistoryModal = shallowRef(false);

function loadSearchHistory() {
  try {
    const stored = storageGet(StorageKeys.SEARCH_HISTORY);
    if (stored) searchHistory.value = JSON.parse(stored);
  } catch { searchHistory.value = []; }
}

function saveSearchHistory() {
  try { storageSet(StorageKeys.SEARCH_HISTORY, JSON.stringify(searchHistory.value)); }
  catch (e) { logger.error('Failed to save search history:', e); ui.showToast(t('search.k85'), 'error'); }
}

function addToHistory(kw: string) {
  if (!kw.trim()) return;
  const trimmed = kw.trim();
  searchHistory.value = [trimmed, ...searchHistory.value.filter(h => h.toLowerCase() !== trimmed.toLowerCase())].slice(0, MAX_HISTORY);
  saveSearchHistory();
}

function clearHistory() {
  searchHistory.value = [];
  storageRemove(StorageKeys.SEARCH_HISTORY);
}

const documentResults = computed(() => rawResults.value.filter(r => r.document_id));
const spaceResults = computed(() => rawResults.value.filter(r => !r.document_id && r.space_id));

const tabOptions = computed(() => [
  { label: t('sentUi.k52', { p1: rawResults.value.length }), value: 'all' },
  { label: t('sentUi.k53', { p1: documentResults.value.length }), value: 'document' },
  { label: t('sentUi.k54', { p1: spaceResults.value.length }), value: 'space' },
]);

const filteredResults = computed(() => {
  let filtered = rawResults.value;
  if (activeTab.value === 'document') filtered = documentResults.value;
  else if (activeTab.value === 'space') filtered = spaceResults.value;
  const sorted = sortResults(filtered, sortBy.value, keyword.value);
  // 归一化相关性分数：基于结果集中的最高分计算百分比
  const maxScore = Math.max(...sorted.map((r) => r.score || 0), 0.0001);
  return sorted.map((r) => ({ ...r, _normScore: (r.score || 0) / maxScore }));
});

const spaceOptions = computed(() => {
  const spaces = spaceStore.mySpaces || [];
  return [{ label: t('search.k22'), value: 0 }, ...spaces.map(s => ({ label: s.name, value: s.id }))];
});

function getFragments(r: SearchResult) {
  return getHighlightFragments(r);
}

function isResultExpanded(r: SearchResult): boolean {
  const key = r.document_id || r.space_id || 0;
  return expandedResults.value.has(key);
}

function toggleResultExpand(r: SearchResult) {
  const key = r.document_id || r.space_id || 0;
  const newSet = new Set(expandedResults.value);
  if (newSet.has(key)) {
    newSet.delete(key);
  } else {
    newSet.add(key);
  }
  expandedResults.value = newSet;
}

// 高亮样式只有一份：注入层的 mark.highlight 规则（utils/core/apple-tokens.ts）。
// 这里原先还把 class 换成内联 style，于是搜索页与阅读页/Spotlight 的命中色各走一路。
function highlightedName(name: string): string {
  return highlightText(name, keyword.value);
}

function highlightedFragment(html: string): string {
  return sanitizeFragmentHTML(html);
}

function resultCardStyle(hovered: boolean): Record<string, string> {
  return {
    ...searchResultCardStyle(),
    padding: `${AppleSpace.sm} ${AppleSpace.sm}`,
    position: 'relative',
    borderColor: hovered ? cv('blue') : cv('border-light'),
    boxShadow: hovered ? cv('shadow-card-hover-refined') : cv('shadow-card'),
    transform: hovered ? 'translateY(-2px)' : 'translateY(0)',
  };
}

function resultQuickActionsStyle(visible: boolean): Record<string, string> {
  return {
    ...quickActionsStyle,
    opacity: visible ? '1' : '0',
  };
}

async function execSearch(query: string, offset: number = 0): Promise<SearchResult[]> {
  searchAbortController?.abort();
  searchAbortController = new AbortController();
  const signal = searchAbortController.signal;
  try {
    // Calculate time range
    const now = Math.floor(Date.now() / 1000);
    let timeFrom: number | undefined;
    if (timeRange.value === 'today') {
      timeFrom = now - 86400;
    } else if (timeRange.value === 'week') {
      timeFrom = now - 86400 * 7;
    } else if (timeRange.value === 'month') {
      timeFrom = now - 86400 * 30;
    }

    // Use hybrid search for hybrid/semantic modes
    if (searchMode.value === 'hybrid' || searchMode.value === 'semantic') {
      const hybridRes = await hybridSearch(
        {
          query,
          search_mode: searchMode.value,
          space_ids: selectedSpaceId.value ? [selectedSpaceId.value] : undefined,
          tags: selectedTag.value ? [selectedTag.value] : undefined,
          date_from: timeFrom ? String(timeFrom) : undefined,
          page: Math.floor(offset / PAGE_SIZE) + 1,
          page_size: PAGE_SIZE,
        },
        { signal },
      );
      if (signal.aborted) return [];
      // Map hybrid results to SearchResult format for rendering compatibility
      return (hybridRes.results || []).map((r) => ({
        document_id: r.document_id,
        space_id: r.space_id,
        space_name: r.space_name,
        name: r.title,
        type: r.type,
        path: r.path,
        score: r.score,
        highlights: {},
        create_time: r.updated_at ? Number(r.updated_at) : 0,
        score_detail: r.score_detail,
      }));
    }

    // Fulltext mode: use the original search with synonym expansion
    const expandedQueries = expandQuery(query);
    const seen = new Set<number>();
    const allResults: SearchResult[] = [];

    const sortByMap: Record<string, 'relevance' | 'time_desc' | 'time_asc'> = {
      relevance: 'relevance',
      time: 'time_desc',
    };
    const apiSortBy = sortByMap[sortBy.value] || 'relevance';

    for (const q of expandedQueries) {
      if (signal.aborted) return [];
      const res = await fulltextSearch(
        {
          query: q,
          limit: PAGE_SIZE,
          offset,
          space_id: selectedSpaceId.value ?? undefined,
          type: searchType.value,
          tag_name: selectedTag.value || undefined,
          time_from: timeFrom,
          sort_by: apiSortBy,
        },
        { signal },
      );
      if (signal.aborted) return [];
      for (const r of res.results || []) {
        const key = r.document_id || r.space_id || 0;
        if (!seen.has(key)) {
          seen.add(key);
          allResults.push(r);
        }
      }
    }
    return allResults;
  } catch (e) {
    if (signal.aborted || (e instanceof Error && e.name === 'AbortError')) return [];
    throw e;
  }
}

function onApSearch(value: string) {
  if (!value.trim()) {
    rawResults.value = [];
    searched.value = false;
    activeTab.value = 'all';
    return;
  }
  // AI 模式：搜索栏回车直接发起 AI 问答
  if (mode.value === 'ai') {
    searched.value = true;
    addToHistory(value);
    runAIMode(value);
    return;
  }
  handleSearch();
}

function onSuggestionSelect(value: string) {
  keyword.value = value;
  if (mode.value === 'ai') {
    searched.value = true;
    runAIMode(value);
    return;
  }
  handleSearch();
}

async function handleSearch() {
  if (!keyword.value.trim()) return;
  searched.value = true;
  loading.value = true;
  searchError.value = '';
  spellSuggestions.value = [];
  hasMore.value = false;
  try {
    const results = await execSearch(keyword.value, 0);
    if (results.length === 0 && searchAbortController?.signal.aborted) return;
    rawResults.value = results;
    hasMore.value = results.length === PAGE_SIZE;
    addToHistory(keyword.value);

    // 拼写纠错：无结果时获取建议词并计算编辑距离
    if (results.length === 0) {
      try {
        const suggestTerms = await searchSuggest(keyword.value);
        if (suggestTerms && suggestTerms.length > 0) {
          spellSuggestions.value = spellCheck(keyword.value, suggestTerms, 2);
        }
      } catch { /* noop */ }
    }
  } catch {
    searchError.value = t('sentUi.k51');
    rawResults.value = [];
  } finally {
    loading.value = false;
  }
}

async function loadMore() {
  if (loadingMore.value || !hasMore.value || loading.value) return;
  loadingMore.value = true;
  try {
    const more = await execSearch(keyword.value, rawResults.value.length);
    if (more.length > 0) {
      // 去重合并
      const seen = new Set(rawResults.value.map(r => r.document_id || r.space_id || 0));
      const deduped = more.filter(r => {
        const key = r.document_id || r.space_id || 0;
        if (seen.has(key)) return false;
        seen.add(key);
        return true;
      });
      rawResults.value = [...rawResults.value, ...deduped];
      hasMore.value = more.length === PAGE_SIZE;
    } else {
      hasMore.value = false;
    }
  } catch {
    hasMore.value = false;
  } finally {
    loadingMore.value = false;
  }
}

const debouncedSearch = debounce(() => handleSearch(), 300);

// 轻量渲染 AI 回答（marked 已在依赖中），消毒统一走 markdown-sanitize.ts
const _markedPromise = import('marked').then((m) => (m as { marked: (s: string) => string }).marked);
const renderedAiHtml = shallowRef('');

async function renderAIMarkdownToHtml(text: string): Promise<string> {
  try {
    const marked = await _markedPromise;
    const html = marked(text || '');
    return sanitizeMarkdownHtml(html);
  } catch {
    return (text || '').replace(/[<>&]/g, (c) => ({ '<': '&lt;', '>': '&gt;', '&': '&amp;' }[c] || c));
  }
}

// aiAnswer 变化时异步渲染为 HTML（v-html 不支持 Promise，需先 await 后赋值）
watch(aiAnswer, async (text) => {
  if (!text) {
    renderedAiHtml.value = '';
    return;
  }
  renderedAiHtml.value = await renderAIMarkdownToHtml(text);
});

// 模式切换离开 AI 时清空残留状态，避免切回 search 仍显示上一次回答
watch(mode, (newMode) => {
  if (newMode !== 'ai') {
    aiAnswer.value = '';
    renderedAiHtml.value = '';
    aiError.value = '';
  }
});

const aiHintStyle = {
  padding: `${AppleSpace.lg} 0`,
  textAlign: 'center' as const,
  color: cv('text-tertiary'),
  fontSize: AppleType.bodyPrimary.size,
};

const aiAnswerStyle = {
  padding: `${AppleSpace.md} 0`,
};

const aiAnswerContentStyle = {
  fontFamily: AppleType.bodyPrimary.font,
  fontSize: AppleType.bodyPrimary.size,
  lineHeight: String(AppleType.bodyPrimary.lineHeight),
  color: cv('text-primary'),
  wordBreak: 'break-word' as const,
};

function handleResultClick(r: SearchResult) {
  if (!r.document_id) {
    if (r.space_id) router.push(`/space/${r.space_id}`);
    return;
  }
  // 直接跳转到文档页面
  router.push({
    path: `/space/${r.space_id}/doc/${r.document_id}`,
    query: keyword.value ? { highlight: keyword.value } : undefined,
  });
}

// 键盘导航
const keyboardEnabled = computed(() => searched.value && filteredResults.value.length > 0);
useSearchKeyboard({
  resultsCount: computed(() => filteredResults.value.length),
  selectedIndex,
  onSelect: (index: number) => {
    const result = filteredResults.value[index];
    if (result) handleResultClick(result);
  },
  onClose: () => {
    // 关闭键盘导航时无需操作
  },
  enabled: keyboardEnabled,
});

// 满月以上的结果不再假装"近期"，退成 yyyy-MM-dd；窗口内的说法归 time.*（口径下沉 format-time.ts）。
const formatRelativeTime = (timestamp: number) => formatRelativeTimeDefault(timestamp);

function openInNewTab(r: SearchResult) {
  if (!r.document_id) return;
  const url = router.resolve({
    path: `/space/${r.space_id}/doc/${r.document_id}`,
    query: keyword.value ? { highlight: keyword.value } : undefined,
  });
  window.open(url.href, '_blank', 'noopener,noreferrer');
}

onMounted(() => {
  loadSearchHistory();
  spaceStore.loadMySpaces();
  loadTags();
  if (route.query.q) {
    keyword.value = route.query.q as string;
    handleSearch();
  }
  setTimeout(() => { isInitialLoad.value = false; }, 500);
});

// 无限滚动：监听 sentinel 元素，触底时加载更多
watch([sentinelRef, hasMore], ([el, more]) => {
  if (intersectionObserver) {
    intersectionObserver.disconnect();
    intersectionObserver = null;
  }
  if (el && more && !loadingMore.value) {
    intersectionObserver = new IntersectionObserver(
      (entries) => {
        if (entries[0]?.isIntersecting && hasMore.value && !loadingMore.value) {
          loadMore();
        }
      },
      { rootMargin: '100px' },
    );
    intersectionObserver.observe(el as HTMLElement);
  }
});

onUnmounted(() => {
  if (intersectionObserver) {
    intersectionObserver.disconnect();
    intersectionObserver = null;
  }
});

async function loadTags() {
  try {
    const res = await getSearchTags('', 100);
    tagOptions.value = (res.tags || []).map(tag => ({ label: tag, value: tag }));
  } catch (e) {
    logger.error('Failed to load tags:', e);
  }
}

watch(() => route.query.q, (newQuery) => {
  if (isInitialLoad.value) return;
  if (newQuery && newQuery !== keyword.value) {
    keyword.value = newQuery as string;
    handleSearch();
  }
}, { immediate: false });

onUnmounted(() => {
  searchAbortController?.abort();
  debouncedSearch.cancel();
});

const statsStyle = {
  fontFamily: AppleFont.text, fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'), marginBottom: AppleSpace.xs,
};

// === 结果卡片样式 ===

const resultHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  gap: AppleSpace.xs,
  marginBottom: AppleSpace.xxs,
};

const breadcrumbStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  overflow: 'hidden',
  flex: '1',
  minWidth: '0',
};

const breadcrumbItemStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const breadcrumbSepStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
  flexShrink: '0',
};

const metaInfoStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  flexShrink: '0',
};

const relativeTimeStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
};

const matchBadgeStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('blue'),
  background: cv('blue-light'),
  padding: `${AppleSpace.xxs} ${AppleSpace.xs}`,
  borderRadius: AppleRadius.micro,
};

const scoreDetailBadgeStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('purple'),
  background: cv('purple-light'),
  padding: `${AppleSpace.xxs} ${AppleSpace.xs}`,
  borderRadius: AppleRadius.micro,
};

const resultTitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  lineHeight: AppleLineHeight.tight,
  marginBottom: AppleSpace.xs,
};

// === 片段展示样式 ===

const fragmentsContainerStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
};

const fragmentCardStyle = {
  background: cv('bg-secondary'),
  border: `1px solid ${cv('border-light')}`,
  borderRadius: AppleRadius.small,
  padding: AppleSpace.xs,
};

const fragmentHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  marginBottom: AppleSpace.xxs,
};

const fragmentLabelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  flex: '1',
};

const fragmentScoreStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
  flexShrink: '0',
};

const fragmentContentStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  lineHeight: AppleLineHeight.relaxed,
  color: cv('text-secondary'),
  overflow: 'hidden',
  display: '-webkit-box',
  WebkitLineClamp: '2',
  WebkitBoxOrient: 'vertical' as const,
};

const ellipsisStyle = {
  color: cv('text-quaternary'),
  fontWeight: String(AppleFontWeight.medium),
};

// === 快速操作样式 ===

const quickActionsStyle = {
  position: 'absolute' as const,
  top: AppleSpace.xs,
  right: AppleSpace.xs,
  display: 'flex',
  gap: AppleSpace.xs,
  opacity: '0',
  transition: AppleTransition.default,
};

// === 布局样式 ===

const resultPathMetaStyle = {
  fontFamily: AppleFont.text, fontSize: AppleType.microUI.size,
  color: cv('text-quaternary'),
  overflow: 'hidden', textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const resultsListStyle = {
  width: '100%',
};
</script>