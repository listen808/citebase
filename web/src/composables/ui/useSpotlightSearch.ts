import { ref, shallowRef, computed, watch, onBeforeUnmount } from 'vue';
import { useRouter } from 'vue-router';
import { icons } from '@/utils/icons';
import { fulltextSearch, searchSuggest } from '@/api/modules/document/search';
import type { SearchResult } from '@/types/models';
import { sanitizeAndHighlightHighlight } from '@/utils/domain/search-highlight';
import { useRequest } from '@/composables/shared/useRequest';
import { useUIStore } from '@/stores/app/ui';
import { sortResults, expandQuery, type SortType } from '@/utils/domain/search-enhanced';
import { useRecentDocs, type RecentDoc } from '@/composables/ui/useRecentDocs';
import { storageGet, storageSet, storageRemove, StorageKeys } from '@/utils/core/storage';
import i18n from '@/i18n';

const { t } = i18n.global;

const MAX_HISTORY = 10;
const DEBOUNCE_DELAY = 150;
const SUGGEST_DEBOUNCE_DELAY = 200;

export function useSpotlightSearch(onClose: () => void) {
  const router = useRouter();
  const ui = useUIStore();
  const { docs: recentDocs, loadRecentDocs } = useRecentDocs();

  const keyword = shallowRef('');
  const selectedIndex = shallowRef(0);
  const results = ref<SearchResult[]>([]);
  const hasSearched = shallowRef(false);
  const clearHistoryHovered = shallowRef(false);
  const suggestTerms = ref<string[]>([]);
  const liveSuggestTerms = ref<string[]>([]);
  const searchHistory = ref<string[]>([]);

  let searchTimer: ReturnType<typeof setTimeout> | null = null;
  let suggestTimer: ReturnType<typeof setTimeout> | null = null;

  function loadSearchHistory() {
    try {
      const stored = storageGet(StorageKeys.SEARCH_SPOTLIGHT_HISTORY);
      if (stored) {
        searchHistory.value = JSON.parse(stored);
      }
    } catch {
      searchHistory.value = [];
    }
  }

  function saveSearchHistory() {
    try {
      storageSet(StorageKeys.SEARCH_SPOTLIGHT_HISTORY, JSON.stringify(searchHistory.value));
    } catch { /* noop */ }
  }

  function addToHistory(kw: string) {
    if (!kw.trim()) return;
    const trimmed = kw.trim();
    searchHistory.value = [trimmed, ...searchHistory.value.filter(h => h.toLowerCase() !== trimmed.toLowerCase())].slice(0, MAX_HISTORY);
    saveSearchHistory();
  }

  function clearHistory() {
    searchHistory.value = [];
    storageRemove(StorageKeys.SEARCH_SPOTLIGHT_HISTORY);
  }

  const suggestions = [
    { id: 'home', label: t('tsUi.k44'), icon: icons.Home, shortcut: '', action: () => {
        onClose();
        router.push('/');
      },
    },
    { id: 'new', label: t('tsUi.k45'), icon: icons.Plus, shortcut: '', action: () => {
        onClose();
        const currentSpaceId = window.location.pathname.match(/\/space\/(\d+)/)?.[1];
        if (currentSpaceId) {
          router.push({ path: `/space/${currentSpaceId}`, query: { createDoc: 'true' } });
        } else {
          ui.showToast(t('tsUi.k48'));
        }
      },
    },
    { id: 'profile', label: t('tsUi.k46'), icon: icons.User, shortcut: '', action: () => {
        onClose();
        router.push('/profile');
      },
    },
    { id: 'settings', label: t('tsUi.k47'), icon: icons.Settings, shortcut: '', action: () => {
        onClose();
        router.push('/settings');
      },
    },
  ];

  const { loading, execute: execSearch, abort: abortSearch } = useRequest(
    async (query: string) => {
      // 同义词全变体并发搜索后去重合并（修复：原仅取第一个变体，其余丢弃）
      const expandedQueries = expandQuery(query);
      const queries = expandedQueries.length > 0 ? expandedQueries : [query];
      const responses = await Promise.all(
        queries.map(q =>
          fulltextSearch({ query: q, limit: 20 }).catch(() => ({ results: [] as SearchResult[], total: 0 })),
        ),
      );
      const seen = new Set<number>();
      const merged: SearchResult[] = [];
      for (const res of responses) {
        for (const r of res.results || []) {
          const key = r.document_id ?? 0;
          if (key > 0 && !seen.has(key)) {
            seen.add(key);
            merged.push(r);
          } else if (key === 0 && !seen.has(r.space_id)) {
            seen.add(r.space_id);
            merged.push(r);
          }
        }
      }
      return sortResults(merged, 'relevance' as SortType, query);
    },
    {
      onSuccess: (data) => {
        results.value = data;
      },
    },
  );

  const showResults = computed(() => {
    return keyword.value.trim().length > 0 || suggestions.length > 0 || searchHistory.value.length > 0;
  });

  const recentStartIndex = computed(() => {
    return suggestions.length;
  });

  const historyStartIndex = computed(() => {
    return keyword.value.trim() ? 0 : suggestions.length + recentDocs.value.length;
  });

  const suggestStartIndex = computed(() => {
    if (keyword.value.trim()) {
      return 0;
    }
    return suggestions.length + recentDocs.value.length + searchHistory.value.length;
  });

  const maxIndex = computed(() => {
    let count: number;
    if (keyword.value.trim()) {
      count = results.value.length + liveSuggestTerms.value.length;
    } else {
      count = suggestions.length + recentDocs.value.length + searchHistory.value.length + suggestTerms.value.length;
    }
    return Math.max(0, count - 1);
  });

  // 补全词与主搜索各走一条 useRequest：它按 requestId 丢弃陈旧响应（并且 execute 自己
  // 会 abort 上一次），所以快速连打时先发的请求晚到也不会覆盖后发的结果。
  // 手写一套计数器就是把这条契约再抄一遍 —— SearchView 的 AbortController 走的是同一思路。
  const { execute: execLiveSuggest, abort: abortLiveSuggest } = useRequest(
    async (query: string) => {
      const terms = await searchSuggest(query);
      return (terms || []).slice(0, 5);
    },
    {
      onSuccess: (terms) => { liveSuggestTerms.value = terms ?? []; },
      onError: () => { liveSuggestTerms.value = []; },
    },
  );

  async function loadSuggestTerms() {
    try {
      const terms = await searchSuggest('');
      suggestTerms.value = (terms || []).slice(0, 5);
    } catch {
      suggestTerms.value = [];
    }
  }

  function loadLiveSuggestions(query: string) {
    if (!query.trim()) {
      abortLiveSuggest();
      liveSuggestTerms.value = [];
      return;
    }
    void execLiveSuggest(query);
  }

  function resetState() {
    keyword.value = '';
    selectedIndex.value = 0;
    hasSearched.value = false;
    results.value = [];
    abortSearch();
    abortLiveSuggest();
    liveSuggestTerms.value = [];
    loadSearchHistory();
    loadSuggestTerms();
    loadRecentDocs();
  }

  watch(keyword, (newVal) => {
    if (searchTimer) {
      clearTimeout(searchTimer);
    }
    if (suggestTimer) {
      clearTimeout(suggestTimer);
    }

    if (newVal.trim()) {
      hasSearched.value = true;
      searchTimer = setTimeout(() => {
        execSearch(newVal);
      }, DEBOUNCE_DELAY);
      suggestTimer = setTimeout(() => {
        loadLiveSuggestions(newVal);
      }, SUGGEST_DEBOUNCE_DELAY);
    } else {
      hasSearched.value = false;
      results.value = [];
      // 清空输入时把在途的补全请求一起作废：否则晚到的那一份会把空结果又填回去
      abortLiveSuggest();
      abortSearch();
      liveSuggestTerms.value = [];
    }
    selectedIndex.value = 0;
  });

  function handleKeydown(e: KeyboardEvent) {
    switch (e.key) {
      case 'Escape':
        e.preventDefault();
        onClose();
        break;
      case 'Enter':
        e.preventDefault();
        handleSelect();
        break;
      case 'ArrowUp':
        e.preventDefault();
        selectedIndex.value = Math.max(0, selectedIndex.value - 1);
        break;
      case 'ArrowDown':
        e.preventDefault();
        selectedIndex.value = Math.min(maxIndex.value, selectedIndex.value + 1);
        break;
    }
  }

  function handleSelect() {
    if (keyword.value.trim() && results.value.length > 0) {
      const result = results.value[selectedIndex.value];
      if (result) {
        handleResultClick(result);
      }
    } else if (keyword.value.trim() && liveSuggestTerms.value.length > 0) {
      const suggestIdx = selectedIndex.value;
      if (suggestIdx >= 0 && suggestIdx < liveSuggestTerms.value.length) {
        const term = liveSuggestTerms.value[suggestIdx];
        if (term) {
          handleLiveSuggestionClick(term);
        }
      }
    } else if (!keyword.value.trim()) {
      if (selectedIndex.value < suggestions.length) {
        const suggestion = suggestions[selectedIndex.value];
        if (suggestion) {
          handleSuggestionClick(suggestion);
        }
      } else if (selectedIndex.value < suggestions.length + recentDocs.value.length) {
        const docIdx = selectedIndex.value - suggestions.length;
        const doc = recentDocs.value[docIdx];
        if (doc) {
          handleRecentDocClick(doc);
        }
      } else {
        const historyIdx = selectedIndex.value - suggestions.length - recentDocs.value.length;
        if (historyIdx >= 0 && historyIdx < searchHistory.value.length) {
          const historyItem = searchHistory.value[historyIdx];
          if (historyItem) {
            handleHistoryClick(historyItem);
          }
        }
      }
    }
  }

  function handleRecentDocClick(doc: RecentDoc) {
    onClose();
    router.push(`/space/${doc.space_id}/doc/${doc.id}`);
  }

  function handleResultClick(result: SearchResult) {
    addToHistory(keyword.value);
    onClose();
    if (result.document_id) {
      router.push({
        path: `/space/${result.space_id}/doc/${result.document_id}`,
        query: { source: 'spotlight', q: keyword.value }
      });
    } else if (result.space_id) {
      router.push({
        path: `/space/${result.space_id}`,
        query: { source: 'spotlight' }
      });
    }
  }

  function handleSuggestionClick(suggestion: typeof suggestions[0]) {
    onClose();
    suggestion.action();
  }

  function handleHistoryClick(kw: string) {
    keyword.value = kw;
    addToHistory(kw);
  }

  function handleLiveSuggestionClick(term: string) {
    if (!term) return;
    addToHistory(term);
    onClose();
    router.push({ path: '/search', query: { q: term } });
  }

  function getHighlightSnippet(result: SearchResult): string {
    if (result.highlights && Object.keys(result.highlights).length > 0) {
      const snippet = Object.values(result.highlights)[0] as string;
      const truncated = snippet.length > 100 ? snippet.slice(0, 100) + '...' : snippet;
      return sanitizeAndHighlightHighlight(truncated, keyword.value);
    }
    return '';
  }

  function goToSearchPage() {
    onClose();
    router.push({ path: '/search', query: { q: keyword.value } });
  }

  onBeforeUnmount(() => {
    if (searchTimer) clearTimeout(searchTimer);
    if (suggestTimer) clearTimeout(suggestTimer);
  });

  return {
    keyword,
    selectedIndex,
    results,
    hasSearched,
    loading,
    showResults,
    clearHistoryHovered,
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
    handleSelect,
    getHighlightSnippet,
    goToSearchPage,
    clearHistory,
    resetState,
  };
}