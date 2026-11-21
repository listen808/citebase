<template>
  <div :style="aiSubPageStyle">
    <ApSegmentedControl v-model="activeTab" :options="tabs" />

    <!-- Tab 1: RAG 策略配置 -->
    <template v-if="activeTab === 'rag'">
      <ApSkeleton v-if="ragLoading" :lines="4" />
      <ApCard v-else size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="Search" :size="16" :color="cv('blue')" />
            <span :style="sectionTitleStyle">{{ $t('aiVec.k1') }}</span>
          </div>
        </template>

        <div :style="formGridStyle">
          <ApFormItem :label="`向量检索权重 (${ragConfig.vector_weight})`">
            <ApSlider v-model="ragConfig.vector_weight" :min="0" :max="1" :step="0.1" :aria-label="$t('aiVec.k31')" />
          </ApFormItem>
          <ApFormItem :label="`关键词权重 (${ragConfig.keyword_weight})`">
            <ApSlider v-model="ragConfig.keyword_weight" :min="0" :max="1" :step="0.1" :aria-label="$t('aiVec.k32')" />
          </ApFormItem>
          <ApInput :model-value="String(ragConfig.similarity_threshold)" type="number" :label="$t('aiVec.k17')" placeholder="0.6" @update:model-value="(v) => ragConfig.similarity_threshold = Number(v)" />
          <ApInput :model-value="String(ragConfig.search_top_k)" type="number" :label="$t('aiVec.k18')" placeholder="10" @update:model-value="(v) => ragConfig.search_top_k = Number(v)" />
          <ApInput :model-value="String(ragConfig.max_context_chars)" type="number" :label="$t('aiVec.k19')" placeholder="8000" @update:model-value="(v) => ragConfig.max_context_chars = Number(v)" />
        </div>

        <div :style="toggleGridStyle">
          <div :style="toggleLabelStyle">
            <ApSwitch
              :model-value="ragConfig.enable_rerank"
              :label="$t('aiVec.k20')"
              @update:model-value="(v) => ragConfig.enable_rerank = v"
            />
          </div>
          <div :style="toggleLabelStyle">
            <ApSwitch
              :model-value="ragConfig.enable_query_rewrite"
              :label="$t('aiVec.k21')"
              @update:model-value="(v) => ragConfig.enable_query_rewrite = v"
            />
          </div>
          <div :style="toggleLabelStyle">
            <ApSwitch
              :model-value="ragConfig.enable_sub_question"
              :label="$t('aiVec.k22')"
              @update:model-value="(v) => ragConfig.enable_sub_question = v"
            />
          </div>
        </div>

        <div :style="actionRowStyle">
          <ApButton variant="primary" size="small" :loading="saving" @click="handleSaveRAG">
            <ApIcon name="Save" :size="16" /> {{ $t('aiVec.k2') }}
          </ApButton>
          <ApButton variant="secondary" size="small" :loading="testing" @click="handleTest">
            <ApIcon name="Zap" :size="16" /> {{ $t('aiVec.k3') }}
          </ApButton>
        </div>

        <div v-if="testResult" :style="testResultStyle">
          <div :style="resultTitleStyle">{{ $t('aiVec.k4') }}</div>
          <pre :style="resultCodeStyle">{{ JSON.stringify(testResult, null, 2) }}</pre>
        </div>
      </ApCard>
    </template>

    <!-- Tab 2: 向量索引管理 -->
    <template v-if="activeTab === 'vector'">
      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="Database" :size="16" :color="cv('purple')" />
            <span :style="sectionTitleStyle">{{ $t('aiVec.k5') }}</span>
            <ApButton variant="ghost" size="small" :loading="vectorLoading" @click="fetchVectorStats">
              <ApIcon name="RefreshCw" :size="16" /> {{ $t('aiVec.refresh') }}
            </ApButton>
          </div>
        </template>
        <ApEmpty v-if="!vectorStats.embedderReady" :icon="'Database'" :title="$t('aiVec.k29')" :description="$t('vAiUi.k60')" />
        <ApEmpty v-else-if="vectorStats.totalDocs === 0" :icon="'Database'" :title="$t('aiVec.k30')" :description="$t('vAiUi.k61')" />
        <div v-else :style="statsRowStyle">
          <ApStatCard :label="$t('aiVec.k23')" :value="String(vectorStats.totalDocs)" :icon="'Database'" :color="cv('blue')" size="compact" />
          <ApStatCard :label="$t('aiVec.k24')" :value="String(vectorStats.totalVectors)" :icon="'Layers'" :color="cv('green')" size="compact" />
          <ApStatCard :label="$t('aiVec.k25')" :value="vectorStats.lastSync" :icon="'Clock'" :color="cv('orange')" size="compact" />
          <ApStatCard :label="$t('aiVec.k26')" :value="vectorStats.status" :icon="'Activity'" :color="cv('purple')" size="compact" />
        </div>
      </ApCard>

      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="Wrench" :size="16" :color="cv('orange')" />
            <span :style="sectionTitleStyle">{{ $t('aiVec.k7') }}</span>
          </div>
        </template>
        <div :style="actionGridStyle">
          <div :style="actionCardStyle">
            <div :style="actionTitleStyle">{{ $t('aiVec.k8') }}</div>
            <div :style="actionDescStyle">{{ $t('aiVec.k9') }}</div>
            <ApButton variant="primary" size="small" :loading="reindexLoading === 'all'" @click="rebuildAll">
              <ApIcon name="RefreshCw" :size="16" /> {{ $t('aiVec.k10') }}
            </ApButton>
          </div>
          <div :style="actionCardStyle">
            <div :style="actionTitleStyle">{{ $t('aiVec.k11') }}</div>
            <div :style="actionDescStyle">{{ $t('aiVec.k12') }}</div>
            <div :style="spaceSelectRowStyle">
              <ApSelect :model-value="reindexSpaceId ?? 0" :options="spaceOptions" :placeholder="$t('aiVec.k27')" @update:model-value="(v) => reindexSpaceId = v as number" />
              <ApButton variant="secondary" size="small" :disabled="!reindexSpaceId" :loading="reindexLoading === 'space'" @click="rebuildBySpace">
                <ApIcon name="RefreshCw" :size="16" /> {{ $t('aiVec.k13') }}
              </ApButton>
            </div>
          </div>
          <div :style="actionCardStyle">
            <div :style="actionTitleStyle">{{ $t('aiVec.k14') }}</div>
            <div :style="actionDescStyle">{{ $t('aiVec.k15') }}</div>
            <div :style="spaceSelectRowStyle">
              <ApInput :model-value="reindexDocId ? String(reindexDocId) : ''" type="number" :placeholder="$t('aiVec.k28')" @update:model-value="(v) => reindexDocId = v ? Number(v) : undefined" />
              <ApButton variant="secondary" size="small" :disabled="!reindexDocId" :loading="reindexLoading === 'doc'" @click="rebuildByDoc">
                <ApIcon name="RefreshCw" :size="16" /> {{ $t('aiVec.k13') }}
              </ApButton>
            </div>
          </div>
        </div>
      </ApCard>

      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="Settings" :size="16" :color="cv('purple')" />
            <span :style="sectionTitleStyle">{{ $t('aiVec.k16') }}</span>
          </div>
        </template>
        <div :style="configContentStyle">
          <div v-for="(item, idx) in vectorConfigItems" :key="idx" :style="configRowItemStyle(idx)">
            <span :style="configLabelStyle">{{ item.label }}</span>
            <span :style="configValueStyle">{{ item.value }}</span>
          </div>
        </div>
      </ApCard>
    </template>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { logger } from '@/utils/logger';
