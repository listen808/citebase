<template>
  <div :style="tableWrapperStyle">
    <table ref="tableEl" :style="tableStyle" role="table" :aria-label="ariaLabel || autoLabel || undefined" :aria-rowcount="data.length" :aria-colcount="columns.length + ($slots.actions ? 1 : 0)">
      <thead v-if="showHeader" ref="theadRef" :style="theadStyle">
        <tr>
          <th
            v-for="col in columns"
            :key="col.key"
            :style="[thStyle, colStyle(col)]"
          >
            {{ col.title }}
          </th>
          <th v-if="$slots.actions" :style="thStyle">{{ $t('apUi1.actions') }}</th>
        </tr>
      </thead>
      <tbody>
        <!-- 加载骨架屏 -->
        <template v-if="loading">
          <tr v-for="n in skeletonRows" :key="'skeleton-' + n" :style="trStyle">
            <td v-for="col in columns" :key="'sk-' + n + '-' + col.key" :style="[tdStyle, colStyle(col)]">
              <div :style="skeletonCellStyle"></div>
            </td>
            <td v-if="$slots.actions" :style="tdStyle">
              <div :style="skeletonCellStyle"></div>
            </td>
          </tr>
        </template>

        <!-- 数据行 -->
        <template v-if="!loading">
          <tr
            v-for="(row, index) in data"
            :key="((row as Record<string, unknown>)[rowKey] as PropertyKey) ?? index"
            :style="trStyle"
            :class="{ 'apple-last-row': index === data.length - 1 }"
            class="ap-table-row"
          >
            <td
              v-for="col in columns"
              :key="col.key"
              :title="cellTitle(row, col)"
              :style="[tdStyle, colStyle(col)]"
            >
              <template v-if="col.render">
                <template v-if="isTextContent(col.render(row))">
                  {{ col.render(row) }}
                </template>
                <component :is="col.render(row)" v-else />
              </template>
              <template v-else>
                {{ (row as Record<string, unknown>)[col.key] }}
              </template>
            </td>
            <td v-if="$slots.actions" :style="tdStyle">
              <slot name="actions" :row="row" />
            </td>
          </tr>
        </template>

        <!-- 空状态：默认就是一枚标准 ApEmpty，页面不必再为「表格里放什么空态」各写一套。
             flex 布局必须放在内层 div 上：直接给 <td> 设 display:flex 会让它不再是
             table-cell，colspan 随之失效，整块空态被压回第一列的宽度（实测 200px），
             文案在词中间断行。 -->
        <tr v-if="!loading && data.length === 0">
          <td :colspan="columns.length + ($slots.actions ? 1 : 0)">
            <div :style="emptyStyle">
              <slot name="empty">
                <ApEmpty :icon="emptyIcon" :title="emptyText || $t('common.noData')" />
              </slot>
            </div>
          </td>
        </tr>
      </tbody>
    </table>
  </div>
</template>

<script setup lang="ts" generic="T extends object">
import { computed, onMounted, ref, shallowRef, watch, type Component } from 'vue';
import type { IconName } from '@/utils/icons';
import { cv, AppleFont, AppleFontWeight, AppleRadius, AppleSpace, AppleType, AppleZIndex, AppleLayout, AppleAnimation, AppleTransition } from '@/utils/core/apple-tokens';
import type { AppleDensity } from '@/utils/core/apple-tokens';
import { emptyStateStyle as emptyStyle } from '@/styles/layout/page-styles';
import { useScrollEdge, shellSurfaceKey } from '@/composables/ui/useScrollEdge';

export interface TableColumn<T extends object = Record<string, unknown>> {
  key: string;
  title: string;
  render?: (row: T) => unknown;
  /** 列宽（CSS 值，如 '200px' / '20%' / 'auto'） */
  width?: string;
  /** 文本对齐方式（默认 'left'） */
  align?: 'left' | 'center' | 'right';
  /** 是否启用单行省略号（默认 false） */
  ellipsis?: boolean;
  /** 允许该列换行（默认所有单元格单行截断；长描述列才需要打开） */
  wrap?: boolean;
}

const props = withDefaults(defineProps<{
  columns: TableColumn<T>[];
  data: T[];
  loading?: boolean;
  /** 密度轴（与 ApModal 的 density 同一档语义）；原先叫 size，是命名债 */
  density?: AppleDensity;
  /** 行唯一键字段名（默认 'id'） */
  rowKey?: string;
  /** 空态文案（缺省走语言包 common.noData，所以切语言会跟着变） */
  emptyText?: string;
  /** 空态图标，取共享图标注册表的名字或组件本身 */
  emptyIcon?: Component | IconName;
  /** 表格名字。缺省时回退到文档顺序上最近的标题 */
  ariaLabel?: string;
  /** 无数据时隐藏表头：空态下 6–9 列窄表头既不可读也不携带信息（P1-9） */
  hideHeaderWhenEmpty?: boolean;
}>(), {
  density: 'default',
  rowKey: 'id',
  ariaLabel: '',
  hideHeaderWhenEmpty: false,
});

// 骨架屏阶段（loading 且尚无数据）仍要保留表头，让列结构在数据到达前就有预期
const showHeader = computed(
  () => props.columns.length > 0 && !(props.hideHeaderWhenEmpty && !props.loading && props.data.length === 0),
);

const skeletonRows = computed(() => (props.loading ? Array(5).fill(0).map((_, i) => i) : []));

function isTextContent(value: unknown): value is string | number {
  return typeof value === 'string' || typeof value === 'number';
}

