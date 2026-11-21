<template>
  <div :style="aiSubPageStyle">
    <!-- 统计概览 -->
    <div :style="statsGridStyle">
      <ApStatCard :label="$t('aiView.k216')" :value="bases.length" :icon="'Database'" :color="cv('blue')" size="compact" />
      <ApStatCard :label="$t('aiView.k200')" :value="totalDocCount" :icon="'FileText'" :color="cv('green')" size="compact" />
      <ApStatCard :label="$t('aiView.k187')" :value="totalVectorized" :icon="'Layers'" :color="cv('purple')" size="compact" />
      <ApStatCard :label="$t('aiView.k217')" :value="`${completionRate}%`" :icon="'Activity'" :color="cv('orange')" size="compact" />
    </div>

    <!-- 知识库列表 -->
    <div :style="{ ...aiSubSectionHeaderStyle, marginBottom: 0 }">
      <span :style="sectionTitleStyle">{{ $t('aiView.k182') }}</span>
      <ApButton variant="ghost" size="small" :loading="loading" @click="fetchBases">
        <ApIcon name="RefreshCw" :size="16" /> {{ $t('aiView.refresh') }}
      </ApButton>
    </div>

    <!-- 加载中 -->
    <ApSkeleton v-if="loading && bases.length === 0" variant="grid" :count="4" />

    <!-- 空状态 -->
    <ApEmpty
      v-else-if="bases.length === 0"
      :title="$t('aiView.k220')"
      :description="$t('vAiUi.k26')"
      icon="Database"
    />

    <!-- 知识库卡片网格 -->
    <div v-else :style="gridStyle">
      <div
        v-for="kb in bases"
        :key="kb.id"
        v-bind="keyboardClick(() => selectBase(kb))"
        :style="cardStyle(kb.id === selectedId)"
      >
        <div :style="cardHeaderRowStyle">
          <div :style="cardTitleWrapStyle">
            <span :style="cardNameStyle">{{ kb.name }}</span>
            <span :style="cardDescStyle">{{ kb.description || $t('fallback.k70') }}</span>
          </div>
          <div :style="statusDotStyle(kb.status === 1)" />
        </div>
        <div :style="cardMetricsStyle">
          <div :style="metricItemStyle">
            <ApIcon name="FileText" :size="14" :color="cv('text-tertiary')" />
            <span>{{ kb.doc_count }} {{ $t('vAiUi.k29') }}</span>
          </div>
          <div v-if="kb.last_updated" :style="metricItemStyle">
            <ApIcon name="Clock" :size="14" :color="cv('text-tertiary')" />
            <span>{{ formatTime(kb.last_updated) }}</span>
          </div>
        </div>
        <div :style="cardFooterStyle">
          <ApButton
            variant="ghost"
            size="small"
            @click.stop="selectBase(kb)"
          >
            <ApIcon name="Eye" :size="16" /> {{ $t('aiView.k184') }}
          </ApButton>
        </div>
      </div>
    </div>

    <!-- 详情面板 -->
    <div v-if="selectedBase" :style="kbDetailPanelStyle">
      <div :style="detailHeaderStyle">
        <div>
          <div :style="detailTitleStyle">{{ selectedBase.name }}</div>
          <div :style="detailSubtitleStyle">{{ selectedBase.description || $t('fallback.k92') }}</div>
        </div>
        <ApButton variant="ghost" size="mini" icon-only :aria-label="$t('aiView.close')" @click="selectedId = null">
          <ApIcon name="X" :size="16" />
        </ApButton>
      </div>

      <!-- Tab 切换 -->
      <div :style="tabContainerStyle">
        <ApSegmentedControl
          v-model="activeTab"
          :options="tabOptions"
          size="small"
        />
      </div>

      <!-- Tab 1: 文档列表 -->
      <div v-if="activeTab === 'docs'">
        <div v-if="docsLoading" :style="loadingStyle">
          <span>{{ $t('aiView.k185') }}</span>
        </div>
        <ApEmpty v-else-if="docList.length === 0" icon="Folder" :title="$t('aiView.k221')" :description="$t('vAiUi.k27')" />
        <div v-else>
          <!-- 文档统计 -->
          <div :style="docStatsRowStyle">
            <div :style="docStatItemStyle">
              <span :style="docStatLabelStyle">{{ $t('aiView.k186') }}</span>
              <span :style="docStatValueStyle()">{{ chunkInfo.total_docs }}</span>
            </div>
            <div :style="docStatItemStyle">
              <span :style="docStatLabelStyle">{{ $t('aiView.k187') }}</span>
              <span :style="docStatValueStyle(cv('green'))">{{ chunkInfo.vectorized_count }}</span>
            </div>
            <div :style="docStatItemStyle">
              <span :style="docStatLabelStyle">{{ $t('aiView.k188') }}</span>
              <span :style="docStatValueStyle(cv('blue'))">{{ docVectorRate }}%</span>
            </div>
          </div>
          <!-- 文档表格 -->
          <div :style="tableContainerStyle">
            <div :style="tableHeaderStyle">
              <span :style="thStyle({ flex: '3' })">{{ $t('aiView.k189') }}</span>
              <span :style="thStyle({ flex: '1', textAlign: 'center' })">{{ $t('aiView.type') }}</span>
              <span :style="thStyle({ flex: '1.5', textAlign: 'center' })">{{ $t('aiView.k191') }}</span>
              <span :style="thStyle({ flex: '1.5', textAlign: 'right' })">{{ $t('aiView.k192') }}</span>
            </div>
            <div
              v-for="doc in docList"
              :key="doc.doc_id"
              :style="tableRowStyle"
            >
              <span :style="tdStyle({ flex: '3' })">
                <ApIcon name="FileText" :size="14" :color="cv('text-tertiary')" :style="{ flexShrink: 0 }" />
                {{ doc.doc_name }}
              </span>
              <span :style="tdStyle({ flex: '1', textAlign: 'center' })">
                <span :style="typeBadgeStyle(doc.type)">{{ doc.type === 1 ? $t('uiTextMain.k38') : $t('uiTextMain.k39') }}</span>
              </span>
              <span :style="tdStyle({ flex: '1.5', textAlign: 'center' })">
                <span :style="vectorBadgeStyle(doc.vector_status)">{{ vectorStatusLabel(doc.vector_status) }}</span>
              </span>
              <span :style="tdStyle({ flex: '1.5', textAlign: 'right' })">
                <span :style="timeTextStyle">{{ doc.update_time ? formatTime(doc.update_time) : '-' }}</span>
              </span>
            </div>
          </div>
        </div>
      </div>

      <!-- Tab 2: 召回测试 -->
      <div v-if="activeTab === 'retrieval'">
        <div :style="retrievalSectionStyle">
          <div :style="retrievalInputRowStyle">
            <ApInput
              v-model="retrievalQuery"
              :placeholder="$t('aiView.k219')"
              :style="{ flex: 1 }"
              @keydown.enter="handleTestRetrieval"
            />
            <ApInput
              v-model="topKStr"
              type="number"
              placeholder="Top K"
              :style="{ width: '100px' }"
            />
            <ApButton
              variant="primary"
              size="small"
              :loading="retrievalLoading"
              :disabled="!retrievalQuery.trim()"
              @click="handleTestRetrieval"
            >
              <ApIcon name="Search" :size="16" /> {{ $t('aiView.k193') }}
            </ApButton>
          </div>

          <!-- 召回结果 -->
          <div v-if="retrievalResults !== null" :style="retrievalResultsStyle">
            <div :style="retrievalMetaStyle">
              <span>{{ $t('aiView.k194') }} <strong>{{ retrievalMethod }}</strong></span>
              <span>{{ $t('aiView.k195') }} <strong>{{ retrievalResults.length }}</strong> {{ $t('aiView.k196') }}</span>
            </div>
            <div v-if="retrievalResults.length === 0" :style="emptyHintStyle">
              {{ $t('aiView.k197') }}
            </div>
            <div
              v-for="r in retrievalResults"
              :key="r.index"
              :style="resultItemStyle"
            >
              <div :style="resultHeaderStyle">
                <span :style="resultIndexStyle">#{{ r.index }}</span>
                <span :style="resultNameStyle">{{ r.document_name }}</span>
                <span :style="resultScoreStyle(r.score)">
                  {{ r.score > 0 ? r.score.toFixed(3) : 'N/A' }}
                </span>
              </div>
              <div v-if="r.snippet" :style="resultSnippetStyle">
                {{ r.snippet.length > 200 ? r.snippet.slice(0, 200) + '...' : r.snippet }}
              </div>
              <div v-if="r.heading_path" :style="resultPathStyle">
                {{ r.heading_path }}
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Tab 3: 统计信息 -->
      <div v-if="activeTab === 'stats'">
        <div v-if="statsLoading" :style="loadingStyle">
          <span>{{ $t('aiView.k198') }}</span>
        </div>
        <div v-else-if="statsData">
          <div :style="statsDetailGridStyle">
            <div :style="statsDetailCardStyle">
              <div :style="statsDetailTitleStyle">{{ $t('aiView.k199') }}</div>
              <div :style="statsDetailRowStyle">
                <span :style="statsLabelStyle">{{ $t('aiView.k200') }}</span>
                <span :style="statsValueStyle()">{{ statsData.total_docs }}</span>
              </div>
              <div :style="statsDetailRowStyle">
                <span :style="statsLabelStyle">{{ $t('aiView.k201') }}</span>
                <span :style="statsValueStyle()">{{ statsData.total_pages }}</span>
              </div>
              <div :style="statsDetailRowStyle">
                <span :style="statsLabelStyle">{{ $t('aiView.k202') }}</span>
                <span :style="statsValueStyle()">{{ statsData.total_dirs }}</span>
              </div>
            </div>
            <div :style="statsDetailCardStyle">
              <div :style="statsDetailTitleStyle">{{ $t('aiView.k203') }}</div>
              <div :style="statsDetailRowStyle">
                <span :style="statsLabelStyle">{{ $t('aiView.k187') }}</span>
                <span :style="statsValueStyle(cv('green'))">{{ statsData.vectorized_count }}</span>
              </div>
              <div :style="statsDetailRowStyle">
                <span :style="statsLabelStyle">{{ $t('aiView.k204') }}</span>
                <span :style="statsValueStyle(cv('orange'))">{{ statsData.pending_vectors }}</span>
              </div>
              <div :style="statsDetailRowStyle">
                <span :style="statsLabelStyle">{{ $t('aiView.failed') }}</span>
                <span :style="statsValueStyle(cv('red'))">{{ statsData.failed_vectors }}</span>
              </div>
            </div>
            <div :style="statsDetailCardStyle">
              <div :style="statsDetailTitleStyle">{{ $t('aiView.k206') }}</div>
              <div :style="progressContainerStyle">
                <ApProgress
                  :value="statsData.vector_completion"
                  :label="$t('aiView.k218')"
                  :tone="toneForScore(statsData.vector_completion / 100)"
                  :style="{ flex: 1 }"
                />
                <span :style="progressLabelStyle">{{ statsData.vector_completion.toFixed(1) }}%</span>
              </div>
              <div v-if="statsData.last_vector_sync" :style="statsDetailRowStyle">
                <span :style="statsLabelStyle">{{ $t('aiView.k207') }}</span>
                <span :style="statsValueStyle()">{{ formatTime(statsData.last_vector_sync) }}</span>
              </div>
            </div>
          </div>
        </div>
      </div>

      <!-- Tab 4: 分析结果 -->
      <div v-if="activeTab === 'analysis'">
        <div v-if="analysisLoading" :style="loadingStyle">
          <span>{{ $t('aiView.k208') }}</span>
        </div>
        <div v-else>
          <!-- 质量统计 -->
          <div v-if="analysisStats" :style="docStatsRowStyle">
            <div :style="docStatItemStyle">
              <span :style="docStatLabelStyle">{{ $t('aiView.k209') }}</span>
              <span :style="docStatValueStyle(analysisStats.avg_score >= 60 ? cv('green') : cv('orange'))">{{ (analysisStats.avg_score || 0).toFixed(1) }}</span>
            </div>
            <div :style="docStatItemStyle">
              <span :style="docStatLabelStyle">{{ $t('aiView.k210') }}</span>
              <span :style="docStatValueStyle()">{{ analysisStats.total }}</span>
            </div>
            <div :style="docStatItemStyle">
              <span :style="docStatLabelStyle">{{ $t('aiView.k211') }}</span>
              <span :style="docStatValueStyle(cv('red'))">{{ analysisStats.low_quality }}</span>
            </div>
            <div :style="docStatItemStyle">
              <span :style="docStatLabelStyle">{{ $t('aiView.k212') }}</span>
              <span :style="docStatValueStyle(cv('green'))">{{ analysisStats.high_quality }}</span>
            </div>
          </div>
          <!-- 分类 -->
          <div v-if="analysisCategories.length > 0" :style="{ marginBottom: AppleSpace.lg }">
            <div :style="statsDetailTitleStyle">{{ $t('aiView.k213') }}</div>
            <div :style="{ display: 'flex', flexWrap: 'wrap' as const, gap: AppleSpace.sm }">
              <span
                v-for="cat in analysisCategories"
                :key="cat.name"
                :style="{ ...typeBadgeStyle(1), padding: `2px ${AppleSpace.md}` }"
              >
                {{ cat.name }} ({{ cat.count }})
              </span>
            </div>
          </div>
          <!-- 知识缺口 -->
          <div v-if="analysisGaps.length > 0" :style="{ marginBottom: AppleSpace.lg }">
            <div :style="statsDetailTitleStyle">{{ $t('aiView.k214') }}</div>
            <div :style="{ display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.sm }">
              <div
                v-for="(gap, idx) in analysisGaps"
                :key="idx"
                :style="{ padding: AppleSpace.md, backgroundColor: cv('bg-secondary'), borderRadius: AppleRadius.medium, border: `1px solid ${cv('border-light')}` }"
              >
                <div :style="{ fontSize: AppleType.bodyPrimary.size, fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary') }">
                  {{ gap.category }}
                </div>
                <div :style="{ fontSize: AppleType.legalMeta.size, color: cv('text-secondary'), marginTop: AppleSpace.xxs }">
                  {{ gap.description }}
                </div>
                <div v-if="gap.suggested_title" :style="{ fontSize: AppleType.microUI.size, color: cv('text-tertiary'), marginTop: AppleSpace.xxs }">
                  {{ $t('uiTextMain.k40', { p1: gap.suggested_title }) }}
                </div>
              </div>
            </div>
          </div>
          <!-- 文档分析列表 -->
          <div v-if="analysisResults.length > 0">
            <div :style="statsDetailTitleStyle">{{ $t('aiView.k215') }}</div>
            <div :style="{ display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.sm, marginTop: AppleSpace.md }">
              <div
                v-for="r in analysisResults"
                :key="r.id"
                :style="{ padding: AppleSpace.md, backgroundColor: cv('bg-secondary'), borderRadius: AppleRadius.medium, border: `1px solid ${cv('border-light')}` }"
              >
                <div :style="{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: AppleSpace.xs }">
                  <span :style="{ fontSize: AppleType.bodyPrimary.size, fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary') }">
                    {{ $t('uiTextMain.k41', { p1: r.doc_id }) }}
                  </span>
                  <span :style="{ fontSize: AppleType.legalMeta.size, color: r.quality_score >= 60 ? cv('green') : cv('orange') }">
                    {{ $t('uiTextMain.k42', { p1: (r.quality_score || 0).toFixed(1) }) }}
                  </span>
                </div>
                <div v-if="r.category" :style="{ fontSize: AppleType.legalMeta.size, color: cv('text-secondary'), marginBottom: AppleSpace.xxs }">
                  {{ $t('uiTextMain.k43', { p1: r.category }) }}
                </div>
                <div v-if="r.summary" :style="{ fontSize: AppleType.legalMeta.size, color: cv('text-secondary'), lineHeight: AppleLineHeight.loose }">
                  {{ r.summary.length > 120 ? r.summary.slice(0, 120) + '...' : r.summary }}
                </div>
                <div v-if="r.tags && r.tags.length" :style="{ display: 'flex', flexWrap: 'wrap' as const, gap: AppleSpace.xs, marginTop: AppleSpace.xs }">
                  <span
                    v-for="tag in r.tags"
                    :key="tag"
                    :style="{ fontSize: AppleType.microUI.size, padding: '1px 6px', borderRadius: AppleRadius.micro, backgroundColor: `${cv('blue-bg-subtle')}`, color: cv('blue') }"
                  >
                    {{ tag }}
                  </span>
                </div>
              </div>
            </div>
          </div>
          <ApPagination
            v-if="analysisTotal > analysisPageSize"
            :total="analysisTotal"
            :page-size="analysisPageSize"
            :page="analysisPage"
            @change="(p: number) => { analysisPage = p; if (selectedId !== null) fetchAnalysis(selectedId); }"
          />
          <ApEmpty v-else-if="!analysisStats && !analysisCategories.length && !analysisGaps.length" :icon="'Zap'" :title="$t('aiView.k222')" :description="$t('vAiUi.k28')" />
        </div>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted, watch } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApPagination from '@/components/apple/ApPagination.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import { keyboardClick } from '@/composables/ui/useKeyboardClick';


