<template>
  <div :style="aiSubPageStyle">
    <ApSegmentedControl v-model="activeTab" :options="tabs" />

    <!-- ==================== 数据集管理 ==================== -->
    <ApCard v-if="activeTab === 'datasets'" size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <div :style="aiSubCardNameRowStyle">
            <ApIcon name="Database" :size="AppleIconSize.stat" :color="cv('blue')" />
            <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k126') }}</span>
            <span :style="aiSubCountBadgeStyle">{{ datasetsTotal }}</span>
          </div>
          <div :style="aiSubActionRowStyle">
            <ApButton variant="secondary" size="small" :loading="datasetsLoading" @click="loadDatasets">
              <ApIcon name="RotateCw" :size="AppleIconSize.control" /> {{ $t('aiSet.refresh') }}
            </ApButton>
            <ApButton variant="primary" size="small" @click="openDatasetModal()">
              <ApIcon name="Plus" :size="AppleIconSize.control" /> {{ $t('aiSet.k127') }}
            </ApButton>
          </div>
        </div>
      </template>

      <p :style="aiSubDescStyle">
        数据集至少包含 1 条问题；难度字段仅作参考、不参与打分。列表按创建时间倒序，最多展示 {{ LIST_LIMIT }} 条。
      </p>

      <ApTable density="compact" :columns="datasetColumns" :data="datasets" :loading="datasetsLoading">
        <template #empty>
          <ApEmpty icon="Inbox" :title="$t('aiSet.k188')" :description="$t('vAiUi.k211')" />
        </template>
        <template #actions="{ row }">
          <ApButton variant="ghost" size="small" @click="openDatasetDetail(row as unknown as EvalDataset)">
            {{ $t('aiSet.k128') }}
          </ApButton>
          <ApButton variant="danger" size="small" @click="handleDeleteDataset(row as unknown as EvalDataset)">
            <ApIcon name="Trash2" :size="AppleIconSize.control" /> {{ $t('aiSet.delete') }}
          </ApButton>
        </template>
      </ApTable>
    </ApCard>

    <!-- ==================== 用例管理 ==================== -->
    <ApCard v-else-if="activeTab === 'cases'" size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <div :style="aiSubCardNameRowStyle">
            <ApIcon name="ListChecks" :size="AppleIconSize.stat" :color="cv('indigo')" />
            <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k129') }}</span>
            <span :style="aiSubCountBadgeStyle">{{ casesTotal }}</span>
          </div>
          <div :style="aiSubActionRowStyle">
            <ApButton variant="secondary" size="small" :loading="casesLoading" @click="loadCases">
              <ApIcon name="RotateCw" :size="AppleIconSize.control" /> {{ $t('aiSet.refresh') }}
            </ApButton>
            <ApButton
              variant="outline"
              size="small"
              :loading="suiteRunning"
              :disabled="cases.length === 0"
              @click="runSuite()"
            >
              <ApIcon name="Play" :size="AppleIconSize.control" /> {{ $t('aiSet.k130') }}
            </ApButton>
            <ApButton variant="primary" size="small" @click="openCaseModal()">
              <ApIcon name="Plus" :size="AppleIconSize.control" /> {{ $t('aiSet.k131') }}
            </ApButton>
          </div>
        </div>
      </template>

      <p :style="aiSubDescStyle">
        {{ $t('aiSet.k132') }}
      </p>

      <ApTable density="compact" :columns="caseColumns" :data="cases" :loading="casesLoading">
        <template #empty>
          <ApEmpty icon="FileText" :title="$t('aiSet.k189')" :description="$t('vAiUi.k212')" />
        </template>
        <template #actions="{ row }">
          <ApButton variant="ghost" size="small" :loading="suiteRunning" @click="runSuite([(row as unknown as EvalTestCase).id])">
            {{ $t('aiSet.k133') }}
          </ApButton>
          <ApButton variant="ghost" size="small" @click="openCaseModal(row as unknown as EvalTestCase)">
            {{ $t('aiSet.edit') }}
          </ApButton>
        </template>
      </ApTable>

      <div v-if="suite" :style="aiSubSectionStyle">
        <div :style="aiSubSectionHeaderStyle">
          <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k134') }}</span>
          <span :style="aiSubCountBadgeStyle">
            通过 {{ suite.passed }} / 失败 {{ suite.failed }} · 通过率 {{ formatPercent(suite.pass_rate) }} · 总耗时 {{ suite.total_latency }}ms
          </span>
        </div>
        <ApTable density="compact" :columns="suiteColumns" :data="suite.results" />
      </div>
    </ApCard>

    <!-- ==================== 运行评估 ==================== -->
    <template v-else-if="activeTab === 'run'">
      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <div :style="aiSubCardNameRowStyle">
              <ApIcon name="FlaskConical" :size="AppleIconSize.stat" :color="cv('purple')" />
              <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k135') }}</span>
            </div>
          </div>
        </template>

        <div :style="aiSubFilterStyle">
          <span :style="aiSubFilterLabelStyle">{{ $t('aiSet.k136') }}</span>
          <ApSelect
            v-model="runDatasetId"
            :options="datasetOptions"
            :placeholder="$t('aiSet.k172')"
            :searchable="true"
          />
          <ApButton
            variant="primary"
            size="small"
            :loading="running"
            :disabled="!runDatasetId"
            @click="handleRunEval"
          >
            <ApIcon name="Play" :size="AppleIconSize.control" /> {{ $t('aiSet.k137') }}
          </ApButton>
        </div>

        <p :style="aiSubDescStyle">
          {{ $t('aiSet.k138') }}
        </p>

        <div v-if="runError" :style="aiSubServerErrorStyle">
          <ApIcon name="AlertTriangle" :size="AppleIconSize.inline" /> {{ runError }}
        </div>

        <template v-if="runResult">
          <div :style="crudStatsRowStyle('compact')">
            <ApStatCard :label="$t('aiSet.k152')" :value="formatPercent(runResult.overall_score)" icon="Gauge" :color="cv('blue')" size="compact" />
            <ApStatCard :label="$t('aiSet.k153')" :value="formatMetric(runResult.metrics?.faithfulness)" icon="ShieldCheck" :color="cv('green')" size="compact" />
            <ApStatCard :label="$t('aiSet.k154')" :value="formatMetric(runResult.metrics?.answer_relevance)" icon="Target" :color="cv('orange')" size="compact" />
            <ApStatCard :label="$t('aiSet.k155')" :value="formatMetric(runResult.metrics?.context_precision)" icon="ListChecks" :color="cv('purple')" size="compact" />
            <ApStatCard :label="$t('aiSet.k156')" :value="formatMetric(runResult.metrics?.context_recall)" icon="Layers" :color="cv('teal')" size="compact" />
            <ApStatCard :label="$t('aiSet.k157')" :value="formatMetric(runResult.metrics?.answer_correctness)" icon="CheckCircle2" :color="cv('indigo')" size="compact" />
          </div>

          <!-- 三层归因：答案错了先看检索层（没找到资料）→ 增强层（资料没进 prompt）→ 生成层（有资料却答错） -->
          <div v-if="runLayerScores" :style="layerGroupStyle">
            <div :style="layerGroupTitleStyle">{{ $t('aiSet.layerAttribution') }}</div>
            <div :style="layerCardsStyle">
              <ApStatCard :label="$t('aiSet.layerRetrieval')" :value="formatMetric(runLayerScores.retrieval)" icon="Search" :color="cv('teal')" size="compact" />
              <ApStatCard :label="$t('aiSet.layerEnrich')" :value="formatMetric(runLayerScores.enrich)" icon="Layers" :color="cv('purple')" size="compact" />
              <ApStatCard :label="$t('aiSet.layerGeneration')" :value="formatMetric(runLayerScores.generation)" icon="PenTool" :color="cv('green')" size="compact" />
            </div>
            <p :style="layerHintStyle">{{ $t('aiSet.layerHint') }}</p>
          </div>

          <div :style="aiSubResultStyle(runResult.status === 'completed')">
            <ApIcon v-if="runResult.status === 'completed'" name="CheckCircle2" :size="AppleIconSize.inline" />
            <span>{{ $t('aiSet.k285', { name: runResult.dataset_name, status: runResult.status, duration: runResult.duration }) }}</span>
          </div>
        </template>
      </ApCard>

      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <div :style="aiSubCardNameRowStyle">
              <ApIcon name="Activity" :size="AppleIconSize.stat" :color="cv('teal')" />
              <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k139') }}</span>
              <span :style="aiSubCountBadgeStyle">{{ resultsTotal }}</span>
            </div>
            <ApButton variant="secondary" size="small" :loading="resultsLoading" @click="loadResults">
              <ApIcon name="RotateCw" :size="AppleIconSize.control" /> {{ $t('aiSet.refresh') }}
            </ApButton>
          </div>
        </template>

        <ApTable density="compact" :columns="resultColumns" :data="results" :loading="resultsLoading || resultDeleting">
          <template #empty>
            <ApEmpty icon="FileText" :title="$t('aiSet.k190')" :description="$t('vAiUi.k213')" />
          </template>
          <template #actions="{ row }">
            <ApButton variant="ghost" size="small" @click="openResultDetail(row as unknown as EvalResult)">
              {{ $t('aiSet.k140') }}
            </ApButton>
            <ApButton variant="danger" size="small" @click="handleDeleteResult(row as unknown as EvalResult)">
              <ApIcon name="Trash2" :size="AppleIconSize.control" /> {{ $t('aiSet.delete') }}
            </ApButton>
          </template>
        </ApTable>
      </ApCard>
    </template>

    <!-- ==================== 结果对比 ==================== -->
    <template v-else>
      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <div :style="aiSubCardNameRowStyle">
              <ApIcon name="GitCompare" :size="AppleIconSize.stat" :color="cv('pink')" />
              <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k141') }}</span>
              <span :style="aiSubCountBadgeStyle">{{ variants.length }}</span>
            </div>
            <div :style="aiSubActionRowStyle">
              <ApButton variant="secondary" size="small" @click="addVariant">
                <ApIcon name="Plus" :size="AppleIconSize.control" /> {{ $t('aiSet.k142') }}
              </ApButton>
              <ApButton
                variant="primary"
                size="small"
                :loading="comparing"
                :disabled="variants.length < 2 || cases.length === 0"
                @click="handleABCompare"
              >
                <ApIcon name="Play" :size="AppleIconSize.control" /> {{ $t('aiSet.k143') }}
              </ApButton>
            </div>
          </div>
        </template>

        <p :style="aiSubDescStyle">
          {{ $t('aiSet.k144') }}
        </p>

        <div v-for="(variant, idx) in variants" :key="idx" :style="aiSubSectionStyle">
          <div :style="aiSubSectionHeaderStyle">
            <span :style="aiSubFilterLabelStyle">{{ $t('aiSet.k286', { n: idx + 1 }) }}</span>
            <ApButton variant="ghost" size="small" :disabled="variants.length <= 2" @click="removeVariant(idx)">
              <ApIcon name="Trash2" :size="AppleIconSize.control" /> {{ $t('aiSet.k145') }}
            </ApButton>
          </div>
          <div :style="formRowStyle">
            <ApInput v-model="variant.id" :label="$t('aiSet.k158')" :placeholder="$t('aiSet.k173')" required />
            <ApInput v-model="variant.name" :label="$t('aiSet.k159')" :placeholder="$t('aiSet.k174')" />
          </div>
          <div :style="formRowStyle">
            <ApInput v-model="variant.provider" label="Provider" :placeholder="$t('aiSet.k175')" />
            <ApInput v-model="variant.model" :label="$t('aiSet.k160')" :placeholder="$t('aiSet.k176')" />
          </div>
          <div :style="formRowStyle">
            <ApInput v-model="variant.temperature" type="number" label="Temperature" placeholder="0.2" />
            <ApInput v-model="variant.top_k" type="number" label="Top K" placeholder="5" />
          </div>
        </div>

        <div v-if="compareError" :style="aiSubServerErrorStyle">
          <ApIcon name="AlertTriangle" :size="AppleIconSize.inline" /> {{ compareError }}
        </div>
      </ApCard>

      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <div :style="aiSubCardNameRowStyle">
              <ApIcon name="GitBranch" :size="AppleIconSize.stat" :color="cv('blue')" />
              <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k146') }}</span>
              <span :style="aiSubCountBadgeStyle">{{ abResults.length }}</span>
            </div>
          </div>
        </template>

        <ApTable
          v-if="abResults.length > 0"
          density="compact"
          :columns="abColumns"
          :data="abResults"
          :row-key="'test_case_id'"
        />
        <ApEmpty
          v-else
          icon="Search"
          :title="$t('aiSet.k191')"
          :description="$t('vAiUi.k214')"
        />
      </ApCard>
    </template>

    <!-- ==================== 弹层 ==================== -->
    <ApModal v-model="showDatasetModal" :title="$t('aiSet.k192')" size="large">
      <div :style="formStyle">
        <div :style="formRowStyle">
          <ApInput v-model="datasetForm.name" :label="$t('aiSet.k161')" :placeholder="$t('aiSet.k177')" required />
          <div>
            <div :style="aiSubFilterLabelStyle">{{ $t('aiSet.k35') }}</div>
            <ApSelect v-model="datasetForm.space_id" :options="spaceOptions" :placeholder="$t('aiSet.k13')" :searchable="true" />
          </div>
        </div>

        <div v-for="(item, idx) in datasetForm.questions" :key="idx" :style="aiSubSectionStyle">
          <div :style="aiSubSectionHeaderStyle">
            <span :style="aiSubFilterLabelStyle">{{ $t('aiSet.k287', { n: idx + 1 }) }}</span>
            <ApButton variant="ghost" size="small" :disabled="datasetForm.questions.length === 1" @click="removeQuestion(idx)">
              <ApIcon name="Trash2" :size="AppleIconSize.control" /> {{ $t('aiSet.k145') }}
            </ApButton>
          </div>
          <ApTextarea v-model="item.question" :label="$t('aiSet.k43')" :placeholder="$t('aiSet.k178')" :rows="2" required />
          <ApTextarea v-model="item.expected_answer" :label="$t('aiSet.k162')" :placeholder="$t('aiSet.k179')" :rows="2" />
          <ApTextarea v-model="item.expected_context" :label="$t('aiSet.k163')" :placeholder="$t('aiSet.k180')" :rows="2" />
          <div>
            <div :style="aiSubFilterLabelStyle">{{ $t('aiSet.k147') }}</div>
            <ApSelect v-model="item.difficulty" :options="difficultyOptions" />
          </div>
        </div>

        <ApButton variant="outline" size="small" @click="addQuestion">
          <ApIcon name="Plus" :size="AppleIconSize.control" /> {{ $t('aiSet.k148') }}
        </ApButton>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showDatasetModal = false">{{ $t('aiSet.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="datasetSaving" @click="handleSaveDataset">{{ $t('aiSet.k149') }}</ApButton>
      </template>
    </ApModal>

    <ApModal v-model="showCaseModal" :title="editingCaseId ? $t('aiSet.k288') : $t('aiSet.k289')" size="large">
      <div :style="formStyle">
        <div :style="formRowStyle">
          <ApInput v-model="caseForm.id" :label="$t('aiSet.k164')" :placeholder="$t('aiSet.k181')" :readonly="!!editingCaseId" />
          <ApInput v-model="caseForm.name" :label="$t('aiSet.k165')" :placeholder="$t('aiSet.k182')" />
        </div>
        <div :style="formRowStyle">
          <ApTextarea v-model="caseForm.question" :label="$t('aiSet.k166')" :rows="3" required />
          <div>
            <div :style="aiSubFilterLabelStyle">{{ $t('aiSet.k35') }}</div>
            <ApSelect v-model="caseForm.space_id" :options="spaceOptions" :placeholder="$t('aiSet.k13')" :searchable="true" />
            <div :style="aiSubFilterLabelStyle">{{ $t('aiSet.k150') }}</div>
            <ApInput v-model="caseForm.max_latency_ms" type="number" :placeholder="$t('aiSet.k183')" />
          </div>
        </div>
        <ApTextarea v-model="caseForm.contains_any" :label="$t('aiSet.k167')" :placeholder="$t('aiSet.k184')" :rows="2" />
        <ApTextarea v-model="caseForm.contains_all" :label="$t('aiSet.k168')" :placeholder="$t('aiSet.k184')" :rows="2" />
        <ApTextarea v-model="caseForm.not_contains" :label="$t('aiSet.k169')" :placeholder="$t('aiSet.k185')" :rows="2" />
        <ApTextarea v-model="caseForm.expected_doc_ids" :label="$t('aiSet.k170')" :placeholder="$t('aiSet.k186')" :rows="2" />
        <ApTextarea v-model="caseForm.tags" :label="$t('aiSet.k171')" :placeholder="$t('aiSet.k187')" :rows="2" />
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showCaseModal = false">{{ $t('aiSet.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="caseSaving" @click="handleSaveCase">
          {{ editingCaseId ? $t('vAiUi.k215') : $t('vAiUi.k216') }}
        </ApButton>
      </template>
    </ApModal>

    <ApModal v-model="showQuestionsModal" :title="$t('aiSet.k290', { name: viewingDataset?.name ?? '' })" size="medium">
      <ApTable density="compact" :columns="questionColumns" :data="viewingQuestions" />
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showQuestionsModal = false">{{ $t('aiSet.close') }}</ApButton>
      </template>
    </ApModal>

    <ApModal v-model="showResultModal" :title="$t('aiSet.k277', { n: resultDetail?.id ?? '' })" size="large">
      <div v-if="resultDetail" :style="aiSubTagRowStyle">
        <span :style="aiSubTagStyle">{{ $t('aiSet.k278', { n: formatPercent(resultDetail.overall_score) }) }}</span>
        <span :style="aiSubTagStyle">{{ $t('aiSet.k279', { n: formatMetric(resultDetail.metrics?.faithfulness) }) }}</span>
        <span :style="aiSubTagStyle">{{ $t('aiSet.k280', { n: formatMetric(resultDetail.metrics?.answer_relevance) }) }}</span>
        <span :style="aiSubTagStyle">{{ $t('aiSet.k281', { n: formatMetric(resultDetail.metrics?.context_precision) }) }}</span>
        <span :style="aiSubTagStyle">{{ $t('aiSet.k282', { n: formatMetric(resultDetail.metrics?.context_recall) }) }}</span>
        <span :style="aiSubTagStyle">{{ $t('aiSet.k283', { n: formatMetric(resultDetail.metrics?.answer_correctness) }) }}</span>
        <span :style="aiSubTagStyle">{{ $t('aiSet.w4Routing', { n: formatMetric(resultDetail.metrics?.routing_accuracy) }) }}</span>
        <span :style="aiSubTagStyle">{{ $t('aiSet.w4HitAtK', { n: formatMetric(resultDetail.metrics?.retrieval_hit_at_k) }) }}</span>
        <span :style="aiSubTagStyle">{{ $t('aiSet.w4NDCG', { n: formatMetric(resultDetail.metrics?.rerank_ndcg) }) }}</span>
        <span :style="aiSubTagStyle">{{ $t('aiSet.w4Noise', { n: formatMetric(resultDetail.metrics?.noise_robustness) }) }}</span>
      </div>
      <div :style="aiSubCardNameRowStyle">
        <span :style="aiSubDescStyle">{{ $t('aiSet.k284', { n1: resultDetail?.details?.length ?? 0, n2: resultDetail?.duration ?? '—' }) }}</span>
      </div>
      <div v-for="(item, idx) in resultDetail?.details ?? []" :key="idx" :style="aiSubSectionStyle">
        <div :style="aiSubSectionHeaderStyle">
          <span :style="aiSubFilterLabelStyle">{{ idx + 1 }}. {{ item.question }}</span>
        </div>
        <div :style="aiSubTagRowStyle">
          <span :style="aiSubTagStyle">{{ $t('aiSet.k291', { n: formatMetric(item.metrics.faithfulness) }) }}</span>
          <span :style="aiSubTagStyle">{{ $t('aiSet.k292', { n: formatMetric(item.metrics.answer_relevance) }) }}</span>
          <span :style="aiSubTagStyle">{{ $t('aiSet.k293', { n: formatMetric(item.metrics.answer_correctness) }) }}</span>
        </div>
        <pre :style="aiSubBodyStyle">{{ item.generated_answer || $t('fallback.k113') }}</pre>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showResultModal = false">{{ $t('aiSet.close') }}</ApButton>
      </template>
    </ApModal>

    <ApConfirmModal
      v-model="showDatasetDeleteModal"
      :title="$t('aiSet.k193')"
      :message="$t('aiSet.k294', { name: deletingDataset?.name ?? '' })"
      type="danger"
      :confirm-text="$t('aiSet.delete')"
      :loading="datasetDeleting"
      @confirm="confirmDeleteDataset"
    />

    <ApConfirmModal
      v-model="showResultDeleteModal"
      :title="$t('aiSet.k194')"
      :message="$t('aiSet.k295', { id: deletingResult?.id ?? '', dataset: deletingResult?.dataset_name ?? '' })"
      type="danger"
      :confirm-text="$t('aiSet.delete')"
      :loading="resultDeleting"
      @confirm="confirmDeleteResult"
    />
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted } from 'vue';

