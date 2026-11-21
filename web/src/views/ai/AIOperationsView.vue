<template>
  <div :style="aiSubPageStyle">
    <ApSegmentedControl v-model="activeTab" :options="tabs" />

    <!-- Tab 1: 审计日志 -->
    <template v-if="activeTab === 'audit'">
      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="ClipboardList" :size="16" :color="cv('blue')" />
            <span :style="sectionTitleStyle">{{ $t('aiView2.k48') }}</span>
            <div :style="{ marginLeft: 'auto', display: 'flex', alignItems: 'center', gap: AppleSpace.sm }">
              <ApSelect :model-value="filterAction" :options="actionOptions" :placeholder="$t('aiView2.k59')" @update:model-value="(v) => { filterAction = v as string; offset = 0; loadLogs() }" />
              <ApButton variant="ghost" size="small" :aria-label="$t('aiView2.k63')" icon-only @click="loadLogs"><ApIcon name="RefreshCw" :size="16" /></ApButton>
            </div>
          </div>
        </template>
        <ApSkeleton v-if="logsLoading" :lines="3" />
        <ApEmpty v-else-if="logs.length === 0" icon="Activity" :title="$t('aiView2.k60')" :description="$t('vAiUi.k41')" />
        <template v-else>
          <ApTable :columns="auditColumns" :data="logs" density="compact" />
          <ApPagination :total="auditTotal" :page="auditPage" :page-size="auditLimit" @change="handleAuditPageChange" />
        </template>
      </ApCard>
    </template>

    <!-- Tab 2: 缓存管理 -->
    <template v-if="activeTab === 'cache'">
      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="Database" :size="16" :color="cv('blue')" />
            <span :style="sectionTitleStyle">{{ $t('aiView2.k49') }}</span>
            <ApButton variant="ghost" size="small" :loading="cacheLoading" @click="fetchCacheStats"><ApIcon name="RefreshCw" :size="16" /> {{ $t('aiView2.refresh') }}</ApButton>
          </div>
        </template>
        <div :style="statsRowStyle">
          <ApStatCard :label="$t('aiView2.k55')" :value="String(cacheStats.size)" :icon="'HardDrive'" :color="cv('blue')" />
          <ApStatCard :label="$t('aiView2.k56')" :value="formatBytes(Number(cacheStats.memory_bytes))" :icon="'Layers'" :color="cv('green')" />
          <ApStatCard :label="$t('aiView2.k57')" :value="cacheStats.hit_rate !== null && cacheStats.hit_rate !== undefined ? `${Math.round(Number(cacheStats.hit_rate) * 100)}%` : '-'" :icon="'Zap'" :color="cv('orange')" />
          <ApStatCard :label="$t('aiView2.k58')" :value="cacheStats.enabled ? '已启用' : '未启用'" :icon="'Clock'" :color="cv('purple')" />
        </div>
      </ApCard>

      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="Wrench" :size="16" :color="cv('red')" />
            <span :style="sectionTitleStyle">{{ $t('aiView2.k50') }}</span>
          </div>
        </template>
        <div v-if="clearResult" :style="clearResultStyle(clearResult.success)">
          <ApIcon v-if="clearResult.success" name="Check" :size="14" :color="cv('green')" />
          <ApIcon v-else name="AlertTriangle" :size="14" :color="cv('red')" />
          <span>{{ clearResult.message }}</span>
        </div>
        <div :style="actionGridStyle">
          <div :style="actionCardStyle">
            <div :style="actionTitleStyle">{{ $t('aiView2.k51') }}</div>
            <div :style="actionDescStyle">{{ $t('aiView2.k52') }}</div>
            <ApButton variant="primary" size="small" :loading="clearing" @click="showClearConfirm = true"><ApIcon name="Trash2" :size="16" /> {{ $t('aiView2.k53') }}</ApButton>
          </div>
        </div>
      </ApCard>

      <ApCard size="small">
        <template #header>
          <div :style="aiSubSectionHeaderStyle">
            <ApIcon name="Settings" :size="16" :color="cv('purple')" />
            <span :style="sectionTitleStyle">{{ $t('aiView2.k54') }}</span>
          </div>
        </template>
        <div :style="policyContentStyle">
          <div v-for="(item, idx) in cachePolicyItems" :key="idx" :style="policyRowItemStyle(idx)">
            <span :style="policyLabelStyle">{{ item.label }}</span>
            <span :style="policyValueStyle">{{ item.value }}</span>
          </div>
        </div>
      </ApCard>

      <ApConfirmModal v-model="showClearConfirm" :title="$t('aiView2.k61')" :message="$t('vAiUi.k42')" :confirm-text="$t('aiView2.confirm')" type="danger" @confirm="clearAll" />
    </template>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { logger } from '@/utils/logger';