import { cv, AppleFontWeight, AppleLineHeight, AppleRadius, AppleSpace, AppleType, AppleTransition, AppleGridFloor, appleCardGrid } from '@/utils/core/apple-tokens';
import { toneForScore } from '@/utils/core/progress-tone';
import { aiSubPageStyle, aiSubCardNameStyle as detailTitleStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle } from '@/styles/ai/ai-page-styles';
import { crudStatsRowStyle } from '@/styles/layout/page-styles';
import { useBreakpoint } from '@/utils/responsive';
import { logger } from '@/utils/logger';
import { formatTime } from '@/utils/format/format-time';
import { useUIStore } from '@/stores/app/ui';
import { listKnowledgeBases, listChunks, testRetrieval, getKnowledgeBaseStats, type KnowledgeBase, type DocChunkInfo, type RetrievalResult, type KnowledgeBaseStats } from '@/api/modules/ai/ai-knowledge-base';
import { getAnalysisResults, getQualityStats, getCategories, getKnowledgeGaps, type AIAnalysisResult, type QualityStats, type CategoryNode, type KnowledgeGap } from '@/api/modules/ai/ai-analysis-result';

const ui = useUIStore();
const { isMobile } = useBreakpoint();

// === State ===
const loading = ref(true);
const bases = ref<KnowledgeBase[]>([]);
const selectedId = ref<number | null>(null);
const activeTab = ref<string | number>('docs');

