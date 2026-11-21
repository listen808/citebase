<template>
  <div :style="aiSubPageStyle">
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <div :style="aiSubCardNameRowStyle">
            <ApIcon name="History" :size="18" :color="cv('blue')" />
            <span :style="aiSubSectionTitleStyle">{{ $t('aiSet2.k108') }}</span>
            <span :style="aiSubCountBadgeStyle">{{ total }}</span>
          </div>
          <span :style="aiSubSectionMetaStyle">{{ $t('aiSet2.k109') }}</span>
        </div>
      </template>

      <div :style="toolbarStyle">
        <div :style="spaceSelectorStyle">
          <span :style="labelStyle">{{ $t('aiSet2.k87') }}</span>
          <ApSelect
            v-model="selectedSpaceId"
            :options="spaceOptions"
            :placeholder="$t('aiSet2.k123')"
            :searchable="true"
            @update:model-value="onFilterChange"
          />
        </div>
        <ApSegmentedControl v-model="scope" :options="scopeOptions" size="small" @update:model-value="onFilterChange" />
        <ApButton variant="secondary" size="small" :loading="loading" @click="loadItems">{{ $t('aiSet2.refresh') }}</ApButton>
      </div>

      <ApTable :columns="columns" :data="items" :loading="loading" density="compact">
        <template #empty>
          <ApEmpty icon="History" :title="$t('aiSet2.k124')" :description="$t('vAiUi.k226')" />
        </template>
        <template #actions="{ row }">
          <ApFavoriteButton
            labeled
            size="small"
            :active="(row as unknown as SQLHistory).is_favorite"
            :busy="favoriteId === (row as unknown as SQLHistory).id"
            noun="SQL"
            @toggle="handleToggleFavorite(row as unknown as SQLHistory)"
          />
          <ApButton variant="ghost" size="small" @click="openDetail(row as unknown as SQLHistory)">
            <ApIcon name="Eye" :size="16" /> {{ $t('aiSet2.detail') }}
          </ApButton>
          <ApButton variant="danger" size="small" @click="handleDelete(row as unknown as SQLHistory)">
            <ApIcon name="Trash2" :size="16" /> {{ $t('aiSet2.delete') }}
          </ApButton>
        </template>
      </ApTable>

      <ApPagination
        v-if="total > PAGE_SIZE"
        :total="total"
        :page="page"
        :page-size="PAGE_SIZE"
        @change="changePage"
      />
    </ApCard>

    <ApModal v-model="showDetailModal" :title="$t('aiSet2.k125')" size="large">
      <div v-if="detail" :style="aiSubSectionStyle">
        <div :style="aiSubFormGridStyle">
          <div>
            <span :style="detailLabelStyle">{{ $t('aiSet2.status') }}</span>
            <ApStatusBadge :status="statusBadge(detail.status)" :label="statusText(detail.status)" size="small" />
          </div>
          <div>
            <span :style="detailLabelStyle">{{ $t('aiSet2.k111') }}</span>
            <span :style="aiSubBadgeStyle">{{ detail.generator_tier || '-' }}</span>
          </div>
          <div>
            <span :style="detailLabelStyle">{{ $t('aiSet2.k60') }}</span>
            <span :style="aiSubSectionMetaStyle">{{ detail.duration_ms }} ms</span>
          </div>
          <div>
            <span :style="detailLabelStyle">{{ $t('aiSet2.k112') }}</span>
            <span :style="aiSubSectionMetaStyle">{{ detail.execution_time_ms }} ms</span>
          </div>
          <div>
            <span :style="detailLabelStyle">{{ $t('aiSet2.k113') }}</span>
            <span :style="aiSubSectionMetaStyle">{{ detail.tables_used || '-' }}</span>
          </div>
          <div>
            <span :style="detailLabelStyle">{{ $t('aiSet2.k82') }}</span>
            <span :style="aiSubSectionMetaStyle">{{ formatTime(detail.created_at) }}</span>
          </div>
        </div>

        <div :style="aiSubSectionStyle">
          <span :style="detailLabelStyle">{{ $t('aiSet2.k92') }}</span>
          <p :style="descStyle">{{ detail.query_text }}</p>
        </div>

        <div v-if="detail.generated_sql" :style="aiSubSectionStyle">
          <span :style="detailLabelStyle">{{ $t('aiSet2.k114') }}</span>
          <pre :style="aiSubBodyStyle">{{ detail.generated_sql }}</pre>
        </div>

        <div v-if="detail.executed_sql" :style="aiSubSectionStyle">
          <span :style="detailLabelStyle">{{ $t('aiSet2.k115') }}</span>
          <pre :style="aiSubBodyStyle">{{ detail.executed_sql }}</pre>
        </div>

        <div v-if="detail.execution_error" :style="aiSubSectionStyle">
          <span :style="detailLabelStyle">{{ $t('aiSet2.k116') }}</span>
          <pre :style="aiSubBodyStyle">{{ detail.execution_error }}</pre>
        </div>

        <div v-if="detail.execution_result" :style="aiSubSectionStyle">
          <span :style="detailLabelStyle">{{ $t('aiSet2.k117') }}</span>
          <pre :style="aiSubBodyStyle">{{ detail.execution_result }}</pre>
        </div>

        <div v-if="detail.feedback" :style="aiSubSectionStyle">
          <span :style="detailLabelStyle">{{ $t('aiSet2.k118') }}</span>
          <p :style="descStyle">{{ detail.feedback }}</p>
        </div>

        <div :style="aiSubSectionStyle">
          <span :style="detailLabelStyle">{{ $t('aiSet2.k119') }}</span>
          <div :style="aiSubActionRowStyle">
            <ApButton
              v-for="n in RATING_STEPS"
              :key="n"
              variant="ghost"
              size="small"
              :loading="ratingBusy === n"
              @click="handleRate(n)"
            >
              <ApIcon name="Star" :size="16" :color="n <= detail.rating ? cv('warning') : cv('text-tertiary')" />
              {{ n }}
            </ApButton>
            <span :style="aiSubSectionMetaStyle">{{ detail.rating > 0 ? `${detail.rating} / 5` : '未评分' }}</span>
          </div>
        </div>

        <div v-if="detail.trace_id" :style="aiSubSectionStyle">
          <div :style="aiSubCardHeaderStyle">
            <span :style="detailLabelStyle">{{ $t('aiSet2.k120') }}</span>
            <ApButton variant="secondary" size="small" :loading="traceLoading" @click="loadTrace">
              <ApIcon name="Activity" :size="16" /> {{ trace ? $t('vAiUi.k227') : $t('vAiUi.k228') }}
            </ApButton>
          </div>
          <div v-if="!trace" :style="aiSubSectionMetaStyle">
            trace_id {{ detail.trace_id }}
          </div>
          <template v-if="trace">
            <div :style="metaStyle">
              <span :style="aiSubBadgeStyle">意图 {{ trace.intent || '-' }}</span>
              <span :style="aiSubBadgeStyle">{{ trace.steps.length }} {{ $t('vAiUi.k229') }}</span>
              <span :style="aiSubBadgeStyle">{{ trace.duration_ms }} ms</span>
              <ApStatusBadge
                :status="trace.execution_ok ? 'enabled' : 'error'"
                :label="trace.execution_ok ? '执行成功' : '执行失败'"
                size="small"
              />
            </div>
            <div v-for="(step, index) in trace.steps" :key="index" :style="aiSubSectionStyle">
              <div :style="aiSubCardHeaderStyle">
                <span :style="aiSubCardNameStyle">{{ index + 1 }} · {{ step.name }}</span>
                <span :style="aiSubSectionMetaStyle">{{ step.duration_ms }} ms · {{ step.status }}</span>
              </div>
              <pre v-if="step.output" :style="aiSubBodyStyle">{{ step.output }}</pre>
              <pre v-if="step.error_msg" :style="aiSubBodyStyle">{{ step.error_msg }}</pre>
            </div>
            <div v-if="trace.final_sql" :style="aiSubSectionStyle">
              <span :style="detailLabelStyle">{{ $t('aiSet2.k121') }}</span>
              <pre :style="aiSubBodyStyle">{{ trace.final_sql }}</pre>
            </div>
          </template>
        </div>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showDetailModal = false">{{ $t('aiSet2.close') }}</ApButton>
      </template>
    </ApModal>

    <ApConfirmModal
      v-model="showDeleteModal"
      :title="$t('aiSet2.k126')"
      :message="`确定删除「${deletingItem?.query_text ?? ''}」这条历史记录吗？此操作不可撤销。`"
      type="danger"
      :confirm-text="$t('aiSet2.delete')"
      :loading="deleting"
      @confirm="confirmDelete"
    />
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, onMounted } from 'vue';
import ApTable from '@/components/apple/ApTable.vue';
import ApFavoriteButton from '@/components/apple/ApFavoriteButton.vue';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import ApStatusBadge from '@/components/apple/ApStatusBadge.vue';

