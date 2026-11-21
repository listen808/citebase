<template>
  <div :style="aiSubPageStyle">
    <ApCard size="small">
      <template #header>
        <div :style="aiSubSectionHeaderStyle">
          <div :style="aiSubCardNameRowStyle">
            <ApIcon name="Layers" :size="18" :color="cv('orange')" />
            <span :style="aiSubSectionTitleStyle">{{ $t('aiSet2.k85') }}</span>
            <span :style="aiSubCountBadgeStyle">{{ total }}</span>
          </div>
          <span :style="aiSubSectionMetaStyle">{{ $t('aiSet2.k86') }}</span>
        </div>
      </template>

      <div :style="toolbarStyle">
        <div :style="spaceSelectorStyle">
          <span :style="labelStyle">{{ $t('aiSet2.k87') }}</span>
          <ApSelect
            v-model="selectedSpaceId"
            :options="spaceOptions"
            :placeholder="$t('aiSet2.k12')"
            :searchable="true"
            @update:model-value="onFilterChange"
          />
        </div>
        <ApSegmentedControl v-model="verifiedFilter" :options="verifiedOptions" size="small" @update:model-value="onFilterChange" />
        <ApButton variant="primary" size="small" @click="openModal()">
          <ApIcon name="Plus" :size="16" /> {{ $t('aiSet2.k88') }}
        </ApButton>
      </div>

      <ApTable :columns="columns" :data="examples" :loading="loading" density="compact">
        <template #empty>
          <ApEmpty icon="Layers" :title="$t('aiSet2.k96')" :description="$t('vAiUi.k224')" />
        </template>
        <template #actions="{ row }">
          <ApButton
            v-if="(row as unknown as SQLExample).verified !== 1"
            variant="outline"
            size="small"
            :loading="verifyingId === (row as unknown as SQLExample).id"
            @click="handleVerify(row as unknown as SQLExample)"
          >
            <ApIcon name="BadgeCheck" :size="16" /> {{ $t('aiSet2.k89') }}
          </ApButton>
          <ApButton variant="ghost" size="small" @click="openModal(row as unknown as SQLExample)">
            {{ $t('aiSet2.edit') }}
          </ApButton>
          <ApButton variant="danger" size="small" @click="handleDelete(row as unknown as SQLExample)">
            <ApIcon name="Trash2" :size="16" /> {{ $t('aiSet2.delete') }}
          </ApButton>
        </template>
      </ApTable>

      <ApPagination
        v-if="total > PAGE_SIZE"
        :total="total"
        :page="page"
        :page-size="PAGE_SIZE"
        @change="changePage"
      />
    </ApCard>

    <ApModal v-model="showModal" :title="editingId ? '编辑少样本示例' : '新建少样本示例'" size="medium">
      <div :style="formStyle">
        <ApTextarea
          v-model="form.question"
          :label="$t('aiSet2.k92')"
          :placeholder="$t('aiSet2.k95')"
          :rows="2"
        />
        <ApTextarea
          v-model="form.sql"
          :label="$t('aiSet2.k93')"
          placeholder="SELECT ..."
          :rows="6"
        />
        <div :style="formRowStyle">
          <div>
            <span :style="labelStyle">{{ $t('aiSet2.k90') }}</span>
            <ApSelect v-model="form.space_id" :options="spaceOptions" :placeholder="$t('aiSet2.k12')" />
          </div>
          <div>
            <span :style="labelStyle">{{ $t('aiSet2.k91') }}</span>
            <span :style="aiSubBadgeStyle">{{ editingItem?.db_type || $t('fallback.k5') }}</span>
          </div>
        </div>
        <ApSwitch v-model="formVerified" :label="$t('aiSet2.k94')" />
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showModal = false">{{ $t('aiSet2.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="saving" @click="handleSave">
          {{ editingId ? $t('vAiUi.k20') : $t('vAiUi.k225') }}
        </ApButton>
      </template>
    </ApModal>

    <ApConfirmModal
      v-model="showDeleteModal"
      :title="$t('aiSet2.k97')"
      :message="`确定删除示例「${deletingItem?.question ?? ''}」吗？此操作不可撤销。`"
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
// Layers 与二级导航「少样本示例」同符号：卡片头由 Lightbulb、空态由 FileText 统一到 Layers。

import { logger } from '@/utils/logger';
import { cv } from '@/utils/core/apple-tokens';
import { formatTime } from '@/utils/format/format-time';
import {
  aiSubPageStyle,
  aiSubSectionHeaderStyle,
  aiSubSectionTitleStyle,
  aiSubSectionMetaStyle,
  aiSubCardNameRowStyle,
  aiSubCountBadgeStyle,
  aiSubBadgeStyle,
  aiSubToolbarBetweenStyle as toolbarStyle,
  aiSubFilterStyle as spaceSelectorStyle,
  aiSubFilterLabelStyle as labelStyle,
  AI_TABLE_WIDTH,
  formStyle,
  formRowStyle,
} from '@/styles/ai/ai-page-styles';
import ApTable from '@/components/apple/ApTable.vue';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import { SQLExampleAPI, type SQLExample } from '@/api/modules/ai/ai-sql-example';
import { useSpaceStore } from '@/stores/workspace/space';
import type { Space } from '@/types/models';
import { useUIStore } from '@/stores/app/ui';

const ui = useUIStore();
const spaceStore = useSpaceStore();

const PAGE_SIZE = 20;

interface SQLExampleForm {
  question: string;
  sql: string;
  space_id: number;
  verified: number;
}