// Docs tab
const docsLoading = ref(false);
const docList = ref<DocChunkInfo[]>([]);
const chunkInfo = ref<{ total_docs: number; vectorized_count: number }>({ total_docs: 0, vectorized_count: 0 });

// Retrieval tab
const retrievalQuery = ref('');
const topKStr = ref('5');
const retrievalLoading = ref(false);
const retrievalResults = ref<RetrievalResult[] | null>(null);
const retrievalMethod = ref('');

// Stats tab
const statsLoading = ref(false);
const statsData = ref<KnowledgeBaseStats | null>(null);

// Analysis tab
const analysisLoading = ref(false);
const analysisResults = ref<AIAnalysisResult[]>([]);
const analysisStats = ref<QualityStats | null>(null);
const analysisCategories = ref<CategoryNode[]>([]);
const analysisGaps = ref<KnowledgeGap[]>([]);
const analysisPage = ref(1);
const analysisPageSize = ref(10);
const analysisTotal = ref(0);

// === Computed ===
const selectedBase = computed(() => bases.value.find(b => b.id === selectedId.value) ?? null);
const totalDocCount = computed(() => bases.value.reduce((sum, b) => sum + (b.doc_count || 0), 0));
const totalVectorized = ref(0);
const completionRate = computed(() => {
  if (totalDocCount.value === 0) return 0;
  return Math.round(totalVectorized.value / totalDocCount.value * 100);
});
const docVectorRate = computed(() => {
  if (chunkInfo.value.total_docs === 0) return 0;
  return Math.round(chunkInfo.value.vectorized_count / chunkInfo.value.total_docs * 100);
});

