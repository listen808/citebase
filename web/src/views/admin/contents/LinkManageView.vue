<template>
  <ApPageContainer
    :title="$t('admin.k30')"
    :subtitle="$t('admin.k33')"
    :max-width="AppleLayout.maxWideContentWidth"
    :show-breadcrumb="true"
    compact
  >
    <AdminCrudPage
      v-model:keyword="keyword"
      v-model:view-mode="viewMode"
      :error-text="loadError"
      :stats="linkStats"
      :filters="{ search: { placeholder: $t('vAdminUi.k7') }, viewToggle: true }"
      :loading="loading"
      :total="total"
      :page="page"
      :page-size="pageSize"
      density="compact"
      @retry="retryLoad"
      @search="handleSearch"
      @page-change="changePage"
    >
      <template #actions>
        <ApButton variant="primary" size="small" @click="openCreateModal"><ApIcon name="Plus" :size="16" />{{ $t('admin.k25') }}</ApButton>
      </template>

      <template #content>
        <template v-if="viewMode === 'card'">
          <div v-if="loading" :style="{ ...appleCardGrid('entity'), marginBottom: AppleSpace.md }">
            <ApCard v-for="i in 3" :key="i" :padding="`${AppleSpace.md} ${AppleSpace.lg}`" accent-color="transparent">
              <div :style="{ display: 'flex', alignItems: 'center', gap: AppleSpace.md }">
                <ApSkeleton variant="circular" :style="{ width: AppleSpace['2xl'], height: AppleSpace['2xl'] }" />
                <div :style="{ flex: '1' }">
                  <ApSkeleton :style="{ width: '100px', height: AppleSpace.md, marginBottom: AppleSpace.sm }" />
                  <ApSkeleton :style="{ width: '150px', height: AppleSpace.md }" />
                </div>
              </div>
            </ApCard>
          </div>
          <ApEmpty v-else-if="items.length === 0" icon="Link" :title="$t('admin.k31')" />
          <div v-else :style="appleCardGrid('entity')">
            <ApEntityCard
              v-for="link in items"
              :key="link.id"
              :title="link.name"
            >
              <div :style="entityMetaRowStyle()">
                <a
                  :href="link.url"
                  target="_blank"
                  rel="noopener noreferrer"
                  :style="inlineLinkStyle"
                  :title="link.url"
                  @click="reportClick(link)"
                >
                  <ApIcon name="ExternalLink" :size="12" />{{ link.url }}
                </a>
              </div>
              <div :style="entityMetaRowStyle()">
                <span :style="entityMetaItemStyle()">
                  <ApIcon name="MousePointer" :size="12" />
                  {{ link.click_count ?? 0 }} {{ $t('vAdminUi.k6') }}
                </span>
                <span :style="entityMetaItemStyle()">
                  <ApIcon name="ArrowUpDown" :size="12" />
                  {{ $t('admin.sort') }} {{ link.sort }}
                </span>
              </div>
              <template #actions>
                <div :style="actionGroupStyle">
                  <ApButton variant="ghost" size="small" @click="openEditModal(link)">{{ $t('admin.edit') }}</ApButton>
                  <ApButton variant="ghost" size="small" @click="openDeleteModal(link)"><ApIcon name="Trash2" :size="16" :style="{ color: cv('red') }" />{{ $t('admin.delete') }}</ApButton>
                </div>
              </template>
            </ApEntityCard>
          </div>
        </template>
        <template v-else>
          <ApTable :columns="linkColumns" :data="items" :loading="loading" density="compact">
            <template #actions="{ row: link }">
              <div :style="actionGroupStyle">
                <ApButton variant="ghost" size="small" @click="openEditModal(link)">{{ $t('admin.edit') }}</ApButton>
                <ApButton variant="ghost" size="small" @click="openDeleteModal(link)"><ApIcon name="Trash2" :size="16" :style="{ color: cv('red') }" />{{ $t('admin.delete') }}</ApButton>
              </div>
            </template>
          </ApTable>
        </template>
      </template>

      <template #modals>
        <ApModal :model-value="showCreateModal || showEditModal" :title="isEdit ? $t('adminForm.k9') : $t('adminForm.k10')" size="small" density="compact" @update:model-value="handleModalClose">
          <ApFormItem :label="$t('admin.name')" required :error="formErrors.name">
            <ApInput v-model="form.name" :placeholder="$t('admin.k29')" />
          </ApFormItem>
          <ApFormItem :label="$t('admin.k27')" required :error="formErrors.url">
            <ApInput v-model="form.url" placeholder="https://" />
          </ApFormItem>
          <ApFormItem :label="$t('admin.sort')" :error="formErrors.sort">
            <div :style="spinnerWrapStyle">
              <ApInput v-model="form.sort" type="number" placeholder="0" :style="{ flex: 1, textAlign: 'center' }" />
              <div :style="spinnerBtnsStyle">
                <ApButton variant="secondary" icon-only type="button" size="mini" :aria-label="$t('admin.k34')" @click="incSort"><ApIcon name="ChevronUp" :size="16" /></ApButton>
                <ApButton variant="secondary" icon-only type="button" size="mini" :aria-label="$t('admin.k35')" @click="decSort"><ApIcon name="ChevronDown" :size="16" /></ApButton>
              </div>
            </div>
          </ApFormItem>
          <template #footer>
            <ApButton variant="ghost" size="small" @click="handleModalClose">{{ $t('admin.cancel') }}</ApButton>
            <ApButton variant="primary" size="small" :disabled="submitting" @click="onSave">{{ submitting ? $t('vAdminUi.k1') : $t('vAdminUi.save') }}</ApButton>
          </template>
        </ApModal>

        <ApConfirmModal
          v-model="showDeleteModal"
          :title="$t('admin.k32')"
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
import { ref, onMounted, computed } from 'vue';
import ApIcon from '@/components/apple/ApIcon.vue';
import { AppleLayout, AppleSpace, cv, appleCardGrid } from '@/utils/core/apple-tokens';
import { actionGroupStyle, entityMetaRowStyle, entityMetaItemStyle, inlineLinkStyle } from '@/styles/layout/page-styles';
import { getLinks, saveLink, updateLink, deleteLink, reportLinkClick } from '@/api/modules/content/link';
import type { Link } from '@/api/modules/content/link';
import { validators } from '@/utils/core/validators';
import { useAdminCrud } from '@/composables/shared/useAdminCrud';
import AdminCrudPage from '@/components/admin/AdminCrudPage.vue';
import type { StatConfig } from '@/components/admin/AdminCrudPage.vue';
import ApModal from '@/components/apple/ApModal.vue';
import ApButton from '@/components/apple/ApButton.vue';
import ApInput from '@/components/apple/ApInput.vue';
import ApCard from '@/components/apple/ApCard.vue';
import ApEntityCard from '@/components/apple/ApEntityCard.vue';
import ApPageContainer from '@/components/apple/ApPageContainer.vue';
import ApConfirmModal from '@/components/apple/ApConfirmModal.vue';
import ApSkeleton from '@/components/apple/ApSkeleton.vue';
import ApFormItem from '@/components/apple/ApFormItem.vue';
import ApTable from '@/components/apple/ApTable.vue';
import type { TableColumn } from '@/components/apple/ApTable.vue';
import { formatTime } from '@/utils/format/format-time';

