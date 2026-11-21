<template>
  <ApPageContainer
    :title="$t('search.k24')"
    :subtitle="$t('search.k26')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <template #actions>
      <ApButton variant="primary" size="small" @click="openModal()">
        <ApIcon name="Plus" :size="16" /> {{ $t('search.k1') }}
      </ApButton>
    </template>

    <div :style="listStyle()">
      <!-- 我的 -->
      <ApCard size="small">
        <template #header>
          <div :style="headerRowStyle">
            <div :style="{ display: 'inline-flex', alignItems: 'center', gap: AppleSpace.sm }">
              <ApIcon name="BookmarkCheck" :size="18" :color="cv('blue')" />
              <span :style="sectionTitleStyle">{{ $t('search.k2') }}</span>
              <span :style="countBadgeStyle">{{ mine.length }}</span>
            </div>
            <ApButton variant="ghost" size="small" :loading="mineLoading" @click="loadMine">{{ $t('search.refresh') }}</ApButton>
          </div>
        </template>

        <ApTable :columns="mineColumns" :data="mine" :loading="mineLoading" density="compact" hide-header-when-empty>
          <template #empty>
            <ApEmpty :icon="'BookmarkCheck'" :title="$t('search.k90')" :description="$t('search.k91')" />
          </template>
          <template #actions="{ row }">
            <div :style="actionGroupStyle">
              <ApButton
                variant="secondary"
                size="small"
                :loading="usingId === (row as unknown as SavedSearch).id"
                @click="handleUse(row as unknown as SavedSearch)"
              >
                <ApIcon name="Play" :size="16" /> {{ $t('search.k4') }}
              </ApButton>
              <ApButton variant="ghost" size="small" @click="openModal(row as unknown as SavedSearch)">{{ $t('search.edit') }}</ApButton>
              <ApButton variant="danger" size="small" @click="askDelete(row as unknown as SavedSearch)">
                <ApIcon name="Trash2" :size="16" /> {{ $t('search.delete') }}
              </ApButton>
            </div>
          </template>
        </ApTable>
      </ApCard>

      <!-- 公开 -->
      <ApCard size="small">
        <template #header>
          <div :style="headerRowStyle">
            <div :style="{ display: 'inline-flex', alignItems: 'center', gap: AppleSpace.sm, flexShrink: '0' }">
              <ApIcon name="Globe" :size="18" :color="cv('green')" />
              <!-- 标题是源码写死的短标签，长度有上界，不该被右边的下拉挤成两行
                   （1440 宽实测「公开热/门」断成两行，头部行就此散掉） -->
              <span :style="{ ...sectionTitleStyle, whiteSpace: 'nowrap' }">{{ $t('search.k7') }}</span>
              <span :style="countBadgeStyle">{{ publicList.length }}</span>
            </div>
            <ApSelect v-model="publicLimit" :options="publicLimitOptions" size="small" @update:model-value="loadPublic" />
          </div>
        </template>

        <ApTable :columns="publicColumns" :data="publicList" :loading="publicLoading" density="compact" hide-header-when-empty>
          <template #empty>
            <ApEmpty :icon="'Globe'" :title="$t('search.k92')" :description="$t('search.k93')" />
          </template>
          <template #actions="{ row }">
            <div :style="actionGroupStyle">
              <ApButton
                variant="secondary"
                size="small"
                :loading="usingId === (row as unknown as SavedSearch).id"
                @click="handleUse(row as unknown as SavedSearch)"
              >
                <ApIcon name="Play" :size="16" /> {{ $t('search.k4') }}
              </ApButton>
              <ApButton
                v-if="(row as unknown as SavedSearch).user_id === currentUserId"
                variant="ghost"
                size="small"
                @click="openModal(row as unknown as SavedSearch)"
              >
                {{ $t('search.edit') }}
              </ApButton>
            </div>
          </template>
        </ApTable>
      </ApCard>

      <p :style="noteStyle">{{ $t('search.k8') }}</p>
    </div>

    <!-- 新建 / 编辑 -->
    <ApModal v-model="showModal" :title="editingId ? $t('search.k86') : $t('search.k87')" size="medium">
      <ApFormItem :label="$t('search.name')" required :error="nameError">
        <ApInput v-model="form.name" :placeholder="$t('search.k18')" :maxlength="100" />
      </ApFormItem>

      <ApFormItem :label="$t('search.k11')" required :error="queryError">
        <ApInput v-model="form.query" :placeholder="$t('search.k19')" :maxlength="500" />
      </ApFormItem>

      <div :style="twoColumnStyle">
        <ApFormItem :label="$t('search.k12')">
          <ApSelect v-model="form.type" :options="typeOptions" :placeholder="$t('search.all')" />
        </ApFormItem>
        <ApFormItem :label="$t('search.k13')">
          <ApSelect v-model="form.sort_by" :options="sortOptions" :placeholder="$t('search.k21')" />
        </ApFormItem>
      </div>

      <div :style="twoColumnStyle">
        <ApFormItem :label="$t('search.k14')">
          <ApSelect v-model="form.space_id" :options="spaceOptions" :placeholder="$t('search.k22')" searchable />
        </ApFormItem>
        <ApFormItem :label="$t('search.k15')">
          <ApSelect v-model="form.tag_filter" :options="tagOptions" :placeholder="$t('search.k23')" searchable />
        </ApFormItem>
      </div>

      <ApFormItem :label="$t('search.k16')" :hint="editingId ? $t('search.k88') : $t('search.k89')">
        <ApSwitch v-model="form.isPublic" :label="$t('search.k17')" />
      </ApFormItem>

      <template #footer>
        <ApButton variant="ghost" size="small" @click="showModal = false">{{ $t('search.cancel') }}</ApButton>
        <ApButton variant="primary" size="small" :loading="saving" @click="handleSave">
          {{ editingId ? $t('vSearchUi.k1') : $t('vSearchUi.create2') }}
        </ApButton>
      </template>
    </ApModal>

    <ApConfirmModal
      v-model="showDeleteModal"
      :title="$t('search.k25')"
      :message="$t('uiTextMain.k31', { p1: deletingItem?.name })"
      type="danger"
      :confirm-text="$t('search.delete')"
      :loading="deleting"
      @confirm="confirmDelete"
    />
  </ApPageContainer>
