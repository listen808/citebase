<template>
  <div :style="aiSubPageStyle">
    <!-- ==================== AI 模块健康 ==================== -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <div :style="aiSubCardNameRowStyle">
            <ApIcon name="HeartPulse" :size="18" :color="healthColor" />
            <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k61') }}</span>
          </div>
          <div :style="aiSubActionRowStyle">
            <ApStatusBadge :status="overallStatus" :label="overallLabel" />
            <ApButton variant="secondary" size="small" :loading="healthLoading" @click="loadHealth">
              <ApIcon name="RotateCw" :size="16" /> {{ $t('aiSet.refresh') }}
            </ApButton>
          </div>
        </div>
      </template>

      <p :style="aiSubDescStyle">
        {{ $t('aiSet.k63') }}
      </p>

      <div v-if="healthError" :style="aiSubServerErrorStyle">
        <ApIcon name="AlertTriangle" :size="14" /> {{ healthError }}
      </div>

      <div v-for="item in healthItems" :key="item.key" :style="aiSubSectionStyle">
        <div :style="statusRowStyle">
          <span :style="aiSubStatusDotStyle(item.ready)" />
          <span :style="aiSubCardNameStyle">{{ item.label }}</span>
          <span :style="aiSubSectionMetaStyle">{{ item.hint }}</span>
          <ApStatusBadge
            :status="item.ready ? 'enabled' : 'disabled'"
            :label="item.ready ? '就绪' : '未就绪'"
            size="small"
          />
        </div>
      </div>
    </ApCard>

    <!-- ==================== 能力成熟度 ==================== -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <div :style="aiSubCardNameRowStyle">
            <ApIcon name="Layers" :size="18" :color="cv('indigo')" />
            <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k64') }}</span>
            <span :style="aiSubCountBadgeStyle">{{ featureCounts.total }}</span>
          </div>
          <div :style="aiSubActionRowStyle">
            <ApButton variant="secondary" size="small" :loading="maturityLoading" @click="loadMaturity">
              <ApIcon name="RotateCw" :size="16" /> {{ $t('aiSet.refresh') }}
            </ApButton>
          </div>
        </div>
      </template>

      <div :style="aiSubToolbarBetweenStyle">
        <div :style="aiSubFilterStyle">
          <span :style="aiSubFilterLabelStyle">{{ $t('aiSet.k65') }}</span>
          <ApSelect v-model="minLevel" :options="minLevelOptions" @update:model-value="loadMaturity" />
        </div>
        <span :style="aiSubSectionMetaStyle">
          能力清单由后端内置，只读；当前生效阈值 {{ MATURITY_LABELS[report.min_maturity || 'experimental'] }}
        </span>
      </div>

      <div :style="crudStatsRowStyle('compact')">
        <ApStatCard :label="$t('aiSet.k67')" :value="featureCounts.stable" :icon="'ShieldCheck'" :color="cv('green')" size="compact" />
        <ApStatCard label="Beta" :value="featureCounts.beta" :icon="'Zap'" :color="cv('blue')" size="compact" />
        <ApStatCard :label="$t('aiSet.k68')" :value="featureCounts.experimental" :icon="'Beaker'" :color="cv('orange')" size="compact" />
        <ApStatCard :label="$t('aiSet.k66')" :value="tools.length" :icon="'Wrench'" :color="cv('purple')" size="compact" />
      </div>

      <div v-if="maturityError" :style="aiSubServerErrorStyle">
        <ApIcon name="AlertTriangle" :size="14" /> {{ maturityError }}
      </div>

      <ApEmpty
        v-if="!maturityLoading && featureCounts.total === 0"
        icon="Search"
        :title="$t('aiSet.k69')"
        :description="$t('vAiUi.k208')"
      />

      <div v-for="group in featureGroups" :key="group.key" :style="aiSubSectionStyle">
        <div :style="aiSubSectionHeaderStyle">
          <div :style="aiSubCardNameRowStyle">
            <span :style="aiSubSectionTitleStyle">{{ group.label }}</span>
            <span :style="aiSubCountBadgeStyle">{{ group.items.length }}</span>
          </div>
        </div>
        <ApTable density="compact" :columns="maturityColumns" :data="group.items" />
      </div>

      <div v-if="tools.length" :style="aiSubSectionStyle">
        <div :style="aiSubSectionHeaderStyle">
          <div :style="aiSubCardNameRowStyle">
            <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k66') }}</span>
            <span :style="aiSubCountBadgeStyle">{{ tools.length }}</span>
          </div>
        </div>
        <ApTable density="compact" :columns="toolColumns" :data="tools" />
      </div>
    </ApCard>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, computed, h, onMounted } from 'vue';

