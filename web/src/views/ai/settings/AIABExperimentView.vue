<template>
  <div :style="aiSubPageStyle">
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="Split" :size="16" :color="cv('blue')" />
          <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k1') }}</span>
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
        <ApButton variant="primary" size="small" @click="openModal()">
          <ApIcon name="Plus" :size="16" /> {{ $t('aiSet.k3') }}
        </ApButton>
      </div>

      <ApTable :columns="columns" :data="experiments" :loading="loading" hide-header-when-empty>
        <template #empty>
          <ApEmpty icon="Split" :title="$t('aiSet.k17')" :description="$t('vAiUi.k203')" />
        </template>
        <template #actions="{ row }">
          <ApButton variant="ghost" size="small" @click="openModal(row as unknown as ABExperiment)">
            {{ $t('aiSet.edit') }}
          </ApButton>
          <ApButton
            variant="danger"
            size="small"
            @click="handleDelete(row as unknown as ABExperiment)"
          >
            <ApIcon name="Trash2" :size="16" /> {{ $t('aiSet.delete') }}
          </ApButton>
        </template>
      </ApTable>
    </ApCard>

    <ApModal v-model="showModal" :title="editingId ? '编辑 A/B 实验' : '新建 A/B 实验'" size="large">
      <div :style="formStyle">
        <ApInput v-model="form.name" :label="$t('aiSet.k7')" :placeholder="$t('aiSet.k14')" />
        <ApInput v-model="form.experiment_id" :label="$t('aiSet.k8')" :placeholder="$t('aiSet.k15')" />
        <ApTextarea v-model="form.description" :label="$t('aiSet.k9')" :placeholder="$t('aiSet.k16')" :rows="3" />
        <div :style="formRowStyle">
          <ApInput v-model="form.traffic_a" type="number" :label="$t('aiSet.k10')" placeholder="50" />
          <ApInput v-model="form.traffic_b" type="number" :label="$t('aiSet.k11')" placeholder="50" />
        </div>
        <div :style="switchRowStyle">
          <ApSwitch v-model="statusActive" :label="$t('aiSet.k12')" />
        </div>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showModal = false">{{ $t('aiSet.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="saving" @click="handleSave">
          {{ editingId ? $t('vAiUi.k20') : $t('vAiUi.k204') }}
        </ApButton>
      </template>
    </ApModal>

    <ApConfirmModal
      v-model="showDeleteModal"
      :title="$t('aiSet.k18')"
      :message="`确定删除实验「${deletingItem?.name}」吗？此操作不可撤销。`"
      type="danger"
      :confirm-text="$t('aiSet.delete')"
      :loading="deleting"
      @confirm="confirmDelete"
    />
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, onMounted } from 'vue';

// Split 与二级导航「A/B 实验」同符号：卡片头与空态由 GitBranch 统一到 Split。










import { logger } from '@/utils/logger';
import { appleFixedColumns, cv, AppleSpace, AppleLayout } from '@/utils/core/apple-tokens';
import { useBreakpoint } from '@/utils/responsive';
import { aiSubPageStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle, aiSubToolbarBetweenStyle as toolbarStyle, aiSubFilterStyle as spaceSelectorStyle, aiSubFilterLabelStyle as labelStyle } from '@/styles/ai/ai-page-styles';
import { ABExperimentAPI, type ABExperiment, type ABExperimentRequest } from '@/api/modules/ai/ai-ab-experiment';
import { useSpaceStore } from '@/stores/workspace/space';
import type { Space } from '@/types/models';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();
const { isMobile } = useBreakpoint();
const spaceStore = useSpaceStore();

const spaces = ref<Space[]>([]);
const selectedSpaceId = ref<number | null>(null);
const experiments = ref<ABExperiment[]>([]);
const loading = shallowRef(false);
const saving = ref(false);
const deleting = ref(false);

const showModal = ref(false);
const showDeleteModal = ref(false);
const editingId = ref<number | null>(null);
const deletingItem = ref<ABExperiment | null>(null);

const statusActive = computed({
  get: () => form.value.status === 1,
  set: (v: boolean) => {
    form.value.status = v ? 1 : 0;
  },
});

interface ABExperimentForm {
  space_id: number;
  name: string;
  experiment_id: string;
  description: string;
  traffic_a: string;
  traffic_b: string;
  status: number;
}

