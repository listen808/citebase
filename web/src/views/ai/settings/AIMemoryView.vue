<template>
  <div :style="aiSubPageStyle">
    <div :style="statsGridStyle">
      <ApStatCard :label="$t('aiSet.k243')" :value="stats?.total ?? 0" icon="Brain" :color="cv('blue')" size="compact" />
      <ApStatCard :label="$t('aiSet.k244')" :value="stats?.preferences ?? 0" icon="SlidersHorizontal" :color="cv('purple')" size="compact" />
      <ApStatCard :label="$t('aiSet.k245')" :value="stats?.knowledge ?? 0" icon="BookOpen" :color="cv('green')" size="compact" />
      <ApStatCard :label="$t('aiSet.k246')" :value="stats?.sessions ?? 0" icon="MessageSquare" :color="cv('orange')" size="compact" />
      <ApStatCard :label="$t('aiSet.k247')" :value="stats?.entities ?? 0" icon="Database" :color="cv('teal')" size="compact" />
      <ApStatCard :label="$t('aiSet.k248')" :value="stats?.user_feedback ?? 0" icon="ThumbsDown" :color="cv('red')" size="compact" />
    </div>

    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <div :style="aiSubCardNameRowStyle">
            <ApIcon name="Brain" :size="AppleIconSize.stat" :color="cv('blue')" />
            <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k236') }}</span>
            <span :style="aiSubCountBadgeStyle">{{ displayedTotal }}</span>
          </div>
          <ApButton variant="primary" size="small" @click="openCreate">
            <ApIcon name="Plus" :size="AppleIconSize.control" /> {{ $t('aiSet.k237') }}
          </ApButton>
        </div>
      </template>

      <!-- 类型切换与语义检索以前各占一行；并成一行：左边「看哪些」，右边「搜什么」 -->
      <div :style="filterBarStyle">
        <ApSegmentedControl
          v-model="typeFilter"
          :options="typeOptions"
          size="small"
          :aria-label="$t('aiSet.k256')"
          @update:model-value="handleTypeChange"
        />
        <ApButton variant="secondary" size="small" :loading="loading" @click="refresh">
          <ApIcon name="RotateCw" :size="AppleIconSize.control" /> {{ $t('aiSet.refresh') }}
        </ApButton>
        <ApSearch
          v-model="keyword"
          :aria-label="$t('aiSet.k257')"
          :placeholder="$t('aiSet.k251')"
          :style="{ flex: '1' }"
          @update:model-value="onKeywordInput"
        />
        <ApButton variant="primary" size="small" :loading="searching" :disabled="!keyword.trim()" @click="runSearch">
          <ApIcon name="Search" :size="AppleIconSize.control" /> {{ $t('aiSet.k238') }}
        </ApButton>
        <ApButton v-if="mode === 'search'" variant="ghost" size="small" @click="exitSearch">{{ $t('aiSet.k239') }}</ApButton>
      </div>

      <!-- 裸 <p> 自带浏览器默认 10px 上下边距：看着"差不多"，其实是没落尺的凑数，
           显式写成阶梯上的 8px 后与相邻行的间距才由设计系统负责。 -->
      <p :style="{ ...aiSubSectionMetaStyle, margin: `${AppleSpace.sm} 0` }">{{ modeHint }}</p>

      <ApTable
        density="compact"
        :columns="columns"
        :data="rows"
        :loading="loading || searching"
        :empty-text="mode === 'search' ? '没有语义检索命中的记忆' : '还没有任何记忆'"
        empty-icon="Brain"
        :aria-label="$t('aiSet.k258')"
      >
        <template #actions="{ row }">
          <ApButton
            v-if="(row as unknown as AIMemory).status === 'pending'"
            variant="primary"
            size="small"
            :loading="confirmingId === (row as unknown as AIMemory).id"
            @click="confirmRow(row as unknown as AIMemory)"
          >
            <ApIcon name="Check" :size="AppleIconSize.control" /> {{ $t('aiSet.confirmMemory') }}
          </ApButton>
          <ApButton variant="danger" size="small" @click="askDelete(row as unknown as AIMemory)">
            <ApIcon name="Trash2" :size="AppleIconSize.control" /> {{ $t('aiSet.delete') }}
          </ApButton>
        </template>
      </ApTable>

      <ApPagination
        v-if="mode === 'list' && displayedTotal > pageSize"
        :page="page"
        :page-size="pageSize"
        :total="displayedTotal"
        @change="handlePageChange"
      />
    </ApCard>

    <ApModal v-model="showCreate" :title="$t('aiSet.k237')" size="medium">
      <div :style="formStyle">
        <ApTextarea
          v-model="form.content"
          :label="$t('aiSet.k249')"
          :placeholder="$t('aiSet.k252')"
          :rows="3"
          required
        />
        <ApInput v-model="form.summary" :label="$t('aiSet.k250')" :placeholder="$t('aiSet.k253')" />
        <div :style="formRowStyle">
          <div :style="formFieldStyle">
            <div :style="aiSubFilterLabelStyle">{{ $t('aiSet.k240') }}</div>
            <ApSelect v-model="form.memory_type" :options="createTypeOptions" />
          </div>
          <div :style="formFieldStyle">
            <div :style="aiSubFilterLabelStyle">{{ $t('aiSet.k171') }}</div>
            <ApInput v-model="form.tags" :placeholder="$t('aiSet.k254')" />
          </div>
        </div>
        <div :style="importanceRowStyle">
          <span :style="aiSubFilterLabelStyle">{{ $t('aiSet.k241') }}</span>
          <ApSlider v-model="form.importance" :min="0" :max="1" :step="0.05" :aria-label="$t('aiSet.k259')" />
          <span :style="importanceValueStyle">{{ form.importance.toFixed(2) }}</span>
        </div>
        <p :style="aiSubSectionMetaStyle">
          {{ $t('aiSet.k242') }}
        </p>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showCreate = false">{{ $t('aiSet.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="saving" @click="handleCreate">{{ $t('aiSet.save') }}</ApButton>
      </template>
    </ApModal>

    <ApConfirmModal
      v-model="showDelete"
      :title="$t('aiSet.k255')"
      :message="$t('aiSet.k296', { type: deletingItem ? (TYPE_LABELS[deletingItem.memory_type] ?? $t('fallback.k99')) : '' })"
      type="danger"
      :confirm-text="$t('aiSet.delete')"
      @confirm="confirmDelete"
    />
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { computed, onMounted, ref } from 'vue';

