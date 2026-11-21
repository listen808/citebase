<template>
  <div>
    <div :style="searchBarStyle">
      <ApSearch
        v-model="keywordModel"
        :placeholder="searchPlaceholder"
        :suggestions="searchHistory"
        :show-suggestions="!searched"
        :style="{ flex: '1 1 0', minWidth: '0' }"
        @search="onApSearch"
        @select-suggestion="onSuggestionSelect"
      />
      <span v-if="loading" :style="spinnerWrapStyle"><ApIcon name="Loader2" :size="18" :style="spinnerStyle" /></span>
      <!-- AI 未启用时不给控件：只有一个选项的"切换"不是选择，以前那里孤零零亮着一颗「搜索」 -->
      <ApSegmentedControl
        v-if="aiStore.enabled"
        :model-value="mode"
        :options="modeOptions"
        size="small"
        :aria-label="$t('shell.k44')"
        @update:model-value="(v: string | number) => emit('update:mode', String(v))"
      />
    </div>


    <div v-if="!searched && searchHistory.length === 0 && mode === 'search'" :style="syntaxHintContainerStyle">
      <div :style="syntaxHintStyle">
        <span :style="syntaxHintTitleStyle">{{ $t('shell.k37') }}</span>
        <span :style="syntaxHintItemStyle">{{ $t('shell.k38') }}</span>
        <span :style="syntaxHintSepStyle">|</span>
        <span :style="syntaxHintItemStyle">| = OR</span>
        <span :style="syntaxHintSepStyle">|</span>
        <span :style="syntaxHintItemStyle">{{ $t('shell.k39') }}</span>
        <span :style="syntaxHintSepStyle">|</span>
        <span :style="syntaxHintItemStyle">{{ $t('shell.k40') }}</span>
      </div>
    </div>

    <div v-if="!searched && searchHistory.length > 0 && mode === 'search'" :style="historyRowStyle">
      <span :style="historyLabelStyle">{{ $t('shell.k41') }}</span>
      <span
        v-for="item in searchHistory"
        :key="item"
        :style="historyTagStyle"
        @click="onHistoryClick(item)"
      >
        {{ item }}
      </span>
      <ApButton variant="ghost" size="mini" icon-only :tooltip="'清除历史'" :aria-label="$t('shell.k45')" @click="$emit('clear-history')">
        <ApIcon name="Trash2" :size="16" />
      </ApButton>
    </div>

    <div v-if="searched" :style="crudFilterBarStyle('compact')">
      <template v-if="mode === 'search'">
        <ApSegmentedControl
          :model-value="activeTab"
          :options="tabOptions"
          size="small"
          :aria-label="$t('shell.k46')"
          @update:model-value="(v: string | number) => emit('update:activeTab', String(v))"
        />
        <div :style="filterDividerStyle" />
        <ApSegmentedControl
          :model-value="sortBy"
          :options="sortByOptions"
          size="small"
          :aria-label="$t('shell.k47')"
          @update:model-value="(v: string | number) => emit('update:sortBy', String(v))"
        />
        <div :style="filterDividerStyle" />
        <ApSegmentedControl
          :model-value="searchType"
          :options="searchTypeOptions"
          size="small"
          :aria-label="$t('shell.k48')"
          @update:model-value="(v: string | number) => onSearchTypeChange(String(v))"
        />
        <div :style="filterDividerStyle" />
        <ApSegmentedControl
          :model-value="searchMode"
          :options="searchModeOptions"
          size="small"
          :aria-label="$t('shell.k49')"
          @update:model-value="(v: string | number) => onSearchModeChange(String(v))"
        />
        <div :style="filterDividerStyle" />
        <div :style="spaceSelectWrapStyle">
          <ApSelect
            :model-value="selectedSpaceId"
            :options="spaceOptions"
            :placeholder="$t('shell.k42')"
            size="small"
            @update:model-value="onSpaceChange"
          />
        </div>
        <div v-if="tagOptions.length > 0" :style="filterDividerStyle" />
        <div v-if="tagOptions.length > 0" :style="spaceSelectWrapStyle">
          <ApSelect
            :model-value="selectedTag"
            :options="tagOptions"
            :placeholder="$t('shell.k43')"
            size="small"
            searchable
            @update:model-value="onTagChange"
          />
        </div>
        <div :style="filterDividerStyle" />
        <ApSegmentedControl
          :model-value="timeRange"
          :options="timeRangeOptions"
          size="small"
          :aria-label="$t('shell.k50')"
          @update:model-value="(v: string | number) => onTimeRangeChange(String(v))"
        />
      </template>
    </div>
  </div>