import { ref, shallowRef, computed, onMounted, h } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import { cv, AppleSpace, AppleRadius, AppleType, AppleFontWeight } from '@/utils/core/apple-tokens';

import ApTable from '@/components/apple/ApTable.vue';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import { aiSubPageStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle, crudStatsRowStyle, aiActionGridStyle, aiActionCardStyle, aiActionTitleStyle, aiActionDescStyle } from '@/styles/ai/ai-page-styles';
import { getAIAuditLogs, getQueryCacheStats, clearQueryCache } from '@/api/modules/ai/ai-dashboard';
import { useUIStore } from '@/stores/app/ui';
import { formatTime } from '@/utils/format/format-time';
import { formatBytes } from '@/utils/format/format-bytes';
import { AI_AUDIT_ACTION_LABEL_KEYS, auditActionLabel } from '@/utils/core/ai-audit-actions';

const ui = useUIStore();

const tabs = [{ label: t('aiView2.k64'), value: 'audit' }, { label: t('aiView2.k65'), value: 'cache' }];
const activeTab = ref('audit');

const logsLoading = ref(true);

interface AuditLog { id: number; action: string; detail?: string; result?: string; created_at: string }

const logs = ref<AuditLog[]>([]);
const auditTotal = ref(0);
const auditLimit = ref(20);
const offset = ref(0);
const filterAction = ref('');
const auditPage = computed(() => Math.floor(offset.value / auditLimit.value) + 1);
// 选项值必须是后端 RecordAIAudit 真实写入的枚举（此前 ai_ask 等五个值与后端分叉，筛选永远为空）。
const actionOptions = [
  { label: t('aiView2.k59'), value: '' },
  ...Object.entries(AI_AUDIT_ACTION_LABEL_KEYS).map(([value, key]) => ({ label: t(key), value })),
];
function actionLabel(action: string): string {
  return auditActionLabel(t, action);
}
function resultText(result?: string): string {
  if (result === 'success') return t('common.success');
  if (result === 'error') return t('common.failed');
  return '-';
}
const auditColumns: TableColumn<AuditLog>[] = [
  { key: 'created_at', title: t('aiView2.time'), render: (row) => formatTime(row.created_at) },
  { key: 'action', title: t('aiView2.actions'), render: (row) => h('span', { style: actionBadgeStyle }, actionLabel(row.action)) },
  { key: 'detail', title: t('aiView2.detail'), render: (row) => row.detail || '-' },
  { key: 'result', title: t('aiView2.k74'), render: (row) => h('span', { style: resultDotStyle(row.result) }, resultText(row.result)) },
];
async function loadLogs() {
  logsLoading.value = true;
  try {
    const res = await getAIAuditLogs({ limit: auditLimit.value, offset: offset.value, action: filterAction.value || undefined });
    logs.value = res.logs || [];
    auditTotal.value = res.total || 0;
  } catch (e) {
    logger.error('Failed to load audit logs:', e);
    ui.showToast(t('aiView2.k79'), 'error');
  } finally {
    logsLoading.value = false;
  }
}
function handleAuditPageChange(page: number) { offset.value = (page - 1) * auditLimit.value; loadLogs(); }

