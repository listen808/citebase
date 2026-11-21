<template>
  <div :style="aiSubPageStyle">
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <ApIcon name="FileText" :size="18" :color="cv('blue')" />
          <span :style="aiSubSectionTitleStyle">{{ $t('aiSet2.k1') }}</span>
        </div>
      </template>

      <div :style="toolbarStyle">
        <div :style="spaceSelectorStyle">
          <span :style="labelStyle">{{ $t('aiSet2.k2') }}</span>
          <ApSelect
            v-model="selectedSpaceId"
            :options="spaceOptions"
            :placeholder="$t('aiSet2.k12')"
            :searchable="true"
            @update:model-value="onSpaceChange"
          />
        </div>
        <ApButton variant="primary" size="small" @click="openModal()">
          <ApIcon name="Plus" :size="16" /> {{ $t('aiSet2.k3') }}
        </ApButton>
      </div>

      <ApTable density="compact" :columns="columns" :data="versions" :loading="loading">
        <template #actions="{ row }">
          <ApButton
            variant="ghost"
            size="small"
            :loading="activatingId === (row as unknown as PromptVersion).id"
            @click="handleActivate(row as unknown as PromptVersion)"
          >
            {{ $t('aiSet2.enable') }}
          </ApButton>
          <ApButton variant="ghost" size="small" @click="openModal(row as unknown as PromptVersion)">
            {{ $t('aiSet2.edit') }}
          </ApButton>
          <ApButton
            variant="danger"
            size="small"
            @click="handleDelete(row as unknown as PromptVersion)"
          >
            <ApIcon name="Trash2" :size="16" /> {{ $t('aiSet2.delete') }}
          </ApButton>
        </template>
      </ApTable>
      <ApEmpty v-if="!loading && versions.length === 0" icon="FileText" :title="$t('aiSet2.k17')" :description="$t('vAiUi.k217')" />
    </ApCard>

    <ApModal v-model="showModal" :title="editingId ? '编辑 Prompt 版本' : '新建 Prompt 版本'" size="large">
      <div :style="formStyle">
        <div :style="formRowStyle">
          <ApInput v-model="form.name" :label="$t('aiSet2.k9')" :placeholder="$t('aiSet2.k13')" />
          <ApInput v-model="form.version" type="number" :label="$t('aiSet2.k10')" placeholder="1" />
        </div>
        <div :style="formRowStyle">
          <div>
            <div :style="labelStyle">{{ $t('aiSet2.k7') }}</div>
            <ApSelect v-model="form.query_type" :options="queryTypeOptions" :placeholder="$t('aiSet2.k14')" />
          </div>
          <div :style="switchRowStyle">
            <ApSwitch v-model="form.active" :label="$t('aiSet2.k11')" />
          </div>
        </div>
        <ApTextarea
          v-model="form.system_prompt"
          label="System Prompt"
          :placeholder="$t('aiSet2.k15')"
          :rows="6"
        />
        <ApTextarea
          v-model="form.user_template"
          label="User Template"
          :placeholder="$t('aiSet2.k16')"
          :rows="6"
        />
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showModal = false">{{ $t('aiSet2.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="saving" @click="handleSave">
          {{ editingId ? $t('vAiUi.k20') : $t('vAiUi.k218') }}
        </ApButton>
      </template>
    </ApModal>

    <ApConfirmModal
      v-model="showDeleteModal"
      :title="$t('aiSet2.k18')"
      :message="`确定删除 Prompt 版本「${deletingItem?.name}」吗？此操作不可撤销。`"
      type="danger"
      :confirm-text="$t('aiSet2.delete')"
      :loading="deleting"
      @confirm="confirmDelete"
    />
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, computed, onMounted } from 'vue';
import { useBreakpoint } from '@/utils/responsive';

const { isMobile } = useBreakpoint();










import { logger } from '@/utils/logger';
import { cv, AppleSpace, AppleLayout, appleFixedColumns } from '@/utils/core/apple-tokens';
import { aiSubPageStyle, aiSubSectionHeaderStyle, aiSubSectionTitleStyle, aiSubToolbarBetweenStyle as toolbarStyle, aiSubFilterStyle as spaceSelectorStyle, aiSubFilterLabelStyle as labelStyle } from '@/styles/ai/ai-page-styles';
import { PromptVersionAPI, type PromptVersion, type PromptVersionRequest } from '@/api/modules/ai/ai-prompt-version';
import { useSpaceStore } from '@/stores/workspace/space';
import type { Space } from '@/types/models';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();
const spaceStore = useSpaceStore();

const spaces = ref<Space[]>([]);
const selectedSpaceId = ref<number | null>(null);
const versions = ref<PromptVersion[]>([]);
const loading = shallowRef(false);
const saving = ref(false);
const activatingId = ref<number | null>(null);
const deleting = ref(false);

