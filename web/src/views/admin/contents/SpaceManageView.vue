<template>
  <ApPageContainer
    :title="$t('admin.k88')"
    :subtitle="$t('admin.k90')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <AdminCrudPage
      v-model:keyword="keyword"
      v-model:view-mode="viewMode"
      :error-text="loadError"
      :stats="spaceStats"
      :filters="{ search: { placeholder: $t('vAdminUi.k11') }, viewToggle: true }"
      :loading="loading"
      density="compact"
      @retry="retryLoad"
    >
      <template #filter-extra>
        <ApButton v-if="viewMode === 'card' && !sortMode && filteredSpaces.length > 1" variant="ghost" size="small" @click="enterSortMode">
          <ApIcon name="ArrowUpNarrowWide" :size="16" />
          {{ $t('admin.sort') }}
        </ApButton>
        <ApButton v-if="viewMode === 'card' && sortMode" variant="primary" size="small" :disabled="sortSaving" @click="exitSortMode">
          <ApIcon name="Check" :size="16" />
          {{ sortSaving ? $t('vAdminUi.k1') : $t('vAdminUi.k8') }}
        </ApButton>
        <!-- 定宽：ApSelect 根是 width:100%，不兜住会把整条工具条撑到换行 -->
        <div :style="filterSelectGroupStyle()">
          <ApSelect v-model="filterVisitLevel" :options="filterVisitLevelOptions" :placeholder="$t('admin.k80')" />
        </div>
        <ApInput v-model="filterTags" :aria-label="$t('admin.k84')" :placeholder="$t('admin.k84')" :style="{ maxWidth: '200px' }" />
        <ApButton v-if="filterVisitLevel || filterTags" variant="ghost" size="small" @click="clearFilters">{{ $t('admin.k74') }}</ApButton>
      </template>

      <template #actions>
        <ApButton variant="primary" size="small" @click="openCreateModal">
          <ApIcon name="Plus" :size="16" />{{ $t('admin.k75') }}
        </ApButton>
      </template>

      <template #content>
        <template v-if="viewMode === 'card'">
          <div v-if="loading && !filteredSpaces.length" :style="loadingStyle">
            <ApSkeleton variant="card" :count="3" />
          </div>
          <ApEmpty
            v-else-if="!filteredSpaces.length"
            :icon="keyword ? 'Search' : 'FolderOpen'"
            :title="keyword ? $t('adminForm.k62') : $t('adminForm.k63')"
          />
          <div v-else :style="spacesGridStyle">
            <ApEntityCard
              v-for="(space, idx) in displaySpaces"
              :key="space.id"
              :title="space.name"
              :subtitle="space.description || undefined"
              @click="!sortMode && toggleSpaceExpand(space.id)"
            >
              <div :style="entityMetaRowStyle()">
                <span v-if="sortMode" :style="sequenceBadgeStyle()">
                  {{ idx + 1 }}
                </span>
                <span :style="entityMetaItemStyle()">
                  <ApIcon name="Users" :size="12" />
                  {{ space.member_count ?? 0 }} {{ $t('vAdminUi.k9') }}
                </span>
                <span :style="entityMetaItemStyle()">
                  <ApIcon name="FileText" :size="12" />
                  {{ space.doc_count ?? 0 }} {{ $t('vAdminUi.k10') }}
                </span>
                <template v-if="sortMode">
                  <ApButton
                    variant="ghost"
                    size="mini"
                    icon-only
                    :aria-label="$t('admin.k91')"
                    :disabled="idx === 0"
                    :style="{ marginLeft: AppleSpace.sm }"
                    @click.stop="moveSpace(idx, -1)"
                  >
                    <ApIcon name="ChevronUp" :size="16" />
                  </ApButton>
                  <ApButton
                    variant="ghost"
                    size="mini"
                    icon-only
                    :aria-label="$t('admin.k92')"
                    :disabled="idx === displaySpaces.length - 1"
                    @click.stop="moveSpace(idx, 1)"
                  >
                    <ApIcon name="ChevronDown" :size="16" />
                  </ApButton>
                </template>
              </div>
            </ApEntityCard>
          </div>
          <div v-if="expandedSpaceId && expandedSpace" :style="expandPanelStyle">
            <div :style="expandTabBarStyle">
              <ApSegmentedControl v-model="expandActiveTab" :options="expandTabs" size="small" />
            </div>
            <div :style="expandContentStyle">
              <template v-if="expandActiveTab === 'members'">
                <div :style="expandInfoStyle">{{ $t('admin.k186', { n: expandedSpace?.member_count ?? 0 }) }} · <router-link :to="'/space/' + expandedSpace?.id" :style="{ color: cv('blue') }">{{ $t('admin.k76') }}</router-link></div>
              </template>
              <template v-else-if="expandActiveTab === 'docs'">
                <div :style="expandInfoStyle">{{ $t('admin.k187', { n: expandedSpace?.doc_count ?? 0 }) }}</div>
              </template>
              <template v-else-if="expandActiveTab === 'settings'">
                <div :style="expandInfoStyle">
                  <div>{{ t('common.createdBy', { name: expandedSpace?.creator_name || t('time.unknown') }) }}</div>
                  <div>{{ $t('admin.k188', { n: visitLevelText(expandedSpace?.visit_level || '') }) }}</div>
                  <div>{{ $t('admin.k189', { n1: expandedSpace?.is_share === 1 ? $t('admin.k190') : $t('admin.k191'), n2: expandedSpace?.is_export === 1 ? $t('admin.k190') : $t('admin.k191') }) }}</div>
                </div>
              </template>
            </div>
            <div :style="actionGroupStyle">
              <ApButton variant="ghost" size="small" @click.stop="openEditModal(expandedSpace)"><ApIcon name="Pencil" :size="16" />{{ $t('admin.edit') }}</ApButton>
              <ApButton variant="ghost" size="small" @click.stop="router.push('/space/' + expandedSpace?.id)"><ApIcon name="Users" :size="16" />{{ $t('admin.k77') }}</ApButton>
              <ApButton variant="ghost" size="small" @click.stop="handleDownload(expandedSpace)"><ApIcon name="Download" :size="16" />{{ $t('admin.download') }}</ApButton>
              <ApButton variant="danger" size="small" @click.stop="openDeleteModal(expandedSpace)"><ApIcon name="Trash2" :size="16" />{{ $t('admin.delete') }}</ApButton>
            </div>
          </div>
        </template>
        <template v-else>
          <ApTable :columns="spaceColumns" :data="filteredSpaces" :loading="loading" density="compact">
            <template #actions="{ row: space }">
              <div :style="actionGroupStyle">
                <ApButton variant="ghost" size="small" @click="openEditModal(space)">{{ $t('admin.edit') }}</ApButton>
                <ApButton variant="ghost" size="small" @click="router.push('/space/' + space.id)"><ApIcon name="Users" :size="16" />{{ $t('admin.k77') }}</ApButton>
                <ApButton variant="ghost" size="small" @click="handleDownload(space)"><ApIcon name="Download" :size="16" />{{ $t('admin.download') }}</ApButton>
                <ApButton variant="danger" size="small" @click="openDeleteModal(space)"><ApIcon name="Trash2" :size="16" />{{ $t('admin.delete') }}</ApButton>
              </div>
            </template>
          </ApTable>
        </template>
      </template>

      <template #modals>
        <ApModal :model-value="showCreateModal || showEditModal" :title="isEdit ? $t('adminForm.k11') : $t('adminForm.k12')" size="medium" density="compact" @update:model-value="(v: boolean) => { if (!v) closeModals() }">
          <ApFormItem :label="$t('admin.k78')" required :error="formErrors.name">
            <ApInput v-model="form.name" :placeholder="$t('admin.k85')" />
          </ApFormItem>
          <ApFormItem :label="$t('admin.k79')" :error="formErrors.description">
            <ApTextarea v-model="form.description" :placeholder="$t('admin.k86')" :rows="3" />
          </ApFormItem>
          <ApFormItem :label="$t('admin.k80')">
            <ApSelect v-model="form.visit_level" :options="visitLevelOptions" />
          </ApFormItem>
          <ApFormItem :label="$t('admin.k81')">
            <ApSwitch v-model="form.is_share" />
          </ApFormItem>
          <ApFormItem :label="$t('admin.k82')">
            <ApSwitch v-model="form.is_export" />
          </ApFormItem>
          <ApFormItem :label="$t('admin.k83')">
            <ApInput v-model="form.tags" :placeholder="$t('admin.k87')" />
          </ApFormItem>
          <template #footer>
            <ApButton variant="ghost" size="small" @click="closeModals">{{ $t('admin.cancel') }}</ApButton>
            <ApButton variant="primary" size="small" :disabled="submitting" @click="onSave">{{ submitting ? $t('vAdminUi.k1') : $t('vAdminUi.save') }}</ApButton>
          </template>
        </ApModal>

        <ApConfirmModal
          v-model="showDeleteModal"
          :title="$t('admin.k89')"
          :message="deleteMessage"
          :confirm-text="$t('admin.delete')"
          :cancel-text="$t('admin.cancel')"
          type="danger"
          @confirm="handleDelete"
        />
      </template>
    </AdminCrudPage>
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, shallowRef, onMounted, computed, watch } from 'vue';
import { useRouter } from 'vue-router';
import ApIcon from '@/components/apple/ApIcon.vue';
import { cv, AppleLayout, AppleSpace, AppleRadius, AppleFont, AppleFontWeight, AppleType, AppleLineHeight, appleFixedColumns, appleCardGrid } from '@/utils/core/apple-tokens';
import { loadingStyle, actionGroupStyle, entityMetaRowStyle, entityMetaItemStyle, filterSelectGroupStyle } from '@/styles/layout/page-styles';
import { useBreakpoint } from '@/utils/responsive';
import { useAdminCrud } from '@/composables/shared/useAdminCrud';
import AdminCrudPage from '@/components/admin/AdminCrudPage.vue';
import type { StatConfig } from '@/components/admin/AdminCrudPage.vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApFormItem from '@/components/apple/ApFormItem.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApTextarea from '@/components/apple/ApTextarea.vue';
import ApSelect from '@/components/apple/ApSelect.vue';
import ApSegmentedControl from '@/components/apple/ApSegmentedControl.vue';
import ApSwitch from '@/components/apple/ApSwitch.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApTable from '@/components/apple/ApTable.vue';
import ApEntityCard from '@/components/apple/ApEntityCard.vue';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import { formatTime } from '@/utils/format/format-time';
import { validators } from '@/utils/core/validators';
import { useUIStore } from '@/stores/app/ui';
import { getAllSpaces, createSpace, updateSpace, deleteSpace, downloadSpace, sortSpaces, preCheckDeleteSpace } from '@/api/modules/space/space';
import type { CreateSpaceRequest, UpdateSpaceRequest, SpaceOrder } from '@/api/modules/space/space';