import { logger } from '@/utils/logger';
import { AppleSpace, cv, AppleIconSize } from '@/utils/core/apple-tokens';
import ApIcon from '@/components/apple/ApIcon.vue';
import { useBreakpoint } from '@/utils/responsive';
import { formatTime, formatTimeAgo } from '@/utils/format/format-time';
import {
  AI_TABLE_WIDTH,
  aiSubCardNameRowStyle,
  aiSubCountBadgeStyle,
  aiSubFilterLabelStyle,
  aiSubPageStyle,
  aiSubSectionHeaderStyle,
  aiSubSectionMetaStyle,
  aiSubSectionTitleStyle,
  crudStatsRowFitStyle,
  filterBarStyle,
  formRowStyle,
  formStyle,
} from '@/styles/ai/ai-page-styles';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import ApCard from '@/components/apple/ApCard.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApPagination from '@/components/apple/ApPagination.vue';
import ApSearch from '@/components/apple/ApSearch.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApSlider from '@/components/apple/ApSlider.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import ApTable from '@/components/apple/ApTable.vue';
import ApTextarea from '@/components/apple/ApTextarea.vue';
import ApButton from '@/components/apple/ApButton.vue';
import {
  createMemory,
  deleteMemory,
  confirmMemory,
  getMemoryStats,
  listMemories,
  searchMemories,
  type AIMemory,
  type MemoryStats,
  type MemoryType,
} from '@/api/modules/ai/ai-memory';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();
const { isMobile } = useBreakpoint();

const TYPE_LABELS: Record<string, string> = {
  preference: '用户偏好',
  knowledge: '知识记忆',
  session: '会话摘要',
  entity: '实体记忆',
  user_feedback: '反馈样本',
};

