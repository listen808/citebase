<template>
  <ApPageContainer :title="$t('aiView.k234')" :subtitle="$t('aiView.k236')" :show-breadcrumb="true" :max-width="AppleLayout.maxWideContentWidth" compact>
    <!-- 统计概览 -->
    <div :style="crudStatsRowFitStyle(3, isMobile, 'compact')">
      <ApStatCard size="compact" :label="$t('aiView.k231')" :value="graphData.nodes?.length || 0" :icon="'Network'" :color="cv('blue')" />
      <ApStatCard size="compact" :label="$t('aiView.k232')" :value="graphData.edges?.length || 0" :icon="'Share2'" :color="cv('green')" />
      <ApStatCard size="compact" :label="$t('aiView.k233')" :value="topCategories.length" :icon="'Tags'" :color="cv('purple')" />
    </div>

    <!-- 图谱容器 -->
    <div :style="graphContainerStyle">
      <div v-if="loading" :style="loadingStyle">
        <span :style="loadingDotStyle" />
        <span :style="loadingTextStyle">{{ $t('aiView.k230') }}</span>
      </div>
      <ApEmpty
        v-else-if="!graphData.nodes?.length"
        :icon="'Network'"
        :title="$t('aiView.k235')"
        :description="$t('vAiUi.k30')"
        :action-text="$t('vAiUi.k235')"
        @action="goAnalysis"
      />
      <!-- P1-5：关系为 0 时把 181 个实体排在同一个圆环上只会得到一团重叠标签，
           换成引导态（关系还没抽出来），而不是照常绘制一张读不出信息的图 -->
      <ApEmpty
        v-else-if="!graphData.edges?.length"
        :icon="'Share2'"
        :title="$t('aiView.k238')"
        :description="$t('aiView.k239', { n: graphData.nodes.length })"
        :action-text="$t('vAiUi.k235')"
        @action="goAnalysis"
      />
      <div v-else ref="chartRef" :style="chartStyle" />
    </div>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, onMounted, onBeforeUnmount, nextTick, watch } from 'vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApEmpty from '@/components/apple/ApEmpty.vue';
import { cv, resolvedColor, AppleLayout, AppleSpace, AppleType, AppleRadius, AppleAnimation } from '@/utils/core/apple-tokens';
import { useChartLifecycle } from '@/composables/ui/useChartLifecycle';
import { crudStatsRowFitStyle } from '@/styles/layout/page-styles';
import { useBreakpoint } from '@/utils/responsive';
import ApStatCard from '@/components/apple/ApStatCard.vue';
import { withAlpha, escapeHtml } from '@/utils/core/shared';
import { logger } from '@/utils/logger';
import { useUIStore } from '@/stores/app/ui';
import { getKnowledgeGraph, type KnowledgeGraphData, type KGNode, type KGEdge } from '@/api/modules/ai/ai-knowledge-graph';
import { useRoute, useRouter } from 'vue-router';
import * as echarts from 'echarts/core';
import { GraphChart } from 'echarts/charts';
import { TooltipComponent, LegendComponent } from 'echarts/components';
import { CanvasRenderer } from 'echarts/renderers';

// ECharts 只吃数字，不认 CSS 变量；从字号/圆角令牌里取数值，
// 避免主题换字号时图谱还停在 10/12/13 这三个字面值上。
const px = (size: string) => Number.parseInt(size, 10);

echarts.use([GraphChart, TooltipComponent, LegendComponent, CanvasRenderer]);

const { isMobile } = useBreakpoint();

const route = useRoute();
const router = useRouter();
const ui = useUIStore();

function goAnalysis() {
  router.push('/ai/settings?section=analysis');
}
const chartRef = ref<HTMLElement | null>(null);
const graphData = ref<KnowledgeGraphData>({ nodes: [], edges: [] });
const loading = ref(true);
const topCategories = ref<string[]>([]);

let chartInstance: ReturnType<typeof echarts.init> | null = null;

