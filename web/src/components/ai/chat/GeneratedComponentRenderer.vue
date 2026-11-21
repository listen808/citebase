<template>
  <div :style="rootStyle">
    <div
      v-for="(comp, i) in components"
      :key="i"
      :style="cardStyle"
    >
      <div v-if="comp.title" :style="titleStyle">{{ comp.title }}</div>

      <!-- 图表 -->
      <div v-if="comp.kind === 'chart'" :ref="(el) => setChartRef(el, i)" :style="chartCanvasStyle" />

      <!-- 表格 -->
      <div v-else-if="comp.kind === 'table'">
        <ApTable :columns="tableColumns(comp)" :data="tableData(comp)" />
      </div>

      <!-- 卡片 -->
      <div v-else-if="comp.kind === 'card'">
        <div v-for="(val, key) in comp.card_fields" :key="key" :style="fieldRowStyle">
          <span :style="fieldKeyStyle">{{ key }}</span>
          <span :style="fieldValStyle">{{ val }}</span>
        </div>
      </div>

      <!-- 代码 -->
      <pre v-else-if="comp.kind === 'code'" :style="codeStyle"><code>{{ comp.code }}</code></pre>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, onMounted, nextTick, watch, onBeforeUnmount } from 'vue';
import * as echarts from 'echarts/core';
import { BarChart, LineChart, PieChart } from 'echarts/charts';
import { TitleComponent, GridComponent, TooltipComponent, LegendComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';
import ApTable from '@/components/apple/ApTable.vue';
import { cv, AppleSpace, AppleRadius, AppleType, AppleFontWeight, AppleLineHeight } from '@/utils/core/apple-tokens';
// P1-7: 与 StreamChart 共用同一份 option 净化，堵住模型可控 formatter 的 innerHTML 注入面
import { sanitizeEChartsOption } from '@/utils/core/sanitize-echarts-option';
import { applyChartTheme } from '@/utils/core/echarts-theme';
import { useChartLifecycle } from '@/composables/ui/useChartLifecycle';
import type { GenComponent } from '@/composables/ai/useChatMessages';

const props = defineProps<{ components: GenComponent[] }>();

echarts.use([
  BarChart, LineChart, PieChart,
  TitleComponent, GridComponent, TooltipComponent, LegendComponent,
  CanvasRenderer,
]);

const chartEls = ref<(HTMLElement | null)[]>([]);
const chartInstances: echarts.ECharts[] = [];

function setChartRef(el: unknown, i: number) {
  chartEls.value[i] = el as HTMLElement | null;
}

function tableColumns(c: GenComponent) {
  return (c.table_columns || []).map((col) => ({ key: col, title: col, dataIndex: col }));
}
function tableData(c: GenComponent) {
  const cols = c.table_columns || [];
  return (c.table_rows || []).map((row) => {
    const obj: Record<string, unknown> = {};
    cols.forEach((col, idx) => { obj[col] = row[idx]; });
    return obj;
  });
}

function disposeCharts() {
  chartInstances.forEach((inst) => {
    inst.dispose();
  });
  chartInstances.length = 0;
}

// ECharts 在 init 时快照容器宽度；只看 window.resize 无法响应侧栏折叠等容器宽度变化
let chartResizeObserver: ResizeObserver | null = null;

function ensureChartResizeObserver() {
  if (chartResizeObserver || typeof ResizeObserver === 'undefined') return;
  chartResizeObserver = new ResizeObserver(() => {
    chartInstances.forEach((inst) => inst.resize());
  });
}

function renderCharts() {
  disposeCharts();
  ensureChartResizeObserver();
  chartResizeObserver?.disconnect();
  props.components.forEach((c, i) => {
    if (c.kind !== 'chart') return;
    const el = chartEls.value[i];
    if (!el || !c.chart_option) return;
    const safeOption = sanitizeEChartsOption(c.chart_option);
    if (!safeOption) return;
    const inst = echarts.init(el);
    // 主题默认值按现读令牌补上：canvas 不跟着 CSS 变量重画，切深色后轴标签会压在深面上看不见
    inst.setOption(applyChartTheme(safeOption) as echarts.EChartsCoreOption);
    chartInstances.push(inst);
    chartResizeObserver?.observe(el);
  });
}

const rootStyle = { display: 'flex', flexDirection: 'column' as const, gap: AppleSpace.sm, marginTop: AppleSpace.xs };
const cardStyle = {
  background: cv('bg-secondary'), borderRadius: AppleRadius.medium, padding: AppleSpace.sm,
};
const titleStyle = { fontSize: AppleType.bodyPrimary.size, fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary'), marginBottom: AppleSpace.xxs };
const chartCanvasStyle = { width: '100%', height: '260px' };
const fieldRowStyle = { display: 'flex', gap: AppleSpace.sm, padding: `${AppleSpace.xs} 0`, borderBottom: `1px solid ${cv('border-light')}` };
const fieldKeyStyle = { width: '90px', flexShrink: '0', color: cv('text-tertiary'), fontSize: AppleType.microUI.size };
const fieldValStyle = { color: cv('text-primary'), fontSize: AppleType.microUI.size };
const codeStyle = {
  background: cv('code-bg'), color: cv('code-fg'), borderRadius: AppleRadius.small, padding: `${AppleSpace['12px']} ${AppleSpace.md}`,
  margin: 0, overflowX: 'auto', fontSize: AppleType.microUIBold.size, lineHeight: AppleLineHeight.loose,
  fontFamily: 'SF Mono, Fira Code, Consolas, monospace',
} as const;

// 主题/材质/透明变体一换就重画（renderCharts 本来就是 dispose + init，重画是幂等的）；
// 主题订阅 + 卸载清理收口进 useChartLifecycle（resize 走下面的 ResizeObserver，不用 window 级）
useChartLifecycle({ onThemeChange: () => renderCharts() });

onMounted(async () => {
  await nextTick();
  renderCharts();
});

watch(() => props.components, async () => {
  await nextTick();
  renderCharts();
}, { deep: true });

onBeforeUnmount(() => {
  disposeCharts();
});
</script>
