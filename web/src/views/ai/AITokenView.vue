<template>
  <div :style="aiSubPageStyle">
    <ApSkeleton v-if="loading" variant="card" :lines="4" />
    <template v-else>
    <!-- 用量概览 -->
    <div :style="compactStatsRowStyle">
      <ApCard size="small">
        <div :style="compactOverviewHeaderStyle">
          <span :style="overviewTitleStyle">{{ $t('aiSetNav.k80') }}</span>
          <span :style="usageBadgeStyle(usageStatus)">
            {{ usageStatus === 'none' ? $t('vAiUi.k78') : usageStatus === 'normal' ? $t('common.normal') : usageStatus === 'warning' ? $t('vAiUi.k76') : $t('vAiUi.k77') }}
          </span>
        </div>
        <div :style="compactUsageAmountStyle">{{ monthlyUsage.toLocaleString() }}</div>
        <div :style="compactUsageUnitStyle">tokens</div>
        <ApProgress :value="usagePercent" :label="$t('aiSetNav.k89')" :tone="toneForLoad(usagePercent)" :style="{ width: '100%' }" />
        <div :style="usageLabelStyle">
          <span>{{ $t('vAiUi.k237', { n: usagePercent }) }}</span>
          <span>{{ $t('vAiUi.k238', { n: monthlyLimit > 0 ? monthlyLimit.toLocaleString() : $t('vAiUi.k78') }) }}</span>
        </div>
      </ApCard>

      <div :style="compactQuickStatsStyle">
        <div v-for="qs in quickStats" :key="qs.label" :style="compactQuickStatItemStyle">
          <div :style="statNumberStyle">{{ qs.value }}</div>
          <div :style="quickStatLabelStyle">{{ qs.label }}</div>
        </div>
      </div>
    </div>

    <!-- 按模型分组 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderGapStyle">
          <ApIcon name="BarChart3" :size="16" :color="cv('blue')" />
          <span :style="sectionTitleStyle">{{ $t('aiSetNav.k81') }}</span>
          <span :style="sectionMetaStyle">近 {{ modelStatsDays }} 天</span>
        </div>
      </template>
      <ApEmpty v-if="modelStats.length === 0" :icon="'Cpu'" :title="$t('aiSetNav.k91')" :description="$t('vAiUi.k72')" />
      <div v-else :style="compactModelGridStyle">
        <div v-for="m in modelStats" :key="m.model" :style="compactModelCardStyle">
          <div :style="compactModelCardHeaderStyle">
            <span :style="modelNameStyle">{{ m.model }}</span>
            <span :style="modelProviderStyle">{{ m.provider }}</span>
          </div>
          <div :style="compactModelUsageStyle">{{ m.total_usage.toLocaleString() }} tokens</div>
          <ApProgress :value="m.percent" :label="$t('aiSetNav.k90')" :style="{ marginBottom: AppleSpace.xxs }" />
          <div :style="compactModelMetaStyle">
            <span>{{ m.percent.toFixed(1) }}%</span>
            <span>{{ m.request_count }} {{ $t('vAiUi.k79') }}</span>
            <span>均值 {{ m.avg_usage.toLocaleString() }}</span>
          </div>
        </div>
      </div>
    </ApCard>

    <!-- 按用户分组 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderGapStyle">
          <ApIcon name="Users" :size="16" :color="cv('green')" />
          <span :style="sectionTitleStyle">{{ $t('aiSetNav.k82') }}</span>
          <span :style="sectionMetaStyle">近 {{ userStatsDays }} 天</span>
        </div>
      </template>
      <ApEmpty v-if="userStats.length === 0" :icon="'Users'" :title="$t('aiSetNav.k92')" :description="$t('vAiUi.k73')" />
      <ApTable v-else :columns="userTableColumns" :data="userStats" row-key="user_id" density="compact" />
    </ApCard>

    <!-- 每日趋势 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderGapStyle">
          <ApIcon name="TrendingUp" :size="16" :color="cv('purple')" />
          <span :style="sectionTitleStyle">{{ $t('aiSetNav.k83') }}</span>
          <span :style="sectionMetaStyle">近 {{ trendDays }} 天</span>
        </div>
      </template>
      <ApEmpty v-if="dailyTrend.length === 0" :icon="'TrendingUp'" :title="$t('aiSetNav.k93')" :description="$t('vAiUi.k74')" />
      <div v-else>
        <div :style="compactTrendChartStyle">
          <!-- P1-4：补 y 轴三档刻度（峰值 / 半值 / 0），柱高才有量级参照 -->
          <div :style="[trendBarColumnStyle, trendAxisColumnStyle]">
            <div :style="trendAxisBoxStyle" aria-hidden="true">
              <span :style="barChartAxisLabelStyle()">{{ formatNumberCompact(trendPeak) }}</span>
              <span :style="barChartAxisLabelStyle()">{{ formatNumberCompact(Math.round(trendPeak / 2)) }}</span>
              <span :style="barChartAxisLabelStyle()">0</span>
            </div>
            <div :style="trendBarLabelStyle" aria-hidden="true">&nbsp;</div>
          </div>
          <div v-for="d in dailyTrend" :key="d.date" :style="trendBarColumnStyle">
            <div :style="compactTrendBarWrapperStyle">
              <div :style="trendBarStyle(d.percent)" :title="`${d.date}: ${d.total_usage.toLocaleString()} tokens`" />
            </div>
            <div :style="trendBarLabelStyle">{{ formatShortDate(d.date) }}</div>
          </div>
        </div>
        <div :style="compactTrendSummaryStyle">
          <div :style="trendSummaryItemStyle">
            <span :style="statNumberStyle">{{ trendTotal.toLocaleString() }}</span>
            <span :style="trendSummaryLabelStyle">{{ $t('aiSetNav.k84') }}</span>
          </div>
          <div :style="trendSummaryItemStyle">
            <span :style="statNumberStyle">{{ trendAvg.toLocaleString() }}</span>
            <span :style="trendSummaryLabelStyle">{{ $t('aiSetNav.k85') }}</span>
          </div>
          <div :style="trendSummaryItemStyle">
            <span :style="statNumberStyle">{{ trendPeak.toLocaleString() }}</span>
            <span :style="trendSummaryLabelStyle">{{ $t('aiSetNav.k86') }}</span>
          </div>
          <div :style="trendSummaryItemStyle">
            <span :style="statNumberStyle">{{ totalRequests.toLocaleString() }}</span>
            <span :style="trendSummaryLabelStyle">{{ $t('aiSetNav.k87') }}</span>
          </div>
        </div>
      </div>
    </ApCard>

    <!-- 服务明细 -->
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderGapStyle">
          <ApIcon name="Cpu" :size="16" :color="cv('orange')" />
          <span :style="sectionTitleStyle">{{ $t('aiSetNav.k88') }}</span>
        </div>
      </template>
      <ApEmpty v-if="services.length === 0" :icon="'Cpu'" :title="$t('aiSetNav.k94')" :description="$t('vAiUi.k75')" />
      <ApTable v-else :columns="serviceTableColumns" :data="services" row-key="name" density="compact" />
    </ApCard>
    </template>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { logger } from '@/utils/logger';
