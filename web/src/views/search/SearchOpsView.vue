<template>
  <ApPageContainer
    :title="$t('search.k57')"
    :subtitle="$t('search.k59')"
    :max-width="AppleLayout.maxContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <!-- 同义词词库 -->
    <div :style="sectionStyle">
      <ApCard size="small">
        <template #header>
          <div :style="cardHeaderStyle">
            <span :style="sectionTitleStyle">{{ $t('search.k42') }}</span>
            <ApButton variant="ghost" size="small" :loading="dictionaryLoading" @click="loadDictionary">
              <ApIcon name="RefreshCw" :size="16" /> {{ $t('search.refresh') }}
            </ApButton>
          </div>
        </template>
        <ApTable :columns="groupColumns" :data="dictionaryRows" :loading="dictionaryLoading">
          <template #empty>
            <ApEmpty
              icon="Settings"
              :title="$t('search.k58')"
              :description="$t('vSearchUi.k3')"
            />
          </template>
        </ApTable>
        <p :style="{ ...descStyle, marginTop: AppleSpace.sm }">
          {{ $t('search.k43') }}
        </p>
      </ApCard>
    </div>

    <!-- 扩展查询试跑 -->
    <div :style="sectionStyle">
      <ApCard size="small">
        <template #header>
          <span :style="sectionTitleStyle">{{ $t('search.k44') }}</span>
        </template>
        <div :style="searchBarStyle">
          <ApInput
            v-model="expandQuery"
            :aria-label="$t('search.k60')"
            :placeholder="$t('search.k55')"
            :style="{ flex: '1', minWidth: 0 }"
            @keyup.enter="handleExpand"
          />
          <ApButton variant="primary" size="small" :loading="expanding" @click="handleExpand">
            <ApIcon name="Search" :size="16" /> {{ $t('search.k45') }}
          </ApButton>
        </div>
        <div v-if="expanded.length" :style="listStyle(AppleSpace.sm)">
          <div v-for="(term, idx) in expanded" :key="term" :style="expandedRowStyle()">
            <span :style="totalStyle">{{ idx + 1 }}</span>
            <span :style="tagStyle(idx === 0 ? cv('blue') : cv('gray'))">{{ term }}</span>
            <span v-if="idx === 0" :style="descStyle">{{ $t('search.k46') }}</span>
          </div>
        </div>
        <p v-else-if="expandDone" :style="descStyle">{{ $t('search.k47') }}</p>
      </ApCard>
    </div>

    <!-- 索引运维 -->
    <div :style="sectionStyle">
      <ApCard size="small">
        <template #header>
          <span :style="sectionTitleStyle">{{ $t('search.k48') }}</span>
        </template>

        <div :style="opsRowStyle()">
          <div :style="cardBodyStyle">
            <div :style="cardTitleStyle">{{ $t('search.k49') }}</div>
            <p :style="descStyle">
              {{ $t('search.k50') }}
            </p>
          </div>
          <ApButton variant="secondary" size="small" :loading="initializing" @click="handleInit">
            <ApIcon name="Stethoscope" :size="16" /> {{ $t('search.k51') }}
          </ApButton>
        </div>

        <template v-if="diagnostic">
          <div :style="dividerStyle" />
          <ApDescriptionList :items="diagnosticItems" />
          <pre :style="resultPreStyle()">{{ JSON.stringify(diagnostic.index_stats, null, 2) }}</pre>
        </template>

        <div :style="dividerStyle" />

        <div :style="opsRowStyle()">
          <div :style="cardBodyStyle">
            <div :style="cardTitleStyle">{{ $t('search.k52') }}</div>
            <p :style="descStyle">
              {{ $t('search.k53') }}
            </p>
          </div>
          <div :style="opsActionRowStyle()">
            <ApInput v-model="indexDocId" :aria-label="$t('search.k61')" :placeholder="$t('search.k56')" :style="{ flex: '1', minWidth: 0 }" />
            <ApButton variant="primary" size="small" :loading="indexing" @click="handleIndexDocument">
              <ApIcon name="FileSymlink" :size="16" /> {{ $t('search.k54') }}
            </ApButton>
          </div>
        </div>
      </ApCard>
    </div>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { computed, onMounted, ref, shallowRef } from 'vue';
import { logger } from '@/utils/logger';
import { useUIStore } from '@/stores/app/ui';
import { AppleFont, AppleLayout, AppleRadius, AppleSpace, AppleType, cv } from '@/utils/core/apple-tokens';
import {
  cardBodyStyle,
  cardHeaderStyle,
  listStyle,
  cardTitleStyle,
  descStyle,
  dividerStyle,
  searchBarStyle,
  sectionStyle,
  sectionTitleStyle,
  tagStyle,
  totalStyle,
} from '@/styles/layout/page-styles';
import {
  expandQueryWithSynonyms,
  getSynonyms,
  indexSingleDocument,
  initIndexDiagnostic,
  type IndexInitDiagnostic,
} from '@/api/modules/document/search';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import type { DescriptionItem } from '@/components/apple/ApDescriptionList.vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApCard from '@/components/apple/ApCard.vue';
import ApTable from '@/components/apple/ApTable.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApDescriptionList from '@/components/apple/ApDescriptionList.vue';

