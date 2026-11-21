<template>
  <div :style="kpiGridStyle">
    <ApStatCard
      v-for="stat in stats"
      :key="stat.label"
      :label="stat.label"
      :value="stat.value"
      :icon="stat.icon"
      :trend="stat.trend"
      size="compact"
    />
  </div>
</template>

<script setup lang="ts">
import { computed, type Component } from 'vue';
import { crudStatsRowFitStyle } from '@/styles/layout/page-styles';
import { useBreakpoint } from '@/utils/responsive';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import type { IconName } from '@/utils/icons';

export interface StatWithTrend {
  label: string;
  value: number;
  icon: Component | IconName;
  trend?: number;
}

const props = defineProps<{
  stats: StatWithTrend[];
}>();

const { isMobile } = useBreakpoint();

// 仪表盘 KPI 恰好与下方「图表 8 栏 + 快捷操作 4 栏」同宽布局：
// 用 fit 版（固定 N 等分）让 4 张卡左右缘与面板对齐。
// 其余 CRUD 页仍用 crudStatsRowStyle 的固定轨道——统计数量会变，铺满会拉成通栏。
// 父级 pageContentStyle 是 flex gap 布局，去掉共享样式的 marginBottom 防止间距翻倍
const kpiGridStyle = computed(() => {
  const fit = crudStatsRowFitStyle(props.stats.length, isMobile.value, 'compact');
  delete fit.marginBottom;
  return fit;
});
</script>