/** 根据列配置生成每列的样式（width / align / 单行截断） */
function colStyle(col: TableColumn<T>): Record<string, string> {
  const style: Record<string, string> = {};
  if (col.width) {
    style.width = col.width;
    style.minWidth = col.width;
  }
  if (col.align) {
    style.textAlign = col.align;
  }
  // 默认单行截断。原先只有列显式写了 width 才带 whiteSpace，结果没写宽度的中文列
  // 在窄容器里被逐字折断（收藏页表格「所属空间」竖排成 4 行）。要换行的长文本列声明 wrap: true。
  if (col.wrap) {
    style.whiteSpace = 'normal';
  } else {
    style.whiteSpace = 'nowrap';
    style.overflow = 'hidden';
    style.textOverflow = 'ellipsis';
  }
  return style;
}

/** 文本单元格补 title，截断后完整内容仍可读到 */
function cellTitle(row: T, col: TableColumn<T>): string | undefined {
  if (col.wrap) return undefined;
  // render 返回 VNode 时（链接标题、带徽标的名称）原先直接放弃兜底，
  // 于是这些格子截断了却读不到全文。行数据本身就是字符串时，用它兜底。
  const field = (row as Record<string, unknown>)[col.key];
  if (typeof field === 'string' || typeof field === 'number') return String(field);
  const raw = col.render ? col.render(row) : field;
  return typeof raw === 'string' || typeof raw === 'number' ? String(raw) : undefined;
}

const tableEl = shallowRef<HTMLElement | null>(null);
const autoLabel = shallowRef('');

/** 表格里没写 caption 时，用文档顺序上最近的一个标题当名字。
    比自己编一句「数据表格」有用：读屏报出来的是「用户 · 共 12 项」这种真上下文。 */
function pickNearestHeading() {
  const el = tableEl.value;
  if (!el) return;
  const heads = [...document.querySelectorAll<HTMLElement>('h1,h2,h3,h4,caption,figcaption')];
  let best: HTMLElement | null = null;
  for (const h of heads) {
    if (el.compareDocumentPosition(h) & Node.DOCUMENT_POSITION_PRECEDING && !h.contains(el)) best = h;
  }
  const text = (best?.textContent || '').trim();
  if (text && text.length <= 40) autoLabel.value = text;
}

onMounted(() => { nextFrame(pickNearestHeading); });
watch(() => props.data.length, () => { if (!autoLabel.value) nextFrame(pickNearestHeading); });

function nextFrame(fn: () => void) {
  requestAnimationFrame(() => requestAnimationFrame(fn));
}

const tableWrapperStyle = computed(() => ({
  flex: '1',
  overflowY: 'auto' as const,
  overflowX: 'auto' as const,
  border: `1px solid ${cv('border-light')}`,
  borderRadius: AppleRadius.card,
  boxShadow: 'none',
}));

const tableStyle = {
  width: '100%',
  borderCollapse: 'collapse' as const,
  fontFamily: AppleFont.text };

// sticky 表头是滚动边缘效应的**真站点**：行会从它底下滚过去，所以它是"压在任意外部内容上的壳面"。
// 静止档用 bg-secondary 保持通透；一旦有行滚到下面就换浮层档（这条地板由 check-contrast 的
// 浮层断言组量：合成到极值内容上，正文 ≥7、次级/三级 ≥4.5）。
const theadRef = ref<HTMLElement | null>(null);
const theadScrollEdge = useScrollEdge(theadRef);
const theadStyle = computed(() => ({
  position: 'sticky' as const,
  top: '0',
  zIndex: String(AppleZIndex.base),
  background: cv(shellSurfaceKey(theadScrollEdge.value, 'bg-secondary')),
  // 磨砂只在这块表面真的"压住内容"时付（滚动边缘效应本体）：静止时没有行在表头底下，
  // blur 是白付的 —— 多表格页面（数据导出 3 张表）常驻 blur 把同屏层数顶到 5（红线 3，
  // 2026-10-26 全面量测实测）。under 态才付，滚动中的表头 +1 层，同屏仍 ≤3。
  ...(theadScrollEdge.value === 'under'
    ? { backdropFilter: cv('glass-blur'), WebkitBackdropFilter: cv('glass-blur') }
    : {}),
}));

const thStyle = computed(() => ({
  height: props.density === 'compact' ? AppleLayout.tableHeaderHeightCompact : AppleLayout.tableHeaderHeight,
  padding: props.density === 'compact' ? `0 ${AppleSpace.md}` : `0 ${AppleSpace.lg}`,
  textAlign: 'left' as const,
  fontWeight: String(AppleFontWeight.medium),
  fontSize: AppleType.microUI.size,
  color: cv('text-secondary'),
  borderBottom: `1px solid ${cv('border-light')}`,
  background: 'transparent',
  textTransform: 'none' as const,
  letterSpacing: '0',
  // 列头是短标签，任何情况下都不该逐字折行（「类型」拆成「类/型」就是这里没护栏）
  whiteSpace: 'nowrap' as const,
  fontVariantNumeric: 'tabular-nums' as const,
}));

const trStyle = computed(() => ({
  height: props.density === 'compact' ? AppleLayout.tableRowHeightCompact : AppleLayout.tableRowHeight,
  transition: AppleTransition.fast,
}));

const tdStyle = computed(() => ({
  padding: props.density === 'compact' ? `0 ${AppleSpace.md}` : `0 ${AppleSpace.lg}`,
  borderBottom: `1px solid ${cv('border-light')}`,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-primary'),
  fontVariantNumeric: 'tabular-nums' as const,
}));

const skeletonCellStyle = {
  padding: `${AppleSpace.md} ${AppleSpace.md}`,
  height: '14px',
  borderRadius: AppleRadius.small,
  background: cv('bg-secondary'),
  animation: `${AppleAnimation.pulseSoft} var(--apple-dur-pulseSoft) ease-in-out infinite`,
};
</script>

<style scoped>
.ap-table-row:hover {
  background: var(--apple-bg-hover);
}
</style>