interface SynonymGroupRow {
  id: number;
  words: string;
  size: number;
}

const ui = useUIStore();

const dictionaryRows = ref<SynonymGroupRow[]>([]);
const dictionaryLoading = shallowRef(false);

const expandQuery = ref('');
const expanded = shallowRef<string[]>([]);
const expanding = shallowRef(false);
const expandDone = shallowRef(false);

const diagnostic = shallowRef<IndexInitDiagnostic | null>(null);
const initializing = shallowRef(false);

const indexDocId = ref('');
const indexing = shallowRef(false);

const groupColumns: TableColumn<SynonymGroupRow>[] = [
  { key: 'id', title: t('search.k62'), width: '12%', align: 'right' },
  { key: 'size', title: t('search.k63'), width: '12%', align: 'right' },
  { key: 'words', title: t('search.k64'), ellipsis: true },
];

const diagnosticItems = computed<DescriptionItem[]>(() => {
  const d = diagnostic.value;
  if (!d) return [];
  return [
    { label: t('search.k65'), value: d.index_path || '--' },
    { label: t('search.k66'), value: d.parent_dir_exists ? t('uiTextMain.k36') : t('uiTextMain.k37') },
    { label: t('search.k67'), value: d.index_dir_exists ? t('uiTextMain.k36') : t('uiTextMain.k37') },
    { label: t('search.k68'), value: d.init_result },
    { label: t('search.k69'), value: d.index_stats?.is_open ? t('uiTextMain.k36') : t('uiTextMain.k37') },
    { label: t('search.k70'), value: String(d.index_stats?.doc_count ?? 0) },
  ];
});

function expandedRowStyle() {
  return {
    display: 'flex',
    alignItems: 'center',
    gap: AppleSpace.sm,
  };
}

function opsRowStyle() {
  return {
    display: 'flex',
    alignItems: 'center',
    justifyContent: 'space-between',
    gap: AppleSpace.md,
    flexWrap: 'wrap' as const,
  };
}

function opsActionRowStyle() {
  return {
    display: 'flex',
    alignItems: 'center',
    gap: AppleSpace.sm,
    flex: '1',
    minWidth: 0,
  };
}

function resultPreStyle() {
  return {
    marginTop: AppleSpace.sm,
    padding: AppleSpace.sm,
    background: cv('bg-tertiary'),
    borderRadius: AppleRadius.control,
    fontFamily: AppleFont.mono,
    fontSize: AppleType.microUI.size,
    color: cv('text-primary'),
    whiteSpace: 'pre-wrap' as const,
    overflowX: 'auto' as const,
    maxHeight: AppleLayout.tocWidth,
  };
}

async function loadDictionary() {
  dictionaryLoading.value = true;
  try {
    const res = await getSynonyms();
    dictionaryRows.value = (res.synonyms || []).map((group, idx) => {
      const words = group.Words || group.words || [];
      return { id: idx + 1, words: words.join(' / '), size: words.length };
    });
  } catch (error) {
    logger.error('[SearchOps] load synonyms failed:', error);
    dictionaryRows.value = [];
  } finally {
    dictionaryLoading.value = false;
  }
}

async function handleExpand() {
  const query = expandQuery.value.trim();
  if (!query) return;
  expanding.value = true;
  expandDone.value = false;
  try {
    const res = await expandQueryWithSynonyms(query);
    expanded.value = res.expanded || [];
    expandDone.value = true;
  } catch (error) {
    logger.error('[SearchOps] expand query failed:', error);
    ui.showToast(t('search.k71'), 'error');
  } finally {
    expanding.value = false;
  }
}

async function handleInit() {
  initializing.value = true;
  try {
    diagnostic.value = await initIndexDiagnostic();
    const result = diagnostic.value?.init_result || '';
    if (result.startsWith('init_failed')) ui.showToast(t('search.k72') + result, 'error');
    else if (result === 'init_succeeded') ui.showToast(t('search.k73'), 'success');
  } catch (error) {
    logger.error('[SearchOps] init diagnostic failed:', error);
    diagnostic.value = null;
    ui.showToast(t('search.k74'), 'error');
  } finally {
    initializing.value = false;
  }
}

async function handleIndexDocument() {
  const id = Number(indexDocId.value);
  if (!id || Number.isNaN(id)) {
    ui.showToast(t('search.k75'), 'warning');
    return;
  }
  indexing.value = true;
  try {
    await indexSingleDocument(id);
    ui.showToast(t('sentUi.k50', { p1: id }), 'success');
  } catch (error) {
    logger.error('[SearchOps] index document failed:', error);
    ui.showToast(t('search.k76'), 'error');
  } finally {
    indexing.value = false;
  }
}

onMounted(loadDictionary);
</script>