import { logger } from '@/utils/logger';
import { cv, AppleType, AppleFontWeight, AppleRadius, AppleIconSize } from '@/utils/core/apple-tokens';
import ApIcon from '@/components/apple/ApIcon.vue';
import {
  AI_TABLE_WIDTH,
  crudStatsRowStyle,
  aiSubActionRowStyle,
  aiSubBodyStyle,
  aiSubCardNameRowStyle,
  aiSubCountBadgeStyle,
  aiSubDescStyle,
  aiSubFilterLabelStyle,
  aiSubFilterStyle,
  aiSubPageStyle,
  aiSubResultStyle,
  aiSubSectionHeaderStyle,
  aiSubSectionStyle,
  aiSubSectionTitleStyle,
  aiSubServerErrorStyle,
  aiSubTagRowStyle,
  aiSubTagStyle,
  formRowStyle,
  formStyle,
} from '@/styles/ai/ai-page-styles';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import {
  EvalAPI,
  evalLayerScores,
  type EvalABResult,
  type EvalAgentVariant,
  type EvalDataset,
  type EvalQuestion,
  type EvalResult,
  type EvalSuiteRun,
  type EvalTestCase,
  type EvalTestResult,
  type ExpectedOutcome,
} from '@/api/modules/ai/ai-eval';
import { useSpaceStore } from '@/stores/workspace/space';
import type { Space } from '@/types/models';
import { useUIStore } from '@/stores/app/ui';