const tabOptions = [
  { label: t('aiView.k223'), value: 'docs' },
  { label: t('aiView.k224'), value: 'retrieval' },
  { label: t('aiView.k225'), value: 'stats' },
  { label: t('aiView.k11'), value: 'analysis' },
];

// === Methods ===
async function fetchBases() {
  loading.value = true;
  try {
    const res = await listKnowledgeBases();
    bases.value = res.knowledge_bases || [];
    // 计算总向量化数（并行请求，避免 N+1 串行）
    const statsResults = await Promise.all(
      bases.value.map((b) => getKnowledgeBaseStats(b.id).catch(() => null))
    );
    totalVectorized.value = statsResults.reduce((sum, st) => sum + (st?.vectorized_count || 0), 0);
  } catch (e) {
    logger.error('Failed to load knowledge bases:', e);
    ui.showToast(t('aiView.k226'), 'error');
  } finally {
    loading.value = false;
  }
}

async function selectBase(kb: KnowledgeBase) {
  selectedId.value = kb.id;
  activeTab.value = 'docs';
  retrievalResults.value = null;
  statsData.value = null;
  await fetchDocs(kb.id);
}

async function fetchDocs(spaceId: number) {
  docsLoading.value = true;
  try {
    const res = await listChunks(spaceId);
    docList.value = res.documents || [];
    chunkInfo.value = { total_docs: res.total_docs, vectorized_count: res.vectorized_count };
  } catch (e) {
    logger.error('Failed to load chunks:', e);
    ui.showToast(t('aiView.k227'), 'error');
  } finally {
    docsLoading.value = false;
  }
}

