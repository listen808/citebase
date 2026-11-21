<template>
  <ApPageContainer
    :title="$t('admin.k130')"
    :subtitle="$t('admin.k132')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <ApSkeleton v-if="loading" variant="grid" :count="4" />
    <ApError
      v-else-if="loadError"
      :message="loadError"
      show-retry
      @retry="fetchDashboardData"
    />
    <ApEmpty v-else-if="!dashboard && !serverStatus" icon="BarChart3" :title="$t('admin.k131')" />
    <div v-else :style="pageContentStyle">
      <DashboardKpiCards :stats="statsWithTrend" />

      <div :style="rowStyle">
        <div :style="col8Style">
          <DashboardCharts
            :trend-data="trendData"
            :space-rank-data="spaceRankData"
          />
        </div>
        <div :style="col4Style">
          <DashboardQuickActions :actions="quickActions" />
        </div>
      </div>

      <div :style="rowStyle">
        <div :style="col4Style">
          <DashboardRankings
            :active-users="dashboard?.active_users ?? 0"
            :top-creator="dashboard?.top_creator ?? null"
            :top-editor="dashboard?.top_editor ?? null"
            :top-collector="dashboard?.top_collector ?? null"
            :top-follower="dashboard?.top_follower ?? null"
          />
        </div>
        <div :style="col4Style">
          <DashboardHeatmap section="docs" :heatmap-data="activityHeatmapData" :hot-documents="hotDocuments" :active-users="activeUsersTop10" />
        </div>
        <div :style="col4Style">
          <DashboardHeatmap section="users" :heatmap-data="activityHeatmapData" :hot-documents="hotDocuments" :active-users="activeUsersTop10" />
        </div>
      </div>

      <div :style="rowStyle">
        <div :style="col7Style">
          <DashboardHeatmap section="heatmap" :heatmap-data="activityHeatmapData" :hot-documents="hotDocuments" :active-users="activeUsersTop10" />
        </div>
        <div :style="col5Style">
          <DashboardActivityTimeline :items="recentActivityItems" />
        </div>
      </div>

      <div :style="rowStyle">
        <div :style="col7Style">
          <DashboardServerStatus
            :server-items="serverItems"
            :error-log-count="serverStatus?.error_log_count"
            :recent-errors="recentErrors"
          />
        </div>
        <div :style="col5Style">
          <ApCard>
            <div :style="cardHeaderStyle">
              <h2 :style="cardTitleStyle">{{ $t('admin.k129') }}</h2>
            </div>
            <div :style="sysInfoGridStyle">
              <div
                v-for="item in systemInfoItems"
                :key="item.label"
                :style="sysInfoItemStyle"
              >
                <div :style="sysInfoIconStyle(item.color)">
                  <component :is="item.icon" :size="16" :color="item.color" />
                </div>
                <div :style="sysInfoTextStyle">
                  <div :style="sysInfoLabelStyle">{{ item.label }}</div>
                  <div :style="sysInfoValueStyle" :title="String(item.value)">{{ item.value }}</div>
                </div>
              </div>
            </div>
          </ApCard>
        </div>
      </div>
    </div>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, onMounted, onUnmounted, computed } from 'vue';
import { useRouter } from 'vue-router';
import { icons } from '@/utils/icons';
import { getDashboardStats, getServerStatus } from '@/api/modules/system/stats';
import { getRecentErrors } from '@/api/modules/system/log';
import type { RecentError } from '@/api/modules/system/log';
import type { DashboardStats as DashboardStatsType, ServerStatus } from '@/types/models';
import { cv, AppleLayout, AppleSpace, AppleRadius, AppleFont, AppleType, AppleGridFloor } from '@/utils/core/apple-tokens';
import { monitorChartTitleStyle as cardTitleStyle, entityIconBoxStyle } from '@/styles/layout/page-styles';
import { useBreakpoint } from '@/utils/responsive';
import { formatBytes } from '@/utils/format/format-bytes';
import { formatDuration, formatPercent as formatPercentValue } from '@/utils/format/format-time';
import { logger } from '@/utils/logger';
import { useUIStore } from '@/stores/app/ui';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApCard from '@/components/apple/ApCard.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApError from '@/components/apple/ApError.vue';
import DashboardQuickActions from './components/DashboardQuickActions.vue';
import DashboardActivityTimeline from './components/DashboardActivityTimeline.vue';
import DashboardKpiCards from './components/DashboardKpiCards.vue';
import DashboardCharts from './components/DashboardCharts.vue';
import DashboardRankings from './components/DashboardRankings.vue';
import DashboardHeatmap from './components/DashboardHeatmap.vue';
import DashboardServerStatus from './components/DashboardServerStatus.vue';
import type { ServerItem } from './components/DashboardServerStatus.vue';
import type { Component } from 'vue';
import type { StatWithTrend } from './components/DashboardKpiCards.vue';
import type { QuickAction } from './components/DashboardQuickActions.vue';

const router = useRouter();
const ui = useUIStore();
const { isMobile } = useBreakpoint();

const pageContentStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.md,
};