/** 后端 list 接口默认 limit 20，这里统一取一页 */
const LIST_LIMIT = 50;

const ui = useUIStore();
const spaceStore = useSpaceStore();

const tabs = [
  { label: t('aiSet.k195'), value: 'datasets' },
  { label: t('aiSet.k196'), value: 'cases' },
  { label: t('aiSet.k197'), value: 'run' },
  { label: t('aiSet.k198'), value: 'compare' },
];
const activeTab = ref<string>('datasets');

const spaces = ref<Space[]>([]);
const difficultyOptions = [
  { label: 'easy', value: 'easy' },
  { label: 'medium', value: 'medium' },
  { label: 'hard', value: 'hard' },
];
const spaceOptions = computed(() => [
  { label: t('aiSet.k199'), value: 0 },
  ...spaces.value.map((s) => ({ label: s.name, value: s.id })),
]);

import { formatPercent } from '@/utils/format/format-time';

function formatMetric(value?: number): string {
  return formatPercent(value);
}

function splitLines(raw: string): string[] {
  return raw
    .split('\n')
    .map((line) => line.trim())
    .filter((line) => line.length > 0);
}

function joinLines(list?: string[] | null): string {
  return (list ?? []).join('\n');
}

/* -------------------- 数据集 -------------------- */

const datasets = ref<EvalDataset[]>([]);
const datasetsTotal = ref(0);
const datasetsLoading = ref(false);
const datasetSaving = ref(false);
const datasetDeleting = ref(false);
const showDatasetModal = ref(false);
const showDatasetDeleteModal = ref(false);
const deletingDataset = ref<EvalDataset | null>(null);

