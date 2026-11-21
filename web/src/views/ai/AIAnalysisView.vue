<template>
  <div :style="aiSubPageStyle">
    <ApSegmentedControl v-model="topTab" :options="topTabs" />

    <!-- 质量工具 Tab -->
    <AIKnowledgeToolsView v-if="topTab === 'tools'" />

    <!-- 分析任务 Tab -->
    <template v-else>
    <div :style="compactStatsRowStyle">
      <ApStatCard v-for="stat in stats" :key="stat.label" :label="stat.label" :value="stat.value" :icon="stat.icon" :color="stat.color" size="compact" />
    </div>

    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderGapStyle">
          <ApIcon name="Play" :size="16" :color="cv('blue')" />
          <span :style="aiSubSectionTitleStyle">{{ $t('aiView.k1') }}</span>
        </div>
      </template>
      <div :style="compactFormGridStyle">
        <ApSelect :model-value="form.type" :options="typeOptions" :label="$t('aiView.k16')" @update:model-value="(v) => form.type = v as string" />
        <ApSelect v-if="form.type === 'space_only'" :model-value="form.spaceId ?? 0" :options="spaceOptions" :label="$t('aiView.k17')" :searchable="true" @update:model-value="(v) => form.spaceId = v as number" />
        <ApInput v-if="form.type === 'single_doc'" :model-value="form.docId ? String(form.docId) : ''" type="number" :label="$t('aiView.k18')" :placeholder="$t('aiView.k23')" @update:model-value="(v) => form.docId = v ? Number(v) : undefined" />
        <ApInput v-if="form.type === 'multi_doc'" :model-value="form.docIdsText" type="text" :label="$t('aiView.k19')" :placeholder="$t('aiView.k24')" @update:model-value="(v) => form.docIdsText = v" />
        <ApInput v-model="form.dateFrom" type="date" :label="$t('aiView.k20')" />
        <ApInput v-model="form.dateTo" type="date" :label="$t('aiView.k21')" />
      </div>
      <div :style="compactFormActionsStyle">
        <ApButton variant="primary" size="small" :loading="running" @click="onStartAnalysis"><ApIcon name="Play" :size="16" /> {{ $t('aiView.k1') }}</ApButton>
      </div>
    </ApCard>

    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderGapStyle">
          <ApIcon name="Clock" :size="16" :color="cv('orange')" />
          <span :style="aiSubSectionTitleStyle">{{ $t('aiView.k2') }}</span>
          <span :style="compactCountBadgeStyle">{{ tasks.length }}</span>
          <ApButton variant="ghost" size="mini" icon-only :aria-label="$t('aiView.k30')" :style="{ marginLeft: 'auto' }" @click="loadTasks"><ApIcon name="RefreshCw" :size="16" /></ApButton>
        </div>
      </template>
      <ApEmpty v-if="tasks.length === 0" icon="BarChart3" :title="$t('aiView.k25')" :description="$t('vAiUi.k1')" />
      <div v-else :style="compactCardListStyle">
        <div v-for="task in tasks" :key="task.id" :style="compactCardStyle">
          <div :style="compactTaskRowStyle">
            <span :style="typeBadgeStyle">{{ typeLabel(task.type) }}</span>
            <span :style="statusBadgeStyle(task.status)">{{ statusLabel(task.status) }}</span>
            <span :style="taskInfoStyle">{{ task.processed_docs }}/{{ task.total_docs }} {{ $t('uiTextMain.k14') }}</span>
            <span v-if="task.token_usage" :style="taskInfoStyle">{{ task.token_usage.toLocaleString() }} tokens</span>
            <div :style="compactTaskActionsStyle">
              <ApButton v-if="task.status === 'running'" variant="ghost" size="small" @click="pauseTask(task.id)">{{ $t('aiView.k3') }}</ApButton>
              <ApButton v-if="task.status === 'paused'" variant="ghost" size="small" @click="resumeTask(task.id)">{{ $t('aiView.k4') }}</ApButton>
            </div>
          </div>
          <ApProgress :value="task.progress" :label="$t('aiView.k22')" />
          <div v-if="task.message" :style="compactMessageStyle">{{ task.message }}</div>
        </div>
      </div>
    </ApCard>

    <div :style="compactTwoColStyle">
      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderGapStyle">
          <ApIcon name="Layers" :size="16" :color="cv('orange')" />
          <span :style="aiSubSectionTitleStyle">{{ $t('aiView.k5') }}</span>
            <ApButton variant="ghost" size="small" :style="{ marginLeft: 'auto' }" :loading="duplicatesLoading" @click="detectDuplicates"><ApIcon name="RefreshCw" :size="16" /> {{ $t('aiView.k6') }}</ApButton>
          </div>
        </template>
        <ApSkeleton v-if="duplicatesLoading" variant="card" :lines="3" />
        <ApEmpty v-else-if="duplicates.length === 0" :icon="'Copy'" :title="$t('aiView.k26')" :description="$t('vAiUi.k2')" />
        <div v-else :style="compactCardListStyle">
          <div v-for="(group, idx) in duplicates" :key="idx" :style="compactCardStyle">
            <div :style="compactDupHeaderStyle">
              <span :style="typeBadgeStyle">{{ $t('uiTextMain.k15', { p1: idx + 1 }) }}</span>
              <span :style="simBadgeStyle">{{ $t('uiTextMain.k16', { p1: Math.round((group.similarity || 0) * 100) }) }}</span>
            </div>
            <div :style="aiSubTagRowStyle">
              <span v-for="(docId, di) in group.doc_ids" :key="di" :style="aiSubTagStyle">{{ $t('uiTextMain.k17', { p1: docId }) }}</span>
            </div>
            <!-- 后端只会取前两篇生成方案，且不改任何文档；这两点必须写在按钮旁边，
                 否则"合并"两个字会让人以为按下去就动了内容 -->
            <div v-if="group.doc_ids.length >= 2" :style="dupActionRowStyle">
              <ApButton variant="secondary" size="small" :loading="mergingIdx === idx" @click="planMerge(group, idx)">
                {{ $t('aiView.k7') }}
              </ApButton>
              <span :style="aiSubSectionMetaStyle">{{ $t('aiView.k8') }}</span>
            </div>
          </div>
        </div>
      </ApCard>

      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderGapStyle">
          <ApIcon name="Network" :size="16" :color="cv('purple')" />
          <span :style="aiSubSectionTitleStyle">{{ $t('aiView.k9') }}</span>
            <ApButton variant="ghost" size="small" :style="{ marginLeft: 'auto' }" :loading="clustersLoading" @click="loadClusters"><ApIcon name="RefreshCw" :size="16" /> {{ $t('aiView.k10') }}</ApButton>
          </div>
        </template>
        <ApSkeleton v-if="clustersLoading" variant="card" :lines="3" />
        <ApEmpty v-else-if="clusters.length === 0" :icon="'Layers'" :title="$t('aiView.k27')" :description="$t('vAiUi.k3')" />
        <div v-else :style="compactCardListStyle">
          <div v-for="(c, idx) in clusters" :key="idx" :style="compactCardStyle">
            <div :style="compactDupHeaderStyle">
              <span :style="typeBadgeStyle">{{ c.label || $t('uiTextMain.k18', { p1: idx + 1 }) }}</span>
              <span :style="simBadgeStyle">{{ c.doc_ids?.length || 0 }} {{ $t('vAiUi.k5') }}</span>
            </div>
            <div :style="aiSubTagRowStyle">
              <span v-for="(docId, di) in (c.doc_ids || []).slice(0, 6)" :key="di" :style="aiSubTagStyle">#{{ docId }}</span>
              <span v-if="(c.doc_ids?.length || 0) > 6" :style="moreTagStyle">+{{ (c.doc_ids?.length || 0) - 6 }}</span>
            </div>
          </div>
        </div>
      </ApCard>
    </div>

    <ApCard v-if="results.length > 0 || resultsLoading" size="small">
      <template #header>
        <div :style="aiSubSectionHeaderGapStyle">
          <ApIcon name="FileText" :size="16" :color="cv('green')" />
          <span :style="aiSubSectionTitleStyle">{{ $t('aiView.k11') }}</span>
          <span :style="compactCountBadgeStyle">{{ results.length }}</span>
          <ApButton variant="ghost" size="mini" icon-only :aria-label="$t('aiView.k31')" :style="{ marginLeft: 'auto' }" @click="loadResults"><ApIcon name="RefreshCw" :size="16" /></ApButton>
        </div>
      </template>
      <ApSkeleton v-if="resultsLoading" variant="list" :lines="3" />
      <div v-else :style="compactCardListStyle">
        <div v-for="r in results" :key="r.id" :style="compactCardStyle">
          <div :style="compactDupHeaderStyle">
            <span :style="typeBadgeStyle">{{ r.category || $t('fallback.k72') }}</span>
            <span :style="scoreBadgeStyle(r.quality_score)">{{ $t('uiTextMain.k19', { p1: Math.round((r.quality_score || 0) * 100) }) }}</span>
          </div>
          <div v-if="r.summary" :style="compactSummaryStyle">{{ r.summary }}</div>
          <div v-if="r.key_points?.length" :style="aiSubTagRowStyle">
            <span v-for="(p, pi) in r.key_points.slice(0, 4)" :key="pi" :style="aiSubTagStyle">{{ p }}</span>
          </div>
        </div>
      </div>
    </ApCard>

    <ApConfirmModal
      v-model="showStartConfirm"
      :title="$t('aiView.k1')"
      :message="$t('vAiUi.k4')"
      :confirm-text="$t('aiView.confirm')"
      @confirm="startAnalysis"
    />

    <ApModal v-model="showPlan" :title="$t('aiView.k28')" size="medium">
      <div v-if="mergePlan" :style="formStyle">
        <div :style="planPairStyle">
          <span :style="typeBadgeStyle">{{ $t('uiTextMain.k20', { p1: mergePlan.target_doc_id }) }}</span>
          <span :style="typeBadgeStyle">{{ $t('uiTextMain.k21', { p1: mergePlan.source_doc_id }) }}</span>
        </div>
        <p :style="planReasonStyle">{{ mergePlan.reason || $t('fallback.k110') }}</p>
        <div v-if="mergePlan.sections_to_keep?.length">
          <div :style="aiSubSectionTitleStyle">{{ $t('aiView.k12') }}</div>
          <ul :style="planListStyle">
            <li v-for="(s, si) in mergePlan.sections_to_keep" :key="'keep-' + si">{{ s }}</li>
          </ul>
        </div>
        <div v-if="mergePlan.gaps_to_fill?.length">
          <div :style="aiSubSectionTitleStyle">{{ $t('aiView.k13') }}</div>
          <ul :style="planListStyle">
            <li v-for="(g, gi) in mergePlan.gaps_to_fill" :key="'gap-' + gi">{{ g }}</li>
          </ul>
        </div>
        <p :style="aiSubSectionMetaStyle">{{ $t('aiView.k14') }}</p>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showPlan = false">{{ $t('aiView.close') }}</ApButton>
      </template>
    </ApModal>
    </template>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { logger } from '@/utils/logger';