// 默认 stretch：同行卡片底边对齐，不再出现短卡下方的大片参差空白
const rowStyle = computed(() => ({
  display: 'grid',
  gridTemplateColumns: isMobile.value ? '1fr' : 'repeat(12, 1fr)',
  gap: AppleSpace.md,
}));

// 列容器用 grid：子卡（ApCard）自动双向拉伸填满格子，卡片才能真正等高
const colSpan = (span: number) => computed(() => ({
  gridColumn: isMobile.value ? 'span 1' : `span ${span}`,
  minWidth: '0',
  display: 'grid',
}));

const col8Style = colSpan(8);
const col7Style = colSpan(7);
const col5Style = colSpan(5);
const col4Style = colSpan(4);

const cardHeaderStyle = {
  marginBottom: AppleSpace.md,
};

const sysInfoGridStyle = {
  display: 'grid',
  // 自适应列数：在 col5 半宽容器里 2 列、全宽时 3+ 列，不再写死
  gridTemplateColumns: `repeat(auto-fill, ${AppleGridFloor.stat})`,
  gap: AppleSpace.sm,
};

const sysInfoItemStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.sm,
  padding: AppleSpace.sm,
  borderRadius: AppleRadius.control,
  background: cv('bg-secondary'),
};

function sysInfoIconStyle(color: string) {
  return entityIconBoxStyle(color);
}

const sysInfoTextStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
  minWidth: '0',
  flex: '1',
};

const sysInfoLabelStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
};

