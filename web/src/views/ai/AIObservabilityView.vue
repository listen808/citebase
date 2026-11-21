<template>
  <div :style="aiSubPageStyle">
    <ApSkeleton v-if="loading" :lines="5" />
    <template v-else>
    <div :style="headerStyle">
      <div :style="refreshStyle">
        <ApSwitch v-model="autoRefresh" size="small" :label="$t('aiView2.k39')" />
      </div>
    </div>

    <div :style="crudStatsRowFitStyle(5, isMobile, 'compact')">
      <ApStatCard :label="$t('aiView2.k40')" :value="formatNumberCompact(dashboard.summary.total_requests)" :icon="'Zap'" :color="cv('blue')" size="compact" />
      <ApStatCard :label="$t('aiView2.k41')" :value="avgDurationText" :icon="'Clock'" :color="cv('green')" size="compact" />
      <ApStatCard :label="$t('aiView2.k42')" :value="formatNumberCompact(dashboard.summary.total_tokens)" :icon="'Cpu'" :color="cv('purple')" size="compact" />
      <ApStatCard :label="$t('aiView2.k43')" :value="cacheHitText" :icon="'HardDrive'" :color="cv('orange')" size="compact" />
      <ApStatCard :label="$t('aiView2.k44')" :value="dashboard.summary.active_requests" :icon="'Activity'" :color="cv('teal')" size="compact" />
    </div>

    <div :style="twoColStyle">
      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="BarChart3" :size="16" :color="cv('blue')" />
            <span :style="sectionTitleStyle">{{ $t('aiView2.k35') }}</span>
          </div>
        </template>
        <div :style="metricsTableStyle">
          <div v-for="metric in dashboard.metrics" :key="metric.name + JSON.stringify(metric.labels)" :style="metricRowStyle">
            <div :style="metricNameStyle">
              <span :style="metricDotStyle(metric.type)"></span>
              {{ metric.name }}
            </div>
            <div :style="metricValueStyle">{{ formatNumberCompact(metric.value) }}</div>
            <div v-if="metric.labels && Object.keys(metric.labels).length > 0" :style="metricLabelsStyle">
              <span v-for="(v, k) in metric.labels" :key="k" :style="metricLabelStyle">
                {{ k }}: {{ v }}
              </span>
            </div>
          </div>
          <ApEmpty v-if="dashboard.metrics.length === 0" icon="BarChart3" :title="$t('aiView2.k45')" :description="$t('vAiUi.k39')" />
        </div>
      </ApCard>

      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="GitBranch" :size="16" :color="cv('purple')" />
            <span :style="sectionTitleStyle">{{ $t('aiView2.k36') }}</span>
            <span :style="traceCountStyle">最近 {{ traces.length }} 条</span>
            <ApButton variant="ghost" size="small" :loading="tracesLoading" @click="fetchTraces">
              <ApIcon name="RefreshCw" :size="16" /> {{ $t('aiView2.refresh') }}
            </ApButton>
          </div>
        </template>
        <div v-if="tracesError" :style="tracesErrorStyle">
          {{ tracesError }}
          <ApButton variant="secondary" size="small" @click="fetchTraces">{{ $t('aiView2.retry') }}</ApButton>
        </div>
        <ApSkeleton v-else-if="tracesLoading && traces.length === 0" variant="list" :lines="4" />
        <div v-else :style="tracesListStyle">
          <div v-for="trace in traces" :key="trace.id" :style="traceItemStyle">
            <div :style="traceHeaderStyle" @click="toggleTrace(trace.id)">
              <div :style="traceLeftStyle">
                <ApIcon name="ChevronRight" :size="14" :color="cv('text-tertiary')" :style="chevronStyle(expandedTraces.has(trace.id))" />
                <span :style="traceNameStyle">{{ trace.name }}</span>
                <span :style="traceStatusStyle(trace.status)">{{ statusLabel(trace.status) }}</span>
              </div>
              <div :style="traceRightStyle">
                <span :style="traceDurationStyle">{{ trace.duration_ms }}ms</span>
                <span :style="traceTimeStyle">{{ formatTime(trace.start_time) }}</span>
              </div>
            </div>
            <div v-if="expandedTraces.has(trace.id)" :style="traceDetailStyle">
              <div v-if="trace.children && trace.children.length > 0" :style="childrenListStyle">
                <div v-for="child in trace.children" :key="child.id" :style="childItemStyle">
                  <div :style="childLineStyle"></div>
                  <div :style="childContentStyle">
                    <div :style="childHeaderStyle">
                      <span :style="childNameStyle">{{ child.name }}</span>
                      <span :style="traceStatusStyle(child.status)">{{ statusLabel(child.status) }}</span>
                      <span :style="childDurationStyle">{{ child.duration_ms }}ms</span>
                    </div>
                    <div v-if="child.attributes && Object.keys(child.attributes).length > 0" :style="childAttrsStyle">
                      <span v-for="(v, k) in child.attributes" :key="k" :style="attrBadgeStyle">
                        {{ k }}: {{ formatAttrValue(v) }}
                      </span>
                    </div>
                  </div>
                </div>
              </div>
              <div v-if="trace.attributes && Object.keys(trace.attributes).length > 0" :style="traceAttrsStyle">
                <div :style="attrsTitleStyle">{{ $t('aiView2.k38') }}</div>
                <div :style="attrsGridStyle">
                  <div v-for="(v, k) in trace.attributes" :key="k" :style="attrItemStyle">
                    <span :style="attrKeyStyle">{{ k }}</span>
                    <span :style="attrValStyle">{{ formatAttrValue(v) }}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>
          <ApEmpty v-if="traces.length === 0" icon="GitBranch" :title="$t('aiView2.k46')" :description="$t('vAiUi.k40')" />
        </div>
      </ApCard>
    </div>
    </template>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, onMounted, onUnmounted, reactive, computed } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApCard from '@/components/apple/ApCard.vue';
