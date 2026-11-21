<template>
  <ApPageContainer
    :title="$t('admin.k160')"
    :subtitle="$t('admin.k162')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <div v-if="loading" :style="compactLoadingStyle"><ApSkeleton variant="card" :count="3" /></div>
    <template v-else>

      <div :style="compactDashboardStatsStyle">
        <ApStatCard
          size="compact"
          :label="$t('admin.k155')"
          :value="stats?.today_searches ?? 0"
          :icon="'Search'"
          :color="cv('blue')"
        />
        <ApStatCard
          size="compact"
          :label="$t('admin.k156')"
          :value="indexFacts?.doc_count ?? 0"
          :icon="'FileText'"
          :color="cv('green')"
        />
        <ApStatCard
          size="compact"
          :label="$t('admin.k157')"
          :value="noResultRate"
          :icon="'AlertTriangle'"
          :color="cv('orange')"
        />
      </div>

      <div :style="compactDashboardMiddleStyle">
        <ApCard>
          <div :style="compactChartHeaderStyle">
            <h2 :style="chartTitleStyle">{{ $t('admin.k148') }}</h2>
            <div :style="compactChartActionsStyle">
              <ApButton variant="secondary" size="small" :loading="rebuilding" @click="handleRebuild">
                {{ $t('admin.k149') }}
              </ApButton>
              <ApButton variant="ghost" size="small" @click="loadData">
                <ApIcon name="RefreshCw" :size="16" />{{ $t('admin.refresh') }}
              </ApButton>
            </div>
          </div>
          <div :style="compactChartBodyStyle">
            <ApEmpty v-if="trendBars.length === 0" icon="BarChart3" :title="$t('admin.k161')" :description="$t('vAdminUi.k17')" />
            <div v-else :style="barChartContainerStyle()">
              <!-- P1-4：补 y 轴三档刻度，柱高才有量级参照 -->
              <div :style="[barChartColumnStyle(), axisColumnStyle]">
                <div :style="[barChartAxisStyle('160px'), axisBoxStyle]" aria-hidden="true">
                  <span :style="barChartAxisLabelStyle()">{{ trendMax }}</span>
                  <span :style="barChartAxisLabelStyle()">{{ Math.round(trendMax / 2) }}</span>
                  <span :style="barChartAxisLabelStyle()">0</span>
                </div>
                <span :style="barChartLabelStyle()" aria-hidden="true">&nbsp;</span>
              </div>
              <div v-for="(bar, i) in trendBars" :key="i" :style="barChartColumnStyle()">
                <div :style="barChartBarWrapperStyle()">
                  <div
                    :title="`${bar.label} · ${bar.count}`"
                    :style="{
                      height: bar.percent + '%',
                      width: '100%',
                      maxWidth: '28px',
                      background: bar.color,
                      borderRadius: `${AppleRadius.micro} ${AppleRadius.micro} 0 0`,
                      transition: AppleTransition.default,
                    }"
                  />
                </div>
                <span :style="barChartLabelStyle()">{{ bar.label }}</span>
              </div>
            </div>
          </div>

          <div v-if="reindexStatus && (reindexStatus.is_running || reindexStatus.progress > 0)" :style="compactProgressSectionStyle">
            <div :style="progressHeaderStyle">
              <span :style="progressLabelStyle">{{ reindexStatus.message }}</span>
              <span :style="progressPercentStyle">{{ reindexStatus.progress }}%</span>
            </div>
            <ApProgress :value="reindexStatus.progress" :label="$t('admin.k158')" :style="{ width: '100%' }" />
            <div v-if="reindexStatus.current > 0" :style="progressDetailStyle">
              {{ reindexStatus.current }} / {{ reindexStatus.total || '?' }}
            </div>
          </div>
        </ApCard>

        <ApCard>
          <div :style="compactChartHeaderStyle">
            <h2 :style="chartTitleStyle">{{ $t('admin.k151') }}</h2>
            <div :style="statusIndicatorLargeStyle(indexHealthStatus)">
              <span :style="statusDotStyle(healthToDotStatus(indexHealthStatus))" />
              {{ indexHealthLabel }}
            </div>
          </div>
          <div :style="compactHealthBodyStyle">
            <div :style="compactHealthCircleStyle">
              <ApIcon v-if="indexHealth === 'online'" name="CheckCircle2" :size="AppleIconSize.status" :color="cv('green')" />
              <ApIcon v-else-if="indexHealth === 'warning'" name="AlertTriangle" :size="AppleIconSize.status" :color="cv('orange')" />
              <ApIcon v-else name="XCircle" :size="AppleIconSize.status" :color="cv('red')" />
            </div>
            <p :style="healthDescStyle">{{ indexHealthDesc }}</p>

            <div :style="compactHealthMetricsStyle">
              <div v-for="metric in perfMetrics" :key="metric.label" :style="healthMetricItemStyle">
                <span :style="healthMetricLabelStyle">{{ metric.label }}</span>
                <span :style="healthMetricValueStyle">{{ metric.value }}</span>
              </div>
            </div>
          </div>
        </ApCard>
      </div>

      <div :style="compactDashboardBottomStyle">
        <ApCard>
          <div :style="compactChartHeaderStyle">
            <h2 :style="chartTitleStyle">{{ $t('admin.k152') }}</h2>
          </div>
          <div :style="compactTagCloudStyle()">
            <span
              v-for="(term, i) in hotSearchTerms"
              :key="i"
              :style="compactPillBadgeStyle(term.weight)"
            >
              {{ term.term }}
              <span :style="hotWordCountStyle()">{{ term.count }}</span>
            </span>
          </div>
          <ApEmpty v-if="hotSearchTerms.length === 0" icon="Search" :title="$t('admin.k161')" :description="$t('vAdminUi.k18')" />
        </ApCard>

        <ApCard>
          <div :style="compactChartHeaderStyle">
            <h2 :style="chartTitleStyle">{{ $t('admin.k153') }}</h2>
          </div>
          <div :style="compactDebugFormStyle()">
            <ApInput v-model="debugQuery" :aria-label="$t('admin.k163')" :placeholder="$t('admin.k159')" :style="{ flex: '1' }" />
            <ApButton variant="primary" size="small" :loading="debugging" @click="handleDebug">
              <ApIcon name="Search" :size="16" />{{ $t('admin.search') }}
            </ApButton>
          </div>
          <div v-if="debugResult" :style="compactDebugResultStyle()">
            <pre :style="{ margin: '0', fontFamily: AppleFont.mono, fontSize: AppleType.microUI.size, color: cv('text-primary'), whiteSpace: 'pre-wrap' }">{{ debugResult }}</pre>
          </div>

          <div :style="compactIndexInfoStyle">
            <div :style="compactChartHeaderStyle">
              <h2 :style="chartTitleStyle">{{ $t('admin.k154') }}</h2>
            </div>
            <div :style="infoGridStyle">
              <div v-for="info in indexInfoItems" :key="info.label" :style="infoItemStyle">
                <span :style="infoLabelStyle">{{ info.label }}</span>
                <!-- 长路径（如索引目录绝对路径）省略号收尾，悬停 title 看全文并可选中复制 -->
                <span
                  :style="{ ...infoValueStyle, maxWidth: '68%', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', userSelect: 'text' }"
                  :title="info.value"
                >{{ info.value }}</span>
              </div>
            </div>
          </div>
        </ApCard>
      </div>
    </template>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, onMounted, onUnmounted } from 'vue';