import { ref, computed, onMounted, onActivated } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import type { IconName } from '@/utils/icons';
import AIKnowledgeToolsView from './AIKnowledgeToolsView.vue';

import { cv, AppleFontWeight, AppleRadius, AppleSpace, AppleType, appleFixedColumns } from '@/utils/core/apple-tokens';
import { aiSubPageStyle, aiSubSectionHeaderGapStyle, aiSubSectionTitleStyle, aiSubSectionMetaStyle, formStyle, crudStatsRowStyle, aiSubCountBadgeStyle, aiSubActionRowStyle, aiSubCardStyle, aiSubCardHeaderStyle, aiSubTagRowStyle, aiSubTagStyle, listStyle, gridStandard } from '@/styles/ai/ai-page-styles';
import { useSpaceStore } from '@/stores/workspace/space';
import { useUIStore } from '@/stores/app/ui';
import { startAnalysis as apiStart, listAnalysisTasks, pauseAnalysis, resumeAnalysis, detectDuplicates as apiDetect, getClusters, mergeCluster, type MergePlan } from '@/api/modules/ai/ai-analysis';
import { getAnalysisResults } from '@/api/modules/ai/ai-analysis-result';
import type { AnalysisTask } from '@/types/ai';
import { useBreakpoint } from '@/utils/responsive';

