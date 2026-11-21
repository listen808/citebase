<template>
  <ApCard>
    <div :style="cardHeaderStyle">
      <h2 :style="cardTitleStyle">{{ $t('admin.k108') }}</h2>
    </div>
    <div :style="chartRowStyle">
      <div v-if="trendData.length > 0" :style="chartColStyle">
        <div :style="chartSubtitleStyle">{{ $t('admin.k109') }}</div>
        <!-- P1-4：柱图补 y 轴三档刻度，否则只有柱子深浅看不出每日新增量级 -->
        <div :style="plotRowStyle">
          <div :style="[barChartAxisStyle(`${BAR_AREA_HEIGHT}px`), axisWidthStyle]" aria-hidden="true">
            <span :style="barChartAxisLabelStyle()">{{ maxTrendValue }}</span>
            <span :style="barChartAxisLabelStyle()">{{ Math.round(maxTrendValue / 2) }}</span>
            <span :style="barChartAxisLabelStyle()">0</span>
          </div>
          <div :style="barChartStyle">
            <div :style="barsRowStyle">
              <div
                v-for="(item, idx) in visibleTrend"
                :key="'bar-' + idx"
                :style="barWrapperStyle"
                :title="`${item.label} · ${item.value}`"
              >
                <div :style="getBarStyle(item.value, maxTrendValue)" />
              </div>
            </div>
            <div :style="labelsRowStyle">
              <span
                v-for="(item, idx) in visibleTrend"
                :key="'label-' + idx"
                :style="barLabelStyle"
              >{{ idx % 2 === 0 ? formatShortDate(item.label) : '' }}</span>
            </div>
          </div>
        </div>
      </div>
      <div v-if="spaceRankData.length > 0" :style="chartColStyle">
        <div :style="chartSubtitleStyle">{{ $t('admin.k110') }}</div>
        <div :style="pieChartContainerStyle">
          <div :style="[pieChartStyle(pieSegments), pieSizeStyle]" />
          <div :style="pieLegendStyle">
            <div
              v-for="(item, idx) in spaceRankData.slice(0, 6)"
              :key="idx"
              :style="pieLegendItemStyle"
            >
              <span :style="[legendDotStyle, { background: getPieColor(idx) }]" />
              <span :style="legendLabelStyle" :title="item.space_name">{{ item.space_name }}</span>
              <span :style="legendValueStyle">{{ item.total }}</span>
            </div>
          </div>
        </div>
      </div>
    </div>
  </ApCard>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { cv, AppleRadius, AppleSpace, AppleTransition, AppleType, AppleFontWeight, AppleFont, appleCardGrid } from '@/utils/core/apple-tokens';
import { formatShortDate } from '@/utils/format/format-time';
import { AppleChartColors } from '@/styles/layout/page-styles';
import { barChartAxisStyle, barChartAxisLabelStyle } from '@/styles/component/chart-styles';
import ApCard from '@/components/apple/ApCard.vue';

export interface TrendDataItem {
  label: string;
  value: number;
}

export interface SpaceRankItem {
  space_name: string;
  total: number;
}

const props = defineProps<{
  trendData: TrendDataItem[];
  spaceRankData: SpaceRankItem[];
}>();

const BAR_AREA_HEIGHT = 140;

// 只画最近 14 天：y 轴上限也必须按这 14 天算，否则刻度与柱子不是同一批数据
const visibleTrend = computed(() => props.trendData.slice(-14));

const maxTrendValue = computed(() => {
  if (!visibleTrend.value.length) return 1;
  return Math.max(...visibleTrend.value.map(d => d.value), 1);
});

const pieSegments = computed(() => {
  const data = props.spaceRankData.slice(0, 6);
  const total = data.reduce((sum, item) => sum + item.total, 0) || 1;
  return data.map((item, idx) => ({
    color: getPieColor(idx),
    percent: (item.total / total) * 100,
  }));
});

function getPieColor(idx: number): string {
  return AppleChartColors[idx % AppleChartColors.length]!;
}

function getBarStyle(value: number, max: number) {
  const percent = max > 0 ? (value / max) * 100 : 0;
  const height = Math.max(percent > 0 ? 4 : 0, (percent / 100) * BAR_AREA_HEIGHT);
  // 细柱 + 只圆上沿：28px 宽配 control 圆角是胶囊，一排并列就成色块（索引监控同一处毛病）
  return {
    width: '100%',
    maxWidth: '28px',
    height: `${height}px`,
    background: cv('blue'),
    borderTopLeftRadius: AppleRadius.micro,
    borderTopRightRadius: AppleRadius.micro,
    transition: AppleTransition.default,
  };
}

const cardHeaderStyle = {
  marginBottom: AppleSpace.md,
};

const cardTitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.subhead.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  margin: '0',
};

const chartRowStyle = appleCardGrid('panel', AppleSpace.lg);

const chartColStyle = {
  minWidth: '0',
};

const chartSubtitleStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  marginBottom: AppleSpace.sm,
};

const barChartStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
  flex: '1',
  minWidth: '0',
};

const plotRowStyle = {
  display: 'flex',
  alignItems: 'flex-start',
  gap: AppleSpace.xs,
};

const axisWidthStyle = { width: '32px' };

const barsRowStyle = {
  display: 'flex',
  alignItems: 'flex-end',
  gap: AppleSpace.xs,
  height: `${BAR_AREA_HEIGHT}px`,
};

const labelsRowStyle = {
  display: 'flex',
  gap: AppleSpace.xs,
};

const barWrapperStyle = {
  flex: '1',
  minWidth: '0',
  height: `${BAR_AREA_HEIGHT}px`,
  display: 'flex',
  alignItems: 'flex-end',
  justifyContent: 'center',
};

const barLabelStyle = {
  flex: '1',
  minWidth: '0',
  textAlign: 'center' as const,
  fontFamily: AppleFont.mono,
  fontSize: AppleType.legalMeta.size,
  letterSpacing: AppleType.legalMeta.letterSpacing,
  color: cv('text-quaternary'),
  whiteSpace: 'nowrap' as const,
};

const pieChartContainerStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.md,
};

const pieSizeStyle = {
  width: '100px',
  height: '100px',
  flexShrink: '0',
};

function pieChartStyle(segments: { color: string; percent: number }[]) {
  if (segments.length === 0) {
    return {
      borderRadius: AppleRadius.circle,
      background: cv('bg-tertiary'),
    };
  }
  let gradient = 'conic-gradient(';
  let currentAngle = 0;
  segments.forEach((seg, idx) => {
    const startAngle = currentAngle;
    currentAngle += seg.percent * 3.6;
    const endAngle = currentAngle;
    gradient += `${seg.color} ${startAngle}deg ${endAngle}deg`;
    if (idx < segments.length - 1) gradient += ', ';
  });
  gradient += ')';
  return {
    borderRadius: AppleRadius.circle,
    background: gradient,
  };
}

const pieLegendStyle = {
  flex: '1',
  minWidth: '0',
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xs,
};

const pieLegendItemStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xs,
};

const legendDotStyle = {
  width: AppleSpace.sm,
  height: AppleSpace.sm,
  borderRadius: AppleRadius.circle,
  flexShrink: '0',
};

const legendLabelStyle = {
  flex: '1',
  minWidth: '0',
  fontFamily: AppleFont.text,
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
  overflow: 'hidden',
  textOverflow: 'ellipsis',
  whiteSpace: 'nowrap' as const,
};

const legendValueStyle = {
  fontFamily: AppleFont.mono,
  fontSize: AppleType.microUI.size,
  color: cv('text-tertiary'),
  flexShrink: '0',
};
</script>