import { logger } from '@/utils/logger';
import {AppleFont, AppleFontWeight, AppleGridFloor, AppleIconSize, AppleLayout, AppleRadius, AppleSpace, AppleTransition, AppleType, cv} from '@/utils/core/apple-tokens';
import { statusIndicatorLargeStyle, statusDotStyle, monitorChartTitleStyle, monitorHealthDescStyle, monitorHealthMetricItemStyle, monitorHealthMetricLabelStyle, monitorHealthMetricValueStyle, monitorProgressHeaderStyle, monitorProgressLabelStyle, monitorProgressPercentStyle, monitorProgressDetailStyle, monitorInfoGridStyle, monitorInfoItemStyle, monitorInfoLabelStyle, monitorInfoValueStyle, barChartContainerStyle, barChartColumnStyle, barChartBarWrapperStyle, barChartLabelStyle, barChartAxisStyle, barChartAxisLabelStyle, crudStatsRowStyle, compactLoadingStyle } from '@/styles/layout/page-styles';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApCard from '@/components/apple/ApCard.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import { searchMonitor, reindexAll, searchDebug, getReindexStatus, type ReindexStatus, type SearchMonitorData } from '@/api/modules/document/search';
import { useUIStore } from '@/stores/app/ui';
import { formatBytes } from '@/utils/format/format-bytes';
import { formatTime } from '@/utils/format/format-time';