const sysInfoValueStyle = {
  fontFamily: AppleFont.mono,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-primary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const quickActions = computed<QuickAction[]>(() => [
  { label: t('admin.k88'), icon: 'LayoutGrid', handler: () => router.push('/spaces') },
  { label: t('admin.k133'), icon: 'Users', handler: () => router.push('/users') },
  { label: t('admin.k134'), icon: 'Settings', handler: () => router.push('/settings') },
  { label: t('admin.k135'), icon: 'Shield', handler: () => router.push('/privileges') },
]);

const recentActivityItems = computed(() => {
  const items: { title: string; time: string; action?: 'create' | 'edit' | 'delete' | 'collect' | 'info' }[] = [];
  const d = dashboard.value;
  if (!d) return items;
  if (d.top_creator) items.push({ title: t('sentUi.k15', { p1: d.top_creator.given_name || d.top_creator.username, p2: d.top_creator.count }), time: t('time.recent'), action: 'create' });
  if (d.top_editor) items.push({ title: t('sentUi.k16', { p1: d.top_editor.given_name || d.top_editor.username, p2: d.top_editor.count }), time: t('time.recent'), action: 'edit' });
  if (d.top_collector) items.push({ title: t('sentUi.k17', { p1: d.top_collector.given_name || d.top_collector.username, p2: d.top_collector.count }), time: t('time.recent'), action: 'collect' });
  return items.slice(0, 4);
});

const loading = shallowRef(false);
const loadError = shallowRef('');
const dashboard = ref<DashboardStatsType | null>(null);
const serverStatus = ref<ServerStatus | null>(null);
const recentErrors = ref<RecentError[]>([]);
let pollTimer: ReturnType<typeof setInterval> | null = null;

const statsWithTrend = computed<StatWithTrend[]>(() => {
  const d = dashboard.value;
  const trend = d?.document_trend ?? [];
  const last7 = trend.slice(-7).map(t => t.count);
  const prev7 = trend.slice(-14, -7).map(t => t.count);

  function calcTrend(current: number[], previous: number[]): number | undefined {
    const curSum = current.reduce((a, b) => a + b, 0);
    const prevSum = previous.reduce((a, b) => a + b, 0);
    if (prevSum === 0 && curSum === 0) return undefined;
    if (prevSum === 0) return curSum > 0 ? 100 : undefined;
    return ((curSum - prevSum) / prevSum) * 100;
  }

  const docTrend = calcTrend(last7, prev7);

  return [
    {
      label: t('admin.k99'),
      value: d?.total_documents ?? 0,
      icon: 'FileText',
      trend: docTrend,
    },
    {
      label: t('admin.k96'),
      value: d?.total_spaces ?? 0,
      icon: 'LayoutGrid',
    },
    {
      label: t('admin.k136'),
      value: d?.total_users ?? 0,
      icon: 'Users',
    },
    {
      label: t('admin.k137'),
      value: d?.active_users ?? 0,
      icon: 'Eye',
    },
  ];
});

const activityHeatmapData = computed(() => {
  const data: { date: string; count: number; intensity: number }[] = [];
  const trend = dashboard.value?.document_trend ?? [];
  if (trend.length === 0) return data;
  const maxCount = Math.max(...trend.map(x => x.count), 1);
  for (let i = Math.max(0, trend.length - 28); i < trend.length; i++) {
    const t = trend[i];
    if (!t) continue;
    data.push({
      date: t.date || `Day ${i + 1}`,
      count: t.count,
      intensity: maxCount > 0 ? t.count / maxCount : 0,
    });
  }
  return data;
});

const hotDocuments = computed(() => {
  const docs = dashboard.value?.hot_documents ?? [];
  if (docs.length > 0) {
    return docs.slice(0, 10).map(d => ({
      name: d.name || t('common.untitled'),
      count: d.count,
    }));
  }
  return [];
});

const ONLINE_THRESHOLD_MINUTES = 30;

const activeUsersTop10 = computed(() => {
  const contributors = dashboard.value?.active_contributors ?? [];
  if (contributors.length > 0) {
    return contributors.slice(0, 10).map((c) => ({
      username: c.username,
      given_name: c.given_name || c.username,
      count: c.count,
      is_online: c.is_delete !== 1 && c.is_forbidden !== 1 && (c.last_time ?? 0) > 0 && (c.last_time ?? 0) >= Date.now() / 1000 - ONLINE_THRESHOLD_MINUTES * 60,
    }));
  }
  return [];
});

interface SystemInfoItem {
  label: string;
  value: string;
  icon: Component;
  id: number;
  color: string;
}

const systemInfoItems = computed<SystemInfoItem[]>(() => {
  const s = serverStatus.value;
  if (!s) return [];
  return [
    { label: t('admin.k138'), value: `${s.platform} ${s.platform_family}`, icon: icons['Globe'], id: 1, color: cv('blue') },
    { label: t('admin.k139'), value: s.hostname || '--', icon: icons['Server'], id: 2, color: cv('purple') },
    { label: t('admin.k140'), value: s.local_ip || '--', icon: icons['HardDrive'], id: 3, color: cv('teal') },
    { label: t('admin.k141'), value: formatDuration(s.uptime), icon: icons['Clock'], id: 4, color: cv('green') },
    { label: t('admin.k142'), value: `${formatBytes(s.memory_used)} / ${formatBytes(s.memory_total)}`, icon: icons['Cpu'], id: 5, color: cv('orange') },
    { label: t('admin.k143'), value: `${formatBytes(s.disk_used)} / ${formatBytes(s.disk_total)}`, icon: icons['HardDrive'], id: 6, color: cv('pink') },
  ];
});

const serverItems = computed<ServerItem[]>(() => {
  // kind 由这里说清楚，组件不再按 label 的中文字猜（英文界面下那种猜永远落空）。
  const s = serverStatus.value;
  const rows: Array<{ kind: ServerItem['kind']; label: string; percent: number }> = [
    { kind: 'cpu', label: t('admin.k144'), percent: s?.cpu_percent ?? 0 },
    { kind: 'memory', label: t('admin.k145'), percent: s?.memory_percent ?? 0 },
    { kind: 'disk', label: t('admin.k146'), percent: s?.disk_percent ?? 0 },
  ];
  return rows.map((row) => ({
    kind: row.kind,
    label: row.label,
    value: row.percent,
    display: s ? formatPercentValue(row.percent, 'percent') : '--',
  }));
});

const trendData = computed(() => {
  if (!dashboard.value?.document_trend) return [];
  return dashboard.value.document_trend.map((item) => ({ label: item.date || '', value: item.count }));
});

const spaceRankData = computed(() => dashboard.value?.space_rank ?? []);

async function pollServerStatus() {
  try {
    serverStatus.value = await getServerStatus();
  } catch (error: unknown) {
    logger.error('Failed to poll server status:', error);
    ui.showToast(t('admin.k147'), 'error');
  }
}

async function fetchDashboardData() {
  loading.value = true;
  loadError.value = '';
  try {
    const [dashboardResult, serverResult, errorsResult] = await Promise.allSettled([
      getDashboardStats(30),
      getServerStatus(),
      getRecentErrors(5),
    ]);

    let hasError = false;
    if (dashboardResult.status === 'fulfilled') {
      dashboard.value = dashboardResult.value;
    } else {
      hasError = true;
      logger.error('Failed to load dashboard stats:', dashboardResult.reason);
    }

    if (serverResult.status === 'fulfilled') {
      serverStatus.value = serverResult.value;
    } else {
      hasError = true;
      logger.error('Failed to load server status:', serverResult.reason);
    }

    if (errorsResult.status === 'fulfilled') {
      recentErrors.value = errorsResult.value;
    } else {
      hasError = true;
      logger.error('Failed to load recent errors:', errorsResult.reason);
    }

    if (hasError) {
      loadError.value = t('sentUi.k13');
    }
  } catch {
    loadError.value = t('sentUi.k14');
  } finally {
    loading.value = false;
  }
}

function handleVisibilityChange() {
  if (document.hidden) {
    if (pollTimer) { clearInterval(pollTimer); pollTimer = null; }
  } else if (!pollTimer) {
    pollServerStatus();
    pollTimer = setInterval(pollServerStatus, 5000);
  }
}

onMounted(async () => {
  await fetchDashboardData();
  pollTimer = setInterval(pollServerStatus, 5000);
  document.addEventListener('visibilitychange', handleVisibilityChange);
});

onUnmounted(() => {
  if (pollTimer) {
    clearInterval(pollTimer);
    pollTimer = null;
  }
  document.removeEventListener('visibilitychange', handleVisibilityChange);
});
</script>