import ApSwitch from '@/components/apple/ApSwitch.vue';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import { cv, AppleSpace, AppleRadius, AppleType, AppleFontWeight, AppleTransition } from '@/utils/core/apple-tokens';
import { useBreakpoint } from '@/utils/responsive';
import { crudStatsRowFitStyle, aiSubSectionHeaderStyle, aiSectionTitleStyle, aiSubPageStyle, aiSubCountBadgeStyle, aiSubServerErrorStyle } from '@/styles/ai/ai-page-styles';
import { getObservabilityDashboard, getObservabilityTraces, type DashboardData, type Span } from '@/api/modules/ai/ai-observability';
import { formatDurationMs, formatNumberCompact, formatTimeOnly as formatTime } from '@/utils/format/format-time';
import { useUIStore } from '@/stores/app/ui';
import { logger } from '@/utils/logger';

// 后端 /ai/observability/traces 的上限是 200（超出会被夹回 50）
const TRACE_LIMIT = 200;

const autoRefresh = ref(true);
const loading = ref(true);
const traces = ref<Span[]>([]);
const tracesLoading = ref(false);
const tracesError = ref('');
const ui = useUIStore();
const { isMobile } = useBreakpoint();
const expandedTraces = ref<Set<string>>(new Set());
let refreshTimer: number | null = null;

const dashboard = reactive<DashboardData>({
  summary: {
    total_requests: 0,
    total_tokens: 0,
    avg_duration_ms: 0,
    active_requests: 0,
    cache_hit_ratio: 0,
    cache_hits: 0,
    cache_misses: 0,
  },
  metrics: [],
  recent_traces: [],
});

// 没有请求时，均值与命中率没有意义 —— 写 0ms / 0.0% 等于撒谎（审查 §四.5：缺数据统一「—」）
const avgDurationText = computed(() =>
  dashboard.summary.total_requests === 0 ? '—' : formatDurationMs(dashboard.summary.avg_duration_ms),
);
const cacheHitText = computed(() =>
  dashboard.summary.total_requests === 0 ? '—' : `${dashboard.summary.cache_hit_ratio.toFixed(1)}%`,
);