</template>

<script setup lang="ts">
const { t } = useI18n();
import { useI18n } from 'vue-i18n';
import { ref, computed, onMounted } from 'vue';
import { useRouter } from 'vue-router';
import { logger } from '@/utils/logger';
import { cv, AppleLayout, AppleSpace } from '@/utils/core/apple-tokens';
import { formatTime } from '@/utils/format/format-time';
import {
  listStyle,
  actionGroupStyle,
  formRowStyle as twoColumnStyle,
  aiSubSectionHeaderStyle as headerRowStyle,
  aiSubSectionTitleStyle as sectionTitleStyle,
  aiSubCountBadgeStyle as countBadgeStyle,
  noteStyle,
} from '@/styles/ai/ai-page-styles';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import {
  createSavedSearch,
  deleteSavedSearch,
  getMySavedSearches,
  getPublicSavedSearches,
  updateSavedSearch,
  useSavedSearch,
  type SavedSearch,
  type SavedSearchSort,
  type SavedSearchType,
} from '@/api/modules/document/saved-search';
import { getAllTags } from '@/api/modules/document/tags';
import { useSpaceStore } from '@/stores/workspace/space';
import { useAuthStore } from '@/stores/auth/auth';
import { useUIStore } from '@/stores/app/ui';
import type { Space } from '@/types/models';

const router = useRouter();
const ui = useUIStore();
const auth = useAuthStore();
const spaceStore = useSpaceStore();

const NO_SPACE = 0;
const NO_TAG = '';