const showModal = ref(false);
const showDeleteModal = ref(false);
const editingId = ref<number | null>(null);
const deletingItem = ref<PromptVersion | null>(null);

interface PromptVersionForm {
  space_id: number;
  name: string;
  query_type: string;
  version: string;
  system_prompt: string;
  user_template: string;
  active: boolean;
  experiment_id?: string;
}

const form = ref<PromptVersionForm>({
  space_id: 0,
  name: '',
  query_type: 'text2sql',
  version: '1',
  system_prompt: '',
  user_template: '',
  active: false,
});

const spaceOptions = computed(() => spaces.value.map((s) => ({ label: s.name, value: s.id })));

const queryTypeOptions = [
  { label: 'Text-to-SQL', value: 'text2sql' },
  { label: t('aiSet2.k19'), value: 'chart' },
  { label: t('aiSet2.k20'), value: 'explain' },
  { label: t('aiSet2.k21'), value: 'insight' },
];

const columns = [
  { key: 'name', title: t('aiSet2.name') },
  { key: 'query_type', title: t('aiSet2.k7') },
  { key: 'version', title: t('aiSet2.k10') },
  { key: 'active', title: t('aiSet2.status') },
  { key: 'updated_at', title: t('aiSet2.k24') },
];

async function loadSpaces() {
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
    const firstSpace = spaces.value[0];
    if (firstSpace && !selectedSpaceId.value) {
      selectedSpaceId.value = firstSpace.id;
      await loadVersions();
    }
  } catch (e) {
    logger.error('Failed to load spaces:', e);
    ui.showToast(t('aiSet2.k25'), 'error');
  }
}

async function loadVersions() {
  if (!selectedSpaceId.value) return;
  loading.value = true;
  try {
    const res = await PromptVersionAPI.list({ space_id: selectedSpaceId.value });
    versions.value = res.data ?? [];
  } catch (e) {
    logger.error('Failed to load prompt versions:', e);
    ui.showToast(t('aiSet2.k26'), 'error');
  } finally {
    loading.value = false;
  }
}

function onSpaceChange() {
  loadVersions();
}

function resetForm() {
  form.value = {
    space_id: selectedSpaceId.value || 0,
    name: '',
    query_type: 'text2sql',
    version: '1',
    system_prompt: '',
    user_template: '',
    active: false,
  };
}

function openModal(item?: PromptVersion) {
  if (item) {
    editingId.value = item.id;
    form.value = {
      space_id: item.space_id,
      name: item.name,
      query_type: item.query_type,
      version: String(item.version),
      system_prompt: item.system_prompt,
      user_template: item.user_template,
      active: item.active,
      experiment_id: item.experiment_id,
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
    const payload: PromptVersionRequest = {
      ...form.value,
      space_id: selectedSpaceId.value,
      version: Number(form.value.version) || 1,
    };
    if (editingId.value) {
      await PromptVersionAPI.update(editingId.value, payload);
      ui.showToast(t('aiSet2.k27'), 'success');
    } else {
      await PromptVersionAPI.create(payload);
      ui.showToast(t('aiSet2.k28'), 'success');
    }
    showModal.value = false;
    await loadVersions();
  } catch (e) {
    logger.error('Failed to save prompt version:', e);
    ui.showToast(t('aiSet2.k29'), 'error');
  } finally {
    saving.value = false;
  }
}

async function handleActivate(item: PromptVersion) {
  if (!item.id) return;
  activatingId.value = item.id;
  try {
    await PromptVersionAPI.activate(item.id);
    ui.showToast(t('aiSet2.k30'), 'success');
    await loadVersions();
  } catch (e) {
    logger.error('Failed to activate prompt version:', e);
    ui.showToast(t('aiSet2.k31'), 'error');
  } finally {
    activatingId.value = null;
  }
}

function handleDelete(item: PromptVersion) {
  deletingItem.value = item;
  showDeleteModal.value = true;
}

async function confirmDelete() {
  if (!deletingItem.value?.id) return;
  deleting.value = true;
  try {
    await PromptVersionAPI.delete(deletingItem.value.id);
    ui.showToast(t('aiSet2.k32'), 'success');
    showDeleteModal.value = false;
    deletingItem.value = null;
    await loadVersions();
  } catch (e) {
    logger.error('Failed to delete prompt version:', e);
    ui.showToast(t('aiSet2.k33'), 'error');
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

// 与 AIABExperimentView 同款响应式两列（移动端降单列）
const formRowStyle = computed(() => ({
  display: 'grid',
  gridTemplateColumns: isMobile.value ? '1fr' : appleFixedColumns(2),
  gap: AppleSpace.md,
}));

const switchRowStyle = {
  display: 'flex',
  alignItems: 'center',
  minHeight: AppleLayout.touchTargetMin,
};
</script>