interface SpaceItem {
  id: number;
  name: string;
  creator_name?: string;
  created_by: number;
  visit_level: string;
  doc_count?: number;
  member_count?: number;
  create_time: number;
  is_share?: number;
  is_export?: number;
  description?: string;
  tags?: string;
}

const router = useRouter();
const ui = useUIStore();
const { isMobile, isTablet } = useBreakpoint();

const filterVisitLevel = shallowRef('');
const filterTags = shallowRef('');

const filterVisitLevelOptions = [
  { label: t('admin.all'), value: '' },
  { label: t('admin.k93'), value: '0' },
  { label: t('admin.k94'), value: '1' },
  { label: t('admin.k95'), value: '2' },
];

const {
  loading, page, loadError, retryLoad, items, keyword,
  showCreateModal, showEditModal, showDeleteModal, editingItem,
  formErrors, viewMode,
  fetchList, openCreate, openEdit, openDelete, closeModals,
  handleDelete, handleSubmit,
} = useAdminCrud<SpaceItem>({
  fetchFn: async (p) => {
    const params: { page: number; page_size: number; visit_level?: string; tags?: string } = { page: p.page, page_size: 200 };
    if (filterVisitLevel.value) params.visit_level = filterVisitLevel.value;
    if (filterTags.value) params.tags = filterTags.value;
    const res = await getAllSpaces(params);
    const data = res.data || [];
    return { data, total: res.total ?? data.length, page: p.page, page_size: 200 };
  },
  createFn: (d) => createSpace({ name: d.name!, description: d.description, tags: d.tags, visit_level: d.visit_level, is_share: !!d.is_share, is_export: !!d.is_export } as CreateSpaceRequest),
  updateFn: (d) => updateSpace(d.id!, { name: d.name!, description: d.description, tags: d.tags, visit_level: d.visit_level, is_share: !!d.is_share, is_export: !!d.is_export } as UpdateSpaceRequest),
  deleteFn: (id) => deleteSpace(id),
  itemName: t('adminForm.k20'),
  resetForm: () => { form.value = { name: '', description: '', tags: '', visit_level: '0', is_share: true, is_export: true }; },
});