interface QuestionForm {
  question: string;
  expected_answer: string;
  expected_context: string;
  difficulty: string;
}

function emptyQuestion(): QuestionForm {
  return { question: '', expected_answer: '', expected_context: '', difficulty: 'medium' };
}

const datasetForm = ref<{ name: string; space_id: number | string | null; questions: QuestionForm[] }>({
  name: '',
  space_id: 0,
  questions: [emptyQuestion()],
});

const datasetColumns: TableColumn<EvalDataset>[] = [
  { key: 'id', title: 'ID', width: AI_TABLE_WIDTH.sm },
  { key: 'name', title: t('aiSet.name'), ellipsis: true },
  { key: 'space_id', title: t('aiSet.k200'), width: AI_TABLE_WIDTH.sm },
  { key: 'question_count', title: t('aiSet.k201'), width: AI_TABLE_WIDTH.md, render: (row) => row.questions?.length ?? 0 },
  { key: 'created_at', title: t('aiSet.k202'), width: AI_TABLE_WIDTH.lg },
];

async function loadDatasets() {
  datasetsLoading.value = true;
  try {
    const res = await EvalAPI.datasets({ limit: LIST_LIMIT });
    datasets.value = res.datasets ?? [];
    datasetsTotal.value = res.total ?? 0;
  } catch (e) {
    logger.error('Failed to load eval datasets:', e);
    ui.showToast(t('aiSet.k222'), 'error');
  } finally {
    datasetsLoading.value = false;
  }
}

