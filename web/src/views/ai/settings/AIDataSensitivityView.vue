<template>
  <div :style="aiSubPageStyle">
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <div :style="aiSubCardNameRowStyle">
            <ApIcon name="Lock" :size="18" :color="cv('red')" />
            <span :style="aiSubSectionTitleStyle">{{ $t('aiSet.k83') }}</span>
            <span :style="aiSubCountBadgeStyle">{{ rules.length }}</span>
          </div>
          <ApButton variant="primary" size="small" @click="openModal()">
            <ApIcon name="Plus" :size="16" /> {{ $t('aiSet.k84') }}
          </ApButton>
        </div>
      </template>

      <div :style="filterBarStyle">
        <div :style="spaceSelectorStyle">
          <span :style="aiSubFilterLabelStyle">{{ $t('aiSet.k35') }}</span>
          <ApSelect
            v-model="selectedSpaceId"
            :options="spaceOptions"
            :placeholder="$t('aiSet.k13')"
            :searchable="true"
            @update:model-value="loadRules"
          />
        </div>
        <ApButton variant="secondary" size="small" :loading="loading" @click="loadRules">
          <ApIcon name="RotateCw" :size="16" /> {{ $t('aiSet.refresh') }}
        </ApButton>
      </div>

      <ApTable density="compact" :columns="columns" :data="rules" :loading="loading || deleting">
        <template #empty>
          <ApEmpty icon="Lock" :title="$t('aiSet.k97')" :description="$t('vAiUi.k209')" />
        </template>
        <template #actions="{ row }">
          <ApButton
            variant="ghost"
            size="small"
            @click="openModal(row as unknown as DataSensitivityRule)"
          >
            {{ $t('aiSet.edit') }}
          </ApButton>
          <ApButton
            variant="danger"
            size="small"
            @click="handleDelete(row as unknown as DataSensitivityRule)"
          >
            <ApIcon name="Trash2" :size="16" /> {{ $t('aiSet.delete') }}
          </ApButton>
        </template>
      </ApTable>
    </ApCard>

    <ApModal v-model="showModal" :title="editingId ? '编辑敏感性规则' : '新建敏感性规则'" size="medium">
      <div :style="formStyle">
        <div :style="formRowStyle">
          <ApInput v-model="form.table_name" :label="$t('aiSet.k88')" :placeholder="$t('aiSet.k91')" required />
          <ApInput v-model="form.field_name" :label="$t('aiSet.k89')" :placeholder="$t('aiSet.k92')" required />
        </div>
        <div :style="formRowStyle">
          <div>
            <div :style="aiSubFilterLabelStyle">{{ $t('aiSet.k85') }}</div>
            <ApSelect v-model="form.level" :options="levelOptions" :placeholder="$t('aiSet.k93')" />
          </div>
          <div>
            <div :style="aiSubFilterLabelStyle">{{ $t('aiSet.k86') }}</div>
            <ApSelect v-model="form.mask_type" :options="maskOptions" :placeholder="$t('aiSet.k94')" />
          </div>
        </div>
        <ApTextarea
          v-model="form.mask_rule"
          :label="$t('aiSet.k90')"
          :placeholder="$t('aiSet.k95')"
          :rows="3"
        />
        <ApInput v-model="form.description" :label="$t('aiSet.k77')" :placeholder="$t('aiSet.k96')" />
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showModal = false">{{ $t('aiSet.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="saving" @click="handleSave">{{ $t('aiSet.save') }}</ApButton>
      </template>
    </ApModal>

    <ApConfirmModal
      v-model="showDeleteModal"
      :title="$t('aiSet.k98')"
      :message="`确定删除「${deletingItem?.table_name}.${deletingItem?.field_name}」规则吗？此操作不可撤销。`"
      type="danger"
      :confirm-text="$t('aiSet.delete')"
      @confirm="confirmDelete"
    />
  </div>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted } from 'vue';
// Lock 与二级导航「数据敏感性」同符号：本页卡片头 / 空态都改画 Lock，不再各用 ShieldCheck / Shield。

import { logger } from '@/utils/logger';
import { cv } from '@/utils/core/apple-tokens';
import {
  aiSubPageStyle,
  aiSubSectionHeaderStyle,
  aiSubSectionTitleStyle,
  aiSubCardNameRowStyle,
  aiSubCountBadgeStyle,
  aiSubFilterLabelStyle,
  aiSubFilterStyle as spaceSelectorStyle,
  filterBarStyle,
  formStyle,
  formRowStyle,
  AI_TABLE_WIDTH,
} from '@/styles/ai/ai-page-styles';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import {
  DataSensitivityAPI,
  type DataSensitivityRule,
  type SensitivityLevel,
} from '@/api/modules/ai/ai-data-sensitivity';
import { useSpaceStore } from '@/stores/workspace/space';
import type { Space } from '@/types/models';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();
const spaceStore = useSpaceStore();

const spaces = ref<Space[]>([]);
const selectedSpaceId = ref<number | null>(null);
const rules = ref<DataSensitivityRule[]>([]);
const loading = ref(false);
const saving = ref(false);
const deleting = ref(false);

const showModal = ref(false);
const showDeleteModal = ref(false);
const editingId = ref<number | null>(null);
const deletingItem = ref<DataSensitivityRule | null>(null);

const form = ref({
  table_name: '',
  field_name: '',
  level: 'medium' as string,
  mask_type: 'partial' as string,
  mask_rule: '',
  description: '',
});