import { logger } from '@/utils/logger';
import { cv } from '@/utils/core/apple-tokens';
import { formatTime } from '@/utils/format/format-time';
import {
  aiSubPageStyle,
  aiSubSectionStyle,
  aiSubSectionHeaderStyle,
  aiSubSectionTitleStyle,
  aiSubSectionMetaStyle,
  aiSubCardNameRowStyle,
  aiSubCardNameStyle,
  aiSubCardHeaderStyle,
  aiSubBadgeStyle,
  aiSubCountBadgeStyle,
  aiSubFormGridStyle,
  aiSubActionRowStyle,
  aiSubToolbarBetweenStyle as toolbarStyle,
  aiSubFilterStyle as spaceSelectorStyle,
  aiSubFilterLabelStyle as labelStyle,
  aiSubBodyStyle,
  aiSubLabelStyle as detailLabelStyle,
  metaStyle,
  descStyle,
  AI_TABLE_WIDTH,
} from '@/styles/ai/ai-page-styles';
import { SQLHistoryAPI, type SQLHistory } from '@/api/modules/schema-sql/sql-history';
import { SQLTraceAPI, type SQLTrace } from '@/api/modules/ai/ai-sql-audit';
import { useSpaceStore } from '@/stores/workspace/space';
import type { Space } from '@/types/models';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();
const spaceStore = useSpaceStore();