async function handleTestRetrieval() {
  if (!selectedId.value || !retrievalQuery.value.trim()) return;
  retrievalLoading.value = true;
  retrievalResults.value = null;
  try {
    const topK = Math.max(1, Math.min(20, Number(topKStr.value) || 5));
    const res = await testRetrieval(selectedId.value, { query: retrievalQuery.value, top_k: topK });
    retrievalResults.value = res.results || [];
    retrievalMethod.value = res.method || 'unknown';
  } catch (e) {
    logger.error('Retrieval test failed:', e);
    ui.showToast(t('aiView.k228'), 'error');
  } finally {
    retrievalLoading.value = false;
  }
}

async function fetchStats(spaceId: number) {
  statsLoading.value = true;
  try {
    statsData.value = await getKnowledgeBaseStats(spaceId);
  } catch (e) {
    logger.error('Failed to load stats:', e);
    ui.showToast(t('aiView.k229'), 'error');
  } finally {
    statsLoading.value = false;
  }
}

async function fetchAnalysis(spaceId: number) {
  analysisLoading.value = true;
  try {
    const [results, stats, categories, gaps] = await Promise.all([
      getAnalysisResults(spaceId, analysisPage.value, analysisPageSize.value).catch(() => null),
      getQualityStats(spaceId).catch(() => null),
      getCategories(spaceId).catch(() => null),
      getKnowledgeGaps(spaceId).catch(() => null),
    ]);
    if (results) {
      analysisResults.value = results.list || [];
      analysisTotal.value = results.total || 0;
    }
    analysisStats.value = stats;
    analysisCategories.value = categories || [];
    analysisGaps.value = gaps || [];
  } catch (e) {
    logger.error('Failed to load analysis:', e);
  } finally {
    analysisLoading.value = false;
  }
}