import { ref, computed, onMounted, h } from 'vue';
import { icons } from '@/utils/icons';
import ApIcon from '@/components/apple/ApIcon.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import ApTable from '@/components/apple/ApTable.vue';
import ApProgress from '@/components/apple/ApProgress.vue';
import type { TableColumn } from '@/components/apple/ApTable.vue';

import { cv, AppleFontWeight, AppleRadius, AppleSpace, AppleType, AppleTransition, AppleLineHeight, AppleGridFloor, appleCardGrid } from '@/utils/core/apple-tokens';
import { toneForLoad } from '@/utils/core/progress-tone';
import { useBreakpoint } from '@/utils/responsive';
import { formatShortDate, formatTimeAgo, formatNumberCompact } from '@/utils/format/format-time';
import { barChartAxisLabelStyle } from '@/styles/component/chart-styles';
import { aiSubCardNameStyle, aiSubPageStyle, aiSubSectionHeaderGapStyle, aiSubSectionTitleStyle as sectionTitleStyle } from '@/styles/ai/ai-page-styles';
import { getTokenUsage, getAIDashboard, getTokenUsageByModel, getTokenUsageByUser, getTokenUsageDailyTrend } from '@/api/modules/ai/ai-dashboard';
import { useUIStore } from '@/stores/app/ui';