function openDatasetModal() {
  datasetForm.value = { name: '', space_id: 0, questions: [emptyQuestion()] };
  showDatasetModal.value = true;
}

function addQuestion() {
  datasetForm.value.questions.push(emptyQuestion());
}

function removeQuestion(index: number) {
  datasetForm.value.questions.splice(index, 1);
}

async function handleSaveDataset() {
  const name = datasetForm.value.name.trim();
  const questions = datasetForm.value.questions
    .map((item) => ({
      question: item.question.trim(),
      expected_answer: item.expected_answer.trim(),
      expected_context: splitLines(item.expected_context),
      difficulty: item.difficulty,
    }))
    .filter((item) => item.question.length > 0);
  if (!name) {
    ui.showToast(t('aiSet.k223'), 'warning');
    return;
  }
  if (questions.length === 0) {
    ui.showToast(t('aiSet.k224'), 'warning');
    return;
  }
  datasetSaving.value = true;
  try {
    await EvalAPI.createDataset({ name, space_id: Number(datasetForm.value.space_id) || 0, questions });
    ui.showToast(t('aiSet.k225'), 'success');
    showDatasetModal.value = false;
    await loadDatasets();
  } catch (e) {
    logger.error('Failed to create eval dataset:', e);
    ui.showToast(t('aiSet.k226'), 'error');
  } finally {
    datasetSaving.value = false;
  }
}

function handleDeleteDataset(row: EvalDataset) {
  deletingDataset.value = row;
  showDatasetDeleteModal.value = true;
}