import { ref, shallowRef, computed, onMounted } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import { cv, AppleSpace, AppleRadius, AppleType, AppleFontWeight } from '@/utils/core/apple-tokens';
import { useBreakpoint } from '@/utils/responsive';
import { aiSubPageStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle, aiSubFormGridStyle, aiSubActionRowStyle, aiSubToggleLabelStyle, aiSubBodyStyle, crudStatsRowStyle, aiActionGridStyle, aiActionCardStyle, aiActionTitleStyle, aiActionDescStyle, aiResultTitleStyle as resultTitleStyle } from '@/styles/ai/ai-page-styles';
import { getRAGConfig, updateRAGConfig, testRAGRetrieval, getVectorStats, reindexAllVectors, reindexSpaceVectors, reindexDocVector } from '@/api/modules/ai/ai-config';
import type { RAGConfig } from '@/types/ai';
import { useSpaceStore } from '@/stores/workspace/space';
import { useUIStore } from '@/stores/app/ui';

const spaceStore = useSpaceStore();
const ui = useUIStore();
const { isMobile } = useBreakpoint();

const tabs = [
  { label: t('aiVec.k33'), value: 'rag' },
  { label: t('aiVec.k34'), value: 'vector' },
];
const activeTab = ref('rag');

// ===== RAG Config =====
const saving = ref(false);
const testing = ref(false);
const ragLoading = ref(true);
const testResult = ref<unknown>(null);