import type { Component } from 'vue';

const ui = useUIStore();
const { isMobile } = useBreakpoint();

const loading = ref(true);

const monthlyUsage = ref(0);
const monthlyLimit = ref(0);
const todayUsage = ref(0);
const avgDaily = ref(0);
const remainingDays = ref(0);

const modelStats = ref<Array<{
  model: string; provider: string; total_usage: number;
  request_count: number; avg_usage: number; percent: number;
}>>([]);
const modelStatsDays = ref(30);

interface UserStatRow {
  user_id: number; username: string; given_name?: string; total_usage: number;
  request_count: number; avg_usage: number; last_used: string; percent: number;
}

const userStats = ref<UserStatRow[]>([]);
const userStatsDays = ref(30);

const dailyTrend = ref<Array<{
  date: string; total_usage: number; request_count: number;
  unique_users: number; percent: number;
}>>([]);
const trendDays = ref(30);

const trendTotal = computed(() => dailyTrend.value.reduce((s, d) => s + d.total_usage, 0));
const trendAvg = computed(() => dailyTrend.value.length > 0 ? Math.round(trendTotal.value / dailyTrend.value.length) : 0);
const trendPeak = computed(() => dailyTrend.value.reduce((max, d) => d.total_usage > max ? d.total_usage : max, 0));
const totalRequests = computed(() => dailyTrend.value.reduce((s, d) => s + d.request_count, 0));

async function fetchData() {
  try {
    const [tokenRes, dashRes] = await Promise.allSettled([getTokenUsage(), getAIDashboard()]);
    if (tokenRes.status === 'fulfilled' && tokenRes.value) {
      const val = tokenRes.value as Record<string, unknown>;
      monthlyUsage.value = Number(val.total_tokens_today || val.total_tokens || val.total || 0);
      if (val.monthly_limit) monthlyLimit.value = Number(val.monthly_limit);
      if (val.today_tokens) todayUsage.value = Number(val.today_tokens);
      else if (val.by_user) todayUsage.value = Object.values(val.by_user as Record<string, number>).reduce((a: number, b: number) => a + b, 0);
      if (val.services) {
        const svcs = val.services as Array<Record<string, unknown>>;
        services.value = svcs.map(s => ({
          name: String(s.name || s.service || t('fallback.k78')),
          usage: Number(s.usage || s.tokens || 0),
          percent: Number(s.percent || s.percentage || 0),
          daily: Number(s.daily || 0),
          color: cv('blue'),
          icon: icons.Zap,
        }));
      }
    }
    if (dashRes.status === 'fulfilled' && dashRes.value) {
      const v = dashRes.value as Record<string, unknown>;
      if (v.token_usage) monthlyUsage.value = Number(v.token_usage);
      if (v.token_limit) monthlyLimit.value = Number(v.token_limit);
      if (v.today_tokens) todayUsage.value = Number(v.today_tokens);
      if (v.services) {
        const svcs = v.services as Array<Record<string, unknown>>;
        services.value = svcs.map(s => ({
          name: String(s.name || s.service || t('fallback.k78')),
          usage: Number(s.usage || s.tokens || 0),
          percent: Number(s.percent || s.percentage || 0),
          daily: Number(s.daily || 0),
          color: cv('blue'),
          icon: icons.Zap,
        }));
      }
    }
  } catch (e) {
    logger.error('Failed to fetch token usage:', e);
    ui.showToast(t('aiSetNav.k108'), 'warning');
  }

  const now = new Date();
  const daysInMonth = new Date(now.getFullYear(), now.getMonth() + 1, 0).getDate();
  remainingDays.value = daysInMonth - now.getDate();
  if (now.getDate() > 0) {
    avgDaily.value = Math.round(monthlyUsage.value / now.getDate());
  }

  const palette = [cv('blue'), cv('green'), cv('orange'), cv('purple'), cv('red'), cv('yellow')];
  services.value = services.value.map((s, i) => ({ ...s, color: palette[i % palette.length] || cv('blue') }));

  // Fetch detailed stats
  try {
    const [modelRes, userRes, trendRes] = await Promise.allSettled([
      getTokenUsageByModel({ days: 30, limit: 10 }),
      getTokenUsageByUser({ days: 30, limit: 10 }),
      getTokenUsageDailyTrend({ days: 30 }),
    ]);
    if (modelRes.status === 'fulfilled' && modelRes.value) {
      modelStats.value = modelRes.value.stats || [];
      modelStatsDays.value = modelRes.value.days || 30;
    }
    if (userRes.status === 'fulfilled' && userRes.value) {
      userStats.value = userRes.value.stats || [];
      userStatsDays.value = userRes.value.days || 30;
    }
    if (trendRes.status === 'fulfilled' && trendRes.value) {
      dailyTrend.value = trendRes.value.trend || [];
      trendDays.value = trendRes.value.days || 30;
    }
    // 三张条走的是 allSettled —— 请求失败不会落到下面的 catch，只会让对应面板停在空态，
    // 于是「这三个接口挂了」在页面上长得和「还没有用量」一模一样。
    if ([modelRes, userRes, trendRes].some((r) => r.status === 'rejected')) {
      logger.error('Failed to fetch usage details (partial):', [modelRes, userRes, trendRes].find((r) => r.status === 'rejected'));
      ui.showToast(t('aiSetNav.k109'), 'error');
    }
  } catch (e) {
    logger.error('Failed to fetch usage details:', e);
    ui.showToast(t('aiSetNav.k109'), 'error');
  }
}