async function loadGraph() {
  loading.value = true;
  try {
    const spaceId = route.query.space_id ? Number(route.query.space_id) : undefined;
    const res = await getKnowledgeGraph(spaceId);
    // 兼容两种返回格式：API 可能包装在 data 字段中
    const response = res as unknown;
    const body = (response as { data?: { nodes?: unknown[]; edges?: unknown[] } }).data || (response as { nodes?: unknown[]; edges?: unknown[] });
    const rawNodes = Array.isArray((body as { nodes?: unknown[] }).nodes) ? (body as { nodes?: unknown[] }).nodes! as KGNode[] : [];
    const rawEdges = Array.isArray((body as { edges?: unknown[] }).edges) ? (body as { edges?: unknown[] }).edges! as KGEdge[] : [];
    graphData.value = { nodes: rawNodes, edges: rawEdges };
    // 统计类别
    const catSet = new Set<string>();
    for (const n of graphData.value.nodes) {
      if (n.type) catSet.add(String(n.type));
    }
    topCategories.value = Array.from(catSet).slice(0, 5);
  } catch (e: unknown) {
    logger.error('加载知识图谱失败', e);
    ui.showToast(t('aiView.k237'), 'error');
    graphData.value = { nodes: [], edges: [] };
  }
  loading.value = false;
  await nextTick();
  renderChart();
}