import { logger } from '@/utils/logger';
import { cv } from '@/utils/core/apple-tokens';
import {
  AI_TABLE_WIDTH,
  crudStatsRowStyle,
  aiSubActionRowStyle,
  aiSubCardNameRowStyle,
  aiSubCardNameStyle,
  aiSubCountBadgeStyle,
  aiSubDescStyle,
  aiSubFilterLabelStyle,
  aiSubFilterStyle,
  aiSubPageStyle,
  aiSubSectionHeaderStyle,
  aiSubSectionMetaStyle,
  aiSubSectionStyle,
  aiSubSectionTitleStyle,
  aiSubServerErrorStyle,
  aiSubStatusDotStyle,
  aiSubToolbarBetweenStyle,
  statusRowStyle,
} from '@/styles/ai/ai-page-styles';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import ApStatusBadge from '@/components/apple/ApStatusBadge.vue';
import {
  AICapabilityAPI,
  type AIHealthStatus,
  type FeatureMaturity,
  type MaturityLevel,
  type ToolMaturityEntry,
} from '@/api/modules/ai/ai-capability';

const MATURITY_LABELS: Record<string, string> = {
  stable: '稳定',
  beta: 'Beta',
  experimental: '实验性',
};

function maturityBadge(level: string): 'enabled' | 'default' | 'warning' {
  if (level === 'stable') return 'enabled';
  if (level === 'beta') return 'default';
  return 'warning';
}

function renderMaturity(level: string) {
  return h(ApStatusBadge, {
    status: maturityBadge(level),
    label: MATURITY_LABELS[level] ?? level,
    size: 'small',
  });
}

/* -------------------- 健康检查 -------------------- */

const health = ref<AIHealthStatus | null>(null);
const healthLoading = ref(false);
const healthError = ref('');

const HEALTH_HINTS: Record<string, string> = {
  enabled: 'AI 模块是否完成装配',
  primary_provider: '主 LLM 供应商是否可用',
  embedding_configured: '嵌入模型与向量写入是否就绪',
  retriever_ready: 'RAG 检索管线是否可用',
  vector_store_ready: '向量存储是否就绪',
};

const healthItems = computed(() => {
  const source = health.value;
  if (!source) return [];
  const keys: ('enabled' | 'primary_provider' | 'embedding_configured' | 'retriever_ready' | 'vector_store_ready')[] = [
    'enabled', 'primary_provider', 'embedding_configured', 'retriever_ready', 'vector_store_ready',
  ];
  const labels: Record<string, string> = {
    enabled: 'AI 模块已启用',
    primary_provider: '主 LLM 供应商',
    embedding_configured: '向量嵌入服务',
    retriever_ready: 'RAG 检索管线',
    vector_store_ready: '向量存储',
  };
  return keys.map((key) => ({
    key,
    label: labels[key],
    hint: HEALTH_HINTS[key],
    ready: Boolean(source[key]),
  }));
});

const overallStatus = computed<'enabled' | 'warning' | 'error'>(() => {
  const overall = health.value?.overall;
  if (overall === 'UP') return 'enabled';
  if (overall === 'DEGRADED') return 'warning';
  return 'error';
});