const {
  loading, loadError, retryLoad, submitting, items, total, page, pageSize, keyword,
  showCreateModal, showEditModal, showDeleteModal,
  formErrors, viewMode,
  fetchList, handleSearch, changePage, openCreate, openEdit, openDelete,
  handleDelete, handleSubmit,
} = useAdminCrud<Link>({
  fetchFn: (p) => getLinks(p.page, p.pageSize, p.keyword),
  createFn: (d) => saveLink(d.name!, d.url!, d.sort ?? 0),
  updateFn: (d) => updateLink(d.id!, d.name!, d.url!, d.sort ?? 0),
  deleteFn: (id) => deleteLink(id),
  itemName: t('adminForm.k19'),
  resetForm: () => { form.value = { name: '', url: '', sort: '0' }; },
});

Object.assign(formErrors, { name: '', url: '', sort: '' });

const form = ref({ name: '', url: '', sort: '0' });
const deleteMessage = ref('');
const isEdit = computed(() => showEditModal.value);

// 打开动作由 <a href> 自己完成，这里只补一次计数：失败也不该拦住用户要看的页面，
// 所以 fire-and-forget；成功后就地更新本地值，卡片上的"次点击"立刻是新的。
function reportClick(link: Link) {
  reportLinkClick(link.id)
    .then((res) => {
      if (res && typeof res.click_count === 'number') link.click_count = res.click_count;
    })
    .catch(() => { /* 计数失败不影响打开 */ });
}

