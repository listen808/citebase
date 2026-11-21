<template>
  <div :style="aiSubPageStyle">
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="Database" :size="18" :color="cv('blue')" />
          <span :style="aiSubSectionTitleStyle">{{ $t('aiSet2.k135') }}</span>
        </div>
      </template>

      <ApSegmentedControl v-model="activeTab" :options="tabs" />

      <div :style="mtStyle">
        <!-- Metrics Tab -->
        <div v-if="activeTab === 'metrics'">
          <div :style="actionRowStyle">
            <ApButton variant="primary" size="small" @click="openMetricModal()">
              <ApIcon name="Plus" :size="16" /> {{ $t('aiSet2.k136') }}
            </ApButton>
          </div>

          <ApTable :columns="metricColumns" :data="metrics" :loading="metricsLoading">
            <template #actions="{ row }">
              <ApButton variant="ghost" size="small" @click="openMetricModal(row as unknown as SemanticMetric)">{{ $t('aiSet2.edit') }}</ApButton>
              <ApButton variant="danger" size="small" @click="handleDeleteMetric(row as unknown as SemanticMetric)">{{ $t('aiSet2.delete') }}</ApButton>
            </template>
          </ApTable>
          <ApEmpty v-if="!metricsLoading && metrics.length === 0" icon="BarChart3" :title="$t('aiSet2.k165')" :description="$t('vAiUi.k220')" />
        </div>

        <!-- Dimensions Tab -->
        <div v-if="activeTab === 'dimensions'">
          <div :style="actionRowStyle">
            <ApButton variant="primary" size="small" @click="openDimensionModal()">
              <ApIcon name="Plus" :size="16" /> {{ $t('aiSet2.k137') }}
            </ApButton>
          </div>

          <ApTable density="compact" :columns="dimensionColumns" :data="dimensions" :loading="dimensionsLoading">
            <template #actions="{ row }">
              <ApButton variant="danger" size="small" @click="handleDeleteDimension(row as unknown as SemanticDimension)">{{ $t('aiSet2.delete') }}</ApButton>
            </template>
          </ApTable>
          <ApEmpty v-if="!dimensionsLoading && dimensions.length === 0" icon="Layers" :title="$t('aiSet2.k166')" :description="$t('vAiUi.k221')" />
        </div>
      </div>
    </ApCard>

    <!-- Metric Modal -->
    <ApModal v-model="showMetricModal" :title="$t('aiSet2.k167')" size="small" density="compact">
      <div :style="formStyle">
        <ApFormItem :label="$t('aiSet2.k139')">
          <ApInput v-model="metricForm.name" :placeholder="$t('aiSet2.k152')" />
        </ApFormItem>
        <ApFormItem :label="$t('aiSet2.k140')">
          <ApInput v-model="metricForm.code" :placeholder="$t('aiSet2.k153')" />
        </ApFormItem>
        <ApFormItem :label="$t('aiSet2.k141')">
          <ApInput v-model="metricForm.default_table" :placeholder="$t('aiSet2.k154')" />
        </ApFormItem>
        <ApFormItem :label="$t('aiSet2.k142')">
          <ApInput v-model="metricForm.formula" :placeholder="$t('aiSet2.k155')" />
        </ApFormItem>
        <ApFormItem :label="$t('aiSet2.k143')">
          <ApInput v-model="metricForm.time_field" :placeholder="$t('aiSet2.k156')" />
        </ApFormItem>
        <ApFormItem :label="$t('aiSet2.k144')">
          <ApInput v-model="metricForm.time_granularity" :placeholder="$t('aiSet2.k157')" />
        </ApFormItem>
        <ApFormItem :label="$t('aiSet2.k145')">
          <ApInput v-model="metricForm.dimensions" :placeholder="$t('aiSet2.k158')" />
        </ApFormItem>
        <ApFormItem :label="$t('aiSet2.desc')">
          <ApInput v-model="metricForm.description" :placeholder="$t('aiSet2.k159')" />
        </ApFormItem>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showMetricModal = false">{{ $t('aiSet2.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="savingMetric" @click="handleSaveMetric">{{ $t('aiSet2.save') }}</ApButton>
      </template>
    </ApModal>

    <!-- Dimension Modal -->
    <ApModal v-model="showDimensionModal" :title="$t('aiSet2.k168')" size="small">
      <div :style="formStyle">
        <ApFormItem :label="$t('aiSet2.k147')">
          <ApInput v-model="dimensionForm.name" :placeholder="$t('aiSet2.k160')" />
        </ApFormItem>
        <ApFormItem :label="$t('aiSet2.k148')">
          <ApInput v-model="dimensionForm.code" :placeholder="$t('aiSet2.k161')" />
        </ApFormItem>
        <ApFormItem :label="$t('aiSet2.k149')">
          <ApInput v-model="dimensionForm.table" :placeholder="$t('aiSet2.k154')" />
        </ApFormItem>
        <ApFormItem :label="$t('aiSet2.k150')">
          <ApInput v-model="dimensionForm.field" :placeholder="$t('aiSet2.k162')" />
        </ApFormItem>
        <ApFormItem :label="$t('aiSet2.k151')">
          <ApInput v-model="dimensionForm.value_type" :placeholder="$t('aiSet2.k163')" />
        </ApFormItem>
        <ApFormItem :label="$t('aiSet2.desc')">
          <ApInput v-model="dimensionForm.description" :placeholder="$t('aiSet2.k164')" />
        </ApFormItem>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showDimensionModal = false">{{ $t('aiSet2.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="savingDimension" @click="handleSaveDimension">{{ $t('aiSet2.save') }}</ApButton>
      </template>
    </ApModal>

    <!-- Delete Confirm -->
    <ApConfirmModal
      v-model="showDeleteConfirm"
      :title="deleteConfirmTitle"
      :message="deleteConfirmMessage"
      :loading="deleting"
      :confirm-text="$t('aiSet2.delete')"
      @confirm="confirmDelete"
    />
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted } from 'vue';
import { icons } from '@/utils/icons';







