<template>
  <div :style="cardStyle">
    <div v-if="data.title" :style="titleStyle">{{ data.title }}</div>
    <!-- echarts 画的是 canvas，读屏拿不到任何东西；容器给 role=img + 标题当文字替代 -->
    <div ref="chartRef" :style="chartStyle" role="img" :aria-label="data.title || $t('fallback.k44')"></div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onBeforeUnmount, watch, nextTick } from 'vue';
import * as echarts from 'echarts/core';
import { BarChart, LineChart, PieChart, ScatterChart } from 'echarts/charts';
import { TitleComponent, GridComponent, TooltipComponent, LegendComponent, DataZoomComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import { cv, AppleSpace, AppleType, AppleFontWeight } from '@/utils/core/apple-tokens';
import { streamCardStyle as cardStyle } from '@/styles/ai/ai-page-styles';
// P1-7: 模型可控的整份 option 必须先净化再进 setOption（echarts tooltip 默认 innerHTML 渲染）
import { sanitizeEChartsOption } from '@/utils/core/sanitize-echarts-option';
import { applyChartTheme } from '@/utils/core/echarts-theme';
import { useChartLifecycle } from '@/composables/ui/useChartLifecycle';

interface StreamComponent {
  id: string;
  type: string;
  title?: string;
  chart_type?: string;
  option?: Record<string, unknown>;
  height?: number;
  [key: string]: unknown;
}

const props = defineProps<{
  data: StreamComponent;
}>();

echarts.use([
  BarChart, LineChart, PieChart, ScatterChart,
  TitleComponent, GridComponent, TooltipComponent, LegendComponent,
  DataZoomComponent,
  CanvasRenderer,
]);

const chartRef = ref<HTMLElement | null>(null);
let chartInstance: echarts.ECharts | null = null;

function renderChart() {
  if (!chartRef.value || !props.data.option) return;
  if (chartInstance) {
    chartInstance.dispose();
  }
  chartInstance = echarts.init(chartRef.value);
  // P1-7: 净化后的 option 才允许交给 setOption
  const safeOption = sanitizeEChartsOption(props.data.option);
  if (!safeOption) return;
  // 主题默认值按现读的令牌补（模型显式给的颜色仍然赢）—— canvas 不会自己换配色
  chartInstance.setOption(applyChartTheme(safeOption));
}

function handleResize() {
  chartInstance?.resize();
}

// P1-28: 卸载竞态守卫——监听器改为 await 之前同步注册；await 之后若已卸载则不再渲染
let unmounted = false;
// 主题订阅 + resize + 卸载清理收口进 useChartLifecycle
useChartLifecycle({ onResize: handleResize, onThemeChange: () => renderChart() });

onMounted(async () => {
  await nextTick();
  if (unmounted) return;
  renderChart();
});

onBeforeUnmount(() => {
  unmounted = true;
  if (chartInstance) {
    chartInstance.dispose();
    chartInstance = null;
  }
});

watch(() => props.data.option, async () => {
  await nextTick();
  if (unmounted) return;
  renderChart();
}, { deep: true });

const titleStyle = {
  fontSize: AppleType.bodyPrimary.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('text-primary'),
  marginBottom: AppleSpace.xxs,
};

const chartStyle = computed(() => ({
  width: '100%',
  height: `${props.data.height || 260}px`,
}));
</script>