const totalClicks = computed(() => items.value.reduce((sum, link) => sum + (link.click_count ?? 0), 0));
const mostClickedLink = computed<Link | null>(() => {
  if (items.value.length === 0) return null;
  return items.value.reduce<Link>((max, link) => ((link.click_count ?? 0) > (max.click_count ?? 0) ? link : max), items.value[0]!);
});

const linkStats = computed<StatConfig[]>(() => [
  { label: t('admin.k36'), value: total.value, icon: 'Link', color: cv('blue') },
  { label: t('admin.k37'), value: totalClicks.value, icon: 'MousePointer', color: cv('blue') },
  { label: t('admin.k38'), value: mostClickedLink.value?.name || '-', icon: 'TrendingUp', color: cv('orange') },
]);

function handleModalClose() {
  showCreateModal.value = false;
  showEditModal.value = false;
}

function openCreateModal() {
  form.value = { name: '', url: '', sort: '0' };
  openCreate();
}

function openEditModal(link: Link) {
  form.value = { name: link.name, url: link.url, sort: String(link.sort) };
  openEdit(link);
}

function incSort() {
  form.value.sort = String(Math.max(0, parseInt(form.value.sort, 10) || 0) + 1);
}

function decSort() {
  form.value.sort = String(Math.max(0, (parseInt(form.value.sort, 10) || 0) - 1));
}

function openDeleteModal(link: Link) {
  deleteMessage.value = t('sentUi.k3', { p1: link?.name || t('fallback.k77') });
  openDelete(link);
}

async function onSave() {
  if (submitting.value) return;
  const sortNum = parseInt(form.value.sort, 10) || 0;
  await handleSubmit(
    { name: form.value.name, url: form.value.url, sort: sortNum } as Partial<Link>,
    (data) => {
      const errors: Record<string, string> = {};
      if (!data.name?.trim()) errors.name = t('adminForm.k52');
      if (!data.url?.trim()) errors.url = t('adminForm.k53');
      else if (!validators.url(data.url)) errors.url = t('adminForm.k54');
      return Object.keys(errors).length > 0 ? errors : null;
    }
  );
}

onMounted(fetchList);

const linkColumns: TableColumn<Link>[] = [
  { key: 'name', title: t('admin.k29') },
  { key: 'url', title: t('admin.k27'), render: (row: Link) => row.url },
  { key: 'click_count', title: t('admin.k39'), render: (row: Link) => row.click_count ?? 0 },
  { key: 'sort', title: t('admin.sort') },
  { key: 'create_time', title: t('admin.k40'), render: (row: Link) => row.create_time ? formatTime(row.create_time) : '-' },
];

const spinnerWrapStyle = {
  display: 'flex',
  alignItems: 'center',
  gap: AppleSpace.xxs,
};

const spinnerBtnsStyle = {
  display: 'flex',
  flexDirection: 'column' as const,
  gap: AppleSpace.xxs,
};
</script>