</template>

<script setup lang="ts">
import { useI18n } from 'vue-i18n';
import { computed, onMounted, onUnmounted, ref } from 'vue';
import { AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleTransition, AppleType, cv } from '@/utils/core/apple-tokens';
import { crudFilterBarStyle } from '@/styles/layout/page-styles';
import ApSearch from '@/components/apple/ApSearch.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import { PlaceholderRotator } from '@/utils/domain/search-enhanced';
import { useAIStore } from '@/stores/ai/ai';

const props = defineProps<{
  keyword: string;
  loading: boolean;
  searched: boolean;
  searchHistory: string[];
  mode: string;
  activeTab: string;
  sortBy: string;
  searchType: string;
  searchMode: string;
  selectedSpaceId: number | null;
  selectedTag: string;
  timeRange: string;
  spaceOptions: Array<{ label: string; value: number }>;
  tagOptions: Array<{ label: string; value: string }>;
  tabOptions: Array<{ label: string; value: string }>;
  hasResults: boolean;
}>();

const emit = defineEmits<{
  'update:keyword': [value: string];
  'update:mode': [value: string];
  'update:activeTab': [value: string];
  'update:sortBy': [value: string];
  'update:searchType': [value: string];
  'update:searchMode': [value: string];
  'update:selectedSpaceId': [value: number | null];
  'update:selectedTag': [value: string];
  'update:timeRange': [value: string];
  search: [value: string];
  'select-suggestion': [value: string];
  'clear-history': [];
  'space-change': [];
  'search-type-change': [];
  'search-mode-change': [];
  'tag-change': [];
  'time-range-change': [];
}>();
const { t } = useI18n();

const aiStore = useAIStore();

/**
 * 选项集。选中态以前是「实心蓝药丸 + 其余 ghost」，全站另一套是 `ApSegmentedControl`
 * （灰底 + 抬升白块，带 role=tab / aria-selected / 方向键）——同一个功能两种语言，
 * 而且药丸那套违反「禁止实心色块当当前项」。这里全部并过去，选项由数据给，不再逐颗写按钮。
 */
const modeOptions = [
  { label: t('shell.search'), value: 'search' },
  { label: t('shell.k51'), value: 'ai' },
];
const sortByOptions = [
  { label: t('shell.k52'), value: 'relevance' },
  { label: t('shell.k53'), value: 'time' },
];
const searchTypeOptions = [
  { label: t('shell.k54'), value: 'all' },
  { label: t('shell.k55'), value: 'title' },
  { label: t('shell.k56'), value: 'content' },
];
const searchModeOptions = [
  { label: t('shell.k57'), value: 'hybrid' },
  { label: t('shell.k54'), value: 'fulltext' },
  { label: t('shell.k58'), value: 'semantic' },
];
const timeRangeOptions = [
  { label: t('shell.all'), value: 'all' },
  { label: t('shell.k60'), value: 'today' },
  { label: t('shell.k61'), value: 'week' },
  { label: t('shell.k62'), value: 'month' },
];

const keywordModel = computed({
  get: () => props.keyword,
  set: (val: string) => emit('update:keyword', val),
});

const searchPlaceholder = ref(t('searchUi.ph1'));

let placeholderRotator: PlaceholderRotator | null = null;

