<template>
  <div :style="aiSubPageStyle">
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <div :style="aiSubCardNameRowStyle">
            <ApIcon name="ScanSearch" :size="18" :color="cv('red')" />
            <span :style="aiSubSectionTitleStyle">{{ $t('aiSet2.k68') }}</span>
            <span :style="aiSubCountBadgeStyle">{{ total }}</span>
          </div>
          <span :style="aiSubSectionMetaStyle">{{ $t('aiSet2.k69') }}</span>
        </div>
      </template>

      <div :style="toolbarStyle">
        <span :style="labelStyle">每页 {{ PAGE_SIZE }} 条 · 第 {{ page }} 页</span>
        <ApButton variant="secondary" size="small" :loading="loading" @click="loadItems">{{ $t('aiSet2.refresh') }}</ApButton>
      </div>

      <div :style="listStyle(AppleSpace.sm)">
        <div v-for="log in items" :key="log.id" :style="aiSubCardStyle">
          <div :style="aiSubCardHeaderStyle">
            <div :style="aiSubCardInfoStyle">
              <span :style="aiSubCardNameStyle">#{{ log.id }}</span>
              <ApStatusBadge :status="resultBadge(log.result)" :label="resultText(log.result)" size="small" />
              <span :style="aiSubSectionMetaStyle">{{ log.model || log.provider || $t('fallback.k81') }}</span>
              <span :style="aiSubSectionMetaStyle">{{ log.duration_ms }} ms</span>
              <span :style="aiSubSectionMetaStyle">{{ formatTime(log.created_at) }}</span>
            </div>
            <ApButton
              variant="ghost"
              size="small"
              :loading="loadingId === log.id"
              @click="toggleExpand(log)"
            >
              <ApIcon v-if="expandedId !== log.id" name="ChevronDown" :size="16" />
              <ApIcon v-else name="ChevronUp" :size="16" />
              {{ expandedId === log.id ? '收起' : $t('vAiUi.k223') }}
            </ApButton>
          </div>

          <p :style="aiSubDescStyle">{{ log.question || $t('fallback.k114') }}</p>

          <div v-if="expandedId === log.id && details[log.id]" :style="aiSubSectionStyle">
            <ApDescriptionList :items="describeItems(log)" />

            <div v-if="log.reason" :style="aiSubSectionStyle">
              <span :style="aiSubLabelStyle">{{ $t('aiSet2.k70') }}</span>
              <pre :style="aiSubBodyStyle">{{ log.reason }}</pre>
            </div>

            <div v-if="log.answer" :style="aiSubSectionStyle">
              <span :style="aiSubLabelStyle">{{ $t('aiSet2.k71') }}</span>
              <pre :style="aiSubBodyStyle">{{ log.answer }}</pre>
            </div>

            <div v-if="log.detail" :style="aiSubSectionStyle">
              <span :style="aiSubLabelStyle">{{ $t('aiSet2.k72') }}</span>
              <pre :style="aiSubBodyStyle">{{ log.detail }}</pre>
            </div>
          </div>
        </div>
      </div>

      <ApEmpty
        v-if="!loading && items.length === 0"
        icon="ScanSearch"
        :title="$t('aiSet2.k73')"
        :description="$t('vAiUi.k222')"
      />

      <ApPagination
        v-if="total > PAGE_SIZE"
        :total="total"
        :page="page"
        :page-size="PAGE_SIZE"
        @change="changePage"
      />
    </ApCard>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, onMounted } from 'vue';
// ScanSearch 与二级导航「SQL 审计追踪」同符号：卡片头不再用 ScrollText、空态不再用 Shield，
// 三处（导航 / 卡片头 / 空态）统一为一枚 ScanSearch。
import ApStatusBadge from '@/components/apple/ApStatusBadge.vue';

import { logger } from '@/utils/logger';
import { cv, AppleSpace } from '@/utils/core/apple-tokens';
import { formatTime } from '@/utils/format/format-time';
import {
  aiSubPageStyle,
  aiSubSectionStyle,
  aiSubSectionHeaderStyle,
  aiSubSectionTitleStyle,
  aiSubSectionMetaStyle,
  aiSubCardStyle,
  aiSubCardHeaderStyle,
  aiSubCardInfoStyle,
  aiSubCardNameStyle,
  aiSubCardNameRowStyle,
  aiSubCountBadgeStyle,
  aiSubLabelStyle,
  aiSubDescStyle,
  aiSubBodyStyle,
  aiSubToolbarBetweenStyle as toolbarStyle,
  aiSubFilterLabelStyle as labelStyle,
  listStyle,
} from '@/styles/ai/ai-page-styles';
import { SQLAuditTraceAPI, type SQLAuditTrace } from '@/api/modules/ai/ai-sql-audit';
import type { DescriptionItem } from '@/components/apple/ApDescriptionList.vue';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();

// 后端 limit 上限 100，超出会回落 20
const PAGE_SIZE = 20;

const items = ref<SQLAuditTrace[]>([]);
const total = ref(0);
const page = ref(1);
const loading = shallowRef(false);
const expandedId = ref<number | null>(null);
const loadingId = ref<number | null>(null);
const details = ref<Record<number, SQLAuditTrace>>({});

function resultBadge(result: string): 'enabled' | 'error' {
  return result === 'success' ? 'enabled' : 'error';
}

function resultText(result: string) {
  if (result === 'success') return '成功';
  if (result === 'failure') return '失败';
  return result || t('fallback.k80');
}

function describeItems(log: SQLAuditTrace): DescriptionItem[] {
  return [
    { label: t('aiSet2.k74'), value: log.user_id || null },
    { label: t('aiSet2.k75'), value: log.target_id || null },
    { label: t('aiSet2.k76'), value: log.action },
    { label: t('aiSet2.k77'), value: log.model },
    { label: t('aiSet2.k78'), value: log.provider },
    { label: t('aiSet2.k79'), value: log.token_usage },
    { label: t('aiSet2.k80'), value: log.source_count },
    { label: t('aiSet2.k60'), value: `${log.duration_ms} ms` },
    { label: t('aiSet2.k81'), value: log.ip_address },
    { label: 'User-Agent', value: log.user_agent },
    { label: t('aiSet2.k82'), value: formatTime(log.created_at) },
  ];
}

async function loadItems() {
  loading.value = true;
  try {
    const res = await SQLAuditTraceAPI.list({ limit: PAGE_SIZE, offset: (page.value - 1) * PAGE_SIZE });
    items.value = res.list ?? [];
    total.value = res.total ?? 0;
    details.value = {};
    expandedId.value = null;
  } catch (e) {
    logger.error('Failed to load sql audit traces:', e);
    ui.showToast(t('aiSet2.k83'), 'error');
  } finally {
    loading.value = false;
  }
}

function changePage(next: number) {
  page.value = next;
  loadItems();
}

async function toggleExpand(log: SQLAuditTrace) {
  if (expandedId.value === log.id) {
    expandedId.value = null;
    return;
  }
  expandedId.value = log.id;
  if (details.value[log.id]) return;
  loadingId.value = log.id;
  try {
    // 展开时按主键单查一次，确认拿到的是这条记录本身（列表按 created_at 排序）
    details.value = { ...details.value, [log.id]: await SQLAuditTraceAPI.detail(log.id) };
  } catch (e) {
    logger.error('Failed to load sql audit trace detail:', e);
    ui.showToast(t('aiSet2.k84'), 'error');
    expandedId.value = null;
  } finally {
    loadingId.value = null;
  }
}

onMounted(() => loadItems());
</script>