const TIER_LABELS: Record<string, string> = {
  short: '短期',
  long: '长期',
  stale: '待清理',
};

// 语义检索走 /ai/memory/search，分页列表走 /ai/memory/{?page}。
// 两者口径不同，必须让用户看清当前用的是哪一个，不能把检索结果伪装成分页列表。
const mode = ref<'list' | 'search'>('list');

const stats = ref<MemoryStats | null>(null);
const rows = ref<AIMemory[]>([]);
const total = ref(0);
const searchTotal = ref(0);
const page = ref(1);
const pageSize = 20;
const loading = ref(false);
const searching = ref(false);
const saving = ref(false);
const confirmingId = ref<number | null>(null);

const typeFilter = ref<string>('all');
const keyword = ref('');

const showCreate = ref(false);
const showDelete = ref(false);
const deletingItem = ref<AIMemory | null>(null);

const form = ref({
  content: '',
  summary: '',
  memory_type: 'knowledge' as MemoryType,
  tags: '',
  importance: 0.5,
});

const typeOptions = [
  { label: t('aiSet.all'), value: 'all' },
  { label: t('aiSet.k261'), value: 'preference' },
  { label: t('aiSet.k262'), value: 'knowledge' },
  { label: t('aiSet.k263'), value: 'session' },
  { label: t('aiSet.k264'), value: 'entity' },
  { label: t('aiSet.k265'), value: 'user_feedback' },
];

const createTypeOptions = typeOptions
  .filter((o) => o.value !== 'all')
  .map((o) => ({ label: o.label, value: o.value }));

const displayedTotal = computed(() => (mode.value === 'search' ? searchTotal.value : total.value));

const modeHint = computed(() => {
  if (mode.value === 'search') {
    return `语义检索模式：命中 ${searchTotal.value} 条，不分页，按相似度返回`;
  }
  if (typeFilter.value === 'all') {
    return `按创建时间分页，共 ${total.value} 条`;
  }
  return `按创建时间分页，当前类型「${TYPE_LABELS[typeFilter.value]}」共 ${total.value} 条`;
});

function displayText(row: AIMemory) {
  return row.summary?.trim() ? row.summary : row.content;
}

const columns: TableColumn<AIMemory>[] = [
  { key: 'content', title: t('aiSet.k266'), ellipsis: true, render: (row) => displayText(row) },
  { key: 'memory_type', title: t('aiSet.type'), width: AI_TABLE_WIDTH.md, render: (row) => TYPE_LABELS[row.memory_type] ?? row.memory_type },
  { key: 'memory_tier', title: t('aiSet.k268'), width: AI_TABLE_WIDTH.sm, render: (row) => TIER_LABELS[row.memory_tier ?? 'short'] ?? '—' },
  { key: 'status', title: t('aiSet.memStatusCol'), width: AI_TABLE_WIDTH.sm, render: (row) => (row.status === 'pending' ? t('aiSet.pendingMemory') : t('aiSet.confirmedMemory')) },
  { key: 'importance', title: t('aiSet.k241'), width: AI_TABLE_WIDTH.sm, align: 'right', render: (row) => row.importance.toFixed(2) },
  { key: 'access_count', title: t('aiSet.k269'), width: AI_TABLE_WIDTH.xs, align: 'right', render: (row) => String(row.access_count) },
  { key: 'last_accessed_at', title: t('aiSet.k270'), width: AI_TABLE_WIDTH.lg, render: (row) => (row.last_accessed_at ? formatTimeAgo(row.last_accessed_at) : '从未') },
  { key: 'created_at', title: t('aiSet.k202'), width: AI_TABLE_WIDTH.xl, render: (row) => (row.created_at ? formatTime(row.created_at) : '—') },
];

async function loadStats() {
  try {
    const res = await getMemoryStats();
    stats.value = res?.stats ?? null;
  } catch (e) {
    logger.error('Failed to load memory stats:', e);
    ui.showToast(t('aiSet.k271'), 'error');
  }
}