Object.assign(formErrors, { name: '', description: '' });

const isEdit = computed(() => !!editingItem.value);
const submitting = shallowRef(false);

const form = ref<{ name: string; description: string; tags: string; visit_level: string; is_share: boolean; is_export: boolean }>({ name: '', description: '', tags: '', visit_level: '0', is_share: true, is_export: true });

const visitLevelOptions = [
  { label: t('admin.k93'), value: '0' },
  { label: t('admin.k94'), value: '1' },
  { label: t('admin.k95'), value: '2' },
];

const publicSpacesCount = computed(() => items.value.filter(s => s.visit_level === 'public' || s.visit_level === '0').length);
const privateSpacesCount = computed(() => items.value.filter(s => s.visit_level === 'private' || s.visit_level === '2').length);
const totalDocuments = computed(() => items.value.reduce((sum, s) => sum + (s.doc_count ?? 0), 0));



const spaceStats = computed<StatConfig[]>(() => [
  { label: t('admin.k96'), value: items.value.length, icon: 'FolderOpen', color: cv('blue') },
  { label: t('admin.k97'), value: publicSpacesCount.value, icon: 'Globe', color: cv('green') },
  { label: t('admin.k98'), value: privateSpacesCount.value, icon: 'Lock', color: cv('purple') },
  { label: t('admin.k99'), value: totalDocuments.value, icon: 'FileText', color: cv('orange') },
]);