const ui = useUIStore();

const loading = shallowRef(false);
const rebuilding = shallowRef(false);
const debugging = shallowRef(false);
const reindexStatus = ref<ReindexStatus | null>(null);
let statusPollTimer: ReturnType<typeof setInterval> | null = null;

const monitor = ref<SearchMonitorData | null>(null);
const debugQuery = ref('');
const debugResult = ref('');

const indexFacts = computed(() => monitor.value?.index ?? null);
const stats = computed(() => monitor.value?.statistics ?? null);

const indexHealth = computed(() => {
  const i = indexFacts.value;
  if (!i || !i.is_open) return 'error';
  if (i.indexing) return 'warning';
  if (i.doc_count > 0) return 'online';
  return 'warning';
});

const indexHealthStatus = computed<'healthy' | 'warning' | 'error'>(() => {
  if (indexHealth.value === 'online') return 'healthy';
  if (indexHealth.value === 'warning') return 'warning';
  return 'error';
});

function healthToDotStatus(status: 'healthy' | 'warning' | 'error'): 'online' | 'offline' | 'enabled' | 'disabled' {
  if (status === 'healthy') return 'online';
  if (status === 'warning') return 'enabled';
  return 'offline';
}

const indexHealthLabel = computed(() => {
  if (indexHealth.value === 'online') return t('common.normal');
  if (indexHealth.value === 'warning') return t('common.warning');
  return t('admin.k176');
});

const indexHealthDesc = computed(() => {
  const i = indexFacts.value;
  if (!i) return t('admin.k177');
  if (!i.is_open) return t('admin.k178');
  if (i.indexing) return t('admin.k179');
  if (i.doc_count === 0) return t('admin.k180');
  return t('admin.k181', { n: i.doc_count });
});

const noResultRate = computed(() => {
  const rate = stats.value?.no_result_rate ?? 0;
  return rate > 0 ? rate.toFixed(1) + '%' : '0%';
});

const perfMetrics = computed(() => {
  const i = indexFacts.value;
  if (!i) return [];
  const avgLatency = Math.round(stats.value?.avg_latency_ms ?? 0);
  // 只报事实。原先每条还挂一根 ApProgress，进度值取自三个写死的"展示上限"
  // （1000 篇 / 100MB / 500ms）—— 用户读到的是一根没有目标的满格蓝条，看着像分隔线。
  return [
    { label: t('admin.k164'), value: String(i.doc_count) },
    { label: t('admin.k165'), value: formatBytes(i.index_size) },
    { label: t('admin.k166'), value: avgLatency + 'ms' },
  ];
});

const trendMax = computed(() => (stats.value?.trend ?? []).reduce((m, t) => (t.count > m ? t.count : m), 0));

const trendBars = computed(() => {
  const trend = stats.value?.trend ?? [];
  const max = trend.reduce((m, t) => (t.count > m ? t.count : m), 0);
  // 全为 0 时不画一排等高空柱，交给空态说明"这个窗口还没有搜索"
  if (max === 0) return [];
  return trend.map((t) => ({
    label: t.date.length >= 10 ? t.date.slice(5) : t.date,
    count: t.count,
    percent: (t.count / max) * 100,
    color: cv('blue'),
  }));
});