const mine = ref<SavedSearch[]>([]);
const publicList = ref<SavedSearch[]>([]);
const spaces = ref<Space[]>([]);
const tags = ref<string[]>([]);
const mineLoading = ref(false);
const publicLoading = ref(false);
const saving = ref(false);
const usingId = ref<number | null>(null);
const deleting = ref(false);
const publicLimit = ref<number | string>(10);

const showModal = ref(false);
const editingId = ref<number | null>(null);
const showDeleteModal = ref(false);
const deletingItem = ref<SavedSearch | null>(null);
const nameError = ref('');
const queryError = ref('');

const form = ref({
  name: '',
  query: '',
  type: 'all' as SavedSearchType,
  sort_by: 'relevance' as SavedSearchSort,
  space_id: NO_SPACE as number | string,
  tag_filter: NO_TAG as string,
  isPublic: false,
});

const currentUserId = computed(() => auth.user?.id ?? 0);

const typeOptions = [
  { label: t('search.all'), value: 'all' },
  { label: t('search.k27'), value: 'title' },
  { label: t('search.k28'), value: 'content' },
];

const sortOptions = [
  { label: t('search.k21'), value: 'relevance' },
  { label: t('search.k29'), value: 'time_desc' },
  { label: t('search.k30'), value: 'time_asc' },
];

const publicLimitOptions = [
  { label: 'Top 10', value: 10 },
  { label: 'Top 20', value: 20 },
  { label: 'Top 50', value: 50 },
];

const spaceOptions = computed(() => [
  { label: t('search.k22'), value: NO_SPACE },
  ...spaces.value.map((s) => ({ label: s.name, value: s.id })),
]);

const tagOptions = computed(() => [
  { label: t('search.k23'), value: NO_TAG },
  ...tags.value.map((t) => ({ label: t, value: t })),
]);

const spaceNames = computed(() => {
  const map = new Map<number, string>();
  spaces.value.forEach((s) => map.set(s.id, s.name));
  return map;
});

const typeLabels: Record<string, string> = { all: t('search.all'), title: t('search.k27'), content: t('search.k28') };
const sortLabels: Record<string, string> = { relevance: t('search.k21'), time_desc: t('search.k29'), time_asc: t('search.k30') };

const baseColumns = computed<TableColumn<SavedSearch>[]>(() => [
  { key: 'name', title: t('search.name'), width: '18%', ellipsis: true },
  { key: 'query', title: t('search.k31'), ellipsis: true },
  { key: 'type', title: t('search.k32'), width: '10%', render: (row) => typeLabels[row.type] ?? row.type },
  { key: 'sort_by', title: t('search.sort'), width: '12%', render: (row) => sortLabels[row.sort_by] ?? row.sort_by },
  { key: 'tag_filter', title: t('search.k34'), width: '12%', render: (row) => row.tag_filter || '—' },
  { key: 'space_id', title: t('search.k35'), width: '14%', render: (row) => spaceNames.value.get(row.space_id) ?? t('fallback.k11') },
  { key: 'use_count', title: t('search.k36'), width: '10%' },
  { key: 'update_time', title: t('search.k37'), width: '16%', render: (row) => formatTime(row.update_time) },
]);

const mineColumns = computed<TableColumn<SavedSearch>[]>(() => [
  ...baseColumns.value,
  { key: 'is_public', title: t('search.k38'), width: '10%', render: (row) => (row.is_public === 1 ? t('uiTextMain.k32') : t('uiTextMain.k33')) },
]);

const publicColumns = computed<TableColumn<SavedSearch>[]>(() => baseColumns.value);

async function loadMine() {
  mineLoading.value = true;
  try {
    const res = await getMySavedSearches();
    mine.value = res ?? [];
  } catch (e) {
    logger.error('Failed to load my saved searches:', e);
    ui.showToast((e as Error).message || t('fallback.k32'), 'error');
  } finally {
    mineLoading.value = false;
  }
}