function renderChart() {
  refreshPalette();
  if (!chartRef.value || !graphData.value.nodes?.length) return;
  // 复用实例：避免重绘时 dispose+init 造成闪屏与 roam 状态丢失
  if (!chartInstance) chartInstance = echarts.init(chartRef.value);

  const allNodes = graphData.value.nodes;
  const allEdges = graphData.value.edges;
  const nodeCount = allNodes.length;
  const hasEdges = allEdges.length > 0;

  // 计算连接度
  const degreeMap = new Map<string, number>();
  for (const e of allEdges) {
    degreeMap.set(String(e.source), (degreeMap.get(String(e.source)) || 0) + 1);
    degreeMap.set(String(e.target), (degreeMap.get(String(e.target)) || 0) + 1);
  }
  const maxDegree = Math.max(1, ...degreeMap.values());

  // 色彩只携带语义：前 5 个类别各得一色，无类别/长尾类别一律中性灰。
  // 不做哈希上色——按名称随机分色让颜色不携带任何信息，画面只会更乱。
  function getNodeColor(n: KGNode): string {
    if (n.type && n.type !== 'unknown' && topCategories.value.includes(String(n.type))) {
      const idx = topCategories.value.indexOf(String(n.type));
      return palette[idx]!;
    }
    return resolvedColor('mid-border-gray');
  }

  // 节点大小：有边时按连接度，无边时按名称长度哈希差异化
  function getNodeSize(n: KGNode): number {
    const base = nodeCount > 50 ? 30 : nodeCount > 20 ? 38 : 46;
    const max = nodeCount > 50 ? 64 : nodeCount > 20 ? 76 : 84;
    if (hasEdges) {
      const degree = degreeMap.get(String(n.id)) || 1;
      const ratio = 0.35 + 0.65 * (degree / maxDegree);
      return Math.round(base + (max - base) * ratio);
    }
    // 无边时用名称哈希产生 0.6~1.0 的大小差异，避免全一样大
    const hash = simpleHash(n.label || String(n.id));
    const ratio = 0.55 + 0.45 * ((hash % 100) / 100);
    return Math.round(base + (max - base) * ratio);
  }

  // 无边时用环形布局，有边时用力导向
  const layout = hasEdges ? 'force' as const : 'circular' as const;

  const nodes = allNodes.map(n => {
    const color = getNodeColor(n);
    const size = getNodeSize(n);

    return {
      id: n.id,
      name: n.label || n.id,
      category: n.type || t('fallback.k72'),
      symbolSize: size,
      itemStyle: {
        color,
        borderColor: cv('surface-panel'),
        borderWidth: 2.5,
      },
      label: {
        show: true,
        position: 'bottom' as const,
        distance: 8,
        fontSize: px(AppleType.microUI.size),
        fontWeight: 500,
        color: resolvedColor('text-primary'),
        formatter: (params: { name?: string }) => {
          const name = params.name || '';
          return name.length > 8 ? name.slice(0, 8) + '…' : name;
        },
        textBorderColor: withAlpha(resolvedColor('surface-panel'), 0.9),
        textBorderWidth: 3,
        textShadowColor: withAlpha(resolvedColor('text-primary'), 0.08),
        textShadowBlur: 6,
      },
      emphasis: {
        scale: 1.2,
        itemStyle: {
          borderWidth: 3,
        },
        label: {
          fontSize: px(AppleType.microUIBold.size),
          fontWeight: 600,
          formatter: (params: { name?: string }) => params.name || '',
        },
      },
    };
  });

  const links = allEdges.map(e => {
    const sourceNode = allNodes.find(n => String(n.id) === String(e.source));
    const color = (sourceNode ? getNodeColor(sourceNode) : palette[0]) ?? resolvedColor('mid-border-gray');
    return {
      source: e.source,
      target: e.target,
      value: e.confidence || 1,
      label: {
        show: nodeCount <= 20 && allEdges.length <= 30,
        formatter: e.label || '',
        fontSize: px(AppleType.legalMeta.size),
        color: resolvedColor('text-tertiary'),
        backgroundColor: withAlpha(resolvedColor('surface-panel'), 0.9),
        padding: [2, 6],
        borderRadius: px(AppleRadius.micro),
      },
      lineStyle: {
        opacity: 0.65,
        width: (e.confidence || 0.5) * 2 + 0.5,
        color: withAlpha(color, 0.31),
        curveness: 0.2,
      },
      emphasis: {
        lineStyle: {
          opacity: 1,
          width: 3,
          color: withAlpha(color, 0.67),
        },
      },
    };
  });

  const categories = topCategories.value.map((name, i) => ({
    name,
    itemStyle: { color: palette[i]! },
  }));

  const forceConfig = hasEdges ? {
    repulsion: nodeCount > 50 ? [350, 900] : nodeCount > 20 ? [280, 650] : [220, 520],
    edgeLength: nodeCount > 50 ? [160, 340] : nodeCount > 20 ? [140, 280] : [110, 220],
    gravity: 0.1,
    edgeForce: 0.15,
    friction: 0.2,
    layoutAnimation: true,
  } : undefined;

  const circularConfig = !hasEdges ? {
    rotateLabel: false,
  } : undefined;

  chartInstance.setOption({
    backgroundColor: {
      type: 'radial' as const,
      x: 0.5,
      y: 0.5,
      r: 0.8,
      colorStops: [
        { offset: 0, color: resolvedColor('bg-tertiary') },
        { offset: 1, color: resolvedColor('surface-panel') },
      ],
    },
    tooltip: {
      // 材质整包交给 extraCssText 走令牌：原先这里把 surface-elevated 用 withAlpha 拧成 0.96
      // （elevated 是「内容滚到顶栏底下」那一档，不是浮层档），边框取中性 border、圆角写死 10px，
      // 于是全站唯一一块"看着像菜单、参数谁都不是"的面。tooltip 属浮层家族，与菜单同档。
      backgroundColor: 'transparent',
      borderWidth: 0,
      padding: [12, 16],
      textStyle: { color: resolvedColor('text-primary'), fontSize: px(AppleType.microUI.size) },
      extraCssText: 'background: var(--apple-surface-float); border: 0.5px solid var(--apple-glass-rim); border-radius: var(--apple-radius-card); backdrop-filter: var(--apple-glass-blur); -webkit-backdrop-filter: var(--apple-glass-blur); box-shadow: var(--apple-shadow-modal);',
      formatter: (params: unknown) => {
        const p = params as { dataType?: string; data?: { name?: string; category?: string; value?: number } };
        if (p.dataType === 'node') {
          const degree = degreeMap.get(p.data?.name || '') || 0;
          // P1-7: 节点名/类型来自 AI 抽取的文档内容，HTML 字符串 tooltip 必须转义，否则是注入面
          return `<div style="font-weight:600;font-size:${AppleType.controlLabel.size};margin-bottom:6px;">${escapeHtml(p.data?.name || '')}</div>
                  <div style="color:${cv('text-tertiary')};font-size:${AppleType.caption.size};line-height:1.6;">
                    类型：${escapeHtml(p.data?.category || t('fallback.k72'))}<br/>
                    ${hasEdges ? `连接数：${degree}` : ''}
                  </div>`;
        }
        return '';
      },
    },
    legend: topCategories.value.length > 0 ? {
      data: topCategories.value,
      orient: 'vertical' as const,
      right: 20,
      top: 20,
      itemWidth: 10,
      itemHeight: 10,
      itemGap: 12,
      textStyle: { fontSize: px(AppleType.microUI.size), color: resolvedColor('text-tertiary') },
      backgroundColor: withAlpha(resolvedColor('surface-panel'), 0.88),
      padding: [14, 18],
      borderRadius: px(AppleRadius.medium),
      borderColor: withAlpha(resolvedColor('text-primary'), 0.06),
      borderWidth: 1,
      shadowColor: withAlpha(resolvedColor('text-primary'), 0.08),
      shadowBlur: 12,
    } : undefined,
    series: [{
      type: 'graph',
      layout,
      roam: true,
      draggable: true,
      ...(categories.length > 0 ? { categories } : {}),
      nodes,
      edges: links,
      ...(forceConfig ? { force: forceConfig } : {}),
      ...(circularConfig ? { circular: circularConfig } : {}),
      emphasis: {
        focus: hasEdges ? 'adjacency' as const : 'self' as const,
        lineStyle: { width: 3 },
      },
      label: { show: true },
      labelLayout: { hideOverlap: true, moveOverlap: 'shiftY' as const },
      lineStyle: { curveness: 0.2 },
      zlevel: 1,
      animationDuration: 1500,
      animationEasingUpdate: 'cubicInOut',
    }],
  });

  // P1-78: 复用实例时先移除旧 click 监听再注册，否则切主题/切空间每次渲染都叠加一个处理器
  chartInstance.off('click');
  chartInstance.on('click', (params: unknown) => {
    const p = params as { dataType?: string; data?: { name?: string } };
    if (p.dataType === 'node') {
      chartInstance?.dispatchAction({ type: 'highlight', seriesIndex: 0, name: p.data?.name });
    }
  });
}