async function loadList() {
  loading.value = true;
  try {
    const res = await listMemories({
      type: typeFilter.value === 'all' ? undefined : typeFilter.value,
      page: page.value,
      page_size: pageSize,
    });
    rows.value = res?.memories ?? [];
    total.value = res?.total ?? 0;
  } catch (e) {
    logger.error('Failed to load memories:', e);
    ui.showToast(t('aiSet.k272'), 'error');
  } finally {
    loading.value = false;
  }
}

async function runSearch() {
  const q = keyword.value.trim();
  if (!q) return;
  searching.value = true;
  try {
    const res = await searchMemories({
      query: q,
      memory_type: typeFilter.value === 'all' ? undefined : typeFilter.value,
      limit: 50,
    });
    rows.value = res?.memories ?? [];
    searchTotal.value = res?.total ?? rows.value.length;
    mode.value = 'search';
  } catch (e) {
    logger.error('Failed to search memories:', e);
    ui.showToast(t('aiSet.k273'), 'error');
  } finally {
    searching.value = false;
  }
}

function exitSearch() {
  mode.value = 'list';
  keyword.value = '';
  searchTotal.value = 0;
  void loadList();
}

function onKeywordInput(value: string) {
  // 清空搜索框就该回到列表，否则界面还停在检索模式而用户以为在看全量
  if (!value.trim() && mode.value === 'search') {
    exitSearch();
  }
}

function handleTypeChange() {
  page.value = 1;
  if (mode.value === 'search' && keyword.value.trim()) {
    void runSearch();
  } else {
    void loadList();
  }
}

function handlePageChange(next: number) {
  page.value = next;
  void loadList();
}

async function refresh() {
  if (mode.value === 'search' && keyword.value.trim()) {
    await runSearch();
  } else {
    await loadList();
  }
  await loadStats();
}

function openCreate() {
  form.value = { content: '', summary: '', memory_type: 'knowledge', tags: '', importance: 0.5 };
  showCreate.value = true;
}

async function handleCreate() {
  const content = form.value.content.trim();
  if (!content) {
    ui.showToast(t('aiSet.k274'), 'warning');
    return;
  }
  saving.value = true;
  try {
    await createMemory({
      content,
      summary: form.value.summary.trim(),
      memory_type: form.value.memory_type,
      importance: form.value.importance,
      tags: form.value.tags
        .split(',')
        .map((t) => t.trim())
        .filter(Boolean),
    });
    ui.showToast(t('aiSet.k275'), 'success');
    showCreate.value = false;
    await refresh();
  } catch (e) {
    logger.error('Failed to create memory:', e);
    ui.showToast(t('aiSet.k276'), 'error');
  } finally {
    saving.value = false;
  }
}

async function confirmRow(item: AIMemory) {
  if (confirmingId.value !== null) return;
  confirmingId.value = item.id;
  try {
    await confirmMemory(item.id);
    ui.showToast(t('aiSet.memConfirmed'), 'success');
    await refresh();
  } catch (e) {
    logger.error('Failed to confirm memory:', e);
    ui.showToast(t('aiSet.memConfirmFailed'), 'error');
  } finally {
    confirmingId.value = null;
  }
}

function askDelete(item: AIMemory) {
  deletingItem.value = item;
  showDelete.value = true;
}

async function confirmDelete() {
  const target = deletingItem.value;
  if (!target?.id) return;
  try {
    await deleteMemory(target.id);
    ui.showToast(t('aiSet.k116'), 'success');
    showDelete.value = false;
    deletingItem.value = null;
    await refresh();
  } catch (e) {
    logger.error('Failed to delete memory:', e);
    ui.showToast(t('aiSet.k32'), 'error');
  }
}

onMounted(() => {
  void loadStats();
  void loadList();
});

// 6 张统计卡：auto-fit 会排出 4+2 的孤儿行（审查 §四.4）—— 桌面固定 3 列两行各半
const statsGridStyle = computed(() => crudStatsRowFitStyle(6, isMobile.value, 'compact'));

const formFieldStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
  flex: '1',
};

const importanceRowStyle = {
  display: 'flex',
  alignItems: 'center' as const,
  gap: AppleSpace.md,
};

const importanceValueStyle = {
  minWidth: '3.5em',
  textAlign: 'right' as const,
};
</script>