async function loadPublic() {
  publicLoading.value = true;
  try {
    const res = await getPublicSavedSearches(Number(publicLimit.value) || 10);
    publicList.value = res ?? [];
  } catch (e) {
    logger.error('Failed to load public saved searches:', e);
    ui.showToast((e as Error).message || t('fallback.k24'), 'error');
  } finally {
    publicLoading.value = false;
  }
}

async function loadSpaces() {
  try {
    await spaceStore.loadMySpaces({ page_size: 100 });
    spaces.value = spaceStore.mySpaces;
  } catch (e) {
    logger.error('Failed to load spaces:', e);
  }
}

async function loadTags() {
  try {
    const res = await getAllTags({ limit: 200 });
    tags.value = res ?? [];
  } catch (e) {
    logger.error('Failed to load tags:', e);
  }
}

function resetForm() {
  form.value = {
    name: '',
    query: '',
    type: 'all',
    sort_by: 'relevance',
    space_id: NO_SPACE,
    tag_filter: NO_TAG,
    isPublic: false,
  };
  nameError.value = '';
  queryError.value = '';
}

function openModal(item?: SavedSearch) {
  if (item) {
    editingId.value = item.id;
    form.value = {
      name: item.name,
      query: item.query,
      type: item.type || 'all',
      sort_by: item.sort_by || 'relevance',
      space_id: item.space_id,
      tag_filter: item.tag_filter,
      isPublic: item.is_public === 1,
    };
  } else {
    editingId.value = null;
    resetForm();
  }
  showModal.value = true;
}

function validate(): boolean {
  nameError.value = form.value.name.trim() ? '' : t('uiTextMain.k34');
  queryError.value = form.value.query.trim() ? '' : t('uiTextMain.k35');
  return !nameError.value && !queryError.value;
}

async function handleSave() {
  if (!validate()) return;
  saving.value = true;
  const payload = {
    name: form.value.name.trim(),
    query: form.value.query.trim(),
    type: form.value.type,
    sort_by: form.value.sort_by,
    space_id: Number(form.value.space_id) || NO_SPACE,
    tag_filter: String(form.value.tag_filter ?? ''),
    is_public: (form.value.isPublic ? 1 : 0) as 0 | 1,
  };
  try {
    if (editingId.value) {
      await updateSavedSearch(editingId.value, payload);
      ui.showToast(t('search.k39'), 'success');
    } else {
      await createSavedSearch(payload);
      ui.showToast(t('search.k40'), 'success');
    }
    showModal.value = false;
    await Promise.all([loadMine(), loadPublic()]);
  } catch (e) {
    logger.error('Failed to save search:', e);
    ui.showToast((e as Error).message || t('fallback.k8'), 'error');
  } finally {
    saving.value = false;
  }
}

async function handleUse(item: SavedSearch) {
  usingId.value = item.id;
  try {
    await useSavedSearch(item.id);
    await Promise.all([loadMine(), loadPublic()]);
    router.push({ path: '/search', query: { q: item.query } });
  } catch (e) {
    logger.error('Failed to use saved search:', e);
    ui.showToast((e as Error).message || t('fallback.k102'), 'error');
  } finally {
    usingId.value = null;
  }
}

function askDelete(item: SavedSearch) {
  deletingItem.value = item;
  showDeleteModal.value = true;
}

async function confirmDelete() {
  const target = deletingItem.value;
  if (!target) return;
  deleting.value = true;
  try {
    await deleteSavedSearch(target.id);
    ui.showToast(t('search.k41'), 'success');
    showDeleteModal.value = false;
    deletingItem.value = null;
    await Promise.all([loadMine(), loadPublic()]);
  } catch (e) {
    logger.error('Failed to delete saved search:', e);
    ui.showToast((e as Error).message || t('fallback.k20'), 'error');
  } finally {
    deleting.value = false;
  }
}

onMounted(() => {
  loadMine();
  loadPublic();
  loadSpaces();
  loadTags();
});
</script>