function visitLevelText(level: string): string {
  const map: Record<string, string> = {
    '0': t('adminForm.k64'),
    '1': t('adminForm.k65'),
    '2': t('adminForm.k66')
  };
  return map[level] || level;
}

function clearFilters() {
  const hadValue = filterVisitLevel.value !== '' || filterTags.value !== '';
  filterVisitLevel.value = '';
  filterTags.value = '';
  if (!hadValue) fetchList();
}

function openCreateModal() {
  form.value = { name: '', description: '', tags: '', visit_level: '0', is_share: true, is_export: true };
  openCreate();
}

function openEditModal(space: SpaceItem) {
  form.value = { name: space.name, description: space.description || '', tags: space.tags || '', visit_level: space.visit_level || '0', is_share: space.is_share === 1, is_export: space.is_export === 1 };
  openEdit(space);
}

async function openDeleteModal(space: SpaceItem) {
  // 先拿权威的影响范围：删除空间会连带其下全部文档与成员关系，只写"不可恢复"
  // 等于让管理员在不知道规模的情况下按下去。pre-check 失败时不假装知道，
  // 也不退回列表里的旧计数冒充刚查过的数。
  deleteMessage.value = t('sentUi.k10', { p1: space.name });
  openDelete(space);
  try {
    const check = await preCheckDeleteSpace(space.id);
    deleteMessage.value = t('sentUi.k11', { p1: space.name, p2: check.document_count, p3: check.member_count });
  } catch {
    deleteMessage.value = t('sentUi.k12', { p1: space.name });
  }
}

const deleteMessage = shallowRef('');

const expandedSpaceId = shallowRef<number | null>(null);
const expandedSpace = computed(() => filteredSpaces.value.find(s => s.id === expandedSpaceId.value) || null);
const expandActiveTab = shallowRef<'members' | 'docs' | 'settings'>('members');
const expandTabs = [
  { value: 'members' as const, label: t('admin.k77') },
  { value: 'docs' as const, label: t('admin.k42') },
  { value: 'settings' as const, label: t('admin.k100') },
];

function toggleSpaceExpand(id: number) {
  expandedSpaceId.value = expandedSpaceId.value === id ? null : id;
  expandActiveTab.value = 'members';
}

const spacesGridStyle = computed(() => {
  if (isMobile.value) {
    return {
      display: 'grid' as const,
      gridTemplateColumns: '1fr',
      gridAutoRows: '1fr', alignContent: 'start',
      gap: AppleSpace.md,
    };
  }
  if (isTablet.value) {
    return {
      display: 'grid' as const,
      gridTemplateColumns: appleFixedColumns(2),
      gridAutoRows: '1fr', alignContent: 'start',
      gap: AppleSpace.md,
    };
  }
  return appleCardGrid('entity', AppleSpace.md);
});