onMounted(() => fetchData().finally(() => { loading.value = false; }));

const usagePercent = computed(() => monthlyLimit.value > 0 ? Math.min(Math.round((monthlyUsage.value / monthlyLimit.value) * 100), 100) : 0);
const usageStatus = computed(() => {
  // 未设额度时「正常」是假信号（审计 B15）：没有上限就不存在「用得很省」
  if (monthlyLimit.value <= 0) return 'none';
  if (usagePercent.value >= 100) return 'over';
  if (usagePercent.value >= 80) return 'warning';
  return 'normal';
});

const quickStats = computed(() => [
  { label: t('aiSetNav.k95'), value: todayUsage.value.toLocaleString() },
  { label: t('aiSetNav.k96'), value: avgDaily.value.toLocaleString() },
  { label: t('aiSetNav.k97'), value: remainingDays.value },
]);

interface ServiceRow { name: string; usage: number; percent: number; daily: number; color: string; icon: Component }

const services = ref<ServiceRow[]>([]);

// --- Layout ---
// 统计数值的唯一写法：全站数值只有 statValue(20)/600 + tabular-nums 一档
// （与 ApStatCard 的 valueStyle 同档）。此前这页在 subhead(19)/bold、
// utilityHeading(24)/bold 上各写了一份，跨页看就是同一类数字好几种尺寸。
const statNumberStyle = {
  fontSize: AppleType.statValue.size,
  fontWeight: String(AppleType.statValue.weight),
  color: cv('text-primary'),
  fontVariantNumeric: 'tabular-nums',
};

const overviewTitleStyle = {
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.medium), color: cv('text-secondary'),
};

const usageBadgeStyle = (status: string) => ({
  fontSize: AppleType.legalMeta.size,
  fontWeight: String(AppleFontWeight.medium), padding: `${AppleSpace.xs} ${AppleSpace.sm}`,
  borderRadius: AppleRadius.pill,
  backgroundColor: status === 'none' ? cv('gray-bg-subtle') : status === 'normal' ? cv('green-light') : status === 'warning' ? cv('orange-light') : cv('red-light'),
  color: status === 'none' ? cv('text-secondary') : status === 'normal' ? cv('green') : status === 'warning' ? cv('orange') : cv('red'),
});

const usageAmountStyle = {
  ...statNumberStyle,
  lineHeight: AppleLineHeight.heading,
};

const usageUnitStyle = {
  fontSize: AppleType.bodyPrimary.size,
  color: cv('text-secondary'), marginBottom: AppleSpace.lg,
};