function vectorStatusLabel(status: string): string {
  const map: Record<string, string> = {
    completed: t('aiView.k40'),
    pending: t('uiTextMain.k44'),
    failed: t('aiView.failed'),
    none: t('uiTextMain.k45'),
  };
  return map[status] || status;
}

// Watch tab changes to load data on demand
watch(activeTab, (tab) => {
  if (!selectedId.value) return;
  if (tab === 'docs') fetchDocs(selectedId.value);
  if (tab === 'stats') fetchStats(selectedId.value);
  if (tab === 'analysis') fetchAnalysis(selectedId.value);
});

watch(selectedId, (id) => {
  if (id === null) {
    retrievalResults.value = null;
    statsData.value = null;
  }
});

onMounted(() => fetchBases());

// === Styles ===

const sectionTitleStyle = aiSubSectionTitleStyle;

const statsGridStyle = crudStatsRowStyle('compact');


const gridStyle = appleCardGrid('entity', AppleSpace.sm);

const cardStyle = (selected: boolean) => ({
  padding: AppleSpace.md,
  backgroundColor: selected ? `${cv('blue-bg-subtle')}` : cv('surface-field'),
  borderRadius: AppleRadius.medium,
  border: `1.5px solid ${selected ? cv('blue') : cv('border-light')}`,
  cursor: 'pointer',
  transition: AppleTransition.default,
});