async function fetchDashboard() {
  loading.value = true;
  try {
    const data = await getObservabilityDashboard();
    Object.assign(dashboard, data);
  } catch (e) {
    logger.error('[Observability] Fetch dashboard failed:', e);
    // 只 log 时这一屏的四个统计卡会停在 0，看着像「还没有调用」而不是「这次请求挂了」
    ui.showToast(t('aiView2.k47'), 'error');
  } finally {
    loading.value = false;
  }
}

// 面板数据源改成 /ai/observability/traces：dashboard 里那份 recent_traces 后端
// 写死只给 10 条（observability.go 的 ListRecentTraces(10)），排障时最近的抖动
// 一多就翻不到想看的 trace。两处返回的都是同一份 cloneSpan(root) 树，
// 所以列表已经带全子 span，无需再按 id 取详情。
async function fetchTraces() {
  tracesLoading.value = true;
  tracesError.value = '';
  try {
    const res = await getObservabilityTraces(TRACE_LIMIT);
    traces.value = res?.traces ?? [];
  } catch (e) {
    logger.error('[Observability] Fetch traces failed:', e);
    traces.value = [];
    // 失败要说成失败：留空列表会让空态写成"暂无追踪数据"，把工装故障报成没有数据
    tracesError.value = t('sentUi.k39');
  } finally {
    tracesLoading.value = false;
  }
}

function toggleTrace(id: string) {
  if (expandedTraces.value.has(id)) {
    expandedTraces.value.delete(id);
  } else {
    expandedTraces.value.add(id);
  }
}

function statusLabel(status: string): string {
  const map: Record<string, string> = {
    ok: '成功',
    error: '失败',
    running: '进行中',
  };
  return map[status] || status;
}

function formatAttrValue(v: unknown): string {
  if (typeof v === 'object') return JSON.stringify(v);
  return String(v);
}

onMounted(() => {
  fetchDashboard();
  // 追踪列表不进 5 秒轮询：一次最多 200 棵 span 树，跟着摘要一起刷等于持续重传
  // 整棵追踪树，而排障看的是已发生的调用，手动刷新就够了。
  fetchTraces();
  startRefresh();
});

onUnmounted(() => {
  stopRefresh();
});

function startRefresh() {
  stopRefresh();
  refreshTimer = window.setInterval(() => {
    if (autoRefresh.value) {
      fetchDashboard();
    }
  }, 5000);
}

function stopRefresh() {
  if (refreshTimer) {
    clearInterval(refreshTimer);
    refreshTimer = null;
  }
}

const headerStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'flex-end',
};

const refreshStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
};

const twoColStyle = computed(() => ({ display: 'grid', gridTemplateColumns: isMobile.value ? '1fr' : '1fr 1fr', gap: AppleSpace.md }));

const sectionTitleStyle = aiSectionTitleStyle;
const traceCountStyle = aiSubCountBadgeStyle;
const tracesErrorStyle = aiSubServerErrorStyle;

const metricsTableStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  maxHeight: '350px',
  overflowY: 'auto' as const,
};

const metricRowStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: `${AppleSpace.xs} 0`,
  borderBottom: `1px solid ${cv('border-light')}`,
  flexWrap: 'wrap' as const,
  gap: AppleSpace.xs,
};

const metricNameStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  fontSize: AppleType.bodyPrimary.size,
  color: cv('text-primary'),
  fontWeight: String(AppleFontWeight.medium),
};

const metricDotStyle = (type: string) => {
  const colorMap: Record<string, string> = {
    counter: cv('blue'),
    gauge: cv('green'),
    histogram: cv('orange'),
  };
  return {
    width: '8px',
    height: '8px',
    borderRadius: AppleRadius.circle,
    backgroundColor: colorMap[type] || cv('text-tertiary'),
  };
};