const usageLabelStyle = {
  display: 'flex', justifyContent: 'space-between', marginTop: AppleSpace.sm,
  fontSize: AppleType.legalMeta.size, color: cv('text-secondary'),
};

const quickStatLabelStyle = {
  fontSize: AppleType.legalMeta.size, color: cv('text-secondary'),
};

// --- Section ---
const sectionMetaStyle = {
  fontSize: AppleType.legalMeta.size, color: cv('text-tertiary'), marginLeft: AppleSpace.sm,
};

// --- Model Grid ---
const modelNameStyle = {
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary'),
};

const modelProviderStyle = {
  fontSize: AppleType.legalMeta.size, color: cv('text-tertiary'),
  padding: `1px ${AppleSpace.sm}`,
  borderRadius: AppleRadius.micro,
  backgroundColor: cv('bg-tertiary'),
};

// --- User Table columns defined below (ApTable render) ---

// --- Trend Chart ---
const trendBarColumnStyle = {
  flex: 1, display: 'flex', flexDirection: 'column' as const,
  alignItems: 'center', justifyContent: 'flex-end', minWidth: 0,
};

// y 轴列与柱列同构（100px 柱区 + 一行 x 标签），刻度才与基线对齐
const trendAxisColumnStyle = { flex: '0 0 38px' };
const trendAxisBoxStyle = {
  width: '100%', height: '100px', display: 'flex', flexDirection: 'column' as const,
  justifyContent: 'space-between', alignItems: 'flex-end',
};

const trendBarStyle = (percent: number) => ({
  width: '100%', height: `${Math.max(percent, 2)}%`,
  backgroundColor: cv('blue'), borderRadius: `${AppleRadius.micro} ${AppleRadius.micro} 0 0`,
  transition: AppleTransition.smooth, cursor: 'pointer',
  minHeight: '2px',
});

const trendBarLabelStyle = {
  fontSize: AppleType.legalMeta.size, color: cv('text-tertiary'), marginTop: AppleSpace.xxs,
  whiteSpace: 'nowrap' as const, overflow: 'hidden', textOverflow: 'ellipsis',
  maxWidth: '100%', textAlign: 'center' as const,
};

const trendSummaryItemStyle = {
  display: 'flex', flexDirection: 'column' as const, alignItems: 'center',
  gap: AppleSpace.xs,
};

const trendSummaryLabelStyle = {
  fontSize: AppleType.legalMeta.size, color: cv('text-secondary'),
};

// --- Service Table ---
const compactStatsRowStyle = computed(() => ({
  display: 'grid', gridTemplateColumns: isMobile.value ? '1fr' : '1fr 240px', gap: AppleSpace.md,
}));
const compactOverviewHeaderStyle = {
  display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: AppleSpace.sm,
};
const compactUsageAmountStyle = usageAmountStyle;
const compactUsageUnitStyle = {
  ...usageUnitStyle, marginBottom: AppleSpace.md, fontSize: AppleType.legalMeta.size,
};
const compactQuickStatsStyle = {
  display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.sm,
};
const compactQuickStatItemStyle = {
  display: 'flex', alignItems: 'center', justifyContent: 'space-between',
  padding: AppleSpace.sm, backgroundColor: cv('surface-panel'),
  borderRadius: AppleRadius.medium,
  border: `1px solid ${cv('border-light')}`,
};
const compactModelGridStyle = computed(() => (isMobile.value
  ? { display: 'grid', gridTemplateColumns: '1fr', gap: AppleSpace.sm, gridAutoRows: '1fr', alignContent: 'start', alignItems: 'stretch' }
  : appleCardGrid('entity', AppleSpace.sm)));
const compactModelCardStyle = {
  padding: AppleSpace.sm,
  backgroundColor: cv('surface-panel'),
  borderRadius: AppleRadius.medium,
  border: `1px solid ${cv('border-light')}`,
};
const compactModelCardHeaderStyle = {
  display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: AppleSpace.xs,
};
// 复用共享的卡片名样式，只补本卡片特有的下间距：别再本地逐字节克隆一份
// （降字重后曾与共享样式撞成完全一致，被 check-style-tokens 的「本地克隆共享样式」拦下）
const compactModelUsageStyle = { ...aiSubCardNameStyle, marginBottom: AppleSpace.xs };
const compactModelMetaStyle = {
  display: 'flex', gap: AppleSpace.sm,
  fontSize: AppleType.legalMeta.size, color: cv('text-secondary'),
};