const cardHeaderRowStyle = {
  display: 'flex',
  alignItems: 'flex-start',
  justifyContent: 'space-between',
  marginBottom: AppleSpace.xs,
};

const cardTitleWrapStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
  minWidth: 0,
};

const cardNameStyle = {
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const cardDescStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const statusDotStyle = (active: boolean) => ({
  width: AppleSpace.sm,
  height: AppleSpace.sm,
  borderRadius: AppleRadius.circle,
  backgroundColor: active ? cv('green') : cv('text-tertiary'),
  flexShrink: 0,
  marginTop: '3px',
});

const cardMetricsStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
  marginBottom: AppleSpace.xs,
};

const metricItemStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
};

const cardFooterStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'flex-end',
  paddingTop: AppleSpace.xs,
  borderTop: `1px solid ${cv('border-light')}`,
};

// Detail panel（命名避开共享层 page-styles.detailPanelStyle —— 两者底色不同，不能同名遮蔽）
const kbDetailPanelStyle = {
  padding: AppleSpace.lg,
  backgroundColor: cv('surface-field'),
  borderRadius: AppleRadius.card,
  border: `1px solid ${cv('border-light')}`,
  boxShadow: cv('shadow-card'),
};

const detailHeaderStyle = {
  display: 'flex',
  alignItems: 'flex-start',
  justifyContent: 'space-between',
  marginBottom: AppleSpace.md,
};


const detailSubtitleStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  marginTop: AppleSpace.xxs,
};

const tabContainerStyle = {
  marginBottom: AppleSpace.md,
};