const RATING_STEPS = [1, 2, 3, 4, 5];
const PAGE_SIZE = 20;

const spaces = ref<Space[]>([]);
const selectedSpaceId = ref<number>(0);
const scope = ref<'all' | 'favorites'>('all');
const items = ref<SQLHistory[]>([]);
const total = ref(0);
const page = ref(1);
const loading = shallowRef(false);

const favoriteId = ref<number | null>(null);
const deleting = ref(false);
const showDeleteModal = ref(false);
const deletingItem = ref<SQLHistory | null>(null);

const showDetailModal = ref(false);
const detail = shallowRef<SQLHistory | null>(null);
const ratingBusy = ref<number | null>(null);
const trace = shallowRef<SQLTrace | null>(null);
const traceLoading = shallowRef(false);

const spaceOptions = computed(() => [
  { label: t('aiSet2.k123'), value: 0 },
  ...spaces.value.map((s) => ({ label: s.name, value: s.id })),
]);

const scopeOptions = [
  { label: t('aiSet2.all'), value: 'all' },
  { label: t('aiSet2.k127'), value: 'favorites' },
];

const columns: TableColumn<SQLHistory>[] = [
  { key: 'query_text', title: t('aiSet2.k45'), ellipsis: true },
  { key: 'status', title: t('aiSet2.status'), width: AI_TABLE_WIDTH.sm, render: (row) => (row.status === 'success' ? '成功' : row.status || '-') },
  { key: 'rating', title: t('aiSet2.k119'), width: AI_TABLE_WIDTH.sm, render: (row) => (row.rating > 0 ? `${row.rating} / 5` : '未评分') },
  { key: 'duration_ms', title: t('aiSet2.k60'), width: AI_TABLE_WIDTH.md, render: (row) => `${row.duration_ms} ms` },
  { key: 'generator_tier', title: t('aiSet2.k128'), width: AI_TABLE_WIDTH.sm, render: (row) => row.generator_tier || '-' },
  { key: 'created_at', title: t('aiSet2.time'), width: AI_TABLE_WIDTH.xl, render: (row) => formatTime(row.created_at) },
];