// 权重来自真实次数占榜首的比例，不再用「第几名」假装热度
const hotSearchTerms = computed(() => {
  const words = stats.value?.hot_words ?? [];
  const max = words.reduce((m, w) => (w.count > m ? w.count : m), 0);
  if (max === 0) return [];
  return words.map((w) => ({
    term: w.term,
    count: w.count,
    weight: w.count / max,
  }));
});

const indexInfoItems = computed(() => {
  const i = indexFacts.value;
  if (!i) return [];
  return [
    { label: t('admin.k167'), value: i.index_path || '--' },
    { label: t('admin.k168'), value: String(i.doc_count) },
    { label: t('admin.k165'), value: formatBytes(i.index_size) },
    { label: t('admin.k169'), value: i.last_updated ? formatTime(i.last_updated) : '--' },
    { label: t('admin.k170'), value: i.indexing ? t('admin.k182') : i.is_open ? t('common.normal') : t('admin.k183') },
  ];
});

async function loadData() {
  loading.value = true;
  try {
    monitor.value = await searchMonitor();
  } catch {
    ui.showToast(t('admin.k171'), 'error');
  } finally {
    loading.value = false;
  }
}

async function handleRebuild() {
  rebuilding.value = true;
  try {
    await reindexAll();
    ui.showToast(t('admin.k172'), 'success');
    startStatusPolling();
  } catch (error: unknown) {
    logger.error('Reindex error:', error);
    let errorMsg = t('sentUi.k18');
    if (typeof error === 'object' && error !== null) {
      const err = error as Record<string, unknown>;
      if (typeof err.message === 'string') errorMsg = err.message;
      else if (typeof err.response === 'object' && err.response !== null) {
        const resp = err.response as Record<string, unknown>;
        const respData = resp.data as Record<string, unknown> | undefined;
        if (respData) {
          if (typeof respData.message === 'string') errorMsg = respData.message;
          else if (typeof respData.msg === 'string') errorMsg = respData.msg;
        }
      }
    }
    ui.showToast(errorMsg, 'error');
    rebuilding.value = false;
  }
}

function startStatusPolling() {
  if (statusPollTimer) clearInterval(statusPollTimer);
  checkReindexStatus();
  statusPollTimer = setInterval(checkReindexStatus, 2000);
}

function stopStatusPolling() {
  if (statusPollTimer) {
    clearInterval(statusPollTimer);
    statusPollTimer = null;
  }
}

async function checkReindexStatus() {
  try {
    const status = await getReindexStatus();
    reindexStatus.value = status;
    if (!status.is_running && status.progress === 100) {
      stopStatusPolling();
      rebuilding.value = false;
      ui.showToast(status.message || t('admin.k184'), 'success');
      await loadData();
    }
  } catch (error) {
    logger.error('Failed to check reindex status:', error);
    ui.showToast(t('admin.k173'), 'error');
    stopStatusPolling();
    rebuilding.value = false;
  }
}

async function handleDebug() {
  if (!debugQuery.value.trim()) return;
  debugging.value = true;
  debugResult.value = '';
  try {
    const res = await searchDebug(Number(debugQuery.value));
    debugResult.value = JSON.stringify(res, null, 2);
  } catch {
    debugResult.value = t('admin.k185');
  } finally {
    debugging.value = false;
  }
}

onMounted(loadData);

onUnmounted(() => {
  stopStatusPolling();
});

const chartTitleStyle = monitorChartTitleStyle;

// y 轴列与柱列同构（160px 柱区 + 一行 x 标签占位），基线才会对齐
const axisColumnStyle = { flex: '0 0 40px' };
const axisBoxStyle = { width: '100%', paddingRight: AppleSpace.xs };
const progressHeaderStyle = monitorProgressHeaderStyle;
const progressLabelStyle = monitorProgressLabelStyle;
const progressPercentStyle = monitorProgressPercentStyle;
const progressDetailStyle = monitorProgressDetailStyle;
const infoGridStyle = monitorInfoGridStyle;
const infoItemStyle = monitorInfoItemStyle;
const infoLabelStyle = monitorInfoLabelStyle;
const infoValueStyle = monitorInfoValueStyle;
const healthMetricItemStyle = monitorHealthMetricItemStyle;
const healthMetricLabelStyle = monitorHealthMetricLabelStyle;
const healthMetricValueStyle = monitorHealthMetricValueStyle;
const healthDescStyle = monitorHealthDescStyle;

