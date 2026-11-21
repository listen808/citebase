<template>
  <ApPageContainer
    :title="$t('adminSys.k62')"
    :subtitle="$t('adminSys.k63')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <div :style="PageLayout.sectionGap">
      <!-- ==================== 数据导出（G037） ==================== -->
      <ApCard size="small">
        <template #header>
          <div :style="statusRowStyle">
            <ApIcon name="FileDown" :size="18" :color="cv('blue')" />
            <span :style="sectionTitleStyle">{{ $t('adminSys.k47') }}</span>
          </div>
        </template>

        <div :style="toolbarStyle">
          <div :style="filterGroupStyle()">
            <span :style="formLabelStyle()">{{ $t('adminSys.k48') }}</span>
            <ApSelect v-model="module" :options="moduleOptions" />
          </div>
          <div :style="filterGroupStyle()">
            <span :style="formLabelStyle()">{{ $t('adminSys.k49') }}</span>
            <ApSelect v-model="format" :options="formatOptions" />
          </div>
          <div :style="toolbarSpacerStyle()" />
          <ApButton variant="primary" size="small" :loading="exporting" @click="handleExport">
            <ApIcon name="Download" :size="16" />{{ $t('adminSys.k50') }}
          </ApButton>
        </div>

        <p :style="descStyle">
          {{ $t('adminSys.k51') }}
        </p>
      </ApCard>

      <!-- ==================== 合规报告（G038） ==================== -->
      <ApCard size="small">
        <template #header>
          <div :style="statusRowStyle">
            <ApIcon name="ShieldCheck" :size="18" :color="cv('purple')" />
            <span :style="sectionTitleStyle">{{ $t('adminSys.k52') }}</span>
          </div>
        </template>

        <div :style="toolbarStyle">
          <div :style="filterGroupStyle()">
            <span :style="formLabelStyle()">{{ $t('adminSys.k53') }}</span>
            <ApSelect v-model="days" :options="daysOptions" />
          </div>
          <span v-if="report" :style="statusIndicatorLargeStyle(healthLevel)">
            {{ $t('adminSys.k382', { n: (report.success_rate ?? 0).toFixed(2) }) }}
          </span>
          <div :style="toolbarSpacerStyle()" />
          <ApButton variant="ghost" size="small" :loading="reportLoading" @click="loadReport">
            <ApIcon name="RefreshCw" :size="16" />{{ $t('adminSys.refresh') }}
          </ApButton>
          <ApButton variant="outline" size="small" :loading="reportExporting" @click="handleExportReport('csv')">
            {{ $t('adminSys.k54') }}
          </ApButton>
          <ApButton variant="outline" size="small" :loading="reportExporting" @click="handleExportReport('json')">
            {{ $t('adminSys.k55') }}
          </ApButton>
        </div>

        <p v-if="report" :style="noteStyle">
          <span :style="metaItemStyle">{{ $t('adminSys.k383', { n: report.period_days }) }}</span>
          <span :style="metaItemStyle">{{ $t('adminSys.k384', { when: formatGeneratedAt(report.generated_at) }) }}</span>
        </p>

        <div v-if="report" :style="crudStatsRowStyle('compact')">
          <ApStatCard
            v-for="card in statCards"
            :key="card.label"
            :label="card.label"
            :value="card.value"
            :icon="card.icon"
            :color="card.color"
            size="compact"
          />
        </div>

        <div :style="standardSectionStyle">
          <div :style="sectionTitleStyle">{{ $t('adminSys.k56') }}</div>
          <ApTable
            density="compact"
            row-key="action"
            :columns="actionColumns"
            :data="report?.by_action ?? []"
            :loading="reportLoading"
          >
            <template #empty>{{ $t('adminSys.k57') }}</template>
          </ApTable>
        </div>

        <div :style="standardSectionStyle">
          <div :style="sectionTitleStyle">{{ $t('adminSys.k58') }}</div>
          <ApTable
            density="compact"
            row-key="user_id"
            :columns="userColumns"
            :data="report?.top_users ?? []"
            :loading="reportLoading"
          >
            <template #empty>{{ $t('adminSys.k59') }}</template>
          </ApTable>
        </div>

        <div :style="standardSectionStyle">
          <div :style="sectionTitleStyle">{{ $t('adminSys.k60') }}</div>
          <ApTable
            density="compact"
            row-key="date"
            :columns="trendColumns"
            :data="report?.daily_trend ?? []"
            :loading="reportLoading"
          >
            <template #empty>{{ $t('adminSys.k61') }}</template>
          </ApTable>
        </div>
      </ApCard>
    </div>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { computed, onMounted, ref, shallowRef, watch } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';

import { logger } from '@/utils/logger';
import { AppleLayout, cv } from '@/utils/core/apple-tokens';
import type { IconName } from '@/utils/icons';
import {
  crudStatsRowStyle,
  descStyle,
  filterGroupStyle,
  formLabelStyle,
  metaItemStyle,
  noteStyle,
  PageLayout,
  sectionTitleStyle,
  standardSectionStyle,
  statusIndicatorLargeStyle,
  statusRowStyle,
  toolbarSpacerStyle,
  toolbarStyle,
} from '@/styles/layout/page-styles';
import {
  EXPORT_MODULES,
  EXPORT_MODULE_LABELS,
  exportComplianceReport,
  exportModuleData,
  getComplianceReport,
  type ComplianceActionStat,
  type ComplianceReport,
  type ExportFormat,
  type ExportModule,
} from '@/api/modules/system/data-export';
import { useUIStore } from '@/stores/app/ui';
import { formatTime } from '@/utils/format/format-time';
import { auditActionLabel } from '@/utils/core/ai-audit-actions';