function statusBadge(status: string): 'enabled' | 'warning' | 'error' {
  if (status === 'success') return 'enabled';
  if (!status) return 'warning';
  return 'error';
}

function statusText(status: string) {
  if (status === 'success') return '成功';
  return status || t('fallback.k76');
}

async function loadSpaces() {
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
  } catch (e) {
    logger.error('Failed to load spaces:', e);
  }
}

async function loadItems() {
  loading.value = true;
  try {
    const params = { space_id: selectedSpaceId.value, page: page.value, page_size: PAGE_SIZE };
    const res = scope.value === 'favorites' ? await SQLHistoryAPI.listFavorites(params) : await SQLHistoryAPI.list(params);
    items.value = res.data ?? [];
    total.value = res.total ?? 0;
  } catch (e) {
    logger.error('Failed to load sql history:', e);
    ui.showToast(t('aiSet2.k129'), 'error');
  } finally {
    loading.value = false;
  }
}

function onFilterChange() {
  page.value = 1;
  loadItems();
}

function changePage(next: number) {
  page.value = next;
  loadItems();
}

async function handleToggleFavorite(row: SQLHistory) {
  favoriteId.value = row.id;
  try {
    await SQLHistoryAPI.toggleFavorite(row.id);
    // toggle 接口不回传新状态，取详情作为权威值
    const fresh = await SQLHistoryAPI.detail(row.id);
    row.is_favorite = fresh.is_favorite;
    if (detail.value?.id === row.id) detail.value = fresh;
    ui.showToast(fresh.is_favorite ? '已收藏' : '已取消收藏', 'success');
    if (scope.value === 'favorites' && !fresh.is_favorite) await loadItems();
  } catch (e) {
    logger.error('Failed to toggle favorite:', e);
    ui.showToast(t('aiSet2.k130'), 'error');
  } finally {
    favoriteId.value = null;
  }
}

async function openDetail(row: SQLHistory) {
  try {
    detail.value = await SQLHistoryAPI.detail(row.id);
    trace.value = null;
    showDetailModal.value = true;
  } catch (e) {
    logger.error('Failed to load sql history detail:', e);
    ui.showToast(t('aiSet2.k131'), 'error');
  }
}

async function loadTrace() {
  const traceId = detail.value?.trace_id;
  if (!traceId) return;
  traceLoading.value = true;
  try {
    trace.value = await SQLTraceAPI.detail(traceId);
  } catch (e) {
    logger.error('Failed to load sql trace:', e);
    ui.showToast(t('aiSet2.k132'), 'error');
  } finally {
    traceLoading.value = false;
  }
}

async function handleRate(rating: number) {
  const current = detail.value;
  if (!current) return;
  ratingBusy.value = rating;
  try {
    await SQLHistoryAPI.rate(current.id, rating);
    detail.value = { ...current, rating };
    items.value = items.value.map((item) => (item.id === current.id ? { ...item, rating } : item));
    ui.showToast(t('sentUi.k44', { p1: rating }), 'success');
  } catch (e) {
    logger.error('Failed to rate sql history:', e);
    ui.showToast(t('aiSet2.k133'), 'error');
  } finally {
    ratingBusy.value = null;
  }
}

function handleDelete(row: SQLHistory) {
  deletingItem.value = row;
  showDeleteModal.value = true;
}

async function confirmDelete() {
  const target = deletingItem.value;
  if (!target) return;
  deleting.value = true;
  try {
    await SQLHistoryAPI.remove(target.id);
    ui.showToast(t('aiSet2.k134'), 'success');
    showDeleteModal.value = false;
    deletingItem.value = null;
    if (detail.value?.id === target.id) showDetailModal.value = false;
    await loadItems();
  } catch (e) {
    logger.error('Failed to delete sql history:', e);
    ui.showToast(t('aiSet2.k33'), 'error');
  } finally {
    deleting.value = false;
  }
}

onMounted(async () => {
  await loadSpaces();
  await loadItems();
});
</script>