async function confirmDeleteDataset() {
  const target = deletingDataset.value;
  if (!target) return;
  datasetDeleting.value = true;
  try {
    await EvalAPI.deleteDataset(target.id);
    ui.showToast(t('aiSet.k227'), 'success');
    showDatasetDeleteModal.value = false;
    deletingDataset.value = null;
    await Promise.all([loadDatasets(), loadResults()]);
  } catch (e) {
    logger.error('Failed to delete eval dataset:', e);
    ui.showToast(t('aiSet.k32'), 'error');
  } finally {
    datasetDeleting.value = false;
  }
}

/* 题目只读视图：列表已带 questions，仍走详情接口确认最新内容 */
const showQuestionsModal = ref(false);
const viewingDataset = ref<EvalDataset | null>(null);
const viewingQuestions = ref<EvalQuestion[]>([]);

const questionColumns: TableColumn<EvalQuestion>[] = [
  { key: 'question', title: t('aiSet.k43'), ellipsis: true },
  { key: 'expected_answer', title: t('aiSet.k162'), ellipsis: true },
  { key: 'difficulty', title: t('aiSet.k203'), width: AI_TABLE_WIDTH.sm },
  { key: 'context_count', title: t('aiSet.k204'), width: AI_TABLE_WIDTH.md, render: (row) => row.expected_context?.length ?? 0 },
];

async function openDatasetDetail(row: EvalDataset) {
  try {
    const res = await EvalAPI.dataset(row.id);
    viewingDataset.value = res.dataset;
    viewingQuestions.value = res.dataset.questions ?? [];
  } catch (e) {
    logger.error('Failed to load eval dataset detail:', e);
    viewingDataset.value = row;
    viewingQuestions.value = row.questions ?? [];
  }
  showQuestionsModal.value = true;
}

/* -------------------- 测试用例 -------------------- */

const cases = ref<EvalTestCase[]>([]);
const casesTotal = ref(0);
const casesLoading = ref(false);
const caseSaving = ref(false);
const suiteRunning = ref(false);
const suite = ref<EvalSuiteRun | null>(null);
const showCaseModal = ref(false);
const editingCaseId = ref<string | null>(null);

function emptyCaseForm() {
  return {
    id: '',
    name: '',
    question: '',
    space_id: 0 as number | string | null,
    max_latency_ms: '',
    contains_any: '',
    contains_all: '',
    not_contains: '',
    expected_doc_ids: '',
    tags: '',
  };
}

const caseForm = ref(emptyCaseForm());

function summarizeExpected(outcome: ExpectedOutcome): string {
  const parts: string[] = [];
  if (outcome.contains_any?.length) parts.push(`任一:${outcome.contains_any.join('/')}`);
  if (outcome.contains_all?.length) parts.push(`全部:${outcome.contains_all.join('/')}`);
  if (outcome.not_contains?.length) parts.push(`禁用:${outcome.not_contains.join('/')}`);
  if (outcome.expected_doc_ids?.length) parts.push(`文档:${outcome.expected_doc_ids.join('/')}`);
  if (outcome.expected_tables?.length) parts.push(`表:${outcome.expected_tables.join('/')}`);
  if (outcome.max_latency_ms) parts.push(`≤${outcome.max_latency_ms}ms`);
  return parts.length ? parts.join(' · ') : '无断言';
}

const caseColumns: TableColumn<EvalTestCase>[] = [
  { key: 'name', title: t('aiSet.name'), width: AI_TABLE_WIDTH.lg, ellipsis: true },
  { key: 'question', title: t('aiSet.k166'), ellipsis: true },
  { key: 'space_id', title: t('aiSet.k200'), width: AI_TABLE_WIDTH.sm },
  { key: 'expected', title: t('aiSet.k205'), width: AI_TABLE_WIDTH.xxl, render: (row) => summarizeExpected(row.expected ?? {}) },
  { key: 'tags', title: t('aiSet.k171'), width: AI_TABLE_WIDTH.md, render: (row) => (row.tags ?? []).join(', ') || '—' },
];

async function loadCases() {
  casesLoading.value = true;
  try {
    const res = await EvalAPI.testCases();
    cases.value = res.test_cases ?? [];
    casesTotal.value = res.total ?? cases.value.length;
  } catch (e) {
    logger.error('Failed to load eval test cases:', e);
    ui.showToast(t('aiSet.k228'), 'error');
  } finally {
    casesLoading.value = false;
  }
}

function openCaseModal(item?: EvalTestCase) {
  if (item) {
    editingCaseId.value = item.id;
    caseForm.value = {
      id: item.id,
      name: item.name,
      question: item.question,
      space_id: item.space_id ?? 0,
      max_latency_ms: item.expected?.max_latency_ms ? String(item.expected.max_latency_ms) : '',
      contains_any: joinLines(item.expected?.contains_any),
      contains_all: joinLines(item.expected?.contains_all),
      not_contains: joinLines(item.expected?.not_contains),
      expected_doc_ids: (item.expected?.expected_doc_ids ?? []).join('\n'),
      tags: joinLines(item.tags),
    };
  } else {
    editingCaseId.value = null;
    caseForm.value = emptyCaseForm();
  }
  showCaseModal.value = true;
}

function buildExpected(form: typeof caseForm.value): ExpectedOutcome {
  const docIds = splitLines(form.expected_doc_ids)
    .map((raw) => Number(raw))
    .filter((num) => Number.isFinite(num));
  const latency = Number(form.max_latency_ms);
  const outcome: ExpectedOutcome = {
    contains_any: splitLines(form.contains_any),
    contains_all: splitLines(form.contains_all),
    not_contains: splitLines(form.not_contains),
    expected_doc_ids: docIds,
  };
  if (Number.isFinite(latency) && latency > 0) outcome.max_latency_ms = latency;
  return outcome;
}