// Docs tab：grid 自适应折行（原先 flex 不换行，窄屏溢出）
const docStatsRowStyle = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fit, ${AppleGridFloor.stat})`,
  gap: AppleSpace.sm,
  marginBottom: AppleSpace.md,
};

const docStatItemStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  backgroundColor: cv('bg-secondary'),
  borderRadius: AppleRadius.medium,
  border: `1px solid ${cv('border-light')}`,
  minWidth: '80px',
};

const docStatLabelStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};

const docStatValueStyle = (color?: string) => ({
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: color || cv('text-primary'),
});

const tableContainerStyle = {
  border: `1px solid ${cv('border-light')}`,
  borderRadius: AppleRadius.medium,
  overflow: 'hidden',
};

const tableHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  backgroundColor: cv('bg-secondary'),
  borderBottom: `1px solid ${cv('border-light')}`,
};

const tableRowStyle = {
  display: 'flex',
  alignItems: 'center',
  padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  borderBottom: `1px solid ${cv('border-light')}`,
};

const thStyle = (extra: Record<string, string>) => ({
  flex: extra.flex || '1',
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-secondary'),
  textAlign: (extra.textAlign || 'left') as 'left' | 'center' | 'right',
});

const tdStyle = (extra: Record<string, string>) => ({
  flex: extra.flex || '1',
  fontSize: AppleType.legalMeta.size,
  color: cv('text-primary'),
  textAlign: (extra.textAlign || 'left') as 'left' | 'center' | 'right',
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  justifyContent: extra.textAlign === 'center' ? 'center' : extra.textAlign === 'right' ? 'flex-end' : 'flex-start',
});

const typeBadgeStyle = (type: number) => ({
  fontSize: AppleType.microUI.size,
  padding: `1px ${AppleSpace.xs}`,
  borderRadius: AppleRadius.micro,
  backgroundColor: type === 1 ? `${cv('blue-bg-subtle')}` : `${cv('orange-bg-subtle')}`,
  color: type === 1 ? cv('blue') : cv('orange'),
});

const vectorBadgeStyle = (status: string) => ({
  fontSize: AppleType.microUI.size,
  padding: `1px ${AppleSpace.xs}`,
  borderRadius: AppleRadius.micro,
  backgroundColor: status === 'completed' ? `${cv('green-bg-subtle')}` : status === 'failed' ? `${cv('red-bg-subtle')}` : status === 'pending' ? `${cv('orange-bg-subtle')}` : cv('bg-tertiary'),
  color: status === 'completed' ? cv('green') : status === 'failed' ? cv('red') : status === 'pending' ? cv('orange') : cv('text-tertiary'),
});

const timeTextStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};

// Retrieval tab
const retrievalSectionStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.md,
};

const retrievalInputRowStyle = computed(() => ({ display: 'flex', gap: AppleSpace.xs, alignItems: 'flex-end' as const, flexWrap: isMobile.value ? 'wrap' as const : 'nowrap' as const }));

const retrievalResultsStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
};

const retrievalMetaStyle = {
  display: 'flex',
  gap: AppleSpace.md,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
  paddingBottom: AppleSpace.xs,
  borderBottom: `1px solid ${cv('border-light')}`,
};

const resultItemStyle = {
  padding: AppleSpace.sm,
  backgroundColor: cv('bg-secondary'),
  borderRadius: AppleRadius.medium,
  border: `1px solid ${cv('border-light')}`,
};

const resultHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  marginBottom: AppleSpace.xxs,
};

const resultIndexStyle = {
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('blue'),
  minWidth: '24px',
};

const resultNameStyle = {
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
  flex: 1,
};

const resultScoreStyle = (score: number) => ({
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: score > 0.7 ? cv('green') : score > 0.4 ? cv('orange') : cv('text-tertiary'),
});

const resultSnippetStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
  lineHeight: AppleLineHeight.loose,
  marginBottom: AppleSpace.xxs,
};

const resultPathStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};

// Stats tab
const statsDetailGridStyle = computed(() => ({
  display: 'grid',
  gridTemplateColumns: isMobile.value ? '1fr' : `repeat(auto-fill, ${AppleGridFloor.panel})`,
  gridAutoRows: '1fr', alignContent: 'start',
  gap: AppleSpace.sm,
}));

const statsDetailCardStyle = {
  padding: AppleSpace.md,
  backgroundColor: cv('bg-secondary'),
  borderRadius: AppleRadius.medium,
  border: `1px solid ${cv('border-light')}`,
};

const statsDetailTitleStyle = {
  fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  marginBottom: AppleSpace.sm,
};

const statsDetailRowStyle = {
  display: 'flex',
  justifyContent: 'space-between',
  alignItems: 'center',
  padding: `${AppleSpace.xxs} 0`,
  borderBottom: `1px solid ${cv('border-light')}`,
};

const statsLabelStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
};

const statsValueStyle = (color?: string) => ({
  fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: color || cv('text-primary'),
});

const progressContainerStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  marginBottom: AppleSpace.sm,
};

const progressLabelStyle = {
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  minWidth: '44px',
  textAlign: 'right' as const,
};

const loadingStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  padding: AppleSpace.lg,
  color: cv('text-tertiary'),
  fontSize: AppleType.legalMeta.size,
};

const emptyHintStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
  textAlign: 'center' as const,
  padding: AppleSpace.lg,
};
</script>