const metricValueStyle = {
  // 指标数值归统计数值那一档：20/600 + 数字等宽（原先 17/600 无 tabular-nums，
  // 与同页其它数值、与别的页的数值都不是一个尺寸）
  fontSize: AppleType.statValue.size,
  fontWeight: String(AppleType.statValue.weight),
  fontVariantNumeric: 'tabular-nums',
  color: cv('text-primary'),
  marginLeft: 'auto',
};

const metricLabelsStyle = {
  display: 'flex',
  gap: AppleSpace.xs,
  flexWrap: 'wrap' as const,
  width: '100%',
  paddingLeft: AppleSpace['20px'],
};

const metricLabelStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
  backgroundColor: cv('bg-secondary'),
  padding: `${AppleSpace.xxs} 6px`,
  borderRadius: AppleRadius.small,
};

const tracesListStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  maxHeight: '350px',
  overflowY: 'auto' as const,
};

const traceItemStyle = {
  borderBottom: `1px solid ${cv('border-light')}`,
};

const traceHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  padding: `${AppleSpace.xs} 0`,
  cursor: 'pointer',
  transition: AppleTransition.fast,
};

const traceLeftStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
};

const chevronStyle = (expanded: boolean) => ({
  transform: expanded ? 'rotate(90deg)' : 'rotate(0deg)',
  transition: AppleTransition.fast,
});

const traceNameStyle = {
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
};

const traceStatusStyle = (status: string) => {
  const colorMap: Record<string, string> = {
    ok: cv('green'),
    error: cv('red'),
    running: cv('orange'),
  };
  return {
    fontSize: AppleType.microUI.size,
    color: colorMap[status] || cv('text-tertiary'),
    backgroundColor: `color-mix(in srgb, ${colorMap[status] || cv('text-tertiary')} 10%, transparent)`,
    padding: `${AppleSpace.xxs} ${AppleSpace.sm}`,
    borderRadius: AppleRadius.small,
    fontWeight: String(AppleFontWeight.medium),
  };
};

const traceRightStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
};

const traceDurationStyle = {
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-secondary'),
  fontVariantNumeric: 'tabular-nums',
};

const traceTimeStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
};

const traceDetailStyle = {
  padding: `${AppleSpace.xs} 0 ${AppleSpace.sm}`,
  paddingLeft: AppleSpace.lg,
};

const childrenListStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
};

const childItemStyle = {
  display: 'flex',
  gap: AppleSpace.xs,
};

const childLineStyle = {
  width: '1px',
  backgroundColor: cv('border-light'),
  marginLeft: AppleSpace['7px'],
};

const childContentStyle = {
  flex: 1,
  backgroundColor: cv('bg-secondary'),
  borderRadius: AppleRadius.small,
  padding: AppleSpace.xs,
};

const childHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
  marginBottom: AppleSpace.xxs,
};

const childNameStyle = {
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
};

const childDurationStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
  marginLeft: 'auto',
  fontVariantNumeric: 'tabular-nums',
};

const childAttrsStyle = {
  display: 'flex',
  flexWrap: 'wrap' as const,
  gap: AppleSpace.xs,
};

const attrBadgeStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-secondary'),
  backgroundColor: cv('surface-panel'),
  padding: `${AppleSpace.xxs} 6px`,
  borderRadius: AppleRadius.small,
  border: `1px solid ${cv('border-light')}`,
};

const traceAttrsStyle = {
  marginTop: AppleSpace.sm,
  paddingTop: AppleSpace.sm,
  borderTop: `1px solid ${cv('border-light')}`,
};

const attrsTitleStyle = {
  fontSize: AppleType.microUI.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-secondary'),
  marginBottom: AppleSpace.xs,
};

const attrsGridStyle = computed(() => ({ display: 'grid', gridTemplateColumns: isMobile.value ? '1fr' : '1fr 1fr', gap: AppleSpace.xs }));

const attrItemStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
};

const attrKeyStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-tertiary'),
};

const attrValStyle = {
  fontSize: AppleType.microUI.size,
  color: cv('text-primary'),
  wordBreak: 'break-all' as const,
};


</script>