const spaces = ref<Space[]>([]);
const selectedSpaceId = ref<number | null>(null);
// 后端 verified=1 才过滤，其它值（含不传）都是「全部」
const verifiedFilter = ref<number>(0);
const examples = ref<SQLExample[]>([]);
const total = ref(0);
const page = ref(1);
const loading = shallowRef(false);

const showModal = ref(false);
const saving = ref(false);
const editingId = ref<number | null>(null);
const editingItem = ref<SQLExample | null>(null);
const verifyingId = ref<number | null>(null);
const deleting = ref(false);
const showDeleteModal = ref(false);
const deletingItem = ref<SQLExample | null>(null);

const form = ref<SQLExampleForm>({ question: '', sql: '', space_id: 0, verified: 0 });

const formVerified = computed({
  get: () => form.value.verified === 1,
  set: (value: boolean) => {
    form.value.verified = value ? 1 : 0;
  },
});

const spaceOptions = computed(() => spaces.value.map((s) => ({ label: s.name, value: s.id })));

const verifiedOptions = [
  { label: t('aiSet2.all'), value: 0 },
  { label: t('aiSet2.k99'), value: 1 },
];

const columns: TableColumn<SQLExample>[] = [
  { key: 'question', title: t('aiSet2.k45'), ellipsis: true },
  { key: 'sql', title: 'SQL', ellipsis: true },
  { key: 'db_type', title: t('aiSet2.k100'), width: AI_TABLE_WIDTH.sm },
  { key: 'verified', title: t('aiSet2.status'), width: AI_TABLE_WIDTH.sm, render: (row) => (row.verified === 1 ? '已验证' : '未验证') },
  { key: 'created_at', title: t('aiSet2.k82'), width: AI_TABLE_WIDTH.lg, render: (row) => formatTime(row.created_at) },
];

async function loadSpaces() {
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
    const first = spaces.value[0];
    if (first && selectedSpaceId.value === null) selectedSpaceId.value = first.id;
  } catch (e) {
    logger.error('Failed to load spaces:', e);
    ui.showToast(t('aiSet2.k25'), 'error');
  }
}

async function loadExamples() {
  if (selectedSpaceId.value === null) return;
  loading.value = true;
  try {
    const res = await SQLExampleAPI.list({
      space_id: selectedSpaceId.value,
      verified: verifiedFilter.value === 1 ? 1 : undefined,
      page: page.value,
      page_size: PAGE_SIZE,
    });
    examples.value = res.data ?? [];
    total.value = res.total ?? 0;
  } catch (e) {
    logger.error('Failed to load sql examples:', e);
    ui.showToast(t('aiSet2.k101'), 'error');
  } finally {
    loading.value = false;
  }
}

function onFilterChange() {
  page.value = 1;
  loadExamples();
}

function changePage(next: number) {
  page.value = next;
  loadExamples();
}

function resetForm() {
  editingId.value = null;
  editingItem.value = null;
  form.value = { question: '', sql: '', space_id: selectedSpaceId.value ?? 0, verified: 0 };
}

function openModal(item?: SQLExample) {
  if (item) {
    editingId.value = item.id;
    editingItem.value = item;
    form.value = { question: item.question, sql: item.sql, space_id: item.space_id, verified: item.verified };
  } else {
    resetForm();
  }
  showModal.value = true;
}

async function handleSave() {
  if (!form.value.question.trim() || !form.value.sql.trim()) {
    ui.showToast(t('aiSet2.k102'), 'error');
    return;
  }
  saving.value = true;
  try {
    if (editingId.value) {
      // 后端 PUT 只接受 question / sql / verified，space_id 不可改
      await SQLExampleAPI.update(editingId.value, {
        question: form.value.question,
        sql: form.value.sql,
        verified: form.value.verified,
      });
      ui.showToast(t('aiSet2.k103'), 'success');
    } else {
      await SQLExampleAPI.create({
        question: form.value.question,
        sql: form.value.sql,
        space_id: form.value.space_id,
        verified: form.value.verified,
      });
      ui.showToast(t('aiSet2.k104'), 'success');
    }
    showModal.value = false;
    await loadExamples();
  } catch (e) {
    logger.error('Failed to save sql example:', e);
    ui.showToast(t('aiSet2.k29'), 'error');
  } finally {
    saving.value = false;
  }
}

async function handleVerify(item: SQLExample) {
  verifyingId.value = item.id;
  try {
    await SQLExampleAPI.verify(item.id);
    ui.showToast(t('aiSet2.k105'), 'success');
    await loadExamples();
  } catch (e) {
    logger.error('Failed to verify sql example:', e);
    ui.showToast(t('aiSet2.k106'), 'error');
  } finally {
    verifyingId.value = null;
  }
}

function handleDelete(item: SQLExample) {
  deletingItem.value = item;
  showDeleteModal.value = true;
}

async function confirmDelete() {
  const target = deletingItem.value;
  if (!target) return;
  deleting.value = true;
  try {
    await SQLExampleAPI.remove(target.id);
    ui.showToast(t('aiSet2.k107'), 'success');
    showDeleteModal.value = false;
    deletingItem.value = null;
    await loadExamples();
  } catch (e) {
    logger.error('Failed to delete sql example:', e);
    ui.showToast(t('aiSet2.k33'), 'error');
  } finally {
    deleting.value = false;
  }
}

onMounted(async () => {
  await loadSpaces();
  await loadExamples();
});
</script>