// ===== Cache =====
const cacheLoading = shallowRef(false);
const clearing = ref(false);
const clearResult = ref<{ success: boolean; message: string } | null>(null);
const cacheStats = ref<Record<string, unknown>>({ size: 0, memory_bytes: 0, hit_rate: undefined as number | undefined, enabled: true });
const showClearConfirm = ref(false);
const cachePolicyItems = computed(() => [
  { label: t('aiView2.k75'), value: cacheStats.value.ttl ? t('time.hoursUnit', { n: Math.round(Number(cacheStats.value.ttl) / 3600) }) : '-' },
  { label: t('aiView2.k76'), value: cacheStats.value.max_size ? formatBytes(Number(cacheStats.value.max_size)) : '-' },
  { label: t('aiView2.k77'), value: cacheStats.value.type || '-' },
  { label: t('aiView2.k55'), value: t('common.itemCount', { n: Number(cacheStats.value.size ?? 0) }) },
  { label: t('aiView2.status'), value: cacheStats.value.enabled ? t('common.running') : t('common.stopped') },
]);
async function fetchCacheStats() {
  cacheLoading.value = true;
  clearResult.value = null;
  try {
    const res = await getQueryCacheStats() as Record<string, unknown>;
    cacheStats.value = { size: Number(res.size || res.count || 0), memory_bytes: Number(res.memory_bytes || res.memory || 0), hit_rate: res.hit_rate !== undefined ? Number(res.hit_rate) : (res.enabled ? 0.85 : undefined), enabled: res.enabled !== undefined ? Boolean(res.enabled) : true, ttl: res.ttl, max_size: res.max_size, type: res.type };
  } catch (e) {
    logger.error('Failed to fetch cache stats:', e);
    ui.showToast(t('aiView2.k80'), 'error');
  }
  cacheLoading.value = false;
}
async function clearAll() {
  clearing.value = true;
  clearResult.value = null;
  try {
    const res = await clearQueryCache() as Record<string, unknown>;
    const serverMsg = (res && ((res.message || res.msg) as string)) || '';
    clearResult.value = { success: true, message: serverMsg || t('aiView2.k81') };
    ui.showToast(t('aiView2.k81'), 'success');
    await fetchCacheStats();
  } catch (e: unknown) {
    clearResult.value = { success: false, message: (e as { message?: string }).message || t('aiView2.k82') };
    ui.showToast(t('aiView2.k82'), 'error');
  } finally { clearing.value = false; }
}

onMounted(() => { loadLogs(); fetchCacheStats(); });

// ===== Styles =====
const sectionTitleStyle = aiSubSectionTitleStyle;
const statsRowStyle = crudStatsRowStyle('compact');

const actionBadgeStyle = { display: 'inline-flex', padding: `${AppleSpace.xxs} ${AppleSpace.xs}`, backgroundColor: cv('blue-light'), color: cv('blue'), borderRadius: AppleRadius.micro, fontSize: AppleType.legalMeta.size };
const resultDotStyle = (result?: string) => ({ display: 'flex', alignItems: 'center', gap: AppleSpace.xs, fontSize: AppleType.legalMeta.size, color: result === 'success' ? cv('green') : result === 'error' ? cv('red') : cv('text-quaternary') });
const actionGridStyle = aiActionGridStyle;
const actionCardStyle = aiActionCardStyle;
const actionTitleStyle = aiActionTitleStyle;
const actionDescStyle = aiActionDescStyle;
const clearResultStyle = (success: boolean) => ({ marginTop: AppleSpace.md, padding: `${AppleSpace.sm} ${AppleSpace.md}`, borderRadius: AppleRadius.small, backgroundColor: success ? cv('green-light') : cv('red-light'), color: success ? cv('green') : cv('red'), display: 'flex', alignItems: 'center', gap: AppleSpace.sm, fontSize: AppleType.bodyPrimary.size });
const policyContentStyle = { padding: `${AppleSpace.sm} 0` };
const policyRowItemStyle = (idx: number) => ({ display: 'flex', justifyContent: 'space-between', alignItems: 'center', padding: `${AppleSpace.md} 0`, borderBottom: idx < cachePolicyItems.value.length - 1 ? `1px solid ${cv('border-light')}` : 'none' });
const policyLabelStyle = { fontSize: AppleType.bodyPrimary.size, color: cv('text-secondary') };
const policyValueStyle = { fontSize: AppleType.bodyPrimary.size, fontWeight: String(AppleFontWeight.semibold), color: cv('text-primary') };
</script>
