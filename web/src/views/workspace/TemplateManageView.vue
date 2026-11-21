<template>
  <ApPageContainer
    :title="$t('ws.k318')"
    :subtitle="$t('ws.k321')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <template #actions>
      <ApButton variant="primary" size="small" @click="openModal()">
        <ApIcon name="Plus" :size="16" /> {{ $t('ws.k300') }}
      </ApButton>
    </template>

    <div :style="listStyle()">
      <ApCard size="small">
        <template #header>
          <div :style="headerRowStyle">
            <div :style="actionRowStyle">
              <ApIcon name="LayoutTemplate" :size="18" :color="cv('blue')" />
              <span :style="sectionTitleStyle">{{ $t('ws.k301') }}</span>
              <span :style="countBadgeStyle">{{ total }}</span>
            </div>
            <div :style="filterStyle">
              <span :style="filterLabelStyle">{{ $t('ws.k263') }}</span>
              <ApSelect v-model="spaceId" :options="spaceOptions" size="small" searchable @update:model-value="reloadFirstPage" />
              <ApSelect v-model="pageSize" :options="pageSizeOptions" size="small" @update:model-value="reloadFirstPage" />
              <ApButton variant="ghost" size="small" :loading="loading" @click="loadList">{{ $t('ws.refresh') }}</ApButton>
            </div>
          </div>
        </template>

        <ApTable :columns="columns" :data="templates" :loading="loading" density="compact" hide-header-when-empty>
          <template #empty>
            <ApEmpty icon="LayoutTemplate" :title="$t('ws.k329')" :description="$t('ws.k330')" />
          </template>
          <template #actions="{ row }">
            <div :style="actionGroupStyle">
              <ApButton variant="secondary" size="small" @click="openPreview(row as unknown as DocumentTemplate)">
                <ApIcon name="Eye" :size="16" /> {{ $t('ws.preview') }}
              </ApButton>
              <ApButton variant="ghost" size="small" @click="openDetail(row as unknown as DocumentTemplate)">{{ $t('ws.detail') }}</ApButton>
              <ApButton variant="ghost" size="small" @click="openModal(row as unknown as DocumentTemplate)">{{ $t('ws.edit') }}</ApButton>
              <ApButton variant="danger" size="small" @click="askDelete(row as unknown as DocumentTemplate)">
                <ApIcon name="Trash2" :size="16" /> {{ $t('ws.delete') }}
              </ApButton>
            </div>
          </template>
        </ApTable>

        <ApPagination :page="page" :page-size="Number(pageSize)" :total="total" @change="goPage" />

        <p :style="noteStyle">
          {{ $t('ws.k303') }}
        </p>
      </ApCard>

      <!-- 选中模板详情 -->
      <ApCard v-if="current" size="small">
        <template #header>
          <div :style="headerRowStyle">
            <div :style="actionRowStyle">
              <ApIcon name="ScrollText" :size="18" :color="cv('purple')" />
              <span :style="sectionTitleStyle">{{ $t('ws.k304') }}</span>
              <span :style="aiSubBadgeStyle">#{{ current.id }}</span>
            </div>
            <ApButton variant="ghost" size="small" @click="current = null">{{ $t('ws.close') }}</ApButton>
          </div>
        </template>

        <div :style="appleCardGrid('entity')">
          <div>
            <span :style="filterLabelStyle">{{ $t('ws.name') }}</span>
            <p :style="noteStyle">{{ current.name }}</p>
          </div>
          <div>
            <span :style="filterLabelStyle">{{ $t('ws.k306') }}</span>
            <p :style="noteStyle">{{ spaceLabel(current.space_id) }}</p>
          </div>
          <div>
            <span :style="filterLabelStyle">{{ $t('ws.k307') }}</span>
            <p :style="noteStyle">用户 #{{ current.create_user_id }}</p>
          </div>
          <div>
            <span :style="filterLabelStyle">{{ $t('ws.k308') }}</span>
            <p :style="noteStyle">{{ formatTime(current.update_time) }}</p>
          </div>
        </div>

        <p :style="descStyle">{{ current.description || $t('fallback.k68') }}</p>

        <div :style="headerRowStyle">
          <span :style="sectionTitleStyle">{{ $t('ws.k309') }}</span>
          <ApButton variant="ghost" size="mini" @click="openPreview(current)">{{ $t('ws.k310') }}</ApButton>
        </div>
        <pre :style="bodyStyle">{{ current.content || $t('fallback.k115') }}</pre>
      </ApCard>
    </div>

    <!-- 新建 / 编辑 -->
    <ApModal v-model="showModal" :title="editingId ? '编辑模板' : '新建模板'" size="large">
      <div :style="formStyle">
        <div :style="formRowStyle">
          <ApFormItem :label="$t('ws.k311')" required :error="nameError">
            <ApInput v-model="form.name" :placeholder="$t('ws.k314')" :maxlength="100" />
          </ApFormItem>
          <ApFormItem v-if="!editingId" :label="$t('ws.k279')" :hint="$t('vWorkspaceUi.k40')">
            <ApSelect v-model="form.space_id" :options="spaceOptions" :placeholder="$t('ws.k315')" searchable />
          </ApFormItem>
        </div>

        <ApFormItem :label="$t('ws.desc')" :error="descError" :hint="editingId ? '留空则保留原描述（后端增量更新，无法清空）' : '一句话说明适用场景'">
          <ApTextarea v-model="form.description" :placeholder="$t('ws.k316')" :rows="2" :maxlength="255" />
        </ApFormItem>

        <ApFormItem :label="$t('ws.k313')" :error="contentError" :hint="$t('vWorkspaceUi.k41')">
          <ApTextarea v-model="form.content" :placeholder="$t('ws.k317')" :rows="12" mono />
        </ApFormItem>
      </div>
      <template #footer>
        <ApButton variant="ghost" size="small" @click="showModal = false">{{ $t('ws.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="saving" @click="handleSave">
          {{ editingId ? $t('vWorkspaceUi.k24') : $t('vWorkspaceUi.k42') }}
        </ApButton>
      </template>
    </ApModal>

    <!-- Markdown 渲染预览 -->
    <ApModal v-model="showPreview" :title="previewItem ? '预览：' + previewItem.name : '模板预览'" size="large">
      <ApMarkdownPreview v-if="previewItem" :content="previewItem.content || ''" />
      <ApEmpty v-else icon="FileText" :title="$t('ws.k319')" />
    </ApModal>

    <ApConfirmModal
      v-model="showDeleteModal"
      :title="$t('ws.k320')"
      :message="`确定删除模板「${deletingItem?.name ?? ''}」吗？此操作不可撤销。`"
      type="danger"
      :confirm-text="$t('ws.delete')"
      :loading="deleting"
      @confirm="confirmDelete"
    />
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted } from 'vue';
import { logger } from '@/utils/logger';
import { cv, AppleLayout, appleCardGrid } from '@/utils/core/apple-tokens';
import { formatTime } from '@/utils/format/format-time';
import {
  listStyle,
  formStyle,
  metaStyle,
  noteStyle,
  descStyle,
  actionGroupStyle,
  aiSubSectionHeaderStyle as headerRowStyle,
  aiSubSectionTitleStyle as sectionTitleStyle,
  aiSubFilterStyle as filterStyle,
  aiSubFilterLabelStyle as filterLabelStyle,
  aiSubCountBadgeStyle as countBadgeStyle,
  aiSubBadgeStyle,
  aiSubActionRowStyle as actionRowStyle,
  aiSubBodyStyle as bodyStyle,
  formRowStyle,
} from '@/styles/ai/ai-page-styles';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import {
  createTemplate,
  deleteTemplate,
  getTemplate,
  listTemplates,
  updateTemplate,
  type DocumentTemplate,
} from '@/api/modules/document/template';
import { useSpaceStore } from '@/stores/workspace/space';
import { useUIStore } from '@/stores/app/ui';
import type { Space } from '@/types/models';

const ui = useUIStore();
const spaceStore = useSpaceStore();

const spaces = ref<Space[]>([]);
const templates = ref<DocumentTemplate[]>([]);
const current = ref<DocumentTemplate | null>(null);
const loading = ref(false);
const saving = ref(false);
const deleting = ref(false);
const total = ref(0);
const page = ref(1);
const pageSize = ref<number | string>(20);
/** 0 = 仅全局模板（后端无「全部空间」查询） */
const spaceId = ref<number | string>(0);

const showModal = ref(false);
const showPreview = ref(false);
const showDeleteModal = ref(false);
const editingId = ref<number | null>(null);
const deletingItem = ref<DocumentTemplate | null>(null);
const previewItem = ref<DocumentTemplate | null>(null);
const nameError = ref('');
const descError = ref('');
const contentError = ref('');

const form = ref({ name: '', description: '', content: '', space_id: 0 as number | string });

const pageSizeOptions = [
  { label: t('ws.k322'), value: 10 },
  { label: t('ws.k323'), value: 20 },
  { label: t('ws.k324'), value: 50 },
];

const spaceOptions = computed(() => [
  { label: t('ws.k315'), value: 0 },
  ...spaces.value.map((s) => ({ label: s.name, value: s.id })),
]);

function spaceLabel(id: number): string {
  if (!id) return '全局模板';
  return spaces.value.find((s) => s.id === id)?.name ?? `空间 #${id}`;
}

const columns = computed<TableColumn<DocumentTemplate>[]>(() => [
  { key: 'name', title: t('ws.name'), width: '22%', ellipsis: true },
  { key: 'description', title: t('ws.desc'), ellipsis: true, render: (row) => row.description || '—' },
  { key: 'space_id', title: t('ws.k306'), width: '16%', render: (row) => spaceLabel(row.space_id) },
  { key: 'create_user_id', title: t('ws.k307'), width: '12%', render: (row) => `用户 #${row.create_user_id}` },
  { key: 'update_time', title: t('ws.k308'), width: '18%', render: (row) => formatTime(row.update_time) },
]);

async function loadSpaces() {
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
  } catch (e) {
    logger.error('Failed to load spaces:', e);
  }
}

async function loadList() {
  loading.value = true;
  try {
    const res = await listTemplates({
      space_id: Number(spaceId.value) || 0,
      page: page.value,
      page_size: Number(pageSize.value) || 20,
    });
    templates.value = res?.data ?? [];
    total.value = res?.total ?? 0;
    if (page.value > 1 && templates.value.length === 0) {
      page.value = Math.max(1, Math.ceil(total.value / (Number(pageSize.value) || 20)));
      await loadList();
    }
  } catch (e) {
    logger.error('Failed to load templates:', e);
    ui.showToast((e as Error).message || t('fallback.k37'), 'error');
  } finally {
    loading.value = false;
  }
}

function reloadFirstPage() {
  page.value = 1;
  loadList();
}

function goPage(next: number) {
  page.value = next;
  loadList();
}

async function openDetail(item: DocumentTemplate) {
  try {
    // 列表已含正文，这里仍按 id 拉一次详情，验证与后端数据一致
    current.value = await getTemplate(item.id);
  } catch (e) {
    logger.error('Failed to load template detail:', e);
    ui.showToast((e as Error).message || t('fallback.k38'), 'error');
    current.value = item;
  }
}

function openPreview(item: DocumentTemplate) {
  previewItem.value = item;
  showPreview.value = true;
}

function resetForm() {
  form.value = { name: '', description: '', content: '', space_id: Number(spaceId.value) || 0 };
  nameError.value = '';
  descError.value = '';
  contentError.value = '';
}

function openModal(item?: DocumentTemplate) {
  if (item) {
    editingId.value = item.id;
    form.value = {
      name: item.name,
      description: item.description,
      content: item.content,
      space_id: item.space_id,
    };
  } else {
    editingId.value = null;
    resetForm();
  }
  showModal.value = true;
}

function validate(): boolean {
  const name = form.value.name.trim();
  nameError.value = !name ? '请填写模板名称' : name.length > 100 ? '名称最长 100 字符' : '';
  descError.value = form.value.description.length > 255 ? '描述最长 255 字符' : '';
  contentError.value = '';
  return !nameError.value && !descError.value;
}

async function handleSave() {
  if (!validate()) return;
  saving.value = true;
  try {
    if (editingId.value) {
      await updateTemplate(editingId.value, {
        name: form.value.name.trim(),
        description: form.value.description.trim(),
        content: form.value.content,
      });
      ui.showToast(t('ws.k325'), 'success');
    } else {
      await createTemplate({
        name: form.value.name.trim(),
        description: form.value.description.trim(),
        content: form.value.content,
        space_id: Number(form.value.space_id) || 0,
      });
      ui.showToast(t('ws.k326'), 'success');
    }
    showModal.value = false;
    await loadList();
  } catch (e) {
    logger.error('Failed to save template:', e);
    // 同空间重名会命中唯一索引 uk_tpl_space_name，后端返回建库错误信息
    ui.showToast((e as Error).message || t('fallback.k9'), 'error');
  } finally {
    saving.value = false;
  }
}

function askDelete(item: DocumentTemplate) {
  deletingItem.value = item;
  showDeleteModal.value = true;
}

async function confirmDelete() {
  const target = deletingItem.value;
  if (!target) return;
  deleting.value = true;
  try {
    await deleteTemplate(target.id);
    ui.showToast(t('ws.k327'), 'success');
    showDeleteModal.value = false;
    deletingItem.value = null;
    if (current.value?.id === target.id) current.value = null;
    await loadList();
  } catch (e) {
    logger.error('Failed to delete template:', e);
    ui.showToast((e as Error).message || t('fallback.k21'), 'error');
  } finally {
    deleting.value = false;
  }
}

onMounted(() => {
  loadSpaces();
  loadList();
});
</script>
