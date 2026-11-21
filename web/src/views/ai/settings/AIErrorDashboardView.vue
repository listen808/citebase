<template>
  <div :style="aiSubPageStyle">
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="PieChart" :size="18" :color="cv('orange')" />
          <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k117') }}</span>
        </div>
      </template>

      <div :style="toolbarStyle">
        <div :style="spaceSelectorStyle">
          <span :style="labelStyle">{{ $t('aiSet.k2') }}</span>
          <ApSelect
            v-model="selectedSpaceId"
            :options="spaceOptions"
            :placeholder="$t('aiSet.k13')"
            :searchable="true"
            @update:model-value="onSpaceChange"
          />
        </div>
        <ApButton variant="ghost" size="small" :loading="summaryLoading" @click="loadData">
          <ApIcon name="RefreshCw" :size="16" /> {{ $t('aiSet.refresh') }}
        </ApButton>
      </div>

      <div v-if="summaryLoading" :style="skeletonGridStyle">
        <div v-for="n in 4" :key="n" :style="skeletonBarStyle">
          <ApSkeleton :style="{ height: AppleSpace.md, width: AppleSpace['4xl'] }" />
          <ApSkeleton :style="{ height: AppleSpace['12px'], width: '60%' }" />
        </div>
      </div>

      <div v-else-if="summaryEntries.length === 0" :style="emptyStyle">
        <ApEmpty :title="$t('aiSet.k120')" :description="$t('vAiUi.k210')" />
      </div>

      <div v-else :style="summaryGridStyle">
        <div
          v-for="entry in summaryEntries"
          :key="entry.type"
          :style="summaryCardStyle"
        >
          <div :style="summaryHeaderStyle">
            <span :style="summaryTypeStyle">{{ entry.type }}</span>
            <span :style="summaryCountStyle">{{ entry.count }}</span>
          </div>
          <ApProgress
            :value="entry.ratio * 100"
            :label="`${entry.type} 占比`"
            tone="warning"
            :style="{ marginBottom: AppleSpace.xs }"
          />
          <div :style="summaryRatioStyle">{{ (entry.ratio * 100).toFixed(1) }}%</div>
        </div>
      </div>
    </ApCard>

    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="AlertTriangle" :size="18" :color="cv('red')" />
          <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k118') }}</span>
        </div>
      </template>

      <ApTable :columns="columns" :data="failures" :loading="failuresLoading">
        <template #empty>
          {{ $t('aiSet.k119') }}
        </template>
      </ApTable>
    </ApCard>
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted } from 'vue';







import { logger } from '@/utils/logger';
import { cv, AppleFontWeight, AppleRadius, AppleSpace, AppleType } from '@/utils/core/apple-tokens';
import { aiSubPageStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle, aiSubToolbarBetweenStyle as toolbarStyle, aiSubFilterStyle as spaceSelectorStyle, aiSubFilterLabelStyle as labelStyle, gridStandard } from '@/styles/ai/ai-page-styles';
import { ErrorDashboardAPI } from '@/api/modules/ai/ai-error-dashboard';
import { useSpaceStore } from '@/stores/workspace/space';
import type { Space } from '@/types/models';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();
const spaceStore = useSpaceStore();

interface FailureRecord {
  id: number;
  space_id: number;
  user_query: string;
  generated_sql?: string;
  execution_ok: boolean;
  error_type: string;
  error_message: string;
  created_at: string;
}

const spaces = ref<Space[]>([]);
const selectedSpaceId = ref<number | null>(null);
const summary = ref<Record<string, number>>({});
const failures = ref<FailureRecord[]>([]);
const summaryLoading = ref(false);
const failuresLoading = ref(false);

const spaceOptions = computed(() => spaces.value.map((s) => ({ label: s.name, value: s.id })));

const summaryEntries = computed(() => {
  const total = Object.values(summary.value).reduce((sum, c) => sum + c, 0) || 1;
  return Object.entries(summary.value)
    .map(([type, count]) => ({
      type: type || 'unknown',
      count,
      ratio: count / total,
    }))
    .sort((a, b) => b.count - a.count);
});

const columns = [
  { key: 'user_query', title: t('aiSet.k121') },
  { key: 'error_type', title: t('aiSet.k122') },
  { key: 'error_message', title: t('aiSet.k123') },
  { key: 'created_at', title: t('aiSet.k124') },
];

async function loadSpaces() {
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
    const firstSpace = spaces.value[0];
    if (firstSpace && !selectedSpaceId.value) {
      selectedSpaceId.value = firstSpace.id;
      await loadData();
    }
  } catch (e) {
    logger.error('Failed to load spaces:', e);
    ui.showToast(t('aiSet.k26'), 'error');
  }
}

async function loadData() {
  if (!selectedSpaceId.value) return;
  const spaceId = selectedSpaceId.value;
  summaryLoading.value = true;
  failuresLoading.value = true;
  try {
    const [summaryRes, failuresRes] = await Promise.all([
      ErrorDashboardAPI.summary({ space_id: spaceId }),
      ErrorDashboardAPI.recentFailures({ space_id: spaceId, limit: 20 }),
    ]);
    summary.value = summaryRes.data || {};
    failures.value = failuresRes.data || [];
  } catch (e) {
    logger.error('Failed to load error dashboard data:', e);
    ui.showToast(t('aiSet.k125'), 'error');
  } finally {
    summaryLoading.value = false;
    failuresLoading.value = false;
  }
}

function onSpaceChange() {
  loadData();
}

onMounted(() => loadSpaces());

const summaryGridStyle = gridStandard;

const summaryCardStyle = {
  padding: AppleSpace.lg,
  backgroundColor: cv('surface-panel'),
  borderRadius: AppleRadius.medium,
  border: `1px solid ${cv('border-light')}`,
};

const summaryHeaderStyle = {
  display: 'flex',
  alignItems: 'center',
  justifyContent: 'space-between',
  marginBottom: AppleSpace.sm,
};

const summaryTypeStyle = {
  fontSize: AppleType.controlLabel.size,
  fontWeight: String(AppleFontWeight.medium),
  color: cv('text-primary'),
};

const summaryCountStyle = {
  fontSize: AppleType.linkActionHeading.size,
  fontWeight: String(AppleFontWeight.semibold),
  color: cv('orange'),
};

const summaryRatioStyle = {
  fontSize: AppleType.legalMeta.size,
  color: cv('text-secondary'),
};

const skeletonGridStyle = gridStandard;

const skeletonBarStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.sm,
  padding: AppleSpace.lg,
  backgroundColor: cv('bg-secondary'),
  borderRadius: AppleRadius.medium,
};

const emptyStyle = {
  padding: `${AppleSpace.xl} 0`,
};
</script>