const levelOptions = [
  { label: t('aiSet.k99'), value: 'low' },
  { label: t('aiSet.k100'), value: 'medium' },
  { label: t('aiSet.k101'), value: 'high' },
  { label: t('aiSet.k102'), value: 'critical' },
];

const maskOptions = [
  { label: t('aiSet.k103'), value: 'full' },
  { label: t('aiSet.k104'), value: 'partial' },
  { label: t('aiSet.k105'), value: 'hash' },
  { label: t('aiSet.k106'), value: 'regex' },
];

const LEVEL_LABELS: Record<string, string> = {
  low: '低',
  medium: '中',
  high: '高',
  critical: '极高',
};

const MASK_LABELS: Record<string, string> = {
  full: '全量遮蔽',
  partial: '部分遮蔽',
  hash: '哈希',
  regex: '正则替换',
};

const spaceOptions = computed(() => spaces.value.map((s) => ({ label: s.name, value: s.id })));

const columns: TableColumn<DataSensitivityRule>[] = [
  { key: 'table_name', title: t('aiSet.k88'), width: AI_TABLE_WIDTH.xl },
  { key: 'field_name', title: t('aiSet.k107'), width: AI_TABLE_WIDTH.lg },
  { key: 'level', title: t('aiSet.k108'), width: AI_TABLE_WIDTH.sm, render: (row) => LEVEL_LABELS[row.level] ?? row.level },
  { key: 'mask_type', title: t('aiSet.k86'), width: AI_TABLE_WIDTH.md, render: (row) => MASK_LABELS[row.mask_type] ?? (row.mask_type || '—') },
  { key: 'mask_rule', title: t('aiSet.k109'), ellipsis: true },
  { key: 'description', title: t('aiSet.k77'), ellipsis: true },
  { key: 'updated_at', title: t('aiSet.k25'), width: AI_TABLE_WIDTH.lg },
];

async function loadSpaces() {
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
    const firstSpace = spaces.value[0];
    if (firstSpace && !selectedSpaceId.value) {
      selectedSpaceId.value = firstSpace.id;
      await loadRules();
    }
  } catch (e) {
    logger.error('Failed to load spaces:', e);
    ui.showToast(t('aiSet.k26'), 'error');
  }
}

async function loadRules() {
  // 后端 space_id 必填，缺失会返回 400 "space_id 无效"
  if (!selectedSpaceId.value) {
    rules.value = [];
    return;
  }
  loading.value = true;
  try {
    const res = await DataSensitivityAPI.listRules(selectedSpaceId.value);
    rules.value = res ?? [];
  } catch (e) {
    logger.error('Failed to load sensitivity rules:', e);
    ui.showToast(t('aiSet.k110'), 'error');
  } finally {
    loading.value = false;
  }
}

function openModal(rule?: DataSensitivityRule) {
  if (rule) {
    editingId.value = rule.id;
    form.value = {
      table_name: rule.table_name,
      field_name: rule.field_name,
      level: rule.level,
      mask_type: rule.mask_type || 'partial',
      mask_rule: rule.mask_rule || '',
      description: rule.description || '',
    };
  } else {
    editingId.value = null;
    form.value = {
      table_name: '',
      field_name: '',
      level: 'medium',
      mask_type: 'partial',
      mask_rule: '',
      description: '',
    };
  }
  showModal.value = true;
}

async function handleSave() {
  if (!selectedSpaceId.value) {
    ui.showToast(t('aiSet.k111'), 'warning');
    return;
  }
  if (!form.value.table_name.trim() || !form.value.field_name.trim()) {
    ui.showToast(t('aiSet.k112'), 'warning');
    return;
  }
  const level = form.value.level as SensitivityLevel;
  if (!LEVEL_LABELS[level]) {
    ui.showToast(t('aiSet.k113'), 'warning');
    return;
  }
  saving.value = true;
  try {
    const payload = {
      space_id: selectedSpaceId.value,
      table_name: form.value.table_name.trim(),
      field_name: form.value.field_name.trim(),
      level,
      mask_type: form.value.mask_type,
      mask_rule: form.value.mask_rule,
      description: form.value.description,
    };
    if (editingId.value) {
      await DataSensitivityAPI.updateRule(editingId.value, payload);
      ui.showToast(t('aiSet.k114'), 'success');
    } else {
      await DataSensitivityAPI.createRule(payload);
      ui.showToast(t('aiSet.k115'), 'success');
    }
    showModal.value = false;
    await loadRules();
  } catch (e) {
    logger.error('Failed to save sensitivity rule:', e);
    ui.showToast(t('aiSet.k30'), 'error');
  } finally {
    saving.value = false;
  }
}

function handleDelete(rule: DataSensitivityRule) {
  deletingItem.value = rule;
  showDeleteModal.value = true;
}

async function confirmDelete() {
  const target = deletingItem.value;
  if (!target?.id) return;
  deleting.value = true;
  try {
    await DataSensitivityAPI.deleteRule(target.id);
    ui.showToast(t('aiSet.k116'), 'success');
    showDeleteModal.value = false;
    deletingItem.value = null;
    await loadRules();
  } catch (e) {
    logger.error('Failed to delete sensitivity rule:', e);
    ui.showToast(t('aiSet.k32'), 'error');
  } finally {
    deleting.value = false;
  }
}

onMounted(() => loadSpaces());
</script>