const { isMobile } = useBreakpoint();
const ui = useUIStore();
const spaceStore = useSpaceStore();
const spaceOptions = computed(() => (spaceStore.mySpaces || []).map(s => ({ label: s.name, value: s.id })));

const topTab = ref('analysis');
const topTabs = [
  { value: 'analysis', label: t('aiView.k2'), icon: 'BarChart3' },
  { value: 'tools', label: t('aiView.k32'), icon: 'Shield' },
];

const running = ref(false);
const duplicatesLoading = ref(false);
const clustersLoading = ref(false);
const resultsLoading = ref(false);
const tasks = ref<AnalysisTask[]>([]);
const duplicates = ref<Array<{ doc_ids: number[]; similarity: number }>>([]);

const mergingIdx = ref<number | null>(null);
const showPlan = ref(false);
const mergePlan = ref<MergePlan | null>(null);
const clusters = ref<Array<{ id: string; label: string; doc_ids: number[] }>>([]);
const results = ref<Array<{ id: number; doc_id?: number; category?: string; quality_score?: number; summary?: string; key_points?: string[] }>>([]);

const form = ref({ type: 'full', docId: undefined as number | undefined, docIdsText: '', spaceId: undefined as number | undefined, dateFrom: '', dateTo: '' });
const typeOptions = [
  { label: t('aiView.k33'), value: 'full' },
  { label: t('aiView.k34'), value: 'quick' },
  { label: t('aiView.k35'), value: 'incremental' },
  { label: t('aiView.k36'), value: 'space_only' },
  { label: t('aiView.k37'), value: 'single_doc' },
  { label: t('aiView.k38'), value: 'multi_doc' },
];