// canvas 渲染不支持 CSS 变量：色板必须在渲染时从主题令牌解析，暗色下才会翻转。
// 5 个语义色与 topCategories 上限一一对位；长尾类别走中性灰，不扩色板。
const PALETTE_NAMES = ['blue', 'green', 'orange', 'purple', 'teal'] as const;
const palette: string[] = [];
function refreshPalette() {
  palette.length = 0;
  PALETTE_NAMES.forEach((n) => palette.push(resolvedColor(n)));
}

function simpleHash(str: string): number {
  let hash = 0;
  for (let i = 0; i < str.length; i++) {
    const char = str.charCodeAt(i);
    hash = ((hash << 5) - hash) + char;
    hash = hash & hash;
  }
  return Math.abs(hash);
}

// 响应窗口大小
function handleResize() { chartInstance?.resize(); }
// 主题订阅走全站那一个出口：这一页 20+ 处 resolvedColor('surface-panel' / 'surface-elevated' / …)，
// 而那些键在 glass ↔ solid 之间**是会变的** —— 原先只 watch(ui.theme)，切材质时画布留着旧配色的表面色。
// 主题订阅 + resize + 卸载清理收口进 useChartLifecycle（此前与另两个图表组件各写一遍）。
useChartLifecycle({ onResize: handleResize, onThemeChange: () => renderChart() });
onMounted(() => {
  loadGraph();
});
onBeforeUnmount(() => {
  chartInstance?.dispose();
});

watch(() => route.query.space_id, () => loadGraph());

// === Styles ===
const graphContainerStyle = {
  background: cv('surface-panel'), borderRadius: AppleRadius.card,
  border: `1px solid ${cv('border-light')}`, overflow: 'hidden' as const,
  minHeight: '500px', position: 'relative' as const,
  boxShadow: cv('shadow-card'),
};
const chartStyle = { width: '100%', height: '520px' };
const loadingStyle = {
  display: 'flex', flexDirection: 'column' as const, alignItems: 'center', justifyContent: 'center',
  height: '400px', gap: AppleSpace.sm,
};
const loadingDotStyle = {
  width: '32px', height: '32px', borderRadius: AppleRadius.circle,
  border: `3px solid ${cv('border-light')}`, borderTopColor: cv('blue'),
  animation: `${AppleAnimation.spin} var(--apple-spinner-duration) linear infinite`,
};
const loadingTextStyle = { fontSize: AppleType.bodyPrimary.size, color: cv('text-secondary') };
</script>