const expandPanelStyle = {
  padding: `${AppleSpace.sm} ${AppleSpace.md}`,
  background: cv('bg-secondary'),
  borderRadius: `0 0 ${AppleRadius.card} ${AppleRadius.card}`,
  border: `1px solid ${cv('border-light')}`,
  borderTop: 'none',
};

const expandTabBarStyle = {
  display: 'flex',
  gap: AppleSpace.xs,
  marginBottom: AppleSpace.sm,
};

const expandContentStyle = {
  padding: AppleSpace.sm,
  background: cv('bg-secondary'),
  borderRadius: AppleRadius.card,
  marginBottom: AppleSpace.sm,
};

const expandInfoStyle = {
  fontFamily: AppleFont.text,
  fontSize: AppleType.controlLabel.size,
  color: cv('text-primary'),
  lineHeight: AppleLineHeight.relaxed,
};

async function handleDownload(space: SpaceItem) {
  try {
    const blob = await downloadSpace(space.id);
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `${space.name}.zip`;
    a.click();
    URL.revokeObjectURL(url);
  } catch {
    ui.showToast(t('admin.k103'), 'error');
  }
}

async function onSave() {
  if (submitting.value) return;
  submitting.value = true;
  try {
    await handleSubmit(
      { name: form.value.name, description: form.value.description, tags: form.value.tags, visit_level: form.value.visit_level, is_share: form.value.is_share ? 1 : 0, is_export: form.value.is_export ? 1 : 0 } as Partial<SpaceItem>,
      (data) => {
        const errors: Record<string, string> = {};
        if (!data.name?.trim()) errors.name = t('adminForm.k67');
        else if (!validators.spaceName(data.name)) errors.name = t('adminForm.k68');
        if (data.description && !validators.spaceDescription(data.description)) errors.description = t('adminForm.k69');
        return Object.keys(errors).length > 0 ? errors : null;
      }
    );
  } finally {
    submitting.value = false;
  }
}

const filteredSpaces = computed(() => {
  const kw = keyword.value.trim().toLowerCase();
  return kw ? items.value.filter(s => s.name.toLowerCase().includes(kw)) : items.value;
});

const sortMode = shallowRef(false);
const sortSaving = shallowRef(false);
const localSpaces = ref<SpaceItem[]>([]);
const displaySpaces = computed(() => (sortMode.value ? localSpaces.value : filteredSpaces.value));

function enterSortMode() {
  localSpaces.value = [...filteredSpaces.value];
  sortMode.value = true;
}

async function exitSortMode() {
  if (sortSaving.value) return;
  sortSaving.value = true;
  try {
    const orders: SpaceOrder[] = localSpaces.value.map((s, idx) => ({ id: s.id, sequence: idx + 1 }));
    await sortSpaces(orders);
    ui.showToast(t('admin.k104'), 'success');
    sortMode.value = false;
    await fetchList();
  } catch {
    ui.showToast(t('admin.k105'), 'error');
  } finally {
    sortSaving.value = false;
  }
}

function moveSpace(idx: number, dir: -1 | 1) {
  const target = idx + dir;
  if (target < 0 || target >= localSpaces.value.length) return;
  const arr = [...localSpaces.value];
  const tmp = arr[idx]!;
  arr[idx] = arr[target]!;
  arr[target] = tmp;
  localSpaces.value = arr;
}

function sequenceBadgeStyle() {
  return {
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    minWidth: '18px',
    height: '18px',
    padding: `0 ${AppleSpace.xs}`,
    borderRadius: AppleRadius.pill,
    background: cv('blue'),
    color: cv('on-blue'),
    fontSize: AppleType.microUI.size,
    fontWeight: AppleFontWeight.semibold,
    fontFamily: AppleType.controlLabel.font,
  };
}

onMounted(() => {
  fetchList();
});

watch([filterVisitLevel, filterTags], () => {
  page.value = 1;
  fetchList();
}, { immediate: false });

const spaceColumns: TableColumn<SpaceItem>[] = [
  { key: 'name', title: t('admin.k78') },
  { key: 'creator_name', title: t('admin.k101') },
  { key: 'visit_level', title: t('admin.k80'), render: (row: SpaceItem) => visitLevelText(row.visit_level) },
  { key: 'doc_count', title: t('admin.k102') },
  { key: 'create_time', title: t('admin.k40'), render: (row: SpaceItem) => formatTime(row.create_time) },
];
</script>