const stats = computed<Array<{ label: string; value: string; icon: IconName; color: string }> >(() => [
  { label: t('aiView.k39'), value: String(tasks.value.filter(t => t.status === 'running').length), icon: 'Play', color: cv('blue') },
  { label: t('aiView.k40'), value: String(tasks.value.filter(t => t.status === 'completed').length), icon: 'BarChart3', color: cv('green') },
  { label: t('aiView.k41'), value: String(duplicates.value.reduce((s, d) => s + (d.doc_ids?.length || 0), 0)), icon: 'Layers', color: cv('orange') },
  { label: t('aiView.k42'), value: String(clusters.value.length), icon: 'Network', color: cv('purple') },
]);

async function startAnalysis() {
  running.value = true;
  try {
    const params: Record<string, unknown> = { type: form.value.type };
    if (form.value.type === 'single_doc' && form.value.docId) params.target_doc_id = form.value.docId;
    if (form.value.type === 'multi_doc' && form.value.docIdsText) {
      const ids = form.value.docIdsText.split(/[,，\s]+/).map(s => parseInt(s.trim(), 10)).filter(n => !isNaN(n) && n > 0);
      if (ids.length === 0) {
        ui.showToast(t('aiView.k44'), 'error');
        running.value = false;
        return;
      }
      params.doc_ids = ids;
    }
    if (form.value.type === 'space_only' && form.value.spaceId) params.target_space_ids = [form.value.spaceId];
    if (form.value.dateFrom) params.date_from = form.value.dateFrom;
    if (form.value.dateTo) params.date_to = form.value.dateTo;
    await apiStart(params as never);
    ui.showToast(t('aiView.k45'), 'success');
    await loadTasks();
  } catch (e) {
    logger.error('Failed to start analysis:', e);
    ui.showToast(t('aiView.k46'), 'error');
  }
  running.value = false;
}

async function loadTasks() {
  try {
    const res = await listAnalysisTasks();
    tasks.value = (res.list || res.tasks || []).map((t: unknown) => {
      const o = t as Record<string, unknown>;
      const rawProgress = Number(o.progress || o.progress_percent || 0);
      const displayProgress = rawProgress < 1 ? rawProgress * 100 : rawProgress;
      return {
        id: String(o.id || o.task_id || ''),
        type: String(o.type || 'full'),
        status: String(o.status || 'pending'),
        progress: Math.min(100, Math.max(0, displayProgress)),
        processed_docs: Number(o.processed_docs || 0),
        total_docs: Number(o.total_docs || 0),
        token_usage: Number(o.token_usage || 0),
        message: String(o.message || ''),
        target_space_ids: (o.target_space_ids as number[]) || [],
      } as AnalysisTask & { target_space_ids?: number[] };
    });
    loadResults();
  } catch (e) {
    logger.error('Failed to load analysis tasks:', e);
    ui.showToast(t('aiView.k47'), 'error');
  }
}