const ragConfig = ref<RAGConfig>({
  vector_weight: 0.7,
  keyword_weight: 0.3,
  similarity_threshold: 0.6,
  search_top_k: 10,
  max_context_chars: 8000,
  enable_rerank: false,
  enable_query_rewrite: true,
  enable_sub_question: false,
});

async function fetchRAGConfig() {
  ragLoading.value = true;
  try {
    const res = await getRAGConfig();
    if (res.config) Object.assign(ragConfig.value, res.config);
  } catch (e) {
    logger.error('Failed to fetch RAG config:', e);
    ui.showToast(t('aiVec.k40'), 'error');
  } finally {
    ragLoading.value = false;
  }
}

async function handleSaveRAG() {
  saving.value = true;
  try {
    await updateRAGConfig(ragConfig.value);
    ui.showToast(t('aiVec.k41'), 'success');
  } catch (e) {
    logger.error('Failed to save RAG config:', e);
    ui.showToast(t('aiVec.k42'), 'error');
  }
  saving.value = false;
}

async function handleTest() {
  testing.value = true;
  testResult.value = null;
  try {
    const res = await testRAGRetrieval({ question: '测试检索效果', config: ragConfig.value });
    testResult.value = res.debug;
  } catch (e) {
    logger.error('Failed to test RAG retrieval:', e);
    ui.showToast(t('aiVec.k43'), 'error');
  }
  testing.value = false;
}

// ===== Vector Config =====
const vectorLoading = shallowRef(false);
const vectorStats = ref({ enabled: false, embedderReady: false, totalDocs: 0, totalVectors: 0, lastSync: '-', status: '未启用', embeddingModel: '', vectorStoreType: 'bolt' });
const reindexLoading = ref<string | null>(null);
const reindexSpaceId = ref<number | undefined>(undefined);
const reindexDocId = ref<number | undefined>(undefined);

const spaceOptions = computed(() => [
  { label: t('aiVec.k35'), value: 0 },
  ...(spaceStore.mySpaces || []).map(s => ({ label: s.name, value: s.id })),
]);

const vectorStoreTypeLabel: Record<string, string> = { bolt: 'BoltDB + HNSW', chroma: 'Chroma', qdrant: 'Qdrant' };

const vectorConfigItems = computed(() => [
  { label: t('aiVec.k36'), value: vectorStoreTypeLabel[vectorStats.value.vectorStoreType] || vectorStats.value.vectorStoreType },
  { label: t('aiVec.k37'), value: vectorStats.value.embeddingModel || t('fallback.k83') },
  { label: t('aiVec.k38'), value: '余弦相似度' },
  { label: t('aiVec.k39'), value: String(vectorStats.value.totalDocs) + ' 个' },
  { label: t('aiVec.k25'), value: vectorStats.value.status },
]);