async function handleSaveCase() {
  const question = caseForm.value.question.trim();
  if (!question) {
    ui.showToast(t('aiSet.k229'), 'warning');
    return;
  }
  const payload: EvalTestCase = {
    id: caseForm.value.id.trim(),
    name: caseForm.value.name.trim() || caseForm.value.id.trim(),
    question,
    space_id: Number(caseForm.value.space_id) || 0,
    expected: buildExpected(caseForm.value),
    tags: splitLines(caseForm.value.tags),
  };
  caseSaving.value = true;
  try {
    await EvalAPI.createTestCase(payload);
    ui.showToast(editingCaseId.value ? '用例已覆盖保存' : '用例已创建', 'success');
    showCaseModal.value = false;
    await loadCases();
  } catch (e) {
    logger.error('Failed to save eval test case:', e);
    ui.showToast(t('aiSet.k230'), 'error');
  } finally {
    caseSaving.value = false;
  }
}

const suiteColumns: TableColumn<EvalTestResult>[] = [
  { key: 'test_case_id', title: t('aiSet.k206'), width: AI_TABLE_WIDTH.lg, ellipsis: true },
  { key: 'passed', title: t('aiSet.k207'), width: AI_TABLE_WIDTH.sm, render: (row) => (row.passed ? '通过' : '失败') },
  { key: 'duration_ms', title: t('aiSet.k208'), width: AI_TABLE_WIDTH.sm, render: (row) => `${row.duration_ms}ms` },
  { key: 'actual_answer', title: t('aiSet.k209'), ellipsis: true },
  {
    key: 'failures',
    title: t('aiSet.k210'),
    width: AI_TABLE_WIDTH.xxl,
    render: (row) => (row.failures ?? []).map((f) => `${f.type}: ${f.expected}`).join(' · ') || '—',
  },
];

async function runSuite(ids?: string[]) {
  suiteRunning.value = true;
  try {
    const res = await EvalAPI.runTestCases(ids?.length ? { test_case_ids: ids } : {});
    suite.value = res;
    ui.showToast(t('sentUi.k41', { p1: res.passed, p2: res.total }), res.failed > 0 ? 'warning' : 'success');
  } catch (e) {
    logger.error('Failed to run eval test suite:', e);
    ui.showToast(e instanceof Error ? e.message: t('aiSet.k211'), 'error');
  } finally {
    suiteRunning.value = false;
  }
}

/* -------------------- 运行评估 -------------------- */

const runDatasetId = ref<number | string | null>(null);
const running = ref(false);
const runError = ref('');
const runResult = ref<EvalResult | null>(null);

// 三层归因：把指标聚合到 检索/增强/生成 三层，定位「答案错了是哪层坏的」
const runLayerScores = computed(() => evalLayerScores(runResult.value?.metrics));

const layerGroupStyle = {
  marginTop: cv('space-12px'),
  padding: cv('space-12px'),
  border: `1px solid ${cv('border')}`,
  borderRadius: AppleRadius.float,
};
const layerGroupTitleStyle = {
  fontWeight: AppleFontWeight.semibold,
  marginBottom: cv('space-10px'),
  fontSize: AppleType.caption.size,
};
const layerCardsStyle = {
  display: 'grid',
  gridTemplateColumns: 'repeat(3, 1fr)',
  gap: cv('space-10px'),
};
const layerHintStyle = {
  marginTop: cv('space-10px'),
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  lineHeight: 1.6,
};

const datasetOptions = computed(() =>
  datasets.value.map((item) => ({ label: t('sentUi.k42', { p1: item.name, p2: item.questions?.length ?? 0 }), value: item.id })),
);

async function handleRunEval() {
  const id = Number(runDatasetId.value);
  if (!id) {
    ui.showToast(t('aiSet.k231'), 'warning');
    return;
  }
  running.value = true;
  runError.value = '';
  try {
    const res = await EvalAPI.run({ dataset_id: id });
    runResult.value = res.result;
    ui.showToast(t('aiSet.k232'), 'success');
    await loadResults();
  } catch (e) {
    logger.error('Failed to run eval:', e);
    runError.value = e instanceof Error ? e.message: t('aiSet.k212');
  } finally {
    running.value = false;
  }
}

/* -------------------- 结果记录 -------------------- */

const results = ref<EvalResult[]>([]);
const resultsTotal = ref(0);
const resultsLoading = ref(false);
const resultDeleting = ref(false);
const showResultDeleteModal = ref(false);
const deletingResult = ref<EvalResult | null>(null);
const showResultModal = ref(false);
const resultDetail = ref<EvalResult | null>(null);