async function planMerge(group: { doc_ids: number[]; similarity: number }, idx: number) {
  if (group.doc_ids.length < 2) return;
  mergingIdx.value = idx;
  try {
    // 后端只用前两篇（第三个及以后不参与），所以传前两篇并说清这件事
    const res = await mergeCluster(idx + 1, group.doc_ids.slice(0, 2));
    if (!res?.plan) {
      ui.showToast(t('aiView.k48'), 'warning');
      return;
    }
    mergePlan.value = res.plan;
    showPlan.value = true;
  } catch (e) {
    logger.error('Failed to generate merge plan:', e);
    ui.showToast(e instanceof Error && e.message ? e.message: t('aiView.k43'), 'error');
  } finally {
    mergingIdx.value = null;
  }
}

async function pauseTask(id: string) {
  try {
    await pauseAnalysis(id);
    ui.showToast(t('aiView.k49'), 'success');
    await loadTasks();
  } catch (e) {
    logger.error('Failed to pause analysis:', e);
    ui.showToast(t('aiView.k50'), 'error');
  }
}
async function resumeTask(id: string) {
  try {
    await resumeAnalysis(id);
    ui.showToast(t('aiView.k51'), 'success');
    await loadTasks();
  } catch (e) {
    logger.error('Failed to resume analysis:', e);
    ui.showToast(t('aiView.k52'), 'error');
  }
}

async function detectDuplicates() {
  duplicatesLoading.value = true;
  try {
    const res = await apiDetect({ space_id: 0, threshold: 0.7 }) as Record<string, unknown>;
    duplicates.value = ((res.duplicates || res.groups || []) as Array<Record<string, unknown>>).map((d: Record<string, unknown>) => ({
      doc_ids: (d.doc_ids as number[]) || [],
      similarity: Number(d.similarity || 0),
    }));
    ui.showToast(t('sentUi.k32', { p1: duplicates.value.length }), 'info');
  } catch (e) {
    logger.error('Failed to detect duplicates:', e);
    ui.showToast(t('aiView.k53'), 'error');
  }
  duplicatesLoading.value = false;
}

async function loadClusters() {
  clustersLoading.value = true;
  try {
    const res = await getClusters();
    clusters.value = (res.clusters || []).map((c: Record<string, unknown>) => ({
      id: String(c.id || ''),
      label: String(c.label || ''),
      doc_ids: (c.doc_ids as number[]) || [],
    }));
  } catch (e) {
    logger.error('Failed to load clusters:', e);
    ui.showToast(t('aiView.k54'), 'error');
  }
  clustersLoading.value = false;
}

async function loadResults() {
  resultsLoading.value = true;
  try {
    const spaceId = resolveResultSpaceId();
    if (spaceId > 0) {
      const res = await getAnalysisResults(spaceId, 1, 20);
      results.value = ((res.list || []) as unknown[]).map((r) => {
        const o = r as Record<string, unknown>;
        return {
          id: Number(o.id || 0),
          doc_id: Number(o.doc_id || 0),
          category: String(o.category || ''),
          quality_score: Number(o.quality_score || 0),
          summary: String(o.summary || ''),
          key_points: (o.key_points as string[]) || [],
        };
      });
    }
  } catch (e) {
    logger.error('Failed to load analysis results:', e);
    ui.showToast(t('aiView.k55'), 'error');
  }
  resultsLoading.value = false;
}

function resolveResultSpaceId(): number {
  if (form.value.type === 'space_only' && form.value.spaceId) {
    return form.value.spaceId;
  }
  if (tasks.value.length > 0) {
    const latestCompleted = [...tasks.value].reverse().find(
      (t) => {
        const spaceIds = (t as Record<string, unknown>).target_space_ids as number[] | undefined;
        return t.status === 'completed' && spaceIds && spaceIds.length > 0;
      },
    );
    if (latestCompleted) {
      const spaceIds = (latestCompleted as Record<string, unknown>).target_space_ids as number[];
      if (spaceIds && spaceIds.length > 0) {
        return spaceIds[0] ?? 0;
      }
    }
  }
  if (spaceStore.mySpaces && spaceStore.mySpaces.length > 0) {
    return spaceStore.mySpaces[0]?.id ?? 0;
  }
  return 0;
}