async function fetchVectorStats() {
  vectorLoading.value = true;
  try {
    const res = await getVectorStats() as Record<string, unknown>;
    vectorStats.value = {
      enabled: Boolean(res.enabled),
      embedderReady: Boolean(res.embedder_ready),
      totalDocs: Number(res.doc_count || res.totalDocs || 0),
      totalVectors: Number((res.tasks as Record<string, unknown> | undefined)?.done || res.total_vectors || res.vector_count || 0),
      lastSync: (res.last_sync as string | undefined) || (res.syncer_ready ? '已就绪' : '未知'),
      status: res.embedder_ready ? (res.syncer_ready ? '运行中' : '未就绪') : '未启用',
      embeddingModel: String(res.embedding_model || ''),
      vectorStoreType: String(res.vector_store_type || 'bolt'),
    };
  } catch (e: unknown) {
    ui.showToast((e as { message?: string }).message || t('fallback.k98'), 'error');
  }
  vectorLoading.value = false;
}

async function rebuildAll() {
  reindexLoading.value = 'all';
  try {
    const res = await reindexAllVectors();
    ui.showToast(res.message || t('fallback.k12'), 'success');
    await fetchVectorStats();
  } catch (e: unknown) {
    ui.showToast((e as { message?: string }).message || t('fallback.k107'), 'error');
  }
  reindexLoading.value = null;
}

async function rebuildBySpace() {
  if (!reindexSpaceId.value) return;
  reindexLoading.value = 'space';
  try {
    const res = await reindexSpaceVectors(reindexSpaceId.value);
    ui.showToast(res.message || `空间 #${reindexSpaceId.value} 重建已启动`, 'success');
    await fetchVectorStats();
  } catch (e: unknown) {
    ui.showToast((e as { message?: string }).message || t('fallback.k107'), 'error');
  }
  reindexLoading.value = null;
}

async function rebuildByDoc() {
  if (!reindexDocId.value) return;
  reindexLoading.value = 'doc';
  try {
    const res = await reindexDocVector(reindexDocId.value);
    ui.showToast(res.message || `文档 #${reindexDocId.value} 重建已启动`, 'success');
    await fetchVectorStats();
  } catch (e: unknown) {
    ui.showToast((e as { message?: string }).message || t('fallback.k107'), 'error');
  }
  reindexLoading.value = null;
}

onMounted(async () => {
  await spaceStore.loadMySpaces();
  fetchRAGConfig();
  fetchVectorStats();
});

// ===== Styles =====
const sectionTitleStyle = aiSubSectionTitleStyle;
const formGridStyle = computed(() => isMobile.value ? { ...aiSubFormGridStyle, gridTemplateColumns: '1fr', gap: AppleSpace.sm, marginBottom: AppleSpace.md } : { ...aiSubFormGridStyle, gap: AppleSpace.sm, marginBottom: AppleSpace.md });
const toggleGridStyle = { display: 'flex', gap: AppleSpace.lg, marginBottom: AppleSpace.md, flexWrap: 'wrap' as const };
const toggleLabelStyle = { ...aiSubToggleLabelStyle, cursor: 'pointer' };
const actionRowStyle = { ...aiSubActionRowStyle, gap: AppleSpace.sm };
const testResultStyle = { marginTop: AppleSpace.md, padding: AppleSpace.md, backgroundColor: cv('surface-panel'), borderRadius: AppleRadius.card, border: `1px solid ${cv('border-light')}` };
const resultCodeStyle = { ...aiSubBodyStyle, maxHeight: '300px' };
const statsRowStyle = { ...crudStatsRowStyle('compact'), marginBottom: 0 };
const actionGridStyle = { ...aiActionGridStyle, gap: AppleSpace.sm };
const actionCardStyle = { ...aiActionCardStyle, padding: AppleSpace.md };
const actionTitleStyle = { ...aiActionTitleStyle, fontSize: AppleType.controlLabel.size };
const actionDescStyle = { ...aiActionDescStyle, fontSize: AppleType.microUI.size };
const spaceSelectRowStyle = { display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.xs, alignItems: 'stretch' as const };
const configContentStyle = { padding: `${AppleSpace.xs} 0` };
const configRowItemStyle = (idx: number) => ({
  display: 'flex', justifyContent: 'space-between', alignItems: 'center',
  padding: `${AppleSpace.sm} 0`,
  borderBottom: idx < vectorConfigItems.value.length - 1 ? `1px solid ${cv('border-light')}` : 'none',
});
const configLabelStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-secondary') };
const configValueStyle = { fontSize: AppleType.legalMeta.size, fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary') };
</script>