const resultColumns: TableColumn<EvalResult>[] = [
  { key: 'id', title: 'ID', width: AI_TABLE_WIDTH.sm, align: 'right' },
  { key: 'dataset_name', title: t('aiSet.k136'), width: AI_TABLE_WIDTH.lg, ellipsis: true },
  { key: 'overall_score', title: t('aiSet.k213'), width: AI_TABLE_WIDTH.md, align: 'right', render: (row) => formatPercent(row.overall_score) },
  { key: 'faithfulness', title: t('aiSet.k153'), width: AI_TABLE_WIDTH.sm, align: 'right', render: (row) => formatMetric(row.metrics?.faithfulness) },
  { key: 'answer_relevance', title: t('aiSet.k214'), width: AI_TABLE_WIDTH.sm, align: 'right', render: (row) => formatMetric(row.metrics?.answer_relevance) },
  { key: 'context_precision', title: t('aiSet.k215'), width: AI_TABLE_WIDTH.sm, align: 'right', render: (row) => formatMetric(row.metrics?.context_precision) },
  { key: 'context_recall', title: t('aiSet.k216'), width: AI_TABLE_WIDTH.sm, align: 'right', render: (row) => formatMetric(row.metrics?.context_recall) },
  { key: 'answer_correctness', title: t('aiSet.k217'), width: AI_TABLE_WIDTH.sm, align: 'right', render: (row) => formatMetric(row.metrics?.answer_correctness) },
  { key: 'status', title: t('aiSet.status'), width: AI_TABLE_WIDTH.sm },
  { key: 'duration', title: t('aiSet.k208'), width: AI_TABLE_WIDTH.md, align: 'right' },
  { key: 'created_at', title: t('aiSet.time'), width: AI_TABLE_WIDTH.lg },
];

async function loadResults() {
  resultsLoading.value = true;
  try {
    const res = await EvalAPI.results({ limit: LIST_LIMIT });
    results.value = res.results ?? [];
    resultsTotal.value = res.total ?? 0;
  } catch (e) {
    logger.error('Failed to load eval results:', e);
    ui.showToast(t('aiSet.k233'), 'error');
  } finally {
    resultsLoading.value = false;
  }
}

async function openResultDetail(row: EvalResult) {
  try {
    const res = await EvalAPI.result(row.id);
    resultDetail.value = res.result;
  } catch (e) {
    logger.error('Failed to load eval result detail:', e);
    resultDetail.value = row;
  }
  showResultModal.value = true;
}

function handleDeleteResult(row: EvalResult) {
  deletingResult.value = row;
  showResultDeleteModal.value = true;
}

async function confirmDeleteResult() {
  const target = deletingResult.value;
  if (!target) return;
  resultDeleting.value = true;
  try {
    await EvalAPI.deleteResult(target.id);
    ui.showToast(t('aiSet.k234'), 'success');
    showResultDeleteModal.value = false;
    deletingResult.value = null;
    await loadResults();
  } catch (e) {
    logger.error('Failed to delete eval result:', e);
    ui.showToast(t('aiSet.k32'), 'error');
  } finally {
    resultDeleting.value = false;
  }
}

/* -------------------- A/B 对比 -------------------- */

interface VariantForm {
  id: string;
  name: string;
  provider: string;
  model: string;
  temperature: string;
  top_k: string;
}

function emptyVariant(id: string): VariantForm {
  return { id, name: '', provider: '', model: '', temperature: '', top_k: '' };
}

const variants = ref<VariantForm[]>([emptyVariant('A'), emptyVariant('B')]);
const comparing = ref(false);
const compareError = ref('');
const abResults = ref<EvalABResult[]>([]);

function addVariant() {
  variants.value.push(emptyVariant(String.fromCharCode(65 + variants.value.length)));
}

function removeVariant(index: number) {
  variants.value.splice(index, 1);
}

function buildVariants(): EvalAgentVariant[] {
  return variants.value.map((item, idx) => {
    const temperature = Number(item.temperature);
    const topK = Number(item.top_k);
    return {
      id: item.id.trim() || String.fromCharCode(65 + idx),
      name: item.name.trim() || item.id.trim(),
      config: {
        provider: item.provider.trim() || undefined,
        model: item.model.trim() || undefined,
        temperature: Number.isFinite(temperature) && item.temperature !== '' ? temperature : undefined,
        top_k: Number.isFinite(topK) && item.top_k !== '' ? topK : undefined,
      },
    };
  });
}

async function handleABCompare() {
  const payload = buildVariants();
  if (payload.length < 2) {
    ui.showToast(t('aiSet.k235'), 'warning');
    return;
  }
  comparing.value = true;
  compareError.value = '';
  try {
    const res = await EvalAPI.abCompare({ variants: payload });
    abResults.value = res.results ?? [];
    ui.showToast(t('sentUi.k43', { p1: res.total }), 'success');
  } catch (e) {
    logger.error('Failed to run A/B compare:', e);
    compareError.value = e instanceof Error ? e.message: t('aiSet.k219');
  } finally {
    comparing.value = false;
  }
}

const abVariantIds = computed<string[]>(() => {
  const ids = new Set<string>();
  abResults.value.forEach((item) => Object.keys(item.variants ?? {}).forEach((key) => ids.add(key)));
  return [...ids];
});

function variantCell(row: EvalABResult, variantId: string): string {
  const result = row.variants?.[variantId];
  if (!result) return '—';
  return `${result.passed ? '通过' : '失败'} ${result.duration_ms}ms`;
}

const abColumns = computed<TableColumn<EvalABResult>[]>(() => [
  { key: 'test_case_id', title: t('aiSet.k206'), width: AI_TABLE_WIDTH.lg, ellipsis: true },
  { key: 'winner', title: t('aiSet.k220'), width: AI_TABLE_WIDTH.md, render: (row) => row.winner || t('fallback.k10') },
  ...abVariantIds.value.map((variantId) => ({
    key: `variant-${variantId}`,
    title: variantId,
    width: AI_TABLE_WIDTH.md,
    render: (row: EvalABResult) => variantCell(row, variantId),
  })),
  { key: 'summary', title: t('aiSet.k221'), ellipsis: true },
]);

onMounted(async () => {
  await Promise.all([loadDatasets(), loadCases(), loadResults()]);
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
  } catch (e) {
    logger.error('Failed to load spaces:', e);
  }
});
</script>