function typeLabel(type: string) {
  const map: Record<string, string> = { quick: t('uiTextMain.k22'), full: t('uiTextMain.k23'), incremental: t('uiTextMain.k24'), space_only: t('uiTextMain.k25'), single_doc: t('uiTextMain.k26'), multi_doc: t('uiTextMain.k27') };
  return map[type] || type;
}
function statusLabel(s: string) {
  const map: Record<string, string> = { pending: t('uiTextMain.k28'), running: t('common.running'), paused: t('uiTextMain.k29'), completed: t('uiTextMain.k30'), failed: t('common.failed') };
  return map[s] || s;
}
function statusBadgeStyle(s: string) {
  const colors: Record<string, string> = { running: cv('blue'), completed: cv('green'), failed: cv('red'), paused: cv('orange'), pending: cv('text-tertiary') };
  return { fontSize: AppleType.legalMeta.size, fontWeight: String(AppleFontWeight.semibold), color: colors[s] || cv('text-tertiary') };
}
function scoreBadgeStyle(score: number | undefined) {
  const s = score ?? 0;
  return { fontSize: AppleType.legalMeta.size, padding: `${AppleSpace.xxs} ${AppleSpace.sm}`, borderRadius: AppleRadius.pill, color: s > 0.7 ? cv('green') : cv('orange'), fontWeight: String(AppleFontWeight.medium) };
}

onMounted(() => {
  spaceStore.loadMySpaces();
  loadTasks();
});

onActivated(() => {
  // KeepAlive 缓存恢复时刷新数据，避免显示旧内容
  loadTasks();
});

const taskInfoStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-secondary'), marginLeft: 'auto' };
const typeBadgeStyle = aiSubTagStyle;
const simBadgeStyle = { fontSize: AppleType.legalMeta.size, padding: `${AppleSpace.xxs} ${AppleSpace.sm}`, borderRadius: AppleRadius.pill, color: cv('orange'), fontWeight: String(AppleFontWeight.medium) };
const moreTagStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-tertiary'), padding: `${AppleSpace.xxs} ${AppleSpace.sm}` };
const showStartConfirm = ref(false);
function onStartAnalysis() {
  showStartConfirm.value = true;
}

const compactStatsRowStyle = { ...crudStatsRowStyle('compact'), marginBottom: 0 };

const compactFormGridStyle = { ...gridStandard, gap: AppleSpace.sm, marginBottom: AppleSpace.sm };
const compactFormActionsStyle = { ...aiSubActionRowStyle, gap: AppleSpace.sm, justifyContent: 'flex-end' as const };
const compactCardListStyle = listStyle(AppleSpace.sm);
const compactCardStyle = { ...aiSubCardStyle, padding: AppleSpace.sm };
const compactTaskRowStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.xs, marginBottom: AppleSpace.xs, flexWrap: 'wrap' as const };
const compactTaskActionsStyle = { marginLeft: AppleSpace.xs };
const compactMessageStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-secondary'), marginTop: AppleSpace.xxs };
const compactTwoColStyle = computed(() => ({ display: 'grid', gridTemplateColumns: isMobile.value ? '1fr' : appleFixedColumns(2), gridAutoRows: '1fr', gap: AppleSpace.md }));
const compactDupHeaderStyle = { ...aiSubCardHeaderStyle, gap: AppleSpace.xs, marginBottom: AppleSpace.xs };
const compactCountBadgeStyle = { ...aiSubCountBadgeStyle };
const compactSummaryStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-secondary'), lineHeight: 1.4, marginBottom: AppleSpace.xs };

const dupActionRowStyle = { display: 'flex', alignItems: 'center' as const, gap: AppleSpace.sm, flexWrap: 'wrap' as const };
const planPairStyle = { display: 'flex', gap: AppleSpace.xs, flexWrap: 'wrap' as const };
const planReasonStyle = { margin: '0', color: cv('text-secondary') };
const planListStyle = { margin: '0', paddingLeft: AppleSpace.lg, display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xxs };
</script>