const overallLabel = computed(() => {
  const map: Record<string, string> = { UP: '服务正常', DEGRADED: '部分降级', DOWN: '不可用' };
  return map[health.value?.overall ?? ''] ?? t('fallback.k77');
});

const healthColor = computed(() => {
  if (overallStatus.value === 'enabled') return cv('green');
  return overallStatus.value === 'warning' ? cv('orange') : cv('red');
});

async function loadHealth() {
  healthLoading.value = true;
  healthError.value = '';
  try {
    health.value = await AICapabilityAPI.health();
  } catch (e) {
    logger.error('Failed to load AI health:', e);
    healthError.value = e instanceof Error ? e.message: t('aiSet.k70');
  } finally {
    healthLoading.value = false;
  }
}

/* -------------------- 能力成熟度 -------------------- */

const minLevel = ref<string>('experimental');
const minLevelOptions = [
  { label: t('aiSet.k71'), value: 'experimental' },
  { label: t('aiSet.k72'), value: 'beta' },
  { label: t('aiSet.k73'), value: 'stable' },
];

const report = ref<{ rag: FeatureMaturity[]; text2sql: FeatureMaturity[]; analyzer: FeatureMaturity[]; tools: ToolMaturityEntry[]; min_maturity: string }>({
  rag: [],
  text2sql: [],
  analyzer: [],
  tools: [],
  min_maturity: 'experimental',
});
const maturityLoading = ref(false);
const maturityError = ref('');

const maturityColumns: TableColumn<FeatureMaturity>[] = [
  { key: 'name', title: t('aiSet.k74'), width: AI_TABLE_WIDTH.xxl },
  { key: 'category', title: t('aiSet.k75'), width: AI_TABLE_WIDTH.md },
  { key: 'maturity', title: t('aiSet.k76'), width: AI_TABLE_WIDTH.md, render: (row) => renderMaturity(row.maturity) },
  { key: 'description', title: t('aiSet.k77'), ellipsis: true },
];

const toolColumns: TableColumn<ToolMaturityEntry>[] = [
  { key: 'name', title: t('aiSet.k78'), width: AI_TABLE_WIDTH.xxl },
  { key: 'maturity', title: t('aiSet.k76'), width: AI_TABLE_WIDTH.md, render: (row) => renderMaturity(row.maturity) },
];

const ragFeatures = computed(() => report.value.rag);
const text2sqlFeatures = computed(() => report.value.text2sql);
const analyzerFeatures = computed(() => report.value.analyzer);
const tools = computed(() => report.value.tools);

const featureGroups = computed(() => [
  { key: 'rag', label: t('aiSet.k79'), items: ragFeatures.value },
  { key: 'text2sql', label: t('aiSet.k80'), items: text2sqlFeatures.value },
  { key: 'analyzer', label: t('aiSet.k81'), items: analyzerFeatures.value },
]);

const featureCounts = computed(() => {
  const all = [...ragFeatures.value, ...text2sqlFeatures.value, ...analyzerFeatures.value];
  const countOf = (level: string) => all.filter((item) => item.maturity === level).length;
  return {
    total: all.length + tools.value.length,
    stable: countOf('stable'),
    beta: countOf('beta'),
    experimental: countOf('experimental'),
  };
});

async function loadMaturity() {
  maturityLoading.value = true;
  maturityError.value = '';
  try {
    const res = await AICapabilityAPI.maturity(minLevel.value as MaturityLevel);
    report.value = {
      rag: res.rag ?? [],
      text2sql: res.text2sql ?? [],
      analyzer: res.analyzer ?? [],
      tools: res.tools ?? [],
      min_maturity: res.min_maturity ?? minLevel.value,
    };
  } catch (e) {
    logger.error('Failed to load feature maturity:', e);
    maturityError.value = e instanceof Error ? e.message: t('aiSet.k82');
  } finally {
    maturityLoading.value = false;
  }
}

onMounted(() => {
  void Promise.all([loadHealth(), loadMaturity()]);
});
</script>