import { logger } from '@/utils/logger';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import { cv, AppleSpace } from '@/utils/core/apple-tokens';
import { aiSubPageStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle } from '@/styles/ai/ai-page-styles';
import { SemanticAPI, type SemanticMetric, type SemanticDimension } from '@/api/modules/ai/ai-semantic';
import { useSpaceStore } from '@/stores/workspace/space';
import type { Space } from '@/types/models';
import { useUIStore } from '@/stores/app/ui';
import ApFormItem from '@/components/apple/ApFormItem.vue';

const ui = useUIStore();
const spaceStore = useSpaceStore();

const activeTab = ref('metrics');
const tabs = [
  { value: 'metrics', label: t('aiSet2.k169'), icon: icons.BarChart3 },
  { value: 'dimensions', label: t('aiSet2.k170'), icon: icons.Layers },
];

const spaces = ref<Space[]>([]);
const selectedSpaceId = ref<number | null>(null);

const metrics = ref<SemanticMetric[]>([]);
const dimensions = ref<SemanticDimension[]>([]);
const metricsLoading = ref(false);
const dimensionsLoading = ref(false);

const showMetricModal = ref(false);
const showDimensionModal = ref(false);
const showDeleteConfirm = ref(false);
const savingMetric = ref(false);
const savingDimension = ref(false);
const deleting = ref(false);

const editingMetric = ref<SemanticMetric | null>(null);
const deleteTarget = ref<{ type: 'metric' | 'dimension'; id: number } | null>(null);

interface MetricFormData {
  space_id: number;
  name: string;
  code: string;
  default_table: string;
  formula: string;
  time_field: string;
  time_granularity: string;
  dimensions: string;
  description: string;
}

interface DimensionFormData {
  space_id: number;
  name: string;
  code: string;
  table: string;
  field: string;
  value_type: string;
  description: string;
}

const metricForm = ref<MetricFormData>({
  space_id: 0,
  name: '',
  code: '',
  default_table: '',
  formula: '',
  time_field: '',
  time_granularity: '',
  dimensions: '',
  description: '',
});

const dimensionForm = ref<DimensionFormData>({
  space_id: 0,
  name: '',
  code: '',
  table: '',
  field: '',
  value_type: '',
  description: '',
});

const metricColumns = [
  { key: 'name', title: t('aiSet2.name') },
  { key: 'code', title: t('aiSet2.k171') },
  { key: 'default_table', title: t('aiSet2.k141') },
  { key: 'formula', title: t('aiSet2.k172') },
  { key: 'description', title: t('aiSet2.desc') },
];

const dimensionColumns = [
  { key: 'name', title: t('aiSet2.name') },
  { key: 'code', title: t('aiSet2.k171') },
  { key: 'table', title: t('aiSet2.k173') },
  { key: 'field', title: t('aiSet2.k174') },
  { key: 'value_type', title: t('aiSet2.type') },
  { key: 'description', title: t('aiSet2.desc') },
];

const deleteConfirmTitle = computed(() => deleteTarget.value?.type === 'metric' ? '删除指标' : '删除维度');
const deleteConfirmMessage = computed(() => '确定要删除吗？此操作不可撤销。');

const formStyle = { display: 'grid', gap: AppleSpace.md };
const mtStyle = { marginTop: AppleSpace.md };
const actionRowStyle = { display: 'flex', justifyContent: 'flex-end', marginBottom: AppleSpace.md };

