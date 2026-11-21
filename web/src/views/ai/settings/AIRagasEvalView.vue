<template>
  <div :style="aiSubPageStyle">
    <!-- 指标均值：后端 total 为 0 时不返回均值键，统一显示为 — -->
    <div :style="crudStatsRowFitStyle(5, isMobile, 'compact')">
      <ApStatCard
        :label="$t('aiSet2.k40')"
        :value="stats.total"
        :icon="'Database'"
        :color="cv('blue')"
        size="compact"
      />
      <ApStatCard
        :label="$t('aiSet2.k41')"
        :value="formatScore(stats.avg_faithfulness)"
        :icon="'ShieldCheck'"
        :color="cv('green')"
        size="compact"
      />
      <ApStatCard
        :label="$t('aiSet2.k42')"
        :value="formatScore(stats.avg_answer_relevancy)"
        :icon="'Target'"
        :color="cv('orange')"
        size="compact"
      />
      <ApStatCard
        :label="$t('aiSet2.k43')"
        :value="formatScore(stats.avg_context_precision)"
        :icon="'ListChecks'"
        :color="cv('purple')"
        size="compact"
      />
      <ApStatCard
        :label="$t('aiSet2.k44')"
        :value="formatScore(stats.avg_context_recall)"
        :icon="'Gauge'"
        :color="cv('teal')"
        size="compact"
      />
    </div>

    <!-- 发起评估 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <div :style="aiSubCardNameRowStyle">
            <ApIcon name="FlaskConical" :size="18" :color="cv('indigo')" />
            <span :style="aiSubSectionTitleStyle">{{ $t('aiSet2.k34') }}</span>
          </div>
          <span :style="aiSubDescStyle">{{ $t('aiSet2.k35') }}</span>
        </div>
      </template>

      <div :style="formStyle">
        <ApTextarea
          v-model="form.question"
          :label="$t('aiSet2.k45')"
          :placeholder="$t('aiSet2.k49')"
          :rows="3"
          required
        />
        <ApTextarea
          v-model="form.answer"
          :label="$t('aiSet2.k46')"
          :placeholder="$t('aiSet2.k50')"
          :rows="4"
          required
        />
        <ApTextarea
          v-model="form.contexts"
          :label="$t('aiSet2.k47')"
          :placeholder="$t('aiSet2.k51')"
          :rows="3"
        />
        <ApInput
          v-model="form.ground_truth"
          :label="$t('aiSet2.k48')"
          :placeholder="$t('aiSet2.k52')"
        />
      </div>

      <div :style="formActionsStyle">
        <ApButton variant="primary" size="small" :loading="evaluating" @click="handleEvaluate">
          <ApIcon name="FlaskConical" :size="16" /> {{ $t('aiSet2.k36') }}
        </ApButton>
        <ApButton variant="ghost" size="small" :disabled="evaluating" @click="resetForm">{{ $t('aiSet2.reset') }}</ApButton>
      </div>

      <div v-if="evaluateError" :style="aiSubServerErrorStyle">
        <ApIcon name="AlertTriangle" :size="14" /> {{ evaluateError }}
      </div>
      <div v-else-if="lastQuestion" :style="aiSubResultStyle(true)">
        最近一次评估「{{ lastQuestion }}」平均分 {{ formatScore(lastAvgScore) }}，耗时 {{ lastDuration }}
      </div>
    </ApCard>

    <!-- 结果表 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <div :style="aiSubCardNameRowStyle">
            <ApIcon name="ListChecks" :size="18" :color="cv('blue')" />
            <span :style="aiSubSectionTitleStyle">{{ $t('aiSet2.k38') }}</span>
            <span :style="aiSubCountBadgeStyle">{{ results.length }}</span>
          </div>
          <ApButton variant="secondary" size="small" :loading="loading" @click="loadAll">
            <ApIcon name="RotateCw" :size="16" /> {{ $t('aiSet2.refresh') }}
          </ApButton>
        </div>
      </template>

      <ApTable density="compact" :columns="columns" :data="results" :loading="loading || deleting">
        <template #actions="{ row }">
          <ApButton
            variant="danger"
            size="small"
            @click="handleDelete(row as unknown as RAGASEvalResult)"
          >
            <ApIcon name="Trash2" :size="16" /> {{ $t('aiSet2.delete') }}
          </ApButton>
        </template>
      </ApTable>
      <ApEmpty
        v-if="!loading && !deleting && results.length === 0"
        icon="FileText"
        :title="$t('aiSet2.k53')"
        :description="$t('vAiUi.k219')"
      />
    </ApCard>

    <ApConfirmModal
      v-model="showDeleteModal"
      :title="$t('aiSet2.k54')"
      :message="`确定删除「${deletingItem?.question ?? ''}」的评估记录吗？数据库历史行不会同步删除。`"
      type="danger"
      :confirm-text="$t('aiSet2.delete')"
      @confirm="confirmDelete"
    />
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, onMounted } from 'vue';