const ui = useUIStore();

// ApSelect 的 modelValue 是 string|number|null，v-model 目标统一用 ref<string>，
// 调用 API 时再收窄回字面量联合类型，避免在模板里做类型断言。
const module = ref<string>(EXPORT_MODULES[0]);
const format = ref<string>('csv');
const days = ref<string>('30');

const moduleOptions = EXPORT_MODULES.map((m) => ({ label: EXPORT_MODULE_LABELS[m], value: m }));
const formatOptions = [
  { label: 'CSV', value: 'csv' },
  { label: 'JSON', value: 'json' },
];
// 后端 days 取值区间 (0,365]，越界会静默回落成 30，因此只提供区间内的选项
const daysOptions = [
  { label: t('adminSys.k64'), value: '7' },
  { label: t('adminSys.k65'), value: '30' },
  { label: t('adminSys.k66'), value: '90' },
  { label: t('adminSys.k67'), value: '180' },
  { label: t('adminSys.k68'), value: '365' },
];

const exporting = shallowRef(false);
const reportExporting = shallowRef(false);
const reportLoading = shallowRef(false);
const report = ref<ComplianceReport | null>(null);

const actionColumns = [
  {
    key: 'action',
    title: t('adminSys.k69'),
    ellipsis: true,
    render: (row: ComplianceActionStat) => auditActionLabel(t, row.action),
  },
  { key: 'count', title: t('adminSys.k70'), align: 'right' as const },
  { key: 'success_count', title: t('adminSys.success'), align: 'right' as const },
  { key: 'fail_count', title: t('adminSys.failed'), align: 'right' as const },
];

const userColumns = [
  { key: 'username', title: t('adminSys.k73'), ellipsis: true },
  { key: 'given_name', title: t('adminSys.k74'), ellipsis: true },
  { key: 'request_count', title: t('adminSys.k75'), align: 'right' as const },
  { key: 'total_usage', title: t('adminSys.k76'), align: 'right' as const },
  { key: 'avg_usage', title: t('adminSys.k77'), align: 'right' as const },
  { key: 'last_used', title: t('adminSys.k27') },
];

const trendColumns = [
  { key: 'date', title: t('adminSys.k78') },
  { key: 'request_count', title: t('adminSys.k75'), align: 'right' as const },
  { key: 'total_usage', title: t('adminSys.k76'), align: 'right' as const },
  { key: 'unique_users', title: t('adminSys.k79'), align: 'right' as const },
];

const statCards = computed(() => {
  const r = report.value;
  const cards: Array<{ label: string; value: string; icon: IconName; color: string }> = [
    { label: t('adminSys.k80'), value: r ? String(r.total_events) : '-', icon: 'Activity', color: cv('blue') },
    { label: t('adminSys.success'), value: r ? String(r.success_count) : '-', icon: 'CircleCheck', color: cv('green') },
    { label: t('adminSys.failed'), value: r ? String(r.fail_count) : '-', icon: 'CircleX', color: cv('red') },
    {
      label: t('adminSys.k81'),
      value: r ? `${r.success_rate.toFixed(2)}%` : '-',
      icon: 'TrendingUp',
      color: cv('purple'),
    },
  ];
  return cards;
});

/** 成功率分档：>=99 健康，>=90 告警，其余异常 */
const healthLevel = computed<'healthy' | 'warning' | 'error'>(() => {
  const rate = report.value?.success_rate ?? 0;
  if (rate >= 99) return 'healthy';
  if (rate >= 90) return 'warning';
  return 'error';
});

const formatGeneratedAt = formatTime;

/** 触发浏览器保存：export 端点回写的是文件流，不是 JSON 信封 */
function saveBlob(blob: Blob, filename: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  // 立即 revoke 在部分浏览器下会截断尚在写入的下载，延后一拍再释放
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}

async function handleExport() {
  if (exporting.value) return;
  exporting.value = true;
  const mod = module.value as ExportModule;
  const fmt = format.value as ExportFormat;
  try {
    const blob = await exportModuleData(mod, fmt);
    saveBlob(blob, `${mod}.${fmt}`);
    ui.showToast(t('sentUi.k20', { p1: EXPORT_MODULE_LABELS[mod] }), 'success');
  } catch (e: unknown) {
    logger.error('Failed to export module:', e);
    ui.showToast((e as Error).message || t('fallback.k49'), 'error');
  } finally {
    exporting.value = false;
  }
}

async function loadReport() {
  reportLoading.value = true;
  try {
    report.value = await getComplianceReport(Number(days.value) || 30);
  } catch (e: unknown) {
    logger.error('Failed to load compliance report:', e);
    ui.showToast((e as Error).message || t('fallback.k25'), 'error');
  } finally {
    reportLoading.value = false;
  }
}

async function handleExportReport(fmt: ExportFormat) {
  if (reportExporting.value) return;
  reportExporting.value = true;
  try {
    const blob = await exportComplianceReport(Number(days.value) || 30, fmt);
    saveBlob(blob, `compliance-report.${fmt}`);
    ui.showToast(t('adminSys.k82'), 'success');
  } catch (e: unknown) {
    logger.error('Failed to export compliance report:', e);
    ui.showToast((e as Error).message || t('fallback.k48'), 'error');
  } finally {
    reportExporting.value = false;
  }
}

// 窗口切换后重新拉取报告。用 watch 而非 @update:model-value，
// 避免与 v-model 的赋值处理器抢执行顺序而读到旧值。
watch(days, () => {
  void loadReport();
});

onMounted(() => loadReport());
</script>