const form = ref<ABExperimentForm>({
  space_id: 0,
  name: '',
  experiment_id: '',
  description: '',
  traffic_a: '50',
  traffic_b: '50',
  status: 1,
});

const spaceOptions = computed(() => spaces.value.map((s) => ({ label: s.name, value: s.id })));

const columns = [
  { key: 'name', title: t('aiSet.name') },
  { key: 'experiment_id', title: t('aiSet.k8') },
  { key: 'traffic_a', title: t('aiSet.k20') },
  { key: 'traffic_b', title: t('aiSet.k21') },
  { key: 'metric_a', title: t('aiSet.k22') },
  { key: 'metric_b', title: t('aiSet.k23') },
  { key: 'status', title: t('aiSet.status') },
  { key: 'updated_at', title: t('aiSet.k25') },
];

async function loadSpaces() {
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
    const firstSpace = spaces.value[0];
    if (firstSpace && !selectedSpaceId.value) {
      selectedSpaceId.value = firstSpace.id;
      await loadExperiments();
    }
  } catch (e) {
    logger.error('Failed to load spaces:', e);
    ui.showToast(t('aiSet.k26'), 'error');
  }
}

async function loadExperiments() {
  if (!selectedSpaceId.value) return;
  loading.value = true;
  try {
    const res = await ABExperimentAPI.list({ space_id: selectedSpaceId.value });
    experiments.value = res.data || [];
  } catch (e) {
    logger.error('Failed to load A/B experiments:', e);
    ui.showToast(t('aiSet.k27'), 'error');
  } finally {
    loading.value = false;
  }
}

function onSpaceChange() {
  loadExperiments();
}

function resetForm() {
  form.value = {
    space_id: selectedSpaceId.value || 0,
    name: '',
    experiment_id: '',
    description: '',
    traffic_a: '50',
    traffic_b: '50',
    status: 1,
  };
}

function openModal(item?: ABExperiment) {
  if (item) {
    editingId.value = item.id;
    form.value = {
      space_id: item.space_id,
      name: item.name,
      experiment_id: item.experiment_id,
      description: item.description,
      traffic_a: String(item.traffic_a),
      traffic_b: String(item.traffic_b),
      status: item.status,
    };
  } else {
    editingId.value = null;
    resetForm();
  }
  showModal.value = true;
}

async function handleSave() {
  if (!selectedSpaceId.value) return;
  saving.value = true;
  try {
    const payload: ABExperimentRequest = {
      space_id: selectedSpaceId.value,
      name: form.value.name,
      experiment_id: form.value.experiment_id,
      description: form.value.description,
      traffic_a: Number(form.value.traffic_a) || 50,
      traffic_b: Number(form.value.traffic_b) || 50,
      status: form.value.status,
    };
    if (editingId.value) {
      await ABExperimentAPI.update(editingId.value, payload);
      ui.showToast(t('aiSet.k28'), 'success');
    } else {
      await ABExperimentAPI.create(payload);
      ui.showToast(t('aiSet.k29'), 'success');
    }
    showModal.value = false;
    await loadExperiments();
  } catch (e) {
    logger.error('Failed to save experiment:', e);
    ui.showToast(t('aiSet.k30'), 'error');
  } finally {
    saving.value = false;
  }
}

function handleDelete(item: ABExperiment) {
  deletingItem.value = item;
  showDeleteModal.value = true;
}

async function confirmDelete() {
  if (!deletingItem.value?.id) return;
  deleting.value = true;
  try {
    await ABExperimentAPI.delete(deletingItem.value.id);
    ui.showToast(t('aiSet.k31'), 'success');
    showDeleteModal.value = false;
    deletingItem.value = null;
    await loadExperiments();
  } catch (e) {
    logger.error('Failed to delete experiment:', e);
    ui.showToast(t('aiSet.k32'), 'error');
  } finally {
    deleting.value = false;
  }
}

onMounted(() => loadSpaces());

const formStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.md,
};

const formRowStyle = computed(() => ({ display: 'grid', gridTemplateColumns: isMobile.value ? '1fr' : appleFixedColumns(2), gap: AppleSpace.md }));

const switchRowStyle = {
  display: 'flex',
  alignItems: 'center',
  minHeight: AppleLayout.touchTargetMin,
};
</script>