const compactUserAvatarStyle = {
  width: '20px', height: '20px', borderRadius: AppleRadius.circle,
  backgroundColor: cv('blue-light'), color: cv('blue'),
  display: 'flex', alignItems: 'center', justifyContent: 'center',
  fontSize: AppleType.microUI.size, fontWeight: String(AppleFontWeight.semibold), flexShrink: 0,
};
const compactTrendChartStyle = {
  display: 'flex', alignItems: 'flex-end', gap: AppleSpace.xxs,
  height: '120px', padding: `${AppleSpace.xs} 0`,
  borderBottom: `1px solid ${cv('border-light')}`, marginBottom: AppleSpace.sm,
};
const compactTrendBarWrapperStyle = {
  width: '100%', height: '100px', display: 'flex', alignItems: 'flex-end',
};
const compactTrendSummaryStyle = {
  display: 'grid', gridTemplateColumns: `repeat(auto-fit, ${AppleGridFloor.stat})`, gap: AppleSpace.sm,
};

const compactServiceIconStyle = {
  width: AppleSpace.md, height: AppleSpace.md, display: 'flex', alignItems: 'center',
  justifyContent: 'center', borderRadius: AppleRadius.small, backgroundColor: cv('blue-light'),
};

// --- ApTable 列定义 ---
const cellFlexStyle = { display: 'flex', alignItems: 'center', gap: AppleSpace.sm };
const percentLabelStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-secondary'), minWidth: '44px', textAlign: 'right' as const };
const metaTextStyle = { fontSize: AppleType.legalMeta.size, color: cv('text-secondary') };

const userTableColumns: TableColumn<UserStatRow>[] = [
  {
    key: 'user', title: t('aiSetNav.k98'),
    render: (u) => h('div', { style: cellFlexStyle }, [
      h('div', { style: compactUserAvatarStyle }, (u.given_name || u.username).charAt(0).toUpperCase()),
      h('span', {}, u.given_name || u.username),
    ]),
  },
  { key: 'total_usage', title: t('aiSetNav.k99'), render: (u) => u.total_usage.toLocaleString() },
  {
    key: 'percent', title: t('aiSetNav.k100'),
    render: (u) => h('div', { style: cellFlexStyle }, [
      h(ApProgress, { value: u.percent, label: t('aiSetNav.k101'), style: { flex: 1 } }),
      h('span', { style: percentLabelStyle }, `${u.percent.toFixed(1)}%`),
    ]),
  },
  { key: 'request_count', title: t('aiSetNav.k102'), render: (u) => String(u.request_count) },
  { key: 'avg_usage', title: t('aiSetNav.k103'), render: (u) => u.avg_usage.toLocaleString() },
  { key: 'last_used', title: t('aiSetNav.k104'), render: (u) => h('span', { style: metaTextStyle }, formatTimeAgo(u.last_used)) },
];

const serviceTableColumns: TableColumn<ServiceRow>[] = [
  {
    key: 'name', title: t('aiSetNav.k105'),
    render: (s) => h('div', { style: cellFlexStyle }, [
      h('div', { style: compactServiceIconStyle }, [h(s.icon, { size: 12, color: cv('blue') })]),
      h('span', {}, s.name),
    ]),
  },
  { key: 'usage', title: t('aiSetNav.k80'), render: (s) => s.usage.toLocaleString() },
  {
    key: 'percent', title: t('aiSetNav.k100'),
    render: (s) => h('div', { style: cellFlexStyle }, [
      h(ApProgress, { value: s.percent, label: t('aiSetNav.k106'), style: { flex: 1 } }),
      h('span', { style: percentLabelStyle }, `${s.percent}%`),
    ]),
  },
  { key: 'daily', title: t('aiSetNav.k107'), render: (s) => s.daily.toLocaleString() },
];
</script>