const compactDashboardStatsStyle = crudStatsRowStyle('compact');

// 面板并排只认 panel 地板（440px）：原来写死 '3fr 2fr' / '1fr 1fr'，
// 在 471px 的内容区里算出 273px + 182px 两栏，卡标题和两个按钮被挤成一团。
// 地板自带 min(…,100%)，窄屏自动塌成单列，不需要再写 isMobile 分支。
const compactDashboardMiddleStyle = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fit, ${AppleGridFloor.panel})`,
  gap: AppleSpace.md,
  marginBottom: AppleSpace.md,
};

const compactDashboardBottomStyle = {
  display: 'grid',
  gridTemplateColumns: `repeat(auto-fit, ${AppleGridFloor.panel})`,
  gap: AppleSpace.md,
};

const compactChartHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between' as const,
  marginBottom: AppleSpace.md,
};

const compactChartActionsStyle = {
  display: 'flex',
  gap: AppleSpace.xs,
};

const compactChartBodyStyle = {
  padding: `${AppleSpace.xs} 0`,
};

const compactHealthBodyStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  alignItems: 'center',
  gap: AppleSpace.sm,
  padding: `${AppleSpace.sm} 0`,
};

const compactHealthCircleStyle = {
  width: AppleSpace['3xl'],
  height: AppleSpace['3xl'],
  borderRadius: AppleRadius.circle,
  background: cv('bg-secondary'),
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'center',
  flexShrink: '0',
};

const compactHealthMetricsStyle = {
  width: '100%',
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
  marginTop: AppleSpace.sm,
};

const compactProgressSectionStyle = {
  marginTop: AppleSpace.md,
  paddingTop: AppleSpace.md,
  borderTop: `1px solid ${cv('border-light')}`,
};

function compactTagCloudStyle() {
  return {
    display: 'flex',
    flexWrap: 'wrap' as const,
    gap: AppleSpace.xxs,
    marginBottom: AppleSpace.xs,
  };
}

function compactPillBadgeStyle(weight: number) {
  return {
    display: 'inline-flex',
    alignItems: 'center',
    padding: `${AppleSpace.xxs} ${AppleSpace.sm}`,
    borderRadius: AppleRadius.pill,
    fontFamily: AppleFont.text,
    fontSize: AppleType.microUI.size,
    fontWeight: String(weight > 0.6 ? AppleFontWeight.semibold : AppleFontWeight.regular),
    background: weight > 0.7 ? cv('blue') : weight > 0.5 ? cv('blue-light') : cv('bg-tertiary'),
    color: weight > 0.7 ? cv('on-accent') : weight > 0.5 ? cv('blue') : cv('text-secondary'),
    transition: AppleTransition.fast,
    cursor: 'default' as const,
  };
}

// 搜索次数：不单独设色，继承药丸自己的前景色。
// 染成 text-tertiary 会压在热度>0.7 的实心蓝上，对比度直接不达标——
// 高热度条目的底色是动态的，前景只能跟着条目走。
function hotWordCountStyle() {
  return {
    marginLeft: AppleSpace.xxs,
    fontSize: AppleType.microUI.size,
    fontWeight: String(AppleFontWeight.regular),
  };
}

function compactDebugFormStyle() {
  return {
    display: 'flex',
    gap: AppleSpace.sm,
    marginBottom: AppleSpace.sm,
    flexWrap: 'wrap' as const,
  };
}

function compactDebugResultStyle() {
  return {
    background: cv('bg-tertiary'),
    borderRadius: AppleRadius.control,
    padding: AppleSpace.sm,
    overflow: 'auto',
    maxHeight: '240px',
  };
}

const compactIndexInfoStyle = {
  marginTop: AppleSpace.md,
  paddingTop: AppleSpace.md,
  borderTop: `1px solid ${cv('border-light')}`,
};
</script>