import { logger } from '@/utils/logger';
import { cv } from '@/utils/core/apple-tokens';
import { useBreakpoint } from '@/utils/responsive';
import {
  aiSubPageStyle,
  aiSubSectionHeaderStyle,
  aiSubSectionTitleStyle,
  aiSubCardNameRowStyle,
  aiSubDescStyle,
  aiSubCountBadgeStyle,
  aiSubServerErrorStyle,
  aiSubResultStyle,
  crudStatsRowFitStyle,
  formStyle,
  formActionsStyle,
  AI_TABLE_WIDTH,
} from '@/styles/ai/ai-page-styles';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import { RAGASAPI, type RAGASEvalResult, type RAGASStats } from '@/api/modules/ai/ai-ragas';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();
const { isMobile } = useBreakpoint();

const stats = ref<RAGASStats>({ total: 0 });
const results = ref<RAGASEvalResult[]>([]);
const loading = ref(false);
const evaluating = ref(false);
const deleting = ref(false);
const evaluateError = ref('');
const lastQuestion = ref('');
const lastAvgScore = ref(0);
const lastDuration = ref('');

const showDeleteModal = ref(false);
const deletingItem = ref<RAGASEvalResult | null>(null);

const form = ref({ question: '', answer: '', contexts: '', ground_truth: '' });

function formatScore(value?: number): string {
  if (value === undefined || value === null) return '—';
  return `${(value * 100).toFixed(1)}%`;
}

const columns: TableColumn<RAGASEvalResult>[] = [
  { key: 'question', title: t('aiSet2.k45'), ellipsis: true },
  { key: 'avg_score', title: t('aiSet2.k55'), width: AI_TABLE_WIDTH.md, align: 'right', render: (row) => formatScore(row.avg_score) },
  { key: 'faithfulness', title: t('aiSet2.k56'), width: AI_TABLE_WIDTH.sm, align: 'right', render: (row) => formatScore(row.metrics.faithfulness) },
  { key: 'answer_relevancy', title: t('aiSet2.k57'), width: AI_TABLE_WIDTH.sm, align: 'right', render: (row) => formatScore(row.metrics.answer_relevancy) },
  { key: 'context_precision', title: t('aiSet2.k58'), width: AI_TABLE_WIDTH.sm, align: 'right', render: (row) => formatScore(row.metrics.context_precision) },
  { key: 'context_recall', title: t('aiSet2.k59'), width: AI_TABLE_WIDTH.sm, align: 'right', render: (row) => formatScore(row.metrics.context_recall) },
  { key: 'duration', title: t('aiSet2.k60'), width: AI_TABLE_WIDTH.sm, align: 'right' },
  { key: 'created_at', title: t('aiSet2.time'), width: AI_TABLE_WIDTH.lg },
];

async function loadStats() {
  try {
    stats.value = await RAGASAPI.stats();
  } catch (e) {
    logger.error('Failed to load ragas stats:', e);
    ui.showToast(t('aiSet2.k63'), 'error');
  }
}

async function loadResults() {
  loading.value = true;
  try {
    const res = await RAGASAPI.results({ limit: 50 });
    results.value = res.results ?? [];
  } catch (e) {
    logger.error('Failed to load ragas results:', e);
    ui.showToast(t('aiSet2.k64'), 'error');
  } finally {
    loading.value = false;
  }
}

async function loadAll() {
  await Promise.all([loadStats(), loadResults()]);
}

function resetForm() {
  form.value = { question: '', answer: '', contexts: '', ground_truth: '' };
  evaluateError.value = '';
}

async function handleEvaluate() {
  if (!form.value.question.trim() || !form.value.answer.trim()) {
    ui.showToast(t('aiSet2.k65'), 'warning');
    return;
  }
  evaluating.value = true;
  evaluateError.value = '';
  try {
    const res = await RAGASAPI.evaluate({
      question: form.value.question.trim(),
      answer: form.value.answer.trim(),
      contexts: splitContexts(form.value.contexts),
      ground_truth: form.value.ground_truth.trim(),
    });
    const result = res.result;
    lastQuestion.value = result.question;
    lastAvgScore.value = result.avg_score;
    lastDuration.value = result.duration;
    ui.showToast(t('aiSet2.k66'), 'success');
    await loadAll();
  } catch (e) {
    logger.error('Failed to run ragas evaluation:', e);
    evaluateError.value = e instanceof Error ? e.message: t('aiSet2.k62');
  } finally {
    evaluating.value = false;
  }
}

function splitContexts(raw: string): string[] {
  return raw
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
}

function handleDelete(row: RAGASEvalResult) {
  deletingItem.value = row;
  showDeleteModal.value = true;
}

async function confirmDelete() {
  const target = deletingItem.value;
  if (!target) return;
  deleting.value = true;
  try {
    await RAGASAPI.deleteResult(target.id);
    ui.showToast(t('aiSet2.k67'), 'success');
    showDeleteModal.value = false;
    deletingItem.value = null;
    await loadAll();
  } catch (e) {
    logger.error('Failed to delete ragas result:', e);
    ui.showToast(t('aiSet2.k33'), 'error');
  } finally {
    deleting.value = false;
  }
}

onMounted(() => loadAll());
</script>