onMounted(() => {
  placeholderRotator = new PlaceholderRotator();
  placeholderRotator.setCallback((text) => {
    // 轮换库给的是 i18n 键，展示时翻译（同时修掉切语言不跟随的问题）
    searchPlaceholder.value = t(text);
  });
  placeholderRotator.start(3500);
});

onUnmounted(() => {
  if (placeholderRotator) placeholderRotator.stop();
});

function onApSearch(value: string) {
  if (!value.trim()) {
    emit('update:keyword', value);
    emit('search', value);
    return;
  }
  emit('search', value);
}

function onSuggestionSelect(value: string) {
  emit('update:keyword', value);
  emit('select-suggestion', value);
}

function onHistoryClick(item: string) {
  emit('update:keyword', item);
  emit('search', item);
}

function onSearchTypeChange(type: string) {
  emit('update:searchType', type);
  emit('search-type-change');
}

function onSearchModeChange(searchMode: string) {
  emit('update:searchMode', searchMode);
  emit('search-mode-change');
}

function onSpaceChange(value: string | number | null) {
  const numVal = typeof value === 'string' ? Number(value) : value;
  emit('update:selectedSpaceId', isNaN(numVal as number) ? null : (numVal as number | null));
  emit('space-change');
}

function onTagChange(value: string | number | null) {
  const tagVal = value === null || value === '' ? '' : String(value);
  emit('update:selectedTag', tagVal);
  emit('tag-change');
}

function onTimeRangeChange(range: string) {
  emit('update:timeRange', range);
  emit('time-range-change');
}

const searchBarStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace.md,
  // 2026-10-26：原先不换行 —— ApSearch(width:100%) + spinner + 模式切换总宽超出容器，
  // 整页被撑出横向滚动（右侧「Search/AI」被截掉、过滤条看着也被切）。
  // 允许换行 + 搜索框给收缩许可，窄视口时模式切换落到第二行。
  flexWrap: 'wrap' as const,
  position: 'relative' as const,
};


const spinnerWrapStyle = {
  position: 'absolute' as const, right: '12px', top: '50%',
  transform: 'translateY(-50%)',
  color: cv('text-tertiary'), display: 'inline-flex',
};

const spinnerStyle = { animation: 'spin var(--apple-spinner-duration) linear infinite' };

const syntaxHintContainerStyle = {
  padding: `${AppleSpace.sm} 0`,
};

const syntaxHintStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace.xs,
  flexWrap: 'wrap' as const,
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  background: cv('bg-secondary'),
  borderRadius: AppleRadius.small,
  border: `1px solid ${cv('border-light')}`,
};

const syntaxHintTitleStyle = {
  fontFamily: AppleFont.text, fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleFontWeight.semibold), color: cv('text-secondary'),
};

const syntaxHintItemStyle = {
  fontFamily: AppleFont.text, fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
  padding: `1px ${AppleSpace.xs}`,
  background: cv('surface-field'),
  borderRadius: AppleRadius.micro,
  border: `1px solid ${cv('border-light')}`,
};

const syntaxHintSepStyle = {
  color: cv('text-quaternary'), fontSize: AppleType.legalMeta.size,
};

const historyRowStyle = {
  display: 'flex', alignItems: 'center', gap: AppleSpace.sm,
  flexWrap: 'wrap' as const,
};

const historyLabelStyle = {
  fontFamily: AppleFont.text, fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'), flexShrink: '0' as const,
};

const historyTagStyle = {
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  background: cv('bg-secondary'), borderRadius: AppleRadius.small,
  fontFamily: AppleFont.text, fontSize: AppleType.microUI.size,
  color: cv('text-secondary'), cursor: 'pointer',
  border: `1px solid ${cv('border-light')}`,
  transition: AppleTransition.fast,
};


const filterDividerStyle = {
  width: '1px', height: '24px', background: cv('border-light'), flexShrink: '0' as const,
};

const spaceSelectWrapStyle = {
  width: '160px',
  flexShrink: '0' as const,
};
</script>