onMounted(async () => {
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
    const firstSpace = spaces.value[0];
    if (firstSpace) {
      selectedSpaceId.value = firstSpace.id;
      await Promise.all([loadMetrics(), loadDimensions()]);
    }
  } catch (e) {
    logger.error('Failed to load spaces:', e);
    ui.showToast(t('aiSet2.k25'), 'error');
  }
});

async function loadMetrics() {
  if (!selectedSpaceId.value) return;
  metricsLoading.value = true;
  try {
    const res = await SemanticAPI.listMetrics({ space_id: selectedSpaceId.value, page_size: 100 });
    // P1-5: 后端返回 { list, total }，此前按 res.data 取值恒为空列表
    metrics.value = res.list || [];
  } catch (e) {
    logger.error('Failed to load metrics:', e);
    ui.showToast(t('aiSet2.k176'), 'error');
  } finally {
    metricsLoading.value = false;
  }
}

async function loadDimensions() {
  if (!selectedSpaceId.value) return;
  dimensionsLoading.value = true;
  try {
    const res = await SemanticAPI.listDimensions({ space_id: selectedSpaceId.value, page_size: 100 });
    // P1-5: 后端返回 { list, total }
    dimensions.value = res.list || [];
  } catch (e) {
    logger.error('Failed to load dimensions:', e);
    ui.showToast(t('aiSet2.k177'), 'error');
  } finally {
    dimensionsLoading.value = false;
  }
}

function openMetricModal(metric?: SemanticMetric) {
  if (metric) {
    editingMetric.value = metric;
    metricForm.value = {
      space_id: metric.space_id,
      name: metric.name,
      code: metric.code,
      default_table: metric.default_table,
      formula: metric.formula,
      time_field: metric.time_field || '',
      time_granularity: metric.time_granularity || '',
      dimensions: metric.dimensions || '',
      description: metric.description || '',
    };
  } else {
    editingMetric.value = null;
    metricForm.value = { space_id: selectedSpaceId.value || 0, name: '', code: '', default_table: '', formula: '', time_field: '', time_granularity: '', dimensions: '', description: '' };
  }
  showMetricModal.value = true;
}

function openDimensionModal() {
  dimensionForm.value = { space_id: selectedSpaceId.value || 0, name: '', code: '', table: '', field: '', value_type: '', description: '' };
  showDimensionModal.value = true;
}

async function handleSaveMetric() {
  if (!selectedSpaceId.value) return;
  savingMetric.value = true;
  try {
    metricForm.value.space_id = selectedSpaceId.value;
    if (editingMetric.value?.id) {
      await SemanticAPI.updateMetric(editingMetric.value.id, metricForm.value);
    } else {
      await SemanticAPI.createMetric(metricForm.value);
    }
    showMetricModal.value = false;
    await loadMetrics();
    ui.showToast(t('aiSet2.k178'), 'success');
  } catch (e) {
    logger.error('Failed to save metric:', e);
    ui.showToast(t('aiSet2.k29'), 'error');
  } finally {
    savingMetric.value = false;
  }
}

async function handleSaveDimension() {
  if (!selectedSpaceId.value) return;
  savingDimension.value = true;
  try {
    dimensionForm.value.space_id = selectedSpaceId.value;
    await SemanticAPI.createDimension(dimensionForm.value);
    showDimensionModal.value = false;
    await loadDimensions();
    ui.showToast(t('aiSet2.k178'), 'success');
  } catch (e) {
    logger.error('Failed to save dimension:', e);
    ui.showToast(t('aiSet2.k29'), 'error');
  } finally {
    savingDimension.value = false;
  }
}

function handleDeleteMetric(metric: SemanticMetric) {
  if (!metric.id) return;
  deleteTarget.value = { type: 'metric', id: metric.id };
  showDeleteConfirm.value = true;
}

function handleDeleteDimension(dimension: SemanticDimension) {
  if (!dimension.id) return;
  deleteTarget.value = { type: 'dimension', id: dimension.id };
  showDeleteConfirm.value = true;
}

async function confirmDelete() {
  if (!deleteTarget.value) return;
  deleting.value = true;
  try {
    if (deleteTarget.value.type === 'metric') {
      await SemanticAPI.deleteMetric(deleteTarget.value.id);
      await loadMetrics();
    } else {
      await SemanticAPI.deleteDimension(deleteTarget.value.id);
      await loadDimensions();
    }
    showDeleteConfirm.value = false;
    ui.showToast(t('aiSet2.k179'), 'success');
  } catch (e) {
    logger.error('Failed to delete:', e);
    ui.showToast(t('aiSet2.k33'), 'error');
  } finally {
    deleting.value = false;
  }
}
</script>
